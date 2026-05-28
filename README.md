# LLM-APM

> **LLM Agent Application Performance Monitoring System**
> 
> 基于 Hook 和 JSONL 的 LLM Agent APM 系统，用于快速分析和定位 Agent 执行问题。

---

## 简介

LLM-APM 是一个专为 LLM Agent 设计的应用性能监控（APM）系统。它通过 Hook 机制和 JSONL 文件监控，实时采集 Claude Code、Codex、Copilot CLI 等 Agent 的执行数据，提供性能分析、问题定位、执行回放和智能分析功能。

### 为什么需要 LLM-APM？

在开发和使用 LLM Agent 时，我们经常遇到这些问题：

- **工具调用失败** - 不知道哪个工具为什么失败
- **执行超时** - 不知道哪个步骤消耗了最多时间
- **成本过高** - 不知道 Token 消耗在哪里
- **缓存效率低** - 不知道为什么缓存命中率低
- **Subagent 嵌套过深** - 不知道 Agent 调用链有多复杂

LLM-APM 帮助你快速定位这些问题，提高 Agent 开发和使用效率。

---

## 功能特性

### 核心功能

| 功能 | 描述 |
|------|------|
| **Session 时间线** | 可视化显示完整的 Agent 执行过程 |
| **执行树组织** | 按 Turn 组织执行树，支持 Subagent 嵌套展示 |
| **异常检测** | 自动检测 9 种异常类型 |
| **根因推断** | 智能推断异常原因，提供修复建议 |
| **实时通知** | 关键事件实时推送，无需手动刷新 |

### 分析功能

| 功能 | 描述 |
|------|------|
| **Token 统计** | 总消耗、输入/输出分布、趋势对比 |
| **成本分析** | 成本排名、Subagent 成本占比 |
| **模型分布** | 各模型 Token 使用占比 |
| **缓存效率** | 缓存命中率、节省成本估算 |
| **TTFT 分布** | 工具执行时间分布分析 |
| **工具热力图** | 各工具调用次数、成功率、平均时间 |

### 异常检测规则

| 异常类型 | 触发条件 |
|----------|----------|
| 慢工具 | 工具执行 > 30s |
| 关键慢工具 | 工具执行 > 60s |
| 工具失败 | 工具执行返回错误 |
| 错误集中 | 5分钟内错误 > 3 次 |
| 高成本 | 单次调用成本 > $1 |
| 缓存失效 | 缓存命中率 < 50% |
| Subagent 深度过深 | Subagent 嵌套 > 3 层 |
| Turn 循环 | Turn 数 > 20 |
| 超时 | 工具执行超时 |

---

## 界面预览

### Sessions View - 执行时间线

展示完整的 Agent 执行过程，包括：
- 用户输入
- LLM 调用（显示模型、Token 消耗、用户输入）
- 工具调用（显示工具名、执行时间、输入参数、输出结果）
- Subagent 嵌套（支持多层嵌套展示）

### Problems View - 问题定位

展示检测到的异常，包括：
- 异常列表（按严重级别排序）
- 根因推断（智能分析异常原因）
- 关联事件链（展示异常前后的事件）
- 一键定位（跳转到 Session 时间线高亮异常段）

### Analysis View - 统计分析

展示全局统计分析，包括：
- 总览统计卡片（Token、成本、缓存、异常）
- 模型分布图
- 缓存效率分析
- TTFT 分布
- 成本排名 Top 10
- 工具调用详情（全行展示）

---

## 快速开始

### 安装

```bash
# 克隆仓库
git clone https://github.com/walleliu1016/llm-apm.git
cd llm-apm/server

# 编译
make build
```

### 启动

```bash
# 使用启动脚本
cd llm-apm
./start.sh
```

访问 Dashboard: http://127.0.0.1:14318/

### 配置 Hook

将 Hook 配置添加到 Claude Code 的 settings 文件：

```json
// ~/.claude/settings.json 或 .claude/settings.json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": {
          "toolName": "Bash"
        },
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:14318/api/hooks"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": {},
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:14318/api/hooks"
          }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "matcher": {},
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:14318/api/hooks"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": {},
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:14318/api/hooks"
          }
        ]
      }
    ]
  }
}
```

---

## 系统要求

| 要求 | 说明 |
|------|------|
| **操作系统** | Linux / macOS |
| **Go 版本** | 1.21+ |
| **内存** | 最小 512MB |
| **磁盘** | 最小 1GB（数据存储） |

---

## 端口配置

| 服务 | 默认端口 | 环境变量 |
|------|----------|----------|
| APM HTTP Server | 14318 | `APM_PORT` |
| GreptimeDB HTTP | 4000 | `APM_GREPTIMEDB_HTTP_PORT` |
| GreptimeDB MySQL | 14002 | `APM_GREPTIMEDB_MYSQL_PORT` |

### 远程 GreptimeDB 配置

如需使用远程 GreptimeDB，设置以下环境变量：

```bash
# 远程 GreptimeDB 主机地址
export APM_GREPTIMEDB_HOST=192.168.1.100

# 禁用嵌入式 GreptimeDB 进程启动
export APM_GREPTIMEDB_EMBEDDED=false

# 远程 GreptimeDB HTTP 端口
export APM_GREPTIMEDB_HTTP_PORT=4000

# 启动服务
./start.sh
```

服务启动时会自动检测并创建所需数据表（使用 `CREATE TABLE IF NOT EXISTS`）。

---

## 项目结构

```
llm-apm/
├── server/           # Go 后端服务
│   ├── cmd/          # 主入口
│   ├── internal/     # 内部模块
│   │   ├── handler/  # HTTP API 处理
│   │   ├── analysis/ # 分析引擎
│   │   ├── broadcaster/ # SSE 广播
│   │   ├── greptimedb/ # 数据库管理
│   │   ├── turn/     # Turn 追踪
│   │   ├── transcript/ # JSONL 监控
│   │   ├── stats/    # 统计聚合
│   │   └── config/  # 配置管理
│   └── web/          # Dashboard 前端
│
├── docs/             # 文档
│   ├── design/       # 设计文档
│   ├── ARCHITECTURE.md # 架构文档
│   └── hooks-config.json # Hook 配置示例
│
├── bin/              # 编译输出
└── start.sh          # 启动脚本
```

---

## 支持的 Agent

| Agent | 数据来源 | 支持状态 |
|-------|----------|----------|
| **Claude Code** | Hooks + JSONL | ✅ 完整支持 |
| **Codex** | JSONL | ⏳ 规划中 |
| **Copilot CLI** | JSONL | ⏳ 规划中 |

---

## 技术栈

| 层级 | 技术 |
|------|------|
| **后端** | Go 1.21+ |
| **数据库** | GreptimeDB（时序数据库） |
| **前端** | 原生 JavaScript |
| **通信** | SSE（Server-Sent Events） |

---

## 常见问题

### Q: 数据存储在哪里？

数据存储在 `~/.llm-apm/` 目录下，由 GreptimeDB 管理。默认保留 60 天。

### Q: 如何查看特定 Session 的执行过程？

在 Sessions View 中点击 Session 条目，右侧会显示完整的执行时间线。

### Q: 如何定位异常的根本原因？

在 Problems View 中点击异常条目，右侧会显示根因推断结果和建议修复方案。

### Q: 如何添加新的异常检测规则？

在 `server/internal/analysis/rules.go` 中添加新的规则类型，实现 `Rule` 接口的 `Check` 方法。

---

## 路线图

### 已完成 ✅

- Hook 事件接收和存储
- JSONL 文件监控
- 9 种异常检测规则
- 根因推断引擎
- SSE 实时推送
- Dashboard 三大视图
- 全部 Analysis API（真实数据）

### 进行中 🚧

- Turn 边界精确追踪
- 工具执行时间精确存储

### 规划中 📋

- Codex 数据支持
- Copilot CLI 数据支持
- 多租户认证
- 成本精确计算（接入官方定价）
- TTFT 真实测量（首字节延迟）

---

## 贡献指南

欢迎提交 Issue 和 Pull Request！

---

## 许可证

MIT License

---

## 相关文档

- [架构文档](docs/ARCHITECTURE.md) - 系统架构详细说明
- [设计文档](docs/design/2024-01-15-llm-apm-design.md) - 完整设计文档
- [Hook 配置示例](docs/hooks-config.json) - Claude Code Hook 配置