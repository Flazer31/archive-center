package httpapi

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// registerTimelineRoutes mounts read-only timeline endpoints.
func (s *Server) registerTimelineRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /timeline", s.handleTimeline)
	mux.HandleFunc("GET /timeline-item", s.handleTimelineItem)
}

// timelineItem is a unified read-only entry for the timeline view.
type timelineItem struct {
	ID            int64  `json:"id"`
	Type          string `json:"type"`
	SourceID      string `json:"source_id"`
	ChatSessionID string `json:"chat_session_id"`
	TurnIndex     int    `json:"turn_index"`
	FromTurn      int    `json:"from_turn,omitempty"`
	ToTurn        int    `json:"to_turn,omitempty"`
	Role          string `json:"role,omitempty"`
	Title         string `json:"title"`
	Preview       string `json:"preview"`
	CreatedAt     any    `json:"created_at"`
	DetailRef     string `json:"detail_ref"`
}

func timelineMatchesSession(itemSessionID, requestedSessionID string) bool {
	return requestedSessionID != "" && itemSessionID == requestedSessionID
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("sessionId")
	beforeTurn, _ := strconv.Atoi(r.URL.Query().Get("beforeTurn"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	items := []timelineItem{}
	sourceCounts := map[string]int{}

	if s.Store != nil {
		ctx := r.Context()

		// Chat logs
		logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			for _, l := range logs {
				if !timelineMatchesSession(l.ChatSessionID, sid) {
					continue
				}
				items = append(items, timelineItem{
					ID:            l.ID,
					Type:          "chat_log",
					SourceID:      strconv.FormatInt(l.ID, 10),
					ChatSessionID: l.ChatSessionID,
					TurnIndex:     l.TurnIndex,
					Role:          l.Role,
					Title:         l.Role,
					Preview:       pythonTextPreview(l.Content, 120),
					CreatedAt:     formatKSTTime(l.CreatedAt),
					DetailRef:     "/timeline-item?type=chat_log&id=" + strconv.FormatInt(l.ID, 10) + "&sessionId=" + l.ChatSessionID,
				})
			}
			sourceCounts["chat_logs"] = len(logs)
		}

		// Memories
		memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			for _, m := range memories {
				if !timelineMatchesSession(m.ChatSessionID, sid) {
					continue
				}
				items = append(items, timelineItem{
					ID:            m.ID,
					Type:          "memory",
					SourceID:      strconv.FormatInt(m.ID, 10),
					ChatSessionID: m.ChatSessionID,
					TurnIndex:     m.TurnIndex,
					Title:         "Memory",
					Preview:       pythonTextPreview(m.SummaryJSON, 120),
					CreatedAt:     formatKSTTime(m.CreatedAt),
					DetailRef:     "/timeline-item?type=memory&id=" + strconv.FormatInt(m.ID, 10) + "&sessionId=" + m.ChatSessionID,
				})
			}
			sourceCounts["memories"] = len(memories)
		}

		// Evidence
		evidence, err := s.Store.ListEvidence(ctx, sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			for _, e := range evidence {
				if !timelineMatchesSession(e.ChatSessionID, sid) {
					continue
				}
				items = append(items, timelineItem{
					ID:            e.ID,
					Type:          "evidence",
					SourceID:      strconv.FormatInt(e.ID, 10),
					ChatSessionID: e.ChatSessionID,
					TurnIndex:     e.TurnAnchor,
					Title:         e.EvidenceKind,
					Preview:       pythonTextPreview(e.EvidenceText, 120),
					CreatedAt:     formatKSTTime(e.CreatedAt),
					DetailRef:     "/timeline-item?type=evidence&id=" + strconv.FormatInt(e.ID, 10) + "&sessionId=" + e.ChatSessionID,
				})
			}
			sourceCounts["evidence"] = len(evidence)
		}

		// KG Triples
		triples, err := s.Store.ListKGTriples(ctx, sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			for _, t := range triples {
				if !timelineMatchesSession(t.ChatSessionID, sid) {
					continue
				}
				items = append(items, timelineItem{
					ID:            t.ID,
					Type:          "kg_triple",
					SourceID:      strconv.FormatInt(t.ID, 10),
					ChatSessionID: t.ChatSessionID,
					TurnIndex:     t.SourceTurn,
					Title:         t.Subject + " " + t.Predicate + " " + t.Object,
					Preview:       t.Subject + " " + t.Predicate + " " + t.Object,
					CreatedAt:     formatKSTTime(t.CreatedAt),
					DetailRef:     "/timeline-item?type=kg_triple&id=" + strconv.FormatInt(t.ID, 10) + "&sessionId=" + t.ChatSessionID,
				})
			}
			sourceCounts["kg_triples"] = len(triples)
		}

		// Episode summaries
		episodes, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			for _, ep := range episodes {
				if !timelineMatchesSession(ep.ChatSessionID, sid) {
					continue
				}
				items = append(items, timelineItem{
					ID:            ep.ID,
					Type:          "episode",
					SourceID:      strconv.FormatInt(ep.ID, 10),
					ChatSessionID: ep.ChatSessionID,
					TurnIndex:     ep.ToTurn,
					FromTurn:      ep.FromTurn,
					ToTurn:        ep.ToTurn,
					Title:         "Episode " + strconv.Itoa(ep.FromTurn) + "-" + strconv.Itoa(ep.ToTurn),
					Preview:       pythonTextPreview(ep.SummaryText, 120),
					CreatedAt:     formatKSTTime(ep.CreatedAt),
					DetailRef:     "/timeline-item?type=episode&id=" + strconv.FormatInt(ep.ID, 10) + "&sessionId=" + ep.ChatSessionID,
				})
			}
			sourceCounts["episodes"] = len(episodes)
		}
	}

	// Sort newest first by turn_index, then id desc for stability.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].TurnIndex == items[j].TurnIndex {
			return items[i].ID > items[j].ID
		}
		return items[i].TurnIndex > items[j].TurnIndex
	})

	// Apply beforeTurn filter (exclusive)
	filtered := []timelineItem{}
	for _, it := range items {
		if beforeTurn > 0 && it.TurnIndex >= beforeTurn {
			continue
		}
		filtered = append(filtered, it)
	}

	// Apply limit
	paged := []timelineItem{}
	nextBeforeTurn := 0
	for i, it := range filtered {
		if i >= limit {
			break
		}
		paged = append(paged, it)
	}
	if len(filtered) > limit && len(paged) > 0 {
		nextBeforeTurn = paged[len(paged)-1].TurnIndex
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"items":  paged,
		"meta": map[string]any{
			"session_id":       sid,
			"limit":            limit,
			"before_turn":      beforeTurn,
			"next_before_turn": nextBeforeTurn,
			"source_counts":    sourceCounts,
			"generated_at":     time.Now().UTC().Format(time.RFC3339Nano),
			"read_only":        true,
			"total_unpaged":    len(items),
		},
	})
}

func (s *Server) handleTimelineItem(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("sessionId")
	itemType := r.URL.Query().Get("type")
	idStr := r.URL.Query().Get("id")
	if itemType == "" || idStr == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "type and id are required")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "id must be numeric")
		return
	}

	if s.Store == nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "store not available")
		return
	}

	ctx := r.Context()
	var detail map[string]any

	switch itemType {
	case "chat_log":
		logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		for _, l := range logs {
			if l.ID == id {
				if !timelineMatchesSession(l.ChatSessionID, sid) {
					continue
				}
				detail = map[string]any{
					"id":              l.ID,
					"type":            "chat_log",
					"chat_session_id": l.ChatSessionID,
					"turn_index":      l.TurnIndex,
					"role":            l.Role,
					"content":         l.Content,
					"created_at":      formatKSTTime(l.CreatedAt),
				}
				break
			}
		}
	case "memory":
		memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		for _, m := range memories {
			if m.ID == id {
				if !timelineMatchesSession(m.ChatSessionID, sid) {
					continue
				}
				detail = map[string]any{
					"id":              m.ID,
					"type":            "memory",
					"chat_session_id": m.ChatSessionID,
					"turn_index":      m.TurnIndex,
					"summary_json":    m.SummaryJSON,
					"importance":      m.Importance,
					"created_at":      formatKSTTime(m.CreatedAt),
				}
				break
			}
		}
	case "evidence":
		evidence, err := s.Store.ListEvidence(ctx, sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		for _, e := range evidence {
			if e.ID == id {
				if !timelineMatchesSession(e.ChatSessionID, sid) {
					continue
				}
				detail = map[string]any{
					"id":                e.ID,
					"type":              "evidence",
					"chat_session_id":   e.ChatSessionID,
					"evidence_kind":     e.EvidenceKind,
					"evidence_text":     e.EvidenceText,
					"source_turn_start": e.SourceTurnStart,
					"source_turn_end":   e.SourceTurnEnd,
					"turn_anchor":       e.TurnAnchor,
					"archive_state":     e.ArchiveState,
					"capture_stage":     e.CaptureStage,
					"created_at":        formatKSTTime(e.CreatedAt),
				}
				break
			}
		}
	case "kg_triple":
		triples, err := s.Store.ListKGTriples(ctx, sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		for _, t := range triples {
			if t.ID == id {
				if !timelineMatchesSession(t.ChatSessionID, sid) {
					continue
				}
				detail = map[string]any{
					"id":              t.ID,
					"type":            "kg_triple",
					"chat_session_id": t.ChatSessionID,
					"subject":         t.Subject,
					"predicate":       t.Predicate,
					"object":          t.Object,
					"valid_from":      t.ValidFrom,
					"valid_to":        t.ValidTo,
					"source_turn":     t.SourceTurn,
					"created_at":      formatKSTTime(t.CreatedAt),
				}
				break
			}
		}
	case "episode":
		episodes, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		for _, ep := range episodes {
			if ep.ID == id {
				if !timelineMatchesSession(ep.ChatSessionID, sid) {
					continue
				}
				detail = map[string]any{
					"id":                        ep.ID,
					"type":                      "episode",
					"chat_session_id":           ep.ChatSessionID,
					"from_turn":                 ep.FromTurn,
					"to_turn":                   ep.ToTurn,
					"summary_text":              ep.SummaryText,
					"key_entities":              ep.KeyEntities,
					"key_events":                ep.KeyEvents,
					"open_loops_json":           ep.OpenLoopsJSON,
					"relationship_changes_json": ep.RelationshipChangesJSON,
					"created_at":                formatKSTTime(ep.CreatedAt),
				}
				break
			}
		}
	default:
		writeError(w, http.StatusBadRequest, CodeBadRequest, "unsupported type")
		return
	}

	if detail == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"status": "not_found",
			"code":   "not_found",
			"type":   itemType,
			"id":     id,
		})
		return
	}
	detail = presentationTimelineItem(detail)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"item":      detail,
		"read_only": true,
	})
}
