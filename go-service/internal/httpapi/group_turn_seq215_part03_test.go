package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq215P607ValidationRecord(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p607","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_validation_record")
	if s["version"] != "s215-p607.v1" {
		t.Fatalf("version=%v, want s215-p607.v1", s["version"])
	}
	if s["record_type"] != "validation_output_and_changed_files" {
		t.Fatalf("record_type=%v, want validation_output_and_changed_files", s["record_type"])
	}
	if s["mode"] != "seq215_validation_record_definition" {
		t.Fatalf("mode=%v, want seq215_validation_record_definition", s["mode"])
	}
}

func TestSeq215P663Phase1ValidationPassed(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p663","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_phase1_validation_passed")
	if s["version"] != "s215-p663.v1" {
		t.Fatalf("version=%v, want s215-p663.v1", s["version"])
	}
	if s["validation_status"] != "passed" {
		t.Fatalf("validation_status=%v, want passed", s["validation_status"])
	}
	if s["mode"] != "seq215_phase1_validation_passed_definition" {
		t.Fatalf("mode=%v, want seq215_phase1_validation_passed_definition", s["mode"])
	}
}

func TestSeq215P664PrepareTurnAssemblyCreated(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p664","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_prepare_turn_assembly_created")
	if s["version"] != "s215-p664.v1" {
		t.Fatalf("version=%v, want s215-p664.v1", s["version"])
	}
	if s["file_created"] != "backend/prepare_turn_assembly.py" {
		t.Fatalf("file_created=%v, want backend/prepare_turn_assembly.py", s["file_created"])
	}
	if s["mode"] != "seq215_prepare_turn_assembly_created_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_assembly_created_definition", s["mode"])
	}
}

func TestSeq215P665FormatMemoryTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p665","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_format_memory_text_moved")
	if s["version"] != "s215-p665.v1" {
		t.Fatalf("version=%v, want s215-p665.v1", s["version"])
	}
	if s["function_name"] != "_format_memory_text_m3a" {
		t.Fatalf("function_name=%v, want _format_memory_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_memory_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_memory_text_moved_definition", s["mode"])
	}
}

func TestSeq215P666FormatKGTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p666","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_format_kg_text_moved")
	if s["version"] != "s215-p666.v1" {
		t.Fatalf("version=%v, want s215-p666.v1", s["version"])
	}
	if s["function_name"] != "_format_kg_text_m3a" {
		t.Fatalf("function_name=%v, want _format_kg_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_kg_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_kg_text_moved_definition", s["mode"])
	}
}

func TestSeq215P667FormatEpisodeTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p667","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_format_episode_text_moved")
	if s["version"] != "s215-p667.v1" {
		t.Fatalf("version=%v, want s215-p667.v1", s["version"])
	}
	if s["function_name"] != "_format_episode_text_m3a" {
		t.Fatalf("function_name=%v, want _format_episode_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_episode_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_episode_text_moved_definition", s["mode"])
	}
}

func TestSeq215P668FormatChapterTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p668","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_format_chapter_text_moved")
	if s["version"] != "s215-p668.v1" {
		t.Fatalf("version=%v, want s215-p668.v1", s["version"])
	}
	if s["function_name"] != "_format_chapter_text_m3a" {
		t.Fatalf("function_name=%v, want _format_chapter_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_chapter_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_chapter_text_moved_definition", s["mode"])
	}
}

func TestSeq215P669FormatFallbackTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p669","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_format_fallback_text_moved")
	if s["version"] != "s215-p669.v1" {
		t.Fatalf("version=%v, want s215-p669.v1", s["version"])
	}
	if s["function_name"] != "_format_fallback_text_m3a" {
		t.Fatalf("function_name=%v, want _format_fallback_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_fallback_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_fallback_text_moved_definition", s["mode"])
	}
}

func TestSeq215P670CleanShortMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p670","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_clean_short_moved")
	if s["version"] != "s215-p670.v1" {
		t.Fatalf("version=%v, want s215-p670.v1", s["version"])
	}
	if s["function_name"] != "_clean_short_m3a" {
		t.Fatalf("function_name=%v, want _clean_short_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_clean_short_moved_definition" {
		t.Fatalf("mode=%v, want seq215_clean_short_moved_definition", s["mode"])
	}
}

func TestSeq215P671JsonLoadMaybeMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p671","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_json_load_maybe_moved")
	if s["version"] != "s215-p671.v1" {
		t.Fatalf("version=%v, want s215-p671.v1", s["version"])
	}
	if s["function_name"] != "_json_load_maybe_m3a" {
		t.Fatalf("function_name=%v, want _json_load_maybe_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_json_load_maybe_moved_definition" {
		t.Fatalf("mode=%v, want seq215_json_load_maybe_moved_definition", s["mode"])
	}
}

func TestSeq215P672PredicateMatchesMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p672","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_predicate_matches_moved")
	if s["version"] != "s215-p672.v1" {
		t.Fatalf("version=%v, want s215-p672.v1", s["version"])
	}
	if s["function_name"] != "_predicate_matches_m3a" {
		t.Fatalf("function_name=%v, want _predicate_matches_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_predicate_matches_moved_definition" {
		t.Fatalf("mode=%v, want seq215_predicate_matches_moved_definition", s["mode"])
	}
}

func TestSeq215P673WorldRuleNoteMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p673","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_world_rule_note_moved")
	if s["version"] != "s215-p673.v1" {
		t.Fatalf("version=%v, want s215-p673.v1", s["version"])
	}
	if s["function_name"] != "_world_rule_note_m3a" {
		t.Fatalf("function_name=%v, want _world_rule_note_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_world_rule_note_moved_definition" {
		t.Fatalf("mode=%v, want seq215_world_rule_note_moved_definition", s["mode"])
	}
}

func TestSeq215P674FormatEntityDigestTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p674","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_format_entity_digest_text_moved")
	if s["version"] != "s215-p674.v1" {
		t.Fatalf("version=%v, want s215-p674.v1", s["version"])
	}
	if s["function_name"] != "_format_entity_digest_text_m3a" {
		t.Fatalf("function_name=%v, want _format_entity_digest_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_entity_digest_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_entity_digest_text_moved_definition", s["mode"])
	}
}

func TestSeq215P675FormatEntityAnchorTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p675","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_format_entity_anchor_text_moved")
	if s["version"] != "s215-p675.v1" {
		t.Fatalf("version=%v, want s215-p675.v1", s["version"])
	}
	if s["function_name"] != "_format_entity_anchor_text_m3a" {
		t.Fatalf("function_name=%v, want _format_entity_anchor_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_entity_anchor_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_entity_anchor_text_moved_definition", s["mode"])
	}
}

func TestSeq215P677DBSessionHelpersStay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p677","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_db_session_helpers_stay")
	if s["version"] != "s215-p677.v1" {
		t.Fatalf("version=%v, want s215-p677.v1", s["version"])
	}
	if s["stayed_in"] != "main.py" {
		t.Fatalf("stayed_in=%v, want main.py", s["stayed_in"])
	}
	if s["mode"] != "seq215_db_session_helpers_stay_definition" {
		t.Fatalf("mode=%v, want seq215_db_session_helpers_stay_definition", s["mode"])
	}
}

func TestSeq215P678CoreLogicStay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p678","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_core_logic_stay")
	if s["version"] != "s215-p678.v1" {
		t.Fatalf("version=%v, want s215-p678.v1", s["version"])
	}
	if s["stayed_in"] != "main.py" {
		t.Fatalf("stayed_in=%v, want main.py", s["stayed_in"])
	}
	if s["mode"] != "seq215_core_logic_stay_definition" {
		t.Fatalf("mode=%v, want seq215_core_logic_stay_definition", s["mode"])
	}
}

func TestSeq215P679InjectionPackFieldsUnchanged(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p679","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_injection_pack_fields_unchanged")
	if s["version"] != "s215-p679.v1" {
		t.Fatalf("version=%v, want s215-p679.v1", s["version"])
	}
	if s["fields_unchanged"] != true {
		t.Fatalf("fields_unchanged=%v, want true", s["fields_unchanged"])
	}
	fields := seq165Slice(t, s, "field_names")
	for _, field := range []string{"memory_text", "kg_text", "fallback_text", "episode_text"} {
		if !sliceContains(fields, field) {
			t.Fatalf("field_names missing %q: %v", field, fields)
		}
	}
	if s["backend_pack_status"] != "advisory_not_authoritative" {
		t.Fatalf("backend_pack_status=%v, want advisory_not_authoritative", s["backend_pack_status"])
	}
	if s["mode"] != "seq215_injection_pack_fields_unchanged_definition" {
		t.Fatalf("mode=%v, want seq215_injection_pack_fields_unchanged_definition", s["mode"])
	}
}

func TestSeq215P680PyCompilePrepareTurnAssembly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p680","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_py_compile_prepare_turn_assembly")
	if s["version"] != "s215-p680.v1" {
		t.Fatalf("version=%v, want s215-p680.v1", s["version"])
	}
	if s["compile_status"] != "pass" {
		t.Fatalf("compile_status=%v, want pass", s["compile_status"])
	}
	if s["mode"] != "seq215_py_compile_prepare_turn_assembly_definition" {
		t.Fatalf("mode=%v, want seq215_py_compile_prepare_turn_assembly_definition", s["mode"])
	}
}

func TestSeq215P681FocusedBackendTestsM3a(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p681","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_focused_backend_tests_m3a")
	if s["version"] != "s215-p681.v1" {
		t.Fatalf("version=%v, want s215-p681.v1", s["version"])
	}
	if s["focused_test_file"] != "backend/test_step21_5_prepare_turn_assembly.py" {
		t.Fatalf("focused_test_file=%v, want backend/test_step21_5_prepare_turn_assembly.py", s["focused_test_file"])
	}
	if s["tests_passed"] != float64(5) {
		t.Fatalf("tests_passed=%v, want 5", s["tests_passed"])
	}
	if s["scoped_regression_tests_passed"] != float64(25) {
		t.Fatalf("scoped_regression_tests_passed=%v, want 25", s["scoped_regression_tests_passed"])
	}
	if s["backend_tests_passed"] != float64(212) {
		t.Fatalf("backend_tests_passed=%v, want 212", s["backend_tests_passed"])
	}
	coverage := seq165Slice(t, s, "test_coverage")
	for _, marker := range []string{"exact_kg_delimiter_preservation", "main_py_import_bridge", "bundle_injection_assembly_consumes_extracted_formatters"} {
		if !sliceContains(coverage, marker) {
			t.Fatalf("test_coverage missing %q: %v", marker, coverage)
		}
	}
	if s["mode"] != "seq215_focused_backend_tests_m3a_definition" {
		t.Fatalf("mode=%v, want seq215_focused_backend_tests_m3a_definition", s["mode"])
	}
}

func TestSeq215P682M3aValidationRecord(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p682","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_m3a_validation_record")
	if s["version"] != "s215-p682.v1" {
		t.Fatalf("version=%v, want s215-p682.v1", s["version"])
	}
	if s["record_type"] != "validation_output_and_changed_files" {
		t.Fatalf("record_type=%v, want validation_output_and_changed_files", s["record_type"])
	}
	if s["mode"] != "seq215_m3a_validation_record_definition" {
		t.Fatalf("mode=%v, want seq215_m3a_validation_record_definition", s["mode"])
	}
}

func TestSeq215P730ProxyPluginMainModelSeparated(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p730","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_proxy_plugin_main_model_separated")
	if s["version"] != "s215-p730.v1" {
		t.Fatalf("version=%v, want s215-p730.v1", s["version"])
	}
	if s["route"] != "/proxy/plugin-main" {
		t.Fatalf("route=%v, want /proxy/plugin-main", s["route"])
	}
	if s["monolith_separated"] != true {
		t.Fatalf("monolith_separated=%v, want true", s["monolith_separated"])
	}
	if s["mode"] != "seq215_proxy_plugin_main_model_separated_definition" {
		t.Fatalf("mode=%v, want seq215_proxy_plugin_main_model_separated_definition", s["mode"])
	}
}

func TestSeq215P731ProviderOwnershipSplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p731","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_provider_ownership_split")
	if s["version"] != "s215-p731.v1" {
		t.Fatalf("version=%v, want s215-p731.v1", s["version"])
	}
	providers := seq165Slice(t, s, "providers")
	for _, p := range []string{"openai", "claude", "gemini", "vertex", "copilot"} {
		if !sliceContains(providers, p) {
			t.Fatalf("providers missing %q: %v", p, providers)
		}
	}
	if s["mode"] != "seq215_provider_ownership_split_definition" {
		t.Fatalf("mode=%v, want seq215_provider_ownership_split_definition", s["mode"])
	}
}

func TestSeq215P732ThinProxyRoute(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p732","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_thin_proxy_route")
	if s["version"] != "s215-p732.v1" {
		t.Fatalf("version=%v, want s215-p732.v1", s["version"])
	}
	if s["route"] != "/proxy/plugin-main" {
		t.Fatalf("route=%v, want /proxy/plugin-main", s["route"])
	}
	if s["delegate"] != "performProxyPluginMain" {
		t.Fatalf("delegate=%v, want performProxyPluginMain", s["delegate"])
	}
	if s["route_type"] != "thin_delegating" {
		t.Fatalf("route_type=%v, want thin_delegating", s["route_type"])
	}
	if s["mode"] != "seq215_thin_proxy_route_definition" {
		t.Fatalf("mode=%v, want seq215_thin_proxy_route_definition", s["mode"])
	}
}

func TestSeq215P733ConfigServiceSplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p733","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_config_service_split")
	if s["version"] != "s215-p733.v1" {
		t.Fatalf("version=%v, want s215-p733.v1", s["version"])
	}
	features := seq165Slice(t, s, "implemented_features")
	for _, f := range []string{"key_mapping", "type_normalization", "runtime_config_trace", "secret_response_masking"} {
		if !sliceContains(features, f) {
			t.Fatalf("implemented_features missing %q: %v", f, features)
		}
	}
	deferred := seq165Slice(t, s, "deferred_features")
	for _, f := range []string{"env_file_persistence", "encrypted_api_key_persistence"} {
		if !sliceContains(deferred, f) {
			t.Fatalf("deferred_features missing %q: %v", f, deferred)
		}
	}
	if s["persistence_mode"] != "runtime_only" {
		t.Fatalf("persistence_mode=%v, want runtime_only", s["persistence_mode"])
	}
	if s["persisted"] != false {
		t.Fatalf("persisted=%v, want false", s["persisted"])
	}
	if s["env_file_persistence"] != "not_enabled_in_2_0" {
		t.Fatalf("env_file_persistence=%v, want not_enabled_in_2_0", s["env_file_persistence"])
	}
	if s["encrypted_api_key_persistence"] != "not_enabled_in_2_0" {
		t.Fatalf("encrypted_api_key_persistence=%v, want not_enabled_in_2_0", s["encrypted_api_key_persistence"])
	}
	if s["mode"] != "seq215_config_service_split_definition" {
		t.Fatalf("mode=%v, want seq215_config_service_split_definition", s["mode"])
	}
}

func TestSeq215P734ThinConfigRoute(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p734","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_thin_config_route")
	if s["version"] != "s215-p734.v1" {
		t.Fatalf("version=%v, want s215-p734.v1", s["version"])
	}
	if s["route"] != "/config/update" {
		t.Fatalf("route=%v, want /config/update", s["route"])
	}
	if s["route_type"] != "thin_delegating" {
		t.Fatalf("route_type=%v, want thin_delegating", s["route_type"])
	}
	if s["mode"] != "seq215_thin_config_route_definition" {
		t.Fatalf("mode=%v, want seq215_thin_config_route_definition", s["mode"])
	}
}

func TestSeq215P735RoutesExplicitlyWired(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p735","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_routes_explicitly_wired")
	if s["version"] != "s215-p735.v1" {
		t.Fatalf("version=%v, want s215-p735.v1", s["version"])
	}
	routes := seq165Slice(t, s, "routes")
	for _, r := range []string{"/proxy/plugin-main", "/config/update"} {
		if !sliceContains(routes, r) {
			t.Fatalf("routes missing %q: %v", r, routes)
		}
	}
	if s["explicit_binding"] != true {
		t.Fatalf("explicit_binding=%v, want true", s["explicit_binding"])
	}
	if s["mode"] != "seq215_routes_explicitly_wired_definition" {
		t.Fatalf("mode=%v, want seq215_routes_explicitly_wired_definition", s["mode"])
	}
}

func TestSeq215P736PublicPathsPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p736","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_public_paths_preserved")
	if s["version"] != "s215-p736.v1" {
		t.Fatalf("version=%v, want s215-p736.v1", s["version"])
	}
	paths := seq165Slice(t, s, "paths")
	for _, p := range []string{"/proxy/plugin-main", "/config/update"} {
		if !sliceContains(paths, p) {
			t.Fatalf("paths missing %q: %v", p, paths)
		}
	}
	if s["unchanged"] != true {
		t.Fatalf("unchanged=%v, want true", s["unchanged"])
	}
	if s["mode"] != "seq215_public_paths_preserved_definition" {
		t.Fatalf("mode=%v, want seq215_public_paths_preserved_definition", s["mode"])
	}
}

func TestSeq215P737CompatibilityWrapper(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p737","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_compatibility_wrapper")
	if s["version"] != "s215-p737.v1" {
		t.Fatalf("version=%v, want s215-p737.v1", s["version"])
	}
	if s["wrapper_present"] != true {
		t.Fatalf("wrapper_present=%v, want true", s["wrapper_present"])
	}
	if s["backward_compatible"] != true {
		t.Fatalf("backward_compatible=%v, want true", s["backward_compatible"])
	}
	if s["compatibility_mode"] != "stable_route_and_handler_compatibility" {
		t.Fatalf("compatibility_mode=%v, want stable_route_and_handler_compatibility", s["compatibility_mode"])
	}
	if s["python_wrapper_not_applicable"] != true {
		t.Fatalf("python_wrapper_not_applicable=%v, want true", s["python_wrapper_not_applicable"])
	}
	if s["mode"] != "seq215_compatibility_wrapper_definition" {
		t.Fatalf("mode=%v, want seq215_compatibility_wrapper_definition", s["mode"])
	}
}

func TestSeq215P738RouteLevelTests(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p738","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_route_level_tests")
	if s["version"] != "s215-p738.v1" {
		t.Fatalf("version=%v, want s215-p738.v1", s["version"])
	}
	routes := seq165Slice(t, s, "test_routes")
	for _, r := range []string{"/config/update", "/proxy/plugin-main"} {
		if !sliceContains(routes, r) {
			t.Fatalf("test_routes missing %q: %v", r, routes)
		}
	}
	assertions := seq165Slice(t, s, "direct_route_assertions")
	for _, a := range []string{"proxy_missing_endpoint_returns_400_without_upstream", "config_update_masks_secret_and_reports_runtime_only"} {
		if !sliceContains(assertions, a) {
			t.Fatalf("direct_route_assertions missing %q: %v", a, assertions)
		}
	}
	proxyReq := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", strings.NewReader(`{"model":"seq215-p738-no-endpoint"}`))
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyRec := httptest.NewRecorder()
	mux.ServeHTTP(proxyRec, proxyReq)
	if proxyRec.Code != http.StatusBadRequest {
		t.Fatalf("proxy missing endpoint status=%d, want 400: %s", proxyRec.Code, proxyRec.Body.String())
	}
	secret := "sk-seq215-p738-secret"
	configReq := httptest.NewRequest(http.MethodPost, "/config/update", strings.NewReader(`{
		"mainProvider":"openai",
		"mainApiKey":"`+secret+`",
		"mainEndpoint":"https://api.example.com/v1",
		"mainModel":"seq215-p738-model"
	}`))
	configReq.Header.Set("Content-Type", "application/json")
	configRec := httptest.NewRecorder()
	mux.ServeHTTP(configRec, configReq)
	if configRec.Code != http.StatusOK {
		t.Fatalf("config/update status=%d, want 200: %s", configRec.Code, configRec.Body.String())
	}
	if strings.Contains(configRec.Body.String(), secret) {
		t.Fatalf("config/update response leaked secret %q: %s", secret, configRec.Body.String())
	}
	var configResp map[string]any
	if err := json.Unmarshal(configRec.Body.Bytes(), &configResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	if configResp["persistence"] != "runtime_only" {
		t.Fatalf("config persistence=%v, want runtime_only", configResp["persistence"])
	}
	if configResp["persisted"] != false {
		t.Fatalf("config persisted=%v, want false", configResp["persisted"])
	}
	if s["mode"] != "seq215_route_level_tests_definition" {
		t.Fatalf("mode=%v, want seq215_route_level_tests_definition", s["mode"])
	}
}

func TestSeq215P739JSRouteUsage(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p739","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_js_route_usage")
	if s["version"] != "s215-p739.v1" {
		t.Fatalf("version=%v, want s215-p739.v1", s["version"])
	}
	if s["config_route"] != "/config/update" {
		t.Fatalf("config_route=%v, want /config/update", s["config_route"])
	}
	if s["proxy_route"] != "/proxy/plugin-main" {
		t.Fatalf("proxy_route=%v, want /proxy/plugin-main", s["proxy_route"])
	}
	if s["mode"] != "seq215_js_route_usage_definition" {
		t.Fatalf("mode=%v, want seq215_js_route_usage_definition", s["mode"])
	}
}

func TestSeq215P740MonolithNotApplicable(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p740","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_monolith_not_applicable")
	if s["version"] != "s215-p740.v1" {
		t.Fatalf("version=%v, want s215-p740.v1", s["version"])
	}
	if s["go_interpretation"] != "monolith_not_applicable" {
		t.Fatalf("go_interpretation=%v, want monolith_not_applicable", s["go_interpretation"])
	}
	if s["go_route_owner_split"] != true {
		t.Fatalf("go_route_owner_split=%v, want true", s["go_route_owner_split"])
	}
	if s["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", s["beta_reference_mutated"])
	}
	if s["mode"] != "seq215_monolith_not_applicable_definition" {
		t.Fatalf("mode=%v, want seq215_monolith_not_applicable_definition", s["mode"])
	}
}

func TestSeq215P776PrepareTurnBundleNormalUse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p776","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_prepare_turn_bundle_normal_use")
	if s["version"] != "s215-p776.v1" {
		t.Fatalf("version=%v, want s215-p776.v1", s["version"])
	}
	if s["bundle_fields"] != "normal_use_data_sources" {
		t.Fatalf("bundle_fields=%v, want normal_use_data_sources", s["bundle_fields"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"preparedBundle && preparedBundle.continuityPack",
		"preparedBundle && preparedBundle.recallResult",
		"preparedBundle && preparedBundle.injectionPack",
	)
	if s["mode"] != "seq215_prepare_turn_bundle_normal_use_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_bundle_normal_use_definition", s["mode"])
	}
}

func TestSeq215P777JSPayloadMutationOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p777","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq215_js_payload_mutation_owner")
	if s["version"] != "s215-p777.v1" {
		t.Fatalf("version=%v, want s215-p777.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "final_payload_mutation" {
		t.Fatalf("responsibility=%v, want final_payload_mutation", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"const { payload: injectedPayload, injectionResult } = applyContextInjection(outgoingPayload, lastOrchResult);",
		"payload: injectedPayload",
	)
	if s["mode"] != "seq215_js_payload_mutation_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_payload_mutation_owner_definition", s["mode"])
	}
}
