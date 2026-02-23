package models

import "time"

// ─── Order primitives ────────────────────────────────────────────────────────

// OrderType is the type of a trading order.
type OrderType string

const (
	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeSL     OrderType = "SL"
	OrderTypeSLM    OrderType = "SLM"
)

// OrderSide is the direction of a trade.
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderStatus is the lifecycle state of an order.
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "PENDING"
	OrderStatusOpen      OrderStatus = "OPEN"
	OrderStatusComplete  OrderStatus = "COMPLETE"
	OrderStatusRejected  OrderStatus = "REJECTED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
	OrderStatusFailed    OrderStatus = "FAILED"
)

// ExecutionMode is the mode of portfolio execution.
type ExecutionMode string

const (
	ExecutionModeFirstTime ExecutionMode = "first_time"
	ExecutionModeRebalance ExecutionMode = "rebalance"
)

// ─── Broker auth ─────────────────────────────────────────────────────────────

// Credentials holds broker-specific authentication data supplied by the client.
// Different brokers use different subsets of these fields.
type Credentials struct {
	APIKey       string `json:"api_key"`
	APISecret    string `json:"api_secret,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	Password     string `json:"password,omitempty"`
	TOTP         string `json:"totp,omitempty"`
	RequestToken string `json:"request_token,omitempty"` // Zerodha OAuth step 2
	ClientCode   string `json:"client_code,omitempty"`   // AngelOne
	AppID        string `json:"app_id,omitempty"`        // Fyers
	AuthCode     string `json:"auth_code,omitempty"`     // Fyers / Upstox OAuth code
	RedirectURI  string `json:"redirect_uri,omitempty"`  // OAuth brokers
}

// AuthRequest is the request body for the broker authentication endpoint.
type AuthRequest struct {
	Credentials Credentials `json:"credentials" validate:"required"`
}

// AuthResponse is returned on successful broker authentication.
type AuthResponse struct {
	SessionID   string `json:"session_id"`
	Broker      string `json:"broker"`
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id,omitempty"`
	Message     string `json:"message"`
}

// ─── Holdings & Orders ───────────────────────────────────────────────────────

// Holding represents a current stock position in a portfolio.
type Holding struct {
	Symbol       string  `json:"symbol"`
	Quantity     int     `json:"quantity"`
	AvgPrice     float64 `json:"avg_price"`
	CurrentPrice float64 `json:"current_price,omitempty"`
	PnL          float64 `json:"pnl,omitempty"`
}

// Order is a normalised trade order passed to broker adapters.
type Order struct {
	Symbol       string    `json:"symbol"    validate:"required"`
	Quantity     int       `json:"quantity"  validate:"required,gt=0"`
	Side         OrderSide `json:"side"      validate:"required"`
	OrderType    OrderType `json:"order_type"`
	Price        float64   `json:"price,omitempty"`
	TriggerPrice float64   `json:"trigger_price,omitempty"`
}

// OrderResult is the result of a single placed order.
type OrderResult struct {
	OrderID   string      `json:"order_id"`
	Symbol    string      `json:"symbol"`
	Quantity  int         `json:"quantity"`
	Side      OrderSide   `json:"side"`
	Status    OrderStatus `json:"status"`
	Message   string      `json:"message,omitempty"`
	Price     float64     `json:"price,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ─── Execution payload ───────────────────────────────────────────────────────

// TradeInstruction is a single item within an execution payload.
type TradeInstruction struct {
	Symbol    string    `json:"symbol"     validate:"required"`
	Qty       int       `json:"qty,omitempty"`
	QtyChange int       `json:"qty_change,omitempty"` // used in REBALANCE: negative = sell, positive = buy
	OrderType OrderType `json:"order_type,omitempty"`
	Price     float64   `json:"price,omitempty"`
}

// OrderPayload contains the three categories of trade instructions.
type OrderPayload struct {
	Buy       []TradeInstruction `json:"buy,omitempty"`
	Sell      []TradeInstruction `json:"sell,omitempty"`
	Rebalance []TradeInstruction `json:"rebalance,omitempty"`
}

// ExecutionRequest is the top-level input for any portfolio execution call.
type ExecutionRequest struct {
	Broker     string        `json:"broker"     validate:"required"`
	Mode       ExecutionMode `json:"mode"       validate:"required,oneof=first_time rebalance"`
	Orders     OrderPayload  `json:"orders"`
	WebhookURL string        `json:"webhook_url,omitempty"`
	SessionID  string        `json:"session_id" validate:"required"`
}

// ExecutionResult is the complete outcome of a portfolio execution run.
type ExecutionResult struct {
	ExecutionID      string        `json:"execution_id"`
	Broker           string        `json:"broker"`
	Mode             ExecutionMode `json:"mode"`
	Status           string        `json:"status"`
	SuccessfulOrders []OrderResult `json:"successful_orders"`
	FailedOrders     []OrderResult `json:"failed_orders"`
	TotalOrders      int           `json:"total_orders"`
	SuccessCount     int           `json:"success_count"`
	FailureCount     int           `json:"failure_count"`
	Timestamp        time.Time     `json:"timestamp"`
	CompletedAt      *time.Time    `json:"completed_at,omitempty"`
}

// ─── HTTP envelope ───────────────────────────────────────────────────────────

// APIResponse is the standard JSON response wrapper for all HTTP endpoints.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
