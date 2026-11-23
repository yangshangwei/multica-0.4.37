# Agent welcome implementation plan

## Sequence

1. [x] Inspect directory, tests, navigation, locale conventions and welcome patterns. Record approved scope and design.
2. [x] Implementation agent owns `packages/views/agents/components/agents-page.tsx`, its tests, new `agent-welcome*` files, and `packages/views/locales/{en,zh-Hans}/agents.json`. Add failing behavior tests, implement the welcome and successful-empty gate, run focused Vitest/lint/typecheck.
3. [x] Coordinator owns `e2e/agent-welcome.spec.ts`, task artifacts and final spec notes. Prepare production-Web verification with isolated fixtures. Verify navigation, carousel, locales/themes/widths and directory transition. Preserve other sessions' services and artifacts.
4. [x] Compare screenshots to the reference with visual-verdict; persist `.omx/state/agent-welcome/ralph-progress.json`. Apply material fixes in one batch and confirm.
5. [x] Dispatch Trellis check for independent behavior, boundaries, CSS/i18n/accessibility and evidence review. Resolve findings and rerun affected checks.
6. [x] Record the empty-state contract in agent discovery spec, update evidence and journal/task state; report scoped changes, tests and limitations. Do not stage unrelated work.

## Commands

```sh
pnpm --filter @multica/views exec vitest run agents/components/agents-page.test.tsx agents/components/agent-welcome.test.tsx locales/parity.test.ts
pnpm --filter @multica/views exec eslint agents/components/agents-page.tsx agents/components/agents-page.test.tsx agents/components/agent-welcome.tsx agents/components/agent-welcome.test.tsx
pnpm --filter @multica/views typecheck
pnpm --filter @multica/web typecheck
pnpm --filter @multica/desktop typecheck
# Only against a verified production Web build containing this change:
pnpm exec playwright test e2e/agent-welcome.spec.ts --workers=1 --retries=0
git diff --check
```

## Integration constraints

Other sessions have dirty squad/automation pages and QA artifacts. Preserve them. Implementation/coordinator write scopes are disjoint. No backend work, dependency changes, destructive setup, real-agent CLI execution or publication.

## Verified outcome

Focused Vitest passed 6 files / 134 tests. Scoped ESLint and Views/Web/Desktop
typechecks passed. Production-Web welcome acceptance and the existing agent
discovery journey both passed without retries. Both locales/themes were measured
at 1440×900, 1024×768, 390×844 and 320×568; document and welcome-container
overflow checks, primary-action visibility and narrow carousel access passed.
The reference verdict improved from 84 to 94 after replacing unresolved
Tailwind radius aliases with runtime formulas. The mechanical design detector
reported no findings. Task-owned API/Web processes and database were cleaned up.
See `evidence.md` for source/build bindings, original failures and coverage limits.
