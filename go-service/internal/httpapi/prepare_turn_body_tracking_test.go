package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func bodyDeliveryFixture46() (*prepareTurnBodyTrackingContext, map[string]any, store.CharacterState) {
	c := defaultBodyCharacterConfig()
	c.EntityID, c.CharacterName = "mira-id", "Mira"
	c.Cycle.ReferenceTime = map[string]any{"date": "2024-01-01"}
	period := map[string]any{"kind": "period_start", "visibility": "private", "occurred_at": map[string]any{"date": "2024-01-03"}, "source": map[string]any{"source_turn": 6, "source_unit_id": "period-start", "evidence_excerpt": "Mira recorded the start of her period."}}
	ended := map[string]any{"kind": "recovery_ended", "status": "ended", "visibility": "private", "occurred_at": map[string]any{"date": "2024-01-08"}, "source": map[string]any{"source_turn": 8, "source_unit_id": "recovery-end", "evidence_excerpt": "Mira completed her recovery."}}
	state := map[string]any{"contract_version": bodyTrackingStateContract, "subject_entity_id": c.EntityID, "subject_label": c.CharacterName, "observed_facts": map[string]any{"period_start": period}, "cycle_reference": period["occurred_at"], "recovery": ended}
	current := store.StatusCurrentValue{ID: 746, ChatSessionID: "body46", StatusKey: bodyTrackingStatusKey, OwnerScope: reversibleStateOwnerScope, OwnerID: c.EntityID, SourceTurn: 8, WriteState: "current", ValueJSON: mustCompactJSON(state)}
	input := &prepareTurnBodyTrackingContext{SessionID: "body46", Config: bodyTrackingConfig{CycleTrackingEnabled: true, Characters: []bodyCharacterConfig{c}}, Values: []store.StatusCurrentValue{current}}
	clock := map[string]any{"version": storyClockContractVersion, "observation_kind": "absolute", "scene_scope": "current", "precision": "exact", "absolute": map[string]any{"date": "2024-01-15"}, "transition": "set", "evidence_excerpt": "The calendar says January fifteenth."}
	return input, clock, store.CharacterState{ID: 747, ChatSessionID: "body46", CharacterName: "Mira", AppearanceJSON: `{"gender":"female"}`, TurnIndex: 19}
}

func bodyAssemblyInput46(body *prepareTurnBodyTrackingContext, clock map[string]any, character store.CharacterState) prepareTurnAssemblyInput {
	input := prepareTurnAssemblyInput{CharacterStates: []store.CharacterState{character}, TopK: 1, MaxChars: 18000, UserInput: "Mira checks her calendar and recovery.", BudgetMode: "auto", Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	input.Perspective.BodyTracking, input.Perspective.StoryClock = body, clock
	input.Perspective.Selection.Query, input.Perspective.Selection.CurrentTurn = input.UserInput, 21
	return input
}

func assertBodyReading46(t *testing.T, text string, interrupted ...bool) {
	t.Helper()
	forecast := "next_period_estimate"
	if len(interrupted) > 0 && interrupted[0] {
		forecast = "ordinary_cycle_model_not_applicable_during_modeled_pregnancy"
		if strings.Contains(text, "next_period_estimate") || strings.Contains(text, "fertile_window_estimate") {
			t.Error("ordinary cyclic forecast survived an eligible modeled pregnancy")
		}
	}
	for _, want := range []string{"calculated_estimate", "observed_fact", "knowledge_not_inferred", "subtext_only_do_not_reveal_private_fact", "Mira recorded the start of her period", "Mira completed her recovery", "2024-01-03", "2024-01-15", forecast} {
		if !strings.Contains(text, want) {
			t.Errorf("body delivery lost %q", want)
		}
	}
}

func Test46BodyDeliveryUsesSharedReadingBudgetAndSupplement(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	before := mustCompactJSON(body)
	input := bodyAssemblyInput46(body, clock, character)
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	assertBodyReading46(t, extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]))
	assert45Budget(t, out.MemoryDeliveryPlan, input.MaxChars)
	facts, _ := multiAgentCandidatePool(&out)
	if len(facts) != 3 {
		t.Fatalf("wanted two body facts and one model reading, got %d", len(facts))
	}
	for _, fact := range facts {
		if fact.Lane != "subjective_relationship" || fact.Visibility != "owner_private" || fact.PerspectiveOwner != "mira-id" || fact.Minimum == nil || fact.Minimum.Chars == 0 {
			t.Fatalf("body reading bypassed shared privacy/budget metadata: %#v", fact)
		}
	}
	projected := out.supplementProjection(nil, input.Perspective.Selection)
	plan := buildPrepareTurnMemoryDeliveryPlan(&projected, input.MaxChars, input.Perspective.Selection)
	assertBodyReading46(t, extractionStringFromAny(plan["final_text"]))
	if mustCompactJSON(body) != before {
		t.Fatal("request calculation changed settings or body facts")
	}
	for _, cap := range []int{100, 700, 18000} {
		input.MaxChars = cap
		limited := buildPrepareTurnInjectionAssemblyWithBudget(input)
		assert45Budget(t, limited.MemoryDeliveryPlan, cap+bodyTrackingAdditionalBudgetChars)
	}
}

func Test46BodyDeliveryDefaultOffAndExistingHolderPrivacy(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	input := bodyAssemblyInput46(body, clock, character)
	body.Config.CycleTrackingEnabled = false
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	if strings.Contains(mustCompactJSON(out.MemoryDeliveryPlan), "body_tracking_settings") || strings.Contains(mustCompactJSON(out.MemoryDeliveryPlan), "Mira recorded") {
		t.Fatal("default-off feature delivered body context")
	}
	body.Config.CycleTrackingEnabled = true
	payload := parseJSONMap(body.Values[0].ValueJSON)
	period := mapFromAny(mapFromAny(payload["observed_facts"])["period_start"])
	period["visibility"] = "restricted"
	period["occurred_at"] = map[string]any{"date": "1888-01-01"}
	payload["cycle_reference"] = period["occurred_at"]
	body.Values[0].ValueJSON = mustCompactJSON(payload)
	input.Perspective.Public = map[string]any{"current_pov": "Rowan", "current_pov_entity_id": "rowan-id", "identity_state": "resolved"}
	out = buildPrepareTurnInjectionAssemblyWithBudget(input)
	if strings.Contains(mustCompactJSON(out.MemoryDeliveryPlan), "1888") || strings.Contains(mustCompactJSON(out.MemoryDeliveryPlan), "Mira recorded") {
		t.Fatal("restricted fact or derived cycle anchor leaked outside its holder")
	}
	if !strings.Contains(extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]), "calculated_estimate") {
		t.Fatal("unknown private fact suppressed the independent author model")
	}
	input.Perspective.Public = map[string]any{"current_pov": "Mira", "current_pov_entity_id": "mira-id", "identity_state": "resolved"}
	out = buildPrepareTurnInjectionAssemblyWithBudget(input)
	if !strings.Contains(mustCompactJSON(out.MemoryDeliveryPlan), "1888") || !strings.Contains(mustCompactJSON(out.MemoryDeliveryPlan), "pov_private_do_not_leak_outside_holder") {
		t.Fatal("existing matching holder policy lost restricted body fact")
	}
}

func Test46HTTPBodyCycleReachesAllOptionalAIModes(t *testing.T) {
	for _, pregnancy := range []bool{false, true} {
		t.Run(fmt.Sprintf("pregnancy-%v", pregnancy), func(t *testing.T) {
			for _, preprocess := range []bool{false, true} {
				for _, publisher := range []bool{false, true} {
					for _, mode := range []string{"normal", "empty", "failure"} {
						if !preprocess && !publisher && mode != "normal" {
							continue
						}
						t.Run(fmt.Sprintf("preprocess-%v/publisher-%v/%s", preprocess, publisher, mode), func(t *testing.T) {
							t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
							agentCalls, publisherCalls := 0, 0
							provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								var body map[string]any
								_ = json.NewDecoder(r.Body).Decode(&body)
								messages := outputFidelityLineageSlice(body["messages"])
								if len(messages) < 2 {
									t.Error("unexpected body provider request")
									http.Error(w, "unexpected", 400)
									return
								}
								var input map[string]any
								_ = json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input)
								assertBodyReading46(t, mustCompactJSON(input), pregnancy)
								if pregnancy {
									assertBodyPregnancyReading46(t, mustCompactJSON(input), "modeled_pre_implantation")
								}
								_, isPublisher := input["supervisor_support_packet"]
								if isPublisher {
									publisherCalls++
								} else {
									agentCalls++
								}
								if mode == "failure" {
									http.Error(w, "fixture unavailable", 400)
									return
								}
								if mode == "empty" {
									_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
									return
								}
								if isPublisher {
									_, _ = w.Write([]byte(publisherV3OpenAIResponse("status_current_values:746", "Keep narrator body estimates separate from knowledge.")))
									return
								}
								ids := []string{}
								for _, raw := range outputFidelityLineageSlice(input["candidates"]) {
									ids = append(ids, extractionStringFromAny(mapFromAny(raw)["ref"]))
								}
								expected := 3
								if pregnancy {
									expected++
								}
								if len(ids) != expected {
									t.Errorf("actual body preprocessing candidates=%d", len(ids))
								}
								_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": mustCompactJSON(map[string]any{"selected_ids": ids})}}}})
							}))
							defer provider.Close()
							cfg := config.Default()
							cfg.StoreMode, cfg.PromptDir = config.StoreModeDualShadow, filepath.Join("..", "..", "..", "prompts")
							s := NewServer(cfg)
							body, clock, character := bodyDeliveryFixture46()
							if pregnancy {
								addBodyPregnancyFixture46(body)
							}
							current := store.StatusCurrentValue{ID: 748, ChatSessionID: "body46", StatusKey: storyClockStatusKey, OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, SourceTurn: 20, ValueJSON: mustCompactJSON(clock)}
							st := &turnRecordingStore{returnCharStates: []store.CharacterState{character}, returnStatusCurrent: append(body.Values, current)}
							s.Store = &priorityPrepareTurnStore{turnRecordingStore: st}
							if _, err := s.saveBodyTrackingConfig("body46", body.Config); err != nil {
								t.Fatal(err)
							}
							s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint, s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = "custom", provider.URL, "fixture", "fixture-key"
							s.RuntimeConfig.SupervisorTimeoutSec = 30
							settings := defaultMultiAgentSettings()
							settings.Enabled = preprocess
							for role, c := range settings.Roles {
								c.Enabled, c.UsePublisher = role == "subjective_relationship", false
								c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
								settings.Roles[role] = c
							}
							encoded, _ := json.Marshal(settings)
							rec := httptest.NewRecorder()
							s.handleMultiAgentSettings(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(encoded)))
							mux := http.NewServeMux()
							s.RegisterRoutes(mux)
							encoded, _ = json.Marshal(map[string]any{"chat_session_id": "body46", "turn_index": 21, "raw_user_input": "Mira checks her calendar and recovery.", "settings": map[string]any{"injection_enabled": true, "max_injection_chars": 18000, "supervisor_enabled": publisher, "guide_mode": "standard", "guide_strength": "strong"}})
							rec = httptest.NewRecorder()
							mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(encoded)))
							if rec.Code != 200 {
								t.Fatal(rec.Body.String())
							}
							var response map[string]any
							_ = json.Unmarshal(rec.Body.Bytes(), &response)
							plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
							bodyBudget := mapFromAny(plan["body_tracking_budget"])
							if intFromAny(bodyBudget["cap_chars"], 0) != 3000 || intFromAny(bodyBudget["used_chars"], 0) > 3000 {
								t.Fatal("optional AI route changed the body allocation")
							}
							bodyLaneFound := false
							for _, raw := range outputFidelityLineageSlice(mapFromAny(response["payload_application_plan"])["lanes"]) {
								lane := mapFromAny(raw)
								if lane["key"] == "body_tracking" {
									bodyLaneFound = true
									if intFromAny(lane["budget_chars"], 0) != 3000 || intFromAny(lane["used_chars"], 0) > 3000 {
										t.Fatal("body payload budget changed")
									}
								}
								if lane["key"] == "long_term_memory" && strings.Contains(stringFromMap(lane, "text"), "/body_tracking/") {
									t.Fatal("body charged twice to main memory")
								}
							}
							if !bodyLaneFound {
								t.Fatal("body payload lane missing")
							}
							assertBodyReading46(t, extractionStringFromAny(plan["final_text"]), pregnancy)
							assertBodyReading46(t, mustCompactJSON(response["payload_application_plan"]), pregnancy)
							if pregnancy {
								assertBodyPregnancyReading46(t, extractionStringFromAny(plan["final_text"]), "modeled_pre_implantation")
								assertBodyPregnancyReading46(t, mustCompactJSON(response["payload_application_plan"]), "modeled_pre_implantation")
							}
							assert45Budget(t, plan, 18000)
							if len(st.savedStatusCurrent)+len(st.savedStatusEvents)+len(st.savedCharacterStates) > 0 {
								t.Fatal("body reading wrote state")
							}
							if (preprocess && agentCalls != 1) || (!preprocess && agentCalls != 0) || (publisher && publisherCalls != 1) || (!publisher && publisherCalls != 0) {
								t.Fatalf("provider calls preprocessing=%d publisher=%d", agentCalls, publisherCalls)
							}
						})
					}
				}
			}
		})
	}
}

func addBodyPregnancyFixture46(body *prepareTurnBodyTrackingContext) {
	body.Config.AutomaticPregnancyEnabled = true
	payload := parseJSONMap(body.Values[0].ValueJSON)
	model := map[string]any{"model_version": bodyPregnancyModelVersion, "reference_family": "RAW-INTERNAL-FAMILY", "visibility": "private", "ovulation_time": map[string]any{"date": "2024-01-12"}, "implantation_time": map[string]any{"date": "2024-01-20"}, "latent_success": true, "day_draw": map[string]any{"token": "RAW-DRAW-TOKEN", "fraction": .01}, "source": map[string]any{"source_turn": 10, "source_unit_id": "model-exposure"}}
	payload["modeled_pregnancy"] = model
	payload["latest_model_result"] = map[string]any{"status": "evaluated", "outcome": "latent_model_implantation", "selected_pregnancy": model, "visibility": "private", "model_cycles": []any{map[string]any{"token": "RAW-CYCLE-TOKEN", "timeline": "RAW-TIMELINE"}}}
	body.Values[0].ValueJSON = mustCompactJSON(payload)
}

func assertBodyPregnancyReading46(t *testing.T, text, stage string) {
	t.Helper()
	for _, want := range []string{"fiction_simulation", stage, "2024-01-12", "2024-01-20", "2024-01-15", "symptoms", "not_inferred", "Explicit observed body facts take precedence"} {
		if !strings.Contains(text, want) {
			t.Errorf("model delivery lost %q", want)
		}
	}
	for _, forbidden := range []string{"RAW-", "day_draw", "model_cycles", "simulation_seed", "reference_family"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("internal model data leaked: %s", forbidden)
		}
	}
}

func Test46BodyPregnancyDeliveryStagesPrivacyOffAndExplicitEnding(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	addBodyPregnancyFixture46(body)
	original := mustCompactJSON(body)
	for _, scene := range []struct{ date, stage string }{{"2024-01-10", "future_modeled_potential"}, {"2024-01-15", "modeled_pre_implantation"}, {"2024-01-25", "modeled_implanted_pregnancy"}} {
		clock["absolute"] = map[string]any{"date": scene.date}
		out := buildPrepareTurnInjectionAssemblyWithBudget(bodyAssemblyInput46(body, clock, character))
		text := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
		if !strings.Contains(text, scene.stage) || strings.Contains(text, "RAW-") {
			t.Fatalf("wrong story-time model stage %s: %s", scene.stage, text)
		}
	}
	if mustCompactJSON(body) != original {
		t.Fatal("reading resampled or mutated saved model")
	}
	body.Config.AutomaticPregnancyEnabled = false
	out := buildPrepareTurnInjectionAssemblyWithBudget(bodyAssemblyInput46(body, clock, character))
	text := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	if strings.Contains(text, "fiction_simulation") || !strings.Contains(text, "observed_fact") || !strings.Contains(text, "calculated_estimate") {
		t.Fatal("pregnancy OFF changed independent facts/cycle delivery")
	}
	body.Config.AutomaticPregnancyEnabled = true
	payload := parseJSONMap(body.Values[0].ValueJSON)
	mapFromAny(payload["modeled_pregnancy"])["visibility"] = "restricted"
	mapFromAny(payload["latest_model_result"])["visibility"] = "restricted"
	body.Values[0].ValueJSON = mustCompactJSON(payload)
	input := bodyAssemblyInput46(body, clock, character)
	input.Perspective.Public = map[string]any{"current_pov": "Rowan", "current_pov_entity_id": "rowan-id", "identity_state": "resolved"}
	out = buildPrepareTurnInjectionAssemblyWithBudget(input)
	if strings.Contains(mustCompactJSON(out.MemoryDeliveryPlan), "2024-01-20") || strings.Contains(mustCompactJSON(out.MemoryDeliveryPlan), "latent_model_implantation") {
		t.Fatal("restricted model crossed holder boundary")
	}
	delete(payload, "modeled_pregnancy")
	mapFromAny(payload["latest_model_result"])["visibility"] = "private"
	payload["pregnancy"] = map[string]any{"kind": "pregnancy_ended", "status": "ended", "visibility": "private", "source": map[string]any{"evidence_excerpt": "Observed pregnancy ended.", "source_turn": 18}}
	body.Values[0].ValueJSON = mustCompactJSON(payload)
	out = buildPrepareTurnInjectionAssemblyWithBudget(bodyAssemblyInput46(body, clock, character))
	text = extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	if strings.Contains(text, "modeled_implanted_pregnancy") || strings.Contains(text, "2024-01-20") || !strings.Contains(text, "Observed pregnancy ended") {
		t.Fatal("old selected model resurrected explicitly ended pregnancy")
	}
}

func Test46BodyModelSettingsViewContainsNoLatentDraws(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	addBodyPregnancyFixture46(body)
	clockValue := store.StatusCurrentValue{ID: 748, ChatSessionID: "body46", OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, StatusKey: storyClockStatusKey, ValueJSON: mustCompactJSON(clock)}
	s := &Server{Store: &turnRecordingStore{returnCharStates: []store.CharacterState{character}, returnStatusCurrent: append(body.Values, clockValue)}}
	view := s.bodyTrackingSettingsView(t.Context(), "body46", body.Config)
	assertBodyPregnancyReading46(t, mustCompactJSON(view["model_readings"]), "modeled_pre_implantation")
	for _, forbidden := range []string{"RAW-", "day_draw", "model_cycles", "simulation_seed", "reference_family"} {
		if strings.Contains(mustCompactJSON(view), forbidden) {
			t.Fatalf("settings view exposed %s", forbidden)
		}
	}
}

func Test46BodyKnowledgeRemainsInExistingHolderLane(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	input := bodyAssemblyInput46(body, clock, character)
	unit := store.PreciseMemoryUnit{UnitID: "mira-knowledge", Kind: "observation", LifecycleState: "active", EpistemicMode: "suspected", KnowledgeHolderEntityID: "mira-id", SourceTurnEnd: 9,
		PayloadJSON: `{"contract_version":"perspective_memory.v1","knowledge_holder_entity_id":"mira-id","epistemic_state":"suspected","subject":"Mira","state_slot":"body_knowledge","claim":"MIRA-SUSPICION-ONLY from a private conversation"}`}
	input.Perspective.Public = map[string]any{"current_pov": "Mira", "current_pov_entity_id": "mira-id", "identity_state": "resolved"}
	packet, text := buildCharacterPerspectivePacket([]store.PreciseMemoryUnit{unit}, input.Perspective.Public, input.MaxChars)
	input.Perspective.CharacterText, input.Perspective.CharacterCount = text, intFromAny(packet["candidate_count"], 0)
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	final := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	if !strings.Contains(final, "suspected | Mira") || !strings.Contains(final, "MIRA-SUSPICION-ONLY") || !strings.Contains(final, "knowledge_not_inferred") {
		t.Fatal("existing suspected knowledge was lost or conflated with body context")
	}
	body.Config.CycleTrackingEnabled = false
	out = buildPrepareTurnInjectionAssemblyWithBudget(input)
	if !strings.Contains(extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]), "MIRA-SUSPICION-ONLY") {
		t.Fatal("turning off calculations suppressed independently stored character knowledge")
	}
	input.Perspective.Public = map[string]any{"current_pov": "Rowan", "current_pov_entity_id": "rowan-id", "identity_state": "resolved"}
	_, input.Perspective.CharacterText = buildCharacterPerspectivePacket([]store.PreciseMemoryUnit{unit}, input.Perspective.Public, input.MaxChars)
	out = buildPrepareTurnInjectionAssemblyWithBudget(input)
	if strings.Contains(extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]), "MIRA-SUSPICION-ONLY") {
		t.Fatal("body feature moved suspicion to a different holder")
	}
}

func Test46ParentageDeliveryKeepsPrivateAssociationAndBodyBudget(t *testing.T) {
	srv, _, cfg := bodyModelState46Fixture(t, 1)
	event := bodyExposure46("counterpart", "1423-01-14")
	event["partners"] = []any{map[string]any{"entity_id": "arin-id", "character_name": "PrivateArin"}}
	saveBodyEvent46(t, srv, 1, event)
	confirmed := bodyEvent46("pregnancy_confirmed", "confirmation", "1423-02-01")
	confirmed["visibility"] = "public"
	saveBodyEvent46(t, srv, 2, confirmed)
	current := bodyCurrent46(t, srv)
	body := &prepareTurnBodyTrackingContext{SessionID: "body-session", Config: cfg, Values: []store.StatusCurrentValue{current}}
	clock := map[string]any{"date": "1423-02-02"}
	input := bodyAssemblyInput46(body, clock, store.CharacterState{ChatSessionID: "body-session", CharacterName: "Mina", AppearanceJSON: `{"gender":"female"}`})
	input.UserInput = "Mina checks her pregnancy."
	input.Perspective.Selection.Query = input.UserInput
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	text := stringFromMap(out.MemoryDeliveryPlan, "final_text")
	if !strings.Contains(text, "PrivateArin") || !strings.Contains(text, "modeled_link") || !strings.Contains(text, "knowledge_not_inferred") {
		t.Fatalf("parentage absent from actual delivery: %s", text)
	}
	facts, _ := multiAgentCandidatePool(&out)
	found := false
	for _, fact := range facts {
		if strings.Contains(mustCompactJSON(fact), "PrivateArin") {
			found = true
			if fact.Visibility == "public" || fact.PerspectiveOwner != "mina-id" {
				t.Fatalf("private association leaked via public pregnancy: %#v", fact)
			}
		}
	}
	if !found {
		t.Fatal("no parentage candidate")
	}
	for _, seed := range out.PriorityFactSeeds {
		if strings.Contains(seed.Fact.Text, "PrivateArin") && seed.Fact.MemoryRole != "fiction_simulation" {
			t.Fatalf("model parentage was promoted to observed fact: %s", seed.Fact.Text)
		}
	}
	if strings.Contains(text, "basis_events") || strings.Contains(text, "arin-id") || strings.Contains(text, "day_draw") {
		t.Fatal("compact parentage leaked internal ledger/IDs")
	}
	assert45Budget(t, out.MemoryDeliveryPlan, input.MaxChars+bodyTrackingAdditionalBudgetChars)
	cfg.AutomaticPregnancyEnabled = false
	cfg.CycleTrackingEnabled = false
	body.Config = cfg
	off := buildPrepareTurnInjectionAssemblyWithBudget(input)
	if strings.Contains(mustCompactJSON(off.MemoryDeliveryPlan), "PrivateArin") {
		t.Fatal("off feature injected parentage")
	}
}
