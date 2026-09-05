package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	MemoryAdmissionContract   = "memory_admission.v1"
	PreciseMemoryUnitContract = "precise_memory_unit.v1"
)

// PreciseMemoryUnit is an additive semantic memory projection. It does
// not replace the legacy aggregate Memory row. Each unit preserves one
// semantic claim or occurrence and its accepted source-turn provenance.
type PreciseMemoryUnit struct {
	ID                      int64     `json:"id"`
	UnitID                  string    `json:"unit_id"`
	ContractVersion         string    `json:"contract_version"`
	ChatSessionID           string    `json:"chat_session_id"`
	SourceTurnStart         int       `json:"source_turn_start"`
	SourceTurnEnd           int       `json:"source_turn_end"`
	SourceContract          string    `json:"source_contract"`
	SourceRevision          string    `json:"source_revision"`
	SourceLogicalTurnID     string    `json:"source_logical_turn_id,omitempty"`
	SourceMessageID         string    `json:"source_message_id,omitempty"`
	SourceGenerationID      string    `json:"source_generation_id,omitempty"`
	SourceContentHash       string    `json:"source_content_hash"`
	SourceRole              string    `json:"source_role"`
	SourceSpanStart         int       `json:"source_span_start"`
	SourceSpanEnd           int       `json:"source_span_end"`
	EvidenceExcerpt         string    `json:"evidence_excerpt"`
	EvidenceHash            string    `json:"evidence_hash"`
	RootEvidenceID          int64     `json:"root_evidence_id"`
	DirectEvidenceIDsJSON   string    `json:"direct_evidence_ids_json"`
	Kind                    string    `json:"kind"`
	Subtype                 string    `json:"subtype,omitempty"`
	PayloadJSON             string    `json:"payload_json"`
	ActorEntityID           string    `json:"actor_entity_id,omitempty"`
	SubjectEntityID         string    `json:"subject_entity_id,omitempty"`
	AffectedEntityID        string    `json:"affected_entity_id,omitempty"`
	LocationEntityID        string    `json:"location_entity_id,omitempty"`
	ObjectEntityID          string    `json:"object_entity_id,omitempty"`
	RelationshipKey         string    `json:"relationship_key,omitempty"`
	TruthScope              string    `json:"truth_scope"`
	EpistemicMode           string    `json:"epistemic_mode"`
	AuthorityClass          string    `json:"authority_class"`
	AdmissionState          string    `json:"admission_state"`
	ReviewState             string    `json:"review_state"`
	Visibility              string    `json:"visibility"`
	KnowledgeHolderEntityID string    `json:"knowledge_holder_entity_id,omitempty"`
	RevealCondition         string    `json:"reveal_condition,omitempty"`
	Confidence              float64   `json:"confidence"`
	IdempotencyKey          string    `json:"idempotency_key"`
	DerivationVersion       string    `json:"derivation_version"`
	ExtractorVersion        string    `json:"extractor_version"`
	IndexVersion            string    `json:"index_version"`
	LifecycleState          string    `json:"lifecycle_state"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
	// Vector fields are transient commit material. They are serialized only
	// into the durable vector outbox and are not persisted as MariaDB columns.
	VectorEmbedding         []float32 `json:"-"`
	VectorEmbeddingModel    string    `json:"-"`
	VectorContextChunks     []string  `json:"-"`
	VectorContextChunkIndex int       `json:"-"`
}

// PreciseMemorySemanticText is the shared searchable projection for precise
// memory. Semantic fields lead; an available source excerpt remains supporting
// context and is never required to make an otherwise understandable unit
// searchable.
func PreciseMemorySemanticText(item *PreciseMemoryUnit) string {
	if item == nil {
		return ""
	}
	payload := map[string]any{}
	_ = json.Unmarshal([]byte(item.PayloadJSON), &payload)
	keys := []string{
		"subject", "subject_entity", "source_entity", "actor", "actor_name", "speaker_name", "speaker", "character",
		"target_entity", "counterpart", "affected_entity", "target",
		"summary", "event", "action", "observation", "claim",
		"state_slot", "relation_dimension", "domain", "trait_domain",
		"behavior_key", "behavior_expression", "trait_key", "supported_expression",
		"principle_key", "utterance_expression", "profile_key", "value",
		"decision", "action_scope", "context_key", "relationship_key",
	}
	parts := []string{}
	seen := map[string]bool{}
	appendPart := func(label string, value any) {
		text := ""
		switch typed := value.(type) {
		case string:
			text = strings.TrimSpace(typed)
		case []any, []string:
			encoded, err := json.Marshal(typed)
			if err == nil {
				text = string(encoded)
			}
		case nil:
			return
		default:
			text = strings.TrimSpace(fmt.Sprint(typed))
		}
		if text == "" {
			return
		}
		key := strings.ToLower(strings.Join(strings.Fields(label+"\x00"+text), " "))
		if seen[key] {
			return
		}
		seen[key] = true
		parts = append(parts, label+": "+text)
	}
	appendPart("kind", item.Kind)
	appendPart("subtype", item.Subtype)
	for _, key := range keys {
		appendPart(key, payload[key])
	}
	if excerpt := strings.TrimSpace(item.EvidenceExcerpt); excerpt != "" {
		appendPart("source", excerpt)
	}
	return strings.Join(parts, "\n")
}

// PreciseMemoryWriter is optional so legacy, fixture, noop, and read-only
// stores remain compatible without claiming that atomic writes succeeded.
type PreciseMemoryWriter interface {
	SavePreciseMemoryUnit(context.Context, *PreciseMemoryUnit) (inserted bool, err error)
}

// CharacterPerspectiveMemoryReader exposes source-active, atomic perspective
// units to prepare-turn. Callers must still enforce the current stable
// knowledge-holder identity before rendering any payload.
type CharacterPerspectiveMemoryReader interface {
	ListCharacterPerspectiveMemoryUnits(context.Context, string, string) ([]PreciseMemoryUnit, error)
}

// ActiveInteractionMemoryReader exposes source-active atomic relationship
// observations and interaction boundaries. Review metadata does not erase the
// semantic occurrence; the caller still applies visibility and current-turn
// selection for the current request.
type ActiveInteractionMemoryReader interface {
	ListActiveInteractionMemoryUnits(context.Context, string) ([]PreciseMemoryUnit, error)
}

// GeneralVectorPreciseMemoryReader lists source-active precise units that are
// eligible for the general vector index, including their canonical fact text
// and provenance for request-time vector-hit hydration. Perspective-scoped
// units remain on their typed lanes.
type GeneralVectorPreciseMemoryReader interface {
	ListGeneralVectorPreciseMemoryUnits(context.Context, string) ([]PreciseMemoryUnit, error)
}

// PreciseMemoryWriteAvailability lets composite stores report whether at least
// one real persistence lane can accept the optional projection.
type PreciseMemoryWriteAvailability interface {
	PreciseMemoryWritesEnabled() bool
}

// MemoryAdmission is the single source-fenced commit payload shared by the
// foreground complete-turn path and the durable reprocessing worker. The
// legacy Memory row remains a compatibility read projection, but it is written
// in the same transaction as its exact Direct Evidence and precise units.
type MemoryAdmission struct {
	ContractVersion   string
	ChatSessionID     string
	SourceRevision    string
	TurnIndex         int
	DerivationVersion string
	ExtractorVersion  string
	IndexVersion      string
	ResultHash        string
	ResultJSON        string
	Memory            *Memory
	Evidence          []*DirectEvidence
	PreciseUnits      []*PreciseMemoryUnit
	Vectors           []MemoryAdmissionVector
	CreatedAt         time.Time
}

type MemoryAdmissionVector struct {
	ArtifactType          string
	EvidenceText          string
	Embedding             []float32
	Tier                  string
	SourceTable           string
	SchemaVersion         string
	DocumentText          string
	SearchTextPolicy      string
	RawLanguage           string
	SummaryLanguage       string
	SessionOutputLanguage string
	AliasCount            int
	EmbeddingModel        string
	ContextChunks         []string
	ContextChunkIndex     int
}

type MemoryAdmissionResult struct {
	Idempotent          bool
	ExistingResultHash  string
	ExistingResultJSON  string
	MemoryInserted      bool
	MemoryUpdated       bool
	EvidenceInserted    int
	EvidenceReactivated int
	EvidenceRetired     int
	PreciseInserted     int
	PreciseReactivated  int
	PreciseRetired      int
	VectorOperations    int
	CommittedResultHash string
	CommittedAt         time.Time
}

// MemoryAdmissionProjectionExpectation is the read-side counterpart of one
// committed admission. Callers derive it from the committed result JSON and
// accepted raw source; stores compare it with the durable rows without
// changing source or projection state.
type MemoryAdmissionProjectionExpectation struct {
	ChatSessionID            string
	SourceRevision           string
	TurnIndex                int
	DerivationVersion        string
	ExtractorVersion         string
	IndexVersion             string
	ResultHash               string
	ResultJSON               string
	MemoryExpected           bool
	MemorySummaryJSON        string
	EvidenceTexts            []string
	PreciseUnitCount         int
	PreciseDependencyCount   int
	AdmissionVectorArtifacts []string
}

// MemoryAdmissionProjectionInspection is complete only when every expected
// atomic projection, dependency, and vector-outbox row is present and current.
// A legitimate zero-count lane is represented by an expected count of zero
// and must also have zero active rows.
type MemoryAdmissionProjectionInspection struct {
	Complete       bool
	ExpectedCounts map[string]int
	ActualCounts   map[string]int
	MissingLanes   []string
	StaleLanes     []string
}

// MemoryAdmissionProjectionInspector proves whether a committed source marker
// still has all of its durable atomic projections. Stores that cannot provide
// this proof must not be treated as complete by administrative replay.
type MemoryAdmissionProjectionInspector interface {
	InspectMemoryAdmissionProjection(
		context.Context,
		MemoryAdmissionProjectionExpectation,
	) (MemoryAdmissionProjectionInspection, error)
}

// MemoryAdmissionWriter is the canonical 3.6 memory write boundary. Stores
// advertising this interface must commit the compatibility aggregate,
// evidence, precise rows, dependencies, source admission marker, and vector
// outbox changes atomically.
type MemoryAdmissionWriter interface {
	CommitMemoryAdmission(context.Context, *MemoryAdmission) (MemoryAdmissionResult, error)
}

type MemoryAdmissionWriteAvailability interface {
	MemoryAdmissionWritesEnabled() bool
}
