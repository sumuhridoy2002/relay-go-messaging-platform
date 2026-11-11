-- Query indexes for conversation lists, message pagination, and user search.
-- Safe to apply to an existing database; no application data is modified.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS conversation_members_user_conversation_idx
    ON conversation_members (user_id, conversation_id);

CREATE INDEX IF NOT EXISTS messages_conversation_cursor_idx
    ON messages (conversation_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS users_username_trgm_idx
    ON users USING GIN (username gin_trgm_ops);

CREATE INDEX IF NOT EXISTS users_display_name_trgm_idx
    ON users USING GIN (display_name gin_trgm_ops);
