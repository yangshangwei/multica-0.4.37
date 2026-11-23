# Late runtime reconciliation boundary

Preserve the tested integration runtime; main is read-only. Compare the captured late bytes and keep their unique semantic improvements without replacing files wholesale.

- Retain production `client_host.go`, `host_checkpoint_ledger.go`, `host_checkpoints.go`, and `host_runner.go`: exact checkpoint ACKs, shared strict retention decoder, known-drain marker, no-gap staging latch, one-second total telemetry budget, original proof priority, immutable seal replay, monotonic updates, and frozen launch inputs already supersede the late variants.
- Add a small client recording ACK suite. Keep request/response binding and representative decoder wiring cases here; the exhaustive retention field/count matrix remains canonical in `pkg/protocol/collaboration_recording_retention_test.go`.
- Extend the existing recovery test file with unique explicit-Stop and malformed-seal restart cases, reusing existing fixtures. Preserve all current tests and the frozen combined/budget/wire suites.
- Retain the endpoint-aware execution-panel fetch stub, which already fixes independent response queues and preserves real handoff wiring; verify the file unchanged.
- Merge the late recovery index description while preserving the startup-handoff index entry. The parent expanded ownership for the linked recovery spec; merge useful late material and correct its obsolete timing/decoder guidance.

No production behavior refactor, source copy to main, commit, runtime state changes, database access or authenticated Provider calls. Verification uses HTTP fakes and the existing test executable only. Capture before/after hashes and exact command logs.
