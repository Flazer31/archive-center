package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func TestIdentityPublicR2NormalizedDisclosurePolicy(t *testing.T) {
	const announcement = "Mira publicly identifies herself as Neris Vale."
	for _, tc := range []struct {
		name, policy string
		scope        map[string]any
	}{
		{"public_policy_with_reveal_scope", "public", map[string]any{"publicly_revealed": true}},
		{"publicly_revealed", "owner_private_until_revealed", map[string]any{"publicly_revealed": true}},
		{"reader_visible", "owner_private_until_revealed", map[string]any{"reader_visible": true}},
		{"protagonist_visible", "owner_private_until_revealed", map[string]any{"protagonist_visible": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ex := normalizeCriticExtraction(map[string]any{
				"turn_summary": announcement, "evidence_excerpts": []any{announcement},
				"character_identity_accuracy": []any{map[string]any{
					"surface_identity_name": "Mira", "true_identity_name": "Neris Vale", "same_entity": true,
					"reveal_policy": tc.policy, "knowledge_scope": tc.scope, "evidence_excerpt": announcement,
				}},
			})
			identities := sliceFromAny(ex["character_identity_accuracy"])
			if len(identities) != 1 || protectedSecretRequiresGuard(mapFromAny(identities[0]), "reveal_policy") {
				t.Fatal("normalized fixture must use the existing public-disclosure policy")
			}
			if len(sliceFromAny(ex["subjective_entity_memories"])) != 0 {
				t.Error("public identity manufactured a private subjective guard")
			}
			before := mustCompactJSON(ex)
			for _, p := range []publicMemoryProjection{
				buildPublicMemoryProjection(ex, "", announcement),
				buildPublicMemoryProjection(ex, memoryAdmissionEvidenceJSON(ex, announcement)),
			} {
				if !p.Eligible || !strings.Contains(p.SearchText.Text, announcement) {
					t.Errorf("public announcement lost: eligible=%v search=%q", p.Eligible, p.SearchText.Text)
				}
			}
			if mustCompactJSON(ex) != before {
				t.Error("projection changed the canonical identity record")
			}
		})
	}
}

func TestIdentityPublicR2DerivedCopyPreservesIndependentRecollection(t *testing.T) {
	private := map[string]any{"owner_entity_name": "Neris Vale", "owner_entity_key": "nerisvale", "owner_visibility": "owner_private", "memory_text": "Mira privately remembers the violet vault password."}
	existing := []any{private}
	identities := []any{
		map[string]any{"canonical_entity_name": "Neris Vale", "reveal_policy": "public", "knowledge_scope": map[string]any{"publicly_revealed": true}},
		map[string]any{"canonical_entity_name": "Sable Orr", "reveal_policy": "owner_private_until_revealed"},
	}
	before := mustCompactJSON(map[string]any{"existing": existing, "identities": identities})
	out := appendIdentityAccuracySubjectiveMemories(existing, identities)
	if len(out) != 2 || mustCompactJSON(out[0]) != mustCompactJSON(private) {
		t.Fatal("independent private recollection was replaced or public identity added a guard")
	}
	guard := mapFromAny(out[1])
	if stringFromMap(guard, "owner_entity_name") != "Sable Orr" || !boolFromAny(guard["secret_guard"]) || stringFromMap(guard, "owner_visibility") != "owner_private" {
		t.Error("private identity no longer derives its existing owner-private guard")
	}
	if before != mustCompactJSON(map[string]any{"existing": existing, "identities": identities}) {
		t.Error("derivation mutated its inputs")
	}
}

func TestIdentityPublicR2AdmissionAndDelivery(t *testing.T) {
	const sid = "identity-public-r2-synthetic"
	const announcement = "Mira publicly identifies herself as Neris Vale."
	const privateIdentity = "Ilan privately admits his true identity is Sable Orr."
	const privateRecollection = "Mira privately remembers the violet vault password."
	const source = announcement + " " + privateIdentity + " " + privateRecollection
	for _, accepted := range []bool{false, true} {
		name := "legacy"
		if accepted {
			name = "admission"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
			ex := normalizeCriticExtraction(map[string]any{
				"turn_summary": source, "evidence_excerpts": []any{announcement, privateIdentity, privateRecollection},
				"character_identity_accuracy": []any{
					map[string]any{"surface_identity_name": "Mira", "true_identity_name": "Neris Vale", "same_entity": true, "reveal_policy": "public", "knowledge_scope": map[string]any{"publicly_revealed": true}, "evidence_excerpt": announcement},
					map[string]any{"surface_identity_name": "Ilan", "true_identity_name": "Sable Orr", "same_entity": true, "reveal_policy": "owner_private_until_revealed", "knowledge_scope": map[string]any{"known_by": []any{"Ilan"}, "unknown_to": []any{"Visitor"}}, "evidence_excerpt": privateIdentity},
				},
				"subjective_entity_memories": []any{map[string]any{"owner_entity_name": "Mira", "owner_entity_key": "mira", "owner_entity_role": "npc", "owner_visibility": "owner_private", "memory_text": privateRecollection, "evidence_excerpt": privateRecollection}},
			})
			identitiesBefore := mustCompactJSON(ex["character_identity_accuracy"])
			memories := sliceFromAny(ex["subjective_entity_memories"])
			if len(memories) != 2 || stringFromMap(mapFromAny(memories[0]), "memory_text") != privateRecollection || stringFromMap(mapFromAny(memories[0]), "owner_visibility") != "owner_private" {
				t.Fatal("normalizer failed to retain the independent private recollection and private identity guard")
			}
			st := &revalidationAdmissionStore{revalidationSourceStore: &revalidationSourceStore{revalidationStore: &revalidationStore{turnRecordingStore: &turnRecordingStore{}}}}
			cfg := config.Default()
			cfg.StoreMode = config.StoreModeDualShadow
			srv := NewServer(cfg)
			srv.Store, srv.StoreOpenError = st, nil
			ctx := context.Background()
			if accepted {
				ctx = context.WithValue(ctx, entityIdentitySourceContextKey{}, entityIdentitySourceContext{ContractVersion: completeTurnSourceAcceptanceContract, Revision: "offline-rev-1"})
			} else {
				srv.Store = st.revalidationStore
			}
			result := srv.saveCriticExtractionArtifacts(ctx, sid, 1, ex, source, completeTurnEmbeddingConfig{}, time.Unix(300, 0))
			if result.Errors != 0 || len(st.savedMemories) != 1 || (accepted && len(st.admissions) != 1) {
				t.Fatalf("production save path not exercised: result=%+v memories=%d admissions=%d", result, len(st.savedMemories), len(st.admissions))
			}
			stored := *st.savedMemories[0]
			canonical := parseJSONMap(stored.SummaryJSON)
			if mustCompactJSON(canonical["character_identity_accuracy"]) != identitiesBefore {
				t.Fatal("save changed or discarded canonical identity records")
			}
			foundRecollection, foundGuard := false, false
			for _, raw := range sliceFromAny(canonical["subjective_entity_memories"]) {
				m := mapFromAny(raw)
				if stringFromMap(m, "memory_text") == privateRecollection && stringFromMap(m, "owner_visibility") == "owner_private" && stringFromMap(m, "evidence_excerpt") == privateRecollection {
					foundRecollection = true
				}
				if stringFromMap(m, "owner_entity_name") == "Sable Orr" && boolFromAny(m["secret_guard"]) && stringFromMap(m, "owner_visibility") == "owner_private" {
					foundGuard = true
				}
			}
			if !foundRecollection || !foundGuard {
				t.Fatal("save discarded independent private recollection or private identity guard")
			}
			if !accepted {
				stored.ID = 1
				st.returnMemories = append(st.returnMemories, stored)
			}
			seen := map[string]bool{}
			for _, e := range st.returnEvidence {
				want := "perspective_scoped_turn_excerpt"
				if e.EvidenceText == announcement {
					want = "turn_excerpt"
				}
				if e.EvidenceText != announcement && e.EvidenceText != privateIdentity && e.EvidenceText != privateRecollection {
					t.Fatalf("unexpected evidence: %+v", e)
				}
				seen[e.EvidenceText] = true
				if e.EvidenceKind != want {
					t.Errorf("evidence %q kind=%s want=%s", e.EvidenceText, e.EvidenceKind, want)
				}
			}
			if len(seen) != 3 {
				t.Fatal("save lost the public announcement or either private source quote")
			}
			projection := buildPublicMemoryProjection(parseJSONMap(stored.SummaryJSON), stored.Evidence)
			if !projection.Eligible || !strings.Contains(projection.SearchText.Text, announcement) || strings.Contains(projection.SearchText.Text, privateIdentity) || strings.Contains(projection.SearchText.Text, privateRecollection) {
				t.Errorf("stored projection violated the mixed boundary: %q", projection.SearchText.Text)
			}
			response := revalidationHTTP(t, srv, map[string]any{
				"chat_session_id": sid, "turn_index": 2, "raw_user_input": "Visitor asks about Mira Neris Vale Ilan Sable Orr violet vault password",
				"client_meta": map[string]any{"perspective_context": map[string]any{"current_pov": "Visitor"}},
				"settings":    map[string]any{"injection_enabled": true, "max_injection_chars": 30000, "input_context_enabled": false, "top_k": 5},
			})
			plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
			for label, text := range map[string]string{
				"main":    stringFromMap(plan, "main_memory_text"),
				"payload": stringFromMap(mapFromAny(response["payload_application_plan"]), "auxiliary_text"),
			} {
				if !strings.Contains(text, announcement) {
					t.Errorf("public announcement missing from %s: %s", label, text)
				}
				if strings.Contains(text, privateIdentity) || strings.Contains(text, privateRecollection) {
					t.Errorf("independent private source reached general %s: %s", label, text)
				}
			}
		})
	}
}
