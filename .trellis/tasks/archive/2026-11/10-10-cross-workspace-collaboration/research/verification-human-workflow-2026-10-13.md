# Human workflow service verification — 2026-10-13

Scope: P2 invitation, ordinary transfer, creator removal and voluntary leave.
This is an internal service slice; it does not complete P2/P5/P6 or authorize
creation, shared launch, positive Resume, native process proof or release.

## Change boundary and cleanup plan

- Add `collaboration_workflow.go`, its DB-backed tests, and
  `queries/collaboration_workflow.sql`; root owns generated SQL and API integration.
- Preserve actual global user IDs and independent source membership. Reuse the
  source issue/session authority, operation receipts, event/notification tables,
  workbench predicate and borrowed iteration recorder.
- Extract only the existing StopQueue run-stop block into `stopRun`. Keep the
  original never-permitted predicate, historical run identity and unknown-proof
  barrier unchanged. Parameterize forced disposition/reason for departure;
  preserve the first effective termination root. Existing Stop tests are the
  behavior lock and must pass again after extraction.
- No changes to creation/launch gates, ordinary task enqueue, resource consent,
  provider credentials or native process supervision.

## Implemented contracts

Workflow requests use actor-scoped request IDs plus collaboration, lifecycle,
participant and account epochs. Current human account and role are rechecked
before replay. Source catalog/I1/iteration locks precede issue/session locks;
relevant account IDs are locked in sorted order, including current recipients.
The hint set is rechecked after the issue lock and growth forces whole-transaction
retry. Retry applies only after rollback, never after an uncertain commit.

Invitation directly enrolls, incrementing a revoked participant epoch without
restoring old commands/grants. Transfer preserves status and execution, appends
handoff and iteration facts, and supersedes old information requests. Removal
of the handler returns responsibility to the creator in the same transaction.
Departure revokes participation/grants, cancels the author's waiting commands,
and pauses/stops only affected execution; no permit or proof is fabricated.

Event and notification intent commit with each mutation. Recipients are the
current valid participants excluding the actor. Removed users receive only the
distinct `removed` notification reference. No source inbox/subscriber writes
occur. Content delivery and UI rendering remain later integration obligations.

Minimal actor-owned operation recovery returns only status/code/revision,
including after voluntary leave; it never reloads current issue content or run
state. Required writes, failed role/target/version checks, and exact replay are
covered by durable receipts when the actor can access the object.

## Verification record

- Before extraction: `go test ./internal/service -run '^TestCollaborationQueueStop' -count=1`
  passed against the original task-owned database (10.697s).
- Workflow test first failed on absent workflow types/methods, before service
  implementation. First implementation plus existing Stop tests passed (7.904s).
- Expanded suite compiled, but its DB run encountered missing collaboration
  tables while the parent session was replacing the isolated test database.
  Those failures are environment evidence, not successful validation.
- Interim `go vet ./internal/service` encountered the sibling account invalidation
  query before SQL generation. It must be rerun after generation.

## Final verification

The replacement database `multica_collaboration_workflow_1791893622377` was
migrated by the parent from empty through 616. The private environment at
`/tmp/multica-collaboration.Qp1TA6/workflow-env` contains shell assignments;
commands use `set -a`, source it, and then `set +a`. An initial source-only run
skipped DB cases and was discarded as verification evidence.

1. `go test -race ./internal/service -run '^TestCollaboration(Workflow|Queue)' -count=1 -json`
   passed: **24 top-level tests + 55 subtests, 0 failures, 0 skips**, 21.292s.
   Log: `/tmp/multica-collaboration.Qp1TA6/human-workflow-race.json`.
2. Added explicit closed-session/unknown-status transfer negatives and uncertain
   leave-commit recovery. Ran
   `go test -race ./internal/service -run '^TestCollaborationWorkflow(CurrentAuthorityAndPersistentRefusals|LostLeaveReplyRecoversWithoutObjectAccess)$' -count=1 -json`:
   **2 top-level tests + 10 subtests, 0 failures, 0 skips**, 4.782s.
   Log: `/tmp/multica-collaboration.Qp1TA6/human-workflow-final-cases.json`.
   Across the two runs, all **25 distinct top-level tests + 57 distinct subtests**
   in the workflow/queue slice passed; the new workflow suite owns 14 top-level tests.
3. `go vet ./internal/service` passed after regeneration and again after the
   final cases. All changed Go files were formatted with `gofmt`.
4. `git diff --check` on the owned paths passed; because the foundation files
   remain untracked, an explicit trailing-whitespace/final-newline check also
   covered all four owned Go/SQL source files and passed.

Coverage includes direct offline enrollment, no fabricated membership/consent,
active-transfer preservation, current former-handler/target account checks,
source departure, independent source read access after leaving, machine denial,
actor-owned minimal receipts, exact large revisions, same-key races, conflicting
transfer revisions, persisted refusal after conditions improve, waiting-command
cancellation and immutable input, active owner revocation, unrelated execution,
never-permitted settlement, contradictory evidence/issued-permit barriers,
first termination-root preservation, reinvitation epochs, notification recipient
intent and workbench responsibility, I1 facts/information supersession, twelve
required-write rollback points and lost transfer/leave commit replies.

Concurrency tests wait on PostgreSQL blocking evidence before releasing held
user/session locks; they do not guess race windows with sleeps. Run/permit rows
are explicit fixtures, not evidence of real launch/proof APIs. No tests use an
installed AI/provider CLI.

## Remaining limits

- Parent owns API/core integration and independent review; this report does not
  claim Web/Desktop end-to-end AC or the entire P2 flow is complete.
- Nested information requests/return, acceptance/lifecycle, outbox delivery,
  account invalidation fanout, resource enrollment, promotion, positive Resume
  and native supervisor proofs remain separate work.
- Departure writes durable supervisor stop intent and retains unknown barriers.
  It cannot claim the native process ended without the future proof protocol.
- No commits, deployment, feature enabling or production database actions were
  performed by this implementer.
