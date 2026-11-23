# AC33 command-history empty-reason verification — 2026-10-21

The existing service repair now has actual database-backed race evidence. A
successful managed command with a valid empty persisted reason projects JSON
`null`; manual and automatic cancellation reasons retain their exact values.
This report does not claim the combined Web/Electron AC33 journey passed and
does not change the acceptance ledger.

## Behavior and change boundary

`collaborationCommandHistoryEntry` uses the already-present
`collaborationCommandHistoryReason` helper. It returns nil for invalid or empty
`pgtype.Text` and returns the original string for every nonempty value. The
generic `util.TextToPtr`, strict Core response schema, and client history
assertions remain unchanged. No trimming or reason rewriting was added.

The existing DB-backed
`TestCollaborationCommandHistoryProjectsEmptyTerminalReasonAsNull` submits a
managed command, resumes, claims, permits, records the business success, accepts
the original termination proof, reads history and checks both the nil reason
and JSON `"reason":null`. A separate cancellation fixture checks the exact
`cancelled_by_participant` reason.

This lane changed only that test's environment handoff: before constructing the
legacy cancellation fixture, `t.Setenv("MULTICA_AUTH_MODE", "legacy")` restores
the mode its JWT principal requires. `t.Setenv` remains test-scoped. The
managed success and strict wire assertions are preserved. The product helper
was already present and was not rewritten by this lane.

## Retained failures

All three failed attempts remain intact:

| Artifact | Actual outcome | Classification |
| --- | --- | --- |
| `history-empty-reason-red.jsonl` | Success history has non-nil empty `Reason`; test fails at line 305, package 1.427s | Genuine product serialization defect before the repair |
| `history-empty-reason-green.jsonl` | Test fails later at line 314 with `forbidden`, package 1.681s | Fixture authentication failure; the filename is not a passing result |
| `history-projection-empty-reason-race-20261021.jsonl` | Current success/null assertions pass, but the later legacy fixture fails with `forbidden`, package 1.730s, exit 1 | DB-backed reproduction of the fixture authentication failure |

The managed fixture leaves `MULTICA_AUTH_MODE=password` for the remainder of the
test. Constructing another `newCollaborationQueueFixture` uses a legacy JWT
principal with no password session, so its creation fails before cancellation.
Restoring the local auth mode fixes this setup mismatch without changing service
authorization or weakening the regression.

Preserved original hashes:

- `history-empty-reason-red.jsonl`:
  `771631428cc70751ea00049d0ebe36ee17882286924cbaff5c00f07fdb23daab`
- `history-empty-reason-green.jsonl`:
  `d1da0a340189431b233da14bd95a82ee220308fa2976ae247e06c0259e4511a1`

The earlier no-`DATABASE_URL` command in `ac33-current-gap-2026-10-21.md` is not
used here as DB evidence: `newCollaborationFixture` skips when that variable is
absent. Current logs below explicitly contain no skips.

## Environment and provenance

All test commands below ran from
`/Volumes/artisan/code/2026/multica-0.4.37/server`, with
`GOTOOLCHAIN=go1.26.6`, on darwin/arm64. The task-owned launcher injects only the
isolated `service_s5` database; private connection credentials stay outside the
repository. The agent CLI guard was active, and no real provider account or
quota was accessed.

The read-only query retained in `history-projection-database-20261021.json`
confirms PostgreSQL 17.11, database `multica_gap_service_s5`, final migration
`737_collaboration_runtime_preparation`, and the actual
`agent_runtime.collaboration_capabilities` column. This avoids the obsolete
schema-729 `service_s1` boundary.

Verified source SHA-256:

| File | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_command_history.go` | `a23ad56dae751bc9cf7f0ef226292f3bea7ff3661cc7ebc6310bde49ef79e526` |
| `server/internal/service/collaboration_command_history_test.go` | `869196731ce555206557c1464a4495a1aad6bab3aed22994ba56fcedb16df976` |

The adjacent race result also records before/after hashes of its two additional
test files. All four source hashes were unchanged during that run.

## Commands and results

Focused regression after the fixture repair:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^TestCollaborationCommandHistoryProjectsEmptyTerminalReasonAsNull$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Result: exit 0, one top-level pass, zero failures, zero skips; package 2.985s.
Raw output and exact command/timestamps/hashes are in
`history-projection-empty-reason-race-fixed-20261021.jsonl`, its `.stderr`,
`-command.json` and `-result.json` companions.

Adjacent command-history and cancellation race verification:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^(TestCollaborationCommandHistory|TestCollaborationCommandCancellation|TestCollaborationFIFOCancellationAndPromotionSerialize)' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Result: exit 0, 13 top-level passes (39 total pass events including subtests),
zero failures, zero skips; package 16.367s. The standalone regression appears
again in this set and is not counted as a distinct additional identity.
Artifacts: `history-projection-history-cancellation-race-20261021.jsonl`,
`.stderr`, `-command.json` and `-result.json`.

The verified top-level identities are:

- `TestCollaborationCommandCancellationBackgroundAttribution`
- `TestCollaborationCommandHistoryM11CancelClaimRace`
- `TestCollaborationCommandHistoryAutomaticM11Detach`
- `TestCollaborationCommandHistorySwitchThenCancelPreservesHistoricalPlacement`
- `TestCollaborationCommandCancellationExactProvenance`
- `TestCollaborationCommandHistoryOutlivesCommentAndAttachmentDisplay`
- `TestCollaborationCommandHistoryPagingAndCurrentAuthority`
- `TestCollaborationCommandHistoryKeepsUnprovenLegacyCancellationUnknown`
- `TestCollaborationCommandHistoryProjectsEmptyTerminalReasonAsNull`
- `TestCollaborationCommandHistoryLegacyManualAndBrokenEvidence`
- `TestCollaborationCommandCancellationRollsBackEventAndState`
- `TestCollaborationCommandCancellationUnknownCommitKeepsOneEvent`
- `TestCollaborationFIFOCancellationAndPromotionSerialize`

These cover exact cancellation attribution, retained input/attachment identity,
current read authority and pagination, unknown legacy attribution, invalid
event correlation, cancellation rollback/original-key recovery, M11
claim/cancel in both deterministic orderings, automatic detach, Switch history,
and FIFO cancellation/promotion isolation.

Static checks:

```bash
# From server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
# From repository root:
gofmt -d server/internal/service/collaboration_command_history.go \
  server/internal/service/collaboration_command_history_test.go
```

Both exit 0 with empty diagnostics/diff. Vet took 2.455s; its command/result and
streams are retained as `history-projection-vet-20261021*`. The gofmt command
and empty streams are recorded with the fixed regression artifacts.

## Remaining boundaries and handoff

- The coordinator must rebuild/restart the owned API with this exact product
  source and rerun the combined production Web/actual Electron AC33 journey.
  Service tests cannot substitute for those client history assertions.
- The service fixture accepts a synthetic termination proof. It does not prove
  native containment, paid provider behavior, or packaging/other-OS support.
- The focused regression directly asserts a natural success with no reason and
  exact manual cancellation reason. Adjacent tests assert automatic/legacy
  cancellation values; no new failure-outcome-specific browser case is claimed.
- The lane did not run a new full `make check`, alter rollout gates, update the
  acceptance matrix/spec/progress, archive either task, or commit shared files.
  The coordinator owns those follow-ups and any Lore slice commit.

Changed files in this lane: the one test environment line in
`server/internal/service/collaboration_command_history_test.go`, this report,
and unique `history-projection-*` evidence artifacts. No production abstraction
or dependency was added; the existing narrow projection repair is preserved.

## Appendix: standalone reason-helper slice for commit isolation

The coordinator requested a tiny extraction so the reason repair can be reviewed
and committed independently of the larger untracked command-history reader.
`collaborationCommandHistoryReason` moved byte-for-byte, with the same name and
body, from the end of `collaboration_command_history.go` into new
`server/internal/service/collaboration_command_history_reason.go`. Its original
call site and the rest of that history file remain unchanged. The full DB-backed
`collaboration_command_history_test.go`, including the verified auth-fixture
repair, is unchanged. AC63's handler test also remains unchanged.

New `collaboration_command_history_reason_test.go` contains two independent
test identities:

- `TestCollaborationCommandHistoryReasonPreservesNullableValues`: SQL null,
  invalid text with a stale value, valid empty text, exact nonempty cancellation
  and failure reasons, and preserved whitespace with no trimming.
- `TestCollaborationCommandHistoryReasonFromSucceededManagedCommand`: the
  existing tracked managed fixture submits/resumes/claims/permits a command,
  records business success and accepts its original proof. An actual
  `SELECT state,reason FROM collaboration_command WHERE id=$1 AND session_id=$2`
  must return `succeeded` and valid empty `pgtype.Text`; projecting that row into
  a local struct must serialize exactly `{"reason":null}`.

The new pair references no untracked `CommandHistory` DTO, query or cancellation
type. Its managed fixture, testutil and protocol dependencies already exist in
tracked source. The coordinator separately verifies an isolated HEAD snapshot
containing only this pair before any commit; that independent run is not claimed
by the implementer.

### Failure-first evidence without worktree reversion

A task-owned temporary compiler overlay substitutes only the helper with:

```go
func collaborationCommandHistoryReason(reason pgtype.Text) *string {
    return util.TextToPtr(reason)
}
```

The replacement source, overlay mapping/path/hash and exact command are retained
in `history-projection-standalone-overlay-red-20261021-command.json`. The actual
overlay is below `/tmp/multica-gap-recovery-20261021-h6phxwo9/`; no shared source
was reverted, overwritten or temporarily exposed to another agent.

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -overlay /tmp/multica-gap-recovery-20261021-h6phxwo9/history-projection-legacy-pgkv3av0/overlay.json \
  -race ./internal/service \
  -run '^TestCollaborationCommandHistoryReason(PreservesNullableValues|FromSucceededManagedCommand)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

The command artifact is authoritative for the allocated overlay path. Result:
exit 1, zero skips, package 1.619s. Exactly `valid_empty` fails with `got ""`
and the natural managed-success test fails with `{"reason":""}`. SQL null,
stale invalid text, cancellation, failure and whitespace cases pass. Raw red
output is `history-projection-standalone-overlay-red-20261021.jsonl`; this is an
intentional regression reproduction, not a current product failure.

### Current extracted source verification

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^(TestCollaborationCommandHistory|TestCollaborationCommandCancellation|TestCollaborationFIFOCancellationAndPromotionSerialize)' \
  -p 1 -parallel 1 -count=1 -timeout=30m
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

The integration command passes 15 top-level identities (47 pass events including
subtests), zero failures and zero skips, exit 0, package 16.798s. This is the two
new standalone identities plus the 13 prior history/cancellation/FIFO identities.
Raw output and command/result/hashes are retained as
`history-projection-standalone-integration-green-20261021*`. All recorded source
hashes were unchanged across both red and green runs.

Vet exits 0 with empty output in 2.387s, recorded as
`history-projection-standalone-vet-20261021*`. The three edited/new production and
test files are gofmt-clean; the extraction scope artifact records the exact
gofmt command and empty output.

| Current file | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_command_history_reason.go` | `b015d70086ff0646cd3ca16ad4012e2d48b2dea15d9138a78754c46c8e0f5a1b` |
| `server/internal/service/collaboration_command_history_reason_test.go` | `3152071d3350f18099e924cd978067763b98dfbaa3e07c17ceb70523f17935ec` |
| `server/internal/service/collaboration_command_history.go` | `6ddc3e26ef0d06a449140e6638731cb9693660b38b1d78ccd83d3c0c860ff2be` |
| Unchanged complete `collaboration_command_history_test.go` | `869196731ce555206557c1464a4495a1aad6bab3aed22994ba56fcedb16df976` |

Earlier source hashes and runs above remain historical evidence from before
the extraction. `history-projection-extraction-baseline-20261021.json` and the
final scope artifact prove the helper moved unchanged and reconstruct the exact
previous history file. This extraction adds no behavior, dependency or schema
change. No browser, environment service, gate, provider account, matrix, spec,
archive or commit operation was performed by this lane.

## Coordinator: isolated commit proof

The coordinator archived exact HEAD `7c77bf157c3e78786abef97034c6c60108211e7a`
outside the shared workspace and copied only the two new reason files. No broad
uncommitted S3b source, generated SQL or API/UI implementation was included.
`history-reason-isolated-source.json` records both hashes and archive identity.

```bash
# cwd: /tmp/multica-gap-recovery-20261021-h6phxwo9/history-reason-head-_fyeuvun/server
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service -run '^TestCollaborationCommandHistoryReason' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Actual result: exit 0, both top-level identities and six nullable/exact-value
subtests pass, zero failures and zero skips; package 3.106s. Exact commands,
timing and outcomes are in `history-reason-isolated-head-race-result.json` and
the neighboring JSONL/stderr. This establishes the pair's independent compile
and real DB regression. The shared-tree integration suite and natural Linux
Web/Electron journey remain separately attributed evidence; the isolated pair
alone does not deliver the broader uncommitted command-history endpoint.
