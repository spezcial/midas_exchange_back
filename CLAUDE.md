# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CaspianEx is an exchange platform where users deposit their currency and do real time exchange into another currency. Then they can do a withdrawal. 
Deposits are going to be handled via crypto-gate (not implemented yet). Withdrawals are handled manually.

## Common Commands

```bash
# Development
make run              # Run locally (requires docker-compose up -d postgres redis first)
make dev              # Start all services + follow logs
make app-refresh      # Rebuild and restart only the app container (fast iteration)

# Build & Test
make build            # Build Go binary to bin/server
make test             # Run all tests
make lint             # Format code (go fmt) and run go vet

# Database
make db-migrate       # Run migrations
make db-reset         # Reset database (deletes all data)
make db-shell         # Open PostgreSQL shell

# Docker
make up               # Start all services (postgres, redis, app)
make down             # Stop all services
make logs             # Follow all service logs
make logs-app         # Follow app logs only
```

## Architecture
We use clean architecture and clean code.
### Layered Structure
```
cmd/server/           → Entry point, router setup, WebSocket service
internal/
  api/client/         → Client-facing HTTP handlers (auth, wallets, exchanges)
  api/admin/          → Admin HTTP handlers (users, exchanges, rates management)
  api/middleware/     → Auth, CORS, logging, recovery middleware
  service/            → Business logic layer
  repository/         → Database access with caching integration
  domain/             → Core domain entities
  dto/                → Data transfer objects
  models/             → Database models
pkg/
  auth/               → JWT token management
  cache/              → In-memory cache (go-cache) with warm-up loader
  config/             → Environment configuration + company payment details
  database/           → PostgreSQL connection
  email/              → SMTP email service
  logger/             → Structured logging
  validator/          → Request validation
  worker/             → Background workers (rate updater)
```

### Key Patterns

- **Repository + Cache**: Repositories integrate with `CacheService` for read-through caching
- **Cache Warm-up**: `CacheLoader` pre-loads data on startup (currencies, rates, users)
- **Background Workers**: `RateUpdater` periodically updates exchange rates
- **WebSocket**: Real-time exchange rate updates via `/ws` endpoint
- **Graceful Shutdown**: Proper cleanup of workers, connections, and server
- **Repository interfaces**: `internal/repository/interfaces.go` defines `UserRepo` and `WalletRepo` interfaces used by services so they can be unit-tested with in-memory fakes
- **2FA / action tokens**: `TwoFactorService` (Redis-backed) manages OTP codes, pending-2FA sessions, and purpose-bound single-use action tokens. `WebAuthnService` handles passkey ceremonies.
- **JWT blocklist**: On logout the JTI is stored in Redis with the token's remaining TTL. `AuthMiddleware` checks the blocklist on every request.
- **Login fingerprint**: SHA256(IP+UA) stored in `user_fingerprints`; new-device Telegram alert sent on first use.
- **Action token pattern**: sensitive actions (withdraw, change password) require a pre-verified, purpose-bound, single-use token (5-min TTL) obtained via the challenge/verify flow.

### Roles (RBAC)

All roles share the same `users` table — staff accounts just have a different `role` value and no wallets.

| Role | Description |
|---|---|
| `client` | End-user; created via `/auth/register`; has wallets |
| `admin` | Admin panel access |
| `super_admin` | Full admin access + staff management |
| `operator` | OTC order operator; manages OTC orders |
| `support` | Support tickets (future) |
| `aml_specialist` | AML review (future) |
| `compliance` | Compliance review (future) |

- `UserRole.IsStaffRole()` — true for any non-client role
- `UserRole.IsValidRole()` — validates against the known set
- `RequireRole(roles ...string)` — variadic middleware, allows any of the listed roles
- Staff log in via the same `/api/v1/auth/login` endpoint

### API Structure

- `/health`, `/health/live`, `/health/ready` - Health checks
- `/ws` - WebSocket for real-time rates
- `/api/v1/auth/*` - Public auth endpoints
- `/api/v1/exchange-rates` - Public rates
- `/api/v1/*` - Protected client endpoints (JWT required)
- `/api/v1/admin/*` - Admin endpoints (JWT + `admin` or `super_admin` role required)
- `/api/v1/admin/super/*` - Super-admin only: staff CRUD

### Staff Management Endpoints (`/api/v1/admin/super/`)

| Method | Path | Description |
|---|---|---|
| GET | `/staff` | List all staff (non-client users) |
| POST | `/staff` | Create staff account; returns `{staff, temp_password}` |
| PUT | `/staff/{id}` | Update staff name/role/is_active |
| DELETE | `/staff/{id}` | Deactivate staff (sets `is_active=false`) |

### OTC Workflow

Clients with KYC level ≥ 2 can create OTC orders. Operators negotiate via in-order chat (text + offer cards). Offer acceptance → `awaiting_payment` with 30-min deadline. Payment is external; operator confirms receipt, then marks complete.

**Status machine:** `awaiting_review` → `negotiating` → `awaiting_payment` → `payment_received` → `completed`. Cancelled or expired are terminal. Expiry is lazy (set on `GetByUID` if deadline passed).

**Client OTC endpoints** (`/api/v1/otc/`): create, list, get order; send messages/offers; accept/reject offers; cancel.
**Admin/Operator OTC endpoints** (`/api/v1/admin/otc/`): list/get all orders; take, message, offer, accept/reject, confirm payment, complete, cancel.

Key files: `internal/domain/otc.go`, `internal/service/otc_service.go`, `internal/repository/otc_repository.go`, `const/queries/otc_repo.go`, `internal/service/notification_service.go` (NoOp stub).

### OTC-013: History & Reporting (implemented 2026-04-03)

**New admin endpoints** (`/api/v1/admin/otc/`, role: admin/super_admin/operator):
| Method | Path | Description |
|---|---|---|
| GET | `/analytics` | Aggregate stats + time-series breakdown |
| GET | `/orders/export` | Stream CSV download |
| GET | `/orders/{uid}/audit-log` | Operator action log for one order |

**Extended `/admin/otc/orders` filter params:** `from_date`, `to_date` (YYYY-MM-DD), `from_currency_id`, `to_currency_id`, `operator_id` (in addition to `status`, `email`).

**Analytics params:** `from`, `to` (YYYY-MM-DD), `granularity` (day/week/month).

**Audit log** written automatically (fire-and-forget goroutine) after: took_order, sent_offer, accepted_offer, rejected_offer, confirmed_payment, completed, cancelled.

### Database

**Always update CLAUDE.md** when adding migrations, new endpoints, or domain changes.

PostgreSQL with golang-migrate. Migrations in `migrations/` directory:
- `000001_init_schema` - Core tables (users, wallets, currencies, exchanges, transactions)
- `000002_seed_currencies` - Pre-seeded crypto (BTC, ETH, USDT, etc.) and fiat (KZT, USD, EUR)
- `000006_add_roles` - CHECK constraint on `users.role` for all valid role values
- `000007_otc` - `otc_orders` and `otc_messages` tables
- `000010_otc_audit_log` - `otc_audit_log` table (actor_id, actor_role, action, details, created_at)
- `000011_crypto_deposit_addresses` - `deposit_addresses` table (user_id, currency_id, chain, address)
- `000016_2fa` - adds `phone_verified` to users, creates `passkey_credentials` and `user_fingerprints` tables

### 2FA / Security Endpoints

**Auth** (`/api/v1/auth/`):
| Method | Path | Description |
|---|---|---|
| POST | `/login` | Returns `{auth}` or `{two_fa_required, temp_token, methods}` |
| POST | `/logout` | Blocklists JWT JTI in Redis |
| POST | `/2fa/telegram` | Sends Telegram OTP to pending login session's phone |
| POST | `/2fa/verify-telegram` | Verifies OTP, issues real tokens |
| POST | `/2fa/passkey/begin` | Begin passkey assertion for login |
| POST | `/2fa/passkey/finish` | Finish passkey assertion, issues real tokens |
| POST | `/forgot-password/send` | Sends OTP to phone (always 200, no enumeration) |
| POST | `/forgot-password/verify` | Verifies OTP, returns action token |
| POST | `/forgot-password/reset` | Resets password with action token |

**Phone management** (`/api/v1/`):
| Method | Path | Description |
|---|---|---|
| POST | `/phone/send-otp` | Sends OTP to new phone (checks uniqueness first) |
| POST | `/phone/verify` | Verifies OTP and saves phone |
| DELETE | `/phone` | Removes phone (requires passkey to exist) |

**Passkey management** (`/api/v1/`):
| Method | Path | Description |
|---|---|---|
| POST | `/passkeys/begin` | Begin passkey registration ceremony |
| POST | `/passkeys/finish` | Finish passkey registration ceremony |
| GET | `/passkeys` | List registered passkeys |
| DELETE | `/passkeys/{id}` | Delete passkey (requires phone to exist) |

**Action token flow** (`/api/v1/`):
| Method | Path | Description |
|---|---|---|
| POST | `/action/challenge` | Sends OTP to verified phone for sensitive action |
| POST | `/action/verify-telegram` | Verifies OTP, returns action_token |
| POST | `/action/passkey/begin` | Begin passkey assertion for action token |
| POST | `/action/passkey/finish` | Finish passkey assertion, returns action_token |

Action token is then passed in `action_token` field of withdraw/change-password requests.