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
	if resp["status"] != "ok" || resp["sha256"] != sha || resp["apply_supported"] != false {
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
