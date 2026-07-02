package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type personaRouteFakeStore struct {
	store.Store
	nextID         int64
	capsules       []store.PersonaMemoryCapsule
	entries        map[int64][]store.PersonaMemoryEntry
	attachments    []store.PersonaCapsuleAttachment
	memories       []*store.Memory
	evidence       []*store.DirectEvidence
	kgTriples      []*store.KGTriple
	entityMemories []store.ProtagonistEntityMemory
}

func newPersonaRouteFakeStore() *personaRouteFakeStore {
	return &personaRouteFakeStore{
		Store:   store.NewNoopStore(),
		nextID:  10,
		entries: map[int64][]store.PersonaMemoryEntry{},
	}
}

func (f *personaRouteFakeStore) CreatePersonaMemoryCapsule(ctx context.Context, capsule *store.PersonaMemoryCapsule, entries []store.PersonaMemoryEntry) (*store.PersonaMemoryCapsule, error) {
	f.nextID++
	now := time.Date(2026, 6, 12, 1, 2, 3, 0, time.UTC)
	out := *capsule
	out.ID = f.nextID
	out.CreatedAt = now
	out.UpdatedAt = now
	f.capsules = append(f.capsules, out)
	for i := range entries {
		entries[i].ID = int64(i + 1)
		entries[i].CapsuleID = out.ID
		entries[i].CreatedAt = now
	}
	f.entries[out.ID] = append([]store.PersonaMemoryEntry(nil), entries...)
	return &out, nil
}

func (f *personaRouteFakeStore) ListPersonaMemoryCapsules(ctx context.Context, filter store.PersonaCapsuleFilter) ([]store.PersonaMemoryCapsule, error) {
	out := []store.PersonaMemoryCapsule{}
	for _, item := range f.capsules {
		if filter.PersonaKey != "" && item.PersonaKey != filter.PersonaKey {
			continue
		}
		if filter.SourceChatSessionID != "" && item.SourceChatSessionID != filter.SourceChatSessionID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *personaRouteFakeStore) GetPersonaMemoryCapsule(ctx context.Context, capsuleID int64) (*store.PersonaMemoryCapsule, []store.PersonaMemoryEntry, error) {
	for _, item := range f.capsules {
		if item.ID == capsuleID {
			copied := item
			return &copied, append([]store.PersonaMemoryEntry(nil), f.entries[capsuleID]...), nil
		}
	}
	return nil, nil, store.ErrNotFound
}

func (f *personaRouteFakeStore) DeletePersonaMemoryCapsule(ctx context.Context, capsuleID int64) error {
	for i, item := range f.capsules {
		if item.ID == capsuleID {
			f.capsules = append(f.capsules[:i], f.capsules[i+1:]...)
			delete(f.entries, capsuleID)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *personaRouteFakeStore) AttachPersonaMemoryCapsule(ctx context.Context, attachment *store.PersonaCapsuleAttachment) error {
	f.attachments = append(f.attachments, *attachment)
	return nil
}

func (f *personaRouteFakeStore) DetachPersonaMemoryCapsule(ctx context.Context, capsuleID int64, targetChatSessionID string) error {
	out := f.attachments[:0]
	for _, item := range f.attachments {
		if item.CapsuleID == capsuleID && item.TargetChatSessionID == targetChatSessionID {
			continue
		}
		out = append(out, item)
	}
	f.attachments = out
	return nil
}

func (f *personaRouteFakeStore) ListPersonaCapsuleAttachments(ctx context.Context, targetChatSessionID string) ([]store.PersonaCapsuleAttachment, error) {
	out := []store.PersonaCapsuleAttachment{}
	for _, item := range f.attachments {
		if item.TargetChatSessionID == targetChatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *personaRouteFakeStore) ListAttachedPersonaMemoryEntries(ctx context.Context, targetChatSessionID string, limit int) ([]store.PersonaMemoryEntry, error) {
	out := []store.PersonaMemoryEntry{}
	for _, att := range f.attachments {
		if att.TargetChatSessionID != targetChatSessionID || !att.Enabled {
			continue
		}
		out = append(out, f.entries[att.CapsuleID]...)
	}
	return out, nil
}

func (f *personaRouteFakeStore) CreateProtagonistEntityMemory(ctx context.Context, item *store.ProtagonistEntityMemory) (*store.ProtagonistEntityMemory, error) {
	f.nextID++
	now := time.Date(2026, 6, 12, 1, 2, 3, 0, time.UTC)
	out := *item
	if out.OwnerEntityKey == "" {
		out.OwnerEntityKey = out.PersonaEntityKey
	}
	if out.PersonaEntityKey == "" {
		out.PersonaEntityKey = out.OwnerEntityKey
	}
	if out.OwnerEntityName == "" {
		out.OwnerEntityName = out.PersonaEntityName
	}
	if out.PersonaEntityName == "" {
		out.PersonaEntityName = out.OwnerEntityName
	}
	if out.OwnerEntityRole == "" {
		out.OwnerEntityRole = "protagonist"
	}
	if out.OwnerVisibility == "" {
		out.OwnerVisibility = "player_known"
	}
	if out.TargetRevealPolicy == "" {
		out.TargetRevealPolicy = "requires_explicit_attachment"
	}
	out.ID = f.nextID
	out.CreatedAt = now
	out.UpdatedAt = now
	f.entityMemories = append(f.entityMemories, out)
	return &out, nil
}

func (f *personaRouteFakeStore) ListProtagonistEntityMemories(ctx context.Context, filter store.ProtagonistEntityMemoryFilter) ([]store.ProtagonistEntityMemory, error) {
	out := []store.ProtagonistEntityMemory{}
	limit := filter.Limit
	if limit <= 0 {
		limit = 80
	}
	for _, item := range f.entityMemories {
		ownerKey := item.OwnerEntityKey
		if ownerKey == "" {
			ownerKey = item.PersonaEntityKey
		}
		if filter.OwnerEntityKey != "" && ownerKey != filter.OwnerEntityKey {
			continue
		}
		if filter.OwnerEntityKey == "" && filter.PersonaEntityKey != "" && item.PersonaEntityKey != filter.PersonaEntityKey && ownerKey != filter.PersonaEntityKey {
			continue
		}
		if filter.OwnerEntityRole != "" && item.OwnerEntityRole != filter.OwnerEntityRole {
			continue
		}
		if filter.OwnerVisibility != "" && item.OwnerVisibility != filter.OwnerVisibility {
			continue
		}
		if filter.SourceChatSessionID != "" && item.SourceChatSessionID != filter.SourceChatSessionID {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *personaRouteFakeStore) UpdateProtagonistEntityMemoryOwner(ctx context.Context, update store.ProtagonistEntityMemoryOwnerUpdate) error {
	for i := range f.entityMemories {
		if f.entityMemories[i].ID != update.ID {
			continue
		}
		f.entityMemories[i].PersonaEntityKey = update.PersonaEntityKey
		f.entityMemories[i].PersonaEntityName = update.PersonaEntityName
		f.entityMemories[i].OwnerEntityKey = update.OwnerEntityKey
		f.entityMemories[i].OwnerEntityName = update.OwnerEntityName
		if update.OwnerEntityRole != "" {
			f.entityMemories[i].OwnerEntityRole = update.OwnerEntityRole
		}
		if update.OwnerVisibility != "" {
			f.entityMemories[i].OwnerVisibility = update.OwnerVisibility
		}
		f.entityMemories[i].TagsJSON = update.TagsJSON
		f.entityMemories[i].UpdatedAt = time.Date(2026, 6, 12, 2, 3, 4, 0, time.UTC)
		return nil
	}
	return store.ErrNotFound
}

func (f *personaRouteFakeStore) UpdateProtagonistEntityMemory(ctx context.Context, update store.ProtagonistEntityMemoryUpdate) error {
	for i := range f.entityMemories {
		if f.entityMemories[i].ID != update.ID {
			continue
		}
		f.entityMemories[i].PersonaEntityKey = update.PersonaEntityKey
		f.entityMemories[i].PersonaEntityName = update.PersonaEntityName
		f.entityMemories[i].OwnerEntityKey = update.OwnerEntityKey
		f.entityMemories[i].OwnerEntityName = update.OwnerEntityName
		f.entityMemories[i].OwnerEntityRole = update.OwnerEntityRole
		f.entityMemories[i].OwnerVisibility = update.OwnerVisibility
		f.entityMemories[i].SourceCharacterName = update.SourceCharacterName
		f.entityMemories[i].MemoryText = update.MemoryText
		f.entityMemories[i].EvidenceExcerpt = update.EvidenceExcerpt
		f.entityMemories[i].SecretGuard = update.SecretGuard
		f.entityMemories[i].Portability = update.Portability
		f.entityMemories[i].TargetRevealPolicy = update.TargetRevealPolicy
		f.entityMemories[i].TagsJSON = update.TagsJSON
		f.entityMemories[i].Importance10 = update.Importance10
		f.entityMemories[i].EmotionalWeight = update.EmotionalWeight
		f.entityMemories[i].UpdatedAt = time.Date(2026, 6, 12, 3, 4, 5, 0, time.UTC)
		return nil
	}
	return store.ErrNotFound
}

func (f *personaRouteFakeStore) DeleteProtagonistEntityMemory(ctx context.Context, id int64) error {
	for i := range f.entityMemories {
		if f.entityMemories[i].ID != id {
			continue
		}
		f.entityMemories = append(f.entityMemories[:i], f.entityMemories[i+1:]...)
		return nil
	}
	return store.ErrNotFound
}

func (f *personaRouteFakeStore) SaveMemory(ctx context.Context, m *store.Memory) error {
	f.memories = append(f.memories, m)
	return nil
}

func (f *personaRouteFakeStore) SaveEvidence(ctx context.Context, e *store.DirectEvidence) error {
	f.evidence = append(f.evidence, e)
	return nil
}

func (f *personaRouteFakeStore) SaveKGTriple(ctx context.Context, triple *store.KGTriple) error {
	f.kgTriples = append(f.kgTriples, triple)
	return nil
}

func TestPersonaCapsuleCreateRouteStoresSupportOnlyPolicy(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := []byte(`{
		"persona_key":"siwoo",
		"source_chat_session_id":"source-sess",
		"source_character_name":"Chloe",
		"title":"Loop 1",
		"mode":"loop",
		"summary":"Siwoo remembers the first loop.",
		"entries":[{"source_turn_index":4,"memory_text":"Siwoo remembers Chloe's warning.","importance_10":12,"tags":["loop","warning"],"evidence_excerpt":"Chloe warned him."}]
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/persona-capsules", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.capsules) != 1 || fake.capsules[0].PersonaKey != "siwoo" {
		t.Fatalf("capsule not stored: %+v", fake.capsules)
	}
	storedEntries := fake.entries[fake.capsules[0].ID]
	if len(storedEntries) != 1 {
		t.Fatalf("entries = %d, want 1", len(storedEntries))
	}
	if storedEntries[0].Importance10 != 10 {
		t.Fatalf("importance_10 = %v, want clamped 10", storedEntries[0].Importance10)
	}
	if storedEntries[0].SourceMemoryType != "" || storedEntries[0].SourceMemoryID != 0 {
		t.Fatalf("manual legacy capsule should remain snapshot-only, got ref %q/%d", storedEntries[0].SourceMemoryType, storedEntries[0].SourceMemoryID)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	policy, ok := resp["policy"].(map[string]any)
	if !ok || policy["authority"] != "support_only_persona_recollection" || policy["canonical_write"] != false {
		t.Fatalf("support-only policy missing: %+v", resp["policy"])
	}
}

func TestProtagonistEntityMemoryRoutesScopeByEntityAndSourceSession(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	createBody := []byte(`{
		"persona_entity_key":"이시우",
		"persona_entity_name":"이시우",
		"source_chat_session_id":"chloe-session",
		"source_character_name":"Chloe",
		"source_turn_index":3,
		"memory_text":"이시우는 클로에가 남긴 약속을 주관적으로 기억한다.",
		"evidence_excerpt":"더 이상 도망치지 않겠다",
		"secret_guard":true,
		"tags":["loop","chloe"],
		"importance_10":12,
		"emotional_weight":2
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/persona-entity-memories", bytes.NewReader(createBody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.entityMemories) != 1 {
		t.Fatalf("entity memories = %d, want 1", len(fake.entityMemories))
	}
	if fake.entityMemories[0].Importance10 != 10 || fake.entityMemories[0].EmotionalWeight != 1 {
		t.Fatalf("scores not clamped: %+v", fake.entityMemories[0])
	}
	if fake.entityMemories[0].PersonaEntityKey != "siwoo" || fake.entityMemories[0].PersonaEntityName != "이시우" || fake.entityMemories[0].SourceChatSessionID != "chloe-session" {
		t.Fatalf("entity/source scope not stored: %+v", fake.entityMemories[0])
	}

	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		PersonaEntityKey:    "이시우",
		PersonaEntityName:   "이시우",
		SourceChatSessionID: "saori-session",
		SourceCharacterName: "Saori",
		SourceTurn:          2,
		MemoryText:          "이시우는 사오리와의 긴장을 기억한다.",
		Importance10:        7,
	})
	if err != nil {
		t.Fatalf("seed saori entity memory: %v", err)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/persona-entity-memories?persona_entity_key=%EC%9D%B4%EC%8B%9C%EC%9A%B0&source_chat_session_id=chloe-session", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Count  int                             `json:"count"`
		Items  []store.ProtagonistEntityMemory `json:"items"`
		Policy map[string]any                  `json:"policy"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if resp.Count != 1 || len(resp.Items) != 1 {
		t.Fatalf("items = %+v, want one source-scoped protagonist memory", resp.Items)
	}
	if resp.Items[0].SourceChatSessionID != "chloe-session" || strings.Contains(resp.Items[0].MemoryText, "사오리") {
		t.Fatalf("source session filter leaked another source memory: %+v", resp.Items[0])
	}
	if resp.Policy["canonical_world_truth"] != false || resp.Policy["requires_explicit_attachment"] != true {
		t.Fatalf("policy must keep entity memories subjective and attachment-gated: %+v", resp.Policy)
	}
}

func TestSubjectiveEntityMemoryRoutesSupportNPCOwnerSeparation(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	createBody := []byte(`{
		"owner_entity_key":"chloe",
		"owner_entity_name":"Chloe",
		"owner_entity_role":"npc",
		"owner_visibility":"owner_private",
		"source_chat_session_id":"chloe-loop-source",
		"source_character_name":"Chloe",
		"source_turn_index":8,
		"memory_text":"Chloe privately remembers that Siwoo avoided the broken bridge before.",
		"evidence_excerpt":"Siwoo avoided the bridge.",
		"secret_guard":true,
		"portability":"npc_private_recollection",
		"target_reveal_policy":"owner_private_until_revealed",
		"tags":["loop","npc_private"],
		"importance_10":8,
		"emotional_weight":0.75
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subjective-entity-memories", bytes.NewReader(createBody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.entityMemories) != 1 {
		t.Fatalf("entity memories = %d, want 1", len(fake.entityMemories))
	}
	stored := fake.entityMemories[0]
	if stored.OwnerEntityKey != "chloe" || stored.PersonaEntityKey != "chloe" {
		t.Fatalf("owner/persona compatibility keys not stored: %+v", stored)
	}
	if stored.OwnerEntityRole != "npc" || stored.OwnerVisibility != "owner_private" {
		t.Fatalf("npc owner policy not stored: %+v", stored)
	}
	if stored.TargetRevealPolicy != "owner_private_until_revealed" {
		t.Fatalf("target reveal policy = %q", stored.TargetRevealPolicy)
	}

	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "saori",
		OwnerEntityName:     "Saori",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "chloe-loop-source",
		MemoryText:          "Saori privately remembers a different loop.",
		TargetRevealPolicy:  "owner_private_until_revealed",
	})
	if err != nil {
		t.Fatalf("seed saori entity memory: %v", err)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/subjective-entity-memories?owner_entity_key=chloe&owner_entity_role=npc&owner_visibility=owner_private&source_chat_session_id=chloe-loop-source", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Count  int                             `json:"count"`
		Items  []store.ProtagonistEntityMemory `json:"items"`
		Policy map[string]any                  `json:"policy"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if resp.Count != 1 || len(resp.Items) != 1 {
		t.Fatalf("items = %+v, want only Chloe-owned private memory", resp.Items)
	}
	if resp.Items[0].OwnerEntityKey != "chloe" || strings.Contains(resp.Items[0].MemoryText, "Saori") {
		t.Fatalf("owner filter leaked another entity memory: %+v", resp.Items[0])
	}
	if resp.Policy["surface"] != "subjective_entity_memory_bank" || resp.Policy["owner_separation_required"] != true {
		t.Fatalf("PMC-13 policy missing owner separation: %+v", resp.Policy)
	}
	if resp.Policy["npc_private_lane"] != "character_private_recollection" || resp.Policy["npc_private_default_player_view"] != false {
		t.Fatalf("PMC-13 policy missing NPC private lane guard: %+v", resp.Policy)
	}
}

func TestSubjectiveEntityMemoryCapsuleRouteCreatesNPCPrivateCapsule(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	chloe, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "chloe-source",
		SourceCharacterName: "Chloe",
		SourceTurn:          7,
		MemoryText:          "Chloe privately remembers that Siwoo avoided the broken bridge before.",
		EvidenceExcerpt:     "Siwoo avoided the bridge.",
		SecretGuard:         true,
		Portability:         "npc_private_recollection",
		TargetRevealPolicy:  "owner_private_until_revealed",
		TagsJSON:            `["bridge","loop"]`,
		Importance10:        8,
		EmotionalWeight:     0.7,
	})
	if err != nil {
		t.Fatalf("seed chloe subjective memory: %v", err)
	}
	_, err = fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "saori",
		OwnerEntityName:     "Saori",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "chloe-source",
		SourceTurn:          7,
		MemoryText:          "Saori privately remembers another route.",
		TargetRevealPolicy:  "owner_private_until_revealed",
		Importance10:        9,
	})
	if err != nil {
		t.Fatalf("seed saori subjective memory: %v", err)
	}

	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"owner_entity_key":"chloe",
		"owner_entity_name":"Chloe",
		"owner_entity_role":"npc",
		"owner_visibility":"owner_private",
		"source_chat_session_id":"chloe-source",
		"source_character_name":"Chloe",
		"title":"Chloe private route memory",
		"target_reveal_policy":"owner_private_until_revealed",
		"memory_ids":[%d]
	}`, chloe.ID)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subjective-entity-memories/capsule", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create subjective capsule status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.capsules) != 1 {
		t.Fatalf("capsules = %d, want 1", len(fake.capsules))
	}
	if fake.capsules[0].PersonaKey != "chloe" || fake.capsules[0].Mode != "npc_private_recollection" {
		t.Fatalf("capsule owner/mode mismatch: %+v", fake.capsules[0])
	}
	entries := fake.entries[fake.capsules[0].ID]
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want only selected Chloe memory", len(entries))
	}
	entry := entries[0]
	if entry.InjectionPolicy != "support_only_npc_private_recollection" || entry.Portability != "npc_private_recollection" {
		t.Fatalf("NPC private transport markers missing: %+v", entry)
	}
	if entry.SourceMemoryType != "subjective_entity_memory" || entry.SourceMemoryID != chloe.ID {
		t.Fatalf("subjective capsule entry must reference source memory %d, got %+v", chloe.ID, entry)
	}
	if strings.Contains(entry.MemoryText, "Saori") {
		t.Fatalf("capsule leaked another NPC memory: %+v", entry)
	}
	for _, needle := range []string{"npc_private", "owner_entity_key:chloe", "owner_entity_role:npc", "owner_visibility:owner_private", "target_reveal_policy:owner_private_until_revealed"} {
		if !strings.Contains(entry.TagsJSON, needle) {
			t.Fatalf("entry tags missing %q: %s", needle, entry.TagsJSON)
		}
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/persona-capsules/"+strconvFormatInt(fake.capsules[0].ID)+"/attach", strings.NewReader(`{"target_chat_session_id":"target-session","injection_mode":"npc_private_recollection","enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("attach status = %d body=%s", rec.Code, rec.Body.String())
	}
	attached, err := fake.ListAttachedPersonaMemoryEntries(context.Background(), "target-session", 12)
	if err != nil {
		t.Fatalf("list attached entries: %v", err)
	}
	if len(attached) != 1 || attached[0].InjectionPolicy != "support_only_npc_private_recollection" {
		t.Fatalf("attached entries mismatch: %+v", attached)
	}
}

func TestSubjectiveEntityMemoryEntityBundlesAndAutoCapsule(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	for _, item := range []*store.ProtagonistEntityMemory{
		{
			OwnerEntityKey:      "chloe",
			OwnerEntityName:     "Chloe",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-a",
			SourceTurn:          2,
			MemoryText:          "Chloe remembers Siwoo choosing the locked hallway.",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        7,
		},
		{
			OwnerEntityKey:      "chloe",
			OwnerEntityName:     "Chloe",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-a",
			SourceTurn:          5,
			MemoryText:          "Chloe remembers a different emotional ending than Siwoo does.",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        9,
		},
		{
			OwnerEntityKey:      "keulroe",
			OwnerEntityName:     "\ud074\ub85c\uc5d0",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-a",
			SourceTurn:          6,
			MemoryText:          "Chloe remembers the same scene under her Korean display name.",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        8,
		},
		{
			OwnerEntityKey:      "saori",
			OwnerEntityName:     "Saori",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-a",
			SourceTurn:          4,
			MemoryText:          "Saori remembers the same scene as unimportant.",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        6,
		},
		{
			OwnerEntityKey:      "chloe",
			OwnerEntityName:     "Chloe",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-b",
			SourceTurn:          1,
			MemoryText:          "Chloe remembers another session.",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        8,
		},
	} {
		if _, err := fake.CreateProtagonistEntityMemory(context.Background(), item); err != nil {
			t.Fatalf("seed subjective entity memory: %v", err)
		}
	}

	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/subjective-entity-memories/entities?source_chat_session_id=source-a", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("entity bundles status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Count  int              `json:"count"`
		Items  []map[string]any `json:"items"`
		Policy map[string]any   `json:"policy"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode entity bundles: %v", err)
	}
	if listResp.Count != 2 || len(listResp.Items) != 2 {
		t.Fatalf("entity bundle count = %d items=%+v, want Chloe and Saori only", listResp.Count, listResp.Items)
	}
	if listResp.Policy["memory_id_selection_required"] != false || listResp.Policy["user_selects"] != "entity_bundle" {
		t.Fatalf("PMC-16 entity bundle policy mismatch: %+v", listResp.Policy)
	}
	var foundChloe bool
	for _, item := range listResp.Items {
		if item["owner_entity_key"] == "chloe" {
			foundChloe = true
			if item["memory_count"] != float64(3) || item["default_prepare_lane"] != "character_private_recollection" {
				t.Fatalf("Chloe bundle mismatch: %+v", item)
			}
		}
	}
	if !foundChloe {
		t.Fatalf("Chloe bundle missing: %+v", listResp.Items)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subjective-entity-memories/capsule", strings.NewReader(`{
		"owner_entity_key":"chloe",
		"source_chat_session_id":"source-a",
		"target_reveal_policy":"owner_private_until_revealed"
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("auto capsule status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.capsules) != 1 {
		t.Fatalf("capsules = %d, want 1", len(fake.capsules))
	}
	if fake.capsules[0].SourceCharacterName != "Chloe" || !strings.Contains(fake.capsules[0].Title, "Chloe") {
		t.Fatalf("auto capsule did not infer entity name from selected memories: %+v", fake.capsules[0])
	}
	entries := fake.entries[fake.capsules[0].ID]
	if len(entries) != 3 {
		t.Fatalf("auto capsule entries = %d, want only Chloe source-a memories", len(entries))
	}
	for _, entry := range entries {
		if strings.Contains(entry.MemoryText, "Saori") || strings.Contains(entry.MemoryText, "another session") {
			t.Fatalf("auto capsule leaked another entity/source memory: %+v", entry)
		}
		if entry.InjectionPolicy != "support_only_npc_private_recollection" {
			t.Fatalf("auto capsule entry policy = %q", entry.InjectionPolicy)
		}
	}
}

func TestSubjectiveEntityMemoryAliasRepairDryRunAndApply(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	for _, item := range []*store.ProtagonistEntityMemory{
		{
			OwnerEntityKey:      "chloe",
			OwnerEntityName:     "Chloe",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-repair",
			SourceTurn:          2,
			MemoryText:          "Chloe remembers the first route.",
			TargetRevealPolicy:  "owner_private_until_revealed",
			TagsJSON:            `["npc_private"]`,
			Importance10:        7,
		},
		{
			OwnerEntityKey:      "keulroe",
			OwnerEntityName:     "\ud074\ub85c\uc5d0",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-repair",
			SourceTurn:          3,
			MemoryText:          "Chloe remembers the same route under a Korean display spelling.",
			TargetRevealPolicy:  "owner_private_until_revealed",
			TagsJSON:            `["npc_private"]`,
			Importance10:        8,
		},
		{
			OwnerEntityKey:      "saori",
			OwnerEntityName:     "Saori",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-repair",
			SourceTurn:          4,
			MemoryText:          "Saori remembers another route.",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        6,
		},
	} {
		if _, err := fake.CreateProtagonistEntityMemory(context.Background(), item); err != nil {
			t.Fatalf("seed subjective entity memory: %v", err)
		}
	}

	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subjective-entity-memories/alias-repair", strings.NewReader(`{"source_chat_session_id":"source-repair"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d body=%s", rec.Code, rec.Body.String())
	}
	var dryRun struct {
		DryRunOnly      bool `json:"dry_run_only"`
		RepairableCount int  `json:"repairable_count"`
		UpdatedCount    int  `json:"updated_count"`
		Groups          []struct {
			CanonicalOwnerKey string `json:"canonical_owner_key"`
			MemoryCount       int    `json:"memory_count"`
			RepairableCount   int    `json:"repairable_count"`
		} `json:"groups"`
		Policy map[string]any `json:"policy"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&dryRun); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if !dryRun.DryRunOnly || dryRun.RepairableCount != 1 || dryRun.UpdatedCount != 0 {
		t.Fatalf("dry-run repair counts mismatch: %+v", dryRun)
	}
	if len(dryRun.Groups) != 1 || dryRun.Groups[0].CanonicalOwnerKey != "chloe" || dryRun.Groups[0].MemoryCount != 2 || dryRun.Groups[0].RepairableCount != 1 {
		t.Fatalf("dry-run group mismatch: %+v", dryRun.Groups)
	}
	if dryRun.Policy["delete_duplicate_rows"] != false {
		t.Fatalf("repair must not delete rows by default: %+v", dryRun.Policy)
	}
	if fake.entityMemories[1].OwnerEntityKey != "keulroe" {
		t.Fatalf("dry-run mutated row: %+v", fake.entityMemories[1])
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subjective-entity-memories/alias-repair", strings.NewReader(`{"source_chat_session_id":"source-repair","apply":true}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d body=%s", rec.Code, rec.Body.String())
	}
	var applied struct {
		DryRunOnly      bool `json:"dry_run_only"`
		RepairableCount int  `json:"repairable_count"`
		UpdatedCount    int  `json:"updated_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&applied); err != nil {
		t.Fatalf("decode apply: %v", err)
	}
	if applied.DryRunOnly || applied.RepairableCount != 1 || applied.UpdatedCount != 1 {
		t.Fatalf("apply repair counts mismatch: %+v", applied)
	}
	repaired := fake.entityMemories[1]
	if repaired.OwnerEntityKey != "chloe" || repaired.PersonaEntityKey != "chloe" {
		t.Fatalf("alias row was not repaired to canonical key: %+v", repaired)
	}
	for _, needle := range []string{"entity_alias_repaired", "owner_entity_alias_key:keulroe", "raw_owner_entity_key:keulroe"} {
		if !strings.Contains(repaired.TagsJSON, needle) {
			t.Fatalf("repair tag %q missing from %s", needle, repaired.TagsJSON)
		}
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/subjective-entity-memories/entities?source_chat_session_id=source-repair", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("entity bundles status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Count int              `json:"count"`
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode entity bundles: %v", err)
	}
	if listResp.Count != 2 {
		t.Fatalf("entity bundle count = %d items=%+v, want Chloe and Saori only", listResp.Count, listResp.Items)
	}
	for _, item := range listResp.Items {
		if item["owner_entity_key"] == "chloe" && item["memory_count"] != float64(2) {
			t.Fatalf("repaired Chloe bundle should include both memories: %+v", item)
		}
	}
}

func TestSubjectiveEntityMemoryAliasRepairVexKoreanAlias(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	for _, item := range []*store.ProtagonistEntityMemory{
		{
			OwnerEntityKey:      "vex",
			OwnerEntityName:     "Vex",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-vex-repair",
			SourceTurn:          1,
			MemoryText:          "Vex remembers the first route.",
			Importance10:        7,
		},
		{
			OwnerEntityKey:      "bekseu",
			OwnerEntityName:     "\ubca1\uc2a4",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-vex-repair",
			SourceTurn:          2,
			MemoryText:          "Vex remembers the same route under a Korean display spelling.",
			Importance10:        8,
		},
	} {
		if _, err := fake.CreateProtagonistEntityMemory(context.Background(), item); err != nil {
			t.Fatalf("seed subjective entity memory: %v", err)
		}
	}

	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subjective-entity-memories/alias-repair", strings.NewReader(`{"source_chat_session_id":"source-vex-repair"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d body=%s", rec.Code, rec.Body.String())
	}
	var dryRun struct {
		RepairableCount int `json:"repairable_count"`
		Groups          []struct {
			CanonicalOwnerKey string `json:"canonical_owner_key"`
			MemoryCount       int    `json:"memory_count"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&dryRun); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if dryRun.RepairableCount != 1 || len(dryRun.Groups) != 1 || dryRun.Groups[0].CanonicalOwnerKey != "vex" || dryRun.Groups[0].MemoryCount != 2 {
		t.Fatalf("vex alias repair dry-run mismatch: %+v", dryRun)
	}
}

func TestSubjectiveEntityMemoryForceMergeRoleVisibility(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	for _, seed := range []store.ProtagonistEntityMemory{
		{
			PersonaEntityKey:    "chloe",
			PersonaEntityName:   "Chloe",
			OwnerEntityKey:      "chloe",
			OwnerEntityName:     "Chloe",
			OwnerEntityRole:     "protagonist",
			OwnerVisibility:     "player_known",
			SourceChatSessionID: "source-force",
			SourceTurn:          2,
			MemoryText:          "Siwoo remembers Chloe's promise as player-known context.",
			TagsJSON:            `["subjective_entity_memory"]`,
		},
		{
			PersonaEntityKey:    "chloe",
			PersonaEntityName:   "클로에",
			OwnerEntityKey:      "chloe",
			OwnerEntityName:     "클로에",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "source-force",
			SourceTurn:          3,
			MemoryText:          "Chloe privately remembers Siwoo's hesitation.",
			TagsJSON:            `["subjective_entity_memory"]`,
		},
	} {
		if _, err := fake.CreateProtagonistEntityMemory(context.Background(), &seed); err != nil {
			t.Fatalf("seed entity memory: %v", err)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subjective-entity-memories/alias-repair", bytes.NewReader([]byte(`{
		"source_chat_session_id":"source-force",
		"apply":false
	}`)))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("alias repair status = %d body=%s", rec.Code, rec.Body.String())
	}
	var repairResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&repairResp); err != nil {
		t.Fatalf("decode alias repair response: %v", err)
	}
	if repairResp["repairable_count"] != float64(0) {
		t.Fatalf("alias repair should not merge role/visibility split memories automatically: %+v", repairResp)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subjective-entity-memories/force-merge", bytes.NewReader([]byte(`{
		"source_chat_session_id":"source-force",
		"target_owner_key":"chloe",
		"target_owner_name":"Chloe",
		"target_owner_role":"npc",
		"target_visibility":"owner_private",
		"source_owner_keys":["chloe"]
	}`)))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("force merge status = %d body=%s", rec.Code, rec.Body.String())
	}
	var mergeResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&mergeResp); err != nil {
		t.Fatalf("decode force merge response: %v", err)
	}
	if mergeResp["updated_count"] != float64(2) {
		t.Fatalf("updated_count = %v response=%+v, want 2", mergeResp["updated_count"], mergeResp)
	}
	for _, memory := range fake.entityMemories {
		if memory.OwnerEntityKey != "chloe" || memory.PersonaEntityKey != "chloe" {
			t.Fatalf("force merge changed identity key unexpectedly: %+v", memory)
		}
		if memory.OwnerEntityRole != "npc" || memory.OwnerVisibility != "owner_private" {
			t.Fatalf("force merge did not normalize role/visibility: %+v", memory)
		}
		if !strings.Contains(memory.TagsJSON, "entity_force_merged") {
			t.Fatalf("force merge tag missing: %s", memory.TagsJSON)
		}
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/subjective-entity-memories/entities?source_chat_session_id=source-force", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("entity bundles status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Count int              `json:"count"`
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode entity bundles: %v", err)
	}
	if listResp.Count != 1 {
		t.Fatalf("entity bundle count = %d items=%+v, want one force-merged Chloe bundle", listResp.Count, listResp.Items)
	}
	if len(listResp.Items) != 1 || listResp.Items[0]["memory_count"] != float64(2) {
		t.Fatalf("force-merged bundle should contain both memories: %+v", listResp.Items)
	}
}

func TestSubjectiveEntityMemoryManualPatchAndDelete(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	created, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		PersonaEntityKey:    "siwoo",
		PersonaEntityName:   "Siwoo",
		OwnerEntityKey:      "siwoo",
		OwnerEntityName:     "Siwoo",
		OwnerEntityRole:     "protagonist",
		OwnerVisibility:     "player_known",
		SourceChatSessionID: "manual-source",
		SourceCharacterName: "Chloe",
		SourceTurn:          4,
		MemoryText:          "Old subjective memory.",
		EvidenceExcerpt:     "old evidence",
		SecretGuard:         false,
		Portability:         "portable_persona_recollection",
		TargetRevealPolicy:  "requires_explicit_attachment",
		TagsJSON:            `["subjective_entity_memory"]`,
		Importance10:        4,
		EmotionalWeight:     0.2,
	})
	if err != nil {
		t.Fatalf("seed entity memory: %v", err)
	}
	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	patchBody := []byte(`{
		"persona_entity_key":"chloe",
		"persona_entity_name":"Chloe",
		"owner_entity_key":"chloe",
		"owner_entity_name":"Chloe",
		"owner_entity_role":"npc",
		"owner_visibility":"owner_private",
		"source_chat_session_id":"manual-source",
		"source_character_name":"Chloe",
		"memory_text":"Chloe privately remembers the edited route.",
		"evidence_excerpt":"edited evidence",
		"secret_guard":true,
		"portability":"npc_private_recollection",
		"target_reveal_policy":"owner_private_until_revealed",
		"tags_json":"[\"manual_edit\"]",
		"importance_10":9,
		"emotional_weight":0.8
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/subjective-entity-memories/"+strconvFormatInt(created.ID), bytes.NewReader(patchBody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", rec.Code, rec.Body.String())
	}
	updated := fake.entityMemories[0]
	if updated.OwnerEntityKey != "chloe" || updated.OwnerEntityRole != "npc" || updated.OwnerVisibility != "owner_private" {
		t.Fatalf("owner fields not patched: %+v", updated)
	}
	if updated.MemoryText != "Chloe privately remembers the edited route." || updated.TargetRevealPolicy != "owner_private_until_revealed" || !updated.SecretGuard {
		t.Fatalf("memory fields not patched: %+v", updated)
	}
	if updated.Importance10 != 9 || updated.EmotionalWeight != 0.8 {
		t.Fatalf("weights not patched: %+v", updated)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/subjective-entity-memories/"+strconvFormatInt(created.ID), nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.entityMemories) != 0 {
		t.Fatalf("delete left entity memories: %+v", fake.entityMemories)
	}
}

func TestPersonaCapsuleListRouteScopesBySourceSession(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	if _, err := fake.CreatePersonaMemoryCapsule(context.Background(), &store.PersonaMemoryCapsule{
		PersonaKey:          "siwoo",
		SourceChatSessionID: "chloe-session",
		SourceCharacterName: "Chloe",
		Title:               "Chloe capsule",
	}, []store.PersonaMemoryEntry{{MemoryText: "Siwoo remembers Chloe's promise.", Importance10: 8}}); err != nil {
		t.Fatalf("seed chloe capsule: %v", err)
	}
	if _, err := fake.CreatePersonaMemoryCapsule(context.Background(), &store.PersonaMemoryCapsule{
		PersonaKey:          "siwoo",
		SourceChatSessionID: "saori-session",
		SourceCharacterName: "Saori",
		Title:               "Saori capsule",
	}, []store.PersonaMemoryEntry{{MemoryText: "Siwoo remembers Saori's touch.", Importance10: 8}}); err != nil {
		t.Fatalf("seed saori capsule: %v", err)
	}

	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/persona-capsules?persona_key=siwoo&source_chat_session_id=chloe-session", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Count int                          `json:"count"`
		Items []store.PersonaMemoryCapsule `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 || len(resp.Items) != 1 {
		t.Fatalf("items = %+v, want only one source-scoped capsule", resp.Items)
	}
	if resp.Items[0].SourceChatSessionID != "chloe-session" || strings.Contains(resp.Items[0].Title, "Saori") {
		t.Fatalf("source session filter leaked another session capsule: %+v", resp.Items[0])
	}
}

func TestPersonaCapsuleAttachAndReadEntriesRoutes(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	created, err := fake.CreatePersonaMemoryCapsule(context.Background(), &store.PersonaMemoryCapsule{
		PersonaKey:          "siwoo",
		SourceChatSessionID: "source",
		Title:               "Transfer",
	}, []store.PersonaMemoryEntry{{MemoryText: "He remembers the other world.", Importance10: 8}})
	if err != nil {
		t.Fatalf("seed capsule: %v", err)
	}
	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/persona-capsules/"+strconvFormatInt(created.ID)+"/attach", bytes.NewReader([]byte(`{"target_chat_session_id":"target","injection_mode":"isekai_carryover"}`)))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("attach status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/persona-capsules/attached-entries?target_chat_session_id=target", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("attached entries status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode attached entries: %v", err)
	}
	if resp["count"] != float64(1) {
		t.Fatalf("count = %v, want 1", resp["count"])
	}
}

func TestPersonaCapsuleLiveSmokeCreateAttachPrepareTurnSupportOnly(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	createBody := []byte(`{
		"persona_key":"siwoo",
		"source_chat_session_id":"loop-source",
		"source_character_name":"Chloe",
		"title":"Loop memory capsule",
		"mode":"full_loop_memory",
		"summary":"Siwoo carries a subjective loop recollection.",
		"entries":[{
			"source_turn_index":9,
			"memory_text":"Siwoo remembers Chloe leaving the silver locket inside the locked desk during the previous loop.",
			"importance_10":8,
			"emotional_weight":0.75,
			"portability":"cross_session",
			"tags":["loop","locket"],
			"evidence_excerpt":"silver locket inside the locked desk",
			"injection_policy":"support_only_persona_recollection"
		}]
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/persona-capsules", bytes.NewReader(createBody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var createResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	capsule, ok := createResp["capsule"].(map[string]any)
	if !ok {
		t.Fatalf("create response missing capsule: %+v", createResp)
	}
	capsuleID := int64(capsule["id"].(float64))
	if capsuleID == 0 {
		t.Fatalf("capsule id not assigned: %+v", capsule)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/persona-capsules/"+strconvFormatInt(capsuleID)+"/attach", bytes.NewReader([]byte(`{"target_chat_session_id":"isekai-target","injection_mode":"full_loop_memory","enabled":true}`)))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("attach status = %d body=%s", rec.Code, rec.Body.String())
	}

	prepareBody := `{"chat_session_id":"isekai-target","turn_index":1,"raw_user_input":"The desk looks familiar.","settings":{"max_injection_chars":1400,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":3}}`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(prepareBody))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare-turn status = %d body=%s", rec.Code, rec.Body.String())
	}
	var prepareResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&prepareResp); err != nil {
		t.Fatalf("decode prepare response: %v", err)
	}
	injectionText, _ := prepareResp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Persona Recollection]") || !strings.Contains(injectionText, "Protected hint") || !strings.Contains(injectionText, "protected private knowledge is present") {
		t.Fatalf("prepare-turn did not inject persona recollection: %q", injectionText)
	}
	if strings.Contains(injectionText, "silver locket") {
		t.Fatalf("prepare-turn leaked protected persona recollection content: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Secret Guard") || !strings.Contains(injectionText, "protagonist-only private intuition") || !strings.Contains(injectionText, "Never reveal its origin") {
		t.Fatalf("prepare-turn did not inject persona secret guard: %q", injectionText)
	}
	inputContextText, _ := prepareResp["input_context_text"].(string)
	if !strings.Contains(inputContextText, "[Persona Recollection]") || !strings.Contains(inputContextText, "support-only private recollection") {
		t.Fatalf("input_context_text missing support-only persona lane: %q", inputContextText)
	}
	ip, ok := prepareResp["injection_pack"].(map[string]any)
	if !ok || ip["persona_recollection_active"] != true {
		t.Fatalf("injection_pack persona lane inactive: %+v", prepareResp["injection_pack"])
	}
	policy, ok := ip["persona_recollection_policy"].(map[string]any)
	if !ok || policy["truth_authority"] != false || policy["canonical_write"] != false || policy["requires_current_session_confirmation"] != true {
		t.Fatalf("persona recollection policy must remain support-only: %+v", policy)
	}
	if policy["secret_guard_active"] != true {
		t.Fatalf("persona recollection policy missing secret guard: %+v", policy)
	}
	secretGuard, ok := policy["secret_guard"].(map[string]any)
	if !ok || secretGuard["active"] != true {
		t.Fatalf("persona recollection policy secret_guard mismatch: %+v", policy["secret_guard"])
	}
	surface, ok := prepareResp["persona_recollection"].(map[string]any)
	if !ok || surface["status"] != "ready" || surface["would_write"] != false || surface["would_call_llm"] != false {
		t.Fatalf("persona recollection surface mismatch: %+v", prepareResp["persona_recollection"])
	}
	if surface["secret_guard_active"] != true {
		t.Fatalf("persona recollection surface missing active secret guard: %+v", surface)
	}
	if len(fake.memories) != 0 || len(fake.evidence) != 0 || len(fake.kgTriples) != 0 {
		t.Fatalf("persona capsule smoke must not create canonical rows before target confirmation: mem=%d evi=%d kg=%d", len(fake.memories), len(fake.evidence), len(fake.kgTriples))
	}
}

func TestPersonaCapsuleKoreanLoopSecretGuardPrepareTurn(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	srv := &Server{Cfg: config.Config{}, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	createBody := []byte(`{
		"persona_key":"siwoo",
		"source_chat_session_id":"old-loop-session",
		"source_character_name":"Chloe",
		"title":"Korean loop secret",
		"mode":"full_loop_memory",
		"summary":"시우가 이전 루프의 사적 기억을 지님.",
		"entries":[{
			"source_turn_index":3,
			"memory_text":"시우는 이전 루프에서 클로에가 은색 로켓을 잠긴 책상 안에 숨겼다는 것을 기억한다.",
			"importance_10":9,
			"emotional_weight":0.8,
			"portability":"cross_session",
			"tags":["회귀","루프","비밀"],
			"evidence_excerpt":"은색 로켓을 잠긴 책상 안에 숨겼다",
			"injection_policy":"support_only_persona_recollection"
		}]
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/persona-capsules", bytes.NewReader(createBody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var createResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	capsule := createResp["capsule"].(map[string]any)
	capsuleID := int64(capsule["id"].(float64))

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/persona-capsules/"+strconvFormatInt(capsuleID)+"/attach", bytes.NewReader([]byte(`{"target_chat_session_id":"new-loop-session","injection_mode":"full_loop_memory","enabled":true}`)))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("attach status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(`{"chat_session_id":"new-loop-session","turn_index":1,"raw_user_input":"책상이 이상하게 낯익다.","settings":{"max_injection_chars":1600,"max_input_context_chars":1000,"injection_enabled":true,"input_context_enabled":true,"top_k":3}}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare-turn status = %d body=%s", rec.Code, rec.Body.String())
	}
	var prepareResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&prepareResp); err != nil {
		t.Fatalf("decode prepare response: %v", err)
	}
	injectionText, _ := prepareResp["injection_text"].(string)
	for _, needle := range []string{
		"[Persona Recollection]",
		"support-only private recollection",
		"Secret Guard",
		"protagonist-only private intuition",
		"Never reveal its origin",
		"hesitation, instinct, or careful choice",
		"Protected hint",
		"protected private knowledge is present",
		"do not reveal content without current evidence",
	} {
		if !strings.Contains(injectionText, needle) {
			t.Fatalf("prepare-turn persona injection missing %q: %q", needle, injectionText)
		}
	}
	for _, leaked := range []string{"이전 루프", "회귀자", "회귀", "루프"} {
		if strings.Contains(injectionText, leaked) {
			t.Fatalf("prepare-turn persona injection leaked explicit secret term %q: %q", leaked, injectionText)
		}
	}

	surface, ok := prepareResp["persona_recollection"].(map[string]any)
	if !ok || surface["status"] != "ready" || surface["secret_guard_active"] != true {
		t.Fatalf("persona recollection surface missing Korean loop secret guard: %+v", prepareResp["persona_recollection"])
	}
	if surface["would_write"] != false || surface["would_call_llm"] != false {
		t.Fatalf("persona recollection must stay read-only support lane: %+v", surface)
	}
	policy, ok := surface["policy"].(map[string]any)
	if !ok || policy["truth_authority"] != false || policy["canonical_write"] != false || policy["current_world_fact"] != false {
		t.Fatalf("persona recollection policy promoted secret memory: %+v", surface["policy"])
	}
	secretGuard, ok := surface["secret_guard"].(map[string]any)
	if !ok || secretGuard["active"] != true {
		t.Fatalf("persona secret guard missing: %+v", surface["secret_guard"])
	}
	blockedReveals := toStringSetFromAny(secretGuard["blocked_reveals"])
	for _, blocked := range []string{"dialogue_announces_regressor_or_reincarnation", "canonical_world_fact_from_capsule_only"} {
		if !blockedReveals[blocked] {
			t.Fatalf("persona secret guard missing blocked reveal %q: %+v", blocked, secretGuard["blocked_reveals"])
		}
	}
	if len(fake.memories) != 0 || len(fake.evidence) != 0 || len(fake.kgTriples) != 0 {
		t.Fatalf("persona capsule prepare-turn must not write canonical artifacts: mem=%d evi=%d kg=%d", len(fake.memories), len(fake.evidence), len(fake.kgTriples))
	}
}

func TestPersonaCapsuleRoutesRequireOptionalStore(t *testing.T) {
	srv := &Server{Cfg: config.Config{}, Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/persona-capsules", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rec.Code)
	}
}

func strconvFormatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func toStringSetFromAny(v any) map[string]bool {
	out := map[string]bool{}
	switch items := v.(type) {
	case []any:
		for _, item := range items {
			out[strings.TrimSpace(fmt.Sprint(item))] = true
		}
	case []string:
		for _, item := range items {
			out[strings.TrimSpace(item)] = true
		}
	}
	return out
}
