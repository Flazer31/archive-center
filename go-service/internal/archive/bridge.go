package archive

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	scopedVerbatimPolicyVersion     = "vr18a.v1"
	scopedVerbatimSurfaceLabel      = "Scoped Verbatim Recall (support surface)"
	scopedVerbatimSurfaceRoute      = "scoped_verbatim_support"
	scopedVerbatimPromptStrategy    = "latest_anchor_only"
	scopedVerbatimMaxItems          = 3
	scopedVerbatimMaxTotalChars     = 720
	scopedVerbatimMaxExcerptChars   = 160
	scopedVerbatimSupportSourceKind = "direct_evidence"
)

// Bridge is the Go-side ArchiveBridge/LibraryBridge read adapter.
type Bridge struct {
	store chatLogLister
}

type chatLogLister interface {
	ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.ChatLog, error)
}

type VerbatimTurn struct {
	TurnIndex int               `json:"turn_index"`
	Messages  []VerbatimMessage `json:"messages"`
}

type VerbatimMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ScopedVerbatimSupport struct {
	Active                  bool                 `json:"active"`
	PolicyVersion           string               `json:"policy_version"`
	SurfaceLabel            string               `json:"surface_label"`
	SupportSurfaceFirst     bool                 `json:"support_surface_first"`
	PromptInjectionStrategy string               `json:"prompt_injection_strategy"`
	SurfaceRoute            string               `json:"surface_route"`
	MaxItems                int                  `json:"max_items"`
	MaxTotalChars           int                  `json:"max_total_chars"`
	MaxExcerptChars         int                  `json:"max_excerpt_chars"`
	Count                   int                  `json:"count"`
	LatestTurnIndex         any                  `json:"latest_turn_index"`
	Text                    string               `json:"text"`
	Items                   []ScopedVerbatimItem `json:"items"`
	SurfacePriority         []string             `json:"surface_priority"`
	Trace                   map[string]any       `json:"trace"`
}

type ScopedVerbatimItem struct {
	SourceTag    string `json:"source_tag"`
	Excerpt      string `json:"excerpt"`
	Scope        string `json:"scope"`
	Turns        string `json:"turns"`
	AnchorTurn   any    `json:"anchor_turn"`
	EvidenceKind string `json:"evidence_kind"`
}

func NewBridge(st store.Store) *Bridge {
	return &Bridge{store: st}
}

// Cosine mirrors Python ArchiveBridge._cosine: invalid vectors return 0.
func Cosine(vec1, vec2 []float64) float64 {
	if len(vec1) == 0 || len(vec2) == 0 || len(vec1) != len(vec2) {
		return 0
	}
	var dot, normA, normB float64
	for i := range vec1 {
		dot += vec1[i] * vec2[i]
		normA += vec1[i] * vec1[i]
		normB += vec2[i] * vec2[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	score := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0
	}
	return score
}

func (b *Bridge) GetVerbatimByTurn(ctx context.Context, turnIndex int, chatSessionID string) (*VerbatimTurn, error) {
	if b == nil || b.store == nil {
		return nil, store.ErrNotEnabled
	}
	rows, err := b.store.ListChatLogs(ctx, strings.TrimSpace(chatSessionID), turnIndex, turnIndex)
	if err != nil {
		return nil, err
	}
	messages := make([]VerbatimMessage, 0, len(rows))
	for _, row := range rows {
		if row.TurnIndex != turnIndex {
			continue
		}
		content := strings.TrimSpace(row.Content)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(row.Role)
		if role == "" {
			role = "unknown"
		}
		messages = append(messages, VerbatimMessage{Role: role, Content: content})
	}
	if len(messages) == 0 {
		return nil, nil
	}
	return &VerbatimTurn{TurnIndex: turnIndex, Messages: messages}, nil
}

func BuildScopedVerbatimSupport(evidence []store.DirectEvidence) ScopedVerbatimSupport {
	rows := append([]store.DirectEvidence(nil), evidence...)
	sort.SliceStable(rows, func(i, j int) bool {
		ai := evidenceAnchorTurn(rows[i])
		aj := evidenceAnchorTurn(rows[j])
		if ai != aj {
			return ai > aj
		}
		return rows[i].ID > rows[j].ID
	})

	items := make([]ScopedVerbatimItem, 0, scopedVerbatimMaxItems)
	lines := make([]string, 0, scopedVerbatimMaxItems)
	totalChars := 0
	latestTurn := any(nil)

	for _, row := range rows {
		if len(items) >= scopedVerbatimMaxItems {
			break
		}
		if row.Tombstoned || row.RepairNeeded || row.SupersededByID != 0 {
			continue
		}
		excerpt := compactSpaces(row.EvidenceText)
		if excerpt == "" {
			continue
		}
		excerpt = truncateRunes(excerpt, scopedVerbatimMaxExcerptChars)
		anchor := evidenceAnchorTurn(row)
		if latestTurn == nil && anchor > 0 {
			latestTurn = anchor
		}
		scope := "turn_window"
		turns := evidenceTurnsLabel(row)
		kind := strings.TrimSpace(row.EvidenceKind)
		if kind == "" {
			kind = "fact_event"
		}
		anchorLabel := "?"
		if anchor > 0 {
			anchorLabel = fmt.Sprintf("%d", anchor)
		}
		sourceTag := fmt.Sprintf("[source=%s scope=%s turns=%s anchor=%s kind=%s]", scopedVerbatimSupportSourceKind, scope, turns, anchorLabel, kind)
		line := strings.TrimSpace(sourceTag + " " + excerpt)
		projected := totalChars + len(line)
		if len(lines) > 0 {
			projected++
		}
		if projected > scopedVerbatimMaxTotalChars {
			break
		}
		lines = append(lines, line)
		totalChars = projected
		anchorValue := any(nil)
		if anchor > 0 {
			anchorValue = anchor
		}
		items = append(items, ScopedVerbatimItem{
			SourceTag:    sourceTag,
			Excerpt:      excerpt,
			Scope:        scope,
			Turns:        turns,
			AnchorTurn:   anchorValue,
			EvidenceKind: kind,
		})
	}

	return ScopedVerbatimSupport{
		Active:                  len(items) > 0,
		PolicyVersion:           scopedVerbatimPolicyVersion,
		SurfaceLabel:            scopedVerbatimSurfaceLabel,
		SupportSurfaceFirst:     true,
		PromptInjectionStrategy: scopedVerbatimPromptStrategy,
		SurfaceRoute:            scopedVerbatimSurfaceRoute,
		MaxItems:                scopedVerbatimMaxItems,
		MaxTotalChars:           scopedVerbatimMaxTotalChars,
		MaxExcerptChars:         scopedVerbatimMaxExcerptChars,
		Count:                   len(items),
		LatestTurnIndex:         latestTurn,
		Text:                    strings.Join(lines, "\n"),
		Items:                   items,
		SurfacePriority:         []string{"latest_direct_evidence", "recent_raw_turn"},
		Trace: map[string]any{
			"source":         "go_archive_bridge",
			"candidate_rows": len(rows),
			"selected_rows":  len(items),
		},
	}
}

func evidenceAnchorTurn(row store.DirectEvidence) int {
	if row.TurnAnchor > 0 {
		return row.TurnAnchor
	}
	if row.SourceTurnEnd > 0 {
		return row.SourceTurnEnd
	}
	return row.SourceTurnStart
}

func evidenceTurnsLabel(row store.DirectEvidence) string {
	start := row.SourceTurnStart
	end := row.SourceTurnEnd
	if start <= 0 && end <= 0 {
		return "?"
	}
	if start <= 0 {
		start = end
	}
	if end <= 0 {
		end = start
	}
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

func compactSpaces(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
