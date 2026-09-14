package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

// Grouping belongs to one prepare request and one analysis round. The key is
// transient effective transport configuration, including credentials, and is
// never exported, logged, or reused as any persistent/request identity.
func (s *Server) callMultiAgentRound(ctx context.Context, settings multiAgentSettings, round int, roles []multiAgentRoleResult, inputs []map[string]any, sessionID string) []multiAgentCall {
	out := make([]multiAgentCall, len(roles))
	groups := [][]int{}
	positions := map[string]int{}
	for i, role := range roles {
		if inputs[i] == nil {
			continue
		}
		call, req := s.multiAgentProxyRequest(role.Role, settings, round, inputs[i])
		req.Messages = nil // Role prompts are separate assignments, not connection options.
		// Match the existing provider transport's normalization without changing
		// the role's saved settings or the request that will be dispatched.
		provider := strings.ToLower(strings.TrimSpace(stringPtrValue(req.Provider, "")))
		endpoint := proxyProviderBaseURL(provider, stringPtrValue(req.Endpoint, ""))
		model, apiKey := strings.TrimSpace(stringPtrValue(req.Model, "")), strings.TrimSpace(stringPtrValue(req.APIKey, ""))
		req.Provider, req.Endpoint, req.Model, req.APIKey = &provider, &endpoint, &model, &apiKey
		if tier := stringPtrValue(req.LLMGatewayServiceTier, ""); tier != "" {
			if normalized, ok := proxyNormalizeLLMGatewayServiceTier(tier); ok {
				req.LLMGatewayServiceTier = &normalized
			}
		}
		encoded, _ := json.Marshal(req)
		key := string(encoded)
		if call.Error != "" {
			key = "unconfigured:" + role.Role
		}
		position, exists := positions[key]
		if !exists {
			position = len(groups)
			positions[key] = position
			groups = append(groups, nil)
		}
		groups[position] = append(groups[position], i)
	}
	var wg sync.WaitGroup
	for _, indexes := range groups {
		wg.Add(1)
		go func(indexes []int) {
			defer wg.Done()
			if len(indexes) == 1 {
				i := indexes[0]
				out[i] = s.callMultiAgent(ctx, roles[i].Role, settings, round, inputs[i], sessionID)
				return
			}
			groupRoles := make([]string, len(indexes))
			groupInputs := make([]map[string]any, len(indexes))
			for j, i := range indexes {
				groupRoles[j], groupInputs[j] = roles[i].Role, inputs[i]
			}
			calls := s.callMultiAgentGroup(ctx, settings, round, groupRoles, groupInputs, sessionID)
			for j, i := range indexes {
				out[i] = calls[j]
			}
		}(indexes)
	}
	wg.Wait()
	return out
}

// Only byte-identical serialized fields are factored. Role-local provenance
// catalogs and C passage choices that differ remain role-local. Expanding
// shared_input over each input reconstructs that role's original model packet.
func multiAgentGroupedInput(calls []multiAgentCall, roles []string, settings multiAgentSettings) string {
	inputs := make([]map[string]json.RawMessage, len(calls))
	for i, call := range calls {
		_ = json.Unmarshal([]byte(call.ModelInput), &inputs[i])
	}
	shared := map[string]json.RawMessage{}
	for key, value := range inputs[0] {
		if key == "role" {
			continue
		}
		same := true
		for _, input := range inputs[1:] {
			if string(input[key]) != string(value) {
				same = false
				break
			}
		}
		if same {
			shared[key] = value
			for _, input := range inputs {
				delete(input, key)
			}
		}
	}
	assignments := make([]map[string]any, len(calls))
	for i, role := range roles {
		prompt := settings.Roles[role].Prompt
		if strings.TrimSpace(prompt) == "" {
			prompt = multiAgentRolePrompts[role]
		}
		cfg, _ := settings.roleConnection(role)
		budget := cfg.MaxTokens
		if budget <= 0 {
			budget = 2048
		}
		assignments[i] = map[string]any{"role": role, "prompt": prompt, "input": inputs[i], "output_tokens": budget}
	}
	b, _ := json.Marshal(map[string]any{"contract_version": "memory_preprocessing.group.v1", "shared_input": shared, "roles": assignments})
	return string(b)
}

func parseMultiAgentGroupedResults(raw string) map[string]string {
	out := map[string]string{}
	raw = repairJSONCandidate(raw)
	start := strings.Index(raw, "{")
	if start < 0 {
		return out
	}
	raw = raw[start:]
	d := json.NewDecoder(strings.NewReader(raw))
	if _, err := d.Token(); err != nil {
		return out
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			break
		}
		if key != "roles" {
			var ignored json.RawMessage
			if d.Decode(&ignored) != nil {
				break
			}
			continue
		}
		token, err := d.Token()
		if err != nil || token != json.Delim('{') {
			break
		}
		for d.More() {
			role, err := d.Token()
			if err != nil {
				break
			}
			offset := d.InputOffset()
			var value json.RawMessage
			err = d.Decode(&value)
			if err != nil {
				// Keep the received prefix for the existing partial-ID parser.
				value = []byte(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw[offset:]), ":")))
			}
			if name, ok := role.(string); ok {
				out[name] = string(value)
			}
			if err != nil {
				break
			}
		}
		break
	}
	return out
}

func (s *Server) callMultiAgentGroup(ctx context.Context, settings multiAgentSettings, round int, roles []string, inputs []map[string]any, sessionID string) []multiAgentCall {
	started := time.Now()
	requestID, _ := ctx.Value(multiAgentHUDRequestKey{}).(string)
	sharedID := fmt.Sprintf("round:%d:%s", round, strings.Join(roles, ","))
	calls := make([]multiAgentCall, len(roles))
	var req dto.ProxyPluginMainRequest
	var totalOutput int64
	for i, role := range roles {
		call, prepared := s.multiAgentProxyRequest(role, settings, round, inputs[i])
		calls[i], req = call, prepared
		totalOutput += int64Value(prepared.MaxTokens, 0)
		s.TurnWorkflows.recordPreprocessingCall(requestID, role, turnWorkflowHUDPreprocessingCall{
			Round: round, Status: "running", StartedAt: started.UTC(), SharedRequestID: sharedID, SharedRoles: roles,
			Provider: stringPtrValue(prepared.Provider, ""), Model: call.Model, Dispatched: true,
		})
	}
	sharedPrompt := settings.SharedPrompt
	if strings.TrimSpace(sharedPrompt) == "" {
		sharedPrompt = multiAgentSharedPrompt
	}
	prompt := sharedPrompt + "\n\n" + multiAgentReviewTransport + `
This request contains several memory editing assignments. For each role, shared_input plus its input is its reading packet; its prompt and output_tokens describe that assignment.
Return ONE JSON object with a roles object keyed by each supplied role, for example {"roles":{"event_recent":{"selected_ids":[],"selected_summary_ids":[],"reasons":{},"search_requests":[],"related_requests":[],"unresolved":[]}}}.
Each role's value follows the recommendation contract above, including its complete final selection in round two. Preserve each assignment's candidate references, perspective, source time and visibility. A source supplied to another assignment remains attributed to that source and role; private evidence remains private. Cross-role requests use the existing public-reference contract. The user retains creative direction.
Keep explanations concise within each assignment's output allocation. Report each role's result separately.`
	modelInput := multiAgentGroupedInput(calls, roles, settings)
	req.MaxTokens, req.MaxCompletionTokens = &totalOutput, &totalOutput
	req.Messages = []any{map[string]any{"role": "system", "content": prompt}, map[string]any{"role": "user", "content": modelInput}}
	upstream, status, err := performProxyPluginMainWithRetryBudgetAndPolicy(ctx, req, nil, proxyRequestPolicy{Purpose: "memory_preprocessing", SessionID: sessionID})
	raw, _, _ := normalizePublisherResponseContent(upstream)
	usage := upstream["usage"]
	if usage == nil {
		usage = upstream["usageMetadata"]
	}
	results := parseMultiAgentGroupedResults(raw)
	duration := time.Since(started).Milliseconds()
	for i, role := range roles {
		call := calls[i]
		call.SharedRequestID, call.SharedRoles, call.Dispatched = sharedID, append([]string(nil), roles...), true
		call.Raw = results[role]
		call = finishMultiAgentCall(call, status, err, stringPtrValue(req.APIKey, ""))
		call.DurationMs = duration
		// Keep canonical per-role input, but attribute the actual wire packet,
		// response and provider usage once, to the first member of the request.
		call.ModelInput, call.Prompt, call.Usage, call.RequestRaw = "", "", nil, ""
		call.ModelInputChars, call.SystemPromptChars, call.ModelInputSectionsChars = 0, 0, nil
		if i == 0 {
			call.ModelInput, call.Prompt, call.Usage, call.RequestRaw = modelInput, prompt, usage, raw
			call.ModelInputChars, call.SystemPromptChars = len([]rune(modelInput)), len([]rune(prompt))
			var sections map[string]json.RawMessage
			_ = json.Unmarshal([]byte(modelInput), &sections)
			call.ModelInputSectionsChars = map[string]int{}
			for key, value := range sections {
				call.ModelInputSectionsChars[key] = len([]rune(string(value)))
			}
		}
		calls[i] = call
		state := "succeeded"
		if call.Error != "" {
			state = "failed"
		}
		if call.ResponseStatus != "" {
			state = call.ResponseStatus
		}
		s.TurnWorkflows.recordPreprocessingCall(requestID, role, turnWorkflowHUDPreprocessingCall{
			Round: round, Status: state, StartedAt: started.UTC(), DurationMS: duration, SharedRequestID: sharedID, SharedRoles: roles,
			Provider: stringPtrValue(req.Provider, ""), Model: call.Model, Dispatched: call.Dispatched,
		})
		if call.Error != "" || state == "partial" || state == "repaired" {
			slog.WarnContext(ctx, "preprocessing result", "request_id", requestID, "shared_request_id", sharedID,
				"role", role, "round", round, "model", call.Model, "status", state, "duration_ms", duration, "error", call.Error)
		}
	}
	return calls
}
