package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	turnWorkflowHUDContractVersion                  = "turn_workflow_hud.v2"
	turnWorkflowHUDNoticeObservationContractVersion = "turn_workflow_notice_observation.v1"
	turnWorkflowHUDEntryTTL                         = 15 * time.Minute
	turnWorkflowHUDMaxEntries                       = 512
	turnWorkflowHUDMaxWait                          = 20 * time.Second
)

const (
	turnWorkflowHUDSeverityNormal  = "normal"
	turnWorkflowHUDSeverityNotice  = "notice"
	turnWorkflowHUDSeverityWarning = "warning"
	turnWorkflowHUDSeverityError   = "error"

	turnWorkflowHUDDismissNone    = "none"
	turnWorkflowHUDDismissCardOrX = "card_or_x"
	turnWorkflowHUDDismissXOnly   = "x_only"
)

const (
	turnWorkflowStagePrepareSource  = "prepare_source"
	turnWorkflowStageRecall         = "recall_materialization"
	turnWorkflowStageContext        = "context_assembly"
	turnWorkflowStagePublisherLLM   = "publisher_llm"
	turnWorkflowStagePayload        = "payload_ready"
	turnWorkflowStageAwaitFinal     = "awaiting_final_output"
	turnWorkflowStageFinalAccepted  = "final_output_accepted"
	turnWorkflowStageRawPersist     = "raw_persist"
	turnWorkflowStageCriticLLM      = "critic_llm"
	turnWorkflowStageDerivedPersist = "derived_persist_and_index"
	turnWorkflowStageCheckpoints    = "summary_checkpoints"
	turnWorkflowStageComplete       = "complete"
)

type turnWorkflowHUDStageTemplate struct {
	Key      string
	LabelKey string
	LLMCall  bool
}

var turnWorkflowHUDStageTemplates = []turnWorkflowHUDStageTemplate{
	{Key: turnWorkflowStagePrepareSource, LabelKey: "turn_hud.stage.prepare_source"},
	{Key: turnWorkflowStageRecall, LabelKey: "turn_hud.stage.recall_materialization"},
	{Key: turnWorkflowStageContext, LabelKey: "turn_hud.stage.context_assembly"},
	{Key: turnWorkflowStagePublisherLLM, LabelKey: "turn_hud.stage.publisher_llm", LLMCall: true},
	{Key: turnWorkflowStagePayload, LabelKey: "turn_hud.stage.payload_ready"},
	{Key: turnWorkflowStageAwaitFinal, LabelKey: "turn_hud.stage.awaiting_final_output"},
	{Key: turnWorkflowStageFinalAccepted, LabelKey: "turn_hud.stage.final_output_accepted"},
	{Key: turnWorkflowStageRawPersist, LabelKey: "turn_hud.stage.raw_persist"},
	{Key: turnWorkflowStageCriticLLM, LabelKey: "turn_hud.stage.critic_llm", LLMCall: true},
	{Key: turnWorkflowStageDerivedPersist, LabelKey: "turn_hud.stage.derived_persist_and_index"},
	{Key: turnWorkflowStageCheckpoints, LabelKey: "turn_hud.stage.summary_checkpoints"},
	{Key: turnWorkflowStageComplete, LabelKey: "turn_hud.stage.complete"},
}

type turnWorkflowHUDCount struct {
	Key      string `json:"key"`
	LabelKey string `json:"label_key"`
	Value    int    `json:"value"`
}

var turnWorkflowHUDCountTemplates = []turnWorkflowHUDCount{
	{Key: "total_committed", LabelKey: "turn_hud.count.total_committed"},
	{Key: "raw_user", LabelKey: "turn_hud.count.raw_user"},
	{Key: "raw_assistant", LabelKey: "turn_hud.count.raw_assistant"},
	{Key: "effective_input", LabelKey: "turn_hud.count.effective_input"},
	{Key: "turn_summary", LabelKey: "turn_hud.count.turn_summary"},
	{Key: "precise_memory", LabelKey: "turn_hud.count.precise_memory"},
	{Key: "direct_evidence", LabelKey: "turn_hud.count.direct_evidence"},
	{Key: "relationship_knowledge", LabelKey: "turn_hud.count.relationship_knowledge"},
	{Key: "entity_identity", LabelKey: "turn_hud.count.entity_identity"},
	{Key: "identity_surface", LabelKey: "turn_hud.count.identity_surface"},
	{Key: "identity_binding", LabelKey: "turn_hud.count.identity_binding"},
	{Key: "speaker_attribution", LabelKey: "turn_hud.count.speaker_attribution"},
	{Key: "subjective_memory", LabelKey: "turn_hud.count.subjective_memory"},
	{Key: "world_rule", LabelKey: "turn_hud.count.world_rule"},
	{Key: "character_state", LabelKey: "turn_hud.count.character_state"},
	{Key: "narrative_state", LabelKey: "turn_hud.count.narrative_state"},
	{Key: "episode_summary", LabelKey: "turn_hud.count.episode_summary"},
	{Key: "vector_index", LabelKey: "turn_hud.count.vector_index"},
}

var turnWorkflowHUDFactTemplates = []turnWorkflowHUDFact{
	{Key: "host_observation", Owner: "risu_host", Scope: "current_request", Status: "unobserved", Severity: turnWorkflowHUDSeverityNormal},
	{Key: "backend_processing", Owner: "go_backend", Scope: "current_request", Status: "pending", Severity: turnWorkflowHUDSeverityNormal},
	{Key: "context_selection", Owner: "go_backend", Scope: "current_request", Status: "pending", Severity: turnWorkflowHUDSeverityNormal},
	{Key: "payload_delivery", Owner: "risu_host", Scope: "current_request", Status: "unobserved", Severity: turnWorkflowHUDSeverityNormal},
	{Key: "finality", Owner: "go_backend", Scope: "current_request", Status: "pending", Severity: turnWorkflowHUDSeverityNormal},
	{Key: "raw_persistence", Owner: "canonical_store", Scope: "current_turn", Status: "pending", Severity: turnWorkflowHUDSeverityNormal},
	{Key: "derived_memory", Owner: "canonical_store", Scope: "current_turn", Status: "pending", Severity: turnWorkflowHUDSeverityNormal},
	{Key: "vector_index", Owner: "vector_store", Scope: "current_turn", Status: "pending", Severity: turnWorkflowHUDSeverityNormal},
}

type turnWorkflowHUDStage struct {
	Key        string     `json:"key"`
	LabelKey   string     `json:"label_key"`
	Ordinal    int        `json:"ordinal"`
	Total      int        `json:"total"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	DurationMS int64      `json:"duration_ms"`
	ReasonCode string     `json:"reason_code,omitempty"`
	LLMCall    bool       `json:"llm_call"`
}

type turnWorkflowHUDNotice struct {
	Code       string `json:"code"`
	MessageKey string `json:"message_key"`
	StageKey   string `json:"stage_key,omitempty"`
}

type turnWorkflowHUDFact struct {
	Key         string `json:"key"`
	Owner       string `json:"owner"`
	Scope       string `json:"scope"`
	Status      string `json:"status"`
	Disposition string `json:"disposition,omitempty"`
	ReasonCode  string `json:"reason_code,omitempty"`
	Severity    string `json:"severity"`
	Count       *int   `json:"count,omitempty"`
}

type turnWorkflowHUDTurnAlignment struct {
	HostTurn    int    `json:"host_turn,omitempty"`
	BackendTurn int    `json:"backend_turn,omitempty"`
	State       string `json:"state"`
	ReasonCode  string `json:"reason_code"`
}

type turnWorkflowHUDError struct {
	Code            string                 `json:"code"`
	MessageKey      string                 `json:"message_key"`
	StageKey        string                 `json:"stage_key"`
	Retryable       bool                   `json:"retryable"`
	PreservedCounts []turnWorkflowHUDCount `json:"preserved_counts"`
}

type turnWorkflowHUDViewModel struct {
	ContractVersion  string                       `json:"contract_version"`
	RequestID        string                       `json:"request_id"`
	ChatSessionID    string                       `json:"chat_session_id"`
	LogicalTurn      int                          `json:"logical_turn"`
	HostTurn         int                          `json:"host_turn,omitempty"`
	BackendTurn      int                          `json:"backend_turn,omitempty"`
	TurnAlignment    turnWorkflowHUDTurnAlignment `json:"turn_alignment"`
	Attempt          int                          `json:"attempt"`
	Revision         int64                        `json:"revision"`
	Status           string                       `json:"status"`
	Severity         string                       `json:"severity"`
	DismissalPolicy  string                       `json:"dismissal_policy"`
	StartedAt        time.Time                    `json:"started_at"`
	UpdatedAt        time.Time                    `json:"updated_at"`
	EndedAt          *time.Time                   `json:"ended_at,omitempty"`
	CurrentStage     *turnWorkflowHUDStage        `json:"current_stage,omitempty"`
	Stages           []turnWorkflowHUDStage       `json:"stages"`
	Counts           []turnWorkflowHUDCount       `json:"counts"`
	Facts            []turnWorkflowHUDFact        `json:"facts"`
	Warnings         []turnWorkflowHUDNotice      `json:"warnings"`
	Error            *turnWorkflowHUDError        `json:"error,omitempty"`
	DisplayMode      string                       `json:"display_mode,omitempty"`
	TitleKey         string                       `json:"title_key,omitempty"`
	MessageKey       string                       `json:"message_key,omitempty"`
	NoticeCode       string                       `json:"notice_code,omitempty"`
	NoticeKind       string                       `json:"notice_kind,omitempty"`
	PresentationTone string                       `json:"presentation_tone,omitempty"`
}

type turnWorkflowHUDEntry struct {
	view       turnWorkflowHUDViewModel
	changed    chan struct{}
	attemptKey string
}

type turnWorkflowHUDNoticeObservation struct {
	ContractVersion string `json:"contract_version"`
	Kind            string `json:"kind"`
	RequestID       string `json:"request_id"`
	ChatSessionID   string `json:"chat_session_id"`
	HostTurn        int    `json:"host_turn,omitempty"`
}

type turnWorkflowHUDLedger struct {
	mu              sync.Mutex
	entries         map[string]*turnWorkflowHUDEntry
	activeBySession map[string]string
	latestByTurn    map[string]string
	attemptByTurn   map[string]int
	ttl             time.Duration
	maxEntries      int
}

func newTurnWorkflowHUDLedger() *turnWorkflowHUDLedger {
	return &turnWorkflowHUDLedger{
		entries:         map[string]*turnWorkflowHUDEntry{},
		activeBySession: map[string]string{},
		latestByTurn:    map[string]string{},
		attemptByTurn:   map[string]int{},
		ttl:             turnWorkflowHUDEntryTTL,
		maxEntries:      turnWorkflowHUDMaxEntries,
	}
}

func newTurnWorkflowHUDCounts() []turnWorkflowHUDCount {
	counts := make([]turnWorkflowHUDCount, len(turnWorkflowHUDCountTemplates))
	copy(counts, turnWorkflowHUDCountTemplates)
	return counts
}

func newTurnWorkflowHUDFacts() []turnWorkflowHUDFact {
	facts := make([]turnWorkflowHUDFact, len(turnWorkflowHUDFactTemplates))
	copy(facts, turnWorkflowHUDFactTemplates)
	return facts
}

func newTurnWorkflowHUDStages() []turnWorkflowHUDStage {
	total := len(turnWorkflowHUDStageTemplates)
	stages := make([]turnWorkflowHUDStage, 0, total)
	for index, item := range turnWorkflowHUDStageTemplates {
		stages = append(stages, turnWorkflowHUDStage{
			Key: item.Key, LabelKey: item.LabelKey, Ordinal: index + 1, Total: total,
			Status: "pending", LLMCall: item.LLMCall,
		})
	}
	return stages
}

func (l *turnWorkflowHUDLedger) begin(requestID, sessionID string, logicalTurn int) *turnWorkflowHUDViewModel {
	if l == nil {
		return nil
	}
	requestID = strings.TrimSpace(requestID)
	sessionID = strings.TrimSpace(sessionID)
	if requestID == "" || sessionID == "" {
		return nil
	}
	now := time.Now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)
	if existing := l.entries[requestID]; existing != nil {
		if logicalTurn > 0 && existing.view.LogicalTurn <= 0 {
			existing.view.LogicalTurn = logicalTurn
			existing.view.BackendTurn = logicalTurn
			syncTurnWorkflowHUDAlignment(&existing.view)
			l.touchLocked(existing, now)
		}
		snapshot := cloneTurnWorkflowHUDView(existing.view)
		return &snapshot
	}
	if previousID := l.activeBySession[sessionID]; previousID != "" && previousID != requestID {
		if previous := l.entries[previousID]; previous != nil && !turnWorkflowHUDTerminal(previous.view.Status) {
			l.invalidateLocked(previous, "superseded_by_new_request", now)
		}
	}
	l.ensureCapacityLocked(now)
	stages := newTurnWorkflowHUDStages()
	stages[0].Status = "running"
	stages[0].StartedAt = timePtr(now)
	entry := &turnWorkflowHUDEntry{
		view: turnWorkflowHUDViewModel{
			ContractVersion: turnWorkflowHUDContractVersion,
			RequestID:       requestID,
			ChatSessionID:   sessionID,
			LogicalTurn:     logicalTurn,
			BackendTurn:     logicalTurn,
			TurnAlignment: turnWorkflowHUDTurnAlignment{
				BackendTurn: logicalTurn,
				State:       "unobserved",
				ReasonCode:  "host_turn_unobserved",
			},
			Attempt:         1,
			Revision:        1,
			Status:          "running",
			Severity:        turnWorkflowHUDSeverityNormal,
			DismissalPolicy: turnWorkflowHUDDismissNone,
			StartedAt:       now,
			UpdatedAt:       now,
			Stages:          stages,
			Counts:          newTurnWorkflowHUDCounts(),
			Facts:           newTurnWorkflowHUDFacts(),
			Warnings:        []turnWorkflowHUDNotice{},
		},
		changed: make(chan struct{}),
	}
	entry.view.CurrentStage = cloneTurnWorkflowHUDStage(&entry.view.Stages[0])
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "backend_processing", Owner: "go_backend", Scope: "current_request",
		Status: "running", ReasonCode: "prepare_started", Severity: turnWorkflowHUDSeverityNormal,
	})
	l.entries[requestID] = entry
	l.resolveAttemptLocked(entry, now)
	l.activeBySession[sessionID] = requestID
	snapshot := cloneTurnWorkflowHUDView(entry.view)
	return &snapshot
}

func (l *turnWorkflowHUDLedger) setLogicalTurn(requestID string, logicalTurn int) {
	if l == nil || logicalTurn <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil {
		return
	}
	if entry.attemptKey != "" {
		return
	}
	entry.view.LogicalTurn = logicalTurn
	entry.view.BackendTurn = logicalTurn
	syncTurnWorkflowHUDAlignment(&entry.view)
	now := time.Now().UTC()
	l.resolveAttemptLocked(entry, now)
	l.touchLocked(entry, now)
}

func (l *turnWorkflowHUDLedger) setHostTurn(requestID string, hostTurn int, observed bool) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil {
		return
	}
	if observed && hostTurn > 0 {
		entry.view.HostTurn = hostTurn
	} else {
		entry.view.HostTurn = 0
	}
	syncTurnWorkflowHUDAlignment(&entry.view)
	l.touchLocked(entry, time.Now().UTC())
}

func (l *turnWorkflowHUDLedger) setFact(requestID string, fact turnWorkflowHUDFact) {
	if l == nil || strings.TrimSpace(fact.Key) == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil {
		return
	}
	for _, existing := range entry.view.Facts {
		if existing.Key == fact.Key && turnWorkflowHUDFactsEqual(existing, fact) {
			return
		}
	}
	setTurnWorkflowHUDFactValue(&entry.view, fact)
	syncTurnWorkflowHUDPresentation(&entry.view)
	l.touchLocked(entry, time.Now().UTC())
}

func turnWorkflowHUDFactsEqual(left, right turnWorkflowHUDFact) bool {
	if left.Key != right.Key ||
		left.Owner != right.Owner ||
		left.Scope != right.Scope ||
		left.Status != right.Status ||
		left.Disposition != right.Disposition ||
		left.ReasonCode != right.ReasonCode ||
		left.Severity != right.Severity {
		return false
	}
	if left.Count == nil || right.Count == nil {
		return left.Count == nil && right.Count == nil
	}
	return *left.Count == *right.Count
}

func (l *turnWorkflowHUDLedger) setPersistenceFacts(
	requestID string,
	rawStatus string,
	rawCount int,
	derivedStatus string,
	derivedCount int,
	vectorStatus string,
	vectorCount int,
) {
	if l == nil {
		return
	}
	for _, fact := range []turnWorkflowHUDFact{
		turnWorkflowHUDPersistenceFact("raw_persistence", "canonical_store", rawStatus, rawCount),
		turnWorkflowHUDPersistenceFact("derived_memory", "canonical_store", derivedStatus, derivedCount),
		turnWorkflowHUDPersistenceFact("vector_index", "vector_store", vectorStatus, vectorCount),
	} {
		l.setFact(requestID, fact)
	}
}

func (l *turnWorkflowHUDLedger) addNotice(requestID, code, messageKey, stageKey string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil || turnWorkflowHUDTerminal(entry.view.Status) {
		return
	}
	notice := turnWorkflowHUDNotice{
		Code: strings.TrimSpace(code), MessageKey: strings.TrimSpace(messageKey), StageKey: strings.TrimSpace(stageKey),
	}
	for _, existing := range entry.view.Warnings {
		if existing.Code == notice.Code && existing.StageKey == notice.StageKey {
			return
		}
	}
	entry.view.Warnings = append(entry.view.Warnings, notice)
	l.touchLocked(entry, time.Now().UTC())
}

func (l *turnWorkflowHUDLedger) startStage(requestID, stageKey string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil || turnWorkflowHUDTerminal(entry.view.Status) {
		return
	}
	now := time.Now().UTC()
	target := turnWorkflowHUDStageIndex(entry.view.Stages, stageKey)
	if target < 0 {
		return
	}
	for index := range entry.view.Stages {
		stage := &entry.view.Stages[index]
		if stage.Status == "running" && index != target {
			stage.Status = "succeeded"
			stage.EndedAt = timePtr(now)
			stage.DurationMS = turnWorkflowHUDDurationMS(stage.StartedAt, stage.EndedAt)
		}
	}
	stage := &entry.view.Stages[target]
	if stage.Status == "pending" {
		stage.Status = "running"
		stage.StartedAt = timePtr(now)
		stage.EndedAt = nil
		stage.DurationMS = 0
		stage.ReasonCode = ""
	}
	entry.view.Status = "running"
	entry.view.CurrentStage = cloneTurnWorkflowHUDStage(stage)
	l.touchLocked(entry, now)
}

func (l *turnWorkflowHUDLedger) finishStage(requestID, stageKey, status, reasonCode string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil || turnWorkflowHUDTerminal(entry.view.Status) {
		return
	}
	index := turnWorkflowHUDStageIndex(entry.view.Stages, stageKey)
	if index < 0 {
		return
	}
	now := time.Now().UTC()
	stage := &entry.view.Stages[index]
	if stage.StartedAt == nil {
		stage.StartedAt = timePtr(now)
	}
	switch status {
	case "succeeded", "skipped", "failed", "invalidated":
		stage.Status = status
	default:
		stage.Status = "succeeded"
	}
	stage.ReasonCode = strings.TrimSpace(reasonCode)
	stage.EndedAt = timePtr(now)
	stage.DurationMS = turnWorkflowHUDDurationMS(stage.StartedAt, stage.EndedAt)
	entry.view.CurrentStage = cloneTurnWorkflowHUDStage(stage)
	l.touchLocked(entry, now)
}

func (l *turnWorkflowHUDLedger) addWarning(requestID, code, messageKey, stageKey string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil || turnWorkflowHUDTerminal(entry.view.Status) {
		return
	}
	notice := turnWorkflowHUDNotice{
		Code: strings.TrimSpace(code), MessageKey: strings.TrimSpace(messageKey), StageKey: strings.TrimSpace(stageKey),
	}
	for _, existing := range entry.view.Warnings {
		if existing.Code == notice.Code && existing.StageKey == notice.StageKey {
			return
		}
	}
	entry.view.Warnings = append(entry.view.Warnings, notice)
	entry.view.Severity = turnWorkflowHUDSeverityWarning
	l.touchLocked(entry, time.Now().UTC())
}

func (l *turnWorkflowHUDLedger) awaitFinal(requestID string) {
	if l == nil {
		return
	}
	l.startStage(requestID, turnWorkflowStageAwaitFinal)
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil || turnWorkflowHUDTerminal(entry.view.Status) {
		return
	}
	entry.view.Status = "awaiting_final_output"
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "finality", Owner: "go_backend", Scope: "current_request",
		Status: "awaiting_final_output", Disposition: "deferred", ReasonCode: "awaiting_risu_host_final", Severity: turnWorkflowHUDSeverityNotice,
	})
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "backend_processing", Owner: "go_backend", Scope: "current_request",
		Status: "waiting", Disposition: "deferred", ReasonCode: "awaiting_risu_host_final", Severity: turnWorkflowHUDSeverityNotice,
	})
	if entry.view.Severity == turnWorkflowHUDSeverityNormal {
		entry.view.Severity = turnWorkflowHUDSeverityNotice
	}
	l.touchLocked(entry, time.Now().UTC())
}

func (l *turnWorkflowHUDLedger) fail(requestID, code, messageKey, stageKey string, retryable bool) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil || entry.view.Status == "failed" || entry.view.Status == "invalidated" {
		return
	}
	now := time.Now().UTC()
	index := turnWorkflowHUDStageIndex(entry.view.Stages, stageKey)
	if index >= 0 {
		stage := &entry.view.Stages[index]
		if stage.StartedAt == nil {
			stage.StartedAt = timePtr(now)
		}
		stage.Status = "failed"
		stage.ReasonCode = strings.TrimSpace(code)
		stage.EndedAt = timePtr(now)
		stage.DurationMS = turnWorkflowHUDDurationMS(stage.StartedAt, stage.EndedAt)
		entry.view.CurrentStage = cloneTurnWorkflowHUDStage(stage)
	}
	entry.view.Status = "failed"
	entry.view.Severity = turnWorkflowHUDSeverityError
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "backend_processing", Owner: "go_backend", Scope: "current_request",
		Status: "failed", Disposition: "dropped", ReasonCode: strings.TrimSpace(code), Severity: turnWorkflowHUDSeverityError,
	})
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "finality", Owner: "go_backend", Scope: "current_request",
		Status: "failed", Disposition: "dropped", ReasonCode: strings.TrimSpace(code), Severity: turnWorkflowHUDSeverityError,
	})
	entry.view.EndedAt = timePtr(now)
	entry.view.Error = &turnWorkflowHUDError{
		Code: strings.TrimSpace(code), MessageKey: strings.TrimSpace(messageKey), StageKey: strings.TrimSpace(stageKey),
		Retryable: retryable, PreservedCounts: cloneTurnWorkflowHUDCounts(entry.view.Counts),
	}
	if l.activeBySession[entry.view.ChatSessionID] == entry.view.RequestID {
		delete(l.activeBySession, entry.view.ChatSessionID)
	}
	l.touchLocked(entry, now)
}

func (l *turnWorkflowHUDLedger) invalidate(requestID, reasonCode string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil || turnWorkflowHUDTerminal(entry.view.Status) {
		return
	}
	l.invalidateLocked(entry, reasonCode, time.Now().UTC())
}

func (l *turnWorkflowHUDLedger) setCounts(requestID string, values map[string]int) {
	if l == nil || len(values) == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil {
		return
	}
	total := 0
	for index := range entry.view.Counts {
		count := &entry.view.Counts[index]
		if count.Key == "total_committed" {
			continue
		}
		if value, ok := values[count.Key]; ok {
			if value < 0 {
				value = 0
			}
			count.Value = value
		}
		total += count.Value
	}
	if index := turnWorkflowHUDCountIndex(entry.view.Counts, "total_committed"); index >= 0 {
		entry.view.Counts[index].Value = total
	}
	if entry.view.Error != nil {
		entry.view.Error.PreservedCounts = cloneTurnWorkflowHUDCounts(entry.view.Counts)
	}
	l.touchLocked(entry, time.Now().UTC())
}

func (l *turnWorkflowHUDLedger) complete(requestID string) {
	l.completeWithNotice(requestID, "", "", "")
}

func (l *turnWorkflowHUDLedger) completeWithNotice(requestID, titleKey, messageKey, noticeCode string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil || entry.view.Status == "failed" || entry.view.Status == "invalidated" {
		return
	}
	if strings.TrimSpace(titleKey) != "" || strings.TrimSpace(messageKey) != "" || strings.TrimSpace(noticeCode) != "" {
		entry.view.DisplayMode = "notice"
		entry.view.TitleKey = strings.TrimSpace(titleKey)
		entry.view.MessageKey = strings.TrimSpace(messageKey)
		entry.view.NoticeCode = strings.TrimSpace(noticeCode)
		entry.view.NoticeKind = turnWorkflowHUDNoticeKind(noticeCode)
		if entry.view.NoticeKind == "ooc" {
			entry.view.PresentationTone = "attention"
		}
		if entry.view.Severity == turnWorkflowHUDSeverityNormal {
			entry.view.Severity = turnWorkflowHUDSeverityNotice
		}
	}
	now := time.Now().UTC()
	for index := range entry.view.Stages {
		stage := &entry.view.Stages[index]
		if stage.Status == "running" {
			stage.Status = "succeeded"
			stage.EndedAt = timePtr(now)
			stage.DurationMS = turnWorkflowHUDDurationMS(stage.StartedAt, stage.EndedAt)
		}
	}
	index := turnWorkflowHUDStageIndex(entry.view.Stages, turnWorkflowStageComplete)
	if index >= 0 {
		stage := &entry.view.Stages[index]
		if stage.StartedAt == nil {
			stage.StartedAt = timePtr(now)
		}
		stage.Status = "succeeded"
		stage.EndedAt = timePtr(now)
		stage.DurationMS = turnWorkflowHUDDurationMS(stage.StartedAt, stage.EndedAt)
		entry.view.CurrentStage = cloneTurnWorkflowHUDStage(stage)
	}
	priorSeverity := normalizeTurnWorkflowHUDSeverity(entry.view.Severity)
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "backend_processing", Owner: "go_backend", Scope: "current_request",
		Status: "completed", Disposition: "delivered", ReasonCode: "workflow_completed", Severity: turnWorkflowHUDSeverityNormal,
	})
	finalityStatus := "accepted"
	finalityDisposition := "delivered"
	finalityReason := "active_final_accepted"
	finalitySeverity := turnWorkflowHUDSeverityNormal
	switch entry.view.NoticeKind {
	case "ooc":
		finalityStatus = "cancelled"
		finalityDisposition = "dropped"
		finalityReason = "ooc_input_cancelled"
		finalitySeverity = turnWorkflowHUDSeverityNotice
	case "duplicate":
		finalityStatus = "existing_preserved"
		finalityDisposition = "dropped"
		finalityReason = strings.ToLower(strings.TrimSpace(entry.view.NoticeCode))
		finalitySeverity = normalizeTurnWorkflowHUDSeverity(entry.view.Severity)
		if finalitySeverity == turnWorkflowHUDSeverityNormal {
			finalitySeverity = turnWorkflowHUDSeverityNotice
		}
	}
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "finality", Owner: "go_backend", Scope: "current_request",
		Status: finalityStatus, Disposition: finalityDisposition, ReasonCode: finalityReason, Severity: finalitySeverity,
	})
	completionSeverity := turnWorkflowHUDCompletionSeverity(entry.view, priorSeverity)
	entry.view.Status = "completed"
	entry.view.Severity = completionSeverity
	if completionSeverity == turnWorkflowHUDSeverityWarning {
		entry.view.Status = "completed_with_warning"
	}
	entry.view.EndedAt = timePtr(now)
	if l.activeBySession[entry.view.ChatSessionID] == entry.view.RequestID {
		delete(l.activeBySession, entry.view.ChatSessionID)
	}
	l.touchLocked(entry, now)
}

func (l *turnWorkflowHUDLedger) snapshot(requestID string) (turnWorkflowHUDViewModel, bool) {
	if l == nil {
		return turnWorkflowHUDViewModel{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(time.Now().UTC())
	entry := l.entries[strings.TrimSpace(requestID)]
	if entry == nil {
		return turnWorkflowHUDViewModel{}, false
	}
	return cloneTurnWorkflowHUDView(entry.view), true
}

func (s *Server) turnWorkflowHUDSnapshot(requestID string) any {
	if s == nil || s.TurnWorkflows == nil || strings.TrimSpace(requestID) == "" {
		return nil
	}
	view, ok := s.TurnWorkflows.snapshot(requestID)
	if !ok {
		return nil
	}
	return view
}

func (l *turnWorkflowHUDLedger) waitSnapshot(ctx context.Context, requestID string, afterRevision int64, wait time.Duration) (turnWorkflowHUDViewModel, bool) {
	if l == nil {
		return turnWorkflowHUDViewModel{}, false
	}
	if wait < 0 {
		wait = 0
	}
	if wait > turnWorkflowHUDMaxWait {
		wait = turnWorkflowHUDMaxWait
	}
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		l.mu.Lock()
		l.pruneLocked(time.Now().UTC())
		entry := l.entries[strings.TrimSpace(requestID)]
		if entry == nil {
			l.mu.Unlock()
			return turnWorkflowHUDViewModel{}, false
		}
		if entry.view.Revision > afterRevision || wait == 0 || turnWorkflowHUDTerminal(entry.view.Status) {
			snapshot := cloneTurnWorkflowHUDView(entry.view)
			l.mu.Unlock()
			return snapshot, true
		}
		changed := entry.changed
		l.mu.Unlock()
		select {
		case <-ctx.Done():
			return turnWorkflowHUDViewModel{}, false
		case <-deadline.C:
			return l.snapshot(requestID)
		case <-changed:
		}
	}
}

func (l *turnWorkflowHUDLedger) touchLocked(entry *turnWorkflowHUDEntry, now time.Time) {
	syncTurnWorkflowHUDPresentation(&entry.view)
	entry.view.Revision++
	entry.view.UpdatedAt = now
	close(entry.changed)
	entry.changed = make(chan struct{})
}

func (l *turnWorkflowHUDLedger) invalidateLocked(entry *turnWorkflowHUDEntry, reasonCode string, now time.Time) {
	if entry == nil || turnWorkflowHUDTerminal(entry.view.Status) {
		return
	}
	for index := range entry.view.Stages {
		stage := &entry.view.Stages[index]
		if stage.Status == "running" {
			stage.Status = "invalidated"
			stage.ReasonCode = strings.TrimSpace(reasonCode)
			stage.EndedAt = timePtr(now)
			stage.DurationMS = turnWorkflowHUDDurationMS(stage.StartedAt, stage.EndedAt)
			entry.view.CurrentStage = cloneTurnWorkflowHUDStage(stage)
		}
	}
	entry.view.Status = "invalidated"
	entry.view.Severity = turnWorkflowHUDSeverityWarning
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "finality", Owner: "go_backend", Scope: "current_request",
		Status: "invalidated", Disposition: "dropped", ReasonCode: strings.TrimSpace(reasonCode), Severity: turnWorkflowHUDSeverityWarning,
	})
	entry.view.EndedAt = timePtr(now)
	if l.activeBySession[entry.view.ChatSessionID] == entry.view.RequestID {
		delete(l.activeBySession, entry.view.ChatSessionID)
	}
	l.touchLocked(entry, now)
}

func (l *turnWorkflowHUDLedger) resolveAttemptLocked(entry *turnWorkflowHUDEntry, now time.Time) {
	if entry == nil || entry.view.LogicalTurn <= 0 || entry.attemptKey != "" {
		return
	}
	key := turnWorkflowHUDAttemptKey(entry.view.ChatSessionID, entry.view.LogicalTurn)
	if previousID := l.latestByTurn[key]; previousID != "" && previousID != entry.view.RequestID {
		l.supersedeAttemptLocked(l.entries[previousID], "superseded_by_new_attempt", now)
	}
	l.attemptByTurn[key]++
	entry.attemptKey = key
	entry.view.Attempt = l.attemptByTurn[key]
	l.latestByTurn[key] = entry.view.RequestID
}

func (l *turnWorkflowHUDLedger) supersedeAttemptLocked(entry *turnWorkflowHUDEntry, reasonCode string, now time.Time) {
	if entry == nil || entry.view.Status == "invalidated" {
		return
	}
	stageIndex := -1
	if entry.view.CurrentStage != nil {
		stageIndex = turnWorkflowHUDStageIndex(entry.view.Stages, entry.view.CurrentStage.Key)
	}
	if stageIndex >= 0 {
		stage := &entry.view.Stages[stageIndex]
		if stage.StartedAt == nil {
			stage.StartedAt = timePtr(now)
		}
		stage.Status = "invalidated"
		stage.ReasonCode = strings.TrimSpace(reasonCode)
		stage.EndedAt = timePtr(now)
		stage.DurationMS = turnWorkflowHUDDurationMS(stage.StartedAt, stage.EndedAt)
		entry.view.CurrentStage = cloneTurnWorkflowHUDStage(stage)
	}
	entry.view.Status = "invalidated"
	entry.view.Severity = turnWorkflowHUDSeverityWarning
	entry.view.Error = nil
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "backend_processing", Owner: "go_backend", Scope: "current_request",
		Status: "invalidated", Disposition: "dropped", ReasonCode: strings.TrimSpace(reasonCode), Severity: turnWorkflowHUDSeverityWarning,
	})
	setTurnWorkflowHUDFactValue(&entry.view, turnWorkflowHUDFact{
		Key: "finality", Owner: "go_backend", Scope: "current_request",
		Status: "invalidated", Disposition: "dropped", ReasonCode: strings.TrimSpace(reasonCode), Severity: turnWorkflowHUDSeverityWarning,
	})
	entry.view.EndedAt = timePtr(now)
	if l.activeBySession[entry.view.ChatSessionID] == entry.view.RequestID {
		delete(l.activeBySession, entry.view.ChatSessionID)
	}
	l.touchLocked(entry, now)
}

func turnWorkflowHUDAttemptKey(sessionID string, logicalTurn int) string {
	return strings.TrimSpace(sessionID) + "\x00" + strconv.Itoa(logicalTurn)
}

func (l *turnWorkflowHUDLedger) pruneLocked(now time.Time) {
	for requestID, entry := range l.entries {
		if !turnWorkflowHUDTerminal(entry.view.Status) && now.Sub(entry.view.UpdatedAt) > l.ttl {
			l.invalidateLocked(entry, "workflow_expired", now)
			continue
		}
		if turnWorkflowHUDTerminal(entry.view.Status) && now.Sub(entry.view.UpdatedAt) > l.ttl {
			l.deleteEntryLocked(requestID)
		}
	}
}

func (l *turnWorkflowHUDLedger) ensureCapacityLocked(now time.Time) {
	l.pruneLocked(now)
	for len(l.entries) >= l.maxEntries {
		oldestID := ""
		var oldest time.Time
		for requestID, entry := range l.entries {
			if !turnWorkflowHUDTerminal(entry.view.Status) {
				continue
			}
			if oldestID == "" || entry.view.UpdatedAt.Before(oldest) {
				oldestID = requestID
				oldest = entry.view.UpdatedAt
			}
		}
		if oldestID == "" {
			for requestID, entry := range l.entries {
				if oldestID == "" || entry.view.UpdatedAt.Before(oldest) {
					oldestID = requestID
					oldest = entry.view.UpdatedAt
				}
			}
		}
		if oldestID == "" {
			return
		}
		l.deleteEntryLocked(oldestID)
	}
}

func (l *turnWorkflowHUDLedger) deleteEntryLocked(requestID string) {
	entry := l.entries[requestID]
	if entry == nil {
		return
	}
	close(entry.changed)
	delete(l.entries, requestID)
	if l.activeBySession[entry.view.ChatSessionID] == requestID {
		delete(l.activeBySession, entry.view.ChatSessionID)
	}
	if entry.attemptKey != "" && l.latestByTurn[entry.attemptKey] == requestID {
		delete(l.latestByTurn, entry.attemptKey)
		delete(l.attemptByTurn, entry.attemptKey)
	}
}

func turnWorkflowHUDTerminal(status string) bool {
	switch status {
	case "completed", "completed_with_warning", "failed", "invalidated":
		return true
	default:
		return false
	}
}

func normalizeTurnWorkflowHUDSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case turnWorkflowHUDSeverityError, "fail", "failed":
		return turnWorkflowHUDSeverityError
	case turnWorkflowHUDSeverityWarning, "warn":
		return turnWorkflowHUDSeverityWarning
	case turnWorkflowHUDSeverityNotice, "info", "informational":
		return turnWorkflowHUDSeverityNotice
	default:
		return turnWorkflowHUDSeverityNormal
	}
}

func syncTurnWorkflowHUDPresentation(view *turnWorkflowHUDViewModel) {
	if view == nil {
		return
	}
	view.Severity = normalizeTurnWorkflowHUDSeverity(view.Severity)
	switch {
	case !turnWorkflowHUDTerminal(view.Status):
		view.DismissalPolicy = turnWorkflowHUDDismissNone
	case view.Severity == turnWorkflowHUDSeverityWarning || view.Severity == turnWorkflowHUDSeverityError:
		view.DismissalPolicy = turnWorkflowHUDDismissXOnly
	default:
		view.DismissalPolicy = turnWorkflowHUDDismissCardOrX
	}
}

func syncTurnWorkflowHUDAlignment(view *turnWorkflowHUDViewModel) {
	if view == nil {
		return
	}
	view.LogicalTurn = view.BackendTurn
	alignment := turnWorkflowHUDTurnAlignment{
		HostTurn: view.HostTurn, BackendTurn: view.BackendTurn,
		State: "unobserved", ReasonCode: "turn_alignment_observation_incomplete",
	}
	switch {
	case view.HostTurn <= 0:
		alignment.ReasonCode = "host_turn_unobserved"
	case view.BackendTurn <= 0:
		alignment.ReasonCode = "backend_turn_unobserved"
	case view.HostTurn == view.BackendTurn:
		alignment.State = "aligned"
		alignment.ReasonCode = "host_backend_turn_aligned"
	case view.HostTurn > view.BackendTurn:
		alignment.State = "host_ahead"
		alignment.ReasonCode = "host_turn_ahead_of_backend"
	default:
		alignment.State = "backend_ahead"
		alignment.ReasonCode = "backend_turn_ahead_of_host"
	}
	view.TurnAlignment = alignment
}

func setTurnWorkflowHUDFactValue(view *turnWorkflowHUDViewModel, fact turnWorkflowHUDFact) {
	if view == nil {
		return
	}
	fact.Key = strings.TrimSpace(fact.Key)
	if fact.Key == "" {
		return
	}
	if strings.TrimSpace(fact.Owner) == "" {
		fact.Owner = "go_backend"
	}
	if strings.TrimSpace(fact.Scope) == "" {
		fact.Scope = "current_request"
	}
	if strings.TrimSpace(fact.Status) == "" {
		fact.Status = "unobserved"
	}
	switch fact.Disposition {
	case "", "eligible", "selected", "delivered", "deferred", "dropped":
	default:
		fact.Disposition = "deferred"
		fact.ReasonCode = firstNonEmpty(fact.ReasonCode, "invalid_disposition_normalized")
	}
	fact.Severity = normalizeTurnWorkflowHUDSeverity(fact.Severity)
	if turnWorkflowHUDSeverityRank(fact.Severity) > turnWorkflowHUDSeverityRank(view.Severity) {
		view.Severity = fact.Severity
	}
	for index := range view.Facts {
		if view.Facts[index].Key == fact.Key {
			view.Facts[index] = fact
			return
		}
	}
	view.Facts = append(view.Facts, fact)
}

func turnWorkflowHUDSeverityRank(value string) int {
	switch normalizeTurnWorkflowHUDSeverity(value) {
	case turnWorkflowHUDSeverityError:
		return 3
	case turnWorkflowHUDSeverityWarning:
		return 2
	case turnWorkflowHUDSeverityNotice:
		return 1
	default:
		return 0
	}
}

func turnWorkflowHUDCompletionSeverity(view turnWorkflowHUDViewModel, priorSeverity string) string {
	severity := turnWorkflowHUDSeverityNormal
	if strings.TrimSpace(view.NoticeCode) != "" {
		severity = turnWorkflowHUDSeverityNotice
	}
	for _, fact := range view.Facts {
		if turnWorkflowHUDSeverityRank(fact.Severity) > turnWorkflowHUDSeverityRank(severity) {
			severity = normalizeTurnWorkflowHUDSeverity(fact.Severity)
		}
	}
	priorSeverity = normalizeTurnWorkflowHUDSeverity(priorSeverity)
	if priorSeverity == turnWorkflowHUDSeverityWarning || priorSeverity == turnWorkflowHUDSeverityError {
		if turnWorkflowHUDSeverityRank(priorSeverity) > turnWorkflowHUDSeverityRank(severity) {
			severity = priorSeverity
		}
	}
	if view.Error != nil {
		severity = turnWorkflowHUDSeverityError
	}
	return severity
}

func intValuePtr(value int) *int {
	out := value
	return &out
}

func turnWorkflowHUDPersistenceFact(key, owner, status string, count int) turnWorkflowHUDFact {
	status = strings.ToLower(strings.TrimSpace(status))
	fact := turnWorkflowHUDFact{
		Key: key, Owner: owner, Scope: "current_turn", Status: firstNonEmpty(status, "unobserved"),
		ReasonCode: firstNonEmpty(status, "persistence_status_unobserved"),
		Severity:   turnWorkflowHUDSeverityNormal,
		Count:      intValuePtr(maxInt(0, count)),
	}
	switch {
	case status == "ok" || status == "saved" || status == "upserted" || status == "empty" || status == "existing":
		fact.Disposition = "delivered"
	case status == "delayed" || status == "queued" || status == "pending" || status == "not_requested":
		fact.Disposition = "deferred"
		fact.Severity = turnWorkflowHUDSeverityNotice
	case status == "skipped":
		fact.Disposition = "dropped"
		fact.Severity = turnWorkflowHUDSeverityNotice
	case status == "vector_not_configured" || status == "missing_embedding_config" || status == "missing_config" ||
		status == "missing_embedding" || status == "empty_embedding" || status == "missing_source_row_id" ||
		status == "not_checked_no_raw":
		fact.Disposition = "dropped"
		fact.Severity = turnWorkflowHUDSeverityWarning
	case status == "error" || strings.HasPrefix(status, "error:") || status == "failed":
		fact.Disposition = "dropped"
		fact.Severity = turnWorkflowHUDSeverityError
	default:
		fact.Disposition = "deferred"
		fact.Severity = turnWorkflowHUDSeverityNotice
	}
	return fact
}

func turnWorkflowHUDStageIndex(stages []turnWorkflowHUDStage, key string) int {
	for index, stage := range stages {
		if stage.Key == key {
			return index
		}
	}
	return -1
}

func (l *turnWorkflowHUDLedger) latestSnapshotForSession(sessionID string) (turnWorkflowHUDViewModel, bool) {
	if l == nil || strings.TrimSpace(sessionID) == "" {
		return turnWorkflowHUDViewModel{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(time.Now().UTC())
	var latest *turnWorkflowHUDEntry
	for _, entry := range l.entries {
		if entry == nil || entry.view.ChatSessionID != strings.TrimSpace(sessionID) {
			continue
		}
		if latest == nil || entry.view.UpdatedAt.After(latest.view.UpdatedAt) {
			latest = entry
		}
	}
	if latest == nil {
		return turnWorkflowHUDViewModel{}, false
	}
	return cloneTurnWorkflowHUDView(latest.view), true
}

func (l *turnWorkflowHUDLedger) recordOperation(view turnWorkflowHUDViewModel) turnWorkflowHUDViewModel {
	if l == nil || strings.TrimSpace(view.RequestID) == "" || strings.TrimSpace(view.ChatSessionID) == "" {
		return view
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().UTC()
	l.pruneLocked(now)
	if existing := l.entries[view.RequestID]; existing != nil &&
		existing.view.ChatSessionID == view.ChatSessionID &&
		existing.view.NoticeCode == view.NoticeCode {
		return cloneTurnWorkflowHUDView(existing.view)
	}
	view = cloneTurnWorkflowHUDView(view)
	view.UpdatedAt = now
	entry := &turnWorkflowHUDEntry{view: view, changed: make(chan struct{})}
	l.entries[view.RequestID] = entry
	l.ensureCapacityLocked(now)
	return cloneTurnWorkflowHUDView(view)
}

func turnWorkflowHUDCountIndex(counts []turnWorkflowHUDCount, key string) int {
	for index, count := range counts {
		if count.Key == key {
			return index
		}
	}
	return -1
}

func cloneTurnWorkflowHUDView(source turnWorkflowHUDViewModel) turnWorkflowHUDViewModel {
	out := source
	out.Stages = append([]turnWorkflowHUDStage(nil), source.Stages...)
	out.Counts = cloneTurnWorkflowHUDCounts(source.Counts)
	out.Facts = append([]turnWorkflowHUDFact(nil), source.Facts...)
	for index := range out.Facts {
		if source.Facts[index].Count != nil {
			out.Facts[index].Count = intValuePtr(*source.Facts[index].Count)
		}
	}
	out.Warnings = append([]turnWorkflowHUDNotice(nil), source.Warnings...)
	out.CurrentStage = cloneTurnWorkflowHUDStage(source.CurrentStage)
	if source.Error != nil {
		errorCopy := *source.Error
		errorCopy.PreservedCounts = cloneTurnWorkflowHUDCounts(source.Error.PreservedCounts)
		out.Error = &errorCopy
	}
	return out
}

func cloneTurnWorkflowHUDCounts(source []turnWorkflowHUDCount) []turnWorkflowHUDCount {
	return append([]turnWorkflowHUDCount(nil), source...)
}

func cloneTurnWorkflowHUDStage(source *turnWorkflowHUDStage) *turnWorkflowHUDStage {
	if source == nil {
		return nil
	}
	out := *source
	return &out
}

func timePtr(value time.Time) *time.Time {
	out := value
	return &out
}

func turnWorkflowHUDDurationMS(startedAt, endedAt *time.Time) int64 {
	if startedAt == nil || endedAt == nil || endedAt.Before(*startedAt) {
		return 0
	}
	return endedAt.Sub(*startedAt).Milliseconds()
}

func (s *Server) handleTurnWorkflowHUDStatus(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "request_id is required")
		return
	}
	afterRevision, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("after_revision")), 10, 64)
	waitMS, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("wait_ms")))
	if waitMS < 0 {
		waitMS = 0
	}
	if waitMS > int(turnWorkflowHUDMaxWait/time.Millisecond) {
		waitMS = int(turnWorkflowHUDMaxWait / time.Millisecond)
	}
	if s.TurnWorkflows == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"contract_version": turnWorkflowHUDContractVersion,
			"status":           "unknown",
			"request_id":       requestID,
		})
		return
	}
	view, ok := s.TurnWorkflows.waitSnapshot(r.Context(), requestID, afterRevision, time.Duration(waitMS)*time.Millisecond)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"contract_version": turnWorkflowHUDContractVersion,
			"status":           "unknown",
			"request_id":       requestID,
		})
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleTurnWorkflowHUDNotice(w http.ResponseWriter, r *http.Request) {
	var observation turnWorkflowHUDNoticeObservation
	if err := json.NewDecoder(r.Body).Decode(&observation); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid turn workflow notice observation")
		return
	}
	if strings.TrimSpace(observation.ContractVersion) != turnWorkflowHUDNoticeObservationContractVersion {
		writeError(w, http.StatusBadRequest, "unsupported_contract", "unsupported turn workflow notice observation contract")
		return
	}
	if strings.TrimSpace(observation.RequestID) == "" || strings.TrimSpace(observation.ChatSessionID) == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "request_id and chat_session_id are required")
		return
	}
	if strings.TrimSpace(observation.Kind) != "ooc_input_cancelled" {
		writeError(w, http.StatusBadRequest, "unsupported_notice_kind", "unsupported turn workflow notice kind")
		return
	}
	view := newTurnWorkflowHUDOperationNotice(
		observation.RequestID,
		observation.ChatSessionID,
		0,
		"completed",
		turnWorkflowHUDSeverityNotice,
		"turn_hud.notice.ooc_recognized",
		"turn_hud.notice.ooc_recognized_detail",
		"OOC_INPUT_CANCELLED",
	)
	view.HostTurn = maxInt(0, observation.HostTurn)
	view.BackendTurn = 0
	setTurnWorkflowHUDFactValue(&view, turnWorkflowHUDFact{
		Key: "host_observation", Owner: "risu_host", Scope: "current_request",
		Status: "observed", Disposition: "dropped", ReasonCode: "ooc_input_cancelled", Severity: turnWorkflowHUDSeverityNotice,
	})
	setTurnWorkflowHUDFactValue(&view, turnWorkflowHUDFact{
		Key: "backend_processing", Owner: "go_backend", Scope: "current_request",
		Status: "skipped", Disposition: "dropped", ReasonCode: "ooc_input_cancelled", Severity: turnWorkflowHUDSeverityNotice,
	})
	setTurnWorkflowHUDFactValue(&view, turnWorkflowHUDFact{
		Key: "finality", Owner: "go_backend", Scope: "current_request",
		Status: "cancelled", Disposition: "dropped", ReasonCode: "ooc_input_cancelled", Severity: turnWorkflowHUDSeverityNotice,
	})
	syncTurnWorkflowHUDAlignment(&view)
	syncTurnWorkflowHUDPresentation(&view)
	if s.TurnWorkflows != nil {
		view = s.TurnWorkflows.recordOperation(view)
	}
	writeJSON(w, http.StatusOK, view)
}

func prepareTurnWorkflowRequestID(contract dto.PrepareTurnSourceContractProjectionV1, request dto.PrepareTurnContractRequest) string {
	if contract.LaneStatus.RequestCorrelationID != nil {
		if value := strings.TrimSpace(*contract.LaneStatus.RequestCorrelationID); value != "" {
			return value
		}
	}
	if request.HostObservations != nil {
		if value := strings.TrimSpace(request.HostObservations.RequestID); value != "" {
			return value
		}
	}
	return ""
}

func completeTurnWorkflowRequestID(req dto.M4CompleteTurnRequest) string {
	lineage := mapFromAny(req.ClientMeta["source_to_final_lineage_observation"])
	if value := strings.TrimSpace(extractionStringFromAny(lineage["archive_center_request_correlation_id"])); value != "" {
		return value
	}
	return strings.TrimSpace(extractionStringFromAny(req.ClientMeta["turn_workflow_request_id"]))
}

func resolvePrepareTurnWorkflowLogicalTurn(request dto.PrepareTurnContractRequest, decision dto.PrepareTurnCurrentInputDecisionV1, stored []store.ChatLog) int {
	if request.PrepareTurnRequest.TurnIndex != nil && *request.PrepareTurnRequest.TurnIndex > 0 {
		return *request.PrepareTurnRequest.TurnIndex
	}
	latestTurn := 0
	latestUser := ""
	for _, item := range stored {
		if item.TurnIndex > latestTurn {
			latestTurn = item.TurnIndex
			latestUser = ""
		}
		if item.TurnIndex == latestTurn && strings.EqualFold(strings.TrimSpace(item.Role), "user") {
			latestUser = strings.TrimSpace(item.Content)
		}
	}
	if hostTurn, observed := prepareTurnWorkflowHostOrdinal(request, decision); observed {
		switch {
		case latestTurn == 0:
			return maxInt(1, hostTurn)
		case hostTurn == latestTurn:
			return latestTurn
		case hostTurn == latestTurn+1:
			return latestTurn + 1
		case hostTurn > latestTurn+1:
			return hostTurn
		}
	}
	if latestTurn == 0 {
		return 1
	}
	rawInput := strings.TrimSpace(stringPtrValue(request.PrepareTurnRequest.RawUserInput, ""))
	if rawInput != "" && latestUser != "" && completeTurnComparableContentForRole("user", rawInput) == completeTurnComparableContentForRole("user", latestUser) {
		return latestTurn
	}
	return latestTurn + 1
}

func prepareTurnWorkflowHostOrdinal(request dto.PrepareTurnContractRequest, decision dto.PrepareTurnCurrentInputDecisionV1) (int, bool) {
	host := request.HostObservations
	if host == nil || decision.Envelope == nil || decision.Envelope.Identity.MessageIndex == nil {
		return 0, false
	}
	currentIndex := *decision.Envelope.Identity.MessageIndex
	type observedRole struct {
		index int
		role  string
	}
	items := make([]observedRole, 0, len(host.ActiveChat))
	for _, item := range host.ActiveChat {
		if item.MessageIndex == nil || *item.MessageIndex > currentIndex || item.Role == nil {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(*item.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		items = append(items, observedRole{index: *item.MessageIndex, role: role})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].index < items[j].index })
	completed := 0
	userOpen := false
	currentObserved := false
	for _, item := range items {
		if item.index == currentIndex && item.role == "user" {
			currentObserved = true
			break
		}
		switch item.role {
		case "user":
			userOpen = true
		case "assistant":
			if userOpen {
				completed++
				userOpen = false
			}
		}
	}
	if !currentObserved {
		return 0, false
	}
	return completed + 1, true
}

func turnWorkflowHUDCountsFromComplete(
	rawUserDurable bool,
	rawAssistantDurable bool,
	effectiveInputSaved int,
	memoriesSaved int,
	preciseMemoryUnitsSaved int,
	evidenceSaved int,
	kgTriplesSaved int,
	subjectiveEntityMemoriesSaved int,
	worldRulesSaved int,
	characterStatesSaved int,
	physicalConditionsSaved int,
	entityConditionsSaved int,
	statusSchemaDefinitionsSaved int,
	statusEffectsSaved int,
	characterEventsSaved int,
	storylinesSaved int,
	narrativeCurrentStatesSaved int,
	narrativeStateEventsSaved int,
	pendingThreadsSaved int,
	activeStatesSaved int,
	canonicalStateLayersSaved int,
	entitiesSaved int,
	trustStatesSaved int,
	entityIdentitiesSaved int,
	identitySurfacesSaved int,
	identityBindingsSaved int,
	speakerAttributionsSaved int,
	episodeSummariesSaved int,
	vectorsUpserted int,
) map[string]int {
	boolCount := func(value bool) int {
		if value {
			return 1
		}
		return 0
	}
	return map[string]int{
		"raw_user":               boolCount(rawUserDurable),
		"raw_assistant":          boolCount(rawAssistantDurable),
		"effective_input":        maxInt(0, effectiveInputSaved),
		"turn_summary":           maxInt(0, memoriesSaved),
		"precise_memory":         maxInt(0, preciseMemoryUnitsSaved),
		"direct_evidence":        maxInt(0, evidenceSaved),
		"relationship_knowledge": maxInt(0, kgTriplesSaved),
		"entity_identity":        maxInt(0, entityIdentitiesSaved),
		"identity_surface":       maxInt(0, identitySurfacesSaved),
		"identity_binding":       maxInt(0, identityBindingsSaved),
		"speaker_attribution":    maxInt(0, speakerAttributionsSaved),
		"subjective_memory":      maxInt(0, subjectiveEntityMemoriesSaved),
		"world_rule":             maxInt(0, worldRulesSaved),
		"character_state": maxInt(0,
			characterStatesSaved+physicalConditionsSaved+entityConditionsSaved+statusSchemaDefinitionsSaved+statusEffectsSaved),
		"narrative_state": maxInt(0,
			characterEventsSaved+storylinesSaved+narrativeCurrentStatesSaved+narrativeStateEventsSaved+
				pendingThreadsSaved+activeStatesSaved+canonicalStateLayersSaved+entitiesSaved+trustStatesSaved),
		"episode_summary": maxInt(0, episodeSummariesSaved),
		"vector_index":    maxInt(0, vectorsUpserted),
	}
}

func (s *Server) completeTurnWorkflowHUDDuplicate(
	requestID string,
	sessionID string,
	logicalTurn int,
	reasonCode string,
	warningCode string,
	warningMessageKey string,
	noticeMessageKey string,
) any {
	requestID = strings.TrimSpace(requestID)
	if s == nil || s.TurnWorkflows == nil || requestID == "" {
		return nil
	}
	if _, ok := s.TurnWorkflows.snapshot(requestID); !ok {
		s.TurnWorkflows.begin(requestID, strings.TrimSpace(sessionID), logicalTurn)
	}
	s.TurnWorkflows.setLogicalTurn(requestID, logicalTurn)
	s.TurnWorkflows.finishStage(requestID, turnWorkflowStageFinalAccepted, "succeeded", "")
	s.TurnWorkflows.finishStage(requestID, turnWorkflowStageRawPersist, "skipped", reasonCode)
	s.TurnWorkflows.finishStage(requestID, turnWorkflowStageCriticLLM, "skipped", reasonCode)
	s.TurnWorkflows.finishStage(requestID, turnWorkflowStageDerivedPersist, "skipped", reasonCode)
	s.TurnWorkflows.finishStage(requestID, turnWorkflowStageCheckpoints, "skipped", reasonCode)
	s.TurnWorkflows.setCounts(requestID, turnWorkflowHUDCountsFromComplete(
		false, false, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	))
	s.TurnWorkflows.setPersistenceFacts(requestID, "existing", 0, "existing", 0, "not_requested", 0)
	if strings.Contains(strings.ToUpper(strings.TrimSpace(warningCode)), "CONFLICT") {
		s.TurnWorkflows.addWarning(requestID, warningCode, warningMessageKey, turnWorkflowStageRawPersist)
	} else {
		s.TurnWorkflows.addNotice(requestID, warningCode, warningMessageKey, turnWorkflowStageRawPersist)
	}
	s.TurnWorkflows.completeWithNotice(
		requestID,
		"turn_hud.notice.duplicate_suspected",
		noticeMessageKey,
		warningCode,
	)
	return s.turnWorkflowHUDSnapshot(requestID)
}

func newTurnWorkflowHUDOperationNotice(
	requestID string,
	sessionID string,
	logicalTurn int,
	status string,
	severity string,
	titleKey string,
	messageKey string,
	noticeCode string,
) turnWorkflowHUDViewModel {
	now := time.Now().UTC()
	view := turnWorkflowHUDViewModel{
		ContractVersion: turnWorkflowHUDContractVersion,
		RequestID:       strings.TrimSpace(requestID),
		ChatSessionID:   strings.TrimSpace(sessionID),
		LogicalTurn:     logicalTurn,
		BackendTurn:     logicalTurn,
		Attempt:         1,
		Revision:        1,
		Status:          strings.TrimSpace(status),
		Severity:        normalizeTurnWorkflowHUDSeverity(severity),
		StartedAt:       now,
		UpdatedAt:       now,
		EndedAt:         timePtr(now),
		Stages:          []turnWorkflowHUDStage{},
		Counts:          []turnWorkflowHUDCount{},
		Warnings:        []turnWorkflowHUDNotice{},
		Facts:           newTurnWorkflowHUDFacts(),
		DisplayMode:     "notice",
		TitleKey:        strings.TrimSpace(titleKey),
		MessageKey:      strings.TrimSpace(messageKey),
		NoticeCode:      strings.TrimSpace(noticeCode),
		NoticeKind:      turnWorkflowHUDNoticeKind(noticeCode),
	}
	if view.NoticeKind == "ooc" {
		view.PresentationTone = "attention"
	}
	operationDisposition := "delivered"
	operationStatus := "completed"
	operationSeverity := view.Severity
	if view.Status == "failed" || view.Severity == turnWorkflowHUDSeverityError {
		operationDisposition = "dropped"
		operationStatus = "failed"
		operationSeverity = turnWorkflowHUDSeverityError
	}
	setTurnWorkflowHUDFactValue(&view, turnWorkflowHUDFact{
		Key: "backend_processing", Owner: "go_backend", Scope: "current_request",
		Status: operationStatus, Disposition: operationDisposition, ReasonCode: strings.TrimSpace(noticeCode), Severity: operationSeverity,
	})
	setTurnWorkflowHUDFactValue(&view, turnWorkflowHUDFact{
		Key: "finality", Owner: "go_backend", Scope: "current_request",
		Status: operationStatus, Disposition: operationDisposition, ReasonCode: strings.TrimSpace(noticeCode), Severity: operationSeverity,
	})
	switch view.NoticeKind {
	case "ooc":
		setTurnWorkflowHUDFactValue(&view, turnWorkflowHUDFact{
			Key: "finality", Owner: "go_backend", Scope: "current_request",
			Status: "cancelled", Disposition: "dropped", ReasonCode: "ooc_input_cancelled", Severity: turnWorkflowHUDSeverityNotice,
		})
		for _, fact := range []turnWorkflowHUDFact{
			turnWorkflowHUDPersistenceFact("raw_persistence", "canonical_store", "skipped", 0),
			turnWorkflowHUDPersistenceFact("derived_memory", "canonical_store", "skipped", 0),
			turnWorkflowHUDPersistenceFact("vector_index", "vector_store", "not_requested", 0),
		} {
			setTurnWorkflowHUDFactValue(&view, fact)
		}
	case "delete":
		setTurnWorkflowHUDFactValue(&view, turnWorkflowHUDFact{
			Key: "host_observation", Owner: "risu_host", Scope: "current_request",
			Status: "observed", Disposition: "eligible", ReasonCode: strings.ToLower(strings.TrimSpace(noticeCode)), Severity: operationSeverity,
		})
		setTurnWorkflowHUDFactValue(&view, turnWorkflowHUDFact{
			Key: "finality", Owner: "go_backend", Scope: "current_request",
			Status: "deleted", Disposition: "dropped", ReasonCode: strings.ToLower(strings.TrimSpace(noticeCode)), Severity: operationSeverity,
		})
	}
	syncTurnWorkflowHUDAlignment(&view)
	syncTurnWorkflowHUDPresentation(&view)
	if view.Status == "failed" || view.Severity == turnWorkflowHUDSeverityError {
		view.Status = "failed"
		view.Severity = turnWorkflowHUDSeverityError
		view.Error = &turnWorkflowHUDError{
			Code:            view.NoticeCode,
			MessageKey:      view.MessageKey,
			StageKey:        "",
			Retryable:       true,
			PreservedCounts: []turnWorkflowHUDCount{},
		}
	}
	syncTurnWorkflowHUDPresentation(&view)
	return view
}

func (s *Server) turnWorkflowHUDOperationNotice(
	requestID string,
	sessionID string,
	logicalTurn int,
	status string,
	severity string,
	titleKey string,
	messageKey string,
	noticeCode string,
) turnWorkflowHUDViewModel {
	view := newTurnWorkflowHUDOperationNotice(
		requestID,
		sessionID,
		logicalTurn,
		status,
		severity,
		titleKey,
		messageKey,
		noticeCode,
	)
	if s != nil && s.TurnWorkflows != nil {
		view = s.TurnWorkflows.recordOperation(view)
	}
	return view
}

func turnWorkflowHUDNoticeKind(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "OOC_INPUT_CANCELLED", "OOC_TURN_SKIPPED":
		return "ooc"
	case "ASSISTANT_OUTPUT_DELETE_CONFIRMED", "ASSISTANT_OUTPUT_DELETE_SYNC_PARTIAL":
		return "delete"
	case "LOGICAL_TURN_REPLACED":
		return "reroll"
	case "DUPLICATE_PAIR_REPLAY", "DUPLICATE_EXISTING_PAIR":
		return "duplicate"
	default:
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(code)), "DUPLICATE_") {
			return "duplicate"
		}
		return "operation"
	}
}
