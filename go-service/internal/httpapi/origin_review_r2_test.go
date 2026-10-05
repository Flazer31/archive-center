package httpapi

import (
	"fmt"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"testing"
	"time"
)

const reviewR2SID = "synthetic-origin-review-r2"
const reviewR2Quote = "Coordinator orders Courier to deliver a sealed parcel; only Coordinator and Auditor know its contents; Courier knows the delivery instruction."

func reviewR2Scope(known, unknown []string) map[string]any {
	return map[string]any{"known_by": known, "unknown_to": unknown}
}
func reviewR2PlanScope() map[string]any {
	return reviewR2Scope([]string{"Coordinator", "Auditor"}, []string{"Courier", "Outsider"})
}
func reviewR2InstructionScope() map[string]any {
	return reviewR2Scope([]string{"Coordinator", "Courier"}, []string{"Outsider"})
}
func reviewR2Secret(key, statement string, scope map[string]any) map[string]any {
	return map[string]any{"secret_id": key, "lifecycle_key": key, "owner": "Coordinator", "subject": statement, "summary": statement, "disclosure_policy": "owner_private_until_revealed", "knowledge_scope": scope, "evidence_excerpt": reviewR2Quote}
}
func reviewR2Claim(key, value, transition, quote string, scope map[string]any) map[string]any {
	c := map[string]any{"subject": "Parcel delivery instruction", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": key, "value": value, "transition": transition, "confidence": 0.99, "evidence_excerpt": quote}
	if scope != nil {
		c["knowledge_scope"] = scope
	}
	return c
}
func reviewR2Evidence(turn int, quote string) []store.DirectEvidence {
	return []store.DirectEvidence{{ID: 700, ChatSessionID: reviewR2SID, EvidenceText: quote, SourceTurnStart: turn, SourceTurnEnd: turn, TurnAnchor: turn, ArchiveState: "verified_direct", CaptureStage: "critic_extract", CaptureVerification: "verified", CommittedGate: "auto_grounded_excerpt"}}
}
func reviewR2Run(t *testing.T, st *turnRecordingStore, turn int, ex map[string]any, quote string) (map[string]any, map[string]any, map[string]any) {
	t.Helper()
	before := mustCompactJSON(ex)
	normalized := normalizeCriticExtraction(cloneMapAny(ex))
	srv := &Server{Store: st}
	ctx := acceptedPreciseMemoryContext(fmt.Sprintf("synthetic-review-rev-%d", turn))
	ev := reviewR2Evidence(turn, quote)
	result := artifactSaveResult{}
	content := "The synthetic scene begins. " + quote + " The synthetic scene ends."
	if len(sliceFromAny(ex["protected_secrets"])) > 0 {
		if len(sliceFromAny(normalized["protected_secrets"])) != 2 {
			t.Fatal("fixture setup: two protected facts did not survive production normalization")
		}
		for _, raw := range sliceFromAny(normalized["protected_secrets"]) {
			item := mapFromAny(raw)
			if !protectedSecretRequiresGuard(item, "disclosure_policy") || stringFromMap(item, "evidence_excerpt") != quote {
				t.Fatal("fixture setup: guarded exact quotation absent")
			}
		}
		t.Logf("protected_input=%s", mustCompactJSON(normalized["protected_secrets"]))
	}
	units := srv.buildPreciseMemoryUnitsFromExtraction(ctx, reviewR2SID, turn, normalized, content, ev, nil, time.Unix(int64(turn), 0), &result)
	var precise, public map[string]any
	for _, u := range units {
		if u.Kind == "state" && u.Subtype == "goal_status" {
			precise = parseJSONMap(u.PayloadJSON)
		}
		if u.Kind == "state" && u.Subtype == "public_profile" {
			public = parseJSONMap(u.PayloadJSON)
		}
	}
	srv.saveNarrativeStateFromExtraction(ctx, reviewR2SID, turn, normalized, content, ev, time.Unix(int64(turn), 0), &result)
	var narrative map[string]any
	st.returnPendingThreads = nil
	for _, v := range st.returnStatusCurrent {
		p := parseJSONMap(v.ValueJSON)
		if stringFromMap(p, "state_slot") == "goal_status" {
			narrative = p
		}
		if pt := narrativePendingSnapshot(p); pt != nil {
			st.returnPendingThreads = append(st.returnPendingThreads, *pt)
		}
	}
	if narrative == nil {
		t.Fatalf("positive narrative missing: %+v", result)
	}
	if precise == nil {
		t.Fatalf("positive precise goal missing: %+v", result)
	}
	if before != mustCompactJSON(ex) {
		t.Fatal("original synthetic input mutated")
	}
	t.Logf("TURN=%d precise=%s narrative_details=%s pending_scope=%s public=%s skips=%s writes=%d", turn, mustCompactJSON(precise), mustCompactJSON(narrative["lifecycle_details"]), mustCompactJSON(func() any {
		if pt := narrativePendingSnapshot(narrative); pt != nil {
			return parseJSONMap(pt.HookMetadataJSON)["knowledge_scope"]
		}
		return nil
	}()), mustCompactJSON(public), mustCompactJSON(result.SkipReasons), result.NarrativeCurrentStates)
	return precise, mapFromAny(narrative["lifecycle_details"]), public
}
func reviewR2Extraction(scope map[string]any, reversed, linked bool) map[string]any {
	a := reviewR2Secret("sealed-contents-fact", "Sealed parcel contents", reviewR2PlanScope())
	b := reviewR2Secret("instruction-fact", "Courier delivery instruction", reviewR2InstructionScope())
	if !linked {
		delete(a, "lifecycle_key")
		delete(b, "lifecycle_key")
	}
	secrets := []any{a, b}
	if reversed {
		secrets = []any{b, a}
	}
	c := reviewR2Claim("instruction-fact", "Delivery accepted", "create", reviewR2Quote, scope)
	pending := cloneMapAny(c)
	pending["title"] = "Parcel delivery instruction"
	pending["description"] = "Delivery accepted"
	sibling := map[string]any{"subject": "Courier", "subject_type": "entity", "state_slot": "public_profile", "value": "Courier is a delivery worker", "transition": "set", "evidence_excerpt": reviewR2Quote, "confidence": 0.99}
	return map[string]any{"state_claims": []any{c, sibling}, "pending_threads": []any{pending}, "protected_secrets": secrets}
}
func TestOriginReviewR2OrderDependentClaimScope(t *testing.T) {
	expected := mustCompactJSON(normalizeProtectedSecretKnowledgeScope(reviewR2InstructionScope(), "Coordinator"))
	var previousPrecise, previousNarrative string
	for _, reversed := range []bool{false, true} {
		p, n, _ := reviewR2Run(t, &turnRecordingStore{}, 1, reviewR2Extraction(nil, reversed, true), reviewR2Quote)
		ps, ns := mustCompactJSON(p["knowledge_scope"]), mustCompactJSON(n["knowledge_scope"])
		if ps != expected {
			t.Errorf("precise borrowed another fact scope reversed=%v: got=%s want=%s", reversed, ps, expected)
		}
		if ns != expected {
			t.Errorf("narrative borrowed another fact scope reversed=%v: got=%s want=%s", reversed, ns, expected)
		}
		if reversed && (ps != previousPrecise || ns != previousNarrative) {
			t.Errorf("protected-array reorder changed claim knowers: precise %s -> %s; narrative %s -> %s", previousPrecise, ps, previousNarrative, ns)
		}
		previousPrecise, previousNarrative = ps, ns
	}
}
func TestOriginReviewR2CoCitationWithoutFactLink(t *testing.T) {
	p, n, _ := reviewR2Run(t, &turnRecordingStore{}, 1, reviewR2Extraction(nil, false, false), reviewR2Quote)
	if len(mapFromAny(p["knowledge_scope"])) > 0 || len(mapFromAny(n["knowledge_scope"])) > 0 {
		t.Errorf("co-citation alone assigned another fact scope precise=%s narrative=%s", mustCompactJSON(p["knowledge_scope"]), mustCompactJSON(n["knowledge_scope"]))
	}
}
func TestOriginReviewR2OwnRecipientScopeAndPublicSibling(t *testing.T) {
	expected := mustCompactJSON(reviewR2InstructionScope())
	var firstPublic string
	for _, reversed := range []bool{false, true} {
		p, n, pub := reviewR2Run(t, &turnRecordingStore{}, 1, reviewR2Extraction(reviewR2InstructionScope(), reversed, true), reviewR2Quote)
		if mustCompactJSON(p["knowledge_scope"]) != expected || mustCompactJSON(n["knowledge_scope"]) != expected {
			t.Error("recipient own supplied instruction scope overwritten")
		}
		if pub == nil {
			t.Fatal("positive public same-root sibling missing")
		}
		if len(mapFromAny(pub["knowledge_scope"])) > 0 {
			t.Error("public sibling borrowed scope")
		}
		if reversed && firstPublic != mustCompactJSON(pub) {
			t.Error("public same-root sibling changed on reorder")
		}
		firstPublic = mustCompactJSON(pub)
	}
}
func TestOriginReviewR2LifecycleContinuityAndDisclosure(t *testing.T) {
	st := &turnRecordingStore{}
	reviewR2Run(t, st, 1, reviewR2Extraction(reviewR2InstructionScope(), false, true), reviewR2Quote)
	quote2 := "Courier continues the delivery instruction."
	c := reviewR2Claim("instruction-fact", "Delivery in progress", "progress", quote2, nil)
	_, n, _ := reviewR2Run(t, st, 2, map[string]any{"state_claims": []any{c}}, quote2)
	if mustCompactJSON(n["knowledge_scope"]) != mustCompactJSON(reviewR2InstructionScope()) {
		t.Error("same lifecycle progression lost own scope")
	}
	revealed := reviewR2Scope([]string{"Coordinator", "Courier", "Outsider"}, []string{})
	quote3 := "Courier reveals the delivery instruction to Outsider."
	c = reviewR2Claim("instruction-fact", "Instruction disclosed", "reveal", quote3, revealed)
	p, n, _ := reviewR2Run(t, st, 3, map[string]any{"state_claims": []any{c}}, quote3)
	if mustCompactJSON(p["knowledge_scope"]) != mustCompactJSON(revealed) || mustCompactJSON(n["knowledge_scope"]) != mustCompactJSON(revealed) {
		t.Error("explicit disclosure failed to supersede inherited scope")
	}
	for _, v := range st.returnStatusCurrent {
		if pt := narrativePendingSnapshot(parseJSONMap(v.ValueJSON)); pt != nil {
			if mustCompactJSON(parseJSONMap(pt.HookMetadataJSON)["knowledge_scope"]) != mustCompactJSON(revealed) {
				t.Error("pending snapshot retained old constraints after disclosure")
			}
		}
	}
}
func TestOriginReviewR2RepeatedDisclosureScopeUpdate(t *testing.T) {
	st := &turnRecordingStore{}
	reviewR2Run(t, st, 1, reviewR2Extraction(reviewR2InstructionScope(), false, true), reviewR2Quote)
	scope1 := reviewR2Scope([]string{"Coordinator", "Courier", "Outsider"}, []string{"Observer"})
	q1 := "Courier reveals the delivery instruction to Outsider."
	c := reviewR2Claim("instruction-fact", "Instruction disclosed", "reveal", q1, scope1)
	reviewR2Run(t, st, 2, map[string]any{"state_claims": []any{c}}, q1)
	scope2 := reviewR2Scope([]string{"Coordinator", "Courier", "Outsider", "Observer"}, []string{})
	q2 := "Courier also reveals the delivery instruction to Observer."
	c = reviewR2Claim("instruction-fact", "Instruction disclosed", "reveal", q2, scope2)
	p, n, _ := reviewR2Run(t, st, 3, map[string]any{"state_claims": []any{c}}, q2)
	if mustCompactJSON(p["knowledge_scope"]) != mustCompactJSON(scope2) {
		t.Error("precise supplied second disclosure lost")
	}
	if mustCompactJSON(n["knowledge_scope"]) != mustCompactJSON(scope2) {
		t.Errorf("narrative second explicit disclosure scope ignored got=%s want=%s", mustCompactJSON(n["knowledge_scope"]), mustCompactJSON(scope2))
	}
}
