CREATE TABLE IF NOT EXISTS content (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    genre VARCHAR(50) NOT NULL,
    popularity_score DOUBLE PRECISION NOT NULL CHECK (popularity_score >= 0),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_genre 
ON content(genre);

CREATE INDEX IF NOT EXISTS idx_content_popularity 
ON content(popularity_score DESC);