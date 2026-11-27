# Research: Material state through selected-runtime Git operations

- Query: Replace direct server GitHub/GitLab publication/merge/read operations with selected-runtime tasks and trustworthy results, preserving human approval, immutable baselines, manual planning and recovery.
- Scope: internal; concrete design recommendation, no product edits or executed tests
- Date: 2026-11-27

## Findings

### 1. Current owners and reuse points

| File / anchor | Actual responsibility |
| --- | --- |
| `server/internal/material/types.go:21` | Public settings require provider/resource/connection and optional plan agent; `ValidateSettings` at199 rejects missing connection. |
| `server/internal/service/material.go:355` | `checkMaterialBinding` fixes resource digest and server integration trust revision. Connection lookup must be replaced, not only made nullable. |
| `server/internal/service/material.go:605` | `Revise` freezes full payload/settings/config version, moves the current revision pointer and creates a publish intent only for ready input. Reuse this. |
| `server/internal/service/material.go:662` | `Decide` requires a human, current material/config/review HEAD and original credential metadata, retaining immutable decision rows. Reuse this. |
| `server/internal/service/material_worker.go:26` | Ticker claims a short server job, then performs synchronous provider I/O. No runtime Git dispatch currently exists. |
| `server/internal/service/material_worker.go:76` | `workerTx` locks workspace/project/job and rechecks configuring human, revision and settings. Reuse authority concepts but do not hold DB locks during a model run or remote read. |
| `server/internal/service/material_worker.go:115` | Server-owned material file list, allowed modes/paths and exact immutable content comparisons. Extract/reuse pure validation for host observations. |
| `server/pkg/db/queries/git_material.sql:63` | `EnsureMaterialJob` deduplicates `(revision_id,kind)` and resets only blocked/done jobs. At67 the lease is two minutes; expiry is unsuitable as permission to launch a second long-running mutation. |
| `server/internal/service/material_plan.go:198` | Existing durable plan issue and automatic/manual policy; one issue per material/baseline. At336–347 it still reads Git via `ResolveProvider`, an otherwise-hidden App/token dependency. |
| `server/internal/service/issue_task_transaction.go:25` | Prepare-before-transaction and transactional ordinary queue insertion; exact agent/runtime and attribution are rechecked. Use patterns for new material dispatch, not dummy issue creation for every Git action. |
| `server/internal/service/material_plan.go:20` | Material claim/start authorization and denial settlement already cover exact task, full handoff and current authorities. Extend to runtime-operation tasks without weakening ordinary tasks. |
| `server/pkg/protocol/messages.go:157` | Ordinary completion only reports TaskID, PRURL and Output. This is not structured remote-state proof. |
| `server/internal/handler/daemon.go:151` | Daemon task callback helper binds workspace and runtime access. New observer callback additionally needs exact attempt/generation/nonce validation. |
| `server/cmd/multica/cmd_repo.go:335` | Existing CLI-to-local-daemon endpoint uses task credential and active task ID. Reuse this pattern for read-only observation requests. |
| `server/pkg/db/queries/agent.sql:92` | `CreateAgentBuilder` is a precedent for hidden system carriers; it freezes runtime configuration without using a shared user agent. Its deletion semantics must not be copied to material history. |
| `server/internal/handler/issue.go:4922`, `project.go:920`, `workspace.go:1150` | Existing material-history guards prevent casual deletion of retained records. New attempt history must join those contracts. |

### 2. Recommended settings and execution selection

Use one public `execution_runtime_id` for publication, decision synchronization, approved merge and plan generation. Remove material `connection_id` and `plan_agent_id` selection, including GitLab encrypted-connection lookup. The selected runtime's provider/profile/host identity selects the execution environment. Keep provider and repository target facts, but derive and validate actual host/repository from the authorized project resource; do not accept a caller-supplied credential or token source.

Ordinary queue tasks require an Agent record. Reuse the existing hidden `kind=system` carrier approach, scoped to material project plus selected runtime, with a recognizable material system key. The carrier is internal plumbing, not another user-facing executor selector. Do not mutate a shared user-created Agent to point at the requested runtime. Runtime access checks must use the current runtime owner/current requesting human predicate, not accidentally enforce the carrier owner's private-agent invocation rule as product semantics.

Freeze runtime ID, daemon/profile identity, relevant runtime configuration epoch/digest and human originator in each operation attempt. The visible plan issue is retained; its queue task uses the internal carrier and selected runtime. A runtime rebind/configuration change blocks stale attempts and requires a new validated attempt. Never silently follow another runtime because the old one is offline.

### 3. Durable state: logical jobs versus runtime attempts

Keep `git_material`, immutable revision/decision/comment/request rows and logical `(revision_id,kind)` job deduplication. Add a material-specific attempt relation instead of placing durable authority only in free-form queue context:

`git_material_operation`: `id`, workspace/project/material/revision/job IDs, monotonic `attempt_no`, action, original operation key, current decision ID where relevant, source baseline ID where relevant, expected descriptor JSON and digest, config version, human authorization identity/credential metadata, internal agent ID, runtime ID, daemon/profile/config snapshot, task ID, claim generation, private host-observation challenge hash, state, observation/receipt JSON and digest, blocker, created/admitted/completed times.

The exact action vocabulary should be frozen with the main design: `publish`, `review_sync`, `merge`, `observe`, `verify_baseline` are sufficient; plan generation retains the existing plan domain. Each row represents one exact task attempt; a changed decision, runtime or claim generation is a new attempt, not an in-place change to its original authority. Bind attempts to the ordinary queue row in one transaction. Add an active-attempt pointer to the logical job and make the state machine the one dispatch owner.

Recommended state handling:

| State/event | Required behavior |
| --- | --- |
| Ready revision / human decision | Durable logical intent created in the existing transaction; zero immediate provider call. |
| Dispatch | Validate current config/revision/human/runtime; insert one task and attempt atomically; publish wakeup only after commit. |
| Queued/claimed/running | Read ordinary queue status; no second task on material job lease expiry. |
| Completed model with no trusted observation | Attempt remains outcome-unconfirmed; never mark review published or baseline ready from final prose. |
| Lost observation HTTP reply | Daemon retries exact stored receipt/challenge; identical replay returns prior outcome, changed body conflicts. |
| Task lost/timeout after possible side effect | Reconcile remote state first under the same logical key/branch. Do not rerun mutation on timer or assume no effect. |
| Proven no effect and terminal task | A new attempt may resume the original immutable intent after current authority checks. |
| Obsolete config/revision/decision | Do not dispatch or promote receipt; preserve historical task/observed facts and show explicit blocker. |
| Permission or runtime removed | Block new work; cancel only unstarted denied tasks, preserving history and ordinary queue progress. |

The current two-minute job lease is scheduler ownership only. It cannot supersede the ordinary task lifecycle or the operation's uncertainty. Persist attempts across process restarts. Do not introduce a general workflow engine or use issue comments as the job queue.

### 4. Trusted observations without a server Git credential

All Git mutations stay in the selected model/runtime: its tools perform commit/push, PR/MR create, explicit audit note synchronization and approved merge. Add a read-only observer in that run's daemon plumbing. The observer independently fetches remote facts using the selected execution environment's Git hosting credentials; the server never calls Git and never receives the credential.

The Agent may request observation through a narrow local CLI command, but may not supply the receipt as authoritative JSON. Suggested surface: `multica project material operation <operation-id>` reads the scoped descriptor; `... observe <operation-id>` asks the local daemon to inspect. The model can provide a candidate PR/MR locator, treated only as a hint and checked against server-owned repo/branch/operation identity. The local daemon retrieves the frozen expectation independently, not from the Agent's request body.

A separate server daemon callback, such as `POST /api/daemon/tasks/{taskId}/material-observations`, accepts receipts only for the admitted material task/runtime/claim generation. A host-only challenge is retained inside daemon state, not exposed in the task prompt, local command output or model credentials. The normal `mat_` task token, arbitrary human POST, final-output JSON or generic `/complete` payload cannot establish a trusted observation. Apply existing daemon workspace/runtime authorization plus the exact attempt binding; do not rely on an opaque operation UUID alone.

Required receipt data:

- schema version, operation/attempt ID, frozen expected-descriptor digest, task/runtime ID, claim generation and host challenge;
- observation start/end times, selected repository/host stable identity, source/target branch and HEAD;
- `source_complete=true` only after complete pagination/tree/blob reads; full changed-file array with path, previous path, status and regular-file mode;
- bounded full expected file bytes and their SHA-256 values at the exact source/merge commit, so the server recomputes digests and compares with frozen revision text rather than trusting a model-provided digest;
- real review number/URL/state, native gate state and scoped block reason, real merge commit and actor if merged;
- explicit observation outcome (`observed`, `not_found`, `blocked`, `unavailable`), never interpret unknown or provider404 as success;
- a canonical receipt digest for immutable duplicate detection. Credentials, raw auth headers and provider error bodies are excluded.

`internal/materialgit` already contains tested `Observe`/`ReadFile`, full pagination, path/mode and native-gate parsing. Reuse those algorithms behind a read-only interface in the runtime host if practical. Its current `New(Connection)` requires an explicit token; this is not permission to fetch an App token on the server. A host-only resolver or bounded `gh api`/GitLab CLI read adapter must source credentials from the selected environment. Extract/read-only capability must not accidentally expose `Publish`, `SyncDecision` or `Merge` as daemon auto-executed actions. Do not pass model-controlled executable paths, shell fragments, API hosts or authorization headers to this observer.

Host attestation establishes what the trusted configured daemon observed, not cryptographic proof against a malicious machine owner. A second LLM observation task on the same host is not an independent trust boundary. It can aid recovery, but cannot replace deterministic remote readback.

### 5. Stage and approval flow

1. **Publish:** freeze candidate first. Selected runtime gets only the publication stage. After its Git write, host readback proves expected branch/review identity, complete allowed changes and exact candidate bytes. Only then expose `in_review`.
2. **Decision synchronization:** a real human decision binds current revision/HEAD. Selected runtime posts an explicitly attributed audit note; observer verifies marker/body/identity. Rejected/withdrawn decisions do not grant merge work.
3. **Merge preflight:** observer reads current remote HEAD/target/full changes/gates; server checks the original current human approval and issues one short-lived operation authorization bound to that observation. Selected runtime receives the merge stage after this check, with exact-head instruction and no admin bypass.
4. **Merge result:** host readback independently sees actual merged state, merge actor, exact target and immutable approved bytes. Server revalidates config/approval/revision and consumes receipt once. `queued`, command exit0, approval or matching prose is never merge completion.
5. **Plan handoff:** remove `processMaterialPlan`'s server `ResolveProvider` read. Create the existing visible todo plan issue immediately after verified baseline. If automatic planning is off, remain waiting with no Agent/model dispatch. If requested, dispatch a runtime observation/preparation bound to this plan issue/baseline; only a valid readback can authorize the selected runtime's planning launch. The readback itself does not release an old manual hold.
6. **Plan output:** selected runtime submits revision through existing API/CLI; source baseline and exact current task remain enforced. Its design/task files follow the same publish/human-review/merge stages; plan approval still does not launch code.

Keep preflight and admission transactions short; network observation occurs outside database locks. On return, lock workspace/project, current job/attempt/decision and relevant runtime/principals and compare exact frozen identity. At task-first claim/finalization boundaries preserve existing project/member/runtime `NOWAIT` patterns to avoid inversions.

Review or runtime authority revoked after dispatch must deny new local-operation permits, observations being promoted, claims and subsequent stages. An already delivered credential cannot be revoked by changing a DB row; the product must not claim an absolute external shell sandbox. If a model already has unrestricted remote Git authority, prompt instructions alone cannot guarantee it never directly merges out of band. Such a merge must remain unapproved/unaccepted in Multica; strict prevention needs remote policy or credential/process confinement beyond this API protocol. Do not weaken this distinction in requirements or tests.

### 6. Bounded observation/recovery

No new model process on every existing three-second material-worker tick. One outstanding operation attempt per job. Native checks pending should record a blocked/waiting observation with `next_at`; bounded observation on the selected runtime may resume remote-read checks, then wake the selected model's merge stage once current policy is satisfied. A read-only watcher never performs the merge itself. Its capability/lease and maximum deadline must be part of the operation contract, and offline/unavailable must be visible.

After an uncertain mutation, retain original operation key, revision branch and admitted decision ID. A reconciliation attempt first uses readback to determine whether the side effect already exists. It only grants a fresh mutation attempt following proof of absence or a clearly recoverable partial result; it does not blindly replay writes. Keep GitLab branch-only ambiguity as an explicit blocker when ownership cannot be established.

### 7. Configuration and historical compatibility

- New settings require `execution_runtime_id`; remove `connection_id`/`plan_agent_id` from new input, execution binding and UI. Keep response parsing defensive for installed clients.
- Legacy settings missing a runtime return an explicit `material_runtime_required` configuration state and are not dispatchable. Do not infer a runtime from an old plan agent or borrow a developer shell's current login. Human config save selects a runtime and increments config version.
- Frozen old revision settings/decisions remain historical evidence; never rewrite them to make them look runtime-approved. Pending unmerged legacy candidates block with configuration-changed/migration-required and require deliberate reissue/review under runtime configuration.
- Existing merged baselines keep their historical source, path, digest and verified-at. A no-App runtime readback can revalidate an existing baseline before a newly authorized plan attempt. Changing the execution method alone must not silently replace Git identity or create a second baseline. Runtime/config changes invalidate old queued operation authority; history remains visible.
- Stop/delete the old direct provider wiring for this material workflow. Ordinary GitHub App/VCS integrations elsewhere remain independent features; user scope does not require deleting all GitHub integration code.
- Hidden system-carrier deletion cannot cascade away plan/operation history. Before generic runtime teardown, detach material carriers under the existing lifecycle locks and retain operation/runtime snapshots. Do not copy builder session deletion's task-row cleanup.
- Use additive migrations with no foreign keys. Each new index is a separate `CREATE [UNIQUE] INDEX CONCURRENTLY` migration. Suggested unique keys: operation ID, task ID, `(job_id,attempt_no)`; active-attempt serialization under project/job locks. New SQL and generated output have one owner.

### 8. Owning files and implementation order

| Owner/lane | Write scope and reason |
| --- | --- |
| State/backend | `internal/material/types.go`, `internal/service/material*.go`, material handlers/capability, material SQL/migrations/generated: settings, dispatch state, current authorization and receipt consumption. Remove `handler/material_provider.go` material-only credential resolver. |
| Runtime | `pkg/protocol` material descriptor/receipt, daemon types/client/claim-preparation/local observer/lifecycle, CLI material operation commands: ordinary selected-runtime execution plus trustworthy readback. |
| Shared frontend | core materials schemas/queries and views settings/detail/locales: direct runtime selection and honest queued/running/blocked/configuration-required state; remove App connection dropdown. |
| Shared review | `task.go`, `task_start.go`, `issue_task_transaction.go`, daemon claim handlers and runtime teardown: explicit ownership/coordination because both state and runtime lanes touch these gates. |
| Docs/tests | built-in material skill/source map, owning spec, task artifacts and fixture runner: new runtime contract, no-App regressions, evidence limitations. |

Freeze settings/descriptor/receipt JSON and task-kind identity first. Then backend state + runtime transport can work in parallel; frontend uses that contract. Integration must test old authority-denial and manual-hold regressions together with new runtime stages.

### 9. Required tests

| Boundary | Assertion |
| --- | --- |
| No App/server credential | Both GitHub and GitLab settings save and stage dispatch with empty App/VCS config and no installation/connection rows; no server provider network request. |
| Selected runtime | Exact runtime and internal carrier; changing runtime/profile/owner/config denies stale tasks, no shared user Agent mutation. |
| Stage authority | Model cannot decide human approval or obtain merge stage before current approval; withdrawn/expired reviewer denies dispatch and receipt promotion. |
| Observation provenance | Fabricated model JSON, direct task-token callback, wrong runtime/task/generation/nonce/digest, duplicate changed body all refuse. |
| Completeness | Partial diff/pagination, changed target/HEAD, added file, symlink/mode mismatch, truncated body and hash mismatch never become review/baseline. |
| Recovery | Lost publish/merge reply, duplicate observation reply, worker crash, expired scheduler lease and live task produce one logical remote effect and no concurrent mutation attempt. |
| Native gates | Pending/unknown native policy remains blocked; later complete observation authorizes only selected runtime merge stage; watcher cannot mutate. |
| Plan handoff | Auto-off creates todo and zero model starts; re-enable does not drain; explicit start reuses issue and verifies exact baseline via runtime observer without server token. |
| Plan retry/output | Failed terminal run reuses issue, retains history; old task cannot submit; active and reviewed/approved plan cannot rerun. |
| Legacy settings/history | Missing runtime produces repairable configuration state; historical approvals not rewritten or silently replayed; merged baseline readback remains traceable. |
| Ordinary task regressions | Denied material work cannot starve normal queue; transient lock/DB failure rolls back without cancelling; cleanup preserves unrelated tasks. |
| Runtime test fixture | Fake selected-runtime executable performs real isolated local fixture writes, host read observer validates them; a pass cannot be obtained by injecting the final receipt or manually filling the UI. |

## Related specs

- `.trellis/spec/server/git-materials.md` — existing immutable material and manual-plan contracts, to update after implementation.
- `.trellis/spec/server/task-api-capabilities.md` — task/resource ceilings and the explicit distinction from shell/host credentials.
- `.trellis/spec/server/security-boundaries.md` — human decision and originator identity.
- `.trellis/spec/server/controlled-artifact-execution.md` — durable claim/uncertain result and actual trusted output validation; use concepts, not its unrelated tar policy.
- `.trellis/spec/server/database-test-isolation.md` — fresh owned DBs per package and separate Redis/runtime state.

## External references

No new external API fact is required for this internal design. Existing adapter READ semantics and prior official provider references remain in `internal/materialgit/README.md`. Host credential resolution must be checked against the actually installed gh/glab interfaces before implementation; do not invent a CLI token-extraction command or change authentication during planning.

## Caveats / Not Found

- Current code has no material operation attempt table, runtime observation receipt protocol or no-App plan-baseline preparation path. These are required implementation work, not configuration switches.
- The ordinary daemon `/complete` callback and native model output are insufficient remote evidence. Readback adds a trusted-host boundary but cannot protect against a malicious host owner or unrestricted ambient Git credentials.
- Exact command names, descriptor JSON types and polling deadlines above are design recommendations to freeze with the main session; no code was changed and no tests/model/DB/remote operations ran in this research lane.
