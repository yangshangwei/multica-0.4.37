# Recording recovery boundary review — first pass

Date: 2026-11-01 (Asia/Shanghai). Reviewer: native `recovery_boundary_review` (`trellis-check`). Scope: the existing task's fixed dirty-source snapshot in `mc-runtime-recording-recovery-20261101`, HEAD `e3620721efacb820b89709c8db9d326d9cf2b00b`; file hashes and captured source set are in `baseline.json`. The implementer owns daemon recovery changes. This first pass wrote no product file and does not replace the final review after those changes.

## Findings (fixed)

None. This pass is read-only for product source; reproductions and logs are retained as evidence.

## Findings (not fixed)

### P1 — Recording recovery is not connected to the shared original-proof path

In the reviewed baseline, `server/internal/daemon/collaborationrun/host_runner.go:439` handles stop, persisted launch observations, the original outcome and original proof, but never calls `recoverHostRecording` or `sealHostRecording`. In contrast, `ledger.go:114` already retains proof-acknowledged host records while `hostRecordingPending` remains true. A restarted Runner consequently returns through `runHost:40` to the recovery path yet never drains recording requests. Repeated scans cannot repair the missing call path.

The implementer owns this fix and its failing combined recovery cases. Keep recording errors separate from termination/proof failures, cap aggregate telemetry work per recovery round, and preserve already-persisted proof priority. Lifecycle Stop already occurs in `host_recording.go:147` before the recording loop can finish; helper-only sender tests do not demonstrate original-proof completion under blocked uploads.

### P2 — A malformed seal receipt is accepted and can clear pending seal recovery

`server/internal/daemon/collaborationrun/client_host.go:62` validates version, stream, sealed flag, retained range and reason, but does not validate `ObservedThrough`. The temporary Go-overlay test `TestBoundaryReviewSealRejectsInvalidRetention` was run without writing a product file and failed for all three responses:

- Negative `observed_through`.
- `reason=completed` with an unknown/null observed tail.
- `observed_through` below `retained_through`.

Go exited **1** with named `accepted malformed seal receipt` assertions. See `boundary-seal-response-red.log`, the exact command in `boundary-seal-response-red.json`, and the retained temporary test in `boundary-seal-response-overlay.go.txt`.

This is observable acknowledgement behavior: `sealHostRecording` sets `SealAccepted=true` after any nil client error, so invalid returned metadata is treated as a successful seal acknowledgement. It does not grant business authority, but violates the strict new-response contract and can falsely stop seal retry. The client file is outside the implementer's assigned ownership; the leader was asked to assign its focused validation change and canonical malformed-response tests.

### P2 — Later learned checkpoint ACKs cannot advance a one-shot local seal

The baseline `host_checkpoints.go:208` returns immediately after `SealAccepted`; at `:245` it creates only `Sequence=1`. If a checkpoint was accepted but its ACK was lost, and an earlier seal is accepted, later original-checkpoint replay can increase the local ACK watermark without any new metadata revision. The server already accepts exact older-key seal replay and monotonic bounded next-sequence metadata (`collaboration_checkpoints.go:247`, `:310`). The implementer was notified to preserve an uncertain seal's exact bytes first, then use a new sequence only after acknowledgement and only to add knowledge.

The server permits at most 32 seals and requires fixed stream/reason plus nondecreasing ACK/known observed watermarks. `ListCollaborationRecordingEvents` uses `LIMIT 290`, covering the 256 checkpoint + 32 seal + one outcome maximum and one corruption sentinel. No SQL/protocol change is necessary for that existing advancement contract.

## Boundaries verified by source

- Checkpoints require the original issued business capability, its attempt sequence, full frozen termination identity and authorized attempt/nonce (`collaboration_checkpoints.go:154`, `:160`, `:167`). Identical retained key/payload ACK precedes current-authority append, while changed key payload conflicts. A novel append locks current business authority and reservation, then rechecks the original key under lock (`:182`, `:191`).
- Seal requests use only the original termination token and frozen attempt identity (`:230`, `:237`). Their DTO has no body/file/messages fields. The handler applies bounded strict unknown/duplicate-field JSON checks. Seal events add completeness metadata; they do not assert another proof, release capacity or restore business authority.
- Retained prefix is computed from stored contiguous messages (`:105`), never from caller-reported ACK. An observed bound below that stored prefix is clamped to the actual retained prefix (`:110`); lost ACK/old seal cannot erase server-retained content.
- The canonical handler purpose matrix actually includes checkpoint, seal, live/snapshot/bind/source handoff and runtime comments/content (`collaboration_runtime_test.go:85`). It refuses human/daemon/ordinary-task credentials and wrong-purpose runtime tokens before reaching write dependencies.
- Host claim extensions are explicit query negotiation and do not alter the strict legacy claim body. Unknown versions and namespace acquisition of host-only extensions are rejected. The bare-handler invalid-version tests verify refusal before any claim service dependency is reached.
- A post-seal novel checkpoint is refused, while a previously stored exact request remains ACKable. Recovery should attempt pending original checkpoints before a new seal, then truthfully retain any unavailable/incomplete tail.

## Verification

Exact commands, actual test names, timestamps, exits and raw JSONL are in `boundary-test-summary.json` and its linked logs.

| Check | Result | Evidence scope |
| --- | --- | --- |
| Protocol checkpoint/record/seal tests | Pass: 3 top-level / 11 including subtests; exit 0; zero skips | DB-free |
| Client host-extension negotiation tests | Pass: 2 top-level / 9 including subtests; exit 0; zero skips | DB-free fake HTTP/Provider fixtures |
| Handler strict purpose/body/version guards | Pass observed: 7 top-level / 11 including subtests; exit 0; zero skips | **Exploratory default-DB bootstrap; not exclusive task-DB evidence** |
| Temporary malformed seal-response test | Red: 1 top-level / 3 named subtest failures; Go exit 1 | DB-free Go overlay; no product writes |
| Lint/typecheck | Not run by this bounded first pass | Leader coordinates full static checks |
| DB checkpoint concurrency/authority matrix | Not run by this reviewer | Leader owns exclusive serialized DB execution |

Environment correction: the handler guard functions themselves are dependency-free, but `server/internal/handler/handler_test.go:44` has package-wide TestMain. With DATABASE_URL removed it selects its default localhost DB, creates/cleans fixtures before `m.Run`, and can exit 0 without running tests when DB setup cannot connect. The run above contains actual named test passes, but used that fallback bootstrap. The leader identified this and will rerun against `/tmp/multica_runtime_recovery_20261101_d9b714bf.env` after the service suite. No further handler/service package was run by this reviewer.

The service checkpoint tests rely on `newCollaborationFixture`, which skips without task-owned DATABASE_URL (`collaboration_test.go:32`). Existing checkpoint tests cover a sequential Stop/exact ACK/body conflict/seal advance and outcome overlap/public-reader projection. They do not establish both transaction lock orders for Stop, grant revocation or author departure. Those AC1 cases remain separately pending even if the current focused tests pass.

## Handoff for final review

After the implementer finishes, independently rerun the combined outage/restart cases with race detection, inspect aggregate deadline handling and unchanged original payload/credential/proof identity, and recheck the malformed seal-response regression after its assigned fix. Do not mark AC1–AC10 or real-model/native-platform acceptance complete from this bounded review. Original experimental slots and live main source remain outside this review's write scope.
