# S2 W13/W14 batch facts

Starting baseline: clean `codex/projects-p1` at `65a76d756`. Earlier S0/S1,
W02–W05, W15 and W17 evidence remains in the linked foundation records. W13/W14
are in progress and FG is not passed. No capability, lifecycle, closure or UI work.

## W13 real RED and initial GREEN

`/tmp/w13-red.log` proves the prior DeleteSquad path transferred issue/autopilot
assignments and archived without an owning transaction or iteration events.
Foreign-workspace rows using the same squad UUID were also transferred. The old
path bypassed transaction-time member/agent/leader revalidation. Initial fixture
runtime/context errors were corrected before the valid behavioral RED.

`/tmp/w13-green.log` completed the new focused race suite with exit 0 (8 top-level
test functions and 13 subcases in the source). Root's JSON/full regression pass is
still required to make the final no-skip/count claim. The implementation owns one
RC transaction, holds workspace/member/catalog/I1 then Squad→leader→Autopilot
locks, locks iterations/issues by ID, prepares facts, transfers and archives in
one commit. New queries carry workspace predicates. Current creator/agent
authority, latest leader, rollback after later writes, actual issue/automation
association races and no execution changes are covered by the focused tests.

## Shared batch preparation and W14 RED

The agreed `PrepareIssueRecords` API returns index-aligned records (nil for
unassociated issues), reuses single-record fact validation, finishes all
participation locks, then samples once for the entire operation. The existing
single-record entry preserves its query/sampling behavior.

The initial W14 attempt could not compile while W13 referenced pending shared
batch/sqlc symbols: `/tmp/i1-s2-w14-red.log`. It is a prerequisite failure, not
behavioral RED. W13's valid RED justified implementing the batch helper, then a
single coordinated sqlc generation included both stable query files
(`/tmp/i1-s2-w14-sqlc.log`). With DeleteProject still unwired, W14 behavioral RED
then succeeded: HTTP 204 but zero events instead of three current participants,
`/tmp/i1-s2-w14-red2.log`.

W14 keeps P1's RC owner, three-attempt database-conflict budget and project CAS.
Lock order is catalog/I1→project→automation parents→triggers→iterations→issues→
participations→single sample. Actor and operation identity are outside the retry
callback. Existing cleanup remains transactional; only associated iteration
participants need an after-row reload/record.

## Existing FK discovery

First W14 GREEN attempt passed rollback/retry/late-join but failed neighbour
preservation, `/tmp/i1-s2-w14-green.log`. The fixture deliberately linked a foreign
workspace issue to the victim project. Migration 034's historical
`ON DELETE SET NULL` then changed that row despite the scoped application detach.
This is malformed data under current API invariants, but it exposes an actual
implicit writer and must not be silently relabeled as a pass.

Keep the normal neighbour fixture valid. A distinct malformed-reference case
proves current deletion succeeds when it must refuse: expected409/got204 in
`/tmp/i1-s2-w14-foreign-red2.log`; the earlier file of that name without `2`
contains a fixture empty-UUID failure, not behavioral RED. The first guard made
the six focused tests pass at `/tmp/i1-s2-w14-green2.log`.

Final review then identified two more children in the same legacy FK boundary.
An actual `pg_constraint` query against the isolated DB confirmed exactly these
three project child FKs: issue (SET NULL, 034), autopilot (SET NULL, 097), and
project_resource (CASCADE, 065). The guard must cover all three malformed foreign
reference shapes, under the exclusive project lock before mutations, returning
only a generic conflict. This is an integrity refusal; an operator must repair
the invalid relationship before retrying deletion. No FK/migration is added or
rewritten. The three-table extension is implemented and verified below.

## Verification environment and intermediate results

To avoid TestMain fixture collisions, root created task-only
`multica_i1_w13_20261006` and migrated from empty through566 (exit0),
`/tmp/multica-i1-w13-migrate.log`. W14 retains `multica_i1_s1_20261006`.
Neither is the original development database. SQL generation remains serialized.

On the W13 DB, root's broader squad/creator/autopilot/association race run passed
135 entries (69 top-level), no skips/failures,
`/tmp/multica-i1-s2-w13-regressions.jsonl`. An initial combined P1/I1 pass had
339 entries (191 top-level), no skips/failures,
`/tmp/multica-i1-s2-batch-handler-final.jsonl`. Build/vet exited0 and all81 sqlc
hashes matched regeneration. These checks preceded the final three-table guard
extension and must not be treated as its final verification.

## Resumed final verification (2026-10-06)

The three malformed foreign-reference cases (issue, autopilot, project_resource)
now assert generic 409, unchanged foreign rows and an intact project. Prior
behavioral RED is preserved in `/tmp/i1-s2-w14-foreign-all-red.log`.

Fresh guarded race regression on the isolated `multica_i1_w13_20261006` database
passed 138 test entries (54 top-level), with zero failures or skips:
`/tmp/i1-batch-resume-final.jsonl`. The selection includes DeleteProjectIteration,
DeleteSquadIteration, ProjectDelete, ProjectAssociation, Squad, Creator,
Autopilot/Squad, UpdateIssueIteration, PatchPluginIssueIteration,
PullRequestWebhookIteration and IssueWriteFenceSQL.

`go -C server build -p 2 ./...`, `go -C server vet -p 2 ./...` and `make sqlc`
exited 0; logs are `/tmp/i1-batch-resume-{build,vet,sqlc}.log`. gofmt and
`git diff --check` are clean. These are scoped checks, not full FG acceptance.

Independent read-only review found no direct blocker in this batch. It identified
an existing W07 gap: dispatchCreateIssue can retain a cached Squad assignee after
archive wins, because the transaction previously refreshed only ProjectID. That
real dispatch race belongs to the next creation-writer slice and remains open;
W13 coverage does not imply every production assignment entry is integrated.
