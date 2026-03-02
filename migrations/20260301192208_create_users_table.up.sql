CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    age INT NOT NULL CHECK (age > 0),
    country VARCHAR(2) NOT NULL,
    subscription_type VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_country 
ON users(country);

CREATE INDEX IF NOT EXISTS idx_users_subscription 
ON users(subscription_type);