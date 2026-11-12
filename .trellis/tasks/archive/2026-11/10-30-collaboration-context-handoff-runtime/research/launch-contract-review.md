# Research: startup handoff injection contracts

- Query: Complete startup injection and verification while a parallel lane owns the handoff service/protocol, without changing checkpoint/recall implementation or authority.
- Scope: internal, read-only code research; no providers, tests, git operations or source edits.
- Date: 2026-10-31
- Sources: existing host code in `/Volumes/artisan/code/2026/mc-handoff-launch-20261031`; live parallel protocol/service work in `/Volumes/artisan/code/2026/multica-0.4.37`. The parallel work is unfinished and changed during this read, so symbol ownership is more durable than its temporary filenames/line positions.

## Findings

### 1. Negotiation and HTTP interface

The parallel client already appends `?handoff_version=1&recording_version=1` to host claims (`server/internal/daemon/collaborationrun/client.go:97`). The server validates each requested version and puts `HandoffVersion` / `RecordingVersion` on the response only for requested host extensions (`server/internal/handler/collaboration_runtime.go:171`). This avoids new strict request-body fields and resource consent/digest changes; the response fields are declared in `server/pkg/protocol/collaboration.go:75`.

- `HandoffVersion == 0`: legacy/unsupported path; do not attempt new snapshot/binding calls or claim that reliable handoff is active.
- Supported version: snapshot/read/binding errors are errors, not evidence of an older server. Do not reinterpret 401/403/404 or malformed JSON as permission to start without handoff.
- Unknown nonzero versions already fail `validateClaimResponse` (`client.go:109`). Retained claims use the same validation through `validateHostLedgerRecord` (`host_ledger.go:41`).

At the last route inspection, no handoff snapshot or binding route was registered in `server/cmd/server/router.go`. The existing method/DTO names are:

| Operation | Existing service and wire shape |
| --- | --- |
| Freeze/retrieve snapshot | `RuntimeHandoffSnapshot(ctx, runID, token, CollaborationHandoffSnapshotRequest)`; request fields `request_id`, `memory_version`; response is `CollaborationHandoffEnvelope` with mode `snapshot` and `snapshot_id == request_id` |
| Bind to actual launch | `BindRuntimeHandoff(ctx, runID, token, CollaborationHandoffBinding)`; request fields `snapshot_id`, `prompt_sha256`, `attempt_seq`, `attempt_nonce` |
| Binding receipt | `protocol_version`, `issue_id`, `run_id`, `snapshot_id`, `prompt_sha256`, `attempt_seq`, `attempt_nonce`, `accepted` |

Main's proposed suffixes are `POST handoff/snapshots` and `POST handoff/bindings` below the existing run URL. These are an integration agreement to confirm with the service owner, not registered endpoints observed during this research. Exact route/handler registration must be verified before claiming a real-server closed loop.

The shared HTTP client uses an exact bearer, bounded response, `decodeStrict`, no ordinary credential fallback (`client.go:270`). Use the handoff-specific 256 KiB response limit instead of silently relying on its general 2 MiB limit. Cross-check response issue/run/snapshot identity and every echoed binding field; `accepted` must be true.

### 2. Where snapshot and binding belong

Existing order (`host_runner.go:95–179`): first permit → preparation → memory base → brief/environment/backend → host prompt → `Backend.Execute`.

Fetch the snapshot after preparation and successful memory-base handling. Keep the resulting handoff text in memory for prompt assembly. Fixed guidance may enter the brief; source text and memory must not enter the brief or local ledger (`host_memory.go:53`).

The actual provider attempt exists only after `hostSupervisor.authorize` inside `BeforeLaunch` (`host_runner.go:292`). Read its current token/attempt/nonce, bind the current snapshot and prompt hash, then permit `burnHost` and native start. Do not bind Git preparation/helper launches; their nonce is not the provider's nonce.

`LaunchLifecycle` calls `BeforeLaunch` for each physical process and rechecks cancellation/seal after it returns (`server/pkg/agent/launch_lifecycle.go:308–334`). Network binding must not hold a lock that prevents Stop. A binding is prepared-delivery metadata; only `Started` and subsequent terminal observations prove a process actually existed. Denied/failed binding must still converge through the existing original permit/termination/proof recovery path, without inventing a started process.

### 3. Exact prompt hash has a Codex-specific trap

`agent.LaunchInfo` contains only `Provider` and `Kind` (`launch_lifecycle.go:41`). The observer does not receive actual prompt text or the eventual RPC input.

Codex can modify input **after** process startup: `codexTurnInput(prompt, ResumeExpected, resumed, ResumeContinuityNotice)` prepends the supplied notice if live resume fails (`server/pkg/agent/codex.go:1519–1532`, `:1874–1879`). Its internal catalog retry can clear `ResumeSessionID` while keeping `ResumeExpected` (`:978–992`). Thus hashing only the original host prompt can disagree with the actual `turn/start` text.

Recommended bounded host-only approach for negotiated handoff: place suitable continuity/authority guidance in the already-hashed host input and set `ResumeContinuityNotice` to empty. The backend explicitly documents that empty means the caller already disclosed the issue (`codex.go:1871`). Preserve the legacy path's established behavior. Do not say a resumed session was lost before that is known; always-present guidance can instead require the supplied snapshot to govern this execution and prohibit assuming missing private context.

Validate both Claude stdin and Codex `turn/start` text against the bound hash. A Claude-only fake test will not catch this Codex transformation. If exact provider input cannot be held constant, the alternative is a truthfully named host-input hash plus later provider-input evidence; that would not satisfy the current design's exact-prelaunch-input promise without adjusting its contract.

### 4. Snapshot request identity and retry lifetime

`RuntimeHandoffSnapshot` stores one event under `(run_id, "handoff:" + request_id)` and compares both the full run identity and requested memory version on replay (`collaboration_handoff_snapshot.go:35–40`). It permits at most 32 snapshots per run, making accidental fresh IDs on HTTP retries both a drift risk and a resource leak.

Use one stable request ID for this logical run's handoff operation. Either persist a generated UUID before the first HTTP call, or derive a namespace UUID deterministically from the complete validated run identity and a versioned operation name. The UUID is an idempotency identity, not an authorization secret.

Do not include attempt sequence/nonce, current memory version or continuity-lost state in that derivation. Those can change across a retry while the authoritative handoff snapshot must remain the same. Uncertain HTTP results reuse the exact request ID and body. The single outer fresh-session fallback can change its fixed continuity text/hash while retaining the same snapshot ID; each new physical launch then gets a new binding under its actual nonce.

Local restart normally performs recovery rather than restarting an ambiguous host execution (`host_runner.go:29–39`). Only a retained session wait with zero permits resumes the start path. Do not add a new restart execution route merely to support handoff.

### 5. Memory purge/reanchor cannot silently refresh handoff

`RuntimeMemoryBase` normally replays the first version, but a purged base reanchors to the current unpurged version (`server/internal/service/collaboration_memory_runtime.go:80–123`). Snapshot request semantics distinguish:

- empty memory version: memory unsupported;
- `"0"`: supported empty document;
- positive decimal: that exact version.

The handoff service stores the requested memory version; same snapshot ID with a different version returns `handoff_snapshot_conflict`. Rehydration rejects a purged or hash-changed saved memory version, and binding repeats that revalidation (`collaboration_handoff_snapshot.go:89–96`, `:179–187`).

For this increment, fail closed when fresh fallback observes memory reanchor against the retained snapshot. Never silently generate another request ID, combine an old authority snapshot with new memory, or use a cached old body to bypass purge. Supporting a newly authorized logical prompt after purge would need an explicit additional contract and is outside this startup lane.

Before each launch, compare the snapshot memory version/availability/hash with the exact memory body included in the prompt. The server binding remains the final source/authority recheck even if the envelope is cached locally.

### 6. DTO semantics that host code must preserve

- Match `Assignment.Role`, `StepKey` and `ParentRunID` to the frozen `claim.HostContext`. DTO validation alone only verifies an allowed role/UUID shape.
- Keep the existing full `claim.Inputs` as executable inputs. Handoff input metadata is paged at 32 items; replacing the claim's up-to-1,024 inputs with that page would silently lose authorized work.
- The member's `Assignment.SourceID` should resolve to its admitted delegation source. A member with missing delegation must not infer that the group's full root request is now its task; main should decide the explicit fail/clarification policy before treating that case as accepted.
- Sources and missing/truncation flags remain typed data. Ordinary handoff/result text is `data_only`; only the linked admitted delegation is `admitted_assignment`. Neither can expand tool/business permissions.
- The inspected result DTO permits `FinalAnswerState` only `unknown` or `absent` (`server/pkg/protocol/collaboration_handoff.go:245`). Inject it as a provider/prior result, not a confirmed final answer.
- `Source.SHA256` identifies the original body, whereas visible source content is redacted and possibly truncated. Do **not** require `hash(Source.Content) == Source.SHA256`; valid redaction would fail that check. Memory hash, by contrast, can be matched against the separately returned memory body.
- At an early WIP read, the builder admitted a 64 KiB delegation and later applied a 256 KiB serialized-envelope cap. Worst JSON escaping can turn that into roughly 384 KiB. Server-lane tests must cover this and enforce serialized budgets without rejecting existing legal claim inputs. This is a current integration risk, not permission for a daemon downgrade.

### 7. Verification recommendations

1. Negotiated happy path: fake provider receives authority, handoff note/source, exact admitted delegation, prior results, memory reference and missing/truncation metadata without choosing `issue get` first.
2. Unsupported claim makes no new HTTP calls and preserves the old prompt path. Unknown version, wrong identity or malformed supported response prevents launch.
3. Lost snapshot ACK retries exact request body/ID; later source changes do not silently rebuild the snapshot.
4. Binding echoes every exact field and precedes process start. Denied/malformed/stale binding, transport cancellation and Stop during binding produce no provider start but preserve original terminal/proof recovery.
5. Git preparation plus provider startup uses distinct launch identities and binds only the provider.
6. Codex internal initialize/catalog retries and outer fresh fallback bind actual new nonces; captured provider input equals the bound prompt hash, including continuity handling.
7. Purge between snapshot and bind, and purge/reanchor during fresh fallback, refuse launch rather than mixing versions.
8. Redacted/truncated sources remain valid; near-legacy-limit claim inputs remain intact; no source body appears in brief or durable local ledger.

## Files found

| File | Purpose |
| --- | --- |
| `server/internal/daemon/collaborationrun/host_runner.go` | Startup sequence, provider options, outer fallback and actual-attempt observer |
| `server/internal/daemon/collaborationrun/host_memory.go` | Bounded memory-base retry and prompt-only memory representation |
| `server/internal/daemon/collaborationrun/host_ledger.go` | Retained claim validation, waiting/recovery and original launch facts |
| `server/internal/daemon/collaborationrun/client.go` | Claim extension negotiation and strict bounded HTTP parsing |
| `server/pkg/agent/launch_lifecycle.go` | Prestart observer, Stop race and native start gate |
| `server/pkg/agent/codex.go` | Internal retries and late continuity-notice transformation |
| `server/pkg/agent/claude.go` | Actual stdin delivery after managed process start |
| `server/pkg/protocol/collaboration.go` | Optional negotiated handoff version |
| `server/pkg/protocol/collaboration_handoff.go` | Typed envelope/request/binding DTOs and bounds |
| `server/internal/service/collaboration_handoff_snapshot.go` | Stable snapshot replay, source rehydration and per-attempt binding |
| `server/internal/service/collaboration_memory_runtime.go` | Memory base freeze and purge reanchor |
| `server/internal/handler/collaboration_runtime.go` | Claim query negotiation |
| `server/cmd/server/router.go` | Actual route registration; new handoff routes were not yet observed |

## Related specs and external references

Read `.trellis/workflow.md`, `.trellis/spec/server/collaboration-host-execution.md`, and the runtime task design. Relevant companion contracts are `.trellis/spec/server/collaboration-squads.md`, `collaboration-records.md` and `collaboration-runtime-cli.md`.

No external SDK/library research was needed. Findings concern the repository's current `agent.ExecOptions` / lifecycle contracts and unpublished handoff version 1. No new dependency is recommended.

## Caveats / Not Found

- No actual server route integration or provider execution was tested. This is research, not acceptance evidence.
- The main root is actively changing. An early handoff projection appeared temporarily in the existing human `collaboration_handoff.go`; that file later returned to its human workflow implementation. Do not copy or replace whole live service files from this report. Confirm the final projection owner/symbol and route names with the service lane.
- The startup lane cannot repair source storage/pagination/checkpoint defects by weakening client validation. Keep those ownership boundaries explicit.
- One initial combined spec read hit the transport-output guard; the workflow and bounded host spec were then read successfully through the CLI/file path. No state cleanup or runtime interruption was performed.
