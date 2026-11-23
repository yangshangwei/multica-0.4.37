# Trigger preview client increment

## Change boundary

The existing composer shows target admission and hold reasons but cannot explain
same-channel merging, the pending batch or a current squad's next leader
continuation. The server owns those facts; this client increment only validates
and presents its read-only projection. Submission remains authoritative.

Expected files: `packages/core/collaboration/content-schema.ts` and canonical
schema tests, `packages/views/collaboration/comment-composer.tsx` and its shared
component tests, and the English / Simplified Chinese collaboration locales.
The API transport test will cover the added projection at the actual response
boundary if needed. Existing locale edits belong to the baseline and stay intact.

The shared receipt parser keeps its existing queued / joined input rules. Missing
new preview data supports older servers; malformed present facts fail closed.
The existing suppression, comment / attachment / reply identity, scoped reads,
permission epochs and original request recovery remain in force. No scheduler,
permissions, runtime capability, app-specific routing or new dependency changes
belong to this lane.

## Validation plan

- Failing canonical schema regressions for additive facts, cross-target identity,
  malformed input and older responses before implementing the schema.
- Shared composer regression for actionable grouping facts, preview invalidation
  on suppression and changed input, and separation from confirmed receipts.
- Focused core / views tests, locale parity, affected package typechecks and lint.
- Main session owns the production browser environment and integrated review.
  Exercise direct A and squad S led by A, an existing pending batch, another
  same-leader squad, and an active squad continuation in that environment.

## Evidence

The server/client contract is an optional per-target `grouping` object:

```ts
{
  kind: "pending_batch" | "squad_continuation";
  target_ids: string[];
  has_existing_inputs: boolean;
  follows_pending_batch: boolean;
}
```

All four fields are required when present. The targets are the eligible,
unsuppressed same-comment members of one projected batch/continuation, including
the target itself. Membership must be reciprocal and all facts must agree;
unavailable/held targets cannot carry grouping. Continuation cannot follow a
pending batch. UUID normalization and duplicate checks precede correlation.
The object has no accepted command/group identity, counts or private input.

Core extends the preview target schema only. Old preview omissions still parse;
the existing receipt schema keeps its original command/group proof requirements
and strips unknown preview data. API transport continues sending the original
read-only preview input without an idempotency key or ambient workspace headers.

The shared composer presents each grouping once, with target names, pending or
continuation expectations, existing-input and later-batch facts, and a reminder
that sending rechecks permissions and queue state. English and Simplified Chinese
copy are colocated in the existing collaboration namespace. The existing reset
paths remove grouping and disable Send after suppression or text changes.

Checks run by the client lane (logs under
`.omx/reports/cwc-preview-wakeup-20261020/client/`):

- `pnpm --filter @multica/core test collaboration/content-schema.test.ts`:
  before implementation, 16 new regressions failed for missing/unchecked grouping;
  after implementation, all 23 tests passed.
- `pnpm --filter @multica/views test collaboration/comment-composer.test.tsx`:
  before rendering changes, both new UI cases failed for the missing grouping
  region while all three existing cases passed.
- Focused core schema, API content, comment recovery and access suites: 43 passed.
- Shared composer and locale parity: 71 passed.
- Full `@multica/core` and `@multica/views` typechecks passed. The first views
  check caught three test-only Playwright-style `exact` options; they were removed
  from Testing Library queries and the corrected check passed.
- ESLint for changed core/view source and tests passed; `git diff --check` passed.
- Playwright discovery for `e2e/collaboration-preview-web.spec.ts` passed (one
  browser journey discovered; discovery is not execution evidence).
- Explicit TypeScript compilation of the E2E spec and both fixtures passed.
- The shared UI mock was corrected to regroup direct A with S2 after suppressing
  S1; its five composer tests pass with that real FIFO expectation.

The added E2E fixture reuses the resource journey, registers/consents direct A
and two squads led by A through real API requests, and creates one real earlier
comment. It clears only those target holds in the isolated DB, leaving the issue
paused and runtime offline. The browser journey covers grouping and a later batch,
English/Chinese wide/narrow captures, keyboard suppression, a required new preview
and an actual comment receipt with two waiting targets and one suppressed target.
The resource-page authentication helper now takes an optional locale, defaulting
to English for all existing callers. No runtime or model is launched.

The main session owns actual production browser execution and visual inspection.
Independent review found that an external delivery-form update can bypass the
composer's local reset callbacks. Composer/source and test ownership was handed
to `review_increment` for full content/suppression/attachment/draft/reply input
correlation and its regression; that fix is recorded in the review lane's evidence.
Continuation is exercised by the Chinese shared UI regression
and the server's canonical service matrix; the pending-batch browser fixture does
not claim native runtime or squad lifecycle evidence. The parent task and rollout
gates remain unchanged.

## Production browser harness follow-up

The initial production journey reached the English grouping assertions and saved
its captures, then failed when the second tab remained in English. The screenshot
and page content showed a correctly authenticated issue; this was not a selector
or timing failure. Web uses the `multica-locale` cookie for server rendering, and
`UserLocaleSync` honors the account's saved language. The harness had set only
localStorage, which is the Desktop locale mechanism.

The E2E spec now saves `language: "zh-Hans"` for its synthetic creator and sets the
same-origin locale cookie before opening the Chinese tab. It explicitly verifies
`html.lang="zh-CN"` before testing Chinese labels. No product source or timeout
changed. The original `web-initial` failure artifacts remain intact.

The task-owned `run.py qa web-harness-fix ...` production run passed **1/1** in
5.9 seconds (runner exit 0). It proved the Chinese draft, grouping, keyboard
suppression, fresh preview and real two-waiting/one-suppressed receipt. Logs and
three English/Chinese screenshots are in
`.omx/reports/cwc-preview-wakeup-20261020/web-harness-fix*`.
The main session still owns visual verdict and a final run against the subsequent
independent-review product fix.

## Existing conflict journey compatibility

The final broader browser run passed four of five journeys. Its existing stale
lifecycle conflict test required the response body to omit `status`, although
the pre-existing dirty `server/internal/handler/collaboration_workflow.go` echoes
`status: 409`. The main session verified that handler was byte-identical to the
start-of-task baseline; no backend change belongs to this correction.

The E2E assertion now requires `code: "stale_revision"` and, when `status` is
present, requires equality with the actual HTTP 409 response. Missing status
remains valid. All same-page refusal, retained input, zero recovery/no automatic
retry, explicit latest-version retry and final-state assertions remain intact.
The canonical core regression continues to prove the statusless response shape.
The independent reviewer was notified of this test-only correction.

Only the previously failing test was rerun on the final production build:
`run.py qa web-conflict-final pnpm exec playwright test
e2e/collaboration-workbench-web.spec.ts -g 'stale lifecycle conflict' ...`.
It passed **1/1** in 3.2 seconds (runner exit 0); original failure logs were kept.
Evidence is under `.omx/reports/cwc-preview-wakeup-20261020/web-conflict-final*`.
