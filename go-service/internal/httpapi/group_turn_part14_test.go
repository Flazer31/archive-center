package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type prepareTurnWorldlineHistoryStore struct {
	*turnRecordingStore
	lineageRecords    []store.ForkLineageRecord
	memoriesBySession map[string][]store.Memory
	evidenceBySession map[string][]store.DirectEvidence
	kgBySession       map[string][]store.KGTriple
	chatBySession     map[string][]store.ChatLog
	memoryReads       []prepareTurnHistorySegment
}

func (s *prepareTurnWorldlineHistoryStore) ListForkLineageRecords(_ context.Context, chatSessionID, scopeID string, limit int) ([]store.ForkLineageRecord, error) {
	rows := []store.ForkLineageRecord{}
	for _, row := range s.lineageRecords {
		if row.ChatSessionID == chatSessionID && (scopeID == "" || row.ScopeID == scopeID) {
			rows = append(rows, row)
		}
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (s *prepareTurnWorldlineHistoryStore) SaveForkLineageRecord(_ context.Context, record store.ForkLineageRecord) (store.ForkLineageRecord, error) {
	s.lineageRecords = append(s.lineageRecords, record)
	return record, nil
}

func (s *prepareTurnWorldlineHistoryStore) LatestSessionTurnIndex(_ context.Context, chatSessionID string) (int, error) {
	latest := 0
	for _, row := range s.chatBySession[chatSessionID] {
		latest = maxInt(latest, row.TurnIndex)
	}
	return latest, nil
}

func (s *prepareTurnWorldlineHistoryStore) ListMemoriesRange(_ context.Context, chatSessionID string, fromTurn, toTurn int, _ []int64) ([]store.Memory, error) {
	s.memoryReads = append(s.memoryReads, prepareTurnHistorySegment{SessionID: chatSessionID, FromTurn: fromTurn, ToTurn: toTurn})
	return append([]store.Memory(nil), s.memoriesBySession[chatSessionID]...), nil
}

func (s *prepareTurnWorldlineHistoryStore) ListEvidenceRange(_ context.Context, chatSessionID string, _, _ int, _ []int64) ([]store.DirectEvidence, error) {
	return append([]store.DirectEvidence(nil), s.evidenceBySession[chatSessionID]...), nil
}

func (s *prepareTurnWorldlineHistoryStore) ListKGTriplesRange(_ context.Context, chatSessionID string, _, _ int) ([]store.KGTriple, error) {
	return append([]store.KGTriple(nil), s.kgBySession[chatSessionID]...), nil
}

func (s *prepareTurnWorldlineHistoryStore) ListChatLogs(_ context.Context, chatSessionID string, _, _ int) ([]store.ChatLog, error) {
	return append([]store.ChatLog(nil), s.chatBySession[chatSessionID]...), nil
}

func (s *prepareTurnWorldlineHistoryStore) ListCharacterStatesCurrentBefore(context.Context, string, int) ([]store.CharacterState, error) {
	return nil, nil
}

func (s *prepareTurnWorldlineHistoryStore) ListActiveStatesRange(context.Context, string, int, int) ([]store.ActiveState, error) {
	return nil, nil
}

func (s *prepareTurnWorldlineHistoryStore) ListCanonicalStateLayersRange(context.Context, string, int, int) ([]store.CanonicalStateLayer, error) {
	return nil, nil
}

func TestPrepareTurnReadsOnlyConfirmedWorldlineOwnedHistory(t *testing.T) {
	fake := &prepareTurnWorldlineHistoryStore{
		turnRecordingStore: &turnRecordingStore{},
		lineageRecords: []store.ForkLineageRecord{
			confirmedWorldlineTopologyRecord("branch-1", "root", 8, "char", "root-turn-8"),
			confirmedWorldlineTopologyRecord("branch-2", "branch-1", 9, "char", "branch-1-turn-9"),
		},
		memoriesBySession: map[string][]store.Memory{
			"root": {
				{ID: 1, ChatSessionID: "root", TurnIndex: 4, SummaryJSON: `{"turn_summary":"ROOT_INHERITED_MEMORY brass observatory oath"}`, Importance: 10},
				{ID: 2, ChatSessionID: "root", TurnIndex: 10, SummaryJSON: `{"turn_summary":"ROOT_POST_FORK_MUST_NOT_LEAK"}`, Importance: 10},
			},
			"branch-1": {
				{ID: 3, ChatSessionID: "branch-1", TurnIndex: 8, SummaryJSON: `{"turn_summary":"BRANCH_COPIED_PREFIX_MUST_NOT_DUPLICATE"}`, Importance: 10},
				{ID: 4, ChatSessionID: "branch-1", TurnIndex: 9, SummaryJSON: `{"turn_summary":"BRANCH_ONE_OWNED_MEMORY brass observatory key"}`, Importance: 10},
			},
			"branch-2": {
				{ID: 5, ChatSessionID: "branch-2", TurnIndex: 9, SummaryJSON: `{"turn_summary":"NESTED_COPIED_PREFIX_MUST_NOT_DUPLICATE"}`, Importance: 10},
				{ID: 6, ChatSessionID: "branch-2", TurnIndex: 10, SummaryJSON: `{"turn_summary":"NESTED_CHILD_OWNED_MEMORY brass observatory door"}`, Importance: 10},
				{ID: 7, ChatSessionID: "branch-2", TurnIndex: -1, SummaryJSON: `{"turn_summary":"HYPAMEMORY_ORIGINAL brass observatory oath engraved on the key","hypamemory_import":{"original_text":"HYPAMEMORY_ORIGINAL brass observatory oath engraved on the key"}}`, Importance: 10},
			},
		},
		chatBySession: map[string][]store.ChatLog{},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	currentInput := "What did the brass observatory oath and key mean?"
	activeChat := make([]map[string]any, 0, 21)
	for turn := 1; turn <= 10; turn++ {
		userText := fmt.Sprintf("prior user %d", turn)
		assistantText := fmt.Sprintf("prior assistant %d", turn)
		activeChat = append(activeChat,
			map[string]any{"observation_ref": fmt.Sprintf("active:user:%d", turn), "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": (turn - 1) * 2, "role": "user", "raw_content": userText, "content_hash": prepareOR1CHash(userText), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
			map[string]any{"observation_ref": fmt.Sprintf("active:assistant:%d", turn), "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": (turn-1)*2 + 1, "role": "assistant", "raw_content": assistantText, "content_hash": prepareOR1CHash(assistantText), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
		)
	}
	activeChat = append(activeChat, map[string]any{"observation_ref": "active:current", "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": 20, "role": "user", "raw_content": currentInput, "content_hash": prepareOR1CHash(currentInput), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"})
	body, err := json.Marshal(map[string]any{
		"chat_session_id": "branch-2",
		"raw_user_input":  currentInput,
		"host_observations": map[string]any{
			"contract_version": prepareHostObservationsVersion,
			"session_id":       "branch-2",
			"request_id":       "branch-2-turn-11",
			"request_type":     "model",
			"payload_writable": true,
			"active_chat":      activeChat,
			"payload":          []map[string]any{{"observation_ref": "payload:current", "source_kind": "before_request_payload", "message_index": 20, "role": "user", "raw_content": currentInput, "content_hash": prepareOR1CHash(currentInput), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"}},
		},
		"settings": map[string]any{"max_injection_chars": 6000, "max_input_context_chars": 2000, "injection_enabled": true, "input_context_enabled": true, "top_k": 8, "guide_strength": "none"},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	allText := extractionStringFromAny(response["injection_text"]) + "\n" + extractionStringFromAny(response["input_context_text"])
	for _, marker := range []string{"ROOT_INHERITED_MEMORY", "BRANCH_ONE_OWNED_MEMORY", "NESTED_CHILD_OWNED_MEMORY", "HYPAMEMORY_ORIGINAL"} {
		if !strings.Contains(allText, marker) {
			t.Fatalf("confirmed history marker %q missing from prepare-turn payload: %q", marker, allText)
		}
	}
	for _, marker := range []string{"ROOT_POST_FORK_MUST_NOT_LEAK", "BRANCH_COPIED_PREFIX_MUST_NOT_DUPLICATE", "NESTED_COPIED_PREFIX_MUST_NOT_DUPLICATE"} {
		if strings.Contains(allText, marker) {
			t.Fatalf("non-owned history marker %q leaked into prepare-turn payload: %q", marker, allText)
		}
	}
	wantReads := []prepareTurnHistorySegment{
		{SessionID: "root", FromTurn: 0, ToTurn: 8},
		{SessionID: "branch-1", FromTurn: 9, ToTurn: 9},
		{SessionID: "branch-2", FromTurn: 10, ToTurn: 10},
	}
	if !prepareTurnHistorySegmentsEqual(fake.memoryReads, wantReads) {
		t.Fatalf("memory reads=%+v, want %+v", fake.memoryReads, wantReads)
	}
	trace := mapFromAny(mapFromAny(response["trace_preview"])["materialization"])
	historyTrace := mapFromAny(trace["worldline_history_scope"])
	if historyTrace["state"] != "ready" || historyTrace["reason"] != "confirmed_worldline_history_composed" {
		t.Fatalf("worldline history trace mismatch: %#v", historyTrace)
	}
}

type prepareTurnWorldlineVectorStore struct {
	vector.VectorStore
	searchSessionIDs []string
}

func (s *prepareTurnWorldlineVectorStore) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", Collection: "test", ModelReady: true}, nil
}

func (s *prepareTurnWorldlineVectorStore) Search(_ context.Context, sessionID string, _ []float32, _ int, _ string) ([]vector.VectorDocument, error) {
	s.searchSessionIDs = append(s.searchSessionIDs, sessionID)
	return nil, vector.ErrNotFound
}

func TestPrepareTurnVectorRecallSearchesEachConfirmedWorldlineHistorySession(t *testing.T) {
	vectorStore := &prepareTurnWorldlineVectorStore{}
	srv := NewServer(config.Default())
	srv.Vector = vectorStore
	rawInput := "observatory oath"
	scope := prepareTurnHistoryScope{
		State:  "ready",
		Reason: "confirmed_worldline_history_composed",
		Segments: []prepareTurnHistorySegment{
			{SessionID: "root", FromTurn: 0, ToTurn: 8},
			{SessionID: "branch-1", FromTurn: 9, ToTurn: 9},
			{SessionID: "branch-2", FromTurn: 10, ToTurn: 10},
		},
	}
	shadow := srv.prepareTurnVectorShadow(context.Background(), dto.PrepareTurnRequest{
		ChatSessionID: "branch-2",
		RawUserInput:  &rawInput,
		ClientMeta:    map[string]any{"chroma_query_vector": []any{0.1, 0.2, 0.3}},
	}, 4, scope)
	want := []string{"root", "branch-1", "branch-2", "root", "branch-1", "branch-2"}
	if len(vectorStore.searchSessionIDs) != len(want) {
		t.Fatalf("search sessions=%v, want %v", vectorStore.searchSessionIDs, want)
	}
	for index := range want {
		if vectorStore.searchSessionIDs[index] != want[index] {
			t.Fatalf("search sessions=%v, want %v", vectorStore.searchSessionIDs, want)
		}
	}
	traceSessions := stringSliceFromAny(shadow["history_session_ids"])
	if len(traceSessions) != 3 || traceSessions[0] != "root" || traceSessions[1] != "branch-1" || traceSessions[2] != "branch-2" {
		t.Fatalf("history_session_ids=%v", traceSessions)
	}
}

func TestPrepareTurnDoesNotExposeStoryCompositionPlanner(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-weak-plan", TurnIndex: 7, Role: "user", Content: "Mina asks Rowan what they should do next."},
			{ID: 2, ChatSessionID: "sess-weak-plan", TurnIndex: 7, Role: "assistant", Content: "Rowan pauses at the shrine gate and waits for Mina's lead."},
		},
		returnResumePack: &store.ResumePack{
			Trigger:       "resume",
			AssembledText: "Mina and Rowan are paused at the shrine gate.",
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-weak-plan","turn_index":8,"raw_user_input":"계속","client_meta":{"language_context":{"session_output_language":"ko","output_language_source":"plugin_setting"}},"settings":{"max_injection_chars":1600,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2,"guide_mode":"standard","guide_strength":"weak","narrative_stance":"balanced"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"weak_input_planner", "progression_choice_ledger", "step25_validation_gate", "autonomy_plan", "micro_beat_proposal", "scene_step_proposal", "combined_proposal"} {
		if _, exists := resp[key]; exists {
			t.Fatalf("prepare-turn exposes story-composition surface %q: %#v", key, resp[key])
		}
	}
	execContract, ok := resp["response_execution_contract"].(map[string]any)
	if !ok {
		t.Fatalf("response_execution_contract missing: %#v", resp["response_execution_contract"])
	}
	if execContract["contract_version"] != "response_execution_contract.v1" || execContract["status"] != "ready" || execContract["truth_authority"] != false {
		t.Fatalf("unexpected execution contract: %#v", execContract)
	}
	for _, key := range []string{"must_preserve", "must_respond", "must_account", "must_not_assert", "source_refs"} {
		if _, ok := execContract[key].(map[string]any); !ok {
			t.Fatalf("response execution contract missing %s: %#v", key, execContract[key])
		}
	}
	for _, key := range []string{"scene_mandate", "required_outcome", "forbidden_move", "pacing_pressure", "ending_requirement", "role_lens_consumption"} {
		if _, exists := execContract[key]; exists {
			t.Fatalf("execution contract controls story composition through %q: %#v", key, execContract[key])
		}
	}
	consumeRule, ok := execContract["consume_rule"].(map[string]any)
	if !ok || len(stringSliceFromAny(consumeRule["blocked_usage"])) == 0 {
		t.Fatalf("execution contract missing consume rule: %#v", execContract["consume_rule"])
	}
	supervisor, ok := resp["supervisor_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_input_pack missing")
	}
	if _, ok := supervisor["response_execution_contract"].(map[string]any); !ok {
		t.Fatalf("supervisor pack missing response execution contract: %#v", supervisor["response_execution_contract"])
	}
	for _, key := range []string{"weak_input_planner", "progression_choice_ledger", "step25_validation_gate"} {
		if _, exists := supervisor[key]; exists {
			t.Fatalf("supervisor pack exposes story-composition planner %q: %#v", key, supervisor[key])
		}
	}
	guidance := extractionStringFromAny(supervisor["final_guidance_suffix"])
	for _, forbidden := range []string{"[Weak Input Planner]", "[Response Execution Contract]", "[Progression Choice Ledger]", "scene_mandate=", "pacing=", "ending_requirement="} {
		if strings.Contains(guidance, forbidden) {
			t.Fatalf("supervisor guidance controls story composition through %q: %q", forbidden, guidance)
		}
	}
}
