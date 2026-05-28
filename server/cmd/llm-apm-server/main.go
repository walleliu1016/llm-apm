package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/akke/llm-apm/server/internal/config"
	"github.com/akke/llm-apm/server/internal/greptimedb"
	"github.com/akke/llm-apm/server/internal/handler"
	"github.com/akke/llm-apm/server/internal/transcript"
	"github.com/akke/llm-apm/server/internal/turn"
)

func main() {
	// Load config
	cfg := config.Load()

	// Setup logger
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	logger.Info("starting llm-apm-server",
		"port", cfg.Port,
		"data_dir", cfg.DataDir,
		"greptime_host", cfg.GreptimeDBHost,
		"greptime_embedded", cfg.GreptimeEmbedded)

	// Start GreptimeDB (only if embedded mode is enabled)
	var greptime *greptimedb.Process
	if cfg.GreptimeEmbedded {
		greptime = greptimedb.NewProcess(cfg.DataDir,
			cfg.GreptimeHTTPPort, cfg.GreptimeGRPCPort, cfg.GreptimeMySQLPort, logger)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := greptime.Start(ctx); err != nil {
			logger.Error("failed to start GreptimeDB", "error", err)
			os.Exit(1)
		}
	}

	// Init tables (use configured host)
	if err := greptimedb.InitTablesAt(cfg.GreptimeDBHost, cfg.GreptimeHTTPPort); err != nil {
		logger.Error("failed to init tables", "error", err)
		os.Exit(1)
	}

	// Create handler server (use configured host)
	srv := handler.NewServer(cfg.GreptimeDBHost, cfg.GreptimeHTTPPort, logger)

	// Create turn tracker (use configured host)
	turnTracker := turn.NewTrackerWithDB(cfg.GreptimeDBHost, cfg.GreptimeHTTPPort, logger)
	srv.SetTurnTracker(turnTracker)

	// Create transcript watcher (use configured host)
	watcher := transcript.NewWatcher(cfg.GreptimeDBHost, cfg.GreptimeHTTPPort, logger)
	srv.SetTranscriptWatcher(watcher)

	// Setup HTTP routes
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Start HTTP server
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: mux,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	logger.Info("server started", "addr", httpSrv.Addr)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	httpSrv.Shutdown(shutdownCtx)
	if greptime != nil {
		greptime.Stop()
	}
	watcher.StopAll()

	logger.Info("server stopped")
}