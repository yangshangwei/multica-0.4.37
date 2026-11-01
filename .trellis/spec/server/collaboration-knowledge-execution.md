# Knowledge preparation and physical launches

## 1. Scope / trigger

Apply when changing ordinary claim/resume/pointer writers, agent launch lifecycle,
or collaboration host consumption of approved knowledge. The
[candidate contract](./collaboration-knowledge.md) owns human authority and source
availability. Existing [host execution](./collaboration-host-execution.md) and
[startup handoff](./collaboration-startup-handoff.md) remain authoritative for
their permits, accepted inputs and termination; knowledge adds current source
authorization to those boundaries.

## 2. Signatures

| Surface | Implemented contract |
| --- | --- |
| Ordinary claim request | Runtime `TaskKnowledgeInvocation {protocol_version:1,provider,mode,builtin,configuration_id,extra_args}`; response `knowledge_context` is a source-free `TaskKnowledgeClaimDescriptor` |
| `POST /api/daemon/tasks/{taskId}/knowledge-context/prepare` | `TaskKnowledgePrepareRequest`: original request UUID, finalized claim generation/dispatched time, exact session binding, invocation fingerprint |
| `POST .../knowledge-context/authorize` | Adds preparation ID, manifest/prompt hashes and physical `attempt_seq`/`attempt_nonce` |
| `POST .../knowledge-context/observations` | Original claim/preparation/attempt plus started, native-session or terminated fact; canonical `{"accepted":true}` required |
| Start/pin/complete/fail | `knowledge_fence {claim_generation,dispatched_at}` |
| Collaboration managed claim | URL `knowledge_version=1`; source-free descriptor binds real run identity, configuration digest and run namespace |
| `POST /api/collaboration/runtime/runs/{runId}/knowledge-context/prepare` and `/authorize` | MDT-only, full `CollaborationRunIdentity`, original request UUID, frozen handoff ID and invocation fingerprint; authorize also binds manifest/prompt/permit sequence/nonce |
| `GET .../knowledge-context/status` | Termination `mct_` authority; identity, prepared-operation boolean and original request ID only |

Wire definitions: [`task_knowledge.go`](../../../server/pkg/protocol/task_knowledge.go)
and [`collaboration_knowledge_runtime.go`](../../../server/pkg/protocol/collaboration_knowledge_runtime.go).
Ordinary private endpoints require the current scoped `mdt_` daemon credential;
PAT/JWT/task-token fallback is not preparation authority. Collaboration business
`mcb_` tokens cannot call its preparation/authorization endpoints.

## 3. Contracts

### Startup and disclosure

`MULTICA_KNOWLEDGE_CONTEXT_ENABLED` is literal `true` opt-in, default false,
read at router construction into `handler.Config.KnowledgeContextEnabled` and
the router-owned `TaskService`. Collaboration helpers copy that same instance
gate. It is not a database feature flag or live global toggle. Gate closure
blocks new positive negotiation, private preparation and launch authorization;
retained-history checks, observations, fences, receipts and cleanup remain active.
See the [operator guide](../../../docs/collaboration-knowledge-runtime.md).

Single/batch HTTP, daemon WebSocket batch, managed claims and legacy fallback
share the claim builder. It finalizes only source-free configuration/session
metadata. Never disclose body, summary or source IDs during claim construction.
The configuration namespace is task-independent and stable across ordinary token
refresh. It is distinct from the **real invocation fingerprint**, computed only
after finalized task cwd, executable/version, model, thinking, default/custom
args, managed MCP bytes and settings path are assembled. Claude rechecks that
fingerprint against actual command options. Prompt/session/credentials/liveness
are separate bindings; this is not a hash of external host config or settings
file contents.

Preparation borrows `ResolveApprovedKnowledge` under an independent transaction,
then checks `ValidateKnowledgeResolutionLocks` before commit/disclosure. The
server reconstructs actual originator, runtime owner, every accepted input
author, current target and output audience. Accountable/display attribution,
installer identity or task prose never fills a missing principal. Workspace
output requires the source workspace; private Chat verifies its creator and
agent access; collaboration output checks its current audience. Approval alone
does not make cross-workspace output eligible.

User/credential → ordered workspace/member → project/independent target revision
→ consumer/Chat/source/candidate locks precede late runtime/installation/agent/task
NOWAIT locks. Collaboration uses its real command/grant/group-run NOWAIT fences,
not an ordinary task fabricated for reuse. Fresh resolution may omit unavailable
sources; exact preparation replay/resume manifest revalidation cannot substitute
today's set. Both manifest and contributions sort by candidate ID. Enforce all
32-item/256KiB decoded-material, 2MiB complete JSON (including newline), and 64KiB
retained-delivery limits before commit. No prefix truncation or base-body copies.

### Ordinary exact-session history and lifecycle

The selected binding is `(provider,runtime,agent,configuration,scope,generation,
native_session_id)`. Scope generation is a server UUID; claim generation is a
separate decimal value. Resume unions **every exposure in that exact session**,
including older, failed/cancelled/reclaimed turns and NULL-native rows. The 129-row
sentinel detects history beyond the 128-row bound; incomplete/corrupt history is
refused, never truncated. Valid claimed-only/empty history differs from actual
private exposure and from legacy NULL metadata.

Before returning any old `PriorSessionID`, unsupported/legacy clients also check
retained provider history across original/current runtime, current agent binding,
immutable daemon/provider and owner/provider identity. A scope/profile/owner
rewrite or a NULL current turn cannot erase older exposure. A known managed
session is not resumable by an unsupported client. A retained negotiated task or
any private exposure in that provider namespace refuses unsupported delivery,
even when the selected pointer is empty. Unsafe explicit selectors are refused;
base execution requires a complete relevant history proving no private exposure.
Proven legacy sessions preserve their existing behavior.

Use one `NewOrdinaryLaunchLifecycle(observer)` per logical input. It preserves
the already assembled ordinary environment; any callback `Environment` replacement
is refused. `BeforeLaunch` authorizes each actual physical Provider attempt,
including retries. Callback closures retain exact preparation/claim/attempt
identity. `NativeSessionObserved` synchronously sends raw system/result IDs before
lossy public messages or legacy fallback; the server atomically binds exposure
and pointer and the daemon requires positive canonical ACK.

Process start, native ID, termination, usage and business outcome are separate
facts. Missing native observations publish no pointer; a changed native ID is
retained as a mismatch and refused, never relabeled onto prior exposure. Start,
pin, complete and fail validate the original fence even after gate/negotiation
changes. HTTP supplies a fence argument even for legacy nil; only trusted server
transitions may omit it. Completion's failure reroute preserves it.

First launch refusal is terminal with zero processes. Later refusal retains prior
usage, messages and termination, then seals the lifecycle. All knowledge failure
reasons are excluded from automatic fresh fallback, requeue and input replay.
`Result.KnowledgeOutput` retains failure evidence separately from failed final
`Output`; both it and `KnowledgeFailure` have `json:"-"`. Deduplicate transcript
evidence and keep private input/Provider stderr out of generic runtime/server logs.
Use persisted task metadata to classify private server failures, not a claimed
client flag or only a `knowledge_*` error string.

### Collaboration host policy

Initial private support is **built-in local Claude only**. A custom profile using
the Claude protocol, remote runtime, launch prefix or unsupported options cannot
claim it. Configuration probes revalidate current host options before retained
work runs; a changed safe-selector/persistence contract does not inherit an old
admission.

Each admitted collaboration run, including an empty approved set, uses a unique
actual host directory/session key and neither reads nor saves reusable native
pointers. Every Claude print invocation includes the official
[`--no-session-persistence`](https://code.claude.com/docs/en/cli-reference.md)
**before fingerprinting**. The retained vendor excerpt/hash is in the
[C4 provenance record](../../tasks/10-30-collaboration-knowledge-promotion/research/collaboration-runtime-integration-2026-11-01/claude-session-persistence-reference.json).
A unique cwd alone is not
native-store isolation. Missing, valued, negated or reenable forms refuse; an old
CLI rejecting the flag has no flag-removal/base-only fallback. Ordinary resumable
knowledge intentionally refuses this flag; do not merge the two policies.

The preparation UUID is `uuid.NewSHA1(runID, []byte("multica:collaboration-knowledge:v1"))`.
Original replay reauthorizes the same manifest and frozen handoff. Knowledge
authorization runs inside each existing launch permit before local burn/start.
`NewLaunchLifecycle`, refreshed `mcb_` business credentials, `mct_` termination,
Stop/nonlaunch/proof recovery and record budgets keep their existing meanings.
Knowledge events retain metadata only and are excluded from public records,
checkpoints and handoff-source allowlists. Status remains available for recovery
after source/gate loss; prepared empty sets prevent it being a contribution-count
oracle. No body goes into a supervisor ledger or stable base/CLAUDE file.

## 4. Validation & error matrix

| Condition | Required result |
| --- | --- |
| Switch false / unsupported safe fresh runtime | No positive private negotiation; retain history guard and ordinary eligible base behavior |
| Unsupported/unsafe selector could recover managed history | Refuse before native launch; never silently strip the selector |
| Stale claim, session, preparation, manifest, prompt or invocation | `knowledge_context_changed`; no replacement snapshot or automatic replay |
| Current consumer/source/credential denied | `knowledge_authorization_denied` |
| Bound exceeded, corrupt proof, missing/ambiguous observation ACK or dependency failure | Fail closed (`knowledge_unavailable` or the precise typed refusal) |
| Lock contention exhausts bounded retries | `knowledge_busy`; terminal for the logical input |
| Missing lifecycle support/mandatory persistence | `knowledge_invocation_unsupported` or collaboration unsupported refusal |
| Source/gate loss after launch | Preserve started/terminated facts and original cleanup authority; refuse another launch |

## 5. Good / base / bad cases

- Good: resolve after claim finalization, disclose exact sources once authorized,
  reauthorize every process, and persist raw native observation before a pointer.
- Base: unexposed legacy work and a safe unsupported backend retain base execution.
- Bad: fingerprint provisional cwd, check only the latest successful turn, reuse
  an exposed session after toggling the gate, or treat process start as native ID.

## 6. Tests required

`pkg/agent/knowledge_launch_test.go` owns physical process/argv/environment/native
callback contracts. Service `task_knowledge*` suites own current locks, all-status
history, fences and cross-runtime downgrade checks; daemon `knowledge_context*`
suites own each claim transport and actual fake stdin. Collaboration service,
handler and `host_knowledge*` suites own MDT/status/retained-options/persistence.
`task_knowledge_config_test.go` owns exact startup parsing and instance propagation.

Tagged `internal/daemon/knowledge_server_integration_test.go` and
`cmd/server/collaboration_knowledge_host_composite_test.go` combine real server/DB
with fake Provider processes. Assert exact original-key loss/replay, canonical
ACK, purge/withdraw/expiry before disclosure/start, first/later refusal, source-free
logs, retained termination and unchanged base strings. Run database tests only
with the [explicit isolated database contract](./database-test-isolation.md).
Fake processes and source-bound passing suites do not establish real
Provider/version/OS acceptance; G2 and final integrated evidence remain explicit
[task gates](../../tasks/10-30-collaboration-knowledge-promotion/implement.md).

## 7. Wrong vs correct

Wrong: disable knowledge and deploy an older claim reader, or regard a private
manifest as permission to launch again.

Correct: keep guarded readers/history/outbox on rollback, recheck the exact
manifest at every disclosure and physical authorization, and preserve cleanup
facts independently of whether another launch is allowed.
