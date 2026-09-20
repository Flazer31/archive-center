package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func bodyState46Fixture(t *testing.T, enabled bool) (*Server, *turnRecordingStore, bodyCharacterConfig) {
	t.Helper()
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	st := &turnRecordingStore{returnCharStates: []store.CharacterState{{ChatSessionID: "body-session", CharacterName: "Mina", AppearanceJSON: `{"gender":"female"}`}}}
	srv := &Server{Cfg: config.Default(), Store: st}
	character := defaultBodyCharacterConfig()
	character.EntityID, character.OriginEntityID, character.CharacterName = "mina-id", "mina-id", "Mina"
	character.Cycle.ReferenceTime = map[string]any{"date": "1423-01-01"}
	character.Cycle.VariationDays = 0
	if _, err := srv.saveBodyTrackingConfig("body-session", bodyTrackingConfig{CycleTrackingEnabled: enabled, Characters: []bodyCharacterConfig{character}}); err != nil {
		t.Fatal(err)
	}
	return srv, st, character
}

func bodyEvent46(kind, key, date string) map[string]any {
	event := map[string]any{"kind": kind, "semantic_event_key": key, "character_id": "mina-id", "evidence_excerpt": "Mina records this observation.", "visibility": "private"}
	if date != "" {
		event["occurred_at"] = map[string]any{"date": date}
	}
	return event
}

func saveBodyEvent46(t *testing.T, srv *Server, turn int, event map[string]any) artifactSaveResult {
	t.Helper()
	clock := storyClockProposal("absolute", "current", "exact", "Today is January thirteenth.")
	clock["absolute"] = map[string]any{"date": "1423-01-13"}
	extraction := map[string]any{"turn_summary": "Mina records this observation.", "body_events": []any{event}, "story_clock": clock, "evidence_excerpts": []any{"Mina records this observation."}}
	result := srv.saveCriticExtractionArtifacts(acceptedStoryClockContext(fmt.Sprintf("body-source-%d", turn), fmt.Sprintf("body-turn-%d", turn), "generation"), "body-session", turn, extraction, "Today is January thirteenth. Mina records this observation.", completeTurnEmbeddingConfig{}, time.Unix(int64(turn), 0), nil)
	if result.Errors != 0 {
		t.Fatalf("body production artifact save: %#v", result.ErrorDetails)
	}
	return result
}

func bodyCurrent46(t *testing.T, srv *Server) store.StatusCurrentValue {
	t.Helper()
	values, err := srv.bodyTrackingCurrentValues(context.Background(), "body-session")
	if err != nil || len(values) != 1 {
		t.Fatalf("body current lookup: %#v %v", values, err)
	}
	return values[0]
}

func bodyHistory46(st *turnRecordingStore) []store.StatusChangeEvent {
	var events []store.StatusChangeEvent
	for _, event := range st.savedStatusEvents {
		if event.StatusKey == bodyTrackingStatusKey {
			events = append(events, event)
		}
	}
	return events
}

func Test46BodyTrackingOffPreservesSourceAndCycleOnlyUsesObservedDate(t *testing.T) {
	srv, st, character := bodyState46Fixture(t, false)
	event := bodyEvent46("period_start", "period-january", "1423-01-10")
	saveBodyEvent46(t, srv, 1, event)
	if len(bodyHistory46(st)) != 0 || len(sliceFromAny(parseJSONMap(st.savedMemories[0].SummaryJSON)["body_events"])) != 1 {
		t.Fatal("OFF changed body state or discarded original observed source")
	}
	if got := srv.bodyTrackingCriticContext(context.Background(), "body-session"); got != nil {
		t.Fatalf("OFF injected optional instructions: %#v", got)
	}
	if _, err := srv.saveBodyTrackingConfig("body-session", bodyTrackingConfig{CycleTrackingEnabled: true, AutomaticPregnancyEnabled: false, Characters: []bodyCharacterConfig{character}}); err != nil {
		t.Fatal(err)
	}
	saveBodyEvent46(t, srv, 2, event)
	current := bodyCurrent46(t, srv)
	payload := parseJSONMap(current.ValueJSON)
	if stringFromMap(mapFromAny(payload["cycle_reference"]), "date") != "1423-01-10" {
		t.Fatalf("period reference borrowed observation or settings date: %#v", payload)
	}
	observation := mapFromAny(mapFromAny(payload["observed_facts"])["period_start"])
	if stringFromMap(mapFromAny(mapFromAny(mapFromAny(observation["observed_at"])["story_clock"])["absolute"]), "date") != "1423-01-13" {
		t.Fatal("source observation date was replaced by occurrence date")
	}
	beforeEvents := len(bodyHistory46(st))
	reading := assessBodyCycle(bodyTrackingCycleSpec(character, current), map[string]any{"date": "1423-01-11"})
	if mapFromAny(reading["cycle_day"])["min"] != float64(2) || reading["knowledge"] != "not_inferred" || payload["pregnancy"] != nil {
		t.Fatalf("cycle-only reading fabricated outcome/knowledge: %#v %#v", reading, payload)
	}
	if len(bodyHistory46(st)) != beforeEvents {
		t.Fatal("read-only cycle estimate persisted inferred periods")
	}
	if _, err := srv.saveBodyTrackingConfig("body-session", bodyTrackingConfig{Characters: []bodyCharacterConfig{character}}); err != nil {
		t.Fatal(err)
	}
	saveBodyEvent46(t, srv, 3, bodyEvent46("pregnancy_ended", "off-event", "1423-02-01"))
	if bodyCurrent46(t, srv).ValueJSON != current.ValueJSON || len(bodyHistory46(st)) != beforeEvents {
		t.Fatal("turning tracking off erased or progressed stored facts")
	}
}

func Test46BodyTrackingReplayAndIndependentObservedLifecycles(t *testing.T) {
	srv, st, _ := bodyState46Fixture(t, true)
	saveBodyEvent46(t, srv, 1, bodyEvent46("pregnancy_confirmed", "confirmed-one", "1423-01-08"))
	saveBodyEvent46(t, srv, 2, bodyEvent46("period_start", "period-one", "1423-01-10"))
	saveBodyEvent46(t, srv, 2, bodyEvent46("period_start", "period-one", "1423-01-10"))
	saveBodyEvent46(t, srv, 8, bodyEvent46("period_start", "period-one", "1423-01-10"))
	if len(bodyHistory46(st)) != 2 {
		t.Fatal("same event replay or later recall wrote another state change")
	}
	saveBodyEvent46(t, srv, 9, bodyEvent46("recovery_started", "recovery-one", "1423-01-11"))
	saveBodyEvent46(t, srv, 10, bodyEvent46("recovery_ended", "recovery-end", "1423-01-12"))
	payload := parseJSONMap(bodyCurrent46(t, srv).ValueJSON)
	if stringFromMap(mapFromAny(payload["pregnancy"]), "status") != "confirmed" || stringFromMap(mapFromAny(payload["recovery"]), "status") != "ended" || payload["knowledge"] != nil {
		t.Fatalf("independent body facts collapsed or transferred knowledge: %#v", payload)
	}
	saveBodyEvent46(t, srv, 11, bodyEvent46("period_start", "older-period", "1422-12-01"))
	flashback := bodyEvent46("pregnancy_ended", "old-pregnancy", "1422-01-01")
	flashback["scene_scope"] = "flashback"
	saveBodyEvent46(t, srv, 12, flashback)
	events := bodyHistory46(st)
	if len(events) != 6 || events[4].EventState != "history_only" || events[5].EventState != "history_only" {
		t.Fatalf("historical observations lost: %#v", events)
	}
	if stringFromMap(mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["cycle_reference"]), "date") != "1423-01-10" {
		t.Fatal("later recall replaced newer effective period date")
	}
	unknown := bodyEvent46("period_start", "unknown-period", "")
	saveBodyEvent46(t, srv, 13, unknown)
	if stringFromMap(mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["cycle_reference"]), "story_time") != "unknown" {
		t.Fatal("unknown event date silently borrowed current story date")
	}
}

func Test46BodyTrackingMetadataIsPrivateAndDynamicContextUsesExistingBudget(t *testing.T) {
	srv, _, _ := bodyState46Fixture(t, true)
	event := bodyEvent46("period_start", "private-period", "1423-01-10")
	raw := map[string]any{"turn_summary": "Mina records this observation.", "evidence_excerpts": []any{"Mina records this observation."}, "body_events": []any{event}}
	salvaged, _, err := validateCriticExtractionSchema(raw)
	if err != nil || len(sliceFromAny(salvaged["body_events"])) != 1 {
		t.Fatalf("optional body events did not survive normal schema salvage: %#v %v", salvaged, err)
	}
	projection := buildPublicMemoryProjection(raw, "")
	if projection.Eligible || strings.Contains(projection.SearchText.Text, "private-period") || projection.Extraction["body_events"] != nil {
		t.Fatalf("private metadata became public objective memory: %#v", projection)
	}
	context := srv.bodyTrackingCriticContext(context.Background(), "body-session")
	_, ledger, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, nil, map[string]any{"body_tracking": context}, nil, "Mina", completeTurnCriticInputPolicy{})
	if len(mapFromAny(ledger["body_tracking"])) == 0 || trace == nil {
		t.Fatalf("existing auxiliary path discarded configured context: %#v %#v", ledger, trace)
	}
	prompt := buildCompleteTurnCriticPrompt("body-session", 1, "", "Mina records this observation.", nil, nil, ledger)
	if !strings.Contains(prompt, "body_events") || !strings.Contains(prompt, "mina-id") || strings.Contains(prompt, "simulation_seed") {
		t.Fatalf("dynamic instructions missing or private seed exposed: %s", prompt)
	}
}

func Test46BodyTrackingSemanticIdentitySurvivesOperationalEntityRemap(t *testing.T) {
	character := bodyCharacterConfig{EntityID: "parent-id", OriginEntityID: "origin-id"}
	first := bodyTrackingEventSourceUnit(character, "observed-period-1")
	character.EntityID = "child-id"
	if got := bodyTrackingEventSourceUnit(character, "observed-period-1"); got != first {
		t.Fatalf("copy changed immutable semantic event token: %s / %s", first, got)
	}
}

func Test46BodyTrackingManualStateAPIUsesExistingRepairUndoAndPreservesUnknown(t *testing.T) {
	srv, base, _ := bodyState46Fixture(t, false)
	st := &adminRepairProjectionRecordingStore{turnRecordingStore: base}
	srv.Store = st
	st.returnChatLogs = []store.ChatLog{{ChatSessionID: "body-session", TurnIndex: 17, Content: "The existing story continues."}}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /config/body-tracking/{chat_session_id}/state", srv.handleBodyTrackingState)
	apply := func(request bodyTrackingStateRequest) map[string]any {
		t.Helper()
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/config/body-tracking/body-session/state", bytes.NewReader(data)))
		if rec.Code != http.StatusOK {
			t.Fatalf("manual correction response %d: %s", rec.Code, rec.Body.String())
		}
		return parseJSONMap(rec.Body.String())
	}
	request := bodyTrackingStateRequest{OperationID: "author-pregnancy", CharacterID: "mina-id", Event: map[string]any{"kind": "pregnancy_confirmed", "evidence_excerpt": "Author confirmed the existing pregnancy."}, DryRun: true}
	preview := apply(request)
	if preview["status"] != "preview" || len(bodyHistory46(base)) != 0 || len(st.savedStatusDefinitions) != 0 {
		t.Fatal("preview wrote state/history/registry")
	}
	request.DryRun = false
	first := apply(request)
	if first["before"] != nil || mapFromAny(mapFromAny(first["current_state"])["pregnancy"])["occurred_at"] != nil {
		t.Fatalf("manual unknown date invented an occurrence: %#v", first)
	}
	events := bodyHistory46(base)
	if len(events) != 1 || events[0].SourceTurn != 0 || store.StatusChangeEventObservationTurn(events[0]) != 17 || stringFromMap(parseJSONMap(events[0].EvidenceJSON), "source") != "author_setting" {
		t.Fatalf("manual fact conflated source and record order: %#v", events)
	}
	apply(request)
	if len(bodyHistory46(base)) != 1 {
		t.Fatal("same explicit operation duplicated history")
	}
	undo := apply(bodyTrackingStateRequest{Action: "undo", EventID: int64(first["event_id"].(float64))})
	if undo["current"] != nil || undo["current_state"] != nil || len(st.returnStatusCurrent) != 0 {
		t.Fatalf("undo did not restore current absence: %#v", undo)
	}
	oldReplay := apply(request)
	if oldReplay["current"] != nil || oldReplay["replayed"] != true || len(bodyHistory46(base)) != 2 {
		t.Fatalf("old apply replay overrode undo: %#v", oldReplay)
	}
}

func Test46BodyTrackingManualCorrectionOverridesKnownDateAndUndoRestoresSource(t *testing.T) {
	srv, base, _ := bodyState46Fixture(t, true)
	saveBodyEvent46(t, srv, 4, bodyEvent46("period_start", "original-source", "1423-02-10"))
	before := bodyCurrent46(t, srv)
	srv.Store = &adminRepairProjectionRecordingStore{turnRecordingStore: base}
	result, err := srv.runBodyTrackingState(context.Background(), "body-session", bodyTrackingStateRequest{CharacterID: "mina-id", OperationID: "correct-older-date", Event: map[string]any{"kind": "period_start", "occurred_at": map[string]any{"date": "1423-02-08"}}})
	if err != nil {
		t.Fatal(err)
	}
	if stringFromMap(mapFromAny(mapFromAny(result["current_state"])["cycle_reference"]), "date") != "1423-02-08" {
		t.Fatalf("explicit correction treated as historical recall: %#v", result)
	}
	_, err = srv.runBodyTrackingState(context.Background(), "body-session", bodyTrackingStateRequest{Action: "undo", EventID: result["event_id"].(int64)})
	if err != nil {
		t.Fatal(err)
	}
	after := bodyCurrent46(t, srv)
	if after.ValueJSON != before.ValueJSON || stringFromMap(parseJSONMap(after.EvidenceJSON), "evidence_excerpt") != stringFromMap(parseJSONMap(before.EvidenceJSON), "evidence_excerpt") {
		t.Fatalf("undo changed original facts or attributed correction proof to them: before=%+v after=%+v", before, after)
	}
}

func Test46ObservedReconfirmationPreservesExistingTerm(t *testing.T) {
	for _, withStart := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit_start_%v", withStart), func(t *testing.T) {
			srv, _, _ := bodyState46Fixture(t, true)
			save := func(turn int, key, date string, includeStart bool) {
				content := "On " + date + ", Mina receives a clinic confirmation that she is pregnant."
				clock := storyClockProposal("absolute", "current", "exact", content)
				clock["absolute"] = map[string]any{"date": date}
				event := bodyEvent46("pregnancy_confirmed", key, date)
				event["evidence_excerpt"] = content
				if includeStart {
					event["conception_time"] = map[string]any{"date": "1423-01-01"}
					content += " The clinic dates the pregnancy start to 1423-01-01."
					event["evidence_excerpt"] = content
				}
				extraction := map[string]any{"turn_summary": content, "body_events": []any{event}, "story_clock": clock, "evidence_excerpts": []any{content}}
				result := srv.saveCriticExtractionArtifacts(acceptedStoryClockContext("audit-source-"+key, "audit-turn-"+key, "generation"), "body-session", turn, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(int64(turn), 0), nil)
				if result.Errors != 0 {
					t.Fatalf("save errors: %v", result.ErrorDetails)
				}
			}
			save(1, "initial-clinic-result", "1423-01-05", withStart)
			before := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["pregnancy"])
			save(2, "followup-clinic-result", "1423-02-01", false)
			after := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["pregnancy"])
			if mustCompactJSON(before["modeled_birth_time"]) != mustCompactJSON(after["modeled_birth_time"]) {
				t.Fatalf("same ongoing pregnancy term moved: before=%s (%v), after=%s (%v); previous start=%s, retained start=%s", mustCompactJSON(before["modeled_birth_time"]), before["term_basis"], mustCompactJSON(after["modeled_birth_time"]), after["term_basis"], mustCompactJSON(before["conception_time"]), mustCompactJSON(after["conception_time"]))
			}
		})
	}
}
func saveBodyContinuity46(t *testing.T, srv *Server, turn int, sceneDate string, event map[string]any) {
	t.Helper()
	content := stringFromMap(event, "evidence_excerpt")
	clock := storyClockProposal("absolute", "current", "exact", "Date: "+sceneDate)
	clock["absolute"] = map[string]any{"date": sceneDate}
	extraction := map[string]any{"turn_summary": content, "body_events": []any{event}, "story_clock": clock, "evidence_excerpts": []any{content}}
	result := srv.saveCriticExtractionArtifacts(acceptedStoryClockContext(fmt.Sprintf("audit-source-%d", turn), fmt.Sprintf("audit-turn-%d", turn), "generation"), "body-session", turn, extraction, "Date: "+sceneDate+". "+content, completeTurnEmbeddingConfig{}, time.Unix(int64(turn), 0), nil)
	if result.Errors != 0 {
		t.Fatalf("save errors: %v", result.ErrorDetails)
	}
}

func Test46NewPregnancyAfterEndingDoesNotForecastOrdinaryCycle(t *testing.T) {
	for _, ending := range []string{"explicit_end", "modeled_birth"} {
		t.Run(ending, func(t *testing.T) {
			srv, _, cfg := bodyModelState46Fixture(t, 1)
			cfg.CycleTrackingEnabled = true
			if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
				t.Fatal(err)
			}
			initial := bodyEvent46("pregnancy_confirmed", "initial", "1423-01-05")
			initial["conception_time"] = map[string]any{"date": "1423-01-01"}
			saveBodyContinuity46(t, srv, 1, "1423-01-05", initial)
			period, exposure, now := "1423-10-01", "1423-10-14", "1423-11-01"
			if ending == "explicit_end" {
				saveBodyContinuity46(t, srv, 2, "1423-01-20", bodyEvent46("pregnancy_ended", "ended", "1423-01-20"))
				period, exposure, now = "1423-02-01", "1423-02-14", "1423-03-01"
			}
			saveBodyContinuity46(t, srv, 3, period, bodyEvent46("period_start", "new-period", period))
			saveBodyContinuity46(t, srv, 4, exposure, bodyExposure46("new-exposure", exposure))
			current := bodyCurrent46(t, srv)
			projection := parseJSONMap(current.ValueJSON)
			clock := map[string]any{"date": now}
			model := bodyTrackingModelReading(projection, clock)
			if model["stage"] != "modeled_implanted_pregnancy" {
				t.Fatalf("fixture did not establish a new pregnancy: %v", model)
			}
			cycle := bodyTrackingCycleReading(cfg.Characters[0], projection, clock)
			body := &prepareTurnBodyTrackingContext{SessionID: "body-session", Config: cfg, Values: []store.StatusCurrentValue{current}}
			input := bodyAssemblyInput46(body, clock, store.CharacterState{ChatSessionID: "body-session", CharacterName: "Mina", AppearanceJSON: `{"gender":"female"}`})
			input.UserInput = "Mina checks her current body state."
			input.Perspective.Selection.Query = input.UserInput
			out := buildPrepareTurnInjectionAssemblyWithBudget(input)
			delivered := stringFromMap(out.MemoryDeliveryPlan, "final_text")
			if cycle["status"] == "estimated" || strings.Contains(delivered, "next_period_estimate") {
				t.Fatalf("active new pregnancy accompanies ordinary cycle: stage=%v cycle_status=%v reason=%v delivered_forecast=%v delivered_pregnancy=%v", model["stage"], cycle["status"], cycle["reason"], strings.Contains(delivered, "next_period_estimate"), strings.Contains(delivered, "modeled_implanted_pregnancy"))
			}
		})
	}
}

func Test46SameEventDateClarificationUpdatesState(t *testing.T) {
	srv, _, _ := bodyState46Fixture(t, true)
	saveBodyContinuity46(t, srv, 1, "1423-01-05", bodyEvent46("period_start", "same-period", ""))
	saveBodyContinuity46(t, srv, 2, "1423-01-06", bodyEvent46("period_start", "same-period", "1423-01-04"))
	state := parseJSONMap(bodyCurrent46(t, srv).ValueJSON)
	if mapFromAny(state["cycle_reference"])["date"] != "1423-01-04" {
		t.Fatalf("later known date was ignored: reference=%s", mustCompactJSON(state["cycle_reference"]))
	}
}

func Test46PlannedEventCanBecomeActualUnderSameKey(t *testing.T) {
	srv, st, _ := bodyState46Fixture(t, true)
	planned := bodyEvent46("pregnancy_confirmed", "clinic-check", "1423-01-10")
	planned["scene_scope"] = "hypothetical"
	saveBodyContinuity46(t, srv, 1, "1423-01-05", planned)
	actual := bodyEvent46("pregnancy_confirmed", "clinic-check", "1423-01-10")
	actual["scene_scope"] = "current"
	saveBodyContinuity46(t, srv, 2, "1423-01-10", actual)
	values, err := srv.bodyTrackingCurrentValues(context.Background(), "body-session")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range values {
		if mapFromAny(parseJSONMap(v.ValueJSON)["pregnancy"])["status"] == "confirmed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("actual confirmation omitted after hypothetical same-key observation; history=%d current=%d", len(bodyHistory46(st)), len(values))
	}
}

func Test46ManualEndStillAppliesAfterLaterPeriodObservation(t *testing.T) {
	srv, st, _ := bodyModelState46Fixture(t, 1)
	saveBodyContinuity46(t, srv, 1, "1423-01-14", bodyExposure46("first", "1423-01-14"))
	srv.Store = &adminRepairProjectionRecordingStore{turnRecordingStore: st}
	if _, err := srv.runBodyTrackingState(context.Background(), "body-session", bodyTrackingStateRequest{CharacterID: "mina-id", OperationID: "end", Event: map[string]any{"kind": "pregnancy_ended", "occurred_at": map[string]any{"date": "1423-02-01"}}}); err != nil {
		t.Fatal(err)
	}
	saveBodyContinuity46(t, srv, 2, "1423-02-02", bodyEvent46("period_start", "later-period", "1423-02-02"))
	saveBodyContinuity46(t, srv, 3, "1423-02-15", bodyExposure46("new", "1423-02-15"))
	events := bodyHistory46(st)
	model := bodyModelRecord46(t, events[len(events)-1])
	if model["reason"] == "modeled_pregnancy_ongoing" || model["reason"] == "observed_pregnancy_ongoing" {
		t.Fatalf("manual ending lost after unrelated period write: %v", model)
	}
}
func Test46SameEventKnownDateCorrectionReplacesEarlierObservation(t *testing.T) {
	srv, st, cfg := bodyState46Fixture(t, true)
	saveBodyContinuity46(t, srv, 1, "1423-01-05", bodyEvent46("period_start", "same-period", "1423-01-05"))
	saveBodyContinuity46(t, srv, 2, "1423-01-06", bodyEvent46("period_start", "same-period", "1423-01-03"))
	current := bodyCurrent46(t, srv)
	if mapFromAny(parseJSONMap(current.ValueJSON)["cycle_reference"])["date"] != "1423-01-03" {
		t.Fatal("same-event correction was treated as an older different event")
	}
	spec := bodyTrackingCycleSpecAtOccurrence(cfg, current, bodyExposure46("later", "1423-01-16"), bodyHistory46(st))
	if spec.ReferenceTime["date"] != "1423-01-03" {
		t.Fatalf("history selected superseded date: %v", spec.ReferenceTime)
	}
	count := len(bodyHistory46(st))
	saveBodyContinuity46(t, srv, 3, "1423-01-07", bodyEvent46("period_start", "same-period", "1423-01-03"))
	if len(bodyHistory46(st)) != count {
		t.Fatal("unchanged repeat created another event")
	}
}
func Test46SameExposureClarificationRetainsFrozenDraw(t *testing.T) {
	srv, st, _ := bodyModelState46Fixture(t, 1)
	original := bodyExposure46("same-exposure", "1423-01-14")
	saveBodyContinuity46(t, srv, 1, "1423-01-14", original)
	before := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
	refined := bodyExposure46("same-exposure", "1423-01-14")
	refined["knowledge_source"] = "clinic history"
	saveBodyContinuity46(t, srv, 2, "1423-01-15", refined)
	records := bodyHistory46(st)
	result := bodyModelRecord46(t, records[len(records)-1])
	after := mapFromAny(result["selected_pregnancy"])
	if len(after) == 0 || mustCompactJSON(before["day_draw"]) != mustCompactJSON(after["day_draw"]) {
		t.Fatalf("refinement blocked by its own old model or changed draw: %v", result)
	}
	corrected := bodyExposure46("same-exposure", "1423-01-14")
	mapFromAny(corrected["exposure"])["classification"] = "not_potentially_conceiving"
	saveBodyContinuity46(t, srv, 3, "1423-01-16", corrected)
	if len(mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])) > 0 {
		t.Fatal("corrected event kept its obsolete modeled pregnancy")
	}
}

func Test46SameConfirmationDateCorrectionUpdatesModelTerm(t *testing.T) {
	srv, _, _ := bodyState46Fixture(t, true)
	saveBodyContinuity46(t, srv, 1, "1423-01-05", bodyEvent46("pregnancy_confirmed", "clinic", "1423-01-05"))
	saveBodyContinuity46(t, srv, 2, "1423-01-06", bodyEvent46("pregnancy_confirmed", "clinic", "1423-01-02"))
	state := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["pregnancy"])
	expected := storyTimeRelative(map[string]any{"date": "1423-01-02"}, map[string]any{"anchor": "source_observation", "unit": "day", "offset": state["gestation_days"]})
	if mustCompactJSON(state["modeled_birth_time"]) != mustCompactJSON(expected) {
		t.Fatalf("same confirmation correction kept wrong model anchor: %v", state)
	}
}
