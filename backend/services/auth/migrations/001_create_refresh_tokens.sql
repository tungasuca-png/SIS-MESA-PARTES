CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_refresh_tokens_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT chk_refresh_tokens_expiry CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_usuario_id
    ON refresh_tokens (usuario_id);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_active_lookup
    ON refresh_tokens (token_hash, expires_at)
    WHERE revoked_at IS NULL;