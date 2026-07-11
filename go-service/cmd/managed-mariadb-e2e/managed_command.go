package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func managedCommand(exec commandExecutor, name string, args ...string) (string, []string, string) {
	if path, err := exec.LookPath(name); err == nil {
		return path, args, name + " " + strings.Join(args, " ")
	}
	if root := goServiceRoot(); root != "" {
		runArgs := append([]string{"run", "-buildvcs=false", "./cmd/" + name}, args...)
		return "go", runArgs, "go " + strings.Join(runArgs, " ")
	}
	return name, args, name + " " + strings.Join(args, " ")
}

func (r *osDirectProviderRunner) run(ctx context.Context, cfg directProviderConfig) (steps []executedStep, err error) {
	// Step: create temp data dir
	start := time.Now()
	if !cfg.KeepTemp && strings.TrimSpace(cfg.DataDir) != "" {
		_ = os.RemoveAll(cfg.DataDir)
	}
	if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
		steps = append(steps, executedStep{
			Name:       "create-datadir",
			Status:     "failed",
			Command:    fmt.Sprintf("mkdir %s", cfg.DataDir),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		return steps, fmt.Errorf("create datadir: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "create-datadir",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Step: check port availability
	start = time.Now()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if err != nil {
		steps = append(steps, executedStep{
			Name:       "port-check",
			Status:     "failed",
			Command:    fmt.Sprintf("net.Listen tcp 127.0.0.1:%d", cfg.Port),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		steps = append(steps, cleanupTempStep(cfg.DataDir, cfg.KeepTemp))
		return steps, fmt.Errorf("port check: %w", err)
	}
	_ = ln.Close()
	steps = append(steps, executedStep{
		Name:       "port-check",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Step: init data dir
	start = time.Now()
	initCmd, initArgs := buildInitCommand(cfg.ProviderPath, cfg.DataDir)
	if out, err := r.exec.Run(ctx, initCmd, initArgs...); err != nil {
		steps = append(steps, executedStep{
			Name:       "init-datadir",
			Status:     "failed",
			Command:    initCmd + " " + strings.Join(initArgs, " "),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("%v: %s", err, string(out)),
		})
		steps = append(steps, cleanupTempStep(cfg.DataDir, cfg.KeepTemp))
		return steps, fmt.Errorf("init datadir: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "init-datadir",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Step: start server
	start = time.Now()
	serverArgs := buildStartArgs(cfg.ProviderPath, cfg.DataDir, cfg.Port)
	serverCmd, err := r.exec.Start(ctx, cfg.ProviderPath, serverArgs...)
	if err != nil {
		steps = append(steps, executedStep{
			Name:       "start-server",
			Status:     "failed",
			Command:    cfg.ProviderPath + " " + strings.Join(serverArgs, " "),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		steps = append(steps, cleanupTempStep(cfg.DataDir, cfg.KeepTemp))
		return steps, fmt.Errorf("start server: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "start-server",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	defer func() {
		steps = append(steps, stopServerStep(cfg, serverCmd, r.exec)...)
		steps = append(steps, cleanupTempStep(cfg.DataDir, cfg.KeepTemp))
	}()

	// Step: wait for server readiness
	start = time.Now()
	if err := waitForServerReady(ctx, cfg.Port); err != nil {
		steps = append(steps, executedStep{
			Name:       "wait-ready",
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		return steps, fmt.Errorf("wait ready: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "wait-ready",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Step: bootstrap database
	start = time.Now()
	if err := bootstrapDatabase(ctx, cfg); err != nil {
		steps = append(steps, executedStep{
			Name:       "bootstrap-database",
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		return steps, fmt.Errorf("bootstrap database: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "bootstrap-database",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	// Optional: sqlite-export
	effectiveExportDir := cfg.ExportDir
	if strings.TrimSpace(cfg.SQLiteDB) != "" {
		start = time.Now()
		effectiveExportDir = filepath.Join(cfg.DataDir, "export")
		exportArgs := []string{"-db", cfg.SQLiteDB, "-out", effectiveExportDir, "-all"}
		exportCmd, exportRunArgs, exportDisplay := managedCommand(r.exec, "sqlite-export", exportArgs...)
		if out, err := r.exec.Run(ctx, exportCmd, exportRunArgs...); err != nil {
			steps = append(steps, executedStep{
				Name:       "sqlite-export",
				Status:     "failed",
				Command:    exportDisplay,
				ExitCode:   -1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Sprintf("%v: %s", err, string(out)),
			})
			return steps, fmt.Errorf("sqlite export: %w", err)
		}
		steps = append(steps, executedStep{
			Name:       "sqlite-export",
			Status:     "ok",
			DurationMs: time.Since(start).Milliseconds(),
		})
	}

	// Step: mariadb-schema
	start = time.Now()
	dsn := buildInternalDSN(cfg.DataDir, cfg.Port, cfg.SessionID)
	schemaArgs := []string{"-dsn", dsn, "-execute"}
	schemaCmd, schemaRunArgs, schemaDisplay := managedCommand(r.exec, "mariadb-schema", schemaArgs...)
	if out, err := r.exec.Run(ctx, schemaCmd, schemaRunArgs...); err != nil {
		steps = append(steps, executedStep{
			Name:       "mariadb-schema",
			Status:     "failed",
			Command:    strings.ReplaceAll(schemaDisplay, dsn, redactDSN(dsn)),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("%v: %s", err, string(out)),
		})
		return steps, fmt.Errorf("mariadb schema: %w", err)
	}
	steps = append(steps, executedStep{
		Name:       "mariadb-schema",
		Status:     "ok",
		DurationMs: time.Since(start).Milliseconds(),
	})

	if !cfg.skipDefaultReadShadow() {
		// Step: mariadb-import
		start = time.Now()
		importArgs := []string{"-export-dir", effectiveExportDir, "-dsn", dsn, "-execute"}
		importCmd, importRunArgs, importDisplay := managedCommand(r.exec, "mariadb-import", importArgs...)
		if out, err := r.exec.Run(ctx, importCmd, importRunArgs...); err != nil {
			steps = append(steps, executedStep{
				Name:       "mariadb-import",
				Status:     "failed",
				Command:    strings.ReplaceAll(importDisplay, dsn, redactDSN(dsn)),
				ExitCode:   -1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Sprintf("%v: %s", err, string(out)),
			})
			return steps, fmt.Errorf("mariadb import: %w", err)
		}
		steps = append(steps, executedStep{
			Name:       "mariadb-import",
			Status:     "ok",
			DurationMs: time.Since(start).Milliseconds(),
		})

		// Step: mariadb-compare
		start = time.Now()
		compareArgs := []string{"-export-dir", effectiveExportDir, "-dsn", dsn}
		compareCmd, compareRunArgs, compareDisplay := managedCommand(r.exec, "mariadb-compare", compareArgs...)
		if out, err := r.exec.Run(ctx, compareCmd, compareRunArgs...); err != nil {
			steps = append(steps, executedStep{
				Name:       "mariadb-compare",
				Status:     "failed",
				Command:    strings.ReplaceAll(compareDisplay, dsn, redactDSN(dsn)),
				ExitCode:   -1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Sprintf("%v: %s", err, string(out)),
			})
			return steps, fmt.Errorf("mariadb compare: %w", err)
		}
		steps = append(steps, executedStep{
			Name:       "mariadb-compare",
			Status:     "ok",
			DurationMs: time.Since(start).Milliseconds(),
		})
	}

	goBinPath := cfg.GoBinPath
	if goBinPath == "" {
		if p, err := r.exec.LookPath("archive-center-go"); err == nil {
			goBinPath = p
		}
	}
	if goBinPath == "" {
		steps = append(steps, executedStep{
			Name:   "start-go-backend",
			Status: "failed",
			Error:  "archive-center-go binary not found in PATH and no -go-bin provided",
		})
		return steps, fmt.Errorf("go backend binary not found")
	}

	effectivePythonBaseURL := cfg.PythonBaseURL
	if (cfg.DefaultSwitch || cfg.AuthorityCutover) && strings.TrimSpace(cfg.PythonFallbackSrc) != "" {
		fallbackDir := filepath.Join(cfg.DataDir, "python-fallback-0.8")
		start = time.Now()
		err := copyPythonFallbackSource(ctx, cfg.PythonFallbackSrc, fallbackDir)
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "python-fallback-copy",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details: map[string]any{
				"source_dir": cfg.PythonFallbackSrc,
				"temp_dir":   fallbackDir,
			},
		})
		if err != nil {
			return steps, fmt.Errorf("python fallback copy: %w", err)
		}

		fallbackPort := cfg.PythonFallbackPort
		if fallbackPort <= 0 {
			fallbackPort = 18106
		}
		fallbackPort, err = findAvailablePort("127.0.0.1", fallbackPort, fallbackPort+20)
		if err != nil {
			steps = append(steps, executedStep{Name: "python-fallback-start", Status: "failed", Error: err.Error()})
			return steps, fmt.Errorf("python fallback port: %w", err)
		}
		effectivePythonBaseURL = fmt.Sprintf("http://127.0.0.1:%d", fallbackPort)
		start = time.Now()
		pythonFallback, step, err := startPythonFallbackBackend(ctx, fallbackDir, fallbackPort)
		step.DurationMs = time.Since(start).Milliseconds()
		step.Details = map[string]any{
			"base_url": effectivePythonBaseURL,
			"temp_dir": fallbackDir,
		}
		steps = append(steps, step)
		if err != nil {
			return steps, fmt.Errorf("python fallback start: %w", err)
		}
		defer pythonFallback.stop()

		waitStep := waitPythonFallbackReady(ctx, fallbackPort)
		steps = append(steps, waitStep)
		if waitStep.Status != "ok" {
			return steps, fmt.Errorf("python fallback not ready: %s", waitStep.Error)
		}
	}

	if cfg.RouteWriteSmoke {
		routePort, err := findAvailablePort("127.0.0.1", cfg.GoHTTPPort, cfg.GoHTTPPort+20)
		if err != nil {
			steps = append(steps, executedStep{
				Name:   "route-start-go-backend",
				Status: "failed",
				Error:  err.Error(),
			})
			return steps, fmt.Errorf("find route write go backend port: %w", err)
		}

		routeCmd, routeStart, err := startGoBackend(ctx, r.exec, goBackendStartConfig{
			BinPath:          goBinPath,
			Port:             routePort,
			DSN:              dsn,
			StoreMode:        "mariadb_shadow",
			ProductReadProof: false,
		})
		routeStart.Name = "route-start-go-backend"
		steps = append(steps, routeStart)
		if err != nil {
			return steps, fmt.Errorf("start route write go backend: %w", err)
		}
		routeStopped := false
		defer func() {
			if !routeStopped {
				step := stopGoBackend(r.exec, routeCmd)
				step.Name = "route-stop-go-backend"
				steps = append(steps, step)
			}
		}()

		waitStep := waitGoReady(ctx, routePort)
		waitStep.Name = "route-wait-go-ready"
		steps = append(steps, waitStep)
		if waitStep.Status != "ok" {
			return steps, fmt.Errorf("route write go backend not ready: %s", waitStep.Error)
		}

		start = time.Now()
		smokeDetails, err := runRouteWriteSmoke(ctx, routePort, dsn, cfg.SessionID, "mariadb_shadow")
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "route-write-smoke",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    smokeDetails,
		})
		stopRoute := stopGoBackend(r.exec, routeCmd)
		stopRoute.Name = "route-stop-go-backend"
		steps = append(steps, stopRoute)
		routeStopped = true
		if err != nil {
			return steps, fmt.Errorf("route write smoke: %w", err)
		}

	}

	if cfg.SessionIsolationSmoke {
		isoPort, err := findAvailablePort("127.0.0.1", cfg.GoHTTPPort, cfg.GoHTTPPort+20)
		if err != nil {
			steps = append(steps, executedStep{
				Name:   "session-isolation-start-go-backend",
				Status: "failed",
				Error:  err.Error(),
			})
			return steps, fmt.Errorf("find session isolation go backend port: %w", err)
		}

		isoCmd, isoStart, err := startGoBackend(ctx, r.exec, goBackendStartConfig{
			BinPath:   goBinPath,
			Port:      isoPort,
			DSN:       dsn,
			StoreMode: "mariadb_authority",
		})
		isoStart.Name = "session-isolation-start-go-backend"
		steps = append(steps, isoStart)
		if err != nil {
			return steps, fmt.Errorf("start session isolation go backend: %w", err)
		}
		isoStopped := false
		defer func() {
			if !isoStopped {
				step := stopGoBackend(r.exec, isoCmd)
				step.Name = "session-isolation-stop-go-backend"
				steps = append(steps, step)
			}
		}()

		waitStep := waitGoReady(ctx, isoPort)
		waitStep.Name = "session-isolation-wait-go-ready"
		steps = append(steps, waitStep)
		if waitStep.Status != "ok" {
			return steps, fmt.Errorf("session isolation go backend not ready: %s", waitStep.Error)
		}

		start = time.Now()
		isoDetails, err := runSessionIsolationSmokeStandalone(ctx, isoPort, cfg.SessionID)
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "session-isolation-smoke",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    isoDetails,
		})
		stopIso := stopGoBackend(r.exec, isoCmd)
		stopIso.Name = "session-isolation-stop-go-backend"
		steps = append(steps, stopIso)
		isoStopped = true
		if err != nil {
			return steps, fmt.Errorf("session isolation smoke: %w", err)
		}
	}

	if cfg.skipDefaultReadShadow() {
		return steps, nil
	}

	if cfg.BackupRestore {
		start = time.Now()
		restoreDetails, err := runBackupRestoreDrill(ctx, cfg.Port, cfg.DataDir)
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "backup-restore-drill",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    restoreDetails,
		})
		if err != nil {
			return steps, fmt.Errorf("backup restore drill: %w", err)
		}
	}

	// --- Go backend steps ---
	goPort, err := findAvailablePort("127.0.0.1", cfg.GoHTTPPort, cfg.GoHTTPPort+20)
	if err != nil {
		steps = append(steps, executedStep{
			Name:   "start-go-backend",
			Status: "failed",
			Error:  err.Error(),
		})
		return steps, fmt.Errorf("find go backend port: %w", err)
	}

	goCmd, goStartStep, err := startGoBackend(ctx, r.exec, goBackendStartConfig{
		BinPath:          goBinPath,
		Port:             goPort,
		DSN:              dsn,
		StoreMode:        "mariadb_read_shadow",
		ProductReadProof: cfg.ProductReadProof,
	})
	steps = append(steps, goStartStep)
	if err != nil {
		return steps, fmt.Errorf("start go backend: %w", err)
	}
	// Ensure Go backend is stopped before server cleanup.
	goStopped := false
	defer func() {
		if !goStopped {
			steps = append(steps, stopGoBackend(r.exec, goCmd))
		}
	}()

	steps = append(steps, waitGoReady(ctx, goPort))
	last := steps[len(steps)-1]
	if last.Status != "ok" {
		return steps, fmt.Errorf("go backend not ready: %s", last.Error)
	}

	// Step: shadow-value-report
	start = time.Now()
	reportArgs := []string{
		"-go-base", fmt.Sprintf("http://127.0.0.1:%d", goPort),
		"-session-id", cfg.SessionID,
		"-out", filepath.Join(cfg.DataDir, "shadow-value-report.md"),
		"-json-out", filepath.Join(cfg.DataDir, "shadow-value-report.json"),
	}
	if effectivePythonBaseURL != "" {
		reportArgs = append(reportArgs, "-python-base", effectivePythonBaseURL)
	}
	reportCmd, reportRunArgs, reportDisplay := managedCommand(r.exec, "shadow-value-report", reportArgs...)
	if out, err := r.exec.Run(ctx, reportCmd, reportRunArgs...); err != nil {
		steps = append(steps, executedStep{
			Name:       "shadow-value-report",
			Status:     "failed",
			Command:    reportDisplay,
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("%v: %s", err, string(out)),
		})
		return steps, &degradedError{msg: fmt.Sprintf("shadow-value-report failed: %v", err)}
	}
	steps = append(steps, executedStep{
		Name:       "shadow-value-report",
		Status:     "ok",
		Command:    reportDisplay,
		DurationMs: time.Since(start).Milliseconds(),
	})

	if cfg.DefaultSwitch {
		start = time.Now()
		candidateDetails, err := runDefaultCandidateProbe(ctx, goPort, cfg.DefaultSwitchActual)
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		stepName := "go-default-candidate-probe"
		if cfg.DefaultSwitchActual {
			stepName = "go-default-actual-switch-gate"
		}
		steps = append(steps, executedStep{
			Name:       stepName,
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    candidateDetails,
		})
		if err != nil {
			return steps, fmt.Errorf("go default candidate probe: %w", err)
		}
	}

	if cfg.AuthorityCutover {
		start = time.Now()
		authorityDetails, authoritySteps, err := runAuthorityCutoverReplay(ctx, r.exec, cfg, goBinPath, dsn, effectivePythonBaseURL, goPort+31)
		for _, step := range authoritySteps {
			if step.DurationMs == 0 && step.Name == "authority-start-go-backend" {
				step.DurationMs = time.Since(start).Milliseconds()
			}
			steps = append(steps, step)
		}
		status := "ok"
		errText := ""
		if err != nil {
			status = "failed"
			errText = err.Error()
		}
		steps = append(steps, executedStep{
			Name:       "authority-cutover-summary",
			Status:     status,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errText,
			Details:    authorityDetails,
		})
		if err != nil {
			return steps, fmt.Errorf("authority cutover replay: %w", err)
		}
	}

	if cfg.ProductReadProof {
		stopStep := stopGoBackend(r.exec, goCmd)
		stopStep.Name = "rollback-stop-product-go-backend"
		steps = append(steps, stopStep)
		goStopped = true

		if cfg.DefaultSwitch || cfg.AuthorityCutover {
			start = time.Now()
			fallbackDetails, err := runPythonFallbackProbe(ctx, effectivePythonBaseURL)
			status := "ok"
			errText := ""
			if err != nil {
				status = "failed"
				errText = err.Error()
			}
			steps = append(steps, executedStep{
				Name:       "python-fallback-replay",
				Status:     status,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      errText,
				Details:    fallbackDetails,
			})
			if err != nil {
				return steps, fmt.Errorf("python fallback replay: %w", err)
			}
		}

		rollbackPort, err := findAvailablePort("127.0.0.1", goPort+1, goPort+30)
		if err != nil {
			steps = append(steps, executedStep{
				Name:   "rollback-start-go-backend",
				Status: "failed",
				Error:  err.Error(),
			})
			return steps, fmt.Errorf("find rollback go backend port: %w", err)
		}

		rollbackCmd, rollbackStart, err := startGoBackend(ctx, r.exec, goBackendStartConfig{
			BinPath:   goBinPath,
			Port:      rollbackPort,
			StoreMode: "noop",
		})
		rollbackStart.Name = "rollback-start-go-backend"
		steps = append(steps, rollbackStart)
		if err != nil {
			return steps, fmt.Errorf("start rollback go backend: %w", err)
		}

		rollbackStopped := false
		defer func() {
			if !rollbackStopped {
				step := stopGoBackend(r.exec, rollbackCmd)
				step.Name = "rollback-stop-go-backend"
				steps = append(steps, step)
			}
		}()

		waitStep := waitGoReady(ctx, rollbackPort)
		waitStep.Name = "rollback-wait-go-ready"
		steps = append(steps, waitStep)
		if waitStep.Status != "ok" {
			return steps, fmt.Errorf("rollback go backend not ready: %s", waitStep.Error)
		}

		checks, err := fetchReadyChecks(ctx, rollbackPort)
		if err != nil {
			steps = append(steps, executedStep{
				Name:   "rollback-ready-check",
				Status: "failed",
				Error:  err.Error(),
			})
			return steps, fmt.Errorf("rollback ready check: %w", err)
		}
		rolledBack := checks["store_mode"] == "noop" &&
			checks["mariadb_product_read"] == "disabled" &&
			checks["mariadb_authority"] == "disabled"
		status := "ok"
		if !rolledBack {
			status = "failed"
		}
		steps = append(steps, executedStep{
			Name:   "rollback-ready-check",
			Status: status,
			Note:   fmt.Sprintf("store_mode=%s mariadb_product_read=%s mariadb_authority=%s", checks["store_mode"], checks["mariadb_product_read"], checks["mariadb_authority"]),
		})
		stopRollback := stopGoBackend(r.exec, rollbackCmd)
		stopRollback.Name = "rollback-stop-go-backend"
		steps = append(steps, stopRollback)
		rollbackStopped = true
		if !rolledBack {
			return steps, fmt.Errorf("rollback proof failed")
		}
	}

	return steps, nil
}

func waitForServerReady(ctx context.Context, port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		_ = conn.Close()
		return nil
	}
}

func bootstrapDatabase(ctx context.Context, cfg directProviderConfig) error {
	rootDSN := fmt.Sprintf("root@tcp(127.0.0.1:%d)/?timeout=3s&readTimeout=3s&writeTimeout=3s", cfg.Port)
	db, err := sql.Open("mysql", rootDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	user := "ac_root"
	password := buildPassword(cfg.SessionID)
	dbName := "archive_center_temp"
	userLit := quoteStringLiteral(user)
	passwordLit := quoteStringLiteral(password)
	dbIdent := escapeIdentifier(dbName)

	createDBSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbIdent)
	if _, err := db.ExecContext(ctx, createDBSQL); err != nil {
		return fmt.Errorf("create database: %w", err)
	}

	createUserSQL := fmt.Sprintf("CREATE USER IF NOT EXISTS %s@'127.0.0.1' IDENTIFIED BY %s", userLit, passwordLit)
	if _, err := db.ExecContext(ctx, createUserSQL); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	grantSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s@'127.0.0.1'", dbIdent, userLit)
	if _, err := db.ExecContext(ctx, grantSQL); err != nil {
		return fmt.Errorf("grant privileges: %w", err)
	}

	createLocalhostSQL := fmt.Sprintf("CREATE USER IF NOT EXISTS %s@'localhost' IDENTIFIED BY %s", userLit, passwordLit)
	if _, err := db.ExecContext(ctx, createLocalhostSQL); err == nil {
		grantLocalhostSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s@'localhost'", dbIdent, userLit)
		_, _ = db.ExecContext(ctx, grantLocalhostSQL)
	}
	_, _ = db.ExecContext(ctx, "FLUSH PRIVILEGES")

	return nil
}

func buildInitArgs(providerPath, dataDir string) []string {
	return []string{"--no-defaults", "--initialize-insecure", "--datadir", dataDir}
}

func buildInitCommand(providerPath, dataDir string) (string, []string) {
	if runtime.GOOS == "windows" {
		binDir := filepath.Dir(providerPath)
		for _, name := range []string{"mariadb-install-db.exe", "mysql_install_db.exe"} {
			candidate := filepath.Join(binDir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, []string{"--datadir=" + dataDir, "--password="}
			}
		}
	}
	return providerPath, buildInitArgs(providerPath, dataDir)
}

func buildStartArgs(providerPath, dataDir string, port int) []string {
	return []string{
		"--no-defaults",
		"--datadir", dataDir,
		"--port", strconv.Itoa(port),
		"--socket", filepath.Join(dataDir, "mysql.sock"),
		"--skip-networking=0",
		"--bind-address=127.0.0.1",
		"--pid-file", filepath.Join(dataDir, "mysqld.pid"),
	}
}

func stopServerStep(cfg directProviderConfig, serverCmd *exec.Cmd, exec commandExecutor) []executedStep {
	var out []executedStep
	if serverCmd != nil && serverCmd.Process != nil {
		_ = exec.Kill(serverCmd)
		out = append(out, executedStep{Name: "stop-server", Status: "ok"})
	} else {
		out = append(out, executedStep{Name: "stop-server", Status: "ok", Note: "no running process"})
	}
	return out
}

func cleanupTempStep(dataDir string, keep bool) executedStep {
	if keep {
		return executedStep{Name: "cleanup", Status: "retained", Note: "keep-temp=true"}
	}
	if err := os.RemoveAll(dataDir); err != nil {
		return executedStep{Name: "cleanup", Status: "failed", Error: err.Error()}
	}
	return executedStep{Name: "cleanup", Status: "ok"}
}

var defaultDirectRunner directProviderRunner = newOSDirectProviderRunner()
