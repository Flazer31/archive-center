package httpapi

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test44InputGroupPrepareRoutingAndPreviousContext(t *testing.T) {
	for _, count := range []int{1, 2, 3} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			const completed = 27
			active := []dto.PrepareTurnMessageObservationV1{}
			add := func(role, text, id string) {
				index := len(active)
				hash, algorithm := prepareOR1CHash(text), "or1c_utf16_djb2.v1"
				active = append(active, dto.PrepareTurnMessageObservationV1{ObservationRef: fmt.Sprintf("active:%d", index), SourceKind: "active_chat", ObservationStage: "active_chat_stored_message", MessageIndex: &index, MessageID: &id, Role: &role, RawContent: &text, ContentHash: &hash, HashAlgorithm: &algorithm, EvidenceState: "observed"})
			}
			for i := 0; i < completed; i++ {
				add("user", fmt.Sprint("input-", i), fmt.Sprint("u-", i))
				add("assistant", fmt.Sprint("answer-", i), fmt.Sprint("a-", i))
			}
			parts := []string{}
			for i := 0; i < count; i++ {
				text := fmt.Sprint("current-", i)
				parts = append(parts, text)
				add("user", text, text)
			}
			req := dto.PrepareTurnContractRequest{HostObservations: &dto.PrepareTurnHostObservationsV1{ContractVersion: "prepare_host_observations.v1", SessionID: "group", RequestID: "request", RequestType: "model", PayloadWritable: true, ActiveChat: active, Payload: active}}
			decision, enforced := buildPrepareTurnCurrentInputDecision(req, "group")
			if !enforced || decision.EffectiveUserInput != strings.Join(parts, "\n\n") {
				t.Fatalf("input lost: %+v", decision)
			}
			server := newCompleteTurnAcceptanceTestServer()
			turn, _ := server.resolvePrepareTurnCurrentLogicalTurn(context.Background(), req, decision, "group")
			if turn != completed+1 {
				t.Fatalf("inputs=%d turn=%d expected=%d", count, turn, completed+1)
			}
			index := len(active) - 1
			routed := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "pair", RisuUserMessageIndex: &index, ObservedPairOrdinal: completed + 1, ObservedInputGroupOrdinal: completed + 1})
			if routed.TurnIndex != turn {
				t.Fatalf("prepare/reserve mismatch %d vs %+v", turn, routed)
			}
			// The physical last-user index remains untouched; parity alone is the old defect.
			if count == 3 && index/2+1 == turn {
				t.Fatal("fixture does not detect the old parity bug")
			}
			add("assistant", "group answer", "current-answer")
			add("user", "next input", "next-user")
			req.HostObservations.ActiveChat, req.HostObservations.Payload = active, active
			nextDecision, _ := buildPrepareTurnCurrentInputDecision(req, "group")
			logs, _ := prepareTurnInputContextChatLogs(req, nextDecision, nil)
			if len(logs) != 2 || logs[0].Content != strings.Join(parts, "\n\n") || logs[1].Content != "group answer" {
				t.Fatalf("previous input group lost: %+v", logs)
			}
		})
	}
}

func groupAcceptanceRequest(ids, texts []string, turn int, at int64, answer string) dto.M4CompleteTurnRequest {
	req := completeTurnAfterRequestAcceptanceTestRequest("group", turn, strings.Join(texts, "\n\n"), answer, at, fmt.Sprint("request-", at))
	obs := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	refs := []completeTurnUserMessageReference{}
	for i, id := range ids {
		refs = append(refs, completeTurnUserMessageReference{MessageIndex: i, MessageChatID: id, ContentHash: prepareOR1CHash(texts[i])})
	}
	obs["user_message_refs"] = refs
	obs["user_message_chat_id"] = ids[len(ids)-1]
	obs["user_message_index"] = len(ids) - 1
	obs["user_observed_pair_ordinal"] = turn
	return req
}

func Test44InputGroupAcceptanceRetryEditDeleteRestartAndNewRow(t *testing.T) {
	storage := &durableSessionIdentityBindingStore{Store: &memoryFakeStore{}, sources: map[string][]store.MemorySourceRevision{}}
	makeServer := func() *Server {
		return &Server{Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}, Store: storage, SourceAcceptances: newCompleteTurnSourceAcceptanceLedger()}
	}
	server := makeServer()
	firstReq := groupAcceptanceRequest([]string{"A", "B"}, []string{"first", "last"}, 1, 1000, "answer")
	first := server.beginCompleteTurnSourceAcceptance(context.Background(), firstReq)
	if !first.Accepted || first.ReplaceExisting {
		t.Fatalf("normal: %+v", first)
	}
	retry := server.beginCompleteTurnSourceAcceptance(context.Background(), firstReq)
	if !retry.Accepted || retry.Revision != first.Revision || retry.BoundTurn != first.BoundTurn {
		t.Fatalf("retry: %+v", retry)
	}
	canonical := func(d completeTurnSourceAcceptanceDecision, at int64) {
		storage.sources["group"] = []store.MemorySourceRevision{{ChatSessionID: "group", TurnIndex: d.BoundTurn, SourceRevision: d.Revision, LogicalTurnID: d.LogicalTurnID, HostObservedAtMS: at, LifecycleState: "active"}}
	}
	canonical(first, 1000)
	cases := []struct {
		name       string
		ids, texts []string
	}{
		{"same rows reroll", []string{"A", "B"}, []string{"first", "last"}},
		{"first input edited after answer removal", []string{"A", "B"}, []string{"first edited", "last"}},
		{"last input deleted", []string{"A"}, []string{"first edited"}},
		{"restore B and remove A", []string{"B"}, []string{"last"}},
	}
	for i, tc := range cases {
		// Recreate the in-memory ledger every time: aliases must survive via the real audit serializer.
		server = makeServer()
		req := groupAcceptanceRequest(tc.ids, tc.texts, first.BoundTurn+1, int64(2000+i*1000), fmt.Sprint("rerolled-", i))
		got := server.beginCompleteTurnSourceAcceptance(context.Background(), req)
		if !got.Accepted || !got.ReplaceExisting || got.BoundTurn != first.BoundTurn || got.LogicalTurnID != first.LogicalTurnID {
			t.Fatalf("%s: %+v", tc.name, got)
		}
		canonical(got, int64(2000+i*1000))
	}
	server = makeServer()
	next := server.beginCompleteTurnSourceAcceptance(context.Background(), groupAcceptanceRequest([]string{"new-B"}, []string{"last"}, first.BoundTurn+1, 9000, "new answer"))
	if !next.Accepted || next.ReplaceExisting || next.BoundTurn != first.BoundTurn+1 || next.LogicalTurnID == first.LogicalTurnID {
		t.Fatalf("new row identical text: %+v", next)
	}
}
