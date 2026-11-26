# Research: Actual account writer causes capability contention

- Query: Can same-account writer locking explain the specific capability503 class without weakening a fence?
- Scope: internal; actual current service, real PostgreSQL synthetic fixture, private Go overlay
- Date: 2026-11-26

## Findings

**Proved deterministic mechanism:** real auth.LockPasswordSession holding the authenticated runtime owner's user row causes actual RuntimeCapabilities to exhaust its unchanged three attempts with SQLSTATE55P03 at LockArtifactUsers, yielding typed503/collaboration_busy. Releasing that writer restores the exact original capability response. No source, retry, timeout, authority or database policy changed.

Fresh parent-owned rc_runtime_contention01; pinned Go1.26.11/CLI guard/p1/parallel2/GOMAXPROCS2; external HOME/TMP. One test PASS, packageexit0, no skips; test0.42s, command28.823s including compilation. HolderPID9188 and readerPID9190 differ; three failures are [55P03,55P03,55P03], context none, refusal129.435ms. These IDs describe this control only.

managed_runtime.go:129 sends managed writers through auth.LockPasswordSession; password.sql:16 takes user FOR UPDATE. Capability reads select lockManagedRuntimeAccountRead at :163 and LockArtifactUsers (agent_artifact_execution.sql:64) takes user FOR SHARE NOWAIT. Collision occurs before later installation-namespace/admission/runtime locks. Existing capability/R4 tests retain reader compatibility and writer exclusion; this control adds the exact capability refusal and SQLSTATE.

Ordinary task claim uses LockManagedRuntime (task.go:3450,3631,3808). Host fixture PollInterval is50ms (collaboration_host_router_test.go:301), and every collaboration dispatch checks EnsureCapabilities (collaboration.go:287). That is a concrete same-account overlap opportunity; it does not prove all33order03 or7memory02 refusals had this holder. Those runs captured code/context, not individual SQLSTATE/holders.

This is an intentional current-authority fence. Do not loosen locks, cache permission, extend deadlines or increase retries. Native capability requests really refused, then complete scenarios recovered. Neither all503-as-harmless nor historical-holder attribution follows.

## Files found / evidence

- runtime-capability-contention-control_test.go and overlay JSON: research-only actual query passthrough. No SQL arguments, credentials or raw error text logged; virtual product test path was never created.
- runtime-capability-contention-control-receipt.json, .jsonl, .stderr: exact argv, source/tool/configuration bindings and terminal results.
- server/internal/service/managed_runtime.go, collaboration_runtime.go, collaboration_runtime_preparation_capabilities.go: real transaction owners.
- server/internal/auth/password_session.go and server/pkg/db/queries/password.sql, agent_artifact_execution.sql: actual incompatible fences.
- Related specs: server/collaboration-host-execution.md and database-test-isolation.md. No new external reference needed.

## Caveats / Not Found

The holder is an actual lower-level authentication transaction deliberately held by this diagnostic, not an entire concurrently scheduled ordinary task request. This proves incompatibility through the actual capability service path. It does not identify the original service PR-table holder, each recorded busy holder, or a Docker implementation defect. Historical missing telemetry is not reconstructed by this new control.
