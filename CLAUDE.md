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

### API Structure

- `/health`, `/health/live`, `/health/ready` - Health checks
- `/ws` - WebSocket for real-time rates
- `/api/v1/auth/*` - Public auth endpoints
- `/api/v1/exchange-rates` - Public rates
- `/api/v1/*` - Protected client endpoints (JWT required)
- `/api/v1/admin/*` - Admin endpoints (JWT + admin role required)

### Database

PostgreSQL with golang-migrate. Migrations in `migrations/` directory:
- `000001_init_schema` - Core tables (users, wallets, currencies, exchanges, transactions)
- `000002_seed_currencies` - Pre-seeded crypto (BTC, ETH, USDT, etc.) and fiat (KZT, USD, EUR)