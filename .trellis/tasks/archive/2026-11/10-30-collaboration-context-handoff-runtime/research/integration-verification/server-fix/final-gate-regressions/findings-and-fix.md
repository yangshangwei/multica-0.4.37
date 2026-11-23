# Final-gate service regressions

All work is in the isolated integration checkout. The original full-gate logs are unchanged. This follow-up uses `/tmp/multica_runtime_full_20261101_2392cdc0.env`; the older targeted database was not used. Credentials were passed only through environment variables. No provider, daemon implementation, protocol validator, task-state or main-workspace edit was made.

## Queue root cause and product fix

`CollaborationRunHasPreparationStartEvidence` previously treated every row in `collaboration_run_event` as possible issued execution evidence. New `wakeup_assigned` records are server-generated scheduling provenance on a queued continuation before its first claim. They can record an unread joined input or a member proof without the continuation having a prompt, capability, attempt, termination identity or process.

The false evidence prevented `settleUnclaimedPreparationRefusal` from settling an actually unclaimed continuation after a late file delivery. Its pending-derived fence then starved another healthy root. The same predicate inspects sibling continuations before settling a reserved but unclaimed root, causing the second failure.

The SQL now excludes exactly `wakeup_assigned`. All other event kinds, including unknown kinds and possible business/record/launch evidence, still block this special nonlaunch path. The termination/attempt/business capability checks, run state, frozen prompt checks, delegation/readers, actual runtime capability, reservation and current authority remain unchanged. `make sqlc` regenerated only the corresponding preparation query for this follow-up.

Existing queue assertions remain in place. Added assertions require one refusal per member/continuation, one continuation, and retention of the exact original proof/join source metadata after nonlaunch settlement. A new matrix verifies `unknown_private`, `outcome`, `comment`, `record_checkpoint` and `handoff_binding` still prevent the same settlement and leave control state unchanged. Existing live-sibling, issued-claim, rollback, evidence-read-failure and capable-runtime tests remain part of the affected rerun.

## Records expectation correction

The approved records query selects only displayable event kinds, so internal or unknown non-display metadata is intentionally excluded before public projection. The old test expected `unknown_private` to break the whole page. Reproduction confirmed this assertion failure, not disclosure. No records production code or allowlist was changed here.

The updated test traverses the actual paginated read with the unknown event present and requires identical retained display items, no secret/kind leakage, no cursor stall, `not_recorded` coverage, nil retention and nil telemetry. The existing long record pagination test now places all new internal handoff/wakeup kinds before, between and after display events, retaining its fixed-watermark, exact IDs, overlap, usage, partial-coverage and post-proof retention checks. A separate negative matrix requires malformed outcome/checkpoint/comment/activity payloads to return a generic error without exposing their sentinel.

Display projection and nonlaunch evidence intentionally have different defaults: unknown metadata is withheld from display, while unknown evidence continues to prevent automatic nonlaunch settlement. Do not replace the latter with the display allowlist.

## Evidence

`reproduce.jsonl` retains all three original failures on the selected database before the fix. `focused-green.jsonl` contains 23 passing cases with no skips. `service-affected.jsonl`, `handler-affected.jsonl`, `vet.log`, `sqlc.log` and `checks.json` record the final affected checks; the coordinator owns the next full repository gate. Final affected results: 236 service cases and 6 handler cases passed, with zero failures/skips. Affected-package go vet, pinned sqlc generation, gofmt and diff whitespace checks passed. No captured source hash drifted during the affected service run. Source hashes and focused diffs are captured in the adjacent manifests. The full-gate database is released after this sequential service/handler run.
