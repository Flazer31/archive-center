package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func auditfixSecret(id int64, turn int, claim, artifact string, subject []string) store.Memory {
	secret := map[string]any{"owner": "Mira", "secret_kind": "plan", "subject": subject, "summary": claim,
		"disclosure_policy": "owner_private_until_revealed", "knowledge_scope": map[string]any{"known_by": []string{"Mira"}, "unknown_to": []string{"Visitor"}}}
	if artifact != "" {
		secret["artifact_id"] = artifact
	}
	return store.Memory{ID: id, ChatSessionID: "auditfix-synthetic", TurnIndex: turn,
		SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": claim, "protected_secrets": []any{secret}})}
}

func TestAuditfixDistinctProtectedSecrets(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []store.Memory
	}{
		{"different_turns", []store.Memory{auditfixSecret(1, 1, "Mira hid the archive key beneath the floor.", "", []string{"Mira"}), auditfixSecret(2, 2, "Mira plans to meet the courier at midnight.", "", []string{"Mira"})}},
		{"untyped_artifact_ids", []store.Memory{auditfixSecret(1, 1, "Mira hides a duplicate seal.", "seal-A", nil), auditfixSecret(2, 2, "Mira hides a duplicate seal.", "seal-B", nil)}},
		{"typed_artifact_ids", []store.Memory{auditfixSecret(1, 1, "Mira hides a duplicate seal.", "seal-A", []string{"Mira"}), auditfixSecret(2, 2, "Mira hides a duplicate seal.", "seal-B", []string{"Mira"})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := mustCompactJSON(tc.items)
			selection := prepareTurnMemoryLaneSelection{ProtectedSelected: tc.items, ProtectedCandidates: tc.items}
			lines, _ := prepareTurnMemoryLaneLines(selection, nil, tc.items)
			t.Logf("cards=%d expected=%d text=%s", len(lines), len(tc.items), strings.Join(lines, "\n"))
			if len(lines) != len(tc.items) {
				t.Error("distinct recorded secrets merged")
			}
			for _, item := range tc.items {
				claim := stringFromMap(mapFromAny(sliceFromAny(parseJSONMap(item.SummaryJSON)["protected_secrets"])[0]), "summary")
				if !strings.Contains(strings.Join(lines, "\n"), claim) {
					t.Errorf("lost claim %q", claim)
				}
			}
			if before != mustCompactJSON(tc.items) {
				t.Fatal("stored rows changed")
			}
		})
	}
}

func TestAuditfixTrueProtectedDuplicatesAndScopes(t *testing.T) {
	claim := "Mira keeps the key beneath the floor."
	first := auditfixSecret(1, 1, claim, "", []string{"Mira"})
	second := auditfixSecret(2, 2, claim, "", []string{"Mira"})
	items := []store.Memory{first, second, first}
	before := mustCompactJSON(items)
	lines, trace := prepareTurnMemoryLaneLines(prepareTurnMemoryLaneSelection{ProtectedSelected: items, ProtectedCandidates: items}, nil, items)
	if len(lines) != 1 {
		t.Fatalf("true duplicates cost more than one card: %v", lines)
	}
	lineage := prepareTurnMemoryLineageSlice(trace["delivery_lineage_items"])
	if len(lineage) != 1 || intFromAny(mapFromAny(lineage[0])["merged_source_count"], 0) != 2 {
		t.Fatalf("source lineage lost: %#v", lineage)
	}
	if before != mustCompactJSON(items) {
		t.Fatal("stored rows changed")
	}

	changed := parseJSONMap(second.SummaryJSON)
	secret := mapFromAny(sliceFromAny(changed["protected_secrets"])[0])
	secret["knowledge_scope"] = map[string]any{"known_by": []string{"Mira", "Rowan"}, "unknown_to": []string{"Visitor"}}
	second.SummaryJSON = mustCompactJSON(changed)
	items = []store.Memory{first, second}
	lines, _ = prepareTurnMemoryLaneLines(prepareTurnMemoryLaneSelection{ProtectedSelected: items, ProtectedCandidates: items}, nil, items)
	if len(lines) != len(items) {
		t.Fatalf("different recorded knowledge scopes merged: %v", lines)
	}
}

type auditfixPerspectiveStore struct {
	*perspectiveIdentityTestStore
	units []store.PreciseMemoryUnit
	t     *testing.T
}

func (s *auditfixPerspectiveStore) ListCharacterPerspectiveMemoryUnits(_ context.Context, sid, holder string) ([]store.PreciseMemoryUnit, error) {
	if sid != "auditfix-synthetic" || holder != "holder-mira" {
		s.t.Fatalf("unexpected perspective read: %s %s", sid, holder)
	}
	return s.units, nil
}

func auditfixPerspectiveUnit(id, state, claim string, turn int) store.PreciseMemoryUnit {
	return store.PreciseMemoryUnit{UnitID: id, ChatSessionID: "auditfix-synthetic", Kind: "observation", Subtype: "plan", LifecycleState: "active",
		KnowledgeHolderEntityID: "holder-mira", EpistemicMode: state, SourceTurnStart: turn, SourceTurnEnd: turn,
		PayloadJSON: mustCompactJSON(map[string]any{"contract_version": "perspective_memory.v1", "knowledge_holder_entity_id": "holder-mira", "knowledge_holder": "Mira", "epistemic_state": state,
			"subject": "Mira", "state_slot": "plan", "secret_kind": "plan", "owner": "Mira", "claim": claim, "disclosure_policy": "owner_private_until_revealed"})}
}

func auditfixPreparePerspective(t *testing.T, memories []store.Memory, units []store.PreciseMemoryUnit, cap int, pov string) map[string]any {
	t.Helper()
	fake := &auditfixPerspectiveStore{perspectiveIdentityTestStore: &perspectiveIdentityTestStore{turnRecordingStore: &turnRecordingStore{returnMemories: memories}, resolvedID: "holder-mira"}, units: units, t: t}
	before := mustCompactJSON([]any{memories, units})
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := mustCompactJSON(map[string]any{"chat_session_id": "auditfix-synthetic", "turn_index": 4, "raw_user_input": "Mira checks the sealed vault.",
		"client_meta": map[string]any{"perspective_context": map[string]any{"current_pov": pov}},
		"settings":    map[string]any{"max_injection_chars": cap, "protected_secret_budget_chars": 5000, "injection_enabled": true, "guide_strength": "none", "top_k": 5}})
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare status=%d: %s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if before != mustCompactJSON([]any{fake.returnMemories, fake.units}) {
		t.Fatal("stored rows changed")
	}
	return mapFromAny(response["injection_pack"])
}

func TestAuditfixSecretPerspectiveRepeatedBudgets(t *testing.T) {
	claim := "AZURE_VAULT_731 " + strings.Repeat("synthetic sealed vault access phrase. ", 18)
	memories := []store.Memory{auditfixSecret(1, 1, claim, "", []string{"Mira"})}
	units := []store.PreciseMemoryUnit{auditfixPerspectiveUnit("unit-known", "known", claim, 1)}
	var firstSecret string
	for _, cap := range []int{1600, 800, 1600, 800} {
		pack := auditfixPreparePerspective(t, memories, units, cap, "Mira")
		plan := mapFromAny(pack["memory_delivery_plan"])
		budget := mapFromAny(plan["protected_secret_budget"])
		text := stringFromMap(budget, "final_text")
		t.Logf("main_cap=%d secret_cap=%v secret_used=%v selected=%v occurrences=%d", cap, budget["cap_chars"], budget["used_chars"], budget["selected_count"], strings.Count(text, "AZURE_VAULT_731"))
		if strings.Count(text, "AZURE_VAULT_731") != 1 {
			t.Errorf("same source/body delivered twice or lost: %s", text)
		}
		if strings.Contains(stringFromMap(plan, "main_memory_text"), "AZURE_VAULT_731") {
			t.Error("secret entered main memory")
		}
		if intFromAny(budget["used_chars"], 0) != len([]rune(text)) {
			t.Error("secret accounting differs from rendered text")
		}
		packet := mapFromAny(pack["character_perspective_packet"])
		if intFromAny(mapFromAny(packet["dropped_counts"])["same_source_protected_secret"], 0) != len(units) || boolFromAny(packet["truncated"]) {
			t.Error("duplicate reached perspective budget accounting", packet)
		}
		if firstSecret == "" {
			firstSecret = text
		} else if text != firstSecret {
			t.Error("main budget changed independent secret delivery")
		}
	}
}

func TestAuditfixPerspectiveConservesIndependentClaims(t *testing.T) {
	claim := "Mira knows the sealed vault phrase."
	memory := auditfixSecret(1, 1, claim, "", []string{"Mira"})
	for _, tc := range []struct {
		name, state     string
		turn            int
		projectionClaim string
	}{
		{"different_source", "known", 2, claim},
		{"different_body", "known", 1, "Mira knows the alternate sealed vault route."},
		{"suspected", "suspected", 1, claim},
		{"misinformed", "misinformed", 1, claim},
		{"revealed", "revealed", 1, claim},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unit := auditfixPerspectiveUnit("independent-"+tc.name, tc.state, tc.projectionClaim, tc.turn)
			pack := auditfixPreparePerspective(t, []store.Memory{memory}, []store.PreciseMemoryUnit{unit}, 1600, "Mira")
			text := stringFromMap(mapFromAny(mapFromAny(pack["memory_delivery_plan"])["protected_secret_budget"]), "final_text")
			if !strings.Contains(text, "- "+tc.state+" | Mira / plan: "+tc.projectionClaim) {
				t.Fatalf("independent perspective lost: %s", text)
			}
		})
	}
	for _, artifact := range []string{"", "seal-A"} {
		t.Run("duplicate_artifact_"+artifact, func(t *testing.T) {
			stored := auditfixSecret(1, 1, claim, artifact, []string{"Mira"})
			unit := auditfixPerspectiveUnit("same-source", "known", claim, 1)
			pack := auditfixPreparePerspective(t, []store.Memory{stored}, []store.PreciseMemoryUnit{unit}, 1600, "Mira")
			text := stringFromMap(mapFromAny(mapFromAny(pack["memory_delivery_plan"])["protected_secret_budget"]), "final_text")
			if strings.Count(text, claim) != 1 {
				t.Fatalf("duplicate body cost twice: %s", text)
			}
		})
	}
	wrongHolder := auditfixPerspectiveUnit("wrong-holder", "known", claim, 1)
	wrongHolder.KnowledgeHolderEntityID = "holder-visitor"
	pack := auditfixPreparePerspective(t, []store.Memory{memory}, []store.PreciseMemoryUnit{wrongHolder}, 1600, "Mira")
	if intFromAny(mapFromAny(mapFromAny(pack["character_perspective_packet"])["dropped_counts"])["wrong_knowledge_holder"], 0) != 1 {
		t.Fatal("holder authorization changed")
	}
}

func TestAuditfixPerspectiveDistinctArtifactIDs(t *testing.T) {
	units := []store.PreciseMemoryUnit{}
	for _, artifact := range []string{"seal-A", "seal-B"} {
		unit := auditfixPerspectiveUnit(artifact, "known", "Mira hides a duplicate seal.", 1)
		payload := parseJSONMap(unit.PayloadJSON)
		payload["artifact_id"] = artifact
		unit.PayloadJSON = mustCompactJSON(payload)
		units = append(units, unit)
	}
	before := mustCompactJSON(units)
	packet, text := buildCharacterPerspectivePacket(units, map[string]any{"current_pov": "Mira", "current_pov_entity_id": "holder-mira", "identity_state": "resolved"}, 1600)
	if intFromAny(packet["candidate_count"], 0) != len(units) || strings.Count(text, "Mira hides a duplicate seal.") != len(units) {
		t.Fatalf("explicit perspective artifacts merged: %s", text)
	}
	if mustCompactJSON(units) != before {
		t.Fatal("stored perspective rows changed")
	}
}
