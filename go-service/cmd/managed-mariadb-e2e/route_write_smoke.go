package main

import (
	"context"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func runRouteWriteSmoke(ctx context.Context, port int, dsn string, sessionID string, storeMode string) (map[string]any, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	smokeSession := safeSessionID(sessionID) + "-route-write"
	before, err := routeSmokeCounts(ctx, dsn, smokeSession)
	if err != nil {
		return nil, fmt.Errorf("route smoke pre-count: %w", err)
	}

	routes := []map[string]any{}
	liveCfg := routeSmokeLiveConfigFromEnv()
	clientMeta := map[string]any{}
	criticProvider := "managed_stub_openai_compatible"
	embeddingProvider := "not_configured"
	var criticStub *routeSmokeCriticStub
	if liveCfg.Enabled {
		clientMeta = routeSmokeLiveClientMeta(liveCfg)
		criticProvider = "configured_live_provider"
		embeddingProvider = "configured_live_provider"
	} else {
		criticStub = startRouteSmokeCriticStub()
		defer criticStub.Close()
		clientMeta = routeSmokeStubClientMeta(criticStub.URL())
	}

	firstCompleteBody := routeSmokeCompleteTurnBodyWithClientMeta(smokeSession, 9101, "first", clientMeta, liveCfg.Enabled)
	firstCompleteRoute, err := postJSONWithTimeout(ctx, baseURL+"/complete-turn", firstCompleteBody, liveCfg.HTTPWait)
	routes = append(routes, firstCompleteRoute)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), err
	}
	afterOneTurn, err := routeSmokeCounts(ctx, dsn, smokeSession)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), fmt.Errorf("route smoke one-turn count: %w", err)
	}

	secondCompleteBody := routeSmokeCompleteTurnBodyWithClientMeta(smokeSession, 9101, "second", clientMeta, liveCfg.Enabled)
	secondCompleteRoute, err := postJSONWithTimeout(ctx, baseURL+"/complete-turn", secondCompleteBody, liveCfg.HTTPWait)
	routes = append(routes, secondCompleteRoute)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), err
	}
	afterTwoTurns, err := routeSmokeCounts(ctx, dsn, smokeSession)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), fmt.Errorf("route smoke two-turn count: %w", err)
	}

	effectiveBody := map[string]any{
		"chat_session_id": smokeSession,
		"turn_index":      9102,
		"effective_input": "route smoke effective input",
	}
	effectiveRoute, err := postJSON(ctx, baseURL+"/effective-inputs", effectiveBody)
	routes = append(routes, effectiveRoute)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), err
	}

	canonicalRoutes := []struct {
		path string
		body map[string]any
	}{
		{"/canonical/" + smokeSession + "/chat-logs", map[string]any{
			"turn_index": 9201,
			"role":       "assistant",
			"content":    "canonical route smoke chat log",
		}},
		{"/canonical/" + smokeSession + "/effective-inputs", map[string]any{
			"turn_index":      9202,
			"effective_input": "canonical route smoke effective input",
		}},
		{"/canonical/" + smokeSession + "/memories", map[string]any{
			"turn_index":             9203,
			"summary_json":           `{"summary":"canonical route smoke memory"}`,
			"embedding":              "[]",
			"embedding_model":        "route-smoke",
			"importance":             0.5,
			"emotional_boost":        0.1,
			"evidence":               `{"text":"canonical route smoke evidence text"}`,
			"emotional_intensity":    0.2,
			"narrative_significance": 0.3,
			"place_wing":             "test",
			"place_room":             "route-smoke",
		}},
		{"/canonical/" + smokeSession + "/evidence", map[string]any{
			"evidence_kind":     "direct",
			"evidence_text":     "canonical route smoke direct evidence",
			"source_turn_start": 9201,
			"source_turn_end":   9203,
			"turn_anchor":       9203,
			"source_hash":       "route-smoke",
			"archive_state":     "active",
			"capture_stage":     "route_smoke",
		}},
		{"/canonical/" + smokeSession + "/kg-triples", map[string]any{
			"subject":     "RouteSmoke",
			"predicate":   "touches",
			"object":      "MariaDB",
			"valid_from":  9201,
			"valid_to":    0,
			"source_turn": 9203,
		}},
		{"/canonical/" + smokeSession + "/audit-logs", map[string]any{
			"event_type":   "route_smoke",
			"target_type":  "managed_mariadb_e2e",
			"target_id":    9203,
			"summary":      "canonical route smoke audit",
			"details_json": `{"source":"managed_mariadb_e2e"}`,
			"source":       "route_smoke",
		}},
		{"/canonical/" + smokeSession + "/critic-feedback", map[string]any{
			"target_type":    "turn",
			"target_id":      9203,
			"feedback_value": "ok",
			"feedback_note":  "canonical route smoke feedback",
			"source":         "route_smoke",
		}},
		{"/canonical/" + smokeSession + "/character-events", map[string]any{
			"character_name": "RouteSmoke",
			"turn_index":     9203,
			"event_type":     "smoke",
			"details_json":   `{"source":"managed_mariadb_e2e"}`,
		}},
	}
	for _, route := range canonicalRoutes {
		routeResult, err := postJSON(ctx, baseURL+route.path, route.body)
		routes = append(routes, routeResult)
		if err != nil {
			return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), err
		}
	}

	after, err := routeSmokeCounts(ctx, dsn, smokeSession)
	if err != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, nil, routes, storeMode), fmt.Errorf("route smoke post-count: %w", err)
	}

	delta := routeSmokeDelta(before, after)
	oneTurnDelta := routeSmokeDelta(before, afterOneTurn)
	twoTurnDelta := routeSmokeDelta(before, afterTwoTurns)
	nonCompleteRouteDelta := routeSmokeDelta(afterTwoTurns, after)
	contentChecks, contentErr := routeSmokeContentChecks(ctx, dsn, smokeSession, liveCfg)
	if contentErr != nil {
		return routeSmokeReport("failed", baseURL, smokeSession, before, after, routes, storeMode), fmt.Errorf("route smoke content check: %w", contentErr)
	}
	liveProviderChecks := routeSmokeLiveProviderResponseChecks(routes, liveCfg)
	expectedMin := map[string]int{
		"chat_logs":               5,
		"effective_input_logs":    4,
		"memories":                3,
		"direct_evidence_records": 3,
		"kg_triples":              3,
		"audit_logs":              5,
		"critic_feedback":         3,
		"character_events":        3,
		"entities":                2,
		"trust_states":            2,
		"storylines":              2,
		"world_rules":             2,
		"character_states":        2,
		"pending_threads":         2,
		"active_states":           6,
	}
	expectedCompleteMin := map[string]int{
		"chat_logs":               4,
		"effective_input_logs":    2,
		"memories":                2,
		"direct_evidence_records": 2,
		"kg_triples":              2,
		"audit_logs":              4,
		"critic_feedback":         2,
		"character_events":        2,
		"entities":                2,
		"trust_states":            2,
		"storylines":              2,
		"world_rules":             2,
		"character_states":        2,
		"pending_threads":         2,
		"active_states":           6,
	}
	ok := true
	for table, expected := range expectedMin {
		if delta[table] < expected {
			ok = false
			break
		}
	}
	for table, expected := range expectedCompleteMin {
		if twoTurnDelta[table] < expected {
			ok = false
			break
		}
	}
	if all, _ := contentChecks["all_expected_content_found"].(bool); !all {
		ok = false
	}
	if liveCfg.Enabled {
		if all, _ := liveProviderChecks["all_live_provider_checks_passed"].(bool); !all {
			ok = false
		}
	}
	status := "ok"
	if !ok {
		status = "failed"
	}
	report := routeSmokeReport(status, baseURL, smokeSession, before, after, routes, storeMode)
	report["delta_counts"] = delta
	report["one_turn_delta_counts"] = oneTurnDelta
	report["two_turn_delta_counts"] = twoTurnDelta
	report["complete_turn_delta_counts"] = twoTurnDelta
	report["non_complete_route_delta_counts"] = nonCompleteRouteDelta
	report["expected_min_delta"] = expectedMin
	report["expected_complete_turn_min_delta"] = expectedCompleteMin
	report["content_checks"] = contentChecks
	report["live_provider_checks"] = liveProviderChecks
	report["critic_stub_calls"] = routeSmokeCriticStubCalls(criticStub)
	report["critic_provider"] = criticProvider
	report["critic_provider_detail"] = routeSmokeLLMReport(liveCfg.Critic, liveCfg.Enabled)
	report["embedding_provider"] = embeddingProvider
	report["embedding_provider_detail"] = routeSmokeLLMReport(liveCfg.Embedding, liveCfg.Enabled)
	report["provider_mode"] = routeSmokeProviderMode(liveCfg)
	report["authority_switch"] = storeMode == "mariadb_authority"
	report["persistent_switch"] = false
	report["go_default_switch"] = false
	if !ok {
		return report, fmt.Errorf("route write smoke count delta below expectation: %+v", delta)
	}
	return report, nil
}
