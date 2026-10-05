package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// Real isolated SQL/Chroma and production prepare route; only the embedding
// provider is replaced. This measures processing, not model or RP quality.
func TestPreparationMeasurementMariaDBChroma(t *testing.T) {
	if os.Getenv("AC_MEASURE_PREPARATION") != "1" {
		t.Skip("opt-in isolated measurement")
	}
	turns, err := strconv.Atoi(os.Getenv("AC_MEASURE_TURNS"))
	if err != nil || turns < 1 {
		t.Fatal("set AC_MEASURE_TURNS")
	}
	endpoint := os.Getenv("AC_FEEDBACK_TEST_CHROMA")
	if endpoint == "" {
		t.Fatal("isolated Chroma required")
	}
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	child := os.Getenv("AC_MEASURE_CHILD") == "1"
	var db *sql.DB
	var st archiveStore.Store
	if child {
		db, err = sql.Open("mysql", os.Getenv("AC_MEASURE_SNAPSHOT_DSN"))
		if err != nil {
			t.Fatal(err)
		}
		st, err = archiveStore.OpenMariaDB(os.Getenv("AC_MEASURE_SNAPSHOT_DSN"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close(); st.(interface{ Close() error }).Close() })
	} else {
		db, st = feedback43Database(t)
	}
	ctx := context.Background()
	collection := fmt.Sprintf("preparation_measure_%d", time.Now().UnixNano())
	if child {
		collection = os.Getenv("AC_MEASURE_COLLECTION")
	}
	vec, err := vector.NewChromaStore(endpoint, collection, "/api/v2")
	if err != nil {
		t.Fatal(err)
	}
	if !child {
		t.Cleanup(func() {
			if err := vec.(vector.CollectionResetter).ResetAll(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	const sid = "synthetic-measurement"
	var docs []vector.VectorDocument
	var storedBytes int
	if !child {
		for turn := 1; turn <= turns; turn++ {
			revision := feedback43SeedSource(t, db, sid, turn)
			summary := fmt.Sprintf("Mira checked archive station %d. Rowan kept the brass key.", turn)
			events := []any{}
			for j := 0; j < 20; j++ {
				events = append(events, map[string]any{"event": fmt.Sprintf("Mira recorded archive parcel %d at station %d with Rowan.", j, turn), "visibility": "public"})
			}
			raw, _ := json.Marshal(map[string]any{"turn_summary": summary, "narrative_events": events})
			memory := &archiveStore.Memory{ChatSessionID: sid, TurnIndex: turn, SummaryJSON: string(raw), Importance: 7}
			if err := st.SaveMemory(ctx, memory); err != nil {
				t.Fatal(err)
			}
			storedBytes += len(raw)
			for _, role := range []string{"user", "assistant"} {
				if err := st.SaveChatLog(ctx, &archiveStore.ChatLog{ChatSessionID: sid, TurnIndex: turn, Role: role, Content: summary}); err != nil {
					t.Fatal(err)
				}
			}
			docs = append(docs, vector.VectorDocument{ID: fmt.Sprintf("memory:%d", memory.ID), ChatSessionID: sid, SourceTable: "memories", SourceRowID: fmt.Sprint(memory.ID), Tier: "memory", DocumentText: summary, Embedding: []float32{1, .3, .2}, Metadata: map[string]any{"source_revision": revision, "turn_index": turn}})
			for j := 0; j < 8; j++ {
				unit := feedback43Unit(revision, turn*8+j)
				unit.ChatSessionID = sid
				unit.SourceTurnStart = turn
				unit.SourceTurnEnd = turn
				text := fmt.Sprintf("Mira examined brass key %d at archive station %d.", j, turn)
				payload, _ := json.Marshal(map[string]any{"event": text, "actor": "Mira", "visibility": "public"})
				unit.PayloadJSON = string(payload)
				unit.EvidenceExcerpt = text
				if _, err := st.(archiveStore.PreciseMemoryWriter).SavePreciseMemoryUnit(ctx, unit); err != nil {
					t.Fatal(err)
				}
				storedBytes += len(payload)
				docs = append(docs, vector.VectorDocument{ID: "precise_memory:" + unit.UnitID, ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: unit.UnitID, Tier: "precise_memory", DocumentText: text, Embedding: []float32{1, float32(j+1) / 10, .2}, Metadata: map[string]any{"source_revision": revision, "turn_index": turn, "unit_id": unit.UnitID}})
			}
		}
		if err := vec.Upsert(ctx, sid, docs); err != nil {
			t.Fatal(err)
		}

		var name string
		if err := db.QueryRow("SELECT DATABASE()").Scan(&name); err != nil {
			t.Fatal(err)
		}
		connection, err := mysql.ParseDSN(os.Getenv("AC_FEEDBACK_TEST_DSN"))
		if err != nil {
			t.Fatal(err)
		}
		connection.DBName = name
		connection.ParseTime = true
		report := os.Getenv("AC_MEASURE_REPORT")
		if report == "" {
			t.Fatal("AC_MEASURE_REPORT required")
		}
		for repeat := 0; repeat < 5; repeat++ {
			modes := []string{"measured", "baseline"}
			if repeat%2 == 1 {
				modes = []string{"baseline", "measured"}
			}
			for _, mode := range modes {
				command := exec.Command(filepath.Join(report, "runtime", mode+".test.exe"), "-test.run=^TestPreparationMeasurementMariaDBChroma$", "-test.v", "-test.timeout=5m")
				baseline := "0"
				if mode == "baseline" {
					baseline = "1"
				}
				command.Env = append(os.Environ(), "AC_MEASURE_CHILD=1", "AC_MEASURE_BASELINE="+baseline, "AC_MEASURE_SNAPSHOT_DSN="+connection.FormatDSN(), "AC_MEASURE_COLLECTION="+collection)
				log, err := os.Create(filepath.Join(report, fmt.Sprintf("http-%d-%d-%s.log", turns, repeat, mode)))
				if err != nil {
					t.Fatal(err)
				}
				command.Stdout = log
				command.Stderr = log
				err = command.Run()
				log.Close()
				if err != nil {
					t.Fatalf("snapshot worker %s repeat %d: %v", mode, repeat, err)
				}
				t.Logf("fixed snapshot %d turns / %s / %d", turns, mode, repeat)
			}
		}
		return
	}
	if err := db.QueryRow("SELECT COALESCE(SUM(OCTET_LENGTH(summary_json)),0) FROM memories WHERE chat_session_id=?", sid).Scan(&storedBytes); err != nil {
		t.Fatal(err)
	}
	var preciseBytes int
	if err := db.QueryRow("SELECT COALESCE(SUM(OCTET_LENGTH(payload_json)),0) FROM precise_memory_units WHERE chat_session_id=?", sid).Scan(&preciseBytes); err != nil {
		t.Fatal(err)
	}
	storedBytes += preciseBytes
	var embeddingCalls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Method != http.MethodPost || r.URL.Path != "/v1/embeddings" || json.NewDecoder(r.Body).Decode(&body) != nil || body["model"] != "synthetic-fixed" {
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", 400)
			return
		}
		text, ok := body["input"].(string)
		if !ok || strings.TrimSpace(text) == "" {
			t.Error("missing query text")
			http.Error(w, "missing", 400)
			return
		}
		embeddingCalls.Add(1)
		// Stable vector per query, preserving the existing six-query request flow.
		h := sha256.Sum256([]byte(text))
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "synthetic-fixed", "data": []any{map[string]any{"embedding": []float64{1, float64(h[0]) / 255, .2}}}})
	}))
	t.Cleanup(provider.Close)
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = endpoint
	server := httpapi.NewServer(cfg)
	server.Store = st
	server.Vector = vec
	server.RuntimeConfig = httpapi.RuntimeConfig{Synced: true, EmbeddingProvider: "custom", EmbeddingEndpoint: provider.URL + "/v1", EmbeddingModel: "synthetic-fixed", EmbeddingAPIKey: "synthetic", EmbeddingTimeoutSec: 30, LLMRetryCount: 1}
	routes := http.NewServeMux()
	server.RegisterRoutes(routes)
	messages := []any{}
	for i := 0; i < 5; i++ {
		messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprintf("Review archive station %d.", i)}, map[string]any{"role": "assistant", "content": fmt.Sprintf("Mira and Rowan examined the brass key at station %d.", i)})
	}
	request := map[string]any{"chat_session_id": sid, "turn_index": turns + 1, "raw_user_input": "Mira checks the archive door and asks Rowan about the brass key.", "messages": messages, "recent_conversation_messages": messages, "response_projection": "prepare_turn.production_compact.v1", "client_meta": map[string]any{"memory_budget_observation": map[string]any{"extra_chars": 4000}}, "settings": map[string]any{"apply_mode": "live", "guide_mode": "off", "guide_strength": "none", "max_injection_chars": 32000, "recent_conversation_reference_count": 5, "top_k": 5}}
	var identity string
	var identityMu sync.Mutex
	run := func(index int, mode string) {
		started := time.Now()
		response := storyTime46Request(t, routes, http.MethodPost, "/prepare-turn", request)
		elapsed := time.Since(started)
		plan := storyTime46Map(storyTime46Map(response["injection_pack"])["memory_delivery_plan"])
		digest := fmt.Sprint(plan["final_text_sha256"])
		if digest == "" || digest == "<nil>" || strings.TrimSpace(fmt.Sprint(plan["final_text"])) == "" {
			t.Fatal("empty memory delivery")
		}
		identityMu.Lock()
		if identity == "" {
			identity = digest
		} else if identity != digest {
			t.Error("fixed snapshot produced different final text")
		}
		identityMu.Unlock()
		vectorTrace := storyTime46Map(storyTime46Map(response["backend_timing"])["vector_recall"])
		if os.Getenv("AC_MEASURE_BASELINE") != "1" {
			counts := storyTime46Map(vectorTrace["query_observations"])
			if counts["embedding_calls"] != float64(6) || counts["precise.calls"] != float64(6) || counts["all.calls"] != float64(6) || counts["memory.calls"] != float64(6) {
				t.Fatalf("query measurements missing from compact response: %v", vectorTrace)
			}
		}
		record := map[string]any{"kind": "http_sql_chroma", "turns": turns, "stored_summary_payload_bytes": storedBytes, "vector_documents": turns * 9, "run": index, "mode": mode, "elapsed_ms": float64(elapsed) / float64(time.Millisecond), "final_text_sha256": digest, "backend_timing": response["backend_timing"], "vector": vectorTrace}
		encoded, _ := json.Marshal(record)
		t.Log(string(encoded))
	}
	run(0, "cold_first_http_request")
	for i := 0; i < 5; i++ {
		run(i, "warm")
	}
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func(i int) { defer workers.Done(); run(i, "concurrent_2") }(i)
	}
	workers.Wait()
	if embeddingCalls.Load() != 6*8 {
		t.Fatalf("actual query coverage: calls=%d want six per request", embeddingCalls.Load())
	}
	var memoriesAfter int
	if err := db.QueryRow("SELECT COUNT(*) FROM memories WHERE chat_session_id=?", sid).Scan(&memoriesAfter); err != nil || memoriesAfter != turns {
		t.Fatalf("fixed rows changed: %d %v", memoriesAfter, err)
	}
}
