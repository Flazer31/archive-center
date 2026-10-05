package httpapi

import (
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestBoundaryCarryR3SecretIdentityDirection(t *testing.T) {
	for _, parentIdentity := range []string{"fact_id", "secret_id"} {
		t.Run(parentIdentity, func(t *testing.T) {
			in := carry49Input(carry49Memory(map[string]any{"secret_id": "F41", "source_refs": []string{"F42"}}))
			in.Memories = append(in.Memories, store.Memory{ID: 22, ChatSessionID: "carry-synthetic", TurnIndex: 5, SummaryJSON: mustCompactJSON(map[string]any{"narrative_events": []any{map[string]any{parentIdentity: "F42", "summary": "Mira amber contingency public cart"}}})})
			in.PendingThreads = []store.PendingThread{
				{ID: 23, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira follows amber contingency", HookMetadataJSON: `{"source_refs":["F41"]}`},
				{ID: 24, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira checks public cart wheels", HookMetadataJSON: `{"source_refs":["F42"]}`},
			}
			before := mustCompactJSON(in)
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			facts, _ := multiAgentCandidatePool(&out)
			control := in
			control.Memories = in.Memories[1:]
			plain := buildPrepareTurnInjectionAssemblyWithBudget(control)
			plainFacts, _ := multiAgentCandidatePool(&plain)
			public, linked := 0, 0
			for _, c := range facts {
				id := intFromAny(c.SourceRowID, 0)
				if c.SourceTable != "pending_threads" && !(c.SourceTable == "memories" && id == 22) {
					continue
				}
				main := carryDirectionR2Main(t, &out, c)
				if c.SourceTable == "pending_threads" && id == 23 {
					linked++
					if len(c.KnowledgeBoundaries) != 1 {
						t.Errorf("secret_id descendant boundary count=%d want=1", len(c.KnowledgeBoundaries))
						continue
					}
					b := c.KnowledgeBoundaries[0]
					if stringFromMap(b, "secret_id") != "F41" || stringFromMap(b, "protected_fact_ref") != "memories:11/protected_secrets/0" || !strings.Contains(main, mustCompactJSON(b)) {
						t.Errorf("secret identity/scope lost in candidate or final main: %s", main)
					}
					if strings.Contains(mustCompactJSON(b), "Mira keeps an amber contingency") {
						t.Error("protected source body cloned into boundary")
					}
					continue
				}
				public++
				if len(c.KnowledgeBoundaries) != 0 {
					t.Errorf("secret child scope exported to public row %d: %s", id, mustCompactJSON(c.KnowledgeBoundaries))
				}
				found := false
				for _, p := range plainFacts {
					if p.SourceTable == c.SourceTable && p.SourceRowID == c.SourceRowID && p.CompleteText == c.CompleteText {
						found = true
						if prepareTurnMemoryReadingText(c) != prepareTurnMemoryReadingText(p) || main != carryDirectionR2Main(t, &plain, p) {
							t.Errorf("public candidate/final main changed from no-secret-child control: row=%d", id)
						}
					}
				}
				if !found {
					t.Fatalf("public positive control absent: row=%d", id)
				}
			}
			if public != 2 || linked != 1 || before != mustCompactJSON(in) {
				t.Fatalf("controls/source preservation: public=%d linked=%d unchanged=%v", public, linked, before == mustCompactJSON(in))
			}
		})
	}
}

func TestBoundaryCarryR3MemoryItemExplicitScope(t *testing.T) {
	for _, scopeField := range []string{"unknown_to", "suspected_by", "misinformed_by"} {
		t.Run(scopeField, func(t *testing.T) {
			scope := map[string]any{"known_by": []string{"Mira"}, scopeField: []string{"Oren"}}
			item := map[string]any{"fact_id": "F42", "summary": "Mira follows amber contingency", "knowledge_scope": scope}
			if len(prepareTurnOwnKnowledgeBoundary(item)) != 1 {
				t.Fatal("existing scope-owner positive control absent")
			}
			in := carry49Input(store.Memory{ID: 22, ChatSessionID: "carry-synthetic", TurnIndex: 5, SummaryJSON: mustCompactJSON(map[string]any{"narrative_events": []any{item, map[string]any{"fact_id": "F99", "summary": "Mira checks amber contingency public cart"}}})})
			in.PendingThreads = []store.PendingThread{
				{ID: 23, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira follows amber contingency", HookMetadataJSON: `{"source_refs":["F42"]}`},
				{ID: 24, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira follows amber contingency"},
				{ID: 25, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira checks amber contingency public cart", HookMetadataJSON: `{"source_refs":["F99"]}`},
			}
			before := mustCompactJSON(in)
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			facts, _ := multiAgentCandidatePool(&out)
			control := in
			publicItem := cloneMapAny(item)
			delete(publicItem, "knowledge_scope")
			control.Memories = []store.Memory{in.Memories[0]}
			control.Memories[0].SummaryJSON = mustCompactJSON(map[string]any{"narrative_events": []any{publicItem, map[string]any{"fact_id": "F99", "summary": "Mira checks amber contingency public cart"}}})
			plain := buildPrepareTurnInjectionAssemblyWithBudget(control)
			plainFacts, _ := multiAgentCandidatePool(&plain)
			linked, public := 0, 0
			for _, c := range facts {
				if c.SourceTable != "pending_threads" {
					continue
				}
				main := carryDirectionR2Main(t, &out, c)
				if intFromAny(c.SourceRowID, 0) == 23 {
					linked++
					if len(c.KnowledgeBoundaries) != 1 {
						t.Errorf("explicit event scope lost: boundaries=%s main=%s", mustCompactJSON(c.KnowledgeBoundaries), main)
						continue
					}
					b := c.KnowledgeBoundaries[0]
					if mustCompactJSON(b["knowledge_scope"]) != mustCompactJSON(scope) || stringFromMap(b, "protected_fact_ref") != "memories:22/narrative_events/0" || !strings.Contains(main, mustCompactJSON(b)) {
						t.Errorf("recorded scope/attribution altered in candidate or final main: %s", main)
					}
					continue
				}
				public++
				if len(c.KnowledgeBoundaries) != 0 {
					t.Error("unlinked/public sibling acquired scoped-event boundary")
				}
				found := false
				for _, p := range plainFacts {
					if p.SourceTable == c.SourceTable && p.SourceRowID == c.SourceRowID && p.CompleteText == c.CompleteText {
						found = true
						if prepareTurnMemoryReadingText(c) != prepareTurnMemoryReadingText(p) || main != carryDirectionR2Main(t, &plain, p) {
							t.Error("unlinked/public sibling bytes changed")
						}
					}
				}
				if !found {
					t.Fatal("public positive control absent")
				}
			}
			if linked != 1 || public != 2 || before != mustCompactJSON(in) {
				t.Fatalf("controls/source preservation: linked=%d public=%d unchanged=%v", linked, public, before == mustCompactJSON(in))
			}
		})
	}
}

func TestBoundaryCarryR3PublicKnownScopeAndDisclosure(t *testing.T) {
	for _, tc := range []struct {
		name, bucket string
		fields       map[string]any
	}{
		{"ordinary_known_by", "narrative_events", nil},
		{"secret_public_disclosure", "protected_secrets", map[string]any{"disclosure_policy": "public"}},
		{"identity_public_disclosure", "character_identity_accuracy", map[string]any{"reveal_policy": "public"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := map[string]any{"fact_id": "F42", "summary": "Mira follows amber contingency", "knowledge_scope": map[string]any{"known_by": []string{"Mira"}}}
			for k, v := range tc.fields {
				item[k] = v
				// The existing explicit disclosure observation remains authoritative
				// even with a recorded non-knower. A bare "public" policy string
				// normalizes to the default private policy in this owner.
				mapFromAny(item["knowledge_scope"])["publicly_revealed"] = true
				mapFromAny(item["knowledge_scope"])["unknown_to"] = []string{"Oren"}
			}
			if len(tc.fields) > 0 && protectedSecretRequiresGuard(item, "disclosure_policy") {
				t.Fatal("existing public-disclosure positive control absent")
			}
			in := carry49Input(store.Memory{ID: 22, ChatSessionID: "carry-synthetic", SummaryJSON: mustCompactJSON(map[string]any{tc.bucket: []any{item}})})
			in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira follows amber contingency", HookMetadataJSON: `{"source_refs":["F42"]}`}}
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			facts, _ := multiAgentCandidatePool(&out)
			found := false
			for _, c := range facts {
				if c.SourceTable == "pending_threads" {
					found = true
					if len(c.KnowledgeBoundaries) != 0 || strings.Contains(carryDirectionR2Main(t, &out, c), "linked fact knowledge boundary") {
						t.Error("ordinary known_by or explicit public disclosure acquired a boundary")
					}
				}
			}
			if !found {
				t.Fatal("public linked candidate absent")
			}
		})
	}
}
