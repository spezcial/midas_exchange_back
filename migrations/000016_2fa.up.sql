-- Add phone_verified flag to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_verified BOOLEAN NOT NULL DEFAULT FALSE;

-- Passkey credentials (WebAuthn / FIDO2)
-- A single user can register multiple credentials (e.g. multiple devices).
CREATE TABLE IF NOT EXISTS passkey_credentials (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id TEXT NOT NULL UNIQUE,   -- base64url-encoded WebAuthn credential ID
    public_key    BYTEA NOT NULL,          -- COSE-encoded public key
    aaguid        BYTEA NOT NULL DEFAULT '',
    sign_count    BIGINT NOT NULL DEFAULT 0,
    name          TEXT NOT NULL DEFAULT '', -- user-visible label, e.g. "iPhone 15"
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_passkey_credentials_user_id ON passkey_credentials(user_id);

-- Login fingerprints: SHA-256(IP + User-Agent) per user.
-- A new fingerprint triggers a security notification via Telegram.
CREATE TABLE IF NOT EXISTS user_fingerprints (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fingerprint  TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_user_fingerprints_user_id ON user_fingerprints(user_id);
