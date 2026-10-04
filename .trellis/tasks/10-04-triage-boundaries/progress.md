# Boundary lane implementation and verification

Lane owner: triage_boundaries_impl. Worktree: `/Volumes/artisan/code/2026/multica-triage-t1`. No commits made; other lanes own schema513–534, triage handlers/imports and client/UI. Parent owns the final535 down-migration retention guard.

## Implemented

- `internal/admission` is the one formal-state allowlist (`not_required`, `accepted`) and typed review-required error. Supplied stale Issue structs do not authorize execution.
- Migration535 captures comment `dispatch_eligible` in the same write transaction as comment creation and adds `lock_issue_execution`. All pending-era comments remain non-dispatchable after acceptance, including fresh enqueue, merges, planned obligations, replay/retry, delivery and runtime start.
- Agent SQL fences direct/deferred enqueue, retry, manual quick-create retry, source-context/parent quick-create, issue linking, deferred promotion, singular/batch claim/reclaim, input merge/planned registration and start/wait/delivery. Final claim token creation is independently fenced even when no comment receipt is requested. Issue-less ordinary chat/autopilot/quick-create remains supported.
- Service guards run before enqueue overlays/rerun cancellation/recovery and validate persisted state. Lifecycle source and reused targets are blocked before preparation and again under transaction locks. Existing invocation, assignee, originator and rollback behavior remains intact.
- Generic issue PATCH/batch refuse status/assignee/project/parent/stage/position writes on nonformal inputs with HTTP409 `triage_review_required`, issue ID and link. Content remains writable. All single/batch writes use the existing locked update primitive; content-only Public API updates use dedicated content SQL, avoiding restoration of stale candidate fields after acceptance.
- Ordinary list/open/group/table/facet predicates, child lists/progress, project totals, suggested assignees, working-agent/squad/runtime statistics exclude nonformal inputs. Explicit global search/detail/history remain readable and expose `admission_status` on HTTP and realtime DTOs. Ordinary duplicate matching cannot silently reuse a pending triage input.
- Pending content/comment events set `suppress_execution`; the plugin bridge respects it. Ordinary triage acceptance remains a separate triage event owned by backend lane.
- Project resource create/update/delete now consume a materialized parent `FOR NO KEY UPDATE` fence. This conflicts with the execution-context snapshot's project `FOR SHARE` lock and prevents resource-set phantoms between fingerprint and task insertion. Create rechecks project/workspace identity.
- Canonical issue deletion removes live issue_triage and undelivered notification rows while preserving action/intake/import tombstones and historical duplicate references.

## Important lock decision

An initial queue `FOR SHARE` issue fence broke the existing legitimate lifecycle concurrent enqueue/savepoint race. T1 formal admission is monotonic: `accepted`/`not_required` cannot transition back to nonformal admission. `FOR KEY SHARE NOWAIT` therefore protects issue existence, reads authoritative admission, refuses pending snapshots until acceptance commits, and preserves lifecycle's `FOR NO KEY UPDATE` behavior. **Any future formal -> nonformal transition must redesign this fence.** Documented in migration535; parent reviewed the rationale. No queue trigger was introduced, and retry refusal returns no rows without aborting failure settlement.

## Regression evidence

Meaningful PostgreSQL RED tests reproduced before fixes:

- FinalizeTaskClaim minted execution credentials for a pending issue when comment receipt recording was disabled.
- Plugin bridge dispatched four execution-capable hooks despite a triage suppression flag.
- ChildIssueProgress counted five total children but only two formal done children.
- Title-only batch loaded pending state, raced acceptance, and restored a null assignee on the accepted issue.
- All three project resource mutations escaped a held execution snapshot lock; new tests observe real `pg_blocking_pids` waits on a separate connection after the fix.

Initial state-matrix RED encountered the absent schema before backend migration, and was not treated as behavioral proof. After schema installation, the complete state matrices ran against real PostgreSQL (not skipped).

Commands (source `.env.worktree`; fake CLI guard always enabled):

- `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service -run 'TestTriage|TestEventBridge|IssueTrigger|RerunIssue|ClaimTask|Enqueue|SourceContext' -count=1` — passed.
- `python3 /tmp/multica-triage-run-locked.py bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/handler -run 'TestTriageBoundary|TestIssueToMap|IssueTrigger|IssueTable|TestUpdateIssue|TestBatchUpdate|LifecycleAtomic|RerunIssue|CommentTrigger|ProjectResource' -count=1` — serialized with shared fcntl lock; latest result below.
- `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/service ./pkg/db/generated ./internal/admission -count=1` — full service/generated suites passed before the final small query/batch additions; final rerun below.
- `make sqlc` — passed after final SQL changes. Coordinated with backend lane; no hand edits to generated sqlc query output. `generated/comment_row.go` is the repository's handwritten converter and now retains dispatch eligibility.
- `git diff --check` — passed.

Tests own real actor/workspace/agent/runtime fixtures without running installed agents. Coverage includes 90 blocked/formal state × enqueue/claim/replay/deferred/link/source scenarios, late comment obligations, final credentials, failure settlement, stale content, nonformal HTTP mutation/comment/lifecycle scope and formal controls. Final integration, browser/mobile compatibility and full repository checks remain parent-owned.

## Final lane checks (2026-10-05)

- Final broad handler suite including the stale-batch regression, comments/lifecycle/table/ProjectResource formal controls: **PASS, 3.993s** (`/tmp/triage-boundary-handler.log`).
- Final complete `internal/service` suite: **PASS, 29.480s**; complete `pkg/db/generated`: **PASS, 1.080s** (`/tmp/triage-boundary-service-full.log`). No ambient agent executable used.
- `go -C server vet ./internal/admission ./internal/service ./internal/handler ./pkg/db/generated`: **PASS**.
- Final `git diff --check`: **PASS**.
- No required implementation work remains in this lane. Parent integration/independent review,535 downgrade rehearsal, full repo/browser checks and task commit remain outside this lane.

## Full-repository follow-up (2026-10-05)

The parent's full `make test` exposed gaps not reached by the earlier focused handler expression. Fixed without changing existing test expectations:

- Legacy grouped list's inner SELECT and scanner included admission_status but outer SELECT did not. Added the missing outer projection; grouped pagination/involvement/status sorting now retain their established behavior.
- Move's project existence lookup accidentally included the issue-only admission predicate. Removed it from the project query; retained it on issue move anchors.
- Queue SQL gate conflated missing/malformed optional quick-create source metadata with nonformal admission. Only resolvable source/parent issue references now participate in the admission allowlist. The established claim handler still owns invalid context/workspace rejection and cancellation; immutable snapshots survive source deletion and follow-ups become top-level as before.

All reported boundary failures plus triage regression controls pass under `-race` (2.585s). First full handler `-race` rerun had no boundary failures but revealed backend's independently confirmed immediate-outbox clock race and one source-context cleanup failure; full-suite completion remains pending those checks below. Earlier lane-complete statement is superseded by this follow-up.
