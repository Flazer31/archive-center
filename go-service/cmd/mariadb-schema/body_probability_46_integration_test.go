package main

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

func bodyProbability46Config(t *testing.T, routes http.Handler, st archiveStore.Store, sid, entity string, peak float64) {
	t.Helper()
	identity := archiveStore.EntityIdentity{StableEntityID: entity, ChatSessionID: sid, IdentityNamespace: "fictional", EntityKind: "character", CanonicalLabel: "Mina", LifecycleState: "active", ReviewState: "reviewed", SourceContract: "manual", IdempotencyKey: "probability-fixture-" + sid, MappingRevision: 1}
	if err := st.(archiveStore.EntityIdentityWriter).SaveEntityIdentity(context.Background(), &identity); err != nil {
		t.Fatal(err)
	}
	bodyTracking46Female(t, st, sid)
	// Explicit author parameters make the two outcome controls deterministic;
	// they do not replace the shipped editable fiction template defaults.
	config := map[string]any{"cycle_tracking_enabled": true, "automatic_pregnancy_enabled": true, "simulation_seed": "disposable-probability-seed", "characters": []any{map[string]any{
		"entity_id": entity, "origin_entity_id": entity, "character_name": "Mina", "cycle_viability": 1, "conditional_peak": peak,
		"cycle": map[string]any{"reference_time": map[string]any{"date": "1423-01-01"}, "reference_kind": "author_setting", "cycle_days": 28, "variation_days": 0, "period_days": 5, "luteal_min_days": 14, "luteal_max_days": 14},
	}}}
	storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+sid, map[string]any{"restore_snapshot": map[string]any{"contract_version": "body_tracking_settings.v1", "config": config}})
}

func bodyProbability46Extraction(entity, key, date string) (string, map[string]any) {
	text, extraction := bodyTracking46Extraction(entity, "conception_exposure", key, date)
	observation := storyTime46Map(extraction["body_events"].([]any)[0])
	observation["exposure"] = map[string]any{"classification": "potentially_conceiving", "partner_compatibility": "compatible", "contraception": "none", "model_profile": "author_allowed"}
	return text, extraction
}

func bodyProbability46Reindex(t *testing.T, routes http.Handler, sid, endpoint string) {
	t.Helper()
	result := storyTime46Request(t, routes, http.MethodPost, "/admin/reindex", map[string]any{"chat_session_id": sid, "force": true, "client_meta": map[string]any{"embedding": map[string]any{"provider": "openai", "api_key": "disposable-local-test", "endpoint": endpoint + "/embeddings", "model": "local-test-embedding"}}})
	if result["status"] != "ok" {
		t.Fatalf("canonical private source replay failed: %#v", result)
	}
}

func TestBodyProbability46HTTPMariaDBFrozenDrawReplayBranchAndReroll(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid, branch, entity = "body-probability", "body-probability-branch", "probability-mina"
	bodyProbability46Config(t, routes, st, sid, entity, 0)
	text, extraction := bodyTracking46Extraction(entity, "period_start", "probability-reference", "1423-01-01")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"period-input"}, []string{"Record the reference."}, text, extraction)
	text, extraction = bodyProbability46Extraction(entity, "exposure-one", "1423-01-14")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 2, 2000, []string{"exposure-input"}, []string{"Record the exposure."}, text, extraction)
	first := storyTime46Current(t, st, sid, "body_tracking")
	firstValue, firstEvidence := storyTime46JSON(t, first.ValueJSON), storyTime46JSON(t, first.EvidenceJSON)
	firstModel := storyTime46Map(firstEvidence["model_result"])
	if firstModel["status"] != "evaluated" || firstModel["outcome"] != "no_modeled_implantation_from_this_day" || firstValue["pregnancy"] != nil || firstValue["modeled_pregnancy"] != nil {
		t.Fatalf("explicit zero-peak control changed outcome: %#v %#v", firstModel, firstValue)
	}
	decisions, ok := firstModel["cycles"].([]any)
	if !ok || len(decisions) == 0 || storyTime46Map(storyTime46Map(decisions[0])["day_draw"])["token"] == "" || firstEvidence["model_cycles"] == nil {
		t.Fatalf("event lacks its persisted immutable draw/checkpoint: %#v", firstEvidence)
	}
	// Retry, process restart and read-only settings delivery
	// cannot resample or change the original event's model/parameters.
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 2, 2000, []string{"exposure-input"}, []string{"Record the exposure."}, text, extraction)
	routes, provider, endpoint = storyTime46Server(t, st)
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 2, 2000, []string{"exposure-input"}, []string{"Record the exposure."}, text, extraction)
	storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+sid, nil)
	bodyTracking46History(t, st, sid, entity, 2)
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.ValueJSON != first.ValueJSON || got.EvidenceJSON != first.EvidenceJSON {
		t.Fatal("retry/read changed the persisted sample")
	}
	text, extraction = bodyProbability46Extraction(entity, "exposure-same-day", "1423-01-14")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 3, 3000, []string{"same-day-input"}, []string{"Record another observation on that day."}, text, extraction)
	second := storyTime46Current(t, st, sid, "body_tracking")
	secondEvidence := storyTime46JSON(t, second.EvidenceJSON)
	if !reflect.DeepEqual(storyTime46Map(secondEvidence["model_result"])["cycles"], firstModel["cycles"]) || !reflect.DeepEqual(secondEvidence["model_cycles"], firstEvidence["model_cycles"]) {
		t.Fatal("distinct mention on the same subject/day obtained another probability trial")
	}
	// Fresh normal admission supplies private projection authority: branch
	// first, before any administrative replay or reindex.
	copyResult := storyTime46Request(t, routes, http.MethodPost, "/sessions/migrate-complete", map[string]any{"source_session_id": sid, "target_session_id": branch, "mode": archiveStore.SessionMigrationModeCopyKeepSource})
	if copyResult["blocked"] == true || copyResult["write_attempted"] != true {
		t.Fatalf("probability branch failed: %#v", copyResult)
	}
	branchCurrent := storyTime46Current(t, st, branch, "body_tracking")
	branchEvidence := storyTime46JSON(t, branchCurrent.EvidenceJSON)
	if branchCurrent.OwnerID == entity || !reflect.DeepEqual(branchEvidence["model_result"], secondEvidence["model_result"]) || !reflect.DeepEqual(branchEvidence["model_cycles"], firstEvidence["model_cycles"]) {
		t.Fatal("branch changed the frozen model outcome or missed new owner")
	}
	bodyProbability46Reindex(t, routes, sid, endpoint)
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.ValueJSON != second.ValueJSON || got.EvidenceJSON != second.EvidenceJSON {
		t.Fatal("canonical source replay changed the persisted probability result")
	}
	branchExport := storyTime46Request(t, routes, http.MethodGet, "/sessions/"+branch+"/export", nil)
	branchConfig := storyTime46Map(storyTime46Map(branchExport["body_tracking_settings"])["config"])
	character := storyTime46Map(branchConfig["characters"].([]any)[0])
	character["conditional_peak"] = float64(1)
	storyTime46Request(t, routes, http.MethodPut, "/config/body-tracking/"+branch, branchConfig)
	text, extraction = bodyProbability46Extraction(branchCurrent.OwnerID, "branch-next-day", "1423-01-15")
	bodyTracking46Complete(t, routes, provider, endpoint, branch, 4, 4000, []string{"branch-new-exposure"}, []string{"Record the next day."}, text, extraction)
	branchNext := storyTime46Current(t, st, branch, "body_tracking")
	branchNextEvidence := storyTime46JSON(t, branchNext.EvidenceJSON)
	if !reflect.DeepEqual(branchNextEvidence["model_cycles"], firstEvidence["model_cycles"]) || storyTime46Map(branchNextEvidence["model_result"])["outcome"] != "no_modeled_implantation_from_this_day" {
		t.Fatal("editing model settings rewrote the already frozen cycle parameters")
	}
	// Parent reroll invalidates its old observation but leaves the copied result
	// intact; rollback restores the exact first surviving event and draw.
	text, extraction = bodyProbability46Extraction(entity, "replacement-exposure", "1423-01-16")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 4, 5000, []string{"same-day-input"}, []string{"Record another observation on that day."}, text, extraction)
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.SourceTurn != 3 {
		t.Fatalf("probability reroll created a different logical turn: %+v", got)
	}
	storyTime46Request(t, routes, http.MethodDelete, "/rollback/3?chat_session_id="+sid+"&req_source=timeline_manual_delete", nil)
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.ValueJSON != first.ValueJSON || got.EvidenceJSON != first.EvidenceJSON {
		t.Fatal("rollback failed to restore exact surviving probability result")
	}
	if got := storyTime46Current(t, st, branch, "body_tracking"); got.ValueJSON != branchNext.ValueJSON {
		t.Fatal("parent reroll/rollback changed branch probability outcome")
	}
}

func TestBodyProbability46HTTPMariaDBLatentSuccessIsNotObservedPregnancy(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid, entity = "body-probability-success", "probability-success-mina"
	bodyProbability46Config(t, routes, st, sid, entity, 1)
	text, extraction := bodyProbability46Extraction(entity, "certain-fiction-control", "1423-01-14")
	bodyTracking46Complete(t, routes, provider, endpoint, sid, 1, 1000, []string{"success-input"}, []string{"Record the exposure."}, text, extraction)
	current := storyTime46Current(t, st, sid, "body_tracking")
	payload, evidence := storyTime46JSON(t, current.ValueJSON), storyTime46JSON(t, current.EvidenceJSON)
	model := storyTime46Map(evidence["model_result"])
	if model["outcome"] != "latent_model_implantation" || payload["modeled_pregnancy"] == nil || payload["pregnancy"] != nil || model["knowledge"] != "not_inferred" || model["symptoms"] != "not_inferred" {
		t.Fatalf("latent result conflated with observed pregnancy/knowledge: %#v %#v", model, payload)
	}
	storyTime46Request(t, routes, http.MethodGet, "/config/body-tracking/"+sid, nil)
	bodyTracking46History(t, st, sid, entity, 1)
	if got := storyTime46Current(t, st, sid, "body_tracking"); got.ValueJSON != current.ValueJSON {
		t.Fatal("reading latent pregnancy mutated its persisted result")
	}
}
