package canonpack

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestDocumentedNeutralManifestPassesPreview(t *testing.T) {
	manifest, err := os.ReadFile("../../../docs/canon-pack-manifest-v1.example.json")
	if err != nil {
		t.Fatal(err)
	}

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create(ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(manifest); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	report := PreviewZIP(archive.Bytes(), "3.1.0")
	if !report.Valid {
		t.Fatalf("documented neutral manifest failed preview: %#v", report.Diagnostics)
	}
	if report.ValidationProfile != ValidationProfile || report.Summary.PackID == "" || report.Summary.StableWorkID == "" {
		t.Fatalf("preview summary/profile missing: %#v", report)
	}
}

func TestInspectZIPBindsExactArchiveAndManifestBytes(t *testing.T) {
	manifest, err := os.ReadFile("../../../docs/canon-pack-manifest-v1.example.json")
	if err != nil {
		t.Fatal(err)
	}

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create(ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(manifest); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	inspection, err := InspectZIP(archive.Bytes(), "3.1.0")
	if err != nil {
		t.Fatal(err)
	}
	archiveHash := sha256.Sum256(archive.Bytes())
	manifestHash := sha256.Sum256(manifest)
	if inspection.ArchiveSHA256 != hex.EncodeToString(archiveHash[:]) {
		t.Fatalf("archive hash=%q", inspection.ArchiveSHA256)
	}
	if inspection.ManifestSHA256 != hex.EncodeToString(manifestHash[:]) {
		t.Fatalf("manifest hash=%q", inspection.ManifestSHA256)
	}
	if !bytes.Equal(inspection.ManifestJSON, manifest) {
		t.Fatal("inspection did not preserve the exact manifest bytes")
	}
}
