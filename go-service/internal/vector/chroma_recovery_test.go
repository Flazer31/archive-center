package vector

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChromaRecoverySnapshotPreservesCurrentMetadataAndReplaysOnlyNewLog(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "chroma.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, query := range []string{
		`CREATE TABLE collections (id TEXT, dimension INTEGER)`,
		`CREATE TABLE segments (id TEXT, collection TEXT, scope TEXT)`,
		`CREATE TABLE max_seq_id (segment_id TEXT, seq_id INTEGER)`,
		`CREATE TABLE embeddings (id INTEGER, segment_id TEXT, embedding_id TEXT)`,
		`CREATE TABLE embedding_metadata (id INTEGER, key TEXT, string_value TEXT, int_value INTEGER, float_value REAL, bool_value INTEGER)`,
		`CREATE TABLE embeddings_queue (seq_id INTEGER, operation INTEGER, topic TEXT, id TEXT, vector BLOB, encoding TEXT, metadata TEXT)`,
		`INSERT INTO collections VALUES ('collection', 2), ('other', 2)`,
		`INSERT INTO segments VALUES ('segment', 'collection', 'METADATA')`,
		`INSERT INTO max_seq_id VALUES ('segment', 10)`,
		`INSERT INTO embeddings VALUES (1,'segment','kept'),(2,'segment','pruned'),(3,'segment','deleted-later')`,
		`INSERT INTO embedding_metadata VALUES (1,'chroma:document','current text',NULL,NULL,NULL),(1,'chat_session_id','session-a',NULL,NULL,NULL),(2,'chroma:document','no saved vector',NULL,NULL,NULL)`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	vec := make([]byte, 8)
	binary.LittleEndian.PutUint32(vec, math.Float32bits(1))
	binary.LittleEndian.PutUint32(vec[4:], math.Float32bits(0.5))
	insert := func(seq, op int, topic, id string, value []byte, meta string) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO embeddings_queue VALUES (?,?,?,?,?,'FLOAT32',?)`, seq, op, topic, id, value, meta); err != nil {
			t.Fatal(err)
		}
	}
	insert(1, 0, "persistent://x/y/collection", "kept", vec, `{"chroma:document":"outdated text"}`)
	insert(2, 0, "persistent://x/y/collection", "previously-deleted", vec, `{}`)
	insert(11, 3, "persistent://x/y/collection", "deleted-later", nil, `{}`)
	insert(12, 2, "persistent://x/y/collection", "new", vec, `{"chroma:document":"새 기억","chat_session_id":"session-b","flag":true}`)
	insert(13, 1, "persistent://x/y/collection", "kept", nil, `{"chroma:document":"updated text"}`)
	insert(14, 0, "persistent://x/y/other", "foreign", vec, `{}`)
	docs, dim, err := ReadChromaRecoverySnapshot(context.Background(), dir, "collection")
	if err != nil {
		t.Fatal(err)
	}
	if dim != 2 || len(docs) != 3 {
		t.Fatalf("dimension=%d docs=%+v", dim, docs)
	}
	if docs[0].ID != "kept" || docs[0].DocumentText != "updated text" || len(docs[0].Embedding) != 2 || docs[0].Embedding[1] != .5 {
		t.Fatalf("kept=%+v", docs[0])
	}
	if docs[1].ID != "new" || docs[1].DocumentText != "새 기억" || docs[1].ChatSessionID != "session-b" {
		t.Fatalf("new=%+v", docs[1])
	}
	if docs[2].ID != "pruned" || len(docs[2].Embedding) != 0 {
		t.Fatalf("missing vector was fabricated: %+v", docs[2])
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM collections`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("source changed: count=%d err=%v", count, err)
	}
}

func TestChromaRecoveryPreservesOriginalAndResumesPromotion(t *testing.T) {
	for _, scenario := range []string{"success", "rebuild-fails", "promotion-interrupted"} {
		t.Run(scenario, func(t *testing.T) {
			collections := map[string]chromaCollection{"original": {ID: "old-id", Name: "original", Metadata: map[string]any{"custom": "kept"}}}
			counts := map[string]int{"old-id": 7}
			failPromotion := scenario == "promotion-interrupted"
			deletes := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/api/v2/tenants/default_tenant/databases/default_database/collections")
				if path == "/api/v2/heartbeat" {
					_, _ = w.Write([]byte(`{}`))
					return
				}
				parts := strings.Split(strings.Trim(path, "/"), "/")
				name := parts[0]
				if path == "" && r.Method == http.MethodPost {
					var body chromaCollection
					_ = json.NewDecoder(r.Body).Decode(&body)
					body.ID = "new-id"
					collections[body.Name] = body
					counts[body.ID] = 0
					_ = json.NewEncoder(w).Encode(body)
					return
				}
				if len(parts) == 2 && parts[1] == "count" {
					_ = json.NewEncoder(w).Encode(counts[name])
					return
				}
				if r.Method == http.MethodPut {
					var body struct {
						Name string `json:"new_name"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					if name == "new-id" && failPromotion {
						failPromotion = false
						http.Error(w, "interrupted", 500)
						return
					}
					for key, col := range collections {
						if col.ID == name {
							delete(collections, key)
							col.Name = body.Name
							collections[body.Name] = col
							_, _ = w.Write([]byte(`{}`))
							return
						}
					}
				}
				if r.Method == http.MethodDelete {
					deletes = append(deletes, name)
					delete(collections, name)
					_, _ = w.Write([]byte(`{}`))
					return
				}
				if col, ok := collections[name]; ok {
					_ = json.NewEncoder(w).Encode(col)
					return
				}
				w.WriteHeader(404)
			}))
			defer server.Close()
			raw, _ := NewChromaStore(server.URL, "original", "/api/v2")
			recovery := raw.(IndexRecovery)
			journal := filepath.Join(t.TempDir(), "recovery.json")
			backup, err := recovery.RecoverIndex(context.Background(), journal, func(v VectorStore) error {
				if collections["original"].ID != "old-id" {
					t.Fatal("original changed before rebuild")
				}
				if scenario == "rebuild-fails" {
					return errors.New("cannot restore")
				}
				counts["new-id"] = 7
				return nil
			})
			if scenario == "rebuild-fails" {
				if err == nil || collections["original"].ID != "old-id" {
					t.Fatalf("err=%v original=%+v", err, collections)
				}
				for _, name := range deletes {
					if name == "original" || name == "old-id" {
						t.Fatal("original deleted")
					}
				}
				return
			}
			if scenario == "promotion-interrupted" {
				if err == nil {
					t.Fatal("interruption not reported")
				}
				fresh, _ := NewChromaStore(server.URL, "original", "/api/v2")
				if err := fresh.(IndexRecovery).ResumeIndexRecovery(context.Background(), journal); err != nil {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if collections["original"].ID != "new-id" || collections[backup].ID != "old-id" {
				t.Fatalf("collections=%+v backup=%s", collections, backup)
			}
			if collections["original"].Metadata["custom"] != "kept" {
				t.Fatal("collection metadata lost")
			}
			if _, err := os.Stat(journal); !os.IsNotExist(err) {
				t.Fatalf("journal remains: %v", err)
			}
		})
	}
}

func TestIndexLoadErrorClassification(t *testing.T) {
	for _, input := range []string{"connection refused", "401 unauthorized", "500 disk full", "dimension mismatch"} {
		if IsIndexLoadError(fmt.Errorf("%s", input)) {
			t.Fatalf("unrelated error matched: %s", input)
		}
	}
	if !IsIndexLoadError(errors.New("Error constructing hnsw segment reader: Error loading hnsw index")) {
		t.Fatal("reported error not recognized")
	}
}
