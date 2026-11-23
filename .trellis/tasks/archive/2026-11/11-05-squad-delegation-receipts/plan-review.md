# Independent plan review — M1-C1 immediate delegation receipts

Date: 2026-11-05
Reviewer: native `trellis-check` / receipts_review
Scope: planning artifacts and current source; no implementation edits or tests.

## Verdict

**Clear to implement the bounded immediate-consumption slice.** No blocking architecture, permission or API compatibility issue was found in this design. This is not approval of durable delegation evidence, server-derived evaluation, model behavior, or the complete M1 milestone.

The PRD correctly separates saving a comment from accepting each requested target and from completing work. The planned CLI warning preserves successful-write semantics and existing JSON. The injected protocol can require exact-target checks without changing queue behavior or lifecycle acceptance.

## Source evidence

- `server/cmd/multica/cmd_issue.go`, `runIssueCommentAdd` / `runIssueCommentUpdate`: ordinary paths receive a complete `map[string]any`, announce the saved write, then print it as JSON unless table output was selected. Add routes collaboration separately before this path. Place ordinary warning consumption after the successful HTTP response and before the table early return; preserve the collaboration branch.
- `server/internal/handler/comment.go`, `CommentTriggerOutcome` / `commentTriggerOutcomes`: target identity is the mentioned agent or squad, not necessarily the executing leader. Several squads can share a leader and return coalesced outcomes. Only create/edit responses carry this array; a later comment read is not a receipt recovery API.
- `server/internal/dispatch/reason.go`: stable enumeration-safe reasons already exist. Reuse the success status vocabulary; do not infer target existence from `invocation_not_allowed`.
- `server/internal/handler/squad_briefing.go`, `squadOperatingProtocolHeader`: current protocol records evaluation and stops when the comment is saved, without checking per-target results. The planned new step closes that concrete instructional gap.
- `server/internal/handler/squad.go`, `RecordSquadLeaderEvaluation`: activity stores caller-provided outcome/reason after current-task/current-leader authorization. It is not proof that every mentioned target ran; leave its storage meaning unchanged.
- `server/internal/handler/squad_briefing_test.go`: existing owner/guest, one-comment, current-turn recording and dual-trigger tests already lock important boundaries. Extend them instead of replacing their assertions.
- `server/internal/daemon/execenv/runtime_config_sections.go`, `writeMentions`: the current unconditional “enqueues a new run” wording contradicts coalesced/deferred/blocked results and should be corrected within the owned paragraph.
- Existing CLI `cmd_issue_test.go` has `newIssueCommentAddTestCmd`, `newIssueCommentUpdateTestCmd`, fake HTTP server and stdout/stderr capture helpers; reuse them.

## Implementation and acceptance clarifications

These are refinements of existing acceptance criteria, not additional product scope:

1. **Trust the entire target entry, not only status.** An entry with `status: queued` but missing/invalid target type or target ID remains malformed. Treat non-array fields, null/non-object array entries, wrong field types and unknown statuses conservatively without panic. Preserve every response field in stdout; warnings must not mutate the decoded response or echo arbitrary error payloads.
2. **Absence is not evidence.** Ordinary comments and older servers can omit `trigger_outcomes`; the generic CLI should remain compatible and must not infer successful delegation from that omission. The leader knows the intended roster targets and must treat missing matching entries as unconfirmed. No broad content re-parser or second invocation path is needed.
3. **Exact matching includes ambiguity handling.** Match each intended `(target_type, target_id)` from the live roster. Missing, mismatched, malformed or contradictory duplicate entries must not establish success. `queued`, `coalesced` and `deferred` mean admission/coverage only, never started/completed/accepted work. Unknown reason codes should not become an authorization inference.
4. **Preserve the write result.** A valid saved-comment response with blocked/unknown outcomes still returns the current successful CLI result; only stderr warns. Keep existing transport/body-decode/output-write errors rather than forcing every post-commit error to exit zero. An HTTP response loss or invalid whole JSON remains uncertain and must not provoke an automatic second POST/PUT.
5. **One evaluation per turn remains sufficient.** For the delegation-only path, all expected targets handled may record `action`; any blocked/unconfirmed target should record `failed` with a reason that also identifies the handled subset. Neither outcome changes the historical storage semantics of `squad_leader_evaluated`, and partial success must not become a claim that every target was dispatched. Do not add a second comment to explain a saved delegation failure; the activity reason is the existing record.
6. **Recovery remains limited.** Reading the saved comment can establish that the write happened, but cannot reconstruct the original admission receipt. The protocol must not say otherwise. Preserve the no-blind-repost rule and hand uncertainty to an authorized follow-up instead of promising idempotent ordinary comment POST.
7. **Keep status authority intact.** Preserve owner/guest status branches, human/existing-integration `done`, no automatic stage advancement, one-comment-per-turn and no assignment-plus-mention double trigger. Update only the owned mention/receipt prose and matching source maps.

## Required implementation evidence

- Real fake-HTTP command calls for both add and update, checking exactly one mutation, unchanged response data on stdout, stderr warning behavior and returned error value.
- Normal queued/coalesced/deferred, blocked, mixed partial, unknown, malformed entry/container, absent/empty outcomes and extra response fields. Table mode still warns. A malformed whole response or transport error does not initiate a retry.
- Owner and guest injected briefing both require JSON and exact expected-target checks, distinguish missing/unknown/blocked from admission, and retain current status/single-comment/dual-trigger assertions.
- Preserve actual pre-fix failure and post-fix named-test output; a missing-symbol compilation failure is not behavioral red evidence.
- `gofmt`, applicable Go build/vet and scoped CLI/execenv/handler tests. Handler tests require the coordinator-owned database even when checking protocol strings.

## Verification limits

Source review only. Lint: not run. Typecheck/build: not run. Tests: not run. No database, model, production or remote operation was executed. Only this review file was written; no other agent’s files were changed.
