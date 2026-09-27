package httpapi

import (
	"encoding/json"
	"io"
	"slices"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const memoryRelationReadingVersion = "memory_relation_reading.v1"

// This is an internal reading of supplied rows, not an admission, visibility,
// retrieval or delivery decision. Production readers consume it after the existing
// source, session, lifecycle and visibility owners have supplied their rows.
type memoryRelationInput struct {
	PreciseUnits  []store.PreciseMemoryUnit
	CurrentStates []store.StatusCurrentValue
	StateHistory  []store.StatusChangeEvent
	Threads       []store.PendingThread
	Triples       []store.KGTriple
}

type memoryRelationReading struct {
	Version string
	Records []memoryRelationRecord
}

type memoryRelationRef struct {
	SessionID    string
	Table        string
	RowID        int64
	UnitID       string
	InputOrdinal int // Request-local fallback for a row without a retained ID.
}

type memoryRelationNode struct {
	Kind     string
	EntityID string
	Text     string
	// Identity is (Record.SessionID, EntityID) when EntityID is supplied;
	// otherwise it is (Record, Path). Text never establishes identity.
	Record memoryRelationRef
	Path   string // A label-only endpoint is local to this source field.
}

type memoryRelationLink struct {
	Kind      string
	Role      string
	Predicate string // Unmodified domain/predicate; never a retrieval policy.
	From      memoryRelationNode
	To        memoryRelationNode
}

type memoryRelationDocument struct {
	Path    string
	RawJSON string
	Value   any
	Parsed  bool
}

type memoryRelationFrame struct {
	Path       string
	Kind       string
	Fields     map[string]any
	Provenance map[string]any
	Time       map[string]any
	Knowledge  map[string]any
	Lifecycle  map[string]any
	Links      []memoryRelationLink
}

type memoryRelationRecord struct {
	Ref       memoryRelationRef
	SourceRow any // Detached typed row; retains fields not understood by this reader.
	Documents []memoryRelationDocument
	Frames    []memoryRelationFrame
}

// readMemoryRelations performs no IO and does not merge, sort, filter or expand
// records. A malformed/unknown JSON document still has its exact raw value and
// original row. Missing metadata is not promoted to public, current or certain.
func readMemoryRelations(input memoryRelationInput) memoryRelationReading {
	out := memoryRelationReading{Version: memoryRelationReadingVersion}
	if count := len(input.PreciseUnits) + len(input.CurrentStates) + len(input.StateHistory) + len(input.Threads) + len(input.Triples); count > 0 {
		out.Records = make([]memoryRelationRecord, 0, count)
	}
	for i, row := range input.PreciseUnits {
		// The only slice-backed row fields are transient vector material. Detach
		// them as well so this read model cannot mutate the supplied source.
		row.VectorEmbedding = slices.Clone(row.VectorEmbedding)
		row.VectorContextChunks = slices.Clone(row.VectorContextChunks)
		r := newMemoryRelationRecord("precise_memory_units", row.ChatSessionID, row.ID, row.UnitID, i, row)
		f := r.frame("payload_json", row.PayloadJSON, row.Kind)
		r.document("direct_evidence_ids_json", row.DirectEvidenceIDsJSON)
		f.Provenance["unit"] = map[string]any{
			"source_contract": row.SourceContract, "source_revision": row.SourceRevision,
			"source_turn_start": row.SourceTurnStart, "source_turn_end": row.SourceTurnEnd,
			"logical_turn_id": row.SourceLogicalTurnID, "message_id": row.SourceMessageID,
			"generation_id": row.SourceGenerationID, "content_hash": row.SourceContentHash,
			"span_start": row.SourceSpanStart, "span_end": row.SourceSpanEnd,
			"evidence_excerpt": row.EvidenceExcerpt, "evidence_hash": row.EvidenceHash,
			"root_evidence_id": row.RootEvidenceID, "direct_evidence_ids_json": row.DirectEvidenceIDsJSON,
		}
		f.Knowledge["unit"] = map[string]any{
			"truth_scope": row.TruthScope, "epistemic_mode": row.EpistemicMode,
			"authority_class": row.AuthorityClass, "visibility": row.Visibility,
			"knowledge_holder_entity_id": row.KnowledgeHolderEntityID,
			"reveal_condition":           row.RevealCondition, "confidence": row.Confidence,
			"admission_state": row.AdmissionState, "review_state": row.ReviewState,
		}
		f.Lifecycle["lifecycle_state"] = row.LifecycleState
		for _, role := range []struct{ name, id string }{
			{"actor", row.ActorEntityID}, {"subject", row.SubjectEntityID},
			{"affected", row.AffectedEntityID}, {"location", row.LocationEntityID},
			{"object", row.ObjectEntityID},
		} {
			if role.id != "" {
				f.Links = append(f.Links, memoryRelationLink{
					Kind: "record_role", Role: role.name,
					From: r.endpoint(role.name+"_entity_id", role.id, ""), To: r.node(f.Path, f.Kind),
				})
			}
		}
		if row.KnowledgeHolderEntityID != "" {
			f.Links = append(f.Links, memoryRelationLink{Kind: "knowledge_held_by",
				From: r.node(f.Path, f.Kind), To: r.endpoint("knowledge_holder_entity_id", row.KnowledgeHolderEntityID, "")})
		}
		// Only the existing versioned observation assigns a relationship meaning
		// to actor -> affected. Generic co-occurrence does not establish one.
		if f.Fields["contract_version"] == relationshipObservationContract {
			f.Links = append(f.Links, memoryRelationLink{Kind: "relationship_observation", Predicate: relationString(f.Fields, "domain"),
				From: r.endpoint("actor_entity_id", row.ActorEntityID, relationString(f.Fields, "source_entity")),
				To:   r.endpoint("affected_entity_id", row.AffectedEntityID, relationString(f.Fields, "target_entity"))})
		}
		r.Frames = append(r.Frames, f)
		out.Records = append(out.Records, r)
	}
	for i, row := range input.CurrentStates {
		r := newMemoryRelationRecord("status_current_values", row.ChatSessionID, row.ID, "", i, row)
		f := r.stateFrame("value_json", row.ValueJSON, row.StatusKey)
		f.Provenance["evidence"] = r.document("evidence_json", row.EvidenceJSON).Value
		f.Lifecycle["write_state"] = row.WriteState
		r.Frames = append(r.Frames, f)
		out.Records = append(out.Records, r)
	}
	for i, row := range input.StateHistory {
		r := newMemoryRelationRecord("status_change_events", row.ChatSessionID, row.ID, "", i, row)
		previous := r.stateFrame("previous_value_json", row.PreviousValueJSON, row.StatusKey)
		next := r.stateFrame("new_value_json", row.NewValueJSON, row.StatusKey)
		// Transition evidence and its observed clock do not become evidence or
		// occurrence time of the previous value. Keep them at record level.
		r.document("evidence_json", row.EvidenceJSON)
		r.document("story_clock_json", row.StoryClockJSON)
		r.Frames = append(r.Frames, previous, next)
		out.Records = append(out.Records, r)
	}
	for i, row := range input.Threads {
		r := newMemoryRelationRecord("pending_threads", row.ChatSessionID, row.ID, "", i, row)
		f := r.frame("details_json", row.DetailsJSON, "commitment")
		r.document("hook_metadata_json", row.HookMetadataJSON)
		f.Lifecycle["thread_key"], f.Lifecycle["status"] = row.ThreadKey, row.Status
		f.Lifecycle["created_turn"], f.Lifecycle["resolved_turn"] = row.CreatedTurn, row.ResolvedTurn
		// PendingThread's optional scalar cannot distinguish an absent legacy
		// value from zero. Keep that raw value in SourceRow, not a new assertion.
		if row.Confidence != 0 {
			f.Knowledge["confidence"] = row.Confidence
		}
		for _, role := range []struct{ name, text string }{{"owner", row.Owner}, {"target", row.Target}} {
			if role.text != "" {
				f.Links = append(f.Links, memoryRelationLink{Kind: "commitment_role", Role: role.name,
					From: r.endpoint(role.name, "", role.text), To: r.node(f.Path, f.Kind)})
			}
		}
		r.Frames = append(r.Frames, f)
		out.Records = append(out.Records, r)
	}
	for i, row := range input.Triples {
		r := newMemoryRelationRecord("kg_triples", row.ChatSessionID, row.ID, "", i, row)
		f := newMemoryRelationFrame("", "unclassified", nil)
		// These integer values are stored turn coordinates, not story dates.
		f.Time["turn_interval"] = map[string]any{"valid_from": row.ValidFrom, "valid_to": row.ValidTo}
		f.Links = append(f.Links, memoryRelationLink{Kind: "unclassified", Predicate: row.Predicate,
			From: r.endpoint("subject", "", row.Subject), To: r.endpoint("object", "", row.Object)})
		r.Frames = append(r.Frames, f)
		out.Records = append(out.Records, r)
	}
	return out
}

func newMemoryRelationRecord(table, session string, id int64, unit string, index int, row any) memoryRelationRecord {
	ref := memoryRelationRef{SessionID: session, Table: table, RowID: id, UnitID: unit}
	if id == 0 && unit == "" {
		ref.InputOrdinal = index + 1
	}
	return memoryRelationRecord{Ref: ref, SourceRow: row}
}

func (r memoryRelationRecord) node(path, kind string) memoryRelationNode {
	return memoryRelationNode{Kind: kind, Record: r.Ref, Path: path}
}

func (r memoryRelationRecord) endpoint(path, id, text string) memoryRelationNode {
	kind := "unknown"
	if id != "" {
		kind = "entity"
	} else if text != "" {
		kind = "unresolved_label"
	}
	return memoryRelationNode{Kind: kind, EntityID: id, Text: text, Record: r.Ref, Path: path}
}

func (r *memoryRelationRecord) document(path, raw string) memoryRelationDocument {
	d := readMemoryRelationDocument(path, raw)
	r.Documents = append(r.Documents, d)
	return d
}

// Payload-only consumers use the same lossless decoder without allocating
// detached row/link/metadata collections that their typed view does not read.
func readMemoryRelationDocument(path, raw string) memoryRelationDocument {
	d := memoryRelationDocument{Path: path, RawJSON: raw}
	if strings.TrimSpace(raw) == "" {
		return d
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber() // Evidence/row IDs can exceed float64's exact range.
	if err := decoder.Decode(&d.Value); err == nil {
		var trailing any
		d.Parsed = decoder.Decode(&trailing) == io.EOF
	}
	if !d.Parsed {
		d.Value = nil // Keep malformed input in RawJSON, without a partial assertion.
	}
	return d
}

func (r *memoryRelationRecord) frame(path, raw, kind string) memoryRelationFrame {
	d := r.document(path, raw)
	fields, _ := d.Value.(map[string]any)
	return newMemoryRelationFrame(path, kind, fields)
}

func newMemoryRelationFrame(path, kind string, fields map[string]any) memoryRelationFrame {
	return memoryRelationFrame{
		Path: path, Kind: kind, Fields: fields,
		Provenance: relationFields(fields, "source", "source_fields", "source_revision", "source_turn", "branch", "evidence_excerpt", "direct_evidence_ids"),
		Time:       relationFields(fields, "observed_at", "occurrence_time", "effective_time", "validity", "story_clock", "due", "schedule", "duration"),
		Knowledge:  relationFields(fields, "claim_scope", "truth_scope", "epistemic_mode", "authority_class", "visibility", "knowledge_holder_entity_id", "perspective_owner", "reveal_condition", "confidence", "expression_scope", "uncertainty", "counterevidence", "reciprocity", "consent", "stability"),
		Lifecycle:  relationFields(fields, "lifecycle_key", "lifecycle_state", "transition", "lifecycle_details", "pending_thread"),
	}
}

func (r *memoryRelationRecord) stateFrame(path, raw, statusKey string) memoryRelationFrame {
	f := r.frame(path, raw, "state")
	if len(f.Fields) == 0 {
		return f // The row/document remains, without inventing a relation.
	}
	switch statusKey {
	case relationshipStateStatusKey:
		f.Kind = "relationship"
		f.Links = append(f.Links, memoryRelationLink{Kind: "relationship_observation", Predicate: relationString(f.Fields, "domain"),
			From: r.endpoint(path+"/source_entity_id", relationString(f.Fields, "source_entity_id"), relationString(f.Fields, "source_label")),
			To:   r.endpoint(path+"/target_entity_id", relationString(f.Fields, "target_entity_id"), relationString(f.Fields, "target_label"))})
		if current, ok := f.Fields["current"].(map[string]any); ok {
			f.Knowledge["current"] = relationFields(current, "visibility", "support_kind")
		}
	case narrativeStateStatusKey:
		f.Links = append(f.Links, memoryRelationLink{Kind: "state_subject", Predicate: relationString(f.Fields, "state_slot"),
			From: r.endpoint(path+"/subject", "", relationString(f.Fields, "subject")), To: r.node(path, "state")})
	}
	return f
}

func relationString(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func relationFields(fields map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, exists := fields[key]; exists {
			out[key] = value
		}
	}
	return out
}
