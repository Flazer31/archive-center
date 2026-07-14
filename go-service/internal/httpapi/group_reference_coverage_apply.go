package httpapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const referenceCoverageApplicationContractVersion = "coverage_apply.v3"

type referenceInjectionItem struct {
	BindingID        string   `json:"binding_id"`
	WorkID           string   `json:"work_id"`
	WorkTitle        string   `json:"work_title"`
	ContinuityID     string   `json:"continuity_id"`
	ReferenceKind    string   `json:"reference_kind"`
	SourceID         string   `json:"source_id"`
	ReferenceMode    string   `json:"reference_mode"`
	Text             string   `json:"text"`
	SourceExcerpt    string   `json:"source_excerpt,omitempty"`
	SourceDocumentID string   `json:"source_document_id,omitempty"`
	SourceChunkIndex *int     `json:"source_chunk_index,omitempty"`
	SourceVerified   bool     `json:"source_verified"`
	ContentMode      string   `json:"content_mode"`
	KnowledgeScope   string   `json:"knowledge_scope,omitempty"`
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
	RelationAppliedCount int            `json:"relation_applied_count"`
	FieldIndexApplied    int            `json:"field_index_applied_count"`
	SkippedStatusCounts  map[string]int `json:"skipped_status_counts"`
	SkippedNoSceneNeed   int            `json:"skipped_no_scene_need_count"`
	SkippedUnknownMode   int            `json:"skipped_unknown_mode_count"`
	SkippedEmptyContent  int            `json:"skipped_empty_content_count"`
	ModeCounts           map[string]int `json:"mode_counts"`
	TruncatedByItemLimit int            `json:"truncated_by_item_limit_count"`
}

func newReferenceCoverageApplicationSummary() referenceCoverageApplicationSummary {
	return referenceCoverageApplicationSummary{
		ContractVersion:     referenceCoverageApplicationContractVersion,
		Mode:                "applied",
		SkippedStatusCounts: map[string]int{},
		ModeCounts:          map[string]int{},
	}
}

func buildReferenceCoverageInjectionItems(bindings []store.SessionReferenceBinding, scopes map[string]referenceRecallScope, selected, relationCompanions []referenceRecallItem, fieldIndex referenceCoverageFieldIndexSummary, limit int) ([]referenceInjectionItem, referenceCoverageApplicationSummary) {
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
	directLimit := limit
	if len(relationCompanions) > 0 && directLimit > 1 {
		reserve := len(relationCompanions)
		if reserve > 2 {
			reserve = 2
		}
		if reserve >= directLimit {
			reserve = directLimit - 1
		}
		directLimit -= reserve
	}
	for _, candidate := range selected {
		if len(items) >= directLimit {
			break
		}
		key := referenceCoverageSourceKey(candidate.BindingID, candidate.ReferenceKind, candidate.SourceID)
		needed, neededFound := neededByKey[key]
		scope, scopeFound := scopes[candidate.BindingID]
		if !scopeFound {
			continue
		}
		referenceMode := referenceBindingMode(scope.binding)
		if referenceMode == referenceModeUnknown {
			summary.SkippedUnknownMode++
			continue
		}
		text := ""
		selectionSource := "chroma_candidate"
		if neededFound {
			if !referenceCoverageStatusInjectable(needed.CoverageStatus) || !needed.Eligible {
				continue
			}
			text = referenceCoverageMissingFieldText(scope, needed)
		} else if referenceMode == referenceModePrimary && referencePrimaryCandidateApplicable(candidate) {
			needed = referenceCoverageNeededSource{
				BindingID:         candidate.BindingID,
				WorkID:            candidate.WorkID,
				ContinuityID:      candidate.ContinuityID,
				ReferenceKind:     candidate.ReferenceKind,
				SourceID:          candidate.SourceID,
				NeededBy:          []string{"primary_chroma_relevance"},
				CoverageStatus:    "primary_context",
				Eligible:          true,
				EligibilityReason: candidate.Reason,
			}
			text = strings.TrimSpace(candidate.Text)
			selectionSource = "primary_chroma_candidate"
		} else {
			summary.SkippedNoSceneNeed++
			continue
		}
		if text == "" {
			summary.SkippedEmptyContent++
			continue
		}
		rank := candidate.ChromaRank
		item := referenceInjectionItem{
			BindingID:        candidate.BindingID,
			WorkID:           candidate.WorkID,
			WorkTitle:        candidate.WorkTitle,
			ContinuityID:     candidate.ContinuityID,
			ReferenceKind:    candidate.ReferenceKind,
			SourceID:         candidate.SourceID,
			ReferenceMode:    referenceMode,
			Text:             text,
			KnowledgeScope:   referenceKnowledgeScopeForItem(candidate, scope),
			CoverageStatus:   needed.CoverageStatus,
			MissingFields:    append([]string{}, needed.MissingFields...),
			NeededBy:         append([]string{}, needed.NeededBy...),
			SelectionSource:  selectionSource,
			ChromaRank:       &rank,
			Distance:         candidate.Distance,
			CosineSimilarity: candidate.CosineSimilarity,
		}
		items = append(items, referenceCoverageAttachSourceExcerpt(item, scope, needed))
		summary.ModeCounts[referenceMode]++
		applied[key] = true
		summary.ChromaAppliedCount++
		if len(items) == directLimit {
			break
		}
	}

	for _, candidate := range relationCompanions {
		if len(items) == limit {
			break
		}
		key := referenceCoverageSourceKey(candidate.BindingID, candidate.ReferenceKind, candidate.SourceID)
		if applied[key] || !candidate.Eligible || !referenceCoverageStatusInjectable(candidate.CoverageStatus) {
			continue
		}
		scope, ok := scopes[candidate.BindingID]
		if !ok || referenceBindingMode(scope.binding) != referenceModePrimary {
			continue
		}
		text := strings.TrimSpace(candidate.Text)
		if text == "" {
			summary.SkippedEmptyContent++
			continue
		}
		needed := referenceRelationCompanionNeededSource(candidate)
		item := referenceInjectionItem{
			BindingID:       candidate.BindingID,
			WorkID:          candidate.WorkID,
			WorkTitle:       candidate.WorkTitle,
			ContinuityID:    candidate.ContinuityID,
			ReferenceKind:   candidate.ReferenceKind,
			SourceID:        candidate.SourceID,
			ReferenceMode:   referenceModePrimary,
			Text:            text,
			KnowledgeScope:  referenceKnowledgeScopeForItem(candidate, scope),
			CoverageStatus:  "primary_context",
			NeededBy:        []string{"primary_relation_companion"},
			SelectionSource: "primary_relation_expansion",
		}
		items = append(items, referenceCoverageAttachSourceExcerpt(item, scope, needed))
		summary.ModeCounts[referenceModePrimary]++
		summary.RelationAppliedCount++
		applied[key] = true
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
		if len(items) >= limit {
			break
		}
		scope, ok := scopes[needed.BindingID]
		if !ok {
			continue
		}
		referenceMode := referenceBindingMode(scope.binding)
		if referenceMode == referenceModeUnknown {
			summary.SkippedUnknownMode++
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
		item := referenceInjectionItem{
			BindingID:       needed.BindingID,
			WorkID:          needed.WorkID,
			WorkTitle:       workTitle,
			ContinuityID:    needed.ContinuityID,
			ReferenceKind:   needed.ReferenceKind,
			SourceID:        needed.SourceID,
			ReferenceMode:   referenceMode,
			Text:            text,
			KnowledgeScope:  referenceKnowledgeScopeForNeededSource(scope, needed),
			CoverageStatus:  needed.CoverageStatus,
			MissingFields:   append([]string{}, needed.MissingFields...),
			NeededBy:        append([]string{}, needed.NeededBy...),
			SelectionSource: "coverage_field_index",
		}
		items = append(items, referenceCoverageAttachSourceExcerpt(item, scope, needed))
		summary.ModeCounts[referenceMode]++
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

func referenceCoverageAttachSourceExcerpt(item referenceInjectionItem, scope referenceRecallScope, needed referenceCoverageNeededSource) referenceInjectionItem {
	excerpt, documentID, chunkIndex := referenceCoverageGroundedSource(scope, needed.ReferenceKind, needed.SourceID)
	item.SourceExcerpt = excerpt
	item.SourceDocumentID = documentID
	item.SourceChunkIndex = chunkIndex
	item.SourceVerified = excerpt != ""
	item.ContentMode = "structured_only"
	if excerpt != "" {
		item.ContentMode = "structured_plus_source"
		if referenceCoverageNormalize(excerpt) == referenceCoverageNormalize(item.Text) {
			item.ContentMode = "structured_matches_source"
		}
	}
	return item
}

func referenceCoverageStringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}
