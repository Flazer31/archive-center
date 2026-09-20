package httpapi

import (
	"strings"
	"testing"
)

func Test46BodyCycleReadingRespectsCurrentFactsAndExplicitReference(t *testing.T) {
	body, clock, _ := bodyDeliveryFixture46()
	character := body.Config.Characters[0]
	base := parseJSONMap(body.Values[0].ValueJSON)
	for _, tc := range []struct {
		name, status, ending, period, author, want string
		model                                      bool
	}{
		{name: "recovery alone", want: "estimated"},
		{name: "confirmed", status: "confirmed", ending: "2024-01-10", want: "ordinary_cycle_model_not_applicable_during_observed_pregnancy"},
		{name: "confirmed date unknown", status: "confirmed", want: "ordinary_cycle_model_not_applicable_during_observed_pregnancy"},
		{name: "future confirmation", status: "confirmed", ending: "2024-02-10", want: "estimated"},
		{name: "ended old reference", status: "ended", ending: "2024-01-10", want: "cycle_reference_after_pregnancy_unresolved"},
		{name: "new actual period", status: "ended", ending: "2024-01-10", period: "2024-01-12", want: "estimated"},
		{name: "author correction", status: "ended", ending: "2024-01-10", author: "2024-01-12", want: "estimated"},
		{name: "unknown ending new period not inferred", status: "ended", period: "2024-01-12", want: "cycle_reference_after_pregnancy_unresolved"},
		{name: "unknown ending author date not physical ordering", status: "ended", author: "2024-01-12", want: "cycle_reference_after_pregnancy_unresolved"},
		{name: "preimplant model", model: true, want: "ordinary_cycle_model_not_applicable_during_modeled_pregnancy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, projection := character, storyClockJSONMap(base)
			if tc.status != "" {
				projection["pregnancy"] = map[string]any{"status": tc.status, "occurred_at": map[string]any{"date": tc.ending}}
			}
			if tc.period != "" {
				projection["cycle_reference"] = map[string]any{"date": tc.period}
			}
			if tc.author != "" {
				c.Cycle.ReferenceTime = map[string]any{"date": tc.author}
			}
			if tc.model {
				projection["modeled_pregnancy"] = map[string]any{"ovulation_time": map[string]any{"date": "2024-01-12"}, "implantation_time": map[string]any{"date": "2024-01-20"}}
			}
			before := mustCompactJSON(projection)
			reading := bodyTrackingCycleReading(c, projection, clock)
			if reading["status"] != tc.want && reading["reason"] != tc.want {
				t.Fatalf("wanted %s: %#v", tc.want, reading)
			}
			if tc.want != "estimated" {
				for _, key := range []string{"cycle_day", "menstruation_possible", "fertile_window_possible", "ovulation_estimate", "next_period_estimate"} {
					if _, exists := reading[key]; exists {
						t.Fatalf("inapplicable forecast retained %s", key)
					}
				}
			}
			if before != mustCompactJSON(projection) {
				t.Fatal("forecast altered authoritative state")
			}
		})
	}
}

func Test46BodyCycleReadingUsesEligibleProjectionAndOperatorView(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	projection := parseJSONMap(body.Values[0].ValueJSON)
	projection["pregnancy"] = map[string]any{"kind": "pregnancy_confirmed", "status": "confirmed", "visibility": "restricted", "occurred_at": map[string]any{"date": "2024-01-10"}}
	body.Values[0].ValueJSON = mustCompactJSON(projection)
	for _, holder := range []string{"rowan-id", "mira-id"} {
		input := bodyAssemblyInput46(body, clock, character)
		input.Perspective.Public = map[string]any{"current_pov_entity_id": holder, "identity_state": "resolved"}
		out := buildPrepareTurnInjectionAssemblyWithBudget(input)
		text := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
		if holder == "rowan-id" {
			if strings.Contains(text, "pregnancy") || !strings.Contains(text, "next_period_estimate") {
				t.Fatalf("hidden fact altered independent forecast: %s", text)
			}
		} else if !strings.Contains(text, "ordinary_cycle_model_not_applicable_during_observed_pregnancy") || strings.Contains(text, "next_period_estimate") {
			t.Fatalf("eligible pregnancy did not limit ordinary forecast: %s", text)
		}
	}
	// The settings operator sees the same limitation with full state authority.
	reading := bodyTrackingCycleReading(body.Config.Characters[0], projection, clock)
	if reading["reason"] != "ordinary_cycle_model_not_applicable_during_observed_pregnancy" {
		t.Fatal(reading)
	}
}
