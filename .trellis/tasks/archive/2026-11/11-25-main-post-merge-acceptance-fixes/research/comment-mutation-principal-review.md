# Research: Bounded comment mutation principal correction

- Query: Independently review the amended comment-mutation authority design and implemented local helper, positive identity behavior and scope.
- Scope: internal defensive code/evidence inspection only.
- Date: 2026-11-25

## Findings

**Accept the bounded design and inspected helper. No new routed-authority or scope-expansion finding. Final full-package normal/race evidence remains pending.** This comment-principal follow-up is distinct from the frontend C1 resume-action work.

`server/internal/handler/comment.go:30–45` uses the private verified task-token context as the only agent-author source. The accessor itself requires a valid token ID (`internal/auth/task_capability.go:90–93`); the local helper additionally requires valid agent/task IDs, matching user/workspace, and no contradictory authenticated credential kind. It rejects a task principal or stamped task source without the verified binding. Non-task callers retain their authenticated member identity regardless of legacy agent/task hints.

Production legacy/password Auth resets old context and sets verified user/task/workspace/source together. The Update/Delete author decisions use the helper before mutations; the existing machine-admin denial remains. Both updated/deleted event operators use that same resolved actor. Agent self body edits take source_task_id from the verified token; member/admin edits clear lineage as before. Legitimate task self writes, human self/admin writes and Cloud member-self writes remain represented in the local router matrix. No human-only route gate was introduced.

Byte comparison confirms `handler.go` (including global resolveActor), `actor_guards.go`, both Auth middleware files and task-capability context helpers are unchanged. The product diff stays in the two comment mutation entrypoints plus their local identity helper/source-task use. The direct autopilot fixture now supplies verified task context to model its documented task editor; this is not a production authentication shortcut.

The new valid-hint HTTP tests include existing running/terminal agent-task pairs, unrelated/caller-owned agents, member/owner/admin JWT/PAT and legacy Cloud credentials, body/attachment/delete, denial snapshots and event attribution. Raw retained events independently counted RED477FAIL/56PASS and GREEN1081PASS, with complete terminals and receipt source/config/tool unchanged. The GREEN command is the combined CommentManagement matrix, including actual claim controls; this review did not run it or contact any external service. Helper-binding/autopilot focused and new full handler/router normal/race runs must still be checked when finished.

## Files found / related specs

- Comment task `design.md:26–36` — narrow correction and regression contract.
- `server/internal/handler/comment.go` — local actor resolution, Update/Delete gates, context-bound lineage and existing event publication.
- `server/internal/handler/comment_authority_test.go` — matching/missing/contradictory context cases and direct handler backstop.
- `server/cmd/server/comment_authority_spoof_test.go` — local valid-hint HTTP matrix and event/state assertions.
- `server/cmd/server/comment_authority_integration_test.go` — actual daemon claim/start token issuance controls, not provider execution.
- `server/internal/handler/autopilot_mention_authority_test.go` — corrected direct-call task context.
- `internal/auth/principal.go` — legacy AgentActor hints are deny-only, never authorization.
- Server security-boundaries/task-api-capabilities specs and root state/security rules — existing credential lifecycle, membership and mutation boundaries.

## Caveats / Not Found

- No product edits, tests, services, SQL, Git or offensive actions performed. This is source and retained local-test review.
- Existing security spec/report language describing only the two role predicates predates this follow-up; parent should update it with the final helper and completed regressions in the final report/spec pass.
- No claim that broader legacy resolveActor consumers changed or were reviewed. No new generic upload permission contract inferred.
- Final full-package verification is not replaced by this focused GREEN or source review. No external sources required.
