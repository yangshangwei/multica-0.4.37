# S06 alert backend verification

Status: bounded alert backend implemented, independently reviewed and verified. Parent production/browser integration and S07 capacity/native acceptance remain separate. Upstream S04 commit: `8ba975947`. Worktree: `/Volumes/artisan/code/2026/multica-platform-admin`.

## Files and integration

- `server/internal/service/admin_alert.go` and its tests: fixed-rule detection, source episode lifecycle, versioned administrator handling, durable operation receipts, bounded failure hints and cyclic recovery scans.
- `server/internal/handler/admin_alert.go` and its tests: alert list/detail/actions, signed stable pagination, all-age active scope and bounded history, permission-filtered DTOs and a controlled resolution-code projection.
- `server/pkg/db/queries/admin_alert.sql`: source/episode/lease/CAS/lineage/list queries. Parent owns migrations500–511, generated SQL, shared operation-kind and audit integration, router and worker startup.
- Root hooks: `NewAdminAlertService(q, tx, deploymentID, liveness).Run(ctx)`, nonblocking `NotifyTaskFailed(taskID)`, and `DetectorHealth(ctx, organizationID)`. Liveness uses a structural two-method interface matching the existing store; no service-to-handler dependency or new package dependency.

## Behavior

- Rules are installation_unreachable, queue_timeout and execution_failed. Failures are one alert per task attempt; optional five-minute grouping is not implemented. Repeated event hints/scans do not increment the occurrence count or recreate closed failed facts.
- `condition_active` is independent of open/acknowledged/resolved/closed handling state. Acknowledgement is not recovery. Manual failure closure keeps the immutable failed source fact deduplicated. Queue/installation recovery ends the episode, and a later observed abnormal transition receives a new ID.
- Queue age uses the parent migration's queued_at/source clock. Transition clocks are exact for the new transition; observation clocks prove only a lower bound. Missing clocks never fall back to old task creation time.
- Installation alerts require saved execution installation/binding provenance, current matching runtime/credential evidence and in-flight work. Runtime-only historical tasks cannot acquire inferred installation ownership. Missing/truncated evidence remains unknown; configured liveness failure pauses offline opening and recovery. No Redis configuration uses the established DB heartbeat evidence.
- All liveness calls happen outside database locks. Detector leases are rechecked before the alert/checkpoint transaction; an expired/superseded worker cannot apply stale offline evidence after a newer observation.
- Batches contain at most200 source subjects. Failure history uses completed_at within the cycle's fixed31-day window, not created_at. Cursor cycles return to the start; a fixed cycle_started_at horizon prevents sustained new tail rows from starving below-cursor late commits. Bounded event hints accelerate current failures without blocking the task event bus.
- Alert/checkpoint writes commit atomically. Source or detector-write failures preserve existing alerts and mark the detector unavailable. Unknown observations accumulate across batches so the final clean batch cannot erase earlier uncertainty.
- A successful global DB absence observation can resolve a deleted queued task as task_removed. An extant unmapped/foreign-scope task and DB query failure remain unknown/unavailable. This records the queue condition ending, never execution success.
- Only full-password super administrators can acknowledge, assign or close; assignees must be currently enabled full-password super administrators. Actor/assignee locks follow the existing sorted-user/global-admin protocol. Concurrent handling uses expected_version and audited operations.
- Definitive version/state/assignee/resolution conflicts commit failed receipts under the original actor/key before returning409. Replays and lookups recover those terminal receipts. Audit failure rolls back the target and operation.
- Nonfailure alerts close only after verified resolution. Failed facts require handled, no_action_needed or retry_succeeded plus a reason; retry_succeeded checks an actual completed same-organization retry/rerun descendant. No task result is rewritten.
- DTOs and audit snapshots share a semantic resolution-code whitelist. Stored arbitrary diagnostics become unknown. No workspace inbox records or external notifications are sent.

## Read contract

The alert list uses immutable first_seen_at/id ordering and the shared signed actor/organization/filter/as_of cursor. Active/open/acknowledged defaults cover all ages; resolved/closed/all history defaults to31days, and explicit time filters are respected. The envelope uses a data_quality string with detector_health as a sibling; incomplete detection never erases already-known alert records.

Mutation responses are `{operation,target}`. Alert versions are decimal strings; common operation versions retain the existing numeric representation. The existing S05 `GET /api/admin/users?role=super_admin&status=active` supplies paginated assignee options, with server eligibility revalidation on assignment.

## Verification evidence

All executions used explicit `DATABASE_URL=postgres://multica:multica@localhost:5432/multica_platform_admin_s01_test?sslmode=disable` and `go -C server` from this worktree. Service fixtures use isolated schemas. Handler process reservations were coordinated because their legacy TestMain uses a shared fixture slug.

Final commands:

```text
go -C server test -race ./internal/service -run '^TestAdminAlert' -count=1
go -C server test -race ./internal/handler -run '^TestAdmin(Alert|Overview|Audit|Health|Workspace|Current|Task|Execution|Issue|Installation|Unassociated)' -count=1
go -C server vet ./internal/service ./internal/handler
git diff --check
```

- All17 alert service regressions passed with race in23.827s.
- Combined alert + S06 read + S03 compatibility handler regressions passed with race in18.681s.
- Vet and diff checks passed. Raw final output: `.omx/reports/platform-admin/s06-alerts/service-race.log` and sibling `handler-race.log`.
- Initial red tests reproduced unsupported mutation integration, an unrecognized resolution text leak through DTO/audit, detector write failure leaving a healthy status, and a missing assignee leaving no definitive receipt; each was repaired and retested.
- Independent review reproduced the moving cycle horizon under sustained full tail batches and the unclosable deleted-task queue alert. Fixed-horizon and deletion-vs-unmapped/source-failure tests passed, including the final full race run.
- Additional coverage includes dual administrator assignment, observer denial, audit rollback, real retry lineage, old active alerts, cursor/filter binding, conservative queue clocks, late commits, duplicate hints, unknown commits, superseded detector leases and proof that liveness reads occur outside detector locks.

## Independent read-backend review

Reviewed the other lane's overview/health/audit/workspace/settings handlers and SQL. Three findings were handed back and repaired by that owner: inactive alert counts/drilldowns used inconsistent windows; historical resolution snapshots used only a shape regex; and unknown live execution states did not degrade completeness. The final combined handler run includes those repairs.

Confirmed denominator-zero and absent usage remain null, cancelled outcomes are separate from success rate, failed execution usage is included, tokens are not reported as invoices, finished/current time bases are distinct, private resource content does not gain an admin bypass, and health/retention/configuration uncertainty remains explicit.

## Remaining limits

This slice has not run the S06 production Web/API acceptance flow or measured the1000-installation/million-task workload. Parent owns build/source provenance, browser evidence and S07 capacity/native acceptance. No real installed daemon/provider, user messaging channel, data-retention deletion job, commit or deployment was invoked by this subagent.
