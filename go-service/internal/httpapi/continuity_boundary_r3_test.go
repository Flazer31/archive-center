package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type continuityR3Store struct {
	*originLineageRecordingStore
	readError bool
}

func (s *continuityR3Store) ListStatusCurrentValues(ctx context.Context, sid, scope, owner, key string, limit int) ([]store.StatusCurrentValue, error) {
	if s.readError {
		return nil, errors.New("synthetic unavailable predecessor")
	}
	return s.turnRecordingStore.ListStatusCurrentValues(ctx, sid, scope, owner, key, limit)
}

func continuityR3Seed(t *testing.T) (*continuityR3Store, map[string]any) {
	t.Helper()
	st := &continuityR3Store{originLineageRecordingStore: &originLineageRecordingStore{turnRecordingStore: &turnRecordingStore{}}}
	ex := normalizeCriticExtraction(reviewR2Extraction(nil, false, false))
	res := artifactSaveResult{}
	(&Server{Store: st}).saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext("synthetic-continuity-origin"), reviewR2SID, 1, ex, "Before. "+reviewR2Quote+" After.", reviewR2Evidence(1, reviewR2Quote), time.Unix(1, 0), &res)
	if res.Errors != 0 {
		t.Fatal(res.ErrorDetails)
	}
	p, _ := lifecycleScopeR2Current(t, st.turnRecordingStore)
	if len(sliceFromAny(mapFromAny(p["lifecycle_details"])["knowledge_boundaries"])) != 2 {
		t.Fatalf("two original attributed boundaries absent: ex=%s current=%s skips=%s", mustCompactJSON(ex), mustCompactJSON(p), mustCompactJSON(res.SkipReasons))
	}
	return st, mapFromAny(p["lifecycle_details"])
}

func continuityR3Current(key string, scope map[string]any) map[string]any {
	const quote = "Courier reaffirms the delivery instruction."
	claim := reviewR2Claim(key, "Delivery accepted", "reaffirm", quote, scope)
	sibling := reviewR2Claim("public-sibling", "Courier checks the public cart", "create", quote, nil)
	return normalizeCriticExtraction(map[string]any{"turn_summary": "Courier checks the public cart", "state_claims": []any{claim, sibling}, "pending_threads": []any{map[string]any{"lifecycle_key": key, "title": "Parcel delivery instruction", "description": "Delivery accepted", "evidence_excerpt": quote, "confidence": 0.99}}, "evidence_excerpts": []string{quote}})
}

func continuityR3Delivery(t *testing.T, unit store.PreciseMemoryUnit, boundaries any) {
	t.Helper()
	semantic, ok := prepareTurnPrioritySemanticFactFromPreciseUnit(unit, 0.8, "synthetic external vector boundary")
	if !ok {
		t.Fatal("precise semantic source absent")
	}
	out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{MaxChars: 36000, TopK: 5, ProtectedSecretBudgetChars: 4000})
	prepareTurnResolvePrioritySourcePool(&out, "Courier delivery", nil, unit.SourceTurnStart+1, []prepareTurnPrioritySemanticFact{semantic})
	facts, _ := multiAgentCandidatePool(&out)
	if len(facts) != 1 {
		t.Fatalf("precise-only candidate count=%d", len(facts))
	}
	c := facts[0]
	canonical := func(v any) string {
		return mustCompactJSON(normalizePreciseMemoryValue(parseJSONMap(mustCompactJSON(map[string]any{"value": v}))["value"]))
	}
	if canonical(c.KnowledgeBoundaries) != canonical(boundaries) {
		t.Errorf("precise candidate lost original boundaries: %s", mustCompactJSON(c.KnowledgeBoundaries))
	}
	selection := &multiAgentSelection{Candidates: []prepareTurnPriorityMemoryCandidate{c}, Roles: []multiAgentRoleResult{{Role: c.Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{c.CanonicalFactID}, Reasons: map[string]string{c.CanonicalFactID: "Synthetic explicit selection boundary"}}}}}
	for _, budget := range []int{36000, 1800} {
		delivery := &prepareTurnInjectionAssembly{Preprocessing: selection, ProtectedSecretBudgetChars: 4000}
		if budget != 36000 {
			delivery.Preprocessing = nil
		}
		plan := renderPrepareTurnPriorityMemoryDeliveryPlan(delivery, budget, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: "Courier delivery", CurrentTurn: unit.SourceTurnStart + 1}, []prepareTurnPriorityMemoryCandidate{c}, nil, nil, nil)
		notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
		main := stringFromMap(plan, "main_memory_text")
		if len([]rune(main)) > budget {
			t.Fatal("main ceiling exceeded")
		}
		if budget == 36000 {
			for name, text := range map[string]string{"main": main, "notes": stringFromMap(notes, "final_text")} {
				for _, raw := range sliceFromAny(parseJSONMap(mustCompactJSON(map[string]any{"value": boundaries}))["value"]) {
					encoded := mustCompactJSON(raw)
					for _, supplied := range c.KnowledgeBoundaries {
						if canonical(supplied) == canonical(raw) {
							encoded = mustCompactJSON(supplied)
							break
						}
					}
					if !strings.Contains(text, encoded) {
						t.Errorf("%s lost attributed boundary", name)
					}
				}
			}
		}
		if strings.Contains(main, reviewR2Quote) || strings.Contains(stringFromMap(notes, "final_text"), reviewR2Quote) {
			t.Fatal("historical protected body cloned into delivery")
		}
	}
}

func TestContinuityR3ProductionPreciseSourceDelivery(t *testing.T) {
	for _, mode := range []string{"same_lifecycle", "current_scope", "unlinked", "read_error", "absent"} {
		t.Run(mode, func(t *testing.T) {
			st, prior := continuityR3Seed(t)
			var scope map[string]any
			priorPayload, _ := lifecycleScopeR2Current(t, st.turnRecordingStore)
			priorBytes := mustCompactJSON(priorPayload)
			key := "instruction-fact"
			if mode == "current_scope" {
				scope = map[string]any{"known_by": []string{"Courier", "Observer"}, "unknown_to": []string{}}
			}
			if mode == "unlinked" {
				key = "different-lifecycle"
			}
			if mode == "read_error" {
				st.readError = true
			}
			if mode == "absent" {
				st.returnStatusCurrent = nil
			}
			ex := continuityR3Current(key, scope)
			before := mustCompactJSON(ex)
			res := (&Server{Store: st}).saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("synthetic-continuity-current"), reviewR2SID, 2, ex, "Before. Courier reaffirms the delivery instruction. After.", completeTurnEmbeddingConfig{}, time.Unix(2, 0))
			if res.Errors != 0 {
				t.Fatal(res.ErrorDetails)
			}
			var goal *store.PreciseMemoryUnit
			siblings := 0
			for _, u := range st.units {
				p := parseJSONMap(u.PayloadJSON)
				if stringFromMap(p, "lifecycle_key") == key {
					goal = u
				}
				if stringFromMap(p, "lifecycle_key") == "public-sibling" {
					siblings++
					if u.Visibility != "public" || len(sliceFromAny(p["knowledge_boundaries"])) > 0 || len(mapFromAny(p["knowledge_scope"])) > 0 {
						t.Fatal("public sibling acquired boundary")
					}
				}
			}
			if goal == nil || siblings != 1 {
				t.Fatalf("ordinary current save lost units: goal=%v siblings=%d", goal != nil, siblings)
			}
			p := parseJSONMap(goal.PayloadJSON)
			if goal.Visibility != "public" || goal.EvidenceExcerpt != "Courier reaffirms the delivery instruction." || goal.SourceSpanStart != len("Before. ") || goal.SourceSpanEnd != len("Before. ")+len(goal.EvidenceExcerpt) {
				t.Fatal("historical citation changed current quotation/visibility")
			}
			if mode == "same_lifecycle" || mode == "current_scope" {
				if mustCompactJSON(p["knowledge_boundaries"]) != mustCompactJSON(normalizePreciseMemoryValue(prior["knowledge_boundaries"])) {
					t.Errorf("precise lost supplied lifecycle predecessor boundary: %s", goal.PayloadJSON)
				}
				if mode == "same_lifecycle" && len(mapFromAny(p["knowledge_scope"])) != 0 {
					t.Fatal("quotation promoted to goal knowers")
				}
				if mode == "current_scope" && mustCompactJSON(p["knowledge_scope"]) != mustCompactJSON(normalizePreciseMemoryValue(scope)) {
					t.Fatal("current scope overwritten")
				}
				continuityR3Delivery(t, *goal, prepareTurnOwnKnowledgeBoundary(p))
			} else if len(sliceFromAny(p["knowledge_boundaries"])) > 0 {
				t.Fatal("unavailable/unlinked predecessor manufactured boundary")
			}
			if len(st.savedMemories) != 1 {
				t.Fatal("ordinary memory acceptance changed")
			}
			stored := parseJSONMap(st.savedMemories[0].SummaryJSON)
			current := mapFromAny(sliceFromAny(stored["state_claims"])[0])
			if mustCompactJSON(normalizePreciseMemoryValue(current["knowledge_boundaries"])) != mustCompactJSON(p["knowledge_boundaries"]) {
				t.Error("stored current source lost precise metadata")
			}
			projection := buildPublicMemoryProjection(stored, "", "Courier reaffirms the delivery instruction.")
			if !projection.Eligible || !strings.Contains(projection.SearchText.Text, "public cart") {
				t.Fatal("public sibling projection changed")
			}
			if before != mustCompactJSON(ex) {
				t.Fatal("input extraction mutated")
			}
			if mode == "same_lifecycle" {
				currentPayload, _ := lifecycleScopeR2Current(t, st.turnRecordingStore)
				if priorBytes != mustCompactJSON(currentPayload) {
					t.Fatal("reaffirm manufactured narrative change")
				}
			}
		})
	}
}

func TestContinuityR3PreciseStoredBoundaryReader(t *testing.T) {
	_, prior := continuityR3Seed(t)
	u := store.PreciseMemoryUnit{UnitID: "synthetic-carrier", ChatSessionID: reviewR2SID, Kind: "state", Subtype: "goal_status", Visibility: "public", SourceTurnStart: 2, PayloadJSON: mustCompactJSON(map[string]any{"subject": "Courier", "state_slot": "goal_status", "lifecycle_key": "instruction-fact", "value": "Delivery accepted", "knowledge_boundaries": prior["knowledge_boundaries"]})}
	continuityR3Delivery(t, u, prior["knowledge_boundaries"])
}

func TestContinuityR3CurrentOwnFactUpdates(t *testing.T) {
	st, _ := continuityR3Seed(t)
	ex := continuityR3Current("instruction-fact", nil)
	quote := "Courier reaffirms the delivery instruction."
	secret := reviewR2Secret("instruction-fact", "Current instruction disclosure", reviewR2Scope([]string{"Courier", "Observer"}, []string{"Outsider"}))
	secret["evidence_excerpt"] = quote
	ex["protected_secrets"] = []any{secret}
	res := (&Server{Store: st}).saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("synthetic-explicit-update"), reviewR2SID, 2, ex, "Before. "+quote+" After.", completeTurnEmbeddingConfig{}, time.Unix(2, 0))
	if res.Errors != 0 {
		t.Fatal(res.ErrorDetails)
	}
	found := false
	for _, u := range st.units {
		p := parseJSONMap(u.PayloadJSON)
		if u.Kind == "state" && stringFromMap(p, "lifecycle_key") == "instruction-fact" {
			found = true
			if mustCompactJSON(p["knowledge_scope"]) != mustCompactJSON(normalizePreciseMemoryValue(secret["knowledge_scope"])) {
				t.Fatalf("current explicit fact scope overwritten: %s", u.PayloadJSON)
			}
		}
	}
	if !found {
		t.Fatal(fmt.Sprint("goal absent ", res.SkipReasons))
	}
}

func TestContinuityR3PublicProjectionMetadataParity(t *testing.T) {
	_, prior := continuityR3Seed(t)
	ex := continuityR3Current("instruction-fact", nil)
	item := mapFromAny(sliceFromAny(ex["state_claims"])[0])
	item["knowledge_boundaries"] = prior["knowledge_boundaries"]
	before := mustCompactJSON(ex)
	projection := buildPublicMemoryProjection(ex, "", "Before. Courier reaffirms the delivery instruction. After.")
	projected := mapFromAny(sliceFromAny(projection.Extraction["state_claims"])[0])
	if mustCompactJSON(projected["knowledge_boundaries"]) != mustCompactJSON(prior["knowledge_boundaries"]) {
		t.Fatal("public projection rewrote historical scope/receipts")
	}
	if len(mapFromAny(projected["knowledge_scope"])) != 0 || !projection.Eligible {
		t.Fatal("source quotation classified current goal privacy")
	}
	if mustCompactJSON(ex) != before {
		t.Fatal("projection mutated canonical source")
	}
}
