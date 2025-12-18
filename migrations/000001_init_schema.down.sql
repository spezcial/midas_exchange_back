-- Drop tables in reverse order due to foreign key constraints
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS currency_exchanges;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS wallets;
DROP TABLE IF EXISTS currencies;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS users;
