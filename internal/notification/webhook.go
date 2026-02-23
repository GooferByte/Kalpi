package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/GooferByte/kalpi/internal/models"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

// WebhookNotifier POSTs the execution result as JSON to a caller-supplied URL.
type WebhookNotifier struct {
	url    string
	client *resty.Client
	logger *zap.Logger
}

// NewWebhookNotifier creates a WebhookNotifier that posts to the given URL.
func NewWebhookNotifier(url string, logger *zap.Logger) Notifier {
	return &WebhookNotifier{
		url:    url,
		logger: logger,
		client: resty.New().
			SetTimeout(10 * time.Second).
			SetRetryCount(2).
			SetRetryWaitTime(1 * time.Second),
	}
}

func (n *WebhookNotifier) Notify(ctx context.Context, r *models.ExecutionResult) error {
	if n.url == "" {
		return nil
	}
	resp, err := n.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Kalpi-Event", "execution.completed").
		SetBody(r).
		Post(n.url)
	if err != nil {
		return fmt.Errorf("webhook notify: %w", err)
	}
	if resp.IsError() {
		n.logger.Warn("webhook returned non-2xx",
			zap.String("url", n.url),
			zap.Int("status", resp.StatusCode()),
		)
	}
	return nil
}
