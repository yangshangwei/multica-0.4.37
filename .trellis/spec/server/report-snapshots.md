# Deterministic report snapshots

## 1. Scope / trigger

Use this contract for report capture, public replay, Reporter reads, withdrawal,
source collection and changes to project-health calculation reused by reporting.
The minimum release reports current ordinary formal stock; business classification
and historical window metrics remain unknown without authoritative evidence.
It does not create a complete event ledger, call providers, publish comments,
change project policy or automatically generate periodic reports.

## 2. Signatures

| Surface | Contract |
| --- | --- |
| POST `/api/issues/{reportUUID}/report-snapshots` | Current human ordinary-report editor; original Idempotency-Key; first201/original replay200 |
| GET `…/report-snapshots/current` | Read the sealed control pointer; never collect or refresh |
| GET `…/report-snapshots/{snapshotUUID}` | Read immutable identity under current report/project authority |
| POST `…/report-snapshots/withdrawals` | Human explicit terminal withdrawal of all this report's snapshot content; first201/original replay200 |
| CLI `issue report-snapshot capture/show/withdraw` | Explicit original intent; withdraw also requires `--accept`; Reporter uses show only |
| `reportsnapshot.Build/Validate/Decode` | Internal frozen material and original internal digest contract |
| `ProjectPublic/ReplayPublic/ValidatePublic/DecodePublic` | Safe public facts, independent computation and strict public validation |
| `projecthealth.ComputeSignals` | Identity-free health rules; existing Compute/Collect wrappers retain current-health behavior |
| `collectReportSnapshotFacts` | Borrow caller tx/q/cut; return Facts and private reportSnapshotAudit |
| DB | `report_snapshot` immutable sealed rows; `report_snapshot_control` mutable report-level serialization/withdrawal fence |

Migrations794–804 use no new FK/cascade. Every unique index is CONCURRENTLY in
its own single-statement migration. Root issue/workspace deletion explicitly
cleans report-owned controls and snapshots in the existing deletion transaction.

## 3. Contracts

### Input, storage and public output

Capture accepts exactly protocol_version=1, expected_issue_revision, timezone,
start/end, period_mode and explicit nullable correction_of. IDs are canonical
nonzero UUIDs; time boundaries are UTC microseconds with `[start,end)` and
`start < end <= as_of`. The caller cannot submit facts, actor/project identity,
source completeness or statistics. IANA zone, resolved offsets/local calendar
dates and rule versions are frozen; replay does not consult today's tzdb or clock.

The storage envelope is version2 with internal, public, audit and audit_digest.
The row's seal_digest binds storage version/identity, internal SnapshotDigest,
public ProjectionDigest and audit_digest. Validate the two objects, private audit
and this pairing. Two independently valid objects need not belong together.
Old internal-only or unbound draft rows are unavailable; GET never backfills or
regenerates their public projection.

PublicSnapshot contains only safe scope/time/identity, per-issue and per-project
facts, source capabilities/gaps, metrics and public digests. It omits creator,
request key, internal sequence/read cut, origin chain, assignee/lead/reference/
runtime/roster identities, native activity IDs and private transport audit.
Joined assignment/environment/lead validity is nullable evidence, not a default
false. Fixed project_reasons are independently computed and digest-bound: null
for unknown/not-applicable health, [] for known no reasons, fixed codes otherwise.

The success envelope is protocol_version, snapshot, replayed and optional
human-only capture_receipt. GET never returns that receipt. It binds request_key,
report_issue_id, snapshot_id and request_digest, where the digest commits canonical
typed `{report_issue_id, request_key, request}`. The private actor/workspace-scoped
idempotency digest is separate. CLI must verify this receipt after removing the
old private Snapshot.RequestKey dependency; it cannot simply drop intent binding.

### Source authority and coverage

The cohort is actual current workspace/project ordinary issues with
admission_status in not_required/accepted. Independent collaboration is excluded.
Formal stock includes reports and unclassified issues; business stock excludes
positively proven reports and retains unknown classifications. Titles and missing
automation metadata are not proof of business purpose. No project means all
project-derived stock/window/health metrics are not_applicable, never workspace0.

Collect project, issues, statuses, references, latest update and activities
independently in one RR cut. Preserve native or explicitly content-based source
identities/revisions, positive report provenance and selected update identity.
An update read failure must not discard independently known project overdue facts.
Recover optional SQL failures only with a valid savepoint/cut; broken connection,
cancel or transaction conflict cannot be sealed as successful partial data.

Private audit retains page boundaries, counts/content identities, EOF/limit/
unavailable termination and cut binding. Its digest may change with page size;
computational FactsDigest/ResultDigest must not. `(created_at,id)` orders retained
rows but proves neither causal order nor global committed history. Activity's
producer history remains unproven; iteration/triage facts cover only their own
subsets. A later row or correction does not change an older seal or its gaps.

Coverage is per metric and unit: E=I+X+U when enumerable; I includes verified zero
contributors, X requires positive exclusion, and a known single-item gap retains
known E/U counts. Only enumeration/explicit scan uncertainty makes the tail and
denominator unknown. Null, zero, false and not_applicable are distinct; lower
bounds are not totals. Created uses birth/issue evidence, not a status-history
dependency. Ratios/net values cannot become exact from partial samples.

Complete summarizes applicable metric dependencies. A report-only cohort can
have exact empty business/window sets while an irrelevant activity source retains
ReadComplete=false, Count=null and a gap. Never relabel that source as complete.

Current project-health remains the current-view authority; manual progress updates
remain the authority for their published text/judgment. F2 is authoritative for
its sealed scope/version/cut. Compare common frozen inputs and both ID set
differences; preserve old open including unknown statuses and done_count including
cancelled. Health reasons do not all change severity; no-update age is null even
when stale_progress falls back to an in-progress anchor. No report/update dual write.

### Current authority and terminal withdrawal

Only verified Reporter capability may use the two new task GETs, bound to exact
current report and frozen project. Legacy/custom/reviewer identities gain no new
access. Recheck token/task/claim/runtime/principals/member/invoke, actual report
ordinary/formal eligibility, project, control and returned snapshot in the same R.
R is the first RR data query, not BEGIN. Fresh valid claims may read older seals;
capture revision/producer claim are provenance, not current-reader CAS values.

Preserve raw identity hints before Auth; compare them with verified identity in R.
Unknown/duplicate query, HEAD, encoded/extra path, noncanonical ID or conflicting
headers cannot widen scope. Set no-store before Auth/guard can exit. The trusted
F2 namespace marker also suppresses wildcard CloudFront-cookie refresh, including
human/legacy requests later refused by the handler.

Withdraw accepts protocol_version=1, scope=report, expected_issue_revision,
expected_current_snapshot_id and bounded nonblank reason with original key.
Current human edit authority and the reviewed current snapshot are rechecked.
Capture/correction/withdraw actually update the shared control row. An insert-only
tombstone plus an unchanged report lock cannot fence a stale RR waiter.

Withdrawal is terminal: all current/old content GETs refuse, capture/new cycles/
correction cannot revive content, and there is no restore. Original human
withdrawal-key replay returns its own minimal receipt without content; capture-key
replay after withdrawal returns no content. A capture/correction that wins first
makes the old withdrawal CAS stale. Retry only a proven retryable transaction
conflict while the original request remains live; cancellation never restarts work.

Ordinary source deletion/move preserves retained report facts. Report/workspace
deletion cleans owned material; project detach/move fails frozen-scope equality.
Move-back cannot revive a withdrawn control. This is report-scoped application
read termination, not source-wide withdrawal or backup erasure. R-before-revoke
requests may finish; already delivered bytes cannot be recalled.

## 4. Validation / error matrix

| Condition | Result |
| --- | --- |
| Missing/invalid initial auth |401, no-store, no snapshot content |
| Current identity/scope/qualification failure |403, no content or hidden IDs/digests/counts |
| Missing/mismatched snapshot or withdrawn content |Minimal404; never fall back to another seal |
| Malformed strict human request |400, no write |
| Stale revision/current, changed original intent, duplicate cycle |409, original state unchanged |
| Capture/correction after terminal withdrawal |409, no content or new seal |
| Unsupported/missing pairing/audit, corrupt content, lost cut/commit |503, no buffered snapshot bytes |
| Known recoverable source gap |Explicit source unknown; only dependent metrics lose exactness |
| Known empty/fully excluded business cohort |Exact0 where provable; ratio undefined; unrelated source gaps retained |
| Capture or withdrawal response loss |Unknown; original explicit request recovery, no automatic new key/retry |
| CLI mat_/mdt_/mcn_ or machine profile writes |Refuse locally without HTTP; server also enforces human-only |

## 5. Good / base / bad cases

Good: a human seals exact formal stock with unknown business history; a current
Reporter replays safe facts, and a later report-wide withdrawal blocks every ID.
Base: existing current-health/manual-update APIs retain their semantics.
Bad: expose the internal object, infer full history from EOF, turn U into0,
recompute old content during GET or let correction undo a withdrawal.

## 6. Tests required

- Public replay/strict decode/private sentinels, calendar/DST, internal digest
  golden, known-item coverage denominator, independent birth, health/reasons parity.
- Private audit validity, actual source IDs, exact EOF and >2,000 rows, late tuple
  commit and same cut, independent update failure, cancel versus live retry.
- Human receipt/intent, original key/cycle/correction, stored-public/audit
  substitution against row seal, source deletion versus owning cleanup.
- Both withdrawal/correction commit orders and original concurrent withdrawal;
  actual Auth/router first-read token/claim/runtime/member/invoke/scope/eligibility/
  withdrawal barriers, next request denial and commit-failure zero bytes.
- All early error no-store and no wildcard cookie; CLI machine-prefix zero writes,
  strict human-only receipts, explicit withdrawal acceptance and no transport retry.

Use newly allocated task databases for handler/router normal and race separately;
record exact named run/pass/skip plus source identities. Pure fixtures do not prove
production history complete, Provider behavior or deployment. Initial failed runs
remain evidence even after corrected new runs pass.

## 7. Wrong vs correct

Wrong: validate internal and public JSON independently, then return public merely
because report/time IDs match. Correct: also verify the immutable row-level
pairing digest and retained audit before releasing the stored public projection.

Wrong: a known issue gap sets E/U.count to null. Correct: preserve known E and
that issue's U membership; only unproven enumeration makes the extent unknown.

Wrong: tombstone insert while locking an unchanged report row. Correct: every
capture/correction/withdraw writer touches the same control row and restarts an
entire still-live transaction after serialization conflict.
