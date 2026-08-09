package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBEntityIdentityWriteRequiresActiveAcceptedSourceRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 30, 1, 2, 3, 0, time.UTC)
	item := &EntityIdentity{
		StableEntityID: "entity-1",
		ChatSessionID:  "session-1",
		SourceContract: acceptedSourceObservationContract,
		SourceRevision: "revision-1",
		SourceTurn:     3,
		IdempotencyKey: "identity-key",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state").
		WithArgs("session-1", "revision-1").
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("active"))
	mock.ExpectExec("INSERT INTO entity_identities").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := m.SaveEntityIdentity(context.Background(), item); err != nil {
		t.Fatalf("active source write failed: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state").
		WithArgs("session-1", "revision-1").
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("superseded"))
	mock.ExpectRollback()
	if err := m.SaveEntityIdentity(context.Background(), item); !errors.Is(err, ErrSourceRevisionStale) {
		t.Fatalf("stale source error = %v, want ErrSourceRevisionStale", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBEntityIdentityLegacyWriteKeepsLegacyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	item := &EntityIdentity{
		StableEntityID: "entity-legacy",
		ChatSessionID:  "session-legacy",
		SourceContract: "legacy_unverified",
		SourceRevision: "legacy-revision",
		SourceTurn:     2,
		IdempotencyKey: "legacy-key",
	}

	mock.ExpectExec("INSERT INTO entity_identities").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := m.SaveEntityIdentity(context.Background(), item); err != nil {
		t.Fatalf("legacy write failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBEntityIdentityLinkWriteUsesExistingLinkTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	item := &EntityIdentityLink{
		LinkID: "link-1", ChatSessionID: "session-1", SourceEntityID: "short-name-1", TargetEntityID: "full-name-1",
		LinkKind: EntityIdentityLinkKindCanonicalEquivalence, LinkState: EntityIdentityLinkStateReviewed,
		EvidenceJSON: `{"evidence_excerpt":"Hyun Jiyu said, Call me Jiyu."}`, MappingRevision: 1,
		SourceContract: "legacy_unverified", SourceRevision: "legacy-revision",
	}
	mock.ExpectExec("INSERT INTO entity_identity_links").
		WithArgs(item.LinkID, item.ChatSessionID, item.SourceEntityID, item.TargetEntityID,
			item.LinkKind, item.LinkState, item.EvidenceJSON, item.MappingRevision, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := m.SaveEntityIdentityLink(context.Background(), item); err != nil {
		t.Fatalf("identity link write failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBResolveReviewedCanonicalEntityIDUnique(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}

	mock.ExpectQuery("SELECT DISTINCT identity_link.target_entity_id").
		WithArgs(
			"session-1",
			"occurrence-1",
			EntityIdentityLinkKindCanonicalEquivalence,
			EntityIdentityLinkStateReviewed,
			EntityIdentityReviewStateSourceObserved,
			EntityIdentityReviewStateReviewed,
		).
		WillReturnRows(sqlmock.NewRows([]string{"target_entity_id"}).AddRow("canonical-1"))

	target, err := m.ResolveReviewedCanonicalEntityID(context.Background(), "session-1", "occurrence-1")
	if err != nil {
		t.Fatalf("resolve reviewed canonical identity: %v", err)
	}
	if target != "canonical-1" {
		t.Fatalf("target = %q, want canonical-1", target)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBResolveReviewedCanonicalEntityIDMissingFailsClosed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}

	mock.ExpectQuery("SELECT DISTINCT identity_link.target_entity_id").
		WithArgs(
			"session-1",
			"occurrence-missing",
			EntityIdentityLinkKindCanonicalEquivalence,
			EntityIdentityLinkStateReviewed,
			EntityIdentityReviewStateSourceObserved,
			EntityIdentityReviewStateReviewed,
		).
		WillReturnRows(sqlmock.NewRows([]string{"target_entity_id"}))

	if _, err := m.ResolveReviewedCanonicalEntityID(context.Background(), "session-1", "occurrence-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing target error = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBResolveReviewedCanonicalEntityIDAmbiguousFailsClosed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}

	mock.ExpectQuery("SELECT DISTINCT identity_link.target_entity_id").
		WithArgs(
			"session-1",
			"occurrence-ambiguous",
			EntityIdentityLinkKindCanonicalEquivalence,
			EntityIdentityLinkStateReviewed,
			EntityIdentityReviewStateSourceObserved,
			EntityIdentityReviewStateReviewed,
		).
		WillReturnRows(sqlmock.NewRows([]string{"target_entity_id"}).
			AddRow("canonical-1").
			AddRow("canonical-2"))

	if _, err := m.ResolveReviewedCanonicalEntityID(context.Background(), "session-1", "occurrence-ambiguous"); !errors.Is(err, ErrReviewedEntityIdentityAmbiguous) {
		t.Fatalf("ambiguous target error = %v, want ErrReviewedEntityIdentityAmbiguous", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBResolveUniqueActiveEntityIdentityBySurfaceReturnsDatabaseNamespace(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rows    *sqlmock.Rows
		want    ResolvedEntityIdentity
		wantErr error
	}{
		{
			name: "unique",
			rows: sqlmock.NewRows([]string{
				"stable_entity_id", "surface_kind", "source_namespace", "source_kind", "source_label", "source_turn",
				"target_entity_id", "target_namespace", "target_kind", "target_label", "target_turn",
			}).
				AddRow("occurrence-1", "alias_0", "session_unknown", "speaker", "Alex", 2, "canonical-1", "session_npc", "character", "Alexander", 1).
				AddRow("occurrence-2", "alias_0", "session_unknown", "speaker", "Alex", 3, "canonical-1", "session_npc", "character", "Alexander", 1),
			want: ResolvedEntityIdentity{
				StableEntityID: "canonical-1", IdentityNamespace: "session_npc",
				EntityKind: "character", CanonicalLabel: "Alexander",
			},
		},
		{
			name: "duplicate exact canonical tuple keeps earliest ID",
			rows: sqlmock.NewRows([]string{
				"stable_entity_id", "surface_kind", "source_namespace", "source_kind", "source_label", "source_turn",
				"target_entity_id", "target_namespace", "target_kind", "target_label", "target_turn",
			}).
				AddRow("canonical-later", "display_name", "session_npc", "character", "Alex", 4, "", "", "", "", 0).
				AddRow("canonical-first", "display_name", "session_npc", "character", "Alex", 1, "", "", "", "", 0),
			want: ResolvedEntityIdentity{
				StableEntityID: "canonical-first", IdentityNamespace: "session_npc",
				EntityKind: "character", CanonicalLabel: "Alex",
			},
		},
		{
			name: "ambiguous",
			rows: sqlmock.NewRows([]string{
				"stable_entity_id", "surface_kind", "source_namespace", "source_kind", "source_label", "source_turn",
				"target_entity_id", "target_namespace", "target_kind", "target_label", "target_turn",
			}).
				AddRow("occurrence-1", "display_name", "session_npc", "character", "Alex", 1, "", "", "", "", 0).
				AddRow("occurrence-2", "display_name", "session_player", "character", "Alex", 2, "", "", "", "", 0),
			wantErr: ErrReviewedEntityIdentityAmbiguous,
		},
		{
			name: "different entity kinds never collapse",
			rows: sqlmock.NewRows([]string{
				"stable_entity_id", "surface_kind", "source_namespace", "source_kind", "source_label", "source_turn",
				"target_entity_id", "target_namespace", "target_kind", "target_label", "target_turn",
			}).
				AddRow("occurrence-1", "display_name", "session_unknown", "character", "Alex", 1, "", "", "", "", 0).
				AddRow("occurrence-2", "display_name", "session_unknown", "item", "Alex", 2, "", "", "", "", 0),
			wantErr: ErrReviewedEntityIdentityAmbiguous,
		},
		{
			name: "aliases never collapse duplicate IDs",
			rows: sqlmock.NewRows([]string{
				"stable_entity_id", "surface_kind", "source_namespace", "source_kind", "source_label", "source_turn",
				"target_entity_id", "target_namespace", "target_kind", "target_label", "target_turn",
			}).
				AddRow("occurrence-1", "alias_0", "session_npc", "character", "Alexander", 1, "", "", "", "", 0).
				AddRow("occurrence-2", "alias_0", "session_npc", "character", "Alexander", 2, "", "", "", "", 0),
			wantErr: ErrReviewedEntityIdentityAmbiguous,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			m := &mariadbStore{db: db}
			mock.ExpectQuery(`FROM entity_identity_surfaces surface[\s\S]+source_identity\.lifecycle_state = 'active'[\s\S]+source_revision\.lifecycle_state = 'active'[\s\S]+canonical_revision\.lifecycle_state = 'active'[\s\S]+surface\.surface_scope IN \(\?, \?\)[\s\S]+surface\.review_state = 'source_observed'`).
				WithArgs(
					EntityIdentityLinkKindCanonicalEquivalence,
					EntityIdentityLinkStateReviewed,
					EntityIdentityReviewStateSourceObserved,
					EntityIdentityReviewStateReviewed,
					"session-1", "alex", EntityIdentitySurfaceScope39, EntityIdentitySurfaceScopeCurrent,
				).
				WillReturnRows(tc.rows)
			got, err := m.ResolveUniqueActiveEntityIdentityBySurface(context.Background(), "session-1", "alex")
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("resolved=%#v err=%v want=%#v err=%v", got, err, tc.want, tc.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type reviewedResolverTestStore struct {
	Store
	target        string
	calls         int
	identity      ResolvedEntityIdentity
	identityCalls int
}

func (s *reviewedResolverTestStore) ResolveReviewedCanonicalEntityID(context.Context, string, string) (string, error) {
	s.calls++
	return s.target, nil
}

func (s *reviewedResolverTestStore) ResolveUniqueActiveEntityIdentityBySurface(context.Context, string, string) (ResolvedEntityIdentity, error) {
	s.identityCalls++
	return s.identity, nil
}

func TestReviewedCanonicalEntityResolverDelegatesAuthoritativeReads(t *testing.T) {
	authoritative := &reviewedResolverTestStore{Store: NewNoopStore(), target: "canonical-authoritative"}
	dual := NewDualWriteStore(NewNoopStore(), authoritative)
	resolver, ok := dual.(ReviewedEntityIdentityResolver)
	if !ok {
		t.Fatal("dual-write store does not expose reviewed identity resolver")
	}
	target, err := resolver.ResolveReviewedCanonicalEntityID(context.Background(), "session-1", "occurrence-1")
	if err != nil || target != authoritative.target || authoritative.calls != 1 {
		t.Fatalf("dual authoritative read target=%q calls=%d err=%v", target, authoritative.calls, err)
	}

	readOnly := NewReadOnlyStore(authoritative)
	resolver, ok = readOnly.(ReviewedEntityIdentityResolver)
	if !ok {
		t.Fatal("read-only store does not expose reviewed identity resolver")
	}
	target, err = resolver.ResolveReviewedCanonicalEntityID(context.Background(), "session-1", "occurrence-1")
	if err != nil || target != authoritative.target || authoritative.calls != 2 {
		t.Fatalf("read-only authoritative read target=%q calls=%d err=%v", target, authoritative.calls, err)
	}
}

func TestUniqueActiveEntitySurfaceIdentityResolverDelegatesDatabaseOwnedNamespace(t *testing.T) {
	authoritative := &reviewedResolverTestStore{
		Store: NewNoopStore(),
		identity: ResolvedEntityIdentity{
			StableEntityID: "entity-alex", IdentityNamespace: "session_npc",
		},
	}
	dual := NewDualWriteStore(authoritative, NewNoopStore())
	resolver, ok := dual.(UniqueActiveEntitySurfaceIdentityResolver)
	if !ok {
		t.Fatal("dual-write store does not expose namespace-aware identity resolver")
	}
	identity, err := resolver.ResolveUniqueActiveEntityIdentityBySurface(context.Background(), "session-1", "alex")
	if err != nil || identity != authoritative.identity || authoritative.identityCalls != 1 {
		t.Fatalf("dual identity=%#v calls=%d err=%v", identity, authoritative.identityCalls, err)
	}
	shadowDual := NewDualWriteStore(NewNoopStore(), authoritative)
	shadowResolver := shadowDual.(UniqueActiveEntitySurfaceIdentityResolver)
	identity, err = shadowResolver.ResolveUniqueActiveEntityIdentityBySurface(context.Background(), "session-1", "alex")
	if err != nil || identity != authoritative.identity || authoritative.identityCalls != 2 {
		t.Fatalf("dual shadow identity=%#v calls=%d err=%v", identity, authoritative.identityCalls, err)
	}

	readOnly := NewReadOnlyStore(authoritative)
	resolver, ok = readOnly.(UniqueActiveEntitySurfaceIdentityResolver)
	if !ok {
		t.Fatal("read-only store does not expose namespace-aware identity resolver")
	}
	identity, err = resolver.ResolveUniqueActiveEntityIdentityBySurface(context.Background(), "session-1", "alex")
	if err != nil || identity != authoritative.identity || authoritative.identityCalls != 3 {
		t.Fatalf("read-only identity=%#v calls=%d err=%v", identity, authoritative.identityCalls, err)
	}
}
