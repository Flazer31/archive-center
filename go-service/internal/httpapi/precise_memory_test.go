package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type preciseMemoryRecordingStore struct {
	*identityRecordingStore
	unitsByKey map[string]*store.PreciseMemoryUnit
	saveOrder  []string
}

func newPreciseMemoryRecordingStore() *preciseMemoryRecordingStore {
	return &preciseMemoryRecordingStore{
		identityRecordingStore: newIdentityRecordingStore(),
		unitsByKey:             map[string]*store.PreciseMemoryUnit{},
	}
}

func (f *preciseMemoryRecordingStore) PreciseMemoryWritesEnabled() bool {
	return true
}

func (f *preciseMemoryRecordingStore) SavePreciseMemoryUnit(_ context.Context, item *store.PreciseMemoryUnit) (bool, error) {
	if _, exists := f.unitsByKey[item.IdempotencyKey]; exists {
		return false, nil
	}
	cp := *item
	cp.ID = int64(len(f.unitsByKey) + 1)
	item.ID = cp.ID
	f.unitsByKey[item.IdempotencyKey] = &cp
	f.saveOrder = append(f.saveOrder, item.IdempotencyKey)
	return true, nil
}

func acceptedPreciseMemoryContext(revision string) context.Context {
	return contextWithEntityIdentitySource(context.Background(), completeTurnSourceAcceptanceDecision{
		Enabled: true, Accepted: true, Revision: revision, LogicalTurnID: "logical-turn",
		Observation: completeTurnSourceObservation{
			HostChatID: "host-chat", MessageIndex: 8, GenerationID: "generation",
		},
	})
}

func preciseMemoryUnitByKind(t *testing.T, fake *preciseMemoryRecordingStore, kind string) *store.PreciseMemoryUnit {
	t.Helper()
	for _, item := range fake.unitsByKey {
		if item.Kind == kind {
			return item
		}
	}
	t.Fatalf("missing precise memory kind %q: %#v", kind, fake.unitsByKey)
	return nil
}

func TestPreciseMemoryAdmissionStoresIndependentAtomicUnits(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	content := "Mira opened the gate. The gate stood open. \"Keep moving,\" Mira said. Rook believed the hall was unsafe."
	extraction := map[string]any{
		"entities": map[string]any{"characters": []any{
			map[string]any{"name": "Mira"},
			map[string]any{"name": "Rook"},
		}},
		"narrative_events": []any{map[string]any{
			"summary": "Mira opened the gate.", "event_type": "action",
			"actor": "Mira", "participants": []any{"Mira"},
			"evidence_excerpt": "Mira opened the gate.", "confidence": 0.96,
		}},
		"state_claims": []any{map[string]any{
			"subject": "gate", "subject_type": "world", "state_slot": "position",
			"value": "open", "claim_scope": "objective",
			"evidence_excerpt": "The gate stood open.", "confidence": 0.95,
		}},
		"speaker_attributions": []any{map[string]any{
			"speaker_name": "Mira", "attribution_kind": "dialogue",
			"attribution_state": "linked", "confidence": 0.98,
			"evidence_excerpt": "\"Keep moving,\" Mira said.",
		}},
		"belief_updates": []any{map[string]any{
			"subject": "hall", "subject_type": "world", "state_slot": "safety",
			"value": "unsafe", "perspective_owner": "Rook", "confidence": 0.82,
			"epistemic_state":  "known",
			"evidence_excerpt": "Rook believed the hall was unsafe.",
		}},
	}
	result := srv.saveCriticExtractionArtifacts(
		acceptedPreciseMemoryContext("revision-atomic"), "session-atomic", 4,
		extraction, content, completeTurnEmbeddingConfig{}, time.Unix(400, 0),
	)
	if result.PreciseMemoryUnits != 4 || len(fake.unitsByKey) != 4 {
		t.Fatalf("precise units result=%d stored=%d errors=%#v skips=%#v", result.PreciseMemoryUnits, len(fake.unitsByKey), result.ErrorDetails, result.SkipReasons)
	}
	event := preciseMemoryUnitByKind(t, fake, "event")
	state := preciseMemoryUnitByKind(t, fake, "state")
	utterance := preciseMemoryUnitByKind(t, fake, "utterance")
	observation := preciseMemoryUnitByKind(t, fake, "observation")
	if event.ActorEntityID == "" || utterance.ActorEntityID == "" {
		t.Fatalf("unique source-observed actor/speaker pointers were not stored: event=%+v utterance=%+v", event, utterance)
	}
	if !strings.Contains(event.PayloadJSON, "participant_entity_ids") {
		t.Fatalf("resolved participant IDs missing from deterministic payload: %s", event.PayloadJSON)
	}
	if observation.KnowledgeHolderEntityID == "" {
		t.Fatalf("unique observation knower pointer was not stored: %+v", observation)
	}
	if state.SubjectEntityID != "" || state.AuthorityClass != "objective_world_state" || state.TruthScope != "objective" {
		t.Fatalf("world fact should remain objective without an entity ID: %+v", state)
	}
	if observation.Kind != "observation" || observation.AuthorityClass != "subjective_episodic" ||
		observation.TruthScope != "owner_scoped" || observation.Visibility != "owner_private" {
		t.Fatalf("belief update gained objective authority: %+v", observation)
	}
	for _, item := range fake.unitsByKey {
		if item.ContractVersion != store.PreciseMemoryUnitContract ||
			item.SourceRole != "combined_turn_pair" ||
			item.RootEvidenceID <= 0 ||
			item.SourceSpanEnd <= item.SourceSpanStart {
			t.Fatalf("unit lost exact accepted source provenance: %+v", item)
		}
	}
}

func TestPreciseMemoryAdmissionRejectsMissingOrUnacceptedEvidence(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	extraction := map[string]any{"state_claims": []any{map[string]any{
		"subject": "weather", "subject_type": "world", "state_slot": "condition",
		"value": "rain", "evidence_excerpt": "Rain covered the road.",
	}}}
	content := "Rain covered the road. The travelers waited."
	result := artifactSaveResult{}
	srv.savePreciseMemoryUnitsFromExtraction(
		acceptedPreciseMemoryContext("revision-no-evidence"), "session-no-evidence", 2,
		extraction, content, nil, nil, time.Unix(200, 0), &result,
	)
	if len(fake.unitsByKey) != 0 {
		t.Fatalf("missing direct evidence stored precise units: %#v", fake.unitsByKey)
	}
	result = artifactSaveResult{}
	srv.savePreciseMemoryUnitsFromExtraction(
		acceptedPreciseMemoryContext("revision-wrong-content"), "session-no-evidence", 2,
		extraction, "The road remained dry.",
		[]store.DirectEvidence{preciseMemoryTestEvidence(2, "session-no-evidence", 2, "Rain covered the road.")},
		nil, time.Unix(200, 0), &result,
	)
	if len(fake.unitsByKey) != 0 {
		t.Fatalf("same-text evidence without a current-content span stored units: %#v", fake.unitsByKey)
	}
	result = artifactSaveResult{}
	srv.savePreciseMemoryUnitsFromExtraction(
		acceptedPreciseMemoryContext("revision-stale-evidence"), "session-no-evidence", 2,
		extraction, content,
		[]store.DirectEvidence{preciseMemoryTestEvidence(3, "session-no-evidence", 1, "Rain covered the road.")},
		nil, time.Unix(200, 0), &result,
	)
	if len(fake.unitsByKey) != 0 {
		t.Fatalf("noncurrent evidence stored precise units: %#v", fake.unitsByKey)
	}
	result = artifactSaveResult{}
	srv.savePreciseMemoryUnitsFromExtraction(
		context.Background(), "session-legacy", 2, extraction, content,
		[]store.DirectEvidence{preciseMemoryTestEvidence(1, "session-legacy", 2, "Rain covered the road.")},
		nil, time.Unix(201, 0), &result,
	)
	if len(fake.unitsByKey) != 0 {
		t.Fatalf("legacy/unaccepted source stored precise units: %#v", fake.unitsByKey)
	}
}

func TestPreciseMemoryStructuredNonFactsNeverBecomeObjective(t *testing.T) {
	tests := []struct {
		name          string
		fields        map[string]any
		wantTruth     string
		wantAuthority string
	}{
		{name: "lie", fields: map[string]any{"is_lie": true}, wantTruth: "speaker_claim", wantAuthority: "subjective_episodic"},
		{name: "deception", fields: map[string]any{"truth_status": "deception"}, wantTruth: "speaker_claim", wantAuthority: "subjective_episodic"},
		{name: "speculation", fields: map[string]any{"modality": "speculation"}, wantTruth: "non_objective", wantAuthority: "support_hypothesis"},
		{name: "uncertain", fields: map[string]any{"transition": "uncertain"}, wantTruth: "non_objective", wantAuthority: "support_hypothesis"},
		{name: "ooc meta", fields: map[string]any{"is_ooc": true}, wantTruth: "ooc_meta", wantAuthority: "ooc_meta"},
	}
	for index, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := newPreciseMemoryRecordingStore()
			srv := &Server{Store: fake}
			excerpt := "A structured claim was recorded."
			item := map[string]any{
				"subject": "session", "subject_type": "session", "state_slot": "claim",
				"value": "pending", "confidence": 0.99, "evidence_excerpt": excerpt,
			}
			for key, value := range tc.fields {
				item[key] = value
			}
			result := artifactSaveResult{}
			srv.savePreciseMemoryUnitsFromExtraction(
				acceptedPreciseMemoryContext("revision-authority-"+tc.name), "session-authority", index+1,
				map[string]any{"state_claims": []any{item}}, excerpt+" Additional context.",
				[]store.DirectEvidence{preciseMemoryTestEvidence(int64(index+1), "session-authority", index+1, excerpt)},
				nil, time.Unix(int64(index+1), 0), &result,
			)
			if len(fake.unitsByKey) != 1 {
				t.Fatalf("stored=%d want 1; errors=%#v skips=%#v", len(fake.unitsByKey), result.ErrorDetails, result.SkipReasons)
			}
			unit := preciseMemoryUnitByKind(t, fake, "state")
			if unit.TruthScope != tc.wantTruth || unit.AuthorityClass != tc.wantAuthority ||
				unit.AuthorityClass == "objective_world_state" {
				t.Fatalf("structured non-fact gained objective authority: %+v", unit)
			}
		})
	}
}

func TestPreciseMemoryAmbiguousParticipantsRequireReview(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	content := "Sentinel opened the inner door. Two masked guards withdrew."
	extraction := map[string]any{
		"entities": map[string]any{"characters": []any{
			map[string]any{"name": "East Guard", "aliases": []any{"Sentinel"}},
			map[string]any{"name": "West Guard", "aliases": []any{"Sentinel"}},
		}},
		"narrative_events": []any{map[string]any{
			"summary": "The inner door opened.", "event_type": "action",
			"participants": []any{"Sentinel"}, "confidence": 0.99,
			"evidence_excerpt": "Sentinel opened the inner door.",
		}},
	}
	result := srv.saveCriticExtractionArtifacts(
		acceptedPreciseMemoryContext("revision-participants"), "session-participants", 6,
		extraction, content, completeTurnEmbeddingConfig{}, time.Unix(600, 0),
	)
	if result.PreciseMemoryUnits != 1 {
		t.Fatalf("precise units=%d want 1; errors=%#v skips=%#v", result.PreciseMemoryUnits, result.ErrorDetails, result.SkipReasons)
	}
	unit := preciseMemoryUnitByKind(t, fake, "event")
	if unit.AdmissionState != "review_required" || unit.ReviewState != "needs_review" ||
		unit.AuthorityClass == "objective_world_state" {
		t.Fatalf("ambiguous participant event gained objective authority: %+v", unit)
	}
	if strings.Contains(unit.PayloadJSON, "participant_entity_ids") {
		t.Fatalf("ambiguous participants gained stable IDs: %s", unit.PayloadJSON)
	}
}

func TestPreciseMemoryAdmissionKeepsProposalAndBeliefNonObjective(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	content := "The council proposed opening the vault. A witness believed the vault was empty."
	extraction := map[string]any{
		"state_claims": []any{map[string]any{
			"subject": "vault", "subject_type": "world", "state_slot": "access",
			"value": "open", "modality": "proposal", "confidence": 0.99,
			"evidence_excerpt": "The council proposed opening the vault.",
		}},
		"belief_updates": []any{map[string]any{
			"subject": "vault", "subject_type": "world", "state_slot": "contents",
			"value": "empty", "perspective_owner": "witness", "confidence": 0.99,
			"evidence_excerpt": "A witness believed the vault was empty.",
		}},
	}
	evidence := []store.DirectEvidence{
		preciseMemoryTestEvidence(1, "session-scope", 3, "The council proposed opening the vault."),
		preciseMemoryTestEvidence(2, "session-scope", 3, "A witness believed the vault was empty."),
	}
	result := artifactSaveResult{}
	srv.savePreciseMemoryUnitsFromExtraction(
		acceptedPreciseMemoryContext("revision-scope"), "session-scope", 3,
		extraction, content, evidence, nil, time.Unix(300, 0), &result,
	)
	if len(fake.unitsByKey) != 2 {
		t.Fatalf("stored units=%d want 2: skips=%#v", len(fake.unitsByKey), result.SkipReasons)
	}
	state := preciseMemoryUnitByKind(t, fake, "state")
	observation := preciseMemoryUnitByKind(t, fake, "observation")
	if state.TruthScope != "proposed" || state.AuthorityClass != "support_hypothesis" {
		t.Fatalf("proposal became objective: %+v", state)
	}
	if observation.TruthScope != "owner_scoped" || observation.AuthorityClass != "subjective_episodic" {
		t.Fatalf("belief became objective: %+v", observation)
	}
}

func TestPreciseMemoryAdmissionRequiresReviewForAmbiguousAndAliasOnlyIdentity(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		entities  []any
		subject   string
		revision  string
		sessionID string
	}{
		{
			name: "shared alias", content: "Shade remained alert. Two guards watched.",
			entities: []any{
				map[string]any{"name": "North Guard", "aliases": []any{"Shade"}},
				map[string]any{"name": "South Guard", "aliases": []any{"Shade"}},
			},
			subject: "Shade", revision: "revision-shared", sessionID: "session-shared",
		},
		{
			name: "alias only", content: "Mira remained alert. The watch continued.",
			entities: []any{map[string]any{"name": "Captain Mira", "aliases": []any{"Mira"}}},
			subject:  "Mira", revision: "revision-alias", sessionID: "session-alias",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := newPreciseMemoryRecordingStore()
			srv := NewServer(config.Default())
			srv.Store = fake
			excerpt := tc.subject + " remained alert."
			extraction := map[string]any{
				"entities": map[string]any{"characters": tc.entities},
				"state_claims": []any{map[string]any{
					"subject": tc.subject, "subject_type": "character", "state_slot": "alertness",
					"value": "alert", "confidence": 0.95, "evidence_excerpt": excerpt,
				}},
			}
			result := srv.saveCriticExtractionArtifacts(
				acceptedPreciseMemoryContext(tc.revision), tc.sessionID, 5,
				extraction, tc.content, completeTurnEmbeddingConfig{}, time.Unix(500, 0),
			)
			if result.PreciseMemoryUnits != 1 {
				t.Fatalf("precise units=%d want 1; errors=%#v skips=%#v", result.PreciseMemoryUnits, result.ErrorDetails, result.SkipReasons)
			}
			unit := preciseMemoryUnitByKind(t, fake, "state")
			if unit.SubjectEntityID != "" || unit.AdmissionState != "review_required" ||
				unit.ReviewState != "needs_review" || unit.AuthorityClass == "objective_world_state" {
				t.Fatalf("unresolved identity gained stable/objective authority: %+v", unit)
			}
		})
	}
}

func TestPreciseMemoryIdempotencyIgnoresExtractionOrder(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	content := "The eastern lamp lit. The western lamp dimmed."
	first := map[string]any{"narrative_events": []any{
		map[string]any{"summary": "Eastern lamp lit", "evidence_excerpt": "The eastern lamp lit."},
		map[string]any{"summary": "Western lamp dimmed", "evidence_excerpt": "The western lamp dimmed."},
	}}
	second := map[string]any{"narrative_events": []any{
		map[string]any{"summary": "Western lamp dimmed", "evidence_excerpt": "The western lamp dimmed."},
		map[string]any{"summary": "Eastern lamp lit", "evidence_excerpt": "The eastern lamp lit."},
	}}
	ctx := acceptedPreciseMemoryContext("revision-order")
	firstResult := srv.saveCriticExtractionArtifacts(ctx, "session-order", 7, first, content, completeTurnEmbeddingConfig{}, time.Unix(700, 0))
	secondResult := srv.saveCriticExtractionArtifacts(ctx, "session-order", 7, second, content, completeTurnEmbeddingConfig{}, time.Unix(701, 0))
	if firstResult.PreciseMemoryUnits != 2 || secondResult.PreciseMemoryUnits != 0 || len(fake.unitsByKey) != 2 {
		t.Fatalf("order/replay changed identity: first=%d second=%d stored=%d", firstResult.PreciseMemoryUnits, secondResult.PreciseMemoryUnits, len(fake.unitsByKey))
	}
}

func preciseMemoryTestEvidence(id int64, sid string, turn int, excerpt string) store.DirectEvidence {
	return store.DirectEvidence{
		ID: id, ChatSessionID: sid, EvidenceKind: "turn_excerpt", EvidenceText: excerpt,
		SourceTurnStart: turn, SourceTurnEnd: turn, TurnAnchor: turn,
		ArchiveState: "verified_direct", CaptureStage: "critic_extract",
		CaptureVerification: "verified", CommittedGate: "auto_grounded_excerpt",
	}
}
