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
	catalog, err := s.entityIdentityCatalogForSession(r.Context(), sid, "")
	if err != nil {
		if !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		catalog = characterIdentityCatalog{Identities: map[string]store.EntityIdentity{}, Surfaces: []store.EntityIdentitySurface{}, Links: []store.EntityIdentityLink{}}
	}
	identityRead := buildItemIdentityReadIndex(sid, catalog)
	catalog = identityRead.Catalog

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
	canonicalCharactersBySession := map[string]map[string]string{
		sid: s.characterCanonicalSurfaceMapForRead(r.Context(), sid, identityRead.AllCatalog),
	}
	items := []map[string]any{}
	seen := map[string]bool{}
	for _, triple := range triples {
		if !itemIdentityPredicate(triple.Predicate) {
			continue
		}
		itemName := strings.TrimSpace(triple.Object)
		stableID := ""
		if triple.ChatSessionID == sid {
			itemName, stableID = identityRead.CanonicalSurface(itemName)
		}
		key := comparableEntityKey(itemName)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		ownerName := strings.TrimSpace(triple.Subject)
		ownerSID := strings.TrimSpace(triple.ChatSessionID)
		if ownerSID == "" {
			ownerSID = sid
		}
		canonicalCharacters, loaded := canonicalCharactersBySession[ownerSID]
		if !loaded {
			canonicalCharacters = s.characterCanonicalSurfaceMapForRead(r.Context(), ownerSID)
			canonicalCharactersBySession[ownerSID] = canonicalCharacters
		}
		if canonical := strings.TrimSpace(canonicalCharacters[comparableEntityKey(ownerName)]); canonical != "" {
			ownerName = canonical
		}
		row := explorerHistoryItem(map[string]any{
			"id": triple.ID, "item": itemName,
			"owner":     ownerName,
			"predicate": triple.Predicate, "source_turn": nullablePositiveInt(triple.SourceTurn),
		}, sid, triple.ChatSessionID)
		if stableID != "" {
			row["stable_entity_id"] = stableID
			row["aliases"] = nonNilSlice(identityRead.Aliases[stableID])
		}
		items = append(items, row)
	}

	identityIDs := make([]string, 0, len(catalog.Identities))
	for id := range catalog.Identities {
		identityIDs = append(identityIDs, id)
	}
	sort.Strings(identityIDs)
	for _, id := range identityIDs {
		rootID := identityRead.RootID(id)
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
			"aliases": nonNilSlice(identityRead.Aliases[rootID]), "mutation_allowed": true,
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

// durableItemIdentityPredicate is deliberately narrower than the explorer's
// display predicate. Session normalization may create durable identity rows,
// so only an exact, already accepted item relation is eligible; substring
// matches such as "causes" must remain display-only observations.
func durableItemIdentityPredicate(predicate string) bool {
	switch strings.ToLower(strings.TrimSpace(predicate)) {
	case "has", "have", "owns", "own",
		"carry", "carries", "carried", "held", "hold", "holds",
		"wield", "wields", "wielded",
		"equip", "equips", "equipped",
		"use", "uses", "used",
		"item", "weapon", "artifact", "tool", "inventory",
		"소유", "보유", "장비", "무기", "아이템", "획득":
		return true
	default:
		return false
	}
}

type itemIdentityReadIndex struct {
	AllCatalog         characterIdentityCatalog
	Catalog            characterIdentityCatalog
	Roots              map[string]string
	CanonicalBySurface map[string]store.ResolvedEntityIdentity
	Aliases            map[string][]string
}

func buildItemIdentityReadIndex(sid string, all characterIdentityCatalog) itemIdentityReadIndex {
	itemCatalog := characterIdentityCatalog{
		Identities: map[string]store.EntityIdentity{},
		Surfaces:   nonNilSlice(all.Surfaces),
		Links:      nonNilSlice(all.Links),
	}
	for id, identity := range all.Identities {
		if identity.ChatSessionID == sid && identity.EntityKind == "item" {
			itemCatalog.Identities[id] = identity
		}
	}
	index := itemIdentityReadIndex{
		AllCatalog: all, Catalog: itemCatalog, Roots: map[string]string{},
		CanonicalBySurface: map[string]store.ResolvedEntityIdentity{}, Aliases: map[string][]string{},
	}
	targets := map[string]map[string]bool{}
	invalidRoot := map[string]bool{}
	for _, link := range all.Links {
		if link.ChatSessionID != sid || link.LinkKind != store.EntityIdentityLinkKindCanonicalEquivalence ||
			link.LinkState != store.EntityIdentityLinkStateReviewed {
			continue
		}
		sourceID := strings.TrimSpace(link.SourceEntityID)
		targetID := strings.TrimSpace(link.TargetEntityID)
		if _, sourceOK := itemCatalog.Identities[sourceID]; !sourceOK {
			continue
		}
		if _, targetOK := itemCatalog.Identities[targetID]; !targetOK {
			invalidRoot[sourceID] = true
			continue
		}
		if targets[sourceID] == nil {
			targets[sourceID] = map[string]bool{}
		}
		targets[sourceID][targetID] = true
	}
	var rootFor func(string, map[string]bool) string
	rootFor = func(entityID string, visiting map[string]bool) string {
		if invalidRoot[entityID] {
			return ""
		}
		if root, ok := index.Roots[entityID]; ok {
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
		index.Roots[entityID] = root
		return root
	}
	for id := range itemCatalog.Identities {
		rootFor(id, map[string]bool{})
	}

	type surfaceCandidate struct {
		resolved map[string]store.ResolvedEntityIdentity
		blocked  bool
	}
	candidates := map[string]*surfaceCandidate{}
	addSurface := func(surface, entityID string) {
		key := comparableEntityKey(surface)
		if key == "" {
			return
		}
		candidate := candidates[key]
		if candidate == nil {
			candidate = &surfaceCandidate{resolved: map[string]store.ResolvedEntityIdentity{}}
			candidates[key] = candidate
		}
		if _, ok := itemCatalog.Identities[entityID]; !ok {
			candidate.blocked = true
			return
		}
		rootID := rootFor(entityID, map[string]bool{})
		root, ok := itemCatalog.Identities[rootID]
		if !ok || rootID == "" || strings.TrimSpace(root.CanonicalLabel) == "" {
			candidate.blocked = true
			return
		}
		candidate.resolved[rootID] = store.ResolvedEntityIdentity{
			StableEntityID: rootID, IdentityNamespace: root.IdentityNamespace,
			EntityKind: root.EntityKind, CanonicalLabel: root.CanonicalLabel,
		}
	}
	for _, surface := range all.Surfaces {
		if surface.ChatSessionID == sid {
			addSurface(surface.SurfaceText, strings.TrimSpace(surface.StableEntityID))
		}
	}
	for key, candidate := range candidates {
		if candidate.blocked || len(candidate.resolved) != 1 {
			continue
		}
		for _, resolved := range candidate.resolved {
			index.CanonicalBySurface[key] = resolved
		}
	}
	for id, identity := range itemCatalog.Identities {
		rootID := rootFor(id, map[string]bool{})
		root, exists := itemCatalog.Identities[rootID]
		if !exists {
			continue
		}
		if label := strings.TrimSpace(identity.CanonicalLabel); label != "" && comparableEntityKey(label) != comparableEntityKey(root.CanonicalLabel) {
			index.Aliases[rootID] = appendUniqueString(index.Aliases[rootID], label)
		}
	}
	for _, surface := range all.Surfaces {
		entityID := strings.TrimSpace(surface.StableEntityID)
		if _, exists := itemCatalog.Identities[entityID]; !exists {
			continue
		}
		rootID := rootFor(entityID, map[string]bool{})
		root, exists := itemCatalog.Identities[rootID]
		if !exists {
			continue
		}
		label := strings.TrimSpace(surface.SurfaceText)
		if label != "" && comparableEntityKey(label) != comparableEntityKey(root.CanonicalLabel) {
			index.Aliases[rootID] = appendUniqueString(index.Aliases[rootID], label)
		}
	}
	return index
}

func (i itemIdentityReadIndex) RootID(entityID string) string {
	entityID = strings.TrimSpace(entityID)
	if root := strings.TrimSpace(i.Roots[entityID]); root != "" {
		return root
	}
	return entityID
}

func (i itemIdentityReadIndex) CanonicalSurface(surface string) (string, string) {
	surface = strings.TrimSpace(surface)
	resolved, ok := i.CanonicalBySurface[comparableEntityKey(surface)]
	if !ok {
		return surface, ""
	}
	label := strings.TrimSpace(resolved.CanonicalLabel)
	if label == "" {
		label = surface
	}
	return label, strings.TrimSpace(resolved.StableEntityID)
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
