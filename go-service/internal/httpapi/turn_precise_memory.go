package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const preciseMemoryObjectiveConfidence = 0.7

type preciseMemoryCandidate struct {
	kind            string
	subtype         string
	excerpt         string
	payload         map[string]any
	confidence      float64
	truthScope      string
	epistemicMode   string
	authorityClass  string
	admissionState  string
	reviewState     string
	visibility      string
	revealCondition string
	relationshipKey string
	surfaces        map[string]string
	requiredRoles   map[string]bool
	resolvedIDs     map[string]string
	participants    []string
}

// appendPreciseMemoryEvidenceExcerpts adds only source-bound utterance spans
// that the existing narrative-state helper does not already add. It is gated
// by accepted-source context, so legacy/rescan paths retain their old evidence
// projection.
func appendPreciseMemoryEvidenceExcerpts(ctx context.Context, extraction map[string]any) map[string]any {
	if extraction == nil {
		return extraction
	}
	source, ok := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	if !ok || source.ContractVersion != completeTurnSourceAcceptanceContract || strings.TrimSpace(source.Revision) == "" {
		return extraction
	}
	excerpts := stringsFromAny(extraction["evidence_excerpts"])
	seen := map[string]bool{}
	for _, excerpt := range excerpts {
		seen[normalizeArtifactDedupeText(excerpt)] = true
	}
	for _, raw := range sliceFromAny(extraction["speaker_attributions"]) {
		item := mapFromAny(raw)
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "evidence_excerpt"),
			stringFromMap(item, "source_excerpt"),
		))
		key := normalizeArtifactDedupeText(excerpt)
		if excerpt == "" || key == "" || seen[key] {
			continue
		}
		seen[key] = true
		excerpts = append(excerpts, excerpt)
	}
	extraction["evidence_excerpts"] = excerpts
	return extraction
}

func (s *Server) savePreciseMemoryUnitsFromExtraction(
	ctx context.Context,
	sid string,
	turnIndex int,
	extraction map[string]any,
	content string,
	evidence []store.DirectEvidence,
	identities *entityIdentityProjection,
	now time.Time,
	result *artifactSaveResult,
) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	writer, ok := s.Store.(store.PreciseMemoryWriter)
	if !ok {
		return
	}
	if availability, ok := s.Store.(store.PreciseMemoryWriteAvailability); ok && !availability.PreciseMemoryWritesEnabled() {
		return
	}
	units := s.buildPreciseMemoryUnitsFromExtraction(
		ctx, sid, turnIndex, extraction, content, evidence, identities, now, result,
	)
	for _, unit := range units {
		result.Attempted++
		inserted, err := writer.SavePreciseMemoryUnit(ctx, unit)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SavePreciseMemoryUnit: "+err.Error())
			continue
		}
		if inserted {
			result.PreciseMemoryUnits++
		} else {
			result.addSkipReason("precise_memory_units", "idempotent_replay", map[string]any{
				"kind": unit.Kind, "idempotency_key": unit.IdempotencyKey,
			})
		}
	}
}

func (s *Server) buildPreciseMemoryUnitsFromExtraction(
	ctx context.Context,
	sid string,
	turnIndex int,
	extraction map[string]any,
	content string,
	evidence []store.DirectEvidence,
	identities *entityIdentityProjection,
	now time.Time,
	result *artifactSaveResult,
) []*store.PreciseMemoryUnit {
	units := []*store.PreciseMemoryUnit{}
	source, accepted := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	if !accepted ||
		source.ContractVersion != completeTurnSourceAcceptanceContract ||
		strings.TrimSpace(source.Revision) == "" {
		if result != nil {
			result.addSkipReason("precise_memory_units", "accepted_current_source_required", nil)
		}
		return units
	}
	source.ContentHash = fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	candidates := preciseMemoryCandidates(extraction)
	for _, candidate := range candidates {
		spanStart, spanEnd, exact := preciseMemoryExactSpan(candidate.payload, candidate.excerpt, content)
		if !exact {
			if result != nil {
				result.addSkipReason("precise_memory_units", "exact_unique_source_span_required", map[string]any{
					"kind": candidate.kind, "excerpt": candidate.excerpt,
				})
			}
			continue
		}
		evidenceIDs := preciseMemoryExactEvidenceIDs(evidence, sid, turnIndex, candidate.excerpt)
		if len(evidenceIDs) == 0 {
			if result != nil {
				result.addSkipReason("precise_memory_units", "accepted_direct_evidence_required", map[string]any{
					"kind": candidate.kind, "source_span_start": spanStart, "source_span_end": spanEnd,
				})
			}
			continue
		}
		candidate.applyIdentityPointers(identities, spanStart, spanEnd)
		payloadJSON := mustCompactJSON(normalizePreciseMemoryValue(candidate.payload))
		roleSurfaceJSON := mustCompactJSON(normalizePreciseMemoryValue(candidate.surfaces))
		keyMaterial := strings.Join([]string{
			source.Revision,
			candidate.kind,
			fmt.Sprintf("%d:%d", spanStart, spanEnd),
			normalizeArtifactComparableText(payloadJSON),
			normalizeArtifactComparableText(roleSurfaceJSON),
			candidate.truthScope,
			candidate.epistemicMode,
			candidate.authorityClass,
			candidate.admissionState,
			candidate.reviewState,
			candidate.visibility,
		}, "\x1f")
		idempotencyKey := fmt.Sprintf("%x", sha256.Sum256([]byte(keyMaterial)))
		evidenceHash := fmt.Sprintf("%x", sha256.Sum256([]byte(candidate.excerpt)))
		directEvidenceJSON := mustCompactJSON(evidenceIDs)
		unit := &store.PreciseMemoryUnit{
			UnitID:                preciseMemoryStableID(sid, idempotencyKey),
			ContractVersion:       store.PreciseMemoryUnitContract,
			ChatSessionID:         sid,
			SourceTurnStart:       turnIndex,
			SourceTurnEnd:         turnIndex,
			SourceContract:        source.ContractVersion,
			SourceRevision:        source.Revision,
			SourceLogicalTurnID:   source.LogicalTurnID,
			SourceMessageID:       source.MessageID,
			SourceGenerationID:    source.GenerationID,
			SourceContentHash:     source.ContentHash,
			SourceRole:            "combined_turn_pair",
			SourceSpanStart:       spanStart,
			SourceSpanEnd:         spanEnd,
			EvidenceExcerpt:       candidate.excerpt,
			EvidenceHash:          evidenceHash,
			RootEvidenceID:        evidenceIDs[0],
			DirectEvidenceIDsJSON: directEvidenceJSON,
			Kind:                  candidate.kind,
			Subtype:               candidate.subtype,
			PayloadJSON:           payloadJSON,
			RelationshipKey:       candidate.relationshipKey,
			TruthScope:            candidate.truthScope,
			EpistemicMode:         candidate.epistemicMode,
			AuthorityClass:        candidate.authorityClass,
			AdmissionState:        candidate.admissionState,
			ReviewState:           candidate.reviewState,
			Visibility:            candidate.visibility,
			RevealCondition:       candidate.revealCondition,
			Confidence:            candidate.confidence,
			IdempotencyKey:        idempotencyKey,
			DerivationVersion:     store.PreciseMemoryUnitContract,
			ExtractorVersion:      "complete_turn.configured_critic_extract",
			IndexVersion:          "not_materialized",
			LifecycleState:        "active",
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		candidate.assignIdentityPointers(unit)
		units = append(units, unit)
	}
	return units
}

func preciseMemoryCandidates(extraction map[string]any) []preciseMemoryCandidate {
	out := []preciseMemoryCandidate{}
	for _, raw := range sliceFromAny(extraction["narrative_events"]) {
		item := mapFromAny(raw)
		summary := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "summary"), stringFromMap(item, "event")))
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")))
		if summary == "" || excerpt == "" {
			continue
		}
		candidate := preciseMemoryCandidate{
			kind: "event", subtype: preciseMemorySubtype(item, "event_type", "observed_event"),
			excerpt: excerpt, confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
			payload: preciseMemorySemanticPayload(item, []string{
				"summary", "event", "event_type", "actor", "actor_name",
				"affected_entity", "target", "location", "object", "participants",
				"relationship_key", "claim_scope", "epistemic_mode", "modality",
				"truth_scope", "truth_status", "event_status", "statement_type",
				"is_lie", "is_deception", "known_false", "is_uncertain",
				"is_speculation", "speculative", "is_proposal", "proposed",
				"hypothetical", "is_ooc", "ooc", "out_of_character",
				"source_span_start", "source_span_end",
			}),
			truthScope: "objective", epistemicMode: "direct", authorityClass: "objective_world_state",
			admissionState: "committed", reviewState: "source_observed", visibility: "public",
			relationshipKey: strings.TrimSpace(stringFromMap(item, "relationship_key")),
			surfaces: map[string]string{
				"actor":    extractionFirstNonEmpty(stringFromMap(item, "actor"), stringFromMap(item, "actor_name")),
				"affected": extractionFirstNonEmpty(stringFromMap(item, "affected_entity"), stringFromMap(item, "target")),
				"location": stringFromMap(item, "location"),
				"object":   stringFromMap(item, "object"),
			},
			requiredRoles: map[string]bool{},
			participants:  preciseMemoryParticipantSurfaces(item["participants"]),
		}
		for role, surface := range candidate.surfaces {
			candidate.requiredRoles[role] = strings.TrimSpace(surface) != ""
		}
		candidate.applySemanticAuthority(item)
		out = append(out, candidate)
	}
	for _, source := range []struct {
		field string
		kind  string
	}{
		{field: "state_claims", kind: "state"},
		{field: "belief_updates", kind: "observation"},
	} {
		for _, raw := range sliceFromAny(extraction[source.field]) {
			item := mapFromAny(raw)
			subject := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "subject"), stringFromMap(item, "entity"), stringFromMap(item, "owner"),
			))
			stateSlot := normalizeNarrativeStateSlot(extractionFirstNonEmpty(
				stringFromMap(item, "state_slot"), stringFromMap(item, "slot"), stringFromMap(item, "relation_dimension"),
			))
			value := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "value"), stringFromMap(item, "state_value"), stringFromMap(item, "belief"),
			))
			excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")))
			if subject == "" || stateSlot == "" || value == "" || excerpt == "" {
				continue
			}
			subjectType := normalizeNarrativeSubjectType(stringFromMap(item, "subject_type"))
			if subjectType == "" {
				subjectType = "entity"
			}
			perspectiveOwner := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "perspective_owner"), stringFromMap(item, "believer"), stringFromMap(item, "knower"),
			))
			candidate := preciseMemoryCandidate{
				kind: source.kind, subtype: stateSlot, excerpt: excerpt,
				confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
				payload: preciseMemorySemanticPayload(item, []string{
					"subject", "entity", "owner", "subject_type", "state_slot", "slot",
					"relation_dimension", "value", "state_value", "belief", "claim_scope",
					"perspective_owner", "believer", "knower", "transition", "epistemic_mode",
					"modality", "truth_scope", "truth_status", "statement_type",
					"is_lie", "is_deception", "known_false", "is_uncertain",
					"is_speculation", "speculative", "is_proposal", "proposed",
					"hypothetical", "is_ooc", "ooc", "out_of_character",
					"visibility", "reveal_condition", "source_span_start", "source_span_end",
				}),
				truthScope: "objective", epistemicMode: "direct", authorityClass: "objective_world_state",
				admissionState: "committed", reviewState: "source_observed", visibility: "public",
				relationshipKey: strings.TrimSpace(stringFromMap(item, "relationship_key")),
				surfaces:        map[string]string{"subject": subject},
				requiredRoles:   map[string]bool{"subject": preciseMemorySubjectNeedsIdentity(subjectType)},
			}
			if source.kind == "observation" {
				candidate.truthScope = "owner_scoped"
				candidate.epistemicMode = preciseMemorySubtype(item, "epistemic_mode", "belief")
				candidate.authorityClass = "subjective_episodic"
				candidate.visibility = preciseMemoryVisibility(item, "owner_private")
				candidate.revealCondition = strings.TrimSpace(stringFromMap(item, "reveal_condition"))
				candidate.surfaces["knower"] = perspectiveOwner
				candidate.requiredRoles["knower"] = true
			} else {
				candidate.visibility = preciseMemoryVisibility(item, "public")
				candidate.applySemanticAuthority(item)
			}
			out = append(out, candidate)
		}
	}
	for _, raw := range sliceFromAny(extraction["speaker_attributions"]) {
		item := mapFromAny(raw)
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "source_excerpt")))
		if excerpt == "" {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(stringFromMap(item, "attribution_state")))
		candidate := preciseMemoryCandidate{
			kind: "utterance", subtype: preciseMemorySubtype(item, "attribution_kind", "unknown"),
			excerpt: excerpt, confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0), 0, 1),
			payload: preciseMemorySemanticPayload(item, []string{
				"speaker_name", "speaker", "attribution_kind", "attribution_state",
				"source_span_start", "source_span_end",
			}),
			truthScope: "source_occurrence", epistemicMode: "utterance_content_unverified",
			authorityClass: "objective_world_state", admissionState: "committed",
			reviewState: "source_observed", visibility: "public",
			surfaces: map[string]string{
				"actor": extractionFirstNonEmpty(stringFromMap(item, "speaker_name"), stringFromMap(item, "speaker")),
			},
			requiredRoles: map[string]bool{"actor": true},
		}
		if state != "linked" {
			candidate.markNeedsReview()
		}
		out = append(out, candidate)
	}
	return out
}

func (candidate *preciseMemoryCandidate) applySemanticAuthority(item map[string]any) {
	classification := preciseMemoryStructuredClassification(item)
	switch classification {
	case "ooc_meta":
		candidate.truthScope = "ooc_meta"
		candidate.epistemicMode = "meta"
		candidate.authorityClass = "ooc_meta"
		candidate.visibility = "restricted"
	case "proposal":
		candidate.truthScope = "proposed"
		candidate.epistemicMode = "proposal"
		candidate.authorityClass = "support_hypothesis"
	case "deception":
		candidate.truthScope = "speaker_claim"
		candidate.epistemicMode = "deception_or_false_claim"
		candidate.authorityClass = "subjective_episodic"
	case "non_objective":
		candidate.truthScope = "non_objective"
		candidate.epistemicMode = "inferred_or_uncertain"
		candidate.authorityClass = "support_hypothesis"
	}
	if candidate.authorityClass == "objective_world_state" && candidate.confidence < preciseMemoryObjectiveConfidence {
		candidate.truthScope = "unverified"
		candidate.epistemicMode = "uncertain"
		candidate.authorityClass = "support_hypothesis"
		candidate.markNeedsReview()
	}
}

func (candidate *preciseMemoryCandidate) applyIdentityPointers(identities *entityIdentityProjection, spanStart, spanEnd int) {
	for role, required := range candidate.requiredRoles {
		surface := strings.TrimSpace(candidate.surfaces[role])
		if surface == "" {
			if required {
				candidate.markNeedsReview()
			}
			continue
		}
		if identities == nil {
			if required {
				candidate.markNeedsReview()
			}
			continue
		}
		id, ok := identities.preciseMemoryEntityPointer(surface, spanStart, spanEnd)
		if !ok {
			if required {
				candidate.markNeedsReview()
			}
			continue
		}
		if candidate.resolvedIDs == nil {
			candidate.resolvedIDs = map[string]string{}
		}
		candidate.resolvedIDs[role] = id
	}
	if len(candidate.participants) == 0 {
		return
	}
	participantIDs := make([]string, 0, len(candidate.participants))
	for _, surface := range candidate.participants {
		if identities == nil {
			candidate.markNeedsReview()
			continue
		}
		id, ok := identities.preciseMemoryEntityPointer(surface, spanStart, spanEnd)
		if !ok {
			candidate.markNeedsReview()
			continue
		}
		participantIDs = append(participantIDs, id)
	}
	if len(participantIDs) == len(candidate.participants) {
		sort.Strings(participantIDs)
		candidate.payload["participant_entity_ids"] = participantIDs
	}
}

func (candidate *preciseMemoryCandidate) assignIdentityPointers(unit *store.PreciseMemoryUnit) {
	for role, id := range candidate.resolvedIDs {
		switch role {
		case "actor":
			unit.ActorEntityID = id
		case "subject":
			unit.SubjectEntityID = id
		case "affected":
			unit.AffectedEntityID = id
		case "location":
			unit.LocationEntityID = id
		case "object":
			unit.ObjectEntityID = id
		case "knower":
			unit.KnowledgeHolderEntityID = id
		}
	}
}

func (candidate *preciseMemoryCandidate) markNeedsReview() {
	candidate.admissionState = "review_required"
	candidate.reviewState = "needs_review"
	if candidate.authorityClass == "objective_world_state" && candidate.truthScope == "objective" {
		candidate.truthScope = "unresolved_subject"
		candidate.epistemicMode = "unresolved"
		candidate.authorityClass = "support_hypothesis"
	}
}

func preciseMemoryStructuredClassification(item map[string]any) string {
	for _, key := range []string{"is_ooc", "ooc", "out_of_character"} {
		if boolFromAny(item[key]) {
			return "ooc_meta"
		}
	}
	for _, key := range []string{"is_proposal", "proposed", "hypothetical"} {
		if boolFromAny(item[key]) {
			return "proposal"
		}
	}
	for _, key := range []string{"is_lie", "is_deception", "known_false"} {
		if boolFromAny(item[key]) {
			return "deception"
		}
	}
	for _, key := range []string{"is_uncertain", "is_speculation", "speculative"} {
		if boolFromAny(item[key]) {
			return "non_objective"
		}
	}
	for _, key := range []string{
		"claim_scope", "epistemic_mode", "modality", "truth_scope",
		"truth_status", "assertion_status", "event_status", "event_type",
		"statement_type", "transition",
	} {
		value := strings.ToLower(strings.TrimSpace(extractionStringFromAny(item[key])))
		switch value {
		case "ooc", "ooc_meta", "meta", "out_of_character":
			return "ooc_meta"
		case "proposal", "proposed", "plan", "planned", "hypothetical":
			return "proposal"
		case "lie", "false", "deception", "deceptive", "known_false":
			return "deception"
		case "belief", "subjective", "perception", "rumor", "hearsay", "inferred",
			"misunderstood", "uncertain", "uncertainty", "speculation", "speculative",
			"suspected", "secret", "private":
			return "non_objective"
		}
	}
	return "objective"
}

func preciseMemoryExactSpan(payload map[string]any, excerpt, content string) (int, int, bool) {
	excerpt = strings.TrimSpace(excerpt)
	if excerpt == "" || content == "" {
		return 0, 0, false
	}
	start := intFromAny(payload["source_span_start"], -1)
	end := intFromAny(payload["source_span_end"], -1)
	if start >= 0 && end > start && end <= len(content) && content[start:end] == excerpt {
		return start, end, true
	}
	first := strings.Index(content, excerpt)
	if first < 0 {
		return 0, 0, false
	}
	if strings.Index(content[first+len(excerpt):], excerpt) >= 0 {
		return 0, 0, false
	}
	return first, first + len(excerpt), true
}

func preciseMemoryExactEvidenceIDs(evidence []store.DirectEvidence, sid string, turnIndex int, excerpt string) []int64 {
	ids := []int64{}
	for _, item := range evidence {
		if item.ID <= 0 || item.ChatSessionID != sid ||
			strings.TrimSpace(item.EvidenceText) != strings.TrimSpace(excerpt) ||
			item.RepairNeeded || item.Tombstoned || item.SupersededByID > 0 ||
			item.ArchiveState != "verified_direct" ||
			item.CaptureStage != "critic_extract" ||
			item.CaptureVerification != "verified" ||
			item.CommittedGate != "auto_grounded_excerpt" {
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
		if start != turnIndex || end != turnIndex {
			continue
		}
		ids = append(ids, item.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func preciseMemorySemanticPayload(item map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for _, field := range fields {
		if value, ok := item[field]; ok {
			out[field] = normalizePreciseMemoryValue(value)
		}
	}
	return out
}

func preciseMemoryParticipantSurfaces(value any) []string {
	out := []string{}
	for _, raw := range sliceFromAny(value) {
		surface := ""
		switch typed := raw.(type) {
		case string:
			surface = typed
		default:
			item := mapFromAny(typed)
			surface = extractionFirstNonEmpty(
				stringFromMap(item, "name"),
				stringFromMap(item, "label"),
				stringFromMap(item, "entity"),
			)
		}
		surface = strings.TrimSpace(surface)
		if surface != "" {
			out = append(out, surface)
		}
	}
	sort.Strings(out)
	return out
}

func normalizePreciseMemoryValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, nested := range typed {
			out[strings.TrimSpace(key)] = normalizePreciseMemoryValue(nested)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, nested := range typed {
			out = append(out, normalizePreciseMemoryValue(nested))
		}
		sort.SliceStable(out, func(i, j int) bool {
			left, _ := json.Marshal(out[i])
			right, _ := json.Marshal(out[j])
			return string(left) < string(right)
		})
		return out
	case string:
		return strings.Join(strings.Fields(strings.TrimSpace(typed)), " ")
	default:
		return value
	}
}

func preciseMemorySubtype(item map[string]any, field, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(stringFromMap(item, field)))
	if value == "" {
		return fallback
	}
	return normalizeNarrativeStateSlot(value)
}

func preciseMemoryVisibility(item map[string]any, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(stringFromMap(item, "visibility"))) {
	case "public", "player_known", "owner_private", "restricted", "reveal_required":
		return strings.ToLower(strings.TrimSpace(stringFromMap(item, "visibility")))
	default:
		return fallback
	}
}

func preciseMemorySubjectNeedsIdentity(subjectType string) bool {
	switch strings.ToLower(strings.TrimSpace(subjectType)) {
	case "world", "session":
		return false
	default:
		return true
	}
}

func preciseMemoryStableID(sid, idempotencyKey string) string {
	sum := sha256.Sum256([]byte("precise-memory\x00" + sid + "\x00" + idempotencyKey))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(bytes)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32]
}
