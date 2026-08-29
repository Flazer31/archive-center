package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type characterIdentityMergeFakeStore struct {
	*narrativeFakeStore
	identities []store.EntityIdentity
	surfaces   []store.EntityIdentitySurface
	links      []store.EntityIdentityLink
	subjective []store.ProtagonistEntityMemory
	kgErr      error
	linkErrors map[string]error
}

type kgIdentityCountingStore struct {
	*memoryFakeStore
	identities    []store.EntityIdentity
	surfaces      []store.EntityIdentitySurface
	links         []store.EntityIdentityLink
	identityReads int
	surfaceReads  int
	linkReads     int
}

type entityExplorerIdentityCountingStore struct {
	*characterIdentityMergeFakeStore
	identityReads      int
	surfaceReads       int
	linkReads          int
	uniqueSurfaceReads int
	reviewedRootReads  int
}

func (f *entityExplorerIdentityCountingStore) ListActiveEntityIdentities(ctx context.Context, sid string) ([]store.EntityIdentity, error) {
	f.identityReads++
	return f.characterIdentityMergeFakeStore.ListActiveEntityIdentities(ctx, sid)
}

func (f *entityExplorerIdentityCountingStore) ListActiveEntityIdentitySurfaces(ctx context.Context, sid string) ([]store.EntityIdentitySurface, error) {
	f.surfaceReads++
	return f.characterIdentityMergeFakeStore.ListActiveEntityIdentitySurfaces(ctx, sid)
}

func (f *entityExplorerIdentityCountingStore) ListReviewedEntityIdentityLinks(ctx context.Context, sid string) ([]store.EntityIdentityLink, error) {
	f.linkReads++
	return f.characterIdentityMergeFakeStore.ListReviewedEntityIdentityLinks(ctx, sid)
}

func (f *entityExplorerIdentityCountingStore) ResolveUniqueActiveEntityIdentityBySurface(ctx context.Context, sid, normalized string) (store.ResolvedEntityIdentity, error) {
	f.uniqueSurfaceReads++
	return f.characterIdentityMergeFakeStore.ResolveUniqueActiveEntityIdentityBySurface(ctx, sid, normalized)
}

func (f *entityExplorerIdentityCountingStore) ResolveReviewedCanonicalEntityID(ctx context.Context, sid, sourceID string) (string, error) {
	f.reviewedRootReads++
	return f.characterIdentityMergeFakeStore.ResolveReviewedCanonicalEntityID(ctx, sid, sourceID)
}

func (f *kgIdentityCountingStore) ListActiveEntityIdentities(_ context.Context, sid string) ([]store.EntityIdentity, error) {
	f.identityReads++
	out := []store.EntityIdentity{}
	for _, item := range f.identities {
		if item.ChatSessionID == sid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *kgIdentityCountingStore) ListActiveEntityIdentitySurfaces(_ context.Context, sid string) ([]store.EntityIdentitySurface, error) {
	f.surfaceReads++
	out := []store.EntityIdentitySurface{}
	for _, item := range f.surfaces {
		if item.ChatSessionID == sid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *kgIdentityCountingStore) ListReviewedEntityIdentityLinks(_ context.Context, sid string) ([]store.EntityIdentityLink, error) {
	f.linkReads++
	out := []store.EntityIdentityLink{}
	for _, item := range f.links {
		if item.ChatSessionID == sid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *characterIdentityMergeFakeStore) ListActiveEntityIdentities(_ context.Context, sid string) ([]store.EntityIdentity, error) {
	out := []store.EntityIdentity{}
	for _, item := range f.identities {
		if item.ChatSessionID == sid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *characterIdentityMergeFakeStore) ListActiveEntityIdentitySurfaces(_ context.Context, sid string) ([]store.EntityIdentitySurface, error) {
	out := []store.EntityIdentitySurface{}
	for _, item := range f.surfaces {
		if item.ChatSessionID == sid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *characterIdentityMergeFakeStore) ListReviewedEntityIdentityLinks(_ context.Context, sid string) ([]store.EntityIdentityLink, error) {
	out := []store.EntityIdentityLink{}
	for _, item := range f.links {
		if item.ChatSessionID == sid && item.LinkKind == store.EntityIdentityLinkKindCanonicalEquivalence && item.LinkState == store.EntityIdentityLinkStateReviewed {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *characterIdentityMergeFakeStore) SaveEntityIdentityLink(_ context.Context, item *store.EntityIdentityLink) error {
	if err := f.linkErrors[item.SourceEntityID]; err != nil {
		return err
	}
	for index := range f.links {
		if f.links[index].ChatSessionID == item.ChatSessionID && f.links[index].SourceEntityID == item.SourceEntityID &&
			f.links[index].TargetEntityID == item.TargetEntityID && f.links[index].LinkKind == item.LinkKind {
			f.links[index] = *item
			return nil
		}
	}
	f.links = append(f.links, *item)
	return nil
}

func (f *characterIdentityMergeFakeStore) ListKGTriples(_ context.Context, sid string) ([]store.KGTriple, error) {
	if f.kgErr != nil {
		return nil, f.kgErr
	}
	return f.narrativeFakeStore.ListKGTriples(context.Background(), sid)
}

func (f *characterIdentityMergeFakeStore) ResolveReviewedCanonicalEntityID(_ context.Context, sid, sourceID string) (string, error) {
	current := sourceID
	seen := map[string]bool{}
	resolved := false
	for {
		if seen[current] {
			return "", store.ErrReviewedEntityIdentityCycle
		}
		seen[current] = true
		next := ""
		for _, link := range f.links {
			if link.ChatSessionID == sid && link.SourceEntityID == current && link.LinkKind == store.EntityIdentityLinkKindCanonicalEquivalence && link.LinkState == store.EntityIdentityLinkStateReviewed {
				if next != "" && next != link.TargetEntityID {
					return "", store.ErrReviewedEntityIdentityAmbiguous
				}
				next = link.TargetEntityID
			}
		}
		if next == "" {
			if resolved {
				return current, nil
			}
			return "", store.ErrNotFound
		}
		resolved = true
		current = next
	}
}

func (f *characterIdentityMergeFakeStore) ResolveUniqueActiveEntityIdentityBySurface(ctx context.Context, sid, normalized string) (store.ResolvedEntityIdentity, error) {
	ids := map[string]bool{}
	for _, surface := range f.surfaces {
		if surface.ChatSessionID != sid || surface.NormalizedSurface != normalized {
			continue
		}
		id := surface.StableEntityID
		if root, err := f.ResolveReviewedCanonicalEntityID(ctx, sid, id); err == nil {
			id = root
		}
		ids[id] = true
	}
	if len(ids) == 0 {
		return store.ResolvedEntityIdentity{}, store.ErrNotFound
	}
	if len(ids) != 1 {
		return store.ResolvedEntityIdentity{}, store.ErrReviewedEntityIdentityAmbiguous
	}
	for id := range ids {
		for _, identity := range f.identities {
			if identity.ChatSessionID == sid && identity.StableEntityID == id {
				return store.ResolvedEntityIdentity{StableEntityID: id, IdentityNamespace: identity.IdentityNamespace, EntityKind: identity.EntityKind, CanonicalLabel: identity.CanonicalLabel}, nil
			}
		}
	}
	return store.ResolvedEntityIdentity{}, store.ErrNotFound
}

func (f *characterIdentityMergeFakeStore) ListProtagonistEntityMemories(_ context.Context, filter store.ProtagonistEntityMemoryFilter) ([]store.ProtagonistEntityMemory, error) {
	out := []store.ProtagonistEntityMemory{}
	for _, item := range f.subjective {
		if item.SourceChatSessionID == filter.SourceChatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *characterIdentityMergeFakeStore) CreateProtagonistEntityMemory(_ context.Context, item *store.ProtagonistEntityMemory) (*store.ProtagonistEntityMemory, error) {
	f.subjective = append(f.subjective, *item)
	return item, nil
}

func abelIdentityMergeFixture() *characterIdentityMergeFakeStore {
	sid := "sess-abel"
	return &characterIdentityMergeFakeStore{
		narrativeFakeStore: &narrativeFakeStore{
			characterStates: []store.CharacterState{
				{ID: 26, ChatSessionID: sid, CharacterName: "아벨슈타인", StatusJSON: `{"weapon":"새 무기"}`, TurnIndex: 26},
				{ID: 28, ChatSessionID: sid, CharacterName: "아벨", StatusJSON: `{"action":"무기 확인"}`, TurnIndex: 28},
			},
			characterEvents: []store.CharacterEvent{
				{ID: 26, ChatSessionID: sid, CharacterName: "아벨슈타인", TurnIndex: 26, EventType: "weapon_equipped"},
				{ID: 28, ChatSessionID: sid, CharacterName: "아벨", TurnIndex: 28, EventType: "weapon_checked"},
			},
			kgTriples: []store.KGTriple{{ID: 26, ChatSessionID: sid, Subject: "아벨슈타인", Predicate: "equipped", Object: "새 무기", SourceTurn: 26}},
		},
		identities: []store.EntityIdentity{
			{StableEntityID: "abelstein-id", ChatSessionID: sid, IdentityNamespace: "session_npc", EntityKind: "character", CanonicalLabel: "아벨슈타인", LifecycleState: "active", ReviewState: "source_observed"},
			{StableEntityID: "abel-id", ChatSessionID: sid, IdentityNamespace: "session_npc", EntityKind: "character", CanonicalLabel: "아벨", LifecycleState: "active", ReviewState: "source_observed"},
		},
		surfaces: []store.EntityIdentitySurface{
			{StableEntityID: "abelstein-id", ChatSessionID: sid, SurfaceKind: "display_name", SurfaceText: "아벨슈타인", NormalizedSurface: comparableEntityKey("아벨슈타인"), Scope: store.EntityIdentitySurfaceScopeCurrent, ReviewState: "source_observed"},
			{StableEntityID: "abel-id", ChatSessionID: sid, SurfaceKind: "display_name", SurfaceText: "아벨", NormalizedSurface: comparableEntityKey("아벨"), Scope: store.EntityIdentitySurfaceScopeCurrent, ReviewState: "source_observed"},
		},
		subjective: []store.ProtagonistEntityMemory{{ID: 1, SourceChatSessionID: sid, OwnerEntityKey: "abel-id", OwnerEntityName: "아벨", MemoryText: "새 무기를 확인했다."}},
	}
}

func TestCharacterIdentityMergeAbelsteinAbelPreviewApplyAndUnmerge(t *testing.T) {
	fake := abelIdentityMergeFixture()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	before := srv.canonicalCharacterReadProjection(context.Background(), "sess-abel", fake.characterStates, fake.characterEvents)
	if len(before.States) != 2 {
		t.Fatalf("fixture must reproduce two cards before merge: %#v", before.States)
	}
	stateRows, eventRows, kgRows, subjectiveRows := len(fake.characterStates), len(fake.characterEvents), len(fake.kgTriples), len(fake.subjective)

	preview := identityMergeHTTP(t, mux, "/characters/sess-abel/identity-merge/preview", `{"target_entity_id":"abelstein-id","source_entity_ids":["abel-id"]}`)
	if preview["writes_performed"] != false {
		t.Fatalf("preview performed writes: %#v", preview)
	}
	impacts := mapFromAny(preview["impacts"])
	if intFromAny(mapFromAny(impacts["character_states"])["count"], 0) != 2 || intFromAny(mapFromAny(impacts["items_equipment"])["count"], 0) != 1 {
		t.Fatalf("preview counts=%#v", impacts)
	}

	apply := identityMergeHTTP(t, mux, "/characters/sess-abel/identity-merge", `{"target_entity_id":"abelstein-id","source_entity_ids":["abel-id"]}`)
	if intFromAny(apply["linked_count"], 0) != 1 || len(fake.links) != 1 {
		t.Fatalf("merge response=%#v links=%#v", apply, fake.links)
	}
	after := srv.canonicalCharacterReadProjection(context.Background(), "sess-abel", fake.characterStates, fake.characterEvents)
	if len(after.States) != 1 || after.States[0].CharacterName != "아벨슈타인" || after.States[0].TurnIndex != 28 {
		t.Fatalf("merged character projection=%#v", after.States)
	}
	aliases := after.Aliases[comparableEntityKey("아벨슈타인")]
	if len(aliases) != 1 || aliases[0] != "아벨" {
		t.Fatalf("aliases=%#v want 아벨", aliases)
	}
	if got := srv.canonicalCharacterName(context.Background(), "sess-abel", "아벨"); got != "아벨슈타인" {
		t.Fatalf("future alias resolution=%q", got)
	}
	canonicalKG := srv.canonicalizeCharacterKGTriplesForRead(context.Background(), "sess-abel", []store.KGTriple{{Subject: "아벨", Predicate: "checked", Object: "새 무기"}})
	if canonicalKG[0].Subject != "아벨슈타인" {
		t.Fatalf("KG read projection did not use representative identity: %#v", canonicalKG)
	}
	canonicalSubjective := srv.canonicalizeSubjectiveEntityMemoriesForRead(context.Background(), "sess-abel", fake.subjective)
	if len(canonicalSubjective) != 1 || canonicalSubjective[0].OwnerEntityName != "아벨슈타인" {
		t.Fatalf("subjective memory did not converge to representative: %#v", canonicalSubjective)
	}
	if len(fake.characterStates) != stateRows || len(fake.characterEvents) != eventRows || len(fake.kgTriples) != kgRows || len(fake.subjective) != subjectiveRows {
		t.Fatalf("merge rewrote existing rows")
	}

	unmerge := identityMergeHTTP(t, mux, "/characters/sess-abel/identity-merge/unmerge", `{"target_entity_id":"abelstein-id","source_entity_ids":["abel-id"]}`)
	if intFromAny(unmerge["unlinked_count"], 0) != 1 {
		t.Fatalf("unmerge response=%#v", unmerge)
	}
	restored := srv.canonicalCharacterReadProjection(context.Background(), "sess-abel", fake.characterStates, fake.characterEvents)
	if len(restored.States) != 2 {
		t.Fatalf("unmerge did not restore two cards: %#v", restored.States)
	}
}

func TestCharacterIdentityMergePreviewKeepsAvailableLanesWhenOneReadFails(t *testing.T) {
	fake := abelIdentityMergeFixture()
	fake.kgErr = errors.New("kg temporarily unavailable")
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	preview := identityMergeHTTP(t, mux, "/characters/sess-abel/identity-merge/preview", `{"target_entity_id":"abelstein-id","source_entity_ids":["abel-id"]}`)
	impacts := mapFromAny(preview["impacts"])
	if mapFromAny(impacts["relationship_knowledge"])["status"] != "unavailable" {
		t.Fatalf("KG failure not reported per lane: %#v", impacts)
	}
	if mapFromAny(impacts["character_states"])["status"] != "ready" || intFromAny(mapFromAny(impacts["character_states"])["count"], 0) != 2 {
		t.Fatalf("available character states were discarded: %#v", impacts)
	}
}

func TestExplorerKGCanonicalizesOnlySelectedPageWithBulkIdentityReads(t *testing.T) {
	const sid = "sess-kg-page"
	triples := make([]store.KGTriple, 1000)
	for index := range triples {
		triples[index] = store.KGTriple{
			ID: int64(index + 1), ChatSessionID: sid, Subject: "아벨", Predicate: "uses", Object: "새 무기", SourceTurn: index + 1,
		}
	}
	fake := &kgIdentityCountingStore{
		memoryFakeStore: &memoryFakeStore{kgTriples: triples},
		identities: []store.EntityIdentity{
			{StableEntityID: "abelstein-id", ChatSessionID: sid, EntityKind: "character", CanonicalLabel: "아벨슈타인"},
			{StableEntityID: "abel-id", ChatSessionID: sid, EntityKind: "character", CanonicalLabel: "아벨"},
		},
		surfaces: []store.EntityIdentitySurface{
			{StableEntityID: "abelstein-id", ChatSessionID: sid, SurfaceText: "아벨슈타인", NormalizedSurface: comparableEntityKey("아벨슈타인")},
			{StableEntityID: "abel-id", ChatSessionID: sid, SurfaceText: "아벨", NormalizedSurface: comparableEntityKey("아벨")},
		},
		links: []store.EntityIdentityLink{
			{ChatSessionID: sid, SourceEntityID: "abel-id", TargetEntityID: "abelstein-id", LinkKind: store.EntityIdentityLinkKindCanonicalEquivalence, LinkState: store.EntityIdentityLinkStateReviewed},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	request := func() map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/explorer/kg_triples?chat_session_id="+sid+"&limit=20&offset=0", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode KG page: %v", err)
		}
		return body
	}

	for attempt := 1; attempt <= 2; attempt++ {
		body := request()
		items := sliceFromAny(body["items"])
		if intFromAny(body["total"], 0) != 1000 || len(items) != 20 {
			t.Fatalf("attempt %d page total/items=%v/%d", attempt, body["total"], len(items))
		}
		for _, raw := range items {
			item := mapFromAny(raw)
			if item["subject"] != "아벨슈타인" {
				t.Fatalf("attempt %d subject=%v want representative name", attempt, item["subject"])
			}
		}
		if fake.identityReads != attempt || fake.surfaceReads != attempt || fake.linkReads != attempt {
			t.Fatalf("attempt %d catalog reads identities/surfaces/links=%d/%d/%d", attempt, fake.identityReads, fake.surfaceReads, fake.linkReads)
		}
	}
	if fake.memoryFakeStore.deletedKGID != 0 || len(fake.memoryFakeStore.updatedKG) != 0 {
		t.Fatal("KG read projection mutated source rows")
	}
}

func TestSubjectiveMemoryPageCanonicalizesWithOneCatalogRead(t *testing.T) {
	const sid = "sess-subjective-page"
	base := abelIdentityMergeFixture()
	base.links = []store.EntityIdentityLink{
		characterIdentityManualLink(sid, "abel-id", "abelstein-id", store.EntityIdentityLinkStateReviewed),
	}
	for index := range base.identities {
		base.identities[index].ChatSessionID = sid
	}
	for index := range base.surfaces {
		base.surfaces[index].ChatSessionID = sid
	}
	base.subjective = make([]store.ProtagonistEntityMemory, 1000)
	for index := range base.subjective {
		base.subjective[index] = store.ProtagonistEntityMemory{
			ID: int64(index + 1), SourceChatSessionID: sid,
			OwnerEntityKey: "abel-id", OwnerEntityName: "아벨",
			PersonaEntityKey: "abel-id", PersonaEntityName: "아벨",
			MemoryText: fmt.Sprintf("주관 기억 %d", index+1), SourceTurn: index + 1,
		}
	}
	fake := &entityExplorerIdentityCountingStore{characterIdentityMergeFakeStore: base}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/subjective-entity-memories?source_chat_session_id="+sid+"&limit=20", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("subjective page status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	items := sliceFromAny(response["items"])
	if len(items) != 20 || response["has_more"] != true {
		t.Fatalf("subjective page size/has_more=%d/%v", len(items), response["has_more"])
	}
	for _, raw := range items {
		item := mapFromAny(raw)
		if stringFromMap(item, "owner_entity_name") != "아벨슈타인" {
			t.Fatalf("subjective owner was not canonicalized: %#v", item)
		}
	}
	if fake.identityReads != 1 || fake.surfaceReads != 1 || fake.linkReads != 1 {
		t.Fatalf("catalog reads identities/surfaces/links=%d/%d/%d want one request-local catalog", fake.identityReads, fake.surfaceReads, fake.linkReads)
	}
	if fake.uniqueSurfaceReads != 0 || fake.reviewedRootReads != 0 {
		t.Fatalf("subjective page issued per-item resolver calls unique/root=%d/%d", fake.uniqueSurfaceReads, fake.reviewedRootReads)
	}
}

func TestItemsGetUsesOneAllKindCatalogWithoutPerItemResolvers(t *testing.T) {
	const sid = "sess-items-bulk"
	base := abelIdentityMergeFixture()
	base.links = []store.EntityIdentityLink{
		characterIdentityManualLink(sid, "abel-id", "abelstein-id", store.EntityIdentityLinkStateReviewed),
	}
	for index := range base.identities {
		base.identities[index].ChatSessionID = sid
	}
	for index := range base.surfaces {
		base.surfaces[index].ChatSessionID = sid
	}
	base.kgTriples = make([]store.KGTriple, 1000)
	for index := range base.kgTriples {
		label := fmt.Sprintf("도구-%04d", index+1)
		stableID := fmt.Sprintf("item-%04d", index+1)
		base.kgTriples[index] = store.KGTriple{
			ID: int64(index + 1), ChatSessionID: sid, Subject: "아벨",
			Predicate: "소유", Object: label, SourceTurn: index + 1,
		}
		base.identities = append(base.identities, store.EntityIdentity{
			StableEntityID: stableID, ChatSessionID: sid, IdentityNamespace: "session_item",
			EntityKind: "item", CanonicalLabel: label,
		})
		base.surfaces = append(base.surfaces, store.EntityIdentitySurface{
			StableEntityID: stableID, ChatSessionID: sid, SurfaceKind: "display_name",
			SurfaceText: label, NormalizedSurface: comparableEntityKey(label),
		})
	}
	fake := &entityExplorerIdentityCountingStore{characterIdentityMergeFakeStore: base}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	response := itemIdentityGetHTTP(t, mux, "/items/"+sid)
	items := sliceFromAny(response["items"])
	if len(items) == 0 {
		t.Fatal("items endpoint returned no page")
	}
	for _, raw := range items {
		item := mapFromAny(raw)
		if owner := strings.TrimSpace(stringFromMap(item, "owner")); owner != "" && owner != "아벨슈타인" {
			t.Fatalf("item owner=%q want canonical character owner", owner)
		}
	}
	if fake.identityReads != 1 || fake.surfaceReads != 1 || fake.linkReads != 1 {
		t.Fatalf("catalog reads identities/surfaces/links=%d/%d/%d want one all-kind catalog", fake.identityReads, fake.surfaceReads, fake.linkReads)
	}
	if fake.uniqueSurfaceReads != 0 || fake.reviewedRootReads != 0 {
		t.Fatalf("items endpoint issued per-item resolver calls unique/root=%d/%d", fake.uniqueSurfaceReads, fake.reviewedRootReads)
	}
}

func TestItemsGetDoesNotTreatCanonicalLabelAsImplicitSurface(t *testing.T) {
	const sid = "sess-item-label-not-surface"
	base := &characterIdentityMergeFakeStore{
		narrativeFakeStore: &narrativeFakeStore{kgTriples: []store.KGTriple{{
			ID: 1, ChatSessionID: sid, Subject: "강한얼", Predicate: "소유", Object: "전대", SourceTurn: 1,
		}}},
		identities: []store.EntityIdentity{{
			StableEntityID: "leather-pouch", ChatSessionID: sid, IdentityNamespace: "session_item",
			EntityKind: "item", CanonicalLabel: "가죽 전대",
		}},
	}
	fake := &entityExplorerIdentityCountingStore{characterIdentityMergeFakeStore: base}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	response := itemIdentityGetHTTP(t, mux, "/items/"+sid)
	items := sliceFromAny(response["items"])
	if len(items) != 2 {
		t.Fatalf("items=%#v want KG observation plus identity card", items)
	}
	byName := map[string]map[string]any{}
	for _, raw := range items {
		item := mapFromAny(raw)
		byName[stringFromMap(item, "item")] = item
	}
	if _, exists := byName["전대"]["stable_entity_id"]; exists {
		t.Fatalf("KG text resolved through an implicit canonical-label surface: %#v", byName["전대"])
	}
	if stringFromMap(byName["가죽 전대"], "stable_entity_id") != "leather-pouch" {
		t.Fatalf("identity-only card was lost: %#v", byName["가죽 전대"])
	}
}

func TestSubjectiveAliasRepairPlanUsesOneCatalogRead(t *testing.T) {
	const sid = "sess-subjective-repair-bulk"
	base := abelIdentityMergeFixture()
	base.links = []store.EntityIdentityLink{
		characterIdentityManualLink(sid, "abel-id", "abelstein-id", store.EntityIdentityLinkStateReviewed),
	}
	for index := range base.identities {
		base.identities[index].ChatSessionID = sid
	}
	for index := range base.surfaces {
		base.surfaces[index].ChatSessionID = sid
	}
	memories := make([]store.ProtagonistEntityMemory, 1000)
	for index := range memories {
		memories[index] = store.ProtagonistEntityMemory{
			ID: int64(index + 1), SourceChatSessionID: sid,
			OwnerEntityKey: "abel-id", OwnerEntityName: "아벨",
			PersonaEntityKey: "abel-id", PersonaEntityName: "아벨",
			MemoryText: fmt.Sprintf("주관 기억 %d", index+1), SourceTurn: index + 1,
		}
	}
	fake := &entityExplorerIdentityCountingStore{characterIdentityMergeFakeStore: base}
	srv := setupTestServer()
	srv.Store = fake
	plan := srv.buildSubjectiveEntityAliasRepairPlan(context.Background(), sid, memories)
	if plan.Scanned != len(memories) {
		t.Fatalf("repair plan scanned=%d want %d", plan.Scanned, len(memories))
	}
	if fake.identityReads != 1 || fake.surfaceReads != 1 || fake.linkReads != 1 {
		t.Fatalf("repair catalog reads identities/surfaces/links=%d/%d/%d want one", fake.identityReads, fake.surfaceReads, fake.linkReads)
	}
	if fake.uniqueSurfaceReads != 0 || fake.reviewedRootReads != 0 {
		t.Fatalf("alias repair issued per-item resolver calls unique/root=%d/%d", fake.uniqueSurfaceReads, fake.reviewedRootReads)
	}
}

func TestEntityIdentityCatalogKindFilterRemainsScoped(t *testing.T) {
	fake := itemIdentityMergeFixture()
	srv := setupTestServer()
	srv.Store = fake
	itemCatalog, err := srv.entityIdentityCatalogForSession(context.Background(), "sess-abel", "item")
	if err != nil {
		t.Fatal(err)
	}
	if len(itemCatalog.Identities) == 0 {
		t.Fatal("item catalog unexpectedly empty")
	}
	for _, identity := range itemCatalog.Identities {
		if identity.EntityKind != "item" {
			t.Fatalf("kind-scoped catalog leaked %q identity", identity.EntityKind)
		}
	}
	allCatalog, err := srv.entityIdentityCatalogForSession(context.Background(), "sess-abel", "")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, identity := range allCatalog.Identities {
		kinds[identity.EntityKind] = true
	}
	if !kinds["item"] || !kinds["character"] {
		t.Fatalf("all-kind catalog kinds=%#v", kinds)
	}
}

func TestKGReadProjectionKeepsAmbiguousSurfaceUnchanged(t *testing.T) {
	const sid = "sess-kg-ambiguous"
	fake := &kgIdentityCountingStore{
		memoryFakeStore: &memoryFakeStore{},
		identities: []store.EntityIdentity{
			{StableEntityID: "raven-a", ChatSessionID: sid, EntityKind: "character", CanonicalLabel: "레이븐 A"},
			{StableEntityID: "raven-b", ChatSessionID: sid, EntityKind: "character", CanonicalLabel: "레이븐 B"},
		},
		surfaces: []store.EntityIdentitySurface{
			{StableEntityID: "raven-a", ChatSessionID: sid, SurfaceText: "레이븐", NormalizedSurface: comparableEntityKey("레이븐")},
			{StableEntityID: "raven-b", ChatSessionID: sid, SurfaceText: "레이븐", NormalizedSurface: comparableEntityKey("레이븐")},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	input := []store.KGTriple{{ID: 1, ChatSessionID: sid, Subject: "레이븐", Predicate: "greets", Object: "방문자"}}
	output := srv.canonicalizeCharacterKGTriplesForRead(context.Background(), sid, input)
	if output[0].Subject != "레이븐" {
		t.Fatalf("ambiguous surface was guessed as %q", output[0].Subject)
	}
	if input[0].Subject != "레이븐" {
		t.Fatal("request-local projection mutated the source KG slice")
	}
}

func TestCharacterIdentityMergeKeepsSuccessfulLinksWhenAnotherLinkFails(t *testing.T) {
	fake := abelIdentityMergeFixture()
	fake.identities = append(fake.identities, store.EntityIdentity{StableEntityID: "bell-id", ChatSessionID: "sess-abel", IdentityNamespace: "session_npc", EntityKind: "character", CanonicalLabel: "벨", LifecycleState: "active", ReviewState: "source_observed"})
	fake.surfaces = append(fake.surfaces, store.EntityIdentitySurface{StableEntityID: "bell-id", ChatSessionID: "sess-abel", SurfaceText: "벨", NormalizedSurface: comparableEntityKey("벨"), Scope: store.EntityIdentitySurfaceScopeCurrent, ReviewState: "source_observed"})
	fake.linkErrors = map[string]error{"bell-id": errors.New("one link failed")}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	result := identityMergeHTTP(t, mux, "/characters/sess-abel/identity-merge", `{"target_entity_id":"abelstein-id","source_entity_ids":["abel-id","bell-id"]}`)
	if intFromAny(result["linked_count"], 0) != 1 || len(fake.links) != 1 || fake.links[0].SourceEntityID != "abel-id" {
		t.Fatalf("one failed link discarded a successful link: result=%#v links=%#v", result, fake.links)
	}
}

func TestCharacterIdentityMergeKeepsValidSourceWhenAnotherSourceIsUnavailable(t *testing.T) {
	fake := abelIdentityMergeFixture()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	result := identityMergeHTTP(t, mux, "/characters/sess-abel/identity-merge", `{"target_entity_id":"abelstein-id","source_entity_ids":["abel-id","stale-id"]}`)
	if intFromAny(result["linked_count"], 0) != 1 || len(fake.links) != 1 || fake.links[0].SourceEntityID != "abel-id" {
		t.Fatalf("unavailable source discarded a valid link: result=%#v links=%#v", result, fake.links)
	}
	results := sliceFromAny(result["results"])
	if len(results) != 2 || stringFromMap(mapFromAny(results[1]), "status") != "failed" {
		t.Fatalf("per-source result was not preserved: %#v", results)
	}
}

func TestCharacterIdentityUnmergeRevokesOnlyRequestedEdgeInChain(t *testing.T) {
	fake := abelIdentityMergeFixture()
	fake.identities = append(fake.identities, store.EntityIdentity{StableEntityID: "canonical-id", ChatSessionID: "sess-abel", IdentityNamespace: "session_npc", EntityKind: "character", CanonicalLabel: "아벨슈타인 경", LifecycleState: "active", ReviewState: "reviewed"})
	fake.links = []store.EntityIdentityLink{
		characterIdentityManualLink("sess-abel", "abel-id", "abelstein-id", store.EntityIdentityLinkStateReviewed),
		characterIdentityManualLink("sess-abel", "abelstein-id", "canonical-id", store.EntityIdentityLinkStateReviewed),
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	result := identityMergeHTTP(t, mux, "/characters/sess-abel/identity-merge/unmerge", `{"target_entity_id":"abelstein-id","source_entity_ids":["abel-id"]}`)
	if intFromAny(result["unlinked_count"], 0) != 1 {
		t.Fatalf("unmerge response=%#v", result)
	}
	if fake.links[0].LinkState != store.EntityIdentityLinkStateRevoked || fake.links[1].LinkState != store.EntityIdentityLinkStateReviewed {
		t.Fatalf("unmerge changed the wrong chain edge: %#v", fake.links)
	}
}

func TestCharacterIdentityMergeRejectsCrossSessionWithoutMutation(t *testing.T) {
	fake := abelIdentityMergeFixture()
	fake.identities = append(fake.identities, store.EntityIdentity{StableEntityID: "other-id", ChatSessionID: "other", EntityKind: "character", CanonicalLabel: "아벨"})
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	result := identityMergeHTTP(t, mux, "/characters/sess-abel/identity-merge", `{"target_entity_id":"abelstein-id","source_entity_ids":["other-id"]}`)
	if intFromAny(result["linked_count"], 0) != 0 || len(fake.links) != 0 {
		t.Fatalf("cross-session source was written: result=%#v links=%#v", result, fake.links)
	}
	results := sliceFromAny(result["results"])
	if len(results) != 1 || stringFromMap(mapFromAny(results[0]), "status") != "failed" {
		t.Fatalf("cross-session source was not reported separately: %#v", results)
	}
}

func TestCharactersGetShowsOnlyCharacterIdentityLinks(t *testing.T) {
	fake := abelIdentityMergeFixture()
	fake.identities = append(fake.identities,
		store.EntityIdentity{StableEntityID: "drink-id", ChatSessionID: "sess-abel", IdentityNamespace: "session_item", EntityKind: "item", CanonicalLabel: "맑은 이슬", LifecycleState: "active", ReviewState: "source_observed"},
		store.EntityIdentity{StableEntityID: "soju-id", ChatSessionID: "sess-abel", IdentityNamespace: "session_item", EntityKind: "item", CanonicalLabel: "중급 소주", LifecycleState: "active", ReviewState: "source_observed"},
	)
	fake.links = []store.EntityIdentityLink{
		characterIdentityManualLink("sess-abel", "abel-id", "abelstein-id", store.EntityIdentityLinkStateReviewed),
		characterIdentityManualLink("sess-abel", "drink-id", "soju-id", store.EntityIdentityLinkStateReviewed),
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/characters/sess-abel", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("characters status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	links := sliceFromAny(response["identity_links"])
	if len(links) != 1 || stringFromMap(mapFromAny(links[0]), "source_entity_id") != "abel-id" {
		t.Fatalf("non-character links leaked into character UI: %#v", links)
	}
}

func itemIdentityMergeFixture() *characterIdentityMergeFakeStore {
	fake := abelIdentityMergeFixture()
	fake.identities = append(fake.identities,
		store.EntityIdentity{StableEntityID: "clear-dew-id", ChatSessionID: "sess-abel", IdentityNamespace: "session_item", EntityKind: "item", CanonicalLabel: "맑은 이슬", LifecycleState: "active", ReviewState: "source_observed"},
		store.EntityIdentity{StableEntityID: "soju-id", ChatSessionID: "sess-abel", IdentityNamespace: "session_item", EntityKind: "item", CanonicalLabel: "중급 소주", LifecycleState: "active", ReviewState: "source_observed"},
	)
	fake.surfaces = append(fake.surfaces,
		store.EntityIdentitySurface{StableEntityID: "clear-dew-id", ChatSessionID: "sess-abel", SurfaceKind: "display_name", SurfaceText: "맑은 이슬", NormalizedSurface: comparableEntityKey("맑은 이슬"), Scope: store.EntityIdentitySurfaceScopeCurrent, ReviewState: "source_observed"},
		store.EntityIdentitySurface{StableEntityID: "soju-id", ChatSessionID: "sess-abel", SurfaceKind: "display_name", SurfaceText: "중급 소주", NormalizedSurface: comparableEntityKey("중급 소주"), Scope: store.EntityIdentitySurfaceScopeCurrent, ReviewState: "source_observed"},
	)
	fake.kgTriples = append(fake.kgTriples,
		store.KGTriple{ID: 27, ChatSessionID: "sess-abel", Subject: "아벨", Predicate: "소유", Object: "맑은 이슬", SourceTurn: 27},
	)
	return fake
}

func TestItemIdentityMergeStaysItemOnlyAndConvergesReadProjection(t *testing.T) {
	fake := itemIdentityMergeFixture()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	kgRows := len(fake.kgTriples)

	preview := identityMergeHTTP(t, mux, "/items/sess-abel/identity-merge/preview", `{"target_entity_id":"soju-id","source_entity_ids":["clear-dew-id"]}`)
	if preview["writes_performed"] != false {
		t.Fatalf("item preview performed writes: %#v", preview)
	}
	if intFromAny(mapFromAny(mapFromAny(preview["impacts"])["knowledge_relations"])["count"], 0) != 1 {
		t.Fatalf("item preview did not count related KG: %#v", preview)
	}

	apply := identityMergeHTTP(t, mux, "/items/sess-abel/identity-merge", `{"target_entity_id":"soju-id","source_entity_ids":["clear-dew-id","abel-id"]}`)
	if intFromAny(apply["linked_count"], 0) != 1 || len(fake.links) != 1 {
		t.Fatalf("valid item link was not saved independently: result=%#v links=%#v", apply, fake.links)
	}
	results := sliceFromAny(apply["results"])
	if len(results) != 2 || stringFromMap(mapFromAny(results[1]), "status") != "failed" {
		t.Fatalf("cross-kind source was not rejected per source: %#v", results)
	}
	if len(fake.kgTriples) != kgRows {
		t.Fatalf("item merge rewrote KG rows")
	}

	response := itemIdentityGetHTTP(t, mux, "/items/sess-abel")
	items := sliceFromAny(response["items"])
	matching := []map[string]any{}
	for _, raw := range items {
		item := mapFromAny(raw)
		if stringFromMap(item, "stable_entity_id") == "soju-id" {
			matching = append(matching, item)
		}
	}
	if len(matching) != 1 || stringFromMap(matching[0], "item") != "중급 소주" {
		t.Fatalf("merged items did not converge to representative: %#v", items)
	}
	aliases := sliceFromAny(matching[0]["aliases"])
	if len(aliases) == 0 || aliases[0] != "맑은 이슬" {
		t.Fatalf("item aliases=%#v want 맑은 이슬", aliases)
	}
	links := sliceFromAny(response["identity_links"])
	if len(links) != 1 || stringFromMap(mapFromAny(links[0]), "source_entity_id") != "clear-dew-id" {
		t.Fatalf("item link list=%#v", links)
	}

	unmerge := identityMergeHTTP(t, mux, "/items/sess-abel/identity-merge/unmerge", `{"target_entity_id":"soju-id","source_entity_ids":["clear-dew-id"]}`)
	if intFromAny(unmerge["unlinked_count"], 0) != 1 {
		t.Fatalf("item unmerge response=%#v", unmerge)
	}
}

func TestItemsGetDoesNotExposeCharacterIdentityLinks(t *testing.T) {
	fake := itemIdentityMergeFixture()
	fake.links = []store.EntityIdentityLink{
		characterIdentityManualLink("sess-abel", "abel-id", "abelstein-id", store.EntityIdentityLinkStateReviewed),
		itemIdentityManualLink("sess-abel", "clear-dew-id", "soju-id", store.EntityIdentityLinkStateReviewed),
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	response := itemIdentityGetHTTP(t, mux, "/items/sess-abel")
	links := sliceFromAny(response["identity_links"])
	if len(links) != 1 || stringFromMap(mapFromAny(links[0]), "source_entity_id") != "clear-dew-id" {
		t.Fatalf("character link leaked into item UI: %#v", links)
	}
}

func itemIdentityGetHTTP(t *testing.T, mux *http.ServeMux, path string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func identityMergeHTTP(t *testing.T, mux *http.ServeMux, path, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}
