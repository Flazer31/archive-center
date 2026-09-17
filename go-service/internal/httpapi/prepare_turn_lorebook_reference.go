package httpapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	prepareTurnLorebookReferenceContractV1    = "lorebook_reference_recall.v1"
	prepareTurnLorebookSelectionObservationV1 = "lorebook_selection_observation.v1"
	prepareTurnLorebookScopeContractV1        = "lorebook_reference_scope.v1"
	prepareTurnLorebookModeOff                = "off" // legacy input normalized to reference_assist
	prepareTurnLorebookModeSearchOnly         = "search_only"
	prepareTurnLorebookModeReferenceAssist    = "reference_assist"
	prepareTurnLorebookModeInvalid            = "invalid"
)

type prepareTurnLorebookCandidate struct {
	Entry          store.LorebookReferenceEntryObservation
	EntryRef       string
	MatchedKeys    []string
	ContextOverlap int
}

type prepareTurnLorebookDeliveredItem struct {
	Text       string
	SourceRefs []string
}

type prepareTurnLorebookObservedText struct {
	Source string
	Text   string
}

type prepareTurnLorebookReferenceResult struct {
	ContractVersion              string           `json:"contract_version"`
	Mode                         string           `json:"mode"`
	Status                       string           `json:"status"`
	ReasonCode                   string           `json:"reason_code"`
	ScopeStatus                  string           `json:"scope_status"`
	StoreRead                    bool             `json:"store_read"`
	CatalogCount                 int              `json:"catalog_count"`
	CandidateCount               int              `json:"candidate_count"`
	CandidateChars               int              `json:"candidate_chars"`
	CandidateRefs                []map[string]any `json:"candidate_refs"`
	SelectionObservationContract string           `json:"selection_observation_contract"`
	AlwaysActiveCandidateCount   int              `json:"always_active_candidate_count"`
	AlwaysActiveDeliveryCount    int              `json:"always_active_delivery_count"`
	KeyMatchedCandidateCount     int              `json:"key_matched_candidate_count"`
	ContextMatchedCandidateCount int              `json:"context_matched_candidate_count"`
	AlreadyPresentCount          int              `json:"already_present_count"`
	NoContextMatchCount          int              `json:"no_context_match_count"`
	CoalescedContentCount        int              `json:"coalesced_content_count"`
	FinalDispositionCounts       map[string]int   `json:"final_disposition_counts"`
	SelectedCount                int              `json:"selected_count"`
	SelectedChars                int              `json:"selected_chars"`
	DeferredCount                int              `json:"deferred_count"`
	BudgetDeferredCount          int              `json:"budget_deferred_count"`
	BudgetChars                  int              `json:"budget_chars"`
	UsedChars                    int              `json:"used_chars"`
	DeliveryCount                int              `json:"delivery_count"`
	DeliveryChars                int              `json:"delivery_chars"`
	PublisherCount               int              `json:"publisher_count"`
	SelectionSource              string           `json:"selection_source,omitempty"`
	candidates                   []prepareTurnLorebookCandidate
	preprocessingRefs            *[]string
	deliveryText                 string
	delivered                    []prepareTurnLorebookDeliveredItem
}

func normalizePrepareTurnLorebookMode(value string) string {
	switch strings.TrimSpace(value) {
	case "", prepareTurnLorebookModeOff, prepareTurnLorebookModeReferenceAssist:
		return prepareTurnLorebookModeReferenceAssist
	case prepareTurnLorebookModeSearchOnly:
		return prepareTurnLorebookModeSearchOnly
	default:
		return prepareTurnLorebookModeInvalid
	}
}

func newPrepareTurnLorebookReferenceResult(mode string) prepareTurnLorebookReferenceResult {
	return prepareTurnLorebookReferenceResult{
		ContractVersion:              prepareTurnLorebookReferenceContractV1,
		Mode:                         mode,
		Status:                       "disabled",
		ReasonCode:                   "lorebook_reference_disabled",
		ScopeStatus:                  "unobserved",
		CandidateRefs:                []map[string]any{},
		SelectionObservationContract: prepareTurnLorebookSelectionObservationV1,
		FinalDispositionCounts:       map[string]int{},
		DeliveryCount:                0,
		DeliveryChars:                0,
		PublisherCount:               0,
	}
}

func finalizePrepareTurnLorebookReference(
	result *prepareTurnLorebookReferenceResult,
	rawUserInput string,
	messages []map[string]any,
	deliveredContextTexts []string,
	injectionEnabled bool,
	budgetChars int,
) {
	if result == nil || result.Mode != prepareTurnLorebookModeReferenceAssist {
		return
	}
	if result.Status == "unavailable" {
		return
	}
	if result.ScopeStatus != "observed" {
		result.Status = "deferred"
		result.ReasonCode = "lorebook_reference_scope_not_fully_observed"
		return
	}
	if len(result.candidates) == 0 {
		return
	}
	if budgetChars < 0 {
		budgetChars = 0
	}
	result.BudgetChars = budgetChars
	aiSelection := result.preprocessingRefs != nil
	result.SelectionSource = "go_default"
	selectedRanks := map[string]int{}
	if aiSelection {
		result.SelectionSource = "ai"
		for rank, ref := range *result.preprocessingRefs {
			if _, exists := selectedRanks[ref]; !exists {
				selectedRanks[ref] = rank
			}
		}
	}

	observed := make([]prepareTurnLorebookObservedText, 0, len(messages)+len(deliveredContextTexts)+1)
	if strings.TrimSpace(rawUserInput) != "" {
		observed = append(observed, prepareTurnLorebookObservedText{Source: "current_user_input", Text: rawUserInput})
	}
	for _, message := range messages {
		if text := extractionStringFromAny(message["content"]); strings.TrimSpace(text) != "" {
			observed = append(observed, prepareTurnLorebookObservedText{Source: "risu_request_message", Text: text})
		}
	}
	for _, text := range deliveredContextTexts {
		if strings.TrimSpace(text) != "" {
			observed = append(observed, prepareTurnLorebookObservedText{Source: "archive_center_delivery", Text: text})
		}
	}

	type selectedGroup struct {
		Text             string
		SourceRefs       []string
		CandidateIndexes []int
		KeyMatched       bool
		ContextOverlap   int
	}
	header := "[Archive Center — Lorebook Reference]"
	candidateLines := []string{}
	groups := []selectedGroup{}
	groupIndexes := map[string]int{}
	candidateIndexes := make([]int, len(result.candidates))
	for index := range candidateIndexes {
		candidateIndexes[index] = index
	}
	if aiSelection {
		sort.SliceStable(candidateIndexes, func(i, j int) bool {
			left, leftSelected := selectedRanks[result.candidates[candidateIndexes[i]].EntryRef]
			right, rightSelected := selectedRanks[result.candidates[candidateIndexes[j]].EntryRef]
			if leftSelected != rightSelected {
				return leftSelected
			}
			return left < right
		})
	}
	for _, candidateIndex := range candidateIndexes {
		candidate := result.candidates[candidateIndex]
		text := strings.TrimSpace(candidate.Entry.Content)
		if text == "" {
			setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, "deferred_empty_content", "", false)
			continue
		}
		candidateLines = append(candidateLines, "- "+text)
		if aiSelection {
			if _, selected := selectedRanks[candidate.EntryRef]; !selected {
				result.DeferredCount++
				setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, "excluded_ai_not_selected", "", false)
				continue
			}
		}
		if source := prepareTurnLorebookPayloadPresenceSource(text, observed); source != "" {
			result.AlreadyPresentCount++
			result.DeferredCount++
			setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, "excluded_already_present", source, false)
			continue
		}
		if !aiSelection && len(candidate.MatchedKeys) == 0 && candidate.ContextOverlap == 0 {
			result.NoContextMatchCount++
			result.DeferredCount++
			setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, "excluded_no_context_match", "", false)
			continue
		}
		normalized := normalizePrepareTurnLorebookText(text)
		if groupIndex, exists := groupIndexes[normalized]; exists {
			group := &groups[groupIndex]
			group.SourceRefs = appendUniqueStringValues(group.SourceRefs, candidate.EntryRef)
			group.CandidateIndexes = append(group.CandidateIndexes, candidateIndex)
			group.KeyMatched = group.KeyMatched || len(candidate.MatchedKeys) > 0
			group.ContextOverlap = maxInt(group.ContextOverlap, candidate.ContextOverlap)
			result.CoalescedContentCount++
			continue
		}
		groupIndexes[normalized] = len(groups)
		groups = append(groups, selectedGroup{
			Text:             text,
			SourceRefs:       []string{candidate.EntryRef},
			CandidateIndexes: []int{candidateIndex},
			KeyMatched:       len(candidate.MatchedKeys) > 0,
			ContextOverlap:   candidate.ContextOverlap,
		})
	}
	if len(candidateLines) > 0 {
		result.CandidateChars = len([]rune(header + "\n" + strings.Join(candidateLines, "\n")))
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if aiSelection {
			return false // Groups already follow the received reference order.
		}
		if groups[i].KeyMatched != groups[j].KeyMatched {
			return groups[i].KeyMatched
		}
		if groups[i].ContextOverlap != groups[j].ContextOverlap {
			return groups[i].ContextOverlap > groups[j].ContextOverlap
		}
		return result.candidates[groups[i].CandidateIndexes[0]].Entry.EntryOrdinal < result.candidates[groups[j].CandidateIndexes[0]].Entry.EntryOrdinal
	})
	hasKeyMatchedGroup := false
	strongestContextOverlap := 0
	for _, group := range groups {
		hasKeyMatchedGroup = hasKeyMatchedGroup || group.KeyMatched
		strongestContextOverlap = maxInt(strongestContextOverlap, group.ContextOverlap)
	}
	frontierGroups := make([]selectedGroup, 0, len(groups))
	for _, group := range groups {
		insideFrontier := aiSelection || group.ContextOverlap == strongestContextOverlap
		if !aiSelection && hasKeyMatchedGroup {
			insideFrontier = group.KeyMatched
		}
		if insideFrontier {
			frontierGroups = append(frontierGroups, group)
			continue
		}
		result.DeferredCount++
		for _, candidateIndex := range group.CandidateIndexes {
			setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, "excluded_below_relevance_frontier", "", false)
		}
	}
	groups = frontierGroups
	selectedLines := make([]string, 0, len(groups))
	for _, group := range groups {
		selectedLines = append(selectedLines, "- "+group.Text)
	}
	result.SelectedCount = len(groups)
	if len(selectedLines) > 0 {
		result.SelectedChars = len([]rune(header + "\n" + strings.Join(selectedLines, "\n")))
	}
	if len(groups) == 0 {
		result.Status = "empty"
		switch {
		case aiSelection && len(selectedRanks) == 0:
			result.ReasonCode = "lorebook_ai_selected_empty"
		case result.AlreadyPresentCount > 0 && result.NoContextMatchCount == 0:
			result.ReasonCode = "lorebook_relevant_context_already_present"
		case result.NoContextMatchCount > 0:
			result.ReasonCode = "lorebook_no_missing_context_match"
		default:
			result.ReasonCode = "lorebook_no_candidates"
		}
		return
	}
	if !injectionEnabled {
		result.Status = "deferred"
		result.ReasonCode = "lorebook_reference_injection_disabled"
		result.DeferredCount += len(groups)
		for _, group := range groups {
			for _, candidateIndex := range group.CandidateIndexes {
				setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, "deferred_injection_disabled", "", false)
			}
		}
		return
	}
	if budgetChars == 0 {
		result.Status = "deferred"
		result.ReasonCode = "lorebook_reference_budget_zero"
		result.DeferredCount += len(groups)
		result.BudgetDeferredCount = len(groups)
		for _, group := range groups {
			for _, candidateIndex := range group.CandidateIndexes {
				setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, "deferred_budget_zero", "", false)
			}
		}
		return
	}

	used := 0
	lines := []string{}
	budgetDeferred := 0
	for _, group := range groups {
		line := "- " + group.Text
		additional := len([]rune(line))
		if len(lines) == 0 {
			additional += len([]rune(header)) + 1
		} else {
			additional++
		}
		if used+additional > budgetChars && !aiSelection {
			budgetDeferred++
			result.DeferredCount++
			for _, candidateIndex := range group.CandidateIndexes {
				setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, "deferred_budget_exhausted", "", false)
			}
			continue
		}
		used += additional
		lines = append(lines, line)
		result.delivered = append(result.delivered, prepareTurnLorebookDeliveredItem{
			Text:       group.Text,
			SourceRefs: append([]string(nil), group.SourceRefs...),
		})
		for sourceIndex, candidateIndex := range group.CandidateIndexes {
			if result.candidates[candidateIndex].Entry.AlwaysActive != nil && *result.candidates[candidateIndex].Entry.AlwaysActive {
				result.AlwaysActiveDeliveryCount++
			}
			disposition := "delivered"
			if sourceIndex > 0 {
				disposition = "delivered_shared_content"
			}
			setPrepareTurnLorebookCandidateDisposition(result, candidateIndex, disposition, "", true)
		}
	}
	result.UsedChars = used
	result.DeliveryCount = len(result.delivered)
	result.DeliveryChars = used
	result.BudgetDeferredCount = budgetDeferred
	if len(lines) == 0 {
		if budgetDeferred > 0 {
			result.Status = "deferred"
			result.ReasonCode = "lorebook_reference_budget_exhausted"
		} else {
			result.Status = "empty"
			result.ReasonCode = "lorebook_no_relevant_candidates"
		}
		return
	}
	result.deliveryText = header + "\n" + strings.Join(lines, "\n")
	result.Status = "ready"
	result.ReasonCode = "lorebook_reference_delivered"
}

// Supplies existing scene-matched reference text to the optional world specialist.
// Canonical memory, the Host lorebook, and the ordinary selection remain unchanged.
func prepareTurnLorebookPreprocessingCandidates(result prepareTurnLorebookReferenceResult) []map[string]any {
	items := []map[string]any{}
	if result.Mode != prepareTurnLorebookModeReferenceAssist || result.ScopeStatus != "observed" || result.Status == "unavailable" {
		return items
	}
	ordered := append([]prepareTurnLorebookCandidate(nil), result.candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		leftKey, rightKey := len(ordered[i].MatchedKeys) > 0, len(ordered[j].MatchedKeys) > 0
		if leftKey != rightKey {
			return leftKey
		}
		return ordered[i].ContextOverlap > ordered[j].ContextOverlap
	})
	for _, candidate := range ordered {
		if len(candidate.MatchedKeys) == 0 && candidate.ContextOverlap == 0 {
			continue
		}
		label := strings.TrimSpace(candidate.Entry.Comment)
		if label == "" {
			label = strings.TrimSpace(candidate.Entry.Key)
		}
		if label == "" {
			label = strings.TrimSpace(strings.TrimLeft(strings.SplitN(strings.TrimSpace(candidate.Entry.Content), "\n", 2)[0], "#"))
		}
		items = append(items, map[string]any{
			"id": candidate.EntryRef, "text": strings.TrimSpace(candidate.Entry.Content),
			"label":        compactPrepareTurnLine(label, 160),
			"matched_keys": candidate.MatchedKeys, "context_overlap": candidate.ContextOverlap,
			"authority": "reference_only",
		})
	}
	return items
}

func setPrepareTurnLorebookCandidateDisposition(result *prepareTurnLorebookReferenceResult, candidateIndex int, disposition, observedSource string, delivered bool) {
	if result == nil || candidateIndex < 0 || candidateIndex >= len(result.CandidateRefs) {
		return
	}
	entry := result.CandidateRefs[candidateIndex]
	if entry == nil {
		entry = map[string]any{}
		result.CandidateRefs[candidateIndex] = entry
	}
	entry["final_disposition"] = disposition
	entry["delivered"] = delivered
	if observedSource != "" {
		entry["observed_source"] = observedSource
	}
	result.FinalDispositionCounts[disposition]++
}

func prepareTurnLorebookPayloadPresenceSource(content string, observed []prepareTurnLorebookObservedText) string {
	content = normalizePrepareTurnLorebookText(content)
	if content == "" {
		return ""
	}
	for _, item := range observed {
		if text := normalizePrepareTurnLorebookText(item.Text); text != "" && strings.Contains(text, content) {
			return strings.TrimSpace(item.Source)
		}
	}
	return ""
}

func (result *prepareTurnLorebookReferenceResult) deliveredSourceRefs() []string {
	refs := []string{}
	if result == nil {
		return refs
	}
	for _, item := range result.delivered {
		refs = appendUniqueStringValues(refs, item.SourceRefs...)
	}
	return refs
}

func (result *prepareTurnLorebookReferenceResult) publisherItems() []map[string]any {
	items := []map[string]any{}
	if result == nil {
		return items
	}
	for _, item := range result.delivered {
		items = append(items, map[string]any{
			"final_text":          item.Text,
			"source_refs":         append([]string(nil), item.SourceRefs...),
			"visibility_boundary": "delivered_lorebook_reference",
			"authority":           "reference_only",
		})
	}
	return items
}

func attachPrepareTurnLorebookPublisherSupport(executionContract, supportPacket map[string]any, result *prepareTurnLorebookReferenceResult) {
	if executionContract == nil || supportPacket == nil || result == nil || len(result.delivered) == 0 {
		return
	}
	refs := result.deliveredSourceRefs()
	sourceRefs := mapFromAny(executionContract["source_refs"])
	sourceRefs["lorebook_reference"] = refs
	all := stringSliceFromAny(sourceRefs["all"])
	all = appendUniqueStringValues(all, refs...)
	sourceRefs["all"] = all
	executionContract["source_refs"] = sourceRefs

	publisherItems := result.publisherItems()
	supportPacket["delivered_lorebook_reference"] = publisherItems
	supportPacket["delivered_lorebook_reference_count"] = len(publisherItems)
	supportPacket["status"] = "ready"
	result.PublisherCount = len(publisherItems)
}

func (s *Server) prepareTurnLorebookReferenceSearch(
	ctx context.Context,
	chatSessionID string,
	selectionQuery string,
	mode string,
	observation *dto.PrepareTurnLorebookReferenceScopeV1,
) prepareTurnLorebookReferenceResult {
	mode = normalizePrepareTurnLorebookMode(mode)
	result := newPrepareTurnLorebookReferenceResult(mode)
	if mode == prepareTurnLorebookModeInvalid {
		result.Status = "unavailable"
		result.ReasonCode = "lorebook_reference_mode_invalid"
		return result
	}
	if observation == nil {
		result.Status = "unavailable"
		result.ReasonCode = "lorebook_scope_unobserved"
		return result
	}
	if strings.TrimSpace(observation.ContractVersion) != prepareTurnLorebookScopeContractV1 {
		result.Status = "unavailable"
		result.ReasonCode = "lorebook_scope_contract_unsupported"
		return result
	}
	switch strings.TrimSpace(observation.ObservationState) {
	case "observed":
		if observation.CharacterIndex == nil || observation.ChatIndex == nil || !observation.EnabledModulesObserved {
			result.Status = "unavailable"
			result.ReasonCode = "lorebook_scope_observation_incomplete"
			return result
		}
		result.ScopeStatus = "observed"
	case "partial":
		result.ScopeStatus = "partial"
	default:
		result.Status = "unavailable"
		result.ReasonCode = "lorebook_scope_unobserved"
		return result
	}
	reader, ok := s.Store.(store.LorebookReferenceStore)
	if !ok {
		result.Status = "unavailable"
		result.ReasonCode = "lorebook_reference_store_unavailable"
		return result
	}
	current, err := reader.GetLorebookReferenceCurrent(ctx, store.LorebookReferenceScope{
		ChatSessionID:          strings.TrimSpace(chatSessionID),
		CharacterIndex:         observation.CharacterIndex,
		ChatIndex:              observation.ChatIndex,
		EnabledModuleIDs:       append([]string(nil), observation.EnabledModuleIDs...),
		EnabledModulesObserved: observation.EnabledModulesObserved,
	})
	result.StoreRead = true
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			result.Status = "empty"
			result.ReasonCode = "lorebook_scope_snapshot_not_found"
			return result
		}
		result.Status = "unavailable"
		result.ReasonCode = "lorebook_reference_read_failed"
		return result
	}
	if current == nil {
		result.Status = "empty"
		result.ReasonCode = "lorebook_scope_snapshot_not_found"
		return result
	}
	result.CatalogCount = len(current.Entries)
	query := normalizePrepareTurnLorebookText(selectionQuery)
	queryTokens := prepareTurnLorebookContextTokens(query)
	for _, entry := range current.Entries {
		if strings.TrimSpace(entry.Content) == "" {
			continue
		}
		searchText := strings.TrimSpace(entry.NormalizedSearch)
		if searchText == "" {
			searchText = strings.Join([]string{entry.Key, entry.SecondKey, entry.Comment, entry.Content}, "\n")
		}
		candidate := prepareTurnLorebookCandidate{
			Entry:          entry,
			EntryRef:       prepareTurnLorebookEntryRef(current.ScopeID, entry),
			MatchedKeys:    prepareTurnLorebookContextMatchedKeys(query, entry.Key, entry.SecondKey),
			ContextOverlap: prepareTurnLorebookContextOverlap(queryTokens, prepareTurnLorebookContextTokens(normalizePrepareTurnLorebookText(searchText))),
		}
		if len(candidate.MatchedKeys) > 0 {
			result.KeyMatchedCandidateCount++
		}
		if len(candidate.MatchedKeys) > 0 || candidate.ContextOverlap > 0 {
			result.ContextMatchedCandidateCount++
		}
		if entry.AlwaysActive != nil && *entry.AlwaysActive {
			result.AlwaysActiveCandidateCount++
		}
		result.candidates = append(result.candidates, candidate)
	}
	for _, candidate := range result.candidates {
		result.CandidateRefs = append(result.CandidateRefs, map[string]any{
			"entry_ref":         candidate.EntryRef,
			"always_active":     candidate.Entry.AlwaysActive != nil && *candidate.Entry.AlwaysActive,
			"matched_keys":      append([]string(nil), candidate.MatchedKeys...),
			"context_overlap":   candidate.ContextOverlap,
			"final_disposition": "candidate",
			"delivered":         false,
		})
	}
	result.CandidateCount = len(result.candidates)
	if result.CandidateCount == 0 {
		result.Status = "empty"
		result.ReasonCode = "lorebook_no_candidates"
		return result
	}
	if result.ScopeStatus == "partial" {
		result.Status = "partial"
		result.ReasonCode = "lorebook_candidates_from_exact_partial_scope"
		return result
	}
	result.Status = "ready"
	result.ReasonCode = "lorebook_candidates_found"
	return result
}

func normalizePrepareTurnLorebookText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func buildPrepareTurnLorebookSelectionQuery(rawUserInput string, previousCompleted []store.ChatLog, maxContextChars int) string {
	parts := nonEmptyStrings([]string{strings.TrimSpace(rawUserInput)})
	remaining := maxInt(0, maxContextChars)
	for _, item := range previousCompleted {
		if remaining == 0 {
			break
		}
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.Join(strings.Fields(item.Content), " ")
		if content == "" {
			continue
		}
		content = truncateRunes(content, remaining)
		if content == "" {
			continue
		}
		parts = append(parts, content)
		remaining -= len([]rune(content))
	}
	return strings.Join(parts, "\n")
}

func prepareTurnLorebookContextMatchedKeys(query string, values ...string) []string {
	matches := []string{}
	for _, value := range values {
		for _, phrase := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ';' || r == '|' || r == '\n' || r == '\r'
		}) {
			phrase = normalizePrepareTurnLorebookText(phrase)
			if phrase != "" && strings.Contains(query, phrase) {
				matches = appendUniqueStringValues(matches, phrase)
			}
		}
	}
	return matches
}

func prepareTurnLorebookContextTokens(value string) map[string]struct{} {
	tokens := map[string]struct{}{}
	for _, token := range strings.FieldsFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		token = strings.TrimSpace(token)
		if prepareTurnLorebookContextTokenEligible(token) {
			tokens[token] = struct{}{}
		}
	}
	return tokens
}

func prepareTurnLorebookContextTokenEligible(token string) bool {
	runes := []rune(strings.TrimSpace(token))
	if len(runes) < 2 {
		return false
	}
	allASCII := true
	for _, r := range runes {
		if r > unicode.MaxASCII {
			allASCII = false
			break
		}
	}
	return !allASCII || len(runes) >= 4
}

func prepareTurnLorebookContextOverlap(left, right map[string]struct{}) int {
	count := 0
	for token := range left {
		if _, exists := right[token]; exists {
			count++
		}
	}
	return count
}

func prepareTurnLorebookEntryRef(scopeID int64, entry store.LorebookReferenceEntryObservation) string {
	if hostID := strings.TrimSpace(entry.HostEntryID); hostID != "" {
		return "host_entry:" + hostID
	}
	return fmt.Sprintf("scope:%d/ordinal:%d", scopeID, entry.EntryOrdinal)
}
