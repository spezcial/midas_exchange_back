-- Store the BackupEligible (BE) flag from the WebAuthn authenticator data.
-- DEFAULT TRUE: all credentials registered before this migration are assumed cloud-synced
-- (iCloud Keychain / Google Password Manager). Hardware security keys (YubiKey etc.)
-- always report backup_eligible=false and would need to be re-registered after this migration.
ALTER TABLE passkey_credentials
    ADD COLUMN IF NOT EXISTS backup_eligible BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS backup_state    BOOLEAN NOT NULL DEFAULT FALSE;
