# Knowledge plan/code-seam review — 2026-10-31

**Current status — recheck 2026-11-01:** K1–K5 are resolved at design level by the revised drafts. The remaining items are named implementation/entry proof gates, including an unconditional server-side guard for capability downgrade; they are not a demand to demonstrate unimplemented code now. See the recheck at the end. Runtime dependency gates still apply.

## Original review — preserved evidence


Verdict: **revise before promoting the drafts to an implementation contract**. The approved product boundary is clear; no new permission decision is needed. The findings below concern missing or contradictory engineering contracts, not vulnerabilities in already implemented knowledge code (that code does not exist yet).

Reviewed inputs: `knowledge-design-draft-2026-10-31.md`, `knowledge-implementation-draft-2026-10-31.md`, the preflight, sibling knowledge PRD and the relevant current Go/SQL paths. The surrounding runtime worktree is still changing. Only this review document was written; no code, draft, spec or task state was changed, and no DB, Provider or test process was run.

## Findings requiring a plan correction

### K1 — Bind resume eligibility to the exact Provider session and its exposure history (P1)

The draft stores a per-task manifest (`design:42`) and says to inspect a possible prior task before reusing a session (`design:184`). That does not yet define the invariant needed by the actual resume selectors:

- `server/internal/handler/daemon.go:2714` resolves manual rerun from its exact source task.
- `:2756` uses `GetLastTaskSession`; `server/pkg/db/queries/agent.sql:1299` reduces rows **per session**, and its final projection returns `session_id/work_dir/runtime_id`, not the identity or knowledge exposure of one authoritative previous task.
- `handler/daemon.go:2850` can use `chat_session.session_id` directly, without a task lookup. `:2917` falls back to `GetLastChatTaskSession` only when needed. `queries/chat.sql:292` deliberately preserves the prior session pointer when a newer task reports no session ID.

A check of “latest task's manifest” can therefore validate a different task/session from the one actually resumed, especially after a failed turn without a new session ID, a cancelled task, a same-session multi-turn history or fallback to an older healthy session. Dropping one candidate from the newest payload does not remove it from that session's older context.

**Smallest correction:** make the resume result carry the exact `(provider, runtime, session ID, scope/generation)` provenance and a provable exposure manifest for that session. This can be a cumulative union of actual deliveries, or an explicitly maintained invariant that every resumed task's complete manifest covers all prior deliveries in that session. Resolve and validate it for the direct Chat pointer, fallback query and manual-rerun path; do not silently associate an arbitrary newest task. Retire or refuse that exact session when its historical contribution set cannot be proved eligible. Preserve the draft's limit: this prevents future unsafe reuse; it does not erase historical outputs.

Add named tests for direct Chat pointer + newer task with NULL session, older-session fallback, cancelled task, two tasks sharing one session, explicit rerun, and withdrawn contribution present only in an older session turn.

### K2 — Define claim-body disclosure and finalization as well as the later launch gate (P1)

`design:179` builds the body-bearing bundle in the claim builder and only specifies saving its metadata in `FinalizeTaskClaim`; `:181–186` puts current authorization and the stated linearization point at the later pre-launch endpoint. The daemon receives the private body **before** that later endpoint. The plan must say which consistent authorization decision permits that disclosure and how it is tied to the finalized claim.

Current `buildClaimedTaskResponse` is called before `FinalizeTaskClaim` (`handler/daemon.go:1867/1909`, `:3490/3549`). The existing finalizer (`service/task.go:3731`) validates password/runtime binding, locks the task and execution admission, and persists credentials/comment receipts; it has no knowledge-source transaction. Adding a manifest write alone does not close a source revoke/withdraw between bundle construction and final response.

**Smallest correction:** define a complete, consistent authorization point for the body-bearing claim read and bind its exact manifest to finalization. One implementation can revalidate candidate/source/consumer/audience state under a declared shared fence before response bytes are written; another can defer private body delivery to the authorized phase. An earlier transactional claim-read authorization point is also defensible if its identity, source versions and linearization are explicit and preserved—piecemeal unlocked reads plus a metadata write are not that contract. Keep the later per-launch check as a separate freshness gate. Describe the earlier daemon disclosure as a real authorized delivery, not only an unobservable preparation step.

The lock design must cover this path explicitly. `FinalizeTaskClaim` currently locks runtime/task before issue execution; workspace deletion is documented as `workspace → chat_session → agent_task_queue` (`queries/workspace.sql:68–80`). Naively adding a workspace/source lock after the finalizer's task lock introduces the exact inverse order that the current comment says is avoided because the finalizer never touches workspace. Reorder a single transaction or use an equivalent ordered/fail-closed fence; do not nest the human knowledge transaction inside the existing finalizer.

Add claim-finalization versus withdraw/source revoke and workspace-delete interleavings, including “bundle prepared, authority changed, finalize attempted” and lost HTTP response/redelivery. These are distinct from approve-versus-revoke and launch-versus-revoke.

### K3 — Replace the assumed running-task requeue seam with a concrete transition (P2)

`design:183` says a pre-launch `knowledge_context_changed` uses an existing before-launch/requeue path. The cited current path is not usable at that point:

- `daemon/daemon.go:7569` calls `StartTask` before creating the final execution options and invoking the backend.
- `:8434–8452` (`executeAndDrain`) returns backend/launch errors to the ordinary failure path.
- `TaskService.RequeueTaskAfterClaimFailure` (`service/task.go:3852`) is specifically for a response that failed before delivery. Its SQL (`queries/agent.sql:946–962`) requires `status='dispatched' AND started_at IS NULL`, plus exact dispatched time and claim generation.

**Smallest correction:** specify a bounded, fenced refresh/requeue or terminal refusal for a running-but-never-launched task, with an exact ownership/claim-generation check and no duplicated business input. Distinguish the first physical launch from an adapter's later retry after a process may already have produced effects; a later authorization refusal cannot automatically replay the entire task as if it had never run. Keep normal cancellation, accounting and final failure reporting intact.

Add real ordinary-entry fake tests for the first-launch change and for change between two physical launches, not only a resolver/observer unit test. Also correct the draft's stale names: the current seam is `agent.ExecOptions` and `executeAndDrain`, not `agent.Options` / `executeWithBackend`.

### K4 — Apply the chosen source-purge rule to human candidate reads, not just injection (P2)

`design:72` permits source candidate body/version-chain reads in any state with only current source Read. Approval and runtime resolution separately test source availability. That leaves human list/detail/target projections underspecified after a referenced memory version or comment is purged/deleted: the candidate's immutable `content` and `summary` may reproduce the cleared text.

The preflight's required source-clearability test and the implementation plan say the candidate copy must not bypass a cleared source. The existing memory path preserves metadata while suppressing purged content; source membership by itself does not make a purged version readable again.

**Smallest correction:** state one shared availability policy for source list summaries, full candidate/version detail and target contribution bodies. When a required source version is no longer available, suppress derived body/summary as required by that policy and retain only permitted audit/control metadata. This need not physically erase the immutable candidate or invent a new cleanup operation. Make the intended distinction from ordinary withdrawal/expiry explicit so legal historical records are not accidentally erased.

Add human list/detail/target tests after source purge/delete, in addition to the already planned resolver and next-input tests.

### K5 — Make content-free recovery reachable for target-only withdrawal (P2)

The draft intentionally hides source identity from a target confirmer who no longer has source Read (`design:76–85`), and gives withdrawal a candidate-only URL. However, the dedicated lost-permission receipt endpoint still requires `/issues/{sourceIssueId}` (`:94`), while `:154` says callers without current action authority may recover only through that dedicated endpoint.

A target-only confirmer whose withdrawal succeeded with a lost response cannot construct that endpoint from the restricted control row. Reposting the write is not a substitute if the target role was also revoked after commit and the plan routes no-authority replay exclusively to the read endpoint.

**Smallest correction:** offer a content-free recovery key/address derivable from the permitted candidate ID + actor request ID (or an opaque operation reference saved before submission). Resolve the source internally and still enforce original actor ownership; do not reveal source ID to make recovery possible. Test lost-response withdrawal followed by source/target permission loss.

## Checks that support the chosen direction

- Separate managed contributions are justified: the existing ordinary claim/execenv path broadly injects `workspace.context` and `project.description`, and project text also has a sidecar. The draft correctly avoids splicing private contributions into those strings.
- `OriginatorUserID` versus `AccountableUserID` is correctly distinguished, and the draft does not invent a human authorization fallback. Runtime owner and output audience checks remain additional restrictions.
- The human-write lock proposal does not rely on a non-shared advisory lock: member revoke actually takes `LockSubscriberWrites` (`handler/workspace_revoke.go:54–66`), and `LockActiveMember` uses `FOR SHARE` (`queries/subscriber.sql:113–126`), which conflicts with the current ordinary role update. The project/source inverse ordering is acknowledged with NOWAIT/full-transaction retry. The separate claim-finalization graph in K2 still needs its own contract and tests.
- No extra generic knowledge service, LLM summarizer or dependencies are proposed. One candidate entity plus existing operation/event ledgers is a reasonable bound. Independent target revisions correctly avoid pretending workspace base text already has project-style CAS.
- Migration/index rules are explicit, including no automatic non-concurrent primary/unique index creation, separate concurrent index migrations and audit-preserving rollback. State CAS, immutable content, supersedes and human approval scope are covered.
- The draft correctly limits revocation promises to future consumption/reuse and does not promise to erase already delivered Provider history or historical output. Do not broaden that promise while fixing K1/K4.

## Routine implementation notes (not additional product decisions)

- Distinguish a proven legacy/no-managed-knowledge session from a negotiated session with missing/corrupt delivery metadata. Treating every old NULL manifest as unknown can unnecessarily reset all pre-feature conversations, despite the stated unchanged base behavior.
- `agent.HostLifecycleSupported` currently permits only local Claude/Codex/OpenCode (`pkg/agent/launch_lifecycle.go:128–146`). Negotiate the new knowledge capability per effective runtime/backend; unsupported backends must not have their otherwise valid ordinary task execution broken merely because the daemon supports the protocol globally.
- Preserve client/server cache and live-revocation tests for source/target views. A target knowledge revision alone cannot invalidate a source-ACL or expiry change; the draft already recognizes that for manifests and should carry it through Query invalidation/polling behavior.
- A client-supplied evidence `verification` value is an attributed claim unless the service independently establishes it. Keep verifier/basis provenance clear; approval itself must not turn a statement into server-verified evidence.

## Verification and handoff

- Source inspection: completed for the named paths; findings are plan/code-seam mismatches with concrete call/SQL evidence.
- Lint/typecheck/tests: **not run**, as this dispatch is plan-only and forbids product changes/DB/Provider work. These findings do not claim a new executable regression was run.
- Report formatting: `git diff --check -- <this report>` is the only command-level check required for the document.
- Next: amend the two drafts for K1–K5, have the main session review the corrected lifecycle/lock/visibility contracts, then promote them only after runtime dependency gates are met. No new user permission question is needed for these engineering corrections.


## Recheck — 2026-11-01

**Verdict: design-level pass, with explicit implementation-entry gates.** The main session's chosen contracts resolve the original plan gaps without changing the approved source-visibility policy. No new user question or historical-output deletion promise is needed. This is permission to formalize a plan once its runtime dependencies are met, not evidence that the knowledge feature already works.

Reviewed revised artifact hashes:

- `knowledge-design-draft-2026-10-31.md`: `a134910ad0a8a05e09504cca9da3d008c7c734567d0802265283b6247adcf16a`.
- `knowledge-implementation-draft-2026-10-31.md`: `fba0e297073b8b551c41814b0fc127a45182c1b08cb32fb41159e78249b5cbf2`.
- Original review before this append: `5dcca7b1e049b4a47d1bc8bcea6693189c01ac2dc650bf7dc4f30376b102ece9`; its findings and source evidence remain above as historical review.

### Resolved/open disposition

| Finding | Recheck disposition | Why it is resolved at plan level |
| --- | --- | --- |
| K1 | Resolved; exact-session and downgrade entry gates below remain mandatory | Data model and §7.4 now use the actual provider/runtime/native-session/scope/generation key and cumulative exposure, including older turns, cancelled/failed turns, pending resume bindings and NULL-session cases. Pointer writes and exposure binding are atomic. Proven legacy absence differs from corrupt negotiated history. |
| K2 | Resolved; implement the stated disclosure/finalizer contention tests | §7.1 sends only a source-free negotiated descriptor and writes no source/candidate body in `FinalizeTaskClaim`. §7.2 moves private body disclosure into a separately ordered NOWAIT/try-lock transaction, with current authorization and exact manifest recorded before response. Lost responses reauthorize before body replay. |
| K3 | Resolved; no invented requeue transition remains | §7.3 uses explicit terminal knowledge refusal and excludes automatic fresh retry/background retry. First-launch refusal and refusal after a physical launch preserve different execution facts. Names now match `agent.ExecOptions.Lifecycle` and `executeAndDrain`. |
| K4 | Resolved; verify every body-bearing projection | §6.1 makes source availability shared by source list/detail/history, target projections and runtime resolution. Purged/deleted/unrestorable required sources suppress retained body/summary/derived quotations, while ordinary withdrawal/expiry need not erase otherwise authorized history. Cache cancellation/late-response recovery is included. |
| K5 | Resolved; verify recovery without hidden IDs | The recovery URL is candidate ID + original request ID, creation preselects candidate ID, and the server internally resolves scope and verifies the original actor/key/receipt. Source/target role loss does not require disclosing source identity to recover the actor's content-free result. |

There is no remaining architecture or product-choice blocker among K1–K5. Do not retain the original “revise before promoting” verdict as the current task status after consuming this recheck.

### K1-D: capability downgrade must be enforced before any resume pointer is returned

The revised design explicitly prohibits unsupported effective backends from resuming a known managed-exposure or corrupt negotiated session (§7.3), and permits legacy continuation only when absence of managed exposure is proved (§7.4). Apply that rule to **legacy clients that omit `knowledge_context_version` as well**, not only to new clients that reach `/prepare` or `/authorize`.

This is a required placement/coverage condition for the chosen design, not a new policy choice: every server claim path must decide whether its **actually selected** session is safe before serializing `PriorSessionID`. For a client/backend that cannot enforce the managed-history protocol, select fresh ordinary execution for any known managed exposure or unprovable negotiated history; do not rely on that client to call the new endpoints. A current legacy task's NULL negotiation value does not prove that an older session has no managed exposure. Apply the same filter to direct Chat pointers, older-session fallback and exact rerun. A genuinely proven legacy/no-managed-exposure session retains existing behavior.

Carry these explicit names/criteria into the implementation checklist (the current broad unsupported-backend and legacy-versus-corrupt tests are useful but do not alone name the version-omitted claim branch):

- `TestKnowledgeLegacyClientCannotResumeManagedSession`: a previously managed session is selected by an otherwise valid legacy claim with no knowledge version; response carries no resumable old session and no private fields, no preparation call is needed, and fresh base execution remains allowed.
- `TestKnowledgeUnsupportedBackendCannotResumeManagedSession`: supported→unsupported effective backend/mode cannot reuse the managed session, including its direct Chat pointer and fallback candidate.
- `TestKnowledgeDowngradeDoesNotTreatNullCurrentTaskAsEmptyHistory`: current legacy task has NULL metadata, but the selected session has older managed exposure or corrupt negotiated proof; it is refused/retired rather than treated as legacy-safe.
- `TestKnowledgeProvenLegacySessionRemainsResumable`: a history with proven no managed exposure is not reset merely because the feature was deployed.

Include withdrawal/source-access loss in the managed-history examples, and also test a currently still-eligible managed session on an unsupported client: the selected conservative contract starts fresh because that client cannot perform later freshness enforcement.

### Precise implementation entry/proof criteria

The following are tests to implement and run in the implementation phase; their present absence is expected and does not invalidate this plan-only recheck:

1. **Exact history binding:** direct Chat pointer, older fallback, explicit rerun, same-session multiple tasks, cancelled/failed/NULL-session turns, native-ID switch and atomic pointer/exposure publication. Preserve all relevant exposure or refuse the session; a 32-item/64 KiB overflow must select a new generation before execution rather than truncate proof.
2. **Two distinct disclosure boundaries:** unfinished/finalized claim emits no private candidate/source fields; authorized preparation sends the exact bounded body; revoke/withdraw/purge between those phases prevents disclosure. Lost preparation response rechecks current eligibility; stale claim generation fails closed.
3. **Actual lock interoperability:** exercise preparation against workspace delete, source mutation, candidate approval/withdraw and current finalization. Preparation must never hold task and then wait for workspace; contention at its late runtime/task step must release source locks through bounded NOWAIT rollback. Use existing lock namespaces, not a new non-cooperating advisory fence.
4. **Ordinary execution behavior:** real fake-executable entry tests show per-physical-launch authorization. After `StartTask`, first-launch refusal produces zero physical launches and a normal terminal failure; later refusal preserves effects/usage/termination without replay. Assert both `shouldRetryWithFreshSession` and `MaybeRetryFailedTask` do not restart a knowledge-denied input. Unsupported combinations continue the original base path without private preparation.
5. **Human/target visibility:** source list, candidate detail/history, target view and stale client responses suppress the same unavailable-source sentinel. Withdrawal/expiry with available sources retains lawful historical content. Actor/request-specific receipt recovery remains content-free after source and target roles are lost.

Current source still supports the revised seams: `FinalizeTaskClaim` locks managed runtime/task before execution admission (`service/task.go:3784–3806`); `StartTask` precedes execution options/backend start (`daemon/daemon.go:7569`, `:7820`, `:8434`); `HostLifecycleSupported` admits only local Claude/Codex/OpenCode (`pkg/agent/launch_lifecycle.go:134–151`). The drafts now account for these facts. Native session identity is not provided by `LaunchInfo` automatically, so adapter/session binding and all pointer writers remain a normal G2 implementation gate, not a claimed existing capability.

### Verification performed for this recheck

- Read both revised drafts and re-inspected the named current ordinary lifecycle/finalization/adapter seams.
- Confirmed the revised decisions are internally consistent at plan level; original review preserved.
- Only this report changed. No draft/product/spec/task-state edit, DB/Provider call, test, lint or typecheck run.
- Scoped `git diff --check` passed for this report. No dynamic proof or passing implementation is claimed.


## Policy-preservation recheck — 2026-11-01

**Pass.** The narrow proposal/destination correction preserves the confirmed PRD and the earlier design-level verdict. Reviewed design SHA-256 `beb30149077688fd5104f431c7cd43d7ab40258bb41651aa5ef73f8b7d98f6b5` and implementation SHA-256 `e0a07fa6f939a3b6c9cab5db0b751b3d825c3095f0f1d43cc17088ac0bf273e1`. No other contract was re-reviewed in this pass.

The revised proposal row and §3.1 correctly separate current source Read + creator/handler authority from destination visibility. A legitimate external current handler without a source-workspace member row can propose back to that already visible source workspace. A source-project destination is allowed only through an already lawful, still-current source relationship/reference; it does not authorize project catalog, description or resource reads. Other destinations require normal visibility, and hidden/nonexistent/stale-source-project cases share `knowledge_target_not_found`.

The shared transaction checklist also removes the accidental unconditional member-row rejection for those pending source-target proposals, while retaining target membership and confirmer roles for approval. Pending creation changes only the source-owned candidate/receipt/audit; it does not mutate target base text, active managed contributions or target knowledge revision. Approval and source/consumer/audience checks remain unchanged, so this correction neither declassifies source content nor grants target-context access.

The writer supplied the required named implementation regressions: `TestKnowledgeExternalHandlerCanProposeToSourceWorkspaceWithoutMembership`, `TestKnowledgeExternalHandlerCanProposeToVisibleCurrentSourceProject`, `TestKnowledgeProposeRejectsInvisibleArbitraryDestination`, `TestKnowledgeSourceProjectProposalRevalidatesCurrentRelationship`, and `TestKnowledgeApprovalStillRequiresTargetMembershipAndAuthority`. These must exercise the real transaction/selector path, including missing membership rows and no extra catalog read, rather than only a pure permission helper. They are planned checks, not executed proof.

Only this small report note was appended. No source/draft changes, tests, DB or Provider calls; scoped report `git diff --check` passed. No new product question is required.
