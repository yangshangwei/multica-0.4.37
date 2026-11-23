# Collaboration documentation coverage audit

Audited current working-tree code on 2026-11-07. This is a read-only semantic review, not a runtime acceptance test. Existing uncommitted implementation is included as the current source of truth. Only this research file was written.

## Main finding

The embedded Chinese documentation has no page for **协作空间** or its end-to-end cross-workspace workflow. `apps/docs/content/docs/meta.zh.json` contains ordinary workspace comments/chat/squads but no collaboration page. The only operational collaboration mentions are memory size and directory settings in `environment-variables.zh.mdx`. This leaves a large implemented product surface undocumented.

Recommended additions: `collaboration.zh.mdx` for entry, human workflow, AI resources, delivery and recovery; `collaboration-context.zh.mdx` for shared context and knowledge lifecycle. A single sufficiently structured page is also viable. Register every added page in `meta.zh.json`, link from index/concepts/workspaces and regenerate the embedded bundle.

## Verified entry and creation

- Global navigation switches between 工作空间 and 协作空间: `packages/views/layout/global-scope-navigation.tsx:16`.
- The account destination is `/account`, independent of workspace membership: `packages/core/paths/account.ts:1`. It is one account-level destination, not another workspace.
- Main lists are 当前处理 and 我参与的; toolbar provides 新建协作任务 and 通知: `packages/views/collaboration/account-workbench-page.tsx:46`.
- List search only covers loaded items, with 加载更多 to extend coverage: same file `:92` and `:115`.
- Creation selects a source workspace, recipient (including self), title/description, initial task fields and optional attachments. It creates a task in the source workspace; it does not move a task into the recipient's workspace. See `packages/core/collaboration/creation.ts:52`, `server/internal/service/collaboration.go:132`, `:248` and `:301`.
- Creation requires membership in a source workspace and an enabled collaboration creation gate. Users without a workspace can still access already-shared tasks. Copy is in `packages/views/locales/zh-Hans/collaboration.json` under `creation.noSources` and service membership enforcement is `server/internal/service/collaboration.go:248`.
- Recipient selection is within this Server, not cross-Server federation. `MULTICA_COLLABORATION_DIRECTORY_ENABLED=false` narrows browsing to related accounts; exact username/email/account-ID lookup remains available (already documented in `environment-variables.zh.mdx:134`). Do not describe arbitrary external-server invitations.
- Creation allows 手动处理 or AI 辅助. Manual policy does not admit new AI input/execution; changing back to assisted does not automatically restore suspended execution. UI labels: `collaboration.json` `policy` section; submit gate: `server/internal/collaboration/access.go:98`.

## Human collaboration and delivery

The normal walkthrough should be: open 协作空间 → create/select task → discuss and share files → 开始处理 / 转交 / 请求补充信息 → 返回信息 → 提交结果 → initiator 验收并完成 or 退回修改. Self-assigned initiators get 记录结果并完成.

Evidence:

- Visible detail: title, stage, current handler, description, 交付结果, comments, AI execution: `packages/views/collaboration/collaboration-detail.tsx:78`.
- Task资料 toolbar opens a right drawer for details, attachments, 协作上下文, 知识候选, children and activity: `packages/views/collaboration/detail-auxiliary.tsx:18`, `:41`, `:72`, `:99`. Avoid describing these as always-visible inline sections.
- Delivery operations and labels: `packages/views/collaboration/workflow.tsx`, `packages/views/locales/zh-Hans/collaboration.json` `delivery` and `workflow` sections. Results contain result text, basis and optional evidence files; history retains submissions and decisions.
- Pending acceptance restricts transfer/status edits; capabilities are derived from the current submission and state: `server/internal/service/collaboration_delivery.go:365`.
- Successful AI execution and task acceptance are distinct. Do not promise successful runs automatically complete the human task.
- Attachments have a 100 MiB per-file limit and at most 32 selected files: `server/internal/service/collaboration_attachments.go:28`. Uploaded drafts remain private until selected files are delivered. Removing a comment/reference does not delete already-delivered files (`collaboration.json` `attachments.retained`).
- For an uncertain request outcome, use 查询原结果 / 重试原请求. Do not tell users to create a fresh duplicate operation. Original-key recovery is implemented in `packages/core/collaboration/creation.ts:61`, `control.ts:72` and exposed in `workflow.tsx`, `comment-composer.tsx:178`.

## Permissions that documentation must preserve

Canonical evidence: `server/internal/collaboration/access.go:82`.

| User relationship | Current contract |
| --- | --- |
| Active source workspace member or active task participant | Can read, comment, react and resolve discussions |
| Source member who is not a task participant | Can read/discuss but cannot submit AI commands or control shared AI |
| Active participant | Can control AI; can submit AI commands only for an open task with assisted policy |
| Initiator or current handler who remains a participant | Can invite, transfer and edit; source structure edits additionally require source membership |
| Initiator | Removes participants, changes AI policy, completes/cancels/reopens |
| Current information recipient/handler | Returns requested information |
| Handler other than initiator | Submits for acceptance |
| Participant who is neither initiator nor current handler | Can leave task |

Capabilities depend on current task state; the table is not a promise that every action is always enabled. Joining a shared task does not grant access to the whole source workspace or private resource workspace. A copied task link does not itself grant access. Removing a participant revokes task/resource authority, cancels their waiting commands and requests stopping active work; the UI explicitly describes this in the `participants` locale section.

## Shared AI setup and execution

Recommended minimal walkthrough:

1. In task AI 执行, open 管理 AI 资源.
2. Invite resource owners as participants, then register an available agent or AI小队. Alternatively an owner 提供资源 and the initiator/current handler 采纳资源.
3. Each owner confirms their own resource authorization. Registration/offer/adoption alone does not authorize execution.
4. Explicitly 恢复目标 or 恢复全部 after authorization.
5. Use 提及 to select a registered AI target in a comment, inspect 预览发送 and the AI impact, then confirm sending. Ordinary comments and `@all` must not be described as unconditional AI triggers.
6. Read queued commands, execution group/runs, preparation/results and stop status in AI 执行; cancellation of a waiting command differs from stopping an active target.

Evidence:

- Resource panel and owner-specific consent buttons: `packages/views/collaboration/resource-panel.tsx:18`, `:27`, `:64`, `:69`.
- Setup/ownership/consent/adoption copy: `packages/views/locales/zh-Hans/collaboration.json` `resources` and `host` sections.
- Explicit AI preview/second send for targets: `packages/views/collaboration/comment-composer.tsx:74`, `:97`, `:132`, `:162`.
- Stop/resume actions and lane scope: `packages/views/collaboration/execution-panel.tsx:90`, `:93`, `:118`.
- Waiting command cancellation only by command author, initiator or current handler: same file `:126`.
- Config changes may require reauthorization or a switch proposal. Switch commits require relevant owner consents and old execution termination; the new configuration stays paused until resumed. Do not tell users changing the ordinary agent settings transparently switches all running collaborative sessions.

Execution boundary must be explicit in user-facing explanation: 普通本机执行 uses the selected runtime's existing AI account/tool permissions, with a separate session/workdir. The workdir is **not** an OS sandbox. 停止请求已受理 does not prove execution stopped, especially with an offline runtime. Managed-process cleanup does not guarantee detached background programs were cleared. Canonical wording is already in the `host` locale section and UI `execution-panel.tsx:91`.

## Shared context and knowledge

### 协作上下文

- One current Markdown document per collaboration task records current findings/decisions/next steps across machines/providers; detailed chronological discussion remains in comments.
- Open the 协作上下文 drawer. Active participants with AI control can edit/restore while the task is open; source-only viewers cannot. A closed task's document is read-only.
- Concurrent edits require reviewing the new version and merging; do not overwrite silently.
- Restore creates a new version and retains later history. Clearing a historical version's body is irreversible, keeps metadata, and cannot clear the current version. It is initiator-only in normal task UI. An administrator recovery path has separate restrictions and must not be advertised as general admin access.
- Service authority and state: `server/internal/service/collaboration_memory.go:277`, `:295`, `:299`, `:321`, `:353`. UI: `packages/views/collaboration/memory-panel.tsx`; labels: `collaboration.json` `memory`.
- Existing `environment-variables.zh.mdx:45` correctly describes default 128 KiB and configured limit, but needs a link to the actual usage guide.

### 知识候选 / 知识贡献

- Open task 知识候选; initiator/current handler can propose a reusable conclusion with exact source versions from memory/comment/run-records and a target project/workspace.
- Set applicability and optional expiry. Confirmation is a human governance decision, not proof that the author's validation claim is true or that a run has used it.
- Target workspace owner/admin, or a human project lead for project knowledge, can approve/reject while retaining source read authority. See `server/internal/collaboration/knowledge.go:28` and `server/internal/service/collaboration_knowledge.go:415`.
- Approved contributions are distinct from manually edited workspace/project context. Manage them in workspace general settings (`packages/views/settings/components/workspace-tab.tsx:542`) and project detail (`packages/views/projects/components/project-detail.tsx:541`). Revision proposes a new version; old approved content remains until replacement is confirmed. Withdrawal/expiry/source access changes affect future eligibility.
- **Missing deployment setting:** `MULTICA_KNOWLEDGE_CONTEXT_ENABLED` is absent from the Chinese env reference. `server/cmd/server/router.go:431` enables it only for exact `true`; default is disabled. Candidate creation/approval does not mean automatic runtime injection.
- Runtime use also requires a supported runtime/provider/mode and current permissions. Collaborative runtime preparation has explicit protocol, local-mode and no-custom-profile restrictions: `server/internal/service/collaboration_knowledge_runtime.go:52`, `:66`, `:73`. Keep docs phrased as conditional usage; do not claim all agents automatically receive approved knowledge.

## Existing inaccurate pages to amend

1. `concepts.zh.mdx:12` says all work/config happens inside one workspace and `:60` says workspace is the container of everything. Describe source ownership plus task-scoped cross-workspace participation; introduce 协作空间 separately.
2. `concepts.zh.mdx:65` says all human notifications go to 收件箱. Collaboration has its own 通知 entry in account workbench (`account-workbench-page.tsx:65`). Qualify the ordinary-workspace rule and link the collaboration guide.
3. `workspaces.zh.mdx:8` says all data is completely isolated. Clarify explicit per-task sharing rather than implying shared participation grants workspace membership.
4. `workspaces.zh.mdx:51` promises owners can delete all workspace content. Current service refuses workspace deletion with retained collaboration records: `server/internal/handler/workspace.go:1150` returns `workspace_has_collaboration_records` (409). Document this limitation without suggesting destructive cleanup.
5. Index, ordinary issue/comment/agent/squad/desktop pages should link to the new guide and explicitly distinguish ordinary-workspace actions from collaboration-task controls. Ordinary assignment/squad triggers should not be copied into collaboration instructions without checking shared resource admission.

## Validation boundary

These findings are supported by current source and UI labels. No live server or real AI execution was invoked. No statements here establish that the screenshot's installed desktop/server already contains all working-tree code. The main audit should preserve the distinction between implemented source capabilities and deployed-version availability.
