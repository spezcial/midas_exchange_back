CREATE TABLE crypto_deposit_addresses (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    currency_id BIGINT      NOT NULL REFERENCES currencies(id) ON DELETE CASCADE,
    chain       VARCHAR(20) NOT NULL,   -- bitcoin | ethereum | binance | tron
    address     VARCHAR(100) NOT NULL,
    created_at  TIMESTAMP   NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, currency_id)
);

-- Used in the deposit webhook to resolve address → user
CREATE INDEX idx_crypto_deposit_addresses_address ON crypto_deposit_addresses(address);
CREATE INDEX idx_crypto_deposit_addresses_user_id  ON crypto_deposit_addresses(user_id);
