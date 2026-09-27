package httpapi

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func Test48PerspectiveOverlapPreservesIndependentObjectiveClaims(t *testing.T) {
	for _, tc := range []struct{ name, owner, subject, claim, value string }{
		{"completion_cause_of_trust", "Mira", "Rowan", "Mira trusts Rowan's workmanship more after he completed the ferry repairs and returned her hammer.", "All ferry repairs finished"},
		{"self_reaction", "Rowan", "Rowan", "Rowan feels relieved after he finished the ferry repairs.", "All ferry repairs finished"},
		{"function_words", "Mira", "bell", "The vault is open.", "The bell is ringing."},
		{"opposite_sentence", "Mira", "ferry", "The ferry is closed.", "The ferry is open."},
		{"negated_sentence", "Mira", "ferry", "The ferry is open.", "The ferry is not open."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, withEvidence := range []bool{false, true} {
				state := map[string]any{"subject": tc.subject, "state_slot": "condition", "value": tc.value}
				if withEvidence {
					state["evidence_excerpt"] = tc.subject + ": " + tc.value
				}
				x := map[string]any{"belief_updates": []any{map[string]any{"perspective_owner": tc.owner, "belief": tc.claim}}, "state_claims": []any{state}}
				if n := quarantineCriticPerspectiveClaimsFromObjectiveLanes(x); n != 0 {
					t.Errorf("independent fact removed (evidence=%v): %s", withEvidence, tc.value)
				}
			}
		})
	}
}

func Test48ObjectiveCompletionSurvivesReactionAndPersists(t *testing.T) {
	for _, owner := range []string{"Mira", "Rowan"} {
		t.Run(owner, func(t *testing.T) {
			source := "Rowan completed the railing and announced that all ferry repairs were finished. " + owner + " felt relieved after the ferry repairs were finished."
			x := map[string]any{
				"subjective_entity_memories": []any{map[string]any{"owner_entity_name": owner, "owner_visibility": "owner_private", "memory_text": owner + " felt relieved after the ferry repairs were finished.", "evidence_excerpt": source}},
				"state_claims":               []any{map[string]any{"subject": "Rowan", "state_slot": "repair_status", "lifecycle_key": "ferry-repair", "value": "All ferry repairs finished", "transition": "complete", "evidence_excerpt": source}},
				"narrative_events":           []any{map[string]any{"event": "Rowan completed the ferry repairs.", "evidence_excerpt": source}},
				"kg_triples":                 []any{map[string]any{"subject": "Rowan", "predicate": "completed", "object": "ferry repairs", "evidence_excerpt": source}},
			}
			filtered, trace := quarantineCriticProtectedCandidates(x, "", source)
			for _, lane := range []string{"state_claims", "narrative_events", "kg_triples"} {
				if len(sliceFromAny(filtered[lane])) != 1 {
					t.Fatalf("%s erased: %#v", lane, trace)
				}
			}
			st := &turnRecordingStore{}
			save46LifecycleFixture(t, st, 1, map[string]any{"pending_threads": []any{map[string]any{"title": "Ferry repair", "lifecycle_key": "ferry-repair"}}}, "Rowan promised to repair the ferry.")
			save46LifecycleFixture(t, st, 2, normalizeCriticExtraction(filtered), "At the ferry, the crew gathered. "+source+" They left together.")
			complete := false
			for _, row := range st.returnStatusCurrent {
				p := parseJSONMap(row.ValueJSON)
				if p["lifecycle_key"] == "ferry-repair" && p["transition"] == "complete" {
					complete = true
				}
			}
			if !complete {
				t.Fatalf("accepted objective completion did not update existing lifecycle: %#v", st.returnStatusCurrent)
			}
		})
	}
}

func Test48CriticTypedPromptExamplesPersistQualifiers(t *testing.T) {
	prompt, where := readCriticSystemPrompt(filepath.Join("..", "..", "..", "prompts"))
	if where == "fallback_builtin" {
		t.Fatal("source prompt missing")
	}
	x := map[string]any{"turn_summary": "Source-grounded observations.", "entities": map[string]any{"characters": []any{map[string]any{"name": "Mira"}, map[string]any{"name": "Rowan"}, map[string]any{"name": "Eren"}}}}
	lanes := []string{"voice_observations", "habit_observations", "relationship_observations", "interaction_boundaries", "protected_secrets"}
	evidence := []string{}
	for _, lane := range lanes {
		found := false
		for _, line := range strings.Split(prompt, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), `{"`+lane+`":`) {
				continue
			}
			var v map[string]any
			if e := json.Unmarshal([]byte(line), &v); e != nil {
				t.Fatal(e)
			}
			x[lane] = v[lane]
			for _, item := range sliceFromAny(v[lane]) {
				evidence = append(evidence, interactionAdmissionEvidence(mapFromAny(item)))
			}
			found = true
			break
		}
		if !found {
			t.Fatalf("executable typed example missing for %s", lane)
		}
	}
	for _, excerpt := range evidence {
		x["evidence_excerpts"] = append(sliceFromAny(x["evidence_excerpts"]), excerpt)
	}
	source := strings.Join(evidence, "\n")
	validated, _, err := validateCriticExtractionSchema(x)
	if err != nil {
		t.Fatal(err)
	}
	protected, _ := quarantineCriticProtectedCandidates(validated, "", source)
	admitted, trace := admitCriticInteractionLanes(protected, "", source)
	for _, lane := range lanes {
		if len(sliceFromAny(admitted[lane])) != 1 {
			t.Fatalf("%s example rejected: %#v", lane, trace)
		}
	}
	normalized := normalizeCriticExtraction(admitted)
	st := newPreciseMemoryRecordingStore()
	srv := NewServer(config.Default())
	srv.Store = st
	result := srv.saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("typed-examples-48"), "typed-examples-48", 1, normalized, source, completeTurnEmbeddingConfig{}, time.Unix(100, 0))
	if result.Errors != 0 {
		t.Fatalf("save: %#v", result)
	}
	found := map[string]bool{}
	known := map[string]bool{}
	for _, unit := range st.unitsByKey {
		p := parseJSONMap(unit.PayloadJSON)
		switch unit.Subtype {
		case "voice_behavior":
			if p["counterpart"] != "Rowan" || p["context_expression"] != "when asking Rowan for help" {
				t.Fatalf("voice context lost: %#v", p)
			}
			found["voice_observations"] = true
		case "habit_observation":
			if p["observation_kind"] != "exception" || p["counterpart"] != "Eren" || !strings.Contains(stringFromMap(p, "context_expression"), "headache") {
				t.Fatalf("habit exception lost: %#v", p)
			}
			found["habit_observations"] = true
		case "relationship_trust":
			found["relationship_observations"] = p["source_entity"] == "Mira" && p["target_entity"] == "Rowan"
		}
		if unit.Kind == "boundary" {
			found["interaction_boundaries"] = p["actor"] == "Eren" && p["counterpart"] == "Mira" && p["decision"] == "refuse"
		}
		if p["contract_version"] == "perspective_memory.v1" && p["epistemic_state"] == "known" {
			known[stringFromMap(p, "knowledge_holder")] = true
			if unit.Visibility != "owner_private" {
				t.Fatal("secret became public")
			}
		}
	}
	found["protected_secrets"] = known["Mira"] && known["Rowan"]
	for _, lane := range lanes {
		if !found[lane] {
			t.Errorf("missing stored semantics for %s: known=%v normalized=%#v result=%#v", lane, known, normalized[lane], result)
		}
	}
}
