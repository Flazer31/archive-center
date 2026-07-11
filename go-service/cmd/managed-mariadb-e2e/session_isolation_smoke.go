package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func runSessionIsolationSmoke(ctx context.Context, baseURL string, criticStub *routeSmokeCriticStub, sessionPrefix string) (map[string]any, error) {
	sessionA := sessionPrefix + "-a"
	sessionB := sessionPrefix + "-b"
	routes := []map[string]any{}
	steps := []struct {
		session string
		turn    int
		label   string
	}{
		{session: sessionA, turn: 1, label: "session-a-first"},
		{session: sessionA, turn: 1, label: "session-a-second-stale-request"},
		{session: sessionB, turn: 1, label: "session-b-first"},
	}
	for _, step := range steps {
		result, err := postJSON(ctx, baseURL+"/complete-turn", routeSmokeCompleteTurnBody(step.session, step.turn, step.label, criticStub.URL()))
		routes = append(routes, result)
		if err != nil {
			return map[string]any{
				"status":   "failed",
				"sessions": []string{sessionA, sessionB},
				"routes":   routes,
			}, err
		}
	}

	sessionsResult, sessionsErr := probeGET(ctx, baseURL+"/sessions")
	timelineA, timelineAErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionA)+"&limit=40")
	timelineB, timelineBErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionB)+"&limit=40")

	sessionChecks := checkSessionListIsolation(sessionsResult, map[string]int{sessionA: 4, sessionB: 2})
	timelineACheck := checkTimelineIsolation(timelineA, sessionA, 4)
	timelineBCheck := checkTimelineIsolation(timelineB, sessionB, 2)
	ok := sessionsErr == nil && timelineAErr == nil && timelineBErr == nil &&
		boolField(sessionChecks, "ok") &&
		boolField(timelineACheck, "ok") &&
		boolField(timelineBCheck, "ok")

	status := "ok"
	if !ok {
		status = "failed"
	}
	report := map[string]any{
		"status":         status,
		"base_url":       baseURL,
		"sessions":       []string{sessionA, sessionB},
		"routes":         routes,
		"sessions_probe": sessionsResult,
		"timeline_a":     timelineA,
		"timeline_b":     timelineB,
		"checks": map[string]any{
			"sessions_list": sessionChecks,
			"timeline_a":    timelineACheck,
			"timeline_b":    timelineBCheck,
		},
		"expected_chat_log_counts": map[string]int{
			sessionA: 4,
			sessionB: 2,
		},
	}
	if !ok {
		errs := []string{}
		for _, err := range []error{sessionsErr, timelineAErr, timelineBErr} {
			if err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) == 0 {
			errs = append(errs, "session isolation checks failed")
		}
		report["errors"] = errs
		return report, errors.New(strings.Join(errs, "; "))
	}
	return report, nil
}

func checkSessionListIsolation(probe map[string]any, expected map[string]int) map[string]any {
	out := map[string]any{
		"ok":       false,
		"expected": expected,
		"found":    map[string]any{},
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "sessions probe failed"
		return out
	}
	body, _ := probe["json"].(map[string]any)
	rows, _ := body["sessions"].([]any)
	found := map[string]any{}
	ok := true
	for _, row := range rows {
		obj, _ := row.(map[string]any)
		sid := strings.TrimSpace(fmt.Sprint(obj["chat_session_id"]))
		if _, wants := expected[sid]; !wants {
			continue
		}
		chatCount := intFromAny(obj["chat_logs_count"])
		found[sid] = map[string]any{
			"chat_logs_count":  chatCount,
			"memories_count":   intFromAny(obj["memories_count"]),
			"kg_triples_count": intFromAny(obj["kg_triples_count"]),
		}
		if chatCount != expected[sid] {
			ok = false
		}
	}
	for sid := range expected {
		if _, exists := found[sid]; !exists {
			ok = false
		}
	}
	out["found"] = found
	out["ok"] = ok
	return out
}

func checkTimelineIsolation(probe map[string]any, sessionID string, expectedChatLogs int) map[string]any {
	out := map[string]any{
		"ok":                 false,
		"session_id":         sessionID,
		"expected_chat_logs": expectedChatLogs,
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "timeline probe failed"
		return out
	}
	body, _ := probe["json"].(map[string]any)
	items, _ := body["items"].([]any)
	meta, _ := body["meta"].(map[string]any)
	sourceCounts, _ := meta["source_counts"].(map[string]any)
	chatLogs := intFromAny(sourceCounts["chat_logs"])
	foreignItems := []string{}
	chatLogItems := 0
	for _, item := range items {
		obj, _ := item.(map[string]any)
		itemSession := strings.TrimSpace(fmt.Sprint(obj["chat_session_id"]))
		itemType := strings.TrimSpace(fmt.Sprint(obj["type"]))
		if itemSession != "" && itemSession != sessionID {
			foreignItems = append(foreignItems, itemSession)
		}
		if itemType == "chat_log" {
			chatLogItems++
		}
	}
	ok := chatLogs == expectedChatLogs && chatLogItems == expectedChatLogs && len(foreignItems) == 0
	out["source_counts_chat_logs"] = chatLogs
	out["chat_log_items"] = chatLogItems
	out["total_items"] = len(items)
	out["foreign_items"] = foreignItems
	out["ok"] = ok
	return out
}

func checkSessionDelete(deleteProbe map[string]any, sessionsProbe map[string]any, timelineProbe map[string]any, sessionID string) map[string]any {
	out := map[string]any{
		"ok":         false,
		"session_id": sessionID,
	}
	deleteOK := false
	if deleteProbe != nil && deleteProbe["status"] == "ok" {
		body, _ := deleteProbe["response"].(map[string]any)
		deleteOK = body["deleted"] == true && body["mutation_enabled"] == true
		out["delete_status"] = body["status"]
		out["delete_source"] = body["source"]
		out["delete_ok"] = deleteOK
	} else {
		out["delete_error"] = "delete probe failed"
	}

	stillListed := false
	if sessionsProbe != nil && sessionsProbe["status"] == "ok" {
		body, _ := sessionsProbe["json"].(map[string]any)
		rows, _ := body["sessions"].([]any)
		for _, row := range rows {
			obj, _ := row.(map[string]any)
			if strings.TrimSpace(fmt.Sprint(obj["chat_session_id"])) == sessionID {
				stillListed = true
				break
			}
		}
		out["still_listed"] = stillListed
	} else {
		out["sessions_after_delete_error"] = "sessions probe failed"
	}

	timelineEmpty := false
	timelineForeign := []string{}
	if timelineProbe != nil && timelineProbe["status"] == "ok" {
		body, _ := timelineProbe["json"].(map[string]any)
		items, _ := body["items"].([]any)
		for _, item := range items {
			obj, _ := item.(map[string]any)
			timelineForeign = append(timelineForeign, strings.TrimSpace(fmt.Sprint(obj["chat_session_id"])))
		}
		timelineEmpty = len(items) == 0
		out["timeline_items_after_delete"] = len(items)
		out["timeline_sessions_after_delete"] = timelineForeign
	} else {
		out["timeline_after_delete_error"] = "timeline probe failed"
	}

	out["ok"] = deleteOK && !stillListed && timelineEmpty
	return out
}

func checkRollbackMutation(deleteProbe map[string]any, chatProbe map[string]any, memProbe map[string]any, kgProbe map[string]any, auditProbe map[string]any, sessionID string, fromTurn int) map[string]any {
	out := map[string]any{
		"ok":         false,
		"session_id": sessionID,
		"from_turn":  fromTurn,
	}

	rollbackOK := false
	deletionsOK := false
	if deleteProbe != nil && deleteProbe["status"] == "ok" {
		body := responseJSONFromProbe(deleteProbe)
		plan := mapFromAny(body["rollback_plan"])
		deletions := mapFromAny(body["deletions"])
		required := []string{
			"chat_logs",
			"effective_inputs",
			"memories",
			"direct_evidence",
			"kg_triples",
			"critic_feedback",
			"character_events",
			"entities",
			"trust_states",
			"storylines",
			"world_rules",
			"character_states",
			"pending_threads",
			"active_states",
			"canonical_state_layers",
			"episode_summaries",
			"vectors",
			"rollback_audit",
		}
		missing := []string{}
		for _, key := range required {
			item := mapFromAny(deletions[key])
			if item["ok"] != true {
				missing = append(missing, key)
			}
		}
		deletionsOK = len(missing) == 0
		rollbackOK = body["status"] == "ok" &&
			plan["status"] == "executed" &&
			plan["mutation_enabled"] == true &&
			plan["would_delete"] == true
		out["rollback_status"] = body["status"]
		out["rollback_source"] = body["source"]
		out["rollback_plan"] = plan
		out["delete_keys_ok"] = deletionsOK
		out["delete_keys_missing_or_failed"] = missing
	} else {
		out["rollback_error"] = "rollback delete probe failed"
	}

	chatCheck := checkSessionScopedItems(chatProbe, sessionID, 2, "total")
	memCheck := checkSessionScopedItems(memProbe, sessionID, 1, "total")
	auditCheck := checkAuditTotal(auditProbe, 1)

	kgOK := false
	kgTotal := 0
	kgInvalidated := 0
	kgStillOpen := 0
	kgForeign := []string{}
	if kgProbe != nil && kgProbe["status"] == "ok" {
		body := responseJSONFromProbe(kgProbe)
		items, _ := body["items"].([]any)
		kgTotal = intFromAny(body["total"])
		for _, item := range items {
			obj := mapFromAny(item)
			itemSession := strings.TrimSpace(fmt.Sprint(obj["chat_session_id"]))
			if itemSession != "" && itemSession != sessionID {
				kgForeign = append(kgForeign, itemSession)
			}
			validTo := intFromAny(obj["valid_to"])
			if validTo == fromTurn-1 {
				kgInvalidated++
			}
			if validTo == 0 || validTo >= fromTurn {
				kgStillOpen++
			}
		}
		kgOK = kgTotal == 2 && len(items) == 2 && kgInvalidated >= 1 && kgStillOpen >= 1 && len(kgForeign) == 0
	}
	kgCheck := map[string]any{
		"ok":                   kgOK,
		"total":                kgTotal,
		"invalidated_valid_to": kgInvalidated,
		"still_open":           kgStillOpen,
		"foreign_items":        kgForeign,
		"expected_total":       2,
		"expected_valid_to":    fromTurn - 1,
	}

	out["checks"] = map[string]any{
		"rollback_response": rollbackOK,
		"delete_keys":       deletionsOK,
		"chat_logs":         chatCheck,
		"memories":          memCheck,
		"kg_valid_to":       kgCheck,
		"audit":             auditCheck,
	}
	out["ok"] = rollbackOK &&
		deletionsOK &&
		boolField(chatCheck, "ok") &&
		boolField(memCheck, "ok") &&
		boolField(kgCheck, "ok") &&
		boolField(auditCheck, "ok")
	return out
}

func responseJSONFromProbe(probe map[string]any) map[string]any {
	if probe == nil {
		return map[string]any{}
	}
	if body, ok := probe["json"].(map[string]any); ok {
		return body
	}
	if body, ok := probe["response"].(map[string]any); ok {
		return body
	}
	return map[string]any{}
}

func checkSessionScopedItems(probe map[string]any, sessionID string, expectedItems int, countField string) map[string]any {
	out := map[string]any{
		"ok":             false,
		"session_id":     sessionID,
		"expected_items": expectedItems,
		"count_field":    countField,
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	items, _ := body["items"].([]any)
	foreignItems := []string{}
	for _, item := range items {
		obj, _ := item.(map[string]any)
		itemSession := strings.TrimSpace(fmt.Sprint(obj["chat_session_id"]))
		if itemSession != "" && itemSession != sessionID {
			foreignItems = append(foreignItems, itemSession)
		}
	}
	count := len(items)
	if countField != "" {
		if value, exists := body[countField]; exists {
			count = intFromAny(value)
		}
	}
	out["item_count"] = len(items)
	out["count"] = count
	out["foreign_items"] = foreignItems
	out["ok"] = count == expectedItems && len(items) == expectedItems && len(foreignItems) == 0
	return out
}

func checkStatsCounts(probe map[string]any, expected map[string]int) map[string]any {
	out := map[string]any{"ok": false, "expected": expected}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	actual := map[string]int{}
	ok := true
	for key, want := range expected {
		got := intFromAny(body[key])
		actual[key] = got
		if got != want {
			ok = false
		}
	}
	out["actual"] = actual
	out["ok"] = ok
	return out
}

func firstItemID(probe map[string]any) int64 {
	body := responseJSONFromProbe(probe)
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		return 0
	}
	item, _ := items[0].(map[string]any)
	return int64(intFromAny(item["id"]))
}

func checkFeedbackPost(probe map[string]any, targetID int64) map[string]any {
	out := map[string]any{"ok": false, "target_id": targetID}
	if targetID <= 0 {
		out["error"] = "target id missing"
		return out
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	out["response_status"] = body["status"]
	out["feedback_value"] = body["feedback_value"]
	out["feedback_id"] = body["feedback_id"]
	out["ok"] = body["status"] == "ok" && body["ok"] == true && body["feedback_value"] == "up"
	return out
}

func checkFeedbackLatest(probe map[string]any, targetID int64, expectedValue string) map[string]any {
	out := map[string]any{"ok": false, "target_id": targetID, "expected_value": expectedValue}
	if targetID <= 0 {
		out["error"] = "target id missing"
		return out
	}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	feedbacks, _ := body["feedbacks"].(map[string]any)
	item, _ := feedbacks[strconv.FormatInt(targetID, 10)].(map[string]any)
	out["count"] = intFromAny(body["count"])
	out["feedbacks_count"] = len(feedbacks)
	out["actual_value"] = item["feedback_value"]
	out["ok"] = body["status"] == "ok" && item["feedback_value"] == expectedValue
	return out
}

func checkAuditTotal(probe map[string]any, minTotal int) map[string]any {
	out := map[string]any{"ok": false, "min_total": minTotal}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	total := intFromAny(body["total"])
	items, _ := body["items"].([]any)
	out["total"] = total
	out["item_count"] = len(items)
	out["ok"] = body["status"] == "ok" && total >= minTotal && len(items) >= minTotal
	return out
}

func checkShadowGuard(probe map[string]any) map[string]any {
	out := map[string]any{"ok": false}
	if probe == nil {
		out["error"] = "probe missing"
		return out
	}
	body := responseJSONFromProbe(probe)
	out["http_status"] = intFromAny(probe["http_status"])
	out["code"] = body["code"]
	out["status"] = body["status"]
	out["ok"] = intFromAny(probe["http_status"]) == http.StatusServiceUnavailable && body["code"] == "shadow_guard"
	return out
}

func checkSessionsCompare(probe map[string]any, expected map[string]map[string]int) map[string]any {
	out := map[string]any{"ok": false, "expected": expected}
	if probe == nil || probe["status"] != "ok" {
		out["error"] = "probe failed"
		return out
	}
	body := responseJSONFromProbe(probe)
	sessions, _ := body["sessions"].(map[string]any)
	actual := map[string]map[string]int{}
	ok := body["status"] == "ok"
	for sid, expectedCounts := range expected {
		payload, _ := sessions[sid].(map[string]any)
		counts, _ := payload["counts"].(map[string]any)
		actualCounts := map[string]int{}
		for key, want := range expectedCounts {
			got := intFromAny(counts[key])
			actualCounts[key] = got
			if got != want {
				ok = false
			}
		}
		logs, _ := payload["logs_preview"].([]any)
		memories, _ := payload["memories_preview"].([]any)
		kgTriples, _ := payload["kg_triples"].([]any)
		actualCounts["logs_preview"] = len(logs)
		actualCounts["memories_preview"] = len(memories)
		actualCounts["kg_preview"] = len(kgTriples)
		if len(logs) == 0 || len(memories) == 0 || len(kgTriples) == 0 {
			ok = false
		}
		actual[sid] = actualCounts
	}
	out["actual"] = actual
	out["ok"] = ok
	return out
}

func runSessionIsolationSmokeStandalone(ctx context.Context, port int, sessionPrefix string) (map[string]any, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	sessionA := safeSessionID(sessionPrefix) + "-rmg03-standalone-a"
	sessionB := safeSessionID(sessionPrefix) + "-rmg03-standalone-b"
	routes := []map[string]any{}

	criticStub := startRouteSmokeCriticStub()
	defer criticStub.Close()

	// Session A: two distinct turns
	for _, turn := range []int{1, 2} {
		body := routeSmokeCompleteTurnBody(sessionA, turn, fmt.Sprintf("standalone-a-turn-%d", turn), criticStub.URL())
		result, err := postJSON(ctx, baseURL+"/complete-turn", body)
		routes = append(routes, result)
		if err != nil {
			return map[string]any{
				"status":   "failed",
				"sessions": []string{sessionA, sessionB},
				"routes":   routes,
			}, err
		}
	}

	// Session B: one turn
	bodyB := routeSmokeCompleteTurnBody(sessionB, 1, "standalone-b-turn-1", criticStub.URL())
	resultB, err := postJSON(ctx, baseURL+"/complete-turn", bodyB)
	routes = append(routes, resultB)
	if err != nil {
		return map[string]any{
			"status":   "failed",
			"sessions": []string{sessionA, sessionB},
			"routes":   routes,
		}, err
	}

	sessionsResult, sessionsErr := probeGET(ctx, baseURL+"/sessions")
	timelineA, timelineAErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionA)+"&limit=40")
	timelineB, timelineBErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionB)+"&limit=40")
	searchA, searchAErr := postJSON(ctx, baseURL+"/search", map[string]any{"chat_session_id": sessionA, "user_input": "route smoke search A", "top_k": 10})
	searchB, searchBErr := postJSON(ctx, baseURL+"/search", map[string]any{"chat_session_id": sessionB, "user_input": "route smoke search B", "top_k": 10})
	explorerChatA, explorerChatAErr := probeGET(ctx, baseURL+"/explorer/chat_logs?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerChatB, explorerChatBErr := probeGET(ctx, baseURL+"/explorer/chat_logs?chat_session_id="+url.QueryEscape(sessionB)+"&limit=40")
	explorerMemA, explorerMemAErr := probeGET(ctx, baseURL+"/explorer/memories?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerMemB, explorerMemBErr := probeGET(ctx, baseURL+"/explorer/memories?chat_session_id="+url.QueryEscape(sessionB)+"&limit=40")
	explorerKGA, explorerKGAErr := probeGET(ctx, baseURL+"/explorer/kg_triples?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerKGB, explorerKGBErr := probeGET(ctx, baseURL+"/explorer/kg_triples?chat_session_id="+url.QueryEscape(sessionB)+"&limit=40")
	statsBeforeDelete, statsBeforeDeleteErr := probeGET(ctx, baseURL+"/stats")

	// Each /complete-turn writes 2 chat logs (user + assistant) plus critic-derived rows.
	// With the managed stub, sessionA (2 turns) -> 4 chat logs, sessionB (1 turn) -> 2 chat logs.
	sessionChecks := checkSessionListIsolation(sessionsResult, map[string]int{sessionA: 4, sessionB: 2})
	timelineACheck := checkTimelineIsolation(timelineA, sessionA, 4)
	timelineBCheck := checkTimelineIsolation(timelineB, sessionB, 2)
	searchACheck := checkSessionScopedItems(searchA, sessionA, 2, "memory_count")
	searchBCheck := checkSessionScopedItems(searchB, sessionB, 1, "memory_count")
	explorerChatACheck := checkSessionScopedItems(explorerChatA, sessionA, 4, "total")
	explorerChatBCheck := checkSessionScopedItems(explorerChatB, sessionB, 2, "total")
	explorerMemACheck := checkSessionScopedItems(explorerMemA, sessionA, 2, "total")
	explorerMemBCheck := checkSessionScopedItems(explorerMemB, sessionB, 1, "total")
	explorerKGACheck := checkSessionScopedItems(explorerKGA, sessionA, 2, "total")
	explorerKGBCheck := checkSessionScopedItems(explorerKGB, sessionB, 1, "total")
	statsCheck := checkStatsCounts(statsBeforeDelete, map[string]int{"chat_logs": 6, "memories": 3, "kg_triples": 3})
	memoryAID := firstItemID(explorerMemA)
	feedbackPost := map[string]any{"status": "skipped", "reason": "no memory id found for session A"}
	var feedbackPostErr error
	feedbackLatest := map[string]any{"status": "skipped", "reason": "no memory id found for session A"}
	var feedbackLatestErr error
	auditFeedback := map[string]any{"status": "skipped", "reason": "feedback post skipped"}
	var auditFeedbackErr error
	protectedMutation := map[string]any{"status": "skipped", "reason": "no memory id found for session A"}
	var protectedMutationErr error
	if memoryAID > 0 {
		feedbackPost, feedbackPostErr = postJSON(ctx, baseURL+"/feedback", map[string]any{
			"chat_session_id": sessionA,
			"target_type":     "memory",
			"target_id":       memoryAID,
			"feedback_value":  "up",
			"feedback_note":   "rmg23 managed smoke",
		})
		feedbackLatest, feedbackLatestErr = probeGET(ctx, baseURL+"/feedback/latest?chat_session_id="+url.QueryEscape(sessionA)+"&target_type=memory&target_ids="+strconv.FormatInt(memoryAID, 10))
		auditFeedback, auditFeedbackErr = probeGET(ctx, baseURL+"/audit?chat_session_id="+url.QueryEscape(sessionA)+"&event_type=critic_feedback&limit=10")
		protectedMutation, protectedMutationErr = patchJSONProbe(ctx, baseURL+"/explorer/memories/"+strconv.FormatInt(memoryAID, 10), map[string]any{
			"chat_session_id": sessionB,
			"importance":      0.1,
		})
	}
	feedbackPostCheck := checkFeedbackPost(feedbackPost, memoryAID)
	feedbackLatestCheck := checkFeedbackLatest(feedbackLatest, memoryAID, "up")
	auditFeedbackCheck := checkAuditTotal(auditFeedback, 1)
	protectedMutationCheck := checkShadowGuard(protectedMutation)
	compareAB, compareABErr := probeGET(ctx, baseURL+"/sessions/compare?session_ids="+url.QueryEscape(sessionA+","+sessionB)+"&preview_limit=2")
	compareABCheck := checkSessionsCompare(compareAB, map[string]map[string]int{
		sessionA: map[string]int{"chat_logs": 4, "memories": 2, "kg_triples": 2, "feedback_up": 1},
		sessionB: map[string]int{"chat_logs": 2, "memories": 1, "kg_triples": 1, "feedback_up": 0},
	})
	deleteB, deleteBErr := deleteJSON(ctx, baseURL+"/sessions/"+url.PathEscape(sessionB))
	sessionsAfterDelete, sessionsAfterDeleteErr := probeGET(ctx, baseURL+"/sessions")
	timelineBAfterDelete, timelineBAfterDeleteErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionB)+"&limit=40")
	deleteBCheck := checkSessionDelete(deleteB, sessionsAfterDelete, timelineBAfterDelete, sessionB)
	rollbackA, rollbackAErr := deleteJSON(ctx, baseURL+"/rollback/2?chat_session_id="+url.QueryEscape(sessionA)+"&req_source=auto_rollback")
	explorerChatAAfterRollback, explorerChatAAfterRollbackErr := probeGET(ctx, baseURL+"/explorer/chat_logs?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerMemAAfterRollback, explorerMemAAfterRollbackErr := probeGET(ctx, baseURL+"/explorer/memories?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	explorerKGAAfterRollback, explorerKGAAfterRollbackErr := probeGET(ctx, baseURL+"/explorer/kg_triples?chat_session_id="+url.QueryEscape(sessionA)+"&limit=40")
	auditRollbackA, auditRollbackAErr := probeGET(ctx, baseURL+"/audit?chat_session_id="+url.QueryEscape(sessionA)+"&event_type=rollback&limit=10")
	timelineAAfterRollback, timelineAAfterRollbackErr := probeGET(ctx, baseURL+"/timeline?sessionId="+url.QueryEscape(sessionA)+"&limit=40")
	rollbackACheck := checkRollbackMutation(rollbackA, explorerChatAAfterRollback, explorerMemAAfterRollback, explorerKGAAfterRollback, auditRollbackA, sessionA, 2)
	timelineAAfterRollbackCheck := checkTimelineIsolation(timelineAAfterRollback, sessionA, 2)
	ok := sessionsErr == nil && timelineAErr == nil && timelineBErr == nil &&
		searchAErr == nil && searchBErr == nil &&
		explorerChatAErr == nil && explorerChatBErr == nil &&
		explorerMemAErr == nil && explorerMemBErr == nil &&
		explorerKGAErr == nil && explorerKGBErr == nil &&
		statsBeforeDeleteErr == nil &&
		feedbackPostErr == nil && feedbackLatestErr == nil && auditFeedbackErr == nil && protectedMutationErr == nil && compareABErr == nil &&
		deleteBErr == nil && sessionsAfterDeleteErr == nil && timelineBAfterDeleteErr == nil &&
		rollbackAErr == nil &&
		explorerChatAAfterRollbackErr == nil && explorerMemAAfterRollbackErr == nil && explorerKGAAfterRollbackErr == nil &&
		auditRollbackAErr == nil && timelineAAfterRollbackErr == nil &&
		boolField(sessionChecks, "ok") &&
		boolField(timelineACheck, "ok") &&
		boolField(timelineBCheck, "ok") &&
		boolField(searchACheck, "ok") &&
		boolField(searchBCheck, "ok") &&
		boolField(explorerChatACheck, "ok") &&
		boolField(explorerChatBCheck, "ok") &&
		boolField(explorerMemACheck, "ok") &&
		boolField(explorerMemBCheck, "ok") &&
		boolField(explorerKGACheck, "ok") &&
		boolField(explorerKGBCheck, "ok") &&
		boolField(statsCheck, "ok") &&
		boolField(feedbackPostCheck, "ok") &&
		boolField(feedbackLatestCheck, "ok") &&
		boolField(auditFeedbackCheck, "ok") &&
		boolField(protectedMutationCheck, "ok") &&
		boolField(compareABCheck, "ok") &&
		boolField(deleteBCheck, "ok") &&
		boolField(rollbackACheck, "ok") &&
		boolField(timelineAAfterRollbackCheck, "ok")

	status := "ok"
	if !ok {
		status = "failed"
	}
	report := map[string]any{
		"status":         status,
		"base_url":       baseURL,
		"sessions":       []string{sessionA, sessionB},
		"routes":         routes,
		"sessions_probe": sessionsResult,
		"timeline_a":     timelineA,
		"timeline_b":     timelineB,
		"search_a":       searchA,
		"search_b":       searchB,
		"explorer": map[string]any{
			"chat_logs_a":  explorerChatA,
			"chat_logs_b":  explorerChatB,
			"memories_a":   explorerMemA,
			"memories_b":   explorerMemB,
			"kg_triples_a": explorerKGA,
			"kg_triples_b": explorerKGB,
		},
		"rmg23": map[string]any{
			"status":              status,
			"memory_a_id":         memoryAID,
			"stats":               statsBeforeDelete,
			"feedback_post":       feedbackPost,
			"feedback_latest":     feedbackLatest,
			"audit_feedback":      auditFeedback,
			"sessions_compare":    compareAB,
			"protected_mutation":  protectedMutation,
			"scope":               "SEQ-02 canonical read/control proof for stats, audit, feedback, compare, and guarded DB editing",
			"product_green":       false,
			"remaining_rmg23_gap": "manual DB editing and edit-history audit are still guarded/not product-green",
		},
		"delete_session_b": map[string]any{
			"delete_probe":          deleteB,
			"sessions_after_delete": sessionsAfterDelete,
			"timeline_after_delete": timelineBAfterDelete,
		},
		"rmg04": map[string]any{
			"status":                    status,
			"rollback_probe":            rollbackA,
			"chat_logs_after_rollback":  explorerChatAAfterRollback,
			"memories_after_rollback":   explorerMemAAfterRollback,
			"kg_triples_after_rollback": explorerKGAAfterRollback,
			"audit_after_rollback":      auditRollbackA,
			"timeline_after_rollback":   timelineAAfterRollback,
			"scope":                     "SEQ-03 actual rollback mutation proof on a writable MariaDB authority store",
			"product_green":             false,
			"remaining_rmg04_gap":       "JS live RisuAI session-delete UI update, repair-replay rebuild, and later guidance/session-active-scope/maintenance cleanup remain open",
		},
		"checks": map[string]any{
			"sessions_list":             sessionChecks,
			"timeline_a":                timelineACheck,
			"timeline_b":                timelineBCheck,
			"search_a":                  searchACheck,
			"search_b":                  searchBCheck,
			"explorer_chat_a":           explorerChatACheck,
			"explorer_chat_b":           explorerChatBCheck,
			"explorer_mem_a":            explorerMemACheck,
			"explorer_mem_b":            explorerMemBCheck,
			"explorer_kg_a":             explorerKGACheck,
			"explorer_kg_b":             explorerKGBCheck,
			"stats":                     statsCheck,
			"feedback_post":             feedbackPostCheck,
			"feedback_latest":           feedbackLatestCheck,
			"audit_feedback":            auditFeedbackCheck,
			"protected_edit":            protectedMutationCheck,
			"sessions_compare":          compareABCheck,
			"delete_session_b":          deleteBCheck,
			"rollback_session_a":        rollbackACheck,
			"timeline_a_after_rollback": timelineAAfterRollbackCheck,
		},
		"expected_chat_log_counts": map[string]int{
			sessionA: 4,
			sessionB: 2,
		},
	}
	if !ok {
		errs := []string{}
		for _, err := range []error{
			sessionsErr, timelineAErr, timelineBErr,
			searchAErr, searchBErr,
			explorerChatAErr, explorerChatBErr,
			explorerMemAErr, explorerMemBErr,
			explorerKGAErr, explorerKGBErr,
			statsBeforeDeleteErr,
			feedbackPostErr, feedbackLatestErr, auditFeedbackErr, protectedMutationErr, compareABErr,
			deleteBErr, sessionsAfterDeleteErr, timelineBAfterDeleteErr,
			rollbackAErr, explorerChatAAfterRollbackErr, explorerMemAAfterRollbackErr, explorerKGAAfterRollbackErr,
			auditRollbackAErr, timelineAAfterRollbackErr,
		} {
			if err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) == 0 {
			errs = append(errs, "session isolation checks failed")
		}
		report["errors"] = errs
		return report, errors.New(strings.Join(errs, "; "))
	}
	return report, nil
}
func boolField(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	default:
		i, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
		return i
	}
}

func mapFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

type routeSmokeCriticStub struct {
	server *httptest.Server
	calls  atomic.Int64
}

func routeSmokeLiveConfigFromEnv() routeSmokeLiveConfig {
	enabled := routeSmokeEnvBool("AC_RMG02_ROUTE_SMOKE_LIVE_PROVIDER")
	waitMs := routeSmokeEnvInt64("AC_RMG02_ROUTE_SMOKE_HTTP_TIMEOUT_MS", 180000)
	if waitMs <= 0 {
		waitMs = 180000
	}
	return routeSmokeLiveConfig{
		Enabled:  enabled,
		HTTPWait: time.Duration(waitMs) * time.Millisecond,
		Critic: routeSmokeLLMConfig{
			Provider:            routeSmokeEnv("AC_RMG02_CRITIC_PROVIDER", "openai"),
			Endpoint:            routeSmokeEnv("AC_RMG02_CRITIC_ENDPOINT", "http://127.0.0.1:11434/v1"),
			APIKey:              routeSmokeEnv("AC_RMG02_CRITIC_API_KEY", "ollama-local"),
			Model:               routeSmokeEnv("AC_RMG02_CRITIC_MODEL", "glm-5.1:cloud"),
			TimeoutMs:           routeSmokeEnvInt64("AC_RMG02_CRITIC_TIMEOUT_MS", 120000),
			Temperature:         routeSmokeEnvFloat("AC_RMG02_CRITIC_TEMPERATURE", 0),
			MaxTokens:           routeSmokeEnvInt64("AC_RMG02_CRITIC_MAX_TOKENS", 1800),
			MaxCompletionTokens: routeSmokeEnvInt64("AC_RMG02_CRITIC_MAX_COMPLETION_TOKENS", 1800),
			ReasoningEffort:     routeSmokeEnv("AC_RMG02_CRITIC_REASONING_EFFORT", ""),
		},
		Embedding: routeSmokeLLMConfig{
			Provider:  routeSmokeEnv("AC_RMG02_EMBEDDING_PROVIDER", "ollama"),
			Endpoint:  routeSmokeEnv("AC_RMG02_EMBEDDING_ENDPOINT", "http://127.0.0.1:11434"),
			APIKey:    routeSmokeEnv("AC_RMG02_EMBEDDING_API_KEY", "ollama-local"),
			Model:     routeSmokeEnv("AC_RMG02_EMBEDDING_MODEL", "nomic-embed-text"),
			TimeoutMs: routeSmokeEnvInt64("AC_RMG02_EMBEDDING_TIMEOUT_MS", 60000),
		},
	}
}

func routeSmokeEnv(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func routeSmokeEnvBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "y", "on", "live":
		return true
	default:
		return false
	}
}

func routeSmokeEnvInt64(name string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func routeSmokeEnvFloat(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func routeSmokeStubClientMeta(criticEndpoint string) map[string]any {
	return map[string]any{
		"critic": map[string]any{
			"provider":    "openai",
			"endpoint":    criticEndpoint,
			"api_key":     "managed-route-smoke-key",
			"model":       "route-smoke-critic",
			"timeout_ms":  10000,
			"temperature": 0,
			"max_tokens":  800,
		},
	}
}

func routeSmokeLiveClientMeta(cfg routeSmokeLiveConfig) map[string]any {
	critic := map[string]any{
		"provider":              cfg.Critic.Provider,
		"endpoint":              cfg.Critic.Endpoint,
		"api_key":               cfg.Critic.APIKey,
		"model":                 cfg.Critic.Model,
		"timeout_ms":            cfg.Critic.TimeoutMs,
		"temperature":           cfg.Critic.Temperature,
		"max_tokens":            cfg.Critic.MaxTokens,
		"max_completion_tokens": cfg.Critic.MaxCompletionTokens,
	}
	if strings.TrimSpace(cfg.Critic.ReasoningEffort) != "" {
		critic["reasoning_effort"] = cfg.Critic.ReasoningEffort
	}
	return map[string]any{
		"critic": critic,
		"embedding": map[string]any{
			"provider":   cfg.Embedding.Provider,
			"endpoint":   cfg.Embedding.Endpoint,
			"api_key":    cfg.Embedding.APIKey,
			"model":      cfg.Embedding.Model,
			"timeout_ms": cfg.Embedding.TimeoutMs,
		},
	}
}

func routeSmokeProviderMode(cfg routeSmokeLiveConfig) string {
	if cfg.Enabled {
		return "configured_live_provider"
	}
	return "managed_stub_openai_compatible"
}

func routeSmokeLLMReport(cfg routeSmokeLLMConfig, enabled bool) map[string]any {
	if !enabled {
		return map[string]any{"configured": false}
	}
	return map[string]any{
		"configured":    true,
		"provider":      cfg.Provider,
		"endpoint_host": routeSmokeEndpointHost(cfg.Endpoint),
		"model":         cfg.Model,
		"timeout_ms":    cfg.TimeoutMs,
	}
}

func routeSmokeEndpointHost(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return strings.TrimSpace(endpoint)
}
