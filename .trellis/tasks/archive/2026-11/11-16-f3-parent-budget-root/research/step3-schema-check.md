# F3 step 3 (schema) independent check

Date: 2026-11-17. Checker: trellis-check sub-agent (did not write step 3).
Product code checked: worktree `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`, branch `codex/f3-budget-root-20261116`, base `86fea651c`, all uncommitted.
Paths below are relative to that worktree's `server/` unless they start with `/`.

## 1. Verdict

**Pass, with five table-shape decisions for the lead before commit.**

- No defect found in the migrations, the queries or the generated code. Every hard rule holds (no FK / REFERENCES / cascade / enum / trigger; twelve single-statement concurrent index files, all registered; guarded downs).
- Every matrix row B01-B60 (with B24a, B35a) and S1 can be expressed. Five rows are served only through a jsonb field or a read-time rule that the design has not written down; those are the decisions in section 2.
- The unique key `issue_budget_attempt_claim_key (task_id, claim_generation)` is safe: no product path reaches a successful finalize twice for one generation.
- One test gap found and fixed (test file only): 17 of the 54 CHECK constraints could be dropped with every test still green. After 23 added cases, 54 of 54 are killed.
- All five requested mutations are caught, each by at least one test with a specific message.
- Final verification after my edit: all suites pass on fresh clones, including `-race`. Generated code is byte-identical to a fresh `sqlc generate`.

## 2. Decisions the lead must make

Ordered by how much later steps depend on them. None is a defect; each changes a table's shape, so nothing was changed.

| # | Decision | Why now | My recommendation |
| --- | --- | --- | --- |
| D1 | `issue_budget_observation` has `task_id` but no `claim_generation` | Design §9.2 and §9.5 (design.md:217, 241) say the in-flight gap observations carry task id and claim generation and are matched against `issue_budget_attempt (task_id, claim_generation)` at read time to decide whether `remaining` is null. Today that match needs `(detail->>'claim_generation')::bigint`. Step 4 writes these rows | Add `claim_generation bigint CHECK (claim_generation >= 0)`, nullable, plus the insert argument. No index needed (reads go through `issue_budget_observation_root_idx`) |
| D2 | Batch reference on entries: three-column copy vs operation `id` | Restructuring after commit is add + backfill + drop; this is the one item that is expensive later | Give `issue_budget_operation` an `id uuid NOT NULL` with its own unique index and replace `operation_scope_kind` / `operation_scope_id` / `operation_key` on the entry by one `operation_id uuid`. Reasons in 3.6. Keeping the three columns is workable, not wrong |
| D3 | `issue_budget_root.tombstoned_at` | Design §8 (design.md:130) says root deletion is derived from the root issue's member row; the column (866 up:45-48) is a second, lagging copy that can stay NULL forever, and §9.4 (design.md:234) has a refused reopen write it, which is a refusal with a committed side effect | Drop the column and derive at read time (one lookup on `issue_budget_member_issue_key`). If kept, write the rule "non-null means deleted; null means check the member row" and let no reader branch on it alone |
| D4 | `would_refuse` has two homes: `issue_budget_attempt.would_refuse_reason` (one value) and `issue_budget_observation` rows (any number) | B46 can produce several reasons for one attempt (closed, deleted, unbounded unknown). Step 5 writes one of the two; B50's `show` may count from the other | Make observation rows the single home and drop the attempt column; or keep the column as "primary reason" and write the precedence order into design §9 before step 5 |
| D5 | The attempt records no `policy_revision` / `policy_epoch` | B40's epoch has no consumer anywhere in the schema: the root's counter trivially never repeats, but no would-refuse evidence can say which policy it was judged under | Add both as nullable bigint on the attempt, or state in the design that the finalize operation's `result` carries them. Additive later, so this one can wait if you prefer |

Can wait (additive, nullable, or a rule for a later step): 3.7 to 3.10.

## 3. Findings by severity

### 3.1 Fixed: 17 of 54 CHECK constraints were not pinned by any test (medium)

Method: on one clone, drop each CHECK in turn, run `TestIssueBudgetSchemaCatalog|TestIssueBudgetSchemaConstraints`, restore it (`/tmp/f3-step3-check/mut-checks.sh`).

Red (`/tmp/f3-step3-check/checks-result-red.txt`): 37 killed, 17 survived:

- root: `ledger_revision`, `policy_epoch`, `policy_revision`, `unknown_count`
- member: `binding_version`
- attempt: `claim_generation`, `protocol_version`, `would_refuse_reason`
- entry: `issue_budget_entry_check2` and `_check3` (usage iff invocation / scope; the only existing case violated both at once, so either could go), `entry_json` size, `receipt` object
- operation: `actor_type`, `operation_kind`, `result` object
- observation: `detail` object, `reason`

Fix: 23 refusal cases added to `internal/handler/issue_budget_schema_test.go` (lines 375-378, 383, 390-392, 400-406, 431-433, 435-436; helper `sized` at :347). Green (`/tmp/f3-step3-check/checks-result-green.txt`): 54 killed, 0 survived, run with all constraints restored exits 0. No migration, query or generated file was touched.

### 3.2 Needs decision: D1 to D5 (medium)

See section 2. Evidence: `migrations/866_issue_budget.up.sql:178-189` (observation columns), `:143-147,153` (batch reference), `:45-48` (root tombstone), `:97` (attempt would-refuse reason), `:72-106` (attempt has no policy columns).

### 3.3 Verified, no change: the claim key cannot turn observe-only into a claim failure

`FinalizeTaskClaim` (`internal/service/task.go:3734`) does not itself consume the claim: its compare-and-set leaves the row `dispatched` at the same generation. The guarantee comes from the callers:

- Every path that hands a dispatched row to a handler bumps the generation: `pkg/db/queries/agent.sql:833` (claim), `:985` and `:1059` (reclaim). All three filter on `runtime_id`, so a claimed row always has a runtime (the attempt's `runtime_id NOT NULL` holds although `agent_task_queue.runtime_id` is nullable).
- `RequeueAgentTaskAfterClaimFailure` (`agent.sql:946`) keeps the generation; the next claim bumps it.
- Each handler calls finalize once per claimed row (`internal/handler/daemon.go:1911` batch, about `:3663` single). `ClaimTasksForRuntimes` (`task.go:3915`) cannot return one task twice: reclaimed rows are already dispatched and the later step only claims queued rows.
- `runInTx` (`task.go:6547`) has no retry. An ambiguous commit ends in requeue and generation + 1, which is B35's `superseded_unproven` path.

So B08 and B35 get a new attempt at the next generation, and B35a leaves no row because attempt and reservation roll back with the finalize transaction. Recommendation for step 5: state this invariant in the code comment where the attempt is inserted, and treat a 23505 on this key as a bug report, not as a refusal reason.

### 3.4 Verified, no change: digests and their coupling

- `decode('6275646765742d70726963652d736e617073686f742d76310a','hex')` is `budget-price-snapshot-v1\n` and `decode('6275646765742d656e7472792d76310a','hex')` is `budget-entry-v1\n` (866 up:104, :149). They equal the domains in `internal/budget/price.go:118` and `internal/budget/ledger.go:388`.
- `digest()` (`price.go:121-130`) is hex of `sha256(domain || json.Marshal(v))`, so the CHECKs recompute exactly what the package computes.
- Limit of the proof: the CHECK shows the digest matches the stored bytes, not that the bytes are the package's marshalling of a valid entry.
- Opinion: keep the coupling. It makes drift between a stored form and its digest impossible for any writer, including a hand-run SQL fix. Two cheap improvements: name the two digest constraints (today they are the positional `issue_budget_entry_check` and `issue_budget_attempt_check2`, which a later migration would have to hardcode) and define `schema_version` as the stored-form version so a second domain can be added as a `schema_version`-conditional CHECK.

### 3.5 Verified, no change: `budget_root_id NOT NULL` on operation and observation

- Operation: refused operations are not stored and are re-evaluated on replay; every committed operation has a root (activation creates it in the same transaction). Nothing is lost except an audit trail of refused management requests, which no matrix row asks for. B33's `pending` cannot be stored, and the row expects all three answers to read as unknown. Wording rule: not found means "no committed result", not "not committed".
- Observation: every observation in B11, B46, B55, V17, V18 can be anchored on the attempt's root or the bound issue's root. With neither there are zero budget writes (B47). The one case that cannot be recorded is a broken binding (a member row whose root row is missing): it must be an explicit error, which is what B47 asks.

### 3.6 Opinion asked for: the envelope columns

Required columns: `source` and `schema_version` NOT NULL is right. The others cannot be made required in the database while `source` is an open pattern; enforce per source in the step 6 handler and add a CHECK when the source list is frozen. Tightening later is cheap (`ADD CONSTRAINT ... NOT VALID`, then validate).

Real columns are justified for `attempt_id`, the batch reference, `completeness` and `producer_epoch`. `payload_digest`, the two version strings and the three timestamps are audit-only but cheap. `received_at` duplicates `created_at` unless it is defined as request arrival time; define it or drop it in step 6.

Batch reference (D2), reasons for the `id`:

- The operation key is caller-supplied text of up to 200 bytes; the copy puts it on every row of an immutable ledger.
- Two of the three columns repeat `attempt_id` and the constant `'attempt'` for nearly every reported entry.
- The scope-kind list exists in two CHECKs that must stay in step; `'issue'` is accepted on an entry although activation never appends one.
- An `id` also lets observation rows and the attempt cite the operation that wrote them.
- Cost: one more unique index (one migration file, one `concurrentIndexCleanups` entry, one line in 879).

### 3.7 Can wait, additive: what has no home yet (low)

- Per-reason unknown counts (B50): the root stores one `unknown_count`. Per-reason numbers need a ledger replay per read or a later column. Already assigned to step 5.
- One `price_snapshot` per attempt: usage entries carry their own `price_digest` (`internal/budget/model.go:98-105`), so an attempt that uses two models has a digest with no stored snapshot. A content-addressed snapshot table can be added in step 6.
- With the generic envelope gone, the final watermark and source revision have no column. Provider and model exist only inside the price snapshot.
- `actor_type` allows `member` and `runtime`; a sweeper-written hold has no actor value. Either such facts get no operation row, or a `system` value is added.

### 3.8 Writer rules for later steps, no schema change (low)

- Entry identity and scope identity are root-wide. Daemon-supplied event IDs must be namespaced by attempt and still fit 200 bytes after the prefix. The package accepts longer IDs; the table refuses them with 23514.
- NUL: PostgreSQL refuses it in the text columns with 22021. The handler must reject it first. Inside `entry_json` it is escaped and harmless.
- `ReservationID` must be the canonical lowercase UUID string of the attempt; the ledger compares strings, the column compares UUIDs.
- `kind`, `attempt_id`, `scope_id`, `invocation_id` are not tied to the bytes. Build them in one constructor from the `Entry` and test that constructor.
- `attempt.reserved_ticks NOT NULL`: an unsized reservation is stored as 0 and must come with a gap entry.
- Gaps have two homes by design (ledger gap entry under the root lock, observation row without it). Write the rule down in step 5 so B50's counts add both without double counting.
- `receipt`, `result`, `detail` jsonb have no size bound. Observation rows are unbounded per root (one `root_conflict` row per repeated move).
- `budget.ReasonPreActivationUnknown` has no writer under the observation-row decision.
- `ListIssueBudgetEntries` is an unbounded read.

### 3.9 Guarded downs (low, no change)

`866` and `879` down take `ACCESS EXCLUSIVE ... NOWAIT` on each existing table, refuse when any holds a row, and are atomic because the runner sends a file as one `Exec`. Outside tests the only caller of `migrate down` is `make migrate-down` (`Makefile:397-400`), which rolls back everything and already stops at older guards. An empty database rolls back cleanly (`TestIssueBudgetMigrationsRunUpDownUp`).

One residual window: 879 down releases its locks before 878-867 drop the indexes. A live writer that inserts in between makes 866 down refuse with the unique indexes already gone. This needs `migrate down` against a serving database, which the upgrade guide already forbids; recovery is `migrate up`.

### 3.10 Minor (no change)

- The query is named `ListIssueBudgetEntries`, not `ListIssueBudgetEntriesByRoot` as the dispatch said. It is scoped by workspace and root and ordered by `seq`.
- 879 does not check operator class, collation or sort order of an index.
- The B56 fixture has no operation or observation row.
- The twelve `concurrentIndexCleanups` entries (`cmd/migrate/main.go:139-150`) sit before the older 862/863 lines; cosmetic.
- `issueBudgetInsertError` (`issue_budget_schema_test.go:536`) open-codes an insert inside a rolled-back transaction for expected refusals. `internal/testutil` has no expected-failure insert, and this is not the banned `INSERT ... RETURNING id` plus cleanup pattern. Accepted.

## 4. Hard rules

| Rule | Result | Evidence |
| --- | --- | --- |
| No FOREIGN KEY / REFERENCES / cascade | Holds | catalog check in `TestIssueBudgetSchemaCatalog`; text check in `TestIssueBudgetTableMigrationContract`; mutations m1a, m1b, f1fk |
| No enum, no trigger, no PK or unique constraint | Holds | same catalog test; 54 CHECK constraints are the only constraints |
| Each index its own single-statement concurrent file | Holds | 867-878 are one line each; mutation f4two |
| Registered in `concurrentIndexCleanups` | Holds | `cmd/migrate/main.go:139-150`; mutation f5cleanup |
| Idempotent DDL | Holds | indexes `IF NOT EXISTS` / `IF EXISTS`; downs use `to_regclass` and `DROP TABLE IF EXISTS`. 866 up uses plain `CREATE TABLE`: acceptable, the migration is unconditional and the file is atomic |
| `workspace_id NOT NULL` on all six, registered `retain_audit` | Holds | mutations m2, f2ws |

## 5. Matrix coverage

"jsonb" marks a row served through an untyped field; "rule" marks one that needs a written read-time or writer rule.

| Row | Served by | Note |
| --- | --- | --- |
| B01 | `issue_budget_root_issue_key (workspace_id, root_issue_id)`; operation key `(workspace_id, 'issue', issue_id, key)` + `intent_digest` | |
| B02 | `issue_budget_member_issue_key`; fence | |
| B03 | root key has no state predicate; member rows survive deletion; `policy_revision`; amounts untouched | |
| B04 | member `bind_source='create'`, `bound_parent_issue_id`, `binding_version` | |
| B05 | member key + fence | |
| B06 | member `bind_source='activation'`; observation `coverage_gap` / `in_flight_legacy` with `task_id` | generation only in `detail` (D1) |
| B07 | new `task_id`, new attempt, same `budget_root_id` | |
| B08 | attempt claim key; `state='superseded_unproven'`; hold entry | 3.3 |
| B09 | no write | |
| B10 | member row unchanged; `bound_parent_issue_id` compared with current parent at read | rule |
| B11 | observation `would_refuse` / `root_conflict`, `issue_id` | |
| B12 | member `tombstoned_at`; entries and attempts keep ID copies | |
| B13 | root issue's member `tombstoned_at`; observation `would_refuse` / `root_deleted` | root column redundant (D3) |
| B14 | `workspace_id` in every query | |
| B15 | usage entry with overlap in `entry_json`; `attempt_id` nullable for usage | |
| B16 | no reference to `task_usage` or hourly tables | |
| B17-B23 | `entry_json` bytes; root `actual_known/estimated/reserved/unknown_hold/overrun_ticks`, `unknown_count` | pure rows |
| B24 | estimate ticks and price digest inside `entry_json`; attempt `price_snapshot` + `price_digest` | one snapshot per attempt (3.7) |
| B24a | bigint columns, decimal strings in `entry_json` | |
| B25-B30 | `entry_json`; `scope_id`, `invocation_id`; entry identity key | |
| B31 | one transaction; operation row + batch reference | D2 |
| B32 | operation scope key, `intent_digest`, `result` | |
| B33 | operation lookup: found or not | `pending` not storable, by design (3.5) |
| B34 | attempt + reserve entry + operation row (`scope_kind='attempt'`, `actor_type='runtime'`) | |
| B35 | claim key; `superseded_unproven`; observation `coverage_gap` / `attempt_unbound` | |
| B35a | same transaction as finalize | |
| B36 | hold entry; attempt `state` | sweeper has no actor value (3.7) |
| B37 | no release entry is written | |
| B38 | root lock; entry identity key | "one terminal contribution" is a reducer rule, not a constraint |
| B39 | operation `actor_type` / `actor_id` | auth is step 7 |
| B40 | root `policy_revision`, `policy_epoch` | epoch has no consumer (D5) |
| B41 | operation `actor_id`, `result` | staleness is jsonb + rule |
| B42 | attempt by id, entry by attempt | |
| B43, B44 | `json:"-"` on all generated structs; root keeps the minimal amounts | step 7 |
| B45 | root lock; `would_refuse_reason` or observation | D4 |
| B46 | observation `would_refuse` rows, any number | attempt column holds one (D4) |
| B47 | no row without a member; broken binding is an error | 3.5 |
| B48 | entry `kind='exclusion'`, mechanism in the stored form | |
| B49 | entry `kind='gap'` | sub-reason only inside `entry_json` |
| B50 | root projection columns; observations for pre-activation | per-reason counts need replay (3.7) |
| B51 | section 4 | |
| B52 | root `mode='off'` | |
| B53 | attempt `protocol_version` NULL; gap entry or observation | two homes (3.8) |
| B54 | untouched | |
| B55 | attempt `issue_id` NULL + `context_parent_issue_id`; observation `attribution_difference` per created issue | |
| B56 | no FK; ID copies | tested |
| B57 | member keeps `bound_parent_issue_id` | |
| B58 | operation actor | step 7 |
| B59 | no depth column or limit | |
| B60 | member rows; observation `coverage_gap` | |
| S1 | `mode IN ('off','observe')` | enforce cannot be stored |

## 6. Mutation table

DB-level mutations ran on their own clone migrated to 879; file-level ones in a scratch copy at `/tmp/f3-step3-check/scratch/server`. The worktree's migration files were never edited. Outputs: `/tmp/f3-step3-check/<name>-*.txt`.

| Name | Mutation | Caught by |
| --- | --- | --- |
| m1a | FK from `issue_budget_member.workspace_id` to `workspace` | Catalog "1 foreign keys, want 0" and trigger count (`issue_budget_schema_test.go:79`, `:85`); teardown test 500 / 23503 (`issue_budget_teardown_test.go:73`) |
| m1b | FK attempt to issue `ON DELETE SET NULL`, plus an inbound FK from a probe table | Catalog "2 foreign keys"; B56 `expected 204, got 500` (`issue_budget_delete_test.go:95`); manifest test reports the unclassified table |
| m2 | drop `workspace_id` from observation | Catalog "5 of 6" (`:89`); manifest "lost workspace_id" (`workspace_delete_manifest_test.go:495`) |
| m3 | claim key rebuilt non-unique | Catalog `unique:false ... want unique:true` (`:202`, `:226`); Constraints "want SQLSTATE 23505" (`:407`) |
| f1fk | `REFERENCES issue(id)` in 866 | `TestIssueBudgetTableMigrationContract` (`internal/migrations/issue_budget_test.go:46`) |
| f2ws | observation without `workspace_id` in 866 | same test (`:55`) |
| f3nonunique | 872 without `UNIQUE` | `TestIssueBudgetIndexMigrationContract` (`:75`); `migrate up` exit 1 at 879 (P0001); two `cmd/migrate` tests |
| f4two | second statement in 870 | index contract (`:78`); `migrate up` exit 1 (25001); `TestEveryConcurrentUpBuildHasCleanup` |
| f5cleanup | 874 entry removed from `concurrentIndexCleanups` | `TestEveryConcurrentUpBuildHasCleanup` (`cmd/migrate/migrate_mul5999_index_retry_test.go:85`) |
| f0base | no edit (control) | all green, 0 SKIP |
| per-CHECK | each of 54 CHECKs dropped in turn | 37 of 54 before, 54 of 54 after (3.1) |

Expected values in the tests are literals; the fixture projection was cross-checked against `budget.Reduce`. Tests use `dbfx.*` and `testutil.Call(...).Want(...)`; no `httptest.NewRecorder()` quartet.

## 7. Commands and results

All from the worktree's `server/`. Each DB-backed suite ran alone, with `-v`, output to a file.

| Command | Exit | Result | Output |
| --- | --- | --- | --- |
| `gofmt -l` on the step 3 Go files and `internal/budget` | 0 | no lines | `/tmp/f3-step3-check/fin-gofmt.txt` |
| `go build ./...` | 0 | | `fin-build.txt` |
| `go vet ./...` | 0 | | `fin-vet.txt` |
| `env -u DATABASE_URL go test ./internal/budget/... -count=1` | 0 | ok | `fin-budget.txt` |
| `go test ./internal/handler/ -run 'TestIssueBudget\|TestDeleteMemberIssueKeepsBudgetRows\|TestWorkspaceDeletionLeavesBudgetRowsUntouched\|TestWorkspaceDeletionManifestCoversPublicSchema' -count=1 -v` on `f3_budget_c3_finhandler` | 0 | 7 PASS, 0 FAIL, 0 SKIP | `fin-handler.txt` |
| same with `-race` on `f3_budget_c3_finrace` | 0 | 7 PASS, 0 FAIL, 0 SKIP | `fin-handler-race.txt` |
| `go test ./cmd/migrate/... -count=1 -v` on `f3_budget_c3_fincmd` | 0 | 149 PASS, 0 FAIL, 1 SKIP | `fin-cmd-migrate.txt` |
| `go test ./internal/migrations/... -count=1 -v` on `f3_budget_c3_finmig` | 0 | 572 PASS, 0 FAIL, 0 SKIP | `fin-migrations.txt` |
| `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` in the scratch copy, then `diff -r` against the worktree's `pkg/db/generated` | 0, 0 | no difference | `sqlc-generate.txt`, `sqlc-diff.txt` |

The seven handler tests: `TestDeleteMemberIssueKeepsBudgetRows`, `TestIssueBudgetLedgerLoadIsOrderedByItsKey`, `TestIssueBudgetQueriesRoundTrip`, `TestIssueBudgetSchemaCatalog`, `TestIssueBudgetSchemaConstraints`, `TestWorkspaceDeletionLeavesBudgetRowsUntouched`, `TestWorkspaceDeletionManifestCoversPublicSchema`.

The one SKIP is `TestCommentContentBigramRequirementMatchesRealPGBigm` (older test, needs the pg_bigm extension, unrelated to step 3). The three budget tests and the three cleanup-registration tests in `cmd/migrate` all passed.

Every clone was created with `CREATE DATABASE f3_budget_c3_<name> TEMPLATE f3_budget_base_865` and migrated with `DATABASE_URL=... go run ./cmd/migrate up` (exit 0 for every unmutated tree). Scripts: `/tmp/f3-step3-check/final.sh`, `mut-db.sh`, `mut-file.sh`, `mut-checks.sh`; run log `final-run.txt`.

Silent-skip hazard, seen once: a clone name containing a hyphen made `CREATE DATABASE` fail, and `cmd/migrate` then returned exit 0 with 3 SKIP (`f0-baseline-*.txt`). That run is not counted anywhere above.

## 8. What I changed

One file: `internal/handler/issue_budget_schema_test.go` in the worktree (sha256 before `fad77e46...59bf9be`, after `7ed7318a...38dba867`): two imports, the `sized` helper, 23 refusal cases. Nothing was staged, committed or pushed. The shared checkout was not edited apart from this report.

## 9. What I could not verify

- The step 4 to 7 readers and writers do not exist, so "can be expressed" in section 5 is a reading of the schema against the design, not a run.
- The double-finalize conclusion (3.3) is static reading of the claim, reclaim, requeue and finalize paths; no two-connection test exists yet.
- Index choice under load: the planner may sort instead of walking `issue_budget_entry_seq_key` on small data. Results are ordered either way.
- The `checks-run-*.txt` files hold the green run; the red state survives only as `checks-result-red.txt`.
- The database container is amd64 emulated on arm64; no timing in this check is meaningful.

## 10. Databases

Created and dropped (15): `f3_budget_c3_handler`, `_m1a`, `_m1b`, `_m2`, `_m3`, `_f0base`, `_f1fk`, `_f3nonunique`, `_f4two`, `_f5cleanup`, `_checks`, `_finhandler`, `_finrace`, `_fincmd`, `_finmig`. Drop log: `/tmp/f3-step3-check/drop.txt` (all exit 0).

After the drops, `select datname from pg_database where datname like 'f3_budget%'` returns only `f3_budget_base_865`, still at `865_knowledge_automation_origin` with no `issue_budget*` relation. No test or migration ran against the template.

A sixteenth clone, `f3_budget_c3_plan`, was created for section 11 and dropped by the same script (`/tmp/f3-step3-check/plan-run.txt`, drop exit 0); the listing again shows only the template.

## 11. Addendum: the lead's round 2 questions

### 11.1 Does `TestIssueBudgetLedgerLoadIsOrderedByItsKey` prove anything useful?

It proves one modest thing: the query text and the key agree, that is, an index named `issue_budget_entry_seq_key` can return this exact statement's rows in `seq` order with no sort. It does not prove which plan production will use, and its own comment says so (`internal/handler/issue_budget_queries_test.go:21-24`). Keep it as a contract between the query file and the index, not as a performance test.

Evidence that it is more than "a forced plan can use some index" (`/tmp/f3-step3-check/plan.sh`, outputs `plan-*.txt`):

| State | Plan test |
| --- | --- |
| migrated, empty table | PASS |
| seq key rebuilt as `(seq, budget_root_id)` | FAIL: `Sort` over an index scan of `issue_budget_entry_identity_key` (`:82`) |
| seq key absent (implementer's run, `/tmp/f3-step3/r2/mutation-no-seq-key.txt`) | FAIL, same shape |
| 120,000 rows in 201 roots, analysed | PASS |

The catalog test and 879 already pin the key's columns, so the plan test adds one thing only: it would fail if someone changed the query's `ORDER BY` or its filter so that the key no longer served it.

### 11.2 Does the list query need anything else?

No index change. What it needs is a bounded variant, and that belongs to the step 5 decision on ledger loading, not to step 3.

Unforced plans on 120,000 rows of about 450 bytes (200 roots of 500 interleaved entries, one root of 20,000), after `ANALYZE` (`/tmp/f3-step3-check/plan-unforced.txt`):

| Read | Plan chosen | Cost |
| --- | --- | --- |
| whole ledger, 500-entry root | bitmap scan on the seq key, then sort | 509 buffers, 314 kB in-memory sort |
| whole ledger, 20,000-entry root | parallel sequential scan of the whole table, then sort | 9,307 buffers, sort spilled 7.6 MB to disk |
| `seq > 10000 ORDER BY seq LIMIT 500`, 20,000-entry root | index scan on the seq key | 47 buffers, no sort |
| `seq > 19990 ORDER BY seq`, 20,000-entry root | index scan on the seq key | 4 buffers, no sort |
| `seq > 0 ORDER BY seq LIMIT 500`, 500-entry root | bitmap scan, then sort | 506 buffers |

Reading:

- For ordinary roots the full load is cheap whichever plan is chosen; the sort is in memory.
- For a large root the full load reads the table and sorts on disk. If step 5 replays the whole ledger under the root lock on every write, this is the cost per write. The 20,000-entry root is 17% of this table, which is why the planner chose a sequential scan; treat it as a worst case, not a forecast.
- A tail read (`seq > @after_seq`) walks the key directly. So the query that step 5 will want is "entries after the revision I already hold", which pairs with storing a serialised ledger state on the root (the open step 5 item at design.md:165).

Recommendation: leave `ListIssueBudgetEntries` as it is for step 3. In step 5 add a tail query `... AND seq > @after_seq ORDER BY seq` with that decision, and give the full load a stated size ceiling. The container is amd64 emulated on arm64, so only plan shapes and buffer counts are meaningful here, not times.

### 11.3 Envelope: required, over-constrained, unnecessary

- **Should `schema_version` be required on server-written entries?** Yes. It versions the stored form in `entry_json`, which every entry has whoever wrote it, and the server always knows which form it marshalled. It is also what a future second digest domain would switch on (3.4).
- **Over-constrained?** I found no NOT NULL or CHECK that a server-written or legacy-daemon entry cannot satisfy:
  - `source` required: a server-written entry names the server. A daemon without the protocol reports no entries; its gap is written by the server.
  - Usage needs `invocation_id` and `scope_id`, and reserve, hold and settle need `attempt_id`: these are the same rules `validateUsage` and `validateEntry` (`internal/budget/ledger.go:175-244`) already apply, so the table is no stricter than the reducer.
  - Everything a daemon supplies is nullable.
  - One thing is over-permissive, not over-constrained: `operation_scope_kind` accepts `issue`, which no entry can have.
- **Unnecessary for the observe-only slice?** No S1 reader needs any of `adapter_version`, `normalizer_version`, `producer_epoch`, `payload_digest`, `observed_at`, `reported_at`, `received_at`, `completeness`. The reducer consumes none of them (finality is the settle entry's proof, and B26 forbids ordering by arrival time). S1 readers need `attempt_id`, `source`, `schema_version` and the batch reference.
  - Keep the unread ones anyway, with one exception. They are nullable and cheap, and they record evidence that cannot be reconstructed later.
  - The exception is `received_at`: it duplicates `created_at` unless step 6 defines it as request arrival time. Drop it now or define it then.
  - Accept that the meaning of the unread columns is untested until the step 6 normaliser writes them.
- **Batch reference:** D2 and 3.6. My answer is the `id`.

### 11.4 Test counts compared with the lead's re-run

- `cmd/migrate`: my 149 counts subtests; top-level it is 67 PASS and 1 SKIP, the same as the lead's.
- `internal/migrations`: my 572 counts subtests; top-level 103.
- Handler: my 7 are the tests my `-run` pattern selects. The lead's and implementer's wider pattern also selects the `TestDeleteIssue*` and other workspace-deletion tests (18 top-level in `/tmp/f3-step3/r2/green-handler.txt`). My seven are a subset of those.
