// Package notification defines the Notifier interface and provides concrete
// implementations: LogNotifier, WebhookNotifier, WebSocketNotifier, and
// CompositeNotifier (which chains them).
package notification

import (
	"context"

	"github.com/GooferByte/kalpi/internal/models"
)

// Notifier is the single interface for all notification strategies.
// Swap or compose implementations without touching the engine.
type Notifier interface {
	// Notify dispatches the execution result to the underlying channel.
	// Implementations should be non-blocking where possible.
	Notify(ctx context.Context, result *models.ExecutionResult) error
}
