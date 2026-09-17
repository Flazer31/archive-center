package main

// canonPackStorageSchemaStatements mirrors migrations/002_canon_pack_storage.sql.
// Keeping these statements in the binary lets the compatibility pass repair an
// existing database even when a caller supplies an older/custom base schema.
func canonPackStorageSchemaStatements() []string {
	return splitSQLStatements(canonPackStorageSchemaSQL)
}

const canonPackStorageSchemaSQL = `
CREATE TABLE IF NOT EXISTS reference_work_editions (
    edition_row_id CHAR(36) PRIMARY KEY,
    work_id CHAR(36) NOT NULL,
    stable_work_id VARCHAR(160) NOT NULL,
    edition_id VARCHAR(160) NOT NULL,
    identity_contract VARCHAR(80) NOT NULL DEFAULT 'canon_identity.v1',
    original_language VARCHAR(32) NOT NULL DEFAULT '',
    edition_language VARCHAR(32) NOT NULL DEFAULT '',
    edition_label VARCHAR(500) NOT NULL,
    edition_status VARCHAR(50) NOT NULL DEFAULT 'active',
    metadata_json JSON NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_reference_work_edition_identity (stable_work_id, edition_id),
    INDEX idx_reference_work_edition_local (work_id, edition_status, updated_at),
    CONSTRAINT fk_reference_work_edition_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE RESTRICT,
    CONSTRAINT chk_reference_work_edition_status CHECK (edition_status IN ('active', 'inactive', 'deprecated'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS canon_pack_installs (
    install_id CHAR(36) PRIMARY KEY,
    pack_id VARCHAR(160) NOT NULL,
    pack_version VARCHAR(80) NOT NULL,
    install_generation BIGINT UNSIGNED NOT NULL,
    manifest_contract VARCHAR(80) NOT NULL,
    manifest_sha256 CHAR(64) NOT NULL,
    manifest_json JSON NOT NULL,
    work_id CHAR(36) NOT NULL,
    edition_row_id CHAR(36) NOT NULL,
    pack_status VARCHAR(50) NOT NULL,
    review_status VARCHAR(50) NOT NULL,
    trust_status VARCHAR(50) NOT NULL,
    lifecycle_status VARCHAR(50) NOT NULL DEFAULT 'staged',
    validation_report_json JSON NOT NULL,
    coverage_report_json JSON NOT NULL,
    installed_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    activated_at DATETIME(3) NULL,
    removed_at DATETIME(3) NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    active_generation_marker TINYINT AS (CASE WHEN lifecycle_status = 'active' THEN 1 ELSE NULL END) STORED,
    UNIQUE KEY uq_canon_pack_generation (pack_id, pack_version, install_generation),
    UNIQUE KEY uq_canon_pack_active_slot (pack_id, edition_row_id, active_generation_marker),
    INDEX idx_canon_pack_work_lifecycle (work_id, edition_row_id, lifecycle_status, updated_at),
    CONSTRAINT fk_canon_pack_install_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE RESTRICT,
    CONSTRAINT fk_canon_pack_install_edition FOREIGN KEY (edition_row_id) REFERENCES reference_work_editions(edition_row_id) ON DELETE RESTRICT,
    CONSTRAINT chk_canon_pack_lifecycle CHECK (lifecycle_status IN ('staged', 'active', 'inactive', 'failed', 'removed')),
    CONSTRAINT chk_canon_pack_manifest_sha CHECK (manifest_sha256 REGEXP '^[0-9a-f]{64}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reference_source_observations (
    observation_id CHAR(36) PRIMARY KEY,
    work_id CHAR(36) NOT NULL,
    edition_row_id CHAR(36) NOT NULL,
    continuity_id CHAR(36) NULL,
    origin_kind VARCHAR(50) NOT NULL,
    install_id CHAR(36) NULL,
    source_key VARCHAR(160) NOT NULL,
    source_type VARCHAR(50) NOT NULL,
    source_uri TEXT NULL,
    license_json JSON NOT NULL,
    access_class VARCHAR(50) NOT NULL,
    retrieved_at DATETIME(3) NOT NULL,
    hash_contract VARCHAR(80) NOT NULL DEFAULT 'source_bytes_sha256.v1',
    document_sha256 CHAR(64) NOT NULL,
    document_id CHAR(36) NULL,
    provenance_json JSON NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_reference_source_scope (work_id, edition_row_id, continuity_id, origin_kind),
    INDEX idx_reference_source_hash (hash_contract, document_sha256),
    UNIQUE KEY uq_reference_source_install_key (install_id, source_key),
    CONSTRAINT fk_reference_source_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_source_edition FOREIGN KEY (edition_row_id) REFERENCES reference_work_editions(edition_row_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_source_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_source_install FOREIGN KEY (install_id) REFERENCES canon_pack_installs(install_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_source_document FOREIGN KEY (document_id) REFERENCES reference_documents(document_id) ON DELETE SET NULL,
    CONSTRAINT chk_reference_source_origin CHECK (origin_kind IN ('canon_pack', 'source_discovery', 'user_local', 'legacy_local', 'local_overlay')),
    CONSTRAINT chk_reference_source_install_owner CHECK ((origin_kind = 'canon_pack' AND install_id IS NOT NULL) OR (origin_kind <> 'canon_pack' AND install_id IS NULL)),
    CONSTRAINT chk_reference_source_hash CHECK (document_sha256 REGEXP '^[0-9a-f]{64}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reference_work_titles (
    title_row_id CHAR(36) PRIMARY KEY,
    work_id CHAR(36) NOT NULL,
    edition_row_id CHAR(36) NULL,
    title_kind VARCHAR(50) NOT NULL,
    title_text VARCHAR(500) NOT NULL,
    language_code VARCHAR(32) NOT NULL DEFAULT '',
    script_code VARCHAR(32) NOT NULL DEFAULT '',
    normalization_contract VARCHAR(80) NOT NULL,
    normalized_lookup_key VARCHAR(500) NOT NULL,
    normalized_lookup_digest CHAR(64) NOT NULL,
    source_observation_id CHAR(36) NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    edition_scope_key CHAR(36) NOT NULL DEFAULT '',
    UNIQUE KEY uq_reference_work_title_identity (work_id, edition_scope_key, title_kind, language_code, normalization_contract, normalized_lookup_digest),
    INDEX idx_reference_title_lookup (normalized_lookup_key(180), language_code, title_kind),
    CONSTRAINT fk_reference_title_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_title_edition FOREIGN KEY (edition_row_id) REFERENCES reference_work_editions(edition_row_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_title_source FOREIGN KEY (source_observation_id) REFERENCES reference_source_observations(observation_id) ON DELETE SET NULL,
    CONSTRAINT chk_reference_title_kind CHECK (title_kind IN ('original', 'translated', 'alias')),
    CONSTRAINT chk_reference_title_edition_scope CHECK (
        (edition_row_id IS NULL AND edition_scope_key = '') OR
        (edition_row_id IS NOT NULL AND edition_scope_key = edition_row_id)
    ),
    CONSTRAINT chk_reference_title_digest CHECK (normalized_lookup_digest REGEXP '^[0-9a-f]{64}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reference_item_origins (
    origin_membership_id CHAR(36) PRIMARY KEY,
    work_id CHAR(36) NOT NULL,
    edition_row_id CHAR(36) NOT NULL,
    item_kind VARCHAR(50) NOT NULL,
    node_id CHAR(36) NULL,
    entity_id CHAR(36) NULL,
    claim_id CHAR(36) NULL,
    origin_kind VARCHAR(50) NOT NULL,
    origin_owner_id VARCHAR(160) NOT NULL,
    install_id CHAR(36) NULL,
    source_item_id VARCHAR(160) NOT NULL,
    review_state VARCHAR(50) NOT NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_reference_item_origin (origin_kind, origin_owner_id, item_kind, source_item_id),
    INDEX idx_reference_item_origin_install (install_id, item_kind),
    INDEX idx_reference_item_origin_scope (work_id, edition_row_id, item_kind),
    CONSTRAINT fk_reference_item_origin_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_item_origin_edition FOREIGN KEY (edition_row_id) REFERENCES reference_work_editions(edition_row_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_item_origin_node FOREIGN KEY (node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_item_origin_entity FOREIGN KEY (entity_id) REFERENCES reference_entities(entity_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_item_origin_claim FOREIGN KEY (claim_id) REFERENCES reference_claims(claim_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_item_origin_install FOREIGN KEY (install_id) REFERENCES canon_pack_installs(install_id) ON DELETE RESTRICT,
    CONSTRAINT chk_reference_item_origin_target CHECK (
        (item_kind = 'timeline' AND node_id IS NOT NULL AND entity_id IS NULL AND claim_id IS NULL) OR
        (item_kind = 'entity' AND node_id IS NULL AND entity_id IS NOT NULL AND claim_id IS NULL) OR
        (item_kind = 'claim' AND node_id IS NULL AND entity_id IS NULL AND claim_id IS NOT NULL)
    ),
    CONSTRAINT chk_reference_item_origin_kind CHECK (origin_kind IN ('canon_pack', 'source_discovery', 'user_local', 'legacy_local', 'local_overlay')),
    CONSTRAINT chk_reference_item_origin_install_owner CHECK ((origin_kind = 'canon_pack' AND install_id IS NOT NULL) OR (origin_kind <> 'canon_pack' AND install_id IS NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reference_item_evidence (
    evidence_edge_id CHAR(36) PRIMARY KEY,
    item_kind VARCHAR(50) NOT NULL,
    node_id CHAR(36) NULL,
    entity_id CHAR(36) NULL,
    claim_id CHAR(36) NULL,
    source_observation_id CHAR(36) NOT NULL,
    document_hash_contract VARCHAR(80) NOT NULL,
    document_sha256 CHAR(64) NOT NULL,
    locator_json JSON NOT NULL,
    locator_digest CHAR(64) NOT NULL,
    evidence_state VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_reference_node_evidence (node_id, source_observation_id, document_hash_contract, document_sha256, locator_digest),
    UNIQUE KEY uq_reference_entity_evidence (entity_id, source_observation_id, document_hash_contract, document_sha256, locator_digest),
    UNIQUE KEY uq_reference_claim_evidence (claim_id, source_observation_id, document_hash_contract, document_sha256, locator_digest),
    INDEX idx_reference_evidence_source (source_observation_id, evidence_state),
    CONSTRAINT fk_reference_evidence_node FOREIGN KEY (node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_evidence_entity FOREIGN KEY (entity_id) REFERENCES reference_entities(entity_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_evidence_claim FOREIGN KEY (claim_id) REFERENCES reference_claims(claim_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_evidence_source FOREIGN KEY (source_observation_id) REFERENCES reference_source_observations(observation_id) ON DELETE RESTRICT,
    CONSTRAINT chk_reference_item_evidence_target CHECK (
        (item_kind = 'timeline' AND node_id IS NOT NULL AND entity_id IS NULL AND claim_id IS NULL) OR
        (item_kind = 'entity' AND node_id IS NULL AND entity_id IS NOT NULL AND claim_id IS NULL) OR
        (item_kind = 'claim' AND node_id IS NULL AND entity_id IS NULL AND claim_id IS NOT NULL)
    ),
    CONSTRAINT chk_reference_evidence_hashes CHECK (document_sha256 REGEXP '^[0-9a-f]{64}$' AND locator_digest REGEXP '^[0-9a-f]{64}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reference_logical_facts (
    logical_fact_id CHAR(36) PRIMARY KEY,
    work_id CHAR(36) NOT NULL,
    edition_row_id CHAR(36) NOT NULL,
    continuity_id CHAR(36) NOT NULL,
    applicability_scope_digest CHAR(64) NOT NULL,
    fact_status VARCHAR(50) NOT NULL DEFAULT 'active',
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_reference_logical_fact_scope (work_id, edition_row_id, continuity_id, fact_status),
    CONSTRAINT fk_reference_logical_fact_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_logical_fact_edition FOREIGN KEY (edition_row_id) REFERENCES reference_work_editions(edition_row_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_logical_fact_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE RESTRICT,
    CONSTRAINT chk_reference_logical_fact_scope CHECK (applicability_scope_digest REGEXP '^[0-9a-f]{64}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reference_fact_identities (
    claim_id CHAR(36) PRIMARY KEY,
    fingerprint_contract VARCHAR(80) NOT NULL,
    exact_fingerprint CHAR(64) NOT NULL,
    logical_fact_id CHAR(36) NOT NULL,
    equivalence_status VARCHAR(50) NOT NULL,
    equivalence_basis VARCHAR(80) NOT NULL,
    edition_row_id CHAR(36) NOT NULL,
    continuity_id CHAR(36) NOT NULL,
    applicability_scope_digest CHAR(64) NOT NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_reference_fact_fingerprint (fingerprint_contract, exact_fingerprint),
    INDEX idx_reference_fact_logical (logical_fact_id, equivalence_status),
    CONSTRAINT fk_reference_fact_identity_claim FOREIGN KEY (claim_id) REFERENCES reference_claims(claim_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_fact_identity_logical FOREIGN KEY (logical_fact_id) REFERENCES reference_logical_facts(logical_fact_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_fact_identity_edition FOREIGN KEY (edition_row_id) REFERENCES reference_work_editions(edition_row_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_fact_identity_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE RESTRICT,
    CONSTRAINT chk_reference_fact_identity_hashes CHECK (exact_fingerprint REGEXP '^[0-9a-f]{64}$' AND applicability_scope_digest REGEXP '^[0-9a-f]{64}$'),
    CONSTRAINT chk_reference_fact_equivalence CHECK (equivalence_status IN ('exact', 'verified_equivalent', 'unresolved'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reference_overlay_rules (
    overlay_rule_id CHAR(36) PRIMARY KEY,
    work_id CHAR(36) NOT NULL,
    edition_row_id CHAR(36) NOT NULL,
    target_logical_fact_id CHAR(36) NULL,
    target_entity_id CHAR(36) NULL,
    target_node_id CHAR(36) NULL,
    overlay_action VARCHAR(50) NOT NULL,
    replacement_claim_id CHAR(36) NULL,
    replacement_entity_id CHAR(36) NULL,
    replacement_node_id CHAR(36) NULL,
    rule_status VARCHAR(50) NOT NULL DEFAULT 'active',
    reason_text TEXT NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_reference_overlay_scope (work_id, edition_row_id, rule_status, overlay_action),
    CONSTRAINT fk_reference_overlay_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_overlay_edition FOREIGN KEY (edition_row_id) REFERENCES reference_work_editions(edition_row_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_overlay_target_fact FOREIGN KEY (target_logical_fact_id) REFERENCES reference_logical_facts(logical_fact_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_overlay_target_entity FOREIGN KEY (target_entity_id) REFERENCES reference_entities(entity_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_overlay_target_node FOREIGN KEY (target_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reference_overlay_replacement_claim FOREIGN KEY (replacement_claim_id) REFERENCES reference_claims(claim_id) ON DELETE SET NULL,
    CONSTRAINT fk_reference_overlay_replacement_entity FOREIGN KEY (replacement_entity_id) REFERENCES reference_entities(entity_id) ON DELETE SET NULL,
    CONSTRAINT fk_reference_overlay_replacement_node FOREIGN KEY (replacement_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL,
    CONSTRAINT chk_reference_overlay_action CHECK (overlay_action IN ('supplement', 'override', 'suppress_for_retrieval', 'conflict')),
    CONSTRAINT chk_reference_overlay_status CHECK (rule_status IN ('active', 'inactive', 'dormant')),
    CONSTRAINT chk_reference_overlay_target CHECK (
        ((target_logical_fact_id IS NOT NULL) + (target_entity_id IS NOT NULL) + (target_node_id IS NOT NULL) = 1) OR
        (rule_status = 'dormant' AND target_logical_fact_id IS NULL AND target_entity_id IS NULL AND target_node_id IS NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS source_discovery_jobs (
    job_id CHAR(36) PRIMARY KEY,
    contract_version VARCHAR(80) NOT NULL DEFAULT 'source-discovery-pipeline.v1',
    work_query VARCHAR(500) NOT NULL,
    original_title VARCHAR(500) NOT NULL DEFAULT '',
    language_code VARCHAR(32) NOT NULL DEFAULT '',
    edition_hint VARCHAR(500) NOT NULL DEFAULT '',
    job_state VARCHAR(60) NOT NULL,
    request_json JSON NOT NULL,
    result_json JSON NOT NULL,
    coverage_report_json JSON NOT NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_source_discovery_state (job_state, updated_at),
    CONSTRAINT chk_source_discovery_state CHECK (job_state IN (
        'created','scope_ready','discovering','fetching','extracting','reconciling',
        'coverage_review','ready_for_admission','awaiting_exception_review',
        'insufficient_source_coverage','blocked_by_access_policy','failed','cancelled'
    ))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`
