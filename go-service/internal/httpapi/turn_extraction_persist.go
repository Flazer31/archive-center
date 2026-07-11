package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func saveCanonicalStateLayerWithCost(ctx context.Context, clSaver canonicalStateLayerSaver, sid string, layer *store.CanonicalStateLayer, existing []store.CanonicalStateLayer, cost *canonicalStateWriteCostMeasurement) error {
	var prev *store.CanonicalStateLayer
	for i := range existing {
		e := &existing[i]
		if e.LayerType != layer.LayerType {
			continue
		}
		if prev == nil || e.TurnIndex > prev.TurnIndex {
			prev = e
		}
	}
	prevChars := 0
	similarity := 0.0
	if prev != nil {
		prevChars = len([]rune(prev.Content))
		similarity = simpleTokenSimilarity(prev.Content, layer.Content)
	}
	newChars := len([]rune(layer.Content))
	charDelta := newChars - prevChars
	if charDelta < 0 {
		charDelta = -charDelta
	}

	var mode string
	if prev == nil {
		mode = "full_rewrite_bootstrap"
		cost.FullRewriteCount++
	} else if similarity >= 0.55 {
		mode = "delta_update"
		cost.DeltaUpdateCount++
	} else {
		mode = "full_rewrite"
		cost.FullRewriteCount++
	}

	start := time.Now()
	err := clSaver.SaveCanonicalStateLayer(ctx, layer)
	latencyMs := time.Since(start).Milliseconds()

	cost.StateWriteCount++
	cost.TotalWriteChars += newChars
	cost.TotalElapsedMs += latencyMs
	cost.Items = append(cost.Items, map[string]any{
		"layer_type":             layer.LayerType,
		"write_mode":             mode,
		"write_latency_ms":       latencyMs,
		"previous_content_chars": prevChars,
		"new_content_chars":      newChars,
		"char_delta_abs":         charDelta,
		"token_similarity":       similarity,
	})
	return err
}

func finalizeCanonicalStateWriteCost(cost *canonicalStateWriteCostMeasurement) {
	if cost == nil || cost.StateWriteCount == 0 {
		return
	}
	cost.PolicyVersion = "lc1b.v1"
	var sum int64
	latencies := make([]int64, 0, len(cost.Items))
	for _, item := range cost.Items {
		l, _ := item["write_latency_ms"].(int64)
		sum += l
		latencies = append(latencies, l)
	}
	cost.AvgWriteLatencyMs = float64(sum) / float64(cost.StateWriteCount)
	if len(latencies) > 0 {
		// simple p95: sort and pick 95th percentile index
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}
		idx := int(float64(len(latencies)-1) * 0.95)
		if idx < 0 {
			idx = 0
		}
		cost.P95WriteLatencyMs = float64(latencies[idx])
	}
}

func (s *Server) saveCriticExtractionArtifacts(ctx context.Context, sid string, turnIndex int, extraction map[string]any, content string, embCfg completeTurnEmbeddingConfig, now time.Time, existingEvidenceArg ...[]store.DirectEvidence) artifactSaveResult {
	result := artifactSaveResult{EmbeddingStatus: "not_requested", VectorStatus: "not_requested"}
	cost := &canonicalStateWriteCostMeasurement{}
	var existingCanonicalLayers []store.CanonicalStateLayer
	if s.Store != nil {
		existingCanonicalLayers, _ = s.Store.ListCanonicalStateLayers(ctx, sid, "")
	}
	existingEvidence := []store.DirectEvidence{}
	if len(existingEvidenceArg) > 0 {
		existingEvidence = existingEvidenceArg[0]
	} else if s.Store != nil {
		existingEvidence, _ = s.Store.ListEvidence(ctx, sid)
	}
	existingKGTriples := []store.KGTriple{}
	if s.Store != nil {
		existingKGTriples, _ = s.Store.ListKGTriples(ctx, sid)
	}
	rawTurnSummary := extraction["turn_summary"]
	summary := normalizeCriticTurnSummary(rawTurnSummary)
	if summary == "" && (isStructuredCriticTurnSummaryValue(rawTurnSummary) || looksLikeStructuredCriticPayloadText(extractionStringFromAny(rawTurnSummary))) {
		if fallback := strings.Join(strings.Fields(content), " "); fallback != "" {
			summary = fallback
			extraction["turn_summary"] = fallback
			result.Warnings = append(result.Warnings, "turn_summary_rebuilt_from_grounded_turn_text")
		}
	} else {
		extraction["turn_summary"] = summary
	}
	languageContext := completeTurnLanguageContextFromExtraction(extraction)
	extraction = applyLanguageMemoryWriteContract(extraction, languageContext)
	if mergedExtraction, applied := applyConfirmedIdentityAliasCanonicalMerge(extraction); applied > 0 {
		extraction = mergedExtraction
		result.Warnings = append(result.Warnings, "confirmed_identity_alias_canonical_merge_applied")
	}
	extraction = appendNarrativeStateEvidenceExcerpts(extraction)
	memorySearchText := completeTurnMemorySearchText(summary, extraction, content)
	searchText := strings.TrimSpace(memorySearchText.Text)
	if searchText == "" {
		searchText = summary
	}
	embedding := "[]"
	embeddingModel := "not_configured"
	var embeddingVector []float32
	if embCfg.hasConfig() && searchText != "" {
		embeddingStartedAt := time.Now()
		emb, model, err := callEmbedding(ctx, embCfg, searchText)
		result.addTiming("embedding", embeddingStartedAt)
		if err != nil {
			result.EmbeddingStatus = "error: " + err.Error()
			result.Warnings = append(result.Warnings, "embedding_call_failed")
		} else {
			embedding = emb
			embeddingModel = model
			result.EmbeddingStatus = "ok"
			embeddingVector = parseFloat32JSONList(emb)
		}
	} else if summary != "" {
		result.EmbeddingStatus = "missing_config"
	}

	recordPersonaCapsuleCandidateTrace(extraction, turnIndex, &result)
	s.saveSubjectiveEntityMemoriesFromExtraction(ctx, sid, turnIndex, extraction, content, now, &result)

	if summary != "" {
		archiveHint := mapFromAny(extraction["archive_hint"])
		emotionalIntensity := clampFloat(extractionFloatFromAny(extraction["emotional_intensity"], 0), 0, 1)
		narrativeSignificance := clampFloat(extractionFloatFromAny(extraction["narrative_significance"], 0), 0, 1)
		baseImportance := clampFloat(extractionFloatFromAny(extraction["importance_score"], 3), 1, 10)
		emotionalBoost := emotionalImportanceBoost(emotionalIntensity)
		finalImportance := clampFloat(baseImportance+emotionalBoost, 1, 10)
		mem := &store.Memory{
			ChatSessionID:         sid,
			TurnIndex:             turnIndex,
			SummaryJSON:           mustCompactJSON(extraction),
			Embedding:             embedding,
			EmbeddingModel:        embeddingModel,
			Importance:            finalImportance / 10.0,
			EmotionalBoost:        emotionalBoost,
			Evidence:              mustCompactJSON(map[string]any{"evidence_excerpts": stringsFromAny(extraction["evidence_excerpts"]), "relationship_memory": extraction["relationship_memory"]}),
			EmotionalIntensity:    emotionalIntensity,
			NarrativeSignificance: narrativeSignificance,
			PlaceWing:             stringFromMap(archiveHint, "wing"),
			PlaceRoom:             stringFromMap(archiveHint, "room"),
			CreatedAt:             now,
		}
		if existingID, existingSummary := s.memoryForTurnAlreadyExists(ctx, sid, turnIndex, &result); existingID > 0 {
			result.addSkipReason("memories", "duplicate_source_turn_memory", map[string]any{
				"turn_index":       turnIndex,
				"existing_id":      existingID,
				"existing_summary": existingSummary,
				"new_summary":      summary,
			})
			result.Warnings = append(result.Warnings, "memory_duplicate_source_turn_skipped")
		} else if s.mergeSimilarMemoryInsteadOfInsert(ctx, sid, summary, mem.Importance, now, &result) {
			result.Warnings = append(result.Warnings, "memory_semantic_dedup_merged")
		} else {
			result.trySave("SaveMemory", func() error {
				return s.Store.SaveMemory(ctx, mem)
			}, &result, func() {
				result.Memories++
				s.upsertMemoryVector(ctx, sid, turnIndex, mem, searchText, embeddingVector, &result)
			})
		}
	}

	for excerptIndex, text := range stringsFromAny(extraction["evidence_excerpts"]) {
		originalText := text
		text = sanitizeEvidenceExcerptForTurn(text, content)
		if text == "" {
			result.addSkipReason("direct_evidence", "not_grounded_in_current_turn", originalText)
			continue
		}
		if directEvidenceAlreadyExistsForTurn(existingEvidence, sid, turnIndex, text) {
			result.addSkipReason("direct_evidence", "duplicate_source_turn_excerpt", map[string]any{"turn_index": turnIndex, "text": text})
			continue
		}
		ev := &store.DirectEvidence{
			ChatSessionID:        sid,
			EvidenceKind:         "turn_excerpt",
			EvidenceText:         text,
			SourceTurnStart:      turnIndex,
			SourceTurnEnd:        turnIndex,
			TurnAnchor:           turnIndex,
			ArchiveState:         "verified_direct",
			CaptureStage:         "critic_extract",
			CaptureVerification:  "verified",
			CommittedGate:        "auto_grounded_excerpt",
			LineageJSON:          mustCompactJSON(completeTurnEvidenceLineage("critic.evidence_excerpts", excerptIndex, languageContext)),
			SourceMessageIDsJSON: mustCompactJSON([]string{fmt.Sprintf("turn:%d", turnIndex)}),
			CreatedAt:            now,
		}
		baseImportance := clampFloat(extractionFloatFromAny(extraction["importance_score"], 3), 1, 10) / 10.0
		result.ConflictResolutions = append(result.ConflictResolutions, resolveCanonicalConflict(*ev, existingEvidence)...)
		result.RetentionDecisions = append(result.RetentionDecisions, applyRetentionPolicy(ev, baseImportance, existingEvidence))
		result.trySave("SaveEvidence", func() error {
			return s.Store.SaveEvidence(ctx, ev)
		}, &result, func() {
			result.Evidence++
			existingEvidence = append(existingEvidence, *ev)
			s.upsertDerivedArtifactVector(ctx, sid, turnIndex, "evidence", "direct_evidence_records", ev.ID, "direct_evidence.v1", directEvidenceVectorDocumentText(*ev), embCfg, &result)
		})
	}

	// Current narrative state is resolved only after direct evidence has been
	// persisted, so every accepted change can point back to concrete evidence.
	s.saveNarrativeStateFromExtraction(ctx, sid, turnIndex, extraction, content, existingEvidence, now, &result)

	for _, item := range sliceFromAny(extraction["kg_triples"]) {
		triple := mapFromAny(item)
		subject := s.canonicalCharacterName(ctx, sid, sanitizeKGPart(stringFromMap(triple, "subject")))
		predicate := sanitizeKGPredicate(stringFromMap(triple, "predicate"))
		object := s.canonicalCharacterName(ctx, sid, sanitizeKGPart(stringFromMap(triple, "object")))
		if shouldSkipKGTriple(subject, predicate, object, sid) {
			result.addSkipReason("kg_triples", "placeholder_or_control_edge", map[string]any{"subject": subject, "predicate": predicate, "object": object})
			continue
		}
		validFrom := intFromAny(triple["valid_from"], turnIndex)
		validTo := intFromAny(triple["valid_to"], 0)
		if kgTripleAlreadyExistsForTurn(existingKGTriples, sid, turnIndex, subject, predicate, object, validFrom, validTo) {
			result.addSkipReason("kg_triples", "duplicate_source_turn_triple", map[string]any{
				"turn_index": turnIndex,
				"subject":    subject,
				"predicate":  predicate,
				"object":     object,
			})
			continue
		}
		result.trySave("SaveKGTriple", func() error {
			return s.Store.SaveKGTriple(ctx, &store.KGTriple{
				ChatSessionID: sid,
				Subject:       subject,
				Predicate:     predicate,
				Object:        object,
				ValidFrom:     validFrom,
				ValidTo:       validTo,
				SourceTurn:    turnIndex,
				CreatedAt:     now,
			})
		}, &result, func() {
			result.KGTriples++
			existingKGTriples = append(existingKGTriples, store.KGTriple{
				ChatSessionID: sid,
				Subject:       subject,
				Predicate:     predicate,
				Object:        object,
				ValidFrom:     validFrom,
				ValidTo:       validTo,
				SourceTurn:    turnIndex,
			})
		})
	}

	s.saveCharacterAndStateArtifacts(ctx, sid, turnIndex, extraction, embCfg, now, &result, existingCanonicalLayers, cost)
	finalizeCanonicalStateWriteCost(cost)
	if cost.StateWriteCount > 0 {
		result.CanonicalStateWriteCost = cost
	}
	s.applyCriticSoftPrune(ctx, sid, turnIndex, extraction, now, &result)
	return result
}

func (s *Server) memoryForTurnAlreadyExists(ctx context.Context, sid string, turnIndex int, result *artifactSaveResult) (int64, string) {
	if s == nil || s.Store == nil || turnIndex < 0 {
		return 0, ""
	}
	memories, err := s.Store.ListMemories(ctx, sid, turnIndex, turnIndex)
	if err != nil {
		if result != nil {
			result.Warnings = append(result.Warnings, "memory_duplicate_turn_check_failed")
		}
		return 0, ""
	}
	for _, mem := range memories {
		if mem.ChatSessionID != sid || mem.TurnIndex != turnIndex || mem.ID <= 0 {
			continue
		}
		summary := memorySummaryText(mem)
		if strings.TrimSpace(summary) == "" {
			continue
		}
		return mem.ID, summary
	}
	return 0, ""
}

func (s *Server) mergeSimilarMemoryInsteadOfInsert(ctx context.Context, sid, summary string, newImportance float64, now time.Time, result *artifactSaveResult) bool {
	if s.Store == nil || strings.TrimSpace(summary) == "" {
		return false
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		result.Warnings = append(result.Warnings, "memory_semantic_dedup_list_failed")
		return false
	}
	var best *store.Memory
	bestScore := 0.0
	for i := range memories {
		mem := &memories[i]
		if mem.ID <= 0 {
			continue
		}
		existingSummary := memorySummaryText(*mem)
		if existingSummary == "" {
			continue
		}
		score := simpleTokenSimilarity(summary, existingSummary)
		if score > bestScore {
			bestScore = score
			best = mem
		}
	}
	if best == nil || bestScore < 0.78 {
		return false
	}
	if updater, ok := s.Store.(memoryImportanceUpdater); ok && newImportance > best.Importance {
		targetImportance := newImportance
		result.trySave("UpdateMemoryImportance(memory_dedup)", func() error {
			return updater.UpdateMemoryImportance(ctx, sid, best.ID, targetImportance)
		}, result, func() {})
	}
	details := map[string]any{
		"policy_version":      "p1250.memory_semantic_dedup.v1",
		"merged_memory_id":    best.ID,
		"similarity":          bestScore,
		"new_turn_summary":    summary,
		"existing_summary":    memorySummaryText(*best),
		"new_importance":      newImportance,
		"existing_importance": best.Importance,
	}
	result.trySave("SaveAuditLog(memory_semantic_dedup)", func() error {
		return s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "memory_semantic_dedup",
			Source:        "critic",
			DetailsJSON:   mustCompactJSON(details),
			CreatedAt:     now,
		})
	}, result, func() {})
	return true
}

func memorySummaryText(mem store.Memory) string {
	raw := strings.TrimSpace(mem.SummaryJSON)
	if raw == "" {
		return ""
	}
	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		for _, key := range []string{"turn_summary", "summary", "memory", "text"} {
			if value := strings.TrimSpace(extractionStringFromAny(parsed[key])); value != "" {
				return value
			}
		}
	}
	return raw
}

func directEvidenceAlreadyExistsForTurn(existing []store.DirectEvidence, sid string, turnIndex int, text string) bool {
	needle := normalizeArtifactDedupeText(text)
	if needle == "" {
		return false
	}
	for _, item := range existing {
		if item.ChatSessionID != sid {
			continue
		}
		start := item.SourceTurnStart
		end := item.SourceTurnEnd
		if start <= 0 {
			start = item.TurnAnchor
		}
		if end <= 0 {
			end = start
		}
		if turnIndex > 0 && start > 0 && end > 0 && (turnIndex < start || turnIndex > end) {
			continue
		}
		existingText := normalizeArtifactDedupeText(item.EvidenceText)
		if existingText == "" {
			continue
		}
		if existingText == needle ||
			strings.Contains(existingText, needle) ||
			strings.Contains(needle, existingText) ||
			simpleTokenSimilarity(existingText, needle) >= 0.86 {
			return true
		}
	}
	return false
}

func normalizeArtifactDedupeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	text = strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t', '\u00a0':
			return ' '
		case '\u201c', '\u201d', '\u2033':
			return '"'
		case '\u2018', '\u2019', '\u2032':
			return '\''
		default:
			return r
		}
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	return strings.Trim(text, " \t\r\n.,;:!?\"'`()[]{}")
}

func normalizeArtifactComparableText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"“", `"`,
		"”", `"`,
		"‘", `'`,
		"’", `'`,
	)
	text = replacer.Replace(text)
	text = strings.Join(strings.Fields(text), " ")
	return strings.Trim(text, " \t\r\n.,;:!?\"'`“”‘’()[]{}")
}

func kgTripleAlreadyExistsForTurn(existing []store.KGTriple, sid string, turnIndex int, subject, predicate, object string, validFrom, validTo int) bool {
	subject = strings.TrimSpace(strings.ToLower(subject))
	predicate = strings.TrimSpace(strings.ToLower(predicate))
	object = strings.TrimSpace(strings.ToLower(object))
	for _, item := range existing {
		if item.ChatSessionID != sid {
			continue
		}
		if strings.TrimSpace(strings.ToLower(item.Subject)) != subject ||
			strings.TrimSpace(strings.ToLower(item.Predicate)) != predicate ||
			strings.TrimSpace(strings.ToLower(item.Object)) != object {
			continue
		}
		if item.SourceTurn == turnIndex {
			return true
		}
		if item.ValidTo == 0 && validTo == 0 {
			return true
		}
		if validFrom > 0 && item.ValidFrom > 0 && item.ValidFrom != validFrom {
			continue
		}
		if validTo > 0 && item.ValidTo > 0 && item.ValidTo != validTo {
			continue
		}
		return true
	}
	return false
}

func (s *Server) applyCriticSoftPrune(ctx context.Context, sid string, turnIndex int, extraction map[string]any, now time.Time, result *artifactSaveResult) {
	targets := stringsFromAny(extraction["prune_targets"])
	if len(targets) == 0 || s.Store == nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(s.Cfg.PrunePolicy), "off") {
		result.Warnings = append(result.Warnings, "soft_prune_disabled")
		return
	}
	updater, ok := s.Store.(memoryImportanceUpdater)
	if !ok {
		result.Warnings = append(result.Warnings, "soft_prune_update_not_supported")
		return
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		result.Warnings = append(result.Warnings, "soft_prune_list_failed")
		return
	}
	pruned := []map[string]any{}
	for _, target := range targets {
		keyword := strings.ToLower(strings.TrimSpace(target))
		if keyword == "" {
			continue
		}
		for _, mem := range memories {
			if mem.ID <= 0 || mem.Importance <= 0.1 {
				continue
			}
			if !strings.Contains(strings.ToLower(mem.SummaryJSON), keyword) {
				continue
			}
			oldImportance := mem.Importance
			newImportance := oldImportance - 0.2
			if newImportance < 0.1 {
				newImportance = 0.1
			}
			result.trySave("UpdateMemoryImportance", func() error {
				return updater.UpdateMemoryImportance(ctx, sid, mem.ID, newImportance)
			}, result, func() {
				pruned = append(pruned, map[string]any{
					"id":      mem.ID,
					"old":     oldImportance,
					"new":     newImportance,
					"keyword": keyword,
				})
			})
		}
	}
	if len(pruned) == 0 {
		return
	}
	result.trySave("SaveAuditLog(soft_prune)", func() error {
		return s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "soft_prune",
			TargetType:    "turn",
			TargetID:      int64(turnIndex),
			Summary:       fmt.Sprintf("Soft prune: %d memories, turn %d", len(pruned), turnIndex),
			DetailsJSON:   mustCompactJSON(map[string]any{"pruned": pruned, "targets": targets}),
			Source:        "critic",
			CreatedAt:     now,
		})
	}, result, func() {})
	for _, item := range pruned {
		memoryID, _ := item["id"].(int64)
		if memoryID <= 0 {
			continue
		}
		keyword, _ := item["keyword"].(string)
		s.saveSupersessionResolutionBestEffort(ctx, store.SupersessionResolutionDecision{
			ChatSessionID:   sid,
			TargetType:      "memory",
			TargetID:        memoryID,
			SourceTurn:      turnIndex,
			ResolutionClass: "stale_demote",
			RelationshipKey: keyword,
			Reason:          "critic_prune_target",
			EvidenceJSON:    mustCompactJSON(item),
			Operator:        "critic",
		}, now, result)
	}
}

func (s *Server) saveSupersessionResolutionBestEffort(ctx context.Context, decision store.SupersessionResolutionDecision, now time.Time, result *artifactSaveResult) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	if resolver, ok := s.Store.(store.SupersessionResolutionStore); ok {
		result.trySave("SaveSupersessionResolution", func() error {
			_, err := resolver.SaveSupersessionResolution(ctx, &decision)
			return err
		}, result, func() {})
		return
	}
	details := map[string]any{
		"contract_version": store.SupersessionResolutionContractVersion,
		"resolution_class": decision.ResolutionClass,
		"source_turn":      decision.SourceTurn,
		"target":           map[string]any{"type": decision.TargetType, "id": decision.TargetID},
		"afterglow_turns":  store.SupersessionResolutionAfterglowTurns,
		"hard_delete":      false,
	}
	if strings.TrimSpace(decision.NewTargetType) != "" || decision.NewTargetID > 0 {
		details["new_target"] = map[string]any{"type": strings.TrimSpace(decision.NewTargetType), "id": decision.NewTargetID}
	}
	if strings.TrimSpace(decision.RelationshipKey) != "" {
		details["relationship_key"] = strings.TrimSpace(decision.RelationshipKey)
	}
	if strings.TrimSpace(decision.Reason) != "" {
		details["reason"] = strings.TrimSpace(decision.Reason)
	}
	if strings.TrimSpace(decision.EvidenceJSON) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(decision.EvidenceJSON), &parsed); err == nil {
			details["evidence"] = parsed
		} else {
			details["evidence_text"] = strings.TrimSpace(decision.EvidenceJSON)
		}
	}
	source := strings.TrimSpace(decision.Operator)
	if source == "" {
		source = "critic"
	}
	summary := fmt.Sprintf("Resolution %s: %s #%d", decision.ResolutionClass, decision.TargetType, decision.TargetID)
	result.trySave("SaveAuditLog(supersession_resolution)", func() error {
		return s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: decision.ChatSessionID,
			EventType:     "supersession_resolution",
			TargetType:    decision.TargetType,
			TargetID:      decision.TargetID,
			Summary:       summary,
			DetailsJSON:   mustCompactJSON(details),
			Source:        source,
			CreatedAt:     now,
		})
	}, result, func() {})
}
