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

func TestSessionMigrationManifestRemainsExplicitlyReleaseBlocked(t *testing.T) {
	direct, indirect, implemented := SessionMigrationManifestSummary()
	if direct != 46 || indirect != 4 {
		t.Fatalf("manifest summary direct=%d indirect=%d, want 46/4", direct, indirect)
	}
	if implemented >= direct+indirect {
		t.Fatalf("manifest unexpectedly reports complete executor: implemented=%d total=%d", implemented, direct+indirect)
	}
	blockers := strings.Join(SessionMigrationManifestReleaseBlockers(), "\n")
	for _, want := range []string{
		SessionMigrationManifestParityUnverifiedReason,
		"source_target_count_hash_unverified",
		"row_map_fk_remap_unverified",
		"vector_expected_id_parity_unverified",
	} {
		if !strings.Contains(blockers, want) {
			t.Errorf("release blockers missing %q: %s", want, blockers)
		}
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
