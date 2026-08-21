package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type prepareTurnLorebookReferenceStore struct {
	store.Store
	current   *store.LorebookReferenceCurrent
	readErr   error
	readCount int
	lastScope store.LorebookReferenceScope
}

func (f *prepareTurnLorebookReferenceStore) ApplyLorebookReferenceSnapshot(context.Context, *store.LorebookReferenceSnapshot) (*store.LorebookReferenceSnapshotResult, error) {
	return nil, errors.New("unexpected lorebook snapshot write during prepare-turn")
}

func (f *prepareTurnLorebookReferenceStore) GetLorebookReferenceCurrent(_ context.Context, scope store.LorebookReferenceScope) (*store.LorebookReferenceCurrent, error) {
	f.readCount++
	f.lastScope = scope
	if f.readErr != nil {
		return nil, f.readErr
	}
	if f.current == nil {
		return nil, store.ErrNotFound
	}
	return f.current, nil
}

func TestPrepareTurnLorebookSearchOnlyUsesExactKeyAndLexicalWithoutDelivery(t *testing.T) {
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{
			ScopeID: 12,
			Entries: []store.LorebookReferenceEntryObservation{
				{HostEntryID: "exact", EntryOrdinal: 0, Content: "세종이 과거 시험을 연다"},
				{HostEntryID: "key", EntryOrdinal: 1, Key: "한얼", Content: "주인공 설정"},
				{HostEntryID: "lexical", EntryOrdinal: 2, Content: "시험장 소식"},
				{HostEntryID: "unrelated", EntryOrdinal: 3, Content: "달의 궤도"},
			},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-g3",
		"raw_user_input":"한얼은 세종이 과거 시험을 연다는 시험장 소식을 들었다",
		"response_projection":"prepare_turn.production_compact.v1",
		"lorebook_reference_scope":{
			"contract_version":"lorebook_reference_scope.v1",
			"observation_state":"observed",
			"character_index":4,
			"chat_index":9,
			"enabled_module_ids":["module-a"],
			"enabled_modules_observed":true
		},
		"settings":{"guide_strength":"none","lorebook_reference_mode":"search_only"}
	}`)

	result := mapFromAny(response["lorebook_reference"])
	if result["status"] != "ready" || intFromAny(result["candidate_count"], 0) != 3 {
		t.Fatalf("lorebook search=%#v", result)
	}
	methods := mapFromAny(result["method_counts"])
	for _, method := range []string{"exact_phrase", "key", "lexical"} {
		if intFromAny(methods[method], 0) == 0 {
			t.Fatalf("method %s was not observed: %#v", method, methods)
		}
	}
	if intFromAny(result["delivery_count"], -1) != 0 || intFromAny(result["publisher_count"], -1) != 0 {
		t.Fatalf("search_only delivered lorebook material: %#v", result)
	}
	if fake.readCount != 1 || fake.lastScope.ChatSessionID != "lore-g3" || fake.lastScope.CharacterIndex == nil || *fake.lastScope.CharacterIndex != 4 {
		t.Fatalf("read_count=%d scope=%#v", fake.readCount, fake.lastScope)
	}
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"세종이 과거 시험을 연다", "주인공 설정", "시험장 소식", "달의 궤도"} {
		if strings.Contains(string(serialized), content) {
			t.Fatalf("search diagnostic exposed raw lorebook content %q: %s", content, serialized)
		}
	}
}

func TestPrepareTurnLorebookLegacyOffEnablesReferenceAssistAndSearchOnlyDoesNotDeliver(t *testing.T) {
	request := func(mode string, fake *prepareTurnLorebookReferenceStore) map[string]any {
		t.Helper()
		srv := setupTestServer()
		srv.Store = fake
		_, response := prepareTurnPerfRequest(t, srv, `{
			"chat_session_id":"lore-g3-invariant",
			"raw_user_input":"한얼이 시험장으로 간다",
			"response_projection":"prepare_turn.production_compact.v1",
			"lorebook_reference_scope":{
				"contract_version":"lorebook_reference_scope.v1",
				"observation_state":"observed",
				"character_index":1,
				"chat_index":2,
				"enabled_module_ids":[],
				"enabled_modules_observed":true
			},
			"settings":{"guide_strength":"none","lorebook_reference_mode":"`+mode+`"}
		}`)
		return response
	}

	alwaysActive := true
	legacyStore := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 2, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "legacy-entry", EntryOrdinal: 0, Key: "한얼", Content: "한얼은 유생이다", AlwaysActive: &alwaysActive},
		}},
	}
	legacy := request("off", legacyStore)
	if legacyStore.readCount != 1 {
		t.Fatalf("legacy off mode read count=%d, want reference-assist read", legacyStore.readCount)
	}
	legacyResult := mapFromAny(legacy["lorebook_reference"])
	if legacyResult["mode"] != prepareTurnLorebookModeReferenceAssist || intFromAny(legacyResult["delivery_count"], 0) != 1 {
		t.Fatalf("legacy off mode did not normalize to delivered reference assist: %#v", legacyResult)
	}
	searchStore := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 3, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "entry", EntryOrdinal: 0, Key: "한얼", Content: "한얼은 유생이다"},
		}},
	}
	search := request("search_only", searchStore)
	if searchStore.readCount != 1 {
		t.Fatalf("search_only read count=%d", searchStore.readCount)
	}
	searchPack := mapFromAny(search["injection_pack"])
	searchResult := mapFromAny(searchPack["lorebook_reference_recall"])
	if intFromAny(searchResult["delivery_count"], -1) != 0 || intFromAny(searchResult["publisher_count"], -1) != 0 {
		t.Fatalf("search_only delivery changed: %#v", searchResult)
	}
}

func TestPrepareTurnLorebookMissingModeDefaultsToReferenceAssist(t *testing.T) {
	alwaysActive := true
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 4, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "default-entry", EntryOrdinal: 0, Key: "한얼", Content: "한얼은 유생이다", AlwaysActive: &alwaysActive},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-default-assist",
		"raw_user_input":"한얼이 시험장으로 간다",
		"response_projection":"prepare_turn.production_compact.v1",
		"lorebook_reference_scope":{
			"contract_version":"lorebook_reference_scope.v1",
			"observation_state":"observed",
			"character_index":1,
			"chat_index":2,
			"enabled_module_ids":[],
			"enabled_modules_observed":true
		},
		"settings":{"guide_strength":"none"}
	}`)
	if fake.readCount != 1 {
		t.Fatalf("missing mode read count=%d, want reference-assist read", fake.readCount)
	}
	result := mapFromAny(response["lorebook_reference"])
	if result["mode"] != prepareTurnLorebookModeReferenceAssist || intFromAny(result["delivery_count"], 0) != 1 {
		t.Fatalf("missing mode did not default to delivered reference assist: %#v", result)
	}
}

func TestPrepareTurnLorebookInvalidModeIsVisibleAndDoesNotRead(t *testing.T) {
	fake := &prepareTurnLorebookReferenceStore{Store: store.NewNoopStore()}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-g3-invalid-mode",
		"raw_user_input":"한얼은 어디로 가야 할까?",
		"lorebook_reference_scope":{
			"contract_version":"lorebook_reference_scope.v1",
			"observation_state":"observed",
			"enabled_module_ids":[],
			"enabled_modules_observed":true
		},
		"settings":{"guide_strength":"none","lorebook_reference_mode":"unexpected_mode"}
	}`)
	result := mapFromAny(response["lorebook_reference"])
	if result["status"] != "unavailable" || result["reason_code"] != "lorebook_reference_mode_invalid" {
		t.Fatalf("invalid mode was hidden or treated as off: %#v", result)
	}
	if fake.readCount != 0 {
		t.Fatalf("invalid mode read lorebook store %d times", fake.readCount)
	}
}

func TestPrepareTurnLorebookMissingScopeAndReadFailureRemainLaneLocal(t *testing.T) {
	tests := []struct {
		name       string
		scope      string
		readErr    error
		wantStatus string
		wantReads  int
	}{
		{name: "missing scope", scope: "", wantStatus: "unavailable", wantReads: 0},
		{name: "store read failure", scope: `,"lorebook_reference_scope":{"contract_version":"lorebook_reference_scope.v1","observation_state":"observed","character_index":1,"chat_index":2,"enabled_module_ids":[],"enabled_modules_observed":true}`, readErr: errors.New("temporary read failure"), wantStatus: "unavailable", wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &prepareTurnLorebookReferenceStore{Store: store.NewNoopStore(), readErr: tc.readErr}
			srv := setupTestServer()
			srv.Store = fake
			_, response := prepareTurnPerfRequest(t, srv, `{
				"chat_session_id":"lore-g3-failure",
				"raw_user_input":"한얼이 시험장으로 간다"`+tc.scope+`,
				"settings":{"guide_strength":"none","lorebook_reference_mode":"search_only"}
			}`)
			if response["status"] != "ok" {
				t.Fatalf("turn failed because lorebook lane failed: %#v", response)
			}
			result := mapFromAny(response["lorebook_reference"])
			if result["status"] != tc.wantStatus || fake.readCount != tc.wantReads {
				t.Fatalf("result=%#v read_count=%d", result, fake.readCount)
			}
		})
	}
}

func TestPrepareTurnLorebookObservedScopeMustBeCompleteBeforeStoreRead(t *testing.T) {
	fake := &prepareTurnLorebookReferenceStore{Store: store.NewNoopStore()}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-g3-incomplete-scope",
		"raw_user_input":"한얼은 어디로 가야 할까?",
		"lorebook_reference_scope":{
			"contract_version":"lorebook_reference_scope.v1",
			"observation_state":"observed",
			"enabled_module_ids":[],
			"enabled_modules_observed":true
		},
		"settings":{"guide_strength":"none","lorebook_reference_mode":"reference_assist"}
	}`)
	result := mapFromAny(response["lorebook_reference"])
	if result["status"] != "unavailable" || result["reason_code"] != "lorebook_scope_observation_incomplete" {
		t.Fatalf("incomplete observed scope was accepted: %#v", result)
	}
	if fake.readCount != 0 {
		t.Fatalf("incomplete observed scope read lorebook store %d times", fake.readCount)
	}
}

func TestPrepareTurnLorebookReferenceAssistAddsSeparateLaneAndPublisherSupport(t *testing.T) {
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 19, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "han-profile", EntryOrdinal: 0, Key: "한얼", Content: "한얼의 신분은 양반 유생이다."},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-g6",
		"raw_user_input":"한얼은 이제 어디로 가야 할까?",
		"lorebook_reference_scope":{
			"contract_version":"lorebook_reference_scope.v1",
			"observation_state":"observed",
			"character_index":1,
			"chat_index":2,
			"enabled_module_ids":[],
			"enabled_modules_observed":true
		},
		"settings":{
			"guide_strength":"none",
			"max_injection_chars":9000,
			"reference_injection_budget_basis_chars":9000,
			"lorebook_reference_mode":"reference_assist"
		}
	}`)

	result := mapFromAny(response["lorebook_reference"])
	if result["status"] != "ready" || intFromAny(result["delivery_count"], 0) != 1 || intFromAny(result["publisher_count"], 0) != 1 {
		t.Fatalf("reference assist result=%#v", result)
	}
	plan := mapFromAny(response["payload_application_plan"])
	if !strings.Contains(extractionStringFromAny(plan["auxiliary_text"]), "한얼의 신분은 양반 유생이다.") {
		t.Fatalf("lorebook text was not delivered in the payload plan: %#v", plan)
	}
	var lorebookLane map[string]any
	for _, raw := range outputFidelityLineageSlice(plan["lanes"]) {
		lane := mapFromAny(raw)
		if extractionStringFromAny(lane["key"]) == "lorebook_reference" {
			lorebookLane = lane
			break
		}
	}
	if lorebookLane == nil || !boolFromAny(lorebookLane["applied"]) || intFromAny(lorebookLane["used_chars"], 0) == 0 {
		t.Fatalf("separate lorebook lane missing: %#v", plan["lanes"])
	}
	supervisorPack := mapFromAny(response["supervisor_input_pack"])
	support := mapFromAny(supervisorPack["support_packet"])
	items := outputFidelityLineageSlice(support["delivered_lorebook_reference"])
	if len(items) != 1 || extractionStringFromAny(mapFromAny(items[0])["final_text"]) != "한얼의 신분은 양반 유생이다." {
		t.Fatalf("publisher did not receive exactly the delivered lorebook item: %#v", support)
	}
}

func TestPrepareTurnLorebookReferenceAssistSuppressesOnlyExactDisplayedDuplicate(t *testing.T) {
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 20, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "native-copy", EntryOrdinal: 0, Key: "한얼", Content: "한얼의 신분은 양반 유생이다."},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-g5-native",
		"raw_user_input":"한얼은 무엇을 할까?",
		"messages":[{"role":"system","content":"한얼의 신분은 양반 유생이다."}],
		"lorebook_reference_scope":{
			"contract_version":"lorebook_reference_scope.v1",
			"observation_state":"observed",
			"character_index":1,"chat_index":2,
			"enabled_module_ids":[],"enabled_modules_observed":true
		},
		"settings":{"guide_strength":"none","max_injection_chars":9000,"reference_injection_budget_basis_chars":9000,"lorebook_reference_mode":"reference_assist"}
	}`)
	result := mapFromAny(response["lorebook_reference"])
	if intFromAny(result["delivery_count"], -1) != 0 || intFromAny(result["duplicate_suppressed_count"], 0) != 1 {
		t.Fatalf("exact native duplicate was not display-only suppressed: %#v", result)
	}
	plan := mapFromAny(response["payload_application_plan"])
	if strings.Contains(extractionStringFromAny(plan["auxiliary_text"]), "한얼의 신분은 양반 유생이다.") {
		t.Fatalf("Archive lorebook copy remained duplicated in auxiliary text: %#v", plan)
	}
}

func TestFinalizeLorebookReferenceKeepsSimilarTextAndAggregatesOnlyExactLoreCopies(t *testing.T) {
	result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
	result.Status = "ready"
	result.ScopeStatus = "observed"
	result.candidates = []prepareTurnLorebookCandidate{
		{Entry: store.LorebookReferenceEntryObservation{Content: "한얼은 유생이다."}, EntryRef: "host_entry:first", Methods: []string{"key"}},
		{Entry: store.LorebookReferenceEntryObservation{Content: " 한얼은   유생이다. "}, EntryRef: "host_entry:second", Methods: []string{"key"}},
	}
	finalizePrepareTurnLorebookReference(
		&result,
		"한얼의 다음 행동은?",
		nil,
		[]string{"한얼은 유생으로서 과거 시험을 준비한다."},
		true,
		9000,
	)
	if result.DeliveryCount != 1 || result.DuplicateCount != 1 {
		t.Fatalf("exact lore copies were not grouped once: %#v", result)
	}
	if len(result.delivered) != 1 || len(result.delivered[0].SourceRefs) != 2 {
		t.Fatalf("exact duplicate provenance was lost: %#v", result.delivered)
	}
	if !strings.Contains(result.deliveryText, "한얼은 유생이다.") {
		t.Fatalf("similar memory text incorrectly removed lorebook text: %q", result.deliveryText)
	}
}

func TestFinalizeLorebookReferenceDoesNotPartiallyCutOversizedItem(t *testing.T) {
	result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
	result.Status = "ready"
	result.ScopeStatus = "observed"
	result.candidates = []prepareTurnLorebookCandidate{{
		Entry:    store.LorebookReferenceEntryObservation{Content: "한얼은 과거 시험을 준비하는 양반 유생이다."},
		EntryRef: "host_entry:large",
		Methods:  []string{"key"},
	}}
	finalizePrepareTurnLorebookReference(&result, "한얼은 무엇을 할까?", nil, nil, true, 8)
	if result.DeliveryCount != 0 || result.DeferredCount != 1 || result.deliveryText != "" {
		t.Fatalf("oversized lorebook item was partially cut or force-filled: %#v text=%q", result, result.deliveryText)
	}
}

func TestPrepareTurnLorebookReferenceAssistDefersLexicalOnlyCandidateWithoutDelivery(t *testing.T) {
	const loreText = "Han-eol prepares for the examination in the eastern hall."
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 31, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "lexical-only", EntryOrdinal: 0, Content: loreText},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-on-demand-lexical",
		"raw_user_input":"What examination is Han-eol preparing for?",
		"response_projection":"prepare_turn.production_compact.v1",
		"lorebook_reference_scope":{
			"contract_version":"lorebook_reference_scope.v1",
			"observation_state":"observed",
			"character_index":1,
			"chat_index":2,
			"enabled_module_ids":[],
			"enabled_modules_observed":true
		},
		"settings":{
			"guide_strength":"none",
			"max_injection_chars":9000,
			"reference_injection_budget_basis_chars":9000,
			"lorebook_reference_mode":"reference_assist"
		}
	}`)

	result := mapFromAny(response["lorebook_reference"])
	if result["status"] != "deferred" || result["reason_code"] != "lorebook_reference_not_directly_activated" ||
		intFromAny(result["candidate_count"], 0) != 1 || intFromAny(result["deferred_count"], 0) != 1 ||
		intFromAny(result["delivery_count"], -1) != 0 || intFromAny(result["publisher_count"], -1) != 0 {
		t.Fatalf("lexical-only lorebook candidate was not retained as deferred: %#v", result)
	}
	plan := mapFromAny(response["payload_application_plan"])
	if strings.Contains(extractionStringFromAny(plan["auxiliary_text"]), loreText) {
		t.Fatalf("lexical-only lorebook candidate was injected: %#v", plan)
	}
	support := mapFromAny(mapFromAny(response["supervisor_input_pack"])["support_packet"])
	if len(outputFidelityLineageSlice(support["delivered_lorebook_reference"])) != 0 {
		t.Fatalf("deferred lorebook candidate reached Publisher: %#v", support)
	}
	criticJSON, err := json.Marshal(response["critic_input_pack"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(criticJSON), loreText) {
		t.Fatalf("deferred lorebook candidate reached Critic: %s", criticJSON)
	}
}

func TestFinalizeLorebookReferenceAlwaysActiveCanSupplyMissingContext(t *testing.T) {
	alwaysActive := true
	result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
	result.Status = "ready"
	result.ScopeStatus = "observed"
	result.candidates = []prepareTurnLorebookCandidate{{
		Entry: store.LorebookReferenceEntryObservation{
			Content:      "The eastern archive opens only at dawn.",
			AlwaysActive: &alwaysActive,
		},
		EntryRef: "host_entry:always-active",
		Methods:  []string{"lexical"},
	}}

	finalizePrepareTurnLorebookReference(&result, "Continue at the eastern archive.", nil, nil, true, 9000)
	if result.Status != "ready" || result.DeliveryCount != 1 || result.DeferredCount != 0 {
		t.Fatalf("observed always-active lorebook context was not supplied: %#v", result)
	}
}

func TestPrepareTurnLorebookSearchIncludesAlwaysActiveWithoutQueryOverlap(t *testing.T) {
	alwaysActive := true
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{
			ScopeID: 42,
			Entries: []store.LorebookReferenceEntryObservation{{
				HostEntryID: "always-active", EntryOrdinal: 0,
				Content: "The eastern archive opens only at dawn.", AlwaysActive: &alwaysActive,
			}},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	characterIndex, chatIndex := int64(1), int64(2)
	result := srv.prepareTurnLorebookReferenceSearch(context.Background(), "lore-always", "A completely unrelated request.", prepareTurnLorebookModeReferenceAssist, &dto.PrepareTurnLorebookReferenceScopeV1{
		ContractVersion:        prepareTurnLorebookScopeContractV1,
		ObservationState:       "observed",
		CharacterIndex:         &characterIndex,
		ChatIndex:              &chatIndex,
		EnabledModuleIDs:       []string{},
		EnabledModulesObserved: true,
	})
	if result.CandidateCount != 1 || intFromAny(result.MethodCounts["always_active"], 0) != 1 {
		t.Fatalf("always-active entry was not recognized without query overlap: %#v", result)
	}
	finalizePrepareTurnLorebookReference(&result, "A completely unrelated request.", nil, nil, true, 9000)
	if result.DeliveryCount != 1 || result.Status != "ready" {
		t.Fatalf("recognized always-active entry was not delivered: %#v", result)
	}
}

func TestFinalizeLorebookReferenceSuppressesOnlyActuallyDeliveredDuplicate(t *testing.T) {
	newResult := func() prepareTurnLorebookReferenceResult {
		result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
		result.Status = "ready"
		result.ScopeStatus = "observed"
		result.candidates = []prepareTurnLorebookCandidate{{
			Entry:    store.LorebookReferenceEntryObservation{Content: "Han-eol carries the bronze pass."},
			EntryRef: "host_entry:bronze-pass",
			Methods:  []string{"key"},
		}}
		return result
	}

	deliveredMemory := newResult()
	finalizePrepareTurnLorebookReference(
		&deliveredMemory,
		"What does Han-eol carry?",
		nil,
		[]string{"Han-eol carries the bronze pass."},
		true,
		9000,
	)
	if deliveredMemory.DeliveryCount != 0 || deliveredMemory.DuplicateCount != 1 {
		t.Fatalf("actually delivered duplicate was not suppressed from lorebook only: %#v", deliveredMemory)
	}

	selectedButUndelivered := newResult()
	finalizePrepareTurnLorebookReference(
		&selectedButUndelivered,
		"What does Han-eol carry?",
		nil,
		nil,
		true,
		9000,
	)
	if selectedButUndelivered.DeliveryCount != 1 || selectedButUndelivered.DuplicateCount != 0 {
		t.Fatalf("an undelivered memory incorrectly suppressed the lorebook fallback: %#v", selectedButUndelivered)
	}
}

func TestFinalizeLorebookReferencePreservesNoCandidateReason(t *testing.T) {
	for _, tc := range []struct {
		name             string
		injectionEnabled bool
		budgetChars      int
	}{
		{name: "injection disabled", injectionEnabled: false, budgetChars: 9000},
		{name: "budget empty", injectionEnabled: true, budgetChars: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
			result.Status = "empty"
			result.ReasonCode = "lorebook_no_relevant_candidates"
			result.ScopeStatus = "observed"

			finalizePrepareTurnLorebookReference(
				&result,
				"Continue.",
				nil,
				nil,
				tc.injectionEnabled,
				tc.budgetChars,
			)
			if result.Status != "empty" || result.ReasonCode != "lorebook_no_relevant_candidates" {
				t.Fatalf("no-candidate diagnosis was overwritten: %#v", result)
			}
		})
	}
}

func TestPrepareTurnDeferredLorebookDoesNotChangeMemoryPlanOrLineage(t *testing.T) {
	request := func(mode string, fake *prepareTurnLorebookReferenceStore) map[string]any {
		t.Helper()
		srv := setupTestServer()
		srv.Store = fake
		_, response := prepareTurnPerfRequest(t, srv, `{
			"chat_session_id":"lore-memory-invariant",
			"raw_user_input":"What examination is Han-eol preparing for?",
			"response_projection":"prepare_turn.production_compact.v1",
			"lorebook_reference_scope":{
				"contract_version":"lorebook_reference_scope.v1",
				"observation_state":"observed",
				"character_index":1,
				"chat_index":2,
				"enabled_module_ids":[],
				"enabled_modules_observed":true
			},
			"settings":{"guide_strength":"none","lorebook_reference_mode":"`+mode+`"}
		}`)
		return mapFromAny(response["injection_pack"])
	}

	searchOnly := request("search_only", &prepareTurnLorebookReferenceStore{Store: store.NewNoopStore()})
	assist := request("reference_assist", &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 32, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "lexical-only", EntryOrdinal: 0, Content: "Han-eol prepares for the examination in the eastern hall."},
		}},
	})
	for _, key := range []string{"memory_delivery_plan", "memory_delivery_lineage", "memory_recall_plan"} {
		if !reflect.DeepEqual(searchOnly[key], assist[key]) {
			t.Fatalf("deferred lorebook changed %s\nsearch_only=%#v\nassist=%#v", key, searchOnly[key], assist[key])
		}
	}
}

func TestPrepareTurnGuideEligibilityUsesOnlyDeliveredLorebookSupport(t *testing.T) {
	request := func(sessionID string, entry store.LorebookReferenceEntryObservation) map[string]any {
		t.Helper()
		srv := setupTestServer()
		srv.Store = &prepareTurnLorebookReferenceStore{
			Store: store.NewNoopStore(),
			current: &store.LorebookReferenceCurrent{
				ScopeID: 41,
				Entries: []store.LorebookReferenceEntryObservation{entry},
			},
		}
		_, response := prepareTurnPerfRequest(t, srv, `{
			"chat_session_id":"`+sessionID+`",
			"raw_user_input":"Continue near the eastern archive.",
			"lorebook_reference_scope":{
				"contract_version":"lorebook_reference_scope.v1",
				"observation_state":"observed",
				"character_index":1,
				"chat_index":2,
				"enabled_module_ids":[],
				"enabled_modules_observed":true
			},
			"settings":{
				"guide_mode":"standard",
				"guide_strength":"weak",
				"injection_enabled":true,
				"max_injection_chars":9000,
				"reference_injection_budget_basis_chars":9000,
				"lorebook_reference_mode":"reference_assist"
			}
		}`)
		return response
	}

	alwaysActive := true
	delivered := request("lore-guide-delivered", store.LorebookReferenceEntryObservation{
		HostEntryID: "always-only", EntryOrdinal: 0,
		Content: "The eastern archive opens only at dawn.", AlwaysActive: &alwaysActive,
	})
	deliveredLorebook := mapFromAny(delivered["lorebook_reference"])
	deliveredEligibility := mapFromAny(mapFromAny(delivered["payload_application_plan"])["guide_eligibility"])
	if intFromAny(deliveredLorebook["delivery_count"], 0) != 1 || deliveredEligibility["status"] != "eligible" ||
		!stringSliceContains(stringSliceFromAny(deliveredEligibility["source_refs"]), "host_entry:always-only") {
		t.Fatalf("delivered lorebook-only support was not guide eligible: lorebook=%#v eligibility=%#v", deliveredLorebook, deliveredEligibility)
	}
	deliveredSourceRefs := mapFromAny(mapFromAny(delivered["response_execution_contract"])["source_refs"])
	if !stringSliceContains(stringSliceFromAny(deliveredSourceRefs["lorebook_reference"]), "host_entry:always-only") {
		t.Fatalf("delivered lorebook ref missing from execution contract: %#v", deliveredSourceRefs)
	}

	deferred := request("lore-guide-deferred", store.LorebookReferenceEntryObservation{
		HostEntryID: "lexical-only", EntryOrdinal: 0,
		Content: "The eastern archive contains the old examination register.",
	})
	deferredLorebook := mapFromAny(deferred["lorebook_reference"])
	deferredEligibility := mapFromAny(mapFromAny(deferred["payload_application_plan"])["guide_eligibility"])
	if intFromAny(deferredLorebook["candidate_count"], 0) != 1 || intFromAny(deferredLorebook["deferred_count"], 0) != 1 ||
		intFromAny(deferredLorebook["delivery_count"], -1) != 0 || deferredEligibility["status"] != "no_support" ||
		len(stringSliceFromAny(deferredEligibility["source_refs"])) != 0 {
		t.Fatalf("candidate or deferred lorebook became guide support: lorebook=%#v eligibility=%#v", deferredLorebook, deferredEligibility)
	}
	deferredSourceRefs := mapFromAny(mapFromAny(deferred["response_execution_contract"])["source_refs"])
	if len(stringSliceFromAny(deferredSourceRefs["lorebook_reference"])) != 0 {
		t.Fatalf("deferred lorebook ref reached execution contract: %#v", deferredSourceRefs)
	}
}
