package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test43MemoryDeliveryBreadthBeyondCore(t *testing.T) {
	rules := []store.WorldRule{}
	details := []string{"lender Rook", "advance fifty crowns", "interest five crowns", "repayment after harvest", "first batch quality", "reimbursable fuel", "grain supplied by lender", "southern gate delivery", "blue wax seal", "sample in guild vault", "witness Mira", "guarantee ends in winter"}
	for i, detail := range details {
		rules = append(rules, store.WorldRule{ID: int64(i + 1), ChatSessionID: "contract", Scope: "root", Key: fmt.Sprintf("clause_%02d", i), ValueJSON: fmt.Sprintf("%q", detail), SourceTurn: 10})
	}
	for _, core := range []int{1, 5, len(details)} {
		a := buildPrepareTurnInjectionAssembly(nil, nil, nil, nil, nil, rules, nil, nil, nil, nil, nil, nil, nil, 5, 12000, "Recall the contract terms.", "default", nil, nil, nil, priorityMemoryTestContext(core))
		text := extractionStringFromAny(a.MemoryDeliveryPlan["final_text"])
		for _, detail := range details {
			if !strings.Contains(text, detail) {
				t.Errorf("core=%d discarded a fitting detail %q: %s", core, detail, text)
			}
		}
		if len([]rune(text)) > 12000 {
			t.Fatal("breadth exceeded the configured budget")
		}
	}
}

func deliveryRepairPerspectiveUnit(id, holder, subtype, claim string, turn int) store.PreciseMemoryUnit {
	payload := map[string]any{"contract_version": "perspective_memory.v1", "knowledge_holder_entity_id": holder, "knowledge_holder": holder, "epistemic_state": "known", "subject": holder, "state_slot": subtype, "claim": claim}
	return store.PreciseMemoryUnit{UnitID: id, ChatSessionID: "perspective", Kind: "observation", Subtype: subtype, PayloadJSON: mustCompactJSON(payload), KnowledgeHolderEntityID: holder, EpistemicMode: "known", AdmissionState: "committed", ReviewState: "source_observed", LifecycleState: "active", Visibility: "owner_private", SourceTurnStart: turn, SourceTurnEnd: turn}
}

func Test43MemoryDeliveryPerspectiveUsesNormalSelection(t *testing.T) {
	units := []store.PreciseMemoryUnit{}
	for i := 0; i < 96; i++ {
		units = append(units, deliveryRepairPerspectiveUnit(fmt.Sprintf("experience-%d", i), "holder", "subjective_memory", fmt.Sprintf("Experience %d: %s", i, strings.Repeat("The garden work left a personal impression. ", 3)), i+1))
	}
	units = append(units, deliveryRepairPerspectiveUnit("secret", "holder", "protected_knowledge", "The guild vault access phrase is amber.", 97))
	units = append(units, deliveryRepairPerspectiveUnit("other-holder", "outsider", "subjective_memory", "OUTSIDER_PRIVATE_TEXT", 98))
	context := map[string]any{"identity_state": "resolved", "current_pov_entity_id": "holder", "current_pov": "holder"}
	packet, candidateText := buildCharacterPerspectivePacket(units, context, 12000)
	context["_character_perspective_text"] = candidateText
	context["_character_perspective_fact_seeds"] = packet["_character_perspective_fact_seeds"]
	context["_priority_memory_enabled"], context["_priority_memory_max_items"], context["_priority_memory_current_turn"] = true, 2, 100
	a := buildPrepareTurnInjectionAssembly(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 5, 12000, "Review the garden work.", "default", nil, nil, nil, context)
	if strings.Contains(a.ProtectedMemoryText, "Experience ") {
		t.Fatal("ordinary experiences still occupy protected memory")
	}
	if !strings.Contains(a.ProtectedMemoryText, "guild vault access phrase") {
		t.Fatal("existing protected knowledge disappeared")
	}
	for _, lane := range multiAgentRoles {
		if lane != "subjective_relationship" {
			a.PriorityFactSeeds = append(a.PriorityFactSeeds, prepareTurnPriorityFactSeed{Lane: lane, SourceTable: "fixture_public_source", SourceOccurrence: lane, Fact: prepareTurnPriorityMemoryFact{Text: "Garden continuity for " + lane}, SourceTurn: 99, Visibility: "general"})
		}
	}
	baseline := buildPrepareTurnPriorityMemoryDeliveryPlan(&a, 12000, 2, "auto", nil, context)
	for _, lane := range multiAgentRoles {
		found := false
		for _, raw := range prepareTurnMemoryLineageSlice(baseline["classes"]) {
			item := mapFromAny(raw)
			if item["key"] == lane && intFromAny(item["selected_count"], 0) > 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("populated lane %s was starved", lane)
		}
	}
	facts, sums := multiAgentCandidatePool(&a, context)
	subjective := []prepareTurnPriorityMemoryCandidate{}
	for _, c := range facts {
		if c.Lane == "subjective_relationship" {
			subjective = append(subjective, c)
			if c.PerspectiveOwner != "holder" || len(c.AllowedViewers) != 1 || c.AllowedViewers[0] != "holder" || c.SourceTurn < 1 {
				t.Fatalf("scope or source lost: %+v", c)
			}
		}
	}
	if len(subjective) != 96 {
		t.Fatalf("ordinary candidate pool prematurely truncated: %d", len(subjective))
	}
	text := extractionStringFromAny(baseline["final_text"])
	if strings.Contains(text, "OUTSIDER_PRIVATE_TEXT") || len([]rune(text)) > 12000 {
		t.Fatal("private isolation or text budget changed")
	}
	finalPacket, _ := finalizeCharacterPerspectivePacket(packet, candidateText, text)
	if intFromAny(finalPacket["selected_count"], 0) < 2 {
		t.Fatal("perspective delivery observation no longer recognizes selected records")
	}
	if _, leaked := finalPacket["_character_perspective_fact_seeds"]; leaked {
		t.Fatal("internal candidate pool leaked into public packet")
	}
	for _, mode := range []string{"no_recommendation", "all_ai", "partial_failure"} {
		t.Run(mode, func(t *testing.T) {
			selection := &multiAgentSelection{Contract: multiAgentContract, Candidates: facts, Summaries: sums}
			selection.captureBaseline(baseline)
			for _, lane := range multiAgentRoles {
				role := multiAgentRoleResult{Role: lane, Source: "go_default"}
				if mode == "all_ai" || (mode == "partial_failure" && lane == "subjective_relationship") {
					role.Source = "ai"
					for i := len(facts) - 1; i >= 0; i-- {
						if facts[i].Lane == lane {
							role.Selection.SelectedIDs = append(role.Selection.SelectedIDs, facts[i].CanonicalFactID)
							if len(role.Selection.SelectedIDs) == 2 {
								break
							}
						}
					}
				}
				selection.Roles = append(selection.Roles, role)
			}
			a.Preprocessing = selection
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&a, 12000, 2, "auto", nil, context)
			if mode == "no_recommendation" && plan["final_text"] != baseline["final_text"] {
				t.Fatal("empty recommendations changed ordinary Go output")
			}
			for _, role := range selection.Roles {
				if role.Source != "ai" {
					continue
				}
				got := []string{}
				for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
					item := mapFromAny(raw)
					if item["lane"] == role.Role && item["selection_reason"] == "ai_recommendation" {
						got = append(got, extractionStringFromAny(item["canonical_fact_id"]))
					}
				}
				if !reflect.DeepEqual(got, role.Selection.SelectedIDs) {
					t.Fatalf("AI selection or order changed: got=%v want=%v", got, role.Selection.SelectedIDs)
				}
			}
		})
	}
}

func Test43MemoryDeliveryPerspectiveRetainsSourceTime(t *testing.T) {
	units := []store.PreciseMemoryUnit{deliveryRepairPerspectiveUnit("old-observation", "holder", "subjective_memory", "I had not learned of the investigation.", 40), deliveryRepairPerspectiveUnit("new-observation", "holder", "subjective_memory", "I learned of the investigation and grew wary.", 113)}
	_, text := buildCharacterPerspectivePacket(units, map[string]any{"identity_state": "resolved", "current_pov_entity_id": "holder"}, 12000)
	for _, unit := range units {
		if !strings.Contains(text, unit.UnitID) || !strings.Contains(text, fmt.Sprintf("source_turn=%d", unit.SourceTurnEnd)) {
			t.Errorf("source time/ID missing: %s", text)
		}
	}
}

func deliveryRepairStateAssembly(layers []store.CanonicalStateLayer) prepareTurnInjectionAssembly {
	q := "Is the home garden rapeseed still planned?"
	return buildPrepareTurnInjectionAssembly(nil, nil, nil, nil, nil, nil, nil, nil, layers, nil, nil, nil, nil, 5, 12000, q, "default", nil, nil, nil, map[string]any{"_priority_memory_enabled": true, "_priority_memory_max_items": 5, "_priority_memory_query": q, "_priority_memory_current_turn": 13})
}

func Test43MemoryDeliveryCurrentStateIndependentOfQuery(t *testing.T) {
	layers := []store.CanonicalStateLayer{{ID: 71, ChatSessionID: "state", LayerType: "scene_state", Content: `{"garden":{"rapeseed":{"status":"planned","location":"home"}}}`, TurnIndex: 10, SourceTurn: 10, LastVerifiedTurn: 10, Confidence: .9}, {ID: 72, ChatSessionID: "state", LayerType: "scene_state", Content: `{"garden":{"rapeseed":{"status":"completed","location":"home"}}}`, TurnIndex: 12, SourceTurn: 12, LastVerifiedTurn: 12, Confidence: .9}}
	for _, rows := range [][]store.CanonicalStateLayer{layers, {layers[1], layers[0]}} {
		a := deliveryRepairStateAssembly(rows)
		text := extractionStringFromAny(a.MemoryDeliveryPlan["final_text"])
		if !strings.Contains(text, "completed") || strings.Contains(text, "status: planned") {
			t.Errorf("query replaced the latest current field: %s", text)
		}
		facts, _ := multiAgentCandidatePool(&a, nil)
		found := false
		for _, c := range facts {
			found = found || strings.Contains(c.CompleteText, "completed")
		}
		if !found {
			t.Error("latest source absent before AI")
		}
	}
	for _, failSecond := range []bool{false, true} {
		t.Run(fmt.Sprintf("supplement_failure_%t", failSecond), func(t *testing.T) {
			first := deliveryRepairStateAssembly(layers[:1])
			searched := deliveryRepairStateAssembly(layers[1:])
			f1, s1 := multiAgentCandidatePool(&first, nil)
			f2, s2 := multiAgentCandidatePool(&searched, nil)
			oldID, newID := "", ""
			for _, c := range f1 {
				if strings.Contains(c.CompleteText, "planned") {
					oldID = c.CanonicalFactID
				}
			}
			for _, c := range f2 {
				if strings.Contains(c.CompleteText, "completed") {
					newID = c.CanonicalFactID
				}
			}
			calls, searches := 0, 0
			secondHasNew := false
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Messages) != 2 {
					t.Error("unexpected provider body")
					http.Error(w, "bad input", 400)
					return
				}
				var input map[string]any
				if err := json.Unmarshal([]byte(body.Messages[1].Content), &input); err != nil {
					t.Error(err)
					http.Error(w, "bad input", 400)
					return
				}
				answer := map[string]any{"selected_ids": []string{oldID}, "search_requests": []string{"Check the latest garden status."}}
				if input["previous_result"] != nil {
					for _, raw := range outputFidelityLineageSlice(input["candidates"]) {
						secondHasNew = secondHasNew || strings.Contains(extractionStringFromAny(mapFromAny(raw)["text"]), "completed")
					}
					if failSecond {
						http.Error(w, "fixture failure", 400)
						return
					}
					answer = map[string]any{"selected_ids": []string{newID}}
				}
				b, _ := json.Marshal(answer)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}})
			}))
			defer provider.Close()
			cfg := defaultMultiAgentSettings()
			cfg.Enabled = true
			for role, c := range cfg.Roles {
				c.Enabled = role == "world_state"
				c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
				cfg.Roles[role] = c
			}
			result := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, f1, s1, 12000, 5, nil, func(q string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
				searches++
				if q != "Check the latest garden status." {
					t.Errorf("unexpected search: %s", q)
				}
				return f2, s2, nil
			})
			if calls != 2 || searches != 1 || !secondHasNew {
				t.Fatalf("new source did not reach the second model: calls=%d searches=%d new=%t sameID=%t", calls, searches, secondHasNew, oldID == newID)
			}
			result.captureBaseline(first.MemoryDeliveryPlan)
			first.Preprocessing = result
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&first, 12000, 5, "auto", nil, nil)
			text := extractionStringFromAny(plan["final_text"])
			wanted := "completed"
			if failSecond {
				wanted = "planned"
			}
			if !strings.Contains(text, wanted) {
				t.Fatalf("accepted recommendation was replaced: %s", text)
			}
		})
	}
}
