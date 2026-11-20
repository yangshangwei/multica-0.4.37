# Codex step 7a management API implementation

> Final takeover update (2026-11-20): the previous pending full Views r4 run completed with exit0,608 files/7441 tests. Original report text below records its earlier review cut. The213-file source manifest was independently verified unchanged before the documentation/comment closeout; all selected normal/race evidence is retained. Final shared-branch integration has separate evidence in `codex-takeover-20261120.md` and the final delivery record.


Status: implementation complete in `/Volumes/artisan/code/2026/mc-f3-completion-20261120`; final combined normal/race evidence belongs to main. No commit, provider execution, installed agent CLI, original-worktree edits, design/PRD edits, or dependencies.

## Files and interface

- `internal/service/issue_budget.go`: `IssueBudgetForbidden`, current owner/admin recheck immediately after isolation declaration and before any management lock or original-key replay. Added `transactManagement`; retained `transact(ctx, callback)` for system lifecycle calls.
- `internal/service/issue_budget_read_wire.go`: frozen management response DTOs, using the existing stored operation result and pure `budget.Ticks`, `Count`, `ReservationProjection`.
- `internal/service/issue_budget_read.go`: `Show`, `Binding`, `LookupOperation`, `PageAttempts`, `PageObservations`; strict microsecond-preserving canonical keyset cursor, UTC RFC3339Nano formatting and nullable conversions.
- `pkg/db/queries/issue_budget_read.sql`: bounded attempt/observation tuple-keyset reads, root/workspace-scoped, newest first, `limit+1`; generated `issue_budget_read.sql.go` produced by main. No migration changes.
- `internal/handler/issue_budget.go`: all eight handlers plus `RequireIssueBudgetHuman` before the generic task-capability guard, and `RequireIssueBudgetWorkspace`, which preserves normal workspace middleware behavior for every non-budget route and translates only budget authorization failures.
- `internal/handler/issue_budget_wire.go`: exact field-set validation, explicit nullable amounts, duplicate/trailing/raw UTF-8 rejection, canonical request key and revision/amount parsing, 16 KiB cap, bounded page parameters. Reuses `iteration.ValidateObjectJSON` and the existing canonical `delegationKey`.
- `internal/middleware/issue_budget.go`: `IssueBudgetNoStore`, path match, and deny-only actor hints retained before Auth strips headers.
- New tests: service `issue_budget_read{,_integration,_plan}_test.go`; handler `issue_budget_{wire,api}_test.go`; middleware `issue_budget_test.go`. Existing service `issue_budget_test.go` statement lists gain the member read; missing-workspace enable now expects forbidden because membership precedes lookup. Sibling 5c adds its own resume statement to that list.

Handlers: `EnableIssueBudget`, `GetIssueBudgetBinding`, `GetIssueBudgetEnableOperation`, `ShowIssueBudget`, `ManageIssueBudget`, `GetIssueBudgetOperation`, `ListIssueBudgetObservations`, `ListIssueBudgetAttempts`. Main wires routes and all three middleware hooks; no router file edited by this lane.

## Decisions and exact boundaries

- Reads use one pool-backed REPEATABLE READ, READ ONLY transaction for each composed projection/page/lookup. Binding is one workspace-scoped SELECT. No advisory/row locks, writes, cache publication, or blind root-row fallback amounts. Existing non-locking ledger loader controls cache validation; current snapshot never publishes over a newer committed cache.
- `Show` merges ledger unknown counts with existing `CountIssueBudgetUncoveredGaps`; new reasons count automatically. Pre-activation ledger reason and uncovered legacy gaps are separate. Remaining uses the pure package’s A+R+H arithmetic and is null unless available/known/limited. Mechanism exclusions are fixed in the specified order.
- Observation page uses the existing task/generation/workspace coverage predicate and explicitly requires `coverage_gap`, so other observation kinds always have covered=false. No unbounded observation list is reachable from HTTP.
- All management responses use typed structs, exact field order and explicit nullable fields. Empty lists are `[]`; all 26 frozen fixture hashes unchanged. Stored JSONB operation results are decoded and re-encoded, never passed through as DB text.
- Shared authorization order: JWT/PAT human allow-list → owner/admin context → root/key syntax → strict body/page → issue/root access → service. The service rechecks current membership before replay as well as mutation. `X-Agent-ID`/`X-Task-ID` presence is forbidden; `X-Actor-Source` is forbidden if any value is nonempty, while an all-empty value list is allowed, per design 12.1.
- Reads evaluate `loadIssueForUser` using a shallow request copy with Method=POST to apply the same ordinary-issue edit boundary; no HTTP write occurs. Otherwise a collaboration-authorized GET could return unknown operation results for a non-ordinary issue. Confirmed with main; tests require collaboration enable422/read404.
- `UpdateIssue` adds field-specific delivery/admission/agent-autonomy guards, not a further human editing ACL for this operation. Existing shared issue-loader query failures may become404 before the budget status wrapper sees them; preserving that helper is explicit design scope.
- Loader refusals are captured, never forwarded as sentences. Root tombstones bypass the missing issue loader only on root-addressed routes; issue-addressed routes remain404.
- Cross-review caught missing membership’s generic404 before the handler: fixed by the narrow workspace wrapper. Non-budget responses are unchanged; budget missing-member403, malformed workspace400, transient failure500.
- Shared policy/no-write comparison covers complete rows of `project`, `project_delivery_policy`, `project_knowledge_policy`, `autopilot_quota_period`, `autopilot_quota_reservation`.

## Canonical regression map

| Matrix / rule | Test |
|---|---|
| B39/B58 credentials, future kind, missing principal, hints, restricted actor, all eight routes | `TestIssueBudgetManagementHumanGate` |
| B39 owner/admin all eight routes and all four management kinds | `TestIssueBudgetManagementOperations` |
| B14 foreign root/issue/key, missing indistinguishability, order403→400→404 | `TestIssueBudgetManagementPrivacyAndParsingOrder` |
| B40 stale revision, epoch on resume; B41 demoted replay, other-admin stale recovery | `TestIssueBudgetManagementOperations`; `TestIssueBudgetManagementMembership` |
| B41 no member-row lock, five transaction statement lists | `TestIssueBudgetManagementMembership`; `TestIssueBudgetTransactionsFollowTheLockOrder` |
| B42 refused current invocation cannot reserve or mint another token | `TestIssueBudgetManagementRevokedInvokeCannotReserve` |
| B44 tombstone, exact errors, UTC/cache headers | `TestIssueBudgetManagementLoaderAndTombstone`; `TestIssueBudgetNoStore`; main router tests |
| B48/B50/B52 nonzero A/R/H, reservations/unknowns, off retention, shared policy unchanged | `TestIssueBudgetManagementShowCoverageAndOff` |
| Membership removed before middleware, original-key lookup, unchanged ordinary middleware | `TestIssueBudgetManagementWorkspaceMiddleware` |
| Frozen 26-file bytes and HTML-sensitive reason | `TestIssueBudgetManagementFrozenWire` |
| Strict body/page matrices | `TestIssueBudgetManagementStrictBody`; `TestIssueBudgetManagementPageParameters` |
| Exact arithmetic, unknown merging, unavailable rows, null versus zero | `TestIssueBudgetReadProjection` |
| Canonical cursors and microsecond UTC times | `TestIssueBudgetReadCursorAndTime` |
| Snapshot/read statement order/cache/covered gaps/unavailable ledger | `TestIssueBudgetReadSnapshotAndUnknowns`; `TestIssueBudgetReadConcurrentSnapshot` |
| Stable 250-row pagination with tied times and unrelated root | `TestIssueBudgetReadPages` |
| Committed/stale/unknown original-key lookup, excludes attempt scope | `TestIssueBudgetReadOperationRecovery` |
| B43 existing F2 capture/exact read/terminal withdrawal | Main final `TestF2ReportSnapshot*` regression run, not duplicated |

## Verification and retained failures

DB commands only through main’s `/tmp/f3-codex-20261120/run-db.py`, exact `management_service` and `management_handler` permits. All pure/compile commands used explicit unreachable `DATABASE_URL=postgres://invalid:invalid@127.0.0.1:1/f3_no_database?sslmode=disable`.

| Run | Result |
|---|---|
| pure service projection/cursor and middleware no-store | passed, no DB use |
| `cx7a-svc-01` | 8 pass / 0 fail / 0 skip |
| `cx7a-hdl-01` | build-only failure during sibling VerifiedTaskID integration; no tests ran |
| `cx7a-hdl-02` | 32 pass / 4 fail: fixture comparison expected omitted zero enable field; deletion fixture replaced chi context. Fixed fixtures, no product loosening |
| `cx7a-hdl-03` | 36 pass / 0 fail / 0 skip |
| compile-only service/handler, `go vet` changed packages, targeted middleware rerun | passed before final new assertions |
| main `cx-router03` | 7 pass incl actual auth/capability/membership chain, claim→usage→replay→show, real deletion |

Final new B42/workspace tests, expanded nested fixture-key assertions and query-plan test must be included in main’s fresh normal/race run; earlier counts do not claim them executed.

`TestIssueBudgetReadQueryPlans` is ready: 20,000 rows in target root and 200 other roots ×50, both attempt/observation first and middle page plus unique operation lookup. Reports compact actual rows, loops, buffers, sort/index and ms to test output. Await final run; no index change is justified by unexecuted plans. Existing root-only indexes mean bounded output may still scan/sort a whole root. Main owns any migration change if evidence warrants it.

Independent read-only 6a cross-check: `research/codex-crosscheck-6a.md`, no confirmed blocker.

STATUS: implemented; final combined normal/race and query-plan evidence pending main
