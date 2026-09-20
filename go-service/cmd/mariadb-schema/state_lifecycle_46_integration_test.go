package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

// These tests use the existing disposable-server harness. All sources and
// sessions are synthetic and the harness creates and drops its own database.
func state46Registry(t *testing.T, st archiveStore.Store, sid, scope string) archiveStore.StatusSchemaDefinition {
	t.Helper()
	definitions, err := st.(archiveStore.StatusSchemaRegistryStore).SaveStatusSchemaDefinitions(context.Background(), []archiveStore.StatusSchemaDefinition{{
		ChatSessionID: sid, SchemaName: "narrative_state", StatusKey: "narrative_state", Label: "Narrative state", OwnerScope: scope, ValueKind: "note", RegistryState: "active",
	}})
	if err != nil {
		t.Fatal(err)
	}
	return definitions[0]
}

func state46Source(t *testing.T, db *sql.DB, sid string, turn int) string {
	t.Helper()
	revision := feedback43SeedSource(t, db, sid, turn)
	if _, err := db.Exec(`INSERT INTO chat_logs (chat_session_id, turn_index, role, content) VALUES (?, ?, 'user', 'Synthetic user'), (?, ?, 'assistant', 'Synthetic assistant')`, sid, turn, sid, turn); err != nil {
		t.Fatal(err)
	}
	return revision
}

func state46Transition(t *testing.T, definition archiveStore.StatusSchemaDefinition, revision, owner string, turn int, lifecycle string, pending *archiveStore.PendingThread) archiveStore.ReversibleStatusTransition {
	t.Helper()
	value := map[string]any{"version": "narrative_state.v1", "domain": "promise", "lifecycle_state": lifecycle, "subject_label": owner}
	if pending != nil {
		value["pending_thread"] = pending
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	evidenceJSON, err := json.Marshal(map[string]any{"source_revision": revision, "source_unit_id": owner, "current_projection": true})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 0, 0, turn, 0, time.UTC)
	current := archiveStore.StatusCurrentValue{ChatSessionID: definition.ChatSessionID, RegistryID: definition.ID, StatusKey: "narrative_state", OwnerScope: definition.OwnerScope, OwnerID: owner, OwnerLabel: owner, ValueKind: "note", ValueJSON: string(valueJSON), EvidenceJSON: string(evidenceJSON), SourceTurn: turn, WriteState: "current", CreatedAt: now, UpdatedAt: now}
	event := archiveStore.StatusChangeEvent{ChatSessionID: current.ChatSessionID, RegistryID: current.RegistryID, StatusKey: current.StatusKey, OwnerScope: current.OwnerScope, OwnerID: owner, EventKind: "change", NewValueJSON: current.ValueJSON, EvidenceJSON: current.EvidenceJSON, SourceTurn: turn, EventState: "recorded", CreatedAt: now}
	return archiveStore.ReversibleStatusTransition{SourceContract: "source_acceptance_observation.v1", SourceRevision: revision, SourceUnitID: owner, CurrentValue: &current, Event: event}
}

func state46Apply(t *testing.T, st archiveStore.Store, transition archiveStore.ReversibleStatusTransition) archiveStore.ReversibleStatusTransitionResult {
	t.Helper()
	result, err := st.(archiveStore.ReversibleStatusTransitionStore).ApplyReversibleStatusTransition(context.Background(), transition)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func state46Pending(t *testing.T, st archiveStore.Store, sid string) []archiveStore.PendingThread {
	t.Helper()
	rows, err := st.(interface {
		ListPendingThreads(context.Context, string, string) ([]archiveStore.PendingThread, error)
	}).ListPendingThreads(context.Background(), sid, "all")
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func state46AssertPending(t *testing.T, st archiveStore.Store, sid, key, status string, createdTurn, sourceTurn int) archiveStore.PendingThread {
	t.Helper()
	rows := state46Pending(t, st, sid)
	if len(rows) != 1 {
		t.Fatalf("%s pending rows = %+v, want one", sid, rows)
	}
	p := rows[0]
	if p.ThreadKey != key || p.Status != status || p.CreatedTurn != createdTurn || p.SourceTurn != sourceTurn {
		t.Fatalf("pending projection = %+v", p)
	}
	return p
}

func TestStateLifecycle46MariaDBTransitionReplayResumeAndDelete(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid = "state-46-lifecycle"
	definition := state46Registry(t, st, sid, "entity")
	pending := archiveStore.PendingThread{ChatSessionID: sid, ThreadKey: "promise-occurrence-1", Description: "Return a borrowed book", Status: "open", CreatedTurn: 1, SourceTurn: 1, HookType: "promise", HookMetadataJSON: `{"lifecycle_state":"active","lifecycle_instance_id":"promise-occurrence-1"}`, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	created := state46Transition(t, definition, state46Source(t, db, sid, 1), "promise-occurrence-1", 1, "active", &pending)
	state46Apply(t, st, created)
	initial := state46AssertPending(t, st, sid, pending.ThreadKey, "open", 1, 1)
	pending.ID, pending.Status, pending.SourceTurn, pending.ResolvedTurn = initial.ID, "resolved", 2, 2
	pending.HookMetadataJSON = `{"lifecycle_state":"completed","lifecycle_instance_id":"promise-occurrence-1"}`
	completed := state46Transition(t, definition, state46Source(t, db, sid, 2), "promise-occurrence-1", 2, "completed", &pending)
	firstCompletion := state46Apply(t, st, completed)
	state46AssertPending(t, st, sid, pending.ThreadKey, "resolved", 1, 2)
	if replay := state46Apply(t, st, completed); !replay.Replayed || replay.Event.ID != firstCompletion.Event.ID {
		t.Fatalf("completion replay = %+v", replay)
	}
	state46AssertPending(t, st, sid, pending.ThreadKey, "resolved", 1, 2)
	pending.Status, pending.SourceTurn, pending.ResolvedTurn = "open", 3, 0
	pending.HookMetadataJSON = `{"lifecycle_state":"active","lifecycle_instance_id":"promise-occurrence-1","transition":"resume"}`
	state46Apply(t, st, state46Transition(t, definition, state46Source(t, db, sid, 3), "promise-occurrence-1", 3, "active", &pending))
	resumed := state46AssertPending(t, st, sid, pending.ThreadKey, "open", 1, 3)
	if resumed.ID != initial.ID {
		t.Fatalf("resume created a different row: %d != %d", resumed.ID, initial.ID)
	}
	newOccurrence := pending
	newOccurrence.ID, newOccurrence.ThreadKey, newOccurrence.CreatedTurn, newOccurrence.SourceTurn = 0, "promise-occurrence-2", 4, 4
	newOccurrence.HookMetadataJSON = `{"lifecycle_state":"active","lifecycle_instance_id":"promise-occurrence-2"}`
	state46Apply(t, st, state46Transition(t, definition, state46Source(t, db, sid, 4), "promise-occurrence-2", 4, "active", &newOccurrence))
	if rows := state46Pending(t, st, sid); len(rows) != 2 {
		t.Fatalf("new occurrence overwrote original: %+v", rows)
	}
	rollback := st.(archiveStore.LogicalTurnReplacementStore)
	for _, stage := range []struct {
		turn, source int
		status       string
	}{{4, 3, "open"}, {3, 2, "resolved"}, {2, 1, "open"}} {
		if err := rollback.RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{ChatSessionID: sid, TurnIndex: stage.turn, LifecycleAction: archiveStore.LogicalTurnLifecycleDeleted}); err != nil {
			t.Fatal(err)
		}
		restored := state46AssertPending(t, st, sid, pending.ThreadKey, stage.status, 1, stage.source)
		if restored.ResolvedTurn != 0 && stage.status == "open" {
			t.Fatalf("restored open retains completion turn: %+v", restored)
		}
	}
	if err := rollback.RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{ChatSessionID: sid, TurnIndex: 2, LifecycleAction: archiveStore.LogicalTurnLifecycleDeleted}); err != nil {
		t.Fatal(err)
	}
	state46AssertPending(t, st, sid, pending.ThreadKey, "open", 1, 1)
	var events int
	if err := db.QueryRow(`SELECT COUNT(*) FROM status_change_events WHERE chat_session_id=?`, sid).Scan(&events); err != nil || events != 4 {
		t.Fatalf("history events=%d err=%v", events, err)
	}
}

func TestStateLifecycle46MariaDBReplacementRestoresSurvivingSourceAndIsolatesSibling(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid, sibling = "state-46-replace", "state-46-sibling"
	definition := state46Registry(t, st, sid, "entity")
	p := archiveStore.PendingThread{ChatSessionID: sid, ThreadKey: "same-occurrence-key", Description: "Keep watch", Status: "open", CreatedTurn: 1, SourceTurn: 1, HookType: "promise", HookMetadataJSON: `{"lifecycle_state":"active"}`}
	state46Apply(t, st, state46Transition(t, definition, state46Source(t, db, sid, 1), p.ThreadKey, 1, "active", &p))
	p.Status, p.SourceTurn, p.ResolvedTurn = "resolved", 2, 2
	p.HookMetadataJSON = `{"lifecycle_state":"completed"}`
	state46Apply(t, st, state46Transition(t, definition, state46Source(t, db, sid, 2), p.ThreadKey, 2, "completed", &p))
	siblingDefinition := state46Registry(t, st, sibling, "entity")
	siblingPending := p
	siblingPending.ChatSessionID, siblingPending.CreatedTurn, siblingPending.SourceTurn, siblingPending.ResolvedTurn = sibling, 1, 1, 1
	state46Apply(t, st, state46Transition(t, siblingDefinition, state46Source(t, db, sibling, 1), p.ThreadKey, 1, "completed", &siblingPending))
	now := time.Now().UTC()
	newSource := &archiveStore.MemorySourceRevision{ContractVersion: "source_acceptance_observation.v1", SourceRevision: "state-46-replacement-source", ChatSessionID: sid, LogicalTurnID: "turn-2", TurnIndex: 2, BranchState: "observed", UserContent: "Replacement user", AssistantContent: "Promise remains open", CombinedContentHash: strings.Repeat("b", 64), HashAlgorithm: "sha256", HostObservedAtMS: 1, LifecycleState: "active", CreatedAt: now, UpdatedAt: now}
	if err := st.(archiveStore.LogicalTurnReplacementStore).ReplaceLogicalTurn(ctx, archiveStore.LogicalTurnReplacement{ChatSessionID: sid, TurnIndex: 2, UserContent: newSource.UserContent, AssistantContent: newSource.AssistantContent, CreatedAt: now, SourceRevision: newSource}); err != nil {
		t.Fatal(err)
	}
	restored := state46AssertPending(t, st, sid, p.ThreadKey, "open", 1, 1)
	if !strings.Contains(restored.HookMetadataJSON, `"active"`) {
		t.Fatalf("completion metadata survived replacement: %s", restored.HookMetadataJSON)
	}
	state46AssertPending(t, st, sibling, p.ThreadKey, "resolved", 1, 1)
	current, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(ctx, sid, "", "", "narrative_state", -1)
	if err != nil || len(current) != 1 || current[0].SourceTurn != 1 {
		t.Fatalf("current = %+v, err=%v", current, err)
	}
	var state string
	if err := db.QueryRow(`SELECT lifecycle_state FROM memory_source_revisions WHERE chat_session_id=? AND source_revision=?`, sid, fmt.Sprintf("%s-revision-2", sid)).Scan(&state); err != nil || state != "superseded" {
		t.Fatalf("old source=%q err=%v", state, err)
	}
}

func TestStateLifecycle46MariaDBUncappedAllScopeActiveCurrent(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid, count = "state-46-many", 130
	entity := state46Registry(t, st, sid, "entity")
	world := state46Registry(t, st, sid, "world")
	revision := state46Source(t, db, sid, 1)
	for i := 0; i < count; i++ {
		definition := entity
		if i%2 != 0 {
			definition = world
		}
		state46Apply(t, st, state46Transition(t, definition, revision, fmt.Sprintf("owner-%03d", i), 1, "active", nil))
	}
	legacy := *state46Transition(t, entity, revision, "legacy-owner", 1, "active", nil).CurrentValue
	legacy.EvidenceJSON = `{}`
	if _, err := st.(archiveStore.StatusCurrentValueStore).SaveStatusCurrentValue(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	future := state46Source(t, db, sid, 2)
	state46Apply(t, st, state46Transition(t, entity, future, "owner-000", 2, "completed", nil))
	if _, err := db.Exec(`UPDATE memory_source_revisions SET lifecycle_state='superseded' WHERE chat_session_id=? AND source_revision=?`, sid, future); err != nil {
		t.Fatal(err)
	}
	reader := st.(archiveStore.StatusCurrentValueStore)
	rows, err := reader.ListStatusCurrentValues(ctx, sid, "", "", "narrative_state", -1)
	if err != nil || len(rows) != count {
		t.Fatalf("active plus legacy current count=%d want=%d err=%v", len(rows), count, err)
	}
	if err := st.(archiveStore.LogicalTurnReplacementStore).RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{ChatSessionID: sid, TurnIndex: 2, LifecycleAction: archiveStore.LogicalTurnLifecycleDeleted}); err != nil {
		t.Fatal(err)
	}
	rows, err = reader.ListStatusCurrentValues(ctx, sid, "", "", "narrative_state", -1)
	if err != nil || len(rows) != count+1 {
		t.Fatalf("restored plus legacy count=%d want=%d err=%v", len(rows), count+1, err)
	}
	for _, row := range rows {
		if row.SourceTurn != 1 {
			t.Fatalf("inactive future projected: %+v", row)
		}
	}
	scoped, err := reader.ListStatusCurrentValues(ctx, sid, "world", "", "narrative_state", -1)
	if err != nil || len(scoped) != count/2 {
		t.Fatalf("scoped count=%d want=%d err=%v", len(scoped), count/2, err)
	}
	history, err := st.(archiveStore.StatusLifecycleStore).ListStatusChangeEvents(ctx, sid, "", "", "narrative_state", -1)
	if err != nil || len(history) != count+1 {
		t.Fatalf("uncapped history count=%d want=%d err=%v", len(history), count+1, err)
	}
}

func TestStateLifecycle46MariaDBStorylineOccurrenceIdentity(t *testing.T) {
	_, st := feedback43Database(t)
	storylineWriter := st.(interface {
		SaveStoryline(context.Context, *archiveStore.Storyline) error
	})
	ctx := context.Background()
	const sid, name = "state-46-storyline", "Return the borrowed book"
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	first := archiveStore.Storyline{ChatSessionID: sid, Name: name, Status: "resolved", FirstTurn: 1, LastTurn: 2, OngoingTensionsJSON: `{"lifecycle_key":"book-occurrence-1"}`, CreatedAt: now, UpdatedAt: now}
	second := archiveStore.Storyline{ChatSessionID: sid, Name: name, Status: "active", FirstTurn: 3, LastTurn: 3, OngoingTensionsJSON: `{"lifecycle_key":"book-occurrence-2"}`, CreatedAt: now, UpdatedAt: now}
	for _, storyline := range []*archiveStore.Storyline{&first, &second, &first, &first} {
		if err := storylineWriter.SaveStoryline(ctx, storyline); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := st.ListStorylines(ctx, sid)
	if err != nil || len(rows) != 2 {
		t.Fatalf("storyline occurrences = %+v err=%v", rows, err)
	}
	for _, row := range rows {
		if row.Name != name {
			t.Fatalf("display name was altered: %+v", row)
		}
		if strings.Contains(row.OngoingTensionsJSON, "book-occurrence-2") && (row.Status != "active" || row.FirstTurn != 3 || row.LastTurn != 3) {
			t.Fatalf("old occurrence overwrote new occurrence: %+v", row)
		}
		if strings.Contains(row.OngoingTensionsJSON, "book-occurrence-1") && (row.Status != "resolved" || row.FirstTurn != 1 || row.LastTurn != 2) {
			t.Fatalf("old occurrence changed: %+v", row)
		}
	}
	second.Name = "Return the replacement book"
	if err := storylineWriter.SaveStoryline(ctx, &second); err != nil {
		t.Fatal(err)
	}
	rows, err = st.ListStorylines(ctx, sid)
	if err != nil || len(rows) != 2 || rows[0].Name != second.Name {
		t.Fatalf("same-key title change duplicated or missed occurrence: %+v err=%v", rows, err)
	}
	legacy := archiveStore.Storyline{ChatSessionID: sid, Name: "Legacy thread", Status: "active", FirstTurn: 4, LastTurn: 4, CreatedAt: now, UpdatedAt: now}
	if err := storylineWriter.SaveStoryline(ctx, &legacy); err != nil {
		t.Fatal(err)
	}
	legacy.Status, legacy.LastTurn = "resolved", 5
	if err := storylineWriter.SaveStoryline(ctx, &legacy); err != nil {
		t.Fatal(err)
	}
	rows, err = st.ListStorylines(ctx, sid)
	if err != nil || len(rows) != 3 {
		t.Fatalf("legacy name identity changed: %+v err=%v", rows, err)
	}
	if rows[0].Name != legacy.Name || rows[0].Status != "resolved" || rows[0].FirstTurn != 4 || rows[0].LastTurn != 5 {
		t.Fatalf("legacy update changed: %+v", rows[0])
	}
}
