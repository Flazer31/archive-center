package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/packageupdate"
)

type updateRoundTripFunc func(*http.Request) (*http.Response, error)

func (f updateRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestUpdateCheckSelectsPlatformAssetAndSHA256(t *testing.T) {
	zipBytes := []byte("archive-center-2.3-windows-package")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	platform := detectUpdatePlatform(runtime.GOOS, runtime.GOARCH)
	assetName := updateTestAssetName("2.3", platform)
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{
				"tag_name":"v2.3.0",
				"name":"Archive Center 2.3",
				"html_url":"https://github.com/Flazer31/archive-center/releases/tag/v2.3.0",
				"assets":[
					{"name":`+strconv.Quote(assetName)+`,"browser_download_url":"https://example.test/package.zip","size":33},
					{"name":"SHA256SUMS-2.3.txt","browser_download_url":"https://example.test/sums.txt","size":90}
				]
			}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  "+assetName+"\n")
		default:
			t.Fatalf("unexpected update HTTP request: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { updateHTTPClient = restore }()

	cfg := config.Default()
	cfg.BuildVersion = "2.2.0"
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/update/check?platform="+platform, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["latest_version"] != "2.3.0" || resp["update_available"] != true {
		t.Fatalf("unexpected update response: %+v", resp)
	}
	asset, ok := resp["selected_asset"].(map[string]any)
	if !ok {
		t.Fatalf("selected_asset missing: %+v", resp)
	}
	if asset["name"] != assetName || asset["sha256"] != sha {
		t.Fatalf("selected_asset = %+v, want runtime asset with sha", asset)
	}
	if resp["runtime_os"] != runtime.GOOS || resp["runtime_arch"] != runtime.GOARCH || resp["platform"] != platform {
		t.Fatalf("runtime identity missing from check response: %+v", resp)
	}
	if resp["apply_supported"] != false || resp["download_supported"] != true {
		t.Fatalf("support flags unexpected: %+v", resp)
	}
}

func TestUpdateDownloadStagesVerifiedAsset(t *testing.T) {
	zipBytes := []byte("archive-center-2.3-windows-package")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	platform := detectUpdatePlatform(runtime.GOOS, runtime.GOARCH)
	assetName := updateTestAssetName("2.3", platform)
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{
				"tag_name":"v2.3.0",
				"name":"Archive Center 2.3",
				"assets":[
					{"name":`+strconv.Quote(assetName)+`,"browser_download_url":"https://example.test/package.zip","size":33},
					{"name":"SHA256SUMS-2.3.txt","browser_download_url":"https://example.test/sums.txt","size":90}
				]
			}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  "+assetName+"\n")
		case "https://example.test/package.zip":
			return bytesResponse(http.StatusOK, zipBytes)
		default:
			t.Fatalf("unexpected update HTTP request: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { updateHTTPClient = restore }()

	cfg := config.Default()
	cfg.BuildVersion = "2.2.0"
	packageRoot := t.TempDir()
	installVerifiedUpdateHelper(t, packageRoot)
	cfg.UpdateStagingDir = filepath.Join(packageRoot, ".updates")
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"platform":"` + platform + `","current_version":"2.2.0"}`
	req := httptest.NewRequest(http.MethodPost, "/update/download", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["sha256"] != sha || resp["apply_supported"] != true || resp["next_step"] != "restart_archive_center_to_apply" {
		t.Fatalf("unexpected download response: %+v", resp)
	}
	stagedPath, _ := resp["staged_path"].(string)
	if stagedPath == "" {
		t.Fatalf("staged_path missing: %+v", resp)
	}
	got, err := os.ReadFile(stagedPath)
	if err != nil {
		t.Fatalf("read staged file: %v", err)
	}
	if string(got) != string(zipBytes) {
		t.Fatalf("staged bytes mismatch")
	}
	pendingBytes, err := os.ReadFile(filepath.Join(packageRoot, ".updates", "pending-update.json"))
	if err != nil {
		t.Fatalf("read pending update: %v", err)
	}
	var pending pendingPackageUpdate
	if err := json.Unmarshal(pendingBytes, &pending); err != nil {
		t.Fatalf("decode pending update: %v", err)
	}
	if pending.ContractVersion != "archive-center.pending-update.v1" || pending.CurrentVersion != "2.2.0" || pending.TargetVersion != "2.3.0" || pending.AssetPath != stagedPath || pending.SHA256 != sha {
		t.Fatalf("pending update mismatch: %+v", pending)
	}
	wantRequired := requiredUpdatePackageFiles(runtime.GOOS)
	if strings.Join(pending.RequiredFiles, "\n") != strings.Join(wantRequired, "\n") {
		t.Fatalf("pending required_files = %v, want %v", pending.RequiredFiles, wantRequired)
	}
	statusReq := httptest.NewRequest(http.MethodGet, "/update/status", nil)
	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK || !strings.Contains(statusRec.Body.String(), `"status":"pending_next_start"`) {
		t.Fatalf("update status=%d body=%s", statusRec.Code, statusRec.Body.String())
	}
	if err := os.WriteFile(filepath.Join(packageRoot, ".updates", "update-state.json"), []byte(`{"status":"committed","current_version":"1.9.0","target_version":"2.0.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	statusRec = httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	var statusResp map[string]any
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode update status: %v", err)
	}
	if statusRec.Code != http.StatusOK || statusResp["status"] != "pending_next_start" || statusResp["current_version"] != "2.2.0" || statusResp["target_version"] != "2.3.0" {
		t.Fatalf("pending must override old committed state: status=%d body=%s", statusRec.Code, statusRec.Body.String())
	}
	if err := os.WriteFile(filepath.Join(packageRoot, ".updates", "update-state.json"), []byte(`{"status":"applied_pending_health","current_version":"2.3.0","target_version":"2.4.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	statusRec = httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	statusResp = nil
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode active update status: %v", err)
	}
	if statusRec.Code != http.StatusOK || statusResp["status"] != "applied_pending_health" || statusResp["current_version"] != "2.3.0" || statusResp["target_version"] != "2.4.0" {
		t.Fatalf("active state must provide versions: status=%d body=%s", statusRec.Code, statusRec.Body.String())
	}
}

func TestRequiredUpdatePackageFilesArePlatformSpecific(t *testing.T) {
	tests := []struct {
		goos string
		want []string
	}{
		{goos: "windows", want: []string{"bin/archive-center-go.exe", "bin/archive-center-updater.exe", "bin/mariadb-schema.exe", "scripts/start-full-windows.ps1", "01_start_archive_center_windows.bat", "PACKAGE_MIGRATION_UPDATE.json", "Archive Center.js"}},
		{goos: "linux", want: []string{"bin/archive-center-go", "bin/archive-center-updater", "bin/mariadb-schema", "scripts/start-full-posix.sh", "scripts/start-full-linux.sh", "PACKAGE_MIGRATION_UPDATE.json", "Archive Center.js"}},
		{goos: "darwin", want: []string{"bin/archive-center-go", "bin/archive-center-updater", "bin/mariadb-schema", "scripts/start-full-posix.sh", "scripts/start-full-macos.sh", "PACKAGE_MIGRATION_UPDATE.json", "Archive Center.js"}},
		{goos: "android", want: []string{"bin/archive-center-go", "bin/archive-center-updater", "bin/mariadb-schema", "scripts/start-full-posix.sh", "scripts/install-and-start-termux.sh", "PACKAGE_MIGRATION_UPDATE.json", "Archive Center.js"}},
	}
	for _, tc := range tests {
		t.Run(tc.goos, func(t *testing.T) {
			got := requiredUpdatePackageFiles(tc.goos)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("required files = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUpdateDownloadRejectsClientSHAOverride(t *testing.T) {
	zipBytes := []byte("verified release asset")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	assetName := updateTestAssetName("3.1", detectUpdatePlatform(runtime.GOOS, runtime.GOARCH))
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{"tag_name":"v3.1.0","assets":[{"name":`+strconv.Quote(assetName)+`,"browser_download_url":"https://example.test/package.zip"},{"name":"SHA256SUMS-3.1.txt","browser_download_url":"https://example.test/sums.txt"}]}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  "+assetName+"\n")
		default:
			t.Fatalf("unexpected update HTTP request: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { updateHTTPClient = restore }()

	cfg := config.Default()
	cfg.BuildVersion = "3.0.0"
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/update/download", strings.NewReader(`{"current_version":"3.0.0","expected_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "must match") {
		t.Fatalf("override status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateDownloadRejectsReleaseThatIsNotNewer(t *testing.T) {
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{"tag_name":"v3.0.0","assets":[]}`)
		default:
			t.Fatalf("unexpected update HTTP request: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { updateHTTPClient = restore }()

	cfg := config.Default()
	cfg.BuildVersion = "3.0.0"
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/update/download", strings.NewReader(`{"current_version":"3.0.0"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "update_not_newer") {
		t.Fatalf("same-version status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRejectsClientCurrentVersionOverride(t *testing.T) {
	cfg := config.Default()
	cfg.BuildVersion = "3.0.0"
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/update/check?current_version=0.1.0", nil),
		httptest.NewRequest(http.MethodPost, "/update/download", strings.NewReader(`{"current_version":"0.1.0"}`)),
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "must match") {
			t.Fatalf("%s %s status=%d body=%s", req.Method, req.URL.Path, rec.Code, rec.Body.String())
		}
	}
}

func TestUpdateApplyResolvesDownloadsStagesAcknowledgesThenRequestsShutdown(t *testing.T) {
	zipBytes := []byte("archive-center-immediate-update")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	platform := detectUpdatePlatform(runtime.GOOS, runtime.GOARCH)
	assetName := updateTestAssetName("3.1", platform)
	requests := make([]string, 0, 3)
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.String())
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{"tag_name":"v3.1.0","assets":[{"name":`+strconv.Quote(assetName)+`,"browser_download_url":"https://example.test/package.zip"},{"name":"SHA256SUMS-3.1.txt","browser_download_url":"https://example.test/sums.txt"}]}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  "+assetName+"\n")
		case "https://example.test/package.zip":
			return bytesResponse(http.StatusOK, zipBytes)
		default:
			t.Fatalf("unexpected update HTTP request: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { updateHTTPClient = restore }()

	packageRoot := t.TempDir()
	installVerifiedUpdateHelper(t, packageRoot)
	cfg := config.Default()
	cfg.BuildVersion = "3.0.0"
	cfg.UpdateStagingDir = filepath.Join(packageRoot, ".updates")
	srv := NewServer(cfg)
	rec := httptest.NewRecorder()
	callbackCalls := 0
	srv.RequestShutdown = func(exitCode int) {
		callbackCalls++
		if exitCode != UpdateApplyExitCode {
			t.Fatalf("shutdown exit code = %d, want %d", exitCode, UpdateApplyExitCode)
		}
		var ack map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &ack); err != nil {
			t.Fatalf("shutdown callback ran before a complete JSON acknowledgement: %v body=%q", err, rec.Body.String())
		}
		if ack["status"] != "accepted" || ack["shutdown_requested"] != true || ack["exit_code"] != float64(UpdateApplyExitCode) {
			t.Fatalf("shutdown callback observed incomplete acknowledgement: %+v", ack)
		}
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/update/apply", strings.NewReader(`{}`)))
	if rec.Code != http.StatusOK || !rec.Flushed || callbackCalls != 1 {
		t.Fatalf("apply status=%d flushed=%t callback_calls=%d body=%s", rec.Code, rec.Flushed, callbackCalls, rec.Body.String())
	}
	if strings.Join(requests, "\n") != strings.Join([]string{
		"https://api.github.com/repos/Flazer31/archive-center/releases/latest",
		"https://example.test/sums.txt",
		"https://example.test/package.zip",
	}, "\n") {
		t.Fatalf("one-call apply request sequence = %v", requests)
	}
	if _, err := os.Stat(filepath.Join(packageRoot, ".updates", "pending-update.json")); err != nil {
		t.Fatalf("one-call apply did not stage pending update: %v", err)
	}
}

func TestUpdateApplyRejectsUnmanagedOrUnverifiedHelperBeforeNetwork(t *testing.T) {
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("rejected apply must not use network: %s", r.URL.String())
		return nil, nil
	})}
	defer func() { updateHTTPClient = restore }()

	for _, tc := range []struct {
		name         string
		withCallback bool
		wantCode     string
	}{
		{name: "unmanaged launcher", wantCode: "update_apply_unmanaged"},
		{name: "unverified helper", withCallback: true, wantCode: "update_apply_helper_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packageRoot := t.TempDir()
			if tc.withCallback {
				rel := updateTestHelperRelativePath()
				path := filepath.Join(packageRoot, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("plain unverified helper"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			cfg := config.Default()
			cfg.UpdateStagingDir = filepath.Join(packageRoot, ".updates")
			srv := NewServer(cfg)
			if tc.withCallback {
				srv.RequestShutdown = func(int) { t.Fatal("rejected apply requested shutdown") }
			}
			mux := http.NewServeMux()
			srv.RegisterRoutes(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/update/apply", strings.NewReader(`{}`)))
			if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), tc.wantCode) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUpdateRoutesRejectClientPlatformMismatch(t *testing.T) {
	runtimePlatform := detectUpdatePlatform(runtime.GOOS, runtime.GOARCH)
	mismatch := otherSupportedUpdatePlatform(runtimePlatform)
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("platform mismatch must be rejected before network: %s", r.URL.String())
		return nil, nil
	})}
	defer func() { updateHTTPClient = restore }()

	cfg := config.Default()
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/update/check?platform="+mismatch, nil),
		httptest.NewRequest(http.MethodPost, "/update/download", strings.NewReader(`{"platform":"`+mismatch+`"}`)),
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "update_platform_mismatch") {
			t.Fatalf("%s mismatch status=%d body=%s", req.URL.Path, rec.Code, rec.Body.String())
		}
	}
	if _, err := authoritativeUpdatePlatform("", "plan9", "amd64"); !errors.Is(err, errUpdatePlatformUnsupported) {
		t.Fatalf("unsupported runtime error = %v", err)
	}
}

func TestWindowsX64AssetMatcherExcludesArm64Package(t *testing.T) {
	if assetMatchesPlatform("Archive Center 3.1 Windows arm64 Update Package.zip", "windows-x64") {
		t.Fatal("windows-x64 matcher accepted the arm64 package")
	}
	if !assetMatchesPlatform("Archive Center 3.1 Windows Update Package.zip", "windows-x64") {
		t.Fatal("windows-x64 matcher rejected the x64 package")
	}
}

func TestParseOSReleaseDistributionObservesUbuntuID(t *testing.T) {
	if got := parseOSReleaseDistribution("NAME=Ubuntu\nID=ubuntu\nVERSION_ID=24.04\n"); got != "ubuntu" {
		t.Fatalf("distribution = %q, want ubuntu", got)
	}
	if got := parseOSReleaseDistribution("ID=\"Ubuntu\"\n"); got != "ubuntu" {
		t.Fatalf("quoted distribution = %q, want ubuntu", got)
	}
}

func TestSelectUpdateAssetMatchesDottedGitHubAssetNameAndSpacedSHAName(t *testing.T) {
	zipBytes := []byte("archive-center-2.3-linux-arm64-package")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	asset := selectUpdateAsset("linux-arm64", []githubAssetRecord{{
		Name:               "Archive.Center.2.3.Linux.arm64.Auto.Install.Package.zip",
		BrowserDownloadURL: "https://example.test/linux-arm64.zip",
		Size:               33,
	}}, map[string]string{"Archive Center 2.3 Linux arm64 Auto Install Package.zip": sha})
	if asset == nil || asset.Name != "Archive.Center.2.3.Linux.arm64.Auto.Install.Package.zip" || asset.SHA256 != sha {
		t.Fatalf("selected asset = %+v, want dotted linux arm64 asset with spaced SHA lookup", asset)
	}
}

func TestUpdateVersionComparisonDistinguishesPrereleaseFromFinal(t *testing.T) {
	for _, tc := range []struct {
		left  string
		right string
		want  int
	}{
		{left: "3.0.0", right: "3.0.0-rc2", want: 1},
		{left: "3.0.0-rc10", right: "3.0.0-rc2", want: 1},
		{left: "3.0.0-rc2", right: "3.0.0", want: -1},
		{left: "v3.1.0", right: "3.0.9", want: 1},
	} {
		if got := compareVersions(tc.left, tc.right); got != tc.want {
			t.Fatalf("compareVersions(%q, %q)=%d want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestSelectUpdateAssetPrefersManagedUpdatePackageOverFullInstallPackage(t *testing.T) {
	assets := []githubAssetRecord{
		{Name: "Archive Center 3.0.1 Windows Package.zip", BrowserDownloadURL: "https://example.test/full.zip", Size: 1000},
		{Name: "Archive Center 3.0.1 Windows Update Package.zip", BrowserDownloadURL: "https://example.test/update.zip", Size: 100},
	}
	shaMap := map[string]string{
		"Archive Center 3.0.1 Windows Package.zip":        strings.Repeat("a", 64),
		"Archive Center 3.0.1 Windows Update Package.zip": strings.Repeat("b", 64),
	}

	asset := selectUpdateAsset("windows-x64", assets, shaMap)
	if asset == nil {
		t.Fatal("expected a Windows update asset")
	}
	if asset.Name != "Archive Center 3.0.1 Windows Update Package.zip" || asset.DownloadURL != "https://example.test/update.zip" {
		t.Fatalf("selected asset = %+v, want managed update package", asset)
	}
	if asset.SHA256 != strings.Repeat("b", 64) {
		t.Fatalf("selected asset sha256 = %q", asset.SHA256)
	}
}

func textResponse(status int, text string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(text)),
	}, nil
}

func bytesResponse(status int, body []byte) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, nil
}

func updateTestAssetName(version, platform string) string {
	switch platform {
	case "windows-x64":
		return "Archive Center " + version + " Windows Update Package.zip"
	case "windows-arm64":
		return "Archive Center " + version + " Windows arm64 Update Package.zip"
	case "linux-x64":
		return "Archive Center " + version + " Linux x64 Update Package.zip"
	case "linux-arm64":
		return "Archive Center " + version + " Linux arm64 Update Package.zip"
	case "macos-intel":
		return "Archive Center " + version + " macOS Intel Update Package.zip"
	case "macos-apple-silicon":
		return "Archive Center " + version + " macOS Apple Silicon Update Package.zip"
	case "termux-arm64":
		return "Archive Center " + version + " Termux arm64 Update Package.zip"
	default:
		return "Archive Center " + version + " Unsupported Update Package.zip"
	}
}

func otherSupportedUpdatePlatform(platform string) string {
	if platform == "windows-x64" {
		return "linux-x64"
	}
	return "windows-x64"
}

func updateTestHelperRelativePath() string {
	if runtime.GOOS == "windows" {
		return "bin/archive-center-updater.exe"
	}
	return "bin/archive-center-updater"
}

func installVerifiedUpdateHelper(t *testing.T, root string) {
	t.Helper()
	rel := updateTestHelperRelativePath()
	body := []byte("verified update helper")
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sum := sha256.Sum256(body)
	manifest := map[string]any{
		"schema_version":  "archive-center.package-file-manifest.v1",
		"package_version": "3.0.0",
		"files": []map[string]any{{
			"path":       rel,
			"size_bytes": len(body),
			"sha256":     hex.EncodeToString(sum[:]),
		}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, packageupdate.ManifestName), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
