package store

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBStoreSaveConsequenceRecordPersistsSupportOnlyChain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 23, 1, 30, 0, 0, time.UTC)
	record := ConsequenceRecord{
		ChatSessionID:          "sess-23",
		SourceTurnStart:        7,
		SourceTurnEnd:          8,
		Decision:               "Rowan opens the sealed door.",
		ImmediateResult:        "Mina sees the hidden room.",
		DelayedEffect:          "The guard may investigate later.",
		AffectedRelationsJSON:  `{"Mina->Rowan":"tense"}`,
		AffectedWorldJSON:      `{"door":"open"}`,
		Status:                 "pending",
		Importance:             0.7,
		Confidence:             0.8,
		ForegroundEligible:     true,
		QuietTurns:             2,
		LastSeenTurn:           8,
		ExpiresAfterQuietTurns: 12,
		SourceHash:             "hash-78",
		EvidenceJSON:           `{"turns":[7,8]}`,
		CreatedAt:              created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO consequence_records")).
		WithArgs(
			"sess-23", 7, 8,
			"Rowan opens the sealed door.",
			"Mina sees the hidden room.",
			"The guard may investigate later.",
			`{"Mina->Rowan":"tense"}`,
			`{"door":"open"}`,
			"pending", 0.7, 0.8, true, 2, 8, 0, 12,
			"hash-78", `{"turns":[7,8]}`, created,
		).
		WillReturnResult(sqlmock.NewResult(77, 1))

	saved, err := m.SaveConsequenceRecord(context.Background(), record)
	if err != nil {
		t.Fatalf("SaveConsequenceRecord: %v", err)
	}
	if saved.ID != 77 || saved.CreatedAt != created || saved.UpdatedAt != created {
		t.Fatalf("unexpected saved record: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListConsequenceRecordsScansLifecycleFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 23, 1, 45, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "source_turn_start", "source_turn_end", "decision", "immediate_result",
		"delayed_effect", "affected_relations", "affected_world", "status", "importance", "confidence",
		"foreground_eligible", "quiet_turns", "last_seen_turn", "paid_turn", "expires_after_quiet_turns",
		"source_hash", "evidence_json", "created_at", "updated_at",
	}).AddRow(
		int64(77), "sess-23", 7, 8, "decision", "result", "delayed",
		`{"rel":"shift"}`, `{"world":"changed"}`, "active", 0.7, 0.8,
		true, 2, int64(8), nil, 12, "hash-78", `{"turns":[7,8]}`, created, created,
	)
	mock.ExpectQuery("FROM consequence_records").
		WithArgs("sess-23", 25).
		WillReturnRows(rows)

	records, err := m.ListConsequenceRecords(context.Background(), "sess-23", 25)
	if err != nil {
		t.Fatalf("ListConsequenceRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records)=%d", len(records))
	}
	got := records[0]
	if got.ID != 77 || got.ChatSessionID != "sess-23" || got.LastSeenTurn != 8 || got.PaidTurn != 0 {
		t.Fatalf("unexpected record: %+v", got)
	}
	if got.AffectedRelationsJSON == "" || got.SourceHash != "hash-78" || !got.ForegroundEligible {
		t.Fatalf("missing scanned fields: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
