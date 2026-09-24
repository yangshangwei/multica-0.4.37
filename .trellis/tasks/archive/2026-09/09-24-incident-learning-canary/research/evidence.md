# 事故学习与发布后验证证据

日期：2026-09-24

## 通过

- `go test ./internal/service -run 'Test(ValidateIncidentLearning|EvaluateRolloutEvidence)' -count=1`：通过。
- `go test ./internal/handler -run 'TestCreateLifecycleHandoff(IncidentLearning|Rollout|FullDevelopmentLoop|PreservesBoundedHistory)' -count=1`：通过。
- handler runtime fixture 覆盖事实/推断/未知、owner/acceptance signal、重复请求复用 prevention issue、已有 prevention 关联、digest mismatch、缺 baseline、观察窗未结束、超阈值 rollback recommendation/hold。
- `TestCreateLifecycleHandoffFullDevelopmentLoop` 验证同一 source issue 的 `rca -> rollout -> incident-learning` 顺序、prevention source linkage 和 bounded history。

## 边界

- 发布后验证只消费脱敏 fixture，不连接生产 Observability，也不执行回滚、通知或发布。
- 同一 artifact digest 和人工审批边界由 validator 保持；重建产物、缺证据和窗口未完成均 fail-closed。
