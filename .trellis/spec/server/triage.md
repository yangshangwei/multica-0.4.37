# Triage admission and manual review

T1 adds explicit intake and human review to an existing Issue identity. These
contracts describe the implementation and its regression surfaces; they are not
a release certification. See `docs/triage.zh-CN.md` for operator/user guidance and
`.trellis/tasks/10-04-triage-t1/api-contract.md` for the wire contract.

## Admission is independent of workflow status

- `issue.admission_status` is server-owned. Only `not_required` and `accepted`
  are formal work; `pending`, `rejected`, `duplicate`, and unknown future values
  must fail closed at execution boundaries. Do not infer admission from `todo`,
  `cancelled`, metadata, project association, or client UI state.
- Explicit manual/CSV intake reuses Issue content, comments and attachments,
  stores `pending` with `backlog`, and keeps candidate project/assignee in
  `issue_triage`. Ordinary issue creation and existing issues remain formal.
- Ordinary lists, search, table/group/facet queries, children, aggregates and
  project totals exclude nonformal issues. Direct detail remains readable under
  current workspace access. Duplicate targets must be formal same-workspace
  issues, including terminal issues; keep the historical identifier if deleted.
- Protected generic writes include status, assignee, project, parent, stage and
  position. Handler conflicts return HTTP 409 with `triage_review_required`,
  `issue_id` and `issue_path`. A missing field in an old client's request does
  not grant authority. `--no-start` is dispatch suppression, not admission.

Authoritative sources: `internal/admission/admission.go`,
`internal/handler/issue_admission.go`, `internal/service/issue_admission.go`,
`pkg/db/queries/issue.sql`, and `internal/handler/issue_table_query.go`.

## Actor and configuration boundaries

No settings row means supported and disabled, default acceptance `todo`, no
priority requirement, and responsibility `none`. Only human owner/admin actors
may configure triage; any current human member may review. Check both machine
credential class and `resolveActor`: legacy agent headers on a human PAT are
still agent decisions. Runtime-owner identity must not launder human approval.

Explicit intake permits agents under existing Contributor creation authority;
recheck the active actor and autonomy in the transaction. It grants no review,
CSV import, settings or execution authorization. Candidate visibility is not
invoke permission: accepting a real agent/squad assignment validates the current
human's permission again. Reviewer/responsibility wire IDs are user UUIDs, never
member-row UUIDs.

Responsibility `notify` sends a notice without assigning a reviewer; `assign`
also sets the reviewer on new intake/reopen. Existing rows are not rewritten.
Inactive reviewers remain intelligible through `reviewer_valid`; delivery and
new writes recheck membership. Disabling refuses every pending issue, including
snoozed ones. Settings disable and all pending-producing writes share the
workspace/settings fence, including initial settings-row creation.

## Decisions, transactions and execution

Four review results are accept, reject, duplicate and snooze. Reject/reopen
require 1–2,000 nonblank characters; snooze is a future instant within 90 days.
Reopen only applies to rejected/duplicate items while enabled, starts another
round, clears formal associations and retains all prior decisions. Batch preview
and commit independently allow only accept/reject/snooze/assign_reviewer.

Acceptance validates current status catalogue, priority requirements and scoped
references, then commits admission, fields, revision and before/after audit as
one transaction. Plain accept never enqueues, inherits a project squad, or emits
an ordinary execution-triggering event. Use `triage:updated` after commit for
cache refresh. Batch items commit separately and preserve per-item outcomes.

I1 adds optional `fields.current_iteration_id` only to accept/accept-and-execute
when `iteration_assignment` is advertised. Target locks precede pending issue
locks; final field validation and membership preparation precede the single
business-time sample and any label/admission mutation. The action ID is stable
across retries. Current agent/squad invocation checks must use the locked grant
rows, not a later permission read. See [iteration contracts](iterations.md) and
`triage_iteration_assignment_test.go` / `triage_iteration_authorization_test.go`.

`accept_and_execute` is a separate human intention. Store the accepted issue
and pending execution action before preparing external resources. Retain the
authorizing human, assignee/leader, runtime and project/resource/content context.
Retry belongs to the original human with current authority. Compare the saved
context both before external preparation and inside the enqueue transaction;
changes return a conflict requiring a fresh explicit run decision.

Task insertion and `triage_action.task_id` commit together. Finalize only after
commit. A failed start leaves the issue accepted; replay/retry uses the original
action, and a stored task ID is returned even after that task becomes terminal.
Reconcile uncertain commits by action identity instead of blindly enqueuing.

Take workspace/settings and revocation/subscriber fences before member and
owned-entity locks. Coordinate issue/action locks with the existing NOWAIT and
rolled-back retry protocol; never wait in an inverted lock order. Revalidate
membership and target references in the final write transaction.

Every enqueue, merge/coalescing, planned-input recovery, retry, claim and start
must read persisted admission; stale `Issue` arguments are insufficient. Shared
service/SQL guards and the `lock_issue_execution` database fence protect
non-HTTP callers. A pending-era comment persists `dispatch_eligible=false`:
acceptance must not make its delayed mention/plugin event executable. Preserve
that check for trigger and coalesced comments as well as current issue admission.

## Durable replay, history and notifications

- Manual intake and review requests use `(workspace, actor, request_id)` plus
  payload hash. Preserve the UUID for the same intention; changed payload is a
  new intention, while reuse with a different payload returns 409.
- Successful CSV rows are consumed identities. Deleting an issue removes live
  annotations but retains request/action tombstones and successful import-row /
  external-ID provenance. Intake/action replay reports the deleted result; a
  consumed import row returns its retained issue ID. Never recreate on replay.
- History uses decision snapshots, not a join that loses old rounds when the
  current issue reopens. CSV batch summaries retain cumulative result counts.
  Workspace deletion explicitly removes owned rows in its application transaction.
- Write durable notification identity with the decision, then deliver separately.
  Deduplicate by workspace/recipient/event and recheck membership. CSV sends one
  cumulative batch summary per recipient. Inbox read/archive/delete never changes
  admission, and delivery failure never rolls back an accepted decision.
- Snooze readiness is a timestamp predicate, not a required scheduled state
  mutation. Refresh clients within 30 seconds; validate current pending state,
  reviewer and snooze timestamp before delivering an overdue reminder.

## CSV validation and recovery

Use native `encoding/csv`, strict UTF-8 with optional BOM, maximum 5 MiB and
1,000 data rows. Reject whole-file encoding/size/parse/header failures without
truncation. Browser decoding uses fatal UTF-8; server rejects already-lossy
replacement characters. Validate title ≤500 runes, description ≤1 MiB, external
ID ≤1,000 UTF-8 bytes, valid date-only values and date order. Preserve original
cells for failure export, including invalid NUL cells through byte-safe storage.

Mapping is source header → canonical field; duplicate target mappings fail.
Priority cells accept the canonical values in any letter case plus the zh-Hans
UI labels (紧急/高/中/低/无优先级/未指定优先级); they are stored canonical and the
original cell is kept for failure export. Only CSV intake normalizes — the issue
and triage intake APIs stay strict. The downloadable template keeps canonical
values so it also imports on servers that predate the aliases.
Ignored state/iteration/attachment values warn rather than becoming workflow
state. Resolve project/labels by scoped unique name, members by email and visible
agents by name; missing, ambiguous or inaccessible references clear the candidate
with warnings, never auto-create resources. Preview persists plans only.

The import dialog's downloadable template is generated client-side from
`packages/views/triage/triage-csv-template.json` (BOM + CRLF, one header set per
UI locale). `TestClientImportTemplateMapsEveryColumn` parses that file, so adding
a mappable field or renaming an alias must update the template in the same change.

Batch detail/commit/failure download are scoped to the original human importer
and current workspace membership. Changed mapping creates a new preview request.
Selected valid warning rows are explicitly acknowledged. Invalid rows cannot
create. External-ID duplicates default to skip with an explicit per-row override;
similar titles only suggest candidates. Revalidate references during each row's
atomic commit. Unselected rows remain selectable; successful rows never recreate.
After a partial response, full batch GET is the cumulative authority. Failure CSV
keeps original columns, source row and error with formula-safe cell escaping.

The recorded local 1,000-row sample was preview 655.7 ms / commit 5.142 s in a
small workspace. It is not P95 or populated-workspace certification; see
`.trellis/tasks/10-04-triage-import/progress.md`.

## Compatibility, migration and rollback

Apply the complete ordered migration sequence: 513 adds tables/admission,
514–534 each build one concurrent index, 535 adds persisted comment eligibility
and the execution fence. No foreign keys or cascade actions; do not group
concurrent-index files in a transaction. Existing issues default to
`not_required`, and rollout remains disabled until a human admin enables it.

Only a settings-endpoint 404 means unsupported on a new client. Permission and
network errors remain errors. Existing HTTP/realtime Issue DTOs carry admission;
client fallback for old servers must never weaken authoritative server checks.
Web/desktop share review UI; old clients/mobile still encounter the same server
fences. There are no triage CLI subcommands in T1.

Disabling hides navigation and blocks fresh intake/reopen; history and existing
formal work remain. It does not erase data or authorize database/code downgrade.
Down migrations refuse retained settings, preview/import/notification data,
action/intake tombstones, nondefault admission or ineligible comments. Merely
having no pending issues is insufficient. Retain data and deploy a forward fix;
never delete audit/identity rows or indexes to bypass refusal. Older servers
without admission checks cannot safely serve a used triage database.

Down guards lock tables with `ACCESS EXCLUSIVE NOWAIT` before checking data and
perform protected DDL in the same block. A busy table is a refusal. Multi-file
rollback requires all API/background writers stopped throughout maintenance;
single-file locks do not span the loop. Rehearse empty/populated/concurrent-writer
cases only in private fixture schemas.

## Regression surfaces

- `internal/handler/triage_test.go`: settings, intake, decisions, replay,
  history, execution intent and notification behavior.
- `internal/handler/triage_boundary_test.go` and
  `internal/service/triage_admission_test.go`: alternate mutation/dispatch paths,
  stale issue inputs, delayed comments and formal-work controls.
- `internal/handler/triage_import_test.go` and `internal/triagecsv/parse_test.go`:
  limits, mapping, ownership, partial retry, concurrent dedup and safe export.
- `internal/service/triage_project_resource_fence_test.go`: resource context
  cannot change during an authorized enqueue transaction.
- `internal/migrations/triage_rollback_test.go`: actual migration down SQL.
- `packages/core/api/triage-client.test.ts`, `packages/core/triage/`,
  `packages/views/triage/` and `e2e/triage.spec.ts`: API parsing, cache isolation,
  client actions and platform flows. Completion evidence belongs in the task's
  `verification.md`, not a passing-test claim in this specification.

## CI precision and error-classification regressions

- A snooze notification compares its serialized deadline with the persisted
  `issue_triage.snoozed_until`. Production must serialize the value read back
  from PostgreSQL. Fixtures that write both a pgx timestamp and JSON text must
  truncate the shared instant to microseconds first: pgx truncates nanoseconds
  while PostgreSQL text casts round them. macOS clock precision can hide the
  Linux failure; retain exact delivery and recipient assertions.
- Admission checks can fail because work is blocked, the issue was deleted, or
  the database failed. Map only `admission.Blocked` to the review-required
  conflict; preserve wrapped errors for the caller's existing missing-resource
  and server-error handling. Lifecycle source and target preflights both obey
  this rule.
- Claim-based handler fixtures must provide a current runtime heartbeat at
  claim time. A TestMain heartbeat can expire during a full race suite; do not
  weaken the production freshness window to keep such fixtures alive.
