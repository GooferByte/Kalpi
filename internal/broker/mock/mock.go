// Package mock provides a fake broker adapter for local development and testing.
// All operations succeed instantly with simulated data — no real broker credentials needed.
// Enable it by setting broker = "mock" in the request payload.
package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Adapter is the mock implementation of broker.Adapter.
type Adapter struct {
	logger *zap.Logger
}

// New creates a new mock broker adapter.
func New(logger *zap.Logger) *Adapter {
	return &Adapter{logger: logger}
}

func (a *Adapter) Name() string { return "mock" }

// Authenticate always succeeds and returns a fake token.
func (a *Adapter) Authenticate(_ context.Context, creds models.Credentials) (*models.AuthResponse, error) {
	a.logger.Info("mock broker: authenticate called")
	return &models.AuthResponse{
		Broker:      a.Name(),
		AccessToken: "mock-access-token-" + uuid.New().String(),
		UserID:      "mock-user-001",
		Message:     "mock authentication successful",
	}, nil
}

// GetHoldings returns a static set of fake equity holdings.
func (a *Adapter) GetHoldings(_ context.Context, _ *session.Session) ([]models.Holding, error) {
	a.logger.Info("mock broker: get_holdings called")
	return []models.Holding{
		{Symbol: "RELIANCE", Quantity: 10, AvgPrice: 2400.00, CurrentPrice: 2550.00, PnL: 1500.00},
		{Symbol: "TCS", Quantity: 5, AvgPrice: 3500.00, CurrentPrice: 3650.00, PnL: 750.00},
		{Symbol: "HDFC", Quantity: 8, AvgPrice: 1600.00, CurrentPrice: 1700.00, PnL: 800.00},
	}, nil
}

// PlaceOrder simulates placing an order and returns a completed result immediately.
func (a *Adapter) PlaceOrder(_ context.Context, _ *session.Session, order models.Order) (*models.OrderResult, error) {
	a.logger.Info("mock broker: place_order called",
		zap.String("symbol", order.Symbol),
		zap.String("side", string(order.Side)),
		zap.Int("qty", order.Quantity),
	)
	return &models.OrderResult{
		OrderID:   fmt.Sprintf("MOCK-%s", uuid.New().String()[:8]),
		Symbol:    order.Symbol,
		Quantity:  order.Quantity,
		Side:      order.Side,
		Status:    models.OrderStatusComplete,
		Price:     1000.00, // Simulated fill price
		Message:   "mock order executed",
		Timestamp: time.Now(),
	}, nil
}

// GetOrderStatus always returns a completed status.
func (a *Adapter) GetOrderStatus(_ context.Context, _ *session.Session, orderID string) (*models.OrderResult, error) {
	a.logger.Info("mock broker: get_order_status called", zap.String("order_id", orderID))
	return &models.OrderResult{
		OrderID:   orderID,
		Status:    models.OrderStatusComplete,
		Price:     1000.00,
		Timestamp: time.Now(),
	}, nil
}

// CancelOrder always succeeds.
func (a *Adapter) CancelOrder(_ context.Context, _ *session.Session, orderID string) error {
	a.logger.Info("mock broker: cancel_order called", zap.String("order_id", orderID))
	return nil
}
