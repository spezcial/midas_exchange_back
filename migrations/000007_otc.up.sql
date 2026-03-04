CREATE TABLE IF NOT EXISTS otc_orders (
    id                 BIGSERIAL PRIMARY KEY,
    uid                UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    user_id            BIGINT NOT NULL REFERENCES users(id),
    operator_id        BIGINT REFERENCES users(id),
    from_currency_id   BIGINT NOT NULL REFERENCES currencies(id),
    to_currency_id     BIGINT NOT NULL REFERENCES currencies(id),
    from_amount        DECIMAL(20,8) NOT NULL,
    proposed_rate      DECIMAL(20,8) NOT NULL,
    agreed_rate        DECIMAL(20,8),
    agreed_from_amount DECIMAL(20,8),
    to_amount          DECIMAL(20,8),
    status             VARCHAR(30) NOT NULL DEFAULT 'awaiting_review',
    comment            TEXT,
    cancel_reason      TEXT,
    cancelled_by       VARCHAR(20),
    payment_deadline   TIMESTAMP,
    created_at         TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_otc_orders_uid      ON otc_orders(uid);
CREATE INDEX idx_otc_orders_user     ON otc_orders(user_id);
CREATE INDEX idx_otc_orders_operator ON otc_orders(operator_id);
CREATE INDEX idx_otc_orders_status   ON otc_orders(status);
CREATE INDEX idx_otc_orders_created  ON otc_orders(created_at DESC);

CREATE TABLE IF NOT EXISTS otc_messages (
    id                BIGSERIAL PRIMARY KEY,
    order_id          BIGINT NOT NULL REFERENCES otc_orders(id) ON DELETE CASCADE,
    sender_id         BIGINT NOT NULL REFERENCES users(id),
    sender_role       VARCHAR(20) NOT NULL,
    message_type      VARCHAR(20) NOT NULL DEFAULT 'text',
    content           TEXT,
    offer_rate        DECIMAL(20,8),
    offer_from_amount DECIMAL(20,8),
    offer_to_amount   DECIMAL(20,8),
    offer_status      VARCHAR(20) DEFAULT 'pending',
    created_at        TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_otc_messages_order   ON otc_messages(order_id);
CREATE INDEX idx_otc_messages_created ON otc_messages(created_at);
