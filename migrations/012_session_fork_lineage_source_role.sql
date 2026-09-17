-- Archive Center 4.0: record the exact official RisuAI role at the confirmed
-- fork source message while preserving NULL for manual and legacy lineage.

ALTER TABLE session_fork_lineage
    ADD COLUMN IF NOT EXISTS fork_source_role VARCHAR(16) NULL AFTER fork_source_message_id;
