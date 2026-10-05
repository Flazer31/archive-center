package httpapi

import (
	"strings"
	"testing"
)

func TestPrivateEvidenceReview2PublicEventWithinMixedCitation(t *testing.T) {
	const public = "The harbor beacon shines blue."
	const private = public + " Mira privately worried about her hidden identity."
	ex := map[string]any{
		"protected_secrets": []any{map[string]any{"owner": "Mira", "evidence_excerpt": private}},
		"narrative_events":  []any{map[string]any{"visibility": "public", "event": public, "evidence_excerpt": public}},
	}
	before := mustCompactJSON(ex)
	for _, p := range []publicMemoryProjection{buildPublicMemoryProjection(ex, "", private), buildPublicMemoryProjection(ex, memoryAdmissionEvidenceJSON(ex, private))} {
		if !p.Eligible || !strings.Contains(p.SearchText.Text, public) {
			t.Errorf("explicit public event lost: %+v", p)
		}
		if strings.Contains(p.SearchText.Text, "hidden identity") || strings.Contains(mustCompactJSON(p.Extraction), "evidence_excerpt") {
			t.Error("private source citation reached public projection")
		}
	}
	if mustCompactJSON(ex) != before {
		t.Error("canonical extraction changed")
	}
}

func TestPrivateEvidenceReview2NormalizedPublicDisclosure(t *testing.T) {
	const public = "The harbor beacon shines blue."
	const source = "Mira announces to everyone: " + public
	ex := normalizeCriticExtraction(map[string]any{
		"turn_summary": public, "evidence_excerpts": []any{public},
		"protected_secrets": []any{map[string]any{
			"owner": "Mira", "summary": "Beacon color", "evidence_excerpt": source,
			"disclosure_policy": "public", "knowledge_scope": map[string]any{"publicly_revealed": true},
		}},
	})
	if protectedSecretRequiresGuard(mapFromAny(sliceFromAny(ex["protected_secrets"])[0]), "disclosure_policy") {
		t.Fatal("fixture disclosure policy not public")
	}
	p := buildPublicMemoryProjection(ex, "", source)
	if !p.Eligible || !strings.Contains(p.SearchText.Text, public) {
		t.Errorf("normalized public disclosure hidden: %+v; subjective=%s", p, mustCompactJSON(ex["subjective_entity_memories"]))
	}
	// An independently recorded private recollection must still own its scope.
	const private = "Mira privately recalled the amber vault code."
	ex["subjective_entity_memories"] = append(sliceFromAny(ex["subjective_entity_memories"]), map[string]any{"owner_entity_name": "Mira", "owner_visibility": "owner_private", "memory_text": "A private code", "evidence_excerpt": private})
	ex["evidence_excerpts"] = []any{public, private}
	p = buildPublicMemoryProjection(ex, "", source+" "+private)
	if strings.Contains(p.SearchText.Text, private) || !strings.Contains(p.SearchText.Text, public) {
		t.Error("independent private/public scope changed")
	}
}

func TestPrivateEvidenceReview2RestrictedObjectiveEvidence(t *testing.T) {
	const private = "The vault code is amber."
	const public = "The harbor beacon shines blue."
	for _, lane := range []string{"state_claims", "narrative_events", "world_rules", "character_deltas", "pending_threads", "reversible_states", "physical_conditions", "entity_conditions"} {
		t.Run(lane, func(t *testing.T) {
			ex := map[string]any{
				lane:                []any{map[string]any{"visibility": "restricted", "summary": private, "evidence_excerpt": private}},
				"evidence_excerpts": []any{private, public},
			}
			before := mustCompactJSON(ex)
			p := buildPublicMemoryProjection(ex, "", private+" "+public)
			if strings.Contains(p.SearchText.Text, private) {
				t.Error("restricted objective quotation rebuilt public summary")
			}
			if !p.Eligible || !strings.Contains(p.SearchText.Text, public) || mustCompactJSON(ex) != before {
				t.Error("public control or canonical extraction changed")
			}
		})
	}
}
