package packageupdate

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	result, err := applyPending(root, func(_ string, _ int) error {
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

func TestApplyFailureRollsBackEveryTouchedFile(t *testing.T) {
	root := newFixture(t, map[string]string{"bin/a.exe": "old-a", "bin/b.exe": "old-b"}, map[string]string{"bin/a.exe": "new-a", "bin/b.exe": "new-b"}, nil)
	_, err := applyPending(root, func(_ string, index int) error {
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

func TestApplyPendingRejectsDatabaseMigrationChanges(t *testing.T) {
	current := map[string]string{"bin/app.exe": "one", "bin/mariadb-schema.exe": "schema-tool", "migrations/001_schema.sql": "old schema"}
	next := map[string]string{"bin/app.exe": "two", "bin/mariadb-schema.exe": "schema-tool", "migrations/001_schema.sql": "new schema"}
	root := newFixture(t, current, next, nil)
	_, err := ApplyPending(root)
	assertUpdateCode(t, err, "database_migration_update_unsupported")
	assertFile(t, filepath.Join(root, "bin/app.exe"), "one")
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
	writeManifest(t, filepath.Join(root, ManifestName), current)
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
	writePackageZipWithManifestPrefix(t, asset, files, manifestPrefix, extras)
	h := fileSHA(t, asset)
	required := make([]string, 0, len(files))
	for rel := range files {
		required = append(required, filepath.ToSlash(rel))
	}
	sort.Strings(required)
	writeJSON(t, filepath.Join(updates, "pending-update.json"), Pending{ContractVersion: PendingContract, CurrentVersion: currentVersion, TargetVersion: targetVersion, AssetPath: filepath.ToSlash(filepath.Join(".updates", assetName)), SHA256: h, RequiredFiles: required})
}

func writePackageZip(t *testing.T, path string, files map[string]string, extras []zipEntry) {
	writePackageZipWithManifestPrefix(t, path, files, nil, extras)
}

func writePackageZipWithManifestPrefix(t *testing.T, path string, files map[string]string, manifestPrefix []byte, extras []zipEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	manifest := manifestFor(files)
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

func writeManifest(t *testing.T, path string, files map[string]string) {
	t.Helper()
	writeJSON(t, path, manifestFor(files))
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
func manifestFor(files map[string]string) packageManifest {
	m := packageManifest{SchemaVersion: "archive-center.package-file-manifest.v1"}
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
