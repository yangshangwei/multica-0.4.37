# R33 M1 policy-wait GREEN independent check

**ACCEPTED_R33_M1_POLICY_PROJECT_WAIT_PRIVATE_GREEN_NORMAL_RACE.** The scoped three-file private repair and exact40-name normal/race union pass with no blocking product finding. M1 PUT waits behind brief project readers/writers; the unchanged CAS rejects a stale writer after release, while long-reader timeout and current-role denial remain non-authorizing. ROOT integration/spec sync, matching builds, actual runtime four and full S2 remain separate gates.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; reviewer `/root/r32_after_check`; source `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-green-r01`. Only this new report pair was written/sealed0444. Saved source/hash/generated/static/raw test evidence review only; no reviewer source/config change, Go/build/test/vet/lint/type-check, DB/SQL/Redis/API/browser/process/Git execution or agent spawn.

## Findings (fixed)

None. No reviewer code correction was needed or authorized.

## Findings (not fixed)

- `.trellis/spec/server/delivery-policy-evaluations.md` still documents project NOWAIT at this review's reading. ROOT has the follow-up to distinguish M1-only Wait under the unchanged250ms per-lock budget while retaining original query/stage NOWAIT. Report-only ownership excludes spec edits; this does not block private source/offline GREEN acceptance.
- ROOT integration and matching product/runtime acceptance remain pending. The local regression PIDs do not identify the historical R32 physical holder or retroactively prove R31's cause. Full A01–A12/S2, parent24 and S3–S5 remain open.

## Verification

### Exact product and source boundary

Accepted genuine RED `b8419366…` precedes permit `37beb8cd…` and owner receipt `2586810b…`. Final5056-file source is independently hashed/mode-checked against ROOT static manifest, with exactly these three product differences and5053 protected files unchanged:

| Path | SHA256 |
| --- | --- |
| `server/internal/handler/delivery_policy.go` |`fff6c379877e869a56036b0ae25331136ee959fcd008415cedd6c7bf1a6e050a` |
| `server/pkg/db/queries/delivery_policy.sql` |`42d1efba5afb5f9b428dc2d4454f9a0a1f4c252375c00feb00ec69115ab5eec2` |
| `server/pkg/db/generated/delivery_policy.sql.go` |`894502c6ec2ceb70c6e30e676ed1203ada06ba658dd95f5bea0f9fb7fb5c697e` |

All modes/path identities are unchanged; no new/removed/nonregular file, test/config/schema/DTO/dependency difference exists. Final path-sorted manifest `ccd6b82b3be808b83ebac35c00b9452715c887cb3041f09628ae22059c8752c7`, protected5053 `5b0d09e225be6398e2c2f9ac4129d960551b1c1482e1b3a0ae056dc5507b695f`. Same regression test remains `443fc8a9…`/491lines/0644 across accepted RED and both GREEN modes.

Replacing only the single handler Wait call/params reconstructs every old handler byte. GET retains `LockProjectForAssociationNowait`; PUT's current member/credential/schema/CAS/append/final credential/transaction/HTTP error handling is byte-identical around the changed call. Removing only the new three-line SQL block reconstructs old SQL. New query is SELECT* with the original ID/workspace predicates and exclusive FOR UPDATE, omitting NOWAIT only in the new variant.

Original `LockDeliveryPolicyProject` remains byte-identical/FOR UPDATE NOWAIT. Stage-policy caller retains its original method and source SHA256 `a892c72b67010b78a7d1383406ba11412efc2a9ecc3acd1e3df36295265cf9d6`; artifact250ms preparation remains unchanged. The only production Wait caller is M1 policy PUT. Waiting behind a short writer is deliberate; no retry/flag/shared-mode switch/no-op/CAS bypass or deadline extension is introduced. The250ms setting bounds each lock acquisition, not entire transaction latency.

### Canonical generated method

Installation `9aacbbd2…` binds isolated command `13fdca5d…` and the retained pinned sqlc1.31.1 tool/origin record. Generation exit0, empty output and DATABASE_URL unused. Of143 generated outputs, only delivery_policy.sql.go changes; other142 outputs match RED/current, and four handwritten files in the generated directory remain protected. Only the intended output was installed.

New generated code is a35-line append; removing it reconstructs the complete old file. New params ID/WorkspaceID are the same UUID types, QueryRow receives the same arguments/context, and the Project scanner/return/error shape matches old code. Generated Wait SQL SHA256 `9583bca55ad6b7c14132c4fc59e0ea1c4a35160eb8236354ab697b13568e2ecd`; changing only query name/NOWAIT reconstructs old SQL hash `f56557e1…`. No authority/helper/generated drift is admitted.

### Actual normal and race union

| Mode | Actual inventory | Saved result SHA256 |
| --- | --- | --- |
| Normal |10top/33leaf/40named;40PASS/0FAIL/0SKIP |`8b364e6f5500981f5a414ba825ae7025167b983fec6b74ac1ab01b2d20870536` |
| Race |10top/33leaf/40named;40PASS/0FAIL/0SKIP |`ea489183e28276f36c4f97e3c36a3c8bae8bbdee500f2dddff25f8c7717e8ee1` |

Each raw stream independently parses209valid events,40unique starts/40unique PASS terminals and packagePASS/exit0. Exact names equal sealed inventory `92e3faf1…`: eight original policy/CAS/writer/schema/activation tops plus unchanged six regression names. No skipped/missing/extra/duplicate/parse/pending/drift/pause/parallel event; verified stderrempty and suite_reranfalse. Actual normal/race permit/command/profile/source references match, one command each; race includes-race, ambient inheritancefalse. Fresh owning-handler normal/race databases are distinct and unused before their bound runs.

Both modes establish actual pre-release pg_blocking_pids waits on the first holder, matching current JWT/project/workspace and distinct PostgreSQL PIDs. The new generated Wait query executes natively; no synthetic row/error/PID witness is substituted.

- Short-reader releases at50.737667/50.08025ms (normal/race), Wait Scan succeeds at58.5885/54.407042ms, GET200/PUT200 and one exact revision1/requested/current-human append. Original protected non-policy facts remain equal.
- Long-reader retains the holder371.869167/372.909708ms; actual Wait Scan fails native55P03 at253.384375/253.037875ms and returns409artifact_authority_busy with zero append. This measures query/transport/Scan elapsed time, not pure server wait or whole-request duration.
- Short competing writer releases50.1645/50.352833ms; second Wait Scan succeeds52.661125/52.700083ms, then unchanged CAS returns exact409delivery_policy_revision_conflict. Winner200/one retained row, loser0append. No generic busy409 is counted as stale CAS.
- Current member's allowed GET and exact403management-denied/no writer query/no append control pass. Existing ConcurrentCAS and project/member writer-first controls, strict request/role/index/schema/activation tests all pass. No new dynamic credential-expiry matrix is claimed; original pre/final credential bytes and source-bound prior evidence remain.

Five raw effect records per mode cover four new leaves, with duplicate long-reader snapshot recording. All protected equality checks pass; per-case policy rows1(short)/0(long)/1(winner)/0(member). Allowed policy append changes full hashes separately; no blanket task-table exclusion or whole-database-zero-change claim.

Before/after audits match each exact fresh database,0otherconnections, unchanged ledger `d648c6e2…` and indexes. Each suite retains counts invocation_target0→1, policy0→27, evaluations0→5, config_invalidation21→70 from its real existing controls/fixture history. These are retained history, not cleanup normalization.

### Static and remaining gates

ROOT static `d963ce7c…` gofmt/vet/handler compile-only exits0 with six empty matched logs and unchanged5056 final source. No TestMain/DB used for compilation. Private test/source and canonical generation pass; this review ran no commands. ROOT owns spec synchronization, four-path integration (three products plus new test), matching programs and the next original-four run.

- **Lint:** PASS, retained actual scoped gofmt/vet.
- **TypeCheck:** PASS, retained exact-source handler compile-only.
- **Tests:** PASS,40named normal/race, no missing/skipped/pending/drift.
- **Source/generation:** PASS, three product files/5053protected and canonical isolated143outputs.
- **Runtime/fullS2:** NOT_ACCEPTED_HERE.

Genuine r02RED2PASS/4FAIL and initial r01coordination1PASS/5FAIL remain preserved; no failure was overwritten or retrospectively accepted. R32 original runtime remains3PASS/1FAIL/0NOT_RUN,retry0 with successful completed keys unreplayed and unknown physical holder. Full A01–A12/parent24/S3–S5 stay open, parent0PASS/9PARTIAL/15NOT_RUN. Captured UTC `2026-11-18T00:54:56.686138Z`. No pending work remains within this private GREEN source/offline review.
