# Independent check: trigger preview and parent wakeup

Review date: 2026-10-20. Parent task remains in progress; this report does not
establish full acceptance or authorize creation/execution rollout.

## Parent wakeup findings (fixed)

None. The independent review required no production or test changes.

## Parent wakeup findings (not fixed)

No concrete defect found in the authorized wakeup increment.

The approved ordinary best-effort contract remains: a crash after the lifecycle
commit, comment/task write failure, or uncertain commit can lose a wakeup.
Original-key replay cannot reconstruct its transient effect. A durable scheduler
or transactional cross-issue coalescing would change the agreed boundary and was
not introduced. This is a verification limit, not an unreported implementation
failure.

## Parent wakeup review evidence

- Inspected the service lifecycle transition and its common workflow owner:
  status/session/history/receipt writes precede commit; mutation, receipt and
  commit failures discard the effect. Only fresh complete/cancel transitions
  carry it. Reopen, refusal and JSON receipt recovery carry none.
- Traced the only production `ChangeLifecycle` caller into
  `notifyParentOfChildDone`, the existing stage barrier, assignee dispatch and
  `TaskService.checkIssueExecution`. The parent remains ordinary; the child
  completion does not admit an ordinary task on the collaboration issue.
- Verified custom done/cancelled/backlog resolution, earlier/same-stage sibling
  barriers, unstaged behavior, squad leader provenance, pending-task dedup and
  current-parent/shared-parent guards against the real code paths and tests.
- The parent comment contains only the shared child's sanitized identifier and
  title, with no `mention://issue` link or private description. The transient
  before/after rows never enter HTTP, event or operation JSON.
- Reviewed the built-in working-on-issues skill and source-map changes. Their
  post-commit stage behavior and minimal-reference descriptions match the code;
  they do not promise durable delivery.

## Parent wakeup verification

Ran independently with Go 1.26.6, the assigned isolated PostgreSQL database from
`integration.env`, and `scripts/go-test-with-agent-cli-guard.sh`. Service and
handler database runs were sequential; no installed AI CLI was invoked.

| Check | Result |
| --- | --- |
| `go test -race ./internal/service -run 'TestCollaboration(Lifecycle\|Status)' -count=1 -v` | PASS: 16 top-level tests, 43 including subtests; 11.260 s |
| `go test -race ./internal/handler -run 'Test(CollaborationChild\|.*ChildDone\|ChildReopen\|Stage\|CollaborationOrdinaryChild)' -count=1 -v` | PASS: 35 top-level tests, 74 including subtests; 4.157 s |
| `go vet ./internal/service ./internal/handler` | PASS, exit 0 |
| `gofmt -l` on the six wakeup Go files | PASS, empty output |
| Scoped `git diff --check` including built-in documentation | PASS, empty output |

The two race runs had zero skips, failures or race reports. Go compile/type
checking is included in the successful test/vet runs. Logs are retained under
`.omx/reports/cwc-preview-wakeup-20261020/review/` as `service-race.log`,
`handler-race.log` and `vet.log`.

## Preview findings (fixed)

- Files: `packages/views/collaboration/comment-composer.tsx` and
  `comment-composer.test.tsx`.
- Issue: the composer only matched a preview's parent comment ID to the live
  delivery draft. Another form's external-store update bypassed its local
  mutation reset, so changed content kept stale grouping visible and Send
  enabled. Replacing a retired draft with identical text could also reuse the
  previous draft's preview.
- Fix: require the exact inspected draft UUID and matching content, reply,
  suppression, attachment selection and attachment draft ID before exposing
  the preview or enabling Send. One local state value retains the inspected
  draft identity; no request schema, server authority or stored intent changed.
- Regression: the shared composer test writes through the real delivery store,
  first changing content and then replacing the draft with identical text.
  Both cases reproduced the stale region before their fixes. A fresh preview
  restores Send, and the regression sends no comment automatically.

The frontend implementation owner also corrected its suppression test fixture
to reflect the canonical A+S2 merge after S1 is suppressed; reviewed that update
before taking exclusive composer/test ownership.

## Preview findings (not fixed)

No unresolved concrete defect found in the authorized preview increment.

## Preview review evidence

- Traced `PreviewComment` through the existing authorized repeatable-read,
  read-only transaction. It only invokes query/projection helpers; no admission
  lock, command, join, event, grant, notification, run or control write occurs.
- Typed deduplication/order and target authorization come from the existing
  mention owner. Preview and FIFO selection share the extracted squad
  compatibility and exact runtime-input serialization helpers and limits.
  Tests compare projected grouping against actual comment submission and FIFO
  selection, including a serialized-byte boundary and same-comment order.
- Held, suppressed and unavailable targets cannot enter grouping. Earlier
  inputs are checked against current author and target authority and contribute
  only flags, never content or private resource/command/group identifiers.
- Continuation inspection checks the current active squad, retained execution
  projection, live run, frozen dependencies/resources and runtime admission.
  Existing eligible FIFO inputs prevent joining. Consumed inputs count toward
  budget while only unread inputs set the existing-input flag.
- Maximum lexical canonical UUID selection matches the canonical join loop:
  `LockCollaborationGroupRuns` orders by PostgreSQL UUID `id` ascending, and the
  loop retains its last eligible run. No parent-order correction was needed.
- Core's additive preview-only object preserves old responses and receipt
  semantics, validates reciprocal eligible target sets and rejects malformed
  present fields with redacted errors. The composer displays each grouping once
  with bilingual factual wording; committed joined receipts stay separate.
- Reviewed the real-API paused-queue browser fixture and journey, the new
  `collaboration-preview-wakeup.md` spec and index links, and built-in skill
  changes. The main session owns production browser rebuild/execution and its
  visual evidence. No SQL/template/platform-config touch point is missing from
  this increment; SQL changes in the pre-existing dirty baseline are separate.

## Preview verification

| Check | Result |
| --- | --- |
| `go test -race ./internal/service -run 'TestCollaboration(CommentPreview\|CommentMixed\|SquadHuman\|FIFOMixed\|Batch)' -count=1 -v` | PASS: 13 top-level tests, 26 including subtests; 12.715 s; zero skips/races |
| Final `go vet ./internal/service ./internal/handler` | PASS, exit 0 |
| Scoped Go formatting and increment `git diff --check` | PASS, empty output |
| ESLint on the two changed core schema files and two changed composer files | PASS, exit 0 |
| `pnpm --filter @multica/core --filter @multica/views typecheck` | PASS, both packages |
| Core `content-schema.test.ts` and `api/collaboration-content.test.ts` | PASS: 2 files, 27 tests |
| Shared `comment-composer.test.tsx` after the correction | PASS: 1 file, 6 tests |
| Locale parity suite | PASS: 1 file, 66 tests |

Independent logs remain under
`.omx/reports/cwc-preview-wakeup-20261020/review/`: `preview-race.log`,
`vet-final.log`, `core-lint.log`, `views-lint.log`, `frontend-typecheck.log`,
`core-tests.log`, `composer-red.log`, `composer-draft-red.log`,
`composer-green.log` and `locale-tests.log`. Red runs deliberately selected the
new regression, so their other skipped cases are filtering, not acceptance
evidence; the final composer run executes all six tests.

No full-repository suite, browser/native runtime or provider execution was run
by this reviewer. The main session's integration/browser evidence is recorded
separately. This review leaves the parent task and rollout gates unchanged.
