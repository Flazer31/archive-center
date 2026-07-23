package httpapi

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	prepareHostObservationsVersion     = "prepare_host_observations.v1"
	prepareBootstrapObservationVersion = "session_bootstrap_observation.v1"
)

// prepareTurnInputContextChatLogs selects the completed logical turn directly
// before the current Host-observed user message. Host observations are the
// authoritative source when present; canonical chat logs are only a legacy
// fallback for requests that predate the source contract.
func prepareTurnInputContextChatLogs(request dto.PrepareTurnContractRequest, decision dto.PrepareTurnCurrentInputDecisionV1, stored []store.ChatLog) ([]store.ChatLog, string) {
	if request.HostObservations != nil && decision.Envelope != nil && decision.Envelope.Identity.MessageIndex != nil {
		currentIndex := *decision.Envelope.Identity.MessageIndex
		type indexedObservation struct {
			index   int
			role    string
			content string
		}
		observed := make([]indexedObservation, 0, len(request.HostObservations.ActiveChat))
		for _, item := range request.HostObservations.ActiveChat {
			if item.MessageIndex == nil || *item.MessageIndex >= currentIndex || item.RawContent == nil || item.Role == nil {
				continue
			}
			role := strings.ToLower(strings.TrimSpace(*item.Role))
			if role != "user" && role != "assistant" {
				continue
			}
			content := strings.TrimSpace(*item.RawContent)
			if content == "" {
				continue
			}
			observed = append(observed, indexedObservation{index: *item.MessageIndex, role: role, content: content})
		}
		sort.SliceStable(observed, func(i, j int) bool { return observed[i].index < observed[j].index })
		assistantAt := -1
		for i := len(observed) - 1; i >= 0; i-- {
			if observed[i].role == "assistant" {
				assistantAt = i
				break
			}
		}
		if assistantAt >= 0 {
			for i := assistantAt - 1; i >= 0; i-- {
				if observed[i].role != "user" {
					continue
				}
				return []store.ChatLog{
					{TurnIndex: observed[i].index, Role: "user", Content: observed[i].content},
					{TurnIndex: observed[i].index, Role: "assistant", Content: observed[assistantAt].content},
				}, "host_active_chat_previous_completed_turn"
			}
		}
		// An observed current Host position with no earlier completed pair is
		// authoritative emptiness. Falling back to the DB here could replay the
		// current turn after backfill or reroll.
		return nil, "host_active_chat_no_previous_completed_turn"
	}

	selected := selectRecentChatLogsByTurn(stored, 1)
	if len(selected) == 0 {
		return nil, "legacy_store_no_completed_turn"
	}
	return selected, "legacy_store_previous_turn"
}

const (
	prepareSourceObservationVersion     = "message_source_observation.v1"
	prepareCapabilityObservationVersion = "host_source_capabilities.v1"
	prepareSourceProjectionVersion      = "prepare_source_projection.v1"
	prepareSourceLaneStatusVersion      = "source_lane_status.v1"
	prepareSourceLane                   = "source_observation"
)

var prepareSourceRequiredCapabilities = []string{
	"message_position",
	"message_role",
	"request_correlation",
	"session_identity",
}

var prepareSourceOptionalCapabilities = []string{
	"raw_input_hash",
	"source_path",
}

func buildPrepareTurnSourceContract(
	request dto.PrepareTurnContractRequest,
	sessionID string,
) dto.PrepareTurnSourceContractProjectionV1 {
	coverage := prepareTurnCapabilityCoverage(request.CapabilityObservation)
	projection := dto.PrepareTurnSourceContractProjectionV1{
		ContractVersion: prepareSourceProjectionVersion,
		LaneStatus: dto.PrepareTurnLaneStatusV1{
			Status:                   "incompatible",
			ReasonCode:               "source_observation_contract_missing",
			Retryable:                false,
			AffectedLane:             prepareSourceLane,
			OriginalPayloadPreserved: true,
			ContractVersion:          prepareSourceLaneStatusVersion,
			CapabilityCoverage:       coverage,
		},
	}

	if observation := request.SourceObservation; observation != nil {
		projection.SourceContractVersion = strings.TrimSpace(observation.ContractVersion)
		projection.NormalizedSource = normalizePrepareTurnSourceObservation(observation)
		projection.LaneStatus.RequestCorrelationID = projection.NormalizedSource.RequestID
	}
	if capabilities := request.CapabilityObservation; capabilities != nil {
		projection.CapabilityContractVersion = strings.TrimSpace(capabilities.ContractVersion)
	}

	if request.SourceObservation == nil || request.CapabilityObservation == nil {
		return projection
	}
	if projection.SourceContractVersion != prepareSourceObservationVersion ||
		projection.CapabilityContractVersion != prepareCapabilityObservationVersion {
		projection.LaneStatus.ReasonCode = "source_observation_contract_incompatible"
		return projection
	}
	if projection.NormalizedSource != nil && projection.NormalizedSource.EvidenceState == "empty" {
		coverage.RequiredMissing = removeCapabilityNames(coverage.RequiredMissing, "message_position", "message_role")
		projection.LaneStatus.CapabilityCoverage = coverage
	}
	if len(coverage.RequiredMissing) > 0 {
		projection.LaneStatus.ReasonCode = "source_observation_required_capability_missing"
		return projection
	}

	normalized := projection.NormalizedSource
	if reason := validatePrepareTurnSourceObservation(normalized, sessionID, coverage); reason != "" {
		projection.LaneStatus.Status = "failed"
		projection.LaneStatus.ReasonCode = reason
		return projection
	}

	switch normalized.EvidenceState {
	case "not_applicable":
		projection.LaneStatus.Status = "not_applicable"
		projection.LaneStatus.ReasonCode = "source_observation_not_applicable"
		return projection
	case "empty":
		projection.LaneStatus.Status = "empty"
		projection.LaneStatus.ReasonCode = "source_observation_empty"
		return projection
	case "deferred":
		projection.LaneStatus.Status = "deferred"
		projection.LaneStatus.ReasonCode = "source_observation_deferred"
		projection.LaneStatus.Retryable = true
		return projection
	case "observed":
		if len(coverage.OptionalMissing) > 0 {
			projection.LaneStatus.Status = "degraded"
			projection.LaneStatus.ReasonCode = "source_observation_optional_capability_missing"
			return projection
		}
		projection.LaneStatus.Status = "eligible"
		projection.LaneStatus.ReasonCode = "source_observation_eligible"
		return projection
	default:
		projection.LaneStatus.Status = "failed"
		projection.LaneStatus.ReasonCode = "source_observation_evidence_state_invalid"
		return projection
	}
}

func prepareTurnCapabilityCoverage(observation *dto.PrepareTurnCapabilityObservationV1) dto.PrepareTurnCapabilityCoverageV1 {
	states := make(map[string]string)
	if observation != nil {
		for name, state := range observation.Capabilities {
			states[strings.TrimSpace(name)] = strings.TrimSpace(state)
		}
	}
	requiredMissing := missingObservedCapabilities(prepareSourceRequiredCapabilities, states)
	optionalMissing := missingObservedCapabilities(prepareSourceOptionalCapabilities, states)
	return dto.PrepareTurnCapabilityCoverageV1{
		Required:        append([]string(nil), prepareSourceRequiredCapabilities...),
		Optional:        append([]string(nil), prepareSourceOptionalCapabilities...),
		States:          states,
		RequiredMissing: requiredMissing,
		OptionalMissing: optionalMissing,
	}
}

func missingObservedCapabilities(names []string, states map[string]string) []string {
	missing := make([]string, 0)
	for _, name := range names {
		if states[name] != "observed" {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func normalizePrepareTurnSourceObservation(source *dto.PrepareTurnSourceObservationV1) *dto.PrepareTurnSourceObservationV1 {
	if source == nil {
		return nil
	}
	normalized := *source
	normalized.ContractVersion = strings.TrimSpace(source.ContractVersion)
	normalized.SessionID = strings.TrimSpace(source.SessionID)
	normalized.ChatID = trimmedStringPointer(source.ChatID)
	normalized.BranchID = trimmedStringPointer(source.BranchID)
	normalized.RequestID = trimmedStringPointer(source.RequestID)
	normalized.GenerationID = trimmedStringPointer(source.GenerationID)
	normalized.ObservedRole = trimmedStringPointer(source.ObservedRole)
	normalized.ObservedSourcePath = trimmedStringPointer(source.ObservedSourcePath)
	normalized.ObservedRevision = trimmedStringPointer(source.ObservedRevision)
	normalized.RawInputHash = trimmedStringPointer(source.RawInputHash)
	normalized.RawInputHashAlgorithm = trimmedStringPointer(source.RawInputHashAlgorithm)
	normalized.DisplayedOutputHash = trimmedStringPointer(source.DisplayedOutputHash)
	normalized.EvidenceState = strings.TrimSpace(source.EvidenceState)
	if source.HostSource != nil {
		hostSource := *source.HostSource
		hostSource.ID = trimmedStringPointer(source.HostSource.ID)
		hostSource.Scope = trimmedStringPointer(source.HostSource.Scope)
		hostSource.Revision = trimmedStringPointer(source.HostSource.Revision)
		normalized.HostSource = &hostSource
	}
	return &normalized
}

func validatePrepareTurnSourceObservation(
	source *dto.PrepareTurnSourceObservationV1,
	sessionID string,
	coverage dto.PrepareTurnCapabilityCoverageV1,
) string {
	if source == nil || source.Observable == nil {
		return "source_observation_malformed"
	}
	if source.SessionID == "" || source.SessionID != sessionID || source.RequestID == nil {
		return "source_observation_malformed"
	}
	if source.EvidenceState != "empty" && (source.MessageIndex == nil || *source.MessageIndex < 0 || source.ObservedRole == nil) {
		return "source_observation_malformed"
	}
	if source.EvidenceState == "observed" && !*source.Observable {
		return "source_observation_malformed"
	}
	if coverage.States["source_path"] == "observed" && source.ObservedSourcePath == nil {
		return "source_observation_malformed"
	}
	if coverage.States["raw_input_hash"] == "observed" && (source.RawInputHash == nil || source.RawInputHashAlgorithm == nil) {
		return "source_observation_malformed"
	}
	for _, state := range coverage.States {
		switch state {
		case "observed", "unavailable", "not_exposed", "degraded", "incompatible":
		default:
			return "source_observation_capability_state_invalid"
		}
	}
	return ""
}

func removeCapabilityNames(values []string, names ...string) []string {
	remove := make(map[string]bool, len(names))
	for _, name := range names {
		remove[name] = true
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !remove[value] {
			out = append(out, value)
		}
	}
	return out
}

func trimmedStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func buildPrepareTurnCurrentInputDecision(request dto.PrepareTurnContractRequest, sessionID string) (dto.PrepareTurnCurrentInputDecisionV1, bool) {
	decision := dto.PrepareTurnCurrentInputDecisionV1{
		ContractVersion:          "current_input_decision.v1",
		Status:                   "incompatible",
		ReasonCode:               "current_user_input_contract_missing",
		RequestOwnership:         "unknown",
		OriginalPayloadPreserved: true,
		CapabilityCoverage:       prepareTurnCapabilityCoverage(request.CapabilityObservation),
	}
	host := request.HostObservations
	if host == nil {
		return decision, request.SourceObservation != nil || request.CapabilityObservation != nil || request.BootstrapObservation != nil
	}
	if strings.TrimSpace(host.ContractVersion) != prepareHostObservationsVersion {
		decision.ReasonCode = "current_user_input_contract_incompatible"
		return decision, true
	}
	if strings.TrimSpace(host.SessionID) != sessionID || strings.TrimSpace(host.RequestID) == "" {
		decision.Status = "failed"
		decision.ReasonCode = "current_user_input_observation_malformed"
		return decision, true
	}
	requestType := strings.TrimSpace(host.RequestType)
	if requestType == "" && request.PrepareTurnRequest.RequestType != nil {
		requestType = strings.TrimSpace(*request.PrepareTurnRequest.RequestType)
	}
	if requestType != "model" {
		decision.Status = "not_applicable"
		decision.ReasonCode = "current_user_input_request_not_applicable"
		decision.RequestOwnership = "auxiliary"
		return decision, true
	}
	if host.InputHook != nil && strings.TrimSpace(host.InputHook.LifecycleKind) == "history_trim_command" {
		decision.Status = "not_applicable"
		decision.ReasonCode = "current_user_input_history_trim_command"
		decision.RequestOwnership = "host_lifecycle_command"
		return decision, true
	}

	selected, reason := selectPrepareTurnCurrentInput(host)
	if selected == nil {
		decision.RequestOwnership = "unverified"
		switch reason {
		case "current_user_input_absent":
			decision.Status = "absent"
		case "current_user_input_observation_unavailable", "current_user_input_active_chat_stale":
			decision.Status = "unverified"
		default:
			decision.Status = "failed"
		}
		decision.ReasonCode = reason
		return decision, true
	}
	if selected.EvidenceState == "empty" {
		decision.Status = "absent"
		decision.ReasonCode = "current_user_input_absent"
		decision.RequestOwnership = "main"
		return decision, true
	}
	if reason := validatePrepareMessageObservation(*selected, true); reason != "" {
		decision.Status = "failed"
		decision.ReasonCode = reason
		decision.RequestOwnership = "unverified"
		return decision, true
	}
	if strings.TrimSpace(selected.SourceKind) != "active_chat" || strings.TrimSpace(selected.ObservationStage) != "active_chat_stored_message" {
		decision.Status = "failed"
		decision.ReasonCode = "current_user_input_provenance_invalid"
		decision.RequestOwnership = "unverified"
		return decision, true
	}

	envelope := prepareMessageSourceEnvelope(sessionID, host, *selected, "user_input")
	decision.Status = "eligible"
	decision.ReasonCode = "current_user_input_observed"
	decision.RequestOwnership = "main"
	decision.EffectiveUserInput = envelope.RawContent
	decision.SelectedObservationRef = &envelope.ObservationRef
	decision.Envelope = &envelope
	payloadObserved, exactPayloadMatch := preparePayloadSupportsCurrentInput(*selected, host.Payload)
	decision.ContextInjectionEligible = host.PayloadWritable && payloadObserved
	if !decision.ContextInjectionEligible {
		decision.Status = "unverified"
		decision.ReasonCode = "current_user_input_payload_not_correlated"
		decision.RequestOwnership = "unverified"
		decision.EffectiveUserInput = ""
		return decision, true
	}
	if !exactPayloadMatch {
		decision.ReasonCode = "current_user_input_observed_with_host_transform"
	}
	decision.MemoryReadsAllowed = true
	return decision, true
}

func selectPrepareTurnCurrentInput(host *dto.PrepareTurnHostObservationsV1) (*dto.PrepareTurnMessageObservationV1, string) {
	if len(host.ActiveChat) == 0 {
		return nil, "current_user_input_observation_unavailable"
	}
	last := host.ActiveChat[len(host.ActiveChat)-1]
	if strings.TrimSpace(pointerString(last.Role)) != "user" {
		return nil, "current_user_input_active_chat_stale"
	}
	if strings.TrimSpace(last.EvidenceState) == "empty" {
		return &last, ""
	}
	if strings.TrimSpace(last.EvidenceState) != "observed" {
		return nil, "current_user_input_observation_unavailable"
	}
	return &last, ""
}

func validatePrepareMessageObservation(observation dto.PrepareTurnMessageObservationV1, requireUser bool) string {
	if strings.TrimSpace(observation.ObservationRef) == "" || strings.TrimSpace(observation.SourceKind) == "" || observation.RawContent == nil {
		return "current_user_input_observation_malformed"
	}
	if requireUser && strings.TrimSpace(pointerString(observation.Role)) != "user" {
		return "current_user_input_role_not_user"
	}
	if observation.ContentHash == nil || observation.HashAlgorithm == nil {
		return "current_user_input_observation_malformed"
	}
	if *observation.HashAlgorithm != "or1c_utf16_djb2.v1" || *observation.ContentHash != prepareOR1CHash(*observation.RawContent) {
		return "current_user_input_hash_mismatch"
	}
	return ""
}

func preparePayloadSupportsCurrentInput(selected dto.PrepareTurnMessageObservationV1, payload []dto.PrepareTurnMessageObservationV1) (bool, bool) {
	if selected.RawContent == nil {
		return false, false
	}
	observedUser := false
	for _, observation := range payload {
		if strings.TrimSpace(pointerString(observation.Role)) != "user" || observation.RawContent == nil {
			continue
		}
		if validatePrepareMessageObservation(observation, true) != "" {
			continue
		}
		observedUser = true
		if *observation.RawContent == *selected.RawContent {
			return true, true
		}
	}
	return observedUser, false
}

func buildPrepareTurnSessionBootstrap(request dto.PrepareTurnContractRequest, sessionID string) dto.PrepareTurnSessionBootstrapV1 {
	projection := dto.PrepareTurnSessionBootstrapV1{
		ContractVersion:          "session_bootstrap.v1",
		Status:                   "incompatible",
		ReasonCode:               "bootstrap_contract_missing",
		Sources:                  []dto.PrepareTurnMessageSourceEnvelopeV1{},
		PriorResponseSource:      dto.PrepareTurnPriorResponseSourceV1{Status: "absent", ReasonCode: "bootstrap_has_no_prior_active_final"},
		RetrievalCandidateStatus: "not_applicable",
		OriginalPayloadPreserved: true,
	}
	observation := request.BootstrapObservation
	if observation == nil {
		return projection
	}
	if strings.TrimSpace(observation.ContractVersion) != prepareBootstrapObservationVersion {
		projection.ReasonCode = "bootstrap_contract_incompatible"
		return projection
	}
	if strings.TrimSpace(observation.SessionID) != sessionID || strings.TrimSpace(observation.RequestID) == "" {
		projection.Status = "failed"
		projection.ReasonCode = "bootstrap_observation_malformed"
		return projection
	}
	switch strings.TrimSpace(observation.ObservationState) {
	case "unavailable":
		projection.Status = "unverified"
		projection.ReasonCode = "bootstrap_observation_unavailable"
		return projection
	case "empty":
		projection.Status = "empty"
		projection.ReasonCode = "bootstrap_sources_absent"
		return projection
	case "observed":
	default:
		projection.Status = "failed"
		projection.ReasonCode = "bootstrap_observation_malformed"
		return projection
	}
	host := &dto.PrepareTurnHostObservationsV1{
		SessionID: sessionID,
		ChatID:    observation.ChatID,
		BranchID:  observation.BranchID,
		RequestID: observation.RequestID,
	}
	for _, source := range observation.LeadingMessages {
		if validatePrepareMessageObservation(source, false) != "" || source.MessageIndex == nil {
			projection.Status = "failed"
			projection.ReasonCode = "bootstrap_observation_malformed"
			projection.Sources = []dto.PrepareTurnMessageSourceEnvelopeV1{}
			return projection
		}
		projection.Sources = append(projection.Sources, prepareMessageSourceEnvelope(sessionID, host, source, "session_bootstrap"))
	}
	if len(projection.Sources) == 0 && observation.SelectionExposed {
		selected := ""
		selectedRef := ""
		if observation.SelectedGreetingIndex != nil && *observation.SelectedGreetingIndex == -1 && observation.FirstGreeting != nil {
			selected = *observation.FirstGreeting
			selectedRef = "host_greeting:first"
		} else if observation.SelectedGreetingIndex != nil && *observation.SelectedGreetingIndex >= 0 && *observation.SelectedGreetingIndex < len(observation.AlternateGreetings) {
			selected = observation.AlternateGreetings[*observation.SelectedGreetingIndex]
			selectedRef = fmt.Sprintf("host_greeting:alternate:%d", *observation.SelectedGreetingIndex)
		} else if observation.SelectedGreetingIndex == nil && observation.FirstGreeting != nil {
			selected = *observation.FirstGreeting
			selectedRef = "host_greeting:first"
		}
		if selectedRef == "" {
			projection.Status = "unverified"
			projection.ReasonCode = "bootstrap_selected_greeting_unavailable"
			return projection
		}
		role := "assistant"
		hash := prepareOR1CHash(selected)
		algorithm := "or1c_utf16_djb2.v1"
		source := dto.PrepareTurnMessageObservationV1{
			ObservationRef: selectedRef,
			SourceKind:     "host_selected_greeting",
			Role:           &role,
			RawContent:     &selected,
			ContentHash:    &hash,
			HashAlgorithm:  &algorithm,
			EvidenceState:  map[bool]string{true: "observed", false: "empty"}[selected != ""],
		}
		projection.Sources = append(projection.Sources, prepareMessageSourceEnvelope(sessionID, host, source, "session_bootstrap"))
	}
	if len(projection.Sources) == 0 {
		projection.Status = "empty"
		projection.ReasonCode = "bootstrap_sources_absent"
		return projection
	}
	projection.Status = "preserved"
	projection.ReasonCode = "bootstrap_sources_preserved"
	return projection
}

func prepareMessageSourceEnvelope(sessionID string, host *dto.PrepareTurnHostObservationsV1, observation dto.PrepareTurnMessageObservationV1, origin string) dto.PrepareTurnMessageSourceEnvelopeV1 {
	role := strings.TrimSpace(pointerString(observation.Role))
	content := pointerString(observation.RawContent)
	hash := pointerString(observation.ContentHash)
	algorithm := pointerString(observation.HashAlgorithm)
	identitySeed := strings.Join([]string{sessionID, pointerString(host.ChatID), pointerString(host.BranchID), host.RequestID, pointerString(observation.MessageID), pointerString(observation.GenerationID), fmt.Sprint(pointerInt64(observation.MessageTime)), fmt.Sprint(pointerInt(observation.MessageIndex)), observation.ObservationRef, hash}, "\x1f")
	return dto.PrepareTurnMessageSourceEnvelopeV1{
		ContractVersion: "message_source_envelope.v1",
		BackendSourceID: "src_" + strings.TrimPrefix(prepareOR1CHash(identitySeed), "or1c_"),
		Identity:        dto.PrepareTurnMessageSourceIdentityV1{SessionID: sessionID, ChatID: host.ChatID, BranchID: host.BranchID, RequestID: stringPointer(host.RequestID), MessageID: observation.MessageID, GenerationID: observation.GenerationID, MessageTime: observation.MessageTime, MessageIndex: observation.MessageIndex},
		RawRole:         role, Channel: "unknown", RawContent: content, ContentHash: hash, HashAlgorithm: algorithm,
		ObservedAt: observation.ObservedAt, ObservedRevision: observation.ObservedRevision, SourceOrigin: origin, OriginProvenance: observation.SourceKind, ObservationRef: observation.ObservationRef,
		LifecycleObservation: dto.PrepareTurnSourceLifecycleV1{State: map[bool]string{true: "bootstrap", false: "observed"}[origin == "session_bootstrap"], Streaming: "unknown", FinalDisplay: "unknown"},
	}
}

func prepareOR1CHash(value string) string {
	if value == "" {
		return ""
	}
	var hash int64 = 5381
	for _, unit := range utf16.Encode([]rune(value)) {
		hash = ((hash << 5) + hash + int64(unit)) & 0x7fffffff
	}
	return "or1c_" + strings.ToLower(fmt.Sprintf("%s", base36(hash)))
}

func base36(value int64) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if value == 0 {
		return "0"
	}
	buf := make([]byte, 0, 16)
	for value > 0 {
		buf = append(buf, digits[value%36])
		value /= 36
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func pointerInt(value *int) int {
	if value == nil {
		return -1
	}
	return *value
}

func pointerInt64(value *int64) int64 {
	if value == nil {
		return -1
	}
	return *value
}
func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
