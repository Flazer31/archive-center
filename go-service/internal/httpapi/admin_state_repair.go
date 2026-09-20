package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// A repair is an explicit interpretation of an existing source. The source is
// read from storage; a summary is never presented as a new raw observation.
type adminStateRepairSource struct {
	Kind string `json:"kind"`
	ID   int64  `json:"id"`
	Turn int    `json:"turn,omitempty"`
}

type adminStateRepairEntry struct {
	OperationID      string                 `json:"operation_id,omitempty"`
	StatusKey        string                 `json:"status_key,omitempty"`
	OwnerScope       string                 `json:"owner_scope,omitempty"`
	OwnerID          string                 `json:"owner_id,omitempty"`
	PendingThreadKey string                 `json:"pending_thread_key,omitempty"`
	Replacement      map[string]any         `json:"replacement,omitempty"`
	Source           adminStateRepairSource `json:"source"`
	UndoEventID      int64                  `json:"undo_event_id,omitempty"`
}

type adminCharacterProvenanceUndo struct {
	CharacterName string `json:"character_name"`
	EventID       int64  `json:"event_id"`
}

type adminStateRepairPlan struct {
	OperationID   string                           `json:"operation_id"`
	Before        *store.StatusCurrentValue        `json:"before,omitempty"`
	BeforePending *store.PendingThread             `json:"before_pending,omitempty"`
	After         *store.StatusCurrentValue        `json:"after,omitempty"`
	Event         store.StatusChangeEvent          `json:"event"`
	Source        map[string]any                   `json:"source"`
	Transition    store.ReversibleStatusTransition `json:"-"`
}

func (s *Server) runAdminStateRepair(ctx context.Context, sid string, req adminSessionNormalizeRequest, progress adminJobProgressFunc) (map[string]any, error) {
	items := []map[string]any{}
	now := time.Now().UTC()
	for _, entry := range req.StateRepairs {
		plan, err := s.planAdminStateRepair(ctx, sid, entry, now)
		item := map[string]any{"operation_id": entry.OperationID, "status": "unconfirmed"}
		if err != nil {
			item["detail"] = err.Error()
		} else {
			item["plan"], item["operation_id"], item["status"] = plan, plan.OperationID, "preview"
			if !req.DryRun {
				writer, ok := s.Store.(store.ReversibleStatusTransitionStore)
				if !ok {
					item["status"], item["detail"] = "error", "atomic state transition store unavailable"
				} else {
					transition := plan.Transition
					if transition.Event.RegistryID == 0 {
						result := &artifactSaveResult{}
						definition, found := s.ensureNarrativeStateDefinition(ctx, sid, now, result)
						if !found {
							return nil, fmt.Errorf("repair definition: %v", result.ErrorDetails)
						}
						transition.Event.RegistryID = definition.ID
						if transition.CurrentValue != nil {
							transition.CurrentValue.RegistryID = definition.ID
						}
					}
					saved, saveErr := writer.ApplyReversibleStatusTransition(ctx, transition)
					if saveErr != nil {
						item["status"], item["detail"] = "error", saveErr.Error()
					} else {
						item["status"], item["event_id"], item["replayed"] = "applied", saved.Event.ID, saved.Replayed
						// These are the same derived views as ordinary lifecycle writes.
						// The pending/current/history transaction is already canonical.
						pending := narrativePendingSnapshot(parseJSONMap(saved.Event.NewValueJSON))
						projectCurrent, projectionErr := s.adminRepairEventIsCurrent(ctx, sid, saved.Event, saved.Replayed)
						if projectionErr != nil {
							item["status"], item["projection_errors"] = "partial_error", []string{projectionErr.Error()}
						} else if pending != nil && projectCurrent {
							result := &artifactSaveResult{}
							projection := *pending
							metadata := parseJSONMap(projection.HookMetadataJSON)
							metadata["repair_recorded_turn"] = store.StatusChangeEventObservationTurn(saved.Event)
							projection.HookMetadataJSON = mustCompactJSON(metadata)
							projectionServer := &Server{Store: &adminRepairProjectionStore{Store: s.Store}}
							projectionServer.projectNarrativePendingArtifacts(ctx, sid, store.StatusChangeEventObservationTurn(saved.Event), projection, map[string]any{}, now, result, nil, &canonicalStateWriteCostMeasurement{})
							if result.Errors > 0 {
								item["status"], item["projection_errors"] = "partial_error", result.ErrorDetails
							}
						}
					}
				}
			}
		}
		items = append(items, item)
	}
	for _, name := range req.CharacterProvenanceRepairs {
		items = append(items, s.adminRepairCharacterProvenance(ctx, sid, name, 0, req.DryRun, now))
	}
	for _, undo := range req.CharacterProvenanceUndo {
		items = append(items, s.adminRepairCharacterProvenance(ctx, sid, undo.CharacterName, undo.EventID, req.DryRun, now))
	}
	status := "ok"
	for _, item := range items {
		if item["status"] == "error" || item["status"] == "partial_error" || item["status"] == "unconfirmed" {
			status = "partial_error"
		}
	}
	result := map[string]any{"status": status, "contract_version": store.StateRepairContract, "chat_session_id": sid, "dry_run": req.DryRun, "items": items, "ai_calls": 0, "reindex_requested": false}
	if progress != nil {
		progress(map[string]any{"status": status, "stage": "state_repair", "progress_percent": 100})
	}
	return result, nil
}

func (s *Server) adminRepairEventIsCurrent(ctx context.Context, sid string, event store.StatusChangeEvent, replayed bool) (bool, error) {
	if !replayed {
		return true, nil
	}
	reader := s.Store.(store.ReversibleStatusTransitionStore)
	events, err := reader.ListLatestReversibleCurrentProjectionEvents(ctx, sid, []string{event.StatusKey})
	if err != nil {
		return false, err
	}
	for _, current := range events {
		if current.OwnerScope == event.OwnerScope && current.OwnerID == event.OwnerID {
			return current.ID == event.ID, nil
		}
	}
	return false, nil
}

// Retrying an explicit repair completes only its missing existing projections.
// The canonical event remains the operation identity and the current authority.
type adminRepairProjectionStore struct{ store.Store }

func (s *adminRepairProjectionStore) SaveActiveState(ctx context.Context, next *store.ActiveState) error {
	rows, err := s.Store.ListActiveStates(ctx, next.ChatSessionID, next.StateType)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.StateType == next.StateType && row.TurnIndex == next.TurnIndex && reflect.DeepEqual(parseJSONMap(row.Content), parseJSONMap(next.Content)) {
			return nil
		}
	}
	writer, ok := s.Store.(activeStateSaver)
	if !ok {
		return fmt.Errorf("active state projection writer unavailable")
	}
	return writer.SaveActiveState(ctx, next)
}

func (s *adminRepairProjectionStore) SaveCanonicalStateLayer(ctx context.Context, next *store.CanonicalStateLayer) error {
	rows, err := s.Store.ListCanonicalStateLayers(ctx, next.ChatSessionID, next.LayerType)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.LayerType == next.LayerType && row.TurnIndex == next.TurnIndex && row.SourceTurn == next.SourceTurn && reflect.DeepEqual(parseJSONMap(row.Content), parseJSONMap(next.Content)) {
			return nil
		}
	}
	writer, ok := s.Store.(canonicalStateLayerSaver)
	if !ok {
		return fmt.Errorf("canonical state projection writer unavailable")
	}
	return writer.SaveCanonicalStateLayer(ctx, next)
}

func (s *adminRepairProjectionStore) SaveStoryline(ctx context.Context, next *store.Storyline) error {
	rows, err := s.Store.ListStorylines(ctx, next.ChatSessionID)
	if err != nil {
		return err
	}
	metadata := parseJSONMap(next.OngoingTensionsJSON)
	key := stringFromMap(metadata, "lifecycle_key")
	for _, row := range rows {
		previousMetadata := parseJSONMap(row.OngoingTensionsJSON)
		if (key != "" && stringFromMap(previousMetadata, "lifecycle_key") != key) || (key == "" && row.Name != next.Name) {
			continue
		}
		merged := row
		merged.Status, merged.LastTurn, merged.LastEvidenceTurn = next.Status, next.LastTurn, next.LastEvidenceTurn
		merged.UpdatedAt = next.UpdatedAt
		if !row.UserCorrected {
			merged.Name = next.Name
		}
		for _, field := range []string{"lifecycle_key", "status", "transition", "value", "source_turn", "repair_recorded_turn"} {
			if value, exists := metadata[field]; exists {
				previousMetadata[field] = value
			}
		}
		merged.OngoingTensionsJSON = mustCompactJSON(previousMetadata)
		if row.Name == merged.Name && row.Status == merged.Status && row.LastTurn == merged.LastTurn && row.LastEvidenceTurn == merged.LastEvidenceTurn && reflect.DeepEqual(parseJSONMap(row.OngoingTensionsJSON), previousMetadata) {
			return nil
		}
		*next = merged
		break
	}
	writer, ok := s.Store.(storylineSaver)
	if !ok {
		return fmt.Errorf("storyline projection writer unavailable")
	}
	return writer.SaveStoryline(ctx, next)
}

func (s *Server) planAdminStateRepair(ctx context.Context, sid string, entry adminStateRepairEntry, now time.Time) (adminStateRepairPlan, error) {
	plan := adminStateRepairPlan{}
	currentReader, ok := s.Store.(store.StatusCurrentValueStore)
	if !ok {
		return plan, fmt.Errorf("current state reader unavailable")
	}
	lifecycle, ok := s.Store.(store.StatusLifecycleStore)
	if !ok {
		return plan, fmt.Errorf("state history reader unavailable")
	}
	var undoEvent *store.StatusChangeEvent
	if entry.UndoEventID > 0 {
		events, err := lifecycle.ListStatusChangeEvents(ctx, sid, "", "", "", -1)
		if err != nil {
			return plan, err
		}
		for _, event := range events {
			if event.ID == entry.UndoEventID {
				copy := event
				undoEvent = &copy
				break
			}
		}
		if undoEvent == nil {
			return plan, fmt.Errorf("repair event %d not found in session", entry.UndoEventID)
		}
		evidence := parseJSONMap(undoEvent.EvidenceJSON)
		if stringFromMap(evidence, "repair_contract") != store.StateRepairContract {
			return plan, fmt.Errorf("event %d is not a state repair", entry.UndoEventID)
		}
		entry.StatusKey, entry.OwnerScope, entry.OwnerID = undoEvent.StatusKey, undoEvent.OwnerScope, undoEvent.OwnerID
		entry.OperationID = extractionFirstNonEmpty(entry.OperationID, fmt.Sprintf("undo:%d", entry.UndoEventID))
	}
	entry.StatusKey = extractionFirstNonEmpty(entry.StatusKey, narrativeStateStatusKey)
	currents, err := currentReader.ListStatusCurrentValues(ctx, sid, entry.OwnerScope, entry.OwnerID, entry.StatusKey, -1)
	if err != nil {
		return plan, err
	}
	var pending *store.PendingThread
	if entry.PendingThreadKey != "" {
		threads, readErr := s.Store.ListPendingThreads(ctx, sid, "all")
		if readErr != nil {
			return plan, readErr
		}
		for _, thread := range threads {
			if thread.ThreadKey == entry.PendingThreadKey {
				copy := thread
				pending = &copy
				break
			}
		}
		if pending == nil {
			return plan, fmt.Errorf("pending source %q not found", entry.PendingThreadKey)
		}
		claim := narrativePendingClaim(*pending, "repair", 0)
		entry.OwnerScope, entry.OwnerID = narrativeStateOwnerScope(claim), narrativeStateOwnerID(claim)
	}
	for _, current := range currents {
		if current.OwnerID == entry.OwnerID && current.OwnerScope == entry.OwnerScope {
			copy := current
			plan.Before = &copy
			break
		}
	}
	if pending == nil && plan.Before != nil {
		payload := parseJSONMap(plan.Before.ValueJSON)
		pending = narrativePendingSnapshot(payload)
		lifecycleKey := normalizeNarrativeLifecycleKey(stringFromMap(payload, "lifecycle_key"))
		if pending != nil || lifecycleKey != "" {
			threads, readErr := s.Store.ListPendingThreads(ctx, sid, "all")
			if readErr != nil {
				return plan, readErr
			}
			for _, thread := range threads {
				matchingSnapshot := pending != nil && thread.ThreadKey == pending.ThreadKey
				matchingLifecycle := pending == nil && lifecycleKey != "" && normalizeNarrativeLifecycleKey(stringFromMap(parseJSONMap(thread.HookMetadataJSON), "lifecycle_key")) == lifecycleKey
				if matchingSnapshot || matchingLifecycle {
					copy := thread
					pending = &copy
					break
				}
			}
		}
	}
	plan.BeforePending = pending
	if plan.Before == nil && pending == nil && undoEvent == nil {
		return plan, fmt.Errorf("existing state or pending target not found")
	}
	var source map[string]any
	var sourceTurn int
	var sourceRevision string
	var after *store.StatusCurrentValue
	var pendingAfter *store.PendingThread
	if undoEvent != nil {
		evidence := parseJSONMap(undoEvent.EvidenceJSON)
		source = mapFromAny(evidence["repair_source"])
		sourceTurn, sourceRevision = undoEvent.SourceTurn, stringFromMap(evidence, "source_revision")
		if before := mapFromAny(evidence["repair_before"]); len(before) > 0 {
			var restored store.StatusCurrentValue
			if err := json.Unmarshal([]byte(mustCompactJSON(before)), &restored); err != nil {
				return plan, err
			}
			// Copied repair history retains its origin snapshot, but undo writes
			// the selected branch using the copied registry and owner identity.
			restored.ChatSessionID, restored.RegistryID = sid, undoEvent.RegistryID
			restored.StatusKey, restored.OwnerScope, restored.OwnerID = undoEvent.StatusKey, undoEvent.OwnerScope, undoEvent.OwnerID
			after = &restored
		}
		if beforePending := mapFromAny(evidence["repair_before_pending"]); len(beforePending) > 0 {
			var restored store.PendingThread
			if err := json.Unmarshal([]byte(mustCompactJSON(beforePending)), &restored); err != nil {
				return plan, err
			}
			restored.ChatSessionID = sid
			pendingAfter = &restored
		}
	} else {
		source, sourceTurn, sourceRevision, err = s.adminStateRepairStoredSource(ctx, sid, entry.Source)
		if err != nil {
			return plan, err
		}
		if len(entry.Replacement) == 0 {
			return plan, fmt.Errorf("stored source available; explicit interpretation still required")
		}
		var current store.StatusCurrentValue
		var payload map[string]any
		if plan.Before != nil {
			current = *plan.Before
			payload = parseJSONMap(current.ValueJSON)
		} else {
			claim := narrativePendingClaim(*pending, "repair", 0)
			current = store.StatusCurrentValue{ChatSessionID: sid, StatusKey: narrativeStateStatusKey, OwnerScope: narrativeStateOwnerScope(claim), OwnerID: narrativeStateOwnerID(claim), OwnerLabel: narrativeStateOwnerLabel(claim), ValueKind: "note", WriteState: "current"}
			payload = narrativeStateValuePayload(claim, "", pending.SourceTurn)
		}
		for key, value := range entry.Replacement {
			payload[key] = value
		}
		payload["source_turn"] = sourceTurn
		// A repair does not create a new promise origin from a completion.
		delete(payload, "pending_thread")
		if pending != nil {
			copy := *pending
			copy.Status = narrativeLifecycleProjectionStatus(normalizeNarrativeTransition(stringFromMap(payload, "transition")))
			copy.SourceTurn, copy.LastSeenTurn, copy.UpdatedAt = sourceTurn, sourceTurn, now
			copy.ResolvedTurn = 0
			if copy.Status == "resolved" {
				copy.ResolvedTurn, copy.ResolutionNote = sourceTurn, stringFromMap(payload, "value")
			}
			metadata := parseJSONMap(copy.HookMetadataJSON)
			metadata["status"], metadata["transition"], metadata["value"] = copy.Status, payload["transition"], payload["value"]
			copy.HookMetadataJSON = mustCompactJSON(metadata)
			pendingAfter = &copy
			payload["pending_thread"] = copy
		}
		current.ValueJSON, current.SourceTurn = mustCompactJSON(payload), sourceTurn
		after = &current
	}
	recordedTurn := sourceTurn
	if tail, tailErr := s.adminStateRepairTail(ctx, sid); tailErr == nil && tail > recordedTurn {
		recordedTurn = tail
	}
	if plan.Before != nil && store.StatusCurrentObservationTurn(*plan.Before) > recordedTurn {
		recordedTurn = store.StatusCurrentObservationTurn(*plan.Before)
	}
	operation := extractionFirstNonEmpty(entry.OperationID, stableKey("state-repair", sid+mustCompactJSON(entry)))
	unit := "state-repair:" + operation
	evidence := map[string]any{"repair_contract": store.StateRepairContract, "repair_operation_id": operation, "repair_recorded_turn": recordedTurn, "repair_at": now.Format(time.RFC3339Nano), "repair_source": source, "repair_before": plan.Before, "repair_before_pending": plan.BeforePending, "source_turn": sourceTurn, "source_revision": sourceRevision, "source_unit_id": unit, "current_projection": true, "source": "admin.state_repair", "evidence_excerpt": stringFromMap(source, "text")}
	if undoEvent != nil {
		evidence["undo_event_id"] = undoEvent.ID
		if after != nil {
			original := parseJSONMap(after.EvidenceJSON)
			evidence["restored_source_turn"], evidence["restored_evidence"] = after.SourceTurn, original
			evidence["evidence_excerpt"], evidence["direct_evidence_ids"] = original["evidence_excerpt"], original["direct_evidence_ids"]
		}
	}
	event := store.StatusChangeEvent{ChatSessionID: sid, StatusKey: entry.StatusKey, OwnerScope: entry.OwnerScope, OwnerID: entry.OwnerID, EventKind: "repair", SourceTurn: sourceTurn, EvidenceJSON: mustCompactJSON(evidence), EventState: "recorded", CreatedAt: now}
	if plan.Before != nil {
		event.RegistryID, event.PreviousValueJSON = plan.Before.RegistryID, plan.Before.ValueJSON
	}
	if after != nil {
		after.EvidenceJSON, after.SourceTurn, after.UpdatedAt = event.EvidenceJSON, sourceTurn, now
		if after.CreatedAt.IsZero() {
			after.CreatedAt = now
		}
		event.RegistryID, event.NewValueJSON = after.RegistryID, after.ValueJSON
	} else {
		evidence["projection_action"] = "remove"
		event.EvidenceJSON, event.EventKind = mustCompactJSON(evidence), "repair_undo"
		if undoEvent != nil {
			event.RegistryID = undoEvent.RegistryID
		}
	}
	plan.OperationID, plan.After, plan.Event, plan.Source = operation, after, event, source
	plan.Transition = store.ReversibleStatusTransition{SourceContract: store.StateRepairContract, SourceRevision: sourceRevision, SourceUnitID: unit, CurrentValue: after, DeleteCurrent: after == nil, PendingSnapshot: pendingAfter, Event: event}
	if undoEvent != nil {
		var changes []store.StateRepairArtifactChange
		if err := json.Unmarshal([]byte(mustCompactJSON(parseJSONMap(undoEvent.EvidenceJSON)["repair_artifacts"])), &changes); err != nil {
			return plan, err
		}
		for _, change := range changes {
			change.VectorBeforeJSON, change.VectorAfterJSON = change.VectorAfterJSON, change.VectorBeforeJSON
			change.Before, change.After = change.After, change.Before
			plan.Transition.ArtifactChanges = append(plan.Transition.ArtifactChanges, change)
		}
	}
	return plan, nil
}

func (s *Server) adminStateRepairTail(ctx context.Context, sid string) (int, error) {
	if reader, ok := s.Store.(interface {
		LatestSessionTurnIndex(context.Context, string) (int, error)
	}); ok {
		return reader.LatestSessionTurnIndex(ctx, sid)
	}
	logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	tail := 0
	for _, row := range logs {
		if row.TurnIndex > tail {
			tail = row.TurnIndex
		}
	}
	return tail, err
}

func (s *Server) adminStateRepairStoredSource(ctx context.Context, sid string, ref adminStateRepairSource) (map[string]any, int, string, error) {
	source := map[string]any{"kind": ref.Kind, "id": ref.ID, "source_session_id": sid}
	turn, text, revision := 0, "", ""
	switch ref.Kind {
	case "memory_summary":
		items, err := s.Store.ListMemories(ctx, sid, ref.Turn, ref.Turn)
		if err != nil {
			return nil, 0, "", err
		}
		for _, row := range items {
			if row.ID == ref.ID {
				turn, text = row.TurnIndex, row.SummaryJSON
				source["evidence_kind"] = "stored_summary"
				break
			}
		}
	case "chat_log":
		items, err := s.Store.ListChatLogs(ctx, sid, ref.Turn, ref.Turn)
		if err != nil {
			return nil, 0, "", err
		}
		for _, row := range items {
			if row.ID == ref.ID {
				turn, text = row.TurnIndex, row.Content
				source["role"], source["evidence_kind"] = row.Role, "raw_log"
				break
			}
		}
	case "status_event":
		reader, ok := s.Store.(store.StatusLifecycleStore)
		if !ok {
			return nil, 0, "", fmt.Errorf("status history unavailable")
		}
		items, err := reader.ListStatusChangeEvents(ctx, sid, "", "", "", -1)
		if err != nil {
			return nil, 0, "", err
		}
		for _, row := range items {
			if row.ID == ref.ID {
				turn, text = row.SourceTurn, row.NewValueJSON
				revision = stringFromMap(parseJSONMap(row.EvidenceJSON), "source_revision")
				source["evidence_kind"] = "change_history"
				break
			}
		}
	case "episode_summary":
		row, err := s.Store.GetEpisodeSummary(ctx, ref.ID)
		if err != nil {
			return nil, 0, "", err
		}
		if row != nil && row.ChatSessionID == sid {
			turn, text = row.ToTurn, row.SummaryText
			source["from_turn"], source["to_turn"], source["evidence_kind"] = row.FromTurn, row.ToTurn, "stored_summary_range"
		}
	default:
		return nil, 0, "", fmt.Errorf("source kind %q is not a stored repair source", ref.Kind)
	}
	if text == "" {
		return nil, 0, "", fmt.Errorf("stored source %s/%d not found; occurrence and completion remain unconfirmed", ref.Kind, ref.ID)
	}
	// Legacy raw/summary rows do not carry a revision identity. Their existing
	// source-turn cleanup remains authoritative; a revision at the same turn
	// is not invented as their origin. Status history retains its actual link.
	source["text"], source["source_turn"] = text, turn
	source["occurrence_time"] = "unknown unless explicitly stated in the stored source"
	return source, turn, revision, nil
}

func (s *Server) adminRepairCharacterProvenance(ctx context.Context, sid, name string, undoID int64, dryRun bool, now time.Time) map[string]any {
	item := map[string]any{"character_name": name, "status": "unconfirmed", "undo_event_id": undoID}
	current, err := s.Store.GetCharacterState(ctx, sid, name)
	if err != nil || current == nil {
		item["detail"] = "character snapshot unavailable"
		return item
	}
	next := *current
	operation := ""
	if undoID > 0 {
		events, err := s.Store.ListCharacterEvents(ctx, sid, name)
		if err != nil {
			item["detail"] = err.Error()
			return item
		}
		found := false
		for _, event := range events {
			if event.ID != undoID || event.EventType != "field_provenance_repair" {
				continue
			}
			var before store.CharacterState
			if json.Unmarshal([]byte(mustCompactJSON(parseJSONMap(event.DetailsJSON)["before"])), &before) != nil {
				continue
			}
			next.FieldProvenanceJSON, found = before.FieldProvenanceJSON, true
			operation = fmt.Sprintf("field-provenance-undo:%d", undoID)
			break
		}
		if !found {
			item["detail"] = "field provenance repair event unavailable"
			return item
		}
	} else {
		reader, ok := s.Store.(store.CharacterStateHistoryStore)
		if !ok {
			item["detail"] = "character snapshot history unavailable"
			return item
		}
		history := []store.CharacterState{}
		for offset := 0; ; offset += 200 {
			page, err := reader.ListCharacterStateHistory(ctx, sid, name, 200, offset)
			if err != nil {
				item["detail"] = err.Error()
				return item
			}
			history = append(history, page...)
			if len(page) < 200 {
				break
			}
		}
		next.FieldProvenanceJSON = store.BuildCharacterFieldProvenanceFromHistory(history)
		operation = stableKey("field-provenance", sid+name+next.FieldProvenanceJSON)
		item["history_rows"] = len(history)
	}
	next.CreatedAt, next.UpdatedAt = now, now
	item["before"], item["after"], item["operation_id"], item["status"] = current.FieldProvenanceJSON, next.FieldProvenanceJSON, operation, "preview"
	if !dryRun {
		writer, ok := s.Store.(store.CharacterProvenanceRepairStore)
		if !ok {
			item["status"], item["detail"] = "error", "atomic character provenance repair unavailable"
			return item
		}
		event, err := writer.ApplyCharacterProvenanceRepair(ctx, *current, next, operation)
		if err != nil {
			item["status"], item["detail"] = "error", err.Error()
		} else {
			item["status"], item["event_id"] = "applied", event.ID
		}
	}
	return item
}
