// Package broker defines the Adapter interface that every broker implementation must satisfy.
// This is the core of the Adapter Pattern: the execution engine only ever talks to this
// interface, so adding a new broker requires zero changes to the engine.
package broker

import (
	"context"

	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
)

// Adapter is the normalised contract for all broker integrations.
// To add broker #6, create a new package under internal/broker/<name>/,
// implement all methods below, and register it in factory.go.
type Adapter interface {
	// Authenticate exchanges raw credentials for a broker access token.
	// The returned AuthResponse contains the access_token stored in the session.
	Authenticate(ctx context.Context, creds models.Credentials) (*models.AuthResponse, error)

	// GetHoldings returns the user's current equity holdings.
	GetHoldings(ctx context.Context, sess *session.Session) ([]models.Holding, error)

	// PlaceOrder submits a single trade order and returns an initial OrderResult.
	// The result status may be PENDING — callers should poll GetOrderStatus.
	PlaceOrder(ctx context.Context, sess *session.Session, order models.Order) (*models.OrderResult, error)

	// GetOrderStatus fetches the latest status of a previously placed order.
	GetOrderStatus(ctx context.Context, sess *session.Session, orderID string) (*models.OrderResult, error)

	// CancelOrder attempts to cancel an open/pending order.
	CancelOrder(ctx context.Context, sess *session.Session, orderID string) error

	// Name returns the lowercase broker identifier (e.g. "zerodha").
	Name() string
}
