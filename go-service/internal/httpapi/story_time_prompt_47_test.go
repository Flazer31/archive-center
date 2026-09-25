package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test47StoryTimePromptKnownUnknownAndPlanned(t *testing.T) {
	clock := map[string]any{"absolute": map[string]any{"date": "2002-06-04", "time": "13:36"}, "source_turn": 103}
	observed := map[string]any{"date": "2002-06-04", "time": "11:12"}
	for _, tc := range []struct {
		name         string
		source       map[string]any
		want, absent []string
	}{
		{"exact", map[string]any{"occurrence_time": observed}, []string{"event=2002-06-04 11:12", "2h 24m before reference"}, []string{"elapsed_days", "0.100000"}},
		{"mention_only", map[string]any{"observed_at": observed}, []string{"event=unknown", "recorded scene=2002-06-04 11:12", "2h 24m before reference"}, []string{"event=2002"}},
		{"planned", map[string]any{"observed_at": observed, "relative_expression": "내일", "relative": map[string]any{"anchor": "source_observation", "offset": 1, "unit": "day", "target_kind": "planned_event"}}, []string{"planned_event", "2002-06-05", "after reference", "내일", "(at source)"}, []string{"completed"}},
		{"date_only", map[string]any{"occurrence_time": map[string]any{"date": "2002-06-02"}}, []string{"date only", "2 calendar days before reference"}, []string{"48h", "2d 13h"}},
		{"midnight", map[string]any{"occurrence_time": map[string]any{"date": "2002-06-03", "time": "23:59"}}, []string{"13h 37m before reference", "2002-06-03"}, []string{"yesterday"}},
		{"range", map[string]any{"occurrence_time": map[string]any{"range": map[string]any{"start": map[string]any{"date": "2002-05-27"}, "end": map[string]any{"date": "2002-05-29"}}}}, []string{"2002-05-27", "2002-05-29", "before reference"}, []string{"event=2002-05-28"}},
		{"unknown_relative", map[string]any{"relative_expression": "yesterday"}, []string{"event=unknown", "distance unknown", "(at source)"}, []string{"event=2002"}},
		{"carried_clock", map[string]any{"observed_at": map[string]any{"story_clock": observed, "resolution_source": "last_confirmed_clock"}}, []string{"recorded scene (carried clock)", "event=unknown"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := mustCompactJSON(tc.source)
			reading := buildStoryTimeReading(tc.source, clock)
			text := storyTimePromptReading(reading)
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q in %s", want, text)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(text, absent) {
					t.Errorf("invented/leaked %q in %s", absent, text)
				}
			}
			if mustCompactJSON(tc.source) != before {
				t.Fatal("prompt formatting wrote source metadata")
			}
			if tc.name == "exact" && utf8.RuneCountInString(text) >= utf8.RuneCountInString(mustCompactJSON(reading)) {
				t.Fatal("compact reading did not reduce supplied time text")
			}
		})
	}
	note := storyTimePromptNote(clock)
	if !strings.Contains(note, "turn 103") || !strings.Contains(note, "last accepted narration") || !strings.Contains(note, "Explicit new user time movement") {
		t.Fatal(note)
	}
}

func Test47TimeRenderingPreservesScoringAndSourceIdentity(t *testing.T) {
	memory, clock := temporal46Fixture()
	out := buildPrepareTurnInjectionAssemblyWithBudget(temporal46AssemblyInput(memory, clock))
	facts, _ := multiAgentCandidatePool(&out)
	checked := 0
	for _, fact := range facts {
		if fact.Reading == nil {
			continue
		}
		values := []string{}
		for _, part := range fact.Reading.Parts {
			values = append(values, part.Value)
		}
		if strings.Join(values, "\n") != fact.Minimum.Meaning {
			t.Fatal("new display prose changed scoring material")
		}
		if strings.Contains(fact.Minimum.Text, "elapsed_days") || !strings.Contains(fact.Minimum.Text, "⏳") {
			t.Fatal(fact.Minimum.Text)
		}
		if fact.SourceTurn != memory.TurnIndex || fact.SourceRef != "memories:481" {
			t.Fatal("time formatting changed identity")
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no production candidate checked")
	}
}

func Test47PrivateRecollectionTimeUsesExactSourceScene(t *testing.T) {
	memory, clock := temporal46Fixture()
	input := temporal46AssemblyInput(memory, clock)
	input.UserInput = "Rowan remembers Mira and the charts."
	input.Perspective.Selection.Query = input.UserInput
	input.CharacterPrivateMemories = []store.ProtagonistEntityMemory{{ID: 912, OwnerEntityKey: "Rowan", OwnerEntityName: "Rowan", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: memory.ChatSessionID, SourceTurn: memory.TurnIndex, MemoryText: "Rowan privately doubts the charts.", Importance10: 8}}
	for _, otherSession := range []bool{false, true} {
		if otherSession {
			input.CharacterPrivateMemories[0].SourceChatSessionID = "different-world"
		}
		out := buildPrepareTurnInjectionAssemblyWithBudget(input)
		facts, _ := multiAgentCandidatePool(&out)
		found := false
		for _, fact := range facts {
			if fact.SourceTable != "protagonist_entity_memories" {
				continue
			}
			found = true
			text := prepareTurnMemoryReadingText(fact)
			if !otherSession && (!strings.Contains(text, "recorded scene=2026-01-01") || !strings.Contains(text, "event=unknown")) {
				t.Fatal(text)
			}
			if otherSession && strings.Contains(text, "⏳") {
				t.Fatal("borrowed another session's date")
			}
			if fact.PerspectiveOwner != "Rowan" || fact.Visibility != "owner_private" {
				t.Fatal("changed private scope")
			}
		}
		if !found {
			t.Fatal("date uncertainty removed recollection")
		}
	}
}

func Test47GroupedPreprocessingKeepsTimeThroughBothRounds(t *testing.T) {
	memory, clock := temporal46Fixture()
	assembly := buildPrepareTurnInjectionAssemblyWithBudget(temporal46AssemblyInput(memory, clock))
	facts, summaries := multiAgentCandidatePool(&assembly)
	// Reuse the same public source to exercise each existing role transport.
	for _, role := range multiAgentRoles[1:] {
		c := facts[0]
		c.Lane = role
		c.CanonicalFactID += "/" + role
		facts = append(facts, c)
	}
	var mu sync.Mutex
	calls, searches := 0, 0
	seen := map[string]int{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wire struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil || len(wire.Messages) != 2 {
			t.Error("unexpected provider request")
			http.Error(w, "bad", 400)
			return
		}
		var packet map[string]any
		if json.Unmarshal([]byte(wire.Messages[1].Content), &packet) != nil {
			t.Error("invalid model packet")
			return
		}
		mu.Lock()
		defer mu.Unlock()
		calls++
		members := sliceFromAny(packet["roles"])
		if len(members) != len(multiAgentRoles) {
			t.Errorf("group fragmented into %d roles", len(members))
		}
		answers := map[string]any{}
		for _, raw := range members {
			member := mapFromAny(raw)
			role := stringFromMap(member, "role")
			expanded := map[string]any{}
			for k, v := range mapFromAny(packet["shared_input"]) {
				expanded[k] = v
			}
			for k, v := range mapFromAny(member["input"]) {
				expanded[k] = v
			}
			expanded = mapFromAny(expand45Shared(expanded, mapFromAny(packet["shared_records"])))
			text := mustCompactJSON(expanded)
			for _, want := range []string{"⏳", "before reference", "planned_event", "2026-01-01", "2026-03-15"} {
				if !strings.Contains(text, want) {
					t.Errorf("%s round %d lost %s", role, calls, want)
				}
			}
			if expanded["story_time_note"] != storyTimePromptNote(clock) {
				t.Error("lost common reference note")
			}
			candidates := sliceFromAny(expanded["candidates"])
			if len(candidates) == 0 {
				t.Errorf("%s has no source", role)
				continue
			}
			ref := stringFromMap(mapFromAny(candidates[0]), "ref")
			answer := map[string]any{"selected_ids": []string{ref}, "reasons": map[string]string{ref: "A historical promise with a recorded due date."}}
			if calls == 1 {
				answer["search_requests"] = []string{"Find the recorded charts promise date."}
			}
			answers[role] = answer
			seen[role]++
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": mustCompactJSON(map[string]any{"roles": answers})}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Enabled = true
		c.UsePublisher = false
		c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
		cfg.Roles[role] = c
	}
	result := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, summaries, 18000, 5, nil, func(q string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		mu.Lock()
		searches++
		mu.Unlock()
		if q != "Find the recorded charts promise date." {
			t.Error(q)
		}
		return facts, summaries, nil
	}, map[string]any{"story_time_note": storyTimePromptNote(clock)})
	// All roles asked the same search, so the existing query owner shares it.
	if calls != 2 || searches != 1 {
		t.Fatalf("calls=%d searches=%d", calls, searches)
	}
	for _, role := range multiAgentRoles {
		if seen[role] != 2 {
			t.Fatalf("%s rounds=%d", role, seen[role])
		}
	}
	for _, role := range result.Roles {
		if len(role.Selection.SelectedIDs) == 0 {
			t.Fatalf("selection lost for %s", role.Role)
		}
	}
	before := memory.SummaryJSON
	assembly.Preprocessing = result
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 18000, 5, "auto", nil, testPrepareTurnMemorySelectionContext(nil))
	final := stringFromMap(plan, "final_text")
	if !strings.Contains(final, "⏳") || !strings.Contains(final, "planned_event") {
		t.Fatal(final)
	}
	assert45Budget(t, plan, 18000)
	if !reflect.DeepEqual(before, memory.SummaryJSON) {
		t.Fatal("read-only test changed history")
	}
}
