# R27 managed-submission preview RED independent check

Verdict: **ACCEPTED_R27_SUBMISSION_RED_BOUNDARY**. The retained genuine RED
justifies a separate, one-file initialization of the temporary preview guard's
bound submission ID. It does not accept GREEN, full preview locking, or S2.

Scope: retained source and evidence inspection only. This reviewer spawned no
agents and executed no product/test binary, TestMain, DB/SQL, API/browser,
lint/type-check, build, process-management or Git command. Only this report,
its JSON companion and new private reconciliation support were written.

Private root: `/Volumes/artisan/.multica-acceptance/pay241-r27-preview-20261117-x12xjirm`.
Support: `submission-red-independent-check/retained-evidence-reconciliation.json`.
The saved native hook, task PRD/design/implementation context, check manifest,
repository guidance and Trellis check role were loaded. This dispatch's explicit
read-only ownership supersedes routine self-fix/check execution steps.

## Findings (fixed)

None. Product, tests, configuration, dependencies and earlier evidence remain
read-only. No product simplification was made by this reviewer.

## Findings (not fixed)

- **R27-SUB-C01 — temporary submission guard is uninitialized.**
  File: frozen `server/internal/service/collaboration_completion_preview.go:241`.
  `GetCollaborationCompletionPreviewInTx` supplies the actual issue/access and
  successful stage facts but omits `boundSubmission`. The unchanged
  `collaboration_stage_completion.go:402` predicate rejects every valid ID
  before its submission query. Both real submitted positives return
  `stage_incomplete` / `verification:stage_submission_unbound`, with nil service
  error and no original PgError. This is a current service defect, independent
  of the unobserved historical HTTP409 holder.
  Recommendation: parse the already-validated target submission ID once,
  initialize only that temporary guard's `boundSubmission`, and pass that same
  ID to the unchanged checker. Preserve all row, lifecycle/candidate/policy/
  role/scope/release/execution, operation/receipt and handoff checks. Leave this
  to the separately dispatched product owner; no checker, writer, access or
  lock predicate is authorized to change by this evidence.
- **R27-SUB-G01 — wider acceptance remains open.** The generic non-null-ID
  branch also covers `collaboration_admin` with a current submission. Its
  separate genuine R38 binding regression is still required before GREEN
  acceptance; nomination, disabled-creator, organization/password and original
  access locks must remain intact. This five-leaf serial slice does not prove
  preview read/read coexistence, its nested lock chain, reader/writer fences,
  revocation races, or same-source normal/race controls. Those are separate
  gates, with no product changes by this review.

## Verification

- **Source identity:** all 5,049 actual files and permission modes match the
  frozen manifest. All 5,048 accepted R26 records remain identical; the sole
  extra is `collaboration_completion_preview_managed_submission_test.go`
  (SHA256 `f1aa77c0749504ee1826942c1511e5ad751a2254c8385ac69c2b0514ebc865e7`).
  All 5,055 permit inputs and retained implementation/AST/compile/binary/result/
  stdout/stderr references match their recorded hashes; zero drift.
- **Inventory and command:** retained real Go AST and raw Go JSON agree on
  one top-level, five leaves, six named identities. The exact permitted command
  is `go test -json -count=1 -timeout=10m ./internal/service -run
  '^TestCollaborationCompletionPreviewManagedSubmission$'` from the frozen
  `source-red-r01/server`, with no ambient environment inheritance. The profile
  hash matches compile, allocation, permit and execution; its URL database and
  `TASK_DATABASE_NAME` both identify the admitted RED service database.
- **Lint:** not run; explicitly prohibited by this read-only dispatch.
- **TypeCheck:** no new run. Retained isolated Go test compilation exited 0,
  without timeout or stdout/stderr; binary SHA256
  `67f63f18ddf2fbdeb73f9a28c373dbd181a20117b658524d1ee6e0f719d4b697` matches.
- **Tests:** retained intentional RED, exit 1; **3 PASS / 3 FAIL named**, including
  the failed parent. Zero SKIP, missing, extra, pending, parse errors or reruns.

| Leaf | Retained actual result |
| --- | --- |
| `enforce_authorization_available` | FAIL: false submission blocker; no PgError |
| `policy_off_non_authorizing` | FAIL: false submission blocker; no PgError |
| `stale_target` | PASS: original 409 `delivery_completion_conflict` |
| `wrong_actor` | PASS: original 403 `delivery_completion_denied` |
| `expired_credential` | PASS: original 403 `delivery_completion_denied` |

Each leaf uses the existing managed/password fixture and real registration,
claim/permit, typed report, human code/QA/release decisions, controlled deployment
and verification advance. Existing synthetic CI/artifact/integration inputs are
explicitly fixture evidence. M1 policy precedes actual `SubmitAcceptance`; its
real confirmation, persisted stage submission and fresh `ResolveIssueAccess`
build the exact target. No SQL-seeded submission, fabricated ready response or
new M1 authorization is introduced. Preview borrows the actual `pool.Begin`
transaction, keeps the five-second action budget/default isolation, then checks
the final credential and commits, or rolls back on refusal. Both positives reach
and commit that path; their assertions fail afterward on the returned blocker.

All five complete 72-relation plus stage-exit before/after hashes match. The
adapter call/target/invocation cleanup assertions show no failure, including
after the positive Fatal. Authorization, terminal and evaluation rows remain
zero. Retained DB metadata has other connections 0 before/after and identical
ledger/indexes. Post-cleanup counts retain five handoffs and configuration/
knowledge invalidation growth; these setup/cleanup facts were not erased or
misrepresented as an unchanged whole database. No all-table, archive-byte or
live external integration audit is claimed.

R24/R25 original holders remain unproved; their failures and retries stay
historical. Full preview lock acceptance, matching Web/Desktop original four
scenarios, A01–A12/final S2, normal recovery, S3, real S4 and final S5 remain
open. Parent 24 cases remain **0 PASS / 9 PARTIAL / 15 NOT_RUN**.
