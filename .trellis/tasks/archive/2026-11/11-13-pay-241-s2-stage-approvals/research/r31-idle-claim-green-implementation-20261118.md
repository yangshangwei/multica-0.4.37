# R31 idle ordinary batch-claim GREEN implementation

**TWO_AUTHORED_PRODUCTS_FROZEN; MAIN_GENERATION_AND_VERIFICATION_PENDING.** After independently accepted genuine idle RED, the batch path now avoids reclaim's account writer fences only when a conservative database existence check reliably finds no potential dispatched recovery. Actual positive reclaim still uses its original transaction, authority and SQL. No GREEN test result is claimed.

Only two private authored product paths were changed in `/Volumes/artisan/.multica-acceptance/pay241-r31-idle-claim-20261118-bz7ym_r1/source-green-r01`. Generated code remains main-owned and unchanged at this handoff, so compilation requires main's canonicalsqlc generation. No test/RED/ROOT/spec/cache/schema/dependency or other product edit was made. This author ran only filesystem/text checks and task.go gofmt, with no Go/AST/sqlc/compile/vet/test/list/TestMain, DB/SQL/API/runtime/browser/signal/Git or recursive agent execution. Reports are new and sealed0444.

| Authored private file | Lines | Mode | GREEN SHA256 |
| --- | ---: | --- | --- |
| `server/internal/service/task.go` |7841|0755|`bcfb7399e75040f8eeb809cada849e11095ba1a105b9553d322d450397fdd7bc`|
| `server/pkg/db/queries/agent.sql` |2821|0755|`4a661a0c1374525177d4cd103b3d47aa39ea67c1a0a32c3eb4bf4067aa6494be`|

## Genuine RED basis

Original normal result SHA256 `bc6e509b2211d7117229a87cb94de8a1c13a44a97b009d80a566f728232496b5` is retained: **2top/4leaf/6named,3PASS/3FAIL/0SKIP**, two idle leavesFAIL and both real reclaim leavesPASS. Idle-first R4 gets actual current-userSHARE55P03 against the no-work claim's UPDATE; reader-first trueidle claim blocks and is explicitly cancelled, with0work/effects. Blocking cancellation is not invented55P03. Independent RED acceptance `49f7686df12cb6463b7181eafd26fd52b4302ff80ed2483155f994f7fbdc6ae0` admits this controlled regression, not the historicalR30holder or a GREEN repair. Original source/test/result/history remain unchanged.

## New conservative probe and preserved positive path

`HasPotentialDispatchedReclaimForRuntimes :one` returns only `SELECT EXISTS`. Its necessary conditions are the **complete `uniqueIDs`** set, statusdispatched, started_atNULL, dispatched_at strictly before **database now minus original90s recovery window**, and prepareleaseNULL or strictly before database now. The query reads no payload, authorizes no task and adds no current-authority/admission joins, row/table locks or `lock_issue_execution` call.

These conditions are a conservative superset of canonical reclaim eligibility: every actually reclaimable row must satisfy them, but rows can still be denied by binding/admission/user/runtime/issue/agent/task checks. A true result therefore grants nothing. The original `LockManagedRuntimes(false)`/managed source transaction, complete original UPDATE and `ValidateManagedTaskBinding` run exactly as before. Text comparison shows their whole original positive block is byte-identical after removing only one nesting indent; no parallel validator or read-lock flag was introduced.

The probe runs only for nonempty dueKeys, after original complete-runtime validation, deferred cancellation/promotion and normal due scheduling. Reliablefalse skips **only** Step2's reclaim transaction. It continues Step3EmptyClaim, queued candidate selection and real fresh claims; it does not return the entire method early or conceal queued/newly promoted work. Error returns a bounded wrapped error (`check potential dispatched reclaim`) and cannot markempty, claimwork or reach MarkChecked as success.

After a reliablefalse or successful canonical check, MarkChecked retains the exact original **all runtimeKeys/checkStarted/retry interval** arguments. Reclaimed tracking and all remaining batch behavior are byte-identical. A row or time boundary becoming eligible after a reliablefalse can wait only for a later normal poll/backstop; no persistent grant/no-recovery state is created. Existing cached-empty timing already permits a bounded observation gap. Newly queued work still follows the original later path. Positive canonical checks revalidate current facts; they never rely on this boolean for permission.

**Scope is batch-only.** Singular ClaimTaskForRuntime and every other TaskService function/file byte remain unchanged. Its analogous nil-cache applicability is an explicit residual scope, not silently declared repaired. Pool/isolation/auth/currentruntime/binding/finalization/partial-return and defaultwriter behavior were not modified. The existing comment/contract under MarkChecked was updated only to explain reliable negatives alongside successful canonical checks.

## Static proof and file preservation

- Exact task.go comparison excludes only ClaimTasksForRuntimes and leaves all other bytes identical, including the singular path. Before-probe validation/promotion and MarkChecked-to-method-end are identical. Original positive block hash is `6deced67582891cab8ae2bc58f685d5c432e37660914596bc5c539453dae75cb` after one indentation normalization.
- Removing the one inserted query from agent.sql reconstructs its complete original bytes. Original singular/batch reclaim SQL, parameters, admission/authority/issue locks and all other queries remain unchanged. New query block SHA256 `12be532005037f07bdbed2fa97cb5c496cbe6e81831ce7284f8f7817936f0078`.
- Actual5054paths/modes match RED; **exactly2authored product deltas/5052protected files** before generation, REDdrift0. Existing generatedagent.sql.go remains `234283e657fcdb39c14bea05decb8005cf20e43b5bb63b29ba7f99a3347a42a5`; original471-line test staysb07397…/0644. Full final authored-source manifest fingerprint `d48f077634d54bcf263bde3cc146ed7f941b5a7a6292be1cf17563a510cc830b` is retained in JSON.
- task.go gofmt-w and final gofmt-l exited0/empty. No compilation or tests ran here; main must generate the missing getter before type/behavior checks. Full diffs and the default/positive/singular textual proofs are in JSON.

## Required main gates and remaining safety evidence

Main owns pinned canonicalsqlc1.31.1/offline generation and generatedagent.sql.go. Final generated snapshot must show **only3product differences with5051protected files**, unchanged b073test and all originalSQL/getters outside the new boolean. Preserve the generated tool/output/configuration receipts; no generatedfile was written by this author.

Then run new6 plus original27service names as the planned **18top/33named/28leaf** normal/race union on fresh owning profiles; actual raw terminals, not proposal counts, determine acceptance. The existing `TestManagedBatchClaimLostResponseRecovery` handler (1top/1named/1leaf normal/race) already covers real young-dispatch refusal, expiredlease still inside recoverywindow, stale dispatch with activelease, eligible recovery, lease prevents duplicate, and started-task refusal with generation/binding/credential retention. It must execute with newgeneratedcode; it was only read here.

Two additional safety cases are still needed if not separately owned: **probe error is not empty/success** and **a row becoming eligible after reliablefalse is recovered on a later ordinary poll without permanent mark loss**. Recommended one new owning service test file after explicit ROOT dispatch: `task_idle_claim_preflight_test.go`, two tops/2named/2leaves `TestIdleBatchClaimProbeFailureIsNotEmpty` and `TestIdleBatchClaimNegativeProbeDefersNewlyDueWork`. Use real existing fixtures/DBTX forwarding: actual cancelled probe execution/error for the first, and a callback after the real false Scan followed by a deliberate fixture age/lease change for the second. Keep raw phase/currentcredential/noeffect checks and do not fake positive authorization or loosen any writer. These are recommendations, not authored or executed tests.

MarkChecked code/arguments remain exact, but reliablefalse now also reaches it. ROOT should decide the already identified complete-runtime/due-hint Redis-backed controls on its explicitly owned Redis/serviceprofile; no ambientRedis, dependencies or cache mutation was introduced here. Probe/temporal/cache/finalclock gaps must not be inferred passed from the idle tests alone.

Permit `60d53de6e3f4d14c75511bb847168547d31ce1705afdccc117da7d6f3d334b17` and copiedsource `a9f1a888fea5411db5713661d5fceb0fc9692f358caaa3add05abc48fa460fcd` bind this handoff. HistoricalR30holder remainsUNPROVED; original fourcase2PASS/1FAIL/1NOT_RUN,retry0 and oldR29/R28completed/failure records remain. This implementation does not close S2/A01–A12/parent24/S3–S5. CapturedUTC `2026-11-17T21:38:33.208106+00:00`.
