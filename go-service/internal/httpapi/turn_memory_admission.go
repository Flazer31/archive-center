package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const memoryAdmissionIndexVersion = store.MemoryVectorOutboxContract

// resolveCommittedMemoryAdmissionExtraction makes crash recovery deterministic.
// Once the common writer has admitted one extraction for this source/version,
// every later foreground or worker pass must finish secondary projections from
// that exact extraction instead of a newly sampled critic response.
func (s *Server) resolveCommittedMemoryAdmissionExtraction(
	ctx context.Context,
	sid string,
	extraction map[string]any,
	result *artifactSaveResult,
) (map[string]any, bool) {
	if s == nil || s.Store == nil || result == nil {
		return extraction, true
	}
	lifecycle, lifecycleOK := s.Store.(store.MemoryDerivationLifecycleAvailability)
	if !lifecycleOK || !lifecycle.MemoryDerivationLifecycleEnabled() {
		return extraction, true
	}
	sourceContext, accepted := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	if !accepted ||
		sourceContext.ContractVersion != completeTurnSourceAcceptanceContract ||
		strings.TrimSpace(sourceContext.Revision) == "" {
		return extraction, true
	}
	sourceStore, ok := s.Store.(store.SourceRevisionStore)
	if !ok {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "ResolveMemoryAdmission: source revision store is unavailable")
		return extraction, false
	}
	source, err := sourceStore.GetSourceRevision(ctx, sid, sourceContext.Revision)
	if err == store.ErrNotFound {
		return extraction, true
	}
	if err != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "ResolveMemoryAdmission: "+err.Error())
		return extraction, false
	}
	if source == nil ||
		source.DerivedAdmissionState != "committed" ||
		source.DerivedAdmissionVersion != store.MemoryAdmissionContract ||
		source.DerivedExtractorVersion != completeTurnCriticPipelineVersion ||
		source.DerivedIndexVersion != memoryAdmissionIndexVersion {
		return extraction, true
	}
	storedJSON := strings.TrimSpace(source.DerivedResultJSON)
	if storedJSON == "" {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "ResolveMemoryAdmission: committed result JSON is missing")
		return extraction, false
	}
	var committed map[string]any
	if err := json.Unmarshal([]byte(storedJSON), &committed); err != nil || committed == nil {
		result.Errors++
		if err == nil {
			err = fmt.Errorf("committed result is not an object")
		}
		result.ErrorDetails = append(result.ErrorDetails, "ResolveMemoryAdmission: "+err.Error())
		return extraction, false
	}
	if mustCompactJSON(normalizePreciseMemoryValue(extraction)) != storedJSON {
		result.Warnings = append(result.Warnings, "memory_admission_committed_result_reused")
	}
	return committed, true
}

// commitAcceptedMemoryAdmission cuts an accepted source revision over to the
// common MariaDB writer. The boolean result distinguishes a handled admission
// from legacy stores that do not advertise the derivation lifecycle.
func (s *Server) commitAcceptedMemoryAdmission(
	ctx context.Context,
	sid string,
	turnIndex int,
	extraction map[string]any,
	content string,
	summary string,
	searchText string,
	memorySearchText memorySearchTextBuild,
	embCfg completeTurnEmbeddingConfig,
	embedding string,
	embeddingModel string,
	embeddingVector []float32,
	languageContext map[string]any,
	existingEvidence []store.DirectEvidence,
	identityProjection *entityIdentityProjection,
	now time.Time,
	result *artifactSaveResult,
) (bool, []store.DirectEvidence, []*store.PreciseMemoryUnit) {
	if s == nil || s.Store == nil || result == nil {
		return false, existingEvidence, nil
	}
	lifecycle, lifecycleOK := s.Store.(store.MemoryDerivationLifecycleAvailability)
	if !lifecycleOK || !lifecycle.MemoryDerivationLifecycleEnabled() {
		return false, existingEvidence, nil
	}
	source, accepted := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	if !accepted ||
		source.ContractVersion != completeTurnSourceAcceptanceContract ||
		strings.TrimSpace(source.Revision) == "" {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: accepted current source is required")
		return true, existingEvidence, nil
	}
	writer, writerOK := s.Store.(store.MemoryAdmissionWriter)
	if !writerOK {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: common writer is unavailable")
		return true, existingEvidence, nil
	}
	if availability, ok := s.Store.(store.MemoryAdmissionWriteAvailability); ok &&
		!availability.MemoryAdmissionWritesEnabled() {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: common writer is disabled")
		return true, existingEvidence, nil
	}

	var memory *store.Memory
	if summary != "" {
		archiveHint := mapFromAny(extraction["archive_hint"])
		emotionalIntensity := clampFloat(extractionFloatFromAny(extraction["emotional_intensity"], 0), 0, 1)
		narrativeSignificance := clampFloat(extractionFloatFromAny(extraction["narrative_significance"], 0), 0, 1)
		baseImportance := clampFloat(extractionFloatFromAny(extraction["importance_score"], 3), 1, 10)
		emotionalBoost := emotionalImportanceBoost(emotionalIntensity)
		finalImportance := clampFloat(baseImportance+emotionalBoost, 1, 10)
		memory = &store.Memory{
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
	}

	desiredEvidence := buildMemoryAdmissionEvidence(
		sid, turnIndex, extraction, content, languageContext, existingEvidence, now, result,
	)
	evidenceSnapshot := replaceCurrentCriticEvidence(
		existingEvidence, sid, turnIndex, desiredEvidence,
	)
	preciseUnits := s.buildPreciseMemoryUnitsFromExtraction(
		ctx, sid, turnIndex, extraction, content, evidenceSnapshot,
		identityProjection, now, result,
	)
	for _, unit := range preciseUnits {
		if unit == nil {
			continue
		}
		unit.DerivationVersion = store.MemoryAdmissionContract
		unit.ExtractorVersion = completeTurnCriticPipelineVersion
		unit.IndexVersion = memoryAdmissionIndexVersion
	}

	perspectiveScoped := memoryAdmissionHasPerspectiveScopedContent(extraction)
	vectors := []store.MemoryAdmissionVector{}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" {
		if perspectiveScoped {
			result.addSkipReason("memory_vector", "perspective_scoped_content_requires_typed_delivery", map[string]any{
				"turn_index": turnIndex,
			})
		}
		if !perspectiveScoped && memory != nil && strings.TrimSpace(searchText) != "" {
			languageMeta := memoryVectorLanguageMetadata(*memory)
			vectors = append(vectors, store.MemoryAdmissionVector{
				ArtifactType:          "memory",
				Embedding:             embeddingVector,
				Tier:                  "memory",
				SourceTable:           "memories",
				SchemaVersion:         "memory.v2",
				DocumentText:          searchText,
				SearchTextPolicy:      extractionFirstNonEmpty(languageMeta["search_text_policy"], languageMemorySearchPolicy),
				RawLanguage:           languageMeta["raw_language"],
				SummaryLanguage:       languageMeta["summary_language"],
				SessionOutputLanguage: languageMeta["session_output_language"],
				AliasCount:            memorySearchText.AliasCount,
			})
		}
		for _, evidence := range desiredEvidence {
			if evidence == nil {
				continue
			}
			if evidence.EvidenceKind == "perspective_scoped_turn_excerpt" {
				result.addSkipReason("evidence_vector", "perspective_scoped_content_requires_typed_delivery", map[string]any{
					"turn_index": turnIndex,
				})
				continue
			}
			vectors = append(vectors, store.MemoryAdmissionVector{
				ArtifactType:     "evidence",
				EvidenceText:     evidence.EvidenceText,
				Tier:             "evidence",
				SourceTable:      "direct_evidence_records",
				SchemaVersion:    "direct_evidence.v1",
				DocumentText:     directEvidenceVectorDocumentText(*evidence),
				SearchTextPolicy: "derived_artifact_search_text.v1",
			})
		}
	}
	if usesVoyageContextualizedEmbedding(embCfg) {
		sourceStore, ok := s.Store.(store.SourceRevisionStore)
		if !ok {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: source revision store is unavailable for contextualized embedding")
			return true, existingEvidence, preciseUnits
		}
		canonicalSource, err := sourceStore.GetSourceRevision(ctx, sid, source.Revision)
		if err != nil || canonicalSource == nil {
			result.Errors++
			if err == nil {
				err = store.ErrNotFound
			}
			result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: canonical source revision is unavailable for contextualized embedding: "+err.Error())
			return true, existingEvidence, preciseUnits
		}
		contextChunks := make([]string, 0, len(vectors)+len(preciseUnits)+2)
		for _, rawTurnPart := range []string{canonicalSource.UserContent, canonicalSource.AssistantContent} {
			if rawTurnPart = strings.TrimSpace(rawTurnPart); rawTurnPart != "" {
				contextChunks = append(contextChunks, rawTurnPart)
			}
		}
		vectorContextPositions := make([]int, len(vectors))
		for i := range vectorContextPositions {
			vectorContextPositions[i] = -1
		}
		preciseContextPositions := make([]int, len(preciseUnits))
		for i := range preciseContextPositions {
			preciseContextPositions[i] = -1
		}
		memoryContextPosition := -1
		for i := range vectors {
			text := strings.TrimSpace(vectors[i].DocumentText)
			if text == "" {
				continue
			}
			vectorContextPositions[i] = len(contextChunks)
			if vectors[i].ArtifactType == "memory" {
				memoryContextPosition = len(contextChunks)
			}
			contextChunks = append(contextChunks, text)
		}
		if memoryContextPosition < 0 && !perspectiveScoped && memory != nil {
			if text := strings.TrimSpace(searchText); text != "" {
				memoryContextPosition = len(contextChunks)
				contextChunks = append(contextChunks, text)
			}
		}
		if len(vectors) == 0 {
			for _, evidence := range desiredEvidence {
				if evidence == nil || evidence.EvidenceKind == "perspective_scoped_turn_excerpt" {
					continue
				}
				if text := strings.TrimSpace(directEvidenceVectorDocumentText(*evidence)); text != "" {
					contextChunks = append(contextChunks, text)
				}
			}
		}
		for i, unit := range preciseUnits {
			if !store.PreciseMemoryGeneralVectorEligible(unit) {
				continue
			}
			text := strings.TrimSpace(unit.EvidenceExcerpt)
			if text == "" {
				continue
			}
			preciseContextPositions[i] = len(contextChunks)
			contextChunks = append(contextChunks, text)
		}
		for vectorIndex, position := range vectorContextPositions {
			if position < 0 {
				continue
			}
			vectors[vectorIndex].ContextChunks = append([]string(nil), contextChunks...)
			vectors[vectorIndex].ContextChunkIndex = position
			vectors[vectorIndex].EmbeddingModel = embCfg.Model
		}
		for preciseIndex, position := range preciseContextPositions {
			if position < 0 {
				continue
			}
			preciseUnits[preciseIndex].VectorContextChunks = append([]string(nil), contextChunks...)
			preciseUnits[preciseIndex].VectorContextChunkIndex = position
			preciseUnits[preciseIndex].VectorEmbeddingModel = embCfg.Model
		}
		if embCfg.hasConfig() && len(contextChunks) > 0 {
			embeddingStartedAt := time.Now()
			grouped, model, err := callDocumentEmbeddings(ctx, embCfg, contextChunks)
			result.addTiming("embedding", embeddingStartedAt)
			if err != nil {
				result.EmbeddingStatus = "error: " + err.Error()
				result.Warnings = append(result.Warnings, "contextualized_embedding_call_failed")
			} else {
				for vectorIndex, position := range vectorContextPositions {
					if position < 0 {
						continue
					}
					vectors[vectorIndex].Embedding = parseFloat32JSONList(grouped[position])
					vectors[vectorIndex].EmbeddingModel = model
				}
				for preciseIndex, position := range preciseContextPositions {
					if position < 0 {
						continue
					}
					preciseUnits[preciseIndex].VectorEmbedding = parseFloat32JSONList(grouped[position])
					preciseUnits[preciseIndex].VectorEmbeddingModel = model
				}
				if memory != nil && memoryContextPosition >= 0 {
					memory.Embedding = grouped[memoryContextPosition]
					memory.EmbeddingModel = model
				}
				result.EmbeddingStatus = "ok"
			}
		}
	}

	resultHash := memoryAdmissionResultHash(
		source.Revision, extraction, store.MemoryAdmissionContract,
		completeTurnCriticPipelineVersion, memoryAdmissionIndexVersion,
	)
	resultJSON := mustCompactJSON(normalizePreciseMemoryValue(extraction))
	admission := &store.MemoryAdmission{
		ContractVersion:   store.MemoryAdmissionContract,
		ChatSessionID:     sid,
		SourceRevision:    source.Revision,
		TurnIndex:         turnIndex,
		DerivationVersion: store.MemoryAdmissionContract,
		ExtractorVersion:  completeTurnCriticPipelineVersion,
		IndexVersion:      memoryAdmissionIndexVersion,
		ResultHash:        resultHash,
		ResultJSON:        resultJSON,
		Memory:            memory,
		Evidence:          desiredEvidence,
		PreciseUnits:      preciseUnits,
		Vectors:           vectors,
		CreatedAt:         now,
	}
	result.Attempted++
	commitStartedAt := time.Now()
	committed, err := writer.CommitMemoryAdmission(ctx, admission)
	result.addTiming("memory_admission_commit", commitStartedAt)
	if err != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: "+err.Error())
		return true, existingEvidence, nil
	}
	if committed.Idempotent {
		if committed.ExistingResultHash != "" && committed.ExistingResultHash != resultHash {
			result.Warnings = append(result.Warnings, "memory_admission_first_result_preserved")
		} else {
			result.addSkipReason("memory_admission", "idempotent_replay", map[string]any{
				"source_revision": source.Revision,
				"result_hash":     committed.CommittedResultHash,
			})
		}
		return true, evidenceSnapshot, preciseUnits
	}
	if committed.MemoryInserted || committed.MemoryUpdated {
		result.Memories++
	}
	result.Evidence += committed.EvidenceInserted + committed.EvidenceReactivated
	result.PreciseMemoryUnits += committed.PreciseInserted + committed.PreciseReactivated
	if committed.VectorOperations > 0 {
		result.VectorStatus = "queued"
		s.wakeMemoryWorkers()
	}
	if len(preciseUnits) > 0 {
		result.Attempted += len(preciseUnits)
	}
	return true, evidenceSnapshot, preciseUnits
}

func memoryAdmissionHasPerspectiveScopedContent(extraction map[string]any) bool {
	if len(extraction) == 0 {
		return false
	}
	for _, key := range []string{
		"belief_updates",
		"protected_secrets",
		"character_identity_accuracy",
		"subjective_entity_memories",
		"relationship_observations",
		"interaction_boundaries",
		"habit_observations",
		"character_profile_observations",
		"voice_observations",
		"user_interaction_profile",
		"rp_character_profile",
	} {
		if len(sliceFromAny(extraction[key])) > 0 {
			return true
		}
	}
	return false
}

func memoryAdmissionHasHolderScopedPerspectiveContent(extraction map[string]any) bool {
	if len(extraction) == 0 {
		return false
	}
	if len(sliceFromAny(extraction["belief_updates"])) > 0 {
		return true
	}
	for _, key := range []string{
		"relationship_observations",
		"interaction_boundaries",
		"habit_observations",
		"character_profile_observations",
		"voice_observations",
		"user_interaction_profile",
		"rp_character_profile",
	} {
		if len(sliceFromAny(extraction[key])) > 0 {
			return true
		}
	}
	for _, raw := range sliceFromAny(extraction["subjective_entity_memories"]) {
		item := mapFromAny(raw)
		if boolFromAny(item["secret_guard"]) ||
			stringSliceContains(stringsFromAny(item["tags"]), "protected_secret") ||
			stringSliceContains(stringsFromAny(item["tags"]), "protected_identity") {
			continue
		}
		return true
	}
	return false
}

func memoryAdmissionPerspectiveEvidenceScope(extraction map[string]any) (map[string]bool, bool) {
	protected := map[string]bool{}
	incomplete := false
	for _, key := range []string{
		"belief_updates",
		"protected_secrets",
		"character_identity_accuracy",
		"subjective_entity_memories",
		"relationship_observations",
		"interaction_boundaries",
		"habit_observations",
		"character_profile_observations",
		"voice_observations",
		"user_interaction_profile",
		"rp_character_profile",
	} {
		for _, raw := range sliceFromAny(extraction[key]) {
			item := mapFromAny(raw)
			excerpt := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "evidence_excerpt"),
				stringFromMap(item, "evidence"),
				stringFromMap(item, "source_excerpt"),
			))
			if excerpt == "" {
				incomplete = true
				continue
			}
			if normalized := normalizeArtifactDedupeText(excerpt); normalized != "" {
				protected[normalized] = true
			}
		}
	}
	return protected, incomplete
}

func buildMemoryAdmissionEvidence(
	sid string,
	turnIndex int,
	extraction map[string]any,
	content string,
	languageContext map[string]any,
	existing []store.DirectEvidence,
	now time.Time,
	result *artifactSaveResult,
) []*store.DirectEvidence {
	out := []*store.DirectEvidence{}
	seen := map[string]bool{}
	perspectiveScoped := memoryAdmissionHasPerspectiveScopedContent(extraction)
	perspectiveEvidenceKeys, perspectiveEvidenceIncomplete := memoryAdmissionPerspectiveEvidenceScope(extraction)
	maxID := int64(0)
	for _, item := range existing {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	for excerptIndex, rawText := range stringsFromAny(extraction["evidence_excerpts"]) {
		text := sanitizeEvidenceExcerptForTurn(rawText, content)
		if text == "" {
			result.addSkipReason("direct_evidence", "not_grounded_in_current_turn", rawText)
			continue
		}
		key := strings.TrimSpace(text)
		if seen[key] {
			result.addSkipReason("direct_evidence", "duplicate_source_turn_excerpt", map[string]any{
				"turn_index": turnIndex, "text": text,
			})
			continue
		}
		seen[key] = true
		evidenceKind := "turn_excerpt"
		if perspectiveScoped &&
			(perspectiveEvidenceIncomplete || perspectiveEvidenceKeys[normalizeArtifactDedupeText(text)]) {
			evidenceKind = "perspective_scoped_turn_excerpt"
		}
		evidence := &store.DirectEvidence{
			ID:                   maxID + int64(len(out)) + 1,
			ChatSessionID:        sid,
			EvidenceKind:         evidenceKind,
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
		for _, prior := range existing {
			if prior.ChatSessionID == sid &&
				prior.SourceTurnStart == turnIndex &&
				prior.SourceTurnEnd == turnIndex &&
				prior.CaptureStage == "critic_extract" &&
				strings.TrimSpace(prior.EvidenceText) == key {
				evidence.ID = prior.ID
				break
			}
		}
		baseImportance := clampFloat(extractionFloatFromAny(extraction["importance_score"], 3), 1, 10) / 10.0
		result.ConflictResolutions = append(result.ConflictResolutions, resolveCanonicalConflict(*evidence, existing)...)
		result.RetentionDecisions = append(result.RetentionDecisions, applyRetentionPolicy(evidence, baseImportance, existing))
		out = append(out, evidence)
	}
	return out
}

func replaceCurrentCriticEvidence(
	existing []store.DirectEvidence,
	sid string,
	turnIndex int,
	desired []*store.DirectEvidence,
) []store.DirectEvidence {
	out := make([]store.DirectEvidence, 0, len(existing)+len(desired))
	for _, item := range existing {
		if item.ChatSessionID == sid &&
			item.SourceTurnStart == turnIndex &&
			item.SourceTurnEnd == turnIndex &&
			item.CaptureStage == "critic_extract" {
			continue
		}
		out = append(out, item)
	}
	for _, item := range desired {
		if item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func memoryAdmissionResultHash(
	sourceRevision string,
	extraction map[string]any,
	derivationVersion string,
	extractorVersion string,
	indexVersion string,
) string {
	material := strings.Join([]string{
		strings.TrimSpace(sourceRevision),
		strings.TrimSpace(derivationVersion),
		strings.TrimSpace(extractorVersion),
		strings.TrimSpace(indexVersion),
		mustCompactJSON(normalizePreciseMemoryValue(extraction)),
	}, "\x1f")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(material)))
}
