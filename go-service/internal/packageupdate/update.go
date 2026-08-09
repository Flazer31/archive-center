package packageupdate

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	PendingContract              = "archive-center.pending-update.v1"
	StateContract                = "archive-center.update-state.v1"
	ResultContract               = "archive-center.updater-result.v1"
	ManifestName                 = "PACKAGE_FILE_MANIFEST.json"
	MigrationUpdateManifestName  = "PACKAGE_MIGRATION_UPDATE.json"
	PackageReleaseStatusName     = "PACKAGE_RELEASE_STATUS.json"
	MigrationUpdateContract      = "archive-center.package-migration-update.v1"
	MigrationUpdateContractV2    = "archive-center.package-migration-update.v2"
	CompleteManagedPackage       = "complete_manifest"
	ExpandFirstCompatibility     = "expand_first_old_backend_compatible"
	DirectUpdateBaselineVersion  = "3.9.9"
	CumulativeMigrationInventory = "cumulative_complete"
	PackageReleaseStatusContract = "archive-center.package-release-status.v1"
)

type extractionLimits struct {
	Files int
	Bytes int64
}

var archiveLimits = extractionLimits{Files: 20000, Bytes: 4 * 1024 * 1024 * 1024}

type Pending struct {
	ContractVersion string   `json:"contract_version"`
	CurrentVersion  string   `json:"current_version"`
	TargetVersion   string   `json:"target_version"`
	AssetPath       string   `json:"asset_path"`
	SHA256          string   `json:"sha256"`
	RequiredFiles   []string `json:"required_files,omitempty"`
}

type JournalEntry struct {
	Path       string `json:"path"`
	Existed    bool   `json:"existed"`
	BackupPath string `json:"backup_path,omitempty"`
}

type State struct {
	ContractVersion string         `json:"contract_version"`
	Status          string         `json:"status"`
	CurrentVersion  string         `json:"current_version,omitempty"`
	TargetVersion   string         `json:"target_version,omitempty"`
	RunnerPath      string         `json:"runner_path,omitempty"`
	RunnerSHA256    string         `json:"runner_sha256,omitempty"`
	BackupDir       string         `json:"backup_dir,omitempty"`
	Journal         []JournalEntry `json:"journal,omitempty"`
	UpdatedAt       string         `json:"updated_at"`
}

type Result struct {
	ContractVersion string `json:"contract_version"`
	Action          string `json:"action"`
	Status          string `json:"status"`
	CurrentVersion  string `json:"current_version,omitempty"`
	TargetVersion   string `json:"target_version,omitempty"`
	HealthRequired  bool   `json:"health_required"`
	Message         string `json:"message,omitempty"`
}

type UpdateError struct {
	Code string
	Err  error
}

func (e *UpdateError) Error() string { return e.Code + ": " + e.Err.Error() }
func (e *UpdateError) Unwrap() error { return e.Err }

type packageManifest struct {
	SchemaVersion  string         `json:"schema_version"`
	PackageVersion string         `json:"package_version"`
	Files          []manifestFile `json:"files"`
}

type manifestFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type migrationUpdateManifest struct {
	ContractVersion       string                    `json:"contract_version"`
	TargetVersion         string                    `json:"target_version"`
	Target                []migrationManifestFile   `json:"target"`
	Sources               []migrationManifestSource `json:"sources,omitempty"`
	ManagedFiles          string                    `json:"managed_files,omitempty"`
	DatabasePolicy        string                    `json:"database_policy,omitempty"`
	MinimumSourceVersion  string                    `json:"minimum_source_version,omitempty"`
	DirectUpdateSupported bool                      `json:"direct_update_supported,omitempty"`
	MigrationInventory    string                    `json:"migration_inventory,omitempty"`
}

type migrationManifestSource struct {
	Version             string                  `json:"version"`
	Files               []migrationManifestFile `json:"files"`
	RemovedManagedPaths []string                `json:"removed_managed_paths,omitempty"`
}

type migrationManifestFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type packageReleaseManifest struct {
	ContractVersion      string `json:"contract_version"`
	TargetVersion        string `json:"target_version"`
	ReleaseReady         bool   `json:"release_ready"`
	AutomaticUpdateApply *bool  `json:"automatic_update_apply,omitempty"`
}

type installFile struct {
	Rel    string
	Src    string
	Mode   fs.FileMode
	Remove bool
}

type applyHook func(relativePath string, index int) error

type Candidate struct {
	CurrentVersion string
	TargetVersion  string
	AssetPath      string
	SHA256         string
	RequiredFiles  []string
}

type candidateValidation struct {
	Files          []installFile
	RemovedManaged []string
}

func ApplyPending(root string) (Result, error) { return applyPending(root, "", nil) }

// PreflightCandidate runs the same archive, manifest, installed-package, direct
// update, and migration validation used by ApplyPending without writing update
// state or changing managed package files. The archive may live outside
// .updates because /update/check downloads it into an isolated temporary file.
func PreflightCandidate(root string, candidate Candidate) error {
	root, _, err := resolveRoot(root)
	if err != nil {
		return updateErr("invalid_root", err)
	}
	if strings.TrimSpace(candidate.CurrentVersion) == "" || strings.TrimSpace(candidate.TargetVersion) == "" {
		return updateErr("candidate_invalid", fmt.Errorf("current_version and target_version are required"))
	}
	asset, err := filepath.Abs(strings.TrimSpace(candidate.AssetPath))
	if err != nil {
		return updateErr("asset_path_invalid", err)
	}
	info, err := os.Lstat(asset)
	if err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = fmt.Errorf("asset is not a regular file")
		}
		return updateErr("asset_path_invalid", err)
	}
	if err := verifyFile(asset, -1, candidate.SHA256); err != nil {
		return updateErr("asset_verification_failed", err)
	}
	tempRoot, err := os.MkdirTemp("", "archive-center-update-preflight-")
	if err != nil {
		return updateErr("preflight_temp_failed", err)
	}
	defer os.RemoveAll(tempRoot)
	extracted, err := extractVerifiedArchive(asset, tempRoot)
	if err != nil {
		return updateErr("archive_rejected", err)
	}
	packageRoot, err := findPackageRoot(extracted)
	if err != nil {
		return updateErr("package_root_invalid", err)
	}
	_, err = validateExtractedCandidate(root, packageRoot, candidate.CurrentVersion, candidate.TargetVersion, candidate.RequiredFiles, true)
	return err
}

// ValidateInstalledApplyHelper verifies the updater binary that the managed
// launcher and backend handoff will execute. The current package manifest is
// authoritative; existence alone is not enough because an incomplete or
// locally modified helper cannot safely own the update journal.
func ValidateInstalledApplyHelper(root string) error {
	root, _, err := resolveRoot(root)
	if err != nil {
		return err
	}
	manifest, err := readPackageManifest(filepath.Join(root, ManifestName))
	if err != nil {
		return fmt.Errorf("current package manifest: %w", err)
	}
	if strings.TrimSpace(manifest.PackageVersion) == "" {
		return fmt.Errorf("current package manifest has no package_version")
	}
	rel := "bin/archive-center-updater"
	if runtime.GOOS == "windows" {
		rel += ".exe"
	}
	var entry *manifestFile
	for i := range manifest.Files {
		candidate := canonicalRelativePath(manifest.Files[i].Path)
		if err := validateManagedPath(candidate); err != nil {
			return fmt.Errorf("manifest path %q: %w", manifest.Files[i].Path, err)
		}
		if strings.EqualFold(candidate, rel) {
			if entry != nil {
				return fmt.Errorf("duplicate apply helper manifest entry %q", rel)
			}
			entry = &manifest.Files[i]
		}
	}
	if entry == nil {
		return fmt.Errorf("apply helper %q is not manifest-managed", rel)
	}
	if err := validateInstallTarget(root, rel); err != nil {
		return fmt.Errorf("apply helper target is unsafe: %w", err)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if !inside(path, root) {
		return fmt.Errorf("apply helper escaped package root")
	}
	if err := verifyFile(path, entry.SizeBytes, entry.SHA256); err != nil {
		return fmt.Errorf("apply helper verification failed: %w", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("stat apply helper: %w", err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("apply helper is not executable")
		}
	}
	return nil
}

// ApplyPendingFromRunner binds the executable that owns recovery to the
// durable update state. Launchers must invoke a preserved copy under
// .updates/runner so a restart never guesses which updater can read a journal.
func ApplyPendingFromRunner(root, runnerPath string) (Result, error) {
	return applyPending(root, runnerPath, nil)
}

func applyPending(root, runnerPath string, hook applyHook) (Result, error) {
	root, paths, err := resolveRoot(root)
	if err != nil {
		return Result{}, updateErr("invalid_root", err)
	}
	state, stateExists, err := readState(paths.state)
	if err != nil {
		return Result{}, updateErr("state_invalid", err)
	}
	if stateExists && state.Status == "applying" {
		if _, err := rollback(root, paths, state, false); err != nil {
			return Result{}, updateErr("interrupted_recovery_failed", err)
		}
	}
	if stateExists && state.Status == "applied_pending_health" {
		pending, exists, err := readPending(paths.pending)
		if err != nil || !exists {
			if err == nil {
				err = fmt.Errorf("pending manifest is missing")
			}
			return Result{}, updateErr("pending_health_state_invalid", err)
		}
		if err := validatePending(pending); err != nil ||
			!sameVersion(pending.CurrentVersion, state.CurrentVersion) ||
			!sameVersion(pending.TargetVersion, state.TargetVersion) {
			if err == nil {
				err = fmt.Errorf("pending manifest versions do not match pending-health state")
			}
			return Result{}, updateErr("pending_health_state_invalid", err)
		}
		return Result{ContractVersion: ResultContract, Action: "apply-pending", Status: "applied_pending_health", CurrentVersion: state.CurrentVersion, TargetVersion: state.TargetVersion, HealthRequired: true, Message: "update already applied; health commit required"}, nil
	}
	pending, exists, err := readPending(paths.pending)
	if err != nil {
		return Result{}, updateErr("pending_invalid", err)
	}
	if stateExists && state.Status == "committed" {
		if exists {
			if err := validatePending(pending); err != nil {
				return Result{}, updateErr("pending_invalid", err)
			}
			if strings.EqualFold(strings.TrimSpace(pending.TargetVersion), strings.TrimSpace(state.TargetVersion)) {
				if _, err := Commit(root); err != nil {
					return Result{}, err
				}
				return Result{ContractVersion: ResultContract, Action: "apply-pending", Status: "no_pending", CurrentVersion: state.CurrentVersion, TargetVersion: state.TargetVersion, Message: "committed update cleanup complete"}, nil
			}
			if !sameVersion(pending.CurrentVersion, state.CurrentVersion) {
				return Result{}, updateErr("pending_invalid", fmt.Errorf("pending current version does not match committed state"))
			}
		} else {
			if _, err := Commit(root); err != nil {
				return Result{}, err
			}
			return Result{ContractVersion: ResultContract, Action: "apply-pending", Status: "no_pending", CurrentVersion: state.CurrentVersion, TargetVersion: state.TargetVersion, Message: "committed update cleanup complete"}, nil
		}
	}
	if !exists {
		return Result{ContractVersion: ResultContract, Action: "apply-pending", Status: "no_pending", HealthRequired: false, Message: "no pending update"}, nil
	}
	if err := validatePending(pending); err != nil {
		return Result{}, updateErr("pending_invalid", err)
	}
	boundRunnerPath, boundRunnerSHA, err := resolveRunnerIdentity(root, paths, runnerPath)
	if err != nil {
		return Result{}, updateErr("runner_identity_invalid", err)
	}
	asset, err := resolveAsset(root, paths.updates, pending.AssetPath)
	if err != nil {
		return Result{}, updateErr("asset_path_invalid", err)
	}
	if err := verifyFile(asset, -1, pending.SHA256); err != nil {
		return Result{}, updateErr("asset_verification_failed", err)
	}

	extracted, err := extractVerifiedArchive(asset, paths.extracted)
	if err != nil {
		return Result{}, updateErr("archive_rejected", err)
	}
	defer os.RemoveAll(extracted)
	packageRoot, err := findPackageRoot(extracted)
	if err != nil {
		return Result{}, updateErr("package_root_invalid", err)
	}
	validation, err := validateExtractedCandidate(
		root,
		packageRoot,
		pending.CurrentVersion,
		pending.TargetVersion,
		pending.RequiredFiles,
		comparePackageVersions(pending.CurrentVersion, DirectUpdateBaselineVersion) >= 0,
	)
	if err != nil {
		return Result{}, err
	}
	files := validation.Files
	removedManaged := validation.RemovedManaged
	files = append(files, installFile{Rel: ManifestName, Src: filepath.Join(packageRoot, ManifestName), Mode: 0o644})
	for _, rel := range removedManaged {
		if err := validateInstallTarget(root, rel); err != nil {
			return Result{}, updateErr("managed_target_unsafe", fmt.Errorf("%s: %w", rel, err))
		}
		files = append(files, installFile{Rel: rel, Remove: true})
	}

	state = State{
		ContractVersion: StateContract,
		Status:          "applying",
		CurrentVersion:  pending.CurrentVersion,
		TargetVersion:   pending.TargetVersion,
		RunnerPath:      boundRunnerPath,
		RunnerSHA256:    boundRunnerSHA,
		BackupDir:       filepath.ToSlash(filepath.Join("backups", safeSegment(pending.TargetVersion)+"-"+time.Now().UTC().Format("20060102T150405.000000000Z"))),
		UpdatedAt:       now(),
	}
	backupAbs := filepath.Join(paths.updates, filepath.FromSlash(state.BackupDir))
	if err := os.MkdirAll(backupAbs, 0o700); err != nil {
		return Result{}, updateErr("backup_failed", err)
	}
	for _, file := range files {
		target := filepath.Join(root, filepath.FromSlash(file.Rel))
		entry := JournalEntry{Path: file.Rel}
		info, statErr := os.Lstat(target)
		if statErr == nil {
			if !info.Mode().IsRegular() {
				os.RemoveAll(backupAbs)
				return Result{}, updateErr("target_not_regular", fmt.Errorf("%s", file.Rel))
			}
			entry.Existed = true
			entry.BackupPath = filepath.ToSlash(filepath.Join(state.BackupDir, file.Rel))
			if err := copyFile(target, filepath.Join(paths.updates, filepath.FromSlash(entry.BackupPath)), info.Mode().Perm()); err != nil {
				os.RemoveAll(backupAbs)
				return Result{}, updateErr("backup_failed", fmt.Errorf("%s: %w", file.Rel, err))
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			os.RemoveAll(backupAbs)
			return Result{}, updateErr("backup_failed", statErr)
		}
		state.Journal = append(state.Journal, entry)
	}
	if err := writeJSONAtomic(paths.state, state); err != nil {
		os.RemoveAll(backupAbs)
		return Result{}, updateErr("journal_write_failed", err)
	}

	for i, file := range files {
		if hook != nil {
			if err := hook(file.Rel, i); err != nil {
				_, rollbackErr := rollback(root, paths, state, false)
				if rollbackErr != nil {
					return Result{}, updateErr("apply_and_rollback_failed", fmt.Errorf("apply: %v; rollback: %w", err, rollbackErr))
				}
				return Result{}, updateErr("apply_failed", err)
			}
		}
		var applyErr error
		target := filepath.Join(root, filepath.FromSlash(file.Rel))
		if file.Remove {
			applyErr = os.Remove(target)
			if errors.Is(applyErr, os.ErrNotExist) {
				applyErr = nil
			}
		} else {
			applyErr = replaceFile(file.Src, target, file.Mode)
		}
		if applyErr != nil {
			_, rollbackErr := rollback(root, paths, state, false)
			if rollbackErr != nil {
				return Result{}, updateErr("apply_and_rollback_failed", fmt.Errorf("apply: %v; rollback: %w", applyErr, rollbackErr))
			}
			return Result{}, updateErr("apply_failed", fmt.Errorf("%s: %w", file.Rel, applyErr))
		}
	}
	state.Status = "applied_pending_health"
	state.UpdatedAt = now()
	if err := writeJSONAtomic(paths.state, state); err != nil {
		_, rollbackErr := rollback(root, paths, state, false)
		if rollbackErr != nil {
			return Result{}, updateErr("apply_and_rollback_failed", fmt.Errorf("state: %v; rollback: %w", err, rollbackErr))
		}
		return Result{}, updateErr("state_write_failed", err)
	}
	return Result{ContractVersion: ResultContract, Action: "apply-pending", Status: state.Status, CurrentVersion: pending.CurrentVersion, TargetVersion: pending.TargetVersion, HealthRequired: true, Message: "update applied; health commit required"}, nil
}

func Commit(root string) (Result, error) {
	_, paths, err := resolveRoot(root)
	if err != nil {
		return Result{}, updateErr("invalid_root", err)
	}
	state, exists, err := readState(paths.state)
	if err != nil {
		return Result{}, updateErr("state_invalid", err)
	}
	if !exists || (state.Status != "applied_pending_health" && state.Status != "committed") {
		return Result{}, updateErr("commit_not_ready", fmt.Errorf("state is not applied_pending_health"))
	}
	if state.Status == "applied_pending_health" {
		state.Status = "committed"
		state.CurrentVersion = state.TargetVersion
		state.UpdatedAt = now()
		// Commit durability must precede cleanup. If the process stops after this
		// write, a repeated commit only finishes cleanup and never reapplies.
		if err := writeJSONAtomic(paths.state, state); err != nil {
			return Result{}, updateErr("state_write_failed", err)
		}
	}
	pending, pendingExists, pendingErr := readPending(paths.pending)
	if pendingErr != nil {
		return Result{}, updateErr("commit_cleanup_failed", pendingErr)
	}
	if pendingExists && strings.EqualFold(strings.TrimSpace(pending.TargetVersion), strings.TrimSpace(state.TargetVersion)) {
		if err := os.Remove(paths.pending); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Result{}, updateErr("commit_cleanup_failed", err)
		}
	}
	if state.BackupDir != "" {
		if err := os.RemoveAll(filepath.Join(paths.updates, filepath.FromSlash(state.BackupDir))); err != nil {
			return Result{}, updateErr("commit_cleanup_failed", err)
		}
	}
	state.BackupDir = ""
	state.Journal = nil
	state.UpdatedAt = now()
	if err := writeJSONAtomic(paths.state, state); err != nil {
		return Result{}, updateErr("state_write_failed", err)
	}
	return Result{ContractVersion: ResultContract, Action: "commit", Status: "committed", CurrentVersion: state.CurrentVersion, TargetVersion: state.TargetVersion, Message: "update committed"}, nil
}

func Rollback(root string) (Result, error) {
	root, paths, err := resolveRoot(root)
	if err != nil {
		return Result{}, updateErr("invalid_root", err)
	}
	state, exists, err := readState(paths.state)
	if err != nil {
		return Result{}, updateErr("state_invalid", err)
	}
	if !exists || (state.Status != "applying" && state.Status != "applied_pending_health") {
		return Result{ContractVersion: ResultContract, Action: "rollback", Status: "nothing_to_rollback", Message: "no active update journal"}, nil
	}
	return rollback(root, paths, state, true)
}

func Status(root string) (Result, error) {
	_, paths, err := resolveRoot(root)
	if err != nil {
		return Result{}, updateErr("invalid_root", err)
	}
	state, exists, err := readState(paths.state)
	if err != nil {
		return Result{}, updateErr("state_invalid", err)
	}
	if !exists {
		return Result{ContractVersion: ResultContract, Action: "status", Status: "no_state", Message: "no update state"}, nil
	}
	return Result{ContractVersion: ResultContract, Action: "status", Status: state.Status, CurrentVersion: state.CurrentVersion, TargetVersion: state.TargetVersion, HealthRequired: state.Status == "applied_pending_health"}, nil
}

type rootPaths struct{ updates, pending, state, extracted string }

func resolveRoot(root string) (string, rootPaths, error) {
	if strings.TrimSpace(root) == "" {
		return "", rootPaths{}, fmt.Errorf("root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", rootPaths{}, err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", rootPaths{}, fmt.Errorf("package root is not a directory")
	}
	u := filepath.Join(abs, ".updates")
	return abs, rootPaths{updates: u, pending: filepath.Join(u, "pending-update.json"), state: filepath.Join(u, "update-state.json"), extracted: filepath.Join(u, "extracted")}, nil
}

func resolveRunnerIdentity(root string, paths rootPaths, runnerPath string) (string, string, error) {
	if strings.TrimSpace(runnerPath) == "" {
		// Direct library callers may omit a runner. The production CLI always
		// supplies one; this exception keeps transaction tests and embedders
		// independent from process executable layout.
		return "", "", nil
	}
	abs, err := filepath.Abs(runnerPath)
	if err != nil {
		return "", "", err
	}
	runnerRoot := filepath.Join(paths.updates, "runner")
	if !inside(abs, runnerRoot) {
		return "", "", fmt.Errorf("runner must be preserved under .updates/runner")
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("runner is not a regular file")
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", err
	}
	sha, err := fileSHA256(abs)
	if err != nil {
		return "", "", err
	}
	return filepath.ToSlash(rel), sha, nil
}

func readPending(path string) (Pending, bool, error) {
	var p Pending
	ok, err := readJSON(path, &p)
	return p, ok, err
}

func readState(path string) (State, bool, error) {
	var s State
	ok, err := readJSON(path, &s)
	if !ok || err != nil {
		var previous State
		previousOK, previousErr := readJSON(path+".previous", &previous)
		if previousOK && previousErr == nil {
			s, ok, err = previous, true, nil
		}
	}
	if err == nil && ok && s.ContractVersion != StateContract {
		return s, ok, fmt.Errorf("unsupported state contract %q", s.ContractVersion)
	}
	if err == nil && ok {
		if err := validateState(s, filepath.Dir(path)); err != nil {
			return s, ok, err
		}
	}
	return s, ok, err
}

func validateState(s State, updatesRoot string) error {
	allowed := map[string]bool{"applying": true, "applied_pending_health": true, "committed": true, "rolled_back": true}
	if !allowed[s.Status] {
		return fmt.Errorf("unsupported state status %q", s.Status)
	}
	if (strings.TrimSpace(s.RunnerPath) == "") != (strings.TrimSpace(s.RunnerSHA256) == "") {
		return fmt.Errorf("runner_path and runner_sha256 must be present together")
	}
	if s.RunnerPath != "" {
		clean := filepath.Clean(filepath.FromSlash(s.RunnerPath))
		runnerRoot := filepath.Join(updatesRoot, "runner")
		if filepath.IsAbs(clean) || !inside(filepath.Join(filepath.Dir(updatesRoot), clean), runnerRoot) {
			return fmt.Errorf("runner_path escapes .updates/runner")
		}
		if normalizeSHA(s.RunnerSHA256) == "" {
			return fmt.Errorf("runner_sha256 is invalid")
		}
	}
	if s.BackupDir != "" {
		clean := filepath.Clean(filepath.FromSlash(s.BackupDir))
		backupRoot := filepath.Join(updatesRoot, "backups")
		if filepath.IsAbs(clean) || !inside(filepath.Join(updatesRoot, clean), backupRoot) {
			return fmt.Errorf("backup_dir escapes backups")
		}
	}
	seen := map[string]bool{}
	for _, e := range s.Journal {
		if err := validateManagedPath(e.Path); err != nil {
			return fmt.Errorf("journal path: %w", err)
		}
		key := strings.ToLower(filepath.ToSlash(e.Path))
		if seen[key] {
			return fmt.Errorf("duplicate journal path %q", e.Path)
		}
		seen[key] = true
		if e.Existed {
			if e.BackupPath == "" || !inside(filepath.Join(updatesRoot, filepath.FromSlash(e.BackupPath)), filepath.Join(updatesRoot, "backups")) {
				return fmt.Errorf("journal backup path escapes backups")
			}
		} else if e.BackupPath != "" {
			return fmt.Errorf("new journal entry has backup path")
		}
	}
	return nil
}

func readJSON(path string, dst any) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 4*1024*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return true, err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return true, fmt.Errorf("trailing JSON data")
	}
	return true, nil
}

func validatePending(p Pending) error {
	if p.ContractVersion != PendingContract {
		return fmt.Errorf("unsupported contract %q", p.ContractVersion)
	}
	if strings.TrimSpace(p.CurrentVersion) == "" || strings.TrimSpace(p.TargetVersion) == "" {
		return fmt.Errorf("current_version and target_version are required")
	}
	if strings.TrimSpace(p.AssetPath) == "" {
		return fmt.Errorf("asset_path is required")
	}
	if normalizeSHA(p.SHA256) == "" {
		return fmt.Errorf("valid sha256 is required")
	}
	for _, rel := range p.RequiredFiles {
		if err := validateManagedPath(rel); err != nil {
			return fmt.Errorf("required file %q: %w", rel, err)
		}
	}
	return nil
}

func resolveAsset(root, updates, value string) (string, error) {
	p := filepath.FromSlash(value)
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if !inside(abs, updates) {
		return "", fmt.Errorf("asset must be under .updates")
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("asset is not a regular file")
	}
	return abs, nil
}

func extractVerifiedArchive(asset, extractedRoot string) (string, error) {
	if err := os.MkdirAll(extractedRoot, 0o700); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(extractedRoot, "apply-")
	if err != nil {
		return "", err
	}
	zr, err := zip.OpenReader(asset)
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	defer zr.Close()
	if len(zr.File) > archiveLimits.Files {
		os.RemoveAll(dir)
		return "", fmt.Errorf("zip contains too many entries")
	}
	seen := map[string]bool{}
	var total int64
	for _, zf := range zr.File {
		rel, err := validateZipName(zf.Name)
		if err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		key := strings.ToLower(rel)
		if seen[key] {
			os.RemoveAll(dir)
			return "", fmt.Errorf("duplicate zip path %q", rel)
		}
		seen[key] = true
		mode := zf.Mode()
		if !zf.FileInfo().IsDir() && !mode.IsRegular() {
			os.RemoveAll(dir)
			return "", fmt.Errorf("non-regular zip entry %q", rel)
		}
		if zf.UncompressedSize64 > uint64(archiveLimits.Bytes) || total > archiveLimits.Bytes-int64(zf.UncompressedSize64) {
			os.RemoveAll(dir)
			return "", fmt.Errorf("zip extracted size exceeds limit")
		}
		total += int64(zf.UncompressedSize64)
	}
	for _, zf := range zr.File {
		rel, _ := validateZipName(zf.Name)
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if !inside(target, dir) {
			os.RemoveAll(dir)
			return "", fmt.Errorf("zip path escaped extraction root")
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				os.RemoveAll(dir)
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		r, err := zf.Open()
		if err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, zf.Mode().Perm())
		if err != nil {
			r.Close()
			os.RemoveAll(dir)
			return "", err
		}
		n, copyErr := io.Copy(out, io.LimitReader(r, int64(zf.UncompressedSize64)+1))
		closeErr := out.Close()
		r.Close()
		if copyErr != nil || closeErr != nil || n != int64(zf.UncompressedSize64) {
			os.RemoveAll(dir)
			return "", fmt.Errorf("extract %q failed or size changed", rel)
		}
	}
	return dir, nil
}

func validateZipName(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, '\x00') {
		return "", fmt.Errorf("invalid empty or NUL zip path")
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "//") || hasDrivePrefix(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("absolute zip path %q", name)
	}
	parts := strings.Split(name, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", fmt.Errorf("traversal zip path %q", name)
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		return "", fmt.Errorf("invalid zip path %q", name)
	}
	return strings.Join(clean, "/"), nil
}

func findPackageRoot(extracted string) (string, error) {
	var roots []string
	err := filepath.WalkDir(extracted, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == ManifestName {
			roots = append(roots, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(roots) != 1 {
		return "", fmt.Errorf("expected exactly one package manifest, found %d", len(roots))
	}
	return roots[0], nil
}

func readPackageManifest(path string) (packageManifest, error) {
	var m packageManifest
	f, err := os.Open(path)
	if err != nil {
		return m, err
	}
	defer f.Close()
	reader := bufio.NewReader(io.LimitReader(f, 16*1024*1024))
	prefix, _ := reader.Peek(3)
	if bytes.Equal(prefix, []byte{0xef, 0xbb, 0xbf}) {
		if _, err := reader.Discard(3); err != nil {
			return m, err
		}
	}
	if err := json.NewDecoder(reader).Decode(&m); err != nil {
		return m, err
	}
	if m.SchemaVersion != "archive-center.package-file-manifest.v1" {
		return m, fmt.Errorf("unsupported schema %q", m.SchemaVersion)
	}
	if len(m.Files) == 0 {
		return m, fmt.Errorf("manifest has no managed files")
	}
	return m, nil
}

func verifyNewPackage(root string, m packageManifest, required []string) ([]installFile, error) {
	seen := map[string]bool{}
	files := make([]installFile, 0, len(m.Files))
	for _, mf := range m.Files {
		rel := canonicalRelativePath(mf.Path)
		if err := validateManagedPath(rel); err != nil {
			return nil, fmt.Errorf("%q: %w", rel, err)
		}
		key := strings.ToLower(rel)
		if seen[key] {
			return nil, fmt.Errorf("duplicate manifest path %q", rel)
		}
		seen[key] = true
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not regular", rel)
		}
		if err := verifyFile(path, mf.SizeBytes, mf.SHA256); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		files = append(files, installFile{Rel: rel, Src: path, Mode: managedInstallMode(rel, info.Mode().Perm(), runtime.GOOS)})
	}
	for _, req := range required {
		if !seen[strings.ToLower(canonicalRelativePath(req))] {
			return nil, fmt.Errorf("required file %q is not manifest-managed", req)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	return files, nil
}

func verifyCurrentPackage(root string) (map[string]manifestFile, string, error) {
	m, err := readPackageManifest(filepath.Join(root, ManifestName))
	if err != nil {
		return nil, "", fmt.Errorf("current package manifest: %w", err)
	}
	seen := make(map[string]manifestFile, len(m.Files))
	for _, mf := range m.Files {
		rel := canonicalRelativePath(mf.Path)
		if err := validateManagedPath(rel); err != nil {
			return nil, "", err
		}
		key := strings.ToLower(rel)
		if _, exists := seen[key]; exists {
			return nil, "", fmt.Errorf("duplicate current manifest path %q", rel)
		}
		mf.Path = rel
		seen[key] = mf
		if err := validateInstallTarget(root, rel); err != nil {
			return nil, "", fmt.Errorf("%s: %w", rel, err)
		}
		if err := verifyFile(filepath.Join(root, filepath.FromSlash(rel)), mf.SizeBytes, mf.SHA256); err != nil {
			return nil, "", fmt.Errorf("%s: %w", rel, err)
		}
	}
	return seen, m.PackageVersion, nil
}

func validateInstallTarget(root, rel string) error {
	parts := strings.Split(canonicalRelativePath(rel), "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("target ancestor is not a regular directory")
		}
	}
	return nil
}

func validateExtractedCandidate(
	root string,
	packageRoot string,
	currentVersion string,
	targetVersion string,
	requiredFiles []string,
	requireDirect bool,
) (candidateValidation, error) {
	newManifest, err := readPackageManifest(filepath.Join(packageRoot, ManifestName))
	if err != nil {
		return candidateValidation{}, updateErr("package_manifest_invalid", err)
	}
	if !sameVersion(newManifest.PackageVersion, targetVersion) {
		return candidateValidation{}, updateErr("package_manifest_invalid", fmt.Errorf("candidate package_version %q does not match target %q", newManifest.PackageVersion, targetVersion))
	}
	files, err := verifyNewPackage(packageRoot, newManifest, requiredFiles)
	if err != nil {
		return candidateValidation{}, updateErr("package_verification_failed", err)
	}
	currentManaged, currentPackageVersion, err := verifyCurrentPackage(root)
	if err != nil {
		return candidateValidation{}, updateErr("managed_target_modified", err)
	}
	if !sameVersion(currentPackageVersion, currentVersion) {
		return candidateValidation{}, updateErr("managed_target_modified", fmt.Errorf("installed package_version %q does not match current %q", currentPackageVersion, currentVersion))
	}
	if requireDirect {
		if comparePackageVersions(currentVersion, DirectUpdateBaselineVersion) < 0 {
			return candidateValidation{}, updateErr("source_version_unsupported", fmt.Errorf("direct updates require source version %s or newer", DirectUpdateBaselineVersion))
		}
		if comparePackageVersions(targetVersion, currentVersion) <= 0 {
			return candidateValidation{}, updateErr("target_version_not_newer", fmt.Errorf("target version %q is not newer than current %q", targetVersion, currentVersion))
		}
	}
	newManaged := make(map[string]manifestFile, len(newManifest.Files))
	for _, file := range newManifest.Files {
		rel := canonicalRelativePath(file.Path)
		file.Path = rel
		newManaged[strings.ToLower(rel)] = file
	}
	if requireDirect {
		if err := validatePackageReleaseReadiness(packageRoot, targetVersion, newManaged); err != nil {
			return candidateValidation{}, updateErr("package_release_unverified", err)
		}
	}
	removedManaged, err := validateDatabaseMigrationUpdate(packageRoot, currentVersion, targetVersion, currentManaged, newManaged, requireDirect)
	if err != nil {
		code := "database_migration_update_unsupported"
		if hasManagedFileRemoval(currentManaged, newManaged) {
			code = "managed_file_removal_unsupported"
		}
		return candidateValidation{}, updateErr(code, err)
	}
	for _, file := range files {
		if err := validateInstallTarget(root, file.Rel); err != nil {
			return candidateValidation{}, updateErr("managed_target_unsafe", fmt.Errorf("%s: %w", file.Rel, err))
		}
	}
	return candidateValidation{Files: files, RemovedManaged: removedManaged}, nil
}

// validateDatabaseMigrationUpdate never attempts to decide whether arbitrary
// SQL is safe. A package that changes the migration inventory must carry an
// authenticated, versioned inventory contract that names the exact current
// package fingerprint and exact candidate fingerprint. This permits a reviewed
// release plan to contain CREATE, foreign-key, index, or MODIFY statements
// without granting a blanket SQL-text exception.
func validateDatabaseMigrationUpdate(
	packageRoot string,
	currentVersion string,
	targetVersion string,
	current map[string]manifestFile,
	next map[string]manifestFile,
	requireDirect bool,
) ([]string, error) {
	currentInventory := migrationInventory(current)
	nextInventory := migrationInventory(next)
	removedManaged := managedFileRemovals(current, next)
	if !requireDirect && migrationInventoriesEqual(currentInventory, nextInventory) && len(removedManaged) == 0 {
		return nil, nil
	}
	contractFile, present := next[strings.ToLower(MigrationUpdateManifestName)]
	if !present {
		return nil, fmt.Errorf("%s is required when migration files change or managed files are removed", MigrationUpdateManifestName)
	}
	contractPath := filepath.Join(packageRoot, MigrationUpdateManifestName)
	if err := verifyFile(contractPath, contractFile.SizeBytes, contractFile.SHA256); err != nil {
		return nil, fmt.Errorf("migration update contract verification failed: %w", err)
	}
	var contract migrationUpdateManifest
	ok, err := readJSON(contractPath, &contract)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("migration update contract is missing")
	}
	if contract.ContractVersion != MigrationUpdateContract && contract.ContractVersion != MigrationUpdateContractV2 {
		return nil, fmt.Errorf("unsupported migration update contract %q", contract.ContractVersion)
	}
	if requireDirect && contract.ContractVersion != MigrationUpdateContractV2 {
		return nil, fmt.Errorf("direct updates require migration contract %q", MigrationUpdateContractV2)
	}
	if !sameVersion(contract.TargetVersion, targetVersion) {
		return nil, fmt.Errorf("migration target version %q does not match pending target %q", contract.TargetVersion, targetVersion)
	}
	contractTarget, err := normalizeMigrationManifestFiles(contract.Target)
	if err != nil {
		return nil, fmt.Errorf("target inventory: %w", err)
	}
	if !migrationInventoriesEqual(contractTarget, nextInventory) {
		return nil, fmt.Errorf("target migration inventory does not match candidate package")
	}
	if contract.ContractVersion == MigrationUpdateContractV2 {
		if contract.ManagedFiles != CompleteManagedPackage {
			return nil, fmt.Errorf("managed_files must declare %q", CompleteManagedPackage)
		}
		if contract.DatabasePolicy != ExpandFirstCompatibility {
			return nil, fmt.Errorf("database_policy must declare %q", ExpandFirstCompatibility)
		}
		if len(contract.Sources) != 0 {
			return nil, fmt.Errorf("v2 complete-manifest contract must not carry historical source inventories")
		}
		if requireDirect {
			if strings.TrimSpace(contract.MinimumSourceVersion) == "" {
				return nil, fmt.Errorf("minimum_source_version is required")
			}
			if !sameVersion(contract.MinimumSourceVersion, DirectUpdateBaselineVersion) {
				return nil, fmt.Errorf("minimum_source_version must declare %q", DirectUpdateBaselineVersion)
			}
			if comparePackageVersions(currentVersion, contract.MinimumSourceVersion) < 0 {
				return nil, fmt.Errorf("current version %q is below minimum source %q", currentVersion, contract.MinimumSourceVersion)
			}
			if !contract.DirectUpdateSupported {
				return nil, fmt.Errorf("direct_update_supported must be true")
			}
			if contract.MigrationInventory != CumulativeMigrationInventory {
				return nil, fmt.Errorf("migration_inventory must declare %q", CumulativeMigrationInventory)
			}
			if err := validateCumulativeMigrationInventory(currentInventory, nextInventory); err != nil {
				return nil, err
			}
		}
		schemaTool := "bin/mariadb-schema"
		if runtime.GOOS == "windows" {
			schemaTool += ".exe"
		}
		if _, present := contractTarget[schemaTool]; !present {
			return nil, fmt.Errorf("target migration inventory must include %q", schemaTool)
		}
		return removedManaged, nil
	}

	removedManagedComparison, err := normalizeRemovedManagedPaths(removedManaged)
	if err != nil {
		return nil, fmt.Errorf("computed removals: %w", err)
	}
	matchedSource := false
	for _, source := range contract.Sources {
		if !sameVersion(source.Version, currentVersion) {
			continue
		}
		sourceInventory, sourceErr := normalizeMigrationManifestFiles(source.Files)
		if sourceErr != nil {
			return nil, fmt.Errorf("source %q inventory: %w", source.Version, sourceErr)
		}
		sourceRemovals, sourceErr := normalizeRemovedManagedPaths(source.RemovedManagedPaths)
		if sourceErr != nil {
			return nil, fmt.Errorf("source %q removals: %w", source.Version, sourceErr)
		}
		if migrationInventoriesEqual(sourceInventory, currentInventory) &&
			stringSlicesEqual(sourceRemovals, removedManagedComparison) {
			matchedSource = true
			break
		}
	}
	if !matchedSource {
		return nil, fmt.Errorf("no source inventory and removal plan matches current version %q and installed package", currentVersion)
	}
	return removedManaged, nil
}

func hasManagedFileRemoval(current, next map[string]manifestFile) bool {
	return len(managedFileRemovals(current, next)) > 0
}

func managedFileRemovals(current, next map[string]manifestFile) []string {
	removed := make([]string, 0)
	for key, file := range current {
		if _, present := next[key]; !present {
			removed = append(removed, canonicalRelativePath(file.Path))
		}
	}
	sort.Strings(removed)
	return removed
}

func normalizeRemovedManagedPaths(paths []string) ([]string, error) {
	normalized := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		rel := strings.ToLower(canonicalRelativePath(path))
		if err := validateManagedPath(rel); err != nil {
			return nil, fmt.Errorf("%q: %w", path, err)
		}
		if _, duplicate := seen[rel]; duplicate {
			return nil, fmt.Errorf("duplicate removed managed path %q", path)
		}
		seen[rel] = struct{}{}
		normalized = append(normalized, rel)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func migrationInventory(files map[string]manifestFile) map[string]migrationManifestFile {
	out := map[string]migrationManifestFile{}
	for rel, file := range files {
		rel = strings.ToLower(canonicalRelativePath(rel))
		if !isMigrationInventoryPath(rel) {
			continue
		}
		out[rel] = migrationManifestFile{Path: rel, SizeBytes: file.SizeBytes, SHA256: normalizeSHA(file.SHA256)}
	}
	return out
}

func normalizeMigrationManifestFiles(files []migrationManifestFile) (map[string]migrationManifestFile, error) {
	out := make(map[string]migrationManifestFile, len(files))
	for _, file := range files {
		rel := strings.ToLower(canonicalRelativePath(file.Path))
		if !isMigrationInventoryPath(rel) {
			return nil, fmt.Errorf("unsupported migration path %q", file.Path)
		}
		if err := validateManagedPath(rel); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if file.SizeBytes <= 0 || normalizeSHA(file.SHA256) == "" {
			return nil, fmt.Errorf("invalid fingerprint for %q", rel)
		}
		if _, duplicate := out[rel]; duplicate {
			return nil, fmt.Errorf("duplicate migration path %q", rel)
		}
		file.Path = rel
		file.SHA256 = normalizeSHA(file.SHA256)
		out[rel] = file
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("migration inventory is empty")
	}
	return out, nil
}

func isMigrationInventoryPath(rel string) bool {
	rel = strings.ToLower(canonicalRelativePath(rel))
	return (strings.HasPrefix(rel, "migrations/") && strings.HasSuffix(rel, ".sql")) ||
		rel == "bin/mariadb-schema" || rel == "bin/mariadb-schema.exe"
}

func migrationInventoriesEqual(left, right map[string]migrationManifestFile) bool {
	if len(left) != len(right) {
		return false
	}
	for rel, leftFile := range left {
		rightFile, ok := right[rel]
		if !ok || leftFile.SizeBytes != rightFile.SizeBytes || normalizeSHA(leftFile.SHA256) != normalizeSHA(rightFile.SHA256) {
			return false
		}
	}
	return true
}

func validateCumulativeMigrationInventory(current, next map[string]migrationManifestFile) error {
	for rel, existing := range current {
		if !strings.HasPrefix(rel, "migrations/") || !strings.HasSuffix(rel, ".sql") {
			continue
		}
		candidate, present := next[rel]
		if !present {
			return fmt.Errorf("cumulative migration inventory omitted existing migration %q", rel)
		}
		if candidate.SizeBytes != existing.SizeBytes || normalizeSHA(candidate.SHA256) != normalizeSHA(existing.SHA256) {
			return fmt.Errorf("cumulative migration inventory changed existing migration %q", rel)
		}
	}
	return validateSequentialMigrationInventory(next)
}

func validateSequentialMigrationInventory(inventory map[string]migrationManifestFile) error {
	revisions := map[int]string{}
	maxRevision := 0
	for rel := range inventory {
		if !strings.HasPrefix(rel, "migrations/") || !strings.HasSuffix(rel, ".sql") {
			continue
		}
		name := strings.TrimPrefix(rel, "migrations/")
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 || len(parts[0]) != 3 {
			return fmt.Errorf("migration %q must start with a three-digit sequential revision", rel)
		}
		revision, err := strconv.Atoi(parts[0])
		if err != nil || revision < 1 {
			return fmt.Errorf("migration %q has an invalid revision", rel)
		}
		if existing, duplicate := revisions[revision]; duplicate {
			return fmt.Errorf("migration revision %03d is duplicated by %q and %q", revision, existing, rel)
		}
		revisions[revision] = rel
		if revision > maxRevision {
			maxRevision = revision
		}
	}
	if maxRevision == 0 {
		return fmt.Errorf("cumulative migration inventory contains no SQL migrations")
	}
	for revision := 1; revision <= maxRevision; revision++ {
		if _, present := revisions[revision]; !present {
			return fmt.Errorf("cumulative migration inventory is missing revision %03d", revision)
		}
	}
	return nil
}

func validatePackageReleaseReadiness(packageRoot, targetVersion string, managed map[string]manifestFile) error {
	if _, ok := managed[strings.ToLower(PackageReleaseStatusName)]; !ok {
		return fmt.Errorf("candidate does not manage %s", PackageReleaseStatusName)
	}
	var release packageReleaseManifest
	ok, err := readJSON(filepath.Join(packageRoot, PackageReleaseStatusName), &release)
	if err != nil {
		return fmt.Errorf("read %s: %w", PackageReleaseStatusName, err)
	}
	if !ok || !release.ReleaseReady {
		return fmt.Errorf("%s does not certify release_ready=true", PackageReleaseStatusName)
	}
	if release.ContractVersion != PackageReleaseStatusContract || !sameVersion(release.TargetVersion, targetVersion) {
		return fmt.Errorf("%s does not certify target version %q", PackageReleaseStatusName, targetVersion)
	}
	if release.AutomaticUpdateApply != nil && !*release.AutomaticUpdateApply {
		return fmt.Errorf("%s disables automatic_update_apply", PackageReleaseStatusName)
	}
	return nil
}

func sameVersion(left, right string) bool {
	normalize := func(value string) string {
		return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "v")
	}
	return normalize(left) != "" && normalize(left) == normalize(right)
}

func comparePackageVersions(left, right string) int {
	lv, lp := parsePackageVersion(left)
	rv, rp := parsePackageVersion(right)
	for i := range lv {
		if lv[i] > rv[i] {
			return 1
		}
		if lv[i] < rv[i] {
			return -1
		}
	}
	if lp == rp {
		return 0
	}
	if lp == "" {
		return 1
	}
	if rp == "" {
		return -1
	}
	if lp > rp {
		return 1
	}
	return -1
}

func parsePackageVersion(value string) ([3]int, string) {
	clean := strings.TrimSpace(value)
	clean = strings.TrimPrefix(strings.TrimPrefix(clean, "v"), "V")
	if plus := strings.Index(clean, "+"); plus >= 0 {
		clean = clean[:plus]
	}
	pre := ""
	if dash := strings.Index(clean, "-"); dash >= 0 {
		pre = strings.ToLower(strings.TrimSpace(clean[dash+1:]))
		clean = clean[:dash]
	}
	parts := strings.Split(clean, ".")
	var parsed [3]int
	for i := 0; i < len(parts) && i < len(parsed); i++ {
		parsed[i], _ = strconv.Atoi(strings.TrimSpace(parts[i]))
	}
	return parsed, pre
}

func managedInstallMode(rel string, archived fs.FileMode, goos string) fs.FileMode {
	if strings.EqualFold(strings.TrimSpace(goos), "windows") {
		return archived.Perm()
	}
	rel = strings.ToLower(canonicalRelativePath(rel))
	if strings.HasPrefix(rel, "bin/") || strings.HasSuffix(rel, ".sh") || strings.HasSuffix(rel, ".command") {
		return 0o755
	}
	if archived.Perm() == 0 {
		return 0o644
	}
	return archived.Perm()
}

func validateManagedPath(rel string) error {
	if rel == "" || strings.ContainsRune(rel, '\x00') {
		return fmt.Errorf("empty or NUL path")
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "//") || hasDrivePrefix(rel) || filepath.VolumeName(rel) != "" {
		return fmt.Errorf("absolute path")
	}
	parts := strings.Split(rel, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return fmt.Errorf("unclean path")
		}
	}
	first := strings.ToLower(parts[0])
	protected := map[string]bool{".runtime": true, ".updates": true, "data": true, "cache": true, "caches": true, "database": true, "databases": true, "db": true, "vector": true, "vectors": true, "chromadb": true, "mariadb": true, "secrets": true}
	if protected[first] {
		return fmt.Errorf("protected path")
	}
	base := strings.ToLower(parts[len(parts)-1])
	managedEnvironmentTemplate := len(parts) == 1 && (base == ".env.full.example" || base == ".env.source.example")
	if !managedEnvironmentTemplate && (base == ".env" || strings.HasPrefix(base, ".env.")) {
		return fmt.Errorf("protected environment file")
	}
	for _, s := range []string{".db", ".sqlite", ".sqlite3", ".pem", ".key"} {
		if strings.HasSuffix(base, s) {
			return fmt.Errorf("protected data or secret file")
		}
	}
	return nil
}

func canonicalRelativePath(v string) string {
	return strings.ReplaceAll(strings.TrimSpace(v), "\\", "/")
}

func verifyFile(path string, size int64, want string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if size >= 0 && info.Size() != size {
		return fmt.Errorf("size mismatch")
	}
	got, err := fileSHA256(path)
	if err != nil {
		return err
	}
	want = normalizeSHA(want)
	if want == "" || got != want {
		return fmt.Errorf("sha256 mismatch")
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func normalizeSHA(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if len(v) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(v); err != nil {
		return ""
	}
	return v
}

func rollback(root string, paths rootPaths, state State, clearPending bool) (Result, error) {
	for i := len(state.Journal) - 1; i >= 0; i-- {
		e := state.Journal[i]
		if err := validateManagedPath(e.Path); err != nil {
			return Result{}, err
		}
		target := filepath.Join(root, filepath.FromSlash(e.Path))
		if e.Existed {
			backup := filepath.Join(paths.updates, filepath.FromSlash(e.BackupPath))
			info, err := os.Lstat(backup)
			if err != nil {
				return Result{}, err
			}
			if err := replaceFile(backup, target, info.Mode().Perm()); err != nil {
				return Result{}, err
			}
		} else {
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return Result{}, err
			}
		}
	}
	state.Status = "rolled_back"
	state.TargetVersion = ""
	state.UpdatedAt = now()
	if err := writeJSONAtomic(paths.state, state); err != nil {
		return Result{}, err
	}
	if clearPending {
		_ = os.Remove(paths.pending)
	}
	if state.BackupDir != "" {
		_ = os.RemoveAll(filepath.Join(paths.updates, filepath.FromSlash(state.BackupDir)))
	}
	state.BackupDir = ""
	state.Journal = nil
	state.UpdatedAt = now()
	if err := writeJSONAtomic(paths.state, state); err != nil {
		return Result{}, err
	}
	return Result{ContractVersion: ResultContract, Action: "rollback", Status: "rolled_back", CurrentVersion: state.CurrentVersion, Message: "update rolled back"}, nil
}

func replaceFile(src, target string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp := target + ".update-tmp"
	_ = os.Remove(tmp)
	if err := copyFile(src, tmp, mode); err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		os.Remove(dst)
		if copyErr != nil {
			return copyErr
		}
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	previous := path + ".previous"
	_ = os.Remove(previous)
	if err := os.Rename(path, previous); err != nil && !errors.Is(err, os.ErrNotExist) {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Rename(previous, path)
		os.Remove(tmp)
		return err
	}
	_ = os.Remove(previous)
	return nil
}

func inside(path, root string) bool {
	p, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(r, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func safeSegment(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "update"
	}
	return b.String()
}
func hasDrivePrefix(v string) bool {
	return len(v) >= 2 && ((v[0] >= 'a' && v[0] <= 'z') || (v[0] >= 'A' && v[0] <= 'Z')) && v[1] == ':'
}
func now() string                            { return time.Now().UTC().Format(time.RFC3339Nano) }
func updateErr(code string, err error) error { return &UpdateError{Code: code, Err: err} }
