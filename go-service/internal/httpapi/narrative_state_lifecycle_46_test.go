package httpapi

import (
	"context"
	"fmt"
	"testing"
	"time"

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
