CREATE TABLE platform_fees (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    operation    VARCHAR(50) NOT NULL CHECK (operation IN ('exchange', 'withdrawal')),
    currency_id  INT NOT NULL REFERENCES currencies(id),
    gross_amount NUMERIC(24, 8) NOT NULL,
    fee          NUMERIC(24, 8) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX platform_fees_user_id_idx ON platform_fees (user_id);
CREATE INDEX platform_fees_created_at_idx ON platform_fees (created_at DESC);
