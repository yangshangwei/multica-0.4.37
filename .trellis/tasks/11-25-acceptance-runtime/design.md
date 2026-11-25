# Runtime diagnosis boundary

## Current gap
Host facts retain HTTP status 0 without the transport/context cause. The fixture has no cross-process ordering for wait failure, stdin EOF, permit completion and daemon exit. Namespace startup suppresses the actual Probe error. These omissions prevent attribution; neither a provider bug nor shared Docker EPERM is established.

## Ownership
Expected changes: server/internal/daemon/collaboration_host_router_test.go (safe HTTP/timeline facts), its boundary tests, e2e/fixtures/collaboration-host-runtime.ts (monotonic parent events), server/internal/daemon/collaboration.go and collaborationrun/controller.go plus tests (safe structured probe cause if necessary). Existing cmd/server harnesses may support fresh controlled journeys. Do not edit handler/service, global limits, other client fixtures, timeout/retry policy, or Docker security. No speculative product behavior fix.

## Evidence → hypotheses → discriminating tests
D04 original runtime-supplement/memory-runtime-01 and order-export-01: claim200, permit client status0/server503, no authorized process, never-launched proof. H1: wait deadline triggers EOF/cancellation after slow API. H2: request deadline/transport/server failure precedes wait failure. H3: provider setup fault (contradicted by preauthorization ledger). Preserve request start/end monotonic times, bounded transport classification and context states; record wait failure/EOF/exit events and clock alignment in the same fresh run. Unit controls distinguish cancellation/deadline/network failure and status503 without injecting acceptance success. API pressure is correlation unless separately proven.

D05 original runtime/cross-client-01 and related lane-close/lifecycle/offline-notifications: managed registration succeeds, readiness refused. H1: executable/root configuration rejected. H2: helper clone/mount refused. H3: helper readiness/exit failure. Record safe stage/error-code/errno classification at the actual probe; use same image/platform/user/network/mount/capability/seccomp flags as the canonical fixture. Never reinterpret backend-linux network-none/cap-drop proof as this cause.

## Contracts
Diagnostics must be allowlisted metadata only; never stringify arbitrary transport error, URL, stderr or request body. Preserve errors.Is semantics and fail-closed admission. Local elapsed time is meaningful only within its clock domain; record a child ready anchor and parent receipt interval, not false exact cross-process ordering. Retain unknown intervals if clocks cannot resolve an order. Original test budgets remain unchanged.

## Rollback
Small uncommitted lane diff; retain failed runs/data. Stop/remove only exact newly owned processes/containers after verification; never delete original evidence or other agents changes.
