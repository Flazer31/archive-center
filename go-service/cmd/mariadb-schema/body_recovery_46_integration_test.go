package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestBodyRecovery46HTTPMariaDBDeleteBackupRestartRestoreAndStoryJump(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	db, st := feedback43Database(t)
	vectors := &bodyRecovery46Vectors{VectorStore: vector.NewFakeVectorStore(), docs: map[string]vector.VectorDocument{}}
	routes, provider, endpoint := bodyRecovery46Server(t, st, vectors)
	const sid, entity = "body-recovery", "recovery-mina"
	bodyProbability46Config(t, routes, st, sid, entity, 1)
	if _, err := db.Exec(`UPDATE character_states SET status_json='{"pregnancy":"confirmed","job":"maid","nearby":{"subject_label":"Vera","status":{"pregnancy":"confirmed","job":"medic"}}}' WHERE chat_session_id=? AND character_name='Mina'`, sid); err != nil {
		t.Fatal(err)
	}
	text, extraction := bodyTracking46Extraction(entity, "pregnancy_confirmed", "pregnancy", "1423-01-14")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"pregnancy-input"}, []string{"Record the pregnancy."}, text, extraction)
	before := storyTime46Current(t, st, sid, "body_tracking")
	memoriesBefore, err := st.ListMemories(context.Background(), sid, 0, 0)
	if err != nil || len(memoriesBefore) == 0 {
		t.Fatalf("fixture memory: %v", err)
	}
	for i, summary := range []string{"Mina buys a book.", "Vera is pregnant."} {
		bodyTracking46Complete(t, routes, provider, endpoint, sid, i+2, int64((i+2)*1000), []string{summary}, []string{summary}, summary, map[string]any{"turn_summary": summary, "importance_score": 5, "evidence_excerpts": []any{summary}})
	}
	episode := &archiveStore.EpisodeSummary{ChatSessionID: sid, FromTurn: 1, ToTurn: 3, SummaryText: "Mina's pregnancy was confirmed.", KeyEntities: `["Mina"]`, KeyEvents: `[]`, OpenLoopsJSON: `[]`, RelationshipChangesJSON: `[]`, EmbeddingVector: `[0.1,0.2,0.3]`, CreatedAt: time.Now().UTC()}
	if err := st.(interface {
		SaveEpisodeSummary(context.Context, *archiveStore.EpisodeSummary) error
	}).SaveEpisodeSummary(context.Background(), episode); err != nil {
		t.Fatal(err)
	}
	episodeVectorID := fmt.Sprintf("episode:%s:%d", sid, episode.ID)
	vectors.docs[episodeVectorID] = vector.VectorDocument{ID: episodeVectorID, ChatSessionID: sid, SourceTable: "episode_summaries", SourceRowID: fmt.Sprint(episode.ID), Tier: "episode", DocumentText: episode.SummaryText, Embedding: []float32{.1, .2, .3}}
	preview := storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid+"/state", map[string]any{"action": "delete", "character_id": entity, "operation_id": "delete-mina", "dry_run": true})
	if changes, ok := preview["memory_changes"].([]any); !ok || len(changes) < 2 {
		t.Fatalf("general memories absent from preview: %#v", preview)
	}
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.ValueJSON != before.ValueJSON {
		t.Fatal("preview mutated state")
	}
	request := map[string]any{"action": "delete", "character_id": entity, "operation_id": "delete-mina"}
	if _, err := db.Exec(`CREATE TRIGGER body_repair_failure BEFORE UPDATE ON direct_evidence_records FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected body repair write failure'`); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(request)
	failed := httptest.NewRecorder()
	routes.ServeHTTP(failed, httptest.NewRequest(http.MethodPut, "/config/body-tracking/"+sid+"/state", bytes.NewReader(encoded)))
	if failed.Code == http.StatusOK || storyTime46Current(t, st, sid, "body_tracking").ValueJSON != before.ValueJSON {
		t.Fatal("failed atomic delete committed body state")
	}
	var summaryAfterFailure string
	if err := db.QueryRow("SELECT summary_json FROM memories WHERE id=?", memoriesBefore[0].ID).Scan(&summaryAfterFailure); err != nil || summaryAfterFailure != memoriesBefore[0].SummaryJSON {
		t.Fatal("failed atomic delete partially removed a memory")
	}
	if _, err := db.Exec("DROP TRIGGER body_repair_failure"); err != nil {
		t.Fatal(err)
	}
	vectors.failDelete = true
	partial := storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid+"/state", request)
	if partial["status"] != "partial_error" || storyTime46Map(partial["current_state"])["deleted"] != true {
		t.Fatal("index failure hid the committed body deletion")
	}
	deleted := storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid+"/state", request)
	if deleted["status"] != "applied" || deleted["event_id"] != partial["event_id"] || vectors.docs[episodeVectorID].ID != "" {
		t.Fatalf("index repair retry: %#v", deleted)
	}
	if storyTime46Map(deleted["current_state"])["deleted"] != true {
		t.Fatalf("body state not removed: %#v", deleted)
	}
	character, err := st.GetCharacterState(context.Background(), sid, "Mina")
	if err != nil || character == nil || storyTime46JSON(t, character.StatusJSON)["pregnancy"] != nil || storyTime46Map(storyTime46Map(storyTime46JSON(t, character.StatusJSON)["nearby"])["status"])["pregnancy"] != "confirmed" || !strings.Contains(character.StatusJSON, "maid") || !strings.Contains(character.AppearanceJSON, "female") {
		t.Fatalf("related character state cleanup: %+v %v", character, err)
	}
	var remaining, preserved, originalLogs int
	if err := db.QueryRow("SELECT COUNT(*) FROM memories WHERE chat_session_id=? AND summary_json LIKE '%Mina%' AND summary_json LIKE '%pregnancy%'", sid).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("deleted pregnancy remained readable: %d %v", remaining, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM memories WHERE chat_session_id=? AND (summary_json LIKE '%buys a book%' OR summary_json LIKE '%Vera%')", sid).Scan(&preserved); err != nil || preserved != 2 {
		t.Fatalf("unrelated memories changed: %d %v", preserved, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM chat_logs WHERE chat_session_id=? AND content=?", sid, text).Scan(&originalLogs); err != nil || originalLogs == 0 {
		t.Fatalf("raw chat changed: %d %v", originalLogs, err)
	}
	view := storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+sid, nil)
	if len(storyTime46Map(view["settings"])["characters"].([]any)) != 0 || len(storyTime46Map(view["data_management"])["backups"].([]any)) != 1 {
		t.Fatalf("deleted woman automatically returned or backup missing: %#v", view)
	}
	// A new accepted story clock advances while the deleted character is absent.
	clockText := "On 1426-01-14, the narrator records that three years have passed."
	clockExtraction := map[string]any{"turn_summary": clockText, "importance_score": 5, "story_clock": map[string]any{"version": "story_clock.v1", "observation_kind": "absolute", "precision": "exact", "scene_scope": "current", "transition": "advance", "absolute": map[string]any{"date": "1426-01-14"}, "evidence_excerpt": clockText}}
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 4, 4000, []string{"three-years"}, []string{"Continue after three years."}, clockText, clockExtraction)
	routes, _, _ = bodyRecovery46Server(t, st, vectors)
	view = storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+sid, nil)
	if len(storyTime46Map(view["data_management"])["backups"].([]any)) != 1 {
		t.Fatal("backup did not survive backend restart")
	}
	const branch = "body-recovery-branch"
	copyResult := storyTime46Request(t, routes, http.MethodPost, "/sessions/migrate-complete", map[string]any{"source_session_id": sid, "target_session_id": branch, "mode": archiveStore.SessionMigrationModeCopyKeepSource})
	if copyResult["blocked"] == true || copyResult["write_attempted"] != true {
		t.Fatalf("deleted-data branch copy: %#v", copyResult)
	}
	branchView := storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+branch, nil)
	branchBackups := storyTime46Map(branchView["data_management"])["backups"].([]any)
	if len(branchBackups) != 1 || len(storyTime46Map(branchView["settings"])["characters"].([]any)) != 0 {
		t.Fatal("branch lost deletion/backup")
	}
	if _, err := db.Exec(`UPDATE character_states SET appearance_json='{"gender":"male"}', status_json=JSON_SET(status_json,'$.job','captain','$.nearby.status.job','doctor') WHERE chat_session_id=? AND character_name='Mina'`, branch); err != nil {
		t.Fatal(err)
	}
	storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+branch+"/state", map[string]any{"action": "restore", "event_id": storyTime46Map(branchBackups[0])["event_id"], "operation_id": "branch-restore"})
	var branchEpisodeID int64
	if err := db.QueryRow("SELECT id FROM episode_summaries WHERE chat_session_id=?", branch).Scan(&branchEpisodeID); err != nil {
		t.Fatal(err)
	}
	branchVectorID := fmt.Sprintf("episode:%s:%d", branch, branchEpisodeID)
	if vectors.docs[branchVectorID].DocumentText != episode.SummaryText || vectors.docs[episodeVectorID].ID != "" {
		t.Fatal("branch vector restoration wrote the origin or lost the backup")
	}
	var branchRestored int
	if err := db.QueryRow("SELECT COUNT(*) FROM memories WHERE chat_session_id=? AND summary_json LIKE '%pregnancy%confirmed%'", branch).Scan(&branchRestored); err != nil || branchRestored == 0 {
		t.Fatalf("branch restore did not remap backup memory IDs: %d %v", branchRestored, err)
	}
	branchCharacter, _ := st.GetCharacterState(context.Background(), branch, "Mina")
	if branchCharacter == nil || !strings.Contains(branchCharacter.AppearanceJSON, "male") || strings.Contains(branchCharacter.AppearanceJSON, "female") {
		t.Fatal("body restoration overwrote a later gender correction")
	}
	status := storyTime46JSON(t, branchCharacter.StatusJSON)
	if status["pregnancy"] != "confirmed" || status["job"] != "captain" || storyTime46Map(storyTime46Map(status["nearby"])["status"])["job"] != "doctor" {
		t.Fatalf("restore rewound same-column fields: %s", branchCharacter.StatusJSON)
	}
	if storyTime46JSON(t, storyTime46Current(t, st, sid, "body_tracking").ValueJSON)["deleted"] != true {
		t.Fatal("branch restore changed original")
	}
	restored := storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid+"/state", map[string]any{"action": "restore", "event_id": deleted["event_id"], "operation_id": "restore-mina"})
	if vectors.docs[episodeVectorID].DocumentText != episode.SummaryText {
		t.Fatal("old hierarchy vector backup not restored")
	}
	if !reflect.DeepEqual(storyTime46Map(restored["current_state"]), storyTime46JSON(t, before.ValueJSON)) {
		t.Fatal("restoration changed body snapshot")
	}
	memoriesAfter, err := st.ListMemories(context.Background(), sid, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range memoriesBefore {
		found := false
		for _, current := range memoriesAfter {
			if current.ID == original.ID {
				found = true
				if current.SummaryJSON != original.SummaryJSON || current.Evidence != original.Evidence {
					t.Fatal("general memory backup was not restored exactly")
				}
			}
		}
		if !found {
			t.Fatal("restoration replaced an existing memory identity")
		}
	}
	view = storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+sid, nil)
	readings := view["model_readings"].([]any)
	if len(readings) != 1 || storyTime46Map(storyTime46Map(readings[0])["reading"])["stage"] != "modeled_birth_completed" {
		states, _ := st.ListCharacterStates(context.Background(), sid)
		t.Fatalf("three-year-old restored pregnancy still ongoing: %#v settings=%#v states=%#v", readings, view["settings"], states)
	}
	replay := storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid+"/state", request)
	if replay["replayed"] != true || storyTime46Map(replay["current_state"])["deleted"] == true {
		t.Fatal("old delete retry overwrote restore")
	}
	// Backup data is kept in repair history; it is not copied into model readings.
	readingJSON, _ := json.Marshal(view["model_readings"])
	if strings.Contains(string(readingJSON), "repair_artifacts") {
		t.Fatal("backup leaked into model context")
	}
}

// The remote vector boundary records real mutations; SQL and repair handlers
// remain the production implementations. Fail one delete to exercise retry.
type bodyRecovery46Vectors struct {
	vector.VectorStore
	docs       map[string]vector.VectorDocument
	failDelete bool
}

func (v *bodyRecovery46Vectors) GetDocuments(_ context.Context, ids []string) ([]vector.VectorDocument, error) {
	out := []vector.VectorDocument{}
	for _, id := range ids {
		if doc, ok := v.docs[id]; ok {
			out = append(out, doc)
		}
	}
	return out, nil
}
func (v *bodyRecovery46Vectors) DeleteDocuments(_ context.Context, ids []string) error {
	if v.failDelete {
		v.failDelete = false
		return fmt.Errorf("injected vector delete failure")
	}
	for _, id := range ids {
		delete(v.docs, id)
	}
	return nil
}
func (v *bodyRecovery46Vectors) Upsert(_ context.Context, sid string, docs []vector.VectorDocument) error {
	for _, doc := range docs {
		if doc.ChatSessionID != sid {
			return fmt.Errorf("wrong vector session")
		}
		v.docs[doc.ID] = doc
	}
	return nil
}
func bodyRecovery46Server(t *testing.T, st archiveStore.Store, vectors vector.VectorStore) (http.Handler, *storyTime46Provider, string) {
	t.Helper()
	provider := &storyTime46Provider{}
	upstream := httptest.NewServer(provider)
	t.Cleanup(upstream.Close)
	server := httpapi.NewServer(config.Config{})
	server.Cfg.StoreMode, server.Cfg.ChromaEndpoint = config.StoreModeMariaDBAuthority, upstream.URL+"/fake-vector-in-process"
	server.Store, server.StoreOpenError, server.Vector = st, nil, vectors
	server.RuntimeConfig = httpapi.RuntimeConfig{Synced: true, CriticTimeoutSec: 10, LLMRetryCount: 1}
	routes := http.NewServeMux()
	server.RegisterRoutes(routes)
	return routes, provider, upstream.URL + "/v1"
}
