# S3 ordinary create authorization

Date: 2026-10-06. Scope: W06 ordinary HTTP `CreateIssue` authorization after
the owning transaction's catalog/I1 fence; existing member/channel/source-context
and borrowed `CreateInTx` owners keep their current behavior.

## Implementation plan

1. Reproduce lost agent autonomy/identity and assignment grants after a real
   PostgreSQL I1-fence wait. Assert no issue/counter/event/dispatch effects.
2. Add an optional transaction-bound preparation/authorization callback to
   ordinary `IssueService.Create`, after member/catalog/I1 and before source,
   issue, attachment or counter work. Keep its existing four-attempt NOWAIT
   retry owner, isolation, stable request actor/task and error contract.
3. Resolve current agent and originating task with existing SHARE NOWAIT
   references, then validate unchanged actor identity and contributor autonomy.
   Revalidate assigned member/agent/squad and public-to grants under existing
   reference locks. Derive automatic task provenance after the fence; a terminal
   task retains ordinary actor identity but cannot lend human invocation rights.
4. Prove references remain held through commit, retry reauthorization, member
   and forged-header behavior, and source/channel regressions. No general auth
   refactor, new transaction owner, dependency or generated query is planned.

## Results

The database was `multica_i1_auth_20261006c` on local PostgreSQL, migrated
through 566 by the integration owner. All substantive test commands used
`scripts/go-test-with-agent-cli-guard.sh`; no real agent CLI ran.

### Changes and invariants

- `IssueCreateOpts.PrepareInTx` borrows the existing create owner after
  workspace/member/catalog/I1 fences. Each existing retry receives a fresh
  params copy and the same captured transport actor/task. It adds no transaction
  owner, nested retry, isolation override, SQL or dependency.
- Ordinary HTTP creation rechecks agent existence/workspace/archive/autonomy
  and the task's workspace/agent identity under existing SHARE NOWAIT locks.
  Source/attachment/issue/counter writes follow this check. Agent creation still
  authorizes the actual request member, not the agent's owner.
- Agent task provenance is derived from the locked current task. Terminal tasks
  retain ordinary creation identity and the existing workspace-public invocation
  exception, but cannot spend a human originator's private/member grant or retain
  an automatic live `agent_create` stamp after becoming terminal during a wait.
  Explicit `quick_create` origin remains unchanged.
- Assigned member/agent/squad references and relevant public-to grant rows stay
  locked through issue commit. The existing assignee validator and HTTP error
  categories remain in use. Generic member, forged-header, source-context and
  channel creates preserve their existing behavior.
- The integration owner approved extending the concrete grant-row gap to W01
  `UpdateIssue` and existing lifecycle handoffs. Both already held target-agent
  references but did not hold the grant rows deleted by member revocation. They
  now lock those rows without changing their transaction/retry owners.
- Authorization uses **the returned locked grant rows**, via the existing
  `loadedInvocationDecision`. `UpdateAgent` replaces targets after its agent
  update commits, so an already-started replacement may insert a new grant even
  while an issue writer holds the agent. A later plain allow-list read must not
  admit a newly inserted row that this operation never locked. Lifecycle applies
  this decision only after its originating task is locked.

### RED evidence

- Fourteen real I1-fence races (legacy/task-token transports) previously returned
  201 after demotion, archive, task deletion/reassignment, or task termination
  invalidated private assignment rights. They incremented the issue counter,
  published events and woke execution.
- Assignment grant removal, target/squad archive and squad-leader replacement
  previously committed stale assignments. Final coverage also includes assigned
  member removal for member and agent actors.
- NOWAIT commit probes could mutate agent/task and target agent/squad/member/
  grant references before the create committed. A real lock refusal followed by
  demotion/task reassignment was not reauthorized because no actor-reference
  locking query ran.
- Allowed unassigned/workspace-public creation after task termination retained
  the preflight live provenance stamp.
- Both agent/squad cases for W01 and lifecycle permitted grant-row UPDATE locks
  before commit; all four now receive PostgreSQL 55P03.
- A deterministic insertion after the grant-locking read previously authorized
  create (201), update (200) and lifecycle (201). All three now return 403 and
  preserve the source, issue counter, audit and dispatch snapshot.
- Initial fixture failures (task accountable/originator CHECK and names reused
  after deliberately successful RED writes) were corrected before GREEN. These
  setup failures are not counted as authorization evidence.

### Final verification

- Final widened handler regression: **286 passed, 0 failed, 0 skipped**,
  including subtests and excluding package-level JSON summary events. Covers
  the new authorization tests, ordinary create and forged-header controls,
  source-context, lifecycle rollback/reuse, originator E2E, ordinary updates,
  project concurrency and member revocation.
- Service creation/content/media compatibility: **7 passed, 0 failed, 0 skipped**.
- Channel engine issue-command/media compatibility: **17 passed, 0 failed,
  0 skipped**.
- Handler/service/channel build and vet passed. `git diff --check` passed.
- The final three late-insertion cases additionally assert the unchanged
  whole-state snapshot after refusal; their final focused rerun is recorded
  separately from the wider run.

```sh
DATABASE_URL='<isolated database URL>' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 2 -parallel 2 ./internal/handler \
  -run '^Test(CreateIssue|IssueCreate|CreationIteration|SourceContextCreateIteration|AgentCreateOriginator|LifecycleAtomic|CreateLifecycleHandoff|UpdateIssue|ProjectAssociation|CommentSourceContextLifecycle|IssueAssignment)' \
  -count=1 -json

DATABASE_URL='<isolated database URL>' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race ./internal/handler \
  -run '^TestIssueAssignmentIgnoresGrantInsertedAfterLockRead$' -count=1 -json

DATABASE_URL='<isolated database URL>' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 1 -parallel 2 ./internal/service \
  -run '^Test(IssueCount|IssueCreate|CreateMediaGated|SanitizeIssueCreate|IssueContent|HydrateDeferred|PublishAttachments)' \
  -count=1 -json

DATABASE_URL='<isolated database URL>' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 2 -parallel 2 ./internal/integrations/channel/engine \
  -run '^Test(Router_IssueCommand|Router_BareIssue|ChannelIssueCommand|BindMediaRefs_Issue|MaterializeIssueChannelMedia)' \
  -count=1 -json
```

Local logs: `/tmp/i1-agent-create-auth-red.jsonl`,
`/tmp/i1-assignment-grants-red.jsonl`,
`/tmp/i1-late-assignment-grants-red.jsonl`,
`/tmp/i1-agent-create-auth-handler-final.jsonl`,
`/tmp/i1-late-assignment-grants-final.jsonl`,
`/tmp/i1-agent-create-auth-service-regression.jsonl`,
`/tmp/i1-agent-create-auth-channel-regression.jsonl`.

## Remaining boundaries

This slice does not certify FG or all backend authorization surfaces. Task-token
credential checks keep the established middleware point-in-time contract; this
work holds member/agent/task/reference authority through issue commit and does
not add a credential-store lock protocol. Ordinary creation still enqueues after
its issue transaction commits; issue+enqueue atomicity is not claimed. The
separate lifecycle handoff retains its existing atomic issue/audit/enqueue owner.
Capability/settings/operation orchestration, capacity evidence, full-package
checks, remote CI and product lifecycle/UI gates belong to the integration owner.
