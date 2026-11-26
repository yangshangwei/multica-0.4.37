# Research: M1 ordinary completion clients and entry-point boundaries

- Query: Current Web/Desktop and CLI status-only completion behavior, reusable client pieces, and the gap to evidence review → explicit authorization → submit → recover actual result.
- Scope: Internal source/spec inspection only. Write scope is this file. No implementation, tests, database access, Provider calls, runtime start, or task activation.
- Date: 2026-11-12
- Task: `.trellis/tasks/11-12-agent-squad-followup-roadmap/` (still planning; parent reported no active pointer).

## Findings

### 1. Current shared status-only path

| Source | Observed contract |
| --- | --- |
| `packages/core/issues/completion-hint.ts:28` | Classifies ordinary transitions from source/target categories and valid issue revision. A custom done key is retained; missing revision/category is unknown. It always builds `evaluation_key:null`. |
| `packages/core/types/api.ts:52` and `:110` | `DeliveryCompletionBinding {authorization_id, operation_id, expected_issue_revision}` already exists and is accepted in `UpdateIssueRequest.completion`. Reuse it. |
| `packages/core/api/client.ts:3097` | Hint API uses bounded request, no-store, explicit three-field body, and schema parsing. It cannot forward a private key even through runtime casts. |
| `packages/core/api/schemas.ts:1369` | Hint response keeps public identity/policy/hint fields, requires both flags false, strips optional private evaluation. Do not turn this DTO into a transferable evaluation/authorization container. |
| `packages/views/issues/actions/completion-hint-gate.ts:46` | Current shared hook bypasses collaboration, no-project, already-done, non-done target, same status and mixed updates. It shows attention/matching confirmation, refusal or unknown; confirmation invokes the existing continuation. |
| `packages/views/issues/actions/use-issue-actions.ts:141` | Detail/actions invoke the gate for exact status-only updates. Surface delegation sets `skipCompletionHint:true` to avoid a second hint. The real mutation settles before single-flight release. |
| `packages/views/issues/surface/use-issue-surface-actions.ts:115` | Surface status-only updates invoke the same hook when loaded issue is ordinary. |
| `packages/views/issues/components/issue-detail.tsx:711` | Subissue row status-only updates invoke the same hook. These are ordinary child issues, distinct from independent collaboration lifecycle. |
| `packages/views/issues/components/issue-detail.tsx:2881` | Detail done continuation uses the mutation's `onSuccess`; failed/cancelled operations do not call `onDone`. |
| `packages/views/modals/issue-completion-hint.tsx:59` | Cancel stops the hint continuation; confirm/retry have local pending UI. Retry closes the old dialog before requesting a replacement. |

Shared Web/Desktop behavior is already located in `packages/views` and `packages/core`; no duplicated app-level business flow is needed. Keep platform routing and downloads in platform adapters if required.

### 2. Specific client gaps

1. **Evidence acquisition is absent from ordinary clients.** No ordinary delivery-evidence evaluation, artifact-package selection/review, completion-authorization create/read/revoke wrappers or client command family were found in `packages/core/api`, `packages/core/issues`, `packages/views/issues`, or `server/cmd/multica`. `agentApprovalListOptions`/`agentApprovalDetailOptions` (`packages/core/agent-approvals/queries.ts:33`, `:47`) provide a generic human approval queue/detail, not a complete current delivery evidence tuple. The list is one bounded newest-first page, filtered by status, not by exact issue/artifact. Do not select “latest approved” as current evidence.
2. **The callback cannot currently carry the new binding.** `CompletionHintGateIssue.continueMutation` is `() => void | Promise<void>` (`completion-hint-gate.ts:25`). Its existing closures capture old `updates`; changing a view flag alone cannot inject `completion`. A bounded completion coordinator must own the exact immutable bound write or accept a typed payload/result continuation.
3. **No original-operation recovery.** The hook retries a *read* by fetching the current issue (`:104`), and then starts a new hint. It has no saved request key, operation ID, authorization ID, revocation intent, or receipt. Reusing this retry action for an uncertain authorization or PUT would be wrong.
4. **Single-flight is local, not durable.** Pending IDs/controllers are per hook instance (`:49`); unmount aborts all (`:212`). This correctly prevents stale hint confirmations but is insufficient to recover a dispatched authorization/completion across remounts, account/workspace navigation, browser reload or another detail window. A persisted original intent must survive ordinary view teardown; stale callbacks must still be fenced.
5. **Ordinary mutation remains optimistic.** `packages/core/issues/mutations.ts:127` always applies a status patch in `onMutate`; it strips the completion control field from cache (`:146`) but does not suppress optimistic status. Bound completion should await a verified server result before reporting done. Reuse cache-coordinator invalidation/reconciliation, not an optimistic done followed by rollback as the truth of an uncertain commit.
6. **Completion response lacks an endpoint-specific parsed contract.** `api.updateIssue` (`client.ts:3085`) directly returns `fetch<Issue>`. A new bound-completion path needs explicit parsing and identity/revision validation. Malformed 2xx, timeout and aborted transport after send must remain unknown rather than successful or definitely uncommitted.
7. **Identity/context must be explicit.** New queries/intents need server + human user + source workspace + issue scope. Hint retry passes workspaceId to `getIssue`, but the initial hint call presently supplies signal only (`completion-hint-gate.ts:140`). New code should carry the original explicit scope through every bounded request and reject late results after identity change.

### 3. Current CLI wire/output contract

- Registered commands are `issue status <id> <status>` and `issue update <id>` (`server/cmd/multica/cmd_issue.go:218`, `:232`). Status accepts a **status key**, including custom keys, not a display label.
- `issue status` flags are `--no-start` and `--output table|json`, default table (`:570`). `issue update` has `--status` plus the ordinary field flags and `--output table|json`, default json (`:551`). No completion-authorization/request-key/operation flags exist.
- Status-only update means no title/description/priority/assignee/project/date/parent/stage/position flags changed (`:1613`). `--no-start` does not itself take the request out of the current hint path; it adds `suppress_run` to the eventual write. The new bound-completion contract must explicitly decide its supported payload, rather than accidentally treating arbitrary control fields as status-only.
- Built-in non-done keys bypass hints; `done` and every custom key query the server (`:1626`). The server resolves custom category.
- The normal request sequence is resolve the issue identifier → GET full issue revision → POST `/api/issues/{uuid}/delivery-evidence/completion-hint` with null evaluation key → PUT `/api/issues/{uuid}` (`:1546`, `:1641`). The PUT is currently plain status (plus optional suppress_run), without completion binding.
- Attention/matching warnings go to stderr and explicitly do not authorize (`:1666`). Only unstructured 404/405 may bypass a missing old hint endpoint; structured permission/delivery/resource errors refuse (`:1714`). Incomplete/authorization-like/mismatched revision or target responses refuse (`:1691`).
- `issue status` emits its success message only after PUT succeeds (`:1582`); JSON stdout is the returned issue. `issue update` returns the updated issue JSON by default or a table (`:1462`). A new completion command must retain machine-readable stdout and avoid printing progress into it.
- Generic `approval get/list/request/executed/cancel` commands (`server/cmd/multica/cmd_approval.go`) operate high-risk agent approval requests, not completion authorizations. `approval cancel` is not completion revocation; `approval executed` is not done or evidence acceptance.

### 4. Existing recovery patterns worth reusing selectively

- `packages/core/collaboration/draft.ts:25` scopes durable local intents to server/user/issue. `saveCollaborationDraft` (`:44`) rejects replacement of an existing different immutable payload and read-verifies storage before allowing work. This is a useful **pattern** for an ordinary completion intent in core using `StorageAdapter`; do not reuse collaboration domain records as ordinary completion.
- `packages/core/collaboration/action.ts:85` first looks up an original operation but then resends an exact write on 404. **Do not copy that resend policy** into completion: the delivery completion contract says commit ambiguity is unknown/non-success with no automatic retry. The ordinary completion recovery action should perform the original lookup only, then separately expose any explicit retry allowed by the reviewed contract.
- `packages/core/issues/mutations.ts:222` invalidates owner projections after errors; `:243` reconciles actual returned state. Reuse these helpers to avoid missing detail/list/children/count/iteration projections after known completion or uncertain-result reads. A historical consumed receipt cannot overwrite a newer reopened issue.

## Proposed bounded client design (planning, not approved implementation)

### Shared Web/Desktop

1. Keep the current ordinary status-only trigger family. Read current server context, exact candidate evidence and selected evidence details before showing an explicit acceptance action. Resolve active custom done target on the server. Off/observe behavior remains the existing non-authorizing hint/plain writer behavior; enforce requires explicit authorization.
2. Show the exact issue/revision/status target, evidence source/approval/spec/artifact and archive identity, observation time/expiry and limitations. A selected current candidate is not authorization. Do not accept another human's evaluation key; authorization creation makes its own current observation.
3. Explicit acceptance freezes the original authorization request key and operation UUID plus the reviewed binding. Persist the minimum original intent in core **before** first dispatch; storage failure prevents dispatch. UI uses ordinary words, exposes advanced IDs only when they help recovery.
4. Create the grant; only a known valid returned/recovered grant enables a second explicit submit action using `UpdateIssueRequest.completion`. Authorization and submission are two distinct user actions. Keep one semantic operation until its outcome is established. Never generate a replacement operation merely because a request timed out.
5. Query creation by original request key and source workspace; query completion by original authorization ID/receipt. Show both the historical operation outcome and a separately fetched current issue state. If the issue was reopened later, report that plainly without replaying done.
6. Cancel before send means no mutation; cancel after grant exists requires explicit revoke intent or a clearly described retained unused grant. Cancel after a request was sent is not proof of server cancellation; continue showing original lookup recovery. Revocation/consumption race must display the winning terminal fact.
7. Duplicate actions share the same original intent. New authorization is required after issue/project/policy/evidence changes, expiry or revocation, or reopen → new completion. Done → done/custom-done switches do not manufacture a new acceptance.

### CLI proposal

A separate explicit `issue completion` command family is easier to keep unambiguous than overloading advisory `issue status` confirmation. The main planning lane selected `prepare`, `authorize`, `submit`, `show`, `revoke` as the proposed verbs; these are not existing commands. Existing `issue status` / `issue update` must not sign automatically.

- `prepare <issue> --status <key>`: read-only machine-readable context and evidence candidates; exact evidence selection is required before authorization.
- `authorize`: accepts a previously reviewed exact immutable request/operation file plus reason/expiry and explicit acceptance flag; the original operation file is durably saved before any write; no implicit grant from `--yes`, a hint, or generic agent approval. Use authenticated human only; ordinary machines cannot impersonate the signer.
- `submit`: accepts exact authorization/operation/revision binding and target; rejects mixed field flags in this first package. Persist/reuse the operation file; never silently re-read latest revision to rebind a previously signed grant.
- `show`: either authorization ID or original creation request key (mutually exclusive; source workspace required for request-key recovery); read-only recovery. A successful query whose terminal fact is absent does not prove an uncertain completion never committed, and current done alone does not prove this operation caused it.
- `revoke`: exact authorization, original revocation key and reason; no hidden completion retry.
- JSON result distinguishes `prepared`, `authorized`, `consumed`, `revoked`, `unknown`, `refused`; human progress goes to stderr. Submission/recovery must return non-success for unknown or refused outcomes, preserving a machine-readable recovery locator. Current issue state is separate from historical receipt. The exact exit-code choices belong in the final CLI contract.
- Support files through the existing safe file-path/encoding conventions (`cmd_issue.go:45`, `:116`); keep secret-free operation files local and identity-scoped. Never put bearer secrets or another person's evaluation key into them.

## Entry-point boundary and follow-up order

| Entry | Current source/contract | First package | Later design needed |
| --- | --- | --- | --- |
| Ordinary single status-only detail/actions/surface/subissue | Shared hook above; plain writer enforces backend authorization when required | Yes: shared workflow plus CLI contract | Exact recovery and cancellation UX |
| Drag/drop | `use-issue-surface-actions.ts:136` goes directly through move mutation; `mutations.ts:131` maps move intents to POST move | No new drag prompt | Next: preserve anchors/position, visible refused drop and exact retry intent; do not reuse frozen numeric optimistic position as current proof |
| Batch | `use-issue-surface-actions.ts:194` uses batch mutation; core completion_bindings and ordered outcomes already exist | No new batch authorizer | After single/drag: per-item evidence/grants, committed prefix, unknown stop; no all-or-nothing fiction, no automatic remainder replay |
| Create as done | Existing create API; backend enforce explicitly refuses including custom done and inherited project | No silent change to todo | Later: explicit two-step create then review/authorize complete, if product chooses it; never invent evidence for an uncreated issue |
| Mixed update + done | Existing hook bypass; CLI skips hint for changed ordinary fields; backend enforce refuses mixed completion | No auto-splitting request | Later: save edits first, obtain new revision/evidence, explicit completion; explain partial user intent without claiming whole update atomic |
| Independent collaboration / R38 | Ordinary gate bypass; shared collaboration action schema has own epochs/submission lifecycle (`core/collaboration/action.ts:17`); backend source policy still applies | Excluded | Separate package: exact child artifact, initiator/submission acceptance, R38 actual admin identity and historical receipt; never ordinary grant shortcut |

Final planning order after the first package: drag → batch → create-as-done → mixed update → collaboration/R38 as separately reviewed work (requirements.md owns the ordering). This is an ordering proposal, not a mandate to implement all F1 before M2; the main roadmap may interleave later packages explicitly. Existing `skip_supported=false` remains accepted.

## Existing tests and validation commands to reuse later

No tests were run during this research. These are source-verified suites/commands for the implementation plan:

- `packages/core/api/delivery-completion-hint.test.ts`: strict three-field/null-key body, no key smuggling, strips private evaluation, malformed 2xx refusal.
- `packages/core/issues/completion-hint.test.ts`: custom done, source/target/revision unknown, structured error classification.
- `packages/views/issues/actions/completion-hint-gate.test.tsx`: current issue refresh, obsolete modal unmount, duplicate confirmation until real settlement, cancel without PUT, unknown no-write, legacy-only fallback.
- `packages/views/issues/actions/__tests__/use-issue-actions.test.tsx`: delayed success continuation, one operation pending through actual mutation, run-confirm compatibility.
- `packages/views/modals/issue-completion-hint.test.tsx`: existing modal UI behavior.
- `server/cmd/multica/cmd_issue_test.go:3379` onward: `TestRunIssueStatusCompletionHintAttentionContinuesWithExactPayload`, structured refusal, legacy404/405, custom done, status-only update, mixed update skips hint, malformed no-PUT. Preserve these contracts where unchanged.
- Targeted command shapes: `pnpm --filter @multica/core exec vitest run api/delivery-completion-hint.test.ts issues/completion-hint.test.ts`; `pnpm --filter @multica/views exec vitest run issues/actions/completion-hint-gate.test.tsx issues/actions/__tests__/use-issue-actions.test.tsx modals/issue-completion-hint.test.tsx`; from `server/`, `go test ./cmd/multica -run 'TestRunIssue(Status|Update).*CompletionHint|TestRunIssueUpdateStatusOnlyUsesCompletionHint|TestRunIssueUpdateMixedStatusSkipsCompletionHint' -count=1`.
- Package lint/typecheck scripts exist in both package.json files: `pnpm --filter @multica/core lint`, `pnpm --filter @multica/core typecheck`, equivalent views checks. CLI package normal tests/vet and built-in skill contract tests accompany new CLI behavior. These are deterministic tests, not real Provider acceptance.

New canonical tests must prove exact evidence selection → immutable authorization binding → one writer invocation, creation timeout lookup with no new key, submit timeout lookup with no automatic PUT, stale/revoked/expired refusal, cancel before/after dispatch, current status after reopen, custom active done, malformed response, account/server/workspace scope isolation, remount recovery and durable-intent save failure. UI suites should keep wiring/accessibility and named regressions rather than duplicating every core state matrix. Shared Web/Desktop integration should use the same component and localized copy.

## Related specs and skill documentation

- `.trellis/spec/server/delivery-completion-enforcement.md`
- `.trellis/spec/server/delivery-completion-hints.md`
- `.trellis/spec/server/controlled-artifact-execution.md`
- `.trellis/spec/core/frontend/issue-batch-outcomes.md`
- `.trellis/spec/core/frontend/api-client-auth-epoch.md`
- `.trellis/spec/core/frontend/collaboration-workbench.md` (identity/durable intent patterns only)
- `.trellis/spec/views/frontend/quality-guidelines.md`
- `.trellis/spec/server/database-test-isolation.md`
- `server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md:380`
- `server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md:272`
- `server/internal/service/builtin_skills/multica-creating-agents/SKILL.md:502`
- `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md:244`

CLI behavior/API documentation changes require updating the relevant built-in skills and source maps in the same implementation PR. No new dependency is needed for the proposed client flow.

## Caveats / Not Found

- This report proves current source shape, not running behavior. Backend evidence identity/readiness and exact expiry/recovery response fields are researched separately by the backend lane; final client contracts must use those real fields, not guesses.
- Generic approval listing is available, but a complete issue-scoped delivery evidence picker was not found. Existing `/api/agent-approvals/{approvalId}/artifact-package` is a server route; ordinary UI/CLI wrappers are absent.
- The main planning lane has selected two explicit UI actions (authorize, then submit), the proposed `issue completion prepare/authorize/submit/show/revoke` family, durable operation files before writes, and lookup-only recovery on unknown. The latest user-facing planning summary still requires review before these become an implementation design. Later entry ordering remains a proposal.
- Cross-platform release, Hosted CI, deployment, knowledge injection, real Provider runs and Windows remain excluded.
