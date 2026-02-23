package broker

import (
	"fmt"

	"github.com/GooferByte/kalpi/internal/broker/angelone"
	"github.com/GooferByte/kalpi/internal/broker/fyers"
	"github.com/GooferByte/kalpi/internal/broker/groww"
	"github.com/GooferByte/kalpi/internal/broker/mock"
	"github.com/GooferByte/kalpi/internal/broker/upstox"
	"github.com/GooferByte/kalpi/internal/broker/zerodha"
	"go.uber.org/zap"
)

// NewAdapter is the factory function that returns the correct broker Adapter
// for a given broker name. To add broker #6:
//  1. Create internal/broker/<name>/<name>.go implementing the Adapter interface.
//  2. Add a case here.
//  3. Done — zero changes to the engine or handlers.
func NewAdapter(brokerName string, logger *zap.Logger) (Adapter, error) {
	switch brokerName {
	case "zerodha":
		return zerodha.New(logger), nil
	case "fyers":
		return fyers.New(logger), nil
	case "angelone":
		return angelone.New(logger), nil
	case "upstox":
		return upstox.New(logger), nil
	case "groww":
		return groww.New(logger), nil
	case "mock":
		return mock.New(logger), nil
	default:
		return nil, fmt.Errorf("unsupported broker %q — supported: %v", brokerName, SupportedBrokers())
	}
}

// SupportedBrokers returns the canonical list of broker identifiers.
func SupportedBrokers() []string {
	return []string{"zerodha", "fyers", "angelone", "upstox", "groww", "mock"}
}
