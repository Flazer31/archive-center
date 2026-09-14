package store

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAuditFailureLoggedEvenIfCallerIgnoresError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	var output bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(old)
	mock.ExpectExec("INSERT INTO audit_logs").WillReturnError(errors.New("fixture audit disk failure"))
	_ = m.SaveAuditLog(context.Background(), &AuditLog{EventType: "fixture"})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "fixture audit disk failure") {
		t.Fatal("ignored audit failure disappeared")
	}
}
