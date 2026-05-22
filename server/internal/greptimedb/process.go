package greptimedb

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Process manages GreptimeDB lifecycle.
type Process struct {
	dataDir    string
	httpPort   int
	grpcPort   int
	mysqlPort  int
	binaryPath string
	configPath string
	cmd        *exec.Cmd
	logger     *slog.Logger
	mu         sync.Mutex
}

// NewProcess creates a GreptimeDB process manager.
func NewProcess(dataDir string, httpPort, grpcPort, mysqlPort int, logger *slog.Logger) *Process {
	return &Process{
		dataDir:    dataDir,
		httpPort:   httpPort,
		grpcPort:   grpcPort,
		mysqlPort:  mysqlPort,
		binaryPath: filepath.Join(dataDir, "bin", "greptime"),
		configPath: filepath.Join(dataDir, "config", "standalone.toml"),
		logger:     logger,
	}
}

// Start launches GreptimeDB process.
func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Ensure data directory exists
	if err := os.MkdirAll(p.dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Write default config
	configDir := filepath.Join(p.dataDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	configContent := strings.ReplaceAll(DefaultConfig(), "{{DATA_DIR}}",
		filepath.Join(p.dataDir, "data"))
	if err := os.WriteFile(p.configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Check if binary exists, download if not (simplified: assume exists or skip)
	if _, err := os.Stat(p.binaryPath); os.IsNotExist(err) {
		p.logger.Warn("GreptimeDB binary not found, skipping start for now")
		return nil // In real impl, would download binary
	}

	// Start process
	p.cmd = exec.CommandContext(ctx, p.binaryPath, "standalone", "start",
		"-c", p.configPath)
	p.cmd.Stdout = io.Discard
	p.cmd.Stderr = io.Discard

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	p.logger.Info("GreptimeDB started", "http_port", p.httpPort)

	// Wait for healthy
	for i := 0; i < 30; i++ {
		if p.IsHealthy() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("GreptimeDB not healthy after 15s")
}

// IsHealthy checks if GreptimeDB is responding.
func (p *Process) IsHealthy() bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", p.httpPort)
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// Stop terminates GreptimeDB process.
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM: %w", err)
	}

	// Wait for graceful shutdown
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()

	select {
	case err := <-done:
		p.logger.Info("GreptimeDB stopped")
		return err
	case <-time.After(5 * time.Second):
		p.cmd.Process.Kill()
		return fmt.Errorf("force killed after timeout")
	}
}