package httpapi

import (
	"sort"
	"strings"
	"unicode/utf8"
)

const referencePrimaryRelationCompanionLimit = 4

type referenceRelationCandidate struct {
	item        referenceRecallItem
	confidence  float64
	sameSubject bool
}

func buildPrimaryReferenceRelationCompanions(scopes map[string]referenceRecallScope, selected []referenceRecallItem, query string, messages []map[string]any, sceneContext referenceCoverageSceneContext, limit int) []referenceRecallItem {
	if limit <= 0 {
		limit = referencePrimaryRelationCompanionLimit
	}
	selectedKeys := map[string]bool{}
	for _, item := range selected {
		selectedKeys[referenceCoverageSourceKey(item.BindingID, item.ReferenceKind, item.SourceID)] = true
	}

	out := make([]referenceRecallItem, 0, limit)
	seen := map[string]bool{}
	bindingIDs := make([]string, 0)
	seenBindings := map[string]bool{}
	for _, item := range selected {
		if item.BindingID != "" && !seenBindings[item.BindingID] {
			seenBindings[item.BindingID] = true
			bindingIDs = append(bindingIDs, item.BindingID)
		}
	}
	for _, bindingID := range bindingIDs {
		scope, ok := scopes[bindingID]
		if !ok {
			continue
		}
		if referenceBindingMode(scope.binding) != referenceModePrimary {
			continue
		}
		subjectIDs, anchorNames := referenceRelationAnchors(scope, bindingID, selected)
		if len(subjectIDs) == 0 && len(anchorNames) == 0 {
			continue
		}
		candidates := make([]referenceRelationCandidate, 0)
		for _, claim := range scope.claims {
			if !strings.EqualFold(strings.TrimSpace(claim.ClaimType), "relationship") {
				continue
			}
			key := referenceCoverageSourceKey(bindingID, "claim", claim.ClaimID)
			if selectedKeys[key] || seen[key] {
				continue
			}
			sameSubject := subjectIDs[strings.TrimSpace(claim.SubjectEntityID)]
			if !sameSubject && !referenceRelationTextContainsAnchor(claim.ClaimText, anchorNames) {
				continue
			}
			eligible, reason := referenceRecallClaimEligible(scope, claim)
			if !eligible {
				continue
			}
			item := referenceRecallItem{
				BindingID:     bindingID,
				WorkID:        scope.binding.WorkID,
				ContinuityID:  scope.binding.ContinuityID,
				ReferenceKind: "claim",
				SourceID:      claim.ClaimID,
				Text:          strings.TrimSpace(claim.ClaimText),
				Eligible:      true,
				Reason:        reason,
				Metadata: map[string]any{
					"claim_type":        claim.ClaimType,
					"subject_entity_id": claim.SubjectEntityID,
					"knowledge_scope":   claim.KnowledgeScope,
				},
			}
			if scope.work != nil {
				item.WorkTitle = scope.work.Title
			}
			item = applyReferenceCoverageShadow(item, scope, query, messages, sceneContext)
			if !item.Eligible || !referenceCoverageStatusInjectable(item.CoverageStatus) {
				continue
			}
			item.Needed = true
			item.NeededBy = []string{"primary_relation_companion"}
			item.DecisionReason = "approved_relationship_connected_to_selected_reference"
			candidates = append(candidates, referenceRelationCandidate{item: item, confidence: claim.Confidence, sameSubject: sameSubject})
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].sameSubject != candidates[j].sameSubject {
				return candidates[i].sameSubject
			}
			if candidates[i].confidence != candidates[j].confidence {
				return candidates[i].confidence > candidates[j].confidence
			}
			return candidates[i].item.SourceID < candidates[j].item.SourceID
		})
		for _, candidate := range candidates {
			key := referenceCoverageSourceKey(bindingID, candidate.item.ReferenceKind, candidate.item.SourceID)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, candidate.item)
			if len(out) == limit {
				return out
			}
		}
	}
	return out
}

func referenceRelationAnchors(scope referenceRecallScope, bindingID string, selected []referenceRecallItem) (map[string]bool, []string) {
	subjectIDs := map[string]bool{}
	names := []string{}
	for _, item := range selected {
		if item.BindingID != bindingID {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(item.ReferenceKind)) {
		case "entity":
			if id := strings.TrimSpace(item.SourceID); id != "" {
				subjectIDs[id] = true
				names = append(names, referenceCoverageEntityNames(scope, id)...)
			}
		case "claim":
			if claim, ok := scope.claims[item.SourceID]; ok {
				if id := strings.TrimSpace(claim.SubjectEntityID); id != "" {
					subjectIDs[id] = true
					names = append(names, referenceCoverageEntityNames(scope, id)...)
				}
			}
		}
		normalizedText := referenceCoverageNormalize(item.Text)
		for entityID, entity := range scope.entities {
			entityNames := append([]string{entity.CanonicalName}, scope.aliases[entityID]...)
			if referenceRelationTextContainsAnchor(normalizedText, entityNames) {
				subjectIDs[entityID] = true
				names = append(names, entityNames...)
			}
		}
	}
	return subjectIDs, referenceCoverageUniqueStrings(names)
}

func referenceRelationTextContainsAnchor(text string, names []string) bool {
	normalizedText := referenceCoverageNormalize(text)
	for _, name := range names {
		normalizedName := referenceCoverageNormalize(name)
		if utf8.RuneCountInString(normalizedName) < 2 {
			continue
		}
		if referenceCoverageContainsNormalized(normalizedText, normalizedName) {
			return true
		}
	}
	return false
}

func referenceKnowledgeScopeForItem(item referenceRecallItem, scope referenceRecallScope) string {
	if value := strings.TrimSpace(stringFromAny(item.Metadata["knowledge_scope"])); value != "" {
		return value
	}
	if claim, ok := scope.claims[item.SourceID]; ok {
		return strings.TrimSpace(claim.KnowledgeScope)
	}
	return ""
}

func referenceKnowledgeScopeForNeededSource(scope referenceRecallScope, needed referenceCoverageNeededSource) string {
	if claim, ok := scope.claims[needed.SourceID]; ok {
		return strings.TrimSpace(claim.KnowledgeScope)
	}
	return ""
}

func referenceRelationCompanionNeededSource(item referenceRecallItem) referenceCoverageNeededSource {
	return referenceCoverageNeededSource{
		BindingID:         item.BindingID,
		WorkID:            item.WorkID,
		ContinuityID:      item.ContinuityID,
		ReferenceKind:     item.ReferenceKind,
		SourceID:          item.SourceID,
		NeededBy:          []string{"primary_relation_companion"},
		CoverageStatus:    "primary_context",
		Eligible:          item.Eligible,
		EligibilityReason: item.Reason,
	}
}
