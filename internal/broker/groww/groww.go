// Package groww implements broker.Adapter for Groww's trading API.
//
// Note: Groww does not publish an official REST trading API at the time of writing.
// This adapter follows the same interface contract and uses a plausible REST structure
// consistent with Groww's mobile app traffic patterns.
// Auth header: "Authorization: <api_key>"
package groww

import (
	"context"
	"fmt"
	"time"

	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const baseURL = "https://growwapi.groww.in/v1"

// Adapter is the Groww implementation of broker.Adapter.
type Adapter struct {
	client *resty.Client
	logger *zap.Logger
}

// New creates a new Groww adapter.
func New(logger *zap.Logger) *Adapter {
	return &Adapter{
		logger: logger,
		client: resty.New().
			SetBaseURL(baseURL).
			SetTimeout(30 * time.Second).
			SetRetryCount(2).
			SetHeader("Content-Type", "application/json"),
	}
}

func (a *Adapter) Name() string { return "groww" }

// Authenticate validates the Groww API key and returns a session token.
// Credentials required: APIKey.
func (a *Adapter) Authenticate(ctx context.Context, creds models.Credentials) (*models.AuthResponse, error) {
	var res struct {
		Status  string `json:"status"`
		Token   string `json:"token"`
		UserID  string `json:"user_id"`
		Message string `json:"message"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetBody(map[string]string{
			"api_key": creds.APIKey,
		}).
		SetResult(&res).
		Post("/auth/token")
	if err != nil {
		return nil, fmt.Errorf("groww authenticate: %w", err)
	}
	if resp.IsError() || res.Token == "" {
		return nil, fmt.Errorf("groww authenticate: %s", res.Message)
	}
	return &models.AuthResponse{
		Broker:      a.Name(),
		AccessToken: res.Token,
		UserID:      res.UserID,
		Message:     "authenticated successfully",
	}, nil
}

// GetHoldings fetches the user's current equity portfolio.
func (a *Adapter) GetHoldings(ctx context.Context, sess *session.Session) ([]models.Holding, error) {
	var res struct {
		Status   string `json:"status"`
		Message  string `json:"message"`
		Holdings []struct {
			Symbol       string  `json:"symbol"`
			Quantity     int     `json:"quantity"`
			AveragePrice float64 `json:"average_price"`
			CurrentPrice float64 `json:"current_price"`
			PnL          float64 `json:"pnl"`
		} `json:"holdings"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", sess.AccessToken).
		SetResult(&res).
		Get("/portfolio")
	if err != nil {
		return nil, fmt.Errorf("groww get_holdings: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("groww get_holdings: %s", res.Message)
	}

	out := make([]models.Holding, 0, len(res.Holdings))
	for _, h := range res.Holdings {
		out = append(out, models.Holding{
			Symbol:       h.Symbol,
			Quantity:     h.Quantity,
			AvgPrice:     h.AveragePrice,
			CurrentPrice: h.CurrentPrice,
			PnL:          h.PnL,
		})
	}
	return out, nil
}

// PlaceOrder places a trade order via Groww.
func (a *Adapter) PlaceOrder(ctx context.Context, sess *session.Session, order models.Order) (*models.OrderResult, error) {
	orderType := string(order.OrderType)
	if orderType == "" {
		orderType = "MARKET"
	}

	var res struct {
		Status  string `json:"status"`
		OrderID string `json:"order_id"`
		Message string `json:"message"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", sess.AccessToken).
		SetBody(map[string]interface{}{
			"symbol":           order.Symbol,
			"transaction_type": string(order.Side),
			"order_type":       orderType,
			"quantity":         order.Quantity,
			"price":            order.Price,
			"exchange":         "NSE",
			"product":          "CNC",
			"validity":         "DAY",
		}).
		SetResult(&res).
		Post("/order")
	if err != nil {
		return failedResult(order, err.Error()), fmt.Errorf("groww place_order: %w", err)
	}
	if resp.IsError() || res.OrderID == "" {
		return failedResult(order, res.Message), fmt.Errorf("groww place_order: %s", res.Message)
	}

	return &models.OrderResult{
		OrderID:   res.OrderID,
		Symbol:    order.Symbol,
		Quantity:  order.Quantity,
		Side:      order.Side,
		Status:    models.OrderStatusPending,
		Timestamp: time.Now(),
	}, nil
}

// GetOrderStatus retrieves the state of a Groww order.
func (a *Adapter) GetOrderStatus(ctx context.Context, sess *session.Session, orderID string) (*models.OrderResult, error) {
	var res struct {
		Status      string  `json:"status"`
		OrderID     string  `json:"order_id"`
		Symbol      string  `json:"symbol"`
		Quantity    int     `json:"quantity"`
		Side        string  `json:"transaction_type"`
		OrderStatus string  `json:"order_status"`
		AvgPrice    float64 `json:"average_price"`
		Message     string  `json:"message"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", sess.AccessToken).
		SetResult(&res).
		Get("/order/" + orderID)
	if err != nil {
		return nil, fmt.Errorf("groww order_status: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("groww order_status: %s", res.Message)
	}

	return &models.OrderResult{
		OrderID:   res.OrderID,
		Symbol:    res.Symbol,
		Quantity:  res.Quantity,
		Side:      models.OrderSide(res.Side),
		Status:    mapStatus(res.OrderStatus),
		Price:     res.AvgPrice,
		Message:   res.Message,
		Timestamp: time.Now(),
	}, nil
}

// CancelOrder cancels an open Groww order.
func (a *Adapter) CancelOrder(ctx context.Context, sess *session.Session, orderID string) error {
	var res struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", sess.AccessToken).
		SetResult(&res).
		Delete("/order/" + orderID)
	if err != nil {
		return fmt.Errorf("groww cancel_order: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("groww cancel_order: %s", res.Message)
	}
	return nil
}

func mapStatus(s string) models.OrderStatus {
	switch s {
	case "COMPLETE", "TRADED":
		return models.OrderStatusComplete
	case "REJECTED":
		return models.OrderStatusRejected
	case "CANCELLED":
		return models.OrderStatusCancelled
	case "OPEN":
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
