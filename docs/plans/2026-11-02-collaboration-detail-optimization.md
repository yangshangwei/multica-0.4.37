# Collaboration Detail Optimization Plan

**Goal:** Make the collaboration detail page explain the requested work, current responsibility, delivered result, and the viewer's next action, with a complete human delivery and acceptance workflow.

**Architecture:** Extend the existing collaboration issue and handoff workflow. Keep one authoritative task across the source workspace and account collaboration entries; put shared behavior in Core and shared Web/Desktop presentation in Views. Separate business progress, human responsibility, and optional AI execution.

**Tech Stack:** Go services and PostgreSQL/sqlc; TypeScript, React, React Query, existing Zustand draft stores, shadcn/Base UI, Vitest, and Playwright.

**Status:** Planning handoff based on a read-only review on 2026-11-02. This document defines product scope, recommended contracts, execution order, and acceptance criteria. It does not claim a completed technical design or verified runtime behavior. No Trellis task was created in the review session.

**For the next Codex session:** Read `CLAUDE.md` and load Trellis context. Use this document to prepare `prd.md`, `design.md`, and `implement.md` for the relevant task before starting implementation. Follow the project Trellis implement/check dispatch and review gates. The user's new-session prompt can authorize task creation; consent from this review session must not be inferred.

## 1. Review context and established facts

The reviewed task was an external collaborator's export-field reconciliation request. Its description asked for a comment with the conclusion and said AI would not be used. The screen showed `待办`, the external collaborator as current handler, a conclusion in a comment, AI queue controls, empty source fields, empty collaboration memory and knowledge candidates, and generic activity entries before the comments.

The central usability problem is that task properties and auxiliary tools precede the task's business outcome. The central contract problem is that a delivery action exists, but its content and acceptance evidence are weakly constrained.

| Verified behavior | Consequence for this plan |
| --- | --- |
| `submit_acceptance` already changes the status to `in_review` and transfers responsibility to the initiator. | Build on this operation; do not introduce a parallel submission workflow. |
| Only the initiator can complete, cancel, or reopen the whole collaboration issue. | Preserve the authority boundary in UI, API, and other callers. |
| Ordinary comments do not hand off responsibility or submit acceptance. | A conclusion in a comment does not prove formal delivery or acceptance. |
| Submission has a note, but that note may be empty. | Require useful delivery content and present the submission as a result. |
| Completion can currently happen from any nonterminal status. Generic status changes can enter `in_review` or `blocked`. | Tightening workflow constraints is a product/API change, not just a layout change. |
| AI commands are created for explicitly parsed AI mentions, not for ordinary no-AI-mention comments. | Preserve explicit triggering; ordinary discussion must remain easy. |
| The composer currently requires a preview before every nonempty comment. | Remove the extra user step for ordinary discussion while preserving server-side validation. |
| The comment's AI-authority badge reports the author's current permission. | Do not present it as a historical execution fact. |
| The collaboration entry and source-workspace entry refer to the same issue. | Keep result and status consistent across both entries; do not introduce copied task state. |
| A parent issue has its own lifecycle. | Completion of a collaboration child does not imply completion of its parent. |
| The reviewed description's “no AI” statement is free text. | Verify available structured policy before designing an enforced manual mode. |

These are source-review findings, not evidence that every button was exercised against a live server. The original screenshots were partial. The full source composition was inspected.

## 2. Product scope and priority

### P1 — Deliver a coherent task workflow

1. Explain the task requirements, reference material, and acceptance criteria.
2. Show current responsibility and one appropriate primary action.
3. Make formal delivery results visible, with author, time, and supporting references.
4. Support submission, acceptance, and return for revision with meaningful records.
5. Distinguish ordinary discussion, delivery submission, and explicit AI triggering.
6. Establish truthful manual/AI presentation and preserve existing execution controls when relevant.
7. Keep task/result identity and permissions consistent in Web, Desktop, and the source-workspace entry.

### P2 — Reduce distraction and improve handoff

1. Move source metadata into a compact sidebar; show meaningful values first.
2. Give reference files and delivered files clear purposes without requiring an upload for text-only work.
3. Present comments and meaningful business events as “讨论与进展”, with a detailed audit entry.
4. Present collaboration memory as optional “工作摘要 / 交接笔记”.
5. Keep knowledge promotion as an optional follow-on activity with its own approval and scope.
6. Preserve responsive layout, keyboard use, deep links, restored scroll, and recoverable drafts.

Changes to the AI scheduler, host runtime, squads, knowledge governance, and unrelated source-picker work are outside this task. Extend a creation/edit surface only when required to express the selected task policy or acceptance requirements. Mobile implementation is a separate deliverable; shared API compatibility still applies.

## 3. Recommended business contracts

### Responsibility and progress

| Business stage | Responsible actor | Primary action |
| --- | --- | --- |
| `待处理` | Current handler | `开始处理` |
| `处理中` | Current handler | `提交核对结果` / `提交验收` |
| `待补充信息` | Named information recipient | `补充并交回` |
| `待验收` | Initiator | `验收通过并完成`, with `退回修改` as a secondary action |
| `已完成` | Viewers; initiator owns reopening | `查看最终结果`; reopening in the contextual menu |

The underlying system also supports backlog, general blocking, and cancellation. Retain their genuine meanings. Render “待验收” or “待补充信息” only when the authoritative workflow record supports that meaning; a status value alone must not invent a handoff.

The initiator owns final confirmation. The current handler owns the next step and can change through handoffs. Other participants provide discussion and material. AI targets provide assistance and are distinct from the human handler.

### Delivery and acceptance

- A formal delivery must contain a nonempty result. For reconciliation work, guide the author to include the object/version being checked, conclusion, basis, discrepancies, and unresolved questions. Require result and basis; allow text-only evidence.
- Reference existing issue-intended or delivered attachment identities. Preserve protected file delivery and retention semantics.
- Reuse the existing acceptance handoff where it can express the required facts. Introduce new persistent structures only after demonstrating a missing contract.
- “引用此评论提交结果” may prepare a submission draft. It must require an explicit submission; reading, replying, or recognizing conclusion-like prose must not advance the workflow.
- A submitted result must remain attributable and stable. Editing its source comment must not silently change an already submitted or accepted result.
- Acceptance must identify the confirmer, time, and exact submitted result being accepted. A return for revision must record its reason and the recipient, defaulting to the actual delivery author.
- Keep prior submissions and decisions visible when work is revised or reopened.
- Recommended completion policy: externally handled work requires formal delivery and initiator confirmation. When the initiator is also the handler, allow entering a result and completing directly without a self-handoff. Confirm this policy against existing callers during planning.
- Completion and cancellation are different outcomes. Preserve the initiator-only boundary and define their audit facts explicitly.
- Restrict generic status changes that would fabricate submission or acceptance. Workflow transitions must atomically update responsibility, status, and the corresponding record.

### Manual discussion and AI assistance

- Ordinary comments should be published with one user action. Existing preview/preflight API validation may run inside that action; do not bypass validated permissions, draft correlation, or execution versions.
- Show an impact preview when a message explicitly targets AI, including target, authorization, and current execution availability.
- Verify whether a task-level manual policy already exists. If a new policy is required, define it in the technical design and enforce it across relevant entry points; hiding a panel or reading a description is insufficient enforcement.
- Distinguish task policy, the environment's new-execution gate, and existing queue/run state.
- When a task has no AI targets, runs, queued inputs, or unresolved execution requests, avoid a large empty AI section. When those facts exist, retain the relevant history, control, and recovery entry points, including when new execution is disabled.
- AI success does not automatically accept or complete human work. If completion requests asynchronous execution shutdown, show any pending shutdown in the AI area rather than implying it has already settled.

## 4. Page composition

The first viewport should communicate: `what is requested → who owes the next step → what has been delivered → what this viewer can do`.

```text
Task title
Business stage · Current responsible person · Relevant deadline
[Role/stage-specific primary action] [Secondary action] [More]

Main content                         Information sidebar
----------------------------------   -----------------------------
Task requirements                   Initiator / delivery author
Reference material                  Participants and relevant roles
Acceptance criteria                 Owning workspace / deadline
                                    Set source properties
Formal delivery result
Conclusion / basis / unresolved work
Submission attribution and time
Acceptance decision / revision trail

Discussion and progress
Comments / information handoffs / transfers / delivery / decisions
[Write a comment] [Publish]

Relevant optional entries:
Files · AI assistance · Working summary · Knowledge promotion · Audit
```

- Use a main column and compact sidebar on wider windows. Stack them deliberately on smaller windows; keep the next action reachable and preserve content order.
- Put edit/manage/cancel/source-property actions in the relevant local area or contextual menu. Avoid an undifferentiated row of all capabilities.
- If no result has been submitted, show a concise result state and the authorized next action. Do not display a comment as accepted delivery.
- Show reference and delivery files where they support the requirement or result. Preserve the ability to add files and recover unfinished uploads.
- Keep empty source properties in the editing surface rather than making each empty field a main-page block.
- Keep participants discoverable through a labeled count/list. Move current AI permissions into that context; comments should emphasize authorship and actual behavior.
- Show working-summary content when it helps a handoff. Keep byte limits, detailed versions, restore, and purge controls inside editing/history.
- Offer knowledge promotion alongside useful results or through its dedicated entry. Keep candidate approval and reuse scope independent of task completion.
- Combine or cross-reference discussion and business events without rendering each comment twice. Use exact record identities for associations. Older or unavailable activity remains factual in the detailed audit view; never reconstruct it by timestamp proximity.
- Pending writes, errors, or uncertain outcomes must reveal their recovery area even when an auxiliary panel would otherwise be collapsed.

For the reviewed example, the existing comment “导出口径与需求一致” is discussion evidence. The intended next step is explicit submission by the current handler, followed by initiator acceptance. The page must obtain the actual workflow record before labeling the task “待杨尚伟验收”.

## 5. Execution sequence and ownership

### Step 1 — Confirm contracts and establish the Trellis task

**Read:** `CLAUDE.md`; `.trellis/spec/core/frontend/collaboration-workbench.md`; `.trellis/spec/views/frontend/index.md`; relevant server foundation, source-editing, content, records, and execution contracts.

**Inspect:** the source map below and all current caller paths for submission, status changes, completion, and comment sending.

**Produce:** the task's PRD, technical design, implementation checklist, and implement/check context manifests. Record the selected completion policy, legacy-data behavior, manual-policy representation, and rework operation. List exact API/schema/storage changes before coding.

**Gate:** Complete the project's planning review and activation requirements. Do not start implementation from this business handoff alone.

### Step 2 — Protect existing behavior and implement the business contracts

**Responsibility:** Server collaboration workflow and Core actions/schemas.

**Likely files:**

- `server/internal/collaboration/access.go`
- `server/internal/service/collaboration_handoff.go`
- `server/internal/service/collaboration_lifecycle.go`
- `server/internal/service/collaboration_workflow.go`
- `server/internal/handler/collaboration_workflow.go`
- `server/pkg/db/queries/collaboration_workflow.sql`
- `packages/core/collaboration/action.ts`
- `packages/core/collaboration/schema.ts`
- Relevant existing workflow/lifecycle API schemas and tests.

First add meaningful regression tests for role boundaries, atomic responsibility transfer, result requirements, accepted-result attribution, and return for revision. Confirm the expected failures before changing behavior. Implement one contract at a time and run its narrow tests. Verify every affected entry point rather than constraining the new page alone.

If migrations are necessary, follow the repository's no-foreign-key rule, put each concurrent index in a separate single-statement migration, regenerate sqlc, and document upgrade behavior. Preserve compatibility for installed Desktop clients and truthful presentation of legacy records.

### Step 3 — Recompose the shared detail page

**Responsibility:** `packages/views/collaboration/`, shared by Web and Desktop.

**Likely files:** `collaboration-detail.tsx`, `workflow.tsx`, `field-summary.tsx`, `participants.tsx`, `attachment-panel.tsx`, `activity.tsx`, `comment-list.tsx`, and their existing tests. Add a focused result presentation component only if existing components cannot carry it cleanly.

Implement requirement/result/decision presentation, role-specific next actions, sidebar metadata, and readable discussion/progress. Keep result derivation and permission rules in Core or the server, not a parallel component-only model. Update both `packages/views/locales/zh-Hans/collaboration.json` and `packages/views/locales/en/collaboration.json`.

Verify behavior through semantic assertions and the critical user journey. Use the existing design tokens and primitives; preserve Web/Desktop routing adapters, comment/run/memory links, focus, and restored scroll.

### Step 4 — Streamline discussion and auxiliary panels

**Responsibility:** comment composer/preflight wiring and relevant panel disclosure.

**Likely files:** `comment-composer.tsx`, `execution-panel.tsx`, `memory-panel.tsx`, `knowledge-panel.tsx`, and corresponding Core comment/policy schemas if required.

Implement one-click ordinary comments with existing validated send semantics. Keep explicit AI previews and target controls. Apply conditional panel presentation without hiding pending operations, meaningful history, errors, or recovery actions. Preserve memory and knowledge permissions and scope.

### Step 5 — Validate, document, and hand off the finished change

Run the acceptance matrix below, inspect Web and Desktop layouts, and collect before/after evidence. Run required quality checks; record environment-dependent failures accurately. Update executable contracts and affected built-in skill documentation when behavior changes. Commit only owned changes using the repository's Lore protocol, after verification and the applicable Trellis review gates.

Independent implementation lanes may use native Codex subagents, with explicit ownership. Server/Core contract changes precede UI integration. Locale files and shared workflow components need a single owner or coordinated handoff.

## 6. Acceptance matrix

| Scenario | Required evidence |
| --- | --- |
| External handler processes a manual task | Correct next actor/action; no irrelevant empty AI console; no expansion of workspace access. |
| Handler requests information and receives a reply | Responsibility follows the actual handoff and returns to the recorded requester. A comment alone does not return it. |
| Handler submits a text-only reconciliation result | Nonempty result/basis; stable attribution; responsibility moves to the initiator; no forced attachment. |
| Initiator returns the result for revision | Reason and recipient recorded; previous result retained; handler can resubmit. |
| Initiator accepts | Exact submission confirmed; whole issue completes; the source-workspace entry shows the same outcome. |
| Initiator handles their own task | Result recorded and direct completion follows the selected self-handled policy. |
| Other participant attempts acceptance/completion | Server refuses unauthorized action; UI exposes only their actual capabilities. |
| Generic status change attempts to fabricate review/completion | No fictitious delivery or acceptance; caller receives a clear contract response. |
| Ordinary comment without AI mention | One user publish action; no AI command; preflight/permission validation retained. |
| Comment explicitly targets AI | Visible target/impact preview; task policy, grant, and execution gate enforced. |
| New AI execution disabled with retained runs/inputs | Meaningful history, stop controls, and uncertain-operation recovery remain accessible. |
| Comment referenced by a submitted result is edited | The accepted/submitted result remains stable and attributable. |
| Legacy review/activity record lacks evidence | Factual unavailable state; no invented result, actor, handoff, or decision. |
| Refresh, reconnect, stale revision, or unknown write result | Draft retained; original-key recovery used; no duplicate submission or false success. |
| Access removed or viewer/server changes | Protected reads, files, and drafts follow existing scope/epoch cleanup rules. |
| Parent task exists | Collaboration completion does not automatically complete the parent. |
| Long title/result, narrow window, keyboard use, and deep links | Readable layout, reachable actions, preserved links/focus/scroll. |

At least one production-Web and one actual Desktop account journey must cover `manual task → information request/return → submit → return for revision → resubmit → accept`. Use existing isolated environments and test fixtures. Use fake AI execution where required; this plan does not authorize authenticated real-model quota consumption.

Keep each business matrix in one canonical test layer. Views tests cover wiring and presentation, rather than duplicating every Core/server case through DOM mounts.

Suggested narrow checks, confirmed against current package scripts:

```bash
pnpm --filter @multica/core exec vitest run collaboration/action.test.ts collaboration/comment.test.ts api/collaboration-workflow.test.ts api/collaboration-lifecycle.test.ts
pnpm --filter @multica/views exec vitest run collaboration/workflow.test.tsx collaboration/collaboration-detail.test.tsx collaboration/comment-composer.test.tsx
```

Run new tests alongside these existing suites. Use `make test` for DB-backed Go verification so this checkout's database setup and migrations are respected. Consult `Makefile` and the E2E environment specification before a direct Go or Playwright command.

Final checks as appropriate to the implemented scope: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `make test`, and `make check`. Record actual results; this planning session ran none of those checks.

## 7. Constraints and current workspace handoff

- Repository: `/Volumes/artisan/code/2026/multica-0.4.37`.
- Review/handoff branch: `codex/cross-workspace-collaboration`. Recheck the branch and worktree at the new session's start.
- The workspace contains substantial unrelated, uncommitted work. At handoff, `workflow.tsx`, source-field/editing components, locale files, and source-field backend files had also acquired pending changes. Preserve them and inspect their current state before planning edits. Do not reset, revert, commit, or archive other sessions' work.
- Recheck source and caller contracts rather than treating line numbers or the earlier screenshot as current runtime truth.
- Reuse existing utilities and components, keep changes reviewable, and add no dependencies without an explicit request.
- React Query owns server state. Use existing draft/operation helpers for intent, revision checks, and original-key recovery; do not add a second task/result state store.
- Respect package boundaries: no platform routing in Views, no UI/storage/platform APIs in Core, no business logic in UI primitives.
- Preserve external-account scope and protected attachment handling when moving information into the sidebar or result panel.
- Do not reopen unrelated AI-runtime or knowledge-acceptance gates, or claim their completion as part of this detail-page task.

## 8. Source map

Use these as entry points, then follow the current definitions and callers:

- [Shared detail composition](../../packages/views/collaboration/collaboration-detail.tsx)
- [Workflow controls and submission form](../../packages/views/collaboration/workflow.tsx)
- [Comment composer and preview](../../packages/views/collaboration/comment-composer.tsx)
- [Comment authorship and authority labels](../../packages/views/collaboration/comment-list.tsx)
- [Server authority](../../server/internal/collaboration/access.go)
- [Acceptance and information handoffs](../../server/internal/service/collaboration_handoff.go)
- [Status and completion lifecycle](../../server/internal/service/collaboration_lifecycle.go)
- [Atomic workflow persistence](../../server/internal/service/collaboration_workflow.go)
- [Activity projection](../../server/internal/service/collaboration_activity.go)
- [Explicit mention processing](../../server/internal/service/collaboration_mentions.go)
- [Comment and command creation](../../server/internal/service/collaboration_comments_create.go)
- [Original cross-account collaboration PRD](../../.trellis/tasks/10-10-cross-workspace-collaboration/prd.md)
- [Collaboration context PRD](../../.trellis/tasks/10-30-collaboration-context-handoff/prd.md)
- [Knowledge promotion PRD](../../.trellis/tasks/10-30-collaboration-knowledge-promotion/prd.md)
- [Core collaboration workbench contract](../../.trellis/spec/core/frontend/collaboration-workbench.md)

## 9. New-session starter prompt

```text
请推进协作任务详情页的业务流程与界面优化。先阅读 CLAUDE.md、AGENTS.md 和 docs/plans/2026-11-02-collaboration-detail-optimization.md，并核对当前分支、相关实现与未提交改动。我授权本会话创建对应的 Trellis 规划任务，按项目流程完善 prd.md、design.md、implement.md，经规划复核和任务激活后推进实现与验证。

目标是让页面清楚回答“要做什么、当前轮到谁、交付了什么、我下一步做什么”。围绕已有“处理人提交验收→转给发起人→发起人完成”流程，按“业务约束→交付结果与角色操作→详情页布局→讨论与辅助模块→Web/Desktop 完整验收”的顺序推进。重点落实结果与依据、明确验收、退回修改和历史保留；普通评论、正式交付和 AI 触发应有清楚区别。

复用现有 issue、handoff、权限、附件与请求恢复机制。方案中的完成约束、退回修改和人工模式属于待核对的业务/API 改动；先形成具体契约再实现。保护其他会话的未提交修改，遵守包边界，按实际目标与运行历史展示 AI 控制，保持外部账号权限及两处任务入口一致。新会话先给出范围、待决策项、状态/角色/操作表和验收矩阵，再按 Trellis 推进；无需重新索取上一会话背景。
```
