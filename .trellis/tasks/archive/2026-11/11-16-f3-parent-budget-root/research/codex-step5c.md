# Step 5c C/D: off-mode and quick-create attribution

Completed in `/Volumes/artisan/code/2026/mc-f3-completion-20261120` only. No source in the original F3 or caller worktrees was modified. The existing lifecycle reconciliation implementation is unchanged.

## Behavior

- `issue_budget_binding.go:165`: reparent retains lifetime member rows while the root is off, but suppresses root-conflict, parent-cycle and in-flight observations. The mode is read by a separate nonlocking query after the exclusive binding lock for first binding; off also skips the in-flight scan. The READ COMMITTED assertion remains.
- `issue_budget.go:606`: off→observe appends exactly one permanent `coverage_gap` / `observation_off`, including the new `policy_epoch`, in the management transaction. Replay returns the original operation without another gap. Observe→off appends none. A failed later policy update rolls back the gap.
- `service/issue.go:83,582` and `handler/issue.go:3215,3326`: `VerifiedTaskID` is populated only from `auth.VerifiedTaskTokenFromContext`, scoped to the creation workspace, then forwarded to the creation seam. Headers and origin JSON do not populate it.
- `issue_budget_binding.go:102`: member inheritance happens first. With a verified task, read its latest attempt across all states; only attempts with no issue ID qualify. A differing actual created-issue root (including no root) appends one attribution observation to the execution's root. No amount, attempt, receipt or root row is changed. Same-root, ordinary-task and off-mode creation append none. Human creation does not look up attempts.
- One query was necessary: `issue_budget_attribution.sql`, `GetLatestIssueBudgetAttemptForTask`, scoped by workspace/task and ordered by descending claim generation. Main regenerated `issue_budget_attribution.sql.go`; no migration or index was added.
- The design fixes the observation kind but not the mismatch reason/detail names. This implementation uses `quick_create_root_mismatch`, with `created_issue_budget_root_id` (empty string when unbound) and `context_parent_issue_id`. The observation carries the created issue, verified task, latest generation and attempt IDs.

## Changed files

Production: `server/internal/service/{issue_budget_binding.go,issue_budget.go,issue.go}`, `server/internal/handler/issue.go`, and the new SQL/generated query files above. The shared `issue_budget.go` change is confined to `SetMode`; 7a's `transactManagement` remains intact.

New regressions: `server/internal/service/issue_budget_attribution_test.go`, `server/internal/handler/issue_budget_attribution_test.go`.

Affected contracts updated:

- `issue_budget_test.go`: create seam argument; rename root guard to `TestIssueBudgetBindingNeverLocksOrUpdatesARootRow`, allowing only the plain mode read; add resume observation to the lock-order list.
- `issue_budget_binding_lock_test.go`: create seam argument and mode read after binding lock.
- `issue_budget_finalize_test.go`: resumed root now includes `unbounded_unknown` alongside `limit` because the off interval is permanently unknown.
- `issue_budget_writers_test.go`: static guard requires the creation seam's fourth argument to be `p.VerifiedTaskID`.
- `handler/issue_budget_concurrency_test.go`: expect one off-period gap per root and no other observations while preserving every existing binding/invariant assertion.

## Verification

- `go -C server build ./...`: pass.
- `go -C server vet ./internal/service ./internal/handler`: pass.
- `gofmt -l` over all affected Go files: empty, exit 0.
- Service and handler `go test -c`: pass. Compile/pure commands used explicit unreachable `DATABASE_URL=postgres://invalid:invalid@127.0.0.1:1/f3_no_database?sslmode=disable`.
- Pure `TestIssueBudgetWriterInventoryIsComplete`: 1 pass, 0 fail/skip.
- Main-permitted `attribution_service`, run `cx5c-svc-01`: 15 top-level / 24 including subtests passed, zero failures/skips. Log: `/tmp/f3-codex-20261120/cx5c-svc-01.jsonl`.
- Main-permitted `attribution_handler`, run `cx5c-hdl-01`: 3 passed, zero failures/skips. Log: `/tmp/f3-codex-20261120/cx5c-hdl-01.jsonl`.
- DB slot released to main after both runs. No race run in this lane; final integrated normal/race runs remain the coordinator's acceptance gate.

New named tests and proof:

| Test | What it proves |
| --- | --- |
| `TestIssueBudgetObservationOffResumeIsAtomicAndReplayed` | Two empty off intervals each leave one epoch-specific permanent gap; replay writes none; policy-overflow failure rolls back the gap; ledger unchanged |
| `TestIssueBudgetOffBindingKeepsMembersWithoutObservations` | Create/reparent keep lifetime bindings, cross-root/cycle/in-flight observations and scan suppressed while off; no root/attempt/entry/operation changes |
| `TestIssueBudgetOffBindingInterleavesWithResume` | Real connections: binder reads off then resume commits before binder; and resume commits while binder waits, then its fresh mode read observes on and scans; both leave permanent unknown |
| `TestIssueBudgetQuickCreateRecordsEachDifferentRoot` | Zero/one/multiple creations, other/no/unbound roots, latest settled attempt, same root, ordinary task, human and off cases; original execution attribution and ledger retained |
| `TestIssueBudgetQuickCreateIgnoresSelfReportedOrigin` | A supplied quick-create origin cannot substitute for the verified task seam argument |
| `TestIssueBudgetCreateUsesVerifiedExecutionIdentity` | Real HTTP creation with another same-agent task as origin selects the verified task's root; actor headers/JSON without verified context produce no attribution observation |
| `TestIssueBudgetVerifiedCreationTaskIsWorkspaceScoped` | Missing or foreign-workspace verification supplies no task identity |

Also ran affected isolation/binding lock lists, finalize-off, lifecycle-off/superseded, mode-epoch, management lock-order, and concurrent-handler writer tests. No existing assertion was removed to obtain a pass; contract changes are enumerated above. Initial transient compile failures during signature propagation and one unused test import were corrected before DB execution.

## Remaining limits

No provider/ambient agent CLI, deployment, enforcement, billing reconciliation, commit or migration. The new attribution is an observation only. Missing/damaged root mode reads fail the enclosing issue write; no root locks were introduced. Final file fingerprints are `/tmp/f3-codex-5c/manifest.json`; shared files may legitimately change afterward under their other owners.

## Cross-review correction: additional task-authorized creation ingresses

The independent 6a reviewer identified two additional ordinary issue creators that accept custom task credentials: lifecycle handoff and triage intake. Both now pass the same private-context `VerifiedTaskID` into `CreateInTx` (`handler/lifecycle_handoff_transaction.go`, `handler/triage.go`). The generic SQL writer guard had only pinned the central service seam, so it did not detect these missing transport parameters.

- `TestIssueBudgetLifecycleFollowUpUsesVerifiedExecutionIdentity`: quick-create execution bound to root A creates a lifecycle follow-up under root B; child membership is B, one attribution difference remains on A, economic rows unchanged.
- `TestIssueBudgetTriageCreationUsesVerifiedExecutionIdentity`: quick-create execution bound to A creates a parentless triage issue; no member row is fabricated, one attribution difference remains on A, economic rows unchanged.
- Main-permitted `attribution_handler` run `cx5c-hdl02`: these two plus the earlier ordinary-create/identity tests passed 4/0/0; log `/tmp/f3-codex-20261120/cx5c-hdl02.jsonl`. `go vet ./internal/handler` passed afterward. DB slot released and product changes frozen.
- Direct production `IssueService.CreateInTx`/`Create` callers were enumerated: ordinary create, lifecycle and triage are now wired; comment sub-issue is protected by `RequireHumanActor` at `cmd/server/router.go:2532`; collaboration creation checks `Principal.Human()` in `service/collaboration.go:192`; stage materialization follows `stageHuman`. These other paths do not forward an ordinary task identity.

STATUS: complete
