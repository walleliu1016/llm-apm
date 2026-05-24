[根目录](../CLAUDE.md) > **server**

# server 模块

> Go 后端服务，提供核心 APM 功能：Hook 接收、异常检测、统计聚合、Dashboard API。

## 变更记录 (Changelog)

| 时间 | 变更内容 |
|------|----------|
| 2026-05-24 11:13:31 | 初始化模块文档 |

---

## 模块职责

server 是 LLM-APM 的核心后端模块，负责：

1. **Hook 事件接收**：接收 Claude Code/Codex/Copilot CLI 的 Hook 事件
2. **JSONL 监控**：监控并解析 Agent 的 JSONL transcript 文件
3. **异常检测**：基于 9 种规则自动检测异常（慢工具、错误集中、高成本等）
4. **根因推断**：智能推断异常的根本原因
5. **统计聚合**：Token 消耗、成本、缓存效率统计
6. **Turn 边界追踪**：划分用户-Agent 交互轮次
7. **SSE 广播**：实时推送关键事件到前端
8. **Dashboard API**：提供 25+ REST API endpoints

---

## 入口与启动

### 主入口

```
server/cmd/llm-apm-server/main.go
```

### 启动流程

```go
// 1. 加载配置
cfg := config.Load()

// 2. 启动 GreptimeDB
greptime := greptimedb.NewProcess(cfg.DataDir, ...)
greptime.Start(ctx)

// 3. 初始化数据表
greptimedb.InitTables(cfg.GreptimeHTTPPort)

// 4. 创建 Handler Server
srv := handler.NewServer("127.0.0.1", cfg.GreptimeHTTPPort, logger)

// 5. 创建 Turn Tracker
turnTracker := turn.NewTrackerWithDB(...)
srv.SetTurnTracker(turnTracker)

// 6. 创建 Transcript Watcher
watcher := transcript.NewWatcher(...)

// 7. 注册 HTTP 路由
mux := http.NewServeMux()
srv.RegisterRoutes(mux)

// 8. 启动 HTTP Server
httpSrv := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), Handler: mux}
httpSrv.ListenAndServe()

// 9. 等待 shutdown signal
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
<-sigCh

// 10. Graceful shutdown
httpSrv.Shutdown(shutdownCtx)
greptime.Stop()
watcher.StopAll()
```

### 运行命令

```bash
# 编译
make build  # 输出到 bin/llm-apm-server

# 直接运行
make run

# 启动脚本（配置 GreptimeDB 端口）
./start.sh
```

---

## 对外接口

### HTTP API Endpoints（25+）

| Endpoint | Method | 功能 | Handler |
|----------|--------|------|---------|
| `/health` | GET | 健康检查 | `handleHealth` |
| `/` | GET | Dashboard 页面 | `handleDashboard` |
| `/api/hooks` | POST | 接收 Hook 事件 | `handleHooks` |
| `/api/hooks/stream` | GET | SSE 实时推送 | `handleSSEStream` |
| `/api/query` | POST | SQL 查询代理 | `handleQuery` |
| `/api/stats/*` | GET | 统计数据 | `handleStatsOverview` 等 |
| `/api/sessions` | GET | Session 列表 | `handleSessionsList` |
| `/api/sessions/{id}` | GET | Session 详情 | `handleSessionDetail` |
| `/api/problems` | GET | 异常列表 | `handleProblemsList` |
| `/api/problems/{id}` | GET | 异常详情 | `handleProblemDetail` |
| `/api/analysis/*` | GET | 分析数据（11 endpoints） | `handleAnalysisOverview` 等 |

### Analysis API 子集（11 endpoints）

| Endpoint | 功能 |
|----------|------|
| `/api/analysis/overview` | 总览统计（4 个卡片） |
| `/api/analysis/timeline` | Session 时间线 |
| `/api/analysis/models` | 模型分布 |
| `/api/analysis/cache` | 缓存效率 |
| `/api/analysis/anomalies` | 异常分布 |
| `/api/analysis/ttft` | TTFT 分布 |
| `/api/analysis/cost-ranking` | 成本排名 Top 10 |
| `/api/analysis/tools` | 工具使用统计 |
| `/api/analysis/subagent` | Subagent 成本占比 |
| `/api/analysis/turn-efficiency` | Turn 效率指标 |
| `/api/analysis/agents` | Agent 对比 |

### SSE 事件类型

| Event | 推送条件 |
|-------|----------|
| `anomaly` | `severity >= medium` 的异常 |
| `error` | `PostToolUseFailure` 事件 |
| `notification` | `notification_type=error` |

---

## 关键依赖与配置

### 依赖

| 包 | 用途 |
|-----|------|
| `github.com/akke/llm-apm/server/internal/*` | 内部模块 |
| Go 标准库 | `net/http`, `encoding/json`, `sync` 等 |

**注意**：`go.mod` 仅声明模块名，未列出外部依赖（Go 1.21 标准库足够）。

### 配置项（环境变量）

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `APM_HOST` | 127.0.0.1 | HTTP 服务绑定地址 |
| `APM_PORT` | 14318 | HTTP 服务端口 |
| `APM_DATA_DIR` | ~/.llm-apm | 数据存储目录 |
| `APM_GREPTIMEDB_HTTP_PORT` | 4000 | GreptimeDB HTTP 端口 |
| `APM_GREPTIMEDB_GRPC_PORT` | 14001 | GreptimeDB GRPC 端口 |
| `APM_GREPTIMEDB_MYSQL_PORT` | 14002 | GreptimeDB MySQL 端口 |
| `APM_LOG_LEVEL` | info | 日志级别 |
| `APM_DATA_TTL` | 60d | 数据保留时间 |
| `APM_TENANT_ID` | "" | 多租户预留字段 |

---

## 数据模型

### GreptimeDB 表结构

#### apm_hook_events（Hook 事件表）

```sql
CREATE TABLE apm_hook_events (
    ts TIMESTAMP TIME INDEX,
    session_id STRING,
    event_type STRING,          -- PreToolUse/PostToolUse/...
    agent_source STRING,        -- claude_code/codex/copilot_cli
    agent_id STRING,
    parent_agent_id STRING,
    agent_depth BIGINT DEFAULT 0,
    turn_id STRING DEFAULT '',
    tool_name STRING,
    tool_input STRING,          -- 截断 2048
    tool_result STRING,         -- 截断 4096
    tool_use_id STRING,
    cwd STRING,
    error_flag BOOLEAN DEFAULT false,
    tenant_id STRING DEFAULT '',
    extra JSON
) ENGINE=mito WITH ('append_mode' = 'true');
```

#### apm_messages（对话消息表）

```sql
CREATE TABLE apm_messages (
    ts TIMESTAMP TIME INDEX,
    session_id STRING,
    message_type STRING,        -- user/assistant/tool_use/tool_result
    msg_role STRING,
    content STRING,             -- 截断 32768
    model STRING,
    tool_name STRING,
    tool_use_id STRING,
    input_tokens BIGINT DEFAULT 0,
    output_tokens BIGINT DEFAULT 0,
    cache_read_tokens BIGINT DEFAULT 0,
    cache_creation_tokens BIGINT DEFAULT 0,
    tenant_id STRING DEFAULT ''
) ENGINE=mito WITH ('append_mode' = 'true');
```

#### apm_turns（Turn 边界表）

```sql
CREATE TABLE apm_turns (
    ts TIMESTAMP TIME INDEX,
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
) ENGINE=mito;
```

#### apm_anomalies（异常检测表）

```sql
CREATE TABLE apm_anomalies (
    ts TIMESTAMP TIME INDEX,
    session_id STRING,
    anomaly_type STRING,        -- slow_tool/error_spike/...
    severity STRING,            -- low/medium/high/critical
    description STRING,
    suggested_cause STRING,
    related_event_id STRING,
    tenant_id STRING DEFAULT '',
    extra JSON
) ENGINE=mito WITH ('append_mode' = 'true');
```

---

## 测试与质量

### 测试文件

| 文件 | 测试内容 |
|------|----------|
| `internal/analysis/engine_test.go` | 异常检测引擎 |
| `internal/analysis/rules_test.go` | 异常检测规则 |
| `internal/analysis/inference_test.go` | 根因推断逻辑 |
| `internal/broadcaster/broadcaster_test.go` | SSE 广播器 |
| `internal/config/config_test.go` | 配置加载 |
| `internal/greptimedb/process_test.go` | GreptimeDB 进程管理 |
| `internal/greptimedb/tables_test.go` | 数据表结构 |
| `internal/handler/hooks_test.go` | Hook 处理逻辑 |
| `internal/stats/aggregator_test.go` | 统计聚合 |
| `internal/stats/cache_test.go` | 缓存统计 |
| `internal/stats/cost_test.go` | 成本计算 |
| `internal/turn/tracker_test.go` | Turn 边界追踪 |

### 运行测试

```bash
make test  # go test -race -count=1 ./...
```

### 代码质量工具

```bash
make vet   # go vet ./...
make lint  # golangci-lint run
```

---

## 内部模块索引

| 包 | 职责 | 关键文件 |
|-----|------|----------|
| `analysis` | 异常检测引擎、根因推断 | `engine.go`, `rules.go`, `inference.go` |
| `broadcaster` | SSE 广播器 | `broadcaster.go` |
| `config` | 配置管理 | `config.go` |
| `greptimedb` | GreptimeDB 进程管理、表结构 | `process.go`, `tables.go` |
| `handler` | HTTP API handlers | `handler.go`, `hooks.go`, `sessions.go`, `problems.go`, `analysis.go` |
| `stats` | 统计聚合、成本计算 | `aggregator.go`, `cost.go`, `cache.go` |
| `transcript` | JSONL 文件监控 | `watcher.go` |
| `turn` | Turn 边界追踪 | `tracker.go` |

---

## 常见问题 (FAQ)

### Q: 如何添加新的异常检测规则？

1. 在 `internal/analysis/rules.go` 定义新规则类型
2. 实现 `Rule` 接口的 `Check` 方法
3. 在 `AllRules()` 函数中注册
4. 添加测试到 `rules_test.go`

### Q: 如何新增 API endpoint？

1. 在 `internal/handler/handler.go` 的 `RegisterRoutes` 中注册路由
2. 在对应 handler 文件实现 handler 函数（如 `sessions.go`、`analysis.go`）
3. 使用 `queryGreptimeDB()` 查询数据
4. 编写格式化函数（如 `formatSessionsResponse`）

### Q: 如何修改数据表结构？

1. 在 `internal/greptimedb/tables.go` 修改 `CreateTablesSQL()`
2. 注意：GreptimeDB 不支持 ALTER TABLE，需要重新建表或添加新表

### Q: Hook 事件格式是什么？

见 `docs/hooks-config.json`，Claude Code 发送的 JSON payload：
```json
{
  "session_id": "xxx",
  "hook_event_name": "PreToolUse",
  "tool_name": "Read",
  "tool_input": {"file_path": "/src/main.go"},
  "tool_use_id": "toolu_xxx",
  "agent_id": "",
  "agent_type": "main",
  "cwd": "/home/user/project",
  "transcript_path": "~/.claude/projects/xxx.jsonl"
}
```

---

## 相关文件清单

### 入口与启动

| 文件 | 用途 |
|------|------|
| `cmd/llm-apm-server/main.go` | 主入口 |
| `Makefile` | 构建脚本 |

### 核心模块

| 文件 | 功能 |
|------|------|
| `internal/handler/handler.go` | 路由注册 |
| `internal/handler/hooks.go` | Hook 处理 |
| `internal/handler/sessions.go` | Session API |
| `internal/handler/problems.go` | Problems API |
| `internal/handler/analysis.go` | Analysis API（11 endpoints） |
| `internal/analysis/engine.go` | 异常检测引擎 |
| `internal/analysis/rules.go` | 9 种异常检测规则 |
| `internal/analysis/inference.go` | 根因推断逻辑 |
| `internal/turn/tracker.go` | Turn 边界追踪 |
| `internal/transcript/watcher.go` | JSONL 监控 |
| `internal/stats/aggregator.go` | 统计聚合 |
| `internal/stats/cost.go` | 成本计算 |
| `internal/greptimedb/process.go` | GreptimeDB 管理 |
| `internal/greptimedb/tables.go` | 数据表定义 |

### 配置与 Web

| 文件 | 功能 |
|------|------|
| `internal/config/config.go` | 配置加载 |
| `web/index.html` | Dashboard 页面 |
| `web/web.go` | 嵌入静态资源 |

---

## 覆盖率缺口

| 方面 | 状态 | 备注 |
|------|------|------|
| API 实现 | 部分完成 | 部分 Analysis API 使用 mock 数据 |
| 成本计算 | 简化估算 | 未接入真实定价 API |
| TTFT 提取 | 未实现 | 需从 Hook 数据中提取首字节延迟 |
| Session Detail 时间线 | 渲染限制 | 前端部分功能未完全实现 |
| 多租户认证 | 预留 | 未实现认证逻辑 |

---

## 下一步建议

1. **补全 Analysis API**：将 mock 数据替换为真实 GreptimeDB 查询
2. **优化成本模型**：接入 Claude API 官方定价
3. **实现 TTFT 监控**：从 Hook 数据中提取首字节延迟
4. **完善前端交互**：Session Detail 时间线渲染优化