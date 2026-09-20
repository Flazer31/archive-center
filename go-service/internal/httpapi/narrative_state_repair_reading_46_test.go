package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test46NarrativeRepairSuppressesLegacyArtifactByObservationOrder(t *testing.T) {
	const artifact = `{"subject":"Atlas Restoration","state_slot":"goal_status"}`
	for _, causeTurn := range []int{0, 2} {
		current := narrativeTestGroundedCurrentValue("entity", "Atlas Restoration", "goal_status", "completed", "", "objective", "", "complete", 0.9, causeTurn)
		evidence := parseJSONMap(current.EvidenceJSON)
		evidence["source_contract"], evidence["repair_recorded_turn"] = store.StateRepairContract, 30
		current.EvidenceJSON = mustCompactJSON(evidence)
		before := mustCompactJSON(current)
		for _, tc := range []struct {
			artifactTurn int
			want         bool
		}{{10, true}, {30, false}, {31, false}} {
			if got := narrativeCurrentStateSupersedesOpenArtifact([]store.StatusCurrentValue{current}, tc.artifactTurn, artifact); got != tc.want {
				t.Errorf("cause=%d repair=30 artifact=%d: suppression=%v want %v", causeTurn, tc.artifactTurn, got, tc.want)
			}
		}
		if narrativeCurrentStateSupersedesOpenArtifact([]store.StatusCurrentValue{current}, 10, `{"subject":"Atlas Restoration Survey","state_slot":"goal_status"}`) {
			t.Fatal("repair suppressed a different legacy goal")
		}
		if mustCompactJSON(current) != before {
			t.Fatal("read path changed actual source turn or repair evidence")
		}
	}
	ordinary := narrativeTestGroundedCurrentValue("entity", "Atlas Restoration", "goal_status", "completed", "", "objective", "", "complete", 0.9, 2)
	if narrativeCurrentStateSupersedesOpenArtifact([]store.StatusCurrentValue{ordinary}, 10, artifact) ||
		!narrativeCurrentStateSupersedesOpenArtifact([]store.StatusCurrentValue{ordinary}, 1, artifact) {
		t.Fatal("ordinary source-turn ordering changed")
	}
}

func Test46NarrativeCurrentViewsOrderRepairsByObservation(t *testing.T) {
	repaired := narrativeTestGroundedCurrentValue("entity", "Atlas Restoration", "goal_status", "completed", "", "objective", "", "complete", 0.9, 2)
	evidence := parseJSONMap(repaired.EvidenceJSON)
	evidence["source_contract"], evidence["repair_recorded_turn"] = store.StateRepairContract, 30
	repaired.EvidenceJSON = mustCompactJSON(evidence)
	ordinary := narrativeTestGroundedCurrentValue("entity", "Gate Survey", "goal_status", "completed", "", "objective", "", "complete", 0.9, 20)
	latest := narrativeTestGroundedCurrentValue("entity", "Bridge Repair", "goal_status", "completed", "", "objective", "", "complete", 0.9, 40)
	values := []store.StatusCurrentValue{ordinary, repaired, latest}
	before := mustCompactJSON(values)
	views := narrativeCurrentStateViews(values)
	if len(views) != 3 || views[0].Subject != "Bridge Repair" || views[1].Subject != "Atlas Restoration" || views[2].Subject != "Gate Survey" {
		t.Fatalf("current views do not use observation order: %#v", views)
	}
	if views[1].Value.SourceTurn != 2 || mustCompactJSON(values) != before {
		t.Fatal("sorting changed actual occurrence provenance or input order")
	}
}
