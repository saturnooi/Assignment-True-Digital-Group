CREATE TABLE IF NOT EXISTS user_watch_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    content_id BIGINT NOT NULL,
    watched_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_watch_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_watch_content
        FOREIGN KEY (content_id)
        REFERENCES content(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_watch_history_user
ON user_watch_history(user_id);

CREATE INDEX IF NOT EXISTS idx_watch_history_content
ON user_watch_history(content_id);

CREATE INDEX IF NOT EXISTS idx_watch_history_composite
ON user_watch_history(user_id, watched_at DESC);