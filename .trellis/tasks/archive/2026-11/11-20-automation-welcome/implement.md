# Automation welcome implementation plan

## Execution

1. [x] Implement agent reads PRD/design and curated specs; inspect the existing
   catalog, page, tests, tokens and user reference. Add meaningful failing tests
   for welcome headline/actions, preview/full-catalog flow and error recovery.
2. [x] Add the welcome presentation and bilingual strings; reuse existing creation,
   schedule and navigation contracts. Preserve the full picker and list.
3. [x] Run affected Vitest suites including catalog/configure restoration and locale
   parity; focused ESLint and views typecheck. Update affected E2E expectations.
4. [x] Coordinator runs a production web verification environment, captures desktop
   and narrow screenshots with Chinese and dark-mode coverage, and tests real
   click/Back/focus behavior. One batched inspection and at most one correction
   batch; persist a visual-verdict against the supplied reference.
5. [x] Independent Trellis check reviews scope, states, accessibility and verification.
   Run the Impeccable detector once on the changed UI; resolve mechanical findings
   and keep reference-pinned choices. Use a fresh finish reviewer for screenshots.
6. [x] Update the discovery spec, document evidence and limitations, commit only this
   task's files using the repository Lore/conventional format, and archive task.

## Commands

- pnpm --filter @multica/views exec vitest run autopilots/components/autopilots-page.test.tsx autopilots/components/autopilot-template-catalog.test.tsx autopilots/components/template-create-autopilot-page.test.tsx locales/parity.test.ts
- pnpm --filter @multica/views typecheck
- pnpm --filter @multica/views exec eslint autopilots/components/autopilots-page.tsx autopilots/components/autopilot-template-catalog.tsx autopilots/components/autopilots-page.test.tsx autopilots/components/autopilot-template-catalog.test.tsx
- PLAYWRIGHT_BASE_URL=<production web> API_URL=<verification API> pnpm exec playwright test e2e/autopilot-template.spec.ts --project=chromium
- git diff --check

## Coordination

Implementation agent owns only the shared automation files, bilingual locale keys
and targeted E2E tests. Coordinator owns task/spec documentation and verification
environment/artifacts. Preserve unrelated pre-existing edits and QA artifacts.
No task implementation is handed to another worker recursively.

## Completion evidence

See `verification.md` and `research/visual-review.md`. Fresh focused checks
passed 122 component/locale tests; the final production Chromium replay passed
all 14 cases in 40.3 seconds. The independent visual review inspected all 18
captures and returned ship with a score of 94/100. The continuation also fixed
the browser helper's loading-header versus welcome-action selector race without
changing product UI, assertions, timeouts or retries.
