# Independent review — completion evidence hints

**PASS — zero remaining blocking findings** for the explicit human-only ordinary-issue completion hint API. This is an API foundation; it does not add automatic UI/CLI prompts, skip, any lifecycle interceptor, global enforcement, M2/M3, or real-model acceptance.

## Findings resolved

- The endpoint opens through `deliveryPolicyTx`, including exact policy/evaluation schema readiness, before any successful response. Missing and nonunique readiness indexes are refused even when policy is off.
- Evaluated hints retain the existing recovery's `applicability.checked_at` as their outer timestamp. They do not present a later unverified time or modify the original observation/expiry.
- Added meaningful current-authority coverage for all four non-evaluating branches: null key, policy off, non-done target and no project. Positive controls succeed, then expired credentials, removed membership and disabled users are refused without business-state changes.
- Independent source review identified the embedded-NUL target-status defect. `handler-nul-red01` actually returned503 where400 was required. The implementation now reuses `issuestatus.ValidateKey` before PostgreSQL text encoding while retaining `Resolve` for current catalog/archival validation. The same named regression passes in both frozen final handler runs.

The reviewer changed no product or test files and performed no database or provider actions. Implementation owners applied the coordinated changes; the reviewer independently checked their final source and evidence.

## Verified behavior

The handler reuses the current human principal, workspace/member/credential fences, lifecycle issue lock, current project policy and immutable evaluation recovery. Custom done targets use the existing status category and shared catalog fence. It does not seed status rows, select evidence on the caller's behalf, duplicate the observer, or create an execution engine.

Only the original human's passing observation with current applicability produces `matching`. Current authorization is checked before historical evidence is returned, including after resource-version drift. Stale, expired, hold and unknown results remain attention; missing/private keys, lost authority/bindings, lock contention and commit failures remain errors. No-key and inapplicable responses disclose no evaluation. Every success explicitly retains `enforced:false` and `usable_for_completion:false`.

The public route tests use actual JWT/password/PAT authentication and task-token packaging, inspect the real tar contents and SHA, and compare exact persisted evaluator/key/evaluation/producer/receipt identities. Rebuilding the router with a renewed human credential recovers the same immutable observation. Actual database-clock expiry changes the hint without renewing evidence. Business snapshots and packaging counts prove no additional execution, task, comment, policy, evaluation or status mutation. Ordinary/custom update, batch update and create-as-done remain possible without an evaluation. The independent collaboration route boundary still refuses this ordinary POST.

The read locks end with this transaction. No result is a ticket for a later completion write. The writer/decision documents explicitly retain ordinary, batch, create/comment-child, move, VCS, collaboration acceptance/self-completion/R38 and custom-category enforcement work.

## Independent evidence verification

Rehashed all13 frozen files after sqlc regeneration: zero drift. The manifest includes all9 changed product/test/spec/skill paths and4 unchanged reused implementation files; those4 also match the baseline commit. Generated SQL bindings, `go.mod` and `go.sum` have no diff. Independently executed `gofmt -l` over the8 frozen Go files and `git diff --check`; both are clean.

Rehashed and independently parsed all29 recorded raw logs, including all9 final checks and all4 failed attempts. Record inventories, counts and the verification summary match the raw logs. Every started named test has exactly one terminal result. Source-derived top-level inventories exactly match the executed tests and `test-inventory.json`.

| Frozen final suite | Ordinary | Race | Top-level tests |
|---|---:|---:|---:|
| Handler and prior policy/observer/artifact/approval/deletion contracts |325|325|76 each|
| Real authenticated router and prior policy/observer/artifact contracts |103|103|37 each|

Every final suite has0fail/0skip. Handler325 comprises89 new hint results and236 existing results; router103 comprises23 new hint results and80 existing results. Counts include subtests. Ordinary/race or iteration counts are never added as independent coverage.

Additional contracts50 comprise21 built-in skill results,16 other service tests selected by the broad `Resolve` prefix,11 status-catalog results,1 migration static contract and1 runtime scan. They are not27 additional status tests. The24 top-level source names exactly match this run. Whole-backend build/vet, sqlc generation and final formatting checks exited0. All final commands started after the frozen manifest timestamp. Exact commands, record/log hashes and independent source bindings are in `independent-review.json`.

## Retained failures and limitations

The4 nonzero attempts remain retained: overlong custom-status fixture keys; fixture `MAX(number)+1` allocation not advancing the product workspace counter before a real create request; the reproduced NUL input defect; and a transient syntax/brace error in the new handler during formatting. The first two are corrected fixture setup, the last is a parser failure, and only the NUL run is the demonstrated behavioral red. Its2 named failures are the parent and subtest, not two independent defects.

No new migration was introduced. Existing migrations were applied to dedicated fresh run databases; this is not a claimed rerun of every prior migration suite. This backend scope has no full-repository, browser, installed-client, performance, deployment or vendor acceptance claim. PostgreSQL17 uses an amd64 image on an arm64 host. Inherited PR SHARE NOWAIT locks and asynchronous PAT last-used writes may legitimately return conflicts.

Real-model calls0. AC43/E5/H3a/knowledge budgets and stopping rules remain unchanged. Coordinator cleanup, scoped Lore commits, protected local integration and archival remain operational handoff work. The parent M1 and six-business roadmap remain open.
