package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

// Grouping belongs to one prepare request and one analysis round. The key is
// transient effective transport configuration, including credentials, and is
// never exported, logged, or reused as any persistent/request identity.
func (s *Server) callMultiAgentRound(ctx context.Context, settings multiAgentSettings, round int, roles []multiAgentRoleResult, inputs []map[string]any, sessionID string) []multiAgentCall {
	if settings.Jev.Enabled && !settings.Enabled {
		return s.callJevRound(ctx, settings, round, roles, inputs)
	}
	groupingStarted := time.Now()
	out := make([]multiAgentCall, len(roles))
	groups := [][]int{}
	positions := map[string]int{}
	for i, role := range roles {
		if inputs[i] == nil {
			continue
		}
		call, req := s.multiAgentProxyRequest(role.Role, settings, round, inputs[i], true)
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
	groupingMS := durationMilliseconds(time.Since(groupingStarted))
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
	// Round setup is serial; physical calls run in parallel. Report setup once.
	if len(groups) > 0 {
		i := groups[0][0]
		if out[i].TimingMS == nil {
			out[i].TimingMS = map[string]float64{}
		}
		out[i].TimingMS["round_grouping"] = groupingMS
	}
	return out
}

// Factor identical fields first, then repeated records inside differing reading
// lists. References are expanded within each role's original local source scope.
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
	packet := map[string]any{"contract_version": "memory_preprocessing.group.v1", "shared_input": shared, "roles": assignments}
	baseline, _ := json.Marshal(packet)
	return multiAgentShareReadingRecords(packet, baseline)
}

// This changes only the wire representation of already selected reading data.
// No fact, source, passage, role-local reference or choice is removed. Small
// packets keep their original format when a dictionary would cost more characters.
func multiAgentShareReadingRecords(packet map[string]any, baseline []byte) string {
	// Work on the wire copy, never the canonical role inputs used by selection.
	_ = json.Unmarshal(baseline, &packet)
	fields := []string{"candidates", "turn_summaries", "lorebook_candidates", "related_evidence", "search_evidence", "recent_conversation", "source_catalog", "source_scopes"}
	inputs := []map[string]any{packet}
	if roles, ok := packet["roles"].([]any); ok {
		inputs = []map[string]any{mapFromAny(packet["shared_input"])}
		for _, role := range roles {
			inputs = append(inputs, mapFromAny(mapFromAny(role)["input"]))
		}
	}
	counts := map[string]int{}
	var visit func(any, bool)
	visit = func(v any, readingText bool) {
		switch node := v.(type) {
		case map[string]any:
			b, _ := json.Marshal(node)
			counts[string(b)]++
			for key, child := range node {
				visit(child, key == "text" || key == "Text")
			}
		case []any:
			for _, child := range node {
				visit(child, false)
			}
		case string:
			if readingText {
				b, _ := json.Marshal(node)
				counts[string(b)]++
			}
		}
	}
	for _, input := range inputs {
		for _, field := range fields {
			if value, ok := input[field]; ok {
				visit(value, false)
			}
		}
	}
	shared := map[string]any{}
	refs := map[string]string{}
	var replace func(any, bool) any
	replace = func(v any, readingText bool) any {
		_, object := v.(map[string]any)
		if object || readingText {
			b, _ := json.Marshal(v)
			key := string(b)
			// Account for both dictionary entry and the referring objects.
			if counts[key] > 1 && (counts[key]-1)*len(b) > counts[key]*40+32 {
				ref := refs[key]
				if ref == "" {
					ref = fmt.Sprintf("R%d", len(shared)+1)
					refs[key] = ref
					shared[ref] = nil // Reserve the number before nested readings.
					if node, ok := v.(map[string]any); ok {
						keys := make([]string, 0, len(node))
						for k := range node {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						for _, k := range keys {
							node[k] = replace(node[k], k == "text" || k == "Text")
						}
					}
					shared[ref] = v
				}
				return map[string]any{"shared_record": ref}
			}
		}
		switch node := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(node))
			for key := range node {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				node[key] = replace(node[key], key == "text" || key == "Text")
			}
			return node
		case []any:
			for i := range node {
				node[i] = replace(node[i], false)
			}
			return node
		default:
			return v
		}
	}
	for _, input := range inputs {
		for _, field := range fields {
			if value, ok := input[field]; ok {
				input[field] = replace(value, false)
			}
		}
	}
	if len(shared) == 0 {
		return string(baseline)
	}
	packet["shared_records"] = shared
	packet["shared_records_format"] = "Resolve {shared_record:Rn} to its exact shared_records object or text string; then combine shared_input and role input. F/S/L/C/P/G refs, source time, holder and visibility stay role-local. Shared text is not shared knowledge. Return F/S/L/C, never R."
	b, _ := json.Marshal(packet)
	if utf8.RuneCount(b) >= utf8.RuneCount(baseline) {
		return string(baseline)
	}
	return string(b)
}

func parseMultiAgentGroupedResults(raw string) map[string]string {
	out := map[string]string{}
	raw = repairJSONCandidate(raw)
	start := strings.Index(raw, "{")
	if start < 0 {
		return out
	}
	inRoles := false
	for at := start + 1; at < len(raw); {
		if raw[at] == '}' {
			inRoles = false
		}
		if raw[at] != '"' {
			at++
			continue
		}
		// Read whole JSON strings/values, so role-like prose and nested metadata
		// cannot become recommendations. A misplaced outer brace does not discard
		// a later, independently received role object.
		keyReader := json.NewDecoder(strings.NewReader(raw[at:]))
		var key string
		if keyReader.Decode(&key) != nil {
			break
		}
		at += int(keyReader.InputOffset())
		for at < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[at])) {
			at++
		}
		if at >= len(raw) || raw[at] != ':' {
			continue
		}
		at++
		for at < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[at])) {
			at++
		}
		if at >= len(raw) {
			break
		}
		if key == "roles" && raw[at] == '{' {
			inRoles = true
			at++
			continue
		}
		valueReader := json.NewDecoder(strings.NewReader(raw[at:]))
		var value json.RawMessage
		err := valueReader.Decode(&value)
		if inRoles || multiAgentRoleNames[key] != "" {
			if err != nil {
				// Preserve the interrupted role for the existing partial-ID parser.
				value = []byte(strings.TrimSpace(raw[at:]))
			}
			out[key] = string(value)
		}
		if err != nil {
			break
		}
		at += int(valueReader.InputOffset())
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
		call, prepared := s.multiAgentProxyRequest(role, settings, round, inputs[i], true)
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
	responseRoles := make(map[string]any, len(roles))
	for _, role := range roles {
		shape := map[string]any{"selected_ids": []string{}, "selected_summary_ids": []string{}, "reasons": map[string]string{}, "search_requests": []string{}, "related_requests": []any{}, "unresolved": []string{}}
		if round == 2 {
			shape["reuse_previous_reasons"] = true
		} else {
			shape["recent_context_refs"] = []string{}
		}
		if role == "world_state" {
			shape["selected_lorebook_refs"] = []string{}
		}
		responseRoles[role] = shape
	}
	responseShape, _ := json.Marshal(map[string]any{"roles": responseRoles})
	prompt := sharedPrompt + "\n\n" + multiAgentReviewTransport + `
This request contains the assignments listed in roles. For each assignment, shared_input plus its input is its reading packet; its prompt is the editorial task and output_tokens is an output ceiling, not a length target.
Return ONE JSON object containing a result for EVERY supplied assignment in roles, not just the first assignment or the role_keys handoff directory. Each value uses the single-assignment schema above. Complete the assignments independently in the same response, preserving each one's useful evidence and uncertainty. Do not stop after answering one role or merely referring work to another role.
Complete response shape for this request (answer every role; each list contains its scene-relevant choices, not all available refs; empty optional search and handoff lists are valid):
` + string(responseShape) + `
For each output role, use that role's selectable_refs directory and reading packet. If role A owns F1 and role B owns F2, A's selected_ids and reason keys use F1; do not copy B's F2 into A's answer. A handoff from A to B sends A's public_handoff_refs, while its reason asks B about B's evidence. Public handoffs and search_evidence supply context for the receiving role's own selections. Keep source time, perspective and visibility attached; private evidence remains private. The user retains creative direction.
Report selections separately for each assignment. Answering every role does not mean selecting every candidate. Original evidence is delivered by Go; reasons explain source-supported connections and retain the source's uncertainty. In round two reassess which evidence helps this scene, reuse unchanged reasons for retained refs, revise unsupported interpretations, and return complete final selection lists. Search and handoff allowances need not be used when the supplied packet already answers the relevant question.`
	modelInput := multiAgentGroupedInput(calls, roles, settings)
	req.MaxTokens, req.MaxCompletionTokens = &totalOutput, &totalOutput
	req.Messages = []any{map[string]any{"role": "system", "content": prompt}, map[string]any{"role": "user", "content": modelInput}}
	callTiming := map[string]float64{"request_preparation": durationMilliseconds(time.Since(started))}
	providerStarted := time.Now()
	upstream, status, err := performProxyPluginMainWithRetryBudgetAndPolicy(ctx, req, nil, proxyRequestPolicy{Purpose: "memory_preprocessing", SessionID: sessionID})
	callTiming["provider_operation"] = durationMilliseconds(time.Since(providerStarted))
	decodeStarted := time.Now()
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
	callTiming["response_processing"] = durationMilliseconds(time.Since(decodeStarted))
	if len(calls) > 0 {
		calls[0].TimingMS = callTiming
	}
	return calls
}
