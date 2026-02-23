# Kalpi Capital — Portfolio Trade Execution Engine

> **Role:** Backend/Infra Engineer (Kalpi Builder)
> **Company:** Kalpi Capital — India's First Systematic Quant Investing Platform

An end-to-end **Portfolio Trade Execution Engine** that takes a desired portfolio state, authenticates with a user's stock broker, and automatically executes the necessary trades in a single click.

---

## Table of Contents

1. [Why Go Instead of Python (FastAPI)](#why-go-instead-of-python-fastapi)
2. [Setup & Run Instructions](#setup--run-instructions)
3. [Architecture](#architecture)
4. [Broker Integration](#broker-integration)
5. [Execution Logic & Rebalance](#execution-logic--rebalance)
6. [Notification System](#notification-system)
7. [Frontend UI](#frontend-ui)
8. [API Reference](#api-reference)
9. [Third-Party Libraries Justification](#third-party-libraries-justification)
10. [Broker Credentials Reference](#broker-credentials-reference)

---

## Why Go Instead of Python (FastAPI)

The assignment recommends Python (FastAPI). Go was chosen instead because **trading infrastructure has hard real-time constraints** where Go's properties are a structural advantage — and this decision is fully justified below.

| Concern | Python (FastAPI) | Go |
|---|---|---|
| **Concurrency model** | `asyncio` / threads — cooperative, GIL-limited | Native goroutines — 2 KB stack, preemptive, true parallelism |
| **Parallel order placement** | Requires `asyncio.gather` or thread pools with overhead | `sync.WaitGroup` — trivially concurrent, zero overhead |
| **Latency** | Interpreted, ~50–100 ms cold-path overhead | Compiled binary, sub-millisecond execution paths |
| **Memory footprint** | 50–200 MB for a FastAPI process | ~10 MB for the entire binary + runtime |
| **Docker image size** | 300–500 MB (Python + deps) | **~8 MB** (distroless + static binary) |
| **Error handling** | Exceptions can be silently swallowed | Explicit `error` returns — every failure path must be handled |
| **Type safety** | Runtime type errors possible | Compile-time type safety — bugs caught before deployment |
| **Graceful shutdown** | Requires `uvicorn` SIGTERM wiring | `http.Server.Shutdown` built into stdlib |
| **Startup time** | 2–5 seconds (import overhead) | < 100 ms |

**Bottom line:** In a trading system, bugs = money loss. Go's explicit error handling, static typing, and compiled nature make it the safer choice for financial infrastructure. Every failed order is captured, retried, and reported — not swallowed by an unhandled exception.

---

## Setup & Run Instructions

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- Go 1.22+ *(only for local development without Docker)*
- Node.js 20+ *(only for running the frontend locally without Docker)*

---

### Option A — Docker (Recommended)

Runs both the API and the frontend in containers with a single command.

```bash
# 1. Clone the repo
git clone https://github.com/GooferByte/kalpi.git
cd kalpi

# 2. Copy the environment file
cp .env.example .env

# 3. Build and start all services
docker compose up --build
```

| Service | URL |
|---------|-----|
| Frontend UI | http://localhost:3000 |
| Backend API | http://localhost:8080 |
| Health check | http://localhost:8080/health |

To stop:
```bash
docker compose down
```

---

### Option B — Local Development

**Backend (Go):**
```bash
# Install dependencies
go mod tidy

# Copy env file
cp .env.example .env

# Start the API server
go run ./cmd/server
# → API running at http://localhost:8080
```

**Frontend (Next.js):**
```bash
cd frontend

# Install dependencies
npm install

# Copy env file
cp .env.local.example .env.local

# Start the dev server
npm run dev
# → UI running at http://localhost:3000
```

---

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | API server port |
| `ENV` | `development` | `development` or `production` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SESSION_TTL_HOURS` | `24` | How long broker sessions stay valid |
| `MOCK_MODE` | `false` | Set `true` to force mock broker |

---

## Architecture

```
kalpi/
├── cmd/server/main.go              ← Entry point — wires all dependencies, graceful shutdown
├── pkg/config/config.go            ← Viper config loader (.env + env vars)
├── internal/
│   ├── models/models.go            ← All shared types: Order, Holding, ExecutionResult…
│   ├── session/manager.go          ← Session store interface + in-memory implementation
│   ├── broker/
│   │   ├── adapter.go              ← BrokerAdapter interface — the core abstraction
│   │   ├── factory.go              ← NewAdapter(brokerName) factory function
│   │   ├── zerodha/zerodha.go      ← Zerodha Kite Connect v3 adapter
│   │   ├── fyers/fyers.go          ← Fyers API v3 adapter
│   │   ├── angelone/angelone.go    ← AngelOne SmartAPI adapter
│   │   ├── upstox/upstox.go        ← Upstox API v2 adapter
│   │   ├── groww/groww.go          ← Groww REST API adapter
│   │   └── mock/mock.go            ← Mock broker for testing (no real credentials needed)
│   ├── engine/
│   │   ├── store.go                ← ExecutionStore interface + in-memory implementation
│   │   ├── order_manager.go        ← Concurrent order placement + exponential-backoff retry
│   │   └── executor.go             ← Core orchestration: session → adapter → orders → notify
│   ├── notification/
│   │   ├── notifier.go             ← Notifier interface
│   │   ├── log.go                  ← Structured log notifier (always active)
│   │   ├── webhook.go              ← HTTP POST notifier
│   │   ├── websocket.go            ← gorilla/websocket hub + WebSocket notifier
│   │   └── composite.go            ← Chains all notifiers together
│   └── api/
│       ├── router.go               ← Gin engine + all route registrations
│       ├── middleware/             ← Request logger + panic recovery
│       └── handlers/               ← Auth, Portfolio, Orders, WebSocket handlers
├── frontend/                       ← Next.js 14 + Tailwind CSS frontend
├── Dockerfile                      ← Multi-stage distroless build (~8 MB image)
└── docker-compose.yml              ← Orchestrates API + frontend services
```

### Key Design Patterns

**Adapter Pattern** — `broker.Adapter` is a Go interface. The execution engine never imports any broker SDK directly. Every broker is a separate package that satisfies the same interface. Adding broker #6 requires:
1. Create `internal/broker/<name>/<name>.go` implementing the interface
2. Add one `case` in `factory.go`
3. Zero other changes

**Interface-driven design** — Session store, execution store, order manager, notifier, and executor are all interfaces. Any implementation can be swapped (e.g. Redis session store, DB-backed execution store) without touching dependent code.

**Dependency injection** — `main.go` constructs every component and passes it down. No package-level globals. Every component is independently testable.

**Composite Notifier** — Log + WebSocket + optional Webhook are chained. All fire after every execution regardless of whether one fails.

---

## Broker Integration

The system supports **5 major Indian brokers** plus a **mock broker** for testing:

| Broker | API | Auth Method |
|---|---|---|
| Zerodha | Kite Connect v3 | OAuth — api_key + request_token + sha256 checksum |
| Fyers | Fyers API v3 | OAuth — app_id + auth_code |
| AngelOne | SmartAPI | TOTP-based login — client_code + password + TOTP |
| Upstox | Upstox API v2 | OAuth2 — client_id + auth_code |
| Groww | REST API | API Key |
| Mock | Built-in | Any string (for local testing) |

All brokers implement the same `BrokerAdapter` interface:

```go
type Adapter interface {
    Authenticate(ctx, credentials) → (*AuthResponse, error)
    GetHoldings(ctx, session)      → ([]Holding, error)
    PlaceOrder(ctx, session, order) → (*OrderResult, error)
    GetOrderStatus(ctx, session, orderID) → (*OrderResult, error)
    CancelOrder(ctx, session, orderID) → error
    Name() string
}
```

---

## Execution Logic & Rebalance

### First-Time Portfolio (`mode: "first_time"`)

Used when the user has **no existing holdings**. All stocks in the `buy` list are purchased concurrently.

```json
{
  "broker": "mock",
  "mode": "first_time",
  "session_id": "<session_id>",
  "orders": {
    "buy": [
      { "symbol": "RELIANCE", "qty": 10 },
      { "symbol": "TCS",      "qty": 5  },
      { "symbol": "HDFC",     "qty": 8  }
    ]
  }
}
```

All 3 BUY orders are placed **concurrently** using goroutines — not sequentially.

---

### Portfolio Rebalance (`mode: "rebalance"`)

Used when the user has **existing holdings** that need to be adjusted. The input payload provides **explicit instructions** — the engine does not need to compute the delta itself.

```json
{
  "broker": "mock",
  "mode": "rebalance",
  "session_id": "<session_id>",
  "webhook_url": "https://webhook.site/your-id",
  "orders": {
    "sell":      [{ "symbol": "HDFC",     "qty": 3         }],
    "buy":       [{ "symbol": "INFY",     "qty": 7         }],
    "rebalance": [{ "symbol": "RELIANCE", "qty_change": -2 }]
  }
}
```

**Three instruction types:**

| Field | Meaning |
|---|---|
| `sell` | Exit specific quantities of existing stocks |
| `buy` | Purchase specific quantities of new stocks |
| `rebalance` | Adjust existing stocks — `qty_change < 0` = sell, `qty_change > 0` = buy more |

**Execution order (enforced for capital safety):**

```
Step 1 → All SELL orders + rebalance[qty_change < 0]   (concurrent)
              ↓ frees up capital
Step 2 → All BUY orders  + rebalance[qty_change > 0]   (concurrent)
```

Sells always run before buys so that capital freed by exits is available for new purchases.

---

### Order Reliability

- Each order in a batch runs in its own goroutine — a single slow order does not block others
- Failed orders are retried up to **3 times** with **exponential backoff** (1 s → 2 s → 4 s)
- Transient errors (rate limits, network timeouts) are retried; permanent errors (invalid symbol, insufficient funds) are not
- Every order outcome — success or failure — is captured in `ExecutionResult` and reported

---

## Notification System

After every execution (first-time or rebalance), the engine fires the `CompositeNotifier` which calls all three channels:

### 1. Log Notifier (always active)
Every executed and failed order is logged to stdout via `zap` structured logger:
```
INFO  execution notification  execution_id=abc  success=3  failed=0
INFO    ✓ order  order_id=MOCK-123  symbol=RELIANCE  side=BUY  qty=10  status=COMPLETE
WARN    ✗ order failed  symbol=TCS  message=insufficient funds
```

### 2. WebSocket Notifier (real-time)
Connect to `ws://localhost:8080/ws/notifications` to receive live `ExecutionResult` JSON pushes as soon as an execution completes.

```bash
# Using wscat
wscat -c ws://localhost:8080/ws/notifications
```

### 3. Webhook Notifier (HTTP callback)
If `webhook_url` is provided in the request payload, the engine POSTs the full `ExecutionResult` to that URL with header `X-Kalpi-Event: execution.completed`.

Test with [webhook.site](https://webhook.site) for a free inspection URL.

**Notification payload:**
```json
{
  "execution_id": "uuid",
  "broker": "zerodha",
  "mode": "rebalance",
  "status": "completed",
  "total_orders": 3,
  "success_count": 2,
  "failure_count": 1,
  "successful_orders": [...],
  "failed_orders": [...],
  "timestamp": "2026-02-23T10:00:00Z",
  "completed_at": "2026-02-23T10:00:01Z"
}
```

---

## Frontend UI

A React/Next.js frontend provides a visual 4-step wizard to test the full flow.

**Start the UI:**
```bash
cd frontend
npm install && npm run dev
# → http://localhost:3000
```

### Step 1 — Connect Broker
Select a broker and fill in credentials. Each broker shows different fields automatically. Use **Mock** to test without real credentials.

### Step 2 — Build Portfolio
- Toggle between **First-Time** and **Rebalance** mode
- Add BUY / SELL / ADJUST rows manually or **upload a CSV** (`symbol,qty` format)
- Rebalance mode shows all three instruction sections

### Step 3 — Execute
Review the full order summary, optionally set a webhook URL, and click **Execute**.

### Step 4 — Results
- Stats: total / success / failed order counts
- Color-coded order table with symbol, side, quantity, status, order ID
- **Live WebSocket feed** — shows real-time pushes from any concurrent executions

---

## API Reference

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/ws/notifications` | WebSocket — real-time execution events |
| `POST` | `/api/v1/auth/:broker` | Authenticate with a broker |
| `DELETE` | `/api/v1/auth/session/:id` | Logout / invalidate session |
| `GET` | `/api/v1/brokers` | List supported brokers |
| `GET` | `/api/v1/holdings?session_id=` | Fetch current holdings |
| `POST` | `/api/v1/portfolio/execute` | First-time portfolio execution |
| `POST` | `/api/v1/portfolio/rebalance` | Rebalance existing portfolio |
| `GET` | `/api/v1/orders` | List all execution results |
| `GET` | `/api/v1/orders/:exec_id` | Get single execution result |

### Quick Test (Mock Broker)

```bash
# 1. Authenticate
curl -X POST http://localhost:8080/api/v1/auth/mock \
  -H "Content-Type: application/json" \
  -d '{"credentials": {"api_key": "test"}}'
# → copy session_id

# 2. First-time portfolio
curl -X POST http://localhost:8080/api/v1/portfolio/execute \
  -H "Content-Type: application/json" \
  -d '{
    "broker": "mock",
    "mode": "first_time",
    "session_id": "<session_id>",
    "orders": {
      "buy": [
        {"symbol": "RELIANCE", "qty": 10},
        {"symbol": "TCS",      "qty": 5}
      ]
    }
  }'

# 3. Rebalance
curl -X POST http://localhost:8080/api/v1/portfolio/rebalance \
  -H "Content-Type: application/json" \
  -d '{
    "broker": "mock",
    "mode": "rebalance",
    "session_id": "<session_id>",
    "orders": {
      "sell":      [{"symbol": "TCS",      "qty": 2}],
      "buy":       [{"symbol": "INFY",     "qty": 7}],
      "rebalance": [{"symbol": "RELIANCE", "qty_change": -3}]
    }
  }'

# 4. Check result
curl http://localhost:8080/api/v1/orders/<execution_id>
```

---

## Third-Party Libraries Justification

### Backend (Go)

| Library | Purpose | Justification |
|---|---|---|
| `gin-gonic/gin` | HTTP framework | Industry-standard Go web framework. Fast radix-tree router, middleware chain, JSON binding, and validator integration. Chosen over stdlib `net/http` for its ergonomics while adding negligible overhead. |
| `gorilla/websocket` | WebSocket server | The de-facto Go WebSocket library. RFC 6455 compliant, battle-tested in production at scale. Used for the real-time notification hub. |
| `go-resty/resty/v2` | HTTP client for broker API calls | Provides retry logic, timeouts, and a fluent builder pattern. Eliminates boilerplate `net/http` code that would otherwise be duplicated across 5 broker adapters. |
| `spf13/viper` | Configuration | Reads `.env` files and OS environment variables with a single unified interface. Standard choice in Go microservices. |
| `go-playground/validator/v10` | Request validation | Struct-tag-based validation — the Go equivalent of Python's Pydantic. Validates incoming JSON payloads at the HTTP boundary. |
| `uber-go/zap` | Structured logging | 10–100× faster than Go's `log/slog` for high-throughput services. JSON-structured logs are essential for production observability in trading systems. |
| `google/uuid` | UUID generation | RFC 4122 compliant. Used for session IDs and execution IDs. |
| `golang.org/x/sync` | `errgroup` | Propagates context cancellation and collects errors across goroutine groups — used in the order batch placement logic. |

**No unified broker SDK used.** Each broker's official REST API is called directly via `resty`. This gives full control over request/response mapping and avoids hidden abstractions or version-lock in a financial context.

### Frontend (Next.js)

| Library | Purpose | Justification |
|---|---|---|
| `next` (v14) | React framework | App Router, TypeScript support, built-in optimisations. `output: standalone` enables minimal Docker images. |
| `tailwindcss` | Styling | Utility-first CSS — zero runtime, small production bundle, no CSS-in-JS overhead. |
| `lucide-react` | Icons | Lightweight, tree-shakeable icon set. Only the icons actually used are included in the bundle. |

---

## Broker Credentials Reference

| Broker | Required Fields |
|---|---|
| Zerodha | `api_key`, `api_secret`, `request_token` |
| Fyers | `app_id`, `api_secret`, `auth_code` |
| AngelOne | `api_key`, `client_code`, `password`, `totp` |
| Upstox | `api_key` (client_id), `api_secret` (client_secret), `auth_code`, `redirect_uri` |
| Groww | `api_key` |
| **Mock** | `api_key` (any string — for testing) |

> **Note:** Broker OAuth flows (Zerodha, Fyers, Upstox) require the user to visit the broker's login page first to obtain a `request_token` / `auth_code`. The engine handles the token exchange step.
