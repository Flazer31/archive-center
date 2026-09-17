-- Archive Center chat-log exactly-once uniqueness fence.
--
-- The production mariadb-schema compatibility pass preflights conflicting
-- content, normalizes roles, and removes only exact duplicates before running
-- this rerunnable additive index statement.

ALTER TABLE chat_logs
    ADD UNIQUE INDEX IF NOT EXISTS uq_chat_logs_turn_role
        (chat_session_id, turn_index, role);
