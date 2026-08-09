package store

import (
	"context"
	"fmt"
	"strings"
)

const (
	SessionMigrationManifestVersion = "session-migration.manifest.v1"

	SessionMigrationPolicyCopy                = "copy"
	SessionMigrationPolicyRetainAudit         = "retain_audit"
	SessionMigrationPolicyRegenerate          = "regenerate"
	SessionMigrationPolicyDeleteAfterVerified = "delete_after_verified"

	SessionMigrationManifestParityUnverifiedReason = "session_migration_manifest_parity_unverified"

	SessionMigrationProofOperationSourceLock      = "source_lock"
	SessionMigrationProofOperationCleanupPrepare  = "cleanup_prepare"
	SessionMigrationProofOperationCleanupFinalize = "cleanup_finalize"
	SessionMigrationProofOperationResume          = "resume"
)

// SessionMigrationManifestEntry classifies one direct session-scoped table or
// an indirect child reached through a direct table. The manifest is exhaustive
// for migrations/001_schema.sql at SessionMigrationManifestVersion.
//
// Implemented is deliberately false for work that the current copy executor
// cannot yet prove with source/target hash, row-map, FK, and vector parity.
// Destructive migration phases must remain fail-closed while any entry is not
// implemented and verified.
type SessionMigrationManifestEntry struct {
	Table                 string                                 `json:"table"`
	SessionColumn         string                                 `json:"session_column,omitempty"`
	RelatedSessionColumns []SessionMigrationRelatedSessionColumn `json:"related_session_columns,omitempty"`
	ParentTable           string                                 `json:"parent_table,omitempty"`
	Policy                string                                 `json:"policy"`
	Direct                bool                                   `json:"direct"`
	Implemented           bool                                   `json:"implemented"`
}

const (
	SessionMigrationKeyAutoIncrement = "auto_increment"
	SessionMigrationKeyUUID          = "uuid"
	SessionMigrationKeyPreserve      = "preserve"
)

// SessionMigrationExecutionPlan is the static allowlist consumed by the
// production executor. Columns are intentionally enumerated instead of being
// discovered and copied from information_schema: a schema change must update
// this versioned contract or migration fails before target mutation.
type SessionMigrationExecutionPlan struct {
	Table              string
	Columns            []string
	PrimaryKey         []string
	PrimaryKeyMode     string
	ParentColumn       string
	ForeignKeys        []SessionMigrationForeignKeyPlan
	SemanticReferences []SessionMigrationSemanticReferencePlan
	GeneratedKeys      []SessionMigrationGeneratedKeyPlan
	DatabaseGenerated  []string
	Vector             *SessionMigrationVectorPlan
}

type SessionMigrationForeignKeyPlan struct {
	Column          string
	ReferenceTable  string
	ReferenceColumn string
	Deferred        bool
}

type SessionMigrationGeneratedKeyPlan struct {
	Column string
	Kind   string
}

const (
	SessionMigrationSemanticJSONIDArray     = "json_id_array"
	SessionMigrationSemanticTypedArtifactID = "typed_artifact_id"
	SessionMigrationSemanticPrefixedID      = "prefixed_id"
)

// SessionMigrationSemanticReferencePlan covers canonical lineage references
// that are not declared as SQL foreign keys. These references must be remapped
// and included in parity just like physical foreign keys, or source cleanup
// could leave valid-looking but dangling target provenance.
type SessionMigrationSemanticReferencePlan struct {
	Column        string
	Kind          string
	TypeColumn    string
	ReferenceType string
	References    map[string]SessionMigrationArtifactReference
}

type SessionMigrationArtifactReference struct {
	Table  string
	Column string
}

type SessionMigrationVectorPlan struct {
	Tier            string
	IDColumn        string
	EmbeddingColumn string
	TextColumns     []string
	TextFormat      string
	Eligibility     string
	SchemaVersion   string
}

type SessionMigrationVectorParityContext struct {
	MigrationID     int64
	TargetSessionID string
	ExpectedIDs     []string
}

type SessionMigrationVectorParityResult struct {
	MigrationID     int64
	TargetSessionID string
	ExpectedIDs     []string
	ActualIDs       []string
	MissingIDs      []string
	UnexpectedIDs   []string
	ExpectedIDHash  string
	ActualIDHash    string
	Verified        bool
}

type SessionMigrationBlockerError struct {
	Code  string
	Phase string
	Table string
}

func (e *SessionMigrationBlockerError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{"session migration blocked", e.Code}
	if e.Phase != "" {
		parts = append(parts, "phase="+e.Phase)
	}
	if e.Table != "" {
		parts = append(parts, "table="+e.Table)
	}
	return strings.Join(parts, ": ")
}

func sessionMigrationBlocker(code, phase, table string) error {
	return &SessionMigrationBlockerError{Code: code, Phase: phase, Table: table}
}

// SessionMigrationVectorParityStore is separate from the legacy count/status
// interface so callers cannot mistake count equality for exact document-ID
// parity.
type SessionMigrationVectorParityStore interface {
	GetSessionMigrationVectorParityContext(ctx context.Context, migrationID int64) (*SessionMigrationVectorParityContext, error)
	VerifySessionMigrationVectorParity(ctx context.Context, migrationID int64, operation string, actualIDs []string) (*SessionMigrationVectorParityResult, error)
}

type SessionMigrationRelatedSessionColumn struct {
	Column    string `json:"column"`
	Semantics string `json:"semantics"`
}

type SessionMigrationMetadataExclusion struct {
	Table          string   `json:"table"`
	SessionColumns []string `json:"session_columns"`
	Semantics      string   `json:"semantics"`
}

var sessionMigrationManifestV1 = []SessionMigrationManifestEntry{
	{Table: "chat_logs", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true, Implemented: true},
	{Table: "effective_input_logs", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true, Implemented: true},
	{Table: "memories", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true, Implemented: true},
	{Table: "direct_evidence_records", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true, Implemented: true},
	{Table: "kg_triples", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true, Implemented: true},
	{Table: "audit_logs", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyRetainAudit, Direct: true},
	{Table: "critic_feedback", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyRetainAudit, Direct: true},
	{Table: "persona_memory_capsules", SessionColumn: "source_chat_session_id", Policy: SessionMigrationPolicyRetainAudit, Direct: true},
	{Table: "protagonist_entity_memories", SessionColumn: "source_chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true, Implemented: true},
	{Table: "persona_capsule_attachments", SessionColumn: "target_chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "character_events", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "entities", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "trust_states", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "storylines", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "guidance_plan_states", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "world_rules", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "session_active_scopes", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "character_states", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "pending_threads", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "active_states", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "canonical_state_layers", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "episode_summaries", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true, Implemented: true},
	{Table: "chapter_summaries", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "arc_summaries", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "saga_digests", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "consequence_records", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "psychology_branches", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{
		Table:         "session_fork_lineage",
		SessionColumn: "chat_session_id",
		RelatedSessionColumns: []SessionMigrationRelatedSessionColumn{
			{Column: "copied_from_session_id", Semantics: "retain_historical_provenance_sid"},
		},
		Policy: SessionMigrationPolicyCopy,
		Direct: true,
	},
	{Table: "theme_offscreen_carries", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "capture_verification_records", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyRetainAudit, Direct: true},
	{Table: "status_schema_proposals", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "status_schema_registry", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "status_current_values", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "status_change_events", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "status_effects", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "session_reference_bindings", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true, Implemented: true},
	{Table: "entity_identities", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "entity_identity_surfaces", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "entity_identity_links", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "entity_identity_artifact_bindings", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "speaker_attributions", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "precise_memory_units", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "memory_source_revisions", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "memory_derivation_dependencies", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyCopy, Direct: true},
	{Table: "memory_reprocessing_jobs", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyDeleteAfterVerified, Direct: true},
	{Table: "memory_vector_outbox", SessionColumn: "chat_session_id", Policy: SessionMigrationPolicyDeleteAfterVerified, Direct: true},

	{Table: "persona_memory_entries", ParentTable: "persona_memory_capsules", Policy: SessionMigrationPolicyRetainAudit},
	{Table: "session_reference_runtime", ParentTable: "session_reference_bindings", Policy: SessionMigrationPolicyRegenerate},
	{Table: "session_reference_coverage_snapshots", ParentTable: "session_reference_bindings", Policy: SessionMigrationPolicyRegenerate},
	{Table: "session_reference_coverage_fields", ParentTable: "session_reference_coverage_snapshots", Policy: SessionMigrationPolicyRegenerate},
}

var sessionMigrationMetadataExclusionsV1 = []SessionMigrationMetadataExclusion{
	{
		Table:          "session_migrations",
		SessionColumns: []string{"source_session_id", "target_session_id"},
		Semantics:      "migration_ledger_metadata_not_source_content",
	},
	{
		Table:          "session_migration_locks",
		SessionColumns: []string{"source_session_id", "target_session_id"},
		Semantics:      "migration_lock_metadata_not_source_content",
	},
	{
		Table:          "session_route_bindings",
		SessionColumns: []string{"canonical_session_id", "redirected_from_session_id"},
		Semantics:      "routing_metadata_updated_only_after_backend_binding_readback",
	},
}

var sessionMigrationExecutionPlansV1 = buildSessionMigrationExecutionPlansV1()

func buildSessionMigrationExecutionPlansV1() map[string]SessionMigrationExecutionPlan {
	columns := map[string]string{
		"chat_logs":                            "id,chat_session_id,turn_index,role,content,created_at",
		"effective_input_logs":                 "id,chat_session_id,turn_index,effective_input,created_at",
		"memories":                             "id,chat_session_id,turn_index,summary_json,embedding,embedding_model,importance,emotional_boost,evidence,emotional_intensity,narrative_significance,place_wing,place_room,created_at",
		"direct_evidence_records":              "id,chat_session_id,evidence_kind,evidence_text,source_turn_start,source_turn_end,turn_anchor,source_message_ids_json,source_hash,archive_state,capture_stage,capture_verification,committed_gate,lineage_json,repair_needed,tombstoned,superseded_by_id,created_at",
		"kg_triples":                           "id,chat_session_id,subject,predicate,object,valid_from,valid_to,source_turn,created_at",
		"audit_logs":                           "id,created_at,event_type,chat_session_id,target_type,target_id,summary,details_json,source",
		"critic_feedback":                      "id,created_at,chat_session_id,target_type,target_id,feedback_value,feedback_note,source",
		"persona_memory_capsules":              "id,persona_key,source_chat_session_id,source_character_name,title,mode,summary,created_at,updated_at",
		"protagonist_entity_memories":          "id,persona_entity_key,persona_entity_name,owner_entity_key,owner_entity_name,owner_entity_role,owner_visibility,source_chat_session_id,source_character_name,source_turn_index,memory_text,evidence_excerpt,secret_guard,portability,target_reveal_policy,tags_json,importance_10,emotional_weight,created_at,updated_at",
		"persona_capsule_attachments":          "id,capsule_id,target_chat_session_id,injection_mode,enabled,created_at,updated_at",
		"character_events":                     "id,chat_session_id,character_name,turn_index,event_type,details_json,created_at",
		"entities":                             "id,chat_session_id,name,entity_type,description,aliases_json,first_seen_turn,last_seen_turn,confidence,pinned,suppressed,user_corrected,created_at,updated_at",
		"trust_states":                         "id,chat_session_id,target_name,target_type,score,reason_json,source_turn,pinned,suppressed,user_corrected,created_at,updated_at",
		"storylines":                           "id,chat_session_id,name,status,entities_json,current_context,key_points_json,ongoing_tensions_json,confidence,evidence_count,last_evidence_turn,first_turn,last_turn,pinned,suppressed,user_corrected,created_at,updated_at",
		"guidance_plan_states":                 "id,chat_session_id,story_plan_json,director_json,state_status,last_turn,warnings_json,created_at,updated_at",
		"world_rules":                          "id,chat_session_id,scope,scope_name,category,key,value_json,genre,source_turn,pinned,suppressed,user_corrected,created_at,updated_at",
		"session_active_scopes":                "id,chat_session_id,active_scope,scope_name,updated_at",
		"character_states":                     "id,chat_session_id,character_name,appearance_json,personality_json,status_json,relationships_json,speech_style_json,turn_index,created_at,updated_at",
		"pending_threads":                      "id,chat_session_id,thread_key,description,status,created_turn,resolved_turn,source_turn,priority,hook_type,hook_metadata_json,pinned,suppressed,user_corrected,created_at,updated_at",
		"active_states":                        "id,chat_session_id,state_type,content,turn_index,created_at",
		"canonical_state_layers":               "id,chat_session_id,layer_type,content,source_state_type,turn_index,source_turn,source_record,last_verified_turn,confidence,created_at",
		"episode_summaries":                    "id,chat_session_id,from_turn,to_turn,summary_text,key_entities,key_events,open_loops_json,relationship_changes_json,embedding_vector,embedding_model,created_at",
		"chapter_summaries":                    "id,chat_session_id,from_turn,to_turn,chapter_index,chapter_title,summary_text,open_loops_json,relationship_changes_json,world_changes_json,callback_candidates_json,resume_text,embedding_vector,embedding_model,created_at",
		"arc_summaries":                        "id,chat_session_id,from_turn,to_turn,arc_index,arc_name,arc_status,core_conflict,key_turning_points_json,active_promises_json,unresolved_debts_json,resolved_payoffs_json,callback_candidates_json,future_payoff_candidates_json,irreversible_turns_json,callback_debts_json,relationship_pivots_json,arc_resume_text,embedding_vector,embedding_model,created_at",
		"saga_digests":                         "id,chat_session_id,from_turn,to_turn,era_label,saga_summary,persistent_facts_json,never_drop_candidates_json,resume_pack_text,embedding_vector,embedding_model,created_at",
		"consequence_records":                  "id,chat_session_id,source_turn_start,source_turn_end,decision,immediate_result,delayed_effect,affected_relations,affected_world,status,importance,confidence,foreground_eligible,quiet_turns,last_seen_turn,paid_turn,expires_after_quiet_turns,source_hash,evidence_json,created_at,updated_at",
		"psychology_branches":                  "id,chat_session_id,character_name,branch_type,axis_name,summary,status,confidence,confidence_label,source_kind,source_turn_start,source_turn_end,source_hash,evidence_json,quiet_turns,last_seen_turn,dormant_after_quiet_turns,created_at,updated_at",
		"session_fork_lineage":                 "id,chat_session_id,scope_id,parent_scope_id,copied_from_scope_id,copied_from_session_id,imported_at,divergence_marker,provenance_source,inheritance_mode,inherited_items_json,created_at,updated_at",
		"theme_offscreen_carries":              "id,chat_session_id,surface_type,label,summary,status,confidence,confidence_label,source_kind,source_turn_start,source_turn_end,source_hash,evidence_json,quiet_turns,last_seen_turn,dormant_after_quiet_turns,foreground_eligible,foreground_reason_json,created_at,updated_at",
		"capture_verification_records":         "id,chat_session_id,turn_index,stage_name,verification_state,degraded_reason,compact_metadata_json,content_hash,evidence_json,previous_record_id,repaired_by_record_id,repair_attempt_count,repair_evidence_json,repaired_at,user_input_preserved,payload_rewrite,created_at,updated_at",
		"status_schema_proposals":              "id,chat_session_id,input_channel,proposal_state,schema_name,ruleset_label,schema_json,provenance_json,review_note,reviewer,reviewed_at,created_at,updated_at",
		"status_schema_registry":               "id,chat_session_id,source_proposal_id,schema_name,ruleset_label,status_key,label,owner_scope,value_kind,bounds_json,options_json,default_value_json,registry_state,created_at,updated_at",
		"status_current_values":                "id,chat_session_id,registry_id,status_key,owner_scope,owner_id,owner_label,value_kind,value_json,evidence_json,source_turn,write_state,created_at,updated_at",
		"status_change_events":                 "id,chat_session_id,registry_id,status_value_id,status_key,owner_scope,owner_id,event_kind,previous_value_json,new_value_json,evidence_json,source_turn,story_clock_json,event_state,created_at",
		"status_effects":                       "id,chat_session_id,registry_id,status_key,owner_scope,owner_id,effect_kind,effect_label,effect_payload_json,evidence_json,source_turn,start_clock_json,duration_json,expires_at_clock_json,effect_state,cleared_evidence_json,cleared_turn,created_at,updated_at",
		"session_reference_bindings":           "binding_id,chat_session_id,work_id,continuity_id,binding_role,reference_mode,enabled,injection_enabled,anchor_mode,current_node_id,reveal_ceiling_node_id,divergence_node_id,future_policy,priority,revision,created_at,updated_at",
		"entity_identities":                    "stable_entity_id,chat_session_id,identity_namespace,entity_kind,canonical_label,lifecycle_state,review_state,presence_authority,occurrence_authority,source_contract,source_revision,source_logical_turn_id,source_message_id,source_generation_id,source_content_hash,source_turn,source_index,idempotency_key,mapping_revision,first_seen_turn,last_seen_turn,created_at,updated_at",
		"entity_identity_surfaces":             "surface_id,stable_entity_id,chat_session_id,identity_namespace,surface_kind,surface_text,normalized_surface,surface_scope,valid_from_turn,valid_to_turn,source_contract,source_revision,source_turn,source_span_start,source_span_end,evidence_excerpt,review_state,idempotency_key,created_at,updated_at",
		"entity_identity_links":                "link_id,chat_session_id,source_entity_id,target_entity_id,link_kind,link_state,evidence_json,mapping_revision,created_at,updated_at",
		"entity_identity_artifact_bindings":    "binding_id,stable_entity_id,chat_session_id,artifact_kind,artifact_role,artifact_ordinal,surface_text,review_state,source_contract,source_revision,source_turn,idempotency_key,created_at",
		"speaker_attributions":                 "attribution_id,chat_session_id,speaker_entity_id,identity_namespace,source_role,attribution_kind,attribution_state,review_state,confidence,source_contract,source_revision,source_logical_turn_id,source_message_id,source_generation_id,source_content_hash,source_turn,source_span_start,source_span_end,evidence_excerpt,idempotency_key,created_at,updated_at",
		"precise_memory_units":                 "id,unit_id,contract_version,chat_session_id,source_turn_start,source_turn_end,source_contract,source_revision,source_logical_turn_id,source_message_id,source_generation_id,source_content_hash,source_role,source_span_start,source_span_end,evidence_excerpt,evidence_hash,root_evidence_id,direct_evidence_ids_json,memory_kind,memory_subtype,payload_json,actor_entity_id,subject_entity_id,affected_entity_id,location_entity_id,object_entity_id,relationship_key,truth_scope,epistemic_mode,authority_class,admission_state,review_state,visibility,knowledge_holder_entity_id,reveal_condition,confidence,idempotency_key,lifecycle_state,created_at,updated_at",
		"memory_source_revisions":              "id,contract_version,source_revision,chat_session_id,logical_turn_id,turn_index,source_message_id,source_generation_id,branch_id,branch_state,raw_user_content,raw_assistant_content,combined_content_hash,user_observed_content_hash,assistant_observed_content_hash,hash_algorithm,host_observed_at_ms,lifecycle_state,derived_admission_state,derived_admission_version,derived_extractor_version,derived_index_version,derived_result_hash,derived_result_json,derived_admitted_at,critic_input_snapshot_json,critic_input_snapshot_hash,active_logical_turn_slot,superseded_by_revision,invalidation_reason,invalidated_at,created_at,updated_at",
		"memory_derivation_dependencies":       "id,contract_version,chat_session_id,source_revision,root_source_pointer,child_artifact_type,child_artifact_id,parent_artifact_type,parent_artifact_id,derivation_version,extractor_version,index_version,lifecycle_state,invalidated_at,created_at,updated_at",
		"memory_reprocessing_jobs":             "id,contract_version,idempotency_key,chat_session_id,source_revision,source_contract,derivation_version,extractor_version,index_version,status,attempts,retry_after,lease_owner,lease_until,last_error,created_at,updated_at",
		"memory_vector_outbox":                 "id,contract_version,operation_key,operation,chat_session_id,source_revision,document_id,document_json,embedding_ready,required_source_state,status,attempts,retry_after,lease_owner,lease_until,last_error,created_at,updated_at",
		"persona_memory_entries":               "id,capsule_id,source_memory_type,source_memory_id,source_turn_index,memory_text,emotional_weight,importance_10,portability,tags_json,evidence_excerpt,injection_policy,created_at",
		"session_reference_runtime":            "binding_id,candidate_node_id,candidate_source_turn,candidate_evidence_json,candidate_confirmed,last_claim_ids_json,diagnostics_json,revision,created_at,updated_at",
		"session_reference_coverage_snapshots": "binding_id,contract_version,context_hash,inventory_hash,snapshot_hash,source_message_count,field_count,covered_field_count,stats_json,revision,created_at,updated_at",
		"session_reference_coverage_fields":    "binding_id,field_key,work_id,continuity_id,reference_kind,source_id,field_name,field_value,normalized_value,match_values_json,present_in_context,matched_locations_json,eligible,eligibility_reason,created_at,updated_at",
	}
	primaryKeys := map[string]string{
		"session_reference_bindings":           "binding_id",
		"entity_identities":                    "stable_entity_id",
		"entity_identity_surfaces":             "surface_id",
		"entity_identity_links":                "link_id",
		"entity_identity_artifact_bindings":    "binding_id",
		"speaker_attributions":                 "attribution_id",
		"session_reference_runtime":            "binding_id",
		"session_reference_coverage_snapshots": "binding_id",
		"session_reference_coverage_fields":    "binding_id,field_key",
	}
	uuidPrimaryKeys := map[string]bool{
		"session_reference_bindings":        true,
		"entity_identities":                 true,
		"entity_identity_surfaces":          true,
		"entity_identity_links":             true,
		"entity_identity_artifact_bindings": true,
		"speaker_attributions":              true,
	}
	plans := make(map[string]SessionMigrationExecutionPlan, len(columns))
	for table, csv := range columns {
		primaryKey := primaryKeys[table]
		if primaryKey == "" {
			primaryKey = "id"
		}
		keyMode := SessionMigrationKeyAutoIncrement
		if uuidPrimaryKeys[table] {
			keyMode = SessionMigrationKeyUUID
		} else if primaryKey != "id" {
			keyMode = SessionMigrationKeyPreserve
		}
		plans[table] = SessionMigrationExecutionPlan{
			Table:          table,
			Columns:        splitSessionMigrationColumns(csv),
			PrimaryKey:     splitSessionMigrationColumns(primaryKey),
			PrimaryKeyMode: keyMode,
		}
	}
	setPlan := func(table string, mutate func(*SessionMigrationExecutionPlan)) {
		plan := plans[table]
		mutate(&plan)
		plans[table] = plan
	}
	setPlan("direct_evidence_records", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{{Column: "superseded_by_id", ReferenceTable: "direct_evidence_records", ReferenceColumn: "id", Deferred: true}}
	})
	setPlan("capture_verification_records", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{
			{Column: "previous_record_id", ReferenceTable: "capture_verification_records", ReferenceColumn: "id", Deferred: true},
			{Column: "repaired_by_record_id", ReferenceTable: "capture_verification_records", ReferenceColumn: "id", Deferred: true},
		}
	})
	setPlan("status_schema_registry", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{{Column: "source_proposal_id", ReferenceTable: "status_schema_proposals", ReferenceColumn: "id"}}
	})
	setPlan("status_current_values", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{{Column: "registry_id", ReferenceTable: "status_schema_registry", ReferenceColumn: "id"}}
	})
	setPlan("status_change_events", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{
			{Column: "registry_id", ReferenceTable: "status_schema_registry", ReferenceColumn: "id"},
			{Column: "status_value_id", ReferenceTable: "status_current_values", ReferenceColumn: "id"},
		}
	})
	setPlan("status_effects", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{{Column: "registry_id", ReferenceTable: "status_schema_registry", ReferenceColumn: "id"}}
	})
	setPlan("entity_identity_surfaces", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{{Column: "stable_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"}}
	})
	setPlan("entity_identity_links", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{
			{Column: "source_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
			{Column: "target_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
		}
	})
	setPlan("entity_identity_artifact_bindings", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{{Column: "stable_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"}}
	})
	setPlan("speaker_attributions", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{
			{Column: "speaker_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
			{Column: "source_revision", ReferenceTable: "memory_source_revisions", ReferenceColumn: "source_revision"},
		}
	})
	setPlan("entity_identities", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = append(plan.ForeignKeys,
			SessionMigrationForeignKeyPlan{Column: "source_revision", ReferenceTable: "memory_source_revisions", ReferenceColumn: "source_revision"})
	})
	setPlan("entity_identity_surfaces", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = append(plan.ForeignKeys,
			SessionMigrationForeignKeyPlan{Column: "source_revision", ReferenceTable: "memory_source_revisions", ReferenceColumn: "source_revision"})
	})
	setPlan("entity_identity_artifact_bindings", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = append(plan.ForeignKeys,
			SessionMigrationForeignKeyPlan{Column: "source_revision", ReferenceTable: "memory_source_revisions", ReferenceColumn: "source_revision"})
	})
	setPlan("precise_memory_units", func(plan *SessionMigrationExecutionPlan) {
		plan.GeneratedKeys = []SessionMigrationGeneratedKeyPlan{{Column: "unit_id", Kind: SessionMigrationKeyUUID}}
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{
			{Column: "source_revision", ReferenceTable: "memory_source_revisions", ReferenceColumn: "source_revision"},
			{Column: "root_evidence_id", ReferenceTable: "direct_evidence_records", ReferenceColumn: "id"},
			{Column: "actor_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
			{Column: "subject_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
			{Column: "affected_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
			{Column: "location_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
			{Column: "object_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
			{Column: "knowledge_holder_entity_id", ReferenceTable: "entity_identities", ReferenceColumn: "stable_entity_id"},
		}
		plan.SemanticReferences = []SessionMigrationSemanticReferencePlan{{
			Column: "direct_evidence_ids_json", Kind: SessionMigrationSemanticJSONIDArray,
			References: map[string]SessionMigrationArtifactReference{
				"default": {Table: "direct_evidence_records", Column: "id"},
			},
		}}
	})
	setPlan("memory_source_revisions", func(plan *SessionMigrationExecutionPlan) {
		plan.GeneratedKeys = []SessionMigrationGeneratedKeyPlan{{Column: "source_revision", Kind: SessionMigrationKeyUUID}}
		plan.DatabaseGenerated = []string{"active_logical_turn_slot"}
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{{Column: "superseded_by_revision", ReferenceTable: "memory_source_revisions", ReferenceColumn: "source_revision", Deferred: true}}
	})
	setPlan("memory_derivation_dependencies", func(plan *SessionMigrationExecutionPlan) {
		plan.ForeignKeys = []SessionMigrationForeignKeyPlan{{Column: "source_revision", ReferenceTable: "memory_source_revisions", ReferenceColumn: "source_revision"}}
		plan.SemanticReferences = []SessionMigrationSemanticReferencePlan{
			{
				Column: "root_source_pointer", Kind: SessionMigrationSemanticPrefixedID,
				ReferenceType: "source_revision",
				References: map[string]SessionMigrationArtifactReference{
					"source_revision": {Table: "memory_source_revisions", Column: "source_revision"},
				},
			},
			{
				Column: "child_artifact_id", Kind: SessionMigrationSemanticTypedArtifactID,
				TypeColumn: "child_artifact_type",
				References: map[string]SessionMigrationArtifactReference{
					"precise_memory_unit": {Table: "precise_memory_units", Column: "unit_id"},
				},
			},
			{
				Column: "parent_artifact_id", Kind: SessionMigrationSemanticTypedArtifactID,
				TypeColumn: "parent_artifact_type",
				References: map[string]SessionMigrationArtifactReference{
					"source_revision": {Table: "memory_source_revisions", Column: "source_revision"},
					"direct_evidence": {Table: "direct_evidence_records", Column: "id"},
				},
			},
		}
	})
	setPlan("persona_memory_entries", func(plan *SessionMigrationExecutionPlan) { plan.ParentColumn = "capsule_id" })
	setPlan("session_reference_runtime", func(plan *SessionMigrationExecutionPlan) { plan.ParentColumn = "binding_id" })
	setPlan("session_reference_coverage_snapshots", func(plan *SessionMigrationExecutionPlan) { plan.ParentColumn = "binding_id" })
	setPlan("session_reference_coverage_fields", func(plan *SessionMigrationExecutionPlan) { plan.ParentColumn = "binding_id" })
	setPlan("memories", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{
			Tier: "memory", IDColumn: "id", EmbeddingColumn: "embedding",
			TextColumns: []string{"summary_json", "evidence", "place_wing", "place_room"},
			TextFormat:  "memory", SchemaVersion: "memory.v2",
		}
	})
	setPlan("direct_evidence_records", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{
			Tier: "evidence", IDColumn: "id",
			TextColumns: []string{"evidence_kind", "evidence_text", "source_turn_start", "source_turn_end", "turn_anchor"},
			TextFormat:  "direct_evidence", Eligibility: "active_direct_evidence", SchemaVersion: "direct_evidence.v1",
		}
	})
	setPlan("world_rules", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{
			Tier: "world_rule", IDColumn: "id",
			TextColumns: []string{"scope", "scope_name", "category", "key", "value_json"},
			TextFormat:  "world_rule", Eligibility: "active_world_rule", SchemaVersion: "world_rule.v1",
		}
	})
	setPlan("kg_triples", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{
			Tier: "kg_triple", IDColumn: "id",
			TextColumns: []string{"subject", "predicate", "object"},
			TextFormat:  "kg_triple", SchemaVersion: "kg_triple.v1",
		}
	})
	setPlan("episode_summaries", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{Tier: "episode", IDColumn: "id", EmbeddingColumn: "embedding_vector", TextColumns: []string{"summary_text"}, TextFormat: "plain", SchemaVersion: "episode.v1"}
	})
	setPlan("chapter_summaries", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{Tier: "chapter", IDColumn: "id", EmbeddingColumn: "embedding_vector", TextColumns: []string{"summary_text", "resume_text"}, TextFormat: "plain", SchemaVersion: "chapter.v1"}
	})
	setPlan("arc_summaries", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{Tier: "arc", IDColumn: "id", EmbeddingColumn: "embedding_vector", TextColumns: []string{"core_conflict", "arc_resume_text"}, TextFormat: "plain", SchemaVersion: "arc.v1"}
	})
	setPlan("saga_digests", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{Tier: "saga", IDColumn: "id", EmbeddingColumn: "embedding_vector", TextColumns: []string{"saga_summary", "resume_pack_text"}, TextFormat: "plain", SchemaVersion: "saga.v1"}
	})
	setPlan("precise_memory_units", func(plan *SessionMigrationExecutionPlan) {
		plan.Vector = &SessionMigrationVectorPlan{
			Tier: "precise_memory", IDColumn: "unit_id",
			TextColumns: []string{"evidence_excerpt"}, TextFormat: "plain",
			Eligibility: "active_precise_memory", SchemaVersion: "precise_memory_unit.v1",
		}
	})
	return plans
}

func splitSessionMigrationColumns(csv string) []string {
	raw := strings.Split(csv, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func SessionMigrationExecutionPlanFor(table string) (SessionMigrationExecutionPlan, bool) {
	plan, ok := sessionMigrationExecutionPlansV1[strings.TrimSpace(table)]
	if !ok {
		return SessionMigrationExecutionPlan{}, false
	}
	plan.Columns = append([]string(nil), plan.Columns...)
	plan.PrimaryKey = append([]string(nil), plan.PrimaryKey...)
	plan.ForeignKeys = append([]SessionMigrationForeignKeyPlan(nil), plan.ForeignKeys...)
	plan.SemanticReferences = cloneSessionMigrationSemanticReferences(plan.SemanticReferences)
	plan.GeneratedKeys = append([]SessionMigrationGeneratedKeyPlan(nil), plan.GeneratedKeys...)
	plan.DatabaseGenerated = append([]string(nil), plan.DatabaseGenerated...)
	if plan.Vector != nil {
		vectorPlan := *plan.Vector
		vectorPlan.TextColumns = append([]string(nil), plan.Vector.TextColumns...)
		plan.Vector = &vectorPlan
	}
	return plan, true
}

func cloneSessionMigrationSemanticReferences(in []SessionMigrationSemanticReferencePlan) []SessionMigrationSemanticReferencePlan {
	out := make([]SessionMigrationSemanticReferencePlan, len(in))
	for index, item := range in {
		out[index] = item
		if item.References != nil {
			out[index].References = make(map[string]SessionMigrationArtifactReference, len(item.References))
			for key, reference := range item.References {
				out[index].References[key] = reference
			}
		}
	}
	return out
}

func SessionMigrationExecutionPlans() []SessionMigrationExecutionPlan {
	out := make([]SessionMigrationExecutionPlan, 0, len(sessionMigrationManifestV1))
	for _, entry := range sessionMigrationManifestV1 {
		if plan, ok := SessionMigrationExecutionPlanFor(entry.Table); ok {
			out = append(out, plan)
		}
	}
	return out
}

func sessionMigrationExecutionPlanError(entry SessionMigrationManifestEntry) error {
	plan, ok := sessionMigrationExecutionPlansV1[entry.Table]
	if !ok {
		return fmt.Errorf("missing execution plan")
	}
	if plan.Table != entry.Table || len(plan.Columns) == 0 || len(plan.PrimaryKey) == 0 {
		return fmt.Errorf("incomplete execution plan")
	}
	columnSet := make(map[string]bool, len(plan.Columns))
	for _, column := range plan.Columns {
		if column == "" || columnSet[column] {
			return fmt.Errorf("invalid or duplicate column %q", column)
		}
		columnSet[column] = true
	}
	for _, column := range plan.PrimaryKey {
		if !columnSet[column] {
			return fmt.Errorf("primary key column %q is not allowlisted", column)
		}
	}
	if entry.Direct && !columnSet[entry.SessionColumn] {
		return fmt.Errorf("session column %q is not allowlisted", entry.SessionColumn)
	}
	if !entry.Direct && (entry.ParentTable == "" || plan.ParentColumn == "" || !columnSet[plan.ParentColumn]) {
		return fmt.Errorf("indirect parent plan is incomplete")
	}
	for _, fk := range plan.ForeignKeys {
		if !columnSet[fk.Column] || fk.ReferenceTable == "" || fk.ReferenceColumn == "" {
			return fmt.Errorf("foreign-key plan for %q is incomplete", fk.Column)
		}
		if _, ok := sessionMigrationExecutionPlansV1[fk.ReferenceTable]; !ok {
			return fmt.Errorf("foreign-key reference table %q is not allowlisted", fk.ReferenceTable)
		}
	}
	for _, generated := range plan.GeneratedKeys {
		if !columnSet[generated.Column] || generated.Kind == "" {
			return fmt.Errorf("generated-key plan for %q is incomplete", generated.Column)
		}
	}
	for _, generated := range plan.DatabaseGenerated {
		if !columnSet[generated] {
			return fmt.Errorf("database-generated column %q is not allowlisted", generated)
		}
	}
	for _, semantic := range plan.SemanticReferences {
		if !columnSet[semantic.Column] || semantic.Kind == "" || len(semantic.References) == 0 {
			return fmt.Errorf("semantic reference plan for %q is incomplete", semantic.Column)
		}
		if semantic.TypeColumn != "" && !columnSet[semantic.TypeColumn] {
			return fmt.Errorf("semantic reference discriminator %q is not allowlisted", semantic.TypeColumn)
		}
		for referenceType, reference := range semantic.References {
			referencePlan, ok := sessionMigrationExecutionPlansV1[reference.Table]
			if referenceType == "" || !ok || !sessionMigrationColumnPresent(referencePlan.Columns, reference.Column) {
				return fmt.Errorf("semantic reference %q for %q is invalid", referenceType, semantic.Column)
			}
		}
	}
	switch entry.Policy {
	case SessionMigrationPolicyCopy, SessionMigrationPolicyRetainAudit, SessionMigrationPolicyRegenerate, SessionMigrationPolicyDeleteAfterVerified:
	default:
		return fmt.Errorf("unsupported policy %q", entry.Policy)
	}
	return nil
}

func sessionMigrationColumnPresent(columns []string, wanted string) bool {
	for _, column := range columns {
		if column == wanted {
			return true
		}
	}
	return false
}

// SessionMigrationManifest returns an isolated copy so callers cannot mutate
// the versioned process-wide contract.
func SessionMigrationManifest() []SessionMigrationManifestEntry {
	out := make([]SessionMigrationManifestEntry, len(sessionMigrationManifestV1))
	copy(out, sessionMigrationManifestV1)
	for index := range out {
		out[index].Implemented = sessionMigrationExecutionPlanError(out[index]) == nil
		if len(out[index].RelatedSessionColumns) == 0 {
			continue
		}
		related := make([]SessionMigrationRelatedSessionColumn, len(out[index].RelatedSessionColumns))
		copy(related, out[index].RelatedSessionColumns)
		out[index].RelatedSessionColumns = related
	}
	return out
}

func SessionMigrationMetadataExclusions() []SessionMigrationMetadataExclusion {
	out := make([]SessionMigrationMetadataExclusion, len(sessionMigrationMetadataExclusionsV1))
	for index, entry := range sessionMigrationMetadataExclusionsV1 {
		out[index] = entry
		out[index].SessionColumns = append([]string(nil), entry.SessionColumns...)
	}
	return out
}

func SessionMigrationManifestSummary() (direct, indirect, implemented int) {
	for _, entry := range sessionMigrationManifestV1 {
		if entry.Direct {
			direct++
		} else {
			indirect++
		}
		if sessionMigrationExecutionPlanError(entry) == nil {
			implemented++
		}
	}
	return direct, indirect, implemented
}

func SessionMigrationManifestReleaseBlockers() []string {
	direct, indirect, implemented := SessionMigrationManifestSummary()
	blockers := []string{}
	if implemented != direct+indirect {
		blockers = append(blockers, fmt.Sprintf("manifest_executor_incomplete:%d_of_%d_entries", implemented, direct+indirect))
	}
	for _, entry := range sessionMigrationManifestV1 {
		if err := sessionMigrationExecutionPlanError(entry); err != nil {
			blockers = append(blockers, fmt.Sprintf("manifest_plan_invalid:%s:%s", entry.Table, err.Error()))
		}
	}
	return blockers
}
