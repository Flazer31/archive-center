package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Immutable, request-owned reading material. Facts, their IDs and stored rows
// remain independent. A minimum form is built before either Go or AI selection.
type prepareTurnMemoryPart struct {
	Key, Label, Value string
	FactTexts         []string
}
type prepareTurnMemoryContext struct {
	Path, Label string
	Parts       []prepareTurnMemoryPart
	fingerprint [32]byte
}

// Source preparation is serialized by the existing request owner. Cache only
// this immutable value's fingerprint, never any question or selection result.
func (c *prepareTurnMemoryContext) sourceFingerprint() [32]byte {
	if c == nil {
		return [32]byte{}
	}
	if c.fingerprint == ([32]byte{}) {
		b, _ := json.Marshal(c)
		c.fingerprint = sha256.Sum256(b)
	}
	return c.fingerprint
}

type prepareTurnMemoryFormPart struct {
	Key, Text string
	Refs      []string
}
type prepareTurnMemoryForm struct {
	Group, Heading, Text, Meaning string
	Parts                         []prepareTurnMemoryFormPart
	Refs                          []string
	Chars                         int
}

func prepareTurnMemoryPath(path []string) string {
	parts := make([]string, len(path))
	for i, part := range path {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(parts, "/")
}

// These labels retain explicit structural roles; they never infer a character,
// event or relationship from model prose. Unknown fields keep their own value.
func prepareTurnMemoryAnchor(field string) bool {
	switch strings.ToLower(field) {
	case "name", "character_name", "display_name", "canonical_name", "subject", "subject_entity", "actor", "actor_name", "item", "scope", "scope_name":
		return true
	}
	return false
}
func prepareTurnMemoryCondition(field string) bool {
	switch strings.ToLower(field) {
	case "condition", "conditions", "exception", "exceptions", "restriction", "restrictions", "when", "unless":
		return true
	}
	return false
}

func prepareTurnAttachStructuredContexts(prefix string, value any, path []string, facts []prepareTurnPriorityMemoryFact) {
	byPath := make(map[string]int, len(facts))
	for i := range facts {
		byPath[facts[i].SourcePath] = i
	}
	var visit func(any, []string, []prepareTurnMemoryPart)
	visit = func(value any, path []string, inherited []prepareTurnMemoryPart) {
		switch node := value.(type) {
		case []any:
			for i, child := range node {
				visit(child, append(append([]string{}, path...), strconv.Itoa(i)), inherited)
			}
		case map[string]any:
			keys := make([]string, 0, len(node))
			for key := range node {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			parts := map[string]prepareTurnMemoryPart{}
			qualifiers := []prepareTurnMemoryPart{}
			anchors := append([]prepareTurnMemoryPart(nil), inherited...)
			for _, key := range keys {
				p := prepareTurnMemoryPath(append(append([]string{}, path...), key))
				if i, ok := byPath[p]; ok {
					part := prepareTurnMemoryPart{Key: p, Label: key, Value: prepareTurnPriorityScalarText(node[key]), FactTexts: []string{facts[i].Text}}
					parts[key] = part
					if prepareTurnMemoryAnchor(key) {
						anchors = append(anchors, part)
					}
					if prepareTurnMemoryCondition(key) {
						qualifiers = append(qualifiers, part)
					}
				} else if prepareTurnMemoryCondition(key) {
					// An explicit condition can itself be an array or object. Keep
					// that value intact rather than dropping its nested qualifiers.
					part := prepareTurnMemoryPart{Key: p, Label: key, Value: mustCompactJSON(node[key])}
					for _, fact := range facts {
						if strings.HasPrefix(fact.SourcePath, p+"/") {
							part.FactTexts = append(part.FactTexts, fact.Text)
						}
					}
					qualifiers = append(qualifiers, part)
				}
			}
			for _, key := range keys {
				part, scalar := parts[key]
				if !scalar {
					if proposition, ok := parts["rule"]; ok && (prepareTurnMemoryCondition(key) || key == "evidence_excerpt") {
						// A nested qualifier is still part of this rule, not a new
						// independent record. Keep its complete explicit value.
						minimum := append([]prepareTurnMemoryPart(nil), anchors...)
						minimum = append(minimum, proposition)
						minimum = append(minimum, qualifiers...)
						p := prepareTurnMemoryPath(append(append([]string{}, path...), key))
						if key == "evidence_excerpt" {
							minimum = append(minimum, prepareTurnMemoryPart{Key: p, Label: key, Value: mustCompactJSON(node[key])})
						}
						reading := &prepareTurnMemoryContext{Path: prepareTurnMemoryPath(path), Label: prefix, Parts: minimum}
						for i := range facts {
							if strings.HasPrefix(facts[i].SourcePath, p+"/") {
								facts[i].Reading = reading
							}
						}
						continue
					}
					visit(node[key], append(append([]string{}, path...), key), anchors)
					continue
				}
				minimum := append([]prepareTurnMemoryPart(nil), anchors...)
				seen := map[string]bool{}
				for _, p := range minimum {
					seen[p.Key] = true
				}
				add := func(p prepareTurnMemoryPart) {
					if p.Key != "" && !seen[p.Key] {
						minimum = append(minimum, p)
						seen[p.Key] = true
					}
				}
				add(part)
				// A rule's scope, exception and quotation qualify this proposition;
				// none is a free-standing rule when selected on its own.
				if proposition, ok := parts["rule"]; ok {
					add(proposition)
					for _, qualifier := range qualifiers {
						add(qualifier)
					}
				}
				// A transfer's direction is explicit in its two named fields.
				if key == "giver" || key == "recipient" {
					add(parts["giver"])
					add(parts["recipient"])
				}
				if !prepareTurnMemoryAnchor(key) && key != "evidence_excerpt" && key != "description" {
					for _, qualifier := range qualifiers {
						add(qualifier)
					}
				}
				// Even a lone JSON field has an observed path/value. Retaining
				// those directly avoids repeating its display prefix inside a
				// source header when typed row scope is attached later.
				facts[byPath[part.Key]].Reading = &prepareTurnMemoryContext{Path: prepareTurnMemoryPath(path), Label: prefix, Parts: minimum}
			}
		}
	}
	visit(value, path, nil)
}

type prepareTurnMemoryReadingSource struct {
	Ref, Occurrence, Parent, Lane, Visibility, Owner, Viewers string
	Turn                                                      int
}

func prepareTurnMemorySourceKey(c prepareTurnPriorityMemoryCandidate) prepareTurnMemoryReadingSource {
	// Same source in another lane/holder remains a separate reading occurrence.
	return prepareTurnMemoryReadingSource{c.SourceRef, c.SourceOccurrence, c.ParentLineText, c.Lane, c.Visibility, c.PerspectiveOwner, strings.Join(c.AllowedViewers, "\x1f"), c.SourceTurn}
}

type prepareTurnMemoryGroupKey struct {
	Source prepareTurnMemoryReadingSource
	Path   string
}
type prepareTurnMemoryFormKey struct {
	Group      string
	Content    [32]byte
	References string
}

func prepareTurnBuildReadingForms(candidates []prepareTurnPriorityMemoryCandidate, relevance func(string) float64, preparation *prepareTurnRequestPreparation) {
	refs := map[prepareTurnMemoryReadingSource]map[string]string{}
	for _, c := range candidates {
		if c.Reading != nil {
			refs[prepareTurnMemorySourceKey(c)] = nil
		}
	}
	if len(refs) == 0 {
		return
	}
	groups := map[prepareTurnMemoryGroupKey]string{}
	forms := map[prepareTurnMemoryFormKey]*prepareTurnMemoryForm{}
	if preparation != nil {
		groups, forms = preparation.readingGroups, preparation.readingForms
	}
	for _, c := range candidates {
		key := prepareTurnMemorySourceKey(c)
		if _, needed := refs[key]; !needed {
			continue
		}
		if refs[key] == nil {
			refs[key] = map[string]string{}
		}
		refs[key][c.CompleteText] = c.CanonicalFactID
	}
	for i := range candidates {
		c := &candidates[i]
		if c.Reading == nil {
			continue
		}
		key := prepareTurnMemorySourceKey(*c)
		group := prepareTurnMemoryGroupKey{key, c.Reading.Path}
		groupID := groups[group]
		if groupID == "" {
			b, _ := json.Marshal(group)
			groupID = fmt.Sprintf("%x", sha256.Sum256(b))
			groups[group] = groupID
		}
		// Contents and provenance are immutable, but a discovery may add a
		// previously missing reference. Include the ordered bindings in the key.
		var bindings strings.Builder
		for _, part := range c.Reading.Parts {
			for _, text := range part.FactTexts {
				bindings.WriteString(refs[key][text])
				bindings.WriteByte('\x1f')
			}
		}
		formKey := prepareTurnMemoryFormKey{groupID, c.Reading.sourceFingerprint(), bindings.String()}
		form := forms[formKey]
		if form == nil {
			form = &prepareTurnMemoryForm{Group: groupID}
			heading := strings.TrimSpace(c.Reading.Label)
			if c.Reading.Path != "" && c.Reading.Path != "/" {
				heading += " [" + c.Reading.Path + "]"
			}
			form.Heading = strings.TrimSpace(heading)
			values, lines := []string{}, []string{}
			seenRefs := map[string]bool{}
			for _, p := range c.Reading.Parts {
				text := p.Value
				if p.Label != "" {
					text = p.Label + ": " + text
				}
				part := prepareTurnMemoryFormPart{Key: p.Key, Text: text}
				for _, factText := range p.FactTexts {
					if ref := refs[key][factText]; ref != "" {
						part.Refs = append(part.Refs, ref)
						if !seenRefs[ref] {
							seenRefs[ref] = true
							form.Refs = append(form.Refs, ref)
						}
					}
				}
				form.Parts = append(form.Parts, part)
				values = append(values, p.Value)
				lines = append(lines, "  "+text)
			}
			form.Meaning = strings.Join(values, "\n")
			form.Text = strings.TrimSpace(form.Heading + "\n" + strings.Join(lines, "\n"))
			form.Chars = utf8.RuneCountInString(form.Text)
			forms[formKey] = form
		}
		c.Minimum = form
		c.ContextRelevance = relevance(form.Meaning)
		c.Relevance = math.Max(c.OriginalRelevance, c.ContextRelevance)
		c.FinalScore = prepareTurnPriorityScore(c.Relevance, c.Importance, c.Recency, c.ContinuityBonus, c.StructuredBias)
	}
	groupScores := map[string]float64{}
	for _, c := range candidates {
		if c.Minimum != nil {
			groupScores[c.Minimum.Group] = math.Max(groupScores[c.Minimum.Group], c.FinalScore)
		}
	}
	for i := range candidates {
		if candidates[i].Minimum != nil {
			candidates[i].ContextGroupScore = groupScores[candidates[i].Minimum.Group]
		}
	}
}

func prepareTurnMemoryReadingText(c prepareTurnPriorityMemoryCandidate) string {
	if c.Minimum != nil {
		return c.Minimum.Text
	}
	return c.CompleteText
}

// A manual restoration has its own recorded revision but retains the restored
// state's original observation and proof, including an explicitly unknown turn.
func prepareTurnCurrentStateReadingOrigin(current store.StatusCurrentValue) (store.StatusCurrentValue, map[string]any) {
	evidence := parseJSONMap(current.EvidenceJSON)
	if turn, restored := evidence["restored_source_turn"]; restored {
		current.SourceTurn = intFromAny(turn, 0)
		original := map[string]any{}
		for key, value := range mapFromAny(evidence["restored_evidence"]) {
			original[key] = value
		}
		original["repair_source_revision"] = evidence["source_revision"]
		original["repair_recorded_turn"] = evidence["repair_recorded_turn"]
		evidence = original
	}
	return current, evidence
}

// A recalled historical promise keeps its own identity and source time. Its
// current lifecycle is source-linked reading material, independent of whether
// the completion Memory also won an ordinary vector recall slot.
func prepareTurnLifecycleReadings(values []store.StatusCurrentValue, clocks ...map[string]any) map[string][]prepareTurnMemoryPart {
	out := map[string][]prepareTurnMemoryPart{}
	var clock map[string]any
	if len(clocks) > 0 {
		clock = clocks[0]
	}
	for _, view := range narrativeCurrentStateViews(values) {
		// Preserve the existing narrative current-state public projection boundary.
		switch view.Scope {
		case "belief", "rumor", "secret":
			continue
		}
		key := normalizeNarrativeLifecycleKey(stringFromMap(view.Payload, "lifecycle_key"))
		if key == "" {
			continue
		}
		prefix := fmt.Sprintf("@lifecycle/%s/%s/%s", key, view.Value.OwnerScope, view.Value.OwnerID)
		origin, evidence := prepareTurnCurrentStateReadingOrigin(view.Value)
		observation := "source turn unknown"
		if origin.SourceTurn > 0 {
			observation = fmt.Sprintf("source turn %d", origin.SourceTurn)
		}
		label := fmt.Sprintf("current progression [lifecycle %s; %s; status_current_values:%d]", key, observation, view.Value.ID)
		text := view.Current
		if view.Subject != "" {
			text = view.Subject + ": " + text
		}
		if transition := stringFromMap(view.Payload, "transition"); transition != "" {
			text += " (" + transition + ")"
		}
		parts := []prepareTurnMemoryPart{{Key: prefix, Label: label, Value: text}}
		if details := mapFromAny(view.Payload["lifecycle_details"]); len(details) > 0 {
			parts = append(parts, prepareTurnMemoryPart{Key: prefix + "/details", Label: "progression details", Value: prepareTurnPriorityScalarText(details)})
			scheduleSource := make(map[string]any, len(details)+1)
			for key, value := range details {
				scheduleSource[key] = value
			}
			scheduleSource["lifecycle_transition"] = stringFromMap(view.Payload, "transition")
			if schedule := buildCommitmentScheduleReading(scheduleSource, clock); len(schedule) > 0 {
				parts = append(parts, prepareTurnMemoryPart{Key: prefix + "/schedule", Label: "schedule reading (read only)", Value: mustCompactJSON(schedule)})
			}
		}
		if excerpt := stringFromMap(evidence, "evidence_excerpt"); excerpt != "" {
			parts = append(parts, prepareTurnMemoryPart{Key: prefix + "/evidence", Label: "current progression evidence", Value: excerpt})
		}
		refs := []string{}
		if source := stringFromMap(evidence, "source"); source != "" {
			refs = append(refs, source)
		}
		if revision := stringFromMap(evidence, "source_revision"); revision != "" {
			refs = append(refs, "source revision "+revision)
		}
		if ids := sliceFromAny(evidence["direct_evidence_ids"]); len(ids) > 0 {
			refs = append(refs, "direct evidence "+mustCompactJSON(ids))
		}
		if len(refs) > 0 {
			parts = append(parts, prepareTurnMemoryPart{Key: prefix + "/source", Label: "progression source", Value: strings.Join(refs, "; ")})
		}
		for _, name := range []string{"repair_source_revision", "repair_recorded_turn"} {
			if raw := evidence[name]; raw != nil {
				parts = append(parts, prepareTurnMemoryPart{Key: prefix + "/" + name, Label: "restoration audit " + name, Value: prepareTurnPriorityScalarText(raw)})
			}
		}
		out[key] = append(out[key], parts...)
	}
	return out
}

func prepareTurnAttachLifecycleContext(out *prepareTurnInjectionAssembly, values []store.StatusCurrentValue, clocks ...map[string]any) {
	readings := prepareTurnLifecycleReadings(values, clocks...)
	for i := range out.PriorityFactSeeds {
		fact := &out.PriorityFactSeeds[i].Fact
		parts := readings[normalizeNarrativeLifecycleKey(fact.LifecycleKey)]
		if len(parts) == 0 {
			continue
		}
		reading := prepareTurnMemoryContext{Path: fact.SourcePath, Parts: []prepareTurnMemoryPart{{Key: fact.SourcePath, Value: fact.Text, FactTexts: []string{fact.Text}}}}
		if fact.Reading != nil {
			reading = *fact.Reading
			reading.Parts = append([]prepareTurnMemoryPart(nil), fact.Reading.Parts...)
		}
		reading.fingerprint = [32]byte{}
		reading.Parts = append(reading.Parts, parts...)
		fact.Reading = &reading
	}
}

func prepareTurnSourceTemporalContext(base, item map[string]any) map[string]any {
	var out map[string]any
	for _, source := range []map[string]any{base, mapFromAny(item["temporal_context"]), item} {
		for _, key := range []string{"observed_at", "occurrence_time", "relative_expression", "relative"} {
			if value, exists := source[key]; exists {
				if out == nil {
					out = map[string]any{}
				}
				out[key] = value
			}
		}
	}
	return out
}

// Interpret only source-linked metadata already admitted to this request. The
// helper neither reads additional records nor rewrites the historical text.
func prepareTurnAttachTemporalContext(out *prepareTurnInjectionAssembly, clock map[string]any) {
	for i := range out.PriorityFactSeeds {
		fact := &out.PriorityFactSeeds[i].Fact
		if len(fact.TemporalContext) == 0 {
			continue
		}
		reading := prepareTurnMemoryContext{Path: fact.SourcePath, Parts: []prepareTurnMemoryPart{{Key: fact.SourcePath, Value: fact.Text, FactTexts: []string{fact.Text}}}}
		if fact.Reading != nil {
			reading = *fact.Reading
			reading.Parts = append([]prepareTurnMemoryPart(nil), fact.Reading.Parts...)
		}
		reading.fingerprint = [32]byte{}
		value := mustCompactJSON(buildStoryTimeReading(fact.TemporalContext, clock))
		part := prepareTurnMemoryPart{Key: fmt.Sprintf("@temporal/%x", sha256.Sum256([]byte(value))), Label: "source-relative time (last confirmed clock; read only)", Value: value}
		reading.Parts = append(reading.Parts, part)
		fact.Reading = &reading
	}
}

func prepareTurnAttachLastConfirmedClock(out *prepareTurnInjectionAssembly, clock map[string]any) {
	projection := storyClockPromptProjection(clock)
	hasTime := false
	for _, key := range []string{"absolute", "partial", "relative", "range", "sequence", "calendar"} {
		if len(mapFromAny(projection[key])) > 0 {
			hasTime = true
			break
		}
	}
	if !hasTime {
		return
	}
	line := "- Last confirmed stored story clock (last accepted narration): " + mustCompactJSON(projection)
	out.ContinuityCorrectionText = strings.TrimSpace(out.ContinuityCorrectionText + "\n" + line)
}

func prepareTurnCharacterFieldPath(path string) string {
	if path == "/state" || strings.HasPrefix(path, "/state/") {
		return "/status" + strings.TrimPrefix(path, "/state")
	}
	return path
}

// Explicit Critic source_fields connect an older field observation to current
// evidence. No relationship between two prose strings is inferred here.
func prepareTurnCharacterFieldCurrentReadings(narrative, reversible []store.StatusCurrentValue, storyClock map[string]any) map[string][]prepareTurnMemoryPart {
	out := map[string][]prepareTurnMemoryPart{}
	appendReading := func(subject string, fields []string, value, transition string, current store.StatusCurrentValue, evidence map[string]any) {
		for _, field := range fields {
			field = prepareTurnCharacterFieldPath(field)
			key := normalizePrepareTurnEntityNeedle(subject) + "\x1f" + field
			prefix := fmt.Sprintf("@field_current/%s/%d", field, current.ID)
			observation := "source turn unknown"
			if current.SourceTurn > 0 {
				observation = fmt.Sprintf("source turn %d", current.SourceTurn)
			}
			parts := []prepareTurnMemoryPart{{Key: prefix, Label: fmt.Sprintf("linked current state [%s; status_current_values:%d]", observation, current.ID), Value: value}}
			if transition != "" {
				parts = append(parts, prepareTurnMemoryPart{Key: prefix + "/transition", Label: "current transition", Value: transition})
			}
			if excerpt := stringFromMap(evidence, "evidence_excerpt"); excerpt != "" {
				parts = append(parts, prepareTurnMemoryPart{Key: prefix + "/evidence", Label: "current state evidence", Value: excerpt})
			}
			for _, name := range []string{"source_revision", "direct_evidence_ids", "observed_at", "occurrence_time", "effective_time", "validity", "repair_source_revision", "repair_recorded_turn"} {
				if raw, exists := evidence[name]; exists && raw != nil {
					parts = append(parts, prepareTurnMemoryPart{Key: prefix + "/" + name, Label: "current " + name, Value: prepareTurnPriorityScalarText(raw)})
				}
			}
			out[key] = append(out[key], parts...)
		}
	}
	for _, view := range narrativeCurrentStateViews(narrative) {
		switch view.Scope {
		case "belief", "rumor", "secret":
			continue
		}
		origin, evidence := prepareTurnCurrentStateReadingOrigin(view.Value)
		for _, key := range []string{"observed_at", "occurrence_time", "effective_time", "validity"} {
			if value, exists := view.Payload[key]; exists {
				evidence[key] = value
			}
		}
		appendReading(view.Subject, stringsFromAny(view.Payload["source_fields"]), view.Current, stringFromMap(view.Payload, "transition"), origin, evidence)
	}
	subjectSlotCounts := reversibleSubjectSlotCounts(reversible)
	for _, current := range reversible {
		projection := parseJSONMap(current.ValueJSON)
		if stringFromMap(projection, "version") != reversibleStateContractVersion {
			continue
		}
		subject := stringFromMap(projection, "subject_label")
		current, evidence := prepareTurnCurrentStateReadingOrigin(current)
		appendSlot := func(slot map[string]any, transition string, source map[string]any) {
			// Reuse the existing public reversible-state delivery boundary.
			if reversiblePublicDeliveryExclusion(slot, storyClock) != "" {
				return
			}
			text := stringFromMap(mapFromAny(slot["value"]), "text")
			if transition == "clear" || transition == "recover" {
				text = "The referenced former condition is no longer current (" + transition + ")."
			}
			observation := current
			observation.SourceTurn = intFromAny(source["source_turn"], 0)
			appendReading(subject, stringsFromAny(slot["source_fields"]), text, transition, observation, source)
		}
		slots := mapFromAny(projection["slots"])
		keys := make([]string, 0, len(slots))
		for key := range slots {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			subjectSlotKey := strings.Join([]string{stringFromMap(projection, "domain"), normalizePrepareTurnEntityNeedle(subject), key}, "\x1f")
			if subjectSlotCounts[subjectSlotKey] > 1 {
				continue
			}
			slot := mapFromAny(slots[key])
			source := mapFromAny(slot["source"])
			appendSlot(slot, "", source)
		}
		history := mapFromAny(evidence["history_observation"])
		transition := stringFromMap(history, "transition")
		if transition == "clear" || transition == "recover" {
			appendSlot(history, transition, evidence)
		}
	}
	return out
}

func prepareTurnAttachCharacterFieldContext(out *prepareTurnInjectionAssembly, start int, state store.CharacterState, current map[string][]prepareTurnMemoryPart) {
	fields := store.DecodeCharacterFieldProvenance(state.FieldProvenanceJSON)
	for i := start; i < len(out.PriorityFactSeeds); i++ {
		seed := &out.PriorityFactSeeds[i]
		path := prepareTurnCharacterFieldPath(seed.Fact.SourceFieldPath)
		if path == "" {
			path = prepareTurnCharacterFieldPath(seed.Fact.SourcePath)
		}
		provenance := store.CharacterFieldProvenanceForPath(fields, path)
		seed.Fact.TemporalContext = prepareTurnSourceTemporalContext(nil, provenance)
		seed.FieldObservationTurn = intFromAny(provenance["source_turn"], 0)
		seed.ProjectionSource += ":field_provenance"
		reading := prepareTurnMemoryContext{Path: seed.Fact.SourcePath, Parts: []prepareTurnMemoryPart{{Key: seed.Fact.SourcePath, Value: seed.Fact.Text, FactTexts: []string{seed.Fact.Text}}}}
		if seed.Fact.Reading != nil {
			reading = *seed.Fact.Reading
			reading.Parts = append([]prepareTurnMemoryPart(nil), seed.Fact.Reading.Parts...)
		}
		reading.fingerprint = [32]byte{}
		observation := "unknown"
		if seed.FieldObservationTurn > 0 {
			observation = fmt.Sprintf("source turn %d", seed.FieldObservationTurn)
		}
		reading.Parts = append(reading.Parts, prepareTurnMemoryPart{Key: "@field_observation/" + path, Label: "field observation " + path, Value: observation}, prepareTurnMemoryPart{Key: "@field_snapshot/" + path, Label: "containing snapshot", Value: fmt.Sprintf("turn %d (does not date this field)", state.TurnIndex)})
		for _, name := range []string{"source_session_id", "source_revision", "recorded_turn", "evidence_excerpt", "evidence_refs", "direct_evidence_ids", "occurrence_time", "learned_time"} {
			if value, exists := provenance[name]; exists && value != nil {
				reading.Parts = append(reading.Parts, prepareTurnMemoryPart{Key: "@field/" + path + "/" + name, Label: name, Value: prepareTurnPriorityScalarText(value)})
			}
		}
		effective := "unknown"
		if value, exists := provenance["effective_time"]; exists && value != nil {
			effective = prepareTurnPriorityScalarText(value)
		}
		reading.Parts = append(reading.Parts, prepareTurnMemoryPart{Key: "@field/" + path + "/effective", Label: "effective time", Value: effective})
		for linkedPath := path; linkedPath != ""; {
			if parts := current[normalizePrepareTurnEntityNeedle(state.CharacterName)+"\x1f"+linkedPath]; len(parts) > 0 {
				reading.Parts = append(reading.Parts, prepareTurnMemoryPart{Key: "@field/" + path + "/historical", Label: "field use", Value: "Historical observation; use the linked current state and evidence for present continuity."})
				reading.Parts = append(reading.Parts, parts...)
				break
			}
			at := strings.LastIndex(linkedPath, "/")
			if at <= 0 {
				break
			}
			linkedPath = linkedPath[:at]
		}
		seed.Fact.Reading = &reading
	}
}

// Keep retrieval fragments and IDs, but read a typed relation (including an
// abbreviation such as "No. 42") or a full-summary projection as one assertion.
func prepareTurnAttachWholeSourceContext(facts []prepareTurnPriorityMemoryFact, path, text string) []prepareTurnPriorityMemoryFact {
	copyFacts := append([]prepareTurnPriorityMemoryFact(nil), facts...)
	part := prepareTurnMemoryPart{Key: path, Value: text}
	for _, f := range facts {
		part.FactTexts = append(part.FactTexts, f.Text)
	}
	reading := &prepareTurnMemoryContext{Path: path, Parts: []prepareTurnMemoryPart{part}}
	for i := range copyFacts {
		copyFacts[i].Reading = reading
	}
	return copyFacts
}

func prepareTurnAttachRecollectionContext(out *prepareTurnInjectionAssembly, start int, text, evidence string) {
	if start >= len(out.PriorityFactSeeds) {
		return
	}
	seeds := out.PriorityFactSeeds[start:]
	memberTexts := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		memberTexts = append(memberTexts, seed.Fact.Text)
	}
	part := prepareTurnMemoryPart{Key: "/memory_text", Label: "recollection", Value: text, FactTexts: memberTexts}
	reading := &prepareTurnMemoryContext{Path: "/", Label: "Personal recollection", Parts: []prepareTurnMemoryPart{part}}
	for i := range seeds {
		seeds[i].Fact.Reading = reading
		seeds[i].Fact.SourcePath = "/memory_text"
	}
	if evidence = strings.TrimSpace(evidence); evidence != "" {
		seed := seeds[0]
		value := "evidence_excerpt: " + evidence
		seed.Fact = prepareTurnPriorityMemoryFact{Text: value, FamilyKey: collapseTextKey(value), ValueKey: collapseTextKey(evidence), SourcePath: "/evidence_excerpt",
			Reading: &prepareTurnMemoryContext{Path: reading.Path, Label: reading.Label, Parts: []prepareTurnMemoryPart{part, {Key: "/evidence_excerpt", Label: "evidence_excerpt", Value: evidence, FactTexts: []string{value}}}}}
		out.PriorityFactSeeds = append(out.PriorityFactSeeds, seed)
	}
	for i := start; i < len(out.PriorityFactSeeds); i++ {
		out.PriorityFactSeeds[i].SourceFactCount = len(out.PriorityFactSeeds) - start
	}
}

func prepareTurnAttachWorldScope(out *prepareTurnInjectionAssembly, start int, label, scope, name string) {
	for i := start; i < len(out.PriorityFactSeeds); i++ {
		fact := &out.PriorityFactSeeds[i].Fact
		reading := fact.Reading
		if reading == nil {
			reading = &prepareTurnMemoryContext{Path: fact.SourcePath, Label: label, Parts: []prepareTurnMemoryPart{{Key: fact.SourcePath, Value: fact.Text, FactTexts: []string{fact.Text}}}}
			// A lone, opaque fact keeps its original text. Scope is additive metadata.
		}
		copyReading := *reading
		copyReading.fingerprint = [32]byte{}
		copyReading.Parts = append([]prepareTurnMemoryPart(nil), reading.Parts...)
		for _, p := range []prepareTurnMemoryPart{{Key: "@source_scope", Label: "scope", Value: scope}, {Key: "@source_scope_name", Label: "scope_name", Value: name}} {
			if p.Value == "" {
				continue
			}
			same := false
			for _, old := range copyReading.Parts {
				if old.Value == p.Value {
					same = true
					break
				}
			}
			if !same {
				copyReading.Parts = append(copyReading.Parts, p)
			}
		}
		if len(copyReading.Parts) > 1 {
			fact.Reading = &copyReading
		}
	}
}

func prepareTurnMemoryModelCandidate(c prepareTurnPriorityMemoryCandidate, refs map[string]string) map[string]any {
	item := map[string]any{"ref": refs[c.CanonicalFactID], "id": c.CanonicalFactID, "source_ref": c.SourceRef, "source_table": c.SourceTable, "text": prepareTurnMemoryReadingText(c), "source_turn": c.SourceTurn, "visibility": c.Visibility, "perspective_owner": c.PerspectiveOwner, "allowed_viewers": c.AllowedViewers}
	if c.Minimum != nil {
		contextRefs := []string{}
		for _, id := range c.Minimum.Refs {
			if ref := refs[id]; ref != "" {
				contextRefs = append(contextRefs, ref)
			}
		}
		item["context_refs"] = contextRefs
		item["minimum_chars"] = utf8.RuneCountInString("- " + prepareTurnMemorySourceHeading(c) + " " + c.Minimum.Text)
	}
	return item
}

func prepareTurnMemorySourceHeading(c prepareTurnPriorityMemoryCandidate) string {
	parts := []string{}
	if c.SourceTable == "character_states" && strings.HasSuffix(c.ProjectionSource, ":field_provenance") && c.SourceTurn > 0 {
		parts = append(parts, fmt.Sprintf("field observation turn %d", c.SourceTurn))
	} else if c.SourceTable == "character_states" && c.SourceTurn > 0 {
		parts = append(parts, fmt.Sprintf("state snapshot turn %d; fields may be older", c.SourceTurn))
	} else if c.SourceTurn > 0 {
		parts = append(parts, fmt.Sprintf("source turn %d", c.SourceTurn))
	}
	parts = append(parts, c.SourceRef)
	if c.PerspectiveOwner != "" {
		parts = append(parts, "owner "+c.PerspectiveOwner)
	}
	if c.Visibility != "" && c.Visibility != "general" && c.Visibility != "public_projection" {
		parts = append(parts, "visibility "+c.Visibility)
	}
	if len(c.AllowedViewers) > 0 && !(len(c.AllowedViewers) == 1 && c.AllowedViewers[0] == c.PerspectiveOwner) {
		parts = append(parts, "viewers "+strings.Join(c.AllowedViewers, ", "))
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

type prepareTurnMemoryReadingRow struct {
	FactID                              string
	Order                               int
	Group, Header, Ref, Plain, rendered string
	Parts                               []prepareTurnMemoryFormPart
	partRefs                            map[prepareTurnMemoryPartIdentity]string
}
type prepareTurnMemoryPartIdentity struct {
	Key, Text string
}

type prepareTurnMemoryReadingLayout struct {
	rows   []*prepareTurnMemoryReadingRow
	groups map[string]*prepareTurnMemoryReadingRow
}
type prepareTurnMemoryReadingEdit struct {
	delta    int
	row      prepareTurnMemoryReadingRow
	previous *prepareTurnMemoryReadingRow
}

// Reuse source-scoped constituents in the group's first visible reading. A later
// selection adds only its new fields there, even across intervening memories.
// Preview is side-effect free: rejected budget reservations cannot consume text.
func (l *prepareTurnMemoryReadingLayout) preview(row prepareTurnMemoryReadingRow) prepareTurnMemoryReadingEdit {
	previous := l.groups[row.Group]
	if row.Group == "" {
		previous = nil
		row.rendered = strings.TrimSpace(row.Plain)
	} else {
		incoming, ref := row.Parts, row.Ref
		row.Parts = nil
		row.partRefs = map[prepareTurnMemoryPartIdentity]string{}
		seen := map[prepareTurnMemoryPartIdentity]bool{}
		if previous != nil {
			row.Order = minInt(row.Order, previous.Order)
			row.Header = previous.Header
			row.Parts = append(row.Parts, previous.Parts...)
			for _, part := range previous.Parts {
				seen[prepareTurnMemoryPartIdentity{part.Key, part.Text}] = true
			}
			for key, value := range previous.partRefs {
				row.partRefs[key] = value
			}
		}
		for _, part := range incoming {
			key := prepareTurnMemoryPartIdentity{part.Key, part.Text}
			if !seen[key] {
				row.Parts = append(row.Parts, part)
				seen[key] = true
				if ref != "" {
					row.partRefs[key], ref = ref, ""
				}
			}
		}
		lines := []string{}
		for _, part := range row.Parts {
			text := part.Text
			if ref := row.partRefs[prepareTurnMemoryPartIdentity{part.Key, part.Text}]; ref != "" {
				text = "[" + ref + "] " + text
			}
			lines = append(lines, "  "+text)
		}
		row.rendered = ""
		if len(lines) > 0 {
			row.rendered = strings.TrimSpace("- " + row.Header + "\n" + strings.Join(lines, "\n"))
		}
	}
	cost := func(text string) int {
		if text == "" {
			return 0
		}
		return 1 + utf8.RuneCountInString(text)
	}
	delta := cost(row.rendered)
	if previous != nil {
		delta -= cost(previous.rendered)
	}
	return prepareTurnMemoryReadingEdit{delta: delta, row: row, previous: previous}
}
func (l *prepareTurnMemoryReadingLayout) apply(edit prepareTurnMemoryReadingEdit) string {
	row := &edit.row
	if edit.previous != nil {
		oldOrder := edit.previous.Order
		*edit.previous = edit.row
		row = edit.previous
		if row.Order != oldOrder {
			// Only an earlier borrowed selection changes group placement.
			sort.SliceStable(l.rows, func(i, j int) bool { return l.rows[i].Order < l.rows[j].Order })
		}
	} else {
		pos := sort.Search(len(l.rows), func(i int) bool { return l.rows[i].Order > row.Order })
		l.rows = append(l.rows, nil)
		copy(l.rows[pos+1:], l.rows[pos:])
		l.rows[pos] = row
	}
	if row.Group != "" {
		if l.groups == nil {
			l.groups = map[string]*prepareTurnMemoryReadingRow{}
		}
		l.groups[row.Group] = row
	}
	return row.rendered
}
func (l *prepareTurnMemoryReadingLayout) texts() []string {
	lines := make([]string, 0, len(l.rows))
	for _, row := range l.rows {
		if row.rendered != "" {
			lines = append(lines, row.rendered)
		}
	}
	return lines
}
func prepareTurnMemoryCandidateRow(c prepareTurnPriorityMemoryCandidate, order int, ref string) prepareTurnMemoryReadingRow {
	row := prepareTurnMemoryReadingRow{Order: order, FactID: c.CanonicalFactID, Plain: "- " + c.RenderedText}
	if c.Minimum != nil {
		row.Group, row.Header, row.Parts, row.Ref = c.Minimum.Group, strings.TrimSpace(prepareTurnMemorySourceHeading(c)+" "+c.Minimum.Heading), c.Minimum.Parts, ref
	}
	return row
}
