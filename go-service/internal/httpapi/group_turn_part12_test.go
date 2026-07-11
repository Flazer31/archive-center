package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// TestPrepareTurnRelationshipAndWorldStateTraceSurface (P486 HS-1i).
// Prepare-turn traces must expose counts/flags proving relationship/world current state
// was saved, read, and injected into the canon text.
func TestPrepareTurnRelationshipAndWorldStateTraceSurface(t *testing.T) {
	fake := &turnRecordingStore{
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-trace", LayerType: "relationship_state", Content: `{"bond_and_distance":"Mina trusts Rowan.","trust":0.9}`, SourceStateType: "relationship_state", TurnIndex: 30, SourceTurn: 30, LastVerifiedTurn: 30, Confidence: 0.92},
			{ID: 2, ChatSessionID: "sess-trace", LayerType: "world_state", Content: `{"rules":[{"key":"faction_north","value":"rising"}],"version":"world_state.v1","faction_status":{"north":"rising"}}`, SourceStateType: "world_state", TurnIndex: 30, SourceTurn: 30, LastVerifiedTurn: 30, Confidence: 0.88},
			{ID: 3, ChatSessionID: "sess-trace", LayerType: "world_state", Content: `{"rules":[],"confidence":0.4,"verification":"pending"}`, SourceStateType: "world_state", TurnIndex: 25, SourceTurn: 25, LastVerifiedTurn: 25, Confidence: 0.4},
		},
		returnChatLogs: []store.ChatLog{{ID: 1, ChatSessionID: "sess-trace", TurnIndex: 31, Role: "assistant", Content: "They pause."}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-trace","turn_index":32,"raw_user_input":"Continue","settings":{"max_injection_chars":900,"max_input_context_chars":300,"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	canonText, _ := injectionPack["canon_text"].(string)
	if !strings.Contains(canonText, "relationship_state") {
		t.Fatalf("canon_text missing relationship_state: %q", canonText)
	}
	if !strings.Contains(canonText, "world_state") {
		t.Fatalf("canon_text missing world_state: %q", canonText)
	}
	counts, ok := injectionPack["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}
	if counts["canonical_state_relationship_layers_count"] != float64(1) {
		t.Fatalf("canonical_state_relationship_layers_count = %v, want 1", counts["canonical_state_relationship_layers_count"])
	}
	if counts["canonical_state_world_layers_count"] != float64(1) {
		t.Fatalf("canonical_state_world_layers_count = %v, want 1", counts["canonical_state_world_layers_count"])
	}
	if counts["canonical_state_layers_filtered_count"] != float64(1) {
		t.Fatalf("canonical_state_layers_filtered_count = %v, want 1", counts["canonical_state_layers_filtered_count"])
	}
	bd, ok := injectionPack["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}
	if bd["canonical_state_hard_floor_enabled"] != true {
		t.Fatalf("canonical_state_hard_floor_enabled = %v, want true", bd["canonical_state_hard_floor_enabled"])
	}
}

func TestPrepareTurnTM1aCanonicalConsistencyRecallDocumentsSurface(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-tm1a", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A quiet scene in the archive room"}`, Importance: 0.9, PlaceWing: "North Wing", PlaceRoom: "Scene Room"},
		},
		returnCharStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-tm1a", CharacterName: "Mina", RelationshipsJSON: `{"Rowan":"trusts"}`},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-tm1a", ThreadKey: "thread_rooftop", Description: "Mina must answer Rowan about the plan", Status: "open", Title: "Rooftop plan", Owner: "Rowan", Target: "Mina"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-tm1a", TurnIndex: 3, Role: "user", Content: "What happens next?"},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume: Mina and Rowan are in the archive.",
		},
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-tm1a", Name: "Archive Plan", CurrentContext: "Mina and Rowan plan in the archive"},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-tm1a", StateType: "location", Content: "Archive room"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-tm1a","turn_index":4,"raw_user_input":"Continue","settings":{"max_injection_chars":900,"max_input_context_chars":400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
		t.Fatalf("status = %v, want ok", resp["status"])
	}

	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "trusts") {
		t.Fatalf("injection_text missing relationship text: %q", injectionText)
	}

	sessionState, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	threads, ok := sessionState["pending_threads"].([]any)
	if !ok || len(threads) == 0 {
		t.Fatalf("session_state.pending_threads missing or empty: %#v", sessionState["pending_threads"])
	}
	firstThread, ok := threads[0].(map[string]any)
	if !ok {
		t.Fatalf("first pending thread is not an object")
	}
	if firstThread["status"] != "open" {
		t.Fatalf("pending thread status = %v, want open", firstThread["status"])
	}

	recallResult, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}
	documents, ok := recallResult["documents"].([]any)
	if !ok || len(documents) == 0 {
		t.Fatalf("recall_result.documents missing or empty: %#v", recallResult["documents"])
	}
	foundMemoryDoc := false
	for _, d := range documents {
		doc, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if doc["source_type"] != "memory" {
			continue
		}
		meta, ok := doc["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("memory document missing metadata: %#v", doc)
		}
		if meta["place_wing"] != "North Wing" {
			t.Fatalf("memory document place_wing = %v, want North Wing", meta["place_wing"])
		}
		if meta["place_room"] != "Scene Room" {
			t.Fatalf("memory document place_room = %v, want Scene Room", meta["place_room"])
		}
		foundMemoryDoc = true
		break
	}
	if !foundMemoryDoc {
		t.Fatalf("no memory document with place_wing/place_room found in recall_result.documents")
	}

	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if injectionPack["would_write"] != false {
		t.Fatalf("injection_pack.would_write = %v, want false", injectionPack["would_write"])
	}

	if recallResult["would_write"] != false {
		t.Fatalf("recall_result.would_write = %v, want false", recallResult["would_write"])
	}
}

func TestSeq123P83LongMemoryPromotionCandidateMarkers(t *testing.T) {
	t.Run("critic_prompt_has_durable_extraction_markers", func(t *testing.T) {
		prompt := buildCompleteTurnCriticPrompt(
			"sess-p83", 3,
			"Mina found the brass key.",
			"Rowan nodded and followed.",
			nil, nil, nil,
		)
		required := []string{
			"Extract durable Archive Center memory data",
			"[User]",
			"[Assistant]",
			"<Latest_Turn>",
			"Omit unknown facts instead of inventing placeholders",
			"evidence_excerpts must be short exact excerpts",
			"persona_capsule_candidates",
			"support_only_persona_recollection",
			"requires later user/operator approval",
			"system/progression stories",
			"randomized or conditional acquisition",
			"challenge entry/clear/reward loops",
			"exchange/cost economy",
			"upgrade or unlock rules",
			"Mandatory world-rule audit",
			"world_rule_audit",
			"world_rules must not be empty",
			"1-7 turn session",
			"progression currency exchange",
			"challenge reward loops",
			"abstract invariant",
		}
		for _, needle := range required {
			if !strings.Contains(prompt, needle) {
				t.Fatalf("critic prompt missing P83 marker %q", needle)
			}
		}
	})

	t.Run("critic_prompt_includes_language_memory_contract", func(t *testing.T) {
		languageContext := map[string]any{
			"contract_version":        "language_memory.v1",
			"session_output_language": "en",
			"output_language_source":  "explicit_override",
			"raw_user_language":       "ko",
			"summary_language":        "en",
			"search_text_policy":      "summary_plus_raw_plus_aliases",
			"locked_for_turn":         true,
			"raw_evidence_rewritten":  true,
		}
		prompt := buildCompleteTurnCriticPromptWithLanguageContext(
			"sess-p83-lang", 4,
			"RAW-KO: Mina found the brass key.",
			"Mina found the brass key.",
			nil, nil, nil, languageContext,
		)
		for _, needle := range []string{
			"Language_Context_JSON",
			"generated natural-language memory fields must use that language",
			"Do not default to English just because these instructions are English",
			"Raw evidence excerpts must stay exact source text",
			"\"session_output_language\":\"en\"",
			"\"raw_evidence_rewritten\":false",
		} {
			if !strings.Contains(prompt, needle) {
				t.Fatalf("critic prompt missing language contract marker %q: %s", needle, prompt)
			}
		}
	})

	t.Run("non_empty_turn_summary_saves_canonical_memory", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina found the brass key.",
			"importance_score": 7,
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p83", 5, extraction, "Mina found the brass key.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if result.Memories != 1 {
			t.Fatalf("expected 1 memory save, got %d", result.Memories)
		}
		if len(fake.savedMemories) != 1 {
			t.Fatalf("expected 1 saved memory, got %d", len(fake.savedMemories))
		}
		mem := fake.savedMemories[0]
		if mem.ChatSessionID != "sess-p83" {
			t.Fatalf("memory ChatSessionID = %q, want sess-p83", mem.ChatSessionID)
		}
		if mem.TurnIndex != 5 {
			t.Fatalf("memory TurnIndex = %d, want 5", mem.TurnIndex)
		}
		if !strings.Contains(mem.SummaryJSON, "turn_summary") {
			t.Fatalf("memory SummaryJSON missing turn_summary: %s", mem.SummaryJSON)
		}
		if mem.Importance <= 0 {
			t.Fatalf("memory Importance = %v, want > 0", mem.Importance)
		}
		if result.EmbeddingStatus != "missing_config" {
			t.Fatalf("EmbeddingStatus = %q, want missing_config when no embed config", result.EmbeddingStatus)
		}
		if result.VectorStatus != "missing_embedding" {
			t.Fatalf("VectorStatus = %q, want missing_embedding when no embed config", result.VectorStatus)
		}
	})

	t.Run("memory_write_contract_persists_language_context_without_rewriting_raw_evidence", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":      "Mina found the brass key.",
			"importance_score":  7,
			"evidence_excerpts": []any{"RAW-KO: Mina found the brass key."},
			"language_context": map[string]any{
				"contract_version":        "language_memory.v1",
				"session_output_language": "en",
				"output_language_source":  "explicit_override",
				"raw_user_language":       "ko",
				"summary_language":        "en",
				"search_text_policy":      "summary_plus_raw_plus_aliases",
				"locked_for_turn":         true,
			},
		}
		content := "RAW-KO: Mina found the brass key.\nMina found the brass key."
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p83-lang", 6, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(600, 0))
		if result.Memories != 1 || result.Evidence != 1 {
			t.Fatalf("expected memory and evidence saves, got result=%#v", result)
		}
		var summary map[string]any
		if err := json.Unmarshal([]byte(fake.savedMemories[0].SummaryJSON), &summary); err != nil {
			t.Fatalf("decode SummaryJSON: %v", err)
		}
		lang, _ := summary["language_context"].(map[string]any)
		if lang["session_output_language"] != "en" || lang["raw_user_language"] != "ko" {
			t.Fatalf("memory language_context = %#v", lang)
		}
		contract, _ := summary["memory_write_contract"].(map[string]any)
		if contract["raw_evidence_rewritten"] != false || contract["summary_language"] != "en" {
			t.Fatalf("memory write contract = %#v", contract)
		}
		ev := fake.savedEvidence[0]
		if ev.EvidenceText != "RAW-KO: Mina found the brass key." {
			t.Fatalf("raw evidence was rewritten: %q", ev.EvidenceText)
		}
		var lineage map[string]any
		if err := json.Unmarshal([]byte(ev.LineageJSON), &lineage); err != nil {
			t.Fatalf("decode LineageJSON: %v", err)
		}
		if lineage["lane"] != "raw_evidence" || lineage["raw_evidence_rewritten"] != false {
			t.Fatalf("evidence lineage missing raw evidence contract: %#v", lineage)
		}
	})

	t.Run("chromadb_memory_document_uses_cross_language_search_text", func(t *testing.T) {
		vec := &turnRecordingVectorStore{}
		cfg := config.Default()
		cfg.ChromaEndpoint = "http://chroma.test"
		srv := NewServer(cfg)
		srv.Vector = vec
		extraction := map[string]any{
			"turn_summary":      "Mina found the brass key.",
			"importance_score":  7,
			"evidence_excerpts": []any{"RAW-KO: Mina found the brass key."},
			"archive_hint": map[string]any{
				"wing": "North Wing",
				"room": "Scene Room",
			},
			"entities": []any{
				map[string]any{
					"name":    "Mina",
					"aliases": []any{"미나"},
				},
				map[string]any{"name": "Rowan"},
			},
			"kg_triples": []any{
				map[string]any{"subject": "Mina", "predicate": "found", "object": "brass key"},
			},
			"language_context": map[string]any{
				"contract_version":        "language_memory.v1",
				"session_output_language": "en",
				"output_language_source":  "explicit_override",
				"raw_user_language":       "ko",
				"summary_language":        "en",
				"search_text_policy":      "summary_plus_raw_plus_aliases",
				"locked_for_turn":         true,
			},
		}
		extraction = applyLanguageMemoryWriteContract(extraction, completeTurnLanguageContextFromExtraction(extraction))
		mem := &store.Memory{
			ID:            42,
			ChatSessionID: "sess-p83-lang-vector",
			TurnIndex:     6,
			SummaryJSON:   mustCompactJSON(extraction),
			Evidence:      mustCompactJSON(map[string]any{"evidence_excerpts": []string{"RAW-KO: Mina found the brass key."}}),
			PlaceWing:     "North Wing",
			PlaceRoom:     "Scene Room",
		}
		searchText := memorySearchTextFromMemory(*mem)
		result := artifactSaveResult{VectorStatus: "not_requested"}
		srv.upsertMemoryVector(context.Background(), "sess-p83-lang-vector", 6, mem, searchText.Text, []float32{0.1, 0.2}, &result)
		if result.VectorStatus != "ok" || result.VectorsUpserted != 1 {
			t.Fatalf("vector upsert result = %#v", result)
		}
		if len(vec.docs) != 1 {
			t.Fatalf("expected 1 vector doc, got %d", len(vec.docs))
		}
		doc := vec.docs[0]
		for _, needle := range []string{
			"[Canonical Summary]\nMina found the brass key.",
			"[Raw Evidence]\nRAW-KO: Mina found the brass key.",
			"[Aliases]",
			"미나",
			"North Wing",
			"brass key",
		} {
			if !strings.Contains(doc.DocumentText, needle) {
				t.Fatalf("vector document missing %q: %s", needle, doc.DocumentText)
			}
		}
		if doc.SearchTextPolicy != "summary_plus_raw_plus_aliases" || doc.RawLanguage != "ko" || doc.SummaryLanguage != "en" || doc.SessionOutputLanguage != "en" {
			t.Fatalf("vector language metadata = %#v", doc)
		}
		if doc.AliasCount == 0 {
			t.Fatalf("expected aliases to be indexed, doc=%#v", doc)
		}
		if doc.SchemaVersion != "memory.v2" {
			t.Fatalf("schema version = %q, want memory.v2", doc.SchemaVersion)
		}
	})

	t.Run("prepare_turn_language_aware_injection_uses_output_summary_and_preserves_raw_evidence", func(t *testing.T) {
		languageContext := map[string]any{
			"contract_version":        "language_memory.v1",
			"session_output_language": "en",
			"output_language_source":  "explicit_override",
			"raw_user_language":       "ko",
			"summary_language":        "en",
			"search_text_policy":      "summary_plus_raw_plus_aliases",
			"locked_for_turn":         true,
		}
		memories := []store.Memory{{
			ID:          77,
			TurnIndex:   4,
			SummaryJSON: `{"turn_summary":"Mina hid the brass key under the old shrine.","language_context":{"contract_version":"language_memory.v1","session_output_language":"en","raw_user_language":"ko","summary_language":"en","search_text_policy":"summary_plus_raw_plus_aliases"}}`,
			Evidence:    `{"evidence_excerpts":["미나는 오래된 사당 아래에 황동 열쇠를 숨겼다."]}`,
			Importance:  0.9,
		}}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 2000, "Where is the brass key?", "default", nil, nil, languageContext)
		if !strings.Contains(assembly.MemoryText, "Mina hid the brass key under the old shrine.") {
			t.Fatalf("memory text missing output-language summary: %q", assembly.MemoryText)
		}
		if !strings.Contains(assembly.MemoryText, "raw_evidence: 미나는 오래된 사당 아래에 황동 열쇠를 숨겼다.") {
			t.Fatalf("memory text missing raw evidence: %q", assembly.MemoryText)
		}
		if !strings.Contains(assembly.MemoryText, "summary_language=en") || !strings.Contains(assembly.MemoryText, "raw_language=ko") {
			t.Fatalf("memory text missing language markers: %q", assembly.MemoryText)
		}
		if assembly.LanguageInjectionTrace["status"] != "ready" {
			t.Fatalf("language injection status = %#v", assembly.LanguageInjectionTrace)
		}
		memTrace, _ := assembly.LanguageInjectionTrace["memory_language_trace"].(map[string]any)
		if intFromAny(memTrace["memory_summary_language_match"], 0) != 1 || intFromAny(memTrace["raw_evidence_attached_count"], 0) != 1 {
			t.Fatalf("memory language trace = %#v", memTrace)
		}
	})

	t.Run("prepare_turn_planner_support_carries_session_output_language_contract", func(t *testing.T) {
		languageContext := map[string]any{
			"contract_version":        "language_memory.v1",
			"session_output_language": "ja",
			"output_language_source":  "plugin_setting",
			"raw_user_language":       "ko",
			"summary_language":        "ja",
			"locked_for_turn":         true,
		}
		pack := buildSupervisorInputPack(
			"sess-planner-lang",
			8,
			"다음 장면으로 이어가줘.",
			"standard",
			"weak",
			"balanced",
			"none",
			"",
			map[string]any{"prompt_source": "test"},
			map[string]any{"memory_count": 1},
			nil,
			storylineSupervisorSelection{},
			false,
			"",
			languageContext,
		)
		contract, _ := pack["planner_language_contract"].(map[string]any)
		if contract["planner_support_language"] != "ja" || contract["current_user_input_priority"] != "highest" {
			t.Fatalf("planner language contract = %#v", contract)
		}
		if contract["raw_user_input_rewritten"] != false || contract["raw_evidence_rewritten"] != false {
			t.Fatalf("planner contract rewrote raw lanes: %#v", contract)
		}
	})

	t.Run("protected_secret_contract_derives_owner_scoped_subjective_memory", func(t *testing.T) {
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":     "A private feeling is established.",
			"importance_score": 6,
			"protected_secrets": []any{
				map[string]any{
					"secret_kind":       "romantic_feeling",
					"owner":             "Mina",
					"subject":           []any{"Rowan"},
					"summary":           "Mina privately likes Rowan but has not revealed it.",
					"sensitivity":       "medium",
					"disclosure_policy": "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":   []any{"Mina"},
						"unknown_to": []any{"Rowan"},
					},
				},
			},
		})
		secrets := sliceFromAny(extraction["protected_secrets"])
		if len(secrets) != 1 {
			t.Fatalf("protected secrets not normalized: %#v", extraction["protected_secrets"])
		}
		memories := sliceFromAny(extraction["subjective_entity_memories"])
		if len(memories) != 1 {
			t.Fatalf("protected secret should derive one subjective memory, got %#v", memories)
		}
		mem := mapFromAny(memories[0])
		if mem["secret_guard"] != true || mem["owner_visibility"] != "owner_private" || mem["target_reveal_policy"] != "owner_private_until_revealed" {
			t.Fatalf("derived protected subjective memory is not guarded: %#v", mem)
		}
		tags := stringsFromAny(mem["tags"])
		for _, want := range []string{"protected_secret", "secret_guard", "protected_secret_kind:romantic_feeling"} {
			if !containsStringFold(tags, want) {
				t.Fatalf("derived tags missing %q: %#v", want, tags)
			}
		}
	})

	t.Run("protected_secret_memory_injection_masks_secret_content", func(t *testing.T) {
		memories := []store.Memory{{
			ID:        88,
			TurnIndex: 5,
			SummaryJSON: mustCompactJSON(map[string]any{
				"turn_summary": "Mina privately likes Rowan but has not revealed it.",
				"protected_secrets": []any{
					map[string]any{
						"secret_kind":       "romantic_feeling",
						"owner":             "Mina",
						"summary":           "Mina privately likes Rowan but has not revealed it.",
						"disclosure_policy": "owner_private_until_revealed",
						"knowledge_scope": map[string]any{
							"known_by":   []string{"Mina"},
							"unknown_to": []string{"Rowan"},
						},
					},
				},
			}),
			Importance: 0.9,
		}}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 2000, "Mina looks away.", "default", nil, nil, nil)
		if !strings.Contains(assembly.MemoryText, "Protected continuity guard") {
			t.Fatalf("protected memory did not produce guard text: %q", assembly.MemoryText)
		}
		if strings.Contains(assembly.MemoryText, "privately likes Rowan") {
			t.Fatalf("protected memory leaked secret content: %q", assembly.MemoryText)
		}
		if !strings.Contains(assembly.MemoryText, "kind=romantic_feeling") {
			t.Fatalf("protected memory guard missing kind: %q", assembly.MemoryText)
		}
	})

	t.Run("protected_secret_memory_requires_current_entity_scope", func(t *testing.T) {
		memories := []store.Memory{{
			ID:        89,
			TurnIndex: 8,
			SummaryJSON: mustCompactJSON(map[string]any{
				"turn_summary": "Elsie concealed her former role.",
				"protected_secrets": []any{
					map[string]any{
						"secret_kind":       "former_role",
						"owner":             "Elsie",
						"summary":           "Elsie concealed her former role.",
						"disclosure_policy": "owner_private_until_revealed",
						"knowledge_scope": map[string]any{
							"known_by": []string{"Elsie"},
						},
					},
				},
			}),
			Importance: 0.9,
		}}
		blocked := buildPrepareTurnInjectionAssembly(memories, nil, nil, []store.ChatLog{
			{TurnIndex: 7, Role: "user", Content: "Niv and Ingrid inspect the courtyard."},
			{TurnIndex: 7, Role: "assistant", Content: "They keep their voices low."},
		}, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 2000, "Niv asks Ingrid about the courtyard.", "default", nil, nil, nil)
		if strings.Contains(blocked.MemoryText, "Protected continuity guard") || strings.Contains(blocked.MemoryText, "former_role") {
			t.Fatalf("off-scene protected memory guard should not inject: %q", blocked.MemoryText)
		}
		if got := intFromAny(blocked.Counts["protected_memory_dropped_count"], 0); got != 1 {
			t.Fatalf("protected memory drop count = %d, want 1; counts=%#v", got, blocked.Counts)
		}

		allowed := buildPrepareTurnInjectionAssembly(memories, nil, nil, []store.ChatLog{
			{TurnIndex: 7, Role: "user", Content: "Niv and Ingrid inspect the courtyard."},
			{TurnIndex: 7, Role: "assistant", Content: "They keep their voices low."},
		}, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 2000, "Niv and Ingrid mention Elsie while speaking in the courtyard.", "default", nil, nil, nil)
		if !strings.Contains(allowed.MemoryText, "Protected continuity guard") || !strings.Contains(allowed.MemoryText, "kind=former_role") {
			t.Fatalf("currently mentioned protected memory guard should inject: %q", allowed.MemoryText)
		}
	})

	t.Run("character_private_recollection_masks_secret_guard_content", func(t *testing.T) {
		text := characterPrivateRecollectionPromptLineText(store.ProtagonistEntityMemory{
			OwnerEntityKey:     "mina",
			OwnerEntityName:    "Mina",
			MemoryText:         "Mina privately likes Rowan but has not revealed it.",
			SecretGuard:        true,
			TargetRevealPolicy: "owner_private_until_revealed",
			TagsJSON:           `["protected_secret","secret_guard","protected_secret_kind:romantic_feeling"]`,
		}, 500)
		if !strings.Contains(text, "protected private knowledge is present") || !strings.Contains(text, "kind=romantic_feeling") {
			t.Fatalf("private recollection guard text missing: %q", text)
		}
		if strings.Contains(text, "privately likes Rowan") {
			t.Fatalf("private recollection leaked secret text: %q", text)
		}
	})

	t.Run("empty_turn_summary_skips_memory_save", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "",
			"importance_score": 7,
			"kg_triples":       []any{map[string]any{"subject": "Mina", "predicate": "found", "object": "key"}},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p83-empty", 2, extraction, "Mina found the brass key.", completeTurnEmbeddingConfig{}, time.Unix(100, 0))
		if result.Memories != 0 {
			t.Fatalf("expected 0 memory saves for empty turn_summary, got %d", result.Memories)
		}
		if len(fake.savedMemories) != 0 {
			t.Fatalf("expected 0 saved memories for empty turn_summary, got %d", len(fake.savedMemories))
		}
	})

	t.Run("ooc_control_turn_guard_skips_memory", func(t *testing.T) {
		fake := &turnRecordingStore{}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeMariaDBAuthority
		srv := NewServer(cfg)
		srv.Store = fake
		srv.StoreOpenError = nil

		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)

		body := `{"chat_session_id":"sess-p83-ooc","turn_index":1,"user_input":"OOC: please change the plugin setting","assistant_content":"Sure, I will help.","context_messages":[]}`
		req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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
		if resp["save_error"] != "skipped_by_ooc_guard" || resp["critic_triggered"] != false {
			t.Fatalf("unexpected OOC guard response: %+v", resp)
		}
		if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
			t.Fatalf("OOC control turn should skip all writes, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
		}
	})
}
