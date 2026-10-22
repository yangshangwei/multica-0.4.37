# Independent source canonical and stage-only review

Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`; reviewer
`/root/evidence_check` (`trellis-check`). Read-only except these two `review-*`
files. No DB tests, UI/client launch, environment/provider action, source/spec/
ledger edit or commit. This review does not judge the still-iterating source UI.

## Decision

**Accept the new stage test file for an isolated HEAD+single-file slice.** It
has **3 top-level tests and 6 subcases**, including the positive creator/current
handler case as well as the two commit-order race families. **Hold full AC85
and AC89** until the final source Web/Electron and actual CLI/board receipts.

## Evidence verified

- `research/continuation-2026-10-22/source-projection-parent-canonical.jsonl`:
  independently reparsed **8 top-level /31 total pass, zero fail/skip**, SHA256
  `93245a8510fbf61348f795926e3e094eb65b3a8e956de135e2e66d19786d0a6b`.
  All three source hashes match current. Source and parent test files equal HEAD;
  boundary file has unrelated dirty tests, but the actual position-test function
  and fixture equal HEAD.
- While reviewing, root's new
  `research/continuation-2026-10-22-final/source-machine-runtime-canonical.stdout`
  completed: independently reparsed **23 top-level /75 total pass, zero fail/skip**,
  SHA `fa46a880bb382b3aeed59f0e918726446a4196697d936998f686114c8dfd6d67`.
  Guarded serial race, service_s5, no source drift. All three new stage tests and
  six subcases ran and passed. All relevant recorded source hashes equal current.
- Stage source SHA `a66cf7821a22bb67e7fdca05fdafef2b650b26bf87d78ad90cdd6b38d8b36e89`.
- Existing nonmanager role test remains HEAD/current/isolated-evidence identical:
  `f006430da597c8a62209f72b8f04c32d9104df2f13886b87e8f9b9ac69fe694d`.
  Its isolated role receipt checks seven roles against stage/project/parent/
  iteration and original refusal equality. It is not falsely included in the
  new23/75 selector.

## Actual stage tests and dependency isolation

`sourceStageFixture` uses the existing query-backed fixture and a seeded retained
run; it launches no provider. A real stage-only edit moves1→2, advances each
appropriate revision once and records exactly one event with actor/request ID
and before/after. Replay preserves the receipt, full business snapshot and
retained execution. Both creator and current source handler are positive cases.

The race helper pauses the true first PostgreSQL transaction at Commit, waits
for `pg_blocking_pids` to name that PID before release, and then asserts both
outcomes. This is a concrete lock barrier, not a timing inference. Stage-first
requires a stale transfer refusal then fresh transfer; transfer-first rejects
old-handler stage. Stage-first holds membership through commit; departure-first
rejects stage. Fresh post-race old-handler/member writes remain forbidden with
exact business/execution equality and one transfer at most.

Every reused helper file is already HEAD-identical:

| Helper | Existing HEAD file |
| --- | --- |
| sourceAuthorityCommitOrder | collaboration_source_authority_race_test.go |
| recipientCommitStarter | collaboration_recipient_eligibility_test.go |
| waitWorkflowBlocked / workflowRequest | collaboration_workflow_test.go |
| sourceRoleBusinessSnapshot / sourceRoleExecutionSnapshot | collaboration_source_roles_test.go |
| queue fixture / submit / seeded run | collaboration_queue_test.go |
| controlStateSnapshot | collaboration_control_test.go |
| fieldJSON | collaboration_fields_test.go |

EditIssue/source fields also equal HEAD. Current workflow.go's only dirty change
is participant-departure command cancellation provenance; stage/transfer and
source-member DELETE do not need it. The single added stage file can therefore
be copied over exact HEAD without pulling shared dirty product changes. Root
should run all three identities in that isolation before Lore staging:

`^TestCollaboration(SourceManagerStageOnlyEditRecordsOnce|SourceStage(Transfer|Departure)BothCommitOrders)$`

Use the existing guarded `-race -p 1 -parallel 1 -count=1 -timeout=30m` service_s5
entry. This reviewer has not performed an isolated DB run.

## AC89 exact clause coverage

- Source layout position and machine denial: current handler observes position
  -42, collaboration revision1 and machine403. Source projection test checks no
  collaboration event from reorder. Actual Reorder performs only its guarded
  source position write. Final same-column board and CLI persistence is pending.
- Current source creator/handler sets child stage and retains history: all3 new
  stage tests now provide direct current execution, exact identity and replay.
- Child completion follows ordinary stage wake: source/parent8/31 and current
  repeated23/75 cover agent/squad parent, same/earlier/later/implicit stages,
  custom terminal categories, parent human/shared/backlog/terminal refusals,
  fresh parent check, sanitized unopenable child reference, task provenance,
  original-key non-repetition and confirmed-commit timing.
- Nonmanager stage denial: unchanged seven-role canonical source-role suite
  retains exact refusal and no partial business/execution mutation.
- Transfer/source departure does not reuse old authority: both commit orders
  are now executed directly for stage-only edits; post-race new key still denies.
- Full Web/Electron board+CLI persisted reorder without snapback, history,
  collaboration revision or notifications remains unreviewed here.

## AC85 remaining full clauses

Canonical source handlers support safe external name/tag projection, filter,
table grouping and machine child/minimal-reference boundary. They do not replace
final production Web and actual Electron list/board/timeline/comment name/avatar/
external tag, no Unknown/private account fields and filter assertions. The
complete actual cross-assignee no-optimistic/no-assignee-write, authorized transfer
entry, unauthorized explanation, nonterminal R47, terminal whole-issue explanation
and ordinary drag controls still require final client receipts and current hashes.

## Findings (fixed)

None. No source changes required in this reviewed test-only slice.

## Findings (not fixed)

- Root's exact HEAD+single-file DB run remains a commit prerequisite; existing
  current23/75 is a working-tree canonical receipt, not that isolated run.
- Parent wake keeps the ordinary best-effort contract: post-commit failure or
  uncertain commit may lose the wake; replay does not repair it. These are known,
  explicit tests/spec limits, not a durable delivery claim.
- AC85/89 final UI/CLI remains in progress and is not promoted in this review.

## Verification

- Lint: new file `gofmt -l` exit0, empty output.
- TypeCheck/static: current `GOTOOLCHAIN=go1.26.6 go vet ./internal/service` exit0.
- Tests: raw8/31 and23/75 independently hash-checked/reparsed; stage3+6 pass.
  No DB/client test run by reviewer. TypeScript not applicable in this slice.
