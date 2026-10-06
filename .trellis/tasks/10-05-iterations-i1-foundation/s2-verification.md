# S2 writer foundation evidence

S1 committed as `837e1abaf` on `codex/projects-p1`. Its independent verification
and frozen performance thresholds are in [S0/S1](s0-s1-verification.md).
S2 is in progress; FG is not passed. No lifecycle/history/UI implementation,
production enablement, push or deployment.

## W02: Plugin/public content updates

Real RED: `TestPatchPluginIssueIterationFacts`, an actual install-token PATCH
with If-Match, succeeded but produced zero events instead of one.
`/tmp/i1-s2-w02-red.log`, exit 1. Work extends the borrowed recorder to an
explicit plugin actor (installation UUID, no fabricated human identity) and
moves UpdateContent into an authorized transaction with current locked facts.

Review identified a conditional-write error regression when the issue disappears
after preflight: the new locking read must retain the former 409 revision
conflict, not return 500. Both missing-issue and missing-workspace lock failures
now retain that contract for conditional writes. The missing-issue begin-barrier
handler test and missing-workspace service test pass; unconditional behavior and
live-workspace membership-revocation errors remain distinct. Final read-only
review found no remaining W02 code blocker.

`/tmp/i1-s2-w02-final.log`: guarded handler race run with
`-run 'Test(PatchPluginIssueIteration|PluginIssue|PluginInstallToken|PluginAction)'`,
16 top-level / 25 including subtests passed, no failures/skips. It covers actual
member/install-token attribution, no-op/description-only no events, CAS/ETag,
event failure rollback, disabled/scopes/token/uninstalled/member pre-Begin
revocation, installation/member locks held until commit, callback revocation
while waiting for I1, and the deleted-issue conditional conflict.

`/tmp/i1-s2-w02-service-final.log`: guarded service race run with
`-run 'Test(IssueContentDeletedWorkspacePreservesConditionalConflict|TriageStaleContentEditPreservesAcceptedAssignment)'`,
both passed. Authorization is mandatory in `IssueService.UpdateContent`, which
owns one default-isolation transaction with no new retry loop. Actor identity is
checked again after the issue lock; plugin actors have the installation UUID and
null user_id, while member actors retain via_plugin_id. Installation/scopes and
member locks cover the commit. The in-memory callback grant is re-resolved after
locks but not held through commit; this slice does not claim to change the
callback token store's revocation linearization contract.

## W15: complete workspace deletion

Committed as `7d53c9ed0` after the final combined checks below. W02 is committed
with this completed verification record.

Real RED: `TestDeleteWorkspaceRemovesIterationHistoryAndKeepsNeighbour` deleted
the workspace successfully but left one row in each of the seven I1 tables.
`/tmp/multica-i1-s2-w15-red.log`, exit 1. Existing inbox deletion already removed
the protected inbox payload. The rollback companion passed before implementation
and is retained to guard the newly inserted cleanup step.

`DeleteWorkspaceIterationData` explicitly purges notification, operation,
snapshot, event, participation, iteration and settings for the resolved workspace.
It runs after inbox leaf cleanup, before issue roots, in the existing deletion
transaction under the workspace FOR UPDATE lock. Ordinary issue/project deletion
does not call it. No FK or migration is added.

Initial sqlc generation rejected unqualified workspace_id references in the
multi-delete CTE; qualifying every table fixed generation. Logs:
`/tmp/multica-i1-s2-sqlc.log` (failed),
`/tmp/multica-i1-s2-sqlc-qualified.log` (exit 0).

The two cleanup/rollback tests then passed under race detection in
`/tmp/multica-i1-s2-w15-green-final.log`. The first expanded deletion suite had
18 passing tests and one test-fixture failure: assigning a new request context
after setting chi route parameters discarded the workspace id. The concurrency
test never reached the delete lock and timed out. The correction reattaches
the route parameter after context assignment; the original output remains
`/tmp/multica-i1-s2-w15-final.jsonl`.

Final corrected `-race -p 2 -parallel 2 ./internal/handler -run '^TestDeleteWorkspace' -count=1 -json`
passed all 19 top-level tests, no failures/skips;
`/tmp/multica-i1-s2-w15-corrected.jsonl`. The new concurrent case observes actual
server waiting with pg_blocking_pids, releases the real W01 transaction, verifies
that the delete transaction sees both the old and newly committed event, then
asserts zero rows in all seven I1 tables and inbox. The neighbour preservation and
post-cleanup failure rollback fixtures also pass. Final read-only review found
no W15 code blocker.

All DB commands use the task-only local `multica_i1_s1_20261006` database and the
agent CLI guard. No original development database was migrated or cleaned.

## Shared verification

Go build and vet (`go -C server {build,vet} -p 2 ./...`) both exited 0;
`/tmp/multica-i1-s2-build.log`, `/tmp/multica-i1-s2-vet.log`. Final sqlc regeneration
matched all 81 generated file SHA256 values;
`/tmp/multica-i1-s2-sqlc-final.log` and `/tmp/multica-i1-s2-sqlc-before.json`.
The final combined W01/W02/W15/P1 handler race run passed 120 entries (63
top-level), no failures/skips; `/tmp/multica-i1-s2-combined-final.jsonl`. Command:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 \
  ./internal/handler -run 'Test(UpdateIssueIteration|IssueWriteFenceSQL|PatchPluginIssueIteration|PluginIssue|PluginInstallToken|PluginAction|DeleteWorkspace|ProjectAssociation|ConcurrentRevisionWritesHaveExactlyOneWinner|NoOpIssue|IssueUpdateAndAttachmentBindingExcludeInterleavingMutation|IssueUpdateRollsBackWhenAttachmentBindingFails|UpdateIssueReassign|BatchUpdateIssueReassign|UpdateIssueCancelStatus|BatchUpdateIssueCancelStatus)' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/iteration -count=1 -json
```

Iteration tests passed 69 entries (30 top-level), no failures/skips;
`/tmp/multica-i1-s2-iteration-final.jsonl`. Together with the two service tests
above, these establish W02/W15 and their immediate integration regression scope.
gofmt and `git diff --check` are clean. No S2 full-repository Go/E2E/CI claim.
No frontend files or response DTOs changed; S1's 10,054 TS tests and 15 static
tasks remain the latest TS evidence rather than claimed new S2 test runs.

## Remaining FG work

W03–W14, W16 and W17 still require actual production-path integration or evidenced
exemption. W02/W15 completion alone does not prove FG. Durable operation query/
replay authorization, settings/capability, explicit-field compatibility, member
revocation cleanup and non-recreation, remaining two-connection races, populated
workspace throughput and server-side lock-wait measurements remain outstanding.
