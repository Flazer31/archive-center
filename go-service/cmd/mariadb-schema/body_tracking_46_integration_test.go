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

	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

// Use the production v3 afterRequest contract: physical input rows identify the
// logical turn even when the host proposes the next ordinal after answer removal.
func bodyTracking46Complete(t *testing.T, routes http.Handler, provider *storyTime46Provider, endpoint, sid string, proposedTurn int, at int64, ids, texts []string, assistant string, extraction map[string]any) map[string]any {
	t.Helper()
	provider.mu.Lock()
	provider.extraction = extraction
	provider.mu.Unlock()
	user, correlation := strings.Join(texts, "\n\n"), fmt.Sprintf("body-request-%s-%d", sid, at)
	refs := []any{}
	for i, id := range ids {
		refs = append(refs, map[string]any{"message_index": i, "message_chat_id": id, "content_hash": storyTime46Hash(texts[i])})
	}
	observation := map[string]any{
		"contract_version": "source_acceptance_observation.v3", "host_lifecycle_contract_version": "risu_host_lifecycle_observation.v1",
		"observed_at_ms": at, "session_id": sid, "finality_source": "risu_afterRequest", "finality_state": "received_final_response", "host_signal_source": "afterRequest",
		"archive_center_request_correlation_id": correlation, "request_id_provenance": "archive_center_correlation", "request_correlation_state": "matched_before_request_context", "request_type": "model", "response_role": "assistant",
		"after_request_content_hash": storyTime46Hash(assistant), "host_chat_id": sid + "-chat", "host_chat_id_state": "observed_before_request",
		"chat_streaming_state": "not_exposed_by_risu_afterRequest", "active_message_count": 0, "message_index": -1, "message_role": "", "message_chat_id": "", "message_chat_id_state": "not_exposed_by_risu_afterRequest",
		"generation_id": "", "generation_id_state": "not_exposed_by_risu_afterRequest", "branch_id": "", "branch_id_state": "not_exposed_by_risuai", "message_swipe_id": -1, "message_swipe_id_state": "unobserved", "message_time_ms": 0, "message_time_state": "not_exposed_by_risu_afterRequest",
		"request_message_count": len(ids), "user_message_index": len(ids) - 1, "user_observed_pair_ordinal": proposedTurn, "user_message_refs": refs,
		"user_message_chat_id": ids[len(ids)-1], "user_message_chat_id_state": "observed_before_request", "user_message_time_ms": 500, "user_message_time_state": "observed_before_request",
		"user_observed_content_hash": storyTime46Hash(user), "user_persistence_content_hash": storyTime46Hash(user), "observed_content_hash": storyTime46Hash(assistant), "persistence_content_hash": storyTime46Hash(assistant), "hash_algorithm": "or1c_utf16_djb2.v1",
		"position_observation": "not_exposed_by_risu_afterRequest", "message_disabled_state": "not_exposed_by_risu_afterRequest", "revision_state": "not_exposed_by_risuai",
	}
	response := storyTime46Request(t, routes, http.MethodPost, "/complete-turn", map[string]any{
		"chat_session_id": sid, "turn_index": proposedTurn, "user_input": user, "assistant_content": assistant,
		"client_meta": map[string]any{"source_acceptance_required": true, "archive_center_request_correlation_id": correlation, "source_acceptance_observation": observation,
			"critic":    map[string]any{"provider": "openai", "api_key": "disposable-local-test", "endpoint": endpoint, "model": "body-fixture", "timeout_ms": 10000},
			"embedding": map[string]any{"provider": "openai", "api_key": "disposable-local-test", "endpoint": endpoint + "/embeddings", "model": "local-test-embedding", "timeout_ms": 10000}},
	})
	if response["status"] != "ok" || (response["critic_triggered"] != true && storyTime46Map(response["trace_handoff"])["idempotent_replay"] != true) {
		t.Fatalf("v3 production extraction was not accepted: %#v", response)
	}
	if errors, ok := response["derived_artifacts_errors"].(float64); ok && errors != 0 {
		t.Fatalf("body reducer failed: %#v", response)
	}
	return response
}

func bodyTracking46Female(t *testing.T, st archiveStore.Store, sid string) {
	t.Helper()
	state := archiveStore.CharacterState{ChatSessionID: sid, CharacterName: "Mina", AppearanceJSON: `{"gender":"female","species":"elf","age":900}`, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := st.(interface {
		SaveCharacterState(context.Context, *archiveStore.CharacterState) error
	}).SaveCharacterState(context.Background(), &state); err != nil {
		t.Fatal(err)
	}
}

func TestBodyTracking46HTTPMariaDBAutomaticWomenAndInitialPhases(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid = "automatic-body-women"
	storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid, map[string]any{"cycle_tracking_enabled": true})
	text := "On 1423-01-15, Mina, a 900-year-old elf woman, meets Vera, a dragon woman, and Rian, a human man."
	extraction := map[string]any{
		"turn_summary": text, "importance_score": 7, "evidence_excerpts": []any{text},
		"story_clock": map[string]any{"version": "story_clock.v1", "observation_kind": "absolute", "precision": "exact", "scene_scope": "current", "transition": "advance", "absolute": map[string]any{"date": "1423-01-15"}, "evidence_excerpt": text},
		"entities":    map[string]any{"characters": []any{map[string]any{"name": "Mina"}, map[string]any{"name": "Vera"}, map[string]any{"name": "Rian"}}},
		"character_deltas": []any{
			map[string]any{"name": "Mina", "appearance": map[string]any{"gender": "female", "species": "elf", "age": 900}},
			map[string]any{"name": "Vera", "appearance": map[string]any{"gender": "female", "species": "dragon"}},
			map[string]any{"name": "Rian", "appearance": map[string]any{"gender": "male", "species": "human"}},
		},
	}
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"auto-A"}, []string{"Introduce the characters."}, text, extraction)
	view := storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+sid, nil)
	settings := storyTime46Map(view["settings"])
	characters := settings["characters"].([]any)
	if len(characters) != 2 || settings["automatic_pregnancy_enabled"] != false {
		t.Fatalf("automatic female scope or independent toggle: %#v", settings)
	}
	for _, raw := range characters {
		character := storyTime46Map(raw)
		cycle := storyTime46Map(character["cycle"])
		if character["character_name"] == "Rian" || cycle["reference_kind"] != "model_initialization" || len(storyTime46Map(cycle["reference_time"])) == 0 {
			t.Fatalf("automatic phase was not persisted: %#v", character)
		}
	}
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"auto-A"}, []string{"Introduce the characters."}, text, extraction)
	after := storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+sid, nil)
	if !reflect.DeepEqual(settings, storyTime46Map(after["settings"])) {
		t.Fatal("same-source replay resampled automatic phases")
	}
	values, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(context.Background(), sid, "fictional_entity", "", "body_tracking", -1)
	if err != nil || len(values) != 0 {
		t.Fatalf("model initialization fabricated observed body facts: %#v %v", values, err)
	}
}

func bodyTracking46Extraction(entity, kind, key, date string) (string, map[string]any) {
	text := fmt.Sprintf("On %s, Mina records %s in her private journal.", date, kind)
	return text, map[string]any{
		"turn_summary": text, "importance_score": 7, "evidence_excerpts": []any{text},
		"story_clock": map[string]any{"version": "story_clock.v1", "observation_kind": "absolute", "precision": "exact", "scene_scope": "current", "transition": "advance", "absolute": map[string]any{"date": date}, "evidence_excerpt": text},
		"body_events": []any{map[string]any{"kind": kind, "character_id": entity, "semantic_event_key": key, "occurred_at": map[string]any{"date": date}, "evidence_excerpt": text, "visibility": "private", "event_token": "observed-token-" + key}},
	}
}

func bodyTracking46History(t *testing.T, st archiveStore.Store, sid, entity string, count int) []archiveStore.StatusChangeEvent {
	t.Helper()
	events, err := st.(archiveStore.StatusLifecycleStore).ListStatusChangeEvents(context.Background(), sid, "fictional_entity", entity, "body_tracking", -1)
	if err != nil {
		t.Fatal(err)
	}
	activeEvents := []archiveStore.StatusChangeEvent{}
	for _, event := range events {
		revision, _ := storyTime46JSON(t, event.EvidenceJSON)["source_revision"].(string)
		active, err := st.(archiveStore.SourceRevisionStore).IsSourceRevisionActive(context.Background(), sid, revision)
		if err != nil {
			t.Fatal(err)
		}
		if active {
			activeEvents = append(activeEvents, event)
		}
	}
	if len(activeEvents) != count {
		t.Fatalf("active body history %s/%s count=%d want=%d total=%d", sid, entity, len(activeEvents), count, len(events))
	}
	return activeEvents
}

func TestBodyTracking46HTTPMariaDBV3ReplaceRetryRollbackAndBranch(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	db, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid, branch, entity = "body-main", "body-branch", "body-main-mina"
	// Missing settings stay OFF; the ordinary admitted source still retains its
	// observation instead of silently creating a body projection.
	offText, offExtraction := bodyTracking46Extraction(entity, "period_start", "off-period", "1422-12-01")
	bodyTracking46Complete(t, routes, provider, endpoint, "body-default-off", 1, 500, []string{"off-input"}, []string{"Record an observation."}, offText, offExtraction)
	bodyTracking46History(t, st, "body-default-off", entity, 0)
	if got := storyTime46JSON(t, storyTime46Memory(t, st, "body-default-off", 1).SummaryJSON)["body_events"]; got == nil {
		t.Fatal("default-OFF setting discarded ordinary source memory")
	}
	identity := archiveStore.EntityIdentity{StableEntityID: entity, ChatSessionID: sid, IdentityNamespace: "fictional", EntityKind: "character", CanonicalLabel: "Mina", LifecycleState: "active", ReviewState: "reviewed", SourceContract: "manual", IdempotencyKey: "body-fixture-identity", MappingRevision: 1}
	if err := st.(archiveStore.EntityIdentityWriter).SaveEntityIdentity(context.Background(), &identity); err != nil {
		t.Fatal(err)
	}
	bodyTracking46Female(t, st, sid)
	storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid, map[string]any{"cycle_tracking_enabled": true, "characters": []any{map[string]any{"entity_id": entity, "character_name": "Mina"}}})
	text, extraction := bodyTracking46Extraction(entity, "period_start", "period-one", "1423-01-01")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"A", "B"}, []string{"first", "last"}, text, extraction)
	first := storyTime46Current(t, st, sid, "body_tracking")
	bodyTracking46History(t, st, sid, entity, 1)
	// Same request and a fresh in-memory server both retain the accepted result.
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"A", "B"}, []string{"first", "last"}, text, extraction)
	routes, provider, endpoint = storyTime46Server(t, st)
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"A", "B"}, []string{"first", "last"}, text, extraction)
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.ValueJSON != first.ValueJSON {
		t.Fatal("retry/restart changed the already observed body event")
	}
	cases := []struct{ ids, texts []string }{{[]string{"A", "B"}, []string{"first", "last"}}, {[]string{"A", "B"}, []string{"first edited", "last"}}, {[]string{"A"}, []string{"first edited"}}}
	for i, tc := range cases {
		routes, provider, endpoint = storyTime46Server(t, st)
		text, extraction = bodyTracking46Extraction(entity, "period_start", fmt.Sprintf("replacement-%d", i), fmt.Sprintf("1423-01-%02d", i+2))
		bodyTracking46Complete(t, routes, provider, endpoint, sid, 2, int64(2000+i*1000), tc.ids, tc.texts, text, extraction)
		current := storyTime46Current(t, st, sid, "body_tracking")
		if current.SourceTurn != 1 || storyTime46Map(storyTime46JSON(t, current.ValueJSON)["cycle_reference"])["date"] != fmt.Sprintf("1423-01-%02d", i+2) {
			t.Fatalf("reroll/edit/delete did not replace the same logical turn: %+v", current)
		}
		bodyTracking46History(t, st, sid, entity, 1)
	}
	var firstSourceState string
	if err := db.QueryRow(`SELECT lifecycle_state FROM memory_source_revisions WHERE source_revision=?`, storyTime46JSON(t, first.EvidenceJSON)["source_revision"]).Scan(&firstSourceState); err != nil || firstSourceState == "active" {
		t.Fatalf("replaced source remained active: %s %v", firstSourceState, err)
	}
	prior := storyTime46Current(t, st, sid, "body_tracking")
	text, extraction = bodyTracking46Extraction(entity, "period_start", "period-february", "1423-02-01")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 2, 6000, []string{"new-A"}, []string{"first edited"}, text, extraction)
	current := storyTime46Current(t, st, sid, "body_tracking")
	if current.SourceTurn != 2 {
		t.Fatal("new input row with identical text did not get a new logical turn")
	}
	parentEvents := bodyTracking46History(t, st, sid, entity, 2)
	exported := storyTime46Request(t, routes, http.MethodGet, "/sessions/"+sid+"/export", nil)
	settings := storyTime46Map(storyTime46Map(exported["body_tracking_settings"])["config"])
	if settings["simulation_seed"] == "" || settings["simulation_seed"] == nil {
		t.Fatal("export did not preserve the operator simulation seed")
	}
	// Fresh private-only admission must establish its own explicit no-public
	// authority. No administrative reindex is performed before this branch.
	var foreignEvidence, inactiveMissingEvidence int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_derivation_dependencies d JOIN direct_evidence_records e ON e.id=d.parent_artifact_id WHERE d.chat_session_id=? AND d.parent_artifact_type='direct_evidence' AND e.chat_session_id<>d.chat_session_id`, sid).Scan(&foreignEvidence); err != nil || foreignEvidence != 0 {
		t.Fatalf("postcommit evidence identity escaped source session: count=%d err=%v", foreignEvidence, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_derivation_dependencies d JOIN memory_source_revisions s ON s.source_revision=d.source_revision LEFT JOIN direct_evidence_records e ON e.id=d.parent_artifact_id WHERE d.chat_session_id=? AND d.parent_artifact_type='direct_evidence' AND e.id IS NULL AND s.lifecycle_state<>'active'`, sid).Scan(&inactiveMissingEvidence); err != nil {
		t.Fatal(err)
	}
	t.Logf("inactive dependency audit: missing old evidence=%d; cross-session evidence=%d", inactiveMissingEvidence, foreignEvidence)
	if inactiveMissingEvidence != 6 {
		t.Fatalf("fixture lost the three replaced sources' retained inactive dependencies: %d", inactiveMissingEvidence)
	}
	// The exception is limited to a proven invalidated relation. Active,
	// foreign-session bindings still fail the existing copy. The source schema
	// separately rejects unknown lifecycle states and duplicate active turns.
	var dependencyID int64
	var oldRevision, foreignRevision string
	if err := db.QueryRow(`SELECT d.id,d.source_revision FROM memory_derivation_dependencies d JOIN memory_source_revisions s ON s.source_revision=d.source_revision LEFT JOIN direct_evidence_records e ON e.id=d.parent_artifact_id WHERE d.chat_session_id=? AND d.parent_artifact_type='direct_evidence' AND e.id IS NULL AND s.lifecycle_state='superseded' ORDER BY d.id LIMIT 1`, sid).Scan(&dependencyID, &oldRevision); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT source_revision FROM memory_source_revisions WHERE chat_session_id='body-default-off' AND lifecycle_state='active'`).Scan(&foreignRevision); err != nil {
		t.Fatal(err)
	}
	for _, control := range []struct {
		name, setSQL, resetSQL string
		args, resetArgs        []any
	}{
		{"active relation", `UPDATE memory_derivation_dependencies SET lifecycle_state='active' WHERE id=?`, `UPDATE memory_derivation_dependencies SET lifecycle_state='invalidated' WHERE id=?`, []any{dependencyID}, []any{dependencyID}},
		{"foreign source binding", `UPDATE memory_derivation_dependencies SET source_revision=? WHERE id=?`, `UPDATE memory_derivation_dependencies SET source_revision=? WHERE id=?`, []any{foreignRevision, dependencyID}, []any{oldRevision, dependencyID}},
	} {
		if _, err := db.Exec(control.setSQL, control.args...); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(map[string]any{"source_session_id": sid, "target_session_id": branch, "mode": archiveStore.SessionMigrationModeCopyKeepSource})
		response := httptest.NewRecorder()
		routes.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/sessions/migrate-complete", bytes.NewReader(raw)))
		if _, err := db.Exec(control.resetSQL, control.resetArgs...); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "row map") {
			t.Fatalf("%s bypassed strict dependency copy: %d %s", control.name, response.Code, response.Body.String())
		}
	}
	copyResult := storyTime46Request(t, routes, http.MethodPost, "/sessions/migrate-complete", map[string]any{"source_session_id": sid, "target_session_id": branch, "mode": archiveStore.SessionMigrationModeCopyKeepSource})
	if copyResult["blocked"] == true || copyResult["write_attempted"] != true {
		t.Fatalf("production branch failed: %#v", copyResult)
	}
	var timestampDifferences, copiedSuperseded int
	if err := db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(source.updated_at<>target.updated_at),0) FROM memory_source_revisions source JOIN session_migration_artifact_row_map mapping ON mapping.table_name='memory_source_revisions' AND mapping.key_column_name='id' AND BINARY mapping.source_key=BINARY CAST(source.id AS CHAR) JOIN memory_source_revisions target ON BINARY CAST(target.id AS CHAR)=BINARY mapping.target_key WHERE source.chat_session_id=? AND target.chat_session_id=? AND source.superseded_by_revision IS NOT NULL`, sid, branch).Scan(&copiedSuperseded, &timestampDifferences); err != nil || copiedSuperseded != 3 || timestampDifferences != 0 {
		t.Fatalf("deferred superseded chain changed source observation timestamps: copied=%d changed=%d err=%v", copiedSuperseded, timestampDifferences, err)
	}
	branchExport := storyTime46Request(t, routes, http.MethodGet, "/sessions/"+branch+"/export", nil)
	branchSettings := storyTime46Map(storyTime46Map(branchExport["body_tracking_settings"])["config"])
	character := storyTime46Map(branchSettings["characters"].([]any)[0])
	branchEntity, _ := character["entity_id"].(string)
	if branchEntity == entity || branchEntity == "" || character["origin_entity_id"] != entity || branchSettings["simulation_seed"] != settings["simulation_seed"] {
		t.Fatalf("branch settings changed immutable identity/seed or missed new identity: %#v", branchSettings)
	}
	branchCurrent := storyTime46Current(t, st, branch, "body_tracking")
	branchPayload := storyTime46JSON(t, branchCurrent.ValueJSON)
	branchFact := storyTime46Map(storyTime46Map(branchPayload["observed_facts"])["period_start"])
	if branchCurrent.OwnerID != branchEntity || branchPayload["subject_entity_id"] != branchEntity || branchFact["character_id"] != branchEntity || branchFact["origin_entity_id"] != entity || branchFact["event_token"] != "observed-token-period-february" || branchPayload["pregnancy"] != nil {
		t.Fatalf("copied state operational/origin identity mismatch: %+v %#v", branchCurrent, branchPayload)
	}
	branchEvents := bodyTracking46History(t, st, branch, branchEntity, 2)
	if storyTime46JSON(t, branchEvents[0].EvidenceJSON)["source_unit_id"] != storyTime46JSON(t, parentEvents[0].EvidenceJSON)["source_unit_id"] || !reflect.DeepEqual(branchFact["source"], storyTime46Map(storyTime46Map(storyTime46JSON(t, current.ValueJSON)["observed_facts"])["period_start"])["source"]) {
		t.Fatal("branch changed immutable semantic event identity or original source provenance")
	}
	active, err := st.(archiveStore.ReversibleStatusTransitionStore).ListReversibleStatusCurrentValues(context.Background(), branch, "fictional_entity", []string{"body_tracking"})
	if err != nil || len(active) != 1 || active[0].OwnerID != branchEntity {
		t.Fatalf("copied body projection is not active under the new identity: %+v %v", active, err)
	}
	// A later mention of the copied semantic event reuses the recorded event.
	text, extraction = bodyTracking46Extraction(branchEntity, "period_start", "period-february", "1423-02-01")
	bodyTracking46Complete(t, routes, provider, endpoint, branch, 3, 7000, []string{"branch-new-row"}, []string{"Recall the recorded observation."}, text, extraction)
	bodyTracking46History(t, st, branch, branchEntity, 2)
	if got := storyTime46Current(t, st, branch, "body_tracking"); got.ValueJSON != branchCurrent.ValueJSON {
		t.Fatal("later branch mention redrew/replaced the copied event")
	}
	text, extraction = bodyTracking46Extraction(branchEntity, "period_start", "period-march", "1423-03-01")
	bodyTracking46Complete(t, routes, provider, endpoint, branch, 4, 8000, []string{"branch-next-row"}, []string{"Record a new occurrence."}, text, extraction)
	bodyTracking46History(t, st, branch, branchEntity, 3)
	if got := storyTime46Current(t, st, branch, "body_tracking"); got.OwnerID != branchEntity || got.SourceTurn != 4 {
		t.Fatalf("branch next update missed the remapped owner: %+v", got)
	}
	storyTime46Request(t, routes, http.MethodDelete, "/rollback/4?chat_session_id="+branch+"&req_source=timeline_manual_delete", nil)
	if got := storyTime46Current(t, st, branch, "body_tracking"); got.ValueJSON != branchCurrent.ValueJSON {
		t.Fatal("branch rollback did not restore the copied predecessor")
	}
	storyTime46Request(t, routes, http.MethodDelete, "/rollback/2?chat_session_id="+sid+"&req_source=timeline_manual_delete", nil)
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.ValueJSON != prior.ValueJSON || got.SourceTurn != 1 {
		t.Fatalf("rollback did not restore exact surviving body state: %+v", got)
	}
	if got := storyTime46Current(t, st, branch, "body_tracking"); got.ValueJSON != branchCurrent.ValueJSON {
		t.Fatal("parent rollback changed sibling body state")
	}
}

func TestBodyTracking46HTTPMariaDBManualBranchUndoKeepsOperationalIdentity(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid, branch, entity = "body-manual-main", "body-manual-branch", "body-manual-mina"
	identity := archiveStore.EntityIdentity{StableEntityID: entity, ChatSessionID: sid, IdentityNamespace: "fictional", EntityKind: "character", CanonicalLabel: "Mina", LifecycleState: "active", ReviewState: "reviewed", SourceContract: "manual", IdempotencyKey: "manual-fixture-identity", MappingRevision: 1}
	if err := st.(archiveStore.EntityIdentityWriter).SaveEntityIdentity(context.Background(), &identity); err != nil {
		t.Fatal(err)
	}
	bodyTracking46Female(t, st, sid)
	storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid, map[string]any{"cycle_tracking_enabled": true, "characters": []any{map[string]any{"entity_id": entity, "character_name": "Mina"}}})
	text, extraction := bodyTracking46Extraction(entity, "period_start", "source-period", "1423-01-01")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"manual-A"}, []string{"Record the beginning."}, text, extraction)
	before := storyTime46Current(t, st, sid, "body_tracking")
	applied := storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid+"/state", map[string]any{"action": "apply", "operation_id": "author-period-correction", "character_id": entity, "event": map[string]any{"kind": "period_start", "occurred_at": map[string]any{"date": "1423-01-03"}, "evidence_excerpt": "Author corrects the date."}})
	if applied["status"] != "applied" {
		t.Fatalf("manual correction failed: %#v", applied)
	}
	current := storyTime46Current(t, st, sid, "body_tracking")
	if current.SourceTurn != 0 || storyTime46JSON(t, current.EvidenceJSON)["source"] != "author_setting" {
		t.Fatal("manual body correction invented a source occurrence turn")
	}
	copyResult := storyTime46Request(t, routes, http.MethodPost, "/sessions/migrate-complete", map[string]any{"source_session_id": sid, "target_session_id": branch, "mode": archiveStore.SessionMigrationModeCopyKeepSource})
	if copyResult["blocked"] == true || copyResult["write_attempted"] != true {
		t.Fatalf("manual branch failed: %#v", copyResult)
	}
	branchCurrent := storyTime46Current(t, st, branch, "body_tracking")
	events, err := st.(archiveStore.StatusLifecycleStore).ListStatusChangeEvents(context.Background(), branch, "fictional_entity", branchCurrent.OwnerID, "body_tracking", -1)
	if err != nil {
		t.Fatal(err)
	}
	var manualID int64
	for _, event := range events {
		if event.EventKind == "author_period_start" {
			manualID = event.ID
		}
	}
	if manualID == 0 {
		t.Fatal("copied manual correction history missing")
	}
	undone := storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+branch+"/state", map[string]any{"action": "undo", "operation_id": "branch-undo-period", "event_id": manualID})
	if undone["status"] != "applied" {
		t.Fatalf("copied manual undo failed: %#v", undone)
	}
	restored := storyTime46Current(t, st, branch, "body_tracking")
	payload := storyTime46JSON(t, restored.ValueJSON)
	fact := storyTime46Map(storyTime46Map(payload["observed_facts"])["period_start"])
	if restored.OwnerID == entity || restored.OwnerID != branchCurrent.OwnerID || payload["subject_entity_id"] != restored.OwnerID || fact["character_id"] != restored.OwnerID || fact["origin_entity_id"] != entity || !reflect.DeepEqual(payload["cycle_reference"], storyTime46JSON(t, before.ValueJSON)["cycle_reference"]) {
		t.Fatalf("branch undo restored stale parent operational identity: %+v %#v", restored, payload)
	}
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.ValueJSON != current.ValueJSON {
		t.Fatal("branch manual undo changed original author state")
	}
}

func TestBodyTracking46HTTPMariaDBClarificationAndPregnancyContinuity(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid, entity = "body-continuity-fix", "continuity-mina"
	bodyProbability46Config(t, routes, st, sid, entity, 1)
	save := func(turn int, kind, key, sceneDate, eventDate string) {
		text, extraction := bodyTracking46Extraction(entity, kind, key, sceneDate)
		event := storyTime46Map(extraction["body_events"].([]any)[0])
		event["occurred_at"] = map[string]any{"date": eventDate}
		if kind == "conception_exposure" {
			event["exposure"] = map[string]any{"classification": "potentially_conceiving", "partner_compatibility": "compatible", "contraception": "none", "model_profile": "author_allowed"}
		}
		text += " The recorded event occurred on " + eventDate + "."
		event["evidence_excerpt"] = text
		bodyTracking46Complete(t, routes, provider, endpoint, sid, turn, int64(turn)*1000, []string{fmt.Sprintf("new-row-%d", turn)}, []string{"Record the new observation."}, text, extraction)
	}
	save(1, "pregnancy_confirmed", "initial", "1423-01-05", "1423-01-05")
	first := storyTime46Map(storyTime46JSON(t, storyTime46Current(t, st, sid, "body_tracking").ValueJSON)["pregnancy"])
	save(2, "pregnancy_confirmed", "followup", "1423-01-15", "1423-01-15")
	second := storyTime46Map(storyTime46JSON(t, storyTime46Current(t, st, sid, "body_tracking").ValueJSON)["pregnancy"])
	if !reflect.DeepEqual(first["modeled_birth_time"], second["modeled_birth_time"]) {
		t.Fatal("reconfirmation moved gestation date in persistent state")
	}
	save(3, "pregnancy_ended", "ending", "1423-01-20", "1423-01-20")
	save(4, "period_start", "period-after", "1423-02-03", "1423-02-03")
	save(5, "period_start", "period-after", "1423-02-04", "1423-02-01")
	if storyTime46Map(storyTime46JSON(t, storyTime46Current(t, st, sid, "body_tracking").ValueJSON)["cycle_reference"])["date"] != "1423-02-01" {
		t.Fatal("dated correction was discarded in persistent path")
	}
	save(6, "conception_exposure", "new-exposure", "1423-02-14", "1423-02-14")
	current := storyTime46Current(t, st, sid, "body_tracking")
	result := storyTime46Map(storyTime46JSON(t, current.EvidenceJSON)["model_result"])
	if storyTime46Map(storyTime46Map(result["configuration"])["cycle_reference"])["date"] != "1423-02-01" {
		t.Fatal("persisted history used superseded period date")
	}
	if storyTime46JSON(t, current.ValueJSON)["modeled_pregnancy"] == nil {
		t.Fatalf("new pregnancy fixture failed: %v", result)
	}
	// An unchanged mention is not another event or random trial, including after restart.
	routes, provider, endpoint = storyTime46Server(t, st)
	save(7, "conception_exposure", "new-exposure", "1423-03-01", "1423-02-14")
	if after := storyTime46Current(t, st, sid, "body_tracking"); after.ValueJSON != current.ValueJSON {
		t.Fatal("unchanged mention changed persisted model")
	}
	bodyTracking46History(t, st, sid, entity, 6)
	view := storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+sid, nil)
	estimates := view["cycle_estimates"].([]any)
	if len(estimates) != 1 || storyTime46Map(storyTime46Map(estimates[0])["estimate"])["status"] != "unknown" {
		t.Fatalf("unexpected cycle reading: %v", estimates)
	}
	raw, _ := json.Marshal(estimates)
	if strings.Contains(string(raw), "next_period_estimate") {
		t.Fatalf("cycle forecast remained during new pregnancy: %s", raw)
	}
	models := view["model_readings"].([]any)
	if len(models) != 1 || storyTime46Map(storyTime46Map(models[0])["reading"])["stage"] != "modeled_implanted_pregnancy" {
		t.Fatalf("new pregnancy reading missing: %v", models)
	}
}
