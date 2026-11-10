# Independent review — delivery policy and durable evaluations

**PASS — zero remaining blocking findings** for owner/admin-managed off/observe project-policy versions and human-scoped immutable delivery evaluations. This does not implement hints, skip, global completion enforcement, M2/M3, or any real-model acceptance.

## Findings resolved

- Planning now separates existing-key recovery from new evaluation admission. Policy off/change and issue moves can invalidate applicability without bypassing current authorization or discarding recovery. Rechecking current locked resource versions avoids the existing evaluator's early stale-version return before artifact authority.
- The new schema is checked independently of artifact schema. Exact usable unique indexes, retained deletion, guarded rollback and current project/member fences are implemented and exercised.
- The generated insert changed to `ON CONFLICT DO NOTHING RETURNING *`. With coordinator ownership transfer, this reviewer corrected two stale migration assertions to require `pgx.ErrNoRows` and exact unchanged retained-row/catalog snapshots. Distinct namespace positives and final row counts remain. Final fresh migrations normal/race passed with the corrected assertions. The earlier34-pass migration result predates that query change and is superseded.
- The baseline workspace-deletion manifest omitted two already-retained tables. The coordinator's actual baseline failure is preserved. Its new retained-audit category adds those two and the two new histories without changing teardown product behavior; actual deletion tests independently prove retention.

The policy-management question was resolved to current workspace owner/admin. Project lead identity alone grants no policy-write authority. The reviewer made no product edits, database writes, provider calls or commits; the only test edit is identified above.

## Verified behavior

The public route performs real human approval and task-credential packaging before creating an evaluation. Tests independently read the archive, verify content and digest, compare the returned evaluation with persisted identity/observation, recreate the router and recover with a new valid human credential. Creating/recovering evaluations leaves the original artifact, queue and lifecycle unchanged. The stateless observer remains ledger-free, and the existing done payload still works.

Policy versions append under project locking and expected-revision CAS. Default off creates no history. Replays retain original actor/key/issue/request and observation, traverse current producer/initiator/originator/runtime-owner/approver/claimant authority even after version drift, and derive applicability separately. Original hold/unknown never upgrades through replay; missing formerly retained execution denies historical content. Original expiry is never extended. Current/pass remains an observation with completion permission explicitly false.

Normal and race coverage includes policy/resource/PR changes, policy ABA/off, malformed requests, actor/workspace/issue isolation, distinct historical-principal revocation, post-authentication credential changes, actual concurrent requests, uncertain commit recovery, partial/incorrect runtime schema, real deletion retention, actual migration/index recovery and concurrent rollback fencing. Database-observed PR head remains distinct from remote live HEAD.

## Independent verification

Rehashed all26 frozen source files after sqlc regeneration: zero drift. The manifest includes every changed/untracked product/test/contract path. Independently ran gofmt over the frozen Go paths and git diff --check: clean.

Rehashed and independently parsed all10 final raw logs and all6 preserved failed logs, and compared their summaries with verification-2026-11-10.json. Every started final named test has exactly one terminal result. Source-derived top-level inventories match the executed logs exactly:64handler,28router,9migration.

| Final check | Ordinary | Race |
| --- | ---: | ---: |
| Handler, existing observer/artifact and deletion manifest |236|236|
| Real authenticated router |80|80|
| Actual migrations and concurrent-index registration |37|37|

All are0fail/0skip. Counts include subtests and separate runs are never added as distinct coverage. Handler236 is95new+140existing+1manifest; router80 is46new+34existing; migrations37 is34new+3registration. Additional contracts23 are21builtin-skill names+1migration contract+1runtime scan. Whole-backend build/vet and sqlc generation exited0; generated source stayed identical to the freeze. Exact commands, record/log hashes and accepted inventories are retained in independent-review.json.

## Preserved failures and limits

The six nonzero runs are: product `pgtype.Bool` compilation mismatch (also seen during router compile), handler-fixture compile errors, unsupported guest-role fixture, baseline audit-manifest drift, and a distinct-approver fixture lacking invocation rights. They remain failures, not successful behavioral-red proof. Superseded successful runs also remain retained and do not replace final evidence.

This is a backend observe increment with migrations776–780. There is no UI/CLI consumer, full-repository/browser/deployed-client or performance claim. PostgreSQL17 ran as an amd64 container on an arm64 host. Current reads can legitimately return409 from short global PR locks or PAT ancillary writes. Multi-file rollback requires admission stopped throughout and refuses retained evidence; off retains history and grants no dangerous action.

Real-model calls0. AC43/E5/H3a/knowledge budgets and stopping rules are unchanged. Coordinator environment cleanup, scoped Lore commits, protected local integration and archival remain operational handoff work; parent M1 and the six-business roadmap remain open.
