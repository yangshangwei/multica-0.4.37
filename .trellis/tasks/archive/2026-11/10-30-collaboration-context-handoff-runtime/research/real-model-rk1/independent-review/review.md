# Independent RK1 framework review

Verdict: **changes-required for paid execution**. Reviewed tip `4b508179f3e6197f170a99acd4df42d567e7ba1b`, including framework product `af04f6429` and locally merged runtime repairs `9e7f18785`. Product files remained read-only; only this report directory was written. No database command, migration, installed Agent CLI or Provider call ran.

## Required fixes

### RKI-1 — high: zero executed trials are recorded as a successful invocation

`run_rk1.py:317–345` relies on the executor exit code and the records that happen to exist. If the executor exits0 without starting a trial, the copy loop sees no directory, `stopped=[]`, and the invocation is saved with `exit_code=0`, `stop_state.triggered=false`, consumed0. The same scenario/trial can then be invoked again. This is reachable: `server/cmd/server/integration_test.go:49–59` exits0 when the selected database cannot connect, before `TestRK1RealProvider` runs. The launcher only checks the database URL name, not that the named trial actually ran.

A scratch-only direct probe exercised the production `main()` with its executor replaced by an exit0/no-output stub. `no-trial-probe.txt` records launcher exit0, zero public records, and a non-triggered stop state. No DB or Provider was involved. This violates the first-incomplete-environment stop rule and can misreport an unexecuted acceptance invocation as complete.

Required boundary: an invocation must prove its requested new trial(s) ran. Missing trial execution/public records, unexpected skipped/empty executor output, or a partial successful return must become an invocation-level incomplete failure even if no trial directory exists. Preserve the failed invocation so later scenario/trial starts refuse; do not create replacement samples. Add a launcher regression for exit0/no-trial and a partial/no-completion executor.

### RKI-2 — medium: effective Provider configuration can change without changing its fingerprint

`run_rk1.py:200–222` hashes fixed `HOME/.claude` paths and managed roots, but does not bind the inherited configuration-selection/environment inputs that production preserves. `CLAUDE_CONFIG_DIR`, `ANTHROPIC_BASE_URL`, provider-selection flags and account-profile selectors can differ while `operator_facts()` remains identical. `rk1ChildEnv` only strips DB/PG/RK1 keys; production `mergeEnv` explicitly preserves user configuration. Thus the launcher can approve the same registration under a changed configuration directory, model settings/endpoint or account-routing environment.

`config-fingerprint-probe.py/.txt` uses an empty scratch HOME, then adds an alternate config directory containing different settings plus endpoint/provider/profile overrides. The computed operator fingerprint stays byte-identical. Current review shell presence checks showed these specific overrides absent; the defect is that their absence is not frozen and changes would not be rejected.

Required boundary: hash the applicable nonsecret configuration-selector facts and selected configuration tree, or explicitly reject unsupported selectors before execution. Do not record credentials or raw config contents. Extend the launcher regression so changing a selected config root or supported routing selector invalidates preregistration. Actual account/usage proof remains a separate AC43 gate.

### RKI-3 — medium: raw Provider evidence persistence errors are discarded

`server/cmd/server/rk1_provider_test.go:115–118` ignores the stdout capture `Sync`/`Close` errors and the final `launch.json` write result, then returns the Provider exit code unchanged. A successful result still readable from the filesystem cache can be scored as pass even though required raw evidence was not durably published. The helper `rk1WriteFile` correctly returns joined write/sync/close errors; this caller throws those errors away.

This finding is direct control-flow inspection; no disk-fault injection was run. Keep the Provider's actual exit/result separate from evidence publication status, propagate or persist an evidence fault that makes the trial incomplete, retain launch accounting, and add a test-owned I/O failure seam/regression. Do not retry a real launch to repair evidence.

## Nonblocking notes

- **RKI-4 — exact format scoring.** H1 uses separate multiline regex searches and H2's multiline expression can match after extra leading text. The scratch regex probe demonstrates extra prose satisfying `format=true` despite prompts/documentation claiming exactly two/three lines. Semantics remain checked; either enforce the registered exact format or describe the scored format accurately before freezing the final framework.
- **RKI-5 — stale regression fingerprint.** `framework-registration.json.launcher_regression_sha256` differs from current `check_launcher.py`. Actual independent execution reports31 successful regression cases, while earlier notes say29. Framework/launcher hashes themselves match their registration. Refresh only the final registration/coverage metadata after required code changes; `runnable` must remainfalse until all final gates close.
- **RKI-6 — current fake proof is not passed.** Parent-owned `fake-proof/runtime-fixed-race.json` reports exit1,42 named passes,2 named failures and1 skip. Its E0a result is incomplete/`measured:handoff_or_recording_event_missing`; this review did not rerun or reinterpret that result. Diagnose it and rerun the corrected combined candidate on a newly allocated task DB. Historical normal proof does not substitute for this failure.

## Verified structure and coverage

- Scenarios use real service creation/consent/queue control, real router and run-scoped CLI, `Runner.Run` and `HostExecutor`; no fake authorizer replaces those layers. `handoff_version=1`/`recording_version=1`, memory/base, snapshot and binding hashes, actual stdin, physical launches, distinct permit identity, termination/proof/settlement, checkpoint and seal events are checked before scoring.
- Real runs keep seed runs fake; sim runs deliberately exercise real-wrapper passthrough with a test-owned CLI. Public launch counts plus private real launch records use the larger count. Budget/order tables agree:25 total, H2b afterH2, H6 two launches/trial; K1–K3/E0b are not admitted by this launcher. Cold ledger/workdir execution prevents native resume; native resume and different machines are not proven.
- H2 measures member result/wake/readers only; no claim of results/causes pages beyond32. H2b requires both actual handoff-source CLI exposure and Provider tool-result visibility for a correct answer to pass. H3a/H3b distinguish HTTP exposure from tool-result visibility, with preview truncation represented separately. H5 requires canary exposure and checks writes/retained content, using two fixture Multica users and one Provider account; it does not satisfy AC43. H6 remains manual review and records contamination/separate-directory criteria rather than self-awarding semantic pass.
- The runtime trace joins already-started HTTP handlers after Runner/Provider work stops. The persisted private ledger and scrubbed Runner error preserve missing telemetry; they do not invent an unsent seal or silently execute an extra recovery/Provider attempt.
- Source ID and clean product-root checks run both in launcher and Go entry. Provider binary and fixed configuration file hashes are checked, actual assistant model observations remain separate from requested/init model. Raw captures and local Provider sessions stay private; public trial summaries are scrubbed/bounded.

## Independent checks

| Check | Actual result |
| --- | --- |
| Existing scratch launcher regressions | exit0;31 OK,0 BAD; no Provider execution |
| Python AST parsing of launcher/regression script | exit0 |
| `go -C server vet -tags=agentintegration ./cmd/server` | exit0; compile/static only, no TestMain |
| Zero-trial executor probe | reproduced RKI-1; launcher exit0 without any trial |
| Alternate config selector probe | reproduced unchanged fingerprint for changed effective selectors |
| H1/H2 format regex probe | reproduced acceptance of extra prose |
| Current source binding after review | reviewed code hashes unchanged |

Exact command/log hashes, source hashes and findings are in `review.json` and per-check JSON files. No product patch or commit was made by this reviewer. A new independent check of the required fixes plus successful same-candidate fake proof is required before enabling paid execution.
