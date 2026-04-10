package domain

import "time"

// DepositAddress stores the blockchain deposit address generated for a user.
type DepositAddress struct {
	ID         int64     `db:"id"          json:"id"`
	UserID     int64     `db:"user_id"     json:"user_id"`
	CurrencyID int64     `db:"currency_id" json:"currency_id"`
	Chain      string    `db:"chain"       json:"chain"`
	Address    string    `db:"address"     json:"address"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}

// CurrencyChain maps a currency code to the crypto-gate chain name and asset identifier.
// Chain is used for deposit address creation; Asset is used for withdrawals.
type CurrencyChain struct {
	Chain string // bitcoin | ethereum | binance | tron
	Asset string // btc | eth | bnb | trx | ethereum_<contract> | tron_<contract>
}

// Known currency → chain/asset mappings supported by the crypto-gate.
// USDT defaults to TRC-20 (cheapest fees, most common for KZT exchanges).
var CryptoGateCurrencies = map[string]CurrencyChain{
	"BTC":  {Chain: "bitcoin", Asset: "btc"},
	"ETH":  {Chain: "ethereum", Asset: "eth"},
	"BNB":  {Chain: "binance", Asset: "bnb"},
	"TRX":  {Chain: "tron", Asset: "trx"},
	"USDT": {Chain: "tron", Asset: "tron_tr7nhqjekqxgtci8q8zy4pl8otszgjlj6t"},
}

// CurrencyChainFor returns the chain config for a currency code, or false if unsupported.
func CurrencyChainFor(code string) (CurrencyChain, bool) {
	c, ok := CryptoGateCurrencies[code]
	return c, ok
}

// Webhook payloads sent by crypto-gate to our exchange.

// DepositWebhook is the payload POSTed to /cg/deposit when a deposit is confirmed.
// Field names match the gateway JSON exactly (lowercase snake_case).
// Amount is a decimal string (e.g. "100.000000000") — parse with strconv.ParseFloat.
type DepositWebhook struct {
	Address string `json:"address"` // user's deposit address
	From    string `json:"from"`    // sender address
	Amount  string `json:"amount"`  // decimal string, e.g. "100.000000000"
	Asset   string `json:"asset"`   // e.g. "ETH", "tron_tr7nhq..."
	Network string `json:"network"` // "bitcoin" | "ethereum" | "binance" | "tron"
	Hash    string `json:"hash"`    // blockchain tx hash
}

// WithdrawWebhook is the payload POSTed to /cg/withdraw when a withdrawal status changes.
// Field names match the gateway JSON exactly.
type WithdrawWebhook struct {
	UUID   string `json:"UUID"`   // exchange transaction ID (string) — gateway echoes it back uppercase
	Hash   string `json:"hash"`   // blockchain tx hash; empty string when status is "failed"
	Status string `json:"status"` // "success" | "failed"
}
