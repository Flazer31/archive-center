-- Archive Center 4.0: automatic RisuAI worldline lineage on the existing
-- session_fork_lineage owner. Nullable idempotency keys preserve manual rows.

ALTER TABLE session_fork_lineage
    ADD COLUMN IF NOT EXISTS contract_version VARCHAR(80) NOT NULL DEFAULT 'session_fork_lineage.v1' AFTER id,
    ADD COLUMN IF NOT EXISTS lineage_state VARCHAR(32) NOT NULL DEFAULT 'manual' AFTER contract_version,
    ADD COLUMN IF NOT EXISTS fork_turn INT NULL AFTER copied_from_session_id,
    ADD COLUMN IF NOT EXISTS fork_source_message_id VARCHAR(255) NULL AFTER fork_turn,
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(160) NULL AFTER fork_source_message_id;

DROP INDEX IF EXISTS uq_session_fork_lineage_idempotency
    ON session_fork_lineage;

CREATE UNIQUE INDEX IF NOT EXISTS uq_session_fork_lineage_idempotency
    ON session_fork_lineage (chat_session_id, idempotency_key);
