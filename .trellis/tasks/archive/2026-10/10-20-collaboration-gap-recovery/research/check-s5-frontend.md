# Independent review: S5 frontend and platform integration

Date: 2026-10-21. Active task: `10-20-collaboration-gap-recovery`.

Reviewed the delivered core administration schema/API/query/draft flow, shared
collaboration administration views, exports/locales, account-route discriminator,
shared account wrapper, Web route and Desktop integration. This follows the
separate S3/S5 backend review; it does not add its earlier test counts again.
No production service, browser, Electron process, credential or real provider
was accessed. No dependency, commit, rollout or S4 file changed.

## Findings fixed

### F1 — P3: shared cache-boundary tests remained in the Web package

After `AdminSessionBoundary` moved into `packages/views/admin`, its four behavior
tests still lived at `apps/web/platform/admin-session-boundary.test.tsx`. They
exercise account/logout/credential/server cache teardown, not Next.js behavior.
This contradicted the repository's canonical shared-test placement rule.

With coordinator approval, moved the four cases unchanged to
`packages/views/admin/admin-session-boundary.test.tsx`, beside the implementation.
Only a scope comment was added. Removed the old Web behavior test and updated
`research/s5-platform-integration.md` with the new verification path. A duplicate
test asserting only the re-export was deliberately not introduced.

Added one shared account-shell wiring regression to
`packages/views/collaboration/account-workbench-page.test.tsx`: administration
list-to-detail rendering keeps administration links, reads only the dedicated
management endpoints, and never mounts ordinary collaboration/workspace reads.
The existing authority and original-key matrices remain in core and the S5 view
suite rather than being repeated through this wrapper.

## Findings not fixed / handoff

No functional defect was found in the reviewed S5 frontend paths.

**F2 — P3, spec sync assigned to the coordinator:**
`.trellis/spec/views/frontend/component-guidelines.md:227` still states that the
Electron renderer never mounts `@multica/views/admin`; the nearby verification
rule also requires absence of administration CSS from Desktop. The new limited
administration page and shared session boundary are intentional exceptions.
Document those exceptions while retaining local theme/CSS isolation and leaving
the full Web administration console distinct. This reviewer left the
coordinator-owned project spec untouched and reported the exact drift.

Real password-mode production Web and actual Electron journeys, including
responsive/zoom and screenshot evidence, remain the coordinator's next step.
The passing navigation tests below are unit tests, not native runtime evidence.

## Verified behavior

- The page and optional account entry require current ready super-admin access.
  Backend read envelopes correlate the same authenticated organization to the
  list/detail/candidate data. API/Query layers retain server/session/actor/org
  identity and reject late responses from a replaced session. They never fall
  back to ordinary collaboration or resource endpoints.
- Input capture pins the original creator/handler, issue, revision/status,
  status-catalog version, reason and explicitly chosen successor. UTF-8 byte
  and NUL checks use the shared schema. Role/source/status/candidate drift
  blocks stale confirmations; candidate refresh does not silently select a
  replacement or revive a previously expired selection.
- A client intent is persisted before its single POST. Remount and uncertain
  transport recover with the original GET. Empty lookup, malformed result or
  a later refusal do not retire an earlier unknown intent or replace its key.
  An explicit retry preserves the exact original payload. Recovery remains
  mounted when creator restoration makes current metadata unavailable.
- Receipts correlate actor, organization, issue, key, original creator/handler,
  action, status and successor. They remain historical business results and
  do not claim process termination, grant restoration or automatic Resume.
  Only a confirmed first-attempt refusal or explicit matching receipt dismissal
  retires the corresponding client intent.
- The account parser preserves `administration=collaboration` in memento keys
  and rejects ordinary content anchors or the participating view mixed with it.
  The wrapper branches before ordinary collaboration hooks mount. Web and
  Desktop provide their own thin links while sharing the business page.
- Web and Desktop both mount the same session-cache boundary below CoreProvider
  and above routes. Account/credential/server/logout changes remove stale admin
  queries and mutations while preserving ordinary workspace cache semantics.
- The existing six navigation groups remain. The execution group contains the
  ordinary execution/business-task destinations and exposes collaboration only
  to a super-admin. Native radios, reason validation focus, explicit close/focus
  return, accessible status messages and bilingual key parity were checked.

## Fresh verification

All commands exited zero; every recorded test passed with no skip/pending case.

| Run | Result |
| --- | --- |
| Core admin plus account paths | 247 tests |
| Initial S5/shared views scope | 46 tests |
| Account wrapper plus relocated boundary after adding the regression | 9 tests |
| Desktop navigation/window/account unit tests | 73 tests |
| Web account/admin routing unit tests | 9 tests |
| Locale parity | 66 tests |

The two shared-view executions overlap by eight cases: they cover **47 distinct
tests**, not 55. Core, views, Web and Desktop typechecks passed, including both
Desktop node and renderer typechecks. All touched/integrated S5 files passed
scoped ESLint; scoped `git diff --check` passed.

Commands from repository root:

```bash
pnpm --filter @multica/core test admin paths/account --maxWorkers=2 --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s5-frontend-core.json
pnpm --filter @multica/views test admin/collaboration admin/admin-session-boundary.test.tsx admin/admin-shell.test.tsx collaboration/account-workbench-page.test.tsx admin/operations/control-form.test.tsx admin/operations/control-panel.test.tsx --maxWorkers=2 --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s5-frontend-views.json
pnpm --filter @multica/views test collaboration/account-workbench-page.test.tsx admin/admin-session-boundary.test.tsx --maxWorkers=2 --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s5-platform-shared.json
pnpm --filter @multica/desktop test src/shared/account-navigation.test.ts src/shared/issue-window.test.ts src/renderer/src/stores/account-tab.test.ts src/renderer/src/platform/navigation.test.tsx src/renderer/src/platform/issue-window-navigation.test.tsx src/renderer/src/components/issue-window.test.tsx --maxWorkers=2 --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s5-desktop-navigation.json
pnpm --filter @multica/web test platform/account-route.test.tsx platform/admin-route.test.tsx --maxWorkers=2 --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s5-web-routing.json
pnpm --filter @multica/views test locales/parity.test.ts --maxWorkers=2 --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s5-locale-parity.json
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/web typecheck
pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/core exec eslint admin/collaboration-schema.ts admin/collaboration-schema.test.ts admin/collaboration-api.test.ts admin/collaboration-queries.ts admin/collaboration-queries.test.ts admin/collaboration-draft.test.ts admin/operation-draft.ts admin/test-fixtures.ts admin/index.ts paths/account.ts paths/account.test.ts
pnpm --filter @multica/views exec eslint admin/collaboration admin/admin-session-boundary.tsx admin/admin-session-boundary.test.tsx admin/admin-shell.tsx admin/index.ts collaboration/account-workbench-page.tsx collaboration/account-workbench-page.test.tsx
pnpm --filter @multica/web exec eslint platform/admin-collaboration-route.tsx platform/admin-session-boundary.tsx platform/account-workbench-route.tsx app/admin/collaboration/page.tsx
pnpm --filter @multica/desktop exec eslint src/renderer/src/App.tsx src/renderer/src/platform/account-workbench-route.tsx
```

Raw JSON/logs and `check-s5-frontend-verification.json` reside under the task's
existing `/tmp/multica-gap-recovery-20261021-h6phxwo9/` directory. The manifest
records exact test identities, hashes and the unchanged HEAD; it is scoped
shared-workspace evidence, not a frozen production-build identity. No full
`make check`, installer, new browser/native journey or parent completion was
claimed.

## Review changes

Only the relocated boundary test, the new account-wrapper test, platform
integration note and this report changed in this review. No product fix was
needed. Reusing the shared implementation/test matrix avoids separate Web and
Desktop cache-cleanup behavior.
