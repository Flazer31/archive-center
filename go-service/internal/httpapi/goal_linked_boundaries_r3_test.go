package httpapi

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func goalR3Fixture(refs []string, supplied map[string]any, reversed, sharedQuote bool) (map[string]any, string, []store.DirectEvidence) {
	a := reviewR2Secret("contents", "Sealed parcel contents", reviewR2PlanScope())
	a["fact_id"], a["evidence_excerpt"] = "F41", "Coordinator privately seals the parcel."
	b := reviewR2Secret("instruction", "Courier instruction", reviewR2InstructionScope())
	b["fact_id"], b["evidence_excerpt"] = "F42", "Courier privately receives the instruction."
	quote := "Courier continues the combined amber contingency."
	if sharedQuote {
		quote = stringFromMap(a, "evidence_excerpt")
	}
	claim := reviewR2Claim("combined-contingency", "Combined amber contingency continues", "create", quote, supplied)
	claim["fact_refs"] = refs
	pending := cloneMapAny(claim)
	pending["title"], pending["description"] = "Parcel delivery instruction", stringFromMap(claim, "value")
	secrets := []any{a, b}
	if reversed {
		secrets = []any{b, a}
	}
	content := stringFromMap(a, "evidence_excerpt") + " " + stringFromMap(b, "evidence_excerpt")
	if !sharedQuote {
		content += " " + quote
	}
	ev := reviewR2Evidence(1, quote)
	for i, item := range []map[string]any{a, b} {
		if stringFromMap(item, "evidence_excerpt") == quote {
			continue
		}
		row := reviewR2Evidence(1, stringFromMap(item, "evidence_excerpt"))[0]
		row.ID = int64(701 + i)
		ev = append(ev, row)
	}
	return map[string]any{"protected_secrets": secrets, "state_claims": []any{claim}, "pending_threads": []any{pending}}, content, ev
}

func goalR3AssertBoundaries(t *testing.T, boundaries []any, ex map[string]any, content string, ev []store.DirectEvidence) {
	t.Helper()
	if len(boundaries) != len(sliceFromAny(ex["protected_secrets"])) {
		t.Fatalf("distinct linked source boundaries lost or duplicated: %s", mustCompactJSON(boundaries))
	}
	for index, raw := range sliceFromAny(ex["protected_secrets"]) {
		item := mapFromAny(raw)
		ref := fmt.Sprintf("source-revision:synthetic-goal-r3/protected_secrets/%d", index)
		var found map[string]any
		for _, rawBoundary := range boundaries {
			b := mapFromAny(rawBoundary)
			if stringFromMap(b, "protected_fact_ref") == ref {
				found = b
			}
		}
		if found == nil || stringFromMap(found, "fact_id") != stringFromMap(item, "fact_id") || mustCompactJSON(found["knowledge_scope"]) != mustCompactJSON(item["knowledge_scope"]) {
			t.Fatalf("fact scope changed, unioned or selected by order: %s", mustCompactJSON(boundaries))
		}
		origin := mapFromAny(found["knowledge_source"])
		if stringFromMap(origin, "source_revision") != "synthetic-goal-r3" || stringFromMap(origin, "source_session") != reviewR2SID || intFromAny(origin["source_turn"], 0) != 1 || intFromAny(origin["source_index"], -1) != index {
			t.Fatalf("source attribution lost: %s", mustCompactJSON(found))
		}
		quote := stringFromMap(item, "evidence_excerpt")
		start, end := intFromAny(origin["source_span_start"], -1), intFromAny(origin["source_span_end"], -1)
		if start < 0 || end > len(content) || end <= start || content[start:end] != quote {
			t.Fatalf("source fact assigned another quotation's span: %s", mustCompactJSON(origin))
		}
		ids := preciseMemoryExactEvidenceIDs(ev, reviewR2SID, 1, quote)
		if len(ids) > 0 && (int64(intFromAny(origin["root_evidence_id"], 0)) != ids[0] || mustCompactJSON(origin["direct_evidence_ids"]) != mustCompactJSON(ids)) {
			t.Fatalf("source fact assigned another quotation's evidence IDs: %s", mustCompactJSON(origin))
		}
		if len(ids) == 0 && (origin["root_evidence_id"] != nil || origin["direct_evidence_ids"] != nil) {
			t.Fatalf("goal receipt substituted for a missing source receipt: %s", mustCompactJSON(origin))
		}
		for _, bodyKey := range []string{"summary", "subject", "evidence_excerpt", "memory_text", "body"} {
			if _, exists := found[bodyKey]; exists {
				t.Fatalf("protected body cloned into boundary: %s", mustCompactJSON(found))
			}
		}
	}
}

func TestGoalLinkedBoundariesR3PendingOnlyDelivery(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		for _, shared := range []bool{false, true} {
			for _, supplied := range []bool{false, true} {
				for _, sourceEvidence := range []bool{false, true} {
					t.Run(fmt.Sprintf("reverse=%t/shared_quote=%t/supplied=%t/source_evidence=%t", reversed, shared, supplied, sourceEvidence), func(t *testing.T) {
						var scope map[string]any
						if supplied {
							scope = reviewR2Scope([]string{"Observer"}, []string{"Coordinator"})
						}
						ex, content, ev := goalR3Fixture([]string{"F41", "F42"}, scope, reversed, shared)
						if !sourceEvidence {
							ev = ev[:1] // Original reviewer probe supplied only the goal receipt.
						}
						before := mustCompactJSON(ex)
						normalized := normalizeCriticExtraction(cloneMapAny(ex))
						normalizedBefore := mustCompactJSON(normalized)
						st := &turnRecordingStore{}
						result := artifactSaveResult{}
						(&Server{Store: st}).saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("synthetic-goal-r3"), reviewR2SID, 1, normalized, content, ev, time.Unix(1, 0), &result)
						if result.Errors != 0 || len(st.savedStatusCurrent) == 0 {
							t.Fatalf("production save missing: %+v", result)
						}
						current, pending := lifecycleScopeR2Current(t, st)
						for _, payload := range []map[string]any{mapFromAny(current["lifecycle_details"]), parseJSONMap(pending.HookMetadataJSON), parseJSONMap(pending.DetailsJSON)} {
							goalR3AssertBoundaries(t, sliceFromAny(payload["knowledge_boundaries"]), normalized, content, ev)
							gotScope := mapFromAny(payload["knowledge_scope"])
							if len(scope) == 0 && len(gotScope) != 0 || len(scope) > 0 && mustCompactJSON(gotScope) != mustCompactJSON(scope) {
								t.Fatalf("multiple origins chose own scope or replaced supplied scope: %s", mustCompactJSON(payload))
							}
						}
						pending.ID = 23
						in := carry49Input(store.Memory{})
						in.Memories, in.PendingThreads, in.UserInput = nil, []store.PendingThread{pending}, "Combined amber contingency"
						inBefore := mustCompactJSON(in)
						out := buildPrepareTurnInjectionAssemblyWithBudget(in)
						facts, _ := multiAgentCandidatePool(&out)
						seen := false
						for _, c := range facts {
							if c.SourceTable != "pending_threads" {
								continue
							}
							seen = true
							model := mustCompactJSON(prepareTurnMemoryModelCandidate(c, map[string]string{c.CanonicalFactID: "F1"}))
							main := carryDirectionR2Main(t, &out, c)
							plan := renderPrepareTurnPriorityMemoryDeliveryPlan(&out, 36000, 10, "auto", nil, prepareTurnMemorySelectionContext{Query: in.UserInput}, nil, nil, nil, nil)
							notes := stringFromMap(buildPrepareTurnPreprocessingNotes(out.Preprocessing, plan, nil), "final_text")
							for _, text := range []string{model, main, notes} {
								for _, boundary := range sliceFromAny(parseJSONMap(pending.HookMetadataJSON)["knowledge_boundaries"]) {
									if !strings.Contains(text, mustCompactJSON(boundary)) {
										t.Fatalf("pending-only candidate/main/notes lost attributed boundary: %s", text)
									}
								}
								for _, raw := range sliceFromAny(normalized["protected_secrets"]) {
									if strings.Contains(text, stringFromMap(mapFromAny(raw), "summary")) {
										t.Fatalf("protected body bypassed independent owner: %s", text)
									}
								}
							}
						}
						if !seen || before != mustCompactJSON(ex) || normalizedBefore != mustCompactJSON(normalized) || inBefore != mustCompactJSON(in) {
							t.Fatal("pending candidate absent or source input mutated")
						}
					})
				}
			}
		}
	}
}

func TestGoalLinkedBoundariesR3SingleAndUnlinkedControls(t *testing.T) {
	for _, refs := range [][]string{nil, {"F999"}, {"F41"}, {"F42"}, {"F41", "F41"}} {
		for _, supplied := range []bool{false, true} {
			t.Run(fmt.Sprintf("refs=%v/supplied=%t", refs, supplied), func(t *testing.T) {
				var scope map[string]any
				if supplied {
					scope = reviewR2Scope([]string{"Observer"}, []string{"Coordinator"})
				}
				ex, content, ev := goalR3Fixture(refs, scope, false, false)
				payload := cloneMapAny(mapFromAny(sliceFromAny(ex["state_claims"])[0]))
				before := mustCompactJSON(payload)
				retainExactGoalKnowledgeMetadata(payload, ex, content, ev, reviewR2SID, 1, "synthetic-goal-r3", stringFromMap(payload, "evidence_excerpt"))
				if len(sliceFromAny(payload["knowledge_boundaries"])) != 0 {
					t.Fatal("single or unlinked quotation gained multi-link boundaries")
				}
				want := scope
				if !supplied && len(refs) > 0 && refs[0] != "F999" {
					for _, raw := range sliceFromAny(ex["protected_secrets"]) {
						if goalKnowledgeFactLinked(payload, mapFromAny(raw)) {
							want = mapFromAny(mapFromAny(raw)["knowledge_scope"])
						}
					}
				}
				gotScope := mapFromAny(payload["knowledge_scope"])
				if len(want) == 0 && len(gotScope) != 0 || len(want) > 0 && mustCompactJSON(gotScope) != mustCompactJSON(want) {
					t.Fatal("single-link or supplied current scope changed")
				}
				if (len(refs) == 0 || refs[0] == "F999" || supplied) && before != mustCompactJSON(payload) {
					t.Fatal("unlinked/supplied single-link payload bytes changed")
				}
			})
		}
	}
}
