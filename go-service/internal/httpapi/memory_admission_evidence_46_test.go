package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// This boundary allocates evidence IDs globally like MariaDB, while each
// session's candidate builder can only see that session's existing records.
type committedEvidence46Store struct {
	*memoryAdmissionWorkerStore
	persisted []store.DirectEvidence
	replay    bool
	unitIDs   []string
}

func (s *committedEvidence46Store) ListEvidence(_ context.Context, sid string) ([]store.DirectEvidence, error) {
	var out []store.DirectEvidence
	for _, evidence := range s.persisted {
		if evidence.ChatSessionID == sid {
			out = append(out, evidence)
		}
	}
	return out, nil
}

func (s *committedEvidence46Store) CommitMemoryAdmission(ctx context.Context, admission *store.MemoryAdmission) (store.MemoryAdmissionResult, error) {
	s.unitIDs = nil
	for _, unit := range admission.PreciseUnits {
		s.unitIDs = append(s.unitIDs, unit.UnitID)
	}
	if s.replay {
		return store.MemoryAdmissionResult{Idempotent: true, CommittedResultHash: admission.ResultHash}, nil
	}
	result, err := s.memoryAdmissionWorkerStore.CommitMemoryAdmission(ctx, admission)
	for _, evidence := range admission.Evidence {
		s.persisted = append(s.persisted, *evidence)
		for _, unit := range admission.PreciseUnits {
			if unit.EvidenceExcerpt == evidence.EvidenceText {
				unit.RootEvidenceID = evidence.ID
				unit.DirectEvidenceIDsJSON = mustCompactJSON([]int64{evidence.ID})
			}
		}
	}
	return result, err
}

func commitEvidence46(t *testing.T, srv *Server, sid, revision string) ([]store.DirectEvidence, []*store.PreciseMemoryUnit) {
	t.Helper()
	extraction := map[string]any{
		"turn_summary": "A bell rang.", "evidence_excerpts": []any{"A bell rang."},
		"narrative_events": []any{map[string]any{"summary": "A bell rang.", "evidence_excerpt": "A bell rang.", "confidence": .9}},
	}
	result := artifactSaveResult{}
	handled, evidence, units := srv.commitAcceptedMemoryAdmission(
		acceptedStoryClockContext(revision, revision+"-turn", "generation"), sid, 1,
		extraction, "A bell rang. The gate opened and the guards continued their work.", "A bell rang.", "A bell rang.", memorySearchTextBuild{},
		completeTurnEmbeddingConfig{}, "[]", "", nil, nil, nil, nil, time.Unix(1, 0), &result,
	)
	if !handled || result.Errors != 0 || len(evidence) != 1 || len(units) != 1 {
		t.Fatalf("production admission failed: handled=%v evidence=%#v units=%#v result=%+v", handled, evidence, units, result)
	}
	return evidence, units
}

func Test46AdmissionReturnsCommittedGlobalEvidenceIDsAcrossSessions(t *testing.T) {
	st := &committedEvidence46Store{memoryAdmissionWorkerStore: &memoryAdmissionWorkerStore{Store: store.NewNoopStore()}}
	srv := &Server{Store: st}
	first, _ := commitEvidence46(t, srv, "first-session", "source-first")
	second, units := commitEvidence46(t, srv, "second-session", "source-second")
	persisted, err := st.ListEvidence(context.Background(), "second-session")
	if err != nil || second[0].ID != persisted[0].ID || second[0].ID == first[0].ID {
		t.Fatalf("second session returned a provisional/cross-session ID: first=%#v second=%#v actual=%#v", first, second, persisted)
	}
	if units[0].RootEvidenceID != second[0].ID || jsonInt64Slice(units[0].DirectEvidenceIDsJSON)[0] != second[0].ID {
		t.Fatal("normal committed units and downstream evidence disagree")
	}
}

func Test46ConcurrentAdmissionReplayReturnsPersistedEvidenceAndPreciseReferences(t *testing.T) {
	st := &committedEvidence46Store{memoryAdmissionWorkerStore: &memoryAdmissionWorkerStore{Store: store.NewNoopStore(), nextEvidenceID: 90}}
	srv := &Server{Store: st}
	_, initialUnits := commitEvidence46(t, srv, "session", "same-source")
	st.replay = true
	// The losing concurrent pass began with no evidence snapshot. SQL reports
	// idempotent without mutating its candidate pointers, as the real owner does.
	evidence, units := commitEvidence46(t, srv, "session", "same-source")
	if evidence[0].ID != st.persisted[0].ID || units[0].RootEvidenceID != st.persisted[0].ID || jsonInt64Slice(units[0].DirectEvidenceIDsJSON)[0] != st.persisted[0].ID {
		t.Fatalf("replay returned provisional IDs: evidence=%#v unit=%+v persisted=%#v", evidence, units[0], st.persisted)
	}
	if units[0].UnitID != initialUnits[0].UnitID || units[0].UnitID != st.unitIDs[0] {
		t.Fatal("evidence binding changed stable precise-unit identity")
	}
}
