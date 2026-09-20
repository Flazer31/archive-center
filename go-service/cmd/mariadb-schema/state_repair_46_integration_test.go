package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

func repair46Transition(t *testing.T, definition archiveStore.StatusSchemaDefinition, revision, owner, operation string, causeTurn, recordedTurn int, state string, pending *archiveStore.PendingThread) archiveStore.ReversibleStatusTransition {
	t.Helper()
	transition := state46Transition(t, definition, revision, owner, causeTurn, state, pending)
	transition.SourceContract = archiveStore.StateRepairContract
	transition.SourceUnitID = operation
	evidence, err := json.Marshal(map[string]any{"source_revision": revision, "source_unit_id": operation, "current_projection": true, "repair_recorded_turn": recordedTurn, "proof_kind": "summary"})
	if err != nil {
		t.Fatal(err)
	}
	transition.Event.EvidenceJSON, transition.CurrentValue.EvidenceJSON = string(evidence), string(evidence)
	return transition
}

func repair46Current(t *testing.T, st archiveStore.Store, sid string) []archiveStore.StatusCurrentValue {
	t.Helper()
	rows, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(context.Background(), sid, "", "", "narrative_state", -1)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func repair46Rollback(t *testing.T, st archiveStore.Store, sid string, turn int) {
	t.Helper()
	if err := st.(archiveStore.LogicalTurnReplacementStore).RollbackCanonicalTail(context.Background(), archiveStore.LogicalTurnRollback{ChatSessionID: sid, TurnIndex: turn, LifecycleAction: archiveStore.LogicalTurnLifecycleDeleted}); err != nil {
		t.Fatal(err)
	}
}

func TestStateRepair46MariaDBObservationOrderReplayAndCauseDeletion(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid, owner = "repair-46-order", "promise-book"
	definition := state46Registry(t, st, sid, "entity")
	pending := archiveStore.PendingThread{ChatSessionID: sid, ThreadKey: owner, Description: "Return the borrowed book", Status: "open", CreatedTurn: 1, SourceTurn: 1, HookType: "promise", HookMetadataJSON: `{"lifecycle_state":"active"}`}
	state46Apply(t, st, state46Transition(t, definition, state46Source(t, db, sid, 1), owner, 1, "active", &pending))
	proofRevision := state46Source(t, db, sid, 2)
	lateRevision := state46Source(t, db, sid, 3)
	state46Source(t, db, sid, 5)
	pending.Status, pending.SourceTurn, pending.ResolvedTurn = "resolved", 2, 2
	pending.HookMetadataJSON = `{"lifecycle_state":"completed"}`
	repair := repair46Transition(t, definition, proofRevision, owner, "summary-correction-1", 2, 5, "completed", &pending)
	first := state46Apply(t, st, repair)
	if replay := state46Apply(t, st, repair); !replay.Replayed || replay.Event.ID != first.Event.ID || replay.CurrentValue.ID != first.CurrentValue.ID {
		t.Fatalf("repair replay = %+v", replay)
	}
	current := repair46Current(t, st, sid)
	if len(current) != 1 || current[0].SourceTurn != repair.Event.SourceTurn || archiveStore.StatusCurrentObservationTurn(current[0]) != 5 {
		t.Fatalf("cause and observation time conflated: %+v", current)
	}
	ordinary := state46Transition(t, definition, lateRevision, owner, 3, "active", nil)
	if _, err := st.(archiveStore.ReversibleStatusTransitionStore).ApplyReversibleStatusTransition(ctx, ordinary); !errors.Is(err, archiveStore.ErrStatusProjectionStale) {
		t.Fatalf("delayed older ordinary source replaced explicit repair: %v", err)
	}
	repair46Rollback(t, st, sid, 5)
	current = repair46Current(t, st, sid)
	if len(current) != 1 || !strings.Contains(current[0].ValueJSON, `"completed"`) {
		t.Fatalf("unrelated tail deletion lost repair: %+v", current)
	}
	state46AssertPending(t, st, sid, owner, "resolved", 1, 2)
	repair46Rollback(t, st, sid, 2)
	current = repair46Current(t, st, sid)
	if len(current) != 1 || current[0].SourceTurn != 1 || !strings.Contains(current[0].ValueJSON, `"active"`) {
		t.Fatalf("actual cause deletion failed to restore prior projection: %+v", current)
	}
	state46AssertPending(t, st, sid, owner, "open", 1, 1)
}

func TestStateRepair46MariaDBAbsentCurrentUndoKeepsExactPendingAndTombstone(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid, owner = "repair-46-legacy", "legacy-promise"
	definition := state46Registry(t, st, sid, "entity")
	for _, turn := range []int{1, 2, 5} {
		if _, err := db.Exec(`INSERT INTO chat_logs (chat_session_id, turn_index, role, content) VALUES (?, ?, 'assistant', 'Synthetic legacy summary source')`, sid, turn); err != nil {
			t.Fatal(err)
		}
	}
	old := archiveStore.PendingThread{ChatSessionID: sid, ThreadKey: owner, Description: "An old promise with original origin", Status: "open", CreatedTurn: 1, SourceTurn: 1, Priority: 2, HookType: "promise", HookMetadataJSON: `{"lifecycle_state":"active","original":"retained"}`, Pinned: true, UserCorrected: true, CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), UpdatedAt: time.Date(2026, 1, 2, 3, 5, 0, 0, time.UTC)}
	if err := st.(interface {
		SavePendingThread(context.Context, *archiveStore.PendingThread) error
	}).SavePendingThread(ctx, &old); err != nil {
		t.Fatal(err)
	}
	before := state46Pending(t, st, sid)[0]
	completed := before
	completed.Status, completed.SourceTurn, completed.ResolvedTurn = "resolved", 2, 2
	completed.HookMetadataJSON = `{"lifecycle_state":"completed","proof":"summary"}`
	repair := repair46Transition(t, definition, "", owner, "legacy-summary-correction", 2, 5, "completed", &completed)
	first := state46Apply(t, st, repair)
	if replay := state46Apply(t, st, repair); !replay.Replayed || replay.Event.ID != first.Event.ID || replay.CurrentValue.ID != first.CurrentValue.ID {
		t.Fatalf("empty revision replay did not find original operation: %+v", replay)
	}
	if _, err := st.(archiveStore.ReversibleStatusTransitionStore).GetReversibleStatusEventBySourceUnit(ctx, sid, "", repair.SourceUnitID); err != nil {
		t.Fatalf("empty revision exact lookup: %v", err)
	}
	undo := repair46Transition(t, definition, "", owner, "legacy-summary-undo", 2, 5, "", nil)
	undo.CurrentValue, undo.DeleteCurrent, undo.PendingSnapshot = nil, true, &before
	undo.Event.PreviousValueJSON = first.CurrentValue.ValueJSON
	undone := state46Apply(t, st, undo)
	if current := repair46Current(t, st, sid); len(current) != 0 {
		t.Fatalf("undo invented prior current: %+v", current)
	}
	if after := state46Pending(t, st, sid)[0]; !reflect.DeepEqual(after, before) {
		t.Fatalf("undo pending differs from actual before snapshot\nbefore=%+v\nafter=%+v", before, after)
	}
	latest, err := st.(archiveStore.ReversibleStatusTransitionStore).ListLatestReversibleCurrentProjectionEvents(ctx, sid, []string{"narrative_state"})
	if err != nil || len(latest) != 1 || latest[0].ID != undone.Event.ID || !strings.Contains(latest[0].EvidenceJSON, `"projection_action":"remove"`) {
		t.Fatalf("latest projection omitted removal: %+v err=%v", latest, err)
	}
	repair46Rollback(t, st, sid, 5)
	if current := repair46Current(t, st, sid); len(current) != 0 {
		t.Fatalf("rebuild resurrected undone current: %+v", current)
	}
	if after := state46Pending(t, st, sid)[0]; !reflect.DeepEqual(after, before) {
		t.Fatalf("rebuild lost exact pending snapshot: %+v", after)
	}
	if replay := state46Apply(t, st, undo); !replay.Replayed || replay.Event.ID != undone.Event.ID {
		t.Fatalf("undo replay appended history: %+v", replay)
	}
	var revisionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_source_revisions WHERE chat_session_id=?`, sid).Scan(&revisionCount); err != nil || revisionCount != 0 {
		t.Fatalf("legacy repair invented source revisions: count=%d err=%v", revisionCount, err)
	}
	repair46Rollback(t, st, sid, 2)
	if current := repair46Current(t, st, sid); len(current) != 0 {
		t.Fatalf("cause deletion resurrected current: %+v", current)
	}
	if after := state46Pending(t, st, sid)[0]; !reflect.DeepEqual(after, before) {
		t.Fatalf("cause deletion changed prior pending: %+v", after)
	}
}

func TestStateRepair46MariaDBExistingCurrentUndoAndSourceReplacement(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid, owner = "repair-46-undo", "promise-watch"
	definition := state46Registry(t, st, sid, "entity")
	priorPending := archiveStore.PendingThread{ChatSessionID: sid, ThreadKey: owner, Description: "Keep watch", Status: "open", CreatedTurn: 1, SourceTurn: 1, HookType: "promise", HookMetadataJSON: `{"lifecycle_state":"active"}`}
	prior := state46Apply(t, st, state46Transition(t, definition, state46Source(t, db, sid, 1), owner, 1, "active", &priorPending))
	priorPending = state46Pending(t, st, sid)[0]
	proofRevision := state46Source(t, db, sid, 2)
	state46Source(t, db, sid, 4)
	completed := priorPending
	completed.Status, completed.SourceTurn, completed.ResolvedTurn = "resolved", 2, 2
	repair := repair46Transition(t, definition, proofRevision, owner, "correct-watch", 2, 4, "completed", &completed)
	first := state46Apply(t, st, repair)
	undo := repair46Transition(t, definition, proofRevision, owner, "undo-watch", 2, 4, "active", &priorPending)
	undo.CurrentValue.ValueJSON, undo.Event.NewValueJSON = prior.CurrentValue.ValueJSON, prior.CurrentValue.ValueJSON
	undo.Event.PreviousValueJSON, undo.PendingSnapshot = first.CurrentValue.ValueJSON, &priorPending
	state46Apply(t, st, undo)
	repair46Rollback(t, st, sid, 4)
	if current := repair46Current(t, st, sid); len(current) != 1 || !strings.Contains(current[0].ValueJSON, `"active"`) {
		t.Fatalf("existing current undo did not survive rebuild: %+v", current)
	}
	if after := state46Pending(t, st, sid)[0]; !reflect.DeepEqual(after, priorPending) {
		t.Fatalf("existing current undo did not restore pending exactly: %+v", after)
	}
	now := time.Now().UTC()
	newSource := &archiveStore.MemorySourceRevision{ContractVersion: "source_acceptance_observation.v1", SourceRevision: sid + "-replacement", ChatSessionID: sid, LogicalTurnID: "turn-2", TurnIndex: 2, BranchState: "observed", UserContent: "Edited source", AssistantContent: "No completion occurred", CombinedContentHash: strings.Repeat("c", 64), HashAlgorithm: "sha256", HostObservedAtMS: 1, LifecycleState: "active", CreatedAt: now, UpdatedAt: now}
	if err := st.(archiveStore.LogicalTurnReplacementStore).ReplaceLogicalTurn(ctx, archiveStore.LogicalTurnReplacement{ChatSessionID: sid, TurnIndex: 2, UserContent: newSource.UserContent, AssistantContent: newSource.AssistantContent, CreatedAt: now, SourceRevision: newSource}); err != nil {
		t.Fatal(err)
	}
	if current := repair46Current(t, st, sid); len(current) != 1 || current[0].SourceTurn != 1 || archiveStore.StatusCurrentObservationTurn(current[0]) != 1 {
		t.Fatalf("replaced cause retained repair authority: %+v", current)
	}
}

func TestStateRepair46MariaDBLegacyConcurrentReplayUsesOneHistoryEvent(t *testing.T) {
	_, st := feedback43Database(t)
	ctx := context.Background()
	const sid, owner = "repair-46-concurrent", "legacy-promise"
	definition := state46Registry(t, st, sid, "entity")
	transition := repair46Transition(t, definition, "", owner, "shared-operation", 2, 6, "completed", nil)
	const workers = 8
	results := make(chan archiveStore.ReversibleStatusTransitionResult, workers)
	errors := make(chan error, workers)
	var ready, finished sync.WaitGroup
	ready.Add(workers)
	finished.Add(workers)
	start := make(chan struct{})
	for range workers {
		go func() {
			defer finished.Done()
			ready.Done()
			<-start
			result, err := st.(archiveStore.ReversibleStatusTransitionStore).ApplyReversibleStatusTransition(ctx, transition)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}
	ready.Wait()
	close(start)
	finished.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	var eventID int64
	created := 0
	for result := range results {
		if eventID != 0 && eventID != result.Event.ID {
			t.Fatalf("same operation produced two history events: %d and %d", eventID, result.Event.ID)
		}
		eventID = result.Event.ID
		if !result.Replayed {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("new writes = %d, want exactly one", created)
	}
}

func TestStateRepair46MariaDBUnknownCauseTurnStaysUnknownAcrossReplayAndRebuild(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid, owner = "repair-46-unknown-cause", "legacy-origin-unknown"
	definition := state46Registry(t, st, sid, "entity")
	// Imported material can have an unknown source turn. Its actual stored
	// evidence remains turn zero; the correction does not manufacture turn one.
	if _, err := db.Exec(`INSERT INTO chat_logs (chat_session_id, turn_index, role, content) VALUES (?, 0, 'assistant', 'Imported summary with no known occurrence date'), (?, 4, 'assistant', 'Unrelated later source')`, sid, sid); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO memories (chat_session_id, turn_index, summary_json) VALUES (?, 0, '{"summary":"The borrowed item was returned; occurrence date unknown"}')`, sid); err != nil {
		t.Fatal(err)
	}
	transition := repair46Transition(t, definition, "", owner, "unknown-cause-repair", 0, 4, "completed", nil)
	first := state46Apply(t, st, transition)
	if first.CurrentValue.SourceTurn != 0 || first.Event.SourceTurn != 0 {
		t.Fatalf("repair invented a cause turn: %+v", first)
	}
	if replay := state46Apply(t, st, transition); !replay.Replayed || replay.Event.ID != first.Event.ID || replay.CurrentValue.SourceTurn != 0 {
		t.Fatalf("unknown cause replay = %+v", replay)
	}
	repair46Rollback(t, st, sid, 4)
	current := repair46Current(t, st, sid)
	if len(current) != 1 || current[0].SourceTurn != 0 || archiveStore.StatusCurrentObservationTurn(current[0]) != 4 {
		t.Fatalf("unknown cause rebuild = %+v", current)
	}
	latest, err := st.(archiveStore.ReversibleStatusTransitionStore).ListLatestReversibleCurrentProjectionEvents(ctx, sid, []string{"narrative_state"})
	if err != nil || len(latest) != 1 || latest[0].SourceTurn != 0 {
		t.Fatalf("latest unknown cause = %+v, err=%v", latest, err)
	}
	undo := repair46Transition(t, definition, "", owner, "unknown-cause-undo", 0, 4, "", nil)
	undo.CurrentValue, undo.DeleteCurrent = nil, true
	state46Apply(t, st, undo)
	repair46Rollback(t, st, sid, 1)
	if current := repair46Current(t, st, sid); len(current) != 0 {
		t.Fatalf("unknown cause tombstone was lost: %+v", current)
	}
}

func TestStateRepair46MariaDBUndoTombstoneRetainsObservationOrder(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid, owner = "repair-46-removed-order", "existing-promise"
	definition := state46Registry(t, st, sid, "entity")
	proof := state46Source(t, db, sid, 2)
	olderSource := state46Source(t, db, sid, 3)
	state46Source(t, db, sid, 5)
	repair := repair46Transition(t, definition, proof, owner, "repair-before-undo", 2, 5, "completed", nil)
	state46Apply(t, st, repair)
	undo := repair46Transition(t, definition, proof, owner, "remove-created-projection", 2, 5, "", nil)
	undo.CurrentValue, undo.DeleteCurrent = nil, true
	state46Apply(t, st, undo)
	ordinary := state46Transition(t, definition, olderSource, owner, 3, "active", nil)
	if _, err := st.(archiveStore.ReversibleStatusTransitionStore).ApplyReversibleStatusTransition(ctx, ordinary); !errors.Is(err, archiveStore.ErrStatusProjectionStale) {
		t.Fatalf("delayed source overwrote removed projection ordering: %v", err)
	}
	if current := repair46Current(t, st, sid); len(current) != 0 {
		t.Fatalf("delayed source recreated removed projection: %+v", current)
	}
}
