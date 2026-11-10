# Independent review — delivery evidence observation

**PASS — zero remaining blocking findings** for the fixed-policy, observe-only backend increment. This does not complete the M1 enforced delivery gate, durable delegation, M2/M3, or any real-model acceptance.

## Findings resolved by implementation owners

- Current evidence authority checks distinct initiator, originator, runtime owner, approver and human claimant after producer completion. Tests keep an independently authorized observer while revoking each other principal.
- Conflicting equally ranked PR heads return unknown. Short four-table SHARE NOWAIT locks cover current PR rows and newly inserted links; tests require the actual55P03 lock refusal.
- Receipt validation restores canonical map bytes with preserved numeric precision, binds every identity/output field, and accounts for the existing separate receipt and finalization timestamps.
- A mismatched nullable project pair now returns400 instead of dereferencing a nil revision; the public route includes a regression.
- Final credential validation occurs after captured observed_at, preventing a PAT expiry between the last authority check and the claimed snapshot time.
- Skill documentation stays within its500-line entrypoint contract; the source-map row is in its table.
- Commit failures before and after commit return503 with no usable observation. Canonical fixture comparisons check JSON decoding errors.
- The final router race exposed an existing asynchronous PAT last_used_at update conflicting with the evaluator's NOWAIT authority lock. The test now observes that same real authentication write commit at the request-body boundary before its one positive evaluation. It still requires200/pass and exact post-auth revocation refusal; no product lock or criterion was relaxed.

The reviewer made no product edits, database writes, real-model calls or duplicate database test runs.

## Verified behavior

The actual route performs create → human decision → task-credential packaging before observation. The test independently reads the tar, checks its contents and committed digest, and verifies repeated evaluations leave issue, approval, receipt, queue, comments and physical bytes unchanged. Completed producer tasks and a disabled packager remain observable.

Selected SHA, issue/project revisions, spec/output digests, current principals and the retained receipt are checked against locked facts. Missing/ambiguous source facts, unclaimed/unknown results, failed/expired/tampered evidence and revoked authority cannot produce pass. GitHub/VCS selection and both orders of relevant writer contention are covered. The legacy done payload still works after a hold/unknown observation because this endpoint grants no transition authority.

## Verification

- Independently rehashed all6 frozen source files: no drift. Independently executed gofmt and git diff --check: clean.
- Recomputed raw-log hashes and named terminal counts for all8 final check records.
- Fresh handler ordinary/race: each140PASS,0FAIL,0SKIP (56 observation names plus84 existing artifact/approval names).
- Fresh real-router ordinary/race: each34PASS,0FAIL,0SKIP (11 observation names plus23 existing artifact names).
- Whole-backend build/vet: exit0. Builtin skill contracts21PASS; runtime lookup scanner1PASS.
- Counts include subtests and represent separate runs; they are not unique cases to add together.
- No SQL/schema/generated changes: sqlc regeneration unnecessary. No frontend changes: TypeScript/frontend checks inapplicable.

The router's final synchronization changed only its test file after handler verification; the other5 source hashes stayed fixed. Builtin/scanner checks preceded only the small final credential clock ordering adjustment, which changed neither their source nor the instrumented runtime lookup path.

Original failed environment/fixture attempts, superseded pre-ordering runs and the PAT-contention router race remain retained. The JSON review binds the accepted records and preserved failures to their raw log hashes.

## Remaining limits

This is a stored-fact observation, not real-time remote HEAD verification or a reusable approval. It uses the conservative artifact admission expiry as an observation horizon, while retaining historical success. PR-table locking is a global contention point; ancillary PAT writes can legitimately return409. No throughput claim follows.

There is no project-configurable blocking policy, complete done-writer enforcement, persistent evaluation ledger, new UI/CLI flow, remote publication/deployment or new model evaluation. Real-model stop rules remain unchanged. Operational cleanup, scoped commit/integration and task archive remain the coordinator's responsibilities.
