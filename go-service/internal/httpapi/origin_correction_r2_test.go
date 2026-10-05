package httpapi

import (
	"strings"
	"testing"
	"time"
)

func TestOriginReviewR2FactIDAttributionAndSeparateQuoteBoundaries(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		ex := reviewR2Extraction(nil, reversed, false)
		for _, raw := range sliceFromAny(ex["protected_secrets"]) {
			item := mapFromAny(raw)
			item["fact_id"] = stringFromMap(item, "secret_id")
		}
		mapFromAny(sliceFromAny(ex["state_claims"])[0])["fact_ref"] = "instruction-fact"
		mapFromAny(sliceFromAny(ex["pending_threads"])[0])["fact_ref"] = "instruction-fact"
		p, n, _ := reviewR2Run(t, &turnRecordingStore{}, 1, ex, reviewR2Quote)
		for _, payload := range []map[string]any{p, n} {
			want := normalizeProtectedSecretKnowledgeScope(reviewR2InstructionScope(), "Coordinator")
			if mustCompactJSON(payload["knowledge_scope"]) != mustCompactJSON(want) {
				t.Fatal("explicit fact reference borrowed sealed contents scope")
			}
			boundaries := sliceFromAny(payload["knowledge_boundaries"])
			if len(boundaries) != 2 {
				t.Fatal("co-cited protected facts were collapsed")
			}
			seen := map[string]bool{}
			for _, raw := range boundaries {
				boundary := mapFromAny(raw)
				id := stringFromMap(boundary, "fact_id")
				want := reviewR2PlanScope()
				if id == "instruction-fact" {
					want = reviewR2InstructionScope()
				}
				if id != "instruction-fact" && id != "sealed-contents-fact" {
					t.Fatal("source fact identity lost")
				}
				if mustCompactJSON(boundary["knowledge_scope"]) != mustCompactJSON(normalizeProtectedSecretKnowledgeScope(want, "Coordinator")) {
					t.Fatal("source facts' scopes were unioned or misattributed")
				}
				origin := mapFromAny(boundary["knowledge_source"])
				index := intFromAny(origin["source_index"], -1)
				if index < 0 || stringFromMap(mapFromAny(sliceFromAny(ex["protected_secrets"])[index]), "fact_id") != id {
					t.Fatal("source index identifies the wrong original fact")
				}
				if strings.Contains(mustCompactJSON(boundary), reviewR2Quote) {
					t.Fatal("boundary cloned protected source body")
				}
				seen[id] = true
			}
			if len(seen) != 2 {
				t.Fatal("separate protected facts missing")
			}
		}
	}
}

func TestOriginReviewR2RepeatedSameValueScopeUpdatesPersistPending(t *testing.T) {
	for _, transition := range []string{"reveal", "reaffirm", "set"} {
		t.Run(transition, func(t *testing.T) {
			st := &turnRecordingStore{}
			scope1 := reviewR2Scope([]string{"Coordinator", "Courier"}, []string{"Observer"})
			scope2 := reviewR2Scope([]string{"Coordinator", "Courier", "Observer"}, []string{})
			for turn, scope := range []map[string]any{scope1, scope2} {
				quote := "Courier explains the instruction to the present audience."
				claim := reviewR2Claim("instruction-fact", "Instruction disclosed", transition, quote, scope)
				pending := cloneMapAny(claim)
				pending["title"], pending["description"] = "Parcel delivery instruction", "Instruction disclosed"
				_, n, _ := reviewR2Run(t, st, turn+1, map[string]any{"state_claims": []any{claim}, "pending_threads": []any{pending}}, quote)
				if mustCompactJSON(n["knowledge_scope"]) != mustCompactJSON(scope) {
					t.Fatal("explicit scope update ignored on same value/phase")
				}
				for _, p := range st.returnPendingThreads {
					if mustCompactJSON(parseJSONMap(p.HookMetadataJSON)["knowledge_scope"]) != mustCompactJSON(scope) || mustCompactJSON(parseJSONMap(p.DetailsJSON)["knowledge_scope"]) != mustCompactJSON(scope) {
						t.Fatal("persisted pending metadata retained old disclosure")
					}
				}
			}
			writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
			quote := "Courier explains the instruction to the present audience."
			claim := reviewR2Claim("instruction-fact", "Instruction disclosed", transition, quote, scope2)
			(&Server{Store: st}).saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("repeat"), reviewR2SID, 3, map[string]any{"state_claims": []any{claim}}, "Before. "+quote+" After.", nil, time.Unix(3, 0), &artifactSaveResult{})
			if len(st.savedStatusCurrent) != writes || len(st.savedStatusEvents) != events {
				t.Fatal("unchanged supplied scope manufactured another write")
			}
		})
	}
}

func TestOriginReviewR2QuotationAttributionSurvivesCarry(t *testing.T) {
	p, n, _ := reviewR2Run(t, &turnRecordingStore{}, 1, reviewR2Extraction(nil, false, false), reviewR2Quote)
	for _, payload := range []map[string]any{p, n} {
		if len(mapFromAny(payload["knowledge_scope"])) != 0 {
			t.Fatal("unlinked quotation inferred claim scope")
		}
		before := mustCompactJSON(payload["knowledge_boundaries"])
		boundaries := prepareTurnOwnKnowledgeBoundary(payload)
		if len(boundaries) != 2 || mustCompactJSON(boundaries) != before {
			t.Fatal("stored original quotation attribution changed during ingestion")
		}
	}
}

func TestOriginReviewR2PendingSnapshotMetadataContinuity(t *testing.T) {
	st := &turnRecordingStore{}
	reviewR2Run(t, st, 1, reviewR2Extraction(reviewR2InstructionScope(), false, true), reviewR2Quote)
	var originalBoundaries any
	// Model an already stored snapshot whose metadata is in pending JSON only.
	for i := range st.returnStatusCurrent {
		payload := parseJSONMap(st.returnStatusCurrent[i].ValueJSON)
		if stringFromMap(payload, "state_slot") != "goal_status" {
			continue
		}
		details := mapFromAny(payload["lifecycle_details"])
		originalBoundaries = details["knowledge_boundaries"]
		for _, key := range []string{"knowledge_scope", "knowledge_source", "knowledge_boundaries"} {
			delete(details, key)
		}
		payload["lifecycle_details"] = details
		st.returnStatusCurrent[i].ValueJSON = mustCompactJSON(payload)
	}
	quote := "Courier continues the delivery instruction."
	claim := reviewR2Claim("instruction-fact", "Delivery in progress", "progress", quote, nil)
	_, n, _ := reviewR2Run(t, st, 2, map[string]any{"state_claims": []any{claim}}, quote)
	if mustCompactJSON(n["knowledge_scope"]) != mustCompactJSON(reviewR2InstructionScope()) || mustCompactJSON(n["knowledge_boundaries"]) != mustCompactJSON(originalBoundaries) {
		t.Fatal("pending-only predecessor metadata lost during progression")
	}
}
