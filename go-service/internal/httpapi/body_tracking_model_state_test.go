package httpapi

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func bodyModelState46Fixture(t *testing.T, viability float64) (*Server, *turnRecordingStore, bodyTrackingConfig) {
	t.Helper()
	srv, st, character := bodyState46Fixture(t, false)
	character.Cycle.LutealMinDays, character.Cycle.LutealMaxDays = 14, 14
	character.CycleViability, character.ConditionalPeak = viability, 1
	cfg, err := srv.saveBodyTrackingConfig("body-session", bodyTrackingConfig{AutomaticPregnancyEnabled: true, SimulationSeed: "persisted-fiction-seed", Characters: []bodyCharacterConfig{character}})
	if err != nil {
		t.Fatal(err)
	}
	return srv, st, cfg
}

func bodyExposure46(key, date string) map[string]any {
	event := bodyEvent46("conception_exposure", key, date)
	event["exposure"] = map[string]any{"classification": "potentially_conceiving"}
	return event
}

func bodyModelRecord46(t *testing.T, event store.StatusChangeEvent) map[string]any {
	t.Helper()
	model := mapFromAny(parseJSONMap(event.EvidenceJSON)["model_result"])
	if len(model) == 0 {
		t.Fatalf("source event omitted model interpretation: %#v", event)
	}
	return model
}

func Test46BodyModelSourcePersistsOneResultAndSameDayDoesNotResample(t *testing.T) {
	srv, st, cfg := bodyModelState46Fixture(t, 1)
	first := bodyExposure46("first-exposure", "1423-01-14") // O-1, conditional peak=1.
	saveBodyEvent46(t, srv, 1, first)
	history := bodyHistory46(st)
	model := bodyModelRecord46(t, history[0])
	current := parseJSONMap(bodyCurrent46(t, srv).ValueJSON)
	if model["status"] != "evaluated" || model["outcome"] != "latent_model_implantation" || current["pregnancy"] != nil || current["knowledge"] != nil {
		t.Fatalf("modeled result became an observed diagnosis/knowledge: %#v %#v", model, current)
	}
	if len(sliceFromAny(parseJSONMap(history[0].EvidenceJSON)["model_cycles"])) == 0 || strings.Contains(history[0].EvidenceJSON, cfg.SimulationSeed) {
		t.Fatal("immutable cycle parameters missing or private seed copied into source evidence")
	}
	selected := mapFromAny(current["modeled_pregnancy"])
	if stringFromMap(mapFromAny(selected["source"]), "source_revision") != "body-source-1" || selected["visibility"] != "private" {
		t.Fatalf("modeled projection lost source/privacy: %#v", selected)
	}
	saveBodyEvent46(t, srv, 2, bodyExposure46("same-day-distinct-observation", "1423-01-14"))
	second := bodyModelRecord46(t, bodyHistory46(st)[1])
	if mustCompactJSON(model["cycles"]) != mustCompactJSON(second["cycles"]) {
		t.Fatal("second mention on one exposure day received a new random trial")
	}
	before := bodyCurrent46(t, srv).ValueJSON
	cfg.Characters[0].CycleViability = 0
	if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
		t.Fatal(err)
	}
	saveBodyEvent46(t, srv, 9, first)
	if len(bodyHistory46(st)) != 2 || bodyCurrent46(t, srv).ValueJSON != before {
		t.Fatal("semantic replay sampled again or rewrote current after settings changed")
	}
	saveBodyEvent46(t, srv, 10, bodyExposure46("later-while-ongoing", "1423-01-16"))
	blocked := bodyModelRecord46(t, bodyHistory46(st)[2])
	if blocked["reason"] != "modeled_pregnancy_ongoing" || len(sliceFromAny(blocked["cycles"])) != 0 {
		t.Fatalf("ongoing modeled pregnancy received another trial: %#v", blocked)
	}
	saveBodyEvent46(t, srv, 11, bodyEvent46("pregnancy_ended", "explicit-end", "1423-02-01"))
	if parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"] != nil {
		t.Fatal("explicit ending left a current modeled pregnancy")
	}
}

func Test46BodyModelUnknownDisabledAndObservedPregnancyKeepExposureHistory(t *testing.T) {
	srv, st, cfg := bodyModelState46Fixture(t, 1)
	saveBodyEvent46(t, srv, 1, bodyExposure46("unknown-date", ""))
	if bodyModelRecord46(t, bodyHistory46(st)[0])["status"] != "unknown" {
		t.Fatal("unknown occurrence borrowed current story date")
	}
	contradicted := bodyExposure46("contraception-context", "1423-01-14")
	contradicted["exposure"] = map[string]any{"model_profile": "author_allowed", "contraception": "present"}
	saveBodyEvent46(t, srv, 2, contradicted)
	if bodyModelRecord46(t, bodyHistory46(st)[1])["status"] == "evaluated" {
		t.Fatal("an exposure profile silently overrode contraception context")
	}
	cfg.AutomaticPregnancyEnabled, cfg.CycleTrackingEnabled = false, true
	if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
		t.Fatal(err)
	}
	saveBodyEvent46(t, srv, 3, bodyExposure46("cycle-only-exposure", "1423-01-14"))
	if bodyModelRecord46(t, bodyHistory46(st)[2])["status"] != "disabled" {
		t.Fatal("cycle-only setting ran pregnancy simulation")
	}
	cfg.AutomaticPregnancyEnabled = true
	if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
		t.Fatal(err)
	}
	saveBodyEvent46(t, srv, 4, bodyEvent46("pregnancy_confirmed", "observed-confirmation", "1423-01-10"))
	saveBodyEvent46(t, srv, 5, bodyExposure46("known-pregnancy-exposure", "1423-01-14"))
	if bodyModelRecord46(t, bodyHistory46(st)[4])["reason"] != "observed_pregnancy_ongoing" || len(bodyHistory46(st)) != 5 {
		t.Fatal("confirmed pregnancy did not override a new model trial, or source observation was discarded")
	}
}

func Test46BodyModelHistoricalExposureUsesPeriodAtOccurrence(t *testing.T) {
	srv, st, _ := bodyModelState46Fixture(t, 0)
	saveBodyEvent46(t, srv, 1, bodyEvent46("period_start", "january-period", "1423-01-02"))
	saveBodyEvent46(t, srv, 2, bodyEvent46("period_start", "february-period", "1423-02-02"))
	before := bodyCurrent46(t, srv).ValueJSON
	exposure := bodyExposure46("historical-january-exposure", "1423-01-15")
	exposure["scene_scope"] = "flashback"
	saveBodyEvent46(t, srv, 3, exposure)
	event := bodyHistory46(st)[2]
	model := bodyModelRecord46(t, event)
	if model["status"] != "evaluated" || stringFromMap(mapFromAny(mapFromAny(model["configuration"])["cycle_reference"]), "date") != "1423-01-02" {
		t.Fatalf("historical exposure used a later current period or settings reference: %#v", model)
	}
	if event.EventState != "history_only" || bodyCurrent46(t, srv).ValueJSON != before {
		t.Fatal("historical model interpretation rewrote current body facts")
	}
}

func Test46BodyModelFrozenActiveCyclesSurviveSettingsAndIgnoreInvalidatedSources(t *testing.T) {
	srv, st, cfg := bodyModelState46Fixture(t, 0)
	owner := &completeTurnReprocessingStore{turnRecordingStore: st, sources: map[string]*store.MemorySourceRevision{}}
	srv.Store = owner
	save := func(turn int, key, date string) {
		revision := fmt.Sprintf("body-source-%d", turn)
		owner.sources[revision] = &store.MemorySourceRevision{ChatSessionID: "body-session", SourceRevision: revision, TurnIndex: turn, LifecycleState: "active"}
		result := artifactSaveResult{}
		srv.saveBodyTrackingFromExtraction(acceptedStoryClockContext(revision, fmt.Sprintf("body-turn-%d", turn), "generation"), "body-session", turn,
			map[string]any{"body_events": []any{bodyExposure46(key, date)}}, "Mina records this observation.", nil, time.Unix(int64(turn), 0), &result)
		if result.Errors != 0 {
			t.Fatal(result.ErrorDetails)
		}
	}
	save(1, "cycle-first", "1423-01-13")
	cfg.Characters[0].CycleViability = 1
	if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
		t.Fatal(err)
	}
	save(2, "cycle-next-day", "1423-01-14")
	second := bodyModelRecord46(t, bodyHistory46(st)[1])
	if second["outcome"] != "no_modeled_implantation_from_this_day" {
		t.Fatalf("new settings resampled a frozen cycle's viability: %#v", second)
	}
	owner.sources["body-source-1"].LifecycleState = "invalidated"
	active, err := srv.bodyTrackingActiveHistory(context.Background(), "body-session", cfg.Characters[0].EntityID)
	if err != nil || len(active) != 1 {
		t.Fatalf("active history retained a replaced source: %#v %v", active, err)
	}
	// The surviving later event carries the entire original immutable cycle,
	// so deletion of its first source does not reset viability or timeline.
	save(3, "surviving-cycle", "1423-01-14")
	if bodyModelRecord46(t, bodyHistory46(st)[2])["outcome"] != "no_modeled_implantation_from_this_day" {
		t.Fatal("deleting an earlier checkpoint changed the surviving cycle outcome")
	}
	owner.sources["body-source-2"].LifecycleState = "invalidated"
	owner.sources["body-source-3"].LifecycleState = "invalidated"
	st.returnStatusCurrent = nil // The existing rollback owner restores absence.
	save(4, "cycle-first", "1423-01-14")
	if bodyModelRecord46(t, bodyHistory46(st)[3])["outcome"] != "latent_model_implantation" {
		t.Fatal("inactive event identity or checkpoint blocked the replacement source")
	}
}

func Test46BodyModelUsesManualReferenceAndUndoWithoutRevivingCorrection(t *testing.T) {
	srv, st, _ := bodyModelState46Fixture(t, 0)
	saveBodyEvent46(t, srv, 1, bodyEvent46("period_start", "source-period", "1423-01-02"))
	srv.Store = &adminRepairProjectionRecordingStore{turnRecordingStore: st}
	correction, err := srv.runBodyTrackingState(context.Background(), "body-session", bodyTrackingStateRequest{CharacterID: "mina-id", OperationID: "correct-reference", Event: map[string]any{"kind": "period_start", "occurred_at": map[string]any{"date": "1423-01-03"}}})
	if err != nil {
		t.Fatal(err)
	}
	saveBodyEvent46(t, srv, 2, bodyExposure46("corrected-reference-exposure", "1423-01-16"))
	model := bodyModelRecord46(t, bodyHistory46(st)[2])
	if stringFromMap(mapFromAny(mapFromAny(model["configuration"])["cycle_reference"]), "date") != "1423-01-03" {
		t.Fatal("explicit author correction did not provide the model reference")
	}
	if _, err := srv.runBodyTrackingState(context.Background(), "body-session", bodyTrackingStateRequest{Action: "undo", EventID: correction["event_id"].(int64)}); err != nil {
		t.Fatal(err)
	}
	saveBodyEvent46(t, srv, 3, bodyExposure46("restored-reference-exposure", "1423-01-15"))
	model = bodyModelRecord46(t, bodyHistory46(st)[4])
	if stringFromMap(mapFromAny(mapFromAny(model["configuration"])["cycle_reference"]), "date") != "1423-01-02" {
		t.Fatal("historical correction overrode its explicit undo")
	}
	saveBodyEvent46(t, srv, 4, bodyEvent46("period_start", "unknown-later-period", ""))
	saveBodyEvent46(t, srv, 5, bodyExposure46("unknown-reference-exposure", "1423-02-01"))
	model = bodyModelRecord46(t, bodyHistory46(st)[6])
	if model["status"] != "unknown" || model["reason"] != "exact_event_date_or_reference_unresolved" {
		t.Fatalf("unknown later period was replaced by a convenient earlier reference: %#v", model)
	}
}

func Test46ParentagePersistsCounterpartAcrossConfirmationAndLaterRelationship(t *testing.T) {
	srv, st, cfg := bodyModelState46Fixture(t, 1)
	first := bodyExposure46("first", "1423-01-14")
	first["partners"] = []any{map[string]any{"entity_id": "arin-id", "character_name": "Arin"}}
	saveBodyEvent46(t, srv, 1, first)
	model := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
	frozen := mustCompactJSON(model["day_draw"])
	p := mapFromAny(model["paternity"])
	if p["status"] != "modeled_link" || !strings.Contains(mustCompactJSON(p), "arin-id") {
		t.Fatalf("lost initial counterpart: %v", p)
	}
	other := bodyExposure46("later", "1423-01-20")
	other["partners"] = []any{map[string]any{"entity_id": "borin-id", "character_name": "Borin"}}
	saveBodyEvent46(t, srv, 2, other)
	saveBodyEvent46(t, srv, 3, bodyEvent46("pregnancy_confirmed", "confirmed", "1423-02-01"))
	saveBodyEvent46(t, srv, 4, bodyEvent46("pregnancy_confirmed", "reconfirmed", "1423-02-05"))
	projection := parseJSONMap(bodyCurrent46(t, srv).ValueJSON)
	p = bodyTrackingPaternityReading(projection, map[string]any{"date": "1423-02-05"})
	if !strings.Contains(mustCompactJSON(p), "Arin") || strings.Contains(mustCompactJSON(p), "Borin") || p["status"] != "modeled_link" {
		t.Fatalf("parentage switched or became confirmed: %v", p)
	}
	if mustCompactJSON(mapFromAny(projection["modeled_pregnancy"])["day_draw"]) != frozen {
		t.Fatal("parentage caused resampling")
	}
	before := bodyCurrent46(t, srv).ValueJSON
	for _, clock := range []map[string]any{{"date": "1423-02-05"}, {"date": "1426-02-05"}, {}} {
		if got := bodyTrackingPaternityReading(projection, clock); !strings.Contains(mustCompactJSON(got), "Arin") {
			t.Fatalf("time passage lost counterpart: %v", got)
		}
	}
	bodySettingsRequest46(t, srv, "body-session", "GET", "")
	if bodyCurrent46(t, srv).ValueJSON != before {
		t.Fatal("settings mutated parentage")
	}
	// Ending and a later conception start a separate association.
	saveBodyEvent46(t, srv, 5, bodyEvent46("pregnancy_ended", "end", "1423-02-06"))
	next := bodyExposure46("next-pregnancy", "1423-02-11")
	next["partners"] = other["partners"]
	saveBodyEvent46(t, srv, 6, next)
	p = bodyTrackingPaternityReading(parseJSONMap(bodyCurrent46(t, srv).ValueJSON), map[string]any{"date": "1423-02-20"})
	if !strings.Contains(mustCompactJSON(p), "Borin") || strings.Contains(mustCompactJSON(p), "Arin") {
		t.Fatalf("new pregnancy inherited old father: %v", p)
	}
	_ = st
	_ = cfg
}

func Test46ParentageMultipleAndUnknownCounterpartsDoNotChooseFather(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprint(unknown), func(t *testing.T) {
			srv, st, _ := bodyModelState46Fixture(t, 1)
			a := bodyExposure46("a", "1423-01-14")
			a["partners"] = []any{map[string]any{"entity_id": "a", "character_name": "Arin"}}
			saveBodyEvent46(t, srv, 1, a)
			before := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
			b := bodyExposure46("b", "1423-01-14")
			if !unknown {
				b["partners"] = []any{map[string]any{"entity_id": "b", "character_name": "Borin"}}
			}
			saveBodyEvent46(t, srv, 2, b)
			after := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
			p := mapFromAny(after["paternity"])
			want := 2
			if unknown {
				want = 1
			}
			if p["status"] != "candidates" || len(sliceFromAny(p["candidates"])) != want || p["unidentified_partner"] != unknown {
				t.Fatalf("ambiguous counterpart selected: %v", p)
			}
			if mustCompactJSON(before["day_draw"]) != mustCompactJSON(after["day_draw"]) {
				t.Fatal("same-day counterpart added a trial")
			}
			count := len(bodyHistory46(st))
			saveBodyEvent46(t, srv, 3, b)
			if len(bodyHistory46(st)) != count {
				t.Fatal("repeated relation appended parentage event")
			}
		})
	}
	srv, _, _ := bodyModelState46Fixture(t, 1)
	saveBodyEvent46(t, srv, 1, bodyExposure46("unknown", "1423-01-14"))
	p := mapFromAny(mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])["paternity"])
	if p["status"] != "unknown" {
		t.Fatalf("missing partner fabricated father or blocked simulation: %v", p)
	}
}

func Test46ParentageSameEventCorrectionAndManualUndo(t *testing.T) {
	srv, st, _ := bodyModelState46Fixture(t, 1)
	e := bodyExposure46("same", "1423-01-14")
	e["partners"] = []any{map[string]any{"character_name": "Wrong"}}
	saveBodyEvent46(t, srv, 1, e)
	before := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
	e["partners"] = []any{map[string]any{"character_name": "Arin"}}
	saveBodyEvent46(t, srv, 2, e)
	after := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
	if strings.Contains(mustCompactJSON(after["paternity"]), "Wrong") || !strings.Contains(mustCompactJSON(after["paternity"]), "Arin") || mustCompactJSON(before["day_draw"]) != mustCompactJSON(after["day_draw"]) {
		t.Fatalf("same-event partner correction failed: %v", after["paternity"])
	}
	saveBodyEvent46(t, srv, 3, bodyEvent46("pregnancy_confirmed", "confirmation", "1423-02-01"))
	baseline := bodyCurrent46(t, srv).ValueJSON
	srv.Store = &adminRepairProjectionRecordingStore{turnRecordingStore: st}
	correction := bodyEvent46("pregnancy_confirmed", "", "1423-02-02")
	correction["paternity"] = map[string]any{"status": "confirmed", "candidates": []any{map[string]any{"character_name": "Borin"}}}
	result, err := srv.runBodyTrackingState(context.Background(), "body-session", bodyTrackingStateRequest{CharacterID: "mina-id", OperationID: "paternity-correction", Event: correction})
	if err != nil {
		t.Fatal(err)
	}
	p := bodyTrackingPaternityReading(parseJSONMap(bodyCurrent46(t, srv).ValueJSON), map[string]any{"date": "1423-02-02"})
	if p["status"] != "confirmed" || !strings.Contains(mustCompactJSON(p), "Borin") {
		t.Fatalf("author correction ignored: %v", p)
	}
	if _, err = srv.runBodyTrackingState(context.Background(), "body-session", bodyTrackingStateRequest{Action: "undo", EventID: result["event_id"].(int64)}); err != nil {
		t.Fatal(err)
	}
	if bodyCurrent46(t, srv).ValueJSON != baseline {
		t.Fatal("undo did not restore association")
	}
	correction["paternity"] = map[string]any{"status": "unknown"}
	if _, err = srv.runBodyTrackingState(context.Background(), "body-session", bodyTrackingStateRequest{CharacterID: "mina-id", OperationID: "paternity-unknown", Event: correction}); err != nil {
		t.Fatal(err)
	}
	if p = bodyTrackingPaternityReading(parseJSONMap(bodyCurrent46(t, srv).ValueJSON), map[string]any{}); p["status"] != "unknown" || len(sliceFromAny(p["candidates"])) != 0 {
		t.Fatalf("unknown correction revived old father: %v", p)
	}
}

func Test46ParentageResolvesAliasesWithoutRequiringAnIdentity(t *testing.T) {
	srv, st, _ := bodyModelState46Fixture(t, 1)
	identities := newIdentityAliasLinkRecordingStore()
	identities.turnRecordingStore = st
	identities.identities = append(identities.identities, &store.EntityIdentity{StableEntityID: "arin-id", ChatSessionID: "body-session", EntityKind: "character", CanonicalLabel: "Arin"})
	identities.surfaces = append(identities.surfaces, &store.EntityIdentitySurface{ChatSessionID: "body-session", StableEntityID: "arin-id", NormalizedSurface: comparableEntityKey("Captain"), Scope: store.EntityIdentitySurfaceScopeCurrent, ReviewState: "source_observed"})
	srv.Store = identities
	got := srv.bodyTrackingPeople(context.Background(), "body-session", []any{map[string]any{"character_name": "Captain"}, map[string]any{"character_name": "Unnamed visitor"}})
	if len(got) != 2 || mapFromAny(got[0])["entity_id"] != "arin-id" || mapFromAny(got[1])["character_name"] != "Unnamed visitor" || mapFromAny(got[1])["entity_id"] != nil {
		t.Fatalf("identity resolution invented/dropped counterpart: %v", got)
	}
}

func Test46ParentageSourceCorrectionAfterConfirmationUpdatesInheritedOnly(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		t.Run(fmt.Sprint(confirmed), func(t *testing.T) {
			srv, _, _ := bodyModelState46Fixture(t, 1)
			e := bodyExposure46("origin", "1423-01-14")
			e["partners"] = []any{map[string]any{"character_name": "Arin"}}
			saveBodyEvent46(t, srv, 1, e)
			fact := bodyEvent46("pregnancy_confirmed", "confirmation", "1423-02-01")
			if confirmed {
				fact["paternity"] = map[string]any{"status": "confirmed", "model_event_key": "origin", "candidates": []any{map[string]any{"character_name": "ConfirmedFather"}}}
			}
			saveBodyEvent46(t, srv, 2, fact)
			e["partners"] = []any{map[string]any{"character_name": "Borin"}}
			saveBodyEvent46(t, srv, 3, e)
			projection := parseJSONMap(bodyCurrent46(t, srv).ValueJSON)
			p := mapFromAny(mapFromAny(projection["pregnancy"])["paternity"])
			want := "Borin"
			if confirmed {
				want = "ConfirmedFather"
			}
			if !strings.Contains(mustCompactJSON(p), want) || strings.Contains(mustCompactJSON(p), "Arin") {
				t.Fatalf("source correction lost parentage authority: %v", p)
			}
			e["exposure"] = map[string]any{"classification": "not_potentially_conceiving"}
			saveBodyEvent46(t, srv, 4, e)
			p = mapFromAny(mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["pregnancy"])["paternity"])
			if !confirmed && p["status"] != "unknown" {
				t.Fatalf("withdrawn model basis retained father: %v", p)
			}
			if confirmed && !strings.Contains(mustCompactJSON(p), want) {
				t.Fatal("model correction erased confirmed father")
			}
		})
	}
}

func Test46ParentageOvulationDayCounterpartWithoutAnotherTrial(t *testing.T) {
	for _, day := range []string{"1423-01-15", "1423-01-16"} {
		t.Run(day, func(t *testing.T) {
			srv, st, _ := bodyModelState46Fixture(t, 1)
			first := bodyExposure46("first", "1423-01-14")
			first["partners"] = []any{map[string]any{"character_name": "Arin"}}
			saveBodyEvent46(t, srv, 1, first)
			original := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
			next := bodyExposure46("second", day)
			next["partners"] = []any{map[string]any{"character_name": "Borin"}}
			saveBodyEvent46(t, srv, 2, next)
			current := mapFromAny(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
			trial := bodyModelRecord46(t, bodyHistory46(st)[1])
			if trial["reason"] != "modeled_pregnancy_ongoing" || len(sliceFromAny(trial["cycles"])) != 0 || mustCompactJSON(current["day_draw"]) != mustCompactJSON(original["day_draw"]) {
				t.Fatal("counterpart association added a pregnancy trial")
			}
			p := mapFromAny(current["paternity"])
			if day == "1423-01-15" {
				if p["status"] != "candidates" || len(sliceFromAny(p["candidates"])) != 2 {
					t.Fatalf("ovulation-day counterpart omitted: %v", p)
				}
			} else if strings.Contains(mustCompactJSON(p), "Borin") {
				t.Fatalf("later partner replaced conception counterpart: %v", p)
			}
		})
	}
}
