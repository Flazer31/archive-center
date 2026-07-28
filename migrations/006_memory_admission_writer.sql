-- Common source-fenced writer admission marker for existing installations.
-- The production mariadb-schema compatibility pass contains the same additive
-- statements, so fresh installs and upgrades converge on one schema.

ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS derived_admission_state VARCHAR(30) NOT NULL DEFAULT 'pending';
ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS derived_admission_version VARCHAR(120) NOT NULL DEFAULT '';
ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS derived_extractor_version VARCHAR(120) NOT NULL DEFAULT '';
ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS derived_index_version VARCHAR(120) NOT NULL DEFAULT '';
ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS derived_result_hash CHAR(64) NULL;
ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS derived_result_json JSON NULL;
ALTER TABLE memory_source_revisions
    ADD COLUMN IF NOT EXISTS derived_admitted_at DATETIME(3) NULL;
