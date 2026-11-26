# Runtime lane requirements / parent handoff

Planning accepted in parent message; child task started. No DB tests needed for initial daemon/controller diagnostic controls.

Fresh frozen-source build inputs (from server module cwd):
- go test -c -o <bin>/host-daemon ./internal/daemon
- go build -o <bin>/multica ./cmd/multica (confirm CLI command path before build)
- go build -o <bin>/memory-provider ../e2e/fixtures/collaboration-memory-provider.go
- go build -o <bin>/multi-account-provider ../e2e/fixtures/collaboration-multi-account-provider.go
- Namespace Linux static daemon and provider similarly, but only after configuration admission.

Host requires E2E_PASSWORD_AUTH=1; MULTICA_DEPLOYMENT_ID; DATABASE_URL from runtime_api01 (API-only); COLLABORATION_E2E_API_URL=NEXT_PUBLIC_API_URL; production PLAYWRIGHT_BASE_URL and COLLABORATION_ELECTRON_RENDERER_URL; COLLABORATION_HOST_DAEMON, COLLABORATION_HOST_CLI, scenario-specific COLLABORATION_HOST_PROVIDER; COLLABORATION_HOST_DIAGNOSTICS=1. Native renderer build required by memory journey. Preserve original max12/min1 API pool for discriminating run; no API pool override inherited into Go tests.

Namespace finding: canonical e2e/fixtures/collaboration-cross-client-execution.ts:260 and historical cross_client_docker.py BOTH explicitly request --privileged, seccomp=unconfined, apparmor=unconfined on redis:7-alpine. This differs from backend default-policy EPERM. Runtime will preserve actual same-configuration identity and will not invent a weaker/default-policy substitute as root-cause evidence. Need parent adjudication of whether the original unchanged configuration is admitted by current no-security-weakening constraint; until then namespace real execution remains pending, not PASS.

Original concrete hashes are saved in artifacts/qa/main-acceptance-fixes-20261125/runtime/original-evidence.json; all six retained host/cross-client evidence hashes match prior review.

## Progress 18:14 local
- Frozen complete memory-runtime-01 passed 1/1 (28.6s), original 300s test/30s visit budget, max12/min1 API, explicit fixture providers, no retries. Receipt/source/build/gate/client cleanup in new runtime evidence. Historical failure unchanged; this pass does not reconstruct prior 503 cause.
- Need a fresh exclusive API DB for full order-export journey (do not reuse memory DB).
- Namespace exact fixture security configuration admission still pending parent adjudication; safe probe diagnostics compiled locally.

## Early review corrections
R1 now keeps read/no-response/malformed/helper-refusal observations separate from precleanup context cancellation; R2 stdin EOF vs read-failed event distinct. Updated regression controls underway. Need new frozen02 for final diagnostic binary because frozen01 remains retained; memory01 remains source-bound passing journey on prior diagnostic code. Parent please allocate order API DB and clarify original namespace container flags admission.

## Progress 18:26 local
Frozen01 full daemon race: 2052 named PASS, 2 existing platform SKIP (Linux batch / Windows drive-root), 0 FAIL. Full collaborationrun race: 379 PASS, 0 SKIP/FAIL. Vet passed. Fixture targeted typecheck passes with declaration generation disabled (runtime fixture typecheck); initial inherited declaration-only TS4058 retained separately. R1/R2 fixes have new frozen02 binaries built; full race on frozen02 running. No API services currently active. Awaiting order DB and namespace admission interpretation (same original privileged/unconfined flags, no global security changes).

## Required coordinator allocations
1. runtime_order01: full order-export on final02 source.
2. runtime_memory02: complete memory journey on final02 postreview diagnostics (prior frozen01 PASS retained).
3. Namespace: new API DB(s) for original cross-client/lane-close/lifecycle/offline-notifications after parent explicitly determines unchanged canonical privileged/unconfined container configuration is admissible. No such container has been started by this lane. The new probe cause cannot honestly be captured under a different Docker security configuration.

## Supported Go final03 progress
Complete order-export PASS1/1 (45.550s body,47.4s PW overall) and postreview memory PASS1/1 (39.159s body). Go1.26.11 frozen03; six runtime source hashes equal worktree. Ten actual permits all200/transport none/context none. Broader503s and poolpressure recur (order320/3, memory116/2); do not label availability root cause fixed. Both API clients0, all listeners/pids absent, original gates restored. Default-policy probe04 actual helper_start errno1/context none exit2; original privileged/unconfined policy never executed. Exact9 historical namespace identities recorded currentBLOCKED; historical causeUNKNOWN. Default probe setup failures01/02/03 retained/reconciled; all created containers/volumes removed. Normal complete daemon+collaborationrun exit0; final race still running, daemon already0. Final check/report imminent.

## Final lane completion
SupportedGo1.26.11 normal and race both: daemon2053PASS/2existingSKIP/0FAIL, collaborationrun383PASS/0SKIP/0FAIL. Complete order and postreview memory bothPASS, no retries/budget changes. Final report.md and final03/completion.json ready. Exact9 namespace policyBLOCKED, historicalcauseUNKNOWN; default-policy Probe distinct actual helper_start errno1/contextnone exit2. Six-file source-ready equals final03; no live lane processes/listeners/containers/volumes, both DB clients0 and gates restored. Parent independent R1/R2/integration review remains parent-owned; lane has no further permissible execution pending.
