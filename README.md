# CaspianEx - OTC Crypto Exchange Backend

A manual OTC (Over-The-Counter) cryptocurrency exchange backend built with Go, featuring manager-verified order processing, multi-currency support, and comprehensive admin controls.

## Business Model

This is an **OTC exchange** where:
1. User creates an exchange order (e.g., USDT → KZT)
2. System shows company payment details (crypto wallet or bank account)
3. User sends payment and provides proof (transaction hash or bank reference with UID)
4. Manager gets notified and verifies the payment
5. Manager manually sends the exchanged currency to the user
6. Manager marks the order as completed or rejected

**No automatic order matching** - all orders are processed manually by managers.

## Features

- **User Authentication**: JWT-based authentication with refresh tokens
- **Role-Based Access Control**: Separate client and admin APIs
- **OTC Order System**: Manual exchange order processing
- **Payment Instructions**: Automatic generation of payment details with unique UIDs
- **Manager Dashboard**: Admin panel for order verification and processing
- **Email Notifications**: Automated notifications at each stage
- **Multi-Currency Support**: Crypto (BTC, ETH, USDT, etc.) and Fiat (KZT, USD, EUR)
- **Exchange Rate Management**: Configurable exchange rates
- **In-Memory Caching**: Fast data access with go-cache
- **Graceful Shutdown**: Safe termination of services
- **Database Migrations**: Version-controlled schema management
- **Docker Support**: Easy deployment with docker-compose

## Architecture

```
exchange-backend/
├── cmd/server/              # Application entry point
├── internal/
│   ├── api/                 # HTTP handlers
│   │   ├── client/          # Client-facing endpoints
│   │   ├── admin/           # Admin endpoints (manager actions)
│   │   └── middleware/      # Auth, logging, CORS
│   ├── service/             # Business logic
│   ├── repository/          # Database access
│   ├── domain/              # Core entities
│   └── worker/              # (removed - no automatic matching)
├── pkg/                     # Reusable packages
│   ├── auth/                # JWT utilities
│   ├── cache/               # In-memory cache
│   ├── config/              # Configuration + payment details
│   ├── database/            # Database connection
│   ├── email/               # Email service
│   ├── logger/              # Structured logging
│   └── validator/           # Request validation
├── migrations/              # SQL migrations
└── docker/                  # Docker configs
```

## Tech Stack

- **Language**: Go 1.22
- **Router**: Chi
- **Database**: PostgreSQL 16
- **Cache**: In-memory (go-cache) + Redis (available, not used yet)
- **Authentication**: JWT with bcrypt
- **Migration Tool**: golang-migrate
- **Container**: Docker & Docker Compose

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Make (optional)
- golang-migrate CLI (for manual migrations)

### Installation

1. Clone the repository:
```bash
cd exchange-backend
```

2. Copy environment variables:
```bash
cp .env.example .env
```

3. Update `.env` with your configuration:
```bash
# Required changes:
JWT_SECRET=<generate-a-secure-secret>
SMTP_USERNAME=<your-email>
SMTP_PASSWORD=<your-app-password>

# Company payment details:
COMPANY_BTC_WALLET=<your-btc-address>
COMPANY_ETH_WALLET=<your-eth-address>
COMPANY_USDT_WALLET=<your-usdt-address>
COMPANY_BANK_NAME=Kaspi Bank
COMPANY_BANK_ACCOUNT=<your-account>
COMPANY_BANK_IBAN=<your-iban>
COMPANY_BANK_SWIFT=CASPKZKA
```

### Running with Docker (Recommended)

1. Start all services:
```bash
docker-compose up -d
```

This will start:
- PostgreSQL database on port 5432
- Redis on port 6379
- API server on port 8080
- Automatic database migrations

2. Check logs:
```bash
docker-compose logs -f app
```

3. Stop services:
```bash
docker-compose down
```

### Running Locally

1. Start PostgreSQL and Redis:
```bash
docker-compose up -d postgres redis
```

2. Run migrations:
```bash
make migrate-up
```

3. Start the application:
```bash
make run
# or
go run cmd/server/main.go
```

## Order Flow

### 1. User Creates Order

**Endpoint**: `POST /api/v1/orders`

```json
{
  "from_currency_code": "USDT",
  "to_currency_code": "KZT",
  "from_amount": 100.0,
  "user_payment_details": "Kaspi: +7 777 123 4567 or BTC: bc1q..."
}
```

**Response**: Order with payment instructions
```json
{
  "id": 1,
  "uid": "550e8400-e29b-41d4-a716-446655440000",
  "from_amount": 100.0,
  "to_amount": 45000.0,
  "exchange_rate": 450.0,
  "status": "pending",
  "payment_method": "crypto",
  "company_wallet_info": "TYvD6eJa4aZ8Zw7qXpUF1rnQxqZfqmRdBs",
  "user_payment_details": "Kaspi: +7 777 123 4567"
}
```

**Emails sent**:
- User: Order created with payment instructions
- Manager: New order notification

### 2. User Submits Payment Proof

**Endpoint**: `POST /api/v1/orders/{id}/submit-payment`

```json
{
  "payment_proof": "0x1234...abcd (transaction hash) or UID: 550e8400..."
}
```

**Status**: `pending` → `payment_sent`

**Emails sent**:
- User: Payment submission confirmed
- Manager: Action required - verify payment

### 3. Manager Reviews Order

**Admin Endpoint**: `GET /api/v1/admin/orders?status=payment_sent`

Manager sees:
- Order details
- User payment details
- Payment proof (tx hash or bank reference)
- All order information

### 4. Manager Marks as Processing (Optional)

**Endpoint**: `POST /api/v1/admin/orders/{id}/mark-processing`

**Status**: `payment_sent` → `processing`

### 5. Manager Processes Order

**Endpoint**: `POST /api/v1/admin/orders/{id}/process`

**Approve**:
```json
{
  "approved": true
}
```

**Reject**:
```json
{
  "approved": false,
  "rejection_reason": "Payment not received"
}
```

**Status**: `processing` → `completed` or `rejected`

**Emails sent**:
- User: Order completed or rejection notification with reason

## API Endpoints

### Public Endpoints

**Authentication**
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login user
- `POST /api/v1/auth/refresh` - Refresh access token
- `POST /api/v1/auth/logout` - Logout user

### Client Endpoints (Authenticated)

**Wallets**
- `GET /api/v1/wallets` - Get user wallets
- `POST /api/v1/wallets/deposit` - Deposit funds
- `POST /api/v1/wallets/withdraw` - Withdraw funds
- `GET /api/v1/transactions` - Get transaction history

**Orders**
- `POST /api/v1/orders` - Create new order
- `GET /api/v1/orders` - Get user orders
- `GET /api/v1/orders/{id}` - Get order details
- `POST /api/v1/orders/{id}/submit-payment` - Submit payment proof
- `DELETE /api/v1/orders/{id}` - Cancel pending order

### Admin Endpoints (Manager Role Required)

**Users**
- `GET /api/v1/admin/users` - List all users
- `GET /api/v1/admin/users/{id}` - Get user details

**Orders**
- `GET /api/v1/admin/orders` - List all orders (filter by status)
- `GET /api/v1/admin/orders/{id}` - Get order details
- `POST /api/v1/admin/orders/{id}/mark-processing` - Mark order as processing
- `POST /api/v1/admin/orders/{id}/process` - Complete or reject order

### Health Check
- `GET /health` - Health check endpoint

## Order Status Flow

```
pending → payment_sent → processing → completed
                                  ↘ rejected
                      ↘ canceled
```

- **pending**: Order created, waiting for user payment
- **payment_sent**: User submitted payment proof, waiting for manager verification
- **processing**: Manager is processing the order
- **completed**: Order successfully completed
- **rejected**: Order rejected by manager (with reason)
- **canceled**: Order canceled by user (only from pending)

## Configuration

All configuration is done via environment variables. See `.env.example` for available options.

### Critical Settings:

```bash
# Security
JWT_SECRET=<change-this-secret-in-production>

# Email
SMTP_USERNAME=<your-email>
SMTP_PASSWORD=<your-app-password>

# Company Payment Details (MUST configure)
COMPANY_BTC_WALLET=<your-btc-address>
COMPANY_ETH_WALLET=<your-eth-address>
COMPANY_USDT_WALLET=<your-usdt-address>
COMPANY_BANK_NAME=Kaspi Bank
COMPANY_BANK_ACCOUNT=<your-account>
COMPANY_BANK_IBAN=<your-iban>
COMPANY_BANK_SWIFT=<your-swift>
```

## Database

### Currencies

Pre-seeded with:
- **Crypto**: BTC, ETH, USDT, BNB, SOL, XRP, ADA, DOGE
- **Fiat**: KZT, USD, EUR

### Exchange Rates

Sample rates are seeded. **IMPORTANT**: Update exchange rates regularly in production:

```sql
UPDATE exchange_rates
SET rate = 450.50, updated_at = NOW()
WHERE from_currency_id = (SELECT id FROM currencies WHERE code = 'USDT')
  AND to_currency_id = (SELECT id FROM currencies WHERE code = 'KZT');
```

## Email Notifications

### User Notifications:
1. **Order Created**: Payment instructions with UID
2. **Payment Submitted**: Confirmation message
3. **Order Completed**: Success notification
4. **Order Rejected**: Rejection with reason

### Manager Notifications:
1. **New Order**: When user creates order
2. **Payment Submitted**: Action required notification

**Manager email** is sent to `SMTP_FROM` address.

## Security Features

- Password hashing with bcrypt
- JWT access tokens (15 min expiry)
- Refresh tokens (7 days expiry)
- Role-based access control
- SQL injection prevention
- CORS configuration
- Request validation

## Development

### Running Tests
```bash
make test
```

### Building Binary
```bash
make build
```

### Database Migrations

**Create new migration**:
```bash
make migrate-create NAME=add_new_feature
```

**Run migrations**:
```bash
make migrate-up
```

**Rollback**:
```bash
make migrate-down
```

## Creating First Admin User

After deployment, create an admin user manually in the database:

```sql
-- First, register a user through the API, then update their role:
UPDATE users SET role = 'admin' WHERE email = 'admin@caspianex.com';
```

Or use the registration API and then update:
```bash
# 1. Register user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@caspianex.com",
    "password": "securepassword",
    "first_name": "Admin",
    "last_name": "User"
  }'

# 2. Update role in database
psql -U exchange -d exchange_db -c \
  "UPDATE users SET role = 'admin' WHERE email = 'admin@caspianex.com';"
```

## Production Deployment Checklist

1. **Change `JWT_SECRET`** to a strong random value
2. **Configure SMTP** credentials
3. **Set company payment details** (wallets, bank info)
4. **Update exchange rates** regularly
5. Set `ENV=production`
6. Use strong database passwords
7. Configure SSL/TLS for PostgreSQL
8. Set up proper logging aggregation
9. Configure backups for PostgreSQL
10. Use secrets management (not .env files)
11. Set up monitoring and alerting
12. Configure rate limiting at API gateway level
13. Set up proper CORS origins

## Monitoring

- Structured logging with log levels
- HTTP request/response logging
- Health check endpoint for container orchestration
- Performance metrics (duration, status codes)

## License

MIT

## Support

For issues and questions, please open an issue on GitHub.
