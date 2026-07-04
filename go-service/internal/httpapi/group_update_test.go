package httpapi

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

type updateRoundTripFunc func(*http.Request) (*http.Response, error)

func (f updateRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestUpdateCheckSelectsPlatformAssetAndSHA256(t *testing.T) {
	zipBytes := []byte("archive-center-2.3-windows-package")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{
				"tag_name":"v2.3.0",
				"name":"Archive Center 2.3",
				"html_url":"https://github.com/Flazer31/archive-center/releases/tag/v2.3.0",
				"assets":[
					{"name":"Archive Center 2.3 Windows Package.zip","browser_download_url":"https://example.test/windows.zip","size":33},
					{"name":"SHA256SUMS-2.3.txt","browser_download_url":"https://example.test/sums.txt","size":90}
				]
			}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  Archive Center 2.3 Windows Package.zip\n")
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

	req := httptest.NewRequest(http.MethodGet, "/update/check?platform=windows-x64", nil)
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
	if asset["name"] != "Archive Center 2.3 Windows Package.zip" || asset["sha256"] != sha {
		t.Fatalf("selected_asset = %+v, want windows asset with sha", asset)
	}
	if resp["apply_supported"] != true || resp["download_supported"] != true {
		t.Fatalf("support flags unexpected: %+v", resp)
	}
}

func TestUpdateDownloadStagesVerifiedAsset(t *testing.T) {
	zipBytes := []byte("archive-center-2.3-windows-package")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{
				"tag_name":"v2.3.0",
				"name":"Archive Center 2.3",
				"assets":[
					{"name":"Archive Center 2.3 Windows Package.zip","browser_download_url":"https://example.test/windows.zip","size":33},
					{"name":"SHA256SUMS-2.3.txt","browser_download_url":"https://example.test/sums.txt","size":90}
				]
			}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  Archive Center 2.3 Windows Package.zip\n")
		case "https://example.test/windows.zip":
			return bytesResponse(http.StatusOK, zipBytes)
		default:
			t.Fatalf("unexpected update HTTP request: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { updateHTTPClient = restore }()

	cfg := config.Default()
	cfg.BuildVersion = "2.2.0"
	cfg.UpdateStagingDir = t.TempDir()
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"platform":"windows-x64","current_version":"2.2.0"}`
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
	if resp["status"] != "ok" || resp["sha256"] != sha || resp["apply_supported"] != true {
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
}

func TestUpdateDownloadAppliesVerifiedAssetWhenRequested(t *testing.T) {
	zipBytes := updatePackageZipBytes(t)
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{
				"tag_name":"v2.3.0",
				"name":"Archive Center 2.3",
				"assets":[
					{"name":"Archive Center 2.3 Linux arm64 Auto Install Package.zip","browser_download_url":"https://example.test/linux-arm64.zip","size":333},
					{"name":"SHA256SUMS-2.3.txt","browser_download_url":"https://example.test/sums.txt","size":90}
				]
			}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  Archive Center 2.3 Linux arm64 Auto Install Package.zip\n")
		case "https://example.test/linux-arm64.zip":
			return bytesResponse(http.StatusOK, zipBytes)
		default:
			t.Fatalf("unexpected update HTTP request: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { updateHTTPClient = restore }()

	installRoot := t.TempDir()
	cfg := config.Default()
	cfg.BuildVersion = "2.2.0"
	cfg.UpdateStagingDir = filepath.Join(installRoot, "releases", "v2.2.0", ".updates")
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"platform":"linux-arm64","current_version":"2.2.0","apply":true,"restart_service":false}`
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
	if resp["apply_status"] != "applied" || resp["restart_scheduled"] != false {
		t.Fatalf("unexpected apply response: %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "releases", "v2.3.0", "Archive Center 2.3 Linux arm64 Auto Install Package", "start-archive-center-linux.sh")); err != nil {
		t.Fatalf("applied package launcher missing: %v", err)
	}
	versionBytes, err := os.ReadFile(filepath.Join(installRoot, "current-version.txt"))
	if err != nil {
		t.Fatalf("current-version missing: %v", err)
	}
	if strings.TrimSpace(string(versionBytes)) != "v2.3.0" {
		t.Fatalf("current-version = %q", string(versionBytes))
	}
	pointerBytes, err := os.ReadFile(filepath.Join(installRoot, "current-package.txt"))
	if err != nil {
		t.Fatalf("current-package missing: %v", err)
	}
	if !strings.Contains(string(pointerBytes), filepath.Join("releases", "v2.3.0")) {
		t.Fatalf("current-package = %q", string(pointerBytes))
	}
}

func TestUpdateCheckMatchesDottedGitHubAssetNameAndSpacedSHAName(t *testing.T) {
	zipBytes := []byte("archive-center-2.3-linux-arm64-package")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{
				"tag_name":"v2.3.0-rc2",
				"name":"Archive Center 2.3 RC2",
				"assets":[
					{"name":"Archive.Center.2.3.Linux.arm64.Auto.Install.Package.zip","browser_download_url":"https://example.test/linux-arm64.zip","size":33},
					{"name":"SHA256SUMS-2.3.txt","browser_download_url":"https://example.test/sums.txt","size":90}
				]
			}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  Archive Center 2.3 Linux arm64 Auto Install Package.zip\n")
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

	req := httptest.NewRequest(http.MethodGet, "/update/check?platform=linux-arm64", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	asset, ok := resp["selected_asset"].(map[string]any)
	if !ok {
		t.Fatalf("selected_asset missing: %+v", resp)
	}
	if asset["name"] != "Archive.Center.2.3.Linux.arm64.Auto.Install.Package.zip" || asset["sha256"] != sha {
		t.Fatalf("selected_asset = %+v, want dotted linux arm64 asset with spaced SHA lookup", asset)
	}
	if resp["download_supported"] != true {
		t.Fatalf("download_supported unexpected: %+v", resp)
	}
}

func TestCompareVersionsHandlesRCProgression(t *testing.T) {
	if compareVersions("2.5.0-rc2", "2.5.0-rc1") <= 0 {
		t.Fatalf("rc2 should be newer than rc1")
	}
	if compareVersions("2.5.0", "2.5.0-rc9") <= 0 {
		t.Fatalf("stable release should be newer than rc")
	}
	if compareVersions("2.5.0-rc1", "2.5.0") >= 0 {
		t.Fatalf("rc should be older than stable release")
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

func updatePackageZipBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"Archive Center 2.3 Linux arm64 Auto Install Package/start-archive-center-linux.sh": "#!/bin/sh\nexit 0\n",
		"Archive Center 2.3 Linux arm64 Auto Install Package/Archive Center.js":             "// plugin\n",
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create: %v", err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}
