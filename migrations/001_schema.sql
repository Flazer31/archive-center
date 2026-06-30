-- Archive Center 2.0 ? Canonical Truth Schema (R0)
-- Engine: InnoDB, Charset: utf8mb4, Collation: utf8mb4_unicode_ci
-- Status: DRAFT ? dry-run only until explicit approval.
-- Reference: contracts/mariadb-truth-schema-plan.md

SET NAMES utf8mb4;

-- ---------------------------------------------------------------------------
-- 1. chat_logs
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS chat_logs (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    turn_index      INT             NOT NULL,
    role            VARCHAR(50)     NOT NULL,
    content         LONGTEXT        NOT NULL,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turn (chat_session_id, turn_index),
    INDEX idx_session_role (chat_session_id, role)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: every turn starts here. Append-only.';

-- ---------------------------------------------------------------------------
-- 2. effective_input_logs
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS effective_input_logs (
    id               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id  VARCHAR(255)    NOT NULL,
    turn_index       INT             NOT NULL,
    effective_input  LONGTEXT        NOT NULL,
    created_at       DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turn (chat_session_id, turn_index),
    INDEX idx_session (chat_session_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: user intent after preprocessing. Append-only.';

-- ---------------------------------------------------------------------------
-- 3. memories
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS memories (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id         VARCHAR(255)    NOT NULL,
    turn_index              INT             NOT NULL,
    summary_json            JSON,
    embedding               JSON            COMMENT 'JSON float array',
    embedding_model         VARCHAR(255),
    importance              DOUBLE,
    emotional_boost         DOUBLE,
    evidence                JSON,
    emotional_intensity     DOUBLE,
    narrative_significance  DOUBLE,
    place_wing              VARCHAR(255),
    place_room              VARCHAR(255),
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turn (chat_session_id, turn_index),
    INDEX idx_importance (chat_session_id, importance),
    INDEX idx_wing_room (chat_session_id, place_wing, place_room)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: core retrieval index. Append-only.';

-- ---------------------------------------------------------------------------
-- 4. direct_evidence_records
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS direct_evidence_records (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id         VARCHAR(255)    NOT NULL,
    evidence_kind           VARCHAR(100)    NOT NULL DEFAULT 'fact_event',
    evidence_text           LONGTEXT        NOT NULL,
    source_turn_start       INT             NOT NULL,
    source_turn_end         INT             NOT NULL,
    turn_anchor             INT             NULL,
    source_message_ids_json JSON,
    source_hash             VARCHAR(255),
    archive_state           VARCHAR(50)     NOT NULL DEFAULT 'pending_capture',
    capture_stage           VARCHAR(50)     NOT NULL DEFAULT 'critic_extract',
    capture_verification    VARCHAR(50)     NOT NULL DEFAULT 'pending',
    committed_gate          VARCHAR(50),
    lineage_json            JSON,
    repair_needed           BOOLEAN         NOT NULL DEFAULT FALSE,
    tombstoned              BOOLEAN         NOT NULL DEFAULT FALSE,
    superseded_by_id        BIGINT UNSIGNED NULL,
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_state (chat_session_id, archive_state),
    INDEX idx_session_kind (chat_session_id, evidence_kind),
    INDEX idx_source_turn (chat_session_id, source_turn_start, source_turn_end)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: verified facts. Append-only with state transition tracked via new audit rows.';

-- ---------------------------------------------------------------------------
-- 5. kg_triples
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS kg_triples (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    subject         VARCHAR(255)    NOT NULL,
    predicate       VARCHAR(255)    NOT NULL,
    object          VARCHAR(255)    NOT NULL,
    valid_from      INT             NULL,
    valid_to        INT             NULL,
    source_turn     INT             NULL,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_spo (chat_session_id(180), subject(180), predicate(180), object(180)),
    INDEX idx_valid (chat_session_id, valid_from, valid_to)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: knowledge graph edges. Append-only with soft-delete (valid_to).';

-- ---------------------------------------------------------------------------
-- 6. audit_logs
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_logs (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    event_type      VARCHAR(100)    NOT NULL,
    chat_session_id VARCHAR(255)    NULL,
    target_type     VARCHAR(100)    NULL,
    target_id       BIGINT UNSIGNED NULL,
    summary         TEXT,
    details_json    JSON,
    source          VARCHAR(50)     DEFAULT 'api',
    INDEX idx_created (created_at),
    INDEX idx_event (event_type, created_at),
    INDEX idx_session_event (chat_session_id, event_type, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: audit trail. Append-only by definition.';

-- ---------------------------------------------------------------------------
-- 7. critic_feedback
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS critic_feedback (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    chat_session_id VARCHAR(255)    NOT NULL,
    target_type     VARCHAR(50)     NOT NULL,
    target_id       BIGINT UNSIGNED NOT NULL,
    feedback_value  VARCHAR(20)     NOT NULL,
    feedback_note   TEXT,
    source          VARCHAR(50)     DEFAULT 'manual_ui',
    INDEX idx_session_target (chat_session_id, target_type, target_id),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: human/operator feedback on truth. Append-only.';

-- ---------------------------------------------------------------------------
-- 7b. persona_memory_capsules
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS persona_memory_capsules (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    persona_key             VARCHAR(255)    NOT NULL,
    source_chat_session_id  VARCHAR(255)    NOT NULL,
    source_character_name   VARCHAR(255),
    title                   VARCHAR(255)    NOT NULL,
    mode                    VARCHAR(80)     NOT NULL DEFAULT 'manual',
    summary                 LONGTEXT,
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_persona_key (persona_key),
    INDEX idx_source_session (source_chat_session_id),
    INDEX idx_capsule_updated (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Persona recollection: portable protagonist memories, support-only in target sessions.';

-- ---------------------------------------------------------------------------
-- 7c. persona_memory_entries
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS persona_memory_entries (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    capsule_id          BIGINT UNSIGNED NOT NULL,
    source_memory_type  VARCHAR(80)     NULL,
    source_memory_id    BIGINT UNSIGNED NULL,
    source_turn_index   INT             NULL,
    memory_text         LONGTEXT        NOT NULL,
    emotional_weight    DOUBLE          NULL,
    importance_10       DOUBLE          NULL,
    portability         VARCHAR(80)     NOT NULL DEFAULT 'same_chat',
    tags_json           JSON,
    evidence_excerpt    TEXT,
    injection_policy    VARCHAR(80)     NOT NULL DEFAULT 'support_only',
    created_at          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_capsule_turn (capsule_id, source_turn_index),
    INDEX idx_source_memory_ref (source_memory_type, source_memory_id),
    CONSTRAINT fk_persona_memory_entries_capsule
        FOREIGN KEY (capsule_id) REFERENCES persona_memory_capsules(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Persona recollection entries. source_memory_* may point to subjective entity memories, snapshot text remains as fallback.';

-- ---------------------------------------------------------------------------
-- 7d. protagonist_entity_memories
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS protagonist_entity_memories (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    persona_entity_key      VARCHAR(255)    NOT NULL,
    persona_entity_name     VARCHAR(255)    NOT NULL,
    owner_entity_key        VARCHAR(255)    NOT NULL DEFAULT '',
    owner_entity_name       VARCHAR(255)    NOT NULL DEFAULT '',
    owner_entity_role       VARCHAR(80)     NOT NULL DEFAULT 'protagonist',
    owner_visibility        VARCHAR(80)     NOT NULL DEFAULT 'player_known',
    source_chat_session_id  VARCHAR(255)    NOT NULL,
    source_character_name   VARCHAR(255),
    source_turn_index       INT             NULL,
    memory_text             LONGTEXT        NOT NULL,
    evidence_excerpt        TEXT,
    secret_guard            BOOLEAN         NOT NULL DEFAULT FALSE,
    portability             VARCHAR(80)     NOT NULL DEFAULT 'portable_persona_recollection',
    target_reveal_policy    VARCHAR(120)    NOT NULL DEFAULT 'requires_explicit_attachment',
    tags_json               JSON,
    importance_10           DOUBLE          NULL,
    emotional_weight        DOUBLE          NULL,
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_entity_source (persona_entity_key, source_chat_session_id, source_turn_index),
    INDEX idx_owner_source (owner_entity_key, source_chat_session_id, source_turn_index),
    INDEX idx_owner_visibility (owner_entity_key, owner_entity_role, owner_visibility),
    INDEX idx_entity_updated (persona_entity_key, updated_at),
    INDEX idx_source_session (source_chat_session_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Subjective entity memory bank. Source for later support-only persona/NPC capsules.';

-- ---------------------------------------------------------------------------
-- 7e. persona_capsule_attachments
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS persona_capsule_attachments (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    capsule_id              BIGINT UNSIGNED NOT NULL,
    target_chat_session_id  VARCHAR(255)    NOT NULL,
    injection_mode          VARCHAR(80)     NOT NULL DEFAULT 'subtle_deja_vu',
    enabled                 BOOLEAN         NOT NULL DEFAULT TRUE,
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_persona_capsule_attachment (capsule_id, target_chat_session_id(180)),
    INDEX idx_persona_attachment_target (target_chat_session_id, enabled),
    CONSTRAINT fk_persona_capsule_attachments_capsule
        FOREIGN KEY (capsule_id) REFERENCES persona_memory_capsules(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Target-session enablement for support-only persona recollection injection.';

-- ---------------------------------------------------------------------------
-- 8. character_events
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS character_events (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    character_name  VARCHAR(255)    NOT NULL,
    turn_index      INT             NULL,
    event_type      VARCHAR(100)    NOT NULL,
    details_json    JSON,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_char (chat_session_id, character_name),
    INDEX idx_session_turn (chat_session_id, turn_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: character change events. Append-only.';

-- ---------------------------------------------------------------------------
-- 8b. entities
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS entities (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    name            VARCHAR(255)    NOT NULL,
    entity_type     VARCHAR(100),
    description     TEXT,
    aliases_json    JSON,
    first_seen_turn INT,
    last_seen_turn  INT,
    confidence      DOUBLE,
    pinned          BOOLEAN         NOT NULL DEFAULT FALSE,
    suppressed      BOOLEAN         NOT NULL DEFAULT FALSE,
    user_corrected  BOOLEAN         NOT NULL DEFAULT FALSE,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_name (chat_session_id, name),
    INDEX idx_session_type (chat_session_id, entity_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: extracted named entities. Append-only snapshots.';

-- ---------------------------------------------------------------------------
-- 8c. trust_states
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS trust_states (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    target_name     VARCHAR(255)    NOT NULL,
    target_type     VARCHAR(100),
    score           DOUBLE,
    reason_json     JSON,
    source_turn     INT,
    pinned          BOOLEAN         NOT NULL DEFAULT FALSE,
    suppressed      BOOLEAN         NOT NULL DEFAULT FALSE,
    user_corrected  BOOLEAN        NOT NULL DEFAULT FALSE,
    created_at      DATETIME(3)    DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at      DATETIME(3)    DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_target (chat_session_id, target_name),
    INDEX idx_session_turn (chat_session_id, source_turn)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: trust and relationship confidence snapshots.';


-- ---------------------------------------------------------------------------
-- 9. storylines
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS storylines (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id         VARCHAR(255)    NOT NULL,
    name                    VARCHAR(255)    NOT NULL,
    status                  VARCHAR(50)     NOT NULL DEFAULT 'active',
    entities_json           JSON,
    current_context         TEXT,
    key_points_json         JSON,
    ongoing_tensions_json   JSON,
    confidence              DOUBLE,
    evidence_count          INT,
    last_evidence_turn      INT,
    first_turn              INT,
    last_turn               INT,
    pinned                  BOOLEAN         NOT NULL DEFAULT FALSE,
    suppressed              BOOLEAN         NOT NULL DEFAULT FALSE,
    user_corrected          BOOLEAN         NOT NULL DEFAULT FALSE,
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_status (chat_session_id, status),
    INDEX idx_session_last_turn (chat_session_id, last_turn)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: storyline registry. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 9b. guidance_plan_states
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS guidance_plan_states (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    story_plan_json LONGTEXT,
    director_json   LONGTEXT,
    state_status    VARCHAR(50)     NOT NULL DEFAULT 'empty',
    last_turn       INT             NOT NULL DEFAULT -1,
    warnings_json   LONGTEXT,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_guidance_plan_session (chat_session_id(180)),
    INDEX idx_guidance_plan_updated (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: K-2 persistent story plan and director guidance cache.';

-- ---------------------------------------------------------------------------
-- 10. world_rules
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS world_rules (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    scope           VARCHAR(100)    NOT NULL,
    scope_name      VARCHAR(255),
    category        VARCHAR(100)    NOT NULL,
    `key`           VARCHAR(255)    NOT NULL,
    value_json      JSON,
    genre           VARCHAR(100),
    source_turn     INT,
    pinned          BOOLEAN         NOT NULL DEFAULT FALSE,
    suppressed      BOOLEAN         NOT NULL DEFAULT FALSE,
    user_corrected  BOOLEAN         NOT NULL DEFAULT FALSE,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_scope (chat_session_id(180), scope, category, `key`(180))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: world rules and constraints. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 10b. session_active_scopes
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS session_active_scopes (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255) NOT NULL,
    active_scope    VARCHAR(50)  NOT NULL DEFAULT 'root',
    scope_name      VARCHAR(500),
    updated_at      DATETIME(3)  DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_session_active_scope (chat_session_id(180))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Derived: current active world-rule scope per session.';

-- ---------------------------------------------------------------------------
-- 11. character_states
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS character_states (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    character_name  VARCHAR(255)    NOT NULL,
    appearance_json JSON,
    personality_json JSON,
    status_json     JSON,
    relationships_json JSON,
    speech_style_json JSON,
    turn_index      INT,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_char (chat_session_id, character_name),
    INDEX idx_session_turn (chat_session_id, turn_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: character state snapshots. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 12. pending_threads
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pending_threads (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    thread_key      VARCHAR(255)    NOT NULL,
    description     TEXT,
    status          VARCHAR(50)     NOT NULL DEFAULT 'open',
    created_turn    INT,
    resolved_turn   INT,
    source_turn     INT,
    priority        INT,
    hook_type       VARCHAR(50),
    hook_metadata_json JSON,
    pinned          BOOLEAN         NOT NULL DEFAULT FALSE,
    suppressed      BOOLEAN         NOT NULL DEFAULT FALSE,
    user_corrected  BOOLEAN         NOT NULL DEFAULT FALSE,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_status (chat_session_id, status),
    INDEX idx_session_source_turn (chat_session_id, source_turn)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: continuity hooks / pending threads. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 13. active_states
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS active_states (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id VARCHAR(255)    NOT NULL,
    state_type      VARCHAR(100)    NOT NULL,
    content         LONGTEXT        NOT NULL,
    turn_index      INT             NOT NULL,
    created_at      DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_type (chat_session_id, state_type, turn_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: active state snapshots per turn. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 14. canonical_state_layers
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS canonical_state_layers (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id     VARCHAR(255)    NOT NULL,
    layer_type          VARCHAR(100)    NOT NULL,
    content             LONGTEXT        NOT NULL,
    source_state_type   VARCHAR(100),
    turn_index          INT             NOT NULL,
    source_turn         INT,
    source_record       BIGINT UNSIGNED,
    last_verified_turn  INT,
    confidence          DOUBLE,
    created_at          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_type (chat_session_id, layer_type, turn_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: verified state layers. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 15. episode_summaries
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS episode_summaries (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id         VARCHAR(255)    NOT NULL,
    from_turn               INT             NOT NULL,
    to_turn                 INT             NOT NULL,
    summary_text            LONGTEXT        NOT NULL,
    key_entities            JSON,
    key_events              JSON,
    open_loops_json         JSON,
    relationship_changes_json JSON,
    embedding_vector        JSON,
    embedding_model         VARCHAR(255),
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turns (chat_session_id, from_turn, to_turn)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: episode summaries. Read-heavy in R1.';


-- ---------------------------------------------------------------------------
-- 16. chapter_summaries
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS chapter_summaries (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id         VARCHAR(255)    NOT NULL,
    from_turn               INT             NOT NULL,
    to_turn                 INT             NOT NULL,
    chapter_index           INT             NOT NULL DEFAULT 0,
    chapter_title           VARCHAR(500),
    summary_text            LONGTEXT        NOT NULL,
    open_loops_json         JSON,
    relationship_changes_json JSON,
    world_changes_json      JSON,
    callback_candidates_json JSON,
    resume_text             LONGTEXT,
    embedding_vector        JSON,
    embedding_model         VARCHAR(255),
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turns (chat_session_id, from_turn, to_turn),
    INDEX idx_session_chapter (chat_session_id, chapter_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: chapter summaries. Read-heavy in R1.';


-- ---------------------------------------------------------------------------
-- 17. arc_summaries
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS arc_summaries (
    id                          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id             VARCHAR(255)    NOT NULL,
    from_turn                   INT             NOT NULL,
    to_turn                     INT             NOT NULL,
    arc_index                   INT             NOT NULL DEFAULT 0,
    arc_name                    VARCHAR(500),
    arc_status                  VARCHAR(50)     NOT NULL DEFAULT 'active',
    core_conflict               LONGTEXT,
    key_turning_points_json     JSON,
    active_promises_json        JSON,
    unresolved_debts_json       JSON,
    resolved_payoffs_json       JSON,
    callback_candidates_json    JSON,
    future_payoff_candidates_json JSON,
    irreversible_turns_json     JSON,
    callback_debts_json         JSON,
    relationship_pivots_json    JSON,
    arc_resume_text             LONGTEXT,
    embedding_vector            JSON,
    embedding_model             VARCHAR(255),
    created_at                  DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turns (chat_session_id, from_turn, to_turn),
    INDEX idx_session_arc (chat_session_id, arc_index),
    INDEX idx_session_status (chat_session_id, arc_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: arc summaries. Read-heavy in R1.';


-- ---------------------------------------------------------------------------
-- 18. saga_digests
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS saga_digests (
    id                          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id             VARCHAR(255)    NOT NULL,
    from_turn                   INT             NOT NULL,
    to_turn                     INT             NOT NULL,
    era_label                   VARCHAR(500),
    saga_summary                LONGTEXT        NOT NULL,
    persistent_facts_json       JSON,
    never_drop_candidates_json  JSON,
    resume_pack_text            LONGTEXT,
    embedding_vector            JSON,
    embedding_model             VARCHAR(255),
    created_at                  DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turns (chat_session_id, from_turn, to_turn),
    INDEX idx_session_created (chat_session_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: saga digests. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 17. arc_summaries
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS arc_summaries (
    id                          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id             VARCHAR(255)    NOT NULL,
    from_turn                   INT             NOT NULL,
    to_turn                     INT             NOT NULL,
    arc_index                   INT             NOT NULL DEFAULT 0,
    arc_name                    VARCHAR(500),
    arc_status                  VARCHAR(50)     NOT NULL DEFAULT 'active',
    core_conflict               LONGTEXT,
    key_turning_points_json     JSON,
    active_promises_json        JSON,
    unresolved_debts_json       JSON,
    resolved_payoffs_json       JSON,
    callback_candidates_json    JSON,
    future_payoff_candidates_json JSON,
    irreversible_turns_json     JSON,
    callback_debts_json         JSON,
    relationship_pivots_json    JSON,
    arc_resume_text             LONGTEXT,
    embedding_vector            JSON,
    embedding_model             VARCHAR(255),
    created_at                  DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turns     (chat_session_id, from_turn, to_turn),
    INDEX idx_session_status    (chat_session_id, arc_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: arc summaries. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 18. saga_digests
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS saga_digests (
    id                      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id         VARCHAR(255)    NOT NULL,
    from_turn               INT             NOT NULL,
    to_turn                 INT             NOT NULL,
    era_label               VARCHAR(500),
    saga_summary            LONGTEXT,
    persistent_facts_json   JSON,
    never_drop_candidates_json JSON,
    resume_pack_text        LONGTEXT,
    embedding_vector        JSON,
    embedding_model         VARCHAR(255),
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turns (chat_session_id, from_turn, to_turn)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical: saga digests. Read-heavy in R1.';

-- ---------------------------------------------------------------------------
-- 19. session_migrations
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS session_migrations (
    id                                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    source_session_id                   VARCHAR(255)    NOT NULL,
    target_session_id                   VARCHAR(255)    NOT NULL,
    mode                                VARCHAR(80)     NOT NULL DEFAULT 'copy_then_lock_source',
    status                              VARCHAR(50)     NOT NULL DEFAULT 'previewed',
    preview_hash                        VARCHAR(128)    NULL,
    operator_note                       TEXT,
    counts_json                         JSON,
    chroma_reindexed_count              INT             NOT NULL DEFAULT 0,
    errors_json                         JSON,
    started_at                          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    completed_at                        DATETIME(3)     NULL,
    locked_at                           DATETIME(3)     NULL,
    cleanup_at                          DATETIME(3)     NULL,
    created_at                          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at                          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_migration_source (source_session_id, status),
    INDEX idx_session_migration_target (target_session_id, status),
    INDEX idx_session_migration_status (status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Session complete migration ledger. Tracks preview, copy, vector reindex, source lock, cleanup, and rollback state.';

-- ---------------------------------------------------------------------------
-- 20. session_migration_row_map
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS session_migration_row_map (
    id                BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    migration_id      BIGINT UNSIGNED NOT NULL,
    table_name        VARCHAR(100)    NOT NULL,
    source_row_id     BIGINT UNSIGNED NOT NULL,
    target_row_id     BIGINT UNSIGNED NULL,
    row_status        VARCHAR(50)     NOT NULL DEFAULT 'copied',
    created_at        DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_session_migration_row (migration_id, table_name, source_row_id),
    INDEX idx_session_migration_target_row (table_name, target_row_id),
    CONSTRAINT fk_session_migration_row_map_migration
        FOREIGN KEY (migration_id) REFERENCES session_migrations(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Session complete migration row provenance. Maps source rows to copied target rows for verification and rollback.';

-- ---------------------------------------------------------------------------
-- 21. session_migration_locks
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS session_migration_locks (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    migration_id        BIGINT UNSIGNED NOT NULL,
    source_session_id   VARCHAR(255)    NOT NULL,
    target_session_id   VARCHAR(255)    NOT NULL,
    locked              BOOLEAN         NOT NULL DEFAULT TRUE,
    lock_status         VARCHAR(50)     NOT NULL DEFAULT 'migrated_away',
    reason              TEXT,
    locked_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    unlocked_at         DATETIME(3)     NULL,
    created_at          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_migration_lock_source (source_session_id, locked),
    INDEX idx_session_migration_lock_target (target_session_id, locked),
    INDEX idx_session_migration_lock_status (lock_status, updated_at),
    CONSTRAINT fk_session_migration_locks_migration
        FOREIGN KEY (migration_id) REFERENCES session_migrations(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Session complete migration source lock. Prevents abandoned source sessions from acting as live memory owners.';

-- ---------------------------------------------------------------------------
-- 23. consequence_records
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS consequence_records (
    id                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id           VARCHAR(255)    NOT NULL,
    source_turn_start         INT             NOT NULL,
    source_turn_end           INT             NOT NULL,
    decision                  VARCHAR(500)    NOT NULL,
    immediate_result          VARCHAR(500)    NOT NULL,
    delayed_effect            VARCHAR(500)    NOT NULL,
    affected_relations        JSON,
    affected_world            JSON,
    status                    VARCHAR(50)     NOT NULL DEFAULT 'active',
    importance                DOUBLE          NOT NULL DEFAULT 0,
    confidence                DOUBLE          NOT NULL DEFAULT 0,
    foreground_eligible       BOOLEAN         NOT NULL DEFAULT FALSE,
    quiet_turns               INT             NOT NULL DEFAULT 0,
    last_seen_turn            INT             NULL,
    paid_turn                 INT             NULL,
    expires_after_quiet_turns INT             NOT NULL DEFAULT 20,
    source_hash               VARCHAR(255)    NULL,
    evidence_json             JSON,
    created_at                DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at                DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_status (chat_session_id, status),
    INDEX idx_session_source_turn (chat_session_id, source_turn_start, source_turn_end),
    INDEX idx_session_foreground (chat_session_id, foreground_eligible, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Support-only: decision -> immediate result -> delayed effect chains. Not a canonical truth writer.';

-- ---------------------------------------------------------------------------
-- 23-2. psychology_branches
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS psychology_branches (
    id                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id           VARCHAR(255)    NOT NULL,
    character_name            VARCHAR(255)    NOT NULL DEFAULT '',
    branch_type               VARCHAR(50)     NOT NULL,
    axis_name                 VARCHAR(255)    NOT NULL,
    summary                   VARCHAR(1000)   NOT NULL,
    status                    VARCHAR(50)     NOT NULL DEFAULT 'active',
    confidence                DOUBLE          NOT NULL DEFAULT 0,
    confidence_label          VARCHAR(20)     NULL,
    source_kind               VARCHAR(100)    NULL,
    source_turn_start         INT             NOT NULL,
    source_turn_end           INT             NOT NULL,
    source_hash               VARCHAR(255)    NULL,
    evidence_json             JSON,
    quiet_turns               INT             NOT NULL DEFAULT 0,
    last_seen_turn            INT             NULL,
    dormant_after_quiet_turns INT             NOT NULL DEFAULT 15,
    created_at                DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at                DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_status (chat_session_id, status),
    INDEX idx_session_type (chat_session_id, branch_type),
    INDEX idx_session_character (chat_session_id, character_name),
    INDEX idx_session_dormancy (chat_session_id, status, quiet_turns)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Support-only: long-running character motivation axes. Never canonical truth about user action, feeling, consent, or choice.';

-- ---------------------------------------------------------------------------
-- 23-3. session_fork_lineage
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS session_fork_lineage (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id       VARCHAR(255)    NOT NULL,
    scope_id              VARCHAR(255)    NULL,
    parent_scope_id       VARCHAR(255)    NULL,
    copied_from_scope_id  VARCHAR(255)    NULL,
    copied_from_session_id VARCHAR(255)   NULL,
    imported_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    divergence_marker     JSON,
    provenance_source     VARCHAR(100)    NOT NULL DEFAULT 'manual',
    inheritance_mode      VARCHAR(100)    NOT NULL DEFAULT 'conservative_import',
    inherited_items_json  JSON,
    created_at            DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at            DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session (chat_session_id),
    INDEX idx_scope (scope_id),
    INDEX idx_parent_scope (parent_scope_id),
    INDEX idx_copied_from_scope (copied_from_scope_id),
    INDEX idx_provenance (provenance_source, imported_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Support-only: copied/forked session lineage and provenance. Inherited items are review-safe support surfaces, not canonical truth writers.';

-- ---------------------------------------------------------------------------
-- 23-4. theme_offscreen_carries
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS theme_offscreen_carries (
    id                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id           VARCHAR(255)    NOT NULL,
    surface_type              VARCHAR(50)     NOT NULL,
    label                     VARCHAR(255)   NOT NULL,
    summary                   VARCHAR(1000)   NOT NULL,
    status                    VARCHAR(50)     NOT NULL DEFAULT 'active',
    confidence                DOUBLE          NOT NULL DEFAULT 0,
    confidence_label          VARCHAR(20)     NULL,
    source_kind               VARCHAR(100)    NULL,
    source_turn_start         INT             NOT NULL,
    source_turn_end           INT             NOT NULL,
    source_hash               VARCHAR(255)    NULL,
    evidence_json             JSON,
    quiet_turns               INT             NOT NULL DEFAULT 0,
    last_seen_turn            INT             NULL,
    dormant_after_quiet_turns INT             NOT NULL DEFAULT 15,
    foreground_eligible       BOOLEAN         NOT NULL DEFAULT FALSE,
    foreground_reason_json    JSON,
    created_at                DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at                DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_type        (chat_session_id, surface_type),
    INDEX idx_session_status      (chat_session_id, status),
    INDEX idx_session_dormancy    (chat_session_id, status, quiet_turns),
    INDEX idx_session_foreground  (chat_session_id, surface_type, foreground_eligible, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Support-only: recurring theme/motif traces and offscreen world progression/carryover. Not canonical world facts.';


-- ---------------------------------------------------------------------------
-- 23-5. capture_verification_records
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS capture_verification_records (
    id                         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id            VARCHAR(255)    NOT NULL,
    turn_index                 INT             NOT NULL,
    stage_name                 VARCHAR(50)     NOT NULL,
    verification_state         VARCHAR(50)     NOT NULL DEFAULT 'single-stage',
    degraded_reason            VARCHAR(500)    NULL,
    compact_metadata_json      JSON,
    content_hash               VARCHAR(255)    NULL,
    evidence_json              JSON,
    previous_record_id         BIGINT UNSIGNED NULL,
    repaired_by_record_id      BIGINT UNSIGNED NULL,
    repair_attempt_count       INT             NOT NULL DEFAULT 0,
    repair_evidence_json       JSON,
    repaired_at                DATETIME(3)     NULL,
    user_input_preserved       BOOLEAN         NOT NULL DEFAULT TRUE,
    payload_rewrite            BOOLEAN         NOT NULL DEFAULT FALSE,
    created_at                 DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at                 DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_session_turn       (chat_session_id, turn_index),
    INDEX idx_session_stage      (chat_session_id, stage_name),
    INDEX idx_session_state      (chat_session_id, verification_state),
    INDEX idx_previous_record    (previous_record_id),
    INDEX idx_repaired_by        (repaired_by_record_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Support-only: per-turn capture integrity verification across streaming/finalize/recovery stages. Stores compact metadata and hashes first, not raw payloads.';


-- ---------------------------------------------------------------------------
-- status_schema_proposals
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS status_schema_proposals (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id     VARCHAR(255)    NOT NULL,
    input_channel       VARCHAR(50)     NOT NULL DEFAULT 'bootstrap',
    proposal_state      VARCHAR(50)     NOT NULL DEFAULT 'pending_review',
    schema_name         VARCHAR(255)    NOT NULL,
    ruleset_label       VARCHAR(255)    NULL,
    schema_json         JSON            NOT NULL,
    provenance_json     JSON            NULL,
    review_note         TEXT            NULL,
    reviewer            VARCHAR(255)    NULL,
    reviewed_at         DATETIME(3)     NULL,
    created_at          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at          DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_status_schema_session       (chat_session_id, updated_at),
    INDEX idx_status_schema_state         (chat_session_id, proposal_state, updated_at),
    INDEX idx_status_schema_input_channel (chat_session_id, input_channel, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Proposal-only status/stat schema input channel. Reviewable input records, not canonical value/effect writers.';


-- ---------------------------------------------------------------------------
-- status_schema_registry
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS status_schema_registry (
    id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id      VARCHAR(255)    NOT NULL,
    source_proposal_id   BIGINT UNSIGNED NULL,
    schema_name          VARCHAR(255)    NOT NULL DEFAULT 'status_schema',
    ruleset_label        VARCHAR(255)    NULL,
    status_key           VARCHAR(255)    NOT NULL,
    label                VARCHAR(255)    NOT NULL,
    owner_scope          VARCHAR(80)     NOT NULL,
    value_kind           VARCHAR(80)     NOT NULL,
    bounds_json          JSON            NULL,
    options_json         JSON            NULL,
    default_value_json   JSON            NULL,
    registry_state       VARCHAR(50)     NOT NULL DEFAULT 'active',
    created_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_status_registry_key (chat_session_id(180), schema_name(120), status_key(120), owner_scope),
    INDEX idx_status_registry_session (chat_session_id, registry_state, status_key),
    INDEX idx_status_registry_proposal (source_proposal_id),
    CONSTRAINT fk_status_registry_proposal FOREIGN KEY (source_proposal_id) REFERENCES status_schema_proposals(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical session-scoped status schema registry. Defines keys and value structure only.';


-- ---------------------------------------------------------------------------
-- status_current_values
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS status_current_values (
    id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id      VARCHAR(255)    NOT NULL,
    registry_id          BIGINT UNSIGNED NOT NULL,
    status_key           VARCHAR(255)    NOT NULL,
    owner_scope          VARCHAR(80)     NOT NULL,
    owner_id             VARCHAR(255)    NOT NULL,
    owner_label          VARCHAR(255)    NULL,
    value_kind           VARCHAR(80)     NOT NULL,
    value_json           JSON            NOT NULL,
    evidence_json        JSON            NOT NULL,
    source_turn          INT             NULL,
    write_state          VARCHAR(50)     NOT NULL DEFAULT 'current',
    created_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_status_current_owner (chat_session_id(180), registry_id, owner_scope, owner_id(180)),
    INDEX idx_status_current_session (chat_session_id, write_state, updated_at),
    INDEX idx_status_current_owner (chat_session_id, owner_scope, owner_id(180), status_key(120)),
    INDEX idx_status_current_key (chat_session_id, status_key(120), owner_scope),
    CONSTRAINT fk_status_current_registry FOREIGN KEY (registry_id) REFERENCES status_schema_registry(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Canonical current status values. Requires registry definition and evidence; history/effects are separate lanes.';


-- ---------------------------------------------------------------------------
-- status_change_events
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS status_change_events (
    id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id      VARCHAR(255)    NOT NULL,
    registry_id          BIGINT UNSIGNED NOT NULL,
    status_value_id      BIGINT UNSIGNED NULL,
    status_key           VARCHAR(255)    NOT NULL,
    owner_scope          VARCHAR(80)     NOT NULL,
    owner_id             VARCHAR(255)    NOT NULL,
    event_kind           VARCHAR(80)     NOT NULL,
    previous_value_json  JSON            NULL,
    new_value_json       JSON            NULL,
    evidence_json        JSON            NOT NULL,
    source_turn          INT             NULL,
    story_clock_json     JSON            NULL,
    event_state          VARCHAR(50)     NOT NULL DEFAULT 'recorded',
    created_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_status_event_session (chat_session_id, created_at),
    INDEX idx_status_event_owner (chat_session_id, owner_scope, owner_id(180), status_key(120), created_at),
    INDEX idx_status_event_registry (registry_id, created_at),
    CONSTRAINT fk_status_event_registry FOREIGN KEY (registry_id) REFERENCES status_schema_registry(id) ON DELETE CASCADE,
    CONSTRAINT fk_status_event_current FOREIGN KEY (status_value_id) REFERENCES status_current_values(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Append-only status change event ledger. Does not mutate current values.';


-- ---------------------------------------------------------------------------
-- status_effects
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS status_effects (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chat_session_id       VARCHAR(255)    NOT NULL,
    registry_id           BIGINT UNSIGNED NOT NULL,
    status_key            VARCHAR(255)    NOT NULL,
    owner_scope           VARCHAR(80)     NOT NULL,
    owner_id              VARCHAR(255)    NOT NULL,
    effect_kind           VARCHAR(80)     NOT NULL,
    effect_label          VARCHAR(255)    NULL,
    effect_payload_json   JSON            NULL,
    evidence_json         JSON            NOT NULL,
    source_turn           INT             NULL,
    start_clock_json      JSON            NOT NULL,
    duration_json         JSON            NULL,
    expires_at_clock_json JSON            NULL,
    effect_state          VARCHAR(50)     NOT NULL DEFAULT 'active',
    cleared_evidence_json JSON            NULL,
    cleared_turn          INT             NULL,
    created_at            DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at            DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_status_effect_session (chat_session_id, effect_state, updated_at),
    INDEX idx_status_effect_owner (chat_session_id, owner_scope, owner_id(180), status_key(120), effect_state),
    INDEX idx_status_effect_registry (registry_id, effect_state),
    CONSTRAINT fk_status_effect_registry FOREIGN KEY (registry_id) REFERENCES status_schema_registry(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Status effect lifecycle rows for temporary effects, buffs, debuffs, injuries, and cooldowns.';
