[根目录](../CLAUDE.md) > **docs**

# docs 模块

> 设计文档、规划文档、API 规范文档。

## 变更记录 (Changelog)

| 时间 | 变更内容 |
|------|----------|
| 2026-05-24 11:13:31 | 初始化模块文档 |

---

## 模块职责

docs 模块存放项目的所有设计文档、规划文档和 API 规范，为开发者提供：

1. **系统架构文档**：完整的架构设计、数据模型、API 设计
2. **规划文档**：功能开发的阶段性规划
3. **API 规范**：Dashboard API 的详细设计
4. **配置示例**：Hook 配置示例

---

## 目录结构

```
docs/
├── design/
│   └── 2024-01-15-llm-apm-design.md    # 系统设计文档（核心）
├── superpowers/
│   ├── plans/
│   │   ├── 2024-01-15-plan1-core-backend.md
│   │   ├── 2024-01-15-plan2-analysis-engine.md
│   │   ├── 2024-01-15-plan3-analysis-view.md
│   │   └── 2026-05-22-dashboard-api-implementation.md
│   └── specs/
│       └── 2026-05-22-api-design-for-dashboard.md
└── hooks-config.json                   # Hook 配置示例
```

---

## 关键文档

### 设计文档

**`docs/design/2024-01-15-llm-apm-design.md`**

这是项目的核心设计文档，包含：

- **项目概述**：目标、支持的 Agent、差异化定位
- **架构设计**：系统架构图、模块职责
- **数据模型**：4 张核心表的结构和索引
- **Turn 边界界定**：Turn 定义、边界界定方案、Subagent 处理
- **Analysis Engine**：9 种异常检测规则、根因推断逻辑、SSE 推送筛选
- **Dashboard UX**：三大主视图、执行树组织、一键跳转机制
- **技术栈**：Go + GreptimeDB + 原生 JS
- **API Endpoints**：完整的 API 列表
- **配置项**：所有环境变量
- **多租户扩展预留**：数据模型、API、Dashboard 的扩展点
- **后续扩展**：未来功能规划

### 规划文档

**`docs/superpowers/plans/`**

- `plan1-core-backend.md` - 核心后端开发计划
- `plan2-analysis-engine.md` - 分析引擎开发计划
- `plan3-analysis-view.md` - Analysis View 开发计划
- `dashboard-api-implementation.md` - Dashboard API 实现计划

### API 规范

**`docs/superpowers/specs/2026-05-22-api-design-for-dashboard.md`**

Dashboard API 的详细设计规范。

### 配置示例

**`docs/hooks-config.json`**

Claude Code Hook 配置示例，包含所有 Hook 事件类型：
- `PreToolUse`
- `PostToolUse`
- `PostToolUseFailure`
- `UserPromptSubmit`
- `AssistantResponse`
- `SessionStart`
- `SessionEnd`

---

## 常见问题 (FAQ)

### Q: 如何使用设计文档？

推荐阅读顺序：
1. 先读 `§1 项目概述` 和 `§2 架构设计`
2. 再读 `§3 数据模型` 和 `§4 Turn 边界界定`
3. 然后读 `§5 Analysis Engine` 理解异常检测
4. 最后读 `§6 Dashboard UX` 了解前端需求

### Q: 如何配置 Hook？

1. 参考 `hooks-config.json` 的示例
2. 将配置添加到 `~/.claude/settings.json`（全局）或 `.claude/settings.json`（项目级）
3. 确保 APM Server 在 `http://127.0.0.1:14318` 运行

### Q: 如何理解多租户扩展？

详见设计文档 `§10 多租户扩展预留`，包含：
- 数据模型预留（`tenant_id` 字段）
- API 扩展预留（认证、tenant_id 自动注入）
- Dashboard 扩展预留（登录、租户选择）
- 存储扩展预留（租户级配额）
- 迁移路径

---

## 相关文件清单

| 文件 | 内容 |
|------|------|
| `design/2024-01-15-llm-apm-design.md` | 系统架构、数据模型、API 设计（11 章） |
| `hooks-config.json` | Hook 配置示例 |
| `superpowers/plans/*.md` | 开发计划文档 |
| `superpowers/specs/*.md` | API 规范文档 |

---

## 覆盖率

- 设计文档：完整（11 章，428 行）
- Hook 配置：完整（7 种事件类型）
- 规划文档：完整（4 个阶段计划）
- API 规范：完整

---

## 下一步建议

1. **更新设计文档**：如果新增功能，同步更新设计文档
2. **添加 API 文档**：可考虑使用 Swagger/OpenAPI 规范化 API 定义
3. **补充部署文档**：添加生产部署指南