package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestForcedNoopEnvClearsDBAndVectorSettings(t *testing.T) {
	env := forcedNoopEnv([]string{
		"PATH=/bin",
		"AC_MODE=live",
		"AC_STORE_MODE=mariadb_read_shadow",
		"AC_MARIADB_DSN=user:secret@tcp(localhost:3306)/db",
		"AC_CHROMA_ENDPOINT=http://127.0.0.1:8000",
		"AC_MILVUS_ENDPOINT=localhost:19530",
		"AC_MILVUS_SDK_ENABLED=true",
	}, 28220)

	joined := strings.Join(env, "\n")
	for _, forbidden := range []string{"secret", "AC_MARIADB_DSN=", "AC_CHROMA_ENDPOINT=", "AC_MILVUS_ENDPOINT=", "AC_MILVUS_SDK_ENABLED=true", "AC_MODE=live", "AC_STORE_MODE=mariadb"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("forced env leaked forbidden value %q in %q", forbidden, joined)
		}
	}
	for _, required := range []string{"AC_BIND_ADDR=127.0.0.1:28220", "AC_MODE=shadow", "AC_STORE_MODE=noop"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("forced env missing %q in %q", required, joined)
		}
	}
}

func TestBuildCommandUsesBinaryWhenProvided(t *testing.T) {
	cmd, mode, display, err := buildCommand(context.Background(), launchSpec{BinPath: "/tmp/archive-center-go", Port: 28222})
	if err != nil {
		t.Fatalf("buildCommand returned error: %v", err)
	}
	if mode != "binary" {
		t.Fatalf("mode = %q, want binary", mode)
	}
	if len(display) != 1 || display[0] != "/tmp/archive-center-go" {
		t.Fatalf("display = %#v", display)
	}
	if cmd.Path != "/tmp/archive-center-go" {
		t.Fatalf("cmd.Path = %q, want binary path", cmd.Path)
	}
}

func TestProbeRequiresJSONAndSuccessStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/bad-json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not-json"))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "down"})
		}
	}))
	defer ts.Close()

	client := ts.Client()
	ok := probe(client, ts.URL, "/health")
	if ok.Status != "ok" || ok.HTTPStatus != http.StatusOK || !ok.JSONValid {
		t.Fatalf("ok probe = %#v", ok)
	}
	badJSON := probe(client, ts.URL, "/bad-json")
	if badJSON.Status != "failed" || badJSON.Error != "invalid_json" {
		t.Fatalf("bad json probe = %#v", badJSON)
	}
	down := probe(client, ts.URL, "/down")
	if down.Status != "failed" || down.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("down probe = %#v", down)
	}
}
