package httpapi

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var updateHTTPClient = http.DefaultClient
var updateRestartProcess = func() { os.Exit(2) }

type githubReleaseResponse struct {
	TagName     string              `json:"tag_name"`
	Name        string              `json:"name"`
	HTMLURL     string              `json:"html_url"`
	Draft       bool                `json:"draft"`
	Prerelease  bool                `json:"prerelease"`
	PublishedAt string              `json:"published_at"`
	Assets      []githubAssetRecord `json:"assets"`
}

type githubAssetRecord struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

type updateAssetInfo struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

type updateCheckResult struct {
	Status              string           `json:"status"`
	PolicyVersion       string           `json:"policy_version"`
	Repository          string           `json:"repository"`
	Channel             string           `json:"channel"`
	CurrentVersion      string           `json:"current_version"`
	LatestVersion       string           `json:"latest_version"`
	UpdateAvailable     bool             `json:"update_available"`
	Platform            string           `json:"platform"`
	SelectedAsset       *updateAssetInfo `json:"selected_asset,omitempty"`
	SHA256Source        string           `json:"sha256_source,omitempty"`
	ApplySupported      bool             `json:"apply_supported"`
	DownloadSupported   bool             `json:"download_supported"`
	ReleaseTag          string           `json:"release_tag"`
	ReleaseName         string           `json:"release_name"`
	ReleaseURL          string           `json:"release_url"`
	ReleasePrerelease   bool             `json:"release_prerelease"`
	ReleasePublishedAt  string           `json:"release_published_at"`
	CompatibleAssetNote string           `json:"compatible_asset_note,omitempty"`
}

type updateDownloadRequest struct {
	CurrentVersion string `json:"current_version"`
	Platform       string `json:"platform"`
	AssetName      string `json:"asset_name"`
	ExpectedSHA256 string `json:"expected_sha256"`
	Apply          bool   `json:"apply"`
	RestartService *bool  `json:"restart_service,omitempty"`
}

func (s *Server) registerUpdateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /update/check", s.handleUpdateCheck)
	mux.HandleFunc("POST /update/download", s.handleUpdateDownload)
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.UpdateEnabled {
		writeError(w, http.StatusServiceUnavailable, "update_disabled", "update checks are disabled")
		return
	}
	current := strings.TrimSpace(r.URL.Query().Get("current_version"))
	if current == "" {
		current = s.Cfg.BuildVersion
	}
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	result, err := s.resolveLatestUpdate(r.Context(), current, platform)
	if err != nil {
		writeError(w, http.StatusBadGateway, "update_check_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUpdateDownload(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.UpdateEnabled {
		writeError(w, http.StatusServiceUnavailable, "update_disabled", "update downloads are disabled")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req updateDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeBadRequest(w, "invalid update download request: "+err.Error())
		return
	}
	current := strings.TrimSpace(req.CurrentVersion)
	if current == "" {
		current = s.Cfg.BuildVersion
	}
	result, err := s.resolveLatestUpdate(r.Context(), current, strings.TrimSpace(req.Platform))
	if err != nil {
		writeError(w, http.StatusBadGateway, "update_check_failed", err.Error())
		return
	}
	if req.Apply && !result.UpdateAvailable {
		writeBadRequest(w, "no newer update is available to apply")
		return
	}
	asset := result.SelectedAsset
	if strings.TrimSpace(req.AssetName) != "" {
		asset = nil
		for _, candidate := range result.assetsForInternalUse() {
			if candidate.Name == req.AssetName {
				assetCopy := candidate
				asset = &assetCopy
				break
			}
		}
		if asset == nil {
			writeBadRequest(w, "requested update asset is not part of the latest release")
			return
		}
	}
	if asset == nil || strings.TrimSpace(asset.DownloadURL) == "" {
		writeError(w, http.StatusNotFound, "update_asset_not_found", "no compatible update asset was found")
		return
	}
	expected := normalizeSHA256(req.ExpectedSHA256)
	if expected == "" {
		expected = normalizeSHA256(asset.SHA256)
	}
	if expected == "" {
		writeError(w, http.StatusBadGateway, "update_sha256_missing", "selected update asset has no SHA256 entry")
		return
	}
	staged, err := s.downloadAndStageUpdateAsset(r.Context(), result.LatestVersion, *asset, expected)
	if err != nil {
		writeError(w, http.StatusBadGateway, "update_download_failed", err.Error())
		return
	}
	staged["apply_supported"] = updateApplySupported(result.Platform)
	if req.Apply {
		restartService := true
		if req.RestartService != nil {
			restartService = *req.RestartService
		}
		applied, err := s.applyStagedUpdateAsset(result.ReleaseTag, result.LatestVersion, staged, restartService)
		if err != nil {
			writeError(w, http.StatusBadGateway, "update_apply_failed", err.Error())
			return
		}
		for k, v := range applied {
			staged[k] = v
		}
	}
	writeJSON(w, http.StatusOK, staged)
}

func (s *Server) resolveLatestUpdate(ctx context.Context, currentVersion, platform string) (*updateCheckResult, error) {
	repo := strings.TrimSpace(s.Cfg.UpdateGitHubRepo)
	if !isSafeGitHubRepo(repo) {
		return nil, fmt.Errorf("invalid update repository %q", repo)
	}
	if strings.TrimSpace(platform) == "" {
		platform = detectUpdatePlatform(runtime.GOOS, runtime.GOARCH)
	} else {
		platform = normalizeUpdatePlatform(platform)
	}
	release, err := fetchGitHubLatestRelease(ctx, repo)
	if err != nil {
		return nil, err
	}
	latest := versionFromTag(release.TagName)
	shaMap, shaSource := fetchReleaseSHA256Map(ctx, release.Assets)
	selected := selectUpdateAsset(platform, release.Assets, shaMap)
	updateAvailable := compareVersions(latest, currentVersion) > 0
	result := &updateCheckResult{
		Status:             "ok",
		PolicyVersion:      "update-check.v1",
		Repository:         repo,
		Channel:            strings.TrimSpace(s.Cfg.UpdateChannel),
		CurrentVersion:     strings.TrimSpace(currentVersion),
		LatestVersion:      latest,
		UpdateAvailable:    updateAvailable,
		Platform:           platform,
		SelectedAsset:      selected,
		SHA256Source:       shaSource,
		ApplySupported:     updateApplySupported(platform),
		DownloadSupported:  selected != nil && selected.SHA256 != "",
		ReleaseTag:         release.TagName,
		ReleaseName:        release.Name,
		ReleaseURL:         release.HTMLURL,
		ReleasePrerelease:  release.Prerelease,
		ReleasePublishedAt: release.PublishedAt,
	}
	if selected == nil {
		result.CompatibleAssetNote = "no asset name matched the requested platform"
	}
	return result, nil
}

func (r *updateCheckResult) assetsForInternalUse() []updateAssetInfo {
	if r == nil || r.SelectedAsset == nil {
		return nil
	}
	return []updateAssetInfo{*r.SelectedAsset}
}

func fetchGitHubLatestRelease(ctx context.Context, repo string) (*githubReleaseResponse, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Archive-Center-Updater")
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github latest release returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var release githubReleaseResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release); err != nil {
		return nil, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return nil, fmt.Errorf("github latest release response had no tag_name")
	}
	return &release, nil
}

func fetchReleaseSHA256Map(ctx context.Context, assets []githubAssetRecord) (map[string]string, string) {
	var sumsAsset *githubAssetRecord
	for i := range assets {
		name := strings.ToLower(strings.TrimSpace(assets[i].Name))
		if strings.HasPrefix(name, "sha256sums") && strings.HasSuffix(name, ".txt") {
			sumsAsset = &assets[i]
			break
		}
	}
	if sumsAsset == nil || strings.TrimSpace(sumsAsset.BrowserDownloadURL) == "" {
		return map[string]string{}, ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sumsAsset.BrowserDownloadURL, nil)
	if err != nil {
		return map[string]string{}, ""
	}
	req.Header.Set("User-Agent", "Archive-Center-Updater")
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return map[string]string{}, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return map[string]string{}, ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return map[string]string{}, ""
	}
	return parseSHA256SUMS(string(body)), sumsAsset.Name
}

func parseSHA256SUMS(text string) map[string]string {
	out := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := normalizeSHA256(fields[0])
		if sum == "" {
			continue
		}
		name := strings.TrimPrefix(strings.Join(fields[1:], " "), "*")
		name = strings.TrimSpace(name)
		if name != "" {
			out[name] = sum
		}
	}
	return out
}

func selectUpdateAsset(platform string, assets []githubAssetRecord, shaMap map[string]string) *updateAssetInfo {
	platform = normalizeUpdatePlatform(platform)
	for _, asset := range assets {
		if !strings.HasSuffix(strings.ToLower(asset.Name), ".zip") {
			continue
		}
		if !assetMatchesPlatform(asset.Name, platform) {
			continue
		}
		return &updateAssetInfo{
			Name:        asset.Name,
			Size:        asset.Size,
			SHA256:      lookupSHA256ForAsset(shaMap, asset.Name),
			DownloadURL: asset.BrowserDownloadURL,
		}
	}
	return nil
}

func assetMatchesPlatform(name, platform string) bool {
	comparable := comparableAssetName(name)
	switch normalizeUpdatePlatform(platform) {
	case "windows-x64":
		return strings.Contains(comparable, "windows") && strings.Contains(comparable, "package zip")
	case "linux-x64":
		return strings.Contains(comparable, "linux x64")
	case "linux-arm64":
		return strings.Contains(comparable, "linux arm64")
	case "macos-intel":
		return strings.Contains(comparable, "macos intel")
	case "macos-apple-silicon":
		return strings.Contains(comparable, "macos apple silicon")
	case "termux-arm64":
		return strings.Contains(comparable, "termux arm64")
	default:
		return false
	}
}

func lookupSHA256ForAsset(shaMap map[string]string, assetName string) string {
	if sha := shaMap[assetName]; sha != "" {
		return sha
	}
	want := comparableAssetName(assetName)
	for name, sha := range shaMap {
		if comparableAssetName(name) == want {
			return sha
		}
	}
	return ""
}

func comparableAssetName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	normalized := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(lower, " ")
	return strings.Join(strings.Fields(normalized), " ")
}

func (s *Server) downloadAndStageUpdateAsset(ctx context.Context, latestVersion string, asset updateAssetInfo, expectedSHA256 string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.DownloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Archive-Center-Updater")
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("asset download returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	maxBytes := s.Cfg.UpdateMaxDownloadBytes
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024 * 1024
	}
	root, err := filepath.Abs(s.Cfg.UpdateStagingDir)
	if err != nil {
		return nil, err
	}
	versionDir := sanitizePathSegment(latestVersion)
	if versionDir == "" {
		versionDir = "latest"
	}
	targetDir := filepath.Join(root, versionDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, err
	}
	fileName := sanitizeAssetFileName(asset.Name)
	if fileName == "" {
		return nil, fmt.Errorf("invalid update asset filename")
	}
	target := filepath.Join(targetDir, fileName)
	if !pathInside(target, root) {
		return nil, fmt.Errorf("refusing to stage update outside staging directory")
	}
	tmp := target + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	n, copyErr := io.Copy(out, io.TeeReader(io.LimitReader(resp.Body, maxBytes+1), h))
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return nil, closeErr
	}
	if n > maxBytes {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("download exceeded configured limit")
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expectedSHA256) {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("sha256 mismatch for %s", asset.Name)
	}
	_ = os.Remove(target)
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	return map[string]any{
		"status":            "ok",
		"policy_version":    "update-download.v1",
		"latest_version":    latestVersion,
		"asset_name":        asset.Name,
		"bytes":             n,
		"sha256":            actual,
		"staged_path":       target,
		"apply_supported":   false,
		"next_step":         "apply_available",
		"staging_directory": targetDir,
	}, nil
}

func updateApplySupported(platform string) bool {
	switch normalizeUpdatePlatform(platform) {
	case "windows-x64", "linux-x64", "linux-arm64", "macos-intel", "macos-apple-silicon", "termux-arm64":
		return true
	default:
		return false
	}
}

func (s *Server) applyStagedUpdateAsset(releaseTag, latestVersion string, staged map[string]any, restartService bool) (map[string]any, error) {
	stagedPath, _ := staged["staged_path"].(string)
	if strings.TrimSpace(stagedPath) == "" {
		return nil, fmt.Errorf("staged update path is missing")
	}
	stagedAbs, err := filepath.Abs(stagedPath)
	if err != nil {
		return nil, err
	}
	installRoot, err := inferUpdateInstallRoot(stagedAbs)
	if err != nil {
		return nil, err
	}
	releasesDir := filepath.Join(installRoot, "releases")
	versionDir := sanitizePathSegment(strings.TrimSpace(releaseTag))
	if versionDir == "" {
		versionDir = "v" + sanitizePathSegment(latestVersion)
	}
	if versionDir == "v" {
		versionDir = "latest"
	}
	targetDir := filepath.Join(releasesDir, versionDir)
	if !pathInside(targetDir, releasesDir) {
		return nil, fmt.Errorf("refusing to apply update outside releases directory")
	}
	tmpDir := targetDir + ".tmp-apply"
	if err := os.MkdirAll(releasesDir, 0o755); err != nil {
		return nil, err
	}
	if err := safeRemoveAll(tmpDir, releasesDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}
	if err := unzipUpdateArchive(stagedAbs, tmpDir); err != nil {
		_ = safeRemoveAll(tmpDir, releasesDir)
		return nil, err
	}
	tmpPackageRoot, err := findUpdatePackageRoot(tmpDir)
	if err != nil {
		_ = safeRemoveAll(tmpDir, releasesDir)
		return nil, err
	}
	if _, err := os.Stat(tmpPackageRoot); err != nil {
		_ = safeRemoveAll(tmpDir, releasesDir)
		return nil, err
	}
	if err := safeRemoveAll(targetDir, releasesDir); err != nil {
		_ = safeRemoveAll(tmpDir, releasesDir)
		return nil, err
	}
	if err := os.Rename(tmpDir, targetDir); err != nil {
		_ = safeRemoveAll(tmpDir, releasesDir)
		return nil, err
	}
	packageRoot, err := findUpdatePackageRoot(targetDir)
	if err != nil {
		return nil, err
	}
	dataDir := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_DATA_DIR"))
	if dataDir == "" {
		dataDir = filepath.Join(installRoot, "data")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	_ = os.Remove(filepath.Join(dataDir, "mysql.sock"))
	_ = os.Remove(filepath.Join(dataDir, "mariadb.pid"))
	currentPath, pointerMode, err := writeUpdateCurrentPointer(installRoot, packageRoot, releaseTag, latestVersion)
	if err != nil {
		return nil, err
	}
	restartScheduled := false
	restartMode := "manual_restart_required"
	nextStep := "manual_restart_required"
	if restartService && updateAutoRestartSupported() {
		restartScheduled = true
		restartMode = "self_exit_for_service_restart"
		nextStep = "restart_scheduled"
		go func() {
			time.Sleep(900 * time.Millisecond)
			updateRestartProcess()
		}()
	} else if restartService {
		restartMode = "manual_restart_required_not_service_managed"
	}
	return map[string]any{
		"apply_status":         "applied",
		"apply_supported":      true,
		"next_step":            nextStep,
		"install_root":         installRoot,
		"target_dir":           targetDir,
		"package_root":         packageRoot,
		"current_path":         currentPath,
		"current_pointer_mode": pointerMode,
		"data_dir":             dataDir,
		"restart_required":     true,
		"restart_scheduled":    restartScheduled,
		"restart_mode":         restartMode,
	}, nil
}

func updateAutoRestartSupported() bool {
	if strings.EqualFold(os.Getenv("AC_UPDATE_ALLOW_SELF_RESTART"), "true") {
		return true
	}
	if runtime.GOOS == "linux" && strings.TrimSpace(os.Getenv("INVOCATION_ID")) != "" {
		return true
	}
	return false
}

func inferUpdateInstallRoot(stagedPath string) (string, error) {
	for _, key := range []string{"AC_UPDATE_INSTALL_DIR", "ARCHIVE_CENTER_INSTALL_DIR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			abs, err := filepath.Abs(value)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
	}
	candidates := []string{stagedPath}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	for _, candidate := range candidates {
		if root := inferInstallRootFromReleasesPath(candidate); root != "" {
			return root, nil
		}
	}
	return "", fmt.Errorf("could not infer install root from staged update path")
}

func inferInstallRootFromReleasesPath(path string) string {
	dir := filepath.Clean(path)
	if filepath.Ext(dir) != "" {
		dir = filepath.Dir(dir)
	}
	for {
		if filepath.Base(dir) == "releases" {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func unzipUpdateArchive(src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		name = strings.TrimPrefix(filepath.Clean(name), string(filepath.Separator))
		if name == "." || name == "" {
			continue
		}
		target := filepath.Join(destAbs, name)
		if !pathInside(target, destAbs) {
			return fmt.Errorf("unsafe path in update archive: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeInErr := in.Close()
		closeOutErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInErr != nil {
			return closeInErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
	}
	return nil
}

func findUpdatePackageRoot(root string) (string, error) {
	needles := map[string]bool{
		"start-archive-center-linux.sh":       true,
		"install-and-start-termux.sh":         true,
		"01_start_archive_center_windows.bat": true,
		"Start Archive Center macOS.command":  true,
	}
	var found string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || found != "" {
			return err
		}
		if entry.IsDir() {
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && rel != "." && strings.Count(rel, string(filepath.Separator)) > 4 {
				return filepath.SkipDir
			}
			return nil
		}
		if needles[entry.Name()] {
			found = filepath.Dir(path)
			return nil
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("update package launcher was not found")
	}
	return found, nil
}

func writeUpdateCurrentPointer(installRoot, packageRoot, releaseTag, latestVersion string) (string, string, error) {
	versionLabel := strings.TrimSpace(releaseTag)
	if versionLabel == "" {
		versionLabel = "v" + strings.TrimSpace(latestVersion)
	}
	if versionLabel == "v" {
		versionLabel = strings.TrimSpace(latestVersion)
	}
	if err := os.WriteFile(filepath.Join(installRoot, "current-version.txt"), []byte(versionLabel+"\n"), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(installRoot, "current-package.txt"), []byte(packageRoot+"\n"), 0o644); err != nil {
		return "", "", err
	}
	currentPath := filepath.Join(installRoot, "current")
	if runtime.GOOS == "windows" {
		return filepath.Join(installRoot, "current-package.txt"), "file", nil
	}
	if info, err := os.Lstat(currentPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 && info.IsDir() {
			return filepath.Join(installRoot, "current-package.txt"), "file", nil
		}
		if err := os.Remove(currentPath); err != nil {
			return "", "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}
	if err := os.Symlink(packageRoot, currentPath); err != nil {
		return "", "", err
	}
	return currentPath, "symlink", nil
}

func safeRemoveAll(path, allowedRoot string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if !pathInside(path, allowedRoot) {
		return fmt.Errorf("refusing to remove path outside update root")
	}
	return os.RemoveAll(path)
}

func isSafeGitHubRepo(repo string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(strings.TrimSpace(repo))
}

func detectUpdatePlatform(goos, goarch string) string {
	switch goos + "/" + goarch {
	case "windows/amd64":
		return "windows-x64"
	case "windows/arm64":
		return "windows-x64"
	case "linux/amd64":
		return "linux-x64"
	case "linux/arm64":
		return "linux-arm64"
	case "darwin/amd64":
		return "macos-intel"
	case "darwin/arm64":
		return "macos-apple-silicon"
	case "android/arm64":
		return "termux-arm64"
	default:
		return goos + "-" + goarch
	}
}

func normalizeUpdatePlatform(platform string) string {
	p := strings.ToLower(strings.TrimSpace(platform))
	p = strings.ReplaceAll(p, "_", "-")
	switch p {
	case "", "windows", "win", "win32", "win64", "windows-amd64", "windows-x86-64":
		return "windows-x64"
	case "linux", "linux-amd64", "linux-x86-64":
		return "linux-x64"
	case "linux-aarch64":
		return "linux-arm64"
	case "darwin-amd64", "macos-amd64", "macos-x64":
		return "macos-intel"
	case "darwin-arm64", "macos-arm64", "macos-silicon":
		return "macos-apple-silicon"
	case "android-arm64", "termux", "termux-aarch64":
		return "termux-arm64"
	default:
		return p
	}
}

func versionFromTag(tag string) string {
	v := strings.TrimSpace(tag)
	v = strings.TrimPrefix(v, "refs/tags/")
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	return v
}

func compareVersions(a, b string) int {
	ap := parseComparableVersion(a)
	bp := parseComparableVersion(b)
	for i := 0; i < 3; i++ {
		if ap.numbers[i] > bp.numbers[i] {
			return 1
		}
		if ap.numbers[i] < bp.numbers[i] {
			return -1
		}
	}
	if ap.prerelease == "" && bp.prerelease != "" {
		return 1
	}
	if ap.prerelease != "" && bp.prerelease == "" {
		return -1
	}
	if ap.prereleaseRank > bp.prereleaseRank {
		return 1
	}
	if ap.prereleaseRank < bp.prereleaseRank {
		return -1
	}
	if ap.prereleaseNumber > bp.prereleaseNumber {
		return 1
	}
	if ap.prereleaseNumber < bp.prereleaseNumber {
		return -1
	}
	if ap.prerelease > bp.prerelease {
		return 1
	}
	if ap.prerelease < bp.prerelease {
		return -1
	}
	return 0
}

type comparableVersion struct {
	numbers          [3]int
	prerelease       string
	prereleaseRank   int
	prereleaseNumber int
}

func parseComparableVersion(version string) comparableVersion {
	clean := versionFromTag(version)
	if idx := strings.Index(clean, "+"); idx >= 0 {
		clean = clean[:idx]
	}
	base := clean
	pre := ""
	if idx := strings.Index(clean, "-"); idx >= 0 {
		base = clean[:idx]
		pre = strings.ToLower(strings.TrimSpace(clean[idx+1:]))
	}
	out := comparableVersion{numbers: parseVersionParts(base), prerelease: pre}
	if pre == "" {
		return out
	}
	label := regexp.MustCompile(`[^a-z]+`).ReplaceAllString(pre, "")
	numText := regexp.MustCompile(`[^0-9]+`).ReplaceAllString(pre, "")
	if numText != "" {
		out.prereleaseNumber, _ = strconv.Atoi(numText)
	}
	switch label {
	case "alpha", "a":
		out.prereleaseRank = 1
	case "beta", "b":
		out.prereleaseRank = 2
	case "rc":
		out.prereleaseRank = 3
	default:
		out.prereleaseRank = 0
	}
	return out
}

func parseVersionParts(version string) [3]int {
	clean := versionFromTag(version)
	if idx := strings.IndexAny(clean, "-+ "); idx >= 0 {
		clean = clean[:idx]
	}
	parts := strings.Split(clean, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(regexp.MustCompile(`[^0-9]`).ReplaceAllString(parts[i], ""))
		out[i] = n
	}
	return out
}

func normalizeSHA256(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	if regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(s) {
		return s
	}
	return ""
}

func sanitizeAssetFileName(name string) string {
	base := filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	base = strings.ReplaceAll(base, "\x00", "")
	return base
}

func sanitizePathSegment(value string) string {
	s := regexp.MustCompile(`[^A-Za-z0-9_.-]+`).ReplaceAllString(strings.TrimSpace(value), "_")
	return strings.Trim(s, "._-")
}

func pathInside(child, parent string) bool {
	childFull, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	parentFull, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(parentFull, childFull)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}
