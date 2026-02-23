# Kalpi Trade Execution Engine — Project Agent

## Project Overview
End-to-end **Portfolio Trade Execution Engine** built in Go for Kalpi Capital.
Authenticates with Indian stock brokers and executes portfolio trades (first-time buy or rebalance) in a single API call.

**Module:** `github.com/GooferByte/kalpi`
**Go version:** 1.22
**Framework:** Gin (HTTP), gorilla/websocket (WS), resty (broker HTTP client), zap (logging)

---

## Project Structure

```
cmd/server/main.go                         ← Entry point, dependency wiring, graceful shutdown
pkg/config/config.go                       ← Viper config (.env + env vars)
internal/
  models/models.go                         ← All shared types (Order, Holding, ExecutionResult…)
  session/manager.go                       ← Session interface + InMemoryManager
  broker/
    adapter.go                             ← BrokerAdapter interface (DO NOT change signatures without updating all 6 impls)
    factory.go                             ← NewAdapter(brokerName) — register new brokers here
    zerodha/zerodha.go                     ← Zerodha Kite Connect v3
    fyers/fyers.go                         ← Fyers API v3
    angelone/angelone.go                   ← AngelOne SmartAPI
    upstox/upstox.go                       ← Upstox API v2
    groww/groww.go                         ← Groww (unofficial REST structure)
    mock/mock.go                           ← Zero-credential broker for local testing
  engine/
    store.go                               ← ExecutionStore interface + InMemoryStore
    order_manager.go                       ← OrderManager interface — concurrent placement + retry
    executor.go                            ← Executor interface — core orchestration logic
  notification/
    notifier.go                            ← Notifier interface
    log.go                                 ← Structured log notifier (always on)
    webhook.go                             ← HTTP POST notifier
    websocket.go                           ← gorilla Hub + WebSocketNotifier
    composite.go                           ← Chains multiple Notifiers
  api/
    router.go                              ← Gin engine + all route registrations
    middleware/logger.go                   ← Structured request logger
    middleware/recovery.go                 ← Panic recovery → 500
    handlers/auth.go                       ← POST /auth/:broker, DELETE /auth/session/:id, GET /brokers
    handlers/portfolio.go                  ← POST /portfolio/execute, POST /portfolio/rebalance, GET /holdings
    handlers/orders.go                     ← GET /orders, GET /orders/:exec_id
    handlers/ws.go                         ← GET /ws/notifications (WebSocket upgrade)
Dockerfile                                 ← Multi-stage distroless build (~8 MB image)
docker-compose.yml
.env.example
README.md
```

---

## How to Run

```bash
# Local
go run ./cmd/server

# Docker
docker compose up --build
```

Server starts at `http://localhost:8080`

---

## How to Test (Quick)

```bash
# 1. Authenticate with mock broker (no real credentials needed)
curl -X POST http://localhost:8080/api/v1/auth/mock \
  -H "Content-Type: application/json" \
  -d '{"credentials": {"api_key": "test"}}'
# → copy session_id from response

# 2. First-time portfolio
curl -X POST http://localhost:8080/api/v1/portfolio/execute \
  -H "Content-Type: application/json" \
  -d '{"broker":"mock","mode":"first_time","session_id":"<id>","orders":{"buy":[{"symbol":"RELIANCE","qty":10}]}}'

# 3. Rebalance
curl -X POST http://localhost:8080/api/v1/portfolio/rebalance \
  -H "Content-Type: application/json" \
  -d '{"broker":"mock","mode":"rebalance","session_id":"<id>","orders":{"sell":[{"symbol":"TCS","qty":2}],"buy":[{"symbol":"HDFC","qty":5}],"rebalance":[{"symbol":"RELIANCE","qty_change":-3}]}}'

# 4. Check result
curl http://localhost:8080/api/v1/orders/<execution_id>
```

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ws/notifications` | WebSocket — real-time execution events |
| POST | `/api/v1/auth/:broker` | Authenticate with a broker |
| DELETE | `/api/v1/auth/session/:id` | Logout / invalidate session |
| GET | `/api/v1/brokers` | List supported brokers |
| GET | `/api/v1/holdings` | Get current holdings (`?session_id=`) |
| POST | `/api/v1/portfolio/execute` | First-time portfolio execution |
| POST | `/api/v1/portfolio/rebalance` | Rebalance existing portfolio |
| GET | `/api/v1/orders` | List all execution results |
| GET | `/api/v1/orders/:exec_id` | Get single execution result |

---

## Core Interfaces (Never Break These)

### `broker.Adapter` — `internal/broker/adapter.go`
```go
type Adapter interface {
    Authenticate(ctx, creds)  → (*AuthResponse, error)
    GetHoldings(ctx, sess)    → ([]Holding, error)
    PlaceOrder(ctx, sess, order) → (*OrderResult, error)
    GetOrderStatus(ctx, sess, orderID) → (*OrderResult, error)
    CancelOrder(ctx, sess, orderID) → error
    Name() string
}
```

### `engine.Executor` — `internal/engine/executor.go`
```go
type Executor interface {
    Execute(ctx, req) → (*ExecutionResult, error)
    GetExecution(id)  → (*ExecutionResult, bool)
    ListExecutions()  → []*ExecutionResult
}
```

### `engine.OrderManager` — `internal/engine/order_manager.go`
```go
type OrderManager interface {
    PlaceBatch(ctx, adapter, sess, orders) → []OrderResult
    PlaceWithRetry(ctx, adapter, sess, order) → (*OrderResult, error)
}
```

### `notification.Notifier` — `internal/notification/notifier.go`
```go
type Notifier interface {
    Notify(ctx, result) → error
}
```

### `session.Manager` — `internal/session/manager.go`
```go
type Manager interface {
    Create(broker, accessToken, apiKey, userID) → *Session
    Get(id) → (*Session, bool)
    Delete(id)
}
```

### `engine.ExecutionStore` — `internal/engine/store.go`
```go
type ExecutionStore interface {
    Save(result)
    Get(id)   → (*ExecutionResult, bool)
    List()    → []*ExecutionResult
}
```

---

## Adding a New Broker (Broker #6+)

1. Create `internal/broker/<name>/<name>.go`
2. Implement all methods of `broker.Adapter`
3. Add one case in `internal/broker/factory.go`:
   ```go
   case "<name>":
       return <name>.New(logger), nil
   ```
4. Add `"<name>"` to `SupportedBrokers()` slice in `factory.go`
5. No other files need to change

---

## Execution Logic

**First-time mode** (`mode: "first_time"`):
- All `orders.buy` → BUY orders, placed concurrently

**Rebalance mode** (`mode: "rebalance"`):
1. `orders.sell` + `rebalance[qty_change < 0]` → SELL concurrently (frees capital)
2. `orders.buy` + `rebalance[qty_change > 0]` → BUY concurrently (after sells)

`qty_change` in rebalance:
- Negative → sell that many units (e.g. `-3` = sell 3)
- Positive → buy that many units (e.g. `+5` = buy 5)

---

## Notification Flow

After every execution, `CompositeNotifier` fires all three:
1. **LogNotifier** — always logs to zap stdout
2. **WebSocketNotifier** — broadcasts JSON to all `/ws/notifications` clients
3. **WebhookNotifier** — if `webhook_url` provided in request, POSTs result

---

## Broker Credentials Reference

| Broker | Fields |
|--------|--------|
| zerodha | `api_key`, `api_secret`, `request_token` |
| fyers | `app_id`, `api_secret`, `auth_code` |
| angelone | `api_key`, `client_code`, `password`, `totp` |
| upstox | `api_key`, `api_secret`, `auth_code`, `redirect_uri` |
| groww | `api_key` |
| mock | `api_key` (any string) |

---

## Development Conventions

- **All interfaces** have their own file (`adapter.go`, `notifier.go`, etc.)
- **Concrete implementations** go in sub-packages to avoid circular imports
- **Errors** are always wrapped: `fmt.Errorf("context: %w", err)`
- **No globals** — everything injected via constructor args
- **Context** (`context.Context`) is the first arg on all methods that call external services
- **Logging** uses `zap.Logger` (structured, not `fmt.Println`)
- **HTTP responses** always use `models.APIResponse{Success: bool, Data: ..., Error: ...}`
- Use `go vet ./...` and `go build ./...` before committing — both must pass clean

---

## Common Commands

```bash
go build ./...          # Verify compilation
go vet ./...            # Static analysis
go mod tidy             # Sync dependencies
go run ./cmd/server     # Start server
docker compose up --build  # Start via Docker
```
