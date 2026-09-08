-- Critic-derived text is not a bounded identifier. Preserve the complete
-- subtype, relationship key and reveal condition, including existing values.
-- These columns are not indexed; widening them does not change identity keys.
ALTER TABLE precise_memory_units
    MODIFY memory_subtype LONGTEXT NULL,
    MODIFY relationship_key LONGTEXT NULL,
    MODIFY reveal_condition LONGTEXT NULL;
