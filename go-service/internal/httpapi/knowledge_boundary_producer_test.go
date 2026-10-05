package httpapi

import (
	"reflect"
	"testing"
)

func TestKnowledgeBoundaryProducerPreservesExplicitLinks(t *testing.T) {
	scope := map[string]any{"known_by": []any{"Mira"}, "unknown_to": []any{"Oren"}, "suspected_by": []any{"Lio"}, "misinformed_by": []any{"Tess"}, "revealed_to": []any{"Paz"}}
	links := map[string]any{"fact_ref": "F41", "source_refs": []any{map[string]any{"source_table": "pending_threads", "source_row_id": "23"}}, "source_memory_id": 11, "lifecycle_key": "amber-plan", "pending_thread_key": "amber-thread", "summary_refs": []any{"S6"}}
	for _, kind := range []string{"protected", "identity", "subjective"} {
		t.Run(kind, func(t *testing.T) {
			item := map[string]any{"owner": "Mira", "owner_entity_name": "Mira", "summary": "The amber contingency remains private.", "memory_text": "The amber contingency remains private.", "evidence_excerpt": "Mira recorded the amber contingency.", "knowledge_scope": scope, "secret_kind": "plan", "surface_identity_name": "Wanderer", "true_identity_name": "Mira", "transition": "reveal"}
			for key, value := range links {
				item[key] = value
			}
			var normalized map[string]any
			var candidates []preciseMemoryCandidate
			switch kind {
			case "protected":
				normalized = mapFromAny(normalizeProtectedSecrets([]any{item})[0])
				candidates = protectedSecretPerspectiveMemoryCandidates(map[string]any{"protected_secrets": []any{normalized}})
			case "identity":
				normalized = mapFromAny(normalizeCharacterIdentityAccuracy([]any{item})[0])
				candidates = protectedSecretPerspectiveMemoryCandidates(map[string]any{"character_identity_accuracy": []any{normalized}})
			case "subjective":
				normalized = mapFromAny(normalizeSubjectiveEntityMemories([]any{item})[0])
				candidates = subjectivePerspectiveMemoryCandidates(map[string]any{"subjective_entity_memories": []any{normalized}})
			}
			for key, want := range links {
				if !reflect.DeepEqual(normalized[key], want) {
					t.Errorf("normalizer lost %s: got %v want %v", key, normalized[key], want)
				}
			}
			if len(candidates) == 0 {
				t.Fatal("perspective positive control missing")
			}
			for _, candidate := range candidates {
				for key, want := range links {
					if !reflect.DeepEqual(candidate.payload[key], want) {
						t.Errorf("perspective writer lost %s", key)
					}
				}
				for key, want := range scope {
					if !reflect.DeepEqual(stringsFromAny(mapFromAny(candidate.payload["knowledge_scope"])[key]), stringsFromAny(want)) {
						t.Errorf("perspective writer changed %s", key)
					}
				}
			}
		})
	}
}

func TestKnowledgeBoundaryProducerUnlinkedPayloadUnchanged(t *testing.T) {
	item := map[string]any{"owner_entity_name": "Mira", "memory_text": "Mira remembers a public walk.", "evidence_excerpt": "Mira walked beside the cart.", "knowledge_scope": map[string]any{"known_by": []any{"Mira"}}}
	withScope := subjectivePerspectiveMemoryCandidates(map[string]any{"subjective_entity_memories": []any{item}})
	delete(item, "knowledge_scope")
	withoutScope := subjectivePerspectiveMemoryCandidates(map[string]any{"subjective_entity_memories": []any{item}})
	if len(withScope) != 1 || len(withoutScope) != 1 {
		t.Fatal("subjective positive control missing")
	}
	if mustCompactJSON(withScope[0].payload) != mustCompactJSON(withoutScope[0].payload) {
		t.Fatal("unlinked perspective payload changed")
	}
	item["knowledge_scope"] = map[string]any{"known_by": []any{"Mira"}}
	item["source_refs"] = []any{}
	item["fact_ref"] = nil
	item["lifecycle_key"] = ""
	emptyLinks := subjectivePerspectiveMemoryCandidates(map[string]any{"subjective_entity_memories": []any{item}})
	if len(emptyLinks) != 1 || mustCompactJSON(emptyLinks[0].payload) != mustCompactJSON(withoutScope[0].payload) {
		t.Fatal("absent optional links changed an unlinked payload")
	}
}
