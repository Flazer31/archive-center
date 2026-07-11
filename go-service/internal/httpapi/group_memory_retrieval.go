package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func (s *Server) handleRetrievalIndexRuntimeConfigGet(w http.ResponseWriter, r *http.Request) {
	// Python 0.8 parity: session_count reflects retrieval-index registry state,
	// not total store sessions. R1 Go does not yet maintain a retrieval-index
	// registry, so session_count remains 0 to match empty-registry fixture-live.
	sessionCount := 0
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":                 "shadow",
		"shadow_write_enabled": true,
		"updated_at":           time.Now().UTC().Format(time.RFC3339Nano),
		"reason":               "default",
		"session_count":        sessionCount,
		"index_version":        "q1e.v1",
	})
}
func (s *Server) handleIntentRoutingRuntimeConfigGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":            "single_query_shared",
		"updated_at":      time.Now().UTC().Format("2006-01-02 15:04:05"),
		"reason":          "default",
		"version":         "v0c.v1",
		"supported_modes": []string{"single_query_shared", "per_intent_shadow"},
	})
}

func (s *Server) handleRetrievalIndexSnapshot(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	documentCount := 0
	status := "empty"
	documents := []map[string]any{}
	documentSchema := map[string]any{
		"version":         "q1a.v1",
		"index_version":   "q1e.v1",
		"document_id":     "tier:id",
		"required_fields": []string{"document_id", "tier", "source_table", "source_row_id", "source_type"},
		"tiers":           []string{"memory", "episode", "chapter", "arc", "saga"},
	}
	addDocument := func(tier string, id int64, sourceTable, sourceType string) {
		documents = append(documents, map[string]any{
			"document_id":   fmt.Sprintf("%s:%d", tier, id),
			"tier":          tier,
			"source_table":  sourceTable,
			"source_row_id": id,
			"source_type":   sourceType,
		})
	}
	sourceTypeCounts := map[string]any{}
	tierCounts := map[string]any{
		"memory":  0,
		"episode": 0,
		"chapter": 0,
		"arc":     0,
		"saga":    0,
	}

	if s.Store != nil {
		memories, err := s.Store.ListMemories(r.Context(), sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			memCount := len(memories)
			documentCount += memCount
			tierCounts["memory"] = memCount
			if memCount > 0 {
				sourceTypeCounts["memories"] = memCount
			}
			for _, m := range memories {
				addDocument("memory", m.ID, "memories", "memory")
			}
		}

		evidence, err := s.Store.ListEvidence(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			evCount := len(evidence)
			documentCount += evCount
			if evCount > 0 {
				sourceTypeCounts["direct_evidence"] = evCount
			}
			for _, e := range evidence {
				addDocument("memory", e.ID, "direct_evidence_records", "direct_evidence")
			}
		}

		kgTriples, err := s.Store.ListKGTriples(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			kgCount := len(kgTriples)
			documentCount += kgCount
			if kgCount > 0 {
				sourceTypeCounts["kg_triples"] = kgCount
			}
			for _, t := range kgTriples {
				addDocument("memory", t.ID, "kg_triples", "kg_triple")
			}
		}

		episodes, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 0, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			epCount := len(episodes)
			documentCount += epCount
			tierCounts["episode"] = epCount
			if epCount > 0 {
				sourceTypeCounts["episode_summaries"] = epCount
			}
			for _, ep := range episodes {
				addDocument("episode", ep.ID, "episode_summaries", "episode_summary")
			}
		}

		if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
			chapters, err := chapterStore.SearchChapterSummaries(r.Context(), sid, "", 0, 0, 0)
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				chapterCount := len(chapters)
				documentCount += chapterCount
				tierCounts["chapter"] = chapterCount
				if chapterCount > 0 {
					sourceTypeCounts["chapter_summaries"] = chapterCount
				}
				for _, ch := range chapters {
					addDocument("chapter", ch.ID, "chapter_summaries", "chapter_summary")
				}
			}
		}

		if arcStore, ok := s.Store.(store.ArcSummaryStore); ok {
			arcs, err := arcStore.ListArcSummaries(r.Context(), sid, "", 0)
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				arcCount := len(arcs)
				documentCount += arcCount
				tierCounts["arc"] = arcCount
				if arcCount > 0 {
					sourceTypeCounts["arc_summaries"] = arcCount
				}
				for _, arc := range arcs {
					addDocument("arc", arc.ID, "arc_summaries", "arc_summary")
				}
			}
		}

		if sagaStore, ok := s.Store.(store.SagaDigestStore); ok {
			sagas, err := sagaStore.ListSagaDigests(r.Context(), sid, 0)
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				sagaCount := len(sagas)
				documentCount += sagaCount
				tierCounts["saga"] = sagaCount
				if sagaCount > 0 {
					sourceTypeCounts["saga_digests"] = sagaCount
				}
				for _, saga := range sagas {
					addDocument("saga", saga.ID, "saga_digests", "saga_digest")
				}
			}
		}

		chatLogs, err := s.Store.ListChatLogs(r.Context(), sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			clCount := len(chatLogs)
			if clCount > 0 {
				sourceTypeCounts["chat_logs"] = clCount
			}
			if clCount > 0 && documentCount == 0 {
				status = "ok"
			}
		}

		effectiveInputs := 0
		if len(chatLogs) > 0 && len(chatLogs) <= 200 {
			for _, l := range chatLogs {
				ei, err := s.Store.GetEffectiveInput(r.Context(), sid, l.TurnIndex)
				if err != nil {
					continue
				}
				if ei != nil {
					effectiveInputs++
				}
			}
		}
		if effectiveInputs > 0 {
			sourceTypeCounts["effective_inputs"] = effectiveInputs
		}

		if documentCount > 0 || len(chatLogs) > 0 {
			status = "ok"
		}
	}

	if s.Vector != nil {
		vecCount, err := s.Vector.Count(r.Context(), sid)
		if err == nil && vecCount > 0 {
			sourceTypeCounts["vectors"] = vecCount
		} else if !errors.Is(err, vector.ErrNotEnabled) {
			// Non-NotEnabled errors are informational only; do not fail.
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chat_session_id":           sid,
		"dirty":                     false,
		"dirty_reason":              nil,
		"dirty_turn":                nil,
		"discard_turn":              nil,
		"document_schema":           documentSchema,
		"documents":                 documents,
		"document_count":            documentCount,
		"index_version":             "q1e.v1",
		"last_dirty_at":             nil,
		"last_discarded_at":         nil,
		"last_event":                nil,
		"last_event_reason":         nil,
		"partition_count":           0,
		"runtime_mode":              "shadow",
		"runtime_reason":            "default",
		"runtime_updated_at":        generatedAt(),
		"session_partitioned":       true,
		"shadow_write_enabled":      true,
		"source_type_counts":        sourceTypeCounts,
		"status":                    status,
		"tier_counts":               tierCounts,
		"retrieval_document_schema": documentSchema,
		"updated_at":                nil,
	})
}

func (s *Server) handleRetrievalIndexSourceRow(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	docID := r.URL.Query().Get("document_id")
	if docID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status": "error",
			"detail": "chat_session_id and document_id are required",
		})
		return
	}

	// Backward-compatible resolution: document_id may be formatted as "type:id"
	parts := strings.SplitN(docID, ":", 2)
	rowType := ""
	var rowID int64
	if len(parts) == 2 {
		rowType = parts[0]
		rowID, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	document := map[string]any(nil)
	sourceRow := map[string]any(nil)
	lookupStatus := "document_not_found"

	if s.Store != nil && rowType != "" && rowID > 0 {
		switch rowType {
		case "memory":
			memories, err := s.Store.ListMemories(r.Context(), sid, 0, 0)
			if err == nil {
				for _, m := range memories {
					if m.ID == rowID {
						document = map[string]any{
							"document_id":   docID,
							"source_table":  "memories",
							"source_row_id": m.ID,
							"tier":          "memory",
							"source_type":   "memory",
						}
						sourceRow = map[string]any{
							"id":              m.ID,
							"chat_session_id": m.ChatSessionID,
							"turn_index":      m.TurnIndex,
							"summary_json":    m.SummaryJSON,
							"importance":      m.Importance,
							"type":            "memory",
						}
						lookupStatus = "ok"
						break
					}
				}
			}
		case "evidence":
			evidence, err := s.Store.ListEvidence(r.Context(), sid)
			if err == nil {
				for _, e := range evidence {
					if e.ID == rowID {
						document = map[string]any{
							"document_id":   docID,
							"source_table":  "direct_evidence_records",
							"source_row_id": e.ID,
							"tier":          "memory",
							"source_type":   "direct_evidence",
						}
						sourceRow = map[string]any{
							"id":              e.ID,
							"chat_session_id": e.ChatSessionID,
							"evidence_kind":   e.EvidenceKind,
							"evidence_text":   e.EvidenceText,
							"archive_state":   e.ArchiveState,
							"type":            "evidence",
						}
						lookupStatus = "ok"
						break
					}
				}
			}
		case "kg_triple":
			triples, err := s.Store.ListKGTriples(r.Context(), sid)
			if err == nil {
				for _, t := range triples {
					if t.ID == rowID {
						document = map[string]any{
							"document_id":   docID,
							"source_table":  "kg_triples",
							"source_row_id": t.ID,
							"tier":          "memory",
							"source_type":   "kg_triple",
						}
						sourceRow = map[string]any{
							"id":              t.ID,
							"chat_session_id": t.ChatSessionID,
							"subject":         t.Subject,
							"predicate":       t.Predicate,
							"object":          t.Object,
							"type":            "kg_triple",
						}
						lookupStatus = "ok"
						break
					}
				}
			}
		case "episode":
			episodes, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 0, 0, 0)
			if err == nil {
				for _, ep := range episodes {
					if ep.ID == rowID {
						document = map[string]any{
							"document_id":   docID,
							"source_table":  "episode_summaries",
							"source_row_id": ep.ID,
							"tier":          "episode",
							"source_type":   "episode_summary",
						}
						sourceRow = map[string]any{
							"id":              ep.ID,
							"chat_session_id": ep.ChatSessionID,
							"from_turn":       ep.FromTurn,
							"to_turn":         ep.ToTurn,
							"summary_text":    ep.SummaryText,
							"key_entities":    ep.KeyEntities,
							"key_events":      ep.KeyEvents,
							"type":            "episode",
						}
						lookupStatus = "ok"
						break
					}
				}
			}
		case "chapter":
			if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
				chapters, err := chapterStore.SearchChapterSummaries(r.Context(), sid, "", 0, 0, 0)
				if err == nil {
					for _, ch := range chapters {
						if ch.ID == rowID {
							document = map[string]any{
								"document_id":   docID,
								"source_table":  "chapter_summaries",
								"source_row_id": ch.ID,
								"tier":          "chapter",
								"source_type":   "chapter_summary",
							}
							sourceRow = map[string]any{
								"id":              ch.ID,
								"chat_session_id": ch.ChatSessionID,
								"from_turn":       ch.FromTurn,
								"to_turn":         ch.ToTurn,
								"chapter_index":   ch.ChapterIndex,
								"chapter_title":   ch.ChapterTitle,
								"summary_text":    ch.SummaryText,
								"resume_text":     ch.ResumeText,
								"type":            "chapter",
							}
							lookupStatus = "ok"
							break
						}
					}
				}
			}
		case "arc":
			if arcStore, ok := s.Store.(store.ArcSummaryStore); ok {
				arcs, err := arcStore.ListArcSummaries(r.Context(), sid, "", 0)
				if err == nil {
					for _, arc := range arcs {
						if arc.ID == rowID {
							document = map[string]any{
								"document_id":   docID,
								"source_table":  "arc_summaries",
								"source_row_id": arc.ID,
								"tier":          "arc",
								"source_type":   "arc_summary",
							}
							sourceRow = map[string]any{
								"id":              arc.ID,
								"chat_session_id": arc.ChatSessionID,
								"from_turn":       arc.FromTurn,
								"to_turn":         arc.ToTurn,
								"arc_index":       arc.ArcIndex,
								"arc_name":        arc.ArcName,
								"arc_status":      arc.ArcStatus,
								"arc_resume_text": arc.ArcResumeText,
								"type":            "arc",
							}
							lookupStatus = "ok"
							break
						}
					}
				}
			}
		case "saga":
			if sagaStore, ok := s.Store.(store.SagaDigestStore); ok {
				sagas, err := sagaStore.ListSagaDigests(r.Context(), sid, 0)
				if err == nil {
					for _, saga := range sagas {
						if saga.ID == rowID {
							document = map[string]any{
								"document_id":   docID,
								"source_table":  "saga_digests",
								"source_row_id": saga.ID,
								"tier":          "saga",
								"source_type":   "saga_digest",
							}
							sourceRow = map[string]any{
								"id":               saga.ID,
								"chat_session_id":  saga.ChatSessionID,
								"from_turn":        saga.FromTurn,
								"to_turn":          saga.ToTurn,
								"era_label":        saga.EraLabel,
								"saga_summary":     saga.SagaSummary,
								"resume_pack_text": saga.ResumePackText,
								"type":             "saga",
							}
							lookupStatus = "ok"
							break
						}
					}
				}
			}
		}
	}

	var sourceTable, sourceRowID, tier, sourceType any
	if document != nil {
		sourceTable = document["source_table"]
		sourceRowID = document["source_row_id"]
		tier = document["tier"]
		sourceType = document["source_type"]
	}

	payload := map[string]any{
		"status":          "ok",
		"lookup_status":   lookupStatus,
		"chat_session_id": sid,
		"document_id":     docID,
		"document":        document,
		"source_ref": map[string]any{
			"source_table":  sourceTable,
			"source_row_id": sourceRowID,
			"tier":          tier,
			"source_type":   sourceType,
		},
		"source_row": sourceRow,
	}

	if lookupStatus != "ok" {
		payload["status"] = "error"
		writeJSON(w, http.StatusNotFound, payload)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleKGRecallGet(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	items := []any{}
	total := 0

	if strings.TrimSpace(sid) == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":        "ok",
			"items":         items,
			"total":         total,
			"limit":         limit,
			"offset":        offset,
			"count":         0,
			"has_more":      false,
			"legacy_compat": true,
		})
		return
	}

	if s.Store != nil {
		triples, err := s.Store.ListKGTriples(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sort.SliceStable(triples, func(i, j int) bool {
				if triples[i].CreatedAt.Equal(triples[j].CreatedAt) {
					return triples[i].ID > triples[j].ID
				}
				return triples[i].CreatedAt.After(triples[j].CreatedAt)
			})
			total = len(triples)
			start := offset
			if start > len(triples) {
				start = len(triples)
			}
			end := start + limit
			if end > len(triples) {
				end = len(triples)
			}
			for _, t := range triples[start:end] {
				items = append(items, kgTripleExplorerItem(t))
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"items":         items,
		"count":         len(items),
		"total":         total,
		"limit":         limit,
		"offset":        offset,
		"has_more":      offset+len(items) < total,
		"legacy_compat": true,
	})
}

func (s *Server) handleKGRecall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChatSessionID string   `json:"chat_session_id"`
		Entities      []string `json:"entities"`
		Limit         int      `json:"limit"`
		CurrentTurn   int      `json:"current_turn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := body.ChatSessionID
	if sid == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"items":             []any{},
			"count":             0,
			"entities_received": 0,
			"entities_sent":     len(body.Entities),
		})
		return
	}

	safeEntities := nonEmptyStrings(body.Entities)
	if len(safeEntities) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"items":             []any{},
			"count":             0,
			"entities_received": 0,
		})
		return
	}

	items := []any{}
	expiredFiltered := 0

	if s.Store != nil {
		triples, err := s.Store.ListKGTriples(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sortKGTriplesForPython(triples)
			for _, t := range triples {
				if kgTripleExpiredAtTurn(t, body.CurrentTurn) {
					expiredFiltered++
					continue
				}
				if kgTripleMatchesEntities(t, safeEntities) {
					items = append(items, kgTripleExplorerItem(t))
				}
				if body.Limit > 0 && len(items) >= body.Limit {
					break
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"items":             items,
		"count":             len(items),
		"entities_received": len(safeEntities),
		"current_turn":      nullablePositiveInt(body.CurrentTurn),
		"expired_filtered":  expiredFiltered,
	})
}

func kgTripleExpiredAtTurn(t store.KGTriple, currentTurn int) bool {
	return currentTurn > 0 && t.ValidTo > 0 && t.ValidTo < currentTurn
}
