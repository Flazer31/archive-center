package vector

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ChromaRoundTripProbeContract = "archive-center.chroma-roundtrip-probe.v1"

// ChromaRoundTripProbeConfig defines an explicit, mutating ChromaDB live probe.
// The probe always uses a unique temporary collection and never runs as part of
// normal backend startup.
type ChromaRoundTripProbeConfig struct {
	Endpoint         string
	APIPath          string
	CollectionPrefix string
	RunID            string
	HTTPClient       *http.Client
	CleanupTimeout   time.Duration
}

type ChromaRoundTripProbeReport struct {
	Contract         string                      `json:"contract"`
	Status           string                      `json:"status"`
	ExplicitMutation bool                        `json:"explicit_mutation"`
	GeneratedAt      string                      `json:"generated_at"`
	EndpointHost     string                      `json:"endpoint_host,omitempty"`
	Collection       string                      `json:"collection"`
	DocumentID       string                      `json:"document_id"`
	SessionID        string                      `json:"session_id"`
	Stages           []ChromaRoundTripProbeStage `json:"stages"`
	CleanupStatus    string                      `json:"cleanup_status"`
}

type ChromaRoundTripProbeStage struct {
	Name      string `json:"name"`
	Operation string `json:"operation"`
	Status    string `json:"status"`
	Expected  string `json:"expected,omitempty"`
	Observed  string `json:"observed,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ChromaRoundTripProbeError struct {
	Stage     string
	Operation string
	Err       error
}

func (e *ChromaRoundTripProbeError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("ChromaDB roundtrip probe failed at %s (%s)", e.Stage, e.Operation)
}

func (e *ChromaRoundTripProbeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type chromaRoundTripProbeStore interface {
	VectorStore
	DocumentDeleter
	DocumentLister
	ExactMetadataQuerier
	CollectionResetter
}

// RunChromaRoundTripProbe performs an explicit temporary-collection roundtrip
// through the same ChromaDB store methods used by production. It upserts one
// document, verifies get and exact-query readback, counts it, deletes it, and
// finally removes the temporary collection.
func RunChromaRoundTripProbe(ctx context.Context, cfg ChromaRoundTripProbeConfig) (*ChromaRoundTripProbeReport, error) {
	runID := normalizeChromaProbeToken(cfg.RunID)
	if runID == "" {
		runID = newChromaProbeRunID()
	}
	prefix := normalizeChromaProbeToken(cfg.CollectionPrefix)
	if prefix == "" {
		prefix = "archive_center_probe"
	}
	collection := prefix + "_" + runID
	sessionID := "probe-session-" + runID
	documentID := "probe-document-" + runID
	documentText := "Archive Center explicit ChromaDB roundtrip probe " + runID
	embedding := []float32{0.125, 0.5, 0.875}

	report := &ChromaRoundTripProbeReport{
		Contract:         ChromaRoundTripProbeContract,
		Status:           "running",
		ExplicitMutation: true,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		EndpointHost:     chromaProbeEndpointHost(cfg.Endpoint),
		Collection:       collection,
		DocumentID:       documentID,
		SessionID:        sessionID,
		CleanupStatus:    "pending",
	}

	raw, err := NewChromaStoreWithHTTPClient(cfg.Endpoint, collection, cfg.APIPath, cfg.HTTPClient)
	if err != nil {
		report.Status = "failed"
		report.CleanupStatus = "not_created"
		report.Stages = append(report.Stages, failedChromaProbeStage("configure", "chroma.new_store", err, cfg.Endpoint))
		return report, &ChromaRoundTripProbeError{Stage: "configure", Operation: "chroma.new_store", Err: err}
	}
	store, ok := raw.(chromaRoundTripProbeStore)
	if !ok {
		err := fmt.Errorf("configured ChromaDB store does not implement the diagnostic roundtrip extensions")
		report.Status = "failed"
		report.CleanupStatus = "not_created"
		report.Stages = append(report.Stages, failedChromaProbeStage("configure", "chroma.assert_extensions", err, cfg.Endpoint))
		return report, &ChromaRoundTripProbeError{Stage: "configure", Operation: "chroma.assert_extensions", Err: err}
	}

	cleanupAttempted := false
	cleanup := func() error {
		if cleanupAttempted {
			return nil
		}
		cleanupAttempted = true
		cleanupCtx := ctx
		cancel := func() {}
		if cfg.CleanupTimeout > 0 {
			cleanupCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), cfg.CleanupTimeout)
		}
		defer cancel()
		if err := store.ResetAll(cleanupCtx); err != nil {
			report.CleanupStatus = "failed"
			report.Stages = append(report.Stages, failedChromaProbeStage("cleanup_collection", "chroma.reset_collection", err, cfg.Endpoint))
			return err
		}
		report.CleanupStatus = "ok"
		report.Stages = append(report.Stages, ChromaRoundTripProbeStage{
			Name:      "cleanup_collection",
			Operation: "chroma.reset_collection",
			Status:    "ok",
			Expected:  "temporary collection removed",
			Observed:  "temporary collection removed",
		})
		return nil
	}
	fail := func(stage, operation string, err error) (*ChromaRoundTripProbeReport, error) {
		report.Status = "failed"
		report.Stages = append(report.Stages, failedChromaProbeStage(stage, operation, err, cfg.Endpoint))
		_ = cleanup()
		return report, &ChromaRoundTripProbeError{Stage: stage, Operation: operation, Err: err}
	}

	doc := VectorDocument{
		ID:            documentID,
		Embedding:     embedding,
		Tier:          "memory",
		ChatSessionID: sessionID,
		SourceTable:   "runtime_dependency_probe",
		SourceRowID:   runID,
		SchemaVersion: ChromaRoundTripProbeContract,
		DocumentText:  documentText,
		Metadata: map[string]any{
			"probe_contract": ChromaRoundTripProbeContract,
			"probe_run_id":   runID,
		},
	}
	if err := store.Upsert(ctx, sessionID, []VectorDocument{doc}); err != nil {
		return fail("upsert", "chroma.upsert", err)
	}
	report.Stages = append(report.Stages, ChromaRoundTripProbeStage{
		Name: "upsert", Operation: "chroma.upsert", Status: "ok",
		Expected: "one temporary document", Observed: "one temporary document",
	})

	documents, err := store.ListDocuments(ctx, sessionID)
	if err != nil {
		return fail("get_readback", "chroma.get", err)
	}
	if len(documents) != 1 || documents[0].ID != documentID || documents[0].DocumentText != documentText {
		return fail("get_readback", "chroma.get", fmt.Errorf("exact get readback mismatch"))
	}
	report.Stages = append(report.Stages, ChromaRoundTripProbeStage{
		Name: "get_readback", Operation: "chroma.get", Status: "ok",
		Expected: documentID, Observed: documents[0].ID,
	})

	results, err := store.QueryExact(ctx, ExactQuery{
		Embedding: embedding,
		Limit:     1,
		Where: map[string]any{
			"$and": []map[string]any{
				{"chat_session_id": sessionID},
				{"probe_run_id": runID},
			},
		},
	})
	if err != nil {
		return fail("search_readback", "chroma.query_exact", err)
	}
	if len(results) != 1 || results[0].Document.ID != documentID || results[0].Document.DocumentText != documentText {
		return fail("search_readback", "chroma.query_exact", fmt.Errorf("exact search readback mismatch"))
	}
	report.Stages = append(report.Stages, ChromaRoundTripProbeStage{
		Name: "search_readback", Operation: "chroma.query_exact", Status: "ok",
		Expected: documentID, Observed: results[0].Document.ID,
	})

	count, err := store.Count(ctx, sessionID)
	if err != nil {
		return fail("count", "chroma.count", err)
	}
	if count != 1 {
		return fail("count", "chroma.count", fmt.Errorf("temporary document count=%d, want 1", count))
	}
	report.Stages = append(report.Stages, ChromaRoundTripProbeStage{
		Name: "count", Operation: "chroma.count", Status: "ok",
		Expected: "1", Observed: "1",
	})

	if err := store.DeleteDocuments(ctx, []string{documentID}); err != nil {
		return fail("delete_document", "chroma.delete_documents", err)
	}
	report.Stages = append(report.Stages, ChromaRoundTripProbeStage{
		Name: "delete_document", Operation: "chroma.delete_documents", Status: "ok",
		Expected: documentID, Observed: documentID,
	})

	count, err = store.Count(ctx, sessionID)
	if err != nil {
		return fail("count_after_delete", "chroma.count", err)
	}
	if count != 0 {
		return fail("count_after_delete", "chroma.count", fmt.Errorf("temporary document count after delete=%d, want 0", count))
	}
	report.Stages = append(report.Stages, ChromaRoundTripProbeStage{
		Name: "count_after_delete", Operation: "chroma.count", Status: "ok",
		Expected: "0", Observed: "0",
	})

	if err := cleanup(); err != nil {
		report.Status = "failed"
		return report, &ChromaRoundTripProbeError{Stage: "cleanup_collection", Operation: "chroma.reset_collection", Err: err}
	}
	report.Status = "ok"
	return report, nil
}

func failedChromaProbeStage(name, operation string, err error, endpoint string) ChromaRoundTripProbeStage {
	return ChromaRoundTripProbeStage{
		Name:      name,
		Operation: operation,
		Status:    "failed",
		Error:     sanitizeChromaProbeError(err, endpoint),
	}
}

func normalizeChromaProbeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			out.WriteRune(r)
		}
	}
	normalized := strings.Trim(out.String(), "_-")
	if len(normalized) > 48 {
		normalized = strings.Trim(normalized[:48], "_-")
	}
	return normalized
}

func newChromaProbeRunID() string {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err == nil {
		return fmt.Sprintf("%d_%s", time.Now().UTC().UnixMilli(), hex.EncodeToString(random))
	}
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}

func chromaProbeEndpointHost(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return ""
	}
	return parsed.Host
}

func sanitizeChromaProbeError(err error, endpoint string) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	parsed, parseErr := url.Parse(strings.TrimSpace(endpoint))
	if parseErr == nil && parsed.User != nil {
		if password, ok := parsed.User.Password(); ok && password != "" {
			message = strings.ReplaceAll(message, password, "***")
		}
		if userInfo := parsed.User.String(); userInfo != "" {
			message = strings.ReplaceAll(message, userInfo, "***")
		}
	}
	return message
}
