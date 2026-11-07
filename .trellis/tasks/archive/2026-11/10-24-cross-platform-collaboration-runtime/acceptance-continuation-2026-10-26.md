# Control and installed-client acceptance — 2026-10-26

The resumed acceptance slice is complete. The user chose to keep the existing task and records.

| Inventory | Final result | Provenance |
| --- | --- | --- |
| Identity | 31/31 original cases passed | Retained actual browser runs, independently reconciled; device login 1.369s |
| Collaboration | 58/58 original cases passed | 30 unchanged qualified-source records plus 28 fresh `1bfd169dc` originals |
| New collaboration executions | 28 final passes, zero automatic retries/skips/extras | 29 actual attempts: one initial Docker-125 prerequisite failure and its explicit successful rerun |
| Current directory package | Linux arm64 full Main/preload/renderer passed | Exact `1bfd169dc` source and three pinned artifact hashes |

The new cases cover Web/Electron partial Resume, scoped/whole Stop, unknown-proof barriers, real cached v0.6.7 CLI restrictions, Linux namespace independent execution/coalescing/lane controls/recovery, offline transfer/results, retained files, source/selection views, limited administration and ordinary host execution. Native host Web/Electron passed in 28.3s; Linux full Main controlling macOS host passed in 24.7s. The separate packaged Main/Web handoff passed in 43.0s.

All Provider executables are explicit test-owned fixtures. Linux namespace execution uses the actual Docker Linux kernel; host execution uses native Darwin arm64. The packaged client and Provider runtime OS are separate. These cases do not authenticate vendor accounts or prove macOS/Windows installers, all architectures, automatic updates or production rollout.

## Environment recovery

The single-API private password configuration lacked `MULTICA_PASSWORD_LIMITER_MODE=single`; completing that configuration restored startup. Proxy-free loopback readiness was verified. Docker denied the original `/private/tmp` bind paths before Main started. Same-HEAD worktree copies at `.omx/reports/collaboration-acceptance-current-20261026/` resolved the mount prerequisite; both copies' 12,950 tracked files and the three package hashes/sizes were verified. Original worktrees, snapshots, stash and failed result remain; old logical paths are symlinks to shared copies.

The existing 30-registration/five-minute limit was retained. Repairs stayed in private environment, readiness/ledger/relocation helpers and registry paths. The current accepted product source, assertions, timeout budgets and dependencies were preserved.

## Verification and retained records

The main session matched all 58 original file/project/title identities to raw expected-pass results and checked each new final selection has one execution without retries/skips. All 31 identity originals were independently reconciled. Source-specific history and hashes are in:

- [Collaboration originals](research/collaboration-acceptance-2026-10-26.json)
- [Identity originals](research/identity-acceptance-2026-10-26.json)
- [Current package](research/continuation-package-2026-10-26.json)
- [Final counts, scopes and cleanup](research/final-acceptance-2026-10-26.json)

Raw reports, commands/exits, original failure, screenshots and inline Main attachments remain under `/tmp/multica-specialized-acceptance-20261026/collaboration/`; exported Main attachments are in `final-evidence/`. Earlier full repository verification passed at snapshot `77e3f2b1` (288 browser passes, 117 environment-gated skips). That precedes the Switch metadata change, whose current service/handler race evidence is retained separately; it is not a new full check at `1bfd169dc`. This turn changed acceptance records/specs, not application code; no redundant full suite was run.

Cleanup passed with all isolated creation/execution/host gates false (epoch 4→5), ports 19180/14100/6274 closed and no running test-container prefixes. Databases, data, unknown barriers, raw evidence and worktrees were retained.

The task remains `in_progress` for native Windows, other-platform installer/delivery, authenticated Provider-account/quota and release acceptance. Production admission remains closed; no release was published.
