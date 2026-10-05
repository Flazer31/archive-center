package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSourceConnectionsPreciseRecallUsesAssemblyContext(t *testing.T) {
	in, unit := sourceConnectionsPreciseInput(t)
	before := mustCompactJSON(unit)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	checkSourceConnectionDelivery(t, &out, in, []string{"fulfilled at the northern quay", "Rowan accepted the compass", "2002-06-02", "Mira owns no compass now"})
	if before != mustCompactJSON(unit) {
		t.Fatal("assembly changed stored precise source")
	}
	search := out.supplementProjection(nil, in.Perspective.Selection)
	checkSourceConnectionDelivery(t, &search, in, []string{"fulfilled at the northern quay", "2002-06-02"})
	// A different lifecycle cannot complete this promise by name similarity.
	in.Perspective.NarrativeValues[0].ValueJSON = strings.ReplaceAll(in.Perspective.NarrativeValues[0].ValueJSON, "compass-voyage", "different-voyage")
	control := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&control)
	for _, f := range facts {
		if strings.Contains(prepareTurnMemoryReadingText(f), "fulfilled at the northern quay") {
			t.Fatal("unrelated lifecycle attached to the source")
		}
	}
}

func sourceConnectionsPreciseInput(t *testing.T) (prepareTurnAssemblyInput, store.PreciseMemoryUnit) {
	t.Helper()
	promise, current := lifecycle46Fixture("source-connections")
	payload := mapFromAny(sliceFromAny(parseJSONMap(promise.SummaryJSON)["narrative_events"])[0])
	payload["occurrence_time"] = map[string]any{"date": "2002-06-02", "time": "10:00"}
	unit := store.PreciseMemoryUnit{UnitID: "compass-promise", ChatSessionID: promise.ChatSessionID, SourceTurnStart: promise.TurnIndex, SourceTurnEnd: promise.TurnIndex, Kind: "event", Visibility: "public", PayloadJSON: mustCompactJSON(payload)}
	fact, ok := prepareTurnPrioritySemanticFactFromPreciseUnit(unit, .8, "fixture_vector_boundary")
	if !ok {
		t.Fatal("source unit did not become a semantic fact")
	}
	in := prepareTurnAssemblyInput{TopK: 5, MaxChars: 32000, UserInput: "Mira recalls the compass promise", Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	in.Perspective.Selection.Query = in.UserInput
	in.Perspective.Selection.SemanticFacts = []prepareTurnPrioritySemanticFact{fact}
	in.Perspective.NarrativeValues = []store.StatusCurrentValue{current}
	in.Perspective.NarrativeValues = append(in.Perspective.NarrativeValues, store.StatusCurrentValue{ID: 463, ChatSessionID: promise.ChatSessionID, StatusKey: narrativeStateStatusKey, WriteState: "current", OwnerScope: "entity", OwnerID: "mira", SourceTurn: 20,
		ValueJSON: `{"subject":"Mira","state_slot":"inventory","claim_scope":"objective","value":"Mira owns no compass now; it belongs to Rowan."}`, EvidenceJSON: `{"source_turn":20,"evidence_excerpt":"Mira handed the compass to Rowan."}`})
	in.Perspective.StoryClock = map[string]any{"absolute": map[string]any{"date": "2002-06-04", "time": "13:00"}}
	return in, unit
}

func TestSourceConnectionsKGValidityReachesSelectedText(t *testing.T) {
	for _, end := range []int{0, 12} {
		t.Run(fmt.Sprint(end), func(t *testing.T) {
			triple := store.KGTriple{ID: 99, Subject: "Mira", Predicate: "promised_to", Object: "Rowan", SourceTurn: 3, ValidFrom: 3, ValidTo: end}
			in := prepareTurnAssemblyInput{Triples: []store.KGTriple{triple}, TopK: 5, MaxChars: 32000, UserInput: "Mira and Rowan revisit their promise", Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
			in.Perspective.Selection.Query = in.UserInput
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			period := fmt.Sprintf("turn %d", triple.ValidFrom)
			ending := "end unrecorded"
			if end > 0 {
				ending = fmt.Sprintf("turn %d", end)
			}
			checkSourceConnectionDelivery(t, &out, in, []string{period, ending, "not current-state authority", "Mira --promised_to--> Rowan"})
		})
	}
}

func TestSourceConnectionsCharacterSupportKeepsProvenance(t *testing.T) {
	const sid = "source-connections-character"
	source := prepareTurnCharacterMemoryTestSource(sid, "accepted-profile", "profile-unit", "owner_private")
	source["source_turn_start"], source["source_turn_end"] = 13, 13
	state := store.CharacterState{ID: 3, ChatSessionID: sid, CharacterName: "Mira", TurnIndex: 45,
		PersonalityJSON: mustCompactJSON(map[string]any{"contract_version": characterProfileContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira", "stable": map[string]any{"observations": []any{map[string]any{"profile_section": "stable", "trait_key": "keeps_a_private_diary", "supported_expression": "Mira privately keeps a diary of the voyage.", "source_ref": source}}}})}
	in := prepareTurnAssemblyInput{CharacterStates: []store.CharacterState{state}, TopK: 5, MaxChars: 32000, UserInput: "Mira recalls the voyage", Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	in.Perspective.Selection.Query = in.UserInput
	in.Perspective.CharacterMemory = prepareTurnCharacterMemoryTestReadContext(sid, map[string]bool{"accepted-profile": true}, nil)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	found := false
	for _, f := range facts {
		if !strings.Contains(f.CompleteText, "Mira profile support") {
			continue
		}
		found = true
		if f.SourceTurn != intFromAny(source["source_turn_end"], 0) || f.PerspectiveOwner != state.CharacterName || f.Visibility != "owner_private" || len(f.AllowedViewers) != 1 || f.AllowedViewers[0] != state.CharacterName {
			t.Errorf("source scope lost: turn=%d owner=%q visibility=%q viewers=%v", f.SourceTurn, f.PerspectiveOwner, f.Visibility, f.AllowedViewers)
		}
	}
	if !found {
		t.Fatal("production character support was not registered")
	}
	checkSourceConnectionDelivery(t, &out, in, []string{"Mira profile support", "Mira privately keeps a diary", "field observation turn 13", "owner_private"})
}

func TestSourceConnectionsDirectionalInteractionKeepsStoredScope(t *testing.T) {
	for _, recorded := range []bool{false, true} {
		t.Run(fmt.Sprint(recorded), func(t *testing.T) {
			extra := map[string]any{"domain": "trust", "observation": "Mira explicitly trusts Rowan.", "support_kind": "explicit_statement"}
			if recorded {
				extra["perspective_owner"], extra["allowed_viewers"] = "Mira", []string{"Mira", "Rowan"}
			}
			unit := interactionProjectionUnit("trust-source", "observation", relationshipObservationContract, "mira-id", "rowan-id", "Mira", "Rowan", "public", 17, extra)
			packet, public, guarded := buildPrepareTurnActiveInteractionProjection([]store.PreciseMemoryUnit{unit}, nil, "Mira asks Rowan for help.", nil, 18, true)
			in := prepareTurnAssemblyInput{TopK: 5, MaxChars: 32000, UserInput: "Mira asks Rowan for help.", Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
			in.Perspective.Selection.Query = in.UserInput
			in.Perspective.InteractionPublicText, in.Perspective.InteractionGuardedText = public, guarded
			in.Perspective.InteractionItems = outputFidelityLineageSlice(packet["items"])
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			facts, _ := multiAgentCandidatePool(&out)
			found := false
			for _, f := range facts {
				if !strings.Contains(f.CompleteText, "Mira -> Rowan") {
					continue
				}
				found = true
				if f.SourceTurn != 17 || f.SourceTable != "precise_memory_units" || f.Visibility != "public" {
					t.Errorf("lost interaction source: %+v", f)
				}
				if recorded && (f.PerspectiveOwner != "Mira" || len(f.AllowedViewers) != 2) {
					t.Error("stored audience lost")
				}
				if !recorded && (f.PerspectiveOwner != "" || len(f.AllowedViewers) != 0) {
					t.Error("participants were invented as knowers")
				}
			}
			if !found {
				t.Fatal("interaction candidate not registered")
			}
			checkSourceConnectionDelivery(t, &out, in, []string{"Mira -> Rowan", "Mira explicitly trusts Rowan.", "source_turn=17"})
		})
	}
}

func TestSourceConnectionsVoiceAndRelationshipSupport(t *testing.T) {
	const sid = "source-connections-voice"
	voice := prepareTurnCharacterMemoryTestSource(sid, "voice-revision", "voice-unit", "owner_private")
	voice["source_turn_start"], voice["source_turn_end"] = 6, 6
	state := store.CharacterState{ChatSessionID: sid, CharacterName: "Mira", TurnIndex: 45,
		SpeechStyleJSON: mustCompactJSON(map[string]any{"contract_version": voiceBehaviorProjectionContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira", "principles": []any{map[string]any{"principle_key": "speaks_warmly", "support_refs": []any{voice}}}})}
	relationship := store.StatusCurrentValue{ChatSessionID: sid, StatusKey: relationshipStateStatusKey, OwnerScope: relationshipStateOwnerScope, OwnerID: "relationship-owner", WriteState: "current", SourceTurn: 8,
		ValueJSON: mustCompactJSON(map[string]any{"version": relationshipStateContractVersion, "source_entity_id": "entity-mira", "source_label": "Mira", "target_entity_id": "entity-rowan", "target_label": "Rowan", "domain": "trust",
			"current": map[string]any{"observation": "trusts Rowan with warnings", "visibility": "public"}, "reciprocity": map[string]any{"state": "not_inferred"},
			"validity": map[string]any{"source_turn_start": 8, "source_turn_end": 8, "source_revision": "relationship-revision", "lifecycle_state": "active"},
			"source":   map[string]any{"source_contract": completeTurnSourceAcceptanceContract, "source_revision": "relationship-revision", "precise_memory_unit_id": "relationship-unit", "content_hash": "relationship-content-hash"}})}
	in := prepareTurnAssemblyInput{CharacterStates: []store.CharacterState{state, {ChatSessionID: sid, CharacterName: "Rowan"}}, TopK: 5, MaxChars: 32000, UserInput: "Mira and Rowan discuss their journey", Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	in.Perspective.Selection.Query = in.UserInput
	in.Perspective.CharacterMemory = prepareTurnCharacterMemoryTestReadContext(sid, map[string]bool{"voice-revision": true, "relationship-revision": true}, []store.StatusCurrentValue{relationship})
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	for marker, turn := range map[string]int{"voice principle": 6, "Mira -> Rowan relationship": 8} {
		found := false
		for _, f := range facts {
			if !strings.Contains(f.CompleteText, marker) {
				continue
			}
			found = true
			if f.SourceTurn != turn || f.PerspectiveOwner != "Mira" || !strings.HasPrefix(f.SourceRef, "character-memory:") {
				t.Errorf("%s source disconnected: %+v", marker, f)
			}
			if marker == "voice principle" && (f.Visibility != "owner_private" || len(f.AllowedViewers) != 1) {
				t.Error("private voice scope lost")
			}
			if marker != "voice principle" && (f.Visibility != "public" || len(f.AllowedViewers) != 0) {
				t.Error("public relationship invented audience")
			}
		}
		if !found {
			t.Errorf("missing %s", marker)
		}
	}
	checkSourceConnectionDelivery(t, &out, in, []string{"speaks_warmly", "trusts Rowan with warnings", "field observation turn 6", "field observation turn 8"})
}

// Exercises the actual grouped provider request, response parser and delivery
// path with a controlled endpoint. It verifies transport, not model judgment.
func TestSourceConnectionsActualPreprocessingRoundTrip(t *testing.T) {
	in, _ := sourceConnectionsPreciseInput(t)
	in.UserInput = "Mira and Rowan recall the compass promise."
	in.Perspective.Selection.Query = in.UserInput
	in.Triples = []store.KGTriple{{ID: 91, Subject: "Mira", Predicate: "promised_to", Object: "Rowan", SourceTurn: 3, ValidFrom: 3}}
	const sid = "source-connections"
	source := prepareTurnCharacterMemoryTestSource(sid, "profile-revision", "profile-unit", "owner_private")
	source["source_turn_start"], source["source_turn_end"] = 13, 13
	in.CharacterStates = []store.CharacterState{{ChatSessionID: sid, CharacterName: "Mira", TurnIndex: 45,
		PersonalityJSON: mustCompactJSON(map[string]any{"contract_version": characterProfileContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira", "stable": map[string]any{"observations": []any{map[string]any{"profile_section": "stable", "trait_key": "private_diary", "supported_expression": "Mira privately keeps a diary.", "source_ref": source}}}})}}
	in.Perspective.CharacterMemory = prepareTurnCharacterMemoryTestReadContext(sid, map[string]bool{"profile-revision": true}, nil)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, summaries := multiAgentCandidatePool(&out)
	var calls, witnessed atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		messages := outputFidelityLineageSlice(body["messages"])
		var packet map[string]any
		if len(messages) < 2 {
			t.Error("missing model input")
			return
		}
		if err := json.Unmarshal([]byte(stringFromMap(mapFromAny(messages[1]), "content")), &packet); err != nil {
			t.Error(err)
			return
		}
		if containsAll(fmt.Sprint(packet), "fulfilled at the northern quay", "2002-06-02", "turn 3 to end unrecorded", "not current-state authority", "Mira privately keeps a diary.", "owner_private", "n:13", "Mira owns no compass now") {
			witnessed.Add(1)
		} else {
			t.Errorf("provider packet lost context: %v", packet)
		}
		choose := func(input map[string]any) map[string]any {
			ids := []string{}
			for _, raw := range outputFidelityLineageSlice(input["candidates"]) {
				ids = append(ids, stringFromMap(mapFromAny(raw), "ref"))
			}
			return map[string]any{"selected_ids": ids}
		}
		results := map[string]any{}
		for _, raw := range outputFidelityLineageSlice(packet["roles"]) {
			role := mapFromAny(raw)
			input := map[string]any{}
			for k, v := range mapFromAny(packet["shared_input"]) {
				input[k] = v
			}
			for k, v := range mapFromAny(role["input"]) {
				input[k] = v
			}
			results[stringFromMap(role, "role")] = choose(input)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": mustCompactJSON(map[string]any{"roles": results})}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Enabled = true
		c.UsePublisher = false
		c.Provider = "custom"
		c.Endpoint = provider.URL
		c.Model = "same-model"
		c.APIKey = "synthetic-key"
		cfg.Roles[role] = c
	}
	out.Preprocessing = (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, summaries, in.MaxChars, in.TopK, nil, nil)
	if calls.Load() != 1 || witnessed.Load() != 1 {
		t.Fatalf("existing grouped call count=%d, source context witnessed=%d", calls.Load(), witnessed.Load())
	}
	for _, fact := range facts {
		role := out.Preprocessing.role(fact.Lane)
		if role.Source != "ai" || len(role.Selection.SelectedIDs) == 0 {
			t.Errorf("provider result not applied for supplied lane %s: source=%s", fact.Lane, role.Source)
		}
	}
	plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, in.MaxChars, in.TopK, in.BudgetMode, in.Budgets, in.Perspective.Selection)
	text := extractionStringFromAny(plan["final_text"])
	if !containsAll(text, "fulfilled at the northern quay", "Rowan accepted the compass", "2002-06-02", "turn 3 to end unrecorded", "not current-state authority", "Mira privately keeps a diary.", "field observation turn 13", "owner_private") {
		t.Fatalf("selected source context lost: %s", text)
	}
	assert45Budget(t, plan, in.MaxChars)
}

// These are assembly/selection tests, not live-model judgment tests. The AI
// selection boundary deliberately chooses the offered candidates; production
// projection, budgeting and final rendering remain under test.
func checkSourceConnectionDelivery(t *testing.T, out *prepareTurnInjectionAssembly, in prepareTurnAssemblyInput, wants []string) {
	t.Helper()
	facts, summaries := multiAgentCandidatePool(out)
	if len(facts) == 0 {
		t.Fatal("no production candidates")
	}
	for _, mode := range []string{"go", "preprocessing"} {
		selected := *out
		if mode == "preprocessing" {
			roles := []multiAgentRoleResult{}
			for _, lane := range []string{"event_recent", "character_objective", "subjective_relationship", "world_state", "unresolved_goal"} {
				role := multiAgentRoleResult{Role: lane, Source: "ai"}
				for _, f := range facts {
					if f.Lane == lane {
						role.Selection.SelectedIDs = append(role.Selection.SelectedIDs, f.CanonicalFactID)
					}
				}
				roles = append(roles, role)
			}
			selected.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: roles}
		}
		plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&selected, in.MaxChars, in.TopK, in.BudgetMode, in.Budgets, in.Perspective.Selection)
		text := extractionStringFromAny(plan["final_text"])
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s final lost %q: %s", mode, want, text)
			}
		}
		for _, f := range facts {
			if f.SourceRef != "" && strings.Contains(text, f.SourceRef) {
				t.Errorf("%s final exposed an opaque source reference", mode)
			}
		}
		if len(out.CharacterMemorySupport) > 0 {
			// This small fixture fits the configured budget; delivered diagnostics
			// must agree with the actual complete support bodies in final text.
			support := finalizePrepareTurnCharacterMemorySupport(out.CharacterMemorySupport, plan)
			if intFromAny(support["delivered_count"], 0) != intFromAny(support["eligible_count"], 0) {
				t.Errorf("%s character delivery diagnostics disagree: %v", mode, support)
			}
		}
	}
	// Each role reads its own candidates. Provenance is structured metadata in
	// the request and a heading in final delivery; the spellings intentionally differ.
	byID := map[string]prepareTurnPriorityMemoryCandidate{}
	lanes := map[string]bool{}
	for _, f := range facts {
		byID[f.CanonicalFactID] = f
		lanes[f.Lane] = true
	}
	seen := map[string]bool{}
	for lane := range lanes {
		packet := multiAgentInput(lane, facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), in.MaxChars, in.TopK, nil)
		for _, raw := range outputFidelityLineageSlice(packet["candidates"]) {
			item := mapFromAny(raw)
			id := stringFromMap(item, "id")
			f, exists := byID[id]
			if !exists {
				t.Fatalf("unexpected model candidate %q", id)
			}
			seen[id] = true
			if intFromAny(item["source_turn"], 0) != f.SourceTurn || stringFromMap(item, "perspective_owner") != f.PerspectiveOwner || stringFromMap(item, "visibility") != f.Visibility || stringFromMap(item, "source_ref") != f.SourceRef || strings.Join(stringsFromAny(item["allowed_viewers"]), ",") != strings.Join(f.AllowedViewers, ",") {
				t.Errorf("preprocessing source metadata lost: %v", item)
			}
		}
	}
	if len(seen) != len(byID) {
		t.Errorf("candidate transport coverage %d/%d", len(seen), len(byID))
	}
}
