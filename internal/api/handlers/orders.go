package handlers

import (
	"net/http"

	"github.com/GooferByte/kalpi/internal/engine"
	"github.com/GooferByte/kalpi/internal/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OrdersHandler handles order status and execution history endpoints.
type OrdersHandler struct {
	executor engine.Executor
	logger   *zap.Logger
}

// NewOrdersHandler creates a new OrdersHandler.
func NewOrdersHandler(exec engine.Executor, logger *zap.Logger) *OrdersHandler {
	return &OrdersHandler{executor: exec, logger: logger}
}

// GetExecution godoc
// GET /api/v1/orders/:exec_id
//
// Returns the full result of a portfolio execution run.
func (h *OrdersHandler) GetExecution(c *gin.Context) {
	execID := c.Param("exec_id")

	result, ok := h.executor.GetExecution(execID)
	if !ok {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "execution not found: " + execID,
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    result,
	})
}

// ListExecutions godoc
// GET /api/v1/orders
//
// Returns all execution results stored in memory.
func (h *OrdersHandler) ListExecutions(c *gin.Context) {
	results := h.executor.ListExecutions()
	h.logger.Debug("list executions", zap.Int("count", len(results)))
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    results,
	})
}
