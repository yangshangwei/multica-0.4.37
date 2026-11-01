# Runtime logging P2 correction boundary

Main authorized a logging-only `agent.Config.PrivateInput` flag, Claude raw stderr sink suppression, ordinary and collaboration backend-construction wiring, and adjacent fake-executable tests. No shared logger mutation, generic logging framework, new dependencies, protocol/schema changes, or database access.

Files: `pkg/agent/agent.go`, `pkg/agent/claude.go`, `pkg/agent/private_stderr_test.go`, `internal/daemon/daemon.go`, `internal/daemon/knowledge_context_stderr_test.go`, `internal/daemon/collaborationrun/host_runner.go`, `internal/daemon/collaborationrun/host_knowledge_stderr_test.go` (all under `server/`). Main’s strict observation ACK and C4 execution changes are preserved.

Red proof: the same C3/C4 fake-executable tests echo actual approved stdin to stderr. All five success/provider-failure/knowledge-refusal cases exposed the private sentinel in the generic provider logger; ordinary knowledge refusal also lost bounded stderr evidence. Raw captured fixture logs are retained in `red-stderr.jsonl`, with command and source hashes alongside it.

Correction: only the raw stderr forwarding sink becomes `io.Discard` for positively negotiated private runs. The original logger still receives process/lifecycle metadata. The existing 2048-byte stderr tail remains available to authorized failure paths and uses existing common-secret/control-character sanitization. Knowledge refusal keeps the canonical content-free error and appends its bounded diagnostic to existing KnowledgeOutput failed transcript evidence; successful Output is unchanged. This is common-pattern sanitization, not a claim that arbitrary private source text can be made public; the diagnostic still belongs only in the authorized failure path.

No commits, real/installed Provider CLI, database operations, or subagents. Independent reviewer and main own the finding disposition and G2 acceptance.
