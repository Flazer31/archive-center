package packageupdate

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Keep the audit's exact 37-byte JS mutation, with the original manifest intact.
const auditJSAppend = "\n// synthetic package audit mutation\n"

func TestManifestIntegrityRejectsMismatchedCandidate(t *testing.T) {
	if len(auditJSAppend) != 37 {
		t.Fatal("audit reproduction must append exactly 37 bytes")
	}
	cases := []struct {
		name, path, mismatch string
		mutate               func(map[string]string, []map[string]any)
	}{
		{"audit_js_append_37_bytes", "Archive Center.js", "size_bytes", func(payload map[string]string, _ []map[string]any) {
			payload["Archive Center.js"] += auditJSAppend
		}},
		{"same_size_wrong_digest", "Archive Center.js", "sha256", func(payload map[string]string, _ []map[string]any) {
			payload["Archive Center.js"] = strings.Replace(payload["Archive Center.js"], "candidate", "corrupted", 1)
		}},
		{"size_only_mismatch", "Archive Center.js", "size_bytes", func(_ map[string]string, entries []map[string]any) {
			for _, entry := range entries {
				if entry["path"] == "Archive Center.js" {
					entry["size_bytes"] = entry["size_bytes"].(int64) + 1
				}
			}
		}},
		{"nonrequired_manifest_tail_wrong_digest", "scripts/z-start.ps1", "sha256", func(payload map[string]string, _ []map[string]any) {
			payload["scripts/z-start.ps1"] = "bad-start"
		}},
	}
	for _, tc := range cases {
		for _, recovery := range []bool{false, true} {
			for _, action := range []string{"apply", "preflight"} {
				if recovery && action == "preflight" {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/recovery_%t", tc.name, action, recovery), func(t *testing.T) {
					root, original, next := newIntegrityFixture(t)
					stageIntegrityCandidate(t, root, next, tc.mutate)
					if recovery {
						seedInterruptedIntegrityApply(t, root, original)
					}
					pending := mustPending(t, root)
					var err error
					if action == "preflight" {
						err = PreflightCandidate(root, Candidate{CurrentVersion: pending.CurrentVersion, TargetVersion: pending.TargetVersion, AssetPath: filepath.Join(root, filepath.FromSlash(pending.AssetPath)), RequiredFiles: pending.RequiredFiles})
					} else {
						result, applyErr := ApplyPending(root)
						err = applyErr
						if result.HealthRequired || result.Status == "applied_pending_health" {
							t.Errorf("mismatched candidate reported installed: %+v", result)
						}
					}
					assertIntegrityInstallation(t, root, original)
					assertUpdateCode(t, err, "package_verification_failed")
					if !strings.Contains(err.Error(), tc.path) || !strings.Contains(err.Error(), tc.mismatch) {
						t.Fatalf("missing file/mismatch diagnostic: %v", err)
					}
					assertIntegrityPending(t, root, pending)
					if recovery {
						assertIntegrityRolledBack(t, root)
					} else if _, statErr := os.Stat(filepath.Join(root, ".updates/update-state.json")); !errors.Is(statErr, os.ErrNotExist) {
						t.Fatalf("mismatch changed update state before installation: %v", statErr)
					}
				})
			}
		}
	}
}

func TestManifestIntegrityValidApplyAndRollback(t *testing.T) {
	root, original, next := newIntegrityFixture(t)
	// Uppercase hex is also produced by SHA-256 tools; an empty file is valid.
	stageIntegrityCandidate(t, root, next, func(_ map[string]string, entries []map[string]any) {
		for _, entry := range entries {
			entry["sha256"] = strings.ToUpper(entry["sha256"].(string))
		}
	})
	pending := mustPending(t, root)
	if err := PreflightCandidate(root, Candidate{CurrentVersion: pending.CurrentVersion, TargetVersion: pending.TargetVersion, AssetPath: filepath.Join(root, filepath.FromSlash(pending.AssetPath)), RequiredFiles: pending.RequiredFiles}); err != nil {
		t.Fatal(err)
	}
	assertIntegrityInstallation(t, root, original)
	result, err := ApplyPending(root)
	if err != nil || result.Status != "applied_pending_health" || !result.HealthRequired {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for rel, body := range next {
		assertFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	if _, err := os.Stat(filepath.Join(root, "scripts/legacy.ps1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("valid managed removal failed: %v", err)
	}
	assertIntegritySentinels(t, root)
	result, err = Rollback(root)
	if err != nil || result.Status != "rolled_back" {
		t.Fatalf("rollback result=%+v err=%v", result, err)
	}
	assertIntegrityInstallation(t, root, original)
	assertIntegrityRolledBack(t, root)
}

func TestManifestIntegrityMutationDuringApplyRollsBack(t *testing.T) {
	for _, mutation := range []string{"append", "same_size"} {
		t.Run(mutation, func(t *testing.T) {
			root, original, next := newIntegrityFixture(t)
			stageIntegrityCandidate(t, root, next, nil)
			pending := mustPending(t, root)
			mutated := false
			_, err := applyPending(root, "", func(rel string, _ int) error {
				if rel != "scripts/z-start.ps1" {
					return nil
				}
				// Prove changes have begun before mutating the extracted next file.
				assertFile(t, filepath.Join(root, "Archive Center.js"), next["Archive Center.js"])
				assertFile(t, filepath.Join(root, "scripts/a-added.ps1"), next["scripts/a-added.ps1"])
				assertFile(t, filepath.Join(root, rel), original[rel])
				matches, err := filepath.Glob(filepath.Join(root, ".updates/extracted/apply-*/release", filepath.FromSlash(rel)))
				if err != nil || len(matches) != 1 {
					t.Fatalf("extracted payload matches=%v err=%v", matches, err)
				}
				body := "bad-start"
				if mutation == "append" {
					body = next[rel] + auditJSAppend
				}
				mustWrite(t, matches[0], body)
				mutated = true
				return nil
			})
			if !mutated {
				t.Fatal("mutation hook was not reached")
			}
			assertIntegrityInstallation(t, root, original)
			assertUpdateCode(t, err, "apply_failed")
			if !strings.Contains(err.Error(), "scripts/z-start.ps1") {
				t.Fatalf("missing mismatch path: %v", err)
			}
			assertIntegrityRolledBack(t, root)
			assertIntegrityPending(t, root, pending)
			if _, err := os.Stat(filepath.Join(root, "scripts/z-start.ps1.update-tmp")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("mismatched temporary copy remains: %v", err)
			}
		})
	}
}

func newIntegrityFixture(t *testing.T) (string, map[string]string, map[string]string) {
	t.Helper()
	current := map[string]string{"Archive Center.js": "// installed plugin\n", "bin/app.exe": "old-backend", "scripts/z-start.ps1": "old-start", "scripts/legacy.ps1": "legacy"}
	next := map[string]string{"Archive Center.js": "// candidate plugin\n", "bin/app.exe": "new-backend", "scripts/a-added.ps1": "added", "scripts/z-start.ps1": "new-start", "prompts/empty.txt": ""}
	addReleaseStatus(t, next, "4.9.0")
	root := t.TempDir()
	for rel, body := range current {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	writeManifest(t, filepath.Join(root, ManifestName), "4.8.0", current)
	manifest, err := os.ReadFile(filepath.Join(root, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	current[ManifestName] = string(manifest)
	// Installed local edits must remain replaceable and restorable.
	current["bin/app.exe"] = "locally-modified-backend"
	mustWrite(t, filepath.Join(root, "bin/app.exe"), current["bin/app.exe"])
	for rel, body := range integritySentinels() {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return root, current, next
}

func integritySentinels() map[string]string {
	return map[string]string{"data/mariadb-data/sentinel.txt": "synthetic SQL data", "data/chromadb-data/sentinel.txt": "synthetic vector data", ".runtime/sentinel.txt": "synthetic runtime", ".env.full.local": "SYNTHETIC=keep", "secrets/provider.txt": "synthetic sentinel"}
}

func assertIntegritySentinels(t *testing.T, root string) {
	t.Helper()
	for rel, body := range integritySentinels() {
		assertFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
}

func assertIntegrityInstallation(t *testing.T, root string, original map[string]string) {
	t.Helper()
	for rel, body := range original {
		assertFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	for _, rel := range []string{"scripts/a-added.ps1", "prompts/empty.txt", PackageReleaseStatusName} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("candidate-only file remains installed: %s: %v", rel, err)
		}
	}
	assertIntegritySentinels(t, root)
}

func assertIntegrityPending(t *testing.T, root string, want Pending) {
	t.Helper()
	got, _ := json.Marshal(mustPending(t, root))
	expected, _ := json.Marshal(want)
	if string(got) != string(expected) {
		t.Fatalf("pending changed: %s want %s", got, expected)
	}
}

func assertIntegrityRolledBack(t *testing.T, root string) {
	t.Helper()
	if state := mustState(t, root); state.Status != "rolled_back" || len(state.Journal) != 0 || state.BackupDir != "" {
		t.Fatalf("rollback did not finish: %+v", state)
	}
}

func seedInterruptedIntegrityApply(t *testing.T, root string, original map[string]string) {
	t.Helper()
	state := State{ContractVersion: StateContract, Status: "applying", CurrentVersion: "4.8.0", TargetVersion: "4.9.0", BackupDir: "backups/interrupted", UpdatedAt: now()}
	for _, rel := range []string{"Archive Center.js", "bin/app.exe", "scripts/legacy.ps1", ManifestName} {
		backup := state.BackupDir + "/" + rel
		mustWrite(t, filepath.Join(root, ".updates", filepath.FromSlash(backup)), original[rel])
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), "partially-applied")
		state.Journal = append(state.Journal, JournalEntry{Path: rel, Existed: true, BackupPath: backup})
	}
	mustWrite(t, filepath.Join(root, "scripts/a-added.ps1"), "partially-added")
	state.Journal = append(state.Journal, JournalEntry{Path: "scripts/a-added.ps1"})
	writeJSON(t, filepath.Join(root, ".updates/update-state.json"), state)
}

// Build the wire manifest independently of the production decoder so the
// regression runs against the original path-only manifestFile implementation.
func stageIntegrityCandidate(t *testing.T, root string, declared map[string]string, mutate func(map[string]string, []map[string]any)) {
	t.Helper()
	payload := make(map[string]string, len(declared))
	keys := make([]string, 0, len(declared))
	for rel, body := range declared {
		payload[rel] = body
		keys = append(keys, rel)
	}
	sort.Strings(keys)
	entries := make([]map[string]any, 0, len(keys))
	for _, rel := range keys {
		body := []byte(declared[rel])
		entries = append(entries, map[string]any{"path": rel, "sha256": fmt.Sprintf("%x", sha256.Sum256(body)), "size_bytes": int64(len(body))})
	}
	if mutate != nil {
		mutate(payload, entries)
	}
	manifest, err := json.Marshal(map[string]any{"schema_version": "archive-center.package-file-manifest.v1", "package_version": "4.9.0", "files": entries})
	if err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(root, ".updates/integrity-candidate.zip")
	if err := os.MkdirAll(filepath.Dir(asset), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(asset)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	payload[ManifestName] = string(manifest)
	for rel, body := range payload {
		w, err := zw.Create("release/" + rel)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// RequiredFiles is deliberately smaller than the manifest: all entries,
	// including the last nonrequired script, must still be verified.
	writeJSON(t, filepath.Join(root, ".updates/pending-update.json"), Pending{ContractVersion: PendingContract, CurrentVersion: "4.8.0", TargetVersion: "4.9.0", AssetPath: ".updates/integrity-candidate.zip", RequiredFiles: []string{"Archive Center.js"}})
}
