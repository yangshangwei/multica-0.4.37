# R51 children and R52 recorded runs: server implementation

2026-10-19. Server implementer ownership only; no commits or task-state changes.
The parent task remains in progress and its creation/execution rollout gates
remain disabled. No general provider adapters, file/Git preparation, new OS
containment, P7 administration or whole-parent acceptance are claimed.

## Implemented behavior

- Shared issue detail now includes the complete formal, same-source **ordinary**
  child reference set, ordered by issue number. The existing authorized detail
  snapshot retains `SourceMember` independently of manager capabilities.
  Source members receive the current source slug/open affordance; external
  participants receive only id/identifier/title and `can_open:false`.
- Ordinary child create, parent change, rename, single delete and batch delete
  keep their original owners. After commit they publish one content-free
  `collaboration:children_changed` hint per affected shared parent. The relay
  targets only the issue scope; normal final-send authorization still applies.
  There is no collaboration revision, participant, handler, queue or notification
  mutation from this hint. Polling/focus/reconnect remains loss recovery.
- The supported static runtime accepts optional typed `record` data alongside
  its existing `{outcome,body}` output. Shared protocol validation bounds count,
  UTF-8 content, aggregate encoded size, tool/message fields and exact nullable
  usage. Duplicate/unknown fields remain refused at runtime HTTP/JSON boundaries.
  A legal result can exceed the old 128 KiB stdout cap without being discarded.
- Accepted recording remains in the immutable source-owned outcome event;
  there is no second task-message or task-usage owner. Replay compares complete
  typed values. A prior squad activity may receive one matching outcome/record,
  but a conflicting business outcome or a second recording cannot overwrite it.
  Partial recording survives proof settlement. Failed/malformed/revoked recording
  never widens proof credentials or prevents a legal sealed proof.
- Claim freezes an allowlisted `resource_snapshot` source event. Resource IDs
  come from the retained run; provider/model are configured provenance, with no
  credentials, host paths, installation details or reconstructed billing account.
  Missing older provenance stays null. Each run reads only its own immutable
  `prompt_snapshot`; current batches and later M11 inputs cannot rewrite it.
- `GET /api/collaboration/issues/{id}/runs/{runId}/records` is an independent
  protocol-1, no-store, current-issue-authorized read. It pages immutable inputs
  and typed event entries with a run/prompt-bound cursor and fixed upper event
  sequence. A page targets 256 KiB, but advances one larger legal historic result
  intact. Missing authorized runs use `run_not_found`; missing/denied issues use
  `not_found`. Cursors require all seven exact non-null fields, and cannot inherit
  missing bindings from a preinitialized current snapshot.
- One newly proven terminal **group** transition creates one durable
  `ai_completed`, `ai_failed` or `ai_stopped` event. Existing proof/delegation
  barriers and forced disposition determine that transition. Intermediate
  leader/member results do not notify group completion. The representative
  final leader/direct run supplies the link, regardless of last proof arrival.
- Result notifications freeze current eligible participant recipients in that
  transaction, including the human input author/resource owner. AI/system results
  have no proxy human actor. Existing human action self-exclusion is unchanged.
  The wire preserves `source_kind:"issue", source_id:null` for older clients,
  adding `run_id` only on currently open result notifications. Access loss strips
  title and run anchor. Retries do not expand recipients or create source inboxes.

The exact coordinated wire contract is
[r51-r52-api-contract-2026-10-19.md](r51-r52-api-contract-2026-10-19.md).
Existing v5 execution SQL lists all authorized groups/runs without a LIMIT, so
record discovery needs no separate capped history owner.

## Files and simplifications

Production work is confined to `server/`:

- Protocol and the supported daemon outcome path: `pkg/protocol/collaboration*`,
  `internal/daemon/collaborationrun/{runner,client,controller}.go`.
- Dedicated record/child/result-notification services, their existing claim,
  settlement and notification owners, and the shared v5 run DTO projection.
- Dedicated records handler/router, additive child detail projection, ordinary
  postcommit write hooks and the issue-scope event listener.
- `pkg/db/queries/collaboration_records.sql`, child reference SQL, proof partial
  preservation and sqlc output. Existing source events and indexes are reused.
- Migration 729 adds only nullable `collaboration_notification.run_id`. It adds
  no FK/index. Its down migration fences concurrent writers and refuses retained
  result links, while preserving old notifications during empty maintenance.
- Colocated protocol/service/handler/router/migration regressions, Linux fake
  provider recording modes, and affected built-in skill/source-map guidance.

The complete 48-file server inventory is in
`../../../../.omx/reports/cwc-r51-r52-20261019T110745Z/server/changed-files.json`.
All server source, tests, documentation and generated files were write-frozen
at 11:59 UTC for the parent's production build. Further report output stays
outside the product source tree.

## Verification evidence

Evidence root: `../../../../.omx/reports/cwc-r51-r52-20261019T110745Z/`.
All database commands use its private `environment/run.py` with
`CWC_DATABASE_ROLE=server`; the shared bootstrap was never reused.

| Evidence | Result |
| --- | --- |
| `server/01-red-wire-children.log` | Expected red: typed recording discarded and ordinary children missing |
| `server/04-red-results.log` | Expected red: no result notification for success/failure/missing/stop/nonlaunch |
| `server/07-red-strict-records.log` | Expected red: missing message fields accepted as recorded empty text |
| `server/12-red-cursor.log` | Expected red: empty/missing-bound cursor inherited current defaults |
| `server/03-migrate.log` | Migration 729 applied to isolated server DB |
| `server/13-final-sqlc.meta.json`, `server/sqlc-drift.json` | sqlc exit 0; all generated bytes unchanged on second pass |
| `server/14-final-cursor-migration.meta.json` | Final targeted race suite exit 0 across service, handler, real router and migration packages |
| `server/15-vet.meta.json` | Explicit vet exit 0 across protocol, daemon runtime, service, handler, router and migrations |
| `linux/recording.meta.json` | Parent-owned actual Linux arm64 run exit 0, all 10 named runner cases passed, zero skips |
| `server/16-collaboration-regression.meta.json` | Broader race regression exit 0 in 498.9 seconds; protocol/service/handler/router/migration selections all passed |

Final targeted race command:

```bash
go -C server test -race -p 1 -parallel 2 \
  ./internal/service ./internal/handler ./cmd/server ./internal/migrations \
  -run 'TestCollaborationRunRecords|TestCollaborationOrdinaryChild|TestCollaborationRuntimeRecordedContentHTTP|TestCollaborationRuntimeRecordingHTTP|TestCollaborationResultNotification|TestCollaborationSquadRecording' \
  -count=1
```

That suite covers 205-child completeness; source/external projection; actual
ordinary create/attach/move/rename/detach/delete/batch-delete hints; unchanged
parent state; invalid-source refusal; recording redaction, strict JSON and limits;
70-entry paging with a fixed upper boundary; exact large/zero/missing usage;
engine/runtime cleanup retention; current participant/source/machine authority;
missing run versus denied issue; no invented unclaimed prompt/provenance;
squad activity/record coexistence; per-run M11 readership; final-leader anchors;
all terminal outcomes; ineligible recipients; notification-write rollback;
concurrent replay and frozen recipients; real HTTP ingestion/read/proof/notification
and revoked-link redaction; additive migration and concurrent-first-writer guards.

The Linux test is `TestLinuxRunnerHTTPOutcomeStopAndRevokedBusinessProof`.
The parent built the test binary and ran it in the labeled isolated arm64
container with required namespace support, `--privileged --network none`.
Test-created executables emit recorded/partial/invalid/oversized output; all
retain the separate proof path. This proves the supported Linux path, not native
macOS/Windows runtime containment or a real paid provider. No installed AI CLI,
account or quota was used.

Broader read-only regression command:

```bash
go -C server test -race -p 1 -parallel 2 \
  ./pkg/protocol ./internal/daemon/collaborationrun ./internal/service \
  ./internal/handler ./cmd/server ./internal/migrations \
  -run 'Collaboration|LegacyExecution|SourceContextOrdinaryChild|TestMigration|TestNewMigration' \
  -count=1
```

The broad service selection completed in 436.9 seconds, handler in 17.6 seconds,
router in 16.0 seconds and migrations in 12.4 seconds. The host daemon package
selection had no matching Linux tests; the separate required-namespace Linux
run above is its recording evidence, and host absence is not counted as a pass.

`git diff --check -- server` and the final changed-file `gofmt -l` check are clean
(`server/format-check.json`). The targeted harness also caught and
corrected fixture-only problems: raw runtime deletion needed the ordinary agent
unbind, fixture issue numbers needed allocator alignment before public create,
and reused Go decode storage retained an omitted pointer from the previous
notification response. Fresh HTTP response decoding confirms the actual denied
wire omits the anchor correctly.

## Remaining parent work

The parent owns integrated production Web/native Electron journeys, full
repository checks, independent review, task/spec records and commits. Existing
unrecorded historic process/usage/context cannot be reconstructed. The shared
recording represents explicit saved telemetry and frozen claim inputs; a
configured provider label is not a verified billing identity. Transient child
hints rely on the already-existing polling/reconnect recovery when missed.
The complete P7/P8 capability matrix and all 97 criteria remain outside this
increment's completion claim.

## Bounded full-gate follow-up

The parent's full Go run found four direct single-runtime reads and a 504-line
built-in skill body. Three runtime call sites predate this increment, but the
parent explicitly authorized their same-domain repair. The separate config
test failure came from inherited `ANALYTICS_DISABLED=true`; its environment
correction remains parent-owned, with no config/test changes here.

Fix boundary before editing: replace only the reads in `freezeResourceRecording`,
`ResourceOptions`, `collaborationResourceSummary` and
`currentCollaborationResourceIdentity` with the existing
`CollaborationService.collaborationRuntimeLookup(q)` contract. Production metrics
are wired through `router.go` into `IssueService.Metrics`; a nil-recorder wrapper
would bypass that instrumentation despite satisfying the AST check. The existing
claim/resource helpers will therefore use service receivers and the same
transaction-bound `q`/`r.q`, including the Switch summary call site. No cache,
new source label, new abstraction, authorization change or pool read is added.
Existing runtime/resource/Switch/authority regressions plus a collector/snapshot
regression verify the change. The AST allowlist remains untouched.

Condense the root skill's collaboration pointer and retain the detailed child,
recording and group-result rules in its existing one-level reference. Preserve
the 500-line limit and source map. Only these bounded fixes reopen the server
write freeze; runner/protocol and the recorded Linux binary stay unchanged.

Follow-up verification passed with no test/flag/allowlist relaxation:

- `server/17-full-gate-fixes.meta.json`: exit 0, 53.6 seconds. The original
  metrics AST gate and complete built-in conformance test pass. Existing
  RuntimeLookup, recorded-run, resource/registration/authority and atomic Switch
  regressions pass with race across metrics, service and handler packages.
- `TestCollaborationRuntimeLookupsUseProductionCollectorAndTransaction` observes
  two real issue-source metric increments for resource options plus summary,
  one for direct resource summary and one for current owner identity. Recording
  independently increments the same configured collector once and persists the
  provider value visible only inside its caller's uncommitted transaction.
  A nil recorder or an escaped pool read fails this test.
- `server/18-full-gate-vet.meta.json`: exit 0, 5.2 seconds. Changed-file gofmt and
  `git diff --check -- server` are clean. No SQL regeneration was needed.

The follow-up changes six existing source/skill files and adds the focused
collaboration runtime-lookup test; the inventory now also includes the three
authorized baseline resource-view files. All server production, tests, built-in
documentation and generated files are write-frozen again at 12:40 UTC. The
parent owns the corrected-analytics full-package rerun and remaining agent test
phase; these focused results do not claim that rerun has already passed.
