package handlers

import (
	"net/http"

	"github.com/GooferByte/kalpi/internal/broker"
	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthHandler handles broker authentication endpoints.
type AuthHandler struct {
	sessionMgr session.Manager
	logger     *zap.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(sessionMgr session.Manager, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{sessionMgr: sessionMgr, logger: logger}
}

// Authenticate godoc
// POST /api/v1/auth/:broker
//
// Authenticates the user with the specified broker and returns a session_id
// that must be passed in subsequent portfolio execution calls.
//
// Path params:
//   - broker: one of zerodha | fyers | angelone | upstox | groww | mock
//
// Body: models.AuthRequest (credentials vary per broker — see README)
func (h *AuthHandler) Authenticate(c *gin.Context) {
	brokerName := c.Param("broker")

	var req models.AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
		return
	}

	// Build adapter for the requested broker
	adapter, err := broker.NewAdapter(brokerName, h.logger)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Authenticate with the broker
	authResp, err := adapter.Authenticate(c.Request.Context(), req.Credentials)
	if err != nil {
		h.logger.Error("broker authentication failed",
			zap.String("broker", brokerName),
			zap.Error(err),
		)
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Create session
	sess := h.sessionMgr.Create(
		brokerName,
		authResp.AccessToken,
		req.Credentials.APIKey,
		authResp.UserID,
	)
	authResp.SessionID = sess.ID

	h.logger.Info("broker authenticated",
		zap.String("broker", brokerName),
		zap.String("session_id", sess.ID),
	)

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "authenticated successfully",
		Data:    authResp,
	})
}

// Logout godoc
// DELETE /api/v1/auth/session/:session_id
//
// Invalidates a session.
func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID := c.Param("session_id")
	h.sessionMgr.Delete(sessionID)
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "session invalidated",
	})
}

// ListBrokers godoc
// GET /api/v1/brokers
//
// Returns the list of supported broker identifiers.
func (h *AuthHandler) ListBrokers(c *gin.Context) {
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    broker.SupportedBrokers(),
	})
}
