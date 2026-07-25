package httpapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	sourceToPayloadLineageContract          = "source_to_payload_lineage.v1"
	payloadApplicationObservationContract   = "payload_application_observation.v1"
	sourceToFinalLineageObservationContract = "source_to_final_lineage_observation.v1"
	sourceToFinalLineageContract            = "source_to_final_lineage.v1"
)

func prepareTurnMemoryLineageSourceRef(sessionID string, rowID any) string {
	sid := strings.TrimSpace(sessionID)
	row := strings.TrimSpace(fmt.Sprint(rowID))
	if sid == "" || row == "" || row == "<nil>" || row == "0" {
		return ""
	}
	return "memory:" + sid + ":" + row
}

func boundedPrepareTurnMemoryDeliveryLineage(sessionID string, raw map[string]any, chatLogs []store.ChatLog) map[string]any {
	assistantByTurn := map[int][]store.ChatLog{}
	for _, item := range chatLogs {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if item.ChatSessionID != sessionID || (role != "assistant" && role != "char") {
			continue
		}
		assistantByTurn[item.TurnIndex] = append(assistantByTurn[item.TurnIndex], item)
	}

	items := make([]any, 0)
	for _, rawItem := range outputFidelityLineageSlice(raw["items"]) {
		item := mapFromAny(rawItem)
		if !boolFromAny(item["delivered"]) {
			continue
		}
		ref := prepareTurnMemoryLineageSourceRef(sessionID, item["source_row_id"])
		if ref == "" {
			continue
		}
		turnIndex := intFromAny(item["turn_index"], 0)
		sourceSpan := map[string]any{
			"status":           "unobserved",
			"reason_code":      "source_span_not_observed",
			"source_kind":      "unobserved",
			"source_lifecycle": "unobserved",
			"semantic_outcome": "unobserved",
			"link_precision":   "turn_only",
			"turn_index":       turnIndex,
		}
		bounded := make(map[string]any, len(item)+6)
		for key, value := range item {
			if key == "final_text" {
				continue
			}
			bounded[key] = value
		}
		bounded["source_ref"] = ref
		bounded["source_table"] = "memories"
		bounded["source_row_id"] = item["source_row_id"]
		bounded["source_turn_index"] = turnIndex
		bounded["selection_lane"] = extractionStringFromAny(item["selection_lane"])
		bounded["delivery_status"] = extractionStringFromAny(item["delivery_status"])
		bounded["delivered"] = true
		bounded["protected_guard"] = boolFromAny(item["protected_guard"])
		bounded["precision"] = "turn_only"
		bounded["link_precision"] = "turn_only"
		bounded["source_span"] = sourceSpan
		if rows := assistantByTurn[turnIndex]; len(rows) == 1 {
			sourceSpan["status"] = "partially_observed"
			sourceSpan["reason_code"] = "turn_only_source_span_observed"
			sourceSpan["source_kind"] = "assistant_output"
			sourceSpan["content_hash"] = prepareTurnTextHash(rows[0].Content)
			sourceSpan["hash_algorithm"] = "sha256"
		} else if len(rows) > 1 {
			sourceSpan["status"] = "ambiguous"
			sourceSpan["reason_code"] = "multiple_assistant_spans_at_turn"
			sourceSpan["source_kind"] = "assistant_output"
		}
		items = append(items, bounded)
	}

	out := make(map[string]any, len(raw)+4)
	for key, value := range raw {
		if key == "items" {
			continue
		}
		out[key] = value
	}
	out["contract_version"] = "memory_delivery_lineage.v1"
	out["status"] = extractionStringFromAny(raw["status"])
	out["precision"] = "turn_only"
	out["source_text_exposed"] = false
	out["final_delivered_count"] = len(items)
	out["items"] = items
	return out
}

func outputFidelityLineageSlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	if items, ok := value.([]map[string]any); ok {
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	}
	return []any{}
}

func deliveredPrepareTurnMemorySourceRefs(sessionID string, lineage map[string]any) []string {
	refs := make([]string, 0)
	for _, raw := range outputFidelityLineageSlice(lineage["items"]) {
		item := mapFromAny(raw)
		if !boolFromAny(item["delivered"]) {
			continue
		}
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if ref == "" {
			ref = prepareTurnMemoryLineageSourceRef(sessionID, item["source_row_id"])
		}
		if ref != "" {
			refs = appendUniqueMemorySearchText(refs, ref)
		}
	}
	return refs
}

func attachPrepareTurnOutputFidelityLineage(
	requestCorrelationID string,
	payloadPlan map[string]any,
	responseExecutionContract map[string]any,
	memoryLineage map[string]any,
) map[string]any {
	correlationID := strings.TrimSpace(requestCorrelationID)
	trace := mapFromAny(payloadPlan["guidance_application_trace"])
	traceItems := outputFidelityLineageSlice(trace["items"])
	for index, raw := range traceItems {
		item := mapFromAny(raw)
		refs := stringSliceFromAny(item["source_refs"])
		sort.Strings(refs)
		itemIDSeed := strings.Join([]string{
			correlationID,
			extractionStringFromAny(item["key"]),
			extractionStringFromAny(item["content_hash"]),
			strings.Join(refs, "\x1f"),
		}, "\x1f")
		item["item_id"] = "gi_" + strings.TrimPrefix(prepareTurnTextHash(itemIDSeed), "sha256:")
		traceItems[index] = item
	}

	planSeed := strings.Join([]string{
		correlationID,
		extractionStringFromAny(payloadPlan["auxiliary_hash"]),
		extractionStringFromAny(payloadPlan["input_context_hash"]),
		fmt.Sprint(traceItems),
	}, "\x1f")
	planID := "stp_" + strings.TrimPrefix(prepareTurnTextHash(planSeed), "sha256:")
	lineageID := "stl_" + strings.TrimPrefix(prepareTurnTextHash(strings.Join([]string{
		correlationID,
		planID,
		strings.Join(stringSliceFromAny(mapFromAny(responseExecutionContract["source_refs"])["all"]), "\x1f"),
	}, "\x1f")), "sha256:")
	payloadPlan["plan_id"] = planID
	payloadPlan["payload_plan_id"] = planID
	payloadPlan["archive_center_request_correlation_id"] = nilIfEmpty(correlationID)
	payloadPlan["official_risu_request_id_state"] = "official_risu_request_id_not_exposed"
	payloadPlan["generation_id_state"] = "unobserved"
	payloadPlan["auxiliary_observation_hash"] = prepareOR1CHash("[Archive Center — Auxiliary Context]\n\n" + extractionStringFromAny(payloadPlan["auxiliary_text"]))
	payloadPlan["input_context_observation_hash"] = prepareOR1CHash("[Archive Center — Input Context]\n\n" + extractionStringFromAny(payloadPlan["input_context_text"]))
	payloadPlan["observation_hash_algorithm"] = "or1c_utf16_djb2.v1"

	trace["archive_center_request_correlation_id"] = nilIfEmpty(correlationID)
	trace["official_risu_request_id_state"] = "official_risu_request_id_not_exposed"
	trace["generation_id_state"] = "unobserved"
	trace["plan_id"] = planID
	trace["payload_plan_id"] = planID
	trace["prepare_lineage_id"] = lineageID
	trace["items"] = traceItems
	trace["final_output_status"] = "unobserved"
	trace["semantic_outcome"] = "unobserved"
	payloadPlan["guidance_application_trace"] = trace

	efficacyItemResults := make([]any, 0, len(traceItems))
	for _, raw := range traceItems {
		item := mapFromAny(raw)
		efficacyItemResults = append(efficacyItemResults, map[string]any{
			"item_id":           item["item_id"],
			"key":               item["key"],
			"source_refs":       stringSliceFromAny(item["source_refs"]),
			"application_state": extractionStringFromAny(item["status"]),
			"semantic_outcome":  "unobserved",
		})
	}
	guideEligibility := mapFromAny(payloadPlan["guide_eligibility"])
	guideEligibilityStatus := extractionStringFromAny(guideEligibility["status"])
	efficacyStatus := "unobserved"
	switch guideEligibilityStatus {
	case "eligible":
		efficacyStatus = "awaiting_final_and_explicit_live_review"
	case "off":
		efficacyStatus = "not_applicable_guide_off"
	case "no_support":
		efficacyStatus = "not_applicable_no_support"
	case "injection_disabled":
		efficacyStatus = "not_applicable_injection_disabled"
	case "budget_disabled":
		efficacyStatus = "not_applicable_budget_disabled"
	}
	if guideEligibilityStatus == "eligible" && intFromAny(trace["applied_count"], 0) == 0 {
		efficacyStatus = "not_applied_no_guidance_item"
	}
	guideEfficacyEvaluation := map[string]any{
		"contract_version":                      "guide_efficacy_evaluation.v1",
		"status":                                efficacyStatus,
		"archive_center_request_correlation_id": nilIfEmpty(correlationID),
		"official_risu_request_id_state":        "official_risu_request_id_not_exposed",
		"payload_plan_id":                       planID,
		"guide_mode":                            guideEligibility["guide_mode"],
		"guide_strength":                        guideEligibility["guide_strength"],
		"eligibility":                           guideEligibilityStatus,
		"eligibility_source_refs":               stringSliceFromAny(guideEligibility["source_refs"]),
		"coverage":                              guideEligibility["coverage"],
		"effective_budget_chars":                intFromAny(trace["budget_chars"], 0),
		"used_chars":                            intFromAny(trace["used_chars"], 0),
		"output_guidance_hash":                  trace["final_hash"],
		"item_results":                          efficacyItemResults,
		"final_output_status":                   "unobserved",
		"first_output_disposition":              "unobserved",
		"reroll_reason":                         "unobserved",
		"provider":                              "unobserved",
		"model":                                 "unobserved",
		"seed_or_cohort_id":                     "unobserved",
		"automatic_text_judgement":              false,
		"body_difference_is_efficacy_evidence":  false,
		"aggregate_score_allowed":               false,
		"semantic_outcome":                      "unobserved",
	}
	payloadPlan["guide_efficacy_evaluation"] = guideEfficacyEvaluation

	executionItems := make([]any, 0)
	for _, key := range []string{"must_preserve", "must_respond", "must_account", "must_not_assert"} {
		group := mapFromAny(responseExecutionContract[key])
		for index, raw := range outputFidelityLineageSlice(group["items"]) {
			item := mapFromAny(raw)
			refs := stringSliceFromAny(item["source_refs"])
			sort.Strings(refs)
			seed := strings.Join([]string{correlationID, key, fmt.Sprint(index), strings.Join(refs, "\x1f")}, "\x1f")
			executionItems = append(executionItems, map[string]any{
				"item_id":           "ei_" + strings.TrimPrefix(prepareTurnTextHash(seed), "sha256:"),
				"kind":              key,
				"source_refs":       refs,
				"application_state": "planned",
				"semantic_outcome":  "unobserved",
			})
		}
	}

	sourceRefs := stringSliceFromAny(mapFromAny(responseExecutionContract["source_refs"])["all"])
	return map[string]any{
		"contract_version":                      sourceToPayloadLineageContract,
		"status":                                "observed",
		"archive_center_request_correlation_id": nilIfEmpty(correlationID),
		"official_risu_request_id_state":        "official_risu_request_id_not_exposed",
		"generation_id_state":                   "unobserved",
		"lineage_id":                            lineageID,
		"payload_plan_id":                       planID,
		"plan_id":                               planID,
		"source_refs":                           sourceRefs,
		"sources":                               outputFidelityLineageSlice(memoryLineage["items"]),
		"execution_items":                       executionItems,
		"guide_efficacy_evaluation":             guideEfficacyEvaluation,
		"payload": map[string]any{
			"status":                         "planned",
			"observation_stage":              "archive_center_before_request_plan",
			"final_provider_payload_state":   "not_exposed",
			"auxiliary_hash":                 payloadPlan["auxiliary_hash"],
			"auxiliary_observation_hash":     payloadPlan["auxiliary_observation_hash"],
			"input_context_hash":             payloadPlan["input_context_hash"],
			"input_context_observation_hash": payloadPlan["input_context_observation_hash"],
			"observation_hash_algorithm":     "or1c_utf16_djb2.v1",
		},
		"final_output": map[string]any{
			"status":           "unobserved",
			"semantic_outcome": "unobserved",
		},
	}
}

func boundedLineageStringSlice(value any, limit int) []string {
	out := make([]string, 0)
	for _, item := range stringSliceFromAny(value) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = appendUniqueMemorySearchText(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func buildSourceToFinalLineage(req dto.M4CompleteTurnRequest, decision completeTurnSourceAcceptanceDecision) map[string]any {
	rawObservation := mapFromAny(req.ClientMeta["source_to_final_lineage_observation"])
	refs := boundedLineageStringSlice(rawObservation["source_refs"], 128)
	itemResults := make([]any, 0, len(refs))
	for _, ref := range refs {
		itemResults = append(itemResults, map[string]any{
			"source_ref":       ref,
			"semantic_outcome": "unobserved",
		})
	}

	status := "unattached"
	reason := "source_to_final_correlation_missing"
	switch {
	case extractionStringFromAny(rawObservation["contract_version"]) != sourceToFinalLineageObservationContract:
		reason = "source_to_final_correlation_missing"
	case strings.TrimSpace(extractionStringFromAny(rawObservation["archive_center_request_correlation_id"])) == "" ||
		strings.TrimSpace(extractionStringFromAny(rawObservation["prepare_lineage_id"])) == "" ||
		strings.TrimSpace(extractionStringFromAny(rawObservation["payload_plan_id"])) == "":
		reason = "source_to_final_correlation_missing"
	case extractionStringFromAny(rawObservation["status"]) != "ready":
		reason = firstNonEmpty(extractionStringFromAny(rawObservation["reason_code"]), "source_to_final_observation_not_ready")
	case extractionStringFromAny(rawObservation["payload_application_status"]) != "applied" &&
		extractionStringFromAny(rawObservation["payload_application_status"]) != "empty":
		reason = "source_to_final_payload_application_unobserved"
	case extractionStringFromAny(rawObservation["payload_observation_stage"]) != "archive_center_before_request_return":
		reason = "source_to_final_payload_stage_unobserved"
	case extractionStringFromAny(rawObservation["final_provider_payload_state"]) != "not_exposed":
		reason = "source_to_final_provider_payload_boundary_invalid"
	case len(refs) == 0:
		reason = "source_to_final_source_refs_missing"
	case !decision.Enabled || !decision.Accepted:
		reason = firstNonEmpty(decision.Reason, "active_final_not_accepted")
	case decision.Observation.GenerationIDState != "observed" ||
		strings.TrimSpace(decision.Observation.GenerationID) == "":
		reason = "source_to_final_generation_unobserved"
	case extractionFirstNonEmpty(
		extractionStringFromAny(rawObservation["generation_id"]),
		extractionStringFromAny(rawObservation["final_generation_id"]),
	) != decision.Observation.GenerationID:
		reason = "source_to_final_generation_mismatch"
	default:
		status = "attached"
		reason = "source_to_final_active_final_attached"
	}
	attached := status == "attached"

	return map[string]any{
		"contract_version":                      sourceToFinalLineageContract,
		"status":                                status,
		"attached":                              attached,
		"reason_code":                           reason,
		"archive_center_request_correlation_id": nilIfEmpty(extractionStringFromAny(rawObservation["archive_center_request_correlation_id"])),
		"request_id_state": extractionFirstNonEmpty(
			extractionStringFromAny(rawObservation["request_id_state"]),
			"official_risu_request_id_not_exposed",
		),
		"official_risu_request_id_state": "official_risu_request_id_not_exposed",
		"prepare_lineage_id":             nilIfEmpty(extractionFirstNonEmpty(extractionStringFromAny(rawObservation["prepare_lineage_id"]), extractionStringFromAny(rawObservation["lineage_id"]))),
		"payload_plan_id":                nilIfEmpty(extractionFirstNonEmpty(extractionStringFromAny(rawObservation["payload_plan_id"]), extractionStringFromAny(rawObservation["plan_id"]))),
		"plan_id":                        nilIfEmpty(extractionFirstNonEmpty(extractionStringFromAny(rawObservation["payload_plan_id"]), extractionStringFromAny(rawObservation["plan_id"]))),
		"payload_application_status":     nilIfEmpty(extractionStringFromAny(rawObservation["payload_application_status"])),
		"payload_observation_stage":      nilIfEmpty(extractionStringFromAny(rawObservation["payload_observation_stage"])),
		"final_provider_payload_state":   nilIfEmpty(extractionStringFromAny(rawObservation["final_provider_payload_state"])),
		"payload_guidance_hash":          nilIfEmpty(extractionStringFromAny(rawObservation["payload_guidance_hash"])),
		"generation_id":                  nilIfEmpty(decision.Observation.GenerationID),
		"generation_id_state":            decision.Observation.GenerationIDState,
		"source_refs":                    refs,
		"final_output": map[string]any{
			"status":           map[bool]string{true: "observed", false: "unattached"}[attached],
			"lifecycle":        map[bool]string{true: "active_final", false: "unobserved"}[attached],
			"generation_id":    nilIfEmpty(decision.Observation.GenerationID),
			"content_hash":     nilIfEmpty(decision.Observation.ObservedContentHash),
			"hash_algorithm":   nilIfEmpty(decision.Observation.HashAlgorithm),
			"semantic_outcome": "unobserved",
		},
		"item_results":     itemResults,
		"semantic_outcome": "unobserved",
		"guide_efficacy_evaluation": map[string]any{
			"contract_version":                     "guide_efficacy_evaluation.v1",
			"status":                               map[bool]string{true: "awaiting_explicit_live_review", false: "unobserved"}[attached],
			"official_risu_request_id_state":       "official_risu_request_id_not_exposed",
			"active_final_message_observed":        attached,
			"active_final_display_observed":        "unobserved",
			"first_output_display_observed":        "unobserved",
			"first_output_disposition":             "unobserved",
			"reroll_reason":                        "unobserved",
			"guide_item_results_state":             "unobserved_not_transmitted",
			"item_results":                         []any{},
			"guide_mode":                           "unobserved",
			"guide_strength":                       "unobserved",
			"eligibility":                          "unobserved",
			"provider":                             "unobserved",
			"model":                                "unobserved",
			"seed_or_cohort_id":                    "unobserved",
			"automatic_text_judgement":             false,
			"body_difference_is_efficacy_evidence": false,
			"aggregate_score_allowed":              false,
			"semantic_outcome":                     "unobserved",
		},
	}
}
