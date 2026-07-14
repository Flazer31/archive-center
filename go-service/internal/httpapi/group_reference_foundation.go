package httpapi

import (
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	referencePrimaryFoundationLastCompletedTurn = 1
	referencePrimaryFoundationItemLimit         = 6
)

type referenceFoundationCandidate struct {
	item     referenceInjectionItem
	score    float64
	category string
	sortKey  string
}

func buildPrimaryReferenceFoundationItems(scopes map[string]referenceRecallScope, sceneContext referenceCoverageSceneContext, limit int) []referenceInjectionItem {
	if sceneContext.RecentCompletedTurn > referencePrimaryFoundationLastCompletedTurn {
		return nil
	}
	if limit <= 0 || limit > referencePrimaryFoundationItemLimit {
		limit = referencePrimaryFoundationItemLimit
	}
	candidates := make([]referenceFoundationCandidate, 0)
	for _, scope := range scopes {
		if referenceBindingMode(scope.binding) != referenceModePrimary {
			continue
		}
		subjectDegree := referenceFoundationSubjectDegree(scope)
		for _, entity := range scope.entities {
			text := strings.TrimSpace(entity.CanonicalName + ": " + entity.DescriptionText)
			if text == "" || strings.Trim(text, ": ") == "" {
				continue
			}
			metadata := parseJSONMap(entity.MetadataJSON)
			category := "entity"
			score := referenceFoundationMetadataScore(metadata)
			switch strings.ToLower(strings.TrimSpace(entity.EntityType)) {
			case "faction":
				score += 42
			case "character":
				score += 32
			case "location", "item":
				score += 14
			default:
				score += 8
			}
			score += float64(minInt(subjectDegree[entity.EntityID], 10) * 2)
			item := referenceInjectionItem{
				BindingID:       scope.binding.BindingID,
				WorkID:          scope.binding.WorkID,
				ContinuityID:    scope.binding.ContinuityID,
				ReferenceKind:   "entity",
				SourceID:        entity.EntityID,
				ReferenceMode:   referenceModePrimary,
				Text:            text,
				CoverageStatus:  "primary_foundation",
				NeededBy:        []string{"primary_canon_foundation"},
				SelectionSource: "primary_canon_foundation",
			}
			if scope.work != nil {
				item.WorkTitle = scope.work.Title
			}
			needed := referenceCoverageNeededSource{BindingID: item.BindingID, WorkID: item.WorkID, ContinuityID: item.ContinuityID, ReferenceKind: item.ReferenceKind, SourceID: item.SourceID, Eligible: true}
			item = referenceCoverageAttachSourceExcerpt(item, scope, needed)
			candidates = append(candidates, referenceFoundationCandidate{item: item, score: score, category: category, sortKey: referenceCoverageNormalize(text)})
		}

		for _, claim := range scope.claims {
			eligible, _ := referenceRecallClaimEligible(scope, claim)
			if !eligible {
				continue
			}
			claimType := strings.ToLower(strings.TrimSpace(claim.ClaimType))
			metadata := parseJSONMap(claim.MetadataJSON)
			explicitRole := strings.ToLower(strings.TrimSpace(stringFromAny(metadata["canon_role"])))
			if claimType != "relationship" && claimType != "world_rule" && claimType != "character" && explicitRole == "" {
				continue
			}
			category := claimType
			if category == "character" {
				category = "entity_fact"
			}
			score := referenceFoundationMetadataScore(metadata) + clampFloat(claim.Confidence, 0, 1)*10
			switch claimType {
			case "relationship":
				score += 48
			case "world_rule":
				score += 40
			case "character":
				score += 28
			default:
				score += 12
			}
			if referenceRecallTimelessScope(claim.TemporalScope) {
				score += 5
			}
			score += float64(minInt(subjectDegree[claim.SubjectEntityID], 10) * 2)
			text := referenceClaimDisplayText(scope, claim)
			if text == "" {
				continue
			}
			item := referenceInjectionItem{
				BindingID:       scope.binding.BindingID,
				WorkID:          scope.binding.WorkID,
				ContinuityID:    scope.binding.ContinuityID,
				ReferenceKind:   "claim",
				SourceID:        claim.ClaimID,
				ReferenceMode:   referenceModePrimary,
				Text:            text,
				KnowledgeScope:  strings.TrimSpace(claim.KnowledgeScope),
				CoverageStatus:  "primary_foundation",
				NeededBy:        []string{"primary_canon_foundation"},
				SelectionSource: "primary_canon_foundation",
			}
			if scope.work != nil {
				item.WorkTitle = scope.work.Title
			}
			needed := referenceCoverageNeededSource{BindingID: item.BindingID, WorkID: item.WorkID, ContinuityID: item.ContinuityID, ReferenceKind: item.ReferenceKind, SourceID: item.SourceID, Eligible: true}
			item = referenceCoverageAttachSourceExcerpt(item, scope, needed)
			candidates = append(candidates, referenceFoundationCandidate{item: item, score: score, category: category, sortKey: referenceCoverageNormalize(text)})
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].sortKey < candidates[j].sortKey
	})
	quotas := map[string]int{"relationship": 3, "world_rule": 2, "entity": 2, "entity_fact": 1}
	counts := map[string]int{}
	out := make([]referenceInjectionItem, 0, limit)
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if len(out) >= limit {
			break
		}
		if quota, ok := quotas[candidate.category]; ok && counts[candidate.category] >= quota {
			continue
		}
		key := referenceCoverageSourceKey(candidate.item.BindingID, candidate.item.ReferenceKind, candidate.item.SourceID)
		if seen[key] {
			continue
		}
		seen[key] = true
		counts[candidate.category]++
		out = append(out, candidate.item)
	}
	return out
}

func referenceFoundationSubjectDegree(scope referenceRecallScope) map[string]int {
	out := map[string]int{}
	for _, claim := range scope.claims {
		if id := strings.TrimSpace(claim.SubjectEntityID); id != "" {
			out[id]++
		}
	}
	return out
}

func referenceFoundationMetadataScore(metadata map[string]any) float64 {
	score := 0.0
	switch strings.ToLower(strings.TrimSpace(stringFromAny(metadata["canon_importance"]))) {
	case "core":
		score += 100
	case "high":
		score += 45
	case "normal":
		score += 10
	}
	switch strings.ToLower(strings.TrimSpace(stringFromAny(metadata["canon_role"]))) {
	case "work_premise":
		score += 90
	case "main_cast", "core_faction":
		score += 80
	case "core_relationship":
		score += 75
	case "core_rule":
		score += 70
	case "core_location", "core_item":
		score += 45
	case "supporting":
		score -= 15
	}
	return score
}

func referenceClaimDisplayText(scope referenceRecallScope, claim store.ReferenceClaim) string {
	text := strings.TrimSpace(claim.ClaimText)
	entity, ok := scope.entities[strings.TrimSpace(claim.SubjectEntityID)]
	if !ok || strings.TrimSpace(entity.CanonicalName) == "" {
		return text
	}
	names := append([]string{entity.CanonicalName}, scope.aliases[entity.EntityID]...)
	if referenceRelationTextContainsAnchor(text, names) {
		return text
	}
	return strings.TrimSpace(entity.CanonicalName) + ": " + text
}

func mergePrimaryReferenceFoundationItems(foundation, scene []referenceInjectionItem, limit int) []referenceInjectionItem {
	if limit <= 0 {
		limit = 8
	}
	out := make([]referenceInjectionItem, 0, limit)
	seen := map[string]bool{}
	appendItems := func(items []referenceInjectionItem) {
		for _, item := range items {
			if len(out) >= limit {
				return
			}
			key := referenceCoverageSourceKey(item.BindingID, item.ReferenceKind, item.SourceID)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, item)
		}
	}
	appendItems(foundation)
	appendItems(scene)
	return out
}
