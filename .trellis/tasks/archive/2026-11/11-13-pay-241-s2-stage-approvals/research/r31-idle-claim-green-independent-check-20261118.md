# R31 idle batch preflight private GREEN independent check

**ACCEPTED_R31_BATCH_IDLE_PREFLIGHT_PRIVATE_GREEN_NORMAL_RACE.** The scoped private repair and all requested normal/race checks are accepted with no blocking findings. Each mode passes34 service names,1 temporal-handler name and2 owned-Redis cache names:37 distinct package-qualified named tests per mode. This accepts the private GREEN source and its verification; ROOT integration, matching builds, runtime business cases and full S2 remain separate gates.

Scope: active task `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; reviewer `/root/r27_submission_check`. Only this new report pair was written and sealed0444. No source/test/config/spec/old-report edits, Go/AST/list/TestMain/build/test/vet, SQL/DB/Redis/API/browser/runtime/signal/Git or recursive-agent execution occurred in this review. All execution evidence below is retained ROOT evidence independently parsed and hash-checked.

## Findings (fixed)

None. No reviewer source correction was needed.

## Findings (not fixed)

No unresolved defect was found in the admitted private GREEN. The singular claim sibling is explicitly unchanged by this batch-only repair; do not generalize acceptance to every claim entry. ROOT integration, matching products and the original business four-case run remain unaccepted here. Recording the new non-authorizing probe contract in the applicable server spec is a ROOT follow-on recommendation outside this report-only write scope.

## Frozen scope and implementation

Current private source contains5055 regular files. Compared with accepted RED5054, exactly three existing product files change and one108-line error regression is added. The other5051 files and their modes are identical, including the original471-line six-name R31 regression. No path is missing/nonregular, and no dependency/config/schema/frontend/authority-helper file changes. Final focal hashes still match after all six runs:

| Path | SHA256 | Mode |
| --- | --- | --- |
| `server/internal/service/task.go` | `bcfb7399e75040f8eeb809cada849e11095ba1a105b9553d322d450397fdd7bc` |0755 |
| `server/pkg/db/queries/agent.sql` | `4a661a0c1374525177d4cd103b3d47aa39ea67c1a0a32c3eb4bf4067aa6494be` |0755 |
| `server/pkg/db/generated/agent.sql.go` | `2f6233d3d901d96c03f3964b69319932cf8246f65402c2123c1d9a5c2eb7c794` |0755 |
| `server/internal/service/task_idle_claim_preflight_failure_test.go` | `1cb4fac1359b96fd29a0b4812a7e461250752b42b1eb7d56e1f750a3c8c8db8a` |0644 |
| Retained `server/internal/service/task_idle_claim_read_concurrency_test.go` | `b07397c51e304ef51e95d3c3757143600a8a37528b96005e5641df9f765f300b` |0644 |

Independent path-sorted compact sorted-key manifest SHA256 is `762d6c078873ac70186e44b20a6056f0846d70ccb693973de0d7ba417e5ead97`. ROOT static before/after manifests each hash `b7abeb50e52af6ab4726933d539161da0e9aa4992d164e7d4626e14e9aefd9e2` and have exactly the same path/hash/mode records; their traversal order differs only. All actual run results report input drift=[] on their bound frozen sources.

Only `ClaimTasksForRuntimes` changes in task.go. Its outside-batch bytes and singular/shared methods remain identical. Initial current runtime/credential validation and due-deferred handling still precede the new probe. When reclaim is due, `HasPotentialDispatchedReclaimForRuntimes` receives the complete normalized uniqueIDs and the original90-second recovery window. Its scalar EXISTS checks runtime set, dispatched status, never-started state, dispatched age against DB now(), and absent/expired prepare lease. It has no authority joins, lock_issue_execution, explicit row locks, writes or private-row output.

Reliable false skips only Step2's reclaim transaction; original EmptyClaim sampling, queued candidates, real queued claims and partial-success handling continue. A probe error returns `check potential dispatched reclaim: %w` before MarkChecked or further claim work, never an empty success. True preserves the original runInTx→LockManagedRuntimes(false)→canonical UPDATE→ValidateManagedTaskBinding block. Independent exact text comparison matches that block after one indentation shift (original SHA256 `6deced67582891cab8ae2bc58f685d5c432e37660914596bc5c539453dae75cb`). MarkChecked through the rest of the batch function is byte-identical: complete runtime keys, original checkStarted and retry/backstop parameters follow reliable false or successful canonical reclaim. Removing the single new SQL query exactly reconstructs the old agent.sql; canonical reclaim SQL and all actual writer/authority defaults are unchanged.

## Canonical generated code

ROOT's installation proof `f303f602053b9cc6720bfdc1970d07503dcb956954727acc32ab5ef2d024b02f` binds isolated generation result `d2063751403ca98197df8a9499de068c00ca8573b60185edbbc6be99927cbd9f` and pinned cached-sqlc build `931aa57644418387d78a3a94928c76cff313325b0705859990d98b75d0daf625`. The retained build uses cached `github.com/sqlc-dev/sqlc@v1.31.1`; generation uses that binary in the isolated workspace, exit0/empty output/no DATABASE_URL. No whole-source generated write occurred.

All143 generated output records were independently filesystem-hashed against the retained generation record and current GREEN. Exactly agent.sql.go differs from RED; the other142 outputs remain identical. The new25-line SQL/params/bool getter block alone reconstructs the entire old generated agent file when removed. It forwards UUID slice and float64 recovery window to QueryRow and returns the actual Scan error. Generated probe SQL SHA256 is `9251a0f76b272961327c34f858b421ac373060717e3d29cc87dbf29c872870f5`. The earlier versioned-go-run GOPROXYoff deprecation failure is retained as a generator setup failure, not overwritten or called a test PASS.

## Actual normal and race checks

| Owning selection | Normal | Race | Exact inventory per mode |
| --- | --- | --- | --- |
| Service union |34PASS/0FAIL/0SKIP |34PASS/0FAIL/0SKIP |19top/29leaf/34named |
| Temporal handler |1PASS/0FAIL/0SKIP |1PASS/0FAIL/0SKIP |1top/1leaf/1named |
| Owned-Redis cache controls |2PASS/0FAIL/0SKIP |2PASS/0FAIL/0SKIP |2top/2leaf/2named |

The service union is the source-derived16top/27named/24leaf original control inventory `aa2dd4e25ab19fddd9ed65718b1da73bc6479cf2eec91270498764570580584f`, plus the frozen six-name RED and one new childless error control. The same34 names run in normal and race; parser independently sees197 valid JSON events,34 unique starts and34 unique PASS terminals in each. The normal/race temporal selections each contain15 valid events and one exact PASS terminal; cache selections each contain14 valid events and two exact PASS terminals. All six runs have zero missing/unexpected/skipped/pending/parse-error/drift/duplicate terminals, no suite replay and empty verified stderr. Race flags and actual command/permit/profile hashes are checked. Each run uses its own fresh owning-layer database; all six before/after audits identify the selected database, matching migration ledger and0 other connections.

Actual result hashes:

- Service normal: `5d763b8b7aa6541d9b3f4c3803e499c03109f575cea6fd99a3a70e40b1ffa82d`; race: `0206354a77861c8fd1f07095e60f703e013e0b971f3697b863a9aeeb734dc095`.
- Temporal normal: `0835f94401ee635037597c0c05087854b11cfd1347fab56966b2fe8f632f6f47`; race: `925cc54146aa6eceb2d2dd5682e31001f100308883a91b7c49bef927093ad092`.
- Cache normal: `89bc19b57260fa728cee3dfbe27eedd179bd0fedda1c6a4365e4de3814d250a1`; race: `ffe72d45d7e73be766f7261f3df0d983a2955f9a704dc67265b6ef6f8cca0894`.

## Behavior and effects proved

Both true-idle orders now finish with empty/no-error ordinary claims and the original same-Q R4 projection. Claim witness account query/digest are empty and cached transaction PID0 because no account transaction begins; this does not mean the initial validation/probe made no DB reads. Read-first returns before releasing the held actual R4; idle-first accepts the genuine completed no-work preflight as designed. No obsolete reclaim barrier or arbitrary cached answer is required. The original b073 test is unchanged from genuine RED.

Actual stale-dispatched controls retain the real exclusive account fence. Normal reclaim/read PIDs33827/33836 and race34882/34891 preserve actual LockArtifactUsers55P03/zero private view while the writer is uncommitted. Reverse order keeps actual LockPasswordUser waiting on the held R4: normal33846→33839, race34904→34894. Explicit test cancellation produces context canceled, not a synthetic55P03. Fresh reclaim after release returns one real persisted task with generation+1 and the original finite prepare lease; all other target-task fields, dispatched time, current binding/epoch/installation and null admission history remain equal. Unrelated task-row assertions pass.

The new error control uses the real valid idle fixture and forwards all non-probe SQL. Only the new generated probe's Scan returns a controlled sentinel. Duplicate input normalizes to one runtime; each mode logs one hit/one Scan/exact9251 SQL/one runtime/90seconds/0transactions/0tasks. errors.Is and exact %w wrapping pass, as do zero subsequent SQL/account/reclaim/candidate work and the full covered no-effect comparison. This is an injected query-error policy regression, not a claimed native PostgreSQL failure or rollback of legitimate earlier deferred-promotion writes.

Seven complete snapshot records per service mode were reconstructed before parsing; every before/after hash is equal. Error/idle/uncommitted/blocked snapshots cover76 relations plus stage-exit. After a successful reclaim,75 other relations plus stage-exit remain equal and exact target/unrelated task assertions cover the task table separately. Task/daemon token and collaboration-run counts are0 in these covered fixtures. Real fixture setup and deliberate writer rows precede baselines. Package TestMain/teardown retained invalidation histories remain separate from per-call zero-effect proofs; no whole-database-zero-change claim is made.

`TestManagedBatchClaimLostResponseRecovery` passes through the actual handler in both modes. Its sequential assertions cover initial queued dispatch with real managed binding, immediate retry refusal, expired lease within a young dispatch window, stale dispatch with active lease, eligible exact-task recovery with credentials and generation/history, immediate duplicate refusal and started-task denial. Only fixture timestamps are aged; original product timeouts and SQL semantics remain intact. These internal checkpoints do not create extra named tests.

The two cache tests genuinely execute with the owned Redis. Any due runtime must still reclaim a stale task on a non-due runtime from the complete machine set. A positive but runtime-ineligible canonical zero-row reclaim preserves its dispatched row and defers/retriggers the original hint at the bounded retry interval. Both assertions pass in normal and race without skipped external-Redis setup.

## Redis and setup evidence

Owned allocation `2d9dea13c862f0e563196d6204c70ce587a6bbf35e9b7d309afe046424e482ea` identifies the task container/ROOT labels, loopback127.0.0.1:51802, DB12 and retained PONG. Each cache test environment preserves all15 base environment keys/values; the sole added key is REDIS_TEST_URL and it matches that allocation exactly. No ambient environment is inherited. Cache profile reconciliation `08540b5d40d72fd28e575cf99a506dc3d6710ec8848ae00e3f4c6f23d485e8d8` preserves the earlier transient-connection allocation refusal and read-only reconciliation; no historical rows were deleted or unowned connection signaled, and this was not a test rerun.

ROOT's stopped record `eebce7565211f07dfb5da625440db53f0f4b184b3177e37fd15fe7c52064bb0c` matches the same container/allocation and completed cache outputs: exit0/running=false/container retained/foreign or repeated commands0. This reviewer performed no Redis operation or shutdown. No foreign Redis/DB12 mutation is accepted or inferred.

## Verification and handoff

- Lint: **PASS**, retained scoped gofmt and service go vet, exits0/six empty stdout+stderr logs verified.
- TypeCheck: **PASS**, retained exact-source service go test-c, exit0; no TypeScript change.
- Tests: **PASS**, all requested37 distinct package-qualified named tests per mode, six exact fresh commands total.
- Source scope/generation: **PASS**,5055 files, only three products plus one new error test differ from RED; original six-name regression and other5051 files/modes remain unchanged; canonical143-output generation installed only the intended agent file.
- Runtime/fullS2: **NOT ACCEPTED HERE**. ROOT integration, matching five programs and original four-case business/runtime acceptance are pending.

Static verification `7b1939fb6f16c79528494ed5876fc4f3f5316dfa014d8c4e646ae82ee819521a` and complete ROOT handoff `fd18a18ac0527f45bf579a6a15779fed63e2dc2e74400c347e333eafd9f355df` bind this final verification. Product owner `e244dc85efe0e4615325fa7bb9c5b335dbf508cbe89028fc11c72d9e049a389b`, error-control owner `20a9d62fc051318c42ececa6f3dafd047f786b7fa12e4b5ff0028e97d775f12c` and accepted genuine RED `49f7686df12cb6463b7181eafd26fd52b4302ff80ed2483155f994f7fbdc6ae0` remain sealed and preserved; stale author-phase pending-generation notes are satisfied by the later canonical/static/executed evidence.

No historical R30 physical holder is attributed by these newly generated fixture PIDs/users. Original R30 native55P03→HTTP503 and business four-case outcomes2PASS/1FAIL/1NOT_RUN,retry0 remain their own retained evidence;37 unit/handler/cache names do not expand that business denominator. R28 completed writes are not replayed; fullS2/A01–A12/parent24/S3–S5 remain open. Captured UTC `2026-11-17T21:59:42.063Z`.
