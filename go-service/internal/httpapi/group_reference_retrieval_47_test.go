package httpapi

import (
	"context"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func Test47ReferenceLimitCountsDeliveredMissingSources(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.entities = []store.ReferenceEntity{{EntityID: "vela", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Vela", ReviewStatus: "approved"}}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "b", ChatSessionID: "s", WorkID: "work-1", ContinuityID: "continuity-1", Enabled: true, ReferenceMode: referenceModeSupplement}}
	for _, row := range []struct{ id, text string }{{"covered", "Vela wears a blue cloak."}, {"z-needed", "Vela's cloak opens the gate only at dusk, except during a storm."}, {"a-other", "Vela bought the cloak at the southern market."}} {
		fake.claims = append(fake.claims, store.ReferenceClaim{ClaimID: row.id, WorkID: "work-1", ContinuityID: "continuity-1", SubjectEntityID: "vela", ClaimType: "fact", ClaimText: row.text, TemporalScope: "timeless", KnowledgeScope: "public_world", BranchKey: "main", ReviewStatus: "approved"})
	}
	embedding, _ := referenceVectorEmbeddingServer(t)
	defer embedding.Close()
	vs := &referenceVectorTestStore{}
	for i, c := range fake.claims {
		vs.exactResults = append(vs.exactResults, vector.ExactQueryResult{Document: referenceRecallVectorDocument("claim", c.ClaimID), ChromaRank: i + 1})
	}
	srv := referenceRecallTestServer(fake, vs, embedding.URL)
	result := srv.buildSessionReferenceRecallWithMessages(context.Background(), "s", "Vela tries to open the gate with the cloak.", 1, nil, []map[string]any{{"role": "system", "content": "Vela\n" + fake.claims[0].ClaimText}})
	if len(result.InjectionItems) != 1 || result.InjectionItems[0].SourceID != fake.claims[1].ClaimID {
		t.Fatalf("covered result consumed the limit or Chroma order was lost: %s", mustCompactJSON(result.InjectionItems))
	}
	item := result.InjectionItems[0]
	if item.ChromaRank == nil || *item.ChromaRank != 2 || !strings.Contains(item.Text, fake.claims[1].ClaimText) {
		t.Fatalf("rank or complete constraint lost: %#v", item)
	}
}

func Test47ReferenceIndirectDescriptionReachesSupplementDelivery(t *testing.T) {
	for _, tc := range []struct{ name, description, query string }{
		{"object", "푸른 귀환 도구는 새벽에만 문을 열며 폭풍 중에는 사용할 수 없다.", "일행은 푸른 귀환 도구를 써서 문을 열려고 한다."},
		{"person", "The silver-haired captain guards the northern gate.", "We seek the silver-haired captain."},
		{"place", "The pavilion with a blue roof stands beside the lake.", "They rest at the pavilion with a blue roof."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newReferenceBindingHTTPStore()
			fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
			fake.entities = []store.ReferenceEntity{{EntityID: "source", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Velaris", DescriptionText: tc.description, ReviewStatus: "approved"}}
			fake.bindings = []store.SessionReferenceBinding{{BindingID: "b", ChatSessionID: "s", WorkID: "work-1", ContinuityID: "continuity-1", Enabled: true, ReferenceMode: referenceModeSupplement}}
			embedding, _ := referenceVectorEmbeddingServer(t)
			defer embedding.Close()
			vs := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{{Document: referenceRecallVectorDocument("entity", "source"), ChromaRank: 1}}}
			srv := referenceRecallTestServer(fake, vs, embedding.URL)
			for _, q := range []string{tc.query, "Everyone eats breakfast and discusses the weather."} {
				result := srv.buildSessionReferenceRecall(context.Background(), "s", q, 3, nil)
				if q == tc.query {
					if len(result.InjectionItems) != 1 || !strings.Contains(result.InjectionItems[0].Text, tc.description) {
						t.Fatalf("description without name lost complete source: %s", mustCompactJSON(result))
					}
				} else if len(result.InjectionItems) != 0 {
					t.Fatalf("unrelated scene injected reference: %s", mustCompactJSON(result.InjectionItems))
				}
			}
		})
	}
}
