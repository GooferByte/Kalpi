// Package upstox implements broker.Adapter for Upstox API v2.
//
// Auth flow (OAuth2):
//  1. User visits Upstox auth URL → authorises → redirect with ?code=<auth_code>.
//  2. We POST /login/authorization/token with code + client_id + client_secret → access_token.
//
// Auth header: "Authorization: Bearer <access_token>"
package upstox

import (
	"context"
	"fmt"
	"time"

	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const baseURL = "https://api.upstox.com/v2"

// Adapter is the Upstox implementation of broker.Adapter.
type Adapter struct {
	client *resty.Client
	logger *zap.Logger
}

// New creates a new Upstox adapter.
func New(logger *zap.Logger) *Adapter {
	return &Adapter{
		logger: logger,
		client: resty.New().
			SetBaseURL(baseURL).
			SetTimeout(30 * time.Second).
			SetRetryCount(2).
			SetHeader("Accept", "application/json"),
	}
}

func (a *Adapter) Name() string { return "upstox" }

// Authenticate exchanges an OAuth auth_code for an Upstox access_token.
// Credentials required: APIKey (client_id), APISecret (client_secret), AuthCode, RedirectURI.
func (a *Adapter) Authenticate(ctx context.Context, creds models.Credentials) (*models.AuthResponse, error) {
	var res struct {
		Status  string `json:"status"`
		Data    struct {
			AccessToken string `json:"access_token"`
			UserID      string `json:"user_id"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetFormData(map[string]string{
			"code":          creds.AuthCode,
			"client_id":     creds.APIKey,
			"client_secret": creds.APISecret,
			"redirect_uri":  creds.RedirectURI,
			"grant_type":    "authorization_code",
		}).
		SetResult(&res).
		Post("/login/authorization/token")
	if err != nil {
		return nil, fmt.Errorf("upstox authenticate: %w", err)
	}
	if resp.IsError() || res.Data.AccessToken == "" {
		msg := "unknown error"
		if len(res.Errors) > 0 {
			msg = res.Errors[0].Message
		}
		return nil, fmt.Errorf("upstox authenticate: %s", msg)
	}
	return &models.AuthResponse{
		Broker:      a.Name(),
		AccessToken: res.Data.AccessToken,
		UserID:      res.Data.UserID,
		Message:     "authenticated successfully",
	}, nil
}

// GetHoldings fetches long-term equity holdings from Upstox.
func (a *Adapter) GetHoldings(ctx context.Context, sess *session.Session) ([]models.Holding, error) {
	var res struct {
		Status string `json:"status"`
		Data   []struct {
			ISIN           string  `json:"isin"`
			TradingSymbol  string  `json:"trading_symbol"`
			Quantity       int     `json:"quantity"`
			AveragePrice   float64 `json:"average_price"`
			LastPrice      float64 `json:"last_price"`
			PnL            float64 `json:"pnl"`
		} `json:"data"`
		Errors []struct{ Message string `json:"message"` } `json:"errors"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+sess.AccessToken).
		SetResult(&res).
		Get("/portfolio/long-term-holdings")
	if err != nil {
		return nil, fmt.Errorf("upstox get_holdings: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("upstox get_holdings: %v", res.Errors)
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

// PlaceOrder places an order via Upstox.
func (a *Adapter) PlaceOrder(ctx context.Context, sess *session.Session, order models.Order) (*models.OrderResult, error) {
	orderType := string(order.OrderType)
	if orderType == "" {
		orderType = "MARKET"
	}

	var res struct {
		Status string `json:"status"`
		Data   struct {
			OrderID string `json:"order_id"`
		} `json:"data"`
		Errors []struct{ Message string `json:"message"` } `json:"errors"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+sess.AccessToken).
		SetBody(map[string]interface{}{
			"quantity":          order.Quantity,
			"product":           "D",  // Delivery
			"validity":          "DAY",
			"price":             order.Price,
			"tag":               "kalpi",
			"instrument_token":  "NSE_EQ|" + order.Symbol,
			"order_type":        orderType,
			"transaction_type":  string(order.Side),
			"disclosed_quantity": 0,
			"trigger_price":     order.TriggerPrice,
			"is_amo":            false,
		}).
		SetResult(&res).
		Post("/order/place")
	if err != nil {
		return failedResult(order, err.Error()), fmt.Errorf("upstox place_order: %w", err)
	}
	if resp.IsError() || res.Data.OrderID == "" {
		msg := "unknown error"
		if len(res.Errors) > 0 {
			msg = res.Errors[0].Message
		}
		return failedResult(order, msg), fmt.Errorf("upstox place_order: %s", msg)
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

// GetOrderStatus retrieves the state of an Upstox order.
func (a *Adapter) GetOrderStatus(ctx context.Context, sess *session.Session, orderID string) (*models.OrderResult, error) {
	var res struct {
		Status string `json:"status"`
		Data   struct {
			OrderID         string  `json:"order_id"`
			Status          string  `json:"status"`
			TradingSymbol   string  `json:"trading_symbol"`
			Quantity        int     `json:"quantity"`
			TransactionType string  `json:"transaction_type"`
			AveragePrice    float64 `json:"average_price"`
			StatusMessage   string  `json:"status_message"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+sess.AccessToken).
		SetResult(&res).
		SetQueryParam("order_id", orderID).
		Get("/order/details")
	if err != nil {
		return nil, fmt.Errorf("upstox order_status: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("upstox order_status: order not found %s", orderID)
	}

	return &models.OrderResult{
		OrderID:   res.Data.OrderID,
		Symbol:    res.Data.TradingSymbol,
		Quantity:  res.Data.Quantity,
		Side:      models.OrderSide(res.Data.TransactionType),
		Status:    mapStatus(res.Data.Status),
		Price:     res.Data.AveragePrice,
		Message:   res.Data.StatusMessage,
		Timestamp: time.Now(),
	}, nil
}

// CancelOrder cancels an open Upstox order.
func (a *Adapter) CancelOrder(ctx context.Context, sess *session.Session, orderID string) error {
	var res struct {
		Status string `json:"status"`
		Errors []struct{ Message string `json:"message"` } `json:"errors"`
	}
	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+sess.AccessToken).
		SetResult(&res).
		SetQueryParam("order_id", orderID).
		Delete("/order/cancel")
	if err != nil {
		return fmt.Errorf("upstox cancel_order: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("upstox cancel_order: %v", res.Errors)
	}
	return nil
}

func mapStatus(s string) models.OrderStatus {
	switch s {
	case "complete":
		return models.OrderStatusComplete
	case "rejected":
		return models.OrderStatusRejected
	case "cancelled":
		return models.OrderStatusCancelled
	case "open":
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
