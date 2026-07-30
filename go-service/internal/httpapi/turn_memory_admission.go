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
	embedding string,
	embeddingModel string,
	embeddingVector []float32,
	languageContext map[string]any,
	existingEvidence []store.DirectEvidence,
	identityProjection *entityIdentityProjection,
	now time.Time,
	result *artifactSaveResult,
) (bool, []store.DirectEvidence) {
	if s == nil || s.Store == nil || result == nil {
		return false, existingEvidence
	}
	lifecycle, lifecycleOK := s.Store.(store.MemoryDerivationLifecycleAvailability)
	if !lifecycleOK || !lifecycle.MemoryDerivationLifecycleEnabled() {
		return false, existingEvidence
	}
	source, accepted := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	if !accepted ||
		source.ContractVersion != completeTurnSourceAcceptanceContract ||
		strings.TrimSpace(source.Revision) == "" {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: accepted current source is required")
		return true, existingEvidence
	}
	writer, writerOK := s.Store.(store.MemoryAdmissionWriter)
	if !writerOK {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: common writer is unavailable")
		return true, existingEvidence
	}
	if availability, ok := s.Store.(store.MemoryAdmissionWriteAvailability); ok &&
		!availability.MemoryAdmissionWritesEnabled() {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "CommitMemoryAdmission: common writer is disabled")
		return true, existingEvidence
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

	vectors := []store.MemoryAdmissionVector{}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" {
		if memory != nil && strings.TrimSpace(searchText) != "" {
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
		return true, existingEvidence
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
		return true, evidenceSnapshot
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
	return true, evidenceSnapshot
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
		evidence := &store.DirectEvidence{
			ID:                   maxID + int64(len(out)) + 1,
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
