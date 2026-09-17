package httpapi

import (
	"context"
	"fmt"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func originBranchRequest(parent, child string, parents []risuWorldlineMessageObservation, reissue bool) sessionRoutingTurnResolutionRequest {
	children := append([]risuWorldlineMessageObservation(nil), parents...)
	for i := range children {
		if reissue {
			children[i].MessageChatID = fmt.Sprintf("%s-%d", child, i)
		}
	}
	end := parents[len(parents)-1]
	bounded := []risuWorldlineMessageObservation{children[len(children)-1], {MessageIndex: len(children), Role: "char", Disabled: true}}
	if end.Role == "char" {
		for i := len(children) - 2; i >= 0; i-- {
			if children[i].Role == "user" && !children[i].Disabled {
				bounded = append(bounded, children[i])
				break
			}
		}
	}
	return sessionRoutingTurnResolutionRequest{
		StableCharacterID: "stable", HostChatID: child, HostChatIDState: "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion: risuWorldlineObservationContract, HostSignalSource: "output",
			BranchShapeContract: risuBranchShapeContract, ObservedAtMS: 1_776_000_000_000,
			MarkerState: "observed", BranchMarker: "{{specialcomment::branchedfrom::" + parent + "::Parent::" + end.MessageChatID + "::}}",
			MarkerIndex: len(children), Messages: bounded,
			MessageOrigins: &risuWorldlineOriginObservation{ContractVersion: risuMessageOriginsContract, ParentHostChatID: parent, ParentMessages: parents, ChildMessages: children},
		},
	}
}

func TestWorldline43PreservedAndReissuedIDsShareParentAuthority(t *testing.T) {
	for _, reissue := range []bool{false, true} {
		for _, role := range []string{"user", "char"} {
			t.Run(fmt.Sprintf("reissue=%t/%s", reissue, role), func(t *testing.T) {
				parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "parent-user"}}
				if role == "char" {
					parents = append(parents, risuWorldlineMessageObservation{MessageIndex: 1, Role: "char", MessageChatID: "parent-assistant"})
				}
				source := activeSourceRevisionForUserAnchor("A", "parent", "parent-user", "", 7) // normal complete-turn omits assistant ID
				st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{"stable\x00parent": "A"}, sources: map[string][]store.MemorySourceRevision{"A": {source}}}
				server := &Server{Store: st}
				req := originBranchRequest("parent", "child", parents, reissue)
				metadata := req.WorldlineObservation.MessageOrigins
				req.WorldlineObservation.MessageOrigins = nil
				first := server.resolveRisuWorldlineObservation(context.Background(), req, "B")
				if first.Reason == "fork_source_message_conflict" || first.OriginReadRequest == nil {
					t.Fatalf("first=%+v", first)
				}
				req.WorldlineObservation.MessageOrigins = metadata
				vm := server.resolveRisuWorldlineObservation(context.Background(), req, "B")
				wantThrough := source.TurnIndex
				if role == "user" {
					wantThrough--
				}
				if vm.State != "confirmed" || vm.ParentSessionID != "A" || vm.ForkTurn != source.TurnIndex || vm.InheritedThroughTurn != wantThrough || !vm.MessageOriginsRecorded || vm.OriginReadRequest != nil {
					t.Fatalf("vm=%+v", vm)
				}
				if vm.ForkSourceMessageID != parents[len(parents)-1].MessageChatID {
					t.Fatalf("parent ID replaced by child's ID: %+v", vm)
				}
				// Reconstruct the server with durable rows only; no host snapshot or cache.
				restarted := &Server{Store: st}
				st.sources = nil
				req.WorldlineObservation.MessageOrigins = nil
				req.WorldlineObservation.BranchMarker = ""
				got := restarted.resolveRisuWorldlineObservation(context.Background(), req, "B")
				if got.State != "confirmed" || !got.MessageOriginsRecorded || got.ForkTurn != vm.ForkTurn {
					t.Fatalf("restart=%+v", got)
				}
			})
		}
	}
}

func TestWorldline43NestedReissuedBranchUsesStoredOriginsAndEarlierCut(t *testing.T) {
	for _, role := range []string{"user", "char"} {
		t.Run(role, func(t *testing.T) {
			parents := []risuWorldlineMessageObservation{}
			sources := []store.MemorySourceRevision{}
			for turn := 1; turn <= 4; turn++ {
				u, a := fmt.Sprintf("u%d", turn), fmt.Sprintf("a%d", turn)
				parents = append(parents, risuWorldlineMessageObservation{MessageIndex: len(parents), Role: "user", MessageChatID: u}, risuWorldlineMessageObservation{MessageIndex: len(parents) + 1, Role: "char", MessageChatID: a})
				sources = append(sources, activeSourceRevisionForUserAnchor("A", "root", u, "", turn))
			}
			st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{"stable\x00root": "A", "stable\x00branch": "B"}, sources: map[string][]store.MemorySourceRevision{"A": sources}, history: map[string][]store.MemorySourceRevision{"A": sources}}
			server := &Server{Store: st}
			parentReq := originBranchRequest("root", "branch", parents[:6], true)
			if vm := server.resolveRisuWorldlineObservation(context.Background(), parentReq, "B"); vm.State != "confirmed" {
				t.Fatalf("parent=%+v", vm)
			}
			end := 4
			if role == "user" {
				end--
			}
			childReq := originBranchRequest("branch", "grandchild", parentReq.WorldlineObservation.MessageOrigins.ChildMessages[:end], true)
			vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), childReq, "C")
			want := sources[1].TurnIndex
			if vm.State != "confirmed" || vm.ParentSessionID != "B" || vm.ForkTurn != want {
				t.Fatalf("child=%+v", vm)
			}
			if role == "user" {
				want--
			}
			scope := resolvePrepareTurnHistoryScope(context.Background(), st, "C", 0)
			for _, segment := range scope.Segments {
				if segment.SessionID == "B" {
					t.Fatalf("parent-owned future exposed: %+v", scope)
				}
				if segment.SessionID == "A" && segment.ToTurn != want {
					t.Fatalf("grandparent fence=%+v want=%d", scope, want)
				}
			}
		})
	}
}

func TestWorldline43NestedUserEndpointAndEmptyInheritedScope(t *testing.T) {
	parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "u"}}
	source := activeSourceRevisionForUserAnchor("A", "root", "u", "", 1)
	st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{"stable\x00root": "A", "stable\x00branch": "B"}, sources: map[string][]store.MemorySourceRevision{"A": {source}}, history: map[string][]store.MemorySourceRevision{"A": {source}}}
	server := &Server{Store: st}
	first := originBranchRequest("root", "branch", parents, true)
	server.resolveRisuWorldlineObservation(context.Background(), first, "B")
	second := originBranchRequest("branch", "child", first.WorldlineObservation.MessageOrigins.ChildMessages, true)
	vm := server.resolveRisuWorldlineObservation(context.Background(), second, "C")
	if vm.State != "confirmed" || vm.ForkTurn != source.TurnIndex || vm.InheritedThroughTurn != 0 {
		t.Fatalf("vm=%+v", vm)
	}
	scope := resolvePrepareTurnHistoryScope(context.Background(), st, "C", 0)
	if len(scope.Segments) != 1 || scope.Segments[0].SessionID != "C" {
		t.Fatalf("empty ancestor scope=%+v", scope)
	}
}

func TestWorldline43ShiftedPrefixDoesNotInventInteriorOrigins(t *testing.T) {
	parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "u"}, {MessageIndex: 1, Role: "char", MessageChatID: "a"}}
	req := originBranchRequest("root", "child", parents, true)
	req.WorldlineObservation.MarkerIndex = 3
	req.WorldlineObservation.MessageOrigins.ChildMessages[1].MessageIndex = 2
	origins := readRisuWorldlineOrigins(risuWorldlineOriginItems(req.WorldlineObservation))
	if origins == nil || len(origins.Items) != 1 || origins.Items[0].ParentMessageID != "a" {
		t.Fatalf("origins=%+v", origins)
	}
}

func TestWorldline43ExistingNestedBranchEnrichesParentWithoutOpeningIt(t *testing.T) {
	parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "u"}, {MessageIndex: 1, Role: "char", MessageChatID: "a"}}
	source := activeSourceRevisionForUserAnchor("A", "root", "u", "", 1)
	st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{"stable\x00root": "A", "stable\x00branch": "B"}, sources: map[string][]store.MemorySourceRevision{"A": {source}}, history: map[string][]store.MemorySourceRevision{"A": {source}}}
	server := &Server{Store: st}
	parentReq := originBranchRequest("root", "branch", parents, true)
	server.resolveRisuWorldlineObservation(context.Background(), parentReq, "B")
	st.lineage[0].InheritedItemsJSON = "[]" // confirmed before this feature (or manual repair)
	parentMetadata := parentReq.WorldlineObservation.MessageOrigins
	childReq := originBranchRequest("branch", "child", parentMetadata.ChildMessages, true)
	parentReq.WorldlineObservation.MessageOrigins = nil
	childReq.WorldlineObservation.MessageOrigins.ParentObservation = parentReq.WorldlineObservation
	first := server.resolveRisuWorldlineObservation(context.Background(), childReq, "C")
	if first.OriginReadRequest == nil || first.OriginReadRequest.AncestorDepth != 1 || first.OriginReadRequest.ChildHostChatID != "branch" || first.OriginReadRequest.ParentHostChatID != "root" {
		t.Fatalf("ancestor read=%+v", first)
	}
	childReq.WorldlineObservation.MessageOrigins.ParentObservation.MessageOrigins = parentMetadata
	vm := server.resolveRisuWorldlineObservation(context.Background(), childReq, "C")
	if vm.State != "confirmed" || vm.ParentSessionID != "B" || vm.ForkTurn != source.TurnIndex || vm.OriginReadRequest != nil {
		t.Fatalf("enriched=%+v", vm)
	}
	if readRisuWorldlineOrigins(st.lineage[0].InheritedItemsJSON) == nil {
		t.Fatal("parent origin metadata was not persisted")
	}
}
