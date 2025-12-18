-- Remove seed data
DELETE FROM exchange_rates;
DELETE FROM currencies WHERE code IN ('BTC', 'ETH', 'USDT', 'BNB', 'SOL', 'XRP', 'ADA', 'DOGE', 'KZT', 'USD', 'EUR');
