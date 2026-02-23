// Package fyers implements broker.Adapter for Fyers API v3.
//
// Auth flow:
//  1. User visits Fyers auth page → authorises app → receives auth_code.
//  2. We POST /token with appIdHash (sha256(app_id:app_secret)) + auth_code → access_token.
//
// Auth header for subsequent calls: "Authorization: <access_token>"
package fyers

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

const baseURL = "https://api-t1.fyers.in/api/v3"

// Adapter is the Fyers implementation of broker.Adapter.
type Adapter struct {
	client *resty.Client
	logger *zap.Logger
}

// New creates a new Fyers adapter.
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

func (a *Adapter) Name() string { return "fyers" }

// Authenticate exchanges an auth_code for a Fyers access_token.
// Credentials required: AppID (app_id), APISecret (app_secret), AuthCode.
func (a *Adapter) Authenticate(ctx context.Context, creds models.Credentials) (*models.AuthResponse, error) {
	appIDHash := fmt.Sprintf("%x",
		sha256.Sum256([]byte(creds.AppID+":"+creds.APISecret)))

	var res struct {
		Code        int    `json:"code"`
		Message     string `json:"message"`
		AccessToken string `json:"access_token"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetBody(map[string]string{
			"grant_type":  "authorization_code",
			"appIdHash":   appIDHash,
			"code":        creds.AuthCode,
		}).
		SetResult(&res).
		Post("/token")
	if err != nil {
		return nil, fmt.Errorf("fyers authenticate: %w", err)
	}
	if resp.IsError() || res.AccessToken == "" {
		return nil, fmt.Errorf("fyers authenticate: %s", res.Message)
	}
	return &models.AuthResponse{
		Broker:      a.Name(),
		AccessToken: res.AccessToken,
		Message:     "authenticated successfully",
	}, nil
}

// GetHoldings returns the user's equity holdings.
func (a *Adapter) GetHoldings(ctx context.Context, sess *session.Session) ([]models.Holding, error) {
	var res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Holdings []struct {
				Symbol       string  `json:"symbol"`
				HoldingType  string  `json:"holdingType"`
				Qty          int     `json:"qty"`
				BuyAvg       float64 `json:"buyAvg"`
				CurrentValue float64 `json:"currentValue"`
			} `json:"holdings"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", sess.AccessToken).
		SetResult(&res).
		Get("/holdings")
	if err != nil {
		return nil, fmt.Errorf("fyers get_holdings: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("fyers get_holdings: %s", res.Message)
	}

	out := make([]models.Holding, 0, len(res.Data.Holdings))
	for _, h := range res.Data.Holdings {
		out = append(out, models.Holding{
			Symbol:       h.Symbol,
			Quantity:     h.Qty,
			AvgPrice:     h.BuyAvg,
			CurrentPrice: h.CurrentValue / float64(max(h.Qty, 1)),
		})
	}
	return out, nil
}

// PlaceOrder places a market/limit order via Fyers.
func (a *Adapter) PlaceOrder(ctx context.Context, sess *session.Session, order models.Order) (*models.OrderResult, error) {
	orderType := 2 // MARKET
	if order.OrderType == models.OrderTypeLimit {
		orderType = 1
	}
	side := 1 // BUY
	if order.Side == models.OrderSideSell {
		side = -1
	}

	var res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		ID      string `json:"id"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", sess.AccessToken).
		SetBody(map[string]interface{}{
			"symbol":       "NSE:" + order.Symbol + "-EQ",
			"qty":          order.Quantity,
			"type":         orderType,
			"side":         side,
			"productType":  "CNC",
			"limitPrice":   order.Price,
			"stopPrice":    0,
			"validity":     "DAY",
			"disclosedQty": 0,
			"offlineOrder": false,
		}).
		SetResult(&res).
		Post("/orders/sync")
	if err != nil {
		return failedResult(order, err.Error()), fmt.Errorf("fyers place_order: %w", err)
	}
	if resp.IsError() || res.ID == "" {
		return failedResult(order, res.Message), fmt.Errorf("fyers place_order: %s", res.Message)
	}

	return &models.OrderResult{
		OrderID:   res.ID,
		Symbol:    order.Symbol,
		Quantity:  order.Quantity,
		Side:      order.Side,
		Status:    models.OrderStatusPending,
		Timestamp: time.Now(),
	}, nil
}

// GetOrderStatus retrieves the state of a Fyers order.
func (a *Adapter) GetOrderStatus(ctx context.Context, sess *session.Session, orderID string) (*models.OrderResult, error) {
	var res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    []struct {
			ID     string  `json:"id"`
			Status int     `json:"status"`
			Symbol string  `json:"symbol"`
			Qty    int     `json:"qty"`
			Side   int     `json:"side"`
			TradedPrice float64 `json:"tradedPrice"`
			Message string `json:"message"`
		} `json:"orderBook"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", sess.AccessToken).
		SetResult(&res).
		SetQueryParam("id", orderID).
		Get("/orders")
	if err != nil {
		return nil, fmt.Errorf("fyers order_status: %w", err)
	}
	if resp.IsError() || len(res.Data) == 0 {
		return nil, fmt.Errorf("fyers order_status: order not found %s", orderID)
	}

	d := res.Data[0]
	side := models.OrderSideBuy
	if d.Side == -1 {
		side = models.OrderSideSell
	}

	return &models.OrderResult{
		OrderID:   d.ID,
		Symbol:    d.Symbol,
		Quantity:  d.Qty,
		Side:      side,
		Status:    mapFyersStatus(d.Status),
		Price:     d.TradedPrice,
		Message:   d.Message,
		Timestamp: time.Now(),
	}, nil
}

// CancelOrder cancels an open Fyers order.
func (a *Adapter) CancelOrder(ctx context.Context, sess *session.Session, orderID string) error {
	var res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", sess.AccessToken).
		SetBody(map[string]string{"id": orderID}).
		SetResult(&res).
		Delete("/orders/sync")
	if err != nil {
		return fmt.Errorf("fyers cancel_order: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("fyers cancel_order: %s", res.Message)
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// Fyers status codes: 1=Cancelled, 2=Traded, 4=Transit, 5=Rejected, 6=Pending
func mapFyersStatus(s int) models.OrderStatus {
	switch s {
	case 2:
		return models.OrderStatusComplete
	case 5:
		return models.OrderStatusRejected
	case 1:
		return models.OrderStatusCancelled
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
