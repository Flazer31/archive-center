package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func statusSchemaProposalFromCreateRequest(req statusSchemaCreateRequest) (store.StatusSchemaProposal, error) {
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		return store.StatusSchemaProposal{}, errors.New("chat_session_id is required")
	}
	channel := statusSchemaNormalizeInputChannel(req.InputChannel)
	if channel == "" {
		return store.StatusSchemaProposal{}, errors.New("input_channel must be one of bootstrap, direct_json, portable_import")
	}
	schemaName := strings.TrimSpace(req.SchemaName)
	if schemaName == "" {
		return store.StatusSchemaProposal{}, errors.New("schema_name is required")
	}
	schemaJSON, err := statusSchemaCompactJSONObject(req.SchemaJSON, "schema_json")
	if err != nil {
		return store.StatusSchemaProposal{}, err
	}
	provenanceJSON, err := statusSchemaCompactOptionalJSONObject(req.ProvenanceJSON, "provenance_json")
	if err != nil {
		return store.StatusSchemaProposal{}, err
	}
	if channel == "portable_import" && provenanceJSON == "" {
		return store.StatusSchemaProposal{}, errors.New("provenance_json is required for portable_import")
	}
	return store.StatusSchemaProposal{
		ChatSessionID:  sid,
		InputChannel:   channel,
		ProposalState:  "pending_review",
		SchemaName:     schemaName,
		RulesetLabel:   strings.TrimSpace(req.RulesetLabel),
		SchemaJSON:     schemaJSON,
		ProvenanceJSON: provenanceJSON,
	}, nil
}

func statusSchemaProposalFromStore(item store.StatusSchemaProposal) statusSchemaProposalResponse {
	return statusSchemaProposalResponse{
		ID:             item.ID,
		ChatSessionID:  item.ChatSessionID,
		InputChannel:   item.InputChannel,
		ProposalState:  item.ProposalState,
		SchemaName:     item.SchemaName,
		RulesetLabel:   item.RulesetLabel,
		SchemaJSON:     item.SchemaJSON,
		ProvenanceJSON: item.ProvenanceJSON,
		ReviewNote:     item.ReviewNote,
		Reviewer:       item.Reviewer,
		ReviewedAt:     item.ReviewedAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func statusSchemaTruthBoundaryValue() statusSchemaTruthBoundary {
	return statusSchemaTruthBoundary{
		ProposalOnly:                 true,
		CanonicalStatusWriter:        false,
		ReviewRequiredBeforeCanon:    true,
		ApprovalRegistersSchema:      false,
		CurrentValueWritesAllowed:    false,
		EffectLifecycleWritesAllowed: false,
		ArbitraryCodeFormulaAllowed:  false,
		AcceptedInputChannels:        []string{"bootstrap", "direct_json", "portable_import"},
		AcceptedReviewStates:         []string{"approved", "rejected", "needs_revision"},
	}
}

func statusSchemaVectorPolicyValue() statusSchemaVectorPolicy {
	return statusSchemaVectorPolicy{
		ChromaLinked:          true,
		VectorLane:            "chroma_support_only",
		Tier:                  "status_schema_proposal",
		SourceTable:           "status_schema_proposals",
		HydrateRequired:       true,
		CanonicalTruthSource:  "mariadb.status_schema_proposals",
		TruthWriter:           false,
		IndexPendingProposals: true,
		IndexReviewedStates:   true,
	}
}

func statusSchemaRegistryPolicyValue() statusSchemaRegistryPolicy {
	return statusSchemaRegistryPolicy{
		CanonicalSchemaRegistry:     true,
		RequiresApprovedProposal:    true,
		CurrentValueWritesAllowed:   false,
		EffectLifecycleAllowed:      false,
		HardcodedStatusNamesAllowed: false,
		AcceptedOwnerScopes:         []string{"character", "party", "faction", "world", "entity", "session"},
		AcceptedValueKinds:          []string{"scalar", "resource", "enum", "boolean", "clock", "tags", "note", "derived"},
		VectorLane:                  "chroma_support_only",
		VectorTier:                  "status_schema_definition",
		VectorSourceTable:           "status_schema_registry",
		HydrateRequired:             true,
	}
}

func statusCurrentValuePolicyValue() statusCurrentValuePolicy {
	return statusCurrentValuePolicy{
		CanonicalCurrentValueWriter: true,
		RequiresActiveRegistry:      true,
		EvidenceRequired:            true,
		HistoryWritesAllowed:        false,
		EffectLifecycleAllowed:      false,
		VectorTruthWriter:           false,
		DirectDerivedWritesAllowed:  false,
		AcceptedOwnerScopes:         []string{"character", "party", "faction", "world", "entity", "session"},
		AcceptedValueKinds:          []string{"scalar", "resource", "enum", "boolean", "clock", "tags", "note"},
		CanonicalTruthSource:        "mariadb.status_current_values",
	}
}

func statusLifecyclePolicyValue() statusLifecyclePolicy {
	return statusLifecyclePolicy{
		ChangeEventLedgerWriter:      true,
		EffectLifecycleWriter:        true,
		RequiresActiveRegistry:       true,
		EvidenceRequired:             true,
		RequiresStoryClockForEffects: true,
		CurrentValueMutationAllowed:  false,
		VectorTruthWriter:            false,
		AcceptedEventKinds:           []string{"set", "change", "reaffirm", "reversal", "recovery", "correction", "reveal", "resolve", "uncertain", "clear", "event_observed", "increase", "decrease", "effect_applied", "effect_expired", "effect_cleared"},
		AcceptedEffectKinds:          []string{"temporary_effect", "buff", "debuff", "injury", "cooldown"},
		AcceptedEffectStates:         []string{"pending", "active", "expired", "cleared"},
		CanonicalEventSource:         "mariadb.status_change_events",
		CanonicalEffectSource:        "mariadb.status_effects",
	}
}

func statusQueryProjectionPolicyValue() statusQueryProjectionPolicy {
	return statusQueryProjectionPolicy{
		CanonFirstQuery:                   true,
		SemanticMemoryFallbackAsTruth:     false,
		ExternalRuntimeAuthoritySupported: true,
		ExternalRuntimeOverridesArchive:   true,
		UnknownStatusCreatesCanon:         false,
		UnknownStatusProposalOnly:         true,
		VectorTruthWriter:                 false,
		AcceptedAuthorityModes:            []string{"auto", "archive_canonical", "external_runtime"},
		AcceptedProjectionDensities:       []string{"auto", "full", "light"},
		CanonicalValueSource:              "mariadb.status_current_values",
		CanonicalEffectSource:             "mariadb.status_effects",
	}
}

func emptyStatusSchemaListResponse(sid, state string) statusSchemaListResponse {
	return statusSchemaListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaContractVersion,
		ChatSessionID:   sid,
		ProposalState:   state,
		Proposals:       []statusSchemaProposalResponse{},
		Counts:          statusSchemaCounts{},
		TruthBoundary:   statusSchemaTruthBoundaryValue(),
		VectorPolicy:    statusSchemaVectorPolicyValue(),
	}
}

func statusSchemaEmptyRegistryListResponse(sid, state string) statusSchemaRegistryListResponse {
	return statusSchemaRegistryListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaRegistryContractVersion,
		ChatSessionID:   sid,
		RegistryState:   state,
		Definitions:     []store.StatusSchemaDefinition{},
		Counts:          map[string]int{"total": 0},
		RegistryPolicy:  statusSchemaRegistryPolicyValue(),
	}
}

func statusCurrentValueEmptyListResponse(sid string) statusCurrentValueListResponse {
	return statusCurrentValueListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaValueContractVersion,
		ChatSessionID:   sid,
		Values:          []store.StatusCurrentValue{},
		Counts:          map[string]int{"total": 0},
		Policy:          statusCurrentValuePolicyValue(),
	}
}

func statusChangeEventEmptyListResponse(sid string) statusChangeEventListResponse {
	return statusChangeEventListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaLifecycleContractVersion,
		ChatSessionID:   sid,
		Events:          []store.StatusChangeEvent{},
		Counts:          map[string]int{"total": 0},
		Policy:          statusLifecyclePolicyValue(),
	}
}

func statusEffectEmptyListResponse(sid string) statusEffectListResponse {
	return statusEffectListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaLifecycleContractVersion,
		ChatSessionID:   sid,
		Effects:         []store.StatusEffect{},
		Counts:          map[string]int{"total": 0},
		Policy:          statusLifecyclePolicyValue(),
	}
}

func statusSchemaRegistryCounts(definitions []store.StatusSchemaDefinition) map[string]int {
	counts := map[string]int{"total": len(definitions)}
	for _, definition := range definitions {
		state := strings.TrimSpace(definition.RegistryState)
		if state == "" {
			state = "active"
		}
		counts[state]++
	}
	return counts
}

func statusCurrentValueCounts(values []store.StatusCurrentValue) map[string]int {
	counts := map[string]int{"total": len(values)}
	for _, value := range values {
		scope := strings.TrimSpace(value.OwnerScope)
		if scope == "" {
			scope = "unknown"
		}
		counts["owner_scope:"+scope]++
	}
	return counts
}

func statusChangeEventCounts(events []store.StatusChangeEvent) map[string]int {
	counts := map[string]int{"total": len(events)}
	for _, event := range events {
		kind := strings.TrimSpace(event.EventKind)
		if kind == "" {
			kind = "unknown"
		}
		counts["event_kind:"+kind]++
	}
	return counts
}

func statusEffectCounts(effects []store.StatusEffect) map[string]int {
	counts := map[string]int{"total": len(effects)}
	for _, effect := range effects {
		state := strings.TrimSpace(effect.EffectState)
		if state == "" {
			state = "active"
		}
		counts[state]++
	}
	return counts
}

func statusProjectionCounts(items []statusProjectionItem) map[string]int {
	counts := map[string]int{"total": len(items)}
	for _, item := range items {
		counts["density:"+item.Density]++
		counts["authority:"+item.AuthorityMode]++
		counts["source:"+item.ValueSource]++
	}
	return counts
}

func statusQueryResultState(items []statusProjectionItem) string {
	if len(items) == 0 {
		return "not_found"
	}
	hasAnswered := false
	hasExternalRequired := false
	hasMissingArchive := false
	for _, item := range items {
		switch item.ValueSource {
		case "archive_current", "external_runtime":
			hasAnswered = true
		case "external_value_required":
			hasExternalRequired = true
		case "archive_value_missing":
			hasMissingArchive = true
		}
	}
	if hasAnswered {
		return "answered"
	}
	if hasExternalRequired {
		return "external_value_required"
	}
	if hasMissingArchive {
		return "archive_value_missing"
	}
	return "not_found"
}

func statusQueryProposalGateResponse(sid string, req statusQueryRequest, reason string) statusQueryResponse {
	suggestedKey := statusSuggestedProposalKey(req)
	return statusQueryResponse{
		Status:          "ok",
		ContractVersion: statusSchemaQueryContractVersion,
		ChatSessionID:   sid,
		ResultState:     "proposal_required",
		Definitions:     []store.StatusSchemaDefinition{},
		Projection:      []statusProjectionItem{},
		ProposalGate: statusProposalGate{
			Required:              true,
			Reason:                reason,
			SuggestedStatusKey:    suggestedKey,
			ProposalOnly:          true,
			AutoCanonWriteAllowed: false,
			ProposalTemplate:      statusProposalTemplate(req, suggestedKey),
		},
		Policy: statusQueryProjectionPolicyValue(),
	}
}

func statusSuggestedProposalKey(req statusQueryRequest) string {
	if statusSchemaValidKey(strings.TrimSpace(req.StatusKey)) {
		return strings.TrimSpace(req.StatusKey)
	}
	for _, key := range req.CandidateKeys {
		if statusSchemaValidKey(strings.TrimSpace(key)) {
			return strings.TrimSpace(key)
		}
	}
	return ""
}

func statusProposalTemplate(req statusQueryRequest, key string) map[string]any {
	if key == "" {
		return nil
	}
	ownerScope := strings.TrimSpace(req.OwnerScope)
	if ownerScope == "" {
		ownerScope = "<review_required>"
	}
	return map[string]any{
		"input_channel":   "direct_json",
		"schema_name":     "status_schema",
		"review_required": true,
		"schema_json": map[string]any{
			"stats": []map[string]any{
				{
					"status_key":  key,
					"label":       key,
					"owner_scope": ownerScope,
					"value_kind":  "<review_required>",
					"note":        "Query saw an unregistered status key; review schema before import.",
				},
			},
		},
	}
}

func (s *Server) buildStatusProjection(ctx context.Context, sid string, definitions []store.StatusSchemaDefinition, ownerScope, ownerID, statusKey, requestedAuthority, requestedDensity string, externalValues []statusExternalRuntimeValue, queryFocused bool) []statusProjectionItem {
	values := statusLoadCurrentValues(ctx, s.Store, sid, ownerScope, ownerID, statusKey)
	effects := statusLoadActiveEffects(ctx, s.Store, sid, ownerScope, ownerID)
	external := statusNormalizeExternalValues(externalValues)
	out := make([]statusProjectionItem, 0, len(definitions))
	for _, definition := range definitions {
		authority := statusDefinitionAuthorityMode(definition, requestedAuthority)
		density := statusDefinitionProjectionDensity(definition, requestedDensity, queryFocused)
		if authority == "external_runtime" {
			matches := statusExternalValuesForDefinition(external, definition, ownerID)
			if len(matches) == 0 {
				if queryFocused {
					out = append(out, statusProjectionItem{
						Definition:     definition,
						AuthorityMode:  authority,
						ValueSource:    "external_value_required",
						Density:        density,
						ProjectionText: statusProjectionText(definition, nil, nil, nil, "external_value_required", density),
					})
				}
				continue
			}
			for _, ext := range matches {
				extCopy := ext
				item := statusProjectionItem{
					Definition:     definition,
					AuthorityMode:  authority,
					ValueSource:    "external_runtime",
					Density:        density,
					ProjectionText: statusProjectionText(definition, nil, &extCopy, nil, "external_runtime", density),
				}
				if density == "full" {
					item.ExternalRuntime = &extCopy
				}
				out = append(out, item)
			}
			continue
		}
		matchedValues := statusValuesForDefinition(values, definition, ownerID)
		if len(matchedValues) == 0 {
			if queryFocused {
				out = append(out, statusProjectionItem{
					Definition:     definition,
					AuthorityMode:  "archive_canonical",
					ValueSource:    "archive_value_missing",
					Density:        density,
					ProjectionText: statusProjectionText(definition, nil, nil, nil, "archive_value_missing", density),
				})
			}
			continue
		}
		for _, value := range matchedValues {
			valueCopy := value
			effectMatches := statusEffectsForValue(effects, valueCopy)
			item := statusProjectionItem{
				Definition:     definition,
				AuthorityMode:  "archive_canonical",
				ValueSource:    "archive_current",
				Density:        density,
				ProjectionText: statusProjectionText(definition, &valueCopy, nil, effectMatches, "archive_current", density),
			}
			if density == "full" {
				item.Value = &valueCopy
				item.Effects = effectMatches
			}
			out = append(out, item)
		}
	}
	return out
}

func statusLoadCurrentValues(ctx context.Context, st store.Store, sid, ownerScope, ownerID, statusKey string) []store.StatusCurrentValue {
	valueStore, ok := st.(store.StatusCurrentValueStore)
	if !ok {
		return nil
	}
	values, err := valueStore.ListStatusCurrentValues(ctx, sid, ownerScope, ownerID, statusKey, statusSchemaMaxListLimit)
	if err != nil {
		return nil
	}
	return values
}

func statusLoadActiveEffects(ctx context.Context, st store.Store, sid, ownerScope, ownerID string) []store.StatusEffect {
	lifecycle, ok := st.(store.StatusLifecycleStore)
	if !ok {
		return nil
	}
	effects, err := lifecycle.ListStatusEffects(ctx, sid, ownerScope, ownerID, "active", statusSchemaMaxListLimit)
	if err != nil {
		return nil
	}
	return effects
}

func statusMatchDefinitions(definitions []store.StatusSchemaDefinition, statusKey string, candidateKeys []string, queryText, ownerScope string) []store.StatusSchemaDefinition {
	if strings.TrimSpace(statusKey) != "" {
		return statusFilterDefinitions(definitions, statusKey, ownerScope)
	}
	candidates := map[string]bool{}
	for _, key := range candidateKeys {
		key = strings.TrimSpace(key)
		if statusSchemaValidKey(key) {
			candidates[strings.ToLower(key)] = true
		}
	}
	query := strings.ToLower(strings.TrimSpace(queryText))
	out := make([]store.StatusSchemaDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if ownerScope != "" && definition.OwnerScope != ownerScope {
			continue
		}
		if candidates[strings.ToLower(definition.StatusKey)] {
			out = append(out, definition)
			continue
		}
		if query != "" {
			key := strings.ToLower(strings.TrimSpace(definition.StatusKey))
			label := strings.ToLower(strings.TrimSpace(definition.Label))
			if (key != "" && strings.Contains(query, key)) || (label != "" && strings.Contains(query, label)) {
				out = append(out, definition)
			}
		}
	}
	return out
}

func statusFilterDefinitions(definitions []store.StatusSchemaDefinition, statusKey, ownerScope string) []store.StatusSchemaDefinition {
	out := make([]store.StatusSchemaDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if statusKey != "" && definition.StatusKey != statusKey {
			continue
		}
		if ownerScope != "" && definition.OwnerScope != ownerScope {
			continue
		}
		out = append(out, definition)
	}
	return out
}

func statusNormalizeExternalValues(values []statusExternalRuntimeValue) []statusExternalRuntimeProjection {
	out := make([]statusExternalRuntimeProjection, 0, len(values))
	for _, value := range values {
		statusKey := strings.TrimSpace(value.StatusKey)
		ownerScope := statusSchemaNormalizeOwnerScope(value.OwnerScope)
		ownerID := strings.TrimSpace(value.OwnerID)
		if !statusSchemaValidKey(statusKey) || ownerScope == "" || ownerID == "" {
			continue
		}
		valueJSON, err := statusSchemaCompactRawJSON(value.ValueJSON, "value_json")
		if err != nil {
			continue
		}
		evidenceJSON, _ := statusSchemaCompactOptionalRawJSON(value.EvidenceJSON, "evidence_json")
		out = append(out, statusExternalRuntimeProjection{
			StatusKey:    statusKey,
			OwnerScope:   ownerScope,
			OwnerID:      ownerID,
			ValueJSON:    valueJSON,
			EvidenceJSON: evidenceJSON,
			RuntimeName:  strings.TrimSpace(value.RuntimeName),
			UpdatedAt:    strings.TrimSpace(value.UpdatedAt),
		})
	}
	return out
}

func statusValuesForDefinition(values []store.StatusCurrentValue, definition store.StatusSchemaDefinition, ownerID string) []store.StatusCurrentValue {
	out := make([]store.StatusCurrentValue, 0, len(values))
	for _, value := range values {
		if value.StatusKey != definition.StatusKey || value.OwnerScope != definition.OwnerScope {
			continue
		}
		if ownerID != "" && value.OwnerID != ownerID {
			continue
		}
		out = append(out, value)
	}
	return out
}

func statusEffectsForValue(effects []store.StatusEffect, value store.StatusCurrentValue) []store.StatusEffect {
	out := make([]store.StatusEffect, 0, len(effects))
	for _, effect := range effects {
		if effect.StatusKey == value.StatusKey && effect.OwnerScope == value.OwnerScope && effect.OwnerID == value.OwnerID {
			out = append(out, effect)
		}
	}
	return out
}

func statusExternalValuesForDefinition(values []statusExternalRuntimeProjection, definition store.StatusSchemaDefinition, ownerID string) []statusExternalRuntimeProjection {
	out := make([]statusExternalRuntimeProjection, 0, len(values))
	for _, value := range values {
		if value.StatusKey != definition.StatusKey || value.OwnerScope != definition.OwnerScope {
			continue
		}
		if ownerID != "" && value.OwnerID != ownerID {
			continue
		}
		out = append(out, value)
	}
	return out
}

func statusDefinitionAuthorityMode(definition store.StatusSchemaDefinition, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested == "archive_canonical" || requested == "external_runtime" {
		return requested
	}
	options := statusDefinitionOptions(definition)
	if statusOptionBool(options, "external_runtime_authority") {
		return "external_runtime"
	}
	for _, key := range []string{"authority_mode", "value_authority", "runtime_authority"} {
		value := strings.ToLower(strings.TrimSpace(statusOptionString(options, key)))
		if value == "external_runtime" || value == "lua" || value == "lua_runtime" {
			return "external_runtime"
		}
	}
	return "archive_canonical"
}

func statusDefinitionProjectionDensity(definition store.StatusSchemaDefinition, requested string, queryFocused bool) string {
	requested = strings.TrimSpace(requested)
	if requested == "full" || requested == "light" {
		return requested
	}
	if queryFocused {
		return "full"
	}
	options := statusDefinitionOptions(definition)
	optionDensity := strings.ToLower(strings.TrimSpace(statusOptionString(options, "projection_density")))
	if optionDensity == "full" || optionDensity == "light" {
		return optionDensity
	}
	if statusOptionBool(options, "scene_blocking") || statusOptionBool(options, "critical") {
		return "full"
	}
	return "light"
}

func statusDefinitionOptions(definition store.StatusSchemaDefinition) map[string]any {
	raw := strings.TrimSpace(definition.OptionsJSON)
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func statusOptionString(options map[string]any, key string) string {
	if len(options) == 0 {
		return ""
	}
	switch value := options[key].(type) {
	case string:
		return value
	default:
		return ""
	}
}

func statusOptionBool(options map[string]any, key string) bool {
	if len(options) == 0 {
		return false
	}
	switch value := options[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
	default:
		return false
	}
}

func statusProjectionText(definition store.StatusSchemaDefinition, value *store.StatusCurrentValue, external *statusExternalRuntimeProjection, effects []store.StatusEffect, source, density string) string {
	key := strings.TrimSpace(definition.StatusKey)
	label := strings.TrimSpace(definition.Label)
	if label == "" {
		label = key
	}
	switch source {
	case "archive_current":
		if value == nil {
			return label + ": archive current value unavailable"
		}
		if density == "light" {
			return label + ": current value available; active_effects=" + strconv.Itoa(len(effects))
		}
		return label + ": " + value.ValueJSON + "; active_effects=" + strconv.Itoa(len(effects))
	case "external_runtime":
		if external == nil {
			return label + ": external runtime value unavailable"
		}
		if density == "light" {
			return label + ": external runtime value available"
		}
		return label + ": " + external.ValueJSON + " (external_runtime)"
	case "external_value_required":
		return label + ": delegated to external runtime; value not supplied"
	case "archive_value_missing":
		return label + ": registered but no archive current value"
	default:
		return label + ": status projection unavailable"
	}
}
