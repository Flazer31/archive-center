package httpapi

import (
	"strings"
	"testing"
)

// A live Critic emitted Korean slot names. They are open story vocabulary,
// and must remain independent claims instead of all collapsing to an empty key.
func Test47NarrativeLocalizedSlotsSurviveExtraction(t *testing.T) {
	for _, slot := range []string{"소유자", "시설 상태", "所有者", "état actuel", "e\u0301tat"} {
		t.Run(slot, func(t *testing.T) {
			claims := normalizeNarrativeStateClaims(map[string]any{"state_claims": []any{map[string]any{
				"subject": "야항패", "state_slot": slot, "value": "리오", "transition": "change",
				"evidence_excerpt": "야항패의 현재 소유자는 리오다.",
			}}})
			if len(claims) != 1 || claims[0].StateSlot != strings.ReplaceAll(slot, " ", "_") || claims[0].Value != "리오" {
				t.Fatalf("localized grounded claim lost: %#v", claims)
			}
		})
	}
	if normalizeNarrativeStateSlot(" Goal-Status ") != "goal_status" {
		t.Fatal("existing ASCII state identity changed")
	}
	if normalizeNarrativeStateSlot("소유자") == normalizeNarrativeStateSlot("시설 상태") {
		t.Fatal("independent slots collapsed")
	}
	if normalizeNarrativeStateSlot(" -- ") != "" {
		t.Fatal("punctuation acquired a state identity")
	}
}
