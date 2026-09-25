package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func characterFields46Fixture() (store.CharacterState, store.StatusCurrentValue) {
	state := store.CharacterState{ID: 466, ChatSessionID: "fields46", CharacterName: "Mira", TurnIndex: 30, CreatedAt: time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC),
		StatusJSON:          `{"mobility":"needs a sling","capabilities":{"manual_dexterity":"one-handed"},"inventory":[{"item":"brass compass","count":1}],"rank":"surveyor"}`,
		RelationshipsJSON:   `{"Rowan":{"trust":"uneasy","shared_plan":"north quay"}}`,
		FieldProvenanceJSON: `{"contract_version":"character_field_provenance.v1","fields":{"/status/mobility":{"source_turn":10,"source_session_id":"origin","recorded_turn":10,"evidence_excerpt":"Mira supported the injured arm with a sling."},"/status/capabilities/manual_dexterity":{"source_turn":10,"effective_time":{"date":"2026-09-17","precision":"date"}},"/status/inventory":{"source_turn":25,"evidence_excerpt":"Mira held the brass compass."},"/relationships/Rowan/trust":{"source_turn":7,"learned_time":"after the first voyage"}}}`}
	current := store.StatusCurrentValue{ID: 467, ChatSessionID: "fields46", StatusKey: narrativeStateStatusKey, OwnerScope: "character", OwnerID: "Mira-mobility", SourceTurn: 28, WriteState: "current",
		ValueJSON:    `{"subject":"Mira","subject_type":"character","state_slot":"mobility","value":"full use of the left arm","transition":"recovery","claim_scope":"objective","source_fields":["/status/mobility","/status/capabilities/manual_dexterity"]}`,
		EvidenceJSON: `{"source_turn":28,"source_revision":"accepted-28","evidence_excerpt":"Mira lifted the crate freely with both hands."}`}
	return state, current
}

func characterFields46Assembly(state store.CharacterState, current []store.StatusCurrentValue) prepareTurnInjectionAssembly {
	input := prepareTurnAssemblyInput{CharacterStates: []store.CharacterState{state}, TopK: 5, MaxChars: 18000, BudgetMode: "auto", UserInput: "Mira and Rowan review Mira's mobility, compass and voyage plan.", Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	input.Perspective.Selection.Query, input.Perspective.Selection.CurrentTurn = input.UserInput, 31
	input.Perspective.NarrativeValues = current
	input.Perspective.ActiveStates = characterFields46Scene()
	return buildPrepareTurnInjectionAssemblyWithBudget(input)
}

func characterFields46Scene() []store.ActiveState {
	return []store.ActiveState{{StateType: "scene", TurnIndex: 30, Content: `{"present_entities":["Mira","Rowan"]}`}}
}

func Test46CharacterFieldsKeepObservationTimeAndLinkedCurrent(t *testing.T) {
	state, current := characterFields46Fixture()
	out := characterFields46Assembly(state, []store.StatusCurrentValue{current})
	facts, _ := multiAgentCandidatePool(&out)
	expected := map[string]int{"/state/mobility": 10, "/state/capabilities/manual_dexterity": 10, "/state/rank": 0, "/relationships/Rowan/trust": 7, "/relationships/Rowan/shared_plan": 0}
	seen := map[string]bool{}
	for _, fact := range facts {
		if strings.HasPrefix(fact.SourceFieldPath, "/state/inventory") {
			t.Fatal("legacy inventory bypassed the existing typed reversible-state boundary")
		}
		want, ok := expected[fact.SourceFieldPath]
		if !ok {
			continue
		}
		seen[fact.SourceFieldPath] = true
		if fact.SourceTurn != want {
			t.Errorf("field %s got row time %d, want observed source %d", fact.SourceFieldPath, fact.SourceTurn, want)
		}
		reading := prepareTurnMemoryReadingText(fact)
		if !strings.Contains(reading, "containing snapshot: turn 30 (does not date this field)") {
			t.Errorf("field lacks separate snapshot time: %s", reading)
		}
		if fact.SourceFieldPath == "/state/mobility" || fact.SourceFieldPath == "/state/capabilities/manual_dexterity" {
			for _, text := range []string{"Historical observation", "full use of the left arm", "Mira lifted the crate freely with both hands"} {
				if !strings.Contains(reading, text) {
					t.Errorf("field %s lacks linked current %s", fact.SourceFieldPath, text)
				}
			}
		} else if strings.Contains(reading, "full use of the left arm") {
			t.Errorf("unlinked field inherited unrelated current: %s", fact.SourceFieldPath)
		}
		if fact.SourceFieldPath == "/state/mobility" && !strings.Contains(reading, "effective time: unknown") {
			t.Error("same calendar day manufactured field effective time")
		}
		if fact.SourceFieldPath == "/state/capabilities/manual_dexterity" && !strings.Contains(reading, "2026-09-17") {
			t.Error("explicit effective date lost")
		}
		if strings.HasPrefix(fact.SourceFieldPath, "/relationships/") && (fact.Lane != "subjective_relationship" || fact.PerspectiveOwner != "Mira") {
			t.Error("relationship holder promoted to objective truth")
		}
	}
	for field := range expected {
		if !seen[field] {
			t.Errorf("existing admitted field missing %s", field)
		}
	}
	final := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	for _, text := range []string{"needs a sling", "full use of the left arm", "Mira lifted the crate freely with both hands", "effective time: unknown"} {
		if !strings.Contains(final, text) {
			t.Errorf("Go delivery lost %s: %s", text, final)
		}
	}
	assert45Budget(t, out.MemoryDeliveryPlan, 18000)
}

func Test46LegacyCharacterFieldDatesRemainUnknown(t *testing.T) {
	state, _ := characterFields46Fixture()
	state.FieldProvenanceJSON = ""
	out := characterFields46Assembly(state, nil)
	facts, _ := multiAgentCandidatePool(&out)
	if len(facts) == 0 {
		t.Fatal("legacy source fixture omitted")
	}
	for _, fact := range facts {
		if fact.SourceTable != "character_states" {
			continue
		}
		if fact.SourceTurn != 0 || !strings.Contains(prepareTurnMemoryReadingText(fact), "effective time: unknown") {
			t.Fatalf("legacy field acquired row date: %+v", fact)
		}
	}
}

func Test46RestoredCurrentReadingsKeepOriginalProof(t *testing.T) {
	for _, turn := range []int{7, 0} {
		t.Run(fmt.Sprintf("source-%d", turn), func(t *testing.T) {
			state, current := characterFields46Fixture()
			value := parseJSONMap(current.ValueJSON)
			value["lifecycle_key"] = "restored-field-life"
			current.ValueJSON = mustCompactJSON(value)
			current.SourceTurn = 90
			current.EvidenceJSON = mustCompactJSON(map[string]any{"source_revision": "repair-revision", "repair_recorded_turn": 90, "evidence_excerpt": "Wrong repair-time assertion.", "restored_source_turn": turn, "restored_evidence": map[string]any{"source": "critic.state_claims", "source_revision": "original-revision", "evidence_excerpt": "Original supporting observation.", "direct_evidence_ids": []int{17}}})
			out := characterFields46Assembly(state, []store.StatusCurrentValue{current})
			facts, _ := multiAgentCandidatePool(&out)
			readings := []string{mustCompactJSON(prepareTurnLifecycleReadings([]store.StatusCurrentValue{current}))}
			for _, fact := range facts {
				if fact.SourceFieldPath == "/state/mobility" {
					if !strings.Contains(mustCompactJSON(fact.Reading.Parts), "original-revision") {
						t.Fatal("original revision missing from diagnostic reading")
					}
					readings = append(readings, prepareTurnMemoryReadingText(fact))
				}
			}
			if len(readings) != 2 {
				t.Fatal("field did not reach production assembly")
			}
			for _, text := range readings {
				wantTurn := "source turn unknown"
				if turn > 0 {
					wantTurn = fmt.Sprintf("source turn %d", turn)
				}
				for _, required := range []string{wantTurn, "Original supporting observation."} {
					if !strings.Contains(text, required) {
						t.Errorf("restored reading lost original proof %s", required)
					}
				}
				if strings.Contains(text, "Wrong repair-time assertion") || strings.Contains(text, "source turn 90") {
					t.Fatal("repair observation replaced state origin")
				}
			}
			if !strings.Contains(readings[0], "original-revision") || !strings.Contains(readings[1], "original-revision") {
				t.Fatal("source and preprocessing must retain the original revision")
			}
			if strings.Contains(stringFromMap(out.MemoryDeliveryPlan, "final_text"), "original-revision") {
				t.Fatal("revision must not enter final memory delivery")
			}
		})
	}
}

func Test46CharacterFieldSourceReferencesRemainStable(t *testing.T) {
	state, current := characterFields46Fixture()
	out := characterFields46Assembly(state, []store.StatusCurrentValue{current})
	marked, _, _ := prepareTurnBuildPriorityCandidates(&out, "Mira", nil, 31, nil)
	prior := out
	prior.PriorityFactSeeds = append([]prepareTurnPriorityFactSeed(nil), out.PriorityFactSeeds...)
	for i := range prior.PriorityFactSeeds {
		prior.PriorityFactSeeds[i].ProjectionSource = strings.TrimSuffix(prior.PriorityFactSeeds[i].ProjectionSource, ":field_provenance")
		prior.PriorityFactSeeds[i].FieldObservationTurn = 0
	}
	unmarked, _, _ := prepareTurnBuildPriorityCandidates(&prior, "Mira", nil, 31, nil)
	byPath := map[string]prepareTurnPriorityMemoryCandidate{}
	for _, fact := range unmarked {
		byPath[fact.SourceFieldPath] = fact
	}
	for _, fact := range marked {
		original := byPath[fact.SourceFieldPath]
		if fact.CanonicalFactID != original.CanonicalFactID || fact.SourcePath != original.SourcePath || fact.CompleteText != original.CompleteText || fact.SourceRef != original.SourceRef {
			t.Errorf("field provenance changed original source reference %s", fact.SourceFieldPath)
		}
	}
}

func Test46CharacterReadProjectionCopiesMatchingFieldProvenance(t *testing.T) {
	state, _ := characterFields46Fixture()
	older := store.CharacterState{ID: 460, ChatSessionID: state.ChatSessionID, CharacterName: state.CharacterName, TurnIndex: 5, AppearanceJSON: `{"hair":"auburn"}`, StatusJSON: `{"mobility":"earlier value"}`, FieldProvenanceJSON: `{"contract_version":"character_field_provenance.v1","fields":{"/appearance/hair":{"source_turn":3},"/status/mobility":{"source_turn":2}}}`}
	s := NewServer(config.Default())
	s.Store = newIdentityAliasLinkRecordingStore()
	projection := s.canonicalCharacterReadProjection(context.Background(), state.ChatSessionID, []store.CharacterState{older, state}, nil)
	if len(projection.States) != 1 {
		t.Fatal("expected existing same-character display merge")
	}
	read := projection.States[0]
	fields := store.DecodeCharacterFieldProvenance(read.FieldProvenanceJSON)
	if read.StatusJSON != state.StatusJSON || read.AppearanceJSON != older.AppearanceJSON || intFromAny(fields["/appearance/hair"]["source_turn"], 0) != 3 || intFromAny(fields["/status/mobility"]["source_turn"], 0) != 10 {
		t.Fatalf("display copied another field's origin: %+v", read)
	}
	if characterResponseItem(read, nil, nil, nil)["field_provenance_json"] == nil {
		t.Fatal("character response dropped field provenance")
	}
}

func Test46ReversibleFieldReadingUsesExistingPublicBoundary(t *testing.T) {
	state, _ := characterFields46Fixture()
	for _, mode := range []string{"public", "private", "sensitive", "other_subject", "ambiguous"} {
		t.Run(mode, func(t *testing.T) {
			slot := map[string]any{"value": map[string]any{"text": "moves both arms freely"}, "visibility": "public", "sensitivity": "ordinary", "source_fields": []string{"/status/mobility"}, "source": map[string]any{"source_turn": 22, "evidence_excerpt": "Evidence from the actual slot turn."}}
			subject := "Mira"
			switch mode {
			case "private":
				slot["visibility"] = "private"
			case "sensitive":
				slot["sensitivity"] = "reproductive"
			case "other_subject":
				subject = "Rowan"
			}
			current := store.StatusCurrentValue{ID: 469, SourceTurn: 29, StatusKey: reversibleBodyStatusKey, ValueJSON: mustCompactJSON(map[string]any{"version": reversibleStateContractVersion, "domain": "body", "subject_label": subject, "slots": map[string]any{"arm": slot}}), EvidenceJSON: `{"evidence_excerpt":"Latest unrelated row evidence."}`}
			values := []store.StatusCurrentValue{current}
			if mode == "ambiguous" {
				other := current
				other.ID++
				values = append(values, other)
			}
			input := prepareTurnAssemblyInput{CharacterStates: []store.CharacterState{state}, TopK: 5, MaxChars: 18000, UserInput: "Mira and Rowan discuss mobility", Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
			input.Perspective.ActiveStates, input.Perspective.ReversibleValues = characterFields46Scene(), values
			out := buildPrepareTurnInjectionAssemblyWithBudget(input)
			facts, _ := multiAgentCandidatePool(&out)
			for _, fact := range facts {
				if fact.SourceFieldPath != "/state/mobility" {
					continue
				}
				reading := prepareTurnMemoryReadingText(fact)
				linked := strings.Contains(reading, "moves both arms freely")
				if linked != (mode == "public") {
					t.Fatalf("linked field disagrees with existing public boundary: %s", mode)
				}
				if strings.Contains(reading, "Latest unrelated row evidence") || (linked && !strings.Contains(reading, "source turn 22")) {
					t.Fatal("slot acquired newer row observation")
				}
				return
			}
			t.Fatal("existing mobility field missing")
		})
	}
}

func Test46PrivateCurrentFieldEvidenceCannotEnterObjectiveReading(t *testing.T) {
	state, current := characterFields46Fixture()
	current.ValueJSON = strings.ReplaceAll(current.ValueJSON, `"claim_scope":"objective"`, `"claim_scope":"secret","perspective_owner":"Rowan"`)
	current.EvidenceJSON = `{"evidence_excerpt":"SECRET-FIELD-FACT"}`
	out := characterFields46Assembly(state, []store.StatusCurrentValue{current})
	facts, _ := multiAgentCandidatePool(&out)
	for _, fact := range facts {
		if strings.Contains(prepareTurnMemoryReadingText(fact), "SECRET-FIELD-FACT") {
			t.Fatal("private current state leaked into a public historical field")
		}
	}
}

func Test46ReversibleRecoveryLinksOldFieldWithoutReintroducingCondition(t *testing.T) {
	state, _ := characterFields46Fixture()
	current := store.StatusCurrentValue{ID: 468, ChatSessionID: state.ChatSessionID, StatusKey: reversibleBodyStatusKey, OwnerScope: reversibleStateOwnerScope, OwnerID: "Mira", SourceTurn: 28, WriteState: "current",
		ValueJSON:    `{"version":"reversible_state.v1","domain":"body","subject_label":"Mira","slots":{}}`,
		EvidenceJSON: `{"source_turn":28,"evidence_excerpt":"Mira lifted the crate freely with both hands.","history_observation":{"transition":"recover","visibility":"public","sensitivity":"ordinary","source_fields":["/status/mobility"]}}`}
	input := prepareTurnAssemblyInput{CharacterStates: []store.CharacterState{state}, TopK: 5, MaxChars: 18000, UserInput: "Mira moves her arm next to Rowan", Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	input.Perspective.ReversibleValues = []store.StatusCurrentValue{current}
	input.Perspective.ActiveStates = characterFields46Scene()
	input.Perspective.Selection.Query = input.UserInput
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	facts, _ := multiAgentCandidatePool(&out)
	for _, fact := range facts {
		if fact.SourceFieldPath == "/state/mobility" {
			reading := prepareTurnMemoryReadingText(fact)
			for _, required := range []string{"needs a sling", "no longer current (recover)", "Mira lifted the crate freely", "source turn 28"} {
				if !strings.Contains(reading, required) {
					t.Errorf("recovery omitted %s: %s", required, reading)
				}
			}
			return
		}
	}
	t.Fatal("mobility source field omitted")
}

func Test46HTTPCharacterFieldsReachAllOptionalAIModes(t *testing.T) {
	for _, preprocess := range []bool{false, true} {
		for _, publisher := range []bool{false, true} {
			for _, mode := range []string{"normal", "empty", "failure"} {
				if !preprocess && !publisher && mode != "normal" {
					continue
				}
				t.Run(fmt.Sprintf("preprocess-%v/publisher-%v/%s", preprocess, publisher, mode), func(t *testing.T) {
					t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
					assertReading := func(text string) {
						t.Helper()
						if strings.Contains(text, "source_session_id: origin") {
							t.Error("session metadata reached provider or final delivery")
						}
						for _, required := range []string{"needs a sling", "full use of the left arm", "Mira lifted the crate freely with both hands", "source turn 10", "effective time: unknown"} {
							if !strings.Contains(text, required) {
								t.Errorf("real HTTP delivery missing %s", required)
							}
						}
					}
					agentCalls, publisherCalls := 0, 0
					provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
							return
						}
						if _, ok := body["input"]; ok {
							_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{.1, .2}, "index": 0}}})
							return
						}
						messages := outputFidelityLineageSlice(body["messages"])
						if len(messages) < 2 {
							t.Error("unexpected provider request")
							http.Error(w, "unexpected", 400)
							return
						}
						var input map[string]any
						_ = json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input)
						assertReading(mustCompactJSON(input))
						_, isPublisher := input["supervisor_support_packet"]
						if isPublisher {
							publisherCalls++
						} else {
							agentCalls++
						}
						if mode == "failure" {
							http.Error(w, "fixture provider unavailable", http.StatusBadRequest)
							return
						}
						if mode == "empty" {
							_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
							return
						}
						if isPublisher {
							_, _ = w.Write([]byte(publisherV3OpenAIResponse("character_states:466", "Use the observed field together with recorded recovery.")))
							return
						}
						ids := []string{}
						for _, raw := range outputFidelityLineageSlice(input["candidates"]) {
							item := mapFromAny(raw)
							ids = append(ids, extractionStringFromAny(item["ref"]))
						}
						if len(ids) == 0 {
							t.Error("source fields did not reach actual preprocessing input")
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": mustCompactJSON(map[string]any{"selected_ids": ids})}}}})
					}))
					defer provider.Close()
					cfg := config.Default()
					cfg.StoreMode = config.StoreModeDualShadow
					cfg.PromptDir = filepath.Join("..", "..", "..", "prompts")
					s := NewServer(cfg)
					state, current := characterFields46Fixture()
					s.Store = &priorityPrepareTurnStore{turnRecordingStore: &turnRecordingStore{returnCharStates: []store.CharacterState{state}, returnStatusCurrent: []store.StatusCurrentValue{current}, returnActiveStates: characterFields46Scene()}}
					s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint, s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = "custom", provider.URL, "fixture-publisher", "fixture-key"
					s.RuntimeConfig.SupervisorTimeoutSec = 30
					settings := defaultMultiAgentSettings()
					settings.Enabled = preprocess
					for role, c := range settings.Roles {
						c.Enabled = role == "character_objective"
						c.UsePublisher = false
						c.Provider = "custom"
						c.Endpoint = provider.URL
						c.Model = "fixture"
						c.APIKey = "fixture-key"
						settings.Roles[role] = c
					}
					b, _ := json.Marshal(settings)
					rec := httptest.NewRecorder()
					s.handleMultiAgentSettings(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(b)))
					if rec.Code != 200 {
						t.Fatal(rec.Body.String())
					}
					mux := http.NewServeMux()
					s.RegisterRoutes(mux)
					b, _ = json.Marshal(map[string]any{"chat_session_id": "fields46", "turn_index": 31, "raw_user_input": "Mira and Rowan review Mira's mobility, compass and voyage plan.", "settings": map[string]any{"injection_enabled": true, "max_injection_chars": 18000, "supervisor_enabled": publisher, "guide_mode": "standard", "guide_strength": "strong"}})
					rec = httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(b)))
					if rec.Code != 200 {
						t.Fatal(rec.Body.String())
					}
					var response map[string]any
					_ = json.Unmarshal(rec.Body.Bytes(), &response)
					plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
					assertReading(extractionStringFromAny(plan["final_text"]))
					assertReading(mustCompactJSON(response["payload_application_plan"]))
					assert45Budget(t, plan, 18000)
					if (preprocess && agentCalls != 1) || (!preprocess && agentCalls != 0) || (publisher && publisherCalls != 1) || (!publisher && publisherCalls != 0) {
						t.Fatalf("provider calls preprocessing=%d publisher=%d", agentCalls, publisherCalls)
					}
				})
			}
		}
	}
}
