package canonpack

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	PreviewContract           = "canon-pack-preview.v1"
	ValidationProfile         = "canon-pack-manifest.v1-install-preview-subset"
	ManifestContract          = "canon-pack-manifest.v1"
	ManifestName              = "canon-pack-manifest.json"
	MaxArchiveBytes     int64 = 16 << 20
	maxManifestBytes          = 4 << 20
	maxExpandedBytes          = 8 << 20
	maxMembers                = 16
	maxCompressionRatio       = 100
)

type Diagnostic struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type Summary struct {
	PackID           string         `json:"pack_id,omitempty"`
	PackVersion      string         `json:"pack_version,omitempty"`
	StableWorkID     string         `json:"stable_work_id,omitempty"`
	EditionID        string         `json:"edition_id,omitempty"`
	ReviewStatus     string         `json:"review_status,omitempty"`
	TrustStatus      string         `json:"trust_status,omitempty"`
	SourceCount      int            `json:"source_count"`
	RecordCounts     map[string]int `json:"record_counts"`
	ConflictCount    int            `json:"conflict_count"`
	UncertaintyCount int            `json:"uncertainty_count"`
	AncillaryFiles   int            `json:"ancillary_files"`
	ExpandedBytes    int64          `json:"expanded_bytes"`
}

type Report struct {
	Contract          string       `json:"contract"`
	ValidationProfile string       `json:"validation_profile"`
	Valid             bool         `json:"valid"`
	Diagnostics       []Diagnostic `json:"diagnostics"`
	Summary           Summary      `json:"summary"`
}

// Inspection is the installation handoff produced from the exact ZIP bytes
// that passed PreviewZIP. Manifest bytes are kept out of the public preview
// response and are only handed to the backend installer.
type Inspection struct {
	Report         Report
	ArchiveSHA256  string
	ManifestSHA256 string
	ManifestJSON   []byte
}

func InspectZIP(data []byte, archiveCenterVersion string) (Inspection, error) {
	report := PreviewZIP(data, archiveCenterVersion)
	inspection := Inspection{Report: report}
	archiveSum := sha256.Sum256(data)
	inspection.ArchiveSHA256 = hex.EncodeToString(archiveSum[:])
	if !report.Valid {
		return inspection, nil
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return inspection, err
	}
	for _, f := range zr.File {
		if f.Name != ManifestName {
			continue
		}
		manifest, err := readMember(f, maxManifestBytes)
		if err != nil {
			return inspection, err
		}
		manifestSum := sha256.Sum256(manifest)
		inspection.ManifestSHA256 = hex.EncodeToString(manifestSum[:])
		inspection.ManifestJSON = manifest
		return inspection, nil
	}
	return inspection, errors.New("validated canon pack manifest disappeared")
}

type collector struct{ diagnostics []Diagnostic }

func (c *collector) add(code, p, message string) {
	c.diagnostics = append(c.diagnostics, Diagnostic{Code: code, Path: p, Message: message})
}

func PreviewZIP(data []byte, archiveCenterVersion string) Report {
	report := Report{Contract: PreviewContract, ValidationProfile: ValidationProfile, Summary: Summary{RecordCounts: map[string]int{}}}
	c := &collector{}
	if int64(len(data)) > MaxArchiveBytes {
		c.add("canon_pack_archive_too_large", "", "compressed archive exceeds the preview limit")
		return finish(report, c)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		c.add("canon_pack_archive_invalid", "", "payload is not a readable ZIP archive")
		return finish(report, c)
	}
	if len(zr.File) > maxMembers {
		c.add("canon_pack_member_limit_exceeded", "", "archive contains too many members")
		return finish(report, c)
	}
	seen := map[string]bool{}
	var manifestFile *zip.File
	var expanded int64
	for _, f := range zr.File {
		name, ok := safeMemberName(f.Name)
		if !ok {
			c.add("canon_pack_unsafe_path", f.Name, "archive member path is not a normalized relative slash path")
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			c.add("canon_pack_duplicate_member", name, "archive member path is duplicated")
			continue
		}
		seen[key] = true
		mode := f.Mode()
		if f.FileInfo().IsDir() || !mode.IsRegular() {
			c.add("canon_pack_unsafe_member_type", name, "only regular files are permitted")
			continue
		}
		if mode.Perm()&0o111 != 0 {
			c.add("canon_pack_executable_member", name, "executable archive members are forbidden")
		}
		if f.UncompressedSize64 > uint64(maxExpandedBytes) || expanded > maxExpandedBytes-int64(f.UncompressedSize64) {
			c.add("canon_pack_expanded_size_exceeded", name, "expanded archive size exceeds the preview limit")
			continue
		}
		expanded += int64(f.UncompressedSize64)
		compressed := f.CompressedSize64
		if compressed == 0 {
			if f.UncompressedSize64 > 0 {
				c.add("canon_pack_compression_ratio_exceeded", name, "archive member compression ratio exceeds the preview limit")
			}
		} else if f.UncompressedSize64 > compressed*maxCompressionRatio {
			c.add("canon_pack_compression_ratio_exceeded", name, "archive member compression ratio exceeds the preview limit")
		}
		if name == ManifestName {
			manifestFile = f
		}
	}
	report.Summary.ExpandedBytes = expanded
	if manifestFile == nil {
		c.add("canon_pack_manifest_missing", ManifestName, "archive root manifest is required")
		return finish(report, c)
	}
	if manifestFile.UncompressedSize64 > maxManifestBytes {
		c.add("canon_pack_manifest_too_large", ManifestName, "manifest exceeds the preview limit")
		return finish(report, c)
	}
	manifestBytes, err := readMember(manifestFile, maxManifestBytes)
	if err != nil {
		c.add("canon_pack_manifest_read_failed", ManifestName, "manifest could not be read within its declared size")
		return finish(report, c)
	}
	if err := rejectDuplicateJSONKeys(manifestBytes); err != nil {
		code := "canon_pack_manifest_json_invalid"
		if errors.Is(err, errDuplicateJSONKey) {
			code = "canon_pack_json_duplicate_key"
		}
		c.add(code, ManifestName, err.Error())
		return finish(report, c)
	}
	var root any
	dec := json.NewDecoder(bytes.NewReader(manifestBytes))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		c.add("canon_pack_manifest_json_invalid", ManifestName, "manifest is not valid JSON")
		return finish(report, c)
	}
	m, ok := root.(map[string]any)
	if !ok {
		c.add("canon_pack_schema_invalid", "$", "manifest root must be an object")
		return finish(report, c)
	}
	validateManifest(m, zr.File, archiveCenterVersion, &report, c)
	return finish(report, c)
}

func finish(r Report, c *collector) Report {
	sort.SliceStable(c.diagnostics, func(i, j int) bool {
		if c.diagnostics[i].Path == c.diagnostics[j].Path {
			return c.diagnostics[i].Code < c.diagnostics[j].Code
		}
		return c.diagnostics[i].Path < c.diagnostics[j].Path
	})
	r.Diagnostics = c.diagnostics
	r.Valid = len(c.diagnostics) == 0
	return r
}

func safeMemberName(name string) (string, bool) {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return "", false
	}
	if len(name) >= 2 && name[1] == ':' {
		return "", false
	}
	if path.Clean(name) != name || strings.HasPrefix(name, "../") || name == ".." || strings.Contains(name, "//") {
		return "", false
	}
	return name, true
}

func readMember(f *zip.File, limit int64) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil || int64(len(b)) > limit || uint64(len(b)) != f.UncompressedSize64 {
		return nil, fmt.Errorf("bounded read failed")
	}
	return b, nil
}

var errDuplicateJSONKey = errors.New("duplicate JSON object key")

func rejectDuplicateJSONKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := keyTok.(string)
				if !ok {
					return fmt.Errorf("invalid object key")
				}
				if seen[key] {
					return fmt.Errorf("%w %q", errDuplicateJSONKey, key)
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := dec.Token(); err == nil {
		return fmt.Errorf("multiple JSON values")
	} else if err != io.EOF {
		return err
	}
	return nil
}

var (
	identifierRE = regexp.MustCompile(`^[a-z][a-z0-9._-]*:[A-Za-z0-9][A-Za-z0-9._:-]*$`)
	localIDRE    = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
	shaRE        = regexp.MustCompile(`^[a-f0-9]{64}$`)
	semverRE     = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
)

func validateManifest(m map[string]any, members []*zip.File, acVersion string, report *Report, c *collector) {
	requireKeys(m, "$", []string{"contract", "pack", "work", "production", "review", "trust", "sources", "content", "conflicts", "uncertainties", "coverage_report", "compatibility", "files"}, c)
	if str(m["contract"]) != ManifestContract {
		c.add("canon_pack_contract_unsupported", "$.contract", "manifest contract must be canon-pack-manifest.v1")
	}
	pack := obj(m["pack"])
	requireKeys(pack, "$.pack", []string{"id", "version", "status", "built_at"}, c)
	report.Summary.PackID, report.Summary.PackVersion = str(pack["id"]), str(pack["version"])
	validateIdentifier(report.Summary.PackID, "$.pack.id", c)
	validateSemver(report.Summary.PackVersion, "$.pack.version", c)
	validateTime(str(pack["built_at"]), "$.pack.built_at", c)
	if !oneOf(str(pack["status"]), "draft", "review_candidate", "published", "deprecated", "withdrawn") {
		c.add("canon_pack_schema_invalid", "$.pack.status", "invalid pack status")
	}
	work := obj(m["work"])
	requireKeys(work, "$.work", []string{"stable_id", "original_title", "translated_titles", "aliases", "original_language", "content_languages", "edition", "continuity_ids"}, c)
	report.Summary.StableWorkID = str(work["stable_id"])
	validateIdentifier(report.Summary.StableWorkID, "$.work.stable_id", c)
	continuities := stringSet(work["continuity_ids"], "$.work.continuity_ids", true, c)
	for _, id := range continuities {
		validateIdentifier(id, "$.work.continuity_ids", c)
	}
	edition := obj(work["edition"])
	requireKeys(edition, "$.work.edition", []string{"edition_id", "label", "language", "scope_note"}, c)
	report.Summary.EditionID = str(edition["edition_id"])
	validateIdentifier(report.Summary.EditionID, "$.work.edition.edition_id", c)
	validateLocalized(obj(work["original_title"]), "$.work.original_title", c)
	if len(stringSet(work["content_languages"], "$.work.content_languages", true, c)) == 0 {
		c.add("canon_pack_schema_invalid", "$.work.content_languages", "at least one content language is required")
	}
	production := obj(m["production"])
	requireKeys(production, "$.production", []string{"method", "created_at", "producer"}, c)
	if !oneOf(str(production["method"]), "human_curated", "machine_assisted", "imported") {
		c.add("canon_pack_schema_invalid", "$.production.method", "invalid production method")
	}
	validateTime(str(production["created_at"]), "$.production.created_at", c)
	validateActor(obj(production["producer"]), "$.production.producer", c)
	review := obj(m["review"])
	requireKeys(review, "$.review", []string{"status"}, c)
	report.Summary.ReviewStatus = str(review["status"])
	if !oneOf(report.Summary.ReviewStatus, "not_reviewed", "in_review", "approved", "changes_requested", "rejected") {
		c.add("canon_pack_schema_invalid", "$.review.status", "invalid review status")
	}
	if report.Summary.ReviewStatus == "approved" {
		requireKeys(review, "$.review", []string{"procedure_id", "reviewed_at", "admission_basis"}, c)
		validateIdentifier(str(review["procedure_id"]), "$.review.procedure_id", c)
		validateTime(str(review["reviewed_at"]), "$.review.reviewed_at", c)
		if !oneOf(str(review["admission_basis"]), "human_review", "trusted_pack_review", "evidence_validated_batch") {
			c.add("canon_pack_review_invalid", "$.review.admission_basis", "approved pack has no admissible review basis")
		}
		if oneOf(str(review["admission_basis"]), "human_review", "trusted_pack_review") && len(arr(review["reviewers"])) == 0 {
			c.add("canon_pack_review_invalid", "$.review.reviewers", "identified reviewer is required")
		}
	}
	if str(pack["status"]) == "published" && report.Summary.ReviewStatus != "approved" {
		c.add("canon_pack_review_invalid", "$.review.status", "published packs must be approved")
	}
	trust := obj(m["trust"])
	requireKeys(trust, "$.trust", []string{"status"}, c)
	report.Summary.TrustStatus = str(trust["status"])
	if !oneOf(report.Summary.TrustStatus, "unsigned_local", "trusted_publisher", "invalid_signature") {
		c.add("canon_pack_schema_invalid", "$.trust.status", "invalid trust status")
	}
	if oneOf(report.Summary.TrustStatus, "trusted_publisher", "invalid_signature") {
		requireKeys(trust, "$.trust", []string{"publisher", "signature_file", "trust_policy_id"}, c)
		validateActor(obj(trust["publisher"]), "$.trust.publisher", c)
		validateIdentifier(str(trust["trust_policy_id"]), "$.trust.trust_policy_id", c)
		if str(trust["signature_file"]) != "signatures/manifest.sig" {
			c.add("canon_pack_trust_invalid", "$.trust.signature_file", "signed trust states require signatures/manifest.sig")
		}
		if report.Summary.TrustStatus == "invalid_signature" {
			c.add("canon_pack_signature_invalid", "$.trust.status", "a pack with an invalid signature cannot pass installation preview")
		} else {
			c.add("canon_pack_trust_verification_unavailable", "$.trust.status", "publisher signature verification is not implemented for this preview profile")
		}
	}
	sources := arr(m["sources"])
	report.Summary.SourceCount = len(sources)
	if len(sources) == 0 {
		c.add("canon_pack_schema_invalid", "$.sources", "at least one source is required")
	}
	sourceHashes := map[string]string{}
	for i, raw := range sources {
		p := fmt.Sprintf("$.sources[%d]", i)
		s := obj(raw)
		requireKeys(s, p, []string{"id", "title", "source_type", "uri", "license", "access_class", "retrieved_at", "document_sha256"}, c)
		id := str(s["id"])
		validateLocalID(id, p+".id", c)
		if _, exists := sourceHashes[id]; exists {
			c.add("canon_pack_duplicate_id", p+".id", "source id is duplicated")
		}
		sourceHashes[id] = str(s["document_sha256"])
		validateSHA(sourceHashes[id], p+".document_sha256", c)
		validateSourceURI(str(s["uri"]), p+".uri", c)
		validateTime(str(s["retrieved_at"]), p+".retrieved_at", c)
		if !oneOf(str(s["source_type"]), "official_primary", "licensed_dataset", "publisher_reference", "scholarly_secondary", "reputable_secondary", "public_wiki", "user_provided_local") {
			c.add("canon_pack_schema_invalid", p+".source_type", "invalid source type")
		}
		if !oneOf(str(s["access_class"]), "public", "open_license", "authorized_distribution", "local_private") {
			c.add("canon_pack_schema_invalid", p+".access_class", "invalid source access class")
		}
		license := obj(s["license"])
		requireKeys(license, p+".license", []string{"name", "redistribution_allowed"}, c)
		if strings.TrimSpace(str(license["name"])) == "" {
			c.add("canon_pack_schema_invalid", p+".license.name", "license name is required")
		}
		if _, ok := license["redistribution_allowed"].(bool); !ok {
			c.add("canon_pack_schema_invalid", p+".license.redistribution_allowed", "redistribution flag must be boolean")
		}
		if uri := str(license["uri"]); uri != "" {
			validateSourceURI(uri, p+".license.uri", c)
		}
	}
	content := obj(m["content"])
	requireKeys(content, "$.content", []string{"entities", "locations", "factions", "settings", "events", "relations", "claims"}, c)
	itemIDs := map[string]bool{}
	for _, kind := range []string{"entities", "locations", "factions", "settings", "events", "relations", "claims"} {
		records := arr(content[kind])
		report.Summary.RecordCounts[kind] = len(records)
		for i, raw := range records {
			p := fmt.Sprintf("$.content.%s[%d]", kind, i)
			rec := obj(raw)
			validateRecord(kind, rec, p, continuities, sourceHashes, itemIDs, c)
		}
	}
	validateReferences(content, itemIDs, c)
	for _, group := range []string{"conflicts", "uncertainties"} {
		list := arr(m[group])
		if group == "conflicts" {
			report.Summary.ConflictCount = len(list)
		} else {
			report.Summary.UncertaintyCount = len(list)
		}
		for i, raw := range list {
			p := fmt.Sprintf("$.%s[%d]", group, i)
			issue := obj(raw)
			requireKeys(issue, p, []string{"id", "item_ids", "reason", "status", "evidence"}, c)
			validateLocalID(str(issue["id"]), p+".id", c)
			for _, id := range stringSet(issue["item_ids"], p+".item_ids", true, c) {
				if !itemIDs[id] {
					c.add("canon_pack_item_reference_missing", p+".item_ids", "issue references an unknown content item")
				}
			}
			validateEvidence(arr(issue["evidence"]), p+".evidence", sourceHashes, c)
		}
	}
	coverage := obj(m["coverage_report"])
	requireKeys(coverage, "$.coverage_report", []string{"generated_at", "saturation", "domains"}, c)
	validateTime(str(coverage["generated_at"]), "$.coverage_report.generated_at", c)
	if !oneOf(str(coverage["saturation"]), "not_assessed", "growing", "saturated", "insufficient_sources") {
		c.add("canon_pack_schema_invalid", "$.coverage_report.saturation", "invalid saturation")
	}
	if len(arr(coverage["domains"])) == 0 {
		c.add("canon_pack_schema_invalid", "$.coverage_report.domains", "at least one coverage domain is required")
	}
	validateCompatibility(obj(m["compatibility"]), acVersion, c)
	validateFiles(arr(m["files"]), members, report, oneOf(report.Summary.TrustStatus, "trusted_publisher", "invalid_signature"), c)
}

func validateRecord(kind string, rec map[string]any, p string, continuity []string, sources map[string]string, ids map[string]bool, c *collector) {
	base := []string{"id", "continuity_ids", "review_state", "evidence"}
	if kind == "relations" {
		base = append(base, "subject_id", "predicate", "object_id")
	} else if kind == "claims" {
		base = append(base, "statement", "claim_type", "subject_ids")
	} else {
		base = append(base, "name")
	}
	if kind == "entities" {
		base = append(base, "kind")
	}
	requireKeys(rec, p, base, c)
	id := str(rec["id"])
	validateLocalID(id, p+".id", c)
	if ids[id] {
		c.add("canon_pack_duplicate_id", p+".id", "content item id is duplicated")
	}
	ids[id] = true
	allowedContinuity := set(continuity)
	for _, id := range stringSet(rec["continuity_ids"], p+".continuity_ids", true, c) {
		if !allowedContinuity[id] {
			c.add("canon_pack_continuity_mismatch", p+".continuity_ids", "record references an undeclared work continuity")
		}
	}
	if !oneOf(str(rec["review_state"]), "approved", "machine_recommended", "conflict", "uncertain", "rejected") {
		c.add("canon_pack_schema_invalid", p+".review_state", "invalid review state")
	}
	validateEvidence(arr(rec["evidence"]), p+".evidence", sources, c)
}

func validateEvidence(list []any, p string, sources map[string]string, c *collector) {
	if len(list) == 0 {
		c.add("canon_pack_schema_invalid", p, "at least one evidence locator is required")
	}
	for i, raw := range list {
		ep := fmt.Sprintf("%s[%d]", p, i)
		e := obj(raw)
		requireKeys(e, ep, []string{"source_id", "document_sha256", "locator"}, c)
		sid, h := str(e["source_id"]), str(e["document_sha256"])
		expected, ok := sources[sid]
		if !ok {
			c.add("canon_pack_evidence_source_missing", ep+".source_id", "evidence references an unknown source")
		} else if h != expected {
			c.add("canon_pack_evidence_hash_mismatch", ep+".document_sha256", "evidence hash does not match its declared source")
		}
		validateSHA(h, ep+".document_sha256", c)
		locator := obj(e["locator"])
		requireKeys(locator, ep+".locator", []string{"type", "value"}, c)
		if !oneOf(str(locator["type"]), "page", "chapter", "section", "paragraph", "fragment", "timestamp", "record") {
			c.add("canon_pack_schema_invalid", ep+".locator.type", "invalid locator type")
		}
	}
}

func validateReferences(content map[string]any, ids map[string]bool, c *collector) {
	for _, kind := range []string{"events", "relations", "claims"} {
		for i, raw := range arr(content[kind]) {
			rec := obj(raw)
			var fields []string
			switch kind {
			case "events":
				fields = []string{"participant_ids", "location_ids"}
			case "relations":
				fields = []string{"subject_id", "object_id"}
			case "claims":
				fields = []string{"subject_ids"}
			}
			for _, field := range fields {
				values := []string{str(rec[field])}
				if a, ok := rec[field].([]any); ok {
					values = nil
					for _, v := range a {
						values = append(values, str(v))
					}
				}
				for _, id := range values {
					if id != "" && !ids[id] {
						c.add("canon_pack_item_reference_missing", fmt.Sprintf("$.content.%s[%d].%s", kind, i, field), "record references an unknown content item")
					}
				}
			}
		}
	}
}

func validateFiles(files []any, members []*zip.File, report *Report, signatureRequired bool, c *collector) {
	declared := map[string]map[string]any{}
	for i, raw := range files {
		p := fmt.Sprintf("$.files[%d]", i)
		f := obj(raw)
		requireKeys(f, p, []string{"path", "role", "media_type", "size_bytes", "sha256"}, c)
		name := str(f["path"])
		if !oneOf(name, "notices/LICENSE.txt", "notices/NOTICE.txt", "signatures/manifest.sig") {
			c.add("canon_pack_file_not_allowed", p+".path", "ancillary path is not permitted")
		}
		expectedRole, expectedMediaType := "", ""
		switch name {
		case "notices/LICENSE.txt":
			expectedRole, expectedMediaType = "license_notice", "text/plain; charset=utf-8"
		case "notices/NOTICE.txt":
			expectedRole, expectedMediaType = "attribution_notice", "text/plain; charset=utf-8"
		case "signatures/manifest.sig":
			expectedRole, expectedMediaType = "manifest_signature", "application/octet-stream"
		}
		if expectedRole != "" && (str(f["role"]) != expectedRole || str(f["media_type"]) != expectedMediaType) {
			c.add("canon_pack_file_metadata_invalid", p, "ancillary role or media type does not match its path")
		}
		if size, ok := integer(f["size_bytes"]); !ok || size < 0 || size > 1<<20 {
			c.add("canon_pack_schema_invalid", p+".size_bytes", "file size must be an integer from 0 through 1048576")
		}
		if _, ok := declared[name]; ok {
			c.add("canon_pack_duplicate_file_declaration", p+".path", "file is declared more than once")
		}
		declared[name] = f
		validateSHA(str(f["sha256"]), p+".sha256", c)
	}
	report.Summary.AncillaryFiles = len(files)
	if signatureRequired {
		if _, ok := declared["signatures/manifest.sig"]; !ok {
			c.add("canon_pack_trust_invalid", "$.files", "signed trust states require a declared manifest signature file")
		}
	}
	actual := map[string]*zip.File{}
	for _, f := range members {
		name, ok := safeMemberName(f.Name)
		if ok && name != ManifestName {
			actual[name] = f
		}
	}
	for name, f := range actual {
		d, ok := declared[name]
		if !ok {
			c.add("canon_pack_unlisted_member", name, "archive member is not listed in manifest files")
			continue
		}
		b, err := readMember(f, maxExpandedBytes)
		if err != nil {
			c.add("canon_pack_member_read_failed", name, "archive member could not be read")
			continue
		}
		n, ok := integer(d["size_bytes"])
		if !ok || n != int64(len(b)) {
			c.add("canon_pack_file_size_mismatch", name, "archive member size does not match manifest")
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != str(d["sha256"]) {
			c.add("canon_pack_file_checksum_mismatch", name, "archive member checksum does not match manifest")
		}
	}
	for name := range declared {
		if actual[name] == nil {
			c.add("canon_pack_listed_file_missing", name, "manifest-listed archive member is missing")
		}
	}
}

func validateCompatibility(v map[string]any, current string, c *collector) {
	requireKeys(v, "$.compatibility", []string{"archive_center", "schema"}, c)
	schema := obj(v["schema"])
	if str(schema["name"]) != "canon-pack-manifest" || str(schema["version"]) != "1.0.0" {
		c.add("canon_pack_schema_unsupported", "$.compatibility.schema", "only canon-pack-manifest 1.0.0 is supported")
	}
	ac := obj(v["archive_center"])
	requireKeys(ac, "$.compatibility.archive_center", []string{"minimum_version"}, c)
	min, max := str(ac["minimum_version"]), str(ac["maximum_version_exclusive"])
	validateSemver(min, "$.compatibility.archive_center.minimum_version", c)
	if max != "" {
		validateSemver(max, "$.compatibility.archive_center.maximum_version_exclusive", c)
	}
	if semverRE.MatchString(min) && max != "" && semverRE.MatchString(max) && compareSemver(max, min) <= 0 {
		c.add("canon_pack_compatibility_invalid", "$.compatibility.archive_center", "maximum version must be greater than minimum version")
	}
	if !semverRE.MatchString(current) {
		c.add("canon_pack_archive_center_version_invalid", "$.compatibility.archive_center", "running Archive Center version is not valid SemVer")
		return
	}
	if compareSemver(current, min) < 0 || (max != "" && compareSemver(current, max) >= 0) {
		c.add("canon_pack_archive_center_incompatible", "$.compatibility.archive_center", "pack does not support this Archive Center version")
	}
}

func requireKeys(m map[string]any, p string, required []string, c *collector) {
	if m == nil {
		c.add("canon_pack_schema_invalid", p, "object is required")
		return
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			c.add("canon_pack_schema_invalid", p+"."+k, "required property is missing")
		}
	}
}
func validateIdentifier(v, p string, c *collector) {
	if len(v) < 3 || len(v) > 160 || !identifierRE.MatchString(v) {
		c.add("canon_pack_schema_invalid", p, "invalid namespaced identifier")
	}
}
func validateLocalID(v, p string, c *collector) {
	if len(v) < 1 || len(v) > 120 || !localIDRE.MatchString(v) {
		c.add("canon_pack_schema_invalid", p, "invalid local identifier")
	}
}
func validateSHA(v, p string, c *collector) {
	if !shaRE.MatchString(v) {
		c.add("canon_pack_schema_invalid", p, "invalid lowercase SHA-256")
	}
}
func validateSemver(v, p string, c *collector) {
	if !semverRE.MatchString(v) {
		c.add("canon_pack_schema_invalid", p, "invalid semantic version")
	}
}
func validateTime(v, p string, c *collector) {
	if _, err := time.Parse(time.RFC3339, v); err != nil {
		c.add("canon_pack_schema_invalid", p, "invalid RFC3339 timestamp")
	}
}
func validateLocalized(v map[string]any, p string, c *collector) {
	requireKeys(v, p, []string{"text", "language"}, c)
	if strings.TrimSpace(str(v["text"])) == "" {
		c.add("canon_pack_schema_invalid", p+".text", "localized text is required")
	}
}
func validateActor(v map[string]any, p string, c *collector) {
	requireKeys(v, p, []string{"id", "display_name"}, c)
	validateIdentifier(str(v["id"]), p+".id", c)
}
func validateSourceURI(v, p string, c *collector) {
	u, err := url.Parse(v)
	if err != nil || !(u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "urn") || u.User != nil {
		c.add("canon_pack_source_uri_invalid", p, "source URI must be HTTP(S) or URN without userinfo")
		return
	}
	secretNames := map[string]bool{
		"access_token": true, "api_key": true, "apikey": true, "auth": true,
		"authorization": true, "client_secret": true, "cookie": true, "key": true,
		"password": true, "passwd": true, "secret": true, "session": true,
		"session_id": true, "sig": true, "signature": true, "token": true,
	}
	for name := range u.Query() {
		normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "-", "_"))
		if secretNames[normalized] || strings.HasSuffix(normalized, "_token") || strings.HasSuffix(normalized, "_secret") {
			c.add("canon_pack_source_uri_secret", p, "source URI contains a forbidden secret query parameter")
			break
		}
	}
}
func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func arr(v any) []any          { a, _ := v.([]any); return a }
func str(v any) string         { s, _ := v.(string); return s }
func integer(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, e := n.Int64()
	return i, e == nil
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func set(v []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range v {
		m[x] = true
	}
	return m
}
func stringSet(v any, p string, required bool, c *collector) []string {
	a := arr(v)
	if required && len(a) == 0 {
		c.add("canon_pack_schema_invalid", p, "non-empty array is required")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range a {
		s, ok := raw.(string)
		if !ok || s == "" {
			c.add("canon_pack_schema_invalid", p, "array entries must be non-empty strings")
			continue
		}
		if seen[s] {
			c.add("canon_pack_schema_invalid", p, "array entries must be unique")
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
func compareSemver(a, b string) int {
	pa, apre := semverParts(a)
	pb, bpre := semverParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
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

func semverParts(v string) ([3]int, string) {
	var out [3]int
	core := strings.SplitN(v, "+", 2)[0]
	pre := ""
	if parts := strings.SplitN(core, "-", 2); len(parts) == 2 {
		core, pre = parts[0], parts[1]
	}
	fmt.Sscanf(core, "%d.%d.%d", &out[0], &out[1], &out[2])
	return out, pre
}

func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(left) || i < len(right); i++ {
		if i >= len(left) {
			return -1
		}
		if i >= len(right) {
			return 1
		}
		ln, lok := numericIdentifier(left[i])
		rn, rok := numericIdentifier(right[i])
		switch {
		case lok && rok && ln != rn:
			if ln < rn {
				return -1
			}
			return 1
		case lok != rok:
			if lok {
				return -1
			}
			return 1
		case left[i] < right[i]:
			return -1
		case left[i] > right[i]:
			return 1
		}
	}
	return 0
}

func numericIdentifier(v string) (int64, bool) {
	if v == "" || (len(v) > 1 && v[0] == '0') {
		return 0, false
	}
	var n int64
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int64(r-'0')
	}
	return n, true
}
