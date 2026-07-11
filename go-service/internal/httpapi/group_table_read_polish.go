package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

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
