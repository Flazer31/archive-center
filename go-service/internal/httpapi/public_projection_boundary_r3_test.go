package httpapi

import "testing"

func TestPublicProjectionBoundaryR3RepeatedReaders(t *testing.T) {
	boundary := map[string]any{
		"fact_ref":         "synthetic-instruction",
		"knowledge_scope":  map[string]any{"known_by": []any{"Courier"}, "unknown_to": []any{"Observer"}},
		"knowledge_source": map[string]any{"source_revision": "synthetic-prior", "root_evidence_id": 17},
	}
	extraction := map[string]any{"state_claims": []any{map[string]any{
		"subject": "Courier", "state_slot": "goal_status", "lifecycle_key": "synthetic-delivery",
		"value": "The delivery remains open", "knowledge_boundaries": []any{boundary},
	}}}
	before := mustCompactJSON(extraction)
	want := mustCompactJSON(boundary)
	for _, pass := range []string{"first_projection", "second_projection"} {
		projection := buildPublicMemoryProjection(extraction, "", "The delivery remains open")
		item := mapFromAny(sliceFromAny(projection.Extraction["state_claims"])[0])
		t.Run(pass, func(t *testing.T) {
			boundaries := prepareTurnOwnKnowledgeBoundary(item)
			if len(boundaries) != 1 || mustCompactJSON(boundaries[0]) != want {
				t.Fatal("an in-memory public projection lost its attributed boundary at the existing reader")
			}
			if len(mapFromAny(item["knowledge_scope"])) != 0 {
				t.Fatal("historical quotation scope became goal own scope")
			}
		})
		if pass == "first_projection" && mustCompactJSON(extraction) != before {
			t.Fatal("projection mutated the caller input")
		}
		extraction = projection.Extraction
	}
}
