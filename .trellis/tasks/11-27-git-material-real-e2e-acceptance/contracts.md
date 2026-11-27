# Runtime material contract v1 — approved and frozen

The user-selected runtime performs all mutations; the daemon only prepares, launches, observes read-only facts and transports receipts. Server has no material Git credential or remote Git API path.

## Public settings

Existing project material GET/PUT settings routes remain. Envelope adds `execution_protocol: 1`. Settings: `{version, enabled, provider, resource_id, repository, base_branch, root_path, reviewer_ids, execution_runtime_id: UUID|null, auto_start_plan}`. Input replaces version with `request_id, expected_version`. Enabled configuration requires an authorized explicit runtime. No `connection_id` or `plan_agent_id` input. Old response missing protocol/runtime is displayable but cannot enable a new runtime write; old pending input is not transformed/replayed.

Detail adds optional `operation: {id,kind,status,task_id,execution_runtime_id,blocker}` from the persisted current attempt. Operation status is not material merge status. Existing material/revision/decision/baseline/plan projections and manual start semantics remain.

## Operation descriptor

Canonical Go wire types belong in `server/pkg/protocol/material.go`; domain validation belongs in `internal/material`, reused at API boundaries. `MaterialExecution` version1 contains operation/job/material/revision/project/workspace IDs; action (`publish|review_sync|merge|observe|verify_baseline`); original operation key; exact resource/repository/instance/base/source branch; config version; approved revision/decision/head as applicable; expected file paths, regular modes, full immutable contents/digests; selected runtime/daemon/profile/config fingerprint; attempt/task identity and expected descriptor digest. No credentials. Server owns every authorizing field.

The immutable operation descriptor is created with the task ID in the enqueue transaction; its digest excludes mutable claim state. The claim response adds a separate `{claim_generation, host_challenge}` binding. A reclaimed task keeps its operation identity but receives a fresh binding; an old binding cannot submit a new receipt. Accepted receipt replay is checked before current-state revalidation and returns only the already stored, currently authorized result.

Daemon claims advertise `material-operations-v1`. Claim/finalization/start and local permit retrieval must compare current selection/authority with frozen operation. Task generation binds the current claim; observations from earlier claims cannot promote state. Git operation tasks are issue-less with narrow material context, while plan generation keeps the existing issue through the same internal runtime carrier. No new task enum is necessary where trigger provenance/context suffices.

## CLI and daemon-only observations

- `multica project material operation <operation-uuid>`: read own active task's descriptor, not arbitrary project/job inventory.
- `multica project material observe <operation-uuid> [--review-url <url>]`: ask local daemon for readback using existing task-local RPC plumbing; locator is a hint only. It never accepts raw receipt fields or a token/command/host override. No remote mutation is performed by this command.
- `GET /api/daemon/tasks/{taskId}/material-operation`: the registered daemon reads the exact task/current generation expectation. This endpoint never creates or reveals a challenge based only on a human PAT. The host challenge is created during the actual authorized claim/finalization exchange and delivered only in its host transport envelope, excluded from model/task context.
- `POST /api/daemon/tasks/{taskId}/material-observations`: immutable versioned readback, authenticated by existing daemon authority plus exact host challenge and expected digest. Reject model task tokens and callers without the registered daemon/runtime/current claim binding. Existing daemon transports may authenticate with owner PAT: credential type alone is not sufficient to accept or reject a receipt.
- `POST /api/projects/{id}/materials/{materialID}/operations/{operationID}/permit`: exact task identity requests current scoped mutation permit. Payload is original request ID and expected descriptor digest; response carries current allowed stage/HEAD/path constraints and expiry, no provider credential. Merge permit requires live current human approval and recent complete native preflight; publication cannot escalate to merge.

The local daemon bridge reads expected operation from trusted server context, not caller arguments; direct model stdout cannot become a receipt. Final endpoint naming may adapt to existing router conventions without changing authority or payload semantics.

## Plan preparation before model launch

An auto-off plan has an issue but no runnable task or preparation request. Once automatic or explicit human start is authorized, its bound queue task can be claimed into preparation; daemon may submit `verify_baseline` observation while dispatched/preparing, before model `backend.Execute`. The server admits this one read-only action for that exact plan/baseline/generation; only a valid preparation receipt permits model start. Ordinary operation receipts still require their own allowed lifecycle state. Missing/unreadable data blocks before model launch, and reuse of an old baseline observation is forbidden. Reclaim invalidates old preparation and requires a fresh check.

## Readback payload

`{protocol_version:1, operation_id, task_id, runtime_id, claim_generation, expected_digest, observation_id, host_challenge, observed_at, outcome, source_complete, repository_identity, review, files, audit_note, blocker}`.

`review` records exact number/URL/source branch/target branch/head/state/merge SHA/actor/native checks and complete changes `{path,previous_path,status,mode}`. For a merged review, `merged_at` is the actual provider-recorded merge timestamp. Validate that instant against the original permit interval; a later independent readback can recover the result without extending that permit. Missing or out-of-window merge time cannot establish a baseline. This additive field closes the implementation review's delayed-recovery gap; the daemon must never substitute its read time. `files` contains bounded full UTF-8 expected file bytes and SHA256 at exact commit; server recomputes hashes and compares frozen bytes. `audit_note` records original decision marker/body/actor identity where required. Unknown/truncated data => source_complete=false and no authoritative transition. Errors contain safe codes, never raw token/provider bodies. Exact observation replay is idempotent; same ID with different payload conflicts.

Host challenge is generated and stored hashed for the exact registered daemon claim/generation; challenge bootstrap requires the existing registration/runtime ownership plus the actual claim/finalization exchange, never a generic human GET. Daemon copies the host-only transport field into private active-task state, never into agent environment/output/prompt. A current owner who can impersonate the registered daemon is part of the trusted-host boundary, not an identity the protocol claims to resist. Daemon persists pending receipt privately before sending and replays it unchanged on response loss. An untrusted/malicious host is outside this guarantee. No shared signing key/provider secret enters product task storage.

## Persistence and lifecycle

One additive material operation table, immutable expected snapshot and append/retained attempts; task and operation insert atomically. Unique operation ID, task ID, `(job_id,attempt_no)`; each index separate CONCURRENTLY migration. No foreign keys/cascade. Existing logical job holds active attempt pointer; queued/running/unknown outcome blocks another mutating attempt even if scheduler lease expires.

Carrier system_key is material-specific `(project_id,runtime_id)`, not agent_builder identity. Detach material carriers before generic runtime system-Agent deletion; preserve terminal task and operation history. Runtime/config switch blocks old permits/receipts and requires deliberate recovery; never rebind a shared user Agent.

## Scope of claims

The server prevents unauthorized platform stages and baseline acceptance. It does not claim to prevent arbitrary remote shell writes by a host with broad Git credentials. All controlled mutation tests exercise selected runtime tooling; read-only host observations verify actual fixture facts, not LLM attestations. Live model/provider execution remains separately authorized and reported.
