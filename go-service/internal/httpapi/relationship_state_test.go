package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test39ARelationshipStateSeparatesDirectionDomainAndExpressionScope(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	result := artifactSaveResult{}
	units := []*store.PreciseMemoryUnit{
		relationshipStateTestUnit("session-a", "revision-a", 1, "a-to-b-trust-private", "entity-a", "entity-b", "A", "B", "trust", "A privately trusts B", "owner_private"),
		relationshipStateTestUnit("session-a", "revision-a", 1, "b-to-a-trust-private", "entity-b", "entity-a", "B", "A", "trust", "B privately distrusts A", "owner_private"),
		relationshipStateTestUnit("session-a", "revision-a", 1, "a-to-b-fear-private", "entity-a", "entity-b", "A", "B", "fear", "A fears B", "owner_private"),
		relationshipStateTestUnit("session-a", "revision-a", 1, "a-to-b-trust-public", "entity-a", "entity-b", "A", "B", "trust", "A publicly claims to trust B", "public"),
	}
	srv.saveRelationshipStatesFromPreciseMemoryUnits(context.Background(), "session-a", units, time.Unix(100, 0), &result)

	if result.Errors != 0 || result.RelationCurrentStates != 4 || result.RelationStateEvents != 4 {
		t.Fatalf("relationship state result=%+v", result)
	}
	if len(fake.returnStatusCurrent) != 4 || len(fake.savedStatusEvents) != 4 || len(fake.savedStatusDefinitions) != 1 {
		t.Fatalf("current=%d events=%d definitions=%d", len(fake.returnStatusCurrent), len(fake.savedStatusEvents), len(fake.savedStatusDefinitions))
	}
	ownerIDs := map[string]bool{}
	for _, current := range fake.returnStatusCurrent {
		ownerIDs[current.OwnerID] = true
		projection := relationshipStateJSONMap(t, current.ValueJSON)
		if relationshipStateHasKey(projection, "score") {
			t.Fatalf("relationship projection flattened to a score: %s", current.ValueJSON)
		}
		if extractionStringFromAny(mapFromAny(projection["reciprocity"])["state"]) != "not_inferred" ||
			extractionStringFromAny(mapFromAny(projection["consent"])["state"]) != "not_inferred" {
			t.Fatalf("reciprocity or consent was inferred: %s", current.ValueJSON)
		}
	}
	if len(ownerIDs) != 4 {
		t.Fatalf("direction/domain/expression scopes collapsed: %#v", fake.returnStatusCurrent)
	}
	options := relationshipStateJSONMap(t, fake.savedStatusDefinitions[0].OptionsJSON)
	if boolFromAny(options["reciprocity_inference"]) || boolFromAny(options["single_relationship_score"]) || boolFromAny(options["user_or_player_priority"]) {
		t.Fatalf("schema enables forbidden relationship shortcuts: %s", fake.savedStatusDefinitions[0].OptionsJSON)
	}
}

func Test39ARelationshipStateKeepsImmutableHistoryAndExactSourceRefs(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	first := relationshipStateTestUnit("session-history", "revision-1", 1, "unit-1", "entity-a", "entity-b", "A", "B", "trust", "A distrusts B", "owner_private")
	second := relationshipStateTestUnit("session-history", "revision-2", 2, "unit-2", "entity-a", "entity-b", "A", "B", "trust", "A now trusts B", "owner_private")
	firstResult := artifactSaveResult{}
	srv.saveRelationshipStatesFromPreciseMemoryUnits(context.Background(), "session-history", []*store.PreciseMemoryUnit{first}, time.Unix(100, 0), &firstResult)
	secondResult := artifactSaveResult{}
	srv.saveRelationshipStatesFromPreciseMemoryUnits(context.Background(), "session-history", []*store.PreciseMemoryUnit{second}, time.Unix(200, 0), &secondResult)

	if firstResult.RelationCurrentStates != 1 || secondResult.RelationCurrentStates != 1 ||
		len(fake.returnStatusCurrent) != 1 || len(fake.savedStatusEvents) != 2 {
		t.Fatalf("first=%+v second=%+v current=%#v events=%#v", firstResult, secondResult, fake.returnStatusCurrent, fake.savedStatusEvents)
	}
	current := relationshipStateJSONMap(t, fake.returnStatusCurrent[0].ValueJSON)
	if extractionStringFromAny(mapFromAny(current["current"])["observation"]) != "A now trusts B" {
		t.Fatalf("latest accepted observation is not current: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
	if fake.savedStatusEvents[1].EventKind != "observed_change" || !strings.Contains(fake.savedStatusEvents[1].PreviousValueJSON, "A distrusts B") {
		t.Fatalf("prior current was not retained in immutable history: %+v", fake.savedStatusEvents[1])
	}
	evidence := relationshipStateJSONMap(t, fake.savedStatusEvents[1].EvidenceJSON)
	if extractionStringFromAny(evidence["source_revision"]) != "revision-2" ||
		extractionStringFromAny(mapFromAny(evidence["previous_current_ref"])["source_revision"]) != "revision-1" ||
		len(sliceFromAny(evidence["direct_evidence_ids"])) != 1 {
		t.Fatalf("history lost exact source lineage: %s", fake.savedStatusEvents[1].EvidenceJSON)
	}
	if relationshipStateHasKey(current, "score") {
		t.Fatalf("current relationship contains a score: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
}

func Test39ARelationshipStateConflictsStayHistoryOnlyAndReplayIsIdempotent(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	left := relationshipStateTestUnit("session-conflict", "revision-conflict", 3, "unit-left", "entity-a", "entity-b", "A", "B", "attachment", "A wants B nearby", "owner_private")
	right := relationshipStateTestUnit("session-conflict", "revision-conflict", 3, "unit-right", "entity-a", "entity-b", "A", "B", "attachment", "A wants distance from B", "owner_private")
	boundary := relationshipStateTestUnit("session-conflict", "revision-conflict", 3, "unit-boundary", "entity-a", "entity-b", "A", "B", "attachment", "A refuses contact", "owner_private")
	boundary.Kind = "boundary"
	boundary.Subtype = "refuse"
	boundary.PayloadJSON = mustCompactJSON(map[string]any{"contract_version": interactionBoundaryContract})
	result := artifactSaveResult{}
	srv.saveRelationshipStatesFromPreciseMemoryUnits(context.Background(), "session-conflict", []*store.PreciseMemoryUnit{left, right, boundary}, time.Unix(300, 0), &result)

	if result.RelationCurrentStates != 0 || result.RelationStateEvents != 2 || len(fake.returnStatusCurrent) != 0 || len(fake.savedStatusEvents) != 2 {
		t.Fatalf("same-source conflict became current or boundary became relationship state: result=%+v current=%#v events=%#v", result, fake.returnStatusCurrent, fake.savedStatusEvents)
	}
	for _, event := range fake.savedStatusEvents {
		evidence := relationshipStateJSONMap(t, event.EvidenceJSON)
		if event.EventState != "history_only" || boolFromAny(evidence["current_projection"]) ||
			extractionStringFromAny(evidence["resolution_status"]) != "same_source_conflict_history_only" {
			t.Fatalf("conflict event was not preserved history-only: %+v", event)
		}
	}
	replay := artifactSaveResult{}
	srv.saveRelationshipStatesFromPreciseMemoryUnits(context.Background(), "session-conflict", []*store.PreciseMemoryUnit{left, right}, time.Unix(301, 0), &replay)
	if replay.RelationCurrentStates != 0 || replay.RelationStateEvents != 0 || len(fake.savedStatusEvents) != 2 {
		t.Fatalf("replay duplicated relationship history: replay=%+v events=%#v", replay, fake.savedStatusEvents)
	}
}

func Test39ARelationshipStateIsSessionScopedWithoutUserPriority(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	for _, test := range []struct {
		sid, revision, unitID, sourceID, targetID, sourceLabel, targetLabel string
	}{
		{"session-one", "revision-one", "user-to-npc", "user", "npc", "User", "NPC"},
		{"session-one", "revision-one", "npc-to-user", "npc", "user", "NPC", "User"},
		{"session-two", "revision-two", "user-to-npc", "user", "npc", "User", "NPC"},
	} {
		result := artifactSaveResult{}
		unit := relationshipStateTestUnit(test.sid, test.revision, 1, test.unitID, test.sourceID, test.targetID, test.sourceLabel, test.targetLabel, "respect", test.sourceLabel+" respects "+test.targetLabel, "owner_private")
		srv.saveRelationshipStatesFromPreciseMemoryUnits(context.Background(), test.sid, []*store.PreciseMemoryUnit{unit}, time.Unix(400, 0), &result)
		if result.RelationCurrentStates != 1 {
			t.Fatalf("session/direction write failed for %+v: %+v", test, result)
		}
	}
	if len(fake.returnStatusCurrent) != 3 {
		t.Fatalf("session or user direction collapsed: %#v", fake.returnStatusCurrent)
	}
	if fake.returnStatusCurrent[0].OwnerID == fake.returnStatusCurrent[1].OwnerID {
		t.Fatalf("user and NPC directions shared one owner: %#v", fake.returnStatusCurrent)
	}
	if fake.returnStatusCurrent[0].OwnerID != fake.returnStatusCurrent[2].OwnerID ||
		fake.returnStatusCurrent[0].ChatSessionID == fake.returnStatusCurrent[2].ChatSessionID {
		t.Fatalf("same directional key should remain isolated by session: %#v", fake.returnStatusCurrent)
	}
}

func Test39ARelationshipStateRunsAfterCommonMemoryAdmission(t *testing.T) {
	fake := newRelationshipMemoryAdmissionStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	excerpt := `Mira told Rook, "I trust Rook."`
	content := "The lamps burned low. " + excerpt
	extraction := map[string]any{
		"entities": map[string]any{"characters": []any{
			map[string]any{"name": "Mira"},
			map[string]any{"name": "Rook"},
		}},
		"relationship_observations": []any{map[string]any{
			"contract_version": relationshipObservationContract,
			"source_entity":    "Mira", "target_entity": "Rook", "domain": "trust",
			"observation": "I trust Rook", "support_kind": "explicit_statement",
			"admission_state": "committed", "review_state": "source_observed",
			"visibility": "owner_private", "evidence_excerpt": excerpt,
		}},
	}
	result := srv.saveCriticExtractionArtifacts(
		acceptedPreciseMemoryContext("revision-admission"), "session-admission", 5,
		extraction, content, completeTurnEmbeddingConfig{}, time.Unix(500, 0),
	)
	if result.Errors != 0 || result.PreciseMemoryUnits != 1 ||
		result.RelationCurrentStates != 1 || result.RelationStateEvents != 1 {
		t.Fatalf("common admission did not materialize relationship state: %+v", result)
	}
	if len(fake.admissions) != 1 || len(fake.returnStatusCurrent) != 1 || len(fake.savedStatusEvents) != 1 {
		t.Fatalf("admissions=%d current=%#v events=%#v", len(fake.admissions), fake.returnStatusCurrent, fake.savedStatusEvents)
	}
	projection := relationshipStateJSONMap(t, fake.returnStatusCurrent[0].ValueJSON)
	if extractionStringFromAny(projection["source_label"]) != "Mira" || extractionStringFromAny(projection["target_label"]) != "Rook" {
		t.Fatalf("materialized direction is wrong: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
}

type relationshipMemoryAdmissionStore struct {
	*preciseMemoryRecordingStore
	admissions []*store.MemoryAdmission
}

func newRelationshipMemoryAdmissionStore() *relationshipMemoryAdmissionStore {
	return &relationshipMemoryAdmissionStore{preciseMemoryRecordingStore: newPreciseMemoryRecordingStore()}
}

func (f *relationshipMemoryAdmissionStore) MemoryDerivationLifecycleEnabled() bool { return true }
func (f *relationshipMemoryAdmissionStore) MemoryAdmissionWritesEnabled() bool     { return true }

func (f *relationshipMemoryAdmissionStore) CommitMemoryAdmission(_ context.Context, admission *store.MemoryAdmission) (store.MemoryAdmissionResult, error) {
	f.admissions = append(f.admissions, admission)
	for _, unit := range admission.PreciseUnits {
		if unit != nil {
			f.unitsByKey[unit.IdempotencyKey] = unit
		}
	}
	return store.MemoryAdmissionResult{PreciseInserted: len(admission.PreciseUnits)}, nil
}

func (f *relationshipMemoryAdmissionStore) RegisterAcceptedSourceRevision(context.Context, *store.MemorySourceRevision) (store.SourceRevisionRegistration, error) {
	return store.SourceRevisionRegistration{}, nil
}

func (f *relationshipMemoryAdmissionStore) GetSourceRevision(context.Context, string, string) (*store.MemorySourceRevision, error) {
	return nil, store.ErrNotFound
}

func (f *relationshipMemoryAdmissionStore) IsSourceRevisionActive(context.Context, string, string) (bool, error) {
	return true, nil
}

func (f *relationshipMemoryAdmissionStore) InvalidateSourceRevisions(context.Context, string, int, string, string, time.Time) error {
	return nil
}

func relationshipStateTestUnit(
	sid, revision string,
	turn int,
	unitID, sourceID, targetID, sourceLabel, targetLabel, domain, observation, visibility string,
) *store.PreciseMemoryUnit {
	payload := map[string]any{
		"contract_version": relationshipObservationContract,
		"source_entity":    sourceLabel,
		"target_entity":    targetLabel,
		"domain":           domain,
		"observation":      observation,
		"support_kind":     "explicit_observed_state",
		"admission_state":  "committed",
		"review_state":     "source_observed",
		"visibility":       visibility,
	}
	return &store.PreciseMemoryUnit{
		UnitID:                unitID,
		ContractVersion:       store.PreciseMemoryUnitContract,
		ChatSessionID:         sid,
		SourceTurnStart:       turn,
		SourceTurnEnd:         turn,
		SourceContract:        completeTurnSourceAcceptanceContract,
		SourceRevision:        revision,
		SourceLogicalTurnID:   "logical-" + revision,
		SourceMessageID:       "message-" + revision,
		SourceGenerationID:    "generation-" + revision,
		SourceContentHash:     "content-hash-" + revision,
		EvidenceExcerpt:       observation,
		EvidenceHash:          "evidence-hash-" + unitID,
		RootEvidenceID:        int64(turn),
		DirectEvidenceIDsJSON: mustCompactJSON([]int64{int64(turn)}),
		Kind:                  "observation",
		Subtype:               "relationship_" + domain,
		PayloadJSON:           mustCompactJSON(payload),
		ActorEntityID:         sourceID,
		AffectedEntityID:      targetID,
		RelationshipKey:       comparableEntityKey(sourceLabel) + "->" + comparableEntityKey(targetLabel) + "/" + domain,
		TruthScope:            "source_scoped",
		EpistemicMode:         "direct",
		AuthorityClass:        "subjective_episodic",
		AdmissionState:        "committed",
		ReviewState:           "source_observed",
		Visibility:            visibility,
		Confidence:            1,
		IdempotencyKey:        "idempotency-" + unitID,
		DerivationVersion:     store.PreciseMemoryUnitContract,
		ExtractorVersion:      "test",
		IndexVersion:          "test",
		LifecycleState:        "active",
		CreatedAt:             time.Unix(int64(turn), 0),
		UpdatedAt:             time.Unix(int64(turn), 0),
	}
}

func relationshipStateJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	value := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return value
}

func relationshipStateHasKey(value any, wanted string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == wanted || relationshipStateHasKey(child, wanted) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if relationshipStateHasKey(child, wanted) {
				return true
			}
		}
	}
	return false
}
