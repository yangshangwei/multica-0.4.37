# Independent review: combined handoff and checkpoint cleanup

The reported reds are valid regressions against the inspected production wiring, not failures caused by an incorrectly shaped seal response. The current test is a useful cleanup regression, but does not yet prove recording/proof independence or recovery across a later bounded round.

## Scope and evidence

- Worktree: `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`.
- Test reviewed: `host_handoff_checkpoint_test.go`, SHA-256 `717c992a0e4b11ba5e6d3c2d9b667e663e325070043bd150bdd0096714b88455`, exactly matching the coordinator's target and retained run metadata.
- Read retained `research/integration-verification/launch-integrated/handoff-checkpoint-regression.log` and metadata. That run reports snapshot refusal passing, binding refusal/fallback drift failing, zero skipped cases.
- This review did not run tests, edit tests or production code, or call any Provider. Test changes belong to `resume_check`; production recovery changes belong to the checkpoint owner.

## 1. The production red is real

`runHost` calls `sender.finish()` at `host_runner.go:221`; this drains/stages a batch and waits for the sender goroutine. It does not send the recording seal and does not retry an uncertain checkpoint.

`recoverHostRecording` and `sealHostRecording` exist in `host_checkpoints.go:170` and `:206`, but the inspected `recoverHost` (`host_runner.go:444–500`) calls neither. It handles outcome and original proof only, including an immediate return for `phaseAcknowledged` at `:497`.

This exactly explains the evidence:

| Case | Expected lifecycle | Observed retained failure |
| --- | --- | --- |
| Snapshot refused | Fail before `startHostCheckpoints`; no recording stream | Pass: nil recording, no checkpoint or seal requests |
| Binding refused | Stream exists, Provider never starts, empty stream can be sealed after original terminal facts | Recording exists but `Seal=nil`, `SealAccepted=false`, zero seal requests |
| Fallback snapshot drift | First Provider ended; no second Provider; retained continuity record needs same-key ACK recovery and seal | One pending checkpoint, only one HTTP checkpoint request, zero seal requests |

`ledger.go:114` already retains acknowledged runs while `hostRecordingPending` is true. Because the main recovery function ignores that pending state, a later inventory scan can keep rediscovering the run without advancing its recording recovery. The direct helper unit test in `host_checkpoints_test.go` cannot cover this missing Runner wiring.

## 2. What the current assertions establish

- **Both negotiated capabilities are active.** `newLaunchHandoffFixture` sets `HandoffVersion=1`; line 28 sets `RecordingVersion=1` before claim. Bypassing either path would fail the expected starts/recording assertions in the affected cases.
- **The three startup positions are distinct.** Snapshot refusal is before sender creation; binding refusal is after sender creation but before process start; fallback drift occurs after one actual fake Provider rejects resume and before a second permit/start. The start count checks are useful.
- **Original proof still reaches ACK in the observed run.** The test requires one unique permit, termination and proof, a stopped ledger and `phaseAcknowledged`. These are logical attempt/fact counts, not raw HTTP request counts; permit retries may legitimately make more HTTP calls.
- **One lost ACK must replay exact content.** The fake advances `retained` before closing the first checkpoint connection. The required two requests plus `reflect.DeepEqual` checks identity, attempt, stream, sequence, offset and message payload equality. The authorization check also requires the same issued-attempt business token. One accepted ledger chunk with cleared payload is a meaningful local recovery assertion.
- **There is no artificial seal-response mismatch.** The seal endpoint returns `CollaborationRecordingRetention` with version, stream, retained/observed counts, `Sealed=true` and the request reason. This matches `Client.RecordingSeal`. The observed red has *zero* seal requests, so response validation cannot explain it.

The fixture is deliberately not a replacement for the production checkpoint sequence/idempotency/retention implementation: it advances a maximum retained offset rather than storing the real event stream. That is sufficient for this one-prefix lost-ACK case, not evidence for the entire storage contract.

## 3. Missing assertions to close combined acceptance

### A. Proof must progress while recording remains unavailable (important)

Current recording endpoints either succeed immediately or lose one response and then succeed. Therefore a fix that waits for successful checkpoint/seal recovery before confirming proof could still pass all three cases. The ten-second end-to-end timeout is a hang bound, not proof of independence.

Add a controlled persistent-503 or blocked checkpoint/seal case. Observe original proof ACK while the recording endpoint remains unavailable (before the test releases its gate), and verify pending recording metadata/payload remains recoverable. Do not require proof before normal in-flight checkpoints during execution; the property is that successful recording recovery/sealing is not a prerequisite for independent termination proof. Use barriers/recorded order rather than a fragile sub-second sleep assertion.

### B. Exercise the next real Runner recovery round (important)

The current lost-ACK case checks only the initial `Runner.Run` cleanup. A direct call to `recoverHostRecording` or a retry added only inside `sender.finish` can make it green while leaving the acknowledged-run recovery entrypoint broken.

Add two phases: finish the run with recording still pending and proof ACKed; then make recording available and call the normal retained-run/Runner recovery path again (optionally reopen the ledger/Runner). Verify the exact original checkpoint payload/token is replayed, no new Provider/permit is created, the seal is acknowledged, and the run no longer remains in recording-pending inventory. Replaying an already accepted checkpoint must not append another local chunk or rewrite its identity.

### C. Pin original identities and termination meaning

Counts at line 128 do not distinguish a truthful `not_started` from a falsely reported managed exit. Assert the sole original attempt's full run identity, sequence and nonce against its permit, and:

- snapshot/binding refusal: termination and proof launch are `not_started`;
- fallback drift: the first launch is `managed_launch_ended`, with no second permit/launch;
- proof contains that same original terminal fact and the ledger retains the corresponding receipt.

The existing fixture validates important pieces, but the combined test should retain these named semantic assertions because they are the reason original proof matters here.

### D. Make seal acceptance faithful and check its coverage

The checkpoint fake checks its issued attempt/nonce. The seal fake at line 100 only checks general protocol shape, run identity and termination token. Production `SealRuntimeRecording` also invokes `validateRecordingAttempt` and checks the exact issued original attempt/nonce. Add that comparison to the seal fixture so a wrong but syntactically valid nonce cannot falsely pass after production wiring is fixed.

After success, assert seal stream identity and coverage against the actual retained prefix and recovered ACK watermark (0 for the empty stream, one message for this fixture's recovered prefix), and check that accepted chunk payload **and business token** are cleared. A completed recording coverage marker must not be confused with successful business work; avoid inventing a task-success assertion from the seal reason.

Do not generalize `len(seals)==1` to a future lost-seal-ACK case: repeated HTTP requests with one unchanged logical seal are valid. In the present cases the seal endpoint never drops a response, so the single-call expectation is reasonable.

## 4. Diagnostic note

The current failure formatter at line 149 prints the whole recording ledger, including a fixed test-issued `mcb_111...` token. This is not a real-account leak, but counts/flags instead of `%+v` would keep retained regression evidence concise and avoid embedding capability values unnecessarily.

## Review disposition

Keep the current red regression; it correctly detects missing production recovery/seal wiring. Add A/B before claiming proof-independent and restart-capable combined recovery, and C/D to make the identity/receipt assertions precise. No production fix is proposed or applied here; the checkpoint owner should preserve original proof priority while integrating the two recovery helpers.

This read-only assessment is tied to the hashes below. A subsequent production fix needs its own run evidence; this report is not a claim that the tests now pass.

```json
{
  "captured_at": "2026-10-31T15:24:05.210229+00:00",
  "files": {
    "server/internal/daemon/collaborationrun/host_handoff_checkpoint_test.go": "717c992a0e4b11ba5e6d3c2d9b667e663e325070043bd150bdd0096714b88455",
    "server/internal/daemon/collaborationrun/host_runner.go": "565d518d352199050f110e847f89558d1980b94ddc67d0c6db5a33eff6e6afa2",
    "server/internal/daemon/collaborationrun/host_checkpoints.go": "b70f1ddd0078c09c35dcea1407e5f7348f0e374b5f8fababe1a7794fd0fc79a1",
    "server/internal/daemon/collaborationrun/host_checkpoint_ledger.go": "e3db1ac44dde190fae20abdd208e9dafdd648f80a540b5e069a71d21338c39cf",
    "server/internal/daemon/collaborationrun/ledger.go": "69b3e6fef876cd802c5690558de874c71d153f16552164b74bffc15090524b33",
    "server/internal/service/collaboration_checkpoints.go": "520a03d8d115210b11fb7d757a2b04d61218a378980b755c84353164d386897d"
  }
}
```
