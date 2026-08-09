package httpapi

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/packageupdate"
)

var updateHTTPClient = http.DefaultClient

var (
	errUpdatePlatformUnsupported = errors.New("update platform is unsupported")
	errUpdatePlatformMismatch    = errors.New("requested update platform does not match the running backend")
)

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
	RuntimeOS           string           `json:"runtime_os"`
	RuntimeArch         string           `json:"runtime_arch"`
	Distribution        string           `json:"distribution,omitempty"`
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
	CompatibilityStatus string           `json:"compatibility_status"`
	CompatibilityReason string           `json:"compatibility_reason,omitempty"`
}

type updateDownloadRequest struct {
	CurrentVersion string `json:"current_version"`
	Platform       string `json:"platform"`
	AssetName      string `json:"asset_name"`
	ExpectedSHA256 string `json:"expected_sha256"`
}

type updateApplyRequest struct {
	CurrentVersion string `json:"current_version"`
	Platform       string `json:"platform"`
}

type pendingPackageUpdate struct {
	ContractVersion string   `json:"contract_version"`
	CurrentVersion  string   `json:"current_version"`
	TargetVersion   string   `json:"target_version"`
	AssetPath       string   `json:"asset_path"`
	SHA256          string   `json:"sha256"`
	RequiredFiles   []string `json:"required_files,omitempty"`
	PreparedAt      string   `json:"prepared_at"`
}

func (s *Server) registerUpdateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /update/check", s.handleUpdateCheck)
	mux.HandleFunc("POST /update/download", s.handleUpdateDownload)
	mux.HandleFunc("POST /update/apply", s.handleUpdateApply)
	mux.HandleFunc("GET /update/status", s.handleUpdateStatus)
}

func (s *Server) handleUpdateStatus(w http.ResponseWriter, _ *http.Request) {
	root, err := s.updateStagingRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update_staging_invalid", err.Error())
		return
	}
	out := map[string]any{
		"status":          "idle",
		"policy_version":  "update-status.v1",
		"apply_supported": s.updateApplySupported(),
	}
	pendingExists := false
	stateStatus := ""
	var pendingValue map[string]any
	var stateValue map[string]any
	for _, item := range []struct {
		name string
		path string
	}{
		{name: "pending", path: filepath.Join(root, "pending-update.json")},
		{name: "state", path: filepath.Join(root, "update-state.json")},
	} {
		data, readErr := os.ReadFile(item.path)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			writeError(w, http.StatusInternalServerError, "update_status_read_failed", readErr.Error())
			return
		}
		var value map[string]any
		if json.Unmarshal(data, &value) == nil {
			out[item.name] = value
			if item.name == "pending" {
				pendingExists = true
				pendingValue = value
			}
			if item.name == "state" {
				stateValue = value
				if status, ok := value["status"].(string); ok {
					stateStatus = strings.TrimSpace(status)
				}
			}
		}
	}
	var versionSource map[string]any
	if stateStatus == "applying" || stateStatus == "applied_pending_health" {
		out["status"] = stateStatus
		versionSource = stateValue
	} else if pendingExists {
		out["status"] = "pending_next_start"
		versionSource = pendingValue
	} else if stateStatus != "" {
		out["status"] = stateStatus
		versionSource = stateValue
	}
	for _, key := range []string{"current_version", "target_version"} {
		if value, ok := versionSource[key].(string); ok && strings.TrimSpace(value) != "" {
			out[key] = strings.TrimSpace(value)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.UpdateEnabled {
		writeError(w, http.StatusServiceUnavailable, "update_disabled", "update checks are disabled")
		return
	}
	current := strings.TrimSpace(s.Cfg.BuildVersion)
	if supplied := strings.TrimSpace(r.URL.Query().Get("current_version")); supplied != "" && !strings.EqualFold(supplied, current) {
		writeBadRequest(w, "current_version must match the running backend version")
		return
	}
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	result, err := s.resolveLatestUpdate(r.Context(), current, platform)
	if err != nil {
		if writeUpdatePlatformError(w, err) {
			return
		}
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
	current := strings.TrimSpace(s.Cfg.BuildVersion)
	if supplied := strings.TrimSpace(req.CurrentVersion); supplied != "" && !strings.EqualFold(supplied, current) {
		writeBadRequest(w, "current_version must match the running backend version")
		return
	}
	result, err := s.resolveLatestUpdate(r.Context(), current, strings.TrimSpace(req.Platform))
	if err != nil {
		if writeUpdatePlatformError(w, err) {
			return
		}
		writeError(w, http.StatusBadGateway, "update_check_failed", err.Error())
		return
	}
	if !result.UpdateAvailable {
		writeUpdateUnavailable(w, result)
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
	expected := normalizeSHA256(asset.SHA256)
	if expected == "" {
		writeError(w, http.StatusBadGateway, "update_sha256_missing", "selected update asset has no SHA256 entry")
		return
	}
	if supplied := strings.TrimSpace(req.ExpectedSHA256); supplied != "" {
		clientExpected := normalizeSHA256(supplied)
		if clientExpected == "" || !strings.EqualFold(clientExpected, expected) {
			writeBadRequest(w, "expected_sha256 must match the selected release SHA256SUMS entry")
			return
		}
	}
	staged, err := s.downloadAndStageUpdateAsset(r.Context(), current, result.LatestVersion, *asset, expected)
	if err != nil {
		writeError(w, http.StatusBadGateway, "update_download_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, staged)
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.UpdateEnabled {
		writeError(w, http.StatusServiceUnavailable, "update_disabled", "updates are disabled")
		return
	}
	requestShutdown := s.RequestShutdown
	if requestShutdown == nil {
		writeError(w, http.StatusServiceUnavailable, "update_apply_unmanaged", "the backend was not started by an update-aware managed launcher")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "update_ack_flush_unsupported", "the HTTP transport cannot flush the update acknowledgement")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req updateApplyRequest
	if err := decoder.Decode(&req); err != nil && err != io.EOF {
		writeBadRequest(w, "invalid update apply request: "+err.Error())
		return
	}
	current := strings.TrimSpace(s.Cfg.BuildVersion)
	if supplied := strings.TrimSpace(req.CurrentVersion); supplied != "" && !strings.EqualFold(supplied, current) {
		writeBadRequest(w, "current_version must match the running backend version")
		return
	}
	root, err := s.updateStagingRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update_staging_invalid", err.Error())
		return
	}
	if err := validateUpdateApplyHelper(root); err != nil {
		writeError(w, http.StatusServiceUnavailable, "update_apply_helper_invalid", err.Error())
		return
	}
	result, err := s.resolveLatestUpdate(r.Context(), current, strings.TrimSpace(req.Platform))
	if err != nil {
		if writeUpdatePlatformError(w, err) {
			return
		}
		writeError(w, http.StatusBadGateway, "update_check_failed", err.Error())
		return
	}
	if !result.UpdateAvailable {
		writeUpdateUnavailable(w, result)
		return
	}
	asset := result.SelectedAsset
	if asset == nil || strings.TrimSpace(asset.DownloadURL) == "" {
		writeError(w, http.StatusNotFound, "update_asset_not_found", "no compatible update asset was found")
		return
	}
	expected := normalizeSHA256(asset.SHA256)
	if expected == "" {
		writeError(w, http.StatusBadGateway, "update_sha256_missing", "selected update asset has no SHA256 entry")
		return
	}
	staged, err := s.downloadAndStageUpdateAsset(r.Context(), current, result.LatestVersion, *asset, expected)
	if err != nil {
		writeError(w, http.StatusBadGateway, "update_download_failed", err.Error())
		return
	}
	staged["status"] = "accepted"
	staged["policy_version"] = "update-apply.v1"
	staged["runtime_os"] = result.RuntimeOS
	staged["runtime_arch"] = result.RuntimeArch
	if result.Distribution != "" {
		staged["distribution"] = result.Distribution
	}
	staged["platform"] = result.Platform
	staged["apply_timing"] = "after_backend_exit"
	staged["shutdown_requested"] = true
	staged["exit_code"] = UpdateApplyExitCode
	staged["next_step"] = "managed_launcher_apply_pending_update"
	writeJSON(w, http.StatusOK, staged)
	flusher.Flush()
	requestShutdown(UpdateApplyExitCode)
}

func (s *Server) resolveLatestUpdate(ctx context.Context, currentVersion, platform string) (*updateCheckResult, error) {
	repo := strings.TrimSpace(s.Cfg.UpdateGitHubRepo)
	if !isSafeGitHubRepo(repo) {
		return nil, fmt.Errorf("invalid update repository %q", repo)
	}
	var err error
	platform, err = authoritativeUpdatePlatform(platform, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	release, err := fetchGitHubLatestRelease(ctx, repo)
	if err != nil {
		return nil, err
	}
	latest := versionFromTag(release.TagName)
	shaMap, shaSource := fetchReleaseSHA256Map(ctx, release.Assets)
	selected := selectUpdateAsset(platform, release.Assets, shaMap)
	result := &updateCheckResult{
		Status:              "ok",
		PolicyVersion:       "update-check.v2",
		Repository:          repo,
		Channel:             strings.TrimSpace(s.Cfg.UpdateChannel),
		CurrentVersion:      strings.TrimSpace(currentVersion),
		LatestVersion:       latest,
		UpdateAvailable:     false,
		Platform:            platform,
		RuntimeOS:           runtime.GOOS,
		RuntimeArch:         runtime.GOARCH,
		Distribution:        detectRuntimeDistribution(runtime.GOOS),
		SelectedAsset:       selected,
		SHA256Source:        shaSource,
		ApplySupported:      s.updateApplySupported(),
		DownloadSupported:   selected != nil && selected.SHA256 != "",
		ReleaseTag:          release.TagName,
		ReleaseName:         release.Name,
		ReleaseURL:          release.HTMLURL,
		ReleasePrerelease:   release.Prerelease,
		ReleasePublishedAt:  release.PublishedAt,
		CompatibilityStatus: "not_newer",
	}
	if compareVersions(latest, currentVersion) <= 0 {
		result.CompatibilityReason = "the latest release is not newer than the running backend"
		return result, nil
	}
	if s.RequestShutdown == nil {
		result.CompatibilityStatus = "managed_launcher_unavailable"
		result.CompatibilityReason = "the backend was not started by an update-aware managed launcher"
		return result, nil
	}
	if selected == nil {
		result.CompatibleAssetNote = "no asset name matched the requested platform"
		result.CompatibilityStatus = "platform_asset_missing"
		result.CompatibilityReason = "the release does not contain a package for the running OS and CPU"
		return result, nil
	}
	if normalizeSHA256(selected.SHA256) == "" {
		result.CompatibilityStatus = "sha256_missing"
		result.CompatibilityReason = "the selected platform package is not covered by the release SHA256SUMS"
		return result, nil
	}
	root, err := s.updateStagingRoot()
	if err != nil {
		result.CompatibilityStatus = "installed_package_invalid"
		result.CompatibilityReason = err.Error()
		return result, nil
	}
	if err := validateUpdateApplyHelper(root); err != nil {
		result.CompatibilityStatus = "apply_helper_unavailable"
		result.CompatibilityReason = err.Error()
		return result, nil
	}
	if err := s.preflightUpdateAsset(ctx, filepath.Dir(root), strings.TrimSpace(currentVersion), latest, *selected); err != nil {
		result.CompatibilityStatus = updateCompatibilityStatus(err)
		result.CompatibilityReason = err.Error()
		return result, nil
	}
	result.UpdateAvailable = true
	result.CompatibilityStatus = "compatible"
	result.CompatibilityReason = "candidate package passed direct-update preflight"
	return result, nil
}

func writeUpdateUnavailable(w http.ResponseWriter, result *updateCheckResult) {
	if result != nil && result.CompatibilityStatus == "not_newer" {
		writeError(w, http.StatusConflict, "update_not_newer", result.CompatibilityReason)
		return
	}
	status := "update_incompatible"
	message := "the latest release cannot be applied directly to this installation"
	if result != nil {
		if strings.TrimSpace(result.CompatibilityStatus) != "" {
			status = result.CompatibilityStatus
		}
		if strings.TrimSpace(result.CompatibilityReason) != "" {
			message = result.CompatibilityReason
		}
	}
	writeError(w, http.StatusConflict, status, message)
}

func updateCompatibilityStatus(err error) string {
	var updateErr *packageupdate.UpdateError
	if errors.As(err, &updateErr) && strings.TrimSpace(updateErr.Code) != "" {
		return "preflight_" + strings.TrimSpace(updateErr.Code)
	}
	return "preflight_failed"
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
	if validateUpdateDownloadURL(sumsAsset.BrowserDownloadURL) != nil {
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
	for _, preferUpdatePayload := range []bool{true, false} {
		for _, asset := range assets {
			if !strings.HasSuffix(strings.ToLower(asset.Name), ".zip") {
				continue
			}
			if !assetMatchesPlatform(asset.Name, platform) {
				continue
			}
			isUpdatePayload := strings.Contains(comparableAssetName(asset.Name), "update package")
			if isUpdatePayload != preferUpdatePayload {
				continue
			}
			return &updateAssetInfo{
				Name:        asset.Name,
				Size:        asset.Size,
				SHA256:      lookupSHA256ForAsset(shaMap, asset.Name),
				DownloadURL: asset.BrowserDownloadURL,
			}
		}
	}
	return nil
}

func assetMatchesPlatform(name, platform string) bool {
	comparable := comparableAssetName(name)
	switch normalizeUpdatePlatform(platform) {
	case "windows-x64":
		return strings.Contains(comparable, "windows") &&
			!strings.Contains(comparable, "windows arm64") &&
			!strings.Contains(comparable, "windows aarch64") &&
			strings.Contains(comparable, "package zip")
	case "windows-arm64":
		return strings.Contains(comparable, "windows arm64") && strings.Contains(comparable, "package zip")
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

func (s *Server) downloadAndStageUpdateAsset(ctx context.Context, currentVersion, latestVersion string, asset updateAssetInfo, expectedSHA256 string) (map[string]any, error) {
	root, err := s.updateStagingRoot()
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
	n, actual, err := s.downloadVerifiedUpdateAsset(ctx, asset, expectedSHA256, target)
	if err != nil {
		return nil, err
	}
	applySupported := updateApplyHelperAvailable(root)
	pendingPath := ""
	if applySupported {
		pending := pendingPackageUpdate{
			ContractVersion: "archive-center.pending-update.v1",
			CurrentVersion:  strings.TrimSpace(currentVersion),
			TargetVersion:   strings.TrimSpace(latestVersion),
			AssetPath:       target,
			SHA256:          actual,
			RequiredFiles:   requiredUpdatePackageFiles(runtime.GOOS),
			PreparedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		}
		pendingPath = filepath.Join(root, "pending-update.json")
		if err := writePendingPackageUpdate(pendingPath, pending); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"status":            "ok",
		"policy_version":    "update-download.v1",
		"latest_version":    latestVersion,
		"asset_name":        asset.Name,
		"bytes":             n,
		"sha256":            actual,
		"staged_path":       target,
		"apply_supported":   applySupported,
		"apply_timing":      "next_start",
		"pending_path":      pendingPath,
		"next_step":         map[bool]string{true: "restart_archive_center_to_apply", false: "manual_apply_or_install_updater_helper"}[applySupported],
		"staging_directory": targetDir,
	}, nil
}

func (s *Server) preflightUpdateAsset(ctx context.Context, packageRoot, currentVersion, targetVersion string, asset updateAssetInfo) error {
	tempRoot, err := os.MkdirTemp("", "archive-center-update-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)
	fileName := sanitizeAssetFileName(asset.Name)
	if fileName == "" {
		return fmt.Errorf("invalid update asset filename")
	}
	target := filepath.Join(tempRoot, fileName)
	_, actual, err := s.downloadVerifiedUpdateAsset(ctx, asset, asset.SHA256, target)
	if err != nil {
		return err
	}
	return packageupdate.PreflightCandidate(packageRoot, packageupdate.Candidate{
		CurrentVersion: currentVersion,
		TargetVersion:  targetVersion,
		AssetPath:      target,
		SHA256:         actual,
		RequiredFiles:  requiredUpdatePackageFiles(runtime.GOOS),
	})
}

func (s *Server) downloadVerifiedUpdateAsset(ctx context.Context, asset updateAssetInfo, expectedSHA256, target string) (int64, string, error) {
	if err := validateUpdateDownloadURL(asset.DownloadURL); err != nil {
		return 0, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.DownloadURL, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", "Archive-Center-Updater")
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, "", fmt.Errorf("asset download returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	maxBytes := s.Cfg.UpdateMaxDownloadBytes
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024 * 1024
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return 0, "", err
	}
	tmp := target + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return 0, "", err
	}
	h := sha256.New()
	n, copyErr := io.Copy(out, io.TeeReader(io.LimitReader(resp.Body, maxBytes+1), h))
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return 0, "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return 0, "", closeErr
	}
	if n > maxBytes {
		_ = os.Remove(tmp)
		return 0, "", fmt.Errorf("download exceeded configured limit")
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, normalizeSHA256(expectedSHA256)) {
		_ = os.Remove(tmp)
		return 0, "", fmt.Errorf("sha256 mismatch for %s", asset.Name)
	}
	_ = os.Remove(target)
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return 0, "", err
	}
	return n, actual, nil
}

func requiredUpdatePackageFiles(goos string) []string {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "windows":
		return []string{
			"bin/archive-center-go.exe",
			"bin/archive-center-updater.exe",
			"bin/mariadb-schema.exe",
			packageupdate.PackageReleaseStatusName,
			"scripts/start-full-windows.ps1",
			"01_start_archive_center_windows.bat",
			"PACKAGE_MIGRATION_UPDATE.json",
			"Archive Center.js",
		}
	case "darwin":
		return []string{
			"bin/archive-center-go",
			"bin/archive-center-updater",
			"bin/mariadb-schema",
			packageupdate.PackageReleaseStatusName,
			"scripts/start-full-posix.sh",
			"scripts/start-full-macos.sh",
			"PACKAGE_MIGRATION_UPDATE.json",
			"Archive Center.js",
		}
	case "android":
		return []string{
			"bin/archive-center-go",
			"bin/archive-center-updater",
			"bin/mariadb-schema",
			packageupdate.PackageReleaseStatusName,
			"scripts/start-full-posix.sh",
			"scripts/install-and-start-termux.sh",
			"PACKAGE_MIGRATION_UPDATE.json",
			"Archive Center.js",
		}
	default:
		return []string{
			"bin/archive-center-go",
			"bin/archive-center-updater",
			"bin/mariadb-schema",
			packageupdate.PackageReleaseStatusName,
			"scripts/start-full-posix.sh",
			"scripts/start-full-linux.sh",
			"PACKAGE_MIGRATION_UPDATE.json",
			"Archive Center.js",
		}
	}
}

func (s *Server) updateStagingRoot() (string, error) {
	root := strings.TrimSpace(s.Cfg.UpdateStagingDir)
	if root == "" {
		root = ".updates"
	}
	return filepath.Abs(root)
}

func (s *Server) updateApplySupported() bool {
	if s.RequestShutdown == nil {
		return false
	}
	root, err := s.updateStagingRoot()
	return err == nil && updateApplyHelperAvailable(root)
}

func updateApplyHelperAvailable(stagingRoot string) bool {
	return validateUpdateApplyHelper(stagingRoot) == nil
}

func validateUpdateApplyHelper(stagingRoot string) error {
	cleanStagingRoot := filepath.Clean(stagingRoot)
	root := filepath.Dir(cleanStagingRoot)
	if !strings.EqualFold(filepath.Base(cleanStagingRoot), ".updates") {
		return fmt.Errorf("update staging directory must be the package .updates directory")
	}
	if err := packageupdate.ValidateInstalledApplyHelper(root); err != nil {
		return fmt.Errorf("installed update helper validation failed: %w", err)
	}
	return nil
}

func writePendingPackageUpdate(path string, pending pendingPackageUpdate) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	previous := path + ".previous"
	_ = os.Remove(previous)
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, previous); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Rename(previous, path)
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(previous)
	return nil
}

func isSafeGitHubRepo(repo string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(strings.TrimSpace(repo))
}

func detectUpdatePlatform(goos, goarch string) string {
	switch goos + "/" + goarch {
	case "windows/amd64":
		return "windows-x64"
	case "windows/arm64":
		return "windows-arm64"
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

func authoritativeUpdatePlatform(supplied, goos, goarch string) (string, error) {
	detected := detectUpdatePlatform(strings.ToLower(strings.TrimSpace(goos)), strings.ToLower(strings.TrimSpace(goarch)))
	if !isSupportedUpdatePlatform(detected) {
		return "", fmt.Errorf("%w: runtime %s/%s", errUpdatePlatformUnsupported, goos, goarch)
	}
	if strings.TrimSpace(supplied) == "" {
		return detected, nil
	}
	requested := normalizeUpdatePlatform(supplied)
	if !isSupportedUpdatePlatform(requested) {
		return "", fmt.Errorf("%w: %s", errUpdatePlatformUnsupported, supplied)
	}
	if requested != detected {
		return "", fmt.Errorf("%w: requested %s, runtime %s", errUpdatePlatformMismatch, requested, detected)
	}
	return detected, nil
}

func isSupportedUpdatePlatform(platform string) bool {
	switch normalizeUpdatePlatform(platform) {
	case "windows-x64", "windows-arm64", "linux-x64", "linux-arm64", "macos-intel", "macos-apple-silicon", "termux-arm64":
		return true
	default:
		return false
	}
}

func writeUpdatePlatformError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, errUpdatePlatformMismatch):
		writeError(w, http.StatusBadRequest, "update_platform_mismatch", err.Error())
		return true
	case errors.Is(err, errUpdatePlatformUnsupported):
		writeError(w, http.StatusBadRequest, "update_platform_unsupported", err.Error())
		return true
	default:
		return false
	}
}

func detectRuntimeDistribution(goos string) string {
	if !strings.EqualFold(strings.TrimSpace(goos), "linux") {
		return ""
	}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	return parseOSReleaseDistribution(string(data))
}

func parseOSReleaseDistribution(data string) string {
	for _, line := range strings.Split(data, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || key != "ID" {
			continue
		}
		value = strings.TrimSpace(value)
		if unquoted, unquoteErr := strconv.Unquote(value); unquoteErr == nil {
			value = unquoted
		}
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
}

func normalizeUpdatePlatform(platform string) string {
	p := strings.ToLower(strings.TrimSpace(platform))
	p = strings.ReplaceAll(p, "_", "-")
	switch p {
	case "", "windows", "win", "win32", "win64", "windows-amd64", "windows-x86-64":
		return "windows-x64"
	case "windows-aarch64":
		return "windows-arm64"
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
	ap, apre := parseVersion(a)
	bp, bpre := parseVersion(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	if apre == "" && bpre != "" {
		return 1
	}
	if apre != "" && bpre == "" {
		return -1
	}
	return comparePrerelease(apre, bpre)
}

func parseVersion(version string) ([3]int, string) {
	clean := versionFromTag(version)
	pre := ""
	if idx := strings.IndexAny(clean, "-+ "); idx >= 0 {
		if clean[idx] == '-' {
			pre = strings.TrimSpace(clean[idx+1:])
			if plus := strings.Index(pre, "+"); plus >= 0 {
				pre = pre[:plus]
			}
		}
		clean = clean[:idx]
	}
	parts := strings.Split(clean, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(regexp.MustCompile(`[^0-9]`).ReplaceAllString(parts[i], ""))
		out[i] = n
	}
	return out, strings.ToLower(pre)
}

func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	left, right := prereleaseParts(a), prereleaseParts(b)
	for i := 0; i < len(left) || i < len(right); i++ {
		if i >= len(left) {
			return -1
		}
		if i >= len(right) {
			return 1
		}
		ln, lnum := strconv.Atoi(left[i])
		rn, rnum := strconv.Atoi(right[i])
		switch {
		case lnum == nil && rnum == nil && ln != rn:
			if ln < rn {
				return -1
			}
			return 1
		case lnum == nil && rnum != nil:
			return -1
		case lnum != nil && rnum == nil:
			return 1
		case left[i] != right[i]:
			if left[i] < right[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func prereleaseParts(value string) []string {
	value = regexp.MustCompile(`([A-Za-z]+)([0-9]+)`).ReplaceAllString(value, `$1.$2`)
	return strings.FieldsFunc(value, func(r rune) bool { return r == '.' || r == '-' || r == '_' })
}

func validateUpdateDownloadURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || strings.TrimSpace(parsed.Hostname()) == "" {
		return fmt.Errorf("update download URL must use HTTPS")
	}
	return nil
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
