-- Archive Center 4.0-G read-only RisuAI/PocketRisu lorebook snapshots.
--
-- Host lore remains separate from canonical memories, evidence, relationships,
-- entities, world rules, and original-work references. A snapshot row records
-- the observation even when it is partial or unavailable; only an explicitly
-- complete observed snapshot may replace the current entry projection.

CREATE TABLE IF NOT EXISTS lorebook_reference_session_locks (
    chat_session_id         VARCHAR(255) NOT NULL PRIMARY KEY,
    created_at              DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Per-session transaction row serializing exact lorebook scope creation and projection replacement.';

CREATE TABLE IF NOT EXISTS lorebook_reference_scopes (
    scope_id                BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id         VARCHAR(255) NOT NULL,
    character_index         BIGINT NULL,
    chat_index              BIGINT NULL,
    enabled_modules_json    JSON NOT NULL,
    scope_identity_json     JSON NOT NULL,
    created_at              DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_lorebook_scope_session (chat_session_id(180)),
    INDEX idx_lorebook_scope_host (chat_session_id(180), character_index, chat_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Exact observed Host scope for read-only lorebook reference snapshots; module-set equality is verified from JSON.';

CREATE TABLE IF NOT EXISTS lorebook_reference_snapshots (
    snapshot_id             CHAR(32) NOT NULL PRIMARY KEY,
    scope_id                BIGINT UNSIGNED NOT NULL,
    contract_version        VARCHAR(80) NOT NULL,
    consent_state           VARCHAR(40) NOT NULL,
    observation_state       VARCHAR(40) NOT NULL,
    complete_snapshot       BOOLEAN NOT NULL DEFAULT FALSE,
    entry_count             INT UNSIGNED NOT NULL DEFAULT 0,
    provenance_json         JSON NOT NULL,
    observed_at             DATETIME(3) NOT NULL,
    created_at              DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_lorebook_snapshot_scope (scope_id, created_at),
    CONSTRAINT fk_lorebook_snapshot_scope
        FOREIGN KEY (scope_id) REFERENCES lorebook_reference_scopes(scope_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Append-only Host lorebook observation ledger including partial, unavailable, and consent-revoked observations.';

CREATE TABLE IF NOT EXISTS lorebook_reference_entries (
    entry_record_id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    scope_id                BIGINT UNSIGNED NOT NULL,
    snapshot_id             CHAR(32) NOT NULL,
    host_entry_id           VARCHAR(255) NULL,
    entry_ordinal           INT UNSIGNED NOT NULL,
    source_kind             VARCHAR(50) NOT NULL DEFAULT 'current_host_aggregate',
    source_identity         VARCHAR(255) NULL,
    entry_key               LONGTEXT NOT NULL,
    second_key              LONGTEXT NOT NULL,
    entry_comment           LONGTEXT NOT NULL,
    content                 LONGTEXT NOT NULL,
    normalized_search_text  LONGTEXT NOT NULL,
    entry_mode              VARCHAR(40) NULL,
    always_active           BOOLEAN NULL,
    selective               BOOLEAN NULL,
    use_regex               BOOLEAN NULL,
    insert_order            INT NULL,
    activation_percent      DOUBLE NULL,
    book_version            BIGINT NULL,
    folder                  VARCHAR(255) NULL,
    extensions_json         JSON NOT NULL,
    content_hash            CHAR(64) NULL,
    lifecycle_state         VARCHAR(40) NOT NULL DEFAULT 'catalog_current',
    is_current              BOOLEAN NOT NULL DEFAULT TRUE,
    first_seen_at           DATETIME(3) NOT NULL,
    last_seen_at            DATETIME(3) NOT NULL,
    created_at              DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_lorebook_entry_current (scope_id, is_current, lifecycle_state),
    INDEX idx_lorebook_entry_host_id (scope_id, host_entry_id),
    INDEX idx_lorebook_entry_snapshot (snapshot_id),
    CONSTRAINT fk_lorebook_entry_scope
        FOREIGN KEY (scope_id) REFERENCES lorebook_reference_scopes(scope_id)
        ON DELETE CASCADE,
    CONSTRAINT fk_lorebook_entry_snapshot
        FOREIGN KEY (snapshot_id) REFERENCES lorebook_reference_snapshots(snapshot_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Read-only lorebook entry revisions. Content hashes are diagnostic only and never identity or admission gates.';
