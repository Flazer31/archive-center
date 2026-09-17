-- Archive Center 3.7-D preparatory safety migration.
--
-- This migration adds durable ledgers required by the exhaustive Go manifest.
-- It does not make source lock or cleanup safe by itself. Runtime keeps those
-- destructive phases blocked until all manifest entries have verified
-- count/hash, row-map/FK, and vector expected-ID parity.

CREATE TABLE IF NOT EXISTS session_route_bindings (
    binding_id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    contract_version              VARCHAR(80)     NOT NULL,
    stable_character_id           VARCHAR(255)    NOT NULL,
    host_chat_id                  VARCHAR(255)    NOT NULL,
    canonical_session_id          VARCHAR(255)    NOT NULL,
    binding_state                 VARCHAR(50)     NOT NULL DEFAULT 'active',
    binding_reason                VARCHAR(100)    NOT NULL,
    redirected_from_session_id    VARCHAR(255)    NULL,
    redirect_migration_id         BIGINT UNSIGNED NULL,
    revision                      BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at                    DATETIME(3)      DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at                    DATETIME(3)      DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_session_route_host_identity (stable_character_id, host_chat_id),
    INDEX idx_session_route_canonical (canonical_session_id(180), binding_state),
    INDEX idx_session_route_redirect (redirect_migration_id),
    CONSTRAINT fk_session_route_redirect_migration
        FOREIGN KEY (redirect_migration_id) REFERENCES session_migrations(id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Durable official-host identity to canonical Archive Center session route with exact readback acknowledgement.';

CREATE TABLE IF NOT EXISTS session_migration_artifact_parity (
    parity_id                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    migration_id             BIGINT UNSIGNED NOT NULL,
    manifest_version         VARCHAR(80)     NOT NULL,
    table_name               VARCHAR(100)    NOT NULL,
    parent_table_name        VARCHAR(100)    NULL,
    session_column_name      VARCHAR(100)    NULL,
    migration_policy         VARCHAR(50)     NOT NULL,
    source_row_count         BIGINT          NULL,
    source_content_hash      CHAR(64)        NULL,
    target_row_count         BIGINT          NULL,
    target_content_hash      CHAR(64)        NULL,
    row_map_expected_count   BIGINT          NULL,
    row_map_verified_count   BIGINT          NULL,
    fk_expected_count        BIGINT          NULL,
    fk_verified_count        BIGINT          NULL,
    vector_expected_count    BIGINT          NULL,
    vector_expected_id_hash  CHAR(64)        NULL,
    vector_actual_count      BIGINT          NULL,
    vector_actual_id_hash    CHAR(64)        NULL,
    parity_state             VARCHAR(50)     NOT NULL DEFAULT 'unverified',
    blocker_code             VARCHAR(120)    NULL,
    verified_at              DATETIME(3)     NULL,
    created_at               DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at               DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_session_migration_manifest_table (migration_id, manifest_version, table_name),
    INDEX idx_session_migration_parity_state (migration_id, parity_state),
    CONSTRAINT fk_session_migration_artifact_parity
        FOREIGN KEY (migration_id) REFERENCES session_migrations(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Per-manifest-table count/hash, row-map, FK, and vector expected-ID parity gate.';

CREATE TABLE IF NOT EXISTS session_migration_vector_expected_ids (
    migration_id       BIGINT UNSIGNED NOT NULL,
    document_id        VARCHAR(255)    NOT NULL,
    source_table       VARCHAR(100)    NOT NULL,
    source_row_id      VARCHAR(255)    NOT NULL,
    observed           BOOLEAN         NOT NULL DEFAULT FALSE,
    observed_at        DATETIME(3)     NULL,
    created_at         DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    PRIMARY KEY (migration_id, document_id),
    INDEX idx_session_migration_vector_observed (migration_id, observed),
    CONSTRAINT fk_session_migration_vector_expected
        FOREIGN KEY (migration_id) REFERENCES session_migrations(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Exact expected Chroma document IDs for migration parity; counts alone are insufficient.';

CREATE TABLE IF NOT EXISTS session_migration_saga_steps (
    step_id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    migration_id    BIGINT UNSIGNED NOT NULL,
    phase            VARCHAR(80)     NOT NULL,
    phase_state      VARCHAR(50)     NOT NULL DEFAULT 'pending',
    attempt_count    INT             NOT NULL DEFAULT 0,
    request_hash     CHAR(64)        NULL,
    result_json      JSON            NULL,
    last_error       TEXT            NULL,
    started_at       DATETIME(3)     NULL,
    completed_at     DATETIME(3)     NULL,
    created_at       DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at       DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_session_migration_saga_phase (migration_id, phase),
    INDEX idx_session_migration_saga_state (phase_state, updated_at),
    CONSTRAINT fk_session_migration_saga_step
        FOREIGN KEY (migration_id) REFERENCES session_migrations(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Recoverable MariaDB/Chroma session migration saga phases; no destructive phase may skip parity.';
