# AC80/AC81 source authority and iteration verification — 2026-10-21

The new source-authority race matrix and external-handler iteration HTTP
journey pass on their first actual guarded database runs. The matrix covers
both commit orderings for structure/iteration versus handler transfer and
source-member departure, plus target-iteration version contention. The HTTP
journey proves source-owned statistics, preview, history and rollover remain
usable when the collaboration handler is external, while that external account
still receives no source catalog. No product code was changed.

## AC80/AC81 authority races

New source: `server/internal/service/collaboration_source_authority_race_test.go`.
It reuses tracked queue/run, source-field iteration and commit/waiter fixtures.
The existing product and helper files are read-only. No copied legacy test suite
or new production abstraction was introduced.

Three canonical identities:

1. `TestCollaborationSourceStructureAndIterationTransferRecheckCurrentHandler`:
   `structure` / `iteration`, each with source-change-first and transfer-first.
2. `TestCollaborationSourceStructureAndIterationHoldMembershipUntilCommit`:
   the same two domains, each with source-change-first and departure-first.
3. `TestCollaborationIterationAssignmentSerializesWithTargetRevision`:
   membership-first and iteration-edit-first.

Each authority scenario has a source-member current handler distinct from the
creator. A retained synthetic running AI group/input exists before the race;
full session/control/target/command/group/run/grant/input rows must remain
unchanged. These are persisted test prerequisites, not natural kernel execution.

`sourceAuthorityCommitOrder` pauses the real leading writer immediately before
commit using `recipientCommitStarter`, then requires `pg_blocking_pids` to show
the following transaction blocked by that exact backend PID. Departure-first
instead holds the actual member row `FOR UPDATE`, starts the source write,
observes the real wait, deletes that member and commits. There is no arbitrary
sleep, fabricated response or generic comment permission proxy.

| Competing writes | Required observed behavior |
| --- | --- |
| Structure/iteration change first, transfer second | Source change commits once; stale transfer refuses. A fresh transfer may then change responsibility; the former handler loses source catalog rights. |
| Transfer first, source change second | The source member remains a participant but is no longer handler/creator. The late source change returns `forbidden`, with no project/parent/title or membership partial write. |
| Source change first, member departure second | The actual membership row remains held until commit, so deletion waits. The chosen project/parent or iteration change commits before departure; subsequent source writes refuse. |
| Member departure first, source change second | The writer waits on the actual source member row, then rechecks after deletion and refuses. No partial source structure, title, iteration membership or collaboration event is applied. |
| Membership first, iteration rename second | Initial join commits under iteration revision 1; public metadata rename later advances the iteration to revision 2. Original membership replay remains an unchanged receipt. |
| Iteration rename first, membership second | Public metadata edit wins revision 2; the queued membership write returns `iteration_revision_conflict` and leaves issue revision/pointer, participation and collaboration membership history untouched. |

Project/parent/title edits are kept separate from iteration assignment. The
tests assert the winning domain exactly and the untouched domain explicitly.
Source member departure does not revoke retained issue participation; the now
external current handler can still receive ordinary issue field options, but
`SourceOptions` is nil. Transfer changes only responsibility/history and never
replays or alters the retained AI run. All resulting scopes/identities and
nonempty running AI records are checked using exact snapshots.

## AC81 external-handler statistics/preview/history

Approved new source:
`server/internal/handler/collaboration_external_iteration_test.go`, identity
`TestCollaborationExternalHandlerIterationStatisticsPreviewAndHistory`.

The journey uses existing I1 HTTP fixtures. It creates two planned source
iterations, a collaboration with an external handler who has no source
membership, and unique private email/username sentinels. A source caller sets
project/parent through the collaboration workflow, then independently joins the
iteration with exact ordinary/collaboration/iteration revisions.

The external handler's collaboration detail has only minimal project/parent/
iteration references; its field-options response has no `SourceOptions`.
Source-owned iteration Get/ListIssues/ListEvents and lifecycle preview deny the
external account with HTTP 403. No source membership is fabricated.

The valid source caller then exercises actual HTTP start preview/apply, active
statistics/current issue history, end preview/apply, frozen statistics/snapshot
and event history, with server-owned rollover into the next source iteration.
Assertions require one complete affected issue, no invalid preview item, current
and original counts, complete destination/counter facts, the external assignee
UUID/type, preserved responsibility, and no collaboration AI command creation.
Every read/preview/apply response is `no-store` and contains no email, username
or private parent body.

The external-handler identity rule follows the existing tenant-scoped iteration
projection: a `member` assignee name is read through a source workspace member
join. With no such membership, the name stays nil and `Available=false` while
the exact assignee ID/type is retained. This does not make the entire preview,
start, statistics, closure or history unavailable, and does not widen the source
member directory. “External handler statistics/preview/history remain usable”
is evidenced for an authorized source caller processing that external assignee;
the external account is still denied source iteration maintenance/catalogs.

The HTTP journey also preserves the ordinary separation between a metadata
project/parent mutation and the dedicated iteration membership request. It does
not claim that an external actor gains generally authorized source iteration
reads or writes, nor that its private username/email may be projected.

## Actual commands and first results

Go 1.26.6, darwin/arm64, task-owned schema-737 `service_s5`. The coordinator
allocated the shared database slot after the prior handoff tests completed.
The two package commands ran sequentially and released the slot immediately
after completion. All agent CLIs were guarded; no real provider/account/browser,
production gate or application environment operation occurred.

From `/Volumes/artisan/code/2026/multica-0.4.37/server`:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^(TestCollaborationSourceStructureAndIterationTransferRecheckCurrentHandler|TestCollaborationSourceStructureAndIterationHoldMembershipUntilCommit|TestCollaborationIterationAssignmentSerializesWithTargetRevision)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/handler \
  -run '^TestCollaborationExternalHandlerIterationStatisticsPreviewAndHistory$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

| First actual run | Result | Artifacts |
| --- | --- | --- |
| New service race matrix | Exit 0; 3 top-level identities / 13 pass events; 0 failures, 0 skips; package 6.033s | `source-authority-service-focused-first-20261021.jsonl`, `.stderr`, `-command.json`, `-result.json` |
| External handler iteration HTTP | Exit 0; 1 top-level identity; 0 failures, 0 skips; package 2.485s | `source-authority-handler-focused-first-20261021.jsonl`, `.stderr`, `-command.json`, `-result.json` |

There was no failed assertion, fixture retry or production fix. All raw outputs
are retained. Each command/result records exact args, UTC start/end, return
code, environment and before/after source SHA-256. All seven recorded source
hashes were unchanged during both runs.

Static checks:

```bash
# From server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service ./internal/handler
# From repository root:
gofmt -d server/internal/service/collaboration_source_authority_race_test.go \
  server/internal/handler/collaboration_external_iteration_test.go
```

Vet passes with empty diagnostics in 3.675s, retained as
`source-authority-iteration-static-first-20261021*`. Gofmt is clean. Baseline
and final-scope artifacts confirm all eight existing product/helper files are
unchanged and the new files have no trailing whitespace.

| New file | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_source_authority_race_test.go` | `992d1afb1ee9d71e48daabffe682a6dc034fe593beac97b101ef37dbe9c0f979` |
| `server/internal/handler/collaboration_external_iteration_test.go` | `fe5b0bfe6ac6d377238e40c3ec1475f84cb17eb9c83983cfb3d830079b1a98f7` |

## Clause assessment and remaining bounds

The new current evidence closes the specific AC80 concurrent transfer/departure
source-structure fencing gap and AC81 external-handler projection plus
transfer/departure/target-version gaps. Existing canonical source tests cover
initial source fields/iteration creation, join/switch/remove reasons, cycles,
completed-join confirmation, original-key refusals and mixed/initial rollback.
The prior current-task service/handler runs cited in `ordinary-control-audit.md`
remain their execution evidence; they were not silently rerun or relabeled here.

The coordinator still owns narrow existing-case adjacency, independent source
review and any combined primary-owner acceptance promotion. This implementer
does not promote either row automatically. Explicit source-only/other-role
negative matrices and mixed request/rollout behavior must be assessed against
the existing canonical tests, not inferred from the new positive counts.

Run/AI fixtures prove retained execution data is unchanged, not kernel process
containment. No new native Web/Electron, packaged client, paid provider,
other-OS, external object storage or full `make check` claim is made. Terminal
and source identities are synthetic isolated test prerequisites.

Only the two new service/handler test files, this report and unique
`source-authority-*` verification artifacts were written. No existing product,
spec, matrix, progress, task state, gate, application environment, commit or
archive was changed by this lane.
