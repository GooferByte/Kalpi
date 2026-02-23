// Package api wires all HTTP handlers into a configured Gin engine.
package api

import (
	"net/http"

	"github.com/GooferByte/kalpi/internal/api/handlers"
	"github.com/GooferByte/kalpi/internal/api/middleware"
	"github.com/GooferByte/kalpi/internal/engine"
	"github.com/GooferByte/kalpi/internal/models"
	"github.com/GooferByte/kalpi/internal/notification"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/GooferByte/kalpi/pkg/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// NewRouter builds and returns the Gin engine with all routes registered.
// Dependency injection: all handlers receive their dependencies here — no globals.
func NewRouter(
	cfg *config.Config,
	logger *zap.Logger,
	sessionMgr session.Manager,
	exec engine.Executor,
	wsHub *notification.Hub,
) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recovery(logger))

	// ── Health check ──────────────────────────────────────────────────────
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, models.APIResponse{Success: true, Message: "ok"})
	})

	// ── WebSocket ─────────────────────────────────────────────────────────
	wsHandler := handlers.NewWebSocketHandler(wsHub, logger)
	r.GET("/ws/notifications", wsHandler.Handle)

	// ── REST API v1 ───────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(sessionMgr, logger)
	portfolioHandler := handlers.NewPortfolioHandler(exec, sessionMgr, logger)
	ordersHandler := handlers.NewOrdersHandler(exec, logger)

	v1 := r.Group("/api/v1")
	{
		// Broker auth
		v1.POST("/auth/:broker", authHandler.Authenticate)
		v1.DELETE("/auth/session/:session_id", authHandler.Logout)
		v1.GET("/brokers", authHandler.ListBrokers)

		// Holdings
		v1.GET("/holdings", portfolioHandler.GetHoldings)

		// Portfolio execution
		v1.POST("/portfolio/execute", portfolioHandler.Execute)
		v1.POST("/portfolio/rebalance", portfolioHandler.Rebalance)

		// Execution history
		v1.GET("/orders", ordersHandler.ListExecutions)
		v1.GET("/orders/:exec_id", ordersHandler.GetExecution)
	}

	return r
}
