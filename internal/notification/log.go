package notification

import (
	"context"

	"github.com/GooferByte/kalpi/internal/models"
	"go.uber.org/zap"
)

// LogNotifier writes execution results to the structured logger.
// Used as a fallback when no webhook URL or WebSocket clients are present.
type LogNotifier struct {
	logger *zap.Logger
}

// NewLogNotifier creates a LogNotifier backed by the given zap logger.
func NewLogNotifier(logger *zap.Logger) Notifier {
	return &LogNotifier{logger: logger}
}

func (n *LogNotifier) Notify(_ context.Context, r *models.ExecutionResult) error {
	n.logger.Info("execution notification",
		zap.String("execution_id", r.ExecutionID),
		zap.String("broker", r.Broker),
		zap.String("mode", string(r.Mode)),
		zap.String("status", r.Status),
		zap.Int("total_orders", r.TotalOrders),
		zap.Int("success", r.SuccessCount),
		zap.Int("failed", r.FailureCount),
	)
	for _, o := range r.SuccessfulOrders {
		n.logger.Info("  ✓ order",
			zap.String("order_id", o.OrderID),
			zap.String("symbol", o.Symbol),
			zap.String("side", string(o.Side)),
			zap.Int("qty", o.Quantity),
			zap.String("status", string(o.Status)),
		)
	}
	for _, o := range r.FailedOrders {
		n.logger.Warn("  ✗ order failed",
			zap.String("symbol", o.Symbol),
			zap.String("side", string(o.Side)),
			zap.Int("qty", o.Quantity),
			zap.String("message", o.Message),
		)
	}
	return nil
}
