package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestLifecycleBoundaryR3AttributedScopeUpdatePersists(t *testing.T) {
	for _, phase := range []string{"create", "complete"} {
		for _, mode := range []string{"claim", "pending", "both"} {
			t.Run(phase+"/"+mode, func(t *testing.T) {
				st := &turnRecordingStore{}
				reviewR2Run(t, st, 1, reviewR2Extraction(nil, false, false), reviewR2Quote)
				if phase == "complete" {
					lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Delivery complete", phase, "Courier completes the delivery instruction.", nil), "claim")
				}
				prior, priorPending := lifecycleScopeR2Current(t, st)
				oldDetails := mapFromAny(prior["lifecycle_details"])
				boundaries := sliceFromAny(cloneMapAny(oldDetails)["knowledge_boundaries"])
				if len(boundaries) != 2 || len(mapFromAny(oldDetails["knowledge_scope"])) != 0 {
					t.Fatal("setup requires two independently attributed scopes without claim scope")
				}
				first := mapFromAny(boundaries[0])
				first["knowledge_scope"] = reviewR2Scope([]string{"Coordinator", "Auditor", "Observer"}, []string{"Courier", "Outsider"})
				claim := reviewR2Claim("instruction-fact", stringFromMap(prior, "value"), "reaffirm", "Observer learns the recorded parcel fact.", nil)
				claim["knowledge_boundaries"] = boundaries
				writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
				lifecycleScopeR2Save(t, st, 3, claim, mode)
				current, pending := lifecycleScopeR2Current(t, st)
				for _, metadata := range []map[string]any{mapFromAny(current["lifecycle_details"]), parseJSONMap(pending.HookMetadataJSON), parseJSONMap(pending.DetailsJSON)} {
					if mustCompactJSON(metadata["knowledge_boundaries"]) != mustCompactJSON(boundaries) {
						t.Error("same-value update discarded supplied fact-specific scope change")
					}
					if len(mapFromAny(metadata["knowledge_scope"])) != 0 {
						t.Error("separate fact scopes were promoted to the goal's own knowers")
					}
				}
				if pending.Status != priorPending.Status || pending.ResolvedTurn != priorPending.ResolvedTurn || pending.ResolutionNote != priorPending.ResolutionNote {
					t.Error("boundary metadata update changed lifecycle phase or resolution")
				}
				if len(st.savedStatusCurrent) != writes+1 || len(st.savedStatusEvents) != events+1 {
					t.Error("attributed scope update must persist one current/history write")
				}
				writes, events = len(st.savedStatusCurrent), len(st.savedStatusEvents)
				lifecycleScopeR2Save(t, st, 3, claim, mode)
				if len(st.savedStatusCurrent) != writes || len(st.savedStatusEvents) != events {
					t.Error("identical attributed scopes manufactured another write")
				}
			})
		}
	}
}

func TestLifecycleBoundaryR3ProvenanceOnlyNoWrite(t *testing.T) {
	for _, phase := range []string{"create", "complete"} {
		for _, mode := range []string{"claim", "pending", "both"} {
			for _, attributed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/attributed=%t", phase, mode, attributed), func(t *testing.T) {
					st := &turnRecordingStore{}
					var initialScope map[string]any
					if !attributed {
						initialScope = reviewR2InstructionScope()
					}
					reviewR2Run(t, st, 1, reviewR2Extraction(initialScope, false, false), reviewR2Quote)
					if phase == "complete" {
						lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Delivery complete", "complete", "Courier completes the delivery instruction.", nil), "claim")
					}
					prior, priorPending := lifecycleScopeR2Current(t, st)
					details := mapFromAny(prior["lifecycle_details"])
					claim := reviewR2Claim("instruction-fact", stringFromMap(prior, "value"), "reaffirm", "Courier repeats the recorded instruction.", nil)
					if attributed {
						boundaries := sliceFromAny(cloneMapAny(details)["knowledge_boundaries"])
						if len(boundaries) != 2 || len(mapFromAny(details["knowledge_scope"])) != 0 {
							t.Fatal("positive separate-fact setup absent")
						}
						for _, raw := range boundaries {
							b := mapFromAny(raw)
							src := cloneMapAny(mapFromAny(b["knowledge_source"]))
							src["source_revision"] = "synthetic-provenance-refresh"
							b["knowledge_source"] = src
						}
						claim["knowledge_boundaries"] = boundaries
					} else {
						claim["knowledge_scope"] = cloneMapAny(mapFromAny(details["knowledge_scope"]))
						claim["knowledge_source"] = map[string]any{"source_revision": "synthetic-provenance-refresh"}
					}
					writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
					lifecycleScopeR2Save(t, st, 3, claim, mode)
					current, pending := lifecycleScopeR2Current(t, st)
					if pending.Status != priorPending.Status || pending.ResolvedTurn != priorPending.ResolvedTurn || pending.ResolutionNote != priorPending.ResolutionNote {
						t.Error("provenance-only mention changed lifecycle")
					}
					if len(st.savedStatusCurrent) != writes || len(st.savedStatusEvents) != events {
						t.Errorf("same-value unchanged knowledge scope wrote current/history solely for provenance: current %d->%d events %d->%d; attributed=%t old=%s new=%s", writes, len(st.savedStatusCurrent), events, len(st.savedStatusEvents), attributed, mustCompactJSON(details["knowledge_boundaries"]), mustCompactJSON(mapFromAny(current["lifecycle_details"])["knowledge_boundaries"]))
					}
				})
			}
		}
	}
}

func TestLifecycleBoundaryR3RegeneratedProvenanceNoWrite(t *testing.T) {
	for _, mode := range []string{"claim", "pending", "both"} {
		t.Run(mode, func(t *testing.T) {
			st := &turnRecordingStore{}
			save := func(turn int, revision, transition string) {
				ex, content, ev := goalR3Fixture([]string{"F41", "F42"}, nil, false, false)
				for _, field := range []string{"state_claims", "pending_threads"} {
					mapFromAny(sliceFromAny(ex[field])[0])["transition"] = transition
				}
				if turn > 1 && mode == "claim" {
					delete(ex, "pending_threads")
				}
				if turn > 1 && mode == "pending" {
					delete(ex, "state_claims")
				}
				for i := range ev {
					ev[i].SourceTurnStart = turn
					ev[i].SourceTurnEnd = turn
					ev[i].TurnAnchor = turn
				}
				result := artifactSaveResult{}
				(&Server{Store: st}).saveNarrativeStateFromExtraction(acceptedPreciseMemoryContext(revision), reviewR2SID, turn, normalizeCriticExtraction(ex), content, ev, time.Unix(int64(turn), 0), &result)
				if result.Errors != 0 {
					t.Fatalf("synthetic save errors=%+v", result)
				}
				st.returnPendingThreads = nil
				for _, row := range st.returnStatusCurrent {
					if p := narrativePendingSnapshot(parseJSONMap(row.ValueJSON)); p != nil {
						st.returnPendingThreads = append(st.returnPendingThreads, *p)
					}
				}
			}
			save(1, "synthetic-generated-provenance-1", "create")
			prior, _ := lifecycleScopeR2Current(t, st)
			before := mapFromAny(prior["lifecycle_details"])
			if len(sliceFromAny(before["knowledge_boundaries"])) != 2 || len(mapFromAny(before["knowledge_scope"])) != 0 {
				t.Fatal("two linked distinct origins setup absent")
			}
			writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
			save(2, "synthetic-generated-provenance-2", "reaffirm")
			current, _ := lifecycleScopeR2Current(t, st)
			after := mapFromAny(current["lifecycle_details"])
			for _, oldRaw := range sliceFromAny(before["knowledge_boundaries"]) {
				old := mapFromAny(oldRaw)
				seen := false
				for _, newRaw := range sliceFromAny(after["knowledge_boundaries"]) {
					b := mapFromAny(newRaw)
					if stringFromMap(b, "fact_id") == stringFromMap(old, "fact_id") {
						seen = true
						if mustCompactJSON(b["knowledge_scope"]) != mustCompactJSON(old["knowledge_scope"]) {
							t.Fatal("scope control changed")
						}
					}
				}
				if !seen {
					t.Fatal("stable fact identity absent")
				}
			}
			if len(st.savedStatusCurrent) != writes || len(st.savedStatusEvents) != events {
				t.Errorf("production-generated provenance caused same-value same-fact-scope current/history write: %d->%d / %d->%d before=%s after=%s", writes, len(st.savedStatusCurrent), events, len(st.savedStatusEvents), mustCompactJSON(before["knowledge_boundaries"]), mustCompactJSON(after["knowledge_boundaries"]))
			}
		})
	}
}

func TestLifecycleBoundaryR3DistinctAssociationControls(t *testing.T) {
	for _, phase := range []string{"create", "complete"} {
		for _, kind := range []string{"swap_fact_scopes", "add_distinct_fact"} {
			t.Run(phase+"/"+kind, func(t *testing.T) {
				st := &turnRecordingStore{}
				reviewR2Run(t, st, 1, reviewR2Extraction(nil, false, false), reviewR2Quote)
				if phase == "complete" {
					lifecycleScopeR2Save(t, st, 2, reviewR2Claim("instruction-fact", "Delivery complete", "complete", "Courier completes the delivery instruction.", nil), "claim")
				}
				prior, priorPending := lifecycleScopeR2Current(t, st)
				details := mapFromAny(prior["lifecycle_details"])
				bs := sliceFromAny(cloneMapAny(details)["knowledge_boundaries"])
				if len(bs) != 2 {
					t.Fatal("distinct association setup absent")
				}
				a, b := mapFromAny(bs[0]), mapFromAny(bs[1])
				if kind == "swap_fact_scopes" {
					a["knowledge_scope"], b["knowledge_scope"] = b["knowledge_scope"], a["knowledge_scope"]
				} else {
					third := cloneMapAny(a)
					third["secret_id"] = "third-distinct-explicit-fact"
					third["protected_fact_ref"] = "source-revision:synthetic-third-origin/protected_secrets/2"
					bs = append(bs, third)
				}
				claim := reviewR2Claim("instruction-fact", stringFromMap(prior, "value"), "reaffirm", "Observer learns the newly attributed fact scopes.", nil)
				claim["knowledge_boundaries"] = bs
				writes, events := len(st.savedStatusCurrent), len(st.savedStatusEvents)
				lifecycleScopeR2Save(t, st, 3, claim, "both")
				current, pending := lifecycleScopeR2Current(t, st)
				if len(st.savedStatusCurrent) != writes+1 || len(st.savedStatusEvents) != events+1 {
					t.Error("actual distinct fact association/scope change was discarded")
				}
				if mustCompactJSON(mapFromAny(current["lifecycle_details"])["knowledge_boundaries"]) != mustCompactJSON(bs) || len(mapFromAny(mapFromAny(current["lifecycle_details"])["knowledge_scope"])) != 0 {
					t.Error("fact boundaries altered or readers merged")
				}
				if pending.Status != priorPending.Status || pending.ResolvedTurn != priorPending.ResolvedTurn {
					t.Error("fact association update changed lifecycle phase")
				}
			})
		}
	}
}
