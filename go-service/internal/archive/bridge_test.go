package archive

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type bridgeStore struct {
	logs []store.ChatLog
}

func (b bridgeStore) ListChatLogs(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	if sid == "err" {
		return nil, errors.New("boom")
	}
	out := []store.ChatLog{}
	for _, item := range b.logs {
		if item.ChatSessionID == sid && item.TurnIndex >= fromTurn && item.TurnIndex <= toTurn {
			out = append(out, item)
		}
	}
	return out, nil
}

func TestCosineMirrorsArchiveBridgeInvalidAndValidInputs(t *testing.T) {
	if got := Cosine([]float64{1, 0}, []float64{0, 1}); got != 0 {
		t.Fatalf("orthogonal cosine = %v, want 0", got)
	}
	if got := Cosine([]float64{1, 1}, []float64{1, 1}); got < 0.999 || got > 1.001 {
		t.Fatalf("matching cosine = %v, want ~1", got)
	}
	if got := Cosine([]float64{1}, []float64{1, 2}); got != 0 {
		t.Fatalf("mismatched cosine = %v, want 0", got)
	}
}

func TestBridgeGetVerbatimByTurnUsesSessionScopedChatLogs(t *testing.T) {
	br := &Bridge{store: bridgeStore{logs: []store.ChatLog{
		{ID: 1, ChatSessionID: "s1", TurnIndex: 7, Role: "user", Content: "Where is the key?"},
		{ID: 2, ChatSessionID: "s1", TurnIndex: 7, Role: "assistant", Content: "Under the blue tile."},
		{ID: 3, ChatSessionID: "s2", TurnIndex: 7, Role: "assistant", Content: "wrong session"},
		{ID: 4, ChatSessionID: "s1", TurnIndex: 8, Role: "assistant", Content: "wrong turn"},
	}}}

	got, err := br.GetVerbatimByTurn(context.Background(), 7, "s1")
	if err != nil {
		t.Fatalf("GetVerbatimByTurn: %v", err)
	}
	if got == nil || got.TurnIndex != 7 || len(got.Messages) != 2 {
		t.Fatalf("verbatim = %#v, want two scoped messages", got)
	}
	if got.Messages[0].Role != "user" || got.Messages[1].Content != "Under the blue tile." {
		t.Fatalf("messages = %#v", got.Messages)
	}
}

func TestBuildScopedVerbatimSupportMatchesVR18Surface(t *testing.T) {
	now := time.Now().UTC()
	evidence := []store.DirectEvidence{
		{ID: 1, EvidenceText: "Turn 10 confirms smoke over the archive yard.", EvidenceKind: "fact_event", SourceTurnStart: 10, SourceTurnEnd: 10, TurnAnchor: 10, CreatedAt: now},
		{ID: 2, EvidenceText: "Turn 12 confirms east bell roof breach. " + longText("E", 220), EvidenceKind: "fact_event", SourceTurnStart: 12, SourceTurnEnd: 12, TurnAnchor: 12, CreatedAt: now},
		{ID: 3, EvidenceText: "Turn 11 confirms sparks crossing the tower rail.", EvidenceKind: "fact_event", SourceTurnStart: 11, SourceTurnEnd: 11, TurnAnchor: 11, CreatedAt: now},
		{ID: 4, EvidenceText: "ignored tombstone", Tombstoned: true, SourceTurnStart: 13, SourceTurnEnd: 13, TurnAnchor: 13, CreatedAt: now},
		{ID: 5, EvidenceText: "Turn 9 confirms lower ladder intact.", EvidenceKind: "fact_event", SourceTurnStart: 9, SourceTurnEnd: 9, TurnAnchor: 9, CreatedAt: now},
	}

	support := BuildScopedVerbatimSupport(evidence)
	if !support.Active {
		t.Fatal("support inactive")
	}
	if support.PolicyVersion != "vr18a.v1" {
		t.Fatalf("policy = %q", support.PolicyVersion)
	}
	if support.Count != 3 || len(support.Items) != 3 {
		t.Fatalf("count/items = %d/%d, want 3/3", support.Count, len(support.Items))
	}
	if support.Items[0].AnchorTurn != 12 {
		t.Fatalf("first anchor = %v, want 12", support.Items[0].AnchorTurn)
	}
	if len([]rune(support.Items[0].Excerpt)) > 160 {
		t.Fatalf("excerpt too long: %d", len([]rune(support.Items[0].Excerpt)))
	}
	if len([]rune(support.Text)) > 720 {
		t.Fatalf("text too long: %d", len([]rune(support.Text)))
	}
	if support.SurfacePriority[0] != "latest_direct_evidence" || support.SurfacePriority[1] != "recent_raw_turn" {
		t.Fatalf("surface priority = %#v", support.SurfacePriority)
	}
}

func longText(s string, count int) string {
	out := ""
	for i := 0; i < count; i++ {
		out += s
	}
	return out
}
