package httpapi

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// These 3.6-A tests characterize the production owners and data shape before
// the identity/source migration starts. They intentionally record current
// limitations as baselines. 3.6-B must replace the relevant negative
// assertions when the versioned identity contract is implemented.

func Test36BNearNameCanonicalizationDoesNotCollapseDistinctPeople(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{
			{
				ChatSessionID: "sess-36a-near-name",
				CharacterName: "Marina",
			},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake

	got := srv.canonicalCharacterName(context.Background(), "sess-36a-near-name", "Marino")
	if got != "Marino" {
		t.Fatalf("3.6-B must not collapse a near-name into another identity, got %q", got)
	}
}

func Test36BCanonicalCharacterNameRequiresExactFullDisplaySurface(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{
			{ChatSessionID: "sess-36b-exact-name", CharacterName: "박한얼"},
			{ChatSessionID: "sess-36b-exact-name", CharacterName: "김한얼"},
			{ChatSessionID: "sess-36b-exact-name", CharacterName: "이시우"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake

	if got := srv.canonicalCharacterName(context.Background(), "sess-36b-exact-name", "박한얼"); got != "박한얼" {
		t.Fatalf("exact full display surface was not retained, got %q", got)
	}
	if got := srv.canonicalCharacterName(context.Background(), "sess-36b-exact-name", "한얼"); got != "한얼" {
		t.Fatalf("suffix surface must not merge to a stored full name, got %q", got)
	}
	if got := srv.canonicalCharacterName(context.Background(), "sess-36b-exact-name", "Siwoo"); got != "Siwoo" {
		t.Fatalf("romanized surface must not merge to a stored display name, got %q", got)
	}
}

func Test36ABaselineOneAggregateMemoryPerSourceTurn(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{
				ID:            36,
				ChatSessionID: "sess-36a-aggregate",
				TurnIndex:     12,
				SummaryJSON:   `{"turn_summary":"The first aggregate memory for this turn."}`,
			},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake

	result := srv.saveCriticExtractionArtifacts(
		context.Background(),
		"sess-36a-aggregate",
		12,
		map[string]any{
			"turn_summary":     "A second fact from another speaker in the same turn.",
			"importance_score": 7,
		},
		"A second fact from another speaker in the same turn.",
		completeTurnEmbeddingConfig{},
		time.Unix(360, 0),
	)

	if result.Errors != 0 {
		t.Fatalf("same-turn aggregate baseline should skip without error, result=%#v", result)
	}
	if result.Memories != 0 || len(fake.savedMemories) != 0 {
		t.Fatalf("3.6-A baseline changed: a second memory was inserted for one source turn, result=%#v saved=%#v", result, fake.savedMemories)
	}
	for _, item := range result.SkipReasons {
		row := mapFromAny(item)
		if stringFromMap(row, "surface") == "memories" && stringFromMap(row, "reason") == "duplicate_source_turn_memory" {
			return
		}
	}
	t.Fatalf("duplicate_source_turn_memory baseline reason missing: %#v", result.SkipReasons)
}

func Test36BTypedIdentityContractAndRemainingSourceLineageGaps(t *testing.T) {
	requireFieldsPresent(t, reflect.TypeOf(store.DirectEvidence{}),
		"SourceMessageIDsJSON",
		"SourceHash",
		"LineageJSON",
		"Tombstoned",
		"SupersededByID",
	)

	requireFieldsAbsent(t, reflect.TypeOf(store.Memory{}),
		"SourceMessageID",
		"SourceRevision",
		"BranchID",
		"SpeakerEntityID",
		"MemoryUnitKind",
	)
	requireFieldsPresent(t, reflect.TypeOf(store.EntityIdentity{}),
		"StableEntityID",
		"IdentityNamespace",
		"SourceRevision",
		"LifecycleState",
		"ReviewState",
		"IdempotencyKey",
	)
	requireFieldsPresent(t, reflect.TypeOf(store.EntityIdentityArtifactBinding{}),
		"StableEntityID",
		"ArtifactKind",
		"ArtifactRole",
		"SourceRevision",
	)
	requireFieldsPresent(t, reflect.TypeOf(store.SpeakerAttribution{}),
		"SpeakerEntityID",
		"AttributionState",
		"EvidenceExcerpt",
		"SourceSpanStart",
		"SourceRevision",
	)
}

func requireFieldsPresent(t *testing.T, typ reflect.Type, fields ...string) {
	t.Helper()
	for _, field := range fields {
		if _, ok := typ.FieldByName(field); !ok {
			t.Fatalf("%s.%s must remain present in the 3.6-A source-lineage baseline", typ.Name(), field)
		}
	}
}

func requireFieldsAbsent(t *testing.T, typ reflect.Type, fields ...string) {
	t.Helper()
	for _, field := range fields {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatalf("%s.%s now exists; replace this 3.6-A gap assertion with the versioned migration contract test", typ.Name(), field)
		}
	}
}
