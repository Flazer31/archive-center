package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type identityRecordingStore struct {
	*turnRecordingStore
	identities   []*store.EntityIdentity
	surfaces     []*store.EntityIdentitySurface
	bindings     []*store.EntityIdentityArtifactBinding
	attributions []*store.SpeakerAttribution
}

func newIdentityRecordingStore() *identityRecordingStore {
	return &identityRecordingStore{turnRecordingStore: &turnRecordingStore{}}
}

func (f *identityRecordingStore) SaveEntityIdentity(ctx context.Context, item *store.EntityIdentity) error {
	cp := *item
	f.identities = append(f.identities, &cp)
	return nil
}

func (f *identityRecordingStore) SaveEntityIdentitySurface(ctx context.Context, item *store.EntityIdentitySurface) error {
	cp := *item
	f.surfaces = append(f.surfaces, &cp)
	return nil
}

func (f *identityRecordingStore) SaveEntityIdentityArtifactBinding(ctx context.Context, item *store.EntityIdentityArtifactBinding) error {
	cp := *item
	f.bindings = append(f.bindings, &cp)
	return nil
}

func (f *identityRecordingStore) SaveSpeakerAttribution(ctx context.Context, item *store.SpeakerAttribution) error {
	cp := *item
	f.attributions = append(f.attributions, &cp)
	return nil
}

func Test36BHomonymsRemainSeparateStableIdentities(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "Alex", "role": "guard"},
				map[string]any{"name": "Alex", "role": "merchant"},
			},
		},
		"kg_triples": []any{
			map[string]any{"subject": "Alex", "predicate": "met", "object": "Alex"},
		},
	}

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-homonym", 4, extraction, "Alex the guard met Alex the merchant.", completeTurnEmbeddingConfig{}, time.Unix(400, 0))
	if result.Errors != 0 {
		t.Fatalf("identity projection errors: %#v", result.ErrorDetails)
	}
	if len(fake.identities) < 3 {
		t.Fatalf("expected two homonyms and an unresolved KG identity, got %#v", fake.identities)
	}
	if fake.identities[0].StableEntityID == fake.identities[1].StableEntityID {
		t.Fatalf("same display name must not merge stable IDs: %#v", fake.identities[:2])
	}
	if fake.identities[0].CanonicalLabel != "Alex" || fake.identities[1].CanonicalLabel != "Alex" {
		t.Fatalf("source labels should be preserved: %#v", fake.identities[:2])
	}
	if fake.identities[0].IdentityNamespace != "session_npc" || fake.identities[1].IdentityNamespace != "session_npc" {
		t.Fatalf("character roles must not replace the character identity kind: %#v", fake.identities[:2])
	}
	foundReviewBinding := false
	for _, binding := range fake.bindings {
		if binding.ArtifactKind == "kg_triple" && binding.ReviewState == "needs_review" {
			foundReviewBinding = true
		}
	}
	if !foundReviewBinding {
		t.Fatalf("ambiguous KG endpoints must retain needs_review bindings: %#v", fake.bindings)
	}
}

func Test36BExactSourceReplayKeepsDeterministicIdentityID(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	makeExtraction := func() map[string]any {
		return map[string]any{
			"entities": map[string]any{
				"characters": []any{map[string]any{"name": "Mina"}},
			},
		}
	}
	content := "Mina entered the workshop."
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-replay", 6, makeExtraction(), content, completeTurnEmbeddingConfig{}, time.Unix(600, 0))
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-replay", 6, makeExtraction(), content, completeTurnEmbeddingConfig{}, time.Unix(601, 0))
	if len(fake.identities) != 2 {
		t.Fatalf("expected two attempted idempotent writes, got %d", len(fake.identities))
	}
	if fake.identities[0].StableEntityID != fake.identities[1].StableEntityID ||
		fake.identities[0].IdempotencyKey != fake.identities[1].IdempotencyKey {
		t.Fatalf("same source replay must reuse identity key: %#v", fake.identities)
	}
}

func Test36BExactSourceReplayDoesNotDependOnExtractionOrder(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	first := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "Mina"},
				map[string]any{"name": "Rowan"},
			},
		},
	}
	second := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "Rowan"},
				map[string]any{"name": "Mina"},
			},
		},
	}
	content := "Mina greeted Rowan."
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-reorder", 6, first, content, completeTurnEmbeddingConfig{}, time.Unix(610, 0))
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-reorder", 6, second, content, completeTurnEmbeddingConfig{}, time.Unix(611, 0))
	if len(fake.identities) != 4 {
		t.Fatalf("expected four attempted writes, got %#v", fake.identities)
	}
	firstIDs := map[string]string{
		fake.identities[0].CanonicalLabel: fake.identities[0].StableEntityID,
		fake.identities[1].CanonicalLabel: fake.identities[1].StableEntityID,
	}
	for _, identity := range fake.identities[2:] {
		if firstIDs[identity.CanonicalLabel] != identity.StableEntityID {
			t.Fatalf("stable identity changed with extraction order: first=%#v replay=%#v", firstIDs, fake.identities[2:])
		}
	}
}

func Test36BNamespaceIsolationDoesNotMergeSameLabel(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "Mina", "identity_namespace": "session_npc"},
				map[string]any{"name": "Mina", "identity_namespace": "reference_entity"},
			},
		},
	}
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-namespace", 7, extraction, "Mina appears in the scene.", completeTurnEmbeddingConfig{}, time.Unix(700, 0))
	if len(fake.identities) != 2 {
		t.Fatalf("expected two namespace-scoped identities, got %#v", fake.identities)
	}
	if fake.identities[0].StableEntityID == fake.identities[1].StableEntityID {
		t.Fatalf("session/reference identities merged: %#v", fake.identities)
	}
	if fake.identities[1].IdentityNamespace != "reference_entity" ||
		fake.identities[1].PresenceAuthority != "unverified" ||
		fake.identities[1].ReviewState != "needs_review" {
		t.Fatalf("reference-only identity must not gain session presence: %#v", fake.identities[1])
	}
}

func Test36BAmbiguousSpeakerPreservesGroundedSpanAndReviewState(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	excerpt := `Alex said, "Wait."`
	content := `Alex said, "Wait." Alex said, "Go."`
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "Alex", "role": "guard"},
				map[string]any{"name": "Alex", "role": "merchant"},
			},
		},
		"speaker_attributions": []any{
			map[string]any{
				"speaker_name":      "Alex",
				"attribution_kind":  "dialogue",
				"attribution_state": "ambiguous",
				"confidence":        0.55,
				"evidence_excerpt":  excerpt,
			},
		},
	}
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-speaker", 8, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(800, 0))
	if len(fake.attributions) != 1 {
		t.Fatalf("expected one grounded attribution, got %#v", fake.attributions)
	}
	got := fake.attributions[0]
	if got.AttributionState != "tentative" || got.ReviewState != "needs_review" {
		t.Fatalf("ambiguous attribution was promoted: %#v", got)
	}
	if got.EvidenceExcerpt != excerpt || got.SourceSpanStart != 0 || got.SourceSpanEnd != len(excerpt) {
		t.Fatalf("grounded raw span was not preserved: %#v", got)
	}
	if got.IdentityNamespace != "session_unknown" {
		t.Fatalf("ambiguous speaker must bind only to an unknown occurrence: %#v", got)
	}
}

func Test36BUniqueSameTurnKGAndStateUseStableBindings(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "Mina"},
				map[string]any{"name": "Rowan"},
			},
		},
		"kg_triples": []any{
			map[string]any{"subject": "Mina", "predicate": "trusts", "object": "Rowan"},
		},
		"character_deltas": []any{
			map[string]any{"name": "Mina", "status": map[string]any{"mood": "calm"}},
		},
	}
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-binding", 9, extraction, "Mina trusts Rowan. Mina remains calm.", completeTurnEmbeddingConfig{}, time.Unix(900, 0))
	identityByLabel := map[string]string{}
	for _, identity := range fake.identities {
		if identity.IdentityNamespace == "session_npc" {
			identityByLabel[identity.CanonicalLabel] = identity.StableEntityID
		}
	}
	roles := map[string]string{}
	for _, binding := range fake.bindings {
		if binding.ArtifactKind == "kg_triple" || binding.ArtifactKind == "character_state" {
			roles[binding.ArtifactKind+":"+binding.ArtifactRole+":"+binding.SurfaceText] = binding.StableEntityID
		}
	}
	if roles["kg_triple:subject:Mina"] != identityByLabel["Mina"] ||
		roles["kg_triple:object:Rowan"] != identityByLabel["Rowan"] ||
		roles["character_state:owner:Mina"] != identityByLabel["Mina"] {
		t.Fatalf("same-turn stable bindings are disconnected: identities=%#v bindings=%#v", identityByLabel, roles)
	}
}

func Test36BUngroundedSpeakerAttributionIsRejected(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-ungrounded", 10, map[string]any{
		"speaker_attributions": []any{
			map[string]any{"speaker_name": "Mina", "evidence_excerpt": "This text is absent."},
		},
	}, "Mina stayed silent.", completeTurnEmbeddingConfig{}, time.Unix(1000, 0))
	if len(fake.attributions) != 0 {
		t.Fatalf("ungrounded attribution must not persist: %#v", fake.attributions)
	}
	for _, reason := range result.SkipReasons {
		if stringFromMap(reason, "surface") == "speaker_attributions" &&
			stringFromMap(reason, "reason") == "not_grounded_in_current_turn" {
			return
		}
	}
	t.Fatalf("missing grounded-attribution skip reason: %#v", result.SkipReasons)
}

func Test36BUngroundedCharacterStateBindingNeedsReview(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-state-review", 11, map[string]any{
		"character_deltas": []any{
			map[string]any{"name": "Mina", "status": map[string]any{"mood": "calm"}},
		},
	}, "Someone stayed calm.", completeTurnEmbeddingConfig{}, time.Unix(1100, 0))

	for _, binding := range fake.bindings {
		if binding.ArtifactKind == "character_state" {
			if binding.ReviewState != "needs_review" {
				t.Fatalf("ungrounded character state binding was promoted: %#v", binding)
			}
			return
		}
	}
	t.Fatalf("missing character state identity binding: %#v", fake.bindings)
}

func Test36BRepeatedSpeakerExcerptUsesDistinctSourceOccurrences(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	excerpt := `Mina said, "Wait."`
	content := excerpt + " " + excerpt
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-speaker-repeat", 12, map[string]any{
		"entities": map[string]any{
			"characters": []any{map[string]any{"name": "Mina"}},
		},
		"speaker_attributions": []any{
			map[string]any{"speaker_name": "Mina", "attribution_kind": "dialogue", "attribution_state": "linked", "evidence_excerpt": excerpt},
			map[string]any{"speaker_name": "Mina", "attribution_kind": "dialogue", "attribution_state": "linked", "evidence_excerpt": excerpt},
		},
	}, content, completeTurnEmbeddingConfig{}, time.Unix(1200, 0))

	if len(fake.attributions) != 2 {
		t.Fatalf("expected two grounded attributions, got %#v", fake.attributions)
	}
	if fake.attributions[0].SourceSpanStart != 0 ||
		fake.attributions[1].SourceSpanStart != len(excerpt)+1 {
		t.Fatalf("repeated excerpts did not retain separate source spans: %#v", fake.attributions)
	}
}

func Test36BNoopDualWriteDoesNotReportIdentityPersistenceErrors(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = store.NewDualWriteStore(store.NewNoopStore(), store.NewNoopStore())
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-noop", 13, map[string]any{
		"entities": map[string]any{
			"characters": []any{map[string]any{"name": "Mina"}},
		},
	}, "Mina waited.", completeTurnEmbeddingConfig{}, time.Unix(1300, 0))

	if result.Errors != 0 {
		t.Fatalf("disabled optional identity lane reported persistence errors: %#v", result.ErrorDetails)
	}
	if result.EntityIdentities != 0 || result.IdentitySurfaces != 0 || result.IdentityBindings != 0 {
		t.Fatalf("disabled optional identity lane reported saved counts: %#v", result)
	}
}
