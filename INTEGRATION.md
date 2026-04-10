# Crypto Gate — Platform Integration Guide

This document is the complete reference for a platform (exchange, wallet app, or any third-party service) integrating with the Crypto Gate payment gateway.

---

## Overview

Crypto Gate is a hot-wallet manager and deposit/withdrawal processor for multiple blockchains. It exposes:

- A **REST API** that your backend calls to create addresses and request withdrawals.
- A **webhook** (callback) that Crypto Gate calls on your backend when a deposit is confirmed or a withdrawal is processed.

**Supported chains and assets**

| Chain | Native asset | Token | Asset identifier |
|-------|-------------|-------|-----------------|
| Bitcoin | BTC | — | `btc` |
| Ethereum | ETH | USDT (ERC-20) | `eth` / `ethereum_0xdac17f958d2ee523a2206206994597c13d831ec7` |
| BSC | BNB | USDT (BEP-20) | `bnb` / `binance_0x55d398326f99059ff775485246999027b3197955` |
| TRON | TRX | USDT (TRC-20) | `trx` / `tron_tr7nhqjekqxgtci8q8zy4pl8otszgjlj6t` |

---

## Setup

Before making any API calls, your platform must be registered in Crypto Gate. This is a one-time operation performed by the gateway operator:

```bash
make platform-add \
  slug=your-platform-name \
  callback_url=https://your-platform.com \
  api_token=your-secret-token
```

This gives your platform:

| Value | Description |
|-------|-------------|
| **slug** | Identifies your platform in every API request |
| **callback_url** | The base URL where Crypto Gate sends deposit/withdrawal notifications |
| **api_token** | The token Crypto Gate includes in the `X-TOKEN` header of webhook calls to your backend |

Your platform also receives a separate **gateway API token** (`API_TOKEN` in the gateway config), which you include in every request _to_ Crypto Gate.

---

## Authentication

All requests to the gateway API must include the `X-TOKEN` header:

```
X-TOKEN: <gateway-api-token>
```

Requests with a missing or incorrect token receive `400 Wrong Token`.

Conversely, when Crypto Gate sends webhooks _to your platform_, it includes your platform's `api_token` in the same `X-TOKEN` header. **You must validate this header** before processing any webhook payload.

---

## API Reference

Base URL: `https://your-gateway-host:8080`

---

### Create deposit address

Generate a new blockchain address for a user deposit, scoped to your platform.

```
GET /wallet?chain={chain}&platform={slug}
```

**Query parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | Yes | `bitcoin` \| `ethereum` \| `binance` \| `tron` |
| `platform` | Yes | Your platform slug |

**Headers**

```
X-TOKEN: <gateway-api-token>
```

**Response `200 OK`**

```json
{
  "address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
}
```

**Error responses**

| Code | Body | Cause |
|------|------|-------|
| 400 | `Invalid chain` | Unknown or unsupported chain |
| 400 | `Missing platform` | `platform` param absent |
| 400 | `Invalid platform` | Slug not registered |
| 400 | `Wrong Token` | Missing or incorrect `X-TOKEN` |
| 500 | `Internal server error` | Key generation or DB failure |

**Notes**

- Each call generates a **new, unique address**. Do not reuse addresses across users.
- The address is immediately added to the deposit scanner. Any funds sent to it will be detected on the next block.
- The private key is encrypted (AES-256-CBC) before storage; the gateway operator never sees plaintext keys.

**Example**

```bash
curl -X GET "https://gateway.example.com/wallet?chain=ethereum&platform=myexchange" \
     -H "X-TOKEN: gateway-secret"
```

```json
{ "address": "0xAbCd1234..." }
```

---

### List all platform addresses

Fetch every address ever created for your platform, across all chains. Use this to sync your local address book or verify that no addresses are missing after a migration or incident.

```
GET /wallet/addresses?platform={slug}
```

**Query parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `platform` | Yes | Your platform slug |

**Headers**

```
X-TOKEN: <gateway-api-token>
```

**Response `200 OK`**

```json
{
  "platform": "myexchange",
  "count": 3,
  "addresses": [
    {
      "address":    "0xAbCd1234...",
      "chain":      "ethereum",
      "status":     "active",
      "created_at": "2024-11-01T10:23:45Z"
    },
    {
      "address":    "bc1qxy2kgdygjrs...",
      "chain":      "bitcoin",
      "status":     "active",
      "created_at": "2024-10-30T08:00:00Z"
    },
    {
      "address":    "TJgmah4VbBPi8hV...",
      "chain":      "tron",
      "status":     "active",
      "created_at": "2024-10-29T14:11:30Z"
    }
  ]
}
```

**Fields**

| Field | Type | Description |
|-------|------|-------------|
| `platform` | string | Echo of the requested platform slug |
| `count` | int | Total number of addresses returned |
| `addresses[].address` | string | Blockchain address |
| `addresses[].chain` | string | `bitcoin` \| `ethereum` \| `binance` \| `tron` |
| `addresses[].status` | string | Always `"active"` for usable addresses |
| `addresses[].created_at` | string | ISO 8601 UTC timestamp of address creation |

Results are ordered newest-first. An empty `addresses` array (with `count: 0`) is returned if the platform has no addresses yet.

**Error responses**

| Code | Body | Cause |
|------|------|-------|
| 400 | `Missing platform` | `platform` param absent |
| 400 | `Invalid platform` | Slug not registered |
| 400 | `Wrong Token` | Missing or incorrect `X-TOKEN` |

**Example**

```bash
curl -X GET "https://gateway.example.com/wallet/addresses?platform=myexchange" \
     -H "X-TOKEN: gateway-secret"
```

**Suggested use cases**

- On-startup sync: compare your local address table against the gateway to catch any gaps.
- Periodic audit: verify no addresses are missing or duplicated.
- After an outage: reconcile which addresses were created while your system was unavailable.

---

### Get address balances

Query the current on-chain balances for an address managed by the gateway.

```
GET /wallet/balances?address={address}
```

**Query parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `address` | Yes | A previously created gateway address |

**Headers**

```
X-TOKEN: <gateway-api-token>
```

**Response `200 OK`**

```json
{
  "address": "0xAbCd1234...",
  "balances": [
    { "eth": "0.052000000" },
    { "ethereum_0xdac17f958d2ee523a2206206994597c13d831ec7": "100.000000000" }
  ]
}
```

The balance array always includes the native asset. If the chain supports USDT, a second entry is included. Balance values are decimal strings.

**Asset keys in balance response**

| Chain | Native key | Token key |
|-------|-----------|-----------|
| Bitcoin | `btc` | — |
| Ethereum | `eth` | `ethereum_0xdac17f958d2ee523a2206206994597c13d831ec7` |
| BSC | `bnb` | `binance_0x55d398326f99059ff775485246999027b3197955` |
| TRON | `trx` | `tron_tr7nhqjekqxgtci8q8zy4pl8otszgjlj6t` |

---

### Request a withdrawal

Send funds from the gateway hot wallet to an external address on behalf of a user.

```
POST /withdraw
Content-Type: application/json
X-TOKEN: <gateway-api-token>
```

**Request body**

```json
{
  "uuid":     "order-uuid-from-your-system",
  "address":  "recipient-blockchain-address",
  "amount":   "1.5",
  "asset":    "eth",
  "platform": "myexchange"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `uuid` | string | Your internal withdrawal ID. Echoed back in the webhook so you can match the callback to the request. |
| `address` | string | On-chain destination address |
| `amount` | string | Decimal amount to send (e.g. `"1.5"`, `"100.00"`) |
| `asset` | string | See **Asset identifiers** table below |
| `platform` | string | Your platform slug |

**Asset identifiers for withdrawals**

| Asset | `asset` value |
|-------|--------------|
| Bitcoin | `btc` |
| Ethereum | `eth` |
| BNB | `bnb` |
| TRON | `trx` |
| USDT on Ethereum | `ethereum_0xdac17f958d2ee523a2206206994597c13d831ec7` |
| USDT on BSC | `binance_0x55d398326f99059ff775485246999027b3197955` |
| USDT on TRON | `tron_tr7nhqjekqxgtci8q8zy4pl8otszgjlj6t` |

**Amount precision**

| Asset | Unit | Example |
|-------|------|---------|
| BTC | BTC (up to 8 decimals) | `"0.00150000"` |
| ETH / BNB | Ether / BNB (up to 18 decimals) | `"1.500000000"` |
| TRX | TRX (up to 6 decimals) | `"50.000000"` |
| USDT (any chain) | USDT (up to 6 decimals) | `"100.000000"` |

**Response `200 OK` — success**

```json
{
  "success": "0xabc123...transactionHash"
}
```

**Response `200 OK` — failure**

```json
{
  "error": "something went wrong"
}
```

> Both success and failure return HTTP 200. Distinguish them by the presence of `"success"` or `"error"` in the body. When the withdrawal fails, the gateway also sends a `status: "failed"` webhook to your callback URL.

**Error conditions**

| Condition | Behaviour |
|-----------|-----------|
| Invalid platform | `400 Invalid platform` (no webhook sent) |
| Missing `platform` field | `400 Missing platform` |
| Unsupported asset | `200` with `{"error": "something went wrong"}` + failed webhook |
| Insufficient hot wallet funds | `200` with `{"error": "..."}` + failed webhook |
| Node connectivity failure | `200` with `{"error": "..."}` + failed webhook |

**Example — withdraw ETH**

```bash
curl -X POST "https://gateway.example.com/withdraw" \
     -H "X-TOKEN: gateway-secret" \
     -H "Content-Type: application/json" \
     -d '{
       "uuid": "wd-8821",
       "address": "0xRecipient...",
       "amount": "0.5",
       "asset": "eth",
       "platform": "myexchange"
     }'
```

**Example — withdraw USDT on TRON**

```bash
curl -X POST "https://gateway.example.com/withdraw" \
     -H "X-TOKEN: gateway-secret" \
     -H "Content-Type: application/json" \
     -d '{
       "uuid": "wd-8822",
       "address": "TRecipientBase58Address",
       "amount": "250.00",
       "asset": "tron_tr7nhqjekqxgtci8q8zy4pl8otszgjlj6t",
       "platform": "myexchange"
     }'
```

---

## Webhooks (Callbacks)

Crypto Gate calls your platform's backend at `{callback_url}/cg/deposit` and `{callback_url}/cg/withdraw`.

**Always validate the `X-TOKEN` header** on incoming webhook requests. The token value is the `api_token` you were assigned during platform setup.

Webhooks are sent with up to **3 retry attempts** (with backoff) if your server returns a non-2xx response or is unreachable.

---

### Deposit confirmed

Called when an incoming transaction to one of your platform's addresses has reached the required number of confirmations.

```
POST {callback_url}/cg/deposit
X-TOKEN: <your-platform-api-token>
Content-Type: application/json
```

**Body**

```json
{
  "address": "0xYourDepositAddress",
  "from":    "0xSenderAddress",
  "amount":  "100.000000000",
  "asset":   "ethereum_0xdac17f958d2ee523a2206206994597c13d831ec7",
  "network": "ethereum",
  "hash":    "0xTransactionHash"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `address` | string | The deposit address on your platform (created via `GET /wallet`) |
| `from` | string | The sender's blockchain address |
| `amount` | string | Amount received, decimal string |
| `asset` | string | Asset identifier (see table below) |
| `network` | string | Chain name: `bitcoin` \| `ethereum` \| `binance` \| `tron` |
| `hash` | string | Transaction hash |

**`asset` values in deposit webhooks**

| Asset received | `asset` value |
|----------------|--------------|
| BTC | `BTC` |
| ETH | `ETH` |
| BNB | `BNB` |
| TRX | `TRX` |
| USDT on Ethereum | `ethereum_0xdac17f958d2ee523a2206206994597c13d831ec7` |
| USDT on BSC | `binance_0x55d398326f99059ff775485246999027b3197955` |
| USDT on TRON | `tron_tr7nhqjekqxgtci8q8zy4pl8otszgjlj6t` |

**Required confirmations before webhook fires**

| Chain | Confirmations |
|-------|--------------|
| Bitcoin | 3 (configurable) |
| Ethereum | 12 (configurable) |
| BSC | 12 (configurable) |
| TRON | 20 (configurable) |

**Your endpoint must return `2xx`** to acknowledge the webhook. Non-2xx responses trigger retries.

**Example handler (pseudocode)**

```python
@app.post("/cg/deposit")
def deposit_callback(request):
    token = request.headers.get("X-TOKEN")
    if token != PLATFORM_API_TOKEN:
        return Response(status=401)

    body = request.json()
    user = db.find_user_by_deposit_address(body["address"])
    if not user:
        return Response(status=200)  # unknown address, acknowledge anyway

    db.credit_user(
        user_id=user.id,
        amount=body["amount"],
        asset=body["asset"],
        network=body["network"],
        tx_hash=body["hash"],
        from_address=body["from"],
    )
    return Response(status=200)
```

---

### Withdrawal processed

Called immediately after the withdrawal transaction is broadcast, whether it succeeded or failed.

```
POST {callback_url}/cg/withdraw
X-TOKEN: <your-platform-api-token>
Content-Type: application/json
```

**Body**

```json
{
  "UUID":   "order-uuid-from-your-system",
  "hash":   "0xTransactionHash",
  "status": "success"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `UUID` | string | The `uuid` you sent in `POST /withdraw` |
| `hash` | string | Transaction hash. Empty string `""` if `status` is `"failed"` |
| `status` | string | `"success"` or `"failed"` |

**Example handler (pseudocode)**

```python
@app.post("/cg/withdraw")
def withdraw_callback(request):
    token = request.headers.get("X-TOKEN")
    if token != PLATFORM_API_TOKEN:
        return Response(status=401)

    body = request.json()
    withdrawal = db.find_withdrawal(uuid=body["UUID"])
    if not withdrawal:
        return Response(status=200)

    if body["status"] == "success":
        db.mark_withdrawal_complete(withdrawal.id, tx_hash=body["hash"])
    else:
        db.mark_withdrawal_failed(withdrawal.id)
        # optionally re-queue or notify user

    return Response(status=200)
```

---

## End-to-End Flows

### Deposit flow

```
User submits deposit intent on your platform
        │
        ▼
GET /wallet?chain=ethereum&platform=myexchange
        │
        ▼
Show address to user: 0xAbc...
        │
        ▼
User sends funds on-chain
        │
        ▼
Crypto Gate detects transaction in new block
        │
        ▼
Waits for N confirmations (12 for Ethereum)
        │
        ▼
POST {your_callback_url}/cg/deposit  ←── X-TOKEN validation
        │
        ▼
Credit user balance in your system
```

### Withdrawal flow

```
User requests withdrawal on your platform
        │
        ▼
Validate user balance, fraud checks, etc.
        │
        ▼
POST /withdraw  {uuid, address, amount, asset, platform}
        │
        ├─► success: {"success": "0xTxHash..."}
        │       │
        │       ▼
        │   POST {callback_url}/cg/withdraw  {status: "success", hash: "0x..."}
        │
        └─► failure: {"error": "..."}
                │
                ▼
            POST {callback_url}/cg/withdraw  {status: "failed", hash: ""}
```

> **Important:** Do not consider a withdrawal final based on the API response alone. Always wait for the `status: "success"` webhook and record the `hash` before updating your ledger.

---

## Idempotency

- **Deposits:** The gateway deduplicates transactions by `(tx_hash, vout)` at the DB level. Your callback will be called exactly once per confirmed output. Safe to rely on `hash` + `address` as a unique key.
- **Withdrawals:** The gateway does not retry failed withdrawals. If you receive `status: "failed"`, you must decide whether to resubmit via `POST /withdraw` with a new `uuid`.

---

## Error Handling Checklist

- [ ] Always check for both `"success"` and `"error"` keys in the `POST /withdraw` response body.
- [ ] Validate `X-TOKEN` on every incoming webhook before processing.
- [ ] Return `2xx` from your webhook endpoints even if you choose to discard the event (unknown address, duplicate, etc.) — non-2xx triggers retries.
- [ ] Do not credit a deposit until you receive the confirmed webhook (not the moment the user sends funds).
- [ ] Store the `hash` from the withdrawal webhook as your blockchain proof.
- [ ] Handle the case where a withdrawal webhook arrives before the API response (rare but possible under load).

---

## Quick Reference

| Operation | Method | Path |
|-----------|--------|------|
| Create deposit address | `GET` | `/wallet?chain=&platform=` |
| List all platform addresses | `GET` | `/wallet/addresses?platform=` |
| Query address balances | `GET` | `/wallet/balances?address=` |
| Request withdrawal | `POST` | `/withdraw` |

| Event | Your endpoint |
|-------|--------------|
| Deposit confirmed | `POST {callback_url}/cg/deposit` |
| Withdrawal processed | `POST {callback_url}/cg/withdraw` |

| Header | Direction | Value |
|--------|-----------|-------|
| `X-TOKEN` | Platform → Gateway | Gateway API token |
| `X-TOKEN` | Gateway → Platform | Your platform's `api_token` |
