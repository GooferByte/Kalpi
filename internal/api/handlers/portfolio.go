package handlers

import (
	"net/http"

	"github.com/GooferByte/kalpi/internal/broker"
	"github.com/GooferByte/kalpi/internal/engine"
	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PortfolioHandler handles portfolio execution endpoints.
type PortfolioHandler struct {
	executor   engine.Executor
	sessionMgr session.Manager
	logger     *zap.Logger
}

// NewPortfolioHandler creates a new PortfolioHandler.
func NewPortfolioHandler(exec engine.Executor, sessionMgr session.Manager, logger *zap.Logger) *PortfolioHandler {
	return &PortfolioHandler{executor: exec, sessionMgr: sessionMgr, logger: logger}
}

// Execute godoc
// POST /api/v1/portfolio/execute
//
// Executes a first-time portfolio: buys all listed stocks.
// Body: models.ExecutionRequest with mode = "first_time"
func (h *PortfolioHandler) Execute(c *gin.Context) {
	var req models.ExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}
	req.Mode = models.ExecutionModeFirstTime
	h.runExecution(c, &req)
}

// Rebalance godoc
// POST /api/v1/portfolio/rebalance
//
// Rebalances an existing portfolio using explicit sell/buy/rebalance instructions.
// Body: models.ExecutionRequest with mode = "rebalance"
func (h *PortfolioHandler) Rebalance(c *gin.Context) {
	var req models.ExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}
	req.Mode = models.ExecutionModeRebalance
	h.runExecution(c, &req)
}

// GetHoldings godoc
// GET /api/v1/holdings?session_id=<id>
//
// Fetches the authenticated user's current equity holdings from their broker.
func (h *PortfolioHandler) GetHoldings(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		sessionID = c.GetHeader("X-Session-ID")
	}
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "session_id query param or X-Session-ID header required",
		})
		return
	}

	sess, ok := h.sessionMgr.Get(sessionID)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "session not found or expired",
		})
		return
	}

	adapter, err := broker.NewAdapter(sess.Broker, h.logger)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	holdings, err := adapter.GetHoldings(c.Request.Context(), sess)
	if err != nil {
		h.logger.Error("get holdings failed", zap.String("broker", sess.Broker), zap.Error(err))
		c.JSON(http.StatusBadGateway, models.APIResponse{
			Success: false,
			Error:   "failed to fetch holdings: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    holdings,
	})
}

// ─── internal helpers ─────────────────────────────────────────────────────────

func (h *PortfolioHandler) runExecution(c *gin.Context, req *models.ExecutionRequest) {
	if req.SessionID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "session_id is required",
		})
		return
	}

	result, err := h.executor.Execute(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("execution failed",
			zap.String("broker", req.Broker),
			zap.String("mode", string(req.Mode)),
			zap.Error(err),
		)
		status := http.StatusInternalServerError
		if err.Error() == "session not found or expired: "+req.SessionID {
			status = http.StatusUnauthorized
		}
		c.JSON(status, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "execution completed",
		Data:    result,
	})
}
