-- Accepted-source derivation lifecycle, durable reprocessing, and vector outbox.
-- Branch identity is intentionally nullable/not_exposed because the complete-turn
-- Host observation contract does not expose it.

CREATE TABLE IF NOT EXISTS memory_source_revisions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    contract_version VARCHAR(80) NOT NULL DEFAULT 'memory_source_revision.v1',
    source_revision VARCHAR(160) NOT NULL,
    chat_session_id VARCHAR(255) NOT NULL,
    logical_turn_id VARCHAR(160) NOT NULL,
    turn_index INT NOT NULL,
    source_message_id VARCHAR(255) NULL,
    source_generation_id VARCHAR(255) NULL,
    branch_id VARCHAR(255) NULL,
    branch_state VARCHAR(50) NOT NULL DEFAULT 'not_exposed',
    raw_user_content LONGTEXT NOT NULL,
    raw_assistant_content LONGTEXT NOT NULL,
    combined_content_hash CHAR(64) NOT NULL,
    user_observed_content_hash VARCHAR(255) NULL,
    assistant_observed_content_hash VARCHAR(255) NULL,
    hash_algorithm VARCHAR(80) NOT NULL,
    host_observed_at_ms BIGINT NOT NULL,
    lifecycle_state VARCHAR(50) NOT NULL DEFAULT 'active',
    active_logical_turn_slot VARCHAR(160)
        GENERATED ALWAYS AS (CASE WHEN lifecycle_state = 'active' THEN logical_turn_id ELSE NULL END) PERSISTENT,
    superseded_by_revision VARCHAR(160) NULL,
    invalidation_reason VARCHAR(500) NULL,
    invalidated_at DATETIME(3) NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_memory_source_revision (source_revision),
    UNIQUE KEY uq_memory_source_active_turn (chat_session_id, active_logical_turn_slot),
    INDEX idx_memory_source_logical_turn (chat_session_id(160), logical_turn_id(120), lifecycle_state, host_observed_at_ms),
    INDEX idx_memory_source_turn (chat_session_id(160), turn_index, lifecycle_state),
    INDEX idx_memory_source_generation (chat_session_id(160), source_generation_id(120)),
    CONSTRAINT chk_memory_source_lifecycle CHECK (lifecycle_state IN ('active', 'superseded', 'invalidated', 'deleted')),
    CONSTRAINT chk_memory_source_turn CHECK (turn_index > 0),
    CONSTRAINT chk_memory_source_branch_state CHECK (branch_state IN ('observed', 'not_exposed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS active_logical_turn_slot VARCHAR(160)
        GENERATED ALWAYS AS (CASE WHEN lifecycle_state = 'active' THEN logical_turn_id ELSE NULL END) PERSISTENT;
ALTER TABLE memory_source_revisions
    ADD UNIQUE INDEX IF NOT EXISTS uq_memory_source_active_turn (chat_session_id, active_logical_turn_slot);

CREATE TABLE IF NOT EXISTS memory_derivation_dependencies (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    contract_version VARCHAR(80) NOT NULL DEFAULT 'memory_derivation_dependency.v1',
    chat_session_id VARCHAR(255) NOT NULL,
    source_revision VARCHAR(160) NOT NULL,
    root_source_pointer VARCHAR(255) NOT NULL,
    child_artifact_type VARCHAR(80) NOT NULL,
    child_artifact_id VARCHAR(255) NOT NULL,
    parent_artifact_type VARCHAR(80) NOT NULL,
    parent_artifact_id VARCHAR(255) NOT NULL,
    derivation_version VARCHAR(120) NOT NULL,
    extractor_version VARCHAR(120) NOT NULL,
    index_version VARCHAR(120) NOT NULL,
    lifecycle_state VARCHAR(50) NOT NULL DEFAULT 'active',
    invalidated_at DATETIME(3) NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_memory_derivation_edge (source_revision(120), child_artifact_type, child_artifact_id(120), parent_artifact_type, parent_artifact_id(120), derivation_version(80), extractor_version(80), index_version(80)),
    INDEX idx_memory_derivation_child (chat_session_id(160), child_artifact_type, child_artifact_id(120), lifecycle_state),
    INDEX idx_memory_derivation_parent (chat_session_id(160), parent_artifact_type, parent_artifact_id(120), lifecycle_state),
    INDEX idx_memory_derivation_source (chat_session_id(160), source_revision(120), lifecycle_state),
    CONSTRAINT fk_memory_derivation_source FOREIGN KEY (source_revision) REFERENCES memory_source_revisions(source_revision) ON DELETE RESTRICT,
    CONSTRAINT chk_memory_derivation_lifecycle CHECK (lifecycle_state IN ('active', 'invalidated', 'deleted'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS memory_reprocessing_jobs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    contract_version VARCHAR(80) NOT NULL DEFAULT 'memory_reprocessing_job.v1',
    idempotency_key CHAR(64) NOT NULL,
    chat_session_id VARCHAR(255) NOT NULL,
    source_revision VARCHAR(160) NOT NULL,
    source_contract VARCHAR(120) NOT NULL,
    derivation_version VARCHAR(120) NOT NULL,
    extractor_version VARCHAR(120) NOT NULL,
    index_version VARCHAR(120) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    retry_after DATETIME(3) NULL,
    lease_owner VARCHAR(255) NULL,
    lease_until DATETIME(3) NULL,
    last_error TEXT NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_memory_reprocessing_idempotency (idempotency_key),
    INDEX idx_memory_reprocessing_claim (status, retry_after, lease_until, created_at),
    INDEX idx_memory_reprocessing_source (chat_session_id(160), source_revision(120), status),
    CONSTRAINT fk_memory_reprocessing_source FOREIGN KEY (source_revision) REFERENCES memory_source_revisions(source_revision) ON DELETE RESTRICT,
    CONSTRAINT chk_memory_reprocessing_status CHECK (status IN ('pending', 'leased', 'retryable', 'permanent', 'completed', 'stale_rejected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS memory_vector_outbox (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    contract_version VARCHAR(80) NOT NULL DEFAULT 'memory_vector_outbox.v1',
    operation_key CHAR(64) NOT NULL,
    operation VARCHAR(20) NOT NULL,
    chat_session_id VARCHAR(255) NOT NULL,
    source_revision VARCHAR(160) NOT NULL,
    document_id VARCHAR(255) NOT NULL,
    document_json JSON NULL,
    embedding_ready BOOLEAN NOT NULL DEFAULT FALSE,
    required_source_state VARCHAR(20) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    retry_after DATETIME(3) NULL,
    lease_owner VARCHAR(255) NULL,
    lease_until DATETIME(3) NULL,
    last_error TEXT NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_memory_vector_operation (operation_key),
    INDEX idx_memory_vector_claim (status, embedding_ready, retry_after, lease_until, created_at),
    INDEX idx_memory_vector_source (chat_session_id(160), source_revision(120), status),
    INDEX idx_memory_vector_document (document_id(180), operation, status),
    CONSTRAINT fk_memory_vector_source FOREIGN KEY (source_revision) REFERENCES memory_source_revisions(source_revision) ON DELETE RESTRICT,
    CONSTRAINT chk_memory_vector_operation CHECK (operation IN ('delete', 'upsert')),
    CONSTRAINT chk_memory_vector_required_source CHECK (required_source_state IN ('active', 'inactive')),
    CONSTRAINT chk_memory_vector_status CHECK (status IN ('pending', 'leased', 'retryable', 'permanent', 'completed', 'stale_rejected', 'needs_embedding'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE precise_memory_units MODIFY root_evidence_id BIGINT UNSIGNED NULL;
ALTER TABLE precise_memory_units DROP FOREIGN KEY IF EXISTS fk_precise_memory_root_evidence;
ALTER TABLE precise_memory_units ADD CONSTRAINT fk_precise_memory_root_evidence FOREIGN KEY (root_evidence_id) REFERENCES direct_evidence_records(id) ON DELETE SET NULL;
ALTER TABLE precise_memory_units DROP FOREIGN KEY IF EXISTS fk_precise_memory_actor;
ALTER TABLE precise_memory_units ADD CONSTRAINT fk_precise_memory_actor FOREIGN KEY (actor_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL;
ALTER TABLE precise_memory_units DROP FOREIGN KEY IF EXISTS fk_precise_memory_subject;
ALTER TABLE precise_memory_units ADD CONSTRAINT fk_precise_memory_subject FOREIGN KEY (subject_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL;
ALTER TABLE precise_memory_units DROP FOREIGN KEY IF EXISTS fk_precise_memory_affected;
ALTER TABLE precise_memory_units ADD CONSTRAINT fk_precise_memory_affected FOREIGN KEY (affected_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL;
ALTER TABLE precise_memory_units DROP FOREIGN KEY IF EXISTS fk_precise_memory_location;
ALTER TABLE precise_memory_units ADD CONSTRAINT fk_precise_memory_location FOREIGN KEY (location_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL;
ALTER TABLE precise_memory_units DROP FOREIGN KEY IF EXISTS fk_precise_memory_object;
ALTER TABLE precise_memory_units ADD CONSTRAINT fk_precise_memory_object FOREIGN KEY (object_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL;
ALTER TABLE precise_memory_units DROP FOREIGN KEY IF EXISTS fk_precise_memory_knower;
ALTER TABLE precise_memory_units ADD CONSTRAINT fk_precise_memory_knower FOREIGN KEY (knowledge_holder_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL;
