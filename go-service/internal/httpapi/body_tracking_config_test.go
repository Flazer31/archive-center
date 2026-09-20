package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test46BodyExposureSettingsReadsSavedTrialWithoutResampling(t *testing.T) {
	for _, name := range []string{"not_recorded", "success", "saved_decision", "outside_window", "zero_setting", "unknown_date", "unknown_context", "contraception", "disabled", "incompatible", "observed_pregnancy"} {
		t.Run(name, func(t *testing.T) {
			srv, st, cfg := bodyModelState46Fixture(t, 1)
			event := bodyExposure46("settings-exposure", "1423-01-14")
			switch name {
			case "saved_decision":
				cfg.Characters[0].CycleViability, cfg.Characters[0].ConditionalPeak = .4, .7
			case "outside_window":
				event["occurred_at"] = map[string]any{"date": "1423-01-16"}
			case "zero_setting":
				cfg.Characters[0].CycleViability = 0
			case "unknown_date":
				delete(event, "occurred_at")
			case "unknown_context":
				event["exposure"] = map[string]any{"classification": "unknown"}
			case "contraception":
				mapFromAny(event["exposure"])["contraception"] = "present"
			case "disabled":
				cfg.CycleTrackingEnabled, cfg.AutomaticPregnancyEnabled = true, false
			case "incompatible":
				mapFromAny(event["exposure"])["partner_compatibility"] = "incompatible"
			case "observed_pregnancy":
				saveBodyEvent46(t, srv, 1, bodyEvent46("pregnancy_confirmed", "confirmed", "1423-01-10"))
			}
			if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
				t.Fatal(err)
			}
			if name != "not_recorded" {
				saveBodyEvent46(t, srv, 2, event)
			}
			before, _ := srv.bodyTrackingCurrentValues(context.Background(), "body-session")
			beforeJSON, historyCount := mustCompactJSON(before), len(st.savedStatusEvents)
			view := bodySettingsRequest46(t, srv, "body-session", http.MethodGet, "")
			items := sliceFromAny(view["exposure_readings"])
			if len(items) != 1 {
				t.Fatalf("settings omitted the exposure diagnostic: %#v", view)
			}
			r := mapFromAny(mapFromAny(items[0])["reading"])
			if name == "not_recorded" {
				if r["recorded"] != false || r["status"] != "not_recorded" || r["probabilities"] != nil {
					t.Fatalf("missing exposure became a zero/successful trial: %#v", r)
				}
				return
			}
			projection := parseJSONMap(before[0].ValueJSON)
			saved := mapFromAny(projection["latest_model_result"])
			if r["recorded"] != true || r["status"] != saved["status"] || r["outcome"] != saved["outcome"] || r["reason"] != saved["reason"] || r["evidence_excerpt"] != event["evidence_excerpt"] || r["source_turn"] != float64(2) {
				t.Fatalf("recorded exposure/status/source mismatch: %#v / %#v", r, saved)
			}
			probabilities, decisions := sliceFromAny(r["probabilities"]), sliceFromAny(saved["cycles"])
			if len(probabilities) != len(decisions) {
				t.Fatalf("cycle probabilities omitted or aggregated: %#v / %#v", probabilities, decisions)
			}
			for i, raw := range probabilities {
				p, decision := mapFromAny(raw), mapFromAny(decisions[i])
				if p["probability"] != decision["single_day_probability_given_latent_ovulation"] || mustCompactJSON(p["ovulation_time"]) != mustCompactJSON(decision["ovulation_time"]) {
					t.Fatalf("UI probability recalculated instead of reading saved trial: %#v / %#v", p, decision)
				}
				if (name == "outside_window" || name == "zero_setting") && p["probability"] != float64(0) {
					t.Fatalf("zero probability lost: %#v", p)
				}
			}
			for _, clock := range []map[string]any{{"date": "1423-01-14"}, {"date": "1423-01-20"}, {"date": "1426-01-20"}, {}} {
				later := bodyTrackingExposureSettingsReading(projection, clock)
				if mustCompactJSON(later["probabilities"]) != mustCompactJSON(r["probabilities"]) || later["outcome"] != r["outcome"] {
					t.Fatal("time passage changed saved chance/outcome")
				}
				if name != "unknown_date" && clock["date"] == "1423-01-20" {
					expected := storyTimeRelation(mapFromAny(event["occurred_at"]), clock)
					if mustCompactJSON(later["current_relation"]) != mustCompactJSON(expected) {
						t.Fatalf("elapsed reading lost story occurrence time: %#v", later)
					}
				}
			}
			for _, forbidden := range []string{"day_draw", "latent_success", "reference_family", cfg.SimulationSeed, "timeline"} {
				if strings.Contains(mustCompactJSON(r), forbidden) {
					t.Fatalf("settings leaked private model internals: %s", forbidden)
				}
			}
			// This diagnostic must not enlarge the existing compact AI reading.
			if strings.Contains(mustCompactJSON(bodyTrackingModelReading(projection, map[string]any{"date": "1423-01-20"})), "probabilit") {
				t.Fatal("settings-only probability leaked into memory reading")
			}
			if name == "success" {
				cfg.AutomaticPregnancyEnabled = false
				if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
					t.Fatal(err)
				}
				off := bodySettingsRequest46(t, srv, "body-session", http.MethodGet, "")
				offReading := mapFromAny(mapFromAny(sliceFromAny(off["exposure_readings"])[0])["reading"])
				if mustCompactJSON(offReading) != mustCompactJSON(r) || len(sliceFromAny(off["model_readings"])) != 0 {
					t.Fatal("turning off hid/recomputed saved exposure or enabled current model display")
				}
			}
			bodySettingsRequest46(t, srv, "body-session", http.MethodGet, "")
			after, _ := srv.bodyTrackingCurrentValues(context.Background(), "body-session")
			if mustCompactJSON(after) != beforeJSON || len(st.savedStatusEvents) != historyCount {
				t.Fatal("settings read mutated current/history")
			}
		})
	}
}

func Test46BodyExposureSettingsPreservesMultipleCyclesAndUnknownCalendar(t *testing.T) {
	cfg, character, spec := pregnancyReviewSetup46()
	spec.CycleDays, spec.PeriodDays, spec.LutealMinDays, spec.LutealMaxDays = 2, 1, 1, 1
	event := pregnancyReviewEvent46(4, "short-cycle-exposure")
	model, _ := calculateBodyPregnancyEvent(cfg, character, spec, event, nil)
	projection := parseJSONMap(mustCompactJSON(map[string]any{"observed_facts": map[string]any{"conception_exposure": event}, "latest_model_result": model}))
	r := bodyTrackingExposureSettingsReading(projection, map[string]any{"calendar": map[string]any{"id": "moon", "day_index": 20}})
	decisions := sliceFromAny(mapFromAny(projection["latest_model_result"])["cycles"])
	if len(decisions) < 2 || len(r["probabilities"].([]map[string]any)) != len(decisions) {
		t.Fatalf("multiple short-cycle candidates lost or aggregated: %#v", r)
	}
	if mapFromAny(r["current_relation"])["relation"] != "unknown" {
		t.Fatal("unrelated calendars invented elapsed days")
	}
}

func bodySettingsRequest46(t *testing.T, s *Server, sid, method, body string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	s.registerConfigRoutes(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(method, "/config/body-tracking/"+sid, strings.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("settings status %d: %s", recorder.Code, recorder.Body.String())
	}
	var view map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("simulation_seed")) {
		t.Fatal("settings UI exposed its internal simulation seed")
	}
	return view
}

func Test46BodyTrackingSettingsDefaultOffAndIndependentControls(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	s := &Server{}
	view := bodySettingsRequest46(t, s, "one", http.MethodGet, "")
	settings := mapFromAny(view["settings"])
	if settings["cycle_tracking_enabled"] != false || settings["automatic_pregnancy_enabled"] != false || len(settings["characters"].([]any)) != 0 {
		t.Fatalf("feature enabled without user selection: %#v", settings)
	}
	path, _ := bodyTrackingSettingsPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a GET created settings or simulation seed: %v", err)
	}
	bodySettingsRequest46(t, s, "one", http.MethodPut, `{"cycle_tracking_enabled":true,"automatic_pregnancy_enabled":false,"simulation_seed":"client-value","characters":[{"entity_id":"mira","character_name":"Mira"}]}`)
	first, err := s.loadBodyTrackingConfig("one")
	if err != nil || len(first.SimulationSeed) != 64 || first.SimulationSeed == "client-value" {
		t.Fatalf("missing backend seed: %#v %v", first, err)
	}
	character := first.Characters[0]
	if !character.CycleEnabled || !character.CanConceive || character.Cycle.CycleDays != 28 || character.Cycle.VariationDays != 2 || character.Cycle.PeriodDays != 5 || character.Cycle.LutealMinDays != 10 || character.Cycle.LutealMaxDays != 16 || character.CycleViability != .4 || character.ConditionalPeak != .7 || len(character.Cycle.ReferenceTime) != 0 || character.OriginEntityID != "mira" {
		t.Fatalf("editable model defaults or explicit unknown date changed: %#v", character)
	}
	bodySettingsRequest46(t, &Server{}, "one", http.MethodPut, `{"cycle_tracking_enabled":false,"automatic_pregnancy_enabled":true,"characters":[{"entity_id":"mira","character_name":"Mira","origin_entity_id":"spoof","species":"moonfolk","world_rule":"no cycles","cycle_enabled":false,"can_conceive":false,"cycle":{"cycle_days":0,"variation_days":0},"cycle_viability":0,"conditional_peak":0}]}`)
	second, _ := (&Server{}).loadBodyTrackingConfig("one")
	if second.CycleTrackingEnabled || !second.AutomaticPregnancyEnabled || second.SimulationSeed != first.SimulationSeed || second.Characters[0].OriginEntityID != "mira" || second.Characters[0].CycleEnabled || second.Characters[0].CanConceive || second.Characters[0].Cycle.CycleDays != 0 || second.Characters[0].CycleViability != 0 {
		t.Fatalf("restart/save reset seed, independent controls or explicit invalid model values: %#v", second)
	}
	other, exists, err := s.storedBodyTrackingConfig("two")
	if err != nil || exists || other.SimulationSeed != "" || other.CycleTrackingEnabled || other.AutomaticPregnancyEnabled {
		t.Fatalf("session settings leaked: %#v %t %v", other, exists, err)
	}
}

func Test46BodyTrackingSettingsRosterClockAndUnknownAreReadOnly(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	f := newIdentityAliasLinkRecordingStore()
	for _, name := range []string{"Mira", "Older configured character", "Invalid model"} {
		f.returnCharStates = append(f.returnCharStates, store.CharacterState{ChatSessionID: "story", CharacterName: name, AppearanceJSON: `{"gender":"female"}`})
	}
	f.identities = []*store.EntityIdentity{
		{ChatSessionID: "story", StableEntityID: "mira", CanonicalLabel: "Mira", EntityKind: "character"},
		{ChatSessionID: "story", StableEntityID: "tower", CanonicalLabel: "Tower", EntityKind: "place"},
		{ChatSessionID: "other", StableEntityID: "other-mira", CanonicalLabel: "Mira", EntityKind: "character"},
	}
	f.returnStatusCurrent = []store.StatusCurrentValue{{ID: 1, ChatSessionID: "story", StatusKey: storyClockStatusKey, OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, SourceTurn: 8, ValueJSON: `{"version":"story_clock.v1","absolute":{"date":"2024-01-15"},"observation_kind":"absolute","precision":"exact","scene_scope":"current","transition":"set","evidence_excerpt":"The calendar says January fifteenth."}`}}
	s := &Server{Store: f}
	view := bodySettingsRequest46(t, s, "story", http.MethodPut, `{"cycle_tracking_enabled":true,"characters":[{"entity_id":"mira","character_name":"Mira","cycle":{"reference_time":{"date":"2024-01-01"}}},{"entity_id":"old","character_name":"Older configured character"},{"entity_id":"invalid","character_name":"Invalid model","cycle":{"cycle_days":-1}}]}`)
	roster := view["roster"].([]any)
	if len(roster) != 3 || strings.Contains(mustCompactJSON(roster), "tower") || strings.Contains(mustCompactJSON(roster), "other-mira") {
		t.Fatalf("wrong roster scope or configured absent character lost: %#v", roster)
	}
	estimates := view["cycle_estimates"].([]any)
	var first map[string]any
	for _, raw := range estimates {
		if mapFromAny(raw)["entity_id"] == "mira" {
			first = mapFromAny(mapFromAny(raw)["estimate"])
		}
	}
	if first["status"] != "estimated" || first["knowledge"] != "not_inferred" || first["kind"] != "calculated_estimate" {
		t.Fatalf("confirmed clock not used in labeled estimate: %#v", first)
	}
	for _, raw := range estimates {
		estimate := mapFromAny(mapFromAny(raw)["estimate"])
		if mapFromAny(raw)["entity_id"] == "invalid" && estimate["status"] != "unknown" {
			t.Fatalf("invalid model invented date: %#v", raw)
		}
	}
	if len(f.savedStatusCurrent) != 0 || len(f.savedStatusEvents) != 0 || len(f.savedCharacterStates) != 0 {
		t.Fatal("configuration or preview wrote story facts")
	}
}

func Test46BodyTrackingSettingsCopyAndExplicitRestorePreserveModelOrigin(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	s := &Server{}
	c := defaultBodyCharacterConfig()
	c.EntityID, c.CharacterName = "source-entity", "Mira"
	first, err := s.saveBodyTrackingConfig("source", bodyTrackingConfig{CycleTrackingEnabled: true, Characters: []bodyCharacterConfig{c}})
	if err != nil {
		t.Fatal(err)
	}
	copied, err := s.copyBodyTrackingConfig("source", "target", map[string]string{"source-entity": "target-entity"})
	if err != nil || !copied {
		t.Fatalf("copy: %v %t", err, copied)
	}
	target, _ := s.loadBodyTrackingConfig("target")
	if target.SimulationSeed != first.SimulationSeed || target.Characters[0].EntityID != "target-entity" || target.Characters[0].OriginEntityID != "source-entity" {
		t.Fatalf("copy changed deterministic identity: %#v", target)
	}
	unchanged, _ := s.loadBodyTrackingConfig("source")
	if !reflect.DeepEqual(first, unchanged) {
		t.Fatal("copy mutated source settings")
	}
	snapshot := mustCompactJSON(map[string]any{"restore_snapshot": map[string]any{"contract_version": "body_tracking_settings.v1", "config": first}})
	bodySettingsRequest46(t, &Server{}, "restored", http.MethodPut, snapshot)
	restored, _ := s.loadBodyTrackingConfig("restored")
	if !reflect.DeepEqual(first, restored) {
		t.Fatalf("explicit restore changed origin/seed: %#v", restored)
	}
	if copied, err := s.copyBodyTrackingConfig("missing", "nothing", nil); err != nil || copied {
		t.Fatalf("missing settings created opt-in config: %t %v", copied, err)
	}
}
