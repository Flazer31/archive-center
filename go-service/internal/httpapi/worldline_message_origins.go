package httpapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const risuMessageOriginsContract = "risu_message_origins.v1"

type risuWorldlineOriginReadRequest struct {
	ParentHostChatID string `json:"parent_host_chat_id"`
	SourceMessageID  string `json:"source_message_id"`
	ChildHostChatID  string `json:"child_host_chat_id"`
	ChildMarkerIndex int    `json:"child_marker_index"`
	AncestorDepth    int    `json:"ancestor_depth,omitempty"`
}

// IDs and positions are host observations; no message text is requested.
type risuWorldlineOriginObservation struct {
	ContractVersion   string                            `json:"contract_version"`
	ParentHostChatID  string                            `json:"parent_host_chat_id"`
	ParentMessages    []risuWorldlineMessageObservation `json:"parent_messages"`
	ChildMessages     []risuWorldlineMessageObservation `json:"child_messages"`
	ParentObservation *risuWorldlineObservation         `json:"parent_observation,omitempty"`
}

// Reuse the same lineage owner to enrich a parent that predates origin recording.
func (s *Server) resolveRisuWorldlineParentObservation(ctx context.Context, req sessionRoutingTurnResolutionRequest) *risuWorldlineOriginReadRequest {
	observation := req.WorldlineObservation
	if observation == nil || observation.MessageOrigins == nil || observation.MessageOrigins.ParentObservation == nil || req.worldlineOriginDepth >= prepareTurnHistoryMaxDepth-1 {
		return nil
	}
	parentHostID, _, _, ok := parseExactRisuBranchMarker(observation.BranchMarker)
	if !ok {
		return nil
	}
	bindings, ok := s.Store.(store.SessionRouteBindingStore)
	if !ok {
		return nil
	}
	parent, err := bindings.BindSessionRoute(ctx, store.SessionRouteBindingRequest{StableCharacterID: req.StableCharacterID, HostChatID: parentHostID, Mode: store.SessionRouteBindingModeResolveExisting})
	if err != nil || parent == nil {
		return nil
	}
	parentReq := req
	parentReq.HostChatID = parentHostID
	parentReq.WorldlineObservation = observation.MessageOrigins.ParentObservation
	parentReq.worldlineOriginDepth++
	vm := s.resolveRisuWorldlineObservation(ctx, parentReq, parent.Binding.CanonicalSessionID)
	if vm.OriginReadRequest != nil {
		plan := *vm.OriginReadRequest
		plan.AncestorDepth++
		return &plan
	}
	return nil
}

type risuMessageOrigin struct {
	ChildMessageID  string `json:"child_message_id"`
	ParentMessageID string `json:"parent_message_id"`
	Role            string `json:"role"`
}

type risuMessageOrigins struct {
	ContractVersion  string              `json:"contract_version"`
	ParentHostChatID string              `json:"parent_host_chat_id"`
	Items            []risuMessageOrigin `json:"items"`
}

func risuWorldlineParentMessages(observation *risuWorldlineObservation, parentID, sourceID string) []risuWorldlineMessageObservation {
	if observation == nil || observation.MessageOrigins == nil {
		return nil
	}
	origins := observation.MessageOrigins
	if origins.ContractVersion != risuMessageOriginsContract || origins.ParentHostChatID != parentID {
		return nil
	}
	for index, message := range origins.ParentMessages {
		if message.MessageChatID == sourceID {
			return origins.ParentMessages[:index+1]
		}
	}
	return nil
}

func risuWorldlineParentUserAnchor(observation *risuWorldlineObservation, parentID, sourceID string) string {
	messages := risuWorldlineParentMessages(observation, parentID, sourceID)
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.Role == "user" && !message.Disabled {
			return message.MessageChatID
		}
	}
	return ""
}

// A branch copies a Host prefix even when the parent's last complete-turn call
// has not run yet. Resolve that prefix's turn coordinate without admitting the
// response, running a Critic, or changing next-input finalization ownership.
func (s *Server) risuWorldlineObservedSourceTurn(ctx context.Context, observation *risuWorldlineObservation, parentID, parentHostID, sourceID string, sources []store.MemorySourceRevision) int {
	messages := risuWorldlineParentMessages(observation, parentHostID, sourceID)
	if len(messages) == 0 {
		return 0
	}
	type position struct {
		message risuWorldlineMessageObservation
		ordinal int
		logical string
	}
	positions := make([]position, 0, len(messages))
	completed, userID := 0, ""
	for _, message := range messages {
		if message.Disabled {
			continue
		}
		ordinal := completed + 1
		switch message.Role {
		case "user":
			userID = message.MessageChatID
		case "char":
			if userID != "" {
				completed++
			}
			ordinal = completed // the leading character greeting remains turn zero
		default:
			continue
		}
		logical := completeTurnLogicalTurnID(parentID, completeTurnSourceObservation{
			HostChatID: parentHostID, HostChatIDState: "observed", UserMessageChatID: userID, UserMessageChatIDState: "observed",
		})
		positions = append(positions, position{message, ordinal, logical})
	}
	if len(positions) == 0 || positions[len(positions)-1].message.MessageChatID != sourceID {
		return 0
	}
	target := positions[len(positions)-1].ordinal
	byMessage, byLogical := map[string][]store.MemorySourceRevision{}, map[string][]store.MemorySourceRevision{}
	for _, source := range sources {
		byMessage[source.SourceMessageID] = append(byMessage[source.SourceMessageID], source)
		byLogical[source.LogicalTurnID] = append(byLogical[source.LogicalTurnID], source)
	}
	// Stored coordinates account for existing offsets and earlier deleted rows.
	// Only positions inside the observed prefix participate; parent future rows
	// cannot supply an anchor or extend the child's cut.
	for index := len(positions) - 1; index >= 0; index-- {
		p := positions[index]
		var candidates []store.MemorySourceRevision
		if p.message.Role == "char" && p.message.MessageChatID != "" {
			candidates = byMessage[p.message.MessageChatID]
		}
		if len(candidates) == 0 {
			candidates = byLogical[p.logical]
		}
		if turns := uniqueWorldlineSourceTurns(candidates); len(turns) == 1 {
			return turns[0] + target - p.ordinal
		}
	}
	// A parent's first owned response can be pending. Its existing inherited
	// endpoint still establishes the coordinate, including reissued child IDs.
	parent := currentWorldlineViewModel(ctx, s.Store, parentID)
	if parent.State == "confirmed" {
		if fs, ok := s.Store.(store.ForkLineageStore); ok {
			records, _ := fs.ListForkLineageRecords(ctx, parentID, "", 0)
			for _, record := range records {
				origins := readRisuWorldlineOrigins(record.InheritedItemsJSON)
				if origins == nil || record.LineageState != "confirmed" {
					continue
				}
				for _, origin := range origins.Items {
					if origin.ParentMessageID != parent.ForkSourceMessageID {
						continue
					}
					for _, p := range positions {
						if p.message.MessageChatID == origin.ChildMessageID {
							return parent.ForkTurn + target - p.ordinal
						}
					}
				}
			}
		}
	}
	resolved := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
		Mode: "pair", ObservedPairOrdinal: target,
		Baseline: s.resolveDurableSessionRoutingBaseline(ctx, parentID, nil),
	})
	return resolved.TurnIndex
}

func risuWorldlineOriginItems(observation *risuWorldlineObservation) string {
	if observation == nil || observation.MessageOrigins == nil {
		return ""
	}
	parentID, _, sourceID, ok := parseExactRisuBranchMarker(observation.BranchMarker)
	if !ok {
		return ""
	}
	parents := risuWorldlineParentMessages(observation, parentID, sourceID)
	children := observation.MessageOrigins.ChildMessages
	if len(parents) == 0 || len(children) == 0 {
		return ""
	}
	origins := risuMessageOrigins{ContractVersion: risuMessageOriginsContract, ParentHostChatID: parentID}
	// Both official branch commands copy an inclusive, ordered prefix. Where
	// that layout is no longer observable, retain only preserved-ID links and
	// the marker's explicit endpoint; never align shifted rows by their text.
	byID := make(map[string]risuWorldlineMessageObservation, len(parents))
	for _, parent := range parents {
		byID[parent.MessageChatID] = parent
	}
	prefixLayout := parents[len(parents)-1].MessageIndex == observation.MarkerIndex-1
	for _, child := range children {
		if child.Disabled || child.MessageChatID == "" {
			continue
		}
		parent, found := byID[child.MessageChatID]
		if !found && prefixLayout && child.MessageIndex >= 0 && child.MessageIndex < len(parents) {
			parent, found = parents[child.MessageIndex], true
		}
		if child.MessageIndex == observation.MarkerIndex-1 {
			parent, found = parents[len(parents)-1], true
		}
		if found && parent.MessageChatID != "" && parent.Role == child.Role && !parent.Disabled {
			origins.Items = append(origins.Items, risuMessageOrigin{child.MessageChatID, parent.MessageChatID, child.Role})
		}
	}
	if len(origins.Items) == 0 {
		return ""
	}
	encoded, _ := json.Marshal([]risuMessageOrigins{origins})
	return string(encoded)
}

func readRisuWorldlineOrigins(raw string) *risuMessageOrigins {
	var items []json.RawMessage
	if json.Unmarshal([]byte(raw), &items) != nil {
		return nil
	}
	for _, item := range items {
		var origins risuMessageOrigins
		if json.Unmarshal(item, &origins) == nil && origins.ContractVersion == risuMessageOriginsContract {
			return &origins
		}
	}
	return nil
}

func risuOriginMessageID(origins *risuMessageOrigins, childID, role string) string {
	if origins != nil {
		for _, item := range origins.Items {
			if item.ChildMessageID == childID && item.Role == role {
				return item.ParentMessageID
			}
		}
	}
	return ""
}

func (s *Server) enrichRisuWorldlineOrigins(ctx context.Context, sid, characterID string, observation *risuWorldlineObservation) {
	fs, ok := s.Store.(store.ForkLineageStore)
	if !ok {
		return
	}
	records, err := fs.ListForkLineageRecords(ctx, sid, "", 0)
	if err != nil {
		return
	}
	parentHostID, _, sourceID, _ := parseExactRisuBranchMarker(observation.BranchMarker)
	bindings, ok := s.Store.(store.SessionRouteBindingStore)
	if !ok {
		return
	}
	parent, err := bindings.BindSessionRoute(ctx, store.SessionRouteBindingRequest{StableCharacterID: characterID, HostChatID: parentHostID, Mode: store.SessionRouteBindingModeResolveExisting})
	if err != nil || parent == nil {
		return
	}
	for _, record := range records {
		if record.LineageState != "confirmed" || record.ForkSourceMessageID != sourceID || record.CopiedFromSessionID != parent.Binding.CanonicalSessionID {
			continue
		}
		if raw := strings.TrimSpace(record.InheritedItemsJSON); raw != "" && raw != "[]" {
			return
		}
		record.InheritedItemsJSON = risuWorldlineOriginItems(observation)
		if record.InheritedItemsJSON != "" {
			_, _ = fs.SaveForkLineageRecord(ctx, record)
		}
		return
	}
}

// Traverse only the already-confirmed parent chain. The immediate parent stays
// the child's parent even when the canonical source belongs to a grandparent.
func (s *Server) risuWorldlineInheritedSourceTurns(ctx context.Context, sid, hostID, messageID, userID, role string) []int {
	fs, ok := s.Store.(store.ForkLineageStore)
	if !ok {
		return nil
	}
	history, ok := s.Store.(store.SourceRevisionHistoryLister)
	if !ok {
		return nil
	}
	upperTurn := 0
	for depth := 0; depth < prepareTurnHistoryMaxDepth; depth++ {
		vm := currentWorldlineViewModel(ctx, s.Store, sid)
		if vm.State != "confirmed" {
			return nil
		}
		records, err := fs.ListForkLineageRecords(ctx, sid, "", 0)
		if err != nil {
			return nil
		}
		var origins *risuMessageOrigins
		for _, record := range records {
			if record.LineageState == "confirmed" && record.CopiedFromSessionID == vm.ParentSessionID {
				origins = readRisuWorldlineOrigins(record.InheritedItemsJSON)
				break
			}
		}
		if origins != nil {
			mapped := risuOriginMessageID(origins, messageID, role)
			if mapped == "" {
				return nil
			}
			messageID = mapped
			if mappedUser := risuOriginMessageID(origins, userID, "user"); mappedUser != "" {
				userID = mappedUser
			}
			hostID = origins.ParentHostChatID
		} else {
			return nil
		}
		// The copied user endpoint belongs to ForkTurn, although its parent's
		// assistant at that turn is excluded from inherited recall.
		if upperTurn == 0 || vm.ForkTurn < upperTurn {
			upperTurn = vm.ForkTurn
		}
		sid = vm.ParentSessionID
		sources, err := history.ListSourceRevisions(ctx, sid, 0, upperTurn)
		if err != nil {
			return nil
		}
		logicalID := completeTurnLogicalTurnID(sid, completeTurnSourceObservation{
			HostChatID: hostID, HostChatIDState: "observed", UserMessageChatID: userID, UserMessageChatIDState: "observed",
		})
		matches := prioritizedWorldlineSourceTurns(sources, logicalID, role, messageID)
		if len(matches) > 0 {
			return matches
		}
	}
	return nil
}
