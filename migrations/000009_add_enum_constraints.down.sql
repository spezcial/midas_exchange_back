ALTER TABLE transactions
    DROP CONSTRAINT IF EXISTS transactions_type_check,
    DROP CONSTRAINT IF EXISTS transactions_status_check;

ALTER TABLE otc_orders
    DROP CONSTRAINT IF EXISTS otc_orders_status_check;

ALTER TABLE otc_messages
    DROP CONSTRAINT IF EXISTS otc_messages_type_check,
    DROP CONSTRAINT IF EXISTS otc_messages_offer_status_check;

ALTER TABLE currency_exchanges
    DROP CONSTRAINT IF EXISTS currency_exchanges_status_check;
