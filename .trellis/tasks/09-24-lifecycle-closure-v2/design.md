# 技术设计

## 边界

本任务只补生命周期交接的运行时可验证边界，不建立新的生产控制面。复用现有 `IssueService.Create`、`Handler.CreateComment`、squad leader enqueue 和模板 copy semantics。结构化生命周期证据由一个无副作用的 service-level contract validator 校验；创建 follow-up 仍通过现有 issue 创建入口完成，避免引入第二套任务存储。

## 交接模型

```text
source issue
  -> lifecycle handoff comment (facts / decision / related issue)
  -> independent follow-up issue (parent or explicit relation)
  -> assignee/squad leader task queue
  -> downstream comment or completion evidence
```

RCA 路由由 `cause_state` 和 `route` 决定：bug-fix 的 `known` 可以 bypass，但必须有 reason；maintenance 的 `unknown` 创建 diagnostician follow-up；incident 先创建 mitigation/recovery 交接，随后创建独立 RCA，不等待 RCA 完成主事故。

Incident-learning 与 rollout/canary 使用同一份证据结构校验器，分别要求 prevention owner/acceptance signal 和 digest/baseline/window/signals/rollback decision。任何缺证据状态都返回 `unknown` 或 `hold`，不会触发生产副作用。

Agent evaluator 消费版本化 manifest；case 必须同时带 category、trace、stop reason 和 result。安全边界、工具失败、成本、延迟和 drift 作为独立 case，不允许只凭 happy path 通过。

## 兼容与风险

- 不改变旧 issue/comment API 的字段和行为；新 contract 仅在显式交接路径调用。
- 复用现有 workspace/permission 校验和 pending-task 去重；follow-up 创建失败时原 issue 不被标记完成。
- 不把 fixture 当作后台自动化证据：至少一条集成测试要观察真实 issue、comment 和 queue 行。
- 不新增 MCP 凭据注入；Git/CI、observability、database 连接继续以未来只读审计协议为前提。

## 回滚

新 validator 和测试可删除；若后续接入自动 follow-up 创建，必须以独立迁移/接口设计和人工审批为前提。模板版本升级不删除已有 workspace 副本。
