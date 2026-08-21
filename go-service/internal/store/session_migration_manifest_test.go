package store

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestSessionMigrationManifestMatchesAllDirectSchemaTables(t *testing.T) {
	raw, err := os.ReadFile("../../../migrations/001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	createRE := regexp.MustCompile(`(?is)CREATE TABLE IF NOT EXISTS\s+` + "`?" + `([a-z0-9_]+)` + "`?" + `\s*\((.*?)\)\s*(?:ENGINE|COMMENT|;)`)
	sessionColumnRE := regexp.MustCompile(`(?im)^\s*` + "`?" + `([a-z0-9_]*session_id)` + "`?" + `\s+`)
	metadataTables := map[string]bool{
		"session_migrations":      true,
		"session_migration_locks": true,
		"session_route_bindings":  true,
	}
	schemaTables := map[string][]string{}
	for _, match := range createRE.FindAllSubmatch(raw, -1) {
		columns := sessionColumnRE.FindAllSubmatch(match[2], -1)
		table := string(match[1])
		if len(columns) == 0 || metadataTables[table] {
			continue
		}
		blockColumns := make([]string, 0, len(columns))
		for _, column := range columns {
			blockColumns = append(blockColumns, string(column[1]))
		}
		if previous, duplicateDefinition := schemaTables[table]; duplicateDefinition {
			if strings.Join(previous, ",") != strings.Join(blockColumns, ",") {
				t.Fatalf("duplicate schema definition for %s has session-column drift: %v vs %v", table, previous, blockColumns)
			}
			continue
		}
		schemaTables[table] = blockColumns
	}
	if len(schemaTables) != 46 {
		t.Fatalf("direct schema table count = %d, want 46: %v", len(schemaTables), sortedManifestKeys(schemaTables))
	}

	manifestTables := map[string][]string{}
	for _, entry := range SessionMigrationManifest() {
		if !entry.Direct {
			continue
		}
		if _, duplicate := manifestTables[entry.Table]; duplicate {
			t.Fatalf("duplicate direct manifest entry %q", entry.Table)
		}
		columns := []string{entry.SessionColumn}
		for _, related := range entry.RelatedSessionColumns {
			if strings.TrimSpace(related.Semantics) == "" {
				t.Errorf("%s related column %s has no remap/retention semantics", entry.Table, related.Column)
			}
			columns = append(columns, related.Column)
		}
		manifestTables[entry.Table] = columns
	}
	if len(manifestTables) != 46 {
		t.Fatalf("direct manifest table count = %d, want 46", len(manifestTables))
	}
	for table, columns := range schemaTables {
		if strings.Join(manifestTables[table], ",") != strings.Join(columns, ",") {
			t.Errorf("manifest mapping %s = %q, want %q", table, manifestTables[table], columns)
		}
	}
	for table := range manifestTables {
		if _, ok := schemaTables[table]; !ok {
			t.Errorf("manifest has non-schema direct table %q", table)
		}
	}
}

func TestSessionMigrationManifestExplicitlyExcludesRoutingAndMigrationMetadata(t *testing.T) {
	exclusions := map[string]SessionMigrationMetadataExclusion{}
	for _, exclusion := range SessionMigrationMetadataExclusions() {
		if exclusion.Semantics == "" {
			t.Errorf("metadata exclusion %s has no semantics", exclusion.Table)
		}
		exclusions[exclusion.Table] = exclusion
	}
	want := map[string][]string{
		"session_migrations":      {"source_session_id", "target_session_id"},
		"session_migration_locks": {"source_session_id", "target_session_id"},
		"session_route_bindings":  {"canonical_session_id", "redirected_from_session_id"},
	}
	for table, columns := range want {
		exclusion, ok := exclusions[table]
		if !ok {
			t.Errorf("missing metadata exclusion %s", table)
			continue
		}
		if strings.Join(exclusion.SessionColumns, ",") != strings.Join(columns, ",") {
			t.Errorf("%s metadata columns = %v, want %v", table, exclusion.SessionColumns, columns)
		}
	}
	raw007, err := os.ReadFile("../../../migrations/007_session_migration_manifest_and_route_binding.sql")
	if err != nil {
		t.Fatal(err)
	}
	routeBlockRE := regexp.MustCompile(`(?is)CREATE TABLE IF NOT EXISTS\s+session_route_bindings\s*\((.*?)\)\s*ENGINE`)
	match := routeBlockRE.FindSubmatch(raw007)
	if len(match) != 2 {
		t.Fatal("007 session_route_bindings definition not found")
	}
	sessionColumnRE := regexp.MustCompile(`(?im)^\s*` + "`?" + `([a-z0-9_]*session_id)` + "`?" + `\s+`)
	var observed []string
	for _, column := range sessionColumnRE.FindAllSubmatch(match[1], -1) {
		observed = append(observed, string(column[1]))
	}
	if strings.Join(observed, ",") != strings.Join(want["session_route_bindings"], ",") {
		t.Fatalf("007 route binding session columns = %v, want explicit exclusion %v", observed, want["session_route_bindings"])
	}
}

func TestSessionMigrationLedgerTablesMatchFreshAndAdditiveSchemas(t *testing.T) {
	raw001, err := os.ReadFile("../../../migrations/001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	raw007, err := os.ReadFile("../../../migrations/007_session_migration_manifest_and_route_binding.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{
		"session_route_bindings",
		"session_migration_artifact_parity",
		"session_migration_vector_expected_ids",
		"session_migration_saga_steps",
	} {
		blockRE := regexp.MustCompile(`(?is)CREATE TABLE IF NOT EXISTS\s+` + regexp.QuoteMeta(table) + `\s*\((.*?)\)\s*ENGINE=InnoDB[^;]+;`)
		fresh := blockRE.Find(raw001)
		additive := blockRE.Find(raw007)
		if len(fresh) == 0 || len(additive) == 0 {
			t.Fatalf("%s missing fresh=%t additive=%t", table, len(fresh) > 0, len(additive) > 0)
		}
		normalize := func(in []byte) string {
			return strings.Join(strings.Fields(string(in)), " ")
		}
		if normalize(fresh) != normalize(additive) {
			t.Errorf("%s definition drifted between 001 and 007", table)
		}
	}
	rowMapRE := regexp.MustCompile(`(?is)CREATE TABLE IF NOT EXISTS\s+session_migration_artifact_row_map\s*\((.*?)\)\s*ENGINE=InnoDB[^;]+;`)
	if len(rowMapRE.Find(raw001)) == 0 {
		t.Fatal("session_migration_artifact_row_map missing from canonical fresh/compatibility schema")
	}
}

func TestSessionMigrationManifestClassifiesIndirectChildrenAndPolicies(t *testing.T) {
	wantIndirect := map[string]string{
		"persona_memory_entries":               "persona_memory_capsules",
		"session_reference_runtime":            "session_reference_bindings",
		"session_reference_coverage_snapshots": "session_reference_bindings",
		"session_reference_coverage_fields":    "session_reference_coverage_snapshots",
	}
	allowedPolicies := map[string]bool{
		SessionMigrationPolicyCopy:                true,
		SessionMigrationPolicyRetainAudit:         true,
		SessionMigrationPolicyRegenerate:          true,
		SessionMigrationPolicyDeleteAfterVerified: true,
	}
	seenPolicies := map[string]bool{}
	for _, entry := range SessionMigrationManifest() {
		if !allowedPolicies[entry.Policy] {
			t.Errorf("%s has unsupported policy %q", entry.Table, entry.Policy)
		}
		seenPolicies[entry.Policy] = true
		if entry.Direct {
			if strings.TrimSpace(entry.SessionColumn) == "" || entry.ParentTable != "" {
				t.Errorf("invalid direct manifest entry %+v", entry)
			}
			continue
		}
		if wantIndirect[entry.Table] != entry.ParentTable {
			t.Errorf("indirect manifest entry %s parent = %q, want %q", entry.Table, entry.ParentTable, wantIndirect[entry.Table])
		}
		delete(wantIndirect, entry.Table)
	}
	if len(wantIndirect) != 0 {
		t.Fatalf("missing indirect children: %v", sortedManifestKeys(wantIndirect))
	}
	for _, policy := range []string{
		SessionMigrationPolicyCopy,
		SessionMigrationPolicyRetainAudit,
		SessionMigrationPolicyRegenerate,
		SessionMigrationPolicyDeleteAfterVerified,
	} {
		if !seenPolicies[policy] {
			t.Errorf("manifest does not exercise policy %q", policy)
		}
	}
}

func TestSessionMigrationManifestHasExecutablePlanForEveryEntry(t *testing.T) {
	direct, indirect, implemented := SessionMigrationManifestSummary()
	if direct != 46 || indirect != 4 {
		t.Fatalf("manifest summary direct=%d indirect=%d, want 46/4", direct, indirect)
	}
	if implemented != direct+indirect {
		t.Fatalf("manifest executor implemented=%d total=%d", implemented, direct+indirect)
	}
	if blockers := SessionMigrationManifestReleaseBlockers(); len(blockers) != 0 {
		t.Fatalf("static manifest release blockers = %v, want none", blockers)
	}
	plans := SessionMigrationExecutionPlans()
	if len(plans) != direct+indirect {
		t.Fatalf("execution plans=%d, want %d", len(plans), direct+indirect)
	}
	for _, entry := range SessionMigrationManifest() {
		if !entry.Implemented {
			t.Errorf("%s is not implemented", entry.Table)
		}
		plan, ok := SessionMigrationExecutionPlanFor(entry.Table)
		if !ok {
			t.Errorf("%s has no execution plan", entry.Table)
			continue
		}
		columnSet := map[string]bool{}
		for _, column := range plan.Columns {
			columnSet[column] = true
		}
		if entry.Direct && !columnSet[entry.SessionColumn] {
			t.Errorf("%s plan omits session column %s", entry.Table, entry.SessionColumn)
		}
		for _, fk := range plan.ForeignKeys {
			if !columnSet[fk.Column] {
				t.Errorf("%s FK plan omits column %s", entry.Table, fk.Column)
			}
		}
	}
}

func TestSessionMigrationExecutionPlanColumnsAndPrimaryKeysMatchFreshSchema(t *testing.T) {
	raw, err := os.ReadFile("../../../migrations/001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	createRE := regexp.MustCompile(`(?is)CREATE TABLE IF NOT EXISTS\s+` + "`?" + `([a-z0-9_]+)` + "`?" + `\s*\((.*?)\)\s*(?:ENGINE|COMMENT|;)`)
	columnRE := regexp.MustCompile(`(?im)^\s*` + "`?" + `([a-z_][a-z0-9_]*)` + "`?" + `\s+(?:BIGINT|INT|TINYINT|DECIMAL|DOUBLE|FLOAT|BOOLEAN|CHAR|VARCHAR|TEXT|LONGTEXT|MEDIUMTEXT|JSON|DATETIME|TIMESTAMP|DATE|BLOB|LONGBLOB|ENUM)\b`)
	inlinePrimaryRE := regexp.MustCompile(`(?im)^\s*` + "`?" + `([a-z_][a-z0-9_]*)` + "`?" + `\s+[^\r\n,]*\bPRIMARY\s+KEY\b`)
	tablePrimaryRE := regexp.MustCompile(`(?im)^\s*PRIMARY\s+KEY\s*\(([^)]+)\)`)
	generatedRE := regexp.MustCompile(`(?im)^\s*` + "`?" + `([a-z_][a-z0-9_]*)` + "`?" + `\s+[^\r\n,]*(?:\r?\n\s*)?GENERATED\s+ALWAYS\b`)
	schema := map[string]struct {
		columns   []string
		primary   []string
		generated []string
	}{}
	for _, match := range createRE.FindAllSubmatch(raw, -1) {
		table := string(match[1])
		if _, tracked := sessionMigrationExecutionPlansV1[table]; !tracked {
			continue
		}
		block := match[2]
		columns := []string{}
		for _, column := range columnRE.FindAllSubmatch(block, -1) {
			columns = append(columns, string(column[1]))
		}
		primary := []string{}
		if tablePrimary := tablePrimaryRE.FindSubmatch(block); len(tablePrimary) == 2 {
			for _, column := range strings.Split(string(tablePrimary[1]), ",") {
				primary = append(primary, strings.Trim(strings.TrimSpace(column), "`"))
			}
		} else if inlinePrimary := inlinePrimaryRE.FindSubmatch(block); len(inlinePrimary) == 2 {
			primary = append(primary, string(inlinePrimary[1]))
		}
		generated := []string{}
		for _, column := range generatedRE.FindAllSubmatch(block, -1) {
			generated = append(generated, string(column[1]))
		}
		if previous, duplicate := schema[table]; duplicate {
			if strings.Join(previous.columns, ",") != strings.Join(columns, ",") ||
				strings.Join(previous.primary, ",") != strings.Join(primary, ",") ||
				strings.Join(previous.generated, ",") != strings.Join(generated, ",") {
				t.Fatalf("duplicate schema definition drift for %s", table)
			}
			continue
		}
		schema[table] = struct {
			columns   []string
			primary   []string
			generated []string
		}{columns: columns, primary: primary, generated: generated}
	}
	for _, plan := range SessionMigrationExecutionPlans() {
		actual, ok := schema[plan.Table]
		if !ok {
			t.Errorf("%s schema definition missing", plan.Table)
			continue
		}
		if strings.Join(actual.columns, ",") != strings.Join(plan.Columns, ",") {
			t.Errorf("%s columns\nschema=%s\nplan=%s", plan.Table, strings.Join(actual.columns, ","), strings.Join(plan.Columns, ","))
		}
		if strings.Join(actual.primary, ",") != strings.Join(plan.PrimaryKey, ",") {
			t.Errorf("%s primary key schema=%v plan=%v", plan.Table, actual.primary, plan.PrimaryKey)
		}
		if strings.Join(actual.generated, ",") != strings.Join(plan.DatabaseGenerated, ",") {
			t.Errorf("%s generated columns schema=%v plan=%v", plan.Table, actual.generated, plan.DatabaseGenerated)
		}
	}
}

func TestSessionMigrationManifestV3IncludesWorldlineSourceRole(t *testing.T) {
	if SessionMigrationManifestVersion != "session-migration.manifest.v3" {
		t.Fatalf("manifest version=%q, want v3 after fork source role column change", SessionMigrationManifestVersion)
	}
	plan, ok := SessionMigrationExecutionPlanFor("session_fork_lineage")
	if !ok {
		t.Fatal("session_fork_lineage execution plan missing")
	}
	columns := map[string]bool{}
	for _, column := range plan.Columns {
		columns[column] = true
	}
	for _, column := range []string{
		"contract_version", "lineage_state", "fork_turn", "fork_source_message_id", "fork_source_role", "idempotency_key",
	} {
		if !columns[column] {
			t.Errorf("session_fork_lineage plan missing %s", column)
		}
	}
	raw011, err := os.ReadFile("../../../migrations/011_session_fork_lineage_worldline.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{
		"ADD COLUMN IF NOT EXISTS contract_version",
		"ADD COLUMN IF NOT EXISTS lineage_state",
		"ON session_fork_lineage (chat_session_id, idempotency_key)",
	} {
		if !strings.Contains(string(raw011), token) {
			t.Errorf("011 migration missing %q", token)
		}
	}
	raw012, err := os.ReadFile("../../../migrations/012_session_fork_lineage_source_role.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw012), "ADD COLUMN IF NOT EXISTS fork_source_role VARCHAR(16) NULL") {
		t.Error("012 migration missing nullable fork_source_role column")
	}
}

func TestSessionMigrationVectorPlansCoverCanonicalManagedTiers(t *testing.T) {
	want := map[string]string{
		"memories":                "memory",
		"direct_evidence_records": "evidence",
		"world_rules":             "world_rule",
		"kg_triples":              "kg_triple",
		"episode_summaries":       "episode",
		"chapter_summaries":       "chapter",
		"arc_summaries":           "arc",
		"saga_digests":            "saga",
		"precise_memory_units":    "precise_memory",
	}
	for table, tier := range want {
		plan, ok := SessionMigrationExecutionPlanFor(table)
		if !ok || plan.Vector == nil {
			t.Errorf("%s vector plan missing", table)
			continue
		}
		wantIDColumn := plan.PrimaryKey[0]
		if table == "precise_memory_units" {
			wantIDColumn = "unit_id"
		}
		if plan.Vector.Tier != tier || plan.Vector.IDColumn != wantIDColumn {
			t.Errorf("%s vector plan tier/id = %s/%s, want %s/%s", table,
				plan.Vector.Tier, plan.Vector.IDColumn, tier, wantIDColumn)
		}
	}
	for _, plan := range SessionMigrationExecutionPlans() {
		if plan.Vector != nil {
			delete(want, plan.Table)
		}
	}
	if len(want) != 0 {
		t.Fatalf("managed vector plans missing: %v", sortedManifestKeys(want))
	}
}

func TestSessionMigrationVectorPlansExposeOnlyExactTurnAnchors(t *testing.T) {
	want := map[string][]string{
		"memories":                {"turn_index"},
		"direct_evidence_records": {"turn_anchor", "source_turn_end"},
		"world_rules":             {"source_turn"},
		"kg_triples":              {"source_turn"},
		"precise_memory_units":    {"source_turn_end"},
	}
	for _, plan := range SessionMigrationExecutionPlans() {
		if plan.Vector == nil {
			continue
		}
		if got, ok := want[plan.Table]; ok {
			if strings.Join(plan.Vector.ContextTurnColumns, ",") != strings.Join(got, ",") {
				t.Errorf("%s context turn columns = %v, want %v", plan.Table, plan.Vector.ContextTurnColumns, got)
			}
			delete(want, plan.Table)
			continue
		}
		if len(plan.Vector.ContextTurnColumns) != 0 {
			t.Errorf("hierarchy/unanchored vector plan %s unexpectedly has turn context %v", plan.Table, plan.Vector.ContextTurnColumns)
		}
	}
	if len(want) != 0 {
		t.Fatalf("turn-anchored vector plans missing: %v", sortedManifestKeys(want))
	}
}

func sortedManifestKeys[V any](in map[string]V) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
