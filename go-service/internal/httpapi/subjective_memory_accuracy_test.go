package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSubjectiveEntityMemoryDuplicateReasonIsConservative(t *testing.T) {
	const evidence = "Chloe watched Siwoo hide the blue key beneath the broken stair."
	fake := &turnRecordingStore{
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				OwnerEntityKey:      "chloe",
				SourceChatSessionID: "sess-subjective-dedupe",
				SourceTurn:          10,
				MemoryText:          "Chloe remembers that Siwoo hid the blue key beneath the broken stair.",
				EvidenceExcerpt:     evidence,
			},
		},
	}

	tests := []struct {
		name       string
		turn       int
		memoryText string
		evidence   string
		want       string
	}{
		{
			name:       "same text on a later turn",
			turn:       14,
			memoryText: "  CHLOE remembers that Siwoo hid the blue key beneath the broken stair.  ",
			want:       "duplicate_owner_memory_text",
		},
		{
			name:       "nearby paraphrase with identical grounded evidence",
			turn:       12,
			memoryText: "Chloe privately concludes that Siwoo concealed the key near the stairs.",
			evidence:   evidence,
			want:       "duplicate_nearby_owner_evidence",
		},
		{
			name:       "distant event with reused evidence is retained",
			turn:       20,
			memoryText: "Chloe now interprets the old key incident differently.",
			evidence:   evidence,
			want:       "",
		},
		{
			name:       "different memory without evidence is retained",
			turn:       11,
			memoryText: "Chloe worries that the western door will not hold.",
			want:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := subjectiveEntityMemoryDuplicateReason(context.Background(), fake, "sess-subjective-dedupe", "chloe", tt.turn, tt.memoryText, tt.evidence)
			if got != tt.want {
				t.Fatalf("duplicate reason = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrepareTurnCharacterPrivateRecollectionUsesOneMemoryPerCurrentOwner(t *testing.T) {
	memories := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "niv", OwnerEntityName: "Niv", OwnerEntityRole: "npc", MemoryText: "Niv privately remembers the garden promise."},
		{ID: 4, OwnerEntityKey: "niv", OwnerEntityName: "Niv", OwnerEntityRole: "npc", SourceTurn: 2, MemoryText: "Niv has an older duplicate owner memory."},
		{ID: 2, OwnerEntityKey: "ingrid", OwnerEntityName: "Ingrid", OwnerEntityRole: "npc", MemoryText: "Ingrid privately doubts the garden promise."},
		{ID: 3, OwnerEntityKey: "ashley", OwnerEntityName: "Ashley", OwnerEntityRole: "npc", MemoryText: "Ashley privately fears being overheard."},
	}

	trace := filterPrepareTurnEntityRecollections(
		"Niv and Ingrid discuss Ashley in the garden.",
		nil,
		nil,
		nil,
		nil,
		nil,
		&memories,
	)
	if len(memories) != 3 {
		t.Fatalf("selected private recollections = %d, want one for each of three current owners: %#v", len(memories), memories)
	}
	if memories[0].OwnerEntityKey != "niv" || memories[1].OwnerEntityKey != "ingrid" || memories[2].OwnerEntityKey != "ashley" {
		t.Fatalf("private recollection ordering changed unexpectedly: %#v", memories)
	}
	if trace["character_private_total_cap"] != "final_subjective_relationship_char_budget" {
		t.Fatalf("character_private_total_cap = %#v, want final char budget ownership", trace["character_private_total_cap"])
	}
	dropped, ok := trace["dropped"].([]map[string]any)
	if !ok || len(dropped) != 1 {
		t.Fatalf("dropped trace = %#v, want one repeated owner memory", trace["dropped"])
	}
	if dropped[0]["owner_entity_key"] != "niv" || dropped[0]["reason"] != "owner_repetition_capped" {
		t.Fatalf("unexpected owner-cap trace: %#v", dropped[0])
	}
}

func TestPersonaPrivateClassificationKeepsProtagonistAndNPCDomainsSeparate(t *testing.T) {
	protagonist := store.PersonaMemoryEntry{
		Portability: "npc_private_recollection",
		TagsJSON:    `["owner_entity_role:protagonist","character_private_recollection"]`,
	}
	npc := store.PersonaMemoryEntry{
		Portability: "npc_private_recollection",
		TagsJSON:    `["owner_entity_role:npc","character_private_recollection"]`,
	}
	ambiguous := store.PersonaMemoryEntry{
		Portability: "npc_private_recollection",
		TagsJSON:    `["character_private_recollection"]`,
	}
	if personaMemoryEntryIsCharacterPrivate(protagonist) {
		t.Fatal("explicit protagonist memory entered the NPC-private domain")
	}
	if !personaMemoryEntryIsCharacterPrivate(npc) {
		t.Fatal("explicit NPC-private memory was not classified into its domain")
	}
	if personaMemoryEntryIsCharacterPrivate(ambiguous) {
		t.Fatal("role-unobserved memory entered the NPC-private domain")
	}
}

func TestPrepareTurnCharacterPrivateRecollectionRejectsNonNPCOwner(t *testing.T) {
	memories := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "mira", OwnerEntityName: "Mira", OwnerEntityRole: "protagonist", OwnerVisibility: "player_known", MemoryText: "Mira remembers the garden."},
		{ID: 2, OwnerEntityKey: "juno", OwnerEntityName: "Juno", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", MemoryText: "Juno privately remembers the garden."},
		{ID: 3, OwnerEntityKey: "mira_legacy", OwnerEntityName: "Mira", OwnerEntityRole: "protagonist", OwnerVisibility: "player_known", TagsJSON: `["owner_entity_role:npc"]`, MemoryText: "Stored role remains the single read-time authority."},
	}
	trace := filterPrepareTurnEntityRecollections(
		"Mira meets Juno in the garden.",
		nil, nil, nil, nil, nil, &memories,
	)
	if len(memories) != 1 || memories[0].OwnerEntityKey != "juno" {
		t.Fatalf("NPC-private selection = %#v, want only Juno; trace=%#v", memories, trace)
	}
}

func TestRisuPersonaObservationOwnsSubjectiveMemoryRoles(t *testing.T) {
	extraction := map[string]any{
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_key":  "entity-17",
				"owner_entity_name": "Mira",
				"owner_entity_role": "npc",
				"owner_visibility":  "owner_private",
				"portability":       "npc_private_recollection",
				"tags":              []string{"owner_entity_role:npc", "secret_guard"},
			},
			map[string]any{
				"owner_entity_key":  "juno",
				"owner_entity_name": "Juno",
				"owner_entity_role": "protagonist",
				"owner_visibility":  "player_known",
				"tags":              []string{"owner_entity_role:protagonist"},
			},
		},
	}
	meta := map[string]any{
		"risu_persona_observation": map[string]any{
			"contract_version":  "risu_persona_observation.v1",
			"observation_state": "observed",
			"source":            "database.selectedPersona",
			"persona_name":      "Mira",
		},
	}

	resolved, trace := applyRisuPersonaSubjectiveMemoryRoles(extraction, meta)
	items := sliceFromAny(resolved["subjective_entity_memories"])
	mira := mapFromAny(items[0])
	juno := mapFromAny(items[1])
	if mira["owner_entity_role"] != "protagonist" || mira["portability"] != "portable_subjective_entity_recollection" {
		t.Fatalf("persona memory role = %#v, want protagonist persona lane", mira)
	}
	if juno["owner_entity_role"] != "npc" || juno["owner_visibility"] != "owner_private" || juno["portability"] != "npc_private_recollection" {
		t.Fatalf("non-persona memory role = %#v, want NPC-private lane", juno)
	}
	if trace["protagonist_count"] != 1 || trace["npc_count"] != 1 {
		t.Fatalf("role trace = %#v", trace)
	}
}

func TestRisuPersonaObservationRemovesMisclassifiedStoredPersonaFromNPCLane(t *testing.T) {
	memories := []store.ProtagonistEntityMemory{
		{ID: 17, OwnerEntityKey: "entity-17", OwnerEntityName: "Mira", OwnerEntityRole: "npc", MemoryText: "older misclassified row"},
		{ID: 18, OwnerEntityKey: "mira", OwnerEntityName: "Mira", OwnerEntityRole: "npc", MemoryText: "duplicate misclassified row"},
		{ID: 19, OwnerEntityKey: "juno", OwnerEntityName: "Juno", OwnerEntityRole: "protagonist", MemoryText: "real NPC memory"},
	}
	meta := map[string]any{
		"risu_persona_observation": map[string]any{
			"contract_version":  "risu_persona_observation.v1",
			"observation_state": "observed",
			"source":            "chat.bindedPersona",
			"persona_name":      "Mira",
		},
	}
	filtered, trace := excludeRisuPersonaFromStoredNPCMemories(memories, meta)
	if len(filtered) != 1 || filtered[0].ID != 19 || filtered[0].OwnerEntityRole != "npc" {
		t.Fatalf("stored persona exclusion = %#v, want only real NPC row", filtered)
	}
	if trace["blocked_misclassified_persona_rows"] != 2 || trace["npc_candidate_count"] != 1 {
		t.Fatalf("stored persona exclusion trace = %#v", trace)
	}
	blockedIDs, ok := trace["blocked_row_ids"].([]int64)
	if !ok || len(blockedIDs) != 2 || blockedIDs[0] != 17 || blockedIDs[1] != 18 {
		t.Fatalf("blocked row ids = %#v, want [17 18]", trace["blocked_row_ids"])
	}
	filterPrepareTurnEntityRecollections(
		"Juno enters the room.",
		nil,
		nil,
		nil,
		nil,
		nil,
		&filtered,
	)
	if len(filtered) != 1 || filtered[0].ID != 19 {
		t.Fatalf("prepare-turn NPC delivery candidates = %#v, want only Juno row", filtered)
	}
}

func TestEntityExtractionDeduplicatesOnlyExactNameWithinSameType(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil
	extraction := normalizeCriticExtraction(map[string]any{
		"entities": map[string]any{
			"characters": []any{
				map[string]any{"name": "이시우", "description": "first character record"},
				map[string]any{"name": "  이시우  ", "description": "duplicate character record"},
				map[string]any{"name": "시우", "description": "unconfirmed short-name variant"},
			},
			"items": []any{
				map[string]any{"name": "이시우", "description": "same label but a different entity type"},
			},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-exact-entity", 4, extraction, "이시우와 시우가 언급되었다.", completeTurnEmbeddingConfig{}, time.Unix(1500, 0))
	if result.Entities != 3 || len(fake.savedEntities) != 3 {
		t.Fatalf("saved entities = %d/%d, want exact duplicate removed but variants/types retained: %#v", result.Entities, len(fake.savedEntities), fake.savedEntities)
	}
	var duplicateSkip bool
	for _, item := range result.SkipReasons {
		if item["surface"] == "entities" && item["reason"] == "duplicate_exact_entity_name_type" {
			duplicateSkip = true
			break
		}
	}
	if !duplicateSkip {
		t.Fatalf("missing exact entity duplicate trace: %#v", result.SkipReasons)
	}
}

func TestSubjectiveMemoryExactOwnerGroupsPreserveHistoryNewestFirst(t *testing.T) {
	srv := NewServer(config.Default())
	memories := []store.ProtagonistEntityMemory{
		{ID: 11, OwnerEntityKey: "siwoo_old", OwnerEntityName: "이시우", OwnerEntityRole: "protagonist", OwnerVisibility: "player_known", SourceTurn: 2, MemoryText: "older memory"},
		{ID: 12, OwnerEntityKey: "siwoo_new", OwnerEntityName: "이시우", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceTurn: 8, MemoryText: "newer memory"},
		{ID: 13, OwnerEntityKey: "siu", OwnerEntityName: "시우", OwnerEntityRole: "protagonist", OwnerVisibility: "player_known", SourceTurn: 7, MemoryText: "unconfirmed short-name variant"},
	}

	ordered := srv.canonicalizeSubjectiveEntityMemoriesForRead(context.Background(), "sess-owner-order", memories)
	if len(ordered) != 3 || ordered[0].SourceTurn != 8 || ordered[1].SourceTurn != 7 || ordered[2].SourceTurn != 2 {
		t.Fatalf("subjective memory order = %#v, want turns 8, 7, 2", ordered)
	}
	groups := srv.subjectiveEntityMemoryGroups(context.Background(), "sess-owner-order", memories)
	if len(groups) != 2 {
		t.Fatalf("exact owner groups = %d, want 이시우 merged and 시우 separate: %#v", len(groups), groups)
	}
	if groups[0]["owner_entity_name"] != "이시우" || groups[0]["memory_count"] != 2 || groups[0]["latest_turn_index"] != 8 {
		t.Fatalf("merged exact-name group mismatch: %#v", groups[0])
	}
	if groups[0]["mixed_owner_scope"] != true || groups[0]["scope_variant_count"] != 2 {
		t.Fatalf("scope history should be preserved inside merged owner group: %#v", groups[0])
	}
	if groups[1]["owner_entity_name"] != "시우" || groups[1]["memory_count"] != 1 {
		t.Fatalf("unconfirmed name variant should remain separate: %#v", groups[1])
	}
}
