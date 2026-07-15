package httpapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type primaryCanonBaseResult struct {
	Status                   string   `json:"status"`
	ConfiguredSubbudgetChars int      `json:"configured_subbudget_chars"`
	EffectiveSubbudgetChars  int      `json:"effective_subbudget_chars"`
	BudgetScope              string   `json:"budget_scope"`
	ReferenceTotalCapChars   int      `json:"reference_total_cap_chars"`
	UsedChars                int      `json:"used_chars"`
	Truncated                bool     `json:"truncated"`
	MissingFields            []string `json:"missing_fields"`
	SelectedSourceIDs        []string `json:"selected_source_ids"`
	FoundationQueries        []string `json:"foundation_queries"`
	SearchStatus             string   `json:"search_status"`
	Text                     string   `json:"text,omitempty"`
	selectedSourceKeys       map[string]bool
}

type primaryCanonBaseCandidate struct {
	bindingID       string
	bindingPriority int
	referenceKind   string
	sourceID        string
	text            string
	queryMatched    bool
	semanticRank    int
	semanticMatch   bool
}

func newPrimaryCanonBaseResult(status string) primaryCanonBaseResult {
	return primaryCanonBaseResult{
		Status:             status,
		BudgetScope:        "within_reference_total",
		MissingFields:      []string{},
		SelectedSourceIDs:  []string{},
		FoundationQueries:  []string{},
		SearchStatus:       "not_attempted",
		selectedSourceKeys: map[string]bool{},
	}
}

func (s *Server) buildPrimaryCanonBase(ctx context.Context, sid, sceneQuery string, configuredBudget *int, referenceTotalCap int, injectionEnabled bool, clientMeta map[string]any) primaryCanonBaseResult {
	result := newPrimaryCanonBaseResult("disabled")
	if referenceTotalCap > 0 {
		result.ReferenceTotalCapChars = referenceTotalCap
	}
	if !injectionEnabled {
		return result
	}
	if configuredBudget == nil {
		result.Status = "budget_missing"
		result.MissingFields = append(result.MissingFields, "primary_canon_base_max_chars")
		return result
	}
	result.ConfiguredSubbudgetChars = *configuredBudget
	if result.ConfiguredSubbudgetChars < 0 {
		result.ConfiguredSubbudgetChars = 0
	}
	result.EffectiveSubbudgetChars = minReferenceBudget(result.ConfiguredSubbudgetChars, result.ReferenceTotalCapChars)
	if result.EffectiveSubbudgetChars <= 0 {
		return result
	}
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok {
		result.Status = "failed"
		result.MissingFields = append(result.MissingFields, "reference_store")
		return result
	}
	bindings, err := ref.ListSessionReferenceBindings(ctx, strings.TrimSpace(sid), false)
	if err != nil {
		result.Status = "failed"
		result.MissingFields = append(result.MissingFields, "reference_bindings")
		return result
	}
	if len(bindings) == 0 {
		result.Status = "empty"
		return result
	}
	primaryBindings := make([]store.SessionReferenceBinding, 0, len(bindings))
	for _, binding := range bindings {
		if referenceBindingMode(binding) == referenceModePrimary {
			primaryBindings = append(primaryBindings, binding)
		}
	}
	if len(primaryBindings) == 0 {
		result.Status = "not_applicable"
		return result
	}
	sort.SliceStable(primaryBindings, func(i, j int) bool {
		if primaryBindings[i].Priority != primaryBindings[j].Priority {
			return primaryBindings[i].Priority > primaryBindings[j].Priority
		}
		return primaryBindings[i].BindingID < primaryBindings[j].BindingID
	})

	sceneEntities := referenceRecallStringSet(clientMeta["reference_scene_entity_ids"])
	identityLines := []string{}
	candidates := []primaryCanonBaseCandidate{}
	failedScopes := 0
	searchFailures := 0
	searchSuccesses := 0
	semanticMatches := 0
	embedder := s.completeTurnExtractionConfig(clientMeta).Embedder
	querier, exactQueryAvailable := s.ReferenceVector.(vector.ExactMetadataQuerier)
	for _, binding := range primaryBindings {
		scope, scopeErr := loadReferenceRecallScope(ctx, ref, binding, sceneEntities)
		if scopeErr != nil {
			failedScopes++
			continue
		}
		workTitle := "bound work"
		if scope.work != nil && strings.TrimSpace(scope.work.Title) != "" {
			workTitle = strings.TrimSpace(scope.work.Title)
		}
		continuityDisplay := "active continuity"
		continuityIdentity := strings.TrimSpace(binding.ContinuityID)
		if continuities, continuityErr := ref.ListReferenceContinuities(ctx, binding.WorkID); continuityErr == nil {
			for _, continuity := range continuities {
				if continuity.ContinuityID == binding.ContinuityID && strings.TrimSpace(continuity.Label) != "" {
					continuityDisplay = strings.TrimSpace(continuity.Label)
					continuityIdentity = continuityDisplay + " (" + continuityIdentity + ")"
					break
				}
			}
		}
		identityLines = append(identityLines, fmt.Sprintf("- Work: %s / Continuity: %s\n", workTitle, continuityDisplay))
		foundationQuery := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(sceneQuery), workTitle, continuityIdentity}, "\n"))
		result.FoundationQueries = append(result.FoundationQueries, foundationQuery)
		semanticRanks := map[string]int{}
		approvedIDs := referenceRecallApprovedIDs(scope)
		switch {
		case !embedder.hasConfig():
			searchFailures++
			result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "embedding_config")
		case s.ReferenceVectorOpenError != nil || s.ReferenceVector == nil || !exactQueryAvailable:
			searchFailures++
			result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "reference_vector")
		case len(approvedIDs) == 0:
			searchSuccesses++
		case foundationQuery == "":
			searchFailures++
			result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "foundation_query")
		default:
			embeddingJSON, model, embeddingErr := callEmbedding(ctx, embedder, foundationQuery)
			if embeddingErr != nil {
				searchFailures++
				result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "foundation_embedding")
				break
			}
			queryVector := parseFloat32JSONList(embeddingJSON)
			if len(queryVector) == 0 {
				searchFailures++
				result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "foundation_embedding")
				break
			}
			if validationErr := validateReferenceQueryEmbeddingSpace(ctx, s.ReferenceVector, binding.WorkID, binding.ContinuityID, approvedIDs, embedder.Provider, model); validationErr != nil {
				searchFailures++
				result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "embedding_space")
				break
			}
			where := map[string]any{"$and": []map[string]any{
				{"work_id": binding.WorkID},
				{"continuity_id": binding.ContinuityID},
				{"review_status": "approved"},
			}}
			rawResults, queryErr := querier.QueryExact(ctx, vector.ExactQuery{Embedding: queryVector, Limit: len(approvedIDs), Where: where})
			if queryErr != nil && !errors.Is(queryErr, vector.ErrNotFound) {
				searchFailures++
				result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "foundation_search")
				break
			}
			searchSuccesses++
			for _, raw := range rawResults {
				item, include := referenceRecallCanonicalItem(scope, raw)
				if !include || !item.Eligible {
					continue
				}
				semanticRanks[referenceCoverageSourceKey(binding.BindingID, item.ReferenceKind, item.SourceID)] = raw.ChromaRank
			}
			semanticMatches += len(semanticRanks)
		}
		for entityID, entity := range scope.entities {
			text := strings.TrimSpace(entity.CanonicalName)
			aliases := referenceCoverageUniqueStrings(scope.aliases[entityID])
			if len(aliases) > 0 {
				text += " (aliases: " + strings.Join(aliases, ", ") + ")"
			}
			if description := strings.TrimSpace(entity.DescriptionText); description != "" {
				text += ": " + description
			}
			if text == "" {
				continue
			}
			candidate := primaryCanonBaseCandidate{
				bindingID: binding.BindingID, bindingPriority: binding.Priority, referenceKind: "entity", sourceID: entityID, text: text,
				queryMatched: primaryCanonBaseQueryMatches(sceneQuery, append([]string{text, entity.CanonicalName}, aliases...)),
			}
			candidate.semanticRank, candidate.semanticMatch = semanticRanks[referenceCoverageSourceKey(binding.BindingID, candidate.referenceKind, candidate.sourceID)]
			candidates = append(candidates, candidate)
		}
		for claimID, claim := range scope.claims {
			eligible, _ := referenceRecallClaimEligible(scope, claim)
			if !eligible || strings.TrimSpace(claim.ClaimText) == "" {
				continue
			}
			anchors := []string{claim.ClaimText}
			if subject, exists := scope.entities[claim.SubjectEntityID]; exists {
				anchors = append(anchors, subject.CanonicalName)
				anchors = append(anchors, scope.aliases[claim.SubjectEntityID]...)
			}
			candidate := primaryCanonBaseCandidate{
				bindingID: binding.BindingID, bindingPriority: binding.Priority, referenceKind: "claim", sourceID: claimID, text: strings.TrimSpace(claim.ClaimText),
				queryMatched: primaryCanonBaseQueryMatches(sceneQuery, anchors),
			}
			candidate.semanticRank, candidate.semanticMatch = semanticRanks[referenceCoverageSourceKey(binding.BindingID, candidate.referenceKind, candidate.sourceID)]
			candidates = append(candidates, candidate)
		}
	}
	switch {
	case searchFailures > 0:
		result.SearchStatus = "degraded"
	case searchSuccesses > 0 && semanticMatches == 0:
		result.SearchStatus = "empty"
		result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "semantic_candidates")
	case searchSuccesses > 0:
		result.SearchStatus = "ready"
	}
	if len(identityLines) == 0 {
		result.Status = "failed"
		result.MissingFields = append(result.MissingFields, "work_identity")
		return result
	}
	if failedScopes > 0 {
		result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "primary_binding_scope")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].semanticMatch != candidates[j].semanticMatch {
			return candidates[i].semanticMatch
		}
		if candidates[i].semanticMatch && candidates[i].semanticRank != candidates[j].semanticRank {
			return candidates[i].semanticRank < candidates[j].semanticRank
		}
		if candidates[i].queryMatched != candidates[j].queryMatched {
			return candidates[i].queryMatched
		}
		if candidates[i].bindingPriority != candidates[j].bindingPriority {
			return candidates[i].bindingPriority > candidates[j].bindingPriority
		}
		if candidates[i].bindingID != candidates[j].bindingID {
			return candidates[i].bindingID < candidates[j].bindingID
		}
		if candidates[i].referenceKind != candidates[j].referenceKind {
			return candidates[i].referenceKind < candidates[j].referenceKind
		}
		return candidates[i].sourceID < candidates[j].sourceID
	})

	var builder strings.Builder
	appendWithinBudget := func(text string) bool {
		if utf8.RuneCountInString(builder.String())+utf8.RuneCountInString(text) > result.EffectiveSubbudgetChars {
			return false
		}
		builder.WriteString(text)
		return true
	}
	if !appendWithinBudget("[Primary Canon Base]\n") {
		result.Status = "budget_missing"
		result.Truncated = true
		result.MissingFields = append(result.MissingFields, "work_identity")
		return result
	}
	for _, line := range identityLines {
		if !appendWithinBudget(line) {
			result.Truncated = true
			result.MissingFields = append(result.MissingFields, "work_identity")
			break
		}
	}
	for _, candidate := range candidates {
		line := fmt.Sprintf("- [%s] %s\n", candidate.referenceKind, candidate.text)
		if !appendWithinBudget(line) {
			result.Truncated = true
			break
		}
		key := referenceCoverageSourceKey(candidate.bindingID, candidate.referenceKind, candidate.sourceID)
		result.selectedSourceKeys[key] = true
		result.SelectedSourceIDs = append(result.SelectedSourceIDs, candidate.referenceKind+":"+candidate.sourceID)
	}
	result.Text = strings.TrimSpace(builder.String())
	result.UsedChars = utf8.RuneCountInString(result.Text)
	if len(result.SelectedSourceIDs) == 0 {
		result.Status = "undercovered"
		result.MissingFields = appendPrimaryCanonBaseMissing(result.MissingFields, "canon_context")
	} else if failedScopes > 0 || searchFailures > 0 || len(result.MissingFields) > 0 {
		result.Status = "degraded"
	} else {
		result.Status = "ready"
	}
	return result
}

func appendPrimaryCanonBaseMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func primaryCanonBaseQueryMatches(query string, candidates []string) bool {
	normalizedQuery := referenceCoverageNormalize(query)
	if normalizedQuery == "" {
		return false
	}
	for _, candidate := range candidates {
		normalizedCandidate := referenceCoverageNormalize(candidate)
		if normalizedCandidate != "" && (referenceCoverageContainsNormalized(normalizedQuery, normalizedCandidate) || referenceCoverageContainsNormalized(normalizedCandidate, normalizedQuery)) {
			return true
		}
	}
	return false
}

func removePrimaryCanonBaseDuplicates(items []referenceInjectionItem, selected map[string]bool) []referenceInjectionItem {
	if len(selected) == 0 {
		return items
	}
	out := make([]referenceInjectionItem, 0, len(items))
	for _, item := range items {
		if selected[referenceCoverageSourceKey(item.BindingID, item.ReferenceKind, item.SourceID)] {
			continue
		}
		out = append(out, item)
	}
	return out
}
