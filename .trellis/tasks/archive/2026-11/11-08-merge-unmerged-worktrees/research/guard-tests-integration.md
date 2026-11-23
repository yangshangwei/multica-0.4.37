# Guard test source integration — 2026-11-08

Base: `e541cfb436fed9c6733174479718b651e2cf030d`. Product changes are limited to eight test/harness files in the integration worktree. Original source worktrees were read only.

## Behavior retained

- Knowledge: explicitly registered `full_continuation` mode validates the original stopped execution plus its single E0b retry, preserves both history ledgers, permits at most 15 new starts within total cap 17, rejects namespace/identity/hash/mode/account drift, and reports prior/new consumption separately. Original full/retry modes retain cap 15. All real execution remains under `agentintegration && !windows` and explicit `MULTICA_RUN_REAL_AGENT_SMOKE=1`; this merge authorizes no real run.
- Context: E5 canary evidence remains a finished behavior failure, persisted before the series stops. Runtime failures stay incomplete. Safe E5 and non-E5 failed behavior retain their original three-trial schedule. A test-created executable emits the fixture canary; installed CLIs are guarded.

## Sources

| Integration path | Source file | SHA256 |
| --- | --- | --- |
| `server/cmd/server/rk1k_codex_registration_test.go` | `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_registration_test.go` | `5becd7530091c30a8f39324c7b3fa30210302e7eb4ba196b1970899ab5dc6bb3` |
| `server/cmd/server/rk1k_codex_retry_test.go` | `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_retry_test.go` | `ac64694427ea4747f9965ec43d949d646e1edaf63377e91062408c23f97ddc1d` |
| `server/cmd/server/rk1k_scenarios_test.go` | `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_scenarios_test.go` | `f1bda40ac490a4580775e1175de2bf54ec5aa04b767f17195f263e6d10d3bd19` |
| `server/cmd/server/rk1k_codex_continuation_controls_test.go` | `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_continuation_controls_test.go` | `8ef3bfa2113bf7abd5c269432b4891e1cdb4177c07bf2fc72ac6a6c643c7cbf0` |
| `server/cmd/server/rk1k_codex_continuation_test.go` | `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/server/cmd/server/rk1k_codex_continuation_test.go` | `f8da971fc1fa3c10e6b06b478da474394a71f9829d339a63878a18ce2624017e` |
| `server/internal/daemon/collaborationrun/host_context_experiment_fixture_test.go` | `/Volumes/artisan/code/2026/mc-context-guard-after-20261102/server/internal/daemon/collaborationrun/host_context_experiment_fixture_test.go` | `0245a0719d82c3b9a2d1c754c7df27504b837776a4d04ac2bdb71864afa794d6` |
| `server/internal/daemon/collaborationrun/host_context_experiment_test.go` | `/Volumes/artisan/code/2026/mc-context-guard-after-20261102/server/internal/daemon/collaborationrun/host_context_experiment_test.go` | `09e5d6cc4e1abca7cb332e6c2b30b563351021fcaf730d8ee6445b2257fa6893` |
| `server/internal/daemon/collaborationrun/host_context_experiment_stop_test.go` | `/Volumes/artisan/code/2026/mc-context-guard-after-20261102/server/internal/daemon/collaborationrun/host_context_experiment_stop_test.go` | `2ddb69382caac52ef6afbaf3b6e4246a07d52aa3c06f0a6e0b1b5f8453a8908d` |

The initial integration matched each inspected source file exactly. The P2 output-namespace fix below intentionally diverges only the two new continuation files from those source snapshots; the other six files still match. Source hashes in the table identify the retained inputs, not the final repaired output.

## Verification

- `gofmt` and `git diff --check`: passed.
- Compile-only `go -C server test -tags=agentintegration -c -o <private>/cmd-server.test ./cmd/server`: passed; no TestMain/database execution.
- Guarded context suite `-run ^TestContextExperiment`: 24 PASS / 1 FAIL (no skips). All four new stop tests passed. The existing source-cleanliness test refused uncommitted Knowledge Go files before its fixture execution. Preserved original output: `guard-tests-integration/context-normal.jsonl`.
- The existing `contextSourceCommit` refusal is unchanged. The complete suite needs replay on the final committed integration SHA; an uncommitted run cannot prove that gate.
- Guarded context stop-condition race run: 4 PASS / 0 FAIL / 0 SKIP. See `guard-tests-integration/context-stop-race.jsonl`; no race reports or ambient Agent CLI invocations.
- Initial Knowledge deterministic controls before the review fix: NORMAL and RACE both PASS, each 8 top-level tests / 98 test and subtest passes, zero failures/skips/race reports/ambient CLI invocations. The coordinator permitted only `cmd/server` against `multica_merge_takeover_20261108_router_1`; an actual `SELECT current_database()` matched before each run. The environment file is external/private, credentials were not printed. Exact commands and source HEAD/status are recorded in `guard-tests-integration/knowledge-*.json`; raw output is in `knowledge-controls-{normal,race}.jsonl`. `MULTICA_RUN_REAL_AGENT_SMOKE=0` at command entry; all selected fixtures use test-created data/executables.

## Remaining validation

The implementation does not need a real-provider run. Deterministic Knowledge controls are complete. Replay context preflight on the fixed integration commit; the original uncommitted-source failure remains retained. Production-wide checks remain the coordinator’s final gate. The router database is released back to the coordinator; this agent did not create/migrate/drop it or change its allocation. The initial 98-case runs matched all eight inputs. Final repaired fingerprints are recorded below and in the final normal/race metadata.


## P2 follow-up: preserve historical output namespaces

The imported continuation guard compared output strings, so `prior/.`, symlink aliases, and ancestor/descendant overlap could reserve a new execution whose output overwrote retained evidence. A fresh output root also allowed its `provider` child to point into historical output because launch containment used lexical paths.

Only `rk1k_codex_continuation_test.go` and `rk1k_codex_continuation_controls_test.go` changed in this correction. The resolver follows existing ancestors through symlinks before appending a cleaned missing tail, rejects ambiguous parent traversal and unresolved/non-directory ancestors, and checks overlap with both original and retry output. Existing ancestor identity also catches filesystem case aliases. Launch containment uses the same effective path resolution. Original registered output/launch strings and history hashes are not rewritten; expiry, account/model, ledger, physical-budget and predecessor checks remain intact.

Evidence retained separately:

- `knowledge-alias-red.jsonl`: all 12 unsafe original/retry root namespace cases failed as expected before the fix; three distinct fresh targets passed.
- `knowledge-launch-alias-red.jsonl`: both nested symlink escape cases failed before physical launch containment was applied.
- `knowledge-alias-final-normal.jsonl` and `knowledge-alias-final-race.jsonl`: each **8 top-level suites / 119 PASS / 0 FAIL / 0 SKIP**, with no race or ambient CLI guard event. Case aliases were proven with `os.SameFile` on this filesystem; on a case-sensitive filesystem only those specific cases explicitly skip.
- Final runs used the permitted `multica_merge_takeover_20261108_router_1` database after identity verification and `MULTICA_RUN_REAL_AGENT_SMOKE=0`. The source HEAD was `7647d6eb853b0eb0d6fea3272e0eecd27012f578`; before/after source fingerprints match in both final metadata files. No source was edited during either final run.

Final file SHA256:

| File | SHA256 |
| --- | --- |
| `server/cmd/server/rk1k_codex_continuation_test.go` | `4cfa83adbe754ca12352ea9635601d534a4f91ef642dbedb19f47d7dad3a4080` |
| `server/cmd/server/rk1k_codex_continuation_controls_test.go` | `3cf5c0cc133b8ddb411f6e0c13d4c71168b3b8d7841d0b3d9ca28aecc6ee24db` |

No dependency, production runtime file, Git index or source worktree changed. Database returned to the coordinator after the final run. Final integration commit and full quality gate remain coordinator-owned.
