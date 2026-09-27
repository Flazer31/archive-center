package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test48MemoryRelationReadingCases(t *testing.T) {
	state := func(id int64, subject, subjectType, slot, value string) store.StatusCurrentValue {
		claim := narrativeStateClaim{Subject: subject, SubjectType: subjectType, StateSlot: slot, Value: value, ClaimScope: "objective", Transition: "set"}
		return store.StatusCurrentValue{ID: id, ChatSessionID: "synthetic", StatusKey: narrativeStateStatusKey, WriteState: "current",
			ValueJSON:    mustCompactJSON(narrativeStateValuePayload(claim, "", 12)),
			EvidenceJSON: mustCompactJSON(narrativeStateEvidencePayload(claim, []int64{id}, 12, "revision-now")), SourceTurn: 12}
	}
	t.Run("R01_direction", func(t *testing.T) {
		a := relationshipStateTestUnit("s", "r", 1, "a-b", "a", "b", "A", "B", "trust", "A privately trusts B", "owner_private")
		b := relationshipStateTestUnit("s", "r", 1, "b-a", "b", "a", "B", "A", "fear", "B fears A", "public")
		st := newPreciseMemoryRecordingStore()
		result := artifactSaveResult{}
		(&Server{Store: st}).saveRelationshipStatesFromPreciseMemoryUnits(context.Background(), "s", []*store.PreciseMemoryUnit{a, b}, time.Unix(100, 0), &result)
		if result.RelationCurrentStates != 2 {
			t.Fatalf("production fixture: %+v", result)
		}
		input := memoryRelationInput{PreciseUnits: []store.PreciseMemoryUnit{*a, *b}, CurrentStates: st.returnStatusCurrent, StateHistory: st.savedStatusEvents}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		for i, unit := range input.PreciseUnits {
			f := out.Records[i].Frames[0]
			link := relationLink(t, f, "relationship_observation")
			if link.From.EntityID != unit.ActorEntityID || link.To.EntityID != unit.AffectedEntityID || f.Knowledge["unit"].(map[string]any)["visibility"] != unit.Visibility {
				t.Fatalf("direction/scope lost: %+v", f)
			}
			if _, ok := f.Knowledge["reciprocity"]; ok {
				t.Fatal("invented reciprocity")
			}
		}
		for _, r := range out.Records[2:4] {
			f := r.Frames[0]
			if f.Knowledge["reciprocity"].(map[string]any)["state"] != "not_inferred" || f.Provenance["source"] == nil || f.Provenance["evidence"] == nil {
				t.Fatalf("stored scope/provenance lost: %+v", f)
			}
		}
	})
	t.Run("R02_item_transfer", func(t *testing.T) {
		old := state(1, "Brass compass", "item", "owner", "Mira")
		now := state(2, "Brass compass", "item", "owner", "Rowan")
		history := store.StatusChangeEvent{ID: 3, ChatSessionID: "synthetic", StatusKey: narrativeStateStatusKey, PreviousValueJSON: old.ValueJSON, NewValueJSON: now.ValueJSON, EvidenceJSON: `{"source_revision":"transfer"}`, StoryClockJSON: `{"observed_at":"dawn"}`}
		input := memoryRelationInput{CurrentStates: []store.StatusCurrentValue{now}, StateHistory: []store.StatusChangeEvent{history}}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		frames := out.Records[1].Frames
		if frames[0].Fields["value"] != "Mira" || frames[1].Fields["value"] != "Rowan" || frames[0].Path == frames[1].Path {
			t.Fatal("transfer collapsed history")
		}
		if frames[0].Provenance["evidence"] != nil || frames[0].Time["observed_at"] != nil {
			t.Fatal("new evidence/clock backdated previous state")
		}
		if relationLink(t, frames[0], "state_subject").Predicate != "owner" {
			t.Fatal("owner slot lost")
		}
	})
	t.Run("R03_place", func(t *testing.T) {
		row := state(1, "Blue pavilion", "location", "condition", "destroyed")
		out := readMemoryRelations(memoryRelationInput{CurrentStates: []store.StatusCurrentValue{row}})
		f := out.Records[0].Frames[0]
		link := relationLink(t, f, "state_subject")
		if link.From.Text != "Blue pavilion" || link.From.EntityID != "" || link.From.Kind != "unresolved_label" || f.Fields["value"] != "destroyed" {
			t.Fatalf("invented identity or changed condition: %+v", f)
		}
	})
	t.Run("R04_organization", func(t *testing.T) {
		rows := []store.KGTriple{{ID: 1, ChatSessionID: "s", Subject: "Captain", Predicate: "left", Object: "City Guard"}, {ID: 2, ChatSessionID: "s", Subject: "Captain", Predicate: "joined", Object: "City Merchants"}}
		input := memoryRelationInput{Triples: rows}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		a, b := out.Records[0].Frames[0].Links[0], out.Records[1].Frames[0].Links[0]
		if a.To == b.To || a.From == b.From || a.Kind != "unclassified" || b.Predicate != rows[1].Predicate {
			t.Fatal("merged names or classified free predicates")
		}
	})
	t.Run("R05_event_role", func(t *testing.T) {
		unit := store.PreciseMemoryUnit{UnitID: "collapse", ChatSessionID: "s", Kind: "event", ActorEntityID: "a", AffectedEntityID: "gate", LocationEntityID: "courtyard", PayloadJSON: `{"event":"A observed the gate collapse","participants":[{"name":"B","role":"witness"}]}`}
		out := readMemoryRelations(memoryRelationInput{PreciseUnits: []store.PreciseMemoryUnit{unit}})
		links := out.Records[0].Frames[0].Links
		if len(links) != 3 {
			t.Fatalf("lost stored entity roles: %+v", links)
		}
		for _, link := range links {
			if link.Kind != "record_role" || link.To.Kind != "event" {
				t.Fatal("co-occurrence became causality or a relationship")
			}
		}
	})
	t.Run("R06_commitment", func(t *testing.T) {
		input := memoryRelationInput{Threads: []store.PendingThread{{ID: 1, ChatSessionID: "s", ThreadKey: "bridge", Owner: "A", Target: "B", Status: "resolved", ResolvedTurn: 9, DetailsJSON: `{"transition":"complete","lifecycle_details":{"progress":"completed"},"due":"2002-06-04"}`}}}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		f := out.Records[0].Frames[0]
		if f.Lifecycle["status"] != "resolved" || f.Lifecycle["thread_key"] != "bridge" || f.Time["due"] != "2002-06-04" {
			t.Fatalf("completion/date lost: %+v", f)
		}
		if len(f.Links) != 2 || f.Links[0].Role != "owner" || f.Links[1].Role != "target" || f.Links[0].From.EntityID != "" {
			t.Fatal("commitment role became confirmed identity")
		}
		if _, exists := f.Knowledge["confidence"]; exists {
			t.Fatal("absent optional thread confidence became an assertion")
		}
	})
	t.Run("R07_recurrence", func(t *testing.T) {
		input := memoryRelationInput{Threads: []store.PendingThread{{ID: 1, ThreadKey: "watch-a", Status: "resolved", DetailsJSON: `{"lifecycle_details":{"occurrence":"first","recurring":true}}`}, {ID: 2, ThreadKey: "watch-b", Status: "open", DetailsJSON: `{"lifecycle_details":{"occurrence":"second","recurring":true}}`}}}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		if out.Records[0].Frames[0].Lifecycle["status"] == out.Records[1].Frames[0].Lifecycle["status"] {
			t.Fatal("completed occurrence closed continuing duty")
		}
	})
	t.Run("R08_field_time", func(t *testing.T) {
		row := state(1, "Mira", "character", "profession", "cartographer")
		row.ValueJSON = `{"subject":"Mira","value":"cartographer","source_turn":2,"source_fields":["/status/profession"],"observed_at":{"date":"2002-06-04"},"occurrence_time":"last winter","validity":{"valid_from":"last winter"}}`
		row.SourceTurn = 99
		row.UpdatedAt = time.Unix(9999, 0)
		out := readMemoryRelations(memoryRelationInput{CurrentStates: []store.StatusCurrentValue{row}})
		f := out.Records[0].Frames[0]
		if f.Provenance["source_turn"] != json.Number("2") || f.Time["occurrence_time"] != "last winter" || f.Time["observed_at"].(map[string]any)["date"] != "2002-06-04" {
			t.Fatal("record freshness replaced field time")
		}
	})
	t.Run("R09_secret_knower", func(t *testing.T) {
		unit := store.PreciseMemoryUnit{UnitID: "belief", Kind: "observation", TruthScope: "subjective", EpistemicMode: "suspected", Visibility: "owner_private", KnowledgeHolderEntityID: "a", Confidence: 0.25, RevealCondition: "only after confession", PayloadJSON: `{"belief":"B might be the heir"}`}
		out := readMemoryRelations(memoryRelationInput{PreciseUnits: []store.PreciseMemoryUnit{unit}})
		f := out.Records[0].Frames[0]
		k := f.Knowledge["unit"].(map[string]any)
		if k["truth_scope"] != unit.TruthScope || k["confidence"] != unit.Confidence || k["reveal_condition"] != unit.RevealCondition || relationLink(t, f, "knowledge_held_by").To.EntityID != "a" {
			t.Fatal("belief/knower changed")
		}
		if _, exists := f.Knowledge["magnitude"]; exists {
			t.Fatal("confidence became strength")
		}
	})
	t.Run("R10_conflict", func(t *testing.T) {
		a := state(1, "Gate", "item", "condition", "open")
		b := state(2, "Gate", "item", "condition", "closed")
		b.WriteState = "history_only"
		input := memoryRelationInput{CurrentStates: []store.StatusCurrentValue{a, b}}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		if out.Records[1].Frames[0].Lifecycle["write_state"] != b.WriteState {
			t.Fatal("history-only observation promoted or suppressed")
		}
	})
	t.Run("R11_shared_quote", func(t *testing.T) {
		a := store.PreciseMemoryUnit{UnitID: "promise", Kind: "event", EvidenceExcerpt: "A promised B the compass, and B feared losing it.", RootEvidenceID: 7, DirectEvidenceIDsJSON: `[7,9007199254740993]`, PayloadJSON: `{"event":"A made a promise"}`}
		b := a
		b.UnitID = "fear"
		b.Kind = "observation"
		b.PayloadJSON = `{"observation":"B feared loss"}`
		input := memoryRelationInput{PreciseUnits: []store.PreciseMemoryUnit{a, b}}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		if out.Records[0].Ref == out.Records[1].Ref || out.Records[0].Documents[1].Value.([]any)[1] != json.Number("9007199254740993") {
			t.Fatal("merged quote or rounded evidence ID")
		}
	})
	t.Run("R12_revision", func(t *testing.T) {
		a := store.PreciseMemoryUnit{UnitID: "old", SourceRevision: "r1", SourceLogicalTurnID: "logical-1", LifecycleState: "superseded", PayloadJSON: `{}`}
		b := a
		b.UnitID = "new"
		b.SourceRevision = "r2"
		b.LifecycleState = "active"
		input := memoryRelationInput{PreciseUnits: []store.PreciseMemoryUnit{a, b}}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		for i, unit := range input.PreciseUnits {
			if out.Records[i].Frames[0].Provenance["unit"].(map[string]any)["source_revision"] != unit.SourceRevision || out.Records[i].Frames[0].Lifecycle["lifecycle_state"] != unit.LifecycleState {
				t.Fatal("revision substituted by latest")
			}
		}
	})
	t.Run("R13_copy_scope", func(t *testing.T) {
		a := store.PreciseMemoryUnit{ID: 1, UnitID: "u", ChatSessionID: "original", ActorEntityID: "a", PayloadJSON: `{}`}
		b := a
		b.ChatSessionID = "copy"
		out := readMemoryRelations(memoryRelationInput{PreciseUnits: []store.PreciseMemoryUnit{a, b}})
		if out.Records[0].Frames[0].Links[0].From == out.Records[1].Frames[0].Links[0].From {
			t.Fatal("same ID in different session merged")
		}
		for _, r := range out.Records {
			if r.Frames[0].Provenance["branch"] != nil {
				t.Fatal("ordinary copy became a branch")
			}
		}
	})
	t.Run("R14_legacy_unknown", func(t *testing.T) {
		input := memoryRelationInput{Triples: []store.KGTriple{{Subject: "Mira", Predicate: " owned_by? ", Object: "Tower", SourceTurn: 20, ValidFrom: 3, ValidTo: 0}}}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		f := out.Records[0].Frames[0]
		if f.Links[0].Predicate != input.Triples[0].Predicate || f.Kind != "unclassified" || len(f.Knowledge) != 0 || f.Time["occurrence_time"] != nil {
			t.Fatal("legacy string gained authority/privacy/date")
		}
		for _, raw := range []string{"", "{", `{"subject":"Mira"} trailing`, `[]`, `null`} {
			row := state(1, "Mira", "character", "x", "y")
			row.ValueJSON = raw
			x := readMemoryRelations(memoryRelationInput{CurrentStates: []store.StatusCurrentValue{row}})
			if len(x.Records) != 1 || x.Records[0].Documents[0].RawJSON != raw || len(x.Records[0].Frames[0].Links) != 0 {
				t.Fatalf("invalid/unknown shape discarded or asserted: %q", raw)
			}
		}
	})
	t.Run("R15_read_only_repeatable", func(t *testing.T) {
		input := memoryRelationInput{PreciseUnits: []store.PreciseMemoryUnit{{UnitID: "x", PayloadJSON: `{"nested":{"value":"original"}}`, VectorEmbedding: []float32{1}, VectorContextChunks: []string{"original"}}}}
		a, b := readMemoryRelations(input), readMemoryRelations(input)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("read is not deterministic")
		}
		row := a.Records[0].SourceRow.(store.PreciseMemoryUnit)
		row.VectorEmbedding[0] = 5
		row.VectorContextChunks[0] = "changed"
		a.Records[0].Frames[0].Fields["nested"].(map[string]any)["value"] = "changed"
		if !reflect.DeepEqual(b, readMemoryRelations(input)) {
			t.Fatal("projection changed source / subsequent read")
		}
		emptySlices := memoryRelationInput{PreciseUnits: []store.PreciseMemoryUnit{{VectorEmbedding: []float32{}, VectorContextChunks: []string{}}}}
		assertRelationRows(t, emptySlices, readMemoryRelations(emptySlices))
	})
	t.Run("R16_no_expansion", func(t *testing.T) {
		input := memoryRelationInput{Triples: []store.KGTriple{{Subject: "A", Predicate: "knows", Object: "B"}, {Subject: "B", Predicate: "knows", Object: "C"}, {Subject: "A", Predicate: "knows", Object: "B"}}}
		out := readMemoryRelations(input)
		assertRelationRows(t, input, out)
		for _, r := range out.Records {
			if len(r.Frames[0].Links) != 1 {
				t.Fatal("transitive edge or duplicate collapse")
			}
		}
		if out.Records[0].Ref == out.Records[2].Ref {
			t.Fatal("unidentified occurrences share fallback identity")
		}
		if empty := readMemoryRelations(memoryRelationInput{}); len(empty.Records) != 0 {
			t.Fatal("invented placeholder record")
		}
	})
}

func relationLink(t *testing.T, frame memoryRelationFrame, kind string) memoryRelationLink {
	t.Helper()
	for _, link := range frame.Links {
		if link.Kind == kind {
			return link
		}
	}
	t.Fatalf("missing %s in %+v", kind, frame)
	return memoryRelationLink{}
}

func assertRelationRows(t *testing.T, input memoryRelationInput, out memoryRelationReading) {
	t.Helper()
	want := []any{}
	for _, row := range input.PreciseUnits {
		want = append(want, row)
	}
	for _, row := range input.CurrentStates {
		want = append(want, row)
	}
	for _, row := range input.StateHistory {
		want = append(want, row)
	}
	for _, row := range input.Threads {
		want = append(want, row)
	}
	for _, row := range input.Triples {
		want = append(want, row)
	}
	if out.Version != memoryRelationReadingVersion || len(out.Records) != len(want) {
		t.Fatalf("row preservation: got %d want %d", len(out.Records), len(want))
	}
	for i, row := range want {
		if !reflect.DeepEqual(out.Records[i].SourceRow, row) {
			t.Fatal(fmt.Sprintf("row %d changed: %#v != %#v", i, out.Records[i].SourceRow, row))
		}
	}
}
