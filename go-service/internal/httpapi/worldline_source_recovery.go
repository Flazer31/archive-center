package httpapi

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// The Host supplies only requested text from the copied child prefix. Go keeps
// the original session/turn owner; inherited responses never become child rows.
type worldlineSourceRecoveryObservation struct {
	MessageIndex     int    `json:"message_index"`
	UserContent      string `json:"user_content"`
	AssistantContent string `json:"assistant_content"`
}

type worldlineSourceRecoveryPlan struct {
	SessionID    string         `json:"chat_session_id"`
	TurnIndex    int            `json:"turn_index"`
	MessageIndex int            `json:"message_index"`
	Status       string         `json:"status"`
	Job          map[string]any `json:"job,omitempty"`
	hostID       string
	userID       string
	assistantID  string
}

const worldlineRecoveryRevisionPrefix = "sar_branch_"

func (s *Server) recoverWorldlineSources(ctx context.Context, req sessionRoutingTurnResolutionRequest, vm *worldlineViewModel) ([]worldlineSourceRecoveryPlan, error) {
	if vm == nil || vm.State != "confirmed" || req.WorldlineObservation == nil || req.WorldlineObservation.MessageOrigins == nil {
		return nil, nil
	}
	active, ok := s.Store.(store.ActiveSourceRevisionLister)
	if !ok {
		return nil, store.ErrNotEnabled
	}
	parentHostID, _, sourceID, _ := parseExactRisuBranchMarker(req.WorldlineObservation.BranchMarker)
	parents := risuWorldlineParentMessages(req.WorldlineObservation, parentHostID, sourceID)
	sources, err := active.ListActiveSourceRevisions(ctx, vm.ParentSessionID, 0, 0)
	if err != nil {
		return nil, err
	}
	// Stored source coordinates remain authoritative, including offsets and
	// deleted messages. This is the same resolver used to establish the fork.
	scope := resolvePrepareTurnHistoryScope(ctx, s.Store, req.ChatSessionID, vm.InheritedThroughTurn+1)
	origins := readRisuWorldlineOrigins(risuWorldlineOriginItems(req.WorldlineObservation))
	childByParent := map[string]risuWorldlineMessageObservation{}
	for _, child := range req.WorldlineObservation.MessageOrigins.ChildMessages {
		childByParent[risuOriginMessageID(origins, child.MessageChatID, child.Role)] = child
	}
	observations := map[int]worldlineSourceRecoveryObservation{}
	for _, item := range req.InheritedRecovery {
		observations[item.MessageIndex] = item
	}
	// One source/audit read per ancestor, rather than one full audit scan for
	// every copied turn. This cache exists only for this routing request.
	ownerSources := map[string]map[int]store.MemorySourceRevision{}
	ownerAudits := map[string][]store.AuditLog{}
	for _, segment := range scope.Segments {
		if segment.SessionID == req.ChatSessionID {
			continue
		}
		items, readErr := active.ListActiveSourceRevisions(ctx, segment.SessionID, segment.FromTurn, segment.ToTurn)
		if readErr != nil {
			return nil, readErr
		}
		ownerSources[segment.SessionID] = map[int]store.MemorySourceRevision{}
		for _, item := range items {
			ownerSources[segment.SessionID][item.TurnIndex] = item
		}
		audits, readErr := s.Store.ListAuditLogs(ctx, segment.SessionID, "critic_ingest_trace", 0)
		if readErr != nil {
			return nil, readErr
		}
		ownerAudits[segment.SessionID] = audits
	}
	var plans []worldlineSourceRecoveryPlan
	for _, message := range parents {
		if message.Disabled || message.Role != "char" {
			continue
		}
		turn := s.risuWorldlineObservedSourceTurn(ctx, req.WorldlineObservation, vm.ParentSessionID, parentHostID, message.MessageChatID, sources)
		if turn <= 0 || turn > vm.InheritedThroughTurn {
			continue
		}
		var owner string
		for _, segment := range scope.Segments {
			if prepareTurnHistorySegmentContains(segment, turn) {
				owner = segment.SessionID
				break
			}
		}
		if owner == "" || owner == req.ChatSessionID {
			continue
		}
		child, found := childByParent[message.MessageChatID]
		if !found {
			continue
		}
		plan := worldlineSourceRecoveryPlan{SessionID: owner, TurnIndex: turn, MessageIndex: child.MessageIndex, Status: "read_original", hostID: parentHostID, assistantID: message.MessageChatID,
			userID: risuWorldlineParentUserAnchor(req.WorldlineObservation, parentHostID, message.MessageChatID)}
		// A copied prefix can belong to a grandparent. Map identities along the
		// already confirmed chain, just as retrieval does, without copying rows.
		cursor := vm.ParentSessionID
		for depth := 0; cursor != owner && depth < prepareTurnHistoryMaxDepth; depth++ {
			parent := currentWorldlineViewModel(ctx, s.Store, cursor)
			fs, ok := s.Store.(store.ForkLineageStore)
			if !ok {
				break
			}
			records, listErr := fs.ListForkLineageRecords(ctx, cursor, "", 0)
			if listErr != nil {
				return nil, listErr
			}
			for _, record := range records {
				if record.LineageState != "confirmed" || record.CopiedFromSessionID != parent.ParentSessionID {
					continue
				}
				if mapping := readRisuWorldlineOrigins(record.InheritedItemsJSON); mapping != nil {
					plan.hostID = mapping.ParentHostChatID
					plan.assistantID = risuOriginMessageID(mapping, plan.assistantID, "char")
					plan.userID = risuOriginMessageID(mapping, plan.userID, "user")
				}
				break
			}
			cursor = parent.ParentSessionID
		}
		if source, exists := ownerSources[owner][turn]; exists {
			if adminRescanSourceProjectionCompleteFromAudit(&source, ownerAudits[owner]) {
				continue
			}
		}
		if s.worldlineRecoveryProcessing(owner, turn) {
			plan.Status = "processing"
			plans = append(plans, plan)
			continue
		}
		// Even existing raw text is compared with the selected branch copy: the
		// parent may since have been rerolled. Repair never overwrites it.
		if observation, supplied := observations[child.MessageIndex]; supplied {
			plan.Job = s.startWorldlineSourceRecovery(plan, observation, req.RecoveryClientMeta)
			plan.Status = "queued"
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (s *Server) worldlineRecoverySource(ctx context.Context, sid string, turn int) (*store.MemorySourceRevision, error) {
	reader, ok := s.Store.(store.ActiveSourceRevisionLister)
	if !ok {
		return nil, store.ErrNotEnabled
	}
	items, err := reader.ListActiveSourceRevisions(ctx, sid, turn, turn)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].TurnIndex == turn {
			return &items[i], nil
		}
	}
	return nil, nil
}

func (s *Server) worldlineRecoveryProcessing(sid string, turn int) bool {
	if s.SourceAcceptances == nil {
		return false
	}
	l := s.SourceAcceptances
	l.mu.Lock()
	defer l.mu.Unlock()
	if worker := l.workers[sourceAcceptanceStateKey(sid, turn)]; worker.done != nil {
		return true
	}
	for _, worker := range l.reprocessingWorkers {
		if worker.sessionID == sid && worker.turnIndex == turn {
			return true
		}
	}
	return false
}

func (s *Server) startWorldlineSourceRecovery(plan worldlineSourceRecoveryPlan, observation worldlineSourceRecoveryObservation, meta map[string]any) map[string]any {
	if s.AdminJobs == nil {
		s.AdminJobs = newAdminJobManager()
	}
	return s.AdminJobs.start(fmt.Sprintf("worldline_source_recovery_%d", plan.TurnIndex), plan.SessionID,
		map[string]any{"turn_index": plan.TurnIndex}, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
			return s.runWorldlineSourceRecovery(ctx, plan, observation, meta)
		})
}

func (s *Server) runWorldlineSourceRecovery(ctx context.Context, plan worldlineSourceRecoveryPlan, observation worldlineSourceRecoveryObservation, meta map[string]any) (map[string]any, error) {
	result := map[string]any{"chat_session_id": plan.SessionID, "turn_index": plan.TurnIndex}
	// Use the same migration write fence as normal completion/reprocessing.
	if lock, err := s.sessionMigrationSourceLock(ctx, plan.SessionID); err != nil {
		return result, err
	} else if lock != nil {
		return result, fmt.Errorf("source_session_migration_locked")
	}
	if s.worldlineRecoveryProcessing(plan.SessionID, plan.TurnIndex) {
		result["status"] = "processing"
		return result, nil
	}
	u, a := sanitizeCriticStorageText(observation.UserContent), sanitizeCriticStorageText(observation.AssistantContent)
	if a == "" {
		return result, fmt.Errorf("worldline_source_original_unavailable")
	}
	repair, err := s.runChatLogRepairReplay(ctx, plan.SessionID, dto.ChatLogRepairReplayRequest{Entries: []dto.ChatLogRepairEntryRequest{{TurnIndex: plan.TurnIndex, UserContent: &u, AssistantContent: &a}}})
	if err != nil {
		return result, err
	}
	result["raw_repair"] = repair
	if len(intSliceFromAny(repair["conflict_turns"])) > 0 {
		return result, fmt.Errorf("worldline_source_existing_revision_differs")
	}
	source, err := s.worldlineRecoverySource(ctx, plan.SessionID, plan.TurnIndex)
	if err != nil {
		return result, err
	}
	if source == nil {
		now := time.Now().UTC()
		source = adminRescanCanonicalRawSourceRevision(plan.SessionID, plan.TurnIndex, u, a, adminRescanSourceObservation{AssistantMessageID: plan.assistantID}, now)
		source.SourceRevision = fmt.Sprintf("%s%x", worldlineRecoveryRevisionPrefix, sha256.Sum256([]byte(strings.Join([]string{plan.SessionID, fmt.Sprint(plan.TurnIndex), plan.assistantID, u, a}, "\x1f"))))
		source.LogicalTurnID = completeTurnLogicalTurnID(plan.SessionID, completeTurnSourceObservation{HostChatID: plan.hostID, HostChatIDState: "observed", UserMessageChatID: plan.userID, UserMessageChatIDState: "observed"})
		writer, ok := s.Store.(store.SourceRevisionStore)
		if !ok {
			return result, store.ErrNotEnabled
		}
		if _, err = writer.RegisterAcceptedSourceRevision(ctx, source); err != nil {
			registrationErr := err
			// A normal completion may have registered this turn while raw repair
			// was running. Reuse its source rather than creating another revision.
			source, err = s.worldlineRecoverySource(ctx, plan.SessionID, plan.TurnIndex)
			if err != nil {
				return result, err
			}
			if source == nil {
				return result, registrationErr
			}
		}
	}
	if s.worldlineRecoveryProcessing(plan.SessionID, plan.TurnIndex) {
		result["status"] = "processing"
		return result, nil
	}
	complete, err := s.adminRescanSourceProjectionComplete(ctx, source)
	if err != nil {
		return result, err
	}
	if complete {
		result["status"] = "already_completed"
		return result, nil
	}
	// A previous attempt already captured its input. Let the existing durable
	// retry job own its retry limit/backoff; another branch is not a fresh call.
	if source.DerivedAdmissionState != "committed" && source.CriticInputSnapshotJSON != "" {
		if jobs, ok := s.Store.(store.MemoryReprocessingJobStore); ok {
			_, err := s.enqueueSourceRevisionReprocessingJob(ctx, jobs, source, "worldline_source_recovery_resume", time.Now().UTC(), time.Now().UTC(), true)
			result["status"] = "queued_existing_reprocessing"
			return result, err
		}
	}
	processed := s.processAcceptedSourceRevision(ctx, source, s.completeTurnExtractionConfig(meta), true)
	result["status"], result["source_revision"] = processed.State, source.SourceRevision
	if processed.State != "completed" && processed.State != "skipped_ooc" {
		stored, _ := s.worldlineRecoverySource(context.WithoutCancel(ctx), plan.SessionID, plan.TurnIndex)
		if jobs, ok := s.Store.(store.MemoryReprocessingJobStore); ok && stored != nil && stored.CriticInputSnapshotJSON != "" {
			_, enqueueErr := s.enqueueSourceRevisionReprocessingJob(context.WithoutCancel(ctx), jobs, source, processed.Failure, time.Now().UTC(), time.Now().UTC().Add(processed.RetryDelay), true)
			if enqueueErr != nil {
				return result, enqueueErr
			}
		}
		return result, fmt.Errorf("worldline_source_critic: %s", processed.Failure)
	}
	return result, nil
}

// A parent's next-input marker can still be pending after its child recovered
// the selected response. Reuse that stored revision instead of interpreting
// this late delivery as a reroll. Actual edits/new assistant rows still use the
// existing replacement owner below handleCompleteTurn.
func (s *Server) reuseWorldlineRecoveredCompletion(ctx context.Context, w http.ResponseWriter, req dto.M4CompleteTurnRequest) bool {
	if lock, err := s.sessionMigrationSourceLock(ctx, req.ChatSessionID); err != nil || lock != nil {
		return false
	}
	source, err := s.worldlineRecoverySource(ctx, req.ChatSessionID, req.TurnIndex)
	if err != nil || source == nil || !strings.HasPrefix(source.SourceRevision, worldlineRecoveryRevisionPrefix) {
		return false
	}
	observation, err := completeTurnSourceObservationFromMeta(req.ClientMeta)
	if err != nil {
		return false
	}
	if completeTurnLogicalTurnID(req.ChatSessionID, observation) != source.LogicalTurnID {
		return false
	}
	if observation.MessageChatID != "" && observation.MessageChatID != source.SourceMessageID {
		return false
	}
	if req.UserInput == nil || req.AssistantContent == nil {
		return false
	}
	if sanitizeCriticStorageText(*req.UserInput) != source.UserContent || sanitizeCriticStorageText(*req.AssistantContent) != source.AssistantContent {
		return false
	}
	complete, err := s.adminRescanSourceProjectionComplete(ctx, source)
	if err != nil {
		return false
	}
	var job map[string]any
	if !complete {
		job = s.startWorldlineSourceRecovery(worldlineSourceRecoveryPlan{SessionID: source.ChatSessionID, TurnIndex: source.TurnIndex},
			worldlineSourceRecoveryObservation{UserContent: source.UserContent, AssistantContent: source.AssistantContent}, req.ClientMeta)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "save_ok": true, "chat_session_id": source.ChatSessionID,
		"turn_index": source.TurnIndex, "chat_logs_saved": 0, "critic_triggered": false, "derived_retry_required": !complete,
		"queue_action": "remove", "source_revision": source.SourceRevision, "worldline_recovery_job": job,
		"source_acceptance": map[string]any{"accepted": true, "reason": "worldline_recovered_source_reused", "queue_action": "remove"}})
	return true
}
