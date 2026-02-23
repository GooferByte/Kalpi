// Package zerodha implements broker.Adapter for Zerodha's Kite Connect v3 API.
//
// Auth flow (3-step OAuth):
//  1. User visits https://kite.zerodha.com/connect/login?api_key=<key>
//  2. After login Zerodha redirects to redirect_uri?request_token=<tok>
//  3. We POST /session/token (api_key + request_token + sha256 checksum) → access_token
//
// Auth header for subsequent calls: "token <api_key>:<access_token>"
package zerodha

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const baseURL = "https://api.kite.trade"

// Adapter is the Zerodha implementation of broker.Adapter.
type Adapter struct {
	client *resty.Client
	logger *zap.Logger
}

// New creates a new Zerodha adapter.
func New(logger *zap.Logger) *Adapter {
	return &Adapter{
		logger: logger,
		client: resty.New().
			SetBaseURL(baseURL).
			SetTimeout(30 * time.Second).
			SetRetryCount(2).
			SetRetryWaitTime(1 * time.Second),
	}
}

func (a *Adapter) Name() string { return "zerodha" }

// authHeader returns the Zerodha-specific authorization header value.
func (a *Adapter) authHeader(sess *session.Session) string {
	return fmt.Sprintf("token %s:%s", sess.APIKey, sess.AccessToken)
}

// Authenticate exchanges api_key + request_token for an access_token.
// Credentials required: APIKey, APISecret, RequestToken.
func (a *Adapter) Authenticate(ctx context.Context, creds models.Credentials) (*models.AuthResponse, error) {
	checksum := fmt.Sprintf("%x",
		sha256.Sum256([]byte(creds.APIKey+creds.RequestToken+creds.APISecret)))

	var res struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			AccessToken string `json:"access_token"`
			UserID      string `json:"user_id"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetFormData(map[string]string{
			"api_key":       creds.APIKey,
			"request_token": creds.RequestToken,
			"checksum":      checksum,
		}).
		SetResult(&res).
		Post("/session/token")
	if err != nil {
		return nil, fmt.Errorf("zerodha authenticate: %w", err)
	}
	if resp.IsError() || res.Status != "success" {
		return nil, fmt.Errorf("zerodha authenticate: %s", res.Message)
	}
	return &models.AuthResponse{
		Broker:      a.Name(),
		AccessToken: res.Data.AccessToken,
		UserID:      res.Data.UserID,
		Message:     "authenticated successfully",
	}, nil
}

// GetHoldings fetches equity holdings for the authenticated user.
func (a *Adapter) GetHoldings(ctx context.Context, sess *session.Session) ([]models.Holding, error) {
	var res struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    []struct {
			TradingSymbol string  `json:"tradingsymbol"`
			Quantity      int     `json:"quantity"`
			AveragePrice  float64 `json:"average_price"`
			LastPrice     float64 `json:"last_price"`
			PnL           float64 `json:"pnl"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", a.authHeader(sess)).
		SetResult(&res).
		Get("/portfolio/holdings")
	if err != nil {
		return nil, fmt.Errorf("zerodha get_holdings: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("zerodha get_holdings: %s", res.Message)
	}

	out := make([]models.Holding, 0, len(res.Data))
	for _, h := range res.Data {
		out = append(out, models.Holding{
			Symbol:       h.TradingSymbol,
			Quantity:     h.Quantity,
			AvgPrice:     h.AveragePrice,
			CurrentPrice: h.LastPrice,
			PnL:          h.PnL,
		})
	}
	return out, nil
}

// PlaceOrder places a regular CNC order on NSE.
func (a *Adapter) PlaceOrder(ctx context.Context, sess *session.Session, order models.Order) (*models.OrderResult, error) {
	orderType := string(order.OrderType)
	if orderType == "" {
		orderType = "MARKET"
	}

	var res struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			OrderID string `json:"order_id"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", a.authHeader(sess)).
		SetFormData(map[string]string{
			"tradingsymbol":    order.Symbol,
			"exchange":         "NSE",
			"transaction_type": string(order.Side),
			"order_type":       orderType,
			"quantity":         fmt.Sprintf("%d", order.Quantity),
			"product":          "CNC",
			"validity":         "DAY",
			"price":            fmt.Sprintf("%.2f", order.Price),
		}).
		SetResult(&res).
		Post("/orders/regular")
	if err != nil {
		return failedResult(order, err.Error()), fmt.Errorf("zerodha place_order: %w", err)
	}
	if resp.IsError() || res.Status != "success" {
		return failedResult(order, res.Message), fmt.Errorf("zerodha place_order rejected: %s", res.Message)
	}

	return &models.OrderResult{
		OrderID:   res.Data.OrderID,
		Symbol:    order.Symbol,
		Quantity:  order.Quantity,
		Side:      order.Side,
		Status:    models.OrderStatusPending,
		Timestamp: time.Now(),
	}, nil
}

// GetOrderStatus retrieves the current state of an order by ID.
func (a *Adapter) GetOrderStatus(ctx context.Context, sess *session.Session, orderID string) (*models.OrderResult, error) {
	var res struct {
		Status string `json:"status"`
		Data   []struct {
			OrderID         string  `json:"order_id"`
			Status          string  `json:"status"`
			TradingSymbol   string  `json:"tradingsymbol"`
			Quantity        int     `json:"quantity"`
			TransactionType string  `json:"transaction_type"`
			AveragePrice    float64 `json:"average_price"`
			StatusMessage   string  `json:"status_message"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", a.authHeader(sess)).
		SetResult(&res).
		Get("/orders/" + orderID)
	if err != nil {
		return nil, fmt.Errorf("zerodha order_status: %w", err)
	}
	if resp.IsError() || len(res.Data) == 0 {
		return nil, fmt.Errorf("zerodha order_status: order not found %s", orderID)
	}

	d := res.Data[0]
	return &models.OrderResult{
		OrderID:   d.OrderID,
		Symbol:    d.TradingSymbol,
		Quantity:  d.Quantity,
		Side:      models.OrderSide(d.TransactionType),
		Status:    mapStatus(d.Status),
		Message:   d.StatusMessage,
		Price:     d.AveragePrice,
		Timestamp: time.Now(),
	}, nil
}

// CancelOrder cancels a regular open order.
func (a *Adapter) CancelOrder(ctx context.Context, sess *session.Session, orderID string) error {
	var res struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", a.authHeader(sess)).
		SetResult(&res).
		Delete("/orders/regular/" + orderID)
	if err != nil {
		return fmt.Errorf("zerodha cancel_order: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("zerodha cancel_order: %s", res.Message)
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func mapStatus(s string) models.OrderStatus {
	switch s {
	case "COMPLETE":
		return models.OrderStatusComplete
	case "REJECTED":
		return models.OrderStatusRejected
	case "CANCELLED":
		return models.OrderStatusCancelled
	case "OPEN", "TRIGGER PENDING":
		return models.OrderStatusOpen
	default:
		return models.OrderStatusPending
	}
}

func failedResult(o models.Order, msg string) *models.OrderResult {
	return &models.OrderResult{
		Symbol:    o.Symbol,
		Quantity:  o.Quantity,
		Side:      o.Side,
		Status:    models.OrderStatusFailed,
		Message:   msg,
		Timestamp: time.Now(),
	}
}
