package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/canonpack"
	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCanonPackPreviewV1ProductionRoute(t *testing.T) {
	notice := []byte("neutral fixture notice")
	sum := sha256.Sum256(notice)
	valid := minimalCanonManifest()
	valid["files"] = []any{map[string]any{
		"path": "notices/NOTICE.txt", "role": "attribution_notice",
		"media_type": "text/plain; charset=utf-8", "size_bytes": len(notice),
		"sha256": hex.EncodeToString(sum[:]),
	}}
	highlyCompressed := bytes.Repeat([]byte{0}, 1<<20)
	highlyCompressedSum := sha256.Sum256(highlyCompressed)
	highlyCompressedManifest := mutateCanonManifest(valid, func(m map[string]any) {
		m["files"] = []any{map[string]any{
			"path": "notices/NOTICE.txt", "role": "attribution_notice",
			"media_type": "text/plain; charset=utf-8", "size_bytes": len(highlyCompressed),
			"sha256": hex.EncodeToString(highlyCompressedSum[:]),
		}}
	})
	signature := []byte("fixture signature bytes")
	signatureSum := sha256.Sum256(signature)
	signedManifest := func(status string) map[string]any {
		return mutateCanonManifest(valid, func(m map[string]any) {
			m["trust"] = map[string]any{
				"status":          status,
				"publisher":       map[string]any{"id": "example.publisher:fixture", "display_name": "Fixture"},
				"signature_file":  "signatures/manifest.sig",
				"trust_policy_id": "example.trust:fixture",
			}
			files := m["files"].([]any)
			m["files"] = append(files, map[string]any{
				"path": "signatures/manifest.sig", "role": "manifest_signature",
				"media_type": "application/octet-stream", "size_bytes": len(signature),
				"sha256": hex.EncodeToString(signatureSum[:]),
			})
		})
	}

	tests := []struct {
		name         string
		manifest     map[string]any
		manifestRaw  []byte
		entries      []canonZipEntry
		buildVersion string
		wantStatus   int
		wantCode     string
	}{
		{name: "valid neutral fixture", manifest: valid, entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice}}, wantStatus: http.StatusOK},
		{name: "unsupported contract", manifest: mutateCanonManifest(valid, func(m map[string]any) { m["contract"] = "canon-pack-manifest.v9" }), wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_contract_unsupported"},
		{name: "invalid schema required property", manifest: mutateCanonManifest(valid, func(m map[string]any) { delete(m, "review") }), wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_schema_invalid"},
		{name: "checksum mismatch", manifest: valid, entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: []byte("changed")}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_file_checksum_mismatch"},
		{name: "missing listed file", manifest: valid, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_listed_file_missing"},
		{name: "extra unlisted file", manifest: mutateCanonManifest(valid, func(m map[string]any) { m["files"] = []any{} }), entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_unlisted_member"},
		{name: "unsafe traversal path", manifest: valid, entries: []canonZipEntry{{name: "../NOTICE.txt", body: notice}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_unsafe_path"},
		{name: "unsafe backslash path", manifest: valid, entries: []canonZipEntry{{name: `notices\NOTICE.txt`, body: notice}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_unsafe_path"},
		{name: "unsafe symlink", manifest: mutateCanonManifest(valid, func(m map[string]any) { m["files"] = []any{} }), entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice, mode: os.ModeSymlink | 0o777}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_unsafe_member_type"},
		{name: "executable member", manifest: valid, entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice, mode: 0o755}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_executable_member"},
		{name: "duplicate member", manifest: valid, entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice}, {name: "notices/NOTICE.txt", body: notice}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_duplicate_member"},
		{name: "compression ratio limit", manifest: highlyCompressedManifest, entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: highlyCompressed}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_compression_ratio_exceeded"},
		{name: "duplicate JSON key", manifestRaw: []byte(`{"contract":"canon-pack-manifest.v1","contract":"canon-pack-manifest.v1"}`), wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_json_duplicate_key"},
		{name: "multiple JSON values", manifestRaw: []byte(`{} {}`), wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_manifest_json_invalid"},
		{name: "evidence source hash mismatch", manifest: mutateCanonManifest(valid, func(m map[string]any) {
			entity := m["content"].(map[string]any)["entities"].([]any)[0].(map[string]any)
			entity["evidence"].([]any)[0].(map[string]any)["document_sha256"] = strings.Repeat("2", 64)
		}), wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_evidence_hash_mismatch"},
		{name: "encoded secret query parameter", manifest: mutateCanonManifest(valid, func(m map[string]any) {
			m["sources"].([]any)[0].(map[string]any)["uri"] = "https://example.invalid/guide?access%5Ftoken=secret"
		}), entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_source_uri_secret"},
		{name: "incompatible Archive Center", manifest: mutateCanonManifest(valid, func(m map[string]any) {
			ac := m["compatibility"].(map[string]any)["archive_center"].(map[string]any)
			ac["minimum_version"] = "4.0.0"
		}), wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_archive_center_incompatible"},
		{name: "prerelease is below final minimum", manifest: valid, entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice}}, buildVersion: "3.1.0-rc.1", wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_archive_center_incompatible"},
		{name: "trusted claim needs cryptographic verifier", manifest: signedManifest("trusted_publisher"), entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice}, {name: "signatures/manifest.sig", body: signature}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_trust_verification_unavailable"},
		{name: "invalid signature status is rejected", manifest: signedManifest("invalid_signature"), entries: []canonZipEntry{{name: "notices/NOTICE.txt", body: notice}, {name: "signatures/manifest.sig", body: signature}}, wantStatus: http.StatusUnprocessableEntity, wantCode: "canon_pack_signature_invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifestRaw := tc.manifestRaw
			if manifestRaw == nil {
				manifestRaw, _ = json.Marshal(tc.manifest)
			}
			payload := makeCanonZIP(t, manifestRaw, tc.entries)
			cfg := config.Default()
			cfg.BuildVersion = tc.buildVersion
			if cfg.BuildVersion == "" {
				cfg.BuildVersion = "3.1.0"
			}
			s := &Server{Cfg: cfg}
			mux := http.NewServeMux()
			s.RegisterRoutes(mux)
			req := httptest.NewRequest(http.MethodPost, "/canon-packs/preview/v1", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/zip")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode != "" && !strings.Contains(rec.Body.String(), `"code":"`+tc.wantCode+`"`) {
				t.Fatalf("missing diagnostic %q: %s", tc.wantCode, rec.Body.String())
			}
			if tc.wantStatus == http.StatusOK && !strings.Contains(rec.Body.String(), `"valid":true`) {
				t.Fatalf("valid response missing: %s", rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"validation_profile":"canon-pack-manifest.v1-install-preview-subset"`) {
				t.Fatalf("validation profile missing: %s", rec.Body.String())
			}
		})
	}
}

func TestCanonPackPreviewV1RejectsOversizedBodyAndWrongMediaType(t *testing.T) {
	cfg := config.Default()
	cfg.BuildVersion = "3.1.0"
	s := &Server{Cfg: cfg}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	wrongType := httptest.NewRequest(http.MethodPost, "/canon-packs/preview/v1", strings.NewReader("not a zip"))
	wrongType.Header.Set("Content-Type", "application/json")
	wrongTypeRecorder := httptest.NewRecorder()
	mux.ServeHTTP(wrongTypeRecorder, wrongType)
	if wrongTypeRecorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong media type status=%d body=%s", wrongTypeRecorder.Code, wrongTypeRecorder.Body.String())
	}

	oversized := httptest.NewRequest(http.MethodPost, "/canon-packs/preview/v1", bytes.NewReader(make([]byte, canonpack.MaxArchiveBytes+1)))
	oversized.Header.Set("Content-Type", "application/zip")
	oversizedRecorder := httptest.NewRecorder()
	mux.ServeHTTP(oversizedRecorder, oversized)
	if oversizedRecorder.Code != http.StatusRequestEntityTooLarge || !strings.Contains(oversizedRecorder.Body.String(), "canon_pack_archive_too_large") {
		t.Fatalf("oversized status=%d body=%s", oversizedRecorder.Code, oversizedRecorder.Body.String())
	}
}

type canonPackLifecycleHTTPStore struct {
	store.Store
	installInput store.CanonPackInstallInput
	action       string
	installID    string
}

func (s *canonPackLifecycleHTTPStore) InstallCanonPack(_ context.Context, input store.CanonPackInstallInput) (*store.CanonPackInstall, error) {
	s.installInput = input
	return &store.CanonPackInstall{Contract: store.CanonPackLifecycleContract, InstallID: "install-1", LifecycleStatus: "active"}, nil
}

func (s *canonPackLifecycleHTTPStore) ListCanonPackInstalls(_ context.Context, lifecycle string) ([]store.CanonPackInstall, error) {
	return []store.CanonPackInstall{{Contract: store.CanonPackLifecycleContract, InstallID: "install-1", LifecycleStatus: lifecycle, InstalledAt: time.Unix(1, 0)}}, nil
}

func (s *canonPackLifecycleHTTPStore) GetCanonPackInstall(_ context.Context, installID string) (*store.CanonPackInstall, error) {
	return &store.CanonPackInstall{Contract: store.CanonPackLifecycleContract, InstallID: installID, LifecycleStatus: "active"}, nil
}

func (s *canonPackLifecycleHTTPStore) SetCanonPackLifecycle(_ context.Context, installID, action string) (*store.CanonPackLifecycleResult, error) {
	s.installID, s.action = installID, action
	return &store.CanonPackLifecycleResult{Contract: store.CanonPackLifecycleContract, InstallID: installID, Action: action, LifecycleStatus: "inactive"}, nil
}

func (s *canonPackLifecycleHTTPStore) SearchCanonRegistry(_ context.Context, query string, _ int) ([]store.CanonRegistryItem, error) {
	return []store.CanonRegistryItem{{InstallID: "install-1", Title: query, LifecycleStatus: "active"}}, nil
}

func (s *canonPackLifecycleHTTPStore) GetCanonPackDiagnostics(_ context.Context, installID string) (*store.CanonPackDiagnostics, error) {
	return &store.CanonPackDiagnostics{Contract: store.CanonPackDiagnosticsContract, InstallID: installID, Quality: map[string]any{"traceability": "satisfied"}}, nil
}

func (s *canonPackLifecycleHTTPStore) CreateCanonOverlay(_ context.Context, input store.CanonOverlayInput) (*store.CanonOverlayRule, error) {
	return &store.CanonOverlayRule{OverlayRuleID: "overlay-1", WorkID: input.WorkID, EditionRowID: input.EditionRowID, Action: input.Action}, nil
}

func (s *canonPackLifecycleHTTPStore) ListCanonOverlays(_ context.Context, workID, editionRowID string) ([]store.CanonOverlayRule, error) {
	return []store.CanonOverlayRule{{OverlayRuleID: "overlay-1", WorkID: workID, EditionRowID: editionRowID}}, nil
}

func (s *canonPackLifecycleHTTPStore) SaveSourceDiscoveryJob(_ context.Context, _ store.SourceDiscoveryInput, state string, result, coverage map[string]any) (*store.SourceDiscoveryJob, error) {
	return &store.SourceDiscoveryJob{Contract: store.SourceDiscoveryContract, JobID: "job-1", State: state, Result: result, CoverageReport: coverage}, nil
}

func (s *canonPackLifecycleHTTPStore) GetSourceDiscoveryJob(_ context.Context, jobID string) (*store.SourceDiscoveryJob, error) {
	return &store.SourceDiscoveryJob{Contract: store.SourceDiscoveryContract, JobID: jobID, State: "insufficient_source_coverage"}, nil
}

func TestCanonPackInstallAndLifecycleV1Routes(t *testing.T) {
	manifest, err := json.Marshal(minimalCanonManifest())
	if err != nil {
		t.Fatal(err)
	}
	payload := makeCanonZIP(t, manifest, nil)
	fake := &canonPackLifecycleHTTPStore{}
	cfg := config.Default()
	cfg.BuildVersion = "3.1.0"
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := &Server{Cfg: cfg, Store: fake}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/canon-packs/install/v1", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/zip")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("install status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"index_status":"pending_reindex"`) {
		t.Fatalf("install did not expose pending reference reindex: %s", rec.Body.String())
	}
	archiveHash := sha256.Sum256(payload)
	manifestHash := sha256.Sum256(manifest)
	if fake.installInput.ArchiveSHA256 != hex.EncodeToString(archiveHash[:]) || fake.installInput.ManifestSHA256 != hex.EncodeToString(manifestHash[:]) {
		t.Fatalf("install did not bind exact bytes: %#v", fake.installInput)
	}
	if !bytes.Equal(fake.installInput.ManifestJSON, manifest) {
		t.Fatal("install store did not receive the exact manifest bytes")
	}

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/canon-packs/v1?lifecycle_status=active", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"count":1`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	detail := httptest.NewRecorder()
	mux.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/canon-packs/install-1/v1", nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"install_id":"install-1"`) {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}

	lifecycle := httptest.NewRecorder()
	mux.ServeHTTP(lifecycle, httptest.NewRequest(http.MethodPost, "/canon-packs/install-1/lifecycle/v1", strings.NewReader(`{"action":"deactivate"}`)))
	if lifecycle.Code != http.StatusOK || fake.installID != "install-1" || fake.action != "deactivate" {
		t.Fatalf("lifecycle status=%d body=%s call=%q/%q", lifecycle.Code, lifecycle.Body.String(), fake.installID, fake.action)
	}
	if !strings.Contains(lifecycle.Body.String(), `"index_status":"pending_reindex"`) {
		t.Fatalf("lifecycle did not expose pending reference reindex: %s", lifecycle.Body.String())
	}
}

func TestCanonPackLifecycleRefreshesOnlyReferenceVectorCollection(t *testing.T) {
	fake := referenceVectorFixtureStore()
	embeddingServer, calls := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{}
	srv := &Server{
		Cfg:   config.Config{ReferenceChromaCollection: "archive_center_reference_vectors"},
		Store: fake, ReferenceVector: vectorStore,
		RuntimeConfig: RuntimeConfig{
			Synced: true, EmbeddingProvider: "openai", EmbeddingAPIKey: "key",
			EmbeddingEndpoint: embeddingServer.URL, EmbeddingModel: "embed-reference",
		},
	}
	status, result := srv.refreshCanonPackReferenceIndex(context.Background(), &store.CanonPackInstall{WorkID: "work-1"}, nil)
	if status != "current" || *calls != 3 || len(vectorStore.upserted) != 3 {
		t.Fatalf("status=%q calls=%d docs=%d result=%#v", status, *calls, len(vectorStore.upserted), result)
	}
	for _, doc := range vectorStore.upserted {
		if doc.ChatSessionID != "work-1" || doc.Metadata["work_id"] != "work-1" {
			t.Fatalf("reference lifecycle wrote outside work-scoped reference collection: %#v", doc)
		}
	}
}

func TestCanonPackInstallRequiresMariaDBAuthority(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeNoop
	srv := &Server{Cfg: cfg, Store: &canonPackLifecycleHTTPStore{}}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/canon-packs/install/v1", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "canon_pack_install_unavailable") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCanonRegistryOverlayDiagnosticsAndDiscoveryRoutes(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := &Server{Cfg: cfg, Store: &canonPackLifecycleHTTPStore{}}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	tests := []struct {
		method string
		path   string
		body   string
		want   string
	}{
		{http.MethodGet, "/canon-registry/v1?q=Neutral", "", `"contract":"canon_registry_search.v1"`},
		{http.MethodGet, "/canon-packs/install-1/diagnostics/v1", "", `"contract":"canon_pack_diagnostics.v1"`},
		{http.MethodPost, "/canon-overlays/v1", `{"work_id":"work-1","edition_row_id":"edition-1","target_kind":"claim","target_id":"claim-1","action":"suppress_for_retrieval"}`, `"contract":"canon_overlay_write.v1"`},
		{http.MethodGet, "/canon-overlays/v1?work_id=work-1&edition_row_id=edition-1", "", `"contract":"canon_overlay_list.v1"`},
		{http.MethodPost, "/source-discovery/jobs/v1", `{"work_query":"Neutral","allowed_source_types":["official_primary"]}`, `"state":"insufficient_source_coverage"`},
		{http.MethodPost, "/source-discovery/preview/v1", `{"work_query":"Neutral","allowed_source_types":["official_primary"]}`, `"db_write":false`},
		{http.MethodGet, "/source-discovery/jobs/job-1/v1", "", `"job_id":"job-1"`},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code < 200 || rec.Code >= 300 || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

type canonZipEntry struct {
	name string
	body []byte
	mode os.FileMode
}

func makeCanonZIP(t *testing.T, manifest []byte, entries []canonZipEntry) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	all := append([]canonZipEntry{{name: "canon-pack-manifest.json", body: manifest}}, entries...)
	for _, entry := range all {
		h := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			h.SetMode(entry.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func mutateCanonManifest(src map[string]any, fn func(map[string]any)) map[string]any {
	b, _ := json.Marshal(src)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	fn(out)
	return out
}

func minimalCanonManifest() map[string]any {
	h := strings.Repeat("1", 64)
	evidence := []any{map[string]any{"source_id": "guide", "document_sha256": h, "locator": map[string]any{"type": "record", "value": "one"}}}
	return map[string]any{
		"contract":   "canon-pack-manifest.v1",
		"pack":       map[string]any{"id": "example.publisher:neutral", "version": "1.0.0", "status": "published", "built_at": "2026-07-18T00:00:00Z"},
		"work":       map[string]any{"stable_id": "example.work:neutral", "original_title": map[string]any{"text": "Neutral", "language": "en"}, "translated_titles": []any{}, "aliases": []any{}, "original_language": "en", "content_languages": []any{"en"}, "edition": map[string]any{"edition_id": "example.edition:neutral", "label": "Neutral", "language": "en", "scope_note": "Neutral fictional fixture."}, "continuity_ids": []any{"example.continuity:main"}},
		"production": map[string]any{"method": "human_curated", "created_at": "2026-07-18T00:00:00Z", "producer": map[string]any{"id": "example.publisher:fixture", "display_name": "Fixture"}},
		"review":     map[string]any{"status": "approved", "procedure_id": "example.review:fixture", "reviewed_at": "2026-07-18T00:00:00Z", "admission_basis": "evidence_validated_batch"},
		"trust":      map[string]any{"status": "unsigned_local"},
		"sources":    []any{map[string]any{"id": "guide", "title": "Guide", "source_type": "official_primary", "uri": "urn:example:guide", "license": map[string]any{"name": "Fixture", "redistribution_allowed": true}, "access_class": "open_license", "retrieved_at": "2026-07-18T00:00:00Z", "document_sha256": h}},
		"content":    map[string]any{"entities": []any{map[string]any{"id": "person", "name": map[string]any{"text": "Person", "language": "en"}, "kind": "person", "continuity_ids": []any{"example.continuity:main"}, "review_state": "approved", "evidence": evidence}}, "locations": []any{}, "factions": []any{}, "settings": []any{}, "events": []any{}, "relations": []any{}, "claims": []any{}},
		"conflicts":  []any{}, "uncertainties": []any{},
		"coverage_report": map[string]any{"generated_at": "2026-07-18T00:00:00Z", "saturation": "not_assessed", "domains": []any{map[string]any{"domain": "entities", "status": "partial", "missing_topics": []any{}}}},
		"compatibility":   map[string]any{"archive_center": map[string]any{"minimum_version": "3.1.0", "maximum_version_exclusive": "4.0.0"}, "schema": map[string]any{"name": "canon-pack-manifest", "version": "1.0.0"}},
		"files":           []any{},
	}
}
