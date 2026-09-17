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
	if c.SourceTable == "character_states" && c.SourceTurn > 0 {
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
