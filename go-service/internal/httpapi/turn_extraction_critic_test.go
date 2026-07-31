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

func TestPerspectiveClaimsCannotBeCopiedIntoObjectiveLanes(t *testing.T) {
	extraction := map[string]any{
		"belief_updates": []any{map[string]any{
			"perspective_owner": "Rowan",
			"subject":           "vault",
			"state_slot":        "access",
			"value":             "vault is open",
			"evidence_excerpt":  "Mira privately told Rowan that the vault was open.",
		}},
		"narrative_events": []any{
			map[string]any{
				"event":            "Rowan knows the vault is open.",
				"evidence_excerpt": "Mira privately told Rowan that the vault was open.",
			},
			map[string]any{
				"event":            "The public bell rang.",
				"evidence_excerpt": "The public bell rang.",
			},
		},
		"state_claims": []any{
			map[string]any{
				"subject":          "vault",
				"state_slot":       "access",
				"value":            "vault is open",
				"evidence_excerpt": "Mira privately told Rowan that the vault was open.",
			},
		},
		"kg_triples": []any{
			map[string]any{"subject": "vault", "predicate": "status", "object": "open"},
			map[string]any{"subject": "bell", "predicate": "rang_at", "object": "noon"},
		},
	}
	quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction)
	if quarantined != 3 {
		t.Fatalf("objective duplicate quarantine count=%d, want 3: %#v", quarantined, extraction)
	}
	events := sliceFromAny(extraction["narrative_events"])
	if len(events) != 1 || stringFromMap(mapFromAny(events[0]), "event") != "The public bell rang." {
		t.Fatalf("public objective event was not preserved: %#v", events)
	}
	if len(sliceFromAny(extraction["state_claims"])) != 0 {
		t.Fatalf("perspective state claim remained objective: %#v", extraction["state_claims"])
	}
	triples := sliceFromAny(extraction["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "subject") != "bell" {
		t.Fatalf("public KG triple was not preserved: %#v", triples)
	}
}

func TestRejectedPrivateCandidateStillQuarantinesObjectiveDuplicate(t *testing.T) {
	source := "Mina opened the garden door."
	extraction := map[string]any{
		"turn_summary": "Mina opened the door.",
		"protected_secrets": []any{map[string]any{
			"owner":             "Mina",
			"summary":           "Mina concealed a murder.",
			"disclosure_policy": "owner_private_until_revealed",
			"evidence_excerpt":  "This excerpt is absent from the source.",
		}},
		"kg_triples": []any{
			map[string]any{
				"subject":   "Mina",
				"predicate": "concealed",
				"object":    "a killing",
			},
			map[string]any{
				"subject":   "garden door",
				"predicate": "state",
				"object":    "open",
			},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(extraction, "", source)
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 0 {
		t.Fatalf("source-unbound private candidate was kept: %#v", filtered["protected_secrets"])
	}
	triples := sliceFromAny(filtered["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "subject") != "garden door" {
		t.Fatalf("private objective duplicate was not quarantined: %#v", triples)
	}
	if intFromAny(trace["objective_lane_quarantined_count"], 0) != 1 {
		t.Fatalf("objective quarantine was not traced: %#v", trace)
	}
}

func TestPerspectiveObjectiveQuarantinePreservesUnrelatedPublicFactForSameOwner(t *testing.T) {
	extraction := map[string]any{
		"subjective_entity_memories": []any{map[string]any{
			"owner_entity_name":    "Mina",
			"owner_visibility":     "owner_private",
			"memory_text":          "Mina secretly fears the magistrate.",
			"target_reveal_policy": "owner_private_until_revealed",
			"evidence_excerpt":     "Mina hid her fear from everyone.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Mina", "predicate": "appointed_as", "object": "captain"},
			map[string]any{"subject": "Mina", "predicate": "secretly_fears", "object": "magistrate"},
		},
	}

	quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction)
	if quarantined != 1 {
		t.Fatalf("objective quarantine count=%d, want 1: %#v", quarantined, extraction)
	}
	triples := sliceFromAny(extraction["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "predicate") != "appointed_as" {
		t.Fatalf("unrelated public fact for the same owner was removed: %#v", triples)
	}
}

func TestPerspectiveObjectiveQuarantinePreservesPublicFactSharingOnlyClaimNoun(t *testing.T) {
	extraction := map[string]any{
		"subjective_entity_memories": []any{map[string]any{
			"owner_entity_name":    "Mina",
			"owner_visibility":     "owner_private",
			"memory_text":          "Mina fears becoming captain.",
			"target_reveal_policy": "owner_private_until_revealed",
			"evidence_excerpt":     "Mina privately feared becoming captain.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Mina", "predicate": "appointed_as", "object": "captain"},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 0 {
		t.Fatalf("public fact sharing only a noun was quarantined: %#v", extraction)
	}
	if got := len(sliceFromAny(extraction["kg_triples"])); got != 1 {
		t.Fatalf("public appointment fact was removed: %#v", extraction["kg_triples"])
	}
}

func TestPerspectiveObjectiveQuarantinePreservesObjectiveTruthOppositeBeliefValue(t *testing.T) {
	extraction := map[string]any{
		"belief_updates": []any{map[string]any{
			"perspective_owner": "Rowan",
			"subject":           "vault",
			"state_slot":        "access",
			"value":             "open",
			"epistemic_state":   "misinformed",
			"evidence_excerpt":  "Rowan wrongly believed the vault was open.",
		}},
		"state_claims": []any{
			map[string]any{
				"subject":          "vault",
				"state_slot":       "access",
				"value":            "closed",
				"evidence_excerpt": "The vault was closed.",
			},
			map[string]any{
				"subject":    "vault",
				"state_slot": "access",
				"value":      "open",
			},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 1 {
		t.Fatalf("belief duplicate quarantine count=%d, want 1: %#v", quarantined, extraction)
	}
	states := sliceFromAny(extraction["state_claims"])
	if len(states) != 1 || stringFromMap(mapFromAny(states[0]), "value") != "closed" {
		t.Fatalf("objective truth opposite the character belief was removed: %#v", states)
	}
}

func TestPerspectiveObjectiveQuarantineCatchesIdentityAcrossBothKGEndpoints(t *testing.T) {
	extraction := map[string]any{
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Shade",
			"true_identity_name":    "Alice",
			"same_entity":           true,
			"reveal_policy":         "owner_private_until_revealed",
			"evidence_excerpt":      "Shade admitted privately that she was Alice.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Shade", "predicate": "is_really", "object": "Alice"},
			map[string]any{"subject": "Shade", "predicate": "entered", "object": "the hall"},
		},
	}

	quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction)
	if quarantined != 1 {
		t.Fatalf("identity objective quarantine count=%d, want 1: %#v", quarantined, extraction)
	}
	triples := sliceFromAny(extraction["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "predicate") != "entered" {
		t.Fatalf("identity objective duplicate was not isolated precisely: %#v", triples)
	}
}

func TestPerspectiveObjectiveQuarantineUsesPrimaryIdentityPairNotEveryAlias(t *testing.T) {
	extraction := map[string]any{
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Shade",
			"alias_name":            "Night",
			"true_identity_name":    "Alice",
			"same_entity":           true,
			"reveal_policy":         "owner_private_until_revealed",
			"evidence_excerpt":      "Shade admitted privately that she was Alice.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Shade", "predicate": "is_really", "object": "Alice"},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 1 {
		t.Fatalf("primary identity pair did not quarantine duplicate: %#v", extraction)
	}
}

func TestPerspectiveObjectiveQuarantineKeepsPubliclyRevealedIdentityObjective(t *testing.T) {
	extraction := map[string]any{
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Shade",
			"true_identity_name":    "Alice",
			"same_entity":           true,
			"transition":            "reveal",
			"knowledge_scope": map[string]any{
				"publicly_revealed": true,
			},
			"evidence_excerpt": "Shade publicly revealed that she was Alice.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Shade", "predicate": "is_really", "object": "Alice"},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 0 {
		t.Fatalf("publicly revealed identity was kept private: %#v", extraction)
	}
	if got := len(sliceFromAny(extraction["kg_triples"])); got != 1 {
		t.Fatalf("publicly revealed identity objective fact was removed: %#v", extraction["kg_triples"])
	}
}

func TestPerspectiveObjectiveQuarantineFailsClosedForSingleAnchorHiddenRoleWithoutEvidence(t *testing.T) {
	extraction := map[string]any{
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Mina",
			"true_identity_name":    "Mina",
			"same_entity":           true,
			"identity_kind":         "hidden_role",
			"true_role":             "spy",
			"reveal_policy":         "owner_private_until_revealed",
			"evidence_excerpt":      "Mina privately admitted that she served as a spy.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Mina", "predicate": "member_of", "object": "intelligence"},
			map[string]any{
				"subject":          "Mina",
				"predicate":        "entered",
				"object":           "the hall",
				"evidence_excerpt": "Mina entered the hall.",
			},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 1 {
		t.Fatalf("single-anchor hidden role quarantine count=%d, want 1: %#v", quarantined, extraction)
	}
	triples := sliceFromAny(extraction["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "predicate") != "entered" {
		t.Fatalf("evidence-bound unrelated public fact was not preserved: %#v", triples)
	}
}

func TestRejectedFalsePublicIdentityStillQuarantinesObjectiveDuplicate(t *testing.T) {
	source := "Shade entered the hall."
	extraction := map[string]any{
		"turn_summary": "Shade entered the hall.",
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Shade",
			"true_identity_name":    "Alice",
			"same_entity":           true,
			"reveal_policy":         "public_after_reveal",
			"transition":            "reveal",
			"knowledge_scope": map[string]any{
				"publicly_revealed": true,
			},
			"evidence_excerpt": "This public reveal is absent from the source.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Shade", "predicate": "is_really", "object": "Alice"},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(extraction, "", source)
	if got := len(sliceFromAny(filtered["character_identity_accuracy"])); got != 0 {
		t.Fatalf("source-unbound public identity candidate was kept: %#v", filtered["character_identity_accuracy"])
	}
	if got := len(sliceFromAny(filtered["kg_triples"])); got != 0 {
		t.Fatalf("rejected false public identity leaked into objective KG: %#v", filtered["kg_triples"])
	}
	if intFromAny(trace["objective_lane_quarantined_count"], 0) != 1 {
		t.Fatalf("rejected public identity objective quarantine was not traced: %#v", trace)
	}
}
