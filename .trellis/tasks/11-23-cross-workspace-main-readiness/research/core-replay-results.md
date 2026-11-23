# Core replay: five recorded failures closed on the bounded candidate

Candidate: `cb192014b2f27e9bb64112db52073cbb31ce8f04`. The latest executions of all **five distinct target cases passed**, with zero skips or automatic retries. Other root-session commits are not included; final acceptance must rebuild and rebind the refreshed candidate.

## Results

| Run | Actual result | Interpretation |
| --- | --- | --- |
| `localized-red` | 0 passed / 2 timed out, 126.97 s | Both original business cases reproduced the missing header-scoped New Squad link at line 82. |
| `localized-green` | 2 passed, 39.13 s | The exact localized link and intended create-route href work in empty welcome and populated header, preserving locale/reuse/collision assertions. |
| `controls-original` | 2 passed, 78.55 s | Untouched live revoke and terminal reopen originals; actual runs GET/claim/permit each returned 200 under the namespace gate. |
| `s1-red` | 0 tests; setup failure | Anchored grep did not match the full Playwright title. Retained separately and never counted as an executed case. |
| `s1-red-original` | 1 failed, 247.33 s | Comment POST exceeded its unchanged 20-second response wait during Docker/resource pressure, before the knowledge disclosure step. |
| `s1-red-isolated` | 1 passed, 34.40 s | Fresh database after Docker API recovery; unchanged original S1 assertions and disclosure helper, actual fixture daemon/CLI/Provider and knowledge reuse. |

The latest count is 2 + 2 + 1 = **5/5**. Failed attempts are retained and do not add distinct test identities. Detailed source hashes, gates and receipts are indexed by `core-replay-current.json`.

## Fix and scope

Only `e2e/localized-template-defaults.spec.ts` changed: remove the obsolete header container requirement and intersect the exact localized creation-link name with `/<slug>/squads/new`. The welcome page intentionally omits the header button. Both tests still assert the actual creation API language and omission of an explicit name, localized role names, preservation/reuse of edited agents, collision atomicity and successful English recovery. The product was not changed.

The historical S1 duplicate-name failure was **not reproduced** in the successful current scenario. The statically ambiguous attention-state branch remains a potential fixture limitation; no speculative `.first()` or disclosure-helper change was made.

The control fixture and server were not edited. Current browser-start gate was explicitly `enabled=true, creation_enabled=true, host_execution_enabled=false`; both formerly failing runtime list requests then claim/permit succeeded. The retained historical actual browser-start receipt had host enabled, which is incompatible with this synthetic namespace-only fixture. Current gate booleans were restored after execution with a monotonic epoch.

## Verification and provenance

- Production Web build `_7zjZ-yIB7lbnEWws5Osv` and Desktop renderer were built in the private worktree.
- API, migrations, CLI, daemon test binary and deterministic host Provider were built with Go 1.26.6. Host fixture compilation used `go test -c`; no normal agent test or installed CLI was invoked.
- Node 22 / Playwright original tests ran one worker, zero retries, on loopback production targets and fresh databases.
- Per-run receipts bind actual API health commit, build ID, fixture and binary SHA-256, runtime origins, listener process groups and gate state.
- Repository-base ESLint passed for the changed spec; Playwright test collection finds the intended originals, and owned diff whitespace check passes.
- The API binary predates the parent's four regenerated embedded-documentation JSON files. This report binds that original API binary and does not claim the regenerated documentation was exercised. The parent must rebuild API for the final refreshed candidate.

## Environment and cleanup

Raw logs, traces and reports are retained privately under:

`.omx/private/qa-work/main-readiness-20261123/core/{localized-red,localized-green,controls-original,s1-red,s1-red-original,s1-red-isolated}/`

All completed attempts stopped their owned API/Web/renderer and recorded closed ports. Each mode restored its initial gate booleans. The UUID-labelled PostgreSQL cluster remains running, intentionally handed to the parent for broader fresh validation; credentials remain only in mode-0600 `database.private.json`. No test data was inserted into a normal user database.

The initial local `pgvector/pgvector:pg17` tag unexpectedly resolved to amd64. Its migrations succeeded, but the empty task-owned container was stopped before any browser execution. All counted runs use cached arm64 image `sha256:215ae4d47393a980e9ef7ff7760f7dad53ec3e84bceb9fbd45cef34e50b5ba0f`. Both allocations remain documented.

The optional fixed profiling address 127.0.0.1:6060 was occupied by an existing unrelated process. The API logged profiling unavailable and served its owned functional port normally; that process was untouched.

During the failed S1 attempt, bounded `docker stats` and `docker inspect` each timed out after ten seconds, and `vm_stat` showed heavy compression/swap with other-session builds running. This establishes resource/Docker API starvation, not a product lock. Parent paused its own TS work, Docker inspect recovered to 14 ms, and the fresh original S1 passed without deadline/assertion changes. The unsuccessful diagnostic queries did not establish a database wait matrix and are not presented as one.

## Parent handoff

- Source can advance now: no application processes from this lane are running.
- Retained harness `core/core-run.py` accepts `run --mode legacy|control|host --label <unique> [--native] [--grep <pattern>] <specs...>` and creates a fresh database for each label.
- Build new candidate API with `-X main.commit=<new SHA>`; rebuild Web/Desktop when UI changes. Keep original reports immutable.
- `core-run.py cleanup` validates exact container ID/owner and zero other client connections before stopping only this cluster.
- Broader suites, final refreshed-source checks and merge recommendation remain parent-owned.
