# Independent M1-A1 design review

Reviewer: `trellis-check` / `least_privilege_plan_review`  
Baseline inspected: `5aef9c19ad3e764d09cff939edfded53f8c513bd`  
Scope: PRD/design/implementation plan and actual existing code, before implementation. No product changes or test execution.

## Verdict

**Revise the implementation contract before starting.** The narrow versioned-role scope, frozen claim identity, current originator intersection, and default-deny API approach are sound. The six implementation requirements below close concrete gaps in the written design; none requires a new user permission flow. The coordinator has already selected comment-only/no automatic leader wake for the first new role revisions. Preserve that explicit boundary and the old-role controls.

## Findings (fixed)

None. This is a design-only review; product and design edits belong to the coordinator/implementer.

## Findings (not fixed)

### 1. Persist comment non-dispatch using the existing fence; request context alone is insufficient (blocking)

`handler/comment.go:1914` calls the immediate trigger path, but `handler/daemon.go:4214` independently reloads comments and recomputes triggers at another task's completion. Tokens are later removed (`daemon.go:4114,4804`), so looking only for a current token cannot protect historical comments. `service/task.go:7268` also creates fallback comments from daemon final output without the original API request context.

Smallest repair: extend `CreateComment` in `pkg/db/queries/comment.sql:424` with a **server-only**, nullable/default-false `suppress_dispatch` input. Persist `dispatch_eligible = admission_eligible AND NOT COALESCE(suppress_dispatch,false)` in the existing atomic issue-touch/comment INSERT. Do not add a second comment suppression field or duplicate task persistence solely to solve replay. Both HTTP comments and daemon-produced comments for restricted profiles must set it from trusted role/claim identity, never a body field. A missing lookup must not become unrestricted. The daemon fallback may use the agent's immutable template provenance when the token no longer exists, provided it fails closed on lookup failure and preserves old versions.

Existing fences already consume this durable bit: `computeCommentAgentTriggers` (`comment.go:2637`), `resolveCommentTriggerEnqueue` (`:2096`), service `checkIssueExecution` (`service/issue_admission.go:12`), and the `suppress_execution` event consumed by `SubscribePluginEvents` (`plugin_event_bridge.go:47,66`). The immediate response should classify explicit mentions as capability-blocked, not falsely as triage-review-required; replay can remain inert using the durable bit. Preserve the bit on edit and never let later task-token cleanup or role autonomy changes reactivate a comment.

Required proof: immediate explicit mention, implicit leader fallback, parent reply, completion reconciliation after token deletion, service-generated final-output comment, and plugin event sink each produce no extra execution; old-role/human controls still dispatch.

### 2. Comment-only must reject attachment binding and avoid secondary mutations (blocking)

Allowing `POST .../comments` wholesale still allows `attachment_ids`; `CreateComment` links them at `comment.go:1879`. It also auto-unresolves an existing thread and cancels deferred escalations at `:1905–1908`. These contradict A3's no attachment writes and the coordinator's no thread/queue mutation decision even if trigger creation is blocked.

Reject nonempty attachment IDs for restricted callers before writing; preserve exact reply-parent checks and the plain comment/progress-update types. Skip auto-unresolve and deferred-escalation cancellation in **both** HTTP and `TaskService.createAgentComment` paths for restricted results. Normal issue activity/revision updates and human notifications remain part of posting a comment. Assert denial leaves attachment bindings, thread state and deferred queue unchanged, not just that HTTP status is 403.

### 3. Scope checks must bind the actual resource read/write, not a preliminary middleware lookup (blocking)

`loadIssueForUser` (`handler.go:1124`) reloads an issue for the handler, while the proposed guard would resolve it earlier. If an issue is moved out of the reporter's project between those reads, the allowed handler can consume a different authorized state. Similarly, `buildClaimedTaskResponse` reads agent/issue/context before `FinalizeTaskClaim`; freezing a different project afterwards can mismatch the delivered claim and token.

Specify where the checked facts are consumed: reuse the loaded issue or recheck capability in the canonical loader on the exact row passed to the handler, and put the scope/current-authority condition on the comment write transaction. For multi-query detail/comments/timeline reads, use a consistent authorization snapshot or appropriate owned lock through the bounded read; do not claim atomic revocation if only an earlier lookup was checked. Finalize must compare the scope/agent identity used to build the payload with the locked current rows before publishing it. Add a deterministic move/revoke race test at these boundaries.

Keep identity separated: do not replace runtime-owner X-User-ID with the originator. Existing `invokeAgentDecision` is in the handler package and returns only bool on query errors; an Auth-level implementation cannot import it without a cycle or preserve the promised 503 distinction by calling it blindly. Place the shared decision in an existing lower layer, or perform query loading and the pure existing invocation predicate in a handler-owned guard after Auth. Retain owner/private/public-to semantics while requiring actual originator membership for restricted profiles.

### 4. A usable restricted claim needs payload and daemon-tool backstops (blocking)

`daemon.go:2212–2369` populates plugin tools, MCP connections/config/overlay and ConnectedApps. Dropping those visible fields is necessary, but existing daemon endpoints (`plugin_agent_hook.go`, `plugin_mcp.go`) accept their own authenticated task reference, outside ordinary Auth. Specify a trusted restricted-task/profile denial there too. The invariant that Finalize refuses a restricted token plus extra daemon credential must be tested through both claim endpoints.

`daemon.go:3298` also injects active sibling issue titles across the workspace, and native session resume can retain prior broader input. If A2 is a claim-time read boundary as well as an HTTP boundary, omit/filter sibling context and prevent reuse of a session whose scope is not proven compatible. Otherwise explicitly label the guarantee as HTTP resource access only and do not claim that the model sees only the permitted issue/project. Existing human-supplied CustomEnv/CustomArgs and host credentials remain outside this slice's sandbox guarantee, as already stated in the PRD.

Do not silently present tools as available after removing them. Inject an authoritative capability briefing describing the allowed reads, comment-only result and unavailable tools. Claim errors for unsupported non-issue runs must be terminal/actionable rather than an infinite requeue loop.

### 5. Runtime instructions and rollout need to match the selected new semantics (blocking)

The current executable spec (`builtin-templates.md:637`) says ordinary delivery goes to in_review and same-issue worker results wake the leader. Existing reporter v1 additionally relies on Contributor for its report issue transition (`:73`). Merely changing four saved role templates leaves shared runtime/role-skill instructions contradicting the new API refusals. Existing skills are also reused without overwriting saved workspace copies.

Add the new authoritative capability briefing at claim time, with precedence over generic workflow/old saved skills: submit the report/review comment, do not change business status, and no automatic leader wake occurs for this profile. Update only fresh defaults and document existing instance/skill reuse; do not auto-upgrade them. Add claim-brief composition tests and an old-version regression. The current template thresholds were independently verified: reporter/code-reviewer/security-reviewer v1 to v2, migration-reviewer v2 to v3.

### 6. Migration rollback must fence concurrent minting (blocking)

The proposed down guard says to refuse if restricted tokens exist, but an unlocked EXISTS check can miss an uncommitted mint, then wait for the DDL lock and erase the new security field. `security-boundaries.md:328` already specifies the fix: lock the affected table(s) in deterministic order with ACCESS EXCLUSIVE NOWAIT, check and drop inside one DO block. Require the same writer-stop operational contract for multi-file rollback. Verify populated, empty-maintenance and concurrent-first-mint cases using private fixture schemas.

## Accepted design choices and verification additions

- Server-created template key/version, never display name/instructions/autonomy alone, selects the new restricted profiles. Existing agent versions, custom agents, user tokens and reused squad members retain their existing behavior.
- Unknown/malformed non-NULL policy is denied; NULL remains legacy only for genuinely unprotected credentials. Both password and ordinary auth must carry the verified policy and revalidate task generation/status/current principals.
- Exact method/path allowlisting plus single exact `project_id` is appropriate. Deny aliases/broad search/grouped/query APIs and prove filter parser agreement. UUID and canonical issue identifiers must reach the same scope check.
- Signed attachment URLs can be minted inside allowed issue/comment reads (`file.go:154–185`). The design currently denies attachment endpoints. Either retain metadata with stable non-granting links, or explicitly treat attachments of allowed resources as an allowed read capability; do not claim endpoint denial revokes an already returned CDN signature. This is a small contract clarification, not a request to broaden scope automatically.
- Run a CLI-level positive flow using the actual injected task identity to ensure strict allowlisting does not prevent normal issue show/list/comment/timeline use. Broad `/api/me` or workspace inventory grants are not justified merely for convenience.

## Verification

- Lint: not run; design-only review, no product edits.
- TypeCheck/build: not run; design-only review.
- Tests: not run by instruction. All listed cases are required implementation evidence, not passing results.
- Read-only evidence: inspected task artifacts/manifests, authorization research, actual claim finalization and both claim callsites, token query, role roster/provenance, issue loaders/list/detail, HTTP and service comment writers, replay, shared dispatch eligibility and plugin event bridge.

After these requirements are incorporated, the bounded child can proceed to implementation. This review does not accept M1 as a whole and does not establish M2/M3 completion.

## R1 follow-up

Read the coordinator's appended R1 before handing back this review. R1 addresses findings 1, 2 and the external-tool portion of 4 at the design level: durable dispatch eligibility is reused, completion receives frozen task policy, comment side mutations are forbidden, configured tools receive explicit refusal and daemon endpoints retain a backstop. Task policy persistence is acceptable for completion provenance; keep each comment's own dispatch bit as the enduring replay fence.

Remaining pre-implementation requirements are findings 3 (consume authorized resource state without a second unchecked lookup and keep invocation error semantics), 5 (authoritative claim briefing/new-version compatibility), and 6 (locked down-migration guard). The sibling/resume and signed-attachment observations require either narrow payload handling or an explicit narrower guarantee in the design; they do not require a general runtime rewrite. Incorporating these into the implementer contract is sufficient to proceed; implementation correctness and races remain the later independent check's job.
