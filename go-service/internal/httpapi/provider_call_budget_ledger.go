package httpapi

import "encoding/json"

const providerCallBudgetLedgerContractV1 = "provider_call_budget_ledger.v1"

type providerCallBudgetComponents struct {
	CurrentTurnChars                      int
	AuxiliaryMemoryChars                  int
	OriginalWorkReferenceChars            int
	OriginalWorkReferenceStatus           string
	LorebookReferenceChars                int
	LorebookReferenceStatus               string
	LanguageContextChars                  int
	JSONSchemaOutputRequirementChars      int
	JSONSchemaOutputRequirementAccounting string
	SupportPacketTextChars                int
	SupportPacketMetadataChars            int
	ExecutionInstructionChars             int
	ExecutionMetadataChars                int
}

func newProviderCallBudgetLedger(callKind, systemPrompt, userPrompt string, components providerCallBudgetComponents) map[string]any {
	systemPromptChars := len([]rune(systemPrompt))
	userPromptChars := len([]rune(userPrompt))
	finalPromptChars := systemPromptChars + userPromptChars
	accountedChars := systemPromptChars +
		components.CurrentTurnChars +
		components.AuxiliaryMemoryChars +
		components.OriginalWorkReferenceChars +
		components.LorebookReferenceChars +
		components.LanguageContextChars +
		components.JSONSchemaOutputRequirementChars
	assemblyChars := finalPromptChars - accountedChars
	if assemblyChars < 0 {
		assemblyChars = 0
	}
	return map[string]any{
		"contract_version":                          providerCallBudgetLedgerContractV1,
		"owner":                                     "go",
		"call_kind":                                 callKind,
		"accounting_unit":                           "unicode_code_points",
		"content_persisted":                         false,
		"base_prompt_chars":                         systemPromptChars,
		"base_prompt_accounting":                    "entire_system_prompt",
		"system_prompt_chars":                       systemPromptChars,
		"current_turn_chars":                        components.CurrentTurnChars,
		"auxiliary_memory_chars":                    components.AuxiliaryMemoryChars,
		"original_work_reference_chars":             components.OriginalWorkReferenceChars,
		"original_work_reference_status":            components.OriginalWorkReferenceStatus,
		"lorebook_reference_chars":                  components.LorebookReferenceChars,
		"lorebook_reference_status":                 components.LorebookReferenceStatus,
		"language_context_chars":                    components.LanguageContextChars,
		"json_schema_output_requirement_chars":      components.JSONSchemaOutputRequirementChars,
		"json_schema_output_requirement_accounting": components.JSONSchemaOutputRequirementAccounting,
		"support_packet_text_chars":                 components.SupportPacketTextChars,
		"support_packet_metadata_chars":             components.SupportPacketMetadataChars,
		"execution_instruction_chars":               components.ExecutionInstructionChars,
		"execution_metadata_chars":                  components.ExecutionMetadataChars,
		"assembly_chars":                            assemblyChars,
		"user_prompt_chars":                         userPromptChars,
		"final_prompt_chars":                        finalPromptChars,
		"provider_usage_status":                     "unreported",
		"status":                                    "prepared",
		"failure_stage":                             "",
	}
}

func providerCallJSONComponentChars(value any) int {
	if value == nil {
		return 0
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return 0
		}
	case []any:
		if len(typed) == 0 {
			return 0
		}
	case []map[string]any:
		if len(typed) == 0 {
			return 0
		}
	case map[string]any:
		if len(typed) == 0 {
			return 0
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return len([]rune(string(raw)))
}

func observeProviderCallBudgetResult(ledger map[string]any, providerResponse map[string]any, httpStatus int, status, failureStage string) {
	if ledger == nil {
		return
	}
	ledger["status"] = status
	ledger["failure_stage"] = failureStage
	if httpStatus > 0 {
		ledger["http_status"] = httpStatus
	}
	if len(providerResponse) == 0 {
		return
	}
	if terminationKind := extractionStringFromAny(providerResponse["termination_kind"]); terminationKind != "" {
		ledger["termination_kind"] = terminationKind
	}
	if finishReason := extractionStringFromAny(providerResponse["native_finish_reason"]); finishReason != "" {
		ledger["native_finish_reason"] = finishReason
	}
	if retryAfterSeconds := intFromAny(providerResponse["retry_after_seconds"], 0); retryAfterSeconds > 0 {
		ledger["retry_after_seconds"] = retryAfterSeconds
	}
	if !boolFromAny(providerResponse["usage_reported"]) {
		return
	}
	ledger["provider_usage_status"] = "reported"
	for _, key := range []string{
		"input_tokens",
		"output_tokens",
		"reasoning_tokens",
		"cached_input_tokens",
		"total_tokens",
	} {
		ledger[key] = intFromAny(providerResponse[key], 0)
	}
}

func safeProviderCallBudgetLedger(value any) map[string]any {
	ledger := mapFromAny(value)
	if ledger["contract_version"] != providerCallBudgetLedgerContractV1 || ledger["owner"] != "go" {
		return nil
	}
	safe := map[string]any{}
	for _, key := range []string{
		"contract_version", "owner", "call_kind", "accounting_unit", "content_persisted",
		"base_prompt_chars", "base_prompt_accounting",
		"system_prompt_chars", "current_turn_chars", "auxiliary_memory_chars",
		"original_work_reference_chars", "original_work_reference_status",
		"lorebook_reference_chars", "lorebook_reference_status", "language_context_chars",
		"json_schema_output_requirement_chars", "json_schema_output_requirement_accounting",
		"support_packet_text_chars", "support_packet_metadata_chars",
		"execution_instruction_chars", "execution_metadata_chars",
		"json_response_format", "json_response_source", "json_response_schema_contract", "json_response_schema_source",
		"assembly_chars", "user_prompt_chars", "final_prompt_chars", "provider_usage_status",
		"input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "total_tokens",
		"requested_max_tokens", "requested_max_completion_tokens",
		"status", "failure_stage", "failure_code", "http_status", "termination_kind",
		"native_finish_reason", "retry_after_seconds",
	} {
		if field, ok := ledger[key]; ok {
			safe[key] = field
		}
	}
	return safe
}
