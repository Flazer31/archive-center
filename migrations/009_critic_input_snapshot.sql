-- Preserve the exact bounded dynamic Critic input used by an accepted source
-- revision so background reprocessing does not rebuild it from mutable state.

ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS critic_input_snapshot_json JSON NULL;

ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS critic_input_snapshot_hash CHAR(64) NULL;
