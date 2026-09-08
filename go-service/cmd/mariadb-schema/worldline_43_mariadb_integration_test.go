package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

func TestWorldline43ReissuedNestedBranchMariaDBIntegration(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	revision := feedback43SeedSource(t, db, "A", 1)
	if _, err := db.Exec(`UPDATE memory_source_revisions SET source_message_id = 'a' WHERE source_revision = ?`, revision); err != nil {
		t.Fatal(err)
	}
	for host, sid := range map[string]string{"root": "A", "branch": "B", "child": "C"} {
		if _, err := st.(archiveStore.SessionRouteBindingStore).BindSessionRoute(ctx, archiveStore.SessionRouteBindingRequest{StableCharacterID: "stable", HostChatID: host, RequestedSessionID: sid, Mode: archiveStore.SessionRouteBindingModeResolveOrCreate}); err != nil {
			t.Fatal(err)
		}
	}
	row := func(index int, role, id string) map[string]any {
		return map[string]any{"message_index": index, "role": role, "message_chat_id": id, "disabled": false}
	}
	branch := func(parent, source, childPrefix string) map[string]any {
		return map[string]any{
			"contract_version": "risu_worldline_observation.v2", "host_signal_source": "output", "branch_shape_contract": "risu_branchedfrom.v1", "observed_at_ms": 1776000000000,
			"marker_state": "observed", "branch_marker": "{{specialcomment::branchedfrom::" + parent + "::Parent::" + source + "::}}", "marker_index": 2,
			"messages": []any{row(0, "user", childPrefix+"u"), row(1, "char", childPrefix+"a"), map[string]any{"message_index": 2, "role": "char", "disabled": true}},
		}
	}
	metadata := func(parent, parentPrefix, childPrefix string) map[string]any {
		return map[string]any{
			"contract_version": "risu_message_origins.v1", "parent_host_chat_id": parent,
			"parent_messages": []any{row(0, "user", parentPrefix+"u"), row(1, "char", parentPrefix+"a")},
			"child_messages":  []any{row(0, "user", childPrefix+"u"), row(1, "char", childPrefix+"a")},
		}
	}
	route := func(sid, host string, observation map[string]any) map[string]any {
		t.Helper()
		// New server on every request: origin identity must come from MariaDB.
		server := httpapi.NewServer(config.Config{})
		server.Cfg.StoreMode = config.StoreModeMariaDBAuthority
		server.Store = st
		mux := http.NewServeMux()
		server.RegisterRoutes(mux)
		body, _ := json.Marshal(map[string]any{"chat_session_id": sid, "mode": "identity", "stable_character_id": "stable", "stable_character_id_state": "observed", "host_chat_id": host, "host_chat_id_state": "observed", "worldline_observation": observation})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(string(body))))
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusOK || response["status"] != "ok" {
			t.Fatalf("route %d: %s", rec.Code, rec.Body.String())
		}
		vm, ok := response["worldline"].(map[string]any)
		if !ok {
			t.Fatalf("missing worldline: %s", rec.Body.String())
		}
		return vm
	}
	parent := branch("root", "a", "b-")
	first := route("B", "branch", parent)
	if first["state"] != "confirmed" {
		t.Fatalf("first=%+v", first)
	}
	parent["message_origins"] = metadata("root", "", "b-")
	if vm := route("B", "branch", parent); vm["message_origins_recorded"] != true {
		t.Fatalf("enriched=%+v", vm)
	}
	child := branch("branch", "b-a", "c-")
	child["message_origins"] = metadata("branch", "b-", "c-")
	vm := route("C", "child", child)
	if vm["state"] != "confirmed" || vm["parent_session_id"] != "B" || vm["fork_turn"] != float64(1) {
		t.Fatalf("nested=%+v", vm)
	}
	delete(child, "message_origins")
	child["branch_marker"] = ""
	child["marker_state"] = "absent"
	if got := route("C", "child", child); got["state"] != "confirmed" || got["message_origins_recorded"] != true {
		t.Fatalf("durable=%+v", got)
	}
	for _, sid := range []string{"B", "C"} {
		records, err := st.(archiveStore.ForkLineageStore).ListForkLineageRecords(ctx, sid, "", 0)
		if err != nil || len(records) != 1 || !strings.Contains(records[0].InheritedItemsJSON, "risu_message_origins.v1") {
			t.Fatalf("rows %s=%+v err=%v", sid, records, err)
		}
	}
}
