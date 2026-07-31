package httpapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type identityRecordingStore struct {
	*turnRecordingStore
	identities                []*store.EntityIdentity
	surfaces                  []*store.EntityIdentitySurface
	bindings                  []*store.EntityIdentityArtifactBinding
	attributions              []*store.SpeakerAttribution
	reviewedCanonicalLabels   map[string]bool
	reviewedCanonicalRoots    map[string]string
	reviewedOccurrenceToRoots map[string]string
}

func newIdentityRecordingStore() *identityRecordingStore {
	return &identityRecordingStore{
		turnRecordingStore:        &turnRecordingStore{},
		reviewedCanonicalLabels:   map[string]bool{},
		reviewedCanonicalRoots:    map[string]string{},
		reviewedOccurrenceToRoots: map[string]string{},
	}
}

func (f *identityRecordingStore) reviewCanonicalLabel(label string) {
	f.reviewedCanonicalLabels[comparableEntityKey(label)] = true
}

func (f *identityRecordingStore) SaveEntityIdentity(ctx context.Context, item *store.EntityIdentity) error {
	cp := *item
	f.identities = append(f.identities, &cp)
	key := comparableEntityKey(item.CanonicalLabel)
	if f.reviewedCanonicalLabels[key] {
		root := f.reviewedCanonicalRoots[key]
		if root == "" {
			root = item.StableEntityID
			f.reviewedCanonicalRoots[key] = root
		}
		f.reviewedOccurrenceToRoots[item.StableEntityID] = root
	}
	return nil
}

func (f *identityRecordingStore) ResolveReviewedCanonicalEntityID(_ context.Context, _ string, sourceEntityID string) (string, error) {
	if root := f.reviewedOccurrenceToRoots[sourceEntityID]; root != "" {
		return root, nil
	}
	return "", store.ErrNotFound
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

func Test36BSharedAliasSurfaceReplayDoesNotDependOnExtractionOrder(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	first := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "박한얼", "aliases": []any{"한얼"}},
				map[string]any{"name": "김한얼", "aliases": []any{"한얼"}},
			},
		},
	}
	second := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "김한얼", "aliases": []any{"한얼"}},
				map[string]any{"name": "박한얼", "aliases": []any{"한얼"}},
			},
		},
	}
	content := "박한얼이 말했다. 김한얼은 대답했다. 한얼은 조용했다."
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-alias-reorder", 6, first, content, completeTurnEmbeddingConfig{}, time.Unix(620, 0))
	firstSurfaceCount := len(fake.surfaces)
	_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-36b-alias-reorder", 6, second, content, completeTurnEmbeddingConfig{}, time.Unix(621, 0))

	aliasEvidence := func(surfaces []*store.EntityIdentitySurface) map[string]string {
		out := map[string]string{}
		for _, surface := range surfaces {
			if surface.SurfaceText != "한얼" {
				continue
			}
			out[surface.StableEntityID] = fmt.Sprintf("%s:%d:%d:%s", surface.SurfaceID, surface.SourceSpanStart, surface.SourceSpanEnd, surface.ReviewState)
		}
		return out
	}
	firstAliases := aliasEvidence(fake.surfaces[:firstSurfaceCount])
	secondAliases := aliasEvidence(fake.surfaces[firstSurfaceCount:])
	if len(firstAliases) != 2 || len(secondAliases) != 2 {
		t.Fatalf("shared alias surfaces missing: first=%#v second=%#v", firstAliases, secondAliases)
	}
	for stableID, firstEvidence := range firstAliases {
		if secondAliases[stableID] != firstEvidence {
			t.Fatalf("shared alias evidence changed with extraction order: first=%#v second=%#v", firstAliases, secondAliases)
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

func Test36BSharedNestedAliasRoutesArtifactsToUnknownIdentity(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		firstName  string
		secondName string
		alias      string
		content    string
		excerpt    string
	}{
		{
			name:       "korean_homonym",
			sessionID:  "sess-36b-shared-alias-ko",
			firstName:  "박한얼",
			secondName: "김한얼",
			alias:      "한얼",
			content:    "박한얼이 말했다. 김한얼은 대답했다.",
			excerpt:    "박한얼이 말했다.",
		},
		{
			name:       "japanese_homonym",
			sessionID:  "sess-36b-shared-alias-ja",
			firstName:  "호시노 아이",
			secondName: "하야사카 아이",
			alias:      "아이",
			content:    "호시노 아이가 말했다. 하야사카 아이는 대답했다.",
			excerpt:    "호시노 아이가 말했다.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := newIdentityRecordingStore()
			srv := NewServer(config.Default())
			srv.Store = fake
			extraction := map[string]any{
				"entities": map[string]any{
					"characters": []any{
						map[string]any{"name": tc.firstName, "aliases": []any{tc.alias}},
						map[string]any{"name": tc.secondName, "aliases": []any{tc.alias}},
					},
				},
				"kg_triples": []any{
					map[string]any{"subject": tc.alias, "predicate": "met", "object": tc.secondName},
				},
				"character_deltas": []any{
					map[string]any{"name": tc.alias, "status": map[string]any{"mood": "calm"}},
				},
				"speaker_attributions": []any{
					map[string]any{
						"speaker_name":      tc.alias,
						"attribution_kind":  "dialogue",
						"attribution_state": "linked",
						"confidence":        0.9,
						"evidence_excerpt":  tc.excerpt,
					},
				},
			}

			result := srv.saveCriticExtractionArtifacts(
				context.Background(),
				tc.sessionID,
				14,
				extraction,
				tc.content,
				completeTurnEmbeddingConfig{},
				time.Unix(1400, 0),
			)
			if result.Errors != 0 {
				t.Fatalf("identity projection errors: %#v", result.ErrorDetails)
			}

			aliasSurfaces := 0
			for _, surface := range fake.surfaces {
				if surface.SurfaceText != tc.alias {
					continue
				}
				aliasSurfaces++
				if surface.SourceSpanStart != -1 || surface.SourceSpanEnd != -1 ||
					surface.ReviewState != "needs_review" || surface.EvidenceExcerpt != "" {
					t.Fatalf("nested alias gained independent source authority: %#v", surface)
				}
			}
			if aliasSurfaces != 2 {
				t.Fatalf("shared alias metadata was not persisted for both owners: %#v", fake.surfaces)
			}

			namespaceByID := map[string]string{}
			for _, identity := range fake.identities {
				namespaceByID[identity.StableEntityID] = identity.IdentityNamespace
			}
			found := map[string]bool{}
			for _, binding := range fake.bindings {
				if binding.SurfaceText != tc.alias {
					continue
				}
				key := binding.ArtifactKind + ":" + binding.ArtifactRole
				if key != "kg_triple:subject" && key != "character_state:owner" {
					continue
				}
				found[key] = true
				if namespaceByID[binding.StableEntityID] != "session_unknown" ||
					binding.ReviewState != "needs_review" {
					t.Fatalf("ambiguous alias binding was promoted: binding=%#v identities=%#v", binding, fake.identities)
				}
			}
			if !found["kg_triple:subject"] || !found["character_state:owner"] {
				t.Fatalf("ambiguous KG/state bindings missing: %#v", fake.bindings)
			}

			if len(fake.attributions) != 1 {
				t.Fatalf("expected one grounded speaker attribution, got %#v", fake.attributions)
			}
			attribution := fake.attributions[0]
			if attribution.IdentityNamespace != "session_unknown" ||
				attribution.ReviewState != "needs_review" ||
				attribution.AttributionState != "tentative" {
				t.Fatalf("ambiguous alias speaker was promoted: %#v", attribution)
			}
		})
	}
}

func Test36BShortDisplayNameDoesNotStealLongerNameSpan(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	content := "호시노 아이가 말했다. 하야사카 아이는 대답했다."
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "호시노 아이", "aliases": []any{"아이"}},
				map[string]any{"name": "하야사카 아이", "aliases": []any{"아이"}},
				map[string]any{"name": "아이"},
			},
		},
	}

	result := srv.saveCriticExtractionArtifacts(
		context.Background(),
		"sess-36b-short-display",
		15,
		extraction,
		content,
		completeTurnEmbeddingConfig{},
		time.Unix(1500, 0),
	)
	if result.Errors != 0 {
		t.Fatalf("identity projection errors: %#v", result.ErrorDetails)
	}

	var shortIdentity *store.EntityIdentity
	for _, identity := range fake.identities {
		if identity.CanonicalLabel == "아이" {
			shortIdentity = identity
			break
		}
	}
	if shortIdentity == nil {
		t.Fatalf("short display identity missing: %#v", fake.identities)
	}
	if shortIdentity.ReviewState != "needs_review" ||
		shortIdentity.PresenceAuthority != "unverified" ||
		shortIdentity.OccurrenceAuthority != "none" {
		t.Fatalf("short display stole a nested full-name span: %#v", shortIdentity)
	}
	for _, surface := range fake.surfaces {
		if surface.StableEntityID != shortIdentity.StableEntityID || surface.SurfaceText != "아이" {
			continue
		}
		if surface.SourceSpanStart != -1 || surface.SourceSpanEnd != -1 ||
			surface.ReviewState != "needs_review" || surface.EvidenceExcerpt != "" {
			t.Fatalf("short display surface gained nested source authority: %#v", surface)
		}
		return
	}
	t.Fatalf("short display surface missing: %#v", fake.surfaces)
}

func Test36BStandaloneSharedAliasRemainsIdentityAmbiguous(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	content := "호시노 아이가 말했다. 하야사카 아이는 대답했다. 아이가 혼자 웃었다."
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "호시노 아이", "aliases": []any{"아이"}},
				map[string]any{"name": "하야사카 아이", "aliases": []any{"아이"}},
				map[string]any{"name": "아이"},
			},
		},
	}

	result := srv.saveCriticExtractionArtifacts(
		context.Background(),
		"sess-36b-standalone-shared-alias",
		16,
		extraction,
		content,
		completeTurnEmbeddingConfig{},
		time.Unix(1600, 0),
	)
	if result.Errors != 0 {
		t.Fatalf("identity projection errors: %#v", result.ErrorDetails)
	}

	var shortIdentity *store.EntityIdentity
	for _, identity := range fake.identities {
		if identity.CanonicalLabel == "아이" {
			shortIdentity = identity
			break
		}
	}
	if shortIdentity == nil {
		t.Fatalf("short display identity missing: %#v", fake.identities)
	}
	if shortIdentity.ReviewState != "needs_review" ||
		shortIdentity.PresenceAuthority != "observed" ||
		shortIdentity.OccurrenceAuthority != "source_span" {
		t.Fatalf("standalone shared alias was promoted or lost source presence: %#v", shortIdentity)
	}
	for _, surface := range fake.surfaces {
		if surface.StableEntityID != shortIdentity.StableEntityID || surface.SurfaceText != "아이" {
			continue
		}
		if surface.SourceSpanStart < 0 || surface.SourceSpanEnd <= surface.SourceSpanStart ||
			surface.ReviewState != "needs_review" || surface.EvidenceExcerpt != "아이" {
			t.Fatalf("standalone shared alias surface lost ambiguity evidence: %#v", surface)
		}
		return
	}
	t.Fatalf("standalone shared alias surface missing: %#v", fake.surfaces)
}

func Test36BSpeakerAliasNestedInAnotherFullNameDoesNotLink(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	excerpt := "아이가 말했다."
	content := "아이는 먼저 떠났다. 하야사카 " + excerpt
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "아이"},
				map[string]any{"name": "하야사카 아이"},
			},
		},
		"speaker_attributions": []any{
			map[string]any{
				"speaker_name":      "아이",
				"attribution_kind":  "dialogue",
				"attribution_state": "linked",
				"confidence":        0.9,
				"evidence_excerpt":  excerpt,
			},
		},
	}

	result := srv.saveCriticExtractionArtifacts(
		context.Background(),
		"sess-36b-speaker-nested-alias",
		17,
		extraction,
		content,
		completeTurnEmbeddingConfig{},
		time.Unix(1700, 0),
	)
	if result.Errors != 0 {
		t.Fatalf("identity projection errors: %#v", result.ErrorDetails)
	}
	if len(fake.attributions) != 1 {
		t.Fatalf("expected one speaker attribution, got %#v", fake.attributions)
	}
	attribution := fake.attributions[0]
	if attribution.IdentityNamespace != "session_unknown" ||
		attribution.ReviewState != "needs_review" ||
		attribution.AttributionState != "tentative" {
		t.Fatalf("nested speaker alias linked to the wrong full name: %#v", attribution)
	}
}

func Test36BAliasOnlyCommonNounMentionDoesNotResolveIdentity(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	excerpt := "아이가 웃었다."
	content := "호시노 아이가 들어왔다. " + excerpt
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "호시노 아이", "aliases": []any{"아이"}},
			},
		},
		"kg_triples": []any{
			map[string]any{"subject": "아이", "predicate": "laughed_near", "object": "호시노 아이"},
		},
		"character_deltas": []any{
			map[string]any{"name": "아이", "status": map[string]any{"mood": "happy"}},
		},
		"speaker_attributions": []any{
			map[string]any{
				"speaker_name":      "아이",
				"attribution_kind":  "dialogue",
				"attribution_state": "linked",
				"confidence":        0.9,
				"evidence_excerpt":  excerpt,
			},
		},
	}

	result := srv.saveCriticExtractionArtifacts(
		context.Background(),
		"sess-36b-common-noun-alias",
		18,
		extraction,
		content,
		completeTurnEmbeddingConfig{},
		time.Unix(1800, 0),
	)
	if result.Errors != 0 {
		t.Fatalf("identity projection errors: %#v", result.ErrorDetails)
	}

	for _, surface := range fake.surfaces {
		if surface.SurfaceText != "아이" || surface.SurfaceKind == "display_name" {
			continue
		}
		if surface.SourceSpanStart < 0 || surface.ReviewState != "needs_review" ||
			surface.EvidenceExcerpt != "아이" {
			t.Fatalf("alias evidence was lost or promoted: %#v", surface)
		}
	}
	namespaceByID := map[string]string{}
	for _, identity := range fake.identities {
		namespaceByID[identity.StableEntityID] = identity.IdentityNamespace
	}
	found := map[string]bool{}
	for _, binding := range fake.bindings {
		if binding.SurfaceText != "아이" {
			continue
		}
		key := binding.ArtifactKind + ":" + binding.ArtifactRole
		if key != "kg_triple:subject" && key != "character_state:owner" {
			continue
		}
		found[key] = true
		if namespaceByID[binding.StableEntityID] != "session_unknown" ||
			binding.ReviewState != "needs_review" {
			t.Fatalf("alias-only mention resolved to a character: binding=%#v identities=%#v", binding, fake.identities)
		}
	}
	if !found["kg_triple:subject"] || !found["character_state:owner"] {
		t.Fatalf("alias-only unresolved bindings missing: %#v", fake.bindings)
	}
	if len(fake.attributions) != 1 ||
		fake.attributions[0].IdentityNamespace != "session_unknown" ||
		fake.attributions[0].ReviewState != "needs_review" {
		t.Fatalf("alias-only speaker resolved to a character: %#v", fake.attributions)
	}
}

func Test36BSkippedPlaceholderEntityCannotCreateAliasConflict(t *testing.T) {
	fake := newIdentityRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "Mina"},
				map[string]any{"name": "user", "aliases": []any{"Mina"}},
			},
		},
	}
	result := srv.saveCriticExtractionArtifacts(
		context.Background(),
		"sess-36b-placeholder-alias",
		19,
		extraction,
		"Mina arrived.",
		completeTurnEmbeddingConfig{},
		time.Unix(1900, 0),
	)
	if result.Errors != 0 {
		t.Fatalf("identity projection errors: %#v", result.ErrorDetails)
	}
	if len(fake.identities) != 1 ||
		fake.identities[0].CanonicalLabel != "Mina" ||
		fake.identities[0].ReviewState != "source_observed" {
		t.Fatalf("skipped placeholder created a ghost alias conflict: %#v", fake.identities)
	}
}
