package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type prepareReadRecordingStore struct {
	turnRecordingStore
	readCalls int
}

type prepareReadRecordingVectorStore struct {
	turnRecordingVectorStore
	searchCalls int
}

type prepareRevisionFilterStore struct {
	store.Store
	active map[string]bool
	checks map[string]int
}

func (s *prepareRevisionFilterStore) MemoryDerivationLifecycleEnabled() bool {
	return true
}

func (s *prepareRevisionFilterStore) RegisterAcceptedSourceRevision(context.Context, *store.MemorySourceRevision) (store.SourceRevisionRegistration, error) {
	return store.SourceRevisionRegistration{}, errors.New("unexpected source revision registration")
}

func (s *prepareRevisionFilterStore) GetSourceRevision(context.Context, string, string) (*store.MemorySourceRevision, error) {
	return nil, store.ErrNotFound
}

func (s *prepareRevisionFilterStore) IsSourceRevisionActive(_ context.Context, sessionID, sourceRevision string) (bool, error) {
	if s.checks == nil {
		s.checks = map[string]int{}
	}
	key := sessionID + ":" + sourceRevision
	s.checks[key]++
	return s.active[key], nil
}

func (s *prepareRevisionFilterStore) InvalidateSourceRevisions(context.Context, string, int, string, string, time.Time) error {
	return errors.New("unexpected source revision invalidation")
}

type prepareRevisionFilterVector struct {
	vector.VectorStore
	documents []vector.VectorDocument
}

func (s *prepareRevisionFilterVector) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{
		Status: "ok", Collection: "test", TotalCount: len(s.documents), ModelReady: true,
	}, nil
}

func (s *prepareRevisionFilterVector) Search(context.Context, string, []float32, int, string) ([]vector.VectorDocument, error) {
	return append([]vector.VectorDocument(nil), s.documents...), nil
}

func TestPrepareTurnVectorShadowDropsInactiveRevisionBeforePreview(t *testing.T) {
	revisionMetadata := func(revision string) map[string]any {
		return map[string]any{
			"source_revision":     revision,
			"source_contract":     store.MemorySourceRevisionContract,
			"index_identity":      "memory-vector-index-v1",
			"content_fingerprint": "fingerprint-" + revision,
		}
	}
	vectorStore := &prepareRevisionFilterVector{documents: []vector.VectorDocument{
		{
			ID: "memory:session:legacy-unversioned", ChatSessionID: "session",
			DocumentText: "legacy source without an active revision", Similarity: 1,
			SimilarityAvailable: true,
		},
		{
			ID: "memory:session:stale-high", ChatSessionID: "session",
			DocumentText: "stale high similarity secret", Similarity: 0.999,
			SimilarityAvailable: true, Metadata: revisionMetadata("sar_stale"),
		},
		{
			ID: "memory:session:stale-duplicate", ChatSessionID: "session",
			DocumentText: "stale duplicate", Similarity: 0.998,
			SimilarityAvailable: true, Metadata: revisionMetadata("sar_stale"),
		},
		{
			ID: "memory:session:active", ChatSessionID: "session",
			DocumentText: "active lower similarity memory", Similarity: 0.7,
			SimilarityAvailable: true, Metadata: revisionMetadata("sar_active"),
		},
	}}
	lifecycleStore := &prepareRevisionFilterStore{
		Store: store.NewNoopStore(),
		active: map[string]bool{
			"session:sar_active": true,
		},
	}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	server := NewServer(cfg)
	server.Store = lifecycleStore
	server.Vector = vectorStore

	shadow := server.prepareTurnVectorShadow(context.Background(), dto.PrepareTurnRequest{
		ChatSessionID: "session",
		ClientMeta: map[string]any{
			"chroma_query_vector": []float32{0.1, 0.2},
		},
	}, 5)
	previews, ok := shadow["search_results"].([]map[string]any)
	if !ok || len(previews) != 1 || previews[0]["id"] != "memory:session:active" {
		t.Fatalf("inactive revisions reached search preview: %#v", shadow["search_results"])
	}
	if strings.Contains(fmt.Sprint(previews), "stale high similarity secret") {
		t.Fatalf("stale source text leaked into preview: %#v", previews)
	}
	filterTrace, _ := shadow["source_revision_filter"].(map[string]any)
	if filterTrace["status"] != "applied" || filterTrace["dropped_count"] != 3 ||
		filterTrace["dropped_missing_revision"] != 1 ||
		filterTrace["checked_count"] != 2 || lifecycleStore.checks["session:sar_stale"] != 1 {
		t.Fatalf("revision filter trace=%#v checks=%#v", filterTrace, lifecycleStore.checks)
	}
}

func (s *prepareReadRecordingVectorStore) Search(ctx context.Context, sessionID string, query []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	s.searchCalls++
	return s.turnRecordingVectorStore.Search(ctx, sessionID, query, limit, filter)
}

func (s *prepareReadRecordingStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	s.readCalls++
	return s.turnRecordingStore.ListMemories(ctx, sid, fromTurn, toTurn)
}

func TestPrepareTurnSourceContractStatuses(t *testing.T) {
	boolTrue := true
	requestID := "request-a"
	role := "user"
	path := `["messages"]`
	hash := "or1c_1234"
	hashAlgorithm := "or1c_utf16_djb2.v1"
	messageIndex := 2

	base := func() dto.PrepareTurnContractRequest {
		return dto.PrepareTurnContractRequest{
			PrepareTurnRequest: dto.PrepareTurnRequest{ChatSessionID: "session-a"},
			SourceObservation: &dto.PrepareTurnSourceObservationV1{
				ContractVersion:       prepareSourceObservationVersion,
				SessionID:             "session-a",
				RequestID:             &requestID,
				MessageIndex:          &messageIndex,
				ObservedRole:          &role,
				ObservedSourcePath:    &path,
				RawInputHash:          &hash,
				RawInputHashAlgorithm: &hashAlgorithm,
				Observable:            &boolTrue,
				EvidenceState:         "observed",
			},
			CapabilityObservation: &dto.PrepareTurnCapabilityObservationV1{
				ContractVersion: prepareCapabilityObservationVersion,
				Capabilities: map[string]string{
					"message_position":    "observed",
					"message_role":        "observed",
					"request_correlation": "observed",
					"session_identity":    "observed",
					"raw_input_hash":      "observed",
					"source_path":         "observed",
				},
			},
		}
	}

	tests := []struct {
		name       string
		mutate     func(*dto.PrepareTurnContractRequest)
		status     string
		reasonCode string
		retryable  bool
	}{
		{name: "eligible", status: "eligible", reasonCode: "source_observation_eligible"},
		{name: "unsupported version", mutate: func(request *dto.PrepareTurnContractRequest) {
			request.SourceObservation.ContractVersion = "message_source_observation.v99"
		}, status: "incompatible", reasonCode: "source_observation_contract_incompatible"},
		{name: "required capability missing", mutate: func(request *dto.PrepareTurnContractRequest) {
			delete(request.CapabilityObservation.Capabilities, "request_correlation")
		}, status: "incompatible", reasonCode: "source_observation_required_capability_missing"},
		{name: "optional capability missing", mutate: func(request *dto.PrepareTurnContractRequest) {
			delete(request.CapabilityObservation.Capabilities, "source_path")
		}, status: "degraded", reasonCode: "source_observation_optional_capability_missing"},
		{name: "not applicable", mutate: func(request *dto.PrepareTurnContractRequest) {
			request.SourceObservation.EvidenceState = "not_applicable"
		}, status: "not_applicable", reasonCode: "source_observation_not_applicable"},
		{name: "empty", mutate: func(request *dto.PrepareTurnContractRequest) {
			request.SourceObservation.EvidenceState = "empty"
			request.SourceObservation.MessageIndex = nil
			request.SourceObservation.ObservedRole = nil
			request.SourceObservation.RawInputHash = nil
			request.SourceObservation.RawInputHashAlgorithm = nil
			request.CapabilityObservation.Capabilities["message_position"] = "unavailable"
			request.CapabilityObservation.Capabilities["message_role"] = "unavailable"
			request.CapabilityObservation.Capabilities["raw_input_hash"] = "unavailable"
		}, status: "empty", reasonCode: "source_observation_empty"},
		{name: "deferred", mutate: func(request *dto.PrepareTurnContractRequest) {
			request.SourceObservation.EvidenceState = "deferred"
		}, status: "deferred", reasonCode: "source_observation_deferred", retryable: true},
		{name: "malformed", mutate: func(request *dto.PrepareTurnContractRequest) {
			badIndex := -1
			request.SourceObservation.MessageIndex = &badIndex
		}, status: "failed", reasonCode: "source_observation_malformed"},
		{name: "observed hash without value", mutate: func(request *dto.PrepareTurnContractRequest) {
			request.SourceObservation.RawInputHash = nil
			request.SourceObservation.RawInputHashAlgorithm = nil
		}, status: "failed", reasonCode: "source_observation_malformed"},
		{name: "whitespace observation", mutate: func(request *dto.PrepareTurnContractRequest) {
			blank := "   "
			request.SourceObservation.RequestID = &blank
		}, status: "failed", reasonCode: "source_observation_malformed"},
		{name: "unknown optional capability", mutate: func(request *dto.PrepareTurnContractRequest) {
			request.CapabilityObservation.Capabilities["future_optional_capability"] = "not_exposed"
		}, status: "eligible", reasonCode: "source_observation_eligible"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := base()
			if test.mutate != nil {
				test.mutate(&request)
			}
			first := buildPrepareTurnSourceContract(request, "session-a")
			second := buildPrepareTurnSourceContract(request, "session-a")
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("normalization is not deterministic: first=%+v second=%+v", first, second)
			}
			status := first.LaneStatus
			if status.Status != test.status || status.ReasonCode != test.reasonCode || status.Retryable != test.retryable {
				t.Fatalf("status=%+v, want status=%q reason=%q retryable=%v", status, test.status, test.reasonCode, test.retryable)
			}
			if status.AffectedLane != prepareSourceLane || !status.OriginalPayloadPreserved || status.ContractVersion != prepareSourceLaneStatusVersion {
				t.Fatalf("stable lane metadata missing: %+v", status)
			}
			for _, forbidden := range []string{"ok", "ready", "skipped"} {
				if status.Status == forbidden {
					t.Fatalf("lane status was hidden as %q", forbidden)
				}
			}
			if first.NormalizedSource != nil && (first.NormalizedSource.ChatID != nil || first.NormalizedSource.BranchID != nil || first.NormalizedSource.GenerationID != nil || first.NormalizedSource.ObservedRevision != nil || first.NormalizedSource.HostSource != nil) {
				t.Fatalf("unobserved identity/source values were inferred: %+v", first.NormalizedSource)
			}
		})
	}
}

func TestPrepareTurnProductionRouteCarriesSourceContractWithoutWrites(t *testing.T) {
	storeSpy := &turnRecordingStore{}
	vectorSpy := &prepareReadRecordingVectorStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = storeSpy
	srv.Vector = vectorSpy
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id": "session-a",
		"raw_user_input":  "observed input",
		"messages":        []map[string]any{{"role": "user", "content": "observed input"}},
		"source_observation": map[string]any{
			"contract_version":         prepareSourceObservationVersion,
			"session_id":               "session-a",
			"request_id":               "request-a",
			"message_index":            0,
			"observed_role":            "user",
			"observed_source_path":     `["messages"]`,
			"raw_input_hash":           "or1c_1234",
			"raw_input_hash_algorithm": "or1c_utf16_djb2.v1",
			"observable":               true,
			"evidence_state":           "observed",
			"future_optional_field":    true,
		},
		"capability_observation": map[string]any{
			"contract_version": prepareCapabilityObservationVersion,
			"capabilities": map[string]string{
				"message_position":    "observed",
				"message_role":        "observed",
				"request_correlation": "observed",
				"session_identity":    "observed",
				"raw_input_hash":      "observed",
				"source_path":         "observed",
			},
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(encoded)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["status"] != "ok" {
		t.Fatalf("main prepare payload was not preserved: status=%v", response["status"])
	}
	contract, ok := response["source_contract"].(map[string]any)
	if !ok {
		t.Fatalf("source_contract missing: %s", recorder.Body.String())
	}
	lane, _ := contract["lane_status"].(map[string]any)
	if lane["status"] != "eligible" || lane["reason_code"] != "source_observation_eligible" || lane["request_correlation_id"] != "request-a" {
		t.Fatalf("unexpected lane status: %+v", lane)
	}
	if len(storeSpy.savedChatLogs)+len(storeSpy.savedEffectiveInputs)+len(storeSpy.savedAuditLogs)+len(storeSpy.savedCriticFeedback)+len(storeSpy.savedMemories)+len(storeSpy.savedEvidence)+len(storeSpy.savedKGTriples)+len(storeSpy.savedStorylines)+len(storeSpy.savedWorldRules)+len(storeSpy.savedEntities)+len(storeSpy.savedTrusts)+len(storeSpy.savedCharacterEvents)+len(storeSpy.savedCharacterStates)+len(storeSpy.savedPendingThreads)+len(storeSpy.savedActiveStates)+len(storeSpy.savedCanonicalLayers) != 0 {
		t.Fatal("prepare source contract caused a canonical DB write")
	}
	if vectorSpy.upsertCalls != 0 || vectorSpy.deleteSessionCalls != 0 || vectorSpy.deleteDocumentCalls != 0 || vectorSpy.rebuildCalls != 0 {
		t.Fatalf("prepare source contract caused vector writes: %+v", vectorSpy)
	}
}

func TestPrepareTurnSourceDecisionOnlyIsReadFree(t *testing.T) {
	storeSpy := &prepareReadRecordingStore{}
	vectorSpy := &prepareReadRecordingVectorStore{}
	srv := NewServer(config.Default())
	srv.Store = storeSpy
	srv.Vector = vectorSpy
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	input := "stored processed input"
	inputHook := "input hook intermediate text"
	body := map[string]any{
		"chat_session_id":      "session-d",
		"request_type":         "model",
		"raw_user_input":       "",
		"source_decision_only": true,
		"messages": []map[string]any{
			{"role": "user", "content": "payload transformed input"},
			{"role": "user", "content": "later host prompt"},
		},
		"host_observations": map[string]any{
			"contract_version": prepareHostObservationsVersion, "session_id": "session-d", "request_id": "request-d", "request_type": "model", "payload_writable": true,
			"input_hook": map[string]any{"observation_ref": "input-hook:request-d", "source_kind": "input_hook_intermediate", "role": "user", "raw_content": inputHook, "content_hash": prepareOR1CHash(inputHook), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
			"active_chat": []map[string]any{
				{"observation_ref": "active:1", "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": 1, "role": "user", "raw_content": input, "content_hash": prepareOR1CHash(input), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
			},
			"payload": []map[string]any{
				{"observation_ref": "payload:0", "source_kind": "before_request_payload", "message_index": 0, "role": "user", "raw_content": "payload transformed input", "content_hash": prepareOR1CHash("payload transformed input"), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
				{"observation_ref": "payload:1", "source_kind": "before_request_payload", "message_index": 1, "role": "user", "raw_content": "later host prompt", "content_hash": prepareOR1CHash("later host prompt"), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
			},
		},
		"bootstrap_observation": map[string]any{"contract_version": prepareBootstrapObservationVersion, "session_id": "session-d", "request_id": "request-d", "observation_state": "empty", "leading_messages": []any{}},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(encoded)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	wantKeys := map[string]bool{
		"backend_instance_id": true,
		"source_contract":     true, "current_input_decision": true, "message_source_envelope": true, "session_bootstrap": true,
		"risu_host_context_snapshot": true, "host_context_reference_evidence": true,
	}
	if response["backend_instance_id"] != srv.BackendInstanceID {
		t.Fatalf("backend_instance_id=%v want=%q", response["backend_instance_id"], srv.BackendInstanceID)
	}
	if len(response) != len(wantKeys) {
		t.Fatalf("decision-only response contains non-contract fields: %v", reflect.ValueOf(response).MapKeys())
	}
	for key := range wantKeys {
		if _, ok := response[key]; !ok {
			t.Fatalf("decision-only response missing %q: %s", key, recorder.Body.String())
		}
	}
	decision := response["current_input_decision"].(map[string]any)
	if decision["status"] != "eligible" || decision["effective_user_input"] != input || decision["selected_observation_ref"] != "active:1" || decision["reason_code"] != "current_user_input_observed_with_host_transform" {
		t.Fatalf("stored active user was not authoritative over intermediate/payload text: %+v", decision)
	}
	if storeSpy.readCalls != 0 || vectorSpy.searchCalls != 0 || vectorSpy.upsertCalls != 0 || vectorSpy.deleteSessionCalls != 0 || vectorSpy.deleteDocumentCalls != 0 || vectorSpy.rebuildCalls != 0 {
		t.Fatalf("source-decision-only crossed persistence/vector boundary: store_reads=%d vector=%+v", storeSpy.readCalls, vectorSpy)
	}
}

func TestPrepareTurnCurrentInputDecisionUsesLifecycleFacts(t *testing.T) {
	text := `{"supervisor":"return JSON only"}`
	hash := prepareOR1CHash(text)
	algorithm := "or1c_utf16_djb2.v1"
	roleUser := "user"
	roleAssistant := "assistant"
	indexZero := 0
	indexOne := 1

	base := func() dto.PrepareTurnContractRequest {
		hookText := "input handler intermediate"
		hookHash := prepareOR1CHash(hookText)
		return dto.PrepareTurnContractRequest{
			PrepareTurnRequest: dto.PrepareTurnRequest{ChatSessionID: "session-d", RequestType: stringPointer("model")},
			HostObservations: &dto.PrepareTurnHostObservationsV1{
				ContractVersion: prepareHostObservationsVersion,
				SessionID:       "session-d",
				RequestID:       "request-d",
				RequestType:     "model",
				PayloadWritable: true,
				InputHook: &dto.PrepareTurnMessageObservationV1{
					ObservationRef: "input-hook:1", SourceKind: "input_hook_intermediate", Role: &roleUser,
					RawContent: &hookText, ContentHash: &hookHash, HashAlgorithm: &algorithm, EvidenceState: "observed",
				},
				ActiveChat: []dto.PrepareTurnMessageObservationV1{{
					ObservationRef: "active:1", SourceKind: "active_chat", ObservationStage: "active_chat_stored_message", MessageIndex: &indexOne,
					Role: &roleUser, RawContent: &text, ContentHash: &hash, HashAlgorithm: &algorithm, EvidenceState: "observed",
				}},
				Payload: []dto.PrepareTurnMessageObservationV1{{
					ObservationRef: "payload:0", SourceKind: "before_request_payload", MessageIndex: &indexZero,
					Role: &roleUser, RawContent: &text, ContentHash: &hash, HashAlgorithm: &algorithm, EvidenceState: "observed",
				}},
			},
		}
	}

	t.Run("stored active user content is never classified by prose", func(t *testing.T) {
		decision, enforced := buildPrepareTurnCurrentInputDecision(base(), "session-d")
		if !enforced || decision.Status != "eligible" || decision.EffectiveUserInput != text || !decision.ContextInjectionEligible || decision.Envelope == nil || decision.SelectedObservationRef == nil || *decision.SelectedObservationRef != "active:1" {
			t.Fatalf("decision=%+v enforced=%v", decision, enforced)
		}
	})

	t.Run("active chat tail is authoritative only when it is a current user", func(t *testing.T) {
		request := base()
		request.HostObservations.InputHook = &dto.PrepareTurnMessageObservationV1{ObservationRef: "hook:none", SourceKind: "input_hook", EvidenceState: "unavailable"}
		request.HostObservations.ActiveChat = []dto.PrepareTurnMessageObservationV1{
			{ObservationRef: "active:0", SourceKind: "active_chat", ObservationStage: "active_chat_stored_message", MessageIndex: &indexZero, Role: &roleAssistant, RawContent: stringPointer("start"), ContentHash: stringPointer(prepareOR1CHash("start")), HashAlgorithm: &algorithm, EvidenceState: "observed"},
			{ObservationRef: "active:1", SourceKind: "active_chat", ObservationStage: "active_chat_stored_message", MessageIndex: &indexOne, Role: &roleUser, RawContent: &text, ContentHash: &hash, HashAlgorithm: &algorithm, EvidenceState: "observed"},
		}
		decision, _ := buildPrepareTurnCurrentInputDecision(request, "session-d")
		if decision.Status != "eligible" || decision.SelectedObservationRef == nil || *decision.SelectedObservationRef != "active:1" {
			t.Fatalf("unexpected active fallback: %+v", decision)
		}
		request.HostObservations.ActiveChat = request.HostObservations.ActiveChat[:1]
		decision, _ = buildPrepareTurnCurrentInputDecision(request, "session-d")
		if decision.Status != "unverified" || decision.ReasonCode != "current_user_input_active_chat_stale" {
			t.Fatalf("stale active tail promoted: %+v", decision)
		}
	})

	t.Run("payload-only prose is not promoted", func(t *testing.T) {
		request := base()
		request.HostObservations.InputHook = nil
		request.HostObservations.ActiveChat = nil
		decision, _ := buildPrepareTurnCurrentInputDecision(request, "session-d")
		if decision.Status != "unverified" || decision.EffectiveUserInput != "" || decision.MemoryReadsAllowed {
			t.Fatalf("payload-only decision=%+v", decision)
		}
	})

	t.Run("host history trim lifecycle remains non narrative", func(t *testing.T) {
		request := base()
		request.HostObservations.InputHook.LifecycleKind = "history_trim_command"
		decision, _ := buildPrepareTurnCurrentInputDecision(request, "session-d")
		if decision.Status != "not_applicable" || decision.ReasonCode != "current_user_input_history_trim_command" || decision.MemoryReadsAllowed {
			t.Fatalf("history trim command entered narrative source lane: %+v", decision)
		}
	})

	t.Run("explicit empty and malformed hash remain distinct", func(t *testing.T) {
		request := base()
		empty := ""
		request.HostObservations.ActiveChat[0].RawContent = &empty
		request.HostObservations.ActiveChat[0].ContentHash = nil
		request.HostObservations.ActiveChat[0].HashAlgorithm = nil
		request.HostObservations.ActiveChat[0].EvidenceState = "empty"
		decision, _ := buildPrepareTurnCurrentInputDecision(request, "session-d")
		if decision.Status != "absent" || decision.ReasonCode != "current_user_input_absent" {
			t.Fatalf("empty decision=%+v", decision)
		}
		request = base()
		request.HostObservations.ActiveChat[0].ContentHash = stringPointer("or1c_wrong")
		decision, _ = buildPrepareTurnCurrentInputDecision(request, "session-d")
		if decision.Status != "failed" || decision.ReasonCode != "current_user_input_hash_mismatch" {
			t.Fatalf("malformed hash decision=%+v", decision)
		}
	})

	t.Run("group style observation without input hook remains eligible", func(t *testing.T) {
		request := base()
		request.HostObservations.InputHook = nil
		decision, _ := buildPrepareTurnCurrentInputDecision(request, "session-d")
		if decision.Status != "eligible" || decision.EffectiveUserInput != text || !decision.MemoryReadsAllowed {
			t.Fatalf("group-style active observation was lost: %+v", decision)
		}
	})

	t.Run("malformed input hook cannot override stored active user", func(t *testing.T) {
		request := base()
		request.HostObservations.InputHook.ContentHash = stringPointer("or1c_wrong")
		decision, _ := buildPrepareTurnCurrentInputDecision(request, "session-d")
		if decision.Status != "eligible" || decision.EffectiveUserInput != text || decision.SelectedObservationRef == nil || *decision.SelectedObservationRef != "active:1" {
			t.Fatalf("intermediate input hook overrode active chat: %+v", decision)
		}
	})
}

func TestPrepareTurnInputContextUsesHostPreviousCompletedTurn(t *testing.T) {
	roleUser := "user"
	roleAssistant := "assistant"
	indexZero, indexOne, indexTwo, indexThree := 0, 1, 2, 3
	greeting := "opening greeting"
	previousUser := "previous user instruction"
	previousAssistant := "previous assistant result"
	currentUser := "current user instruction"
	request := dto.PrepareTurnContractRequest{
		HostObservations: &dto.PrepareTurnHostObservationsV1{
			ActiveChat: []dto.PrepareTurnMessageObservationV1{
				{MessageIndex: &indexZero, Role: &roleAssistant, RawContent: &greeting},
				{MessageIndex: &indexOne, Role: &roleUser, RawContent: &previousUser},
				{MessageIndex: &indexTwo, Role: &roleAssistant, RawContent: &previousAssistant},
				{MessageIndex: &indexThree, Role: &roleUser, RawContent: &currentUser},
			},
		},
	}
	decision := dto.PrepareTurnCurrentInputDecisionV1{Envelope: &dto.PrepareTurnMessageSourceEnvelopeV1{
		Identity: dto.PrepareTurnMessageSourceIdentityV1{MessageIndex: &indexThree},
	}}
	stored := []store.ChatLog{
		{TurnIndex: 80, Role: "user", Content: currentUser},
		{TurnIndex: 80, Role: "assistant", Content: "post-generation output must not enter the request"},
	}

	selected, source := prepareTurnInputContextChatLogs(request, decision, stored)
	if source != "host_active_chat_previous_completed_turn" || len(selected) != 2 {
		t.Fatalf("source=%q selected=%+v", source, selected)
	}
	if selected[0].Role != "user" || selected[0].Content != previousUser || selected[1].Role != "assistant" || selected[1].Content != previousAssistant {
		t.Fatalf("wrong logical turn selected: %+v", selected)
	}
	for _, item := range selected {
		if item.Content == currentUser || strings.Contains(item.Content, "post-generation") {
			t.Fatalf("current/post-generation content leaked into Input Context: %+v", selected)
		}
	}
}

func TestPrepareTurnInputContextDoesNotFallbackWhenHostHasNoPreviousPair(t *testing.T) {
	roleUser := "user"
	indexOne := 1
	currentUser := "first current user instruction"
	request := dto.PrepareTurnContractRequest{
		HostObservations: &dto.PrepareTurnHostObservationsV1{ActiveChat: []dto.PrepareTurnMessageObservationV1{{
			MessageIndex: &indexOne, Role: &roleUser, RawContent: &currentUser,
		}}},
	}
	decision := dto.PrepareTurnCurrentInputDecisionV1{Envelope: &dto.PrepareTurnMessageSourceEnvelopeV1{
		Identity: dto.PrepareTurnMessageSourceIdentityV1{MessageIndex: &indexOne},
	}}
	stored := []store.ChatLog{{TurnIndex: 99, Role: "assistant", Content: "stale database tail"}}

	selected, source := prepareTurnInputContextChatLogs(request, decision, stored)
	if source != "host_active_chat_no_previous_completed_turn" || len(selected) != 0 {
		t.Fatalf("authoritative host emptiness fell back to DB: source=%q selected=%+v", source, selected)
	}
}

func TestPrepareTurnSessionBootstrapPreservesOrderedSources(t *testing.T) {
	algorithm := "or1c_utf16_djb2.v1"
	assistant := "assistant"
	first := "same greeting"
	second := "same greeting"
	indexZero := 0
	indexOne := 1
	request := dto.PrepareTurnContractRequest{BootstrapObservation: &dto.PrepareTurnBootstrapObservationV1{
		ContractVersion:  prepareBootstrapObservationVersion,
		SessionID:        "session-d",
		RequestID:        "request-d",
		ObservationState: "observed",
		LeadingMessages: []dto.PrepareTurnMessageObservationV1{
			{ObservationRef: "bootstrap:0", SourceKind: "active_chat_bootstrap", MessageIndex: &indexZero, Role: &assistant, RawContent: &first, ContentHash: stringPointer(prepareOR1CHash(first)), HashAlgorithm: &algorithm, EvidenceState: "observed"},
			{ObservationRef: "bootstrap:1", SourceKind: "active_chat_bootstrap", MessageIndex: &indexOne, Role: &assistant, RawContent: &second, ContentHash: stringPointer(prepareOR1CHash(second)), HashAlgorithm: &algorithm, EvidenceState: "observed"},
		},
	}}
	projection := buildPrepareTurnSessionBootstrap(request, "session-d")
	if projection.Status != "preserved" || len(projection.Sources) != 2 {
		t.Fatalf("projection=%+v", projection)
	}
	if projection.Sources[0].Identity.MessageIndex == nil || *projection.Sources[0].Identity.MessageIndex != 0 || projection.Sources[1].Identity.MessageIndex == nil || *projection.Sources[1].Identity.MessageIndex != 1 {
		t.Fatalf("order/boundaries lost: %+v", projection.Sources)
	}
	if projection.Sources[0].BackendSourceID == projection.Sources[1].BackendSourceID {
		t.Fatal("duplicate text at distinct host indexes was hash-deduplicated")
	}
	if projection.PriorResponseSource.Status != "absent" || projection.MemoryReadsAllowed || projection.InjectionAllowed || projection.CanonicalWriteAllowed {
		t.Fatalf("bootstrap authority widened: %+v", projection)
	}
	selectedIndex := 1
	request.BootstrapObservation.LeadingMessages = nil
	request.BootstrapObservation.SelectionExposed = true
	request.BootstrapObservation.SelectedGreetingIndex = &selectedIndex
	request.BootstrapObservation.AlternateGreetings = []string{"alternate zero", "alternate one"}
	projection = buildPrepareTurnSessionBootstrap(request, "session-d")
	if projection.Status != "preserved" || len(projection.Sources) != 1 || projection.Sources[0].RawContent != "alternate one" || projection.Sources[0].OriginProvenance != "host_selected_greeting" {
		t.Fatalf("selected alternate greeting was not decided by Go: %+v", projection)
	}
	firstGreetingIndex := -1
	firstGreeting := "first greeting"
	request.BootstrapObservation.SelectedGreetingIndex = &firstGreetingIndex
	request.BootstrapObservation.FirstGreeting = &firstGreeting
	projection = buildPrepareTurnSessionBootstrap(request, "session-d")
	if projection.Status != "preserved" || len(projection.Sources) != 1 || projection.Sources[0].RawContent != firstGreeting || projection.Sources[0].ObservationRef != "host_greeting:first" {
		t.Fatalf("fmIndex=-1 did not preserve first greeting: %+v", projection)
	}
	for _, invalidIndex := range []int{-2, len(request.BootstrapObservation.AlternateGreetings)} {
		request.BootstrapObservation.SelectedGreetingIndex = &invalidIndex
		projection = buildPrepareTurnSessionBootstrap(request, "session-d")
		if projection.Status != "unverified" || projection.ReasonCode != "bootstrap_selected_greeting_unavailable" || len(projection.Sources) != 0 {
			t.Fatalf("invalid greeting index %d was guessed: %+v", invalidIndex, projection)
		}
	}
}

func TestPrepareTurnProductionRouteSuppressesUnverifiedInputBeforeStoreReads(t *testing.T) {
	storeSpy := &prepareReadRecordingStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = storeSpy
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id": "session-d",
		"raw_user_input":  "payload must not become authority",
		"messages":        []map[string]any{{"role": "user", "content": "payload must not become authority"}},
		"host_observations": map[string]any{
			"contract_version": prepareHostObservationsVersion, "session_id": "session-d", "request_id": "request-d", "request_type": "model", "payload_writable": true,
			"payload": []map[string]any{{"observation_ref": "payload:0", "source_kind": "before_request_payload", "message_index": 0, "role": "user", "raw_content": "payload must not become authority", "content_hash": prepareOR1CHash("payload must not become authority"), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"}},
		},
		"bootstrap_observation": map[string]any{"contract_version": prepareBootstrapObservationVersion, "session_id": "session-d", "request_id": "request-d", "observation_state": "empty", "leading_messages": []any{}},
	}
	encoded, _ := json.Marshal(body)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(encoded)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if storeSpy.readCalls != 0 {
		t.Fatalf("unverified source reached store reads: %d", storeSpy.readCalls)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	decision := response["current_input_decision"].(map[string]any)
	if decision["status"] != "unverified" || response["effective_user_input"] != "" || response["injection_text"] != "" {
		t.Fatalf("suppression response=%s", recorder.Body.String())
	}
}
