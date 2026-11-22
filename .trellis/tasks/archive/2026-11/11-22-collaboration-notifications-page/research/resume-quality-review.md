# Notification page continuation review

The continuation resumed the existing in-progress task at its R6 follow-up and
production acceptance gate. Three native Trellis agent roles reviewed existing
implementation, old E2E entry adaptation and the full task contract. No task
creation or product planning decision was needed.

## Completed owner checks

- Core: 145 route/API/schema/query/access cases plus two actual notification
  mutation cases passed.
- Shared Views: the final composed page/link/workbench/activity/detail/sidebar,
  scroll and locale inventory passed 194 cases in 12 files.
- Desktop: account tab/store/navigation and route/tab/window inventories passed
  180 distinct cases in seven files.
- Go: seven named notification handler/service tests passed on separate fresh,
  migrated task databases. Handler children cover tied-time filtered cursors,
  other-recipient sentinels, global unread counts, legacy filters and privacy.
  Focused Go vet and gofmt checks passed.
- Repository `pnpm lint` and `pnpm typecheck` passed. Lint retains 31 unrelated
  pre-existing warnings; it reported no errors.
- Independent full-scope and follow-up source reviews found no additional
  production or spec defect. `research/quality-review.md` retains the original
  R6 finding rather than retroactively claiming it had passed.

## Live-derived fixes

The first browser runs exposed assertions based on test mocks instead of real
primitives. Corrected ordinary-title button roles, Base UI boolean-active
presence, NumberFlow accessible numeric labels and committed-filter capture.
The original strict content, pagination, identity and offset assertions remain.

Live acceptance then found owner defects absent from the original unit tests:

- Mixed incoming Next queries/outgoing hashes could reject an unavailable
  notification. The Web adapter publishes one complete matching destination.
- Next default page scrolling and UA fragment restoration competed with the
  notification page's inner mementos. Valid notification router operations use
  `scroll: false`; the physical notification entry temporarily uses manual
  browser restoration, retaining the original mode on generic routes.
- Native Window history callbacks follow registration order. The existing
  intent/listener moved outside Suspense into the outer layout lifetime, with
  synchronous event publication. Real event traces and a no-framework browser
  auto/manual control are retained in `web-return-diagnosis.md` and JSON files.
- Confirmed row removal could recover focus to a row link before the Query DOM
  projection deleted it. Validated receipt/filter membership now selects a
  stable list/back/detail target; four actual Query/receipt regressions cover
  delayed projection, refresh failure, unrelated live focus and All-view reads.
- Preserving a mounted list during notification selection does not emit a new
  scroll event. The departing account route therefore needs an authoritative
  snapshot into the existing memento map before navigation. The final owning
  platform fix and live replay are recorded separately.

## Acceptance and evidence boundaries

R17 passed five of six production Web/actual Electron journeys: native full
notification processing/restoration, native recorded results/popout/restart,
native global scope restoration, Web recorded results and Web global scope
restoration. Its remaining Web notification list-return failure is preserved;
only the final complete replay can satisfy the whole task.

The task-owned detached QA source exists because another environment holds this
checkout's shared Web build. Runtime records bind the API database, listener,
compiled production Web, native renderer/preload and current source hashes.
The native harness uses public human setup plus synthetic retained records;
no provider account, real agent subprocess, daemon dispatch or runtime-ingestion
claim is part of this acceptance. Independent offline/recovery allocations were
statically collected, retaining their original runtime gates.

The notification spec and its Core/Views/Web/Desktop/Server pointers now describe
page navigation, view markers, complete filters, exact targets, truthful
processing, retained history and the owning platform contracts. Final visual,
runtime cleanup, source verification and commit evidence belong to the
coordinator's verification record.

## Final quality gate

Final r19 completed all six production Web/actual Electron journeys with no failures, skips, flakes or retries. Both notification acceptance records contain complete filter pages, retained same-issue nodes/offsets, cross-page/scope restoration, confirmed processing and zero page errors. Final lint reports six successful tasks with the same31 unrelated warnings; typecheck reports nine successful tasks. Owned renderer/Web/API listeners are stopped and all three ports are free. No acceptance gap remains in AC1–AC7. The source/spec inventory64/11 and comment-only final byte delta are independently reviewed.
