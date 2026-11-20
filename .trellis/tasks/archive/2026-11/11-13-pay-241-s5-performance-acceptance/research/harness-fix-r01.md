# S5 r01 harness heartbeat repair

Date: 2026-11-14. Scope: implement.md step 1.6. **Ready for independent check and r02 rehearsal; no S5/TC-24 acceptance claim.** The coordinator owns the new environment and live rehearsal. No API, database, model or actual daemon requests were made by this implementation/check run.

## Change boundary

The original host fixture registers a runtime but does not run the production daemon heartbeat loop. r01 ran long enough for the unchanged 150-second freshness fence to mark it offline. The four S5 harness files now own that missing lifetime behavior. Product runtime cleanup, freshness, connection pool, host fixture, Provider source and original r01 evidence remain unchanged. No dependency or Git writes were made.

| File | Final SHA-256 |
| --- | --- |
| `e2e/collaboration-performance.spec.ts` | `904857638743cccc59347880c63ba28ec151cbbc95e69dc32504301dc9651723` |
| `e2e/fixtures/collaboration-performance.ts` | `5f98915071bc4d358ea4eacec8aaf687a113bc859f14b0c98a8510c3a6a7d23a` |
| `e2e/fixtures/collaboration-performance-stats.ts` | `a728075a5d1c5d52ef13929dc3cda5c5427cc6cc1adb8f1874f823d0f2b3a73d` |
| `e2e/fixtures/collaboration-performance-stats.node-test.mjs` | `e903e85955708cafe56bcb4e9c82a17916078413fb47e4bcd52f98683bbe0cd1` |

`collaboration-performance.ts` adds `startRuntimeHeartbeat`, integrates its lifetime into `runWriteback`, records pre-trigger runtime observations, and settles cleanup on failures. `collaboration-performance-stats.ts` adds the fixed liveness constants and bounded runtime identity/status projection. The node test file adds eight named regressions; the spec comment describes the new teardown order. The two research files record this repair and its evidence.

## Runtime behavior

- After enrollment, one immediate heartbeat is awaited before daemon setup. Later beats run in the background, 15 seconds after the previous request completes, with a 10-second HTTP timeout. This is the production HTTP endpoint and payload (`POST /api/daemon/heartbeat`, `runtime_id`, `supports_batch_import:true`), using the enrolled owner-scoped `mdt_` credential. `principal_user_id` must equal actor 0. Human JWTs and readers never supply heartbeat authority.
- Every attempt appends `heartbeat_start` and `heartbeat_result` rows to `writeback-runs.jsonl`, with sequence, owner/runtime IDs, monotonic/UTC times, HTTP status and a bounded error code. No token, raw response, error text or business body is logged. Failure is sticky even if a subsequent beat succeeds; it prevents a passing overall run.
- Before each actual trigger, actor 0 reads `/api/runtimes?owner=me` with its own human token and execution workspace header. Exact runtime/owner/workspace/daemon identity, online status, `last_seen_at`, age and read times are logged before `triggerStart`, `t0` and `t1`. Offline/stale state is `runtime_offline`; malformed/unreadable state and identity mismatches have separate codes. Such a run is not triggered. A missing launch performs another owner read, retaining a transition-to-offline diagnosis rather than hiding it under `no_provider_launch`.
- Runtime heartbeats never await inside either measured interval. Their incidental background resource cost remains in the measurement; no latency is subtracted. The schedule is deliberately local to this one test runtime. It does not reproduce the production fleet's initial jitter or all pending-work handling.
- Cleanup closes sockets, lets the existing daemon close/settle, then cancels and joins the heartbeat before restoring the gate and closing logs. Failed socket/daemon cleanup does not skip later steps. The setup summary records heartbeat stopped time and failure. A failed initial beat or setup still emits every planned pair: 2 warmup rows plus 200 formal rows. Existing rows stay unchanged; only absent pairs are filled as NOT_RUN.

## Other lifetime audit

| Item | Existing lifetime / authority | Handling |
| --- | --- | --- |
| Runtime freshness | 150s (`RuntimeClaimFreshnessSeconds`) | 15s background beat + 10s request bound; pre-trigger online/read check |
| Managed daemon token | 24h (`managed_installation.go`) | Actual issued expiry recorded; expired or <=10s remaining refuses send; no renewal |
| Enrollment PAT | 1 day requested by `enrollHostRuntime` | Used only during binding before measurement |
| Human JWT | Default30 days; configurable `AUTH_TOKEN_TTL` | Each phase performs fresh100-account login; coordinator must retain actual r02 config; no silent replacement |
| Installation challenges | 2min | Consumed during enrollment/binding before measurement |
| Host test process context | **5min** (`runCollaborationHostRouterHarness`) | Unchanged; start request/ready timestamps recorded; `host_fixture_lifetime_elapsed` after >=5min since ready, without restarting |
| Provider callback | 150s per new run | Unchanged; fresh callback each run |
| Agent execution timeout | 180s default per run | Unchanged; metric cutoff remains30s each |
| Run business/termination capability | Exact run/fence/claim generation/attempt plus revocation/confirmation; no wall-clock TTL in selected rows | Existing production claim/permit path owns them; no invented lease or renewal |
| Runtime reservation | Released only on existing proven-end predicate | No timeout-based release or synthetic proof |
| Account WebSockets | Existing authentication, ACK and ping protocol | Native ping response and boundary-only reconnect unchanged |

Source symbols and paths for each entry are in `harness-fix-r01.json`. The 5-minute test-process context is a separate potential r02 limit. A heartbeat cannot extend it. A lifetime classification uses elapsed time since readiness, a conservative lower bound on process age; it does not claim to observe the exact process exit.

## Verification and retained failures

All logs are new files below `/Users/artisan/.multica/acceptance/pay241-20261114-resume/`.

| Check | Result | Evidence |
| --- | --- | --- |
| First red node run, before helper implementation | 20 existing pass, 5 new fail; exit1 | `s5-heartbeat-red-1.log` |
| Initial implementation node run | 25/25; exit0 | `s5-heartbeat-green-1.log` |
| Extended failure/cleanup tests | 28/28; exit0 | `s5-heartbeat-green-2.log` |
| First targeted TypeScript | exit0 | `s5-heartbeat-typecheck-1.log` |
| First ESLint | exit1, six test-loader Node globals missing from lint scope | `s5-heartbeat-eslint-1.log` |
| Final TypeScript + ESLint | Both exit0; source bytes unchanged during checks | `s5-heartbeat-static-final.json` |
| Final node regressions | **28/28 pass**, zero failed/skipped; exit0 | `s5-heartbeat-tests-final.log` |
| Playwright opt-out | **3 skipped, 0 executed**, exit0 | `s5-heartbeat-playwright-skipped.json` / `.log` |

The five red-first cases cover owner credential/payload and repeating cadence, wrong owner/expired credential refusal before transport, HTTP/malformed/network failures with sanitized logs, cancellation waiting for an in-flight request, and offline/stale identity classification with intact denominators. Three added named tests cover single-flight/sticky failure after retry, logging failure, and actual `runWriteback` orchestration under six fake scenarios: initial heartbeat403, offline, offline plus socket-close failure, offline plus daemon-close failure, offline plus gate-restore failure, and daemon startup failure. The last test checks no comment trigger, 202 retained rows, cleanup order, timer removal and no token disclosure.

Node uses the existing TypeScript transpiler to execute the actual harness in a restricted VM with explicit in-memory transport/daemon/DB doubles. Unexpected imports/network requests refuse. These tests never connect to a database, create a socket, or execute a Provider. Test-created evidence directories under `/tmp` are removed after each scenario.

The static config extends `packages/tsconfig/react-library.json` with the three explicit TS roots and Node types; lint reuses `packages/core/eslint.config.mjs` scoped to the four owned files. Effective lint rule counts are91/91/91/84; all effective configs and exact argv are retained. Final node command: `node --experimental-strip-types --test e2e/fixtures/collaboration-performance-stats.node-test.mjs`. Opt-out command sets `COLLABORATION_PERF_ACCEPTANCE=0` and directs Playwright output outside the repository. The runner emitted existing pnpm configuration and color-environment warnings; no suite ran.

## Remaining scope

The root session can now freeze these four hashes and run an independent check plus the new r02 rehearsal. Real owner authentication, sustained delivery past150s, retained100-run writeback results and actual gate/process teardown still need that live run. No product build, full repository check, production Web/Desktop or final candidate acceptance was performed here. S2–S4 dependencies, real human sign-off and all parent acceptance gaps remain open.
