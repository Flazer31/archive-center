package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func TestRuntimeConfigPublisherOffRetainsRoleReadinessAndCritic(t *testing.T) {
	s := NewServer(config.Default())
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	for _, mode := range []string{"off", "shadow", "reviewed_apply", "", "off"} {
		body := map[string]any{
			"mainProvider": "openai", "mainApiKey": "fixture-unused-key", "mainModel": "", "mainTimeout": 120,
			"supervisorProvider": "openai", "supervisorApiKey": "fixture-unused-key", "supervisorModel": "", "supervisorTimeout": 120,
			"criticProvider": "vertex", "criticApiKey": "fixture-critic-key", "criticEndpoint": "https://aiplatform.googleapis.com/v1", "criticModel": "fixture-gemini-model", "criticTimeout": 120,
		}
		if mode != "" {
			body["publisherApplyMode"] = mode
		}
		encoded, _ := json.Marshal(body)
		r := httptest.NewRecorder()
		mux.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader(encoded)))
		if r.Code != http.StatusOK {
			t.Fatal(r.Code, r.Body)
		}
		var result map[string]any
		if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		trace := result["runtime_config_trace"].(map[string]any)
		if result["status"] != "ok" || trace["synced"] != true || trace["critic"].(map[string]any)["configured"] != true {
			t.Fatal(result)
		}
		for _, role := range []string{"main", "supervisor"} {
			fields := trace[role].(map[string]any)
			if fields["configured"] != false || !reflect.DeepEqual(fields["missing_fields"], []any{"model"}) || fields["required_for_sync"] != (mode != "off") {
				t.Fatalf("mode=%q role=%s trace=%v", mode, role, fields)
			}
		}
		runtime := s.runtimeConfigSnapshot()
		if runtime.MainAPIKey != "fixture-unused-key" || runtime.SupervisorAPIKey != "fixture-unused-key" || runtime.CriticModel != "fixture-gemini-model" {
			t.Fatal("stored connection fields changed")
		}
		if s.chapterLLMConfig().hasConfig() {
			t.Fatal("disabled Publisher was made configured for other consumers")
		}
	}
}
