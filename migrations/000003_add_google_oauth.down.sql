-- Remove OAuth state table
DROP TABLE IF EXISTS oauth_states;

-- Remove google_id index and columns from users
DROP INDEX IF EXISTS idx_users_google_id;
ALTER TABLE users DROP COLUMN IF EXISTS google_id;
ALTER TABLE users DROP COLUMN IF EXISTS auth_method;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
