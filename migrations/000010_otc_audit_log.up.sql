CREATE TABLE otc_audit_log (
    id         BIGSERIAL PRIMARY KEY,
    order_id   BIGINT      NOT NULL REFERENCES otc_orders(id) ON DELETE CASCADE,
    actor_id   BIGINT      NOT NULL REFERENCES users(id),
    actor_role VARCHAR(30) NOT NULL,
    action     VARCHAR(50) NOT NULL,
    details    TEXT,
    created_at TIMESTAMP   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_otc_audit_log_order_id   ON otc_audit_log(order_id);
CREATE INDEX idx_otc_audit_log_actor_id   ON otc_audit_log(actor_id);
CREATE INDEX idx_otc_audit_log_created_at ON otc_audit_log(created_at DESC);
