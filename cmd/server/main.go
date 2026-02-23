package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GooferByte/kalpi/internal/api"
	"github.com/GooferByte/kalpi/internal/engine"
	"github.com/GooferByte/kalpi/internal/notification"
	"github.com/GooferByte/kalpi/internal/session"
	"github.com/GooferByte/kalpi/pkg/config"
	"go.uber.org/zap"
)

func main() {
	// ── Config ────────────────────────────────────────────────────────────
	cfg := config.Load()

	// ── Logger ────────────────────────────────────────────────────────────
	var logger *zap.Logger
	var err error
	if cfg.Env == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	logger.Info("starting Kalpi Trade Execution Engine",
		zap.String("env", cfg.Env),
		zap.Int("port", cfg.Port),
	)

	// ── Session store ─────────────────────────────────────────────────────
	sessionMgr := session.NewInMemoryManager(cfg.SessionTTLHours)

	// ── WebSocket hub ─────────────────────────────────────────────────────
	wsHub := notification.NewWebSocketHub(logger)
	go wsHub.Run()

	// ── Notification chain: always log + broadcast over WS ────────────────
	notifier := notification.NewCompositeNotifier(
		notification.NewLogNotifier(logger),
		notification.NewWebSocketNotifier(wsHub, logger),
	)

	// ── Execution engine ──────────────────────────────────────────────────
	execStore := engine.NewInMemoryStore()
	orderMgr := engine.NewOrderManager(logger, 3) // 3 retries with exponential backoff
	exec := engine.NewExecutor(sessionMgr, execStore, orderMgr, notifier, logger)

	// ── HTTP server ───────────────────────────────────────────────────────
	router := api.NewRouter(cfg, logger, sessionMgr, exec, wsHub)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start serving in a goroutine so the main goroutine can wait for signals.
	go func() {
		logger.Info("server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received, draining connections...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	} else {
		logger.Info("server stopped cleanly")
	}
}
