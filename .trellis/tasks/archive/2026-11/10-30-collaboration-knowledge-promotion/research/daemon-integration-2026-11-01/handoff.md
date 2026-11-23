# C3 ordinary daemon handoff

Implemented the ordinary task integration in the assigned worktree. Nine daemon code/test files changed; see `boundary.md` for ownership and behavior. No commits were made and the server production knowledge gate remains default false.

## Final verification

| Check | Evidence | Result |
| --- | --- | --- |
| Full daemon suite with installed-CLI guard | `daemon-suite-final.jsonl` / `daemon-suite-final-command.json` | 886 top-level tests passed; 1668 pass events including subtests; 2 platform skips; 75.663 seconds |
| Focused plus adjacent regression race suite, same guard | `daemon-focused-race-final.jsonl` / command JSON | 95 top-level tests passed; 298 pass events including subtests; no skips/failures; 35.499 seconds |
| New C3 tests | `test-inventory.json` | All 24 top-level tests executed and passed in both final suites |
| Build, vet, whitespace | `static-checks-final.json` / log | All passed |
| Source/dependency identity | `final-source-before.json`, `final-source-after.json` | All nine owned source hashes and four C1/C2 dependency hashes unchanged during final checks |
| Evidence artifact integrity | `artifact-sha256.json` | SHA-256 inventory |

The two full-suite skips are `TestCollaborationBatchPollerSlotsAndRunnerLinux` and `TestIsDriveRoot`, on this macOS host. No handler/service/router TestMain or database access was run. The guard observed no installed agent invocation; executable paths in the new tests are test-created absolute paths.

## Coverage and behavior

All ordinary prompt branches use the common BuildPrompt seam after actual cwd/settings/model/default/custom/MCP options exist. Capability/configuration IDs remain source-free and stable across task-specific options and transport-token refresh. HTTP single/batch, WS batch, legacy per-runtime fallback, and managed single/HTTP/WS claims execute the returned descriptor through real fake-executable stdin checks.

Preparation and every physical authorization carry exact original claim, manifest, prompt and invocation identities. Network/busy retries reuse identical serialized requests. Native system/result observations are synchronous and require accepted:true, precede pointer publication, retain native mismatch, and preserve unknown-fresh nonreuse. First refusal starts zero processes; later refusal retains usage, original process termination and deduplicated transcript evidence, without successful final output or automatic fresh replay. Cancellation accounting uses actual Started/Wait callbacks for negotiated tasks.

Private contributions never enter stable base/project files or generic stream logs. The existing workspace base context and task-scoped child token remain intact. Original claim fences are sent on StartTask and every terminal report (including zero-session refusal); pin accepts the fence for explicit callers, but negotiated execution uses the dedicated observation endpoint rather than asynchronous normal pinning.

## Remaining main-session gates

- C2 owns actual database authority/history/default-gate logic, unsafe selector/cross-scope history tightening, and combined API/database tests. C3 sent the concrete selector and shared-native-store review edges to C2.
- This suite uses fake HTTP/WS authorization transports; the combined real server/database plus daemon acceptance remains to be run by main in its authorized isolated environment.
- C4 collaboration-run integration is outside this slice. No collaborationrun or pkg/agent files changed.
- No real Provider CLI/model understanding or broader OS/backend support is claimed. Private knowledge remains built-in local Claude only when positively negotiated; production enablement remains main’s G2 decision.
