package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestNormalizeNarrativeStateClaimsSeparatesFactAndBelief(t *testing.T) {
	extraction := map[string]any{
		"state_claims": []any{map[string]any{
			"subject": "A", "subject_type": "character", "state_slot": "life_status", "value": "alive", "evidence_excerpt": "A opened their eyes.",
		}},
		"belief_updates": []any{map[string]any{
			"perspective_owner": "B", "subject": "A", "state_slot": "life_status", "value": "dead", "evidence_excerpt": "B still believed A was dead.",
		}},
	}
	claims := normalizeNarrativeStateClaims(extraction)
	if len(claims) != 2 {
		t.Fatalf("claims=%d, want 2", len(claims))
	}
	if claims[0].ClaimScope != "objective" || claims[0].PerspectiveOwner != "" {
		t.Fatalf("objective claim normalized incorrectly: %#v", claims[0])
	}
	if claims[1].ClaimScope != "belief" || claims[1].PerspectiveOwner != "B" {
		t.Fatalf("belief claim normalized incorrectly: %#v", claims[1])
	}
}

func TestSaveNarrativeStateKeepsOneCurrentValueAndLinksEvidence(t *testing.T) {
	ctx := context.Background()
	st := &turnRecordingStore{}
	srv := &Server{Store: st}
	now := time.Now().UTC()
	evidence := []store.DirectEvidence{{ID: 77, ChatSessionID: "sess", TurnAnchor: 1, EvidenceText: "A was believed dead."}}
	first := map[string]any{"state_claims": []any{map[string]any{
		"subject": "A", "subject_type": "character", "state_slot": "life_status", "value": "dead", "transition": "set", "evidence_excerpt": "A was believed dead.",
	}}}
	result := artifactSaveResult{}
	srv.saveNarrativeStateFromExtraction(ctx, "sess", 1, first, "The witnesses lowered their heads. A was believed dead. The room fell silent.", evidence, now, &result)
	if len(st.returnStatusCurrent) != 1 || len(st.savedStatusEvents) != 1 {
		t.Fatalf("first write current=%d events=%d", len(st.returnStatusCurrent), len(st.savedStatusEvents))
	}
	var evidencePayload map[string]any
	if err := json.Unmarshal([]byte(st.returnStatusCurrent[0].EvidenceJSON), &evidencePayload); err != nil {
		t.Fatal(err)
	}
	ids, _ := evidencePayload["direct_evidence_ids"].([]any)
	if len(ids) != 1 || int(ids[0].(float64)) != 77 {
		t.Fatalf("evidence ids=%v, want [77]", evidencePayload["direct_evidence_ids"])
	}

	evidence = append(evidence, store.DirectEvidence{ID: 88, ChatSessionID: "sess", TurnAnchor: 2, EvidenceText: "A returned alive."})
	second := map[string]any{"state_claims": []any{map[string]any{
		"subject": "A", "subject_type": "character", "state_slot": "life_status", "value": "alive", "transition": "reversal", "evidence_excerpt": "A returned alive.",
	}}}
	srv.saveNarrativeStateFromExtraction(ctx, "sess", 2, second, "The door opened without warning. A returned alive. B dropped the cup.", evidence, now.Add(time.Minute), &result)
	if len(st.returnStatusCurrent) != 1 {
		t.Fatalf("current rows=%d, want one replaced row", len(st.returnStatusCurrent))
	}
	if len(st.savedStatusEvents) != 2 {
		t.Fatalf("events=%d, want 2", len(st.savedStatusEvents))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(st.returnStatusCurrent[0].ValueJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["value"] != "alive" || payload["previous_value"] != "dead" {
		t.Fatalf("current payload=%v", payload)
	}

	srv.saveNarrativeStateFromExtraction(ctx, "sess", 3, second, "Everyone saw it clearly. A returned alive. No one could deny it.", evidence, now.Add(2*time.Minute), &result)
	if len(st.savedStatusCurrent) != 2 || len(st.savedStatusEvents) != 2 {
		t.Fatalf("exact reaffirm should skip writes: current writes=%d events=%d", len(st.savedStatusCurrent), len(st.savedStatusEvents))
	}
}

func TestNarrativeStateInjectionDropsOffSceneBelief(t *testing.T) {
	values := []store.StatusCurrentValue{
		narrativeTestCurrentValue("A", "life_status", "alive", "objective", "", 10),
		narrativeTestCurrentValue("A", "life_status", "dead", "belief", "B", 9),
		narrativeTestCurrentValue("A", "life_status", "dangerous", "belief", "C", 8),
	}
	chatLogs := []store.ChatLog{{TurnIndex: 10, Role: "assistant", Content: "A met B at the gate."}}
	facts, perceptions, dropped := filterNarrativeCurrentStateViews(values, "B asks whether A survived.", chatLogs, nil)
	if len(facts) != 1 || len(perceptions) != 1 || dropped != 1 {
		t.Fatalf("facts=%d perceptions=%d dropped=%d", len(facts), len(perceptions), dropped)
	}
	if perceptions[0].Perspective != "B" {
		t.Fatalf("perspective=%q, want B", perceptions[0].Perspective)
	}
}

func TestCurrentStateBlocksOlderMemoryThatRepeatsPreviousValue(t *testing.T) {
	current := narrativeTestCurrentValue("A", "life_status", "alive", "objective", "", 20)
	var payload map[string]any
	if err := json.Unmarshal([]byte(current.ValueJSON), &payload); err != nil {
		t.Fatal(err)
	}
	payload["previous_value"] = "dead"
	current.ValueJSON = mustCompactJSON(payload)
	selection := prepareTurnMemoryLaneSelection{
		VectorRelevant: []store.Memory{{ID: 1, TurnIndex: 5, SummaryJSON: `{"turn_summary":"A is dead"}`}},
		Relevant:       []store.Memory{{ID: 2, TurnIndex: 6, SummaryJSON: `{"turn_summary":"A once trusted B"}`}},
		Trace:          map[string]any{},
	}
	filtered := filterMemorySelectionAgainstNarrativeCurrentState(selection, []store.StatusCurrentValue{current})
	if len(filtered.VectorRelevant) != 0 || len(filtered.Relevant) != 1 {
		t.Fatalf("filtered vector=%d relevant=%d", len(filtered.VectorRelevant), len(filtered.Relevant))
	}
	if intFromAny(filtered.Trace["superseded_state_memory_dropped_count"], 0) != 1 {
		t.Fatalf("trace=%v", filtered.Trace)
	}
}

func TestRestoreNarrativeCurrentStateUsesLatestRemainingLedgerValue(t *testing.T) {
	st := &turnRecordingStore{}
	claim := narrativeStateClaim{Subject: "A", SubjectType: "character", StateSlot: "life_status", ClaimScope: "objective"}
	ownerID := narrativeStateOwnerID(claim)
	dead := claim
	dead.Value = "dead"
	alive := claim
	alive.Value = "alive"
	st.savedStatusEvents = []store.StatusChangeEvent{
		{ID: 1, ChatSessionID: "sess", RegistryID: 9, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: ownerID, EventKind: "set", NewValueJSON: mustCompactJSON(narrativeStateValuePayload(dead, "", 4)), EvidenceJSON: `{}`, SourceTurn: 4},
		{ID: 2, ChatSessionID: "sess", RegistryID: 9, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: ownerID, EventKind: "change", NewValueJSON: mustCompactJSON(narrativeStateValuePayload(alive, "dead", 8)), EvidenceJSON: `{}`, SourceTurn: 8},
	}
	restored, err := restoreNarrativeCurrentStatesAfterRollback(context.Background(), st, "sess")
	if err != nil {
		t.Fatal(err)
	}
	if restored != 1 || len(st.returnStatusCurrent) != 1 {
		t.Fatalf("restored=%d current=%d", restored, len(st.returnStatusCurrent))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(st.returnStatusCurrent[0].ValueJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["value"] != "alive" || st.returnStatusCurrent[0].SourceTurn != 8 {
		t.Fatalf("restored payload=%v turn=%d", payload, st.returnStatusCurrent[0].SourceTurn)
	}
}

func TestPrepareTurnAssemblyAppendsOnlyNeededContinuityCorrection(t *testing.T) {
	values := []store.StatusCurrentValue{
		narrativeTestCurrentValueWithPrevious("A", "life_status", "alive", "dead", "objective", "", "reversal", 20),
		narrativeTestCurrentValue("A", "life_status", "dead", "belief", "C", 18),
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, values, nil)
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{ID: 2, TurnIndex: 19, SummaryJSON: `{"turn_summary":"A is dead"}`, Importance: 0.8}},
		nil, nil, []store.ChatLog{{TurnIndex: 20, Role: "assistant", Content: "A spoke with B."}}, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "B asks A what happened.", "default", nil, nil, nil, perspective,
	)
	correctionIndex := strings.Index(assembly.Text, "[Continuity Correction]")
	if correctionIndex < 0 || !strings.HasSuffix(assembly.Text, assembly.ContinuityCorrectionText) {
		t.Fatalf("continuity correction must follow the 2.5 memory assembly:\n%s", assembly.Text)
	}
	if strings.Contains(assembly.Text, "A is dead") {
		t.Fatalf("superseded memory must be removed before correction is appended:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.ContinuityCorrectionText, "A / life_status: alive") {
		t.Fatalf("current correction missing:\n%s", assembly.ContinuityCorrectionText)
	}
	if strings.Contains(assembly.Text, "C believes") {
		t.Fatalf("off-scene perspective leaked into injection:\n%s", assembly.Text)
	}
}

func TestContinuityCorrectionSkipsCurrentValueAlreadyPresentInRecentChat(t *testing.T) {
	values := []store.StatusCurrentValue{
		narrativeTestCurrentValueWithPrevious("A", "life_status", "alive", "dead", "objective", "", "reversal", 20),
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, values, nil)
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{ID: 2, TurnIndex: 10, SummaryJSON: `{"turn_summary":"A is dead"}`, Importance: 0.8}},
		nil, nil, []store.ChatLog{{TurnIndex: 20, Role: "assistant", Content: "A is alive and standing at the gate."}}, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "B watches A.", "default", nil, nil, nil, perspective,
	)
	if assembly.ContinuityCorrectionText != "" {
		t.Fatalf("recent chat already carries current truth; correction must stay empty:\n%s", assembly.ContinuityCorrectionText)
	}
	trace := mapFromAny(assembly.Counts["continuity_correction"])
	if intFromAny(trace["already_present_dropped"], 0) != 1 {
		t.Fatalf("trace=%v", trace)
	}
}

func TestContinuityCorrectionDoesNotInjectUnchangedRelevantStateWithoutConflict(t *testing.T) {
	values := []store.StatusCurrentValue{narrativeTestCurrentValue("A", "location", "market", "objective", "", 20)}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, values, nil)
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, []store.ChatLog{{TurnIndex: 20, Role: "assistant", Content: "A looks around."}}, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "A takes a breath.", "default", nil, nil, nil, perspective,
	)
	if assembly.ContinuityCorrectionText != "" {
		t.Fatalf("unchanged state without a conflicting recall must not become a standing prompt:\n%s", assembly.ContinuityCorrectionText)
	}
}

func TestCurrentQueryDoesNotPromoteOldUnrelatedMemoryByImportanceAlone(t *testing.T) {
	selection := selectPrepareTurnMemoryLanes([]store.Memory{
		{ID: 1, TurnIndex: 2, SummaryJSON: `{"turn_summary":"Ashley guarded the old tower"}`, Importance: 1.0},
		{ID: 2, TurnIndex: 40, SummaryJSON: `{"turn_summary":"The group reached the market"}`, Importance: 0.2},
	}, "Nive and Ingrid whisper nearby", 1)
	if len(selection.Deep) != 0 {
		t.Fatalf("query-present selection must not promote old memory by importance: %#v", selection.Deep)
	}
	if len(selection.Recent) != 1 || selection.Recent[0].ID != 2 {
		t.Fatalf("recent fallback=%#v, want newest memory id 2", selection.Recent)
	}
}

func narrativeTestCurrentValue(subject, slot, value, scope, perspective string, turn int) store.StatusCurrentValue {
	claim := narrativeStateClaim{Subject: subject, SubjectType: "character", StateSlot: slot, Value: value, ClaimScope: scope, PerspectiveOwner: perspective, Transition: "set", Confidence: 0.9}
	return store.StatusCurrentValue{ID: int64(turn), ChatSessionID: "sess", RegistryID: 1, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: narrativeStateOwnerID(claim), OwnerLabel: narrativeStateOwnerLabel(claim), ValueKind: "note", ValueJSON: mustCompactJSON(narrativeStateValuePayload(claim, "", turn)), EvidenceJSON: `{}`, SourceTurn: turn, WriteState: "current"}
}

func narrativeTestCurrentValueWithPrevious(subject, slot, value, previous, scope, perspective, transition string, turn int) store.StatusCurrentValue {
	claim := narrativeStateClaim{Subject: subject, SubjectType: "character", StateSlot: slot, Value: value, ClaimScope: scope, PerspectiveOwner: perspective, Transition: transition, Confidence: 0.9}
	return store.StatusCurrentValue{ID: int64(turn), ChatSessionID: "sess", RegistryID: 1, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: narrativeStateOwnerID(claim), OwnerLabel: narrativeStateOwnerLabel(claim), ValueKind: "note", ValueJSON: mustCompactJSON(narrativeStateValuePayload(claim, previous, turn)), EvidenceJSON: `{}`, SourceTurn: turn, WriteState: "current"}
}
