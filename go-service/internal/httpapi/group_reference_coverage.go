package httpapi

import (
	"fmt"
	"strings"
	"unicode"
)

const referenceCoverageContractVersion = "coverage_shadow.v3"

type referenceCoverageMessage struct {
	location   string
	normalized string
	request    bool
}

type referenceCoverageLocations struct {
	All     []string
	Request []string
}

func newReferenceCoverageSummary(sceneContext referenceCoverageSceneContext) referenceCoverageSummary {
	return referenceCoverageSummary{
		ContractVersion:   referenceCoverageContractVersion,
		Mode:              "shadow",
		StatusCounts:      map[string]int{},
		InjectionFiltered: false,
		SceneSignals:      summarizeReferenceCoverageSceneSignals(sceneContext),
		FieldIndex:        newReferenceCoverageFieldIndexSummary(),
		Application:       newReferenceCoverageApplicationSummary(),
	}
}

func summarizeReferenceCoverage(selected, excluded []referenceRecallItem, sceneContext referenceCoverageSceneContext, fieldIndex referenceCoverageFieldIndexSummary) referenceCoverageSummary {
	summary := newReferenceCoverageSummary(sceneContext)
	summary.FieldIndex = fieldIndex
	for _, item := range append(append([]referenceRecallItem{}, selected...), excluded...) {
		status := strings.TrimSpace(item.CoverageStatus)
		if status == "" {
			status = "unknown"
		}
		summary.EvaluatedCount++
		summary.StatusCounts[status]++
	}
	return summary
}

func applyReferenceCoverageShadow(item referenceRecallItem, scope referenceRecallScope, query string, messages []map[string]any, sceneContext referenceCoverageSceneContext) referenceRecallItem {
	item.NeededBy = []string{}
	item.MatchedRequestLocations = []string{}
	item.MatchedContextLocations = []string{}
	item.MissingFields = []string{}

	if !item.Eligible {
		item.CoverageStatus = "not_applicable"
		item.CoverageConfidence = "high"
		item.DecisionReason = item.Reason
		return item
	}

	item.NeededBy = referenceCoverageNeededBy(item, scope, query, sceneContext)
	item.Needed = len(item.NeededBy) > 0
	if !item.Needed {
		if referenceBindingMode(scope.binding) == referenceModePrimary {
			item.Needed = true
			item.NeededBy = []string{"primary_chroma_relevance"}
		} else {
			item.CoverageStatus = "not_applicable"
			item.CoverageConfidence = "high"
			item.DecisionReason = "no_current_scene_need_signal"
			return item
		}
	}

	coverageSources := append(referenceCoverageMessages(messages), referenceCoverageSceneMessages(sceneContext)...)
	if len(coverageSources) == 0 {
		item.CoverageStatus = "unknown"
		item.CoverageConfidence = "low"
		item.DecisionReason = "coverage_sources_unavailable"
		return item
	}

	if locations := referenceCoverageTextLocations(item.Text, coverageSources); len(locations.All) > 0 {
		item.CoverageStatus = "covered"
		item.CoverageConfidence = "high"
		applyReferenceCoverageLocations(&item, locations)
		item.DecisionReason = "exact_reference_text_present"
		return item
	}

	switch item.ReferenceKind {
	case "entity":
		locations := referenceCoverageNameLocations(referenceCoverageItemEntityNames(item, scope, item.SourceID), coverageSources)
		if len(locations.All) > 0 {
			item.CoverageStatus = "partial"
			item.CoverageConfidence = "high"
			applyReferenceCoverageLocations(&item, locations)
			item.MissingFields = []string{"description"}
			item.DecisionReason = "entity_present_description_missing"
			return item
		}
		item.CoverageStatus = "missing"
		item.CoverageConfidence = "high"
		item.MissingFields = []string{"entity_profile"}
		item.DecisionReason = "needed_entity_absent_from_coverage_sources"
	case "claim":
		claim := scope.claims[item.SourceID]
		locations := referenceCoverageNameLocations(referenceCoverageClaimSubjectNames(item, scope, claim.SubjectEntityID), coverageSources)
		if len(locations.All) > 0 {
			item.CoverageStatus = "partial"
			item.CoverageConfidence = "high"
			applyReferenceCoverageLocations(&item, locations)
			item.MissingFields = []string{referenceCoverageClaimField(claim.ClaimType)}
			item.DecisionReason = "claim_subject_present_claim_missing"
			return item
		}
		item.CoverageStatus = "missing"
		item.CoverageConfidence = "high"
		item.MissingFields = []string{referenceCoverageClaimField(claim.ClaimType)}
		item.DecisionReason = "needed_claim_absent_from_coverage_sources"
	case "timeline":
		item.CoverageStatus = "missing"
		item.CoverageConfidence = "high"
		item.MissingFields = []string{"event_detail"}
		item.DecisionReason = "needed_timeline_fact_absent_from_coverage_sources"
	default:
		item.CoverageStatus = "unknown"
		item.CoverageConfidence = "low"
		item.DecisionReason = "unsupported_reference_kind"
	}
	return item
}

func referenceCoverageNeededBy(item referenceRecallItem, scope referenceRecallScope, query string, sceneContext referenceCoverageSceneContext) []string {
	reasons := []string{}
	addReason := func(reason string) {
		for _, existing := range reasons {
			if existing == reason {
				return
			}
		}
		reasons = append(reasons, reason)
	}

	queryNormalized := referenceCoverageNormalize(query)
	switch item.ReferenceKind {
	case "entity":
		if scope.sceneEntities[item.SourceID] {
			addReason("current_scene_entity_id")
		}
		if referenceCoverageContainsAnyName(queryNormalized, referenceCoverageItemEntityNames(item, scope, item.SourceID)) {
			addReason("explicit_user_entity_mention")
		}
	case "claim":
		claim := scope.claims[item.SourceID]
		if claim.SubjectEntityID != "" && scope.sceneEntities[claim.SubjectEntityID] {
			addReason("current_scene_subject_id")
		}
		if referenceCoverageContainsAnyName(queryNormalized, referenceCoverageClaimSubjectNames(item, scope, claim.SubjectEntityID)) {
			addReason("explicit_user_subject_mention")
		}
		if referenceCoverageContainsNormalized(queryNormalized, referenceCoverageNormalize(claim.ClaimText)) {
			addReason("explicit_user_fact_mention")
		}
	case "timeline":
		node := scope.nodes[item.SourceID]
		if referenceCoverageContainsNormalized(queryNormalized, referenceCoverageNormalize(node.Label)) {
			addReason("explicit_user_timeline_mention")
		}
	}
	if referenceCoverageSceneSourcesMatchItem(item, scope, sceneContext.RecentDialogue) {
		addReason("recent_completed_dialogue")
	}
	if referenceCoverageSceneSourcesMatchItem(item, scope, sceneContext.CurrentLocations) {
		addReason("current_location")
	}
	if referenceCoverageSceneSourcesMatchItem(item, scope, sceneContext.ActiveRules) {
		addReason("active_world_rule")
	}
	return reasons
}

func referenceCoverageMessages(messages []map[string]any) []referenceCoverageMessage {
	out := []referenceCoverageMessage{}
	for index, message := range messages {
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(message["role"])))
		text := ""
		if value, ok := message["content"]; ok && value != nil {
			text = strings.TrimSpace(fmt.Sprint(value))
		}
		if text == "" {
			if value, ok := message["data"]; ok && value != nil {
				text = strings.TrimSpace(fmt.Sprint(value))
			}
		}
		if text == "" {
			continue
		}
		if role == "system" && strings.HasPrefix(text, "[Original Work Reference]") {
			continue
		}
		normalized := referenceCoverageNormalize(text)
		if normalized == "" {
			continue
		}
		if role == "" {
			role = "unknown"
		}
		out = append(out, referenceCoverageMessage{
			location:   fmt.Sprintf("%s#%d", role, index),
			normalized: normalized,
			request:    true,
		})
	}
	return out
}

func referenceCoverageTextLocations(text string, messages []referenceCoverageMessage) referenceCoverageLocations {
	needle := referenceCoverageNormalize(text)
	if needle == "" {
		return referenceCoverageLocations{}
	}
	locations := referenceCoverageLocations{}
	for _, message := range messages {
		if strings.Contains(message.normalized, needle) {
			locations.All = append(locations.All, message.location)
			if message.request {
				locations.Request = append(locations.Request, message.location)
			}
		}
	}
	return locations
}

func referenceCoverageNameLocations(names []string, messages []referenceCoverageMessage) referenceCoverageLocations {
	locations := referenceCoverageLocations{}
	seen := map[string]bool{}
	requestSeen := map[string]bool{}
	for _, name := range names {
		needle := referenceCoverageNormalize(name)
		if needle == "" {
			continue
		}
		for _, message := range messages {
			if strings.Contains(message.normalized, needle) && !seen[message.location] {
				seen[message.location] = true
				locations.All = append(locations.All, message.location)
			}
			if message.request && strings.Contains(message.normalized, needle) && !requestSeen[message.location] {
				requestSeen[message.location] = true
				locations.Request = append(locations.Request, message.location)
			}
		}
	}
	return locations
}

func applyReferenceCoverageLocations(item *referenceRecallItem, locations referenceCoverageLocations) {
	item.MatchedContextLocations = append([]string{}, locations.All...)
	item.MatchedRequestLocations = append([]string{}, locations.Request...)
}

func referenceCoverageContainsAnyName(normalizedText string, names []string) bool {
	for _, name := range names {
		if referenceCoverageContainsNormalized(normalizedText, referenceCoverageNormalize(name)) {
			return true
		}
	}
	return false
}

func referenceCoverageContainsNormalized(haystack, needle string) bool {
	return haystack != "" && needle != "" && strings.Contains(haystack, needle)
}

func referenceCoverageEntityNames(scope referenceRecallScope, entityID string) []string {
	entityID = strings.TrimSpace(entityID)
	if entityID == "" {
		return nil
	}
	names := []string{}
	if entity, ok := scope.entities[entityID]; ok && strings.TrimSpace(entity.CanonicalName) != "" {
		names = append(names, strings.TrimSpace(entity.CanonicalName))
	}
	names = append(names, scope.aliases[entityID]...)
	return names
}

func referenceCoverageItemEntityNames(item referenceRecallItem, scope referenceRecallScope, entityID string) []string {
	names := referenceCoverageEntityNames(scope, entityID)
	names = append(names, referenceCoverageStringSlice(item.Metadata["aliases"])...)
	return referenceCoverageUniqueStrings(names)
}

func referenceCoverageClaimSubjectNames(item referenceRecallItem, scope referenceRecallScope, entityID string) []string {
	names := referenceCoverageEntityNames(scope, entityID)
	names = append(names, referenceCoverageStringSlice(item.Metadata["subject_names"])...)
	return referenceCoverageUniqueStrings(names)
}

func referenceCoverageStringSlice(value any) []string {
	out := []string{}
	switch values := value.(type) {
	case []string:
		out = append(out, values...)
	case []any:
		for _, item := range values {
			out = append(out, fmt.Sprint(item))
		}
	case string:
		out = append(out, strings.Split(values, ",")...)
	}
	return referenceCoverageUniqueStrings(out)
}

func referenceCoverageUniqueStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		text := strings.TrimSpace(value)
		normalized := referenceCoverageNormalize(text)
		if text == "" || normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, text)
	}
	return out
}

func referenceCoverageClaimField(claimType string) string {
	claimType = strings.ToLower(strings.TrimSpace(claimType))
	if claimType == "" {
		return "claim"
	}
	return claimType
}

func referenceCoverageNormalize(text string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
