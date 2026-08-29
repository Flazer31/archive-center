package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const itemIdentityManualMergeContract = "item_identity_manual_merge.v1"

func (s *Server) handleItemsGet(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	catalog, err := s.entityIdentityCatalogForSession(r.Context(), sid, "item")
	if err != nil {
		if !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		catalog = characterIdentityCatalog{Identities: map[string]store.EntityIdentity{}, Surfaces: []store.EntityIdentitySurface{}, Links: []store.EntityIdentityLink{}}
	}

	historyScope := explorerHistoryScope(r.Context(), s.Store, sid, 0, 0)
	triples, err := listExplorerHistoryKGTriples(r.Context(), s.Store, historyScope.Segments)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		writeInternalError(w, err.Error())
		return
	}
	sortKGTriplesForPython(triples)
	// Preserve the previous explorer read envelope: it inspected the newest
	// 200 KG rows and rendered at most 40 item cards.
	if len(triples) > 200 {
		triples = triples[:200]
	}
	aliases := s.itemIdentityAliases(r.Context(), sid, catalog)
	items := []map[string]any{}
	seen := map[string]bool{}
	for _, triple := range triples {
		if !itemIdentityPredicate(triple.Predicate) {
			continue
		}
		itemName := strings.TrimSpace(triple.Object)
		stableID := ""
		if triple.ChatSessionID == sid {
			itemName, stableID = s.canonicalEntitySurfaceOfKind(r.Context(), sid, itemName, "item")
		}
		key := comparableEntityKey(itemName)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		row := explorerHistoryItem(map[string]any{
			"id": triple.ID, "item": itemName,
			"owner":     s.canonicalCharacterName(r.Context(), triple.ChatSessionID, triple.Subject),
			"predicate": triple.Predicate, "source_turn": nullablePositiveInt(triple.SourceTurn),
		}, sid, triple.ChatSessionID)
		if stableID != "" {
			row["stable_entity_id"] = stableID
			row["aliases"] = nonNilSlice(aliases[stableID])
		}
		items = append(items, row)
	}

	identityIDs := make([]string, 0, len(catalog.Identities))
	for id := range catalog.Identities {
		identityIDs = append(identityIDs, id)
	}
	sort.Strings(identityIDs)
	for _, id := range identityIDs {
		rootID := s.characterIdentityRoot(r.Context(), sid, id)
		identity, exists := catalog.Identities[rootID]
		if !exists {
			continue
		}
		key := comparableEntityKey(identity.CanonicalLabel)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, map[string]any{
			"item": identity.CanonicalLabel, "stable_entity_id": rootID,
			"aliases": nonNilSlice(aliases[rootID]), "mutation_allowed": true,
			"history_ownership": "current_branch", "source_session_id": sid,
		})
	}
	total := len(items)
	if len(items) > 40 {
		items = items[:40]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "chat_session_id": sid, "items": items,
		"identity_links": itemIdentityLinkItems(catalog), "count": len(items), "total": total,
		"omitted_count": total - len(items),
		"history_scope": prepareTurnHistoryScopeTrace(historyScope),
	})
}

func (s *Server) handleItemIdentityMergePreview(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	req, ok := decodeCharacterIdentityMergeRequest(w, r, sid)
	if !ok {
		return
	}
	catalog, selected, target, sourceSelections, err := s.entityIdentityMergeSelection(r.Context(), sid, req, "item")
	if err != nil {
		writeError(w, http.StatusBadRequest, "item_identity_merge_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "contract_version": itemIdentityManualMergeContract,
		"chat_session_id": sid, "target": target,
		"sources":          characterIdentitySelectionItems(req.SourceEntityIDs, catalog.Identities),
		"source_results":   characterIdentitySourceSelectionItems(sourceSelections),
		"impacts":          s.itemIdentityMergeImpacts(r.Context(), sid, selected, catalog),
		"writes_performed": false,
	})
}

func (s *Server) handleItemIdentityMerge(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	req, ok := decodeCharacterIdentityMergeRequest(w, r, sid)
	if !ok {
		return
	}
	catalog, _, target, sourceSelections, err := s.entityIdentityMergeSelection(r.Context(), sid, req, "item")
	if err != nil {
		writeError(w, http.StatusBadRequest, "item_identity_merge_invalid", err.Error())
		return
	}
	writer, ok := s.Store.(store.EntityIdentityLinkWriter)
	if !ok {
		writeError(w, http.StatusConflict, "item_identity_merge_unavailable", "entity identity links are not writable")
		return
	}
	targetID := strings.TrimSpace(target.StableEntityID)
	results := make([]map[string]any, 0, len(sourceSelections))
	succeeded := 0
	for _, selection := range sourceSelections {
		if selection.Status != "ready" {
			results = append(results, map[string]any{"source_entity_id": selection.RequestedID, "status": "failed", "detail": selection.Detail})
			continue
		}
		if selection.RootID == targetID {
			results = append(results, map[string]any{"source_entity_id": selection.RequestedID, "status": "already_merged", "target_entity_id": targetID})
			continue
		}
		link := itemIdentityManualLink(sid, selection.RootID, targetID, store.EntityIdentityLinkStateReviewed)
		if err := writer.SaveEntityIdentityLink(r.Context(), &link); err != nil {
			results = append(results, map[string]any{"source_entity_id": selection.RequestedID, "status": "failed", "detail": err.Error()})
			continue
		}
		succeeded++
		results = append(results, map[string]any{"source_entity_id": selection.RequestedID, "status": "linked", "target_entity_id": targetID, "link_id": link.LinkID})
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid, EventType: "item_identity_manual_merge", TargetType: "entity_identity",
		Summary:     fmt.Sprintf("linked %d item identities to %s", succeeded, target.CanonicalLabel),
		DetailsJSON: mustCompactJSON(map[string]any{"target_entity_id": targetID, "results": results}), Source: s.storeWriteSource(),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "contract_version": itemIdentityManualMergeContract,
		"chat_session_id": sid, "target": target, "results": results, "linked_count": succeeded,
		"stored_rows_rewritten": 0, "critic_calls": 0, "vector_reindex_queued": false,
		"catalog_identity_count": len(catalog.Identities),
	})
}

func (s *Server) handleItemIdentityUnmerge(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	req, ok := decodeCharacterIdentityMergeRequest(w, r, sid)
	if !ok {
		return
	}
	catalog, err := s.entityIdentityCatalogForSession(r.Context(), sid, "item")
	if err != nil {
		writeError(w, http.StatusBadRequest, "item_identity_unmerge_invalid", err.Error())
		return
	}
	target, exists := catalog.Identities[req.TargetEntityID]
	if !exists {
		writeError(w, http.StatusBadRequest, "item_identity_unmerge_invalid", "target entity is not an active item in this session")
		return
	}
	writer, ok := s.Store.(store.EntityIdentityLinkWriter)
	if !ok {
		writeError(w, http.StatusConflict, "item_identity_unmerge_unavailable", "entity identity links are not writable")
		return
	}
	active := map[string]bool{}
	for _, link := range catalog.Links {
		if _, sourceOK := catalog.Identities[link.SourceEntityID]; !sourceOK {
			continue
		}
		if _, targetOK := catalog.Identities[link.TargetEntityID]; targetOK {
			active[link.SourceEntityID+"\x1f"+link.TargetEntityID] = true
		}
	}
	results := make([]map[string]any, 0, len(req.SourceEntityIDs))
	revoked := 0
	for _, sourceID := range uniqueNonEmptyStrings(req.SourceEntityIDs) {
		if _, exists := catalog.Identities[sourceID]; !exists {
			results = append(results, map[string]any{"source_entity_id": sourceID, "status": "failed", "detail": "source entity is not an active item in this session"})
			continue
		}
		if !active[sourceID+"\x1f"+target.StableEntityID] {
			results = append(results, map[string]any{"source_entity_id": sourceID, "status": "not_linked", "target_entity_id": target.StableEntityID})
			continue
		}
		link := itemIdentityManualLink(sid, sourceID, target.StableEntityID, store.EntityIdentityLinkStateRevoked)
		if err := writer.SaveEntityIdentityLink(r.Context(), &link); err != nil {
			results = append(results, map[string]any{"source_entity_id": sourceID, "status": "failed", "detail": err.Error()})
			continue
		}
		revoked++
		results = append(results, map[string]any{"source_entity_id": sourceID, "status": "unlinked", "target_entity_id": target.StableEntityID})
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid, EventType: "item_identity_manual_unmerge", TargetType: "entity_identity",
		Summary:     fmt.Sprintf("revoked %d item identity links", revoked),
		DetailsJSON: mustCompactJSON(map[string]any{"target_entity_id": target.StableEntityID, "results": results}), Source: s.storeWriteSource(),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "contract_version": itemIdentityManualMergeContract,
		"chat_session_id": sid, "results": results, "unlinked_count": revoked,
		"stored_rows_deleted": 0, "critic_calls": 0, "vector_reindex_queued": false,
	})
}

func itemIdentityManualLink(sid, sourceID, targetID, state string) store.EntityIdentityLink {
	now := time.Now().UTC()
	return store.EntityIdentityLink{
		LinkID:        entityIdentityStableID("item_identity_manual_link", sid, sourceID, targetID),
		ChatSessionID: sid, SourceEntityID: sourceID, TargetEntityID: targetID,
		LinkKind: store.EntityIdentityLinkKindCanonicalEquivalence, LinkState: state,
		EvidenceJSON:    mustCompactJSON(map[string]any{"contract_version": itemIdentityManualMergeContract, "operator_explicit": true, "link_state": state, "entity_kind": "item"}),
		MappingRevision: 1, SourceContract: itemIdentityManualMergeContract,
		SourceRevision: itemIdentityManualMergeContract + ":" + sourceID + ":" + targetID,
		CreatedAt:      now, UpdatedAt: now,
	}
}

func (s *Server) itemIdentityMergeImpacts(ctx context.Context, sid string, selected map[string]bool, catalog characterIdentityCatalog) map[string]characterIdentityImpact {
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
	impacts := map[string]characterIdentityImpact{"alias_surfaces": {Status: "ready", Count: len(selectedNames)}}
	triples, err := s.Store.ListKGTriples(ctx, sid)
	if err != nil {
		impacts["knowledge_relations"] = characterIdentityUnavailableImpact(err)
		return impacts
	}
	count := 0
	for _, triple := range triples {
		if selectedNames[comparableEntityKey(triple.Subject)] || selectedNames[comparableEntityKey(triple.Object)] {
			count++
		}
	}
	impacts["knowledge_relations"] = characterIdentityImpact{Status: "ready", Count: count}
	return impacts
}

func itemIdentityPredicate(predicate string) bool {
	predicate = strings.ToLower(strings.TrimSpace(predicate))
	for _, hint := range []string{"has", "have", "owns", "own", "carry", "carries", "held", "holds", "wield", "equip", "use", "item", "weapon", "artifact", "tool", "inventory", "소유", "보유", "장비", "무기", "아이템", "획득"} {
		if strings.Contains(predicate, hint) {
			return true
		}
	}
	return false
}

func (s *Server) canonicalEntitySurfaceOfKind(ctx context.Context, sid, surface, entityKind string) (string, string) {
	surface = strings.TrimSpace(surface)
	resolver, ok := s.Store.(store.UniqueActiveEntitySurfaceIdentityResolver)
	if !ok || surface == "" {
		return surface, ""
	}
	resolved, err := resolver.ResolveUniqueActiveEntityIdentityBySurface(ctx, sid, comparableEntityKey(surface))
	if err != nil || resolved.EntityKind != entityKind {
		return surface, ""
	}
	label := strings.TrimSpace(resolved.CanonicalLabel)
	if label == "" {
		label = surface
	}
	return label, strings.TrimSpace(resolved.StableEntityID)
}

func (s *Server) itemIdentityAliases(ctx context.Context, sid string, catalog characterIdentityCatalog) map[string][]string {
	aliases := map[string][]string{}
	for id, identity := range catalog.Identities {
		rootID := s.characterIdentityRoot(ctx, sid, id)
		root, exists := catalog.Identities[rootID]
		if !exists {
			continue
		}
		if label := strings.TrimSpace(identity.CanonicalLabel); label != "" && comparableEntityKey(label) != comparableEntityKey(root.CanonicalLabel) {
			aliases[rootID] = appendUniqueString(aliases[rootID], label)
		}
	}
	for _, surface := range catalog.Surfaces {
		rootID := s.characterIdentityRoot(ctx, sid, surface.StableEntityID)
		root, exists := catalog.Identities[rootID]
		if !exists {
			continue
		}
		label := strings.TrimSpace(surface.SurfaceText)
		if label != "" && comparableEntityKey(label) != comparableEntityKey(root.CanonicalLabel) {
			aliases[rootID] = appendUniqueString(aliases[rootID], label)
		}
	}
	return aliases
}

func itemIdentityLinkItems(catalog characterIdentityCatalog) []map[string]any {
	out := []map[string]any{}
	for _, link := range catalog.Links {
		source, sourceOK := catalog.Identities[link.SourceEntityID]
		target, targetOK := catalog.Identities[link.TargetEntityID]
		if !sourceOK || !targetOK {
			continue
		}
		out = append(out, map[string]any{
			"link_id": link.LinkID, "source_entity_id": link.SourceEntityID,
			"source_label": source.CanonicalLabel, "target_entity_id": link.TargetEntityID,
			"target_label": target.CanonicalLabel, "link_state": link.LinkState,
		})
	}
	return out
}
