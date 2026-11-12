# Startup injection review — verified

No remaining blocking finding was found in the owned startup-injection delta. This approves this lane for coordinator integration, not the entire runtime task or model behavior.

## Scope and source

- Worktree: `/Volumes/artisan/code/2026/mc-handoff-launch-20261031`.
- Committed starting point: `4d0d1a76ddcbbf2ab985550d694a38263aaff25f`.
- Review base for borrowed files: exact `git_blob` entries in `research/launch-injection-baseline.json`; the broader uncommitted protocol/checkpoint/service work was not adopted as this lane's changes.
- Reviewed owned delta: `host_handoff.go`, `host_runner.go`, the two new launch test files, and the minimal tagged smoke-authorizer update. Also reviewed `collaboration-startup-handoff.md` and its server index link.
- No production source was edited by this reviewer. Requested local fixes were applied by the implementation owner to avoid concurrent writes.

## Findings (fixed)

1. `host_handoff.go`: fresh fallback originally minted another snapshot. The request now uses the versioned namespace UUID derived from the immutable run ID, excludes attempt/memory version, and rejects a replayed envelope that differs from the first.
2. `host_runner.go`: Codex could prepend continuity text after the prelaunch hash was bound. Negotiated startup owns truthful conditional guidance in the hashed input and clears the adapter's extra notice; the legacy path retains its prior behavior.
3. `host_handoff.go`: syntax-only DTO validation did not ensure the assignment matched the claim. Role/step/parent, memory availability/version/hash, and the linked complete admitted delegation are now checked; additional/unlinked admitted-assignment sources are rejected.
4. `host_handoff.go`: input IDs/decimals and binding requests now validate before HTTP; snapshot transport uses the handoff-specific bound. Retry backoff now uses `backoff << (i - 1)`, matching the established memory retry convention.
5. `host_agentintegration_test.go`: the old smoke whitelist rejected the new negotiated claim query. Exact supported keys/value are allowed only on managed-token claim POST; unknown, duplicate, malformed, misplaced and wrongly authenticated requests remain rejected.
6. Launch tests now compare the full process-visible envelope and all frozen claim inputs, including a 1,024-input case. They propagate privacy-scan file errors and verify the original unlaunched termination/proof path on failures.

## Verified behavior

- The actual first fake Provider input contains authority/current handler, original frozen inputs, information request/handoff note, exact delegation, prior-result uncertainty, source text, memory reference and explicit missing facts. The bounded metadata page does not replace executable claim inputs.
- Transport reply loss reuses the exact snapshot/binding request. Outer fresh fallback retains the snapshot but binds its changed continuity input to a new actual attempt. Memory reanchor cannot silently remint a snapshot or mix new memory with old authority.
- Unsupported claims make no new handoff requests. Wrong identity/version/role/step/parent, absent or incomplete delegation, inconsistent memory, refused snapshot and invalid/refused binding leave the Provider unlaunched.
- Binding runs after actual provider authorization and before burn/start, never for Git preparation. Each physical retry retains separate launch identity and original terminal/proof evidence.
- A test-owned Codex app-server captures real `turn/start` application text for live resume rejection and a two-process catalog retry. Each text hashes to its corresponding binding. The counterfactual Go overlay that removes only notice suppression fails on launch 1 / launch 2 hash mismatch respectively; the actual source is not mutated.
- No proactive handoff snapshot/source copy enters the brief or supervisor metadata/ledger in the controlled fixture. Existing Provider output and tool-record retention may contain quotations; this review does not extend purge coverage or claim a Provider cannot echo input.

## Verification

Independent reviewer commands in the isolated worktree:

| Check | Result |
| --- | --- |
| `gofmt -l` on all six owned Go files | Pass; no output |
| `go vet -tags=agentintegration ./internal/daemon/collaborationrun` | Pass, exit 0 |
| Focused tagged suite (exact command below) | Pass: 16 top-level tests + 39 subtests, zero failures/skips; package 8.750s, wall 9.170s |

```sh
MULTICA_RUN_REAL_AGENT_SMOKE=0 go test -tags=agentintegration ./internal/daemon/collaborationrun -run '^(TestHostHandoffLaunch|TestHostProviderAccountSmokeAuthorizes)' -count=1 -json
```

Go test compilation and tagged vet provide the relevant Go type/static checks. No frontend files changed in this lane; no redundant frontend check was run by this reviewer.

Implementation-owner evidence under `research/verification/launch-injection/` was also inspected: full daemon suite passed (18.741s), tagged focused suite passed (11.140s), focused race passed (27.759s), tagged vet passed. The retained counterfactual log fails specifically because actual Codex input differs from the bound hash, not because setup failed.

## Findings (not fixed) / boundaries

No known unfixed defect remains in the owned delta. The following are separate integration/verification responsibilities:

- Full server authority, snapshot-storage/purge races, bounded source recall, checkpoint record behavior and provider/account/native-platform acceptance remain with the coordinator and their owning lanes. Root router registration for `/handoff/snapshots` and `/handoff/bind` was observed to match this client; this review did not run a DB-backed server end-to-end case.
- The handoff fixture uses `RecordingVersion=0`. Code inspection confirms initial snapshot failures precede sender creation, and binding/fallback failures still reach `sender.finish()`, lifecycle seal, Stop and recovery. Active checkpoint-sender cancellation/backpressure with combined capabilities is not established by these handoff tests and must retain the checkpoint owner's tests.
- Real Provider calls were not made. The actual payload delivery evidence does not imply model comprehension, successful E1–E6 behavior, or broader privacy/purge guarantees.
- Borrowed prerequisites and this lane's delta must be integrated together without replacing ongoing root edits; the coordinator owns that step.

## Reviewed file hashes (SHA-256)

```json
{
  "server/internal/daemon/collaborationrun/host_handoff.go": "b742493bedbf9f8f2da7360c094289b61c413c38ee43e66863860a50ecc36ebe",
  "server/internal/daemon/collaborationrun/host_runner.go": "565d518d352199050f110e847f89558d1980b94ddc67d0c6db5a33eff6e6afa2",
  "server/internal/daemon/collaborationrun/host_handoff_launch_test.go": "16d9f62c8515de5468240bc85e8ddeddb0b186f3f99b62b1d23c2617f66b3336",
  "server/internal/daemon/collaborationrun/host_handoff_codex_launch_test.go": "1684ca013c6e1739efada2161851f0679c94ce88a031303f2740ae0c70e00a5c",
  "server/internal/daemon/collaborationrun/host_agentintegration_test.go": "cf3e07252dd44fef36f5d151891e27cda2440f52aca282baee1e876270c142cc",
  "server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go": "43cfadd19330a96310c7f6d4990d4183798e3e82017c24af6f30537b4284eecc",
  ".trellis/spec/server/collaboration-startup-handoff.md": "3b64e875f2e7f3ccfd8db577ed41a1a556372a4d376c8e52053d402dd6dfd04e"
}
```
