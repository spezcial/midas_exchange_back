-- Store the BackupEligible (BE) flag from the WebAuthn authenticator data.
-- Existing rows default to TRUE — cloud-synced passkeys (iCloud, Google) are backup-eligible,
-- which is what triggered the inconsistency error on login.
ALTER TABLE passkey_credentials
    ADD COLUMN IF NOT EXISTS backup_eligible BOOLEAN NOT NULL DEFAULT TRUE;
