-- Insert initial currencies
INSERT INTO currencies (code, name, symbol, is_active, is_crypto) VALUES
    ('BTC', 'Bitcoin', '₿', true, true),
    ('ETH', 'Ethereum', 'Ξ', true, true),
    ('USDT', 'Tether', '₮', true, true),
    ('BNB', 'Binance Coin', 'BNB', true, true),
    ('SOL', 'Solana', 'SOL', true, true),
    ('XRP', 'Ripple', 'XRP', true, true),
    ('ADA', 'Cardano', '₳', true, true),
    ('DOGE', 'Dogecoin', 'Ð', true, true),
    ('KZT', 'Tenge', '₸', true, false),
    ('USD', 'US Dollar', '$', true, false),
    ('EUR', 'Euro', '€', true, false);

-- Insert sample exchange rates (these should be updated regularly in production)
-- Example rates: USDT to KZT
INSERT INTO exchange_rates (from_currency_id, to_currency_id, rate, is_active, fee) VALUES
    ((SELECT id FROM currencies WHERE code = 'USDT'), (SELECT id FROM currencies WHERE code = 'KZT'), 450.00, true, 2),
    ((SELECT id FROM currencies WHERE code = 'KZT'), (SELECT id FROM currencies WHERE code = 'USDT'), 0.0022, true, 2),
    ((SELECT id FROM currencies WHERE code = 'BTC'), (SELECT id FROM currencies WHERE code = 'USDT'), 45000.00, true, 2),
    ((SELECT id FROM currencies WHERE code = 'USDT'), (SELECT id FROM currencies WHERE code = 'BTC'), 0.000022, true, 2),
    ((SELECT id FROM currencies WHERE code = 'ETH'), (SELECT id FROM currencies WHERE code = 'USDT'), 2500.00, true, 2),
    ((SELECT id FROM currencies WHERE code = 'USDT'), (SELECT id FROM currencies WHERE code = 'ETH'), 0.0004, true, 2);
