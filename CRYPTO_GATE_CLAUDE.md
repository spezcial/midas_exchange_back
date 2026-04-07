# crypto-gate — Technical Reference

## What This Is

A **multi-chain cryptocurrency payment gateway and hot wallet manager** for CaspianEx exchange. It handles deposit monitoring, withdrawal processing, fund collection (hot→cold), and transaction confirmation tracking across four blockchains.

**Not a trading engine, not an OTC desk.** It is purely the blockchain I/O layer for a crypto exchange.

---

## Supported Chains

| Chain    | Adapter | Native | Token         | Contract                                       |
|----------|---------|--------|---------------|------------------------------------------------|
| Bitcoin  | UTXO    | BTC    | —             | —                                              |
| Ethereum | EVM     | ETH    | USDT (ERC-20) | `0xdac17f958d2ee523a2206206994597c13d831ec7`   |
| BSC      | EVM     | BNB    | USDT (BEP-20) | `0x55d398326f99059ff775485246999027b3197955`   |
| TRON     | TVM     | TRX    | USDT (TRC-20) | `TR7NHQjEKQxgtci8q8zy4pl8otszgjlj6t`          |

---

## Tech Stack

- **Language:** Go 1.23.5
- **Database:** PostgreSQL (pgx/lib/pq)
- **Cache:** Redis (go-redis/v8)
- **Config:** Viper + `.env` file
- **Crypto:** AES-256-CBC + scrypt (for private key encryption at rest)
- **Logging:** logrus
- **Blockchain libs:** go-ethereum (EVM+TRON RPC), btcsuite/btcd (Bitcoin)
- **Infra:** Docker Compose (postgres:latest + redis:latest)

---

## Architecture

```
HTTP API  ─────────────────────────────────────┐
  GET /wallet?chain=                            │
  GET /wallet/balances?address=                 │
  POST /withdraw                                │
                                                ▼
                              ┌─────────────────────────────┐
                              │        Services Layer        │
                              │  wallets / withdrawals /     │
                              │  deposits / collector /      │
                              │  confirmator                 │
                              └────────────┬────────────────┘
                                           │
              ┌────────────────────────────┼────────────────────────┐
              ▼                            ▼                         ▼
        adapters/utxo              adapters/evm               adapters/tvm
          (Bitcoin)            (Ethereum, BSC)                  (TRON)
              │                            │                         │
              └────────────────────────────┴────────────────────────┘
                                           │
                          ┌────────────────┴────────────────┐
                          │          Shared Layer            │
                          │  PostgreSQL · Redis · RPC client │
                          │  AES crypto · Notifications      │
                          └─────────────────────────────────┘
```

### Four Independent Goroutine Workers

| Worker       | Binary                | What it does                                         |
|--------------|-----------------------|------------------------------------------------------|
| API          | `cmd/main.go`         | HTTP server for wallet/withdraw requests             |
| Deposits     | `cmd/deposits.go`     | Polls new blocks per chain, detects incoming txs     |
| Collector    | `cmd/collector.go`    | Sweeps hot wallet → cold wallet periodically         |
| Confirmator  | `cmd/confirmator.go`  | Tracks confirmation count, fires exchange webhook    |

---

## Key Data Flows

### Deposit
```
New block detected
  → parse transactions
  → match to monitored address (Redis list: monitoredAddresses_{chain})
  → write to DB (transactions table, status=pending)
  → cache tx in Redis (monitoredTransactions:{txId}:{type})
  → Confirmator polls until N confirmations reached
  → POST {EXCHANGE_URL}/cg/deposit  (webhook)
  → Collector sweeps hot→cold
```

### Withdrawal
```
POST /withdraw {uuid, address, amount, asset}
  → look up hot wallet, decrypt private key
  → build & sign chain-specific tx (UTXO/EVM/TVM)
  → broadcast to network
  → write to DB + Redis
  → POST {EXCHANGE_URL}/cg/withdraw (webhook, status=pending)
  → Confirmator fires webhook again once confirmed
```

### Wallet Creation
```
GET /wallet?chain=bitcoin
  → generate chain-specific address + keypair
  → encrypt private key (AES-256-CBC)
  → INSERT into addresses + addresses_networks tables
  → push address to Redis monitoredAddresses_{chain}
  → return {address}
```

---

## Database Schema (tables)

| Table                 | Purpose                                                  |
|-----------------------|----------------------------------------------------------|
| `transactions`        | Full tx history (hash, amount, status, collected flag)   |
| `addresses`           | Wallet addresses + AES-encrypted private keys            |
| `addresses_networks`  | Many-to-many address↔chain mapping                       |
| `block_heights`       | Last processed block per chain (prevents re-processing)  |

Transaction `status` values: `pending` → `confirmed` / `failed`  
Transaction `type` values: `deposit`, `withdrawal`

---

## HTTP API

**Auth:** `X-TOKEN` header required on all requests.

```
GET  /wallet?chain={bitcoin|ethereum|binance|tron}
  → { "address": "..." }

GET  /wallet/balances?address={address}
  → { "address": "...", "balances": [...] }

POST /withdraw
  Body: {
    "uuid":    "...",         // exchange-side withdrawal ID
    "address": "...",         // recipient
    "amount":  "1.5",
    "asset":   "btc"          // btc | eth | bnb | trx
                              // or token: "ethereum_0xdac17f958d2ee523a2206206994597c13d831ec7"
  }
  → { "success": "txHash" } | { "error": "..." }
```

---

## Exchange Webhook Payloads

```go
// Deposit notification → POST {EXCHANGE_URL}/cg/deposit
DepositMessage {
    Address string  // to address (user's deposit address)
    From    string  // sender address
    Amount  float64
    Asset   string  // "BTC", "ETH", "USDT", ...
    Network string  // "bitcoin", "ethereum", "binance", "tron"
    Hash    string  // tx hash
}

// Withdrawal notification → POST {EXCHANGE_URL}/cg/withdraw
WithdrawMessage {
    UUID   string  // same UUID sent in /withdraw request
    Hash   string  // tx hash
    Status string  // "pending" | "confirmed" | "failed"
}
```

---

## Configuration (`.env`)

```bash
# API
API_PORT=8080
API_TOKEN=secret

# Database
DB_HOST= DB_PORT= DB_USER= DB_PASS= DB_NAME=

# Redis
REDIS_HOST= REDIS_PORT= REDIS_USER= REDIS_PASS= REDIS_DB=

# RPC Nodes
BTC_URL=        # Bitcoin node RPC
ETH_URL=        # Ethereum node RPC
BSC_URL=        # BSC node RPC
TRON_URL=       # TRON node RPC
TRON_KEY=       # TRON API key
TRON_URLS=      # comma-separated TRON RPC URLs for key rotation

# Network
CHAIN_ID=testnet  # or mainnet (Bitcoin)

# Contract Addresses
ETH_USDT_CONTRACT=
BSC_USDT_CONTRACT=
TRON_USDT_CONTRACT=

# Hot Wallets (with private keys — encrypted at rest in DB, plaintext in env for hot wallet)
BTC_HOT_WALLET=     BTC_COLD_WALLET=
ETH_HOT_WALLET=     ETH_HOT_WALLET_PRIVATE=     ETH_COLD_WALLET=
BSC_HOT_WALLET=     BSC_HOT_WALLET_PRIVATE=     BSC_COLD_WALLET=
TRON_HOT_WALLET=    TRON_HOT_WALLET_PRIVATE=    TRON_COLD_WALLET=

# Encryption
CRYPTO_SEC=   CRYPTO_SALT=

# Polling (seconds)
BTC_COLLECT_PERIOD=   ETH_COLLECT_PERIOD=   BSC_COLLECT_PERIOD=   TRON_COLLECT_PERIOD=
CONFIRMATOR_TIMEOUT=

# Confirmations required
BTC_CONFIRMATIONS=   ETH_CONFIRMATIONS=   BSC_CONFIRMATIONS=   TRON_CONFIRMATIONS=

# Callback
EXCHANGE_URL=https://your-exchange.com
```

---

## Notable Patterns

- **TRON API key rotation:** Multiple RPC URLs in a queue, rotated every hour to avoid rate limits.
- **Redis address list:** Per-chain list of monitored addresses reloaded at startup, appended on wallet creation.
- **Redis tx cache:** `monitoredTransactions:{txId}:{type}` — deleted once confirmed.
- **Block height tracking:** DB stores last processed block per chain to resume safely after restart.
- **AES-256-CBC encryption:** All user wallet private keys encrypted before DB write; hot wallet key lives in env.
- **UTXO tracking:** Bitcoin UTXOs stored in DB with `vout` index; marked `withdraw_used=true` after spend.

---

## File Map

```
/cmd/
  main.go          ← API server entry
  deposits.go      ← Deposit listener entry
  collector.go     ← Collector entry
  confirmator.go   ← Confirmator entry
  encode.go / decode.go  ← CLI key encryption tools

/app/app.go        ← Bootstrap: DB, Redis, all services
/config/config.go  ← Viper .env loader

/api/
  api.go           ← HTTP server setup
  endpoints.go     ← Route handlers
  middleware.go    ← Token auth + method validation

/services/
  wallets/         ← Wallet creation & balance queries
  withdrawals/     ← Withdrawal processing
  deposits/        ← Per-chain block polling
  collector/       ← Hot→cold sweeping
  confirmator/     ← Confirmation tracking

/adapters/
  utxo/            ← Bitcoin-specific logic
  evm/             ← Ethereum & BSC logic
  tvm/             ← TRON logic

/shared/
  database/db.go       ← PostgreSQL connection + queries
  cache/cache.go       ← Redis operations
  crypto/              ← AES encrypt/decrypt
  rpc_client.go        ← Generic JSON-RPC client
  notifications/       ← Exchange webhook sender
```

---

## OTC / Exchange Integration Assessment

### What crypto-gate provides
- Deposit address generation per user per chain
- Real-time deposit detection and exchange notification
- Withdrawal execution from hot wallet with confirmation tracking
- Hot→cold fund sweeping

### What it does NOT provide
- Order books, matching engine, trading pairs
- OTC pricing or quote engine
- User account management / KYC
- Fiat on/off ramps
- Internal ledger / accounting
- Multi-signature or MPC wallet support
- Any REST API beyond wallet + withdraw

### Fit for a crypto exchange with OTC
**crypto-gate is a good fit as the blockchain I/O layer.** It plugs into an exchange backend via:
1. HTTP API for wallet creation and withdrawals (exchange calls crypto-gate)
2. Webhook callbacks for deposit/withdrawal status (crypto-gate calls exchange)

The exchange backend is responsible for the ledger, user accounts, OTC logic, and order flow. crypto-gate only moves funds on-chain. The integration surface is small and well-defined.

**Gaps to fill before using in OTC context:**
- No support for stablecoins beyond USDT (add token contracts in config)
- Hot wallet private key is in env plaintext (acceptable for hot wallet, but consider HSM for OTC volumes)
- No webhook retry logic — if exchange endpoint is down, notification is lost
- No built-in fee estimation API (fee is computed internally at withdrawal time)
- Single hot wallet per chain — high-volume OTC may need multiple or MPC
