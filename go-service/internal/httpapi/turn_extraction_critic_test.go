package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestSanitizeContextMessagesUsesHostProvenanceInsteadOfProseKeywords(t *testing.T) {
	messages := []map[string]any{
		{
			"contract_version":  risuChatMessageObservationContract,
			"observation_state": "observed",
			"source_kind":       "active_chat_message",
			"role":              "user",
			"content":           "The character opens a book titled Persona and reads the rules aloud.",
		},
		{
			"contract_version":  risuChatMessageObservationContract,
			"observation_state": "observed",
			"source_kind":       "prompt_template",
			"role":              "user",
			"content":           "This is not an active chat message.",
		},
		{"role": "system", "content": "hidden prompt"},
	}

	got := sanitizeContextMessagesForCriticInput(messages)
	if len(got) != 1 || stringFromMap(got[0], "content") != messages[0]["content"] {
		t.Fatalf("structured context provenance mismatch: %#v", got)
	}
}

func TestSensitiveCriticRedactionRemainsFailureRecoveryOnly(t *testing.T) {
	plain := "Mira opened the archive door."
	if got, changed := redactSensitiveCriticRetryText(plain); changed || got != plain {
		t.Fatalf("ordinary critic input changed: got=%q changed=%v", got, changed)
	}
	sensitive := "The source contains explicit penetration detail and then Mira leaves."
	got, changed := redactSensitiveCriticRetryText(sensitive)
	if !changed || strings.Contains(strings.ToLower(got), "penetration") || !strings.Contains(got, "Mira leaves") {
		t.Fatalf("sensitive provider-retry recovery mismatch: got=%q changed=%v", got, changed)
	}
}

/*
Obsolete fixed entity-reference admission contract retained only in history.

	func TestCriticEntityReferenceContractReplacesDescriptorWordList(t *testing.T) {
		turnText := "경비가 문을 열었고, Mira가 안으로 들어왔다."
		turnLocal := map[string]any{
			"reference_contract": criticEntityReferenceContract,
			"reference_scope":    "turn_local_descriptor",
			"name":               "경비",
			"name_expression":    "경비",
			"evidence_excerpt":   "경비가 문을 열었고",
		}
		if ok, reason := criticEntityCanonicalWriteEligible(turnLocal, "경비", turnText); ok || reason != "turn_local_descriptor" {
			t.Fatalf("turn-local descriptor should not become canonical: ok=%v reason=%s", ok, reason)
		}

		stable := map[string]any{
			"reference_contract": criticEntityReferenceContract,
			"reference_scope":    "session_stable",
			"name":               "Mira",
			"name_expression":    "Mira",
			"evidence_excerpt":   "Mira가 안으로 들어왔다.",
		}
		if ok, reason := criticEntityCanonicalWriteEligible(stable, "Mira", turnText); !ok || reason != "session_stable" {
			t.Fatalf("grounded stable entity was rejected: ok=%v reason=%s", ok, reason)
		}

		stableUnknownLanguage := map[string]any{
			"reference_contract": criticEntityReferenceContract,
			"reference_scope":    "session_stable",
			"name_expression":    "守門人甲",
			"evidence_excerpt":   "守門人甲留下了自己的名字。",
		}
		if ok, reason := criticEntityCanonicalWriteEligible(stableUnknownLanguage, "守門人甲", "鐘が鳴った。守門人甲留下了自己的名字。門が閉じた。"); !ok {
			t.Fatalf("typed stable entity should be language-neutral: reason=%s", reason)
		}
	}
*/
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

func TestCriticOptionalStateErrorsDoNotDiscardIndependentKG(t *testing.T) {
	payload := map[string]any{
		"turn_summary":      "Mina entered the archive room.",
		"importance_score":  float64(6),
		"evidence_excerpts": []any{"Mina entered the archive room."},
		"story_clock":       nil,
		"kg_triples": []any{map[string]any{
			"semantic_class": "location_fact", "subject": "Mina", "predicate": "entered_location",
			"predicate_expression": "entered", "object": "archive room",
			"subject_binding": map[string]any{
				"contract_version": "kg_endpoint_binding.v1", "endpoint_kind": "entity",
				"entity_kind": "character", "expression": "Mina",
			},
			"object_binding": map[string]any{
				"contract_version": "kg_endpoint_binding.v1", "endpoint_kind": "scalar",
				"scalar_type": "string", "expression": "archive room",
			},
			"evidence_excerpt": "Mina entered the archive room.",
		}},
		"reversible_states": []any{map[string]any{
			"version": reversibleStateContractVersion, "domain": "appearance", "transition": "set",
			"subject_name": "Mina", "state_slot": "appearance", "value": map[string]any{"text": "dusty"},
			"evidence_excerpt": "Mina entered the archive room.", "scene_scope": "current",
			"authority": "canonical_in_fiction", "assertion_kind": "literal", "polarity": "affirmative",
			"visibility": "public", "sensitivity": "ordinary",
		}},
	}
	if err := validateCriticExtractionSchema(payload); err != nil {
		t.Fatalf("optional state error rejected the full extraction: %v", err)
	}
	extraction := normalizeCriticExtraction(payload)
	if len(sliceFromAny(extraction["kg_triples"])) != 1 {
		t.Fatalf("runtime extraction lost valid KG: %#v", extraction)
	}
	if _, exists := extraction["story_clock"]; exists || len(sliceFromAny(extraction["reversible_states"])) != 1 {
		t.Fatalf("open collection discarded the reversible observation or kept an empty story clock: %#v", extraction)
	}
}

func TestCriticProtectedCandidateCollectionKeepsStructurallyCompleteItems(t *testing.T) {
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
				"memory_text":       "Mina remembers that she hid the brass key.",
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
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 2 {
		t.Fatalf("protected secrets kept = %d, want 2: %#v", got, filtered["protected_secrets"])
	}
	if got := len(sliceFromAny(filtered["subjective_entity_memories"])); got != 2 {
		t.Fatalf("subjective memories kept = %d, want 2: %#v", got, filtered["subjective_entity_memories"])
	}
	defaulted := mapFromAny(sliceFromAny(filtered["subjective_entity_memories"])[1])
	if stringFromMap(defaulted, "target_reveal_policy") != "owner_private_until_revealed" {
		t.Fatalf("default-private policy was not normalized before quarantine: %#v", defaulted)
	}
	if got := len(sliceFromAny(filtered["character_identity_accuracy"])); got != 3 {
		t.Fatalf("identity mappings kept = %d, want 3: %#v", got, filtered["character_identity_accuracy"])
	}
	if intFromAny(trace["candidate_count"], 0) != 7 ||
		intFromAny(trace["kept_count"], 0) != 7 ||
		intFromAny(trace["quarantined_count"], 0) != 0 {
		t.Fatalf("quarantine trace = %#v", trace)
	}
}

func TestCriticSubjectiveQuarantineDefaultsPrivatePolicyButKeepsExplicitPublicNPC(t *testing.T) {
	payload := map[string]any{
		"turn_summary": "Mina spoke to Rowan.",
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_name": "Mina",
				"memory_text":       "Mina remembers that Mina spoke to Rowan.",
				"evidence_excerpt":  "Mina spoke to Rowan",
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
	if len(items) != 2 {
		t.Fatalf("kept subjective memories = %d, want default-private and explicit-public items: %#v", len(items), items)
	}
	private := mapFromAny(items[0])
	if stringFromMap(private, "owner_visibility") != "owner_private" ||
		stringFromMap(private, "target_reveal_policy") != "owner_private_until_revealed" ||
		stringFromMap(private, "portability") != "npc_private_recollection" {
		t.Fatalf("default-private memory was not conservatively normalized: %#v", private)
	}
	public := mapFromAny(items[1])
	if stringFromMap(public, "owner_entity_name") != "Rowan" {
		t.Fatalf("unexpected public memory kept: %#v", public)
	}
	if intFromAny(trace["quarantined_count"], 0) != 0 {
		t.Fatalf("quarantine trace = %#v", trace)
	}

	normalized := normalizeSubjectiveEntityMemories(items)
	if len(normalized) != 2 {
		t.Fatalf("normalized public memories = %#v", normalized)
	}
	got := mapFromAny(normalized[1])
	if stringFromMap(got, "owner_visibility") != "player_known" ||
		stringFromMap(got, "target_reveal_policy") != "" ||
		stringFromMap(got, "portability") != "portable_subjective_entity_recollection" {
		t.Fatalf("explicit public NPC was promoted to private: %#v", got)
	}
}

func TestCriticSubjectiveCollectionPreservesStorySpecificRevealPolicy(t *testing.T) {
	payload := map[string]any{
		"subjective_entity_memories": []any{map[string]any{
			"owner_entity_name":    "Mina",
			"owner_visibility":     "owner_private",
			"memory_text":          "Mina remembers opening the door.",
			"evidence_excerpt":     "Mina opened the door",
			"target_reveal_policy": "reveal_whenever_convenient",
		}},
	}
	filtered, _ := quarantineCriticProtectedCandidates(payload, "", "Mina opened the door.")
	items := sliceFromAny(filtered["subjective_entity_memories"])
	if len(items) != 1 || stringFromMap(mapFromAny(items[0]), "target_reveal_policy") != "reveal_whenever_convenient" {
		t.Fatalf("story-specific reveal policy was deleted during collection: %#v", filtered)
	}
}

func TestCriticPromptRequiresEvidenceEligibleSubjectiveCoverageAndAllowsValidZero(t *testing.T) {
	prompt := buildCompleteTurnCriticPrompt("session", 3, "Mina opens the door.", "Rowan watches.", nil, nil, nil)
	for _, required := range []string{
		"Before leaving subjective_entity_memories empty, inspect every named in-story entity",
		"valid when the latest accepted turn contains no distinct source-grounded perspective content",
		"NPC coverage is evidence-eligible, not mandatory",
		"Each subjective memory needs an owner and memory text",
		"Extract useful source-grounded in-story facts and relationships broadly as kg_triples",
		"Do not omit a fact merely because another typed lane also records it",
		"evidence_excerpts are durable citations, not transcript samples",
		"Speech-style examples belong in voice_observations",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("critic prompt missing subjective-memory contract %q", required)
		}
	}
	if strings.Contains(prompt, "Generic entity-to-entity KG edges are review-only") ||
		strings.Contains(prompt, "emit only source-bound entity-to-scalar") ||
		strings.Contains(prompt, "Every kg_triples item requires semantic_class") ||
		strings.Contains(prompt, "subject_binding/object_binding") ||
		strings.Contains(prompt, "Represent each record with subject, predicate, object") {
		t.Fatalf("critic prompt still contains the KG suppression policy")
	}
}

func TestCriticNormalizationCanonicalizesPendingThreadAliasesThroughPersistence(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"pending_threads": []any{map[string]any{
			"thread_type": "Open Questions",
			"title":       "Who opened the gate?",
			"confidence":  0.8,
		}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "pending-alias", 4, extraction, "Who opened the gate?", completeTurnEmbeddingConfig{}, time.Unix(400, 0))
	if result.PendingThreads != 1 || len(fake.savedPendingThreads) != 1 {
		t.Fatalf("canonical pending thread was not persisted: result=%#v saved=%#v", result, fake.savedPendingThreads)
	}
	if fake.savedPendingThreads[0].ThreadType != "open_question" || fake.savedPendingThreads[0].HookType != "open_question" {
		t.Fatalf("pending thread enum was not canonicalized: %#v", fake.savedPendingThreads[0])
	}
}

func TestCriticEnricherKeepsStoryClockEvidenceWithoutPromotingVoiceSamples(t *testing.T) {
	storyEvidence := "At dawn, the seventh bell rang."
	voiceEvidence := `Mina said, "Please wait here."`
	extraction := normalizeCriticExtraction(map[string]any{
		"evidence_excerpts": []any{"rewritten evidence not in source"},
		"story_clock": map[string]any{
			"version": "story_clock.v1", "observation_kind": "partial", "scene_scope": "current",
			"precision": "partial", "partial": map[string]any{"daypart": "dawn"},
			"evidence_excerpt": storyEvidence, "transition": "set",
		},
		"voice_observations": []any{
			map[string]any{"evidence_excerpt": voiceEvidence},
			map[string]any{"evidence_excerpt": "Mina politely asked everyone to wait."},
		},
	})
	enriched := enrichNormalizedCriticExtractionForFocusedRecall(extraction, "", storyEvidence+" "+voiceEvidence, 8)
	got := stringsFromAny(enriched["evidence_excerpts"])
	if len(got) != 1 || got[0] != storyEvidence {
		t.Fatalf("direct nested evidence = %#v", got)
	}
	voices := sliceFromAny(enriched["voice_observations"])
	if len(voices) != 2 || stringFromMap(mapFromAny(voices[0]), "evidence_excerpt") != voiceEvidence {
		t.Fatalf("voice observations lost their own evidence = %#v", voices)
	}
	if enriched["focused_recall_fallback"] != nil {
		t.Fatalf("exact nested evidence incorrectly triggered fallback: %#v", enriched["focused_recall_fallback"])
	}
}

func TestCriticBeliefTransferCreatesGroundedSubjectiveMemoryPerNamedListener(t *testing.T) {
	excerpt := "Mira told Rowan and Jules that Rowan is captain."
	normalized := normalizeCriticExtraction(map[string]any{
		"belief_updates": []any{map[string]any{
			"subject": "Rowan", "slot": "role", "claim": "captain",
			"speaker": "Mira", "listener_names": []any{"Rowan", "Jules"},
			"epistemic_state": "known", "acquisition_mode": "heard", "evidence": excerpt,
		}},
	})
	beliefs := sliceFromAny(normalized["belief_updates"])
	if len(beliefs) != 1 {
		t.Fatalf("normalized beliefs = %#v", beliefs)
	}
	belief := mapFromAny(beliefs[0])
	if stringFromMap(belief, "state_slot") != "role" || stringFromMap(belief, "value") != "captain" ||
		stringFromMap(belief, "speaker_name") != "Mira" || stringFromMap(belief, "evidence_excerpt") != excerpt {
		t.Fatalf("belief transfer aliases were not canonicalized: %#v", belief)
	}
	memories := sliceFromAny(normalized["subjective_entity_memories"])
	if len(memories) != 2 {
		t.Fatalf("listener subjective memories = %#v", memories)
	}
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	result := srv.saveCriticExtractionArtifacts(context.Background(), "belief-transfer", 6, normalized, "The room quieted. "+excerpt+" Then the bell rang.", completeTurnEmbeddingConfig{}, time.Unix(600, 0))
	if result.SubjectiveEntityMemories != 2 || len(fake.savedEntityMemories) != 2 {
		t.Fatalf("grounded listener memories were not persisted: result=%#v saved=%#v", result, fake.savedEntityMemories)
	}
	owners := map[string]bool{}
	for _, memory := range fake.savedEntityMemories {
		owners[memory.OwnerEntityName] = true
		if memory.EvidenceExcerpt != excerpt || memory.MemoryText != excerpt {
			t.Fatalf("listener memory was not exact-source grounded: %#v", memory)
		}
	}
	if !owners["Rowan"] || !owners["Jules"] {
		t.Fatalf("named listener coverage = %#v", owners)
	}
}

func TestCriticBeliefWithNamedKnowledgeHoldersCreatesSubjectiveMemoryWithoutSpeakerField(t *testing.T) {
	excerpt := "Rowan and Jules learned that Rowan is captain."
	normalized := normalizeCriticExtraction(map[string]any{
		"belief_updates": []any{map[string]any{
			"subject": "Rowan", "state_slot": "role", "value": "captain",
			"listener_names": []any{"Rowan", "Jules"}, "epistemic_state": "known",
			"evidence_excerpt": excerpt,
		}},
	})
	if got := len(sliceFromAny(normalized["belief_updates"])); got != 1 {
		t.Fatalf("source-grounded belief proposal was unexpectedly removed: %#v", normalized["belief_updates"])
	}
	if got := len(sliceFromAny(normalized["subjective_entity_memories"])); got != 2 {
		t.Fatalf("named knowledge holders did not receive subjective recollections: %#v", normalized["subjective_entity_memories"])
	}
}

func TestCriticObjectiveOnlyTurnDoesNotFabricateSubjectiveMemory(t *testing.T) {
	normalized := normalizeCriticExtraction(map[string]any{
		"state_claims": []any{map[string]any{
			"subject": "gate", "state_slot": "access", "value": "open",
			"evidence_excerpt": "The gate is open.",
		}},
	})
	if got := len(sliceFromAny(normalized["subjective_entity_memories"])); got != 0 {
		t.Fatalf("objective-only turn fabricated %d subjective memories: %#v", got, normalized["subjective_entity_memories"])
	}
}

func TestCriticWorldRulePersistenceOmitsExactUnchangedRepeat(t *testing.T) {
	fake := &turnRecordingStore{returnWorldRules: []store.WorldRule{{
		ChatSessionID: "world-repeat", Scope: "system", Category: "access",
		Key: "gate_requires_seal", ValueJSON: `"Gate access requires a seal."`, SourceTurn: 2,
	}}}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"world_rules": []any{map[string]any{
			"scope": "system", "category": "access", "key": "gate_requires_seal", "value": "Gate access requires a seal.",
		}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "world-repeat", 3, extraction, "The party approaches the gate.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.WorldRules != 0 || len(fake.savedWorldRules) != 0 {
		t.Fatalf("unchanged world rule was written again: result=%#v saved=%#v", result, fake.savedWorldRules)
	}
	found := false
	for _, skip := range result.SkipReasons {
		if skip["surface"] == "world_rules" && skip["reason"] == "unchanged_existing_rule" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unchanged rule omission was not traced: %#v", result.SkipReasons)
	}
}

type criticWorldRuleReadFailingStore struct {
	*turnRecordingStore
}

func (s *criticWorldRuleReadFailingStore) ListWorldRules(context.Context, string) ([]store.WorldRule, error) {
	return nil, errors.New("world-rule read unavailable")
}

func TestCriticWorldRulePersistenceKeepsCandidateWhenExistingRulesCannotBeRead(t *testing.T) {
	base := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = &criticWorldRuleReadFailingStore{turnRecordingStore: base}
	extraction := normalizeCriticExtraction(map[string]any{
		"world_rules": []any{map[string]any{
			"scope": "system", "category": "access", "key": "gate_requires_seal", "value": "Gate access requires a seal.",
		}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "world-read-failure", 3, extraction, "The gate requires a seal.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.WorldRules != 1 || len(base.savedWorldRules) != 1 {
		t.Fatalf("world rule candidate was dropped because duplicate lookup failed: result=%#v saved=%#v", result, base.savedWorldRules)
	}
	if !containsString(result.Warnings, "world_rule_existing_read_failed") {
		t.Fatalf("world-rule read failure was not surfaced: %#v", result.Warnings)
	}
	found := false
	for _, skip := range result.SkipReasons {
		if skip["surface"] == "world_rules" && skip["reason"] == "existing_world_rules_read_failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("world-rule read failure was not traced: %#v", result.SkipReasons)
	}
}

func TestCriticPromptReceivesActiveWorldRuleKeysWithoutSuppressedRows(t *testing.T) {
	fake := &turnRecordingStore{returnWorldRules: []store.WorldRule{
		{Scope: "system", Category: "access", Key: "gate_requires_seal", ValueJSON: `"Gate access requires a seal."`, SourceTurn: 2},
		{Scope: "system", Category: "access", Key: "obsolete_gate_rule", ValueJSON: `"obsolete"`, Suppressed: true},
	}}
	srv := NewServer(config.Default())
	srv.Store = fake
	active, trace := srv.buildCompleteTurnActiveWorldRuleInput(context.Background(), "world-prompt")
	if trace["status"] != "ok" || len(active) != 1 || stringFromMap(active[0], "key") != "gate_requires_seal" {
		t.Fatalf("active world-rule input = %#v trace=%#v", active, trace)
	}
	prompt := buildCompleteTurnCriticPrompt("world-prompt", 3, "Approach the gate.", "The guard checks the seal.", nil, nil, nil, map[string]any{"active_world_rules": active})
	for _, expected := range []string{"gate_requires_seal", "reuse its exact scope, scope_name, category, and key", "omit that unchanged repeat"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("critic prompt missing active world-rule contract %q", expected)
		}
	}
	if strings.Contains(prompt, "obsolete_gate_rule") {
		t.Fatalf("suppressed world rule leaked into prompt: %s", prompt)
	}
}

func TestCriticPromptKeepsReversibleStateCollectionVocabularyOpen(t *testing.T) {
	prompt := buildCompleteTurnCriticPrompt("session", 3, "Mina opens the door.", "Rowan watches.", nil, nil, nil)
	for _, required := range []string{
		"reversible_states, physical_conditions, entity_conditions, state_deltas, and character_deltas may all preserve source-grounded continuity observations",
		"Saving broad observations is separate from deciding which value is current or injectable",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("critic prompt missing reversible-state sensitivity contract %q", required)
		}
	}
}

func TestCriticSystemPromptKeepsReversibleStateCollectionVocabularyOpen(t *testing.T) {
	prompt, source := readCriticSystemPrompt(filepath.Join("..", "..", "..", "prompts"))
	if source == "fallback_builtin" {
		t.Fatal("source critic_system.txt was not loaded")
	}
	for _, required := range []string{
		"Preserve the condition, subject, change, uncertainty, visibility, and sensitivity",
		"without forcing a fixed vocabulary at collection time",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("critic system prompt missing reversible-state sensitivity contract %q", required)
		}
	}
}

func TestCriticProtectedCollectionDoesNotUseContentSimilarityAsSaveGate(t *testing.T) {
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
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 1 {
		t.Fatalf("structurally complete protected secret was dropped: %#v", filtered["protected_secrets"])
	}
	if got := len(sliceFromAny(filtered["subjective_entity_memories"])); got != 1 {
		t.Fatalf("structurally complete subjective memory was dropped: %#v", filtered["subjective_entity_memories"])
	}
	reasons := mapFromAny(trace["reasons"])
	if intFromAny(reasons["protected_secret_claim_unbound"], 0) != 0 ||
		intFromAny(reasons["protected_subjective_claim_unbound"], 0) != 0 {
		t.Fatalf("content-similarity save gate returned: %#v", trace)
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

func TestArchivedPrivateCandidateStillQuarantinesObjectiveDuplicate(t *testing.T) {
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
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 1 {
		t.Fatalf("structurally complete private candidate was deleted during collection: %#v", filtered["protected_secrets"])
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

func TestArchivedUnboundPublicIdentityStillQuarantinesObjectiveDuplicate(t *testing.T) {
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
	if got := len(sliceFromAny(filtered["character_identity_accuracy"])); got != 1 {
		t.Fatalf("structurally complete identity candidate was deleted during collection: %#v", filtered["character_identity_accuracy"])
	}
	if got := len(sliceFromAny(filtered["kg_triples"])); got != 0 {
		t.Fatalf("rejected false public identity leaked into objective KG: %#v", filtered["kg_triples"])
	}
	if intFromAny(trace["objective_lane_quarantined_count"], 0) != 1 {
		t.Fatalf("rejected public identity objective quarantine was not traced: %#v", trace)
	}
}
