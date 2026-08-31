package httpapi

import (
	"context"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type normalizeCharacterIdentityStore struct {
	*characterIdentityMergeFakeStore
	sources          []store.MemorySourceRevision
	savedIdentities  []store.EntityIdentity
	savedSurfaces    []store.EntityIdentitySurface
	identitySaveFail map[string]error
}

func (f *normalizeCharacterIdentityStore) ListSourceRevisions(_ context.Context, sid string, fromTurn, toTurn int) ([]store.MemorySourceRevision, error) {
	out := []store.MemorySourceRevision{}
	for _, source := range f.sources {
		if source.ChatSessionID != sid || (fromTurn > 0 && source.TurnIndex < fromTurn) || (toTurn > 0 && source.TurnIndex > toTurn) {
			continue
		}
		out = append(out, source)
	}
	return out, nil
}

func (f *normalizeCharacterIdentityStore) EntityIdentityWritesEnabled() bool { return true }

func (f *normalizeCharacterIdentityStore) SaveEntityIdentity(_ context.Context, item *store.EntityIdentity) error {
	if err := f.identitySaveFail[item.CanonicalLabel]; err != nil {
		return err
	}
	for _, existing := range f.identities {
		if existing.StableEntityID == item.StableEntityID {
			return nil
		}
	}
	f.identities = append(f.identities, *item)
	f.savedIdentities = append(f.savedIdentities, *item)
	return nil
}

func (f *normalizeCharacterIdentityStore) SaveEntityIdentitySurface(_ context.Context, item *store.EntityIdentitySurface) error {
	for _, existing := range f.surfaces {
		if existing.SurfaceID == item.SurfaceID {
			return nil
		}
	}
	f.surfaces = append(f.surfaces, *item)
	f.savedSurfaces = append(f.savedSurfaces, *item)
	return nil
}

func (f *normalizeCharacterIdentityStore) SaveEntityIdentityArtifactBinding(context.Context, *store.EntityIdentityArtifactBinding) error {
	return nil
}

func (f *normalizeCharacterIdentityStore) SaveSpeakerAttribution(context.Context, *store.SpeakerAttribution) error {
	return nil
}

func TestSessionNormalizeRepairsOnlyMissingExactCharacterIdentities(t *testing.T) {
	const sid = "sess-normalize-character"
	fake := &normalizeCharacterIdentityStore{
		characterIdentityMergeFakeStore: &characterIdentityMergeFakeStore{
			narrativeFakeStore: &narrativeFakeStore{characterStates: []store.CharacterState{
				{ID: 1, ChatSessionID: sid, CharacterName: "강한얼", TurnIndex: 1},
				{ID: 2, ChatSessionID: sid, CharacterName: "흉터 있는 큰 장정", TurnIndex: 4},
				{ID: 3, ChatSessionID: sid, CharacterName: "복면인", TurnIndex: 5},
				{ID: 4, ChatSessionID: sid, CharacterName: "근거 없는 인물", TurnIndex: 9},
			}},
			identities: []store.EntityIdentity{
				{StableEntityID: "hero-id", ChatSessionID: sid, EntityKind: "character", CanonicalLabel: "강한얼"},
				{StableEntityID: "masked-a", ChatSessionID: sid, EntityKind: "character", CanonicalLabel: "복면인"},
				{StableEntityID: "masked-b", ChatSessionID: sid, EntityKind: "character", CanonicalLabel: "복면인"},
			},
			surfaces: []store.EntityIdentitySurface{
				{SurfaceID: "hero-surface", StableEntityID: "hero-id", ChatSessionID: sid, SurfaceText: "강한얼", NormalizedSurface: comparableEntityKey("강한얼")},
			},
		},
		sources: []store.MemorySourceRevision{
			{ChatSessionID: sid, SourceRevision: "rev-1", LogicalTurnID: "turn-1", TurnIndex: 1, LifecycleState: "active"},
			{ChatSessionID: sid, SourceRevision: "rev-4", LogicalTurnID: "turn-4", TurnIndex: 4, LifecycleState: "active", CombinedContentHash: "hash-4"},
			{ChatSessionID: sid, SourceRevision: "rev-5", LogicalTurnID: "turn-5", TurnIndex: 5, LifecycleState: "active"},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	originalStateCount := len(fake.characterStates)

	dryRun := srv.repairMissingCharacterIdentities(context.Background(), sid, true)
	if intFromAny(dryRun["would_create"], 0) != 1 || len(fake.savedIdentities) != 0 || len(fake.savedSurfaces) != 0 {
		t.Fatalf("dry-run result=%#v identities=%#v surfaces=%#v", dryRun, fake.savedIdentities, fake.savedSurfaces)
	}
	if intFromAny(dryRun["skipped"], 0) != 2 {
		t.Fatalf("dry-run did not report ambiguous/no-source items separately: %#v", dryRun)
	}

	result := srv.repairMissingCharacterIdentities(context.Background(), sid, false)
	if result["status"] != "ok" || intFromAny(result["created_identities"], 0) != 1 || intFromAny(result["created_surfaces"], 0) != 1 {
		t.Fatalf("repair result=%#v", result)
	}
	if len(fake.savedIdentities) != 1 || fake.savedIdentities[0].CanonicalLabel != "흉터 있는 큰 장정" ||
		fake.savedIdentities[0].SourceRevision != "rev-4" || fake.savedIdentities[0].SourceContract != completeTurnSourceAcceptanceContract {
		t.Fatalf("saved identity=%#v", fake.savedIdentities)
	}
	if len(fake.savedSurfaces) != 1 || fake.savedSurfaces[0].SurfaceText != "흉터 있는 큰 장정" ||
		fake.savedSurfaces[0].ReviewState != store.EntityIdentityReviewStateSourceObserved {
		t.Fatalf("saved surfaces=%#v", fake.savedSurfaces)
	}
	if len(fake.characterStates) != originalStateCount {
		t.Fatal("identity repair rewrote character-state history")
	}

	repeat := srv.repairMissingCharacterIdentities(context.Background(), sid, false)
	if intFromAny(repeat["created_identities"], 0) != 0 || intFromAny(repeat["created_surfaces"], 0) != 0 || len(fake.savedIdentities) != 1 || len(fake.savedSurfaces) != 1 {
		t.Fatalf("repeated repair was not idempotent: result=%#v identities=%d surfaces=%d", repeat, len(fake.savedIdentities), len(fake.savedSurfaces))
	}
}

func TestSessionNormalizeRepairsMissingExactItemIdentitiesWithoutReprocessing(t *testing.T) {
	const sid = "sess-normalize-items"
	fake := &normalizeCharacterIdentityStore{
		characterIdentityMergeFakeStore: &characterIdentityMergeFakeStore{
			narrativeFakeStore: &narrativeFakeStore{kgTriples: []store.KGTriple{
				{ID: 4, ChatSessionID: sid, Subject: "강한얼", Predicate: "소유", Object: "가죽 전대", SourceTurn: 4},
				{ID: 5, ChatSessionID: sid, Subject: "강한얼", Predicate: "장비", Object: "은전 주머니", SourceTurn: 5},
				{ID: 6, ChatSessionID: sid, Subject: "화재", Predicate: "causes", Object: "연기", SourceTurn: 6},
				{ID: 7, ChatSessionID: sid, Subject: "강한얼", Predicate: "haste", Object: "성급함", SourceTurn: 7},
				{ID: 9, ChatSessionID: sid, Subject: "강한얼", Predicate: "소유", Object: "출처 모호한 물품", SourceTurn: 9},
			}},
			identities: []store.EntityIdentity{
				{StableEntityID: "silver-pouch", ChatSessionID: sid, IdentityNamespace: "session_item", EntityKind: "item", CanonicalLabel: "은전 주머니"},
			},
		},
		sources: []store.MemorySourceRevision{
			{ChatSessionID: sid, SourceRevision: "rev-4", LogicalTurnID: "turn-4", TurnIndex: 4, LifecycleState: "active", CombinedContentHash: "hash-4"},
			{ChatSessionID: sid, SourceRevision: "rev-5", LogicalTurnID: "turn-5", TurnIndex: 5, LifecycleState: "active", CombinedContentHash: "hash-5"},
			{ChatSessionID: sid, SourceRevision: "rev-6", LogicalTurnID: "turn-6", TurnIndex: 6, LifecycleState: "active", CombinedContentHash: "hash-6"},
			{ChatSessionID: sid, SourceRevision: "rev-7", LogicalTurnID: "turn-7", TurnIndex: 7, LifecycleState: "active", CombinedContentHash: "hash-7"},
			{ChatSessionID: sid, SourceRevision: "rev-9-a", LogicalTurnID: "turn-9-a", TurnIndex: 9, LifecycleState: "active"},
			{ChatSessionID: sid, SourceRevision: "rev-9-b", LogicalTurnID: "turn-9-b", TurnIndex: 9, LifecycleState: "active"},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	originalKGCount := len(fake.kgTriples)

	dryRun := srv.repairMissingCharacterIdentities(context.Background(), sid, true)
	if intFromAny(dryRun["item_candidates"], 0) != 3 || intFromAny(dryRun["would_create"], 0) != 2 {
		t.Fatalf("item dry-run result=%#v", dryRun)
	}
	if len(fake.savedIdentities) != 0 || len(fake.savedSurfaces) != 0 {
		t.Fatal("item identity dry-run performed writes")
	}

	result := srv.repairMissingCharacterIdentities(context.Background(), sid, false)
	if result["status"] != "ok" || intFromAny(result["item_created_identities"], 0) != 1 || intFromAny(result["item_created_surfaces"], 0) != 2 {
		t.Fatalf("item repair result=%#v", result)
	}
	if intFromAny(result["skipped"], 0) != 1 {
		t.Fatalf("ambiguous item source was not reported separately: %#v", result)
	}
	var leatherIdentity *store.EntityIdentity
	for index := range fake.savedIdentities {
		if fake.savedIdentities[index].CanonicalLabel == "가죽 전대" {
			leatherIdentity = &fake.savedIdentities[index]
		}
	}
	if leatherIdentity == nil || leatherIdentity.EntityKind != "item" || leatherIdentity.IdentityNamespace != "session_item" ||
		leatherIdentity.SourceRevision != "rev-4" || leatherIdentity.SourceTurn != 4 {
		t.Fatalf("missing item identity did not use exact name/source revision: %#v", fake.savedIdentities)
	}
	surfaceSources := map[string]string{}
	for _, surface := range fake.savedSurfaces {
		surfaceSources[surface.SurfaceText] = surface.SourceRevision
	}
	if surfaceSources["가죽 전대"] != "rev-4" || surfaceSources["은전 주머니"] != "rev-5" {
		t.Fatalf("item surfaces did not preserve exact source revisions: %#v", fake.savedSurfaces)
	}
	if surfaceSources["연기"] != "" || surfaceSources["성급함"] != "" {
		t.Fatalf("partial predicate matches created durable item surfaces: %#v", fake.savedSurfaces)
	}
	if len(fake.kgTriples) != originalKGCount {
		t.Fatal("item identity repair rewrote KG history")
	}

	repeat := srv.repairMissingCharacterIdentities(context.Background(), sid, false)
	if intFromAny(repeat["item_created_identities"], 0) != 0 || intFromAny(repeat["item_created_surfaces"], 0) != 0 ||
		len(fake.savedIdentities) != 1 || len(fake.savedSurfaces) != 2 {
		t.Fatalf("repeated item identity repair was not idempotent: result=%#v identities=%d surfaces=%d", repeat, len(fake.savedIdentities), len(fake.savedSurfaces))
	}
}
