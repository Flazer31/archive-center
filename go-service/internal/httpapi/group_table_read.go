package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
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

func (s *Server) handleTableReadPolish(w http.ResponseWriter, r *http.Request) {
	var req tableReadPolishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if strings.TrimSpace(req.AssistantOutputOriginal) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "assistant_output_original is required")
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
	if tableReadHasLLMConfig(req.LLM) {
		if err := tableReadValidateLLM(req.LLM); err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
			return
		}
		systemPrompt, userPrompt := buildTableReadPolishPrompt(sid, req, agents)
		maxTokens := tableReadInt64PtrValue(req.LLM.MaxTokens, 2200)
		if maxTokens <= 0 {
			maxTokens = 2200
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
				"status":                   "error",
				"contract_version":         tableReadPolish2ContractVersion,
				"code":                     "table_read_polish_llm_failed",
				"detail":                   err.Error(),
				"llm_call_attempted":       true,
				"write_attempted":          false,
				"route_can_replace_output": true,
				"assistant_output_final":   req.AssistantOutputOriginal,
				"changed":                  false,
				"fallback_reason":          "llm_call_failed",
			})
			return
		}

		content := strings.TrimSpace(chatCompletionText(upstream))
		parsed, parseErr := parseJSONFromLLMContent(content)
		parseStatus := "ok"
		if parseErr != nil {
			parseStatus = "raw_text_only"
		}
		assistantOutputFinal := strings.TrimSpace(extractionFirstNonEmpty(
			extractionStringFromAny(parsed["assistant_output_final"]),
			extractionStringFromAny(parsed["final_output"]),
			extractionStringFromAny(parsed["revised_draft"]),
		))
		fallbackReason := ""
		if assistantOutputFinal == "" {
			assistantOutputFinal = req.AssistantOutputOriginal
			if parseErr != nil {
				fallbackReason = "llm_output_not_json"
			} else {
				fallbackReason = "assistant_output_final_missing"
			}
		}
		changed := strings.TrimSpace(assistantOutputFinal) != strings.TrimSpace(req.AssistantOutputOriginal)
		if !changed && fallbackReason == "" {
			fallbackReason = "llm_returned_original_or_equivalent"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                   "ok",
			"contract_version":         tableReadPolish2ContractVersion,
			"dry_run_only":             false,
			"write_attempted":          false,
			"llm_call_attempted":       true,
			"route_can_replace_output": true,
			"changed":                  changed,
			"fallback_reason":          nullableString(fallbackReason),
			"chat_session_id":          sid,
			"turn_index":               req.TurnIndex,
			"assistant_output_final":   assistantOutputFinal,
			"issues":                   tableReadParsedArrayOrEmpty(parsed, "issues"),
			"protected_reveals":        tableReadParsedArrayOrEmpty(parsed, "protected_reveals"),
			"entity_review_trace":      tableReadParsedArrayOrEmpty(parsed, "entity_review_trace"),
			"table_read": map[string]any{
				"mode":          "live_llm_output_polish",
				"agents":        agents,
				"orchestration": buildTableReadOrchestration(req.MultiModel, len(agents)),
				"context":       buildTableReadPolishContext(req, maxMemories, "llm_final_output"),
				"guards":        buildTableReadPolishGuards(),
				"polish": map[string]any{
					"model":                    extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), tableReadStringPtrValue(req.LLM.Model, "")),
					"provider":                 tableReadStringPtrValue(req.LLM.Provider, ""),
					"content":                  content,
					"parsed_json":              nilIfEmptyMap(parsed),
					"parse_status":             parseStatus,
					"usage":                    upstream["usage"],
					"truth_authority":          false,
					"write_attempted":          false,
					"replaces_output":          true,
					"assistant_output_changed": changed,
					"fallback_reason":          nullableString(fallbackReason),
					"prepare_turn_role":        "post_output_polish_final",
				},
			},
		})
		return
	}

	assistantOutputFinal := req.AssistantOutputOriginal
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"contract_version":         tableReadPolish2ContractVersion,
		"dry_run_only":             true,
		"write_attempted":          false,
		"llm_call_attempted":       false,
		"route_can_replace_output": true,
		"changed":                  false,
		"fallback_reason":          "tr_polish_2_route_contract_only",
		"chat_session_id":          sid,
		"turn_index":               req.TurnIndex,
		"assistant_output_final":   assistantOutputFinal,
		"issues":                   []any{},
		"protected_reveals":        []any{},
		"entity_review_trace": []any{
			map[string]any{
				"status":          "not_run",
				"reason":          "live_llm_polish_not_connected_until_tr_polish_3",
				"agent_count":     len(agents),
				"support_only":    true,
				"truth_authority": false,
			},
		},
		"table_read": map[string]any{
			"mode":          "output_polish_route_contract",
			"agents":        agents,
			"orchestration": buildTableReadOrchestration(req.MultiModel, len(agents)),
			"context":       buildTableReadPolishContext(req, maxMemories, "original_output_passthrough"),
			"guards":        buildTableReadPolishGuards(),
			"next_step":     "TR-POLISH-3 connects a live LLM to produce assistant_output_final; TR-POLISH-2 only proves the backend output replacement contract.",
		},
	})
}

func (s *Server) buildTableReadAgents(r *http.Request, sid string, entities []tableReadEntityRequest, maxMemories int) []map[string]any {
	out := make([]map[string]any, 0, len(entities))
	memStore, hasMemStore := s.Store.(store.ProtagonistEntityMemoryStore)
	for idx, entity := range entities {
		owner := s.canonicalSubjectiveEntityOwner(r.Context(), sid, entity.EntityKey, entity.EntityName)
		role := strings.TrimSpace(entity.Role)
		if role == "" {
			role = "participant"
		}
		memories := []store.ProtagonistEntityMemory{}
		if hasMemStore && owner.Key != "" {
			items, err := s.listProtagonistEntityMemoriesByCanonicalOwner(r.Context(), memStore, store.ProtagonistEntityMemoryFilter{
				OwnerEntityKey:      owner.Key,
				SourceChatSessionID: sid,
				Limit:               maxMemories,
			})
			if err == nil {
				memories = s.canonicalizeSubjectiveEntityMemoriesForRead(r.Context(), sid, items)
			}
		}
		out = append(out, map[string]any{
			"slot":                  idx + 1,
			"entity_key":            owner.Key,
			"entity_name":           firstNonEmpty(owner.Name, entity.EntityName, entity.EntityKey),
			"role":                  role,
			"perspective":           firstNonEmpty(strings.TrimSpace(entity.Perspective), tableReadDefaultPerspective(role)),
			"model_provider":        nullableString(strings.TrimSpace(entity.Provider)),
			"model":                 nullableString(strings.TrimSpace(entity.Model)),
			"memory_count":          len(memories),
			"memory_cards":          tableReadMemoryCards(memories, maxMemories),
			"private_memory_policy": tableReadPrivateMemoryPolicy(role),
		})
	}
	return out
}

func buildTableReadSingleModelPrompt(sid string, req tableReadDraftRequest, agents []map[string]any) (string, string) {
	systemPrompt := strings.Join([]string{
		"You are Archive Center's Table Read moderator.",
		"Simulate a private writers-room style discussion between the listed entities.",
		"Use each entity's subjective memory cards only as support, interpretation, bias, or suspicion.",
		"Never promote private recollection into objective canon.",
		"Never reveal loop, regression, reincarnation, or isekai secrets unless the current user input explicitly reveals them.",
		"Return concise JSON only.",
	}, "\n")
	payload := map[string]any{
		"contract_version": tableReadTR2ContractVersion,
		"chat_session_id":  sid,
		"scene_text":       tableReadPreview(req.SceneText, 1600),
		"user_input":       tableReadPreview(req.UserInput, 900),
		"agents":           agents,
		"required_json_schema": map[string]any{
			"agent_notes":       "array of {entity_name, private_read, concern, desired_direction}",
			"discussion":        "array of short character-perspective comments",
			"moderator_summary": "short support-only synthesis",
			"story_hints":       "array of subtle next-scene hints; no canonical writes",
			"blocked_reveals":   "array of secrets or subjective claims that must not be narrated directly",
		},
	}
	b, _ := json.Marshal(payload)
	return systemPrompt, string(b)
}

func buildTableReadReviewPrompt(sid string, req tableReadReviewRequest, agents []map[string]any) (string, string) {
	systemPrompt := strings.Join([]string{
		"You are Archive Center's Table Read review moderator.",
		"Review the assistant draft after it was generated, as if the involved characters are reading the script.",
		"Do not rewrite the final answer in TR-Review-1. Only evaluate and suggest revision notes.",
		"Use each entity's subjective memory cards only as support, interpretation, bias, or suspicion.",
		"Flag voice mismatch, knowledge leaks, private memory leaks, continuity breaks, and rushed story movement.",
		"Never promote private recollection into objective canon.",
		"Never reveal loop, regression, reincarnation, or isekai secrets unless already explicit in the draft or user input.",
		"Return concise JSON only.",
	}, "\n")
	payload := map[string]any{
		"contract_version": tableReadReview1ContractVersion,
		"chat_session_id":  sid,
		"scene_text":       tableReadPreview(req.SceneText, 1200),
		"user_input":       tableReadPreview(req.UserInput, 900),
		"assistant_draft":  tableReadPreviewPreserveLines(req.AssistantDraft, 2400),
		"agents":           agents,
		"required_json_schema": map[string]any{
			"verdict":           "accept | minor_revise | major_revise | regenerate_recommended",
			"character_reviews": "array of {entity_name, voice_fit, knowledge_leak, private_memory_leak, emotion_fit, concern}",
			"story_continuity":  "object with location_ok, time_ok, relationship_ok, scene_flow_ok, notes",
			"protected_reveals": "array of subjective/private/secret facts that should not be narrated directly",
			"revision_notes":    "array of short actionable notes; no full rewritten output in TR-Review-1",
			"review_dialogue":   "optional array of {speaker, line}; make the review read like a short script table-read discussion",
			"safe_to_publish":   "boolean, true only when no major revision/regeneration is needed",
		},
	}
	b, _ := json.Marshal(payload)
	return systemPrompt, string(b)
}

func buildTableReadOutputCheckPrompt(sid string, req tableReadOutputCheckRequest, agents []map[string]any) (string, string) {
	systemPrompt := strings.Join([]string{
		"You are Archive Center's TR-OUT-1 output check moderator.",
		"Evaluate the assistant draft before it is allowed to become the final RisuAI output.",
		"Do not rewrite the draft. Do not provide a revised candidate. Do not output final prose.",
		"Only decide whether the draft can pass, needs a mini Table Read, or needs output enhancement.",
		"Use each entity's subjective memory cards only as support, interpretation, bias, or suspicion.",
		"Never promote private recollection into objective canon.",
		"Never reveal loop, regression, reincarnation, or isekai secrets unless already explicit in the user input or draft.",
		"Return concise JSON only.",
	}, "\n")
	payload := map[string]any{
		"contract_version":       tableReadOutputCheck1ContractVersion,
		"chat_session_id":        sid,
		"turn_index":             req.TurnIndex,
		"scene_text":             tableReadPreview(req.SceneText, 1200),
		"user_input":             tableReadPreview(req.UserInput, 900),
		"assistant_draft":        tableReadPreviewPreserveLines(req.AssistantDraft, 2600),
		"recent_context_summary": tableReadPreview(req.RecentContextSummary, 800),
		"agents":                 agents,
		"required_json_schema": map[string]any{
			"verdict":                 "accept | minor_revise | major_revise",
			"requires_table_read":     "boolean; true only when character-perspective discussion is needed before improvement",
			"requires_output_enhance": "boolean; true when the draft needs repair before final return",
			"issues":                  "array of short labels such as voice_mismatch, knowledge_leak, private_memory_leak, emotion_fit, scene_flow",
			"active_entities":         "array of entity names actually relevant to this draft",
			"protected_reveals":       "array of private/secret/subjective facts that must stay out of direct narration",
			"fallback_reason":         "empty string unless the check cannot decide safely",
		},
		"forbidden_response_fields": []string{
			"assistant_output_final",
			"revised_draft",
			"candidate_text",
			"final_answer",
		},
		"guards": buildTableReadOutputCheckGuards(),
	}
	b, _ := json.Marshal(payload)
	return systemPrompt, string(b)
}

func buildTableReadMiniReadPrompt(sid string, req tableReadMiniReadRequest, agents []map[string]any, relevanceTrace []map[string]any) (string, string) {
	systemPrompt := strings.Join([]string{
		"You are Archive Center's TR-OUT-2 mini Table Read moderator.",
		"Run a small private review meeting only for the selected scene-relevant entities.",
		"This is deliberation, not roleplay. Participants review the draft from behind the table; they do not speak as characters inside the scene.",
		"Do not write in-character dialogue, stage directions, or next-scene lines inside mini_discussion.",
		"Do not rewrite the assistant draft. Do not provide a candidate output. Do not return final prose.",
		"Use subjective entity memory cards only as private interpretation, bias, concern, or subtext.",
		"Never narrate private recollection as objective fact.",
		"Never reveal loop, regression, reincarnation, isekai, or other secret knowledge unless already explicit in the user input or draft.",
		"Each participant should identify voice fit, emotional fit, secret/private-memory leakage, continuity, or scene-flow risks.",
		"The moderator should turn those reviews into concise output_enhance_notes for TR-OUT-3.",
		"Keep the result short enough for a later output enhancer to consume.",
		"Return concise JSON only.",
	}, "\n")
	payload := map[string]any{
		"contract_version":       tableReadMiniRead2ContractVersion,
		"chat_session_id":        sid,
		"turn_index":             req.TurnIndex,
		"scene_text":             tableReadPreview(req.SceneText, 1200),
		"user_input":             tableReadPreview(req.UserInput, 900),
		"assistant_draft":        tableReadPreviewPreserveLines(req.AssistantDraft, 2600),
		"recent_context_summary": tableReadPreview(req.RecentContextSummary, 800),
		"output_check_context":   req.OutputCheckContext,
		"selected_agents":        agents,
		"relevance_trace":        relevanceTrace,
		"required_json_schema": map[string]any{
			"participant_notes":    "array of {entity_name, perspective, concern, safe_direction}; do not expose private memory directly",
			"mini_discussion":      "array of {speaker, stance, comment}; review meeting notes only. Do not write in-character dialogue, quoted scene lines, stage directions, or next-scene prose",
			"moderator_summary":    "short support-only synthesis for the later output enhancer; no story prose",
			"protected_reveals":    "array of secrets/private interpretations that must stay out of direct narration",
			"story_risks":          "array of voice, continuity, emotional fit, scene flow, or knowledge-leak risks",
			"output_enhance_notes": "array of concise editorial instructions for TR-OUT-3; no rewritten draft and no new scene beats",
			"safe_to_enhance":      "boolean",
			"fallback_reason":      "empty string unless mini read cannot decide safely",
		},
		"forbidden_response_fields": []string{
			"assistant_output_final",
			"revised_draft",
			"candidate_text",
			"final_answer",
		},
		"guards": buildTableReadMiniReadGuards(),
	}
	b, _ := json.Marshal(payload)
	return systemPrompt, string(b)
}

func buildTableReadOutputEnhancePrompt(sid string, req tableReadOutputEnhanceRequest, agents []map[string]any, relevanceTrace []map[string]any) (string, string) {
	systemPrompt := strings.Join([]string{
		"You are Archive Center's TR-OUT-3 output enhancement editor.",
		"Your job is to return the final assistant response that RisuAI may display.",
		"This is not a candidate list and not a review report.",
		"Prefer returning patches for mutable body segments instead of rewriting the whole draft.",
		"Only patch segment IDs listed as mutable_segments in output_structure_guard.",
		"Never patch protected_segments. Never alter image tags, scene headers, chapter/title lines, Chatindex lines, or regex-facing markers.",
		"Preserve the complete assistant draft. Do not summarize, truncate, skip paragraphs, or return only the changed part.",
		"Preserve output-control lines, image tags, scene headers, chapter/title lines, and regex-facing markers exactly when present.",
		"Preserve the user's current input, scene action, and intended story beat.",
		"Apply Output Check and Mini Table Read editorial notes only when they safely improve voice, continuity, emotional fit, or secret handling.",
		"Do not treat mini_discussion as scene dialogue. Do not import participant comments as new character speech or new scene action.",
		"Use mini_read_context.moderator_summary and output_enhance_notes as constraints, not as story content.",
		"Do not add new events, new reactions, new dialogue turns, or new setting facts that are not already in the draft/user input.",
		"Use subjective entity memories only as subtext, hesitation, bias, misunderstanding, tone, or private pressure.",
		"Never narrate NPC private recollection as objective fact.",
		"Never reveal loop, regression, reincarnation, isekai, or other private knowledge unless already explicit in user input or draft.",
		"If safe improvement is impossible, return the original assistant draft exactly as assistant_output_final.",
		"If you cannot preserve the full draft while improving it, return the original assistant draft exactly.",
		"Return concise JSON only. Do not wrap the final answer in markdown.",
	}, "\n")
	payload := map[string]any{
		"contract_version":       tableReadOutputEnhance3ContractVersion,
		"chat_session_id":        sid,
		"turn_index":             req.TurnIndex,
		"scene_text":             tableReadPreview(req.SceneText, 1200),
		"user_input":             tableReadPreview(req.UserInput, 900),
		"assistant_draft":        tableReadPreviewPreserveLines(req.AssistantDraft, 12000),
		"output_structure_guard": req.OutputStructureGuard,
		"recent_context_summary": tableReadPreview(req.RecentContextSummary, 800),
		"output_check_context":   req.OutputCheckContext,
		"mini_read_context":      req.MiniReadContext,
		"selected_agents":        agents,
		"relevance_trace":        relevanceTrace,
		"required_json_schema": map[string]any{
			"patches":                "preferred: array of {segment_id, replacement, reason}; only target mutable_segments listed in output_structure_guard; never target protected_segments",
			"assistant_output_final": "fallback only: the complete final assistant response text; preserve all original output-control/header/image/regex lines and do not omit later paragraphs",
			"changed":                "boolean",
			"issues_repaired":        "array of short issue labels repaired or noticed",
			"protected_reveals":      "array of private/secret/subjective items intentionally kept out of direct narration",
			"entity_review_trace":    "array of {entity_name, concern, applied_change, private_memory_reveal_blocked}",
			"fallback_reason":        "empty string when changed safely; otherwise why original was returned",
		},
		"guards": buildTableReadOutputEnhanceGuards(),
	}
	b, _ := json.Marshal(payload)
	return systemPrompt, string(b)
}

func buildTableReadRevisePrompt(sid string, req tableReadReviseRequest, agents []map[string]any) (string, string) {
	systemPrompt := strings.Join([]string{
		"You are Archive Center's Table Read revision editor.",
		"Suggest one revised assistant draft based on the review findings and the involved entities' subjective memories.",
		"This is TR-Review-2: produce a revision suggestion only. Do not claim the output has been applied.",
		"Preserve the user's current input and scene action before adding interpretation.",
		"Keep private recollections as subtext, gesture, hesitation, or ambiguity. Do not expose them as narrator fact.",
		"Never reveal loop, regression, reincarnation, or isekai secrets unless already explicit in the draft or user input.",
		"Return concise JSON only.",
	}, "\n")
	payload := map[string]any{
		"contract_version": tableReadReview2ContractVersion,
		"chat_session_id":  sid,
		"scene_text":       tableReadPreview(req.SceneText, 1200),
		"user_input":       tableReadPreview(req.UserInput, 900),
		"assistant_draft":  tableReadPreview(req.AssistantDraft, 2600),
		"review_context":   req.ReviewContext,
		"agents":           agents,
		"required_json_schema": map[string]any{
			"verdict":                     "accept | minor_revise | major_revise | regenerate_recommended",
			"revision_strategy":           "short explanation of how the draft should be repaired",
			"revised_draft":               "the suggested replacement text only; do not include JSON or analysis inside it",
			"change_notes":                "array of concise changes made",
			"protected_reveals_preserved": "boolean; true only if private/secret recollections remain subtext",
			"remaining_risks":             "array of any unresolved concerns",
		},
	}
	b, _ := json.Marshal(payload)
	return systemPrompt, string(b)
}

func buildTableReadPolishPrompt(sid string, req tableReadPolishRequest, agents []map[string]any) (string, string) {
	systemPrompt := strings.Join([]string{
		"You are Archive Center's Table Read output polish editor.",
		"Your job is to return the final assistant response, not a candidate and not analysis.",
		"Repair voice, scene flow, continuity, private-memory leakage, and emotional fit only when needed.",
		"Preserve the complete assistant output. Do not summarize, truncate, skip paragraphs, or return only the changed part.",
		"Preserve output-control lines, image tags, scene headers, chapter/title lines, and regex-facing markers exactly when present.",
		"Preserve the user's current input and the already generated scene intent.",
		"Use subjective entity memories only as subtext, hesitation, bias, misunderstanding, tone, or private pressure.",
		"Never narrate NPC private recollection as objective fact.",
		"Never reveal loop, regression, reincarnation, or isekai knowledge unless already explicit in the user input or original output.",
		"If you cannot improve safely, return the original assistant output exactly as assistant_output_final.",
		"If you cannot preserve the full output while improving it, return the original assistant output exactly.",
		"Return concise JSON only. Do not wrap the final answer in markdown.",
	}, "\n")
	payload := map[string]any{
		"contract_version":          tableReadPolish2ContractVersion,
		"chat_session_id":           sid,
		"turn_index":                req.TurnIndex,
		"scene_text":                tableReadPreview(req.SceneText, 1200),
		"user_input":                tableReadPreview(req.UserInput, 900),
		"assistant_output_original": tableReadPreviewPreserveLines(req.AssistantOutputOriginal, 12000),
		"recent_context_summary":    tableReadPreview(req.RecentContextSummary, 1000),
		"review_context":            req.ReviewContext,
		"agents":                    agents,
		"required_json_schema": map[string]any{
			"assistant_output_final": "the complete final assistant response text only; preserve all original output-control/header/image/regex lines and do not omit later paragraphs",
			"changed":                "boolean",
			"issues":                 "array of short issue labels repaired or noticed",
			"protected_reveals":      "array of private/secret/subjective items intentionally kept out of direct narration",
			"entity_review_trace":    "array of {entity_name, concern, applied_change, private_memory_reveal_blocked}",
			"fallback_reason":        "empty string when changed safely; otherwise why original was returned",
		},
		"guards": buildTableReadPolishGuards(),
	}
	b, _ := json.Marshal(payload)
	return systemPrompt, string(b)
}

func buildTableReadPolishContext(req tableReadPolishRequest, maxMemories int, finalOutputSource string) map[string]any {
	return map[string]any{
		"scene_text_preview":        tableReadPreview(req.SceneText, 800),
		"user_input_preview":        tableReadPreview(req.UserInput, 600),
		"assistant_output_preview":  tableReadPreview(req.AssistantOutputOriginal, 1200),
		"recent_context_summary":    tableReadPreview(req.RecentContextSummary, 600),
		"review_context_attached":   len(req.ReviewContext) > 0,
		"max_memories_per_entity":   maxMemories,
		"original_output_preserved": true,
		"final_output_source":       finalOutputSource,
	}
}

func buildTableReadPolishGuards() map[string]any {
	return map[string]any{
		"support_only":                 true,
		"canonical_truth_write":        false,
		"memory_write":                 false,
		"kg_write":                     false,
		"direct_evidence_write":        false,
		"private_memory_reveal":        "forbidden_in_final_output",
		"npc_recollection_treatment":   "interpretation_not_objective_fact",
		"loop_regression_direct_terms": "block_unless_already_explicit_in_output_or_user_input",
		"fallback_to_original":         true,
	}
}

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

func tableReadValidateLLM(req dto.ProxyPluginMainRequest) error {
	if strings.TrimSpace(tableReadStringPtrValue(req.Provider, "")) == "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.Endpoint, "")) == "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.APIKey, "")) == "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.Model, "")) == "" {
		return &tableReadValidationError{"llm.provider, llm.endpoint, llm.api_key, and llm.model are required"}
	}
	return nil
}

type tableReadValidationError struct {
	message string
}

func (e *tableReadValidationError) Error() string {
	return e.message
}

func tableReadStringPtrValue(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

func tableReadInt64PtrValue(v *int64, fallback int64) int64 {
	if v == nil {
		return fallback
	}
	return *v
}

func tableReadFloatPtrValue(v *float64, fallback float64) float64 {
	if v == nil {
		return fallback
	}
	return *v
}

func nilIfEmptyMap(v map[string]any) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

func buildTableReadOrchestration(req tableReadMultiModelRequest, agentCount int) map[string]any {
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "single_orchestrator_dry_run"
	}
	maxParallel := req.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 1
	}
	if maxParallel > agentCount && agentCount > 0 {
		maxParallel = agentCount
	}
	return map[string]any{
		"multi_model_supported": true,
		"multi_model_enabled":   req.Enabled,
		"execution_mode":        mode,
		"max_parallel":          maxParallel,
		"require_consensus":     req.RequireConsensus,
		"execution_order": []string{
			"agent_private_notes",
			"cross_character_discussion",
			"moderator_synthesis",
			"support_only_prepare_turn_hint",
		},
		"tr1_execution_guard": "no_llm_call_in_tr1",
	}
}

func tableReadMemoryCards(memories []store.ProtagonistEntityMemory, limit int) []map[string]any {
	if limit <= 0 || limit > len(memories) {
		limit = len(memories)
	}
	out := make([]map[string]any, 0, limit)
	for _, memory := range memories[:limit] {
		out = append(out, map[string]any{
			"id":                   memory.ID,
			"source_turn_index":    memory.SourceTurn,
			"memory_text_preview":  tableReadPreview(memory.MemoryText, 180),
			"secret_guard":         memory.SecretGuard,
			"target_reveal_policy": memory.TargetRevealPolicy,
			"portability":          memory.Portability,
			"importance_10":        memory.Importance10,
			"emotional_weight":     memory.EmotionalWeight,
		})
	}
	return out
}

func tableReadPrivateMemoryPolicy(role string) map[string]any {
	role = strings.ToLower(strings.TrimSpace(role))
	private := role == "npc" || role == "character" || strings.Contains(role, "private")
	lane := "persona_recollection"
	if private {
		lane = "character_private_recollection"
	}
	return map[string]any{
		"lane":              lane,
		"support_only":      true,
		"reveal_to_player":  !private,
		"treat_as":          "subjective_interpretation",
		"truth_authority":   false,
		"canonical_write":   false,
		"scene_use_allowed": true,
	}
}

func tableReadDefaultPerspective(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "protagonist", "player", "persona":
		return "what this person remembers, fears, and intends without declaring it as narrator truth"
	case "npc", "character":
		return "private character recollection and possible misunderstanding, not direct exposition"
	default:
		return "scene participant reading"
	}
}

func tableReadPreview(text string, max int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if max <= 0 || len(text) <= max {
		return text
	}
	return strings.TrimSpace(text[:max]) + "..."
}

func tableReadPreviewPreserveLines(text string, max int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if max <= 0 || len(text) <= max {
		return text
	}
	return strings.TrimSpace(text[:max]) + "..."
}
