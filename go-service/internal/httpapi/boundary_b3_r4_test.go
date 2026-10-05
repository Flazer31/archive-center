package httpapi

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestBoundaryB3R4SourceRefsSurviveSave(t *testing.T) {
	for _, field := range []string{"fact_refs", "source_refs"} {
		for _, reversed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reversed=%t", field, reversed), func(t *testing.T) {
				ex, content, ev := goalR3Fixture([]string{"F41", "F42"}, nil, reversed, false)
				for _, lane := range []string{"state_claims", "pending_threads"} {
					item := mapFromAny(sliceFromAny(ex[lane])[0])
					item[field] = item["fact_refs"]
					if field != "fact_refs" {
						delete(item, "fact_refs")
					}
				}
				ex = normalizeCriticExtraction(ex)
				before := mustCompactJSON(ex)
				st, result := &turnRecordingStore{}, artifactSaveResult{}
				srv := &Server{Store: st}
				ctx := acceptedPreciseMemoryContext("synthetic-goal-r3")
				srv.saveNarrativeStateFromExtraction(ctx, reviewR2SID, 1, ex, content, ev, time.Unix(1, 0), &result)
				units := srv.buildPreciseMemoryUnitsFromExtraction(ctx, reviewR2SID, 1, ex, content, ev, nil, time.Unix(1, 0), &result)
				if result.Errors != 0 {
					t.Fatalf("SETUP_FAILURE: production save: %+v", result)
				}
				current, pending := lifecycleScopeR2Current(t, st)
				for _, p := range []map[string]any{mapFromAny(current["lifecycle_details"]), parseJSONMap(pending.HookMetadataJSON), parseJSONMap(pending.DetailsJSON)} {
					goalR3AssertBoundaries(t, sliceFromAny(p["knowledge_boundaries"]), ex, content, ev)
					if len(mapFromAny(p["knowledge_scope"])) != 0 {
						t.Fatal("multiple linked facts selected or unioned a claim scope")
					}
				}
				found := false
				for _, u := range units {
					if u.Kind == "state" && u.Subtype == "goal_status" {
						found = true
						p := parseJSONMap(u.PayloadJSON)
						goalR3AssertBoundaries(t, sliceFromAny(p["knowledge_boundaries"]), ex, content, ev)
						continuityR3Delivery(t, *u, p["knowledge_boundaries"])
					}
				}
				if !found {
					t.Fatal("SETUP_FAILURE: precise goal missing")
				}
				pending.ID = 23
				in := carry49Input(store.Memory{})
				in.Memories, in.PendingThreads, in.UserInput = nil, []store.PendingThread{pending}, "Combined amber contingency"
				out := buildPrepareTurnInjectionAssemblyWithBudget(in)
				candidates, _ := multiAgentCandidatePool(&out)
				found = false
				for _, c := range candidates {
					if c.SourceTable == "pending_threads" {
						found = true
						boundaries := []any{}
						for _, b := range c.KnowledgeBoundaries {
							boundaries = append(boundaries, b)
						}
						goalR3AssertBoundaries(t, boundaries, ex, content, ev)
						main := carryDirectionR2Main(t, &out, c)
						for _, boundary := range c.KnowledgeBoundaries {
							if !strings.Contains(main, mustCompactJSON(boundary)) {
								t.Fatal("pending-only final lost individual source fact scope")
							}
						}
					}
				}
				if !found {
					t.Fatal("SETUP_FAILURE: pending-only candidate missing")
				}
				if before != mustCompactJSON(ex) {
					t.Fatal("accepted source input mutated")
				}
			})
		}
	}
}

func TestBoundaryB3R4CurrentFactScopeOverridesPredecessor(t *testing.T) {
	for _, disclosed := range []bool{false, true} {
		for _, field := range []string{"lifecycle_key", "fact_refs", "source_refs", "source_ref"} {
			t.Run(fmt.Sprintf("disclosed=%t/link=%s", disclosed, field), func(t *testing.T) {
				st := &continuityR3Store{originLineageRecordingStore: &originLineageRecordingStore{turnRecordingStore: &turnRecordingStore{}}}
				lifecycleScopeR2Save(t, st.turnRecordingStore, 1, reviewR2Claim("instruction-fact", "Delivery accepted", "create", reviewR2Quote, reviewR2InstructionScope()), "both")
				const quote = "Coordinator publicly explains the delivery instruction to Outsider."
				claim := reviewR2Claim("instruction-fact", "Delivery accepted", "reaffirm", quote, nil)
				secret := reviewR2Secret("instruction-fact", "Courier delivery instruction", reviewR2Scope([]string{"Coordinator", "Courier", "Outsider"}, []string{}))
				secret["evidence_excerpt"] = quote
				if field != "lifecycle_key" {
					secret["lifecycle_key"] = "distinct-record"
					if field == "source_ref" {
						claim[field] = secret["secret_id"]
					} else {
						claim[field] = []string{stringFromMap(secret, "secret_id")}
					}
				}
				if disclosed {
					secret["disclosure_policy"] = "public"
					mapFromAny(secret["knowledge_scope"])["publicly_revealed"] = true
				}
				pending := cloneMapAny(claim)
				pending["title"], pending["description"] = "Parcel delivery instruction", "Delivery accepted"
				ex := normalizeCriticExtraction(map[string]any{"state_claims": []any{claim}, "pending_threads": []any{pending}, "protected_secrets": []any{secret}, "evidence_excerpts": []string{quote}})
				secret = mapFromAny(sliceFromAny(ex["protected_secrets"])[0])
				if protectedSecretRequiresGuard(secret, "disclosure_policy") == disclosed {
					t.Fatal("SETUP_FAILURE: public/guarded control missing")
				}
				before, want := mustCompactJSON(ex), mustCompactJSON(secret["knowledge_scope"])
				res := (&Server{Store: st}).saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("synthetic-b3-r4-current"), reviewR2SID, 2, ex, "Before. "+quote+" After.", completeTurnEmbeddingConfig{}, time.Unix(2, 0))
				if res.Errors != 0 {
					t.Fatalf("SETUP_FAILURE: production save: %+v", res)
				}
				found := false
				for _, u := range st.units {
					p := parseJSONMap(u.PayloadJSON)
					if u.Kind != "state" || stringFromMap(p, "lifecycle_key") != "instruction-fact" {
						continue
					}
					found = true
					if mustCompactJSON(p["knowledge_scope"]) != want {
						t.Fatalf("current precise scope differs from supplied same fact: want=%s got=%s", want, u.PayloadJSON)
					}
					semantic, ok := prepareTurnPrioritySemanticFactFromPreciseUnit(*u, 0.8, "synthetic external vector boundary")
					if !ok {
						t.Fatal("SETUP_FAILURE: precise semantic source missing")
					}
					out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{MaxChars: 36000, TopK: 5, ProtectedSecretBudgetChars: 4000})
					prepareTurnResolvePrioritySourcePool(&out, "Courier delivery", nil, 3, []prepareTurnPrioritySemanticFact{semantic})
					candidates, _ := multiAgentCandidatePool(&out)
					if len(candidates) != 1 {
						t.Fatalf("SETUP_FAILURE: precise-only candidates=%d", len(candidates))
					}
					c := candidates[0]
					out.Preprocessing = &multiAgentSelection{Candidates: []prepareTurnPriorityMemoryCandidate{c}, Roles: []multiAgentRoleResult{{Role: c.Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{c.CanonicalFactID}, Reasons: map[string]string{c.CanonicalFactID: "Synthetic explicit selection"}}}}}
					plan := renderPrepareTurnPriorityMemoryDeliveryPlan(&out, 36000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: "Courier delivery", CurrentTurn: 3}, nil, nil, nil, nil)
					main := stringFromMap(plan, "main_memory_text")
					if !strings.Contains(main, "Delivery accepted") || !strings.Contains(main, want) {
						t.Fatalf("final lost current explicit scope: %s", main)
					}
					if strings.Contains(main, `"unknown_to":["Outsider"]`) {
						t.Fatalf("final resurrected predecessor restriction: %s", main)
					}
				}
				if !found {
					t.Fatal("SETUP_FAILURE: current precise goal missing")
				}
				if before != mustCompactJSON(ex) {
					t.Fatal("accepted source input mutated")
				}
			})
		}
	}
}

func TestBoundaryB3R4UnlinkedPublicQuotationUnchanged(t *testing.T) {
	ex, content, ev := goalR3Fixture(nil, nil, false, true)
	for _, raw := range sliceFromAny(ex["protected_secrets"]) {
		mapFromAny(raw)["disclosure_policy"] = "public"
		mapFromAny(mapFromAny(raw)["knowledge_scope"])["publicly_revealed"] = true
	}
	payload := cloneMapAny(mapFromAny(sliceFromAny(ex["state_claims"])[0]))
	before := mustCompactJSON(payload)
	retainExactGoalKnowledgeMetadata(payload, ex, content, ev, reviewR2SID, 1, "synthetic-b3-r4-unlinked", stringFromMap(payload, "evidence_excerpt"))
	if before != mustCompactJSON(payload) {
		t.Fatal("unlinked public quotation acquired protected authority")
	}
}
