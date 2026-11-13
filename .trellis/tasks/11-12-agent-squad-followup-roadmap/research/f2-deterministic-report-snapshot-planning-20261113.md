# F2 Deterministic Report Snapshot Planning

Status: design and acceptance conditions only. This document does not authorize
F2 implementation, a new route, a migration, a Reporter capability, a model
call, or a rollout.

## Existing source inventory

- `projecthealth.StatisticsSnapshot` records `calculated_at`, planning
  timezone/reference date, project revision, aggregate counts, a
  `snapshot_version` digest, and incomplete reasons. Its current `complete`
  predicate only means there is no `unknown_status`; it does not certify
  source pagination, lifecycle history, membership history, or overall
  coverage.
- `project_update_revision.statistics_snapshot` retains that current
  project-health calculation inside a versioned human progress update. It is
  a parity baseline, not a standalone snapshot resource or source-ID/revision
  manifest.
- `activity_log` records issue lifecycle activity and is ordered by
  `(created_at,id)`. The public issue Timeline endpoint deliberately caps
  activity at 2,000 rows and reports `X-Timeline-Truncated`; it is not a
  complete F2 source feed. `(created_at,id)` is an ordering cursor, not a
  global monotonic commit sequence. F2 must materialize from one authorized
  database snapshot or record the exact incomplete/truncation boundary as a
  gap.
- Current issue/project rows cannot alone reconstruct historical status or
  project-membership transitions. If a requested window needs a transition
  absent from retained activity, F2 must report unknown/gap or narrow the
  metric contract; it cannot infer history from `updated_at` or the latest row.

The first comparison gate runs the candidate calculation read-only beside the
existing project-health/progress-report calculation for matched scopes and cut
times. It reports which fields agree, which existing fields cannot prove
coverage, and the authority chosen for each metric. This gate does not
dual-write issue or project state.

## Frozen result contract

Every report snapshot must have an immutable `snapshot_id`, `protocol_version`,
`semantics_version`, `scope`, `timezone`, half-open `[start,end)` window,
`as_of` bound to the database read snapshot, source identities and revisions,
per-source watermarks/cursors with their ordering definition, materialized
source-ID manifests or verifiable digests, coverage, gaps, and materialized
result facts. A later read of the same snapshot returns the stored facts and
digests; it does not re-run the current issue/project query or silently adopt a
newer source revision.

The frozen scope records workspace, project (when present), included issue IDs
and the report issue's self-exclusion rule. Scope membership is evaluated at
`as_of`; an issue moved in or out later cannot rewrite the old snapshot. A
reopened or cancelled issue is represented by the event/state facts visible at
the frozen watermark, with the later transition retained as a later fact.

## Sources, watermarks, coverage and gaps

Source rows identify the issue/event stream, project membership projection,
status history, and retained report inputs. Each source has a frozen
watermark/cursor and `complete`/`unknown` state. Use the transaction's
consistent read cut as `as_of`; a per-issue `(created_at,id)` cursor alone is
not a global commit watermark. A source that cannot materialize every row in
its declared cut is a gap, not an empty result. Coverage is explicit:
`eligible`, `included`, `excluded`, and `unknown` counts plus gap records and a
coverage formula/version. Unknown and late records never collapse to zero.

Late data arriving after `as_of` or after a source watermark is excluded from
the frozen result and recorded as a named gap for a later snapshot. Pagination
must preserve the source cursor and deterministic ordering, including equal
timestamps; a repeated page cannot duplicate or drop an item.

## Required acceptance cases

1. Re-reading a snapshot after pagination, a late event, reopen, cancel, or
   project move returns the original result digest and watermarks while the
   caller retains Reporter authority. If the caller's Reporter capability or
   current issue/project access is revoked, replay is denied without disclosing
   the snapshot body; revocation is not converted into an `unknown=0` result.
2. A current report excludes its own report issue, while the snapshot records
   that exclusion and does not hide unrelated eligible issues.
3. A missing source, incomplete page, conflicting cursor, or source permission
   failure while collecting the snapshot produces an explicit gap/unknown
   result and nonzero unknown coverage; it cannot be presented as an empty
   successful report. This source-collection gap is distinct from requester
   authorization loss at replay, which returns a denied response with no
   snapshot body.
4. Scope/timezone/window/as-of and semantics version are visible in the API
   and retained artifact. A new snapshot is required for a new window or
   source revision.
5. A report whose source issue is reopened or cancelled after the snapshot is
   read as historical consumed facts plus the separately read current state;
   no historical fact is rewritten.

## Reporter minimum-permission gate

The route shape is still a planning candidate: read-only current/frozen report
lookup anchored to the current Reporter issue. The contract is fixed: current
report lookup binds the verified task's `IssueID` and frozen `ProjectID`; a
historical snapshot lookup also requires explicit `snapshot_id`. Every read
uses the same repeatable-read authorization and source snapshot. A Reporter
without a project may read only its current report; it cannot enumerate a
workspace or another project. Snapshot reads do not mint, consume, or extend
completion authorization.

The route must be added to the closed `TaskCapabilityGuard` allowlist only as a
GET, with exact path arity and canonical issue/snapshot IDs. `POST`, `PUT`,
`PATCH`, `DELETE`, duplicate query keys, path ambiguity, workspace-wide list
requests, mismatched project IDs, attachment access, and credential exchange
remain denied. The handler must read the actual frozen scope through the
transaction snapshot, not trust a project or actor header. Today the guard
enumerates only `/api/issues`, comments, timeline and delegation receipt reads;
the new report route remains denied until its exact GET shape and data-level
scope checks are admitted together.
Reporter responses contain only the allowed current/frozen report projection;
they do not expose private artifact keys, other projects, hidden counts, or
the ability to create or submit a completion authorization.

## Planning exit criteria

F2 can leave planning only after the result schema, source/watermark model,
coverage/gap semantics, replay test matrix, and Reporter route authorization
tests are reviewed together. No F2 implementation permission is inferred from
M1-S1 approval or from this handoff.
