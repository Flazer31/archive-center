package httpapi

import (
	"strings"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

const (
	prepareRisuHostContextSnapshotVersion = "risu_host_context_snapshot.v1"
	prepareHostContextReferenceVersion    = "host_context_reference_evidence.v1"
	prepareOfficialRisuInspectionCommit   = "7bd1120a6ad6ef8e2e8d52fcd7c8a325b4b896af"
	prepareOfficialRisuInspectionDate     = "2026-07-23"
)

// buildPrepareTurnRisuHostContextSnapshot projects only the exact messages
// seen by Archive Center's official RisuAI beforeRequest hook. Official API v3
// exposes OpenAIChat role/content/name/function_call, but no lorebook,
// character-card, scenario, author-note, source ID, revision, activation, or
// visibility metadata. Those fields therefore remain not_exposed unless a
// future official shape supplies HostSource explicitly.
func buildPrepareTurnRisuHostContextSnapshot(request dto.PrepareTurnContractRequest, sessionID string) dto.PrepareTurnRisuHostContextSnapshotV1 {
	out := dto.PrepareTurnRisuHostContextSnapshotV1{
		ContractVersion:         prepareRisuHostContextSnapshotVersion,
		Status:                  "unavailable",
		ReasonCode:              "host_context_observation_missing",
		SessionID:               sessionID,
		ObservationStage:        "unobserved",
		FinalPayloadObservation: "not_exposed",
		SourceMetadataState:     "not_exposed",
		Spans:                   []dto.PrepareTurnHostContextSpanV1{},
		CanonicalWriteAllowed:   false,
	}
	host := request.HostObservations
	if host == nil {
		return out
	}
	out.ChatID = host.ChatID
	out.BranchID = host.BranchID
	out.RequestID = strings.TrimSpace(host.RequestID)
	out.PayloadPath = host.PayloadPath
	out.ObservationStage = strings.TrimSpace(host.PayloadObservationStage)
	if out.ObservationStage == "" {
		out.ObservationStage = "unobserved"
	}
	out.FinalPayloadObservation = strings.TrimSpace(host.FinalPayloadObservation)
	if out.FinalPayloadObservation == "" {
		out.FinalPayloadObservation = "not_exposed"
	}
	if strings.TrimSpace(host.ContractVersion) != prepareHostObservationsVersion || strings.TrimSpace(host.SessionID) != sessionID || out.RequestID == "" {
		out.ReasonCode = "host_context_observation_contract_incompatible"
		return out
	}

	malformed := 0
	metadataObserved := false
	for order, observation := range host.Payload {
		if strings.TrimSpace(observation.EvidenceState) != "observed" || observation.RawContent == nil {
			continue
		}
		text := *observation.RawContent
		hash := strings.TrimSpace(stringPtrValue(observation.ContentHash, ""))
		algorithm := strings.TrimSpace(stringPtrValue(observation.HashAlgorithm, ""))
		if hash == "" || algorithm != "or1c_utf16_djb2.v1" || hash != prepareOR1CHash(text) {
			malformed++
			continue
		}
		role := strings.ToLower(strings.TrimSpace(stringPtrValue(observation.Role, "")))
		eligibility, reason := prepareHostContextReferenceEligibility(role)
		span := dto.PrepareTurnHostContextSpanV1{
			ObservationRef:        strings.TrimSpace(observation.ObservationRef),
			PayloadPath:           host.PayloadPath,
			MessageOrder:          order,
			Role:                  role,
			SpanStart:             0,
			SpanEnd:               utf8.RuneCountInString(text),
			ExactText:             text,
			ContentHash:           hash,
			HashAlgorithm:         algorithm,
			HostSource:            clonePrepareTurnHostSource(observation.HostSource),
			SourceKindState:       prepareHostContextFieldState(observation.HostSource, "kind"),
			SourceIDState:         prepareHostContextFieldState(observation.HostSource, "id"),
			SourceScopeState:      prepareHostContextFieldState(observation.HostSource, "scope"),
			SourceRevisionState:   prepareHostContextFieldState(observation.HostSource, "revision"),
			ActivationState:       prepareHostContextFieldState(observation.HostSource, "activation"),
			VisibilityState:       prepareHostContextFieldState(observation.HostSource, "visibility"),
			NativePresent:         true,
			ReferenceEligibility:  eligibility,
			ReferenceReasonCode:   reason,
			CanonicalWriteAllowed: false,
		}
		if span.ObservationRef == "" {
			malformed++
			continue
		}
		if prepareHostContextSourceMetadataObserved(span.HostSource) {
			metadataObserved = true
		}
		if eligibility == "eligible" {
			out.ReferenceEligibleCount++
		}
		out.Spans = append(out.Spans, span)
	}
	out.ObservedSpanCount = len(out.Spans)
	if metadataObserved {
		out.SourceMetadataState = "partially_observed"
	}
	if malformed > 0 {
		out.Status = "degraded"
		out.ReasonCode = "host_context_observation_malformed_span_ignored"
		return out
	}
	if len(out.Spans) == 0 {
		out.Status = "empty"
		out.ReasonCode = "host_context_observation_empty"
		return out
	}
	if out.FinalPayloadObservation != "observed" {
		out.Status = "degraded"
		out.ReasonCode = "host_context_final_payload_not_exposed"
		return out
	}
	out.Status = "ready"
	out.ReasonCode = "host_context_observed"
	return out
}

func buildPrepareTurnHostContextReferenceEvidence(snapshot dto.PrepareTurnRisuHostContextSnapshotV1) dto.PrepareTurnHostContextReferenceEvidenceV1 {
	out := dto.PrepareTurnHostContextReferenceEvidenceV1{
		ContractVersion:         prepareHostContextReferenceVersion,
		Status:                  snapshot.Status,
		ReasonCode:              snapshot.ReasonCode,
		SessionID:               snapshot.SessionID,
		ChatID:                  snapshot.ChatID,
		BranchID:                snapshot.BranchID,
		RequestID:               snapshot.RequestID,
		SnapshotContractVersion: snapshot.ContractVersion,
		Items:                   []dto.PrepareTurnHostContextReferenceEvidenceItemV1{},
		Duplicates:              []dto.PrepareTurnHostContextDuplicateV1{},
		CanonicalWriteAllowed:   false,
	}
	type selectedSpan struct {
		index int
		refs  []string
	}
	seen := map[string]*selectedSpan{}
	groups := make([]*selectedSpan, 0)
	for _, span := range snapshot.Spans {
		if span.ReferenceEligibility != "eligible" {
			out.DeferredCount++
			continue
		}
		key := span.HashAlgorithm + "\x00" + span.ContentHash + "\x00" + span.ExactText
		if prior, ok := seen[key]; ok {
			prior.refs = append(prior.refs, span.ObservationRef)
			out.DuplicateCount++
			continue
		}
		sourceKind := "unknown"
		if span.HostSource != nil && span.HostSource.Kind != nil && strings.TrimSpace(*span.HostSource.Kind) != "" {
			sourceKind = strings.TrimSpace(*span.HostSource.Kind)
		}
		item := dto.PrepareTurnHostContextReferenceEvidenceItemV1{
			EvidenceRef:            "host-context:" + span.ObservationRef,
			ObservationRef:         span.ObservationRef,
			PayloadPath:            span.PayloadPath,
			MessageOrder:           span.MessageOrder,
			Role:                   span.Role,
			SpanStart:              span.SpanStart,
			SpanEnd:                span.SpanEnd,
			ExactText:              span.ExactText,
			ContentHash:            span.ContentHash,
			HashAlgorithm:          span.HashAlgorithm,
			HostSource:             clonePrepareTurnHostSource(span.HostSource),
			SourceKind:             sourceKind,
			SourceMetadataState:    prepareHostContextEvidenceMetadataState(span),
			Authority:              "request_scoped_reference_only",
			ConflictState:          "unobserved",
			ConflictReasonCode:     "semantic_conflict_not_inferred",
			SessionDivergenceState: "unobserved",
			CanonicalWriteAllowed:  false,
		}
		out.Items = append(out.Items, item)
		group := &selectedSpan{index: len(out.Items) - 1, refs: []string{span.ObservationRef}}
		seen[key] = group
		groups = append(groups, group)
	}
	for _, group := range groups {
		if len(group.refs) < 2 {
			continue
		}
		item := out.Items[group.index]
		out.Duplicates = append(out.Duplicates, dto.PrepareTurnHostContextDuplicateV1{
			KeptObservationRef:       group.refs[0],
			DuplicateObservationRefs: append([]string(nil), group.refs[1:]...),
			ContentHash:              item.ContentHash,
			ReasonCode:               "host_context_exact_span_duplicate",
		})
	}
	out.SelectedCount = len(out.Items)
	if out.Status == "empty" || out.Status == "unavailable" {
		return out
	}
	if len(out.Items) == 0 {
		out.Status = "empty"
		out.ReasonCode = "host_context_reference_evidence_empty"
	}
	return out
}

func prepareHostContextReferenceEligibility(role string) (string, string) {
	switch role {
	case "system":
		return "eligible", "observed_system_role_reference"
	case "user":
		return "excluded", "user_source_owned_by_current_input_contract"
	case "assistant":
		return "excluded", "assistant_source_owned_by_acceptance_lifecycle"
	case "function":
		return "deferred", "function_source_kind_not_exposed"
	default:
		return "deferred", "message_role_not_exposed"
	}
}

func prepareHostContextFieldState(source *dto.PrepareTurnHostSourceV1, field string) string {
	if source == nil {
		return "not_exposed"
	}
	var value *string
	switch field {
	case "kind":
		value = source.Kind
	case "id":
		value = source.ID
	case "scope":
		value = source.Scope
	case "revision":
		value = source.Revision
	case "activation":
		value = source.Activation
	case "visibility":
		value = source.Visibility
	}
	if value == nil || strings.TrimSpace(*value) == "" {
		return "not_exposed"
	}
	return "observed"
}

func prepareHostContextSourceMetadataObserved(source *dto.PrepareTurnHostSourceV1) bool {
	return source != nil && (prepareHostContextFieldState(source, "kind") == "observed" ||
		prepareHostContextFieldState(source, "id") == "observed" ||
		prepareHostContextFieldState(source, "scope") == "observed" ||
		prepareHostContextFieldState(source, "revision") == "observed" ||
		prepareHostContextFieldState(source, "activation") == "observed" ||
		prepareHostContextFieldState(source, "visibility") == "observed")
}

func prepareHostContextEvidenceMetadataState(span dto.PrepareTurnHostContextSpanV1) string {
	if prepareHostContextSourceMetadataObserved(span.HostSource) {
		return "partially_observed"
	}
	return "not_exposed"
}

func clonePrepareTurnHostSource(source *dto.PrepareTurnHostSourceV1) *dto.PrepareTurnHostSourceV1 {
	if source == nil {
		return nil
	}
	clone := *source
	return &clone
}
