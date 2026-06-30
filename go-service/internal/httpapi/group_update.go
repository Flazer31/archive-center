package httpapi

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var updateHTTPClient = http.DefaultClient

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
		ApplySupported:     false,
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
			SHA256:      shaMap[asset.Name],
			DownloadURL: asset.BrowserDownloadURL,
		}
	}
	return nil
}

func assetMatchesPlatform(name, platform string) bool {
	n := strings.ToLower(name)
	switch normalizeUpdatePlatform(platform) {
	case "windows-x64":
		return strings.Contains(n, "windows") && strings.Contains(n, "package.zip")
	case "linux-x64":
		return strings.Contains(n, "linux x64")
	case "linux-arm64":
		return strings.Contains(n, "linux arm64")
	case "macos-intel":
		return strings.Contains(n, "macos intel")
	case "macos-apple-silicon":
		return strings.Contains(n, "macos apple silicon")
	case "termux-arm64":
		return strings.Contains(n, "termux arm64")
	default:
		return false
	}
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
		"next_step":         "manual_apply_or_future_helper",
		"staging_directory": targetDir,
	}, nil
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
	ap := parseVersionParts(a)
	bp := parseVersionParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
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
