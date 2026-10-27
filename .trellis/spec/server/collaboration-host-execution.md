# Collaboration execution with ordinary host permissions

> Implementation contract, 2026-10-26. Protocol/service/daemon and shared-client implementations exist behind default-closed admission. Existing Linux runs keep their original namespace semantics. Native/account/packaged acceptance is tracked separately in [the task verification ledger](../../tasks/10-24-cross-platform-collaboration-runtime/verification.md); unverified combinations are not released.

## 1. Scope / Trigger

Apply when implementing host_process_supervision_v1 for collaboration runs by reusing the ordinary Provider backend. This is a deliberate change in execution guarantees, not an OS exception to linux_pid_namespace_v1.

The authoritative product requirements, architecture, implementation order and user-facing copy live in [the existing runtime task](../../tasks/10-24-cross-platform-collaboration-runtime/prd.md). Retain the current human collaboration model and ordinary task behavior.

Ordinary host access may include the daemon user's files, provider accounts, configured tools and local services. Separate directories and provider homes organize executions; they do not establish an OS security boundary. Business and termination credentials remain logically separated, but this mode does not guarantee secrecy from a hostile process with the same OS-user access.

## 2. Signatures

The reusable leaf remains Backend.Execute(context.Context, prompt, agent.ExecOptions) returning agent.Session, whose Messages and Result carry provider events. The collaboration adapter supplies its own context, credentials, lifecycle observer and report sink. Do not call the ordinary runTask/StartTask/ReportProgress path with a synthetic ordinary Task.

Execution-handle responsibilities:

~~~text
Probe(runtime configuration) -> verified adapter capability
Prepare(frozen collaboration input) -> collaboration environment
BeforeLaunch(run identity, launch identity) -> durable registration + Server permit
Started(launch identity, native process instance) -> persisted observation
Waited(launch identity, actual adapter terminal evidence) -> persisted observation
Stop(run identity) -> seal launches + cancel managed executions
Seal(run identity) -> mode-specific termination receipt
~~~

BeforeLaunch must cover Provider-internal retries as well as outer fresh-session fallback. Instrument the common launch/wait construction or tested provider lifecycle callbacks; one Backend.Execute call is not one physical process.

Wire/storage contracts:

| Contract | Current shape |
| --- | --- |
| Runtime protocol | New host messages use protocol_version=3; Linux protocol 2 remains strictly decoded under its original branch |
| Execution mode | Reuse containment with linux_pid_namespace_v1 or host_process_supervision_v1; do not add a second conflicting mode field to writes |
| Read projection | v6 run fields are execution_mode, launch_count (decimal string), descendant_cleanup (nullable state/scope/attempted); proof_state adds host_exit_observed. New clients also parse v5 |
| Read negotiation | execution_view_version=6 query parameter; omitted means the existing v5 contract |
| Human AI writes | execution_view_version=6 for host-mode submissions/controls; metadata declares comprehension, never permission |
| Preparation | Host preparation_version=2 with explicit ordinary Provider adapter, effective invocation/configuration identity and bounded input manifest; old version 1 keeps its Linux bundle contract |
| Local ledger | Version 2 records mode and all launches; version 1 remains a legacy Linux record |
| Database | Migration 738 adds nullable collaboration_attempt.launch_descriptor/terminal_observation JSONB, nullable authorized_at for unissued tombstones, immutable attempt/proof guards, a shared mode-aware terminal predicate and default-false host_execution_enabled |

`agent.ExecOptions.Lifecycle` carries `*agent.LaunchLifecycle`. Its observer implements `BeforeLaunch(context.Context, agent.LaunchInfo) (agent.LaunchCallbacks, error)`; `Started(ProcessObservation)` and `Terminated(LaunchTermination)` persist native observations. `LaunchCallbacks.Environment` refreshes the current attempt's narrow business token and temporary path immediately before spawn. `Stop` fences constructed, denied and active commands; a repeated Start/Wait cannot discard the original observation or Job ownership.

`agent.NormalizeHostProviderVersion` removes only known Claude/Codex/OpenCode banner decoration and preserves prerelease/build identity. Capability fingerprints, daemon registration comparisons and guarded smoke tests use the same normalized identifier. Ordinary diagnostic banners are unchanged. Host diagnostics use the frozen version; they cannot launch a late `--version` process outside permits.

Keep existing collaboration runtime URI families for claim, permit, outcome, stop-intent and proof, dispatching through strictly versioned schemas. Every old request/response remains its original shape. The dedicated CLI context/environment must validate both supported protocol branches; unknown versions fail before ordinary profile lookup.

The five existing provider identity variables remain MULTICA_COLLABORATION_TOKEN, MULTICA_COLLABORATION_SERVER, MULTICA_COLLABORATION_CONTEXT, MULTICA_COLLABORATION_ISSUE_ID and MULTICA_COLLABORATION_RUN_ID. Normal mat_, user PAT/JWT and daemon management identities are not substitutes.

## 3. Contracts

### Capability and consent

A host capability binds deployment, installation, runtime, OS/arch, adapter/version, Provider/version/mode, configuration epoch and digest, preparation version and supported lifecycle observations. Register only after the actual combination passes its probes and tests; an advertised Provider name is insufficient. Declare allowed supervision scopes and ordinary fallback, such as Job attachment failure falling back to direct-process ownership. Record the actual scope for each launch. A permitted downgrade is a disclosed fact within the host capability, never a successful Job observation or a switch to another execution mode. An undeclared scope or missing direct lifecycle observation is not admissible.

Host capability requires an actual local runtime, not a cloud runtime advertising local observations. When host cutover is enabled, a new namespace root cannot be promoted or claim a new reservation. Already claimed Linux runs/groups retain their mode, including required member/leader continuation work. An untouched pre-cutover reservation is never relabeled or deleted to bypass consent; existing nonlaunch/Stop/configuration-invalidation paths remain authoritative.

Installation metadata names macOS `macos`, while native Go capability names use `darwin`. `collaborationInstallationMatchesOS` normalizes only that boundary in both registration and later target admission. Linux/Windows and cross-platform mismatches remain exact; never enroll a false OS merely to make a native test pass.

Windows shell/batch wrappers are a distinct native case: their Provider executes in a child. A host wrapper must acquire its Job while suspended and its managed child work must end before terminal observation. Failed attachment kills and waits the still-suspended wrapper, rather than running it unowned. Native executable direct-process downgrade remains allowed with truthful limited cleanup. Ordinary nil-lifecycle launch behavior is unchanged. Unix group existence alone can include zombies and does not expand the registered managed-execution set into an all-descendant guarantee.

Freeze that capability with the run and compare it at claim, every launch permit and termination. A newly registered mode is not permission to reinterpret an in-flight run.

A switch from namespace isolation to ordinary host access invalidates prior resource consent through the existing configuration/epoch mechanism. The actual resource owner confirms the changed boundary once per affected issue/configuration, not once per instruction. Waiting inputs survive; explicit Resume remains necessary.

Fingerprint the managed invocation, account-source identity and declared tool permissions. Ordinary provider token refresh is not a new consent event. Host-side configuration outside the managed snapshot is not hermetically frozen; document that host trust rather than claiming the process cannot access it. Detectable managed permission expansion requires renewed consent.

### Independent context and identity

Stable session identity includes collaboration, deployment, resource owner, agent, issue session, channel, provider runtime, role and session generation. Member execution adds group/member identity. Per-run temporary identity adds run/launch.

Never resolve session pointers by display name or most-recent session. Runtime/provider/account identity or incompatible role/config changes create a new session generation. Human handoff alone does not.

An unverifiable or rejected session pointer may lead to an explicitly recorded fresh session using authorized issue context; it cannot silently attach to private chat or a normal issue session.

Server-origin input authors are retained individually across batching. Agent owner, runtime/resource owner, initiating human and parent execution are distinct. A daemon identity authenticates delivery; it does not replace the human author. Provider text cannot mint or change those identities.

Only one managed execution may own a stable session/work-directory identity at a time. Reuse scheduling/environment locks and persist that ownership with the launch ledger; an OS lock released by a daemon crash does not settle an unknown logical owner. Two otherwise valid runs sharing a member/session identity must serialize. Do not hold a scarce machine slot while waiting in a way that prevents the current owner from finishing. Release ownership only after reliable managed termination.

Use shared files only through the frozen attachment/Git preparation contract. Keep path validation, size limits and original request identity even though host filesystem isolation is no longer promised.

The selected Multica CLI is copied into a private run-local bin under `multica`/`multica.exe`, and that bin precedes PATH. Prepending only the selected file's source directory is insufficient: a differently named binary could otherwise resolve an ambient CLI. Codex uses ordinary account/config helpers with a collaboration-local session home; source account refresh is not a private-session import.

### Shared context

Host claims (protocol 3 only) set `memory_version=1`. It is a claim-only declaration: not a registered capability, not part of the capability digest, so it never advances a configuration epoch or invalidates consent. The first claim records `collaboration_run.memory_mode` (`supported`/`unsupported`) in the transaction that freezes the prompt; later claims never rewrite it.

After the launch permit and session wait, the runner calls `POST .../runs/{runId}/memory/base` with the business capability. The server freezes `memory_base_version` on the first call (`RuntimeMemoryBase`); later calls return that version (a purged base is replaced by the current version). The fresh-session retry calls it again and injects what it returns. The injected base reaches the Provider only as the prompt's `memory` field `{version, content}`. It is not written to the brief file, the local ledger or audit fields, and the prompt is not part of the run recording; the brief carries only fixed guidance and the `memory show`/`update` commands. Text an agent later reads with `memory show` is a tool result and is recorded like any other, redacted and bounded. `409 memory_unsupported` runs without `memory` and without the brief guidance. Any other final failure, including exhausted transport/5xx/408/429 retries, leaves the Provider unlaunched and settles the authorized launch as `not_started`; a fresh-session retry that cannot re-read its base never starts the fallback.

Runtime `GET`/`PUT .../memory` use the runtime comment gate plus `memory_mode=supported`. AI writes are redacted with `redact.Text` before the byte limit, the `(run_id, request_id)` replay comparison and storage.

### Starts, retries and audit

Each physical launch has an immutable tuple containing run/fence/claim generation, launch_id UUID, monotonic attempt sequence and nonce, launch kind, fixed capability and configuration. Persist before the irreversible start and acknowledge the actual process-instance identity afterwards.

A consumed launch identity never starts again. Lost network replies replay the same permit or receipt. Ambiguous spawn/error/crash remains unknown; it must not roll back to never_launched_proven or trigger a new attempt.

Provider/daemon-created launches and their preparation helpers are registered here. Arbitrary subprocesses created by model tool calls are not all intercepted; their cleanup is limited to the declared ordinary supervision scope.

The v3 per-launch endpoint is POST /api/collaboration/runtime/runs/{runId}/launches/{launchId}/termination, authenticated with the original run termination credential. A strict request binds protocol_version=3, original run identity/containment, attempt_seq, attempt_nonce/launch_id, event_id, and observation kind not_started or managed_launch_ended, with bounded cleanup facts. In host mode launch_id is the UUID attempt_nonce, not a second independent identity.

Persist the original observation and its receipt idempotently on the existing collaboration_attempt row. Add nullable launch_descriptor JSONB and terminal_observation JSONB in a forward migration; the descriptor stores launch kind/capability/configuration and the observation stores validated terminal facts plus Server acceptance metadata. Old v2 rows retain NULL and their original interpretation. A changed payload for an existing observation key conflicts rather than overwriting history.

The terminal receipt includes required `accepted` and `authorized` booleans. An exact next-sequence `not_started` whose permit never reached the Server may create an unauthorized tombstone (`authorized_at` and `launch_descriptor` remain NULL). It creates no business capability and does not advance authorized_attempt_seq. A delayed permit cannot upgrade that tombstone. Final proof lists only the contiguous authorized launches; the unissued local nonce stays retained. Missing/null receipt booleans are protocol errors.

Per-launch terminal acceptance does not seal or settle the run, release its reservation, or grant another attempt. Permit N+1 atomically requires the accepted terminal observation of N, all older permits accounted for, current authority, and an unsealed/not-stopped run. A run-level final receipt seals the maximum authorized sequence only after all authorized launches are accounted for; only that final receipt can produce host_exit_observed and settle the run/group. An observation may still be submitted after revocation for original cleanup, but never authorizes restart.

The old v2 attempt_seq=1 and final-proof behavior stay unchanged. The new per-launch transition is explicit; it cannot be simulated by sending the old final Proof endpoint early.

Every additional Provider launch, including an internal transport/session retry, re-enters the gate. It requires prior managed attempts ended, current authorization, an unsealed run and a new Server permit. Offline execution cannot obtain that new permit. Do not add more retries than the existing ordinary Provider policy.

Serialize local stop/seal against physical start. If stop wins, a granted but unused permit cannot launch. If start wins or the outcome is uncertain, retain that launch until its own terminal observation. No late fallback after Stop, revoke or run seal.

Audit events bind request, input author, target, group/parent, run, launch, mode/configuration, resource identities, stop actor/scope, business outcome, usage and receipt. Events and usage are deduplicated by stable event/launch identity. Private arguments, secrets, raw environment or filesystem paths are not public audit fields.

The anti-duplication guarantee covers dispatch/start and record replay. It is not exactly-once for arbitrary model tool calls, shell commands, Git pushes or MCP side effects.

### Three independent end facts

| Fact | Values | Meaning |
| --- | --- | --- |
| Business outcome | success, failure, missing/incomplete result | Provider/application outcome; never inferred solely from exit code |
| Managed termination | never_started_proven, never_launched_proven, host_exit_observed, unknown | Whether the fixed mode has a valid terminal observation |
| Descendant cleanup | tracked_scope_complete, best_effort, unverified | Evidence limited to the declared Job/process-group/direct/remote-session scope |

never_started_proven remains a Server-only settlement with zero permits and no contradictory launch evidence. never_launched_proven requires a durable record proving no irreversible start occurred. A burn followed by an ambiguous failure is neither.

host_exit_observed requires all registered direct execution/helper launches confirmed not started or actually waited/terminated through a verified lifecycle callback, and the run sealed against further launches. There must be no unknown launch or known still-running execution within the adapter's claimed managed scope.

A receipt includes the run identity and mode, sealed launch count/last sequence, each launch's terminal observation and a bounded cleanup summary. Native process identifiers remain local unless an existing private protocol requires them; public views expose only safe mode/status/scope summaries.

Do not use agent.Result.ProcessExited as the only cross-provider predicate. Some providers already wait but leave it false; others mean only direct-process exit. Normalize actual lifecycle facts. Result channel close, stdout EOF, success prose, elapsed time, lost heartbeat or a missing PID alone is insufficient.

`agent.Result.BusinessOutcomeMissing` explicitly withholds a business outcome when an adapter has no terminal result. In particular, OpenCode's ordinary parser default `Status=completed` with no events is not host business success. Host recording keeps the result incomplete; explicit completed results with empty text remain valid. Before any shared record is sent, the existing redact utility removes bare `mcb_`/`mct_` 64-hex capabilities as well as its existing secret patterns.

Best-effort/unverified descendants outside the claimed managed scope do not block host_exit_observed. Known live managed work, an unsealed launch path, broken ledger or unresolved instance identity does. A failed process-group/Job cleanup that leaves known managed work active cannot be relabeled successful.

Remote/gateway execution requires remote terminal/cancel evidence appropriate to that mode. A local CLI exit cannot prove remote completion. Unverified combinations remain unavailable.

Legacy linux_pid_namespace_v1 accepts its original proof states including exited_proven; host mode never generates that value. Neither mode may settle an unknown run by relabeling it under the other mode.

### Settlement, cancellation and recovery

One mode-aware predicate is shared by proof acceptance, run/group settlement, reservation release, Resume, Switch and lifecycle cleanup. A group ends only after all admitted child/member/continuation runs independently meet their fixed contracts.

Physical machine capacity may be released after reliable local terminal observation is persisted. Server reservation and queue progression wait for the same-identity receipt to be accepted. An offline local release never grants a new Server execution permit.

A user Stop pauses the existing scope and seals expansion before cancellation. Completion/cancellation of an issue and authority withdrawal still request stopping all affected registered work. These Server actions do not guarantee instantaneous delivery to an offline machine.

Temporary transport errors and 5xx preserve an already-running ordinary-mode execution and retry synchronization. No new launch or delegation without current Server permission. A definitive rejected/revoked identity or closed issue triggers cancellation; no retry under another identity.

A persisted local end observation is resent with its original identity until acknowledged. A daemon restart without reliable end observation retains unknown and its launch tombstones/barriers. Do not recreate a nonce, auto-restart the instruction or release merely because the boot/PID changed. New native process-instance recovery evidence requires a tested adapter contract.

Outcome availability cannot delay a reliable final proof. A bounded recovery round gives a sealed proof priority and reserves time for it. An ACKed host proof with a pending original outcome receipt remains recoverable without holding a machine slot or session barrier. Under the original issued business token, the Server may ACK only an already stored identical outcome (run/attempt/nonce/mode/event/full payload), including after settlement or revocation. Missing or changed late outcomes cannot write content/usage, change settled business state or restart work. The termination token never becomes a business writer.

Retained host claims are revalidated against the same manifest, all input authors, resource identities, capability/configuration digests and session key used on HTTP admission. A malformed saved claim is not a fresh launch opportunity. A callback arriving after a failed/closed supervisor cannot modify the retained record; unknown execution ownership survives. Interrupted HTTP response bodies are transport uncertainty, not affirmative revocation.

Resume still requires current authority/configuration and no unresolved managed execution in the selected scope. A received receipt, reconnect or renewed grant never implies Resume. This version adds no force-release/ignore-unknown action.

### Client and upgrade behavior

A v6 reader understands the fixed mode and all three facts. For clients that did not negotiate v6, any host-affected execution graph becomes an internally consistent v5 unavailable projection, with its AI command/control capabilities disabled. Do not remove a node but leave group/channel references dangling, or convert host_exit_observed to exited_proven.

The shared execution panel retains the v5 unavailable message for legacy reads. For v6, render the new-execution-disabled message only when `execution_gate.enabled !== true`; an enabled gate must not retain the legacy global unavailable banner. A closed dispatch gate does not remove prior run facts, records or Stop controls. Individual target eligibility remains separate.

Human issue reads/comments remain available under existing authorization. Old clients may only show their existing unavailable label; explicit new upgrade wording is limited to clients/diagnostics that understand the new code. Direct old AI command/control requests aimed at a host-affected scope return a stable upgrade-required refusal. For a mixed comment, preserve the comment and valid human notifications under AC91; only incompatible host AI targets receive blocked/upgrade-required receipts without queued commands. Original-request replay preserves that result even after upgrade. Human lifecycle/leave/removal operations remain available and may trigger policy-driven system cancellation under their original authority. A mixed-mode graph is not partially rewritten into stronger evidence.

New clients must accept v5 from an old server, mark unsupported host features unavailable and never downgrade a requested host run to another execution mode. The capability metadata is not a security credential.

Some old read endpoints reject the extra query parameter. A new client may retry a GET once, removing only `execution_view_version`, on HTTP 400 codes `invalid_records_query`, `invalid_resource_scope` or `invalid_resource_offer_query`. Authentication, remaining query/cursor fields, cancellation/access epochs and response parsing stay unchanged. Writes, authorization errors, 5xx and malformed/newer responses never use that fallback. Partial v6 resource metadata, or missing mode metadata when consenting from a v6 issue, cannot select legacy consent copy or grant authority.

Historical mode classification reads only bounded runtime references and retained run modes. It does not require opaque legacy private rosters to parse as a current authorization object; actual submission/consent/Resume still validates the complete typed roster. Historical proof/preparation projections compare the original termination-bound claim identity, because Stop can advance the current claim generation.

Forward migrations extend existing CHECK constraints and update sqlc/Go/read validators together. Old run mode, grant history, nonce tombstones and unknown barriers survive. Keep legacy v1 ledger/proof processing only to drain/read old data; do not make it a silent fallback for new host launches.

Rollback disables host admission first while preserving receipt readers and settlement. Do not deploy a server binary that cannot read persisted new states, or delete host history with a downgrade migration.

## 4. Validation & Error Matrix

Runtime handlers and service admission use the following stable refusals. Configuration/grant failures retain their existing codes; local adapter rejection remains `ErrUnsupported` before registration.

| Condition | Required outcome |
| --- | --- |
| Unsupported mode/protocol/provider combination | execution_mode_unsupported, unsupported_runtime_protocol or runtime_preparation_unsupported at the matching boundary; no claim/launch |
| Prior consent for a different execution boundary | grant_required or target_configuration_changed; keep waiting inputs and hold |
| Mismatched mode/config/run/attempt/nonce | execution_identity_mismatch; no state change |
| Old client sends direct host AI command/control | collaboration_client_upgrade_required; preserve human access and policy-driven lifecycle cleanup |
| Old client sends a comment containing a host AI mention | Persist authorized comment/person notifications; block only incompatible AI targets with original replayable receipt |
| Duplicate launch/permit/event/receipt identity | Return original result; no second spawn or usage |
| Stop/revoke wins before fallback | Reject launch; record original stopped/refused attempt |
| Per-launch terminal observation replay | Return original receipt without sealing/settling run or granting the next attempt |
| Permit N+1 without accepted terminal observation of N | Refuse; no new permit/spawn, including after restart |
| Burn or native start is uncertain | unknown; retain barrier; no automatic retry |
| Adapter has actual end observation but old ProcessExited=false | Normalize supported lifecycle evidence; do not permanently block solely on the old field |
| Only success text, EOF, timeout or missing PID | No termination assertion |
| Remote/gateway Provider mode | Local adapter ErrUnsupported before registration; no local-exit substitution |
| Managed work ended, descendant coverage unverified | Accept truthful host observation, expose cleanup limits |
| Known managed process still running | stop_unconfirmed; no host terminal receipt/Resume |
| Business outcome unavailable after valid termination | End execution, retain incomplete result and original pending receipt; never invent success or block proof |
| Known old GET query refusal | Retry once without only read-version metadata; retain all auth/parser guards |
| Missing mode on v6 consent resource | Refuse consent; never show legacy permission copy for a v6 intent |
| Late identical stored outcome | ACK only, with original business capability and unchanged state |
| Unsupported preparation or ignored invocation option | Explicit refusal before launch; no preparation=0 bypass |
| Ambiguous historical record or unreadable ledger | Retain original identity/barrier, report recovery failure |
| Shared-context base `memory_unsupported` | Launch without `memory` field or brief guidance |
| Shared-context base refused or retries exhausted | No Provider start; authorized launch settles `not_started` |
| Fresh-session retry cannot re-read its base | Fallback never requests a permit or starts |

## 5. Good / Base / Bad Cases

- Good: B invokes A's authorized agent on runtime C; a session retry is a second recorded/authorized launch, both waits complete, outcome and cleanup facts keep B as author and A/C as separate owners.
- Good: OpenCode waits for its CLI but omits the old ProcessExited boolean; the verified lifecycle observer records actual end and releases the new-mode reservation after receipt acceptance.
- Base: ordinary tasks retain their existing behavior; legacy Linux runs continue using original namespace receipts; UI human reads still work on older clients.
- Bad: copy an ordinary Task and call runTask with the owner's identity, report direct exit as namespace destruction, or quietly expand old consent.
- Bad: hold every provider forever because its old result struct omitted ProcessExited, or release an unknown spawn merely because a timeout fired.

## 6. Tests Required

- Provider fake executable tests: observe every start/wait, internal/outer retry, pre/post-start error, zero-output result, separate business outcome and stop.
- Race tests: duplicate daemons, repeated request IDs/nonces, per-launch observation replay, Stop versus the next-attempt permit, stop versus physical spawn, revoked author/resource during fallback, original outcome/usage replay.
- Session tests: same names/different IDs, deployments, users, roles, ordinary/private sessions, two collaboration issues, compatible continuation and honest fresh session; two valid runs for one stable identity serialize, and unknown ownership survives daemon restart.
- Authorization tests: existing owner consent shared by participants; invalidated consent on new mode/managed permissions; actual author versus resource identity; ordinary tokens still cannot read/write collaboration content.
- Native tests: Windows Job attachment downgrade, Unix changed session/group, closed stdio, ordinary cleanup facts, permission errors, daemon crash/restart and ledger locking.
- Service/SQL composition: new/old mode cross-rejection, state CHECKs, group/member/continuation and capacity-one, partial Resume, Switch, issue lifecycle, retained unknown.
- API/client tests: v3/v2 and v6/v5 strict parsing, malformed response behavior, valid legacy unavailable projections, stable per-target upgrade refusals, mixed comments/human lifecycle preservation, existing old-client unavailable labels, and no dangling graph references.
- User copy tests/review: no all-descendants, OS-sandbox, instantaneous offline stop or exactly-once tool-effect claim for host mode; enabled v6 execution cannot display the legacy global unavailable banner.
- Default tests use only test-owned executable paths. Authenticated real-provider smoke remains separately authorized under CLAUDE.md.
- Shared context (`host_memory_test.go`): the claim declares support, the prompt carries the base while brief, ledger and permit/preparation/termination/outcome/proof payloads do not, unsupported runs omit it, refusal/exhaustion never launch, a transient failure recovers, and a fresh-session retry re-reads its base or never starts; the client rejects malformed bases and keeps refusal codes. The base freeze itself is a server contract and belongs in the service tests.

Canonical added suites include `collaboration_host_runtime_test.go`, `collaboration_host_integration_test.go`, `collaboration_host_cutover_test.go`, `collaboration_outcome_receipt_test.go`, `collaboration_execution_version_test.go`, `host_ledger_test.go`, `host_session_test.go`, `host_runner_test.go`, `host_review_test.go`, `host_git_test.go`, `launch_lifecycle*_test.go` and `execution-host.test.ts`. CI `collaboration-host-native` records actual OS/arch and requires named native tests to pass rather than skip. `host_agentintegration_test.go` is behind both `agentintegration` and explicit `MULTICA_RUN_REAL_AGENT_SMOKE=1`; its default gate must precede Provider discovery/account access.

`execution-panel.test.tsx` owns the availability-banner regression. `e2e/collaboration-host-runtime.spec.ts` composes production Web, a native Electron renderer and real test-owned Provider launches against a password-authenticated API; it verifies shared-Agent participants, separate sessions/directories, attachments, current CLI identity, scoped Stop and explicit Resume. Record its harness limits separately from packaged full Main, real Provider accounts, native platform and squad/Git acceptance.

`e2e/collaboration-host-installed.spec.ts` is a separate opt-in for a pinned directory-packaged Linux full Main/preload/renderer controlling the native host fixture. It uses the real password form, preserves pause across app restart and explicitly resumes mixed-author inputs. An absent opt-in skips before lookup; an explicitly selected missing image fails before Docker can pull it. Record the packaged client OS separately from the Provider runtime OS, and pin the ASAR hash. This scenario does not authenticate a vendor account or replace the native daemon fault matrix.

`scripts/test-go.sh` serializes packages with `-p 1` because DB-backed packages share `DATABASE_URL` and mutate the organization singleton. Keep within-package `-parallel 2` and the separate bounded agent batch. Running handler and service packages concurrently requires separate databases, not merely different fixture workspace IDs. The shell regression covers this runner boundary.

## 7. Wrong vs Correct

Wrong: remove namespaceSupported, pass the same ordinary task token, set NamespaceID to a PID, and submit exited_proven.

Correct: admit a separately versioned host capability, preserve issue-bound authority and actual authors, observe and seal all registered starts, submit host_exit_observed with its limited cleanup facts, and let the mode-aware Server predicate settle the run.

Wrong: change a legacy unknown run to host mode during upgrade and retry the original instruction.

Correct: preserve the old run, mode and nonce/barrier; only subsequent authorized new executions use the new mode. Renew the resource owner's consent before expanding host access.
