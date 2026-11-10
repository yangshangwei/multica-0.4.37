# Controlled artifact execution implementation plan

Status: implementation and independent verification passed, 2026-11-10. Final execution mapping and commands are in delivery.md and verification-2026-11-10.json; checkbox items below retain the original execution blueprint rather than claiming unrun full-package commands. Child is in_progress in codex/controlled-artifact-20261110; parent remains in_progress. design.md is the current contract; original reviews remain intact.

## 1. Review before activation

- [x] Independently review all six original blockers against design.md and the ten-case matrix below. Check canonical bytes, private detail versus broad queue, task/principal scope, and unclaimed versus result states.
- [x] Independently check the explicit source-backed graph in design.md against the actual SQL: workspace KEY SHARE NOWAIT -> verified task token when present -> joined task/agent/runtime/issue/originator/runtime-owner locks -> supplemental human user/member/credential locks -> managed-binding fences when present -> approval then execution. Check the separate cancellation graph and nonce-only finalization graph. No universal repository lock-order assumption or blocking substitute for NOWAIT is permitted.
- [x] Record baseline HEAD/dirty paths; isolate overlapping work without reverting another session. Load server security, task capability, database-isolation and migration guidelines.
- [x] Main activates the existing child only after planning review. No duplicate task, provider calls or shared business DB.

## 2. Canonical spec and deterministic adapter

Files: narrow new service spec/packager files and OS helpers/tests. Reuse archive/tar, crypto/JSON and existing x/sys/root patterns. Do not import daemon business code into service or add a generic execution framework.

- [ ] Write failing strict-decoding/canonical-vector tests, including duplicate JSON keys, equivalent manifest ordering, digest domain separation and creation replay versus generated timestamps.
- [ ] Implement exact limits: 64 files, 8 MiB each/32 MiB total, 96-byte portable paths, 32 KiB body/24 KiB spec, 15-minute DB admission, 120-second adapter deadline, 4 KiB receipt metadata.
- [ ] Add failing filesystem tests before adapter code: component/leaf symlinks, hardlinks, special files/device changes, byte mutation, parent/root replacement, path/case collisions, overflow and output collision/symlink. All roots/files are test-owned synthetic material.
- [ ] Implement operator manifest provenance, missing-config disablement, Linux/macOS no-follow traversal and unsupported-platform refusal. Capture bounded verified bytes once; archive the exact snapshot.
- [ ] Implement deterministic USTAR, exclusive execution directory/partial file, no-replace publication, file/parent sync and final reopen/hash.
- [ ] Add fault seams only for actual I/O/clock/commit boundaries. Known clean prepublication refusals fail; uncertain partial/publication/cleanup stays unknown. Collision targets and uncertain evidence are retained.

## 3. Schema and lifecycle

Files: new numbered server/migrations files, server/pkg/db/queries/agent_artifact_execution.sql, generated sqlc, focused execution service, bounded changes to approval/authority queries, workspace_delete.sql and generated workspace_delete.sql.go, and cmd/migrate/main.go cleanup registration.

- [ ] Create table without FK/inline PK/unique; one concurrent unique index per migration; final validation/rollback fence last.
- [ ] Execute actual SQL in private schemas for empty/populated/concurrent-first-insert/claim rollback. Test runner ordering, all index registrations, and partial-schema refusal.
- [ ] Atomically create trusted canonical spec, pending approval, safe generated plan and one unclaimed row. Verify same-key replay/race and changed-key-content conflict.
- [ ] Add bound human decision requiring expected_spec_digest and current authority/expiry; no spec replacement.
- [ ] Claim updates existing row to unknown with nonce under shared locks; invoke adapter only after definite commit. Ambiguous commit invokes zero times.
- [ ] Finalize success receipt and legacy executed in one transaction, CAS by execution/spec/nonce. Failed/unknown never mark executed. No restart/expiry/background auto-retry.
- [ ] Implement retained receipt lookup and bounded machine facts. Existing output never substitutes for committed success.
- [ ] Implement the exact NOWAIT authority graph from design.md, including independent current human/initiating human/retained approver user and membership rows. Do not reuse KEY SHARE membership locks for role changes or the waiting PAT lock unchanged. Recheck original verified credential/version under these locks; missing/busy/inconsistent hints roll back with no automatic retry.
- [ ] In DeleteWorkspaceLeafData, retain every approval with a matching execution approval_id; keep unbound cleanup unchanged. Preserve the handler's existing workspace FOR UPDATE fence and exclude execution receipts/output from cleanup. Under a new workspace KEY SHARE NOWAIT fence, creation cannot commit unnoticed after the sweep.
- [ ] Allow internal finalization after workspace/task/agent deletion using only retained approval -> receipt locks and nonce/spec CAS. Verified success still changes both rows atomically. No workspace event/activity insert, external authority lookup, rerun, background recovery or public read bypass is part of that path.

## 4. API and compatibility

Files: additive artifact handler, bounded controlled branches in agent_approval.go, router.go, operator-root configuration, bounded workspace.go cleanup/fence integration as needed, workspace-deletion/approval concurrency tests, and relevant API/built-in skill source maps if documented.

- [ ] Wire create/restricted-detail/execution routes; preserve TaskCapabilityGuard and reporter/reviewer denial.
- [ ] Use verified task-token context and transaction-bound current authority. Test human header spoofing/cloud PAT/another task generation; preserve Operator execution ceiling.
- [ ] Bound decision/cancel/legacy execution use same approval/execution locks. Postclaim cancel conflicts; old self-report refuses bound rows; legacy generic behavior remains.
- [ ] Full spec only in authorized no-store detail. Keep manifests/paths/root identities out of legacy list/get plan, activity and broad notifications. Opaque artifact ID/digest only.
- [ ] Use real router/auth plus internal/testutil fixtures. Test legacy and password authentication; do not fake verified authority using headers alone.

## Required ten-case acceptance matrix

Canonical/parser/filesystem matrices belong beside helpers. Transaction/router suites own authority, wiring and concurrency; avoid repeating pure matrices through every layer.

| # | Observable proof |
| --- | --- |
| 1 | Human creation yields exactly one pending approval/unclaimed row; canonical bytes/digests/expiry match full detail and safe summary. Equivalent reordered request/key reuses IDs; changed payload conflicts. Invalid/unknown/duplicate/trailing JSON, IDs/digests/policy and overlimits write zero rows. |
| 2 | Real agent token cannot substitute agent/task/generation/workspace/issue/staging/revision/manifest/originator. Header impersonation/cloud PAT/unrelated agent refuse. Authorized human controls succeed. Membership/account/invocation revocation and restricted capability deny. Ordinary workspace member sees safe queue summary but no full spec. |
| 3 | Concurrent decisions yield one winner. Cancel-first means zero adapter invocation; claim-first means cancellation conflicts. Decision cannot edit spec. Use C1–C4 below to prove actual membership/password/token/deletion admission ordering. |
| 4 | Concurrent claims update ONE precreated row and cause ONE invocation of a synchronized real synthetic packager. Same-digest replay while running/after success/failure returns same ID/result, never a second output. |
| 5 | Mutating initiator/agent/task/runtime/generation/originator/resource/action/revision/staging/input/environment/policy/root identity/invocation digest refuses before invocation. Caller argv, decision-field replacement and corrupted canonical bytes/digests fail closed. |
| 6 | Database expiry including equality, rejected/cancelled/revoked, consumed/failed/unknown and unrecognized status never run again. Approval/replay cannot extend expiry. Already claimed expired receipts remain readable under current read authority without new execution. |
| 7 | Traversal/absolute/volume/duplicate/case/file-parent paths, symlinks, hardlinks/special files/device boundary, root/parent replacement, changed bytes, limits and output collision/symlink produce no published artifact. Existing collision content stays unchanged; uncertain cleanup remains unknown. |
| 8 | Two separately approved equal-input executions with differing request order produce byte-identical USTAR, independently verified hash and exact fixed headers/content. No extra metadata/files; final sync/reopen verification precedes receipt success. |
| 9 | Known clean prepublication refusal becomes failed. Crash/panic/deadline/ambiguous claim or receipt commit/sync/publication uncertainty remains unknown/no rerun. Inject before/after actual commit: ambiguous claim invokes zero times; success changes both rows or neither, including after workspace deletion under D2. Existing artifact with unknown DB state is not success. Fresh authorized read can recover only the original committed receipt. |
| 10 | Unbound request/list/get/decision/execution/cancel and legacy teardown cleanup remain. Bound self-report/postclaim cancel refuse. Bound approval/receipt facts survive teardown under D1–D4, but missing workspace denies public access. Queues/logs contain no manifests/paths; failed/unknown never appear executed. Human-only route and direct-handler backstop hold. |

### Two-connection authority and teardown acceptance

Use real transactions and barriers that pause after the actual lock/commit boundary, plus an instrumented real synthetic packager for invocation counts. Exercise both commit orders. A synthetic HTTP status or mocked query alone is not proof. All cases use a fresh owned database; workspace cases have no retained collaboration records that would independently reject deletion.

| Case | Ordering and expected evidence |
| --- | --- |
| C1 Membership/role versus claim | Connection A holds an originator/runtime-owner/current human/initiating human/retained approver member-row update or deletion; B's claim refuses busy, rolls back and invokes zero times. After A commits a revocation, a fresh request is denied. In the reverse order, B holds required member SHARE locks through definite claim commit; A cannot commit its conflicting change first. The admitted operation invokes exactly once; later revocation blocks public access/admission, not internal receipt persistence. Vary each independent principal, including a human distinct from both task originator and runtime owner. |
| C2 Password/PAT/task-token revocation versus claim | Hold actual user FOR UPDATE/password change, PAT revoke UPDATE, or task-token DELETE in A. B takes NOWAIT refusal and invokes zero times; subsequent request rejects original version/revoked hash. Reverse order proves the authority rows stay locked through claim commit and exactly one admitted invocation. Assert token/credential identity is not upgraded to the new version and no server retry occurs. Include existing managed-binding revoke/epoch drift when that credential type is present. |
| C3 Workspace deletion versus creation/claim | Deletion holds workspace FOR UPDATE before its leaf sweep; controlled create/claim cannot acquire KEY SHARE NOWAIT and commits no row/claim or invocation. After deletion commits, it returns missing scope. In reverse order, create/claim holds KEY SHARE until commit; deletion sees the newly committed controlled binding, retains the approval/receipt, and cannot remove workspace before that admission point. Assert row identities, retained decisions and zero/one physical invocations, not only responses. |
| C4 Cancellation and cross-path inversion | Pause the real chat->task cancellation path after its task-row lock (a chat lock alone does not fence this adapter), or hold the agent permission writer's agent-row lock before claim; NOWAIT rolls the controlled transaction back without waiting/retry or invocation. For approval cancel versus claim, cancel-first commits cancelled and zero invocation; claim-first preserves unknown/one invocation and cancel refuses. Inject a finalization lock conflict: leave unknown/evidence retained, no automatic finalization or execution retry. |
| D1 Delete before claim | Create pending and approved unclaimed bound requests plus an unbound legacy request. Delete workspace. All bound approval/spec/decision rows survive, unbound approval is deleted; new claim and all public reads/decisions/cancels refuse missing workspace. Adapter invocation count remains zero. |
| D2 Delete after definite claim | Pause the sole adapter owner after durable unknown claim; commit workspace teardown, removing live task/agent/member rows. Resume that original owner with a verified artifact: internal nonce-only finalization succeeds, atomically setting retained receipt succeeded and retained approval executed. Inject failure between the two updates to prove rollback leaves both prior states; zero second invocation and no recreated workspace, task, activity row or notification. |
| D3 Delete terminal/unknown evidence | For succeeded, failed and unknown results (also rejected/cancelled unclaimed requests), compare retained approval decision fields/spec/claim/receipt digests before and after deletion. They remain intact; unknown stays unknown with no recovery. Output files are untouched. Run with former owner credentials to confirm no public receipt access after workspace disappearance. |
| D4 Existing teardown compatibility | Run canonical workspace cleanup fixtures proving unbound approvals and other existing leaf data still delete. Exercise create-versus-delete with the actual SQL CTE; ensure matching approval_id retention does not depend on a second workspace-field match that could purge damaged evidence. Preserve the existing retained-collaboration deletion gate. |

## 5. Verify and retain evidence

- [ ] Preserve meaningful red regressions and source hashes, then actual green output/named test counts. A skipped test does not close a matrix case.
- [ ] Pure tests need no DB. DB service/handler/router tests use fresh explicitly named owned DBs per package under database-test-isolation.md, repository CLI guard and required serialized heavy windows. No model CLI/network execution.
- [ ] After real names exist: from server run go test ./internal/service -run '^TestArtifact(Package|Spec)' -count=1. DB-enabled packages individually run go test ./internal/service, ./internal/handler and ./cmd/server with -run 'Test(Artifact|ControlledArtifact|AgentApproval)' -count=1. Run -race on actual decision/claim/cancel/receipt concurrency cases. Record actual commands, not these proposed names as completed.
- [ ] Run new actual-SQL migration tests in internal/migrations and runner tests in cmd/migrate, including rollback order and index cleanup mapping. Never rehearse rollback in application schema.
- [ ] Run the real workspace-delete and account/member/task-token revocation integration tests plus C1–C4/D1–D4 in ordinary and race modes where applicable. Retain named test output, commit-order barriers, DB before/after snapshots and physical adapter invocation counts. Verify missing workspace stops public reads while nonce finalization remains independent of deleted live rows.
- [ ] Run gofmt, make sqlc and regeneration idempotence review, go build ./... and go vet ./... from server, plus required migration/static checks. If no TS/client code changes, record why TS lint/typecheck/test are inapplicable; expand checks if scope changes.
- [ ] Independent trellis-check reviews full diff and all ten evidence rows: authority/locks, exact bytes, physical invocation count, uncertainty, legacy behavior, leak prevention and rollback safety. Close blockers before reporting implementation complete.

## 6. Integrate and hand off

- [ ] Main updates executable server spec/API docs and records a NEW independent review, preserving original plan-review.md.
- [ ] Main commits only reviewed scope with Lore trailers and tested/not-tested limits. Archive child only after actual acceptance. Remaining M1/M2/M3 and real-provider acceptance stay open.
- [ ] Operational rollback disables admission but retains evidence and bound-route guards. Older application code or dropping indexes/table is unsupported with retained controlled rows. No external release, automatic cleanup or reconciliation is included.
