package httpapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const referenceCoverageApplicationContractVersion = "coverage_apply.v1"

type referenceInjectionItem struct {
	BindingID        string   `json:"binding_id"`
	WorkID           string   `json:"work_id"`
	WorkTitle        string   `json:"work_title"`
	ContinuityID     string   `json:"continuity_id"`
	ReferenceKind    string   `json:"reference_kind"`
	SourceID         string   `json:"source_id"`
	Text             string   `json:"text"`
	CoverageStatus   string   `json:"coverage_status"`
	MissingFields    []string `json:"missing_fields"`
	NeededBy         []string `json:"needed_by"`
	SelectionSource  string   `json:"selection_source"`
	ChromaRank       *int     `json:"chroma_rank,omitempty"`
	Distance         *float64 `json:"distance,omitempty"`
	CosineSimilarity *float64 `json:"cosine_similarity,omitempty"`
}

type referenceCoverageApplicationSummary struct {
	ContractVersion      string         `json:"contract_version"`
	Mode                 string         `json:"mode"`
	RawCandidateCount    int            `json:"raw_candidate_count"`
	NeededSourceCount    int            `json:"needed_source_count"`
	AppliedCount         int            `json:"applied_count"`
	ChromaAppliedCount   int            `json:"chroma_applied_count"`
	FieldIndexApplied    int            `json:"field_index_applied_count"`
	SkippedStatusCounts  map[string]int `json:"skipped_status_counts"`
	SkippedNoSceneNeed   int            `json:"skipped_no_scene_need_count"`
	SkippedEmptyContent  int            `json:"skipped_empty_content_count"`
	TruncatedByItemLimit int            `json:"truncated_by_item_limit_count"`
}

func newReferenceCoverageApplicationSummary() referenceCoverageApplicationSummary {
	return referenceCoverageApplicationSummary{
		ContractVersion:     referenceCoverageApplicationContractVersion,
		Mode:                "applied",
		SkippedStatusCounts: map[string]int{},
	}
}

func buildReferenceCoverageInjectionItems(bindings []store.SessionReferenceBinding, scopes map[string]referenceRecallScope, selected []referenceRecallItem, fieldIndex referenceCoverageFieldIndexSummary, limit int) ([]referenceInjectionItem, referenceCoverageApplicationSummary) {
	summary := newReferenceCoverageApplicationSummary()
	summary.RawCandidateCount = len(selected)
	summary.NeededSourceCount = len(fieldIndex.NeededSourceItems)
	if limit <= 0 {
		limit = 8
	}

	neededByKey := map[string]referenceCoverageNeededSource{}
	for _, needed := range fieldIndex.NeededSourceItems {
		neededByKey[referenceCoverageSourceKey(needed.BindingID, needed.ReferenceKind, needed.SourceID)] = needed
		if !referenceCoverageStatusInjectable(needed.CoverageStatus) {
			status := strings.TrimSpace(needed.CoverageStatus)
			if status == "" {
				status = "unknown"
			}
			summary.SkippedStatusCounts[status]++
		}
	}

	items := make([]referenceInjectionItem, 0, limit)
	applied := map[string]bool{}
	for _, candidate := range selected {
		key := referenceCoverageSourceKey(candidate.BindingID, candidate.ReferenceKind, candidate.SourceID)
		needed, ok := neededByKey[key]
		if !ok {
			summary.SkippedNoSceneNeed++
			continue
		}
		if !referenceCoverageStatusInjectable(needed.CoverageStatus) || !needed.Eligible {
			continue
		}
		scope, ok := scopes[candidate.BindingID]
		if !ok {
			continue
		}
		text := referenceCoverageMissingFieldText(scope, needed)
		if text == "" {
			summary.SkippedEmptyContent++
			continue
		}
		rank := candidate.ChromaRank
		items = append(items, referenceInjectionItem{
			BindingID:        candidate.BindingID,
			WorkID:           candidate.WorkID,
			WorkTitle:        candidate.WorkTitle,
			ContinuityID:     candidate.ContinuityID,
			ReferenceKind:    candidate.ReferenceKind,
			SourceID:         candidate.SourceID,
			Text:             text,
			CoverageStatus:   needed.CoverageStatus,
			MissingFields:    append([]string{}, needed.MissingFields...),
			NeededBy:         append([]string{}, needed.NeededBy...),
			SelectionSource:  "chroma_candidate",
			ChromaRank:       &rank,
			Distance:         candidate.Distance,
			CosineSimilarity: candidate.CosineSimilarity,
		})
		applied[key] = true
		summary.ChromaAppliedCount++
		if len(items) == limit {
			summary.TruncatedByItemLimit += referenceCoverageRemainingInjectableCount(fieldIndex.NeededSourceItems, applied)
			summary.AppliedCount = len(items)
			return items, summary
		}
	}

	structural := make([]referenceCoverageNeededSource, 0, len(fieldIndex.NeededSourceItems))
	for _, needed := range fieldIndex.NeededSourceItems {
		key := referenceCoverageSourceKey(needed.BindingID, needed.ReferenceKind, needed.SourceID)
		if applied[key] || !needed.Eligible || !referenceCoverageStatusInjectable(needed.CoverageStatus) {
			continue
		}
		structural = append(structural, needed)
	}
	sort.SliceStable(structural, func(i, j int) bool {
		leftPriority := referenceRecallBindingPriority(bindings, structural[i].BindingID)
		rightPriority := referenceRecallBindingPriority(bindings, structural[j].BindingID)
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		leftKey := structural[i].ReferenceKind + ":" + structural[i].SourceID
		rightKey := structural[j].ReferenceKind + ":" + structural[j].SourceID
		return leftKey < rightKey
	})
	for _, needed := range structural {
		scope, ok := scopes[needed.BindingID]
		if !ok {
			continue
		}
		text := referenceCoverageMissingFieldText(scope, needed)
		if text == "" {
			summary.SkippedEmptyContent++
			continue
		}
		workTitle := ""
		if scope.work != nil {
			workTitle = scope.work.Title
		}
		items = append(items, referenceInjectionItem{
			BindingID:       needed.BindingID,
			WorkID:          needed.WorkID,
			WorkTitle:       workTitle,
			ContinuityID:    needed.ContinuityID,
			ReferenceKind:   needed.ReferenceKind,
			SourceID:        needed.SourceID,
			Text:            text,
			CoverageStatus:  needed.CoverageStatus,
			MissingFields:   append([]string{}, needed.MissingFields...),
			NeededBy:        append([]string{}, needed.NeededBy...),
			SelectionSource: "coverage_field_index",
		})
		applied[referenceCoverageSourceKey(needed.BindingID, needed.ReferenceKind, needed.SourceID)] = true
		summary.FieldIndexApplied++
		if len(items) == limit {
			break
		}
	}
	summary.TruncatedByItemLimit += referenceCoverageRemainingInjectableCount(fieldIndex.NeededSourceItems, applied)
	summary.AppliedCount = len(items)
	return items, summary
}

func referenceCoverageStatusInjectable(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "partial", "missing":
		return true
	default:
		return false
	}
}

func referenceCoverageSourceKey(bindingID, kind, sourceID string) string {
	return strings.TrimSpace(bindingID) + "|" + strings.ToLower(strings.TrimSpace(kind)) + "|" + strings.TrimSpace(sourceID)
}

func referenceCoverageRemainingInjectableCount(items []referenceCoverageNeededSource, applied map[string]bool) int {
	count := 0
	for _, item := range items {
		key := referenceCoverageSourceKey(item.BindingID, item.ReferenceKind, item.SourceID)
		if !applied[key] && item.Eligible && referenceCoverageStatusInjectable(item.CoverageStatus) {
			count++
		}
	}
	return count
}

func referenceCoverageMissingFieldText(scope referenceRecallScope, needed referenceCoverageNeededSource) string {
	switch strings.ToLower(strings.TrimSpace(needed.ReferenceKind)) {
	case "entity":
		entity, ok := scope.entities[needed.SourceID]
		if !ok {
			return ""
		}
		name := strings.TrimSpace(entity.CanonicalName)
		description := strings.TrimSpace(entity.DescriptionText)
		missingIdentity := referenceCoverageStringSliceContains(needed.MissingFields, "identity_name")
		missingDescription := referenceCoverageStringSliceContains(needed.MissingFields, "description_text")
		switch {
		case missingIdentity && missingDescription && description != "":
			return strings.TrimSpace(name + ": " + description)
		case missingDescription && description != "":
			return fmt.Sprintf("%s: missing profile detail: %s", name, description)
		case missingIdentity:
			return "Canonical identity: " + name
		default:
			return ""
		}
	case "claim":
		claim, ok := scope.claims[needed.SourceID]
		if !ok || !referenceCoverageStringSliceContains(needed.MissingFields, "claim_text") {
			return ""
		}
		return strings.TrimSpace(claim.ClaimText)
	case "timeline":
		node, ok := scope.nodes[needed.SourceID]
		if !ok {
			return ""
		}
		if !referenceCoverageStringSliceContains(needed.MissingFields, "label") && !referenceCoverageStringSliceContains(needed.MissingFields, "node_key") {
			return ""
		}
		return strings.TrimSpace(node.Label)
	default:
		return ""
	}
}

func referenceCoverageStringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}
