package httpapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Only the external Store boundary is modeled. Resolution, enrichment,
// canonicalization, evidence planning and secondary reducers are production.
type admissionBoundaryR3Store struct {
	*turnRecordingStore
	store.SourceRevisionStore
	source     *store.MemorySourceRevision
	admissions []*store.MemoryAdmission
	writes     int
	refreshes  int
}

func admissionBoundaryR3NewStore() *admissionBoundaryR3Store {
	return &admissionBoundaryR3Store{
		turnRecordingStore:  &turnRecordingStore{},
		SourceRevisionStore: &memoryAdmissionWorkerStore{Store: store.NewNoopStore()},
		source:              &store.MemorySourceRevision{ChatSessionID: reviewR2SID, SourceRevision: "admission-r3-first", TurnIndex: 1, LifecycleState: "active"},
	}
}

func (s *admissionBoundaryR3Store) MemoryDerivationLifecycleEnabled() bool { return true }
func (s *admissionBoundaryR3Store) MemoryAdmissionWritesEnabled() bool     { return true }
func (s *admissionBoundaryR3Store) GetSourceRevision(_ context.Context, sid, revision string) (*store.MemorySourceRevision, error) {
	if s.source == nil || s.source.ChatSessionID != sid || s.source.SourceRevision != revision {
		return nil, store.ErrNotFound
	}
	copy := *s.source
	return &copy, nil
}

func (s *admissionBoundaryR3Store) CommitMemoryAdmission(ctx context.Context, a *store.MemoryAdmission) (store.MemoryAdmissionResult, error) {
	if a.ContractVersion != store.MemoryAdmissionContract || a.ResultHash != memoryAdmissionResultHashFromCanonicalJSON(a.SourceRevision, a.ResultJSON, a.DerivationVersion, a.ExtractorVersion, a.IndexVersion) {
		return store.MemoryAdmissionResult{}, fmt.Errorf("invalid admission contract/hash")
	}
	if s.source == nil || s.source.LifecycleState != "active" || a.ChatSessionID != s.source.ChatSessionID || a.SourceRevision != s.source.SourceRevision || a.TurnIndex != s.source.TurnIndex {
		return store.MemoryAdmissionResult{}, store.ErrSourceRevisionStale
	}
	s.admissions = append(s.admissions, a)
	if s.source.DerivedAdmissionState == "committed" && s.source.DerivedAdmissionVersion == a.DerivationVersion && s.source.DerivedExtractorVersion == a.ExtractorVersion && s.source.DerivedIndexVersion == a.IndexVersion {
		// All refresh calls in these tests set Refresh=true, Reconcile=false.
		if store.MemoryAdmissionVectorReplayRequested(ctx) {
			if a.ResultJSON != s.source.DerivedResultJSON || a.ResultHash != s.source.DerivedResultHash {
				return store.MemoryAdmissionResult{}, fmt.Errorf("memory admission committed result conflict")
			}
			s.refreshes++
			return store.MemoryAdmissionResult{CommittedResultHash: s.source.DerivedResultHash}, nil
		}
		return store.MemoryAdmissionResult{Idempotent: true, ExistingResultJSON: s.source.DerivedResultJSON, ExistingResultHash: s.source.DerivedResultHash, CommittedResultHash: s.source.DerivedResultHash}, nil
	}
	s.writes++
	if a.Memory != nil {
		a.Memory.ID = int64(s.writes)
		s.returnMemories = append(s.returnMemories, *a.Memory)
	}
	// The builder's session-local planned IDs are provisional. SQL assigns
	// global IDs and rebinds precise receipts by their matching quotation.
	for i, e := range a.Evidence {
		if e == nil || e.ID <= 0 || e.ChatSessionID != a.ChatSessionID || e.SourceTurnStart != a.TurnIndex {
			return store.MemoryAdmissionResult{}, fmt.Errorf("invalid planned evidence")
		}
		e.ID = int64(1000*s.writes + i + 1)
		s.returnEvidence = append(s.returnEvidence, *e)
	}
	for _, u := range a.PreciseUnits {
		ids := preciseMemoryExactEvidenceIDs(s.returnEvidence, a.ChatSessionID, a.TurnIndex, u.EvidenceExcerpt)
		u.RootEvidenceID = 0
		if len(ids) > 0 {
			u.RootEvidenceID = ids[0]
		}
		u.DirectEvidenceIDsJSON = mustCompactJSON(ids)
	}
	s.source.DerivedAdmissionState = "committed"
	s.source.DerivedAdmissionVersion = a.DerivationVersion
	s.source.DerivedExtractorVersion = a.ExtractorVersion
	s.source.DerivedIndexVersion = a.IndexVersion
	s.source.DerivedResultJSON = a.ResultJSON
	s.source.DerivedResultHash = a.ResultHash
	return store.MemoryAdmissionResult{MemoryInserted: a.Memory != nil, EvidenceInserted: len(a.Evidence), PreciseInserted: len(a.PreciseUnits), CommittedResultHash: a.ResultHash}, nil
}

func admissionBoundaryR3Extraction() map[string]any {
	ex := normalizeCriticExtraction(reviewR2Extraction(nil, false, false))
	ex["turn_summary"] = "Courier accepts the delivery instruction."
	return ex
}

func admissionBoundaryR3Metadata(ex map[string]any, field string) string {
	out := map[string]any{}
	for _, raw := range sliceFromAny(ex[field]) {
		item := mapFromAny(raw)
		id := stringFromMap(item, "lifecycle_key")
		if field == "protected_secrets" {
			id = stringFromMap(item, "secret_id")
		}
		if id == "" {
			continue
		}
		metadata := map[string]any{}
		for _, key := range []string{"knowledge_scope", "knowledge_source", "knowledge_boundaries"} {
			if value, supplied := item[key]; supplied {
				metadata[key] = value
			}
		}
		out[id] = metadata
	}
	return mustCompactJSON(out)
}

func TestAdmissionBoundaryR3CommittedRetry(t *testing.T) {
	for _, fixture := range []string{"scope_arrays", "predecessor"} {
		for _, refresh := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/refresh_%t", fixture, refresh), func(t *testing.T) {
				st := admissionBoundaryR3NewStore()
				srv := &Server{Store: st}
				ex := admissionBoundaryR3Extraction()
				if fixture == "scope_arrays" {
					// Typed slices are supplied by the production protected normalizer.
					// Preserve its existing first-write order; decoding changes types.
					mapFromAny(sliceFromAny(ex["protected_secrets"])[0])["knowledge_scope"] = reviewR2Scope([]string{"Zulu", "Alpha"}, []string{"Outsider"})
				}
				before := mustCompactJSON(ex["protected_secrets"])
				goalBefore := admissionBoundaryR3Metadata(ex, "state_claims") + admissionBoundaryR3Metadata(ex, "pending_threads")
				ctx := acceptedPreciseMemoryContext(st.source.SourceRevision)
				res := srv.saveCriticExtractionArtifacts(ctx, reviewR2SID, 1, ex, "Before. "+reviewR2Quote+" After.", completeTurnEmbeddingConfig{}, time.Unix(1, 0))
				if res.Errors != 0 || len(st.admissions) != 1 || st.writes != 1 || len(st.returnEvidence) == 0 || len(st.admissions[0].PreciseUnits) == 0 {
					t.Fatalf("first admission not exercised: %+v admissions=%d writes=%d evidence=%d", res, len(st.admissions), st.writes, len(st.returnEvidence))
				}
				if before != mustCompactJSON(ex["protected_secrets"]) {
					t.Fatal("supplied protected input changed")
				}
				if goalBefore != admissionBoundaryR3Metadata(ex, "state_claims")+admissionBoundaryR3Metadata(ex, "pending_threads") {
					t.Fatal("supplied goal input metadata changed")
				}
				firstJSON, firstHash := st.source.DerivedResultJSON, st.source.DerivedResultHash
				if fixture == "scope_arrays" {
					// Isolate re-normalization from predecessor enrichment.
					st.returnStatusCurrent = nil
					if memoryAdmissionCanonicalResultJSON(parseJSONMap(firstJSON)) == firstJSON {
						t.Fatal("fixture cannot detect decoded-array re-normalization")
					}
				} else {
					if len(st.returnStatusCurrent) == 0 {
						t.Fatal("production secondary reducer did not create predecessor")
					}
					probe := srv.retainPredecessorGoalKnowledgeMetadata(ctx, reviewR2SID, 1, parseJSONMap(firstJSON), "Before. "+reviewR2Quote+" After.", st.returnEvidence, time.Unix(1, 0), &artifactSaveResult{})
					if mustCompactJSON(probe["pending_threads"]) == mustCompactJSON(parseJSONMap(firstJSON)["pending_threads"]) {
						t.Fatal("fixture cannot detect retry predecessor enrichment")
					}
				}
				if refresh {
					ctx = store.WithMemoryAdmissionVectorReplay(ctx, true, false)
				}
				// A different sampled proposal must be replaced with accepted bytes.
				ex["turn_summary"] = "A newly sampled, unaccepted proposal."
				res = srv.saveCriticExtractionArtifacts(ctx, reviewR2SID, 1, ex, "Before. "+reviewR2Quote+" After.", completeTurnEmbeddingConfig{}, time.Unix(2, 0))
				if res.Errors != 0 || len(st.admissions) != 2 {
					t.Fatalf("committed retry failed: errors=%v calls=%d", res.ErrorDetails, len(st.admissions))
				}
				second := st.admissions[1]
				if second.ResultJSON != firstJSON || second.ResultHash != firstHash {
					t.Errorf("committed retry changed submitted result/hash: first=%s second=%s", firstJSON, second.ResultJSON)
				}
				if st.source.DerivedResultJSON != firstJSON || st.source.DerivedResultHash != firstHash || st.writes != 1 || (refresh && st.refreshes != 1) {
					t.Fatal("first accepted result was not preserved")
				}
				// Source/public projection must retain the accepted goal metadata too.
				if second.Memory != nil {
					for _, field := range []string{"state_claims", "pending_threads", "protected_secrets"} {
						if admissionBoundaryR3Metadata(parseJSONMap(second.Memory.SummaryJSON), field) != admissionBoundaryR3Metadata(parseJSONMap(firstJSON), field) {
							t.Errorf("retry source metadata changed: %s", field)
						}
					}
				}
				t.Logf("first_hash=%s submitted_retry_hash=%s refresh=%t evidence_ids=%s", firstHash, second.ResultHash, refresh, mustCompactJSON(st.returnEvidence))
			})
		}
	}
}

func TestAdmissionBoundaryR3DifferentAcceptedRevisionUpdatesScope(t *testing.T) {
	st := admissionBoundaryR3NewStore()
	srv := &Server{Store: st}
	res := srv.saveCriticExtractionArtifacts(acceptedPreciseMemoryContext(st.source.SourceRevision), reviewR2SID, 1, admissionBoundaryR3Extraction(), "Before. "+reviewR2Quote+" After.", completeTurnEmbeddingConfig{}, time.Unix(1, 0))
	if res.Errors != 0 || st.writes != 1 {
		t.Fatalf("first: %+v", res)
	}
	first := st.source.DerivedResultJSON
	st.source = &store.MemorySourceRevision{ChatSessionID: reviewR2SID, SourceRevision: "admission-r3-reroll-edit", TurnIndex: 1, LifecycleState: "active"}
	ex := admissionBoundaryR3Extraction()
	scope := reviewR2Scope([]string{"NewKnower"}, []string{"Courier"})
	for _, field := range []string{"state_claims", "pending_threads"} {
		mapFromAny(sliceFromAny(ex[field])[0])["knowledge_scope"] = scope
	}
	res = srv.saveCriticExtractionArtifacts(acceptedPreciseMemoryContext(st.source.SourceRevision), reviewR2SID, 1, ex, "Before. "+reviewR2Quote+" After.", completeTurnEmbeddingConfig{}, time.Unix(2, 0))
	if res.Errors != 0 || st.writes != 2 || st.source.DerivedResultJSON == first {
		t.Fatalf("new accepted revision not applied: %+v", res)
	}
	for _, field := range []string{"state_claims", "pending_threads"} {
		found := false
		for _, raw := range sliceFromAny(parseJSONMap(st.source.DerivedResultJSON)[field]) {
			item := mapFromAny(raw)
			if stringFromMap(item, "lifecycle_key") == "instruction-fact" {
				found = true
				if mustCompactJSON(item["knowledge_scope"]) != mustCompactJSON(scope) {
					t.Fatalf("current explicit scope overwritten: %s", mustCompactJSON(item))
				}
			}
		}
		if !found {
			t.Fatalf("updated %s absent", field)
		}
	}
}
