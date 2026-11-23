# R31 ordinary idle claim genuine RED independent check

**ACCEPTED_R31_GENUINE_IDLE_CLAIM_RED_REAL_RECLAIM_CONTROLS_PASS.** The single normal run proves both proposed idle/read coexistence leaves fail on the current reachable account-fence path, while both genuine stale-dispatched reclaim writer controls pass. This is accepted RED evidence for ROOT's bounded repair planning. No product repair, GREEN or full S2 acceptance is granted by this report.

Scope: reviewer `/root/r27_submission_check`, active task `.trellis/tasks/11-13-pay-241-s2-stage-approvals`. Only this new report pair was written and sealed0444. The review parsed retained files and checked focal source/input hashes. No source/test/config/spec/old-report changes or Go/SQL/DB/Redis/API/browser/runtime/signal/Git/recursive-agent execution occurred here. ROOT owns subsequent product scope, execution and progress.

## Findings (fixed)

None. Product fixes were outside the authorized report-only scope.

## Findings (not fixed)

- File: `server/internal/service/task.go:3989`, with the original managed account fence in `managed_runtime.go`.
- Issue: A valid positive-capacity nil-cache batch poll enters the exclusive account/reclaim transaction before discovering there is no selected-runtime work. The actual idle-first R4 fails55P03; in reverse order the idle poll remains blocked until the test explicitly cancels it.
- Recommendation: ROOT may now select the previously scoped conservative, non-authorizing necessary-condition probe immediately before due reclaim Step2. A reliable negative skips only that reclaim transaction and continues the original queued-claim path. A positive keeps the entire original account/binding/admission/issue/task/generation writer path and canonical UPDATE. Errors/unknowns cannot become empty success. Current validation, writer defaults and real-work controls remain mandatory. This is a recommendation, not an implementation or authorization to widen scope.

## Exact retained run

Result `/Volumes/artisan/.multica-acceptance/pay241-r31-idle-claim-20261118-bz7ym_r1/verification/idle-claim-red-r01/result.json` SHA256 `bc6e509b2211d7117229a87cb94de8a1c13a44a97b009d80a566f728232496b5` records exit1 with **2top/4leaf/6named:3PASS/3FAIL/0SKIP**. At leaf level it is **2PASS/2FAIL**; the failed idle top is propagation of its two failed leaves. Independently parsed stdout contains64 valid Go JSON events, six unique start names and exactly one terminal for each expected name. There are no missing/unexpected names, pending tests, parse errors, pause/parallel events, input drift or suite replay. Stderr is empty.

| Actual leaf | Terminal | Evidence |
| --- | --- | --- |
| `TestOrdinaryIdleClaimAndR4Coexistence/idle_first` | FAIL | Empty claim21297 retains real exclusive account fence; same-Q R421306 gets actual shared-user55P03. |
| `TestOrdinaryIdleClaimAndR4Coexistence/r4_first` | FAIL | Held R421309 blocks actual account query21318; explicit250ms test cancellation yields context canceled, tasks0. |
| `TestOrdinaryReclaimPreservesAuthorityFences/reclaim_first` | PASS | Real reclaim21326 denies R421336 with55P03/zero private view, then commits one permitted task mutation. |
| `TestOrdinaryReclaimPreservesAuthorityFences/r4_first` | PASS | Held R421340 blocks writer21349; cancellation causes no effects; fresh claim after release returns one actual reclaimed task. |

The command is the exact normal `go test -json -count=1 -timeout=120s ./internal/service -run '^(TestOrdinaryIdleClaimAndR4Coexistence|TestOrdinaryReclaimPreservesAuthorityFences)$'`, run from the frozen source's server directory. Command SHA256 `7978560a9d2b94e0d597942f3d9e73dadcb5f63959e1c2b10ed61a815217b7c7` binds final permit `d22734723cd1db11d4199c8ba3f1babbae15c83f608bb39aea0141d85cd38fae`, accepted boundary `0718f58bc8c358ff0333ce8784ab62d57fa99c1bb0f7fd441975a69f1f56738e`, fresh RED profile `b0e44807f28b24fe05fbb9b2563fa9af8af064e4a6d6fce7c08efab443319d15` and explicit database `multica_pay241_20261117_s2_idleclaim_red_r31`. Ambient environment inheritance is false; maximum command count1 and suite_reran=false are retained.

## Genuine empty and current-authority prerequisites

The accepted471-line/18961-byte/0644 test remains SHA256 `b07397c51e304ef51e95d3c3757143600a8a37528b96005e5641df9f765f300b`. Pre-run independent review verified5054files/5053 protected bytes+modes and the sole test addition; the real runner reports zero protected-input drift. Focal task/query/generated/managed-runtime/fixture inputs still match those accepted hashes and modes. The owner JSON `e4a781ec726c392290904ba81df851d27654f78d7dcc4bbb88b04c208221dda8` remains sealed0444.

The actual source calls `r31Fixture` before each leaf. The real stage-capabilities family registers/reads the current QA's managed runtime and obtains its actual artifact R4 baseline. Current QA password session, runtime owner, binding ID/epoch, online/current-heartbeat/private runtime+agent, positive agent capacity and maxTasks1 are retained. Nil EmptyClaim/ReclaimCheck are asserted before the real claim. The absence assertion covers **all ordinary task rows on the selected QA runtime**, not just queued rows. Both idle leaves pass these prerequisites and reach the actual claimed barrier/wait; their only assertion errors are the expected R4 failure at402 and idle claim non-success at417. This is neither a setup failure nor maxTasks0/invalid-runtime/failed-authority/cached-answer evidence.

Global snapshot task count1 is the deliberate artifact-producer task from the real baseline fixture on another runtime. It does not contradict the selected-Q runtime's zero-work prerequisite. Neither idle leaf seeds a reclaim task; only the two writer controls do.

## Native query, error and lock ordering

Idle-first actor `a06266a0-2b4e-4e94-8672-02f661a13318` appears in both real witnesses. Claim PID21297 records canonical `LockPasswordUser`, SQL SHA256 `a68ab03fd570aef97e0c37f65a25e1c2bb7e1ee88d525c6173fe943210025880`, singleton matching QA UUID, account error=nil and final tasks0/error=nil. The source barrier follows actual account Scan and real reclaim Query return while TxStatus isT; reaching R4 without a barrier assertion error confirms that branch. The distinct PID record pairs21297/21306. Before claim release, R4 PID21306 captures actual `LockArtifactUsers` SQL SHA256 `43bdd2c80bda2504391b87839ea62ff27895c1bbf8f6e87e1f8da02ac3f10d04`, phase rows_err, PgError55P03 and row-lock failure on user. Its candidate projection is empty. The claim subsequently completes with no task.

Idle R4-first actor `7f2d129e-5c74-44cb-a30b-3e8519abb2f4` has actual writer PID21318 and held reader PID21309. The canonical pg_stat_activity/pg_blocking_pids observation logs that exact LockPasswordUser backend blocked by that reader. The test then cancels its original5s context after the retained250ms diagnostic interval. Account error is `context canceled` (*errors.errorString), outer service error is `reclaim stale dispatched tasks: context canceled` (*fmt.wrapError), first_pg_error=null and tasks0. This is **not55P03** and no NOWAIT error is invented. The held R4 finishes its original projection after release and the fresh subsequent idle claim's empty/no-error assertions pass; there is no additional failure beyond417.

Captured R4 first_pg_error.uuid_args is an empty array because the reused tracer captures scalar UUID/string arguments, not the UUID slice supplied to this query. Same-Q identity is established by the real context/first-current-user code path plus the matching actor and singleton claim witness; this report does not invent a raw R4 UUID-array capture.

The actual-work controls use their own QA identities and deliberately seeded eligible stale-dispatched task with current matching binding/epoch/installation, generation3, no started time/lease and retained null admission version. Current installation active/accepting/positive-admission and ValidateManagedTaskBinding assertions pass; original reclaim SQL expressly supports this branch. Both controls return one real row after allowed release, persist that same returned row, advance generation once to4, acquire the original finite future prepare lease and preserve all other task fields, original dispatched timestamp and binding/admission history. Unrelated task equality assertions pass. The second writer's context-canceled refusal remains distinct from the read55P03. No task, token, run or cached permission is substituted.

## Effects and retained package history

All six fragmented snapshot records were reconstructed before parsing:

- Idle-first:76relations + stage-exit, before=after `4dc1b5f8a9b9174f8c34a51187c15dc71090ca08da2b5e012e529f3aa8dcfdfe`.
- Idle R4-first:76relations + stage-exit, before=after `e9d035c63dcd600fa1e26c9ab0bcff78b1c82339ac6f13f840284ab4ce005de4`.
- Reclaim-first uncommitted:76relations, before=after `a95557c05bc9ad58567f2dac5d259663b0fafe4a73fbee2ae902a45ad8904bc1`.
- Reclaim-first after allowed task mutation:75otherrelations + stage-exit, before=after `26e73ec342de0e882cc4ad24f18e185d8a1021dd93d27b5ea7441240fae5ad1d`; target/unrelated task assertions are separate and passed.
- Reclaim R4-first blocked:76relations, before=after `7019fa2a2591ff1b62c22dd777995b95657892ea96b380891efcff5d3853e528`.
- Reclaim R4-first after allowed task mutation:75otherrelations + stage-exit, before=after `4bc73424e914630812c33317f0adf9ddab562fc156561f0a8b1c2700006dae24`; exact task/history assertions passed.

Task/daemon token counts and collaboration run counts are0 in every covered snapshot. Baselines follow real fixture/setup; no partial committed claim effects precede release. The allowed writer delta is generation/lease only, not a claim that the entire task table is ignored or unchanged.

Retained DB audit hashes `75972970ea1255c385810dec0d78467709dbfc98d96f73353f562b11b1901784` before and `5ffbc8d71bf4df57b31a18ef34af6a5e31554c3a7f1dceaa4ffc7ab7e7797d3e` after both identify the exact RED database with0 other connections. Migration ledger remains `d648c6e258f50d6abbeb3f025475ea6b3bb6ff02501c57713897fc1be606ec3e`. Fixture teardown deliberately retains config invalidation21→53 and knowledge invalidation18→30; those are the only table-count changes. All business fixture rows drain. The per-call no-effect proof does not become a whole-database-zero-change claim.

## Verification and limits

- Lint: **PASS** — previously independently accepted retained scoped gofmt/service vet on the unchanged frozen inputs.
- TypeCheck: **PASS** — accepted compile-only service check; no new Go execution by this reviewer.
- Tests: **FAIL as genuine RED**, exact3PASS/3FAIL/0SKIP named outcomes; both real-work leavesPASS. This is accepted regression evidence, not a GREEN declaration.
- Raw logs: stdout34657bytes/hash `7c6ffa03dcc34ab420ee0cc1b1b8407fc2b5b0865607a135ae4bcbdc857ca586`,64valid events; empty stderr SHA256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- Database after: recorded0otherconnections; this reviewer made no live SQL query.
- Spec/source edits: none; any later repair/spec sync is ROOT's separate task scope.

These PIDs and newly generated QA users identify the holder/read contention in **this controlled run only**. They do not identify the old R30 physical holder or prove a new browser HTTP503; the retained original R30 native55P03→HTTP503 correlation remains its own accepted record. Original business four-case outcomes remain2PASS/1FAIL/1NOT_RUN,retry0, and the six unit-test names do not expand that denominator. R28 completed writes remain unreplayed; fullS2/A01–A12/parent24/S3–S5 remain open. Captured UTC `2026-11-17T21:32:07.017Z`.
