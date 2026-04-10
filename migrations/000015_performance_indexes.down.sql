DROP INDEX IF EXISTS idx_transactions_tx_hash;
DROP INDEX IF EXISTS idx_transactions_user_id_created;
DROP INDEX IF EXISTS idx_currency_exchanges_user_id_created;
DROP INDEX IF EXISTS idx_otc_messages_unread;
DROP INDEX IF EXISTS idx_otc_orders_operator_status;
CREATE INDEX idx_user_sessions_expires ON user_sessions(expires_at);
