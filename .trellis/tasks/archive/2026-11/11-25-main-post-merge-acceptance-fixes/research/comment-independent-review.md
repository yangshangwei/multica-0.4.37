# Independent comment repair review

Reviewed 2026-11-25 against the assigned acceptance-fixes worktree and its final router-race01 receipt. The existing parent/child task authorizes this review; stale session `no_task` and historical planning notes do not require creating another task. Source remained read-only. No commits, database/service mutations, test reruns, provider calls, or recursive agents were performed. Only this report and `comment-independent-review.json` are reviewer-owned changes.

**Decision: accept the recorded execution evidence and the narrow administrator-role repair; do not accept the complete spoof-resistant comment authorization requirement until C1 is resolved.** C1 is a pre-existing author-path bypass exposed by reviewing the actual call chain, not a regression introduced by the two changed predicates.

## Findings (fixed)

None by this reviewer. The implementation changes only the administrator predicates in `server/internal/handler/comment.go:3333` and `:3594` to require `!isMachineCredentialActor(r)`. Those predicates correctly prevent task/Cloud credentials from borrowing owner/admin membership. No abstraction, dependency, migration, or broader source simplification was added by this review.

## Findings (not fixed)

### C1 — P1: a valid client-supplied agent/task pair still grants another agent's author authority

Trigger: a valid legacy-mode `mcn_` credential whose owner belongs to the workspace sends an ordinary comment PUT or DELETE with `X-Agent-ID` naming the comment's agent author and `X-Task-ID` naming any existing task belonging to that agent. The agent can belong to another runtime/user. The caller must know those IDs; the IDs are not authenticated proof of acting as the agent.

The complete source path is:

1. `middleware/auth.go:214` stamps the verified Cloud owner and `cloud_pat`, but retains client `X-Agent-ID` and `X-Task-ID`. The authenticated-principal `AgentActor` flag does not grant authority, but ordinary comment authorization does not consume that flag.
2. `handler/task_capability.go:25` passes requests without a verified task token. The Cloud request has no such token. `RequireIssueBudgetHuman` applies only to budget-management paths; installation metadata without a proof is a passthrough; the ordinary workspace middleware validates membership only. Router wiring is at `cmd/server/router.go:1649`, `:2237`, and `:2537`.
3. `handler/handler.go:952` recognizes authoritative task credentials first, then applies the same legacy fallback to every other credential, including Cloud. At `:978` it checks that the agent exists in the workspace; at `:992` it checks only that the task belongs to that agent. Neither query checks caller/runtime ownership, task liveness, or a credential binding. `pkg/db/queries/agent.sql:26` and `:776` are ID-only reads.
4. `resolveActor` consequently returns the victim agent. Both `comment.go:3332` and `:3593` set `isAuthor=true`. The new `isAdmin=false` cannot reject the request because the condition is `!isAuthor && !isAdmin`. Body changes, attachment replacement and deletion therefore reach their normal mutation paths. Ordinary member JWT/PAT callers can use the same fallback in both auth modes.

This is a source-path finding, not a newly executed exploit. It conflicts with the child PRD's forged-actor-header prohibition and ordinary-member denial requirement, and limits the implementation report's assertion that machine credentials cannot manage other agents' comments. Password mode rejects Cloud credentials outright, but retains the member JWT/PAT fallback.

The added tests miss this condition: `comment_authority_integration_test.go:160` uses `uuid.NewString()` as the spoofed task ID, guaranteeing the fallback cannot identify a task. The Cloud test at `:340` supplies only `X-Actor-Source: member` and targets a human-authored comment. The direct handler test similarly supplies no valid task pair. These are valid denial tests but do not establish resistance to valid-pair impersonation.

Recommendation: define the authorized legacy agent-author relationship and enforce it before granting comment author authority, or introduce a narrowly scoped verified-author check for comment mutations. Add real-router regressions with a valid unrelated same-workspace agent/task pair for Cloud and ordinary-member JWT/PAT, plus a terminal-task pair. Assert unchanged comment, attachments, tasks, inbox and events. Preserve actual `mat_` self writes, human self writes, and human administrator management. Changing shared `resolveActor` affects other callers and needs parent scope/design review; this reviewer was authorized for read-only source review, so did not patch it.

## Accepted scoped evidence

The independent JSON records complete named inventories, raw SHA256s, receipt hashes, source-manifest hashes, configuration/tool checks, and resource-record hashes. Every final run has one terminal per started named test, no orphan/duplicate/incomplete names, no malformed/unknown log events, and a passing package terminal matching exit 0.

| Frozen run | Named PASS | Named SKIP | Exit |
| --- | ---: | ---: | ---: |
| claim-legacy01 | 134 | 0 | 0 |
| claim-password01 | 134 | 0 | 0 |
| handler-normal02 | 6200 | 7 | 0 |
| handler-race02 | 6200 | 7 | 0 |
| router-normal01 | 1751 | 8 | 0 |
| router-race01 | 1751 | 8 | 0 |

Counts include parent tests and subtests; they are not counts of independent security scenarios. Handler source has 3150 top-level tests and all execute. Router source has 519 default top-level tests and all execute; 48 additional declarations are excluded by explicit `agentintegration`/`knowledgeintegration` tags. The exact comment name/result sets agree between normal and race. Other raw name differences are 10 handler and 38 router fixture-UUID-bearing names per mode; replacing UUID values only produces identical name/action multisets, preserving duplicate multiplicities. Raw inventories remain unchanged in the JSON.

All 9207 files per claim snapshot and 9211 files per full-package snapshot match their recorded SHA256 and permissions, with no added frozen files. The manifest digest, execution environment, supported Go compiler and resource-probe hashes match. Every frozen server `.go`, `go.mod`, and `go.sum` file matches current checkout bytes. The six final lane file hashes also match, including the spec and built-in skill/source map. The older skill prose in earlier snapshots is separately covered by the final template conformance log: 10 named PASS, no failures/skips. The final Go source and tests match across those snapshots.

Original legacy/password probe-result hashes match their read-only originals. Earlier RED, allocation-mismatch, omitted-fixture, and timeout evidence remains distinct from final acceptance; none is relabeled as passing. RED includes the recorded incorrect membership expectations, so it is not described as a pristine all-negative security oracle.

The actual-claim cases obtain `mat_` from the daemon claim endpoint, check its exact database user/workspace/task/agent binding, start the task through HTTP, and retain that issued credential until final API cancellation. The claimed variants omit manual token restoration. JWTs and PAT records are fixture-created; this proves authentication/HTTP and claim lifecycle, not registration UI or provider execution.

Covered denials meaningfully compare complete workspace comment/attachment/issue/task/inbox rows and domain-event counts; the negative matrix also creates a queued consumer to expose accidental cancellation. The gates precede body decoding, attachment replacement, cancellation, revision writes and event publication. Positive JWT/PAT and task-self cases verify authorship, revision 2, attachment removal, deletion, and stale-revision refusal. Cross-workspace, expired/revoked credentials, actual task cancellation, member loss, restricted reviewer and real `mdt_` general-auth rejection are exercised. These controls do not cover C1.

Seven handler skips remain the existing stub-query TODO, two iteration performance opt-ins, live GitHub opt-in, triage performance, wire export, and absent HTTP MCP recipe. Eight router skips are six Linux namespace integration gates, the dedicated stage harness, and browser fixture export. Exact names/reasons are in the JSON. None is counted as PASS; no broad platform acceptance follows.

## Separate contract and evidence limits

Generic `UploadFile(comment_id)` at `handler/file.go:450` and `:489` checks membership/same-workspace association and records independent uploader provenance. Its existing lack of an author/admin condition is retained as an unresolved product-contract question. This review neither declares that behavior fixed nor invents a new authorization requirement for it.

Cloud tests use a local verifier, not deployed Fleet. Existing event metadata is the operator witness; unchanged author fields are not proof that the original author performed the latest edit, and no durable per-edit audit is added. Resource records show distinct databases and isolated HOME/TMP paths; handler final records retain five users and router final records three, with zero comments/workspaces. The reviewer checked those files and identities without querying live databases or asserting fresh cleanup. Parent owns retained resources. Dedicated Redis configuration and the handler's effective DB14 override remain as documented by the lane; no shared-service operations were performed here.

The child report's appended final handoff and completed receipts supersede its earlier pending-router text. Parent should reconcile that historical prose during final integration. The user's no-commit instruction overrides workflow commit/clean-tree language: a deliberately uncommitted source candidate is not an authorization defect or a reason to commit during review.

## Verification

- Lint/static: reviewer ran supported `gofmt -l` on all three comment Go files and focused `git diff --check`; both pass with no output. Existing `go vet ./internal/handler ./cmd/server` receipt records exit 0 on the hash-verified claimed-password snapshot; the receipt does not retain raw vet output.
- TypeCheck: Go compilation passes as part of all six audited test runs. No TypeScript changes belong to this lane; no TS checks rerun.
- Tests: final normal/race and actual-claim evidence accepted as above, with explicit skips. No full test rerun was needed or performed. Full task security acceptance remains blocked by C1.
- Reviewer changes: this Markdown report and its JSON companion only. Remaining risk is the concrete author-path bypass above; source repair and its new regression evidence belong to the parent/implementation lane.
