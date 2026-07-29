package store

import "fmt"

const (
	SessionMigrationManifestVersion = "session-migration.manifest.v1"

	SessionMigrationPolicyCopy                = "copy"
	SessionMigrationPolicyRetainAudit         = "retain_audit"
	SessionMigrationPolicyRegenerate          = "regenerate"
	SessionMigrationPolicyDeleteAfterVerified = "delete_after_verified"

	SessionMigrationManifestParityUnverifiedReason = "session_migration_manifest_parity_unverified"
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

// SessionMigrationManifest returns an isolated copy so callers cannot mutate
// the versioned process-wide contract.
func SessionMigrationManifest() []SessionMigrationManifestEntry {
	out := make([]SessionMigrationManifestEntry, len(sessionMigrationManifestV1))
	copy(out, sessionMigrationManifestV1)
	for index := range out {
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
		if entry.Implemented {
			implemented++
		}
	}
	return direct, indirect, implemented
}

func SessionMigrationManifestReleaseBlockers() []string {
	direct, indirect, implemented := SessionMigrationManifestSummary()
	return []string{
		SessionMigrationManifestParityUnverifiedReason,
		fmt.Sprintf("manifest_executor_incomplete:%d_of_%d_entries", implemented, direct+indirect),
		"source_target_count_hash_unverified",
		"row_map_fk_remap_unverified",
		"vector_expected_id_parity_unverified",
	}
}
