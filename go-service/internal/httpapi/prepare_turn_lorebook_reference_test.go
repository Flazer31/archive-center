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
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type prepareTurnLorebookReferenceStore struct {
	store.Store
	current   *store.LorebookReferenceCurrent
	readErr   error
	readCount int
	lastScope store.LorebookReferenceScope
}

type prepareTurnSplitReferenceBudgetStore struct {
	*referenceBindingHTTPStore
	current *store.LorebookReferenceCurrent
}

func (f *prepareTurnSplitReferenceBudgetStore) ApplyLorebookReferenceSnapshot(context.Context, *store.LorebookReferenceSnapshot) (*store.LorebookReferenceSnapshotResult, error) {
	return nil, errors.New("unexpected lorebook snapshot write during prepare-turn")
}

func (f *prepareTurnSplitReferenceBudgetStore) GetLorebookReferenceCurrent(context.Context, store.LorebookReferenceScope) (*store.LorebookReferenceCurrent, error) {
	if f.current == nil {
		return nil, store.ErrNotFound
	}
	return f.current, nil
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

func TestPrepareTurnLorebookSearchOnlyListsScopedCatalogWithoutDelivery(t *testing.T) {
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
	if result["status"] != "ready" || intFromAny(result["candidate_count"], 0) != 4 {
		t.Fatalf("lorebook search=%#v", result)
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

	legacyStore := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 2, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "legacy-entry", EntryOrdinal: 0, Key: "한얼", Content: "한얼은 유생이다"},
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
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 4, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "default-entry", EntryOrdinal: 0, Key: "한얼", Content: "한얼은 유생이다"},
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
	if result["status"] != "ready" || intFromAny(result["delivery_count"], 0) != 1 || intFromAny(result["publisher_count"], 0) != 1 || intFromAny(result["budget_chars"], 0) != 3000 {
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

func TestPrepareTurnLorebookReferenceAssistPreservesExplicitBudgetValues(t *testing.T) {
	request := func(sessionID, maxChars string) map[string]any {
		t.Helper()
		srv := setupTestServer()
		srv.Store = &prepareTurnLorebookReferenceStore{
			Store: store.NewNoopStore(),
			current: &store.LorebookReferenceCurrent{ScopeID: 119, Entries: []store.LorebookReferenceEntryObservation{
				{HostEntryID: "explicit-budget", EntryOrdinal: 0, Key: "archive", Content: "The archive opens only at dusk."},
			}},
		}
		_, response := prepareTurnPerfRequest(t, srv, `{
			"chat_session_id":"`+sessionID+`",
			"raw_user_input":"Return to the archive.",
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
				"lorebook_reference_mode":"reference_assist",
				"lorebook_reference_max_chars":`+maxChars+`
			}
		}`)
		return mapFromAny(response["lorebook_reference"])
	}

	t.Run("explicit 6000", func(t *testing.T) {
		result := request("lore-explicit-6000", "6000")
		if intFromAny(result["budget_chars"], 0) != 6000 || intFromAny(result["delivery_count"], 0) != 1 || result["status"] != "ready" {
			t.Fatalf("explicit 6000 lorebook budget was not preserved: %#v", result)
		}
	})
	t.Run("explicit zero", func(t *testing.T) {
		result := request("lore-explicit-zero", "0")
		if intFromAny(result["budget_chars"], -1) != 0 || intFromAny(result["delivery_count"], -1) != 0 ||
			result["status"] != "deferred" || result["reason_code"] != "lorebook_reference_budget_zero" {
			t.Fatalf("explicit zero lorebook budget was not preserved: %#v", result)
		}
	})
}

func TestPrepareTurnLorebookReferenceAssistUsesContentFallbackForStrongestMatch(t *testing.T) {
	const loreText = "달의 궤도와 별자리 관측 기록이다."
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 20, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "single-anchor", EntryOrdinal: 0, Content: loreText},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-single-context-anchor",
		"raw_user_input":"별자리 기록을 확인한다.",
		"lorebook_reference_scope":{
			"contract_version":"lorebook_reference_scope.v1",
			"observation_state":"observed",
			"character_index":1,
			"chat_index":2,
			"enabled_module_ids":[],
			"enabled_modules_observed":true
		},
		"settings":{"guide_strength":"none","lorebook_reference_mode":"reference_assist"}
	}`)

	result := mapFromAny(response["lorebook_reference"])
	if result["status"] != "ready" || intFromAny(result["candidate_count"], 0) != 1 ||
		intFromAny(result["selected_count"], -1) != 1 || intFromAny(result["delivery_count"], -1) != 1 ||
		intFromAny(result["no_context_match_count"], -1) != 0 {
		t.Fatalf("content fallback did not deliver its strongest match: %#v", result)
	}
	candidates := outputFidelityLineageSlice(result["candidate_refs"])
	if len(candidates) != 1 || intFromAny(mapFromAny(candidates[0])["context_overlap"], 0) <= 0 ||
		extractionStringFromAny(mapFromAny(candidates[0])["final_disposition"]) != "delivered" {
		t.Fatalf("content-fallback selection observation=%#v", candidates)
	}
	plan := mapFromAny(response["payload_application_plan"])
	if !strings.Contains(extractionStringFromAny(plan["auxiliary_text"]), loreText) {
		t.Fatalf("content-fallback lorebook entry did not reach the payload plan: %#v", plan)
	}
	support := mapFromAny(mapFromAny(response["supervisor_input_pack"])["support_packet"])
	items := outputFidelityLineageSlice(support["delivered_lorebook_reference"])
	if len(items) != 1 {
		t.Fatalf("delivered content-fallback lorebook entry did not reach Publisher support: %#v", support)
	}
}

func TestFinalizeLorebookReferenceExcludesWeakerContextBelowFrontierWithSpareBudget(t *testing.T) {
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 120, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "strong-context", EntryOrdinal: 0, NormalizedSearch: "archive silver seal", Content: "The silver archive seal opens the restricted stacks."},
			{HostEntryID: "weak-context", EntryOrdinal: 1, NormalizedSearch: "archive route", Content: "The southern road passes an abandoned watchtower."},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	characterIndex, chatIndex := int64(1), int64(2)
	const query = "Inspect the archive silver seal before departure."
	result := srv.prepareTurnLorebookReferenceSearch(context.Background(), "lore-context-frontier", query, prepareTurnLorebookModeReferenceAssist, &dto.PrepareTurnLorebookReferenceScopeV1{
		ContractVersion:        prepareTurnLorebookScopeContractV1,
		ObservationState:       "observed",
		CharacterIndex:         &characterIndex,
		ChatIndex:              &chatIndex,
		EnabledModuleIDs:       []string{},
		EnabledModulesObserved: true,
	})
	finalizePrepareTurnLorebookReference(&result, query, nil, nil, true, 30000)
	if result.SelectedCount != 1 || result.DeliveryCount != 1 || result.DeferredCount != 1 ||
		result.FinalDispositionCounts["excluded_below_relevance_frontier"] != 1 {
		t.Fatalf("weaker context filled spare lorebook capacity: %#v", result)
	}
	dispositions := map[string]string{}
	for _, candidate := range result.CandidateRefs {
		dispositions[extractionStringFromAny(candidate["entry_ref"])] = extractionStringFromAny(candidate["final_disposition"])
	}
	if dispositions["host_entry:strong-context"] != "delivered" || dispositions["host_entry:weak-context"] != "excluded_below_relevance_frontier" {
		t.Fatalf("context frontier dispositions=%#v", dispositions)
	}
	stats := prepareTurnLorebookPayloadBudgetStats(result)
	if stats.ExclusionReason["lorebook_below_relevance_frontier"] != 1 {
		t.Fatalf("context frontier was absent from the payload budget ledger stats: %#v", stats)
	}
}

func TestFinalizeLorebookReferenceKeyFrontierExcludesContextOnlyGroupWithSpareBudget(t *testing.T) {
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 121, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "direct-key", EntryOrdinal: 0, Key: "eastern archive", Content: "The eastern archive opens only at dawn."},
			{HostEntryID: "context-only", EntryOrdinal: 1, NormalizedSearch: "eastern archive route", Content: "The western gate leads toward the old bridge."},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	characterIndex, chatIndex := int64(1), int64(2)
	const query = "Return to the eastern archive."
	result := srv.prepareTurnLorebookReferenceSearch(context.Background(), "lore-key-frontier", query, prepareTurnLorebookModeReferenceAssist, &dto.PrepareTurnLorebookReferenceScopeV1{
		ContractVersion:        prepareTurnLorebookScopeContractV1,
		ObservationState:       "observed",
		CharacterIndex:         &characterIndex,
		ChatIndex:              &chatIndex,
		EnabledModuleIDs:       []string{},
		EnabledModulesObserved: true,
	})
	finalizePrepareTurnLorebookReference(&result, query, nil, nil, true, 30000)
	if result.KeyMatchedCandidateCount != 1 || result.SelectedCount != 1 || result.DeliveryCount != 1 ||
		result.FinalDispositionCounts["excluded_below_relevance_frontier"] != 1 {
		t.Fatalf("context-only group crossed the direct-key frontier: %#v", result)
	}
	dispositions := map[string]string{}
	for _, candidate := range result.CandidateRefs {
		dispositions[extractionStringFromAny(candidate["entry_ref"])] = extractionStringFromAny(candidate["final_disposition"])
	}
	if dispositions["host_entry:direct-key"] != "delivered" || dispositions["host_entry:context-only"] != "excluded_below_relevance_frontier" {
		t.Fatalf("key frontier dispositions=%#v", dispositions)
	}
}

func TestPrepareTurnLorebookReferenceAssistUsesPreviousCompletedTurnWithoutFillingFromUnrelatedEntries(t *testing.T) {
	alwaysActive := true
	const relevantLore = "영산포는 세곡 물류의 중심이다."
	const unrelatedLore = "달의 궤도와 별자리 관측 기록이다."
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 21, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "previous-context", EntryOrdinal: 0, Key: "영산포", Content: relevantLore},
			{HostEntryID: "unrelated-always", EntryOrdinal: 1, Content: unrelatedLore, AlwaysActive: &alwaysActive},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	query := buildPrepareTurnLorebookSelectionQuery("도착하자마자 움직였다.", []store.ChatLog{
		{TurnIndex: 25, Role: "user", Content: "호남의 곡창을 살펴보자."},
		{TurnIndex: 25, Role: "assistant", Content: "영산포에서 세곡선을 확인했다."},
	}, 4000)
	characterIndex, chatIndex := int64(1), int64(2)
	result := srv.prepareTurnLorebookReferenceSearch(context.Background(), "lore-previous-completed-context", query, prepareTurnLorebookModeReferenceAssist, &dto.PrepareTurnLorebookReferenceScopeV1{
		ContractVersion:        prepareTurnLorebookScopeContractV1,
		ObservationState:       "observed",
		CharacterIndex:         &characterIndex,
		ChatIndex:              &chatIndex,
		EnabledModuleIDs:       []string{},
		EnabledModulesObserved: true,
	})
	finalizePrepareTurnLorebookReference(&result, "도착하자마자 움직였다.", nil, nil, true, 6000)
	if result.Status != "ready" || result.CandidateCount != 2 || result.ContextMatchedCandidateCount != 1 ||
		result.SelectedCount != 1 || result.DeliveryCount != 1 || result.NoContextMatchCount != 1 {
		t.Fatalf("previous-turn supplemental selection=%#v", result)
	}
	dispositions := map[string]string{}
	for _, candidate := range result.CandidateRefs {
		dispositions[extractionStringFromAny(candidate["entry_ref"])] = extractionStringFromAny(candidate["final_disposition"])
	}
	if dispositions["host_entry:previous-context"] != "delivered" || dispositions["host_entry:unrelated-always"] != "excluded_no_context_match" {
		t.Fatalf("previous-turn dispositions=%#v", dispositions)
	}
	if !strings.Contains(result.deliveryText, relevantLore) || strings.Contains(result.deliveryText, unrelatedLore) {
		t.Fatalf("supplement delivery included unrelated lorebook content: %q", result.deliveryText)
	}
}

func TestPrepareTurnOriginalWorkUsageDoesNotReduceLorebookBudget(t *testing.T) {
	referenceStore := newReferenceBindingHTTPStore()
	referenceStore.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Archive Work", Status: "ready"}}
	referenceStore.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Status: "active"}}
	referenceStore.timeline = []store.ReferenceTimelineNode{{NodeID: "node-budget", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Current", Ordinal: 1, BranchKey: "main", ReviewStatus: "approved"}}
	referenceStore.claims = []store.ReferenceClaim{{ClaimID: "claim-budget", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "The archive opens only at dusk.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved"}}
	referenceStore.bindings = []store.SessionReferenceBinding{{BindingID: "binding-budget", ChatSessionID: "split-budget", WorkID: "work-1", ContinuityID: "continuity-1", Enabled: true, InjectionEnabled: true, CurrentNodeID: "node-budget", ReferenceMode: referenceModePrimary}}

	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{{
		Document:   referenceRecallVectorDocument("claim", "claim-budget"),
		ChromaRank: 1,
	}}}
	srv := referenceRecallTestServer(referenceStore, vectorStore, embeddingServer.URL)
	srv.Store = &prepareTurnSplitReferenceBudgetStore{
		referenceBindingHTTPStore: referenceStore,
		current: &store.LorebookReferenceCurrent{ScopeID: 77, Entries: []store.LorebookReferenceEntryObservation{{
			HostEntryID: "lore-budget", EntryOrdinal: 0, Key: "archive", Content: "The archive keeper carries a silver seal.",
		}}},
	}

	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"split-budget",
		"raw_user_input":"What happens at the archive?",
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
			"reference_recall_limit":3,
			"lorebook_reference_mode":"reference_assist",
			"primary_canon_base_max_chars":3000
		}
	}`)

	referenceInjection := mapFromAny(response["reference_injection"])
	policy := mapFromAny(referenceInjection["budget_policy"])
	if policy["contract_version"] != "reference_injection_budget.v2" || intFromAny(policy["main_injection_cap_chars"], 0) != 18000 || intFromAny(policy["total_cap_chars"], 0) != 3000 || intFromAny(policy["used_chars"], 0) <= 0 {
		t.Fatalf("original-work budget was not independently applied: policy=%#v recall=%#v", policy, response["reference_recall"])
	}
	lorebook := mapFromAny(response["lorebook_reference"])
	if lorebook["status"] != "ready" || intFromAny(lorebook["budget_chars"], 0) != 3000 || intFromAny(lorebook["delivery_count"], 0) != 1 {
		t.Fatalf("original-work usage reduced the lorebook cap: %#v", lorebook)
	}
	payloadPlan := mapFromAny(response["payload_application_plan"])
	lanes := outputFidelityLineageSlice(payloadPlan["lanes"])
	budgets := map[string]int{}
	for _, raw := range lanes {
		lane := mapFromAny(raw)
		budgets[extractionStringFromAny(lane["key"])] = intFromAny(lane["budget_chars"], 0)
	}
	if budgets["original_work"] != 3000 || budgets["lorebook_reference"] != 3000 {
		t.Fatalf("payload lane budgets borrowed from each other: %#v", budgets)
	}
	ledger := mapFromAny(payloadPlan["budget_ledger"])
	if ledger["contract_version"] != "payload_budget_ledger.v1" || ledger["owner"] != "go" {
		t.Fatalf("payload budget ledger contract = %#v", ledger)
	}
	ledgerLanes := map[string]map[string]any{}
	configuredTotal := 0
	effectiveTotal := 0
	laneContentTotal := 0
	for _, raw := range outputFidelityLineageSlice(ledger["lanes"]) {
		lane := mapFromAny(raw)
		ledgerLanes[extractionStringFromAny(lane["key"])] = lane
		configuredTotal += intFromAny(lane["configured_cap_chars"], 0)
		effectiveTotal += intFromAny(lane["effective_cap_chars"], 0)
		laneContentTotal += intFromAny(lane["final_delivery_chars"], 0)
	}
	if intFromAny(ledgerLanes["long_term_memory"]["configured_cap_chars"], 0) != 18000 ||
		intFromAny(ledgerLanes["original_work"]["configured_cap_chars"], 0) != 3000 ||
		intFromAny(ledgerLanes["lorebook_reference"]["configured_cap_chars"], 0) != 3000 {
		t.Fatalf("independent configured caps = %#v", ledgerLanes)
	}
	if intFromAny(ledgerLanes["long_term_memory"]["effective_cap_chars"], 0) != 18000 ||
		intFromAny(ledgerLanes["original_work"]["effective_cap_chars"], 0) != 3000 ||
		intFromAny(ledgerLanes["lorebook_reference"]["effective_cap_chars"], 0) != 3000 ||
		intFromAny(ledgerLanes["output_guidance"]["effective_cap_chars"], -1) != 0 {
		t.Fatalf("effective caps did not preserve independent active lanes: %#v", ledgerLanes)
	}
	for _, key := range []string{"original_work", "lorebook_reference"} {
		lane := ledgerLanes[key]
		if intFromAny(lane["candidate_chars"], 0) <= 0 || intFromAny(lane["selected_chars"], 0) <= 0 || intFromAny(lane["final_delivery_chars"], 0) <= 0 {
			t.Fatalf("%s candidate-to-final ledger = %#v", key, lane)
		}
	}
	auxiliary := extractionStringFromAny(payloadPlan["auxiliary_text"])
	expectedFinal := len([]rune(prepareTurnAuxiliaryMessageHeader + "\n\n" + auxiliary))
	if intFromAny(ledger["configured_cap_chars"], 0) != configuredTotal ||
		intFromAny(ledger["effective_cap_chars"], 0) != effectiveTotal ||
		intFromAny(ledger["lane_content_chars"], 0) != laneContentTotal ||
		intFromAny(ledger["final_delivery_chars"], 0) != expectedFinal ||
		intFromAny(ledger["assembly_chars"], 0) != expectedFinal-laneContentTotal {
		t.Fatalf("payload title/separator ledger = %#v plan=%#v", ledger, payloadPlan)
	}
}

func TestPrepareTurnLorebookReferenceAssistSkipsRelevantEntryAlreadyPresentInRisuRequest(t *testing.T) {
	fake := &prepareTurnLorebookReferenceStore{
		Store: store.NewNoopStore(),
		current: &store.LorebookReferenceCurrent{ScopeID: 20, Entries: []store.LorebookReferenceEntryObservation{
			{HostEntryID: "native-copy", EntryOrdinal: 0, NormalizedSearch: "unrelated index", Content: "한얼의 신분은 양반 유생이다."},
		}},
	}
	srv := setupTestServer()
	srv.Store = fake
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"lore-g5-native",
		"raw_user_input":"무엇을 할까?",
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
	if intFromAny(result["delivery_count"], -1) != 0 || intFromAny(result["already_present_count"], 0) != 1 ||
		intFromAny(result["no_context_match_count"], -1) != 0 || result["reason_code"] != "lorebook_relevant_context_already_present" {
		t.Fatalf("Risu-delivered lorebook content was injected a second time: %#v", result)
	}
	candidates := outputFidelityLineageSlice(result["candidate_refs"])
	if len(candidates) != 1 || extractionStringFromAny(mapFromAny(candidates[0])["final_disposition"]) != "excluded_already_present" ||
		extractionStringFromAny(mapFromAny(candidates[0])["observed_source"]) != "risu_request_message" {
		t.Fatalf("already-present decision was not observable: %#v", candidates)
	}
	plan := mapFromAny(response["payload_application_plan"])
	if strings.Contains(extractionStringFromAny(plan["auxiliary_text"]), "한얼의 신분은 양반 유생이다.") {
		t.Fatalf("already-present lorebook content remained in the Archive Center lane: %#v", plan)
	}
}

func TestFinalizeLorebookReferenceCoalescesIdenticalSelectedEntriesWithoutLosingSources(t *testing.T) {
	result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
	result.Status = "ready"
	result.ScopeStatus = "observed"
	result.candidates = []prepareTurnLorebookCandidate{
		{Entry: store.LorebookReferenceEntryObservation{Content: "한얼은 유생이다."}, EntryRef: "host_entry:first", ContextOverlap: 1},
		{Entry: store.LorebookReferenceEntryObservation{Content: "한얼은 유생이다."}, EntryRef: "host_entry:second", ContextOverlap: 1},
	}
	finalizePrepareTurnLorebookReference(
		&result,
		"한얼의 다음 행동은?",
		nil,
		[]string{"한얼은 유생으로서 과거 시험을 준비한다."},
		true,
		9000,
	)
	if result.DeliveryCount != 1 || result.SelectedCount != 1 || result.CoalescedContentCount != 1 {
		t.Fatalf("identical selected lorebook entries were not coalesced once: %#v", result)
	}
	if len(result.delivered) != 1 || !reflect.DeepEqual(result.delivered[0].SourceRefs, []string{"host_entry:first", "host_entry:second"}) {
		t.Fatalf("coalesced lorebook provenance was lost: %#v", result.delivered)
	}
	if strings.Count(result.deliveryText, "한얼은 유생이다.") != 1 {
		t.Fatalf("identical lorebook content was delivered more than once: %q", result.deliveryText)
	}
}

func TestFinalizeLorebookReferenceDoesNotPartiallyCutOversizedItem(t *testing.T) {
	result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
	result.Status = "ready"
	result.ScopeStatus = "observed"
	result.candidates = []prepareTurnLorebookCandidate{{
		Entry:          store.LorebookReferenceEntryObservation{Content: "한얼은 과거 시험을 준비하는 양반 유생이다."},
		EntryRef:       "host_entry:large",
		ContextOverlap: 1,
	}}
	finalizePrepareTurnLorebookReference(&result, "한얼은 무엇을 할까?", nil, nil, true, 8)
	if result.DeliveryCount != 0 || result.DeferredCount != 1 || result.deliveryText != "" {
		t.Fatalf("oversized lorebook item was partially cut or force-filled: %#v text=%q", result, result.deliveryText)
	}
}

func TestFinalizeLorebookReferenceAlwaysActiveHasNoSeparateActivationGate(t *testing.T) {
	alwaysActive := true
	result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
	result.Status = "ready"
	result.ScopeStatus = "observed"
	result.candidates = []prepareTurnLorebookCandidate{{
		Entry: store.LorebookReferenceEntryObservation{
			Content:      "The eastern archive opens only at dawn.",
			AlwaysActive: &alwaysActive,
		},
		EntryRef:       "host_entry:always-active",
		ContextOverlap: 1,
	}}

	finalizePrepareTurnLorebookReference(&result, "Continue at the eastern archive.", nil, nil, true, 9000)
	if result.Status != "ready" || result.DeliveryCount != 1 || result.DeferredCount != 0 ||
		result.AlwaysActiveDeliveryCount != 1 {
		t.Fatalf("always-active lorebook entry did not pass through: %#v", result)
	}
	if len(result.CandidateRefs) != 0 {
		t.Fatalf("manual finalizer fixture unexpectedly exposed candidate refs: %#v", result.CandidateRefs)
	}
}

func TestPrepareTurnLorebookAlwaysActiveAloneDoesNotForceSupplementDelivery(t *testing.T) {
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
	result := srv.prepareTurnLorebookReferenceSearch(context.Background(), "lore-always", "Continue the scene.", prepareTurnLorebookModeReferenceAssist, &dto.PrepareTurnLorebookReferenceScopeV1{
		ContractVersion:        prepareTurnLorebookScopeContractV1,
		ObservationState:       "observed",
		CharacterIndex:         &characterIndex,
		ChatIndex:              &chatIndex,
		EnabledModuleIDs:       []string{},
		EnabledModulesObserved: true,
	})
	if result.CandidateCount != 1 || result.AlwaysActiveCandidateCount != 1 {
		t.Fatalf("always-active entry was not listed from the scoped catalog: %#v", result)
	}
	finalizePrepareTurnLorebookReference(&result, "Continue the scene.", nil, nil, true, 9000)
	if result.DeliveryCount != 0 || result.Status != "empty" || result.NoContextMatchCount != 1 {
		t.Fatalf("always-active flag forced unrelated supplemental delivery: %#v", result)
	}
	if result.AlwaysActiveDeliveryCount != 0 {
		t.Fatalf("always-active candidate/activation/delivery counts=%#v", result)
	}
	if len(result.CandidateRefs) != 1 || extractionStringFromAny(result.CandidateRefs[0]["final_disposition"]) != "excluded_no_context_match" {
		t.Fatalf("always-active final disposition missing: %#v", result.CandidateRefs)
	}
}

func TestFinalizeLorebookReferenceSkipsEntryAlreadyInArchiveCenterDelivery(t *testing.T) {
	result := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
	result.Status = "ready"
	result.ScopeStatus = "observed"
	result.candidates = []prepareTurnLorebookCandidate{{
		Entry:          store.LorebookReferenceEntryObservation{Content: "Han-eol carries the bronze pass."},
		EntryRef:       "host_entry:bronze-pass",
		ContextOverlap: 1,
	}}
	finalizePrepareTurnLorebookReference(
		&result,
		"What does Han-eol carry?",
		nil,
		[]string{"Han-eol carries the bronze pass."},
		true,
		9000,
	)
	if result.DeliveryCount != 0 || result.AlreadyPresentCount != 1 || result.ReasonCode != "lorebook_relevant_context_already_present" {
		t.Fatalf("Archive Center-delivered content was injected again: %#v", result)
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

func TestPrepareTurnLorebookDeliveryDoesNotChangeMemoryPlanOrLineage(t *testing.T) {
	request := func(mode string, fake *prepareTurnLorebookReferenceStore) map[string]any {
		t.Helper()
		srv := setupTestServer()
		srv.Store = fake
		_, response := prepareTurnPerfRequest(t, srv, `{
			"chat_session_id":"lore-memory-invariant",
			"raw_user_input":"과거 이야기를 계속해줘.",
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
			{HostEntryID: "lexical-only", EntryOrdinal: 0, Content: "한얼은 동쪽 서고에서 과거 시험을 준비한다."},
		}},
	})
	for _, key := range []string{"memory_delivery_plan", "memory_delivery_lineage", "memory_recall_plan"} {
		if !reflect.DeepEqual(searchOnly[key], assist[key]) {
			t.Fatalf("lorebook delivery changed %s\nsearch_only=%#v\nassist=%#v", key, searchOnly[key], assist[key])
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

	delivered := request("lore-guide-delivered", store.LorebookReferenceEntryObservation{
		HostEntryID: "key-match", EntryOrdinal: 0, Key: "eastern archive",
		Content: "The eastern archive opens only at dawn.",
	})
	deliveredLorebook := mapFromAny(delivered["lorebook_reference"])
	deliveredEligibility := mapFromAny(mapFromAny(delivered["payload_application_plan"])["guide_eligibility"])
	if intFromAny(deliveredLorebook["delivery_count"], 0) != 1 || deliveredEligibility["status"] != "eligible" ||
		!stringSliceContains(stringSliceFromAny(deliveredEligibility["source_refs"]), "host_entry:key-match") {
		t.Fatalf("delivered lorebook-only support was not guide eligible: lorebook=%#v eligibility=%#v", deliveredLorebook, deliveredEligibility)
	}
	deliveredSourceRefs := mapFromAny(mapFromAny(delivered["response_execution_contract"])["source_refs"])
	if !stringSliceContains(stringSliceFromAny(deliveredSourceRefs["lorebook_reference"]), "host_entry:key-match") {
		t.Fatalf("delivered lorebook ref missing from execution contract: %#v", deliveredSourceRefs)
	}

	unmatched := request("lore-guide-unmatched", store.LorebookReferenceEntryObservation{
		HostEntryID: "lexical-only", EntryOrdinal: 0, NormalizedSearch: "sealed registry",
		Content: "The old examination register remains sealed.",
	})
	unmatchedLorebook := mapFromAny(unmatched["lorebook_reference"])
	unmatchedEligibility := mapFromAny(mapFromAny(unmatched["payload_application_plan"])["guide_eligibility"])
	if intFromAny(unmatchedLorebook["candidate_count"], 0) != 1 || intFromAny(unmatchedLorebook["no_context_match_count"], 0) != 1 ||
		intFromAny(unmatchedLorebook["delivery_count"], -1) != 0 || unmatchedEligibility["status"] != "no_support" ||
		len(stringSliceFromAny(unmatchedEligibility["source_refs"])) != 0 {
		t.Fatalf("unmatched lorebook became delivered guide support: lorebook=%#v eligibility=%#v", unmatchedLorebook, unmatchedEligibility)
	}
	unmatchedSourceRefs := mapFromAny(mapFromAny(unmatched["response_execution_contract"])["source_refs"])
	if len(stringSliceFromAny(unmatchedSourceRefs["lorebook_reference"])) != 0 {
		t.Fatalf("unmatched lorebook ref reached execution contract: %#v", unmatchedSourceRefs)
	}
}
