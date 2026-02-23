// Package engine contains the core execution logic for portfolio trade runs.
// The Executor orchestrates broker adapters, order management, and notifications
// without knowing the concrete implementations of any of them.
package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/GooferByte/kalpi/internal/broker"
	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/notification"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Executor is the interface for running portfolio executions.
type Executor interface {
	// Execute runs a full portfolio execution (first-time or rebalance) and returns the result.
	Execute(ctx context.Context, req *models.ExecutionRequest) (*models.ExecutionResult, error)

	// GetExecution fetches a previously run execution result by ID.
	GetExecution(id string) (*models.ExecutionResult, bool)

	// ListExecutions returns all execution results.
	ListExecutions() []*models.ExecutionResult
}

type executorImpl struct {
	sessionMgr session.Manager
	store      ExecutionStore
	orderMgr   OrderManager
	notifier   notification.Notifier
	logger     *zap.Logger
}

// NewExecutor creates a new Executor wired with its dependencies.
func NewExecutor(
	sessionMgr session.Manager,
	store ExecutionStore,
	orderMgr OrderManager,
	notifier notification.Notifier,
	logger *zap.Logger,
) Executor {
	return &executorImpl{
		sessionMgr: sessionMgr,
		store:      store,
		orderMgr:   orderMgr,
		notifier:   notifier,
		logger:     logger,
	}
}

// Execute is the main entry point for a portfolio execution run.
//
// Execution order (important for capital management):
//  1. SELL orders run concurrently first — frees up capital.
//  2. REBALANCE adjustments: negative qty_change → SELL, positive → BUY.
//  3. BUY orders run concurrently after sells complete.
func (e *executorImpl) Execute(ctx context.Context, req *models.ExecutionRequest) (*models.ExecutionResult, error) {
	// ── 1. Resolve session ────────────────────────────────────────────────
	sess, ok := e.sessionMgr.Get(req.SessionID)
	if !ok {
		return nil, fmt.Errorf("session not found or expired: %s", req.SessionID)
	}

	// ── 2. Build broker adapter ───────────────────────────────────────────
	adapter, err := broker.NewAdapter(req.Broker, e.logger)
	if err != nil {
		return nil, fmt.Errorf("unknown broker: %w", err)
	}

	// ── 3. Initialise result ──────────────────────────────────────────────
	result := &models.ExecutionResult{
		ExecutionID: uuid.New().String(),
		Broker:      req.Broker,
		Mode:        req.Mode,
		Status:      "in_progress",
		Timestamp:   time.Now(),
	}
	e.store.Save(result)

	e.logger.Info("execution started",
		zap.String("execution_id", result.ExecutionID),
		zap.String("broker", req.Broker),
		zap.String("mode", string(req.Mode)),
	)

	// ── 4. Build order lists ──────────────────────────────────────────────
	sellOrders, buyOrders := buildOrders(req)

	// ── 5. Execute SELLs first (concurrent) ──────────────────────────────
	var allResults []models.OrderResult
	if len(sellOrders) > 0 {
		e.logger.Info("placing sell orders", zap.Int("count", len(sellOrders)))
		sellResults := e.orderMgr.PlaceBatch(ctx, adapter, sess, sellOrders)
		allResults = append(allResults, sellResults...)
	}

	// ── 6. Execute BUYs (concurrent, after sells) ─────────────────────────
	if len(buyOrders) > 0 {
		e.logger.Info("placing buy orders", zap.Int("count", len(buyOrders)))
		buyResults := e.orderMgr.PlaceBatch(ctx, adapter, sess, buyOrders)
		allResults = append(allResults, buyResults...)
	}

	// ── 7. Compile final result ───────────────────────────────────────────
	for _, r := range allResults {
		r := r
		if r.Status == models.OrderStatusComplete ||
			r.Status == models.OrderStatusPending ||
			r.Status == models.OrderStatusOpen {
			result.SuccessfulOrders = append(result.SuccessfulOrders, r)
			result.SuccessCount++
		} else {
			result.FailedOrders = append(result.FailedOrders, r)
			result.FailureCount++
		}
	}

	result.TotalOrders = len(allResults)
	result.Status = "completed"
	now := time.Now()
	result.CompletedAt = &now

	e.store.Save(result)

	e.logger.Info("execution completed",
		zap.String("execution_id", result.ExecutionID),
		zap.Int("total", result.TotalOrders),
		zap.Int("success", result.SuccessCount),
		zap.Int("failed", result.FailureCount),
	)

	// ── 8. Notify (non-blocking) ──────────────────────────────────────────
	go func() {
		if err := e.notifier.Notify(context.Background(), result); err != nil {
			e.logger.Error("notification failed", zap.Error(err))
		}
	}()

	return result, nil
}

func (e *executorImpl) GetExecution(id string) (*models.ExecutionResult, bool) {
	return e.store.Get(id)
}

func (e *executorImpl) ListExecutions() []*models.ExecutionResult {
	return e.store.List()
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// buildOrders translates the raw ExecutionRequest into two categorised order slices.
// For first_time mode: everything goes into buyOrders.
// For rebalance mode:
//   - req.Orders.Sell  → sell orders
//   - req.Orders.Buy   → buy orders
//   - req.Orders.Rebalance with QtyChange < 0 → sell (|qty_change| qty)
//   - req.Orders.Rebalance with QtyChange > 0 → buy (qty_change qty)
func buildOrders(req *models.ExecutionRequest) (sells, buys []models.Order) {
	defaultOrderType := models.OrderTypeMarket

	toOrder := func(inst models.TradeInstruction, side models.OrderSide, qty int) models.Order {
		ot := inst.OrderType
		if ot == "" {
			ot = defaultOrderType
		}
		return models.Order{
			Symbol:    inst.Symbol,
			Quantity:  qty,
			Side:      side,
			OrderType: ot,
			Price:     inst.Price,
		}
	}

	if req.Mode == models.ExecutionModeFirstTime {
		for _, inst := range req.Orders.Buy {
			if inst.Qty > 0 {
				buys = append(buys, toOrder(inst, models.OrderSideBuy, inst.Qty))
			}
		}
		return
	}

	// Rebalance mode
	for _, inst := range req.Orders.Sell {
		if inst.Qty > 0 {
			sells = append(sells, toOrder(inst, models.OrderSideSell, inst.Qty))
		}
	}
	for _, inst := range req.Orders.Buy {
		if inst.Qty > 0 {
			buys = append(buys, toOrder(inst, models.OrderSideBuy, inst.Qty))
		}
	}
	for _, inst := range req.Orders.Rebalance {
		switch {
		case inst.QtyChange < 0:
			sells = append(sells, toOrder(inst, models.OrderSideSell, -inst.QtyChange))
		case inst.QtyChange > 0:
			buys = append(buys, toOrder(inst, models.OrderSideBuy, inst.QtyChange))
		}
	}
	return
}
