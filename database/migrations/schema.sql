CREATE TABLE IF NOT EXISTS auth
(
    user_id       VARCHAR(255) UNIQUE NOT NULL,
    refresh_token VARCHAR(255)        NOT NULL,
    created_at    TIMESTAMP           NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP
);
