package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func routeSmokeCriticStubCalls(stub *routeSmokeCriticStub) int64 {
	if stub == nil {
		return 0
	}
	return stub.Calls()
}

func startRouteSmokeCriticStub() *routeSmokeCriticStub {
	stub := &routeSmokeCriticStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls.Add(1)
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		evidenceExcerpt := "route smoke first assistant content"
		if strings.Contains(string(raw), "route smoke second assistant content") {
			evidenceExcerpt = "route smoke second assistant content"
		}
		extraction := map[string]any{
			"turn_summary":           "Route smoke critic saved a durable memory.",
			"importance_score":       7,
			"emotional_intensity":    0.42,
			"narrative_significance": 0.66,
			"relationship_memory": map[string]any{
				"bond_and_distance": "Nova trusts Orion after the route smoke check.",
				"target_name":       "Orion",
				"trust":             0.74,
			},
			"entities": map[string]any{
				"characters": []any{map[string]any{
					"name":           "Nova",
					"role":           "character",
					"status_emotion": "focused",
					"confidence":     0.91,
				}},
			},
			"kg_triples": []any{map[string]any{
				"subject":    "Nova",
				"predicate":  "trusts",
				"object":     "Orion",
				"valid_from": 1,
			}},
			"evidence_excerpts": []any{evidenceExcerpt},
			"character_deltas": []any{map[string]any{
				"name":          "Nova",
				"status":        map[string]any{"mood": "focused"},
				"relationships": map[string]any{"Orion": "trusted"},
				"events": []any{map[string]any{
					"type":   "route_smoke",
					"detail": "Nova completed a managed route smoke check.",
				}},
			}},
			"world_rules": []any{map[string]any{
				"scope":     "session",
				"category":  "migration_smoke",
				"key":       "route_smoke_rule",
				"value":     "Managed route smoke writes must be visible in MariaDB.",
				"source":    "managed_mariadb_e2e",
				"source_id": "route-write-smoke",
			}},
			"pending_threads": []any{map[string]any{
				"title":       "Route smoke continuity check",
				"details":     "Follow up if managed route smoke writes are missing.",
				"thread_type": "migration_smoke",
				"priority":    2,
				"confidence":  0.85,
			}},
			"state_deltas": map[string]any{
				"scene_pressure": "steady",
			},
		}
		extractionBytes, _ := json.Marshal(extraction)
		resp := map[string]any{
			"id":      "route-smoke-critic",
			"object":  "chat.completion",
			"model":   "route-smoke-critic",
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": string(extractionBytes)}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return stub
}

func (s *routeSmokeCriticStub) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func (s *routeSmokeCriticStub) Close() {
	if s != nil && s.server != nil {
		s.server.Close()
	}
}

func (s *routeSmokeCriticStub) Calls() int64 {
	if s == nil {
		return 0
	}
	return s.calls.Load()
}

func routeSmokeCompleteTurnBody(sessionID string, requestedTurn int, label string, criticEndpoint string) map[string]any {
	return routeSmokeCompleteTurnBodyWithClientMeta(sessionID, requestedTurn, label, routeSmokeStubClientMeta(criticEndpoint), false)
}

func routeSmokeCompleteTurnBodyWithClientMeta(sessionID string, requestedTurn int, label string, clientMeta map[string]any, liveProvider bool) map[string]any {
	userInput := fmt.Sprintf("route smoke %s user input", label)
	assistantContent := fmt.Sprintf("route smoke %s assistant content", label)
	if liveProvider {
		userInput = fmt.Sprintf("route smoke %s user input: Nova tells Orion that she trusts him with the lighthouse key and asks him to remember the Archive Hall promise.", label)
		assistantContent = fmt.Sprintf("route smoke %s assistant content: Nova gives Orion the silver compass in the Archive Hall. Nova says exactly, \"I trust you with the lighthouse key.\" Orion accepts responsibility. The world rule is that the lighthouse key opens the north archive only during moonrise. The unresolved storyline is to repair the clock bridge before dawn. Nova feels focused and relieved.", label)
	}
	return map[string]any{
		"chat_session_id":   sessionID,
		"turn_index":        requestedTurn,
		"user_input":        userInput,
		"assistant_content": assistantContent,
		"request_type":      "model",
		"context_messages": []map[string]any{
			{"role": "critic", "content": "route smoke", "score": 1},
		},
		"improvement_trace": map[string]any{"score": 1, "source": "managed_mariadb_e2e", "label": label},
		"client_meta":       clientMeta,
	}
}

func routeSmokeDelta(before map[string]int, after map[string]int) map[string]int {
	delta := map[string]int{}
	for key, afterValue := range after {
		delta[key] = afterValue - before[key]
	}
	return delta
}

func postJSON(ctx context.Context, url string, payload map[string]any) (map[string]any, error) {
	return postJSONWithTimeout(ctx, url, payload, 10*time.Second)
}

func postJSONWithTimeout(ctx context.Context, url string, payload map[string]any, timeout time.Duration) (map[string]any, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	status := "ok"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "failed"
	}
	out := map[string]any{
		"url":         url,
		"method":      http.MethodPost,
		"http_status": resp.StatusCode,
		"status":      status,
		"response":    decoded,
	}
	if status != "ok" {
		return out, fmt.Errorf("POST %s returned HTTP %d", url, resp.StatusCode)
	}
	return out, nil
}

func deleteJSON(ctx context.Context, url string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	status := "ok"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "failed"
	}
	out := map[string]any{
		"url":         url,
		"method":      http.MethodDelete,
		"http_status": resp.StatusCode,
		"status":      status,
		"response":    decoded,
	}
	if status != "ok" {
		return out, fmt.Errorf("DELETE %s returned HTTP %d", url, resp.StatusCode)
	}
	return out, nil
}

func patchJSONProbe(ctx context.Context, url string, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	status := "ok"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "failed"
	}
	return map[string]any{
		"url":         url,
		"method":      http.MethodPatch,
		"http_status": resp.StatusCode,
		"status":      status,
		"response":    decoded,
	}, nil
}

func routeSmokeCounts(ctx context.Context, dsn string, sessionID string) (map[string]int, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	out := map[string]int{}
	for _, table := range []string{
		"chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"audit_logs",
		"critic_feedback",
		"character_events",
		"entities",
		"trust_states",
		"storylines",
		"world_rules",
		"character_states",
		"pending_threads",
		"active_states",
	} {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE chat_session_id = ?", table)
		if err := db.QueryRowContext(ctx, query, sessionID).Scan(&count); err != nil {
			return nil, fmt.Errorf("%s count: %w", table, err)
		}
		out[table] = count
	}
	return out, nil
}

func routeSmokeContentChecks(ctx context.Context, dsn string, sessionID string, liveCfg routeSmokeLiveConfig) (map[string]any, error) {
	if liveCfg.Enabled {
		return routeSmokeLiveContentChecks(ctx, dsn, sessionID, liveCfg)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	checks := map[string]bool{}
	specs := []struct {
		name  string
		query string
		args  []any
	}{
		{
			name:  "complete_turn_first_user_log",
			query: "SELECT COUNT(*) FROM chat_logs WHERE chat_session_id = ? AND role = 'user' AND content = ?",
			args:  []any{sessionID, "route smoke first user input"},
		},
		{
			name:  "complete_turn_first_assistant_log",
			query: "SELECT COUNT(*) FROM chat_logs WHERE chat_session_id = ? AND role = 'assistant' AND content = ?",
			args:  []any{sessionID, "route smoke first assistant content"},
		},
		{
			name:  "complete_turn_second_user_log",
			query: "SELECT COUNT(*) FROM chat_logs WHERE chat_session_id = ? AND role = 'user' AND content = ?",
			args:  []any{sessionID, "route smoke second user input"},
		},
		{
			name:  "complete_turn_second_assistant_log",
			query: "SELECT COUNT(*) FROM chat_logs WHERE chat_session_id = ? AND role = 'assistant' AND content = ?",
			args:  []any{sessionID, "route smoke second assistant content"},
		},
		{
			name:  "complete_turn_effective_input",
			query: "SELECT COUNT(*) FROM effective_input_logs WHERE chat_session_id = ? AND effective_input LIKE ?",
			args:  []any{sessionID, "%route smoke first user input%"},
		},
		{
			name:  "critic_memory_summary",
			query: "SELECT COUNT(*) FROM memories WHERE chat_session_id = ? AND CAST(summary_json AS CHAR) LIKE ?",
			args:  []any{sessionID, "%Route smoke critic saved a durable memory%"},
		},
		{
			name:  "critic_direct_evidence",
			query: "SELECT COUNT(*) FROM direct_evidence_records WHERE chat_session_id = ? AND evidence_text IN (?, ?)",
			args:  []any{sessionID, "route smoke first assistant content", "route smoke second assistant content"},
		},
		{
			name:  "critic_kg_triple",
			query: "SELECT COUNT(*) FROM kg_triples WHERE chat_session_id = ? AND subject = ? AND predicate = ? AND object = ?",
			args:  []any{sessionID, "Nova", "trusts", "Orion"},
		},
		{
			name:  "critic_entity",
			query: "SELECT COUNT(*) FROM entities WHERE chat_session_id = ? AND name = ?",
			args:  []any{sessionID, "Nova"},
		},
		{
			name:  "critic_trust_state",
			query: "SELECT COUNT(*) FROM trust_states WHERE chat_session_id = ? AND target_name = ?",
			args:  []any{sessionID, "Orion"},
		},
		{
			name:  "critic_world_rule",
			query: "SELECT COUNT(*) FROM world_rules WHERE chat_session_id = ? AND `key` = ?",
			args:  []any{sessionID, "route_smoke_rule"},
		},
		{
			name:  "critic_storyline",
			query: "SELECT COUNT(*) FROM storylines WHERE chat_session_id = ? AND name = ?",
			args:  []any{sessionID, "Route smoke continuity check"},
		},
		{
			name:  "critic_character_state",
			query: "SELECT COUNT(*) FROM character_states WHERE chat_session_id = ? AND character_name = ?",
			args:  []any{sessionID, "Nova"},
		},
		{
			name:  "critic_character_event",
			query: "SELECT COUNT(*) FROM character_events WHERE chat_session_id = ? AND character_name = ? AND event_type = ?",
			args:  []any{sessionID, "Nova", "route_smoke"},
		},
		{
			name:  "critic_pending_thread",
			query: "SELECT COUNT(*) FROM pending_threads WHERE chat_session_id = ? AND description = ?",
			args:  []any{sessionID, "Follow up if managed route smoke writes are missing."},
		},
		{
			name:  "critic_active_state_entities",
			query: "SELECT COUNT(*) FROM active_states WHERE chat_session_id = ? AND state_type = ?",
			args:  []any{sessionID, "entities"},
		},
	}
	all := true
	for _, spec := range specs {
		ok, err := routeSmokeQueryExists(ctx, db, spec.query, spec.args...)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.name, err)
		}
		checks[spec.name] = ok
		if !ok {
			all = false
		}
	}
	return map[string]any{
		"checks":                     checks,
		"all_expected_content_found": all,
		"scope":                      "complete_turn_route_rows_and_critic_artifacts",
	}, nil
}

func routeSmokeLiveContentChecks(ctx context.Context, dsn string, sessionID string, liveCfg routeSmokeLiveConfig) (map[string]any, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	countChecks := map[string]int{
		"chat_logs":               4,
		"effective_input_logs":    2,
		"memories":                2,
		"direct_evidence_records": 2,
		"kg_triples":              2,
		"entities":                2,
		"trust_states":            2,
		"world_rules":             2,
		"storylines":              2,
		"character_states":        2,
		"character_events":        2,
		"pending_threads":         2,
		"active_states":           2,
	}
	checks := map[string]bool{}
	counts := map[string]int{}
	all := true
	for table, minCount := range countChecks {
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE chat_session_id = ?", table)
		count, err := routeSmokeQueryCount(ctx, db, query, sessionID)
		if err != nil {
			return nil, fmt.Errorf("%s count: %w", table, err)
		}
		counts[table] = count
		ok := count >= minCount
		checks[table+"_min_count"] = ok
		if !ok {
			all = false
		}
	}
	embeddingModel := strings.TrimSpace(liveCfg.Embedding.Model)
	if embeddingModel != "" {
		count, err := routeSmokeQueryCount(ctx, db, "SELECT COUNT(*) FROM memories WHERE chat_session_id = ? AND embedding_model = ? AND TRIM(embedding) <> '' AND TRIM(embedding) <> '[]'", sessionID, embeddingModel)
		if err != nil {
			return nil, fmt.Errorf("memory embedding model count: %w", err)
		}
		counts["memories_with_live_embedding_model"] = count
		ok := count >= 2
		checks["memories_with_live_embedding_model"] = ok
		if !ok {
			all = false
		}
	}
	placeholderKG, err := routeSmokeQueryCount(ctx, db, "SELECT COUNT(*) FROM kg_triples WHERE chat_session_id = ? AND (subject LIKE 'char\\_%' OR subject LIKE 'cid\\_%' OR object LIKE 'turn\\_%' OR object LIKE 'char\\_%' OR predicate = 'has_turn')", sessionID)
	if err != nil {
		return nil, fmt.Errorf("placeholder kg count: %w", err)
	}
	counts["placeholder_kg_triples"] = placeholderKG
	checks["no_placeholder_kg_triples"] = placeholderKG == 0
	if placeholderKG != 0 {
		all = false
	}
	rawEvidence, err := routeSmokeQueryCount(ctx, db, "SELECT COUNT(*) FROM direct_evidence_records WHERE chat_session_id = ? AND (CHAR_LENGTH(evidence_text) > 320 OR evidence_text LIKE '%route smoke first user input%route smoke first assistant content%')", sessionID)
	if err != nil {
		return nil, fmt.Errorf("raw direct evidence count: %w", err)
	}
	counts["raw_or_whole_turn_direct_evidence"] = rawEvidence
	checks["no_raw_or_whole_turn_direct_evidence"] = rawEvidence == 0
	if rawEvidence != 0 {
		all = false
	}
	return map[string]any{
		"checks":                     checks,
		"counts":                     counts,
		"all_expected_content_found": all,
		"scope":                      "complete_turn_live_provider_rows_and_quality_guards",
	}, nil
}

func routeSmokeQueryExists(ctx context.Context, db *sql.DB, query string, args ...any) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func routeSmokeQueryCount(ctx context.Context, db *sql.DB, query string, args ...any) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func routeSmokeLiveProviderResponseChecks(routes []map[string]any, liveCfg routeSmokeLiveConfig) map[string]any {
	out := map[string]any{
		"requested": liveCfg.Enabled,
	}
	if !liveCfg.Enabled {
		out["status"] = "not_requested"
		out["all_live_provider_checks_passed"] = true
		return out
	}
	completeRoutes := []map[string]any{}
	for _, route := range routes {
		if strings.HasSuffix(strings.TrimSpace(fmt.Sprint(route["url"])), "/complete-turn") {
			completeRoutes = append(completeRoutes, route)
		}
	}
	checks := map[string]bool{
		"two_complete_turn_responses": len(completeRoutes) >= 2,
	}
	details := []map[string]any{}
	all := checks["two_complete_turn_responses"]
	for index, route := range completeRoutes {
		resp := mapFromAny(route["response"])
		trace := mapFromAny(resp["trace_handoff"])
		llmTrace := mapFromAny(resp["llm_config_trace"])
		criticTrace := mapFromAny(llmTrace["critic"])
		embeddingTrace := mapFromAny(llmTrace["embedding"])
		detail := map[string]any{
			"index":                   index,
			"critic_triggered":        resp["critic_triggered"],
			"derived_artifacts_saved": resp["derived_artifacts_saved"],
			"embedding_status":        trace["embedding_status"],
			"vector_status":           trace["vector_status"],
			"critic_configured":       criticTrace["configured"],
			"embedding_configured":    embeddingTrace["configured"],
			"memories_saved":          resp["memories_saved"],
			"evidence_saved":          resp["evidence_saved"],
			"kg_triples_saved":        resp["kg_triples_saved"],
			"entities_saved":          resp["entities_saved"],
			"trust_states_saved":      resp["trust_states_saved"],
			"world_rules_saved":       resp["world_rules_saved"],
		}
		details = append(details, detail)
		ok := resp["critic_triggered"] == true &&
			intFromAny(resp["derived_artifacts_saved"]) > 0 &&
			fmt.Sprint(trace["embedding_status"]) == "ok" &&
			criticTrace["configured"] == true &&
			embeddingTrace["configured"] == true &&
			intFromAny(resp["memories_saved"]) > 0 &&
			intFromAny(resp["evidence_saved"]) > 0 &&
			intFromAny(resp["kg_triples_saved"]) > 0
		checks[fmt.Sprintf("complete_turn_%d_live_provider_artifacts", index+1)] = ok
		if !ok {
			all = false
		}
	}
	out["status"] = "ok"
	if !all {
		out["status"] = "failed"
	}
	out["all_live_provider_checks_passed"] = all
	out["checks"] = checks
	out["details"] = details
	return out
}

func routeSmokeReport(status string, baseURL string, sessionID string, before map[string]int, after map[string]int, routes []map[string]any, storeMode string) map[string]any {
	if strings.TrimSpace(storeMode) == "" {
		storeMode = "mariadb_shadow"
	}
	return map[string]any{
		"requested":       true,
		"status":          status,
		"base_url":        baseURL,
		"store_mode":      storeMode,
		"chat_session_id": sessionID,
		"before_counts":   before,
		"after_counts":    after,
		"routes":          routes,
	}
}
