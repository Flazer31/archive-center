package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func save46LifecycleFixture(t *testing.T, st *turnRecordingStore, turn int, extraction map[string]any, content string) {
	t.Helper()
	srv := &Server{Store: st}
	result := artifactSaveResult{}
	now := time.Unix(int64(turn), 0)
	srv.saveNarrativeStateFromExtraction(context.Background(), "lifecycle-46", turn, extraction, content, nil, now, &result)
	srv.saveCharacterAndStateArtifacts(context.Background(), "lifecycle-46", turn, extraction, content, completeTurnEmbeddingConfig{}, now, &result, nil, &canonicalStateWriteCostMeasurement{})
	if result.Errors != 0 {
		t.Fatalf("production lifecycle save failed: %#v", result.ErrorDetails)
	}
	// Materialize only the external store boundary's recorded pending writes.
	for _, saved := range st.savedPendingThreads {
		found := false
		for i := range st.returnPendingThreads {
			if st.returnPendingThreads[i].ThreadKey == saved.ThreadKey {
				st.returnPendingThreads[i] = *saved
				found = true
				break
			}
		}
		if !found {
			st.returnPendingThreads = append(st.returnPendingThreads, *saved)
		}
	}
}

func Test46ResolvedOnlySynchronizesCurrentAndLaterMentionProjections(t *testing.T) {
	st := &turnRecordingStore{}
	key := "museum-repair-occurrence-one"
	pending := func(title string) map[string]any {
		return map[string]any{"pending_threads": []any{map[string]any{"title": title, "lifecycle_key": key, "confidence": .9}}}
	}
	save46LifecycleFixture(t, st, 1, pending("Repair the museum"), "The team promises to repair the museum.")
	if len(st.savedStatusEvents) != 1 {
		t.Fatalf("creation did not retain restorable history: %d", len(st.savedStatusEvents))
	}
	save46LifecycleFixture(t, st, 2, map[string]any{"resolved_threads": []any{map[string]any{"lifecycle_key": key, "resolution_note": "All repair work finished."}}}, "All repair work finished.")
	if len(st.returnStatusCurrent) != 1 || len(st.savedStatusEvents) != 2 {
		t.Fatalf("resolved-only input did not create one authoritative transition: current=%d history=%d", len(st.returnStatusCurrent), len(st.savedStatusEvents))
	}
	payload := parseJSONMap(st.returnStatusCurrent[0].ValueJSON)
	if payload["transition"] != "resolve" || payload["value"] != "All repair work finished." {
		t.Fatalf("resolution meaning lost: %#v", payload)
	}
	assertClosed := func() {
		t.Helper()
		if got := st.savedPendingThreads[len(st.savedPendingThreads)-1]; got.Status != "resolved" || got.CreatedTurn != 1 || got.ResolvedTurn != 2 {
			t.Fatalf("pending is not the same completed occurrence: %#v", got)
		}
		if got := parseJSONMap(st.savedActiveStates[len(st.savedActiveStates)-1].Content); got["status"] != "resolved" {
			t.Fatalf("active projection reopened: %#v", got)
		}
		if got := parseJSONMap(st.savedCanonicalLayers[len(st.savedCanonicalLayers)-1].Content); got["status"] != "resolved" {
			t.Fatalf("canonical projection reopened: %#v", got)
		}
		if got := st.savedStorylines[len(st.savedStorylines)-1].Status; got != "resolved" {
			t.Fatalf("storyline projection reopened: %s", got)
		}
	}
	assertClosed()
	projectionCounts := []int{len(st.savedPendingThreads), len(st.savedActiveStates), len(st.savedCanonicalLayers), len(st.savedStorylines)}
	save46LifecycleFixture(t, st, 9, pending("Remember the restored museum"), "They remember the old repair promise.")
	assertClosed()
	for i, count := range []int{len(st.savedPendingThreads), len(st.savedActiveStates), len(st.savedCanonicalLayers), len(st.savedStorylines)} {
		if count != projectionCounts[i] {
			t.Fatalf("historical mention rematerialized current projection %d: before=%d after=%d", i, projectionCounts[i], count)
		}
	}
	if len(st.savedStatusEvents) != 2 {
		t.Fatalf("mere mention created a new transition: %d", len(st.savedStatusEvents))
	}
	oldArtifact := mustCompactJSON(map[string]any{"lifecycle_key": key, "title": "Remember the restored museum"})
	if !narrativeCurrentStateSupersedesOpenArtifact(st.returnStatusCurrent, 9, oldArtifact) {
		t.Fatal("later same-key open representation defeated the authoritative completion")
	}
}

func TestStorylineLifecycleRetainsExplicitPendingStatus(t *testing.T) {
	for _, tc := range []struct {
		name, status, transition, want string
	}{
		{"resolved_status", "resolved", "", "resolved"},
		{"paused_status", "paused", "", "paused"},
		{"explicit_progress_wins", "resolved", "partial", "active"},
		{"explicit_completion", "open", "complete", "resolved"},
		{"unknown_status_is_not_completion", "unknown", "", "active"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &turnRecordingStore{}
			key := "shield-handover-one"
			initial := map[string]any{"title": "Collect the shield", "description": "The shield is ready for collection.", "lifecycle_key": key, "status": "open"}
			save46LifecycleFixture(t, st, 1, normalizeCriticExtraction(map[string]any{"pending_threads": []any{initial}}), "The shield is ready for collection.")
			update := map[string]any{"title": "Shield handover", "description": "The shield handover is recorded.", "lifecycle_key": key, "status": tc.status, "transition": tc.transition}
			save46LifecycleFixture(t, st, 2, normalizeCriticExtraction(map[string]any{"pending_threads": []any{update}}), "The shield handover is recorded.")
			got := st.savedStorylines[len(st.savedStorylines)-1]
			if got.Status != tc.want {
				t.Fatalf("explicit model decision lost: status=%s transition=%s storyline=%s; want=%s", tc.status, tc.transition, got.Status, tc.want)
			}
			if len(st.returnPendingThreads) != 1 || len(st.returnStatusCurrent) != 1 || got.FirstTurn != 1 {
				t.Fatalf("renamed occurrence duplicated or lost origin: pending=%+v current=%+v storyline=%+v", st.returnPendingThreads, st.returnStatusCurrent, got)
			}
			if tc.want == "resolved" {
				count := len(st.savedStatusEvents)
				save46LifecycleFixture(t, st, 2, normalizeCriticExtraction(map[string]any{"pending_threads": []any{update}}), "The shield handover is recorded.")
				save46LifecycleFixture(t, st, 3, normalizeCriticExtraction(map[string]any{"pending_threads": []any{initial}}), "They remember collecting the shield.")
				if len(st.savedStatusEvents) != count || st.returnPendingThreads[0].Status != "resolved" || st.savedStorylines[len(st.savedStorylines)-1].Status != "resolved" {
					t.Fatal("replay or historical mention duplicated/reopened the completed occurrence")
				}
			}
		})
	}
}

func TestStorylineLifecyclePreservesIndependentDescription(t *testing.T) {
	st := &turnRecordingStore{}
	description := "Deliver both crates to Mira; handing over only one does not complete the promise."
	save46LifecycleFixture(t, st, 1, normalizeCriticExtraction(map[string]any{"pending_threads": []any{map[string]any{
		"title": "Workshop delivery", "description": description, "lifecycle_key": "workshop-crates", "status": "open",
	}}}), description)
	if len(st.returnPendingThreads) != 1 || st.returnPendingThreads[0].Description != description {
		t.Fatalf("fulfillment terms lost in current pending projection: %+v", st.returnPendingThreads)
	}
	if len(st.savedStorylines) != 1 || st.savedStorylines[0].CurrentContext != description {
		t.Fatalf("fulfillment terms lost in Storyline projection: %+v", st.savedStorylines)
	}
}

// Only provider and persistence boundaries are substituted. Cold start, Critic
// parsing, source admission, lifecycle projection and the UI's GET are real.
type storylineColdStartStore struct {
	*canonicalRawReplaySessionNormalizeStore
	store.StatusSchemaRegistryStore
	store.StatusCurrentValueStore
	store.StatusLifecycleStore
	store.ReversibleStatusTransitionStore
	projection *turnRecordingStore
}

func (st *storylineColdStartStore) SaveStoryline(ctx context.Context, item *store.Storyline) error {
	if err := st.projection.SaveStoryline(ctx, item); err != nil {
		return err
	}
	key := stringFromMap(parseJSONMap(item.OngoingTensionsJSON), "lifecycle_key")
	if key == "" {
		return fmt.Errorf("fixture expected an occurrence key")
	}
	copyItem := *item
	for i, previous := range st.projection.returnStorylines {
		if stringFromMap(parseJSONMap(previous.OngoingTensionsJSON), "lifecycle_key") == key {
			copyItem.ID = previous.ID
			st.projection.returnStorylines[i] = copyItem
			return nil
		}
	}
	copyItem.ID = int64(len(st.projection.returnStorylines) + 1)
	st.projection.returnStorylines = append(st.projection.returnStorylines, copyItem)
	return nil
}

func TestStorylineLifecycleColdStartPreservesDecisions(t *testing.T) {
	const sid = "storyline-cold-start"
	initial := []any{}
	updates := []any{}
	wants := map[string]string{"quest-selection": "resolved", "shield-handover": "resolved", "greaves-fitting": "paused", "relic-recovery": "active"}
	for _, key := range []string{"quest-selection", "shield-handover", "greaves-fitting", "relic-recovery"} {
		initial = append(initial, map[string]any{"title": key, "lifecycle_key": key, "status": "open"})
		if wants[key] != "active" {
			updates = append(updates, map[string]any{"title": key + " update", "lifecycle_key": key, "status": wants[key]})
		}
	}
	contents := []string{"The group selects tasks: quest selection, shield handover, greaves fitting and relic recovery.", "The quest is accepted and the shield collected. Greaves fitting is paused; relic recovery continues."}
	logs := []store.ChatLog{}
	for i, content := range contents {
		logs = append(logs, store.ChatLog{ChatSessionID: sid, TurnIndex: i + 1, Role: "user", Content: "Proceed with the tasks."}, store.ChatLog{ChatSessionID: sid, TurnIndex: i + 1, Role: "assistant", Content: content})
	}
	projection := &turnRecordingStore{}
	fake := &storylineColdStartStore{
		canonicalRawReplaySessionNormalizeStore: &canonicalRawReplaySessionNormalizeStore{
			memoryAdmissionWorkerStore: &memoryAdmissionWorkerStore{Store: projection, logs: logs},
			sources:                    map[string]*store.MemorySourceRevision{},
		},
		StatusSchemaRegistryStore: projection, StatusCurrentValueStore: projection,
		StatusLifecycleStore: projection, ReversibleStatusTransitionStore: projection, projection: projection,
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store, srv.StoreOpenError = fake, nil
	oldClient := proxyHTTPClient
	calls := 0
	responses := []string{
		criticWireJSONForTest(map[string]any{"turn_summary": contents[0], "importance_score": 6, "pending_threads": initial}),
		criticWireJSONForTest(map[string]any{"turn_summary": contents[1], "importance_score": 6, "pending_threads": updates}),
	}
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.example.com" || !strings.HasSuffix(r.URL.Path, "/chat/completions") || calls >= len(responses) {
			t.Fatalf("unexpected provider call: %s, calls=%d", r.URL, calls)
		}
		request, err := io.ReadAll(r.Body)
		if err != nil || !strings.Contains(string(request), contents[calls]) {
			t.Fatalf("cold start did not replay the expected source: %s, %v", request, err)
		}
		body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": responses[calls]}}}})
		calls++
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	t.Cleanup(func() { proxyHTTPClient = oldClient })
	result, err := srv.runAdminSessionNormalize(context.Background(), sid, adminSessionNormalizeRequest{
		SkipRepair: true, SkipReindex: true,
		ClientMeta: map[string]any{"critic": map[string]any{"api_key": "synthetic-key", "endpoint": "https://api.example.com/v1", "model": "critic", "provider": "openai", "timeout_ms": 45000}},
	}, nil)
	if err != nil || result["status"] != "ok" || calls != len(contents) || len(fake.admissions) != len(contents) || len(fake.enqueuedJobs) != 0 {
		t.Fatalf("cold start failed: result=%+v error=%v calls=%d admissions=%d queued=%d", result, err, calls, len(fake.admissions), len(fake.enqueuedJobs))
	}
	req := httptest.NewRequest(http.MethodGet, "/storylines/"+sid, nil)
	req.SetPathValue("chat_session_id", sid)
	rec := httptest.NewRecorder()
	srv.handleStorylinesGet(rec, req)
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("Storylines response: %d %s %v", rec.Code, rec.Body.String(), err)
	}
	items := sliceFromAny(payload["storylines"])
	if len(items) != len(wants) {
		t.Fatalf("unexpected Storyline count: %s", rec.Body.String())
	}
	for _, raw := range items {
		item := mapFromAny(raw)
		key := strings.TrimSuffix(stringFromMap(item, "name"), " update")
		if want, ok := wants[key]; !ok || stringFromMap(item, "status") != want {
			t.Fatalf("cold start lost explicit decision: %s want=%s got=%v", key, want, item["status"])
		}
	}
}

func Test46LifecycleProgressPauseResumeAndRecurringOccurrences(t *testing.T) {
	st := &turnRecordingStore{}
	key := "delivery-week-1"
	save46LifecycleFixture(t, st, 1, map[string]any{"pending_threads": []any{map[string]any{
		"title": "Deliver the weekly supplies", "lifecycle_key": key, "series_key": "weekly-supplies", "confidence": .9,
	}}}, "The weekly delivery was promised.")
	phase := func(turn int, transition, remaining string) {
		t.Helper()
		excerpt := fmt.Sprintf("Delivery evidence %d: %s.", turn, remaining)
		save46LifecycleFixture(t, st, turn, map[string]any{"state_claims": []any{map[string]any{
			"subject": "Weekly supplies", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": key,
			"value": "Weekly supply delivery", "transition": transition, "confidence": .9,
			"remaining_obligations": remaining, "evidence_excerpt": excerpt,
		}}}, "The narrator reviews the delivery. "+excerpt+" The team records the update.")
	}
	phase(2, "partial", "two boxes remain")
	phase(3, "partial", "one box remains")
	if len(st.savedStatusEvents) != 3 || st.returnPendingThreads[0].Status != "open" {
		t.Fatalf("same-worded partial progress lost or completed obligation: history=%d pending=%#v", len(st.savedStatusEvents), st.returnPendingThreads)
	}
	if got := stringFromMap(mapFromAny(parseJSONMap(st.returnStatusCurrent[0].ValueJSON)["lifecycle_details"]), "remaining_obligations"); got != "one box remains" {
		t.Fatalf("remaining obligation lost: %q", got)
	}
	phase(4, "pause", "one box remains")
	if st.returnPendingThreads[0].Status != "paused" {
		t.Fatal("pause was collapsed to completion")
	}
	phase(5, "resume", "one box remains")
	if st.returnPendingThreads[0].Status != "open" || st.returnPendingThreads[0].CreatedTurn != 1 {
		t.Fatal("same-value resume did not continue the original occurrence")
	}
	phase(6, "complete", "none remain")
	phase(7, "complete", "none remain")
	if len(st.savedStatusEvents) != 6 {
		t.Fatalf("same completed value reaffirmation created a second completion: %d", len(st.savedStatusEvents))
	}
	save46LifecycleFixture(t, st, 8, map[string]any{"pending_threads": []any{map[string]any{
		"title": "Deliver the weekly supplies", "lifecycle_key": "delivery-week-2", "series_key": "weekly-supplies", "confidence": .9,
	}}}, "Next week's delivery was promised.")
	if len(st.returnStatusCurrent) != 2 || len(st.returnPendingThreads) != 2 {
		t.Fatalf("new recurrence merged with completed occurrence: current=%d pending=%d", len(st.returnStatusCurrent), len(st.returnPendingThreads))
	}
	if st.returnPendingThreads[0].Status != "resolved" || st.returnPendingThreads[1].Status != "open" || st.returnPendingThreads[1].CreatedTurn != 8 {
		t.Fatalf("recurrence states are not independent: %#v", st.returnPendingThreads)
	}
}

func Test46LifecycleSourceUnitsPreserveMultipleSameTurnPhasesAndReplay(t *testing.T) {
	st := &turnRecordingStore{}
	srv := &Server{Store: st}
	ctx := context.WithValue(context.Background(), entityIdentitySourceContextKey{}, entityIdentitySourceContext{
		ContractVersion: completeTurnSourceAcceptanceContract, Revision: "accepted-source-46",
	})
	claims := []any{}
	for _, phase := range []string{"partial", "complete"} {
		claims = append(claims, map[string]any{"subject": "Canal repairs", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": "canal-repairs", "value": "Repair account", "transition": phase, "confidence": .9, "evidence_excerpt": "The canal repairs were partially finished, then completed."})
	}
	result := artifactSaveResult{}
	extraction := map[string]any{"state_claims": claims}
	for i := 0; i < 2; i++ {
		srv.saveNarrativeStateFromExtraction(ctx, "lifecycle-46", 5, extraction, "The foreman described the work. The canal repairs were partially finished, then completed. Everyone checked the outcome.", nil, time.Unix(5, 0), &result)
	}
	if len(st.savedStatusEvents) != 2 || len(st.returnStatusCurrent) != 1 {
		t.Fatalf("phase or source replay identity collapsed transitions: events=%d current=%d", len(st.savedStatusEvents), len(st.returnStatusCurrent))
	}
	if got := stringFromMap(parseJSONMap(st.returnStatusCurrent[0].ValueJSON), "transition"); got != "complete" {
		t.Fatalf("final confirmed phase is %q", got)
	}
}

func Test46LegacyResolutionDoesNotInventPromiseOrigin(t *testing.T) {
	st := &turnRecordingStore{}
	save46LifecycleFixture(t, st, 12, map[string]any{"resolved_threads": []any{map[string]any{
		"lifecycle_key": "origin-unknown", "resolution_note": "The balance was settled.",
	}}}, "The balance was settled.")
	if len(st.returnStatusCurrent) != 1 || len(st.savedStatusEvents) != 1 {
		t.Fatal("resolution lacking redundant state_claim or excerpt was not preserved")
	}
	payload := parseJSONMap(st.returnStatusCurrent[0].ValueJSON)
	if payload["subject"] != "" || payload["pending_thread"] != nil || len(st.savedPendingThreads) != 0 {
		t.Fatalf("unknown origin gained a fabricated title/creation: %#v", payload)
	}
	if len(narrativeCurrentStateViews(st.returnStatusCurrent)) != 1 {
		t.Fatal("known lifecycle resolution disappeared because original title is unknown")
	}
	if stringFromMap(parseJSONMap(st.returnStatusCurrent[0].EvidenceJSON), "evidence_excerpt") != "" {
		t.Fatal("legacy resolution manufactured an exact quotation")
	}
	paired := &turnRecordingStore{}
	save46LifecycleFixture(t, paired, 12, map[string]any{
		"state_claims":     []any{map[string]any{"subject": "Balance", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": "origin-unknown", "value": "settled", "transition": "complete", "confidence": .9, "evidence_excerpt": "The balance was settled."}},
		"resolved_threads": []any{map[string]any{"lifecycle_key": "origin-unknown"}},
	}, "They opened the book. The balance was settled. They left the bank.")
	if len(paired.returnStatusCurrent) != 1 || parseJSONMap(paired.returnStatusCurrent[0].ValueJSON)["pending_thread"] != nil || len(paired.savedPendingThreads) != 0 {
		t.Fatalf("redundant completion surfaces fabricated a creation: current=%#v pending=%#v", paired.returnStatusCurrent, paired.savedPendingThreads)
	}
	lowConfidence := &turnRecordingStore{}
	save46LifecycleFixture(t, lowConfidence, 12, map[string]any{
		"state_claims":     []any{map[string]any{"subject": "Balance", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": "origin-unknown", "value": "possibly settled", "transition": "complete", "confidence": .2, "evidence_excerpt": "The balance was settled."}},
		"resolved_threads": []any{map[string]any{"lifecycle_key": "origin-unknown", "resolution_note": "The balance was settled."}},
	}, "They opened the book. The balance was settled. They left the bank.")
	if len(lowConfidence.returnStatusCurrent) != 1 || stringFromMap(parseJSONMap(lowConfidence.returnStatusCurrent[0].ValueJSON), "value") != "The balance was settled." {
		t.Fatal("independently accepted legacy resolution inherited a redundant state_claim confidence requirement")
	}
}

type lifecycle46RestoreStore struct {
	*turnRecordingStore
	activeEvents []store.StatusChangeEvent
	activeReads  int
}

func (st *lifecycle46RestoreStore) ListLatestReversibleCurrentProjectionEvents(_ context.Context, sid string, keys []string) ([]store.StatusChangeEvent, error) {
	if sid != "restore-46" || len(keys) != 1 || keys[0] != narrativeStateStatusKey {
		return nil, fmt.Errorf("unexpected source-active restoration request: %s %v", sid, keys)
	}
	st.activeReads++
	return st.activeEvents, nil
}

func Test46RestoreLegacyHistoryDoesNotResurrectInactiveSource(t *testing.T) {
	claim := narrativeStateClaim{Subject: "Museum", SubjectType: "entity", StateSlot: "goal_status", ClaimScope: "objective", Value: "open"}
	legacy := store.StatusChangeEvent{ID: 1, ChatSessionID: "restore-46", RegistryID: 1, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: narrativeStateOwnerID(claim), EventKind: "set", NewValueJSON: mustCompactJSON(narrativeStateValuePayload(claim, "", 2)), EvidenceJSON: `{}`, SourceTurn: 2}
	inactive := legacy
	inactive.ID, inactive.SourceTurn, inactive.EvidenceJSON = 2, 3, `{"source_revision":"inactive-source","current_projection":true}`
	claim.Value = "closed by deleted branch"
	inactive.NewValueJSON = mustCompactJSON(narrativeStateValuePayload(claim, "open", 3))
	st := &lifecycle46RestoreStore{turnRecordingStore: &turnRecordingStore{savedStatusEvents: []store.StatusChangeEvent{legacy, inactive}}}
	if count, err := restoreNarrativeCurrentStatesAfterRollback(context.Background(), st, "restore-46", 4); err != nil || count != 1 {
		t.Fatalf("restore count=%d err=%v", count, err)
	}
	if st.activeReads != 1 || len(st.returnStatusCurrent) != 1 || st.returnStatusCurrent[0].SourceTurn != legacy.SourceTurn {
		t.Fatalf("inactive source event restored through legacy path: reads=%d current=%#v", st.activeReads, st.returnStatusCurrent)
	}
}

func Test46RestoreLegacyCompletionRestoresOriginalPendingOccurrence(t *testing.T) {
	st := &turnRecordingStore{}
	save46LifecycleFixture(t, st, 1, map[string]any{"pending_threads": []any{map[string]any{"title": "Repair archive", "lifecycle_key": "archive-repair"}}}, "Repair was promised.")
	save46LifecycleFixture(t, st, 2, map[string]any{"resolved_threads": []any{map[string]any{"lifecycle_key": "archive-repair", "resolution_note": "Repair complete."}}}, "Repair complete.")
	st.returnStatusCurrent = nil
	st.returnPendingThreads = nil
	st.savedPendingThreads = nil
	if count, err := restoreNarrativeCurrentStatesAfterRollback(context.Background(), st, "lifecycle-46", 1); err != nil || count != 1 {
		t.Fatalf("legacy rollback count=%d err=%v", count, err)
	}
	if len(st.savedPendingThreads) != 1 || st.savedPendingThreads[0].Status != "open" || st.savedPendingThreads[0].CreatedTurn != 1 || st.savedPendingThreads[0].SourceTurn != 1 || st.savedPendingThreads[0].ResolvedTurn != 0 {
		t.Fatalf("legacy rollback lost original pending occurrence: %#v", st.savedPendingThreads)
	}
	if len(st.returnStatusCurrent) != 1 || st.returnStatusCurrent[0].SourceTurn != 1 {
		t.Fatalf("legacy current and pending were not restored together: %#v", st.returnStatusCurrent)
	}
}

type lifecycle46AtomicFailureStore struct {
	*turnRecordingStore
	atomicAttempts int
}

func (st *lifecycle46AtomicFailureStore) ApplyReversibleStatusTransition(_ context.Context, transition store.ReversibleStatusTransition) (store.ReversibleStatusTransitionResult, error) {
	if transition.Event.StatusKey != narrativeStateStatusKey || transition.CurrentValue == nil || narrativePendingSnapshot(parseJSONMap(transition.CurrentValue.ValueJSON)) == nil {
		return store.ReversibleStatusTransitionResult{}, fmt.Errorf("unexpected atomic write: %#v", transition)
	}
	st.atomicAttempts++
	return store.ReversibleStatusTransitionResult{}, fmt.Errorf("injected atomic lifecycle store failure")
}

func Test46AtomicLifecycleFailureHasNoIndependentPendingWriter(t *testing.T) {
	st := &lifecycle46AtomicFailureStore{turnRecordingStore: &turnRecordingStore{}}
	srv := &Server{Store: st}
	ctx := context.WithValue(context.Background(), entityIdentitySourceContextKey{}, entityIdentitySourceContext{
		ContractVersion: completeTurnSourceAcceptanceContract, Revision: "accepted-failing-source-46",
	})
	result := srv.saveCriticExtractionArtifacts(ctx, "lifecycle-46", 1, map[string]any{
		"turn_summary":    "The caravan accepted the delivery promise.",
		"pending_threads": []any{map[string]any{"title": "Caravan delivery", "lifecycle_key": "caravan-delivery", "confidence": .9}},
		"state_claims":    []any{map[string]any{"subject": "Caravan delivery", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": "caravan-delivery", "value": "delivery promised", "transition": "set", "confidence": .9, "evidence_excerpt": "The caravan accepted the delivery promise."}},
	}, "The captain stood. The caravan accepted the delivery promise. They departed.", completeTurnEmbeddingConfig{}, time.Unix(1, 0))
	if st.atomicAttempts != 1 || result.Errors == 0 {
		t.Fatalf("atomic failure was not observed: attempts=%d result=%#v", st.atomicAttempts, result)
	}
	if len(st.savedPendingThreads) != 0 || len(st.savedStatusCurrent) != 0 || len(st.savedStatusEvents) != 0 || len(st.savedStorylines) != 0 {
		t.Fatalf("atomic failure escaped through independent projection writer: pending=%d current=%d history=%d storyline=%d", len(st.savedPendingThreads), len(st.savedStatusCurrent), len(st.savedStatusEvents), len(st.savedStorylines))
	}
	if result.Memories == 0 {
		t.Fatal("lifecycle projection failure blocked ordinary memory persistence")
	}
}

func Test46UpgradeCompletionPreservesObservedPreviousPendingSnapshot(t *testing.T) {
	for _, withCurrent := range []bool{false, true} {
		t.Run(fmt.Sprint(withCurrent), func(t *testing.T) {
			thread := store.PendingThread{ID: 9, ChatSessionID: "lifecycle-46", ThreadKey: narrativeLifecycleStorageKey("prior-release-promise"), Title: "Prior release promise", Description: "Deliver instruments", Status: "open", CreatedTurn: 2, SourceTurn: 4, HookMetadataJSON: `{"lifecycle_key":"prior-release-promise","title":"Prior release promise"}`}
			st := &turnRecordingStore{returnPendingThreads: []store.PendingThread{thread}}
			if withCurrent {
				claim := narrativePendingClaim(thread, "pending_threads", 0)
				claim.PendingThread = nil
				st.returnStatusCurrent = []store.StatusCurrentValue{{ChatSessionID: "lifecycle-46", StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: narrativeStateOwnerID(claim), WriteState: "current", ValueJSON: mustCompactJSON(narrativeStateValuePayload(claim, "", 4)), SourceTurn: 4}}
			}
			save46LifecycleFixture(t, st, 8, map[string]any{"resolved_threads": []any{map[string]any{"lifecycle_key": "prior-release-promise", "resolution_note": "Instruments delivered."}}}, "Instruments delivered.")
			if len(st.savedStatusEvents) != 1 {
				t.Fatalf("completion fabricated an earlier event: %#v", st.savedStatusEvents)
			}
			prior := narrativePendingSnapshot(parseJSONMap(st.savedStatusEvents[0].PreviousValueJSON))
			if prior == nil || prior.ID != thread.ID || prior.Status != "open" || prior.CreatedTurn != 2 || prior.SourceTurn != 4 || st.savedStatusEvents[0].SourceTurn != 8 {
				t.Fatalf("prior observed state lost or retimed: snapshot=%#v event=%#v", prior, st.savedStatusEvents[0])
			}
		})
	}
}

func Test46PendingCreationRetainsLegacyAcceptanceBesideUncertainClaim(t *testing.T) {
	for _, scenario := range []struct {
		name, excerpt string
		confidence    float64
	}{{"low-confidence", "The delivery remains uncertain.", .2}, {"ungrounded", "The captain swore an oath absent from the source.", .9}} {
		t.Run(scenario.name, func(t *testing.T) {
			st := &turnRecordingStore{}
			save46LifecycleFixture(t, st, 1, map[string]any{
				"pending_threads": []any{map[string]any{"title": "Uncertain delivery", "lifecycle_key": "uncertain-delivery", "confidence": .2}},
				"state_claims":    []any{map[string]any{"subject": "Uncertain delivery", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": "uncertain-delivery", "value": "may happen", "transition": "set", "confidence": scenario.confidence, "evidence_excerpt": scenario.excerpt}},
			}, "The scout reported. The delivery remains uncertain. They wait.")
			if len(st.savedPendingThreads) != 1 || st.savedPendingThreads[0].Status != "open" || len(st.savedStatusEvents) != 1 {
				t.Fatalf("independent pending input gained a stricter current-claim condition: pending=%#v history=%#v", st.savedPendingThreads, st.savedStatusEvents)
			}
		})
	}
}

func Test46LegacyNoKeyResolutionPreservesUnrelatedObligations(t *testing.T) {
	st := &turnRecordingStore{}
	save46LifecycleFixture(t, st, 1, map[string]any{"pending_threads": []any{
		map[string]any{"title": "Repair the bell"}, map[string]any{"title": "Repair the bridge"},
	}}, "Two repairs were promised.")
	save46LifecycleFixture(t, st, 2, map[string]any{"resolved_threads": []any{"Repair the bell"}}, "The bell was repaired.")
	states := map[string]string{}
	for _, thread := range st.returnPendingThreads {
		states[thread.Title] = thread.Status
	}
	if states["Repair the bell"] != "resolved" || states["Repair the bridge"] != "open" {
		t.Fatalf("legacy completion gained a key requirement or closed unrelated obligation: %#v", states)
	}
}
