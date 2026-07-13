package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestArchiveCenter24ReplayRegressionGate(t *testing.T) {
	t.Run("language_replay_cases_preserve_raw_evidence_and_output_summary_contract", func(t *testing.T) {
		cases := []struct {
			name    string
			rawLang string
			outLang string
		}{
			{name: "ko_to_ko", rawLang: "ko", outLang: "ko"},
			{name: "ko_to_en", rawLang: "ko", outLang: "en"},
			{name: "ko_to_ja", rawLang: "ko", outLang: "ja"},
			{name: "en_to_ja", rawLang: "en", outLang: "ja"},
			{name: "mid_session_ja_to_en", rawLang: "ja", outLang: "en"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				languageContext := normalizeCompleteTurnLanguageContext(map[string]any{
					"contract_version":        "language_memory.v1",
					"session_output_language": tc.outLang,
					"output_language_source":  "replay_fixture",
					"raw_user_language":       tc.rawLang,
					"summary_language":        tc.outLang,
					"search_text_policy":      "summary_plus_raw_plus_aliases",
					"locked_for_turn":         true,
				})
				planner := buildPrepareTurnPlannerLanguageContract(languageContext)
				if planner["planner_support_language"] != tc.outLang || planner["raw_evidence_rewritten"] != false {
					t.Fatalf("planner language contract mismatch for %s: %#v", tc.name, planner)
				}
				rawEvidence := "RAW-" + tc.rawLang + ": sealed-oath evidence remains exact."
				extraction := applyLanguageMemoryWriteContract(map[string]any{
					"turn_summary":      "SUMMARY-" + tc.outLang + ": The sealed oath remains active.",
					"importance_score":  7,
					"evidence_excerpts": []any{rawEvidence},
					"language_context":  languageContext,
				}, languageContext)
				searchText := completeTurnMemorySearchText("", extraction, rawEvidence+"\nSUMMARY-"+tc.outLang+": The sealed oath remains active.")
				if !strings.Contains(searchText.Text, "SUMMARY-"+tc.outLang) || !strings.Contains(searchText.Text, rawEvidence) {
					t.Fatalf("search text missing summary/raw evidence for %s: %s", tc.name, searchText.Text)
				}
				if got := extractionStringFromAny(searchText.LanguageContext["session_output_language"]); got != tc.outLang {
					t.Fatalf("search text language context output=%q, want %q", got, tc.outLang)
				}
			})
		}
	})

	t.Run("identity_accuracy_aliases_feed_vector_search_and_prompt_preserves_private_same_entity_continuity", func(t *testing.T) {
		content := "Lia spoke about when she was Gloria."
		languageContext := normalizeCompleteTurnLanguageContext(map[string]any{
			"contract_version":        "language_memory.v1",
			"session_output_language": "en",
			"raw_user_language":       "en",
			"summary_language":        "en",
			"search_text_policy":      "summary_plus_raw_plus_aliases",
			"locked_for_turn":         true,
		})
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":      "Identity continuity evidence exists.",
			"importance_score":  9,
			"evidence_excerpts": []any{content},
			"language_context":  languageContext,
			"character_identity_accuracy": []any{
				map[string]any{
					"surface_identity_name": "Lia",
					"true_identity_name":    "Gloria",
					"canonical_entity_name": "Gloria",
					"aliases":               []any{"리아", "글로리아"},
					"identity_kind":         "cover_identity",
					"same_entity":           true,
					"public_role":           "maid",
					"true_role":             "marchioness",
					"reveal_policy":         "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":     []any{"Gloria"},
						"unknown_to":   []any{"Siwoo"},
						"suspected_by": []any{"Lucia"},
					},
				},
			},
		})
		extraction = applyLanguageMemoryWriteContract(extraction, languageContext)
		searchText := completeTurnMemorySearchText("", extraction, content)
		for _, needle := range []string{"Lia", "Gloria", "cover_identity", "maid", "marchioness", "Siwoo", "Lucia"} {
			if !strings.Contains(searchText.Text, needle) {
				t.Fatalf("identity replay search text missing alias/scope %q: %s", needle, searchText.Text)
			}
		}
		memories := []store.Memory{
			{
				ID:          2405,
				TurnIndex:   13,
				SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": "An unrelated harvest meeting happened elsewhere."}),
				Importance:  1.0,
			},
			{
				ID:          2406,
				TurnIndex:   12,
				SummaryJSON: mustCompactJSON(extraction),
				Evidence:    mustCompactJSON(map[string]any{"evidence_excerpts": []string{content}}),
				Importance:  0.5,
			},
		}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 3000, "Lia enters the room.", "default", nil, nil, languageContext)
		if !strings.Contains(assembly.MemoryText, "Protected identity continuity") || !strings.Contains(assembly.MemoryText, "kind=cover_identity") {
			t.Fatalf("identity replay did not inject protected guard: %q", assembly.MemoryText)
		}
		for _, needle := range []string{
			"Lia and Gloria refer to the same internal person",
			"not portray the surface identity and true identity as separate people",
			"not public character knowledge",
		} {
			if !strings.Contains(assembly.MemoryText, needle) {
				t.Fatalf("identity replay missing continuity guard %q: %q", needle, assembly.MemoryText)
			}
		}
		if !strings.Contains(assembly.MemoryText, "knowledge_scope=known:1 suspected:1") {
			t.Fatalf("identity replay missing scoped knowledge counts: %q", assembly.MemoryText)
		}
		for _, leaked := range []string{"marchioness", "spoke about when"} {
			if strings.Contains(assembly.MemoryText, leaked) {
				t.Fatalf("identity replay leaked protected identity content %q: %q", leaked, assembly.MemoryText)
			}
		}
	})

	t.Run("pov_scoped_identity_replay_treats_cover_identity_as_self_for_knower", func(t *testing.T) {
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":     "Gloria uses Lia as a protected cover identity.",
			"importance_score": 9,
			"character_identity_accuracy": []any{
				map[string]any{
					"surface_identity_name": "Lia",
					"true_identity_name":    "Gloria",
					"canonical_entity_name": "Gloria",
					"identity_kind":         "cover_identity",
					"same_entity":           true,
					"public_role":           "maid",
					"true_role":             "marchioness",
					"reveal_policy":         "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":     []any{"Gloria"},
						"unknown_to":   []any{"Siwoo"},
						"suspected_by": []any{"Lucia"},
					},
				},
			},
		})
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{{
				ID:            2408,
				ChatSessionID: "sess-pov-identity",
				TurnIndex:     18,
				SummaryJSON:   mustCompactJSON(extraction),
				Importance:    0.9,
			}},
		}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeDualShadow
		srv := NewServer(cfg)
		srv.Store = fake
		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)

		body := `{
			"chat_session_id":"sess-pov-identity",
			"turn_index":19,
			"raw_user_input":"Gloria considers Lia's field report.",
			"client_meta":{"perspective_context":{"current_pov":"Gloria","source":"hidden_spoiler_pov"}},
			"settings":{"max_injection_chars":3000,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
		}`
		req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
		injectionPack := mapFromAny(resp["injection_pack"])
		memoryText := extractionStringFromAny(injectionPack["memory_text"])
		for _, needle := range []string{
			"POV-scoped identity continuity",
			"Lia is Gloria's own protected surface identity/persona",
			"treat Lia and Gloria as the same internal person",
			"not two separate characters",
			"Keep this as POV/private knowledge",
		} {
			if !strings.Contains(memoryText, needle) {
				t.Fatalf("POV identity replay missing %q: %q", needle, memoryText)
			}
		}
		perspective := mapFromAny(injectionPack["perspective_context"])
		if perspective["current_pov"] != "Gloria" || perspective["source"] != "hidden_spoiler_pov" {
			t.Fatalf("perspective context mismatch: %#v", perspective)
		}
	})

	t.Run("pov_scoped_identity_replay_infers_hidden_spoiler_pov_from_raw_input", func(t *testing.T) {
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":     "Gloria uses Lia as a protected cover identity.",
			"importance_score": 9,
			"character_identity_accuracy": []any{
				map[string]any{
					"surface_identity_name": "Lia",
					"true_identity_name":    "Gloria",
					"canonical_entity_name": "Gloria",
					"identity_kind":         "cover_identity",
					"same_entity":           true,
					"reveal_policy":         "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":   []any{"Gloria"},
						"unknown_to": []any{"Siwoo"},
					},
				},
			},
		})
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{{
				ID:            2410,
				ChatSessionID: "sess-pov-infer",
				TurnIndex:     20,
				SummaryJSON:   mustCompactJSON(extraction),
				Importance:    0.9,
			}},
		}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeDualShadow
		srv := NewServer(cfg)
		srv.Store = fake
		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)

		body := `{
			"chat_session_id":"sess-pov-infer",
			"turn_index":21,
			"raw_user_input":"히든 스포일러: 글로리아 시점으로 파웰에게 리아의 현장 보고를 묻는다.",
			"settings":{"max_injection_chars":3000,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
		}`
		req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
		injectionPack := mapFromAny(resp["injection_pack"])
		memoryText := extractionStringFromAny(injectionPack["memory_text"])
		for _, needle := range []string{
			"POV-scoped identity continuity",
			"current_pov=글로리아",
			"treat Lia and Gloria as the same internal person",
		} {
			if !strings.Contains(memoryText, needle) {
				t.Fatalf("inferred POV identity replay missing %q: %q", needle, memoryText)
			}
		}
		perspective := mapFromAny(injectionPack["perspective_context"])
		if perspective["current_pov"] != "글로리아" || !strings.Contains(extractionStringFromAny(perspective["source"]), "inferred_raw_user_input") {
			t.Fatalf("inferred perspective context mismatch: %#v", perspective)
		}
	})

	t.Run("confirmed_identity_alias_canonicalizes_saved_artifacts_without_rewriting_raw_evidence", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake

		evidence := "Lia wrote the field report while hiding that she was Gloria."
		content := evidence + " Powell noticed only the careful handwriting."
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":      "Lia coordinated the field report under her cover identity.",
			"importance_score":  9,
			"evidence_excerpts": []any{evidence},
			"entities": map[string]any{
				"characters": []any{
					map[string]any{"name": "Lia", "entity_type": "character", "description": "maid cover"},
				},
			},
			"kg_triples": []any{
				map[string]any{"subject": "Lia", "predicate": "submitted_report_to", "object": "Powell"},
			},
			"character_deltas": []any{
				map[string]any{"name": "Lia", "status": map[string]any{"cover_identity": "active"}},
			},
			"subjective_entity_memories": []any{
				map[string]any{
					"owner_entity_key":     "lia",
					"owner_entity_name":    "Lia",
					"owner_entity_role":    "npc",
					"owner_visibility":     "owner_private",
					"memory_text":          "Lia worries Powell will see through the maid cover.",
					"evidence_excerpt":     evidence,
					"secret_guard":         true,
					"target_reveal_policy": "owner_private_until_revealed",
				},
			},
			"character_identity_accuracy": []any{
				map[string]any{
					"surface_identity_name": "Lia",
					"true_identity_name":    "Gloria",
					"canonical_entity_name": "Gloria",
					"identity_kind":         "cover_identity",
					"same_entity":           true,
					"public_role":           "maid",
					"true_role":             "marchioness",
					"reveal_policy":         "owner_private_until_revealed",
					"knowledge_scope": map[string]any{
						"known_by":   []any{"Gloria"},
						"unknown_to": []any{"Siwoo", "Powell"},
					},
				},
			},
		})

		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-identity-merge", 24, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(2400, 0))
		if result.Entities != 1 || result.KGTriples != 1 || result.CharacterStates != 1 || result.SubjectiveEntityMemories == 0 {
			t.Fatalf("expected identity merge to save canonical artifacts, result=%#v", result)
		}
		if len(fake.savedEvidence) != 1 || fake.savedEvidence[0].EvidenceText != evidence {
			t.Fatalf("raw direct evidence was rewritten or skipped: %#v", fake.savedEvidence)
		}
		if len(fake.savedEntities) != 1 || fake.savedEntities[0].Name != "Gloria" {
			t.Fatalf("entity was not canonicalized to Gloria: %#v", fake.savedEntities)
		}
		if !strings.Contains(fake.savedEntities[0].AliasesJSON, "Lia") {
			t.Fatalf("entity alias did not preserve surface identity: %s", fake.savedEntities[0].AliasesJSON)
		}
		if len(fake.savedKGTriples) != 1 || fake.savedKGTriples[0].Subject != "Gloria" || fake.savedKGTriples[0].Object != "Powell" {
			t.Fatalf("KG triple was not canonicalized safely: %#v", fake.savedKGTriples)
		}
		if len(fake.savedCharacterStates) != 1 || fake.savedCharacterStates[0].CharacterName != "Gloria" {
			t.Fatalf("character state was not canonicalized: %#v", fake.savedCharacterStates)
		}
		foundCanonicalOwner := false
		foundRawOwnerTag := false
		for _, memory := range fake.savedEntityMemories {
			if memory.OwnerEntityName == "Gloria" && memory.OwnerEntityKey == "gloria" {
				foundCanonicalOwner = true
			}
			if strings.Contains(memory.TagsJSON, "raw_owner_entity_name:Lia") && strings.Contains(memory.TagsJSON, "confirmed_identity_alias_canonicalized") {
				foundRawOwnerTag = true
			}
		}
		if !foundCanonicalOwner || !foundRawOwnerTag {
			t.Fatalf("subjective owner canonicalization missing, memories=%#v", fake.savedEntityMemories)
		}
		if len(fake.savedMemories) != 1 || !strings.Contains(fake.savedMemories[0].SummaryJSON, "confirmed_identity_alias_canonical_merge") {
			t.Fatalf("memory summary did not record canonical merge trace: %#v", fake.savedMemories)
		}
		if !strings.Contains(fake.savedMemories[0].SummaryJSON, `"surface_identity_name":"Lia"`) {
			t.Fatalf("surface identity was not retained for protected alias search: %s", fake.savedMemories[0].SummaryJSON)
		}
	})

	t.Run("protected_secret_replay_preserves_partial_reveal_scope_without_discovery", func(t *testing.T) {
		extraction := normalizeCriticExtraction(map[string]any{
			"turn_summary":     "A protected inheritance secret exists.",
			"importance_score": 8,
			"protected_secrets": []any{
				map[string]any{
					"secret_kind":       "power_inheritance",
					"owner":             "Ari",
					"subject":           []any{"sealed crest"},
					"summary":           "Ari inherited the sealed crest but has not revealed it.",
					"sensitivity":       "critical",
					"disclosure_policy": "explicit_reveal_event_required",
					"knowledge_scope": map[string]any{
						"known_by":     []any{"Ari", "Mentor"},
						"unknown_to":   []any{"Guard"},
						"suspected_by": []any{"Oracle"},
					},
				},
			},
		})
		searchText := completeTurnMemorySearchText("", extraction, "Ari inherited the sealed crest but has not revealed it.")
		for _, needle := range []string{"Ari", "sealed crest", "power_inheritance", "Mentor", "Guard", "Oracle"} {
			if !strings.Contains(searchText.Text, needle) {
				t.Fatalf("protected secret replay search text missing %q: %s", needle, searchText.Text)
			}
		}
		memories := []store.Memory{{
			ID:          2407,
			TurnIndex:   14,
			SummaryJSON: mustCompactJSON(extraction),
			Importance:  0.9,
		}}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 3000, "Ari faces the guard.", "default", nil, nil, nil)
		if !strings.Contains(assembly.MemoryText, "Protected continuity guard") || !strings.Contains(assembly.MemoryText, "kind=power_inheritance") {
			t.Fatalf("protected secret replay did not inject guard: %q", assembly.MemoryText)
		}
		if !strings.Contains(assembly.MemoryText, "knowledge_scope=known:2 suspected:1") {
			t.Fatalf("protected secret replay missing partial reveal counts: %q", assembly.MemoryText)
		}
		for _, leaked := range []string{"sealed crest", "inherited", "Mentor", "Guard", "Oracle"} {
			if strings.Contains(assembly.MemoryText, leaked) {
				t.Fatalf("protected secret replay leaked scoped secret content %q: %q", leaked, assembly.MemoryText)
			}
		}
	})

	t.Run("vector_artifact_hits_hydrate_and_report_evidence_world_rule_counters", func(t *testing.T) {
		evidence := []store.DirectEvidence{
			{ID: 101, ChatSessionID: "sess-artifact-vector", EvidenceText: "Lia and Gloria are the same person under a cover identity.", TurnAnchor: 7, SourceTurnStart: 7, SourceTurnEnd: 7},
			{ID: 102, ChatSessionID: "sess-artifact-vector", EvidenceText: "This stale evidence is tombstoned.", TurnAnchor: 6, SourceTurnStart: 6, SourceTurnEnd: 6, Tombstoned: true},
		}
		worldRules := []store.WorldRule{
			{ID: 201, ChatSessionID: "sess-artifact-vector", Scope: "session", Category: "identity", Key: "cover_identity_same_entity", ValueJSON: `"Lia and Gloria must be resolved as one internal person."`, SourceTurn: 7},
			{ID: 202, ChatSessionID: "sess-artifact-vector", Scope: "session", Category: "identity", Key: "suppressed_rule", ValueJSON: `"hidden"`, SourceTurn: 7, Suppressed: true},
		}
		vectorShadow := map[string]any{
			"search_result": "ok",
			"search_results": []map[string]any{
				{"id": "evidence:sess-artifact-vector:101", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "101", "similarity": 0.87, "similarity_source": "cosine_from_query_and_stored_embedding"},
				{"id": "world_rule:sess-artifact-vector:201", "tier": "world_rule", "source_table": "world_rules", "source_row_id": "201", "similarity": 0.82, "similarity_source": "cosine_from_query_and_stored_embedding"},
				{"id": "evidence:sess-artifact-vector:102", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "102", "similarity": 0.78, "similarity_source": "cosine_from_query_and_stored_embedding"},
				{"id": "world_rule:sess-artifact-vector:202", "tier": "world_rule", "source_table": "world_rules", "source_row_id": "202", "similarity": 0.76, "similarity_source": "cosine_from_query_and_stored_embedding"},
			},
		}
		assembly := buildPrepareTurnInjectionAssembly(nil, nil, evidence, nil, nil, worldRules, nil, nil, nil, nil, nil, nil, nil, 4, 4000, "Gloria thinks about Lia.", "default", nil, vectorShadow, nil, map[string]any{"current_pov": "Gloria"})
		if !strings.Contains(assembly.DirectEvidenceText, "Lia and Gloria are the same person") {
			t.Fatalf("direct evidence vector hit was not injected: %q", assembly.DirectEvidenceText)
		}
		if !strings.Contains(assembly.WorldRulesText, "cover_identity_same_entity") {
			t.Fatalf("world rule vector hit was not prioritized/injected: %q", assembly.WorldRulesText)
		}
		counts := prepareTurnRenderCounts(assembly, "")
		if got := intFromAny(counts["vector_found"], 0); got != 4 {
			t.Fatalf("vector_found=%d, want 4 raw artifact hits", got)
		}
		if got := intFromAny(counts["vector_hydrated"], 0); got != 2 {
			t.Fatalf("vector_hydrated=%d, want 2", got)
		}
		if got := intFromAny(counts["vector_scope_filtered_count"], 0); got != 2 {
			t.Fatalf("vector_scope_filtered_count=%d, want 2", got)
		}
		if got := intFromAny(counts["vector_injected"], 0); got < 2 {
			t.Fatalf("vector_injected=%d, want at least 2: %#v", got, counts)
		}
	})

	t.Run("identity_guard_mentions_alias_merge_for_pov_cover_identity", func(t *testing.T) {
		identityAccuracy := []any{map[string]any{
			"surface_identity_name": "Lia",
			"true_identity_name":    "Gloria",
			"canonical_entity_name": "Gloria",
			"identity_kind":         "cover_identity",
			"same_entity":           true,
			"reveal_policy":         "owner_private_until_revealed",
			"knowledge_scope": map[string]any{
				"known_by":   []any{"Gloria"},
				"unknown_to": []any{"Siwoo"},
			},
		}}
		globalLine := prepareTurnProtectedIdentityContinuityGuardLine(identityAccuracy)
		if !strings.Contains(globalLine, "keep aliases merged in entity resolution") {
			t.Fatalf("global identity guard missing alias merge instruction: %q", globalLine)
		}
		povLine := prepareTurnPOVScopedIdentityGuardLine(identityAccuracy, map[string]any{"current_pov": "Gloria"})
		if !strings.Contains(povLine, "self/cover-role continuity") {
			t.Fatalf("POV identity guard missing cover-role continuity instruction: %q", povLine)
		}
	})

	t.Run("vector_replay_respects_topk_and_does_not_fill_recent_when_hits_exist", func(t *testing.T) {
		memories := []store.Memory{
			{ID: 1, TurnIndex: 3, SummaryJSON: `{"turn_summary":"Old semantic gate oath.","language_context":{"session_output_language":"en","raw_user_language":"ko","summary_language":"en","search_text_policy":"summary_plus_raw_plus_aliases"},"entities":[{"name":"Mina","aliases":["Gatekeeper"]}]}`, Importance: 0.4},
			{ID: 2, TurnIndex: 4, SummaryJSON: `{"turn_summary":"Old semantic shrine oath.","language_context":{"session_output_language":"en","raw_user_language":"ja","summary_language":"en","search_text_policy":"summary_plus_raw_plus_aliases"},"entities":[{"name":"Rowan","aliases":["Shrine witness"]}]}`, Importance: 0.6},
			{ID: 3, TurnIndex: 30, SummaryJSON: `{"turn_summary":"Recent unrelated dinner detail."}`, Importance: 1},
		}
		vectorShadow := map[string]any{
			"search_result": "ok",
			"search_results": []map[string]any{
				{"id": "episode:sess-24:77", "source_table": "episode_summaries", "source_row_id": "77"},
				{"id": "memory:sess-24:2", "source_table": "memories", "source_row_id": "2", "raw_language": "ja", "summary_language": "en", "session_output_language": "en", "alias_count": 2, "similarity": 0.86, "similarity_source": "cosine_from_query_and_stored_embedding"},
				{"id": "memory:sess-24:99", "source_table": "memories", "source_row_id": "99"},
				{"id": "memory:sess-24:1", "source_table": "memories", "source_row_id": "1", "raw_language": "ko", "summary_language": "en", "session_output_language": "en", "alias_count": 2, "similarity": 0.81, "similarity_source": "cosine_from_query_and_stored_embedding"},
			},
		}
		assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 2, 3000, "Recall the old oath.", "default", nil, vectorShadow, nil)
		for _, want := range []string{"[vector_relevant, turn 4", "[vector_relevant, turn 3", "Old semantic shrine oath", "Old semantic gate oath"} {
			if !strings.Contains(assembly.MemoryText, want) {
				t.Fatalf("vector replay missing %q: %q", want, assembly.MemoryText)
			}
		}
		if strings.Contains(assembly.MemoryText, "Recent unrelated dinner detail") {
			t.Fatalf("vector replay filled topK with recent fallback despite vector hits: %q", assembly.MemoryText)
		}
		for key, want := range map[string]int{
			"vector_memory_hit_count":                       3,
			"vector_memory_hydrated_count":                  2,
			"vector_memory_selected_count":                  2,
			"vector_memory_injected_count":                  2,
			"vector_non_memory_hit_count":                   1,
			"vector_memory_missing_count":                   1,
			"vector_memory_hit_language_context_count":      2,
			"vector_memory_hydrated_language_context_count": 2,
			"selected_memory_total_count":                   2,
		} {
			if got := intFromAny(assembly.Counts[key], 0); got != want {
				t.Fatalf("vector replay count %s=%d, want %d; counts=%#v", key, got, want, assembly.Counts)
			}
		}
	})
}

func TestRunCompleteTurnCriticAuditsWorldRulesWhenInitialExtractionEmpty(t *testing.T) {
	firstExtraction, _ := json.Marshal(map[string]any{
		"turn_summary":      "The island run establishes a companion gacha and dungeon progression loop.",
		"importance_score":  8,
		"evidence_excerpts": []any{"companions are drawn by gacha and dungeon rewards buy skills"},
		"world_rule_audit":  map[string]any{"durable_rule_found": true, "reason": "The extraction noticed a progression system but omitted rules."},
		"world_rules":       []any{},
		"world_state":       map[string]any{"version": "world_state.v1", "confidence": 0, "verification": "", "rules": []any{}},
	})
	auditExtraction, _ := json.Marshal(map[string]any{
		"audit": map[string]any{"durable_rule_found": true, "reason": "The turn confirms a recurring progression economy."},
		"world_rules": []any{
			map[string]any{
				"scope":        "session",
				"scope_name":   "progression_system",
				"category":     "progression",
				"key":          "dungeon_rewards_buy_skills",
				"value":        "Dungeon rewards can be exchanged for skills and progression upgrades.",
				"confidence":   0.9,
				"verification": "verified",
			},
		},
		"world_state": map[string]any{
			"version":      "world_state.v1",
			"confidence":   0.9,
			"verification": "verified",
			"rules": []any{
				map[string]any{"scope": "session", "scope_name": "progression_system", "category": "progression", "key": "dungeon_rewards_buy_skills", "value": "Dungeon rewards can be exchanged for skills and progression upgrades."},
			},
		},
	})
	responses := []string{string(firstExtraction), string(auditExtraction)}
	calls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls >= len(responses) {
			t.Fatalf("unexpected extra critic call %d", calls+1)
		}
		payload, _ := json.Marshal(map[string]any{
			"model":   "critic-test",
			"choices": []any{map[string]any{"message": map[string]any{"content": responses[calls]}}},
		})
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(payload))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := NewServer(config.Default())
	cfg := completeTurnLLMConfig{
		APIKey:              "sk-test",
		Endpoint:            "https://api.example.com/v1",
		Model:               "critic-test",
		Provider:            "openai",
		TimeoutMs:           60000,
		Temperature:         0.2,
		MaxTokens:           1600,
		MaxCompletionTokens: 1600,
	}
	extraction, trace, err := srv.runCompleteTurnCritic(
		context.Background(),
		"sess-world-audit",
		3,
		"섬에서는 동료를 뽑기로 얻고 던전 보상 포인트로 스킬을 구매한다.",
		"관리자는 이 섬의 기본 진행 구조가 뽑기 동료와 던전 보상 경제라고 설명했다.",
		nil,
		nil,
		cfg,
	)
	if err != nil {
		t.Fatalf("runCompleteTurnCritic error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("critic calls = %d, want 2", calls)
	}
	if got := len(worldRuleItemsForSave(extraction)); got == 0 {
		t.Fatalf("world-rule audit did not merge rules: %#v", extraction)
	}
	auditTrace := mapFromAny(trace["world_rule_audit"])
	if auditTrace["status"] != "ok" || intFromAny(auditTrace["merged_world_rule_count"], 0) == 0 {
		t.Fatalf("world_rule_audit trace mismatch: %+v", auditTrace)
	}
}

func TestRunCompleteTurnCriticForceWorldRuleAuditWhenInitialAuditMissing(t *testing.T) {
	firstExtraction, _ := json.Marshal(map[string]any{
		"turn_summary":      "The first session setup establishes a stable progression economy.",
		"importance_score":  8,
		"evidence_excerpts": []any{"dungeon points can be spent on skills"},
		"world_rules":       []any{},
		"world_state":       map[string]any{"version": "world_state.v1", "confidence": 0, "verification": "", "rules": []any{}},
	})
	auditExtraction, _ := json.Marshal(map[string]any{
		"audit": map[string]any{"durable_rule_found": true, "reason": "Forced cold-start audit found a progression economy."},
		"world_rules": []any{
			map[string]any{"scope": "session", "scope_name": "progression_system", "category": "economy", "key": "points_buy_skills", "value": "Dungeon points can be spent to purchase skills."},
		},
	})
	responses := []string{string(firstExtraction), string(auditExtraction)}
	calls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls >= len(responses) {
			t.Fatalf("unexpected extra critic call %d", calls+1)
		}
		payload, _ := json.Marshal(map[string]any{
			"model":   "critic-test",
			"choices": []any{map[string]any{"message": map[string]any{"content": responses[calls]}}},
		})
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(payload))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := NewServer(config.Default())
	cfg := completeTurnLLMConfig{
		APIKey:              "sk-test",
		Endpoint:            "https://api.example.com/v1",
		Model:               "critic-test",
		Provider:            "openai",
		TimeoutMs:           60000,
		Temperature:         0.2,
		MaxTokens:           1600,
		MaxCompletionTokens: 1600,
		ForceWorldRuleAudit: true,
	}
	extraction, trace, err := srv.runCompleteTurnCritic(
		context.Background(),
		"sess-world-audit-force",
		1,
		"던전 포인트로 스킬을 구매할 수 있다는 세계 구조를 설명한다.",
		"관리자는 던전 보상 포인트가 스킬 구매 재화라고 확인했다.",
		nil,
		nil,
		cfg,
	)
	if err != nil {
		t.Fatalf("runCompleteTurnCritic error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("critic calls = %d, want 2", calls)
	}
	if got := len(worldRuleItemsForSave(extraction)); got != 1 {
		t.Fatalf("forced world-rule audit saved rules = %d, want 1: %#v", got, extraction)
	}
	auditTrace := mapFromAny(trace["world_rule_audit"])
	if auditTrace["status"] != "ok" {
		t.Fatalf("world_rule_audit trace mismatch: %+v", auditTrace)
	}
}

func TestSeq123P84MemorySummaryNormalizationMinimumFields(t *testing.T) {
	t.Run("normalize_trims_summary_and_clamps_importance", func(t *testing.T) {
		raw := map[string]any{
			"turn_summary":     "  Mina found the key.  ",
			"importance_score": 15,
		}
		out := normalizeCriticExtraction(raw)
		summary, _ := out["turn_summary"].(string)
		if summary != "Mina found the key." {
			t.Fatalf("turn_summary not trimmed/normalized: %q", summary)
		}
		score, _ := out["importance_score"].(float64)
		if score != 10 {
			t.Fatalf("importance_score not clamped to 10: got %v", score)
		}
	})

	t.Run("normalize_clamps_importance_low", func(t *testing.T) {
		raw := map[string]any{
			"turn_summary":     "Brief aside.",
			"importance_score": -3,
		}
		out := normalizeCriticExtraction(raw)
		score, _ := out["importance_score"].(float64)
		if score != 1 {
			t.Fatalf("importance_score not clamped to 1: got %v", score)
		}
	})

	t.Run("memory_save_populates_minimum_fields", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":        "Mina found the brass key in the North Wing Scene Room.",
			"importance_score":    8,
			"evidence_excerpts":   []string{"found the brass key"},
			"relationship_memory": map[string]any{"trust_shift": 1},
			"archive_hint":        map[string]any{"wing": "North Wing", "room": "Scene Room"},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p84", 3, extraction, "Mina found the brass key in the North Wing Scene Room.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
		if result.Memories != 1 {
			t.Fatalf("expected 1 memory save, got %d", result.Memories)
		}
		if len(fake.savedMemories) != 1 {
			t.Fatalf("expected 1 saved memory, got %d", len(fake.savedMemories))
		}
		mem := fake.savedMemories[0]
		if mem.ChatSessionID != "sess-p84" {
			t.Fatalf("ChatSessionID = %q, want sess-p84", mem.ChatSessionID)
		}
		if mem.TurnIndex != 3 {
			t.Fatalf("TurnIndex = %d, want 3", mem.TurnIndex)
		}
		var summaryPayload map[string]any
		if err := json.Unmarshal([]byte(mem.SummaryJSON), &summaryPayload); err != nil {
			t.Fatalf("SummaryJSON must be compact JSON: %v", err)
		}
		if summaryPayload["turn_summary"] != "Mina found the brass key in the North Wing Scene Room." {
			t.Fatalf("SummaryJSON turn_summary = %v", summaryPayload["turn_summary"])
		}
		if summaryPayload["importance_score"] != float64(8) {
			t.Fatalf("SummaryJSON importance_score = %v, want 8", summaryPayload["importance_score"])
		}
		if mem.Importance <= 0 || mem.Importance > 1 {
			t.Fatalf("Importance = %v, want 0 < importance <= 1", mem.Importance)
		}
		if mem.Importance != 0.8 {
			t.Fatalf("Importance = %v, want 0.8", mem.Importance)
		}
		if mem.PlaceWing != "North Wing" {
			t.Fatalf("PlaceWing = %q, want North Wing", mem.PlaceWing)
		}
		if mem.PlaceRoom != "Scene Room" {
			t.Fatalf("PlaceRoom = %q, want Scene Room", mem.PlaceRoom)
		}
		var evidencePayload map[string]any
		if err := json.Unmarshal([]byte(mem.Evidence), &evidencePayload); err != nil {
			t.Fatalf("Evidence must be compact JSON: %v", err)
		}
		excerpts, ok := evidencePayload["evidence_excerpts"].([]any)
		if !ok || len(excerpts) != 1 || excerpts[0] != "found the brass key" {
			t.Fatalf("Evidence excerpts mismatch: %#v", evidencePayload["evidence_excerpts"])
		}
		relationship, ok := evidencePayload["relationship_memory"].(map[string]any)
		if !ok || relationship["trust_shift"] != float64(1) {
			t.Fatalf("Evidence relationship_memory mismatch: %#v", evidencePayload["relationship_memory"])
		}
		if result.EmbeddingStatus != "missing_config" {
			t.Fatalf("EmbeddingStatus = %q, want missing_config", result.EmbeddingStatus)
		}
	})

	t.Run("entity_placeholder_skipped", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina met someone.",
			"importance_score": 5,
			"entities": map[string]any{
				"characters": []any{
					map[string]any{"name": "Mina", "entity_type": "character"},
					map[string]any{"name": "char_1", "entity_type": "character"},
				},
			},
		}
		_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-p84-ent", 4, extraction, "Mina met someone.", completeTurnEmbeddingConfig{}, time.Unix(400, 0))
		var names []string
		for _, e := range fake.savedEntities {
			names = append(names, e.Name)
		}
		if len(names) != 1 || names[0] != "Mina" {
			t.Fatalf("expected only concrete entity [Mina], got %v", names)
		}
	})

	t.Run("pending_thread_save_fields", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "A new mystery appears.",
			"importance_score": 6,
			"pending_threads": []any{
				map[string]any{
					"title":       "Find the hidden door",
					"details":     "Mina noticed a draft behind the bookshelf.",
					"thread_type": "open_question",
					"priority":    2,
					"confidence":  0.85,
					"owner":       "Mina",
				},
			},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p84-th", 5, extraction, "A new mystery appears.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if result.PendingThreads != 1 {
			t.Fatalf("expected 1 pending thread save, got %d", result.PendingThreads)
		}
		if len(fake.savedPendingThreads) != 1 {
			t.Fatalf("expected 1 saved pending thread, got %d", len(fake.savedPendingThreads))
		}
		pt := fake.savedPendingThreads[0]
		if pt.ChatSessionID != "sess-p84-th" {
			t.Fatalf("ChatSessionID = %q, want sess-p84-th", pt.ChatSessionID)
		}
		if pt.SourceTurn != 5 {
			t.Fatalf("SourceTurn = %d, want 5", pt.SourceTurn)
		}
		if pt.ThreadKey == "" {
			t.Fatalf("ThreadKey must not be empty")
		}
		if pt.Status != "open" {
			t.Fatalf("Status = %q, want open", pt.Status)
		}
		if pt.ThreadType != "open_question" {
			t.Fatalf("ThreadType = %q, want open_question", pt.ThreadType)
		}
		if pt.HookType != "open_question" {
			t.Fatalf("HookType = %q, want open_question", pt.HookType)
		}
		if pt.LastSeenTurn != 5 {
			t.Fatalf("LastSeenTurn = %d, want 5", pt.LastSeenTurn)
		}
		if pt.Confidence != 0.85 {
			t.Fatalf("Confidence = %v, want 0.85", pt.Confidence)
		}
		if pt.Title != "Find the hidden door" {
			t.Fatalf("Title = %q, want Find the hidden door", pt.Title)
		}
		if pt.Owner != "Mina" {
			t.Fatalf("Owner = %q, want Mina", pt.Owner)
		}
	})
}

func TestSeq123P85DedupMergeCorrectionPathMarkers(t *testing.T) {
	t.Run("same_incident_dedup_skips_insert_reinforces_importance", func(t *testing.T) {
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 42, ChatSessionID: "sess-p85", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina promised Rowan she would return with the brass key."}`, Importance: 0.4},
			},
		}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina promised Rowan she would return with the brass key.",
			"importance_score": 8,
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p85", 6, extraction, "Mina promised Rowan she would return with the brass key.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if len(fake.savedMemories) != 0 || result.Memories != 0 {
			t.Fatalf("expected 0 SaveMemory calls for duplicate incident, saved=%d result.Memories=%d", len(fake.savedMemories), result.Memories)
		}
		if got := fake.updatedImportance[42]; got < 0.79 || got > 0.81 {
			t.Fatalf("expected existing memory importance reinforced to ~0.8, got %.2f", got)
		}
		if !containsString(result.Warnings, "memory_semantic_dedup_merged") {
			t.Fatalf("expected memory_semantic_dedup_merged warning, got %#v", result.Warnings)
		}
		foundAudit := false
		for _, item := range fake.savedAuditLogs {
			if item.EventType == "memory_semantic_dedup" && item.Source == "critic" {
				var details map[string]any
				if err := json.Unmarshal([]byte(item.DetailsJSON), &details); err == nil {
					if details["merged_memory_id"] == float64(42) {
						foundAudit = true
					}
				}
			}
		}
		if !foundAudit {
			t.Fatalf("expected memory_semantic_dedup audit log with merged_memory_id=42, got %#v", fake.savedAuditLogs)
		}
	})

	t.Run("truly_new_memory_inserts_once_without_dedup", func(t *testing.T) {
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 42, ChatSessionID: "sess-p85-new", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina promised Rowan she would return with the brass key."}`, Importance: 0.4},
			},
		}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "The dragon attacked the northern village without warning.",
			"importance_score": 7,
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p85-new", 6, extraction, "The dragon attacked the northern village without warning.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if len(fake.savedMemories) != 1 || result.Memories != 1 {
			t.Fatalf("expected 1 SaveMemory call for truly new memory, saved=%d result.Memories=%d", len(fake.savedMemories), result.Memories)
		}
		if containsString(result.Warnings, "memory_semantic_dedup_merged") {
			t.Fatalf("unexpected memory_semantic_dedup_merged warning for truly new memory: %#v", result.Warnings)
		}
		for _, item := range fake.savedAuditLogs {
			if item.EventType == "memory_semantic_dedup" {
				t.Fatalf("unexpected dedup audit log for truly new memory: %#v", item)
			}
		}
	})

	t.Run("superseding_summary_similar_to_existing_uses_merge_path", func(t *testing.T) {
		fake := &turnRecordingStore{
			returnMemories: []store.Memory{
				{ID: 43, ChatSessionID: "sess-p85-corr", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina promised Rowan she would return with the brass key."}`, Importance: 0.4},
			},
		}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina promised Rowan that she would return with the brass key.",
			"importance_score": 9,
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p85-corr", 6, extraction, "Mina promised Rowan that she would return with the brass key.", completeTurnEmbeddingConfig{}, time.Unix(500, 0))
		if len(fake.savedMemories) != 0 || result.Memories != 0 {
			t.Fatalf("expected merge path (0 inserts) for superseding similar summary, saved=%d result.Memories=%d", len(fake.savedMemories), result.Memories)
		}
		if !containsString(result.Warnings, "memory_semantic_dedup_merged") {
			t.Fatalf("expected memory_semantic_dedup_merged warning for superseding similar summary, got %#v", result.Warnings)
		}
		foundAudit := false
		for _, item := range fake.savedAuditLogs {
			if item.EventType == "memory_semantic_dedup" && item.Source == "critic" {
				var details map[string]any
				if err := json.Unmarshal([]byte(item.DetailsJSON), &details); err == nil {
					if details["merged_memory_id"] == float64(43) {
						foundAudit = true
					}
				}
			}
		}
		if !foundAudit {
			t.Fatalf("expected memory_semantic_dedup audit log for superseding similar summary, got %#v", fake.savedAuditLogs)
		}
	})
}

func TestSeq123P86TemporalEntityAnchorHardeningMarkers(t *testing.T) {
	t.Run("temporal_anchors_preserved_on_store_artifacts", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":      "Mina found the brass key under the desk.",
			"importance_score":  7,
			"evidence_excerpts": []string{"found the brass key under the desk"},
			"kg_triples": []any{
				map[string]any{"subject": "Mina", "predicate": "found", "object": "brass key", "valid_from": 3},
			},
			"pending_threads": []any{
				map[string]any{"title": "Find the exit", "thread_type": "open_question", "priority": 1, "confidence": 0.9},
			},
			"entities": map[string]any{
				"characters": []any{map[string]any{"name": "Mina", "entity_type": "character"}},
			},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p86", 7, extraction, "Mina found the brass key under the desk.", completeTurnEmbeddingConfig{}, time.Unix(600, 0))

		if len(fake.savedMemories) != 1 {
			t.Fatalf("expected 1 saved memory, got %d", len(fake.savedMemories))
		}
		if fake.savedMemories[0].TurnIndex != 7 {
			t.Fatalf("memory TurnIndex = %d, want 7", fake.savedMemories[0].TurnIndex)
		}

		if len(fake.savedEvidence) != 1 {
			t.Fatalf("expected 1 saved evidence, got %d", len(fake.savedEvidence))
		}
		ev := fake.savedEvidence[0]
		if ev.SourceTurnStart != 7 || ev.SourceTurnEnd != 7 || ev.TurnAnchor != 7 {
			t.Fatalf("evidence temporal anchors mismatch: SourceTurnStart=%d SourceTurnEnd=%d TurnAnchor=%d, want 7", ev.SourceTurnStart, ev.SourceTurnEnd, ev.TurnAnchor)
		}

		if len(fake.savedKGTriples) != 1 {
			t.Fatalf("expected 1 saved KG triple, got %d", len(fake.savedKGTriples))
		}
		kg := fake.savedKGTriples[0]
		if kg.SourceTurn != 7 {
			t.Fatalf("kg triple SourceTurn = %d, want 7", kg.SourceTurn)
		}
		if kg.ValidFrom != 3 {
			t.Fatalf("kg triple ValidFrom = %d, want 3", kg.ValidFrom)
		}

		if len(fake.savedPendingThreads) != 1 {
			t.Fatalf("expected 1 saved pending thread, got %d", len(fake.savedPendingThreads))
		}
		pt := fake.savedPendingThreads[0]
		if pt.SourceTurn != 7 {
			t.Fatalf("pending thread SourceTurn = %d, want 7", pt.SourceTurn)
		}
		if pt.CreatedTurn != 7 {
			t.Fatalf("pending thread CreatedTurn = %d, want 7", pt.CreatedTurn)
		}
		if pt.LastSeenTurn != 7 {
			t.Fatalf("pending thread LastSeenTurn = %d, want 7", pt.LastSeenTurn)
		}

		if result.VectorStatus == "ok" || result.VectorsUpserted > 0 {
			t.Fatalf("expected no vector live write assumption when embedding config missing, VectorStatus=%q VectorsUpserted=%d", result.VectorStatus, result.VectorsUpserted)
		}
	})

	t.Run("entity_concrete_names_stored_with_seen_turns", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Mina and Rowan entered the garden.",
			"importance_score": 6,
			"entities": map[string]any{
				"characters": []any{
					map[string]any{"name": "Mina", "entity_type": "character"},
					map[string]any{"name": "Rowan", "entity_type": "character"},
				},
				"locations": []any{
					map[string]any{"name": "garden", "entity_type": "location"},
				},
			},
		}
		_ = srv.saveCriticExtractionArtifacts(context.Background(), "sess-p86-ent", 4, extraction, "Mina and Rowan entered the garden.", completeTurnEmbeddingConfig{}, time.Unix(700, 0))

		if len(fake.savedEntities) != 3 {
			t.Fatalf("expected 3 saved entities, got %d", len(fake.savedEntities))
		}
		for _, e := range fake.savedEntities {
			if e.FirstSeenTurn != 4 || e.LastSeenTurn != 4 {
				t.Fatalf("entity %q missing turn anchors: FirstSeenTurn=%d LastSeenTurn=%d", e.Name, e.FirstSeenTurn, e.LastSeenTurn)
			}
		}
	})

	t.Run("placeholder_entities_and_kg_skipped", func(t *testing.T) {
		fake := &turnRecordingStore{}
		srv := NewServer(config.Default())
		srv.Store = fake
		extraction := map[string]any{
			"turn_summary":     "Someone found something.",
			"importance_score": 5,
			"entities": map[string]any{
				"characters": []any{
					map[string]any{"name": "char_1", "entity_type": "character"},
					map[string]any{"name": "assistant", "entity_type": "character"},
					map[string]any{"name": "Mina", "entity_type": "character"},
				},
			},
			"kg_triples": []any{
				map[string]any{"subject": "char_1", "predicate": "found", "object": "assistant"},
				map[string]any{"subject": "Mina", "predicate": "found", "object": "brass key"},
			},
		}
		result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-p86-ph", 5, extraction, "Someone found something.", completeTurnEmbeddingConfig{}, time.Unix(800, 0))

		// only Mina should be stored as entity
		var names []string
		for _, e := range fake.savedEntities {
			names = append(names, e.Name)
		}
		if len(names) != 1 || names[0] != "Mina" {
			t.Fatalf("expected only concrete entity [Mina], got %v", names)
		}

		if len(fake.savedKGTriples) != 1 {
			t.Fatalf("expected 1 saved KG triple after placeholder skip, got %d", len(fake.savedKGTriples))
		}
		if fake.savedKGTriples[0].Subject != "Mina" || fake.savedKGTriples[0].Object != "brass key" {
			t.Fatalf("expected KG triple subject=Mina object=brass key, got subject=%q object=%q", fake.savedKGTriples[0].Subject, fake.savedKGTriples[0].Object)
		}

		if result.VectorStatus == "ok" || result.VectorsUpserted > 0 {
			t.Fatalf("unexpected vector live write for placeholder skip test, VectorStatus=%q VectorsUpserted=%d", result.VectorStatus, result.VectorsUpserted)
		}
	})
}
