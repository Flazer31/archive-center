package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAuditUnknownTargetPreservesDiagnostic(t *testing.T) {
	for _, id := range []int64{-1, 0, 42, 1 << 40} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var want any = id
			if id < 0 {
				want = nil
			}
			mock.ExpectExec("INSERT INTO audit_logs").WithArgs(sqlmock.AnyArg(), "effective_input_saved", "s", "turn", want, nil, `{"turn_index":-1}`, nil).WillReturnResult(sqlmock.NewResult(1, 1))
			if err := (&mariadbStore{db: db}).SaveAuditLog(context.Background(), &AuditLog{
				ChatSessionID: "s", EventType: "effective_input_saved", TargetType: "turn", TargetID: id, DetailsJSON: `{"turn_index":-1}`,
			}); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

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
