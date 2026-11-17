# Lane C: context confirmation and optional reads

Date: 2026-11-18. Scope: C05, C07 and the context-history portion of C08.

## Change boundary

- `packages/views/collaboration/workflow.tsx`: reuse status-options `refetch()` and show read loading in the existing form; retain all confirmation and current-status guards.
- `packages/views/collaboration/memory-panel.tsx`: require `active` for history and selected-version observers; keep the current-context root read, client state and navigation consumption. Bound the local restore/purge confirmation with a nonshrinking title/footer and scrolling body.
- Adjacent `workflow.test.tsx` and `memory-panel.test.tsx`: real-component retry, hidden Query observers, legal multiline reason and snapshot-confirmation regressions.

No shared primitive, Core query, locale, task-state, plan or specification edits. No agents, commits or live API/browser writes. Other lanes retain ownership of their changes.

## Baseline

The four assigned files were clean before Lane C edits. All native task/spec/research context was loaded, including `research/knowledge-remediation.md`. Impeccable context already ran in the main session; this lane read the craft floor and relevant adapt/harden/optimize references.

C05 uses the retained original-component Chromium failure: a legal 1,000-byte, 40-line reason produces a 1,245px popup at 320×568 and a 1,041px popup at 1280×800, with actions outside the viewport. Main owns the current-CSS geometry confirmation. Unit tests will verify reason/snapshot/request semantics, never Tailwind class strings.

## Regression evidence

### Red: behavioral regressions before production edits

Command:

```sh
pnpm --dir packages/views exec vitest run collaboration/memory-panel.test.tsx collaboration/workflow.test.tsx -t 'retries failed status candidates|suspends hidden historical pages|suspends a hidden selected version' --maxWorkers=2
```

Result: exit 1; 2 files failed, 3 tests failed, 34 skipped. Exact output: `research/lane-c-red.log`.

- History: expected two initial page GETs while hidden; got six after 30 seconds of interval polling.
- Selected version: expected one initial GET while hidden; got four after invalidation, refocus and reconnect.
- Workflow: could not find the accessible `Refresh` button after a transient 503 status-options response.

These failures were verified before any production change. The implementation now adds only observer visibility gates and the existing Query retry/loading wiring. C05 keeps the title and action footer nonshrinking, moves description/reason/errors/stale-version content into the scrolling middle, bounds textarea growth, and preserves existing action/snapshot guards.

## Resumed verification

The first combined run found a test-clock issue in the history reopen assertion:
the assertion timed out reporting three calls, but temporary diagnostics showed
all four expected calls (`undefined`, `"2"`, `undefined`, `"2"`), two retained
pages, and a successful idle Query. Only `setInterval` was faked for the hidden
polling check; Testing Library's `waitFor` also uses that interval, so it could
not retry its first assertion when structurally unchanged data caused no DOM
mutation. Restore real timers after the 30-second hidden-polling assertions,
before invalidation and reopening. Exact two-page reread and editor/page
retention assertions remain intact. No Core or production change was needed.

The failed attempt is retained in `research/lane-c-first-green-attempt.log` and
the temporary diagnostic output in `research/lane-c-history-diagnostic.log`.
Temporary instrumentation was removed before final verification.

### Green: focused component and compatibility suites

Command:

```sh
pnpm --dir packages/views exec vitest run collaboration/memory-panel.test.tsx collaboration/workflow.test.tsx collaboration/workflow-upload.test.tsx collaboration/source-bridge.test.tsx --maxWorkers=2
```

Result: exit 0; **4 files, 48 tests passed**, no failures or skips. Final run
started at `07:19:38` and took 7.63 seconds. Exact output:
`research/lane-c-green.log`.

This run includes both new C05 semantic regressions: the exact 1,000-byte,
40-line reason is retained and sent once only after explicit confirmation, and
restore remains disabled after snapshot drift until the current version is
explicitly reviewed. It also covers hidden historical pages, hidden selected
versions, unchanged editor nodes/drafts/pages, workflow retry and pending-submit
lock, permission revocation, original-key recovery, upload cancellation and
source-bridge success/cancel/refusal/recovery. No class-string geometry tests
were added.

### Scoped lint and diff checks

```sh
pnpm --dir packages/views exec eslint collaboration/memory-panel.tsx collaboration/memory-panel.test.tsx collaboration/workflow.tsx collaboration/workflow.test.tsx
git diff --check -- packages/views/collaboration/memory-panel.tsx packages/views/collaboration/memory-panel.test.tsx packages/views/collaboration/workflow.tsx packages/views/collaboration/workflow.test.tsx
```

Both returned exit 0. ESLint reported no findings; exact output is in
`research/lane-c-eslint.log` (only the existing pnpm configuration advisory).
The whitespace check had no output.

## Self-review and handoff

- Production changes remain confined to the existing history/selected observer
  activation, status-options retry/loading, and local context confirmation.
  Current-context root reads, current-status/permission guards, original receipt
  ownership and snapshot request semantics are unchanged.
- The confirmation has a viewport-bounded flex popup, a nonshrinking title-only
  header and action footer, and a scrollable middle containing description,
  reason, errors and stale-version feedback. The textarea has a 40dvh growth
  bound; action text wraps and coarse targets have a 44px floor. The visible
  reason label names the actual textarea. The global AlertDialog is unchanged.
- This resume changed only the history test's clock boundary and the lane's
  research evidence; the earlier production diff was retained after review.
- Main owns real compiled-CSS browser geometry at 1280/320/390 widths, short
  420px windows and 200% text in both themes, plus full typecheck/lint, static
  detector and integrated independent review. This lane does not claim browser,
  native Electron, physical-device or spoken assistive-technology verification.

No agents, commits, task-state/spec/plan updates, live API writes or unrelated
file changes were made by Lane C.
