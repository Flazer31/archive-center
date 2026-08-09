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

CREATE TABLE IF NOT EXISTS session_migration_reference_binding_map (
    migration_id       BIGINT UNSIGNED NOT NULL,
    source_binding_id  CHAR(36)        NOT NULL,
    target_binding_id  CHAR(36)        NOT NULL,
    row_status         VARCHAR(50)     NOT NULL DEFAULT 'copied',
    created_at         DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    PRIMARY KEY (migration_id, source_binding_id),
    UNIQUE KEY uq_session_migration_reference_target (migration_id, target_binding_id),
    CONSTRAINT fk_session_migration_reference_binding_map
        FOREIGN KEY (migration_id) REFERENCES session_migrations(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Maps reusable reference bindings copied by a session migration so rollback removes only migration-owned links.';

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
  COMMENT='Canonical current status values. Requires registry definition and evidence. History/effects are separate lanes.';


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


-- ---------------------------------------------------------------------------
-- Archive Center 3.0 reference work library
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS reference_works (
    work_id           CHAR(36)        PRIMARY KEY,
    title             VARCHAR(500)    NOT NULL,
    work_type         VARCHAR(50)     NOT NULL DEFAULT 'custom',
    default_language  VARCHAR(32)     NOT NULL DEFAULT '',
    status            VARCHAR(50)     NOT NULL DEFAULT 'draft',
    metadata_json     JSON            NULL,
    revision          BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at        DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at        DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_reference_work_status (status, updated_at),
    INDEX idx_reference_work_title (title(180))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Reusable original-work/reference library root, independent from chat sessions.';

CREATE TABLE IF NOT EXISTS reference_continuities (
    continuity_id         CHAR(36)        PRIMARY KEY,
    work_id               CHAR(36)        NOT NULL,
    continuity_key        VARCHAR(255)    NOT NULL,
    label                 VARCHAR(500)    NOT NULL,
    parent_continuity_id  CHAR(36)        NULL,
    status                VARCHAR(50)     NOT NULL DEFAULT 'active',
    metadata_json         JSON            NULL,
    revision              BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at            DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at            DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_reference_continuity_key (work_id, continuity_key),
    INDEX idx_reference_continuity_work (work_id, status, updated_at),
    CONSTRAINT fk_reference_continuity_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_continuity_parent FOREIGN KEY (parent_continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Separates novel, animation, remake, branch, and user-defined continuities.';

CREATE TABLE IF NOT EXISTS reference_documents (
    document_id      CHAR(36)        PRIMARY KEY,
    work_id          CHAR(36)        NOT NULL,
    continuity_id    CHAR(36)        NOT NULL,
    source_type      VARCHAR(50)     NOT NULL DEFAULT 'manual_text',
    source_uri       TEXT            NULL,
    content_hash     CHAR(64)        NOT NULL,
    raw_retention    VARCHAR(50)     NOT NULL DEFAULT 'full',
    raw_text         LONGTEXT        NULL,
    import_status    VARCHAR(50)     NOT NULL DEFAULT 'pending',
    provenance_json  JSON            NULL,
    created_at       DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at       DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_reference_document_hash (work_id, continuity_id, content_hash),
    INDEX idx_reference_document_status (work_id, continuity_id, import_status, updated_at),
    CONSTRAINT fk_reference_document_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_document_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Manual, file, or web provenance records for reference material.';

CREATE TABLE IF NOT EXISTS reference_timeline_nodes (
    node_id          CHAR(36)        PRIMARY KEY,
    work_id          CHAR(36)        NOT NULL,
    continuity_id    CHAR(36)        NOT NULL,
    node_key         VARCHAR(255)    NOT NULL,
    label            VARCHAR(500)    NOT NULL,
    ordinal_value    BIGINT          NOT NULL DEFAULT 0,
    parent_node_id   CHAR(36)        NULL,
    branch_key       VARCHAR(255)    NOT NULL DEFAULT 'main',
    node_kind        VARCHAR(50)     NOT NULL DEFAULT 'event',
    metadata_json    JSON            NULL,
    review_status    VARCHAR(50)     NOT NULL DEFAULT 'pending',
    review_source    VARCHAR(50)     NOT NULL DEFAULT '',
    review_reason    TEXT            NULL,
    reviewed_at      DATETIME(3)     NULL,
    created_at       DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at       DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_reference_timeline_key (continuity_id, branch_key, node_key),
    INDEX idx_reference_timeline_order (continuity_id, branch_key, ordinal_value),
    INDEX idx_reference_timeline_review (work_id, continuity_id, review_status, ordinal_value),
    CONSTRAINT fk_reference_timeline_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_timeline_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_timeline_parent FOREIGN KEY (parent_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Event-oriented chronology and branch anchors; ordinal is not story-clock time.';

CREATE TABLE IF NOT EXISTS reference_entities (
    entity_id         CHAR(36)        PRIMARY KEY,
    work_id           CHAR(36)        NOT NULL,
    continuity_id     CHAR(36)        NOT NULL,
    entity_type       VARCHAR(50)     NOT NULL,
    canonical_name    VARCHAR(500)    NOT NULL,
    description_text  LONGTEXT        NULL,
    metadata_json     JSON            NULL,
    review_status     VARCHAR(50)     NOT NULL DEFAULT 'pending',
    review_source     VARCHAR(50)     NOT NULL DEFAULT '',
    review_reason     TEXT            NULL,
    reviewed_at       DATETIME(3)     NULL,
    created_at        DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at        DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_reference_entity_name (work_id, continuity_id, canonical_name(180)),
    INDEX idx_reference_entity_type (work_id, continuity_id, entity_type, review_status),
    CONSTRAINT fk_reference_entity_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_entity_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Reference-only people, places, items, and factions; never auto-merged into session entities.';

CREATE TABLE IF NOT EXISTS reference_entity_aliases (
    alias_id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    work_id           CHAR(36)        NOT NULL,
    continuity_id     CHAR(36)        NOT NULL,
    entity_id         CHAR(36)        NOT NULL,
    alias_text        VARCHAR(500)    NOT NULL,
    normalized_alias  VARCHAR(255)    NOT NULL,
    language_code     VARCHAR(32)     NOT NULL DEFAULT '',
    created_at        DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_reference_entity_alias (work_id, continuity_id, entity_id, normalized_alias),
    INDEX idx_reference_alias_lookup (work_id, continuity_id, normalized_alias),
    CONSTRAINT fk_reference_alias_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_alias_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_alias_entity FOREIGN KEY (entity_id) REFERENCES reference_entities(entity_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Surface aliases scoped to one work and continuity.';

CREATE TABLE IF NOT EXISTS reference_claims (
    claim_id             CHAR(36)        PRIMARY KEY,
    work_id              CHAR(36)        NOT NULL,
    continuity_id        CHAR(36)        NOT NULL,
    document_id          CHAR(36)        NOT NULL,
    claim_type           VARCHAR(50)     NOT NULL,
    subject_entity_id    CHAR(36)        NULL,
    claim_text           LONGTEXT        NOT NULL,
    evidence_excerpt     TEXT            NULL,
    temporal_scope       VARCHAR(50)     NOT NULL DEFAULT 'bounded',
    valid_from_node_id   CHAR(36)        NULL,
    valid_to_node_id     CHAR(36)        NULL,
    reveal_from_node_id  CHAR(36)        NULL,
    branch_key           VARCHAR(255)    NOT NULL DEFAULT 'main',
    knowledge_scope      VARCHAR(50)     NOT NULL DEFAULT 'public_world',
    confidence           DOUBLE          NOT NULL DEFAULT 0,
    review_status        VARCHAR(50)     NOT NULL DEFAULT 'pending',
    review_source        VARCHAR(50)     NOT NULL DEFAULT '',
    review_reason        TEXT            NULL,
    reviewed_at          DATETIME(3)     NULL,
    metadata_json        JSON            NULL,
    created_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at           DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_reference_claim_recall (work_id, continuity_id, review_status, branch_key, reveal_from_node_id),
    INDEX idx_reference_claim_subject (subject_entity_id, claim_type),
    INDEX idx_reference_claim_document (document_id, review_status),
    CONSTRAINT fk_reference_claim_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_claim_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_claim_document FOREIGN KEY (document_id) REFERENCES reference_documents(document_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_claim_subject FOREIGN KEY (subject_entity_id) REFERENCES reference_entities(entity_id) ON DELETE SET NULL,
    CONSTRAINT fk_reference_claim_valid_from FOREIGN KEY (valid_from_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL,
    CONSTRAINT fk_reference_claim_valid_to FOREIGN KEY (valid_to_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL,
    CONSTRAINT fk_reference_claim_reveal_from FOREIGN KEY (reveal_from_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Approved factual units with chronology, spoiler, branch, and knowledge scope.';

CREATE TABLE IF NOT EXISTS reference_claim_knowers (
    claim_id    CHAR(36)    NOT NULL,
    entity_id   CHAR(36)    NOT NULL,
    created_at  DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    PRIMARY KEY (claim_id, entity_id),
    INDEX idx_reference_knower_entity (entity_id, claim_id),
    CONSTRAINT fk_reference_knower_claim FOREIGN KEY (claim_id) REFERENCES reference_claims(claim_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_knower_entity FOREIGN KEY (entity_id) REFERENCES reference_entities(entity_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Character-scoped knowledge for private or limited reference claims.';

CREATE TABLE IF NOT EXISTS session_reference_bindings (
    binding_id              CHAR(36)        PRIMARY KEY,
    chat_session_id         VARCHAR(255)    NOT NULL,
    work_id                 CHAR(36)        NOT NULL,
    continuity_id           CHAR(36)        NOT NULL,
    binding_role            VARCHAR(50)     NOT NULL DEFAULT 'primary',
    reference_mode          VARCHAR(50)     NOT NULL DEFAULT 'supplement',
    enabled                 BOOLEAN         NOT NULL DEFAULT TRUE,
    injection_enabled       BOOLEAN         NOT NULL DEFAULT FALSE,
    anchor_mode             VARCHAR(50)     NOT NULL DEFAULT 'manual',
    current_node_id         CHAR(36)        NULL,
    reveal_ceiling_node_id  CHAR(36)        NULL,
    divergence_node_id      CHAR(36)        NULL,
    future_policy           VARCHAR(50)     NOT NULL DEFAULT 'block',
    priority                INT             NOT NULL DEFAULT 0,
    revision                BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_session_reference_binding (chat_session_id(180), work_id, continuity_id),
    INDEX idx_session_reference_enabled (chat_session_id, enabled, priority),
    INDEX idx_reference_binding_work (work_id, continuity_id, enabled),
    CONSTRAINT fk_session_reference_work FOREIGN KEY (work_id) REFERENCES reference_works(work_id) ON DELETE RESTRICT,
    CONSTRAINT fk_session_reference_continuity FOREIGN KEY (continuity_id) REFERENCES reference_continuities(continuity_id) ON DELETE RESTRICT,
    CONSTRAINT fk_session_reference_current_node FOREIGN KEY (current_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL,
    CONSTRAINT fk_session_reference_reveal_node FOREIGN KEY (reveal_ceiling_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL,
    CONSTRAINT fk_session_reference_divergence_node FOREIGN KEY (divergence_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Explicit reusable-work links for one RisuAI chat session.';

CREATE TABLE IF NOT EXISTS session_reference_runtime (
    binding_id                CHAR(36)        PRIMARY KEY,
    candidate_node_id         CHAR(36)        NULL,
    candidate_source_turn     INT             NULL,
    candidate_evidence_json   JSON            NULL,
    candidate_confirmed       BOOLEAN         NOT NULL DEFAULT FALSE,
    last_claim_ids_json       JSON            NULL,
    diagnostics_json          JSON            NULL,
    revision                  BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at                DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at                DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    CONSTRAINT fk_session_reference_runtime_binding FOREIGN KEY (binding_id) REFERENCES session_reference_bindings(binding_id) ON DELETE CASCADE,
    CONSTRAINT fk_session_reference_runtime_candidate FOREIGN KEY (candidate_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='One mutable assisted-anchor candidate per binding; not an accumulating memory lane.';

CREATE TABLE IF NOT EXISTS session_reference_coverage_snapshots (
    binding_id             CHAR(36)        PRIMARY KEY,
    contract_version       VARCHAR(50)     NOT NULL,
    context_hash           CHAR(64)        NOT NULL,
    inventory_hash         CHAR(64)        NOT NULL,
    snapshot_hash          CHAR(64)        NOT NULL,
    source_message_count   INT             NOT NULL DEFAULT 0,
    field_count            INT             NOT NULL DEFAULT 0,
    covered_field_count    INT             NOT NULL DEFAULT 0,
    stats_json             JSON            NULL,
    revision               BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at             DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at             DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    INDEX idx_reference_coverage_snapshot_hash (snapshot_hash),
    CONSTRAINT fk_reference_coverage_snapshot_binding FOREIGN KEY (binding_id)
        REFERENCES session_reference_bindings(binding_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='One replaceable active-request coverage snapshot per reference binding; no turn history.';

CREATE TABLE IF NOT EXISTS session_reference_coverage_fields (
    binding_id              CHAR(36)        NOT NULL,
    field_key               CHAR(64)        NOT NULL,
    work_id                 CHAR(36)        NOT NULL,
    continuity_id           CHAR(36)        NOT NULL,
    reference_kind          VARCHAR(50)     NOT NULL,
    source_id               CHAR(36)        NOT NULL,
    field_name              VARCHAR(255)    NOT NULL,
    field_value             LONGTEXT        NOT NULL,
    normalized_value        LONGTEXT        NOT NULL,
    match_values_json       JSON            NULL,
    present_in_context      BOOLEAN         NOT NULL DEFAULT FALSE,
    matched_locations_json  JSON            NULL,
    eligible                BOOLEAN         NOT NULL DEFAULT TRUE,
    eligibility_reason      VARCHAR(100)    NOT NULL DEFAULT 'eligible',
    created_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at              DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    PRIMARY KEY (binding_id, field_key),
    INDEX idx_reference_coverage_source (binding_id, reference_kind, source_id),
    INDEX idx_reference_coverage_presence (binding_id, present_in_context, eligible),
    CONSTRAINT fk_reference_coverage_field_snapshot FOREIGN KEY (binding_id)
        REFERENCES session_reference_coverage_snapshots(binding_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_coverage_field_work FOREIGN KEY (work_id)
        REFERENCES reference_works(work_id) ON DELETE CASCADE,
    CONSTRAINT fk_reference_coverage_field_continuity FOREIGN KEY (continuity_id)
        REFERENCES reference_continuities(continuity_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Current field-level comparison between active request context and approved reference inventory.';

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

-- ---------------------------------------------------------------------------
-- 3.6-B. entity_identity.v1 and speaker attribution
-- ---------------------------------------------------------------------------
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

-- ---------------------------------------------------------------------------
-- Exact-source atomic memory projection
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS precise_memory_units (
    id                         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    unit_id                    CHAR(36) NOT NULL,
    contract_version           VARCHAR(80) NOT NULL DEFAULT 'precise_memory_unit.v1',
    chat_session_id            VARCHAR(255) NOT NULL,
    source_turn_start          INT NOT NULL,
    source_turn_end            INT NOT NULL,
    source_contract            VARCHAR(80) NOT NULL,
    source_revision            VARCHAR(160) NOT NULL,
    source_logical_turn_id     VARCHAR(160) NULL,
    source_message_id          VARCHAR(255) NULL,
    source_generation_id       VARCHAR(255) NULL,
    source_content_hash        CHAR(64) NOT NULL,
    source_role                VARCHAR(80) NOT NULL,
    source_span_start          INT NOT NULL,
    source_span_end            INT NOT NULL,
    evidence_excerpt           TEXT NOT NULL,
    evidence_hash              CHAR(64) NOT NULL,
    root_evidence_id           BIGINT UNSIGNED NULL,
    direct_evidence_ids_json   JSON NOT NULL,
    memory_kind                VARCHAR(80) NOT NULL,
    memory_subtype             VARCHAR(120) NULL,
    payload_json               JSON NOT NULL,
    actor_entity_id            CHAR(36) NULL,
    subject_entity_id          CHAR(36) NULL,
    affected_entity_id         CHAR(36) NULL,
    location_entity_id         CHAR(36) NULL,
    object_entity_id           CHAR(36) NULL,
    relationship_key           VARCHAR(255) NULL,
    truth_scope                VARCHAR(80) NOT NULL,
    epistemic_mode             VARCHAR(80) NOT NULL,
    authority_class            VARCHAR(80) NOT NULL,
    admission_state            VARCHAR(50) NOT NULL,
    review_state               VARCHAR(50) NOT NULL,
    visibility                 VARCHAR(80) NOT NULL,
    knowledge_holder_entity_id CHAR(36) NULL,
    reveal_condition           VARCHAR(255) NULL,
    confidence                 DOUBLE NOT NULL DEFAULT 0,
    idempotency_key            VARCHAR(255) NOT NULL,
    lifecycle_state            VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at                 DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at                 DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    UNIQUE KEY uq_precise_memory_unit_id (unit_id),
    UNIQUE KEY uq_precise_memory_source_payload (chat_session_id(120), idempotency_key(160)),
    INDEX idx_precise_memory_source (chat_session_id(120), source_revision(120), source_turn_start),
    INDEX idx_precise_memory_kind (chat_session_id(120), memory_kind, lifecycle_state, source_turn_start),
    INDEX idx_precise_memory_review (chat_session_id(120), admission_state, review_state, updated_at),
    INDEX idx_precise_memory_root_evidence (root_evidence_id),
    CONSTRAINT fk_precise_memory_root_evidence FOREIGN KEY (root_evidence_id) REFERENCES direct_evidence_records(id) ON DELETE SET NULL,
    CONSTRAINT fk_precise_memory_actor FOREIGN KEY (actor_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL,
    CONSTRAINT fk_precise_memory_subject FOREIGN KEY (subject_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL,
    CONSTRAINT fk_precise_memory_affected FOREIGN KEY (affected_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL,
    CONSTRAINT fk_precise_memory_location FOREIGN KEY (location_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL,
    CONSTRAINT fk_precise_memory_object FOREIGN KEY (object_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL,
    CONSTRAINT fk_precise_memory_knower FOREIGN KEY (knowledge_holder_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE SET NULL,
    CONSTRAINT chk_precise_memory_kind CHECK (memory_kind IN ('event', 'state', 'utterance', 'observation', 'boundary', 'profile')),
    CONSTRAINT chk_precise_memory_span CHECK (source_span_start >= 0 AND source_span_end > source_span_start),
    CONSTRAINT chk_precise_memory_turn_range CHECK (source_turn_start > 0 AND source_turn_end >= source_turn_start)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='precise_memory_unit.v1 exact-source atomic projection; legacy aggregate memories remain unchanged.';

-- ---------------------------------------------------------------------------
-- Accepted-source derivation lifecycle and durable vector work
-- ---------------------------------------------------------------------------
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
    derived_admission_state VARCHAR(30) NOT NULL DEFAULT 'pending',
    derived_admission_version VARCHAR(120) NOT NULL DEFAULT '',
    derived_extractor_version VARCHAR(120) NOT NULL DEFAULT '',
    derived_index_version VARCHAR(120) NOT NULL DEFAULT '',
    derived_result_hash CHAR(64) NULL,
    derived_result_json JSON NULL,
    derived_admitted_at DATETIME(3) NULL,
    critic_input_snapshot_json JSON NULL,
    critic_input_snapshot_hash CHAR(64) NULL,
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
    CONSTRAINT chk_memory_source_admission CHECK (derived_admission_state IN ('pending', 'committed')),
    CONSTRAINT chk_memory_source_turn CHECK (turn_index > 0),
    CONSTRAINT chk_memory_source_branch_state CHECK (branch_state IN ('observed', 'not_exposed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Host-observed accepted raw user/assistant source revisions; no inferred branch identity.';

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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='memory_derivation_dependency.v1 normalized parent-to-child lineage.';

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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='memory_reprocessing_job.v1 durable revision-aware worker queue.';

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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Idempotent revision-fenced vector delete/upsert outbox; provider secrets are never stored.';

-- ---------------------------------------------------------------------------
-- 3.7-D session route binding and exhaustive migration parity ledgers
-- The canonical schema command reruns these IF NOT EXISTS definitions as its
-- existing-install compatibility pass; previously shipped ledgers also remain
-- recorded in immutable migration 007.
-- ---------------------------------------------------------------------------

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

CREATE TABLE IF NOT EXISTS session_migration_artifact_row_map (
    migration_id       BIGINT UNSIGNED NOT NULL,
    table_name         VARCHAR(100)    NOT NULL,
    key_column_name    VARCHAR(100)    NOT NULL,
    source_key         VARCHAR(255)    NOT NULL,
    target_key         VARCHAR(255)    NOT NULL,
    row_status         VARCHAR(50)     NOT NULL DEFAULT 'copied',
    created_at         DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,
    updated_at         DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL,
    PRIMARY KEY (migration_id, table_name, key_column_name, source_key),
    UNIQUE KEY uq_session_migration_artifact_target
        (migration_id, table_name, key_column_name, target_key),
    INDEX idx_session_migration_artifact_row_status (migration_id, row_status),
    CONSTRAINT fk_session_migration_artifact_row_map
        FOREIGN KEY (migration_id) REFERENCES session_migrations(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Typed-as-text allowlisted row and alternate-key remap ledger for numeric, UUID, and revision-key artifacts.';

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
