# Runtime checkpoint/seal diagnostic plan

Status: read-only plan. No product/framework edits, compilation, tests, database
operations or Provider calls were performed for this plan. Heavy execution is
paused until the password7/original14 owner explicitly releases the shared
window. A lack of visible processes is not that release.

## Current conclusion and execution order

The recorded race run is a real failed acceptance attempt, but it did not meet
the registered quiet-window precondition: another session's password7 run
overlapped 15:42:54–15:45:03 UTC. Preserve its `60 pass / 2 fail`, database,
private ledger, raw trace and source identities. The overlap is not proven to
have caused the failure.

After explicit release, root's next action is one newly numbered complete
ordinary/race pair, serially, on two fresh migrated databases. Keep the original
seventeen framework files, selector, timeouts, scoring, sample numbering and
Provider budget unchanged; bind the observed quiet window and source before and
after. This is a new acceptance attempt, not a retry appended to the original
trial. A passing pair neither erases the earlier failure nor proves the
underlying timing mechanism was repaired. If the quiet pair remains incomplete,
use the focused diagnostics below before proposing a product change.

The current data does **not** prove a product contract defect. The product
explicitly permits bounded foreground recording failure while retaining original
telemetry for later recovery and completing termination/proof independently.
The RK1 framework calls `Runner.Run` once and immediately scores; it does not
exercise the daemon's later `recoverProofs` loop. These are different boundaries.
We must establish whether eventual recovery works, and which data can still be
accepted after settlement, before changing either contract.

## Retained facts

- Actual failing signal: `TestRK1FakeProof/wrong_continues`, trial2,
  `incomplete/measured:handoff_or_recording_event_missing`. Its trial subtest
  itself reports a measured incomplete result; the enclosing fixture compares
  the expected two complete wrong answers and fails.
- One physical launch and one authorized attempt; Provider exited0. Runner error
  combines `collaboration runtime transport failed` and `context deadline exceeded`.
- Five checkpoint503 responses took approximately975–1002ms. A checkpoint200
  took992.799ms; its second chunk is present server-side but lacks a local ACK.
  A separate stop-intent503 occurred. Outcome508.922ms and proof99.867ms succeeded.
- Local phase is `proof_acknowledged`; outcome is recorded. Chunk1 is ACKed;
  chunks2–5 remain pending. Recording is closed/drained with reason `incomplete`.
  Seal sequence1, acknowledged-through1, unknown observed tail is persisted but
  unacknowledged. No seal HTTP request exists in the82-event trace.
- Server events show checkpoint1/2, outcome and proof settlement. Ordinary mode
  also records a partial outcome with reason `incomplete`, but it has five
  checkpoints and an accepted seal. A partial outcome alone is not this failure.
- Trace `at` records handler completion, not start; middleware logs add complete
  handler durations. Neither separates CPU validation from database waits.

Exact fields and source hashes are in [race-diagnosis.json](race-diagnosis.json).
Credentials and original ledger bodies stay private.

## Three falsifiable hypotheses

| Rank | Hypothesis | Distinguishing probe after release | Disproof |
| --- | --- | --- | --- |
| 1 | Processing a64KiB recorded message under race instrumentation consumes most of the one-second checkpoint request budget before the write completes. | On a fresh diagnostic fixture, record stage durations around protocol validation, `redact.Text`, transaction entry, lock acquisition and commit. Separately time the exact test-owned body through the pure validation/redaction functions. Keep original request deadlines. | CPU stages are small while delay is concentrated in pool or lock waits. |
| 2 | Concurrent termination/stop polling or shared environment pressure causes pool/row-lock waits in the checkpoint path. | Record bounded stage timings and exact fixture-owned lock waits in the same fresh fixture; preserve the original workload and use a controlled contender to distinguish a lock wait from CPU work. Never terminate another session's connections. | The checkpoint deadline is consumed before database work, or the wait remains absent while the failure reproduces. |
| 3 | `recoverHost` defers seal delivery using a recording context created before checkpoint recovery and independent outcome/proof, so the context has expired when seal delivery begins. | A DB-free `httptest` diagnostic gives checkpoint an immediate ACK and delays the independent outcome past the existing recording deadline while keeping the caller deadline live; count seal requests and inspect the persisted seal. Include an immediate-outcome control. | A seal request is still issued with the exhausted original context, or this controlled path does not reproduce a persisted unacknowledged seal. |

Hypothesis3 is visible as a reachable mechanism in
`server/internal/daemon/collaborationrun/host_runner.go:476` and
`host_checkpoints.go:192`, but it does not explain the earlier checkpoint503s.
In this failed sample, checkpoint recovery may itself consume the whole budget;
removing lifecycle time from that budget would not necessarily change the result.

## Smallest useful recovery diagnostic

Proposed isolated test name:
`TestHostRecordingRecoveryReplaysCommittedPrefixAndSealsAfterAcknowledgedOutcome`.
Place a future test beside
`server/internal/daemon/collaborationrun/host_recording_recovery_test.go`, using
the existing helpers instead of a new framework. This is a plan, not an added
test or a passing result.

The available seams are:

- `retainedHostRecordingFixture`, `newHostTestLedger`, `registerHostTestRun` and
  `hostTestPermit` in the collaborationrun tests create valid private ledger
  records with issued identities. `recordHostOutcome`,
  `acknowledgeHostOutcome`, `finish`, `stageHostCheckpoint` and normal ledger
  persistence construct canonical synthetic state without editing raw records.
- `newRecordingServer` in `host_checkpoint_delivery_test.go:36` already models
  exact-key replay before refusing new content after outcome/seal. Its server
  state must reflect settlement; do not use an always-accepting HTTP fake.
- `TestHostHandoffCheckpointRestartRecoversWithoutExecution` in
  `host_handoff_checkpoint_lifecycle_test.go:193` demonstrates reopening the
  on-disk ledger and using `Runner{Ledger: reopened, Client: client}` with no Host.
- `TestHostRecordingSealReplaysBeforeAdvancingAcknowledgement` and
  `TestHostRecordingRecoveryMalformedSealACKKeepsOriginalPending` provide the
  original-seal identity and malformed-receipt checks.
- Production `collaborationPoller.recoverProofs` in
  `server/internal/daemon/collaboration.go:178` uses this same no-Host Runner
  under one ten-second round and the persisted recovery cursor. A direct Runner
  test proves recovery of one record; it does not prove scheduler fairness.

First use a separate private byte-for-byte copy of the failed ledger plus a
loopback mock reflecting retained server facts. Preserve original file hashes
before and after, never point the diagnostic Client at the original API/DB, and
never log its credentials. The canonical regression should construct the same
state with the helpers above; it must not depend on a personal absolute ledger
path or commit real capabilities as test data.

The copied/synthetic starting state is the acknowledged run described above:
one issued launch, stopped process, immutable accepted outcome/proof, two server
checkpoint keys, only first checkpoint locally ACKed, pending chunks2–5 and an
original pending seal. The first two original payloads can be reconstructed for
the mock from the sanitized trace; the pending payloads remain in the private
ledger. Compare full payloads in memory; publish only hashes and ordinal counts.

The mock's exact behavior matters:

1. ACK chunk2 only when its original key, identity, attempt, nonce, stream,
   offset and payload equal the already committed second checkpoint. It is an
   idempotent receipt, not new content.
2. Reject new chunk3 with403 `business_capability_required` for the final-proof
   state: `collaboration_runtime_capabilities.go:728` revokes run business
   capabilities, and `lockRuntimeBusiness` checks that revocation. The existing
   `collaboration_outcome_receipt_test.go` verifies this boundary. Keep a
   separate409 `execution_stopped` control for a settled run whose capability
   was not the first refusing gate. `RecordRuntimeCheckpoint` checks retained
   exact keys before `lockRuntimeBusiness`; new writes require that later gate.
3. Expect the client to resolve later uncommitted chunks as rejected through
   the existing cascade. Never accept chunks3–5 after settlement just to make
   the diagnostic pass. Retention remains a prefix and missing content remains
   explicit.
4. Accept the original persisted seal byte-for-byte first: sequence1,
   acknowledged-through1, same reason and unknown observed bound. Return the
   actual server retained-through2.
5. If the client then learns the higher contiguous ACK, accept only the valid
   monotonic sequence2 update with acknowledged-through2 and the same reason,
   identity and observed semantics. The old uncertain seal cannot be rewritten.
6. Fail every request except the permitted checkpoint and seal requests. Count
   Claim, Permit, preparation, handoff/snapshot/bind, termination, outcome and
   proof calls explicitly and require all zero for this acknowledged starting
   state. `Host == nil` and an unchanged launch inventory provide an additional
   no-execution assertion; no Provider is installed or invoked.

Final assertions:

- Original run/attempt/nonce, outcome/proof values and one physical launch remain
  unchanged; the existing accepted proof is not posted again.
- Chunk2 becomes accepted with its payload/token released; chunk3 and the later
  suffix become rejected, not falsely accepted. Server retained-through stays2.
- The original seal and any valid advance obtain durable ACKs. Unknown/missing
  tail and `incomplete` reason remain visible.
- `hostRecordingPending` becomes false and `Ledger.Retained()` no longer offers
  this record after reopening the diagnostic ledger again. Clearing retained
  work does not mean the original recording became complete.
- Negative controls retain pending state on503 or malformed seal ACK and keep
  the original proof/identity intact. Do not add an unbounded recovery loop.

This test should pass if the current production recovery works as intended.
It would go red on a recovery defect such as lost original payload identity,
failure to settle the pending seal, unauthorized re-execution, or false suffix
acceptance. We have not run it and cannot assert that current code fails it.

## What is, and is not, a justified red regression

The one existing test that demonstrably goes red on the observed acceptance
failure is `TestRK1FakeProof/wrong_continues`. The full recorded command and raw
failure are preserved; a narrow future selector may be used only as a separately
numbered diagnostic on a new fixture after release, never as a replacement
sample or a selective final-acceptance pass.

A deterministic fast diagnostic of hypothesis3 can reuse
`retainedHostRecordingFixture` plus `newRecordingServer` and a pending outcome,
with controlled handler barriers. Compare an immediate independent outcome
with one held beyond the unchanged recording deadline while the caller stays
live. Both must preserve the proof; the slow branch should expose the pending
seal and its unchanged identity. This establishes budget behavior. Requiring
the slow branch to synchronously complete its seal would invent a stronger
product guarantee than the current bounded-recovery contract, so it is not by
itself a valid failing product regression.

If the no-Host recovery diagnostic passes, the evidence supports intentionally
deferred persistence plus a single-Run acceptance boundary; it does not grant
permission to silently add recovery to old RK1 samples. A framework revision
would require an explicit new measurement contract, independent review, new
framework/source binding and fresh full fake verification. Root currently
directs no such change.

If recovery fails against a faithful post-settlement mock, confirm that failure
at the real service boundary in a separately allocated diagnostic database, then
write the smallest product regression before a patch. If recovery succeeds but
the same incomplete result persists in a coordinated quiet full pair, investigate
checkpoint stage timings before modifying a deadline or retention rule.

All diagnostic execution and the fresh acceptance pair remain pending explicit
shared-window release. No original failed sample will be changed to pass.
