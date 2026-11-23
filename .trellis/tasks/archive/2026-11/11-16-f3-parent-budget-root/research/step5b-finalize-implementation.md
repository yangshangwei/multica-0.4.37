# Step 5b: budget ledger wired into claim finalization (implementation report)

Lane: `f3-binding` follow-up lane for step 5b, round 1 (stages A to D).
Worktree: `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`
(branch `codex/f3-budget-root-20261116`, base `86fea651c`, uncommitted).
Brief: `/tmp/f3-main-s5b/brief-5b.md`.

This file is extended after every stage. The last line is the status line and
is written last; while it is missing, round 1 is still in progress.

## 1. What was built

### Stage A: schema and queries

Done. Migration `866_issue_budget.up.sql` changed in place (no `ALTER`
migration, no new index, no foreign key):

| Table | Change |
| --- | --- |
| `issue_budget_root` | new `attempt_reserve_ticks bigint CHECK (attempt_reserve_ticks > 0)`, nullable |
| `issue_budget_attempt` | `reserved_ticks` loses `NOT NULL` (NULL = unsized, `>= 0` kept); `price_snapshot`, `price_digest`, `issue_budget_attempt_price_pair_check` and `issue_budget_attempt_price_digest_check` removed |
| `issue_budget_entry` | new `price_snapshot bytea CHECK (octet_length BETWEEN 2 AND 65536)` and `price_digest text`; three named constraints: `issue_budget_entry_price_pair_check` (both or neither), `issue_budget_entry_price_digest_check` (digest = SHA-256 of `budget-price-snapshot-v1\n` + snapshot), `issue_budget_entry_price_usage_check` (`kind = 'usage' OR price_snapshot IS NULL`) |

The size rule stays a column-level check, as the step 3 correction in design
section 8 has it for the attempt columns; its catalog name is
`issue_budget_entry_price_snapshot_check`. The three constraint names are
mine: design 10.8 asks for three named constraints and names none.

Queries (`pkg/db/queries/issue_budget.sql`): `InsertIssueBudgetRoot` and
`UpdateIssueBudgetRootPolicy` take `attempt_reserve_ticks`;
`InsertIssueBudgetAttempt` takes a nullable `reserved_ticks` and no price
columns; `InsertIssueBudgetEntry` takes the nullable price pair.

Regeneration: `make sqlc` runs `go run …sqlc@v1.31.1`, which needs the module
proxy for a deprecation lookup (`GOPROXY=off` fails with "module lookup
disabled"). The proxy was not contacted. sqlc v1.31.1 was built from the
module cache (`GOPROXY=off go build ./cmd/sqlc` in
`$GOMODCACHE/github.com/sqlc-dev/sqlc@v1.31.1`) and that binary ran
`sqlc generate` in `server/`. No generated file was edited by hand.

`pkg/protocol`'s capability constant (also listed in design 10.8) belongs to
stage F and is not added in round 1.

"`cost.price_digest` equals the column, guaranteed by the writer, with a
test" (design 10.8) is the usage writer's rule (step 6). Round 1 has no writer
of priced entries; `TestIssueBudgetQueriesRoundTrip` inserts one priced entry
through the generated query and compares the two digests, which pins the
query, not a writer.

### Stage B: management operations

Done (`internal/service/issue_budget.go`).

- **Limit and reserve together (design 10.1).** `IssueBudgetEnableParams`
  gained `AttemptReserveTicks *budget.Ticks`; `SetLimit(ctx, p, limit,
  attemptReserve *budget.Ticks)` takes both, each nullable, and writes both
  as a whole. `issueBudgetAmounts` refuses a negative limit and a reserve
  that is zero or negative with `invalid_request` before any statement is
  sent. `SetLimit` answers `state_conflict` only when both values equal the
  stored ones; the same limit with another reserve (or the reverse) is a
  change and one new policy revision. `Close`, `Reopen` and `SetMode` carry
  the stored reserve through `UpdateIssueBudgetRootPolicy` unchanged.
- **Result.** `IssueBudgetOperationResult` gained `attempt_reserve_ticks`
  (after `limit_ticks`; `null` when nothing is declared). It is part of the
  stored operation result, so a replay returns it.
- **Intent digest.** `issueBudgetIntent` carries `attempt_reserve_ticks`
  right after `limit_ticks`. The domain string is unchanged
  (`budget-operation-intent-v1\n`): nothing was deployed under it. Field
  order is now `kind, workspace_id, scope_kind, scope_id,
  expected_policy_revision, limit_ticks, attempt_reserve_ticks, mode, reason`.
- **Golden vectors.** `TestIssueBudgetIntentDigestGoldenVectors` holds three
  fixed inputs with their digests as literals: a set-limit with both amounts
  (315 input bytes), an enable with neither amount (293 bytes, both `null`),
  and a mode change whose reason contains `&`, `<`, `>`, a double quote and
  a non-ASCII letter (308 bytes). The digests were derived outside Go: the
  byte sequences were built by hand in Python from the documented form and
  hashed with `hashlib` and with `shasum -a 256`
  (`/tmp/f3-s5b/golden/v1.bin` to `v3.bin`; re-derived once more after the
  test was written, same three values). They were not copied from a run of
  the code under test. The first two vectors also carry the input as a
  literal, and the test first checks that the two literals agree with each
  other. The third pins that Go's default HTML escaping is part of the
  digest input; its input is not a literal in the test, because a typed
  backslash-u escape does not survive this lane's tooling (see section 9).
  The test needs no database.
- **`Enable` order (design 9.2 with 10.10).** Refusal checks, binding lock
  (exclusive), in-flight scan, activation sample from the statement clock,
  then root row, member rows, gap observations, operation row.
  `recordIssueBudgetInFlightTasks` is gone; `listIssueBudgetInFlightTasks`
  reads and `recordIssueBudgetInFlightGaps` writes. A refused or replayed
  enable ends before the lock and the scan.
- **`transact` states its level.** Its first statement is
  `SET TRANSACTION ISOLATION LEVEL READ COMMITTED` (constant
  `issueBudgetIsolationSQL`, the text `workflowInTx` sends). Pinned by
  `TestIssueBudgetOperationsRefuseATransactionAtAnotherLevel` (an open
  REPEATABLE READ or SERIALIZABLE transaction as the starter, before and
  after it took its snapshot, each of the five operations: SQLSTATE 25001,
  no statement after the level statement, no budget refusal code, the outer
  transaction still usable and holding no advisory lock, nothing written)
  and `TestIssueBudgetOperationsRunInsideAReadCommittedTransaction` (an open
  READ COMMITTED transaction: all five operations run as savepoints, the
  outer level is unchanged, nothing is visible outside before the outer
  transaction ends, a rollback leaves nothing, a commit leaves everything).
  SERIALIZABLE is my addition to the brief's two cases; it behaves like
  REPEATABLE READ.

Mutation check (each mutant applied to the production file, the budget
service tests run, the file restored and its SHA-256 compared; harness
`/tmp/f3-s5b/mutate.sh`). Every mutant was caught:

| Mutant | Tests that fail |
| --- | --- |
| `transact` sends no level statement | `…TransactionsFollowTheLockOrder`, `…OperationsRefuseATransactionAtAnotherLevel`, `…OperationsRunInsideAReadCommittedTransaction`, `…EnableThatBindsNothingTakesNoBindingLock` |
| activation time sampled before the lock and the scan (step 4 order) | `…TransactionsFollowTheLockOrder` |
| `state_conflict` as soon as the limit alone is unchanged (step 4 rule) | `…SetLimitToTheCurrentLimitIsRefused`, `…LimitAndReserveAreSetTogether` |
| set-limit keeps the stored reserve | `…ManagementReplaysByOriginalKey`, `…SetLimitToTheCurrentLimitIsRefused`, `…LimitAndReserveAreSetTogether` |
| reserve left out of the digest | `…ManagementReplaysByOriginalKey`, `…OriginalKeyStandsForTheWholeIntent`, `…IntentDigestGoldenVectors` |
| reserve in the digest under another field name | `…IntentDigestGoldenVectors` (only the golden vector sees it) |
| `Enable` does not store the reserve | `…LimitAndReserveAreSetTogether` |
| a reserve of zero accepted | `…MalformedRequestsSendNoStatement` |
| close, reopen and mode drop the reserve | `…LimitAndReserveAreSetTogether` |

### Stage C: binding lock, binder side

Done (`internal/service/issue_budget_binding.go`, `issue_budget.go`,
`task.go`, `pkg/db/queries/issue_budget.sql`).

- **The lock.** A per-workspace transaction-level advisory lock with the key
  `hashtextextended('issue_budget_binding:' || <workspace uuid>::text, 0)`.
  The key text is my choice; the design asks only that it is not the
  fence's (`iteration:<workspace>`). Two queries: `LockIssueBudgetBinding`
  (exclusive, waited for) and `TryLockIssueBudgetBindingShared` (shared,
  never waited for). The second has no production caller in round 1: it is
  finalize's side (stage E). It exists now so that the round 1 tests stand a
  second connection in for a claim through the very query finalize will
  send, which proves both sides are one lock.
- **`Enable`** takes the exclusive side after every refusal check and before
  the in-flight scan and the first budget write. A refused or replayed
  enable never takes it.
- **Reparent rule 2** (`BindIssueBudgetOnReparent`, an unbound issue moved
  under a member) takes the exclusive side after the descendant walk and
  before the member rows. The switch read of 10.6 that the design places
  between the two does not exist yet (step 5c). The in-flight scan stays
  where it was, after the member rows, which is after the lock.
- **Child creation** (`BindIssueBudgetOnCreate`), a move of an issue that is
  already a member (rules 1 and 3) and the delete seam take neither side.
- **In-flight scan (design 10.2, supplement of 2026-11-18).** The issue
  branch of `ListIssueBudgetInFlightTasks` joins `issue` and takes its
  workspace condition from the issue row; the agent join is gone from that
  branch. The quick-create branch keeps the agent join and its three
  conditions and the `lower(context->>'parent_issue_id')` expression
  unchanged (10.13), and additionally returns `chat_session_id`,
  `autopilot_run_id` and `context`.
- **One definition of a quick-create task (N3).** `quickCreateTaskContext`
  (`task.go`) is the one definition, on the four columns that decide it;
  `TaskService.parseQuickCreateContext` is now a one-line call of it, and
  `listIssueBudgetInFlightTasks` applies it to every candidate the
  quick-create branch proposes and drops what it rejects. See "Readings" in
  section 3 for why the definition is a Go function and not two more SQL
  conditions.
- **Level assertion in the binding functions (section 5, rule 5).**
  `requireIssueBudgetReadCommitted` reads
  `current_setting('transaction_isolation')` and fails the write with
  `ErrIssueBudgetIsolation` unless it is `read committed`. Create and
  reparent ask it once, right after they found the parent's member row; a
  tree without a root is never asked (its cost stays one lookup); the delete
  seam is never asked. The brief's stage list names only finalize's half of
  this assertion (stage E); the design assigns both halves to 5b, and the
  binder half belongs with the binding lock, so it is built here. Every
  production transaction that reaches the two functions today starts with a
  plain `Begin` (the database default) or declares READ COMMITTED
  (`workflowInTx`): `updateIssueAtomicallyOnce` (`handler/issue.go:3940`),
  `writeLifecycleHandoffInTx` (`lifecycle_handoff_transaction.go:249`),
  `beginTriageWrite` (`triage.go:220`), `IssueService.createOnce`
  (`service/issue.go:278`), `CollaborationService.Create`
  (`collaboration.go:165`) and the workflow transactions behind
  `collaboration_edit.go:124` and `collaboration_stage_actions.go:577`. The
  one REPEATABLE READ read-write transaction in the tree,
  `previewIterationOnce`, creates and moves no issue. Checked by reading
  those call sites, not by a test over all of them.

Tests (new file `internal/service/issue_budget_binding_lock_test.go`, ten
tests; two-connection cases use real connections and read `pg_locks` and
`pg_blocking_pids`, no sleep decides an outcome):

| Test | What it proves |
| --- | --- |
| `…EnableHoldsTheBindingLockUntilItCommits` | before its commit the enabling backend holds exactly the fence and the binding lock, both exclusive; a claim's shared try-lock is refused meanwhile, granted for another workspace, granted again after the commit |
| `…EnableThatBindsNothingTakesNoBindingLock` | eight enables that bind nothing (replay, key conflict, already enabled, member, above a member, parent loop, not eligible, not found) send no lock, no scan and no insert |
| `…EnableWaitsForAClaimThatReadUnbound` | a claim holds the shared side and has read "unbound"; `Enable` waits on that backend with the fence granted and the binding lock not granted, its last statement is the lock; a later claim is turned away while it waits; after the claim commits its task, the scan finds it: one gap with the committed generation |
| `…ReparentWaitsForAClaimThatReadUnbound` | the same for a move of an unbound subtree, including the exact five statements sent before the wait and that no member row exists yet |
| `…OnlyAFirstBindingTakesTheBindingLock` | exact statement lists, written from the design, for create, move inside a root, move across roots, delete (no lock, a claim gets in while the transaction is open) and for two shapes of rule 2 (lock after the walk and before `InsertIssueBudgetMembers`, scan after it; no claim gets in) |
| `…WritersThatBindNoExistingIssueDoNotWaitForABinder` | with the exclusive side held by another connection: child creation through the real `IssueService.CreateInTx`, both member moves, the delete seam and the four management operations all complete |
| `…InFlightScanFindsATaskRunFromAnotherWorkspace` | the `Enable` half of the cross-workspace case: a running task on a member issue whose agent and runtime belong to another workspace is recorded as a gap; the same for a move; a task of that agent on its own workspace's issue is not; the other workspace gets no budget row |
| `…InFlightScanDecidesQuickCreateByTheSharedDefinition` | candidates linked to a chat session or an autopilot run, with a context that does not decode or of another type are no gaps; a plain one and one with an upper-case parent id are |
| `…QuickCreateDefinitionIsShared` | `parseQuickCreateContext` and `quickCreateTaskContext` give the same answer for eight task shapes (no database) |
| `…BindingRefusesATransactionThatIsNotReadCommitted` | from REPEATABLE READ and SERIALIZABLE: create under a member, the three moves under a member are refused with `ErrIssueBudgetIsolation` after exactly two statements; a tree without a root and the delete seam are not asked; through the real create path the issue write fails; nothing is written |

Mutation check, as in stage B. Every mutant was caught:

| Mutant | Tests that fail |
| --- | --- |
| `Enable` takes no binding lock | `…EnableHoldsTheBindingLockUntilItCommits`, `…EnableWaitsForAClaimThatReadUnbound`, `…TransactionsFollowTheLockOrder` |
| `Enable` takes it right after the fence, before the refusal checks | `…EnableThatBindsNothingTakesNoBindingLock`, `…TransactionsFollowTheLockOrder` |
| `Enable` scans before it has the lock | `…EnableWaitsForAClaimThatReadUnbound` (the committed claim is missed), `…TransactionsFollowTheLockOrder` |
| reparent rule 2 takes no lock | `…ReparentWaitsForAClaimThatReadUnbound`, `…OnlyAFirstBindingTakesTheBindingLock`, `…BindingNeverTouchesARootRow` |
| reparent rule 2 takes it after the member rows | `…ReparentWaitsForAClaimThatReadUnbound`, `…OnlyAFirstBindingTakesTheBindingLock` |
| child creation takes the lock | `…OnlyAFirstBindingTakesTheBindingLock`, `…WritersThatBindNoExistingIssueDoNotWaitForABinder` |
| the binder's lock uses the fence's key | five tests, among them both waiting tests |
| the binder asks for the shared side | the same five |
| the scan keeps every quick-create candidate | `…InFlightScanDecidesQuickCreateByTheSharedDefinition` |
| the issue branch asks for the agent's workspace again | `…InFlightScanFindsATaskRunFromAnotherWorkspace` |
| create does not ask for its level | `…OnlyAFirstBindingTakesTheBindingLock`, `…BindingRefusesATransactionThatIsNotReadCommitted` |
| reparent does not ask for its level | the same two and `…ReparentWaitsForAClaimThatReadUnbound` |
| create asks before it knows there is a member row | `…BindingNeverTouchesARootRow` (a rootless tree pays a second statement), and two more |

A fault of my own tests, found by the first mutation run and fixed: a test
that opened a transaction and then failed a check left the connection out of
the pool, and the fixture's `pool.Close` cleanup then waited for it until
the 10 minute test timeout. Every transaction the new tests open now has its
rollback registered before the first check, and the two tests that start a
writer on a second goroutine wait for it in a cleanup.

### Stage D: ledger load and cache

Done (new file `internal/service/issue_budget_ledger.go`; two queries in
`pkg/db/queries/issue_budget.sql`). At the end of round 1 nothing in
production called it; its first caller is finalize (stage E).
`internal/budget` was not touched and no accessor was added to it. The
review of round 1 changed two things here, and each has its own part below:
"Round 2, change 1.1" and "Round 2, change 1.3". The bullets and the two
tables of this section describe the code as it is after both.

- **The cache (design 10.4).** `issueBudgetLedgerCache`, one per process
  (`issueBudgetLedgers`): root id to a revision and an immutable
  `*budget.Ledger`, a list in order of use, and the sum of the weights. A
  root weighs its entries plus `issueBudgetLedgerRootWeight` (2); the limit
  is `issueBudgetLedgerCacheLimit` (50,000 units of weight). The cache
  stores and swaps pointers only. `publish` replaces a state only by one of
  a higher revision, moves the root to the front and removes roots from the
  back until the sum fits. A state whose own weight exceeds the limit is not
  kept, and an older state of that root is removed with it.
- **Two entry points, one load.** `loadUnderRootLock(ctx, q, root)` is for a
  writer whose `root` is the row `LockIssueBudgetRoot` returned in its
  transaction; `loadWithoutRootLock(ctx, q, root)` is for a reader at any
  level (step 7). The load asks for no lock and no isolation level itself.
  It takes the cached state when that is not ahead of the root row, reads
  the entries behind it with one query (`ListIssueBudgetEntryFormsAfter`:
  `seq`, `schema_version`, `entry_json` after a seq, in seq order; a full
  replay asks for the entries after 0, and a cached state at the root's
  revision reads no row), decodes each row with `budget.DecodeEntry(row
  version, bytes)` and applies all of them with one `With` call.
- **A cached state ahead of the root row.** Under the root lock it is
  removed and the whole ledger is read (the database was put back); since
  round 2 the removal is logged (change 1.1 below). Without the lock it is
  left exactly as it is, the reader replays from its own rows, and rows
  beyond its root row's revision are left out as later commits.
- **Checks on both paths.** The rows must be numbered from the base revision
  plus one to the root's revision without a gap, and end there; the replayed
  ledger's `Len()` must equal the root's revision; its `Projection()` must
  equal the six figures on the root row (`actual_known_ticks`,
  `estimated_ticks`, `reserved_ticks`, `unknown_hold_ticks`,
  `overrun_ticks`, `unknown_count`). A cached state at the root's revision
  is compared with the root row as well.
- **Ledger unavailable (design 10.5).** Every failed check returns an
  `*issueBudgetLedgerUnavailableError{RootID, Revision, Cause, Detail}`,
  which `errors.Is` reports as `ErrIssueBudgetLedgerUnavailable`. `Cause` is
  one of `stored_form` (`DecodeEntry` refused the row, an unknown form
  version included), `replay` (`With` refused the entries, or a row repeats
  an entry), `sequence`, `projection`. The metering package's error is kept
  as text in `Detail`, so a caller cannot take the failure for a refused
  entry. Since round 2 only a replay of every row returns this error: a
  cached state that fails a check is dropped (by R1 in section 3.1) and the
  rows are read in full within the same load (change 1.1 below). A failed
  read of the rows is an ordinary error and drops nothing.
- **Publish moments.** (1) `loadUnderRootLock` publishes what it loaded.
  (2) `issueBudgetLedgerAppend.Committed()` publishes the appended state;
  the caller calls it after its commit is confirmed and not otherwise.
  Nothing else publishes; a reader publishes nothing (R2).
- **Too large to cache.** A loaded ledger whose weight exceeds the limit is
  returned and not kept; each load of it logs one warning with the root id,
  the entry count and the limit.
- **The append.** `appendEntries(ctx, q, root, ledger, entries)` encodes
  every entry with `budget.EncodeEntry` first, applies the whole batch with
  one `With` call, and only then sends statements: one
  `InsertIssueBudgetEntry` for each entry the ledger did not hold (a
  replayed entry gets no row), with `schema_version =
  budget.EntryFormVersion`, the encoded bytes and digest, the receipt, and
  `seq` counting on from the root's revision; then
  `UpdateIssueBudgetRootLedger`, which sets the revision and the six
  figures and is guarded by `ledger_revision = <the revision the append
  started from>`. The new `Len()` is compared with the new revision before
  the first statement. A batch of replays only sends nothing. One refused
  entry means nothing is written and nothing is published.
- **One constructor for an entry row (design section 8).**
  `issueBudgetEntryRow(entry)` derives from the entry its identity, its
  kind, the attempt, the invocation and the scope, the stored form, its
  digest and the form version (`entry_id`, `kind`, `attempt_id`,
  `invocation_id`, `scope_id`, `entry_json`, `entry_digest`,
  `schema_version`). The table has no provider or model column and the
  function derives none; round 1 of this report named them by mistake.
  `issueBudgetLedgerEntry` adds only `Source` and `OperationID` (R3).

Tests (new file `internal/service/issue_budget_ledger_test.go`; 15 tests, 56
with their subtests, at the end of round 1; 18 and 77 after change 1.1; 20
and 84 after change 1.3). The entry rows are written by the tests through
`EncodeEntry` and the insert query, as the brief asks; five of the round 1
tests and one of the two added for change 1.3 need no database.

| Test | What it proves |
| --- | --- |
| `…LedgerLoadsTheWholeLedgerOnceAndThenTheTail` | an empty cache reads every row once (`… after 0`); the next load reads none; after more rows only the tail is read (`… after <revision>`); the exact statements are compared, and the figures after each step are literals |
| `…LedgerIsUnavailableForEachCause` | 17 damages, each on a full replay and on a tail load: four stored forms (unknown version, white space behind the form, a form without most of its keys, bytes that are no entry), three replay errors (a hold on a reservation no entry opened, an identity used again with other content, a row that repeats an entry), four sequence faults (a seq missing, entries one seq late, entries ending before the root's revision, an entry beyond it), and the root row off by one in each of the six figures. Each gives the named cause on a full replay and on a load that starts from a cached state. The load from a cached state reads the tail, drops the state with one warning (cached revision 3, the root row's revision, the cause) and reads every row within the same load (`… after 3`, then `… after 0`); the full replay and a second load of the cache that lost its state read once and log nothing; nothing is left cached; a reader without the lock is told the same and drops the state it used, except for the entry beyond the root's revision, which to a reader is a later commit: that load succeeds from the tail and keeps its state; (rewritten in round 2, section 7) no load writes anything; the state handed out before the damage is unchanged; after a repair the ledger loads and is cached again |
| `…LedgerMustStartAtTheFirstSeq` | a ledger whose first row is not seq 1 is unavailable |
| `…LedgerCacheKeepsWithinItsLimit` | the limit is a sum of weights with 2 for each root; the least recently used roots make room, and a lookup and a replacement count as a use; an older or equal revision replaces nothing; a state that alone exceeds the limit is not kept and takes the root's older state with it; `forget` with a pointer removes only that state (no database) |
| `…LedgerTooLargeToCacheIsReplayedOnEveryLoad` | a root one unit over a small limit is read in full on each load, by a writer and by a reader, with a warning carrying the root id, the entry count and the limit each time, and is not cached; one that just fits is cached and warns nothing; an append that takes a cached root over the limit removes its state when it is published |
| `…LedgerUnderTheRootLockDropsAStateAheadOfTheDatabase` | a writer that finds the cache ahead of its locked root row removes that state, logs one warning with the cached revision 4, the root row's revision 2 and the cause `ahead_of_root`, reads every row and publishes the result (rewritten in round 2, section 7) |
| `…LedgerReaderWithoutTheRootLockPublishesNothing` | a reader with an empty cache; a REPEATABLE READ reader whose snapshot is older than the cache (the cache is neither used nor changed, the answer is its snapshot's); a READ COMMITTED reader that sees rows committed after its root row (left out); a reader with an older cached state (reads the tail). In none of them the cache changes |
| `…LedgerFailedReaderDropsOnlyTheStateItUsed` | three readers on damaged rows, each told the ledger is unavailable by its full replay: one behind the cache uses no cached state, leaves the newer state alone and logs nothing; one that went on from the cached state drops it with one warning and reads every row within the same load, and the next reader reads once and logs nothing; one overtaken by a writer between its two reads drops nothing and logs nothing, and what the writer published stays (rewritten in round 2, section 7) |
| `…LedgerCachedStateThatDisagreesGivesWayToTheRows` (round 2) | a cached state of three entries against rows that say something else, in three cases: the same revision with other content (cause `projection`; one read of every row), rows that the cached state cannot be continued with (cause `replay`; the tail, then every row), rows that continue it to figures the root row does not carry (cause `projection`; the tail, then every row). Each under the root lock and for a reader: the load succeeds with the ledger the rows replay to, logs one warning with the cause of the failed check, and the lock holder publishes the result while the reader publishes nothing. With a state published while the tail was read, the lock holder drops that one too; a reader overtaken by a writer drops nothing and logs nothing (the one warning there is the writer's, which dropped the state first), and what the writer published stays. Loads that agree with the cache log nothing |
| `…LedgerUnavailableIsDecidedByTheFullReplay` (round 2) | rows that a cached state cannot be continued with and that are damaged themselves: the error carries the cause the full replay found (`projection`), the warning the cause of the check that failed first (`replay`); a state published during the full replay is dropped by the lock holder and kept by the reader |
| `…LedgerFailedReadIsNotAnUnavailableLedger` (round 2) | a read that fails (a context cancelled between the statements) is the error of the read and not an unavailable ledger: with the tail read the cached state stays and nothing is logged, for a writer and a reader, and the next load reads the tail; with the read of every row a state published meanwhile stays; after a cached state was dropped the failed full read leaves the root uncached with the one warning of the drop, and the next load reads every row once |
| `…LedgerAppendWritesRowsAndIsPublishedOnceCommitted` | a rolled-back append leaves the database and the cache as they were, and the next load works from the cache; a committed batch of a new entry, a replay and a new entry writes two rows whose every column is compared with a literal, moves the root row, and reaches the cache only at `Committed()`; a batch of replays sends no statement |
| `…LedgerAppendThatIsRefusedWritesNothing` | eight refusals behind a good entry (an identity of 201 bytes, fields of another kind, a reservation in upper case, one that is no attempt id, a hold on an unknown reservation, an identity with other content, a ledger that is not the root's, and since change 1.3 a stored form of 2,049 bytes) send no statement; a stale root row fails at the guarded update |
| `…LedgerEntryFormIsCappedWhereItIsWritten` (round 2) | the limit is 2,048 bytes; an entry one byte over is refused as an invalid entry, with its ID, its length and the limit in the error, no statement, and cache and rows as before; an entry of exactly the limit is written and read back whole by a writer and by a reader; a row of 65,536 bytes that the test inserted is loaded under the root lock and by a reader; behind such a row the ledger goes on: the long entry given again is refused, a reservation behind it is appended, and a full replay gives both |
| `…LedgerEntryLimitHoldsTheLargestUsageEntry` (round 2) | the largest usage entry a version 1 report can produce (a 200-byte identity whose model is 120 times `<`, the largest int64 in the four token counts and in an estimated cost, a 64-byte price version, a 64-digit digest) has a stored form within the limit, and a ledger takes it; the length is logged, not pinned: 1,708 bytes (no database) |
| `TestIssueBudgetEntryRowIsDerivedFromTheEntry` | eight accepted entries with their columns as literals, the 200-byte usage identity among them; nine refused (no database) |
| `…LedgerReplayCostGrowsWithTheRowsRead` | what a replay allocates for 800 rows is less than six times that for 200: one `With` call, not one for each row (no database) |
| `…LedgerCacheIsSafeForConcurrentUse` | lookups, publications and removals from many goroutines; for the race detector (no database) |
| `…LedgerConcurrentWritersAndReadersAgree` | four writers appending six times each under the root lock, two readers without it, on real connections: no load fails, and the cache ends with the ledger the rows replay to |
| `…LedgerStatesAreNeverAssignedThrough` | no non-test file of the package that imports the metering package assigns through a pointer (a syntax scan) |

`BenchmarkIssueBudgetLedgerCacheMemory` in the same file is the measurement
of section 6. It is a benchmark, not a test, and no default run executes it.

Mutation check, as in stages B and C: mutants of `issue_budget_ledger.go`
and of the two generated queries, each applied, the tests of the file run,
the file restored and its SHA-256 compared. Round 1 ran 54 and every one was
caught. After change 1.1 ten of them no longer fitted the rewritten `load`,
`lookup` and `forget` and were expressed again on the new code (marked "again"
below), 22 were added for the new rule (they are in the part on change 1.1),
and all 76 were run against the 18 tests: every one was caught (log
`/tmp/f3-s5b/logs/d-mutants-r2.txt`). The rows below give the tests that
fail in that second run; test names are short for the ones in the table
above. After change 1.3 ten more were added and all 86 were run a third
time against the 20 tests, and every one was caught (log
`/tmp/f3-s5b/logs/d-mutants-r3.txt`); the part on change 1.3 says where the
lists of the third run differ from the rows below.

| Mutant | Tests that fail |
| --- | --- |
| **Cache** | |
| the limit is another value (500,000) | `…CacheKeepsWithinItsLimit` |
| a root weighs nothing beyond its entries | six tests, among them `…CacheKeepsWithinItsLimit` and `…TooLargeToCache…` |
| eviction from the most recently used end | `…CacheKeepsWithinItsLimit` |
| eviction already at the limit (`>=`) | `…CacheKeepsWithinItsLimit`, `…TooLargeToCache…` |
| a state of exactly the limit is refused | `…CacheKeepsWithinItsLimit`, `…TooLargeToCache…` |
| a lookup is not a use (again) | `…CacheKeepsWithinItsLimit` |
| a replacement is not a use | `…CacheKeepsWithinItsLimit` |
| a replacement keeps the old weight | `…LoadsTheWholeLedger…`, `…CacheKeepsWithinItsLimit`, `…AppendWritesRows…`, `…CacheIsSafeForConcurrentUse` |
| an older revision replaces a newer one | `…CacheKeepsWithinItsLimit` |
| the same revision replaces | `…CacheKeepsWithinItsLimit` |
| a root too large keeps its older state | `…CacheKeepsWithinItsLimit`, `…TooLargeToCache…` |
| `forget` removes whatever pointer is there (again) | `…CacheKeepsWithinItsLimit`, `…FailedReader…`, `…CachedStateThatDisagrees…` |
| **Load** | |
| a writer's load is not published (again) | seven tests |
| a reader's load is published | `…IsUnavailableForEachCause`, `…ReaderWithoutTheRootLock…`, `…CachedStateThatDisagrees…` |
| a state ahead of the row is handed out (again) | `…CacheKeepsWithinItsLimit`, `…UnderTheRootLockDropsAStateAhead…`, `…ReaderWithoutTheRootLock…`, `…FailedReader…`, `…CachedStateThatDisagrees…` (`…ConcurrentWritersAndReadersAgree` caught it as well in round 1 and in the third run, and not in the second: whether it does depends on how its goroutines interleave) |
| the lock holder keeps a state ahead of its row (again) | `…CacheKeepsWithinItsLimit`, `…UnderTheRootLockDropsAStateAhead…` |
| a reader removes a state ahead of its row (again) | `…CacheKeepsWithinItsLimit`, `…ReaderWithoutTheRootLock…`, `…FailedReader…`, `…CachedStateThatDisagrees…` |
| rows are read when the cache is current | `…LoadsTheWholeLedger…`, `…ReaderWithoutTheRootLock…`, `…CachedStateThatDisagrees…`, `…AppendWritesRows…`, `…AppendThatIsRefused…` |
| the tail load reads from seq 0 | `…LoadsTheWholeLedger…`, `…IsUnavailableForEachCause`, `…ReaderWithoutTheRootLock…`, `…FailedReader…`, `…CachedStateThatDisagrees…` |
| the query reads `seq >= n` | the same five and `…UnavailableIsDecidedByTheFullReplay` |
| the query reads any root's entries | `…IsUnavailableForEachCause`, `…TooLargeToCache…`, `…FailedReader…`, `…CachedStateThatDisagrees…` |
| the row's form version is ignored | `…IsUnavailableForEachCause`, `…FailedReader…` |
| one `With` call for each row | `…ReplayCostGrowsWithTheRowsRead` (only this one) |
| no warning for a root too large | `…TooLargeToCache…` |
| the warning already at the limit (`>=`) | `…TooLargeToCache…` |
| **Checks and the unavailable path** | |
| no seq check | `…IsUnavailableForEachCause`, `…MustStartAtTheFirstSeq` |
| no check that the rows end at the root's revision | `…IsUnavailableForEachCause` |
| no check of `Len()` against the revision | `…IsUnavailableForEachCause` |
| no projection check | `…IsUnavailableForEachCause`, `…FailedReader…`, `…CachedStateThatDisagrees…`, `…UnavailableIsDecidedByTheFullReplay` |
| estimated and reserved swapped on both sides of the comparison (it still holds, and an append writes them swapped) | `…TooLargeToCache…`, `…AppendWritesRows…` |
| a reader does not leave out later rows | `…IsUnavailableForEachCause`, `…ReaderWithoutTheRootLock…`, `…ConcurrentWriters…` (the race really happens there) |
| a writer leaves out rows beyond its row | `…IsUnavailableForEachCause` |
| after a full replay that failed, the lock holder leaves the cache as it is (again; in round 1: a failed writer's load drops nothing) | `…UnavailableIsDecidedByTheFullReplay` (only this one) |
| a reader whose cached state failed a check drops nothing (again) | `…IsUnavailableForEachCause`, `…FailedReader…`, `…CachedStateThatDisagrees…`, `…UnavailableIsDecidedByTheFullReplay` |
| a reader whose cached state failed a check drops whatever is cached by then (again) | `…FailedReader…`, `…CachedStateThatDisagrees…` |
| a reader whose full replay failed drops whatever is cached (again) | `…FailedReader…`, `…UnavailableIsDecidedByTheFullReplay` |
| the error is not `ErrIssueBudgetLedgerUnavailable` | `…IsUnavailableForEachCause`, `…MustStartAtTheFirstSeq`, `…FailedReader…`, `…UnavailableIsDecidedByTheFullReplay` |
| the error also matches the metering package's stored-form error | the same four |
| **Append** | |
| entries are encoded one by one between the inserts | `…AppendThatIsRefused…` (only this one) |
| no `Len()` check before the first statement | `…AppendThatIsRefused…` (only this one) |
| the root update without its revision guard | `…AppendThatIsRefused…` (only this one) |
| a replayed entry gets a row | `…AppendWritesRows…` |
| the revision counts replays | `…AppendWritesRows…` |
| the receipt column is empty | `…AppendWritesRows…` |
| `source` is fixed | `…AppendWritesRows…` |
| the operation id is dropped | `…AppendWritesRows…` |
| the root row gets the figures of the old ledger | `…TooLargeToCache…`, `…AppendWritesRows…`, `…ConcurrentWriters…` |
| the append returns the old ledger | the same three |
| the append publishes before the commit | `…AppendWritesRows…` |
| `Committed()` publishes nothing | `…TooLargeToCache…`, `…AppendWritesRows…`, `…ConcurrentWriters…` |
| **Entry row** | |
| a gap that names an attempt gets no `attempt_id` | `TestIssueBudgetEntryRowIsDerivedFromTheEntry` |
| a reservation that is not canonical is accepted | `…AppendThatIsRefused…`, `TestIssueBudgetEntryRow…` |
| the usage columns are dropped | `…AppendWritesRows…`, `…AppendThatIsRefused…`, `TestIssueBudgetEntryRow…` |
| `scope_id` is the invocation id | `…AppendWritesRows…`, `TestIssueBudgetEntryRow…` |

One mutant was considered and not added, because no test can tell it from
the original: the list query without `ORDER BY seq` (the unique index hands
the rows out in seq order either way; the order is pinned by the step 3 test
`TestIssueBudgetLedgerLoadIsOrderedByItsKey` on the other query, and by the
seq check on this one once a plan returns another order). Round 1 left out a
second one for the same reason, a lock holder that forgets only the pointer
it used; since change 1.1 a test can tell it (a state published while the
tail is read), and it is one of the 22 new mutants.

One mutant (`the revision counts replays`) did not compile in its first
form and was rewritten and run again; the counts are of mutants that built.

#### Round 2, change 1.1: only a full replay decides "unavailable"

Built test-first: the three tests that pinned the round 1 behaviour were
rewritten and two were added, the run was red for the reasons the rule
predicts (five tests: one read only, unavailable at once, no warning; log
`r2-11-red`), then `load` was changed and the run was green. A third test
and one more subtest were added while the mutants were written, for two
things no test told apart yet (see the last two rows of the list below).

What the load does now (`load`, `lookup`, `forget`, `warnDropped`, `loaded`
and `replayIssueBudgetLedgerFrom` in `issue_budget_ledger.go`):

- A load that finds a cached state not ahead of its root row reads the tail
  (no statement when the state is at the root's revision) and runs the
  checks. If they hold, that is the result, as in round 1.
- If a check fails, the state is dropped by R1: under the root lock whatever
  is cached for the root goes; a reader drops the state it used, and only if
  that is still the one cached. One warning is logged, with four fields:
  `budget_root_id`, `cached_revision` (of the state the load started from),
  `root_revision` (of the root row the load was given) and `cause` (of the
  check that failed: `stored_form`, `replay`, `sequence` or `projection`).
  Then the same load reads every row (`… after 0`) and replays them on an
  empty ledger.
- Only that full replay returns `ErrIssueBudgetLedgerUnavailable`, with the
  cause it found itself, which need not be the cause in the warning. After
  it failed, the lock holder leaves nothing cached for the root (a state
  published while it read included); a reader drops nothing more.
- A full replay that succeeds is an ordinary result: published under the
  root lock, returned and not published for a reader (R2 as before).
- A cached state ahead of the root row: the lock holder drops it and logs
  the same warning with the cause `ahead_of_root`
  (`issueBudgetLedgerAheadOfRoot`; it is a cause of a dropped state only and
  never of an unavailable ledger). A reader that meets one drops nothing and
  logs nothing.
- A read that fails (the connection, a cancelled context) is returned as it
  is, is not an unavailable ledger, drops nothing and logs nothing, on the
  tail read and on the full read alike.

Three points the rule leaves to the code, and what was built:

- **The warning is logged only when a state was dropped.** A reader whose
  state was replaced or evicted before the drop logs nothing: the state that
  failed is gone already and the one in the cache was not looked at. So
  there is one warning for each dropped state, not one for each load.
- **Under the root lock the warning names the state the load started
  from**, also when another state had taken its place by the time of the
  drop (a `Committed()` that arrived while the tail was read). The lock
  holder drops that other state as well; R1 says "whatever is cached".
- **Statements.** A failing load from a cached state sends at most two
  reads, the tail and every row; one when the cached state was at the root's
  revision. A root at revision 0 is loaded without a read, so a root row at
  revision 0 with figures other than zero is unavailable with no statement
  sent.

Tests: `…IsUnavailableForEachCause`,
`…UnderTheRootLockDropsAStateAheadOfTheDatabase` and
`…FailedReaderDropsOnlyTheStateItUsed` were rewritten to the new rule
(section 7); `…CachedStateThatDisagreesGivesWayToTheRows`,
`…UnavailableIsDecidedByTheFullReplay` and
`…FailedReadIsNotAnUnavailableLedger` are new (rows in the table above). The
warnings are read from the cache's own logger, set by the test; the
statements of each load are compared as an exact list. After the change:
18 tests, 77 with their subtests, all passing (log `r2-11-green2`).

The 22 mutants added for this change, all caught:

| Mutant | Tests that fail |
| --- | --- |
| no second read: a cached state that fails a check makes the ledger unavailable at once (the round 1 behaviour) | `…IsUnavailableForEachCause`, `…FailedReader…`, `…CachedStateThatDisagrees…` |
| the second read starts from the cached state again | the same three |
| the second read asks for the rows after the cached revision and replays them on an empty ledger | the same three |
| the state that failed is published again | `…CachedStateThatDisagrees…`, `…UnavailableIsDecidedByTheFullReplay`, `…FailedReadIsNotAnUnavailableLedger` |
| the state that failed is not dropped | `…IsUnavailableForEachCause`, `…FailedReader…`, `…CachedStateThatDisagrees…`, `…UnavailableIsDecidedByTheFullReplay` |
| a reader publishes what its second read gave | `…CachedStateThatDisagrees…` (only this one) |
| a reader publishes every full replay | `…ReaderWithoutTheRootLock…`, `…CachedStateThatDisagrees…` |
| no warning about a dropped state | `…IsUnavailableForEachCause`, `…FailedReader…`, `…CachedStateThatDisagrees…` |
| the warning is logged although nothing was dropped | `…FailedReader…`, `…CachedStateThatDisagrees…` |
| cached and root revision swapped in the warning | `…IsUnavailableForEachCause`, `…FailedReader…`, `…CachedStateThatDisagrees…`, `…UnavailableIsDecidedByTheFullReplay` |
| the warning carries a fixed cause | those four and `…FailedReadIsNotAnUnavailableLedger` |
| the warning at info level | `…IsUnavailableForEachCause`, `…UnderTheRootLockDropsAStateAhead…`, `…FailedReader…`, `…CachedStateThatDisagrees…` |
| the warning without the root id | the same four |
| the error carries the cause of the check that failed first, not the full replay's | `…UnavailableIsDecidedByTheFullReplay` (only this one) |
| a state ahead of the root row is dropped without a warning | `…UnderTheRootLockDropsAStateAhead…` |
| that warning with the cause of a load check (`sequence`) | `…UnderTheRootLockDropsAStateAhead…` |
| that warning with the two revisions swapped | `…UnderTheRootLockDropsAStateAhead…` |
| a reader that meets a state ahead of its root row logs the warning | `…FailedReader…`, `…CachedStateThatDisagrees…` |
| a failed read of the tail drops the cached state | `…FailedReadIsNotAnUnavailableLedger` (only this one) |
| a failed read of every row drops what is cached | `…FailedReadIsNotAnUnavailableLedger` (only this one) |
| a failed read counts as a failed check (drop, warning, second read) | `…FailedReadIsNotAnUnavailableLedger` (only this one) |
| the lock holder drops only the state it used | `…CachedStateThatDisagrees…`, by its subtest with a state published while the tail is read (only this one) |

#### Round 2, change 1.3: an entry a server writes is at most 2,048 bytes

The ruling on R4 (round 2 file, 1.3): the weight of a root stays its entries
plus 2, and the bytes are bounded where entries are written.

What was built (`issue_budget_ledger.go`):

- `issueBudgetLedgerEntryMaxBytes = 2048`, beside the cache limit and the
  root weight, with a comment that says what it protects (the cache limit as
  a bound on memory) and that a load does not apply it.
- `appendEntries` refuses an entry whose stored form is longer. The check is
  in the first loop, right behind `issueBudgetEntryRow`, so it runs for every
  entry before the ledger is asked and before a statement is sent. The error
  wraps `budget.ErrInvalidEntry` and reads `budget: invalid ledger entry:
  budget entry "<id>": a stored form of 2049 bytes, the limit for an entry a
  server writes is 2048`. It is not `ErrIssueBudgetLedgerUnavailable`; the
  append returns its zero value, and the cache and the rows are as before.
- `load`, `replayIssueBudgetLedger`, `budget.MaxStoredEntryBytes` and the
  table's check on `entry_json` (2 to 65,536 bytes) are unchanged. A row of
  65,536 bytes is loaded, cached by its weight of one unit and replayed like
  any other.

One point the ruling leaves to the code, also in section 3.3: **the cap
applies to every entry given, also to one the ledger already holds.** An
entry that is in the ledger gets no row (a replay). `appendEntries` still
refuses it when its form is over the limit, so a row of 65,536 bytes that an
operator wrote cannot be handed back to a server as a replay; entries behind
such a row are appended as usual. The ruling's words are "refuses an entry
whose stored form is longer"; the other reading (only an entry that would get
a row) is the mutant `the cap only for entries that get a row` below.

Tests, written before the code. The run was red for the reason the rule
predicts (log `r2-13-red`: `…AppendThatIsRefusedWritesNothing` and
`…EntryFormIsCappedWhereItIsWritten` failed, an entry of 2,049 bytes and the
65,536-byte entry given again were both accepted). One more failure of that
run was a fault of the new test, a writer and a reader sharing one cache so
that the reader sent no statement; it was corrected before the green run.
After the change: 20 tests, 84 with their subtests, all passing (log
`r2-13-green`).

- `…AppendThatIsRefusedWritesNothing` got an eighth refusal: a gap entry
  padded to 2,049 bytes behind a valid entry. As for the other seven: the
  error and its text, no statement, cache and rows unchanged.
- `TestIssueBudgetLedgerEntryFormIsCappedWhereItIsWritten` (database) pins
  the constant to 2,048 and has four parts: one byte over is refused with
  nothing sent (the error is an invalid entry and not an unavailable ledger,
  and names the ID, the length and the limit); exactly the limit is written
  (statements `InsertIssueBudgetEntry`, `UpdateIssueBudgetRootLedger`; the
  row is 2,048 bytes and equal to `EncodeEntry`) and read back by a fresh
  cache as a writer and as a reader, with the detail whole in the unknown
  item; a load reads a row of 65,536 bytes that the test inserted, under the
  root lock and as a reader; the ledger goes on behind that row (the long
  entry given again is refused, a reservation behind it is appended, and a
  full replay gives both entries). No warning is logged in the whole test.
- `TestIssueBudgetLedgerEntryLimitHoldsTheLargestUsageEntry` (no database)
  is the test the round 2 file asks for beside the constant: the usage entry
  `usage/<attempt>/<invocation>/<model>` with a model of 120 times `<`, on
  the attempt's reservation, scope `<attempt>/<invocation>`, mode delta, the
  largest int64 in the four token counts, an estimated cost of the largest
  int64 in USD, a price version of 64 bytes and a digest of 64 hex digits.
  It logs the length: **1,708 bytes, and 1,108 with a model of letters, on
  go1.26.9 and on go1.27.0** (logs `r2-13-largest-go1269`, `…-go1270`), the
  figures of the round 2 file. It asserts that the form is within the limit
  and that a ledger with the attempt's reservation takes the entry. Its
  comment says that it is the test that fails when a later change of the
  stored form or of the limits of design 11.5 pushes a valid entry over the
  limit.

Mutants. Ten were added and one of round 1 (`entries are encoded one by one
between the inserts`) was expressed again, because its text spans the changed
loop. All 86 stage D mutants were then run again on the final code and all
were caught (log `/tmp/f3-s5b/logs/d-mutants-r3.txt`, `mutants exit=0`; the
two ledger files have the same SHA-256 after the run as before it).

| Mutant | Tests that fail |
| --- | --- |
| no cap | `…AppendThatIsRefused…`, `…EntryFormIsCapped…` |
| the cap off by one, downwards (`>=`: exactly 2,048 is refused) | `…EntryFormIsCapped…` (only this one) |
| the cap off by one, upwards (2,049 is written) | `…AppendThatIsRefused…`, `…EntryFormIsCapped…` |
| the constant is 1,024 | those two and `…EntryLimitHoldsTheLargestUsageEntry` |
| the load applies the cap (a longer row is a `stored_form` failure) | `…EntryFormIsCapped…` (only this one) |
| the refusal wraps `ErrConflict` instead of `ErrInvalidEntry` | `…AppendThatIsRefused…`, `…EntryFormIsCapped…` |
| the refusal also wraps `ErrIssueBudgetLedgerUnavailable` | the same two |
| the refusal does not name the entry | the same two |
| the cap only for entries that get a row | `…EntryFormIsCapped…`, by its part on the ledger behind a longer row (only this one) |
| the cap between the inserts, so that an entry before the long one is sent | `…AppendThatIsRefused…`, `…EntryFormIsCapped…` |
| (again) entries are encoded one by one between the inserts | the same two (in the first and the second run: the first only) |

Where the third run differs from the rows of the table further up. Six of
the earlier mutants are now caught by `…EntryFormIsCappedWhereItIsWritten` as
well, because it appends, publishes and reads back: `rows are read when the
cache is current`, `estimated and reserved swapped`, `entries are encoded one
by one between the inserts`, `the root row gets the figures of the old
ledger`, `the append returns the old ledger` and `Committed() publishes
nothing`. One list differs without a change of the code it mutates: `a state
ahead of the row is handed out` was also caught by
`…ConcurrentWritersAndReadersAgree` in round 1 and in the third run, and not
in the second. That test catches it only when its goroutines interleave
so; the five other tests that catch the mutant do not depend on timing. The
logs keep the names of the failing tests and not their messages, so how the
concurrent test fails under that mutant was not looked at.

The benchmark got its cells at the limit; the figures and what they bound
are in section 6.1.

### Stages E to G

round 2

### Files changed

All under `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server/`.
Fourteen files. Ten were in the worktree as uncommitted work of steps 3 and
4 and were changed in place; `internal/service/task.go` is a committed file
whose only difference from `HEAD` is this lane's (11 lines added, 3
removed); three are new. Nothing was committed.

| File | Stage | What |
| --- | --- | --- |
| `migrations/866_issue_budget.up.sql` | A | the three table changes of design 10.8, in place |
| `pkg/db/queries/issue_budget.sql` | A, C, D | four statements with the changed columns; `LockIssueBudgetBinding`, `TryLockIssueBudgetBindingShared`, `GetIssueBudgetTransactionIsolation`; the issue branch of `ListIssueBudgetInFlightTasks`; `ListIssueBudgetEntryFormsAfter`, `UpdateIssueBudgetRootLedger` |
| `pkg/db/generated/issue_budget.sql.go` | A, C, D | generated |
| `pkg/db/generated/models.go` | A | generated (the three model structs) |
| `internal/service/issue_budget.go` | B, C | limit and reserve together, the digest field, the result field, `transact`'s level statement, the order of `Enable` with the binding lock |
| `internal/service/issue_budget_binding.go` | C | the lock in reparent rule 2, the level assertion, the scan split into read and write, the shared quick-create decision |
| `internal/service/task.go` | C | `quickCreateTaskContext`; `parseQuickCreateContext` calls it |
| `internal/service/issue_budget_ledger.go` | D | new: cache, load, append |
| `internal/service/issue_budget_test.go` | B, C | section 7; four new tests |
| `internal/service/issue_budget_binding_lock_test.go` | C | new: ten tests |
| `internal/service/issue_budget_ledger_test.go` | D | new: 15 tests and the memory benchmark |
| `internal/handler/issue_budget_schema_test.go` | A | section 7 |
| `internal/handler/issue_budget_queries_test.go` | A | section 7 |
| `internal/migrations/issue_budget_test.go` | A | section 7 |

`sqlc generate` rewrote the 143 files it owns under `pkg/db/generated/` (the
directory holds 147; four are hand-written); only the two listed differ in
content from what was there (`git status` shows no other).
The mutation harness wrote the three production files and the generated
query file many times and restored each by SHA-256; the last state of each
is the unmutated one, which the verification runs of section 8 start from.

Not changed: `internal/budget/**` (no accessor was needed),
`internal/daemon/**`, `pkg/protocol`, every migration but 866's `up`, the
handlers, `cmd/migrate`, and `internal/service/issue_budget_writers_test.go`.

## 2. Exit rows to tests

Finalize rows (B07, B08, B34, B35, B35a, B45, B46, B47, B55 finalize part,
B53 finalize half, the five 10.9 groups): round 2.

Round 1 has no exit row of its own; the exit rows are finalize's. What
round 1 has is the list of brief section 5, and the cases "over both
rounds" whose round 1 half exists. Each to its tests (`svc` =
`internal/service`, `hdl` = `internal/handler`, `mig` =
`internal/migrations`; every name starts with `TestIssueBudget` unless
written out):

| Round 1 case (brief section 5) | Tests |
| --- | --- |
| step 3 schema tests, moved and extended for the changed columns and constraints | `hdl` `…SchemaCatalog` (columns, types, the named checks, `> 0` for the reserve), `…SchemaConstraints` (accepted and rejected rows; the seven price cases on the entry table by constraint name), `…QueriesRoundTrip`, `…LedgerLoadIsOrderedByItsKey`; `mig` `…TableMigrationContract`, `…IndexMigrationContract`, `…RollbackKeepsLedgerRows` |
| migrations `up` from a fresh clone of the template | section 8, item 7 (the command, not a Go test), and `cmd/migrate`'s own tests |
| `Enable` and set-limit with `attempt_reserve_ticks`: each combination of null and set | `svc` `…LimitAndReserveAreSetTogether` |
| the same-values refusal | `svc` `…SetLimitToTheCurrentLimitIsRefused` |
| the replay under the original key | `svc` `…ManagementReplaysByOriginalKey`, `…OriginalKeyStandsForTheWholeIntent`, `…EnableIsOnePerIssueAndReplaysByOriginalKey` |
| a reserve of zero or below is a malformed request | `svc` `…MalformedRequestsSendNoStatement` |
| the golden vector of the intent digest | `svc` `…IntentDigestGoldenVectors` |
| statement lists written from the design: the level statement in all five, the reordered `Enable` | `svc` `…TransactionsFollowTheLockOrder` |
| the exclusive binding lock in `Enable` | `svc` `…TransactionsFollowTheLockOrder`, `…EnableHoldsTheBindingLockUntilItCommits`, `…EnableWaitsForAClaimThatReadUnbound`, `…EnableThatBindsNothingTakesNoBindingLock` |
| the exclusive binding lock in reparent rule 2 | `svc` `…OnlyAFirstBindingTakesTheBindingLock`, `…ReparentWaitsForAClaimThatReadUnbound` |
| its absence in child creation | `svc` `…OnlyAFirstBindingTakesTheBindingLock`, `…WritersThatBindNoExistingIssueDoNotWaitForABinder`, `…BindingNeverTouchesARootRow` |
| `transact` under an open REPEATABLE READ transaction | `svc` `…OperationsRefuseATransactionAtAnotherLevel` |
| `transact` under an open READ COMMITTED transaction | `svc` `…OperationsRunInsideAReadCommittedTransaction` |
| the `Enable` half of the cross-workspace case | `svc` `…InFlightScanFindsATaskRunFromAnotherWorkspace` |
| one definition of a quick-create task (N3) | `svc` `…InFlightScanDecidesQuickCreateByTheSharedDefinition`, `…QuickCreateDefinitionIsShared` |
| the isolation assertion fires from REPEATABLE READ on a tree with a root and costs no statement on a tree without one (binder half) | `svc` `…BindingRefusesATransactionThatIsNotReadCommitted`, `…BindingNeverTouchesARootRow` |
| ledger: full load, tail load | `svc` `…LedgerLoadsTheWholeLedgerOnceAndThenTheTail` |
| the continuity and projection checks failing on a damaged row; every cause of 10.5 | `svc` `…LedgerIsUnavailableForEachCause`, `…LedgerMustStartAtTheFirstSeq` |
| a root too large to cache | `svc` `…LedgerTooLargeToCacheIsReplayedOnEveryLoad`, `…LedgerCacheKeepsWithinItsLimit` |
| a reader that meets a cache newer than its snapshot | `svc` `…LedgerReaderWithoutTheRootLockPublishesNothing`, `…LedgerFailedReaderDropsOnlyTheStateItUsed` |
| a cache that survives a rolled-back append | `svc` `…LedgerAppendWritesRowsAndIsPublishedOnceCommitted` |
| `EncodeEntry` before insert, one row for each new entry, `Len()` against the revision | `svc` `…LedgerAppendWritesRowsAndIsPublishedOnceCommitted`, `…LedgerAppendThatIsRefusedWritesNothing`, `TestIssueBudgetEntryRowIsDerivedFromTheEntry` |
| one `With` call; pointers only; safe for concurrent use | `svc` `…LedgerReplayCostGrowsWithTheRowsRead`, `…LedgerStatesAreNeverAssignedThrough`, `…LedgerCacheIsSafeForConcurrentUse`, `…LedgerConcurrentWritersAndReadersAgree` |

What a ledger that is unavailable does to finalize (no entry, the attempt
still inserted, the observation, the log) is round 2; round 1 pins the
detection and the cache's side of it.

## 3. Deviations and "Needs a ruling"

### 3.1 Needs a ruling

No frozen rule turned out impossible or wrong in round 1, so nothing was
replaced by an interim step. Three choices below (R1 to R3) are mine where
the design is silent, and one measurement (R4) shows a limit of a rule as it
is written. The main session may want to rule on them before round 2 and
step 6 build on them. Each is one line in the final message.

- **R1. A reader's failed load drops only the state it went on from.**
  Code: `issueBudgetLedgerCache.load`, `internal/service/issue_budget_ledger.go`
  (the `case cached:` branch). Design 10.5 says the cached state of a root
  whose ledger is unavailable is dropped; 10.4 says a reader that meets a
  newer cache does not touch it. A reader can fail by the root row of an old
  snapshot while the cache holds a newer, correct state. Interim choice: a
  writer under the root lock drops whatever is cached; a reader drops the
  cached state only if it used it and it is still the one cached.
- **R2. A reader does not publish.** Code: the same function, the last `if`.
  The brief and 10.4 name two publish moments, both a writer's. Design 12.4
  (step 7) allows the read side to publish what it loaded when its snapshot
  is not older than the cache. Interim choice: `loadWithoutRootLock`
  publishes nothing. Allowing it later is a one-line change plus the
  condition of 12.4; nothing in round 1 depends on it.
- **R3. An entry row's `attempt_id` comes from the entry's reservation and
  from nothing else.** Code: `issueBudgetEntryRow`, same file. Design
  section 8 wants `kind`, `attempt_id`, `scope_id`, `invocation_id` derived
  from the entry by one constructor. An entry without a reservation (a gap
  that names no attempt, an exclusion) therefore gets a NULL `attempt_id`,
  and a reservation that is not the canonical lowercase text of a UUID is
  refused. If step 6 needs an attempt on a row whose entry names none, the
  constructor is the one place to change.
- **R4. The cap bounds the number of entries, not their bytes.** *Ruled in
  round 2 (round 2 file, 1.3) and built: see "Round 2, change 1.3" in the
  stage D section and section 6.1. What follows is the round 1 text.* Code:
  `issueBudgetLedgerWeight`, same file; measurement in section 6.1. Design
  10.4 weighs a root by its entries plus 2 and asks 5b for the memory bound.
  With identities in the 10.2 format and no free text the bound is 2,000
  bytes for each unit, 100 MB at the cap. But `EncodeEntry` lets a stored
  form of 65,536 bytes through, the ledger keeps every entry whole, and a
  cache full of such entries would hold about 3 GB. Interim choice: the
  weight is as the design defines it, and the bound is stated with its
  condition. Nothing round 1 or round 2 writes carries free text (reserve
  and hold entries). To be ruled before step 6 writes reported entries:
  either the writer limits their text fields (provider, price version,
  detail), or the weight counts bytes.

Round 2 status of the four: R1, R2 and R3 are accepted as built (round 2
file, 1.2). R1 is also the rule by which change 1.1 drops a cached state that
fails a check. R4 is ruled and built as change 1.3. The rulings that round 2
itself asks for are listed behind these, from R5 on.

### 3.2 Where the brief and the design differ

- **Level assertion in the binding functions.** The brief's stage list names
  only finalize's half (stage E). Design section 5 rule 5 assigns both
  halves to 5b. The design wins: the binder half is built in stage C.
- **The unit of the cap.** The brief's stage D says "50,000 entries
  process-wide". Design 10.4 (correction of 2026-11-18) makes the cap a sum
  of weights, a root weighing its entries plus 2. Built as the design has
  it: the constant is 50,000 units of weight.

### 3.3 Readings

Stages A to C:

- **Constraint and key names are mine.** Design 10.8 asks for three named
  price constraints on the entry table and 10.10 for a lock key that is not
  the fence's; neither gives a name. Names used:
  `issue_budget_entry_price_pair_check`, `…_price_digest_check`,
  `…_price_usage_check`, and the key text `issue_budget_binding:<workspace>`.
- **One definition of a quick-create task is a Go function, not SQL.**
  `parseQuickCreateContext` decides by four things: no issue, no chat
  session, no autopilot run, and a context that decodes as the quick-create
  type. The last one is a JSON decode with Go's rules; written a second time
  in SQL it would be a second definition that can drift. Design 10.13 also
  pins the quick-create branch's expression for its plan. So the SQL branch
  proposes candidates with its conditions unchanged and
  `quickCreateTaskContext` decides each one.
- **`TryLockIssueBudgetBindingShared` exists without a production caller.**
  It is finalize's side of the lock (stage E). The round 1 tests use it as
  the claim.
- **`transact` refuses SERIALIZABLE as well.** The brief names REPEATABLE
  READ and READ COMMITTED as the two cases. The level statement fails the
  same way under SERIALIZABLE; the test covers it.
- **Locks of an operation that runs as a savepoint.** When `Enable` runs
  inside an open READ COMMITTED transaction, the fence and the binding lock
  it takes are held until that outer transaction ends, not until the
  savepoint is released. A caller that starts an operation inside a long
  transaction keeps claims of the workspace out of the binding lock for that
  long. No production code calls an operation before step 7
  (`NewIssueBudgetService` has no production caller); only the tests pass an
  open transaction.
- **Reparent rule 2 has no switch read yet.** Design 10.10 places the lock
  before the read of the switch of 10.6, which is step 5c's. The lock is
  taken after the descendant walk and before the member rows.

Stage D:

- **One query serves both load paths.** `ListIssueBudgetEntryFormsAfter`
  reads `seq, schema_version, entry_json` after a seq; a full replay asks
  for the entries after 0. The step 3 query `ListIssueBudgetEntries`
  (`SELECT *`) is unchanged and has no production caller; the tests read
  rows back through it.
- **Rows beyond a reader's root row are later commits.** A READ COMMITTED
  reader can read its root row at revision n and then see rows n+1 and
  later that a writer committed in between. `loadWithoutRootLock` leaves
  them out. Under the root lock the same rows are damage (cause
  `sequence`), because nothing can be appended behind the lock holder's
  root row.
- **A cached state at the root's revision is still checked.** No row is
  read, but the cached state's figures are compared with the root row on
  every load, so a root row that was changed by hand is seen.
- **Too large to cache.** The warning is logged on every load of such a
  root (design 10.4: "replayed in full on every load, with one warn"). A
  root that grows past the limit loses the smaller state it had: keeping it
  would keep a tail read cheap, and would make the rule "not cached" depend
  on history.
- **The append guards the root row by its revision.**
  `UpdateIssueBudgetRootLedger` carries `ledger_revision = <the revision the
  append started from>`. Under the root lock it always holds; it turns a
  caller that passes a stale root row into an error instead of a root row
  that disagrees with its entries.
- **The length check runs before the first row is written.** Design 10.2:
  the new `Len()` must equal the new revision or the transaction fails.
  `appendEntries` compares them before it sends a statement, so the failure
  leaves nothing to roll back.
- **What the types do not enforce.** A transaction that has appended must go
  on with `Root` and `Ledger` of its append; if it called
  `loadUnderRootLock` again it would publish its own uncommitted entries.
  `Committed()` is to be called only after a confirmed commit. Both are in
  the doc comments, and both are for the round 2 call points to follow.
- **`issueBudgetLedgerEntry` carries only `Source` and `OperationID` of the
  envelope.** The price pair and the columns a reporting daemon supplies are
  for the usage writer (step 6).

Round 2, changes 1.1 and 1.3:

- **One warning for each dropped state, not for each load** (1.1). A reader
  whose cached state was replaced or evicted before the drop logs nothing.
  Under the root lock the warning names the state the load started from,
  also when another one had taken its place and is dropped with it.
- **The warning's cause is the failed check's; the error's cause is the full
  replay's** (1.1). They can differ, and
  `…UnavailableIsDecidedByTheFullReplay` pins one such case.
- **The write limit applies to every entry given to `appendEntries`, also to
  one the ledger already holds** (1.3). A replay gets no row, and is still
  refused when its stored form is over 2,048 bytes. The ruling's sentence is
  "refuses an entry whose stored form is longer"; read as "an entry that
  would get a row" the check would move behind `With`. If the main session
  reads it that way, the check moves there (the mutant `the cap only for
  entries that get a row` is that code) and the subtest "the ledger goes on
  behind a longer row" changes its first expectation.
- **The hard bound of section 6.1 is not the cell the ruling names** (1.3).
  The ruling expects the cell whose entries are all at the limit to be the
  bound. Usage entries lengthened to the limit behind one reservation cost
  more for each unit of weight (2,960 bytes against 2,464), so that figure is
  the one stated. Both are in the table of 6.1.

## 4. Entry-point inventory (stage G)

round 2

## 5. Requeue call sites and the laps measurement

round 2

## 6. Cache cap and query plans

### 6.1 The cap and its memory bound

The cap is the design's initial value: `issueBudgetLedgerCacheLimit =
50_000` units of weight for the process, a root weighing its entries plus 2.
Since round 2 (change 1.3, the ruling on R4) an entry a server writes has a
stored form of at most `issueBudgetLedgerEntryMaxBytes = 2048` bytes, so the
limit bounds bytes as well as entries. Two figures follow, for design 10.4:

| Figure | Bytes for each unit of weight | A cache at its limit |
| --- | ---: | ---: |
| **Measured maximum over the shapes servers write** (identities of design 10.2, no free text; round 1) | 1,838 measured; **2,000** stated, about 9 % above it | 87.7 MiB measured; **100 MB** (95.4 MiB) stated |
| **Hard bound over what a server may write** (every entry that can be is at the 2,048 bytes of the write limit; round 2) | 2,960 measured; **3,200** stated, about 8 % above it | 141.1 MiB (148.0 MB) measured; **160 MB** (152.6 MiB) stated |

Both are for go1.26.9, the higher of the two toolchains measured. The second
figure is not the cell the round 2 instructions expected. They ask for a cell
whose entries are all at the write limit and call its figure the hard bound;
that cell exists in two forms (exclusions, and gaps that name no attempt) and
gives 2,384 and 2,464 bytes (117.5 MiB). A usage entry costs more than its
text: it keeps a meter scope, and an estimate stays in the unknown list. The
metering package checks the length of an entry's identity and of its whole
form and of nothing else, so a usage entry may carry free text (its price
version, for one) up to the write limit. Such entries behind one reservation
give 2,960 bytes, and that is the figure stated as the hard bound. See "The
hard bound" below for the cells and for what the measurement does not cover.

**First figure: 2,000 bytes for each unit of weight, 100 MB (95.4 MiB) for a
cache at its limit, for ledgers whose entries carry identities in the format
of design 10.2 and no free text.** The highest figure measured is 1,838 bytes
(87.7 MiB at the limit); the bound leaves about 9 % above it.

How it was measured. `BenchmarkIssueBudgetLedgerCacheMemory`
(`internal/service/issue_budget_ledger_test.go`, no database):

```
cd server && GOTOOLCHAIN=go1.26.9 GOPROXY=off \
  go test ./internal/service/ -run '^$' -bench IssueBudgetLedgerCacheMemory -benchtime 1x
```

(exit 0, 45 s, darwin/arm64; log `/tmp/f3-s5b/logs/d-mem-go1269.log`). Each
cell fills a cache to its limit with roots of one shape and one size. Every
root is loaded the way a process loads it: entries encoded with
`EncodeEntry`, the stored forms replayed by `replayIssueBudgetLedger`, the
result published. The live heap is read after two collections before and
after, and the growth is divided by the cache's weight. The figure therefore
includes what the cache keeps for a root itself (map slot, list element),
which 5a's figure left out, and the strings are the decoder's own. Attempt
and invocation ids are 36-byte UUID texts and the model is 120 bytes, so a
usage identity is exactly 200 bytes, the longest 10.2 allows.

Shapes: `claimed` is one unsized reservation for each attempt (what a claim
writes); `held` adds a hold; `gap`, `reported` and `estimated` put 19 gap or
usage entries behind each reservation; the two `-run` shapes are one attempt
for the whole root. An estimated usage entry stays in the ledger's unknown
list, which is why it is the dearest.

Bytes for each unit of weight, go1.26.9:

| Entries in a root | claimed | held | gap | reported | reported-run | estimated | estimated-run |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 981 | 981 | 981 | 981 | 981 | 981 | 981 |
| 2 | 865 | 873 | 877 | 1,191 | 1,191 | 1,231 | 1,231 |
| 8 | 680 | 693 | 704 | 1,078 | 1,078 | 1,196 | 1,196 |
| 9 | 915 | 891 | 903 | 1,225 | 1,225 | 1,404 | 1,404 |
| 20 | 918 | 900 | 896 | 1,361 | 1,361 | 1,567 | 1,567 |
| 57 | 1,036 | 1,006 | 980 | 1,457 | 1,471 | 1,629 | 1,649 |
| 100 | 856 | 845 | 837 | 1,319 | 1,334 | 1,507 | 1,531 |
| 113 | 1,076 | 1,044 | 1,015 | 1,495 | 1,513 | 1,670 | 1,697 |
| 226 | 1,085 | 1,053 | 1,023 | 1,506 | 1,652 | 1,683 | **1,838** |
| 237 | 1,056 | 1,027 | 999 | 1,604 | 1,622 | 1,777 | 1,803 |
| 450 | 1,086 | 1,041 | 1,011 | 1,505 | 1,643 | 1,683 | 1,831 |
| 473 | 1,055 | 1,014 | 986 | 1,595 | 1,612 | 1,767 | 1,795 |
| 898 | 1,071 | 1,032 | 995 | 1,510 | 1,648 | 1,670 | 1,818 |
| 5,000 | 919 | 896 | 876 | 1,449 | 1,467 | 1,604 | 1,631 |
| 49,998 | 852 | 837 | 824 | 1,356 | 1,374 | 1,518 | 1,545 |
| highest | 1,086 | 1,053 | 1,023 | 1,604 | 1,652 | 1,777 | 1,838 |
| MiB at the limit | 51.8 | 50.2 | 48.8 | 76.5 | 78.8 | 84.7 | 87.7 |

A root without an entry costs 533 bytes (266 for each of its two units); a
root of one entry 2,943 bytes.

What the figures say:

- **The cost moves with the size of the root.** A Go map keeps between 7/16
  and 7/8 of its slots in use, so an entry is dearest just behind a
  doubling. The sizes in the table are not round numbers for that reason: a
  scan of every size from 1 to 1,100 entries for the three dearest shapes
  (`/tmp/f3-s5b/logs/d-mem-scan.log`) found the peaks at 226 entries for a
  run and 237 with twenty entries to an attempt, and the table holds those,
  their doubles and sizes between. The first version of the benchmark used
  other sizes and missed the peaks.
- **5a's bound does not hold for the 10.2 identities.** 5a measured at most
  (entries + 2) × 1,400 bytes with 36-byte ids, about 70 MB. Reservations,
  holds and gaps stay below that (1,086 at most). Usage entries with the
  200-byte identity and their scope do not: 1,652 reported, 1,838
  estimated. This is the measurement design 10.4 asked 5b for.
- **Go 1.27.0 gives lower figures** (same command, the tree's default
  toolchain on this machine; `d-mem-go1270.log`): 1,675 at most
  (79.9 MiB), reported-run 1,567, claimed 1,070. The bound is stated for
  1.26.9, the higher of the two.
- **The weight bounds entries, not bytes.** The cell `form-limit/100` holds
  entries whose stored form is the 65,536 bytes `EncodeEntry` lets through
  (an exclusion with a long `Detail`): 64,891 bytes for each unit, which
  would be 3,094 MiB at the limit. A ledger keeps its entries whole. The
  first figure is therefore valid as long as no writer puts free text into
  an entry. This was R4 of round 1. It is ruled (round 2 file, 1.3): the
  bytes are bounded where entries are written, and the second figure is the
  bound that follows.

**The hard bound (round 2).** Four more shapes, each at the 15 sizes of the
table above, with every entry that can be at the write limit of 2,048 bytes
and all of the added bytes text that a ledger keeps byte for byte:

- `written-limit-exclusion` and `written-limit-gap`: exclusions, and gaps
  that name no attempt, with a long `Detail`. Neither needs another entry,
  so every entry of the root is at the limit.
- `written-limit-estimated` and `written-limit-estimated-run`: the two
  dearest shapes of round 1 with the price version of each usage entry
  lengthened until its stored form is 2,048 bytes. A usage entry hangs on a
  reservation, and a reservation a server writes has no text to lengthen:
  its kind takes no detail, and `issueBudgetEntryRow` refuses a reservation
  ID that is not the text of a UUID. (The round 2 file gives its form as 609
  bytes unsized and 625 sized; that is the main session's measurement, not
  repeated here.) So one entry in twenty, or one in the root, is below the
  limit.

```
cd server && GOTOOLCHAIN=go1.26.9 GOPROXY=off go test ./internal/service/ -run '^$' \
  -bench 'IssueBudgetLedgerCacheMemory/(written-limit|form-limit)' -benchtime 1x -timeout 900s
```

(exit 0, 68 s; log `/tmp/f3-s5b/logs/r2-13-mem-go1269.log`.) Bytes for each
unit of weight, go1.26.9:

| Entries in a root | exclusion | gap | estimated | estimated-run |
| ---: | ---: | ---: | ---: | ---: |
| 1 | 1,392 | 1,360 | 981 | 981 |
| 2 | 1,517 | 1,521 | 1,515 | 1,515 |
| 8 | 1,760 | 1,835 | 1,991 | 1,991 |
| 9 | 1,952 | 2,065 | 2,231 | 2,231 |
| 20 | 2,079 | 2,205 | 2,548 | 2,548 |
| 57 | 2,306 | 2,382 | 2,668 | 2,728 |
| 100 | 2,144 | 2,261 | 2,565 | 2,634 |
| 113 | 2,362 | 2,443 | 2,727 | 2,803 |
| 226 | 2,384 | 2,464 | 2,749 | 2,959 |
| 237 | 2,363 | 2,441 | 2,846 | 2,925 |
| 450 | 2,378 | 2,458 | 2,756 | **2,960** |
| 473 | 2,356 | 2,434 | 2,841 | 2,924 |
| 898 | 2,384 | 2,446 | 2,746 | 2,951 |
| 5,000 | 2,256 | 2,328 | 2,683 | 2,766 |
| 49,998 | 2,189 | 2,277 | 2,597 | 2,681 |
| highest | 2,384 | 2,464 | 2,846 | 2,960 |
| MiB at the limit | 113.7 | 117.5 | 135.7 | 141.1 |

Go 1.27.0 (`r2-13-mem-go1270.log`, 48 s): 2,368, 2,448, 2,705 and 2,813
at most (134.1 MiB). `form-limit/100` in these two runs: 64,931 and 64,875
bytes; round 1 had 64,891, so a figure moves by less than a tenth of a
percent from run to run.

What this bound rests on, and what it does not cover:

- It is the highest figure over the shapes measured, not a proof over every
  entry the metering package accepts. The shapes are the ones the first
  slice writes (reserve, hold, gap, usage in mode delta with a meter scope
  for each request, which is the dearest way to spread usage) and the
  exclusion. Cumulative observations, corrections and settlements are not
  written before a later step and were not measured: the step that starts
  to write one adds its shape to the benchmark.
- The added text is in one field. Spread over several fields it is as many
  allocations, each rounded up to a size class; the 8 % between 2,960 and
  3,200 is meant to cover that and was not measured.
- It holds for rows that went through `appendEntries`. A row another build
  or an operator wrote is loaded as it is (up to 65,536 bytes) and cached by
  its weight like any other; `form-limit` is what such rows can cost.

### 6.2 What a load and an append cost beyond what is kept

`With` copies the whole state it extends. Measured on go1.26.9 (scratch
harness through `-overlay`, nothing added to the tree; best of five;
`/tmp/f3-s5b/logs/d-load-cost-go1269.log`):

| Case | held shape | estimated-run shape |
| --- | --- | --- |
| full replay, time for each row | 5.5 to 6.8 µs | 8.0 to 11.3 µs |
| full replay, allocated for each row | 3.7 to 4.1 KB | 5.0 to 5.5 KB |
| full replay of 49,998 entries | 286 ms, 187.5 MB allocated | 457 ms, 249.6 MB |
| full replay of 60,000 entries (too large to cache: every load) | 408 ms, 245.3 MB | 483 ms, 328.2 MB |
| tail of 2 rows onto a cached state of 49,998 | 5.3 ms, 38.9 MB | 16.8 ms, 55.5 MB |
| `With` of one entry onto 49,998 (an append) | 4.6 ms, 34.9 MB | 21.1 ms, 51.5 MB |
| tail of 2 rows onto 5,000 | 250 µs, 4.2 MB | 1.7 ms, 6.0 MB |
| `With` of one entry onto 5,000 | 212 µs, 3.8 MB | 956 µs, 5.6 MB |
| tail of 2 rows onto 1,000 | 69 µs, 0.94 MB | 229 µs, 1.3 MB |
| `With` of one entry onto 1,000 | 50 µs, 0.85 MB | 182 µs, 1.2 MB |

- The time for each row is close to 5a's 5.4 to 6.2 µs for entries without
  usage; usage entries with the long identity cost up to twice that. The
  stored forms are 619 bytes a row in the held shape and 981 in the other.
- The allocation is transient: it is garbage once the new state is built.
  But it means the bound of 6.1 is for what the cache keeps. A load with a
  tail and every append allocate one more copy of that root's state for a
  moment, and a full replay about four times the state. Several large
  roots loading at once add up.
- An append to a root costs in proportion to the entries the root already
  has. For a root of 1,000 entries that is 50 to 180 µs; for one near the
  cap it is 5 to 21 ms and 35 to 52 MB of garbage for each append.
- A root beyond the cap pays the full replay on every load, as the design
  says: about 0.4 to 0.5 s and 250 to 330 MB of garbage at 60,000 entries,
  before the query (6.3). The warning names it each time.
- One `With` call for all rows is what keeps a replay linear:
  `…LedgerReplayCostGrowsWithTheRowsRead` pins it, and the mutant with one
  call for each row allocates 260 MB for 800 rows.

### 6.3 Query plans

Database: `f3_budget_s5b_plan`, a clone of `f3_budget_s4c_plan` (no test
was connected to the template or to the clone; only `psql`). PostgreSQL
17.11 in a container that is amd64 emulated on arm64, so the times are
pessimistic and say more against each other than by themselves.
`shared_buffers` 128 MB, `work_mem` 4 MB, `random_page_cost` 4, `jit` on,
`plan_cache_mode` auto.

**What was changed by hand in the clone**, as the brief allows and asks to
be said beside the plans:

- The template carries step 4's form of migration 866. In the clone
  `issue_budget_root` got `attempt_reserve_ticks`, and `issue_budget_entry`
  got `price_snapshot`, `price_digest` and the three named constraints, by
  `ALTER TABLE`, so the new columns are at the end of the row. The attempt
  table was not altered: no query planned here reads it.
- The template has members, issues, tasks and agents (22 workspaces, 65,031
  issues, 60,023 queue rows of which 23 are in flight, 300 agents, 57,000
  member rows in 19,010 roots) but no root row and no entry. Seeded: 19,010
  root rows, one for each member root, at the revision of their entries, and
  406,762 entries. The entries are the real stored forms and receipts of an
  unsized reservation and a hold for each attempt, taken from `EncodeEntry`
  (609 and 629 bytes, receipts 296 bytes). Root sizes: one of 60,000, one
  of 49,998, ten of 5,000, 200 of 200, and 18,798 of 2 to 20 entries.
- Every root's entries are spread over the whole heap in seq order, which
  puts almost every entry of a root on a page of its own: the worst layout,
  chosen on purpose. `VACUUM (ANALYZE)` afterwards.
- Sizes: entry heap 530 MB in 67,794 pages (6 rows to a page),
  `issue_budget_entry_seq_key` 23 MB in 2,941 pages, the database 757 MB.
- Scripts and full outputs are in `/tmp/f3-s5b/plan/` (`seed.sql`,
  `facts.out`, `plans.sql` and `plans.out`, `inflight.sql` and
  `inflight.out`). That directory is scratch; the lines that matter are
  below.

pgx prepares and caches a statement for each connection. The server plans
the first five executions with the parameters (custom plans) and from the
sixth uses the generic plan when its estimate is not above the average
custom one. Both kinds are shown: custom with literal parameters, generic
with `PREPARE`, `SET plan_cache_mode = force_generic_plan` and `EXPLAIN
(ANALYZE, BUFFERS) EXECUTE`.

**`ListIssueBudgetEntryFormsAfter`** (new: the ledger load, whole and tail).
Custom plans:

| Case | Plan | Buffers | Time |
| --- | --- | ---: | --- |
| root of 8, after 0 | `Index Scan using issue_budget_entry_seq_key`, `Index Cond: (budget_root_id = … AND seq > 0)`, `Filter: workspace_id = …` | 14 | 1.6 ms (cold) |
| root of 200, after 0 | `Bitmap Index Scan` on the same index and condition, `Bitmap Heap Scan`, `Sort` (quicksort, 153 kB) | 206 | 7.3 ms |
| root of 5,000, after 4,990 (a tail of 10) | the same (715 rows estimated, 10 read) | 13 | 0.57 ms |
| root of 5,000, after 0 | the same; sort in memory, 3.3 MB | 5,042 | 168 ms |
| root of 49,998, after 49,990 (a tail of 8) | the same | 11 | 0.28 ms |
| root of 49,998, after 0 | `Gather Merge`, 2 workers, `Parallel Bitmap Heap Scan` on the same index condition, external merge sort (about 31 MB of temp files) | 45,218 | 415 ms, then 156 ms |
| root of 60,000, after 0 | `Gather Merge`, 2 workers, `Parallel Seq Scan` of the entry table, external merge sort | 67,870 | 171 ms, then 112 ms |

Generic plan, one for every parameter: `Index Scan using
issue_budget_entry_seq_key`, `Index Cond: ((budget_root_id = $2) AND (seq >
$3))`, `Filter: (workspace_id = $1)`, cost 0.42..48.63, no sort. A root of
8: 0.2 ms, 11 buffers. A tail of 8 behind 49,990: 0.04 ms, 11 buffers. A
root of 60,000: 139 and 163 ms, 51,262 buffers. With the server's default
setting the sixth execution of the prepared statement for the 60,000-entry
root is this plan (507 ms in that run, which also wrote 26 buffers).

- Every plan but one reads only what the index condition on
  `(budget_root_id, seq > n)` selects. A tail costs the tail: 11 to 13
  buffers whatever the size of the root.
- The exception is the planner's choice for the root that holds 14.7 % of
  all entries: a parallel sequential scan of the table, about as fast here
  as the index scan (112 to 171 ms against 139 to 163 ms). A root of that
  size is one that is too large to cache and pays this on every load.
- `ORDER BY seq` comes from the index in the index-scan plans and from a
  sort in the others; the sort of 50,000 rows spills to temp files at
  `work_mem` 4 MB. The load does not trust the order: its seq check refuses
  rows that arrive out of it.
- `workspace_id` is a filter in every plan and removes nothing, because the
  entries of a root all carry its workspace. It is there for the tenant
  rule.

**`UpdateIssueBudgetRootLedger`** (new: the root row after an append).
`Update on issue_budget_root`, `Index Scan using issue_budget_root_id_key`,
`Index Cond: (id = …)`, `Filter: ((workspace_id = …) AND (ledger_revision =
60000))`, cost 0.29..8.31, one row: 8.8 ms cold, 0.15 ms warm. The generic
plan is the same (`id = $9`, the filter on `$8` and `$10`). At a revision
the root is not at: `Rows Removed by Filter: 1`, no row, 0.07 ms. Each was
run in a transaction that was rolled back, and the root row was read
afterwards: unchanged. No column the statement sets is in an index.

**`ListIssueBudgetInFlightTasks`** (changed in stage C: the issue branch).
Scope: the 3,000 issues of one tree, in a workspace that has 15 of the 300
agents. Second runs.

| Case | Cost | Time | Buffers | Issue branch |
| --- | ---: | ---: | ---: | --- |
| custom, 3,000 ids | 1140.72 | 1.50 ms | 140 | nested loop: `Bitmap Heap Scan` on `agent_task_queue` through `idx_agent_task_queue_agent` with `Index Cond: status = ANY (…)` (the whole `(agent_id, status)` index, 74 buffers; 23 rows in flight, 19 removed by the scope filter), then `Index Only Scan using idx_issue_workspace_id_keyset` on `issue` (`workspace_id = … AND id = t.issue_id`, no heap fetch) |
| custom, one id | 242.43 | 0.40 ms | 58 | `Index Scan using idx_agent_task_queue_issue_id_keyset` (`issue_id = ANY (…)`, the status as filter), then `issue_pkey` with the workspace as filter |
| generic, 3,000 ids | 327.26 | 5.79 ms | 1,390 | `Bitmap Heap Scan` through `idx_agent_task_queue_issue_id_keyset` (`issue_id = ANY ($2)`: 2,481 rows fetched, 2,477 removed by the status filter, 1,308 heap blocks), then `issue_pkey` (`Filter: workspace_id = $1`) |
| generic, one id | as custom | 0.17 ms | 58 | as custom |

The quick-create branch is as step 4 left it in every case: the workspace's
agents (`Seq Scan on agent`, 15 rows), `idx_agent_task_queue_agent` on
`(agent_id = a.id AND status = ANY (…))` with the filter `issue_id IS NULL
AND context->>'type' = 'quick_create'`, joined to the scope. The 3,000 ids
return 6 rows (4 on issues, 2 quick-create), the set step 4 reported.

Step 4's figures for the old query on the same data
(`step4-binding-check.md` 8.2 and 8.3), old to new: custom with 3,000 ids
620.57, 0.75 ms, 102 buffers to 1140.72, 1.50 ms, 140; one id 243.41,
0.10 ms, 54 to 242.43, 0.40 ms, 58; generic 328.24, 5.57 ms, 1,390 to
327.26, 5.79 ms, 1,390.

The issue branch no longer starts from the workspace's agents. With many
ids its custom plan therefore reads the whole `(agent_id, status)` index,
whose size follows the queue table of the installation and not the
workspace (75 pages at 60 thousand queue rows; step 4 measured 305 pages at
300 thousand), and keeps the rows in scope. That is what the supplement to
design 10.2 costs: a task run by another workspace's agent on a member
issue has to be found. Design 10.13 adds no index for the in-flight scan in
the first slice, and none was added.

**Statements that read no relation.** `LockIssueBudgetBinding`,
`TryLockIssueBudgetBindingShared` and `GetIssueBudgetTransactionIsolation`:
`Result (cost=0.00..0.01 rows=1 …)`.

**Stage A's changed statements** (`InsertIssueBudgetRoot`,
`InsertIssueBudgetAttempt`, `InsertIssueBudgetEntry`,
`UpdateIssueBudgetRootPolicy`) changed their column lists only and were not
planned again.

**Member lookup, open attempts of a task, the unknown-set query of design
10.12**: round 2 (they are finalize's and the inventory's).

## 7. Step 3 and step 4 tests changed and why

No test was deleted, skipped or weakened. Each change below follows a rule
the design changed in section 10; the assertion that was there is kept in its
new form.

Stage A (design 10.8: the reserve on the root, the nullable attempt
reservation, the price snapshot moved from the attempt to the entry):

| File and test | Change | Why |
| --- | --- | --- |
| `internal/handler/issue_budget_schema_test.go`, `issueBudgetNamedChecks` | the two attempt price constraint names removed, the three entry price constraint names added | the constraints moved tables |
| same file, amounts | `attempt_reserve_ticks` joins a new list `issueBudgetPositiveAmounts` that must read `> 0`; every other `_ticks` column still must read `>= 0` | the reserve is a positive amount (10.1) |
| same file, column types | four columns expected present (`attempt_reserve_ticks`, entry `price_snapshot`, `price_digest`, nullable attempt `reserved_ticks`), the two attempt price columns expected absent | 10.8 |
| same file, accepted and rejected rows | added: an unsized attempt, a sized attempt of zero, two roots with a reserve, a priced usage entry, a reserve of 0 and of -1 refused. The four attempt price cases moved to a loop over the entry table (seven cases, each asserting the constraint name PostgreSQL reports) plus two accepts at the inclusive size limits | the same rules, on the table that now carries them |
| `internal/handler/issue_budget_queries_test.go`, `TestIssueBudgetQueriesRoundTrip` | root insert and policy update carry the reserve; a bare root reads NULL and NULL; attempts insert a nullable reservation, one of them unsized; one priced entry is inserted in a transaction that is rolled back | the generated parameters changed |
| `internal/migrations/issue_budget_test.go`, `TestIssueBudgetTableMigrationContract` | accepts `> 0` for the columns in `issueBudgetPositiveAmounts`; was "every `_ticks` column is `>= 0`" | the reserve is the one positive amount |

Stages B and C (`internal/service/issue_budget_test.go`):

| Test | Change | Why |
| --- | --- | --- |
| 21 calls of `SetLimit` across the file | a fourth argument, `nil` | the signature takes limit and reserve together (10.1) |
| `…ManagementReplaysByOriginalKey` | the replayed result is compared with the reserve in it | the reserve is part of the stored result |
| `…SetLimitToTheCurrentLimitIsRefused` | rewritten to the two-amount rule: `state_conflict` only when both amounts equal the stored ones; the same limit with another reserve is accepted and is one policy revision | 10.1 changed what a no-op is. The step 4 assertion (same limit refused) is kept for the case where the reserve is unchanged too |
| `…OriginalKeyStandsForTheWholeIntent` | two more cases: the same key with another limit, and with another reserve | the digest gained a field; the test walks the digest fields one by one |
| `…MalformedRequestsSendNoStatement` | two more refusals: a reserve of 0 and of -1 | 10.1 |
| `…TransactionsFollowTheLockOrder` | all five statement lists rewritten from the design: the level statement first in each, and in `Enable` the binding lock, then the scan, then the activation sample | section 5 rule 5, 9.2 with 10.10 |
| `…BindingNeverTouchesARootRow` | the map of statements a binding function may send gained `GetIssueBudgetTransactionIsolation` and `LockIssueBudgetBinding` | stage C sends them; neither touches a root row, which is what the test guards |

`internal/handler` budget tests other than the two stage A files, and
`internal/service/issue_budget_writers_test.go`, were not changed.

Round 2, change 1.1 (`internal/service/issue_budget_ledger_test.go`; these
are this lane's own round 1 tests, listed here because the rule they pinned
changed):

| Test | Change | Why |
| --- | --- | --- |
| `…LedgerIsUnavailableForEachCause` | round 1 pinned "a load from a cached state that fails a check is unavailable and sends one read". Now the load from a cached state (cache `warm`) reads the tail and then every row, is unavailable with the cause of the damage because the full replay fails too, leaves nothing cached and logs one warning about the dropped state; the full replay (cache `cold`) and a second load read once and log nothing; the reader (cache `behind`) is checked the same way. The 17 damages, the causes, "no load writes anything", "the state handed out before is unchanged" and the repair at the end are kept | only a full replay decides "unavailable" (round 2 file, 1.1) |
| `…LedgerUnderTheRootLockDropsAStateAheadOfTheDatabase` | added: exactly one warning with cached revision 4, root revision 2 and the cause `ahead_of_root`. The drop, the full read and the publication are asserted as before | the drop of a state ahead of the root row is logged (1.1) |
| `…LedgerFailedReaderDropsOnlyTheStateItUsed` | round 1 pinned "the failing reader is told the ledger is unavailable after one read". Now the reader that went on from a cached state reads the tail and then every row, and is told the ledger is unavailable because the full replay fails too (the rows are damaged in each part of this test). What it drops (its own state; not a newer one that it did not use; not one published between its two reads) is asserted as before, and the warnings are asserted with it: one for the dropped state, none where nothing was dropped | 1.1; R1 is unchanged |

No assertion of round 1 was dropped without its counterpart under the new
rule: where a load used to be refused, the test now pins the second read,
the result of the full replay and the warning.

## 8. Verification (brief section 8)

Run on 2026-11-19 against the tree as it stands at the end of round 1. No
file's content changed between the first run below and the last: item 6
rewrote the generated files with the same bytes (same SHA-256 before and
after, and again after the last run), no other file in the worktree has a
modification time later than the first run, and `git status --porcelain`
after the last run equals the one taken at item 6. `HEAD` is still
`86fea651c`. All commands
from `server/`, with `TMPDIR` set to the real path of the temporary
directory and `DATABASE_URL` pointing at the database named in the row.
Every database is a fresh clone of `f3_budget_base_865`, migrated `up` with
`go run ./cmd/migrate up` right after the clone (`/tmp/f3-s5b/db.sh new
<suffix>`). One DB-backed suite ran at a time. Counts are top-level `---
PASS` / `--- FAIL` / `--- SKIP` lines of the `-v` output; logs are in
`/tmp/f3-s5b/logs/`.

The default toolchain of the tree on this machine is go1.27.0; the second
one is `GOTOOLCHAIN=go1.26.9 GOPROXY=off`.

### 8.1 go1.27.0

| Item | Command | Database | Exit | PASS / FAIL / SKIP |
| --- | --- | --- | ---: | --- |
| 1 | `gofmt -l` over the twelve changed `.go` files | none | 0 | prints nothing |
| 1 | `go build ./...` | none | 0 | no output |
| 1 | `go vet ./...` | none | 0 | no output |
| 2 | `go test ./internal/budget/... -count=1 -v` | none | 0 | 112 / 0 / 0 |
| 3 | `go test ./internal/service/... ./internal/handler/... -run 'Budget' -count=1 -v` | `f3_budget_s5b_v3a` | 0 | 114 / 0 / 1 |
| 3 | the same with `-race` (`-timeout 30m`) | `f3_budget_s5b_v3b` | 0 | 114 / 0 / 1, no data race reported |
| 4 | `go test ./cmd/migrate/... -count=1 -v -timeout 20m` | `f3_budget_s5b_v4m` | 0 | 67 / 0 / 1 |
| 4 | `go test ./internal/service/... -count=1 -v -timeout 40m` | `f3_budget_s5b_v4s` | 0 | 1673 / 0 / 18 |
| 4 | `go test ./internal/handler/... -count=1 -v -timeout 40m` | `f3_budget_s5b_v4h` | 0 | 3028 / 0 / 53 |
| 6 | `/tmp/f3-s5b/bin/sqlc generate` (sqlc v1.31.1, the version `make sqlc` names, built from the module cache) | none | 0 | the SHA-256 of all 147 files under `pkg/db/generated/` and `git status --porcelain` are the same before and after |
| 7 | `go run ./cmd/migrate up` on a fresh clone of the template | `f3_budget_s5b_v7` | 0 | 866 to 880 applied, `schema_migrations` ends at `880_issue_budget_ready` |
| 7 | `go test ./internal/handler/ -run 'TestIssueBudgetSchemaCatalog\|TestIssueBudgetSchemaConstraints\|TestIssueBudgetLedgerLoadIsOrderedByItsKey\|TestIssueBudgetQueriesRoundTrip\|TestWorkspaceDeletionManifestCoversPublicSchema' -count=1 -v` | `f3_budget_s5b_v7` | 0 | 5 / 0 / 0 |
| 7 | `go test ./internal/migrations/... -run TestIssueBudget -count=1 -v` | `f3_budget_s5b_v7` | 0 | 3 / 0 / 0 |

The item 3 run by package: `internal/service` 84 / 0 / 0, of which 60 are
`TestIssueBudget…` and 24 are tests of other features with `Budget` in
their name; `internal/handler` 30 / 0 / 1, of which 13 are
`TestIssueBudget…`. With subtests 195 / 0 / 1. The one skip is
`TestRedisWebhookIPRateLimiter_HasSeparateBudgetFromTokenLimiter`
(`REDIS_TEST_URL not set`), a webhook test the pattern `Budget` happens to
select; it is not a database skip and not a test of this feature.

The baseline selections, for comparison, on `f3_budget_s5b_v3a`:

| Selection | Baseline | Now | Exit |
| --- | --- | --- | ---: |
| `./internal/service/ -run TestIssueBudget` | 31 / 0 / 0 | 60 / 0 / 0 | 0 |
| `./internal/handler/ -run 'TestIssueBudget\|TestDeleteMemberIssueKeepsBudgetRows\|TestWorkspaceDeletion\|TestDeleteIssue\|TestBatchDeleteIssues'` | 26 / 0 / 0 | 26 / 0 / 0 | 0 |
| `./internal/budget/...` | 112 / 0 / 0 | 112 / 0 / 0 | 0 |
| `./cmd/migrate/...` | 67 / 0 / 1 | 67 / 0 / 1 (the same skip: `TestCommentContentBigramRequirementMatchesRealPGBigm`, no `pg_bigm`) | 0 |
| `./internal/migrations/... -run TestIssueBudget` | 3 / 0 / 0 | 3 / 0 / 0 | 0 |

The service count went up by 29: four new tests in stage B
(`…LimitAndReserveAreSetTogether`, `…IntentDigestGoldenVectors`,
`…OperationsRefuseATransactionAtAnotherLevel`,
`…OperationsRunInsideAReadCommittedTransaction`), ten in stage C and
fifteen in stage D. No count went down.

The whole packages (item 4), against the step 4 checker's run:

| Package | Baseline | Now | Exit | With subtests | Time `go test` printed |
| --- | --- | --- | ---: | --- | ---: |
| `./internal/service/...` | 1644 / 0 / 18 | 1673 / 0 / 18 | 0 | 4044 / 0 / 18 | 922 s |
| `./internal/handler/...` | 3028 / 0 / 53 | 3028 / 0 / 53 | 0 | 5955 / 0 / 53 | 160 s |
| `./cmd/migrate/...` | 67 / 0 / 1 | 67 / 0 / 1 | 0 | 149 / 0 / 1 | 25 s |

- `internal/service`: +29, the same 29 new tests; 60 of the 1673 are
  `TestIssueBudget…`. All 18 skips read `REDIS_TEST_URL not set`, as in the
  baseline.
- `internal/handler`: the count is the baseline's, because this round added
  no handler test (the two stage A files were changed in place); 13 of the
  3028 are `TestIssueBudget…`. The 53 skips by reason: 46 `REDIS_TEST_URL
  not set`; 2 `opt-in: MULTICA_RUN_I1_P95=1 with an exclusive DATABASE_URL`
  (a latency measurement that is off by default, not a missing database);
  and one each of `set MULTICA_TRIAGE_PERF=1 …`, `set
  MULTICA_RUN_SKILLS_SH_INTEGRATION=1 to run live GitHub integration test`,
  `wire fixture export requested only by cross-package verification`, `the
  catalog ships no HTTP recipes` and `500-branch requires injecting a stub
  Queries …`. None is a database skip, and none is a test of this feature.
- The tests the brief names as the ones that would show a regression of the
  claim path all ran: top-level tests with `Claim` in their name, 58 in
  `internal/service` and 134 in `internal/handler`; with `Finaliz`, 20 and
  11; with `Sweep`, 6 and 7. None failed. Round 1 does not touch
  `FinalizeTaskClaim`: the only change to `internal/service/task.go` is the
  extraction of `quickCreateTaskContext`, the body of `FinalizeTaskClaim`
  (lines 3734 to 3866) does not contain the word budget, and nothing outside
  `issue_budget_ledger.go` and the tests calls the two loads,
  `appendEntries` or `TryLockIssueBudgetBindingShared`.

No count went down and the number of skips is the baseline's in every
selection.

### 8.2 go1.26.9

`GOTOOLCHAIN=go1.26.9 GOPROXY=off` in the environment of every command;
`go version` printed `go1.26.9 darwin/arm64` under it. The two clones were
also migrated with this toolchain. No file changed since section 8.1.

| Item | Command | Database | Exit | PASS / FAIL / SKIP |
| --- | --- | --- | ---: | --- |
| 1 | `gofmt -l` over the same twelve files, with the `gofmt` of this toolchain (`$(go env GOROOT)/bin/gofmt`) | none | 0 | prints nothing |
| 1 | `go build ./...` | none | 0 | no output |
| 1 | `go vet ./...` | none | 0 | no output |
| 2 | `go test ./internal/budget/... -count=1 -v` | none | 0 | 112 / 0 / 0 |
| 3 | `go test ./internal/service/... ./internal/handler/... -run 'Budget' -count=1 -v` | `f3_budget_s5b_v3c` | 0 | 114 / 0 / 1 |
| 3 | the same with `-race` (`-timeout 30m`) | `f3_budget_s5b_v3d` | 0 | 114 / 0 / 1, no data race reported |

By package the same as on go1.27.0: `internal/service` 84 / 0 / 0,
`internal/handler` 30 / 0 / 1 with the same one skip
(`TestRedisWebhookIPRateLimiter_HasSeparateBudgetFromTokenLimiter`,
`REDIS_TEST_URL not set`); with subtests 195 / 0 / 1 in both runs.

Beyond what the brief asks for on this toolchain, the baseline selections
ran on it too, one after another on `f3_budget_s5b_v3c` after the item 3 run
(so not on a fresh clone):

| Selection | Baseline | Now | Exit |
| --- | --- | --- | ---: |
| `./internal/handler/ -run 'TestIssueBudget\|TestDeleteMemberIssueKeepsBudgetRows\|TestWorkspaceDeletion\|TestDeleteIssue\|TestBatchDeleteIssues'` | 26 / 0 / 0 | 26 / 0 / 0 | 0 |
| `./internal/service/ -run TestIssueBudget` | 31 / 0 / 0 | 60 / 0 / 0 | 0 |
| `./internal/migrations/... -run TestIssueBudget` | 3 / 0 / 0 | 3 / 0 / 0 | 0 |

### 8.3 Not run in round 1

- `./cmd/server/...` (item 4) and item 5 (`./internal/daemon/...`,
  `./pkg/protocol/...`): the brief puts them in round 2.
- The whole packages under `-race`, and the whole packages on go1.26.9.
- `make sqlc` itself; `sqlc generate` of the same version ran instead
  (item 6). The reasons are in section 9.

## 9. Not done / not verified

**Not done, because it is round 2:**

- Stages E, F and G: finalize's budget step (design 10.2), the two claim
  call points with the capability header and `budget_attempt_id`, the
  entry-point inventory. `FinalizeTaskClaim` is as it was before this
  round: finalize does nothing with budgets.
- Nothing in production calls the two loads, `appendEntries`,
  `Committed()` or `TryLockIssueBudgetBindingShared` yet. They are built
  and tested for the round 2 call points.
- Finalize's half of the isolation assertion (stage E) and `pkg/protocol`'s
  capability constant (stage F).
- Sections 4 and 5 of this report, the finalize rows of section 2, and the
  plans of the member lookup, the open attempts of a task and the
  unknown-set query of design 10.12.

**Left to other steps:**

- "`cost.price_digest` equals the column", guaranteed by the writer: the
  usage writer is step 6.
- The read of the switch of design 10.6 in reparent rule 2: step 5c.
- A bound on the text fields of reported entries: R4.

**Not run:**

- `make sqlc` itself. It runs `go run …sqlc@v1.31.1`, which asks the module
  proxy, and this lane has no network beyond PostgreSQL. The same sqlc
  version, built from the module cache with `GOPROXY=off`, ran `sqlc
  generate` (section 8, item 6).
- Of section 8: `./cmd/server/...` (item 4) and item 5
  (`./internal/daemon/...`, `./pkg/protocol/...`). The brief puts them in
  round 2. Round 1 changed no file in those packages; `go build ./...` and
  `go vet ./...` cover that they compile against the changed ones.
- The whole packages under `-race`. The race detector ran over every test
  whose name contains `Budget` (item 3), which includes all tests this
  round added.
- The whole packages on go1.26.9. The brief asks for items 1 to 3 on the
  second toolchain, and those ran.
- No real provider call, no installed agent CLI, no `agentintegration` tag,
  no hosted CI, no release or deploy. Nothing was committed, stashed, reset
  or pushed.

**Verified only in part, or not verifiable here:**

- **The memory bound** was measured on darwin/arm64 only. Go's 64-bit
  layouts and allocator size classes are the same on linux/amd64, so the
  figure should carry over, but it was not measured there.
- **The plans** are from one data set, seeded by hand, on an emulated
  container. The mix of root sizes is invented, and the planner's choices
  at the edges (bitmap against index scan, the sequential scan for a root
  holding 15 % of the table) follow from it.
- **The isolation level of the production transactions** that reach the
  binding functions was established by reading the call sites (stage C),
  not by a test over all of them.
- **What the cache cannot see.** The main session read this item and
  accepted it for the first slice with no code and no tombstone (round 2
  file, 1.4); (2) and (3) are restated here under the rule of change 1.1.
  (1) Damage to a row that a cached state already covers is not found until
  that state is dropped (a restart, an eviction, another process), because a
  load with a cached state reads only the rows behind it; the root row's
  figures are still compared on every load. (2) A database put back under a
  running process and then grown to or past the cached revision with other
  entries is found only if the tail does not apply to the cached state or
  the figures differ. When it is found, the load no longer fails for that
  reason: the cached state is dropped with one warning and every row is read
  within the same load, so the process goes on with what the rows say, and
  the ledger is unavailable only if the rows themselves fail. When it is not
  found, it is case (1). (3) A `Committed()` that arrives while the root has
  no cached state puts a committed state back. A lock holder leaves a root
  without a state only after a full replay that failed, or after a read that
  failed behind the drop; a state that fails a check is otherwise replaced
  by the full replay within the same load. A reader that dropped its state
  leaves the root without one whatever its full replay gave, because a
  reader publishes nothing (R2). In each case the state that comes back is a
  committed one and is checked against the root row by the next load like
  any other; whether the rows below its revision still are what it was built
  from is case (1). (1) and (3) together are the warm process: it does not
  look at rows below its cached revision, so it may go on appending to a
  root whose rows were damaged underneath it, and a restart makes every
  process agree.
- **The syntax scan** (`…LedgerStatesAreNeverAssignedThrough`) reads the
  files that import the metering package. A ledger handled in a file that
  does not import it is not seen.
- **`…LedgerConcurrentWritersAndReadersAgree`** relies on interleavings
  that real connections produce. The mutant without the reader's trim fails
  it, so the race does occur in it, but an interleaving it does not happen
  to produce is not covered by it. The cases that must hold are pinned
  one by one in the reader tests.
- **The third golden vector's input is not a literal in the test.** The
  tooling of this lane decodes a typed backslash-u escape before it reaches
  a file, so a Go literal that must contain the six characters of such an
  escape cannot be typed through it. The input was built in Python with
  `chr(92)`, written to `/tmp/f3-s5b/golden/v3.bin` and hashed outside Go;
  the test holds that digest as a literal.

**For the reviewer before round 2:** R1 to R4 (section 3.1); in section 3.3
the two things the types do not enforce about an append, and that an
operation run as a savepoint keeps the fence and the binding lock until the
outer transaction ends.

## Databases created by this lane

Twelve, all named `f3_budget_s5b_*`, each created with `CREATE DATABASE …
TEMPLATE …` and, for the clones of the base template, migrated `up` with the
worktree's `cmd/migrate`. The list was kept in `/tmp/f3-s5b/databases.txt`
as they were created.

| Database | Template | Used for |
| --- | --- | --- |
| `f3_budget_s5b_a1` | `f3_budget_base_865` | stage A schema and migration runs; the working runs of the service tests in stages A to C; the mutation runs of stages B and C |
| `f3_budget_s5b_c1` | `f3_budget_base_865` | the handler budget run of stage C |
| `f3_budget_s5b_d1` | `f3_budget_base_865` | the ledger tests and the mutation runs of stage D |
| `f3_budget_s5b_plan` | `f3_budget_s4c_plan` | the query plans of section 6.3; altered and seeded by hand as stated there; no test connected to it |
| `f3_budget_s5b_v7` | `f3_budget_base_865` | section 8 item 7 (go1.27.0) |
| `f3_budget_s5b_v3a` | `f3_budget_base_865` | item 3 and the baseline selections (go1.27.0) |
| `f3_budget_s5b_v3b` | `f3_budget_base_865` | item 3 with `-race` (go1.27.0) |
| `f3_budget_s5b_v4m` | `f3_budget_base_865` | item 4, `./cmd/migrate/...` |
| `f3_budget_s5b_v4s` | `f3_budget_base_865` | item 4, `./internal/service/...` |
| `f3_budget_s5b_v4h` | `f3_budget_base_865` | item 4, `./internal/handler/...` |
| `f3_budget_s5b_v3c` | `f3_budget_base_865` | item 3 and the baseline selections (go1.26.9) |
| `f3_budget_s5b_v3d` | `f3_budget_base_865` | item 3 with `-race` (go1.26.9) |

All twelve were dropped on 2026-11-19 after the last run (`DROP DATABASE IF
EXISTS … WITH (FORCE)`, one statement each; no connection to any of them was
open at the time). Checked afterwards: `pg_database` has no row named
`f3_budget_s5b_%`, and the two templates are still there at the size they
had before, `f3_budget_base_865` (19 MB) and `f3_budget_s4c_plan` (136 MB).
Neither template was connected to by a test, migrated, altered or dropped;
both were used only as the source of `CREATE DATABASE … TEMPLATE`. No other
database was touched.

## Hooks and reminders that asked for out-of-scope edits

None asked to edit, shorten or reorganize the memory index, a `CLAUDE.md`
or any file outside this lane's scope, and nothing was written under
`~/.claude/**` or under `.trellis/` beyond this report. For completeness,
what did arrive and was not followed, each because the dispatch says
otherwise:

- The session-start context of the task workflow, repeated after every
  compaction, routes an in-progress task to sub-agents (`trellis-implement`,
  `trellis-check`) and lists the workflow's finishing steps (spec update,
  commit). No sub-agent was started, no spec was updated, nothing was
  committed.
- The global instructions of the orchestration layer ask to delegate
  multi-file work and to have a separate reviewer approve it. The work was
  done in this lane; the review is the main session's, between the rounds.
- The harness's standing instruction to save memories under
  `~/.claude/projects/…/memory/`. Nothing was saved there.
- A reminder about attribution lines for commits and pull requests. Not
  applicable.

STATUS: round 1 complete
