package httpapi

import (
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
	if resp["apply_supported"] != false || resp["download_supported"] != true {
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
	packageRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(packageRoot, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, "bin", "archive-center-updater.exe"), []byte("test helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.UpdateStagingDir = filepath.Join(packageRoot, ".updates")
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

func TestUpdateDownloadRejectsClientSHAOverride(t *testing.T) {
	zipBytes := []byte("verified release asset")
	sum := sha256.Sum256(zipBytes)
	sha := hex.EncodeToString(sum[:])
	restore := updateHTTPClient
	updateHTTPClient = &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.github.com/repos/Flazer31/archive-center/releases/latest":
			return textResponse(http.StatusOK, `{"tag_name":"v3.1.0","assets":[{"name":"Archive Center 3.1 Windows Package.zip","browser_download_url":"https://example.test/windows.zip"},{"name":"SHA256SUMS-3.1.txt","browser_download_url":"https://example.test/sums.txt"}]}`)
		case "https://example.test/sums.txt":
			return textResponse(http.StatusOK, sha+"  Archive Center 3.1 Windows Package.zip\n")
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
	req := httptest.NewRequest(http.MethodPost, "/update/download", strings.NewReader(`{"current_version":"3.0.0","platform":"windows-x64","expected_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
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
	req := httptest.NewRequest(http.MethodPost, "/update/download", strings.NewReader(`{"current_version":"3.0.0","platform":"windows-x64"}`))
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
