package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	step23ForkLineageContractVersion   = "step23_fork_lineage.v1"
	worldlineViewModelContract         = "worldline_view_model.v1"
	worldlineTopologyViewModelContract = "worldline_topology.viewmodel.v2"
	worldlineTopologyFamilyLimit       = 256
	step23ForkLineageListRoute         = "/step23/fork-lineage/{chat_session_id}"
	step23ForkLineageDeclareRoute      = "/step23/fork-lineage"
)

type step23ForkLineageRecordResponse struct {
	ID                   int64     `json:"id"`
	ContractVersion      string    `json:"contract_version"`
	LineageState         string    `json:"lineage_state"`
	ChatSessionID        string    `json:"chat_session_id"`
	ScopeID              string    `json:"scope_id,omitempty"`
	ParentScopeID        string    `json:"parent_scope_id,omitempty"`
	CopiedFromScopeID    string    `json:"copied_from_scope_id,omitempty"`
	CopiedFromSessionID  string    `json:"copied_from_session_id,omitempty"`
	ForkTurn             int       `json:"fork_turn,omitempty"`
	ForkSourceMessageID  string    `json:"fork_source_message_id,omitempty"`
	ForkSourceRole       string    `json:"fork_source_role,omitempty"`
	InheritedThroughTurn int       `json:"inherited_through_turn"`
	ImportedAt           time.Time `json:"imported_at"`
	DivergenceMarker     string    `json:"divergence_marker,omitempty"`
	ProvenanceSource     string    `json:"provenance_source"`
	InheritanceMode      string    `json:"inheritance_mode"`
	InheritedItemsJSON   string    `json:"inherited_items_json,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type worldlineViewModel struct {
	ContractVersion        string                          `json:"contract_version"`
	State                  string                          `json:"state"`
	CurrentSessionID       string                          `json:"current_session_id"`
	ParentSessionID        string                          `json:"parent_session_id,omitempty"`
	ForkTurn               int                             `json:"fork_turn,omitempty"`
	ForkSourceMessageID    string                          `json:"fork_source_message_id,omitempty"`
	ForkSourceRole         string                          `json:"fork_source_role,omitempty"`
	InheritedThroughTurn   int                             `json:"inherited_through_turn"`
	Reason                 string                          `json:"reason"`
	CandidateParentID      string                          `json:"candidate_parent_session_id,omitempty"`
	CandidateForkTurns     []int                           `json:"candidate_fork_turns,omitempty"`
	MessageOriginsRecorded bool                            `json:"message_origins_recorded,omitempty"`
	OriginReadRequest      *risuWorldlineOriginReadRequest `json:"origin_read_request,omitempty"`
}

type worldlineTopologyViewModel struct {
	ContractVersion    string                         `json:"contract_version"`
	State              string                         `json:"state"`
	Nodes              []worldlineTopologyNode        `json:"nodes"`
	Edges              []worldlineTopologyEdge        `json:"edges"`
	CurrentSessionID   string                         `json:"current_session_id"`
	SelectedSessionID  string                         `json:"selected_session_id"`
	CurrentNodeID      string                         `json:"current_node_id"`
	SelectedNodeID     string                         `json:"selected_node_id"`
	ActiveAncestorPath []string                       `json:"active_ancestor_path"`
	Bounds             worldlineTopologyLogicalBounds `json:"bounds"`
	Truncated          bool                           `json:"truncated"`
	Reason             string                         `json:"reason"`
}

type worldlineTopologyNode struct {
	NodeID             string `json:"node_id"`
	SessionID          string `json:"session_id"`
	TurnKey            string `json:"turn_key"`
	TurnIndex          int    `json:"turn_index"`
	TurnText           string `json:"turn_text"`
	X                  int    `json:"x"`
	Y                  int    `json:"y"`
	Current            bool   `json:"current"`
	Selected           bool   `json:"selected"`
	ActiveAncestorPath bool   `json:"active_ancestor_path"`
}

type worldlineTopologyEdge struct {
	ParentNodeID       string `json:"parent_node_id"`
	ChildNodeID        string `json:"child_node_id"`
	Kind               string `json:"kind"`
	ActiveAncestorPath bool   `json:"active_ancestor_path"`
}

type worldlineTopologyLogicalBounds struct {
	MinX   int `json:"min_x"`
	MinY   int `json:"min_y"`
	MaxX   int `json:"max_x"`
	MaxY   int `json:"max_y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type worldlineTopologyCandidate struct {
	ChildSessionID      string
	ParentSessionID     string
	ForkTurn            int
	ForkSourceMessageID string
	ForkSourceRole      string
	SourceBoundaryTurn  int
}

func emptyWorldlineTopologyViewModel(currentSessionID, selectedSessionID, reason string) worldlineTopologyViewModel {
	return worldlineTopologyViewModel{
		ContractVersion:    worldlineTopologyViewModelContract,
		State:              "unavailable",
		Nodes:              []worldlineTopologyNode{},
		Edges:              []worldlineTopologyEdge{},
		CurrentSessionID:   strings.TrimSpace(currentSessionID),
		SelectedSessionID:  strings.TrimSpace(selectedSessionID),
		ActiveAncestorPath: []string{},
		Reason:             reason,
	}
}

func buildWorldlineTopologyViewModel(snapshot store.WorldlineTopologySnapshot, currentSessionID, selectedSessionID string) worldlineTopologyViewModel {
	currentSessionID = strings.TrimSpace(currentSessionID)
	selectedSessionID = strings.TrimSpace(selectedSessionID)
	vm := emptyWorldlineTopologyViewModel(currentSessionID, selectedSessionID, "topology_ready")
	vm.State = "ready"
	vm.Truncated = snapshot.Truncated || snapshot.TurnsTruncated
	partialReasons := map[string]bool{}
	if snapshot.Truncated {
		partialReasons["family_limit_reached"] = true
	}
	if snapshot.TurnsTruncated {
		partialReasons["turn_limit_reached"] = true
	}

	sessionSet := map[string]bool{}
	for _, rawSessionID := range snapshot.SessionIDs {
		if sessionID := strings.TrimSpace(rawSessionID); sessionID != "" {
			sessionSet[sessionID] = true
		}
	}
	anchorSessionID := strings.TrimSpace(snapshot.AnchorSessionID)
	if anchorSessionID != "" {
		sessionSet[anchorSessionID] = true
	}
	sessionIDs := make([]string, 0, len(sessionSet))
	for sessionID := range sessionSet {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Strings(sessionIDs)
	if len(sessionIDs) == 0 || anchorSessionID == "" {
		vm.State = "unavailable"
		vm.Reason = "session_anchor_unavailable"
		return vm
	}

	recordsBySession := make(map[string][]store.ForkLineageRecord, len(sessionIDs))
	for _, record := range snapshot.LineageRecords {
		childSessionID := strings.TrimSpace(record.ChatSessionID)
		if !sessionSet[childSessionID] {
			continue
		}
		recordsBySession[childSessionID] = append(recordsBySession[childSessionID], record)
	}

	authoritativeCandidates := make(map[string]worldlineTopologyCandidate, len(sessionIDs))
	graphCandidates := make(map[string]worldlineTopologyCandidate, len(sessionIDs))
	unsafeOwnership := make(map[string]bool, len(sessionIDs))
	lineageReasonBySession := make(map[string]string, len(sessionIDs))
	for _, childSessionID := range sessionIDs {
		candidateByTuple := map[string]worldlineTopologyCandidate{}
		hasInvalidConfirmedV2 := false
		hasUnresolvedV2 := false
		hasLegacy := false
		for _, record := range recordsBySession[childSessionID] {
			if record.ContractVersion != store.RisuWorldlineForkLineageContractVersion {
				hasLegacy = true
				continue
			}
			if record.LineageState != "confirmed" {
				hasUnresolvedV2 = true
				continue
			}
			if strings.TrimSpace(record.IdempotencyKey) == "" {
				hasInvalidConfirmedV2 = true
				continue
			}
			parentSessionID := strings.TrimSpace(record.CopiedFromSessionID)
			forkSourceMessageID := strings.TrimSpace(record.ForkSourceMessageID)
			sourceBoundaryTurn, boundaryOK := worldlineInheritedThroughTurn(record.ForkTurn, record.ForkSourceRole)
			if parentSessionID == "" || parentSessionID == childSessionID || forkSourceMessageID == "" || !boundaryOK {
				hasInvalidConfirmedV2 = true
				continue
			}
			candidate := worldlineTopologyCandidate{
				ChildSessionID:      childSessionID,
				ParentSessionID:     parentSessionID,
				ForkTurn:            record.ForkTurn,
				ForkSourceMessageID: forkSourceMessageID,
				ForkSourceRole:      strings.TrimSpace(record.ForkSourceRole),
				SourceBoundaryTurn:  sourceBoundaryTurn,
			}
			candidateByTuple[worldlineTopologyCandidateKey(candidate)] = candidate
		}

		switch {
		case hasInvalidConfirmedV2:
			unsafeOwnership[childSessionID] = true
			lineageReasonBySession[childSessionID] = "confirmed_v2_tuple_invalid"
		case len(candidateByTuple) > 1:
			unsafeOwnership[childSessionID] = true
			lineageReasonBySession[childSessionID] = "confirmed_v2_tuple_conflict"
		case len(candidateByTuple) == 1:
			var candidate worldlineTopologyCandidate
			for _, value := range candidateByTuple {
				candidate = value
			}
			authoritativeCandidates[childSessionID] = candidate
			if sessionSet[candidate.ParentSessionID] {
				graphCandidates[childSessionID] = candidate
			} else {
				lineageReasonBySession[childSessionID] = "parent_outside_current_family"
			}
		case hasUnresolvedV2:
			unsafeOwnership[childSessionID] = true
			lineageReasonBySession[childSessionID] = "unresolved_v2_lineage"
		case hasLegacy:
			unsafeOwnership[childSessionID] = true
			lineageReasonBySession[childSessionID] = "legacy_lineage_non_authoritative"
		}
	}

	cycleMembers := worldlineTopologyCycleMembers(graphCandidates, sessionIDs)
	for sessionID := range cycleMembers {
		unsafeOwnership[sessionID] = true
		lineageReasonBySession[sessionID] = "confirmed_v2_cycle_cut"
		delete(graphCandidates, sessionID)
	}
	for childSessionID, candidate := range graphCandidates {
		if cycleMembers[candidate.ParentSessionID] {
			lineageReasonBySession[childSessionID] = "confirmed_v2_cycle_cut"
			delete(graphCandidates, childSessionID)
		}
	}

	adjacentSessions := make(map[string][]string, len(sessionIDs))
	for childSessionID, candidate := range graphCandidates {
		adjacentSessions[candidate.ParentSessionID] = append(adjacentSessions[candidate.ParentSessionID], childSessionID)
		adjacentSessions[childSessionID] = append(adjacentSessions[childSessionID], candidate.ParentSessionID)
	}
	componentSessions := map[string]bool{anchorSessionID: true}
	queue := []string{anchorSessionID}
	for len(queue) > 0 {
		sessionID := queue[0]
		queue = queue[1:]
		sort.Strings(adjacentSessions[sessionID])
		for _, adjacentSessionID := range adjacentSessions[sessionID] {
			if componentSessions[adjacentSessionID] {
				continue
			}
			componentSessions[adjacentSessionID] = true
			queue = append(queue, adjacentSessionID)
		}
	}
	for sessionID := range componentSessions {
		if reason := lineageReasonBySession[sessionID]; reason != "" {
			partialReasons[reason] = true
		}
	}

	childrenByParent := make(map[string][]string, len(componentSessions))
	for childSessionID, candidate := range graphCandidates {
		if componentSessions[childSessionID] && componentSessions[candidate.ParentSessionID] {
			childrenByParent[candidate.ParentSessionID] = append(childrenByParent[candidate.ParentSessionID], childSessionID)
		}
	}
	for parentSessionID := range childrenByParent {
		sort.Slice(childrenByParent[parentSessionID], func(i, j int) bool {
			left := graphCandidates[childrenByParent[parentSessionID][i]]
			right := graphCandidates[childrenByParent[parentSessionID][j]]
			if left.SourceBoundaryTurn != right.SourceBoundaryTurn {
				return left.SourceBoundaryTurn < right.SourceBoundaryTurn
			}
			return left.ChildSessionID < right.ChildSessionID
		})
	}

	roots := make([]string, 0, len(componentSessions))
	for sessionID := range componentSessions {
		if _, hasParent := graphCandidates[sessionID]; !hasParent {
			roots = append(roots, sessionID)
		}
	}
	sort.Strings(roots)
	if len(roots) != 1 {
		partialReasons["component_root_conflict"] = true
	}
	orderedSessionIDs := make([]string, 0, len(componentSessions))
	laneBySession := make(map[string]int, len(componentSessions))
	usedLanes := map[int]bool{0: true}
	claimLane := func(direction, start int) int {
		lane := start
		if direction < 0 {
			if lane >= 0 {
				lane = -1
			}
			for usedLanes[lane] {
				lane--
			}
		} else {
			if lane <= 0 {
				lane = 1
			}
			for usedLanes[lane] {
				lane++
			}
		}
		usedLanes[lane] = true
		return lane
	}
	var visit func(string)
	visit = func(sessionID string) {
		orderedSessionIDs = append(orderedSessionIDs, sessionID)
		parentLane := laneBySession[sessionID]
		for childIndex, childSessionID := range childrenByParent[sessionID] {
			direction := 1
			start := parentLane + 1
			switch {
			case parentLane < 0:
				direction = -1
				start = parentLane - 1
			case parentLane == 0 && childIndex%2 == 0:
				direction = -1
				start = -1
			}
			laneBySession[childSessionID] = claimLane(direction, start)
			visit(childSessionID)
		}
	}
	for index, rootSessionID := range roots {
		if index > 0 {
			direction := -1
			if index%2 == 0 {
				direction = 1
			}
			laneBySession[rootSessionID] = claimLane(direction, direction)
		}
		visit(rootSessionID)
	}

	completedTurnSets := make(map[string]map[int]bool, len(componentSessions))
	for _, completedTurn := range snapshot.CompletedTurns {
		sessionID := strings.TrimSpace(completedTurn.ChatSessionID)
		if !componentSessions[sessionID] || completedTurn.TurnIndex <= 0 || unsafeOwnership[sessionID] {
			continue
		}
		if candidate, ok := authoritativeCandidates[sessionID]; ok && completedTurn.TurnIndex <= candidate.SourceBoundaryTurn {
			continue
		}
		if completedTurnSets[sessionID] == nil {
			completedTurnSets[sessionID] = map[int]bool{}
		}
		completedTurnSets[sessionID][completedTurn.TurnIndex] = true
	}

	ownedTurnsBySession := make(map[string][]int, len(componentSessions))
	nodeIDBySessionTurn := make(map[string]map[int]string, len(componentSessions))
	nodesByID := map[string]*worldlineTopologyNode{}
	for _, sessionID := range orderedSessionIDs {
		turns := make([]int, 0, len(completedTurnSets[sessionID]))
		for turnIndex := range completedTurnSets[sessionID] {
			turns = append(turns, turnIndex)
		}
		sort.Ints(turns)
		ownedTurnsBySession[sessionID] = turns
		if len(turns) > 0 {
			expectedFirstTurn := 1
			if candidate, ok := authoritativeCandidates[sessionID]; ok {
				expectedFirstTurn = candidate.SourceBoundaryTurn + 1
			}
			if turns[0] != expectedFirstTurn {
				partialReasons["completed_turn_gap"] = true
			}
		}
		for index, turnIndex := range turns {
			if index > 0 && turnIndex != turns[index-1]+1 {
				partialReasons["completed_turn_gap"] = true
			}
			nodeID := worldlineTopologyTurnNodeID(sessionID, turnIndex)
			if nodeIDBySessionTurn[sessionID] == nil {
				nodeIDBySessionTurn[sessionID] = map[int]string{}
			}
			nodeIDBySessionTurn[sessionID][turnIndex] = nodeID
			nodesByID[nodeID] = &worldlineTopologyNode{
				NodeID:    nodeID,
				SessionID: sessionID,
				TurnKey:   fmt.Sprintf("turn:%d", turnIndex),
				TurnIndex: turnIndex,
				TurnText:  fmt.Sprintf("%d", turnIndex),
				X:         turnIndex,
				Y:         laneBySession[sessionID],
			}
		}
	}

	edges := make([]worldlineTopologyEdge, 0, len(nodesByID))
	for _, sessionID := range orderedSessionIDs {
		turns := ownedTurnsBySession[sessionID]
		for index := 1; index < len(turns); index++ {
			if turns[index] != turns[index-1]+1 {
				continue
			}
			edges = append(edges, worldlineTopologyEdge{
				ParentNodeID: nodeIDBySessionTurn[sessionID][turns[index-1]],
				ChildNodeID:  nodeIDBySessionTurn[sessionID][turns[index]],
				Kind:         "continuation",
			})
		}
	}

	branchSessionIDs := make([]string, 0, len(graphCandidates))
	for childSessionID, candidate := range graphCandidates {
		if componentSessions[childSessionID] && componentSessions[candidate.ParentSessionID] {
			branchSessionIDs = append(branchSessionIDs, childSessionID)
		}
	}
	sort.Slice(branchSessionIDs, func(i, j int) bool {
		left := graphCandidates[branchSessionIDs[i]]
		right := graphCandidates[branchSessionIDs[j]]
		if left.SourceBoundaryTurn != right.SourceBoundaryTurn {
			return left.SourceBoundaryTurn < right.SourceBoundaryTurn
		}
		return left.ChildSessionID < right.ChildSessionID
	})
	for _, childSessionID := range branchSessionIDs {
		candidate := graphCandidates[childSessionID]
		childTurns := ownedTurnsBySession[childSessionID]
		if len(childTurns) == 0 {
			continue
		}
		if childTurns[0] != candidate.SourceBoundaryTurn+1 {
			continue
		}
		parentNodeID, ok := worldlineTopologyVisibleTurnNodeID(
			candidate.ParentSessionID,
			candidate.SourceBoundaryTurn,
			nodeIDBySessionTurn,
			graphCandidates,
		)
		if !ok {
			partialReasons["fork_source_turn_missing"] = true
			continue
		}
		edges = append(edges, worldlineTopologyEdge{
			ParentNodeID: parentNodeID,
			ChildNodeID:  nodeIDBySessionTurn[childSessionID][childTurns[0]],
			Kind:         "fork",
		})
	}

	vm.CurrentNodeID = worldlineTopologySessionTipNodeID(currentSessionID, ownedTurnsBySession, nodeIDBySessionTurn, graphCandidates)
	vm.SelectedNodeID = worldlineTopologySessionTipNodeID(selectedSessionID, ownedTurnsBySession, nodeIDBySessionTurn, graphCandidates)
	if currentNode := nodesByID[vm.CurrentNodeID]; currentNode != nil {
		currentNode.Current = true
	}
	if selectedNode := nodesByID[vm.SelectedNodeID]; selectedNode != nil {
		selectedNode.Selected = true
	}

	incomingNodeID := make(map[string]string, len(edges))
	for _, edge := range edges {
		incomingNodeID[edge.ChildNodeID] = edge.ParentNodeID
	}
	for cursor, seen := vm.CurrentNodeID, map[string]bool{}; cursor != "" && !seen[cursor]; cursor = incomingNodeID[cursor] {
		seen[cursor] = true
		vm.ActiveAncestorPath = append(vm.ActiveAncestorPath, cursor)
	}
	for left, right := 0, len(vm.ActiveAncestorPath)-1; left < right; left, right = left+1, right-1 {
		vm.ActiveAncestorPath[left], vm.ActiveAncestorPath[right] = vm.ActiveAncestorPath[right], vm.ActiveAncestorPath[left]
	}
	activeNodeIDs := make(map[string]bool, len(vm.ActiveAncestorPath))
	activeEdges := map[string]bool{}
	for index, nodeID := range vm.ActiveAncestorPath {
		activeNodeIDs[nodeID] = true
		if index > 0 {
			activeEdges[vm.ActiveAncestorPath[index-1]+"\x1f"+nodeID] = true
		}
	}
	for nodeID := range activeNodeIDs {
		if node := nodesByID[nodeID]; node != nil {
			node.ActiveAncestorPath = true
		}
	}
	for index := range edges {
		edges[index].ActiveAncestorPath = activeEdges[edges[index].ParentNodeID+"\x1f"+edges[index].ChildNodeID]
	}

	nodeIDs := make([]string, 0, len(nodesByID))
	for nodeID := range nodesByID {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Slice(nodeIDs, func(i, j int) bool {
		left := nodesByID[nodeIDs[i]]
		right := nodesByID[nodeIDs[j]]
		if left.X != right.X {
			return left.X < right.X
		}
		if left.Y != right.Y {
			return left.Y < right.Y
		}
		return left.NodeID < right.NodeID
	})
	vm.Nodes = make([]worldlineTopologyNode, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		vm.Nodes = append(vm.Nodes, *nodesByID[nodeID])
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].ParentNodeID != edges[j].ParentNodeID {
			return edges[i].ParentNodeID < edges[j].ParentNodeID
		}
		if edges[i].ChildNodeID != edges[j].ChildNodeID {
			return edges[i].ChildNodeID < edges[j].ChildNodeID
		}
		return edges[i].Kind < edges[j].Kind
	})
	vm.Edges = edges
	if len(vm.Nodes) > 0 {
		minX, maxX := vm.Nodes[0].X, vm.Nodes[0].X
		minY, maxY := vm.Nodes[0].Y, vm.Nodes[0].Y
		for _, node := range vm.Nodes[1:] {
			if node.X < minX {
				minX = node.X
			}
			if node.X > maxX {
				maxX = node.X
			}
			if node.Y < minY {
				minY = node.Y
			}
			if node.Y > maxY {
				maxY = node.Y
			}
		}
		vm.Bounds = worldlineTopologyLogicalBounds{
			MinX:   minX,
			MinY:   minY,
			MaxX:   maxX,
			MaxY:   maxY,
			Width:  maxX - minX + 1,
			Height: maxY - minY + 1,
		}
	}
	if len(partialReasons) > 0 {
		vm.State = "partial"
		vm.Reason = worldlineTopologyPartialReason(partialReasons)
	}
	return vm
}

func worldlineTopologyVisibleTurnNodeID(
	parentSessionID string,
	turnIndex int,
	nodeIDBySessionTurn map[string]map[int]string,
	candidates map[string]worldlineTopologyCandidate,
) (string, bool) {
	visited := map[string]bool{}
	for sessionID := strings.TrimSpace(parentSessionID); sessionID != "" && !visited[sessionID]; {
		visited[sessionID] = true
		if nodeID := nodeIDBySessionTurn[sessionID][turnIndex]; nodeID != "" {
			return nodeID, true
		}
		candidate, ok := candidates[sessionID]
		if !ok || turnIndex > candidate.SourceBoundaryTurn {
			return "", false
		}
		sessionID = candidate.ParentSessionID
	}
	return "", false
}

func worldlineTopologySessionTipNodeID(
	sessionID string,
	ownedTurnsBySession map[string][]int,
	nodeIDBySessionTurn map[string]map[int]string,
	candidates map[string]worldlineTopologyCandidate,
) string {
	sessionID = strings.TrimSpace(sessionID)
	turns := ownedTurnsBySession[sessionID]
	if len(turns) > 0 {
		return nodeIDBySessionTurn[sessionID][turns[len(turns)-1]]
	}
	if candidate, ok := candidates[sessionID]; ok {
		if nodeID, resolved := worldlineTopologyVisibleTurnNodeID(candidate.ParentSessionID, candidate.SourceBoundaryTurn, nodeIDBySessionTurn, candidates); resolved {
			return nodeID
		}
	}
	return ""
}

func worldlineTopologyTurnNodeID(sessionID string, turnIndex int) string {
	return fmt.Sprintf("worldline-turn:%d:%s:turn:%d", len([]byte(sessionID)), sessionID, turnIndex)
}

func worldlineTopologyPartialReason(reasons map[string]bool) string {
	for _, reason := range []string{
		"family_limit_reached",
		"turn_limit_reached",
		"confirmed_v2_cycle_cut",
		"confirmed_v2_tuple_invalid",
		"confirmed_v2_tuple_conflict",
		"unresolved_v2_lineage",
		"legacy_lineage_non_authoritative",
		"parent_outside_current_family",
		"component_root_conflict",
		"completed_turn_gap",
		"fork_source_turn_missing",
	} {
		if reasons[reason] {
			return reason
		}
	}
	return "topology_partial"
}

func worldlineTopologyCandidateKey(candidate worldlineTopologyCandidate) string {
	return strings.Join([]string{
		candidate.ParentSessionID,
		fmt.Sprintf("%d", candidate.ForkTurn),
		candidate.ForkSourceMessageID,
		candidate.ForkSourceRole,
	}, "\x1f")
}

func worldlineTopologyCycleMembers(candidates map[string]worldlineTopologyCandidate, sessionIDs []string) map[string]bool {
	cycleMembers := map[string]bool{}
	settled := map[string]bool{}
	for _, startSessionID := range sessionIDs {
		if settled[startSessionID] {
			continue
		}
		path := []string{}
		pathIndex := map[string]int{}
		cursor := startSessionID
		for cursor != "" && !settled[cursor] {
			if index, exists := pathIndex[cursor]; exists {
				for _, sessionID := range path[index:] {
					cycleMembers[sessionID] = true
				}
				break
			}
			pathIndex[cursor] = len(path)
			path = append(path, cursor)
			candidate, ok := candidates[cursor]
			if !ok {
				break
			}
			cursor = candidate.ParentSessionID
		}
		for _, sessionID := range path {
			settled[sessionID] = true
		}
	}
	return cycleMembers
}

func currentWorldlineViewModel(ctx context.Context, st store.Store, sessionID string) worldlineViewModel {
	sid := strings.TrimSpace(sessionID)
	vm := worldlineViewModel{
		ContractVersion:  worldlineViewModelContract,
		State:            "not_applicable",
		CurrentSessionID: sid,
		Reason:           "no_confirmed_fork_lineage",
	}
	lineageStore, ok := st.(store.ForkLineageStore)
	if !ok || sid == "" {
		return vm
	}
	records, err := lineageStore.ListForkLineageRecords(ctx, sid, "", 100)
	if err != nil {
		vm.State = "unresolved"
		vm.Reason = "fork_lineage_read_unavailable"
		return vm
	}
	var latestAutomatic *store.ForkLineageRecord
	var legacyConfirmed *store.ForkLineageRecord
	var confirmed *store.ForkLineageRecord
	for index := range records {
		record := records[index]
		if strings.TrimSpace(record.IdempotencyKey) == "" ||
			(record.ContractVersion != store.ForkLineageContractVersion &&
				record.ContractVersion != store.RisuWorldlineForkLineageContractVersion) {
			continue
		}
		if record.LineageState == "confirmed" && record.ContractVersion == store.RisuWorldlineForkLineageContractVersion {
			if _, ok := worldlineInheritedThroughTurn(record.ForkTurn, record.ForkSourceRole); !ok ||
				strings.TrimSpace(record.CopiedFromSessionID) == "" {
				vm.State = "conflict"
				vm.Reason = "confirmed_worldline_tuple_invalid"
				return vm
			}
			if confirmed == nil {
				confirmed = &records[index]
			} else if worldlineConfirmedTupleKey(*confirmed) != worldlineConfirmedTupleKey(record) {
				vm.State = "conflict"
				vm.Reason = "confirmed_worldline_tuple_conflict"
				return vm
			}
			continue
		}
		if record.LineageState == "confirmed" && legacyConfirmed == nil {
			legacyConfirmed = &records[index]
		}
		if latestAutomatic == nil {
			latestAutomatic = &records[index]
		}
	}
	if confirmed != nil {
		return worldlineViewModelFromRecord(*confirmed)
	}
	if legacyConfirmed != nil {
		return worldlineViewModelFromRecord(*legacyConfirmed)
	}
	if latestAutomatic != nil {
		return worldlineViewModelFromRecord(*latestAutomatic)
	}
	return vm
}

func worldlineViewModelFromRecord(record store.ForkLineageRecord) worldlineViewModel {
	state := strings.TrimSpace(record.LineageState)
	if state != "confirmed" && state != "unresolved" && state != "conflict" {
		state = "unresolved"
	}
	reason := "worldline_observation_unresolved"
	if state == "confirmed" {
		reason = "official_branch_marker_validated"
	}
	var detail struct {
		Reason             string `json:"reason"`
		CandidateParentID  string `json:"candidate_parent_session_id"`
		CandidateForkTurns []int  `json:"candidate_fork_turns"`
	}
	if strings.TrimSpace(record.DivergenceMarker) != "" && json.Unmarshal([]byte(record.DivergenceMarker), &detail) == nil {
		if strings.TrimSpace(detail.Reason) != "" {
			reason = strings.TrimSpace(detail.Reason)
		}
	}
	vm := worldlineViewModel{
		ContractVersion:        worldlineViewModelContract,
		State:                  state,
		CurrentSessionID:       strings.TrimSpace(record.ChatSessionID),
		ForkSourceMessageID:    strings.TrimSpace(record.ForkSourceMessageID),
		ForkSourceRole:         strings.TrimSpace(record.ForkSourceRole),
		Reason:                 reason,
		CandidateParentID:      strings.TrimSpace(detail.CandidateParentID),
		CandidateForkTurns:     append([]int(nil), detail.CandidateForkTurns...),
		MessageOriginsRecorded: readRisuWorldlineOrigins(record.InheritedItemsJSON) != nil,
	}
	if state == "confirmed" {
		vm.ParentSessionID = strings.TrimSpace(record.CopiedFromSessionID)
		vm.ForkTurn = record.ForkTurn
		if inheritedThroughTurn, ok := worldlineInheritedThroughTurn(record.ForkTurn, record.ForkSourceRole); ok &&
			record.ContractVersion == store.RisuWorldlineForkLineageContractVersion {
			vm.InheritedThroughTurn = inheritedThroughTurn
		}
	}
	return vm
}

func worldlineInheritedThroughTurn(forkTurn int, sourceRole string) (int, bool) {
	if forkTurn <= 0 {
		return 0, false
	}
	switch strings.TrimSpace(sourceRole) {
	case "char":
		return forkTurn, true
	case "user":
		return maxInt(0, forkTurn-1), true
	default:
		return 0, false
	}
}

func worldlineConfirmedTupleKey(record store.ForkLineageRecord) string {
	return strings.Join([]string{
		strings.TrimSpace(record.CopiedFromSessionID),
		strings.TrimSpace(record.ForkSourceRole),
		fmt.Sprintf("%d", record.ForkTurn),
		strings.TrimSpace(record.ForkSourceMessageID),
	}, "\x1f")
}

type step23ForkLineageTruthBoundary struct {
	SupportOnly            bool   `json:"support_only"`
	CanonicalTruthWriter   bool   `json:"canonical_truth_writer"`
	SilentMergeBackAllowed bool   `json:"silent_merge_back_allowed"`
	HiddenOverwriteAllowed bool   `json:"hidden_overwrite_allowed"`
	DefaultInheritanceMode string `json:"default_inheritance_mode"`
	AutomaticHookAvailable bool   `json:"automatic_hook_available"`
	CloseoutMode           string `json:"closeout_mode"`
}

type step23ForkLineageListResponse struct {
	Status          string                            `json:"status"`
	ContractVersion string                            `json:"contract_version"`
	ChatSessionID   string                            `json:"chat_session_id"`
	ScopeID         string                            `json:"scope_id,omitempty"`
	Records         []step23ForkLineageRecordResponse `json:"records"`
	TruthBoundary   step23ForkLineageTruthBoundary    `json:"truth_boundary"`
}

type step23ForkLineageDeclareRequest struct {
	Operation           string `json:"operation"`
	ChatSessionID       string `json:"chat_session_id"`
	ScopeID             string `json:"scope_id"`
	ParentScopeID       string `json:"parent_scope_id"`
	CopiedFromScopeID   string `json:"copied_from_scope_id"`
	CopiedFromSessionID string `json:"copied_from_session_id"`
	ImportedAt          string `json:"imported_at"`
	DivergenceMarker    string `json:"divergence_marker"`
	ProvenanceSource    string `json:"provenance_source"`
	InheritanceMode     string `json:"inheritance_mode"`
	InheritedItemsJSON  string `json:"inherited_items_json"`
	ForkTurn            int    `json:"fork_turn"`
	ForkSourceMessageID string `json:"fork_source_message_id"`
	ForkSourceRole      string `json:"fork_source_role"`
}

type step23ForkLineageDeclareResponse struct {
	Status          string                          `json:"status"`
	ContractVersion string                          `json:"contract_version"`
	Record          step23ForkLineageRecordResponse `json:"record"`
	TruthBoundary   step23ForkLineageTruthBoundary  `json:"truth_boundary"`
	Worldline       *worldlineViewModel             `json:"worldline,omitempty"`
}

func (s *Server) registerStep23ForkLineageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+step23ForkLineageListRoute, s.handleStep23ForkLineageList)
	mux.HandleFunc("POST "+step23ForkLineageDeclareRoute, s.handleStep23ForkLineageDeclare)
}

func (s *Server) handleStep23ForkLineageList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	scopeID := strings.TrimSpace(r.URL.Query().Get("scope_id"))
	limit := step23ClampInt(step23IntQuery(r.URL.Query().Get("limit"), 100), 1, 1000)

	fs, ok := s.Store.(store.ForkLineageStore)
	if !ok {
		writeJSON(w, http.StatusOK, step23ForkLineageListResponse{
			Status:          "ok",
			ContractVersion: step23ForkLineageContractVersion,
			ChatSessionID:   sid,
			ScopeID:         scopeID,
			Records:         []step23ForkLineageRecordResponse{},
			TruthBoundary:   step23ForkLineageTruthBoundaryValue(),
		})
		return
	}
	records, err := fs.ListForkLineageRecords(r.Context(), sid, scopeID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	resp := step23ForkLineageListResponse{
		Status:          "ok",
		ContractVersion: step23ForkLineageContractVersion,
		ChatSessionID:   sid,
		ScopeID:         scopeID,
		Records:         make([]step23ForkLineageRecordResponse, 0, len(records)),
		TruthBoundary:   step23ForkLineageTruthBoundaryValue(),
	}
	for _, record := range records {
		resp.Records = append(resp.Records, step23ForkLineageFromStore(record))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStep23ForkLineageDeclare(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.Store.(store.ForkLineageStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "fork lineage store is not available")
		return
	}
	var req step23ForkLineageDeclareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if strings.TrimSpace(req.Operation) == "lineage_repair" {
		s.handleStep23ForkLineageRepair(w, r, fs, req)
		return
	}
	if err := step23ValidateForkLineageDeclare(req); err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	importedAt, err := step23ParseOptionalRFC3339(req.ImportedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "imported_at must be RFC3339")
		return
	}
	record := store.ForkLineageRecord{
		ChatSessionID:       strings.TrimSpace(req.ChatSessionID),
		ScopeID:             strings.TrimSpace(req.ScopeID),
		ParentScopeID:       strings.TrimSpace(req.ParentScopeID),
		CopiedFromScopeID:   strings.TrimSpace(req.CopiedFromScopeID),
		CopiedFromSessionID: strings.TrimSpace(req.CopiedFromSessionID),
		ImportedAt:          importedAt,
		DivergenceMarker:    strings.TrimSpace(req.DivergenceMarker),
		ProvenanceSource:    step23NormalizeForkLineageProvenance(req.ProvenanceSource),
		InheritanceMode:     step23NormalizeForkLineageInheritance(req.InheritanceMode),
		InheritedItemsJSON:  strings.TrimSpace(req.InheritedItemsJSON),
	}
	saved, err := fs.SaveForkLineageRecord(r.Context(), record)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, step23ForkLineageDeclareResponse{
		Status:          "ok",
		ContractVersion: step23ForkLineageContractVersion,
		Record:          step23ForkLineageFromStore(saved),
		TruthBoundary:   step23ForkLineageTruthBoundaryValue(),
	})
}

func (s *Server) handleStep23ForkLineageRepair(
	w http.ResponseWriter,
	r *http.Request,
	fs store.ForkLineageStore,
	req step23ForkLineageDeclareRequest,
) {
	childSessionID := strings.TrimSpace(req.ChatSessionID)
	parentSessionID := strings.TrimSpace(req.CopiedFromSessionID)
	sourceMessageID := strings.TrimSpace(req.ForkSourceMessageID)
	sourceRole := strings.TrimSpace(req.ForkSourceRole)
	if childSessionID == "" || parentSessionID == "" || childSessionID == parentSessionID || req.ForkTurn <= 0 ||
		sourceMessageID == "" || (sourceRole != "user" && sourceRole != "char") {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "lineage_repair requires distinct child/parent sessions, a positive fork_turn, fork_source_message_id, and fork_source_role=user|char")
		return
	}
	historyStore, ok := s.Store.(store.SourceRevisionHistoryLister)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "source revision history is not available")
		return
	}
	sources, err := historyStore.ListSourceRevisions(r.Context(), parentSessionID, req.ForkTurn, req.ForkTurn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	turnExists := false
	matchingCharSource := false
	charSourceObserved := false
	for _, source := range sources {
		if source.TurnIndex != req.ForkTurn {
			continue
		}
		turnExists = true
		if sourceRole == "char" && strings.TrimSpace(source.SourceMessageID) != "" {
			charSourceObserved = true
			if strings.TrimSpace(source.SourceMessageID) == sourceMessageID {
				matchingCharSource = true
			}
		}
	}
	if !turnExists || (sourceRole == "char" && charSourceObserved && !matchingCharSource) {
		writeError(w, http.StatusConflict, "fork_lineage_repair_source_mismatch", "selected fork turn does not match the parent session source revision")
		return
	}

	keyHash := sha256.Sum256([]byte(strings.Join([]string{
		childSessionID, parentSessionID, sourceRole, fmt.Sprintf("%d", req.ForkTurn), sourceMessageID,
	}, "\x1f")))
	markerJSON, _ := json.Marshal(map[string]any{
		"reason":                      "manual_branch_lineage_repaired",
		"candidate_parent_session_id": parentSessionID,
		"candidate_fork_turns":        []int{req.ForkTurn},
	})
	record := store.ForkLineageRecord{
		ContractVersion:     store.RisuWorldlineForkLineageContractVersion,
		LineageState:        "confirmed",
		ChatSessionID:       childSessionID,
		CopiedFromSessionID: parentSessionID,
		ForkTurn:            req.ForkTurn,
		ForkSourceMessageID: sourceMessageID,
		ForkSourceRole:      sourceRole,
		IdempotencyKey:      "manual-worldline-repair:" + hex.EncodeToString(keyHash[:]),
		ImportedAt:          time.Now().UTC(),
		DivergenceMarker:    string(markerJSON),
		ProvenanceSource:    "manual",
		InheritanceMode:     "none",
		InheritedItemsJSON:  "[]",
	}
	saved, err := fs.SaveForkLineageRecord(r.Context(), record)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if worldlineConfirmedTupleKey(saved) != worldlineConfirmedTupleKey(record) {
		writeError(w, http.StatusConflict, "fork_lineage_repair_conflict", "an existing confirmed lineage has different coordinates")
		return
	}
	vm := worldlineViewModelFromRecord(saved)
	writeJSON(w, http.StatusCreated, step23ForkLineageDeclareResponse{
		Status:          "ok",
		ContractVersion: step23ForkLineageContractVersion,
		Record:          step23ForkLineageFromStore(saved),
		TruthBoundary:   step23ForkLineageTruthBoundaryValue(),
		Worldline:       &vm,
	})
}

func step23ValidateForkLineageDeclare(req step23ForkLineageDeclareRequest) error {
	if strings.TrimSpace(req.ChatSessionID) == "" {
		return errors.New("chat_session_id is required")
	}
	if strings.TrimSpace(req.ScopeID) == "" {
		return errors.New("scope_id is required")
	}
	if strings.TrimSpace(req.ParentScopeID) == "" && strings.TrimSpace(req.CopiedFromScopeID) == "" && strings.TrimSpace(req.CopiedFromSessionID) == "" {
		return errors.New("parent_scope_id, copied_from_scope_id, or copied_from_session_id is required")
	}
	if strings.TrimSpace(req.ScopeID) == strings.TrimSpace(req.ParentScopeID) || strings.TrimSpace(req.ScopeID) == strings.TrimSpace(req.CopiedFromScopeID) {
		return errors.New("scope_id must differ from parent/copied scope")
	}
	if strings.TrimSpace(req.InheritedItemsJSON) == "" {
		return errors.New("inherited_items_json is required, use [] when nothing was imported")
	}
	if step23NormalizeForkLineageProvenance(req.ProvenanceSource) == "automatic_hook" {
		return errors.New("automatic_hook provenance is reserved for automatic host observation")
	}
	return nil
}

func step23ForkLineageTruthBoundaryValue() step23ForkLineageTruthBoundary {
	return step23ForkLineageTruthBoundary{
		SupportOnly:            true,
		CanonicalTruthWriter:   false,
		SilentMergeBackAllowed: false,
		HiddenOverwriteAllowed: false,
		DefaultInheritanceMode: "conservative_import",
		AutomaticHookAvailable: true,
		CloseoutMode:           "manual_or_validated_official_risu_observation",
	}
}

func step23NormalizeForkLineageProvenance(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "manual", "operator_manual", "user_manual":
		return "manual"
	case "automatic_hook":
		return "automatic_hook"
	default:
		return "manual"
	}
}

func step23NormalizeForkLineageInheritance(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "conservative_import", "review_safe_support_only":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "conservative_import"
	}
}

func step23ParseOptionalRFC3339(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now().UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func step23ForkLineageFromStore(record store.ForkLineageRecord) step23ForkLineageRecordResponse {
	response := step23ForkLineageRecordResponse{
		ID:                  record.ID,
		ContractVersion:     record.ContractVersion,
		LineageState:        record.LineageState,
		ChatSessionID:       record.ChatSessionID,
		ScopeID:             record.ScopeID,
		ParentScopeID:       record.ParentScopeID,
		CopiedFromScopeID:   record.CopiedFromScopeID,
		CopiedFromSessionID: record.CopiedFromSessionID,
		ForkTurn:            record.ForkTurn,
		ForkSourceMessageID: record.ForkSourceMessageID,
		ForkSourceRole:      record.ForkSourceRole,
		ImportedAt:          record.ImportedAt,
		DivergenceMarker:    record.DivergenceMarker,
		ProvenanceSource:    record.ProvenanceSource,
		InheritanceMode:     record.InheritanceMode,
		InheritedItemsJSON:  record.InheritedItemsJSON,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
	}
	if inheritedThroughTurn, ok := worldlineInheritedThroughTurn(record.ForkTurn, record.ForkSourceRole); ok &&
		record.ContractVersion == store.RisuWorldlineForkLineageContractVersion {
		response.InheritedThroughTurn = inheritedThroughTurn
	}
	return response
}
