package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCriticPipelineErrorClassificationPreservesStageAndHTTPStatus(t *testing.T) {
	timeoutErr := classifyCriticProviderError(context.DeadlineExceeded, http.StatusBadGateway)
	timeoutDetails := criticPipelineErrorDetails(timeoutErr)
	if timeoutDetails["code"] != "CRITIC_PROVIDER_TIMEOUT" ||
		timeoutDetails["stage"] != "provider_call" ||
		timeoutDetails["retryable"] != true ||
		timeoutDetails["http_status"] != http.StatusBadGateway {
		t.Fatalf("timeout details = %#v", timeoutDetails)
	}

	httpErr := classifyCriticProviderError(errors.New("unauthorized"), http.StatusUnauthorized)
	httpDetails := criticPipelineErrorDetails(httpErr)
	if httpDetails["code"] != "CRITIC_PROVIDER_HTTP_ERROR" ||
		httpDetails["stage"] != "provider_response" ||
		httpDetails["retryable"] != false ||
		httpDetails["http_status"] != http.StatusUnauthorized {
		t.Fatalf("HTTP details = %#v", httpDetails)
	}

	emptyDetails := criticPipelineErrorDetails(classifyCriticProviderError(
		&proxyEmptyContentError{Provider: "claude"}, http.StatusNoContent,
	))
	if emptyDetails["code"] != "CRITIC_EMPTY_RESPONSE" ||
		emptyDetails["stage"] != "provider_response" ||
		emptyDetails["retryable"] != true ||
		emptyDetails["http_status"] != http.StatusNoContent {
		t.Fatalf("empty response details = %#v", emptyDetails)
	}

	localDetails := criticPipelineErrorDetails(classifyCriticProviderError(
		&proxyLocalRequestError{Stage: "request_build", Cause: errors.New("conflict")},
		http.StatusBadRequest,
	))
	if localDetails["code"] != "CRITIC_REQUEST_BUILD_FAILED" ||
		localDetails["stage"] != "request_build" ||
		localDetails["retryable"] != false {
		t.Fatalf("local request details = %#v", localDetails)
	}
	if _, hasHTTPStatus := localDetails["http_status"]; hasHTTPStatus {
		t.Fatalf("local request error must not claim upstream HTTP status: %#v", localDetails)
	}

	trace := criticFailureTrace("test", completeTurnLLMConfig{
		Provider: "openai",
		Model:    "critic-test",
		APIKey:   "secret-key",
	}, http.StatusUnauthorized, httpErr, "provider rejected secret-key")
	if trace["http_status"] != http.StatusUnauthorized ||
		!strings.Contains(stringFromMap(trace, "raw_preview"), "[redacted]") ||
		strings.Contains(stringFromMap(trace, "raw_preview"), "secret-key") {
		t.Fatalf("failure trace = %#v", trace)
	}
}

func TestCriticFailureTraceRedactsCredentialValuesNotEqualToConfiguredKey(t *testing.T) {
	raw := `Authorization: Bearer different-token-123 ` +
		`{"password":"hunter2","access_token":"other-access","api_key":"configured-key"}`
	trace := criticFailureTrace("test", completeTurnLLMConfig{
		Provider: "openai",
		Model:    "critic-test",
		APIKey:   "configured-key",
	}, http.StatusUnauthorized, errors.New(raw), raw)
	preview := stringFromMap(trace, "raw_preview")
	for _, secret := range []string{"different-token-123", "hunter2", "other-access", "configured-key"} {
		if strings.Contains(preview, secret) {
			t.Fatalf("credential %q leaked in preview: %s", secret, preview)
		}
	}
	if strings.Count(preview, "[redacted]") < 4 {
		t.Fatalf("expected credential values to be redacted: %s", preview)
	}
}

func TestCriticRedactedRetryFailureTraceIsSerializableAndKeepsFinalPreview(t *testing.T) {
	oldClient := proxyHTTPClient
	callCount := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		callCount++
		marker := "first failure marker"
		if callCount == 2 {
			marker = "second failure marker"
		}
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"message":"` + marker + `"}}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
	_, trace, err := srv.runCompleteTurnCritic(
		context.Background(),
		"session",
		1,
		"Mina asks Rowan to be gentle.",
		"The intimate scene involved penetration.",
		nil,
		nil,
		completeTurnLLMConfig{
			Provider:    "openai",
			Endpoint:    "https://example.invalid/v1",
			APIKey:      "test-key",
			Model:       "critic-test",
			TimeoutMs:   30_000,
			RetryBudget: newLLMRetryBudget(1),
		},
	)
	if err == nil || callCount != 2 {
		t.Fatalf("error=%v calls=%d trace=%+v", err, callCount, trace)
	}
	if !strings.Contains(stringFromMap(trace, "raw_preview"), "second failure marker") {
		t.Fatalf("final retry preview was lost: %+v", trace)
	}
	retry := mapFromAny(trace["provider_retry"])
	if stringFromMap(retry, "retry_failure_recorded") != "top_level" {
		t.Fatalf("retry lineage missing: %+v", trace)
	}
	if _, err := json.Marshal(trace); err != nil {
		t.Fatalf("retry failure trace is cyclic or unserializable: %v; trace=%+v", err, trace)
	}
}

func TestCriticSensitiveRedactionRespectsZeroRetryBudget(t *testing.T) {
	oldClient := proxyHTTPClient
	callCount := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
	_, trace, err := srv.runCompleteTurnCritic(
		context.Background(), "session", 1,
		"Mina asks Rowan to be gentle.",
		"The intimate scene involved penetration.",
		nil, nil,
		completeTurnLLMConfig{
			Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "test-key",
			Model: "critic-test", TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(0),
		},
	)
	if err == nil || callCount != 1 {
		t.Fatalf("error=%v calls=%d trace=%+v", err, callCount, trace)
	}
	if len(mapFromAny(trace["provider_retry"])) != 0 {
		t.Fatalf("zero retry budget unexpectedly recorded a retry: %+v", trace)
	}
}

func TestCriticExtractionSchemaRejectsParsedButInvalidPayload(t *testing.T) {
	for name, payload := range map[string]map[string]any{
		"empty":              {},
		"wrong_summary_type": {"turn_summary": []any{"not", "text"}},
		"wrong_array_type":   {"turn_summary": "ok", "evidence_excerpts": "not-array"},
		"unknown_only":       {"unrecognized": "value"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateCriticExtractionSchema(payload); err == nil {
				t.Fatalf("payload should be rejected: %#v", payload)
			}
		})
	}
	if err := validateCriticExtractionSchema(map[string]any{
		"turn_summary":      "Mina found the key.",
		"importance_score":  float64(7),
		"evidence_excerpts": []any{"Mina found the key."},
	}); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
}

func TestCriticProtectedCandidateQuarantineRequiresGroundedEvidenceAndPolicy(t *testing.T) {
	source := "Masked Mina told Rowan that she was Mina. Mina told Rowan that she hid the brass key."
	payload := map[string]any{
		"turn_summary": "Mina kept a secret.",
		"protected_secrets": []any{
			map[string]any{
				"owner":             "Mina",
				"summary":           "Mina hid the brass key.",
				"disclosure_policy": "owner_private_until_revealed",
				"evidence_excerpt":  "she hid the brass key",
			},
			map[string]any{
				"owner":             "Mina",
				"summary":           "Invented secret.",
				"disclosure_policy": "owner_private_until_revealed",
				"evidence_excerpt":  "text absent from source",
			},
		},
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_name":    "Mina",
				"owner_entity_role":    "npc",
				"owner_visibility":     "owner_private",
				"memory_text":          "Mina hid the brass key and remembers it.",
				"target_reveal_policy": "owner_private_until_revealed",
				"evidence_excerpt":     "she hid the brass key",
			},
			map[string]any{
				"owner_entity_name": "Mina",
				"owner_entity_role": "npc",
				"memory_text":       "Missing policy must not be promoted.",
				"evidence_excerpt":  "she hid the brass key",
			},
		},
		"character_identity_accuracy": []any{
			map[string]any{
				"same_entity":           true,
				"surface_identity_name": "Masked Mina",
				"true_identity_name":    "Mina",
				"reveal_policy":         "owner_private_until_revealed",
				"evidence_excerpt":      "Masked Mina told Rowan that she was Mina",
			},
			map[string]any{
				"same_entity":           true,
				"surface_identity_name": "Unknown",
				"true_identity_name":    "Other",
				"reveal_policy":         "owner_private_until_revealed",
				"evidence_excerpt":      "text absent from source",
			},
			map[string]any{
				"same_entity":           true,
				"surface_identity_name": "Masked Mina",
				"true_identity_name":    "Mina",
				"reveal_policy":         "owner_private_until_revealed",
				"evidence_excerpt":      "Masked Mina told Rowan",
			},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(payload, "", source)
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 1 {
		t.Fatalf("protected secrets kept = %d, want 1: %#v", got, filtered["protected_secrets"])
	}
	if got := len(sliceFromAny(filtered["subjective_entity_memories"])); got != 1 {
		t.Fatalf("subjective memories kept = %d, want 1: %#v", got, filtered["subjective_entity_memories"])
	}
	if got := len(sliceFromAny(filtered["character_identity_accuracy"])); got != 1 {
		t.Fatalf("identity mappings kept = %d, want 1: %#v", got, filtered["character_identity_accuracy"])
	}
	if intFromAny(trace["candidate_count"], 0) != 7 ||
		intFromAny(trace["kept_count"], 0) != 3 ||
		intFromAny(trace["quarantined_count"], 0) != 4 {
		t.Fatalf("quarantine trace = %#v", trace)
	}
}

func TestCriticSubjectiveQuarantineBlocksDefaultPrivatePromotionButKeepsExplicitPublicNPC(t *testing.T) {
	payload := map[string]any{
		"turn_summary": "Mina spoke to Rowan.",
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_name": "Mina",
				"memory_text":       "Missing role and visibility must not become private by default.",
			},
			map[string]any{
				"owner_entity_name": "Rowan",
				"owner_entity_role": "npc",
				"owner_visibility":  "player_known",
				"memory_text":       "Rowan openly remembers the meeting.",
			},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(payload, "", "Mina spoke to Rowan.")
	items := sliceFromAny(filtered["subjective_entity_memories"])
	if len(items) != 1 {
		t.Fatalf("kept subjective memories = %d, want only explicit public NPC: %#v", len(items), items)
	}
	public := mapFromAny(items[0])
	if stringFromMap(public, "owner_entity_name") != "Rowan" {
		t.Fatalf("unexpected public memory kept: %#v", public)
	}
	if intFromAny(trace["quarantined_count"], 0) != 1 {
		t.Fatalf("quarantine trace = %#v", trace)
	}

	normalized := normalizeSubjectiveEntityMemories(items)
	if len(normalized) != 1 {
		t.Fatalf("normalized public memories = %#v", normalized)
	}
	got := mapFromAny(normalized[0])
	if stringFromMap(got, "owner_visibility") != "player_known" ||
		stringFromMap(got, "target_reveal_policy") != "" ||
		stringFromMap(got, "portability") != "portable_subjective_entity_recollection" {
		t.Fatalf("explicit public NPC was promoted to private: %#v", got)
	}
}

func TestCriticProtectedQuarantineRejectsUnrelatedGroundedEvidence(t *testing.T) {
	source := "Mina opened the garden door. Rowan watched from the hall."
	payload := map[string]any{
		"turn_summary": "Mina opened the door.",
		"protected_secrets": []any{
			map[string]any{
				"owner":             "Mina",
				"summary":           "Mina concealed a murder.",
				"disclosure_policy": "owner_private_until_revealed",
				"evidence_excerpt":  "Mina opened the garden door.",
			},
		},
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_name":    "Mina",
				"owner_entity_role":    "npc",
				"owner_visibility":     "owner_private",
				"memory_text":          "Mina remembers stealing the crown.",
				"target_reveal_policy": "owner_private_until_revealed",
				"evidence_excerpt":     "Mina opened the garden door.",
			},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(payload, "", source)
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 0 {
		t.Fatalf("unrelated protected secret was kept: %#v", filtered["protected_secrets"])
	}
	if got := len(sliceFromAny(filtered["subjective_entity_memories"])); got != 0 {
		t.Fatalf("unrelated protected subjective memory was kept: %#v", filtered["subjective_entity_memories"])
	}
	reasons := mapFromAny(trace["reasons"])
	if intFromAny(reasons["protected_secret_claim_unbound"], 0) != 1 ||
		intFromAny(reasons["protected_subjective_claim_unbound"], 0) != 1 {
		t.Fatalf("unexpected quarantine reasons: %#v", trace)
	}
}
