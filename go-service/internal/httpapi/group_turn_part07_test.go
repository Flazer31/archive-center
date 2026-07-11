package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPrepareTurnPromptAssemblyNotConfigured(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-pa","turn_index":1,"raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}

	pa, ok := gp["prompt_assembly"].(map[string]any)
	if !ok {
		t.Fatalf("prompt_assembly missing")
	}

	if pa["prompt_source"] != "not_configured" {
		t.Errorf("prompt_source = %v, want not_configured", pa["prompt_source"])
	}
	if pa["files_found"] != float64(0) {
		t.Errorf("files_found = %v, want 0", pa["files_found"])
	}
	if pa["would_call_llm"] != false {
		t.Errorf("would_call_llm = %v, want false", pa["would_call_llm"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "degraded" {
		t.Errorf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["source"] != "go_r1_read_shadow" {
		t.Errorf("recall_result.source = %v, want go_r1_read_shadow", rr["source"])
	}
	if rr["would_write"] != false {
		t.Errorf("would_write = %v, want false", rr["would_write"])
	}
	vs, ok := rr["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow missing")
	}
	if vs["status"] != "shadow" {
		t.Errorf("vector_shadow.status = %v, want shadow", vs["status"])
	}
	if vs["health_checked"] != true {
		t.Errorf("vector_shadow.health_checked = %v, want true", vs["health_checked"])
	}
	if vs["search_attempted"] != false {
		t.Errorf("vector_shadow.search_attempted = %v, want false", vs["search_attempted"])
	}

	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	if ss["snapshot_status"] != "degraded" {
		t.Errorf("session_state.snapshot_status = %v, want degraded", ss["snapshot_status"])
	}

	nc, ok := resp["narrative_control"].(map[string]any)
	if !ok {
		t.Fatalf("narrative_control is not an object")
	}
	if nc["state_status"] != "skeleton" {
		t.Errorf("narrative_control.state_status = %v, want skeleton", nc["state_status"])
	}

	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_pack is not an object")
	}
	if cp["status"] != "degraded" {
		t.Errorf("continuity_pack.status = %v, want degraded", cp["status"])
	}
	if cp["would_call_llm"] != false {
		t.Errorf("continuity_pack.would_call_llm = %v, want false", cp["would_call_llm"])
	}

	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("progression_ledger is not an object")
	}
	if pl["status"] != "degraded" {
		t.Errorf("progression_ledger.status = %v, want degraded", pl["status"])
	}
	if pl["would_write"] != false {
		t.Errorf("progression_ledger.would_write = %v, want false", pl["would_write"])
	}

	ap, ok := resp["autonomy_plan"].(map[string]any)
	if !ok {
		t.Fatalf("autonomy_plan is not an object")
	}
	if ap["status"] != "degraded" {
		t.Errorf("autonomy_plan.status = %v, want degraded", ap["status"])
	}
	if ap["suggested_action"] != "continue" {
		t.Errorf("autonomy_plan.suggested_action = %v, want continue", ap["suggested_action"])
	}
	if ap["would_call_llm"] != false {
		t.Errorf("autonomy_plan.would_call_llm = %v, want false", ap["would_call_llm"])
	}
	if ap["would_write"] != false {
		t.Errorf("autonomy_plan.would_write = %v, want false", ap["would_write"])
	}

	mb, ok := resp["micro_beat_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("micro_beat_proposal is not an object")
	}
	if mb["status"] != "degraded" {
		t.Errorf("micro_beat_proposal.status = %v, want degraded", mb["status"])
	}
	mbBeats, _ := mb["beats"].([]any)
	if len(mbBeats) != 0 {
		t.Errorf("micro_beat_proposal.beats len = %d, want 0", len(mbBeats))
	}
	if mb["would_call_llm"] != false {
		t.Errorf("micro_beat_proposal.would_call_llm = %v, want false", mb["would_call_llm"])
	}

	sp, ok := resp["scene_step_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("scene_step_proposal is not an object")
	}
	if sp["status"] != "degraded" {
		t.Errorf("scene_step_proposal.status = %v, want degraded", sp["status"])
	}
	spSteps, _ := sp["steps"].([]any)
	if len(spSteps) != 0 {
		t.Errorf("scene_step_proposal.steps len = %d, want 0", len(spSteps))
	}
	if sp["would_call_llm"] != false {
		t.Errorf("scene_step_proposal.would_call_llm = %v, want false", sp["would_call_llm"])
	}

	cp2, ok := resp["combined_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("combined_proposal is not an object")
	}
	if cp2["status"] != "degraded" {
		t.Errorf("combined_proposal.status = %v, want degraded", cp2["status"])
	}
	if cp2["micro_beat_count"] != float64(0) {
		t.Errorf("combined_proposal.micro_beat_count = %v, want 0", cp2["micro_beat_count"])
	}
	if cp2["scene_step_count"] != float64(0) {
		t.Errorf("combined_proposal.scene_step_count = %v, want 0", cp2["scene_step_count"])
	}
	if cp2["source"] != "go_r1_read_shadow" {
		t.Errorf("combined_proposal.source = %v, want go_r1_read_shadow", cp2["source"])
	}
	if cp2["would_call_llm"] != false {
		t.Errorf("combined_proposal.would_call_llm = %v, want false", cp2["would_call_llm"])
	}
	if cp2["would_write"] != false {
		t.Errorf("combined_proposal.would_write = %v, want false", cp2["would_write"])
	}

	wp, ok := resp["writeback_preview"].(map[string]any)
	if !ok {
		t.Fatalf("writeback_preview is not an object")
	}
	if wp["status"] != "degraded" {
		t.Errorf("writeback_preview.status = %v, want degraded", wp["status"])
	}
	if wp["would_write"] != false {
		t.Errorf("writeback_preview.would_write = %v, want false", wp["would_write"])
	}
}

func TestPrepareTurnPromptAssemblyWithDir(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "supervisor_system.txt"), []byte("supervisor system content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "critic_prompt.txt"), []byte("critic prompt content here"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg := config.Default()
	cfg.PromptDir = tmpDir
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-pa2","turn_index":1,"raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}

	pa, ok := gp["prompt_assembly"].(map[string]any)
	if !ok {
		t.Fatalf("prompt_assembly missing")
	}

	if pa["prompt_source"] != "configured" {
		t.Errorf("prompt_source = %v, want configured", pa["prompt_source"])
	}
	if pa["files_found"] != float64(2) {
		t.Errorf("files_found = %v, want 2", pa["files_found"])
	}
	if pa["total_chars"] != float64(51) {
		t.Errorf("total_chars = %v, want 51", pa["total_chars"])
	}
	if pa["would_call_llm"] != false {
		t.Errorf("would_call_llm = %v, want false", pa["would_call_llm"])
	}

	files, ok := pa["files"].([]any)
	if !ok {
		t.Fatalf("files is not an array")
	}
	if len(files) != 4 {
		t.Fatalf("files len = %d, want 4", len(files))
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "degraded" {
		t.Errorf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["would_write"] != false {
		t.Errorf("would_write = %v, want false", rr["would_write"])
	}
	vs, ok := rr["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow missing")
	}
	if vs["status"] != "shadow" {
		t.Errorf("vector_shadow.status = %v, want shadow", vs["status"])
	}
	if vs["health_checked"] != true {
		t.Errorf("vector_shadow.health_checked = %v, want true", vs["health_checked"])
	}
	if vs["search_attempted"] != false {
		t.Errorf("vector_shadow.search_attempted = %v, want false", vs["search_attempted"])
	}
}

func TestRepairReplayShadowPlan(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-repair","dry_run":true,"entries":[{"assistant_content":"entry one"},{"assistant_content":"entry two"},{"assistant_content":"entry three"},{"assistant_content":"entry four"}]}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
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
		t.Errorf("status = %v, want ok", resp["status"])
	}

	rp, ok := resp["repair_replay_plan"].(map[string]any)
	if !ok {
		t.Fatalf("repair_replay_plan is not an object")
	}
	if rp["status"] != "shadow_plan" {
		t.Errorf("repair_replay_plan.status = %v, want shadow_plan", rp["status"])
	}
	if rp["entries_count"] != float64(4) {
		t.Errorf("repair_replay_plan.entries_count = %v, want 4", rp["entries_count"])
	}
	if rp["dry_run"] != true {
		t.Errorf("repair_replay_plan.dry_run = %v, want true", rp["dry_run"])
	}
	if rp["would_replay"] != false {
		t.Errorf("repair_replay_plan.would_replay = %v, want false", rp["would_replay"])
	}
	if rp["would_write"] != false {
		t.Errorf("repair_replay_plan.would_write = %v, want false", rp["would_write"])
	}
	if rp["mutation_enabled"] != false {
		t.Errorf("repair_replay_plan.mutation_enabled = %v, want false", rp["mutation_enabled"])
	}

	preview, _ := rp["entries_preview"].([]any)
	if len(preview) != 3 {
		t.Errorf("entries_preview len = %d, want 3", len(preview))
	}

	notes, _ := rp["notes"].(string)
	if !strings.Contains(notes, "R1 read-shadow") {
		t.Errorf("repair_replay_plan.notes missing R1 marker: %q", notes)
	}
}

func TestRepairReplayEmptyEntries(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-repair-empty"}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
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

	rp, ok := resp["repair_replay_plan"].(map[string]any)
	if !ok {
		t.Fatalf("repair_replay_plan is not an object")
	}
	if rp["entries_count"] != float64(0) {
		t.Errorf("repair_replay_plan.entries_count = %v, want 0", rp["entries_count"])
	}
	if rp["dry_run"] != false {
		t.Errorf("repair_replay_plan.dry_run = %v, want false", rp["dry_run"])
	}
}

func TestRepairReplayWriteStoreDryRunAndReplay(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-repair-write", TurnIndex: 1, Role: "user", Content: "already saved"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-repair-write","dry_run":true,"entries":[{"turn_index":1,"user_content":"already saved","assistant_content":"missing assistant"},{"turn_index":2,"user_content":"new user","assistant_content":"new assistant"}]}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var dryResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dryResp); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if dryResp["total_missing_role_count"] != float64(3) || dryResp["total_repaired_role_count"] != float64(0) {
		t.Fatalf("dry-run counts mismatch: %+v", dryResp)
	}
	if len(fake.savedChatLogs) != 0 {
		t.Fatalf("dry-run should not save chat logs, got %d", len(fake.savedChatLogs))
	}

	body = strings.Replace(body, `"dry_run":true`, `"dry_run":false`, 1)
	req = httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("replay status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var replayResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &replayResp); err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	if replayResp["total_repaired_role_count"] != float64(3) {
		t.Fatalf("total_repaired_role_count = %v, want 3", replayResp["total_repaired_role_count"])
	}
	if len(fake.savedChatLogs) != 3 {
		t.Fatalf("savedChatLogs = %d, want 3", len(fake.savedChatLogs))
	}
	if fake.savedChatLogs[0].Role != "assistant" || fake.savedChatLogs[0].TurnIndex != 1 {
		t.Fatalf("first repaired role = %#v, want missing assistant turn 1", fake.savedChatLogs[0])
	}
	foundAudit := false
	for _, audit := range fake.savedAuditLogs {
		if audit.EventType == "repair_replay" {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected repair_replay audit, got %#v", fake.savedAuditLogs)
	}
}

func TestAdminRescanRegeneratesMissingArtifactsFromRawTurn(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-rescan", TurnIndex: 1, Role: "user", Content: "Mina found the blue key.", CreatedAt: time.Now()},
			{ChatSessionID: "sess-rescan", TurnIndex: 1, Role: "assistant", Content: "Mina promised to keep the blue key safe.", CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Mina found and kept the blue key safe.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Mina promised to keep the blue key safe."},
		"kg_triples":        []any{map[string]any{"subject": "Mina", "predicate": "keeps", "object": "blue key"}},
		"entities":          map[string]any{"characters": []any{map[string]any{"name": "Mina"}}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "rescan-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	updateBody := `{"criticApiKey":"sk-rescan","criticEndpoint":"https://api.example.com/v1","criticModel":"rescan-critic","criticProvider":"openai","criticTimeout":45}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", bytes.NewReader([]byte(`{"chat_session_id":"sess-rescan","max_items":10}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode rescan: %v", err)
	}
	if resp["succeeded"] != float64(1) || resp["failed"] != float64(0) {
		t.Fatalf("rescan counts mismatch: %+v", resp)
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 || len(fake.savedEntities) != 1 {
		t.Fatalf("expected memory/evidence/KG/entity from rescan, memories=%d evidence=%d kg=%d entities=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples), len(fake.savedEntities))
	}
	if !hasAuditEvent(fake.savedAuditLogs, "admin_rescan") {
		t.Fatalf("expected admin_rescan audit, got %#v", fake.savedAuditLogs)
	}
}

func TestAdminRescanIncludesTurnZeroAndTrustsCanonicalPlanBlocks(t *testing.T) {
	starter := "The rain had not stopped when Mina reached the old gate.\n\n# Narrative Guide\nScene Mandate: preserve the language barrier.\nForbidden Moves:\n- instant mutual understanding"
	if !looksLikeSourceControlResidue(starter) {
		t.Fatal("test fixture must exercise the legacy source-aware content heuristic")
	}
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-starter-rescan", TurnIndex: 0, Role: "assistant", Content: starter, CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":     "Mina reaches the old gate while the language barrier remains active.",
		"importance_score": 8,
		"world_rules": []any{
			map[string]any{"scope": "session", "scope_name": "Communication", "category": "setting", "key": "language_barrier", "value": "Mina and the locals cannot yet understand each other."},
		},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "rescan-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	updateBody := `{"criticApiKey":"sk-rescan","criticEndpoint":"https://api.example.com/v1","criticModel":"rescan-critic","criticProvider":"openai","criticTimeout":45}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", bytes.NewReader([]byte(`{"chat_session_id":"sess-starter-rescan","max_items":10,"client_meta":{"full_session_backfill":true}}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode rescan: %v", err)
	}
	if resp["candidate_count"] != float64(1) || resp["succeeded"] != float64(1) {
		t.Fatalf("turn-zero rescan counts mismatch: %+v", resp)
	}
	processed, _ := resp["processed_turns"].([]any)
	if len(processed) != 1 || processed[0] != float64(0) {
		t.Fatalf("processed_turns = %#v, want [0]", resp["processed_turns"])
	}
	if len(fake.savedMemories) != 1 || fake.savedMemories[0].TurnIndex != 0 {
		t.Fatalf("starter memory was not generated at turn 0: %#v", fake.savedMemories)
	}
}

func TestRepairReplayAcceptsAssistantStarterAtTurnZero(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-starter-repair","entries":[{"turn_index":0,"assistant_content":"The story opens at the rain-soaked gate."}]}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("repair replay status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode repair replay: %v", err)
	}
	if resp["total_repaired_role_count"] != float64(1) {
		t.Fatalf("turn-zero repair result mismatch: %+v", resp)
	}
	if len(fake.savedChatLogs) != 1 || fake.savedChatLogs[0].TurnIndex != 0 || fake.savedChatLogs[0].Role != "assistant" {
		t.Fatalf("starter chat log was not repaired at turn 0: %#v", fake.savedChatLogs)
	}
}

func TestAdminRescanNoCandidatesHonest(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-rescan-empty", TurnIndex: 1, Role: "user", Content: "already indexed", CreatedAt: time.Now()},
		},
		returnMemories: []store.Memory{
			{ChatSessionID: "sess-rescan-empty", TurnIndex: 1, SummaryJSON: `{"turn_summary":"already indexed"}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", bytes.NewReader([]byte(`{"chat_session_id":"sess-rescan-empty","max_items":10}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode rescan: %v", err)
	}
	if resp["candidate_count"] != float64(0) || resp["succeeded"] != float64(0) || resp["failed"] != float64(0) {
		t.Fatalf("expected honest no-candidates response, got %+v", resp)
	}
	if !strings.Contains(fmt.Sprint(resp["note"]), "no raw chat_log turns missing memory") {
		t.Fatalf("unexpected note: %v", resp["note"])
	}
	if !hasAuditEvent(fake.savedAuditLogs, "admin_rescan") {
		t.Fatalf("expected admin_rescan audit for no-candidates path, got %#v", fake.savedAuditLogs)
	}
}

func TestAdminReindexWritesAuditWithoutClaimingExecution(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", bytes.NewReader([]byte(`{"chat_session_id":"sess-reindex","dry_run":true}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("reindex status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode reindex: %v", err)
	}
	if resp["audit_written"] != true {
		t.Fatalf("audit_written = %v, want true: %#v", resp["audit_written"], resp)
	}
	if resp["reindex_executed"] != false {
		t.Fatalf("reindex_executed = %v, want false", resp["reindex_executed"])
	}
	if !hasAuditEvent(fake.savedAuditLogs, "admin_reindex") {
		t.Fatalf("expected admin_reindex audit, got %#v", fake.savedAuditLogs)
	}
}

func hasAuditEvent(items []*store.AuditLog, eventType string) bool {
	for _, item := range items {
		if item != nil && item.EventType == eventType {
			return true
		}
	}
	return false
}

func TestSeq123P104SaveUpdateDeleteSyncReplayGateMarkers(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-p104","dry_run":true,"entries":[{"turn_index":4,"user_content":"u","assistant_content":"a"}]}`
	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("repair-replay status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var replayResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &replayResp); err != nil {
		t.Fatalf("decode repair-replay: %v", err)
	}
	replayPlan, ok := replayResp["repair_replay_plan"].(map[string]any)
	if !ok {
		t.Fatalf("repair_replay_plan missing: %#v", replayResp)
	}
	if replayPlan["sync_replay_gate"] != true || replayPlan["save_update_delete_gate"] != true {
		t.Fatalf("repair replay gate markers missing: %#v", replayPlan)
	}
	if replayPlan["mutation_enabled"] != false || replayPlan["would_replay"] != false || replayPlan["would_write"] != false {
		t.Fatalf("shadow repair replay should not mutate: %#v", replayPlan)
	}
	if replayPlan["write_scope"] != "chat_log_effective_input_memory_evidence_kg" {
		t.Fatalf("write_scope = %v", replayPlan["write_scope"])
	}
	if replayPlan["delete_scope"] != "rollback_delete_gate_only" || replayPlan["canonical_input_source"] != "sqlite_store" {
		t.Fatalf("delete/canonical gate mismatch: %#v", replayPlan)
	}

	req = httptest.NewRequest(http.MethodDelete, "/rollback/4?chat_session_id=sess-p104&req_source=validation_gate", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var rollbackResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rollbackResp); err != nil {
		t.Fatalf("decode rollback: %v", err)
	}
	rollbackPlan, ok := rollbackResp["rollback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("rollback_plan missing: %#v", rollbackResp)
	}
	if rollbackPlan["save_update_delete_gate"] != true || rollbackPlan["would_delete"] != false || rollbackPlan["mutation_enabled"] != false {
		t.Fatalf("rollback shadow gate mismatch: %#v", rollbackPlan)
	}
}

func TestSeq123P105StaleVectorRollbackRebuildReplayGateMarkers(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/release-hygiene", strings.NewReader(`{"chat_session_id":"sess-p105"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("release-hygiene status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var hygieneResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &hygieneResp); err != nil {
		t.Fatalf("decode release-hygiene: %v", err)
	}
	evidence, ok := hygieneResp["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("release hygiene evidence missing: %#v", hygieneResp)
	}
	if evidence["stale_vector_policy"] != "tombstone_before_delete" || evidence["delete_policy"] != "canonical_row_first" || evidence["rollback_policy"] != "vector_doc_rollback_with_id" {
		t.Fatalf("stale vector policy mismatch: %#v", evidence)
	}

	req = httptest.NewRequest(http.MethodDelete, "/rollback/8?chat_session_id=sess-p105&req_source=validation_gate", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var rollbackResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rollbackResp); err != nil {
		t.Fatalf("decode rollback: %v", err)
	}
	rollbackPlan := rollbackResp["rollback_plan"].(map[string]any)
	if rollbackPlan["stale_vector_replay_gate"] != true || rollbackPlan["rollback_vector_delete_gate"] != true {
		t.Fatalf("rollback stale vector gate mismatch: %#v", rollbackPlan)
	}
	if rollbackPlan["vector_doc_delete_policy"] != "canonical_row_first_then_vector" || rollbackPlan["rebuild_owner"] != "chroma_shadow_orchestrator" {
		t.Fatalf("rollback vector/rebuild markers mismatch: %#v", rollbackPlan)
	}

	req = httptest.NewRequest(http.MethodPost, "/chroma-shadow/rebuild-drill", strings.NewReader(`{"chat_session_id":"sess-p105"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("rebuild-drill status = %d, want 503", rec.Code)
	}
	var rebuildResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rebuildResp); err != nil {
		t.Fatalf("decode rebuild-drill: %v", err)
	}
	trace := rebuildResp["trace_summary"].(map[string]any)
	if trace["rebuild_owner"] != "chroma_shadow_orchestrator" {
		t.Fatalf("rebuild_owner = %v", trace["rebuild_owner"])
	}
}

func TestSeq123P106FailOpenSQLiteBaselineReplayGateMarkers(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/fallback-runbook", strings.NewReader(`{"chat_session_id":"sess-p106"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fallback-runbook status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode fallback-runbook: %v", err)
	}
	evidence := resp["evidence"].(map[string]any)
	if evidence["fallback_policy"] != "store_first_then_vector" || evidence["fail_open_baseline"] != true {
		t.Fatalf("fallback fail-open markers mismatch: %#v", evidence)
	}
	if evidence["retrieval_baseline"] != "sqlite_canonical" || evidence["canonical_baseline_source"] != "sqlite_store" || evidence["sqlite_canonical_baseline"] != true {
		t.Fatalf("sqlite canonical baseline markers mismatch: %#v", evidence)
	}
}

func TestSeq123P107Future125OwnerDecisionChecklistMarkers(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/adoption-gate", strings.NewReader(`{"chat_session_id":"sess-p107"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("adoption-gate status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode adoption-gate: %v", err)
	}
	if resp["live_cutover_allowed"] != false || resp["adoption_gate_state"] != "closed" {
		t.Fatalf("adoption gate must stay closed: %#v", resp)
	}
	if resp["owner_decision_state"] != "pending_pre_12_5" || resp["scope_truth_authority"] != "store_canonical_truth" {
		t.Fatalf("owner decision top-level markers mismatch: %#v", resp)
	}
	decision, ok := resp["future_125_owner_decision"].(map[string]any)
	if !ok {
		t.Fatalf("future_125_owner_decision missing: %#v", resp)
	}
	if decision["long_memory_input_quality"] != "requires_replay_green" || decision["scope_truth_authority"] != "store_canonical_truth" {
		t.Fatalf("future 12.5 decision mismatch: %#v", decision)
	}
}

func TestSeq123P111SQLiteCanonicalInputDisciplineChecklistPass(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/turns/repair-replay", bytes.NewReader([]byte(`{"chat_session_id":"sess-p111","entries":[{"turn_index":1,"user_content":"u"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("repair-replay status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var replayResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &replayResp); err != nil {
		t.Fatalf("decode repair-replay: %v", err)
	}
	replayPlan := replayResp["repair_replay_plan"].(map[string]any)
	if replayPlan["canonical_input_source"] != "sqlite_store" || replayPlan["sync_replay_gate"] != true {
		t.Fatalf("canonical input discipline markers mismatch: %#v", replayPlan)
	}
}

func TestSeq123P112ChromaSyncStaleGuardComplete(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/release-hygiene", strings.NewReader(`{"chat_session_id":"sess-p112"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("release-hygiene status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode release-hygiene: %v", err)
	}
	evidence := resp["evidence"].(map[string]any)
	if evidence["stale_vector_policy"] != "tombstone_before_delete" || evidence["merge_policy"] != "merge_stale_vectors_to_tombstone" {
		t.Fatalf("stale guard markers mismatch: %#v", evidence)
	}
}

func TestSeq123P113FailOpenDriftRebuildVocabularyCleanupComplete(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Vector = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/fallback-runbook", strings.NewReader(`{"chat_session_id":"sess-p113"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fallback-runbook status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var fallbackResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &fallbackResp); err != nil {
		t.Fatalf("decode fallback-runbook: %v", err)
	}
	fallbackEvidence := fallbackResp["evidence"].(map[string]any)
	if fallbackEvidence["degraded_mode"] != "canonical_baseline" || fallbackEvidence["retrieval_baseline"] != "sqlite_canonical" {
		t.Fatalf("fallback vocabulary mismatch: %#v", fallbackEvidence)
	}

	req = httptest.NewRequest(http.MethodPost, "/chroma-shadow/visibility-guard", strings.NewReader(`{"chat_session_id":"sess-p113"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("visibility-guard status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var visibilityResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &visibilityResp); err != nil {
		t.Fatalf("decode visibility-guard: %v", err)
	}
	visibilityEvidence := visibilityResp["evidence"].(map[string]any)
	if visibilityEvidence["drift_policy"] != "shadow_degraded" || visibilityEvidence["drift_action"] != "keep_canonical_baseline" {
		t.Fatalf("drift vocabulary mismatch: %#v", visibilityEvidence)
	}
}

func TestSeq123P114Future125ScopeTruthAuthorityLongMemoryInputQualityExtend(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/adoption-gate", strings.NewReader(`{"chat_session_id":"sess-p114"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("adoption-gate status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode adoption-gate: %v", err)
	}
	decision := resp["future_125_owner_decision"].(map[string]any)
	if decision["scope_truth_authority"] != "store_canonical_truth" || decision["long_memory_input_quality"] != "requires_replay_green" {
		t.Fatalf("scope/input-quality extension mismatch: %#v", decision)
	}
	gates, ok := decision["required_green_gates"].([]any)
	if !ok || len(gates) < 3 {
		t.Fatalf("required_green_gates missing: %#v", decision)
	}
}

func TestRollbackShadowPlan(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/5?chat_session_id=sess-rollback&req_source=adapter", nil)
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
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["turn_index"] != float64(5) {
		t.Errorf("turn_index = %v, want 5", resp["turn_index"])
	}

	rb, ok := resp["rollback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("rollback_plan is not an object")
	}
	if rb["status"] != "shadow_plan" {
		t.Errorf("rollback_plan.status = %v, want shadow_plan", rb["status"])
	}
	if rb["turn_index"] != float64(5) {
		t.Errorf("rollback_plan.turn_index = %v, want 5", rb["turn_index"])
	}
	if rb["chat_session_id"] != "sess-rollback" {
		t.Errorf("rollback_plan.chat_session_id = %v, want sess-rollback", rb["chat_session_id"])
	}
	if rb["req_source"] != "adapter" {
		t.Errorf("rollback_plan.req_source = %v, want adapter", rb["req_source"])
	}
	if rb["would_delete"] != false {
		t.Errorf("rollback_plan.would_delete = %v, want false", rb["would_delete"])
	}
	if rb["would_write"] != false {
		t.Errorf("rollback_plan.would_write = %v, want false", rb["would_write"])
	}
	if rb["mutation_enabled"] != false {
		t.Errorf("rollback_plan.mutation_enabled = %v, want false", rb["mutation_enabled"])
	}
	if rb["reason"] != "R1 shadow mode: rollback not executed" {
		t.Errorf("rollback_plan.reason = %v, want R1 shadow mode: rollback not executed", rb["reason"])
	}
}

func TestRollbackInvalidTurnIndex(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/abc?chat_session_id=sess-bad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRollbackNegativeTurnIndex(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/rollback/-1?chat_session_id=sess-neg", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// rollbackRecordingStore wraps a Store and records RollbackStore calls.
type rollbackRecordingStore struct {
	store.Store
	deletes   []string
	deleteErr error
	audits    []*store.AuditLog
}

func (r *rollbackRecordingStore) DeleteChatLogs(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("chat_logs:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteEffectiveInputs(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("effective_inputs:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteMemories(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("memories:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteEvidence(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("direct_evidence:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteKGTriples(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("kg_triples:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteCriticFeedback(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("critic_feedback:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteCharacterEvents(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("character_events:%s:%d", sid, fromTurn))
	return nil
}

func (r *rollbackRecordingStore) DeleteEntities(ctx context.Context, sid string, fromTurn int) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes = append(r.deletes, fmt.Sprintf("entities:%s:%d", sid, fromTurn))
	return nil
}
