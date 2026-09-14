package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func saveSharingSettings(t *testing.T, s *Server, cfg multiAgentSettings) multiAgentSettings {
	t.Helper()
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleMultiAgentSettings(rec, httptest.NewRequest("PUT", "/config/memory-preprocessing", bytes.NewReader(b)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %s", rec.Body)
	}
	// A fresh server reads the persisted reference, not a request-local cache.
	loaded, err := (&Server{}).loadMultiAgentSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, cfg) {
		t.Fatal("saved connection references or local drafts changed")
	}
	return loaded
}

func Test44PeerSettingsPersistResolveAndDetach(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	s := &Server{Cfg: config.Default(), RuntimeConfig: RuntimeConfig{SupervisorProvider: "llmgateway", SupervisorModel: "gpt-5.6-luna", SupervisorAPIKey: "fixture-publisher-key"}}
	cfg := defaultMultiAgentSettings()
	owner := cfg.Roles["event_recent"]
	owner.Provider, owner.Model, owner.APIKey = "llmgateway", "gpt-5.6-luna", "fixture-source-key"
	owner.Temperature, owner.MaxTokens, owner.TimeoutMs = .35, 4096, 180000
	owner.ReasoningEffort, owner.ReasoningBudgetTokens = "low", int64Ptr(3000)
	owner.LLMGatewayServiceTier, owner.VertexFlexMode = "standard", "flex"
	cfg.Roles["event_recent"] = owner
	peer := cfg.Roles["unresolved_goal"]
	peer.Model, peer.APIKey, peer.Prompt, peer.UseRole = "old-local", "fixture-local-key", "Keep my goal assignment", "event_recent"
	peer.Enabled = false
	cfg.Roles["unresolved_goal"] = peer
	for _, publisher := range []bool{false, true} {
		owner.UsePublisher = publisher
		cfg.Roles["event_recent"] = owner
		cfg = saveSharingSettings(t, s, cfg)
		resolved, source := cfg.roleConnection("unresolved_goal")
		want := owner
		want.Prompt, want.Enabled = peer.Prompt, peer.Enabled
		if source != "event_recent" || !reflect.DeepEqual(resolved, want) {
			t.Fatalf("resolved=%+v source=%s", resolved, source)
		}
		_, ownerReq := s.multiAgentProxyRequest("event_recent", cfg, 1, map[string]any{})
		call, peerReq := s.multiAgentProxyRequest("unresolved_goal", cfg, 1, map[string]any{})
		ownerReq.Messages, peerReq.Messages = nil, nil
		if !reflect.DeepEqual(ownerReq, peerReq) {
			t.Fatal("peer did not inherit the actual provider request settings")
		}
		if !strings.Contains(call.Prompt, peer.Prompt) {
			t.Fatal("peer task replaced with source prompt")
		}
	}
	owner.MaxTokens, owner.Temperature = 3072, .6
	cfg.Roles["event_recent"] = owner
	cfg = saveSharingSettings(t, s, cfg)
	resolved, _ := cfg.roleConnection("unresolved_goal")
	if resolved.MaxTokens != owner.MaxTokens || resolved.Temperature != owner.Temperature {
		t.Fatal("source edit not followed")
	}
	peer.UseRole = ""
	cfg.Roles["unresolved_goal"] = peer
	cfg = saveSharingSettings(t, s, cfg)
	resolved, source := cfg.roleConnection("unresolved_goal")
	if source != "unresolved_goal" || !reflect.DeepEqual(resolved, peer) {
		t.Fatal("detaching did not restore own settings")
	}
}

func Test44PeerSettingsTwoProviderGroupsOverlapAndHUD(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	entered := make(chan map[string]any, len(multiAgentRoles))
	release := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wire map[string]any
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
			t.Error(err)
			return
		}
		entered <- wire
		<-release
		var packet map[string]any
		messages := sliceFromAny(wire["messages"])
		if len(messages) != 2 {
			t.Error("missing model packet")
			return
		}
		if err := json.Unmarshal([]byte(stringFromMap(mapFromAny(messages[1]), "content")), &packet); err != nil {
			t.Error(err)
			return
		}
		results := map[string]any{}
		for _, raw := range sliceFromAny(packet["roles"]) {
			assignment := mapFromAny(raw)
			role := stringFromMap(assignment, "role")
			if assignment["prompt"] != "Own assignment "+role {
				t.Errorf("role prompt lost: %s", role)
			}
			results[role] = map[string]any{"selected_ids": []string{"fact-" + role}}
		}
		b, _ := json.Marshal(map[string]any{"roles": results})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20}})
	}))
	defer provider.Close()
	s := &Server{Cfg: config.Default(), TurnWorkflows: newTurnWorkflowHUDLedger()}
	s.TurnWorkflows.begin("fixture-request", "fixture-session", 1)
	cfg := defaultMultiAgentSettings()
	roles := make([]multiAgentRoleResult, len(multiAgentRoles))
	inputs := make([]map[string]any, len(multiAgentRoles))
	for i, role := range multiAgentRoles {
		c := cfg.Roles[role]
		c.Provider, c.Model, c.Endpoint, c.APIKey = "custom", "old-local", provider.URL, "fixture-local-key"
		c.MaxTokens = 999
		c.Prompt = "Own assignment " + role
		switch role {
		case "event_recent":
			c.Provider, c.Model, c.APIKey, c.MaxTokens = "llmgateway", "gpt-5.6-luna", "fixture-gateway-key", 1000
			c.LLMGatewayServiceTier = "standard"
		case "character_objective":
			c.Provider, c.Model, c.APIKey, c.MaxTokens = "ollama", "deepseek-v4.1-flash:cloud", "fixture-ollama-key", 2000
		case "unresolved_goal":
			c.UseRole = "event_recent"
		default:
			c.UseRole = "character_objective"
			c.ReasoningEffort = "none"
		}
		cfg.Roles[role] = c
		roles[i] = multiAgentRoleResult{Role: role}
		inputs[i] = map[string]any{"role": role, "current_input": "shared input", "candidates": []any{map[string]any{"id": "fact-" + role, "text": "original evidence"}}}
	}
	cfg = saveSharingSettings(t, s, cfg)
	ctx := context.WithValue(context.Background(), multiAgentHUDRequestKey{}, "fixture-request")
	done := make(chan []multiAgentCall, 1)
	go func() { done <- s.callMultiAgentRound(ctx, cfg, 1, roles, inputs, "fixture-session") }()
	wires := []map[string]any{}
	for len(wires) < 2 {
		select {
		case wire := <-entered:
			wires = append(wires, wire)
		case <-time.After(10 * time.Second):
			close(release)
			<-done
			t.Fatal("two provider requests did not overlap before either response")
		}
	}
	view, _ := s.TurnWorkflows.snapshot("fixture-request")
	close(release)
	calls := <-done
	if len(entered) != 0 {
		t.Errorf("extra actual requests=%d", len(entered))
	}
	if len(view.PreprocessingRequests) != len(wires) {
		t.Fatalf("HUD requests=%+v", view.PreprocessingRequests)
	}
	for _, request := range view.PreprocessingRequests {
		if request.Status != "running" || len(request.SharedRoles) < 2 || request.Model == "" || request.Provider == "" {
			t.Errorf("running HUD=%+v", request)
		}
	}
	for _, wire := range wires {
		var packet map[string]any
		_ = json.Unmarshal([]byte(stringFromMap(mapFromAny(sliceFromAny(wire["messages"])[1]), "content")), &packet)
		members := sliceFromAny(packet["roles"])
		wantPerRole := int64(1000)
		if wire["model"] == "deepseek-v4.1-flash:cloud" {
			wantPerRole = 2000
			if _, present := wire["reasoning_effort"]; present {
				t.Error("blank owner effort became explicit off")
			}
		}
		budget := wire["max_completion_tokens"]
		if budget == nil {
			budget = wire["max_tokens"]
		}
		if budget != float64(wantPerRole*int64(len(members))) {
			t.Errorf("wrong inherited group budget: %v", budget)
		}
	}
	usage := 0
	for i, call := range calls {
		if call.Error != "" || !reflect.DeepEqual(call.Result.SelectedIDs, []string{"fact-" + roles[i].Role}) {
			t.Errorf("result lost: %+v", call)
		}
		if call.Usage != nil {
			usage++
		}
	}
	if usage != len(wires) {
		t.Error("billed usage counted per role rather than request")
	}
	view, _ = s.TurnWorkflows.snapshot("fixture-request")
	if len(view.PreprocessingRequests) != len(wires) {
		t.Fatal("completed requests duplicated")
	}
	for _, request := range view.PreprocessingRequests {
		if request.Status != "succeeded" {
			t.Errorf("final request=%+v", request)
		}
	}
}

func Test44SharingChainAndUnresolvedReference(t *testing.T) {
	cfg := defaultMultiAgentSettings()
	a, b, c := multiAgentRoles[0], multiAgentRoles[1], multiAgentRoles[2]
	root := cfg.Roles[a]
	root.Model = "source-model"
	cfg.Roles[a] = root
	peer := cfg.Roles[b]
	peer.UseRole = a
	cfg.Roles[b] = peer
	tail := cfg.Roles[c]
	tail.UseRole = b
	tail.Prompt = "own prompt"
	cfg.Roles[c] = tail
	got, source := cfg.roleConnection(c)
	if source != a || got.Model != root.Model || got.Prompt != tail.Prompt {
		t.Fatal("chain did not follow source")
	}
	for _, ref := range []string{c, "removed-role"} {
		root.UseRole = ref
		cfg.Roles[a] = root
		got, source = cfg.roleConnection(c)
		if source != c || !reflect.DeepEqual(got, tail) {
			t.Fatal("unresolved reference changed local configuration")
		}
	}
}

func Test44HUDRequestProjectionCountsAndDurations(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("r", "s", 1)
	for i, role := range multiAgentRoles {
		status := "succeeded"
		if i == 1 {
			status = "failed"
		}
		ledger.recordPreprocessingCall("r", role, turnWorkflowHUDPreprocessingCall{Round: 1, Status: status, SharedRequestID: "shared", SharedRoles: multiAgentRoles, DurationMS: 1234, Dispatched: true})
	}
	// Local configuration failure is retained as a role result, not a network request.
	ledger.recordPreprocessingCall("r", multiAgentRoles[0], turnWorkflowHUDPreprocessingCall{Round: 2, Status: "failed"})
	view, _ := ledger.snapshot("r")
	if len(view.PreprocessingRequests) != 1 || view.PreprocessingRequests[0].DurationMS != 1234 || view.PreprocessingRequests[0].Status != "partial" {
		t.Fatalf("projection=%+v", view.PreprocessingRequests)
	}
	if len(view.Preprocessing[0].Calls) != 2 {
		t.Fatal("local failure vanished from role details")
	}
	view.PreprocessingRequests[0].SharedRoles[0] = "mutated"
	fresh, _ := ledger.snapshot("r")
	if fresh.PreprocessingRequests[0].SharedRoles[0] == "mutated" {
		t.Fatal("HUD snapshot retained caller mutation")
	}
}

func Test44HUDRequestRowsPairRoundsWithoutCombiningSeparateCalls(t *testing.T) {
	for _, variant := range []string{"same", "member_order", "subset", "changed_model"} {
		t.Run(variant, func(t *testing.T) {
			ledger := newTurnWorkflowHUDLedger()
			ledger.begin("request", "session", 1)
			groups := [][]string{{"event_recent"}, {"character_objective"}, {"subjective_relationship", "world_state"}, {"unresolved_goal"}}
			for round := 1; round <= 2; round++ {
				for i, group := range groups {
					members := append([]string(nil), group...)
					provider, model := "ollama", "deepseek-v4.1-flash:cloud"
					if i == 0 || i == 3 {
						provider, model = "llmgateway", "gpt-5.6-luna"
					}
					if round == 2 && i == 2 {
						switch variant {
						case "member_order":
							members[0], members[1] = members[1], members[0]
						case "subset":
							members = members[:1]
						case "changed_model":
							model = "different-model"
						}
					}
					id := fmt.Sprintf("request-%d-%d", round, i)
					for _, role := range members {
						ledger.recordPreprocessingCall("request", role, turnWorkflowHUDPreprocessingCall{Round: round, SharedRequestID: id, SharedRoles: members, Provider: provider, Model: model, Dispatched: true, Status: "succeeded", DurationMS: int64(round * 1000)})
					}
				}
			}
			view, _ := ledger.snapshot("request")
			if len(view.PreprocessingRequests) != len(groups)*2 {
				t.Fatal("display placement changed actual request count")
			}
			byID := map[string]turnWorkflowHUDPreprocessingRequest{}
			rows := map[int]map[int]bool{}
			for _, call := range view.PreprocessingRequests {
				byID[call.ID] = call
				if call.DisplayRow <= 0 || call.DurationMS != int64(call.Round*1000) {
					t.Fatalf("invalid placement or altered elapsed time: %+v", call)
				}
				if rows[call.DisplayRow] == nil {
					rows[call.DisplayRow] = map[int]bool{}
				}
				if rows[call.DisplayRow][call.Round] {
					t.Fatal("two distinct requests occupy the same round cell")
				}
				rows[call.DisplayRow][call.Round] = true
			}
			wantRows := len(groups)
			if variant == "subset" || variant == "changed_model" {
				wantRows++
			}
			if len(rows) != wantRows {
				t.Fatalf("rows=%d want=%d", len(rows), wantRows)
			}
			for i := range groups {
				paired := byID[fmt.Sprintf("request-1-%d", i)].DisplayRow == byID[fmt.Sprintf("request-2-%d", i)].DisplayRow
				if paired != (i != 2 || wantRows == len(groups)) {
					t.Fatalf("incorrect round pairing for group %d", i)
				}
			}
		})
	}
}
