package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GooferByte/kalpi/internal/broker"
	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
	"go.uber.org/zap"
)

// OrderManager is the interface for placing orders with retry and concurrency.
type OrderManager interface {
	// PlaceBatch places a slice of orders concurrently and returns all results.
	// It never returns an error — individual order failures are captured in the result slice.
	PlaceBatch(ctx context.Context, adapter broker.Adapter, sess *session.Session, orders []models.Order) []models.OrderResult

	// PlaceWithRetry places a single order with exponential-backoff retry on transient errors.
	PlaceWithRetry(ctx context.Context, adapter broker.Adapter, sess *session.Session, order models.Order) (*models.OrderResult, error)
}

type orderManagerImpl struct {
	maxRetries int
	logger     *zap.Logger
}

// NewOrderManager creates an OrderManager with a configurable retry count.
func NewOrderManager(logger *zap.Logger, maxRetries int) OrderManager {
	return &orderManagerImpl{
		maxRetries: maxRetries,
		logger:     logger,
	}
}

// PlaceBatch launches each order in its own goroutine using a WaitGroup.
// Results are written into a pre-allocated slice (one slot per order) so no mutex is needed.
func (m *orderManagerImpl) PlaceBatch(
	ctx context.Context,
	adapter broker.Adapter,
	sess *session.Session,
	orders []models.Order,
) []models.OrderResult {
	if len(orders) == 0 {
		return nil
	}

	results := make([]models.OrderResult, len(orders))
	var wg sync.WaitGroup

	for i, order := range orders {
		wg.Add(1)
		go func(idx int, o models.Order) {
			defer wg.Done()
			result, err := m.PlaceWithRetry(ctx, adapter, sess, o)
			if err != nil {
				results[idx] = models.OrderResult{
					Symbol:    o.Symbol,
					Quantity:  o.Quantity,
					Side:      o.Side,
					Status:    models.OrderStatusFailed,
					Message:   err.Error(),
					Timestamp: time.Now(),
				}
				m.logger.Error("order failed",
					zap.String("symbol", o.Symbol),
					zap.String("side", string(o.Side)),
					zap.Error(err),
				)
				return
			}
			results[idx] = *result
			m.logger.Info("order placed",
				zap.String("symbol", o.Symbol),
				zap.String("order_id", result.OrderID),
				zap.String("status", string(result.Status)),
			)
		}(i, order)
	}

	wg.Wait()
	return results
}

// PlaceWithRetry places an order and retries on transient (rate-limit / network) errors
// using exponential backoff: 1s → 2s → 4s → ...
func (m *orderManagerImpl) PlaceWithRetry(
	ctx context.Context,
	adapter broker.Adapter,
	sess *session.Session,
	order models.Order,
) (*models.OrderResult, error) {
	var lastErr error

	for attempt := 0; attempt <= m.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoff):
			}
			m.logger.Warn("retrying order placement",
				zap.String("symbol", order.Symbol),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
			)
		}

		result, err := adapter.PlaceOrder(ctx, sess, order)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	return nil, fmt.Errorf("order failed after %d attempt(s): %w", m.maxRetries+1, lastErr)
}

// isRetryable returns true for transient errors (rate limits, network timeouts).
// Permanent errors (insufficient funds, invalid symbol) should not be retried.
func isRetryable(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "too many requests")
}
