# Windows Node application resolution

## Root cause and scope

Run 45663530640 reached a successful production Web build, then Prepare isolated installed collaboration services failed: Start-Process FilePath cannot accept System.Object[]. The script projects Source from all Get-Command results without selecting one. Multiple PATH matches can therefore supply an array. Preserve the raw failure log at the parent-owned artifact path.

Select the first Application in PowerShell execution precedence order, validate a single absolute existing file path, and use that same scalar for launch and ownership records. Do not stringify/join paths, weaken cleanup identity, change production code, or alter workflow gates. The adjacent business script has no Get-Command projection; it launches its explicitly constructed serverExe, so it does not need this change.

Add a small local resolver function so the existing PowerShell fixture suite can extract and exercise the actual function through its existing AST pattern without starting services. Add two PATH fixtures plus missing-command coverage; keep existing installer checks intact. The fixture is already run by Validate acceptance scripts; existing local Node contracts remain unchanged. The behavioral regression lives only in the PowerShell fixture; no duplicate implementation-matching regex test is retained.

Reference: Microsoft Get-Command documentation, Example12: -All returns commands in execution precedence order. https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/get-command?view=powershell-7.5

## Implemented

- `windows-collaboration-server.ps1`: Get-NodeApplication selects one Application with explicit `-All | Select-Object -First 1`, fails on non-string/non-absolute/missing file paths, returns the scalar used by Start-Process and the existing web-private ownership record. Stop-OwnedWeb path/start-time comparison and taskkill tree cleanup are unchanged.
- `accept-windows-installer.test.ps1`: reuse its existing AST extraction pattern to run the actual resolver without starting services. Copy the runner's pwsh.exe into two temporary `node.exe` paths, prove two command matches, prove first-path selection, reverse PATH and prove the new first path wins, then prove a missing command throws CommandNotFoundException. Restore PATH in finally. Existing installer checks remain intact.
- No adjacent business-script change: its serverExe path comes from its own build, not an enumerated command lookup.

## Validation

- Existing Node suites `windows-collaboration-workflow.node-test.mjs`, `windows-candidate-version.test.mjs`, and `agent-critical-path-workflow.test.mjs`: 11 PASS, 0 FAIL, 0 SKIP.
- `git diff --check`: passed.
- PowerShell fixture not executed locally: pwsh is absent. It is already called by the Windows workflow's Validate acceptance scripts step and must pass on the next native candidate. This local result does not prove the actual Windows Web process started.
- A proposed duplicate regex test was removed during parent review; only the behavior-level PowerShell regression remains new.
- No commit, push, global Git changes, production-code edits, or heavy builds performed. Other agents' server edits were left untouched.
