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
	"strings"
	"time"
)

const (
	PendingContract = "archive-center.pending-update.v1"
	StateContract   = "archive-center.update-state.v1"
	ResultContract  = "archive-center.updater-result.v1"
	ManifestName    = "PACKAGE_FILE_MANIFEST.json"
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
	SchemaVersion string         `json:"schema_version"`
	Files         []manifestFile `json:"files"`
}

type manifestFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type installFile struct {
	Rel  string
	Src  string
	Mode fs.FileMode
}

type applyHook func(relativePath string, index int) error

func ApplyPending(root string) (Result, error) { return applyPending(root, nil) }

func applyPending(root string, hook applyHook) (Result, error) {
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
		if _, exists, err := readPending(paths.pending); err != nil || !exists {
			if err == nil {
				err = fmt.Errorf("pending manifest is missing")
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
		if !exists {
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
	newManifest, err := readPackageManifest(filepath.Join(packageRoot, ManifestName))
	if err != nil {
		return Result{}, updateErr("package_manifest_invalid", err)
	}
	files, err := verifyNewPackage(packageRoot, newManifest, pending.RequiredFiles)
	if err != nil {
		return Result{}, updateErr("package_verification_failed", err)
	}
	currentManaged, err := verifyCurrentPackage(root)
	if err != nil {
		return Result{}, updateErr("managed_target_modified", err)
	}
	newManaged := make(map[string]manifestFile, len(newManifest.Files))
	for _, file := range newManifest.Files {
		newManaged[strings.ToLower(canonicalRelativePath(file.Path))] = file
	}
	for rel, currentFile := range currentManaged {
		newFile, present := newManaged[rel]
		if !present {
			return Result{}, updateErr("managed_file_removal_unsupported", fmt.Errorf("new package omits managed path %q", rel))
		}
		if isDatabaseMigrationPath(rel) && normalizeSHA(currentFile.SHA256) != normalizeSHA(newFile.SHA256) {
			return Result{}, updateErr("database_migration_update_unsupported", fmt.Errorf("automatic update changes %q", rel))
		}
	}
	for rel := range newManaged {
		if isDatabaseMigrationPath(rel) {
			if _, present := currentManaged[rel]; !present {
				return Result{}, updateErr("database_migration_update_unsupported", fmt.Errorf("automatic update adds %q", rel))
			}
		}
	}
	for _, file := range files {
		if err := validateInstallTarget(root, file.Rel); err != nil {
			return Result{}, updateErr("managed_target_unsafe", fmt.Errorf("%s: %w", file.Rel, err))
		}
	}
	files = append(files, installFile{Rel: ManifestName, Src: filepath.Join(packageRoot, ManifestName), Mode: 0o644})

	state = State{
		ContractVersion: StateContract,
		Status:          "applying",
		CurrentVersion:  pending.CurrentVersion,
		TargetVersion:   pending.TargetVersion,
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
		if err := replaceFile(file.Src, filepath.Join(root, filepath.FromSlash(file.Rel)), file.Mode); err != nil {
			_, rollbackErr := rollback(root, paths, state, false)
			if rollbackErr != nil {
				return Result{}, updateErr("apply_and_rollback_failed", fmt.Errorf("apply: %v; rollback: %w", err, rollbackErr))
			}
			return Result{}, updateErr("apply_failed", fmt.Errorf("%s: %w", file.Rel, err))
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

func verifyCurrentPackage(root string) (map[string]manifestFile, error) {
	m, err := readPackageManifest(filepath.Join(root, ManifestName))
	if err != nil {
		return nil, fmt.Errorf("current package manifest: %w", err)
	}
	seen := make(map[string]manifestFile, len(m.Files))
	for _, mf := range m.Files {
		rel := canonicalRelativePath(mf.Path)
		if err := validateManagedPath(rel); err != nil {
			return nil, err
		}
		key := strings.ToLower(rel)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate current manifest path %q", rel)
		}
		seen[key] = mf
		if err := validateInstallTarget(root, rel); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if err := verifyFile(filepath.Join(root, filepath.FromSlash(rel)), mf.SizeBytes, mf.SHA256); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
	}
	return seen, nil
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

func isDatabaseMigrationPath(rel string) bool {
	rel = strings.ToLower(canonicalRelativePath(rel))
	return rel == "bin/mariadb-schema.exe" || rel == "bin/mariadb-schema" || strings.HasPrefix(rel, "migrations/")
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
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	want = normalizeSHA(want)
	if want == "" || got != want {
		return fmt.Errorf("sha256 mismatch")
	}
	return nil
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
