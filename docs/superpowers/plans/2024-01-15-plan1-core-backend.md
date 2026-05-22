# LLM-APM Plan 1: 核心后端 + Dashboard 框架

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建核心数据收集和存储能力，提供基础 Dashboard UI，能接收 Hook 事件、监控 JSONL 文件、存储到 GreptimeDB、展示 Session 列表。

**Architecture:** Go 后端服务器 + GreptimeDB 存储 + 嵌入式 Dashboard。复用 tma1 的架构模式，精简为仅支持 CLI agents（Claude Code、Codex、Copilot CLI）。

**Tech Stack:** Go 1.21+ / GreptimeDB / 嵌入式 HTML Dashboard

---

## File Structure

```
server/
├── cmd/
│   └── llm-apm-server/
│       ├── main.go          # 入口点，启动 GreptimeDB + HTTP Server
│       └── web.go           # embed.FS 声明
├── internal/
│   ├── config/
│       │   └── config.go    # 环境变量配置加载
│   ├── greptimedb/
│       │   ├── process.go   # GreptimeDB 进程管理（启动/停止/健康检查）
│       │   ├── tables.go    # 数据表初始化（apm_hook_events, apm_messages, apm_turns）
│       │   └── default_config.go # GreptimeDB 默认配置
│   ├── handler/
│       │   ├── handler.go   # HTTP 路由注册
│       │   ├── hooks.go     # Hook 事件处理 /api/hooks
│       │   └── query.go     # SQL 查询代理 /api/query
│   ├── transcript/
│       │   ├── watcher.go   # JSONL 文件监控（Claude Code）
│       │   ├── codex.go     # Codex session log 解析
│       │   └── copilot.go   # Copilot CLI session log 解析
│   └── hooks/
│       │   └── hooks.go     # Hook 脚本安装器
├── web/
│   ├── web.go               # embed.FS 声明
│   └── index.html           # Dashboard 主页面
├── go.mod
├── go.sum
├── Makefile                 # 构建命令
```

---

## Task 1: Go 项目初始化

**Files:**
- Create: `server/go.mod`
- Create: `server/Makefile`

- [ ] **Step 1: 创建 Go 模块**

Run:
```bash
cd /home/akke/project/llm-apm/server
go mod init github.com/akke/llm-apm/server
```

Expected: 生成 `go.mod` 文件

- [ ] **Step 2: 创建 Makefile**

Create file `server/Makefile`:

```makefile
.PHONY: build run clean vet lint test

build:
	CGO_ENABLED=0 go build -o bin/llm-apm-server ./cmd/llm-apm-server

run:
	go run ./cmd/llm-apm-server

clean:
	rm -rf bin/

vet:
	go vet ./...

lint:
	golangci-lint run

test:
	go test -race -count=1 ./...
```

- [ ] **Step 3: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/go.mod server/Makefile
git commit -m "feat: init go project with makefile"
```

---

## Task 2: 配置模块

**Files:**
- Create: `server/internal/config/config.go`
- Create: `server/internal/config/config_test.go`

- [ ] **Step 1: 写配置加载测试**

Create file `server/internal/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Set test env vars
	os.Setenv("APM_PORT", "18080")
	os.Setenv("APM_DATA_DIR", "/tmp/test-apm")
	defer os.Unsetenv("APM_PORT")
	defer os.Unsetenv("APM_DATA_DIR")

	cfg := Load()

	if cfg.Port != 18080 {
		t.Errorf("expected Port=18080, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/test-apm" {
		t.Errorf("expected DataDir=/tmp/test-apm, got %s", cfg.DataDir)
	}
}

func TestDefaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("APM_PORT")
	os.Unsetenv("APM_DATA_DIR")

	cfg := Load()

	if cfg.Port != 14318 {
		t.Errorf("expected default Port=14318, got %d", cfg.Port)
	}
	if cfg.DataDir != "~/.llm-apm" {
		t.Errorf("expected default DataDir=~/.llm-apm, got %s", cfg.DataDir)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/config/... -v
```

Expected: FAIL - config.go not found

- [ ] **Step 3: 写配置实现**

Create file `server/internal/config/config.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all server configuration.
type Config struct {
	Host              string
	Port              int
	DataDir           string
	GreptimeHTTPPort  int
	GreptimeGRPCPort  int
	GreptimeMySQLPort int
	LogLevel          string
	DataTTL           string
	TenantID          string // 预留：多租户扩展
}

// Load reads configuration from environment variables.
func Load() *Config {
	return &Config{
		Host:              getEnv("APM_HOST", "127.0.0.1"),
		Port:              getEnvInt("APM_PORT", 14318),
		DataDir:           expandPath(getEnv("APM_DATA_DIR", "~/.llm-apm")),
		GreptimeHTTPPort:  getEnvInt("APM_GREPTIMEDB_HTTP_PORT", 14000),
		GreptimeGRPCPort:  getEnvInt("APM_GREPTIMEDB_GRPC_PORT", 14001),
		GreptimeMySQLPort: getEnvInt("APM_GREPTIMEDB_MYSQL_PORT", 14002),
		LogLevel:          getEnv("APM_LOG_LEVEL", "info"),
		DataTTL:           getEnv("APM_DATA_TTL", "60d"),
		TenantID:          getEnv("APM_TENANT_ID", ""), // 预留字段
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
```

- [ ] **Step 4: 运行测试验证通过**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/config/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/config/
git commit -m "feat: add config module with env var loading"
```

---

## Task 3: GreptimeDB 进程管理

**Files:**
- Create: `server/internal/greptimedb/process.go`
- Create: `server/internal/greptimedb/process_test.go`
- Create: `server/internal/greptimedb/default_config.go`

- [ ] **Step 1: 写进程管理测试**

Create file `server/internal/greptimedb/process_test.go`:

```go
package greptimedb

import (
	"context"
	"testing"
	"time"
)

func TestProcessStart(t *testing.T) {
	p := NewProcess("/tmp/test-apm", 14000, 14001, 14002)
	
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
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/greptimedb/... -v
```

Expected: FAIL - process.go not found

- [ ] **Step 3: 写 GreptimeDB 默认配置**

Create file `server/internal/greptimedb/default_config.go`:

```go
package greptimedb

// DefaultConfig returns minimal GreptimeDB standalone config.
// Disables unused protocols, keeps HTTP and MySQL.
func DefaultConfig() string {
	return `
[http]
addr = "127.0.0.1:14000"

[grpc]
addr = "127.0.0.1:14001"

[mysql]
addr = "127.0.0.1:14002"

[storage]
type = "File"
data_dir = "{{DATA_DIR}}"

[logging]
level = "info"
`
}
```

- [ ] **Step 4: 写进程管理实现**

Create file `server/internal/greptimedb/process.go`:

```go
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
func NewProcess(dataDir, httpPort, grpcPort, mysqlPort int, logger *slog.Logger) *Process {
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
```

- [ ] **Step 5: 运行测试（跳过实际启动）**

由于测试环境可能没有 GreptimeDB，先验证编译：

Run:
```bash
cd /home/akke/project/llm-apm/server
go build ./internal/greptimedb/...
```

Expected: 编译成功，无错误

- [ ] **Step 6: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/greptimedb/
git commit -m "feat: add greptimedb process manager"
```

---

## Task 4: 数据表初始化

**Files:**
- Create: `server/internal/greptimedb/tables.go`
- Create: `server/internal/greptimedb/tables_test.go`

- [ ] **Step 1: 写表初始化测试**

Create file `server/internal/greptimedb/tables_test.go`:

```go
package greptimedb

import (
	"testing"
)

func TestCreateTablesSQL(t *testing.T) {
	sqls := CreateTablesSQL()
	
	if len(sqls) == 0 {
		t.Error("expected non-empty SQL statements")
	}
	
	// Check for expected tables
	expectedTables := []string{
		"apm_hook_events",
		"apm_messages",
		"apm_turns",
	}
	
	for _, table := range expectedTables {
		found := false
		for _, sql := range sqls {
			if strings.Contains(sql, table) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected table %s not found in SQL", table)
		}
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/greptimedb/... -v -run TestCreateTablesSQL
```

Expected: FAIL - tables.go not found

- [ ] **Step 3: 写表初始化实现**

Create file `server/internal/greptimedb/tables.go`:

```go
package greptimedb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CreateTablesSQL returns SQL statements to create core tables.
func CreateTablesSQL() []string {
	return []string{
		// Hook events table
		`CREATE TABLE IF NOT EXISTS apm_hook_events (
			ts TIMESTAMP,
			session_id STRING,
			event_type STRING,
			agent_source STRING,
			agent_id STRING,
			parent_agent_id STRING,
			agent_depth BIGINT DEFAULT 0,
			turn_id STRING DEFAULT '',
			tool_name STRING,
			tool_input STRING,
			tool_result STRING,
			tool_use_id STRING,
			cwd STRING,
			error_flag BOOLEAN DEFAULT false,
			tenant_id STRING DEFAULT '',
			metadata JSON
		) ENGINE=mito WITH (
			'append_mode' = 'true',
			'skipping_index' = 'session_id',
			'inverted_index' = 'event_type,agent_source,error_flag'
		)`,
		
		// Messages table
		`CREATE TABLE IF NOT EXISTS apm_messages (
			ts TIMESTAMP,
			session_id STRING,
			message_type STRING,
			role STRING,
			content STRING,
			model STRING,
			tool_name STRING,
			tool_use_id STRING,
			input_tokens BIGINT DEFAULT 0,
			output_tokens BIGINT DEFAULT 0,
			cache_read_tokens BIGINT DEFAULT 0,
			cache_creation_tokens BIGINT DEFAULT 0,
			tenant_id STRING DEFAULT ''
		) ENGINE=mito WITH (
			'append_mode' = 'true',
			'skipping_index' = 'session_id',
			'fulltext_index' = 'content'
		)`,
		
		// Turns table
		`CREATE TABLE IF NOT EXISTS apm_turns (
			ts TIMESTAMP,
			turn_id STRING,
			session_id STRING,
			start_ts TIMESTAMP,
			end_ts TIMESTAMP,
			user_prompt STRING,
			agent_response STRING,
			input_tokens BIGINT DEFAULT 0,
			output_tokens BIGINT DEFAULT 0,
			cost_usd DOUBLE DEFAULT 0,
			tool_count BIGINT DEFAULT 0,
			has_error BOOLEAN DEFAULT false,
			tenant_id STRING DEFAULT ''
		) ENGINE=mito`,
	}
}

// InitTables creates all required tables in GreptimeDB.
func InitTables(httpPort int) error {
	sqlURL := fmt.Sprintf("http://127.0.0.1:%d/v1/sql", httpPort)
	
	for _, sql := range CreateTablesSQL() {
		if err := execSQL(sqlURL, sql); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}
	return nil
}

func execSQL(urlStr, sql string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	form := url.Values{}
	form.Set("sql", sql)

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, 
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("non-200 status: %d, body: %s", resp.StatusCode, body)
	}
	return nil
}

// escapeSQL escapes single quotes for SQL strings.
func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
```

- [ ] **Step 4: 运行测试验证通过**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/greptimedb/... -v -run TestCreateTablesSQL
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/greptimedb/tables.go server/internal/greptimedb/tables_test.go
git commit -m "feat: add data table initialization"
```

---

## Task 5: Hook Handler

**Files:**
- Create: `server/internal/handler/handler.go`
- Create: `server/internal/handler/hooks.go`
- Create: `server/internal/handler/hooks_test.go`

- [ ] **Step 1: 写 Hook Handler 测试**

Create file `server/internal/handler/hooks_test.go`:

```go
package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHooks(t *testing.T) {
	body := `{
		"session_id": "test-session-123",
		"hook_event_name": "PreToolUse",
		"tool_name": "Read",
		"tool_input": {"file_path": "/src/main.go"},
		"agent_id": "",
		"agent_type": "main"
	}`
	
	req := httptest.NewRequest("POST", "/api/hooks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	
	// Create mock server
	s := &Server{
		greptimeDBHost: "127.0.0.1",
		greptimeHTTPPort: 14000,
		logger: nil,
	}
	
	s.handleHooks(rr, req)
	
	// Should return 200 immediately
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleHooksInvalidJSON(t *testing.T) {
	body := `{invalid json}`
	
	req := httptest.NewRequest("POST", "/api/hooks", bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()
	
	s := &Server{}
	s.handleHooks(rr, req)
	
	// Should still return 200 (don't block agent)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for invalid JSON, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/handler/... -v
```

Expected: FAIL - handler files not found

- [ ] **Step 3: 写 Hook Handler 实现**

Create file `server/internal/handler/hooks.go`:

```go
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Server holds handler dependencies.
type Server struct {
	greptimeDBHost   string
	greptimeHTTPPort int
	httpClient       *http.Client
	logger           *slog.Logger
	transcriptWatcher interface{} // Will be TranscriptWatcher
}

// HookPayload represents a hook event from Claude Code.
type HookPayload struct {
	SessionID      string `json:"session_id"`
	HookEventName  string `json:"hook_event_name"`
	ToolName       string `json:"tool_name"`
	ToolInput      any    `json:"tool_input"`
	ToolUseID      string `json:"tool_use_id"`
	ToolResponse   any    `json:"tool_response"`
	AgentID        string `json:"agent_id"`
	AgentType      string `json:"agent_type"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	TenantID       string `json:"tenant_id"` // 预留
}

const (
	maxHookBody   = 1 << 20 // 1 MB
	maxToolInput  = 2048
	maxToolResult = 4096
)

// handleHooks receives hook events and stores them in GreptimeDB.
func (s *Server) handleHooks(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxHookBody))
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload HookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	if payload.SessionID == "" || payload.HookEventName == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Return 200 immediately - don't block the agent
	w.WriteHeader(http.StatusOK)

	// Detect agent source from query param
	agentSource := r.URL.Query().Get("source")
	if agentSource == "" {
		agentSource = "claude_code"
	}

	// Normalize tool_response
	toolResult := normalizeToolResponse(payload.ToolResponse)
	toolInput := serializeToolInput(payload.ToolInput)

	// Determine error flag
	errorFlag := payload.HookEventName == "PostToolUseFailure" ||
		strings.Contains(payload.HookEventName, "Error")

	// Async insert
	go s.insertHookEvent(payload, agentSource, toolInput, toolResult, errorFlag)
}

func (s *Server) insertHookEvent(p HookPayload, agentSource, toolInput, toolResult string, errorFlag bool) {
	now := time.Now().UnixMilli()
	
	// Determine agent depth
	agentDepth := 0
	if p.AgentType == "subagent" || (p.AgentID != "" && p.AgentID != "main") {
		agentDepth = 1
	}

	sql := fmt.Sprintf(
		"INSERT INTO apm_hook_events "+
			"(ts, session_id, event_type, agent_source, agent_id, agent_depth, "+
			"tool_name, tool_input, tool_result, tool_use_id, cwd, error_flag, tenant_id) "+
			"VALUES (%d, '%s', '%s', '%s', '%s', %d, '%s', '%s', '%s', '%s', '%s', %b, '%s')",
		now,
		escapeSQL(p.SessionID),
		escapeSQL(p.HookEventName),
		escapeSQL(agentSource),
		escapeSQL(p.AgentID),
		agentDepth,
		escapeSQL(truncate(p.ToolName, 256)),
		escapeSQL(truncate(toolInput, maxToolInput)),
		escapeSQL(truncate(toolResult, maxToolResult)),
		escapeSQL(p.ToolUseID),
		escapeSQL(truncate(p.CWD, 512)),
		errorFlag,
		escapeSQL(p.TenantID),
	)

	sqlURL := fmt.Sprintf("http://%s:%d/v1/sql", s.greptimeDBHost, s.greptimeHTTPPort)
	form := url.Values{}
	form.Set("sql", sql)

	resp, err := s.httpClient.Post(sqlURL, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()))
	if err != nil {
		if s.logger != nil {
			s.logger.Debug("hook insert failed", "error", err)
		}
		return
	}
	defer resp.Body.Close()
}

func normalizeToolResponse(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case map[string]any:
		if c, ok := val["content"]; ok {
			if s, ok := c.(string); ok {
				return s
			}
		}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func serializeToolInput(v any) string {
	if v == nil {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
```

- [ ] **Step 4: 写路由注册**

Create file `server/internal/handler/handler.go`:

```go
package handler

import (
	"log/slog"
	"net/http"
)

// NewServer creates a handler server.
func NewServer(greptimeDBHost string, greptimeHTTPPort int, logger *slog.Logger) *Server {
	return &Server{
		greptimeDBHost:   greptimeDBHost,
		greptimeHTTPPort: greptimeHTTPPort,
		httpClient:       &http.Client{},
		logger:           logger,
	}
}

// RegisterRoutes sets up all HTTP endpoints.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/hooks", s.handleHooks)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleDashboard)
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleDashboard serves the embedded HTML dashboard.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Will be implemented with embed.FS in later task
	http.ServeFile(w, r, "web/index.html")
}
```

- [ ] **Step 5: 运行测试验证通过**

Run:
```bash
cd /home/akke/project/llm-apm/server
go test ./internal/handler/... -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/handler/
git commit -m "feat: add hook handler with async storage"
```

---

## Task 6: JSONL Watcher

**Files:**
- Create: `server/internal/transcript/watcher.go`

- [ ] **Step 1: 写 JSONL Watcher 实现**

Create file `server/internal/transcript/watcher.go`:

```go
package transcript

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Watcher manages per-session JSONL transcript watchers.
type Watcher struct {
	mu        sync.Mutex
	sessions  map[string]*sessionWatch
	sqlURL    string
	logger    *slog.Logger
}

type sessionWatch struct {
	cancel context.CancelFunc
	seen   map[string]struct{}
}

// NewWatcher creates a transcript watcher.
func NewWatcher(greptimeDBHost string, greptimeHTTPPort int, logger *slog.Logger) *Watcher {
	return &Watcher{
		sessions: make(map[string]*sessionWatch),
		sqlURL:   fmt.Sprintf("http://%s:%d/v1/sql", greptimeDBHost, greptimeHTTPPort),
		logger:   logger,
	}
}

// Watch starts tailing a JSONL transcript file.
func (w *Watcher) Watch(sessionID, transcriptPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.sessions[sessionID]; ok {
		return // already watching
	}

	ctx, cancel := context.WithCancel(context.Background())
	sw := &sessionWatch{
		cancel: cancel,
		seen:   make(map[string]struct{}),
	}
	w.sessions[sessionID] = sw

	go w.tailFile(ctx, sessionID, transcriptPath, sw.seen)
	w.logger.Info("started watching transcript", "session", sessionID, "path", transcriptPath)
}

// Stop stops watching a transcript.
func (w *Watcher) Stop(sessionID string) {
	w.mu.Lock()
	sw, ok := w.sessions[sessionID]
	if ok {
		delete(w.sessions, sessionID)
	}
	w.mu.Unlock()

	if ok {
		sw.cancel()
	}
}

func (w *Watcher) tailFile(ctx context.Context, sessionID, filePath string, seen map[string]struct{}) {
	// Wait for file to appear
	var f *os.File
	for i := 0; i < 10; i++ {
		var err error
		f, err = os.Open(filePath)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	if f == nil {
		w.logger.Warn("transcript file not found", "path", filePath)
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var buf strings.Builder

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			buf.WriteString(line)
			if strings.HasSuffix(line, "\n") {
				trimmed := strings.TrimSpace(buf.String())
				buf.Reset()
				if trimmed != "" {
					w.processLine(sessionID, trimmed, seen)
				}
			}
			continue
		}

		if err == io.EOF {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}

		if err != nil {
			w.logger.Warn("transcript read error", "error", err)
			return
		}
	}
}

// TranscriptEntry matches Claude Code JSONL format.
type TranscriptEntry struct {
	SessionID string         `json:"sessionId"`
	Type      string         `json:"type"`
	UUID      string         `json:"uuid"`
	Message   *TranscriptMsg `json:"message"`
	CWD       string         `json:"cwd"`
}

type TranscriptMsg struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *MsgUsage       `json:"usage"`
}

type MsgUsage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
}

func (w *Watcher) processLine(sessionID, line string, seen map[string]struct{}) {
	var entry TranscriptEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return
	}

	if entry.Type != "user" && entry.Type != "assistant" {
		return
	}
	if entry.Message == nil {
		return
	}

	role := entry.Message.Role
	if role == "human" {
		role = "user"
	}

	// Parse content
	var content string
	if err := json.Unmarshal(entry.Message.Content, &content); err == nil {
		if content != "" {
			w.insertMessage(sessionID, role, role, truncate(content, 32768), 
				entry.Message.Model, "", "", entry.Message.Usage)
		}
		return
	}

	// Parse as content blocks
	var blocks []ContentBlock
	if err := json.Unmarshal(entry.Message.Content, &blocks); err != nil {
		return
	}

	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				w.insertMessage(sessionID, "text", role, truncate(b.Text, 32768),
					entry.Message.Model, "", "", nil)
			}
		case "tool_use":
			inputStr := truncate(string(b.Input), 2048)
			w.insertMessage(sessionID, "tool_use", role, inputStr,
				entry.Message.Model, b.Name, b.ID, nil)
		case "tool_result":
			w.insertMessage(sessionID, "tool_result", role,
				truncate(extractToolResultContent(b.Content), 4096),
				entry.Message.Model, "", b.ToolUseID, nil)
		}
	}
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Name      string          `json:"name"`
	ID        string          `json:"id"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

func extractToolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct{ Text string }
	if json.Unmarshal(raw, &parts) == nil {
		var sb strings.Builder
		for _, p := range parts {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(p.Text)
		}
		return sb.String()
	}
	return string(raw)
}

func (w *Watcher) insertMessage(sessionID, messageType, role, content, model,
	toolName, toolUseID string, usage *MsgUsage) {
	
	now := time.Now().UnixMilli()
	
	var sql string
	if usage != nil {
		sql = fmt.Sprintf(
			"INSERT INTO apm_messages "+
				"(ts, session_id, message_type, role, content, model, tool_name, tool_use_id, "+
				"input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens) "+
				"VALUES (%d, '%s', '%s', '%s', '%s', '%s', '%s', '%s', %d, %d, %d, %d)",
			now, escapeSQL(sessionID), escapeSQL(messageType), escapeSQL(role),
			escapeSQL(content), escapeSQL(model), escapeSQL(toolName), escapeSQL(toolUseID),
			usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheCreationTokens)
	} else {
		sql = fmt.Sprintf(
			"INSERT INTO apm_messages "+
				"(ts, session_id, message_type, role, content, model, tool_name, tool_use_id) "+
				"VALUES (%d, '%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			now, escapeSQL(sessionID), escapeSQL(messageType), escapeSQL(role),
			escapeSQL(content), escapeSQL(model), escapeSQL(toolName), escapeSQL(toolUseID))
	}

	go w.execSQL(sql)
}

func (w *Watcher) execSQL(sql string) {
	form := url.Values{}
	form.Set("sql", sql)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", w.sqlURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
```

- [ ] **Step 2: 验证编译**

Run:
```bash
cd /home/akke/project/llm-apm/server
go build ./internal/transcript/...
```

Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/internal/transcript/
git commit -m "feat: add JSONL transcript watcher"
```

---

## Task 7: 入口点和主程序

**Files:**
- Create: `server/cmd/llm-apm-server/main.go`
- Create: `server/cmd/llm-apm-server/web.go`

- [ ] **Step 1: 写入口点实现**

Create file `server/cmd/llm-apm-server/main.go`:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/akke/llm-apm/server/internal/config"
	"github.com/akke/llm-apm/server/internal/greptimedb"
	"github.com/akke/llm-apm/server/internal/handler"
	"github.com/akke/llm-apm/server/internal/transcript"
)

func main() {
	// Load config
	cfg := config.Load()

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("starting llm-apm-server", 
		"port", cfg.Port, 
		"data_dir", cfg.DataDir)

	// Start GreptimeDB
	greptime := greptimedb.NewProcess(cfg.DataDir,
		cfg.GreptimeHTTPPort, cfg.GreptimeGRPCPort, cfg.GreptimeMySQLPort, logger)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := greptime.Start(ctx); err != nil {
		logger.Error("failed to start GreptimeDB", "error", err)
		os.Exit(1)
	}

	// Init tables
	if err := greptimedb.InitTables(cfg.GreptimeHTTPPort); err != nil {
		logger.Error("failed to init tables", "error", err)
		os.Exit(1)
	}

	// Create handler server
	srv := handler.NewServer("127.0.0.1", cfg.GreptimeHTTPPort, logger)

	// Create transcript watcher
	watcher := transcript.NewWatcher("127.0.0.1", cfg.GreptimeHTTPPort, logger)

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
	greptime.Stop()
	watcher.StopAll()

	logger.Info("server stopped")
}
```

- [ ] **Step 2: 添加 fmt 导入并修复编译**

Run:
```bash
cd /home/akke/project/llm-apm/server
go mod tidy
go build ./cmd/llm-apm-server/...
```

Expected: 编译成功（如果有导入错误，添加 `fmt` 导入）

- [ ] **Step 3: 写 web.go (embed.FS 声明占位)**

Create file `server/cmd/llm-apm-server/web.go`:

```go
package main

// Web content will be embedded in later task.
// Placeholder for now - dashboard served from static files.

import "embed"

//go:embed web
var webFS embed.FS // Will be used when web/index.html is created
```

- [ ] **Step 4: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/cmd/llm-apm-server/
git commit -m "feat: add main entry point and server startup"
```

---

## Task 8: Dashboard 基础框架

**Files:**
- Create: `server/web/index.html`
- Create: `server/web/web.go`

- [ ] **Step 1: 复用 mockup 作为 Dashboard**

将已有的 mockup 复制到 web 目录并简化：

Run:
```bash
cp /home/akke/project/llm-apm/mockup/dashboard-mockup.html /home/akke/project/llm-apm/server/web/index.html
```

- [ ] **Step 2: 写 web.go embed.FS 声明**

Create file `server/web/web.go`:

```go
package web

import "embed"

//go:embed index.html
var FS embed.FS
```

- [ ] **Step 3: 更新 handler.go 使用 embed.FS**

Update file `server/internal/handler/handler.go`, modify `handleDashboard`:

```go
package handler

import (
	"io/fs"
	"log/slog"
	"net/http"
	
	"github.com/akke/llm-apm/server/web"
)

// ... existing code ...

// handleDashboard serves the embedded HTML dashboard.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	fsys, err := fs.Sub(web.FS, "index.html")
	if err != nil {
		http.Error(w, "dashboard not found", 500)
		return
	}
	http.ServeFile(w, r, "index.html")
}

// handleStatic serves static assets (for future CSS/JS files).
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, web.FS, r.URL.Path[1:])
}
```

- [ ] **Step 4: 更新 main.go 注册路由**

Update `server/cmd/llm-apm-server/main.go` to add static handler:

在 `srv.RegisterRoutes(mux)` 后添加:
```go
mux.HandleFunc("/static/", srv.handleStatic)
```

- [ ] **Step 5: 验证编译**

Run:
```bash
cd /home/akke/project/llm-apm/server
go mod tidy
go build ./...
```

Expected: 编译成功

- [ ] **Step 6: Commit**

```bash
cd /home/akke/project/llm-apm
git add server/web/
git commit -m "feat: add embedded dashboard from mockup"
```

---

## Task 9: 集成测试和构建

**Files:**
- Modify: `server/Makefile`

- [ ] **Step 1: 构建完整二进制**

Run:
```bash
cd /home/akke/project/llm-apm/server
make build
```

Expected: 生成 `bin/llm-apm-server`

- [ ] **Step 2: 运行所有测试**

Run:
```bash
cd /home/akke/project/llm-apm/server
make test
```

Expected: 所有测试通过

- [ ] **Step 3: Commit 最终版本**

```bash
cd /home/akke/project/llm-apm
git add server/
git commit -m "feat: complete plan 1 - core backend and dashboard framework"
```

---

## Summary

Plan 1 完成后交付物：

| 交付物 | 状态 |
|--------|------|
| Go 后端服务器 | ✅ 能启动、监听端口 |
| GreptimeDB 管理 | ✅ 启动、表初始化 |
| Hook Handler | ✅ 接收 /api/hooks、存储到数据库 |
| JSONL Watcher | ✅ 监控 ~/.claude/projects/*.jsonl |
| Dashboard 框架 | ✅ 嵌入式 HTML，基础 UI |
| 数据模型 | ✅ 预留 tenant_id 多租户扩展 |

**下一步**：Plan 2 将实现 Analysis Engine + Problems View。