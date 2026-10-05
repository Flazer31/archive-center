package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// This boundary records production-produced units/current values. It never
// connects to a database, vector service or provider. Pending query results are
// supplied from the actual current-value snapshot written by the state owner.
type originLineageRecordingStore struct {
	*turnRecordingStore
	units      []*store.PreciseMemoryUnit
	pendingIDs map[string]int64
}

func (s *originLineageRecordingStore) PreciseMemoryWritesEnabled() bool { return true }
func (s *originLineageRecordingStore) SavePreciseMemoryUnit(_ context.Context, p *store.PreciseMemoryUnit) (bool, error) {
	copy := *p
	s.units = append(s.units, &copy)
	return true, nil
}

func originLineagePending(t *testing.T, st *originLineageRecordingStore, key string) store.PendingThread {
	t.Helper()
	st.returnPendingThreads = nil
	var found *store.PendingThread
	for _, value := range st.returnStatusCurrent {
		if p := narrativePendingSnapshot(parseJSONMap(value.ValueJSON)); p != nil {
			// SQL materialization assigns a distinct persistent row ID per
			// logical thread. The generic recording Store leaves every ID zero;
			// model that external boundary without altering any claim inputs.
			if st.pendingIDs == nil {
				st.pendingIDs = map[string]int64{}
			}
			if st.pendingIDs[p.ThreadKey] == 0 {
				st.pendingIDs[p.ThreadKey] = int64(len(st.pendingIDs) + 1)
			}
			p.ID = st.pendingIDs[p.ThreadKey]
			st.returnPendingThreads = append(st.returnPendingThreads, *p)
			if stringFromMap(parseJSONMap(p.HookMetadataJSON), "lifecycle_key") == key {
				found = p
			}
		}
	}
	if found == nil {
		t.Fatalf("missing production pending snapshot for %s", key)
	}
	return *found
}

func originLineageEvidence(id int64, sid string, turn int, quote string) store.DirectEvidence {
	return store.DirectEvidence{ID: id, ChatSessionID: sid, SourceTurnStart: turn, SourceTurnEnd: turn,
		EvidenceText: quote, ArchiveState: "verified_direct", CaptureStage: "critic_extract",
		CaptureVerification: "verified", CommittedGate: "auto_grounded_excerpt"}
}

func TestGoalExactOriginKnowledgeLifecycleRetentionR2(t *testing.T) {
	const sid, key = "synthetic-origin", "orchard-promise"
	const quote = "Aster promised to preserve the orchard."
	const instruction = "Kite heard the instruction to stand by the gate."
	const public = "Aster discussed the orchard beside the open gate."
	content := "Before. " + quote + " " + instruction + " " + public + " After."
	scope := map[string]any{"known_by": []any{"Aster", "Lumen"}, "unknown_to": []any{"Kite", "Wren"}, "suspected_by": []any{"Sable"}, "misinformed_by": []any{"Vale"}, "revealed_to": []any{}}
	instructionScope := map[string]any{"known_by": []any{"Kite"}, "unknown_to": []any{"Wren"}}
	secret := map[string]any{"secret_kind": "promise", "owner": "Aster", "summary": "Orchard promise", "evidence_excerpt": quote, "knowledge_scope": scope, "disclosure_policy": "owner_private_until_revealed"}
	claim := func(k, value, excerpt, transition string) map[string]any {
		return map[string]any{"subject": "Orchard", "state_slot": "goal_status", "lifecycle_key": k, "value": value, "transition": transition, "evidence_excerpt": excerpt}
	}
	thread := func(k, description, excerpt string) map[string]any {
		return map[string]any{"title": "Orchard", "description": description, "lifecycle_key": k, "evidence_excerpt": excerpt}
	}
	ex := normalizeCriticExtraction(map[string]any{
		"protected_secrets": []any{secret, map[string]any{"secret_kind": "instruction", "owner": "Kite", "summary": "Gate instruction only", "evidence_excerpt": instruction, "knowledge_scope": instructionScope, "disclosure_policy": "owner_private_until_revealed"}},
		"state_claims":      []any{claim(key, "Promise active", quote, "create"), claim("gate-instruction", "Instruction heard", instruction, "create"), claim("public-orchard", "Public discussion", public, "create")},
		"pending_threads":   []any{thread(key, "Orchard promise continues", quote), thread("gate-instruction", "Gate instruction heard", instruction), thread("public-orchard", "Public orchard discussion", public)},
	})
	scope = mapFromAny(mapFromAny(sliceFromAny(ex["protected_secrets"])[0])["knowledge_scope"])
	instructionScope = mapFromAny(mapFromAny(sliceFromAny(ex["protected_secrets"])[1])["knowledge_scope"])
	before := mustCompactJSON(ex)
	st := &originLineageRecordingStore{turnRecordingStore: &turnRecordingStore{}}
	srv, result := &Server{Store: st}, artifactSaveResult{}
	evidence := []store.DirectEvidence{originLineageEvidence(41, sid, 2, quote), originLineageEvidence(42, sid, 2, instruction), originLineageEvidence(43, sid, 2, public)}
	ctx := acceptedPreciseMemoryContext("synthetic-origin-revision")
	srv.savePreciseMemoryUnitsFromExtraction(ctx, sid, 2, ex, content, evidence, nil, time.Unix(2, 0), &result)
	srv.saveNarrativeStateFromExtraction(ctx, sid, 2, ex, content, evidence, time.Unix(2, 0), &result)
	if result.Errors != 0 {
		t.Fatalf("unexpected boundary error: %v", result.ErrorDetails)
	}
	pending := originLineagePending(t, st, key)
	metadata := parseJSONMap(pending.HookMetadataJSON)
	boundary := originLineageQuoteBoundary(t, metadata, scope)
	origin := mapFromAny(boundary["knowledge_source"])
	if intFromAny(origin["root_evidence_id"], 0) != int(evidence[0].ID) || stringFromMap(origin, "source_revision") != "synthetic-origin-revision" || intFromAny(origin["source_span_start"], -1) != strings.Index(content, quote) {
		t.Fatalf("origin metadata lost: %v", origin)
	}
	if strings.Contains(mustCompactJSON(origin), quote) {
		t.Fatal("boundary metadata cloned the protected body")
	}
	publicPending := originLineagePending(t, st, "public-orchard")
	if len(mapFromAny(parseJSONMap(publicPending.HookMetadataJSON)["knowledge_scope"])) != 0 {
		t.Fatal("public same-source sibling acquired scope")
	}
	commandPending := originLineagePending(t, st, "gate-instruction")
	originLineageQuoteBoundary(t, parseJSONMap(commandPending.HookMetadataJSON), instructionScope)
	seen := map[string]bool{}
	for _, unit := range st.units {
		if unit.Kind != "state" {
			continue
		}
		payload := parseJSONMap(unit.PayloadJSON)
		k := stringFromMap(payload, "lifecycle_key")
		seen[k] = true
		if k == key {
			originLineageQuoteBoundary(t, payload, scope)
			if unit.Visibility != "restricted" {
				t.Fatal("precise goal lost quotation restriction")
			}
		}
		if k == "public-orchard" && (unit.Visibility != "public" || len(mapFromAny(payload["knowledge_scope"])) != 0) {
			t.Fatal("public precise sibling changed")
		}
	}
	if !seen[key] || !seen["public-orchard"] {
		t.Fatal("positive precise units absent")
	}
	if mustCompactJSON(ex) != before {
		t.Fatal("canonical extraction mutated")
	}
	if stringFromMap(mapFromAny(sliceFromAny(ex["protected_secrets"])[0]), "lifecycle_key") != "" {
		t.Fatal("protected fact acquired invented lifecycle")
	}
	progressQuote := "Lumen thanked Aster for saving the orchard."
	progress := normalizeCriticExtraction(map[string]any{"state_claims": []any{claim(key, "Promise progressed", progressQuote, "progress")}, "pending_threads": []any{thread(key, "Orchard promise continues", progressQuote)}})
	srv.saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("synthetic-progress-revision"), sid, 4, progress, "Before. "+progressQuote+" After.", []store.DirectEvidence{originLineageEvidence(44, sid, 4, progressQuote)}, time.Unix(4, 0), &result)
	pending = originLineagePending(t, st, key)
	metadata = parseJSONMap(pending.HookMetadataJSON)
	boundary = originLineageQuoteBoundary(t, metadata, scope)
	if pending.SourceTurn != 4 || mustCompactJSON(boundary["knowledge_source"]) != mustCompactJSON(origin) {
		t.Fatal("same lifecycle progression lost original provenance")
	}
	writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
	srv.saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("synthetic-reaffirm-revision"), sid, 5, progress, "Before. "+progressQuote+" After.", nil, time.Unix(5, 0), &result)
	if len(st.savedStatusCurrent) != writes || len(st.savedStatusEvents) != events {
		t.Fatal("retained metadata manufactured a new progression write")
	}
	out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{PendingThreads: []store.PendingThread{pending, publicPending, commandPending}, UserInput: pending.Description, TopK: 5, MaxChars: 36000, ProtectedSecretBudgetChars: 4000})
	prepareTurnResolvePrioritySourcePool(&out, pending.Description, []string{pending.Description}, 4, nil)
	facts, _ := multiAgentCandidatePool(&out)
	found := false
	var selected prepareTurnPriorityMemoryCandidate
	for _, candidate := range facts {
		if candidate.SourceTable == "pending_threads" && strings.Contains(candidate.CompleteText, pending.Description) {
			found = true
			selected = candidate
			if mustCompactJSON(candidate.KnowledgeBoundaries) != mustCompactJSON(metadata["knowledge_boundaries"]) {
				t.Fatalf("candidate lost original source scope/ref/provenance: got=%s want=%s", mustCompactJSON(candidate.KnowledgeBoundaries), mustCompactJSON(metadata["knowledge_boundaries"]))
			}
			if !strings.Contains(mustCompactJSON(prepareTurnPriorityCandidateMap(candidate, true)), "unknown_to") {
				t.Fatal("production candidate lost scope")
			}
		}
	}
	if !found {
		t.Fatal("production candidate positive control absent")
	}
	selection := &multiAgentSelection{Candidates: []prepareTurnPriorityMemoryCandidate{selected}, Roles: []multiAgentRoleResult{{Role: selected.Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{selected.CanonicalFactID}}}}}
	plan := renderPrepareTurnPriorityMemoryDeliveryPlan(&prepareTurnInjectionAssembly{Preprocessing: selection, ProtectedSecretBudgetChars: 4000}, 36000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: pending.Description, CurrentTurn: 4}, nil, nil, nil, nil)
	if !strings.Contains(stringFromMap(plan, "main_memory_text"), mustCompactJSON(boundary)) {
		t.Fatal("selected main reading lost exact source scope/ref/provenance")
	}
}

func originLineageQuoteBoundary(t *testing.T, payload, scope map[string]any) map[string]any {
	t.Helper()
	if len(mapFromAny(payload["knowledge_scope"])) > 0 || len(mapFromAny(payload["knowledge_source"])) > 0 {
		t.Fatal("source quotation was promoted to target-owned claim scope")
	}
	boundaries := sliceFromAny(payload["knowledge_boundaries"])
	if len(boundaries) != 1 {
		t.Fatalf("expected one individually attributed source quotation: %s", mustCompactJSON(boundaries))
	}
	boundary := mapFromAny(boundaries[0])
	if mustCompactJSON(boundary["knowledge_scope"]) != mustCompactJSON(scope) || stringFromMap(boundary, "protected_fact_ref") == "" {
		t.Fatal("original source scope or attribution lost or unioned")
	}
	return boundary
}

func TestGoalSuppliedPendingScopeAndExplicitUpdateR2(t *testing.T) {
	const sid, key = "synthetic-supplied", "canal-promise"
	scope := map[string]any{"known_by": []any{"Lumen"}, "unknown_to": []any{"Kite"}}
	st := &originLineageRecordingStore{turnRecordingStore: &turnRecordingStore{}}
	srv, result := &Server{Store: st}, artifactSaveResult{}
	claim := func(value, quote, transition string) map[string]any {
		return map[string]any{"subject": "Canal", "state_slot": "goal_status", "lifecycle_key": key, "value": value, "transition": transition, "evidence_excerpt": quote}
	}
	thread := map[string]any{"title": "Canal", "description": "Canal promise", "lifecycle_key": key, "knowledge_scope": scope}
	ex := map[string]any{"state_claims": []any{claim("active", "The canal promise began.", "create")}, "pending_threads": []any{thread}}
	srv.saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("supplied-1"), sid, 1, ex, "Before. The canal promise began. After.", nil, time.Unix(1, 0), &result)
	originLineagePending(t, st, key)
	progress := map[string]any{"state_claims": []any{claim("progress", "The canal promise progressed.", "progress")}, "pending_threads": []any{map[string]any{"title": "Canal", "description": "Canal promise", "lifecycle_key": key}}}
	srv.saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("supplied-2"), sid, 2, progress, "Before. The canal promise progressed. After.", nil, time.Unix(2, 0), &result)
	p := originLineagePending(t, st, key)
	if mustCompactJSON(parseJSONMap(p.HookMetadataJSON)["knowledge_scope"]) != mustCompactJSON(scope) {
		t.Fatal("already supplied pending scope lost on progression")
	}
	revealed := map[string]any{"known_by": []any{"Lumen", "Kite"}, "unknown_to": []any{}, "publicly_revealed": true}
	updated := claim("revealed", "The canal promise was announced.", "reveal")
	updated["knowledge_scope"] = revealed
	srv.saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("supplied-3"), sid, 3, map[string]any{"state_claims": []any{updated}}, "Before. The canal promise was announced. After.", nil, time.Unix(3, 0), &result)
	p = originLineagePending(t, st, key)
	if mustCompactJSON(parseJSONMap(p.HookMetadataJSON)["knowledge_scope"]) != mustCompactJSON(revealed) {
		t.Fatal("explicit supplied scope update was overridden or unioned")
	}
}

func TestGoalMissingOriginDoesNotInventScopeR2(t *testing.T) {
	for _, withAuthority := range []bool{false, true} {
		t.Run(map[bool]string{false: "unclassified_wording", true: "absent_accepted_root"}[withAuthority], func(t *testing.T) {
			quote := "Lumen privately promised to guard the hidden canal."
			ex := map[string]any{"state_claims": []any{map[string]any{"subject": "Canal", "state_slot": "goal_status", "lifecycle_key": "unlinked", "value": "Private promise", "transition": "create", "evidence_excerpt": quote}}, "pending_threads": []any{map[string]any{"title": "Canal", "lifecycle_key": "unlinked", "description": "Private promise", "evidence_excerpt": quote}}}
			if withAuthority {
				ex["protected_secrets"] = []any{map[string]any{"summary": "Canal", "disclosure_policy": "owner_private_until_revealed", "evidence_excerpt": quote, "knowledge_scope": map[string]any{"known_by": []any{"Lumen"}, "unknown_to": []any{"Kite"}}}}
			}
			st := &originLineageRecordingStore{turnRecordingStore: &turnRecordingStore{}}
			result := artifactSaveResult{}
			(&Server{Store: st}).saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("missing-root"), "synthetic-missing", 1, ex, "Before. "+quote+" After.", nil, time.Unix(1, 0), &result)
			p := originLineagePending(t, st, "unlinked")
			if len(mapFromAny(parseJSONMap(p.HookMetadataJSON)["knowledge_scope"])) > 0 {
				t.Fatal("missing original binding was invented from wording")
			}
			if len(sliceFromAny(parseJSONMap(p.HookMetadataJSON)["knowledge_boundaries"])) > 0 {
				t.Fatal("missing accepted source quotation manufactured a boundary")
			}
			if result.Errors != 0 || result.NarrativeCurrentStates == 0 {
				t.Fatal("normal persistence was rejected")
			}
		})
	}
}
