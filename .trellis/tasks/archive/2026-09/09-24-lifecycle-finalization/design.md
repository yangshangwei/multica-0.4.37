# 技术设计

## 边界

复用现有内置角色、role skill、squad template 和 issue/comment API。生产代码只在发现当前路由或模板契约无法表达验收证据时做最小修改；不引入新的持久化模型、观测控制面或凭据型 MCP。

## 数据流

```text
failure -> triage -> RCA comment -> separate repair issue
        -> immutable artifact + approval -> same-digest observation
        -> incident learning -> prevention/regression gate

Agent/Skill/MCP change -> versioned case manifest -> evaluator verdict
        -> code/security/QA/release gate -> approved artifact
        -> cost/latency/safety/tool drift -> hold/new case/prevention issue
```

RCA、事故学习、发布验证和 Agent 评测都采用脱敏 JSON fixture 作为离线合约证据。fixture 只证明阶段产物可以被下一阶段消费，不伪装成后台自动调度器。

## 专项角色判定

先从两个最近发布周期、迁移记录和 E2E/浏览器证据计算 gate，再决定是否新增 listed role。体验角色要求每周期至少两条真实浏览器/可访问性/跨端关键路径并有 QA 覆盖缺口；迁移角色要求每周期至少一项 schema/API/客户端迁移并有独立兼容窗口、校验和回滚交付物。任一条件未达标时只保留现有角色组合和负向 roster 断言。

## 安全与审批

Reliability Engineer、Agent Evaluator、专项角色均不具备 Operator 权限。生产发布、回滚、迁移、恢复和通知仍由 Release Engineer 在人工审批后执行；缺 baseline、真实轨迹、同 digest 或观察窗口的结论只能是 `unknown` 或 hold。
