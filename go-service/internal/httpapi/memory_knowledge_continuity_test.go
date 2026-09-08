package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Regression cases reproduced in the 2026-09-08 knowledge continuity audit.
func TestMemoryKnowledgeIdentityUnknownPOV(t *testing.T) {
	identity := map[string]any{
		"surface_identity_name": "Mask", "true_identity_name": "Alice",
		"canonical_entity_name": "Alice", "same_entity": true,
		"identity_kind": "cover_identity", "reveal_policy": "owner_private_until_revealed",
		"knowledge_scope": map[string]any{"known_by": []any{"Alice"}, "unknown_to": []any{"Bob"}},
	}
	if !prepareTurnPerspectiveKnowsIdentity(identity, "Alice", "alice") {
		t.Fatal("control: the owner should know her own identity")
	}
	if prepareTurnPerspectiveKnowsIdentity(identity, "Bob", "bob") {
		t.Error("unknown_to Bob is incorrectly classified as knowing Alice's identity")
	}
	raw, _ := json.Marshal(map[string]any{
		"turn_summary":                "Bob meets Mask in the courtyard.",
		"characters":                  []any{"Alice", "Bob", "Mask"},
		"character_identity_accuracy": []any{identity},
	})
	memories := []store.Memory{{ID: 8101, ChatSessionID: "audit-identity", TurnIndex: 10, SummaryJSON: string(raw), Importance: .8}}
	assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 30000, "Bob meets Mask in the courtyard.", "default", nil, nil, nil,
		map[string]any{"current_pov": "Bob", "_priority_memory_enabled": true, "_priority_memory_max_items": 5})
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	t.Logf("final memory: %s", finalText)
	if strings.Contains(finalText, "For current_pov=Bob") && strings.Contains(finalText, "self/cover-role continuity") {
		t.Error("production assembly tells unaware Bob to treat another person's cover identity as self")
	}
}

func TestMemoryKnowledgeSecretKnownAndRevealed(t *testing.T) {
	for _, mode := range []string{"known_only", "revealed_only", "known_and_revealed"} {
		t.Run(mode, func(t *testing.T) {
			fake := newPreciseMemoryRecordingStore()
			srv := &Server{Store: fake}
			excerpt := "Mira revealed her vault map to Rowan."
			scope := map[string]any{}
			if mode != "revealed_only" {
				scope["known_by"] = []any{"Rowan"}
			}
			if mode != "known_only" {
				scope["revealed_to"] = []any{"Rowan"}
			}
			extraction := map[string]any{
				"entities":          map[string]any{"characters": []any{map[string]any{"name": "Mira"}, map[string]any{"name": "Rowan"}}},
				"evidence_excerpts": []any{excerpt},
				"protected_secrets": []any{map[string]any{
					"secret_kind": "vault_map", "owner": "Mira", "subject": []any{"Mira"},
					"summary": "Mira owns the vault map.", "transition": "reveal", "evidence_excerpt": excerpt,
					"knowledge_scope": scope,
				}},
			}
			extraction["protected_secrets"] = normalizeProtectedSecrets(extraction["protected_secrets"])
			result := srv.saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("audit-reveal-"+mode), "audit-reveal", 20,
				extraction, "At dawn, "+excerpt+" Then Rowan nodded.", completeTurnEmbeddingConfig{}, time.Unix(2000, 0))
			units := []store.PreciseMemoryUnit{}
			holder := ""
			rowanIDs := map[string]bool{}
			for _, identity := range fake.identities {
				if identity.CanonicalLabel == "Rowan" {
					rowanIDs[identity.StableEntityID] = true
				}
			}
			for _, key := range fake.saveOrder {
				u := fake.unitsByKey[key]
				if u.Kind != "observation" || !rowanIDs[u.KnowledgeHolderEntityID] {
					continue
				}
				if u.AdmissionState != "committed" || u.ReviewState != "source_observed" {
					t.Fatalf("compatible holder knowledge was not admitted: %+v", u)
				}
				units = append(units, *u)
				holder = u.KnowledgeHolderEntityID
				t.Logf("saved: state=%s admission=%s review=%s lifecycle=%s holder=%s", u.EpistemicMode, u.AdmissionState, u.ReviewState, u.LifecycleState, holder)
			}
			if len(units) == 0 || holder == "" {
				t.Fatalf("fixture did not reach stored holder records: %+v", result)
			}
			packet, text := buildCharacterPerspectivePacket(units, map[string]any{"identity_state": "resolved", "current_pov_entity_id": holder}, 30000)
			t.Logf("perspective=%q dropped=%v", text, packet["dropped_counts"])
			if !strings.Contains(text, "Mira owns the vault map.") {
				t.Error("explicit disclosure and known status describe compatible knowledge, but the stored knowledge vanished from the packet")
			}
		})
	}
}

func TestMemoryKnowledgeOldSecretAfterPublicReveal(t *testing.T) {
	oldSecret := map[string]any{"secret_id": "alice-cover", "secret_kind": "cover_identity", "owner": "Alice", "subject": []any{"Alice"},
		"summary": "Alice's cover name is Mask.", "disclosure_policy": "owner_private_until_revealed",
		"knowledge_scope": map[string]any{"known_by": []any{"Alice"}, "unknown_to": []any{"Bob"}}}
	newSecret := map[string]any{"secret_id": "alice-cover", "secret_kind": "cover_identity", "owner": "Alice", "subject": []any{"Alice"},
		"summary": "Alice's cover name is Mask.", "disclosure_policy": "public", "transition": "reveal",
		"knowledge_scope": map[string]any{"known_by": []any{"Alice", "Bob"}, "publicly_revealed": true}}
	row := func(id int64, turn int, summary string, secret map[string]any) store.Memory {
		body := map[string]any{"turn_summary": summary, "characters": []any{"Alice", "Bob"}, "protected_secrets": []any{secret}}
		if boolFromAny(mapFromAny(secret["knowledge_scope"])["publicly_revealed"]) {
			body["narrative_events"] = []any{map[string]any{"summary": summary, "event": summary, "actor": "Alice", "participants": []any{"Alice", "Bob"}, "visibility": "public"}}
		}
		raw, _ := json.Marshal(body)
		return store.Memory{ID: id, ChatSessionID: "audit-stale-secret", TurnIndex: turn, SummaryJSON: string(raw), Importance: .8}
	}
	old := row(8201, 10, "Alice concealed her cover identity from Bob.", oldSecret)
	latest := row(8202, 20, "Alice publicly revealed that she is Mask. Bob now knows her identity.", newSecret)
	assemble := func(rows []store.Memory) prepareTurnInjectionAssembly {
		return buildPrepareTurnInjectionAssembly(rows, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			5, 30000, "Alice and Bob discuss Mask in the courtyard.", "default", nil, nil, nil,
			map[string]any{"_priority_memory_enabled": true, "_priority_memory_max_items": 5, "_priority_memory_current_turn": 21})
	}
	control := assemble([]store.Memory{latest})
	if control.ProtectedMemoryText != "" {
		t.Fatalf("control: public record alone still generated a secret guard: %s", control.ProtectedMemoryText)
	}
	t.Logf("control public-only final: %s", extractionStringFromAny(control.MemoryDeliveryPlan["final_text"]))
	if !strings.Contains(extractionStringFromAny(control.MemoryDeliveryPlan["final_text"]), "Bob now knows her identity") {
		t.Fatal("control: explicit public event never entered final memory")
	}
	combined := assemble([]store.Memory{old, latest})
	finalText := extractionStringFromAny(combined.MemoryDeliveryPlan["final_text"])
	t.Logf("final memory: %s", finalText)
	if !strings.Contains(finalText, "Bob now knows her identity") {
		t.Error("control: public disclosure did not reach final memory")
	}
	if strings.Contains(finalText, "owner_private_until_revealed") {
		t.Error("same secret's old until-revealed instruction survives in final memory alongside its later public disclosure")
	}
}

func TestMemoryKnowledgeIndependentSecretsSameCategory(t *testing.T) {
	firstExcerpt := "Mira told Rowan she would leave by the east gate."
	secondExcerpt := "Mira told Rowan she would conceal the silver ledger."
	firstClaim := "Mira plans to leave by the east gate."
	secondClaim := "Mira plans to conceal the silver ledger."
	for _, mode := range []string{"different_categories", "same_category_same_turn", "same_category_later_turn"} {
		t.Run(mode, func(t *testing.T) {
			secondKind := "hidden_plan"
			if mode == "different_categories" {
				secondKind = "hidden_document"
			}
			secret := func(kind, claim, excerpt string) map[string]any {
				return map[string]any{"secret_kind": kind, "owner": "Mira", "subject": []any{"Mira"}, "summary": claim,
					"evidence_excerpt": excerpt, "knowledge_scope": map[string]any{"known_by": []any{"Rowan"}}}
			}
			extraction := normalizeCriticExtraction(map[string]any{
				"entities":          map[string]any{"characters": []any{map[string]any{"name": "Mira"}, map[string]any{"name": "Rowan"}}},
				"evidence_excerpts": []any{firstExcerpt, secondExcerpt},
				"protected_secrets": []any{secret("hidden_plan", firstClaim, firstExcerpt), secret(secondKind, secondClaim, secondExcerpt)},
			})
			fake := newPreciseMemoryRecordingStore()
			srv := &Server{Store: fake}
			result := srv.saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("audit-distinct-"+mode), "audit-distinct", 20,
				extraction, firstExcerpt+" "+secondExcerpt, completeTurnEmbeddingConfig{}, time.Unix(2000, 0))
			units := []store.PreciseMemoryUnit{}
			holder := ""
			for _, key := range fake.saveOrder {
				u := *fake.unitsByKey[key]
				if u.Kind != "observation" {
					continue
				}
				if u.AdmissionState != "committed" || u.ReviewState != "source_observed" {
					t.Fatalf("fixture not admitted: %+v", u)
				}
				if holder != "" && holder != u.KnowledgeHolderEntityID {
					t.Fatal("fixture holder identities differ")
				}
				holder = u.KnowledgeHolderEntityID
				// Reader fixture variant: independent second observation has a later source turn.
				if mode == "same_category_later_turn" && strings.Contains(u.PayloadJSON, secondClaim) {
					u.SourceTurnStart, u.SourceTurnEnd = 21, 21
				}
				units = append(units, u)
			}
			if len(units) != 2 || holder == "" {
				t.Fatalf("expected two admitted observations: result=%+v count=%d", result, len(units))
			}
			packet, text := buildCharacterPerspectivePacket(units, map[string]any{"identity_state": "resolved", "current_pov_entity_id": holder}, 30000)
			t.Logf("two independent admitted secrets; perspective=%q dropped=%v", text, packet["dropped_counts"])
			if !strings.Contains(text, firstClaim) || !strings.Contains(text, secondClaim) {
				t.Error("independent compatible plans share a category but one or both disappeared from the perspective packet")
			}
		})
	}
}

func TestMemoryKnowledgeIdentityDisclosureCreatesKnowerMemory(t *testing.T) {
	excerpt := "Mira told Rowan that her cover name was Mask."
	for _, lane := range []string{"protected_secrets", "character_identity_accuracy", "identity_scope_only_holder"} {
		t.Run(lane, func(t *testing.T) {
			field := lane
			if lane == "identity_scope_only_holder" {
				field = "character_identity_accuracy"
			}
			raw := map[string]any{
				"turn_summary":      "Rowan learned that Mira's cover name is Mask.",
				"entities":          map[string]any{"characters": []any{map[string]any{"name": "Mira"}, map[string]any{"name": "Rowan"}}},
				"evidence_excerpts": []any{excerpt},
			}
			if lane == "identity_scope_only_holder" {
				raw["entities"] = map[string]any{"characters": []any{map[string]any{"name": "Mira"}}}
			}
			if lane == "protected_secrets" {
				raw[lane] = []any{map[string]any{"secret_kind": "cover_identity", "owner": "Mira", "subject": []any{"Mira"},
					"summary": "Mira's cover name is Mask.", "evidence_excerpt": excerpt,
					"knowledge_scope": map[string]any{"known_by": []any{"Rowan"}}}}
			} else {
				raw[field] = []any{map[string]any{"canonical_entity_name": "Mira", "true_identity_name": "Mira", "surface_identity_name": "Mask",
					"same_entity": true, "identity_kind": "cover_identity", "evidence_excerpt": excerpt,
					"source_evidence_turns": []any{20}, "knowledge_scope": map[string]any{"known_by": []any{"Rowan"}}}}
			}
			parsed, _, err := validateCriticExtractionSchema(raw)
			if err != nil {
				t.Fatal(err)
			}
			parsed, _ = quarantineCriticProtectedCandidates(parsed, "Mira explains her cover name.", "At dawn, "+excerpt+" Rowan nodded in recognition.")
			extraction := normalizeCriticExtraction(parsed)
			fake := newPreciseMemoryRecordingStore()
			srv := &Server{Store: fake}
			saved := srv.saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("audit-identity-lane-"+lane), "audit-identity-lane", 20,
				extraction, "At dawn, "+excerpt+" Rowan nodded in recognition.", completeTurnEmbeddingConfig{}, time.Unix(2000, 0))
			knowerRecords := 0
			rowanIDs := map[string]bool{}
			for _, identity := range fake.identities {
				if identity.CanonicalLabel == "Rowan" {
					rowanIDs[identity.StableEntityID] = true
				}
			}
			if len(rowanIDs) == 0 {
				t.Fatal("fixture did not persist Rowan's identity")
			}
			for _, u := range fake.unitsByKey {
				if u.Kind == "observation" && rowanIDs[u.KnowledgeHolderEntityID] {
					if u.AdmissionState != "committed" || u.ReviewState != "source_observed" {
						t.Fatalf("holder identity knowledge not admitted: %+v", u)
					}
					_, delivered := buildCharacterPerspectivePacket([]store.PreciseMemoryUnit{*u}, map[string]any{"identity_state": "resolved", "current_pov_entity_id": u.KnowledgeHolderEntityID}, 30000)
					if !strings.Contains(delivered, "Mask") {
						t.Fatalf("stored identity knowledge did not reach holder packet: %q", delivered)
					}
					knowerRecords++
				}
			}
			owners := []string{}
			for _, item := range sliceFromAny(extraction["subjective_entity_memories"]) {
				owners = append(owners, stringFromMap(mapFromAny(item), "owner_entity_name"))
			}
			t.Logf("lane=%s holder-specific observations for Rowan=%d generated subjective owners=%v", lane, knowerRecords, owners)
			if knowerRecords == 0 {
				t.Errorf("explicit known_by Rowan was retained as identity metadata but no holder-specific knowledge was produced; artifact save=%+v", saved)
			}
		})
	}
}

func TestMemoryKnowledgeIdentityPOVIsOwnerOrInformedObserver(t *testing.T) {
	identity := map[string]any{
		"canonical_entity_name": "Mira", "true_identity_name": "Mira", "surface_identity_name": "Mask", "same_entity": true,
		"knowledge_scope": map[string]any{"known_by": []any{"Rowan"}, "revealed_to": []any{"Tomas"}, "unknown_to": []any{"Jules"}},
	}
	for _, name := range []string{"Mira", "Mask", "Rowan", "Tomas", "Jules"} {
		t.Run(name, func(t *testing.T) {
			line := prepareTurnPOVScopedIdentityGuardLine([]any{identity}, map[string]any{"current_pov": name})
			if name == "Jules" {
				if line != "" {
					t.Fatalf("uninformed observer gained identity knowledge: %q", line)
				}
				return
			}
			if line == "" {
				t.Fatal("owner or informed observer lost identity context")
			}
			if (name == "Mira" || name == "Mask") != strings.Contains(line, "self/cover-role continuity") {
				t.Fatalf("owner identity was confused with observer knowledge: %q", line)
			}
		})
	}
}

func TestMemoryKnowledgeDisclosureResolvesOnlyTheSameSecret(t *testing.T) {
	for _, mode := range []string{"stable_id", "legacy_identical_claim", "other_session", "older_disclosure", "later_private"} {
		t.Run(mode, func(t *testing.T) {
			secret := func(id, claim string, public bool) map[string]any {
				item := map[string]any{"secret_kind": "hidden_plan", "owner": "Mira", "subject": []any{"Mira"}, "summary": claim,
					"knowledge_scope": map[string]any{"known_by": []any{"Rowan"}, "publicly_revealed": public}}
				if mode != "legacy_identical_claim" {
					item["secret_id"] = id
				}
				return item
			}
			row := func(id int64, turn int, sid string, items ...any) store.Memory {
				body, _ := json.Marshal(map[string]any{"turn_summary": "Mira discussed her plans.", "protected_secrets": items})
				return store.Memory{ID: id, ChatSessionID: sid, TurnIndex: turn, SummaryJSON: string(body)}
			}
			old := row(901, 10, "session", secret("east", "Mira plans to leave by the east gate.", false), secret("ledger", "Mira plans to conceal the silver ledger.", false))
			public := row(902, 20, "session", secret("east", "Mira plans to leave by the east gate.", true))
			if mode == "other_session" {
				public.ChatSessionID = "other"
			}
			if mode == "older_disclosure" {
				public.TurnIndex = old.TurnIndex - 1
			}
			canonical := []store.Memory{old, public}
			if mode == "later_private" {
				canonical = append(canonical, row(903, 30, "session", secret("east", "Mira plans to leave by the east gate.", false)))
			}
			original := old.SummaryJSON
			groups, _ := buildPrepareTurnProtectedDeliveryGroups(prepareTurnMemoryLaneSelection{ProtectedSelected: []store.Memory{old}}, canonical...)
			active, released := 0, 0
			for _, group := range groups[prepareTurnMemoryLaneKey(old)] {
				if group.Disclosure != nil {
					released++
					if group.Disclosure.ID != public.ID {
						t.Fatal("disclosure source lineage changed")
					}
				} else {
					active++
				}
			}
			wantRelease := mode == "stable_id" || mode == "legacy_identical_claim"
			if wantRelease && (active != 1 || released != 1) {
				t.Fatalf("independent secret was lost or obsolete guard retained: active=%d released=%d", active, released)
			}
			if !wantRelease && (active != 2 || released != 0) {
				t.Fatalf("unrelated or past disclosure released current secret: active=%d released=%d", active, released)
			}
			if old.SummaryJSON != original {
				t.Fatal("canonical secret record was rewritten")
			}
			_, trace := prepareTurnMemoryLaneLines(prepareTurnMemoryLaneSelection{ProtectedSelected: []store.Memory{old}}, nil, canonical)
			if wantRelease {
				found := false
				for _, raw := range prepareTurnMemoryLineageSlice(trace["delivery_lineage_items"]) {
					item := mapFromAny(raw)
					if item["delivery_status"] == "released_by_later_disclosure" {
						found = true
						if boolFromAny(item["delivered"]) || item["disclosure_source_row_id"] != public.ID {
							t.Fatalf("release lineage inaccurate: %#v", item)
						}
					}
				}
				if !found {
					t.Fatal("release reason was absent from delivery lineage")
				}
			}
		})
	}
}

func TestMemoryKnowledgeIndependentSubjectiveMemories(t *testing.T) {
	first := "Rowan remembers that Mira gave him a brass ring."
	second := "Rowan remembers that Tomas taught him a lullaby."
	extraction := normalizeCriticExtraction(map[string]any{
		"entities":          map[string]any{"characters": []any{map[string]any{"name": "Rowan"}, map[string]any{"name": "Mira"}, map[string]any{"name": "Tomas"}}},
		"evidence_excerpts": []any{first, second},
		"subjective_entity_memories": []any{
			map[string]any{"owner_entity_name": "Rowan", "memory_text": first, "evidence_excerpt": first},
			map[string]any{"owner_entity_name": "Rowan", "memory_text": second, "evidence_excerpt": second},
		},
	})
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	srv.saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("audit-subjective"), "audit-subjective", 20,
		extraction, first+" "+second, completeTurnEmbeddingConfig{}, time.Unix(2000, 0))
	units := []store.PreciseMemoryUnit{}
	holder := ""
	for _, key := range fake.saveOrder {
		u := *fake.unitsByKey[key]
		if u.Kind != "observation" {
			continue
		}
		if u.AdmissionState != "committed" {
			t.Fatalf("fixture not admitted: %+v", u)
		}
		if holder != "" && holder != u.KnowledgeHolderEntityID {
			t.Fatal("fixture holder identities differ")
		}
		holder = u.KnowledgeHolderEntityID
		units = append(units, u)
	}
	if len(units) != 2 || holder == "" {
		t.Fatalf("expected two admitted observations, got %d", len(units))
	}
	packet, text := buildCharacterPerspectivePacket(units, map[string]any{"identity_state": "resolved", "current_pov_entity_id": holder}, 30000)
	t.Logf("two independent subjective memories; perspective=%q dropped=%v", text, packet["dropped_counts"])
	if !strings.Contains(text, first) || !strings.Contains(text, second) {
		t.Error("independent recollections collide in the shared subjective_memory slot")
	}
}
