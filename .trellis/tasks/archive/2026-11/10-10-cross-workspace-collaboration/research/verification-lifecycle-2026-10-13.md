# P2 lifecycle and manual status service verification

Date: 2026-10-13. This evidence covers the service/SQL slice for R19, R34–R36
and R47. HTTP/core adapters, complete P1–P8 acceptance, native execution and
enablement remain owned by the main task.

## Implemented boundary

- `CollaborationService.ChangeLifecycle` completes, cancels or reopens only for
  the current active creator participant. Source membership is not required.
- `ChangeStatus` permits current active creator/handler participants to change
  between source nonterminal categories. It cannot reopen an ended issue.
- `StatusOptions` returns only currently eligible action options. Its SHA256
  digest includes source identity, action, current/default inputs and sorted
  real catalog key/name/category/archive/update facts. Missing catalog facts do
  not become built-in defaults. Writes require the exact target and digest.
- Close updates source status, lifecycle/authority/revision, permanent session,
  waiting cancellations, stop intent, iteration facts, history and receipt in
  one transaction. The shared stop primitive retains the first forced root,
  fence/nonce and any unknown barrier. Only the existing server-proven absence
  of a launch permit can settle a run without daemon proof.
- Reopen retains the session and old run/command records, advances lifecycle
  and control versions and leaves the queue paused. Close/reopen supersede the
  information chain. Manual status changes preserve that chain, roles,
  authority/lifecycle/control versions, queue and commands.
- Status/lifecycle changes append history/outbox events. They do not create
  account notifications or invoke ordinary AI dispatch.

The existing workflow owner retains sorted user/account locks, source workspace
KEY SHARE, catalog fence, I1 fence/iteration, source issue, collaboration/session
and participant locks. Status actions additionally take ordered catalog row
SHARE NOWAIT locks before I1/source locks: metadata editing does not take an
exclusive catalog advisory lock. The bounded owner retries whole rolled-back
lock conflicts, never an uncertain commit.

Repeated keys retain original status/code/revision. Supplemental status choices
are current projections: a refused request may receive freshly authorized
options, or none after the action becomes ineligible. Reprojection never changes
the stored original result, retries the callback or rebases the request.
`GetWorkflowOperation` still returns only the actor's minimal code/status/revision.

## Files

- `server/internal/service/collaboration_lifecycle.go`
- `server/internal/service/collaboration_lifecycle_test.go`
- `server/pkg/db/queries/collaboration_lifecycle.sql`
- `server/pkg/db/generated/collaboration_lifecycle.sql.go` (root ran sqlc)
- Small shared-owner extensions in `collaboration_workflow.go`: action roles,
  target requirements, catalog locks, supplemental choices and recovery kinds.

No migration, dependency, queue/auth/router implementation or creation/Resume
enablement was added by this slice.

## Environment and commands

Task-owned PostgreSQL database: `multica_collaboration_workflow_1791893622377`,
migrated through 616. Environment: `/tmp/multica-collaboration.Qp1TA6/workflow-env`.
The previous task environment was not used. All DB tests ran while the main
session explicitly released the shared service-test lane.

```sh
cd server
set -a
source /tmp/multica-collaboration.Qp1TA6/workflow-env
set +a
../scripts/go-test-with-agent-cli-guard.sh go test ./internal/service \
  -run '^TestCollaboration(Lifecycle|Status)' -race -count=1 -timeout=180s
../scripts/go-test-with-agent-cli-guard.sh go test ./internal/service \
  -run '^TestCollaboration' -race -count=1 -timeout=240s
go vet ./internal/service
```

The tests were written before service implementation; the initial compile failed
on the missing lifecycle/status methods. The first DB attempt also exposed that
the low-level workspace fixture omits real workspace creation's catalog seeding.
The new fixture explicitly seeds source statuses; production validation was not
weakened. The focused service run then passed (12.758s), followed by the expanded
race run (8.331s). The final all-collaboration race result is recorded below.
`go vet ./internal/service`, `gofmt -l` on changed Go files and scoped
`git diff --check` passed.

## Meaningful assertions

- Creator closes and reopens after leaving source membership; noncreator,
  source admin, disabled/revoked creator and machine actors cannot do so.
- All authors' waiting inputs cancel, immutable input and first forced reason
  remain, and permitted runs keep unknown proof and the active barrier.
- Reopen leaves old run records byte-for-byte equivalent, does not revive
  cancelled input, and still rejects Resume while stop is unconfirmed.
- Manual blocked/review changes retain responsibility, information requests,
  execution, notifications and command order while writing real I1/history facts.
- Missing/foreign/archived/terminal catalog choices fail closed; options are
  limited by role/action. Refusals remain refusals after catalog recovery.
- Deterministic PostgreSQL blocking tests cover catalog archive, new permit and
  handler change while the action waits, plus a metadata edit blocked until
  the status write commits. No sleeps determine lock ordering.
- Concurrent close/submission leaves no waiting work; two identical close
  requests create one lifecycle event and one control-version increment.
- Required write failures roll back source/session/commands/epochs/receipt.
  I1 event failure rolls back both close and reopen. A lost commit reply causes
  one attempt and recovers through the original request key.
- Versions above JavaScript's safe integer range remain exact decimal strings.
- Refusal replay after catalog change returns current choices without changing
  the stored receipt; ended/former-manager replay cannot recover stale choices.

No real agent executable, native process proof, paid account, Web/Desktop E2E,
deployment or production enablement was exercised.
