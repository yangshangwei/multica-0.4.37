# Independent 7a cross-check by 6a implementer

Read-only review on 2026-11-20 of completion worktree management code against design §§12.1–12.9. The reviewer authored none of these files. Findings sent directly to 7a and leader. No product edits made for this review.

## Finding C7-1 — membership middleware bypasses frozen refusal shape

**Medium; fixed by author after review, verification pending.** `cmd/server/router.go` places all management routes under `middleware.RequireWorkspaceMember`. In `internal/middleware/workspace.go`, missing membership returns 404 `{"error":"workspace not found","code":"workspace_access_denied"}`; DB errors return 503 with a sentence; missing/invalid workspace input similarly emits the general middleware format. These responses occur before `Handler.budgetRequest` and its code-only refusal mapping.

A valid human JWT/PAT whose membership has been deleted therefore receives the generic 404 instead of design 12.1's 403 `budget_forbidden` (condition 2 is unmet). The handler's existing missing-member tests manually inject context and do not exercise this routing short circuit. No budget lookup is exposed, but frozen status/body semantics and uniform authority refusal are violated.

Suggested repair: a budget-scoped workspace boundary that retains current membership lookup and emits the frozen error shape; leave non-budget behavior untouched. Add a request through actual Auth + workspace middleware for a removed/nonmember human and a database lookup failure, in addition to the direct handler matrix. Sibling owns fix.

## Confirmed contracts

- Human allowlist accepts only verified JWT/PAT with neither restricted nor AgentActor. Pre-auth middleware preserves deny-only hints before Auth strips X-Actor-Source. Unknown credential kinds do not pass; task credential guard occurs before generic TaskCapabilityGuard.
- Authorization/role precede body and budget reads in handlers. Root/issue operations remain scoped by current workspace; original-key input uses canonical lowercase nonzero UUID and cannot select attempt operations.
- `transactManagement` reads current owner/admin inside READ COMMITTED before the root/fence/replay operations; it adds no member lock. Lifecycle retains separate unprivileged system transaction helper.
- `Show`, `PageAttempts`, `PageObservations`, and `LookupOperation` use one REPEATABLE READ READ ONLY transaction. Show reads root/member, replay and uncovered observations in that snapshot. No reader publication to ledger cache.
- Unknown reasons merge ledger and uncovered observation counts; pre-activation is separate; unavailable projection does not expose unverified amounts; fixed exclusion list and nullable counters match the frozen wire.
- Pagination uses strict `(created_at,id)` descending boundary and LIMIT+1; canonical cursor encoding requires UTC, valid UUID, microsecond precision, bounded length and exact base64 round-trip. No unlimited entry reader is exposed by HTTP.
- Request parsing preserves duplicate-key detection via `iteration.ValidateObjectJSON`, exact field spelling and presence, nullable amounts, decimal counts, kind-specific field sets and no trailing value. Response structs preserve field order/nullability and use UTC time.
- Recovery retains original result under issue/root scopes, marks stale against current policy revision in the same snapshot, and answers absent key as unknown plus original query path.
- Tombstones bypass issue content lookup only on root-addressed routes and expose retained economic IDs/state; issue-addressed routes still require the shared loader.

## Inherited limitation noted to sibling

`loadIssueForUser` itself collapses `GetIssueInWorkspace` database errors to 404 (`internal/handler/handler.go`, around 1177), so the status-only response writer cannot recognize and turn those errors into 500. Design 12.1 explicitly requires retaining this shared loader rather than refactoring it. This is inherited behavior, not introduced by 7a; disclose instead of claiming every DB failure is recoverable 500.

## Evidence limits

This review used source inspection, existing test assertions and frozen design, not a separate database run. The shared database lane remains owned by the management implementer. Actual-router credential/nonmember checks and normal/race suites still need coordinator verification. No security bypass, mixed snapshot calculation, recovery-key widening, or amount leak was found beyond C7-1's refusal-contract mismatch.

## C7-1 fix inspection

Author added `Handler.RequireIssueBudgetWorkspace`, delegating ordinary routes unchanged and using a status-only capture for management workspace resolution. Successful middleware passes the original writer and resolved member context directly onward; denied membership maps to 403 budget_forbidden, malformed workspace to 400 invalid_request, storage failures to 500 internal_error. Source reread confirms the finding is addressed without changing global workspace middleware. Leader owns routing it before management endpoints and runtime verification.
