package main

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

const fixedAppProbeTable = "ac_permission_probe_fixed"

func TestValidateLocalAppAccountProbeDSNAcceptsManagedHostVariants(t *testing.T) {
	for index, dsn := range []string{
		"archive_center:secret@tcp(127.0.0.1:3307)/archive_center?parseTime=true",
		"archive_center:secret@tcp(localhost:3307)/archive_center?parseTime=true",
	} {
		if err := validateLocalAppAccountProbeDSN(dsn); err != nil {
			t.Fatalf("validate managed host variant %d: %v", index+1, err)
		}
	}
}

func TestLocalAppAccountProbeTargetsAlwaysDerivesBothGrantHosts(t *testing.T) {
	targets, err := localAppAccountProbeTargets("archive_center:secret@tcp(localhost:3307)/archive_center?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0].host != "127.0.0.1" || targets[1].host != "localhost" {
		t.Fatalf("target hosts=%v, want 127.0.0.1 then localhost", []string{targets[0].host, targets[1].host})
	}
	for _, target := range targets {
		if strings.Contains(target.host, "secret") {
			t.Fatalf("target host leaked credential material: %q", target.host)
		}
	}
}

func TestValidateLocalAppAccountProbeDSNRejectsUnsafeTargets(t *testing.T) {
	for index, dsn := range []string{
		"root:secret@tcp(127.0.0.1:3307)/archive_center",
		"archive_center:secret@tcp(192.0.2.4:3307)/archive_center",
		"archive_center:secret@tcp(localhost:3307)/",
		"archive_center:secret@unix(/tmp/mysql.sock)/archive_center",
	} {
		if err := validateLocalAppAccountProbeDSN(dsn); err == nil {
			t.Fatalf("expected unsafe DSN rejection for case %d", index+1)
		}
	}
}

func TestRunAppAccountPermissionProbeExercisesCRUDAndDDL(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	expectSuccessfulAppAccountPermissionProbe(mock)
	report, err := runAppAccountPermissionProbe(context.Background(), db, fixedAppProbeTable)
	if err != nil {
		t.Fatalf("runAppAccountPermissionProbe: %v report=%+v", err, report)
	}
	if report.Status != "ok" || report.CleanupStatus != "ok" {
		t.Fatalf("unexpected report: %+v", report)
	}
	wantStages := []string{"connect", "ddl_create", "dml_insert", "dml_select", "dml_update", "dml_update_readback", "dml_delete", "ddl_alter", "ddl_drop"}
	if len(report.Stages) != len(wantStages) {
		t.Fatalf("stages=%+v", report.Stages)
	}
	for i, want := range wantStages {
		if report.Stages[i].Stage != want || report.Stages[i].Status != "ok" {
			t.Fatalf("stage %d=%+v want=%s", i, report.Stages[i], want)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunAppAccountPermissionProbeInsertFailureCleansWithoutLeakingCause(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	table := "`" + fixedAppProbeTable + "`"
	mock.ExpectPing()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE " + table + " (`id` BIGINT NOT NULL PRIMARY KEY, `probe_value` VARCHAR(64) NOT NULL) ENGINE=InnoDB")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO "+table+" (`id`, `probe_value`) VALUES (?, ?)")).
		WithArgs(1, "inserted").
		WillReturnError(errors.New("access denied for archive-center-local-pass in archive_center:secret@tcp(127.0.0.1:3307)"))
	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE IF EXISTS " + table)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	report, err := runAppAccountPermissionProbe(context.Background(), db, fixedAppProbeTable)
	if err == nil {
		t.Fatal("expected insert failure")
	}
	var typed *appAccountPermissionProbeError
	if !errors.As(err, &typed) || typed.Stage != "dml_insert" || typed.Operation != "INSERT" {
		t.Fatalf("typed error=%#v", err)
	}
	if report.Status != "failed" || report.CleanupStatus != "ok" {
		t.Fatalf("unexpected report: %+v", report)
	}
	encoded, marshalErr := json.Marshal(report)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, forbidden := range []string{"archive-center-local-pass", "archive_center:secret", "@tcp("} {
		if strings.Contains(string(encoded), forbidden) || strings.Contains(err.Error(), forbidden) {
			t.Fatalf("probe leaked %q: report=%s err=%v", forbidden, encoded, err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunAppAccountPermissionProbeAlterFailureReportsTypedStageAndCleans(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	table := "`" + fixedAppProbeTable + "`"
	expectAppAccountProbeThroughDelete(mock)
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE " + table + " ADD COLUMN `probe_marker` INT NOT NULL DEFAULT 0")).
		WillReturnError(&mysql.MySQLError{Number: 1142, Message: "ALTER command denied"})
	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE IF EXISTS " + table)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	report, err := runAppAccountPermissionProbe(context.Background(), db, fixedAppProbeTable)
	if err == nil {
		t.Fatal("expected ALTER failure")
	}
	var typed *appAccountPermissionProbeError
	if !errors.As(err, &typed) || typed.Stage != "ddl_alter" || typed.Operation != "ALTER" {
		t.Fatalf("typed error=%#v", err)
	}
	if report.CleanupStatus != "ok" {
		t.Fatalf("cleanup report=%+v", report)
	}
	if got := appAccountProbeErrorClass(err); got != "DB_PERMISSION_DENIED" {
		t.Fatalf("error class=%q, want DB_PERMISSION_DENIED", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecordAppAccountProbeFailurePromotesPermissionClassWithoutCauseLeak(t *testing.T) {
	report := newReport("schema.sql", true)
	cause := &mysql.MySQLError{
		Number:  1142,
		Message: "ALTER denied for archive_center:top-secret@tcp(127.0.0.1:3307)",
	}
	recordAppAccountProbeFailure(report, &appAccountPermissionProbeError{
		Stage:     "ddl_alter",
		Operation: "ALTER",
		Err:       cause,
	})
	if report.ErrorClass != "DB_PERMISSION_DENIED" || report.ErrorStage != "ddl_alter" || report.ErrorOperation != "ALTER" {
		t.Fatalf("top-level failure classification=%+v", report)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"top-secret", "@tcp(", "ALTER denied for"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("top-level report leaked %q: %s", forbidden, encoded)
		}
	}
}

func expectSuccessfulAppAccountPermissionProbe(mock sqlmock.Sqlmock) {
	table := "`" + fixedAppProbeTable + "`"
	expectAppAccountProbeThroughDelete(mock)
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE " + table + " ADD COLUMN `probe_marker` INT NOT NULL DEFAULT 0")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE " + table)).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func expectAppAccountProbeThroughDelete(mock sqlmock.Sqlmock) {
	table := "`" + fixedAppProbeTable + "`"
	mock.ExpectPing()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE " + table + " (`id` BIGINT NOT NULL PRIMARY KEY, `probe_value` VARCHAR(64) NOT NULL) ENGINE=InnoDB")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO "+table+" (`id`, `probe_value`) VALUES (?, ?)")).
		WithArgs(1, "inserted").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `probe_value` FROM " + table + " WHERE `id` = ?")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"probe_value"}).AddRow("inserted"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE "+table+" SET `probe_value` = ? WHERE `id` = ?")).
		WithArgs("updated", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `probe_value` FROM " + table + " WHERE `id` = ?")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"probe_value"}).AddRow("updated"))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM " + table + " WHERE `id` = ?")).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))
}
