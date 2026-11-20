# R27 binding R01 independent check

Verdict: **NOT_ACCEPTED_R27_BINDING_R01_UTC**. The single-field initializer
correctly removes the false submission blocker for both human and R38 previews,
but the actual normal run fails the existing UTC expiry contract. R01 is not a
passing GREEN acceptance. Main explicitly retained the original race permit as
**NOT_RUN** after this failure and authorized a separate R02 source.

Private root: `/Volumes/artisan/.multica-acceptance/pay241-r27-preview-20261117-x12xjirm`.
Scope: new admin RED, the frozen R01 binding delta, retained compile/vet and
actual normal evidence. No checker product/test, DB/SQL, API/browser, build,
process-management, Git or subagent execution occurred. Only this report, its
JSON and new `binding-green-independent-check/` support were written.

## Findings (fixed)

- File: frozen `source-green-r01/server/internal/service/collaboration_completion_preview.go:241`.
  Issue: the temporary guard lacked the ID required by the unchanged
  submission check. Fix by implementation owner: added only
  `boundSubmission: completionUUIDPG(*request.Target.SubmissionID)`.
  Both actual enforced human/admin previews now return
  `authorization_available` with empty stage blockers. The original checker,
  row/operation/receipt/handoff predicates, current access/nomination, locks
  and writer defaults remain byte-identical. No product edit was made by this
  reviewer, and this finding does not make the failed normal run pass.

## Findings (not fixed)

- **R27-BIND-C01 — UTC response expiry, P2.**
  File: frozen `collaboration_completion_preview.go:294`.
  The final minimum can retain the database release timestamp's local zone;
  assigning `&expiry` emits `+08:00`. Human line68 and admin line171 fail only
  their `MaxExpiresAt.Location() != time.UTC` condition. Returned and expected
  expiry instants are identical and later than `CheckedAt`; binding, canonical
  observation and non-permission assertions otherwise pass.
  Task `research/wire-contract.md:65` requires RFC3339 UTC timestamps; the
  unchanged ordinary preview already calls `expiry.UTC().Truncate(time.Microsecond)`
  at `delivery_completion_preview.go:358`. This is a contract defect, so tests
  must retain their assertion. Main authorized the same normalization in a
  separate R02 file after all horizon minima and before the expiry comparison.
  It is outside this reviewer's write scope; R01 source/results stay unchanged.
- **Acceptance gates remain open.** R01 race was not executed. R02 needs its
  own frozen source, fresh normal/race evidence and independent review. The
  genuine current-submission R38 branch is now exercised; no new full matrix
  is required for unchanged R38 helpers. Full preview/R4 locking, missing-binding
  negatives, self/admin concurrency and original joint scenarios remain separate.

## Verification

- **Source:** actual admin RED and GREEN each match all 5,050 frozen files/modes.
  Admin RED preserves initial 5,049 files and adds only the 174-line admin test,
  SHA256 `658bb12c83611a78cb6974bc054e82bbb872a3d9a8ee51bfbb5a195db64be925`.
  GREEN differs from admin RED only by the exact initializer; the recorded
  patch matches actual bytes. Product SHA256
  `9272930e7d665f61eb7ad1c8bafbd412e02651aa77138c682678fa8765cdd3ba`.
  Initial test and accepted RED report bytes are preserved. All three permits'
  5,056 required input hashes match; zero source or artifact drift.
- **Inventory:** real retained Go AST and raw normal terminals agree on two
  top-level, six leaves, eight named tests; no skip, missing, extra, pending or
  parse errors. Original admin RED has one top-level/one leaf/two named failures.
- **Lint/static analysis:** no new execution; retained service `go vet` exit 0,
  no timeout, empty stdout/stderr.
- **TypeCheck:** no new execution; retained isolated Go test compilation exit 0,
  no timeout, empty logs. Binary SHA256
  `a2ad369a96e9a7e3bc3513cbc6b0f191b6755925655fc2f7ff178c43f01a05f2` matches.
- **Tests:** original admin RED **0 PASS / 2 FAIL named**, exit1, same real
  `verification:stage_submission_unbound`, no PgError. R01 normal **4 PASS /
  4 FAIL named**, exit1; failed parents are included. Race **NOT_RUN**, not PASS.

| Normal leaf | Actual retained result |
| --- | --- |
| Human `enforce_authorization_available` | FAIL: available binding; UTC expiry only |
| `policy_off_non_authorizing` | PASS: off, no binding/observation/expiry |
| `stale_target` | PASS: original 409 `delivery_completion_conflict` |
| `wrong_actor` | PASS: original 403 `delivery_completion_denied` |
| `expired_credential` | PASS: original 403 `delivery_completion_denied` |
| R38 `admin_available` | FAIL: available admin/org binding; UTC expiry only |

Admin setup uses the existing independent artifact/product signers, actual
platform-admin creator disable/revocation, current persisted admin password/
role/org and source membership. A real borrowed transaction performs current
nomination, obtains the genuine submission and builds the fresh admin target,
then commits; another borrowed transaction previews and commits. Source
membership does not replace nomination. There is no new M1 grant or admin
lifecycle completion. Simulated human/CI/artifact/integration fixtures are
explicit and do not prove real external execution.

Every admin RED and six normal leaves retain matching complete 72-relation
plus stage-exit hashes, zero M1 authorization/terminal/evaluation rows, no
original PgError and no adapter-effect assertion failure. The admin/control
snapshot assertions also report no change. Exact source/profile/command
bindings reconcile without ambient inheritance, with the original five-second
actions/default isolation and ten-minute suite budget. Retained before/after
other DB connections are 0; ledger/indexes match. Setup/cleanup handoffs and
configuration/knowledge invalidation growth remain recorded, not represented
as an unchanged whole database or an archive-byte audit.

R24/R25 holders remain unproved. Full preview, A01–A12/final S2, matching
Web/Desktop four scenarios and normal recovery, S3, real S4 and final S5 stay
open. Parent remains **0 PASS / 9 PARTIAL / 15 NOT_RUN**.
