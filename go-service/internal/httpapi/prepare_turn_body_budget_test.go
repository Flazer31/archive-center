package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test46BodyBudgetAdds3000WithoutBorrowingMainMemory(t *testing.T) {
	for _, mode := range []string{"auto", "custom"} {
		for _, flags := range []struct{ cycle, pregnancy bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
			t.Run(fmt.Sprintf("%s/cycle-%v/pregnancy-%v", mode, flags.cycle, flags.pregnancy), func(t *testing.T) {
				body, clock, character := bodyDeliveryFixture46()
				addBodyPregnancyFixture46(body)
				body.Config.CycleTrackingEnabled, body.Config.AutomaticPregnancyEnabled = flags.cycle, flags.pregnancy
				character.StatusJSON = `{"job":"Mira maintains the recovery calendar for the archive."}`
				input := bodyAssemblyInput46(body, clock, character)
				input.MaxChars, input.BudgetMode = 800, mode
				input.Budgets = map[string]int{"character_objective": 800}
				baselineInput := input
				baselinePerspective := *input.Perspective
				baselinePerspective.BodyTracking = nil
				baselineInput.Perspective = &baselinePerspective
				baseline := buildPrepareTurnInjectionAssemblyWithBudget(baselineInput)
				baselineText := stringFromMap(baseline.MemoryDeliveryPlan, "final_text")
				if baselineText == "" {
					t.Fatal("missing ordinary-memory control")
				}
				out := buildPrepareTurnInjectionAssemblyWithBudget(input)
				extra := mapFromAny(out.MemoryDeliveryPlan["body_tracking_budget"])
				want := 0
				if flags.cycle || flags.pregnancy {
					want = 3000
				}
				if intFromAny(extra["cap_chars"], 0) != want {
					t.Fatalf("extra allocation=%v", extra)
				}
				bodyText := stringFromMap(extra, "final_text")
				final := stringFromMap(out.MemoryDeliveryPlan, "final_text")
				mainText := strings.TrimSpace(strings.TrimSuffix(final, bodyText))
				if mainText != baselineText {
					t.Fatalf("body changed ordinary memory allocation\nbefore=%s\nafter=%s", baselineText, mainText)
				}
				if len([]rune(bodyText)) > want || (want > 0 && bodyText == "") {
					t.Fatalf("body allocation not used/bounded: %d", len([]rune(bodyText)))
				}
				assert45Budget(t, out.MemoryDeliveryPlan, input.MaxChars+want)
			})
		}
	}
}

func Test46BodyBudgetManyCharactersRemainWithin3000(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	original := body.Config.Characters[0]
	originalValue := body.Values[0]
	names := []string{}
	for i := 1; i <= 25; i++ {
		c := original
		c.EntityID, c.CharacterName = fmt.Sprintf("woman-%d", i), fmt.Sprintf("Woman%d", i)
		body.Config.Characters = append(body.Config.Characters, c)
		v := originalValue
		v.ID, v.OwnerID = int64(1000+i), c.EntityID
		v.ValueJSON = strings.ReplaceAll(strings.ReplaceAll(v.ValueJSON, original.EntityID, c.EntityID), original.CharacterName, c.CharacterName)
		body.Values = append(body.Values, v)
		names = append(names, c.CharacterName)
	}
	input := bodyAssemblyInput46(body, clock, character)
	input.CharacterStates = append(input.CharacterStates, store.CharacterState{CharacterName: "Other", StatusJSON: `{"job":"unrelated"}`})
	input.UserInput += " " + strings.Join(names, " ")
	input.Perspective.Selection.Query = input.UserInput
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	extra := mapFromAny(out.MemoryDeliveryPlan["body_tracking_budget"])
	if intFromAny(extra["candidate_count"], 0) <= intFromAny(extra["selected_count"], 0) {
		t.Fatal("fixture did not exceed the dedicated budget")
	}
	if intFromAny(extra["used_chars"], 0) > 3000 || intFromAny(extra["used_chars"], 0) == 0 {
		t.Fatalf("body cap not enforced: %#v", extra)
	}
	if prepareTurnPayloadBudgetReasonCounts(out.MemoryDeliveryPlan["exclusion_reasons"])["body_tracking_char_budget_reached"] == 0 {
		t.Fatal("body overflow was not accounted")
	}
	facts, summaries := multiAgentCandidatePool(&out)
	ids := []string{}
	for _, fact := range facts {
		ids = append(ids, fact.CanonicalFactID)
	}
	out.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "subjective_relationship", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: ids}}}}
	plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, input.MaxChars, 100, "auto", nil, input.Perspective.Selection)
	extra = mapFromAny(plan["body_tracking_budget"])
	if intFromAny(extra["used_chars"], 0) > 3000 || intFromAny(extra["selected_count"], 0) >= intFromAny(extra["candidate_count"], 0) {
		t.Fatal("AI selecting every character bypassed the body limit")
	}
}
