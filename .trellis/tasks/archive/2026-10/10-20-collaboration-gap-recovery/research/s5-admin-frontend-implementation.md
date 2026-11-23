# S5 shared collaboration administration frontend

Status: bounded frontend implemented and verified; independent review and real
Web/Electron integration remain with the coordinator. No parent acceptance claim.

## Boundary and reuse plan

The missing behavior is the shared, password-super-admin-only list/detail and
complete/cancel/reopen surface for disabled-creator collaborations. The backend
owns authority, lifecycle effects, successor eligibility and operation receipts.
This lane owns scoped Query transport, exact original-intent persistence, the
shared form and its focused tests. The coordinator owns schema/API integration,
barrels, locales, platform routes and production/native verification.

- Extend `core/admin/operation-draft.ts` with one collaboration input kind and
  typed reads, reusing its enumerated per-request storage and exact-key removal.
  Keep existing admission/cancellation/alert callers and storage behavior intact.
- Add `core/admin/collaboration-queries.ts` for scoped paginated reads, strict
  candidate/revision and receipt correlation, a single POST, and read-only
  recovery. Store only client intent before transport. A successful original
  intent remains recoverable across remount until the user dismisses its known
  receipt; an empty lookup never permits a replacement key.
- Add `views/admin/collaboration/` for a protected minimal list/detail, explicit
  paginated successor selection, frozen confirmation and historical receipt.
  Recovery stays mounted when current metadata is unavailable or the creator is
  restored. No ordinary issue reads or control/hold mutations are introduced.
- Write behavioral regressions before the storage/query changes, and keep the
  boundary matrix in core. Views tests cover actual form wiring, authorization
  removal, recovery independence, paging, accessibility and selection expiry.

No new dependency, Mobile feature, additional role grant, deployment, real
provider execution, broad cleanup or schema change is included. Existing admin
draft tests provide regression coverage for the small persistence extension.

## Implemented behavior

- `AdminCollaborationPage({issueId, hrefForIssue})` and
  `AdminCollaborationEntry({href})` both require ready current super-admin access.
  The page rekeys by API/session, actor, organization and issue; its ordinary
  collaboration permissions are never consulted or extended.
- List, detail and candidate queries verify the backend's transaction-scoped
  organization envelope as well as the current API/authentication scope. The
  coordinator added this envelope during integration. An extra `/admin/me`
  round-trip was rejected because it cannot correlate an earlier response to
  the same read transaction.
- Candidate pages retain issue/revision/cursor identity. The shared form shows
  all pages, requires an explicit eligible participant only for double-disabled
  reopen, and clears an expired selection without auto-selecting it later.
  Source, creator, handler, status, revision and status-option drift block the
  opened confirmation until an explicit close/refresh/new confirmation.
- The existing administration draft prefix, storage adapter, enumeration and
  exact-key removal now support a collaboration input branch. Legacy
  `AdminControlInput` and the admission/cancellation/alert transport stay intact.
  A separate typed reader prevents legacy controls from consuming the new kind.
- The original actor/server/organization/issue/key, creator/handler identity,
  revision/status/options/reason/successor are persisted before POST. Mutation
  retries are disabled. Ambiguous transport uses only the original GET;
  an empty lookup preserves the same intent. An explicit retry sends that exact
  original payload. Current denial is propagated to the existing admin cache
  guard; late responses cannot settle another scope.
- Recovery results must match the operation actor/organization and the original
  request, creator, previous handler, selected successor and lifecycle facts.
  A known receipt stays in Query, while only its original client intent remains
  persisted until explicit receipt dismissal. Remount recovery remains usable
  when a restored creator makes current metadata return 409.
- Only explicit, first-attempt transactional refusals retire the corresponding
  intent. A previous ambiguous request stays frozen after a later refusal or
  empty lookup. The rejected form also stays frozen until explicitly closed.
- The UI uses existing admin controls, semantic tokens, scoped field/heading
  IDs, native keyboard-selectable radios, validation focus and focus return.
  Metadata and receipts use wrapping grids; no new dependency, store or global
  stylesheet was introduced. A lifecycle receipt never claims process
  termination or automatic execution resume.

## Files owned by this lane

- `packages/core/admin/operation-draft.ts`
- `packages/core/admin/collaboration-draft.test.ts`
- `packages/core/admin/collaboration-queries.ts`
- `packages/core/admin/collaboration-queries.test.ts`
- `packages/core/admin/test-fixtures.ts`
- `packages/views/admin/collaboration/collaboration-page.tsx`
- `packages/views/admin/collaboration/collaboration-controls.tsx`
- `packages/views/admin/collaboration/collaboration-form.tsx`
- `packages/views/admin/collaboration/collaboration-receipt.tsx`
- `packages/views/admin/collaboration/collaboration-page.test.tsx`
- This report and `research/s5-admin-frontend-locales.json`.

The coordinator owns the schema/API/client tests, package exports, common
barrels, locale merge and both platform entry points. These are integration
dependencies, not additional files claimed by this lane.

## Verification log

- The initial four persistence regressions failed because the existing draft
  schema rejected the new collaboration intent. After the narrow branch
  extension, those tests plus the existing storage/control query suites passed:
  26 tests, zero failures/skips.
- Scoped Query/intent testing then passed with the existing suites: 68 tests.
  Additional final-login/source/refusal cases are recorded in the final rerun.
- The first 13-case UI run passed 12 and failed an immediate assertion following
  a scheduled Query notification. The assertion now awaits the resulting DOM
  state with `waitFor`; no product behavior or timeout was changed. Its rerun
  with the old control form/panel suites passed 26 tests.
- A broader pre-byte-limit integration run passed all 237 core administration
  tests (including 10 coordinator API and 20 schema cases), plus 27 views tests
  (14 S5 and 13 adjacent controls), with zero failures or skips. Both package
  typechecks, all touched-file ESLint checks and scoped `git diff --check`
  passed at that increment.
- Final review found that `validateAdminReason` counts 1000 UTF-8 bytes and
  forbids NUL while the initial S5 schema counted characters. The new UI
  regression with 334 Chinese characters (1002 bytes) proved the incorrect POST
  before the coordinator's schema fix. The form calls the same schema reason
  validator and retains editable input on failure; it has no duplicate byte
  policy. The `invalid_reason` first-attempt refusal is also recognized.
  RED: `s5-frontend-byte-limit-red.json`, one behavioral failure and 14 cases
  intentionally not selected by the focused `-t` invocation.
- Final rerun after the coordinator's shared byte/NUL schema and locale fix:
  **239 core administration tests across 28 files passed**, including 45 scoped
  Query/intent cases, 4 new draft cases, and the coordinator's 10 API/21 schema
  cases. **28 views tests across 3 files passed**, including all 15 S5 interaction
  tests and 13 adjacent control cases. Both runs have zero failures or skips.
  The byte-limit regression now prevents the POST and preserves focused,
  editable input. Both package typechecks and touched-file ESLint passed again.
  These final totals include earlier runs' tests and must not be added together.

Raw reports live under
`/tmp/multica-gap-recovery-20261021-h6phxwo9/`:
`s5-frontend-core-admin.json`, `s5-frontend-views.json`,
`s5-frontend-byte-limit-red.json`, `s5-frontend-core-admin-final.json`, and
`s5-frontend-views-final.json`. `s5-frontend-verification.json` records final
test totals, test identities, and source hashes at handoff; it is not a
production-build identity.

Commands (from repository root; all tests use synthetic in-memory identities):

```bash
pnpm --filter @multica/core test admin --maxWorkers=2 --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/s5-frontend-core-admin-final.json
pnpm --filter @multica/views test admin/collaboration/collaboration-page.test.tsx admin/operations/control-form.test.tsx admin/operations/control-panel.test.tsx --maxWorkers=2 --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/s5-frontend-views-final.json
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core exec eslint admin/collaboration-queries.ts admin/collaboration-queries.test.ts admin/collaboration-draft.test.ts admin/test-fixtures.ts admin/operation-draft.ts
pnpm --filter @multica/views exec eslint admin/collaboration
```

## Remaining integration and evidence limits

The coordinator owns independent review, real production Web/Electron checks,
wide/narrow/200-percent visual verification, the isolated password/admin-enabled
environment and acceptance-matrix updates. Existing admin screenshots were read
as style references only; they do not verify this new page. Platform account
routing and session-cache cleanup are not claimed by this lane. No real
provider/account access, production authentication change, rollout, commit or
task archival occurred.
