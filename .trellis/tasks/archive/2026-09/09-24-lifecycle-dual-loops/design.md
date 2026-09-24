# 技术设计

## 边界

本任务把生命周期能力落在现有 built-in role/skill/squad/autopilot/MCP 模板体系中，不引入新的 agent 实体类型、数据库表或生产控制面。模板创建仍然是复制：后续模板版本不得覆盖 workspace 副本。

## 两条闭环

```text
研发闭环: 观测失败 -> 分诊/诊断 -> 修复 -> 回归验证 -> 发布审批 -> 同产物发布后验证
                         \-> 事故止血 -> 脱敏复盘 -> 预防任务 -> 回归/门禁
Agent 闭环: Agent/Skill/MCP 变更 -> 版本化评测集 -> 人工门禁 -> 发布 -> 轨迹/成本/延迟/安全漂移 -> 新评测或回滚建议
```

## 角色策略

首批只增加 `reliability-engineer` 和 `agent-evaluator`，因为两者有稳定、互不重叠的证据产物。`experience-validation-engineer` 与 `migration-reviewer` 先受 workload gate 控制：未达到阈值只提供 skill/路由；达到阈值后再加入 listed roster。契约兼容、威胁建模、供应链、产品结果、灾备先作为 skill 或 squad 路由，不按技术栈拆角色。

## 权限与兼容

- Observer 角色只读分析；Contributor 可在隔离分支写测试/文档；Operator 仍要求每个生产动作单独人工审批。
- skill 输出使用证据表：事实、推断、未知、负责人、验收信号、停止条件。
- MCP 继续采用无凭据内置模板；外部服务需要 workspace 管理员配置，运行时不得从模板读取秘密。
- 现有 API、桌面旧版本和插件响应遵守 `parseWithFallback` 兼容规则；数据库迁移遵守无 FK/级联、`CREATE INDEX CONCURRENTLY` 单语句规则。

## 小队与 MCP

推荐把 incident learning/canary、reliability review、Agent quality gate、migration review 和 experience validation 作为可组合的路由，而不是为每个主题增加一个 coordinator。内置 MCP 继续只提供无凭据的浏览器和推理工具；Git/CI、observability 和 database MCP 只有在版本固定、最小权限、脱敏和审批协议明确后才进入 catalog。

## 验证面

模板单元测试锁定 roster、skill 附着、权限、本地化和副本保护；Go 集成测试验证 squad 路由；浏览器 E2E 验证模板创建；真实模型评测只在显式授权环境运行，原始轨迹脱敏保存。
