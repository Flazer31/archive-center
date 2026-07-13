package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	sqliteDB := flag.String("sqlite-db", "", "Path to SQLite database")
	exportDir := flag.String("export-dir", "", "Path to export directory")
	pythonBaseURL := flag.String("python-base", "http://127.0.0.1:8000", "Python 0.8 backend base URL for shadow-value-report")
	outPath := flag.String("out", "", "JSON report path; stdout if empty")
	execute := flag.Bool("execute", false, "Execute the plan (default: guarded)")
	keepTemp := flag.Bool("keep-temp", false, "Keep temporary directory after run")
	sessionID := flag.String("session-id", "managed-mariadb-e2e", "Session ID for temp resources")
	productReadProof := flag.Bool("product-read-proof", false, "Enable R2 MariaDB product-read flag and rollback proof on the disposable Go backend")
	routeWriteSmoke := flag.Bool("route-write-smoke", false, "Run disposable Go HTTP route write smoke against temp MariaDB in mariadb_shadow mode")
	sessionIsolationSmoke := flag.Bool("session-isolation-smoke", false, "Run disposable RMG-03 session isolation smoke against temp MariaDB in mariadb_authority mode")
	backupRestoreDrill := flag.Bool("backup-restore-drill", false, "Clone source MariaDB into a restored database and verify table row counts before cutover")
	authorityCutoverReplay := flag.Bool("authority-cutover-replay", false, "Run a managed disposable MariaDB authority cutover replay with post-cutover replay and rollback proof")
	defaultSwitchRehearsal := flag.Bool("default-switch-rehearsal", false, "Probe Go as a disposable default-runtime candidate, stop it, then prove Python fallback remains reachable")
	defaultSwitchActual := flag.Bool("default-switch-actual", false, "Run a managed disposable Go default-runtime actual switch gate with post-switch replay and Python fallback proof")
	pythonFallbackSrc := flag.String("python-fallback-src-dir", "", "Optional 0.8 source tree to temp-copy and start as Python fallback for default-switch rehearsal")
	pythonFallbackPort := flag.Int("python-fallback-port", 18106, "Preferred Python fallback temp backend port")
	goBin := flag.String("go-bin", "", "Optional archive-center-go binary path for read-shadow value report")
	providerBin := flag.String("provider-bin", "", "Explicit path to MariaDB server binary (mariadbd or mysqld)")
	flag.Parse()
	if *providerBin == "" {
		*providerBin = os.Getenv("AC_MARIADB_PROVIDER_BIN")
	}

	if *defaultSwitchActual {
		*defaultSwitchRehearsal = true
	}
	if *authorityCutoverReplay {
		*productReadProof = true
	}
	r := runWithOptions(*sqliteDB, *exportDir, *pythonBaseURL, *execute, *keepTemp, *sessionID, *productReadProof, *routeWriteSmoke, *backupRestoreDrill, *authorityCutoverReplay, *defaultSwitchRehearsal, *defaultSwitchActual, *sessionIsolationSmoke, *pythonFallbackSrc, *pythonFallbackPort, *goBin, *providerBin, bundledLookup{fallback: osExecLookup{}})
	reportJSON, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal report: %v\n", err)
		os.Exit(1)
	}
	reportJSON = append(reportJSON, '\n')
	if *outPath != "" {
		if err := os.WriteFile(*outPath, reportJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(1)
		}
	} else {
		_, _ = os.Stdout.Write(reportJSON)
	}
	switch r.Status {
	case "ok":
		os.Exit(0)
	case "blocked":
		os.Exit(3)
	case "degraded":
		os.Exit(4)
	default:
		os.Exit(2)
	}
}

func run(sqliteDB, exportDir, pythonBaseURL string, execute, keepTemp bool, sessionID string, lookup providerLookup) *report {
	return runWithOptions(sqliteDB, exportDir, pythonBaseURL, execute, keepTemp, sessionID, false, false, false, false, false, false, false, "", 0, "", "", lookup)
}

func runWithOptions(sqliteDB, exportDir, pythonBaseURL string, execute, keepTemp bool, sessionID string, productReadProof bool, routeWriteSmoke bool, backupRestoreDrill bool, authorityCutoverReplay bool, defaultSwitchRehearsal bool, defaultSwitchActual bool, sessionIsolationSmoke bool, pythonFallbackSrc string, pythonFallbackPort int, goBinPath string, explicitProviderBin string, lookup providerLookup) *report {
	sessionOnly := (directProviderConfig{
		ProductReadProof:      productReadProof,
		RouteWriteSmoke:       routeWriteSmoke,
		BackupRestore:         backupRestoreDrill,
		AuthorityCutover:      authorityCutoverReplay,
		DefaultSwitch:         defaultSwitchRehearsal,
		DefaultSwitchActual:   defaultSwitchActual,
		SessionIsolationSmoke: sessionIsolationSmoke,
	}).skipDefaultReadShadow()
	r := &report{
		Status:                "ok",
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		SourceMode:            deriveSourceMode(sqliteDB, exportDir),
		SQLiteDB:              sqliteDB,
		ExportDir:             exportDir,
		Execute:               execute,
		KeepTemp:              keepTemp,
		ProductReadProof:      productReadProof,
		RouteWriteSmoke:       summarizeRouteWriteSmoke(nil, routeWriteSmoke),
		SessionIsolationSmoke: summarizeSessionIsolationSmoke(nil, sessionIsolationSmoke),
		BackupRestore:         summarizeBackupRestore(nil, backupRestoreDrill),
		AuthorityCutover:      summarizeAuthorityCutover(nil, authorityCutoverReplay),
		DefaultSwitch:         summarizeDefaultSwitchRehearsal(nil, defaultSwitchRehearsal),
		DefaultRuntime:        summarizeDefaultRuntimeSwitch(nil, defaultSwitchActual),
		RollbackProof:         summarizeRollbackProof(nil, productReadProof),
		VectorRuntime:         summarizeVectorRuntime(),
		SafetyFlags: map[string]bool{
			"authority_switch":                  false,
			"mariadb_product_read_persisted":    false,
			"mariadb_authority_default_enabled": false,
			"chromadb_required":                 true,
			"chromadb_endpoint_configured":      strings.TrimSpace(os.Getenv("AC_CHROMA_ENDPOINT")) != "",
			"chroma_retired":                    false,
			"go_default_switch":                 false,
		},
		SchemaTables:    parseSchemaTables(discoverSchemaSQL()),
		StoreSaveTables: knownStoreSaveTables,
		StoreListTables: knownStoreListTables,
	}

	hasSQLite := strings.TrimSpace(sqliteDB) != ""
	hasExport := strings.TrimSpace(exportDir) != ""

	if !hasSQLite && !hasExport && !sessionOnly {
		r.Status = "failed"
		r.Errors = append(r.Errors, "missing source: provide -sqlite-db or -export-dir")
		return r
	}

	if hasSQLite && hasExport {
		r.Status = "failed"
		r.Errors = append(r.Errors, "ambiguous source: provide only one of -sqlite-db or -export-dir")
		return r
	}

	if !execute {
		r.Status = "guarded"
		r.Warnings = append(r.Warnings, "execute=false: no provider start, no DB touch")
		return r
	}
	if defaultSwitchActual && !productReadProof {
		r.Status = "failed"
		r.Errors = append(r.Errors, "default-switch-actual requires -product-read-proof so rollback can be proven")
		return r
	}
	if authorityCutoverReplay && !productReadProof {
		r.Status = "failed"
		r.Errors = append(r.Errors, "authority-cutover-replay requires -product-read-proof so rollback can be proven")
		return r
	}

	directProviders := []string{"mariadbd", "mysqld"}
	containerProviders := []string{"docker", "podman", "nerdctl"}

	detected := ""
	detectedPath := ""
	detectedType := ""

	// Check explicit provider first.
	explicit := strings.TrimSpace(explicitProviderBin)
	if explicit != "" {
		name := filepath.Base(explicit)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		pi := providerInfo{Name: name, Path: explicit, Available: false, Type: "explicit_direct"}
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			if name == "mariadbd" || name == "mysqld" {
				pi.Available = true
			}
		}
		r.ProvidersChecked = append(r.ProvidersChecked, pi)
		if pi.Available {
			detected = pi.Name
			detectedPath = pi.Path
			detectedType = pi.Type
		}
	}

	if detected == "" {
		for _, name := range directProviders {
			pi := lookupProvider(lookup, name, "direct")
			r.ProvidersChecked = append(r.ProvidersChecked, pi)
			if pi.Available && detected == "" {
				detected = name
				detectedPath = pi.Path
				detectedType = pi.Type
			}
		}
		for _, name := range containerProviders {
			pi := lookupProvider(lookup, name, "container")
			r.ProvidersChecked = append(r.ProvidersChecked, pi)
			if pi.Available && detected == "" {
				detected = name
				detectedPath = pi.Path
				detectedType = pi.Type
			}
		}
	}

	if detected == "" {
		if explicit != "" {
			r.Status = "blocked"
			r.ProviderStatus = "missing_explicit"
			r.Errors = append(r.Errors, fmt.Sprintf("explicit MariaDB provider not found or not a server binary: %s", explicit))
			return r
		}
		r.Status = "blocked"
		r.ProviderStatus = "missing"
		r.Errors = append(r.Errors, "no MariaDB provider available")
		return r
	}

	safeID := safeSessionID(sessionID)
	tempDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("archive-center-mariadb-%s", safeID))
	r.TempPlan = tempPlan{
		DataDir:     tempDataDir,
		Port:        13306,
		DSNRedacted: redactDSN(buildInternalDSN(tempDataDir, 13306, safeID)),
	}

	if hasSQLite {
		r.ChildPlan = append(r.ChildPlan, childPlanStep{
			Name: "sqlite-export",
			Note: "sqlite-export -all -db " + sqliteDB,
		})
	}
	r.ChildPlan = append(r.ChildPlan, childPlanStep{Name: "mariadb-schema", Note: "apply schema to temp instance"})
	if !sessionOnly {
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "mariadb-import", Note: "import canonical NDJSON into temp instance"},
			{Name: "mariadb-compare", Note: "compare temp instance against source"},
		}...)
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "start-go-backend", Note: "start Go shadow backend against temp MariaDB"},
			{Name: "wait-go-ready", Note: "wait for Go backend health"},
			{Name: "shadow-value-report", Note: "mariadb_read_shadow report with -go-base"},
			{Name: "stop-go-backend", Note: "stop Go shadow backend"},
		}...)
	}
	if productReadProof {
		rollbackSteps := []childPlanStep{
			{Name: "rollback-stop-product-go-backend", Note: "stop product-read proof backend"},
			{Name: "rollback-start-go-backend", Note: "restart Go backend with AC_STORE_MODE=noop and without AC_MARIADB_PRODUCT_READ_ENABLED"},
			{Name: "rollback-wait-go-ready", Note: "wait for rollback backend health"},
			{Name: "rollback-ready-check", Note: "prove store_mode=noop, mariadb_product_read=disabled, and mariadb_authority=disabled"},
			{Name: "rollback-stop-go-backend", Note: "stop rollback backend"},
		}
		if defaultSwitchRehearsal || authorityCutoverReplay {
			rollbackSteps = append([]childPlanStep{rollbackSteps[0], childPlanStep{
				Name: "python-fallback-replay",
				Note: "with Go candidate stopped, prove the Python fallback base remains reachable",
			}}, rollbackSteps[1:]...)
		}
		r.ChildPlan = append(r.ChildPlan, rollbackSteps...)
	}
	if defaultSwitchRehearsal {
		stepName := "go-default-candidate-probe"
		stepNote := "probe Go as the selected default-runtime candidate while keeping authority/default flags non-persistent"
		if defaultSwitchActual {
			stepName = "go-default-actual-switch-gate"
			stepNote = "run the managed disposable Go default-runtime actual switch gate with post-switch replay evidence"
		}
		r.ChildPlan = append(r.ChildPlan, childPlanStep{
			Name: stepName,
			Note: stepNote,
		})
	}
	if authorityCutoverReplay {
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "authority-start-go-backend", Note: "start Go backend in AC_STORE_MODE=mariadb_authority against temp MariaDB"},
			{Name: "authority-wait-go-ready", Note: "wait for authority backend health"},
			{Name: "authority-ready-check", Note: "prove store_mode=mariadb_authority, mariadb_product_read=enabled, and mariadb_authority=enabled"},
			{Name: "authority-route-write-smoke", Note: "POST migrated write routes, then verify MariaDB row deltas through the authority store"},
			{Name: "authority-post-cutover-replay", Note: "run read replay against the authority backend"},
			{Name: "authority-stop-go-backend", Note: "stop authority backend before rollback proof"},
		}...)
	}
	if routeWriteSmoke {
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "route-start-go-backend", Note: "start Go backend in AC_STORE_MODE=mariadb_shadow against temp MariaDB for disposable write smoke"},
			{Name: "route-wait-go-ready", Note: "wait for route write smoke backend health"},
			{Name: "route-write-smoke", Note: "POST /complete-turn and /effective-inputs, then verify MariaDB row deltas"},
			{Name: "route-stop-go-backend", Note: "stop route write smoke backend"},
		}...)
	}
	if sessionIsolationSmoke {
		r.ChildPlan = append(r.ChildPlan, []childPlanStep{
			{Name: "session-isolation-start-go-backend", Note: "start disposable Go backend in AC_STORE_MODE=mariadb_authority for RMG-03 session isolation smoke"},
			{Name: "session-isolation-wait-go-ready", Note: "wait for session isolation smoke backend health"},
			{Name: "session-isolation-smoke", Note: "POST /complete-turn for two sessions, verify /sessions and /timeline isolation"},
			{Name: "session-isolation-stop-go-backend", Note: "stop session isolation smoke backend"},
		}...)
	}
	if backupRestoreDrill {
		r.ChildPlan = append(r.ChildPlan, childPlanStep{
			Name: "backup-restore-drill",
			Note: "clone temp MariaDB source database into a restored database and verify all table row counts",
		})
	}

	if detectedType == "container" {
		r.Status = "blocked"
		r.ProviderStatus = "detected_not_implemented"
		r.Warnings = append(r.Warnings, fmt.Sprintf("provider %q detected but container bootstrap not yet implemented", detected))
		return r
	}

	// Direct provider execution flow.
	switch detectedType {
	case "explicit_direct":
		r.ProviderStatus = "detected_explicit_direct"
	case "bundled_direct":
		r.ProviderStatus = "detected_bundled_direct"
	default:
		r.ProviderStatus = "detected_direct"
	}
	cfg := directProviderConfig{
		ProviderName:          detected,
		ProviderPath:          detectedPath,
		DataDir:               tempDataDir,
		Port:                  13306,
		SessionID:             safeID,
		SQLiteDB:              sqliteDB,
		ExportDir:             exportDir,
		KeepTemp:              keepTemp,
		GoHTTPPort:            28180,
		PythonBaseURL:         pythonBaseURL,
		PythonFallbackSrc:     pythonFallbackSrc,
		PythonFallbackPort:    pythonFallbackPort,
		ProductReadProof:      productReadProof,
		RouteWriteSmoke:       routeWriteSmoke,
		SessionIsolationSmoke: sessionIsolationSmoke,
		BackupRestore:         backupRestoreDrill,
		AuthorityCutover:      authorityCutoverReplay,
		DefaultSwitch:         defaultSwitchRehearsal,
		DefaultSwitchActual:   defaultSwitchActual,
		GoBinPath:             goBinPath,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	steps, runErr := defaultDirectRunner.run(ctx, cfg)
	r.ExecutedSteps = steps
	r.RollbackProof = summarizeRollbackProof(steps, productReadProof)
	r.RouteWriteSmoke = summarizeRouteWriteSmoke(steps, routeWriteSmoke)
	r.SessionIsolationSmoke = summarizeSessionIsolationSmoke(steps, sessionIsolationSmoke)
	r.BackupRestore = summarizeBackupRestore(steps, backupRestoreDrill)
	r.AuthorityCutover = summarizeAuthorityCutover(steps, authorityCutoverReplay)
	r.DefaultSwitch = summarizeDefaultSwitchRehearsal(steps, defaultSwitchRehearsal)
	r.DefaultRuntime = summarizeDefaultRuntimeSwitch(steps, defaultSwitchActual)
	if runErr != nil {
		var degr *degradedError
		if errors.As(runErr, &degr) {
			r.Status = "degraded"
			r.Errors = append(r.Errors, runErr.Error())
			r.Warnings = append(r.Warnings, "temp MariaDB import/compare succeeded but shadow-value-report could not complete")
		} else {
			r.Status = "failed"
			r.Errors = append(r.Errors, runErr.Error())
		}
		return r
	}
	r.Status = "ok"
	return r
}
