# S2 browser integration and selection recovery

- Date: 2026-10-21.
- Owner: `s2_resource_frontend` implementation lane; no nested agents.
- Scope: new real API/UI E2E fixture and Web spec, then the confirmed unsubmitted-selection defect after the coordinator's first current-production visual capture.
- Coordinator-provided running source: API `http://localhost:18973`, production Web `http://localhost:13893`, build `eX3BXb5wTmAJSFzdab670`. This lane does not start, rebuild or stop those processes.
- Current status: the first real Web run reached supply/adoption but failed before consent POST. Both confirmed UI defects are now repaired and pass focused/component checks. A production rebuild/rerun and native acceptance remain coordinator-owned; there is no complete browser-journey pass yet.

## Existing independent findings

Read `research/check-s2-frontend.md` and the exact external failing reproducer at `/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s2-frontend/ui-diagnostic/authority-selection.test.tsx`.

The reviewer's `resource_options_unavailable` receipt correction and its two canonical core regressions remain intact. The open P2 is local selection state: when an unsubmitted adoption loses organizer authority, its form disappears but `selection !== null` keeps permitted owner actions disabled. The planned repair discards invalid unsubmitted UI selection while preserving persisted original-operation drafts and current ACL filtering. No style changes are in scope.

## New real journey

Files:

- `e2e/fixtures/collaboration-resource-offers.ts`
- `e2e/collaboration-resource-offers-web.spec.ts`

Exact Playwright test:

```text
a private owner offers a resource to an external organizer and independently authorizes it
```

The fixture creates synthetic P creator, H handler and D ordinary participant through `TestApiClient`. P owns the source workspace; D owns a separate execution workspace. P/H have no execution-space membership and D has no source-space membership. Creation and participation use actual public APIs and original request IDs. One offline runtime and its enrollment follow the existing resource-fixture pattern; no daemon, executable or provider account is attached. The selected and undisclosed agents both use private invocation permissions. The fixture rejects default databases, non-loopback services and mismatched browser/API origins and never updates deployment gates.

The UI journey, using real responses throughout:

1. Opens D and H account views in independently authenticated pages.
2. D selects and offers exactly one private resource. P/H inventory reads of D's workspace stay 404, their offer lists expose only the supplied item, and no target/grant/AI candidate exists yet.
3. H explicitly adopts the offer after reviewing its minimal owner/runtime summary. The target appears without a grant and remains held/paused.
4. D independently authorizes the target through the existing consent UI. Only the selected target becomes mentionable to current participants.
5. D offers the same selected resource again and withdraws this second unadopted offer. The original target, grant and controls remain equal. The second agent is never offered or disclosed to P/H.
6. H uses `@all` plus the selected agent in the actual preview/comment form. One AI input remains waiting; P and D each receive exactly one mention notification for the exact comment/event, and H receives none. No collaboration run, group or ordinary agent dispatch is created.

Every resource POST is correlated with its actual request header and original resource-operation GET. Browser request evidence records path/method and ambient-workspace-header presence only, excluding credentials. The returned evidence includes exact issue, workspace, actor, runtime, agent, offer, target, grant, request, comment, command and notification identities.

## Native reuse and screenshots

Exports:

- `seedCollaborationResourceOfferJourney()`
- `resourceOfferAccountPath(fixture)`
- `exerciseCollaborationResourceOffers(ownerPage, organizerPage, fixture, { navigate, capture? })`

The caller authenticates D/H pages first. Navigation and capture are injected so the coordinator can use actual Electron deep links and `capturePage`; the helper creates no native/browser stand-ins and starts no environment.

Web screenshot names:

- `resource-offers-supplier-wide.png`, `resource-offers-supplier-narrow.png`
- `resource-offers-adoption-wide.png`, `resource-offers-adoption-narrow.png`
- `resource-offers-authorized-wide.png`, `resource-offers-authorized-narrow.png`
- `resource-offers-withdrawn-wide.png`, `resource-offers-withdrawn-narrow.png`
- `resource-offers-zh-history-wide.png`, `resource-offers-zh-history-narrow.png`

Captures scroll the actual supply/adoption form, authorized resource item or offer-history region into view. They assert complete focus-element viewport bounds, document width, and actual PNG dimensions at 1280×1000 and 390×844. They do not reset the account detail to its top. Chinese coverage uses the real synthetic H profile language plus the Web locale cookie in a separate page.

Attachments: `resource-offer-journey-identities` and `resource-offer-capture-geometry`.

## Authoring checks actually run

| Check | Result |
| --- | --- |
| `pnpm exec playwright test e2e/collaboration-resource-offers-web.spec.ts --list` | Exit 0; exactly one Chromium test discovered. No browser/fixture execution. |
| `./packages/core/node_modules/.bin/tsc --noEmit --module ESNext --moduleResolution Bundler --target ES2022 --skipLibCheck --strict --esModuleInterop --types node e2e/fixtures/collaboration-resource-offers.ts e2e/collaboration-resource-offers-web.spec.ts` | Exit 0. |
| `./node_modules/.bin/eslint --config packages/core/eslint.config.mjs --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true}]' e2e/fixtures/collaboration-resource-offers.ts e2e/collaboration-resource-offers-web.spec.ts` | Exit 0. The E2E fixture legitimately consumes root-declared `pg` and Playwright dev dependencies; the source-only rule classifies a fixture `.ts` as production without this test-context override. No rule or dependency file was changed. |
| `git diff --check` | Exit 0. |

The first stronger `--noUncheckedIndexedAccess` check exposed an existing unrelated `e2e/fixtures.ts:442` unchecked result row plus a new native-Fetch `status` property/method typo. The new typo was corrected. The shared legacy fixture was left untouched; the final strict command above is the recorded passing scope.

## First production Web result

The coordinator ran the real test on build `eX3BXb5wTmAJSFzdab670`. `/tmp/multica-gap-recovery-20261021-h6phxwo9/s2-web-first.json` records one failed execution, zero passes/skips/flaky results. Supply and adoption POSTs succeeded; waiting for the owner's consent POST timed out after 15 seconds because the form rejected its inconsistent snapshot before transport.

Current production captures and trace are under `.omx/reports/cwc-gap-recovery-20261021/web-first/collaboration-resource-off-c74df-independently-authorizes-it-chromium/`. The coordinator inspected the supplier/adoption wide/narrow UI and recorded a visual-only pass, score 92, in `.omx/reports/cwc-gap-recovery-20261021/visual-first-offers.json`, then released the behavior edit. These four screenshots do not establish successful consent, withdrawal, the later mixed comment, Chinese history or native acceptance.

The implementer read that trace and `/tmp/multica-gap-recovery-20261021-h6phxwo9/s2-web-first-state.json`. Owner page `page@80ef4e1e761bf460ae96c8fb5c3712b3` has this exact monotonic order:

| Trace fact | Time |
| --- | --- |
| `call@164`: click Refresh resources | 6974.764 |
| GET resources returning target-set version 2 | Request started 6991.845 |
| `call@166`: click Authorize | 6993.272 |
| GET current resource identity | Request started 7054.683 |
| `call@171`: click Confirm authorization | 7058.846 |
| GET issue returning target-set version 2 | Request started 7075.8 |

The previous owner issue snapshot had target-set version 1 and no target. The form captured that issue with the new resource projection; a subsequent issue refresh could not repair the already-captured pair. `prepareCollaborationResource` correctly rejected the inconsistency. No consent POST appears in the trace. The fixture's consent assertions, timeouts and navigation were not changed to conceal this failure.

## Behavior repairs after the first capture

Only `resource-panel.tsx`, `resource-form.tsx`, `resource-offers.tsx` and the two canonical resource view test files changed in this follow-up. The S3 execution-panel/history mount, core version checks, API/schema/locale files, E2E journey and processes were left untouched.

- Resource offer selections are discarded when their current action authority disappears: an adoption loses organizer rights, a supply loses its open-issue authority, or withdrawal no longer belongs to the viewer. This removes only local unsubmitted selection and never deletes a core pending-operation draft. Restoring authority does not reopen that discarded form.
- Opening resource management refreshes the authoritative issue. Refresh resources now refreshes the issue prefix, including its resource projection, and explicitly clears the unsubmitted form/error so a user can select a fresh observation.
- Register/adopt opening requires matching issue/lifecycle/target-set observations. Owner consent/revoke opening also checks the actual target identity, target epoch and configuration version. A newly returned resource cannot open consent against an older issue with no such target.
- An open consent preserves its captured observation. Changed target configuration, lifecycle, actor epochs or current consent eligibility disable confirmation; explicit Refresh resources then permits a fresh selection. The core's independent final validation remains intact, and no read or refresh automatically replays a write.
- Existing appearance/classes are retained. The stale confirmation uses the existing localized stale-request status and leaves Close available; no new visual design or locale text was added.

Canonical regressions:

1. `releases an unsubmitted adoption when the handler loses organizer authority without restoring it later`.
2. `releases a hidden supply form on close so the owner can still withdraw their offered resource`.
3. `preserves the original pending operation when organizer authority disappears during submission`.
4. `waits for coherent issue and resource observations before opening consent after another actor adopts` — uses the actual issue Query with a deterministic deferred HTTP response, resources arriving first, and a later configuration change requiring explicit refreshed selection. No timer sleep or page reload substitutes for the fix.

The focused pre-fix command produced **3 failures / 1 pass**; the pending-operation preservation case already passed. After repair, all four executed cases passed. Ten unrelated cases were excluded by the diagnostic `-t` filter and then included in the final unfiltered commands below. A single intermediate assertion needed `waitFor` for TanStack Query's scheduled observer notification; the assertion and product behavior were retained.

## Follow-up verification actually run

| Command | Result |
| --- | --- |
| `DEBUG_PRINT_LIMIT=1500 pnpm --filter @multica/views test collaboration/resource-offers.test.tsx collaboration/resource-panel.test.tsx collaboration/execution-panel.test.tsx` | Exit 0; **3 files / 19 tests passed**, no skips/failures. |
| `pnpm --filter @multica/core test collaboration/resource collaboration/switch` | Exit 0; **7 files / 65 tests passed**, no skips/failures. Preserves the reviewer's two visibility-refusal regressions. |
| `pnpm --filter @multica/core typecheck` | Exit 0. |
| `pnpm --filter @multica/views typecheck` | Final rerun **exit 0**. The initial attempt was blocked only by concurrent S3 `command-history.test.tsx:9` importing a core fixture through a relative path outside views' `rootDir` (TS6059). Its owning lane/coordinator moved that import to the existing package test-fixtures export; this lane then reran the unchanged command successfully. |
| From `packages/views`: `pnpm exec eslint collaboration/resource-panel.tsx collaboration/resource-form.tsx collaboration/resource-offers.tsx collaboration/resource-panel.test.tsx collaboration/resource-offers.test.tsx` | Exit 0. |
| `git diff --check` | Exit 0. |

## Pending integration results

- Coordinator's production rebuild and actual Web rerun through consent/withdrawal/mixed comment/Chinese captures.
- Actual Electron/native execution, 200% zoom and packaged-Main evidence.
- No AC-I03 or parent/full-product completion is claimed by these component checks or the first visual-only pass.
