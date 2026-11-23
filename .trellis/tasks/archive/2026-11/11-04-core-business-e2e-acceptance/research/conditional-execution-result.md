# Controlled Go conditional execution results

Candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`. No product source changed, no real model/account accessed, no database allocated or dropped by this agent. Root supplied all six database identities.

| Group | Actual result | Evidence |
| --- | --- | --- |
| Knowledge ordinary normal | 14 top /27 including subcases PASS, process exit0 | private conditional/run-1/knowledge-ordinary-normal |
| Knowledge Host normal | 9 top /17 including subcases PASS, process exit0 | private conditional/run-1/knowledge-host-normal |
| Knowledge ordinary race | 14 top /27 including subcases PASS; Go package terminal PASS | private conditional/run-1/knowledge-ordinary-race; launch-1/ended-for-isolation.json |
| Knowledge Host race | 9 top /17 including subcases PASS, process exit0 | evidence/conditional-run-2/summary.json |
| Actual Go triage wire export | 1 top PASS, process exit0 | evidence/conditional-run-2/summary.json |
| Actual Go JSON through current TS | 7/7 PASS via schema.parse plus production parseTriageResponse | private conditional/run-2/wire-typescript/results.json |
| Linux namespace daemon | 1 top PASS, no skip | evidence/linux-conditionals-2/summary.json |
| Linux namespace router/CLI | 6 top PASS, no skip | evidence/linux-conditionals-2/summary.json |
| Linux signal/stdin/root handling | 3 top +3 subcases PASS, no skip | evidence/linux-conditionals-2/summary.json |

Knowledge has23 unique top-level identities, each with normal/race passing events (46 executions). This is separate from the default package inventory. Linux adds actual execution of10 top-level identities; it proves controlled Linux test/runtime behavior, not desktop installation or real Provider support.

## Isolation change and retained limitations

Root found a different package could discover the parent Git repository when HOME/TMP lived under `.omx/private`. The conditional coordinator was SIGSTOP-paused immediately; its in-flight ordinary-race child completed naturally with Go PASS. After no descendants remained, only the paused coordinator was terminated to prevent its next row. Its outer guard exit code was not reaped by that coordinator; keep null, not invented exit0. No test process was killed.

The first three groups used repository-contained HOME/TMP. Their source/tool hashes matched and named tests passed, but this original environmental isolation limitation remains. Source review found ordinary fixtures explicitly use test-created fake Claude in managed task directories (not local worktree mode), and Host fixtures use test-created fake executable plus a HostExecutor directory with no Git bundle. These paths do not enter the Cursor Git-ancestor configuration branch. This is source-based scoping, not a full before/after snapshot proving every unrelated root file unchanged. The original root `.cursor/mcp.json` was left untouched; no attribution or speculative cleanup was made. Root directed that these passing groups not be repeated without actual contamination evidence.

`run-2` executes Host race and wire on their untouched root-allocated databases. HOME/TMP are under `/Volumes/artisan/e2e-private/core1a32-20261105/conditional/run-2`; each is checked by git rev-parse before use and must be outside Git. Existing Go cache paths remain explicit. Clean child env excludes deployment/model credentials, API pool overrides and task-root overrides. Every Go test is wrapped in the existing agent-CLI guard.

## Linux execution and first setup failure

Current Linux arm64 daemon and CLI bytes were reused only after hash comparison. Current router and agent test binaries plus a read-only DB probe were compiled with CGO_ENABLED=0, GOOS=linux, GOARCH=arm64, GOMAXPROCS2. Compilation itself does not count as Linux execution.

The first Docker DB probe failed with exit125 before startup because Docker could not bind the new Git-external binary directory. Retain private `conditional/linux-run-1` as an environment failure with zero test executions. No global Docker sharing settings changed. The same frozen binaries were copied to the already shared task-private `conditional/linux-bin` and mounted read-only in new `linux-run-2`; runtime HOME/TMP remained container-owned tmpfs, with executable `/tmp`. No recompilation or database reset was used to hide the failed preflight.

Actual image: cached Linux arm64 `sha256:db164dd5195d37ded1e6732eec4cc6b87decac66825be15fae9c7b87b2bac6c4`, no pull. Host and container independently checked the exact allocated database,752 migration ledger and initially empty issues. Router uses bridge only to the designated local PostgreSQL; daemon and agent cases use network none. Namespace suites explicitly require containment so unsupported isolation cannot become a successful skip. These containers execute only test binaries/fakes, not the installed application.

## Final audits

All six assigned databases have zero other connections after execution. All four owned coordinator processes and every `core1a32-linux-*` container have exited. Source/tool fingerprints and the three actual TypeScript schema consumer files remain identical. Raw logs, credentials and JSON fixture bodies stay in task-private directories. Structured safe summary: `research/conditional-execution-result.json`.

Performance, public GitHub skill import, missing pg_bigm and target Windows behavior were not executed in this slice. Default package lint/type checks were owned by the separate verification lane; no need to repeat them for private orchestration scripts. Private Python AST checks, gofmt and the actual seven-case Vitest consumer run passed.
