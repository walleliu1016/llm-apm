# LLM-APM 设计文档

> 基于 Hook 和 JSONL 的 LLM Agent APM 系统，用于快速分析和定位 Agent 执行问题。

## 1. 项目概述

### 1.1 目标

构建一个全方位的 LLM Agent APM 系统，核心能力：

| 能力 | 描述 |
|------|------|
| **性能分析** | Token 消耗、延迟、成本追踪 |
| **问题定位** | 快速排查执行失败、工具调用异常、逻辑错误 |
| **执行回放** | 完整的对话/工具调用时间线 |
| **智能分析** | 自动异常检测、根因推断 |

### 1.2 支持的 Agent

- Claude Code
- Codex
- Copilot CLI

### 1.3 差异化定位

相比 tma1 的改进：

| 方面 | 改进 |
|------|------|
| **轻量化** | 精简模块，专注核心 APM |
| **智能分析** | 内置异常检测规则 + 根因推断引擎 |
| **问题定位 UX** | 一键跳转到错误上下文，时间线高亮异常段 |
| **混合实时** | 关键事件实时推送，普通数据事后分析 |

---

## 2. 架构设计

### 2.1 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLI Agents                                │
│   Claude Code | Codex | Copilot CLI                              │
└─────────────────────────────────────────────────────────────────┘
         │                    │                    │
         │ Hooks (HTTP)       │ JSONL files        │ JSONL files
         │ /api/hooks         │ ~/.claude/...      │ ~/.codex/...
         ▼                    ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                     llm-apm-server                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │ Hook Handler │  │ JSONL Watcher│  │ Analysis Engine      │   │
│  │ - 接收事件   │  │ - 监控文件   │  │ - 异常检测规则       │   │
│  │ - 解析存储   │  │ - 解析写入   │  │ - 根因推断           │   │
│  │ - SSE广播    │  │ - 去重处理   │  │ - 关键事件筛选       │   │
│  └──────────────┘  └──────────────┘  └──────────────────────┘   │
│         │                  │                  │                 │
│         └──────────────────┼──────────────────┘                 │
│                            ▼                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    SSE Broadcaster                        │   │
│  │  - 关键事件实时推送（错误、异常、慢工具）                 │   │
│  │  - 普通数据不推送，仅事后查询                             │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ HTTP SQL API
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                       GreptimeDB                                 │
│  Tables:                                                         │
│  - apm_hook_events      (hook 事件)                             │
│  - apm_messages         (对话内容)                              │
│  - apm_anomalies        (检测到的异常)                          │
│  - apm_turns            (Turn 边界)                             │
│  - opentelemetry_logs   (OTel 日志)                             │
│  - opentelemetry_traces (OTel traces，可选)                    │
│  - apm_sessions_1m      (每分钟聚合)                            │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ SQL Query
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Dashboard                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ Sessions View   │  │ Problems View   │  │ Analysis View   │  │
│  │ - Session列表   │  │ - 异常列表      │  │ - Token/成本    │  │
│  │ - 执行树       │  │ - 根因推断      │  │ - 延迟分析      │  │
│  │ - Turn组织     │  │ - 一键定位      │  │ - 工具热力图    │  │
│  │ - Subagent嵌套 │  │ - 时间线高亮    │  │ - 消耗时间线    │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│                                                                  │
│  实时通知区：关键事件弹出提醒                                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. 数据模型

### 3.1 核心表结构

#### `apm_hook_events` — Hook 事件表

```sql
CREATE TABLE apm_hook_events (
    ts              TIMESTAMP,
    session_id      STRING,
    event_type      STRING,        -- PreToolUse/PostToolUse/SessionEnd...
    agent_source    STRING,        -- claude_code/codex/copilot_cli
    agent_id        STRING,        -- 调用者的 agent ID
    parent_agent_id STRING,        -- 父 agent ID（Subagent 场景）
    agent_depth     BIGINT,        -- 层级深度：0=父, 1=一级Subagent
    turn_id         STRING,        -- 归属哪个 Turn
    tool_name       STRING,
    tool_input      STRING,        -- 截断 2048
    tool_result     STRING,        -- 截断 4096
    tool_use_id     STRING,
    cwd             STRING,
    error_flag      BOOLEAN,       -- 是否错误事件
    metadata        JSON,
);
-- INDEX: SKIPPING INDEX on session_id
-- INDEX: INVERTED INDEX on event_type, agent_source, error_flag
```

#### `apm_messages` — 对话消息表

```sql
CREATE TABLE apm_messages (
    ts                  TIMESTAMP,
    session_id          STRING,
    message_type        STRING,    -- user/assistant/thinking/tool_use/tool_result
    role                STRING,
    content             STRING,    -- 截断 32768
    model               STRING,
    tool_name           STRING,
    tool_use_id         STRING,
    input_tokens        BIGINT,
    output_tokens       BIGINT,
    cache_read_tokens   BIGINT,
    cache_creation_tokens BIGINT,
);
-- INDEX: FULLTEXT INDEX on content
-- INDEX: SKIPPING INDEX on session_id
```

#### `apm_turns` — Turn 边界表

```sql
CREATE TABLE apm_turns (
    ts              TIMESTAMP,
    turn_id         STRING,
    session_id      STRING,
    start_ts        TIMESTAMP,     -- UserPromptSubmit 时间
    end_ts          TIMESTAMP,     -- AssistantResponse 时间
    user_prompt     STRING,        -- 用户输入内容
    agent_response  STRING,        -- Agent 最终响应摘要
    input_tokens    BIGINT,        -- 该 Turn 消耗
    output_tokens   BIGINT,
    cost_usd        DOUBLE,
    tool_count      BIGINT,
    has_error       BOOLEAN,
);
```

#### `apm_anomalies` — 异常检测表

```sql
CREATE TABLE apm_anomalies (
    ts              TIMESTAMP,
    session_id      STRING,
    anomaly_type    STRING,        -- slow_tool/high_cost/error_spike...
    severity        STRING,        -- low/medium/high/critical
    description     STRING,
    related_event_id STRING,
    suggested_cause STRING,        -- 根因推断
    metadata        JSON,
);
-- INDEX: INVERTED INDEX on anomaly_type, severity, session_id
```

#### `opentelemetry_logs` — OTel 日志表（保留）

用于 Claude Code / Codex 的 API 请求/错误事件。

---

## 4. Turn 边界界定

### 4.1 Turn 定义

**Turn = 一轮用户-Agent 交互**

用户发送消息 → Agent 执行操作（LLM + 工具）→ Agent 响应 → 等待用户下一条消息

### 4.2 边界界定方案（混合）

**实时场景（Hook 流）**：

```
UserPromptSubmit → 标记 Turn 开始，记录 start_ts
工具调用事件 → 归属当前 active Turn
AssistantResponse → 标记 Turn 结束
```

**事后分析（JSONL 回填）**：

```
解析 JSONL，补充用户提示内容、完整工具列表
修正 Turn 边界（处理 Hook 缺失情况）
```

### 4.3 Subagent 处理

Subagent 内部按 **思考轮次** 组织（LLM 调用边界），嵌套在父 Turn 下：

```
Turn 1: 用户请求重构
├─ LLM [main]: 思考 → 决定启动 Subagent
├─ Agent [main → sub_001]: 启动 Subagent
│   └─ Subagent 执行详情（嵌套展示）
│       ├─ 思考轮次 1 [sub_001]: LLM + Read/Read
│       ├─ 思考轮次 2 [sub_001]: LLM + Edit/Write
│       └─ 思考轮次 3 [sub_001]: LLM + Bash
├─ LLM [main]: 处理 Subagent 结果
└─ 响应完成
```

---

## 5. Analysis Engine（智能分析层）

### 5.1 异常检测规则

| 规则 ID | 异常类型 | 触发条件 | 严重程度 |
|---------|----------|----------|----------|
| `slow_tool` | 工具执行慢 | 单次工具调用 > 30s | medium |
| `slow_tool_critical` | 工具极慢 | 单次工具调用 > 60s | critical |
| `high_cost_turn` | 单轮成本高 | 单轮 token > 50000 或 cost > $1 | high |
| `token_burn_fast` | Token 燃烧快 | 5分钟内消耗 > 100k tokens | medium |
| `error_spike` | 错误集中 | 1分钟内 > 3 个 PostToolUseFailure | critical |
| `subagent_timeout` | Subagent 超时 | SubagentStop 无响应 > 5min | high |
| `repeated_tool` | 重复工具调用 | 同一工具 3 次失败后仍调用 | medium |
| `context_overflow` | 上下文溢出 | compaction 事件 > 3 次/分钟 | medium |
| `tool_reject_spike` | 权限拒绝多 | 1分钟内 > 5 次 tool reject | medium |

### 5.2 根因推断逻辑

基于规则链的推断引擎：

```
异常事件 → 关联分析 → 推断根因 → 输出建议

示例：
1. 检测到 slow_tool (Bash, 45s)
2. 关联查找同一 session 的其他事件
3. 发现 PreToolUse 时有 permission prompt
4. 推断根因：用户确认耗时导致执行延迟
5. 建议：检查 permission 配置，考虑 auto-approve
```

### 5.3 SSE 推送筛选

只推送关键事件：

| 事件 | 推送条件 |
|------|----------|
| PostToolUse | success=false |
| SessionEnd | 有异常标记 |
| Notification | notification_type=error |
| Anomaly | severity >= medium |

---

## 6. Dashboard UX

### 6.1 三大主视图

| 视图 | 功能 |
|------|------|
| **Sessions** | Session 列表 + 执行树（Turn + Subagent 嵌套） |
| **Problems** | 异常列表 + 根因推断 + 一键跳转 |
| **Analysis** | 消耗时间线 + 缓存效率 + 异常分布 + 工具热力图 |

### 6.2 执行树组织

- **父 Agent**：按 Turn 组织（用户交互边界）
- **Subagent**：按思考轮次组织（LLM 调用边界），嵌套显示

### 6.3 一键跳转机制

```
Problems View 点击异常 → switchView('sessions') → expandSession → scrollToTimelinePosition → highlightEvent
```

### 6.4 Analysis 视图核心模块

| 模块 | 内容 |
|------|------|
| 时间范围选择器 | 今日/7天/30天/自定义 |
| 消耗时间线 | Session 归因，按时间排序，点击跳转 |
| 缓存效率 | 缓存读取、节省成本、命中率 |
| 异常分布 | 饼图展示异常类型占比 |
| TTFT 分布 | 首字节延迟分布 |
| 成本归因 Top 10 | 高成本 Session 排名 |
| Subagent 成本占比 | Main Agent vs Subagent |
| Turn 效率 | 平均 Turns/Session、工具/Turn、输入输出比 |
| 工具详情 | 成功率、平均耗时、失败原因 |

---

## 7. 技术栈

| 组件 | 选择 | 原因 |
|------|------|------|
| 后端 | Go | 复用 tma1 架构，可靠性高 |
| 存储 | GreptimeDB | 时序数据库，适合 metrics/traces |
| 前端 | 原生 JS + HTML | 轻量、嵌入式 |
| SSE | EventSource API | 浏览器原生支持 |

---

## 8. API Endpoints

| Endpoint | Method | 描述 |
|----------|--------|------|
| `/health` | GET | 健康检查 |
| `/api/hooks` | POST | 接收 Hook 事件 |
| `/api/hooks/stream` | GET | SSE 实时推送 |
| `/api/query` | POST | SQL 查询代理 |
| `/v1/otlp` | POST | OTLP 数据接收 |
| `/v1/traces` | POST | Traces 接收 |
| `/v1/metrics` | POST | Metrics 接收 |
| `/v1/logs` | POST | Logs 接收 |

---

## 9. 配置项

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `APM_HOST` | 127.0.0.1 | 绑定地址 |
| `APM_PORT` | 14318 | HTTP 端口 |
| `APM_DATA_DIR` | ~/.llm-apm | 数据目录 |
| `APM_GREPTIMEDB_HTTP_PORT` | 14000 | GreptimeDB HTTP 端口 |
| `APM_GREPTIMEDB_MYSQL_PORT` | 14002 | GreptimeDB MySQL 端口 |
| `APM_LOG_LEVEL` | info | 日志级别 |
| `APM_DATA_TTL` | 60d | 数据保留时间 |

---

## 10. 多租户扩展预留

当前设计为单用户（本地部署），但预留以下扩展点，便于后期升级为多租户：

### 10.1 数据模型预留

所有核心表预留 `tenant_id` 字段（当前版本为空或默认值）：

```sql
-- 当前版本
CREATE TABLE apm_hook_events (
    ...
    tenant_id       STRING DEFAULT '',  -- 预留：多租户时填充
    ...
);

-- 多租户版本（未来）
CREATE TABLE apm_hook_events (
    ...
    tenant_id       STRING NOT NULL,    -- 必填，租户隔离
    ...
);
-- INDEX: INVERTED INDEX on tenant_id（多租户版本添加）
```

| 表 | 预留字段 |
|---|---------|
| apm_hook_events | tenant_id |
| apm_messages | tenant_id |
| apm_turns | tenant_id |
| apm_anomalies | tenant_id |
| opentelemetry_logs | tenant_id（已有 scope_name，可复用） |

### 10.2 API 扩展预留

当前版本无认证，多租户版本预留：

| Endpoint | 当前版本 | 多租户版本 |
|----------|----------|-----------|
| `/api/hooks` | 无认证 | X-Tenant-ID header + API Key |
| `/api/hooks/stream` | 无认证 | SSE 认证 |
| `/api/query` | 无认证 | tenant_id 自动注入 WHERE 条件 |
| `/v1/otlp` | 无认证 | X-Tenant-ID header |

### 10.3 Dashboard 扩展预留

| 方面 | 当前版本 | 多租户版本 |
|------|----------|-----------|
| 登录 | 无 | 用户登录页面 |
| 租户筛选 | 无 | 顶部租户选择器（超级管理员） |
| 数据范围 | 全部数据 | 当前租户数据 |
| 权限控制 | 无 | RBAC（只读/管理员/超级管理员） |

### 10.4 存储扩展预留

| 方面 | 当前版本 | 多租户版本 |
|------|----------|-----------|
| GreptimeDB | 单实例 | 可保持单实例（tenant_id 过滤）或分实例 |
| 数据目录 | ~/.llm-apm | ~/.llm-apm/tenants/{tenant_id}/ |
| 配额 | 无限制 | 租户级配额（每日 token 上限、存储上限） |

### 10.5 迁移路径

从单用户升级到多租户：

```
1. 添加 tenant_id 字段（ALTER TABLE）
2. 回填现有数据：UPDATE ... SET tenant_id = 'default'
3. 添加认证中间件
4. 修改查询逻辑：自动注入 tenant_id 过滤
5. Dashboard 添加登录和租户选择
```

---

## 11. 后续扩展

- 自定义分析规则配置
- 更多 Agent 支持
- API 接口供外部系统调用