# 技术设计

## 验证层次

先用 registry/service 单元测试锁定模板和职责边界，再用 handler/E2E 验证创建、skill 附着、权限和副本保护；生命周期闭环用脱敏 fixture 和任务/评论交接文件验证。真实模型和生产信号属于显式授权验证，不混入默认套件。

## 闭环契约

```text
failure -> triage -> diagnostician -> fix issue -> regression/QA
        -> immutable release artifact -> human approval -> canary observation
        -> reliability signal -> incident learning -> prevention issue -> regression gate

Agent/Skill/MCP change -> versioned case manifest -> evaluator verdict
        -> code/security/QA/release gate -> same artifact release
        -> cost/latency/safety/tool drift -> new case or rollback recommendation
```

每个阶段输出结构化证据：`facts`, `inferences`, `unknowns`, `owner`, `acceptance_signal`, `stop_condition`, `related_issue`。发布证据额外要求 `artifact_digest`, `baseline`, `window`, `signals`, `decision`, `rollback_outcome`；Agent 评测额外要求组件版本、case manifest、轨迹脱敏、正确性/安全/成本/延迟/漂移分项。

## 实现边界

- 复用现有 built-in role/skill/squad registry、issue/comment API 和模板 copy semantics。
- 事故学习、canary、RCA 和 Agent evaluator 先交付为可执行方法和交接 fixture；不假造后台自动触发器。
- 体验/迁移角色先通过 workload gate 产出结论；只有证据达标才新增 listed roster、四语文案、权限、skill 和测试。
- MCP catalog 仍保持 keyless；任何带 token/secret/API key 的连接必须先有 workspace 管理员配置和审计协议。

## 回滚

模板注册表变更可通过恢复 registry、instructions、skill source 和测试回退；已创建的 workspace 副本不自动删除。若将来引入真实事件持久化，必须另建迁移设计和审批，不在本任务中隐式扩展。
