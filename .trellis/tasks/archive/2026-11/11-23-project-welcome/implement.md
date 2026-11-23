# Implementation and verification

1. Read PRD, design, curated context and the incumbent iteration/triage welcome components. Read Impeccable craft-floor immediately before UI edits.
2. Extend the canonical projects-page test for the new welcome and real modal/docs wiring; cover successful empty vs loading, placeholder, initial error, failed refresh, denied access and existing filtered results where not already protected.
3. Implement the local passive welcome, bilingual copy and shared-page gate/header wiring. Do not change the list, modal, data model or sibling welcome components.
4. Run focused project tests and locale parity, views lint/typecheck and affected consumer typechecks. Request a bounded Trellis check review.
5. Render the actual application using the existing isolated environment helpers. Capture bilingual, light/dark, desktop/narrow views in one batched pass; verify create/docs operations and no overflow. Run the Impeccable detector once on changed UI targets.
6. Persist a visual verdict before further UI edits. Fix only demonstrated issues in one batch and, if needed, confirm once. Evidence must identify the renderer and source revision; do not claim native Electron behavior from a Web screenshot.
7. Record the local successful-empty/presentation contract in the views spec, recheck changes, commit only owned files with Lore trailers, archive the completed task and record the session.

## Commands

- `pnpm --filter @multica/views exec vitest run projects/components/projects-page.test.tsx locales/parity.test.ts`
- `pnpm --filter @multica/views typecheck`
- `pnpm --filter @multica/views exec eslint projects/components/project-welcome.tsx projects/components/projects-page.tsx projects/components/projects-page.test.tsx`
- `pnpm --filter @multica/web typecheck` and `pnpm --filter @multica/desktop typecheck`
- Production Web: `make up C=api,web ARGS="--ephemeral --web-mode production"`, then scoped existing Playwright fixture usage. Prefer a task-owned browser script for screenshot-only verification; do not add tests that assert only static class names or copy.

## Known environment condition

At discovery the registered checkout environment was stopped and expired; unrelated edits and QA artifacts already exist. Preserve them. Server/renderer startup is verification infrastructure, not product scope. If unrelated checks fail, record precise pre-existing evidence instead of widening the diff.
