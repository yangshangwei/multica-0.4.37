# Research: Minimal real-model knowledge framework and preregistration

- Query: What is the smallest test-only extension that can execute the registered K1/K2/K3 contracts without changing runtime RK1 trials, product policy or funding?
- Scope: internal; future implementation proposal, not an executable registration or approval
- Date: 2026-11-02

## Findings

### Recommendation and current gates

Build a knowledge-only sibling of RK1 that composes the existing router fixture, HostExecutor/Runner, Provider capture and ordinary daemon execution. Keep all 17 existing rk1_*_test.go files unchanged. The finite proposal is K1 = 4 measured launches, K2 = 2, K3 = 8; ordinary E0b = 1 is a separate prerequisite. Thus the proposed knowledge set is 14 launches, and the proposed set including E0b is 15. Neither number is an allocation: knowledge allocated/consumed remain 0/0, paid source binding remains empty and runnable remains false. Original remaining14 and runtime cap25 are different ledgers.

The executable runtime table contains only E0a/H1/H2/H2b/H3a/H3b/H5/H6 (server/cmd/server/rk1_scenarios_recall_test.go:223). newRK1Fixture explicitly sets both TaskService and CollaborationService knowledge false (:75,86). The current wrapper requires MULTICA_COLLABORATION_RUN_ID (rk1_provider_test.go:69); it cannot capture an ordinary task by assigning it a fake collaboration run ID. run_rk1.py:269–271 selects only runtime-addendum scenarios. Those are specific extension seams, not evidence that the existing runtime framework is absent.

Read-only hashing reproduced the current 17-file framework digest 5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9. The companion entry-matrix.json retains 21 source-file digests. These are file inspections, not tests or a new final-candidate binding.

The shared complete-check owner is thread 01a19c0d-cce1-73e3-99ee-2c3c9e9239ba, task 10-30-collaboration-fix-closure; its final4 worktree, Redis16422 and renderer19845 are outside this proposal's execution resources. Its actual complete-check result remains the prerequisite described in #7 advance-plan-2026-11-02.md, section3. No duplicate full check is proposed.

### Current boundaries that the fixture must preserve

| Boundary | Current contract and source |
| --- | --- |
| Startup switch | Literal true at router construction, copied to the router-owned TaskService; default false, not a live DB toggle. .trellis/spec/server/collaboration-knowledge-execution.md:36–41 |
| Human producer | Authenticated human source creator/current handler proposes; human source Read plus current target confirmer role approves the exact state/version/hash tuple. Candidate content/source/target/expiry are immutable. .trellis/spec/server/collaboration-knowledge.md:41–78 |
| Ordinary consumer | Server reconstructs task originator, runtime.OwnerID, input-comment authors, real issue/Chat/autopilot/quick-create scope and audience. AccountableUserID is not a principal. task_knowledge.go:197–296 |
| Eligibility | Only five applicability surfaces exist: issue, chat, quick_create, autopilot, collaboration. Every originator/owner/input author must read the source; workspace output requires the same source workspace; private Chat verifies creator/agent; a different collaboration issue also checks its actual audience. collaboration_knowledge_resolve.go:231–445 |
| Ordinary transport | Authenticated MDT claim, separate prepare, separate authorization for each physical process, raw native observation with canonical ACK, original fence on start/pin/complete/fail. .trellis/spec/server/collaboration-knowledge-execution.md:17–30,157–195 |
| Ordinary persistence | Exact provider/runtime/agent/configuration/scope/generation/native binding; complete cumulative history. Valid history can resume; invalid history detected at a supported new claim selects fresh. task_knowledge.go:400–415,463–587 |
| Host persistence | Every knowledge-granted run has a unique run namespace/cwd/ledger and --no-session-persistence before fingerprinting. No reusable native pointer. Preparation/authorization use MDT; business uses MCB; termination/proof/status use MCT. .trellis/spec/server/collaboration-knowledge-execution.md:202–231 |
| Host input | Original accepted input and frozen handoff remain verbatim; the approved knowledge JSON is appended as data. host_knowledge.go:72–100 |
| Refusal | Exact-manifest replay cannot replace revoked content with today's set. Before-first-process refusal starts zero processes; later refusal retains prior evidence and never replays business input. task_knowledge_prepare.go:404–418,512–527; execution spec:186–198 |
| Accounting | Physical underlying Provider starts are counted even on incomplete results; fake/stand-in starts are separately labelled and cost zero paid calls. Runtime ledger/numbering is not reused. runtime-addendum.json:118 and registration.json stop/window rules |

### Producer and consumer entry map

The precise eight-row machine-readable map is entry-matrix.json. It is supportable as eight producer/role samples after the new fake proof exists. It is not eight distinct knowledge API contracts, eight UI journeys, or currently eight executable knowledge scenarios.

| K3 row | Actual producer | Consumer and trusted setup |
| --- | --- | --- |
| issue | Human B POST /api/issues, agent-assigned active issue in W/P; CreateIssue → IssueService.Create → maybeEnqueueOnAssign → EnqueueTaskForIssue. handler/issue.go:3187–3239; service/issue.go:368,860–883 | Ordinary issue scope; task originator from actual issue/trigger attribution; local Claude runtime owned by A. task_knowledge.go:206,221–230 |
| comment | B POST /api/issues/{I}/comments with explicit agent mention on an initially unassigned active issue; CreateComment → triggerTasksForComment → enqueueSingleCommentTrigger/EnqueueTaskForMention. handler/comment.go:1910–1914,1955,2568–2594 | Still surface=issue and scope=I; actual member TriggerCommentID author plus all coalesced authors must read S. task_knowledge.go:263–283 |
| Chat | B POST /api/chat/sessions, then /{chatId}/messages; SendChatMessage → SendDirectChatMessage. handler/chat.go:40,828–941 | Private creator-only Chat in W/P; B can invoke A's agent; raw task.ChatSessionID, creator B and runtime owner A remain distinct. resolver:286–305 |
| quick-create | AI branch POST /api/issues/quick-create → QuickCreateIssue → EnqueueQuickCreateTask. handler/issue.go:2607,2802–2818 | Genuine quick_create task with no issue/Chat/autopilot at initial claim, context built by server and OriginatorUserID=requester B. task.go:1648–1738; task_knowledge.go:245–256 |
| autopilot | B POST /api/autopilots/{id}/trigger with fixed Idempotency-Key; TriggerAutopilot → DispatchAutopilotManualWithKey → dispatchRunOnlyOnce. handler/autopilot.go:2465–2501; service/autopilot.go:1045–1087,1177–1196 | Freeze active run_only autopilot in W/P, assignee agent A. Originator is manual human B, not rule publisher/installer/accountable. Schedule/webhook and create-issue mode remain separate unmeasured branches. |
| root | Register/consent an actual target, then human POST /api/collaboration/issues/{C}/comments with authorized agent/squad mention, target_versions and execution_view_version=6; control/resume through /control. router.go:1700,1712,1721,1726 | Leader of a frozen squad target, resource owner A, original input authors A/B and actual audience all read S. claim/handoff/knowledge must report role=leader. |
| member | Original root MCB POST /api/collaboration/runtime/runs/{root}/comments mentioning the frozen member → CreateRuntimeComment → createDerivedRun(...,"member",...). collaboration_squad.go:220–278 | Actual delegated member run, ParentRunID=root, frozen member agent/owner A, original human input authors preserved; fresh run knowledge namespace. |
| continuation | Actual member comment/proof causes queueContinuation → createDerivedRun(...,"continuation",...) → runtime list/claim. server/internal/service/collaboration_squad.go:286,363–380; server/internal/service/collaboration_runtime_capabilities.go:607 | Actual frozen leader continuation, new run ID, original human authors and real reader/wakeup links. There is no separate human continuation-launch HTTP endpoint. |

Root detail matters: existing RK1 f.submit calls service.SubmitCommand directly (rk1_fixture_test.go:212–216). There is no current POST /commands submission route. The public producer is CreateCollaborationComment → service.CreateComment → AppendCollaborationInputInTx (handler/collaboration_comments_create.go:22–75; service/collaboration_comments_create.go:40–100). Use that existing producer when claiming HTTP/client entry coverage.

Client distinctions: createIssue uses /api/issues (packages/core/api/client.ts:2899–2915); AI quick-create uses /api/issues/quick-create (:2922–2935), while the quick-create modal's manual form shares ordinary issue creation. The comment-anchored AI branch uses /api/comments/{id}/sub-issues (quick-create-issue.tsx:510–526) and is explicitly excluded from the eight-row minimum. Floating/full-page Chat shares the send API (use-chat-controller.ts:581). Root/member/continuation share runtime claim/prepare/authorize; their distinguishing evidence is admitted provenance and role, not invented routes.

### Fixed materials and actor setup

Use fixture-created human A and B, one selected permitted Provider account later, and only task-owned users/resources. No two-Provider-account acceptance is inferred. Human fixtures are not Provider accounts.

For K1 and the K3 workspace/host samples, use a source collaboration issue S in W, with A as creator/current handler and target owner/admin (or an explicit human project lead), and B with current source Read. Use an independent consumer issue/Chat/task C in W/P, a built-in local Claude runtime owned by A, and an agent B is explicitly allowed to invoke. All accepted authors and the collaboration audience must read S. No channel-backed output or agent-authored ordinary trigger is used as a positive consumer.

Source memory version1 contains a fixed inert review anchor; candidate facts are authored and approved through human HTTP. The candidate's exact body/hash/source/version and expected semantic fields are frozen before real access. Keep decisive facts out of consumer base description/context, original task prompt, nonknowledge handoff sources, fake-seed output and real-child plan/environment. The real wrapper must not receive fake expected answers. Candidate/source IDs and raw body-bearing traces remain private.

Suggested bounded material: v1 says canary7%, wait90s; v2 corrects it to canary11%, wait120s, with a per-sample code. Expected output names approved version, canary, wait and source anchor; no-approved output is POLICY_UNAVAILABLE. Before real calls, freeze the complete text, generated IDs/codes, hashes and scoring rubric in the new framework registration. These values are a proposal, not measured facts.

### K1: one trial, exactly four measured calls

| Label | Committed state before new claim/preparation | Fresh consumer and expected evidence |
| --- | --- | --- |
| RK1-K1-t1-pending | v1 pending, no approved lineage | New C1/run/host cwd/ledger; positive knowledge negotiation with empty contribution manifest. Actual stdin/tool/brief excludes v1 facts; semantic POLICY_UNAVAILABLE. |
| RK1-K1-t1-approved-v1 | A approved exact candidate/state revision/version/hash via human HTTP | New C2/run/cwd/ledger; knowledge manifest and actual stdin contain v1 exactly once, correct source/version/hash. Model reports v1 policy. |
| RK1-K1-t1-approved-v2 | New immutable v2 proposal stays pending while v1 remains the target contribution; then exact v2 approval atomically supersedes v1 | Pending-v2 retention is an explicitly registered human target GET/state assertion, no Provider call. New C3/run/cwd/ledger gets only v2 in the knowledge section; model reports v2 policy. |
| RK1-K1-t1-withdrawn | A withdraws approved v2 with a new request UUID and exact current state revision before C4 claim | New C4/run/cwd/ledger, empty current manifest, no v1/v2 facts in any decisive input. Semantic POLICY_UNAVAILABLE. Retain previous authorized disclosures/receipts. |

All four consumers are distinct collaboration issues, not just four cwds in one issue. Otherwise previous lawful Provider-result/handoff text can carry the answer into the withdrawn case despite correct current filtering. Each uses --no-session-persistence, including the pending/withdrawn empty set. No ordinary native session is reused for this schedule.

Withdrawal is the recommended real fourth-state policy; expires_at=null for both v1/v2. Expiry remains a named fake-first negative control, not a claimed real expiry observation. The registry says withdrawal/expiry, not both independent real experiments. A real expiry alternative must be selected and frozen before calls, using a future immutable UTC expiry and recording the actual clock boundary; it replaces the fourth state, never adds an unallocated fifth call or changes TTL in place.

Separate zero-Provider controls use an already prepared run followed by withdrawal/expiry before authorization; they must refuse, rather than silently replacing the manifest with an empty set. CCG1 and the host composite supply the existing pattern (server/cmd/server/collaboration_knowledge_correction_composite_test.go:120–136; server/cmd/server/collaboration_knowledge_host_composite_test.go:599–734).

### K2: one trial, two measured calls and zero-launch refusal/recovery

Use private Chat to isolate source Read revocation from workspace output permission. Source S is in W_source; consumer Chat/project and target are in W_target. A creates/handles S and can normally see W_target. B is a target owner/admin, Chat creator and runtime/agent owner in W_target, and an ordinary invited source participant with no W_source membership and no creator/handler role. B initially approves the exact candidate with source Read. A can remove B's source participation without changing B's target account/credential/configuration. This intentionally uses B as both originator and owner; separate-principal omission/refusal remains a fake boundary matrix.

| Step | Schedule and required observation | Paid launches |
| --- | --- | --- |
| Original decision key | Drop one committed B approval response; recover with candidate ID plus original request ID. Preserve exact payload/hash. Never mint a replacement candidate/action key. | 0 |
| RK1-K2-t1-fresh | Healthy task T1, fresh Chat logical generation; approve α. Real prepare/authorize, one raw native S1 observed and positively ACKed before pointer publication. | 1 |
| RK1-K2-t1-resume | While α remains eligible, approve distinct β from the same source; enqueue T2 in the same Chat. Claim chooses the exact S1/generation/configuration. Drop one committed T2 preparation response; the daemon's bounded retry sends the identical serialized request UUID/hash and gets the same preparation/session/manifest. Actual argv resumes S1, actual raw native observation remains S1, current manifest α+β and complete retained history union agree. | 1 |
| Valid third binding | Enqueue T3 while B still has source Read; finalize a valid S1 claim and complete preparation with original R3. Pause at the real authorization boundary before any third process. | 0 |
| Revocation | A POST /api/collaboration/issues/{S}/participants/{B}/remove. Confirm the mutation committed and B is no longer a source member/participant, while B's target/runtime/Chat authority and invocation configuration remain unchanged. | 0 |
| Refuse frozen launch | Resume T3 authorization with its unchanged identity, manifest, prompt hash and attempt nonce. Cumulative S1 history now fails revalidation: expected409 knowledge_context_changed (task_knowledge_prepare.go:404–418). Task settles with its original fence; no third wrapper/real start, automatic fresh fallback, requeue or business-input replay. | 0 |
| Original-key recovery | Same R3 prepare replay must refuse without body; B's original candidate approval GET operations receipt remains content-free after source-role loss. Earlier lawful T1/T2 history is retained. | 0 |

Important timing: revocation before a supported new claim legitimately yields an empty native pointer and a fresh generation, not a universal refusal (task_knowledge.go:463–478). That fresh decision can be checked without launching another Provider. Never force it to resume S1 or count a new fresh launch as the registered refusal.

This real schedule covers two native-session turns plus a new turn's before-first-process refusal. A later physical retry of the same logical input is covered by the named fake composite, which retains prior usage/messages and confirms exactly one side effect (knowledge_server_integration_test.go:683–718). Do not report that as an additional real retry experiment. Every HTTP drop is bounded to one committed response and named in the pre-registration; ordinary transmission retries do not change business identity.

### K3: one trial, finite eight-call table

Freeze the order issue → comment → Chat → quick-create → autopilot → root → member → continuation, with one measured underlying Provider process per row. Each ordinary row uses a new task/scope; each host row uses an independent measured run namespace. No row may launch extra recovery/health/model calls.

The smallest isolated member sample uses one test-owned fake root prefix to create the real admitted delegation. The isolated continuation sample uses fake root+member prefixes to create real wake/proof/reader history. These are 3 explicitly labelled fake seed starts, 0 paid starts, never fabricated process proof. Seed stdout/records must omit the decisive approved policy. This borrows the existing H2/H2b pattern (rk1_scenarios_handoff_test.go:96–110); it proves each measured role after a synthetic prefix, not an entirely real three-agent conversation.

For ordinary issue/comment/Chat/autopilot, require the current compiled CLI to read the ordinary project through 'multica project get P --output json' once, with actual task-scoped credentials; the approved knowledge still comes from actual stdin, not that GET. This command exists at cmd_project.go:30,264–280. The fake proof must establish its task-token authorization, without a PAT/human fallback. Quick-create additionally performs one inert 'multica issue create --title ... --description ... --project P --status backlog --output json', no assignee, and preserves the created issue/task link; flags are at cmd_issue.go:530–545. A failed/link-changing task contract is a prototype blocker, not permission to weaken the observation/fence checks.

For host rows, require actual run-scoped 'collaboration context'/'handoff' CLI calls, exact admitted role/parent/readers/wakeup causes, human authors and fresh namespace. Model behavior and current application input are separate verdicts. A server response or fake answer alone never proves the real model saw or used the material.

### Smallest owned implementation seams

Future implementation in a newly isolated owned worktree; no code is changed by this report.

| Proposed new file | Responsibility |
| --- | --- |
| server/cmd/server/rk1k_fixture_test.go | Construct a positively granting knowledge router from newCollaborationRouteFixture; compose existing rk1Fixture target/control/Host helpers without calling the knowledge-off constructor; build true human producer HTTP setup and owned source/target/audience rows. |
| server/cmd/server/rk1k_provider_test.go | New basename-dispatched wrapper/stand-in, before TestMain; accept actual ordinary MULTICA_TASK_ID or actual collaboration run identity, never alias one into the other. Reuse bounded frame/capture/finalizer/child-wait primitives. Preserve ordinary business environment; do not apply Host environment replacement/filtering to ordinary tasks. |
| server/cmd/server/rk1k_scenarios_test.go | Frozen four/two/eight schedules, eight producer/role dispatch entries, E0b distinct entry, finite response-drop and revocation barriers; no dynamic N or product behavior change. |
| server/cmd/server/rk1k_evidence_test.go | Discriminated ordinary-task versus collaboration-run observations; reuse HTTP trace/hash/capture publication. Record native facts/fences and Host proof/records under their actual types. Never fabricate a CollaborationRunIdentity for a task. |
| server/cmd/server/rk1k_entry_test.go | Fake-proof and guarded real entry; exact table/budget/order/cursor/stop validation. Real entry cannot pass admission while the knowledge allocation is0/runnable=false. Hash new files plus their unchanged RK1 dependencies and the tagged driver. |
| server/internal/daemon/rk1k_driver_agentintegration.go | Narrow agentintegration && !windows bridge: compose Daemon.New, authenticated Client.ClaimTask, existing runTask and reportTaskResult, retain one daemon/configuration across K2 turns. No full Daemon.Run/auth resolution/GC/update loops, forged task fields or alternate lifecycle. Excluded from default builds. |

The daemon bridge is needed because runTask/reportTaskResult and the existing ordinary fixtures are package-private; cmd/server cannot import another package's _test helpers. The exact current composition is knowledge_server_integration_test.go:281–337; Daemon.New:636–688 supplies the production invocation callback. A new tagged bridge avoids exporting a production API or copying the daemon execution implementation.

Five new rk1k_*_test.go files do not match the old rk1_*_test.go hash glob (rk1_entry_test.go:296–313). Their new initialization must explicitly leave the old rk1-provider/rk1-sim-claude basenames alone. Future source binding nevertheless changes because the product manifest includes test files; unchanged old framework hash is not unchanged full product source.

After local proof and independent review, the coordinator owns a small knowledge-only launcher/registration artifact under the existing #6 task. It must admit this fixed table with a separate ledger and source/framework/CLI/configuration hashes, still allocated0 and runnable=false until explicit funding/window authority. Do not mutate run_rk1.py's runtime budget or old registration/trials while implementing the local framework. If sharing launcher helpers requires changes to an old runtime file, that is a separately reviewed handoff, not implicit write scope.

### Meaningful fake-first validation for the future owner

Commands below are planned, never executed here. Each DB-backed package/normal/race invocation needs its own fresh recorded migrated database; cmd/server TestMain initializes the DB before a test-body skip. A name filter is not an isolation guard. Use a distinct knowledge-only prefix (for example multica_rk1k_) and private workspaces/build/CLI artifacts, not the runtime owner's DB or final4 resources.

~~~sh
env -u MULTICA_RUN_REAL_AGENT_SMOKE DATABASE_URL="$TASK_KNOWLEDGE_NORMAL_DB_URL" \
  go -C server test -tags=agentintegration ./cmd/server \
  -run '^TestRK1KnowledgeFakeProof$' -count=1 -parallel=1 -v
env -u MULTICA_RUN_REAL_AGENT_SMOKE DATABASE_URL="$TASK_KNOWLEDGE_RACE_DB_URL" \
  go -C server test -tags=agentintegration ./cmd/server \
  -run '^TestRK1KnowledgeFakeProof$' -count=1 -parallel=1 -race -v
env -u MULTICA_RUN_REAL_AGENT_SMOKE DATABASE_URL="$TASK_KNOWLEDGE_GUARD_DB_URL" \
  go -C server test -tags=agentintegration ./cmd/server \
  -run '^TestRK1Knowledge(EntryGuard|WrapperGuard|EvidenceFaults)$' -count=1 -parallel=1 -v
~~~

Required useful negative controls: wrong but complete semantic answer remains a valid failure; stale version/body present in any decisive input prevents a filtering pass; unavailable/revoked/busy exact preparation starts zero processes; missing/changed native ID or noncanonical ACK never publishes a pointer; missing stdout/write/sync/close/termination/seal makes the sample incomplete and stops the next numbered hop. Test the wrapper's real pass-through branch using only a test-created stand-in, and trap ambient executable/config/token discovery. Verify real-child mode contains no fake answer/source material.

Actual named normal/race results, all eight row identities, four/two/eight measured counters, fake seed counters, raw hashes and a separate independent review are required. Compilation, [no tests to run], a test-body skip or a fake expected-answer match alone cannot close a row. Broaden checks only for new changes/failures or unresolved review concerns; do not rerun the sole-owner full check.

### Future account, window and evidence admission

After local proof exists, freeze final source/overlay and all new/borrowed framework files; API/daemon binaries, copied wrapper and real CLI executable/version; CLI build; selected private DB identity; actual configuration fingerprints; requested versus output-observed model; provider-default thinking/tier; allowed nonsecret account/gateway identity; all run/task/session/attempt/output IDs and private artifact paths. CLI version and configured model are not observations of the effective model.

The retained registration names macOS arm64, Claude CLI2.1.316 and claude-opus-5-5 with a gateway model mapping. Those are historical inputs to reverify under future authority, not account inspection performed here. Do not manufacture a replacement account/profile to satisfy isolation: K2's ordinary native store must remain stable across its two calls, whereas Host persistence is disabled. Capture wrapper and underlying executable identity separately.

Only after an actual upstream recovery basis, applicable explicit funding/account permission, final binding and exclusive serial gateway window exist may the coordinator propose the exact reviewed calls. No coding/review model calls, original-series run, heavy tests or migrations overlap that window. First incomplete environment/framework/runtime result stops all later numbered calls; preserve its consumed starts and never fill the slot, extend timeouts, switch models or add a health call. An expected zero-launch refusal is a tested contract, not an unexpected environment fault. A complete wrong answer is a valid sample-level failure.

Per-call evidence must independently bind: human producer receipts; source-free claim; private committed preparation and exact sorted manifest; actual prompt/first frame/argv/cwd/environment fingerprints; per-process authorization and nonce; raw Provider-visible tool result and model output; ordinary native observation ACK/fence/usage/task terminal state or Host launch-start/termination/proof/record-seal/ledger facts; semantic criterion verdict; private/public/raw SHA-256 and measured real/fake start counts. Keep private bodies/credentials out of generic server/daemon logs and public records. Do not substitute an HTTP response, configured model, business success, process start or test exit0 for the fact it does not prove.

## Related specs and external references

- .trellis/spec/server/collaboration-knowledge.md — human immutable versions, source/target authority, original receipts and projections.
- .trellis/spec/server/collaboration-knowledge-execution.md — startup, consumer/namespace, exact session, per-process authorization and Host persistence.
- .trellis/spec/server/database-test-isolation.md — TestMain, fresh per-package DB, connection/registry ownership.
- .trellis/spec/server/collaboration-context-experiments.md — default-safe entry, private artifacts, cursor/stop and observed-model distinctions.
- .trellis/spec/core/frontend/collaboration-knowledge.md and .trellis/spec/views/frontend/collaboration-knowledge.md — exact human intent/leases and shared views; static UI wiring is not a browser journey.
- https://code.claude.com/docs/en/cli-reference.md — --no-session-persistence is cited by the repository's retained vendor excerpt and execution spec. No network lookup/download or current CLI/account verification was performed for this report.

## Caveats / Not Found

- Product entry/role contracts are wired; the proposed knowledge sibling, ordinary-aware wrapper, tagged daemon bridge and finite dispatch are not implemented or run. Proposed filenames/tests/commands are not existing passing evidence.
- K3=8 is bounded producer/role coverage only. Source-context quick-create, schedule/webhook/create-issue autopilot, channel Chat, batch/WS transports, a fully real squad prefix, Windows/installer/account quota matrices and UI→real Provider journeys are not covered by these eight calls.
- K1 selects withdrawal, not both independent withdrawal and expiry real observations. Its fresh consumer issues prevent lawful previous output from confounding the absence criterion.
- K2 revocation after a frozen valid binding must not be moved before claim; its same-input later-process refusal remains a fake control. Auxiliary Chat title/quick-action model generators must be unset in the fixture so they cannot create unregistered calls.
- New code must first prove ordinary CLI task-token access, quick-create issue-link/fence behavior, exact actor revocation and all process/evidence counters. A failing prototype gets a bounded owning module and reviewed repair; it never grants an extra call or a fallback.
- No implementation/check manifest, git operation, test/Go metadata, DB/service/browser, Provider/account/installed CLI operation or download was executed. Only this owned research directory was written.
