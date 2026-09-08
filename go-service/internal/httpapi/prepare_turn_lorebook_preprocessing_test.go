package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestLorebookPreprocessingSelectionReachesFinalizer(t *testing.T) {
	newResult := func(refs *[]string) prepareTurnLorebookReferenceResult {
		r := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
		r.ScopeStatus, r.Status = "observed", "ready"
		r.preprocessingRefs = refs
		for i, item := range []struct {
			ref, text string
			overlap   int
		}{
			{"bio", "An official's family biography.", 20},
			{"soil", "The riverbank soil drains quickly.", 2},
			{"tools", "The workshop makes adjustable handles.", 1},
		} {
			r.candidates = append(r.candidates, prepareTurnLorebookCandidate{EntryRef: item.ref, ContextOverlap: item.overlap, Entry: store.LorebookReferenceEntryObservation{EntryOrdinal: i, Content: item.text}})
			r.CandidateRefs = append(r.CandidateRefs, map[string]any{"entry_ref": item.ref})
		}
		return r
	}
	t.Run("explicit_empty", func(t *testing.T) {
		refs := []string{}
		r := newResult(&refs)
		finalizePrepareTurnLorebookReference(&r, "Work the soil", nil, nil, true, 2000)
		if r.deliveryText != "" || r.DeliveryCount != 0 || r.SelectionSource != "ai" {
			t.Fatalf("explicit empty selection was replaced: %#v", r)
		}
		stats := prepareTurnLorebookPayloadBudgetStats(r)
		if stats.ExclusionReason["lorebook_ai_not_selected"] != len(r.candidates) {
			t.Fatalf("explicit empty AI assessment disappeared from budget reasons: %#v", stats)
		}
	})
	t.Run("source_order_and_complete_text", func(t *testing.T) {
		refs := []string{"tools", "soil"}
		r := newResult(&refs)
		finalizePrepareTurnLorebookReference(&r, "Work the soil", nil, nil, true, 2000)
		if !reflect.DeepEqual(r.deliveredSourceRefs(), refs) || strings.Contains(r.deliveryText, "biography") || !strings.Contains(r.deliveryText, "The riverbank soil drains quickly.") {
			t.Fatalf("AI source/order changed: %#v", r)
		}
		stats := prepareTurnLorebookPayloadBudgetStats(r)
		if stats.ExclusionReason["lorebook_ai_not_selected"] != len(r.candidates)-len(refs) {
			t.Fatalf("partial AI assessment exclusion count is incorrect: %#v", stats)
		}
	})
	t.Run("missing_recommendation_keeps_go", func(t *testing.T) {
		r := newResult(nil)
		finalizePrepareTurnLorebookReference(&r, "Work the soil", nil, nil, true, 2000)
		if !reflect.DeepEqual(r.deliveredSourceRefs(), []string{"bio"}) || r.SelectionSource != "go_default" {
			t.Fatalf("existing selection changed: %#v", r)
		}
	})
	t.Run("existing_payload_kept", func(t *testing.T) {
		refs := []string{"tools", "soil"}
		r := newResult(&refs)
		finalizePrepareTurnLorebookReference(&r, "Work the soil", []map[string]any{{"content": "The workshop makes adjustable handles."}}, nil, true, 2000)
		if !reflect.DeepEqual(r.deliveredSourceRefs(), []string{"soil"}) || r.AlreadyPresentCount != 1 {
			t.Fatalf("existing payload duplicated: %#v", r)
		}
	})
	t.Run("received_selection_preserved_with_budget_diagnostic", func(t *testing.T) {
		refs := []string{"tools", "soil"}
		r := newResult(&refs)
		finalizePrepareTurnLorebookReference(&r, "Work the soil", nil, nil, true, 60)
		if !reflect.DeepEqual(r.deliveredSourceRefs(), refs) || r.UsedChars <= r.BudgetChars {
			t.Fatalf("received selection silently shortened: %#v", r)
		}
	})
	t.Run("repeated_refs_and_shared_content_keep_first_selected_original", func(t *testing.T) {
		refs := []string{"tools", "tools", "tools", "soil", "soil", "bio"}
		r := newResult(&refs)
		r.candidates[0].Entry.Content = "THE RIVERBANK SOIL DRAINS QUICKLY."
		finalizePrepareTurnLorebookReference(&r, "Work", nil, nil, true, 2000)
		if !reflect.DeepEqual(r.deliveredSourceRefs(), []string{"tools", "soil", "bio"}) || strings.Contains(r.deliveryText, "THE RIVERBANK") {
			t.Fatalf("coalescing replaced the first selected source: %q refs=%v", r.deliveryText, r.deliveredSourceRefs())
		}
	})
}

func TestPrepareTurnPreprocessingLorebookProviderToPayload(t *testing.T) {
	for _, empty := range []bool{false, true} {
		name := "selected"
		if empty {
			name = "explicit_empty"
		}
		t.Run(name, func(t *testing.T) {
			dataDir := t.TempDir()
			t.Setenv("ARCHIVE_CENTER_DATA_DIR", dataDir)
			calls := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				messages := body["messages"].([]any)
				var input map[string]any
				if err := json.Unmarshal([]byte(messages[1].(map[string]any)["content"].(string)), &input); err != nil {
					t.Error(err)
					return
				}
				if input["role"] != "world_state" {
					t.Errorf("unexpected role: %v", input["role"])
				}
				refs := []string{}
				candidates, _ := input["lorebook_candidates"].([]any)
				if len(candidates) != 2 {
					t.Errorf("scoped reference candidates missing: %v", candidates)
				}
				for _, value := range candidates {
					candidate := value.(map[string]any)
					if candidate["text"] == "The riverbank soil drains quickly." && !empty {
						refs = append(refs, candidate["ref"].(string))
					}
				}
				out, _ := json.Marshal(map[string]any{"selected_ids": []string{}, "selected_lorebook_refs": refs})
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(out)}}}})
			}))
			defer provider.Close()
			cfg := defaultMultiAgentSettings()
			cfg.Enabled = true
			for role, roleCfg := range cfg.Roles {
				roleCfg.Enabled = role == "world_state"
				roleCfg.Provider, roleCfg.Endpoint, roleCfg.Model, roleCfg.APIKey = "custom", provider.URL, "fixture", "test-key"
				cfg.Roles[role] = roleCfg
			}
			encoded, _ := json.Marshal(cfg)
			if err := os.WriteFile(filepath.Join(dataDir, "memory-preprocessing.json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			srv := setupTestServer()
			srv.Store = &prepareTurnLorebookReferenceStore{Store: store.NewNoopStore(), current: &store.LorebookReferenceCurrent{ScopeID: 12, Entries: []store.LorebookReferenceEntryObservation{
				{HostEntryID: "biography", EntryOrdinal: 0, Key: "river", Content: "The river official has a large family."},
				{HostEntryID: "soil", EntryOrdinal: 1, Key: "river", Content: "The riverbank soil drains quickly."},
			}}}
			_, response := prepareTurnPerfRequest(t, srv, `{"chat_session_id":"lore-preprocess","raw_user_input":"Work the river soil","response_projection":"prepare_turn.production_compact.v1","lorebook_reference_scope":{"contract_version":"lorebook_reference_scope.v1","observation_state":"observed","character_index":1,"chat_index":2,"enabled_module_ids":[],"enabled_modules_observed":true},"settings":{"guide_strength":"none","lorebook_reference_mode":"reference_assist"}}`)
			lore := mapFromAny(response["lorebook_reference"])
			wanted := 1
			if empty {
				wanted = 0
			}
			if calls != 1 || intFromAny(lore["delivery_count"], -1) != wanted || lore["selection_source"] != "ai" {
				t.Fatalf("provider recommendation not applied: calls=%d lore=%#v", calls, lore)
			}
			plan := mapFromAny(response["payload_application_plan"])
			planJSON, _ := json.Marshal(plan)
			if strings.Contains(string(planJSON), "large family") || (!empty && !strings.Contains(string(planJSON), "soil drains quickly")) {
				t.Fatalf("payload differs from recommendation: %s", planJSON)
			}
			foundBudgetLane := false
			for _, raw := range anySliceFromAny(mapFromAny(plan["budget_ledger"])["lanes"]) {
				lane := mapFromAny(raw)
				if lane["key"] != "lorebook_reference" {
					continue
				}
				foundBudgetLane = true
				excluded := intFromAny(lore["candidate_count"], 0) - wanted
				if intFromAny(mapFromAny(lane["exclusion_reasons"])["lorebook_ai_not_selected"], -1) != excluded || intFromAny(lane["excluded_count"], -1) != excluded {
					t.Fatalf("AI exclusions did not reach the final payload budget ledger: %#v", lane)
				}
			}
			if !foundBudgetLane {
				t.Fatal("final payload budget ledger omitted the lorebook lane")
			}
		})
	}
}

func TestPreprocessingSearchTraceSeparatesPartialSearchAndRedactsSecrets(t *testing.T) {
	trace := prepareTurnPreprocessingSearchTrace(map[string]any{
		"status": "ok", "memory_search_result": "ok", "precise_memory_search_result": "error",
		"precise_memory_search_error": "unexpected end of JSON input; Authorization: Bearer synthetic-token",
	})
	if trace["status"] != "partial" || trace["precise_hydration"] != "unavailable" {
		t.Fatalf("search success conflated with hydration: %#v", trace)
	}
	detail := trace["precise_memory_search_error"].(string)
	if !strings.Contains(detail, "unexpected end of JSON input") || strings.Contains(detail, "synthetic-token") {
		t.Fatalf("missing or unsafe diagnostic: %q", detail)
	}
}
