# I1 original §6.2–6.3 history fields and participation lookup

2026-10-07. Backend implementation slice; final project gate remains owned by parent.

## Changes

- `HistoricalIssue.priority` is nullable string; `labels` is nullable array of `{id,name}`. New captures always provide actual priority and an array (empty means known no labels). Missing/null old snapshot values remain unknown; no live fallback or schema-version rewrite.
- Current/original issue list accepts priority (urgent/high/medium/low/none) and label_id UUID filters. Unknown historical values cannot match a concrete filter. Filters do not alter whole-period statistics. Existing filters and DTO stay intact.
- `GET iterations?issue_id=UUID` uses tenant-scoped participation EXISTS and normal collection DTO/keyset pagination. It includes periods of tasks added then removed, including deleted tasks; no live issue join, no dependence on O/S. Nonexistent or other-workspace issue ID returns empty under current workspace authorization. Malformed/repeated/empty/zero issue ID rejected.
- Label display capture is one batched query for the affected issue set. Writer capture SHARE NOWAIT locks existing junction/label rows; the four production label mutation owners (attach/detach/rename/delete) take workspace KEY SHARE → status-catalog shared → I1 workspace fence before any mutation. This fence excludes new-assignment phantoms too. Existing creation/admission owners already hold I1 fences. No FK, migration, dependency, or per-issue label lookup added.
- Preview hash includes captured historical display facts. Rename/delete that leaves issue revision unchanged still causes stale409 after preview. Issue list cursors additionally bind the coherent display projection so labels/priority/reference changes cannot silently cross page boundaries.
- Label mutations remain separate from scope/statistics events. Their existing realtime label events should invalidate live iteration history on clients. Generic label rename/delete (including other resource types) now incurs workspace serialization; ordinary issue writer query counts unchanged.

## Verification

Exclusive DB: `multica_i1_final_20261007_0538`, localhost:5432. Parent created/migrated it and assigned it exclusively to this slice. All Go test commands used the agent CLI guard. No real agent CLIs invoked.

Initial compile-red fixture confirmed missing priority/labels fields. A subsequent command accidentally omitted DATABASE_URL: handler TestMain used its default `multica` DB; the new test failed immediately with missing `iteration` table before any I1 fixture inserted. TestMain runs its standard named fixture cleanup; no manual cleanup of unrelated data was performed. All later DB invocations explicitly used the exclusive DB.

```
DATABASE_URL='postgres://multica:multica@localhost:5432/multica_i1_final_20261007_0538?sslmode=disable' \
 bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 -parallel 2 \
 ./internal/iteration ./internal/service ./internal/handler \
 -run '^(TestIteration|TestHistorySnapshot|TestLabel|TestAttachLabel|TestDetachLabel)' -count=1
```

PASS: iteration 1.679s, service 14.598s, handler 6.640s. Captured output `/tmp/i1-history-fields-regression.log`.

Additional legacy snapshot focused race suite passed (iteration 1.622s) after adding explicit omission of old priority/labels JSON fields.

Canonical new regression files:

- `server/internal/handler/iteration_history_fields_test.go`: priority/label filters, stale label-rename cursor, retained/deleted participation, actual HTTP post-close label rename/delete + priority update + task delete, stable snapshot digest, original fields not backfilled; ten two-connection ordered races (attach/detach/rename/delete/priority × writer-first/closure-first). `pg_blocking_pids` verifies actual lock queues; new attachment phantom must remain outside a closure-first snapshot; writer-first closure returns409 with no snapshot.
- `server/internal/service/iteration_history_fields_test.go`: actual start captures priority/label name; rename invalidates close preview even without issue revision change; original and close values remain distinct.
- `server/internal/iteration/history_fields_test.go`: v1 snapshot with omitted additive fields decodes as unknown.

Initial race fixture issues were corrected: nesting withURLParam dropped issue ID, and same-member subscriber serialization prevented the priority test reaching its intended I1 queue. Using withURLParams and a separate authorized closing member makes the queue assertion test the intended production boundary.

## Remaining integration

Parent owns API contract documentation/client schema/filter/group controls/realtime invalidation, full checks, P95 rerun and Web/Desktop E2E. No commits, push, merge, deployment, or release-flag change performed by this slice.

`go -C server vet ./internal/iteration ./internal/service ./internal/handler` and `git diff --check` both passed after the final edits. Formatting applied with gofmt. sqlc regenerated through `make sqlc` (pinned go-run tool); unrelated preexisting generated changes were preserved.

## Follow-up: authorization held across label-write waits

Parent review identified a current-permission gap in the first label fence: a preflight membership/actor decision could outlive an I1 wait. Fixed `beginIssueLabelWrite` to reuse the exact ordinary issue `lockIssueWriteFences` batch (workspace KEY SHARE → subscriber fence → current member SHARE → catalog shared → I1), then lock the resolved iteration/issue for assignment edits and reload the scoped label. Missing current member yields403; a deleted/moved issue or missing/changed-resource label yields404, without generic500 masking. Agent and origin task references are locked and revalidated after the wait; archived agent, missing/rebound task, or changed resolved actor yields403. No cached member, grants, or actor result is accepted as authority after waiting. Label routes retain their existing autonomy policy: this does not import the stronger delete-issue contributor threshold or grant new rights.

Read-only writer inventory checked all production SQL/Go INSERT/UPDATE/DELETE occurrences of issue_label / issue_to_label, not only generated method calls:

- Catalog insert creates an unassociated identity; the four mutated existing-label owners are in label.go.
- `IssueService.CreateInTx` takes catalog/I1 before creating and attaching labels (`AttachLabelToIssueOnCreate`); its ordinary creation owner follows the same fences.
- `triage_actions.go` raw label replacement runs under the owning triage transaction whose `triage.go` fence precedes acceptance, then locks labels SHARE NOWAIT before replacement.
- Workspace teardown deletes label catalog/links under its workspace exclusive lock, which excludes label writers' workspace KEY SHARE and closure.
- No independent bulk link mutation owner was found outside these paths.

Added real two-connection tests in `iteration_history_fields_test.go`:

- `TestIterationLabelWritesReauthorizeRevokedMember`: revoke-owned subscriber lock, uncommitted member deletion, queued attach/rename/detach/delete; actual PostgreSQL queue verified; after revoke commit every writer returns403 and label/association state stays unchanged.
- `TestIterationLabelWritesKeepBoundMachineIdentity`: task-token label attach waits at I1 fence; winning agent archive or task-agent rebinding returns403 without attachment.

Guarded explicit-exclusive-DB race regression passed (handler 3.513s), regex `^(TestIterationLabelWrites|TestIterationClosureLabelWriterFence|TestLabel|TestAttachLabel|TestDetachLabel|TestUpdateIssueIterationActorTaskFence)`, output `/tmp/i1-label-authorization-regression.log`. Includes prior ten ordered closure races and existing label/actor-task permission tests.

## Follow-up: explicit membership source and target facts (ITR-010)

Membership events now add `source_iteration_id` and `target_iteration_id` to their non-null IssueFacts object. Both keys are always present in new events, including explicit null for known unassigned; old missing keys remain unknown. Source leave/delete carries both keys on before_facts and retains literal-null after_facts. Target join/reenter carries both keys on after_facts and retains literal-null before_facts. Planned activity uses the same shape. Serial and batched membership writers share a small marshal helper; issue deletion also emits source→null. No new SQL, query, table or migration; no modification to IssueFacts or statistical reducer. Existing JSON decoders ignore the additive metadata while raw event DTO/snapshot retains it.

New `server/internal/service/iteration_membership_event_facts_test.go` exercises one and two issue complete operations (serial/batch): null→A, A→B, B→null, null→B, start B, B→C rollover. It asserts both period sides' explicit IDs/nulls, unchanged null scope semantics, before/after rollover counters, event cardinality and frozen pre-close event identities after live period rename. It failed red on absent keys before implementation, then passed guarded race (service 2.238s). `TestIterationDeletionEventKeepsMembershipTransition` drives real HTTP deletion, asserts source→null metadata and unchanged current0/original1/removed1 statistics. Older HG fixtures without metadata continue through the unchanged reducer.

Full relevant guarded explicit-DB race regression passed:

```
DATABASE_URL='postgres://multica:multica@localhost:5432/multica_i1_final_20261007_0538?sslmode=disable' \
 bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 \
 ./internal/iteration ./internal/service ./internal/handler \
 -run '^(TestIteration|TestHistorySnapshot|TestIssueDelete|TestDeleteIssue)' -count=1
```

Result: iteration 1.652s, service 14.811s, handler 7.278s; `/tmp/i1-event-transition-regression.log`. Vet for all three packages and git diff --check passed. Database now idle. UI integration should render transition IDs from whichever facts object is non-null, not infer a missing legacy source as unassigned; live display names may be resolved independently but IDs remain frozen. Closure-created leaves remain in the append-only audit stream; the existing pre-release snapshot event cutoff and explicit snapshot destinations are unchanged.

## Contract-only synchronization during P95 freeze

Updated only parent `api-contract.md` and this note during the parent-owned performance run. No source edits, tests or builds in this follow-up. The contract now explicitly documents additive priority/labels (old omission/null unknown versus new none/empty known), exact priority and label_id filters, participation-based issue_id collection filtering, issue cursor display digest, preview hash historical display inputs, membership source/target metadata and existing facts-null semantics, and member/actor/label transaction fences.

Field names and behavior were re-read from current code: `HistoricalLabel` / `HistoricalIssue` in `iteration/types.go`; `validateHistoryIssueFilters`, `matchesHistoricalIssue`, `ListIterationIssues` in handler iteration_history.go; `ListIterations` and iteration_list.sql; `marshalMembershipFacts` in membership.go; `beginIssueLabelWrite` in label.go. Actual protocol event values checked in `protocol/events.go` are `label:updated`, `label:deleted`, `issue_labels:changed`. Source/target metadata is nested in the non-null before_facts/after_facts object, not Event top-level. The display digest is internal cursor binding, not a response field. Current server marshaling normalizes absent old HistoricalIssue fields to null while compatibility clients still accept omission from older servers.
