package notification

import (
	"context"
	"fmt"
	"strings"

	"github.com/GooferByte/kalpi/internal/models"
)

// CompositeNotifier chains multiple Notifiers and calls each one in sequence.
// All notifiers are attempted even if one fails; errors are aggregated.
type CompositeNotifier struct {
	notifiers []Notifier
}

// NewCompositeNotifier creates a Notifier that dispatches to all provided notifiers.
func NewCompositeNotifier(notifiers ...Notifier) Notifier {
	return &CompositeNotifier{notifiers: notifiers}
}

func (c *CompositeNotifier) Notify(ctx context.Context, r *models.ExecutionResult) error {
	var errs []string
	for _, n := range c.notifiers {
		if err := n.Notify(ctx, r); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("composite notifier errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
