---
name: api-design-for-dashboard
description: 设计 Dashboard API 接口，替换 mock 数据为真实数据，严格匹配前端 HTML 数据结构
metadata:
  type: project
---

# LLM-APM Dashboard API 设计文档

**目标：** 将 demo/dashboard-mockup.html 的硬编码 mock 数据替换为真实 API 调用，严格保持 HTML 内容和渲染效果不变。

**策略：** 方法 1 - 分离式 API（多个独立接口，职责清晰，易于维护）。

---

## 一、Sessions View API

### 1.1 `/api/sessions` - Session列表

**用途：** 返回 Session 列表数据，用于左侧列表渲染。

**请求参数：**
- `filter`: string (可选) - "all" | "anomaly" | "running"
- `range`: string (可选) - "today" | "7d" | "30d"

**响应格式（JSON）：**
```json
{
  "sessions": [
    {
      "session_id": "abc-123-def",
      "status": "completed",
      "status_text": "已完成",
      "agent_source": "Claude Code",
      "start_time": "2024-01-15 14:30",
      "anomaly_count": 2,
      "tool_count": 45,
      "cost": "$2.35",
      "total_tokens": "12k",
      "has_anomaly": true
    },
    {
      "session_id": "ghi-789-jkl",
      "status": "running",
      "status_text": "运行中",
      "agent_source": "Codex",
      "start_time": "2024-01-15 15:00",
      "anomaly_count": 0,
      "tool_count": 23,
      "cost": "$1.20",
      "total_tokens": "8k",
      "has_anomaly": false
    }
  ]
}
```

**数据映射：**
- `session_id` → `.session-id`
- `status_text` → `.session-status` 文本（"已完成"/"运行中"）
- `status` → CSS class（"status-completed"/"status-running"）
- `agent_source` → `.session-meta` 第一个 span
- `start_time` → `.session-meta` 第二个 span
- `anomaly_count` → `.session-stat` "🔴 X 异常"
- `tool_count` → `.session-stat` "X 工具调用"
- `cost` → `.session-stat` "$X.XX"
- `total_tokens` → `.session-stat` "Xk tokens"
- `has_anomaly` → CSS class "has-anomaly"（用于左侧边框红色标识）

---

### 1.2 `/api/sessions/{session_id}` - Session详情

**用途：** 返回 Session 详情、执行时间线、执行树数据。

**响应格式（JSON）：**
```json
{
  "session_id": "abc-123-def",
  "status": "completed",
  "status_text": "已完成",
  "agent_source": "Claude Code",
  "duration": "8min 32s",
  "total_cost": "$2.35",
  "timeline_blocks": [
    {
      "type": "user",
      "left": "0%",
      "width": "8%",
      "tooltip": "User: 实现登录功能",
      "label": "User",
      "has_error": false
    },
    {
      "type": "thinking",
      "left": "9%",
      "width": "5%",
      "tooltip": "Thinking: 分析需求",
      "label": "思考",
      "has_error": false
    },
    {
      "type": "tool",
      "left": "15%",
      "width": "10%",
      "tooltip": "Read: src/auth.ts",
      "label": "Read",
      "has_error": false
    },
    {
      "type": "error",
      "left": "35%",
      "width": "12%",
      "tooltip": "Bash: npm install (慢)",
      "label": "🔴 Bash",
      "has_error": true
    }
  ],
  "turns": [
    {
      "turn_id": "turn-1",
      "turn_name": "Turn 1: 用户请求实现登录功能",
      "turn_time": "14:30:00",
      "expanded": true,
      "llm_call": {
        "llm_id": "llm-1",
        "model": "claude-sonnet-4",
        "model_full": "claude-sonnet-4-20250514",
        "token_badge": "in: 2.1k | out: 1.5k",
        "cost": "$0.08",
        "duration": "(3.2s)",
        "status": "成功",
        "input_tokens": "2,148",
        "output_tokens": "1,523",
        "cache_read_tokens": "512",
        "cache_creation_tokens": "0",
        "estimated_cost": "$0.08",
        "ttft": "0.8s",
        "total_time": "3.2s",
        "generation_speed": "476 tok/s",
        "user_prompt": "请帮我实现一个登录功能，包括：\n1. 用户名密码验证\n2. JWT token 生成\n3. 登录状态持久化\n需要写测试代码。",
        "model_response": "I'll implement the login functionality step by step:\n1. First, let me read the existing auth module...\n[调用工具: Read src/auth.ts]\n[调用工具: Edit src/auth.ts]\n[调用工具: Write tests/auth.test.ts]"
      },
      "tool_calls": [
        {
          "tool_id": "read-auth",
          "tool_name": "Read",
          "tool_detail": "src/auth.ts",
          "duration": "(2s)",
          "has_error": false,
          "status": "成功",
          "execution_time": "2.1s",
          "tool_use_id": "tool_001",
          "input_params": "{\"file_path\": \"/src/auth.ts\"}",
          "output_result": "// auth.ts - Authentication module\nexport function validateToken(token: string) { ... }\nexport function loginUser(credentials) { ... }",
          "lines_changed": null
        },
        {
          "tool_id": "bash-install",
          "tool_name": "🔴 Bash",
          "tool_detail": "npm install",
          "duration": "(45s) ← 异常",
          "has_error": true,
          "status": "慢速执行",
          "execution_time": "45.2s (阈值: 30s)",
          "permission_decision": "用户手动批准 (耗时 40s)",
          "tool_use_id": "tool_005",
          "input_params": "{\"command\": \"npm install\", \"timeout\": 60000}",
          "output_result": "added 1423 packages in 45s\n128 packages are looking for funding\nrun `npm fund` for details"
        }
      ],
      "subagent_calls": [
        {
          "subagent_id": "sub-001",
          "subagent_label": "启动 Subagent",
          "subagent_detail": "重构验证逻辑 (总耗时 12s)",
          "total_tokens": "5.8k",
          "total_cost": "$0.15",
          "thinking_rounds": 3,
          "status": "任务完成",
          "thinking_rounds_detail": [
            {
              "round_id": "round-1",
              "round_label": "思考轮次 1",
              "token_badge": "in: 1.5k | out: 0.8k",
              "llm_model": "claude-sonnet-4",
              "llm_duration": "(2.1s)",
              "tool_calls_mini": [
                {"tool_icon": "R", "tool_detail": "Read: src/auth.ts (1.5s)"},
                {"tool_icon": "R", "tool_detail": "Read: src/validate.ts (0.8s)"}
              ]
            }
          ],
          "return_result": "验证逻辑已拆分到 validate.ts，测试通过"
        }
      ]
    }
  ]
}
```

**数据映射：**
- Session 详情部分 → `.session-detail-header`
- `timeline_blocks` → `.timeline-full-track` 中的 `.timeline-block`
- `turns` → `.tool-tree` 中的 `.tree-node`

---

## 二、Problems View API

### 2.1 `/api/problems` - Problem列表

**用途：** 返回异常问题列表。

**请求参数：**
- `severity`: string (可选) - "critical" | "high" | "medium" | "low"
- `range`: string (可选) - "today" | "7d" | "30d"

**响应格式（JSON）：**
```json
{
  "problems": [
    {
      "problem_id": "prob-001",
      "problem_type": "slow_tool: Bash",
      "severity": "critical",
      "session_id_short": "abc-123",
      "agent_source": "Claude Code",
      "time": "2024-01-15 14:32:18"
    },
    {
      "problem_id": "prob-002",
      "problem_type": "repeated_tool: Read (4次失败)",
      "severity": "high",
      "session_id_short": "def-456",
      "agent_source": "Codex",
      "time": "2024-01-15 14:28:05"
    }
  ],
  "severity_counts": {
    "critical": 2
  }
}
```

**数据映射：**
- `problem_type` → `.problem-type` 文本（severity badge 后）
- `severity` → `.severity-badge` CSS class（"severity-critical"等）
- `session_id_short` → `.problem-meta` "Session: XXX"
- `agent_source` → `.problem-meta` "Session: XXX | Agent"
- `time` → `.problem-time`

---

### 2.2 `/api/problems/{problem_id}` - Problem详情

**用途：** 返回 Problem 详细信息，包括推断、建议、关联事件、上下文。

**响应格式（JSON）：**
```json
{
  "problem_id": "prob-001",
  "problem_title": "slow_tool: Bash (45秒)",
  "severity": "critical",
  "time": "2024-01-15 14:32:18",
  "stat_cards": [
    {"label": "执行时间", "value": "45s", "has_error": true},
    {"label": "工具名称", "value": "Bash", "has_error": false},
    {"label": "Session", "value": "abc-123-def", "has_error": false},
    {"label": "Agent", "value": "Claude Code", "has_error": false}
  ],
  "inference": {
    "title": "推断结果",
    "text": "permission prompt 导致用户确认延迟。在 PreToolUse 和 PostToolUse 之间存在 PermissionPrompt 事件，耗时约 40 秒，说明用户需要手动批准 Bash 工具调用。"
  },
  "suggestion": {
    "title": "建议",
    "text": "检查 ~/.claude/settings.json 的 permission 配置，考虑将 Bash 工具设置为 auto-approve 或添加到 allowedTools 列表中。"
  },
  "related_events": [
    {"label": "PreToolUse → Bash", "highlight": false},
    {"label": "PermissionPrompt (40s)", "highlight": true},
    {"label": "PostToolUse → Bash", "highlight": false}
  ],
  "context_preview": {
    "tool_input": "{\"command\": \"npm install\", \"timeout\": 60000}",
    "tool_result": "added 1423 packages in 45s"
  },
  "timeline_events": [
    {"left": "10%", "type": "tool", "title": "PreToolUse: Read"},
    {"left": "25%", "type": "tool", "title": "PostToolUse: Read"},
    {"left": "40%", "type": "tool", "title": "PreToolUse: Bash"},
    {"left": "55%", "type": "anomaly", "title": "PermissionPrompt", "highlight": true},
    {"left": "70%", "type": "error", "title": "slow_tool detected"},
    {"left": "85%", "type": "tool", "title": "PostToolUse: Bash"}
  ],
  "timeline_labels": ["14:30:00", "14:31:00", "14:32:00", "14:33:00"]
}
```

---

## 三、Analysis View API

### 3.1 `/api/analysis/overview` - 顶部统计卡片

**用途：** 返回顶部 4 个统计卡片数据。

**请求参数：**
- `range`: string - "today" | "7d" | "30d"

**响应格式（JSON）：**
```json
{
  "total_tokens": {
    "value": "125,430",
    "trend": "↑ 15% vs 昨日",
    "trend_type": "up"
  },
  "total_cost": {
    "value": "$12.35",
    "trend": "↓ 5% vs 昨日",
    "trend_type": "down",
    "has_color": true
  },
  "cache_saved": {
    "value": "$3.20",
    "trend": "↑ 26% vs 昨日",
    "trend_type": "up",
    "has_color": true
  },
  "anomaly_count": {
    "value": "8",
    "trend": "↓ 3 vs 昨日",
    "trend_type": "down",
    "has_color": true
  }
}
```

---

### 3.2 `/api/analysis/timeline` - Session时间线（消耗时间线）

**用途：** 返回 Analysis 视图中的 Session 时间线列表（带成本、异常标签）。

**请求参数：**
- `range`: string - "today" | "7d" | "30d"

**响应格式（JSON）：**
```json
{
  "summary_stats": {
    "total_tokens": "125,430",
    "total_cost": "$12.35",
    "session_count": "45"
  },
  "timeline_rows": [
    {
      "time": "08:30",
      "session_id": "abc-123-def",
      "agent": "Claude Code",
      "cost": "$1.20",
      "level": "normal",
      "level_text": "中",
      "tag": null,
      "tag_anomaly": false,
      "dot_type": "normal",
      "is_high": false
    },
    {
      "time": "10:00",
      "session_id": "mno-456-pqr",
      "agent": "Claude Code",
      "cost": "$2.35",
      "level": "high",
      "level_text": "🔴 高",
      "tag": "包含 3 Subagent",
      "tag_anomaly": false,
      "dot_type": "high",
      "is_high": true
    },
    {
      "time": "14:00",
      "session_id": "bbb-222-ccc",
      "agent": "Claude Code",
      "cost": "$1.85",
      "level": "anomaly",
      "level_text": "异常",
      "tag": "⚠️ slow_tool 异常",
      "tag_anomaly": true,
      "dot_type": "anomaly",
      "is_high": true
    }
  ]
}
```

---

### 3.3 `/api/analysis/models` - 模型Token分布

**用途：** 返回模型分布柱状图数据。

**响应格式（JSON）：**
```json
{
  "models": [
    {
      "name": "Sonnet",
      "percentage": "45%",
      "height": "96px",
      "bar_class": "model-bar-sonnet"
    },
    {
      "name": "Opus",
      "percentage": "25%",
      "height": "40px",
      "bar_class": "model-bar-opus"
    },
    {
      "name": "Haiku",
      "percentage": "15%",
      "height": "24px",
      "bar_class": "model-bar-haiku"
    },
    {
      "name": "GPT-4",
      "percentage": "15%",
      "height": "24px",
      "bar_class": "model-bar-gpt"
    }
  ],
  "cost_distribution": "Sonnet 成本占比: 35% | Opus 成本占比: 45% (单价更高)"
}
```

---

### 3.4 `/api/analysis/cache` - 缓存效率分析

**用途：** 返回缓存效率统计。

**响应格式（JSON）：**
```json
{
  "cache_stats": [
    {
      "icon": "⚡",
      "value": "32,150",
      "label": "缓存读取 Tokens",
      "stat_class": "saved"
    },
    {
      "icon": "💰",
      "value": "$3.20",
      "label": "节省成本",
      "stat_class": "savings"
    },
    {
      "icon": "📊",
      "value": "26%",
      "label": "缓存命中率",
      "stat_class": "hit-rate"
    },
    {
      "icon": "❌",
      "value": "93,280",
      "label": "未命中 Tokens",
      "stat_class": "miss"
    }
  ]
}
```

---

### 3.5 `/api/analysis/anomalies` - 异常类型分布

**用途：** 返回异常分布饼图数据。

**响应格式（JSON）：**
```json
{
  "total_count": "8",
  "anomaly_types": [
    {"type": "工具失败", "count": 2, "legend_class": "error"},
    {"type": "执行慢速", "count": 2, "legend_class": "slow"},
    {"type": "成本过高", "count": 3, "legend_class": "cost"},
    {"type": "其他异常", "count": 1, "legend_class": "other"}
  ]
}
```

---

### 3.6 `/api/analysis/ttft` - TTFT分布

**用途：** 返回首字节延迟分布数据。

**响应格式（JSON）：**
```json
{
  "ttft_distribution": [
    {"label": "<0.5s", "percentage": "45%", "count": "28", "bar_class": "fast"},
    {"label": "0.5-1s", "percentage": "35%", "count": "22", "bar_class": "normal"},
    {"label": "1-2s", "percentage": "15%", "count": "10", "bar_class": "slow"},
    {"label": ">2s", "percentage": "5%", "count": "3", "bar_class": "very-slow"}
  ],
  "stats": "平均 TTFT: 0.8s | p95: 1.5s | p99: 2.8s"
}
```

---

### 3.7 `/api/analysis/cost-ranking` - 成本归因Top 10

**用途：** 返回成本最高的 Session 列表。

**响应格式（JSON）：**
```json
{
  "cost_ranking": [
    {
      "position": 1,
      "position_class": "top1",
      "session_id": "abc-123-def",
      "meta": "Claude Code | 45 工具调用 | 3 Subagent",
      "cost": "$2.35"
    },
    {
      "position": 2,
      "position_class": "top2",
      "session_id": "ghi-789-jkl",
      "meta": "Claude Code | 32 工具调用",
      "cost": "$1.85"
    },
    {
      "position": 4,
      "position_class": "other",
      "session_id": "xyz-111-aaa",
      "meta": "Copilot CLI | 18 工具调用",
      "cost": "$0.85"
    }
  ],
  "summary": "Top 5 占总成本 55% | 共 105 个 Sessions"
}
```

---

### 3.8 `/api/analysis/tools` - 工具调用详情

**用途：** 返回工具调用统计（热度图）。

**响应格式（JSON）：**
```json
{
  "tool_heatmap": [
    {
      "tool_name": "Read",
      "call_count": "156",
      "success_rate": "98%",
      "avg_time": "1.5s",
      "value_class": "high"
    },
    {
      "tool_name": "Bash",
      "call_count": "78",
      "success_rate": "85%",
      "avg_time": "5.2s",
      "value_class": "high",
      "has_warning": true
    },
    {
      "tool_name": "Agent",
      "call_count": "12",
      "success_rate": "92%",
      "avg_time": "12s",
      "value_class": "low"
    }
  ],
  "bash_detail": {
    "fail_count": "12",
    "timeout_count": "5",
    "user_approved": "28",
    "common_failures": "权限拒绝 (4), 命令不存在 (3), 网络超时 (5)"
  }
}
```

---

### 3.9 `/api/analysis/subagent` - Subagent成本占比

**用途：** 返回 Subagent 成本分布。

**响应格式（JSON）：**
```json
{
  "main_agent": {
    "cost": "$9.26",
    "percentage": "75%",
    "label": "Main Agent: $9.26 (75%)"
  },
  "subagent": {
    "cost": "$3.09",
    "percentage": "25%",
    "label": "Subagent: $3.09 (25%)"
  },
  "stats": {
    "call_count": "12",
    "avg_cost": "$0.26",
    "max_depth": "2"
  }
}
```

---

### 3.10 `/api/analysis/turn-efficiency` - Turn效率分析

**用途：** 返回 Turn 效率统计。

**响应格式（JSON）：**
```json
{
  "turn_efficiency": [
    {
      "label": "平均 Turns/Session",
      "value": "3.2",
      "desc": "理想: 2-4"
    },
    {
      "label": "平均工具/Turn",
      "value": "4.5",
      "desc": "理想: 3-6"
    },
    {
      "label": "输入/输出比",
      "value": "2.8",
      "desc": "理想: 1-2",
      "has_warning": true
    }
  ],
  "warning": "⚠️ 输入/输出比偏高，提示可能有冗余上下文"
}
```

---

### 3.11 `/api/analysis/agents` - Agent性能对比

**用途：** 返回不同 Agent 的性能对比表。

**响应格式（JSON）：**
```json
{
  "agents": [
    {
      "name": "Claude Code",
      "sessions": "45",
      "avg_cost": "$1.20",
      "avg_ttft": "0.8s",
      "errors": "5",
      "has_error_highlight": true
    },
    {
      "name": "Codex",
      "sessions": "28",
      "avg_cost": "$0.85",
      "avg_ttft": "1.2s",
      "errors": "2",
      "has_error_highlight": true
    },
    {
      "name": "Copilot CLI",
      "sessions": "32",
      "avg_cost": "$0.65",
      "avg_ttft": "0.9s",
      "errors": "1",
      "has_error_highlight": false
    }
  ]
}
```

---

## 四、实时通知（已有）

### `/api/hooks/stream` - SSE实时推送

**用途：** 已存在，推送实时异常事件。

**当前实现：** 已有 SSE handler，广播 anomaly 事件。

**前端需要：** 监听 SSE，动态更新 `.notification-dropdown` 内容。

---

## 五、前端 JavaScript 修改策略

**核心原则：**
1. 仅修改 `<script>` 部分，HTML/CSS 完全不变
2. 页面加载时调用 API，动态渲染数据
3. 保持所有交互逻辑（展开/折叠、视图切换）不变
4. 添加加载状态（可选）

**实现步骤：**
1. 添加 API 调用函数
2. 添加数据渲染函数（将 JSON 映射到 DOM）
3. 页面加载时调用 API 并渲染
4. 监听 SSE 更新通知

---

## 六、后端实现策略

**核心原则：**
1. 在 `handler.go` 中添加新的路由
2. 在 `stats.go` 中添加新的 handler 函数
3. 基于 GreptimeDB 表结构编写 SQL 查询
4. 严格匹配上述 JSON 格式返回数据

**实现步骤：**
1. 扩展 `RegisterRoutes` 添加新路由
2. 为每个 API 编写 handler 函数
3. 编写 SQL 查询逻辑
4. 数据格式化（计算成本、格式化 tokens、生成 CSS class）
5. 返回 JSON

---

## 七、验证策略

1. 启动 server，调用每个 API，验证返回格式
2. 用浏览器打开 dashboard，验证渲染效果
3. 逐个视图验证数据正确性
4. 验证交互功能（展开/折叠、点击跳转）

---

## 八、风险与注意事项

1. **数据缺失风险：** GreptimeDB 数据可能不足，API 返回空数组时前端应显示"无数据"
2. **格式匹配风险：** 数字格式化（tokens 显示为 "Xk"、cost 显示为 "$X.XX"）需要精确匹配
3. **CSS class 生成：** "has-anomaly"、"status-completed" 等 class 名称必须精确匹配
4. **时间格式：** "2024-01-15 14:30" 等格式必须精确匹配
5. **前端状态：** 选中状态（`.active` class）需要 JavaScript 动态控制

---

**文档完成。请审核并确认是否开始实施。**