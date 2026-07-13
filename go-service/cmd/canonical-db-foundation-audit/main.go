// canonical-db-foundation-audit verifies the R1 Canonical DB foundation.
//
// This command does not start MariaDB, write data, switch authority, or read the
// 0.8 runtime tree. It closes the R1 foundation question by checking that the
// 2.0-side schema, Store/MariaDB source usage, import/compare tools, managed
// runner, and guarded execution contracts all exist and line up.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const schemaVersion = "archive-center.canonical-db-foundation-audit.v1"

var canonicalTables = []string{
	"chat_logs",
	"effective_input_logs",
	"memories",
	"direct_evidence_records",
	"kg_triples",
	"audit_logs",
	"critic_feedback",
	"character_events",
	"storylines",
	"world_rules",
	"character_states",
	"pending_threads",
	"active_states",
	"canonical_state_layers",
	"episode_summaries",
}

var storeSaveTables = []string{
	"chat_logs",
	"effective_input_logs",
	"memories",
	"direct_evidence_records",
	"kg_triples",
	"audit_logs",
	"critic_feedback",
	"character_events",
}

var storeReadTables = []string{
	"chat_logs",
	"effective_input_logs",
	"memories",
	"direct_evidence_records",
	"kg_triples",
	"audit_logs",
	"critic_feedback",
	"character_events",
	"storylines",
	"world_rules",
	"character_states",
	"pending_threads",
	"active_states",
	"canonical_state_layers",
	"episode_summaries",
}

var requiredExecutors = []executorCheck{
	{Name: "python_sqlite_export", Role: "0.8 SQLite snapshot export", Path: filepath.Join("tools", "export_sqlite_to_ndjson.py")},
	{Name: "go_sqlite_export", Role: "Go-side SQLite export parity", Path: filepath.Join("go-service", "cmd", "sqlite-export", "main.go")},
	{Name: "dry_run_validator", Role: "NDJSON manifest and checksum validation", Path: filepath.Join("go-service", "cmd", "dry-run-validator", "main.go")},
	{Name: "compare_dry_run", Role: "SQLite snapshot versus dry-run report comparison", Path: filepath.Join("go-service", "cmd", "compare-dry-run", "main.go")},
	{Name: "mariadb_dry_run_import", Role: "MariaDB import planning without writes", Path: filepath.Join("go-service", "cmd", "mariadb-dry-run-import", "main.go")},
	{Name: "mariadb_schema", Role: "Guarded schema apply executor", Path: filepath.Join("go-service", "cmd", "mariadb-schema", "main.go")},
	{Name: "mariadb_import", Role: "Guarded real MariaDB import executor", Path: filepath.Join("go-service", "cmd", "mariadb-import", "main.go")},
	{Name: "mariadb_compare", Role: "MariaDB read-compare executor", Path: filepath.Join("go-service", "cmd", "mariadb-compare", "main.go")},
	{Name: "mariadb_shadow_smoke", Role: "MariaDB shadow smoke", Path: filepath.Join("go-service", "cmd", "mariadb-shadow-smoke", "main.go")},
	{Name: "mariadb_e2e_smoke", Role: "SQLite snapshot to MariaDB E2E chain", Path: filepath.Join("go-service", "cmd", "mariadb-e2e-smoke", "main.go")},
	{Name: "managed_mariadb_e2e", Role: "Managed temp MariaDB provider runner", Path: filepath.Join("go-service", "cmd", "managed-mariadb-e2e", "main.go")},
	{Name: "canonical_route_smoke", Role: "Store-backed route smoke", Path: filepath.Join("go-service", "cmd", "canonical-route-smoke", "main.go")},
}

var guardChecks = []sourcePatternCheck{
	{
		Name:    "schema_execute_guard",
		Path:    filepath.Join("go-service", "cmd", "mariadb-schema", "main.go"),
		Pattern: "--execute is required",
	},
	{
		Name:    "import_execute_guard",
		Path:    filepath.Join("go-service", "cmd", "mariadb-import", "main.go"),
		Pattern: "--execute is required",
	},
	{
		Name:    "managed_runner_no_user_dsn",
		Path:    filepath.Join("go-service", "cmd", "managed-mariadb-e2e", "main.go"),
		Pattern: "does not accept a user-prepared DSN",
	},
	{
		Name:    "read_shadow_mode_present",
		Path:    filepath.Join("go-service", "internal", "config", "config.go"),
		Pattern: "mariadb_read_shadow",
	},
	{
		Name:    "store_mode_allowlist_guard_present",
		Path:    filepath.Join("go-service", "internal", "config", "config.go"),
		Pattern: "not allowed in this slice",
	},
}

type report struct {
	SchemaVersion      string               `json:"schema_version"`
	Status             string               `json:"status"`
	FoundationStatus   string               `json:"foundation_status"`
	ProductGateStatus  string               `json:"product_gate_status"`
	ProductGateGreen   bool                 `json:"product_gate_green"`
	GeneratedAt        string               `json:"generated_at"`
	Root               string               `json:"root"`
	Schema             schemaSection        `json:"schema"`
	StoreCoverage      storeCoverageSection `json:"store_coverage"`
	ToolCoverage       toolCoverageSection  `json:"tool_coverage"`
	ProviderBootstrap  providerSection      `json:"provider_bootstrap"`
	SafetyFlags        map[string]bool      `json:"safety_flags"`
	Errors             []string             `json:"errors,omitempty"`
	Warnings           []string             `json:"warnings,omitempty"`
	Blockers           []string             `json:"blockers,omitempty"`
	NonGoals           []string             `json:"non_goals"`
	NextRequiredAction string               `json:"next_required_action"`
}

type schemaSection struct {
	Path               string   `json:"path"`
	ExpectedTables     []string `json:"expected_tables"`
	DetectedTables     []string `json:"detected_tables"`
	MissingTables      []string `json:"missing_tables"`
	UnexpectedTables   []string `json:"unexpected_tables"`
	ExpectedTableCount int      `json:"expected_table_count"`
	DetectedTableCount int      `json:"detected_table_count"`
}

type storeCoverageSection struct {
	SourcePath                 string   `json:"source_path"`
	ExpectedSaveTables         []string `json:"expected_save_tables"`
	SaveTablesWithInsert       []string `json:"save_tables_with_insert"`
	MissingSaveInsertTables    []string `json:"missing_save_insert_tables"`
	ExpectedReadTables         []string `json:"expected_read_tables"`
	ReadTablesWithSelect       []string `json:"read_tables_with_select"`
	MissingReadSelectTables    []string `json:"missing_read_select_tables"`
	SchemaTablesWithoutSaveAPI []string `json:"schema_tables_without_save_api"`
	SchemaTablesWithoutReadAPI []string `json:"schema_tables_without_read_api"`
	StoreInterfaceMentionsAll  bool     `json:"store_interface_mentions_all"`
}

type toolCoverageSection struct {
	Executors            []executorResult      `json:"executors"`
	MissingExecutors     []string              `json:"missing_executors"`
	ImportTables         []string              `json:"import_tables"`
	ImportMissingTables  []string              `json:"import_missing_tables"`
	CompareTables        []string              `json:"compare_tables"`
	CompareMissingTables []string              `json:"compare_missing_tables"`
	GuardChecks          []sourcePatternResult `json:"guard_checks"`
	MissingGuardChecks   []string              `json:"missing_guard_checks"`
}

type providerSection struct {
	LocalProvidersChecked []providerResult `json:"local_providers_checked"`
	AnyLocalProvider      bool             `json:"any_local_provider"`
	BundledLayouts        []string         `json:"bundled_layouts"`
	ContainerProviders    []string         `json:"container_providers"`
	LiveEvidenceRequired  bool             `json:"live_evidence_required_for_product_gate"`
}

type providerResult struct {
	Name  string `json:"name"`
	Found bool   `json:"found"`
	Path  string `json:"path,omitempty"`
	Kind  string `json:"kind"`
}

type executorCheck struct {
	Name string
	Role string
	Path string
}

type executorResult struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

type sourcePatternCheck struct {
	Name    string
	Path    string
	Pattern string
}

type sourcePatternResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	Present bool   `json:"present"`
}

func main() {
	rootFlag := flag.String("root", "", "Archive Center 2.0 root. Defaults to auto-discovery from cwd.")
	outPath := flag.String("out", "", "Path to write JSON report. Defaults to stdout.")
	mdOutPath := flag.String("md-out", "", "Optional path to write a Markdown summary.")
	flag.Parse()

	root, err := resolveRoot(*rootFlag)
	if err != nil {
		rep := failureReport(err)
		writeJSON(rep, *outPath)
		if *mdOutPath != "" {
			_ = writeMarkdown(rep, *mdOutPath)
		}
		os.Exit(1)
	}

	rep := buildReport(root)
	writeJSON(rep, *outPath)
	if *mdOutPath != "" {
		if err := writeMarkdown(rep, *mdOutPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: writing markdown report: %v\n", err)
			os.Exit(1)
		}
	}
	if rep.Status != "complete" {
		os.Exit(1)
	}
}

func failureReport(err error) report {
	now := time.Now().UTC().Format(time.RFC3339)
	return report{
		SchemaVersion:      schemaVersion,
		Status:             "failed",
		FoundationStatus:   "blocked",
		ProductGateStatus:  "not_green",
		GeneratedAt:        now,
		Errors:             []string{err.Error()},
		NonGoals:           defaultNonGoals(),
		NextRequiredAction: "Fix audit root discovery before claiming Canonical DB foundation closure.",
	}
}

func resolveRoot(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if isArchiveCenter20Root(abs) {
			return abs, nil
		}
		return "", fmt.Errorf("root %q does not look like Archive Center 2.0", abs)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if isArchiveCenter20Root(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("could not discover Archive Center 2.0 root from %q", wd)
}

func isArchiveCenter20Root(path string) bool {
	if path == "" {
		return false
	}
	required := []string{
		filepath.Join(path, "go-service", "go.mod"),
		filepath.Join(path, "migrations", "001_schema.sql"),
		filepath.Join(path, "docs", "2.0-prep-readiness.md"),
	}
	for _, p := range required {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func buildReport(root string) report {
	now := time.Now().UTC().Format(time.RFC3339)
	schemaPath := filepath.Join(root, "migrations", "001_schema.sql")
	storePath := filepath.Join(root, "go-service", "internal", "store", "mariadb.go")
	storeInterfacePath := filepath.Join(root, "go-service", "internal", "store", "store.go")

	detectedTables, schemaErr := parseSchemaTables(schemaPath)
	schema := schemaSection{
		Path:               schemaPath,
		ExpectedTables:     append([]string(nil), canonicalTables...),
		DetectedTables:     detectedTables,
		MissingTables:      missing(canonicalTables, detectedTables),
		UnexpectedTables:   missing(detectedTables, canonicalTables),
		ExpectedTableCount: len(canonicalTables),
		DetectedTableCount: len(detectedTables),
	}

	storeCoverage, storeErrs := inspectStoreCoverage(storePath, storeInterfacePath, detectedTables)
	toolCoverage, toolErrs := inspectToolCoverage(root)
	providers := inspectProviders(root)

	rep := report{
		SchemaVersion:     schemaVersion,
		Status:            "complete",
		FoundationStatus:  "complete",
		ProductGateStatus: "not_green_provider_or_cutover_evidence_missing",
		ProductGateGreen:  false,
		GeneratedAt:       now,
		Root:              root,
		Schema:            schema,
		StoreCoverage:     storeCoverage,
		ToolCoverage:      toolCoverage,
		ProviderBootstrap: providers,
		SafetyFlags: map[string]bool{
			"authority_switch_enabled":       false,
			"mariadb_live_default_enabled":   false,
			"writes_reference_08_tree":       false,
			"requires_execute_for_db_writes": true,
			"managed_temp_runner_only":       true,
			"product_gate_green":             false,
		},
		Blockers: []string{
			"real MariaDB provider/runtime evidence is still required before product gate green",
			"Python 0.8 dual-write shadow window is still open",
			"backup-first cutover drill and rollback checkpoint are still open",
			"MariaDB authority switch and post-cutover audit are still open",
		},
		NonGoals:           defaultNonGoals(),
		NextRequiredAction: "Run managed MariaDB evidence with a real bundled/direct provider, then execute backup-first cutover and authority-switch drills.",
	}

	if schemaErr != nil {
		rep.Errors = append(rep.Errors, schemaErr.Error())
	}
	rep.Errors = append(rep.Errors, storeErrs...)
	rep.Errors = append(rep.Errors, toolErrs...)
	if len(schema.MissingTables) > 0 {
		rep.Errors = append(rep.Errors, "schema is missing expected canonical tables: "+strings.Join(schema.MissingTables, ", "))
	}
	if len(schema.UnexpectedTables) > 0 {
		rep.Warnings = append(rep.Warnings, "schema contains extra tables outside current canonical foundation: "+strings.Join(schema.UnexpectedTables, ", "))
	}
	if len(storeCoverage.MissingSaveInsertTables) > 0 {
		rep.Errors = append(rep.Errors, "MariaDB Store write coverage missing tables: "+strings.Join(storeCoverage.MissingSaveInsertTables, ", "))
	}
	if len(storeCoverage.MissingReadSelectTables) > 0 {
		rep.Errors = append(rep.Errors, "MariaDB Store read coverage missing tables: "+strings.Join(storeCoverage.MissingReadSelectTables, ", "))
	}
	if !storeCoverage.StoreInterfaceMentionsAll {
		rep.Errors = append(rep.Errors, "Store interface does not mention all expected R1 table families")
	}
	if len(toolCoverage.MissingExecutors) > 0 {
		rep.Errors = append(rep.Errors, "missing required executors: "+strings.Join(toolCoverage.MissingExecutors, ", "))
	}
	if len(toolCoverage.ImportMissingTables) > 0 {
		rep.Errors = append(rep.Errors, "mariadb-import canonical table list is missing: "+strings.Join(toolCoverage.ImportMissingTables, ", "))
	}
	if len(toolCoverage.CompareMissingTables) > 0 {
		rep.Errors = append(rep.Errors, "mariadb-compare canonical table list is missing: "+strings.Join(toolCoverage.CompareMissingTables, ", "))
	}
	if len(toolCoverage.MissingGuardChecks) > 0 {
		rep.Errors = append(rep.Errors, "missing guarded execution checks: "+strings.Join(toolCoverage.MissingGuardChecks, ", "))
	}
	if !providers.AnyLocalProvider {
		rep.Warnings = append(rep.Warnings, "no local MariaDB provider detected; this is a product-gate blocker, not a foundation failure")
	}

	if len(rep.Errors) > 0 {
		rep.Status = "incomplete"
		rep.FoundationStatus = "incomplete"
	}
	return rep
}

func parseSchemaTables(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	re := regexp.MustCompile(`(?im)^\s*CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS\s+` + "`?" + `([a-zA-Z0-9_]+)` + "`?" + `\s*\(`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	var tables []string
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		tables = append(tables, name)
	}
	return tables, nil
}

func inspectStoreCoverage(mariadbPath, storeInterfacePath string, schemaTables []string) (storeCoverageSection, []string) {
	var errs []string
	srcBytes, err := os.ReadFile(mariadbPath)
	if err != nil {
		errs = append(errs, fmt.Sprintf("read mariadb store source: %v", err))
	}
	src := string(srcBytes)

	interfaceBytes, err := os.ReadFile(storeInterfacePath)
	if err != nil {
		errs = append(errs, fmt.Sprintf("read store interface source: %v", err))
	}
	interfaceSrc := string(interfaceBytes)

	savePresent := tablesWithPattern(src, storeSaveTables, func(table string) string {
		return `(?is)INSERT\s+INTO\s+` + "`?" + regexp.QuoteMeta(table) + "`?"
	})
	readPresent := tablesWithPattern(src, storeReadTables, func(table string) string {
		return `(?is)\b(FROM|JOIN)\s+` + "`?" + regexp.QuoteMeta(table) + "`?"
	})

	mentionsAll := true
	for _, table := range storeReadTables {
		if !storeInterfaceMentionsTable(interfaceSrc, table) {
			mentionsAll = false
			break
		}
	}

	return storeCoverageSection{
		SourcePath:                 mariadbPath,
		ExpectedSaveTables:         append([]string(nil), storeSaveTables...),
		SaveTablesWithInsert:       savePresent,
		MissingSaveInsertTables:    missing(storeSaveTables, savePresent),
		ExpectedReadTables:         append([]string(nil), storeReadTables...),
		ReadTablesWithSelect:       readPresent,
		MissingReadSelectTables:    missing(storeReadTables, readPresent),
		SchemaTablesWithoutSaveAPI: missing(schemaTables, storeSaveTables),
		SchemaTablesWithoutReadAPI: missing(schemaTables, storeReadTables),
		StoreInterfaceMentionsAll:  mentionsAll,
	}, errs
}

func storeInterfaceMentionsTable(src, table string) bool {
	switch table {
	case "chat_logs":
		return strings.Contains(src, "SaveChatLog") && strings.Contains(src, "ListChatLogs")
	case "effective_input_logs":
		return strings.Contains(src, "SaveEffectiveInput") && strings.Contains(src, "GetEffectiveInput")
	case "memories":
		return strings.Contains(src, "SaveMemory") && strings.Contains(src, "ListMemories")
	case "direct_evidence_records":
		return strings.Contains(src, "SaveEvidence") && strings.Contains(src, "ListEvidence")
	case "kg_triples":
		return strings.Contains(src, "SaveKGTriple") && strings.Contains(src, "ListKGTriples")
	case "audit_logs":
		return strings.Contains(src, "SaveAuditLog") && strings.Contains(src, "ListAuditLogs")
	case "critic_feedback":
		return strings.Contains(src, "SaveCriticFeedback") && strings.Contains(src, "ListCriticFeedback")
	case "character_events":
		return strings.Contains(src, "SaveCharacterEvent") && strings.Contains(src, "ListCharacterEvents")
	case "storylines":
		return strings.Contains(src, "ListStorylines")
	case "world_rules":
		return strings.Contains(src, "ListWorldRules") && strings.Contains(src, "ListInheritedWorldRules")
	case "character_states":
		return strings.Contains(src, "ListCharacterStates") && strings.Contains(src, "GetCharacterState")
	case "pending_threads":
		return strings.Contains(src, "ListPendingThreads")
	case "active_states":
		return strings.Contains(src, "ListActiveStates")
	case "canonical_state_layers":
		return strings.Contains(src, "ListCanonicalStateLayers")
	case "episode_summaries":
		return strings.Contains(src, "ListEpisodeSummaries") && strings.Contains(src, "GetEpisodeSummary")
	default:
		return false
	}
}

func tablesWithPattern(src string, tables []string, pattern func(string) string) []string {
	var out []string
	for _, table := range tables {
		if regexp.MustCompile(pattern(table)).MatchString(src) {
			out = append(out, table)
		}
	}
	return out
}

func inspectToolCoverage(root string) (toolCoverageSection, []string) {
	var errs []string
	var executors []executorResult
	var missingExecutors []string
	for _, check := range requiredExecutors {
		p := filepath.Join(root, check.Path)
		exists := fileExists(p)
		executors = append(executors, executorResult{
			Name:   check.Name,
			Role:   check.Role,
			Path:   p,
			Exists: exists,
		})
		if !exists {
			missingExecutors = append(missingExecutors, check.Name)
		}
	}

	importTables, err := parseGoStringArray(filepath.Join(root, "go-service", "cmd", "mariadb-import", "main.go"), "canonicalTables")
	if err != nil {
		errs = append(errs, fmt.Sprintf("parse mariadb-import canonicalTables: %v", err))
	}
	compareTables, err := parseGoStringArray(filepath.Join(root, "go-service", "cmd", "mariadb-compare", "main.go"), "canonicalTables")
	if err != nil {
		errs = append(errs, fmt.Sprintf("parse mariadb-compare canonicalTables: %v", err))
	}

	var guardResults []sourcePatternResult
	var missingGuards []string
	for _, check := range guardChecks {
		p := filepath.Join(root, check.Path)
		data, err := os.ReadFile(p)
		present := err == nil && strings.Contains(string(data), check.Pattern)
		if err != nil {
			errs = append(errs, fmt.Sprintf("read guard check %s: %v", check.Name, err))
		}
		guardResults = append(guardResults, sourcePatternResult{
			Name:    check.Name,
			Path:    p,
			Pattern: check.Pattern,
			Present: present,
		})
		if !present {
			missingGuards = append(missingGuards, check.Name)
		}
	}

	return toolCoverageSection{
		Executors:            executors,
		MissingExecutors:     missingExecutors,
		ImportTables:         importTables,
		ImportMissingTables:  missing(canonicalTables, importTables),
		CompareTables:        compareTables,
		CompareMissingTables: missing(canonicalTables, compareTables),
		GuardChecks:          guardResults,
		MissingGuardChecks:   missingGuards,
	}, errs
}

func parseGoStringArray(path, name string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`(?s)var\s+` + regexp.QuoteMeta(name) + `\s*=\s*\[\]string\s*\{(.*?)\}`)
	m := re.FindStringSubmatch(string(data))
	if len(m) < 2 {
		return nil, fmt.Errorf("array %s not found", name)
	}
	itemRe := regexp.MustCompile(`"([^"]+)"`)
	matches := itemRe.FindAllStringSubmatch(m[1], -1)
	var out []string
	for _, match := range matches {
		if len(match) >= 2 {
			out = append(out, match[1])
		}
	}
	return out, nil
}

func inspectProviders(root string) providerSection {
	names := []struct {
		name string
		kind string
	}{
		{"mariadbd", "direct"},
		{"mysqld", "direct"},
		{"docker", "container"},
		{"podman", "container"},
		{"nerdctl", "container"},
	}
	var checked []providerResult
	any := false
	for _, n := range names {
		path, err := exec.LookPath(n.name)
		found := err == nil
		if found {
			any = true
		}
		checked = append(checked, providerResult{Name: n.name, Found: found, Path: path, Kind: n.kind})
	}
	for _, layout := range bundledLayouts(root) {
		info, err := os.Stat(layout)
		found := err == nil && !info.IsDir()
		if found {
			any = true
		}
		checked = append(checked, providerResult{Name: filepath.Base(layout), Found: found, Path: layout, Kind: "bundled_direct"})
	}
	return providerSection{
		LocalProvidersChecked: checked,
		AnyLocalProvider:      any,
		BundledLayouts:        bundledLayouts(root),
		ContainerProviders:    []string{"docker", "podman", "nerdctl"},
		LiveEvidenceRequired:  true,
	}
}

func bundledLayouts(root string) []string {
	names := []string{"mariadbd", "mysqld"}
	if runtime.GOOS == "windows" {
		names = []string{"mariadbd.exe", "mysqld.exe"}
	}
	roots := []string{
		filepath.Join(root, "mariadb", "bin"),
		filepath.Join(root, "MariaDB", "bin"),
		filepath.Join(root, "runtime", "mariadb", "bin"),
		filepath.Join(root, "runtime", "MariaDB", "bin"),
		filepath.Join(root, "vendor", "mariadb", "bin"),
		filepath.Join(root, "vendor", "MariaDB", "bin"),
		filepath.Join(root, "resources", "mariadb", "bin"),
		filepath.Join(root, "resources", "MariaDB", "bin"),
	}
	var out []string
	for _, r := range roots {
		for _, n := range names {
			out = append(out, filepath.Join(r, n))
		}
	}
	return out
}

func defaultNonGoals() []string {
	return []string{
		"does not run or mutate Archive Center Beta 0.8(fix)",
		"does not start MariaDB or require a DSN",
		"does not switch MariaDB authority",
		"does not mark product cutover readiness green",
		"does not migrate ChromaDB authority",
		"does not edit Archive Center.js",
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func missing(expected, actual []string) []string {
	seen := make(map[string]bool, len(actual))
	for _, v := range actual {
		seen[v] = true
	}
	var out []string
	for _, v := range expected {
		if !seen[v] {
			out = append(out, v)
		}
	}
	return out
}

func writeJSON(rep report, outPath string) {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encode report: %v\n", err)
		return
	}
	data = append(data, '\n')
	if outPath == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: create output dir: %v\n", err)
		return
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write report: %v\n", err)
	}
}

func writeMarkdown(rep report, outPath string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Canonical DB Foundation Audit\n\n")
	fmt.Fprintf(&b, "- schema_version: `%s`\n", rep.SchemaVersion)
	fmt.Fprintf(&b, "- status: `%s`\n", rep.Status)
	fmt.Fprintf(&b, "- foundation_status: `%s`\n", rep.FoundationStatus)
	fmt.Fprintf(&b, "- product_gate_status: `%s`\n", rep.ProductGateStatus)
	fmt.Fprintf(&b, "- product_gate_green: `%v`\n", rep.ProductGateGreen)
	fmt.Fprintf(&b, "- generated_at: `%s`\n", rep.GeneratedAt)
	fmt.Fprintf(&b, "- schema_tables: `%d / %d`\n", rep.Schema.DetectedTableCount, rep.Schema.ExpectedTableCount)
	fmt.Fprintf(&b, "- store_save_insert_coverage: `%d / %d`\n", len(rep.StoreCoverage.SaveTablesWithInsert), len(rep.StoreCoverage.ExpectedSaveTables))
	fmt.Fprintf(&b, "- store_read_select_coverage: `%d / %d`\n", len(rep.StoreCoverage.ReadTablesWithSelect), len(rep.StoreCoverage.ExpectedReadTables))
	fmt.Fprintf(&b, "- import_table_coverage: `%d / %d`\n", len(rep.ToolCoverage.ImportTables)-len(rep.ToolCoverage.ImportMissingTables), len(canonicalTables))
	fmt.Fprintf(&b, "- compare_table_coverage: `%d / %d`\n", len(rep.ToolCoverage.CompareTables)-len(rep.ToolCoverage.CompareMissingTables), len(canonicalTables))
	fmt.Fprintf(&b, "- missing_executors: `%d`\n", len(rep.ToolCoverage.MissingExecutors))
	fmt.Fprintf(&b, "- missing_guard_checks: `%d`\n", len(rep.ToolCoverage.MissingGuardChecks))
	fmt.Fprintf(&b, "- local_provider_detected: `%v`\n\n", rep.ProviderBootstrap.AnyLocalProvider)

	if len(rep.Errors) > 0 {
		fmt.Fprintf(&b, "## Errors\n\n")
		for _, e := range rep.Errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(rep.Warnings) > 0 {
		fmt.Fprintf(&b, "## Warnings\n\n")
		for _, w := range rep.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
		fmt.Fprintf(&b, "\n")
	}
	fmt.Fprintf(&b, "## Blockers Kept Open\n\n")
	for _, blocker := range rep.Blockers {
		fmt.Fprintf(&b, "- %s\n", blocker)
	}
	fmt.Fprintf(&b, "\n## Next Required Action\n\n%s\n", rep.NextRequiredAction)

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(b.String()), 0644)
}

func init() {
	sort.Strings(canonicalTables)
	sort.Strings(storeSaveTables)
	sort.Strings(storeReadTables)
}
