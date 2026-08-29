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
