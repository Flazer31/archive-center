-- Nullable provenance preserves the unknown origin of existing snapshots.
-- Observation, occurrence, effectiveness and learning time are separate JSON
-- metadata dimensions; no backfill assigns the snapshot turn to old fields.
ALTER TABLE character_states
    ADD COLUMN IF NOT EXISTS field_provenance_json JSON NULL AFTER speech_style_json;
