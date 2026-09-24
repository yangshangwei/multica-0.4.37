# 技术设计

## API

新增 `POST /api/issues/{id}/lifecycle-handoffs`。请求包含 `kind`（`rca`、`incident-learning`、`rollout`、`agent-evaluation`）和对应证据字段。源 issue 通过现有 `loadIssueForUser` 解析，所有写入使用其 workspace。

## 持久化

- `lifecycle_handoff`：源 issue metadata 中的结构化 JSON，保存 kind、decision、source、follow-up、evidence 和 recorded_at。
- RCA/incident-learning 的后续 issue 用 `IssueService.Create` 创建，`ParentIssueID` 指向源 issue；已有 follow-up/prevention issue 先校验同 workspace 后复用。
- 结构化评论只做可读审计摘要；assigned agent/squad 仍由现有 IssueService/TaskService 负责排队和去重。

## 安全与门禁

所有决策函数 fail-closed。API 不执行 rollback、生产发布或真实模型调用；`rollback-recommendation`、`hold`、`unknown` 均由人工/现有 release 流程继续处理。

## 幂等

incident-learning 首选源 issue metadata 中已记录的 prevention issue；首次创建复用 `IssueService` 的 workspace/parent/title active-duplicate guard，避免并发重试产生第二个 prevention issue。
