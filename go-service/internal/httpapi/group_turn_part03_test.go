package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCompleteTurnSubjectiveEntityMemoriesAutoSaveByOwner(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	content := "Siwoo noticed the warning sign in Exit 2. Asuna thought Siwoo was hiding fear from her."
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "",
		"importance_score":  7,
		"evidence_excerpts": []any{},
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_key":     "siwoo",
				"owner_entity_name":    "Siwoo",
				"owner_entity_role":    "protagonist",
				"owner_visibility":     "player_known",
				"memory_text":          "Siwoo remembers the Exit 2 hallway as a warning sign.",
				"source_turn_index":    2,
				"importance_10":        7,
				"emotional_weight":     0.6,
				"evidence_excerpt":     "Exit 2",
				"target_reveal_policy": "requires_explicit_attachment",
			},
			map[string]any{
				"owner_entity_key":  "asuna",
				"owner_entity_name": "Asuna",
				"owner_entity_role": "npc",
				"memory_text":       "Asuna privately believes Siwoo is hiding fear from her.",
				"importance_10":     6,
				"emotional_weight":  0.7,
				"evidence_excerpt":  "hiding fear",
				"secret_guard":      true,
			},
		},
		"persona_capsule_candidates": []any{},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-subjective", 2, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(1200, 0))
	if result.SubjectiveEntityMemories != 2 {
		t.Fatalf("SubjectiveEntityMemories = %d, want 2; result=%#v", result.SubjectiveEntityMemories, result)
	}
	if len(fake.savedEntityMemories) != 2 {
		t.Fatalf("savedEntityMemories = %d, want 2: %#v", len(fake.savedEntityMemories), fake.savedEntityMemories)
	}
	siwoo := fake.savedEntityMemories[0]
	asuna := fake.savedEntityMemories[1]
	if siwoo.OwnerEntityKey != "siwoo" || siwoo.OwnerEntityName != "Siwoo" || siwoo.OwnerEntityRole != "protagonist" || siwoo.OwnerVisibility != "player_known" {
		t.Fatalf("Siwoo subjective memory owner mismatch: %+v", siwoo)
	}
	if siwoo.SourceChatSessionID != "sess-subjective" || siwoo.SourceTurn != 2 || siwoo.Importance10 != 7 || siwoo.EmotionalWeight != 0.6 {
		t.Fatalf("Siwoo subjective memory source/score mismatch: %+v", siwoo)
	}
	if asuna.OwnerEntityKey != "asuna" || asuna.OwnerEntityName != "Asuna" || asuna.OwnerEntityRole != "npc" || asuna.OwnerVisibility != "owner_private" {
		t.Fatalf("Asuna subjective memory owner/private mismatch: %+v", asuna)
	}
	if !asuna.SecretGuard || asuna.TargetRevealPolicy != "owner_private_until_revealed" || asuna.Portability != "npc_private_recollection" {
		t.Fatalf("Asuna private policy mismatch: %+v", asuna)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedEvidence) != 0 {
		t.Fatalf("subjective memories must not create canonical artifacts: mem=%d kg=%d evi=%d", len(fake.savedMemories), len(fake.savedKGTriples), len(fake.savedEvidence))
	}
}

func TestCompleteTurnSubjectiveEntityMemoryDuplicateSkipped(t *testing.T) {
	memoryText := "Siwoo remembers that the same hallway becomes dangerous when revisited."
	fake := &turnRecordingStore{
		returnEntityMemories: []store.ProtagonistEntityMemory{{
			OwnerEntityKey:      "siwoo",
			OwnerEntityName:     "Siwoo",
			OwnerEntityRole:     "protagonist",
			OwnerVisibility:     "player_known",
			SourceChatSessionID: "sess-subjective",
			SourceTurn:          3,
			MemoryText:          memoryText,
		}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_key":  "siwoo",
				"owner_entity_name": "Siwoo",
				"memory_text":       memoryText,
				"source_turn_index": 3,
			},
		},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-subjective", 3, extraction, "The same hallway becomes dangerous when revisited.", completeTurnEmbeddingConfig{}, time.Unix(1300, 0))
	if result.SubjectiveEntityMemories != 0 || len(fake.savedEntityMemories) != 0 {
		t.Fatalf("duplicate subjective memory should be skipped, result=%#v saved=%#v", result, fake.savedEntityMemories)
	}
	var foundDuplicate bool
	for _, skip := range result.SkipReasons {
		if skip["surface"] == "subjective_entity_memories" && skip["reason"] == "duplicate_source_turn_owner_memory" {
			foundDuplicate = true
			break
		}
	}
	if !foundDuplicate {
		t.Fatalf("missing duplicate skip reason: %#v", result.SkipReasons)
	}
}

func TestCompleteTurnSubjectiveEntityMemoryCanonicalizesOwnerAliases(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{{CharacterName: "\uc774\uc2dc\uc6b0"}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_key":  "siwoo",
				"owner_entity_name": "Siwoo",
				"memory_text":       "Siwoo privately remembers that Exit 2 felt unsafe.",
				"source_turn_index": 4,
			},
		},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-subjective", 4, extraction, "Exit 2 felt unsafe.", completeTurnEmbeddingConfig{}, time.Unix(1400, 0))
	if result.SubjectiveEntityMemories != 1 || len(fake.savedEntityMemories) != 1 {
		t.Fatalf("expected one canonical subjective memory, result=%#v saved=%#v", result, fake.savedEntityMemories)
	}
	saved := fake.savedEntityMemories[0]
	if saved.OwnerEntityKey != "siwoo" || saved.PersonaEntityKey != "siwoo" {
		t.Fatalf("owner alias key was not canonicalized: %+v", saved)
	}
	if saved.OwnerEntityName != "\uc774\uc2dc\uc6b0" || saved.PersonaEntityName != "\uc774\uc2dc\uc6b0" {
		t.Fatalf("owner display name should use canonical character state: %+v", saved)
	}
	for _, needle := range []string{"owner_entity_alias:Siwoo", "entity_alias_canonicalized", "raw_owner_entity_name:Siwoo"} {
		if !strings.Contains(saved.TagsJSON, needle) {
			t.Fatalf("canonical alias tag %q missing from %s", needle, saved.TagsJSON)
		}
	}
}

func TestSaveCriticExtractionArtifactsNormalizesMultilingualEntityAliases(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{{CharacterName: "Mina"}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina promised Rowan she would return.",
		"importance_score":  6,
		"evidence_excerpts": []any{"Mina promised Rowan she would return."},
		"entities": map[string]any{
			"characters": []any{map[string]any{
				"name":        "\uBBFC\uC544",
				"aliases":     []any{"Mina", "\uBBFC\uC544"},
				"description": "returning ally",
			}},
		},
		"kg_triples": []any{map[string]any{"subject": "\uBBFC\uC544", "predicate": "promised", "object": "Rowan"}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-multi", 4, extraction, "Mina promised Rowan she would return.", completeTurnEmbeddingConfig{}, time.Unix(100, 0))
	if result.Entities != 1 || result.KGTriples != 1 {
		t.Fatalf("expected entity and KG saves, result=%#v entities=%#v kg=%#v", result, fake.savedEntities, fake.savedKGTriples)
	}
	if len(fake.savedEntities) != 1 || fake.savedEntities[0].Name != "Mina" {
		t.Fatalf("expected Korean entity name to canonicalize to Mina, got %#v", fake.savedEntities)
	}
	if !strings.Contains(fake.savedEntities[0].AliasesJSON, "Mina") || !strings.Contains(fake.savedEntities[0].AliasesJSON, "\uBBFC\uC544") {
		t.Fatalf("expected aliases to preserve display variants, got %#v", fake.savedEntities[0])
	}
	if len(fake.savedKGTriples) != 1 || fake.savedKGTriples[0].Subject != "Mina" || fake.savedKGTriples[0].Object != "Rowan" {
		t.Fatalf("expected KG subject to use canonical display name, got %#v", fake.savedKGTriples)
	}
}

func TestSaveCriticExtractionArtifactsGuardsTransientDescriptorsAndParticipants(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina and Rowan made a plan while a red-haired girl watched.",
		"importance_score":  5,
		"evidence_excerpts": []any{"Mina and Rowan made a plan while a red-haired girl watched."},
		"state_deltas": map[string]any{
			"relationship_changes": []any{
				map[string]any{"pair": "Mina <-> Rowan", "detail": "They trust each other more."},
				map[string]any{"pair": "Mina <-> {{user}}", "detail": "The participant placeholder should not persist."},
			},
		},
		"character_deltas": []any{
			map[string]any{"name": "red-haired girl", "status": map[string]any{"emotion": "curious"}, "events": []any{map[string]any{"type": "sighting", "detail": "She watched from the door."}}},
			map[string]any{"name": "Mina", "events": []any{map[string]any{"type": "relationship_shift", "detail": "Mina trusted Rowan more."}}},
		},
		"pending_threads": []any{map[string]any{"thread_type": "promise", "title": "Mina asks Rowan about the plan", "owner": "{{user}}", "target": "Mina"}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-scrub", 5, extraction, "Mina and Rowan made a plan while a red-haired girl watched.", completeTurnEmbeddingConfig{}, time.Unix(200, 0))
	if result.CharacterStates != 1 || len(fake.savedCharacterStates) != 1 || fake.savedCharacterStates[0].CharacterName != "Mina" {
		t.Fatalf("expected only anchored Mina character state, result=%#v states=%#v", result, fake.savedCharacterStates)
	}
	if result.CharacterEvents != 1 || len(fake.savedCharacterEvents) != 1 || fake.savedCharacterEvents[0].CharacterName != "Mina" {
		t.Fatalf("expected only anchored Mina character event, result=%#v events=%#v", result, fake.savedCharacterEvents)
	}
	var stateDeltas string
	for _, item := range fake.savedActiveStates {
		if item.StateType == "state_deltas" {
			stateDeltas = item.Content
			break
		}
	}
	if !strings.Contains(stateDeltas, "Mina") || !strings.Contains(stateDeltas, "Rowan") || strings.Contains(stateDeltas, "{{user}}") {
		t.Fatalf("expected safe pair-only relationship delta and no participant placeholder, state_deltas=%s", stateDeltas)
	}
	if len(fake.savedPendingThreads) != 1 || fake.savedPendingThreads[0].Owner != "" || fake.savedPendingThreads[0].Target != "Mina" {
		t.Fatalf("expected pending thread participant owner scrubbed and safe target kept, got %#v", fake.savedPendingThreads)
	}
}

func TestSaveCriticExtractionArtifactsAppliesSoftPrune(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 11, ChatSessionID: "sess-prune", TurnIndex: 1, SummaryJSON: `{"turn_summary":"obsolete clue should fade"}`, Importance: 0.7},
			{ID: 12, ChatSessionID: "sess-prune", TurnIndex: 1, SummaryJSON: `{"turn_summary":"fresh clue remains"}`, Importance: 0.8},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "The obsolete clue was superseded.",
		"importance_score":  5,
		"evidence_excerpts": []any{"The obsolete clue was superseded."},
		"prune_targets":     []any{"obsolete clue"},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-prune", 6, extraction, "The obsolete clue was superseded.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.Errors != 0 {
		t.Fatalf("soft prune should not error, result=%#v", result)
	}
	if got := fake.updatedImportance[11]; got < 0.499 || got > 0.501 {
		t.Fatalf("expected memory 11 importance to be softly pruned to 0.5, got %.2f updates=%#v", got, fake.updatedImportance)
	}
	if _, exists := fake.updatedImportance[12]; exists {
		t.Fatalf("memory 12 should not be pruned, updates=%#v", fake.updatedImportance)
	}
	foundAudit := false
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "soft_prune" && item.Source == "critic" && strings.Contains(item.DetailsJSON, "obsolete clue") {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected soft_prune audit log, got %#v", fake.savedAuditLogs)
	}
	foundResolution := false
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "supersession_resolution" && item.TargetType == "memory" && item.TargetID == 11 && strings.Contains(item.DetailsJSON, "stale_demote") {
			foundResolution = true
			break
		}
	}
	if !foundResolution {
		t.Fatalf("expected supersession_resolution stale_demote audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestCompleteTurnMemorySemanticDedupSkipsInsertAndReinforcesExisting(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 42, ChatSessionID: "sess-dedup", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina promised Rowan she would return with the brass key."}`, Importance: 0.4},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina promised Rowan she would return with the brass key.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Mina promised Rowan she would return with the brass key."},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-dedup", 6, extraction, "Mina promised Rowan she would return with the brass key.", completeTurnEmbeddingConfig{}, time.Unix(310, 0))
	if result.Errors != 0 {
		t.Fatalf("semantic dedup should not error, result=%#v", result)
	}
	if len(fake.savedMemories) != 0 || result.Memories != 0 {
		t.Fatalf("expected duplicate memory insert to be skipped, result=%#v saved=%#v", result, fake.savedMemories)
	}
	if got := fake.updatedImportance[42]; got < 0.79 || got > 0.81 {
		t.Fatalf("expected existing memory importance reinforced to 0.8, got %.2f updates=%#v", got, fake.updatedImportance)
	}
	if !containsString(result.Warnings, "memory_semantic_dedup_merged") {
		t.Fatalf("expected memory_semantic_dedup_merged warning, got %#v", result.Warnings)
	}
	foundAudit := false
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "memory_semantic_dedup" && item.Source == "critic" && strings.Contains(item.DetailsJSON, `"merged_memory_id":42`) {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected memory_semantic_dedup audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestCompleteTurnMemorySameTurnRegenerationSkipsDuplicateInsert(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 77, ChatSessionID: "sess-turn-dupe", TurnIndex: 5, SummaryJSON: `{"turn_summary":"Luna explains that apostles borrow divine power to defeat monsters."}`, Importance: 0.7},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Luna recounts the church doctrine about apostles and monsters.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Apostles borrow divine power to defeat monsters."},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-turn-dupe", 5, extraction, "Luna explains apostles and monsters.", completeTurnEmbeddingConfig{}, time.Unix(315, 0))
	if result.Errors != 0 {
		t.Fatalf("same-turn duplicate guard should not error, result=%#v", result)
	}
	if len(fake.savedMemories) != 0 || result.Memories != 0 {
		t.Fatalf("same-turn regeneration inserted duplicate memory, result=%#v saved=%#v", result, fake.savedMemories)
	}
	foundSkip := false
	for _, item := range result.SkipReasons {
		row := mapFromAny(item)
		if stringFromMap(row, "surface") == "memories" && stringFromMap(row, "reason") == "duplicate_source_turn_memory" {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Fatalf("duplicate_source_turn_memory skip reason missing: %#v", result.SkipReasons)
	}
}

func TestSaveCriticExtractionArtifactsRespectsPrunePolicyOff(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 11, ChatSessionID: "sess-prune", TurnIndex: 1, SummaryJSON: `{"turn_summary":"obsolete clue should fade"}`, Importance: 0.7},
		},
	}
	cfg := config.Default()
	cfg.PrunePolicy = "off"
	srv := NewServer(cfg)
	srv.Store = fake

	extraction := map[string]any{
		"turn_summary":      "The obsolete clue was superseded.",
		"importance_score":  5,
		"evidence_excerpts": []any{"The obsolete clue was superseded."},
		"prune_targets":     []any{"obsolete clue"},
	}
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-prune", 6, extraction, "The obsolete clue was superseded.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.Errors != 0 {
		t.Fatalf("prune policy off should not error, result=%#v", result)
	}
	if len(fake.updatedImportance) != 0 {
		t.Fatalf("expected no importance update when prune policy is off, got %#v", fake.updatedImportance)
	}
	if !containsString(result.Warnings, "soft_prune_disabled") {
		t.Fatalf("expected soft_prune_disabled warning, got %#v", result.Warnings)
	}
}

func TestConfigUpdateRuntimeSettingsFeedCompleteTurnCritic(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Runtime config critic extracted one durable memory.",
		"importance_score":  7,
		"evidence_excerpts": []any{"The runtime-configured critic is active."},
		"kg_triples":        []any{map[string]any{"subject": "Critic", "predicate": "extracts", "object": "Memory"}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "runtime-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	var capturedUpstreamReq map[string]any
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&capturedUpstreamReq); err != nil {
			t.Fatalf("decode upstream critic request: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	updateBody := `{"criticApiKey":"sk-runtime","criticEndpoint":"https://api.example.com/v1","criticModel":"runtime-critic","criticProvider":"openai","criticTimeout":45,"criticReasoningPreset":"glm","criticReasoningEffort":"enable","criticReasoningBudgetTokens":2048}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}
	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update: %v", err)
	}
	runtimeTrace, ok := updateResp["runtime_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_config_trace missing from config/update: %+v", updateResp)
	}
	criticConfigTrace, ok := runtimeTrace["critic"].(map[string]any)
	if !ok || criticConfigTrace["configured"] != true {
		t.Fatalf("critic runtime config trace not configured: %+v", runtimeTrace["critic"])
	}
	if criticConfigTrace["reasoning_preset"] != "glm" || criticConfigTrace["reasoning_effort"] != "enable" || criticConfigTrace["reasoning_budget_tokens"] != float64(2048) || criticConfigTrace["glm_thinking_type"] != "enabled" {
		t.Fatalf("critic runtime config trace missing reasoning settings: %+v", criticConfigTrace)
	}

	turnBody := `{"chat_session_id":"sess-runtime-config","turn_index":1,"user_input":"critic should run","assistant_content":"The runtime-configured critic is active.","request_type":"model"}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(turnBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["critic_triggered"] != true {
		t.Fatalf("critic_triggered = %v, want true with runtime config", resp["critic_triggered"])
	}
	llmTrace, ok := resp["llm_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("llm_config_trace missing from complete-turn response: %+v", resp)
	}
	criticTrace, ok := llmTrace["critic"].(map[string]any)
	if !ok || criticTrace["configured"] != true {
		t.Fatalf("critic complete-turn trace not configured: %+v", llmTrace["critic"])
	}
	if criticTrace["reasoning_preset"] != "glm" || criticTrace["reasoning_effort"] != "enable" || criticTrace["reasoning_budget_tokens"] != float64(2048) || criticTrace["glm_thinking_type"] != "enabled" {
		t.Fatalf("critic complete-turn trace missing reasoning settings: %+v", criticTrace)
	}
	thinking, _ := capturedUpstreamReq["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Fatalf("upstream critic request missing GLM thinking payload: %+v", capturedUpstreamReq)
	}
	if got, _ := resp["derived_artifacts_saved"].(float64); got < 3 {
		t.Fatalf("derived_artifacts_saved = %v, want at least memory/evidence/KG", resp["derived_artifacts_saved"])
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 {
		t.Fatalf("expected runtime-config critic artifacts, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestFocusedRecallFallbackAddsGroundedSummaryAndEvidence(t *testing.T) {
	extraction := enrichNormalizedCriticExtractionForFocusedRecall(normalizeCriticExtraction(map[string]any{}),
		"Mina promises Rowan she will return with the brass key.",
		"Rowan accepts the promise and marks the cellar route as the only safe path.",
		12,
	)
	if strings.TrimSpace(extractionStringFromAny(extraction["turn_summary"])) == "" {
		t.Fatalf("turn_summary was not backfilled: %+v", extraction)
	}
	excerpts := stringsFromAny(extraction["evidence_excerpts"])
	if len(excerpts) == 0 {
		t.Fatalf("evidence_excerpts were not backfilled: %+v", extraction)
	}
	if !strings.Contains(strings.Join(excerpts, "\n"), "Mina promises Rowan") {
		t.Fatalf("fallback evidence is not grounded in latest turn: %#v", excerpts)
	}
}

func TestSaveCriticExtractionArtifactsSkipsDuplicateEvidenceAndKGForSameTurn(t *testing.T) {
	fake := &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{{
			ChatSessionID:   "sess-dupe-artifacts",
			EvidenceText:    "Mina promises Rowan she will return.",
			SourceTurnStart: 7,
			SourceTurnEnd:   7,
			TurnAnchor:      7,
		}},
		returnKGTriples: []store.KGTriple{{
			ChatSessionID: "sess-dupe-artifacts",
			Subject:       "Mina",
			Predicate:     "promises",
			Object:        "Rowan",
			SourceTurn:    7,
			ValidFrom:     7,
		}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina promises Rowan she will return.",
		"importance_score":  6,
		"evidence_excerpts": []any{"Mina promises Rowan she will return."},
		"kg_triples":        []any{map[string]any{"subject": "Mina", "predicate": "promises", "object": "Rowan", "valid_from": 7}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-dupe-artifacts", 7, extraction, "Mina promises Rowan she will return.", completeTurnEmbeddingConfig{}, time.Unix(700, 0))
	if result.Evidence != 0 || result.KGTriples != 0 {
		t.Fatalf("duplicate artifacts were saved, evidence=%d kg=%d", result.Evidence, result.KGTriples)
	}
	if len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("duplicate save calls occurred, evidence=%d kg=%d", len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestSaveCriticExtractionArtifactsSkipsFuzzyEvidenceAndActiveKGDuplicates(t *testing.T) {
	fake := &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{{
			ChatSessionID:   "sess-dupe-artifacts-fuzzy",
			EvidenceText:    "Mina promises Rowan she will return before dawn.",
			SourceTurnStart: 5,
			SourceTurnEnd:   9,
			TurnAnchor:      7,
		}},
		returnKGTriples: []store.KGTriple{{
			ChatSessionID: "sess-dupe-artifacts-fuzzy",
			Subject:       "Mina",
			Predicate:     "protects",
			Object:        "Rowan",
			SourceTurn:    3,
			ValidFrom:     3,
			ValidTo:       0,
		}},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina repeats her promise to return and protect Rowan.",
		"importance_score":  6,
		"evidence_excerpts": []any{"Mina promises Rowan she will return."},
		"kg_triples":        []any{map[string]any{"subject": "Mina", "predicate": "protects", "object": "Rowan", "valid_from": 7}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-dupe-artifacts-fuzzy", 7, extraction, "Mina promises Rowan she will return. Mina protects Rowan.", completeTurnEmbeddingConfig{}, time.Unix(701, 0))
	if result.Evidence != 0 || result.KGTriples != 0 {
		t.Fatalf("fuzzy/active duplicate artifacts were saved, evidence=%d kg=%d skip=%#v", result.Evidence, result.KGTriples, result.SkipReasons)
	}
	if len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("duplicate save calls occurred, evidence=%d kg=%d", len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCriticJudgedWorldRulesPersistKoreanInstitutionalRules(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "학생회는 기숙사 통금 시간을 밤 10시로 확정했고, 지각자는 다음 날 청소 당번을 맡아야 한다.",
		"importance_score":  7,
		"evidence_excerpts": []any{"학생회는 기숙사 통금 시간을 밤 10시로 확정했고, 지각자는 다음 날 청소 당번을 맡아야 한다."},
	})
	extraction["world_rules"] = []any{map[string]any{
		"scope":      "location",
		"scope_name": "dormitory",
		"category":   "institution",
		"key":        "dormitory_curfew_and_cleanup_penalty",
		"value":      "Dormitory curfew is 10 PM, and late students must clean the common room the next day.",
		"confidence": 0.86,
	}}
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-kr-world", 9, extraction, "학생회는 기숙사 통금 시간을 밤 10시로 확정했고, 지각자는 다음 날 청소 당번을 맡아야 한다.", completeTurnEmbeddingConfig{}, time.Unix(900, 0))
	if result.WorldRules == 0 || len(fake.savedWorldRules) == 0 {
		t.Fatalf("expected critic-judged institutional world rule to persist, result=%+v", result)
	}
}

func TestExplorerRegenerateMemoryUsesCompleteTurnArtifactPipeline(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-explorer-regen", TurnIndex: 4, Role: "user", Content: "Mina asks Rowan to guard the cellar route."},
			{ChatSessionID: "sess-explorer-regen", TurnIndex: 4, Role: "assistant", Content: "Rowan accepts and confirms the cellar route is the only safe path."},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Rowan accepts Mina's request and confirms the cellar route.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Rowan accepts and confirms the cellar route is the only safe path."},
		"kg_triples":        []any{map[string]any{"subject": "Rowan", "predicate": "guards", "object": "cellar route"}},
		"world_rules":       []any{map[string]any{"scope": "location", "scope_name": "cellar", "category": "access", "key": "cellar_route_only_safe_path", "value": "The cellar route is the only safe path."}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-test",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(chatResp))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-explorer-regen","turn_index":4,"client_meta":{"critic":{"api_key":"sk-test","endpoint":"https://api.example.com/v1","model":"critic-test","provider":"openai"}}}`
	req := httptest.NewRequest(http.MethodPost, "/explorer/memories/regenerate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, response: %+v", resp["status"], resp)
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 || len(fake.savedWorldRules) != 1 {
		t.Fatalf("regenerate did not save full artifact set: memories=%d evidence=%d kg=%d world=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples), len(fake.savedWorldRules))
	}
}

func TestCompleteTurnCriticLedgerWiringBehindFeatureFlag(t *testing.T) {
	run := func(t *testing.T, enabled bool) (string, map[string]any) {
		t.Helper()
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 77, ChatSessionID: "sess-ledger-wiring", TurnIndex: 3, SummaryJSON: `{"summary":"Mina already trusts Rowan after the bridge scene."}`, Importance: 8.5},
			},
			returnEvidence: []store.DirectEvidence{
				{ID: 88, ChatSessionID: "sess-ledger-wiring", EvidenceText: "<thinking>private draft</thinking>Rowan protected Mina at the bridge.", SourceTurnStart: 3, SourceTurnEnd: 3, CaptureVerification: "verified"},
			},
		}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeMariaDBAuthority
		cfg.CriticLedgerEnabled = enabled
		srv := NewServer(cfg)
		srv.Store = fake
		srv.StoreOpenError = nil

		extractionBytes, _ := json.Marshal(map[string]any{
			"turn_summary":      "Latest turn produced a small durable memory.",
			"importance_score":  5,
			"evidence_excerpts": []any{"Latest turn produced a small durable memory."},
		})
		chatResp, _ := json.Marshal(map[string]any{
			"model":   "critic-ledger-test",
			"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
		})

		oldClient := proxyHTTPClient
		var capturedPrompt string
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var captured map[string]any
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode upstream critic request: %v", err)
			}
			messages, _ := captured["messages"].([]any)
			if len(messages) < 2 {
				t.Fatalf("upstream messages missing: %+v", captured)
			}
			userMessage, _ := messages[1].(map[string]any)
			capturedPrompt, _ = userMessage["content"].(string)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(chatResp)),
			}, nil
		})}
		defer func() { proxyHTTPClient = oldClient }()

		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)

		body := `{"chat_session_id":"sess-ledger-wiring","turn_index":6,"user_input":"Mina asks what Rowan remembers.","assistant_content":"Rowan answers with a new visible reply.","request_type":"model","output_language_override":{"language":"ko"},"client_meta":{"critic":{"api_key":"sk-test","endpoint":"https://api.example.com/v1","model":"critic-ledger-test","provider":"openai"}}}`
		req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("complete-turn status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		traceHandoff, _ := resp["trace_handoff"].(map[string]any)
		criticTrace, _ := traceHandoff["critic_trace"].(map[string]any)
		return capturedPrompt, criticTrace
	}

	disabledPrompt, disabledTrace := run(t, false)
	if !strings.Contains(disabledPrompt, "<Critic_Archive_Ledger_JSON>") || !strings.Contains(disabledPrompt, "null") {
		t.Fatalf("disabled prompt should carry a null ledger section, prompt=%s", disabledPrompt)
	}
	if strings.Contains(disabledPrompt, "Mina already trusts Rowan") {
		t.Fatalf("disabled prompt leaked archive ledger content: %s", disabledPrompt)
	}
	if strings.Contains(disabledPrompt, "private draft") || strings.Contains(disabledPrompt, "<thinking>") {
		t.Fatalf("disabled prompt leaked private reasoning: %s", disabledPrompt)
	}
	disabledLedgerTrace, _ := disabledTrace["critic_archive_ledger"].(map[string]any)
	if disabledLedgerTrace["enabled"] != false || disabledLedgerTrace["included"] != false {
		t.Fatalf("disabled ledger trace mismatch: %+v", disabledLedgerTrace)
	}

	enabledPrompt, enabledTrace := run(t, true)
	if !strings.Contains(enabledPrompt, "<Critic_Archive_Ledger_JSON>") {
		t.Fatalf("enabled prompt missing ledger section: %s", enabledPrompt)
	}
	for _, expected := range []string{"critic_archive_ledger.v1", "Mina already trusts Rowan after the bridge scene.", "Rowan protected Mina at the bridge.", "raw_archive_dump_blocked"} {
		if !strings.Contains(enabledPrompt, expected) {
			t.Fatalf("enabled prompt missing %q: %s", expected, enabledPrompt)
		}
	}
	if strings.Contains(enabledPrompt, "private draft") || strings.Contains(enabledPrompt, "<thinking>") {
		t.Fatalf("enabled prompt leaked private reasoning: %s", enabledPrompt)
	}
	enabledLedgerTrace, _ := enabledTrace["critic_archive_ledger"].(map[string]any)
	if enabledLedgerTrace["enabled"] != true || enabledLedgerTrace["included"] != true || enabledLedgerTrace["llm_call_attempted"] != false || enabledLedgerTrace["write_attempted"] != false {
		t.Fatalf("enabled ledger trace mismatch: %+v", enabledLedgerTrace)
	}
	language, _ := enabledLedgerTrace["language"].(map[string]any)
	if language["assistant_final_language"] != "ko" {
		t.Fatalf("enabled ledger language mismatch: %+v", language)
	}
}

func TestImportHypamemoryRequiresCriticConfig(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-hypa-missing","summaries":[{"text":"Chloe remembers the rooftop promise.","is_important":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/import/hypamemory", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("import/hypamemory status = %d, want 200 fail-closed response: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import/hypamemory response: %v", err)
	}
	if resp["status"] != "error" || resp["code"] != "critic_config_missing" {
		t.Fatalf("expected critic_config_missing without runtime critic config, got %+v", resp)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedAuditLogs) != 0 {
		t.Fatalf("HypaMemory import must not fake writes without critic config, memories=%d evidence=%d kg=%d audit=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples), len(fake.savedAuditLogs))
	}
}

func TestImportHypamemoryWithRuntimeCriticSavesArtifacts(t *testing.T) {
	fake := &turnRecordingStore{}
	vec := &turnRecordingVectorStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Imported HypaMemory says Chloe trusts Hero after the rooftop promise.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Chloe trusts Hero after the rooftop promise."},
		"kg_triples":        []any{map[string]any{"subject": "Chloe", "predicate": "trusts", "object": "Hero"}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "runtime-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	updateBody := `{"criticApiKey":"sk-runtime","criticEndpoint":"https://api.example.com/v1","criticModel":"runtime-critic","criticProvider":"openai","embeddingApiKey":"sk-embed","embeddingEndpoint":"https://api.example.com/v1/embeddings","embeddingModel":"embed-model","embeddingProvider":"openai"}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	body := `{"chat_session_id":"sess-hypa-runtime","summaries":[{"text":"Chloe trusts Hero after the rooftop promise.","is_important":true,"category":"relationship","tags":["trust","rooftop"],"source_turn_index":-42}]}`
	req := httptest.NewRequest(http.MethodPost, "/import/hypamemory", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("import/hypamemory status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import/hypamemory response: %v", err)
	}
	if resp["status"] != "ok" || resp["succeeded"] != float64(1) || resp["failed"] != float64(0) {
		t.Fatalf("expected successful HypaMemory import, got %+v", resp)
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 {
		t.Fatalf("expected HypaMemory Critic artifacts, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	if fake.savedMemories[0].TurnIndex != -42 || fake.savedEvidence[0].SourceTurnStart != -42 || fake.savedKGTriples[0].SourceTurn != -42 {
		t.Fatalf("expected HypaMemory import artifacts to use negative import turn index, memory=%d evidence=%d kg=%d", fake.savedMemories[0].TurnIndex, fake.savedEvidence[0].SourceTurnStart, fake.savedKGTriples[0].SourceTurn)
	}
	if len(vec.docs) != 2 {
		t.Fatalf("expected memory and evidence vector upserts for imported memory, got %d", len(vec.docs))
	}
	if !hasAuditEvent(fake.savedAuditLogs, "hypamemory_import") {
		t.Fatalf("expected hypamemory_import audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestImportHypamemoryScoringPassRaisesLowCriticImportance(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	scoringBytes, _ := json.Marshal(map[string]any{
		"importance_10":               8.4,
		"retrieval_priority":          0.91,
		"continuity_weight":           0.88,
		"dialogue_or_sensory_density": 0.42,
		"memory_kind":                 "injury_continuity",
		"entity_relevance":            []any{"Hero", "Chloe"},
		"time_anchor_quality":         "summary_level",
		"keep_reason":                 "The imported memory changes how future danger should be interpreted.",
	})
	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Imported HypaMemory says Hero was shot before and Chloe took him to hospital.",
		"importance_score":  2,
		"evidence_excerpts": []any{"Hero was shot before and Chloe took him to hospital."},
		"kg_triples":        []any{map[string]any{"subject": "Hero", "predicate": "was_taken_to", "object": "hospital"}},
	})
	scoringResp, _ := json.Marshal(map[string]any{
		"model":   "runtime-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(scoringBytes)}}},
	})
	criticResp, _ := json.Marshal(map[string]any{
		"model":   "runtime-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	chatCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
			}, nil
		}
		chatCalls++
		body := scoringResp
		if chatCalls >= 2 {
			body = criticResp
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	updateBody := `{"criticApiKey":"sk-runtime","criticEndpoint":"https://api.example.com/v1","criticModel":"runtime-critic","criticProvider":"openai","embeddingApiKey":"sk-embed","embeddingEndpoint":"https://api.example.com/v1/embeddings","embeddingModel":"embed-model","embeddingProvider":"openai"}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	body := `{"chat_session_id":"sess-hypa-score","summaries":[{"text":"Hero was shot before and Chloe took him to hospital.","is_important":false,"category":"injury","source_turn_index":12}]}`
	req := httptest.NewRequest(http.MethodPost, "/import/hypamemory", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("import/hypamemory status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import/hypamemory response: %v", err)
	}
	if resp["status"] != "ok" || resp["scoring_policy"] != hypaMemoryImportScoringPolicyVersion {
		t.Fatalf("expected scored HypaMemory import response, got %+v", resp)
	}
	if chatCalls != 2 {
		t.Fatalf("expected scoring + critic chat calls, got %d", chatCalls)
	}
	if len(fake.savedMemories) != 1 {
		t.Fatalf("expected one memory, got %d", len(fake.savedMemories))
	}
	if fake.savedMemories[0].TurnIndex != -12 {
		t.Fatalf("expected positive source_turn_index to normalize to -12, got %d", fake.savedMemories[0].TurnIndex)
	}
	if fake.savedMemories[0].Importance < 0.84 {
		t.Fatalf("expected scoring pass to raise low critic importance to >=0.84, got %.3f", fake.savedMemories[0].Importance)
	}
	saved := map[string]any{}
	if err := json.Unmarshal([]byte(fake.savedMemories[0].SummaryJSON), &saved); err != nil {
		t.Fatalf("decode saved summary: %v", err)
	}
	if saved["hypamemory_import_score"] == nil || saved["importance_score"] != float64(8.4) {
		t.Fatalf("saved extraction missing scoring payload or raised importance: %+v", saved)
	}
}

func TestCompleteTurnCriticProviderFailurePreservesRawTurnAndReportsReason(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	const apiKey = "sk-critic-fail"
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad key sk-critic-fail"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-critic-provider-fail",
		"turn_index":        1,
		"user_input":        "Mina asks Rowan to remember the blue key.",
		"assistant_content": "Rowan promises to keep the blue key safe.",
		"client_meta": map[string]any{"critic": map[string]any{
			"api_key":  apiKey,
			"endpoint": "https://api.example.com/v1",
			"model":    "critic-model",
			"provider": "openai",
		}},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200 fail-open persistence: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), apiKey) {
		t.Fatalf("complete-turn response leaked API key: %s", rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["save_ok"] != true || resp["critic_triggered"] != false {
		t.Fatalf("expected raw save with critic failure, got %+v", resp)
	}
	if resp["chat_logs_saved"] != float64(2) || resp["derived_artifacts_saved"] != float64(0) {
		t.Fatalf("unexpected save counters: %+v", resp)
	}
	if len(fake.savedChatLogs) != 2 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("critic failure should preserve raw turn only, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	reasons, _ := resp["fail_reasons"].([]any)
	found := false
	for _, item := range reasons {
		if strings.Contains(fmt.Sprint(item), "critic_extract_failed") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected critic_extract_failed reason, got %+v", resp["fail_reasons"])
	}
	if !hasAuditEvent(fake.savedAuditLogs, "critic_extract_failed") {
		t.Fatalf("expected critic_extract_failed audit log, got %#v", fake.savedAuditLogs)
	}
}
