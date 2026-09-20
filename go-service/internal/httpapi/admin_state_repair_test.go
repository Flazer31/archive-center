package httpapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test46StateRepairPreviewUsesStoredSummaryWithoutPaidRescanOrInventedTime(t *testing.T) {
	ctx := context.Background()
	sid := "repair-summary-only"
	thread := store.PendingThread{ID: 9, ChatSessionID: sid, ThreadKey: "museum-one", Description: "Repair the museum", Status: "open", CreatedTurn: 1, SourceTurn: 1, Confidence: .9, HookMetadataJSON: `{"title":"Repair the museum","lifecycle_key":"museum-one","confidence":0.9}`}
	st := &turnRecordingStore{returnPendingThreads: []store.PendingThread{thread}, returnMemories: []store.Memory{{ID: 17, ChatSessionID: sid, TurnIndex: 3, SummaryJSON: `{"summary":"The team completed the museum repairs."}`}}, returnChatLogs: []store.ChatLog{{TurnIndex: 20, Role: "user", Content: "A much later scene."}}}
	srv := &Server{Store: st}
	req := adminSessionNormalizeRequest{DryRun: true, StateRepairs: []adminStateRepairEntry{{PendingThreadKey: thread.ThreadKey, Replacement: map[string]any{"transition": "complete", "value": "The team completed the museum repairs."}, Source: adminStateRepairSource{Kind: "memory_summary", ID: 17, Turn: 3}}}}
	got, err := srv.runAdminSessionNormalize(ctx, sid, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got["ai_calls"] != 0 || got["reindex_requested"] != false || got["status"] != "ok" {
		t.Fatalf("repair-only route changed: %#v", got)
	}
	item := got["items"].([]map[string]any)[0]
	plan := item["plan"].(adminStateRepairPlan)
	if plan.Before != nil || plan.BeforePending == nil || plan.BeforePending.CreatedTurn != thread.CreatedTurn {
		t.Fatalf("invented current/origin: %+v", plan)
	}
	if plan.After.SourceTurn != 3 || store.StatusCurrentObservationTurn(*plan.After) != 20 {
		t.Fatalf("proof and recording time conflated: %+v", plan.After)
	}
	if plan.Source["evidence_kind"] != "stored_summary" {
		t.Fatalf("summary promoted to raw: %#v", plan.Source)
	}
	value := parseJSONMap(plan.After.ValueJSON)
	if value["occurrence_time"] != nil || value["effective_time"] != nil {
		t.Fatalf("invented event date: %#v", value)
	}
	pending := narrativePendingSnapshot(value)
	if pending.Status != "resolved" || pending.CreatedTurn != 1 || pending.ResolvedTurn != 3 {
		t.Fatalf("not same occurrence: %+v", pending)
	}
	if len(st.savedStatusDefinitions)+len(st.savedStatusCurrent)+len(st.savedStatusEvents)+len(st.savedPendingThreads)+len(st.savedCharacterStates) != 0 {
		t.Fatal("preview performed writes")
	}
	plan2, err := srv.planAdminStateRepair(ctx, sid, req.StateRepairs[0], time.Now().Add(time.Hour))
	if err != nil || plan2.OperationID != plan.OperationID {
		t.Fatalf("replay operation identity changed with clock: %v %+v", err, plan2)
	}
}

func Test46StateRepairMissingSourceLeavesLegacyStateUnconfirmed(t *testing.T) {
	st := &turnRecordingStore{returnPendingThreads: []store.PendingThread{{ThreadKey: "old", Description: "Old promise", SourceTurn: 2, Status: "open"}}}
	srv := &Server{Store: st}
	result, err := srv.runAdminSessionNormalize(context.Background(), "s", adminSessionNormalizeRequest{StateRepairs: []adminStateRepairEntry{{PendingThreadKey: "old", Replacement: map[string]any{"transition": "complete", "value": "Unknown completion"}, Source: adminStateRepairSource{Kind: "memory_summary", ID: 99}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	item := result["items"].([]map[string]any)[0]
	if item["status"] != "unconfirmed" || len(st.savedStatusCurrent) != 0 || len(st.savedPendingThreads) != 0 {
		t.Fatalf("missing proof changed state: %#v", result)
	}
}

func Test46StateRepairUndoPlanRestoresAbsentCurrentAndExactPending(t *testing.T) {
	before := store.PendingThread{ID: 8, ChatSessionID: "s", ThreadKey: "p", Description: "Promise", Status: "paused", SourceTurn: 2, CreatedTurn: 1, Pinned: true, UserCorrected: true, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(2, 0)}
	current := store.StatusCurrentValue{ChatSessionID: "s", RegistryID: 1, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: "p", ValueJSON: `{"transition":"complete"}`, SourceTurn: 3}
	event := store.StatusChangeEvent{ID: 44, ChatSessionID: "s", RegistryID: 1, StatusKey: current.StatusKey, OwnerScope: current.OwnerScope, OwnerID: current.OwnerID, SourceTurn: 3, EvidenceJSON: mustCompactJSON(map[string]any{"repair_contract": store.StateRepairContract, "repair_source": map[string]any{"kind": "memory_summary", "id": 7}, "repair_before_pending": before})}
	st := &turnRecordingStore{returnStatusCurrent: []store.StatusCurrentValue{current}, savedStatusEvents: []store.StatusChangeEvent{event}, returnChatLogs: []store.ChatLog{{TurnIndex: 10}}}
	plan, err := (&Server{Store: st}).planAdminStateRepair(context.Background(), "s", adminStateRepairEntry{UndoEventID: event.ID}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Transition.DeleteCurrent || plan.After != nil || plan.Transition.PendingSnapshot == nil {
		t.Fatalf("undo absence not explicit: %+v", plan)
	}
	got := plan.Transition.PendingSnapshot
	if mustCompactJSON(*got) != mustCompactJSON(before) {
		t.Fatalf("undo altered existing pending: %+v vs %+v", got, before)
	}
	if parseJSONMap(plan.Event.EvidenceJSON)["projection_action"] != "remove" {
		t.Fatal("undo is not a restoration tombstone")
	}
}

func Test46StateRepairUndoKeepsOriginalEvidenceAndTargetsCopiedBranch(t *testing.T) {
	before := store.StatusCurrentValue{ChatSessionID: "origin", RegistryID: 1, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: "p", ValueJSON: `{"value":"still bound","source_turn":1}`, EvidenceJSON: `{"evidence_excerpt":"The ropes bind Mira.","source":"critic.state_claims"}`, SourceTurn: 1}
	event := store.StatusChangeEvent{ID: 44, ChatSessionID: "branch", RegistryID: 90, StatusKey: before.StatusKey, OwnerScope: before.OwnerScope, OwnerID: before.OwnerID, SourceTurn: 3, EvidenceJSON: mustCompactJSON(map[string]any{"repair_contract": store.StateRepairContract, "repair_source": map[string]any{"kind": "memory_summary", "text": "Mira was released."}, "repair_before": before})}
	st := &turnRecordingStore{savedStatusEvents: []store.StatusChangeEvent{event}, returnChatLogs: []store.ChatLog{{TurnIndex: 10}}}
	plan, err := (&Server{Store: st}).planAdminStateRepair(context.Background(), "branch", adminStateRepairEntry{UndoEventID: event.ID}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if plan.After.ChatSessionID != "branch" || plan.After.RegistryID != event.RegistryID {
		t.Fatalf("undo targeted origin registry: %+v", plan.After)
	}
	evidence := parseJSONMap(plan.After.EvidenceJSON)
	if evidence["evidence_excerpt"] != "The ropes bind Mira." || intFromAny(evidence["restored_source_turn"], 0) != before.SourceTurn {
		t.Fatalf("undo paired original state with contrary repair proof: %#v", evidence)
	}
}

type adminRepairProjectionRecordingStore struct {
	*turnRecordingStore
	failStorylineOnce bool
	storylineWrites   int
}

func (st *adminRepairProjectionRecordingStore) ApplyReversibleStatusTransition(ctx context.Context, transition store.ReversibleStatusTransition) (store.ReversibleStatusTransitionResult, error) {
	if event, err := st.GetReversibleStatusEventBySourceUnit(ctx, transition.Event.ChatSessionID, transition.SourceRevision, transition.SourceUnitID); err == nil {
		return store.ReversibleStatusTransitionResult{Event: event, Replayed: true}, nil
	}
	event := transition.Event
	evidence := parseJSONMap(event.EvidenceJSON)
	evidence["source_contract"] = transition.SourceContract
	event.EvidenceJSON = mustCompactJSON(evidence)
	value := parseJSONMap(event.NewValueJSON)
	if transition.PendingSnapshot != nil {
		value["pending_thread"], value["repair_pending_snapshot"] = transition.PendingSnapshot, true
		updated := false
		for i := range st.returnPendingThreads {
			if st.returnPendingThreads[i].ThreadKey == transition.PendingSnapshot.ThreadKey {
				st.returnPendingThreads[i] = *transition.PendingSnapshot
				updated = true
			}
		}
		if !updated {
			st.returnPendingThreads = append(st.returnPendingThreads, *transition.PendingSnapshot)
		}
	}
	event.NewValueJSON = mustCompactJSON(value)
	if transition.DeleteCurrent {
		kept := st.returnStatusCurrent[:0]
		for _, current := range st.returnStatusCurrent {
			if current.OwnerID != event.OwnerID || current.OwnerScope != event.OwnerScope || current.StatusKey != event.StatusKey {
				kept = append(kept, current)
			}
		}
		st.returnStatusCurrent = kept
	} else if transition.CurrentValue != nil {
		current := *transition.CurrentValue
		current.ValueJSON, current.EvidenceJSON = event.NewValueJSON, event.EvidenceJSON
		if _, err := st.SaveStatusCurrentValue(ctx, current); err != nil {
			return store.ReversibleStatusTransitionResult{}, err
		}
	}
	saved, err := st.SaveStatusChangeEvent(ctx, event)
	return store.ReversibleStatusTransitionResult{Event: saved}, err
}

func (st *adminRepairProjectionRecordingStore) ListLatestReversibleCurrentProjectionEvents(_ context.Context, sid string, _ []string) ([]store.StatusChangeEvent, error) {
	latest := map[string]store.StatusChangeEvent{}
	for _, event := range st.savedStatusEvents {
		if event.ChatSessionID != sid {
			continue
		}
		key := event.StatusKey + ":" + event.OwnerScope + ":" + event.OwnerID
		previous, found := latest[key]
		if !found || store.StatusChangeEventObservationTurn(event) > store.StatusChangeEventObservationTurn(previous) || (store.StatusChangeEventObservationTurn(event) == store.StatusChangeEventObservationTurn(previous) && event.ID > previous.ID) {
			latest[key] = event
		}
	}
	rows := []store.StatusChangeEvent{}
	for _, event := range latest {
		rows = append(rows, event)
	}
	return rows, nil
}

func (st *adminRepairProjectionRecordingStore) SaveActiveState(ctx context.Context, row *store.ActiveState) error {
	st.returnActiveStates = append(st.returnActiveStates, *row)
	return st.turnRecordingStore.SaveActiveState(ctx, row)
}

func (st *adminRepairProjectionRecordingStore) SaveCanonicalStateLayer(ctx context.Context, row *store.CanonicalStateLayer) error {
	st.returnCanonicalLayers = append(st.returnCanonicalLayers, *row)
	return st.turnRecordingStore.SaveCanonicalStateLayer(ctx, row)
}

func (st *adminRepairProjectionRecordingStore) SaveStoryline(ctx context.Context, row *store.Storyline) error {
	if st.failStorylineOnce {
		st.failStorylineOnce = false
		return fmt.Errorf("injected storyline write failure")
	}
	st.storylineWrites++
	key := stringFromMap(parseJSONMap(row.OngoingTensionsJSON), "lifecycle_key")
	for i := range st.returnStorylines {
		if stringFromMap(parseJSONMap(st.returnStorylines[i].OngoingTensionsJSON), "lifecycle_key") == key {
			st.returnStorylines[i] = *row
			return nil
		}
	}
	st.returnStorylines = append(st.returnStorylines, *row)
	return nil
}

func Test46StateRepairRetriesOnlyMissingProjectionsAndPreservesLaterUndo(t *testing.T) {
	const sid = "repair-retry"
	thread := store.PendingThread{ID: 8, ChatSessionID: sid, ThreadKey: "museum-one", Title: "Repair museum", Status: "open", SourceTurn: 1, CreatedTurn: 1, Confidence: .9, HookMetadataJSON: `{"title":"Repair museum","lifecycle_key":"museum-one","confidence":0.9}`}
	st := &adminRepairProjectionRecordingStore{turnRecordingStore: &turnRecordingStore{returnPendingThreads: []store.PendingThread{thread}, returnMemories: []store.Memory{{ID: 17, ChatSessionID: sid, TurnIndex: 3, SummaryJSON: `{"summary":"Museum repair complete."}`}}, returnChatLogs: []store.ChatLog{{TurnIndex: 20}}}, failStorylineOnce: true}
	srv := &Server{Store: st}
	entry := adminStateRepairEntry{OperationID: "museum-completion", PendingThreadKey: thread.ThreadKey, Replacement: map[string]any{"transition": "complete", "value": "Museum repair complete."}, Source: adminStateRepairSource{Kind: "memory_summary", ID: 17, Turn: 3}}
	apply := func(entry adminStateRepairEntry) map[string]any {
		t.Helper()
		result, err := srv.runAdminSessionNormalize(context.Background(), sid, adminSessionNormalizeRequest{StateRepairs: []adminStateRepairEntry{entry}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if first := apply(entry); first["status"] != "partial_error" || len(st.savedStatusEvents) != 1 || len(st.returnActiveStates) != 1 || len(st.returnCanonicalLayers) != 1 || len(st.returnStorylines) != 0 {
		t.Fatalf("first failure did not preserve successful canonical/projection writes: %#v", first)
	}
	// Recomputed request data is not the saved operation's projection snapshot.
	retry := entry
	retry.Replacement = map[string]any{"transition": "complete", "value": "A different interpretation must not replace the saved operation."}
	if second := apply(retry); second["status"] != "ok" || len(st.returnStorylines) != 1 {
		t.Fatalf("retry did not finish missing storyline: %#v", second)
	}
	if value := stringFromMap(parseJSONMap(st.returnStorylines[0].OngoingTensionsJSON), "value"); value != "Museum repair complete." {
		t.Fatalf("retry used recomputed transition rather than saved event: %q", value)
	}
	apply(entry)
	if len(st.savedStatusEvents) != 1 || len(st.returnActiveStates) != 1 || len(st.returnCanonicalLayers) != 1 || st.storylineWrites != 1 {
		t.Fatalf("successful replay duplicated history/views: events=%d active=%d canonical=%d storyline=%d", len(st.savedStatusEvents), len(st.returnActiveStates), len(st.returnCanonicalLayers), st.storylineWrites)
	}
	if undo := apply(adminStateRepairEntry{UndoEventID: st.savedStatusEvents[0].ID}); undo["status"] != "ok" || len(st.returnStatusCurrent) != 0 || st.returnPendingThreads[0].Status != "open" {
		t.Fatalf("undo did not restore original absence/pending: %#v", undo)
	}
	activeCount, canonicalCount, storylineWrites := len(st.returnActiveStates), len(st.returnCanonicalLayers), st.storylineWrites
	if replay := apply(entry); replay["status"] != "ok" || len(st.returnActiveStates) != activeCount || len(st.returnCanonicalLayers) != canonicalCount || st.storylineWrites != storylineWrites || st.returnStorylines[0].Status != "active" || len(st.savedStatusEvents) != 2 {
		t.Fatalf("old repair replay replaced later undo: %#v", replay)
	}
}

func Test46StatusTargetRepairHydratesLivePendingManualEditsAndUndo(t *testing.T) {
	const sid = "repair-manual"
	stale := store.PendingThread{ID: 8, ChatSessionID: sid, ThreadKey: "museum-one", Title: "Old title", Status: "open", SourceTurn: 1, CreatedTurn: 1, Confidence: .9, HookMetadataJSON: `{"title":"Old title","lifecycle_key":"museum-one","confidence":0.9}`}
	claim := narrativePendingClaim(stale, "pending_threads", 0)
	current := store.StatusCurrentValue{ChatSessionID: sid, RegistryID: 1, StatusKey: narrativeStateStatusKey, OwnerScope: narrativeStateOwnerScope(claim), OwnerID: narrativeStateOwnerID(claim), WriteState: "current", SourceTurn: 1, ValueJSON: mustCompactJSON(narrativeStateValuePayload(claim, "", 1))}
	live := stale
	live.Title, live.Pinned, live.Suppressed, live.UserCorrected, live.UpdatedAt = "My museum obligation", true, true, true, time.Unix(6, 0)
	live.HookMetadataJSON = `{"title":"My museum obligation","lifecycle_key":"museum-one","confidence":0.9,"manual_note":"keep"}`
	storyline := store.Storyline{ChatSessionID: sid, Name: "My storyline name", Status: "active", Pinned: true, Suppressed: true, UserCorrected: true, CurrentContext: "My storyline context", EntitiesJSON: `["Mira"]`, OngoingTensionsJSON: `{"lifecycle_key":"museum-one","manual_note":"storyline note"}`}
	st := &adminRepairProjectionRecordingStore{turnRecordingStore: &turnRecordingStore{returnStatusCurrent: []store.StatusCurrentValue{current}, returnPendingThreads: []store.PendingThread{live}, returnStorylines: []store.Storyline{storyline}, returnMemories: []store.Memory{{ID: 17, ChatSessionID: sid, TurnIndex: 3, SummaryJSON: `{"summary":"Museum repair complete."}`}}, returnChatLogs: []store.ChatLog{{TurnIndex: 20}}}}
	srv := &Server{Store: st}
	entry := adminStateRepairEntry{OperationID: "manual-preservation", OwnerScope: current.OwnerScope, OwnerID: current.OwnerID, StatusKey: current.StatusKey, Replacement: map[string]any{"transition": "complete", "value": "Museum repair complete."}, Source: adminStateRepairSource{Kind: "memory_summary", ID: 17, Turn: 3}}
	legacy := current
	legacyPayload := parseJSONMap(legacy.ValueJSON)
	delete(legacyPayload, "pending_thread")
	legacy.ValueJSON = mustCompactJSON(legacyPayload)
	st.returnStatusCurrent = []store.StatusCurrentValue{legacy}
	preview, previewErr := srv.runAdminSessionNormalize(context.Background(), sid, adminSessionNormalizeRequest{DryRun: true, StateRepairs: []adminStateRepairEntry{entry}}, nil)
	if previewErr != nil || preview["status"] != "ok" {
		t.Fatalf("legacy current preview failed: %#v %v", preview, previewErr)
	}
	previewPlan := preview["items"].([]map[string]any)[0]["plan"].(adminStateRepairPlan)
	if previewPlan.BeforePending == nil || mustCompactJSON(*previewPlan.BeforePending) != mustCompactJSON(live) {
		t.Fatal("legacy current without snapshot did not hydrate exact lifecycle occurrence")
	}
	st.returnStatusCurrent = []store.StatusCurrentValue{current}
	result, err := srv.runAdminSessionNormalize(context.Background(), sid, adminSessionNormalizeRequest{StateRepairs: []adminStateRepairEntry{entry}}, nil)
	if err != nil || result["status"] != "ok" {
		t.Fatalf("status-target repair failed: %#v %v", result, err)
	}
	got := st.returnPendingThreads[0]
	if got.Title != live.Title || !got.Pinned || !got.Suppressed || !got.UserCorrected || stringFromMap(parseJSONMap(got.HookMetadataJSON), "manual_note") != "keep" {
		t.Fatalf("status snapshot overwrote live pending manual edits: %+v", got)
	}
	story := st.returnStorylines[0]
	if story.Name != storyline.Name || !story.Pinned || !story.Suppressed || !story.UserCorrected || story.CurrentContext != storyline.CurrentContext || story.EntitiesJSON != storyline.EntitiesJSON || stringFromMap(parseJSONMap(story.OngoingTensionsJSON), "manual_note") != "storyline note" || story.Status != "resolved" {
		t.Fatalf("repair erased unrelated storyline edits: %+v", story)
	}
	result, err = srv.runAdminSessionNormalize(context.Background(), sid, adminSessionNormalizeRequest{StateRepairs: []adminStateRepairEntry{{UndoEventID: st.savedStatusEvents[0].ID}}}, nil)
	if err != nil || result["status"] != "ok" || mustCompactJSON(st.returnPendingThreads[0]) != mustCompactJSON(live) {
		t.Fatalf("undo did not restore exact live-before pending: result=%#v pending=%+v err=%v", result, st.returnPendingThreads[0], err)
	}
}
