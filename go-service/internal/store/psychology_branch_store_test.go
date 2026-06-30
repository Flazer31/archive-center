package store

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBStoreSavePsychologyBranchPersistsSupportOnlyBranch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 23, 2, 30, 0, 0, time.UTC)
	branch := PsychologyBranch{
		ChatSessionID:          "sess-psy",
		CharacterName:          "Mina",
		BranchType:             "fear",
		AxisName:               "fear",
		Summary:                "Mina fears Rowan will abandon the plan.",
		Status:                 "active",
		Confidence:             0.8,
		ConfidenceLabel:        "high",
		SourceKind:             "critic_extraction",
		SourceTurnStart:        3,
		SourceTurnEnd:          4,
		SourceHash:             "hash-34",
		EvidenceJSON:           `{"turns":[3,4]}`,
		QuietTurns:             2,
		LastSeenTurn:           4,
		DormantAfterQuietTurns: 15,
		CreatedAt:              created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO psychology_branches")).
		WithArgs(
			"sess-psy", "Mina", "fear", "fear", "Mina fears Rowan will abandon the plan.",
			"active", 0.8, "high", "critic_extraction", 3, 4, "hash-34",
			`{"turns":[3,4]}`, 2, 4, 15, created,
		).
		WillReturnResult(sqlmock.NewResult(55, 1))

	saved, err := m.SavePsychologyBranch(context.Background(), branch)
	if err != nil {
		t.Fatalf("SavePsychologyBranch: %v", err)
	}
	if saved.ID != 55 || saved.CreatedAt != created || saved.UpdatedAt != created {
		t.Fatalf("unexpected saved branch: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListPsychologyBranchesScansDormancyFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 23, 2, 45, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "character_name", "branch_type", "axis_name", "summary", "status",
		"confidence", "confidence_label", "source_kind", "source_turn_start", "source_turn_end",
		"source_hash", "evidence_json", "quiet_turns", "last_seen_turn", "dormant_after_quiet_turns",
		"created_at", "updated_at",
	}).AddRow(
		int64(55), "sess-psy", "Mina", "fear", "fear", "summary", "dormant",
		0.8, "high", "critic_extraction", 3, 4, "hash-34", `{"turns":[3,4]}`,
		16, int64(4), 15, created, created,
	)
	mock.ExpectQuery("FROM psychology_branches").
		WithArgs("sess-psy", 25).
		WillReturnRows(rows)

	branches, err := m.ListPsychologyBranches(context.Background(), "sess-psy", 25)
	if err != nil {
		t.Fatalf("ListPsychologyBranches: %v", err)
	}
	if len(branches) != 1 {
		t.Fatalf("len(branches)=%d", len(branches))
	}
	got := branches[0]
	if got.ID != 55 || got.CharacterName != "Mina" || got.Status != "dormant" || got.LastSeenTurn != 4 {
		t.Fatalf("unexpected branch: %+v", got)
	}
	if got.ConfidenceLabel != "high" || got.SourceKind != "critic_extraction" || got.SourceHash != "hash-34" {
		t.Fatalf("missing scanned fields: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreUpdatePsychologyBranchStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectExec("UPDATE psychology_branches").
		WithArgs("dormant", 16, int64(55)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := m.UpdatePsychologyBranchStatus(context.Background(), 55, "dormant", 16); err != nil {
		t.Fatalf("UpdatePsychologyBranchStatus: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
