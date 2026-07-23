package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCanonPackStorageMigrationMariaDBIntegration(t *testing.T) {
	if os.Getenv("AC_CANON_PACK_MIGRATION_TEST_DISPOSABLE") != "YES" {
		t.Skip("requires an explicitly disposable MariaDB")
	}
	dsn := strings.TrimSpace(os.Getenv("AC_CANON_PACK_MIGRATION_TEST_DSN"))
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.DBName, "archive_center_canon_pack_test") {
		t.Fatalf("refusing non-disposable database %q", cfg.DBName)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	for _, statement := range canonPackMigrationPrerequisites() {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("create prerequisite: %v", err)
		}
	}
	seedExistingReferenceRows(t, ctx, db)
	before := existingReferenceFingerprint(t, ctx, db)

	statements, err := loadStatements(canonPackStorageMigrationPath(t))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		report := newReport(canonPackStorageMigrationPath(t), true)
		if err := applyStatements(ctx, db, statements, report); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt+1, err)
		}
	}

	var tableCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name IN ('reference_work_editions','reference_work_titles','canon_pack_installs','reference_source_observations','reference_item_origins','reference_item_evidence','reference_logical_facts','reference_fact_identities','reference_overlay_rules','source_discovery_jobs')
	`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != len(canonPackStorageTables) {
		t.Fatalf("created table count=%d want=%d", tableCount, len(canonPackStorageTables))
	}
	if after := existingReferenceFingerprint(t, ctx, db); after != before {
		t.Fatalf("existing reference rows changed: before=%q after=%q", before, after)
	}

	const workID = "00000000-0000-0000-0000-000000000001"
	const editionID = "00000000-0000-0000-0000-000000000010"
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_work_editions
		(edition_row_id, work_id, stable_work_id, edition_id, edition_label)
		VALUES (?, ?, 'example.work:neutral', 'example.edition:neutral', 'Neutral edition')`, editionID, workID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM reference_works WHERE work_id = ?`, workID); err == nil {
		t.Fatal("edition foreign key did not restrict work deletion")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_work_titles
		(title_row_id, work_id, edition_row_id, title_kind, title_text, normalization_contract,
		 normalized_lookup_key, normalized_lookup_digest)
		VALUES ('00000000-0000-0000-0000-000000000011', ?, ?, 'original', 'Neutral title',
		 'canon_title_normalization.v1', 'neutral title', ?)`, workID, editionID, strings.Repeat("3", 64)); err == nil {
		t.Fatal("edition title without a matching edition scope key was accepted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_work_titles
		(title_row_id, work_id, edition_row_id, edition_scope_key, title_kind, title_text,
		 normalization_contract, normalized_lookup_key, normalized_lookup_digest)
		VALUES ('00000000-0000-0000-0000-000000000012', ?, ?, ?, 'original', 'Neutral title',
		 'canon_title_normalization.v1', 'neutral title', ?)`, workID, editionID, editionID, strings.Repeat("3", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_work_titles
		(title_row_id, work_id, edition_row_id, edition_scope_key, title_kind, title_text,
		 normalization_contract, normalized_lookup_key, normalized_lookup_digest)
		VALUES ('00000000-0000-0000-0000-000000000013', ?, ?, ?, 'original', 'Duplicate title',
		 'canon_title_normalization.v1', 'neutral title', ?)`, workID, editionID, editionID, strings.Repeat("3", 64)); err == nil {
		t.Fatal("duplicate title identity in the same edition was accepted")
	}

	insertInstall := `INSERT INTO canon_pack_installs
		(install_id, pack_id, pack_version, install_generation, manifest_contract, manifest_sha256,
		 manifest_json, work_id, edition_row_id, pack_status, review_status, trust_status,
		 lifecycle_status, validation_report_json, coverage_report_json)
		VALUES (?, 'example.publisher:neutral', ?, ?, 'canon-pack-manifest.v1', ?, '{}', ?, ?,
		 'published', 'approved', 'unsigned_local', 'active', '{}', '{}')`
	hash := strings.Repeat("1", 64)
	if _, err := db.ExecContext(ctx, insertInstall, "00000000-0000-0000-0000-000000000020", "1.0.0", 1, hash, workID, editionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, insertInstall, "00000000-0000-0000-0000-000000000021", "1.1.0", 2, hash, workID, editionID); err == nil {
		t.Fatal("second active generation for the same pack and edition was accepted")
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO reference_item_origins
		(origin_membership_id, work_id, edition_row_id, item_kind, origin_kind, origin_owner_id, source_item_id, review_state)
		VALUES ('00000000-0000-0000-0000-000000000030', ?, ?, 'timeline', 'canon_pack', 'owner', 'item', 'approved')`, workID, editionID); err == nil {
		t.Fatal("item origin without exactly one selected foreign key was accepted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_overlay_rules
		(overlay_rule_id, work_id, edition_row_id, overlay_action, rule_status)
		VALUES ('00000000-0000-0000-0000-000000000040', ?, ?, 'supplement', 'active')`, workID, editionID); err == nil {
		t.Fatal("active overlay without a target was accepted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_overlay_rules
		(overlay_rule_id, work_id, edition_row_id, overlay_action, rule_status)
		VALUES ('00000000-0000-0000-0000-000000000041', ?, ?, 'supplement', 'dormant')`, workID, editionID); err != nil {
		t.Fatalf("dormant overlay without a target was rejected: %v", err)
	}

	const continuityID = "00000000-0000-0000-0000-000000000002"
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_source_observations
		(observation_id, work_id, edition_row_id, continuity_id, origin_kind, source_key,
		 source_type, source_uri, license_json, access_class, retrieved_at, document_sha256)
		VALUES ('00000000-0000-0000-0000-000000000050', ?, ?, ?, 'canon_pack', 'bad-source',
		 'official_primary', 'urn:example:bad', '{}', 'public', NOW(3), ?)`, workID, editionID, continuityID, hash); err == nil {
		t.Fatal("canon pack source observation without install owner was accepted")
	}
	const observationID = "00000000-0000-0000-0000-000000000051"
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_source_observations
		(observation_id, work_id, edition_row_id, continuity_id, origin_kind, install_id, source_key,
		 source_type, source_uri, license_json, access_class, retrieved_at, document_sha256)
		VALUES (?, ?, ?, ?, 'canon_pack', '00000000-0000-0000-0000-000000000020', 'guide',
		 'official_primary', 'urn:example:guide', '{}', 'public', NOW(3), ?)`, observationID, workID, editionID, continuityID, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reference_item_evidence
		(evidence_edge_id, item_kind, source_observation_id, document_hash_contract,
		 document_sha256, locator_json, locator_digest)
		VALUES ('00000000-0000-0000-0000-000000000052', 'entity', ?, 'source_bytes_sha256.v1', ?, '{}', ?)`, observationID, hash, strings.Repeat("2", 64)); err == nil {
		t.Fatal("evidence edge without exactly one selected foreign key was accepted")
	}

}

func TestCanonPackStorageProductionRegistrationMariaDBIntegration(t *testing.T) {
	if os.Getenv("AC_CANON_PACK_MIGRATION_TEST_DISPOSABLE") != "YES" {
		t.Skip("requires an explicitly disposable MariaDB")
	}
	baseDSN := strings.TrimSpace(os.Getenv("AC_CANON_PACK_MIGRATION_TEST_DSN"))
	cfg, err := mysql.ParseDSN(baseDSN)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.DBName, "archive_center_canon_pack_test") {
		t.Fatalf("refusing non-disposable database %q", cfg.DBName)
	}

	productionDBName := cfg.DBName + "_production"
	if strings.Trim(productionDBName, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" {
		t.Fatalf("unsafe disposable database name %q", productionDBName)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	adminDB, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if _, err := adminDB.ExecContext(ctx, "DROP DATABASE IF EXISTS `"+productionDBName+"`"); err != nil {
		t.Fatal(err)
	}
	if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE `"+productionDBName+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, "DROP DATABASE IF EXISTS `"+productionDBName+"`")
	})

	productionCfg := *cfg
	productionCfg.DBName = productionDBName
	productionDSN := productionCfg.FormatDSN()
	schemaPath, ok := findSchemaUp(".", 6)
	if !ok {
		t.Fatal("production 001 schema not found")
	}
	report, exitCode := run(schemaPath, productionDSN, true, 90*time.Second)
	if exitCode != 0 {
		t.Fatalf("fresh production schema failed: %+v", report.Errors)
	}
	if report.CompatibilityStatementsRun != len(compatibilityMigrationStatements()) {
		t.Fatalf("fresh production compatibility statements=%d want=%d", report.CompatibilityStatementsRun, len(compatibilityMigrationStatements()))
	}

	db, err := sql.Open("mysql", productionDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedExistingProductionReferenceRows(t, ctx, db)
	before := existingReferenceFingerprint(t, ctx, db)

	report, exitCode = run(schemaPath, productionDSN, true, 90*time.Second)
	if exitCode != 0 {
		t.Fatalf("retry production schema failed: %+v", report.Errors)
	}
	if after := existingReferenceFingerprint(t, ctx, db); after != before {
		t.Fatalf("production registration changed existing reference rows: before=%q after=%q", before, after)
	}
	var tableCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name IN ('reference_work_editions','reference_work_titles','canon_pack_installs','reference_source_observations','reference_item_origins','reference_item_evidence','reference_logical_facts','reference_fact_identities','reference_overlay_rules','source_discovery_jobs')
	`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != len(canonPackStorageTables) {
		t.Fatalf("production table count=%d want=%d", tableCount, len(canonPackStorageTables))
	}
	verifyCanonPackInstallLifecycle(t, ctx, productionDSN)
}

func verifyCanonPackInstallLifecycle(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()
	verificationDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer verificationDB.Close()
	if _, err := verificationDB.ExecContext(ctx, `INSERT INTO chat_logs (chat_session_id, turn_index, role, content) VALUES ('canon-boundary-sentinel', 1, 'user', 'private chat sentinel')`); err != nil {
		t.Fatal(err)
	}
	if _, err := verificationDB.ExecContext(ctx, `INSERT INTO memories (chat_session_id, turn_index, summary_json, importance) VALUES ('canon-boundary-sentinel', 1, '{"summary":"private memory sentinel"}', 1)`); err != nil {
		t.Fatal(err)
	}
	memoryFingerprint := func() string {
		t.Helper()
		var value string
		if err := verificationDB.QueryRowContext(ctx, `SELECT CONCAT(
			(SELECT COUNT(*) FROM chat_logs WHERE chat_session_id='canon-boundary-sentinel'), ':',
			(SELECT SHA2(content, 256) FROM chat_logs WHERE chat_session_id='canon-boundary-sentinel' LIMIT 1), ':',
			(SELECT COUNT(*) FROM memories WHERE chat_session_id='canon-boundary-sentinel'), ':',
			(SELECT SHA2(CAST(summary_json AS CHAR), 256) FROM memories WHERE chat_session_id='canon-boundary-sentinel' LIMIT 1)
		)`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	boundaryBefore := memoryFingerprint()
	runtimeStore, err := archiveStore.OpenMariaDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if closer, ok := runtimeStore.(interface{ Close() error }); ok {
		defer closer.Close()
	}
	packStore, ok := runtimeStore.(archiveStore.CanonPackStore)
	if !ok {
		t.Fatal("MariaDB store does not implement CanonPackStore")
	}
	referenceStore, ok := runtimeStore.(archiveStore.ReferenceLibraryStore)
	if !ok {
		t.Fatal("MariaDB store does not implement ReferenceLibraryStore")
	}
	registryStore, ok := runtimeStore.(archiveStore.CanonRegistryStore)
	if !ok {
		t.Fatal("MariaDB store does not implement CanonRegistryStore")
	}
	discoveryStore, ok := runtimeStore.(archiveStore.SourceDiscoveryStore)
	if !ok {
		t.Fatal("MariaDB store does not implement SourceDiscoveryStore")
	}

	documentHash := strings.Repeat("b", 64)
	evidence := []any{map[string]any{
		"source_id": "guide", "document_sha256": documentHash,
		"locator": map[string]any{"type": "record", "value": "one"},
	}}
	manifest := map[string]any{
		"contract": "canon-pack-manifest.v1",
		"pack":     map[string]any{"id": "example.publisher:lifecycle", "version": "1.0.0", "status": "published"},
		"work": map[string]any{
			"stable_id": "example.work:lifecycle", "original_language": "en",
			"original_title":    map[string]any{"text": "Lifecycle fixture", "language": "en"},
			"translated_titles": []any{}, "aliases": []any{},
			"edition":        map[string]any{"edition_id": "example.edition:lifecycle", "label": "Lifecycle", "language": "en"},
			"continuity_ids": []any{"example.continuity:main"},
		},
		"review": map[string]any{"status": "approved"},
		"trust":  map[string]any{"status": "unsigned_local"},
		"sources": []any{map[string]any{
			"id": "guide", "title": "Guide", "source_type": "official_primary",
			"uri": "urn:example:lifecycle", "license": map[string]any{"name": "Fixture"},
			"access_class": "open_license", "retrieved_at": "2026-07-18T00:00:00Z",
			"document_sha256": documentHash,
		}},
		"content": map[string]any{
			"entities": []any{map[string]any{
				"id": "person", "name": map[string]any{"text": "Person", "language": "en"},
				"kind": "person", "continuity_ids": []any{"example.continuity:main"},
				"review_state": "approved", "evidence": evidence,
			}},
			"locations": []any{}, "factions": []any{}, "settings": []any{},
			"events": []any{}, "relations": []any{}, "claims": []any{map[string]any{
				"id": "claim-one", "claim_type": "fact", "statement": "Person guards the archive.",
				"subject_ids": []any{"person"}, "continuity_ids": []any{"example.continuity:main"},
				"review_state": "approved", "evidence": evidence,
			}},
		},
		"coverage_report": map[string]any{"saturation": "not_assessed", "domains": []any{}},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := packStore.InstallCanonPack(ctx, archiveStore.CanonPackInstallInput{
		ManifestJSON: manifestJSON, ManifestSHA256: strings.Repeat("c", 64),
		ArchiveSHA256: strings.Repeat("d", 64), ValidationReportJSON: []byte(`{"valid":true}`),
	})
	if err != nil {
		t.Fatalf("install Canon Pack: %v", err)
	}
	if installed.LifecycleStatus != "active" || installed.RecordCounts["entities"] != 1 || installed.RecordCounts["claims"] != 1 {
		t.Fatalf("installed pack=%+v", installed)
	}
	assertApprovedEntityCount := func(want int) {
		t.Helper()
		items, err := referenceStore.ListReferenceEntities(ctx, installed.WorkID, "", "approved")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != want {
			t.Fatalf("approved entity count=%d want=%d lifecycle=%s", len(items), want, installed.LifecycleStatus)
		}
	}
	assertApprovedClaimCount := func(want int) []archiveStore.ReferenceClaim {
		t.Helper()
		items, err := referenceStore.ListReferenceClaims(ctx, installed.WorkID, "", "approved", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != want {
			t.Fatalf("approved claim count=%d want=%d", len(items), want)
		}
		return items
	}
	assertApprovedEntityCount(1)
	assertApprovedClaimCount(1)
	if _, err := packStore.SetCanonPackLifecycle(ctx, installed.InstallID, "deactivate"); err != nil {
		t.Fatal(err)
	}
	assertApprovedEntityCount(0)
	assertApprovedClaimCount(0)
	if _, err := packStore.SetCanonPackLifecycle(ctx, installed.InstallID, "rollback"); err != nil {
		t.Fatal(err)
	}
	assertApprovedEntityCount(1)
	assertApprovedClaimCount(1)

	manifest["pack"].(map[string]any)["version"] = "1.1.0"
	secondManifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := packStore.InstallCanonPack(ctx, archiveStore.CanonPackInstallInput{
		ManifestJSON: secondManifestJSON, ManifestSHA256: strings.Repeat("e", 64),
		ArchiveSHA256: strings.Repeat("f", 64), ValidationReportJSON: []byte(`{"valid":true}`),
	})
	if err != nil {
		t.Fatalf("install second Canon Pack: %v", err)
	}
	if second.RecordCounts["claims"] != 1 {
		t.Fatalf("second pack claim origin count=%#v", second.RecordCounts)
	}
	claims := assertApprovedClaimCount(1)
	var claimRows, factRows, evidenceRows int
	if err := verificationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_claims WHERE work_id=?`, installed.WorkID).Scan(&claimRows); err != nil {
		t.Fatal(err)
	}
	if err := verificationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_fact_identities fi JOIN reference_claims c ON c.claim_id=fi.claim_id WHERE c.work_id=?`, installed.WorkID).Scan(&factRows); err != nil {
		t.Fatal(err)
	}
	if err := verificationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_item_evidence WHERE claim_id=?`, claims[0].ClaimID).Scan(&evidenceRows); err != nil {
		t.Fatal(err)
	}
	if claimRows != 1 || factRows != 1 || evidenceRows != 2 {
		t.Fatalf("exact fact dedup failed claims=%d facts=%d evidence=%d", claimRows, factRows, evidenceRows)
	}
	registryItems, err := registryStore.SearchCanonRegistry(ctx, "Lifecycle fixture", 10)
	if err != nil || len(registryItems) != 2 {
		t.Fatalf("registry items=%#v err=%v", registryItems, err)
	}
	diagnostics, err := registryStore.GetCanonPackDiagnostics(ctx, second.InstallID)
	if err != nil || len(diagnostics.Sources) != 1 || diagnostics.Quality["traceability"] == nil {
		t.Fatalf("diagnostics=%#v err=%v", diagnostics, err)
	}
	discoveryJob, err := discoveryStore.SaveSourceDiscoveryJob(ctx, archiveStore.SourceDiscoveryInput{
		WorkQuery: "Lifecycle fixture", AllowedSourceTypes: []string{"official_primary"},
	}, "insufficient_source_coverage", map[string]any{"admission_status": "pending"}, map[string]any{"saturation": "insufficient_source_coverage"})
	if err != nil || discoveryJob.State != "insufficient_source_coverage" {
		t.Fatalf("discovery job=%#v err=%v", discoveryJob, err)
	}

	if _, err := packStore.SetCanonPackLifecycle(ctx, second.InstallID, "remove"); err != nil {
		t.Fatal(err)
	}
	assertApprovedClaimCount(0)
	if _, err := packStore.SetCanonPackLifecycle(ctx, installed.InstallID, "rollback"); err != nil {
		t.Fatal(err)
	}
	claims = assertApprovedClaimCount(1)
	overlay, err := registryStore.CreateCanonOverlay(ctx, archiveStore.CanonOverlayInput{
		WorkID: installed.WorkID, EditionRowID: installed.EditionRowID,
		TargetKind: "claim", TargetID: claims[0].ClaimID, Action: "suppress_for_retrieval",
		Reason: "local fixture suppression",
	})
	if err != nil || overlay.TargetLogicalFactID == "" {
		t.Fatalf("overlay=%#v err=%v", overlay, err)
	}
	assertApprovedClaimCount(0)
	if _, err := packStore.SetCanonPackLifecycle(ctx, installed.InstallID, "remove"); err != nil {
		t.Fatal(err)
	}
	assertApprovedEntityCount(0)
	if boundaryAfter := memoryFingerprint(); boundaryAfter != boundaryBefore {
		t.Fatalf("Canon Pack work changed main long-term memory: before=%q after=%q", boundaryBefore, boundaryAfter)
	}
}

func canonPackMigrationPrerequisites() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS reference_works (work_id CHAR(36) PRIMARY KEY, title VARCHAR(500) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS reference_continuities (continuity_id CHAR(36) PRIMARY KEY, work_id CHAR(36) NOT NULL, label VARCHAR(500) NOT NULL, CONSTRAINT fk_test_continuity_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS reference_documents (document_id CHAR(36) PRIMARY KEY, work_id CHAR(36) NOT NULL, continuity_id CHAR(36) NOT NULL, content_hash CHAR(64) NOT NULL, raw_text LONGTEXT NULL, CONSTRAINT fk_test_document_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE, CONSTRAINT fk_test_document_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS reference_timeline_nodes (node_id CHAR(36) PRIMARY KEY, work_id CHAR(36) NOT NULL, continuity_id CHAR(36) NOT NULL, label VARCHAR(500) NOT NULL, CONSTRAINT fk_test_node_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE, CONSTRAINT fk_test_node_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS reference_entities (entity_id CHAR(36) PRIMARY KEY, work_id CHAR(36) NOT NULL, continuity_id CHAR(36) NOT NULL, canonical_name VARCHAR(500) NOT NULL, CONSTRAINT fk_test_entity_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE, CONSTRAINT fk_test_entity_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS reference_claims (claim_id CHAR(36) PRIMARY KEY, work_id CHAR(36) NOT NULL, continuity_id CHAR(36) NOT NULL, document_id CHAR(36) NOT NULL, claim_text LONGTEXT NOT NULL, CONSTRAINT fk_test_claim_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE, CONSTRAINT fk_test_claim_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE, CONSTRAINT fk_test_claim_document FOREIGN KEY (document_id) REFERENCES reference_documents(document_id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
}

func seedExistingReferenceRows(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO reference_works (work_id, title) VALUES ('00000000-0000-0000-0000-000000000001', 'Existing neutral work')`,
		`INSERT INTO reference_continuities (continuity_id, work_id, label) VALUES ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Existing continuity')`,
		`INSERT INTO reference_documents (document_id, work_id, continuity_id, content_hash, raw_text) VALUES ('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', ?, 'existing private fixture text')`,
		`INSERT INTO reference_timeline_nodes (node_id, work_id, continuity_id, label) VALUES ('00000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'Existing node')`,
		`INSERT INTO reference_entities (entity_id, work_id, continuity_id, canonical_name) VALUES ('00000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'Existing entity')`,
		`INSERT INTO reference_claims (claim_id, work_id, continuity_id, document_id, claim_text) VALUES ('00000000-0000-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000003', 'Existing claim')`,
	}
	for i, statement := range statements {
		var err error
		if i == 2 {
			_, err = db.ExecContext(ctx, statement, strings.Repeat("a", 64))
		} else {
			_, err = db.ExecContext(ctx, statement)
		}
		if err != nil {
			t.Fatalf("seed statement %d: %v", i+1, err)
		}
	}
}

func seedExistingProductionReferenceRows(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO reference_works (work_id, title) VALUES ('00000000-0000-0000-0000-000000000001', 'Existing neutral work')`,
		`INSERT INTO reference_continuities (continuity_id, work_id, continuity_key, label) VALUES ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'existing-neutral', 'Existing continuity')`,
		`INSERT INTO reference_documents (document_id, work_id, continuity_id, content_hash, raw_text) VALUES ('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', ?, 'existing private fixture text')`,
		`INSERT INTO reference_timeline_nodes (node_id, work_id, continuity_id, node_key, label) VALUES ('00000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'existing-node', 'Existing node')`,
		`INSERT INTO reference_entities (entity_id, work_id, continuity_id, entity_type, canonical_name) VALUES ('00000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'person', 'Existing entity')`,
		`INSERT INTO reference_claims (claim_id, work_id, continuity_id, document_id, claim_type, claim_text) VALUES ('00000000-0000-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000003', 'fact', 'Existing claim')`,
	}
	for i, statement := range statements {
		var err error
		if i == 2 {
			_, err = db.ExecContext(ctx, statement, strings.Repeat("a", 64))
		} else {
			_, err = db.ExecContext(ctx, statement)
		}
		if err != nil {
			t.Fatalf("seed production statement %d: %v", i+1, err)
		}
	}
}

func existingReferenceFingerprint(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	var fingerprint string
	err := db.QueryRowContext(ctx, `SELECT CONCAT(
		(SELECT COUNT(*) FROM reference_works), ':',
		(SELECT COUNT(*) FROM reference_continuities), ':',
		(SELECT COUNT(*) FROM reference_documents), ':',
		(SELECT COUNT(*) FROM reference_timeline_nodes), ':',
		(SELECT COUNT(*) FROM reference_entities), ':',
		(SELECT COUNT(*) FROM reference_claims), ':',
		(SELECT SHA2(CONCAT(title, '|', COALESCE((SELECT raw_text FROM reference_documents LIMIT 1), ''), '|', COALESCE((SELECT claim_text FROM reference_claims LIMIT 1), '')), 256) FROM reference_works LIMIT 1)
	)`).Scan(&fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}
