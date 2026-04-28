DROP TABLE IF EXISTS user_fingerprints;
DROP TABLE IF EXISTS passkey_credentials;
ALTER TABLE users DROP COLUMN IF EXISTS phone_verified;
