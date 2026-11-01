# C3 ordinary daemon implementation boundary

The existing ordinary daemon claimed tasks without knowledge capability metadata, constructed a base prompt, and used asynchronous first-only session pinning. This slice connects the finalized C2 descriptor and private preparation/authorization/observation contract to C1’s ordinary launch lifecycle.

Owned code files:

- `server/internal/daemon/client.go`: single/batch capability bodies, original claim fences on start/terminal/pin requests.
- `server/internal/daemon/types.go`: optional claim descriptor and internal terminal fence.
- `server/internal/daemon/wsrpc.go`: common batch request assembly for WS and HTTP fallback.
- `server/internal/daemon/managed_transport.go`: shared source-free invocation resolver in scoped clients.
- `server/internal/daemon/prompt.go`: private per-turn carrier after every existing prompt branch.
- `server/internal/daemon/daemon.go`: finalized options/fingerprint/preparation, one ordinary lifecycle, failure/evidence routing, guarded pin/replay, actual process cancellation accounting.
- `server/internal/daemon/knowledge_context.go`: stable runtime configuration namespace, bounded private HTTP transactions, strict full preparation validation, immutable observer and original receipt retries.
- `server/internal/daemon/knowledge_context_test.go` and `knowledge_context_claim_test.go`: test-owned executable and HTTP/WS/managed transport evidence.

No changes to agent adapters, protocol, service/handler/SQL/sqlc, frontend, or collaborationrun. No commits, database connections/writes, installed Provider CLI, or subagents were used.

C2 coordinated `TaskKnowledgeFence` after identifying the zero-session late-terminal/reclaimed-claim hole. C3 carries the original generation/time through StartTask and terminal reports, including refusals; C2 validates these under existing mutation locks. C3 also reported ambiguous selector/cross-scope history risks to C2; those server guards remain C2-owned.

Ordinary task credentials and independent workspace/project base context remain on their existing paths. Private contributions are appended only to final stdin. Knowledge process counts come from C1 actual Started/Wait callbacks, not a Backend.Execute call that might be refused before start. Generic stream text/error logging is suppressed for negotiated runs; task transcript evidence remains scoped to the ordinary task.

Known acceptance boundaries: all integration servers here are fake HTTP/WS transports. The combined real service/database + daemon flow, default-closed G2 enabling decision, C4 collaboration execution, real Provider behavior, and broader platform matrix remain main-session work. The fake executable suites prove application input and launch fencing, not model understanding. The POSIX executable tests are intentionally skipped on Windows.
