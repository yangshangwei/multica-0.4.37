# Windows collaboration failure diagnostics

## Evidence and change boundary

Run45668452215 retained the six source-bound binary/package hashes but exited1 after8.6s inside the Playwright subprocess. The driver discarded stderr and only kept generic public error; stdout JSON stayed private and no journey was copied. This proves subprocess failure, not which operation failed. No root cause claim can be made from duration alone.

The spec now has two full cases (OFF and separately opt-in default-ON), while the driver requires one pass/zero skips and its setup pins knowledge=false. Select the original full OFF testcase explicitly, keep all pass/count/skip/retry assertions; this fixes a separate reproducible selector mismatch, not the proven cause of this run's exit1. ON remains untested by this Windows lane.

Diagnostic plan: keep existing public multica-windows-collaboration-<phase>.json (already uploaded). Add only fixed driver stage, bounded stats/statuses, allowlisted error kinds/codes, and known source-relative file/line/column references. Private messages/stacks/expected/actual/snippets/stdout/stderr/test titles/attachment contents are never copied. If the failed result includes the owned installed-host-evidence attachment, project only fixed milestone-presence booleans. Store raw stdout+stderr privately under the phase directory, never add that directory to upload globs. Prove the public projection rejects synthetic secrets and unsupported paths/metadata using existing Node test entry.

## Implemented artifact contract

No workflow upload glob changed. The existing `windows-x64-acceptance-evidence` artifact already includes `multica-windows-collaboration-*.json`; the phase report now includes `diagnostic` on failure. Fields are fixed stage, numeric child exit, allowlisted process code/signal, parsed/unavailable report flag, bounded numeric counts/result statuses, allowlisted error kinds/codes, known source-relative file+line+column references, and fixed milestone-presence booleans. Attachment loading is constrained to that phase's real output path, ordinary files <=16MiB; rejected/missing attachment is a fixed status. No failure journey payload or screenshot is copied into the public report. Successful evidence is only assigned after all source, count, status and screenshot checks pass.

Both stdout and stderr are retained as mode0600 files inside the phase's private directory. Public output never serializes messages, raw stacks, expected/actual, snippets, arbitrary titles/file paths, command lines, environments or attachment bodies. Stack parsing projects only known repository source file coordinates; all other text stays private. These diagnostics establish failure position, not a new passing claim.

## Tests and current uncertainty

- Actual Playwright discovery with the locked CLI (`--list --reporter=json`) found2tests before the selector, exactly1original full OFF test after `--grep`. No test body executed during discovery. The known default-ON case is separately untested by this Windows lane.
- The Windows step's ten Node suites after this change:48PASS /0FAIL /0SKIP on macOS. New tests include synthetic credential/password in messages, stacks, expected/actual, attachments, stdout, stderr, unknown metadata and paths; only permitted diagnostics survive. Malformed nested reports fail closed and inventories are capped.
- `node --check` for driver/diagnostic helper and `git diff --check`: passed.
- No installed app/real Windows subprocess was executed locally. The 8.6s failure remains unresolved until the next run provides the safe report. Six package hashes prove it passed initial source/artifact checks; duration alone does not prove boot/registration/browser failure. Reviewed early Windows path/home/source bindings without finding a reproducible defect; no speculative product change made.
- PowerShell checks remain native-only on this host. No timeout increase, skip relaxation, retry enablement or E2E product-flow change. Parent review required before commit; no commit/push by this agent.

## Run45673226507: service lifetime evidence and scoped repair

The safe report locates the TypeError at controller `collaboration-installed-main.mjs:489`, fetching API/config before Electron launch. boot/runtime/login are false, transport is present. Full log records collaboration API PID5388 absent at cleanup; last normal pool log18:29:10, preparation completed18:29:19 and the next lifecycle step starts18:29:24. Business API PID8916 served its own step through18:30:36 and was also absent at later cleanup. Both PostgreSQL clusters stopped successfully. No panic or graceful-shutdown cause appears in retained tails.

Searched the business driver, installer, updater and service cleanup for basename/global kills. None can explain killing both business-server.exe processes: business cleanup only stops its managed daemon/app, updater (not reached) targets installed-directory executables, explicit service cleanup targets recorded PID and executable. Independent state paths and assigned endpoints are not overwritten on disk. This closes the specific broad-basename-kill hypothesis, not every possible external process terminator.

Observed lifetime failure aligns with the PowerShell step boundary. Microsoft Start-Process documentation states local Windows children normally outlive the caller, so the exact hosted-runner/redirect/process mechanism is **not proven**: https://learn.microsoft.com/powershell/module/Microsoft.PowerShell.Management/Start-Process . The repair removes this known unsafe cross-step dependency without claiming a determined OS root cause.

### Change

- Browser preparation stays in its prior step; actual collaboration API/PG/Web startup moves into the same PowerShell step that runs installed acceptance. Start collaboration first, then business to restore business-specific environment before its tests. No timeouts or acceptance assertions change. Independent always-cleanup remains.
- Record API process start time privately and set current-process collaboration state env as well as GITHUB_ENV.
- Before each full installed journey, `-Probe` verifies actual API/Web process presence, liveness, path/start ownership and source. A Node preflight then checks /health against saved PID/SHA, /healthz, and /api/config password/managed/deployment identity. It does not restart missing services or relax later assertions.
- Public report.services contains only boolean comparisons, HTTP status and allowlisted fetch error kinds/cause codes. No URL, process path, deployment ID, response body, environment or raw exception. A failed service gate reports stage service_probe; the existing artifact glob captures this report.

### Verification

- All ten Node acceptance suites:50PASS/0FAIL/0SKIP onmacOS. Tests use an actual loopback HTTP server, wrongPID control and stopped listener, and verify private fields never enter output. A stopped pooled socket can return ECONNRESET/UND_ERR_SOCKET instead of ECONNREFUSED; all are fixed transport codes and fail readiness.
- Existing PowerShell fixture now extracts the real probe and checks current process, foreign executable, wrong start and absent process. No local pwsh; native execution remains required.
- `node --check` for service probe and driver; `git diff --check`: pass.
- No build/dispatch/commit by this agent. Parent review pending. Windows process survival, full cooperation and final source-bound acceptance must be established by the next native run.

## Independent review: JSON timestamp identity

Review identified that PowerShell ConvertFrom-Json may materialize startedAt as DateTime, while both probe and Web cleanup compared it against a formatted string. Replaced that culture-sensitive comparison with one Test-ServiceStartTime helper: retain DateTime values directly, parse stored round-trip strings with invariant culture/RoundtripKind, reject unsupported/malformed values, and compare UTC ticks. Both probe and Stop-OwnedWeb use it; path and PID checks remain unchanged. No PowerShell7.5-only DateKind parameter is required.

The existing native PowerShell regression now passes owned records through actual ConvertTo-Json→ConvertFrom-Json, verifies correct identity, a timestamp differing by one tick, malformed time, foreign executable and absent process, plus the direct round-trip string case. Local related Node tests16PASS/0FAIL/0SKIP and diff-check pass; this host still lacks pwsh, so native PowerShell execution must occur in CI. No commit or dispatch by this agent.
