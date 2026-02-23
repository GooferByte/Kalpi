package handlers

import (
	"github.com/GooferByte/kalpi/internal/notification"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WebSocketHandler upgrades HTTP connections to WebSocket for real-time notifications.
type WebSocketHandler struct {
	hub    *notification.Hub
	logger *zap.Logger
}

// NewWebSocketHandler creates a new WebSocketHandler.
func NewWebSocketHandler(hub *notification.Hub, logger *zap.Logger) *WebSocketHandler {
	return &WebSocketHandler{hub: hub, logger: logger}
}

// Handle godoc
// GET /ws/notifications
//
// Upgrades to a WebSocket connection. The server pushes ExecutionResult JSON
// messages whenever an execution completes.
func (h *WebSocketHandler) Handle(c *gin.Context) {
	h.hub.ServeWS(c.Writer, c.Request)
}
