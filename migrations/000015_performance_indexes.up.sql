-- transactions: tx_hash lookup for deposit deduplication (hot path — runs on every webhook)
CREATE INDEX idx_transactions_tx_hash ON transactions(tx_hash) WHERE tx_hash IS NOT NULL AND tx_hash != '';

-- transactions: composite for user history pagination (avoids post-filter sort)
CREATE INDEX idx_transactions_user_id_created ON transactions(user_id, created_at DESC);

-- currency_exchanges: composite for user history pagination (avoids post-filter sort)
CREATE INDEX idx_currency_exchanges_user_id_created ON currency_exchanges(user_id, created_at DESC);

-- otc_messages: partial index for unread-count correlated subquery (runs per row in order lists)
CREATE INDEX idx_otc_messages_unread ON otc_messages(order_id, sender_role) WHERE is_read = false;

-- otc_orders: composite for operator dashboard filtered by status
CREATE INDEX idx_otc_orders_operator_status ON otc_orders(operator_id, status) WHERE operator_id IS NOT NULL;

-- user_sessions: drop the never-used expires_at index
-- (the refresh_token unique index covers the only query: WHERE refresh_token = $1 AND expires_at > NOW())
DROP INDEX idx_user_sessions_expires;
