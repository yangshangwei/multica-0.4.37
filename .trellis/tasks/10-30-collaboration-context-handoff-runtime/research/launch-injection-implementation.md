# Startup injection lane

Scope agreed with the parent: daemon startup injection and verification only. Product source baseline is 4d0d1a76d; borrowed protocol and client prerequisite snapshots are recorded in launch-injection-baseline.json. Server snapshots, checkpoints, recall, router and protocol stay owned by the parallel workflow.

## Behavior and ownership

A negotiated HandoffVersion=1 claim must fetch an authenticated frozen handoff after preparation and the memory base. The provider receives it in the startup JSON, with its missing/truncation/provenance fields intact. Unsupported claims preserve the existing path; a supported failure cannot silently downgrade. Snapshot identity is stable for one logical run input, including transport replay and outer fresh fallback.

Owned files: host_runner.go; the parallel lane's host_handoff.go (minimal corrections only); focused host_handoff/client_handoff tests. Avoid ledger changes if a deterministic run-bound snapshot request UUID supplies stable identity without retaining plaintext. No proactive snapshot/source copies enter brief or supervisor metadata; existing Provider output retention is separate. No checkpoint or service implementation is included.

Provider BeforeLaunch reauthorizes first, then binds the snapshot and actual prompt hash to that attempt before burn/spawn. Git preparation never binds. Internal provider retries bind each actual attempt. A fresh fallback uses the same snapshot and checks the re-read memory base against it. Supported Codex executions must not allow adapter-added continuity text to change the bound input; put conditional continuity guidance in the hashed host prompt and suppress adapter prepending.

## Verification

Baseline host memory/runner/prompt tests passed (5.954s). First write a failing real Runner/fake executable regression for automatic snapshot injection/binding. Then cover rejected/malformed snapshots and receipts, lost ACK replay, fresh fallback, changed memory/purge, old unsupported claims, Git preparation and Codex internal launch retry/turn input. All executables are test-owned; no real providers or accounts. Run the daemon suite and vet, record any borrowed-dependency failures separately.


## Integrated prerequisite update

While this lane was starting, the parallel workflow delivered its host_handoff.go and checkpoint wiring. The parent refreshed the borrowed snapshot and saved original Git blobs. This lane adds only corrections to that implementation, preserves checkpoint hooks, and uses its established `handoff/snapshots` and `handoff/bind` routes. The separate client_handoff.go originally considered was not created.

The privacy assertion is limited to no proactive handoff/source duplication into brief files or supervisor ledger metadata. A provider can still quote supplied facts in the existing redacted output/tool records; this lane does not claim otherwise.

## Confirmed regressions

- Original startup path omitted handoff in actual fake-provider stdin (failed before prerequisite refresh).
- Refreshed implementation minted a new logical snapshot on fresh fallback and accepted a member-to-leader assignment mismatch; focused tests failed before this lane's fixes.
- Current claim negotiation received 403 from the formal smoke authorizer; a tagged, non-provider test reproduced it before the narrow query allowlist repair.
- A Go overlay removing only negotiated continuity-notice suppression fails actual Codex turn/start hash assertions in both live resume rejection and internal catalog retry. The worktree source was never mutated for this counterfactual; raw log and source hash are retained in verification/launch-injection/.

## Final implementation and verification

- Corrected host_handoff.go: deterministic run-bound snapshot identity; same-envelope fallback validation; claim role/step/parent and linked assignment checks; exact supported/empty memory semantics; bounded response/read validation; binding request/receipt checks; correct retry backoff.
- Corrected host_runner.go: use the same snapshot across outer fallback; include direct member assignment guidance; prevent Codex from adding unbound continuity text; retain the other lane's checkpoint hooks unchanged.
- Corrected tagged smoke authorizer: only managed-token POST /claim permits known, single-valued version=1 negotiation query keys. No real smoke was run.
- Added host_handoff_launch_test.go and host_handoff_codex_launch_test.go. Fake executables verify actual Claude stdin and Codex turn/start text, one-process resume rejection and two-process internal retry, original proof cleanup, direct assignment, all source facts, 1024 frozen inputs, failure refusal, lost replies and privacy boundaries.

- daemon-suite: exit 0, 18.741s; top-level PASS 78, subtest PASS 175, skips 0.
- tagged-launch: exit 0, 11.14s; top-level PASS 16, subtest PASS 39, skips 0.
- launch-race: exit 0, 27.759s; nonverbose scoped race run, individual names not enumerated.
- tagged-vet: exit 0, 0.262s; static analysis, not a test count.

Evidence commands, exits, raw logs and owned source hashes are in verification/launch-injection/summary.json and adjacent files. No product UI changed, no frontend checks were added, no real providers/accounts were invoked, and no commits/root edits were made by this lane. Service-side snapshot persistence, authority/purge races and full HTTP-router integration remain the parallel coordinator's checks. Source is frozen for independent review and owned-only patch packaging.
