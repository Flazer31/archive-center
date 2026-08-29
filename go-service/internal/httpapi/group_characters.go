package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Character: R1 read, R2 write

func (s *Server) handleCharactersGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	items, err := s.Store.ListCharacterStates(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.CharacterState{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	events, err := s.Store.ListCharacterEvents(r.Context(), sid, "")
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			events = []store.CharacterEvent{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	events = nonNilSlice(events)
	projection := s.canonicalCharacterReadProjection(r.Context(), sid, items, events)
	items = projection.States
	events = projection.Events
	referenceTurn := s.characterReferenceTurn(r.Context(), sid, items)
	recentMentionText, recentMentionKeywords := s.characterRecentMentionSignal(r.Context(), sid, referenceTurn)
	characters := characterResponseItems(items, events, referenceTurn, recentMentionText, recentMentionKeywords)
	for _, character := range characters {
		name := strings.TrimSpace(stringFromMap(character, "character_name"))
		key := comparableEntityKey(name)
		character["aliases"] = nonNilSlice(projection.Aliases[key])
		if stableID := strings.TrimSpace(projection.StableIDs[key]); stableID != "" {
			character["stable_entity_id"] = stableID
		}
	}
	identityLinks := []map[string]any{}
	if reader, ok := s.Store.(store.EntityIdentityCatalogReader); ok {
		if links, readErr := reader.ListReviewedEntityIdentityLinks(r.Context(), sid); readErr == nil {
			labels := map[string]string{}
			characterIDs := map[string]bool{}
			if identities, identityErr := reader.ListActiveEntityIdentities(r.Context(), sid); identityErr == nil {
				for _, identity := range identities {
					labels[identity.StableEntityID] = identity.CanonicalLabel
					if identity.EntityKind == "character" {
						characterIDs[identity.StableEntityID] = true
					}
				}
			}
			for _, link := range links {
				if !characterIDs[link.SourceEntityID] || !characterIDs[link.TargetEntityID] {
					continue
				}
				identityLinks = append(identityLinks, map[string]any{
					"link_id": link.LinkID, "source_entity_id": link.SourceEntityID,
					"source_label": labels[link.SourceEntityID], "target_entity_id": link.TargetEntityID,
					"target_label": labels[link.TargetEntityID], "link_state": link.LinkState,
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"characters":      characters,
		"identity_links":  identityLinks,
		"count":           len(characters),
		"omitted_count":   characterOmittedCount(items, events, referenceTurn, recentMentionText, recentMentionKeywords),
	})
}

const characterIdentityManualMergeContract = "character_identity_manual_merge.v1"

type characterIdentityMergeRequest struct {
	TargetEntityID  string   `json:"target_entity_id"`
	SourceEntityIDs []string `json:"source_entity_ids"`
}

type characterIdentityImpact struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
	Detail string `json:"detail,omitempty"`
}

type characterIdentityCatalog struct {
	Identities map[string]store.EntityIdentity
	Surfaces   []store.EntityIdentitySurface
	Links      []store.EntityIdentityLink
}

type characterIdentitySourceSelection struct {
	RequestedID string
	RootID      string
	Status      string
	Detail      string
}

func (s *Server) handleCharacterIdentityMergePreview(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	req, ok := decodeCharacterIdentityMergeRequest(w, r, sid)
	if !ok {
		return
	}
	catalog, selected, target, sourceSelections, err := s.characterIdentityMergeSelection(r.Context(), sid, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "character_identity_merge_invalid", err.Error())
		return
	}
	impacts := s.characterIdentityMergeImpacts(r.Context(), sid, selected, catalog)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "contract_version": characterIdentityManualMergeContract,
		"chat_session_id": sid, "target": target,
		"sources":        characterIdentitySelectionItems(req.SourceEntityIDs, catalog.Identities),
		"source_results": characterIdentitySourceSelectionItems(sourceSelections),
		"impacts":        impacts, "writes_performed": false,
	})
}

func (s *Server) handleCharacterIdentityMerge(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	req, ok := decodeCharacterIdentityMergeRequest(w, r, sid)
	if !ok {
		return
	}
	catalog, _, target, sourceSelections, err := s.characterIdentityMergeSelection(r.Context(), sid, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "character_identity_merge_invalid", err.Error())
		return
	}
	writer, ok := s.Store.(store.EntityIdentityLinkWriter)
	if !ok {
		writeError(w, http.StatusConflict, "character_identity_merge_unavailable", "entity identity links are not writable")
		return
	}
	targetID := strings.TrimSpace(target.StableEntityID)
	results := make([]map[string]any, 0, len(sourceSelections))
	succeeded := 0
	for _, selection := range sourceSelections {
		requestedSourceID := selection.RequestedID
		if selection.Status != "ready" {
			results = append(results, map[string]any{"source_entity_id": requestedSourceID, "status": "failed", "detail": selection.Detail})
			continue
		}
		sourceID := selection.RootID
		if sourceID == targetID {
			results = append(results, map[string]any{"source_entity_id": requestedSourceID, "status": "already_merged", "target_entity_id": targetID})
			continue
		}
		link := characterIdentityManualLink(sid, sourceID, targetID, store.EntityIdentityLinkStateReviewed)
		if err := writer.SaveEntityIdentityLink(r.Context(), &link); err != nil {
			results = append(results, map[string]any{"source_entity_id": requestedSourceID, "status": "failed", "detail": err.Error()})
			continue
		}
		succeeded++
		results = append(results, map[string]any{"source_entity_id": requestedSourceID, "status": "linked", "target_entity_id": targetID, "link_id": link.LinkID})
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid, EventType: "character_identity_manual_merge", TargetType: "entity_identity",
		Summary:     fmt.Sprintf("linked %d character identities to %s", succeeded, target.CanonicalLabel),
		DetailsJSON: mustCompactJSON(map[string]any{"target_entity_id": targetID, "results": results}), Source: s.storeWriteSource(),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "contract_version": characterIdentityManualMergeContract,
		"chat_session_id": sid, "target": target, "results": results,
		"linked_count": succeeded, "stored_rows_rewritten": 0, "critic_calls": 0, "vector_reindex_queued": false,
		"catalog_identity_count": len(catalog.Identities),
	})
}

func (s *Server) handleCharacterIdentityUnmerge(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	req, ok := decodeCharacterIdentityMergeRequest(w, r, sid)
	if !ok {
		return
	}
	catalog, err := s.characterIdentityCatalogForSession(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusBadRequest, "character_identity_unmerge_invalid", err.Error())
		return
	}
	// Unmerge addresses stored direct edges without resolving them first. This
	// also lets a user break an accidental ambiguous or cyclic link graph.
	target, exists := catalog.Identities[req.TargetEntityID]
	if !exists {
		writeError(w, http.StatusBadRequest, "character_identity_unmerge_invalid", "target entity is not an active character in this session")
		return
	}
	for _, sourceID := range req.SourceEntityIDs {
		if _, exists := catalog.Identities[sourceID]; !exists {
			writeError(w, http.StatusBadRequest, "character_identity_unmerge_invalid", "source entity is not an active character in this session")
			return
		}
	}
	writer, ok := s.Store.(store.EntityIdentityLinkWriter)
	if !ok {
		writeError(w, http.StatusConflict, "character_identity_unmerge_unavailable", "entity identity links are not writable")
		return
	}
	active := map[string]store.EntityIdentityLink{}
	for _, link := range catalog.Links {
		active[link.SourceEntityID+"\x1f"+link.TargetEntityID] = link
	}
	results := make([]map[string]any, 0, len(req.SourceEntityIDs))
	revoked := 0
	for _, sourceID := range uniqueNonEmptyStrings(req.SourceEntityIDs) {
		key := sourceID + "\x1f" + target.StableEntityID
		if _, exists := active[key]; !exists {
			results = append(results, map[string]any{"source_entity_id": sourceID, "status": "not_linked", "target_entity_id": target.StableEntityID})
			continue
		}
		link := characterIdentityManualLink(sid, sourceID, target.StableEntityID, store.EntityIdentityLinkStateRevoked)
		if err := writer.SaveEntityIdentityLink(r.Context(), &link); err != nil {
			results = append(results, map[string]any{"source_entity_id": sourceID, "status": "failed", "detail": err.Error()})
			continue
		}
		revoked++
		results = append(results, map[string]any{"source_entity_id": sourceID, "status": "unlinked", "target_entity_id": target.StableEntityID})
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid, EventType: "character_identity_manual_unmerge", TargetType: "entity_identity",
		Summary:     fmt.Sprintf("revoked %d character identity links", revoked),
		DetailsJSON: mustCompactJSON(map[string]any{"target_entity_id": target.StableEntityID, "results": results}), Source: s.storeWriteSource(),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "contract_version": characterIdentityManualMergeContract,
		"chat_session_id": sid, "results": results, "unlinked_count": revoked,
		"stored_rows_deleted": 0, "critic_calls": 0, "vector_reindex_queued": false,
	})
}

func decodeCharacterIdentityMergeRequest(w http.ResponseWriter, r *http.Request, sid string) (characterIdentityMergeRequest, bool) {
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return characterIdentityMergeRequest{}, false
	}
	var req characterIdentityMergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return characterIdentityMergeRequest{}, false
	}
	req.TargetEntityID = strings.TrimSpace(req.TargetEntityID)
	req.SourceEntityIDs = uniqueNonEmptyStrings(req.SourceEntityIDs)
	if req.TargetEntityID == "" || len(req.SourceEntityIDs) == 0 {
		writeError(w, http.StatusBadRequest, "missing_param", "target_entity_id and source_entity_ids are required")
		return characterIdentityMergeRequest{}, false
	}
	return req, true
}

func (s *Server) characterIdentityMergeSelection(ctx context.Context, sid string, req characterIdentityMergeRequest) (characterIdentityCatalog, map[string]bool, store.EntityIdentity, []characterIdentitySourceSelection, error) {
	return s.entityIdentityMergeSelection(ctx, sid, req, "character")
}

func (s *Server) entityIdentityMergeSelection(ctx context.Context, sid string, req characterIdentityMergeRequest, entityKind string) (characterIdentityCatalog, map[string]bool, store.EntityIdentity, []characterIdentitySourceSelection, error) {
	catalog, err := s.entityIdentityCatalogForSession(ctx, sid, entityKind)
	if err != nil {
		return characterIdentityCatalog{}, nil, store.EntityIdentity{}, nil, err
	}
	targetID, err := s.characterIdentityRootForWrite(ctx, sid, req.TargetEntityID)
	if err != nil {
		return catalog, nil, store.EntityIdentity{}, nil, err
	}
	target, exists := catalog.Identities[targetID]
	if !exists {
		return catalog, nil, store.EntityIdentity{}, nil, fmt.Errorf("target entity %q is not an active %s in this session", req.TargetEntityID, entityKind)
	}
	selected := map[string]bool{targetID: true, req.TargetEntityID: true}
	selections := make([]characterIdentitySourceSelection, 0, len(req.SourceEntityIDs))
	for _, sourceID := range req.SourceEntityIDs {
		if _, exists := catalog.Identities[sourceID]; !exists {
			selections = append(selections, characterIdentitySourceSelection{
				RequestedID: sourceID, Status: "unavailable",
				Detail: fmt.Sprintf("source entity %q is not an active %s in this session", sourceID, entityKind),
			})
			continue
		}
		selected[sourceID] = true
		rootID, rootErr := s.characterIdentityRootForWrite(ctx, sid, sourceID)
		if rootErr != nil {
			selections = append(selections, characterIdentitySourceSelection{RequestedID: sourceID, Status: "unavailable", Detail: rootErr.Error()})
			continue
		}
		if _, exists := catalog.Identities[rootID]; !exists {
			selections = append(selections, characterIdentitySourceSelection{
				RequestedID: sourceID, Status: "unavailable",
				Detail: fmt.Sprintf("resolved source entity %q is not an active %s in this session", rootID, entityKind),
			})
			continue
		}
		selected[rootID] = true
		selections = append(selections, characterIdentitySourceSelection{RequestedID: sourceID, RootID: rootID, Status: "ready"})
	}
	return catalog, selected, target, selections, nil
}

func (s *Server) characterIdentityCatalogForSession(ctx context.Context, sid string) (characterIdentityCatalog, error) {
	return s.entityIdentityCatalogForSession(ctx, sid, "character")
}

func (s *Server) entityIdentityCatalogForSession(ctx context.Context, sid, entityKind string) (characterIdentityCatalog, error) {
	reader, ok := s.Store.(store.EntityIdentityCatalogReader)
	if !ok {
		return characterIdentityCatalog{}, store.ErrNotEnabled
	}
	identities, err := reader.ListActiveEntityIdentities(ctx, sid)
	if err != nil {
		return characterIdentityCatalog{}, err
	}
	surfaces, err := reader.ListActiveEntityIdentitySurfaces(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return characterIdentityCatalog{}, err
	}
	links, err := reader.ListReviewedEntityIdentityLinks(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return characterIdentityCatalog{}, err
	}
	catalog := characterIdentityCatalog{Identities: map[string]store.EntityIdentity{}, Surfaces: nonNilSlice(surfaces), Links: nonNilSlice(links)}
	for _, identity := range identities {
		if identity.ChatSessionID == sid && (entityKind == "" || identity.EntityKind == entityKind) {
			catalog.Identities[strings.TrimSpace(identity.StableEntityID)] = identity
		}
	}
	return catalog, nil
}

func (s *Server) characterIdentityRoot(ctx context.Context, sid, entityID string) string {
	entityID = strings.TrimSpace(entityID)
	resolver, ok := s.Store.(store.ReviewedEntityIdentityResolver)
	if !ok || entityID == "" {
		return entityID
	}
	root, err := resolver.ResolveReviewedCanonicalEntityID(ctx, sid, entityID)
	if err == nil && strings.TrimSpace(root) != "" {
		return strings.TrimSpace(root)
	}
	return entityID
}

func (s *Server) characterIdentityRootForWrite(ctx context.Context, sid, entityID string) (string, error) {
	entityID = strings.TrimSpace(entityID)
	resolver, ok := s.Store.(store.ReviewedEntityIdentityResolver)
	if !ok || entityID == "" {
		return entityID, nil
	}
	root, err := resolver.ResolveReviewedCanonicalEntityID(ctx, sid, entityID)
	if errors.Is(err, store.ErrNotFound) {
		return entityID, nil
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(root) == "" {
		return entityID, nil
	}
	return strings.TrimSpace(root), nil
}

func characterIdentityManualLink(sid, sourceID, targetID, state string) store.EntityIdentityLink {
	now := time.Now().UTC()
	return store.EntityIdentityLink{
		LinkID:        entityIdentityStableID("character_identity_manual_link", sid, sourceID, targetID),
		ChatSessionID: sid, SourceEntityID: sourceID, TargetEntityID: targetID,
		LinkKind: store.EntityIdentityLinkKindCanonicalEquivalence, LinkState: state,
		EvidenceJSON:    mustCompactJSON(map[string]any{"contract_version": characterIdentityManualMergeContract, "operator_explicit": true, "link_state": state}),
		MappingRevision: 1, SourceContract: characterIdentityManualMergeContract,
		SourceRevision: characterIdentityManualMergeContract + ":" + sourceID + ":" + targetID,
		CreatedAt:      now, UpdatedAt: now,
	}
}

func characterIdentitySelectionItems(ids []string, identities map[string]store.EntityIdentity) []map[string]any {
	out := []map[string]any{}
	for _, id := range uniqueNonEmptyStrings(ids) {
		if item, ok := identities[id]; ok {
			out = append(out, map[string]any{"stable_entity_id": id, "canonical_label": item.CanonicalLabel})
		}
	}
	return out
}

func characterIdentitySourceSelectionItems(items []characterIdentitySourceSelection) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{"source_entity_id": item.RequestedID, "status": item.Status}
		if item.RootID != "" {
			row["resolved_entity_id"] = item.RootID
		}
		if item.Detail != "" {
			row["detail"] = item.Detail
		}
		out = append(out, row)
	}
	return out
}

func (s *Server) characterIdentityMergeImpacts(ctx context.Context, sid string, selected map[string]bool, catalog characterIdentityCatalog) map[string]characterIdentityImpact {
	impacts := map[string]characterIdentityImpact{}
	selectedNames := map[string]bool{}
	for id := range selected {
		if identity, ok := catalog.Identities[id]; ok {
			selectedNames[comparableEntityKey(identity.CanonicalLabel)] = true
		}
	}
	for _, surface := range catalog.Surfaces {
		if selected[surface.StableEntityID] {
			selectedNames[comparableEntityKey(surface.SurfaceText)] = true
		}
	}
	impacts["alias_surfaces"] = characterIdentityImpact{Status: "ready", Count: len(selectedNames)}

	states, err := s.Store.ListCharacterStates(ctx, sid)
	if err != nil {
		impacts["character_states"] = characterIdentityUnavailableImpact(err)
	} else {
		count := 0
		for _, item := range states {
			if selectedNames[comparableEntityKey(item.CharacterName)] {
				count++
			}
		}
		impacts["character_states"] = characterIdentityImpact{Status: "ready", Count: count}
	}
	events, err := s.Store.ListCharacterEvents(ctx, sid, "")
	if err != nil {
		impacts["character_events"] = characterIdentityUnavailableImpact(err)
	} else {
		count := 0
		for _, item := range events {
			if selectedNames[comparableEntityKey(item.CharacterName)] {
				count++
			}
		}
		impacts["character_events"] = characterIdentityImpact{Status: "ready", Count: count}
	}
	triples, err := s.Store.ListKGTriples(ctx, sid)
	if err != nil {
		impacts["relationship_knowledge"] = characterIdentityUnavailableImpact(err)
		impacts["items_equipment"] = characterIdentityUnavailableImpact(err)
	} else {
		relations, equipment := 0, 0
		for _, item := range triples {
			involvesSelected := selectedNames[comparableEntityKey(item.Subject)] || selectedNames[comparableEntityKey(item.Object)]
			if !involvesSelected {
				continue
			}
			relations++
			predicate := strings.ToLower(strings.TrimSpace(item.Predicate))
			if strings.Contains(predicate, "equip") || strings.Contains(predicate, "possess") ||
				strings.Contains(predicate, "own") || strings.Contains(predicate, "wear") ||
				strings.Contains(predicate, "장착") || strings.Contains(predicate, "소유") || strings.Contains(predicate, "착용") {
				equipment++
			}
		}
		impacts["relationship_knowledge"] = characterIdentityImpact{Status: "ready", Count: relations}
		impacts["items_equipment"] = characterIdentityImpact{Status: "ready", Count: equipment}
	}
	if reader, ok := s.Store.(store.ProtagonistEntityMemoryStore); ok {
		memories, readErr := reader.ListProtagonistEntityMemories(ctx, store.ProtagonistEntityMemoryFilter{SourceChatSessionID: sid})
		if readErr != nil {
			impacts["subjective_memories"] = characterIdentityUnavailableImpact(readErr)
		} else {
			count := 0
			for _, item := range memories {
				if selectedNames[comparableEntityKey(item.OwnerEntityName)] || selectedNames[comparableEntityKey(item.SourceCharacterName)] || selected[item.OwnerEntityKey] {
					count++
				}
			}
			impacts["subjective_memories"] = characterIdentityImpact{Status: "ready", Count: count}
		}
	} else {
		impacts["subjective_memories"] = characterIdentityImpact{Status: "unavailable", Detail: "subjective memory reader is not enabled"}
	}
	return impacts
}

func characterIdentityUnavailableImpact(err error) characterIdentityImpact {
	return characterIdentityImpact{Status: "unavailable", Detail: err.Error()}
}

func uniqueNonEmptyStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

type characterReadProjection struct {
	States    []store.CharacterState
	Events    []store.CharacterEvent
	Aliases   map[string][]string
	StableIDs map[string]string
}

// canonicalCharacterReadProjection composes existing alias rows for display
// through the reviewed entity-identity owner. It does not rewrite stored rows
// or infer identity from display-name similarity.
func (s *Server) canonicalCharacterReadProjection(ctx context.Context, sid string, states []store.CharacterState, events []store.CharacterEvent) characterReadProjection {
	out := characterReadProjection{
		States: append([]store.CharacterState(nil), states...), Events: append([]store.CharacterEvent(nil), events...),
		Aliases: map[string][]string{}, StableIDs: map[string]string{},
	}
	resolver, ok := s.Store.(store.UniqueActiveEntitySurfaceIdentityResolver)
	if !ok || strings.TrimSpace(sid) == "" {
		return out
	}
	type resolvedSurface struct {
		groupKey string
		label    string
		stableID string
	}
	resolvedByName := map[string]resolvedSurface{}
	resolve := func(name string) resolvedSurface {
		name = strings.TrimSpace(name)
		if cached, exists := resolvedByName[name]; exists {
			return cached
		}
		fallback := resolvedSurface{groupKey: "surface:" + name, label: name}
		resolved, err := resolver.ResolveUniqueActiveEntityIdentityBySurface(ctx, sid, comparableEntityKey(name))
		if err == nil && strings.TrimSpace(resolved.StableEntityID) != "" {
			fallback.groupKey = "entity:" + strings.TrimSpace(resolved.StableEntityID)
			fallback.stableID = strings.TrimSpace(resolved.StableEntityID)
			if label := strings.TrimSpace(resolved.CanonicalLabel); label != "" {
				fallback.label = label
			}
		}
		resolvedByName[name] = fallback
		return fallback
	}

	sorted := append([]store.CharacterState(nil), states...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TurnIndex != sorted[j].TurnIndex {
			return sorted[i].TurnIndex > sorted[j].TurnIndex
		}
		return sorted[i].ID > sorted[j].ID
	})
	groupOrder := []string{}
	groups := map[string]*store.CharacterState{}
	aliasesByGroup := map[string][]string{}
	stableIDByGroup := map[string]string{}
	for _, state := range sorted {
		rawName := strings.TrimSpace(state.CharacterName)
		if rawName == "" {
			continue
		}
		resolved := resolve(rawName)
		current := groups[resolved.groupKey]
		if current == nil {
			copyState := state
			copyState.CharacterName = resolved.label
			groups[resolved.groupKey] = &copyState
			groupOrder = append(groupOrder, resolved.groupKey)
			current = &copyState
			stableIDByGroup[resolved.groupKey] = resolved.stableID
		}
		if rawName != resolved.label {
			aliasesByGroup[resolved.groupKey] = appendUniqueString(aliasesByGroup[resolved.groupKey], rawName)
		}
		// The list is newest-first. Fill only surfaces absent from the newest
		// row so a profile or voice stored under an older alias is not lost.
		if strings.TrimSpace(current.AppearanceJSON) == "" && strings.TrimSpace(state.AppearanceJSON) != "" {
			current.AppearanceJSON = state.AppearanceJSON
		}
		if strings.TrimSpace(current.PersonalityJSON) == "" && strings.TrimSpace(state.PersonalityJSON) != "" {
			current.PersonalityJSON = state.PersonalityJSON
		}
		if strings.TrimSpace(current.StatusJSON) == "" && strings.TrimSpace(state.StatusJSON) != "" {
			current.StatusJSON = state.StatusJSON
		}
		if strings.TrimSpace(current.RelationshipsJSON) == "" && strings.TrimSpace(state.RelationshipsJSON) != "" {
			current.RelationshipsJSON = state.RelationshipsJSON
		}
		if strings.TrimSpace(current.SpeechStyleJSON) == "" && strings.TrimSpace(state.SpeechStyleJSON) != "" {
			current.SpeechStyleJSON = state.SpeechStyleJSON
		}
		if current.CreatedAt.IsZero() || (!state.CreatedAt.IsZero() && state.CreatedAt.Before(current.CreatedAt)) {
			current.CreatedAt = state.CreatedAt
		}
		if state.UpdatedAt.After(current.UpdatedAt) {
			current.UpdatedAt = state.UpdatedAt
		}
	}
	out.States = make([]store.CharacterState, 0, len(groupOrder))
	for _, groupKey := range groupOrder {
		state := *groups[groupKey]
		stableID := stableIDByGroup[groupKey]
		state.PersonalityJSON = canonicalCharacterTypedProjectionForRead(state.PersonalityJSON, characterProfileContractVersion, stableID, state.CharacterName)
		state.SpeechStyleJSON = canonicalCharacterTypedProjectionForRead(state.SpeechStyleJSON, voiceBehaviorProjectionContractVersion, stableID, state.CharacterName)
		out.States = append(out.States, state)
		nameKey := comparableEntityKey(state.CharacterName)
		out.Aliases[nameKey] = append([]string(nil), aliasesByGroup[groupKey]...)
		out.StableIDs[nameKey] = stableID
	}
	if catalogReader, ok := s.Store.(store.EntityIdentityCatalogReader); ok {
		if surfaces, err := catalogReader.ListActiveEntityIdentitySurfaces(ctx, sid); err == nil {
			for _, surface := range surfaces {
				rootID := s.characterIdentityRoot(ctx, sid, surface.StableEntityID)
				for canonicalNameKey, stableID := range out.StableIDs {
					if stableID != rootID {
						continue
					}
					label := strings.TrimSpace(surface.SurfaceText)
					if label != "" && comparableEntityKey(label) != canonicalNameKey {
						out.Aliases[canonicalNameKey] = appendUniqueString(out.Aliases[canonicalNameKey], label)
					}
				}
			}
		}
	}
	for index := range out.Events {
		resolved := resolve(out.Events[index].CharacterName)
		if resolved.stableID != "" && resolved.label != "" {
			out.Events[index].CharacterName = resolved.label
		}
	}
	return out
}

func canonicalCharacterTypedProjectionForRead(raw, contractVersion, stableID, canonicalLabel string) string {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(stableID) == "" || strings.TrimSpace(canonicalLabel) == "" {
		return raw
	}
	payload := map[string]any{}
	if json.Unmarshal([]byte(raw), &payload) != nil || extractionStringFromAny(payload["contract_version"]) != contractVersion {
		return raw
	}
	payload["subject_entity_id"] = strings.TrimSpace(stableID)
	payload["subject_label"] = strings.TrimSpace(canonicalLabel)
	return mustCompactJSON(payload)
}

// canonicalizeCharacterKGTriplesForRead changes only the request-local copy.
// Historical KG rows keep their original labels and provenance in storage.
func (s *Server) canonicalizeCharacterKGTriplesForRead(ctx context.Context, sid string, items []store.KGTriple) []store.KGTriple {
	out := append([]store.KGTriple(nil), items...)
	canonicalBySession := map[string]map[string]string{}
	for index := range out {
		itemSID := strings.TrimSpace(out[index].ChatSessionID)
		if itemSID == "" {
			itemSID = sid
		}
		canonical, loaded := canonicalBySession[itemSID]
		if !loaded {
			canonical = s.characterCanonicalSurfaceMapForRead(ctx, itemSID)
			canonicalBySession[itemSID] = canonical
		}
		if label := canonical[comparableEntityKey(out[index].Subject)]; label != "" {
			out[index].Subject = label
		}
		if label := canonical[comparableEntityKey(out[index].Object)]; label != "" {
			out[index].Object = label
		}
	}
	return out
}

// characterCanonicalSurfaceMapForRead resolves the request-local character
// catalog without issuing one database lookup per KG endpoint. Ambiguous or
// broken reviewed links are omitted from the map, leaving the original KG
// label intact for that surface.
func (s *Server) characterCanonicalSurfaceMapForRead(ctx context.Context, sid string, preloaded ...characterIdentityCatalog) map[string]string {
	out := map[string]string{}
	sid = strings.TrimSpace(sid)
	if sid == "" || s.Store == nil {
		return out
	}
	catalog := characterIdentityCatalog{}
	if len(preloaded) > 0 {
		catalog = preloaded[0]
	} else {
		var err error
		catalog, err = s.characterIdentityCatalogForSession(ctx, sid)
		if err != nil {
			return out
		}
	}
	characterIdentities := map[string]store.EntityIdentity{}
	for id, identity := range catalog.Identities {
		if identity.ChatSessionID == sid && identity.EntityKind == "character" {
			characterIdentities[id] = identity
		}
	}
	if len(characterIdentities) == 0 {
		return out
	}

	targets := map[string]map[string]bool{}
	for _, link := range catalog.Links {
		if link.ChatSessionID != sid || link.LinkKind != store.EntityIdentityLinkKindCanonicalEquivalence ||
			link.LinkState != store.EntityIdentityLinkStateReviewed {
			continue
		}
		sourceID := strings.TrimSpace(link.SourceEntityID)
		targetID := strings.TrimSpace(link.TargetEntityID)
		if _, ok := characterIdentities[sourceID]; !ok {
			continue
		}
		if _, ok := characterIdentities[targetID]; !ok {
			continue
		}
		if targets[sourceID] == nil {
			targets[sourceID] = map[string]bool{}
		}
		targets[sourceID][targetID] = true
	}
	rootCache := map[string]string{}
	invalidRoot := map[string]bool{}
	var rootFor func(string, map[string]bool) string
	rootFor = func(entityID string, visiting map[string]bool) string {
		if invalidRoot[entityID] {
			return ""
		}
		if root, ok := rootCache[entityID]; ok {
			return root
		}
		if visiting[entityID] || len(targets[entityID]) > 1 {
			invalidRoot[entityID] = true
			return ""
		}
		visiting[entityID] = true
		root := entityID
		for targetID := range targets[entityID] {
			root = rootFor(targetID, visiting)
		}
		delete(visiting, entityID)
		if root == "" {
			invalidRoot[entityID] = true
			return ""
		}
		rootCache[entityID] = root
		return root
	}

	type surfaceCandidate struct {
		roots   map[string]string
		blocked bool
	}
	candidates := map[string]*surfaceCandidate{}
	addSurface := func(surface, entityID string) {
		key := comparableEntityKey(surface)
		if key == "" {
			return
		}
		candidate := candidates[key]
		if candidate == nil {
			candidate = &surfaceCandidate{roots: map[string]string{}}
			candidates[key] = candidate
		}
		if _, ok := characterIdentities[entityID]; !ok {
			candidate.blocked = true
			return
		}
		rootID := rootFor(entityID, map[string]bool{})
		identity, ok := characterIdentities[rootID]
		label := strings.TrimSpace(identity.CanonicalLabel)
		if !ok || rootID == "" || label == "" {
			candidate.blocked = true
			return
		}
		candidate.roots[rootID] = label
	}
	for id, identity := range characterIdentities {
		addSurface(identity.CanonicalLabel, id)
	}
	for _, surface := range catalog.Surfaces {
		if surface.ChatSessionID == sid {
			addSurface(surface.SurfaceText, strings.TrimSpace(surface.StableEntityID))
		}
	}
	for key, candidate := range candidates {
		if candidate.blocked || len(candidate.roots) != 1 {
			continue
		}
		for _, label := range candidate.roots {
			out[key] = label
		}
	}
	return out
}

func (s *Server) characterReferenceTurn(ctx context.Context, sid string, characters []store.CharacterState) int {
	ref := 0
	if s.Store != nil {
		if logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			for _, log := range logs {
				if log.TurnIndex > ref {
					ref = log.TurnIndex
				}
			}
		}
	}
	for _, ch := range characters {
		if ch.TurnIndex > ref {
			ref = ch.TurnIndex
		}
	}
	return ref
}

func (s *Server) characterRecentMentionSignal(ctx context.Context, sid string, referenceTurn int) (string, map[string]struct{}) {
	if s.Store == nil || sid == "" || referenceTurn <= 0 {
		return "", map[string]struct{}{}
	}
	fromTurn := referenceTurn - 2
	if fromTurn < 0 {
		fromTurn = 0
	}
	logs, err := s.Store.ListChatLogs(ctx, sid, fromTurn, 0)
	if err != nil {
		return "", map[string]struct{}{}
	}
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].TurnIndex != logs[j].TurnIndex {
			return logs[i].TurnIndex > logs[j].TurnIndex
		}
		return logs[i].ID > logs[j].ID
	})
	if len(logs) > 8 {
		logs = logs[:8]
	}
	parts := []string{}
	for _, log := range logs {
		if text := strings.TrimSpace(log.Content); text != "" {
			parts = append(parts, text)
		}
	}
	recentText := strings.Join(parts, " ")
	return recentText, extractCharacterRecentKeywords(recentText)
}

func characterRecentMentionSignalFromLogs(logs []store.ChatLog, referenceTurn int) (string, map[string]struct{}) {
	if referenceTurn <= 0 {
		return "", map[string]struct{}{}
	}
	fromTurn := referenceTurn - 2
	if fromTurn < 0 {
		fromTurn = 0
	}
	filtered := make([]store.ChatLog, 0, len(logs))
	for _, log := range logs {
		if log.TurnIndex < fromTurn {
			continue
		}
		filtered = append(filtered, log)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].TurnIndex != filtered[j].TurnIndex {
			return filtered[i].TurnIndex > filtered[j].TurnIndex
		}
		return filtered[i].ID > filtered[j].ID
	})
	if len(filtered) > 8 {
		filtered = filtered[:8]
	}
	parts := []string{}
	for _, log := range filtered {
		if text := strings.TrimSpace(log.Content); text != "" {
			parts = append(parts, text)
		}
	}
	recentText := strings.Join(parts, " ")
	return recentText, extractCharacterRecentKeywords(recentText)
}

func characterResponseItems(items []store.CharacterState, events []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) []map[string]any {
	latest := latestCharacterStatesByName(items)
	out := []map[string]any{}
	for _, item := range latest {
		recentEvents := recentCharacterEvents(events, item.CharacterName, 8)
		snapshot := characterStaleSnapshot(item, recentEvents, referenceTurn, recentMentionText, recentMentionKeywords)
		if stale, _ := snapshot["is_stale"].(bool); stale {
			continue
		}
		var latestEvent *store.CharacterEvent
		if len(recentEvents) > 0 {
			latestEvent = &recentEvents[0]
		}
		out = append(out, characterResponseItem(item, snapshot, latestEvent, recentEvents))
	}
	return out
}

func characterOmittedCount(items []store.CharacterState, events []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) int {
	omitted := 0
	for _, item := range latestCharacterStatesByName(items) {
		snapshot := characterStaleSnapshot(item, recentCharacterEvents(events, item.CharacterName, 8), referenceTurn, recentMentionText, recentMentionKeywords)
		if stale, _ := snapshot["is_stale"].(bool); stale {
			omitted++
		}
	}
	return omitted
}

func latestCharacterStatesByName(items []store.CharacterState) []store.CharacterState {
	byName := map[string]store.CharacterState{}
	for _, item := range items {
		name := strings.TrimSpace(item.CharacterName)
		if name == "" {
			continue
		}
		current, ok := byName[name]
		if !ok || item.TurnIndex > current.TurnIndex || (item.TurnIndex == current.TurnIndex && item.ID > current.ID) {
			byName[name] = item
		}
	}
	out := make([]store.CharacterState, 0, len(byName))
	for _, item := range byName {
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CharacterName < out[j].CharacterName })
	return out
}

func characterResponseItem(item store.CharacterState, snapshot map[string]any, latestEvent *store.CharacterEvent, recentEvents []store.CharacterEvent) map[string]any {
	relationshipLane := buildCharacterRelationshipLane(item)
	latestAnchor := buildCharacterLatestInteractionAnchor(latestEvent)
	stableSheet := buildStableCharacterSheet(item, snapshot)
	dynamicDigest := buildDynamicCharacterDigest(item, snapshot, relationshipLane, latestAnchor, recentEvents)
	return map[string]any{
		"id":                        item.ID,
		"chat_session_id":           item.ChatSessionID,
		"character_name":            item.CharacterName,
		"appearance_json":           nullableJSONString(item.AppearanceJSON),
		"personality_json":          nullableJSONString(item.PersonalityJSON),
		"status_json":               nullableJSONString(item.StatusJSON),
		"relationships_json":        nullableJSONString(item.RelationshipsJSON),
		"speech_style_json":         nullableJSONString(item.SpeechStyleJSON),
		"turn_index":                item.TurnIndex,
		"last_observed_turn":        snapshot["last_observed_turn"],
		"freshness_turn_gap":        snapshot["freshness_turn_gap"],
		"stale_after_turns":         snapshot["stale_after_turns"],
		"is_stale":                  snapshot["is_stale"],
		"stale_reason":              snapshot["stale_reason"],
		"admission_class":           snapshot["admission_class"],
		"admission_basis":           snapshot["admission_basis"],
		"continuity_anchor_types":   snapshot["continuity_anchor_types"],
		"recent_event_count":        snapshot["recent_event_count"],
		"stale_guard":               snapshot["stale_guard"],
		"stable_character_sheet":    stableSheet,
		"dynamic_continuity_digest": dynamicDigest,
		"relationship_lane":         relationshipLane,
		"latest_interaction_anchor": latestAnchor,
		"created_at":                formatNaiveUTCTime(item.CreatedAt),
		"updated_at":                formatNaiveUTCTime(item.UpdatedAt),
	}
}

func characterStaleSnapshot(item store.CharacterState, recentEvents []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) map[string]any {
	lastObserved := item.TurnIndex
	gapInt := 0
	var gap any
	if referenceTurn > 0 && lastObserved > 0 {
		gapInt = referenceTurn - lastObserved
		if gapInt < 0 {
			gapInt = 0
		}
		gap = gapInt
	}
	anchors := []string{}
	if strings.TrimSpace(item.AppearanceJSON) != "" {
		anchors = append(anchors, "appearance")
	}
	if strings.TrimSpace(item.PersonalityJSON) != "" {
		anchors = append(anchors, "personality")
	}
	if strings.TrimSpace(item.RelationshipsJSON) != "" {
		anchors = append(anchors, "relationships")
	}
	if strings.TrimSpace(item.SpeechStyleJSON) != "" {
		anchors = append(anchors, "speech_style")
	}
	for _, ev := range recentEvents {
		eventType := strings.TrimSpace(ev.EventType)
		if eventType == "relationship_shift" || eventType == "personality_change" {
			anchors = appendUniqueString(anchors, "event_anchor")
			break
		}
	}
	hasAnchor := len(anchors) > 0
	staleAfter := 3
	descriptorLike := looksLikeTransientCharacterName(item.CharacterName)
	recentlyRementioned := descriptorRecentlyRementioned(item.CharacterName, recentMentionText, recentMentionKeywords)
	isStale := descriptorLike && referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor && !recentlyRementioned
	staleReason := any(nil)
	if isStale {
		staleReason = "transient_descriptor_not_rementioned"
	}
	admissionClass := "lightweight_named"
	if hasAnchor || recentlyRementioned || len(recentEvents) >= 2 {
		admissionClass = "major_recurring"
	} else if descriptorLike {
		admissionClass = "transient_descriptor"
	}
	admissionBasis := make([]string, len(anchors))
	copy(admissionBasis, anchors)
	recentEventCount := len(recentEvents)
	if recentEventCount > 3 {
		recentEventCount = 3
	}
	if recentlyRementioned {
		admissionBasis = append(admissionBasis, "recent_remention")
	}
	if len(recentEvents) >= 2 {
		admissionBasis = append(admissionBasis, "recent_event_history")
	}
	return map[string]any{
		"last_observed_turn":      nullablePositiveInt(lastObserved),
		"freshness_turn_gap":      gap,
		"stale_after_turns":       staleAfter,
		"is_stale":                isStale,
		"stale_reason":            staleReason,
		"admission_class":         admissionClass,
		"admission_basis":         admissionBasis,
		"continuity_anchor_types": anchors,
		"recent_event_count":      recentEventCount,
		"stale_guard": map[string]any{
			"active":                         isStale || (referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor),
			"reason":                         staleReasonIfNeeded(staleReason, referenceTurn, gapInt, staleAfter, hasAnchor),
			"allow_weak_input_carry_forward": !isStale && (hasAnchor || recentlyRementioned),
			"admission_class":                admissionClass,
			"admission_basis":                admissionBasis,
		},
	}
}

func nullableJSONString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func staleReasonIfNeeded(staleReason any, referenceTurn, gapInt, staleAfter int, hasAnchor bool) any {
	if staleReason != nil {
		return staleReason
	}
	if referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor {
		return "low_anchor_freshness_gap"
	}
	return nil
}

func descriptorRecentlyRementioned(name string, recentText string, recentKeywords map[string]struct{}) bool {
	rawName := normalizeCharacterDescriptorText(name)
	rawRecentText := normalizeCharacterDescriptorText(recentText)
	if rawName != "" && strings.Contains(rawRecentText, rawName) {
		return true
	}
	nameKeywords := extractCharacterDescriptorKeywords(name)
	genericHit := false
	for token := range characterGenericDescriptorTokens() {
		if _, ok := recentKeywords[token]; ok {
			genericHit = true
			break
		}
		if rawRecentText != "" && strings.Contains(rawRecentText, token) {
			genericHit = true
			break
		}
	}
	if !genericHit {
		return false
	}
	nonGeneric := []string{}
	generic := characterGenericDescriptorTokens()
	for _, token := range nameKeywords {
		if _, ok := generic[token]; !ok {
			nonGeneric = append(nonGeneric, token)
		}
	}
	if len(nonGeneric) == 0 {
		return true
	}
	overlap := 0
	for _, token := range nonGeneric {
		if _, ok := recentKeywords[token]; ok {
			overlap++
		}
	}
	required := 1
	if len(nonGeneric) > 1 {
		required = 2
		if len(nonGeneric) < required {
			required = len(nonGeneric)
		}
	}
	return overlap >= required
}

func extractCharacterRecentKeywords(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range splitCharacterDescriptorTokens(text) {
		if len([]rune(token)) >= characterKeywordMinLength(token) {
			out[token] = struct{}{}
		}
	}
	return out
}

func extractCharacterDescriptorKeywords(name string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, token := range splitCharacterDescriptorTokens(name) {
		if token == "" || characterDescriptorStopwords()[token] {
			continue
		}
		if len([]rune(token)) < characterKeywordMinLength(token) {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func splitCharacterDescriptorTokens(text string) []string {
	return strings.FieldsFunc(strings.ToLower(strings.TrimSpace(text)), func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', '.', ',', '!', '?', ':', ';', '(', ')', '[', ']', '{', '}', '/', '|', '\\', '"', '\'', '`', '~', '@', '#', '$', '%', '^', '&', '*', '+', '=', '<', '>', '-', '_':
			return true
		default:
			return false
		}
	})
}

func normalizeCharacterDescriptorText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}

func characterKeywordMinLength(token string) int {
	for _, r := range token {
		if (r >= '\uAC00' && r <= '\uD7A3') || (r >= '\u1100' && r <= '\u11FF') {
			return 2
		}
	}
	return 3
}

func characterGenericDescriptorTokens() map[string]struct{} {
	return map[string]struct{}{
		"woman": {}, "man": {}, "girl": {}, "boy": {}, "lady": {}, "gentleman": {}, "stranger": {}, "figure": {}, "person": {}, "voice": {},
	}
}

func characterDescriptorStopwords() map[string]bool {
	return map[string]bool{"a": true, "an": true, "the": true, "this": true, "that": true, "these": true, "those": true, "in": true, "on": true, "at": true, "of": true}
}

func looksLikeTransientCharacterName(name string) bool {
	text := strings.TrimSpace(name)
	if text == "" {
		return true
	}
	lower := strings.ToLower(text)
	transientTokens := []string{"unknown", "unnamed", "npc", "woman", "man", "girl", "boy", "person", "voice", "figure", "descriptor"}
	for _, token := range transientTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return strings.Count(text, " ") >= 2
}

func recentCharacterEvents(events []store.CharacterEvent, characterName string, limit int) []store.CharacterEvent {
	filtered := []store.CharacterEvent{}
	for _, ev := range events {
		if ev.CharacterName == characterName {
			filtered = append(filtered, ev)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].TurnIndex != filtered[j].TurnIndex {
			return filtered[i].TurnIndex > filtered[j].TurnIndex
		}
		if !filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		}
		return filtered[i].ID > filtered[j].ID
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func parseSurfacePayload(raw string) any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		return parsed
	}
	return text
}

func hasSurfaceValue(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			return rv.Len() > 0
		}
		return true
	}
}

func surfaceStatus(payloads map[string]any) string {
	filled := 0
	for _, value := range payloads {
		if hasSurfaceValue(value) {
			filled++
		}
	}
	if filled == 0 {
		return "empty"
	}
	if filled == len(payloads) {
		return "ready"
	}
	return "partial"
}

func filledAxes(payloads map[string]any) []string {
	out := []string{}
	for _, key := range sortedMapKeys(payloads) {
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func filledAxesInOrder(payloads map[string]any, order ...string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, key := range order {
		seen[key] = true
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	for _, key := range sortedMapKeys(payloads) {
		if seen[key] {
			continue
		}
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatNaiveUTCTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

func cleanShadowText(value any, maxLen int) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return truncateRunes(strings.Join(strings.Fields(v), " "), maxLen)
	case float64, bool, int, int64:
		return truncateRunes(strings.TrimSpace(compactJSONForShadow(v, maxLen)), maxLen)
	case map[string]any, []any:
		return truncateRunes(strings.TrimSpace(compactJSONForShadow(v, maxLen)), maxLen)
	default:
		return truncateRunes(strings.Join(strings.Fields(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(compactJSONForShadow(v, maxLen)), "\n", " "), "\t", " "))), " "), maxLen)
	}
}

func preferredSummaryText(payload any, maxLen int, keys ...string) string {
	if m, ok := payload.(map[string]any); ok {
		parts := []string{}
		rawTextKeys := map[string]bool{"summary": true, "summary_text": true, "detail": true, "note": true, "message": true, "interaction": true}
		for _, key := range keys {
			text := cleanShadowText(m[key], 90)
			if text == "" {
				continue
			}
			if rawTextKeys[key] {
				parts = append(parts, text)
			} else {
				parts = append(parts, key+": "+text)
			}
			if len(parts) >= 3 {
				break
			}
		}
		if len(parts) > 0 {
			return truncateRunes(strings.Join(parts, "; "), maxLen)
		}
	}
	return cleanShadowText(payload, maxLen)
}

func buildCharacterRelationshipLane(item store.CharacterState) map[string]any {
	payload := parseSurfacePayload(item.RelationshipsJSON)
	protagonistItems := []map[string]any{}
	otherItems := []map[string]any{}
	seen := map[string]bool{}
	var protagonist map[string]any
	appendItem := func(targetRaw any, relation any) {
		target := relationDisplayTarget(targetRaw)
		if target == "" {
			return
		}
		summary := preferredSummaryText(relation, 180, "summary", "summary_text", "status", "state", "detail", "note", "trust", "closeness", "tension")
		if summary == "" {
			return
		}
		key := strings.ToLower(target + "\x00" + summary)
		if seen[key] {
			return
		}
		seen[key] = true
		isPlayer := isPlayerReference(targetRaw)
		relationMap := map[string]any{"value": relation}
		if m, ok := relation.(map[string]any); ok {
			relationMap = m
		}
		entry := map[string]any{
			"target":           target,
			"summary_text":     summary,
			"state_snapshot":   projectRelationPayload(relationMap),
			"descriptor_bands": relationDescriptorBands(relationMap),
			"display_priority": 1,
		}
		if isPlayer {
			entry["display_priority"] = 0
			protagonistItems = append(protagonistItems, entry)
			if protagonist == nil {
				protagonist = entry
			}
			return
		}
		otherItems = append(otherItems, entry)
	}
	switch v := payload.(type) {
	case []any:
		for _, raw := range v {
			if m, ok := raw.(map[string]any); ok {
				appendItem(firstPresentValue(m, "target", "name", "character_name", "title", "scope_name"), m)
			}
		}
	case map[string]any:
		for _, key := range sortedMapKeys(v) {
			value := v[key]
			target := any(key)
			if m, ok := value.(map[string]any); ok {
				if tv := firstPresentValue(m, "target", "name", "character_name"); tv != nil {
					target = tv
				}
			}
			appendItem(target, value)
		}
	}
	ordered := append([]map[string]any{}, protagonistItems...)
	ordered = append(ordered, otherItems...)
	if len(ordered) > 6 {
		ordered = ordered[:6]
	}
	preferred := protagonist
	if preferred == nil && len(ordered) > 0 {
		preferred = ordered[0]
	}
	secondary := []map[string]any{}
	for _, entry := range ordered {
		if preferred != nil && entry["target"] == preferred["target"] && entry["summary_text"] == preferred["summary_text"] {
			continue
		}
		secondary = append(secondary, entry)
	}
	summary := ""
	if preferred != nil {
		summary, _ = preferred["summary_text"].(string)
	}
	if summary == "" {
		summary = preferredSummaryText(payload, 180, "summary", "summary_text", "status", "state", "detail", "note", "trust", "closeness", "tension")
	}
	status := "empty"
	if len(ordered) > 0 {
		status = "ready"
	} else if summary != "" {
		status = "summary_only"
	}
	descriptorSummary := ""
	if preferred != nil {
		if bands, ok := preferred["descriptor_bands"].([]string); ok {
			descriptorSummary = truncateRunes(strings.Join(bands, "; "), 180)
		}
	}
	return map[string]any{
		"surface_version":          "rl14a.v1",
		"surface_type":             "relationship_lane",
		"status":                   status,
		"display_mode":             "protagonist_first_then_observed_order",
		"count":                    len(protagonistItems) + len(otherItems),
		"summary_text":             nullableString(summary),
		"descriptor_summary":       descriptorSummary,
		"primary_target":           mapStringOrNil(preferred, "target"),
		"primary_descriptor_bands": mapAnyOrEmptyStringSlice(preferred, "descriptor_bands"),
		"protagonist_relation":     protagonist,
		"other_relations":          limitMapSlice(secondary, 5),
		"items":                    mapSliceToAny(ordered),
	}
}

func buildCharacterLatestInteractionAnchor(event *store.CharacterEvent) any {
	if event == nil {
		return nil
	}
	details := parseSurfacePayload(event.DetailsJSON)
	summary := preferredSummaryText(details, 180, "interaction", "detail", "summary", "summary_text", "note", "message", "status", "change")
	if summary == "" {
		summary = cleanShadowText(event.EventType, 80)
	}
	return map[string]any{
		"surface_version": "rl14b.v1",
		"surface_type":    "latest_interaction_anchor",
		"status":          "ready",
		"event_type":      event.EventType,
		"turn_index":      event.TurnIndex,
		"summary_text":    summary,
		"details":         details,
		"created_at":      formatNaiveUTCTime(event.CreatedAt),
	}
}

func buildStableCharacterSheet(item store.CharacterState, snapshot map[string]any) map[string]any {
	appearance := parseSurfacePayload(item.AppearanceJSON)
	personality := parseSurfacePayload(item.PersonalityJSON)
	speechStyle := parseSurfacePayload(item.SpeechStyleJSON)
	appearanceCore, appearanceSnapshot := splitAppearancePayload(appearance)
	appearanceObservable, appearanceNonObservable := splitObservableAppearancePayload(appearanceCore)
	axes := map[string]any{"appearance": appearanceCore, "personality": personality, "speech_style": speechStyle}
	return map[string]any{
		"surface_version":           "cc14a.v1",
		"surface_type":              "stable_character_sheet",
		"status":                    surfaceStatus(axes),
		"filled_axes":               filledAxes(axes),
		"appearance":                appearance,
		"appearance_core":           appearanceCore,
		"appearance_observable":     appearanceObservable,
		"appearance_non_observable": appearanceNonObservable,
		"appearance_snapshot_keys":  sortedMapKeys(appearanceSnapshot),
		"personality":               personality,
		"speech_style":              speechStyle,
		"durable_profile": map[string]any{
			"appearance":   appearanceObservable,
			"personality":  personality,
			"speech_style": speechStyle,
		},
		"sparse_policy": map[string]any{
			"mode":              "omit_unknown_fields",
			"filled_axes":       filledAxes(axes),
			"empty_axes":        emptyAxes(axes),
			"dynamic_redirects": []string{"current_status", "relationship_lane", "latest_interaction_anchor", "appearance_snapshot"},
		},
		"source_turn": snapshot["last_observed_turn"],
	}
}

func buildDynamicCharacterDigest(item store.CharacterState, snapshot map[string]any, relationshipLane map[string]any, latestAnchor any, recentEvents []store.CharacterEvent) map[string]any {
	currentStatus := parseSurfacePayload(item.StatusJSON)
	appearance := parseSurfacePayload(item.AppearanceJSON)
	_, appearanceSnapshot := splitAppearancePayload(appearance)
	currentStatusSummary := ""
	if m, ok := currentStatus.(map[string]any); ok {
		currentStatusSummary = preferredSummaryText(m, 180, "location", "emotion", "goal", "status", "state", "condition", "mood")
	}
	relationshipItems := mapAnySlice(relationshipLane["items"])
	relationshipDescriptorLane := []map[string]any{}
	for _, entry := range relationshipItems[:minInt(len(relationshipItems), 4)] {
		relationshipDescriptorLane = append(relationshipDescriptorLane, map[string]any{
			"target":           entry["target"],
			"summary_text":     entry["summary_text"],
			"descriptor_bands": mapAnyOrEmptyStringSlice(entry, "descriptor_bands"),
		})
	}
	var preferredRelation map[string]any
	if pr, ok := relationshipLane["protagonist_relation"].(map[string]any); ok && pr != nil {
		preferredRelation = pr
	} else if len(relationshipItems) > 0 {
		preferredRelation = relationshipItems[0]
	}
	var relationshipFocus map[string]any
	if preferredRelation != nil {
		relationshipFocus = compactMap(map[string]any{
			"target":           mapStringOrNil(preferredRelation, "target"),
			"summary_text":     mapStringOrNil(preferredRelation, "summary_text"),
			"descriptor_bands": mapAnyOrEmptyStringSlice(preferredRelation, "descriptor_bands"),
		})
	}
	var relationshipSurface any
	if preferredRelation != nil {
		relationshipSurface = preferredRelation
	} else if len(relationshipItems) > 0 {
		relationshipSurface = relationshipItems
	} else if st, ok := relationshipLane["summary_text"].(string); ok && strings.TrimSpace(st) != "" {
		relationshipSurface = st
	}
	axes := map[string]any{"current_status": currentStatus, "relationship_surface": relationshipSurface, "latest_interaction_anchor": latestAnchor}
	return map[string]any{
		"surface_version":              "cc14b.v1",
		"surface_type":                 "dynamic_continuity_digest",
		"status":                       surfaceStatus(axes),
		"filled_axes":                  filledAxesInOrder(axes, "current_status", "relationship_surface", "latest_interaction_anchor"),
		"admission_class":              snapshot["admission_class"],
		"admission_basis":              snapshot["admission_basis"],
		"stale_guard":                  snapshot["stale_guard"],
		"current_status":               currentStatus,
		"current_status_summary":       nullableString(currentStatusSummary),
		"current_snapshot":             compactMap(map[string]any{"status": currentStatus, "appearance": appearanceSnapshot, "relationship_focus": relationshipFocus}),
		"appearance_snapshot":          appearanceSnapshot,
		"relationship_summary_text":    relationshipLane["summary_text"],
		"relationship_primary_target":  relationshipLane["primary_target"],
		"relationship_display_mode":    relationshipLane["display_mode"],
		"protagonist_relation":         relationshipLane["protagonist_relation"],
		"relationship_lane":            relationshipLane["items"],
		"other_relations":              relationshipLane["other_relations"],
		"relationship_descriptor_lane": mapSliceToAny(relationshipDescriptorLane),
		"latest_interaction_anchor":    latestAnchor,
		"milestone_ledger":             characterMilestoneLedger(recentEvents),
		"digest_budget": map[string]any{
			"policy":                     "priority_capped",
			"relationship_lane_cap":      4,
			"milestone_cap":              3,
			"milestone_read_window":      8,
			"milestone_selection_policy": "latest_plus_priority_events",
			"relationship_lane_used":     len(relationshipDescriptorLane),
			"milestones_used":            len(characterMilestoneLedger(recentEvents)),
		},
		"recent_change_summary": recentChangeSummary(latestAnchor),
		"source_turn":           snapshot["last_observed_turn"],
	}
}

func firstPresentValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && hasSurfaceValue(v) {
			return v
		}
	}
	return nil
}

func isPlayerReference(value any) bool {
	text := strings.ToLower(cleanShadowText(value, 60))
	switch text {
	case "__player__", "{{user}}", "user", "player", "participant":
		return true
	default:
		return false
	}
}

func relationDisplayTarget(value any) string {
	if isPlayerReference(value) {
		return "{{user}}"
	}
	return cleanShadowText(value, 60)
}

func projectRelationPayload(payload any) any {
	switch v := payload.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range v {
			if key == "target" || key == "name" || key == "character_name" || key == "owner" || key == "subject" || key == "object" || key == "from" || key == "to" {
				if display := relationDisplayTarget(value); display != "" {
					out[key] = display
					continue
				}
			}
			out[key] = projectRelationPayload(value)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, projectRelationPayload(item))
		}
		return out
	case string:
		if isPlayerReference(v) {
			return "{{user}}"
		}
		return v
	default:
		return v
	}
}

func relationDescriptorBands(payload map[string]any) []string {
	keys := []string{"trust", "closeness", "tension", "bond", "distance", "stance"}
	out := []string{}
	for _, key := range keys {
		if text := cleanShadowText(payload[key], 60); text != "" {
			out = append(out, key+": "+text)
		}
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func splitAppearancePayload(payload any) (any, map[string]any) {
	m, ok := payload.(map[string]any)
	if !ok {
		return payload, map[string]any{}
	}
	durable := map[string]any{}
	snapshot := map[string]any{}
	snapshotTokens := []string{
		"outfit", "clothes", "clothing", "uniform", "coat", "jacket", "dress", "armor", "accessory",
		"expression", "posture", "condition", "injury", "blood", "mud", "wet",
	}
	for key, value := range m {
		normalized := strings.ToLower(strings.ReplaceAll(key, " ", ""))
		isSnapshot := false
		for _, token := range snapshotTokens {
			if strings.Contains(normalized, token) {
				isSnapshot = true
				break
			}
		}
		if isSnapshot {
			snapshot[key] = value
			continue
		}
		durable[key] = value
	}
	if len(durable) == 0 {
		return payload, snapshot
	}
	return durable, snapshot
}

func splitObservableAppearancePayload(payload any) (any, map[string]any) {
	m, ok := payload.(map[string]any)
	if !ok {
		return payload, map[string]any{}
	}
	observable := map[string]any{}
	nonObservable := map[string]any{}
	for key, value := range m {
		lower := strings.ToLower(strings.ReplaceAll(key, " ", ""))
		if strings.Contains(lower, "thought") || strings.Contains(lower, "emotion") || strings.Contains(lower, "feeling") || strings.Contains(lower, "internal") {
			nonObservable[key] = value
			continue
		}
		observable[key] = value
	}
	return observable, nonObservable
}

func emptyAxes(payloads map[string]any) []string {
	out := []string{}
	for _, key := range sortedMapKeys(payloads) {
		if !hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func compactMap(payload map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range payload {
		if hasSurfaceValue(value) {
			out[key] = value
		}
	}
	return out
}

func characterMilestoneLedger(events []store.CharacterEvent) []any {
	candidates := []map[string]any{}
	for recencyIndex, ev := range events {
		details := parseSurfacePayload(ev.DetailsJSON)
		summary := preferredSummaryText(details, 180, "interaction", "detail", "summary", "summary_text", "note", "message", "status", "change")
		if summary == "" {
			summary = cleanShadowText(ev.EventType, 80)
		}
		if summary == "" {
			continue
		}
		priority := characterEventPriority(ev.EventType)
		candidates = append(candidates, map[string]any{
			"event_type":      ev.EventType,
			"turn_index":      ev.TurnIndex,
			"summary_text":    summary,
			"details":         details,
			"created_at":      formatNaiveUTCTime(ev.CreatedAt),
			"_event_priority": priority,
			"_recency_index":  recencyIndex,
		})
	}
	if len(candidates) == 0 {
		return []any{}
	}

	selected := []map[string]any{}
	seen := map[string]bool{}
	appendCandidate := func(candidate map[string]any) {
		key := fmt.Sprintf("%v|%v|%v", candidate["event_type"], candidate["turn_index"], candidate["summary_text"])
		if seen[key] {
			return
		}
		seen[key] = true
		selected = append(selected, candidate)
	}

	appendCandidate(candidates[0])
	if len(candidates) > 1 {
		remaining := make([]map[string]any, len(candidates[1:]))
		copy(remaining, candidates[1:])
		sort.SliceStable(remaining, func(i, j int) bool {
			pi, _ := remaining[i]["_event_priority"].(int)
			pj, _ := remaining[j]["_event_priority"].(int)
			ri, _ := remaining[i]["_recency_index"].(int)
			rj, _ := remaining[j]["_recency_index"].(int)
			if pi != pj {
				return pi < pj
			}
			return ri < rj
		})
		for _, candidate := range remaining {
			if len(selected) >= 3 {
				break
			}
			appendCandidate(candidate)
		}
	}
	if len(selected) < 3 {
		for _, candidate := range candidates[1:] {
			if len(selected) >= 3 {
				break
			}
			appendCandidate(candidate)
		}
	}

	sort.SliceStable(selected, func(i, j int) bool {
		ri, _ := selected[i]["_recency_index"].(int)
		rj, _ := selected[j]["_recency_index"].(int)
		return ri < rj
	})

	out := []any{}
	for _, candidate := range selected[:minInt(len(selected), 3)] {
		cleaned := map[string]any{}
		for k, v := range candidate {
			if !strings.HasPrefix(k, "_") {
				cleaned[k] = v
			}
		}
		out = append(out, cleaned)
	}
	return out
}

func recentChangeSummary(anchor any) any {
	if m, ok := anchor.(map[string]any); ok {
		return m["summary_text"]
	}
	return nil
}

func mapStringOrNil(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return nil
}

func mapAnyOrEmptyStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return []string{}
	}
	if v, ok := m[key].([]string); ok {
		return v
	}
	return []string{}
}

func limitMapSlice(items []map[string]any, limit int) []any {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return mapSliceToAny(items)
}

func mapSliceToAny(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func mapAnySlice(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		return []map[string]any{}
	}
	out := []map[string]any{}
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (s *Server) handleCharacterDetail(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	item, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			item = nil
		} else if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"status": "error",
				"detail": fmt.Sprintf("character not found: %s", cname),
			})
			return
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"found":           item != nil,
		"character":       item,
	})
}

func (s *Server) handleCharacterEvents(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	limit := 30
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	items, err := s.Store.ListCharacterEvents(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	total := len(items)
	start := offset
	if start > len(items) {
		start = len(items)
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := items[start:end]
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"events":          page,
		"total":           total,
		"limit":           limit,
		"offset":          offset,
	})
}

func (s *Server) handleCharacterStateHistory(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	historyStore, ok := s.Store.(store.CharacterStateHistoryStore)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "ok",
			"chat_session_id": sid,
			"character_name":  cname,
			"state_history":   []store.CharacterState{},
			"count":           0,
			"limit":           limit,
			"offset":          offset,
			"mode":            "history_store_not_available",
		})
		return
	}
	items, err := historyStore.ListCharacterStateHistory(r.Context(), sid, cname, limit, offset)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.CharacterState{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"state_history":   items,
		"count":           len(items),
		"limit":           limit,
		"offset":          offset,
		"mode":            "append_only_snapshots_latest_first",
	})
}

func (s *Server) handleCharacterPatch(w http.ResponseWriter, r *http.Request) {
	s.handleCharacterStatePatch(w, r, false)
}

func (s *Server) handleCharacterSpeech(w http.ResponseWriter, r *http.Request) {
	s.handleCharacterStatePatch(w, r, true)
}

func (s *Server) handleCharacterDelete(w http.ResponseWriter, r *http.Request) {
	endpoint := "DELETE /characters/{chat_session_id}/{character_name}"
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	cname := strings.TrimSpace(r.PathValue("character_name"))
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	current, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("character not found: %s", cname))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	events, _ := s.Store.ListCharacterEvents(r.Context(), sid, cname)
	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteCharacterByName(r.Context(), sid, cname); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_delete",
		TargetType:    "character",
		TargetID:      current.ID,
		Summary:       "Explorer manual character delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"character_name": cname,
			"previous": map[string]any{
				"turn_index":         current.TurnIndex,
				"appearance_json":    current.AppearanceJSON,
				"personality_json":   current.PersonalityJSON,
				"status_json":        current.StatusJSON,
				"relationships_json": current.RelationshipsJSON,
				"speech_style_json":  current.SpeechStyleJSON,
				"created_at":         current.CreatedAt,
				"updated_at":         current.UpdatedAt,
			},
			"character_events_deleted": len(events),
			"changed_at":               changedAt,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"source":                   s.storeWriteSource(),
		"mutation_enabled":         true,
		"chat_session_id":          sid,
		"target_type":              "character",
		"target_id":                current.ID,
		"character_name":           cname,
		"deleted":                  true,
		"character_events_deleted": len(events),
		"changed_at":               changedAt,
		"audit_written":            true,
	})
}

func (s *Server) handleCharacterStatePatch(w http.ResponseWriter, r *http.Request, speechOnly bool) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	cname := strings.TrimSpace(r.PathValue("character_name"))
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	saver, ok := s.Store.(characterStateSaver)
	if !ok {
		writeShadowGuard(w, r.Method+" "+r.URL.Path)
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeCharacterPatchPayload(payload, speechOnly)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if len(updates) == 0 {
		writeBadRequest(w, "no supported character fields to update")
		return
	}
	current, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("character not found: %s", cname))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, r.Method+" "+r.URL.Path)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	now := time.Now().UTC()
	next := *current
	if speechOnly {
		updates = preserveTypedVoiceProjectionManualOverrides(current.SpeechStyleJSON, updates)
	}
	next.ChatSessionID = sid
	next.CharacterName = cname
	next.UpdatedAt = now
	if next.CreatedAt.IsZero() {
		next.CreatedAt = now
	}
	changed := make([]string, 0, len(updates))
	for _, key := range []string{"appearance_json", "personality_json", "status_json", "relationships_json", "speech_style_json", "turn_index"} {
		val, exists := updates[key]
		if !exists {
			continue
		}
		changed = append(changed, key)
		switch key {
		case "appearance_json":
			next.AppearanceJSON = stringFromAnyNullable(val)
		case "personality_json":
			next.PersonalityJSON = stringFromAnyNullable(val)
		case "status_json":
			next.StatusJSON = stringFromAnyNullable(val)
		case "relationships_json":
			next.RelationshipsJSON = stringFromAnyNullable(val)
		case "speech_style_json":
			next.SpeechStyleJSON = stringFromAnyNullable(val)
		case "turn_index":
			if i, ok := val.(int); ok {
				next.TurnIndex = i
			}
		}
	}
	if err := saver.SaveCharacterState(r.Context(), &next); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, r.Method+" "+r.URL.Path)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	eventType := "manual_patch"
	if speechOnly {
		eventType = "speech_style_patch"
	}
	_ = s.Store.SaveCharacterEvent(r.Context(), &store.CharacterEvent{
		ChatSessionID: sid,
		CharacterName: cname,
		TurnIndex:     next.TurnIndex,
		EventType:     eventType,
		DetailsJSON:   mustCompactJSON(map[string]any{"updated_fields": changed, "source": "manual_patch"}),
		CreatedAt:     now,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"updated_fields":  changed,
		"character":       characterResponseItem(next, characterStaleSnapshot(next, nil, next.TurnIndex, "", nil), nil, nil),
	})
}

func preserveTypedVoiceProjectionManualOverrides(currentRaw string, updates map[string]any) map[string]any {
	current := map[string]any{}
	if json.Unmarshal([]byte(strings.TrimSpace(currentRaw)), &current) != nil ||
		extractionStringFromAny(current["contract_version"]) != voiceBehaviorProjectionContractVersion {
		return updates
	}
	nextRaw, exists := updates["speech_style_json"]
	if !exists {
		return updates
	}
	manual := map[string]any{}
	switch typed := nextRaw.(type) {
	case string:
		_ = json.Unmarshal([]byte(strings.TrimSpace(typed)), &manual)
	case map[string]any:
		manual = typed
	}
	allowed := map[string]any{}
	for _, key := range []string{"default_tone", "honorific_style", "speech_notes"} {
		if value := strings.TrimSpace(extractionStringFromAny(manual[key])); value != "" {
			allowed[key] = value
		}
	}
	current["manual_overrides"] = allowed
	copyUpdates := map[string]any{}
	for key, value := range updates {
		copyUpdates[key] = value
	}
	copyUpdates["speech_style_json"] = mustCompactJSON(current)
	return copyUpdates
}

func normalizeCharacterPatchPayload(payload map[string]any, speechOnly bool) (map[string]any, error) {
	updates := map[string]any{}
	fieldMap := map[string]string{
		"appearance_json":    "appearance_json",
		"appearance":         "appearance_json",
		"personality_json":   "personality_json",
		"personality":        "personality_json",
		"status_json":        "status_json",
		"status":             "status_json",
		"relationships_json": "relationships_json",
		"relationships":      "relationships_json",
		"speech_style_json":  "speech_style_json",
		"speech_style":       "speech_style_json",
	}
	if speechOnly {
		fieldMap = map[string]string{"speech_style_json": "speech_style_json", "speech_style": "speech_style_json"}
	}
	for rawKey, targetKey := range fieldMap {
		val, exists := payload[rawKey]
		if !exists {
			continue
		}
		normalized, err := normalizeStorylineJSONPatchValue(targetKey, val)
		if err != nil {
			return nil, err
		}
		updates[targetKey] = normalized
	}
	if !speechOnly {
		if val, exists := payload["turn_index"]; exists {
			i, ok := storylineIntPatchValue(val)
			if !ok || i < 0 {
				return nil, fmt.Errorf("turn_index must be a non-negative integer")
			}
			updates["turn_index"] = i
		}
	}
	return updates, nil
}

func stringFromAnyNullable(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
