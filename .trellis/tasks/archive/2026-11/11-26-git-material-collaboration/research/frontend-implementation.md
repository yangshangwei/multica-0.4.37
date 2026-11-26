# Frontend implementation and verification

- Date: 2026-11-26
- Scope: shared core/views only, plus project path and local-draft cleanup registration.
- Branch: parent-owned `codex/git-engineering-material-workflow`.

## Implemented

- `packages/core/materials/`: response schemas, strict scoped identity/reference checks, redacted malformed-response parsing, current-revision review eligibility, Query hooks, original request recovery and durable local editor drafts.
- `packages/core/api/client.ts`: the settings/list/create/detail/command methods in contracts.md, every request captures its workspace header.
- `packages/core/paths/paths.ts`: shared project `section=materials` and material detail link; no platform router fork.
- `packages/views/projects/components/project-materials.tsx`: project materials collection, search, creation, settings and selected detail.
- `project-material-settings.tsx`: existing repository resource, GitHub installation row/GitLab connection, reviewers, planning Agent and default-on automatic plan switch. Explicit false survives reads/saves. Saves await the response and retain uncertain input.
- `material-detail.tsx`: complete PRD/plan content, baseline, comparison, source versions, review history, comments, retry and manual plan start. Candidate ID/head/digest are pinned across polling. Newer revisions or a changed head cannot silently inherit the open review.
- `material-revision-editor.tsx`: full requirement or design+task breakdown drafts, numbered acceptance criteria, source SHA entries, blocking questions and ready-to-publish flag. Draft saves do not implicitly publish.
- Matching English/Simplified Chinese text and actionable blocker messages. Semantic tokens and existing UI/Markdown primitives; no new dependency.
- The local pending-request store persists the original body before a write. Remount recovery resends that exact request. Project denial/deletion and account draft cleanup erase private local content through existing boundaries.

## Verification actually run

- `pnpm --filter @multica/core typecheck`: passed.
- `pnpm --filter @multica/views typecheck`: passed.
- `pnpm --filter @multica/core lint`: passed.
- `pnpm --filter @multica/views lint`: passed with 27 existing warnings; changed new component files have no lint findings. Two warnings in existing ProjectDetail concern pre-existing hook dependencies.
- `pnpm --filter @multica/core exec vitest run materials projects/access-lifecycle.test.ts drafts/cleanup-registry.test.ts paths/paths.test.ts`: 5 files, 36 tests passed. After tightening latest-revision review selection, reran material suites: 2 files, 10 tests passed.
- `pnpm --filter @multica/views exec vitest run projects/components/project-materials.test.tsx projects/components/project-detail.test.tsx locales/parity.test.ts`: 3 files, 105 tests passed on latest UI.
- `git diff --check -- packages/core packages/views`: passed.
- Impeccable mechanical detector over the five new production UI files: `[]` findings. This is not screenshot or browser acceptance.

## Integration checks remaining for parent

- Production Web/Desktop browser flow and visual-verdict evidence must run against the integrated backend. This lane did not start an application server, touch a test database, call real Git/Agent services or claim browser verification.
- Backend publication status agreement: `draft`, `publishing`, `in_review`, `merged`, `blocked`. Requirement baseline remains the original requirement revision when a plan is approved.
- Backend should distinguish missing reviewer/manager action permission from membership/credential revocation. Shared project access guards deliberately erase private caches for generic 403; an ordinary action refusal must not accidentally be treated as workspace membership loss.
- Backend was notified to keep returned/retracted revisions from being re-approved without revision and to ensure only the latest cross-kind candidate can be decided. UI already gates `changes_requested` and superseded candidates.
- No commits, product spec changes, unrelated file cleanup, or remote writes were performed by this lane.
