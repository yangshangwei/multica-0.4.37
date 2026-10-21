# AC10 paused-period followup verification — 2026-10-21

Status: the coordinator's actual guarded database race run passes the canonical
test and both cases (one top-level identity, three pass events, zero failures or
skips). The implementer reviewed its raw output and unchanged source hash;
this report leaves ledger promotion and adjacent/isolated verification with
the coordinator.

AC10 (R10, primary owner P5):

> 明确恢复后 2、3（及暂停期间加入的指令）合为一批执行一次（M07），1 不自动重试；如无后续指令则保持空闲。

The independent review identified two concrete gaps in prior evidence: inputs
2/3 were added before Stop rather than during pause, and the idle test had no
target rather than a stopped target with no next input.

## New canonical source

`internal/service::TestCollaborationPausedFollowupRequiresProofFreshResumeAndNeverRetriesStoppedInput`
is in `server/internal/service/collaboration_paused_followup_test.go`. It has
two cases, `paused_inputs` and `empty_target`.

Both use the tracked managed fixture and public Submit/Resume/List/Claim/Permit,
Stop, proof and control-recovery services. Input 1 is claimed and granted an
actual persisted attempt permit; its reservation and unknown proof must survive
Stop. An unrelated managed target shares the runtime but has distinct control
scope.

- `paused_inputs` submits inputs 2 and 3 only after the target's Stop. Both
  submissions must retain paused state, one separate waiting batch, exact
  channel sequences and two human authors. Poll/reload cannot claim them before
  proof. Proof cannot automatically Resume or return input 1 to waiting. An
  earlier refused Resume keeps its exact refusal after proof; a fresh explicit
  Resume releases the future batch. One new claimed run contains exactly 2/3,
  with fresh group/run/launch identity, one permitted attempt and one final
  success. Old Stop/Claim/Resume identities never reach the new run.
- `empty_target` has the same attempted input 1 and physical barrier, but no
  added commands. After original proof and a fresh explicit Resume, repeated
  Poll/current execution reads must remain empty, retain exactly the old
  stopped group/run/attempt, and keep the target idle.

Both compare retained control snapshots across repeated List/current v5
ExecutionView reads, verify original group input membership stays identical,
and require the unrelated target plus issue-level control to remain unchanged.
Original-key GET/POST recovery is compared exactly; no read becomes a future
Resume. All existing product/helper files remain read-only.

The service proof is synthetic persisted protocol evidence. This new test does
not observe a provider process or natural kernel namespace termination. Prior
actual Linux Stop/proof tests remain corroborating evidence and are not
reclassified as this test's own kernel evidence.

## Executed command and provenance

The coordinator executed from
`/Volumes/artisan/code/2026/multica-0.4.37/server` after serially allocating the
task-owned schema-737 `service_s5` slot. No concurrent global-gate fixture was
allowed on that database.

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^TestCollaborationPausedFollowupRequiresProofFreshResumeAndNeverRetriesStoppedInput$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Result artifacts:
`paused-followup-coordinator-focused-20261021-result.json`, `.jsonl` and
`.stderr`. The result file contains the exact argument array, working directory,
start UTC `2026-10-21T12:22:55.828937+00:00`, return code 0 and all three pass
identities. Raw package result: 3.974s, with `paused_inputs` 1.20s and
`empty_target` 0.67s. There are no failed assertions or skips. This was the first
actual focused DB run; the prepared command was never executed separately by
the implementer.

`paused-followup-race-prepared-20261021.json` remains a historical prepared-only
record, not the execution result. Earlier static
`go vet -p 1 ./internal/service` with Go 1.26.6 passes in 2.510s; exact
command/source hash/streams are in
`paused-followup-vet-before-db-20261021*`. That run predates the final extra
GET/old-key assertions and is not misrepresented as a vet run of their final
hash. Final gofmt is clean. The baseline hashes seven existing product/helper
files in `paused-followup-baseline-20261021.json`; all remain unchanged.

Final new test SHA-256:
`85ba407981009a381aed0c6e408032cf999891f6cba6b4b645ff70fb02e99793`.
`paused-followup-final-scope-20261021.json` records the actual raw log/result
hashes, final source identity, format check and unchanged read-only source
baselines. The coordinator owns any final adjacent race/vet and isolated HEAD
test-only snapshot; these are not claimed before their artifacts exist.

## Criterion assessment and boundaries

The current service evidence now directly asserts paused-period inputs 2/3 are
batched once after proof plus a fresh explicit Resume, with exact authors,
distinct original/followup run identities, one new attempt and success. The
attempted input 1 stays stopped and its immutable historical membership and
attempt count remain unchanged. The stopped target with no next input remains
idle across repeated actual Poll and current v5 reads. Historical Stop/Resume
and Claim identities cannot affect later work, and another target plus the
issue-level controls remain untouched. These close both concrete AC10 gaps
from the independent review at the service protocol layer.

The proof is synthetic service protocol evidence, not natural namespace
termination or real provider execution. Existing Linux router/daemon Stop/proof
evidence supplies independent corroboration; any full primary-owner AC10
promotion remains the coordinator's review decision. No new Web/Electron,
packaging, other-OS or quota-consuming provider claim is made.

No matrix/spec/progress change, commit, task creation, archive, provider account,
environment service or production gate operation was performed by this lane.
Only the new canonical test, this report and unique paused-followup preparation
and verification artifacts were written; existing shared code was preserved.

## Coordinator isolated HEAD and adjacent checks

The coordinator copied only the new pause test into the recorded exact-HEAD
snapshot, which already held the independent content/grant test-only additions.
No shared uncommitted product implementation was included. Against task-owned
`service_s5`, the new two-scenario identity and four existing proof/Resume/batch/
stale-Stop companions pass: **5 top-level /7 total, zero failures/skips**, exit0.
`paused-followup-isolated-adjacent-race-result.json` records the exact selector,
HEAD, added-test inventory, source hash85ba407981009a381aed0c6e408032cf999891f6cba6b4b645ff70fb02e99793
and execution. Its JSONL contains each real PASS; no readiness or missing-DB
skip is counted. This completes the previously pending adjacent/isolation step.
The coordinator's initial current-tree focused invocation also passed both
scenarios with no skips; its exact command/result remains separately retained.
Independent criterion review and Lore commit follow without gate changes.
