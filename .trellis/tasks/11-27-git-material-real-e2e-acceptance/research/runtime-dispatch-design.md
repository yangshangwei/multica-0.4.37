# Research: selected-runtime material dispatch and receipts

- Query: How can GitHub/GitLab material publication and post-approval merge run through the user-selected runtime without server App credentials, while retaining current approval, version, retry and baseline guarantees?
- Scope: internal; source design research only, no product edits, test executions, database access, Git operations or model calls.
- Date: 2026-11-27

## Findings

### 1. Reuse the ordinary task pipeline, select one exact runtime

Use one project setting `execution_runtime_id` for all material stages, including plan generation. Remove the material `connection_id` and `plan_agent_id` inputs. Existing configured agents are not an equivalent selector: choosing an agent silently selects its runtime, model and later edits. The user explicitly selected the runtime model. No second user-facing execution agent is necessary.

The ordinary pipeline is agent-backed. A narrowly scoped hidden material carrier is the smallest reuse seam:

- Existing raw-runtime precedent: `server/internal/handler/agent_builder.go:53` creates a hidden execution carrier from a requested runtime; `resolveBuilderRuntime` at348 validates same workspace, `canUseRuntimeForAgent`, and online state. `server/pkg/db/queries/agent.sql:92` creates `kind='system'`, a server-issued system key, private visibility and max concurrency1. Reuse that pattern, not builder-specific functions/keys or the chat UI.
- Create or resolve one hidden carrier per `(project_id, execution_runtime_id)` with a dedicated server-issued system key such as `material:<project>:<runtime>`. Persist its ID in internal material configuration/binding. Do not accept `kind`, carrier ID or system key from a settings request. Its owner is the human configuring the workflow; human approval identity remains separately persisted on each decision/operation.
- Give the carrier only product-owned material instructions and no arbitrary connected-app or plugin overlay. Select the runtime's normal default model unless an explicit model option is part of the eventual UI; do not hardcode Codex model versions. If a model is persisted, bind it per operation and reject drift.
- Existing ordinary enqueue pins `agent.RuntimeID` in the queue and checks it under locks: `PrepareIssueTaskEnqueue` in `issue_task_transaction.go:31`, `EnqueuePreparedIssueTaskInTx` at49, snapshot comparison at65, runtime lock at78 and queue insert at91. The claim hydrator refuses an agent that moved runtime (`handler/daemon.go:2300`). It does not silently route to another provider.
- For material jobs, additionally bind the chosen runtime ID, daemon ID, provider/profile identity and relevant configuration fingerprint to the immutable operation attempt. Recheck at enqueue, claim, token finalization, start and receipt. Never silently follow a reconfigured carrier or another available runtime. Runtime deletion/offline means an explicit waiting/blocked state; choose a new runtime through project configuration and issue a new attempt only after prior execution is settled.

### 2. Carrier history cannot use builder teardown unchanged

`kind='system'` carriers are hard-deleted by `DeleteSystemAgentsByRuntime` (`server/pkg/db/queries/runtime.sql:384`) during `RuntimeTeardown` (`service/runtime_teardown.go:215`). The builder cleanup explicitly deletes its carrier and task rows (`agent.sql:108`). Material audit history must not disappear with a runtime.

Before the existing system-agent cleanup, add a narrow material-carrier detach under the existing runtime/agent lifecycle locks: set only recognized material carriers' runtime_id to NULL, retaining kind/system_key and metadata. Existing `UnbindTasksFromRuntime` at runtime.sql:356 already detaches terminal task history before runtime deletion. Material attempt/receipt records retain the original runtime/provider/daemon identity as immutable snapshots without foreign keys. The settings picker can report the configured runtime as unavailable and retain a historical label. New configuration selects/creates another carrier; it does not mutate a carrier that owns a live attempt. Existing builder/system cleanup behavior remains unchanged.

### 3. Separate logical Git jobs, execution attempts and plan issues

Keep `git_material_job` as the logical publication/review/merge intent. Its unique `(revision_id, kind)` and current lease/state model already supply logical idempotency (`server/pkg/db/queries/git_material.sql`). A model run cannot be represented by the worker's current two-minute lease: a slow live task must not be reclaimed into a duplicate subprocess.

Add a material-specific immutable operation/attempt record linking:

- job, workspace, project, material, revision and kind;
- configuration version, resource fingerprint, selected runtime/provider/daemon and internal carrier;
- exact task ID, claim generation and operation generation;
- expected repository identity, deterministic branch, base branch, exact files/content hashes;
- decision ID, signer, reviewed HEAD and decision snapshot for approved mutation stages;
- status/outcome, request identity and receipt digest; preserve previous attempts.

An active-attempt unique index belongs in its own single-statement concurrent migration. Use application transactions for owner checks and dependent writes; no new foreign keys/cascades. The state lane owns final schema naming, but these bindings are required.

For Git operations, prefer an issue-less queue task with `context.type='material_operation'` and immutable operation ID/version. Do not create visible fake development issues for PR publication or polling. The queue already supports issue-less quick-create/autopilot task types (`handler/daemon.go:3100`, `:3163`), so a narrow material branch is preferable to a new scheduler.

- Add a material-specific enqueue helper that follows the transaction/attribution/runtime locking shape of `issue_task_transaction.go`, inserts the material task and attempt atomically, and calls `FinalizeIssueTaskEnqueue`/`NotifyTaskEnqueued` only after commit. Do not generalize every ordinary enqueue API merely to add one context type.
- Current `CreateAgentTask` builds context only from head_sha (`agent.sql:337`), so add a narrow material task insert/query accepting the typed context, force-fresh-session and max_attempts1. Avoid misusing `CreateQuickCreateTask` with a forged quick-create prompt.
- Existing queue SQL serializes issue-less rows with all issue/chat/autopilot keys NULL per agent (`agent.sql:808`). A per-project/runtime carrier consequently serializes material Git jobs for that project; this is sufficient initially. A distinct material attempt uniqueness fence still prevents duplicate job dispatch. Do not remove existing serialization globally.
- The plan remains the existing real `git_material_plan.issue_id`. Its internal agent ID becomes the same configured carrier, derived server-side. Its default/manual requested flags, one issue per baseline, active-task binding and explicit terminal retry remain unchanged. `checkMaterialRunAuthority` must compare the configured runtime/carrier, runtime visibility and current configuring human, not the removed plan_agent_id or a user-facing agent selection.
- Important hidden App read: `processMaterialPlan` currently resolves a server provider and reads Git before enqueue (`material_plan.go:336–347`). Replace this with a fresh daemon-verified baseline observation or a bound baseline-read preparation in the selected runtime; otherwise a no-App publish path still cannot start planning.

Generic task auto-retries can clone rows without an operation reauthorization. Set material operation attempts to max_attempts1 and explicitly exclude material contexts from unsupported rerun/comment successor paths. `retryAttemptCeiling` (`service/task.go:5362`) respects max_attempts<=1. Recovery creates a new attempt for the same logical intent after checking remote facts and current authority; never blind-run another mutation after timeout.

### 4. Claim descriptor, prompt and credential scope

Add a versioned credential-free `MaterialTaskDescriptor` in `server/pkg/protocol`, then attach it to both `handler.AgentTaskResponse` (`agent.go:378`) and `daemon.Task` (`types.go:101`). Fields should identify operation/attempt, material/revision, action, exact runtime, claim generation, config version, source/target and the expected content/approval hashes. Large canonical document content can be fetched through an exact task-authorized operation read instead of relying on truncated general issue context.

- In `buildClaimedTaskResponse` (`handler/daemon.go:2218`), recognize material task context before quick-create/default handling, load the actual operation under its workspace, validate runtime and carrier identity, then hydrate project resources with `resolveClaimProjectContext`. Never hydrate from attacker-supplied context IDs before ownership checks.
- `daemon.handleTask` already resolves provider from the task's exact runtime (`daemon.go:5134–5160`) and the normal runner calls `backend.Execute` (`:8521`). Do not short-circuit that path into a server/daemon mutation executor. Add material prompt dispatch before generic issue prompts (`prompt.go:240`), because the current default instructs `multica issue get <id>` even for an empty ID.
- Clear PriorSessionID/PriorWorkDir and sibling instructions for publication/merge stages. Do not carry authoring conversation state into a newly authorized merge task. Inject a narrow material brief instead of generic status-write/leader/delegation instructions.
- Extend material task capability lookup to issue-less operations. Current `materialTaskGuard` first searches `git_material_plan` by issue and only allows material reads/revision submission (`handler/material_capability.go`); an operation token must not fall through to the broad legacy API merely because it has no issue.
- Permit only current operation/material reads and its receipt/preflight entry points. Human approve/settings/start-plan remain human-only. Current claim generation, task state, selected runtime, material revision, config and signer authority must be rechecked in the same transaction that consumes a write.
- `FinalizeTaskClaim` (`service/task.go:3752`) already locks the exact current queue row and compares runtime/context/dispatched timestamp/claim generation at3826 before minting mat_. Extend its existing `checkMaterialClaim` hook to operations. Apply the same current binding at initial claim, singular/batch reclaim, token finalization, task start, local-directory waiting and daemon receipt consumption.
- Task credentials are intentionally isolated from daemon credentials (`daemon.go:7645–7658`). Preserve that rule. Do not inject an unrestricted daemon token into the model so it can report its own trusted receipt.

### 5. Read-only verifier and exact remote receipts

The root/state design chooses a daemon-owned, bounded read-only verifier in the selected run's environment. This does not replace the selected runtime's mutation work: the model executes clone/edit/commit/push, PR/MR creation, review-note writes and post-approval merge. The verifier reads actual provider state and files; it never creates/updates/merges remotely.

Use the existing active localhost task registry pattern: `daemon.go:7939–7951` binds the local repo-checkout endpoint to the exact mat_ and current task during provider execution. A material-specific local endpoint/CLI subcommand can accept a bounded request like `observe <operation-id> --review <number>`; derive task/runtime identity and allowed repository/paths from the registered descriptor, not body/header/env values chosen by the model.

Proposed sequence:

1. Model requests current operation instructions through its narrow Multica CLI.
2. Before an approved merge, local material preflight obtains a current server authorization for exact operation/decision/HEAD/config and independently reads current native gates, full changed-file set and approved content.
3. Selected runtime performs the allowed remote mutation using its local Git/hosting credentials. It may use git+gh, git+glab or equivalent installed runtime tools; no token is sent to the server.
4. Model invokes the observation CLI. The daemon checks the active invocation, runs hardcoded read-only provider queries against the bound repository, verifies complete response/file bytes and emits the receipt using its own daemon-authenticated API, outside the model's mat_ authority.
5. Server consumes the typed receipt in one transaction, checking operation generation, exact current task/claim, selected runtime, resource/config/decision and expected hashes before changing local review/baseline state. An identical request/digest returns the original receipt; changed payload under the same ID conflicts. Never infer a baseline from output prose, PRURL or exit0.

A receipt should include protocol version, operation/attempt/task/runtime IDs, claim generation, config version, decision/head (if applicable), actual provider repository identity, PR/MR number/URL/source/base/head/state, full changed paths/modes, native-gate facts, merge SHA/actor when merged, per-file path+digest at observed immutable commits, observation timestamp and bounded verifier evidence references. Receipt acceptance cannot trust a caller-supplied `verified:true`; the daemon computes observation contents itself. Server should compare the whole normalized receipt, not only an overall success flag.

Existing completion is insufficient: `TaskCompleteRequest` (`handler/daemon.go:4049`) and `TaskCompletedPayload` (`protocol/messages.go:157`) have output/PRURL/session fields, with no typed material proof. Keep ordinary completion as process lifecycle, and use a distinct material receipt endpoint. Completing without receipt yields unknown/recoverable material outcome, never success. Ordinary final callback replay can remain intact.

Do not give generic `gh api` an arbitrary URL/method from a model request. Read-only verifier arguments must be built from frozen resource identity; paginate fully, refuse redirects/unknown shapes, reject symlink/submodule and oversized/truncated files, sanitize transport errors, and never print auth-token output. Git transport credentials and hosting API auth are separate. The daemon user's HOME/XDG are intentionally retained for tools such as gh (`daemon.go:7695`), but each provider's child configuration still needs actual verification. `repocache/cache.go:24` already passes daemon environment to Git credential helpers.

### 6. Capability negotiation and model-call bounds

Add `material-operations-v1` to shared protocol constants and daemon's advertised list (`daemon/client.go:214`). Gate every material claim on the header for the actual claiming client, following the local-worktree capability precedent (`handler/daemon.go:3428`). Register/heartbeat metadata is a useful UI hint; the claim header is the final proof. Old clients must receive an actionable blocked task state, not a generic issue prompt with ignored descriptor. Wire both HTTP and WS claim paths through the common builder.

Do not create one model run for every15-second material worker tick:

- Publish once per immutable revision; awaiting human review requires no running model.
- A new valid human decision creates one merge/review-note attempt.
- A pending native check can be observed by the bounded read-only daemon verifier at controlled intervals on the same logical intent; only the actual allowed mutation stage invokes the model. If this is deferred from the first version, expose blocked+explicit retry rather than silently launching repeated model attempts.
- Reconnect/lost receipt replays the durable observation/receipt or reobserves the existing remote intent; no second mutation unless evidence proves needed and current authority permits it.
- Changes to selected runtime or config invalidate queued authority and require a new attempt after settling/cancelling the old one; they do not migrate a live invocation.

### 7. Proposed affected files and tests

| Area | Concrete files / purpose |
| --- | --- |
| Material domain/state | `internal/material/types.go`, `internal/service/material*.go`, `pkg/db/queries/git_material.sql`, migrations/generated output: runtime settings, operation/attempt/receipt state, current authorization and plan preparation |
| Raw runtime carrier | scoped material carrier query plus `service/runtime_teardown.go` / `queries/runtime.sql`: select exact runtime and retain material history on deletion |
| Queue reuse | material-specific enqueue helper beside `issue_task_transaction.go`, scoped task insert, `service/task.go`, `task_start.go`: attributed enqueue, max-attempt and current operation fences |
| Claim wire | `pkg/protocol` material descriptor/receipt, `handler/agent.go`, `handler/daemon.go`, `daemon/types.go`, `daemon/client.go`: capability-gated current payload and daemon-only receipt transport |
| Selected model execution | `daemon/prompt.go`, narrow runtime config sections and `daemon.go` registration/lifecycle integration: run normal selected backend, narrow operation brief, fresh stage context |
| Independent observation | new daemon material verifier/local endpoint and dedicated read-only provider interface: reuse parsing/validation helpers only, never remote mutation methods |
| CLI/API scope | `cmd/multica/cmd_project_material.go`, material handlers/router/task guard: operation read/preflight/observe submission, strict bounded JSON and task-only route authority |
| UI/API and docs | selected raw runtime picker and schema, removal of App connection/plan-agent selectors, material runtime state; built-in skill and source map updates |

Required regressions: no-App/no-server-GitLab-secret settings and real fake-runtime publish/merge paths; selected runtime A cannot be executed/reported by B; malformed/unknown descriptor refusal; old daemon refuses; no mat_ to daemon receipt escalation; stale task/generation/config/approval receipt refusal; identical receipt replay and conflicting ID; model fake success cannot create baseline; verifier full bytes/diff refusal; runtime offline/restart and lost receipt; terminal failure no blind auto retry; carrier deletion preserves records; existing manual plan holds and terminal retries; old provider path absent from production material execution. Fake executables only in default tests; real Codex/Git provider execution separately authorized.

## Related specs

- `.trellis/spec/server/git-materials.md`
- `.trellis/spec/server/task-api-capabilities.md`
- `.trellis/spec/server/security-boundaries.md`
- `.trellis/spec/server/database-test-isolation.md`
- `.trellis/spec/server/verification-environment.md`
- `.trellis/spec/server/managed-installations.md`

## External references

No new external API assumption is required for this source-routing design. The earlier task research already checked GitHub CLI auth and exact-head merge references. Implementation of the daemon verifier must verify the installed gh/glab behavior and actual target API version before depending on additional commands or response fields.

## Caveats / Not Found

- No existing general material-runtime descriptor, operation receipt endpoint or issue-less material queue branch was found. Existing plain task output is not structured remote evidence.
- The suggested raw-runtime carrier must preserve history explicitly; reusing builder teardown unchanged deletes evidence.
- This design trusts the authenticated registered daemon and its read-only verifier. It does not cryptographically prove an honest host or protect against a compromised same-user process/OS.
- Most importantly, host Git/gh credentials may permit direct remote merge regardless of the Multica stage. Task API ceilings and prompts do not prevent a model from using ambient shell credentials. Multica can refuse unapproved dispatch/receipt and never bless an unauthorized external merge, but cannot claim to make that remote side effect impossible without separate OS/credential/provider enforcement. Native protections remain required; do not promise a shell sandbox that does not exist.
- Same-runtime read-only verification is an observation boundary, not another mutation executor. Moving Publish/Merge into a daemon helper would violate the user's selected-runtime execution requirement.
