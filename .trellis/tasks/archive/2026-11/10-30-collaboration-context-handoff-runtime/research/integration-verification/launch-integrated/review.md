# Launch integration review

The READY launch patch `40772c20f0848e48e96c4eb2e340c1d3ed5441ad1849177c29d933c8d7ea8927` is integrated. All eight final file hashes still match its owner handoff. No production source was changed by this review and no DB or real Provider was used.

## Passing checks

- Tagged launch/formal-authorizer suite: **16 top-level / 55 total test items**, zero failures/skips.
- Launch race suite: **14 / 44**, zero failures/skips.
- Tagged vet, final test-file gofmt and scoped diff check: pass.
- Imported only the explicitly approved fixture-release delta from main `host_checkpoints_test.go`, after verifying the integration copy was untouched. Before/after/source hashes are in `checkpoint-fixture-import.json`. The previously hung `TestHostCheckpointBackpressureDoesNotBlockMessageDrain` now passes **with `-race`**, in 2.925 seconds wall time.

Exact commands, timings, exits, names and raw output are retained in the adjacent `*-metadata.json` and `.log` files. Earlier lane-only or initial-snapshot results are not substituted for these integrated runs.

## New combined regression — blocking

Only `server/internal/daemon/collaborationrun/host_handoff_checkpoint_test.go` was added. It reuses the existing launch fixture with `RecordingVersion=1` and adds the two narrow recording HTTP operations. It does not repeat the standalone launch validation matrix.

| Case | Result |
| --- | --- |
| Snapshot refused before sender creation | Pass: no stream/Provider; original termination and proof ACK retained |
| Binding refused after sender creation | Fail: Provider does not start and proof ACK succeeds, but recording has no seal and `SealAccepted=false` |
| Fresh fallback snapshot drift, first checkpoint ACK intentionally lost | Fail: no second Provider/permit and original proof ACK succeeds, but only one checkpoint request occurs, its original payload remains pending, and no seal is sent |

The complete regression returns in 1.756 seconds, so these failures are missing recovery/seal behavior, not a reproduced sender hang. Its original raw failure is retained in `handoff-checkpoint-regression.log`.

Code inspection agrees with the failures: the integrated `recoverHost` path does not call `recoverHostRecording` or `sealHostRecording`. The helpers are present and have direct unit coverage, but the actual handoff failure path never invokes them. The main coordinator has routed this to the runtime/checkpoint owner. This reviewer did not patch production recovery ordering.

## Required handoff

After the owner imports the complete recording recovery change, rerun `^TestHostHandoffCheckpointFailureCleanup$` and its race version, then the affected daemon suite and tagged launch/authorizer checks. Preserve original termination/proof progress when recording delivery is unavailable, and only replay the original checkpoint key/payload. Do not treat the repaired fixture or isolated startup suite as whole-runtime acceptance.

The new regression file is owned by this review lane. SHA-256: `717c992a0e4b11ba5e6d3c2d9b667e663e325070043bd150bdd0096714b88455`.
