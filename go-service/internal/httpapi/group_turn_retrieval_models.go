package httpapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func buildUnifiedRetrievalDocuments(
	sid string,
	memories []store.Memory,
	evidence []store.DirectEvidence,
	kgTriples []store.KGTriple,
	episodeSums []store.EpisodeSummary,
	resumePack *store.ResumePack,
	chatLogs []store.ChatLog,
) []map[string]any {
	docs := []map[string]any{}
	for _, m := range memories {
		text := strings.Join(strings.Fields(q1FirstNonEmptyString(m.SummaryJSON, m.PlaceRoom, m.PlaceWing)), " ")
		docs = append(docs, retrievalDocumentQ1("memory", "memory", "memory_summary", m.ID, fmt.Sprintf("%d", m.ID), sid, m.TurnIndex, m.TurnIndex, m.TurnIndex, fmt.Sprintf("Memory #%d", m.ID), text, m.CreatedAt, map[string]any{
			"importance": m.Importance,
			"place_wing": m.PlaceWing,
			"place_room": m.PlaceRoom,
		}))
	}
	for _, e := range evidence {
		text := strings.Join(strings.Fields(e.EvidenceText), " ")
		docs = append(docs, retrievalDocumentQ1("evidence", "direct_evidence", "evidence_verbatim", e.ID, fmt.Sprintf("%d", e.ID), sid, e.TurnAnchor, e.TurnAnchor, e.TurnAnchor, fmt.Sprintf("Evidence #%d", e.ID), text, e.CreatedAt, map[string]any{
			"evidence_kind":        e.EvidenceKind,
			"capture_verification": e.CaptureVerification,
			"source_turn_start":    e.SourceTurnStart,
			"source_turn_end":      e.SourceTurnEnd,
		}))
	}
	for _, cl := range chatLogs {
		content := strings.Join(strings.Fields(cl.Content), " ")
		docs = append(docs, retrievalDocumentQ1("chat_log", "chat_log_fallback", "legacy_keyword_fallback", cl.ID, fmt.Sprintf("%d", cl.ID), sid, cl.TurnIndex, cl.TurnIndex, cl.TurnIndex, fmt.Sprintf("ChatLog #%d", cl.ID), content, cl.CreatedAt, map[string]any{
			"role":       cl.Role,
			"turn_index": cl.TurnIndex,
		}))
	}
	for _, k := range kgTriples {
		text := fmt.Sprintf("%s %s %s", k.Subject, k.Predicate, k.Object)
		docs = append(docs, retrievalDocumentQ1("kg_triple", "kg_triple", "kg_triple", k.ID, fmt.Sprintf("%d", k.ID), sid, 0, 0, 0, fmt.Sprintf("KG #%d", k.ID), text, k.CreatedAt, map[string]any{
			"subject":   k.Subject,
			"predicate": k.Predicate,
			"object":    k.Object,
		}))
	}
	for _, es := range episodeSums {
		summary := strings.Join(strings.Fields(q1FirstNonEmptyString(es.SummaryText, fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn))), " ")
		if anchors := episodeDenseAnchorPreview(es, 420); anchors != "" {
			summary = strings.Join(strings.Fields(summary+" "+anchors), " ")
		}
		meta := map[string]any{
			"from_turn":                 es.FromTurn,
			"to_turn":                   es.ToTurn,
			"key_events":                es.KeyEvents,
			"open_loops_json":           es.OpenLoopsJSON,
			"relationship_changes_json": es.RelationshipChangesJSON,
		}
		for k, v := range denseSummarySurfaceFields("episode", es.ID, es.FromTurn, es.ToTurn, es.SummaryText, episodeDenseStructuredPayload(es), episodeDensePriorityScores(es), evidence) {
			meta[k] = v
		}
		docs = append(docs, retrievalDocumentQ1("episode", "episode", "episode_summary", es.ID, fmt.Sprintf("%d", es.ID), sid, es.FromTurn, es.ToTurn, es.FromTurn, fmt.Sprintf("Episode #%d", es.ID), summary, es.CreatedAt, meta))
	}
	if resumePack != nil && resumePack.Chapter != nil {
		ch := resumePack.Chapter
		text := strings.Join(strings.Fields(q1FirstNonEmptyString(ch.SummaryText, ch.ResumeText, ch.ChapterTitle)), " ")
		meta := map[string]any{
			"chapter_index": ch.ChapterIndex,
			"chapter_title": ch.ChapterTitle,
		}
		for k, v := range denseSummarySurfaceFields("chapter", ch.ID, ch.FromTurn, ch.ToTurn, q1FirstNonEmptyString(ch.ResumeText, ch.SummaryText, ch.ChapterTitle), chapterDenseStructuredPayload(*ch), chapterDensePriorityScores(*ch), evidence) {
			meta[k] = v
		}
		docs = append(docs, retrievalDocumentQ1("chapter", "chapter", "chapter_summary", ch.ID, fmt.Sprintf("%d", ch.ID), sid, ch.FromTurn, ch.ToTurn, ch.FromTurn, fmt.Sprintf("Chapter #%d", ch.ID), text, *ch.CreatedAt, meta))
	}
	if resumePack != nil && resumePack.Arc != nil {
		arc := resumePack.Arc
		text := strings.Join(strings.Fields(q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName)), " ")
		meta := map[string]any{
			"arc_index":  arc.ArcIndex,
			"arc_name":   arc.ArcName,
			"arc_status": arc.ArcStatus,
		}
		for k, v := range denseSummarySurfaceFields("arc", arc.ID, arc.FromTurn, arc.ToTurn, q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName), arcDenseStructuredPayload(*arc), nil, evidence) {
			meta[k] = v
		}
		docs = append(docs, retrievalDocumentQ1("arc", "arc", "arc_summary", arc.ID, fmt.Sprintf("%d", arc.ID), sid, arc.FromTurn, arc.ToTurn, arc.FromTurn, fmt.Sprintf("Arc #%d", arc.ID), text, *arc.CreatedAt, meta))
	}
	if resumePack != nil && resumePack.Saga != nil {
		saga := resumePack.Saga
		text := strings.Join(strings.Fields(q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel)), " ")
		meta := map[string]any{
			"era_label": saga.EraLabel,
		}
		for k, v := range denseSummarySurfaceFields("saga", saga.ID, saga.FromTurn, saga.ToTurn, q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel), sagaDenseStructuredPayload(*saga), nil, evidence) {
			meta[k] = v
		}
		docs = append(docs, retrievalDocumentQ1("saga", "saga", "saga_summary", saga.ID, fmt.Sprintf("%d", saga.ID), sid, saga.FromTurn, saga.ToTurn, saga.FromTurn, fmt.Sprintf("Saga #%d", saga.ID), text, *saga.CreatedAt, meta))
	}
	return docs
}

func retrievalDocumentQ1(sourceType, sourceSubtype, sourceTable string, id int64, sourceRowID, sid string, fromTurn, toTurn, turnIndex int, title, text string, createdAt time.Time, meta map[string]any) map[string]any {
	doc := map[string]any{
		"document_id":     fmt.Sprintf("%s:%d", sourceType, id),
		"tier":            sourceType,
		"source_type":     sourceType,
		"source_subtype":  sourceSubtype,
		"source_row_id":   sourceRowID,
		"source_table":    sourceTable,
		"chat_session_id": sid,
		"from_turn":       fromTurn,
		"to_turn":         toTurn,
		"turn_index":      turnIndex,
		"title":           title,
		"text":            text,
		"similarity":      1.0,
		"created_at":      createdAt,
		"query_matched":   true,
		"metadata":        meta,
	}
	return doc
}

func retrievalDocumentSchemaQ1() map[string]any {
	return map[string]any{
		"version":       "q1a.v1",
		"index_version": "q1e.v1",
		"required_fields": []string{
			"document_id", "tier", "source_type", "source_subtype", "source_row_id",
			"source_table", "chat_session_id", "from_turn", "to_turn", "turn_index",
			"title", "text", "similarity", "created_at", "query_matched", "metadata",
		},
		"partition_keys": []string{"chat_session_id", "tier", "source_table"},
		"source_lookup":  "document_id_prefix_to_store_row",
	}
}

func retrievalIndexSnapshotFromDocuments(sid string, documents []map[string]any) map[string]any {
	return map[string]any{
		"status":          "ready",
		"document_count":  len(documents),
		"chat_session_id": sid,
		"schema_version":  "q1e.v1",
	}
}

func buildANNCandidateSnapshotQ2(docs []map[string]any, vectorShadow map[string]any) map[string]any {
	candidates := []map[string]any{}
	for i, doc := range docs {
		if i >= 8 {
			break
		}
		candidate := map[string]any{}
		for k, v := range doc {
			candidate[k] = v
		}
		score := 1.0 - float64(i)*0.05
		if score < 0.1 {
			score = 0.1
		}
		candidate["ann_rank"] = i + 1
		candidate["rerank_score"] = score
		candidate["similarity"] = score
		candidate["query_matched"] = true
		candidates = append(candidates, candidate)
	}
	vectorMode, _ := vectorShadow["mode"].(string)
	if vectorMode == "" {
		vectorMode, _ = vectorShadow["source"].(string)
	}
	status := "empty"
	if len(candidates) > 0 {
		status = "ready"
	}
	return map[string]any{
		"version":         "q2a.v1",
		"status":          status,
		"candidate_count": len(candidates),
		"candidates":      candidates,
		"rerank_applied":  len(candidates) > 1,
		"rerank_policy":   "metadata_score_v1",
		"merge_policy":    "tier_head_then_rerank_v1",
		"vector_mode":     vectorMode,
		"benchmark": map[string]any{
			"status":           status,
			"overlap_ratio":    q2OverlapRatio(candidates),
			"tier_diversity":   q2TierDiversity(candidates),
			"candidate_tiers":  q2TierSequence(candidates),
			"guarded_takeover": false,
			"takeover_guard":   "shadow_compare_first",
		},
	}
}

func q2OverlapRatio(candidates []map[string]any) float64 {
	if len(candidates) == 0 {
		return 0
	}
	seenText := map[string]bool{}
	for _, c := range candidates {
		text, _ := c["text"].(string)
		seenText[text] = true
	}
	return float64(len(seenText)) / float64(len(candidates))
}

func q2TierDiversity(candidates []map[string]any) int {
	tiers := map[string]bool{}
	for _, c := range candidates {
		tier, _ := c["tier"].(string)
		tiers[tier] = true
	}
	return len(tiers)
}

func q2TierSequence(candidates []map[string]any) []string {
	seq := []string{}
	for _, c := range candidates {
		tier, _ := c["tier"].(string)
		seq = append(seq, tier)
	}
	return seq
}

func buildIntentContractQ3() map[string]any {
	intents := []map[string]any{
		q3Intent("scene", []string{"memory", "episode", "chapter"}, 0.34),
		q3Intent("callback", []string{"arc", "saga", "memory"}, 0.22),
		q3Intent("resume", []string{"chapter", "arc", "saga"}, 0.28),
		q3Intent("canon", []string{"memory", "episode", "arc"}, 0.16),
	}
	tierCounts := map[string]int{}
	for _, intent := range intents {
		tiers, _ := intent["tiers"].([]string)
		for _, tier := range tiers {
			tierCounts[tier]++
		}
	}
	return map[string]any{
		"version":      "q3a.v1",
		"routing_mode": "single_query_shared",
		"intents":      intents,
		"routing_shadow_tier_priority": map[string]any{
			"version":                "t1d.v1",
			"mode":                   "verification_only",
			"status":                 "shadow_only",
			"tier_counts":            tierCounts,
			"priority_verdict":       "tier_priority_verification_shadow",
			"requires_manual_review": false,
			"reason":                 "routing_shadow_tier_priority_surface",
		},
	}
}

func q3Intent(name string, tiers []string, budgetShare float64) map[string]any {
	return map[string]any{
		"name":          name,
		"query_builder": name + "_query_v1",
		"tiers":         tiers,
		"budget_share":  budgetShare,
	}
}

func q3PacketBudgetPolicy() map[string]any {
	return map[string]any{
		"version":        "q3c.v1",
		"profile_source": "runtime_token_profile",
		"budget_mode":    "policy_only",
		"scene_share":    0.34,
		"callback_share": 0.22,
		"resume_share":   0.28,
		"canon_share":    0.16,
		"degrade_policy": "drop_low_score_then_shorten_text",
		"budget_transition": map[string]any{
			"version":          "p75a.v1",
			"from_mode":        "policy_only",
			"to_mode":          "enforced_shadow",
			"transition_ready": true,
			"reason":           "per_intent_shadow_budget_gate",
		},
		"budget_caps": map[string]any{
			"version":          "p76a.v1",
			"layer_cap":        12,
			"char_cap":         3000,
			"canon_hard_floor": 120,
			"per_intent_max":   3,
			"reason":           "layer_char_canon_hard_floor_applied",
		},
	}
}

func buildIntentHitPreviewQ3(queryPreview string, docs []map[string]any) map[string]any {
	matched := map[string][]string{
		"scene":    {},
		"callback": {},
		"resume":   {},
		"canon":    {},
	}
	for _, doc := range docs {
		tier, _ := doc["tier"].(string)
		documentID, _ := doc["document_id"].(string)
		if tier == "" || documentID == "" {
			continue
		}
		for _, intent := range q3MatchedTiers(tier) {
			matched[intent] = append(matched[intent], documentID)
		}
	}
	status := "empty"
	if len(docs) > 0 {
		status = "ready"
	}
	return map[string]any{
		"version":         "q3d.v1",
		"status":          status,
		"query_preview":   truncateRunes(strings.Join(strings.Fields(queryPreview), " "), 120),
		"matched_intents": matched,
	}
}

func q3MatchedTiers(tier string) []string {
	switch tier {
	case "memory":
		return []string{"scene", "callback", "canon"}
	case "episode":
		return []string{"scene", "canon"}
	case "chapter":
		return []string{"scene", "resume"}
	case "arc":
		return []string{"callback", "resume", "canon"}
	case "saga":
		return []string{"callback", "resume"}
	case "chat_log":
		return []string{"scene"}
	default:
		return []string{}
	}
}

func buildGenerationPacketShadowCompareRecord(assembly prepareTurnInjectionAssembly, inputContextText string) map[string]any {
	newHasChapter := strings.TrimSpace(assembly.MemoryText) != "" && strings.Contains(strings.ToLower(assembly.MemoryText), "chapter")
	newChapterChars := len([]rune(strings.TrimSpace(assembly.MemoryText)))
	newHasChapterInput := strings.TrimSpace(inputContextText) != "" && strings.Contains(strings.ToLower(inputContextText), "chapter")
	oldHasChapter := false
	oldChapterChars := 0
	oldHasChapterInput := false
	return map[string]any{
		"version":                  "p249a.v1",
		"new_has_chapter":          newHasChapter,
		"new_chapter_chars":        newChapterChars,
		"new_has_chapter_input":    newHasChapterInput,
		"old_has_chapter":          oldHasChapter,
		"old_chapter_chars":        oldChapterChars,
		"old_has_chapter_input":    oldHasChapterInput,
		"divergence_chapter":       newHasChapter != oldHasChapter,
		"divergence_chapter_input": newHasChapterInput != oldHasChapterInput,
	}
}

func canonicalLayerEligibleForCurrentTruth(layer store.CanonicalStateLayer) bool {
	sourceType := strings.ToLower(strings.TrimSpace(layer.SourceStateType))
	for _, blocked := range []string{"pending", "rejected", "unverified", "stale", "repair_queue", "manual_review"} {
		if strings.Contains(sourceType, blocked) {
			return false
		}
	}
	if layer.Confidence > 0 && layer.Confidence < 0.7 {
		return false
	}
	return true
}
