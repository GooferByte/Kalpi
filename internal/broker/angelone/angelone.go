// Package angelone implements broker.Adapter for AngelOne SmartAPI.
//
// Auth flow:
//  1. POST /user/v1/loginByPassword with clientcode, password, totp → jwtToken.
//  2. All subsequent calls use "Authorization: Bearer <jwtToken>".
package angelone

import (
	"context"
	"fmt"
	"time"

	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const (
	baseURL = "https://apiconnect.angelone.in"
	appName = "KalpiEngine"
)

// Adapter is the AngelOne implementation of broker.Adapter.
type Adapter struct {
	client *resty.Client
	logger *zap.Logger
}

// New creates a new AngelOne adapter.
func New(logger *zap.Logger) *Adapter {
	return &Adapter{
		logger: logger,
		client: resty.New().
			SetBaseURL(baseURL).
			SetTimeout(30 * time.Second).
			SetRetryCount(2).
			SetHeader("Content-Type", "application/json").
			SetHeader("Accept", "application/json").
			SetHeader("X-UserType", "USER").
			SetHeader("X-SourceID", "WEB"),
	}
}

func (a *Adapter) Name() string { return "angelone" }

// Authenticate logs in via SmartAPI and returns a JWT token.
// Credentials required: APIKey, ClientCode, Password, TOTP.
func (a *Adapter) Authenticate(ctx context.Context, creds models.Credentials) (*models.AuthResponse, error) {
	var res struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			JwtToken    string `json:"jwtToken"`
			RefreshToken string `json:"refreshToken"`
			FeedToken   string `json:"feedToken"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("X-ClientLocalIP", "192.168.1.1").
		SetHeader("X-ClientPublicIP", "192.168.1.1").
		SetHeader("X-MACAddress", "00:00:00:00:00:00").
		SetHeader("X-PrivateKey", creds.APIKey).
		SetBody(map[string]string{
			"clientcode": creds.ClientCode,
			"password":   creds.Password,
			"totp":       creds.TOTP,
		}).
		SetResult(&res).
		Post("/rest/auth/angelbroking/user/v1/loginByPassword")
	if err != nil {
		return nil, fmt.Errorf("angelone authenticate: %w", err)
	}
	if resp.IsError() || !res.Status {
		return nil, fmt.Errorf("angelone authenticate: %s", res.Message)
	}
	return &models.AuthResponse{
		Broker:      a.Name(),
		AccessToken: res.Data.JwtToken,
		Message:     "authenticated successfully",
	}, nil
}

// GetHoldings fetches equity holdings from AngelOne.
func (a *Adapter) GetHoldings(ctx context.Context, sess *session.Session) ([]models.Holding, error) {
	var res struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    []struct {
			TradingSymbol string  `json:"tradingsymbol"`
			Quantity      int     `json:"quantity"`
			AveragePrice  float64 `json:"averageprice"`
			Close         float64 `json:"close"`
			PnL           float64 `json:"profitandloss"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+sess.AccessToken).
		SetHeader("X-PrivateKey", sess.APIKey).
		SetResult(&res).
		Get("/rest/secure/angelbroking/portfolio/v1/getHolding")
	if err != nil {
		return nil, fmt.Errorf("angelone get_holdings: %w", err)
	}
	if resp.IsError() || !res.Status {
		return nil, fmt.Errorf("angelone get_holdings: %s", res.Message)
	}

	out := make([]models.Holding, 0, len(res.Data))
	for _, h := range res.Data {
		out = append(out, models.Holding{
			Symbol:       h.TradingSymbol,
			Quantity:     h.Quantity,
			AvgPrice:     h.AveragePrice,
			CurrentPrice: h.Close,
			PnL:          h.PnL,
		})
	}
	return out, nil
}

// PlaceOrder submits an order to AngelOne.
func (a *Adapter) PlaceOrder(ctx context.Context, sess *session.Session, order models.Order) (*models.OrderResult, error) {
	orderType := string(order.OrderType)
	if orderType == "" {
		orderType = "MARKET"
	}

	var res struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			OrderID string `json:"orderid"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+sess.AccessToken).
		SetHeader("X-PrivateKey", sess.APIKey).
		SetBody(map[string]interface{}{
			"variety":         "NORMAL",
			"tradingsymbol":   order.Symbol,
			"symboltoken":     "",
			"transactiontype": string(order.Side),
			"exchange":        "NSE",
			"ordertype":       orderType,
			"producttype":     "DELIVERY",
			"duration":        "DAY",
			"price":           fmt.Sprintf("%.2f", order.Price),
			"squareoff":       "0",
			"stoploss":        "0",
			"quantity":        fmt.Sprintf("%d", order.Quantity),
		}).
		SetResult(&res).
		Post("/rest/secure/angelbroking/order/v1/placeOrder")
	if err != nil {
		return failedResult(order, err.Error()), fmt.Errorf("angelone place_order: %w", err)
	}
	if resp.IsError() || !res.Status {
		return failedResult(order, res.Message), fmt.Errorf("angelone place_order: %s", res.Message)
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

// GetOrderStatus retrieves the state of an AngelOne order.
func (a *Adapter) GetOrderStatus(ctx context.Context, sess *session.Session, orderID string) (*models.OrderResult, error) {
	var res struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			OrderID         string  `json:"orderid"`
			Status          string  `json:"orderstatus"`
			TradingSymbol   string  `json:"tradingsymbol"`
			Quantity        int     `json:"quantity"`
			TransactionType string  `json:"transactiontype"`
			AveragePrice    float64 `json:"averageprice"`
			Text            string  `json:"text"`
		} `json:"data"`
	}

	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+sess.AccessToken).
		SetHeader("X-PrivateKey", sess.APIKey).
		SetResult(&res).
		Get("/rest/secure/angelbroking/order/v1/details/" + orderID)
	if err != nil {
		return nil, fmt.Errorf("angelone order_status: %w", err)
	}
	if resp.IsError() || !res.Status {
		return nil, fmt.Errorf("angelone order_status: %s", res.Message)
	}

	return &models.OrderResult{
		OrderID:   res.Data.OrderID,
		Symbol:    res.Data.TradingSymbol,
		Quantity:  res.Data.Quantity,
		Side:      models.OrderSide(res.Data.TransactionType),
		Status:    mapStatus(res.Data.Status),
		Price:     res.Data.AveragePrice,
		Message:   res.Data.Text,
		Timestamp: time.Now(),
	}, nil
}

// CancelOrder cancels a pending AngelOne order.
func (a *Adapter) CancelOrder(ctx context.Context, sess *session.Session, orderID string) error {
	var res struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	resp, err := a.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+sess.AccessToken).
		SetHeader("X-PrivateKey", sess.APIKey).
		SetBody(map[string]string{
			"variety": "NORMAL",
			"orderid": orderID,
		}).
		SetResult(&res).
		Post("/rest/secure/angelbroking/order/v1/cancelOrder")
	if err != nil {
		return fmt.Errorf("angelone cancel_order: %w", err)
	}
	if resp.IsError() || !res.Status {
		return fmt.Errorf("angelone cancel_order: %s", res.Message)
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
