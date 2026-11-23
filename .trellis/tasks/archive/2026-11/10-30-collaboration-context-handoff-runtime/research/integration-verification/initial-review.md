# Runtime integration initial review — 2026-10-31

This review used the fixed overlay on `e3620721efacb820b89709c8db9d326d9cf2b00b` in `mc-runtime-integration-20261031`. All files still match `runtime-source-snapshot.json`; no product source was edited. Exact command, package/test identities, exits, skips and timings are in the `*-metadata.json` files. `non-db-source.json` and `non-db-source.diff` retain snapshot/diff provenance.

## Executed verification

| Suite | Result |
| --- | --- |
| Protocol | PASS: 16 top-level / 66 test items, zero skips |
| CLI and internal CLI | PASS: 24 / 92, zero skips |
| Initial daemon/client selection | 11 / 47 passed, then one test timed out during fixture cleanup; package exit 1 |
| Remaining daemon/client tests not reached by that timeout | PASS: 24 / 50, zero skips; previously passed tests and the known blocked fixture were not rerun |
| New checkpoint/handoff/recall service tests | PASS: 7 / 7, zero skips |
| Recall and extension-refusal handler tests | PASS: 2 / 6, zero skips |
| gofmt, targeted go vet, source diff check | PASS |

DB tests used only the explicitly delegated `multica_runtime_integrate_20261031_f2478725`, service then handler sequentially. No real Provider or shared application DB was used. The original baseline-only protocol result was not substituted for the overlay run.

## Blockers and handoffs

1. **Checkpoint test fixture cleanup hangs.** `TestHostCheckpointBackpressureDoesNotBlockMessageDrain` reaches deferred `httptest.Server.Close` at `host_checkpoints_test.go:124`; the HTTP handler at line 83 remains blocked on `r.Context().Done`. `daemon-client.jsonl` records the active connection and the two-minute timeout. Route a bounded fixture-release fix to the checkpoint owner, then rerun that test and the affected daemon suite. The stack does not establish a product sender-drain failure.
2. **Seal update contract needs a coordinated decision.** `SealRuntimeRecording` in `collaboration_checkpoints.go` accepts a second seal only when `reflect.DeepEqual` to the first; every changed observation/completeness payload returns `recording_seal_conflict`. The approved design requires monotonic later completeness updates, including after settlement. Existing passing tests cover the identical replay and retained prefix, not this transition. The checkpoint owner/main must implement the agreed contract or explicitly reconcile the design, with a DB regression; this reviewer did not rewrite it.
3. **Startup injection remains a pending owner handoff.** The separate launch lane was not imported into this snapshot. Its actual-stdin, bind-before-launch, unsupported/malformed response, retry/fallback and purge tests are still required after import. Absence of that handoff was not classified as a current implementation failure.

## Wiring inspected

The earlier router omission is resolved in this snapshot. Runtime checkpoint/seal, comments GET/content, handoff/snapshots/bind/sources and the human run-handoff route are registered in `cmd/server/router.go` and match their daemon/CLI/core callers. Human reads stay under human authentication; runtime methods retain their narrow bearer handlers. Claim extensions are negotiated via query parameters while the strict old claim body remains unchanged; namespace claims reject host-only extensions. The targeted client and handler tests exercised those negotiation/refusal contracts.

A registered route is not end-to-end authorization evidence. The service/handler runs above do not replace router-level negative-principal tests, final frontend checks or native acceptance.

## Next checks after owner integration

- Fix and rerun the checkpoint fixture; verify the full affected daemon suite, including newly imported launch tests.
- Add/run the remaining acceptance matrices: stop/revoke/author-exit in both commit orders; permanent-gap and stale/late-seal cases; snapshot source revision/purge; claim-frozen versus late wake causes; actual routed negative principals and request identity.
- Validate generated SQL drift and targeted endpoint/API/schema contracts on the final integrated source.
- Run the main coordinator's frontend checks once, then production Web and native Desktop acceptance. No frontend checks were duplicated in this review.

Overall quality gate remains **not passed** while the fixture timeout, seal-contract decision and pending integration/acceptance work remain.
