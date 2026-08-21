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
	prepareTurnLorebookReferenceContractV1 = "lorebook_reference_recall.v1"
	prepareTurnLorebookScopeContractV1     = "lorebook_reference_scope.v1"
	prepareTurnLorebookModeOff             = "off" // legacy input normalized to reference_assist
	prepareTurnLorebookModeSearchOnly      = "search_only"
	prepareTurnLorebookModeReferenceAssist = "reference_assist"
	prepareTurnLorebookModeInvalid         = "invalid"
)

type prepareTurnLorebookCandidate struct {
	Entry    store.LorebookReferenceEntryObservation
	EntryRef string
	Methods  []string
	Overlap  int
}

type prepareTurnLorebookDeliveredItem struct {
	Text       string
	SourceRefs []string
}

type prepareTurnLorebookReferenceResult struct {
	ContractVersion string           `json:"contract_version"`
	Mode            string           `json:"mode"`
	Status          string           `json:"status"`
	ReasonCode      string           `json:"reason_code"`
	ScopeStatus     string           `json:"scope_status"`
	StoreRead       bool             `json:"store_read"`
	CatalogCount    int              `json:"catalog_count"`
	CandidateCount  int              `json:"candidate_count"`
	MethodCounts    map[string]int   `json:"method_counts"`
	CandidateRefs   []map[string]any `json:"candidate_refs"`
	SelectedCount   int              `json:"selected_count"`
	DeferredCount   int              `json:"deferred_count"`
	DuplicateCount  int              `json:"duplicate_suppressed_count"`
	BudgetChars     int              `json:"budget_chars"`
	UsedChars       int              `json:"used_chars"`
	DeliveryCount   int              `json:"delivery_count"`
	DeliveryChars   int              `json:"delivery_chars"`
	PublisherCount  int              `json:"publisher_count"`
	candidates      []prepareTurnLorebookCandidate
	deliveryText    string
	delivered       []prepareTurnLorebookDeliveredItem
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
		ContractVersion: prepareTurnLorebookReferenceContractV1,
		Mode:            mode,
		Status:          "disabled",
		ReasonCode:      "lorebook_reference_disabled",
		ScopeStatus:     "unobserved",
		MethodCounts:    map[string]int{"exact_phrase": 0, "key": 0, "lexical": 0},
		CandidateRefs:   []map[string]any{},
		DeliveryCount:   0,
		DeliveryChars:   0,
		PublisherCount:  0,
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
	if !injectionEnabled {
		result.Status = "deferred"
		result.ReasonCode = "lorebook_reference_injection_disabled"
		return
	}
	if budgetChars < 0 {
		budgetChars = 0
	}
	result.BudgetChars = budgetChars
	if budgetChars == 0 {
		result.Status = "deferred"
		result.ReasonCode = "lorebook_reference_budget_zero"
		result.DeferredCount = len(result.candidates)
		return
	}

	nativeTexts := make([]string, 0, len(messages)+len(deliveredContextTexts)+1)
	if strings.TrimSpace(rawUserInput) != "" {
		nativeTexts = append(nativeTexts, rawUserInput)
	}
	for _, message := range messages {
		if text := extractionStringFromAny(message["content"]); strings.TrimSpace(text) != "" {
			nativeTexts = append(nativeTexts, text)
		}
	}
	for _, text := range deliveredContextTexts {
		if strings.TrimSpace(text) != "" {
			nativeTexts = append(nativeTexts, text)
		}
	}

	type groupedCandidate struct {
		Text              string
		SourceRefs        []string
		DirectlyActivated bool
	}
	groups := []groupedCandidate{}
	groupIndex := map[string]int{}
	for _, candidate := range result.candidates {
		text := strings.TrimSpace(candidate.Entry.Content)
		normalized := normalizePrepareTurnLorebookText(text)
		if normalized == "" {
			continue
		}
		ref := candidate.EntryRef
		directlyActivated := false
		for _, method := range candidate.Methods {
			if method == "key" {
				directlyActivated = true
				break
			}
		}
		if candidate.Entry.AlwaysActive != nil && *candidate.Entry.AlwaysActive {
			directlyActivated = true
		}
		if index, exists := groupIndex[normalized]; exists {
			groups[index].SourceRefs = appendUniqueStringValues(groups[index].SourceRefs, ref)
			groups[index].DirectlyActivated = groups[index].DirectlyActivated || directlyActivated
			result.DuplicateCount++
			continue
		}
		groupIndex[normalized] = len(groups)
		groups = append(groups, groupedCandidate{
			Text:              text,
			SourceRefs:        []string{ref},
			DirectlyActivated: directlyActivated,
		})
	}
	result.SelectedCount = len(groups)

	header := "[Archive Center — Lorebook Reference]"
	used := 0
	lines := []string{}
	activationDeferred := 0
	budgetDeferred := 0
	for _, group := range groups {
		if prepareTurnLorebookExactTextPresent(group.Text, nativeTexts) {
			result.DuplicateCount++
			continue
		}
		if !group.DirectlyActivated {
			activationDeferred++
			result.DeferredCount++
			continue
		}
		line := "- " + group.Text
		additional := len([]rune(line))
		if len(lines) == 0 {
			additional += len([]rune(header)) + 1
		} else {
			additional++
		}
		if used+additional > budgetChars {
			budgetDeferred++
			result.DeferredCount++
			continue
		}
		used += additional
		lines = append(lines, line)
		result.delivered = append(result.delivered, prepareTurnLorebookDeliveredItem{
			Text:       group.Text,
			SourceRefs: append([]string(nil), group.SourceRefs...),
		})
	}
	result.UsedChars = used
	result.DeliveryCount = len(result.delivered)
	result.DeliveryChars = used
	if len(lines) == 0 {
		switch {
		case activationDeferred > 0 && budgetDeferred > 0:
			result.Status = "deferred"
			result.ReasonCode = "lorebook_reference_candidates_deferred"
		case activationDeferred > 0:
			result.Status = "deferred"
			result.ReasonCode = "lorebook_reference_not_directly_activated"
		case budgetDeferred > 0:
			result.Status = "deferred"
			result.ReasonCode = "lorebook_reference_budget_exhausted"
		case result.DuplicateCount > 0:
			result.Status = "empty"
			result.ReasonCode = "lorebook_reference_exact_duplicates_suppressed"
		default:
			result.Status = "empty"
			result.ReasonCode = "lorebook_no_relevant_candidates"
		}
		return
	}
	result.deliveryText = header + "\n" + strings.Join(lines, "\n")
	result.Status = "ready"
	result.ReasonCode = "lorebook_reference_delivered"
}

func prepareTurnLorebookExactTextPresent(content string, haystacks []string) bool {
	content = normalizePrepareTurnLorebookText(content)
	if content == "" {
		return false
	}
	for _, haystack := range haystacks {
		if normalized := normalizePrepareTurnLorebookText(haystack); normalized != "" && strings.Contains(normalized, content) {
			return true
		}
	}
	return false
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
	rawUserInput string,
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
	query := normalizePrepareTurnLorebookText(rawUserInput)
	if query == "" {
		result.Status = "empty"
		result.ReasonCode = "lorebook_query_empty"
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
	queryTokens := prepareTurnLorebookTokens(query)
	for _, entry := range current.Entries {
		candidate := prepareTurnLorebookCandidate{
			Entry:    entry,
			EntryRef: prepareTurnLorebookEntryRef(current.ScopeID, entry),
		}
		if entry.AlwaysActive != nil && *entry.AlwaysActive {
			candidate.Methods = append(candidate.Methods, "always_active")
			result.MethodCounts["always_active"]++
		}
		content := normalizePrepareTurnLorebookText(entry.Content)
		if content != "" && strings.Contains(query, content) {
			candidate.Methods = append(candidate.Methods, "exact_phrase")
			result.MethodCounts["exact_phrase"]++
		}
		if prepareTurnLorebookKeyMatch(query, entry.Key, entry.SecondKey) {
			candidate.Methods = append(candidate.Methods, "key")
			result.MethodCounts["key"]++
		}
		searchText := entry.NormalizedSearch
		if strings.TrimSpace(searchText) == "" {
			searchText = strings.Join([]string{entry.Key, entry.SecondKey, entry.Comment, entry.Content}, "\n")
		}
		candidate.Overlap = prepareTurnLorebookTokenOverlap(queryTokens, prepareTurnLorebookTokens(searchText))
		if candidate.Overlap > 0 {
			candidate.Methods = append(candidate.Methods, "lexical")
			result.MethodCounts["lexical"]++
		}
		if len(candidate.Methods) == 0 {
			continue
		}
		result.candidates = append(result.candidates, candidate)
	}
	sort.SliceStable(result.candidates, func(i, j int) bool {
		left, right := result.candidates[i], result.candidates[j]
		if prepareTurnLorebookMethodRank(left.Methods) != prepareTurnLorebookMethodRank(right.Methods) {
			return prepareTurnLorebookMethodRank(left.Methods) > prepareTurnLorebookMethodRank(right.Methods)
		}
		if left.Overlap != right.Overlap {
			return left.Overlap > right.Overlap
		}
		return left.Entry.EntryOrdinal < right.Entry.EntryOrdinal
	})
	for _, candidate := range result.candidates {
		result.CandidateRefs = append(result.CandidateRefs, map[string]any{
			"entry_ref": candidate.EntryRef,
			"methods":   append([]string(nil), candidate.Methods...),
		})
	}
	result.CandidateCount = len(result.candidates)
	if result.CandidateCount == 0 {
		result.Status = "empty"
		result.ReasonCode = "lorebook_no_relevant_candidates"
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

func prepareTurnLorebookKeyMatch(query string, values ...string) bool {
	for _, value := range values {
		for _, phrase := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ';' || r == '|' || r == '\n' || r == '\r'
		}) {
			phrase = normalizePrepareTurnLorebookText(phrase)
			if phrase != "" && strings.Contains(query, phrase) {
				return true
			}
		}
	}
	return false
}

func prepareTurnLorebookTokens(value string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, token := range strings.FieldsFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		token = strings.TrimSpace(token)
		if token != "" {
			result[token] = struct{}{}
		}
	}
	return result
}

func prepareTurnLorebookTokenOverlap(left, right map[string]struct{}) int {
	count := 0
	for token := range left {
		if _, ok := right[token]; ok {
			count++
		}
	}
	return count
}

func prepareTurnLorebookMethodRank(methods []string) int {
	rank := 0
	for _, method := range methods {
		switch method {
		case "exact_phrase":
			if rank < 3 {
				rank = 3
			}
		case "key":
			if rank < 2 {
				rank = 2
			}
		case "lexical":
			if rank < 1 {
				rank = 1
			}
		}
	}
	return rank
}

func prepareTurnLorebookEntryRef(scopeID int64, entry store.LorebookReferenceEntryObservation) string {
	if hostID := strings.TrimSpace(entry.HostEntryID); hostID != "" {
		return "host_entry:" + hostID
	}
	return fmt.Sprintf("scope:%d/ordinal:%d", scopeID, entry.EntryOrdinal)
}
