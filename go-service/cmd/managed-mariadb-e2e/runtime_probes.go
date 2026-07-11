package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func runDefaultCandidateProbe(ctx context.Context, port int, actual bool) (map[string]any, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	probes, err := probeReadEndpoints(ctx, baseURL, []string{"/health", "/ready", "/version", "/stats"})
	selectedRuntime := "go_rehearsal"
	switchScope := "managed_disposable_rehearsal"
	if actual {
		selectedRuntime = "go"
		switchScope = "managed_disposable_actual"
	}
	report := map[string]any{
		"requested":              true,
		"role":                   "go_default_candidate",
		"base_url":               baseURL,
		"selected_runtime":       selectedRuntime,
		"switch_scope":           switchScope,
		"candidate_store_mode":   "mariadb_read_shadow",
		"candidate_product_read": true,
		"authority_switch":       false,
		"go_default_switch":      actual,
		"persistent_switch":      false,
		"python_runtime_retired": false,
		"probes":                 probes,
	}
	readyChecks := map[string]any{}
	if ready, ok := probes["/ready"]; ok {
		if body, ok := ready["json"].(map[string]any); ok {
			if checks, ok := body["checks"].(map[string]any); ok {
				readyChecks = checks
			}
		}
	}
	report["ready_checks"] = readyChecks
	ok := err == nil &&
		allProbeStatusOK(probes) &&
		readyChecks["store_mode"] == "mariadb_read_shadow" &&
		readyChecks["mariadb_product_read"] == "enabled" &&
		readyChecks["mariadb_authority"] == "disabled"
	status := "ok"
	if !ok {
		status = "failed"
	}
	report["status"] = status
	if err != nil {
		report["error"] = err.Error()
		return report, err
	}
	if !ok {
		return report, fmt.Errorf("go default candidate probe failed")
	}
	return report, nil
}

func runAuthorityCutoverReplay(ctx context.Context, exec commandExecutor, cfg directProviderConfig, goBinPath string, dsn string, pythonBaseURL string, startPort int) (map[string]any, []executedStep, error) {
	base := map[string]any{
		"requested":              true,
		"store_mode":             "mariadb_authority",
		"authority_switch":       true,
		"persistent_switch":      false,
		"go_default_switch":      false,
		"rollback_required":      true,
		"python_runtime_retired": false,
	}
	var steps []executedStep
	port, err := findAvailablePort("127.0.0.1", startPort, startPort+30)
	if err != nil {
		base["status"] = "failed"
		base["error"] = err.Error()
		return base, []executedStep{{Name: "authority-start-go-backend", Status: "failed", Error: err.Error()}}, err
	}

	cmd, startStep, err := startGoBackend(ctx, exec, goBackendStartConfig{
		BinPath:   goBinPath,
		Port:      port,
		DSN:       dsn,
		StoreMode: "mariadb_authority",
	})
	startStep.Name = "authority-start-go-backend"
	steps = append(steps, startStep)
	if err != nil {
		base["status"] = "failed"
		base["error"] = err.Error()
		return base, steps, err
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = exec.Kill(cmd)
		}
	}()

	waitStep := waitGoReady(ctx, port)
	waitStep.Name = "authority-wait-go-ready"
	steps = append(steps, waitStep)
	if waitStep.Status != "ok" {
		base["status"] = "failed"
		base["error"] = waitStep.Error
		return base, steps, fmt.Errorf("authority backend not ready: %s", waitStep.Error)
	}

	checks, err := fetchReadyChecks(ctx, port)
	readyStatus := "ok"
	readyErr := ""
	if err != nil {
		readyStatus = "failed"
		readyErr = err.Error()
	} else if checks["store_mode"] != "mariadb_authority" || checks["mariadb_authority"] != "enabled" || checks["mariadb_product_read"] != "enabled" {
		readyStatus = "failed"
		readyErr = fmt.Sprintf("unexpected authority checks: store_mode=%s mariadb_authority=%s mariadb_product_read=%s", checks["store_mode"], checks["mariadb_authority"], checks["mariadb_product_read"])
	}
	steps = append(steps, executedStep{
		Name:   "authority-ready-check",
		Status: readyStatus,
		Error:  readyErr,
		Note:   fmt.Sprintf("store_mode=%s mariadb_product_read=%s mariadb_authority=%s", checks["store_mode"], checks["mariadb_product_read"], checks["mariadb_authority"]),
	})
	base["ready_checks"] = checks
	if readyStatus != "ok" {
		base["status"] = "failed"
		base["error"] = readyErr
		return base, steps, fmt.Errorf("authority ready check failed: %s", readyErr)
	}

	start := time.Now()
	smokeDetails, err := runRouteWriteSmoke(ctx, port, dsn, cfg.SessionID+"-authority", "mariadb_authority")
	smokeStatus := "ok"
	smokeErr := ""
	if err != nil {
		smokeStatus = "failed"
		smokeErr = err.Error()
	}
	steps = append(steps, executedStep{
		Name:       "authority-route-write-smoke",
		Status:     smokeStatus,
		DurationMs: time.Since(start).Milliseconds(),
		Error:      smokeErr,
		Details:    smokeDetails,
	})
	base["route_write_smoke"] = smokeDetails
	if err != nil {
		base["status"] = "failed"
		base["error"] = err.Error()
		return base, steps, fmt.Errorf("authority route write smoke: %w", err)
	}

	start = time.Now()
	reportArgs := []string{
		"-go-base", fmt.Sprintf("http://127.0.0.1:%d", port),
		"-session-id", cfg.SessionID,
		"-out", filepath.Join(cfg.DataDir, "authority-shadow-value-report.md"),
		"-json-out", filepath.Join(cfg.DataDir, "authority-shadow-value-report.json"),
	}
	if strings.TrimSpace(pythonBaseURL) != "" {
		reportArgs = append(reportArgs, "-python-base", pythonBaseURL)
	}
	reportCmd, reportRunArgs, reportDisplay := managedCommand(exec, "shadow-value-report", reportArgs...)
	replay := map[string]any{
		"requested": true,
		"command":   reportDisplay,
		"json_out":  filepath.Join(cfg.DataDir, "authority-shadow-value-report.json"),
	}
	if out, err := exec.Run(ctx, reportCmd, reportRunArgs...); err != nil {
		replay["status"] = "failed"
		replay["error"] = fmt.Sprintf("%v: %s", err, string(out))
		steps = append(steps, executedStep{
			Name:       "authority-post-cutover-replay",
			Status:     "failed",
			Command:    reportDisplay,
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      replay["error"].(string),
		})
		base["post_cutover_replay"] = replay
		base["status"] = "failed"
		base["error"] = replay["error"]
		return base, steps, fmt.Errorf("authority post-cutover replay: %w", err)
	}
	replay["status"] = "ok"
	steps = append(steps, executedStep{
		Name:       "authority-post-cutover-replay",
		Status:     "ok",
		Command:    reportDisplay,
		DurationMs: time.Since(start).Milliseconds(),
		Details:    replay,
	})
	base["post_cutover_replay"] = replay

	stopStep := stopGoBackend(exec, cmd)
	stopStep.Name = "authority-stop-go-backend"
	steps = append(steps, stopStep)
	stopped = true
	if stopStep.Status != "ok" {
		base["status"] = "failed"
		base["error"] = stopStep.Error
		return base, steps, fmt.Errorf("authority backend stop failed: %s", stopStep.Error)
	}

	base["base_url"] = fmt.Sprintf("http://127.0.0.1:%d", port)
	base["status"] = "ok"
	base["post_cutover_replay_ok"] = true
	return base, steps, nil
}

func runPythonFallbackProbe(ctx context.Context, baseURL string) (map[string]any, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	report := map[string]any{
		"requested":              true,
		"role":                   "python_fallback",
		"base_url":               baseURL,
		"selected_runtime":       "python_fallback",
		"fallback_available":     false,
		"authority_switch":       false,
		"go_default_switch":      false,
		"python_runtime_retired": false,
	}
	if baseURL == "" {
		report["status"] = "failed"
		report["error"] = "python fallback base URL is empty"
		return report, fmt.Errorf("python fallback base URL is empty")
	}
	probes, _ := probeReadEndpoints(ctx, baseURL, []string{"/health", "/ready", "/version", "/stats"})
	report["probes"] = probes
	ok := probeStatusOK(probes["/health"]) && probeStatusOK(probes["/stats"])
	report["fallback_available"] = ok
	report["required_probes"] = []string{"/health", "/stats"}
	report["optional_probes"] = []string{"/ready", "/version"}
	status := "ok"
	if !ok {
		status = "failed"
	}
	report["status"] = status
	if !ok {
		return report, fmt.Errorf("python fallback probe failed")
	}
	return report, nil
}

func probeReadEndpoints(ctx context.Context, baseURL string, paths []string) (map[string]map[string]any, error) {
	out := map[string]map[string]any{}
	var firstErr error
	for _, path := range paths {
		result, err := probeGET(ctx, baseURL+path)
		out[path] = result
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return out, firstErr
}

func probeGET(ctx context.Context, url string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"url": url, "status": "failed", "error": err.Error()}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	status := "ok"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = "failed"
	}
	result := map[string]any{
		"url":         url,
		"method":      http.MethodGet,
		"http_status": resp.StatusCode,
		"status":      status,
		"json":        decoded,
	}
	if status != "ok" {
		return result, fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}
	return result, nil
}

func allProbeStatusOK(probes map[string]map[string]any) bool {
	for _, probe := range probes {
		if !probeStatusOK(probe) {
			return false
		}
	}
	return true
}

func probeStatusOK(probe map[string]any) bool {
	if probe == nil {
		return false
	}
	status, _ := probe["status"].(string)
	switch httpStatus := probe["http_status"].(type) {
	case int:
		return status == "ok" && httpStatus >= 200 && httpStatus < 300
	case float64:
		return status == "ok" && httpStatus >= 200 && httpStatus < 300
	default:
		return false
	}
}

func runBackupRestoreDrill(ctx context.Context, port int, dataDir string) (map[string]any, error) {
	const sourceDB = "archive_center_temp"
	const restoreDB = "archive_center_restore_temp"
	rootDSN := fmt.Sprintf("root@tcp(127.0.0.1:%d)/?timeout=10s&parseTime=true", port)
	db, err := sql.Open("mysql", rootDSN)
	if err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}

	tables, err := listBaseTables(ctx, db, sourceDB)
	if err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}
	if len(tables) == 0 {
		err := fmt.Errorf("source database %s has no base tables", sourceDB)
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}

	if _, err := db.ExecContext(ctx, "DROP DATABASE IF EXISTS "+escapeIdentifier(restoreDB)); err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}
	if _, err := db.ExecContext(ctx, "CREATE DATABASE "+escapeIdentifier(restoreDB)+" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, nil, err.Error(), ""), err
	}

	tableReports := make([]map[string]any, 0, len(tables))
	for _, table := range tables {
		source := escapeIdentifier(sourceDB) + "." + escapeIdentifier(table)
		restore := escapeIdentifier(restoreDB) + "." + escapeIdentifier(table)
		if _, err := db.ExecContext(ctx, "CREATE TABLE "+restore+" LIKE "+source); err != nil {
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO "+restore+" SELECT * FROM "+source); err != nil {
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
		sourceCount, err := tableCount(ctx, db, sourceDB, table)
		if err != nil {
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
		restoreCount, err := tableCount(ctx, db, restoreDB, table)
		if err != nil {
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
		tableReports = append(tableReports, map[string]any{
			"table":         table,
			"source_rows":   sourceCount,
			"restored_rows": restoreCount,
			"match":         sourceCount == restoreCount,
		})
		if sourceCount != restoreCount {
			err := fmt.Errorf("restore count mismatch for %s: source=%d restored=%d", table, sourceCount, restoreCount)
			return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
		}
	}

	manifestPath := filepath.Join(dataDir, "backup-restore-drill.json")
	report := backupRestoreReport("ok", sourceDB, restoreDB, tableReports, "", manifestPath)
	manifest, _ := json.MarshalIndent(report, "", "  ")
	manifest = append(manifest, '\n')
	if err := os.WriteFile(manifestPath, manifest, 0644); err != nil {
		return backupRestoreReport("failed", sourceDB, restoreDB, tableReports, err.Error(), ""), err
	}
	return report, nil
}

func listBaseTables(ctx context.Context, db *sql.DB, database string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW FULL TABLES FROM "+escapeIdentifier(database)+" WHERE Table_type = 'BASE TABLE'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		var tableType string
		if err := rows.Scan(&table, &tableType); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(tables)
	return tables, nil
}

func tableCount(ctx context.Context, db *sql.DB, database string, table string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM " + escapeIdentifier(database) + "." + escapeIdentifier(table)
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func backupRestoreReport(status string, sourceDB string, restoreDB string, tables []map[string]any, errText string, manifestPath string) map[string]any {
	totalSource := 0
	totalRestored := 0
	allMatch := status == "ok"
	for _, table := range tables {
		if v, ok := table["source_rows"].(int); ok {
			totalSource += v
		}
		if v, ok := table["restored_rows"].(int); ok {
			totalRestored += v
		}
		if v, ok := table["match"].(bool); ok && !v {
			allMatch = false
		}
	}
	out := map[string]any{
		"requested":           true,
		"status":              status,
		"method":              "managed_sql_clone_restore",
		"source_database":     sourceDB,
		"restored_database":   restoreDB,
		"tables_checked":      len(tables),
		"source_rows_total":   totalSource,
		"restored_rows_total": totalRestored,
		"row_count_match":     allMatch && totalSource == totalRestored,
		"tables":              tables,
		"authority_switch":    false,
		"go_default_switch":   false,
	}
	if manifestPath != "" {
		out["manifest_path"] = manifestPath
	}
	if errText != "" {
		out["error"] = errText
	}
	return out
}

func stopGoBackend(exec commandExecutor, cmd *exec.Cmd) executedStep {
	if cmd == nil {
		return executedStep{Name: "stop-go-backend", Status: "ok", Note: "no process to stop"}
	}
	if err := exec.Kill(cmd); err != nil {
		return executedStep{Name: "stop-go-backend", Status: "failed", Error: err.Error()}
	}
	return executedStep{Name: "stop-go-backend", Status: "ok"}
}
