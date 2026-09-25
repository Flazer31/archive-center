package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func Test47EditorReadingDefaultAndSavedChoices(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	cfg, err := readMultiAgentSettings()
	if err != nil || cfg.CandidateChars != 64000 {
		t.Fatalf("new reading default: %d, %v", cfg.CandidateChars, err)
	}
	path, err := multiAgentSettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"candidate_chars":32000,"shared_prompt":"My instructions"}`), 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := readMultiAgentSettings()
	if err != nil || saved.CandidateChars != 32000 || saved.SharedPrompt != "My instructions" {
		t.Fatal("saved reading allocation or custom prompt was reset")
	}
	cfg.Enabled = true
	facts := []prepareTurnPriorityMemoryCandidate{}
	for _, id := range []string{"earlier-purpose", "later-change", "qualifying-condition"} {
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: id, Lane: "event_recent", CompleteText: id + strings.Repeat(" preserved source detail", 750)})
	}
	wide := multiAgentInput("event_recent", facts, nil, dto.PrepareTurnRequest{}, cfg, 16000, 8, nil)
	cfg.CandidateChars = saved.CandidateChars
	narrow := multiAgentInput("event_recent", facts, nil, dto.PrepareTurnRequest{}, cfg, 16000, 8, nil)
	if len(outputFidelityLineageSlice(wide["candidates"])) != len(facts) || len(outputFidelityLineageSlice(narrow["candidates"])) >= len(facts) {
		t.Fatal("fixture failed to expose the previous reading-window loss")
	}
	budget := mapFromAny(wide["budgets"])
	if budget["delivery_policy"] != "editor_ordered_evidence" || budget["global_delivery_chars"] != nil || intFromAny(budget["go_baseline_delivery_chars"], 0) != 16000 {
		t.Fatal("Go baseline is still presented as the editor's final ceiling")
	}
	for i, item := range outputFidelityLineageSlice(wide["candidates"]) {
		if stringFromMap(mapFromAny(item), "text") != facts[i].CompleteText {
			t.Fatal("wider reading truncated or rewrote its evidence")
		}
	}
}

func Test47EditorFullReadingAndJevOnlyProjectionRemainDistinct(t *testing.T) {
	n := 2
	req := dto.PrepareTurnRequest{Settings: dto.PrepareTurnSettings{RecentConversationReferenceCount: &n}, Messages: []map[string]any{
		{"role": "user", "content": "Keep the terms."}, {"role": "assistant", "content": "The promise included an exception.\n\nThe key was not transferred."},
		{"role": "user", "content": "Continue."}, {"role": "assistant", "content": "They discussed the exception."},
	}}
	reading := []map[string]any{{"Source": "recent_conversation_stored_summary", "Text": "A shorter stored account."}}
	for _, enabled := range []bool{true, false} {
		cfg := defaultMultiAgentSettings()
		cfg.Enabled, cfg.Jev.Enabled = enabled, !enabled
		input := multiAgentInput("event_recent", nil, nil, req, cfg, 16000, 8, nil, map[string]any{"recent_conversation_reading": reading})
		before, _ := json.Marshal(input)
		for _, chosen := range [][]string{{}, {"C1.1"}, {"C999.1"}} {
			input["previous_result"] = multiAgentRecommendation{RecentContextRefs: &chosen}
			var packet map[string]any
			if err := json.Unmarshal([]byte(multiAgentModelInput(input, 2)), &packet); err != nil {
				t.Fatal(err)
			}
			if enabled {
				want := prepareTurnRecentConversationQueries(req.Messages, n)
				got := outputFidelityLineageSlice(packet["recent_conversation"])
				if len(got) != len(want) {
					t.Fatal("configured original scope changed")
				}
				for i := range want {
					if modelRecentTextForTest(mapFromAny(got[i])) != want[i].Text {
						t.Fatal("first-round focus hid a counterexample from round two")
					}
				}
			} else if !reflect.DeepEqual(input["recent_conversation_reading"], reading) || packet["recent_context_policy"] != nil {
				t.Fatal("Jev-only compact reading inherited the LLM full-context policy")
			}
		}
		delete(input, "previous_result")
		after, _ := json.Marshal(input)
		if !bytes.Equal(before, after) {
			t.Fatal("wire preparation mutated the canonical reading")
		}
	}
}

func Test47EditorSupplementRetainsUnselectedCounterevidenceAndOwnRefs(t *testing.T) {
	facts := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "promise", Lane: "event_recent", CompleteText: "Mira promised to return the key.", Visibility: "public"},
		{CanonicalFactID: "exception", Lane: "event_recent", CompleteText: "The owner later extended the deadline.", Visibility: "public"},
		{CanonicalFactID: "private", Lane: "event_recent", CompleteText: "Mira privately worries.", Visibility: "owner_private", PerspectiveOwner: "Mira"},
		{CanonicalFactID: "world", Lane: "world_state", CompleteText: "The gate needs its original key.", Visibility: "public"},
	}
	refs := multiAgentReferences(facts, nil, nil, nil)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		var input map[string]any
		_ = json.Unmarshal([]byte(stringFromMap(mapFromAny(outputFidelityLineageSlice(body["messages"])[1]), "content")), &input)
		role := stringFromMap(input, "role")
		second := intFromAny(input["analysis_round"], 0) == 2
		directory := stringsFromAny(mapFromAny(input["selectable_refs"])["selected_ids"])
		answer := multiAgentRecommendation{}
		if role == "event_recent" {
			if !reflect.DeepEqual(directory, []string{refs["promise"], refs["exception"], refs["private"]}) {
				t.Errorf("own selection directory changed: %+v", directory)
			}
			if !reflect.DeepEqual(stringsFromAny(input["public_handoff_refs"]), []string{refs["promise"], refs["exception"]}) {
				t.Error("handoff directory borrowed another role or exposed private evidence")
			}
			answer.SelectedIDs = []string{refs["promise"]}
			if !second {
				answer.RelatedRequests = []multiAgentRelatedRequest{{Role: "world_state", Refs: []string{refs["promise"], refs["world"], refs["private"], "F404"}, Reason: "Which recorded gate condition matters?"}}
			} else {
				answer.SelectedIDs = append(answer.SelectedIDs, refs["exception"])
			}
		} else {
			if !reflect.DeepEqual(directory, []string{refs["world"]}) {
				t.Error("world editor received another role's selectable facts")
			}
			answer.SelectedIDs = []string{refs["world"]}
			if second {
				items := outputFidelityLineageSlice(input["related_evidence"])
				if len(items) != 1 || stringFromMap(mapFromAny(items[0]), "ref") != refs["promise"] {
					t.Error("handoff changed original scope or lost its valid public source")
				}
			}
		}
		b, _ := json.Marshal(answer)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Enabled = role == "event_recent" || role == "world_state"
		c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, role, "fixture-key"
		cfg.Roles[role] = c
	}
	searches := 0
	got := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 16000, 8, nil, func(string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		searches++
		return nil, nil, map[string]any{"status": "ok"}
	})
	if searches != 1 || got.AnalysisCalls != 4 || !reflect.DeepEqual(got.role("event_recent").Selection.SelectedIDs, []string{"promise", "exception"}) {
		t.Fatal("supplement could not revise its first choice or changed call counts")
	}
	issues := got.role("event_recent").Unresolved
	for _, want := range []string{"related_reference_outside_assignment: " + refs["world"], "related_reference_not_public: private", "related_reference_unresolved: F404"} {
		if !stringSliceContains(issues, want) {
			t.Errorf("diagnostic %q missing from %+v", want, issues)
		}
	}
}
