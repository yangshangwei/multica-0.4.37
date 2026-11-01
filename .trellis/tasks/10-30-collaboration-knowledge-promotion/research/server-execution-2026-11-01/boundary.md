# C2 server execution boundary

Owner: knowledge_launch_leaf, 2026-11-01. Worktree: `mc-knowledge-promotion-20261101`. No commits, subagents or real Provider calls.

The present claim builder selects native sessions without managed-exposure proof; finalization does not record a source-free negotiation, and ordinary native pointer writers cannot bind exposure atomically. C2 owns the server-side protocol, retained metadata/exact-session history, independent prepare/authorize transactions, observation binding and terminal nonretryable classification.

Owned additions: `pkg/protocol/task_knowledge.go` and tests; `internal/service/task_knowledge*.go` and tests; `pkg/db/queries/task_knowledge.sql`; `internal/handler/daemon_knowledge.go` and tests. Narrow existing writer edits belong in service `task.go`, `task_cancellation.go`, handler `task_lifecycle.go`, `daemon.go`, `agent.go`, `daemon_rpc.go`, and existing claim query files when needed. Main additionally assigned canonical `pkg/taskfailure/failure.go` and related classify/tests for knowledge reasons. Router registration waits for main's sequential slot. A2 remains sole sqlc generator.

C1's five agent files are frozen. A2 owns human knowledge service/HTTP/resolver and F3 owns invalidation/hook/realtime changes; no edits to either lane. No migrations, UI/core or actual daemon/client/prompt wiring in this batch.

Use an exclusive C2 database; A2's connection environment is a template only. Tests must execute actual authenticated claim/prepare/authorize/pointer flows, including old clients, exact older/cancelled/NULL history, replay after authority change, both lock orders and nonretryable refusal. Preserve source-free claims and keep private knowledge out of stable context. G2 remains disabled until C3 and full acceptance.
