package packageupdate

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestApplyPendingNoPendingIsNoOp(t *testing.T) {
	root := t.TempDir()
	result, err := ApplyPending(root)
	if err != nil || result.Status != "no_pending" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".updates")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no-pending apply created update state: %v", err)
	}
}

func TestApplyPendingValidPackage(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new", "scripts/start.ps1": "start"}, nil)
	result, err := ApplyPending(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "applied_pending_health" || !result.HealthRequired {
		t.Fatalf("unexpected result: %+v", result)
	}
	assertFile(t, filepath.Join(root, "bin/app.exe"), "new")
	assertFile(t, filepath.Join(root, "scripts/start.ps1"), "start")
	state := mustState(t, root)
	if state.Status != "applied_pending_health" || len(state.Journal) != 3 {
		t.Fatalf("unexpected state: %+v", state)
	}
	if _, err := os.Stat(filepath.Join(root, ".updates", "pending-update.json")); err != nil {
		t.Fatalf("pending cleared before health commit: %v", err)
	}
}

func TestApplyPendingUpdatesManagedTemplatesAndPreservesDatabaseRuntimeAndSecrets(t *testing.T) {
	root := newFixture(t,
		map[string]string{
			".env.full.example":   "FULL=old",
			".env.source.example": "SOURCE=old",
			"bin/app.exe":         "old",
		},
		map[string]string{
			".env.full.example":   "FULL=new",
			".env.source.example": "SOURCE=new",
			"bin/app.exe":         "new",
		}, nil)
	mustWrite(t, filepath.Join(root, ".env.full.local"), "SECRET=keep")
	mustWrite(t, filepath.Join(root, "data/mariadb-data/sentinel.txt"), "mariadb=keep")
	mustWrite(t, filepath.Join(root, "data/chromadb-data/sentinel.txt"), "chromadb=keep")
	mustWrite(t, filepath.Join(root, ".runtime/sentinel.txt"), "runtime=keep")
	mustWrite(t, filepath.Join(root, "secrets/provider.txt"), "provider=keep")

	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertFile(t, filepath.Join(root, ".env.full.example"), "FULL=new")
	assertFile(t, filepath.Join(root, ".env.source.example"), "SOURCE=new")
	assertFile(t, filepath.Join(root, ".env.full.local"), "SECRET=keep")
	assertFile(t, filepath.Join(root, "data/mariadb-data/sentinel.txt"), "mariadb=keep")
	assertFile(t, filepath.Join(root, "data/chromadb-data/sentinel.txt"), "chromadb=keep")
	assertFile(t, filepath.Join(root, ".runtime/sentinel.txt"), "runtime=keep")
	assertFile(t, filepath.Join(root, "secrets/provider.txt"), "provider=keep")

	if _, err := Rollback(root); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(root, "data/mariadb-data/sentinel.txt"), "mariadb=keep")
	assertFile(t, filepath.Join(root, "data/chromadb-data/sentinel.txt"), "chromadb=keep")
	assertFile(t, filepath.Join(root, ".runtime/sentinel.txt"), "runtime=keep")
	assertFile(t, filepath.Join(root, "secrets/provider.txt"), "provider=keep")
}

func TestApplyPendingRejectsProtectedDataSecretAndEnvironmentFiles(t *testing.T) {
	for _, rel := range []string{
		"data/mariadb-data/value",
		".runtime/chromadb-data/value",
		"mariadb/value",
		"chromadb/value",
		"secrets/provider.txt",
		"database/archive.db",
		".env",
		".env.local",
		".env.full.local",
		".env.full.local.protected",
		".env.example.key",
		".env.other.example",
		"config/.env.full.example",
	} {
		t.Run(strings.ReplaceAll(rel, "/", "_"), func(t *testing.T) {
			root := newFixture(t,
				map[string]string{"bin/app.exe": "old"},
				map[string]string{"bin/app.exe": "new", rel: "forbidden"}, nil)
			_, err := ApplyPending(root)
			assertUpdateCode(t, err, "pending_invalid")
			assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
			if _, statErr := os.Stat(filepath.Join(root, ".updates", "update-state.json")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("state written before protected environment rejection: %v", statErr)
			}
		})
	}
}

func TestApplyPendingAcceptsUTF8BOMCandidateManifest(t *testing.T) {
	root := newFixtureWithManifestPrefixes(t,
		map[string]string{"bin/app.exe": "old"},
		map[string]string{"bin/app.exe": "new"},
		nil, []byte{0xef, 0xbb, 0xbf})
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertFile(t, filepath.Join(root, "bin/app.exe"), "new")
}

func TestApplyPendingAcceptsUTF8BOMCurrentManifest(t *testing.T) {
	root := newFixtureWithManifestPrefixes(t,
		map[string]string{"bin/app.exe": "old"},
		map[string]string{"bin/app.exe": "new"},
		[]byte{0xef, 0xbb, 0xbf}, nil)
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertFile(t, filepath.Join(root, "bin/app.exe"), "new")
}

func TestApplyPendingBindsCandidateAndInstalledPackageVersions(t *testing.T) {
	t.Run("candidate", func(t *testing.T) {
		root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, nil)
		pending := mustPending(t, root)
		pending.TargetVersion = "3"
		writeJSON(t, filepath.Join(root, ".updates", "pending-update.json"), pending)
		_, err := ApplyPending(root)
		assertUpdateCode(t, err, "package_manifest_invalid")
		assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
	})

	t.Run("installed", func(t *testing.T) {
		root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, nil)
		writeManifest(t, filepath.Join(root, ManifestName), "9", map[string]string{"bin/app.exe": "old"})
		_, err := ApplyPending(root)
		assertUpdateCode(t, err, "managed_target_modified")
		assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
	})
}

func TestApplyPendingRejectsCorruptedManifestPrefix(t *testing.T) {
	root := newFixtureWithManifestPrefixes(t,
		map[string]string{"bin/app.exe": "old"},
		map[string]string{"bin/app.exe": "new"},
		nil, []byte{0xef, 0xbb, 0x00})
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "package_manifest_invalid")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
}

func TestApplyPendingResumesExistingPendingHealthWithoutReapplying(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, nil)
	if _, err := ApplyPending(root); err != nil {
		t.Fatal(err)
	}
	result, err := applyPending(root, "", func(_ string, _ int) error {
		t.Fatal("pending-health resume must not apply files again")
		return nil
	})
	if err != nil || result.Status != "applied_pending_health" || !result.HealthRequired {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertFile(t, filepath.Join(root, "bin/app.exe"), "new")
}

func TestApplyPendingSHA256MismatchDoesNotMutatePackage(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, nil)
	p := mustPending(t, root)
	p.SHA256 = strings.Repeat("0", 64)
	writeJSON(t, filepath.Join(root, ".updates", "pending-update.json"), p)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "asset_verification_failed")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
	if _, statErr := os.Stat(filepath.Join(root, ".updates", "update-state.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("state written before asset verification: %v", statErr)
	}
}

func TestApplyPendingRejectsUnsafeZipEntries(t *testing.T) {
	cases := map[string]zipEntry{
		"traversal": {name: "../outside", body: "x"},
		"absolute":  {name: "/absolute", body: "x"},
		"drive":     {name: `C:\\outside`, body: "x"},
		"unc":       {name: `\\\\server\\share`, body: "x"},
		"nul":       {name: "release/bad\x00name", body: "x"},
		"symlink":   {name: "release/link", body: "target", mode: os.ModeSymlink | 0o777},
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, []zipEntry{bad})
			_, err := ApplyPending(root)
			assertUpdateCode(t, err, "archive_rejected")
			assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
		})
	}
}

func TestApplyPendingEnforcesZipLimits(t *testing.T) {
	oldLimits := archiveLimits
	t.Cleanup(func() { archiveLimits = oldLimits })
	t.Run("file_count", func(t *testing.T) {
		archiveLimits = extractionLimits{Files: 2, Bytes: 1024}
		root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, []zipEntry{{name: "release/extra", body: "x"}})
		_, err := ApplyPending(root)
		assertUpdateCode(t, err, "archive_rejected")
		assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
	})
	t.Run("expanded_size", func(t *testing.T) {
		archiveLimits = extractionLimits{Files: 20, Bytes: 8}
		root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "payload larger than limit"}, nil)
		_, err := ApplyPending(root)
		assertUpdateCode(t, err, "archive_rejected")
		assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
	})
}

func TestApplyPendingRejectsModifiedManagedTargetBeforeMutation(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "old", "scripts/start.ps1": "old-start"}, map[string]string{"bin/app.exe": "new", "scripts/start.ps1": "new-start"}, nil)
	if err := os.WriteFile(filepath.Join(root, "scripts/start.ps1"), []byte("user-modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "managed_target_modified")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
	assertFile(t, filepath.Join(root, "scripts/start.ps1"), "user-modified")
}

func TestApplyPendingRejectsManagedFileRemovalWithoutDeleting(t *testing.T) {
	root := newFixture(t,
		map[string]string{"bin/app.exe": "old", "scripts/legacy.ps1": "legacy"},
		map[string]string{"bin/app.exe": "new"}, nil)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "managed_file_removal_unsupported")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
	assertFile(t, filepath.Join(root, "scripts/legacy.ps1"), "legacy")
	if _, statErr := os.Stat(filepath.Join(root, ".updates", "update-state.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("state written before removal-policy rejection: %v", statErr)
	}
}

func TestApplyPendingAppliesAuthenticatedManagedRemovalAndRollbackRestoresIt(t *testing.T) {
	current := map[string]string{
		"bin/app.exe":               "old",
		"migrations/001_schema.sql": "old schema",
		"scripts/legacy.ps1":        "legacy",
	}
	next := map[string]string{
		"bin/app.exe":               "new",
		"migrations/001_schema.sql": "new schema",
	}
	addMigrationUpdateContractWithRemovals(t, current, next, "1", "2", []string{"scripts/legacy.ps1"})
	root := newFixture(t, current, next, nil)
	if _, err := ApplyPending(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "scripts", "legacy.ps1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("authenticated managed removal was not applied: %v", err)
	}
	if _, err := Rollback(root); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(root, "scripts", "legacy.ps1"), "legacy")
	assertFile(t, filepath.Join(root, "bin", "app.exe"), "old")
}

func TestApplyPendingRejectsRemovalNotNamedByAuthenticatedSource(t *testing.T) {
	current := map[string]string{
		"bin/app.exe":               "old",
		"migrations/001_schema.sql": "old schema",
		"scripts/legacy.ps1":        "legacy",
	}
	next := map[string]string{
		"bin/app.exe":               "new",
		"migrations/001_schema.sql": "new schema",
	}
	addMigrationUpdateContract(t, current, next, "1", "2")
	root := newFixture(t, current, next, nil)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "managed_file_removal_unsupported")
	assertFile(t, filepath.Join(root, "scripts", "legacy.ps1"), "legacy")
	assertFile(t, filepath.Join(root, "bin", "app.exe"), "old")
}

func TestApplyFailureRollsBackEveryTouchedFile(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/a.exe": "old-a", "bin/b.exe": "old-b"}, map[string]string{"bin/a.exe": "new-a", "bin/b.exe": "new-b"}, nil)
	_, err := applyPending(root, "", func(_ string, index int) error {
		if index == 1 {
			return errors.New("injected write failure")
		}
		return nil
	})
	assertUpdateCode(t, err, "apply_failed")
	assertFile(t, filepath.Join(root, "bin/a.exe"), "old-a")
	assertFile(t, filepath.Join(root, "bin/b.exe"), "old-b")
	if state := mustState(t, root); state.Status != "rolled_back" {
		t.Fatalf("state=%+v", state)
	}
}

func TestApplyRecoversInterruptedJournalBeforeRetry(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, nil)
	backupRel := "backups/interrupted/bin/app.exe"
	backupAbs := filepath.Join(root, ".updates", filepath.FromSlash(backupRel))
	mustWrite(t, backupAbs, "old")
	mustWrite(t, filepath.Join(root, "bin/app.exe"), "partially-applied")
	writeJSON(t, filepath.Join(root, ".updates", "update-state.json"), State{ContractVersion: StateContract, Status: "applying", CurrentVersion: "1", TargetVersion: "2", BackupDir: "backups/interrupted", Journal: []JournalEntry{{Path: "bin/app.exe", Existed: true, BackupPath: backupRel}}, UpdatedAt: now()})
	result, err := ApplyPending(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v", result)
	}
	assertFile(t, filepath.Join(root, "bin/app.exe"), "new")
}

func TestStatusRecoversAtomicPreviousStateFile(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, ".updates", "update-state.json")
	writeJSON(t, statePath+".previous", State{ContractVersion: StateContract, Status: "rolled_back", CurrentVersion: "1", UpdatedAt: now()})
	result, err := Status(root)
	if err != nil || result.Status != "rolled_back" || result.CurrentVersion != "1" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestApplyPendingDoesNotReapplyWhileHealthIsPending(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, nil)
	if _, err := ApplyPending(root); err != nil {
		t.Fatal(err)
	}
	// This marker is deliberately outside the managed manifest. A second call
	// must report the durable pending-health state rather than start a new journal.
	mustWrite(t, filepath.Join(root, "operator-marker.txt"), "keep")
	firstState := mustState(t, root)
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	secondState := mustState(t, root)
	if firstState.BackupDir != secondState.BackupDir {
		t.Fatalf("apply restarted: before=%q after=%q", firstState.BackupDir, secondState.BackupDir)
	}
	assertFile(t, filepath.Join(root, "operator-marker.txt"), "keep")
}

func TestApplyPendingFailsClosedWhenPendingHealthVersionsMismatch(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, nil)
	if _, err := ApplyPending(root); err != nil {
		t.Fatal(err)
	}
	pending := mustPending(t, root)
	pending.TargetVersion = "unexpected-target"
	writeJSON(t, filepath.Join(root, ".updates", "pending-update.json"), pending)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "pending_health_state_invalid")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "new")
}

func TestRollbackRejectsEscapingBackupState(t *testing.T) {
	root := t.TempDir()
	updates := filepath.Join(root, ".updates")
	if err := os.MkdirAll(updates, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	mustWrite(t, outside, "do-not-touch")
	writeJSON(t, filepath.Join(updates, "update-state.json"), State{
		ContractVersion: StateContract, Status: "applying", CurrentVersion: "1", TargetVersion: "2",
		BackupDir: "../escape", Journal: []JournalEntry{{Path: "bin/app.exe", Existed: true, BackupPath: "../escape/outside"}}, UpdatedAt: now(),
	})
	_, err := Rollback(root)
	assertUpdateCode(t, err, "state_invalid")
	assertFile(t, outside, "do-not-touch")
}

func TestExplicitRollbackAndCommit(t *testing.T) {
	t.Run("rollback", func(t *testing.T) {
		root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new", "bin/new.exe": "created"}, nil)
		if _, err := ApplyPending(root); err != nil {
			t.Fatal(err)
		}
		result, err := Rollback(root)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != "rolled_back" {
			t.Fatalf("result=%+v", result)
		}
		assertFile(t, filepath.Join(root, "bin/app.exe"), "old")
		if _, err := os.Stat(filepath.Join(root, "bin/new.exe")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("new file survived rollback: %v", err)
		}
		second, err := Rollback(root)
		if err != nil || second.Status != "nothing_to_rollback" {
			t.Fatalf("second=%+v err=%v", second, err)
		}
	})
	t.Run("commit", func(t *testing.T) {
		root := newFixture(t, map[string]string{"bin/app.exe": "old"}, map[string]string{"bin/app.exe": "new"}, nil)
		if _, err := ApplyPending(root); err != nil {
			t.Fatal(err)
		}
		result, err := Commit(root)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != "committed" || result.CurrentVersion != "2" {
			t.Fatalf("result=%+v", result)
		}
		if _, err := os.Stat(filepath.Join(root, ".updates", "pending-update.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("pending still exists: %v", err)
		}
		status, err := Status(root)
		if err != nil || status.Status != "committed" {
			t.Fatalf("status=%+v err=%v", status, err)
		}
		second, err := Commit(root)
		if err != nil || second.Status != "committed" {
			t.Fatalf("idempotent commit=%+v err=%v", second, err)
		}
	})
}

func TestCommittedStateDoesNotDiscardNextPendingUpdate(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "one"}, map[string]string{"bin/app.exe": "two"}, nil)
	if _, err := ApplyPending(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Commit(root); err != nil {
		t.Fatal(err)
	}
	stagePending(t, root, "2", "3", map[string]string{"bin/app.exe": "three"})
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" || result.TargetVersion != "3" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertFile(t, filepath.Join(root, "bin/app.exe"), "three")
}

func TestApplyPendingFinishesCommittedSameTargetCrashCleanupWithoutReapply(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "one"}, map[string]string{"bin/app.exe": "two"}, nil)
	if _, err := ApplyPending(root); err != nil {
		t.Fatal(err)
	}
	state := mustState(t, root)
	state.Status = "committed"
	state.CurrentVersion = state.TargetVersion
	writeJSON(t, filepath.Join(root, ".updates", "update-state.json"), state)
	assertFile(t, filepath.Join(root, "bin/app.exe"), "two")

	result, err := applyPending(root, "", func(_ string, _ int) error {
		t.Fatal("committed same-target cleanup must not reapply files")
		return nil
	})
	if err != nil || result.Status != "no_pending" || result.CurrentVersion != "2" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".updates", "pending-update.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("same-target pending marker survived cleanup: %v", err)
	}
	cleaned := mustState(t, root)
	if cleaned.BackupDir != "" || len(cleaned.Journal) != 0 {
		t.Fatalf("committed cleanup retained recovery data: %+v", cleaned)
	}
}

func TestApplyPendingBindsPreservedRunnerIdentityToState(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/app.exe": "one"}, map[string]string{"bin/app.exe": "two"}, nil)
	runner := filepath.Join(root, ".updates", "runner", "archive-center-updater-fixture.exe")
	mustWrite(t, runner, "runner-v1")
	result, err := ApplyPendingFromRunner(root, runner)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	state := mustState(t, root)
	if state.RunnerPath != ".updates/runner/archive-center-updater-fixture.exe" || state.RunnerSHA256 != fileSHA(t, runner) {
		t.Fatalf("runner identity not bound to state: %+v", state)
	}

	outside := filepath.Join(t.TempDir(), "outside-runner.exe")
	mustWrite(t, outside, "outside")
	otherRoot := newFixture(t, map[string]string{"bin/app.exe": "one"}, map[string]string{"bin/app.exe": "two"}, nil)
	_, err = ApplyPendingFromRunner(otherRoot, outside)
	assertUpdateCode(t, err, "runner_identity_invalid")
	assertFile(t, filepath.Join(otherRoot, "bin/app.exe"), "one")
}

func TestApplyPendingRejectsDatabaseMigrationChanges(t *testing.T) {
	current := map[string]string{"bin/app.exe": "one", "bin/mariadb-schema.exe": "schema-tool", "migrations/001_schema.sql": "old schema"}
	next := map[string]string{"bin/app.exe": "two", "bin/mariadb-schema.exe": "schema-tool", "migrations/001_schema.sql": "new schema"}
	root := newFixture(t, current, next, nil)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "database_migration_update_unsupported")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "one")
}

func TestDatabaseMigrationToolUpgradeRequiresCompleteManifestContract(t *testing.T) {
	schemaTool := platformSchemaToolPath()
	current := map[string]string{"bin/archive-center-go": "one", schemaTool: "schema-tool"}
	next := map[string]string{"bin/archive-center-go": "two", schemaTool: "changed-schema-tool"}
	addCompleteMigrationUpdateContract(t, next, "2")
	root := newFixture(t, current, next, nil)
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertFile(t, filepath.Join(root, "bin/archive-center-go"), "two")
}

func TestApplyPendingAllowsOnlyManifestBoundMigrationPlan(t *testing.T) {
	current := map[string]string{
		"bin/app.exe": "one", "bin/mariadb-schema.exe": "schema-tool",
		"migrations/001_schema.sql":   "old schema",
		"migrations/005_existing.sql": "ALTER TABLE records ADD COLUMN IF NOT EXISTS old_value INT;",
	}
	next := map[string]string{
		"bin/app.exe": "two", "bin/mariadb-schema.exe": "new-schema-tool",
		"migrations/001_schema.sql":    "new schema",
		"migrations/005_existing.sql":  "ALTER TABLE records ADD COLUMN IF NOT EXISTS old_value INT;",
		"migrations/006_admission.sql": "CREATE TABLE admission (id BIGINT PRIMARY KEY, parent_id BIGINT, CONSTRAINT fk_parent FOREIGN KEY (parent_id) REFERENCES admission(id)); ALTER TABLE admission MODIFY COLUMN parent_id BIGINT NULL;",
	}
	addMigrationUpdateContract(t, current, next, "1", "2")
	root := newFixture(t, current, next, nil)
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertFile(t, filepath.Join(root, "migrations/006_admission.sql"), next["migrations/006_admission.sql"])
}

func TestApplyPendingRejectsChangedHistoricalMigration(t *testing.T) {
	current := map[string]string{
		"bin/app.exe":                 "one",
		"migrations/005_existing.sql": "ALTER TABLE records ADD COLUMN IF NOT EXISTS old_value INT;",
	}
	next := map[string]string{
		"bin/app.exe":                 "two",
		"migrations/005_existing.sql": "ALTER TABLE records ADD COLUMN IF NOT EXISTS changed_value INT;",
	}
	root := newFixture(t, current, next, nil)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "database_migration_update_unsupported")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "one")
}

func TestApplyPendingRejectsUnboundNewMigrationRegardlessOfSQLText(t *testing.T) {
	current := map[string]string{
		"bin/app.exe":                 "one",
		"migrations/001_schema.sql":   "old schema",
		"migrations/005_existing.sql": "ALTER TABLE records ADD COLUMN IF NOT EXISTS old_value INT;",
	}
	next := map[string]string{
		"bin/app.exe":                 "two",
		"migrations/001_schema.sql":   "new schema",
		"migrations/005_existing.sql": "ALTER TABLE records ADD COLUMN IF NOT EXISTS old_value INT;",
		"migrations/006_bad.sql":      "DROP TABLE records;",
	}
	root := newFixture(t, current, next, nil)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "database_migration_update_unsupported")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "one")
}

func TestApplyPendingRejectsMigrationContractWithWrongTargetFingerprint(t *testing.T) {
	current := map[string]string{
		"bin/app.exe":               "one",
		"migrations/001_schema.sql": "old schema",
	}
	next := map[string]string{
		"bin/app.exe":                  "two",
		"migrations/001_schema.sql":    "new schema",
		"migrations/006_admission.sql": "CREATE TABLE admission (id BIGINT PRIMARY KEY);",
	}
	addMigrationUpdateContract(t, current, next, "1", "2")
	next["migrations/006_admission.sql"] = "CREATE TABLE silently_changed (id BIGINT PRIMARY KEY);"
	root := newFixture(t, current, next, nil)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "database_migration_update_unsupported")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "one")
}

func TestApplyPendingV2UsesCompleteManifestWithoutHistoricalSourceInventory(t *testing.T) {
	schemaTool := platformSchemaToolPath()
	current := map[string]string{
		"bin/app.exe":               "old",
		"migrations/001_schema.sql": "old schema",
		"scripts/legacy.ps1":        "remove me",
	}
	current[schemaTool] = "old-schema-tool"
	next := map[string]string{
		"bin/app.exe":                    "new",
		"migrations/001_schema.sql":      "old schema",
		"migrations/002_expand_only.sql": "ALTER TABLE records ADD COLUMN IF NOT EXISTS added_value INT;",
	}
	next[schemaTool] = "new-schema-tool"
	addCompleteMigrationUpdateContract(t, next, "2")
	root := newFixture(t, current, next, nil)
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, "scripts", "legacy.ps1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("complete-manifest removal was not applied: %v", err)
	}
	assertFile(t, filepath.Join(root, filepath.FromSlash(schemaTool)), "new-schema-tool")
}

func TestApplyPendingV2RequiresExplicitExpandFirstCompatibility(t *testing.T) {
	schemaTool := platformSchemaToolPath()
	current := map[string]string{
		"migrations/001_schema.sql": "old schema",
	}
	current[schemaTool] = "old-schema-tool"
	next := map[string]string{
		"migrations/001_schema.sql": "new schema",
	}
	next[schemaTool] = "new-schema-tool"
	addCompleteMigrationUpdateContract(t, next, "2")
	var contract migrationUpdateManifest
	if err := json.Unmarshal([]byte(next[MigrationUpdateManifestName]), &contract); err != nil {
		t.Fatal(err)
	}
	contract.DatabasePolicy = "unspecified"
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	next[MigrationUpdateManifestName] = string(data)
	root := newFixture(t, current, next, nil)
	_, err = ApplyPending(root)
	assertUpdateCode(t, err, "database_migration_update_unsupported")
}

func TestDirectUpdatePreflightAndApplyAllows399To42WithCumulativeMigrations(t *testing.T) {
	schemaTool := platformSchemaToolPath()
	current := map[string]string{
		"bin/app.exe":               "old-app",
		"migrations/001_schema.sql": "CREATE TABLE existing_table (id BIGINT PRIMARY KEY);",
		"Legacy/Removed.TXT":        "remove-me",
		schemaTool:                  "old-schema-tool",
	}
	next := map[string]string{
		"bin/app.exe":                    "new-app",
		"migrations/001_schema.sql":      current["migrations/001_schema.sql"],
		"migrations/002_expand_only.sql": "ALTER TABLE existing_table ADD COLUMN IF NOT EXISTS title TEXT;",
		schemaTool:                       "new-schema-tool",
	}
	addCompleteMigrationUpdateContract(t, next, "4.2.0")

	root := t.TempDir()
	for rel, body := range current {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "3.9.9", current)
	asset := filepath.Join(t.TempDir(), "archive-center-4.2.zip")
	writePackageZipWithManifestPrefix(t, asset, "4.2.0", next, nil, nil)
	required := make([]string, 0, len(next))
	for rel := range next {
		required = append(required, rel)
	}
	sort.Strings(required)
	candidate := Candidate{CurrentVersion: "3.9.9", TargetVersion: "4.2.0", AssetPath: asset, SHA256: fileSHA(t, asset), RequiredFiles: required}
	if err := PreflightCandidate(root, candidate); err != nil {
		t.Fatalf("direct preflight failed: %v", err)
	}
	assertFile(t, filepath.Join(root, "bin/app.exe"), "old-app")
	if _, err := os.Stat(filepath.Join(root, ".updates")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preflight mutated package update state: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, ".updates"), 0o700); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, ".updates", "archive-center-4.2.zip")
	data, err := os.ReadFile(asset)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, data, 0o600); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(root, ".updates", "pending-update.json"), Pending{
		ContractVersion: PendingContract,
		CurrentVersion:  "3.9.9",
		TargetVersion:   "4.2.0",
		AssetPath:       filepath.ToSlash(filepath.Join(".updates", "archive-center-4.2.zip")),
		SHA256:          fileSHA(t, staged),
		RequiredFiles:   required,
	})
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" {
		t.Fatalf("direct apply result=%+v err=%v", result, err)
	}
	assertFile(t, filepath.Join(root, "bin/app.exe"), "new-app")
	if _, err := os.Stat(filepath.Join(root, "Legacy", "Removed.TXT")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed managed file remains after direct update: %v", err)
	}
}

func TestManagedFileRemovalsPreserveManifestPathCase(t *testing.T) {
	current := map[string]manifestFile{
		"archive center.js":  {Path: "Archive Center.js"},
		"legacy/removed.txt": {Path: "Legacy/Removed.TXT"},
	}
	next := map[string]manifestFile{
		"archive center.js": {Path: "Archive Center.js"},
	}
	removed := managedFileRemovals(current, next)
	if len(removed) != 1 || removed[0] != "Legacy/Removed.TXT" {
		t.Fatalf("removed paths = %v", removed)
	}
}

func TestDirectUpdatePreflightRejectsCumulativeMigrationRevisionGap(t *testing.T) {
	schemaTool := platformSchemaToolPath()
	current := map[string]string{
		"migrations/001_schema.sql": "CREATE TABLE existing_table (id BIGINT PRIMARY KEY);",
		schemaTool:                  "old-schema-tool",
	}
	next := map[string]string{
		"migrations/001_schema.sql": current["migrations/001_schema.sql"],
		"migrations/003_later.sql":  "CREATE TABLE later_table (id BIGINT PRIMARY KEY);",
		schemaTool:                  "new-schema-tool",
	}
	addCompleteMigrationUpdateContract(t, next, "4.2.0")
	root := t.TempDir()
	for rel, body := range current {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "3.9.9", current)
	stagePending(t, root, "3.9.9", "4.2.0", next)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "database_migration_update_unsupported")
}

func TestDirectUpdatePreflightRejectsUnverifiedReleasePackage(t *testing.T) {
	schemaTool := platformSchemaToolPath()
	current := map[string]string{
		"migrations/001_schema.sql": "CREATE TABLE existing_table (id BIGINT PRIMARY KEY);",
		schemaTool:                  "old-schema-tool",
	}
	next := map[string]string{
		"migrations/001_schema.sql": current["migrations/001_schema.sql"],
		schemaTool:                  "new-schema-tool",
	}
	addCompleteMigrationUpdateContract(t, next, "4.2.0")
	next[PackageReleaseStatusName] = fmt.Sprintf(`{"contract_version":%q,"target_version":"4.2.0","release_ready":false,"automatic_update_apply":true}`, PackageReleaseStatusContract)
	root := t.TempDir()
	for rel, body := range current {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "3.9.9", current)
	asset := filepath.Join(t.TempDir(), "candidate.zip")
	writePackageZipWithManifestPrefix(t, asset, "4.2.0", next, nil, nil)
	err := PreflightCandidate(root, Candidate{
		CurrentVersion: "3.9.9",
		TargetVersion:  "4.2.0",
		AssetPath:      asset,
		SHA256:         fileSHA(t, asset),
		RequiredFiles:  []string{MigrationUpdateManifestName, PackageReleaseStatusName, schemaTool},
	})
	assertUpdateCode(t, err, "package_release_unverified")
}

func TestDirectUpdatePreflightRejectsIncompatibleContractBeforeMutation(t *testing.T) {
	schemaTool := platformSchemaToolPath()
	current := map[string]string{
		"migrations/001_schema.sql": "CREATE TABLE existing_table (id BIGINT PRIMARY KEY);",
		schemaTool:                  "old-schema-tool",
	}
	root := t.TempDir()
	for rel, body := range current {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "3.9.9", current)

	for _, tc := range []struct {
		name   string
		mutate func(*migrationUpdateManifest, map[string]string)
	}{
		{name: "minimum source 4.0", mutate: func(contract *migrationUpdateManifest, _ map[string]string) { contract.MinimumSourceVersion = "4.0.0" }},
		{name: "direct jump disabled", mutate: func(contract *migrationUpdateManifest, _ map[string]string) { contract.DirectUpdateSupported = false }},
		{name: "legacy source inventory contract", mutate: func(contract *migrationUpdateManifest, _ map[string]string) {
			contract.ContractVersion = MigrationUpdateContract
		}},
		{name: "historical migration changed", mutate: func(_ *migrationUpdateManifest, next map[string]string) {
			next["migrations/001_schema.sql"] = "CREATE TABLE changed_table (id BIGINT PRIMARY KEY);"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := map[string]string{
				"migrations/001_schema.sql": current["migrations/001_schema.sql"],
				schemaTool:                  "new-schema-tool",
			}
			addCompleteMigrationUpdateContract(t, next, "4.2.0")
			var contract migrationUpdateManifest
			if err := json.Unmarshal([]byte(next[MigrationUpdateManifestName]), &contract); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&contract, next)
			contract.Target = migrationFilesFromBodies(next)
			data, err := json.Marshal(contract)
			if err != nil {
				t.Fatal(err)
			}
			next[MigrationUpdateManifestName] = string(data)
			asset := filepath.Join(t.TempDir(), "candidate.zip")
			writePackageZipWithManifestPrefix(t, asset, "4.2.0", next, nil, nil)
			err = PreflightCandidate(root, Candidate{CurrentVersion: "3.9.9", TargetVersion: "4.2.0", AssetPath: asset, SHA256: fileSHA(t, asset), RequiredFiles: []string{MigrationUpdateManifestName, schemaTool}})
			assertUpdateCode(t, err, "database_migration_update_unsupported")
			if _, statErr := os.Stat(filepath.Join(root, ".updates")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("rejected preflight mutated update state: %v", statErr)
			}
		})
	}
}

func TestDirectUpdatePreflightDoesNotOfferPreBaseline390Source(t *testing.T) {
	schemaTool := platformSchemaToolPath()
	current := map[string]string{schemaTool: "old-schema-tool"}
	next := map[string]string{schemaTool: "new-schema-tool"}
	addCompleteMigrationUpdateContract(t, next, "4.2.0")
	root := t.TempDir()
	for rel, body := range current {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "3.9.0", current)
	asset := filepath.Join(t.TempDir(), "candidate.zip")
	writePackageZipWithManifestPrefix(t, asset, "4.2.0", next, nil, nil)
	err := PreflightCandidate(root, Candidate{CurrentVersion: "3.9.0", TargetVersion: "4.2.0", AssetPath: asset, SHA256: fileSHA(t, asset), RequiredFiles: []string{MigrationUpdateManifestName, schemaTool}})
	assertUpdateCode(t, err, "source_version_unsupported")
	if _, statErr := os.Stat(filepath.Join(root, ".updates")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("pre-baseline rejection mutated update state: %v", statErr)
	}
}

func platformSchemaToolPath() string {
	if runtime.GOOS == "windows" {
		return "bin/mariadb-schema.exe"
	}
	return "bin/mariadb-schema"
}

// These fingerprints come from the published Windows Update Package ZIPs, not
// Git blobs. The historical updater cannot consume this contract itself; the
// external compatibility bridge launches the authenticated candidate updater.
func TestNewUpdaterContractRecognizesPublished300301And35MigrationFingerprints(t *testing.T) {
	sourceFixtures := []migrationManifestSource{
		{
			Version: "3.0.0",
			Files: []migrationManifestFile{
				{Path: "migrations/001_schema.sql", SizeBytes: 73630, SHA256: "09fc00c560436d46fe295e6dddbf01432c7daf8148beebef9c959ae7b5cb06d3"},
			},
		},
		{
			Version: "3.0.1",
			Files: []migrationManifestFile{
				{Path: "migrations/001_schema.sql", SizeBytes: 73630, SHA256: "09fc00c560436d46fe295e6dddbf01432c7daf8148beebef9c959ae7b5cb06d3"},
			},
		},
		{
			Version: "3.5.0",
			Files: []migrationManifestFile{
				{Path: "migrations/001_schema.sql", SizeBytes: 92635, SHA256: "85df8ac1480eadb85e9cf572cbc2696dd53a8f83b1b3bfd06be8f01016132e6f"},
				{Path: "migrations/002_canon_pack_storage.sql", SizeBytes: 18802, SHA256: "27579f2efe123768c24dd873764105cfbc5901b4f3b81d81e4fb39ca8a1c2bd6"},
			},
		},
	}
	targetFiles := actualMigrationInventory(t)
	contract := migrationUpdateManifest{
		ContractVersion: MigrationUpdateContract,
		TargetVersion:   "3.7.0",
		Target:          targetFiles,
		Sources:         sourceFixtures,
	}
	packageRoot := t.TempDir()
	contractPath := filepath.Join(packageRoot, MigrationUpdateManifestName)
	writeJSON(t, contractPath, contract)
	next := manifestMapFromMigrationFiles(targetFiles)
	info, err := os.Stat(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	next[strings.ToLower(MigrationUpdateManifestName)] = manifestFile{
		Path: MigrationUpdateManifestName, SizeBytes: info.Size(), SHA256: fileSHA(t, contractPath),
	}
	for _, source := range sourceFixtures {
		current := manifestMapFromMigrationFiles(source.Files)
		if _, err := validateDatabaseMigrationUpdate(packageRoot, source.Version, "3.7.0", current, next, false); err != nil {
			t.Fatalf("%s -> 3.7 inventory contract rejected: %v", source.Version, err)
		}
	}
}

func TestManagedInstallModePreservesPOSIXExecutables(t *testing.T) {
	for _, rel := range []string{"bin/archive-center-go", "bin/archive-center-updater", "scripts/start-full-posix.sh", "Start Archive Center macOS.command"} {
		if got := managedInstallMode(rel, 0o644, "linux"); got != 0o755 {
			t.Fatalf("managedInstallMode(%q) = %#o, want 0755", rel, got)
		}
	}
	if got := managedInstallMode("prompts/critic_system.txt", 0, "linux"); got != 0o644 {
		t.Fatalf("regular POSIX mode = %#o, want 0644", got)
	}
	if got := managedInstallMode("bin/archive-center-go.exe", 0o640, "windows"); got != 0o640 {
		t.Fatalf("Windows mode = %#o, want archived 0640", got)
	}
}

func TestValidateInstalledApplyHelperUsesManifestFingerprint(t *testing.T) {
	rel := "bin/archive-center-updater"
	if runtime.GOOS == "windows" {
		rel += ".exe"
	}
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(rel))
	mustWrite(t, path, "verified updater")
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest(t, filepath.Join(root, ManifestName), "1", map[string]string{rel: "verified updater"})
	if err := ValidateInstalledApplyHelper(root); err != nil {
		t.Fatalf("valid apply helper rejected: %v", err)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "", map[string]string{rel: "verified updater"})
	if err := ValidateInstalledApplyHelper(root); err == nil || !strings.Contains(err.Error(), "no package_version") {
		t.Fatalf("missing package_version error = %v", err)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "1", map[string]string{rel: "verified updater"})
	if err := os.WriteFile(path, []byte("tampered updater"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ValidateInstalledApplyHelper(root); err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("tampered apply helper error = %v", err)
	}
}

func TestValidateInstalledApplyHelperRequiresPOSIXExecutableBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use POSIX executable mode bits")
	}
	root := t.TempDir()
	rel := "bin/archive-center-updater"
	path := filepath.Join(root, filepath.FromSlash(rel))
	mustWrite(t, path, "updater")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "1", map[string]string{rel: "updater"})
	if err := ValidateInstalledApplyHelper(root); err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Fatalf("non-executable helper error = %v", err)
	}
}

func addMigrationUpdateContract(t *testing.T, current, next map[string]string, currentVersion, targetVersion string) {
	addMigrationUpdateContractWithRemovals(t, current, next, currentVersion, targetVersion, nil)
}

func addMigrationUpdateContractWithRemovals(t *testing.T, current, next map[string]string, currentVersion, targetVersion string, removals []string) {
	t.Helper()
	contract := migrationUpdateManifest{
		ContractVersion: MigrationUpdateContract,
		TargetVersion:   targetVersion,
		Target:          migrationFilesFromBodies(next),
		Sources: []migrationManifestSource{{
			Version:             currentVersion,
			Files:               migrationFilesFromBodies(current),
			RemovedManagedPaths: removals,
		}},
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	next[MigrationUpdateManifestName] = string(data)
}

func addCompleteMigrationUpdateContract(t *testing.T, next map[string]string, targetVersion string) {
	t.Helper()
	next[PackageReleaseStatusName] = fmt.Sprintf(`{"contract_version":%q,"target_version":%q,"release_ready":true,"automatic_update_apply":true}`, PackageReleaseStatusContract, targetVersion)
	contract := migrationUpdateManifest{
		ContractVersion:       MigrationUpdateContractV2,
		TargetVersion:         targetVersion,
		Target:                migrationFilesFromBodies(next),
		ManagedFiles:          CompleteManagedPackage,
		DatabasePolicy:        ExpandFirstCompatibility,
		MinimumSourceVersion:  DirectUpdateBaselineVersion,
		DirectUpdateSupported: true,
		MigrationInventory:    CumulativeMigrationInventory,
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	next[MigrationUpdateManifestName] = string(data)
}

func migrationFilesFromBodies(files map[string]string) []migrationManifestFile {
	out := []migrationManifestFile{}
	for rel, body := range files {
		rel = strings.ToLower(filepath.ToSlash(rel))
		if !isMigrationInventoryPath(rel) {
			continue
		}
		sum := sha256.Sum256([]byte(body))
		out = append(out, migrationManifestFile{
			Path: rel, SizeBytes: int64(len([]byte(body))), SHA256: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func actualMigrationInventory(t *testing.T) []migrationManifestFile {
	t.Helper()
	root := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	out := []migrationManifestFile{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		out = append(out, migrationManifestFile{
			Path:      "migrations/" + strings.ToLower(entry.Name()),
			SizeBytes: int64(len(data)),
			SHA256:    hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	if len(out) == 0 {
		t.Fatal("production migration inventory is empty")
	}
	return out
}

func manifestMapFromMigrationFiles(files []migrationManifestFile) map[string]manifestFile {
	out := make(map[string]manifestFile, len(files))
	for _, file := range files {
		rel := strings.ToLower(filepath.ToSlash(file.Path))
		out[rel] = manifestFile{Path: rel, SizeBytes: file.SizeBytes, SHA256: file.SHA256}
	}
	return out
}

type zipEntry struct {
	name, body string
	mode       os.FileMode
}

func newFixture(t *testing.T, current, next map[string]string, extras []zipEntry) string {
	return newFixtureWithManifestPrefixes(t, current, next, nil, nil, extras...)
}

func newFixtureWithManifestPrefixes(t *testing.T, current, next map[string]string, currentPrefix, candidatePrefix []byte, extras ...zipEntry) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range current {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "1", current)
	if len(currentPrefix) > 0 {
		prependFile(t, filepath.Join(root, ManifestName), currentPrefix)
	}
	updates := filepath.Join(root, ".updates")
	if err := os.MkdirAll(updates, 0o700); err != nil {
		t.Fatal(err)
	}
	stagePendingWithManifestPrefix(t, root, "1", "2", next, candidatePrefix, extras)
	return root
}

func stagePending(t *testing.T, root, currentVersion, targetVersion string, files map[string]string) {
	t.Helper()
	stagePendingWithExtras(t, root, currentVersion, targetVersion, files, nil)
}

func stagePendingWithExtras(t *testing.T, root, currentVersion, targetVersion string, files map[string]string, extras []zipEntry) {
	stagePendingWithManifestPrefix(t, root, currentVersion, targetVersion, files, nil, extras)
}

func stagePendingWithManifestPrefix(t *testing.T, root, currentVersion, targetVersion string, files map[string]string, manifestPrefix []byte, extras []zipEntry) {
	t.Helper()
	updates := filepath.Join(root, ".updates")
	assetName := "staged-" + targetVersion + ".zip"
	asset := filepath.Join(updates, assetName)
	writePackageZipWithManifestPrefix(t, asset, targetVersion, files, manifestPrefix, extras)
	h := fileSHA(t, asset)
	required := make([]string, 0, len(files))
	for rel := range files {
		required = append(required, filepath.ToSlash(rel))
	}
	sort.Strings(required)
	writeJSON(t, filepath.Join(updates, "pending-update.json"), Pending{ContractVersion: PendingContract, CurrentVersion: currentVersion, TargetVersion: targetVersion, AssetPath: filepath.ToSlash(filepath.Join(".updates", assetName)), SHA256: h, RequiredFiles: required})
}

func writePackageZip(t *testing.T, path string, files map[string]string, extras []zipEntry) {
	writePackageZipWithManifestPrefix(t, path, "2", files, nil, extras)
}

func writePackageZipWithManifestPrefix(t *testing.T, path, packageVersion string, files map[string]string, manifestPrefix []byte, extras []zipEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	manifest := manifestFor(packageVersion, files)
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	entries := []zipEntry{{name: "release/" + ManifestName, body: string(append(append([]byte{}, manifestPrefix...), data...))}}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		entries = append(entries, zipEntry{name: "release/" + filepath.ToSlash(k), body: files[k]})
	}
	entries = append(entries, extras...)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, e.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeManifest(t *testing.T, path, packageVersion string, files map[string]string) {
	t.Helper()
	writeJSON(t, path, manifestFor(packageVersion, files))
}
func prependFile(t *testing.T, path string, prefix []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(append([]byte{}, prefix...), data...), 0o600); err != nil {
		t.Fatal(err)
	}
}
func manifestFor(packageVersion string, files map[string]string) packageManifest {
	m := packageManifest{SchemaVersion: "archive-center.package-file-manifest.v1", PackageVersion: packageVersion}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b := []byte(files[k])
		sum := sha256.Sum256(b)
		m.Files = append(m.Files, manifestFile{Path: filepath.ToSlash(k), SizeBytes: int64(len(b)), SHA256: hex.EncodeToString(sum[:])})
	}
	return m
}
func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}
func fileSHA(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func assertFile(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Fatalf("%s=%q want %q", path, b, want)
	}
}
func assertUpdateCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("wanted %s error", want)
	}
	var updateErr *UpdateError
	if !errors.As(err, &updateErr) || updateErr.Code != want {
		t.Fatalf("err=%v want code %s", err, want)
	}
}
func mustPending(t *testing.T, root string) Pending {
	t.Helper()
	var p Pending
	b, err := os.ReadFile(filepath.Join(root, ".updates", "pending-update.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func mustState(t *testing.T, root string) State {
	t.Helper()
	var s State
	b, err := os.ReadFile(filepath.Join(root, ".updates", "update-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
