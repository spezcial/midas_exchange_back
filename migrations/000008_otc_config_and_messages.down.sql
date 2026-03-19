ALTER TABLE otc_messages
    DROP COLUMN IF EXISTS is_read,
    DROP COLUMN IF EXISTS read_at;

DROP TABLE IF EXISTS otc_config;
