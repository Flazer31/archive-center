package httpapi

import (
	"net/http"
)

// registerMemoryRoutes mounts search, retrieval, explorer, and chroma-shadow.
func (s *Server) registerMemoryRoutes(mux *http.ServeMux) {
	// R1 read-only
	mux.HandleFunc("POST /search", s.handleSearch)
	mux.HandleFunc("GET /retrieval-index/runtime-config", s.handleRetrievalIndexRuntimeConfigGet)
	mux.HandleFunc("GET /intent-routing/runtime-config", s.handleIntentRoutingRuntimeConfigGet)
	mux.HandleFunc("GET /retrieval-index/{chat_session_id}", s.handleRetrievalIndexSnapshot)
	mux.HandleFunc("GET /retrieval-index/{chat_session_id}/source-row", s.handleRetrievalIndexSourceRow)
	mux.HandleFunc("GET /kg/recall", s.handleKGRecallGet)
	mux.HandleFunc("POST /kg/recall", s.handleKGRecall)

	// Chroma shadow: probes
	mux.HandleFunc("GET /chroma-shadow/preflight", s.handleChromaPreflight)

	// Chroma shadow: R1 read/audit
	mux.HandleFunc("POST /chroma-shadow/backfill-dry-run", s.handleChromaBackfillDryRun)
	mux.HandleFunc("POST /chroma-shadow/reembed-audit", s.handleChromaReembedAudit)
	mux.HandleFunc("POST /chroma-shadow/fallback-runbook", s.handleChromaFallbackRunbook)
	mux.HandleFunc("POST /chroma-shadow/release-hygiene", s.handleChromaReleaseHygiene)
	mux.HandleFunc("POST /chroma-shadow/visibility-guard", s.handleChromaVisibilityGuard)
	mux.HandleFunc("POST /chroma-shadow/health-probe", s.handleChromaHealthProbe)

	// Chroma shadow: R2 write
	mux.HandleFunc("POST /chroma-shadow/bootstrap", s.handleChromaBootstrap)
	mux.HandleFunc("POST /chroma-shadow/backfill-batch", s.handleChromaBackfillBatch)
	mux.HandleFunc("POST /chroma-shadow/rebuild-drill", s.handleChromaRebuildDrill)
	mux.HandleFunc("POST /chroma-shadow/adoption-gate", s.handleChromaAdoptionGate)

	// EM-1d: session-level reembed schedule (shadow/dry-run contract)
	mux.HandleFunc("POST /chroma-shadow/reembed-schedule", s.handleChromaReembedSchedule)

	// R2 write
	mux.HandleFunc("POST /retrieval-index/runtime-config", s.handleRetrievalIndexRuntimeConfigPost)
	mux.HandleFunc("POST /intent-routing/runtime-config", s.handleIntentRoutingRuntimeConfigPost)

	// Explorer: R1 read
	mux.HandleFunc("GET /explorer/chat_logs", s.handleExplorerChatLogs)
	mux.HandleFunc("GET /explorer/memories", s.handleExplorerMemories)
	mux.HandleFunc("GET /explorer/direct-evidence", s.handleExplorerDirectEvidence)
	mux.HandleFunc("GET /explorer/kg_triples", s.handleExplorerKGTriples)
	mux.HandleFunc("GET /explorer/chapter_summaries", s.handleExplorerChapterSummaries)
	mux.HandleFunc("GET /explorer/arc_summaries", s.handleExplorerArcSummaries)
	mux.HandleFunc("GET /explorer/saga_digests", s.handleExplorerSagaDigests)

	// Explorer: fake-id 404 parity
	mux.HandleFunc("GET /explorer/{sid}", s.handleExplorerGet404)

	// Explorer: R2 write
	mux.HandleFunc("PATCH /explorer/memories/{memory_id}", s.handlePatchMemory)
	mux.HandleFunc("PATCH /explorer/kg_triples/{triple_id}", s.handlePatchKGTriple)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}", s.handlePatchEvidenceEdit)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}/review", s.handlePatchEvidenceReview)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}/revalidate", s.handlePatchEvidenceRevalidate)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}/tombstone", s.handlePatchEvidenceTombstone)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}/supersede", s.handlePatchEvidenceSupersede)
	mux.HandleFunc("POST /explorer/memories/regenerate", s.handleRegenerateMemory)
	mux.HandleFunc("DELETE /explorer/memories/{memory_id}", s.handleDeleteMemory)
	mux.HandleFunc("POST /explorer/memories/{memory_id}/delete", s.handleDeleteMemoryPost)
	mux.HandleFunc("DELETE /explorer/direct-evidence/{record_id}", s.handleDeleteDirectEvidence)
	mux.HandleFunc("POST /explorer/direct-evidence/{record_id}/delete", s.handleDeleteDirectEvidencePost)
	mux.HandleFunc("DELETE /explorer/kg_triples/{triple_id}", s.handleDeleteKGTriple)
	mux.HandleFunc("POST /explorer/kg_triples/{triple_id}/delete", s.handleDeleteKGTriplePost)
}

// R1 read-only: Store/Vector-backed
