-- otc_config: per-pair settings (min amount, payment timeout, active flag)
CREATE TABLE IF NOT EXISTS otc_config (
    id                  BIGSERIAL PRIMARY KEY,
    from_currency_id    BIGINT NOT NULL REFERENCES currencies(id),
    to_currency_id      BIGINT NOT NULL REFERENCES currencies(id),
    min_from_amount     DECIMAL(20,8) NOT NULL DEFAULT 0,
    payment_timeout_min INT NOT NULL DEFAULT 30,
    is_active           BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (from_currency_id, to_currency_id)
);

CREATE INDEX idx_otc_config_pair     ON otc_config(from_currency_id, to_currency_id);
CREATE INDEX idx_otc_config_active   ON otc_config(is_active);

-- otc_messages: add per-reader read tracking
ALTER TABLE otc_messages
    ADD COLUMN is_read   BOOLEAN   NOT NULL DEFAULT false,
    ADD COLUMN read_at   TIMESTAMP;
