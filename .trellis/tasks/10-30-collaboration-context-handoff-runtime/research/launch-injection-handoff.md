# Launch injection handoff — READY

User scope: **补齐启动注入与验证**. This lane is complete within that scope and independently reviewed. Parent coordinator `01a1920d-a340-7dd3-9d25-9191aa128cb0` owns integration, parent status and commits. No code was applied to the main worktree by this lane.

## Deliverable

- Source worktree: `/Volumes/artisan/code/2026/mc-handoff-launch-20261031`.
- Initial HEAD: `4d0d1a76d`; borrowed live protocol/client/checkpoint/startup prerequisites have exact hashes/Git blobs in `launch-injection-baseline.json`.
- Patch: `launch-injection-owned.patch` (only 6 Go files + startup spec/index).
- Patch SHA-256: `40772c20f0848e48e96c4eb2e340c1d3ed5441ad1849177c29d933c8d7ea8927`.
- Full before/after hashes: `launch-injection-owned-files.json`.
- Applying the patch to the saved baseline reconstructs every owned file byte-for-byte. It does not contain borrowed protocol/client/checkpoint/service/router changes.
- No standalone source commit was made: the worktree contains another lane's uncommitted prerequisites. Integrate this owned delta into that lane's completed tree, then commit the coherent result.

## Changes

1. Keep one run-bound logical snapshot request across transport retries and outer fresh fallback. Replayed facts and memory must stay equal; purge/reanchor cannot silently remint context.
2. Validate claim role/step/parent, memory availability/version/hash, complete admitted member source and exact binding receipt. Reject extra/unlinked executable assignment labels. Preserve original source hashes when returned content is redacted.
3. Inject the typed handoff directly into actual startup input; retain frozen claim inputs, current handler/source/result uncertainty and missing markers. Members do not need a discretionary comment read to discover their assignment.
4. Bind after the actual Provider authorization and before burn/spawn; Git helpers do not bind. Codex continuity guidance is in the hashed application input, preventing adapter-internal resume/catalog fallback from changing text after binding.
5. Repair the formal smoke authorizer for the new claim query negotiation: only recognized version=1 keys on managed-token POST /claim; unknown, duplicate or misplaced queries remain rejected. This change is tested without running a real provider.
6. Preserve checkpoint hooks and existing original termination/proof cleanup. Do not proactively copy snapshot bodies into brief/supervisor metadata. Existing provider output retention is a separate boundary.

## Owned paths and base blobs

| Path | Base Git blob | Final Git blob |
| --- | --- | --- |
| `server/internal/daemon/collaborationrun/host_handoff.go` | `9de708756f863a2cbddbe9881e62e3dd64d8687a` | `93c818cfda6857c1d5a1a7e065d9aa2f48ec322c` |
| `server/internal/daemon/collaborationrun/host_runner.go` | `7efbe861901a62e839d2f8a1208c8486060c5342` | `ee183dd2e97350ec85198ba583edef0792e6a8ae` |
| `server/internal/daemon/collaborationrun/host_agentintegration_test.go` | `ae0c73838a83b0c103289f6b986fe6418aa72ef5` | `34e109f553ae2803ae90d479f67b0dab6e4da71f` |
| `server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go` | `be673557370120aa5944fc6de6e1b84376a3d10b` | `4debfa3fe35cbca2d48b2f271857aace0b8d25f4` |
| `server/internal/daemon/collaborationrun/host_handoff_launch_test.go` | `new` | `002d9e14a11f3e8efbe2dd486dd14fb7b373206a` |
| `server/internal/daemon/collaborationrun/host_handoff_codex_launch_test.go` | `new` | `0407fa1631c34dbfe06375d30c476459145acf08` |
| `.trellis/spec/server/index.md` | `54172d93a9f555c337dedf1a394d131e6b90b16b` | `9c57b14985ff99b0e02f784452554be17466a1e2` |
| `.trellis/spec/server/collaboration-startup-handoff.md` | `new` | `7cb2e9f18b1586d362e0171daa8f80344bd5bb55` |

Use the base blob when reconciling a shared file with newer checkpoint work. Do not replace entire shared files. New source files in this patch are test/spec files; `host_handoff.go` is a modification of the borrowed implementation even though it was untracked in the initial repository HEAD.

## Verification

| Check | Result |
| --- | --- |
| Complete collaborationrun suite | 78 top-level + 175 subtests passed; 0 skips; 18.741s |
| Tagged launch + formal-authorizer suite | 16 top-level + 39 subtests passed; 0 skips; 11.140s |
| Launch race suite | Exit 0; 27.759s; nonverbose, no invented test count |
| Tagged go vet | Exit 0; 0.262s |
| Independent checker tagged suite | 16 top-level + 39 subtests passed; 0 failures/skips; 8.750s |
| Independent formatting/static review | Passed |
| Codex counterfactual | Removing only negotiated continuity suppression reproduces both actual-input hash failures |
| Patch reconstruction | All 8 files match final SHA-256 |

Raw logs, exact commands, timestamps and source hashes: `verification/launch-injection/` (including `summary.json`). Review: `launch-injection-check.md`. Implementation/regression notes: `launch-injection-implementation.md`.

Coverage includes actual Claude-compatible process stdin; actual fake Codex `turn/start` after same-process resume rejection and two-process catalog retry; 1,024 frozen inputs; stable snapshots/fallback; mismatched, missing, refused and lost-reply snapshots/bindings; memory unsupported/empty/purge distinctions; correlated member source; redacted content; separate Git helper launches; no proactive body duplication; and retained unlaunched proof.

## Integration checks and remaining responsibilities

- Reconcile against the latest runtime integration tree and rerun the named Go suites there. The isolated results are not a claim that the coordinator's eventual final tree passed.
- Current server routes are `POST .../handoff/snapshots` and `POST .../handoff/bind`, matching this client. Claim capability negotiation is borrowed and not changed by this patch.
- Combined active checkpoint sender shutdown is covered by the checkpoint owner. Handoff failure tests use RecordingVersion=0; static review confirms binding/fallback failures still reach sender.finish/Seal/Stop/recoverHost when a sender exists.
- Server snapshot persistence, authority/purge races and maximum serialized delegation budgets remain server-lane checks. A 64 KiB assignment with worst-case JSON escaping can exceed the current 256 KiB envelope; coordinator has forwarded this to that owner. This client rejects incomplete assignment instead of treating it as permission to execute the whole root request.
- Real provider/model behavior, paid experiments and native product rollout are not performed or claimed in this coding lane.
- Parent and runtime tasks must not be marked wholly complete from this slice alone.

Direct patch check against the concurrently changing main worktree at packaging time: passed. The parent requested integration in its own worktree, so this lane performed no main application.
