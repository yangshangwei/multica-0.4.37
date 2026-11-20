# Independent bounded 5c cross-check

Read-only inspection on 2026-11-20 in `/Volumes/artisan/code/2026/mc-f3-completion-20261120`, requested by coordinator while 5c DB tests ran. No product edits. Scope: new observation-off and quick-create attribution changes against design §§10.6–10.7, plus reachable creation callers. This is not a new lifecycle sweep audit.

## C5-1 — verified task identity omitted by two reachable creation paths

**Medium; coordinator notified promptly.** The new `IssueCreateParams.VerifiedTaskID` is filled only by ordinary `Handler.CreateIssue`. Two other task-credential-accessible paths reach `IssueService.CreateInTx` with it unset:

1. `internal/handler/lifecycle_handoff_transaction.go`, `writeLifecycleHandoffInTx`, around line 398: new follow-up params set origin from `origin.ID` but not `VerifiedTaskID`. The route `/api/issues/{id}/lifecycle-handoffs` is not human-only. A custom mat_ credential passes `TaskCapabilityGuard` when its agent has no restricted profile. `lifecycleOrigin` / `trustedIssueCreationTask` verify live task, matching agent and workspace but do not require task.issue_id or require its issue to equal the handoff source. A quick-create task attributed to root A can therefore create a new follow-up under an ordinary source in root B and omit the expected root-A attribution_difference.
2. `internal/handler/triage.go`, `createTriageInTx` call to `CreateInTx`, around line 609: new parentless intake issue has no verified task parameter. `CreateTriageItem` calls `beginTriageWrite(..., false)` and explicitly permits contributor agents. A quick-create task under root A can create a parentless intake issue and omit the root-A mismatch observation required for new rootless issues.

Suggested fix: pass `verifiedIssueCreationTaskID(r.Context(), workspace)` at both parameter constructions. Do **not** reuse the lifecycle `origin.ID`: that older helper accepts checked actor headers, while this new attribution contract trusts only the server-verified credential context. Add one regression per reachable surface with a quick-create attempt and forged origin/hints not selecting the budget root.

Other inspected callsite: comment-to-child manual creation is behind router `RequireHumanActor`, so it does not need a verified mat_ parameter. Agent mode there enqueues a quick-create task rather than creating the resulting issue; the later ordinary issue-creation path supplies its verified identity.

## Confirmed contracts

- `verifiedIssueCreationTaskID` reads `auth.VerifiedTaskTokenFromContext`, checks workspace equality and never consumes origin or actor headers. Auth clears stale context before authenticating and sets this context only after task-token verification.
- Normal `Handler.CreateIssue` passes that value through the transport-neutral params, and `sanitizeIssueCreateParams` preserves it. `BindIssueBudgetOnCreate` binds the new issue according to its actual parent before comparing attribution.
- `GetLatestIssueBudgetAttemptForTask` scopes by workspace/task and orders claim generation descending with LIMIT 1; no state predicate causes a settled/latest attempt to fall back to an earlier one. The existing unique `(task_id,claim_generation)` index supports this lookup.
- Ordinary attempts (nonnull issue_id) and same-root creations write no difference. New rootless/different-root quick-create issues record one observation per created issue on the attempt's original root, with creating issue, verified task, chosen attempt and generation. This neither moves spend nor changes attempts/entries/root figures.
- Human creation skips the attempt lookup. Missing attempt has no observation. Off mode suppresses attribution observations while retaining member binding.
- Reparenting an unbound subtree takes exclusive workspace binding lock before its READ COMMITTED mode read; it writes member rows regardless of off state, then returns before any conflict/cycle/in-flight observations when off. Existing member moves read the original root's mode without acquiring a root lock and preserve permanent binding.
- `SetMode` writes the permanent observation_off gap only when returning to observe, in the same transaction as epoch/revision change and operation receipt. Original-key replay precedes reapplication. A failed subsequent root update rolls the new gap back. The gap is never treated as covered.
- Finalize checks off under root lock before append/attempt creation. Lifecycle reconciliation skips off before load or writes. No additional root/task locks were added to issue binding.

## Evidence limits

No DB commands were run for this review. Existing test assertions cover zero/one/multiple quick-create results, latest settled attempt, same/other/rootless outputs, ordinary/human/self-reported origins, off writes, repeated resume epochs, rollback, and both mode-read/resume interleavings. The missing transport propagation above is not covered by those existing assertions. Coordinator owns fixes and the final fresh normal/race suites.
