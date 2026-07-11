package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

const tableReadTR1ContractVersion = "tr1.table_read_planning.v1"
const tableReadTR2ContractVersion = "tr2.single_model_simulation.v1"
const tableReadReview1ContractVersion = "tr_review_1.assistant_draft_read_only.v1"
const tableReadReview2ContractVersion = "tr_review_2.revision_suggestion.v1"
const tableReadPolish2ContractVersion = "tr_polish_2.output_polish_route_contract.v1"
const tableReadOutputCheck1ContractVersion = "tr_out_1.output_check.v1"
const tableReadMiniRead2ContractVersion = "tr_out_2.mini_table_read.v2"
const tableReadOutputEnhance3ContractVersion = "tr_out_3.output_enhance.v1"

// registerTableReadRoutes mounts read-only Table Read planning endpoints.
func (s *Server) registerTableReadRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /table-read/draft", s.handleTableReadDraft)
	mux.HandleFunc("POST /table-read/simulate", s.handleTableReadSimulate)
	mux.HandleFunc("POST /table-read/review", s.handleTableReadReview)
	mux.HandleFunc("POST /table-read/revise", s.handleTableReadRevise)
	mux.HandleFunc("POST /table-read/polish", s.handleTableReadPolish)
	mux.HandleFunc("POST /table-read/output-check", s.handleTableReadOutputCheck)
	mux.HandleFunc("POST /table-read/mini-read", s.handleTableReadMiniRead)
	mux.HandleFunc("POST /table-read/output-enhance", s.handleTableReadOutputEnhance)
}

type tableReadDraftRequest struct {
	ChatSessionID        string                     `json:"chat_session_id"`
	SceneText            string                     `json:"scene_text"`
	UserInput            string                     `json:"user_input"`
	Entities             []tableReadEntityRequest   `json:"entities"`
	MultiModel           tableReadMultiModelRequest `json:"multi_model"`
	LLM                  dto.ProxyPluginMainRequest `json:"llm"`
	MaxMemoriesPerEntity int                        `json:"max_memories_per_entity"`
	OutputStructureGuard map[string]any             `json:"output_structure_guard"`
}

type tableReadEntityRequest struct {
	EntityKey   string `json:"entity_key"`
	EntityName  string `json:"entity_name"`
	Role        string `json:"role"`
	Perspective string `json:"perspective"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
}

type tableReadMultiModelRequest struct {
	Enabled          bool   `json:"enabled"`
	Mode             string `json:"mode"`
	MaxParallel      int    `json:"max_parallel"`
	RequireConsensus bool   `json:"require_consensus"`
}

type tableReadReviewRequest struct {
	tableReadDraftRequest
	AssistantDraft string `json:"assistant_draft"`
}

type tableReadReviseRequest struct {
	tableReadReviewRequest
	ReviewContext map[string]any `json:"review_context"`
}

type tableReadPolishRequest struct {
	tableReadDraftRequest
	TurnIndex               int            `json:"turn_index"`
	AssistantOutputOriginal string         `json:"assistant_output_original"`
	RecentContextSummary    string         `json:"recent_context_summary"`
	ReviewContext           map[string]any `json:"review_context"`
}

type tableReadOutputCheckRequest struct {
	tableReadDraftRequest
	TurnIndex            int    `json:"turn_index"`
	AssistantDraft       string `json:"assistant_draft"`
	RecentContextSummary string `json:"recent_context_summary"`
}

type tableReadMiniReadRequest struct {
	tableReadDraftRequest
	TurnIndex            int            `json:"turn_index"`
	AssistantDraft       string         `json:"assistant_draft"`
	RecentContextSummary string         `json:"recent_context_summary"`
	OutputCheckContext   map[string]any `json:"output_check_context"`
	MaxEntities          int            `json:"max_entities"`
}

type tableReadOutputEnhanceRequest struct {
	tableReadDraftRequest
	TurnIndex            int            `json:"turn_index"`
	AssistantDraft       string         `json:"assistant_draft"`
	RecentContextSummary string         `json:"recent_context_summary"`
	OutputCheckContext   map[string]any `json:"output_check_context"`
	MiniReadContext      map[string]any `json:"mini_read_context"`
	MaxEntities          int            `json:"max_entities"`
}

func (s *Server) handleTableReadDraft(w http.ResponseWriter, r *http.Request) {
	var req tableReadDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	maxMemories := req.MaxMemoriesPerEntity
	if maxMemories <= 0 {
		maxMemories = 4
	}
	if maxMemories > 8 {
		maxMemories = 8
	}

	agents := s.buildTableReadAgents(r, sid, req.Entities, maxMemories)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"contract_version": tableReadTR1ContractVersion,
		"dry_run_only":     true,
		"write_attempted":  false,
		"would_call_llm":   false,
		"chat_session_id":  sid,
		"table_read": map[string]any{
			"purpose": "Let story-relevant entities review the current scene from their own subjective memory banks before a final narrative suggestion is made.",
			"scene": map[string]any{
				"scene_text_preview": tableReadPreview(req.SceneText, 360),
				"user_input_preview": tableReadPreview(req.UserInput, 240),
			},
			"memory_source": "subjective_entity_memories",
			"agents":        agents,
			"orchestration": buildTableReadOrchestration(req.MultiModel, len(agents)),
			"guards": map[string]any{
				"support_only":               true,
				"canonical_truth_write":      false,
				"private_memory_reveal":      "forbidden_without_explicit_current_scene_permission",
				"npc_recollection_treatment": "interpretation_not_objective_fact",
				"loop_regression_wording":    "avoid_direct_reveal_unless_user_explicitly_names_it",
			},
			"next_step": "TR-2 may execute configured agents; TR-1 only proves routing, memory binding, and multi-model slots.",
		},
	})
}

func (s *Server) handleTableReadSimulate(w http.ResponseWriter, r *http.Request) {
	var req tableReadDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if err := tableReadValidateLLM(req.LLM); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	maxMemories := req.MaxMemoriesPerEntity
	if maxMemories <= 0 {
		maxMemories = 4
	}
	if maxMemories > 8 {
		maxMemories = 8
	}

	agents := s.buildTableReadAgents(r, sid, req.Entities, maxMemories)
	systemPrompt, userPrompt := buildTableReadSingleModelPrompt(sid, req, agents)
	maxTokens := tableReadInt64PtrValue(req.LLM.MaxTokens, 1400)
	if maxTokens <= 0 {
		maxTokens = 1400
	}
	maxCompletionTokens := tableReadInt64PtrValue(req.LLM.MaxCompletionTokens, maxTokens)
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := tableReadFloatPtrValue(req.LLM.Temperature, 0.4)
	timeout := tableReadInt64PtrValue(req.LLM.TimeoutMs, 60000)
	proxyReq := req.LLM
	proxyReq.Messages = []any{
		map[string]any{"role": "system", "content": systemPrompt},
		map[string]any{"role": "user", "content": userPrompt},
	}
	proxyReq.MaxTokens = &maxTokens
	proxyReq.MaxCompletionTokens = &maxCompletionTokens
	proxyReq.Temperature = &temp
	proxyReq.TimeoutMs = &timeout
	upstream, status, err := performProxyPluginMain(r.Context(), proxyReq)
	if err != nil {
		writeJSON(w, status, map[string]any{
			"status":             "error",
			"contract_version":   tableReadTR2ContractVersion,
			"code":               "table_read_llm_failed",
			"detail":             err.Error(),
			"llm_call_attempted": true,
			"write_attempted":    false,
		})
		return
	}

	content := strings.TrimSpace(chatCompletionText(upstream))
	parsed, parseErr := parseJSONFromLLMContent(content)
	parseStatus := "ok"
	if parseErr != nil {
		parseStatus = "raw_text_only"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"contract_version":   tableReadTR2ContractVersion,
		"dry_run_only":       false,
		"write_attempted":    false,
		"llm_call_attempted": true,
		"chat_session_id":    sid,
		"table_read": map[string]any{
			"agents":        agents,
			"orchestration": buildTableReadOrchestration(req.MultiModel, len(agents)),
			"guards": map[string]any{
				"support_only":               true,
				"canonical_truth_write":      false,
				"private_memory_reveal":      "forbidden_without_explicit_current_scene_permission",
				"npc_recollection_treatment": "interpretation_not_objective_fact",
				"loop_regression_wording":    "avoid_direct_reveal_unless_user_explicitly_names_it",
			},
			"simulation": map[string]any{
				"mode":              "single_model_table_read",
				"model":             extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), tableReadStringPtrValue(req.LLM.Model, "")),
				"provider":          tableReadStringPtrValue(req.LLM.Provider, ""),
				"content":           content,
				"parsed_json":       nilIfEmptyMap(parsed),
				"parse_status":      parseStatus,
				"usage":             upstream["usage"],
				"truth_authority":   false,
				"prepare_turn_role": "support_only_candidate",
			},
		},
	})
}

func (s *Server) handleTableReadReview(w http.ResponseWriter, r *http.Request) {
	var req tableReadReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if strings.TrimSpace(req.AssistantDraft) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "assistant_draft is required")
		return
	}
	if err := tableReadValidateLLM(req.LLM); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	maxMemories := req.MaxMemoriesPerEntity
	if maxMemories <= 0 {
		maxMemories = 4
	}
	if maxMemories > 8 {
		maxMemories = 8
	}

	baseReq := req.tableReadDraftRequest
	agents := s.buildTableReadAgents(r, sid, req.Entities, maxMemories)
	systemPrompt, userPrompt := buildTableReadReviewPrompt(sid, req, agents)
	maxTokens := tableReadInt64PtrValue(req.LLM.MaxTokens, 1600)
	if maxTokens <= 0 {
		maxTokens = 1600
	}
	maxCompletionTokens := tableReadInt64PtrValue(req.LLM.MaxCompletionTokens, maxTokens)
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := tableReadFloatPtrValue(req.LLM.Temperature, 0.25)
	timeout := tableReadInt64PtrValue(req.LLM.TimeoutMs, 60000)
	proxyReq := req.LLM
	proxyReq.Messages = []any{
		map[string]any{"role": "system", "content": systemPrompt},
		map[string]any{"role": "user", "content": userPrompt},
	}
	proxyReq.MaxTokens = &maxTokens
	proxyReq.MaxCompletionTokens = &maxCompletionTokens
	proxyReq.Temperature = &temp
	proxyReq.TimeoutMs = &timeout
	upstream, status, err := performProxyPluginMain(r.Context(), proxyReq)
	if err != nil {
		writeJSON(w, status, map[string]any{
			"status":             "error",
			"contract_version":   tableReadReview1ContractVersion,
			"code":               "table_read_review_llm_failed",
			"detail":             err.Error(),
			"llm_call_attempted": true,
			"write_attempted":    false,
			"replaces_output":    false,
		})
		return
	}

	content := strings.TrimSpace(chatCompletionText(upstream))
	parsed, parseErr := parseJSONFromLLMContent(content)
	parseStatus := "ok"
	if parseErr != nil {
		parseStatus = "raw_text_only"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"contract_version":   tableReadReview1ContractVersion,
		"dry_run_only":       false,
		"write_attempted":    false,
		"llm_call_attempted": true,
		"replaces_output":    false,
		"chat_session_id":    sid,
		"table_read": map[string]any{
			"agents":        agents,
			"orchestration": buildTableReadOrchestration(baseReq.MultiModel, len(agents)),
			"guards": map[string]any{
				"review_only":                  true,
				"output_replacement":           false,
				"support_only":                 true,
				"canonical_truth_write":        false,
				"private_memory_reveal":        "forbidden_in_final_output",
				"npc_recollection_treatment":   "interpretation_not_objective_fact",
				"loop_regression_direct_terms": "block_unless_already_explicit_in_draft_or_user_input",
			},
			"review": map[string]any{
				"mode":              "assistant_draft_read_only_table_read_review",
				"model":             extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), tableReadStringPtrValue(req.LLM.Model, "")),
				"provider":          tableReadStringPtrValue(req.LLM.Provider, ""),
				"assistant_draft":   tableReadPreview(req.AssistantDraft, 1200),
				"content":           content,
				"parsed_json":       nilIfEmptyMap(parsed),
				"parse_status":      parseStatus,
				"usage":             upstream["usage"],
				"truth_authority":   false,
				"review_only":       true,
				"replaces_output":   false,
				"prepare_turn_role": "post_output_review_only",
			},
		},
	})
}

func (s *Server) handleTableReadOutputCheck(w http.ResponseWriter, r *http.Request) {
	var req tableReadOutputCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if strings.TrimSpace(req.AssistantDraft) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "assistant_draft is required")
		return
	}
	maxMemories := req.MaxMemoriesPerEntity
	if maxMemories <= 0 {
		maxMemories = 4
	}
	if maxMemories > 8 {
		maxMemories = 8
	}

	agents := s.buildTableReadAgents(r, sid, req.Entities, maxMemories)
	if !tableReadHasLLMConfig(req.LLM) {
		writeJSON(w, http.StatusOK, buildTableReadOutputCheckFallbackResponse(
			sid,
			req,
			agents,
			maxMemories,
			false,
			"llm_not_configured",
			"llm_not_configured",
			"",
			nil,
		))
		return
	}
	if err := tableReadValidateLLM(req.LLM); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	systemPrompt, userPrompt := buildTableReadOutputCheckPrompt(sid, req, agents)
	maxTokens := tableReadInt64PtrValue(req.LLM.MaxTokens, 1200)
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	maxCompletionTokens := tableReadInt64PtrValue(req.LLM.MaxCompletionTokens, maxTokens)
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := tableReadFloatPtrValue(req.LLM.Temperature, 0.15)
	timeout := tableReadInt64PtrValue(req.LLM.TimeoutMs, 60000)
	proxyReq := req.LLM
	proxyReq.Messages = []any{
		map[string]any{"role": "system", "content": systemPrompt},
		map[string]any{"role": "user", "content": userPrompt},
	}
	proxyReq.MaxTokens = &maxTokens
	proxyReq.MaxCompletionTokens = &maxCompletionTokens
	proxyReq.Temperature = &temp
	proxyReq.TimeoutMs = &timeout
	upstream, _, err := performProxyPluginMain(r.Context(), proxyReq)
	if err != nil {
		writeJSON(w, http.StatusOK, buildTableReadOutputCheckFallbackResponse(
			sid,
			req,
			agents,
			maxMemories,
			true,
			"llm_failed",
			"llm_failed",
			err.Error(),
			nil,
		))
		return
	}

	content := strings.TrimSpace(chatCompletionText(upstream))
	parsed, parseErr := parseJSONFromLLMContent(content)
	if parseErr != nil {
		writeJSON(w, http.StatusOK, buildTableReadOutputCheckFallbackResponse(
			sid,
			req,
			agents,
			maxMemories,
			true,
			"raw_text_only",
			"llm_response_not_json",
			tableReadPreview(content, 600),
			upstream["usage"],
		))
		return
	}

	verdict := tableReadNormalizeOutputCheckVerdict(tableReadParsedString(parsed, "verdict", "accept"))
	requiresTableRead := tableReadParsedBool(parsed, "requires_table_read", verdict != "accept")
	requiresOutputEnhance := tableReadParsedBool(parsed, "requires_output_enhance", verdict != "accept")
	fallbackReason := tableReadParsedString(parsed, "fallback_reason", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"contract_version":         tableReadOutputCheck1ContractVersion,
		"dry_run_only":             false,
		"write_attempted":          false,
		"llm_call_attempted":       true,
		"replaces_output":          false,
		"route_can_replace_output": false,
		"chat_session_id":          sid,
		"turn_index":               req.TurnIndex,
		"verdict":                  verdict,
		"requires_table_read":      requiresTableRead,
		"requires_output_enhance":  requiresOutputEnhance,
		"issues":                   tableReadParsedArrayOrEmpty(parsed, "issues"),
		"active_entities":          tableReadParsedArrayOrEmpty(parsed, "active_entities"),
		"protected_reveals":        tableReadParsedArrayOrEmpty(parsed, "protected_reveals"),
		"fallback_reason":          fallbackReason,
		"table_read": map[string]any{
			"mode":          "output_check",
			"agents":        agents,
			"orchestration": buildTableReadOrchestration(req.MultiModel, len(agents)),
			"context":       buildTableReadOutputCheckContext(req, maxMemories),
			"guards":        buildTableReadOutputCheckGuards(),
			"output_check": map[string]any{
				"mode":                     "pre_output_decision_only",
				"model":                    extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), tableReadStringPtrValue(req.LLM.Model, "")),
				"provider":                 tableReadStringPtrValue(req.LLM.Provider, ""),
				"content":                  content,
				"parsed_json":              nilIfEmptyMap(parsed),
				"parse_status":             "ok",
				"usage":                    upstream["usage"],
				"truth_authority":          false,
				"write_attempted":          false,
				"replaces_output":          false,
				"route_can_replace_output": false,
				"candidate_generation":     false,
				"prepare_turn_role":        "pre_output_gate",
			},
		},
	})
}

func (s *Server) handleTableReadMiniRead(w http.ResponseWriter, r *http.Request) {
	var req tableReadMiniReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if strings.TrimSpace(req.AssistantDraft) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "assistant_draft is required")
		return
	}
	maxMemories := req.MaxMemoriesPerEntity
	if maxMemories <= 0 {
		maxMemories = 4
	}
	if maxMemories > 8 {
		maxMemories = 8
	}
	maxEntities := req.MaxEntities
	if maxEntities <= 0 {
		maxEntities = 3
	}
	if maxEntities > 3 {
		maxEntities = 3
	}

	selectedEntities, relevanceTrace := tableReadSelectMiniReadEntities(req, maxEntities)
	agents := s.buildTableReadAgents(r, sid, selectedEntities, maxMemories)
	if len(selectedEntities) == 0 {
		writeJSON(w, http.StatusOK, buildTableReadMiniReadFallbackResponse(
			sid,
			req,
			selectedEntities,
			agents,
			relevanceTrace,
			maxMemories,
			false,
			"no_relevant_entities",
			"no_relevant_entities",
			"",
			nil,
		))
		return
	}
	if !tableReadHasLLMConfig(req.LLM) {
		writeJSON(w, http.StatusOK, buildTableReadMiniReadFallbackResponse(
			sid,
			req,
			selectedEntities,
			agents,
			relevanceTrace,
			maxMemories,
			false,
			"llm_not_configured",
			"llm_not_configured",
			"",
			nil,
		))
		return
	}
	if err := tableReadValidateLLM(req.LLM); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	systemPrompt, userPrompt := buildTableReadMiniReadPrompt(sid, req, agents, relevanceTrace)
	maxTokens := tableReadInt64PtrValue(req.LLM.MaxTokens, 1600)
	if maxTokens <= 0 {
		maxTokens = 1600
	}
	maxCompletionTokens := tableReadInt64PtrValue(req.LLM.MaxCompletionTokens, maxTokens)
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := tableReadFloatPtrValue(req.LLM.Temperature, 0.25)
	timeout := tableReadInt64PtrValue(req.LLM.TimeoutMs, 60000)
	proxyReq := req.LLM
	proxyReq.Messages = []any{
		map[string]any{"role": "system", "content": systemPrompt},
		map[string]any{"role": "user", "content": userPrompt},
	}
	proxyReq.MaxTokens = &maxTokens
	proxyReq.MaxCompletionTokens = &maxCompletionTokens
	proxyReq.Temperature = &temp
	proxyReq.TimeoutMs = &timeout
	upstream, _, err := performProxyPluginMain(r.Context(), proxyReq)
	if err != nil {
		writeJSON(w, http.StatusOK, buildTableReadMiniReadFallbackResponse(
			sid,
			req,
			selectedEntities,
			agents,
			relevanceTrace,
			maxMemories,
			true,
			"llm_failed",
			"llm_failed",
			err.Error(),
			nil,
		))
		return
	}

	content := strings.TrimSpace(chatCompletionText(upstream))
	parsed, parseErr := parseJSONFromLLMContent(content)
	if parseErr != nil {
		writeJSON(w, http.StatusOK, buildTableReadMiniReadFallbackResponse(
			sid,
			req,
			selectedEntities,
			agents,
			relevanceTrace,
			maxMemories,
			true,
			"raw_text_only",
			"llm_response_not_json",
			tableReadPreview(content, 600),
			upstream["usage"],
		))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"contract_version":         tableReadMiniRead2ContractVersion,
		"dry_run_only":             false,
		"write_attempted":          false,
		"llm_call_attempted":       true,
		"replaces_output":          false,
		"route_can_replace_output": false,
		"candidate_generation":     false,
		"chat_session_id":          sid,
		"turn_index":               req.TurnIndex,
		"selected_entities":        tableReadMiniReadSelectedEntitySurface(selectedEntities),
		"relevance_trace":          relevanceTrace,
		"participant_notes":        tableReadParsedArrayOrEmpty(parsed, "participant_notes"),
		"mini_discussion":          tableReadParsedArrayOrEmpty(parsed, "mini_discussion"),
		"moderator_summary":        tableReadParsedString(parsed, "moderator_summary", ""),
		"protected_reveals":        tableReadParsedArrayOrEmpty(parsed, "protected_reveals"),
		"story_risks":              tableReadParsedArrayOrEmpty(parsed, "story_risks"),
		"output_enhance_notes":     tableReadParsedArrayOrEmpty(parsed, "output_enhance_notes"),
		"safe_to_enhance":          tableReadParsedBool(parsed, "safe_to_enhance", true),
		"fallback_reason":          tableReadParsedString(parsed, "fallback_reason", ""),
		"table_read": map[string]any{
			"mode":            "mini_table_read",
			"agents":          agents,
			"orchestration":   buildTableReadOrchestration(req.MultiModel, len(agents)),
			"context":         buildTableReadMiniReadContext(req, maxMemories, maxEntities),
			"guards":          buildTableReadMiniReadGuards(),
			"relevance_trace": relevanceTrace,
			"mini_read": map[string]any{
				"mode":                     "selected_entities_private_review_meeting",
				"model":                    extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), tableReadStringPtrValue(req.LLM.Model, "")),
				"provider":                 tableReadStringPtrValue(req.LLM.Provider, ""),
				"content":                  content,
				"parsed_json":              nilIfEmptyMap(parsed),
				"parse_status":             "ok",
				"usage":                    upstream["usage"],
				"truth_authority":          false,
				"write_attempted":          false,
				"replaces_output":          false,
				"route_can_replace_output": false,
				"candidate_generation":     false,
				"prepare_turn_role":        "pre_output_support_discussion",
			},
		},
	})
}

func (s *Server) handleTableReadOutputEnhance(w http.ResponseWriter, r *http.Request) {
	var req tableReadOutputEnhanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if strings.TrimSpace(req.AssistantDraft) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "assistant_draft is required")
		return
	}
	maxMemories := req.MaxMemoriesPerEntity
	if maxMemories <= 0 {
		maxMemories = 4
	}
	if maxMemories > 8 {
		maxMemories = 8
	}
	maxEntities := req.MaxEntities
	if maxEntities <= 0 {
		maxEntities = 3
	}
	if maxEntities > 3 {
		maxEntities = 3
	}

	miniReq := tableReadMiniReadRequest{
		tableReadDraftRequest: req.tableReadDraftRequest,
		TurnIndex:             req.TurnIndex,
		AssistantDraft:        req.AssistantDraft,
		RecentContextSummary:  req.RecentContextSummary,
		OutputCheckContext:    req.OutputCheckContext,
		MaxEntities:           maxEntities,
	}
	selectedEntities, relevanceTrace := tableReadSelectMiniReadEntities(miniReq, maxEntities)
	agents := s.buildTableReadAgents(r, sid, selectedEntities, maxMemories)
	if !tableReadHasLLMConfig(req.LLM) {
		writeJSON(w, http.StatusOK, buildTableReadOutputEnhanceFallbackResponse(
			sid,
			req,
			selectedEntities,
			agents,
			relevanceTrace,
			maxMemories,
			false,
			"llm_not_configured",
			"llm_not_configured",
			"",
			nil,
		))
		return
	}
	if err := tableReadValidateLLM(req.LLM); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	systemPrompt, userPrompt := buildTableReadOutputEnhancePrompt(sid, req, agents, relevanceTrace)
	maxTokens := tableReadInt64PtrValue(req.LLM.MaxTokens, 2400)
	if maxTokens <= 0 {
		maxTokens = 2400
	}
	maxCompletionTokens := tableReadInt64PtrValue(req.LLM.MaxCompletionTokens, maxTokens)
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := tableReadFloatPtrValue(req.LLM.Temperature, 0.25)
	timeout := tableReadInt64PtrValue(req.LLM.TimeoutMs, 60000)
	proxyReq := req.LLM
	proxyReq.Messages = []any{
		map[string]any{"role": "system", "content": systemPrompt},
		map[string]any{"role": "user", "content": userPrompt},
	}
	proxyReq.MaxTokens = &maxTokens
	proxyReq.MaxCompletionTokens = &maxCompletionTokens
	proxyReq.Temperature = &temp
	proxyReq.TimeoutMs = &timeout
	upstream, _, err := performProxyPluginMain(r.Context(), proxyReq)
	if err != nil {
		writeJSON(w, http.StatusOK, buildTableReadOutputEnhanceFallbackResponse(
			sid,
			req,
			selectedEntities,
			agents,
			relevanceTrace,
			maxMemories,
			true,
			"llm_failed",
			"llm_failed",
			err.Error(),
			nil,
		))
		return
	}

	content := strings.TrimSpace(chatCompletionText(upstream))
	parsed, parseErr := parseJSONFromLLMContent(content)
	if parseErr != nil {
		writeJSON(w, http.StatusOK, buildTableReadOutputEnhanceFallbackResponse(
			sid,
			req,
			selectedEntities,
			agents,
			relevanceTrace,
			maxMemories,
			true,
			"raw_text_only",
			"llm_response_not_json",
			tableReadPreview(content, 600),
			upstream["usage"],
		))
		return
	}

	finalOutput := tableReadParsedString(parsed, "assistant_output_final", "")
	patches := tableReadParsedArrayOrEmpty(parsed, "patches")
	if strings.TrimSpace(finalOutput) == "" && len(patches) == 0 {
		writeJSON(w, http.StatusOK, buildTableReadOutputEnhanceFallbackResponse(
			sid,
			req,
			selectedEntities,
			agents,
			relevanceTrace,
			maxMemories,
			true,
			"ok",
			"assistant_output_final_missing",
			"",
			upstream["usage"],
		))
		return
	}
	changed := tableReadParsedBool(parsed, "changed", len(patches) > 0 || strings.TrimSpace(finalOutput) != strings.TrimSpace(req.AssistantDraft))
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"contract_version":         tableReadOutputEnhance3ContractVersion,
		"dry_run_only":             false,
		"write_attempted":          false,
		"llm_call_attempted":       true,
		"replaces_output":          true,
		"route_can_replace_output": true,
		"candidate_generation":     false,
		"chat_session_id":          sid,
		"turn_index":               req.TurnIndex,
		"assistant_output_final":   finalOutput,
		"patches":                  patches,
		"changed":                  changed,
		"selected_entities":        tableReadMiniReadSelectedEntitySurface(selectedEntities),
		"relevance_trace":          relevanceTrace,
		"issues_repaired":          tableReadParsedArrayOrEmpty(parsed, "issues_repaired"),
		"protected_reveals":        tableReadParsedArrayOrEmpty(parsed, "protected_reveals"),
		"entity_review_trace":      tableReadParsedArrayOrEmpty(parsed, "entity_review_trace"),
		"fallback_reason":          tableReadParsedString(parsed, "fallback_reason", ""),
		"table_read": map[string]any{
			"mode":            "output_enhance",
			"agents":          agents,
			"orchestration":   buildTableReadOrchestration(req.MultiModel, len(agents)),
			"context":         buildTableReadOutputEnhanceContext(req, maxMemories, maxEntities, "llm_final_output"),
			"guards":          buildTableReadOutputEnhanceGuards(),
			"relevance_trace": relevanceTrace,
			"output_enhance": map[string]any{
				"mode":                     "pre_output_final_rewrite",
				"model":                    extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), tableReadStringPtrValue(req.LLM.Model, "")),
				"provider":                 tableReadStringPtrValue(req.LLM.Provider, ""),
				"content":                  content,
				"parsed_json":              nilIfEmptyMap(parsed),
				"parse_status":             "ok",
				"usage":                    upstream["usage"],
				"truth_authority":          false,
				"write_attempted":          false,
				"replaces_output":          true,
				"route_can_replace_output": true,
				"candidate_generation":     false,
				"prepare_turn_role":        "pre_output_final_enhance",
			},
		},
	})
}

func (s *Server) handleTableReadRevise(w http.ResponseWriter, r *http.Request) {
	var req tableReadReviseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if strings.TrimSpace(req.AssistantDraft) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "assistant_draft is required")
		return
	}
	if err := tableReadValidateLLM(req.LLM); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	maxMemories := req.MaxMemoriesPerEntity
	if maxMemories <= 0 {
		maxMemories = 4
	}
	if maxMemories > 8 {
		maxMemories = 8
	}

	agents := s.buildTableReadAgents(r, sid, req.Entities, maxMemories)
	systemPrompt, userPrompt := buildTableReadRevisePrompt(sid, req, agents)
	maxTokens := tableReadInt64PtrValue(req.LLM.MaxTokens, 1800)
	if maxTokens <= 0 {
		maxTokens = 1800
	}
	maxCompletionTokens := tableReadInt64PtrValue(req.LLM.MaxCompletionTokens, maxTokens)
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := tableReadFloatPtrValue(req.LLM.Temperature, 0.35)
	timeout := tableReadInt64PtrValue(req.LLM.TimeoutMs, 60000)
	proxyReq := req.LLM
	proxyReq.Messages = []any{
		map[string]any{"role": "system", "content": systemPrompt},
		map[string]any{"role": "user", "content": userPrompt},
	}
	proxyReq.MaxTokens = &maxTokens
	proxyReq.MaxCompletionTokens = &maxCompletionTokens
	proxyReq.Temperature = &temp
	proxyReq.TimeoutMs = &timeout
	upstream, status, err := performProxyPluginMain(r.Context(), proxyReq)
	if err != nil {
		writeJSON(w, status, map[string]any{
			"status":             "error",
			"contract_version":   tableReadReview2ContractVersion,
			"code":               "table_read_revise_llm_failed",
			"detail":             err.Error(),
			"llm_call_attempted": true,
			"write_attempted":    false,
			"replaces_output":    false,
		})
		return
	}

	content := strings.TrimSpace(chatCompletionText(upstream))
	parsed, parseErr := parseJSONFromLLMContent(content)
	parseStatus := "ok"
	if parseErr != nil {
		parseStatus = "raw_text_only"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"contract_version":   tableReadReview2ContractVersion,
		"dry_run_only":       false,
		"write_attempted":    false,
		"llm_call_attempted": true,
		"replaces_output":    false,
		"chat_session_id":    sid,
		"table_read": map[string]any{
			"agents":        agents,
			"orchestration": buildTableReadOrchestration(req.MultiModel, len(agents)),
			"guards": map[string]any{
				"revision_suggestion_only": true,
				"output_replacement":       false,
				"support_only":             true,
				"canonical_truth_write":    false,
				"private_memory_reveal":    "forbidden_in_revised_draft",
				"copy_only":                true,
				"auto_apply":               false,
			},
			"revision": map[string]any{
				"mode":              "assistant_draft_revision_suggestion",
				"model":             extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), tableReadStringPtrValue(req.LLM.Model, "")),
				"provider":          tableReadStringPtrValue(req.LLM.Provider, ""),
				"assistant_draft":   tableReadPreview(req.AssistantDraft, 1200),
				"content":           content,
				"parsed_json":       nilIfEmptyMap(parsed),
				"parse_status":      parseStatus,
				"usage":             upstream["usage"],
				"truth_authority":   false,
				"replaces_output":   false,
				"copy_only":         true,
				"prepare_turn_role": "post_output_revision_suggestion",
			},
		},
	})
}
