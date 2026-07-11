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

func TestPrepareTurnCharacterPrivateRecollectionCapsTotalOwners(t *testing.T) {
	memories := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "niv", OwnerEntityName: "Niv", MemoryText: "Niv privately remembers the garden promise."},
		{ID: 2, OwnerEntityKey: "ingrid", OwnerEntityName: "Ingrid", MemoryText: "Ingrid privately doubts the garden promise."},
		{ID: 3, OwnerEntityKey: "ashley", OwnerEntityName: "Ashley", MemoryText: "Ashley privately fears being overheard."},
	}

	trace := filterPrepareTurnEntityRecollections(
		"Niv and Ingrid discuss Ashley in the garden.",
		nil,
		nil,
		nil,
		nil,
		&memories,
	)
	if len(memories) != 2 {
		t.Fatalf("selected private recollections = %d, want 2: %#v", len(memories), memories)
	}
	if memories[0].OwnerEntityKey != "niv" || memories[1].OwnerEntityKey != "ingrid" {
		t.Fatalf("private recollection ordering changed unexpectedly: %#v", memories)
	}
	if trace["character_private_total_cap"] != 2 {
		t.Fatalf("character_private_total_cap = %#v, want 2", trace["character_private_total_cap"])
	}
	dropped, ok := trace["dropped"].([]map[string]any)
	if !ok || len(dropped) != 1 {
		t.Fatalf("dropped trace = %#v, want one capped owner", trace["dropped"])
	}
	if dropped[0]["owner_entity_key"] != "ashley" || dropped[0]["reason"] != "private_recollection_total_capped" {
		t.Fatalf("unexpected total-cap trace: %#v", dropped[0])
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
