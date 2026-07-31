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

type reviewedResolverTestStore struct {
	Store
	target string
	calls  int
}

func (s *reviewedResolverTestStore) ResolveReviewedCanonicalEntityID(context.Context, string, string) (string, error) {
	s.calls++
	return s.target, nil
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
