# R51/R52 independent server integration review

Reviewed the increment against `a157fd140`. This assignment began with server
and shared UI/E2E review; the main session subsequently assigned the shared
UI/app/E2E lane to `/root/check_views`. The completed coverage below is server,
protocol, supported runtime wiring, source events, storage/notifications and
server-to-core wire compatibility. Reused the separate
[completed core review](check-r51-r52-core-2026-10-19.md).

Product and test files remained read-only in this reviewer session because the
main session was preparing production builds. The server owner implemented the
reported correction after coordination. This reviewer wrote only this report
and isolated command evidence. No agents, commits, dependencies or paid/real
provider calls were used.

## Context and scope

Loaded the complete saved task hook output, including PRD/design/implementation,
current continuation, API contract and acceptance context. Read the full server
security boundaries and collaboration foundation, targets, squads, runtime CLI,
Switch/skills, content, realtime and source/editing contracts. Reused the root
CLAUDE/AGENTS and core contracts loaded for the earlier review. Also read shared
component/quality guidance before the UI lane was reassigned.

The accepted increment does not enable rollout, add general provider or new OS
support, perform runtime Git/file preparation, implement P7 administration or
complete the parent task's 97 criteria.

## Findings (fixed by the implementation owner)

### P2: A supplied cursor inherited omitted binding fields

- File: `server/internal/service/collaboration_records.go:201`.
- Issue: `RunRecords` initialized the cursor with trusted issue/run IDs, prompt
  hash and current event boundary, then decoded the caller's JSON into that
  populated value. The base64url cursor `e30` (`{}`), or a cursor missing binding
  or position fields, silently inherited valid defaults instead of returning
  `invalid_records_query`. The human issue ACL still ran; this was malformed
  paging acceptance, not a bypass of object authorization.
- Evidence: reported the exact code path to the main/server agents before any
  product edit. The server owner's regression failed with
  `invalid paging was accepted: <nil>` in
  `server/12-red-cursor.log` under the evidence root.
- Fix: the owner decodes a nonempty cursor into a fresh structure, requires all
  seven exact non-null fields, then compares source/run/prompt identity and
  validates sequence/offset bounds. Added service and actual HTTP negatives for
  `{}` and omitted identity/position fields.
- Verification: independently reran
  `TestCollaborationRunRecordsCurrentAuthorityAndLegacyAbsence` and
  `TestCollaborationRuntimeRecordedContentHTTP` with the race detector against
  the isolated check database after the fix. Both passed.

The earlier core review also identified the server's incomplete-recording status
priority: a persisted incomplete flag without typed telemetry incorrectly
reported `not_recorded`. The server owner corrected it. This review's terminal
notification matrix verifies the proof-without-outcome read now reports
`partial`, retains `recording_incomplete` and leaves usage null.

## Reviewed behavior and evidence

### Ordinary children and scoped refresh

`ListCollaborationOrdinaryChildren` selects only same-source formal ordinary
children, excluding independently shared children, with no limit and source
number ordering. The authorized repeatable-read detail snapshot supplies the
explicit internal `SourceMember` decision. External references expose only
id/identifier/title/can_open and no slug; ordinary source members can open them
without being collaboration managers or participants.

The ordinary create, atomic update and single/batch delete owners publish after
commit. Updates retain the locked old/new parent values; batch updates reuse
that atomic owner. Deletion rebuilds selected issues from locked current rows
before collecting parents. `PublishCollaborationChildrenChanged` deduplicates
parents without changing collaboration versions, participants, history or queue.
The listener accepts exactly the issue hint, forwards through the issue scope
and excludes it from generic workspace/user broadcasts. Existing final-send
scope authorization remains applicable. This is a transient refresh hint with
poll/focus/reconnect recovery, not a new durable child-event contract.

The old execution/source-copy owners check the actual source issue rather than
its parent. Independently reran the canonical ordinary-child SQL execution
matrix and ordinary-child source-copy positive control, alongside shared-source
capture and machine/table negatives. No inherited collaboration grant, queue or
parent content path was introduced.

### Typed recording, storage and protected reads

The runtime client, runner, handler and service share the typed outcome
validation. Duplicate/unknown JSON is rejected at the existing strict runtime
decoders; recorded messages additionally require the exact discriminant fields.
The message count, UTF-8 body/name lengths, outcome/envelope budgets and canonical
nonnegative signed-int64/decimal telemetry remain bounded. Null counters remain
missing observations, and legacy outcome/body records remain readable.

Claim writes the prompt and configured-resource projection in the same source
transaction. An existing prompt without provenance does not consult today's
resource configuration to reconstruct history. Only allowlisted identity and
provider/model/account-source fields leave the record projection. Per-run prompt
readership remains separate from mutable current batch placement and M11 joins.

The read route is in the Auth-only collaboration group, clears ambient workspace
headers and returns no-store. `readCollaborationContent` authorizes the current
human issue in its repeatable-read transaction before run lookup. Run/session/
group identity is checked; wrong/missing runs use `run_not_found`, while missing
issue authority uses `not_found`. Every cursor page repeats this authorization.

Queries use retained source run/session/group/events, without engine task,
message or live-resource joins. Tests remove the engine task/runtime and change
resource configuration, then verify old inputs, recorded usage and original
provider survive without private installation/credential fields. Inputs precede
event items; the cursor preserves input, event and within-event positions and a
frozen upper event bound. Later events enter a new read, not the old page chain.
Page count and encoded-byte limits advance a larger legal historic entry intact.

Matching stdout recording after a squad activity attaches one immutable outcome
record without replacing its business result. Conflicting or duplicate replacement
outcomes are refused. Partial coverage is preserved through proof ingestion.

### Group result notifications and retention

`recordCollaborationGroupResult` is called only after
`SettleCollaborationProvenGroup` returns a newly settled row. The existing shared
ledger validator, all-run sealed proof coverage, closed expansion and settled
delegation barrier precede this transition. Result alone or Stop intent alone
does not notify completion. The final allocated leader turn determines the
representative run; member/interim leader results do not become group results.
Forced disposition remains authoritative.

The event and recipient rows commit with settlement. AI/system results use no
human actor, so neither the input author nor resource owner is incorrectly
excluded. The participant query excludes revoked/disabled accounts; departure
revokes participation before stopping/settling affected work. Source-only members
are not added as recipients. Original proof/settlement replay neither creates a
second event nor expands recipients to a later joiner. Required notification
write failure rolls back proof/settlement; subsequent concurrent replay produces
one original recipient set.

The additive `run_id` keeps `source_kind: "issue"`, `source_id: null` for older
clients. Current denied projections omit the title and run anchor and set
`can_open: false`. Ordinary inbox writes are absent. Migration 729 adds only a
nullable column, without an index or foreign key. Its rollback takes the table
NOWAIT fence before checking retained links; the private-schema rehearsal proves
old rows survive, retained/new uncommitted links refuse rollback, and the empty
maintenance rollback is safe.

The updated built-in issue skill, workflow reference and source map accurately
describe ordinary-child isolation, frozen recorded inputs, missing telemetry,
runtime limits and group-level result semantics. Their changes do not claim a
general provider or completed-squad capability from one member result.

## Findings (not fixed)

No unresolved server defect remains from this review. All reported product
corrections were coordinated with and applied by the server owner.

Permanent `.trellis/spec/` synchronization and final acceptance bookkeeping
remain main-owned. Shared UI/app/E2E review is owned by `/root/check_views`;
production Web/Electron execution and broad repository checks remain with the
main session. This report does not substitute the server tests for actual
browser/native journeys or kernel containment evidence.

## Verification

Evidence root: `.omx/reports/cwc-r51-r52-20261019T110745Z/`.
Reviewer commands and raw output are under `check-integration/`, each with a
command/exit `.meta.json` and `.log`.

Every database command used `CWC_DATABASE_ROLE=check` through the task wrapper.
Prerequisite migration used `go -C server run ./cmd/migrate up` directly, never
the legacy shared-database bootstrap. Migration rehearsals used private schemas
inside that isolated database.

| Evidence | Result |
| --- | --- |
| `migrate` | Direct isolated check-database upgrade passed |
| `server-scoped` | Race tests passed for records/notifications/squad, child projection/writes, HTTP/hints, strict protocol and existing shared-source/machine boundaries |
| `cursor-final` | Post-fix service and actual HTTP cursor regressions passed with race detector |
| `child-isolation-and-rollback` | Ordinary-child SQL execution matrix, quick-create/source-reference boundary, own-source capture and migration rollback rehearsal passed with race detector |
| `gofmt` | 36 changed Go files checked; no formatting failures |
| `vet` | Five affected packages, including the runtime client/controller/runner package, passed `go vet` |
| `git diff --check -- server` | Passed |

Across the three test runs: 21 distinct top-level tests; 70 passing test/subtest
results including the targeted repeats; zero failures and zero skips. Test
commands use `-race -p 2 -parallel 2 -count=1 -v` and the repository's agent CLI
guard. Go compilation/type checking passed as part of those package builds.
The earlier core review's 238-test/typecheck/lint evidence remains separate and
was not rerun here.
