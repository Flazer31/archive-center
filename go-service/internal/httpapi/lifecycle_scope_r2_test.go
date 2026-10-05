package httpapi

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func lifecycleScopeR2Current(t *testing.T, st *turnRecordingStore) (map[string]any, store.PendingThread) {
	t.Helper()
	for _, v := range st.returnStatusCurrent {
		payload := parseJSONMap(v.ValueJSON)
		if stringFromMap(payload, "state_slot") == "goal_status" {
			if pending := narrativePendingSnapshot(payload); pending != nil {
				return payload, *pending
			}
		}
	}
	t.Fatal("positive current goal and pending snapshot missing")
	return nil, store.PendingThread{}
}

func lifecycleScopeR2Save(t *testing.T, st *turnRecordingStore, turn int, claim map[string]any, mode string) {
	t.Helper()
	quote := stringFromMap(claim, "evidence_excerpt")
	ex := map[string]any{}
	if mode != "pending" {
		ex["state_claims"] = []any{claim}
	}
	if mode != "claim" {
		pending := cloneMapAny(claim)
		pending["title"], pending["description"] = "Parcel delivery instruction", stringFromMap(claim, "value")
		ex["pending_threads"] = []any{pending}
	}
	before := mustCompactJSON(ex)
	normalized := normalizeCriticExtraction(cloneMapAny(ex))
	normalizedBefore := mustCompactJSON(normalized)
	result := artifactSaveResult{}
	(&Server{Store: st}).saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext(fmt.Sprint("synthetic-lifecycle-scope-", turn)), reviewR2SID, turn, normalized, "Before. "+quote+" After.", reviewR2Evidence(turn, quote), time.Unix(int64(turn), 0), &result)
	if result.Errors != 0 {
		t.Fatalf("production narrative save failed: %+v", result.ErrorDetails)
	}
	if before != mustCompactJSON(ex) || normalizedBefore != mustCompactJSON(normalized) {
		t.Fatal("original or normalized synthetic input mutated")
	}
	st.returnPendingThreads = nil
	for _, v := range st.returnStatusCurrent {
		if pending := narrativePendingSnapshot(parseJSONMap(v.ValueJSON)); pending != nil {
			st.returnPendingThreads = append(st.returnPendingThreads, *pending)
		}
	}
}

func TestLifecycleScopeR2ClosedMetadataKeepsPhase(t *testing.T) {
	for _, closing := range []string{"complete", "abandon", "cancel", "resolve", "clear", "supersede", "pause", "defer"} {
		for _, incoming := range []string{"reaffirm", "set", "reveal", "uncertain", closing} {
			for _, mode := range []string{"claim", "pending", "both"} {
				t.Run(closing+"/"+incoming+"/"+mode, func(t *testing.T) {
					st := &turnRecordingStore{}
					reviewR2Run(t, st, 1, reviewR2Extraction(reviewR2InstructionScope(), false, true), reviewR2Quote)
					lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Delivery closed", closing, "Courier closes the delivery instruction.", reviewR2InstructionScope()), "claim")
					old, prior := lifecycleScopeR2Current(t, st)
					oldBytes := mustCompactJSON(old)
					if prior.Status != narrativeLifecycleProjectionStatus(closing) {
						t.Fatal("setup did not enter requested closed phase")
					}
					boundaries := mustCompactJSON(mapFromAny(old["lifecycle_details"])["knowledge_boundaries"])
					writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
					scope := reviewR2Scope([]string{"Coordinator", "Courier", "Outsider", "Observer"}, []string{})
					claim := reviewR2Claim("instruction-fact", "Delivery closed", incoming, "Courier reports the closed instruction to Observer.", scope)
					lifecycleScopeR2Save(t, st, 3, claim, mode)
					current, pending := lifecycleScopeR2Current(t, st)
					if pending.Status != prior.Status || stringFromMap(current, "transition") != closing || stringFromMap(mapFromAny(current["lifecycle_details"]), "transition") != closing {
						t.Errorf("scope-only %s changed %s phase: status=%s transition=%s", incoming, closing, pending.Status, stringFromMap(current, "transition"))
					}
					if pending.ResolvedTurn != prior.ResolvedTurn || pending.ResolutionNote != prior.ResolutionNote || pending.CreatedTurn != prior.CreatedTurn || !pending.CreatedAt.Equal(prior.CreatedAt) {
						t.Error("scope-only update changed resolved turn, resolution or occurrence origin")
					}
					for _, payload := range []map[string]any{mapFromAny(current["lifecycle_details"]), parseJSONMap(pending.HookMetadataJSON), parseJSONMap(pending.DetailsJSON)} {
						if mustCompactJSON(payload["knowledge_scope"]) != mustCompactJSON(scope) {
							t.Error("new current supplied scope lost")
						}
						if mustCompactJSON(payload["knowledge_boundaries"]) != boundaries {
							t.Error("historical quotation scopes/ref/provenance changed")
						}
					}
					if len(st.savedStatusCurrent) != writes+1 || len(st.savedStatusEvents) != events+1 {
						t.Error("scope-only update must persist one current/history write")
					}
					if oldBytes != mustCompactJSON(old) || mustCompactJSON(parseJSONMap(st.savedStatusEvents[len(st.savedStatusEvents)-1].PreviousValueJSON)) != oldBytes {
						t.Error("previous phase/input or reversible event predecessor mutated")
					}
					writes, events = len(st.savedStatusCurrent), len(st.savedStatusEvents)
					for _, turn := range []int{3, 4} {
						lifecycleScopeR2Save(t, st, turn, claim, mode)
					}
					if len(st.savedStatusCurrent) != writes || len(st.savedStatusEvents) != events {
						t.Error("repeated no-change scope manufactured extra write")
					}
				})
			}
		}
	}
}

func TestLifecycleScopeR2OpenScopeAndUnchangedClosedControls(t *testing.T) {
	for _, phase := range []string{"create", "complete", "abandon", "cancel"} {
		for _, incoming := range []string{"set", "reaffirm", "reveal"} {
			t.Run(phase+"/"+incoming, func(t *testing.T) {
				st := &turnRecordingStore{}
				reviewR2Run(t, st, 1, reviewR2Extraction(reviewR2InstructionScope(), false, true), reviewR2Quote)
				if phase != "create" {
					lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Delivery closed", phase, "Courier closes the delivery instruction.", reviewR2InstructionScope()), "claim")
				}
				payload, prior := lifecycleScopeR2Current(t, st)
				claim := reviewR2Claim("instruction-fact", stringFromMap(payload, "value"), incoming, "Courier repeats the recorded instruction.", reviewR2InstructionScope())
				writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
				lifecycleScopeR2Save(t, st, 3, claim, "both")
				if incoming != "reveal" || phase != "create" {
					if len(st.savedStatusCurrent) != writes || len(st.savedStatusEvents) != events {
						t.Error("unchanged scope manufactured extra write")
					}
				}
				if phase == "create" {
					claim["knowledge_scope"] = reviewR2Scope([]string{"Coordinator", "Courier", "Observer"}, []string{"Outsider"})
					lifecycleScopeR2Save(t, st, 4, claim, "both")
					current, pending := lifecycleScopeR2Current(t, st)
					if pending.Status != "open" || mustCompactJSON(mapFromAny(current["lifecycle_details"])["knowledge_scope"]) != mustCompactJSON(claim["knowledge_scope"]) {
						t.Error("open goal scope update lost or closed the goal")
					}
				} else {
					_, pending := lifecycleScopeR2Current(t, st)
					if pending.Status != prior.Status || pending.ResolvedTurn != prior.ResolvedTurn {
						t.Error("unchanged-scope mention reactivated closed goal")
					}
				}
			})
		}
	}
}

func TestLifecycleScopeR2ExplicitReactivationControls(t *testing.T) {
	for _, closing := range []string{"complete", "abandon", "cancel", "pause"} {
		for _, incoming := range []string{"reopen", "resume", "correction", "reversal"} {
			for _, changed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/scope_changed=%t", closing, incoming, changed), func(t *testing.T) {
					st := &turnRecordingStore{}
					reviewR2Run(t, st, 1, reviewR2Extraction(reviewR2InstructionScope(), false, true), reviewR2Quote)
					lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Delivery closed", closing, "Courier closes the delivery instruction.", reviewR2InstructionScope()), "claim")
					scope := reviewR2InstructionScope()
					if changed {
						scope = reviewR2Scope([]string{"Coordinator", "Courier", "Observer"}, []string{"Outsider"})
					}
					lifecycleScopeR2Save(t, st, 3, reviewR2Claim("instruction-fact", "Delivery closed", incoming, "Courier explicitly reactivates the delivery instruction.", scope), "claim")
					current, pending := lifecycleScopeR2Current(t, st)
					if pending.Status != "open" || pending.ResolvedTurn != 0 || stringFromMap(current, "transition") != incoming {
						t.Error("explicit reactivation behavior changed")
					}
					if mustCompactJSON(mapFromAny(current["lifecycle_details"])["knowledge_scope"]) != mustCompactJSON(scope) {
						t.Error("explicit reactivation scope lost")
					}
				})
			}
		}
	}
}

func TestLifecycleScopeR2CurrentDisclosureDeliveredBesideHistory(t *testing.T) {
	st := &turnRecordingStore{}
	_, old, public := reviewR2Run(t, st, 1, reviewR2Extraction(nil, false, false), reviewR2Quote)
	history := mustCompactJSON(old["knowledge_boundaries"])
	publicBefore := mustCompactJSON(public)
	scope := reviewR2Scope([]string{"Coordinator", "Courier", "Outsider", "Observer"}, []string{})
	lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Instruction disclosed", "reveal", "Courier discloses the instruction to the audience.", scope), "claim")
	_, pending := lifecycleScopeR2Current(t, st)
	pending.ID = 77 // Materialize only the recording SQL boundary, avoiding ID-zero fallback.
	before := mustCompactJSON(pending)
	out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{PendingThreads: []store.PendingThread{pending}, UserInput: pending.Description, TopK: 5, MaxChars: 36000, ProtectedSecretBudgetChars: 1})
	prepareTurnResolvePrioritySourcePool(&out, pending.Description, []string{pending.Description}, 3, nil)
	candidates, _ := multiAgentCandidatePool(&out)
	var chosen prepareTurnPriorityMemoryCandidate
	found := false
	for _, candidate := range candidates {
		if candidate.SourceTable == "pending_threads" {
			chosen, found = candidate, true
		}
	}
	if !found {
		t.Fatal("positive pending candidate absent")
	}
	if chosen.Visibility != "general" || len(chosen.AllowedViewers) != 0 {
		t.Error("public disclosure was reclassified or restricted")
	}
	selection := &multiAgentSelection{Candidates: []prepareTurnPriorityMemoryCandidate{chosen}, Roles: []multiAgentRoleResult{{Role: chosen.Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{chosen.CanonicalFactID}, Reasons: map[string]string{chosen.CanonicalFactID: "Current instruction explicitly disclosed"}}}}}
	plan := renderPrepareTurnPriorityMemoryDeliveryPlan(&prepareTurnInjectionAssembly{Preprocessing: selection, ProtectedSecretBudgetChars: 1}, 36000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: pending.Description, CurrentTurn: 3}, nil, nil, nil, nil)
	notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
	for _, rendered := range []string{mustCompactJSON(prepareTurnPriorityCandidateMap(chosen, true)), stringFromMap(plan, "main_memory_text"), stringFromMap(notes, "final_text")} {
		if !strings.Contains(rendered, mustCompactJSON(scope)) {
			t.Error("current supplied disclosure scope lost from candidate/main/notes")
		}
		for _, raw := range sliceFromAny(old["knowledge_boundaries"]) {
			if !strings.Contains(rendered, mustCompactJSON(raw)) {
				t.Error("historical quotation scope/ref/provenance lost")
			}
		}
	}
	current, _ := lifecycleScopeR2Current(t, st)
	if history != mustCompactJSON(mapFromAny(current["lifecycle_details"])["knowledge_boundaries"]) || before != mustCompactJSON(pending) || publicBefore != mustCompactJSON(public) {
		t.Error("canonical/history/public sibling input mutated")
	}
}

func TestLifecycleScopeR2ExplicitLinkedFactUpdates(t *testing.T) {
	for _, phase := range []string{"create", "complete", "abandon", "cancel"} {
		t.Run(phase, func(t *testing.T) {
			st := &turnRecordingStore{}
			reviewR2Run(t, st, 1, reviewR2Extraction(nil, false, true), reviewR2Quote)
			if phase != "create" {
				lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Delivery closed", phase, "Courier closes the instruction.", nil), "claim")
			}
			prior, priorPending := lifecycleScopeR2Current(t, st)
			for index, scope := range []map[string]any{
				reviewR2Scope([]string{"Coordinator", "Courier", "Outsider"}, []string{"Observer"}),
				reviewR2Scope([]string{"Coordinator", "Courier", "Outsider", "Observer"}, []string{}),
			} {
				ex := reviewR2Extraction(nil, false, true)
				mapFromAny(sliceFromAny(ex["protected_secrets"])[1])["knowledge_scope"] = scope
				for _, field := range []string{"state_claims", "pending_threads"} {
					claim := mapFromAny(sliceFromAny(ex[field])[0])
					claim["transition"], claim["value"] = "reveal", stringFromMap(prior, "value")
				}
				_, narrative, _ := reviewR2Run(t, st, index+3, ex, reviewR2Quote)
				if mustCompactJSON(narrative["knowledge_scope"]) != mustCompactJSON(normalizeProtectedSecretKnowledgeScope(scope, "Coordinator")) {
					t.Error("explicit linked-fact repeated value/phase scope update lost")
				}
				_, pending := lifecycleScopeR2Current(t, st)
				if pending.Status != priorPending.Status || pending.ResolvedTurn != priorPending.ResolvedTurn {
					t.Error("explicit fact metadata changed lifecycle phase or resolved turn")
				}
				writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
				reviewR2Run(t, st, index+3, ex, reviewR2Quote)
				if len(st.savedStatusCurrent) != writes || len(st.savedStatusEvents) != events {
					t.Error("explicit linked-fact no-change replay manufactured extra write")
				}
			}
		})
	}
}

func TestLifecycleScopeR2ClosedMetadataProjectsClosed(t *testing.T) {
	for _, phase := range []string{"complete", "abandon", "cancel"} {
		t.Run(phase, func(t *testing.T) {
			st := &turnRecordingStore{}
			reviewR2Run(t, st, 1, reviewR2Extraction(reviewR2InstructionScope(), false, true), reviewR2Quote)
			lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Delivery closed", phase, "Courier closes the instruction.", reviewR2InstructionScope()), "claim")
			scope := reviewR2Scope([]string{"Coordinator", "Courier", "Observer"}, []string{"Outsider"})
			quote := "Courier reports the closed instruction to Observer."
			claim := reviewR2Claim("instruction-fact", "Delivery closed", "set", quote, scope)
			lifecycleScopeR2Save(t, st, 3, claim, "both")
			ex := normalizeCriticExtraction(map[string]any{"state_claims": []any{claim}, "pending_threads": []any{map[string]any{"title": "Parcel delivery instruction", "lifecycle_key": "instruction-fact", "description": "Delivery closed", "evidence_excerpt": quote, "confidence": 0.99}}})
			before := mustCompactJSON(ex)
			result := artifactSaveResult{}
			(&Server{Store: st}).saveCharacterAndStateArtifacts(acceptedPreciseMemoryContext("synthetic-lifecycle-scope-3"), reviewR2SID, 3, ex, "Before. "+quote+" After.", completeTurnEmbeddingConfig{}, time.Unix(3, 0), &result, nil, &canonicalStateWriteCostMeasurement{})
			if result.Errors != 0 || len(st.returnPendingThreads) == 0 || len(st.savedActiveStates) == 0 || len(st.savedCanonicalLayers) == 0 || len(st.savedStorylines) == 0 {
				t.Fatalf("positive production projections missing/failed: %+v", result)
			}
			// Accepted-source pending persistence is part of the atomic Store
			// transition, represented above by its stored current snapshot.
			// The dependent projector must not add an independent pending writer.
			if len(st.savedPendingThreads) != 0 {
				t.Error("dependent projection introduced an independent pending writer")
			}
			pending := st.returnPendingThreads[len(st.returnPendingThreads)-1]
			if pending.Status != "resolved" || pending.ResolvedTurn != 2 || mustCompactJSON(parseJSONMap(pending.HookMetadataJSON)["knowledge_scope"]) != mustCompactJSON(scope) {
				t.Error("pending projection lost closed phase/current scope")
			}
			for _, content := range []string{st.savedActiveStates[len(st.savedActiveStates)-1].Content, st.savedCanonicalLayers[len(st.savedCanonicalLayers)-1].Content} {
				if stringFromMap(parseJSONMap(content), "status") != "resolved" {
					t.Error("current active/canonical projection reopened")
				}
			}
			if st.savedStorylines[len(st.savedStorylines)-1].Status != "resolved" || before != mustCompactJSON(ex) {
				t.Error("storyline reopened or canonical projection input mutated")
			}
		})
	}
}
