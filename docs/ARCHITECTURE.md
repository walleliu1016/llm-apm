# LLM-APM 架构文档

> 基于 Hook 和 JSONL 的 LLM Agent APM 系统架构设计

---

## 系统架构

### 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI Agents                              │
│   Claude Code | Codex | Copilot CLI                          │
└─────────────────────────────────────────────────────────────┘
         │                │                │
         │ Hooks (HTTP)   │ JSONL files    │ JSONL files
         │                │                │
         ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────┐
│                   llm-apm-server                             │
│                                                              │
│  ┌────────────┐ ┌────────────┐ ┌────────────────────────┐  │
│  │   Handler  │ │  Watcher   │ │   Analysis Engine      │  │
│  │            │ │            │ │                        │  │
│  │ Hook接收   │ │ JSONL监控  │ │ 9种异常检测规则        │  │
│  │ 解析存储   │ │ 实时解析   │ │ 根因推断引擎           │  │
│  │ SSE广播    │ │ 去重处理   │ │ 关键事件筛选           │  │
│  └────────────┘ └────────────┘ └────────────────────────┘  │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              SSE Broadcaster                          │  │
│  │  实时推送：错误、异常、慢工具                         │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            │
                            │ HTTP SQL API
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                     GreptimeDB                               │
│                                                              │
│  数据表：                                                    │
│  • apm_hook_events   - Hook 事件记录                        │
│  • apm_messages      - 对话消息记录                         │
│  • apm_anomalies     - 异常检测结果                         │
│  • apm_turns         - Turn 边界记录                        │
└─────────────────────────────────────────────────────────────┘
                            │
                            │ SQL Query
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      Dashboard                               │
│                                                              │
│  ┌────────────┐ ┌────────────┐ ┌────────────────────────┐  │
│  │  Sessions  │ │  Problems  │ │      Analysis          │  │
│  │            │ │            │ │                        │  │
│  │ Session列表│ │ 异常列表   │ │ Token/成本统计         │  │
│  │ 执行时间线 │ │ 根因推断   │ │ 延迟分布分析           │  │
│  │ Turn组织   │ │ 一键定位   │ │ 工具调用热力图         │  │
│  └────────────┘ └────────────┘ └────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 模块架构

### 服务端模块结构

```
server/
├── cmd/
│   └── llm-apm-server/      # 主入口
│       └── main.go          # 启动逻辑
│
├── internal/
│   ├── handler/             # HTTP API 处理
│   │   ├── handler.go       # 路由注册
│   │   ├── hooks.go         # Hook 事件接收
│   │   ├── sessions.go      # Session API
│   │   ├── problems.go      # Problems API
│   │   ├── analysis.go      # Analysis API (11 endpoints)
│   │   └── stats.go         # Stats API
│   │
│   ├── analysis/            # 分析引擎
│   │   ├── engine.go        # 分析引擎核心
│   │   ├── rules.go         # 9 种异常检测规则
│   │   └── inference.go     # 根因推断逻辑
│   │
│   ├── broadcaster/         # SSE 广播器
│   │   └── broadcaster.go   # 实时事件推送
│   │
│   ├── greptimedb/          # 数据库管理
│   │   ├── process.go       # GreptimeDB 进程管理
│   │   └── tables.go        # 数据表结构定义
│   │
│   ├── turn/                # Turn 边界追踪
│   │   └── tracker.go       # Turn 边界划分
│   │
│   ├── transcript/          # JSONL 监控
│   │   └── watcher.go       # 文件监控解析
│   │
│   ├── stats/               # 统计聚合
│   │   ├── aggregator.go    # 统计聚合逻辑
│   │   ├── cost.go          # 成本计算
│   │   └── cache.go         # 缓存统计
│   │
│   └── config/              # 配置管理
│       └── config.go        # 配置加载
│
└── web/
    ├── index.html           # Dashboard 页面
    └── web.go               # 静态资源嵌入
```

---

## 数据流程

### Hook 事件流程

```
1. Agent 触发 Hook
   └─→ HTTP POST /api/hooks

2. Handler 接收处理
   └─→ 解析 JSON payload
   └─→ 提取关键字段

3. 数据存储
   └─→ 写入 apm_hook_events 表

4. 分析引擎
   └─→ 实时异常检测
   └─→ 根因推断
   └─→ 生成 anomaly 记录

5. SSE 广播
   └─→ 推送关键事件到前端
```

### JSONL 监控流程

```
1. 文件监控
   └─→ 监听 ~/.claude/projects/*.jsonl

2. 实时解析
   └─→ 解析 JSONL 行
   └─→ 提取 message 内容

3. 数据存储
   └─→ 写入 apm_messages 表

4. 去重处理
   └─→ 避免重复写入
```

---

## API 架构

### REST API Endpoints (25+)

| 分类 | Endpoint | 功能 |
|------|----------|------|
| **核心** | `/health` | 健康检查 |
| | `/` | Dashboard 页面 |
| | `/api/hooks` | Hook 事件接收 |
| | `/api/hooks/stream` | SSE 实时推送 |
| **Session** | `/api/sessions` | Session 列表 |
| | `/api/sessions/{id}` | Session 详情 |
| **Problems** | `/api/problems` | 异常列表 |
| | `/api/problems/{id}` | 异常详情 |
| **Stats** | `/api/stats/overview` | 总览统计 |
| | `/api/stats/tools` | 工具统计 |
| | `/api/stats/cache` | 缓存统计 |
| **Analysis** | `/api/analysis/overview` | 分析总览 |
| | `/api/analysis/timeline` | Session 时间线 |
| | `/api/analysis/models` | 模型分布 |
| | `/api/analysis/cache` | 缓存效率 |
| | `/api/analysis/anomalies` | 异常分布 |
| | `/api/analysis/ttft` | TTFT 分布 |
| | `/api/analysis/cost-ranking` | 成本排名 |
| | `/api/analysis/tools` | 工具详情 |
| | `/api/analysis/subagent` | Subagent 成本 |
| | `/api/analysis/turn-efficiency` | Turn 效率 |
| | `/api/analysis/agents` | Agent 对比 |

---

## 异常检测架构

### 9 种检测规则

| 规则类型 | 触发条件 | 严重级别 |
|----------|----------|----------|
| **slow_tool** | 工具执行 > 30s | medium |
| **slow_tool_critical** | 工具执行 > 60s | high |
| **tool_failure** | 工具执行失败 | high |
| **error_spike** | 5分钟内错误 > 3 | medium |
| **high_cost** | 单次成本 > $1 | medium |
| **cache_miss_spike** | 缓存命中率 < 50% | low |
| **subagent_depth** | Subagent 嵌套 > 3 | medium |
| **turn_loop** | Turn 数 > 20 | low |
| **timeout** | 工具超时 | high |

### 根因推断逻辑

```
1. 异常事件收集
   └─→ 提取异常上下文

2. 关联分析
   └─→ 查找相关事件链
   └─→ 分析前后因果关系

3. 原因推断
   └─→ 根据异常类型匹配预设模板
   └─→ 生成建议修复方案

4. 结果输出
   └─→ suggested_cause 字段
   └─→ 前端展示根因推断结果
```

---

## 前端架构

### Dashboard 页面结构

```
Dashboard
├── 顶部导航栏
│   ├── Sessions Tab
│   ├── Problems Tab
│   ├── Analysis Tab
│   └── 实时通知区
│
├── Sessions View
│   ├── Session 列表 (左侧)
│   ├── 执行时间线 (右侧)
│   │   ├── 用户输入
│   │   ├── LLM 调用详情
│   │   ├── 工具调用详情
│   │   └── Subagent 嵌套
│   │
│
├── Problems View
│   ├── 异常列表 (左侧)
│   ├── 异常详情 (右侧)
│   │   ├── 统计卡片
│   │   ├── 根因推断
│   │   ├── 关联事件链
│   │   └── 上下文预览
│
└── Analysis View
    ├── Row 1: 总览统计 (4 卡片)
    ├── Row 2: 模型分布 + 缓存效率 + 异常分布
    ├── Row 3: TTFT分布 + 成本排名 + Turn效率 + Subagent成本
    └── Row 4: 工具调用详情 (全行)
```

---

## 数据模型

### 核心数据表

| 表名 | 用途 | 关键字段 |
|------|------|----------|
| **apm_hook_events** | Hook 事件记录 | session_id, event_type, tool_name, tool_input, tool_result, error_flag |
| **apm_messages** | 对话消息记录 | session_id, message_type, content, input_tokens, output_tokens, cache_read_tokens |
| **apm_anomalies** | 异常检测结果 | session_id, anomaly_type, severity, description, suggested_cause |
| **apm_turns** | Turn 边界记录 | session_id, turn_id, user_prompt, agent_response, tool_count |

---

## 部署架构

### 本地部署

```
┌─────────────────────────────────────────┐
│              本地机器                    │
│                                         │
│  ┌─────────────┐  ┌─────────────────┐  │
│  │ llm-apm     │  │ GreptimeDB      │  │
│  │ -server     │  │ (嵌入式进程)    │  │
│  │             │  │                 │  │
│  │ Port: 14318 │  │ HTTP: 4000      │  │
│  │             │  │ MySQL: 14002    │  │
│  └─────────────┘  └─────────────────┘  │
│                                         │
│  数据目录: ~/.llm-apm/                  │
└─────────────────────────────────────────┘
```

### 配置项

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `APM_HOST` | 127.0.0.1 | HTTP 服务地址 |
| `APM_PORT` | 14318 | HTTP 服务端口 |
| `APM_DATA_DIR` | ~/.llm-apm | 数据存储目录 |
| `APM_GREPTIMEDB_HTTP_PORT` | 4000 | GreptimeDB HTTP 端口 |
| `APM_DATA_TTL` | 60d | 数据保留时间 |

---

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| **后端** | Go 1.21+ | HTTP 服务、Hook 处理 |
| **数据库** | GreptimeDB | 时序数据库，SQL 接口 |
| **前端** | 原生 JS | Dashboard 页面 |
| **通信** | SSE | 实时事件推送 |
| **存储** | 本地文件 | JSONL transcript |

---

## 扩展预留

### 多租户扩展

- 所有表预留 `tenant_id` 字段
- API 预留认证扩展点
- Dashboard 预留登录界面

### Agent 扩展

- Hook payload 支持 `agent_source` 字段
- API 支持 Codex/Copilot CLI 数据格式
- Dashboard 支持多 Agent 对比视图