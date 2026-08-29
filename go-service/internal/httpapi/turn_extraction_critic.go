package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

var (
	criticAuthorizationSecretPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer\s+)?[^\s,;}\]]+`)
	criticBearerSecretPattern        = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`)
	criticJSONSecretPattern          = regexp.MustCompile(`(?i)("(?:x-api-key|api[_-]?key|password|client_secret|access_token|refresh_token)"\s*:\s*)"[^"]*"`)
	criticKVSecretPattern            = regexp.MustCompile(`(?i)((?:x-api-key|api[_-]?key|password|client_secret|access_token|refresh_token)\s*[:=]\s*)[^\s,;}\]]+`)
)

type criticPipelineError struct {
	Code       string
	Stage      string
	Retryable  bool
	HTTPStatus int
	Cause      error
}

const completeTurnCriticInputBudgetObservationContract = "critic_input_budget_observation.v1"
const completeTurnCriticInputSnapshotContract = "critic_reprocessing_input.v1"
const criticOutputPolicyVersion = "critic_sparse_output.v1"

type completeTurnCriticInputPolicy struct {
	AuxiliaryMaxChars int    `json:"auxiliary_max_chars"`
	ConfiguredChars   int    `json:"configured_chars"`
	LedgerChars       int    `json:"ledger_chars"`
	Source            string `json:"source"`
}

type completeTurnCriticInputSnapshot struct {
	ContractVersion    string                        `json:"contract_version"`
	SourceRevision     string                        `json:"source_revision"`
	ChatSessionID      string                        `json:"chat_session_id"`
	TurnIndex          int                           `json:"turn_index"`
	UserInput          string                        `json:"user_input"`
	AssistantContent   string                        `json:"assistant_content"`
	ContextMessages    []map[string]any              `json:"context_messages"`
	ArchiveLedger      map[string]any                `json:"archive_ledger"`
	ActiveWorldRules   []map[string]any              `json:"active_world_rules"`
	LanguageContext    map[string]any                `json:"language_context"`
	InputPolicy        completeTurnCriticInputPolicy `json:"input_policy"`
	PipelineVersion    string                        `json:"pipeline_version"`
	SystemPromptSHA256 string                        `json:"system_prompt_sha256"`
}

type completeTurnCriticInputReplay struct {
	SourceRevision string
	SnapshotJSON   string
	SnapshotHash   string
	Required       bool
}

type completeTurnCriticAuxiliaryCandidate struct {
	Kind       string
	ID         string
	Order      int
	Relevance  float64
	Persistent bool
	TurnIndex  int
	Value      map[string]any
	Messages   []map[string]any
}

func (e *criticPipelineError) Error() string {
	if e == nil {
		return "critic pipeline failed"
	}
	if e.Cause == nil {
		return e.Code
	}
	return e.Code + ": " + e.Cause.Error()
}

func (e *criticPipelineError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newCriticPipelineError(code, stage string, retryable bool, httpStatus int, cause error) *criticPipelineError {
	return &criticPipelineError{
		Code:       strings.TrimSpace(code),
		Stage:      strings.TrimSpace(stage),
		Retryable:  retryable,
		HTTPStatus: httpStatus,
		Cause:      cause,
	}
}

func criticPipelineErrorDetails(err error) map[string]any {
	var pipelineErr *criticPipelineError
	if !errors.As(err, &pipelineErr) || pipelineErr == nil {
		return map[string]any{
			"code":      "CRITIC_UNKNOWN_FAILED",
			"stage":     "unknown",
			"retryable": true,
		}
	}
	out := map[string]any{
		"code":      pipelineErr.Code,
		"stage":     pipelineErr.Stage,
		"retryable": pipelineErr.Retryable,
	}
	if pipelineErr.HTTPStatus > 0 {
		out["http_status"] = pipelineErr.HTTPStatus
	}
	return out
}

func classifyCriticProviderError(err error, status int) *criticPipelineError {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return newCriticPipelineError("CRITIC_PROVIDER_TIMEOUT", "provider_call", true, status, err)
	case errors.Is(err, context.Canceled):
		return newCriticPipelineError("CRITIC_PROVIDER_CANCELED", "provider_call", false, status, err)
	}
	var exhaustedErr *proxyFinalOutputExhaustedError
	if errors.As(err, &exhaustedErr) {
		return newCriticPipelineError("CRITIC_OUTPUT_TOKEN_EXHAUSTED", "provider_response", true, status, err)
	}
	var emptyContentErr *proxyEmptyContentError
	if errors.As(err, &emptyContentErr) {
		return newCriticPipelineError("CRITIC_EMPTY_RESPONSE", "provider_response", true, status, err)
	}
	var localRequestErr *proxyLocalRequestError
	if errors.As(err, &localRequestErr) {
		stage := strings.TrimSpace(localRequestErr.Stage)
		code := "CRITIC_REQUEST_BUILD_FAILED"
		if stage == "configuration" {
			code = "CRITIC_CONFIG_INVALID"
		}
		if stage == "" {
			stage = "request_build"
		}
		return newCriticPipelineError(code, stage, false, 0, err)
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		if networkErr.Timeout() {
			return newCriticPipelineError("CRITIC_PROVIDER_TIMEOUT", "provider_call", true, status, err)
		}
		return newCriticPipelineError("CRITIC_PROVIDER_CALL_FAILED", "provider_call", true, status, err)
	}
	if status >= http.StatusBadRequest {
		retryable := status == http.StatusRequestTimeout ||
			status == http.StatusTooEarly ||
			status == http.StatusTooManyRequests ||
			status >= http.StatusInternalServerError
		return newCriticPipelineError("CRITIC_PROVIDER_HTTP_ERROR", "provider_response", retryable, status, err)
	}
	return newCriticPipelineError("CRITIC_PROVIDER_CALL_FAILED", "provider_call", true, status, err)
}

func criticFailureTrace(promptSource string, cfg completeTurnLLMConfig, status int, err error, content string) map[string]any {
	trace := map[string]any{
		"prompt_source": promptSource,
		"provider":      strings.TrimSpace(cfg.Provider),
		"model":         strings.TrimSpace(cfg.Model),
	}
	for key, value := range criticPipelineErrorDetails(err) {
		trace[key] = value
	}
	if status > 0 {
		trace["http_status"] = status
	}
	preview := strings.TrimSpace(scrubCriticFailureText(content, cfg.APIKey))
	if preview == "" && err != nil {
		preview = scrubCriticFailureText(err.Error(), cfg.APIKey)
	}
	if preview != "" {
		trace["raw_preview"] = truncateRunes(preview, 1000)
	}
	return trace
}

func scrubCriticFailureText(text, apiKey string) string {
	out := text
	if key := strings.TrimSpace(apiKey); key != "" {
		out = strings.ReplaceAll(out, key, "[redacted]")
	}
	out = criticAuthorizationSecretPattern.ReplaceAllString(out, `${1}[redacted]`)
	out = criticBearerSecretPattern.ReplaceAllString(out, "Bearer [redacted]")
	out = criticJSONSecretPattern.ReplaceAllString(out, `${1}"[redacted]"`)
	out = criticKVSecretPattern.ReplaceAllString(out, `${1}[redacted]`)
	return out
}

func (s *Server) completeTurnCriticInputPolicy(clientMeta map[string]any) completeTurnCriticInputPolicy {
	defaults := dto.PrepareTurnSettings{}
	defaults.ApplyDefaults()
	configuredChars := intPtrValue(defaults.MaxInputContextChars, 0)
	source := "prepare_turn_default"
	observation := mapFromAny(clientMeta["critic_input_budget_observation"])
	if stringFromMap(observation, "contract_version") == completeTurnCriticInputBudgetObservationContract {
		if observed, ok := observation["max_input_context_chars"]; ok {
			configuredChars = intFromAny(observed, configuredChars)
			source = "risu_host_setting_observation"
		}
	}
	if configuredChars < 0 {
		configuredChars = 0
	}
	ledgerChars := 0
	if s != nil && s.Cfg.CriticLedgerEnabled {
		ledgerChars = criticArchiveLedgerDefaultLimits(s.Cfg.RuntimeProfile).MaxCharsTotal
	}
	return completeTurnCriticInputPolicy{
		AuxiliaryMaxChars: configuredChars + ledgerChars,
		ConfiguredChars:   configuredChars,
		LedgerChars:       ledgerChars,
		Source:            source,
	}
}

func criticSystemPromptHash(prompt string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(prompt)))
}

func mapFromOptionalMap(value *map[string]any) map[string]any {
	if value == nil || *value == nil {
		return nil
	}
	return cloneMapAny(*value)
}

func (s *Server) persistCompleteTurnCriticInputSnapshot(
	ctx context.Context,
	snapshot completeTurnCriticInputSnapshot,
) (string, error) {
	if s == nil || s.Store == nil {
		return "", store.ErrNotEnabled
	}
	writer, ok := s.Store.(store.CriticInputSnapshotStore)
	if !ok {
		return "", store.ErrNotEnabled
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(encoded))
	if err := writer.SaveCriticInputSnapshot(
		ctx,
		snapshot.ChatSessionID,
		snapshot.SourceRevision,
		string(encoded),
		hash,
		time.Now().UTC(),
	); err != nil {
		return "", err
	}
	return hash, nil
}

func decodeCompleteTurnCriticInputSnapshot(
	replay completeTurnCriticInputReplay,
	sid string,
	turnIndex int,
	currentUserInput string,
	currentAssistantContent string,
) (*completeTurnCriticInputSnapshot, string, error) {
	raw := strings.TrimSpace(replay.SnapshotJSON)
	expectedHash := strings.ToLower(strings.TrimSpace(replay.SnapshotHash))
	if raw == "" || expectedHash == "" {
		return nil, "", errors.New("critic_input_snapshot_missing")
	}
	actualHash := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	if actualHash != expectedHash {
		return nil, "", errors.New("critic_input_snapshot_hash_mismatch")
	}
	var snapshot completeTurnCriticInputSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, "", fmt.Errorf("critic_input_snapshot_invalid_json: %w", err)
	}
	if snapshot.ContractVersion != completeTurnCriticInputSnapshotContract ||
		strings.TrimSpace(snapshot.SourceRevision) != strings.TrimSpace(replay.SourceRevision) ||
		strings.TrimSpace(snapshot.ChatSessionID) != strings.TrimSpace(sid) ||
		snapshot.TurnIndex != turnIndex {
		return nil, "", errors.New("critic_input_snapshot_identity_mismatch")
	}
	if snapshot.UserInput != currentUserInput || snapshot.AssistantContent != currentAssistantContent {
		return nil, "", errors.New("critic_input_snapshot_source_mismatch")
	}
	return &snapshot, actualHash, nil
}

func (s *Server) runCompleteTurnCritic(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, cfg completeTurnLLMConfig, languageContextArg ...map[string]any) (map[string]any, map[string]any, error) {
	return s.runCompleteTurnCriticWithInputPolicy(ctx, sid, turnIndex, userInput, assistantContent, contextMessages, outputLanguageOverride, cfg, false, s.completeTurnCriticInputPolicy(nil), completeTurnCriticInputReplay{}, languageContextArg...)
}

func (s *Server) runCompleteTurnCriticFromCanonicalLogs(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, cfg completeTurnLLMConfig) (map[string]any, map[string]any, error) {
	return s.runCompleteTurnCriticWithInputPolicy(ctx, sid, turnIndex, userInput, assistantContent, nil, nil, cfg, true, s.completeTurnCriticInputPolicy(nil), completeTurnCriticInputReplay{})
}

func completeTurnCriticLanguageContextFromAssistantOutput(raw map[string]any) map[string]any {
	languageContext := normalizeCompleteTurnLanguageContext(raw)
	if languageContext == nil {
		languageContext = map[string]any{
			"contract_version":       languageMemoryContractVersion,
			"search_text_policy":     languageMemorySearchPolicy,
			"raw_evidence_rewritten": false,
		}
	}

	observed := strings.ToLower(strings.TrimSpace(extractionStringFromAny(languageContext["assistant_output_language"])))
	effective := "auto"
	source := "assistant_output_unknown"
	confidence := float64(0)
	switch observed {
	case "ko", "en", "ja":
		effective = observed
		source = "current_assistant"
		confidence = 0.95
	case "":
		observed = "unknown"
	}

	languageContext["assistant_output_language"] = observed
	languageContext["session_output_language"] = effective
	languageContext["summary_language"] = effective
	languageContext["output_language_source"] = source
	languageContext["locked_for_turn"] = true
	languageContext["confidence"] = confidence
	return languageContext
}

func (s *Server) runCompleteTurnCriticWithInputPolicy(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, cfg completeTurnLLMConfig, canonicalChatLogs bool, inputPolicy completeTurnCriticInputPolicy, replay completeTurnCriticInputReplay, languageContextArg ...map[string]any) (map[string]any, map[string]any, error) {
	if !cfg.hasConfig() {
		err := newCriticPipelineError("CRITIC_CONFIG_MISSING", "configuration", false, 0, errors.New("critic_config_missing"))
		return nil, criticFailureTrace("", cfg, 0, err, ""), err
	}
	var languageContext map[string]any
	if len(languageContextArg) > 0 {
		languageContext = normalizeCompleteTurnLanguageContext(languageContextArg[0])
	}
	systemPrompt, promptSource := readCriticSystemPrompt(s.Cfg.PromptDir)
	sanitizedUserInput := ""
	sanitizedAssistantContent := ""
	if canonicalChatLogs {
		sanitizedUserInput = sanitizeCriticStorageText(userInput)
		sanitizedAssistantContent = sanitizeCriticStorageText(assistantContent)
	} else {
		sanitizedUserInput = sanitizeTextForCriticInput(userInput)
		sanitizedAssistantContent = sanitizeTextForCriticInput(assistantContent)
	}
	criticUserInput := boundCompleteTurnCriticInput(sanitizedUserInput, 0)
	criticAssistantContent := boundCompleteTurnCriticInput(sanitizedAssistantContent, 0)
	criticInputMode := "paired"
	criticUserInputState := "observed"
	if strings.TrimSpace(criticUserInput) == "" && strings.TrimSpace(criticAssistantContent) != "" {
		criticInputMode = "assistant_only"
		criticUserInputState = "missing"
	}
	if strings.TrimSpace(criticUserInput+"\n"+criticAssistantContent) == "" {
		err := newCriticPipelineError("CRITIC_INPUT_EMPTY", "input", false, 0, errors.New("critic_input_empty_after_sanitize"))
		trace := criticFailureTrace(promptSource, cfg, 0, err, "")
		trace["source_aware_ingest_guard"] = !canonicalChatLogs
		trace["canonical_chat_logs"] = canonicalChatLogs
		return nil, trace, err
	}
	criticContextMessages := []map[string]any{}
	criticArchiveLedgerPromptInput := map[string]any(nil)
	selectedActiveWorldRules := []map[string]any{}
	contextSelectionTrace := map[string]any{}
	criticArchiveLedgerTrace := map[string]any{}
	activeWorldRuleTrace := map[string]any{}
	snapshotTrace := map[string]any{"status": "not_required"}

	if replay.Required {
		snapshot, snapshotHash, err := decodeCompleteTurnCriticInputSnapshot(replay, sid, turnIndex, criticUserInput, criticAssistantContent)
		if err != nil {
			snapshotErr := newCriticPipelineError("CRITIC_INPUT_SNAPSHOT_INVALID", "input_snapshot", false, 0, err)
			trace := criticFailureTrace(promptSource, cfg, 0, snapshotErr, "")
			trace["input_snapshot"] = map[string]any{"status": "invalid", "source_revision": replay.SourceRevision}
			return nil, trace, snapshotErr
		}
		currentSystemPromptHash := criticSystemPromptHash(systemPrompt)
		if snapshot.PipelineVersion != completeTurnCriticPipelineVersion ||
			snapshot.SystemPromptSHA256 != currentSystemPromptHash {
			snapshotErr := newCriticPipelineError(
				"CRITIC_INPUT_SNAPSHOT_INVALID",
				"input_snapshot",
				false,
				0,
				errors.New("critic_input_snapshot_prompt_contract_mismatch"),
			)
			trace := criticFailureTrace(promptSource, cfg, 0, snapshotErr, "")
			trace["input_snapshot"] = map[string]any{
				"status":           "invalid",
				"source_revision":  replay.SourceRevision,
				"pipeline_version": snapshot.PipelineVersion,
			}
			return nil, trace, snapshotErr
		}
		criticUserInput = snapshot.UserInput
		criticAssistantContent = snapshot.AssistantContent
		criticContextMessages = snapshot.ContextMessages
		selectedActiveWorldRules = snapshot.ActiveWorldRules
		languageContext = completeTurnCriticLanguageContextFromAssistantOutput(snapshot.LanguageContext)
		criticArchiveLedgerPromptInput = cloneMapAny(snapshot.ArchiveLedger)
		if criticArchiveLedgerPromptInput != nil {
			ledgerLanguage := cloneMapAny(mapFromAny(criticArchiveLedgerPromptInput["language"]))
			if ledgerLanguage == nil {
				ledgerLanguage = map[string]any{}
			}
			ledgerLanguage["assistant_final_language"] = extractionStringFromAny(languageContext["session_output_language"])
			ledgerLanguage["source"] = "request_assistant_final_language"
			ledgerLanguage["override_applied"] = false
			criticArchiveLedgerPromptInput["language"] = ledgerLanguage
		}
		inputPolicy = snapshot.InputPolicy
		contextSelectionTrace = map[string]any{
			"mode":                  "durable_critic_input_snapshot",
			"snapshot_contract":     snapshot.ContractVersion,
			"context_message_count": len(criticContextMessages),
		}
		criticArchiveLedgerTrace = map[string]any{
			"status":              "snapshot_replay",
			"selected_item_count": len(sliceFromAny(mapFromAny(criticArchiveLedgerPromptInput)["items"])),
		}
		activeWorldRuleTrace = map[string]any{
			"status":         "snapshot_replay",
			"selected_count": len(selectedActiveWorldRules),
		}
		snapshotTrace = map[string]any{
			"status":            "replayed",
			"contract_version":  snapshot.ContractVersion,
			"source_revision":   snapshot.SourceRevision,
			"snapshot_hash":     snapshotHash,
			"prompt_hash_match": true,
		}
	} else {
		languageContext = completeTurnCriticLanguageContextFromAssistantOutput(languageContext)
		criticContextMessages = sanitizeContextMessagesForCriticInput(contextMessages)
		contextSelectionTrace = map[string]any{"mode": "host_context", "host_messages_used": len(criticContextMessages)}
		relevantMemoryContext := []map[string]any{}
		if canonicalChatLogs {
			criticContextMessages, relevantMemoryContext, contextSelectionTrace = s.buildCompleteTurnCriticCanonicalContext(ctx, sid, turnIndex, criticUserInput+"\n"+criticAssistantContent, len(criticContextMessages))
		}
		criticArchiveLedgerPromptInput, criticArchiveLedgerTrace = s.buildCompleteTurnCriticArchiveLedgerInput(ctx, sid, turnIndex, criticAssistantContent, extractionStringFromAny(languageContext["session_output_language"]))
		activeWorldRules, activeTrace := s.buildCompleteTurnActiveWorldRuleInput(ctx, sid)
		activeWorldRuleTrace = activeTrace
		selectedActiveWorldRules = activeWorldRules
		if canonicalChatLogs {
			var auxiliaryTrace map[string]any
			criticContextMessages, criticArchiveLedgerPromptInput, auxiliaryTrace = applyCompleteTurnCriticAuxiliaryBudget(
				criticContextMessages,
				relevantMemoryContext,
				criticArchiveLedgerPromptInput,
				activeWorldRules,
				criticUserInput+"\n"+criticAssistantContent,
				inputPolicy,
			)
			contextSelectionTrace["auxiliary_input"] = auxiliaryTrace
			criticArchiveLedgerTrace["selected_item_count"] = len(sliceFromAny(mapFromAny(criticArchiveLedgerPromptInput)["items"]))
			selectedActiveWorldRules = []map[string]any{}
			for _, raw := range sliceFromAny(mapFromAny(criticArchiveLedgerPromptInput)["active_world_rules"]) {
				if item := mapFromAny(raw); len(item) > 0 {
					selectedActiveWorldRules = append(selectedActiveWorldRules, item)
				}
			}
			activeWorldRuleTrace["selected_count"] = len(selectedActiveWorldRules)
		} else {
			if len(relevantMemoryContext) > 0 {
				if criticArchiveLedgerPromptInput == nil {
					criticArchiveLedgerPromptInput = map[string]any{}
				}
				criticArchiveLedgerPromptInput["relevant_turn_memories"] = relevantMemoryContext
			}
			if len(activeWorldRules) > 0 {
				if criticArchiveLedgerPromptInput == nil {
					criticArchiveLedgerPromptInput = map[string]any{}
				}
				criticArchiveLedgerPromptInput["active_world_rules"] = activeWorldRules
			}
		}
		if strings.TrimSpace(replay.SourceRevision) != "" {
			snapshotHash, err := s.persistCompleteTurnCriticInputSnapshot(ctx, completeTurnCriticInputSnapshot{
				ContractVersion:    completeTurnCriticInputSnapshotContract,
				SourceRevision:     strings.TrimSpace(replay.SourceRevision),
				ChatSessionID:      strings.TrimSpace(sid),
				TurnIndex:          turnIndex,
				UserInput:          criticUserInput,
				AssistantContent:   criticAssistantContent,
				ContextMessages:    criticContextMessages,
				ArchiveLedger:      criticArchiveLedgerPromptInput,
				ActiveWorldRules:   selectedActiveWorldRules,
				LanguageContext:    languageContext,
				InputPolicy:        inputPolicy,
				PipelineVersion:    completeTurnCriticPipelineVersion,
				SystemPromptSHA256: criticSystemPromptHash(systemPrompt),
			})
			if err != nil {
				snapshotTrace = map[string]any{
					"status":          "persist_failed",
					"source_revision": replay.SourceRevision,
					"reason":          "critic_input_snapshot_persist_failed",
				}
			} else {
				snapshotTrace = map[string]any{
					"status":           "persisted",
					"contract_version": completeTurnCriticInputSnapshotContract,
					"source_revision":  replay.SourceRevision,
					"snapshot_hash":    snapshotHash,
				}
			}
		}
	}
	userPrompt := buildCompleteTurnCriticPromptWithLanguageContext(sid, turnIndex, criticUserInput, criticAssistantContent, criticContextMessages, outputLanguageOverride, languageContext, criticArchiveLedgerPromptInput)
	contextMessagesJSON, _ := json.Marshal(criticContextMessages)
	archiveLedgerJSON, _ := json.Marshal(criticArchiveLedgerPromptInput)
	languageContextJSON, _ := json.Marshal(normalizeCompleteTurnLanguageContext(languageContext))
	inputBudgetTrace := map[string]any{
		"contract_version":             completeTurnCriticInputBudgetObservationContract,
		"input_mode":                   criticInputMode,
		"user_input_state":             criticUserInputState,
		"user_input_chars":             len([]rune(criticUserInput)),
		"assistant_content_chars":      len([]rune(criticAssistantContent)),
		"current_turn_chars":           len([]rune(criticUserInput)) + len([]rune(criticAssistantContent)),
		"current_turn_bounded":         false,
		"current_turn_content_changed": false,
		"context_messages_chars":       len([]rune(string(contextMessagesJSON))),
		"archive_ledger_chars":         len([]rune(string(archiveLedgerJSON))),
		"system_prompt_chars":          len([]rune(systemPrompt)),
		"user_prompt_chars":            len([]rune(userPrompt)),
		"final_prompt_chars":           len([]rune(systemPrompt)) + len([]rune(userPrompt)),
	}
	callLedger := newProviderCallBudgetLedger("critic", systemPrompt, userPrompt, providerCallBudgetComponents{
		CurrentTurnChars:                      len([]rune(criticUserInput)) + len([]rune(criticAssistantContent)),
		AuxiliaryMemoryChars:                  providerCallJSONComponentChars(criticContextMessages) + providerCallJSONComponentChars(criticArchiveLedgerPromptInput),
		OriginalWorkReferenceStatus:           "not_in_call_contract",
		LorebookReferenceStatus:               "not_in_call_contract",
		LanguageContextChars:                  len([]rune(string(languageContextJSON))),
		JSONSchemaOutputRequirementAccounting: "embedded_in_system_prompt_not_separable",
	})
	providerResponse := map[string]any{}
	outputObservation := map[string]any{}
	attachInputBudgetTrace := func(trace map[string]any) map[string]any {
		if trace == nil {
			trace = map[string]any{}
		}
		trace["input_budget"] = inputBudgetTrace
		trace["provider_call_budget_ledger"] = callLedger
		trace["context_selection"] = contextSelectionTrace
		trace["critic_archive_ledger"] = criticArchiveLedgerTrace
		trace["active_world_rule_contract"] = activeWorldRuleTrace
		trace["input_snapshot"] = snapshotTrace
		if len(providerResponse) > 0 {
			trace["provider_response"] = providerResponse
		}
		if len(outputObservation) > 0 {
			trace["output_observation"] = outputObservation
		}
		return trace
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = completeTurnExtractionConfigFromMeta(nil).Critic.MaxTokens
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	callLedger["requested_max_tokens"] = maxTokens
	callLedger["requested_max_completion_tokens"] = maxCompletionTokens
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		req.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		req.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		req.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		req.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		req.GlmThinkingType = &cfg.GlmThinkingType
	}
	applyProxyOverridesFromLLMConfig(&req, cfg)
	jsonPolicy := proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"}

	upstream, upstreamStatus, err := performProxyPluginMainWithRetryBudgetAndPolicy(ctx, req, nil, jsonPolicy)
	providerResponse = mapFromAny(upstream[proxyResponseMetadataKey])
	if err != nil {
		providerErr := classifyCriticProviderError(err, upstreamStatus)
		observeProviderCallBudgetResult(callLedger, providerResponse, upstreamStatus, "failed", providerErr.Stage)
		callLedger["failure_code"] = providerErr.Code
		firstFailureTrace := criticFailureTrace(promptSource, cfg, upstreamStatus, providerErr, "")
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			firstFailureTrace["request_overrides"] = requestOverrides
		}
		return nil, attachInputBudgetTrace(firstFailureTrace), providerErr
	}
	content := chatCompletionText(upstream)
	outputObservation = map[string]any{
		"contract_version": "critic_output_observation.v1",
		"response_bytes":   len([]byte(content)),
		"response_chars":   len([]rune(content)),
	}
	if strings.TrimSpace(content) == "" {
		emptyErr := newCriticPipelineError("CRITIC_EMPTY_RESPONSE", "provider_response", true, upstreamStatus, errors.New("critic provider returned no assistant content"))
		observeProviderCallBudgetResult(callLedger, providerResponse, upstreamStatus, "failed", "provider_response")
		callLedger["failure_code"] = emptyErr.Code
		trace := criticFailureTrace(promptSource, cfg, upstreamStatus, emptyErr, "")
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			trace["request_overrides"] = requestOverrides
		}
		return nil, attachInputBudgetTrace(trace), emptyErr
	}
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		code := "CRITIC_JSON_PARSE_FAILED"
		if strings.Contains(err.Error(), "critic_json_missing") {
			code = "CRITIC_JSON_MISSING"
		} else if strings.Contains(err.Error(), "critic_json_incomplete") && stringFromMap(providerResponse, "termination_kind") == "length" {
			code = "CRITIC_JSON_TRUNCATED"
		}
		parseErr := newCriticPipelineError(code, "json_parse", true, upstreamStatus, err)
		observeProviderCallBudgetResult(callLedger, providerResponse, upstreamStatus, "failed", "json_parse")
		callLedger["failure_code"] = code
		parseTrace := criticFailureTrace(promptSource, cfg, upstreamStatus, parseErr, content)
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			parseTrace["request_overrides"] = requestOverrides
		}
		return nil, attachInputBudgetTrace(parseTrace), parseErr
	}
	wireFieldCount := len(parsed)
	wireItemCount := 0
	for _, value := range parsed {
		switch typed := value.(type) {
		case []any:
			wireItemCount += len(typed)
		case map[string]any:
			wireItemCount++
		}
	}
	outputObservation["wire_field_count"] = wireFieldCount
	outputObservation["wire_item_count"] = wireItemCount
	parsed, schemaQuarantineTrace, err := validateCriticExtractionSchema(parsed)
	if err != nil {
		schemaErr := newCriticPipelineError("CRITIC_SCHEMA_INVALID", "schema_validation", true, upstreamStatus, err)
		observeProviderCallBudgetResult(callLedger, providerResponse, upstreamStatus, "failed", "schema_validation")
		callLedger["failure_code"] = schemaErr.Code
		schemaTrace := criticFailureTrace(promptSource, cfg, upstreamStatus, schemaErr, content)
		if len(schemaQuarantineTrace) > 0 {
			schemaTrace["schema_quarantine"] = schemaQuarantineTrace
		}
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			schemaTrace["request_overrides"] = requestOverrides
		}
		return nil, attachInputBudgetTrace(schemaTrace), schemaErr
	}
	outputObservation["canonical_field_count"] = len(parsed)
	if len(schemaQuarantineTrace) > 0 {
		outputObservation["quarantined_field_count"] = intFromAny(schemaQuarantineTrace["dropped_field_count"], 0)
		outputObservation["quarantined_item_count"] = intFromAny(schemaQuarantineTrace["dropped_item_count"], 0)
	} else {
		outputObservation["quarantined_field_count"] = 0
		outputObservation["quarantined_item_count"] = 0
	}
	parsed, quarantineTrace := quarantineCriticProtectedCandidates(parsed, criticUserInput, criticAssistantContent)
	trustedRPIdentities := s.resolveTrustedRPCharacterIdentities(ctx, sid, parsed)
	parsed, interactionAdmissionTrace := admitCriticInteractionLanesWithTrustedIdentities(parsed, criticUserInput, criticAssistantContent, trustedRPIdentities)
	observeProviderCallBudgetResult(callLedger, providerResponse, upstreamStatus, "succeeded", "")
	trace := map[string]any{
		"prompt_source":               promptSource,
		"model":                       extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"provider":                    strings.TrimSpace(cfg.Provider),
		"http_status":                 upstreamStatus,
		"usage":                       upstream["usage"],
		"input_budget":                inputBudgetTrace,
		"provider_call_budget_ledger": callLedger,
		"pipeline": map[string]any{
			"policy_version": completeTurnCriticPipelineVersion,
			"stages": map[string]any{
				"evidence_extractor": map[string]any{
					"status": "ok",
					"owner":  "complete_turn.configured_critic_extract",
				},
				"deterministic_reducer": map[string]any{
					"status": "ok",
					"owner":  "complete_turn.normalizeCriticExtraction",
				},
				"focused_recall_enricher": map[string]any{
					"status": "ok",
					"owner":  "complete_turn.enrichNormalizedCriticExtractionForFocusedRecall",
				},
				"summary_compactor_background": map[string]any{
					"status": "handoff",
					"owner":  "complete_turn.maintenance_handoff",
				},
			},
		},
	}
	if len(providerResponse) > 0 {
		trace["provider_response"] = providerResponse
	}
	trace["output_observation"] = outputObservation
	if len(schemaQuarantineTrace) > 0 {
		trace["schema_quarantine"] = schemaQuarantineTrace
	}
	if len(quarantineTrace) > 0 {
		trace["protected_candidate_quarantine"] = quarantineTrace
	}
	if len(interactionAdmissionTrace) > 0 {
		trace["interaction_admission"] = interactionAdmissionTrace
	}
	if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
		trace["request_overrides"] = requestOverrides
	}
	trace["critic_archive_ledger"] = criticArchiveLedgerTrace
	trace["input_mode"] = criticInputMode
	trace["user_input_state"] = criticUserInputState
	trace["context_selection"] = contextSelectionTrace
	trace["active_world_rule_contract"] = activeWorldRuleTrace
	trace["input_snapshot"] = snapshotTrace
	if len(languageContext) > 0 {
		trace["language_context"] = languageContext
		trace["memory_write_contract"] = completeTurnMemoryWriteContract(languageContext)
	}
	normalized := normalizeCriticExtraction(parsed)
	normalized["input_mode"] = criticInputMode
	normalized["user_input_state"] = criticUserInputState
	worldRuleCount := len(worldRuleItemsForSave(normalized))
	if worldRuleCount > 0 {
		trace["world_rule_audit"] = map[string]any{
			"status":           "ok",
			"reason":           "single_critic_call_extracted_world_rules",
			"llm_call_attempt": false,
			"world_rule_count": worldRuleCount,
		}
	} else if cfg.ForceWorldRuleAudit || shouldRunFocusedWorldRuleAudit(normalized) {
		trace["world_rule_audit"] = map[string]any{
			"status":           "incomplete",
			"reason":           "single_critic_call_returned_no_world_rules",
			"llm_call_attempt": false,
			"world_rule_count": 0,
		}
	} else {
		trace["world_rule_audit"] = map[string]any{
			"status":           "skipped",
			"reason":           "single_critic_call_found_no_durable_world_rule",
			"llm_call_attempt": false,
			"world_rule_count": 0,
		}
	}
	normalized = enrichNormalizedCriticExtractionForFocusedRecall(normalized, criticUserInput, criticAssistantContent, turnIndex)
	normalized = applyLanguageMemoryWriteContract(normalized, languageContext)
	return normalized, trace, nil
}

func (s *Server) resolveTrustedRPCharacterIdentities(ctx context.Context, sid string, extraction map[string]any) map[string]*interactionStableCharacterIdentity {
	resolver, ok := s.Store.(store.UniqueActiveEntitySurfaceIdentityResolver)
	if !ok {
		return nil
	}
	resolved := map[string]*interactionStableCharacterIdentity{}
	for _, raw := range sliceFromAny(extraction["rp_character_profile"]) {
		profile := mapFromAny(raw)
		character := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(profile, "character"), stringFromMap(profile, "entity"), stringFromMap(profile, "name")))
		proof := mapFromAny(profile["identity_proof"])
		stableEntityID := strings.TrimSpace(stringFromMap(proof, "stable_entity_id"))
		namespace := strings.ToLower(strings.TrimSpace(stringFromMap(proof, "identity_namespace")))
		if character == "" || stableEntityID == "" || stringFromMap(proof, "contract_version") != inWorldIdentityProofContract ||
			(namespace != "session_npc" && namespace != "session_player") {
			continue
		}
		identity, err := resolver.ResolveUniqueActiveEntityIdentityBySurface(ctx, sid, comparableEntityKey(character))
		if err != nil || strings.TrimSpace(identity.StableEntityID) != stableEntityID ||
			strings.TrimSpace(identity.IdentityNamespace) != namespace {
			continue
		}
		resolved[comparableEntityKey(character)] = &interactionStableCharacterIdentity{
			stableEntityID: identity.StableEntityID,
			namespace:      identity.IdentityNamespace,
		}
	}
	return resolved
}

func shouldRunFocusedWorldRuleAudit(extraction map[string]any) bool {
	audit := mapFromAny(extraction["world_rule_audit"])
	if len(audit) == 0 {
		audit = mapFromAny(extraction["world_rules_audit"])
	}
	if len(audit) == 0 {
		return false
	}
	for _, key := range []string{"durable_rule_found", "rule_found", "needs_world_rule", "audit_positive"} {
		if boolFromAny(audit[key]) {
			return true
		}
	}
	status := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(audit, "status"),
		stringFromMap(audit, "verdict"),
		stringFromMap(audit, "decision"),
	)))
	return status == "positive" || status == "found" || status == "needs_world_rule"
}

func (s *Server) runCompleteTurnWorldRuleAudit(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, initialExtraction map[string]any, cfg completeTurnLLMConfig, selectedActiveWorldRuleInput ...[]map[string]any) (map[string]any, map[string]any) {
	trace := map[string]any{
		"status":           "skipped",
		"policy_version":   "world_rule_audit.v1",
		"llm_call_attempt": false,
	}
	if !cfg.hasConfig() {
		trace["reason"] = "critic_config_missing"
		return nil, trace
	}
	if strings.TrimSpace(userInput+"\n"+assistantContent) == "" {
		trace["reason"] = "empty_turn"
		return nil, trace
	}
	activeWorldRules := []map[string]any{}
	activeWorldRuleTrace := map[string]any{}
	if len(selectedActiveWorldRuleInput) > 0 {
		activeWorldRules = selectedActiveWorldRuleInput[0]
		activeWorldRuleTrace = map[string]any{
			"status":         "selected_primary_critic_input",
			"included_count": len(activeWorldRules),
		}
	} else {
		activeWorldRules, activeWorldRuleTrace = s.buildCompleteTurnActiveWorldRuleInput(ctx, sid)
	}
	trace["active_world_rule_contract"] = activeWorldRuleTrace
	prompt := buildCompleteTurnWorldRuleAuditPrompt(sid, turnIndex, userInput, assistantContent, contextMessages, initialExtraction, activeWorldRules)
	maxTokens := cfg.MaxTokens
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": "You are Archive Center's world-rule audit extractor. Return only one sparse JSON object. Do not use markdown fences."}, map[string]any{"role": "user", "content": prompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		req.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		req.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		req.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		req.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		req.GlmThinkingType = &cfg.GlmThinkingType
	}
	applyProxyOverridesFromLLMConfig(&req, cfg)
	trace["llm_call_attempt"] = true
	upstream, _, err := performProxyPluginMainWithRetryBudgetAndPolicy(ctx, req, nil, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_world_rule_audit"})
	if err != nil {
		trace["status"] = "error"
		trace["error"] = err.Error()
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			trace["request_overrides"] = requestOverrides
		}
		return nil, trace
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		trace["status"] = "error"
		trace["error"] = err.Error()
		trace["raw_preview"] = truncateRunes(content, 1000)
		return nil, trace
	}
	parsed, schemaTrace, err := validateCriticExtractionSchema(parsed)
	if err != nil {
		trace["status"] = "error"
		trace["error"] = err.Error()
		if len(schemaTrace) > 0 {
			trace["schema_quarantine"] = schemaTrace
		}
		return nil, trace
	}
	if len(schemaTrace) > 0 {
		trace["schema_quarantine"] = schemaTrace
	}
	normalized := normalizeCriticExtraction(parsed)
	count := len(worldRuleItemsForSave(normalized))
	trace["status"] = "ok"
	trace["model"] = extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model)
	trace["usage"] = upstream["usage"]
	if providerResponse := mapFromAny(upstream[proxyResponseMetadataKey]); len(providerResponse) > 0 {
		trace["provider_response"] = providerResponse
	}
	if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
		trace["request_overrides"] = requestOverrides
	}
	trace["world_rule_count"] = count
	if count == 0 {
		trace["reason"] = extractionFirstNonEmpty(stringFromMap(mapFromAny(parsed["world_rule_audit"]), "reason"), "audit_returned_no_durable_rule")
	}
	return normalized, trace
}

func buildCompleteTurnWorldRuleAuditPrompt(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, initialExtraction map[string]any, activeWorldRuleInput ...[]map[string]any) string {
	ctx, _ := json.Marshal(contextMessages)
	initial, _ := json.Marshal(initialExtraction)
	var activeRules any
	if len(activeWorldRuleInput) > 0 {
		activeRules = activeWorldRuleInput[0]
	}
	active, _ := json.Marshal(activeRules)
	return strings.Join([]string{
		"Audit whether the completed turn establishes durable world rules that the main extraction missed.",
		"Return ONLY one sparse JSON object. Do not use markdown fences.",
		"Use this JSON shape:",
		`{"turn_summary":"world-rule audit","importance_score":5,"world_rule_audit":{"durable_rule_found":false,"reason":""},"world_rules":[{"key":"","value":""}],"world_state":{"version":"world_state.v1","confidence":0,"verification":"","rules":[]}}`,
		"Decision contract:",
		"- This is an AI judgement step. Do not rely on keyword lists, genre names, or instruction examples as facts.",
		"- Extract the abstract invariant established by the session's own evidence.",
		"- A world rule is any source-grounded constraint or invariant that should remain true beyond this exchange. Judge durability from the story evidence rather than a fixed category or genre list.",
		"- If the latest turn only has a temporary action, mood, one-off dialogue, rejected plan, speculation, or private thought with no durable setting constraint, omit world_rules.",
		"- If the latest turn confirms a durable rule, emit at least one world_rules item. Preserve the rule with whatever descriptive fields the evidence supports; key and value are sufficient for collection.",
		"- Active_World_Rules_JSON contains the current unsuppressed stored rules. For a changed or explicitly reaffirmed existing rule, reuse its exact scope, scope_name, category, and key even when the output language differs. Never translate an existing key into a new key. If the latest turn supplies no new evidence or change for an existing rule, omit that unchanged repeat.",
		"- When scope or category is useful, describe the story's own structure. Do not discard a rule because its scope or category is unfamiliar.",
		"- Mirror the same durable rules in world_state.rules when they shape the current setting state.",
		"- Do not invent mechanics. If uncertain, use audit.reason and return empty arrays.",
		"",
		fmt.Sprintf("chat_session_id: %s", sid),
		fmt.Sprintf("turn_index: %d", turnIndex),
		"",
		"<Latest_Turn>",
		"[User]",
		userInput,
		"",
		"[Assistant]",
		assistantContent,
		"</Latest_Turn>",
		"",
		"<Recent_Context_JSON>",
		string(ctx),
		"</Recent_Context_JSON>",
		"",
		"<Initial_Critic_Extraction_JSON>",
		string(initial),
		"</Initial_Critic_Extraction_JSON>",
		"",
		"<Active_World_Rules_JSON>",
		string(active),
		"</Active_World_Rules_JSON>",
	}, "\n")
}

func (s *Server) buildCompleteTurnCriticArchiveLedgerInput(ctx context.Context, sid string, turnIndex int, assistantContent string, assistantFinalLanguage string) (map[string]any, map[string]any) {
	trace := map[string]any{
		"enabled":          s != nil && s.Cfg.CriticLedgerEnabled,
		"included":         false,
		"contract_version": criticArchiveLedgerContractVersion,
	}
	if s == nil || !s.Cfg.CriticLedgerEnabled {
		trace["status"] = "disabled"
		return nil, trace
	}
	req := criticArchiveLedgerPreviewRequest{
		ChatSessionID:          sid,
		TurnIndex:              turnIndex,
		AssistantFinalText:     assistantContent,
		AssistantFinalLanguage: strings.TrimSpace(assistantFinalLanguage),
		StreamingMismatch:      "unknown",
	}
	resp := s.buildCriticArchiveLedgerPreviewWithContext(ctx, req)
	promptInput := criticArchiveLedgerPromptInput(resp)
	trace["included"] = true
	trace["status"] = resp.Status
	trace["item_count"] = len(resp.Items)
	trace["vector_status"] = resp.VectorStatus
	trace["language"] = resp.Language
	trace["degraded"] = resp.Degraded
	trace["warnings"] = resp.Warnings
	trace["write_attempted"] = resp.WriteAttempted
	trace["vector_write_attempted"] = resp.VectorWriteAttempted
	trace["llm_call_attempted"] = resp.LLMCallAttempted
	return promptInput, trace
}

func (s *Server) buildCompleteTurnCriticCanonicalContext(ctx context.Context, sid string, turnIndex int, query string, hostMessageCount int) ([]map[string]any, []map[string]any, map[string]any) {
	contextMessages := []map[string]any{}
	relevantMemories := []map[string]any{}
	warnings := []string{}
	selectedTurns := map[int]bool{}
	previousTurnChars := 0
	readPair := func(sourceTurn int, kind string) ([]map[string]any, bool) {
		rows, err := s.Store.ListChatLogs(ctx, sid, sourceTurn, sourceTurn)
		if err != nil {
			warnings = append(warnings, err.Error())
			return nil, false
		}
		userText := ""
		assistantText := ""
		for _, row := range rows {
			if row.TurnIndex != sourceTurn || (row.ChatSessionID != "" && row.ChatSessionID != sid) {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(row.Role)) {
			case "user":
				if userText == "" {
					userText = sanitizeCriticStorageText(row.Content)
				}
			case "assistant":
				if assistantText == "" {
					assistantText = sanitizeCriticStorageText(row.Content)
				}
			}
		}
		if userText == "" || assistantText == "" {
			return nil, false
		}
		return []map[string]any{
			{"role": "user", "content": userText, "turn_index": sourceTurn, "source": kind, "support_only": true},
			{"role": "assistant", "content": assistantText, "turn_index": sourceTurn, "source": kind, "support_only": true},
		}, true
	}

	previousTurn := turnIndex - 1
	if previousTurn > 0 {
		if pair, ok := readPair(previousTurn, "previous_canonical_turn"); ok {
			contextMessages = append(contextMessages, pair...)
			selectedTurns[previousTurn] = true
			for _, message := range pair {
				previousTurnChars += len([]rune(stringFromMap(message, "content")))
				query = strings.TrimSpace(query + "\n" + stringFromMap(message, "content"))
			}
		}
	}
	rows, err := s.Store.ListMemories(ctx, sid, 0, maxInt(turnIndex-1, 0))
	selectionTrace := map[string]any{}
	if err != nil {
		warnings = append(warnings, err.Error())
	} else {
		eligible := make([]store.Memory, 0, len(rows))
		for _, row := range rows {
			if row.TurnIndex > 0 && row.TurnIndex < turnIndex {
				eligible = append(eligible, row)
			}
		}
		eligible, projectionTrace := projectPrepareTurnGeneralMemories(eligible)
		selection := selectPrepareTurnMemoryLanes(eligible, query, len(eligible))
		selectionTrace = selection.Trace
		selectionTrace["public_projection"] = projectionTrace
		for _, memory := range selection.Relevant {
			if !selectedTurns[memory.TurnIndex] {
				pair, ok := readPair(memory.TurnIndex, "relevant_memory_source_turn")
				if !ok {
					continue
				}
				contextMessages = append(contextMessages, pair...)
				selectedTurns[memory.TurnIndex] = true
			}
			relevantMemories = append(relevantMemories, map[string]any{
				"source": "mariadb_memory", "id": memory.ID, "turn_index": memory.TurnIndex,
				"summary": prepareTurnMemorySummary(memory), "support_only": true,
			})
		}
	}
	trace := map[string]any{
		"mode":                    "canonical_previous_plus_relevant_memory_sources",
		"host_messages_received":  hostMessageCount,
		"host_messages_used":      0,
		"previous_turn":           previousTurn,
		"previous_turn_chars":     previousTurnChars,
		"query_chars":             len([]rune(query)),
		"query_includes_previous": previousTurnChars > 0,
		"context_message_count":   len(contextMessages),
		"relevant_memory_count":   len(relevantMemories),
		"memory_selection":        selectionTrace,
		"warnings":                warnings,
	}
	return contextMessages, relevantMemories, trace
}

func applyCompleteTurnCriticAuxiliaryBudget(
	contextMessages []map[string]any,
	relevantMemories []map[string]any,
	archiveLedger map[string]any,
	activeWorldRules []map[string]any,
	query string,
	policy completeTurnCriticInputPolicy,
) ([]map[string]any, map[string]any, map[string]any) {
	mandatoryContext := []map[string]any{}
	sourcePairs := map[int][]map[string]any{}
	selectionQuery := strings.TrimSpace(query)
	for _, message := range contextMessages {
		source := stringFromMap(message, "source")
		if source == "previous_canonical_turn" {
			mandatoryContext = append(mandatoryContext, message)
			selectionQuery = strings.TrimSpace(selectionQuery + "\n" + stringFromMap(message, "content"))
			continue
		}
		if source == "relevant_memory_source_turn" {
			turn := intFromAny(message["turn_index"], 0)
			if turn > 0 {
				sourcePairs[turn] = append(sourcePairs[turn], message)
			}
		}
	}

	ledgerBase := cloneMapAny(archiveLedger)
	if ledgerBase == nil {
		ledgerBase = map[string]any{}
	}
	ledgerItems := []map[string]any{}
	if typedItems, ok := ledgerBase["items"].([]map[string]any); ok {
		ledgerItems = append(ledgerItems, typedItems...)
	} else {
		for _, raw := range sliceFromAny(ledgerBase["items"]) {
			if item := mapFromAny(raw); len(item) > 0 {
				ledgerItems = append(ledgerItems, item)
			}
		}
	}
	delete(ledgerBase, "items")
	delete(ledgerBase, "relevant_turn_memories")
	delete(ledgerBase, "active_world_rules")

	candidates := []completeTurnCriticAuxiliaryCandidate{}
	excluded := []map[string]any{}
	order := 0
	pairAdded := map[int]bool{}
	for _, memory := range relevantMemories {
		turn := intFromAny(memory["turn_index"], 0)
		summary := stringFromMap(memory, "summary")
		relevance := simpleTokenSimilarity(selectionQuery, summary)
		memoryID := fmt.Sprint(memory["id"])
		candidates = append(candidates, completeTurnCriticAuxiliaryCandidate{
			Kind: "relevant_memory", ID: memoryID, Order: order,
			Relevance: relevance, TurnIndex: turn, Value: memory,
		})
		order++
		if !pairAdded[turn] && len(sourcePairs[turn]) > 0 {
			candidates = append(candidates, completeTurnCriticAuxiliaryCandidate{
				Kind: "relevant_memory_source_turn", ID: fmt.Sprint(turn), Order: order,
				Relevance: relevance, TurnIndex: turn, Messages: sourcePairs[turn],
			})
			order++
			pairAdded[turn] = true
		}
	}
	for _, item := range ledgerItems {
		summary := stringFromMap(item, "summary")
		lane := stringFromMap(item, "lane")
		id := extractionFirstNonEmpty(stringFromMap(item, "id"), fmt.Sprint(order))
		related := prepareTurnRequestFirstRelevant(selectionQuery, selectionQuery, summary, lane)
		if !related {
			excluded = append(excluded, map[string]any{
				"kind": "critic_archive_ledger", "id": id, "reason": "not_related_to_current_or_previous_turn",
			})
			continue
		}
		candidates = append(candidates, completeTurnCriticAuxiliaryCandidate{
			Kind: "critic_archive_ledger", ID: id, Order: order,
			Relevance: simpleTokenSimilarity(selectionQuery, summary), Value: item,
		})
		order++
	}
	for _, rule := range activeWorldRules {
		scope := strings.ToLower(strings.TrimSpace(stringFromMap(rule, "scope")))
		persistent := scope == "root" || scope == "global"
		encoded, _ := json.Marshal(rule)
		text := string(encoded)
		id := extractionFirstNonEmpty(stringFromMap(rule, "key"), fmt.Sprint(order))
		related := persistent || prepareTurnRequestFirstRelevant(
			selectionQuery, selectionQuery, text, stringFromMap(rule, "scope_name"), stringFromMap(rule, "key"),
		)
		if !related {
			excluded = append(excluded, map[string]any{
				"kind": "active_world_rule", "id": id, "reason": "not_related_to_current_or_previous_turn",
			})
			continue
		}
		candidates = append(candidates, completeTurnCriticAuxiliaryCandidate{
			Kind: "active_world_rule", ID: id, Order: order,
			Relevance: simpleTokenSimilarity(selectionQuery, text), Persistent: persistent, Value: rule,
		})
		order++
	}

	slices.SortStableFunc(candidates, func(a, b completeTurnCriticAuxiliaryCandidate) int {
		aRelevant := a.Relevance > 0
		bRelevant := b.Relevance > 0
		if aRelevant != bRelevant {
			if aRelevant {
				return -1
			}
			return 1
		}
		if a.Relevance != b.Relevance {
			if a.Relevance > b.Relevance {
				return -1
			}
			return 1
		}
		if a.Persistent != b.Persistent {
			if a.Persistent {
				return -1
			}
			return 1
		}
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return 0
	})

	selectedContext := append([]map[string]any(nil), mandatoryContext...)
	selectedAuxiliaryContext := []map[string]any{}
	selectedMemories := []map[string]any{}
	selectedLedgerItems := []any{}
	selectedWorldRules := []map[string]any{}
	selectedMemoryTurns := map[int]bool{}
	selected := []map[string]any{}
	truncated := []map[string]any{}
	buildLedger := func() map[string]any {
		if len(selectedLedgerItems) == 0 && len(selectedMemories) == 0 && len(selectedWorldRules) == 0 {
			return nil
		}
		out := cloneMapAny(ledgerBase)
		if out == nil {
			out = map[string]any{}
		}
		out["items"] = append([]any(nil), selectedLedgerItems...)
		if len(selectedMemories) > 0 {
			items := make([]any, 0, len(selectedMemories))
			for _, item := range selectedMemories {
				items = append(items, item)
			}
			out["relevant_turn_memories"] = items
		}
		if len(selectedWorldRules) > 0 {
			items := make([]any, 0, len(selectedWorldRules))
			for _, item := range selectedWorldRules {
				items = append(items, item)
			}
			out["active_world_rules"] = items
		}
		return out
	}
	measure := func() int {
		empty, _ := json.Marshal(map[string]any{"context_messages": []map[string]any{}, "archive_ledger": nil})
		payload, _ := json.Marshal(map[string]any{
			"context_messages": selectedAuxiliaryContext,
			"archive_ledger":   buildLedger(),
		})
		chars := len([]rune(string(payload))) - len([]rune(string(empty)))
		if chars < 0 {
			return 0
		}
		return chars
	}
	budget := policy.AuxiliaryMaxChars
	if budget < 0 {
		budget = 0
	}
	used := measure()
	baseChars := used
	for _, candidate := range candidates {
		if candidate.Kind == "relevant_memory_source_turn" && !selectedMemoryTurns[candidate.TurnIndex] {
			excluded = append(excluded, map[string]any{
				"kind": candidate.Kind, "id": candidate.ID, "reason": "related_memory_not_selected",
			})
			continue
		}
		before := used
		switch candidate.Kind {
		case "relevant_memory":
			selectedMemories = append(selectedMemories, candidate.Value)
		case "relevant_memory_source_turn":
			selectedAuxiliaryContext = append(selectedAuxiliaryContext, candidate.Messages...)
		case "critic_archive_ledger":
			selectedLedgerItems = append(selectedLedgerItems, candidate.Value)
		case "active_world_rule":
			selectedWorldRules = append(selectedWorldRules, candidate.Value)
		}
		used = measure()
		if used > budget {
			attemptedChars := used - before
			switch candidate.Kind {
			case "relevant_memory":
				selectedMemories = selectedMemories[:len(selectedMemories)-1]
			case "relevant_memory_source_turn":
				selectedAuxiliaryContext = selectedAuxiliaryContext[:len(selectedAuxiliaryContext)-len(candidate.Messages)]
			case "critic_archive_ledger":
				selectedLedgerItems = selectedLedgerItems[:len(selectedLedgerItems)-1]
			case "active_world_rule":
				selectedWorldRules = selectedWorldRules[:len(selectedWorldRules)-1]
			}
			used = before
			excluded = append(excluded, map[string]any{
				"kind": candidate.Kind, "id": candidate.ID, "reason": "auxiliary_input_budget_exhausted",
				"candidate_chars": maxInt(attemptedChars, 0),
			})
			continue
		}
		if candidate.Kind == "relevant_memory" {
			selectedMemoryTurns[candidate.TurnIndex] = true
		}
		selected = append(selected, map[string]any{
			"kind": candidate.Kind, "id": candidate.ID, "chars": used - before,
			"relevance": candidate.Relevance,
		})
	}
	selectedContext = append(selectedContext, selectedAuxiliaryContext...)
	trace := map[string]any{
		"contract_version":                       "critic_input_selection.v1",
		"budget_source":                          policy.Source,
		"configured_context_chars":               policy.ConfiguredChars,
		"ledger_budget_chars":                    policy.LedgerChars,
		"auxiliary_budget_chars":                 budget,
		"auxiliary_base_chars":                   baseChars,
		"auxiliary_selected_chars":               used,
		"auxiliary_remaining_chars":              maxInt(budget-used, 0),
		"selected":                               selected,
		"excluded":                               excluded,
		"truncated":                              truncated,
		"selected_count":                         len(selected),
		"excluded_count":                         len(excluded),
		"truncated_count":                        len(truncated),
		"partial_item_truncation":                false,
		"current_turn_bounded":                   false,
		"previous_turn_bounded":                  false,
		"selection_query_includes_previous_turn": len(mandatoryContext) > 0,
	}
	return selectedContext, buildLedger(), trace
}

func (s *Server) buildCompleteTurnActiveWorldRuleInput(ctx context.Context, sid string) ([]map[string]any, map[string]any) {
	trace := map[string]any{"status": "unavailable", "included_count": 0}
	if s == nil || s.Store == nil {
		return nil, trace
	}
	rows, err := s.Store.ListWorldRules(ctx, sid)
	if err != nil {
		trace["status"] = "read_failed"
		trace["error"] = err.Error()
		return nil, trace
	}
	out := []map[string]any{}
	for _, row := range rows {
		if row.Suppressed || strings.TrimSpace(row.Key) == "" {
			continue
		}
		var value any = strings.TrimSpace(row.ValueJSON)
		if strings.TrimSpace(row.ValueJSON) != "" {
			var decoded any
			if json.Unmarshal([]byte(row.ValueJSON), &decoded) == nil {
				value = decoded
			}
		}
		out = append(out, map[string]any{
			"scope":       row.Scope,
			"scope_name":  row.ScopeName,
			"category":    row.Category,
			"key":         row.Key,
			"value":       value,
			"source_turn": row.SourceTurn,
		})
	}
	trace["status"] = "ok"
	trace["included_count"] = len(out)
	return out, trace
}

func criticArchiveLedgerPromptInput(resp criticArchiveLedgerPreviewResponse) map[string]any {
	items := make([]map[string]any, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, map[string]any{
			"lane":       item.Lane,
			"id":         item.ID,
			"authority":  item.Authority,
			"status":     item.Status,
			"summary":    item.Summary,
			"updated_at": item.UpdatedAt,
			"source_ref": item.SourceRef,
		})
	}
	return map[string]any{
		"contract_version":         resp.ContractVersion,
		"status":                   resp.Status,
		"session_id":               resp.SessionID,
		"runtime_profile":          resp.RuntimeProfile,
		"store_mode":               resp.StoreMode,
		"vector_status":            resp.VectorStatus,
		"language":                 resp.Language,
		"limits":                   resp.Limits,
		"counts":                   resp.Counts,
		"degraded":                 resp.Degraded,
		"warnings":                 resp.Warnings,
		"items":                    items,
		"read_only":                true,
		"write_attempted":          false,
		"vector_write_attempted":   false,
		"llm_call_attempted":       false,
		"raw_archive_dump_blocked": true,
		"usage_policy":             "support_only_do_not_copy_as_new_evidence_without_latest_turn_support",
	}
}

func readCriticSystemPrompt(configuredDir string) (string, string) {
	candidates := []string{}
	if strings.TrimSpace(configuredDir) != "" {
		candidates = append(candidates, filepath.Join(configuredDir, "critic_system.txt"))
	}
	candidates = append(candidates,
		filepath.Join("..", "prompts", "critic_system.txt"),
		filepath.Join("prompts", "critic_system.txt"),
		filepath.Join("..", "..", "prompts", "critic_system.txt"),
	)
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return string(data), path
		}
	}
	return "You are Archive Center's critic extractor. Return only valid JSON matching the configured critic schema.", "fallback_builtin"
}

func readSupervisorSystemPrompt(configuredDir string) (string, string, error) {
	candidates := []string{}
	if strings.TrimSpace(configuredDir) != "" {
		candidates = append(candidates, filepath.Join(configuredDir, "supervisor_system.txt"))
	} else {
		candidates = append(candidates,
			filepath.Join("..", "prompts", "supervisor_system.txt"),
			filepath.Join("prompts", "supervisor_system.txt"),
			filepath.Join("..", "..", "prompts", "supervisor_system.txt"),
			filepath.Join("..", "..", "..", "prompts", "supervisor_system.txt"),
		)
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == "" {
			return "", path, fmt.Errorf("publisher system prompt is empty: %s", path)
		}
		return string(data), path, nil
	}
	return "", "missing", errors.New("publisher system prompt is missing")
}

func buildCompleteTurnCriticPrompt(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, _ *map[string]any, archiveLedger ...map[string]any) string {
	return buildCompleteTurnCriticPromptWithLanguageContext(sid, turnIndex, userInput, assistantContent, contextMessages, nil, nil, archiveLedger...)
}

func buildCompleteTurnCriticPromptWithLanguageContext(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, _ *map[string]any, languageContext map[string]any, archiveLedger ...map[string]any) string {
	ctx, _ := json.Marshal(contextMessages)
	langCtx, _ := json.Marshal(normalizeCompleteTurnLanguageContext(languageContext))
	var ledgerInput any
	if len(archiveLedger) > 0 && archiveLedger[0] != nil {
		ledgerInput = archiveLedger[0]
	}
	ledger, _ := json.Marshal(ledgerInput)
	inputMode := "paired"
	userInputState := "observed"
	if strings.TrimSpace(userInput) == "" && strings.TrimSpace(assistantContent) != "" {
		inputMode = "assistant_only"
		userInputState = "missing"
	}
	return strings.Join([]string{
		fmt.Sprintf("chat_session_id: %s", sid),
		fmt.Sprintf("turn_index: %d", turnIndex),
		fmt.Sprintf("input_mode: %s", inputMode),
		fmt.Sprintf("user_input_state: %s", userInputState),
		"When input_mode is assistant_only, extract only claims grounded in the assistant output. Do not invent missing user actions or dialogue. Keep every independently valid extracted item even when another field has no grounded item.",
		"",
		"<Latest_Turn>",
		"[User]",
		userInput,
		"",
		"[Assistant]",
		assistantContent,
		"</Latest_Turn>",
		"",
		"<Recent_Context_JSON>",
		string(ctx),
		"</Recent_Context_JSON>",
		"",
		"<Critic_Archive_Ledger_JSON>",
		string(ledger),
		"</Critic_Archive_Ledger_JSON>",
		"<Language_Context_JSON>",
		string(langCtx),
		"</Language_Context_JSON>",
	}, "\n")
}

func parseJSONFromLLMContent(content string) (map[string]any, error) {
	candidate, err := extractJSONCandidateFromLLMContent(content)
	if err == nil {
		if out, parseErr := unmarshalJSONCandidate(candidate); parseErr == nil {
			return out, nil
		}
		if out, repairErr := unmarshalJSONCandidate(repairJSONCandidate(candidate)); repairErr == nil {
			return out, nil
		}
	}

	structuralRepair := repairStructuralJSONQuotes(normalizeLLMJSONText(content))
	repairedCandidate, repairExtractErr := extractJSONCandidateFromLLMContent(structuralRepair)
	if repairExtractErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, repairExtractErr
	}
	repairedCandidate = repairJSONCandidate(repairedCandidate)
	out, repairErr := unmarshalJSONCandidate(repairedCandidate)
	if repairErr != nil {
		return nil, repairErr
	}
	return out, nil
}

func unmarshalJSONCandidate(candidate string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(candidate), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func extractJSONCandidateFromLLMContent(content string) (string, error) {
	cleaned := normalizeLLMJSONText(content)
	start := strings.Index(cleaned, "{")
	if start < 0 {
		return "", errors.New("critic_json_missing")
	}
	stack := []byte{}
	inString := false
	escaped := false
	for i := start; i < len(cleaned); i++ {
		ch := cleaned[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return "", errors.New("critic_json_mismatched_braces")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return strings.TrimSpace(cleaned[start : i+1]), nil
			}
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return "", errors.New("critic_json_mismatched_brackets")
			}
			stack = stack[:len(stack)-1]
		}
	}
	return "", errors.New("critic_json_incomplete")
}

func normalizeLLMJSONText(content string) string {
	cleaned := strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```JSON")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

func repairJSONCandidate(candidate string) string {
	repaired := replaceJSONLiteralsOutsideStrings(candidate)
	repaired = removeJSONTrailingCommasOutsideStrings(repaired)
	return strings.TrimSpace(repaired)
}

func repairStructuralJSONQuotes(input string) string {
	runes := []rune(input)
	var b strings.Builder
	b.Grow(len(input))
	inASCIIString := false
	inCurlyString := false
	escaped := false
	var previousSignificant rune
	for i, current := range runes {
		if inASCIIString {
			b.WriteRune(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inASCIIString = false
			}
			continue
		}
		if inCurlyString {
			if isCurlyJSONQuote(current) && curlyJSONQuoteClosesToken(runes, i) {
				b.WriteByte('"')
				inCurlyString = false
				previousSignificant = '"'
			} else {
				b.WriteRune(current)
			}
			continue
		}
		if current == '"' {
			b.WriteRune(current)
			inASCIIString = true
			escaped = false
			continue
		}
		if isCurlyJSONQuote(current) && curlyJSONQuoteCanOpenToken(previousSignificant) {
			b.WriteByte('"')
			inCurlyString = true
			escaped = false
			continue
		}
		b.WriteRune(current)
		if !isJSONWhitespaceRune(current) {
			previousSignificant = current
		}
	}
	return b.String()
}

func isCurlyJSONQuote(value rune) bool {
	switch value {
	case '\u2018', '\u2019', '\u201c', '\u201d', '\u201e', '\u201f':
		return true
	default:
		return false
	}
}

func curlyJSONQuoteCanOpenToken(previous rune) bool {
	switch previous {
	case 0, '{', '[', ',', ':':
		return true
	default:
		return false
	}
}

func curlyJSONQuoteClosesToken(input []rune, index int) bool {
	for i := index + 1; i < len(input); i++ {
		if isJSONWhitespaceRune(input[i]) {
			continue
		}
		switch input[i] {
		case ':', ',', '}', ']':
			return true
		default:
			return false
		}
	}
	return true
}

func isJSONWhitespaceRune(value rune) bool {
	switch value {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func removeJSONTrailingCommasOutsideStrings(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		current := input[i]
		if inString {
			b.WriteByte(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			b.WriteByte(current)
			continue
		}
		if current == ',' {
			next := i + 1
			for next < len(input) && (input[next] == ' ' || input[next] == '\t' || input[next] == '\r' || input[next] == '\n') {
				next++
			}
			if next < len(input) && (input[next] == '}' || input[next] == ']') {
				continue
			}
		}
		b.WriteByte(current)
	}
	return b.String()
}

func replaceJSONLiteralsOutsideStrings(input string) string {
	var b strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(input); {
		ch := input[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			i++
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			i++
			continue
		}
		if hasJSONLiteralAt(input, i, "None") {
			b.WriteString("null")
			i += len("None")
			continue
		}
		if hasJSONLiteralAt(input, i, "True") {
			b.WriteString("true")
			i += len("True")
			continue
		}
		if hasJSONLiteralAt(input, i, "False") {
			b.WriteString("false")
			i += len("False")
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func hasJSONLiteralAt(input string, pos int, literal string) bool {
	if pos+len(literal) > len(input) || input[pos:pos+len(literal)] != literal {
		return false
	}
	beforeOK := pos == 0 || !isJSONLiteralChar(input[pos-1])
	after := pos + len(literal)
	afterOK := after >= len(input) || !isJSONLiteralChar(input[after])
	return beforeOK && afterOK
}

func isJSONLiteralChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func validateCriticExtractionSchema(raw map[string]any) (map[string]any, map[string]any, error) {
	if raw == nil || len(raw) == 0 {
		return nil, nil, errors.New("critic schema requires a non-empty JSON object")
	}

	out := map[string]any{}
	trace := map[string]any{
		"contract_version":    "critic_record_quarantine.v1",
		"output_policy":       criticOutputPolicyVersion,
		"dropped_field_count": 0,
		"dropped_item_count":  0,
		"dropped_fields":      []any{},
		"dropped_items":       []any{},
	}
	dropField := func(field, reason, expected string, value any) {
		delete(out, field)
		trace["dropped_field_count"] = intFromAny(trace["dropped_field_count"], 0) + 1
		trace["dropped_fields"] = append(sliceFromAny(trace["dropped_fields"]), map[string]any{
			"field": field, "reason": reason, "expected": expected, "actual": fmt.Sprintf("%T", value),
		})
	}
	dropItem := func(field string, itemIndex int, reason, expected string, value any) {
		trace["dropped_item_count"] = intFromAny(trace["dropped_item_count"], 0) + 1
		detail := map[string]any{
			"field": field, "reason": reason, "expected": expected, "actual": fmt.Sprintf("%T", value),
		}
		if itemIndex >= 0 {
			detail["item_index"] = itemIndex
		}
		trace["dropped_items"] = append(sliceFromAny(trace["dropped_items"]), detail)
	}
	recognizedPayload := false
	for field, value := range raw {
		switch field {
		case "turn_summary":
			if _, ok := value.(string); ok {
				out[field] = value
				recognizedPayload = true
			} else {
				dropField(field, "wrong_type", "string", value)
			}
		case "importance_score", "emotional_intensity", "narrative_significance":
			switch value.(type) {
			case float64, float32, int, int32, int64, json.Number:
				out[field] = value
				recognizedPayload = true
			default:
				dropField(field, "wrong_type", "number", value)
			}
		case "evidence_excerpts", "prune_targets":
			items, ok := value.([]any)
			if !ok {
				dropField(field, "wrong_type", "array of strings", value)
				continue
			}
			for itemIndex, item := range items {
				text, ok := item.(string)
				if !ok || strings.TrimSpace(text) == "" {
					dropItem(field, itemIndex, "text_invalid", "non-empty string", item)
					continue
				}
				out[field] = append(sliceFromAny(out[field]), text)
				recognizedPayload = true
			}
		case "kg_triples", "character_deltas", "pending_threads", "speaker_attributions", "world_rules", "reversible_states",
			"physical_conditions", "entity_conditions", "narrative_events", "state_claims", "belief_updates",
			"subjective_entity_memories", "protected_secrets", "character_identity_accuracy", "persona_capsule_candidates",
			"interaction_events", "relationship_observations", "interaction_boundaries", "habit_observations",
			"character_profile_observations", "voice_observations", "user_interaction_profile", "rp_character_profile":
			items, ok := value.([]any)
			if !ok {
				dropField(field, "wrong_type", "array of objects", value)
				continue
			}
			for itemIndex, item := range items {
				data, ok := item.(map[string]any)
				if !ok || data == nil {
					dropItem(field, itemIndex, "item_invalid", "object", item)
					continue
				}
				out[field] = append(sliceFromAny(out[field]), data)
				recognizedPayload = true
			}
		case "entities", "relationship_memory", "state_deltas", "world_rule_audit", "world_state", "archive_hint", "story_clock":
			data, ok := value.(map[string]any)
			if !ok || data == nil {
				dropField(field, "wrong_type", "object", value)
				continue
			}
			if len(data) > 0 {
				out[field] = data
				recognizedPayload = true
			}
		default:
			dropField(field, "unsupported_field", "supported critic field", value)
		}
	}
	if !recognizedPayload {
		return nil, trace, errors.New("critic schema has no recognized extraction payload")
	}
	if intFromAny(trace["dropped_field_count"], 0) == 0 && intFromAny(trace["dropped_item_count"], 0) == 0 {
		trace = nil
	}
	return out, trace, nil
}

func quarantineCriticProtectedCandidates(raw map[string]any, userInput, assistantContent string) (map[string]any, map[string]any) {
	if raw == nil {
		return raw, nil
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[key] = value
	}
	// Collect perspective-scoped claims before structural validation. A private
	// claim must not be projected as an objective event, state, or KG fact.
	perspectiveClaims := criticPerspectiveClaims(raw)
	reasons := map[string]int{}
	total := 0
	kept := 0
	subjectiveLaneObserved := false
	subjectiveCandidateCount := 0
	subjectiveKeptCount := 0
	quarantine := func(reason string) {
		reasons[reason]++
	}

	if values, ok := raw["protected_secrets"].([]any); ok {
		keptItems := make([]any, 0, len(values))
		for _, value := range values {
			total++
			item, itemOK := value.(map[string]any)
			if !itemOK || item == nil {
				quarantine("protected_secret_not_object")
				continue
			}
			owner := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "owner"),
				stringFromMap(item, "owner_entity_name"),
				stringFromMap(item, "character_name"),
			))
			summary := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "summary"),
				stringFromMap(item, "memory_text"),
				stringFromMap(item, "secret_summary"),
				stringFromMap(item, "text"),
			))
			if owner == "" || summary == "" {
				quarantine("protected_secret_identity_or_summary_missing")
				continue
			}
			keptItems = append(keptItems, item)
			kept++
		}
		out["protected_secrets"] = keptItems
	}

	if values, ok := raw["subjective_entity_memories"].([]any); ok {
		subjectiveLaneObserved = true
		subjectiveCandidateCount = len(values)
		keptItems := make([]any, 0, len(values))
		for _, value := range values {
			total++
			item, itemOK := value.(map[string]any)
			if !itemOK || item == nil {
				quarantine("subjective_memory_not_object")
				continue
			}
			normalizeSubjectiveEntityMemoryProtection(item)
			owner := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "owner_entity_name"),
				stringFromMap(item, "owner_entity_key"),
				stringFromMap(item, "entity_name"),
				stringFromMap(item, "name"),
			))
			text := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "memory_text"),
				stringFromMap(item, "subjective_memory"),
				stringFromMap(item, "recollection"),
				stringFromMap(item, "interpretation"),
				stringFromMap(item, "summary"),
			))
			if owner == "" || text == "" {
				quarantine("protected_subjective_identity_or_text_missing")
				continue
			}
			keptItems = append(keptItems, item)
			kept++
			subjectiveKeptCount++
		}
		out["subjective_entity_memories"] = keptItems
	}

	if values, ok := raw["character_identity_accuracy"].([]any); ok {
		keptItems := make([]any, 0, len(values))
		for _, value := range values {
			total++
			item, itemOK := value.(map[string]any)
			if !itemOK || item == nil {
				quarantine("protected_identity_not_object")
				continue
			}
			surface := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "surface_identity_name"),
				stringFromMap(item, "public_identity_name"),
				stringFromMap(item, "alias_name"),
			))
			trueName := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "true_identity_name"),
				stringFromMap(item, "canonical_entity_name"),
				stringFromMap(item, "real_identity_name"),
			))
			if surface == "" || trueName == "" {
				quarantine("protected_identity_mapping_incomplete")
				continue
			}
			keptItems = append(keptItems, item)
			kept++
		}
		out["character_identity_accuracy"] = keptItems
	}

	perspectiveClaims = excludeValidatedPublicCriticPerspectiveClaims(
		perspectiveClaims,
		out,
		strings.TrimSpace(userInput+"\n"+assistantContent),
	)
	objectiveQuarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanesUsingClaims(out, perspectiveClaims)
	if objectiveQuarantined > 0 {
		reasons["perspective_claim_copied_to_objective_lane"] += objectiveQuarantined
	}

	if total == 0 && objectiveQuarantined == 0 && !subjectiveLaneObserved {
		return out, nil
	}
	reasonPayload := map[string]any{}
	for reason, count := range reasons {
		reasonPayload[reason] = count
	}
	trace := map[string]any{
		"contract_version":                 "critic_protected_candidate_quarantine.v1",
		"policy":                           "structural_collection_then_prepare_turn_selection",
		"candidate_count":                  total,
		"kept_count":                       kept,
		"quarantined_count":                total - kept,
		"objective_lane_quarantined_count": objectiveQuarantined,
		"reasons":                          reasonPayload,
	}
	if subjectiveLaneObserved {
		coverageStatus := "candidate_kept"
		switch {
		case subjectiveCandidateCount == 0:
			coverageStatus = "zero_unclassified_no_candidate"
		case subjectiveKeptCount == 0:
			coverageStatus = "all_candidates_rejected"
		}
		trace["subjective_memory_coverage"] = map[string]any{
			"candidate_count": subjectiveCandidateCount,
			"kept_count":      subjectiveKeptCount,
			"status":          coverageStatus,
			"zero_policy":     "valid_only_when_no_distinct_source_grounded_perspective_evidence",
			"npc_policy":      "evidence_eligible_not_required",
		}
	}
	return out, trace
}

type criticPerspectiveClaim struct {
	evidence              string
	claim                 string
	owner                 string
	subject               string
	kind                  string
	anchors               []string
	identityRoleProtected bool
}

func quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction map[string]any) int {
	claims := excludeValidatedPublicCriticPerspectiveClaims(criticPerspectiveClaims(extraction), extraction)
	return quarantineCriticPerspectiveClaimsFromObjectiveLanesUsingClaims(extraction, claims)
}

func quarantineCriticPerspectiveClaimsFromObjectiveLanesUsingClaims(
	extraction map[string]any,
	claims []criticPerspectiveClaim,
) int {
	if len(claims) == 0 {
		return 0
	}
	quarantined := 0
	for _, lane := range []string{"narrative_events", "state_claims", "kg_triples"} {
		items := sliceFromAny(extraction[lane])
		keptItems := make([]any, 0, len(items))
		for _, raw := range items {
			item := mapFromAny(raw)
			if criticObjectiveItemConflictsWithPerspectiveClaim(item, claims) {
				quarantined++
				continue
			}
			keptItems = append(keptItems, raw)
		}
		extraction[lane] = keptItems
	}
	return quarantined
}

func criticPerspectiveClaims(extraction map[string]any) []criticPerspectiveClaim {
	out := []criticPerspectiveClaim{}
	add := func(kind string, item map[string]any, ownerKeys, claimKeys []string) {
		evidence := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "evidence_excerpt"),
			stringFromMap(item, "evidence"),
			stringFromMap(item, "source_excerpt"),
		))
		claimValues := make([]string, 0, len(claimKeys))
		for _, key := range claimKeys {
			if value := strings.TrimSpace(extractionStringFromAny(item[key])); value != "" {
				claimValues = append(claimValues, value)
			}
		}
		ownerValues := make([]string, 0, len(ownerKeys))
		for _, key := range ownerKeys {
			ownerValues = append(ownerValues, stringsFromAny(item[key])...)
			if value := strings.TrimSpace(extractionStringFromAny(item[key])); value != "" {
				ownerValues = append(ownerValues, value)
			}
		}
		claim := strings.TrimSpace(strings.Join(claimValues, " "))
		owner := strings.TrimSpace(strings.Join(ownerValues, " "))
		subject := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "subject"),
			stringFromMap(item, "subject_name"),
			stringFromMap(item, "target"),
			stringFromMap(item, "entity_name"),
			stringFromMap(item, "canonical_entity_name"),
			stringFromMap(item, "true_identity_name"),
			stringFromMap(item, "surface_identity_name"),
			extractionFirstNonEmpty(ownerValues...),
		))
		if evidence == "" && claim == "" {
			return
		}
		anchors := []string{}
		if kind == "identity" {
			for _, value := range []string{
				extractionFirstNonEmpty(
					stringFromMap(item, "surface_identity_name"),
					stringFromMap(item, "public_identity_name"),
					stringFromMap(item, "alias_name"),
				),
				extractionFirstNonEmpty(
					stringFromMap(item, "true_identity_name"),
					stringFromMap(item, "canonical_entity_name"),
					stringFromMap(item, "real_identity_name"),
				),
			} {
				value = strings.TrimSpace(value)
				if value == "" || slices.Contains(anchors, value) {
					continue
				}
				anchors = append(anchors, value)
			}
		} else if subject != "" {
			anchors = append(anchors, subject)
		}
		out = append(out, criticPerspectiveClaim{
			evidence: evidence,
			claim:    claim,
			owner:    owner,
			subject:  subject,
			kind:     kind,
			anchors:  anchors,
			identityRoleProtected: kind == "identity" && strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "true_role"),
				stringFromMap(item, "true_allegiance"),
			)) != "",
		})
	}
	for _, raw := range sliceFromAny(extraction["belief_updates"]) {
		add("belief", mapFromAny(raw),
			[]string{"perspective_owner", "owner", "owner_entity_name", "knower", "believer", "listener_names", "knowledge_holders"},
			[]string{"value", "state_value", "belief", "claim"},
		)
	}
	for _, raw := range sliceFromAny(extraction["protected_secrets"]) {
		item := mapFromAny(raw)
		add("secret", item, []string{"owner", "subject"}, []string{"summary", "secret_summary", "text"})
	}
	for _, raw := range sliceFromAny(extraction["character_identity_accuracy"]) {
		item := mapFromAny(raw)
		add("identity", item,
			[]string{"canonical_entity_name", "true_identity_name", "surface_identity_name"},
			[]string{"true_identity_name", "surface_identity_name", "identity_kind", "true_role", "true_allegiance"},
		)
	}
	for _, raw := range sliceFromAny(extraction["subjective_entity_memories"]) {
		item := mapFromAny(raw)
		visibility := strings.ToLower(strings.TrimSpace(stringFromMap(item, "owner_visibility")))
		if !boolFromAny(item["secret_guard"]) &&
			visibility != "owner_private" &&
			strings.ToLower(strings.TrimSpace(stringFromMap(item, "portability"))) != "npc_private_recollection" {
			continue
		}
		add("subjective", item,
			[]string{"owner_entity_name", "owner_entity_key", "entity_name"},
			[]string{"memory_text", "subjective_memory", "recollection", "interpretation", "summary"},
		)
	}
	return out
}

func excludeValidatedPublicCriticPerspectiveClaims(
	claims []criticPerspectiveClaim,
	validatedExtraction map[string]any,
	acceptedSource ...string,
) []criticPerspectiveClaim {
	source := strings.TrimSpace(strings.Join(acceptedSource, "\n"))
	publicExtraction := map[string]any{}
	for _, lane := range []string{"protected_secrets", "character_identity_accuracy"} {
		publicItems := []any{}
		for _, raw := range sliceFromAny(validatedExtraction[lane]) {
			item := mapFromAny(raw)
			publiclyRevealed := boolFromAny(mapFromAny(item["knowledge_scope"])["publicly_revealed"])
			if publiclyRevealed && source != "" {
				evidence := strings.TrimSpace(extractionFirstNonEmpty(
					stringFromMap(item, "evidence_excerpt"),
					stringFromMap(item, "evidence"),
					stringFromMap(item, "source_excerpt"),
				))
				publiclyRevealed = evidence != "" && criticEvidenceOccursInSource(evidence, source)
			}
			if publiclyRevealed {
				publicItems = append(publicItems, raw)
			}
		}
		if len(publicItems) > 0 {
			publicExtraction[lane] = publicItems
		}
	}
	publicClaims := criticPerspectiveClaims(publicExtraction)
	if len(publicClaims) == 0 {
		return claims
	}
	privateClaims := make([]criticPerspectiveClaim, 0, len(claims))
	for _, claim := range claims {
		public := false
		for _, candidate := range publicClaims {
			if criticPerspectiveClaimsEquivalent(claim, candidate) {
				public = true
				break
			}
		}
		if !public {
			privateClaims = append(privateClaims, claim)
		}
	}
	return privateClaims
}

func criticPerspectiveClaimsEquivalent(left, right criticPerspectiveClaim) bool {
	if left.kind != right.kind {
		return false
	}
	if left.kind == "identity" {
		if len(left.anchors) != len(right.anchors) {
			return false
		}
		for index := range left.anchors {
			if normalizeArtifactDedupeText(left.anchors[index]) != normalizeArtifactDedupeText(right.anchors[index]) {
				return false
			}
		}
		return len(left.anchors) > 0
	}
	return normalizeArtifactDedupeText(left.subject) == normalizeArtifactDedupeText(right.subject) &&
		normalizeArtifactDedupeText(left.claim) == normalizeArtifactDedupeText(right.claim)
}

func criticObjectiveItemConflictsWithPerspectiveClaim(item map[string]any, claims []criticPerspectiveClaim) bool {
	if len(item) == 0 {
		return false
	}
	evidence := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "evidence_excerpt"),
		stringFromMap(item, "evidence"),
		stringFromMap(item, "source_excerpt"),
	))
	encoded, _ := json.Marshal(item)
	text := strings.TrimSpace(string(encoded))
	for _, protected := range claims {
		if evidence != "" && protected.evidence != "" &&
			normalizeArtifactDedupeText(evidence) == normalizeArtifactDedupeText(protected.evidence) {
			return true
		}
		if protected.claim != "" &&
			criticProtectedClaimSupported(protected.claim, text, protected.owner) {
			return true
		}
		if evidence == "" && criticObjectiveItemCarriesPerspectiveClaimSignal(item, protected) {
			return true
		}
	}
	return false
}

func criticObjectiveItemCarriesPerspectiveClaimSignal(item map[string]any, claim criticPerspectiveClaim) bool {
	if claim.kind == "identity" {
		if len(claim.anchors) >= 2 {
			encoded, _ := json.Marshal(item)
			text := strings.ToLower(string(encoded))
			for _, anchor := range claim.anchors {
				if !strings.Contains(text, strings.ToLower(strings.TrimSpace(anchor))) {
					return false
				}
			}
			return true
		}
		if len(claim.anchors) == 1 && claim.identityRoleProtected {
			return criticObjectiveItemHasExactPerspectiveAnchor(item, claim.anchors[0])
		}
		return false
	}

	anchor := normalizeArtifactDedupeText(claim.subject)
	if anchor == "" {
		return false
	}
	anchorMatched := criticObjectiveItemHasExactPerspectiveAnchor(item, anchor)
	if !anchorMatched {
		return false
	}

	itemJSON, _ := json.Marshal(item)
	itemTokens := criticSubstantiveTokens(string(itemJSON))
	claimTokens := criticSubstantiveTokens(claim.claim)
	for token := range criticSubstantiveTokens(claim.owner + " " + claim.subject) {
		delete(claimTokens, token)
	}
	overlap := 0
	for token := range claimTokens {
		if _, matched := itemTokens[token]; matched {
			overlap++
		}
	}
	if overlap >= 2 || (claim.kind == "belief" && overlap >= 1) {
		return true
	}
	if overlap == 0 {
		return false
	}
	predicateTokens := map[string]struct{}{}
	for _, key := range []string{"predicate", "relation", "relationship_type", "state_slot", "slot", "state_key"} {
		for token := range criticSubstantiveTokens(extractionStringFromAny(item[key])) {
			predicateTokens[token] = struct{}{}
		}
	}
	for token := range claimTokens {
		if _, matched := predicateTokens[token]; matched {
			return true
		}
	}
	return false
}

func criticObjectiveItemHasExactPerspectiveAnchor(item map[string]any, anchor string) bool {
	anchor = normalizeArtifactDedupeText(anchor)
	if anchor == "" {
		return false
	}
	for _, key := range []string{
		"subject", "subject_name", "target", "target_name",
		"entity", "entity_name", "character", "character_name",
		"actor", "actor_name", "owner", "owner_entity_name",
		"object", "object_name",
	} {
		if normalizeArtifactDedupeText(extractionStringFromAny(item[key])) == anchor {
			return true
		}
	}
	return false
}

func criticEvidenceOccursInSource(evidence, source string) bool {
	evidence = strings.TrimSpace(evidence)
	source = strings.TrimSpace(source)
	return evidence != "" && source != "" && strings.Contains(source, evidence)
}

func criticOwnerOccursInSource(owner, source string) bool {
	owner = strings.ToLower(strings.TrimSpace(owner))
	source = strings.ToLower(strings.TrimSpace(source))
	return owner != "" && source != "" && strings.Contains(source, owner)
}

func criticProtectedIdentitySupported(surface, trueName, evidence, source string) bool {
	surface = strings.ToLower(strings.TrimSpace(surface))
	trueName = strings.ToLower(strings.TrimSpace(trueName))
	evidence = strings.ToLower(strings.TrimSpace(evidence))
	source = strings.ToLower(strings.TrimSpace(source))
	if surface == "" || trueName == "" || evidence == "" || source == "" {
		return false
	}
	return criticTextContainsDistinctIdentityPair(source, surface, trueName) &&
		criticTextContainsDistinctIdentityPair(evidence, surface, trueName)
}

func criticTextContainsDistinctIdentityPair(text, surface, trueName string) bool {
	if !strings.Contains(text, surface) || !strings.Contains(text, trueName) {
		return false
	}
	if surface == trueName {
		return true
	}
	if strings.Contains(surface, trueName) {
		return strings.Contains(strings.ReplaceAll(text, surface, " "), trueName)
	}
	if strings.Contains(trueName, surface) {
		return strings.Contains(strings.ReplaceAll(text, trueName, " "), surface)
	}
	return true
}

func criticProtectedClaimSupported(claim, evidence, owner string) bool {
	claim = strings.ToLower(strings.TrimSpace(claim))
	evidence = strings.ToLower(strings.TrimSpace(evidence))
	if claim == "" || evidence == "" {
		return false
	}
	if strings.Contains(claim, evidence) || strings.Contains(evidence, claim) {
		return true
	}
	ownerTokens := criticSubstantiveTokens(owner)
	claimTokens := criticSubstantiveTokens(claim)
	evidenceTokens := criticSubstantiveTokens(evidence)
	overlap := 0
	for token := range claimTokens {
		if _, ownerToken := ownerTokens[token]; ownerToken {
			continue
		}
		if _, supported := evidenceTokens[token]; supported {
			overlap++
			if overlap >= 2 {
				return true
			}
		}
	}
	return false
}

func criticSubstantiveTokens(value string) map[string]struct{} {
	tokens := map[string]struct{}{}
	var current []rune
	flush := func() {
		if len(current) > 0 {
			tokens[string(current)] = struct{}{}
		}
		current = current[:0]
	}
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current = append(current, r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func normalizeCriticExtraction(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		out[k] = v
	}
	out["turn_summary"] = normalizeCriticTurnSummary(raw["turn_summary"])
	out["importance_score"] = clampFloat(extractionFloatFromAny(raw["importance_score"], 3), 1, 10)
	out["emotional_intensity"] = clampFloat(extractionFloatFromAny(raw["emotional_intensity"], 0), 0, 1)
	out["narrative_significance"] = clampFloat(extractionFloatFromAny(raw["narrative_significance"], 0), 0, 1)
	out["evidence_excerpts"] = stringsFromAny(raw["evidence_excerpts"])
	if storyClock := normalizeStoryClockProposal(raw["story_clock"]); len(storyClock) > 0 {
		out["story_clock"] = storyClock
	} else {
		delete(out, "story_clock")
	}
	out["kg_triples"] = sliceFromAny(raw["kg_triples"])
	out["character_deltas"] = normalizeCriticCharacterDeltas(raw["character_deltas"])
	out["pending_threads"] = normalizeCriticPendingThreads(raw["pending_threads"])
	out["entities"] = mapFromAny(raw["entities"])
	out["speaker_attributions"] = normalizeSpeakerAttributionCandidates(raw["speaker_attributions"])
	out["relationship_memory"] = mapFromAny(raw["relationship_memory"])
	out["interaction_events"] = sliceFromAny(raw["interaction_events"])
	out["relationship_observations"] = sliceFromAny(raw["relationship_observations"])
	out["interaction_boundaries"] = sliceFromAny(raw["interaction_boundaries"])
	out["habit_observations"] = sliceFromAny(raw["habit_observations"])
	out["character_profile_observations"] = sliceFromAny(raw["character_profile_observations"])
	out["voice_observations"] = sliceFromAny(raw["voice_observations"])
	out["user_interaction_profile"] = sliceFromAny(raw["user_interaction_profile"])
	out["rp_character_profile"] = sliceFromAny(raw["rp_character_profile"])
	out["state_deltas"] = mapFromAny(raw["state_deltas"])
	out["world_rules"] = sliceFromAny(raw["world_rules"])
	out["reversible_states"] = sliceFromAny(raw["reversible_states"])
	out["physical_conditions"] = sliceFromAny(raw["physical_conditions"])
	out["entity_conditions"] = sliceFromAny(raw["entity_conditions"])
	out["narrative_events"] = sliceFromAny(raw["narrative_events"])
	out["state_claims"] = sliceFromAny(raw["state_claims"])
	out["belief_updates"] = normalizeCriticBeliefUpdates(raw["belief_updates"])
	protectedSecrets := normalizeProtectedSecrets(raw["protected_secrets"])
	characterIdentityAccuracy := normalizeCharacterIdentityAccuracy(raw["character_identity_accuracy"])
	subjectiveMemories := normalizeSubjectiveEntityMemories(raw["subjective_entity_memories"])
	subjectiveMemories = appendBeliefUpdateSubjectiveMemories(subjectiveMemories, out["belief_updates"])
	subjectiveMemories = appendProtectedSecretSubjectiveMemories(subjectiveMemories, protectedSecrets)
	subjectiveMemories = appendIdentityAccuracySubjectiveMemories(subjectiveMemories, characterIdentityAccuracy)
	out["protected_secrets"] = protectedSecrets
	out["character_identity_accuracy"] = characterIdentityAccuracy
	out["subjective_entity_memories"] = subjectiveMemories
	out["persona_capsule_candidates"] = normalizePersonaCapsuleCandidates(raw["persona_capsule_candidates"])
	return out
}

func normalizeCriticCharacterDeltas(value any) []any {
	items := sliceFromAny(value)
	out := make([]any, 0, len(items))
	for _, raw := range items {
		item := mapFromAny(raw)
		if len(item) == 0 {
			out = append(out, raw)
			continue
		}
		normalized := make(map[string]any, len(item)+2)
		for key, field := range item {
			normalized[key] = field
		}
		if name := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "name"),
			stringFromMap(item, "character_name"),
			stringFromMap(item, "character"),
		)); name != "" {
			normalized["name"] = name
		}

		if status, structured := item["status"].(map[string]any); !structured || len(status) == 0 {
			change := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "change"),
				stringFromMap(item, "value"),
				stringFromMap(item, "status"),
			))
			if change != "" {
				slot := strings.TrimSpace(extractionFirstNonEmpty(
					stringFromMap(item, "delta_type"),
					stringFromMap(item, "change_type"),
					stringFromMap(item, "dimension"),
					stringFromMap(item, "aspect"),
				))
				if slot == "" {
					slot = "observed_change"
				}
				normalized["status"] = map[string]any{slot: change}
			}
		}
		out = append(out, normalized)
	}
	return out
}

func enrichNormalizedCriticExtractionForFocusedRecall(extraction map[string]any, userInput, assistantContent string, turnIndex int) map[string]any {
	if extraction == nil {
		extraction = map[string]any{}
	}
	extraction["turn_summary"] = normalizeCriticTurnSummary(extraction["turn_summary"])
	if strings.TrimSpace(extractionStringFromAny(extraction["turn_summary"])) == "" {
		if summary := focusedRecallFallbackSummary(userInput, assistantContent); summary != "" {
			extraction["turn_summary"] = summary
		}
	}
	extraction["evidence_excerpts"] = criticDirectEvidenceExcerpts(extraction, userInput, assistantContent)
	if len(stringsFromAny(extraction["evidence_excerpts"])) == 0 {
		if excerpts := focusedRecallFallbackEvidenceExcerpts(userInput, assistantContent); len(excerpts) > 0 {
			extraction["evidence_excerpts"] = excerpts
			extraction["focused_recall_fallback"] = map[string]any{
				"policy_version": "focused_recall_fallback.v1",
				"source":         "latest_turn_exact_excerpts",
				"turn_index":     turnIndex,
				"reason":         "critic_returned_no_evidence_excerpts",
			}
		}
	}
	return extraction
}

func normalizeCriticPendingThreads(raw any) []any {
	out := []any{}
	for _, candidate := range sliceFromAny(raw) {
		thread := mapFromAny(candidate)
		if len(thread) == 0 {
			continue
		}
		if threadType := normalizeCriticPendingThreadType(stringFromMap(thread, "thread_type")); threadType != "" {
			thread["thread_type"] = threadType
		}
		out = append(out, thread)
	}
	return out
}

func normalizeCriticPendingThreadType(raw string) string {
	var token []rune
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if separator && len(token) > 0 {
				token = append(token, '_')
			}
			token = append(token, r)
			separator = false
		default:
			separator = true
		}
	}
	switch strings.Trim(string(token), "_") {
	case "promise", "promises", "commitment", "commitments":
		return "promise"
	case "unresolved_goal", "unresolved_goals", "open_goal", "open_goals", "goal", "goals":
		return "unresolved_goal"
	case "open_question", "open_questions", "unresolved_question", "unresolved_questions", "question", "questions":
		return "open_question"
	case "risk", "risks", "threat", "threats":
		return "risk"
	case "emotional_debt", "emotional_debts", "emotional_obligation", "emotional_obligations":
		return "emotional_debt"
	default:
		return ""
	}
}

func normalizeCriticBeliefUpdates(raw any) []any {
	out := []any{}
	for _, candidate := range sliceFromAny(raw) {
		item := mapFromAny(candidate)
		if len(item) == 0 {
			continue
		}
		if strings.TrimSpace(stringFromMap(item, "subject")) == "" {
			item["subject"] = extractionFirstNonEmpty(stringFromMap(item, "topic"), stringFromMap(item, "fact_subject"))
		}
		if strings.TrimSpace(stringFromMap(item, "state_slot")) == "" {
			item["state_slot"] = extractionFirstNonEmpty(stringFromMap(item, "slot"), stringFromMap(item, "relation_dimension"))
		}
		if strings.TrimSpace(stringFromMap(item, "value")) == "" {
			item["value"] = extractionFirstNonEmpty(stringFromMap(item, "claim"), stringFromMap(item, "fact"), stringFromMap(item, "belief"))
		}
		if strings.TrimSpace(stringFromMap(item, "speaker_name")) == "" {
			item["speaker_name"] = extractionFirstNonEmpty(stringFromMap(item, "speaker"), stringFromMap(item, "actor"))
		}
		if strings.TrimSpace(stringFromMap(item, "evidence_excerpt")) == "" {
			item["evidence_excerpt"] = extractionFirstNonEmpty(stringFromMap(item, "evidence"), stringFromMap(item, "source_excerpt"))
		}
		if strings.TrimSpace(stringFromMap(item, "perspective_owner")) == "" {
			owner := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "owner"), stringFromMap(item, "owner_entity_name"),
			))
			if owner != "" {
				item["perspective_owner"] = owner
			}
		}
		listeners := append([]string{}, stringsFromAny(item["listener_names"])...)
		for _, listener := range []string{
			stringFromMap(item, "listener_name"),
			stringFromMap(item, "knowledge_holder"),
		} {
			listener = strings.TrimSpace(listener)
			if listener != "" && !slices.Contains(listeners, listener) {
				listeners = append(listeners, listener)
			}
		}
		if len(listeners) > 0 {
			item["listener_names"] = listeners
		}
		if strings.TrimSpace(stringFromMap(item, "perspective_owner")) == "" && len(listeners) == 1 {
			item["perspective_owner"] = listeners[0]
		}
		out = append(out, item)
	}
	return out
}

func criticDirectEvidenceExcerpts(extraction map[string]any, userInput, assistantContent string) []string {
	source := strings.TrimSpace(strings.Join([]string{userInput, assistantContent}, "\n"))
	out := []string{}
	seen := map[string]bool{}
	add := func(candidate string) {
		excerpt := sanitizeEvidenceExcerptForTurn(candidate, source)
		key := normalizeArtifactDedupeText(excerpt)
		if excerpt == "" || key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, excerpt)
	}
	for _, excerpt := range stringsFromAny(extraction["evidence_excerpts"]) {
		add(excerpt)
	}
	if clock := mapFromAny(extraction["story_clock"]); len(clock) > 0 {
		add(stringFromMap(clock, "evidence_excerpt"))
	}
	return out
}

func normalizeCriticTurnSummary(value any) string {
	if value == nil || isStructuredCriticTurnSummaryValue(value) {
		return ""
	}
	text := strings.TrimSpace(extractionStringFromAny(value))
	if looksLikeStructuredCriticPayloadText(text) {
		return ""
	}
	return text
}

func isStructuredCriticTurnSummaryValue(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func looksLikeStructuredCriticPayloadText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "map[") && !strings.HasPrefix(trimmed, "[") {
		return false
	}
	lower := strings.ToLower(trimmed)
	hits := 0
	for _, marker := range []string{
		"archive_hint",
		"character_deltas",
		"reversible_states",
		"evidence_excerpts",
		"kg_triples",
		"pending_threads",
		"relationship_memory",
		"habit_observations",
		"character_profile_observations",
		"voice_observations",
		"narrative_events",
		"state_claims",
		"belief_updates",
		"state_deltas",
		"subjective_entity_memories",
		"turn_summary",
		"world_rules",
	} {
		if strings.Contains(lower, marker) {
			hits++
		}
	}
	return hits >= 2
}

func focusedRecallFallbackSummary(userInput, assistantContent string) string {
	user := strings.TrimSpace(sanitizeCriticStorageText(userInput))
	assistant := strings.TrimSpace(sanitizeCriticStorageText(assistantContent))
	parts := []string{}
	if user != "" {
		parts = append(parts, "user: "+user)
	}
	if assistant != "" {
		parts = append(parts, "assistant: "+assistant)
	}
	return strings.Join(parts, " / ")
}

func focusedRecallFallbackEvidenceExcerpts(userInput, assistantContent string) []string {
	out := []string{}
	add := func(text string) {
		for _, excerpt := range focusedRecallExcerptCandidates(text) {
			if excerpt == "" || containsStringFold(out, excerpt) {
				continue
			}
			out = append(out, excerpt)
		}
	}
	add(userInput)
	add(assistantContent)
	return out
}

func focusedRecallExcerptCandidates(text string) []string {
	text = strings.TrimSpace(sanitizeCriticStorageText(text))
	if text == "" {
		return nil
	}
	candidates := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, piece := range splitFocusedRecallLine(line) {
			piece = strings.TrimSpace(piece)
			if !looksLikeFocusedRecallExcerpt(piece) {
				continue
			}
			candidates = append(candidates, piece)
		}
	}
	if len(candidates) == 0 && looksLikeFocusedRecallExcerpt(text) {
		candidates = append(candidates, text)
	}
	return candidates
}

func splitFocusedRecallLine(line string) []string {
	out := []string{}
	start := 0
	runes := []rune(line)
	for i, r := range runes {
		switch r {
		case '.', '!', '?', '。', '！', '？', '…':
			if i+1-start >= 12 {
				out = append(out, string(runes[start:i+1]))
				start = i + 1
			}
		}
	}
	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}
	return out
}

func looksLikeFocusedRecallExcerpt(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	runeLen := len([]rune(text))
	if runeLen < 8 {
		return false
	}
	lower := strings.ToLower(text)
	blocked := []string{"```", "archive center", "auxiliary context", "direct evidence", "latest direct evidence", "recent raw turn"}
	for _, item := range blocked {
		if strings.Contains(lower, item) {
			return false
		}
	}
	return true
}

func containsStringFold(items []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	for _, item := range items {
		if strings.TrimSpace(strings.ToLower(item)) == target {
			return true
		}
	}
	return false
}

func appendUniqueTurnRoleText(existing, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	if existing == "" {
		return next
	}
	for _, part := range strings.Split(existing, "\n") {
		if strings.EqualFold(strings.TrimSpace(part), next) {
			return existing
		}
	}
	return existing + "\n" + next
}
