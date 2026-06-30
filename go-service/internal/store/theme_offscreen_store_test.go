package store

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBStoreSaveThemeOffscreenCarryPersistsSupportSurface(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 23, 4, 30, 0, 0, time.UTC)
	record := ThemeOffscreenCarryRecord{
		ChatSessionID:          "sess-theme",
		SurfaceType:            "theme_trace",
		Label:                  "winter promises",
		Summary:                "Recurring image of promises made during snowstorms.",
		Status:                 "active",
		Confidence:             0.8,
		ConfidenceLabel:        "high",
		SourceKind:             "critic_extraction",
		SourceTurnStart:        3,
		SourceTurnEnd:          5,
		SourceHash:             "hash-theme",
		EvidenceJSON:           `{"turns":[3,5]}`,
		QuietTurns:             2,
		LastSeenTurn:           5,
		DormantAfterQuietTurns: 15,
		ForegroundEligible:     true,
		ForegroundReasonJSON:   `{"reason":"current scene repeats motif"}`,
		CreatedAt:              created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO theme_offscreen_carries")).
		WithArgs(
			"sess-theme", "theme_trace", "winter promises",
			"Recurring image of promises made during snowstorms.",
			"active", 0.8, "high", "critic_extraction", 3, 5,
			"hash-theme", `{"turns":[3,5]}`, 2, 5, 15, true,
			`{"reason":"current scene repeats motif"}`, created,
		).
		WillReturnResult(sqlmock.NewResult(66, 1))

	saved, err := m.SaveThemeOffscreenCarry(context.Background(), record)
	if err != nil {
		t.Fatalf("SaveThemeOffscreenCarry: %v", err)
	}
	if saved.ID != 66 || saved.CreatedAt != created || saved.UpdatedAt != created {
		t.Fatalf("unexpected saved record: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListThemeOffscreenCarriesScansEligibilityFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 23, 4, 45, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "surface_type", "label", "summary", "status", "confidence",
		"confidence_label", "source_kind", "source_turn_start", "source_turn_end",
		"source_hash", "evidence_json", "quiet_turns", "last_seen_turn",
		"dormant_after_quiet_turns", "foreground_eligible", "foreground_reason_json",
		"created_at", "updated_at",
	}).AddRow(
		int64(66), "sess-theme", "offscreen_thread", "north gate unrest", "summary", "dormant",
		0.6, "medium", "critic_extraction", 7, 8, "hash-offscreen", `{"turns":[7,8]}`,
		16, int64(8), 15, true, `{"reason":"callback opportunity"}`, created, created,
	)
	mock.ExpectQuery("FROM theme_offscreen_carries").
		WithArgs("sess-theme", "offscreen_thread", "offscreen_thread", 25).
		WillReturnRows(rows)

	records, err := m.ListThemeOffscreenCarries(context.Background(), "sess-theme", "offscreen_thread", 25)
	if err != nil {
		t.Fatalf("ListThemeOffscreenCarries: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records)=%d", len(records))
	}
	got := records[0]
	if got.ID != 66 || got.SurfaceType != "offscreen_thread" || got.Status != "dormant" || got.LastSeenTurn != 8 {
		t.Fatalf("unexpected record: %+v", got)
	}
	if !got.ForegroundEligible || got.ForegroundReasonJSON == "" || got.SourceHash != "hash-offscreen" {
		t.Fatalf("missing eligibility/evidence fields: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreUpdateThemeOffscreenCarryStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectExec("UPDATE theme_offscreen_carries").
		WithArgs("dormant", 16, int64(66)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := m.UpdateThemeOffscreenCarryStatus(context.Background(), 66, "dormant", 16); err != nil {
		t.Fatalf("UpdateThemeOffscreenCarryStatus: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
