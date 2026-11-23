# M1 UI/CLI 自动完成提示 PRD

状态：规划中，仅挂在既有父任务 `11-05-agent-squad-rnd-improvements` 下。本文件不创建新 Trellis 任务，不执行 `task.py start`，不改变产品实现。

基线事实源为当前 HEAD `45ec006d96445b41d02c5d26acd5b8c574ea0289`、分支 `codex/cross-workspace-collaboration`、已归档的完成事务强制门禁 v1 提交 `18b743291`，以及：

- `POST /api/issues/{id}/delivery-evidence/completion-hint`
- 请求体严格为 `expected_issue_revision`、`target_status`、`evaluation_key` 三个字段；本轮 Web/Desktop/CLI 始终发送 `evaluation_key: null`
- 所有成功响应仍为 `enforced=false`、`usable_for_completion=false`
- `off`、无项目、非 done 类别目标返回 `not_applicable`
- `observe + done + null key` 返回 `attention/evaluation_required`
- `matching` 只代表这次读取事务仍匹配，不能作为完成许可
- 缺失、过期、撤权、权限变化、锁/数据库/提交不确定性仍是拒绝或错误

## 目标

在 Web/Desktop 共享 UI 和 CLI 的普通单 issue 完成入口上调用现有提示 API，让用户在真正写入前看到当前交付证据状态，同时保持完成写入、授权消费和 v1 事务门禁由服务端决定。客户端必须把“提示结果”“实际写入结果”“未知结果”分开表达。

## 本轮明确覆盖

完成事件定义为：当前 issue 不在 `done` 类别，目标 status 属于 `done` 类别；目标可以是内置 `done`，也可以是当前状态目录中 active 的 custom done。请求必须使用当前 issue revision 和用户选中的精确 status key。

Web/Desktop 共享入口：

1. issue detail 侧栏的 `StatusPicker`。
2. issue detail 顶部“标记为已完成”按钮；`onDone` 只能在实际 PUT 成功后触发。
3. issue actions dropdown/context menu 的 status 子菜单。
4. list/board/swimlane/table 中通过共享 `IssueSurfaceActions.updateIssue` 的单 issue status picker、右键菜单和行内状态编辑。
5. issue detail 的普通 sub-issue 行状态 picker（`SubIssueRow`）。这些入口都必须经过同一个 completion-hint 协调器，避免某个表面绕过提示。

CLI 入口：

1. `multica issue status <issue-id> <status>`。
2. `multica issue update <issue-id> --status <status>`，仅在 status-only 请求中纳入；它与 status 命令共享同一提示逻辑。
3. built-in done 与 active custom done 使用同一 category 规则。CLI 先取得 issue revision，再发提示；不能依赖旧的 `resolvedID` 只提供 UUID 的结果来伪造 revision。

## 入口盘点结论

本轮只读盘点确认了以下事实：

- `packages/core/api/client.ts` 当前有 `createIssue`、`createCommentSubIssue`、`updateIssue`、`moveIssue`、`batchUpdateIssues`，没有 completion-hint client method；`updateIssue`/`moveIssue` 仍返回原 Issue，batch 已有 partial/unknown 结果协议。
- `packages/core/api/schemas.ts` 的 `IssueSchema` 已将 `revision`、`status_category` 作为兼容可选字段；没有 completion-hint response schema。新 schema 必须复用 `parseWithFallback`，不能把网络 JSON cast 成 DTO。
- `packages/core/issues/mutations.ts` 的 `useUpdateIssue` 负责单 issue optimistic cache、rollback 和 settle；`useBatchUpdateIssues` 负责逐项结果/部分提交恢复；create 与 comment-sub-issue mutation 是独立路径。本轮 gate 只包住单 issue status-only mutation，不能让 hint 进入 batch 或 create fan-out。
- `packages/views/issues/actions/issue-actions-menu-items.tsx` 的 status 菜单统一调用 `useIssueActions.updateField`；`packages/views/issues/actions/use-issue-actions.ts` 还负责 run-confirm、surface action fallback、toast 和导航，是 detail/context-menu 单 issue 的共享入口。
- `packages/views/issues/components/issue-detail.tsx` 同时包含 detail `StatusPicker`、顶部 mark-done、普通 `SubIssueRow` 的直接 `useUpdateIssue`、comment-to-child opener；只有前 3 个 status-only 普通 issue 入口纳入，comment-to-child 创建排除。
- `packages/views/issues/components/batch-action-toolbar.tsx` 直接调用 `useBatchUpdateIssues`/surface batch；它必须保持现有逐项 partial/unknown 合同，不在本轮调用一个共享 hint。
- list/board/swimlane/table 通过 `IssueSurfaceActions` 和 `TableView` 的 inline status editor 写普通 issue；这些 status-only PUT 纳入。拖拽通过 `moveIssue`，因同时改变相对 position，明确排除。
- create modal / quick-create / comment-sub-issue 允许设置初始 status，但没有持久 issue revision；enforce 下 create-as-done 的拒绝由服务端负责，本轮不伪造 preflight。

CLI 盘点确认：

- `server/cmd/multica/cmd_issue.go` 的 `runIssueStatus` 和 `runIssueUpdate` 都最终 PUT `/api/issues/{id}`；`runIssueUpdate` 在 status-only 时可以复用提示。
- `runIssueCreate` 直接 POST `/api/issues`，`runIssueCommentAdd` 只写 comment；二者不是本轮 hint 目标。
- 当前 `cmd_issue.go` 没有用户级 `issue batch` 命令；server batch endpoint 与 Web toolbar 仍按 v1 partial/unknown 合同独立处理。
- `resolveIssueRef` 对 full UUID 只返回 id，对 issue key 会读取一次但丢弃 revision；提示实现必须另取完整 issue，不能把 id resolution 当作 revision 证明。

## 本轮明确不覆盖

- `POST /api/issues/{id}/move` 以及 board/list/swimlane 拖拽换列。它同时承担相对排序和 move writer，需要独立的拖拽前置检查、取消和 optimistic mirror 合同。
- `POST /api/issues/batch-update`、`BatchActionToolbar` 及任何批量 status。v1 的逐项 partial/unknown 协议保持现状，本轮不把一次 hint 共享给多个 issue；当前 CLI 也没有用户级 issue batch 子命令。
- `POST /api/issues` 创建 issue、create modal 的初始 done/custom done、评论转子任务和 `createCommentSubIssue`。enforce 下创建即 done 本来就拒绝，且新 issue 没有自身 revision/evidence，不能把父 issue 或 hint 借作许可。
- 带 title/description/assignee/project/parent/attachment 等其他字段的混合 `--status` 更新；服务端 v1 要求先保存实质变更，再单独完成。本轮不重写旧混合更新协议。
- comment add/update、assign、reorder、delete、项目完成状态、GitHub/VCS、独立协作、R38 管理完成和真实供应商验收。
- 新的 completion authorization UI、evaluation key 获取/共享、skip、自动授权、自动重试或任何完成门禁核心改动。

## 用户可观察结果

Web/Desktop：

- `not_applicable`：不显示提示，按原入口执行原 status mutation。
- `attention`：打开共享确认提示，明确“当前证据需要关注；此提示只描述本次读取，不授予完成权限”。用户选择取消时不发完成 PUT；选择继续后才发原 status-only mutation。
- `matching`：显示“当前观察仍匹配，但这不是完成许可”，同样不能直接显示成功；用户明确继续后才写入。当前 UI/CLI 默认传 null key，因此主路径通常是 `attention`，该分支仍由协议测试覆盖。
- hint 端点在旧服务上返回 404/405：仅当响应没有结构化资源/权限/交付 code 时视为旧后端不支持提示，继续原 mutation，保持安装版 Desktop 与旧 server 兼容；结构化 404/403/409 仍是拒绝或刷新。
- hint 的 409/403/其他明确拒绝：不发送 mutation，显示可操作的拒绝/刷新文案。
- 网络、5xx、响应畸形、提交不确定性：不自动继续，不显示完成成功；提供显式重试或取消。重试使用新的当前 issue revision，不能自动重放原完成写入。
- 实际 status mutation 的成功/失败仍由原 mutation 回调决定；提示响应永远不能触发成功 toast、导航或 cache commit。

CLI：

- hint `attention`/`matching` 只写到 stderr 作为警示，然后继续原 status-only PUT；只有 PUT 2xx 才打印“status changed”成功信息，JSON 输出仍只写 stdout。
- hint 明确拒绝或未知错误时退出非零且不发 PUT；stderr 必须区分“提示拒绝”和“无法确认提示”，不能把未知当成功。
- 旧服务 404/405 时绕过提示并保持原 status/update 行为；CLI 不自动重试 hint，Ctrl-C 在 hint 阶段只取消读取，不会发送 PUT。
- 实际 PUT 返回 428、409、403、5xx 或传输未知时，沿用 CLI 现有错误分类；不得因为 hint 曾返回 attention/matching 而显示成功。

## 兼容与安全约束

- `evaluation_key` 不由 UI/CLI 生成、读取、持久化、打印或转交；本轮不扩大 private evaluation 可见性。
- 不添加 completion binding、operation id、authorization id 或 skip 字段到旧 status/update 请求；v1 writer 仍是唯一授权和拒绝来源。
- 没有 revision、status category 或新端点的旧后端以兼容路径继续原行为；兼容降级只适用于提示读取，不得绕过服务端完成门禁。
- 不缓存 hint；服务端 `Cache-Control: no-store` 保持不变，客户端只保存当前交互中的短暂结果。
- 同一用户重复点击只能产生一次进行中的 hint 和一次后续 mutation；取消、卸载和失败都释放单飞状态。

## 产品验收结论

本 PRD 只规划“完成前提示消费者”，不代表 M1 完成，不改变 `full_delivery_gate_complete=true` 的 v1 事实，也不关闭父任务、真实供应商验收、M2 或 M3。
