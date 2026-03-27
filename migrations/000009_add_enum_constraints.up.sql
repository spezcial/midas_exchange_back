-- Enforce valid enum values at DB level.
-- transactions.type must include both existing and the new OTC types.
ALTER TABLE transactions
    ADD CONSTRAINT transactions_type_check
        CHECK (type IN ('deposit', 'withdrawal', 'otc_debit', 'otc_credit')),
    ADD CONSTRAINT transactions_status_check
        CHECK (status IN ('pending', 'completed', 'failed', 'canceled'));

ALTER TABLE otc_orders
    ADD CONSTRAINT otc_orders_status_check
        CHECK (status IN ('awaiting_review','negotiating','awaiting_payment',
                          'payment_received','completed','cancelled','expired'));

ALTER TABLE otc_messages
    ADD CONSTRAINT otc_messages_type_check
        CHECK (message_type IN ('text', 'offer')),
    ADD CONSTRAINT otc_messages_offer_status_check
        CHECK (offer_status IS NULL OR offer_status IN ('pending','accepted','rejected'));

ALTER TABLE currency_exchanges
    ADD CONSTRAINT currency_exchanges_status_check
        CHECK (status IN ('pending','completed','canceled'));
