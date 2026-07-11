package httpapi

import (
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func buildTableReadOutputCheckFallbackResponse(sid string, req tableReadOutputCheckRequest, agents []map[string]any, maxMemories int, llmAttempted bool, parseStatus string, fallbackReason string, detail string, usage any) map[string]any {
	return map[string]any{
		"status":                   "ok",
		"contract_version":         tableReadOutputCheck1ContractVersion,
		"dry_run_only":             !llmAttempted,
		"write_attempted":          false,
		"llm_call_attempted":       llmAttempted,
		"replaces_output":          false,
		"route_can_replace_output": false,
		"chat_session_id":          sid,
		"turn_index":               req.TurnIndex,
		"verdict":                  "accept",
		"requires_table_read":      false,
		"requires_output_enhance":  false,
		"issues":                   []any{},
		"active_entities":          []any{},
		"protected_reveals":        []any{},
		"fallback_reason":          fallbackReason,
		"table_read": map[string]any{
			"mode":          "output_check_fail_open",
			"agents":        agents,
			"orchestration": buildTableReadOrchestration(req.MultiModel, len(agents)),
			"context":       buildTableReadOutputCheckContext(req, maxMemories),
			"guards":        buildTableReadOutputCheckGuards(),
			"output_check": map[string]any{
				"mode":                     "pre_output_decision_only",
				"model":                    tableReadStringPtrValue(req.LLM.Model, ""),
				"provider":                 tableReadStringPtrValue(req.LLM.Provider, ""),
				"parse_status":             parseStatus,
				"fallback_reason":          fallbackReason,
				"detail":                   detail,
				"usage":                    usage,
				"truth_authority":          false,
				"write_attempted":          false,
				"replaces_output":          false,
				"route_can_replace_output": false,
				"candidate_generation":     false,
				"prepare_turn_role":        "pre_output_gate_fail_open",
			},
		},
	}
}

func buildTableReadMiniReadFallbackResponse(sid string, req tableReadMiniReadRequest, selectedEntities []tableReadEntityRequest, agents []map[string]any, relevanceTrace []map[string]any, maxMemories int, llmAttempted bool, parseStatus string, fallbackReason string, detail string, usage any) map[string]any {
	return map[string]any{
		"status":                   "ok",
		"contract_version":         tableReadMiniRead2ContractVersion,
		"dry_run_only":             !llmAttempted,
		"write_attempted":          false,
		"llm_call_attempted":       llmAttempted,
		"replaces_output":          false,
		"route_can_replace_output": false,
		"candidate_generation":     false,
		"chat_session_id":          sid,
		"turn_index":               req.TurnIndex,
		"selected_entities":        tableReadMiniReadSelectedEntitySurface(selectedEntities),
		"relevance_trace":          relevanceTrace,
		"participant_notes":        []any{},
		"mini_discussion":          []any{},
		"moderator_summary":        "",
		"protected_reveals":        []any{},
		"story_risks":              []any{},
		"output_enhance_notes":     []any{},
		"safe_to_enhance":          false,
		"fallback_reason":          fallbackReason,
		"table_read": map[string]any{
			"mode":            "mini_table_read_fail_open",
			"agents":          agents,
			"orchestration":   buildTableReadOrchestration(req.MultiModel, len(agents)),
			"context":         buildTableReadMiniReadContext(req, maxMemories, req.MaxEntities),
			"guards":          buildTableReadMiniReadGuards(),
			"relevance_trace": relevanceTrace,
			"mini_read": map[string]any{
				"mode":                     "selected_entities_private_review_meeting",
				"model":                    tableReadStringPtrValue(req.LLM.Model, ""),
				"provider":                 tableReadStringPtrValue(req.LLM.Provider, ""),
				"parse_status":             parseStatus,
				"fallback_reason":          fallbackReason,
				"detail":                   detail,
				"usage":                    usage,
				"truth_authority":          false,
				"write_attempted":          false,
				"replaces_output":          false,
				"route_can_replace_output": false,
				"candidate_generation":     false,
				"prepare_turn_role":        "pre_output_support_discussion_fail_open",
			},
		},
	}
}

func buildTableReadOutputEnhanceFallbackResponse(sid string, req tableReadOutputEnhanceRequest, selectedEntities []tableReadEntityRequest, agents []map[string]any, relevanceTrace []map[string]any, maxMemories int, llmAttempted bool, parseStatus string, fallbackReason string, detail string, usage any) map[string]any {
	return map[string]any{
		"status":                   "ok",
		"contract_version":         tableReadOutputEnhance3ContractVersion,
		"dry_run_only":             !llmAttempted,
		"write_attempted":          false,
		"llm_call_attempted":       llmAttempted,
		"replaces_output":          true,
		"route_can_replace_output": true,
		"candidate_generation":     false,
		"chat_session_id":          sid,
		"turn_index":               req.TurnIndex,
		"assistant_output_final":   req.AssistantDraft,
		"patches":                  []any{},
		"changed":                  false,
		"selected_entities":        tableReadMiniReadSelectedEntitySurface(selectedEntities),
		"relevance_trace":          relevanceTrace,
		"issues_repaired":          []any{},
		"protected_reveals":        []any{},
		"entity_review_trace":      []any{},
		"fallback_reason":          fallbackReason,
		"table_read": map[string]any{
			"mode":            "output_enhance_fail_open",
			"agents":          agents,
			"orchestration":   buildTableReadOrchestration(req.MultiModel, len(agents)),
			"context":         buildTableReadOutputEnhanceContext(req, maxMemories, req.MaxEntities, "original_draft_passthrough"),
			"guards":          buildTableReadOutputEnhanceGuards(),
			"relevance_trace": relevanceTrace,
			"output_enhance": map[string]any{
				"mode":                     "pre_output_final_rewrite",
				"model":                    tableReadStringPtrValue(req.LLM.Model, ""),
				"provider":                 tableReadStringPtrValue(req.LLM.Provider, ""),
				"parse_status":             parseStatus,
				"fallback_reason":          fallbackReason,
				"detail":                   detail,
				"usage":                    usage,
				"truth_authority":          false,
				"write_attempted":          false,
				"replaces_output":          true,
				"route_can_replace_output": true,
				"candidate_generation":     false,
				"prepare_turn_role":        "pre_output_final_enhance_fail_open",
			},
		},
	}
}

func buildTableReadOutputCheckContext(req tableReadOutputCheckRequest, maxMemories int) map[string]any {
	return map[string]any{
		"scene_text_preview":       tableReadPreview(req.SceneText, 800),
		"user_input_preview":       tableReadPreview(req.UserInput, 600),
		"assistant_draft_preview":  tableReadPreview(req.AssistantDraft, 1000),
		"recent_context_summary":   tableReadPreview(req.RecentContextSummary, 600),
		"max_memories_per_entity":  maxMemories,
		"output_check_only":        true,
		"final_output_unmodified":  true,
		"candidate_generation_off": true,
	}
}

func buildTableReadOutputEnhanceContext(req tableReadOutputEnhanceRequest, maxMemories int, maxEntities int, finalOutputSource string) map[string]any {
	if maxEntities <= 0 || maxEntities > 3 {
		maxEntities = 3
	}
	return map[string]any{
		"scene_text_preview":       tableReadPreview(req.SceneText, 800),
		"user_input_preview":       tableReadPreview(req.UserInput, 600),
		"assistant_draft_preview":  tableReadPreview(req.AssistantDraft, 1200),
		"recent_context_summary":   tableReadPreview(req.RecentContextSummary, 600),
		"output_check_attached":    len(req.OutputCheckContext) > 0,
		"mini_read_attached":       len(req.MiniReadContext) > 0,
		"max_memories_per_entity":  maxMemories,
		"max_entities":             maxEntities,
		"final_output_source":      finalOutputSource,
		"output_enhance_only":      true,
		"database_write_attempted": false,
		"candidate_generation_off": true,
	}
}

func buildTableReadMiniReadContext(req tableReadMiniReadRequest, maxMemories int, maxEntities int) map[string]any {
	if maxEntities <= 0 || maxEntities > 3 {
		maxEntities = 3
	}
	return map[string]any{
		"scene_text_preview":       tableReadPreview(req.SceneText, 800),
		"user_input_preview":       tableReadPreview(req.UserInput, 600),
		"assistant_draft_preview":  tableReadPreview(req.AssistantDraft, 1000),
		"recent_context_summary":   tableReadPreview(req.RecentContextSummary, 600),
		"output_check_attached":    len(req.OutputCheckContext) > 0,
		"max_memories_per_entity":  maxMemories,
		"max_entities":             maxEntities,
		"mini_read_only":           true,
		"final_output_unmodified":  true,
		"candidate_generation_off": true,
	}
}

func buildTableReadOutputCheckGuards() map[string]any {
	return map[string]any{
		"support_only":                 true,
		"canonical_truth_write":        false,
		"memory_write":                 false,
		"kg_write":                     false,
		"direct_evidence_write":        false,
		"output_replacement":           false,
		"route_can_replace_output":     false,
		"candidate_generation":         false,
		"private_memory_reveal":        "forbidden_in_final_output",
		"npc_recollection_treatment":   "interpretation_not_objective_fact",
		"loop_regression_direct_terms": "block_unless_already_explicit_in_draft_or_user_input",
		"deliberation_only":            true,
		"roleplay_dialogue":            false,
		"new_scene_generation":         false,
		"fail_open":                    true,
	}
}

func buildTableReadOutputEnhanceGuards() map[string]any {
	return map[string]any{
		"support_only":                 true,
		"canonical_truth_write":        false,
		"memory_write":                 false,
		"kg_write":                     false,
		"direct_evidence_write":        false,
		"output_replacement":           true,
		"route_can_replace_output":     true,
		"candidate_generation":         false,
		"max_entities":                 3,
		"selection_basis":              "current_scene_mentions_and_output_check_active_entities",
		"subjective_memory_use":        "private_support_only",
		"private_memory_reveal":        "forbidden_in_final_output",
		"npc_recollection_treatment":   "interpretation_not_objective_fact",
		"loop_regression_direct_terms": "block_unless_already_explicit_in_draft_or_user_input",
		"fallback_to_original":         true,
	}
}

func buildTableReadMiniReadGuards() map[string]any {
	return map[string]any{
		"support_only":                 true,
		"canonical_truth_write":        false,
		"memory_write":                 false,
		"kg_write":                     false,
		"direct_evidence_write":        false,
		"output_replacement":           false,
		"route_can_replace_output":     false,
		"candidate_generation":         false,
		"max_entities":                 3,
		"selection_basis":              "current_scene_mentions_and_output_check_active_entities",
		"subjective_memory_use":        "private_support_only",
		"private_memory_reveal":        "forbidden_in_final_output",
		"npc_recollection_treatment":   "interpretation_not_objective_fact",
		"loop_regression_direct_terms": "block_unless_already_explicit_in_draft_or_user_input",
		"fail_open":                    true,
	}
}

func tableReadSelectMiniReadEntities(req tableReadMiniReadRequest, maxEntities int) ([]tableReadEntityRequest, []map[string]any) {
	if maxEntities <= 0 || maxEntities > 3 {
		maxEntities = 3
	}
	type scoredEntity struct {
		entity  tableReadEntityRequest
		score   int
		reasons []string
		index   int
	}
	userText := strings.ToLower(req.UserInput)
	draftText := strings.ToLower(req.AssistantDraft)
	sceneText := strings.ToLower(req.SceneText)
	recentText := strings.ToLower(req.RecentContextSummary)
	activeText := strings.ToLower(strings.Join(tableReadOutputCheckActiveEntityNames(req.OutputCheckContext), " "))

	scored := make([]scoredEntity, 0, len(req.Entities))
	for i, entity := range req.Entities {
		terms := tableReadEntityMatchTerms(entity)
		score := 0
		reasons := []string{}
		if tableReadTextHasAnyTerm(activeText, terms) {
			score += 120
			reasons = append(reasons, "output_check_active_entity")
		}
		if tableReadTextHasAnyTerm(userText, terms) {
			score += 100
			reasons = append(reasons, "user_input_direct_mention")
		}
		if tableReadTextHasAnyTerm(draftText, terms) {
			score += 80
			reasons = append(reasons, "assistant_draft_direct_mention")
		}
		if tableReadTextHasAnyTerm(sceneText, terms) {
			score += 50
			reasons = append(reasons, "scene_text_direct_mention")
		}
		if tableReadTextHasAnyTerm(recentText, terms) {
			score += 20
			reasons = append(reasons, "recent_context_mention")
		}
		scored = append(scored, scoredEntity{entity: entity, score: score, reasons: reasons, index: i})
	}

	hasDirectSignal := false
	for _, item := range scored {
		if item.score > 0 {
			hasDirectSignal = true
			break
		}
	}
	if !hasDirectSignal && len(scored) <= maxEntities {
		for i := range scored {
			if tableReadMiniReadFallbackRole(scored[i].entity.Role) {
				scored[i].score = 30
				scored[i].reasons = append(scored[i].reasons, "small_entity_set_scene_participant_fallback")
			}
		}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score > scored[j].score
	})

	selected := []tableReadEntityRequest{}
	selectedIndexes := map[int]bool{}
	for _, item := range scored {
		if item.score <= 0 {
			continue
		}
		if len(selected) >= maxEntities {
			break
		}
		selected = append(selected, item.entity)
		selectedIndexes[item.index] = true
	}

	trace := make([]map[string]any, 0, len(scored))
	for _, item := range scored {
		reason := "not_current_scene_relevant"
		if len(item.reasons) > 0 {
			reason = strings.Join(item.reasons, ",")
		}
		trace = append(trace, map[string]any{
			"entity_key":   item.entity.EntityKey,
			"entity_name":  item.entity.EntityName,
			"role":         item.entity.Role,
			"score":        item.score,
			"selected":     selectedIndexes[item.index],
			"reason":       reason,
			"support_only": true,
		})
	}
	return selected, trace
}

func tableReadEntityMatchTerms(entity tableReadEntityRequest) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" || len([]rune(term)) < 2 || strings.HasPrefix(term, "char_") || strings.Contains(term, "_cid_") {
			return
		}
		if !seen[term] {
			seen[term] = true
			out = append(out, term)
		}
	}
	add(entity.EntityName)
	add(entity.EntityKey)
	for _, raw := range []string{entity.EntityName, entity.EntityKey} {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ' ' || r == '_' || r == '-' || r == '/' || r == '(' || r == ')' || r == '[' || r == ']'
		}) {
			add(part)
		}
	}
	return out
}

func tableReadTextHasAnyTerm(text string, terms []string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func tableReadOutputCheckActiveEntityNames(ctx map[string]any) []string {
	if ctx == nil {
		return nil
	}
	v, ok := ctx["active_entities"]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := strings.TrimSpace(extractionStringFromAny(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	case string:
		return []string{t}
	default:
		return nil
	}
}

func tableReadMiniReadFallbackRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "protagonist", "player", "persona", "npc", "character":
		return true
	default:
		return false
	}
}

func tableReadMiniReadSelectedEntitySurface(entities []tableReadEntityRequest) []map[string]any {
	out := make([]map[string]any, 0, len(entities))
	for _, entity := range entities {
		out = append(out, map[string]any{
			"entity_key":   entity.EntityKey,
			"entity_name":  entity.EntityName,
			"role":         entity.Role,
			"support_only": true,
		})
	}
	return out
}

func tableReadHasLLMConfig(req dto.ProxyPluginMainRequest) bool {
	return strings.TrimSpace(tableReadStringPtrValue(req.Provider, "")) != "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.Endpoint, "")) != "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.APIKey, "")) != "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.Model, "")) != ""
}

func tableReadParsedArrayOrEmpty(parsed map[string]any, key string) []any {
	if parsed == nil {
		return []any{}
	}
	v, ok := parsed[key]
	if !ok || v == nil {
		return []any{}
	}
	if arr, ok := v.([]any); ok {
		return arr
	}
	if arr, ok := v.([]string); ok {
		out := make([]any, 0, len(arr))
		for _, item := range arr {
			out = append(out, item)
		}
		return out
	}
	return []any{v}
}

func tableReadParsedString(parsed map[string]any, key string, fallback string) string {
	if parsed == nil {
		return fallback
	}
	if v, ok := parsed[key]; ok {
		if s := strings.TrimSpace(extractionStringFromAny(v)); s != "" {
			return s
		}
	}
	return fallback
}

func tableReadParsedBool(parsed map[string]any, key string, fallback bool) bool {
	if parsed == nil {
		return fallback
	}
	v, ok := parsed[key]
	if !ok {
		return fallback
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "1":
			return true
		case "false", "no", "0":
			return false
		}
	}
	return fallback
}

func tableReadNormalizeOutputCheckVerdict(verdict string) string {
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case "accept":
		return "accept"
	case "minor_revise":
		return "minor_revise"
	case "major_revise", "regenerate_recommended":
		return "major_revise"
	default:
		return "accept"
	}
}
