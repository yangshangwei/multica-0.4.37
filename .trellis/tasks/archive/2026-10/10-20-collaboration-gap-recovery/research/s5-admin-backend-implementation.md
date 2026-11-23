# S5 limited collaboration administration backend

Status: bounded backend implemented and verified. Frontend/native integration
and independent review remain with the coordinator. No full P7 or parent
acceptance claim.

## Boundary and reuse plan

The missing behavior is a dedicated, minimal platform administration path for a
disabled creator's complete/cancel/reopen actions. The ordinary collaboration
capability resolver and lifecycle owner remain unchanged. The service borrows
`collaborationWorkflowTx.changeLifecycle` and, only for a double-disabled reopen,
`changeHandler(..., "admin_reopen", reason)` inside its own transaction. The
existing immutable cancellation provenance and termination ledger stay in that
same transaction.

The write locks the existing platform advisory fence, sorted workflow users
(including the real administrator and explicit successor), source workspace,
active organization/mapping, status catalog, W01/iteration, issue, collaboration,
session, channels and targets. It then rechecks the user set, actual password
session/version/role, source scope, disabled creator, captured revision/status
and current candidate account/participation. A changed lock set is refused;
uncertain commits are never automatically retried.

The existing `admin_operation` retains real actor/organization/key and an explicit
non-secret payload digest, including the successor. `collaboration_operation`
retains the precisely correlated minimal result under an administration-only
kind. Both ledgers, lifecycle effects, successor handoff, information-request
supersession, audit and collaboration history/outbox commit together. No new
schema or dependency is planned. The original creator and ordinary permissions
remain unchanged, and reopen keeps the issue paused and old proof barriers.

## API contract for frontend integration

All routes are under `/api/admin/collaboration` and require a complete password
JWT/cookie, administration enabled, a current super-admin role and active source
organization scope. The administrator is never enrolled by this interface.

- `GET /issues?limit=50&cursor=<issue UUID>` returns `items` and nullable
  `next_cursor`. Items contain `issue_id`, `identifier`, `source_workspace_id`,
  `source_name`, `status`, `status_category`, `collaboration_revision` (decimal
  string), `creator`, `handler`, `queue_state` and `unsettled_runs`.
  Creator/handler summaries contain `user_id`, `name`, `avatar_url`, `disabled`.
  Only disabled-creator issues are listed; no title, body, messages or run logs.
- `GET /issues/{id}` adds `actions` (the existing
  `CollaborationStatusOptions` contract) and `successor_required`.
- `GET /issues/{id}/reopen-candidates?limit=50&cursor=<user UUID>` requires a
  closed issue and both creator and handler disabled. It returns `issue_id`,
  `collaboration_revision`, `items` and nullable `next_cursor`; candidates use
  `user_id`, `name`, `avatar_url`, `external`. No general account directory.
- `POST /issues/{id}/lifecycle` uses UUID `Idempotency-Key` and JSON fields
  `action`, `expected_collaboration_revision` (decimal string),
  `expected_status`, `target_status`, `status_options_version`, `reason`, and
  optional `successor_user_id`. The successor is required only for the
  double-disabled reopen and forbidden otherwise, including explicit null.
- `GET /issues/{id}/operations/{requestId}` recovers the same result under the
  current administrator and source-scope checks. Creator restoration prevents
  new changes but does not prevent this administrator from reading their own
  already committed result.

POST and recovery return `{operation, result}`. `operation` is the existing
minimal administration operation DTO. `result` contains `issue_id`, `request_id`,
`action`, `code`, `collaboration_revision`, `creator_user_id`,
`previous_handler_user_id`, `handler_user_id`, `previous_status`, `status`,
`queue_state`, `control_version` (decimal string), and `unsettled_runs`.
The result is historical; clients refresh current metadata and never infer
termination from a successful business lifecycle action.

## Ownership

This lane owns new `admin_collaboration` service/handler/query/test files and the
minimal new action allowlist in `platform_admin.go`. The coordinator owns sqlc,
router, shared API/schema/barrels, locales and frontend integration. Existing
lifecycle/workflow source need no edits. No commits, archive, rollout, production
auth changes or real provider/account use are part of this lane.

## Implementation and preserved behavior

- New service: `server/internal/service/admin_collaboration.go` and
  `admin_collaboration_view.go`; handler: `server/internal/handler/admin_collaboration.go`.
- Queries: `server/pkg/db/queries/admin_collaboration.sql`; the coordinator ran
  sqlc and owns the generated `admin_collaboration.sql.go` and five router entries.
- The only existing service edit is the three-kind allowlist in
  `platform_admin.go`. Existing lifecycle, workflow, cancellation and proof
  helpers were reused without editing their implementation.
- The usual lifecycle event remains. A separate `admin_completed`,
  `admin_cancelled` or `admin_reopened` history/outbox entry records the actual
  administrator, original request, reason, exact optional handoff, and before/
  after creator/handler/status/lifecycle/control facts. It exposes no private
  issue body or resource data through administration responses.
- Workbench responsibility derives from the same committed issue handler and
  nonterminal status. Reopen neither creates participants nor restores grants,
  cancelled commands, channels, ordinary AI tasks or old process reservations.
- The post-commit ordinary-parent wakeup remains the existing transient effect;
  operation recovery and replay never produce a second wakeup.

## Verification log

- Database: task-owned `multica_gap_service_s5`, through migration 737, using
  `/tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py` and the repository agent
  CLI guard. No other lane's database is used.
- First `TestAdminCollaborationOperationKinds` attempt was blocked by a pending
  S4 generated query (`CollaborationRunHasPreparationStartEvidence`). This is
  preserved in `s5-operations-red.log` and is not a behavioral red result.
- The actual operation red run rejected all three kinds with `invalid_operation`
  (`s5-operations-behavior-red.log`); the minimal allowlist made them pass
  (`s5-operations-green.log`). Lifecycle and double-disabled successor tests
  initially returned the unavailable scaffold result (`s5-lifecycle-red.log`),
  then passed with the real transaction implementation (`s5-lifecycle-green.log`).
- A preliminary handler build was blocked before execution by another pending
  S4 generated query (`CollaborationAgentHasPreparationSkills`), recorded in
  `s5-handler-build.log`. All actual database test executions used `service_s5`.
- First HTTP run passed two tests and failed one fixture registration because
  its generated username exceeded 32 characters (`s5-handler-test.log`). The
  fixture prefix was shortened; all three passed in `s5-handler-test-2.log`.
- Fresh final service race run: **76 top-level tests passed, 0 failed, 0 skipped**,
  including all **18 S5 service tests**, plus lifecycle/workflow/status and
  platform administration regressions. Package elapsed time: 197.042 seconds.
- Fresh handler race runs: **4 neighboring tests** and **3 S5 HTTP tests** passed,
  0 failed, 0 skipped. The neighboring run's anchored `AdminCollaboration` branch
  did not select prefixed S5 tests; the separate prefix run explicitly verified
  all three. Package elapsed times: 8.655 and 11.752 seconds.
- `GOTOOLCHAIN=go1.26.6 go vet ./internal/service ./internal/handler` passed;
  `gofmt -l` on this lane's Go files returned no paths and `git diff --check`
  passed. No new dependencies, migrations, real provider invocation, production
  account access or authentication configuration changes were made.

Commands below ran from `server/` with the task-owned wrapper and CLI guard:

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service -run '^Test(AdminCollaboration|PlatformAdmin|Collaboration(Lifecycle|Workflow|Status))' -count=1 -json
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/handler -run '^Test(AdminCollaboration|AdminMeUsesHumanSessionWithoutWorkspace|AdminFeatureGateAndRoleOperationRecovery|LegacyIssueWritesCannotBypassCollaborationLifecycle|LegacyCollaborationCommentAndRerunPathsStayInert)$' -count=1 -json
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/handler -run '^TestAdminCollaboration' -count=1 -json
GOTOOLCHAIN=go1.26.6 go vet ./internal/service ./internal/handler
```

Raw outputs are in that `/tmp` task directory as `s5-service-race.jsonl`,
`s5-handler-race.jsonl`, `s5-handler-race-final.jsonl` and `s5-vet.log`.
`s5-verification.json` records all 83 passing top-level test identities, package
results, the current HEAD and SHA-256 hashes of the delivered backend sources.

The regression matrix covers full password identity, super-admin versus
observer/source-owner, disabled/stale/restricted administrator, active/departed/
restored creator, source and organization changes, exact revision/status/catalog,
required/forbidden/unavailable successors, same creator/handler, current password
candidate readiness, both-lock ordering outcomes, lock retention through commit,
every required write rollback, iteration failure, old-key payload conflicts,
lost commit responses and concurrent same-key submissions. Assertions inspect
the unchanged full queue ledger and both operation stores, true actor/creator
audit, information supersession, workbench ownership, cancellation provenance,
unknown run/proof/reservation retention and negative ordinary content/resource
access.

## Remaining handoff

The coordinator owns Web/Desktop shared forms, current-scope query/draft recovery,
native/production browser validation, generated/common integration, and the
independent review. These service/HTTP fixtures are synthetic password accounts;
they are not evidence of production-Web or actual Electron P7 delivery. The
creation/execution rollout gates and parent acceptance status remain unchanged.
