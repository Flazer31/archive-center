package main

// entityIdentitySchemaStatements mirrors migrations/003_entity_identity_v1.sql.
// The compatibility pass remains additive and can repair an existing 3.5/3.6-A
// database without resetting or renumbering legacy rows.
func entityIdentitySchemaStatements() []string {
	return splitSQLStatements(entityIdentitySchemaSQL)
}

const entityIdentitySchemaSQL = `
CREATE TABLE IF NOT EXISTS entity_identities (
    stable_entity_id       CHAR(36) PRIMARY KEY,
    chat_session_id        VARCHAR(255) NOT NULL,
    identity_namespace     VARCHAR(80) NOT NULL,
    entity_kind            VARCHAR(80) NOT NULL,
    canonical_label        VARCHAR(500) NOT NULL,
    lifecycle_state        VARCHAR(50) NOT NULL DEFAULT 'active',
    review_state           VARCHAR(50) NOT NULL DEFAULT 'needs_review',
    presence_authority     VARCHAR(50) NOT NULL DEFAULT 'unverified',
    occurrence_authority   VARCHAR(50) NOT NULL DEFAULT 'none',
    source_contract        VARCHAR(80) NOT NULL,
    source_revision        VARCHAR(160) NOT NULL,
    source_logical_turn_id VARCHAR(160) NULL,
    source_message_id      VARCHAR(255) NULL,
    source_generation_id   VARCHAR(255) NULL,
    source_content_hash    CHAR(64) NOT NULL,
    source_turn            INT NOT NULL,
    source_index           INT NOT NULL,
    idempotency_key        VARCHAR(255) NOT NULL,
    mapping_revision       BIGINT UNSIGNED NOT NULL DEFAULT 1,
    first_seen_turn        INT NOT NULL,
    last_seen_turn         INT NOT NULL,
    created_at             DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at             DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_entity_identity_source_occurrence (chat_session_id(120), idempotency_key(160)),
    INDEX idx_entity_identity_session (chat_session_id(180), identity_namespace, lifecycle_state),
    INDEX idx_entity_identity_source (chat_session_id(120), source_revision(120), source_turn),
    INDEX idx_entity_identity_review (chat_session_id(180), review_state, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='entity_identity.v1 source-bound stable identities; display labels are not merge keys.';

CREATE TABLE IF NOT EXISTS entity_identity_surfaces (
    surface_id          CHAR(36) PRIMARY KEY,
    stable_entity_id    CHAR(36) NOT NULL,
    chat_session_id     VARCHAR(255) NOT NULL,
    identity_namespace  VARCHAR(80) NOT NULL,
    surface_kind        VARCHAR(50) NOT NULL,
    surface_text        VARCHAR(500) NOT NULL,
    normalized_surface  VARCHAR(255) NOT NULL,
    surface_scope       VARCHAR(80) NOT NULL DEFAULT 'source_turn',
    valid_from_turn     INT NOT NULL,
    valid_to_turn       INT NULL,
    source_contract     VARCHAR(80) NOT NULL,
    source_revision     VARCHAR(160) NOT NULL,
    source_turn         INT NOT NULL,
    source_span_start   INT NULL,
    source_span_end     INT NULL,
    evidence_excerpt    TEXT NULL,
    review_state        VARCHAR(50) NOT NULL DEFAULT 'needs_review',
    idempotency_key     VARCHAR(255) NOT NULL,
    created_at          DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at          DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_entity_surface_source_occurrence (chat_session_id(120), idempotency_key(160)),
    INDEX idx_entity_surface_lookup (chat_session_id(120), identity_namespace, normalized_surface(120)),
    INDEX idx_entity_surface_identity (stable_entity_id, valid_from_turn, valid_to_turn),
    CONSTRAINT fk_entity_surface_identity FOREIGN KEY (stable_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Source-linked names and aliases; lookup candidates only, never automatic identity merges.';

CREATE TABLE IF NOT EXISTS entity_identity_links (
    link_id             CHAR(36) PRIMARY KEY,
    chat_session_id     VARCHAR(255) NOT NULL,
    source_entity_id    CHAR(36) NOT NULL,
    target_entity_id    CHAR(36) NOT NULL,
    link_kind           VARCHAR(80) NOT NULL,
    link_state          VARCHAR(50) NOT NULL DEFAULT 'needs_review',
    evidence_json       JSON NOT NULL,
    mapping_revision    BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at          DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at          DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_entity_identity_link (chat_session_id(120), source_entity_id, target_entity_id, link_kind),
    INDEX idx_entity_identity_link_review (chat_session_id(180), link_state, updated_at),
    CONSTRAINT fk_entity_identity_link_source FOREIGN KEY (source_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT,
    CONSTRAINT fk_entity_identity_link_target FOREIGN KEY (target_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Reviewed split, merge, alias, and cross-namespace links; no implicit promotion.';

CREATE TABLE IF NOT EXISTS entity_identity_artifact_bindings (
    binding_id          CHAR(36) PRIMARY KEY,
    stable_entity_id    CHAR(36) NOT NULL,
    chat_session_id     VARCHAR(255) NOT NULL,
    artifact_kind       VARCHAR(80) NOT NULL,
    artifact_role       VARCHAR(80) NOT NULL,
    artifact_ordinal    INT NOT NULL,
    surface_text        VARCHAR(500) NOT NULL,
    review_state        VARCHAR(50) NOT NULL DEFAULT 'needs_review',
    source_contract     VARCHAR(80) NOT NULL,
    source_revision     VARCHAR(160) NOT NULL,
    source_turn         INT NOT NULL,
    idempotency_key     VARCHAR(255) NOT NULL,
    created_at          DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_entity_artifact_binding_source (chat_session_id(120), idempotency_key(160)),
    INDEX idx_entity_artifact_binding_lookup (chat_session_id(120), source_turn, artifact_kind, artifact_ordinal),
    INDEX idx_entity_artifact_binding_identity (stable_entity_id, source_turn),
    CONSTRAINT fk_entity_artifact_binding_identity FOREIGN KEY (stable_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Parallel stable-ID pointers for legacy entity, KG, and state projections.';

CREATE TABLE IF NOT EXISTS speaker_attributions (
    attribution_id        CHAR(36) PRIMARY KEY,
    chat_session_id       VARCHAR(255) NOT NULL,
    speaker_entity_id     CHAR(36) NOT NULL,
    identity_namespace    VARCHAR(80) NOT NULL,
    source_role           VARCHAR(50) NOT NULL,
    attribution_kind      VARCHAR(80) NOT NULL,
    attribution_state     VARCHAR(50) NOT NULL,
    review_state          VARCHAR(50) NOT NULL,
    confidence            DOUBLE NOT NULL DEFAULT 0,
    source_contract       VARCHAR(80) NOT NULL,
    source_revision       VARCHAR(160) NOT NULL,
    source_logical_turn_id VARCHAR(160) NULL,
    source_message_id     VARCHAR(255) NULL,
    source_generation_id  VARCHAR(255) NULL,
    source_content_hash   CHAR(64) NOT NULL,
    source_turn           INT NOT NULL,
    source_span_start     INT NOT NULL,
    source_span_end       INT NOT NULL,
    evidence_excerpt      TEXT NOT NULL,
    idempotency_key       VARCHAR(255) NOT NULL,
    created_at            DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at            DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_speaker_attribution_source (chat_session_id(120), idempotency_key(160)),
    INDEX idx_speaker_attribution_source (chat_session_id(120), source_revision(120), source_turn),
    INDEX idx_speaker_attribution_review (chat_session_id(180), review_state, updated_at),
    INDEX idx_speaker_attribution_entity (speaker_entity_id, source_turn),
    CONSTRAINT fk_speaker_attribution_identity FOREIGN KEY (speaker_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Grounded in-world speaker spans; ambiguous speakers remain needs_review.';
`
