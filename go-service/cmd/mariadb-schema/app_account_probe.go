package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

const appAccountPermissionProbeContract = "archive-center.mariadb-app-account-permission-probe.v1"

type appAccountPermissionProbeReport struct {
	Contract      string                           `json:"contract"`
	Status        string                           `json:"status"`
	GeneratedAt   string                           `json:"generated_at"`
	TargetClass   string                           `json:"target_class"`
	TargetHost    string                           `json:"target_host"`
	Table         string                           `json:"table"`
	Stages        []appAccountPermissionProbeStage `json:"stages"`
	CleanupStatus string                           `json:"cleanup_status"`
}

type appAccountPermissionProbeStage struct {
	Stage        string `json:"stage"`
	Operation    string `json:"operation"`
	Status       string `json:"status"`
	RowsAffected int64  `json:"rows_affected,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
}

type appAccountPermissionProbeError struct {
	Stage     string
	Operation string
	Err       error
}

func (e *appAccountPermissionProbeError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("MariaDB application-account permission probe failed at %s (%s)", e.Stage, e.Operation)
}

func (e *appAccountPermissionProbeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type appAccountProbeDB interface {
	sqlExecer
	PingContext(ctx context.Context) error
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type localAppAccountProbeTarget struct {
	host string
	dsn  string
}

func localAppAccountProbeTargets(dsn string) ([]localAppAccountProbeTarget, error) {
	cfg, err := mysql.ParseDSN(strings.TrimSpace(dsn))
	if err != nil {
		return nil, errors.New("application-account probe DSN is invalid")
	}
	if strings.TrimSpace(cfg.User) == "" || strings.EqualFold(strings.TrimSpace(cfg.User), "root") {
		return nil, errors.New("application-account probe requires a non-root database account")
	}
	if strings.TrimSpace(cfg.DBName) == "" {
		return nil, errors.New("application-account probe requires an existing target database")
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Net), "tcp") {
		return nil, errors.New("application-account probe requires a local TCP connection")
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(cfg.Addr))
	if err != nil {
		return nil, errors.New("application-account probe requires a host and port")
	}
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "127.0.0.1", "localhost":
	default:
		return nil, errors.New("application-account probe is restricted to 127.0.0.1 or localhost")
	}

	targets := make([]localAppAccountProbeTarget, 0, 2)
	for _, targetHost := range []string{"127.0.0.1", "localhost"} {
		targetConfig := *cfg
		targetConfig.Addr = net.JoinHostPort(targetHost, port)
		targets = append(targets, localAppAccountProbeTarget{
			host: targetHost,
			dsn:  targetConfig.FormatDSN(),
		})
	}
	return targets, nil
}

func validateLocalAppAccountProbeDSN(dsn string) error {
	_, err := localAppAccountProbeTargets(dsn)
	return err
}

func runAppAccountPermissionProbe(ctx context.Context, db appAccountProbeDB, tableName string) (*appAccountPermissionProbeReport, error) {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		tableName = newAppAccountProbeTableName()
	}
	if !validAppAccountProbeTableName(tableName) {
		report := newAppAccountPermissionProbeReport(tableName)
		report.Status = "failed"
		report.CleanupStatus = "not_created"
		err := errors.New("invalid temporary probe table name")
		report.Stages = append(report.Stages, failedAppAccountProbeStage("configure", "VALIDATE", err))
		return report, &appAccountPermissionProbeError{Stage: "configure", Operation: "VALIDATE", Err: err}
	}

	report := newAppAccountPermissionProbeReport(tableName)
	quotedTable := "`" + tableName + "`"
	created := false
	cleanupAttempted := false
	cleanup := func() error {
		if cleanupAttempted || !created {
			return nil
		}
		cleanupAttempted = true
		if _, err := db.ExecContext(context.WithoutCancel(ctx), "DROP TABLE IF EXISTS "+quotedTable); err != nil {
			report.CleanupStatus = "failed"
			report.Stages = append(report.Stages, failedAppAccountProbeStage("cleanup_drop", "DROP", err))
			return err
		}
		created = false
		report.CleanupStatus = "ok"
		report.Stages = append(report.Stages, appAccountPermissionProbeStage{
			Stage: "cleanup_drop", Operation: "DROP", Status: "ok",
		})
		return nil
	}
	fail := func(stage, operation string, err error) (*appAccountPermissionProbeReport, error) {
		report.Status = "failed"
		report.Stages = append(report.Stages, failedAppAccountProbeStage(stage, operation, err))
		_ = cleanup()
		return report, &appAccountPermissionProbeError{Stage: stage, Operation: operation, Err: err}
	}

	if err := db.PingContext(ctx); err != nil {
		report.CleanupStatus = "not_created"
		return fail("connect", "PING", err)
	}
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "connect", Operation: "PING", Status: "ok",
	})

	createStatement := "CREATE TABLE " + quotedTable + " (`id` BIGINT NOT NULL PRIMARY KEY, `probe_value` VARCHAR(64) NOT NULL) ENGINE=InnoDB"
	if _, err := db.ExecContext(ctx, createStatement); err != nil {
		report.CleanupStatus = "not_created"
		return fail("ddl_create", "CREATE", err)
	}
	created = true
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "ddl_create", Operation: "CREATE", Status: "ok",
	})

	result, err := db.ExecContext(ctx, "INSERT INTO "+quotedTable+" (`id`, `probe_value`) VALUES (?, ?)", 1, "inserted")
	if err != nil {
		return fail("dml_insert", "INSERT", err)
	}
	rows, err := exactRowsAffected(result, 1)
	if err != nil {
		return fail("dml_insert", "INSERT", err)
	}
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "dml_insert", Operation: "INSERT", Status: "ok", RowsAffected: rows,
	})

	var selected string
	if err := db.QueryRowContext(ctx, "SELECT `probe_value` FROM "+quotedTable+" WHERE `id` = ?", 1).Scan(&selected); err != nil {
		return fail("dml_select", "SELECT", err)
	}
	if selected != "inserted" {
		return fail("dml_select", "SELECT", errors.New("insert readback mismatch"))
	}
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "dml_select", Operation: "SELECT", Status: "ok",
	})

	result, err = db.ExecContext(ctx, "UPDATE "+quotedTable+" SET `probe_value` = ? WHERE `id` = ?", "updated", 1)
	if err != nil {
		return fail("dml_update", "UPDATE", err)
	}
	rows, err = exactRowsAffected(result, 1)
	if err != nil {
		return fail("dml_update", "UPDATE", err)
	}
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "dml_update", Operation: "UPDATE", Status: "ok", RowsAffected: rows,
	})

	if err := db.QueryRowContext(ctx, "SELECT `probe_value` FROM "+quotedTable+" WHERE `id` = ?", 1).Scan(&selected); err != nil {
		return fail("dml_update_readback", "SELECT", err)
	}
	if selected != "updated" {
		return fail("dml_update_readback", "SELECT", errors.New("update readback mismatch"))
	}
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "dml_update_readback", Operation: "SELECT", Status: "ok",
	})

	result, err = db.ExecContext(ctx, "DELETE FROM "+quotedTable+" WHERE `id` = ?", 1)
	if err != nil {
		return fail("dml_delete", "DELETE", err)
	}
	rows, err = exactRowsAffected(result, 1)
	if err != nil {
		return fail("dml_delete", "DELETE", err)
	}
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "dml_delete", Operation: "DELETE", Status: "ok", RowsAffected: rows,
	})

	if _, err := db.ExecContext(ctx, "ALTER TABLE "+quotedTable+" ADD COLUMN `probe_marker` INT NOT NULL DEFAULT 0"); err != nil {
		return fail("ddl_alter", "ALTER", err)
	}
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "ddl_alter", Operation: "ALTER", Status: "ok",
	})

	if _, err := db.ExecContext(ctx, "DROP TABLE "+quotedTable); err != nil {
		return fail("ddl_drop", "DROP", err)
	}
	created = false
	cleanupAttempted = true
	report.Stages = append(report.Stages, appAccountPermissionProbeStage{
		Stage: "ddl_drop", Operation: "DROP", Status: "ok",
	})
	report.CleanupStatus = "ok"
	report.Status = "ok"
	return report, nil
}

func newAppAccountPermissionProbeReport(tableName string) *appAccountPermissionProbeReport {
	return &appAccountPermissionProbeReport{
		Contract:      appAccountPermissionProbeContract,
		Status:        "running",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		TargetClass:   "local_non_root_application_account",
		Table:         tableName,
		CleanupStatus: "pending",
	}
}

func failedAppAccountProbeStage(stage, operation string, err error) appAccountPermissionProbeStage {
	return appAccountPermissionProbeStage{
		Stage:     stage,
		Operation: operation,
		Status:    "failed",
		ErrorCode: appAccountProbeErrorCode(err),
	}
}

func appAccountProbeErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return fmt.Sprintf("mariadb_error_%d", mysqlErr.Number)
	}
	return "database_operation_failed"
}

func appAccountProbeErrorClass(err error) string {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1044, 1045, 1142, 1143, 1227, 1370:
			return "DB_PERMISSION_DENIED"
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "DB_TIMEOUT"
	}
	if errors.Is(err, context.Canceled) {
		return "DB_CONTEXT_CANCELED"
	}
	return "DB_OPERATION_FAILED"
}

func exactRowsAffected(result sql.Result, want int64) (int64, error) {
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if rows != want {
		return rows, fmt.Errorf("rows affected=%d, want %d", rows, want)
	}
	return rows, nil
}

func validAppAccountProbeTableName(value string) bool {
	if !strings.HasPrefix(value, "ac_permission_probe_") || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func newAppAccountProbeTableName() string {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err == nil {
		return fmt.Sprintf("ac_permission_probe_%d_%s", time.Now().UTC().UnixMilli(), hex.EncodeToString(random))
	}
	return fmt.Sprintf("ac_permission_probe_%d", time.Now().UTC().UnixNano())
}
