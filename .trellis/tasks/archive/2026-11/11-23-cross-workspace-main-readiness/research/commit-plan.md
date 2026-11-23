# 本地提交方案

候选基线：`73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`。用户已明确批准执行。下列5组工作提交已完成，第6组交付本任务记录；未 push、创建 PR 或合并 main。

## 1. fix(ci): keep large merge candidates fully checkable

- `.github/workflows/ci.yml`
- `.trellis/spec/web/frontend/image-budget-check.md`
- `.trellis/spec/web/frontend/index.md`
- `scripts/check-image-budget.mjs`
- `scripts/check-image-budget.test.mjs`

## 2. fix(navigation): preserve collaboration drafts across viewport changes

- `.trellis/spec/core/frontend/global-scope-navigation.md`
- `.trellis/spec/views/frontend/collaboration-creation.md`
- `.trellis/spec/web/frontend/e2e-run-environment.md`
- `e2e/capability-market.spec.ts`
- `e2e/collaboration-account-desktop.spec.ts`
- `e2e/collaboration-workbench-web.spec.ts`
- `e2e/fixtures/collaboration-knowledge.ts`
- `e2e/fixtures/collaboration-offline-handoff.ts`
- `e2e/fixtures/iterations-i1.ts`
- `e2e/fixtures/sidebar-navigation.ts`
- `e2e/iterations-audit-desktop.spec.ts`
- `e2e/localized-template-defaults.spec.ts`
- `e2e/mcp-desktop.spec.ts`
- `e2e/mcp-welcome-desktop.spec.ts`
- `e2e/navigation.spec.ts`
- `e2e/sidebar-icon-rail.spec.ts`
- `e2e/skill-market.spec.ts`
- `e2e/skill-welcome-desktop.spec.ts`
- `e2e/workspace-defaults.spec.ts`
- `packages/ui/components/ui/sidebar.tsx`
- `packages/views/collaboration/creation-form.test.tsx`
- `packages/views/collaboration/creation-form.tsx`
- `packages/views/layout/app-sidebar.test.tsx`
- `packages/views/layout/app-sidebar.tsx`
- `packages/views/settings/components/settings-page.test.tsx`

## 3. fix(collaboration): stop denied subscriptions from refreshing indefinitely

- `.trellis/spec/server/collaboration-realtime.md`
- `packages/core/realtime/collaboration.test.ts`
- `packages/core/realtime/collaboration.ts`

## 4. test(dbid): preserve notification identity through replay

- `server/pkg/dbid/writes_test.go`

## 5. docs: keep embedded guides aligned with current sources

- `server/internal/docs/content/pages/collaboration.json`
- `server/internal/docs/content/pages/desktop-app.json`
- `server/internal/docs/content/pages/feature-overview.json`
- `server/internal/docs/content/pages/tutorial.json`

## 6. Task evidence

`chore(task): retain merge readiness evidence and explicit verification limits`

- 本任务 PRD、设计、计划、READINESS 与精简研究/结果清单。
- 不包含 `.omx/private` 下的原始日志、trace、截图、凭据或数据库。

## 未纳入文件

- 仓库中既有的其他任务、历史 artifacts 和未跟踪材料均不纳入。
- 提交前核验所有38个工作文件与已验收哈希一致；工作提交后没有剩余已跟踪代码改动。

每条工作提交使用约定前缀与 Lore 决策/验证 trailers；提交前再核对 HEAD、文件列表与哈希。


## 执行记录

- `153103f75` — fix(ci): keep large merge candidates fully checkable
- `2a8d44e87` — fix(navigation): preserve collaboration drafts across viewport changes
- `e5c843a75` — fix(collaboration): stop denied subscriptions from refreshing indefinitely
- `513417d49` — test(dbid): preserve notification identity through replay
- `f0a3f71cf` — docs: keep embedded guides aligned with current sources

完整逐文件记录见 [local-commit-delivery.json](local-commit-delivery.json)。
