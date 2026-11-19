# R28 submission-preview RED independent check

Verdict: **ACCEPTED_R28_RED_READ_BOUNDARY**. The genuine RED supports the
bounded three-file submission-only read design. GREEN and full preview/S2
acceptance are not established. This reviewer changed no source/config/history
and ran no SQL/DB, tests/builds, API/browser, signals, Git or subagents.

## Findings (fixed)

None. Only this report, its JSON and new private `read-red-independent-check/`
support were written; original RED source/results remain unchanged.

## Findings (not fixed)

- **R28-READ-C01: incompatible preparation-read fences.** Actual read/read
  refusals have original PostgreSQL 55P03 and correct root/child identities:

| Leaf | Held PID → second PID | Actual refused fence / phase |
| --- | --- | --- |
| `r4_first` | 2427 → 2458 | literal `issue_collaboration FOR UPDATE NOWAIT` / Exec |
| `preview_first` | 2460 → 2484 | R4 `LockDeliveryEvidenceIssueRead` on child / row_scan |
| `same_creator` | 2486 → 2507 | `TryLockCollaborationStageRootShared` / row_scan |

The held standing R4 reaches `ListStageExecutionIntents` after its genuine QA/
release/report/artifact chain and observed issuer collaboration/session fences.
Its release follows the second terminal. In the other two orders, the first
preview returns real availability, remains active/uncommitted through the
second terminal, then commits. Exact backend-pair logs, SQL hashes, UUIDs and
ordering assertions reconcile. No early-release/deadline assertion failed.

- **Repair remains implementation-owned.** Accepted design ownership is only
  `collaboration_completion_preview.go`, `delivery_completion_collaboration.go`
  and `collaboration_stage_completion.go`. Select private read entries solely
  for validated `collaboration_submission`. Reuse the exact existing value
  marker `collaborationStageEvidenceReadTx` around the actual borrowed Tx;
  preserve it through queries, source observations, retained QA issuer/run and
  final signer checks. Reuse existing shared artifact/observer/child helpers.
  Original intent/facts/state entries must dispatch constant false; original
  QA/access/observer defaults, self/admin paths and binding/UTC checks stay.

Use the actual id-only stage-task `FOR SHARE NOWAIT` literal, then the current
getter/details and existing virtual development head. No generated shared-task
symbol exists. Existing root SHARE plus schema/readiness checks and workflow
root UPDATE before task creation protect absent-head semantics; absence cannot
become ready or policy-off. Immutable submission/operation/handoff queries
already use SHARE and need no change. Fixing root alone leaves other upgrades.

## Verification

- **Source:** all 5,051 actual files/modes match the frozen manifest. All 5,050
  accepted R27 baseline records are unchanged; sole extra is the 602-line test,
  SHA256 `31885022def923d4d4d07a27855ab7d7e19f51776a69682ba61a2c1cc0993bf8`.
  Product 5fc, original managed f1aa and admin 658b remain unchanged. All 5,057
  permit inputs and implementation/AST/compile/raw references match; zero drift.
- **Lint:** retained gofmt exited 0; no new checker lint/vet execution.
- **TypeCheck:** retained single isolated Go test compilation exited 0, empty
  logs; binary SHA256 `b50cad40ef95785bae5708c6780fdac3bf146936175ff169aa19b0bdaccf44a6`.
  Implementer executed no test body/TestMain/DB; checker performed no reruns.
- **Tests:** exact 5 top-level/13 leaves/18 named terminals: **13 PASS / 5 FAIL /
  0 SKIP**, exit 1. Failures are the three read leaves and two parents only;
  zero missing/extra/pending/parse/retry. Source/profile/command identities match.

All ten control leaves pass: real M1 observer writer-first/reader-first, original
stage-state UPDATE with plain/marked Tx, disabled-account/password 404,
revoked-manager 403, revoked-QA issuer blocker, missing binding and actual stale
stage/rework. Unheld baselines establish genuine available preview and R4 data
before controls. Intentional negative setup precedes the new no-write baseline;
old cleanup is neither suppressed nor modified. No fake ready row/M1 grant.

Preview preserves default read committed; R4 executes production repeatable
read. Both keep the five-second action, 250ms artifact lock and ten-minute suite
budget. All 26 complete 72-relation/stage-exit comparisons are equal, M1
authorization/terminal/evaluation rows 0, and admin/control plus adapter snapshot
assertions report no effects. Final other DB connections 0; ledger/indexes match.
Setup/cleanup history growth remains recorded, not a whole-DB/archive-byte audit.

The new private helper names are design proposals. Same-source fresh GREEN/
race, default-writer/source/authority controls and independent review remain
required. Nonempty attachments, self/R38 read paths, original four/A01–A12/S2,
parent 24 and S3–S5 remain separate. Historical R24/R25 holders are not attributed
by this deterministic RED; synthetic external fixtures do not prove real S4.
