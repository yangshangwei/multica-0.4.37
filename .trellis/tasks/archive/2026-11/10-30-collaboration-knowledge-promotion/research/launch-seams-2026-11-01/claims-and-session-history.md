# Research: Ordinary claim and exact-session history seams

- Query: Map every first-party claim surface, trusted consumer/launch facts, actual prior-session choices, and cumulative knowledge exposure lookup needed for design §7/G2.
- Scope: internal, read-only production source; no tests, DB operations, Provider execution, or git operations.
- Date: 2026-11-01
- Checkout: `/Volumes/artisan/code/2026/mc-knowledge-promotion-20261101`. References describe the inspected working tree; A/B are concurrently implementing separate files.

## Findings

### One claim builder, multiple transports

| Entry | Actual code | Required G2 wiring |
| --- | --- | --- |
| Per-runtime HTTP | `server/cmd/server/router.go:1561`; `handler/daemon.go:3420` `ClaimTaskByRuntime` → `TaskService.ClaimTaskForRuntime` → `buildClaimedTaskResponse` at :3490 → `FinalizeTaskClaim` at :3549 | Advertise/parse optional knowledge capability; guard selected session even when capability absent; persist source-free descriptor under finalization CAS. |
| Machine batch HTTP and alias | router :1565–1566; `handler/daemon.go:1739` `ClaimTasksByRuntime` → `ClaimTasksForRuntimes` → same builder :1867 → same finalizer :1909 | Per-runtime effective capability, not a global provider boolean; each delivered row independently finalized. |
| WS batch / HTTP fallback | `handler/daemon_rpc.go:59,103,119` handles `tasks.claim` by constructing the request for `ClaimTasksByRuntime`; `daemon/wsrpc.go:323,350` selects transport | Carry capability through WS request and synthetic HTTP bridge; current bridge copies X-Client-Capabilities from connection identity at daemon_rpc.go:111–114, so new per-runtime invocation facts belong in the claim body; test actual WS-to-handler seam. |
| Legacy per-runtime fallback | `daemon/client.go:231` `ClaimTask`; :295 `ClaimTasks`; :328 `claimTasksLegacy` | Omitted response fields mean unsupported. New daemon against old server retains legacy behavior; old daemon against new server still receives only safe prior-session choices. |
| Managed installation transport | `daemon/managed_transport.go:177,242` records task/runtime/workspace ownership and calls scoped clients; `managed_wakeup.go:115,129` uses same transport selection | Register new task subresources with existing managed routing/auth allowlists; do not bypass workspace-scoped credentials. |

The shared builder is `server/internal/handler/daemon.go:2150`; do not create separate issue/Chat knowledge engines. Knowledge negotiation must not be conditional on `recordCommentReceipt`: `FinalizeTaskClaim` returns early for non-comment tasks at `service/task.go:3829` (near `if !recordCommentReceipt`). Persist negotiation before that return.

`FinalizeTaskClaim` (`service/task.go:3731`) currently locks password session, managed runtime, then task; checks task ID, runtime, issue, context, dispatched timestamp and claim generation; then locks issue execution and creates credentials/receipts (:3797 onward). Keep private source loading out of this transaction. Claim preparation and source authority cannot be inferred from the token's runtime-owner user ID.

### Actual session selection per surface

| Surface | Trusted scope/context | Prior session actually returned |
| --- | --- | --- |
| Issue assignment, mention/comment/coalescing | DB task agent/runtime/issue; issue workspace/project resolved by builder; task originator represents admitted human | `handler/daemon.go:2762` calls `GetLastTaskSession(agent_id, issue_id)` unless forced fresh. Returns session only when selected row runtime equals task runtime. Workdir is independently reused. |
| Explicit rerun | `task.RerunOfTaskID`, exact source task, `rerunSourceMatchesTaskScope` | :2729–2744 loads exact source, checks resume safety and runtime equality. It deliberately overrides the legacy `ForceFreshSession=true` safety flag; checking only that flag is wrong. |
| Private first-party Chat | `chat_session.ID`, immutable `CreatorID`, workspace/project, task's real originator and runtime owner | :2850–2865 first uses `chat_session.session_id/runtime_id`; then :2919–2948 can use `GetLastChatTaskSession` only when pointer/workdir missing (or channel generation scoped). A complete direct pointer can skip the task query entirely. |
| Channel Chat | Task-owned `channel_task_delivery`, not historical Chat binding; `task.ChannelContextRevision` | :2822–2838 resolves actual output audience. Scoped channel generations bypass the Chat-wide session pointer and use generation-filtered task rows. Unknown audience gets no private contribution. |
| Quick-create (including source-context retry) | Validated DB `task.Context` with `QuickCreateContextType`, workspace/project/requester; builder :3074 onward; task originator stamped in `service/task.go:1728` | No ordinary prior-session selector before issue creation. Use a fresh logical scope keyed by task/input identity; do not invent an issue ID. After completion `notifyQuickCreateCompleted` links the new issue via `LinkTaskToIssue` (:7548); retain original exposure scope in immutable metadata. |
| Autopilot run-only | Builder :3012 resolves run → autopilot workspace/project; `service/autopilot.go:1071–1095` distinguishes manual actor from immutable trigger creator | No ordinary prior-session selector here. Issue-creating autopilot execution subsequently follows issue path. Missing originator/unknown audience gives no private contribution, even if accountable human exists. |

`BuildPrompt` at `daemon/prompt.go:218` first calls `buildPromptBody`, then appends common per-turn blocks. Body branches at :238–247 select Chat, triggering comment, autopilot, quick-create, then ordinary issue. Append the managed source-qualified block in this common wrapper. Do not put it in `TaskContext`/`InjectRuntimeConfig`, project sidecars, skills, or stable runtime brief. This covers all five bodies without duplicate injection logic.

### Trusted identities versus observations

- Authorization human: persisted `AgentTaskQueue.OriginatorUserID`. `AccountableUserID`, `InitiatorUserID` display compatibility, `OriginatorSource`, and wire attribution labels are not substitute principals. Quick-create stamps requester as originator (`service/task.go:1704–1730`); Chat creation stamps actual initiator (`:2150–2164`); autopilot trigger creator logic is `service/autopilot.go:1071–1095`.
- Runtime owner and workspace: DB `AgentRuntime.OwnerID/WorkspaceID`, resolved by runtime-access gate; builder rechecks `agent.RuntimeID == task.RuntimeID` and private runtime owner gate (`handler/daemon.go:2220–2255`). Both runtime owner and originator need current source access; token user ID alone grants neither.
- Private Chat audience: immutable creator plus current private-agent access, mirroring `handler/chat.go:269–297`; task-owned external delivery snapshot must be absent. Do not infer private audience from Chat's original creation source.
- Server provider: current DB `runtime.Provider`, runtime mode, managed binding/epoch where applicable. Actual daemon execution selects registered `rt.Provider` (`daemon.go:5144`) then may substitute a custom executable and fixed args (`:6880–6916`). `Config.BuiltinRuntime` (`pkg/agent/agent.go:284` vicinity) distinguishes a true built-in binary from another implementation of the protocol family. Provider alone is not an effective-backend proof.
- Claim generation: `agent_task_queue.claim_generation` increments on claim and stale-dispatch reclaim (`queries/agent.sql:833,985,1059`). Managed reclaim can retain `dispatched_at`; timestamp alone cannot identify a claim. The ordinary response currently lacks a claim-generation field (`handler/agent.go` / `daemon/types.go`); the new descriptor must carry it.
- Knowledge scope generation is NOT the existing claim generation, task attempt, or channel context revision. It must be issued server-side for a fresh logical session and inherited only by an exact matching binding. No existing ordinary field establishes it today.

### Cumulative history query contract

The existing selectors choose a candidate session; they are NOT exposure queries. `GetLastTaskSession` (`queries/agent.sql:1182`, CTE at :1284) and `GetLastChatTaskSession` (`queries/chat.sql:1140`, CTE at :1181) use latest-per-session terminal rows and poison/retirement filters. They discard earlier rows and NULL native pointers, which is incorrect for managed knowledge proof.

Add a separate service/SQL query after final actual pointer selection, scoped by the selected native ID plus agent/scope. Return bounded history metadata across **all statuses**, not only the latest or completed task. Include rows matching either:

1. The persisted task native pointer; or
2. Any immutable `knowledge_delivery` original resume binding / observed native binding, including prior claim generations and unknown-result turns.

Then decode/validate and union the complete exposure set by candidate ID/version/hash. A relevant version=1 disclosed/possibly-fed history with missing/malformed metadata, inconsistent provider/runtime/configuration/scope/generation, or over-budget proof must refuse that session. Do not SQL-filter away a mismatched negotiated record and reinterpret the empty result as legacy. Errors are unavailable, not empty success.

Why the second arm matters: `CompleteAgentTask` can write `session_id=NULL` (`queries/agent.sql:1173`), while `UpdateChatSessionSession` preserves the direct pointer via COALESCE (`queries/chat.sql:298`); failure also preserves an earlier pin (:1410). Cancelled turns are valid history. A new task's NULL knowledge version says nothing about the selected session's older turns.

Current migration 747 adds the two JSON/version columns and a 64 KiB envelope check (`:86–92`); it does not implement this reader, lifecycle bindings, generations, or phase validation. The inspected `collaboration_knowledge.sql` contains candidate/target/invalidation queries, no delivery reader. Do not depend on one already existing.

Reclaim is an overwrite hazard too: finalizing a later claim generation on the SAME task must preserve earlier disclosed/possibly-fed resume bindings. An array or provably cumulative representation inside the bounded delivery document is acceptable; resetting the document to a new `claimed` object is not. Reject/retire on budget overflow; never truncate away proof.

## Files found / related specs

- `server/internal/handler/daemon.go` — shared claim builder and single/batch finalization.
- `server/internal/handler/daemon_rpc.go`; `server/internal/daemon/wsrpc.go` — WS bridge and fallback transport.
- `server/internal/service/task.go` — claim CAS, provenance, retry, completion/failure.
- `server/pkg/db/queries/agent.sql`, `chat.sql` — actual selectors and pointer persistence.
- `server/internal/daemon/prompt.go`, `types.go`, `client.go` — common prompt and wire seams.
- `.trellis/spec/server/security-boundaries.md` — automation originator and machine credential boundary.
- `.trellis/spec/server/managed-installations.md` — managed runtime/binding ownership.
- `.trellis/spec/server/collaboration-host-execution.md`, `collaboration-startup-handoff.md` — per-launch evidence versus application input.
- Formal `design.md` §6–7 and `implement.md` §4 remain authoritative; this is a seam handoff, not a replacement plan.

## Caveats / Not Found

- No G2 functionality was executed or verified; source inspection only. No product edits or DB writes.
- No current ordinary `KnowledgeSessionBinding`, preparation/authorize DTO, or delivery reader found. Candidate/contribution DTOs in `pkg/protocol/collaboration_knowledge.go` are usable building blocks, not those missing contracts.
- External references: none required for claim behavior; repository source is the primary reference.
