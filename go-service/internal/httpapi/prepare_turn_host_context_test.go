package httpapi

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func TestPrepareTurnRisuHostContextUsesOnlyObservedPayloadSpans(t *testing.T) {
	systemRole, userRole, assistantRole := "system", "user", "assistant"
	systemText := "Lorebook-looking prose must not become a lorebook attribution."
	userText := "current input"
	assistantText := "prior output"
	algorithm := "or1c_utf16_djb2.v1"
	payloadPath := `["messages"]`
	request := dto.PrepareTurnContractRequest{
		PrepareTurnRequest: dto.PrepareTurnRequest{ChatSessionID: "session-a"},
		HostObservations: &dto.PrepareTurnHostObservationsV1{
			ContractVersion:         prepareHostObservationsVersion,
			SessionID:               "session-a",
			RequestID:               "request-a",
			RequestType:             "model",
			PayloadPath:             &payloadPath,
			PayloadWritable:         true,
			PayloadObservationStage: "before_request_replacer",
			FinalPayloadObservation: "not_exposed",
			Payload: []dto.PrepareTurnMessageObservationV1{
				prepareHostContextTestObservation("payload:0", systemRole, systemText, algorithm, nil),
				prepareHostContextTestObservation("payload:1", userRole, userText, algorithm, nil),
				prepareHostContextTestObservation("payload:2", assistantRole, assistantText, algorithm, nil),
				prepareHostContextTestObservation("payload:3", systemRole, systemText, algorithm, nil),
			},
		},
	}

	snapshot := buildPrepareTurnRisuHostContextSnapshot(request, "session-a")
	if snapshot.Status != "degraded" || snapshot.ReasonCode != "host_context_final_payload_not_exposed" {
		t.Fatalf("snapshot status=%q reason=%q", snapshot.Status, snapshot.ReasonCode)
	}
	if snapshot.ObservedSpanCount != 4 || snapshot.ReferenceEligibleCount != 2 || len(snapshot.Spans) != 4 {
		t.Fatalf("snapshot counts=%+v", snapshot)
	}
	if snapshot.SourceMetadataState != "not_exposed" || snapshot.Spans[0].SourceKindState != "not_exposed" {
		t.Fatalf("prose was promoted to host metadata: %+v", snapshot.Spans[0])
	}
	if snapshot.Spans[0].PayloadPath == nil || *snapshot.Spans[0].PayloadPath != payloadPath || snapshot.Spans[0].SpanStart != 0 || snapshot.Spans[0].SpanEnd != len([]rune(systemText)) {
		t.Fatalf("exact locator was not preserved: %+v", snapshot.Spans[0])
	}

	evidence := buildPrepareTurnHostContextReferenceEvidence(snapshot)
	if evidence.SelectedCount != 1 || evidence.DuplicateCount != 1 || evidence.DeferredCount != 2 || len(evidence.Items) != 1 || len(evidence.Duplicates) != 1 {
		t.Fatalf("evidence=%+v", evidence)
	}
	item := evidence.Items[0]
	if item.SourceKind != "unknown" || item.SourceMetadataState != "not_exposed" || item.Authority != "request_scoped_reference_only" || item.CanonicalWriteAllowed {
		t.Fatalf("unknown source gained authority: %+v", item)
	}
	if item.ConflictState != "unobserved" || item.SessionDivergenceState != "unobserved" || item.ConflictReasonCode != "semantic_conflict_not_inferred" {
		t.Fatalf("semantic conflict was guessed: %+v", item)
	}
	if got := evidence.Duplicates[0].DuplicateObservationRefs; len(got) != 1 || got[0] != "payload:3" {
		t.Fatalf("duplicate lineage=%v", got)
	}

	encoded, err := json.Marshal(map[string]any{"snapshot": snapshot, "evidence": evidence})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), systemText) || strings.Contains(string(encoded), `"exact_text"`) {
		t.Fatalf("production projection copied host payload text: %s", encoded)
	}
}

func TestPrepareTurnRisuHostContextHonorsOnlyDirectMetadata(t *testing.T) {
	role, text, algorithm := "system", "direct source metadata", "or1c_utf16_djb2.v1"
	kind, id, revision := "module", "module-1", "revision-2"
	request := dto.PrepareTurnContractRequest{HostObservations: &dto.PrepareTurnHostObservationsV1{
		ContractVersion:         prepareHostObservationsVersion,
		SessionID:               "session-b",
		RequestID:               "request-b",
		PayloadObservationStage: "before_request_replacer",
		FinalPayloadObservation: "observed",
		Payload: []dto.PrepareTurnMessageObservationV1{
			prepareHostContextTestObservation("payload:0", role, text, algorithm, &dto.PrepareTurnHostSourceV1{Kind: &kind, ID: &id, Revision: &revision}),
		},
	}}
	snapshot := buildPrepareTurnRisuHostContextSnapshot(request, "session-b")
	if snapshot.Status != "ready" || snapshot.SourceMetadataState != "partially_observed" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	evidence := buildPrepareTurnHostContextReferenceEvidence(snapshot)
	if len(evidence.Items) != 1 || evidence.Items[0].SourceKind != kind || evidence.Items[0].HostSource == nil || evidence.Items[0].HostSource.ID == nil || *evidence.Items[0].HostSource.ID != id {
		t.Fatalf("direct metadata was not preserved: %+v", evidence)
	}
}

func TestPrepareTurnRisuHostContextRejectsMalformedHashWithoutBlockingMainLane(t *testing.T) {
	role, text, algorithm := "system", "observed text", "or1c_utf16_djb2.v1"
	observation := prepareHostContextTestObservation("payload:0", role, text, algorithm, nil)
	observation.ContentHash = stringPointer("or1c_wrong")
	request := dto.PrepareTurnContractRequest{HostObservations: &dto.PrepareTurnHostObservationsV1{
		ContractVersion: prepareHostObservationsVersion,
		SessionID:       "session-c",
		RequestID:       "request-c",
		Payload:         []dto.PrepareTurnMessageObservationV1{observation},
	}}
	snapshot := buildPrepareTurnRisuHostContextSnapshot(request, "session-c")
	if snapshot.Status != "degraded" || snapshot.ReasonCode != "host_context_observation_malformed_span_ignored" || len(snapshot.Spans) != 0 || snapshot.CanonicalWriteAllowed {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestPrepareTurnOfficialRisuInspectionBaseline(t *testing.T) {
	if prepareOfficialRisuInspectionCommit != "7bd1120a6ad6ef8e2e8d52fcd7c8a325b4b896af" || prepareOfficialRisuInspectionDate != "2026-07-23" {
		t.Fatalf("official RisuAI inspection baseline changed without an explicit review")
	}
}

func TestResponseExecutionSourceRulesAreDeterministicAndDoNotCopyNativeText(t *testing.T) {
	role, nativeText, algorithm := "system", "Arbitrary native constraint text without a known template.", "or1c_utf16_djb2.v1"
	request := dto.PrepareTurnContractRequest{HostObservations: &dto.PrepareTurnHostObservationsV1{
		ContractVersion:         prepareHostObservationsVersion,
		SessionID:               "session-rules",
		RequestID:               "request-rules",
		PayloadObservationStage: "before_request_replacer",
		FinalPayloadObservation: "observed",
		Payload: []dto.PrepareTurnMessageObservationV1{
			prepareHostContextTestObservation("payload:0", role, nativeText, algorithm, nil),
		},
	}}
	evidence := buildPrepareTurnHostContextReferenceEvidence(buildPrepareTurnRisuHostContextSnapshot(request, "session-rules"))
	currentRef := "input-hook:7"
	current := dto.PrepareTurnCurrentInputDecisionV1{SelectedObservationRef: &currentRef}

	first := buildResponseExecutionSourceRules(current, evidence)
	second := buildResponseExecutionSourceRules(current, evidence)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same source snapshot produced different rules:\nfirst=%#v\nsecond=%#v", first, second)
	}

	assertRule := func(name string, wantCount int, wantRef string) {
		t.Helper()
		section := mapFromAny(first[name])
		if intFromAny(section["count"], -1) != wantCount {
			t.Fatalf("%s count=%v, want %d", name, section["count"], wantCount)
		}
		items, ok := section["items"].([]map[string]any)
		if !ok || len(items) != wantCount {
			t.Fatalf("%s items=%#v", name, section["items"])
		}
		if wantCount > 0 && !containsString(stringSliceFromAny(items[0]["source_refs"]), wantRef) {
			t.Fatalf("%s source refs=%#v, want %q", name, items[0]["source_refs"], wantRef)
		}
	}
	assertRule("must_preserve", 1, "host-context:payload:0")
	assertRule("must_respond", 1, currentRef)
	assertRule("must_account", 1, currentRef)
	assertRule("must_not_assert", 1, "host-context:payload:0")

	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), nativeText) {
		t.Fatalf("compiled contract duplicated native payload text: %s", encoded)
	}
}

func prepareHostContextTestObservation(ref, role, text, algorithm string, source *dto.PrepareTurnHostSourceV1) dto.PrepareTurnMessageObservationV1 {
	return dto.PrepareTurnMessageObservationV1{
		ObservationRef: ref,
		SourceKind:     "before_request_payload",
		Role:           &role,
		RawContent:     &text,
		ContentHash:    stringPointer(prepareOR1CHash(text)),
		HashAlgorithm:  &algorithm,
		HostSource:     source,
		EvidenceState:  "observed",
	}
}
