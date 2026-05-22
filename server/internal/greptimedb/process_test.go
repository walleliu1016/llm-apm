package greptimedb

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessStart(t *testing.T) {
	// Skip if GreptimeDB binary not installed
	binaryPath := filepath.Join("/tmp/test-apm", "bin", "greptime")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("GreptimeDB binary not installed, skipping process test")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	p := NewProcess("/tmp/test-apm", 14000, 14001, 14002, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Check health
	if !p.IsHealthy() {
		t.Error("process not healthy after start")
	}

	// Stop
	if err := p.Stop(); err != nil {
		t.Fatalf("failed to stop: %v", err)
	}
}