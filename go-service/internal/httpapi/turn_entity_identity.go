package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	entityIdentityLegacySource = "legacy_unverified"
)

type entityIdentitySourceContext struct {
	ContractVersion string
	Revision        string
	LogicalTurnID   string
	MessageID       string
	GenerationID    string
	ContentHash     string
}

type entityIdentitySourceContextKey struct{}

func contextWithEntityIdentitySource(ctx context.Context, decision completeTurnSourceAcceptanceDecision) context.Context {
	if !decision.Enabled || !decision.Accepted {
		return ctx
	}
	messageID := ""
	if decision.Observation.HostChatID != "" || decision.Observation.MessageIndex >= 0 {
		messageID = fmt.Sprintf("%s:index:%d", decision.Observation.HostChatID, decision.Observation.MessageIndex)
	}
	return context.WithValue(ctx, entityIdentitySourceContextKey{}, entityIdentitySourceContext{
		ContractVersion: completeTurnSourceAcceptanceContract,
		Revision:        decision.Revision,
		LogicalTurnID:   decision.LogicalTurnID,
		MessageID:       messageID,
		GenerationID:    decision.Observation.GenerationID,
	})
}

func entityIdentitySourceFromContext(ctx context.Context, sid string, turnIndex int, content string) entityIdentitySourceContext {
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	if value, ok := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext); ok {
		value.ContentHash = contentHash
		if strings.TrimSpace(value.ContractVersion) == "" {
			value.ContractVersion = completeTurnSourceAcceptanceContract
		}
		if strings.TrimSpace(value.Revision) == "" {
			value.Revision = "source:" + contentHash
		}
		return value
	}
	return entityIdentitySourceContext{
		ContractVersion: entityIdentityLegacySource,
		Revision:        fmt.Sprintf("legacy:%s:%d:%s", sid, turnIndex, contentHash[:16]),
		ContentHash:     contentHash,
	}
}

type entityIdentityOccurrence struct {
	StableEntityID  string
	Name            string
	Namespace       string
	EntityKind      string
	ReviewState     string
	SourceSpanStart int
	SourceSpanEnd   int
}

type entityIdentitySurfaceCandidate struct {
	Occurrence  *entityIdentityOccurrence
	ReviewState string
}

type entityIdentityProjection struct {
	sid              string
	turnIndex        int
	content          string
	source           entityIdentitySourceContext
	now              time.Time
	writer           store.EntityIdentityWriter
	bySurface        map[string][]entityIdentitySurfaceCandidate
	fullDisplayNames []string
	conflictedKeys   map[string]bool
	displaySeen      map[string]int
	aliasSeen        map[string]int
	speakerSeen      map[string]int
	sourceIndex      int
}

func (s *Server) buildEntityIdentityProjection(ctx context.Context, sid string, turnIndex int, extraction map[string]any, content string, now time.Time, result *artifactSaveResult) *entityIdentityProjection {
	writer, ok := s.Store.(store.EntityIdentityWriter)
	if !ok {
		return nil
	}
	if availability, ok := s.Store.(store.EntityIdentityWriteAvailability); ok && !availability.EntityIdentityWritesEnabled() {
		return nil
	}
	projection := &entityIdentityProjection{
		sid:            sid,
		turnIndex:      turnIndex,
		content:        content,
		source:         entityIdentitySourceFromContext(ctx, sid, turnIndex, content),
		now:            now,
		writer:         writer,
		bySurface:      map[string][]entityIdentitySurfaceCandidate{},
		conflictedKeys: map[string]bool{},
		displaySeen:    map[string]int{},
		aliasSeen:      map[string]int{},
		speakerSeen:    map[string]int{},
	}
	entities := mapFromAny(extraction["entities"])
	buckets := []struct {
		key        string
		entityKind string
	}{
		{key: "characters", entityKind: "character"},
		{key: "locations", entityKind: "location"},
		{key: "places", entityKind: "location"},
		{key: "items", entityKind: "item"},
		{key: "objects", entityKind: "item"},
		{key: "groups", entityKind: "group"},
	}
	surfaceOwners := map[string]map[string]struct{}{}
	for _, bucket := range buckets {
		for itemIndex, raw := range sliceFromAny(entities[bucket.key]) {
			item := mapFromAny(raw)
			name := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "name"),
				stringFromMap(item, "label"),
				stringFromMap(item, "title"),
			))
			if name == "" || isPlaceholderKGPart(name) {
				continue
			}
			projection.fullDisplayNames = append(projection.fullDisplayNames, name)
			owner := fmt.Sprintf("%s:%d", bucket.key, itemIndex)
			for _, surface := range append([]string{name}, stringsFromAny(item["aliases"])...) {
				key := comparableEntityKey(surface)
				if key == "" {
					continue
				}
				if surfaceOwners[key] == nil {
					surfaceOwners[key] = map[string]struct{}{}
				}
				surfaceOwners[key][owner] = struct{}{}
			}
		}
	}
	for key, owners := range surfaceOwners {
		if len(owners) > 1 {
			projection.conflictedKeys[key] = true
		}
	}
	artifactOrdinal := 0
	for _, bucket := range buckets {
		for itemIndex, raw := range sliceFromAny(entities[bucket.key]) {
			item := mapFromAny(raw)
			name := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "name"),
				stringFromMap(item, "label"),
				stringFromMap(item, "title"),
			))
			if name == "" || isPlaceholderKGPart(name) {
				continue
			}
			kind := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "entity_type"), bucket.entityKind))
			namespace := identityNamespaceForOccurrence(kind, stringFromMap(item, "identity_namespace"))
			occurrence := projection.persistOccurrence(ctx, name, namespace, kind, bucket.key, itemIndex, stringsFromAny(item["aliases"]), result)
			if occurrence != nil {
				projection.persistArtifactBinding(ctx, occurrence, "entity", bucket.key, artifactOrdinal, name, occurrence.ReviewState, result)
			}
			artifactOrdinal++
		}
	}
	projection.persistSpeakerAttributions(ctx, extraction, result)
	return projection
}

func identityNamespaceForOccurrence(entityKind, requested string) string {
	requested = strings.ToLower(strings.TrimSpace(requested))
	switch requested {
	case "session_npc", "session_player", "session_group", "session_location", "session_item", "session_unknown",
		"host_setting", "reference_entity", "user_ooc", "global":
		return requested
	}
	switch strings.ToLower(strings.TrimSpace(entityKind)) {
	case "location", "place":
		return "session_location"
	case "item", "object", "artifact":
		return "session_item"
	case "group", "faction", "organization":
		return "session_group"
	case "player", "protagonist", "player_character":
		return "session_player"
	case "character", "npc", "person":
		return "session_npc"
	default:
		return "session_unknown"
	}
}

func (p *entityIdentityProjection) persistOccurrence(ctx context.Context, name, namespace, entityKind, sourceBucket string, itemIndex int, aliases []string, result *artifactSaveResult) *entityIdentityOccurrence {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	sourceIndex := p.sourceIndex
	p.sourceIndex++
	spanStart, spanEnd := p.nextExactSpan(name)
	occurrenceLocator := "artifact:" + fmt.Sprint(itemIndex)
	if spanStart >= 0 {
		occurrenceLocator = fmt.Sprintf("span:%d:%d", spanStart, spanEnd)
	}
	idempotencyKey := entityIdentityIdempotencyKey("identity", p.source.Revision, sourceBucket, occurrenceLocator, namespace, entityKind)
	stableID := entityIdentityStableID("entity", p.sid, idempotencyKey)
	reviewState := "source_observed"
	presenceAuthority := "observed"
	occurrenceAuthority := "source_span"
	if spanStart < 0 || namespace == "host_setting" || namespace == "reference_entity" || namespace == "global" {
		reviewState = "needs_review"
		presenceAuthority = "unverified"
		occurrenceAuthority = "none"
	} else if p.conflictedKeys[comparableEntityKey(name)] {
		reviewState = "needs_review"
	}
	occurrence := &entityIdentityOccurrence{
		StableEntityID:  stableID,
		Name:            name,
		Namespace:       namespace,
		EntityKind:      entityKind,
		ReviewState:     reviewState,
		SourceSpanStart: spanStart,
		SourceSpanEnd:   spanEnd,
	}
	result.trySave("SaveEntityIdentity", func() error {
		return p.writer.SaveEntityIdentity(ctx, &store.EntityIdentity{
			StableEntityID:      stableID,
			ChatSessionID:       p.sid,
			IdentityNamespace:   namespace,
			EntityKind:          entityKind,
			CanonicalLabel:      name,
			LifecycleState:      "active",
			ReviewState:         reviewState,
			PresenceAuthority:   presenceAuthority,
			OccurrenceAuthority: occurrenceAuthority,
			SourceContract:      p.source.ContractVersion,
			SourceRevision:      p.source.Revision,
			SourceLogicalTurnID: p.source.LogicalTurnID,
			SourceMessageID:     p.source.MessageID,
			SourceGenerationID:  p.source.GenerationID,
			SourceContentHash:   p.source.ContentHash,
			SourceTurn:          p.turnIndex,
			SourceIndex:         sourceIndex,
			IdempotencyKey:      idempotencyKey,
			MappingRevision:     1,
			FirstSeenTurn:       p.turnIndex,
			LastSeenTurn:        p.turnIndex,
			CreatedAt:           p.now,
			UpdatedAt:           p.now,
		})
	}, result, func() { result.EntityIdentities++ })
	p.persistSurface(ctx, occurrence, "display_name", name, spanStart, spanEnd, reviewState, result)
	p.indexSurface(name, occurrence, reviewState)
	for aliasIndex, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || strings.EqualFold(alias, name) {
			continue
		}
		aliasStart, aliasEnd := -1, -1
		if !p.conflictedKeys[comparableEntityKey(alias)] {
			aliasStart, aliasEnd = p.nextIndependentAliasSpan(alias)
		}
		aliasReview := "needs_review"
		p.persistSurface(ctx, occurrence, fmt.Sprintf("alias_%d", aliasIndex), alias, aliasStart, aliasEnd, aliasReview, result)
		p.indexSurface(alias, occurrence, aliasReview)
	}
	return occurrence
}

func (p *entityIdentityProjection) indexSurface(surface string, occurrence *entityIdentityOccurrence, reviewState string) {
	key := comparableEntityKey(surface)
	if key == "" || occurrence == nil {
		return
	}
	candidates := p.bySurface[key]
	for index, candidate := range candidates {
		if candidate.Occurrence == nil || candidate.Occurrence.StableEntityID != occurrence.StableEntityID {
			continue
		}
		if candidate.ReviewState != "source_observed" && reviewState == "source_observed" {
			candidates[index].ReviewState = reviewState
			p.bySurface[key] = candidates
		}
		return
	}
	p.bySurface[key] = append(candidates, entityIdentitySurfaceCandidate{
		Occurrence:  occurrence,
		ReviewState: reviewState,
	})
}

func (p *entityIdentityProjection) persistSurface(ctx context.Context, occurrence *entityIdentityOccurrence, kind, text string, spanStart, spanEnd int, reviewState string, result *artifactSaveResult) {
	key := entityIdentityIdempotencyKey("surface", occurrence.StableEntityID, kind, comparableEntityKey(text), fmt.Sprint(spanStart))
	surfaceID := entityIdentityStableID("surface", p.sid, key)
	evidence := ""
	if spanStart >= 0 && spanEnd > spanStart && spanEnd <= len(p.content) {
		evidence = p.content[spanStart:spanEnd]
	}
	result.trySave("SaveEntityIdentitySurface", func() error {
		return p.writer.SaveEntityIdentitySurface(ctx, &store.EntityIdentitySurface{
			SurfaceID:         surfaceID,
			StableEntityID:    occurrence.StableEntityID,
			ChatSessionID:     p.sid,
			IdentityNamespace: occurrence.Namespace,
			SurfaceKind:       kind,
			SurfaceText:       text,
			NormalizedSurface: comparableEntityKey(text),
			Scope:             "source_turn",
			ValidFromTurn:     p.turnIndex,
			SourceContract:    p.source.ContractVersion,
			SourceRevision:    p.source.Revision,
			SourceTurn:        p.turnIndex,
			SourceSpanStart:   spanStart,
			SourceSpanEnd:     spanEnd,
			EvidenceExcerpt:   evidence,
			ReviewState:       reviewState,
			IdempotencyKey:    key,
			CreatedAt:         p.now,
			UpdatedAt:         p.now,
		})
	}, result, func() { result.IdentitySurfaces++ })
}

func (p *entityIdentityProjection) nextExactSpan(text string) (int, int) {
	return p.nextIndependentSurfaceSpan(text, p.displaySeen)
}

func (p *entityIdentityProjection) nextIndependentAliasSpan(text string) (int, int) {
	return p.nextIndependentSurfaceSpan(text, p.aliasSeen)
}

func (p *entityIdentityProjection) nextIndependentSurfaceSpan(text string, seen map[string]int) (int, int) {
	nth := seen[text]
	seen[text] = nth + 1
	offset := 0
	for {
		found := strings.Index(p.content[offset:], text)
		if found < 0 {
			return -1, -1
		}
		spanStart := offset + found
		spanEnd := spanStart + len(text)
		if !p.surfaceSpanNestedInLongerDisplayName(text, spanStart, spanEnd) {
			if nth == 0 {
				return spanStart, spanEnd
			}
			nth--
		}
		offset = spanEnd
	}
}

func (p *entityIdentityProjection) surfaceSpanNestedInLongerDisplayName(surface string, surfaceStart, surfaceEnd int) bool {
	return p.surfaceSpanNestedInLongerDisplayNameWithin(p.content, surface, surfaceStart, surfaceEnd)
}

func (p *entityIdentityProjection) surfaceSpanNestedInLongerDisplayNameWithin(content, surface string, surfaceStart, surfaceEnd int) bool {
	for _, displayName := range p.fullDisplayNames {
		if len(displayName) <= len(surface) {
			continue
		}
		offset := 0
		for {
			found := strings.Index(content[offset:], displayName)
			if found < 0 {
				break
			}
			nameStart := offset + found
			nameEnd := nameStart + len(displayName)
			if nameStart <= surfaceStart && surfaceEnd <= nameEnd {
				return true
			}
			offset = nameEnd
		}
	}
	return false
}

func (p *entityIdentityProjection) surfaceAppearsIndependentlyWithinSourceSpan(surface string, spanStart, spanEnd int) bool {
	if strings.TrimSpace(surface) == "" || spanStart < 0 || spanEnd <= spanStart || spanEnd > len(p.content) {
		return false
	}
	content := p.content[spanStart:spanEnd]
	offset := 0
	for {
		found := strings.Index(content[offset:], surface)
		if found < 0 {
			return false
		}
		surfaceStart := spanStart + offset + found
		surfaceEnd := surfaceStart + len(surface)
		if !p.surfaceSpanNestedInLongerDisplayName(surface, surfaceStart, surfaceEnd) {
			return true
		}
		offset += found + len(surface)
	}
}

func nextExactSpanWithSeen(content, text string, seen map[string]int) (int, int) {
	key := text
	nth := seen[key]
	seen[key] = nth + 1
	offset := 0
	for i := 0; i <= nth; i++ {
		found := strings.Index(content[offset:], text)
		if found < 0 {
			return -1, -1
		}
		offset += found
		if i < nth {
			offset += len(text)
		}
	}
	return offset, offset + len(text)
}

func (p *entityIdentityProjection) resolveUnique(surface string) (*entityIdentityOccurrence, string, bool) {
	candidates := p.bySurface[comparableEntityKey(surface)]
	var selected *entityIdentityOccurrence
	reviewState := "source_observed"
	for _, candidate := range candidates {
		if candidate.Occurrence == nil {
			continue
		}
		if selected != nil && selected.StableEntityID != candidate.Occurrence.StableEntityID {
			return nil, "needs_review", true
		}
		selected = candidate.Occurrence
		if candidate.ReviewState != "source_observed" || selected.ReviewState != "source_observed" {
			reviewState = "needs_review"
		}
	}
	if selected == nil {
		reviewState = ""
	} else if reviewState != "source_observed" {
		return nil, "needs_review", true
	}
	return selected, reviewState, false
}

func (p *entityIdentityProjection) ensureArtifactIdentity(ctx context.Context, surface, entityKind, artifactKind string, ordinal int, result *artifactSaveResult) (*entityIdentityOccurrence, string) {
	if occurrence, reviewState, ambiguous := p.resolveUnique(surface); occurrence != nil {
		return occurrence, reviewState
	} else if ambiguous {
		return p.persistUnknownOccurrence(ctx, surface, entityKind, artifactKind, ordinal, result), "needs_review"
	}
	if artifactKind == "character_state" {
		occurrence := p.persistOccurrence(ctx, surface, "session_npc", entityKind, artifactKind, ordinal, nil, result)
		if occurrence == nil {
			return nil, ""
		}
		return occurrence, occurrence.ReviewState
	}
	return nil, ""
}

func (p *entityIdentityProjection) persistUnknownOccurrence(ctx context.Context, label, entityKind, sourceBucket string, itemIndex int, result *artifactSaveResult) *entityIdentityOccurrence {
	sourceIndex := p.sourceIndex
	p.sourceIndex++
	if strings.TrimSpace(label) == "" {
		label = "unknown speaker"
	}
	key := entityIdentityIdempotencyKey("unknown", p.source.Revision, sourceBucket, fmt.Sprint(itemIndex))
	stableID := entityIdentityStableID("entity", p.sid, key)
	occurrence := &entityIdentityOccurrence{
		StableEntityID:  stableID,
		Name:            label,
		Namespace:       "session_unknown",
		EntityKind:      entityKind,
		ReviewState:     "needs_review",
		SourceSpanStart: -1,
		SourceSpanEnd:   -1,
	}
	result.trySave("SaveEntityIdentity(unknown)", func() error {
		return p.writer.SaveEntityIdentity(ctx, &store.EntityIdentity{
			StableEntityID: stableID, ChatSessionID: p.sid, IdentityNamespace: "session_unknown",
			EntityKind: entityKind, CanonicalLabel: label, LifecycleState: "tentative",
			ReviewState: "needs_review", PresenceAuthority: "unverified", OccurrenceAuthority: "none",
			SourceContract: p.source.ContractVersion, SourceRevision: p.source.Revision,
			SourceLogicalTurnID: p.source.LogicalTurnID, SourceMessageID: p.source.MessageID,
			SourceGenerationID: p.source.GenerationID, SourceContentHash: p.source.ContentHash,
			SourceTurn: p.turnIndex, SourceIndex: sourceIndex, IdempotencyKey: key,
			MappingRevision: 1, FirstSeenTurn: p.turnIndex, LastSeenTurn: p.turnIndex,
			CreatedAt: p.now, UpdatedAt: p.now,
		})
	}, result, func() { result.EntityIdentities++ })
	return occurrence
}

func (p *entityIdentityProjection) persistArtifactBinding(ctx context.Context, occurrence *entityIdentityOccurrence, artifactKind, role string, ordinal int, surface, reviewState string, result *artifactSaveResult) {
	if occurrence == nil {
		return
	}
	key := entityIdentityIdempotencyKey("binding", p.source.Revision, artifactKind, role, fmt.Sprint(ordinal))
	result.trySave("SaveEntityIdentityArtifactBinding", func() error {
		return p.writer.SaveEntityIdentityArtifactBinding(ctx, &store.EntityIdentityArtifactBinding{
			BindingID:       entityIdentityStableID("binding", p.sid, key),
			StableEntityID:  occurrence.StableEntityID,
			ChatSessionID:   p.sid,
			ArtifactKind:    artifactKind,
			ArtifactRole:    role,
			ArtifactOrdinal: ordinal,
			SurfaceText:     surface,
			ReviewState:     reviewState,
			SourceContract:  p.source.ContractVersion,
			SourceRevision:  p.source.Revision,
			SourceTurn:      p.turnIndex,
			IdempotencyKey:  key,
			CreatedAt:       p.now,
		})
	}, result, func() { result.IdentityBindings++ })
}

func (p *entityIdentityProjection) bindKGTriple(ctx context.Context, triple map[string]any, ordinal int, result *artifactSaveResult) {
	for _, endpoint := range []struct {
		field string
		role  string
	}{
		{field: "subject", role: "subject"},
		{field: "object", role: "object"},
	} {
		surface := strings.TrimSpace(stringFromMap(triple, endpoint.field))
		occurrence, reviewState, ambiguous := p.resolveUnique(surface)
		if occurrence == nil && ambiguous {
			occurrence = p.persistUnknownOccurrence(ctx, surface, "unknown", "kg_triple_"+endpoint.role, ordinal, result)
			reviewState = "needs_review"
		}
		if occurrence == nil {
			continue
		}
		if ambiguous {
			reviewState = "needs_review"
		}
		p.persistArtifactBinding(ctx, occurrence, "kg_triple", endpoint.role, ordinal, surface, reviewState, result)
	}
}

func (p *entityIdentityProjection) bindCharacterState(ctx context.Context, surface string, ordinal int, result *artifactSaveResult) {
	occurrence, reviewState := p.ensureArtifactIdentity(ctx, surface, "character", "character_state", ordinal, result)
	if occurrence != nil {
		p.persistArtifactBinding(ctx, occurrence, "character_state", "owner", ordinal, surface, reviewState, result)
	}
}

func (p *entityIdentityProjection) persistSpeakerAttributions(ctx context.Context, extraction map[string]any, result *artifactSaveResult) {
	for index, raw := range sliceFromAny(extraction["speaker_attributions"]) {
		item := mapFromAny(raw)
		excerpt := strings.TrimSpace(stringFromMap(item, "evidence_excerpt"))
		if excerpt == "" {
			excerpt = strings.TrimSpace(stringFromMap(item, "source_excerpt"))
		}
		spanStart, spanEnd := nextExactSpanWithSeen(p.content, excerpt, p.speakerSeen)
		if excerpt == "" || spanStart < 0 {
			result.addSkipReason("speaker_attributions", "not_grounded_in_current_turn", map[string]any{"index": index, "excerpt": excerpt})
			continue
		}
		speakerName := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "speaker_name"), stringFromMap(item, "speaker")))
		occurrence, surfaceReviewState, ambiguous := p.resolveUnique(speakerName)
		state := strings.ToLower(strings.TrimSpace(stringFromMap(item, "attribution_state")))
		explicitlyAmbiguous := state == "ambiguous" || state == "unknown" || state == "tentative" || state == "needs_review"
		linkedBySource := occurrence != nil && !ambiguous && surfaceReviewState == "source_observed" &&
			p.surfaceAppearsIndependentlyWithinSourceSpan(speakerName, spanStart, spanEnd)
		if !linkedBySource || explicitlyAmbiguous {
			occurrence = p.persistUnknownOccurrence(ctx, speakerName, "speaker", "speaker_attribution", index, result)
		}
		attributionState := "linked"
		reviewState := "source_observed"
		if !linkedBySource || explicitlyAmbiguous || ambiguous {
			attributionState = "unknown"
			if speakerName != "" {
				attributionState = "tentative"
			}
			reviewState = "needs_review"
		}
		kind := strings.ToLower(strings.TrimSpace(stringFromMap(item, "attribution_kind")))
		switch kind {
		case "dialogue", "quoted_speech", "thought", "narration":
		default:
			kind = "unknown"
		}
		key := entityIdentityIdempotencyKey("speaker", p.source.Revision, fmt.Sprint(index), fmt.Sprint(spanStart), fmt.Sprintf("%x", sha256.Sum256([]byte(excerpt))))
		result.trySave("SaveSpeakerAttribution", func() error {
			return p.writer.SaveSpeakerAttribution(ctx, &store.SpeakerAttribution{
				AttributionID: entityIdentityStableID("speaker", p.sid, key), ChatSessionID: p.sid,
				SpeakerEntityID: occurrence.StableEntityID, IdentityNamespace: occurrence.Namespace,
				SourceRole: "latest_turn", AttributionKind: kind, AttributionState: attributionState,
				ReviewState: reviewState, Confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0), 0, 1),
				SourceContract: p.source.ContractVersion, SourceRevision: p.source.Revision,
				SourceLogicalTurn: p.source.LogicalTurnID, SourceMessageID: p.source.MessageID,
				SourceGeneration: p.source.GenerationID, SourceContentHash: p.source.ContentHash,
				SourceTurn: p.turnIndex, SourceSpanStart: spanStart, SourceSpanEnd: spanEnd,
				EvidenceExcerpt: excerpt, IdempotencyKey: key, CreatedAt: p.now, UpdatedAt: p.now,
			})
		}, result, func() { result.SpeakerAttributions++ })
	}
}

func normalizeSpeakerAttributionCandidates(raw any) []any {
	out := []any{}
	for _, value := range sliceFromAny(raw) {
		item := mapFromAny(value)
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "evidence_excerpt"),
			stringFromMap(item, "source_excerpt"),
		))
		if excerpt == "" {
			continue
		}
		out = append(out, map[string]any{
			"speaker_name":      strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "speaker_name"), stringFromMap(item, "speaker"))),
			"attribution_kind":  strings.ToLower(strings.TrimSpace(stringFromMap(item, "attribution_kind"))),
			"attribution_state": strings.ToLower(strings.TrimSpace(stringFromMap(item, "attribution_state"))),
			"confidence":        clampFloat(extractionFloatFromAny(item["confidence"], 0), 0, 1),
			"evidence_excerpt":  excerpt,
		})
	}
	return out
}

func entityIdentityStableID(namespace string, values ...string) string {
	sum := sha256.Sum256([]byte(namespace + "\x00" + strings.Join(values, "\x00")))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(b)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32]
}

func entityIdentityIdempotencyKey(values ...string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(values, "\x1f"))))
}
