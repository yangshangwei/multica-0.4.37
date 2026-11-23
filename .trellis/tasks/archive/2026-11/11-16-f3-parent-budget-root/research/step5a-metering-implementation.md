# F3-S1 step 5a: pure metering package — implementation report

Written on 2026-11-17 by the step 5a implementer, for the main session.

- **Scope:** `server/internal/budget/` only (package `budget`; no database, no I/O).
- **Where:** scratch worktree `/Volumes/artisan/code/2026/mc-f3-budget-5a-20261117`, detached at `86fea651c`. The package is untracked there (`git status --short` prints only `?? server/internal/budget/`). The main session copies it back.
- **State:** complete and frozen. The last change to the package was `bench_test.go` at 18:52:45; the other 14 files have not changed since 16:49:58. Section 7 lists the sha256 of every file.

## Summary

- **Changed since (line added on 2026-11-18):** the "Addendum: second pass" at the end of this file changes the API listed in section 1.1. `Apply` is no longer exported, and the stored form (`EncodeEntry`, `DecodeEntry`) and the read method `Reservation` are new. Section P.2 lists the API as it now stands.
- **Delivered:** the unsized reservation (design §10.1), the non-mutating apply method `With` (§10.4), and the read methods `Len` and `HasReservation` (§10.5).
- **Tests:** 89 pass, 0 fail, 0 skipped, with and without `-race` (70 existed before, 19 are new). Two existing expectations changed; section 4 explains both. These runs predate the last edit, which touched only the benchmark file (section 4.1).
- **Mutation run:** 62 hand-written mutants, one per new rule; all 62 are caught.
- **The 5 ms rule:** one clone-plus-apply on a 10,000-entry ledger takes 1.0 to 1.9 ms (median, four ledger shapes). It does not exceed about 5 ms, so §10.13 asks for no cheaper structure.
- **A cost the rule does not cover:** a full replay is quadratic while nothing settles, which is the case in the first package (§11.8). A ledger of reserve-and-hold pairs replays in 63 ms at 10,000 entries and in 4.9 s at 100,000. This needs a decision (section 6, question 1).
- **Stored form:** every stored entry gained the key `"unsized"`, so every entry digest changed. No code outside the package needs an edit to compile (section 2).
- **Deviations from design.md §10:** none.

Sections: 1 what changed, 2 stored form, 3 unsized reservation semantics, 4 tests, 5 benchmarks, 6 deviations and open questions, 7 housekeeping, appendix A the mutants.

## 1. What changed

### 1.1 New exported identifiers

| Identifier | Declaration | Meaning |
| --- | --- | --- |
| `Entry.Unsized` | field `Unsized bool`, tag `json:"unsized"`, placed directly after `ReservedTicks` | Marks a reserve entry that has no amount. |
| `ReasonReservationUnsized` | `const ReasonReservationUnsized UnknownReason = "reservation_unsized"` | Reason of the unknown item that an unsettled unsized reservation contributes. Also the `Receipt.Reason` of its reserve entry. |
| `(*Ledger).With` | `func (l *Ledger) With(entries ...Entry) (*Ledger, []Receipt, error)` | Returns a new ledger holding the receiver's state plus `entries`, and their receipts. It never writes to the receiver. |
| `(*Ledger).Len` | `func (l *Ledger) Len() int` | Number of entries the ledger holds. |
| `(*Ledger).HasReservation` | `func (l *Ledger) HasReservation(id string) bool` | Whether a reserve entry opened a reservation with this ID. |

Nothing exported was removed or renamed. The unexported additions are the field `reservation.unsized` and the method `reservation.overrun()`.

Details of the three methods:

- **`With`** judges a batch exactly as `Apply` does (same validation, same receipts, same replay and conflict rules). The result shares no map, scope or slice with the receiver. With no entries it returns an independent copy, also for the zero `Ledger`. On any error it returns `(nil, nil, err)` and keeps nothing of the valid part of the batch.
- **`Len`** counts every stored entry once, including entries kept only as a record (receipts `stale`, `quarantined`, `comparison`). A replayed entry and the entries of a refused batch are not counted.
- **`HasReservation`** stays true after a hold and after settlement. It is false for the empty string, and an entry ID is not a reservation ID.

### 1.2 Behaviour changes to existing identifiers

- **`(*Ledger).Apply`**: same signature and same results. It is now `With` followed by replacing the receiver's state.
- **`Entry` stored form and `Entry.Digest()`**: one more key in the form, so a different digest for every entry (section 2).
- **Validation**: a reserve entry with `Unsized` and a non-zero amount is `ErrInvalidEntry`; `Unsized` on any other kind is `ErrInvalidEntry`.
- **`Receipt` of a reserve entry**: `Reason` is `reservation_unsized` for an unsized reserve. It was always empty for reserves and still is for sized ones.
- **`Projection()`**: `Unknown` may hold `reservation_unsized` items, and `Overrun` leaves unsized reservations out. `TotalKnown()`, `Remaining()` and `UnknownCounts()` follow from that without a code change.
- **`Coverage.IncludedAttempts`**: counts unsized reservations like sized ones.
- **Concurrency contract of `Ledger`** (doc comment only): "A Ledger is not safe for concurrent use" became "Apply writes to the ledger, so it must not run beside any other use of the same Ledger. Everything else only reads it: With, Projection, Len and HasReservation may run concurrently on a ledger that nothing applies to." The cache of §10.4 depends on this, and a test under `-race` pins it.
- **`Reduce`**, the price snapshot, the decimal helpers and all usage rules are unchanged.

### 1.3 Files

| File | Status | Content |
| --- | --- | --- |
| `model.go` | changed | `Entry.Unsized`, `ReasonReservationUnsized`, comment of `KindReserve` |
| `ledger.go` | changed | `With`, `Len`, `HasReservation`, `Apply` through `With`, validation, reserve, settle and overrun rules |
| `ledger_test.go` | changed (2 lines) | pinned stored form and digest |
| `invariants_test.go` | changed (helper `checkLedger` only) | overrun rule and one new invariant |
| `unsized_test.go` | new | 10 tests |
| `with_test.go` | new | 9 tests |
| `bench_test.go` | new | 5 benchmarks over 4 ledger shapes |
| `decimal.go`, `decimal_test.go`, `edge_cases_test.go`, `price.go`, `price_test.go`, `scale_test.go`, `usage.go`, `usage_test.go` | byte-identical to the step 4 tree | — |

The diffs of the four changed files against the step 4 tree are in `/tmp/f3-5a/diff-model.patch`, `diff-ledger.patch`, `diff-ledger-test.patch` and `diff-invariants-test.patch`.

## 2. Stored form

**What changed.** The stored form of an entry is `json.Marshal(Entry)`: keys in struct-field order, every key always present. `Entry` gained `Unsized` directly after `ReservedTicks`, so every stored entry of every kind now carries `"unsized":false` or `"unsized":true` between `"reserved_ticks"` and `"hold_reason"`.

**Digests.** The digest is still the lowercase hex SHA-256 of `budget-entry-v1\n` followed by the stored form. Because the form changed, the digest of every entry differs from the one the step 2 package gave for the same entry. The price snapshot form and its domain `budget-price-snapshot-v1` are unchanged (`price.go` is byte-identical).

| Pinned entry | Test | Digest |
| --- | --- | --- |
| usage entry `e1`, step 2 form | (old value in `ledger_test.go`) | `a3ff7a1296a15e687d3407b38255b6122d95471a6e163f2a34c5aee6af6f2b3f` |
| usage entry `e1`, new form | `TestEntryStoredFormRoundTripsAndDigests` | `5f07d194587118bd12ebe5834f4d42d2838648c32b4238448948aff5ae79bbc9` |
| unsized reserve `reserve-1` | `TestUnsizedFlagIsPartOfTheEntryIdentity` | `a9fd918db913b78298cf666b0d4c14bee8eb40a53741e63eb2a30ec9b6c1dd7b` |
| the same reserve sized at zero | `TestUnsizedFlagIsPartOfTheEntryIdentity` | `4ea6c1daffd840c9a0b2ae92534e52aa6b03a103b363adade4d2d8bb728d4c2c` |

All four values were computed outside Go with Python `hashlib` (`/tmp/f3-5a/digests.py`, output in `/tmp/f3-5a/digests.out`). The script first reproduces the old pinned digest from the old form, which shows that it computes what the package computes.

The pinned stored form of an unsized reserve entry (540 bytes):

```json
{"id":"reserve-1","kind":"reserve","reservation_id":"r1","reserved_ticks":"0","unsized":true,"hold_reason":"","proof":{"final_usage":false,"scope_complete":false,"no_further_execution":false},"gap_reason":"","detail":"","usage":{"scope_id":"","invocation_id":"","mode":"","sequence":"0","overlap":"","baseline_event_id":"","supersedes":"","correction_authorized":false,"tokens":{"input":"0","output":"0","cache_read":"0","cache_write":"0"},"cost":{"basis":"","ticks":"0","currency":"","zero_basis":"","price_version":"","price_digest":""}}}
```

**Receipts.** `Receipt` has no new field. The stored receipt of an unsized reserve has `"reason":"reservation_unsized"`.

**What a step 3 or step 4 caller must change: nothing to compile.**

- I type-checked the step 4 tree against the new package without writing into it: `go vet -overlay /tmp/f3-5a/copyback/overlay.json ./internal/budget/... ./internal/service/ ./internal/handler/`, run in `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server` with the 15 new files overlaid. Exit 0, empty output (`/tmp/f3-5a/copyback/vet.log`).
- The importers are `internal/service/issue_budget.go` (uses only `budget.Ticks` and `budget.Count`), `internal/service/issue_budget_test.go`, and four handler test files: `issue_budget_queries_test.go`, `issue_budget_schema_test.go`, `issue_budget_delete_test.go`, `issue_budget_teardown_test.go`.
- None of them holds a hand-written entry form or a pinned entry digest. I searched the step 4 `server/` tree for `"reserved_ticks":"`, `hold_reason` and `budget-entry-v1`. The only hits are the comment in `migrations/866_issue_budget.up.sql` and the digest domain written out in `issue_budget_schema_test.go:426`, which hashes whatever bytes the test passes in. The only 64-digit hex literal in the importing files is an operation intent digest of zeros (`issue_budget_teardown_test.go:39`). The tests marshal `budget.Entry` at run time, so they pick up the new form by themselves.
- The database rules are not affected. `issue_budget_entry_digest_check` recomputes the digest over the stored bytes. `entry_json` must be 2 to 65,536 bytes, and a reserve or hold entry with the §10.2 identifiers is 609 to 643 bytes (computed in `/tmp/f3-5a/stored_sizes.py`). `receipt` only has to be a JSON object.
- **Not run:** the database-backed handler and service tests of the step 4 tree against the new package. This step may not use a database, so the type-check is all I can claim. I expect them to pass because nothing pins a form, but that is an expectation.

**Rows in the old form.** `json.Unmarshal` is lenient: a row without the `unsized` key decodes as `Unsized=false` and replays without error, but re-marshalling it gives other bytes and another digest than the stored ones. No such row exists outside test databases, since migration 866 is not committed. Section 6 has the related question (question 2) and a proposal for a strict decoder (proposal 1).

## 3. Unsized reservation semantics as implemented

Each rule names its origin: the design, or a choice I made where the design is silent. The choices are collected in section 6 (question 3) so they can be confirmed or changed by number.

1. **Form.** A reserve entry with `Unsized: true` and `ReservedTicks: 0` opens an unsized reservation. *Design §10.1.*
2. **Validation.** `Unsized` with any non-zero amount is `ErrInvalidEntry` ("an unsized reservation carries no amount"). This check runs before the negative-amount check, so an unsized entry with a negative amount is `ErrInvalidEntry`, not `ErrNegativeAmount`. `Unsized` on a usage, hold, settle, gap or exclusion entry is `ErrInvalidEntry` ("fields of another kind are set"). As before, a refused entry refuses its whole batch and leaves no trace. *My choice of error class.*
3. **Figures.** The entry adds nothing to R or to A+R+H. It adds one to `Coverage.IncludedAttempts`. *Design for the amount; counting it as an included attempt is my choice.*
4. **The unknown item.** Accepting the entry appends exactly one item to the unknown set: `{Reason: "reservation_unsized", EntryID: <ID of the reserve entry>, ReservationID: <reservation ID>}`, with empty `ScopeID` and `Detail`. It goes to the end of the list, which keeps recording order. The receipt is `accepted` with `Reason: "reservation_unsized"`. Replaying the entry adds nothing. *The design names the reason; the fields of the item and the receipt reason are my choice.*
5. **While it is unsettled** `TotalKnown()` is false and `Remaining(limit)` is nil for every limit. *Follows from rule 4 and the existing rules.*
6. **When the item is removed.** Only an accepted settle entry for that reservation removes it, together with the reservation's lifecycle items as before. Usage, a hold or a replay never removes it. *Design §10.1.*
7. **Spend and overrun.** Usage that names the reservation counts in A in full, as on a sized reservation, so an unsized reservation occupies exactly its known actual spend in A+R+H. None of that spend is overrun, before or after settlement: `Overrun` sums over sized reservations only. *Design §10.1.*
8. **A sized reservation of zero is a different fact.** A reserve entry with amount 0 and `Unsized: false` has a known amount of zero: no unknown item, an empty receipt reason, and every tick spent on it is overrun. The same entry ID once unsized and once sized at zero is `ErrConflict`, because the flag is part of the entry's content. *My choice, following "0 不能表示未定额" in §10.1.*
9. **Hold.** A hold on an unsized reservation is accepted as on a sized one: the state becomes held, the hold's own item is appended (its reason is the hold reason), and the receipt carries that reason. H does not change, because there is no unspent amount to move. The `reservation_unsized` item stays. An ended unsized attempt therefore contributes two unknown items, or more if holds with different reasons arrive. A second hold with the same reason is `stale`, and so is a hold after settlement. *Follows from the existing hold rules; the design does not spell it out.*
10. **Settle.** It needs all three proofs, otherwise `ErrReleaseProofIncomplete` (unchanged). It is refused with the same error while usage with unknown cost hangs on the reservation (unchanged; by the same code a gap entry naming the reservation also blocks it). The unsized item and the lifecycle items do not stand in the way. On success it releases 0 (`Receipt.Released` is 0 and `Reason` is empty), marks the reservation settled and removes every unknown item of that reservation. A second settle is `stale`. *Design §10.1 for the removal; the rest is unchanged behaviour.*
11. **After settlement** the total is known again if nothing else is unknown, and `Remaining` is the limit minus A+R+H. Usage that arrives later still counts in A, is still not overrun, and does not bring the item back. *Follows from rule 7 and from what a settled sized reservation does; the design does not spell it out.*
12. **Independence.** Sized and unsized reservations in one ledger keep their own figures and their own unknown items.

**What this means in the first package.** No production path settles (§10.3, §11.8). Every attempt on a root without `attempt_reserve_ticks` therefore leaves a `reservation_unsized` item for good, plus one lifecycle item once the attempt has ended and reconciliation has written its hold.

## 4. Tests

### 4.1 Verification runs

All commands ran in `server/` of the scratch worktree with go1.27.0 darwin/arm64. The run is recorded in `/tmp/f3-5a/final/summary.txt` (18:20:30 to 18:22:01); the logs are in `/tmp/f3-5a/final/`.

| Command | Exit | Result | Log |
| --- | --- | --- | --- |
| `gofmt -l internal/budget` | 0 | no output | `gofmt.out` |
| `go vet ./internal/budget/...` | 0 | no output | `vet.log` |
| `go test ./internal/budget/... -count=1 -v` | 0 | PASS 89, FAIL 0, SKIP 0 | `test-v.log` |
| `go test -race ./internal/budget/... -count=1` | 0 | `ok`, no data race reported | `race.log` |
| `go test -race ./internal/budget/... -count=1 -v` | 0 | PASS 89, FAIL 0, SKIP 0 | `race-v.log` |
| `go test -race ./internal/budget/... -count=5 -shuffle=on` | 0 | `ok` | `race-shuffle.log` |
| `go build ./...` | 0 | no output | `build.log` |

Before any change the same test command gave PASS 70, FAIL 0, SKIP 0 (`/tmp/f3-5a/baseline.log`).

**These runs are older than the last edit.** At 18:52:45 I added one more ledger shape to `bench_test.go`; no test function changed. On the final bytes I ran `gofmt -l` (no output), `go vet` (exit 0, `/tmp/f3-5a/vet8.log`), the benchmarks of the new shape (exit 0) and one mutant run in which only the expected test failed. I did not repeat the full test, `-race` and build runs on the final bytes; the main session is running them.

### 4.2 Properties that had to stay

- **No floating point.** `grep -rniE 'float' internal/budget` finds nothing (exit 1) on the final bytes. A type-level check on the final bytes agrees: `/tmp/f3-5a/nofloat/nofloat.go` type-checks the package with its test files and reports every expression or literal of a floating-point or complex type. Result: 15 files, 17,440 expressions, 1,707 definitions, 3,269 literals, none of such a type (`/tmp/f3-5a/nofloat/run-final.log`). Its positive control, a small package that uses floats, reports 9 findings and exits 1 (`/tmp/f3-5a/nofloat/control.log`). The benchmarks compute bytes per entry with integer division.
- **Standard library only in non-test code.** `go list -deps ./internal/budget` lists 107 packages, and the only one outside the standard library is the package itself (`/tmp/f3-5a/final/deps-nonstd.txt`). The three new test files also import only the standard library. One existing test file, `scale_test.go` (external test package, unchanged since step 2), imports `server/pkg/agent` to compare one constant, so the test binary links that package and its dependencies (`/tmp/f3-5a/final/deps-test-nonstd.txt`).

### 4.3 Red before green

| Stage | State of the package | Result | Log in `/tmp/f3-5a/` |
| --- | --- | --- | --- |
| red 1 | new tests written, nothing implemented | does not compile, exit 1 (`unknown field Unsized`, `With undefined`, `undefined: ReasonReservationUnsized`) | `red1-compile.log` |
| red 2 | declarations only: the field, the constant and methods that do nothing | PASS 69, FAIL 19: every new test of that time (18) and the pinned stored form | `red2-stubs.log` |
| red 3 | unsized reservation implemented; `With`, `Len`, `HasReservation` still do nothing | PASS 78, FAIL 10 | `red3-unsized-only.log` |
| green | everything implemented | PASS 88, FAIL 0 | `green3.log` |
| final | one more test, `TestLedgersDerivedFromOneOriginDoNotAffectEachOther` | PASS 89, FAIL 0 | `final/test-v.log` |

The last test was added after green, so it has no red run of its own. The mutation run stands in for one: 16 mutants turn it red (appendix A).

### 4.4 Changed expectations in existing tests

Only two existing test files changed. Both changes are forced by the design, not by convenience.

1. **`ledger_test.go`, `TestEntryStoredFormRoundTripsAndDigests`.** The pinned stored form gained `"unsized":false` and the pinned digest changed from `a3ff7a12…` to `5f07d194…` (section 2). Reason: every stored entry has the new key.
2. **`invariants_test.go`, helper `checkLedger`.** The helper recomputed overrun as "actual above reserved" for every reservation, which contradicts §10.1 for unsized ones; it now leaves them out. It also gained one invariant: an unsized reservation is in the unknown set exactly once from its reserve entry until it settles, a sized one never is, and an unsized reservation has a reserved amount of zero. The existing random test that uses the helper never creates an unsized reservation, so for that test the helper checks what it checked before.

To show that nothing else had to move, I ran the seven test files of the step 4 tree unchanged against the new code through an overlay: PASS 69, FAIL 1, and the one failure is the pinned stored form (`/tmp/f3-5a/old-tests/run.log`). All existing tests of `Apply` pass unmodified, although `Apply` now goes through `With`.

### 4.5 Mutation run

**Method.** `/tmp/f3-5a/mutate.py` makes one edit to a copy of one source file and hands the copy to the compiler with `go test -overlay`, so the package directory never holds broken code. A mutant counts as caught when the test binary was built and at least one test failed. Every mutant ran once without `-race`; the mutants of `With` also ran with `-race`.

**Result: 62 mutants, 79 runs, 62 caught, no survivor.** No mutant failed to build and no edit missed its anchor.

| IDs | What the mutants break | Mutants | Caught |
| --- | --- | --- | --- |
| U01–U30 | the rules of the unsized reservation (section 3) | 30 | 30 |
| E01–E06 | the stored form and the entry identity of the flag | 6 | 6 |
| W00–W16 | `With`: independence from the receiver, results on error, equality with `Apply` | 17 | 17, and all 17 again under `-race` |
| A01–A09 | `Len` and `HasReservation` | 9 | 9 |

Appendix A lists every mutant with its rule and the number of tests it turns red. The raw results are in `/tmp/f3-5a/mutations.out` (56 mutants), `/tmp/f3-5a/mutations-extra.out` (5) and `/tmp/f3-5a/mutations-w16.out` (1); the logs of each mutant are under `/tmp/f3-5a/mut/`, `/tmp/f3-5a/mut-extra/` and `/tmp/f3-5a/mut-w16/`.

Three things to know about the `With` mutants:

1. **Shared state (W03–W12).** These let the result share a map, a scope or the unknown list with the receiver. All ten are caught without `-race` by comparing values, and all ten are also reported as data races under `-race`.
2. **A write that is undone (W16).** This mutant makes `With` change a field of its receiver and put the old value back. `TestWithIsSafeBesideReadersOfTheOrigin` catches it. Under `-race` the detector reports the write itself (28 reports), whatever the timing. Without `-race` it was caught in the one run I made, because a reader happened to see the value in between; that catch depends on scheduling. **So this property is reliably guarded only when the package's tests run with `-race`.**
3. **A catch through a panic (W01).** One existing test catches W01 only because the mutant panics on a nil map. I added W00, which breaks the same rule without the panic; eight tests fail on it by assertion.

**Limits.** The mutants are hand-written, one per rule I identified. This is not a generated or exhaustive mutation run, and each mutant ran once.

### 4.6 The new tests and what they pin

"Rule n" refers to section 3. The last column is the number of mutants that turn the test red without `-race`; no new test is red for fewer than five.

| Test | Pins | Mutants |
| --- | --- | --- |
| `TestUnsizedReservationIsUnknownUntilItSettles` | rules 3 to 6: no figure, one unknown item, no remaining amount, removed by settle | 14 |
| `TestSettleOfAnUnsizedReservationNeedsEveryProof` | rule 10: each missing proof refuses the settle | 6 |
| `TestLifecycleHoldKeepsTheUnsizedItem` | rule 9 | 13 |
| `TestUnsizedIsValidOnlyOnAReserveWithNoAmount` | rule 2 | 16 |
| `TestSpendOnAnUnsizedReservationIsNeverOverrun` | rules 7, 8 and 11 | 11 |
| `TestSizedAndUnsizedReservationsDoNotAffectEachOther` | rule 12 | 15 |
| `TestSettleOfAnUnsizedReservationStillRefusesUnknownUsage` | rule 10: unknown usage still blocks | 12 |
| `TestUnsizedReservationsReplayToTheSameProjection` | a history with sized and unsized reservations replays to the same projection and receipts | 19 |
| `TestUnsizedFlagIsPartOfTheEntryIdentity` | rule 8 and the pinned forms and digests of section 2 | 9 |
| `TestRandomInterleavingsWithUnsizedReservationsKeepTheInvariants` | 300 seeds of 60 steps: invariants after every accepted entry, a misplaced flag always refused without a trace, replay gives the same state | 28 |
| `TestWithLeavesTheReceiverUntouched` | the receiver equals a ledger rebuilt from its entries, before and after | 27 |
| `TestLedgersDerivedFromOneOriginDoNotAffectEachOther` | two results of one origin, using the same IDs with different content | 16 |
| `TestWithNoEntriesIsAnIndependentCopy` | also for the zero `Ledger` | 16 |
| `TestWithOnErrorReturnsNothingAndLeavesTheReceiverUntouched` | seven error classes | 18 |
| `TestWithGivesWhatApplyGives` | same state and receipts, for new and replayed entries | 5 |
| `TestChainOfWithEqualsAFullReplay` | 150 seeds: one entry at a time, a prefix and then the rest, or all at once | 17 |
| `TestWithIsSafeBesideReadersOfTheOrigin` | 4 readers and 4 derivers on one ledger, 150 rounds; meant for `-race` | 17 |
| `TestLenCountsDistinctAcceptedEntries` | `Len` as described in section 1.1 | 7 |
| `TestHasReservation` | `HasReservation` as described in section 1.1 | 11 |

The random test fails unless its generator actually reached the cases it is meant to cover. Its floors are 300 unsized reservations, 150 spent on, 80 held, 120 settled and 500 misplaced flags; the logged run reached 520, 244, 128, 201 and 959.

## 5. Benchmarks

### 5.1 Setup

- **Machine:** Apple M4, 10 CPUs, 16 GB, go1.27.0 darwin/arm64. The machine is shared with other agents: the load average was 3.3 to 8.9 during the main run (17:52 to 17:58) and 4.0 to 4.3 during the later run of the fourth shape (21:04 to 21:07). Runs of the same benchmark differ by up to about 25 percent (reserve and hold at 10,000 entries: 0.98 to 1.33 ms), so read every time with that margin.
- **Commands** (also in the header of `bench_test.go`): `go test ./internal/budget -run '^$' -bench . -benchmem` and `go test ./internal/budget -run '^$' -bench LoadedLedgerMemory -benchtime 1x`.
- **Identifiers** are 36 bytes, the length of a UUID in text form. The identifiers of §10.2 are longer (section 5.5).
- **Ledger shapes.** A ledger of n entries is made of attempts:

| Shape (name in the benchmark) | One attempt | Unknown list |
| --- | --- | --- |
| cumulative | unsized reserve, 18 reported running totals in one meter scope, settle (20 entries) | stays empty |
| delta | unsized reserve, 18 reported delta events, each in its own meter scope, settle (20 entries) | stays empty |
| mixed | attempts of the cumulative shape alternate with attempts of an unsized reserve and 19 delta events whose cost is an estimate; those never settle | half of all entries |
| reserve and hold (`held`) | unsized reserve and a lost-contact hold, no settlement (2 entries) | one item per entry |

The last shape is what steps 5b and 5c write on their own in the first package, where nothing settles. I added it at 18:52 after reading §11.8.

### 5.2 Clone plus apply: `With` with one reserve entry

`BenchmarkWith` derives the next state from a ledger of n entries by appending the reserve entry of one more attempt. Times are the minimum, median and maximum of five runs.

| Shape | Entries | Min | Median | Max | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| cumulative | 1,000 | 120.5 µs | 123.3 µs | 131.7 µs | 775,893 | 1,376 |
| cumulative | 10,000 | 0.99 ms | 1.00 ms | 1.10 ms | 7,049,315 | 13,582 |
| cumulative | 100,000 | 15.19 ms | 15.31 ms | 16.39 ms | 64,476,144 | 135,558 |
| delta | 1,000 | 201.4 µs | 202.2 µs | 228.1 µs | 1,141,790 | 3,733 |
| delta | 10,000 | 1.93 ms | 1.93 ms | 2.08 ms | 10,262,235 | 37,145 |
| delta | 100,000 | 30.75 ms | 31.41 ms | 33.47 ms | 93,163,760 | 371,055 |
| mixed | 1,000 | 175.2 µs | 175.5 µs | 216.6 µs | 1,004,406 | 2,629 |
| mixed | 10,000 | 1.50 ms | 1.51 ms | 1.52 ms | 9,099,453 | 26,113 |
| mixed | 100,000 | 23.73 ms | 23.95 ms | 25.56 ms | 83,276,416 | 260,799 |
| reserve and hold | 1,000 | 127.0 µs | 129.6 µs | 142.7 µs | 849,963 | 1,023 |
| reserve and hold | 10,000 | 0.98 ms | 1.28 ms | 1.33 ms | 7,645,093 | 10,093 |
| reserve and hold | 100,000 | 15.68 ms | — | 20.74 ms | 69,775,972 | 100,653 |

The last row has two runs of 30 iterations, so it has no median. Sources: `/tmp/f3-5a/bench1/with.log` with the summary in `/tmp/f3-5a/bench1/summary.txt`, `/tmp/f3-5a/bench2/with-held.log` and `/tmp/f3-5a/bench2/with-held-100k.log`.

**Answer to the 5 ms rule: no.** One clone-plus-apply on a 10,000-entry ledger takes 1.0, 1.9, 1.5 and 1.3 ms (median) for the four shapes. That is below about 5 ms, so §10.13 asks for no cheaper structure. At 100,000 entries it takes 15 to 31 ms.

One earlier figure should be ignored: `/tmp/f3-5a/bench-smoke-held.log` shows 4.45 ms for reserve and hold at 10,000 entries. That was a single cold iteration; the five timed runs above replace it.

**Where the time goes.** The clone, not the apply. One reducer step without the clone costs 0.4 to 1.4 µs (`/tmp/f3-5a/probe/step.log`). The clone copies every map and every meter scope: at 10,000 entries of the cumulative shape it takes 1.23 ms, of which the entries map is 0.67 ms, the receipts map 0.20 ms and the scopes 0.11 ms (`/tmp/f3-5a/probe/clone.log`). The cost is linear in the number of entries. Each call allocates 7 to 10 MB at 10,000 entries (64 to 93 MB at 100,000), and after each write one full copy of the root's state is garbage.

### 5.3 Full replay and load, with nothing cached

`BenchmarkReplay` applies n decoded entries to an empty ledger; `BenchmarkLoad` also decodes their stored forms. Times are the median of three runs, except the last shape, which has one iteration per size (`/tmp/f3-5a/bench-smoke-held.log`).

| Shape | Entries | Replay | Load (decode and replay) |
| --- | ---: | ---: | ---: |
| cumulative | 1,000 | 0.48 ms | 2.65 ms |
| cumulative | 10,000 | 4.46 ms | 25.6 ms |
| cumulative | 100,000 | 54.0 ms | 263.8 ms |
| delta | 1,000 | 0.62 ms | 2.68 ms |
| delta | 10,000 | 5.50 ms | 27.3 ms |
| delta | 100,000 | 77.8 ms | 294.9 ms |
| mixed | 1,000 | 0.91 ms | 3.05 ms |
| mixed | 10,000 | 40.8 ms | 62.3 ms |
| mixed | 100,000 | 3.79 s | 4.19 s |
| reserve and hold | 1,000 | 1.74 ms | 3.29 ms |
| reserve and hold | 10,000 | 62.6 ms | 73.4 ms |
| reserve and hold | 100,000 | 4.91 s | 7.55 s |

Decoding is linear: 2.1 µs per entry, or 3.8 to 4.0 µs when the digest is recomputed as well (`/tmp/f3-5a/probe2/run.log`).

**Replay is quadratic in the last two shapes.** Three reducer steps scan the whole unknown list: a hold, a settle, and an accepted cumulative running total. A hold costs about 2 ns per unknown item (20 µs at 10,000 items, 114 µs at about 60,000); a running total costs about 4 ns per item (`/tmp/f3-5a/probe/step.log`). This code is from step 2. 5a did not change the scans; it makes the list longer by the one item of each unsized reservation, and its benchmarks are the first to show the cost.

In the first package only the hold matters: usage arrives as delta events without corrections (§11), which never scan, and nothing settles. Each ended attempt pays one scan of everything unknown so far. A root with T attempts that leave U unknown items each therefore spends about U × T² ns in scans during a full replay. For reserve and hold (U = 2) that predicts 50 ms at 5,000 attempts and 5 s at 50,000, which matches the 63 ms and 4.9 s measured.

**When it is paid.** Once per process and root when nothing is cached (§10.4), inside the finalize transaction while the root row is locked, and by a `show` that falls back to a full replay. A load of the tail pays only the per-step cost.

### 5.4 `Projection()`

- Cumulative and delta shapes: 0.25 µs, 2.9 µs and 26.5 µs at 1,000, 10,000 and 100,000 entries, with no allocation.
- Mixed shape: 5.3 µs, 41.5 µs and 417 µs. The call copies the unknown list, 80 bytes per item: 0.04, 0.4 and 4.0 MB.
- Reserve and hold (one iteration): 8.3 µs, 99.6 µs and 1.85 ms, copying 0.08, 0.8 and 8.0 MB.

### 5.5 Memory and stored size per entry

**Live heap per entry after a load** (bytes; the heap is measured after two garbage collections, before and after the load). The first three rows are three identical runs each (`/tmp/f3-5a/bench1/memory.log`); the last row is one run.

| Shape | 1,000 entries | 10,000 entries | 100,000 entries |
| --- | ---: | ---: | ---: |
| cumulative | 960 | 887 | 827 |
| delta | 1,322 | 1,202 | 1,106 |
| mixed | 1,209 | 1,096 | 1,021 |
| reserve and hold | 945 | 869 | 803 |

**Stored size per entry** (bytes of JSON, 10,000 entries, 36-byte identifiers; `/tmp/f3-5a/probe2/run.log`): cumulative 698 on average (largest 712), delta 693 (706), mixed 736 (785). The probe did not run for the reserve-and-hold shape; computed from the pinned form, its entries are 601 bytes (reserve) and 611 bytes (hold). With the identifiers of §10.2, an unsized reserve entry is 609 bytes, a reserve sized at 10 USD is 621, and a hold is 619 to 643 depending on its reason (`/tmp/f3-5a/stored_sizes.py`).

**For the cache cap of §10.4:** count 0.8 to 1.3 KB of live heap per cached entry, so 100,000 cached entries are 80 to 130 MB. Two things come on top. Each write allocates one more copy of that root's state (section 5.2). And usage entries with the §10.2 identifiers are longer than the benchmark's: the entry ID is up to 200 bytes instead of 36 and the scope ID 73 instead of 36, which I estimate at 100 to 200 bytes more per usage entry. That estimate is not measured.

### 5.6 Cheaper structures (proposals only, nothing built)

The 5 ms rule does not ask for any of these. I list them because the measurements point at them.

1. **Index the unknown list.** Keep, per reservation, which lifecycle reasons are already recorded and how many items block its settlement, so that a hold and a settle stop scanning; find the items of one entry or one reservation through an index instead of a pass over the list. Expected effect: a hold step goes from 20 to 114 µs down to about 1 µs, and a full replay of 100,000 entries goes from 3.8 to 4.9 s down to the 55 to 80 ms of the shapes that do not scan. The stored form, the receipts and the projection do not change. This is the only one of the four that removes a quadratic cost.
2. **Share immutable entry records.** Keep each entry with its receipt as one immutable record behind a pointer and copy a meter scope only when it is written to. Measured building blocks (`/tmp/f3-5a/probe/shared.log`): cloning a map of 10,000 pointers takes 46 to 60 µs and 0.44 MB, against 0.87 ms for the two value maps today; at 100,000 it takes 0.34 to 0.37 ms. Expected `With`: 0.1 to 0.25 ms at 10,000 entries and 0.8 to 1.5 ms at 100,000, with about a tenth of today's allocation.
3. **Persistent maps.** `With` becomes independent of the ledger size (a few µs) at the price of more code and slower reads. Not justified at these sizes.
4. **Figures without the unknown list.** An accessor that returns A, E, R, H, overrun and the unknown counts without copying the list, for callers that do not need the items.

## 6. Deviations and open questions

### 6.1 Deviations from design.md §10

None. The rules that apply to 5a are implemented as written: the unsized reservation (§10.1, third item), the apply method that does not modify its receiver (§10.4), `HasReservation` (§10.5) and the 5a item of §10.13. No rule turned out wrong or impossible against the code.

### 6.2 Open questions for the main session

Questions 1 to 4 need an answer. Questions 5 to 8 are things 5b and step 6 should know; they need no answer now.

1. **Replay cost while nothing settles (section 5.3).** A full replay is quadratic in the number of attempts as long as nothing settles, and a cold load runs inside the finalize transaction with the root row locked. It shows only on large roots: 63 ms at 5,000 attempts and 4.9 s at 50,000 with two unknown items per attempt. Usage with an estimated or absent cost adds one unknown item per entry and moves the point earlier. The options:
   - (a) Accept it for the first package and record the limit.
   - (b) Index the unknown list (section 5.6, proposal 1) before step 6.

   I recommend (b) as a small step of its own with its own check, because it changes step 2 code that has already passed an independent check. Neither option blocks 5b.
2. **Version of the stored form.** The form of every entry changed, yet `issue_budget_entry.schema_version` is written as 1 by the step 3 tests and the digest domain is still `budget-entry-v1`. I kept both, because migration 866 is not committed and no stored entry exists outside test databases. Please confirm, or say that the new form is version 2. If it is 2, the places that write 1 today are `internal/handler/issue_budget_queries_test.go:177` and `:192` and `internal/handler/issue_budget_schema_test.go:385` and `:608`.
3. **Choices where the design is silent.** They are rules 2, 3, 4, 8, 9 and 11 of section 3, and four in section 1.1: `With` with no entries returns an independent copy; `With` returns `(nil, nil, err)` on error; `Len` counts entries kept only as a record; `HasReservation("")` is false. A test pins each, so changing one means changing its test. The two I would look at first:
   - Rule 2: an unsized entry with a negative amount is `ErrInvalidEntry`, not `ErrNegativeAmount`.
   - Rule 9: an ended unsized attempt contributes two unknown items. The `unbounded_unknown` detail of §10.2 lists counts by reason, so it will show both.
4. **Concurrency contract of `Ledger`.** The reworded doc comment (section 1.2) is now a promise the cache relies on. Please confirm the wording. Related: a cached `*Ledger` still has `Apply`, so "never modified in place" (§10.4) is a convention that the type does not enforce. A read-only type would enforce it (section 6.4, proposal 6). I did not add one because the design does not ask for it.
5. **Where the cache cap lives.** §10.4 calls the cap a package constant and asks the implementer for the memory per entry. I delivered the measurement (section 5.5) and defined no constant, because the cache is 5b's. If the constant belongs in `internal/budget`, name the value and I or 5b can add it.
6. **Two hold entries with the same ID must have the same content.** §10.2 fixes the hold ID as `hold/<attempt id>/<reason>`. In the package the same ID with other content is `ErrConflict`, and `Detail` is content. If two writers record the same reason for the same attempt (two reconciliation rounds, or finalize and reconciliation), their entries must be identical including `Detail`; otherwise the second batch fails instead of being a replay. Server-written holds should leave `Detail` empty or derive it only from the attempt and the reason.
7. **Replays and the ledger revision.** `Len()` equals `ledger_revision` (the `seq` of the last row, §10.2) only if the writer inserts exactly one row for every entry whose receipt has `Replayed` false. A replayed entry must not get a row. An entry kept only as a record (`stale`, `quarantined`, `comparison`) must get one, because it is part of the replay.
8. **Gap entries that name a reservation.** A gap entry may carry a reservation ID. Its unknown item then blocks the settlement of that reservation for good: this is a step 2 rule, and nothing removes a gap item. It has no effect in the first package, but it decides what S2 can release. §10.2 defines gap entries per attempt and invocation for step 6, so step 6 should choose on purpose whether they name the reservation.

### 6.3 Changes to design.md while I worked

I read §10 at about 17:25, when the file had 451 lines. At 18:59 it had 707 lines (changed at 18:42), and it changed once more at 19:36 (708 lines, 105,970 bytes). At 21:00 I re-read §10.1 to §10.5 and §10.8 to §10.13 in the current text, with §11.7 and §11.8. I compared by reading, not with a diff. I did not re-read §1 to §9, §10.6, §10.7, the rest of §11 or §12 after 18:59.

**No change alters a 5a rule.** The unsized item of §10.1 and the 5a item of §10.13 read as they did. These changes touch what 5a delivers:

1. **§10.3 and §11.8: no production path settles in the first package.** Holds and unsized items therefore stay, and the unknown list grows with every attempt. This is why I added the reserve-and-hold benchmark shape, and it is the ground of question 1.
2. **§10.2 supplement: entry IDs in fixed formats, up to 200 bytes.** The package puts no limit on the length of an ID. My benchmarks use 36-byte IDs, so their memory and size figures are low for usage entries (section 5.5).
3. **§10.3 hold reasons** (`process_exit`, `cancelled`, `timeout`, `lost_contact`, and `superseded_unproven` in §10.2 step 7). All five exist in the package as `HoldReason` constants. Nothing to add.
4. **§10.4, last item: a `show` without the root lock may replay in full from its snapshot.** That path pays the load times of section 5.3.
5. **§11 (read at 18:59): usage entries are delta events with sequence 0 in the scope `<attempt id>/<invocation id>`.** The package accepts that form, and the delta benchmark shape matches it.
6. **§10.8: the price snapshot and its digest move to the entry table, under the domain `budget-price-snapshot-v1`.** That is the domain `PriceSnapshot.Digest()` already uses. No change in the package.

The other changes I saw (limit and reserve given together at enable in §10.1, the wider 5b and 5c items of §10.13, §12) are outside 5a.

### 6.4 What the finalize integration still needs from this package (proposals only)

The dispatch asked for this list. Nothing here is built.

1. **A strict decoder for a stored entry**, for example `DecodeEntry(raw []byte) (Entry, error)`. It should refuse unknown keys, trailing data and any form that does not marshal back to the same bytes. Today `json.Unmarshal` accepts a row in the old form, a row with extra keys and a row with keys in another order. The database proves that the digest matches the stored bytes, not that the bytes are the form of the entry they decode to; §10.5 wants an entry that cannot be decoded to make the ledger unavailable. Cost: I estimate 1.5 to 2 µs per entry for marshalling it back, on top of the 2.1 µs of decoding.
2. **A constant for the version of the stored form**, for example `EntryFormVersion`, so that the writer and the loader do not each write 1 (question 2).
3. **The state of one reservation**: open, held or settled, whether it is unsized, its reserved amount and its known actual spend. Today the only question the package answers about a reservation is `HasReservation`. 5b needs more only if it wants to cross-check the attempt table against the ledger or to show figures per attempt in step 7.
4. **Builders for the entry IDs of §10.2** and the 200-byte limit as a constant, if the main session wants them next to the ledger. If they live in the service, the package needs nothing: it only requires a non-empty ID.
5. **A size limit.** The package caps neither `Detail` nor an ID, while the table caps `entry_id` at 200 bytes and `entry_json` at 65,536 bytes. An entry with a `Detail` of about 64 KB passes the reducer and then fails the insert, which fails the whole finalize transaction. Either the writer checks sizes before it applies, or the package gets a rule; a rule in the package would need a line in the design.
6. **A read-only view of a ledger**, a type without `Apply`, for cached states (question 4).
7. **Figures without a copy of the unknown list** (section 5.6, proposal 4).

**Already sufficient, as far as I can see:**

- Loading the tail: `With`.
- Checking the revision: `Len`.
- Attempts without a reservation (§10.5): `HasReservation`.
- The `limit` detail: A, R and H are fields of `Projection`, and A+R+H is `Committed()`.
- The `unbounded_unknown` detail: `Projection.UnknownCounts()`.
- The index columns of the entry table (kind, attempt, scope, invocation): they are fields of `Entry`.

## 7. Housekeeping

### 7.1 Files created outside the package

- This report.
- Everything else is scratch under `/tmp/f3-5a/`, which does not survive a reboot:
  - `baseline-budget/` (the package before my changes), `baseline.log` and the hash lists;
  - the red and green logs (`red*.log`, `green*.log`), `vet*.log` and `final/`;
  - `digests.py` with `digests.out`, and `stored_sizes.py`;
  - `mutate.py`, `mutations*.out`, `mut/`, `mut-extra/` and `mut-w16/`;
  - `bench1/`, `bench2/`, `bench-smoke*.log`, `run-bench.sh` and `summarize_bench.py`;
  - `probe/` and `probe2/` (probe test files), `nofloat/` (the type-level check and its control);
  - `old-tests/` and `copyback/` (overlays and their logs), `diff-*.patch` and `verify.sh`.
- The probe test files and every mutated source were compiled from `/tmp/f3-5a/` through `go test -overlay`. None of them was ever written into the package directory.
- **Added at 22:55, after this report was first delivered at 53,029 bytes:** one note in the project's Claude memory directory (`~/.claude/projects/<project>/memory/`, outside every repository): the new file `long-report-in-one-request-times-out.md` and one index line appended to `MEMORY.md` there. It records that a long report is written in pieces. This bullet is the only change to the report since delivery; the copy as delivered is `/tmp/f3-5a/report-as-delivered.md`.

### 7.2 What was not touched

- **Scratch worktree:** only `server/internal/budget/`. `git status --short` prints `?? server/internal/budget/` and nothing else; HEAD is still `86fea651cc5d09360327b6cf0f8357c1f360070d`.
- **Step 4 worktree** (`/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`): read-only commands (`ls`, `grep`, `sed -n`, `shasum`, `git status`, `git rev-parse`) and the one `go vet -overlay` of section 2, which writes nothing into the tree. The hashes of its `internal/budget` are the same now as before I started (`/tmp/f3-5a/step4-budget-sha256.txt` and `/tmp/f3-5a/step4-budget-sha256-now.txt`).
- **Shared checkout:** I wrote this report and nothing else.
- **Nothing was committed, stashed, pushed or merged**, and no branch was switched. I used no database, ran no agent CLI and made no provider call.

### 7.3 Runs made after the freeze

You asked for no new long runs and for the existing logs to be cited. I made four short runs anyway, between 21:04 and 21:46. None of them writes into the package.

1. **`BenchmarkWith` for the reserve-and-hold shape** (16 s and 22 s). The only figure on record was one cold iteration, 4.45 ms at 10,000 entries, which is too close to the 5 ms threshold to report as the answer. The timed runs give 1.28 ms.
2. **Mutant W16** (about one minute, through an overlay). A comment in `with_test.go` claims what this mutant checks, and it had not been run.
3. **The type-level floating-point check on the final bytes** (seconds), because the last edit was to the benchmark file.
4. **`go list -test -deps`** (seconds). It showed the import in `scale_test.go` that section 4.2 reports.

### 7.4 Not verified

- The full test, `-race` and build runs on the final bytes, after the edit of 18:52 (section 4.1). The main session is running them.
- The database-backed tests of the step 4 tree against the new package. Only the type-check ran.
- The benchmarks ran on a loaded, shared machine with 36-byte identifiers. For the reserve-and-hold shape only `With` has timed runs; its replay, load, projection and memory figures are single iterations.
- No cache exists yet, so nothing is measured under concurrent use or under garbage-collection pressure.
- Everything ran on darwin/arm64 only.
- The mutants are hand-written and each ran once.
- The parts of design.md listed in section 6.3 were not re-read after 18:59.
- A nil `*Ledger` is not supported. That is unchanged and I checked it by reading only.

### 7.5 sha256 of the 15 files in `server/internal/budget/`

Taken when this report was finished. They equal the list recorded at 20:41 in `/tmp/f3-5a/final/sha256-final.txt`.

| File | Bytes | sha256 |
| --- | ---: | --- |
| `bench_test.go` | 7,655 | `4d1a50a76ce8680b3bd8c6e8809e21f67f92e617078e65e0f3b8352f2cffad71` |
| `decimal.go` | 2,451 | `6db8462df4606ab587b1ab9e50a8b5ee0eade386927bb5893e9f80ebc3a5f26a` |
| `decimal_test.go` | 3,945 | `2c95daf7225b4142b29926f45344c4aea10cab0bbd9718fd1c6f91071acb4db4` |
| `edge_cases_test.go` | 15,264 | `8ff64ed04f05cd43425406ef2c94760f9d1f2b43eb129b02b3cc590c77745320` |
| `invariants_test.go` | 14,396 | `d302dc61546c8a20e9e820545d9df355b5aaf1ffea7776ca6eed1211359eba6c` |
| `ledger.go` | 15,238 | `77669832b4d8d070d7c79f478473f4ed6497bdc2f76b91af7598931ac5c33052` |
| `ledger_test.go` | 23,544 | `980f783d59abe82db048c89fc34c19daa1e8ea9e5ee8e3643f01a4abb8251627` |
| `model.go` | 10,157 | `ec9faca798fa8939bbfd889a7782c6c6a349aa21cc09cffaa2ee99b4b96fe877` |
| `price.go` | 6,572 | `a2b4cd68128b4877d39620b61419973b0f6dec0d254e683c837b86125379b31a` |
| `price_test.go` | 12,033 | `b57dd9df6dd8d072ac2a50f762cc294a684b00e29e81b8b4c225cfbc12005646` |
| `scale_test.go` | 562 | `aaac212c787f403c89670c0dbb18287380440244d561badada6f23f7e82473c2` |
| `unsized_test.go` | 21,308 | `a47777c8e2c82466f0310ff91531680dbe8ea79c6e557d8d4b8592695b2c2a48` |
| `usage.go` | 11,418 | `90e000c435e29e47029abf1d05900c12387c004becd118749d7c3b99250ed2d9` |
| `usage_test.go` | 18,548 | `e323602f135a9d73f87fc561c60b8c5dde732ce1a4c50abc983cc8099e4399a2` |
| `with_test.go` | 18,827 | `1f742fac04f12f2cfe1db1c3fed0c285c1794931c793087fcee27d4073ee4f8c` |

## Appendix A: the mutants

One row per mutant. "Red" is the number of tests that fail without `-race`; "Red with -race" is the number that fail with `-race`, which only the mutants of `With` were run with. A note beside a number is what the run reported in addition: a panic, the runtime's fatal error for concurrent map access, or data races from the detector.

| ID | What the mutant does | Red | Red with -race |
| --- | --- | ---: | --- |
| U01 | an unsized reserve with an amount is accepted | 2 |  |
| U02 | a negative amount is judged before the unsized flag | 1 |  |
| U03 | the flag is accepted on a usage entry | 2 |  |
| U04 | the flag is accepted on a hold entry | 2 |  |
| U05 | the flag is accepted on a settle entry | 2 |  |
| U06 | the flag is accepted on a gap entry | 2 |  |
| U07 | the flag is accepted on an exclusion entry | 2 |  |
| U08 | the flag is refused on a reserve entry | 18 |  |
| U09 | an unsized reserve adds no unknown item | 10 |  |
| U10 | every reserve is treated as unsized | 25 |  |
| U11 | the unknown item lacks the reserve entry's ID | 4 |  |
| U12 | the unknown item lacks the reservation ID | 8 |  |
| U13 | the unknown item carries another reason | 16 |  |
| U14 | the receipt of an unsized reserve has no reason | 4 |  |
| U15 | the reservation is not remembered as unsized | 8 |  |
| U16 | a replayed unsized reserve adds its item again | 3 |  |
| U17 | settle counts the unsized item as unknown usage | 13 |  |
| U18 | settle of an unsized reservation ignores its unknown usage | 1 |  |
| U19 | settle leaves the unsized item in the unknown set | 8 |  |
| U20 | settle removes the unsized items of other reservations | 3 |  |
| U21 | settle of an unsized reservation needs no proof | 1 |  |
| U22 | a lifecycle hold removes the unsized item | 3 |  |
| U23 | a hold on an unsized reservation is ignored | 3 |  |
| U24 | spend on an unsized reservation is overrun | 8 |  |
| U25 | a sized reservation of zero is treated as unsized | 4 |  |
| U26 | one open unsized reservation hides every overrun | 3 |  |
| U27 | an unsized item alone leaves the total known | 7 |  |
| U28 | an unsized reservation is not an included attempt | 4 |  |
| U29 | spend on an unsized reservation is not counted | 8 |  |
| U30 | the unsized item is put in front of the unknown list | 1 |  |
| E01 | unsized is omitted from the stored form when false | 2 |  |
| E02 | unsized is stored under another key | 2 |  |
| E03 | unsized is not stored | 2 |  |
| E04 | unsized is stored at the end of the entry | 2 |  |
| E05 | the entry digest uses another domain | 2 |  |
| E06 | the flag is ignored when an entry ID is seen again | 1 |  |
| W00 | With applies to its receiver unless it is the zero Ledger | 8 (concurrent map access) | 11, data race reported |
| W01 | With applies to its receiver | 1 (panic) | 1, panic |
| W02 | With without entries returns its receiver | 1 | 1 |
| W03 | the result shares the entry map | 11 | 11, data race reported |
| W04 | the result shares the receipt map | 5 (concurrent map access) | 5, data race reported, concurrent map access |
| W05 | the result shares the reservation map | 9 | 9, data race reported |
| W06 | the result shares the contribution map | 5 | 5, data race reported |
| W07 | the result shares the exclusion map | 6 (concurrent map access) | 6, data race reported, concurrent map access |
| W08 | the result shares the unknown list | 6 | 6, data race reported |
| W09 | the result shares the scope map | 7 | 7, data race reported |
| W10 | the result shares each scope | 6 | 6, data race reported |
| W11 | the result shares each scope's sequence set | 6 | 6, data race reported |
| W12 | the result shares each scope's attribution | 5 (concurrent map access) | 5, data race reported, concurrent map access |
| W13 | a failed With returns the ledger it got to | 2 | 2 |
| W14 | a failed With returns the receipts it got to | 1 | 1 |
| W15 | Apply does not take the ledger With returns | 70 | 70 |
| W16 | With writes to its receiver for a moment and puts the value back | 1 | 1, data race reported |
| A01 | Len counts a replay | 6 |  |
| A02 | Len leaves out entries kept as a record only | 2 |  |
| A03 | Len counts reservations | 5 |  |
| A04 | Len is one too many | 4 |  |
| A05 | HasReservation forgets a settled reservation | 1 |  |
| A06 | HasReservation forgets a held reservation | 1 |  |
| A07 | HasReservation forgets an unsized reservation | 2 |  |
| A08 | HasReservation looks up entry IDs | 3 |  |
| A09 | HasReservation is true for the empty ID | 1 |  |

## Addendum: constant-time hold check

Written on 2026-11-18, after the report above was delivered. It answers the main session's message "5a follow-up: make the duplicate-hold check constant time". Nothing above this heading was edited: the first 53,504 bytes of this file still have the sha256 `c1cffb972e1d3d8226d37c1354265b461e753a8f76d1acc676d973aad2cc40ec`. Where this addendum and the report disagree, the addendum is the later state; section H.9 lists those places.

Sections: H.1 summary, H.2 the change, H.3 why the field and the scan agree, H.4 tests, H.5 mutants, H.6 old code against new code, H.7 verification runs, H.8 benchmarks, H.9 what this supersedes, H.10 proposals, H.11 housekeeping.

### H.1 Summary

- **The change.** A reservation now carries the set of hold reasons recorded for it: one `uint8`, one bit for each reason. `applyHold` reads that set where it searched the unknown list, and `hasUnknown` is gone. Among the non-test files only `ledger.go` changed (34 lines added, 8 removed). Nothing outside `ledger.go` and the test files was needed.
- **Behaviour is the same**, shown in three ways. By reading: H.3 has the argument with line references, and I agree with your reading of the three other places that drop unknown items. By 10 new tests, among them the differential property test you asked for (H.4). By two transcripts of 600,000 reducer steps that the old and the new code write byte for byte the same, with controls in which a wrong field changes them (H.6).
- **Tests:** 99 pass, 0 fail, 0 skipped, with and without `-race`. `gofmt`, `go vet` and `go build ./...` are clean, and the step-4 importers type-check against the 17 files (H.7). The 89 earlier tests are unchanged and green.
- **Mutants:** 38 new ones for the field, 100 in all. The whole suite catches 99. The hundredth, H31, does not compile against `holdset_test.go` and is caught without it. Two of the 38 change no receipt and no unknown item, and only the two tests that read the field catch them (H.5).
- **Full replay of the held shape** at 1,000, 10,000 and 100,000 entries: 0.81 to 0.85 ms, 48 to 49 ms and 4.6 to 4.8 s before; 0.39 to 0.40 ms, 3.4 ms and 44 to 46 ms after. **Load:** 2.7 to 2.8 ms, 68 to 76 ms and 5.1 to 6.0 s before; 2.3 to 2.5 ms, 23 to 24 ms and 238 to 249 ms after. These are the medians of three runs in each of three before-and-after pairs, at a load average of 2.8 to 9.7 (H.8).
- **Unchanged:** the cumulative, delta and mixed shapes, one `With` on a cached root, `Projection()` and the memory per entry. The mixed shape is still quadratic, 3.7 s at 100,000 entries. It has no hold entry, and you told me to leave its scans alone (H.10, item 3).
- **Two notes for 5b** (H.10). A replay must give all its entries to one `With` call: one call per entry costs 4.3 s at 10,000 entries. A root above the cache cap pays 2.3 to 2.5 µs per entry on every load.
- **Not started:** the other three items of the second round in design §10.13. The message that orders them has not reached me (H.11).

### H.2 The change

Among the non-test files only `ledger.go` changed: 34 lines added, 8 removed (`/tmp/f3-5a/hold/final/ledger.diff`). Line numbers below are those of the new file.

- **The field.** `reservation` has a new field `holds holdSet` (`ledger.go:28-33`). `holdSet` is a `uint8` (`:38`). `holdBit` gives each of the five `HoldReason` constants one bit (`:41-55`); a string that is not a hold reason has none.
- **The check.** `applyHold` (`:340-352`) now reads `r.holds&bit != 0` where it called `hasUnknown` (`:344`). When it accepts the hold it sets the bit (`:348`) in the same step that stores the reservation and appends the unknown item (`:349-350`).
- **Removed.** `hasUnknown` had no other caller and is gone (`grep -rn hasUnknown internal/` finds nothing).
- **Not touched.** The loop in `applySettle`, `dropUnknown`, the copy in `Projection` and `clone` are as they were. `clone` needed no edit: it copies the reservation map with `maps.Clone` (`:122`, `:129`), and a reservation still holds plain values only, so the copy shares nothing.
- **Size.** `unsafe.Sizeof(reservation{})` is 24 bytes before and after; the new byte sits where padding was (`/tmp/f3-5a/hold/probe/size-old.log`, `size-new.log`).
- **A settled reservation keeps its bits.** Nothing reads them after the settlement, because the state is checked first (`:344`).
- **Unchanged:** the exported API, the stored form and the digests. The other 14 files of the report's state are byte-identical: `model.go`, `usage.go`, `decimal.go`, `price.go` and the 10 test files (H.11).

### H.3 Why the field and the scan agree

You asked me to verify this myself. I read every place that adds or removes an unknown item and every place that stores a reservation.

**The claim.** For a reservation that has not settled, the unknown list has an item with a given reason exactly when the reservation's set has the bit of that reason. For a settled reservation the old check and the new one both answer from the state before they look any further (`ledger.go:344`).

**What adds an unknown item** (every call of `addUnknown`, `ledger.go:429`):

| Where | Reason of the item | A hold reason? |
| --- | --- | --- |
| `ledger.go:350`, `applyHold` | the reason of the hold entry | Yes. The bit is set two lines above (`:348`), on the same reservation. |
| `ledger.go:225`, gap entry | `coverage_gap` or `pre_activation_unknown`; `validateEntry` refuses any other (`:283`) | No |
| `ledger.go:331`, unsized reserve | `reservation_unsized` | No |
| `usage.go:170`, `quarantine` | `scope_ambiguity`, `sequence_conflict`, `correction_unresolved` or `sequence_regression` (callers at `:152`, `:192`, `:201`, `:207`, `:213`, `:218`, `:223`, `:229`, `:232`, `:238`, `:286`) | No |
| `usage.go:175`, `addCostUnknown` | what `classifyCost` returns (`:120-134`): `cost_absent`, `unsupported_currency`, `estimate_only` or `zero_without_basis` | No |

So only an accepted hold puts a hold reason into the list, and it records the bit in the same step.

**What removes an unknown item** (every call of `dropUnknown`, `ledger.go:431`):

| Where | Which items | Can it be a hold item of a reservation that has not settled? |
| --- | --- | --- |
| `ledger.go:377`, settlement | every item of the reservation being settled | No. The same entry marks that reservation settled (`:372-373`). Items of other reservations do not match. |
| `usage.go:253-255`, reported running total | items of the entry's meter scope that have a cost-quality reason (`isCostQualityReason`, `:136-142`) | No. A hold item has a hold reason. It also has no scope, and a usage entry must name one (`:69-71`). |
| `usage.go:259`, estimated correction of a running total | the item whose entry ID is the head of the scope (`sc.headID`, set only at `:273`, to the ID of an accepted usage entry) | No. A hold item carries the ID of its hold entry, and one ID cannot belong to two entries: `applyOne` answers an ID it already holds with a replay or with `ErrConflict` before anything else (`ledger.go:198-205`). |
| `usage.go:295`, correction of a delta event | the item whose entry ID is `u.Supersedes`, which is a key of `l.contributions` there (`:281`, `:285`); a key is added only at `:321`, with the ID of an accepted delta entry | No, for the same reason. |

These are the settlement and the three places you named. I agree with your reading of each.

**What stores a reservation** (every assignment to `l.reservations[…]`; there are four, listed in `/tmp/f3-5a/hold/final/grep-writers.out`):

| Where | What happens to the set |
| --- | --- |
| `ledger.go:325`, `applyReserve` | A new reservation with an empty set. It never replaces one: an ID that exists is refused first (`:317-319`). No hold item can be waiting for the new ID, because an entry of any other kind that names a reservation is refused until that reservation exists (`:206-210`). |
| `ledger.go:349`, `applyHold` | Reads the whole value (`:341`), sets the state and the bit, stores it. `:348` is the only line that writes `holds`. |
| `ledger.go:373`, `applySettle` | Reads the whole value (`:360`), sets the state, stores it. The set comes along unchanged. |
| `ledger.go:416`, `addActual` | Reads the whole value (`:404`), changes `actual`, stores it. The set comes along unchanged. |

**Copies, refusals and replays.**

- `clone` copies the reservation map by value (`:122`, `:129`), so each ledger that `With` returns has its own sets. `Apply` is `With` followed by replacing the receiver (`:145-149`).
- A refused batch leaves neither a bit nor an item. `With` applies the batch to the clone and returns nil at the first error (`:159`, `:163-165`).
- A replayed entry returns at `:198-205`, before `applyHold` runs. A stale hold returns at `:345`, before either the bit or the item is written.

**Both directions**, for a reservation X that has not settled and a reason R:

- *The bit is set, so the item is there.* The bit is set only at `:348`, by an accepted hold of X with reason R, and the same step appends the item (`:350`). Only `:377` can take that item out, and only when X settles.
- *The item is there, so the bit is set.* An item with a hold reason for X comes only from `:350`, after `:348-349` stored the bit on X. Nothing clears a bit, and every later store of X carries the set.

So at `:344` the new operand is evaluated only for a reservation that has not settled, and there it has the value `hasUnknown(X, R)` had. The lines after the check differ only by the bit. The receipts, the unknown items and their order are therefore the same. An entry's stored form and digest never involved the reservation (`Entry.Digest`, `:464-465`).

One condition sits under this: each of the five reasons needs a bit of its own that is not zero. A reason without a bit would never be stale. `validateEntry` admits only the five constants for a hold (`:274-278`), `holdBit` names the same five (`:41-55`), and `TestEveryHoldReasonHasItsOwnBit` reads the declared constants from the source and fails if the lists drift apart.

**Checked by running as well.**

- A probe walked the 300 histories of the property test and classified every entry that took an unknown item out (`/tmp/f3-5a/hold/probe/removals_test.go`, given to the compiler through an overlay; logs `removals-old.log` and `removals-new.log` are identical). A settlement took items out 289 times and removed 345 hold items, every one of them from the reservation it settled. A reported running total took items out 11 times, 10 of them while hold items were in the list, and removed no hold item. No correction removed anything in those histories.
- That last line was a gap in my own test: its comment claimed corrections remove no fact, and its histories never made a correction remove an item. I closed it with a directed test for each remover (H.4), four mutants in which a remover takes a fact out (H.5) and a second old-against-new transcript that is dense in corrections (H.6). In that transcript the removers took items out 24,709 times, 23,377 of them beside a lifecycle fact that had to stay, and took none of those facts.

**What would break it later.** A new entry that removes a hold item of a reservation that has not settled, or a new producer of an item with a hold reason, must update the set too. The comment on the field says so (`:28-33`). `TestHoldSetIsWhatTheAcceptedHoldsRecorded` compares every set with the unknown list after each accepted entry, and the differential test predicts each disposition from a search of the list, so both fail when the two drift apart inside their histories.

### H.4 Tests

Two new test files, 10 tests. The 10 test files of the report's state are byte-identical (H.11), and the 89 earlier tests all pass in the final run: no name that passed before is missing, and the 10 names that are new are the 10 below (`/tmp/f3-5a/hold/final/tests-compared.txt`).

**The order of work.**

1. `hold_test.go` first, run on the old `ledger.go` before anything changed (2026-11-17 23:46 to 23:59, `/tmp/f3-5a/hold/red/behaviour-on-old.*.log`).
2. `holdset_test.go` next. At 00:06 both files ran against the old code and against a stub (`red/`, with the hashes of what ran in `red/inputs.sha256`).
3. `ledger.go` changed at 00:09:13. First green run at 00:10 (`green/test-first.log`).
4. Two revisions after that. At 00:14 I put the reach counts and their floors into `holdset_test.go`; they are known only from a passing run. At 02:26 I added two tests and three counters to `hold_test.go`, when the probe in H.3 showed the gap. Those two tests were written after the code. They pass on the old code as well, and what shows they can fail is the mutants in H.5, not a red run.

The three stages were then repeated with the final bytes of both files (`/tmp/f3-5a/hold/run-red.sh`, 02:44:42 to 02:44:47, load average 4.46 to 4.43):

| Stage | Test files | `ledger.go` | Result | Log under `/tmp/f3-5a/hold/red-final/` |
| --- | --- | --- | --- | --- |
| a | `hold_test.go` | before the change | exit 0, 97 PASS, 0 FAIL, 0 SKIP | `a-behaviour-on-old.test.log` |
| b | both | before the change | exit 1, does not build: `undefined: holdSet`, `undefined: holdBit`, `r.holds undefined` | `b-both-on-old.test.log` |
| c | both | stub: the names are declared, `holdBit` returns 0, the check still calls `hasUnknown` | exit 1, 97 PASS, 2 FAIL (`TestEveryHoldReasonHasItsOwnBit`, `TestHoldSetIsWhatTheAcceptedHoldsRecorded`) | `c-both-on-stub.test.log` |

Stage a is what `hold_test.go` is for. It uses no new identifier, and the same bytes pass on the old code and on the new. A test of behaviour that must not change cannot be red first; H.5 is where these tests are shown to fail.

**`hold_test.go`: behaviour only (8 tests).** They read receipts, the unknown list and projections, never the field.

| Test | What it pins |
| --- | --- |
| `TestHoldDispositionMatchesASearchOfTheUnknownList` | The differential property test you asked for; described below. |
| `TestEachHoldReasonIsRecordedOnceForAReservation` | All five reasons, in each of the five rotations of their order, on a sized and an unsized reservation. The first of each is accepted with the exact receipt. Every reason on record is stale when it comes again under another entry. The exact unknown list after each step. |
| `TestHoldReasonsAreRecordedPerReservation` | What one reservation has on record says nothing about another: a sized one, an unsized one, and a third opened when the other two have every reason. 16 unknown items at the end, each reason three times. |
| `TestHoldAfterSettlementIsStaleWhateverWasRecorded` | After a settlement all five reasons are stale, on a reservation that recorded one before and on one that recorded none. A neighbour that has not settled is not affected. |
| `TestRecordedHoldReasonsFollowWith` | A ledger made by `With` knows what its origin recorded. What it records reaches neither the origin nor a second ledger made from the same origin. The zero `Ledger` records as any other. |
| `TestRefusedBatchAndReplayLeaveTheRecordAlone` | A refused batch (through `Apply` and through `With`) records nothing, so its reason is a new fact afterwards. A replay returns the first receipt. |
| `TestUsageRemovesNoLifecycleFact` | One row for each way usage takes items out: a reported running total, a reported and an estimated correction of a running total, a reported and an estimated correction of a delta event. Each row on a sized and an unsized reservation, with the fact of a second reservation beside it. The exact unknown list before the entry, after it and at the end. The recorded reason is stale afterwards by the receipt and by a search of the list; another reason is still accepted. |
| `TestGapEntryCannotCarryALifecycleReason` | A gap entry with any of the five reasons is refused, for a reservation and for none, and the hold after it is still a new fact. |

**The differential property test.** 300 seeds of 80 steps.

- *Generator.* `randomEntryWithHolds` is the report's `randomEntryWithUnsized` (every entry kind, sized and unsized reservations) with a third of the entries that are not reservations replaced by a hold. Four reservations and five reasons leave 20 facts to record, so reasons soon repeat: on the reservation that has them, on its neighbours and after settlements.
- *Prediction.* Before each hold, `wantHold` searches the unknown list the ledger shows for the entry's reservation and reason. Whether the reservation has settled is taken from the history's accepted settle entries, not from the ledger. The receipt must equal the predicted one, and the unknown list afterwards must equal the predicted list, item for item and in order.
- *After every accepted entry*, the list must show for each of the four reservations exactly the reasons its accepted holds recorded, in that order, and none once it has settled.
- *`With` chains.* The ledger under test is a chain of states, each made by `With` from the one before. A second ledger takes the same entries through `Apply`, and the two must give the same receipts. After each step a state of the chain, picked at random, is asked about a hold of its own: the answer must agree with a search of that state's list, and the state must be unchanged. At the end, from the first state and every ninth, the rest of the history goes in as one batch and must give the same receipts and a ledger deeply equal to the one applied in place.

4,496 holds were predicted and compared, and the earlier states answered 24,000 probes. The generator is seeded, so the test logs how often each case was reached and fails below a floor:

| Counter | Reached | Floor | Meaning |
| --- | ---: | ---: | --- |
| `first` | 861 | 500 | accepted, the reservation's first fact |
| `further` | 1,384 | 800 | accepted, another reason on a reservation that has one |
| `neighbour` | 869 | 500 | accepted although another reservation has that reason on record |
| `unsized` | 1,163 | 700 | accepted on an unsized reservation |
| `repeated` | 1,075 | 600 | stale, the reason is on record |
| `older` | 601 | 350 | stale, and the reservation recorded another reason after that one |
| `unsizedRepeated` | 571 | 340 | stale on an unsized reservation |
| `settled` | 1,176 | 700 | stale, the reservation has settled |
| `unrecorded` | 992 | 600 | stale after a settlement, with a reason never recorded |
| `earlier` | 2,049 | 1,200 | an earlier state of the chain accepts a hold the newest state calls stale |
| `settledBeside` | 188 | 110 | a settlement took items out beside the fact of another reservation |
| `totalBeside` | 10 | 5 | a reported running total took items out beside a fact |
| `correctedBeside` | 0 | none | a correction took items out beside a fact: never reached here, which is why the directed test exists |

**`holdset_test.go`: the field (2 tests).**

| Test | What it pins |
| --- | --- |
| `TestEveryHoldReasonHasItsOwnBit` | The `HoldReason` constants are read from the package source and must be the five the tests cover. Each has exactly one bit and no two share one. Each is admitted by `validateEntry` and is a lifecycle reason, so the three lists in `ledger.go` name the same five. Five strings that are not reasons have no bit. No declared `UnknownReason` is a hold reason. |
| `TestHoldSetIsWhatTheAcceptedHoldsRecorded` | 200 seeds of 80 steps, forward by `Apply` or `With` at random. After each accepted entry every reservation's set is what its accepted holds recorded. Until it settles the unknown list shows the same reasons, each once, and the reservation is held exactly when the set is not empty. A settled one has no lifecycle item left. Reached: a set with a reason 12,349 times (floor 7,000), with several 8,449 (5,000), kept by a settled reservation 3,499 (2,000). |

### H.5 Mutants

The harness now has 100 mutants: the report's 62 and 38 new ones for the field (H01 to H38). It is still `/tmp/f3-5a/mutate.py`, now with the sha256 `225ea00731fe75dee0dfd7772d860985a3be24af5f38b7fc72956e57e4abfae8`. The harness as the report left it is kept as `/tmp/f3-5a/hold/mutate-before-hold.py`, and `/tmp/f3-5a/hold/patch-mutate.py` turns that file into the current one (rebuilt and compared in `/tmp/f3-5a/hold/rebuild-harness/sha256.txt`).

One full run on the final bytes: `/tmp/f3-5a/hold/mut-final/`, 02:45:51 to 02:48:06, load average 4.36 before and 14.98 after, exit 0, `run.err` empty. `results.tsv` has one row for each mutant and mode with the names of the tests that failed. It calls the three modes below `plain`, `behaviour` and `old-tests`.

Each H mutant runs three times, because the question is which tests catch it:

| Mode | Tests that run | H mutants caught |
| --- | --- | --- |
| all | all 99 | 37 of 38; H31 does not build in this mode |
| behaviour | 97: without `holdset_test.go`, so nothing reads the field | 36 of 38; H29 and H30 survive |
| earlier | the report's 89: without both new files | 25 of 38; 13 survive |

The report's 62 mutants are all still caught, and the 17 `With` mutants are caught again under `-race` (17 of 17).

The numbers in the table are how many tests fail.

| ID | Way of being wrong | Mutant | all | behaviour | earlier |
| --- | --- | --- | --- | --- | --- |
| H01 | never set | an accepted hold does not record its reason | 10 | 9 | 2 |
| H02 | never set | the reason is recorded after the reservation is stored, and lost | 10 | 9 | 2 |
| H03 | never set | an unsized reservation does not record the reason | 6 | 5 | 1 |
| H04 | never set | a sized reservation does not record the reason | 9 | 8 | 1 |
| H05 | never set | only the first reason of a reservation is recorded | 5 | 4 | survives |
| H06 | never set | cancelled has no bit | 5 | 3 | survives |
| H07 | never set | timeout has no bit | 10 | 8 | 1 |
| H08 | never set | process_exit has no bit | 6 | 4 | survives |
| H09 | never set | lost_contact has no bit | 7 | 5 | 1 |
| H10 | never set | superseded_unproven has no bit | 5 | 3 | survives |
| H11 | set for the wrong reason | a hold records the bit next to its own | 12 | 11 | 3 |
| H12 | set for the wrong reason | a hold records every reason | 11 | 10 | 3 |
| H13 | set for the wrong reason | cancelled has the bit of timeout | 6 | 5 | survives |
| H14 | set for the wrong reason | timeout has the bit of process_exit | 8 | 7 | 2 |
| H15 | set for the wrong reason | process_exit has the bit of lost_contact | 5 | 4 | survives |
| H16 | set for the wrong reason | lost_contact has the bit of superseded_unproven | 5 | 4 | survives |
| H17 | set for the wrong reason | superseded_unproven has the bit of cancelled | 5 | 4 | survives |
| H18 | set for the wrong reason | recording a reason forgets the ones before it | 6 | 5 | 1 |
| H19 | set for the wrong reason | the check wants the set to hold this reason and no other | 6 | 5 | 1 |
| H20 | set for the wrong reason | the check is inverted | 21 | 20 | 12 |
| H21 | set for the wrong reason | the reason is recorded before it is checked | 21 | 20 | 12 |
| H22 | shared between reservations | one set is kept for the ledger, not one for each reservation | 6 | 5 | 1 |
| H23 | shared between reservations | a hold records its reason on every reservation | 6 | 5 | 1 |
| H31 | shared between reservations | the set is behind a pointer that a copy of the reservation shares | does not build | 5 | 2 |
| H24 | not carried by `With` | a ledger derived by `With` forgets the sets | 15 | 14 | 7 |
| H25 | not carried by `With` | the sets are kept in a map that `With` shares | 6 | 5 | 3 |
| H38 | not carried by another writer | usage charged to a reservation forgets its recorded reasons | 3 | 2 | survives |
| H26 | a settled reservation is not stale | a hold on a settled reservation is judged by the set alone | 8 | 7 | 5 |
| H27 | a settled reservation is not stale | a settlement empties the set, and the set alone is asked | 8 | 7 | 5 |
| H28 | the state is asked wrongly | any hold on a held reservation is stale | 11 | 10 | 3 |
| H32 | the list loses a fact, the set keeps it | a reported running total takes the facts of its reservation out | 7 | 6 | 4 |
| H33 | the list loses a fact, the set keeps it | an estimated correction of a running total takes them out | 1 | 1 | survives |
| H34 | the list loses a fact, the set keeps it | a correction of a delta event takes them out | 2 | 2 | 1 |
| H35 | the list loses a fact, the set keeps it | a settlement takes the facts of every reservation out | 7 | 6 | 4 |
| H36 | a fact enters the list without a bit | a gap entry may carry a lifecycle reason | 2 | 2 | 1 |
| H37 | a fact enters the list without a bit | a gap entry may carry lost_contact | 1 | 1 | survives |
| H29 | same receipts, another meaning | a string that is not a hold reason has a bit | 1 | survives | survives |
| H30 | same receipts, another meaning | a settlement empties the set, and the state is still asked | 1 | survives | survives |

Five of these ways are the ones you listed: never set, set for the wrong reason, shared between reservations, not carried by `With`, and a settled reservation that is not stale. The others are mine. Most of them are the other direction of H.3: the list changes and the field does not (H32 to H37), or another writer of a reservation drops the set (H38).

Four things to read out of the table:

- **H31 has no result in the first mode.** It changes the field's type to a pointer, so `holdset_test.go` does not compile against it. It is judged by the other two modes: five tests catch it without that file (`TestChainOfWithEqualsAFullReplay`, `TestWithLeavesTheReceiverUntouched`, `TestHoldDispositionMatchesASearchOfTheUnknownList`, `TestRecordedHoldReasonsFollowWith`, `TestRefusedBatchAndReplayLeaveTheRecordAlone`), and the first two of them are earlier tests.
- **H29 and H30 change no receipt and no unknown item.** Only the tests that read the field can see them: `TestEveryHoldReasonHasItsOwnBit` catches H29 and `TestHoldSetIsWhatTheAcceptedHoldsRecorded` catches H30. Both transcripts of H.6 confirm they are invisible from outside (0 lines differ). They are in the harness because they are the reason `holdset_test.go` exists.
- **The earlier tests miss 13.** Eight of them give one reason no bit or a borrowed one, or record only the first reason: H05, H06, H08, H10, H13, H15, H16, H17. The earlier tests notice a missing bit for `timeout` and for `lost_contact` only (H07 and H09, one test each). The other five are H29, H30, H33, H37 and H38.
- **Two mutants are caught by one test only**, apart from H29 and H30: H33 by `TestUsageRemovesNoLifecycleFact` and H37 by `TestGapEntryCannotCarryALifecycleReason`, the two tests added at 02:26. H38 is caught by three tests, all of them new.

Limits: the mutants are written by hand and each ran once. I wrote none in which a reservation starts with a reason on record, or in which a settlement or usage adds one.

### H.6 Old code against new code

H.3 is an argument and H.4 is tests I wrote. This section is the direct comparison: the same histories go through the `ledger.go` from before the change and through the new one, everything the package shows is written out, and the two files are compared byte for byte.

**What is written for each history.** Every entry tried, as its JSON and its digest. Then the receipt as JSON, or the text of the refusal. After each accepted entry the projection as JSON with `Committed()`, `TotalKnown()` and `Len()`. At the end the accepted entries once more through `Reduce` in one batch, with the projection and every receipt. The ledger moves forward by `Apply` and by `With` in turn. The text of a history is hashed, and the transcript has one line for each history with that hash. The full text of the first two seeds of each generator is kept beside it as a sample.

The test files that write the transcripts are not in the package. They reach the compiler through an overlay, and the old side gets `/tmp/f3-5a/hold/pre/ledger.go` the same way.

| | First transcript | Second transcript |
| --- | --- | --- |
| Directory | `/tmp/f3-5a/hold/diff/` | `/tmp/f3-5a/hold/diff2/` |
| Histories | 3,000: the report's `randomEntry` and `randomEntryWithUnsized`, and the second of them with a third of the entries that are not reservations turned into holds; 1,000 seeds each, 120 steps. One more line for a first-slice history: 3,000 attempts, each an unsized reserve and a lost-contact hold, every second hold sent twice. | 2,000 seeds of 120 steps from a generator that is dense in corrections: three reservations, a third of the steps holds, and half of the usage a correction of the running total's head or of one of the last four delta events. |
| Steps | 360,000 (219,288 refused, 841 replays) | 240,000 (13,274 refused) |
| Holds | 13,199 accepted, 18,362 stale | 22,097 accepted, 55,624 stale |
| Settlements | 3,293 | 1,552 |
| Result | `old.txt` and `new.txt` identical: 3,002 lines, 372,369 bytes, sha256 `0a5251e9…76384b` (in full in `sha256.txt`). Samples identical. | `old.txt` and `new.txt` identical: 2,007 lines, 265,320 bytes, sha256 `694b64c6…96cbec` (in full in `run.txt`). Samples identical. |
| When | Written at 00:54. Written again on the final bytes at 02:55:49 to 02:56:01 (load average 3.79 to 3.68): identical to the 00:54 files (`rerun/run.txt`). | 02:37:17 to 02:37:33 (load average 5.79 to 5.96), `run.txt`. |

The second transcript also counts what each kind of remover did (the last lines of `diff2/new.txt`):

| Entry that took items out | Times | Beside a lifecycle fact that had to stay | Facts taken |
| --- | ---: | ---: | ---: |
| settlement | 1,300 | 865 | 0 |
| reported running total | 6,223 | 6,000 | 0 |
| reported correction of a running total | 4,175 | 4,007 | 0 |
| estimated correction of a running total | 1,041 | 953 | 0 |
| correction of a delta event | 11,970 | 11,552 | 0 |

For a settlement, the facts that had to stay are those of other reservations. 12,766 accepted and 34,980 stale holds came on a reservation that had a fact in the list when a correction took an item out. That is the case the property test never reached.

**Control: the transcripts can tell a wrong field from a right one.** The same transcripts written with a mutant in place of the new code (`diff/control.txt`, `diff2/control.txt`):

| Mutant | First transcript: histories whose line differs, of 3,000 | Second transcript: lines that differ, of 2,007 |
| --- | ---: | ---: |
| H01 an accepted hold does not record its reason | 1,205 | 1,984 |
| H13 cancelled has the bit of timeout | 912 | 1,921 |
| H22 one set for the ledger | 1,252 | 1,912 |
| H26 a settled reservation judged by the set alone | 1,488 | 1,197 |
| H38 usage forgets the recorded reasons | not run | 1,982 |
| H32 a reported running total takes facts out | not run | 1,966 |
| H33 an estimated correction of a running total takes facts out | not run | 468 |
| H34 a correction of a delta event takes facts out | not run | 1,959 |
| H35 a settlement takes the facts of every reservation out | not run | 853 |
| H29 a string that is not a hold reason has a bit | 0 | 0 |
| H30 a settlement empties the set, the state is still asked | 0 | 0 |

H29 and H30 are expected to be identical: they change no receipt.

What this does not show: equality on histories the generators do not produce. The generators are seeded, so the same 5,000 histories come out on every run.

### H.7 Verification runs

All on the final bytes (hashes in H.11), go1.27.0 darwin/arm64, in `server/` of the scratch worktree at HEAD `86fea651c`. One script ran them in a row from 02:41:05 to 02:42:43; the load average was 3.16 at the start and 5.78 at the end (`/tmp/f3-5a/hold/final/summary.txt`). Each command's output is in its own file under `/tmp/f3-5a/hold/final/`.

| Command | Result | Output file |
| --- | --- | --- |
| `gofmt -l internal/budget` | exit 0, no output | `gofmt.out` |
| `go vet ./internal/budget/...` | exit 0, no output | `vet.log` |
| `go test ./internal/budget/... -count=1 -v` | exit 0, 99 PASS, 0 FAIL, 0 SKIP | `test-v.log` |
| `go test -race ./internal/budget/... -count=1` | exit 0, no race reported | `race.log` |
| `go test -race ./internal/budget/... -count=1 -v` | exit 0, 99 PASS, 0 FAIL, 0 SKIP | `race-v.log` |
| `go test -race ./internal/budget/... -count=5 -shuffle=on` | exit 0 | `race-shuffle.log` |
| `go build ./...` | exit 0, no output | `build.log` |
| `go list -deps ./internal/budget` | 107 packages; this package is the only one from the repository and the only one outside the standard library | `deps.txt`, `deps-repo.txt`, `deps-nonstd.txt` |
| `grep -rniE 'float' internal/budget` | no line | `grep-float.out` |
| `go run nofloat.go <server dir> ./internal/budget`, the type-level check of section 4.2, run later at 05:03 | exit 0: 17 files, 20,505 expressions, 2,025 definitions and 3,735 literals, none of a floating-point or complex type | `nofloat.log` |
| `git status --short` | `?? server/internal/budget/` and nothing else | `git-status.txt` |

99 is the report's 89 and the 10 new tests. The 5 benchmarks are unchanged. The two new test files import only the standard library.

**The importers of the step-4 tree, type-checked again.** Run in `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server` with an overlay that puts the 17 files of this package over that tree's `internal/budget`; nothing there was written (`/tmp/f3-5a/hold/run-copyback.sh`, output in `/tmp/f3-5a/hold/copyback/`, 02:51:23 to 02:51:48, load average 2.81 to 8.15).

| Command | Result | Output file |
| --- | --- | --- |
| `go vet -overlay overlay.json ./internal/budget/... ./internal/service/ ./internal/handler/` | exit 0, no output | `vet.log` |
| `go build -overlay overlay.json ./...` | exit 0, no output | `build.log` |

`go vet` loads test files, so it type-checks all seven files that import the package: `internal/service/issue_budget.go`, `internal/service/issue_budget_test.go`, four handler tests (`issue_budget_delete_test.go`, `issue_budget_queries_test.go`, `issue_budget_schema_test.go`, `issue_budget_teardown_test.go`) and `internal/budget/scale_test.go` (`importers.txt`). The tree's 12 budget files and its `git status` (51 lines) were the same before and after (`env.txt`).

This is a type check. The tests of that tree were not run against the new package: they need a database.

### H.8 Benchmarks

The machine is the one of section 5.1 (Apple M4, 10 CPUs, 16 GB, go1.27.0 darwin/arm64), still shared with other agents. Every figure is the median of three runs (`-count 3`), with the smallest and the largest in brackets. "Before" is the report's `ledger.go` (`/tmp/f3-5a/hold/pre/ledger.go`, sha256 `77669832…`) given to the compiler through an overlay. "After" is the package on disk. `bench_test.go` did not change, so the held shape is the one you ran: an unsized reserve and a lost-contact hold for each attempt, and nothing settles.

Three before-and-after pairs, all under `/tmp/f3-5a/hold/bench/`; `summary/` has the minimum, median and maximum of every log:

| Pair | How | When | Load average, 1 min | Logs |
| --- | --- | --- | --- | --- |
| 1 | `go test -bench '^Benchmark(Replay\|Load)$/^held$' -benchmem -count 3`, twice in a row | before 01:03:27 to 01:04:15, after until 01:04:47 | 3.89, 4.10, 4.21 | `r1-before-overlay.log`, `r1-after-ondisk.log` |
| 2 | two test binaries built once and run one after the other | before from 01:33:22, after 01:34:08 to 01:34:37 | 5.18, 5.10, 4.43 | `interleaved-replay-load-before.log`, `interleaved-replay-load-after.log` |
| 3 | the same, for all four shapes, on the final bytes | Replay before 03:09:29, after 03:10:39; Load before 03:11:37, after 03:12:59 to 03:14:10 | 2.78, 3.58, 3.72, 9.71, 9.19 | `all-shapes-Replay-before.log` and its three siblings |

A first "before" run on the untouched package (23:31:15 to 23:32:04, load average 2.53 to 4.06, `before-ondisk.log`) agrees with the before columns: replay 830 µs, 49.5 ms and 4.78 s; load 2.85 ms, 68.3 ms and 5.15 s.

**`BenchmarkReplay`, held shape**

| Entries | Pair | Before | After | Before ÷ after |
| ---: | ---: | ---: | ---: | ---: |
| 1,000 | 1 | 805.1 µs (801.0 to 805.3) | 385.7 µs (385.2 to 434.3) | 2.1 |
| 1,000 | 2 | 831.9 µs (820.7 to 878.3) | 393.3 µs (384.5 to 419.4) | 2.1 |
| 1,000 | 3 | 845.9 µs (815.0 to 864.9) | 403.5 µs (395.2 to 414.1) | 2.1 |
| 10,000 | 1 | 47.89 ms (47.62 to 52.25) | 3.41 ms (3.35 to 3.52) | 14 |
| 10,000 | 2 | 49.38 ms (47.78 to 50.97) | 3.43 ms (3.42 to 3.83) | 14 |
| 10,000 | 3 | 47.63 ms (47.55 to 48.96) | 3.42 ms (3.42 to 3.51) | 14 |
| 100,000 | 1 | 4.76 s (4.73 to 5.00) | 43.97 ms (43.23 to 44.16) | 108 |
| 100,000 | 2 | 4.80 s (4.80 to 4.88) | 46.07 ms (45.80 to 47.41) | 104 |
| 100,000 | 3 | 4.57 s (4.56 to 4.74) | 44.85 ms (42.36 to 46.42) | 102 |

**`BenchmarkLoad`, held shape** (decode the stored forms, then replay)

| Entries | Pair | Before | After | Before ÷ after |
| ---: | ---: | ---: | ---: | ---: |
| 1,000 | 1 | 2.73 ms (2.72 to 2.76) | 2.33 ms (2.32 to 2.34) | 1.2 |
| 1,000 | 2 | 2.79 ms (2.78 to 2.85) | 2.46 ms (2.32 to 2.55) | 1.1 |
| 1,000 | 3 | 2.79 ms (2.76 to 2.86) | 2.34 ms (2.33 to 2.39) | 1.2 |
| 10,000 | 1 | 68.19 ms (67.88 to 73.69) | 23.74 ms (23.58 to 26.02) | 2.9 |
| 10,000 | 2 | 73.50 ms (68.87 to 74.02) | 22.84 ms (22.73 to 23.24) | 3.2 |
| 10,000 | 3 | 76.40 ms (72.19 to 95.14) | 22.99 ms (22.79 to 23.57) | 3.3 |
| 100,000 | 1 | 5.06 s (4.94 to 5.57) | 248.25 ms (244.41 to 248.67) | 20 |
| 100,000 | 2 | 5.14 s (5.03 to 5.41) | 248.64 ms (238.90 to 252.33) | 21 |
| 100,000 | 3 | 6.00 s (5.23 to 7.54) | 237.54 ms (235.69 to 259.93) | 25 |

How to read them:

- **The replay is linear now.** It costs 0.39, 0.34 and 0.45 µs per entry at the three sizes. The cumulative shape, whose unknown list stays almost empty, costs 0.49, 0.43 and 0.54 µs after the change (pair 3, table below). A load costs 2.3 to 2.5 µs per entry at every size, most of it decoding (2.1 µs, section 5.3).
- **Pair 3's load rows are the noisy ones.** The load average went from 3.72 to 9.71 while they ran. Read pairs 1 and 2 for the load.
- **Allocation is the same.** B/op and allocs/op agree before and after at 1,000 and 10,000 entries. At 100,000 the logs show 101,381 allocations before and 101,379 after. That comes with the iteration count: before, the benchmark gets one iteration; after, 22 to 26. With one iteration a side (three runs each, 04:19:15 to 04:19:29, load average 3.63 to 3.90) the old code reports 101,399, 101,381 and 101,381 allocations and the new code 101,405, 101,382 and 101,381. The last run of each side is 144,260,520 bytes (`one-iteration-before.log`, `one-iteration-after.log`). The first run of each binary is a little higher on both sides.

**The other three shapes did not change, and the mixed shape is still quadratic** (pair 3):

| Shape | Entries | Replay before | Replay after | Load before | Load after |
| --- | ---: | ---: | ---: | ---: | ---: |
| cumulative | 1,000 | 469.8 µs | 492.6 µs | 2.58 ms | 2.72 ms |
| cumulative | 10,000 | 4.21 ms | 4.26 ms | 26.29 ms | 27.43 ms |
| cumulative | 100,000 | 53.04 ms | 54.41 ms | 263.82 ms | 263.55 ms |
| delta | 1,000 | 609.4 µs | 610.1 µs | 2.66 ms | 2.66 ms |
| delta | 10,000 | 5.13 ms | 5.32 ms | 26.99 ms | 26.44 ms |
| delta | 100,000 | 71.87 ms | 73.45 ms | 281.60 ms | 280.31 ms |
| mixed | 1,000 | 884.9 µs | 893.1 µs | 3.02 ms | 3.08 ms |
| mixed | 10,000 | 41.92 ms | 40.63 ms | 61.93 ms | 61.67 ms |
| mixed | 100,000 | 3.67 s | 3.71 s | 3.96 s | 3.92 s |

The differences are between 3.1 percent down and 5.4 percent up, inside the run-to-run spread of section 5.1; the after binary ran second each time. The mixed shape has no hold entry. Its cost is the pass over the unknown list that each reported running total and each settlement makes. You told me not to touch those, and the first package reaches neither.

**One write to a cached root is unchanged** (`BenchmarkWith`, held shape; the two binaries in turn, three rounds of three, so nine runs a side; 01:30:40 to 01:33:22, load average 3.27 to 4.84; `interleaved-with-before-1.log` to `interleaved-with-after-3.log`):

| Entries | Before | After |
| ---: | ---: | ---: |
| 1,000 | 130.7 µs (117.8 to 143.4) | 118.3 µs (117.7 to 144.5) |
| 10,000 | 1.01 ms (0.94 to 1.06) | 0.97 ms (0.94 to 1.01) |
| 100,000 | 14.22 ms (13.74 to 16.11) | 14.37 ms (13.65 to 17.39) |

One earlier log says otherwise and should be ignored: `extra-with-after.log` (01:22:52 to 01:24, load average 5.59 rising to 8.44) has `With` at 3.04 ms (2.03 to 5.09) for 10,000 entries, against 1.05 ms in `extra-with-before.log`. The after side ran second, as the machine got busier. The benchmark appends a reserve entry, whose code did not change, and B/op and allocs/op are equal on both sides. The nine interleaved runs above replace it.

**Memory.** Live heap per entry after a load, one run a side: 950, 870 and 803 bytes before, 945, 870 and 803 after (`extra-memory-before.log`, `extra-memory-after.log`).

**Profile.** Your profile of the old code on `BenchmarkReplay/held/entries=10000` has `applyHold` at 620 ms of the 710 ms under `With` (87 percent; 63.3 percent of all samples), with 580 ms in `hasUnknown`. The same benchmark on the new code has `applyHold` at 0.07 s of the 0.71 s under `With` (10 percent; 2.6 percent of all samples). Files: `/tmp/f3-5a/hold/prof/top-cum-before-lead.txt` (my listing of your `replay-held.prof`), `replay-held-after.prof` and `top-cum-after-all.txt`.

**A first-slice root with usage gains less than the held shape.** The held shape is the worst case for the old check: every second entry is a hold. I also measured the shape the first package writes when attempts report usage: an unsized reserve, 18 delta events, each in a meter scope of its own, and a lost-contact hold, with nothing settled. The probe is `/tmp/f3-5a/hold/probe/firstslice_test.go`, run through an overlay (01:40:14 to 01:41:19, load average 5.37, 4.87, 7.62; `firstslice-old.log`, `firstslice-new.log`).

| Cost of the usage | Entries | Unknown items | Replay before | Replay after |
| --- | ---: | ---: | ---: | ---: |
| reported | 1,000 | 100 | 616.6 µs (614.3 to 658.0) | 671.9 µs (607.7 to 699.7) |
| reported | 10,000 | 1,000 | 6.42 ms (6.16 to 6.61) | 5.52 ms (5.33 to 5.56) |
| reported | 100,000 | 10,000 | 132.41 ms (131.41 to 141.80) | 78.16 ms (75.31 to 80.51) |
| estimated | 1,000 | 1,000 | 693.2 µs (674.8 to 737.1) | 655.6 µs (622.8 to 659.9) |
| estimated | 10,000 | 10,000 | 10.37 ms (10.32 to 10.75) | 5.88 ms (5.79 to 6.22) |
| estimated | 100,000 | 100,000 | 540.97 ms (539.33 to 548.84) | 82.53 ms (80.97 to 83.78) |

At 100,000 entries that is 1.7 times faster with reported costs and 6.6 times with estimated ones. At 1,000 entries with reported costs the after run is 9 percent slower; a scan of at most 100 items was never a cost, and the after side again ran second on a busier machine. After the change the replay costs 0.55 to 0.83 µs per entry for both kinds of cost at all three sizes. The delta shape of section 5.1, whose unknown list stays almost empty, costs 0.53 to 0.73 µs after the change (pair 3).

**`Projection()` is unchanged** (`BenchmarkProjection`, held shape, the two binaries of pair 3 in turn; 04:52:33 to 04:57:58, load average 2.81 to 4.75; `projection-held-*.log` with `projection.env.txt` and `projection-2.env.txt`):

| Entries | Runs a side | Before | After | B/op on both sides |
| ---: | ---: | ---: | ---: | ---: |
| 1,000 | 3 | 12.5 µs (12.2 to 13.1) | 11.9 µs (11.8 to 12.2) | 81,920 |
| 10,000 | 3 | 103.7 µs (101.5 to 117.3) | 105.7 µs (100.6 to 108.2) | 802,816 |
| 100,000 | 13 | 791.6 µs (689.3 to 889.3) | 792.1 µs (706.5 to 894.4) | 8,003,584 to 8,003,588 |

In the first three runs at 100,000 entries the after side was 10 percent higher (767.4 µs against 698.1 µs). I ran ten more a side with the order reversed, and the table has all 13: the two sides do not differ. The call copies the unknown list, 80 bytes per item; that copy is the allocation. These figures replace the single iterations of section 5.4 (8.3 µs, 99.6 µs and 1.85 ms).

### H.9 What this supersedes in the report above

The report above was not edited. These are the places where it is no longer the last word.

| Place | The report says | Now |
| --- | --- | --- |
| Header, State | Complete and frozen; the last change was `bench_test.go` at 18:52:45. | `ledger.go` changed on 2026-11-18 at 00:09:13, and two test files are new (`holdset_test.go` 00:14:44, `hold_test.go` 02:32:12). The other 14 files are as the report left them. The package has 17 files (H.11). |
| Summary, Tests; section 4.1 | 89 pass, 0 fail, 0 skipped. The full runs predate the last edit and were left to the main session. | 99 pass, 0 fail, 0 skipped, with and without `-race`, on the final bytes (H.7). Your run of the 18:52 state is in `/tmp/f3-main-s5a/v1/`. |
| Summary, Mutation run; appendix A | 62 mutants, all caught. | 100 mutants. The 62 are still caught; appendix A lists them and H.5 lists the other 38. |
| Summary, "A cost the rule does not cover" | Reserve-and-hold pairs replay in 63 ms at 10,000 entries and in 4.9 s at 100,000. This needs a decision. | 3.4 ms and 44 to 46 ms (H.8). You decided, and this change is the result. |
| Section 5.2, reserve and hold | `With`: 129.6 µs, 1.28 ms, 15.68 to 20.74 ms. | Still right. Nine runs a side give 118.3 µs, 0.97 ms and 14.37 ms after the change (H.8). The answer to the 5 ms rule stands. |
| Section 5.3, table, reserve and hold | Replay 1.74 ms, 62.6 ms, 4.91 s. Load 3.29 ms, 73.4 ms, 7.55 s. One iteration each. | Replay 0.39 to 0.40 ms, 3.4 ms, 44 to 46 ms. Load 2.3 to 2.5 ms, 23 to 24 ms, 238 to 249 ms. Three pairs of three runs (H.8). |
| Section 5.3, "Replay is quadratic in the last two shapes" | Three steps scan the unknown list: a hold, a settle, an accepted cumulative running total. | A hold no longer scans. A settlement and a reported running total still do. So do an estimated correction of a running total (`usage.go:259`) and a correction of a delta event (`usage.go:295`), which that sentence left out. The mixed shape is still quadratic: 3.7 s at 100,000 entries. |
| Section 5.3, "In the first package only the hold matters" | About U × T² ns of scans in a full replay. | No longer applies. Nothing the first package writes scans the list. |
| Section 5.3, "When it is paid" | A cold load pays the quadratic cost. | The same moments, at a linear cost: 2.3 to 2.5 µs per entry for the held shape at all three sizes. |
| Section 5.4, reserve and hold | `Projection()`: 8.3 µs, 99.6 µs, 1.85 ms. One iteration each. | 11.9 µs, 105.7 µs and 0.79 ms, timed (H.8). The change does not touch it. |
| Section 5.5, reserve and hold | 945, 869 and 803 bytes of live heap per entry. | 945, 870 and 803 (H.8). No change: the new byte sits where padding was. |
| Section 5.6, proposal 1 | Index the unknown list: per reservation the recorded reasons and a count of the items that block its settlement, and an index by entry and by reservation. | Built in part: the recorded reasons only. Its forecast for 100,000 entries (3.8 to 4.9 s down to 55 to 80 ms) came true for the held shape and not for the mixed shape, which needs the rest (H.10). |
| Section 6.2, question 1 | Options (a) and (b); I recommended (b). | Answered by you: the hold part of (b). Done here. |
| Section 7.1 | The scratch files. | More of them (H.11). |
| Section 7.4 | Not verified: the full runs on the final bytes. Replay, load, projection and memory of the reserve-and-hold shape are single iterations. | The full runs are done (H.7). Replay, load and projection have timed runs (H.8). Memory is still one run a side. |
| Section 7.5 | sha256 of 15 files. | 17 files; `ledger.go` has a new hash (H.11). |

Sections 1 to 3, 4.2 to 4.6, 5.1, 6.1, 6.3, 7.2 and 7.3 describe the first round, and this change touches nothing they say. The two properties of section 4.2 (no floating point, standard library only) were checked again on the 17 files (H.7). The other questions and proposals of sections 6.2 and 6.4 are a separate matter: H.11, "Not started".

### H.10 Proposals only

Nothing in this section is built. Items 1 and 3 are proposals; items 2 and 4 are notes for 5b. You asked for a proposal if I think one of the other scans matters for the first package. I do not think one does, and item 3 says why.

1. **One list of the five hold reasons.** `ledger.go` now names them in three switches: the validation (`:274-278`), `isLifecycleReason` (`:381-387`) and `holdBit` (`:41-55`). A sixth reason that reaches the first and not the last would be accepted and never recorded, so every repeat of it would add one more unknown item. `TestEveryHoldReasonHasItsOwnBit` fails in that case: it reads the constants from the source and wants a bit for each. One table read by all three would make the mistake impossible to write. I did not build it because your message limits the change to the check.

2. **For 5b: give a load all its entries in one call.** `With` clones the ledger before it applies, and so does `Apply`. A replay that calls either once per entry pays one clone per entry and is quadratic for every shape. Measured on the held shape, median of three runs (`/tmp/f3-5a/hold/probe/stepwise-new.log`, 04:48:02 to 04:48:45, load average 2.54 to 3.79):

   | Entries | One `With` call | One `With` call per entry | One `Apply` call per entry |
   | ---: | ---: | ---: | ---: |
   | 1,000 | 0.40 ms | 57.3 ms | 56.3 ms |
   | 10,000 | 3.33 ms | 4.26 s | 4.28 s |

   At 10,000 entries the loop allocates 39 GB. This change did not cause it: `Apply` has cloned for its all-or-nothing batch since step 2. Nothing in the step-4 tree does it either; outside the package that tree calls only `Reduce`, with the whole list (8 calls in four handler test files, no `Apply`, `With` or `NewLedger`). I write it down because a loader reads rows one at a time, and the loop that applies each row as it arrives is the slow one. A tail applied in one call costs one clone: the 1 ms of section 5.2 at 10,000 entries.

3. **The scans that are left.** A settlement passes over the unknown list twice (`ledger.go:366-370` and `:377`). A reported running total, an estimated correction of a running total and a correction of a delta event pass over it once (`usage.go:253-255`, `:259`, `:295`). The first package reaches none of them: nothing settles, and usage arrives as delta events with sequence 0 and without corrections (design §11.6 and §11.8). The two shapes I measured of what it does write are linear after this change: holds only, and delta usage followed by a hold (H.8). The other entries §11.6 writes are gap entries and usage that ends in quarantine. Each appends one item and searches nothing (`ledger.go:225`, `usage.go:170`); I read that and did not measure it. That is every place that goes over the list, apart from `clone` (`ledger.go:132`) and the copy in `Projection` (`:441`): the field is used at `ledger.go:132`, `:366`, `:429` (the append), `:432` (inside `dropUnknown`) and `:441`, and nowhere else (`/tmp/f3-5a/hold/final/grep-unknown.out`). So I propose nothing for the first package. The mixed shape shows the cost once these steps run against a long list: 3.7 s for 100,000 entries. Before S2 lets a settlement or a running total into a root that carries estimates, the rest of section 5.6's proposal 1 is the fix: a count per reservation of the items that block its settlement, and a way to reach the items of one reservation, scope or entry without a pass over the list.

4. **For 5b: a root above the cache cap.** §10.4 does not cache a root with more entries than the cap (50,000, a constant in `service`), so every load of it replays in full, and it says the cost grows linearly with the entries. For what the first package writes, that is true only since this change. A load of the held shape costs 2.3 to 2.5 µs per entry, most of it decoding: 238 to 249 ms at 100,000 entries, measured, which makes 115 to 125 ms at 50,000, computed. Before the change the load at 100,000 entries took 5.1 to 6.0 s. The benchmark decodes with `json.Unmarshal`. The strict decoder that §10.4 now asks for encodes each entry again; section 6.4 put that at 1.5 to 2 µs more per entry, and nobody has measured it.

### H.11 Housekeeping

**Files changed or created.**

- In the package: `ledger.go` changed; `hold_test.go` and `holdset_test.go` are new. Nothing else.
- In the shared checkout: this addendum, appended to the report. Nothing else.
- Scratch under `/tmp/f3-5a/hold/` (about 1,700 files, 75 MB; it does not survive a reboot):
  - `pre/`: the package as the report left it. Every "before" in this addendum is built from it.
  - `stub/`, `red/`, `red-final/`, `green/`: the stages of H.4.
  - `final/`: the runs of H.7, `ledger.diff`, `sha256.txt`, the grep outputs and `messages.txt`.
  - `copyback/`: the type-check of the step-4 importers.
  - `mut-final/`: the mutation run of H.5. `rebuild-harness/`, `mutate-before-hold.py`, `patch-mutate*.py` and `mutate*.diff` show how the harness got from the report's state to this one.
  - `diff/` and `diff2/`: the two transcripts of H.6 and their controls.
  - `bench/`, `prof/` and `probe/`: the benchmarks, the profile and the probe test files of H.3, H.8 and H.10.
  - Scripts: `run-red.sh`, `run-copyback.sh`, `run-mutants.sh`, `run-bench-held.sh`, `compare-tests.sh`, `h-table.py`, `list-messages.py`, and the run scripts inside `bench/` and `probe/`.
  - `report-before-addendum.md`: the report as it was before this addendum.
  - Superseded and kept for the record: `mut-h/`, `mut-all/`, `mut-rows/`, `mut-h38/`, `rows/`, `red-97-tests/`, `final-97-tests/` and `copyback-97-tests/`. They are earlier runs: of the state before 02:26, when `hold_test.go` had six tests and the package 97, and of the steps that added the other two. The harness copies `mutate-93.py` and `mutate-candidate.py` belong to them.
- Changed outside that directory: the harness `/tmp/f3-5a/mutate.py` (H.5).
- The probe files, the transcript files, the stub and every mutated source reached the compiler through `go test -overlay`. None was written into the package directory, which holds the 17 files of the table below and nothing else.
- No memory note was written in this round.

**Not touched.**

- **Scratch worktree:** only the three files above. `git status --short` prints `?? server/internal/budget/` and nothing else; HEAD is still `86fea651cc5d09360327b6cf0f8357c1f360070d`.
- **Step-4 tree** (`/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`): read-only commands (`ls`, `grep`, `shasum`, `git status`) and the `go vet -overlay` and `go build -overlay` of H.7, which write nothing into the tree. Its 12 budget files and its `git status` were the same before and after. At 05:35 the 12 files still have the hashes of the first round (`/tmp/f3-5a/hold/final/step4-budget-sha256-now.txt`). Its `git status` has one more line than at 02:51: `?? server/internal/service/issue_budget_writers_test.go`, written at 05:12:24. That file is not mine. It does not use the budget package, so the list of seven importers stands.
- **Shared checkout:** the report file only. Its first 53,504 bytes are as delivered.
- **`design.md` and `implement.md`:** read only.
- **Nothing was committed, stashed, pushed or merged**, and no branch was switched. I used no database, ran no agent CLI and made no provider call.

**What I read again in the task files.** `design.md` last changed at 23:38:03 (110,713 bytes, sha256 `60300449f8345032082630a688fcc18fef3feeb0f7a301145977559c1325a68f`) and `implement.md` at 23:25:32 (32,218 bytes). Both changes are later than your message, which reached me at 23:02. In that text of `design.md` I re-read the supplement of §10.2, §10.3, §10.4, §10.13, §11.6 to §11.8 and §12.5; in `implement.md`, lines 26 to 28 and 128 to 136. I read nothing else again. I found no rule there that the package cannot follow. Three things those sections ask of it are not built; the next paragraph lists them.

**Not started: the other three items of the second round.** §10.13 now gives 5a a second round of four items. This change is the first. The others are:

- `Apply` becomes private to the package (§10.4);
- `EntryFormVersion`, `EncodeEntry`, a strict `DecodeEntry` and the two length limits, 200 bytes for an ID and 65,536 for the stored form (§10.2, §10.4, §10.8);
- a `Reservation` read method per reservation (§12.5).

`implement.md` line 136 says a second-round message lists the hold check as its item 1 and widens my files to `ledger.go`, `model.go`, one new non-test file and the tests. That message has not reached me. My transcript holds three messages from you: the dispatch (14:53 on 2026-11-17), "Resume 5a: write the report only, in small pieces" (20:34) and "5a follow-up: make the duplicate-hold check constant time" (23:02); `/tmp/f3-5a/hold/final/messages.txt` lists every turn. The instruction I hold says the freeze is lifted "for this change only" and limits me to `ledger.go` and the tests. So I built item 1 and did not start the other three: I did not want to change the exported API on the strength of a notes file. One fact for when they start: the step-4 tree has no caller of `Apply`, `With` or `NewLedger` outside the package (H.10, item 2), which agrees with your count of 8 `Reduce` calls.

**Not verified.**

- The database-backed tests of the step-4 tree against this package. Only the type check ran.
- Anything on another platform than darwin/arm64.
- The mutants are hand-written and each ran once in the final run. H31 has no result against the whole suite (H.5).
- The benchmarks ran on a loaded, shared machine with 36-byte identifiers. The first-package shape with usage and the stepwise replay are probes of mine, not benchmarks of the package. Memory and the profile have one run a side.
- The two transcripts cover 5,000 seeded histories of 600,000 steps. They are evidence for H.3, not a proof of it; the proof is the reading.
- Gap entries and quarantined usage in a long ledger: read, not measured (H.10, item 3).
- The cost of a strict decoder (H.10, item 4).
- No cache exists yet, so nothing is measured under concurrent use.

**sha256 of the 17 files in `server/internal/budget/`.** Taken at 05:17 on 2026-11-18 and the same when this addendum was finished. They equal `/tmp/f3-5a/hold/final/sha256.txt`, the list the runs of H.7 were made on.

| File | Bytes | sha256 | Against the report |
| --- | ---: | --- | --- |
| `bench_test.go` | 7,655 | `4d1a50a76ce8680b3bd8c6e8809e21f67f92e617078e65e0f3b8352f2cffad71` | same |
| `decimal.go` | 2,451 | `6db8462df4606ab587b1ab9e50a8b5ee0eade386927bb5893e9f80ebc3a5f26a` | same |
| `decimal_test.go` | 3,945 | `2c95daf7225b4142b29926f45344c4aea10cab0bbd9718fd1c6f91071acb4db4` | same |
| `edge_cases_test.go` | 15,264 | `8ff64ed04f05cd43425406ef2c94760f9d1f2b43eb129b02b3cc590c77745320` | same |
| `hold_test.go` | 29,547 | `3a9a7e6acb60a88ac8532197f6ab4699f58b68a33974ba7cd6455c460f72b36b` | new |
| `holdset_test.go` | 6,812 | `6eb3388c573d5dd9a05bbc274a55e56f452c4b2bc1de570574b9cd4d969eaad5` | new |
| `invariants_test.go` | 14,396 | `d302dc61546c8a20e9e820545d9df355b5aaf1ffea7776ca6eed1211359eba6c` | same |
| `ledger.go` | 16,095 | `6f7e176946498560ae3aa5f70e6a1db318b68dc7e4e05c7a588ca00617262e6d` | changed; was 15,238 bytes, `77669832b4d8d070d7c79f478473f4ed6497bdc2f76b91af7598931ac5c33052` |
| `ledger_test.go` | 23,544 | `980f783d59abe82db048c89fc34c19daa1e8ea9e5ee8e3643f01a4abb8251627` | same |
| `model.go` | 10,157 | `ec9faca798fa8939bbfd889a7782c6c6a349aa21cc09cffaa2ee99b4b96fe877` | same |
| `price.go` | 6,572 | `a2b4cd68128b4877d39620b61419973b0f6dec0d254e683c837b86125379b31a` | same |
| `price_test.go` | 12,033 | `b57dd9df6dd8d072ac2a50f762cc294a684b00e29e81b8b4c225cfbc12005646` | same |
| `scale_test.go` | 562 | `aaac212c787f403c89670c0dbb18287380440244d561badada6f23f7e82473c2` | same |
| `unsized_test.go` | 21,308 | `a47777c8e2c82466f0310ff91531680dbe8ea79c6e557d8d4b8592695b2c2a48` | same |
| `usage.go` | 11,418 | `90e000c435e29e47029abf1d05900c12387c004becd118749d7c3b99250ed2d9` | same |
| `usage_test.go` | 18,548 | `e323602f135a9d73f87fc561c60b8c5dde732ce1a4c50abc983cc8099e4399a2` | same |
| `with_test.go` | 18,827 | `1f742fac04f12f2cfe1db1c3fed0c285c1794931c793087fcee27d4073ee4f8c` | same |

## Addendum: second pass

Written on 2026-11-18. It answers your message "5a second pass: my rulings on your report, and three more changes to the pure package on top of the constant-time hold check", which reached me at 05:45, and "Resume the 5a second pass" after the failed request of 06:36.

Above this heading exactly one line was added: the first bullet of the Summary, as you asked. Without that line, and without the empty line before this heading, the text above is the 113,932 bytes delivered at 05:41, sha256 `9780334ac78ef365acccbc10cac863cccc9950ca1fda4aaf25d2daaa7de0907c`; a copy is `/tmp/f3-5a/pass2/report-before-second-addendum.md`, and `/tmp/f3-5a/pass2/final/check_report.py` compares the two. The first addendum's statement about its first 53,504 bytes holds for the file without that line. Where this addendum and the text above disagree, the addendum is the later state; P.11 lists those places.

Sections: P.1 summary, P.2 the API as it now stands, P.3 item 1, P.4 item 2, P.5 item 3, P.6 item 4, P.7 mutants, P.8 verification runs, P.9 benchmarks, P.10 what needs your decision, P.11 what this supersedes, P.12 housekeeping.

### P.1 Summary

- **All four items are built and green.** You accepted item 1. For items 2 to 4: `Apply` is now the unexported `apply`; the stored form has its own file, `stored.go`, with `EntryFormVersion`, `MaxEntryIDBytes`, `MaxStoredEntryBytes`, `EncodeEntry`, a strict `DecodeEntry` and the sentinel `ErrInvalidStoredForm`; `(*Ledger).Reservation` returns a `ReservationProjection`. I found no design rule that is wrong or impossible against the code.
- **The four sums you expected hold as they are.** They are checked on every ledger `checkLedger` sees: 51,038 ledgers and 115,290 readings in one run of the suite, with every status sized and unsized. I did not have to bend the accessor (P.6).
- **The stored bytes did not change.** The pinned form is the same 578 bytes, and the three pinned digests are the same; the tests now obtain them through `EncodeEntry` (P.5).
- **Files:** 21, were 17. Four are new: `stored.go`, `stored_test.go`, `readonly_test.go` and `reservation_test.go`. `ledger.go`, `model.go` and five test files changed. Four test files changed only by the rename of `Apply`. Six files are byte-identical (P.12).
- **Tests:** 112 pass, 0 fail, 0 skipped, with and without `-race`, on Go 1.27.0 (the local `go`) and on Go 1.26.9 (the release line the repository builds with). There were 99 before this pass; 13 are new. `gofmt`, `go vet` and `go build ./...` are clean, and the seven step-4 importers type-check against the 21 files (P.8).
- **Mutants:** 80 new ones, 180 in all, and every one is caught. The plain run catches 171 and cannot build a 172nd. Four are caught only under `-race`, three only in a copy of the package on disk, and one only without `holdset_test.go`, as in H.5 (P.7).
- **Item 1 measured again, on both toolchains.** Full replay of the held shape at 1,000, 10,000 and 100,000 entries on Go 1.27.0: 0.84 ms, 49 to 50 ms and 4.8 s before; 0.39 to 0.40 ms, 3.5 ms and 47 to 51 ms after. On Go 1.26.9: 0.86 ms, 54 ms and 5.3 to 6.1 s before; 0.40 ms, 3.7 to 3.9 ms and 49 to 50 ms after (P.9).
- **What `DecodeEntry` costs.** A load of the held shape goes from 2.3 to 2.5 µs per entry to 3.9 to 4.2 µs on Go 1.27.0, and from 4.5 µs to 5.6 to 5.9 µs on Go 1.26.9 (P.9).
- **A finding that changes a figure in design §10.4.** Go 1.26.9 decodes JSON at about half the speed of Go 1.27.0. Every load figure so far was taken with Go 1.27.0 and plain decoding: section 5, H.8 and the 2.3 to 2.5 µs per entry in §10.4, which your own re-measurement agrees with. The repository builds with Go 1.26.x. With the strict decoder on that release line a root of 50,000 entries costs about 0.29 s per load, where §10.4 says about 0.12 s (P.9; P.10, item 1).
- **For your decision** (P.10): that figure; a strict decoder reads a row only while the encoder still writes the same bytes, across Go releases; the one write the type cannot forbid; and six smaller points to confirm.
- **Since decided (added at 18:40):** `design.md` §10.4 and `implement.md` now carry your rulings on all nine points of P.10 and on the three proposals. §10.4 has the corrected load figure, so the two bullets above describe its text as it was when I measured. Nothing in the package changes (P.10, closing note).
- **Still open, and new (added at 18:40): what a cached root keeps.** `DecodeEntry` does not change it. For roots of 1,000 entries or more it is 0.80 to 1.33 KB per entry on Go 1.27.0 and 0.83 to 1.36 KB on Go 1.26.9. Small roots keep more: 2.8 KB for a root of one entry, 3.3 to 4.4 KB for a root of two. A cap of 50,000 entries therefore bounds the memory at about 138 MB, not at the 40 to 65 MB of §10.4 (P.9; P.10, closing note).
- **Not verified:** the database-backed tests of the step-4 tree, and any platform other than darwin/arm64 (P.12).

### P.2 The API as it now stands

Section 1.1 lists what the first round added. This pass changes that list as follows. Line numbers are those of the final files.

**No longer exported:** `(*Ledger).Apply`. It is the unexported `apply` (`ledger.go:158`), used by `Reduce` (`:222`) and by the tests.

**New exported identifiers:**

| Identifier | Declaration | Meaning |
| --- | --- | --- |
| `EntryFormVersion` | `const EntryFormVersion = 1`, untyped, `stored.go:23` | The version of the stored form: what a row's `schema_version` holds (design §10.8). |
| `MaxEntryIDBytes` | `const MaxEntryIDBytes = 200`, `stored.go:33` | The longest entry ID `EncodeEntry` takes, in bytes. |
| `MaxStoredEntryBytes` | `const MaxStoredEntryBytes = 65536`, `stored.go:35` | The longest stored form `EncodeEntry` returns, in bytes. |
| `EncodeEntry` | `func EncodeEntry(e Entry) (raw []byte, digest string, err error)`, `stored.go:44` | The stored form of an entry a ledger could hold, and its digest. |
| `DecodeEntry` | `func DecodeEntry(formVersion int, raw []byte) (Entry, error)`, `stored.go:71` | The entry whose stored form in this version is exactly `raw`. |
| `ErrInvalidStoredForm` | `errors.New("budget: not the stored form of an entry in this version")`, `model.go:28` | The sentinel of every refusal of `DecodeEntry`. |
| `(*Ledger).Reservation` | `func (l *Ledger) Reservation(id string) (ReservationProjection, bool)`, `ledger.go:201` | The net state of one reservation. |
| `ReservationProjection` | a struct of six values, `model.go:272` | What `Reservation` returns (P.6). |
| `ReservationStatus` | `type ReservationStatus string`, `model.go:253` | `ReservationOpen` is `"open"`, `ReservationHeld` is `"held"`, `ReservationSettled` is `"settled"`. |

**The exported methods of `*Ledger`** are now exactly `HasReservation`, `Len`, `Projection`, `Reservation` and `With`. All five only read their receiver.

**Unchanged:** `With`, `Len`, `HasReservation`, `Projection`, `Reduce`, `NewLedger`, `Entry` and its stored form, every receipt and every error text that existed, the price snapshot, the decimal helpers and the usage rules. `Entry.Digest()` moved from `ledger.go` to `stored.go:95` and gives what it gave.

**Unexported additions:** `reservationState.status()` (`ledger.go:23`), `entryDigestDomain` (`stored.go:28`) and `storedDigest` (`stored.go:104`).

**Who is affected:** nobody outside the package. The seven importers in the step-4 tree use `Reduce` (8 call sites), types and constants. None uses `Apply`, `With` or `NewLedger`; the list of what they use is `/tmp/f3-5a/pass2/final/copyback/identifiers.txt`.

### P.3 Item 1: the constant-time hold check

The first addendum (H.1 to H.11) delivered it and you accepted it after your own run of 320,000 `With` calls. I redid nothing of it. What this pass did around it:

- **The code is as you verified it.** `applyHold` reads `r.holds&bit != 0` at `ledger.go:381` and sets the bit at `:385`. Those lines were `:344` and `:348` in H.2 and H.3: `status()`, the longer comments and `Reservation` now stand above them.
- **Its two test files changed by the rename only:** 4 words in `hold_test.go` and 2 in `holdset_test.go`, each `Apply` to `apply` (`/tmp/f3-5a/pass2/final/delta/`).
- **The three switches are as they were**, as you ruled on H.10, item 1: the validation (`ledger.go:312`), `isLifecycleReason` (`:418`) and `holdBit` (`:52`).
- **Its 38 mutants ran again** on the final bytes, with the outcome of H.5: the whole suite catches 37, and H31 does not compile against `holdset_test.go` and is caught without it (P.7).
- **Its differential test checks more than before.** `TestHoldDispositionMatchesASearchOfTheUnknownList` calls `checkLedger` (`hold_test.go:547`), which now also compares every reservation's reading with the projection (P.6).
- **Measured again** on both toolchains, with the load average of each run (P.9).

### P.4 Item 2: no exported method modifies a Ledger

**The change.**

- `Apply` is now `apply` (`ledger.go:158`). A script renamed the whole word: 5 occurrences in `ledger.go` and 48 in eight test files (`/tmp/f3-5a/pass2/item2/rename.py`). `grep -w Apply` now finds nothing in the package. The test `TestWithGivesWhatApplyGives` keeps its name, which is one word.
- `Reduce` still builds its ledger with `apply` (`ledger.go:222-229`). Nothing else can see that ledger.
- The doc comment on `Ledger` (`ledger.go:93-102`) now says: "Nothing this package exports modifies a Ledger or returns a part of one: the fields are unexported, the exported methods only read, and With returns a new Ledger and leaves its receiver as it was. So one Ledger may be shared and read by any number of goroutines. The one write left to code outside the package is to assign a whole Ledger over another (*a = *b); it must not be made to a Ledger that is shared."
- The comment on `With` (`:167-173`) took over the batch rules that stood on `Apply`: the batch is atomic, an equal entry under a known ID is a replay, a different one is `ErrConflict`. `With` is the method a caller now reads.

**Tests** (`readonly_test.go`, new, 2 tests).

- `TestNothingExportedCanModifyALedger` (`:169`). By reflection, the exported methods of `*Ledger` must be exactly the list `ledgerMethods` (`:29`). The comment above the list (`:22-28`) tells the next person that a new method must not write to its receiver or return a part of it, that it needs a read in the shared-ledger test, and that a method which has to modify the ledger stays unexported. The test asks three things more, because each is another way to reach into a Ledger from outside: the struct has no exported field; no exported function, and no exported method of another type, has a `Ledger` among its parameters (it reads the non-test source files of the package for that); and every method of the list has a read in `sharedLedgerReads`, so the list and the race test cannot drift apart.
- `TestSharedLedgerIsOnlyReadThroughItsExportedMethods` (`:203`). One ledger of 7 entries is read by 15 goroutines at a time, three for each of the five methods, 100 rounds each. A read compares its result with what a twin ledger gives. It then writes to everything it was handed: the unknown list and the not-applicable list of a projection, the receipts of `With` and the ledger `With` returned. `With` runs in three ways: a batch that is accepted, a batch that is refused and no entries at all. Afterwards the shared ledger must equal a freshly built one. Under `-race` any write to the receiver is reported.

**Red before green.** Before the rename the new file did not compile: `derived.apply undefined (type *Ledger has no field or method apply, but does have method Apply)` (`/tmp/f3-5a/pass2/item2/red/vet-current.log`, 06:08). With a stub that put `apply` beside `Apply`, 100 tests passed and 1 failed: `*Ledger exports the methods ["Apply" "HasReservation" "Len" "Projection" "With"], the read-only ones are ["HasReservation" "Len" "Projection" "With"]` (`red/test-v.log`, 06:09). Green at 06:11 with 101 tests, and under `-race` at 06:12 (`item2/green/`). `Reservation` joined the list with item 4.

**What these two tests do not show.**

- A method that writes to the ledger and leaves it changed is caught by the plain run. A method that writes and puts the value back is caught only under `-race` (mutants W17 to W20, P.7). CI and `make test` both run the Go tests with `--race` (`.github/workflows/ci.yml:577`, `Makefile:387`), so the gap is a plain `go test` on someone's machine.
- The type does not forbid one write: code outside the package can assign a whole Ledger over another. The doc comment says so, and P.10, item 3 has what closing it would take.

### P.5 Item 3: the stored form belongs to the package

**The change.** A new file, `stored.go` (109 lines), holds everything about the stored form. `model.go` gained the sentinel (`:28`). `Entry.Digest()` moved here from `ledger.go` and gives what it gave.

- **Constants.** `EntryFormVersion = 1`, untyped (`stored.go:23`); `MaxEntryIDBytes = 200` (`:33`); `MaxStoredEntryBytes = 65536` (`:35`). The digest domain `"budget-entry-v1\n"` is the unexported `entryDigestDomain` (`:28`). The comment on the version says what design §10.8 says: a change of the form needs a new version and a new digest domain, and `DecodeEntry` must go on reading the versions that rows already have.
- **The two limits are the entry table's, in bytes.** I read them in migration 866 of the step-4 tree: `octet_length(entry_id) BETWEEN 1 AND 200` (`server/migrations/866_issue_budget.up.sql:123`) and `octet_length(entry_json) BETWEEN 2 AND 65536` (`:132`). The digest check of that table uses the same domain bytes (`:159`).
- **`EncodeEntry`** (`stored.go:44-59`) does four things, in this order:
  1. It refuses an ID longer than 200 bytes with `ErrInvalidEntry`. This comes first because the validation's messages quote the ID, and a refusal should not repeat an ID of any length.
  2. It runs `validateEntry`, the function `With` runs on every entry before it looks at the ledger. So it refuses what every ledger refuses on sight, with the same sentinel (`ErrInvalidEntry` or `ErrNegativeAmount`) and the same text.
  3. It writes the form with `json.Marshal(e)`, as the form has been written since step 2.
  4. It refuses a form longer than 65,536 bytes with `ErrInvalidEntry`.

  It returns the bytes and their digest, which is the digest `e.Digest()` gives. On a refusal it returns no bytes and no digest.
- **`DecodeEntry`** (`stored.go:71-91`) does three things:
  1. It refuses every version other than `EntryFormVersion`.
  2. It decodes with `json.Unmarshal`.
  3. It encodes the decoded entry again and refuses unless the result equals `raw` byte for byte.

  Every refusal wraps `ErrInvalidStoredForm` and nothing else, and returns the zero entry. The decoder's own error is in the text only. Otherwise a row with an amount beyond the 64-bit range would read as `ErrOverflow`, which is one of the errors a replay gives, and a loader log could not tell the two apart.
- **What `DecodeEntry` leaves alone**, as you specified: it does not judge the entry and it does not apply the two limits. The form of an entry that no ledger accepts is read as that entry, and `With` refuses it at replay.

**The stored bytes are what they were.** The pinned form is the same 578 bytes before and after this pass, sha256 `0ca855d62918964c45f2d9217d58cdce178a74d6bf0bd5db1988c542aef5f3e3` (`/tmp/f3-5a/pass2/final/pinned-same.txt`). The three pinned digests are the same: `5f07d194…` for the pinned entry, and `a9fd918d…` and `4ea6c1da…` for the unsized reserve and its sized twin. For each of the 16,400 entries of the property test below, `EncodeEntry` returns exactly `json.Marshal(e)`.

**Tests** (`stored_test.go`, new, 8 tests; three existing tests now go through the two functions).

- `TestStoredFormRoundTripsForEveryEntryALedgerCouldHold` (`:144`) is the property you asked for. It draws 18,000 entries from the three random generators of the suite, 200 seeds of 90 steps. A third of them get awkward text added: quotes, backslashes, text that reads like an escape, the characters JSON escapes for HTML, control characters, the two Unicode line separators and runes of two, three and four bytes.
  - A ledger refuses 1,600 of them on sight. `EncodeEntry` must refuse each with the ledger's own text.
  - For the other 16,400 (more than 1,000 of each kind, 5,428 with awkward text) the form equals `json.Marshal(e)`; the digest equals `e.Digest()` and a SHA-256 the test computes itself; `DecodeEntry` gives the entry back; and the decoded entry encodes to the same bytes and digest.
  - The same bytes are then refused three times: as another version, with one white-space byte put in outside a string, and cut short.
  - The test fails if a generator stops reaching a kind.
- `TestDecodeEntryRefusesWhatIsNotTheStoredForm` (`:235`) has the named refusals: 32 ways to write the pinned entry or something near it. `encoding/json` alone reads 19 of them as exactly the pinned entry, and the test asserts that as well. Each of the 19 is therefore a row where only the byte comparison stands between the bytes and the ledger.

  | You asked for | Cases, by their names in the test |
  | --- | --- |
  | an extra key | "an extra key", "an extra key inside the usage", "an extra key at the end" |
  | a missing key | "a missing key", "a missing key inside the usage" |
  | the step 2 form | "the step 2 form, without the unsized key"; the whole form is a constant in the test (`:32`) |
  | two keys swapped | "two keys swapped", "two keys of the token counts swapped" |
  | trailing data | "trailing data", "a second entry behind the first" |
  | trailing whitespace | "a trailing line feed", "a trailing space"; also "a leading space" and "a space after a colon" |
  | a duplicate key | "a duplicate key", "a duplicate key with another value in front" |
  | a key in another letter case | "a key in another letter case", "a key in capitals" |
  | a wrong version | `TestDecodeEntryReadsOnlyTheCurrentFormVersion`, below |

  The other 14 cases: an escape written for a plain letter; `null` for a text and for a flag; a number where the form has a string; an amount beyond the 64-bit range, with a sign, and with a zero in front; a count of no digits; a byte that is not UTF-8; a byte order mark; an array of one entry; an empty object; `null`; and no bytes at all.
- `TestDecodeEntryRefusesEveryCutAndEveryInsertedSpace` (`:295`) is exhaustive over three stored forms of 1,706 bytes together. Every prefix is refused (1,706). Each of the four white-space bytes at every offset outside a string is refused (1,844). Each byte is taken out in turn: 1,587 results are refused, and the other 119 are the stored form of a different entry, which the test proves by encoding that entry back to the same bytes. No two byte strings stand for one entry.
- `TestDecodeEntryReadsOnlyTheCurrentFormVersion` (`:346`). `EntryFormVersion` is 1 and the digest domain names the same number. The pinned form is read as version 1 and refused as version 0, 2, 3, 10, -1 and the largest and smallest 32-bit integers. A wrong version is refused before the bytes are looked at.
- `TestEncodeEntryRefusesWhatALedgerRefusesOnSight` (`:372`). 15 named invalid entries, each refused with the text an empty ledger gives for it, 13 as `ErrInvalidEntry` and 2 as `ErrNegativeAmount`. Three entries that only some ledgers refuse (a hold, a usage and a settle for a reservation that does not exist) do get a stored form: that question is for `With`.
- `TestEncodeEntryKeepsToTheLimitsOfTheEntryTable` (`:456`), the two limits at the boundary.
  - **ID:** 1, 199 and 200 bytes fit; 201 and 1,000 do not. The 200-byte ID is the longest one the service builds, `usage/<uuid>/<uuid>/<model of 120 bytes>` (design §10.2). The limit counts bytes: 100 runes of two bytes fit, 67 runes of three bytes (201 bytes) do not.
  - **Stored form:** a detail that leaves the form at 65,535 or 65,536 bytes fits; 65,537 does not. The form counts as written: a detail of under 11,000 bytes, nearly all of them "<", fills it, because the form writes each "<" in six bytes. One more byte gives 65,537 and one more "<" gives 65,542; both are refused.
  - **Order:** an ID of 100,000 bytes is refused for its length, never as `ErrNegativeAmount`, and the refusal does not repeat the ID.
- `TestDecodeEntryProvesTheFormAndNothingElse` (`:557`). Nine entries that `EncodeEntry` refuses: seven that no ledger accepts, one with an ID of 201 bytes and one with a form beyond the limit. `DecodeEntry` reads the JSON encoding of each as that entry, and the digest of the bytes is the entry's `Digest()`. `With` then refuses the seven.
- `TestStoredFormSpellsTextTheSameInEveryToolchain` (`:625`) pins how the form writes every character that is not written as itself. The next paragraph says why it exists.
- **Existing tests.** `TestEntryStoredFormRoundTripsAndDigests` (`ledger_test.go`) and `TestUnsizedFlagIsPartOfTheEntryIdentity` (`unsized_test.go`) obtain their pinned form and digests through `EncodeEntry` and read them back through `DecodeEntry`. `TestInvalidUTF8IsRejectedInEveryStringField` (`edge_cases_test.go`) also requires `EncodeEntry` to refuse each of its entries.

**Two toolchains, one form.** I did not expect this part.

- The local `go` is Go 1.27.0. The repository builds with Go 1.26.x: `server/go.mod` says `go 1.26.6`, CI installs `go-version: "1.26.x"` (`.github/workflows/ci.yml:195` and four more places), and the image is built `FROM golang:1.26-alpine` (`Dockerfile:2`). A Go 1.26.9 toolchain was already in this machine's module cache. I ran it with `GOTOOLCHAIN=local GOPROXY=off`, so nothing was downloaded.
- Go 1.27.0 builds `encoding/json` on its new implementation (`JSONv2: true` in `internal/buildcfg/exp.go:87` of its GOROOT). Go 1.26.9 has the classic encoder and decoder.
- My first run of the finished code, at 07:32 on Go 1.27.0, passed 106 tests and failed one (`/tmp/f3-5a/pass2/item3/green/test-v.log`). That test fed `DecodeEntry` the JSON encoding of an ID that is not UTF-8 and wanted a refusal, which is what the encoder I knew gives: Go 1.26 writes such a byte as a six-byte escape, and no stored form contains one. Go 1.27 writes the replacement character itself, and those bytes are the stored form of another, valid entry. No entry with such a byte reaches the form, because `validateEntry` refuses it, so the package had no defect there. What the failure showed is that the form depends on the encoder.
- So I compared the two encoders over every Unicode scalar value. They write the same bytes for all 1,112,064: 39 characters are written in a way of their own and every other one as itself (`/tmp/f3-5a/pass2/item3/runes/go1.26.9.txt` and `go1.27.0.txt` are identical, sha256 `9cc0dba2…`). The property test logs one SHA-256 over all its 16,400 stored forms, and it is `ee257bc1…` on both toolchains.
- `TestStoredFormSpellsTextTheSameInEveryToolchain` pins the 39 spellings: the 32 control characters, the quote, the backslash, the three characters escaped for HTML and the two Unicode line separators. It also pins a sample of what is written as itself, and the digest of an entry that carries all of them. Then it gives `DecodeEntry` 12 other spellings of the same text and requires each to be refused. A toolchain that spells one of the 39 in another way fails this test in CI before it refuses a row written earlier.
- The test of the byte that is not UTF-8 now states the difference between the two releases and accepts either (`stored_test.go:412-433`).

**Red before green.** Without a stub the tests did not compile: `undefined: EntryFormVersion`, `EncodeEntry`, `DecodeEntry` (`/tmp/f3-5a/pass2/item3/red/vet-without-stub.log`). With a stub that encodes without judging and decodes without a version check or a comparison, 100 tests passed and 7 failed (`red/test-v.log`, 07:29): `TestInvalidUTF8IsRejectedInEveryStringField` and the six tests of `stored_test.go` that existed then, which are the first six of the list above. Green on both toolchains at 07:46 with 108 tests, and under `-race` at 07:47 (`item3/green/`). `TestDecodeEntryProvesTheFormAndNothingElse` came at 08:12 (`/tmp/f3-5a/pass2/extra/`), when I saw that nothing pinned what `DecodeEntry` must leave alone.

**What follows for 5b.**

- A row in the step 2 form, without the key `"unsized"`, is refused. By design §10.5 that makes the root's ledger unavailable. Such rows exist only in test databases (§10.8), so a database filled before 5a has to be rebuilt.
- `With` applies no size limit: a ledger takes an entry of any size, and only `EncodeEntry` knows the table. §10.2 already sends every new entry through `EncodeEntry` first (P.10, item 4).
- A strict decoder reads a row only while the encoder still writes the bytes the row has (P.10, item 2).

### P.6 Item 4: reading one reservation

**The change.** `(*Ledger).Reservation(id)` (`ledger.go:201-218`) returns a `ReservationProjection` (`model.go:272-288`) and whether a reserve entry opened a reservation with that ID. `HasReservation` stays. The reducer is untouched, as design §12.5 wants: the method reads the reservation through the two helpers `Projection()` already uses, `unspent()` (`ledger.go:69`) and `overrun()` (`:78`). The one other addition is `status()` (`:23`), which gives the state its exported name.

| Field | JSON key | Value |
| --- | --- | --- |
| `Status` | `status` | `ReservationOpen`, `ReservationHeld` or `ReservationSettled`. |
| `Unsized` | `unsized` | Whether the reservation was opened without an amount. |
| `Reserved` | `reserved_ticks` | The amount of the reserve entry; 0 when unsized. It stays after a hold and after the settlement. |
| `ActualKnown` | `actual_known_ticks` | The known actual spend recorded against the reservation, usage that arrives after the settlement included. |
| `Unspent` | `unspent_ticks` | The part of the amount that still occupies allowance: `Reserved` less `ActualKnown`, never below 0. It counts in R while the reservation is open and in H while it is held. 0 once settled and 0 when unsized. |
| `Overrun` | `overrun_ticks` | `ActualKnown` beyond `Reserved`. 0 when unsized. It stays after the settlement. |

A reading is a plain value of six fields. An ID that is no reservation's reads the zero value and `false`.

**Tests** (`reservation_test.go`, new, 3 tests; one helper in `invariants_test.go`).

- `TestReservationReadsEachStatusSizedAndUnsized` (`:40`) builds 14 reservations in one ledger and requires the exact reading of each.
  - **Sized:** fresh, partly spent, spent to the last tick, overrun, and a reservation of 0 with spend, each open; held with unspent and held with overrun; settled, settled with overrun, and held then settled.
  - **Unsized:** fresh, with spend, held and settled.
  - **No reservation:** eight IDs read nothing: the empty ID, an unknown one, an ID in another letter case, one with a trailing space, and the IDs of a reserve entry, a usage entry, a scope and a settle entry. The zero `Ledger` and `NewLedger()` read nothing for any ID.
  - The projection of that ledger is A 662, R 170, H 65, overrun 100.
- `TestReservationReadingFollowsTheEntries` (`:119`) takes one reservation through 15 entries and reads it after each one. The ledgers obtained along the way keep the reading they had.
- `TestReservationReadingHoldsNoReference` (`:183`). The struct has six fields, each a string, a bool or an int64, so a reading cannot point into its ledger. The three status strings and the JSON form are pinned: `{"status":"held","unsized":true,"reserved_ticks":"1","actual_known_ticks":"2","unspent_ticks":"3","overrun_ticks":"4"}`.
- `checkReadings` (`invariants_test.go:180-232`) is the tie to the projection you asked for. `checkLedger` calls it last (`:138`), so it runs on every ledger of the five random-history tests and of the first two tests above. For each reservation it computes the expected reading from the reservation's raw fields, without the two helpers, and compares. Then it adds the readings up.

**Your four sums hold as written.** I changed nothing in the accessor to make them hold.

| Sum over all reservations | Equals |
| --- | --- |
| `Unspent` of the open ones | `Projection.Reserved` |
| `Unspent` of the held ones | `Projection.UnknownHold` |
| `Overrun` | `Projection.Overrun` |
| `ActualKnown`, plus the known actual of usage that names no reservation | `Projection.ActualKnown` |
| `ActualKnown` + `Unspent`, plus that unreserved actual | A + R + H, the ledger's committed total (a fifth relation, which I added) |

One run of the suite checks them on 51,038 ledgers, 49,848 of which have a reservation, and on 115,290 readings. I counted through an overlay that adds counters to the helper; nothing was written into the package (`/tmp/f3-5a/pass2/final/readings/test-v.log`).

| Status | Sized | Unsized | With unspent | With overrun | With spend |
| --- | ---: | ---: | ---: | ---: | ---: |
| open | 33,718 | 8,487 | 18,991 | 14,548 | 20,176 |
| held | 17,101 | 10,872 | 10,349 | 6,664 | 11,824 |
| settled | 37,042 | 8,070 | 0 | 26,426 | 32,229 |

**Red before green.** Without a stub the tests did not compile (`/tmp/f3-5a/pass2/item4/red/vet-without-stub.log`: `undefined: ReservationProjection`, `ReservationStatus`). With a stub whose `Reservation` reads nothing, 103 tests passed and 8 failed (`red/test-v.log`, 07:57): the five random-history tests that call `checkLedger`, the shared-ledger test of item 2, and the first two tests above. Green on both toolchains at 07:59 with 111 tests, and under `-race` at 07:59 (`item4/green/`).

**Three things to know when step 7 reads it.**

- After a settlement `Reserved` still shows the amount and `Unspent` is 0, so `Reserved` less `ActualKnown` is no longer what the reservation occupies.
- A settled reservation keeps its overrun. The projection counts it, so the third sum needs it.
- A sized reservation of 0 is not unsized. Everything it spends is overrun.

### P.7 Mutants

**Totals.** 180 mutants and one control, in 241 runs on the final bytes (`/tmp/f3-5a/pass2/mutants/results.tsv`, 08:32:58 to 08:36:40, load average 5.47 to 17.51). 100 are the mutants of appendix A and H.5, run again. 80 are new: 7 for the API of item 2 (X01 to X07), 5 for read methods that write (W17 to W21), 42 for the stored form (S01 to S42) and 26 for `Reservation` (R01 to R26). Every one of the 180 is caught in at least one mode.

| Mode | What runs | Runs | Outcome |
| --- | --- | ---: | --- |
| `plain` | the whole suite, the mutant given to the compiler through `-overlay` | 177 | 171 caught; S26 does not compile against the tests; W17 to W20 survive; H31 does not compile |
| `race` | the same under `-race`, for W00 to W21 | 22 | 22 caught |
| `behaviour` | the suite without `holdset_test.go`, for H01 to H38 | 38 | 36 caught; H29 and H30 survive |
| `copy` | the suite in a copy of the package directory, for X04 to X06 | 4 | 3 caught; the control X00 passes |

**The mutants the plain run does not catch, and what does.**

- **W17 to W20:** `Len`, `HasReservation`, `Reservation` or `Projection` writes to its receiver and puts the value back. No result changes, so a plain run cannot see it. Under `-race` `TestSharedLedgerIsOnlyReadThroughItsExportedMethods` reports the data race for all four, and `TestWithIsSafeBesideReadersOfTheOrigin` for the three methods it reads.
- **S26:** the version is a typed constant. `stored_test.go:24-27` assigns it to an `int32` and to an `int`, so the mutant does not compile: `cannot use EntryFormVersion (constant 1 of type int) as int32 value`. That is what holds the constant untyped, as you asked.
- **X04 to X06:** an exported function, or an exported method of another type, takes a Ledger. The test reads the package's source files from disk, which an overlay does not change. So these three run in a copy of the directory with the mutant written into the copy. X00 is that copy without a mutant, and it passes.
- **H31, H29, H30:** as in H.5. H31 does not compile against `holdset_test.go` and is caught without it. H29 and H30 change no behaviour, and the two tests that read the field catch them.

**The mutants you named.**

| You asked for | Mutants | Caught by |
| --- | --- | --- |
| Item 3: no version check | S01; also S02, S03, S04 (a later version, an earlier one, version 0 read as the current one) | `TestDecodeEntryReadsOnlyTheCurrentFormVersion` and the property test |
| Item 3: no byte comparison | S05; also S06, where a decoder that refuses unknown keys and trailing data stands in for the comparison | 4 tests each, the named refusals among them |
| Item 3: each limit off by one | S12 (200 refused), S13 (201 taken), S17 (65,536 refused), S18 (65,537 taken); the constants themselves in S22, S23, S24, S27 | `TestEncodeEntryKeepsToTheLimitsOfTheEntryTable` |
| Item 3: validation skipped in `EncodeEntry` | S28 | 4 tests |
| Item 4: two statuses swapped | R01, R02, R03, one for each pair | 7 or 8 tests each |
| Item 4: unspent not zero after settlement | R07 | 6 tests |
| Item 4: overrun reported for unsized | R11 | 3 tests |
| Item 4: the wrong amount returned | R10, R13, R14, R15, R16, R17 | 7 or 8 tests each |

S06 is worth a second look: it is the cheaper strict decoder one might write instead. It lets through a missing key, swapped keys, a duplicate key, a key in another letter case and white space, and the named refusals catch it.

**The 80 new mutants.** The last column is the number of tests that fail in the plain run, unless it names another mode. `/tmp/f3-5a/pass2/final/new-mutants-with-tests.txt` names the tests.

| ID | What the mutant does | Tests that fail |
| --- | --- | --- |
| X01 | An exported `Apply` is back | 1 |
| X02 | An exported method empties the ledger | 1 |
| X03 | An exported method hands out the entry map | 1 |
| X04 | An exported function appends to a ledger | 1, in `copy` |
| X05 | An exported method of an entry appends it to a ledger | 1, in `copy` |
| X06 | An exported function takes ledgers in a map | 1, in `copy` |
| X07 | The ledger has an exported field | 1 |
| W17 | `Len` writes to its receiver and puts the value back | 0; 2 under `-race` |
| W18 | `HasReservation` writes to its receiver and puts the value back | 0; 2 under `-race` |
| W19 | `Reservation` writes to its receiver and puts the value back | 0; 1 under `-race` |
| W20 | `Projection` writes to its receiver and puts the value back | 0; 2 under `-race` |
| W21 | `Projection` hands out the ledger's own unknown list | 2; 2 under `-race` |
| S01 | `DecodeEntry` does not look at the version | 2 |
| S02 | A later version is read as the current one | 2 |
| S03 | An earlier version is read as the current one | 2 |
| S04 | Version 0 is read as the current one | 2 |
| S05 | `DecodeEntry` does not compare the bytes | 4 |
| S06 | `DecodeEntry` refuses unknown keys and trailing data and compares no bytes | 4 |
| S07 | White space around the form is let through | 3 |
| S08 | White space behind the form is let through | 3 |
| S09 | White space between the tokens is let through | 3 |
| S10 | The bytes are compared without regard to letter case | 2 |
| S11 | Only the length of the bytes is compared | 3 |
| S12 | An ID of 200 bytes is refused | 1 |
| S13 | An ID of 201 bytes is taken | 2 |
| S14 | The ID is measured in characters | 1 |
| S15 | The ID has no limit | 2 |
| S16 | The ID is measured after the entry is judged | 1 |
| S17 | A stored form of 65,536 bytes is refused | 1 |
| S18 | A stored form of 65,537 bytes is taken | 1 |
| S19 | The stored form is measured in characters | 1 |
| S20 | The detail is measured in place of the stored form | 2 |
| S21 | The stored form has no limit | 2 |
| S22 | `MaxEntryIDBytes` is 201 | 1 |
| S23 | `MaxEntryIDBytes` is 199 | 1 |
| S24 | `MaxStoredEntryBytes` is 65,535 | 1 |
| S25 | The form version is 2 | 1 |
| S26 | The form version is a typed constant | does not compile |
| S27 | `MaxStoredEntryBytes` is 65,537 | 1 |
| S28 | `EncodeEntry` does not judge the entry | 4 |
| S29 | `EncodeEntry` rewords the ledger's refusal | 2 |
| S30 | `EncodeEntry` hands out the bytes it refuses for their length | 2 |
| S31 | `EncodeEntry` writes <, > and & as themselves | 4 |
| S32 | `EncodeEntry` writes the form with a line end | 8 |
| S33 | `EncodeEntry` digests the form with a line end | 4 |
| S34 | The digest has no domain in front | 6 |
| S35 | `Digest` refuses an entry no ledger accepts | 2 |
| S36 | `DecodeEntry` passes the decoder's error on as its cause | 1 |
| S37 | `DecodeEntry` hands out the half-read entry with its refusal | 2 |
| S38 | `DecodeEntry` hands out the entry whose bytes it refuses | 4 |
| S39 | `DecodeEntry` judges the entry | 2 |
| S40 | `DecodeEntry` refuses an ID beyond the limit | 1 |
| S41 | `DecodeEntry` refuses a form beyond the limit | 1 |
| S42 | A row that is not a stored form is refused as an invalid entry | 5 |
| R01 | Open and held are swapped | 8 |
| R02 | Held and settled are swapped | 7 |
| R03 | Open and settled are swapped | 8 |
| R04 | A held reservation reads as open | 7 |
| R05 | A settled reservation reads as held | 6 |
| R06 | Held goes by another name outside the package | 1 |
| R07 | Unspent is not zero after the settlement | 6 |
| R08 | Unspent is zero for a held reservation | 7 |
| R09 | Unspent goes below zero | 7 |
| R10 | Unspent is the whole amount | 8 |
| R11 | Overrun is reported for an unsized reservation | 3 |
| R12 | Overrun is not reported after the settlement | 6 |
| R13 | Overrun and unspent are swapped | 8 |
| R14 | Reserved and actual are swapped | 8 |
| R15 | Reserved is what is left of the amount | 8 |
| R16 | Actual stops at the amount | 7 |
| R17 | Actual is what the reservation occupies | 8 |
| R18 | Unsized is not reported | 4 |
| R19 | Unsized is reported for the sized | 8 |
| R20 | An amount of zero reads as unsized | 5 |
| R21 | An ID that is no reservation's reads as an open one | 8 |
| R22 | An ID that is no reservation's reads false with a status | 8 |
| R23 | An entry ID reads the reservation the entry names | 1 |
| R24 | The empty ID reads the spend that names no reservation | 1 |
| R25 | A reading carries a pointer to its ledger | 1 |
| R26 | Unspent is stored under another key | 1 |

**The harness changed; the package did not change for it.** The first run of this pass (`/tmp/f3-5a/pass2/mutants-first-run/`, 08:20) showed three faults of the harness. I fixed them there (`/tmp/f3-5a/pass2/patch_mutate_2.py`; the final `mutate.py` has sha256 `bde516dc…`).

- **E01 to E03 no longer found their place.** The text `json:"unsized"` now occurs twice in `model.go`, in `Entry` and in `ReservationProjection`. Their anchors now name the field of `Entry`.
- **X04 to X06 survived**, for the reason given above. They got the `copy` mode and the control X00.
- **The third mode of H.5 no longer builds.** `old-tests` ran only the 89 tests of the first round, without `hold_test.go` and `holdset_test.go`. The property test of item 3 uses a generator that `hold_test.go` defines (`undefined: randomEntryWithHolds`), so I dropped the mode. The column "earlier" of H.5 describes the state of H.5 and was not run again.

The mutants are written by hand and each ran once. They show that the tests notice these 180 faults, not that they notice every fault.

### P.8 Verification runs

One script ran everything once, after the last edit of the package (08:11:28): `/tmp/f3-5a/pass2/verify.sh`, from 08:41:16 to 08:44:15. The load average went from 2.89 to 13.09 during it, which matters for no result here because nothing in this run is a timing. Every command wrote to its own file under `/tmp/f3-5a/pass2/final/`; `summary.txt` there has the exit codes.

| Check | Command, run in `server/` | Result | Output |
| --- | --- | --- | --- |
| Format | `gofmt -l internal/budget` | exit 0, no output | `gofmt.out` |
| Vet | `go vet ./internal/budget/...` | exit 0, no output | `vet.log` |
| Tests | `go test ./internal/budget/... -count=1 -v` | exit 0; 112 PASS, 0 FAIL, 0 SKIP | `test-v.log` |
| Race | `go test -race ./internal/budget/... -count=1` | exit 0, no race reported | `race.log` |
| Race, verbose | the same with `-v` | exit 0; 112 PASS, 0 FAIL, 0 SKIP | `race-v.log` |
| Race, shuffled | `go test -race ./internal/budget/... -count=5 -shuffle=on` | exit 0 | `race-shuffle.log` |
| Build | `go build ./...` | exit 0, no output | `build.log` |
| Vet, Go 1.26.9 | `go vet ./internal/budget/...` | exit 0, no output | `vet-go1.26.9.log` |
| Tests, Go 1.26.9 | `go test ./internal/budget/... -count=1 -v` | exit 0; 112 PASS, 0 FAIL, 0 SKIP | `test-v-go1.26.9.log` |
| Race, Go 1.26.9 | `go test -race ./internal/budget/... -count=1` | exit 0, no race reported | `race-go1.26.9.log` |
| Dependencies | `go list -deps ./internal/budget` | 107 packages; this package is the only one of the repository and the only one outside the standard library | `deps.txt`, `deps-repo.txt`, `deps-nonstd.txt` |
| Direct imports | `go list -f '{{join .Imports "\n"}}' ./internal/budget` | 13, all standard library: `bytes`, `crypto/sha256`, `encoding/hex`, `encoding/json`, `errors`, `fmt`, `maps`, `math`, `math/big`, `slices`, `sort`, `strconv`, `unicode/utf8` | `imports.txt` |
| No floating point | `grep -rniE 'float' internal/budget`, and the type check of every expression from the first round | 0 lines; 21 files, 25,343 expressions, none of a floating-point or complex type | `grep-float.out`, `nofloat.log` |
| `Apply` is gone | `grep -rnE '\bApply\b' internal/budget --include='*.go'` | 0 lines | `grep-Apply.out` |
| Worktree | `git status --short`, `git stash list` | `?? server/internal/budget/`; 10 stashes, none of them mine | `git-status.txt`, `git-stash.txt` |
| Hashes | `shasum -a 256 *.go` before and after the script | 21 files, the same before and after | `sha256-before.txt`, `sha256.txt` |

The Go 1.26.9 runs used the toolchain already on this machine, with `GOTOOLCHAIN=local GOPROXY=off`, so nothing was downloaded. `bytes` is the one import added since the first addendum; `DecodeEntry` compares with it.

**The same bytes on both toolchains.** The round-trip test logs the SHA-256 of all the stored forms it encoded. Both toolchains log `ee257bc1…daf42a30`.

**What the property tests logged** (`test-v.log`; the Go 1.26.9 log has the same lines):

- `stored_test.go:220`: 16,400 entries encoded (7,030 usage, 2,587 hold, 2,287 reserve, 2,214 settle, 1,181 exclusion, 1,101 gap), 5,428 of them with awkward text; 1,600 refused.
- `stored_test.go:337`: 1,706 cuts, 1,844 inserted spaces and 1,587 removed bytes refused; 119 removed bytes left the form of another entry.
- `hold_test.go:675`, `holdset_test.go:194` and `unsized_test.go:506` log the counters of the earlier rounds with the values they had in section 4 and in H.4. Every one is above zero except `correctedBeside`, which is 0 here as it was there, and which a directed test covers.

**The package has not changed since.** I hashed it again after the benchmarks (09:55) and before writing this (12:24). Both times the 21 hashes were the ones of `sha256.txt`.

**The importers of the step 4 tree** (`/tmp/f3-5a/pass2/final/copyback/`, 08:46:46 to 08:47:12). I read that tree and wrote nothing into it. An overlay of the 21 files (`overlay.json`) stood in for its `internal/budget`.

| Check | Result | Output |
| --- | --- | --- |
| Files that import the package | 7: `internal/service/issue_budget.go`, its test, four handler tests (`issue_budget_delete_test.go`, `issue_budget_queries_test.go`, `issue_budget_schema_test.go`, `issue_budget_teardown_test.go`) and the package's own `scale_test.go` | `importers.txt` |
| `go vet -overlay` of `internal/budget`, `internal/service` and `internal/handler` | exit 0, no output | `vet.log` |
| `go build -overlay ./...` | exit 0, no output | `build.log` |
| The tree's 12 budget files, and its `git status`, before and after | the same | `step4-*-before.txt`, `step4-*-after.txt` |

Vet type-checks the test files too, which the build does not. None of the 7 files names `Apply`; what they use is in `identifiers.txt`. I ran no test of that tree: its budget tests need a database.

### P.9 Benchmarks

Everything here is under `/tmp/f3-5a/pass2/final/bench/`. The machine is an Apple M4 with 10 CPUs and 16 GB that other sessions were using, so each run has its load average beside it. A figure is the median of 3 runs (`-test.count 3`), or of 5 for the one-entry benchmarks.

**Method.** I built three test binaries per toolchain once, with `go test -c`, and ran them in turn. Nothing was rebuilt between the rounds, and the package had the same 21 hashes before and after (`env.txt`, `env2.txt`).

| Binary | What it holds | How |
| --- | --- | --- |
| `pre` | the package as the first round delivered it, before item 1 | an overlay puts the 15 files of that snapshot in place and hides the 6 newer ones (`overlay-pre.json`, `prepare.txt`) |
| `plain` | the package on disk, but `benchLoad` decodes with `json.Unmarshal` as it did before item 3 | an overlay replaces `bench_test.go` by a copy with 3 lines taken out and 1 put in (`plain.diff`) |
| `final` | the package on disk; `benchLoad` decodes through `DecodeEntry` | no overlay |

So `pre` against `plain` shows item 1 alone, and `plain` against `final` shows what `DecodeEntry` adds. The scripts are `run-bench.sh` (09:00 to 09:20) and `run-bench-2.sh` (09:28 to 09:43). The second repeats what a busy machine had disturbed and gives Go 1.26.9 a second round. `summary/` has the minimum, median and maximum of every benchmark, and `per-entry.txt` the cost per entry.

**Item 1, the held shape: full replay** (`BenchmarkReplay/held`, medians).

| Toolchain | State | Round | Load average, start to end | 1,000 entries | 10,000 | 100,000 |
| --- | --- | --- | --- | ---: | ---: | ---: |
| Go 1.27.0 | before | 1 | 3.39 to 2.77 | 835.5 µs | 48.79 ms | 4.78 s |
| Go 1.27.0 | before | 2 | 3.33 to 3.42 | 838.0 µs | 49.63 ms | 4.84 s |
| Go 1.27.0 | after | 1 | 3.18 to 3.33 | 398.2 µs | 3.55 ms | 46.96 ms |
| Go 1.27.0 | after | 2 | 4.92 to 8.88 | 397.7 µs | 3.49 ms | 50.91 ms |
| Go 1.27.0 | after | 3 | 3.64 to 8.89 | 391.6 µs | 3.52 ms | 48.13 ms |
| Go 1.26.9 | before | 1 | 4.44 to 3.62 | 860.6 µs | 53.63 ms | 5.30 s |
| Go 1.26.9 | before | 2 | 7.23 to 5.99 | 854.5 µs | 53.79 ms | 6.06 s |
| Go 1.26.9 | after | 1 | 4.80 to 3.75 | 403.0 µs | 3.73 ms | 50.43 ms |
| Go 1.26.9 | after | 2 | 5.88 to 5.25 | 401.3 µs | 3.90 ms | 49.38 ms |

At 100,000 entries the replay is about 100 times faster on both toolchains, at 10,000 about 14 times, at 1,000 about twice. The cost per entry is now flat: 0.35 to 0.51 µs at every size. The Go 1.27.0 "after" rows are close to your own medians (0.39 ms, 3.37 ms, 45.3 ms) and to H.8 at the two smaller sizes. At 100,000 entries they are 47 to 51 ms where H.8 had 44 to 46 ms. I did not look for the cause. It is not the reducer: since item 1 `ledger.go` changed by comments, the rename, `status()`, `Reservation` and the move of `Digest`, and by no statement the replay runs (`/tmp/f3-5a/pass2/final/ledger-since-item1.diff`). The replay decodes nothing, so the two toolchains differ little here.

**Item 1, the held shape: load with the decoding it had** (`BenchmarkLoad/held`, `json.Unmarshal` on both sides, medians).

| Toolchain | State | Round | Load average, start to end | 1,000 entries | 10,000 | 100,000 |
| --- | --- | --- | --- | ---: | ---: | ---: |
| Go 1.27.0 | before | 1 | 3.39 to 2.77 | 2.91 ms | 68.69 ms | 5.04 s |
| Go 1.27.0 | before | 2 | 3.33 to 3.42 | 2.80 ms | 68.38 ms | 5.06 s |
| Go 1.27.0 | after | 1 | 2.77 to 3.18 | 2.37 ms | 23.29 ms | 243.23 ms |
| Go 1.27.0 | after | 2 | 3.42 to 4.92 | 2.34 ms | 23.28 ms | 245.53 ms |
| Go 1.27.0 | after | 3 | 4.10 to 3.64 | 2.48 ms | 23.26 ms | 245.90 ms |
| Go 1.26.9 | before | 1 | 4.44 to 3.62 | 4.92 ms | 95.68 ms | 6.24 s |
| Go 1.26.9 | before | 2 | 7.23 to 5.99 | 5.13 ms | 97.82 ms | 7.61 s |
| Go 1.26.9 | after | 1 | 3.62 to 4.80 | 4.47 ms | 44.50 ms | 452.46 ms |
| Go 1.26.9 | after | 2 | 5.99 to 5.88 | 4.54 ms | 51.21 ms | 455.41 ms |

Round 2 of Go 1.26.9 ran on a busier machine and shows it in two places: its "before" at 100,000 entries (replay 6.06 s and load 7.61 s, against 5.30 s and 6.24 s in round 1) and its "after" load at 10,000 (51.21 ms against 44.50 ms). The Go 1.27.0 "after" rows are within the range of H.8 (2.33 to 2.46 ms, 22.8 to 23.7 ms, 238 to 249 ms) and close to what you measured (2.34 ms, 23.2 ms, 236.9 ms).

**Item 3: a load per entry, before and after `DecodeEntry`** (`BenchmarkLoad/held`; the median divided by the number of entries, in µs).

| Toolchain | Round | Decoding | Load average, start to end | 1,000 entries | 10,000 | 100,000 | Whole load at 100,000 |
| --- | --- | --- | --- | ---: | ---: | ---: | ---: |
| Go 1.27.0 | 1 | `json.Unmarshal` | 2.77 to 3.18 | 2.37 | 2.33 | 2.43 | 243.23 ms |
| Go 1.27.0 | 1 | `DecodeEntry` | 3.18 to 3.33 | 3.96 | 3.85 | 3.87 | 386.63 ms |
| Go 1.27.0 | 3 | `json.Unmarshal` | 4.10 to 3.64 | 2.48 | 2.33 | 2.46 | 245.90 ms |
| Go 1.27.0 | 3 | `DecodeEntry` | 3.64 to 8.89 | 4.19 | 4.03 | 3.98 | 398.12 ms |
| Go 1.26.9 | 1 | `json.Unmarshal` | 3.62 to 4.80 | 4.47 | 4.45 | 4.52 | 452.46 ms |
| Go 1.26.9 | 1 | `DecodeEntry` | 4.80 to 3.75 | 5.75 | 5.59 | 5.72 | 572.32 ms |
| Go 1.26.9 | 2 | `json.Unmarshal` | 5.99 to 5.88 | 4.54 | 5.12 | 4.55 | 455.41 ms |
| Go 1.26.9 | 2 | `DecodeEntry` | 5.88 to 5.25 | 5.84 | 5.75 | 5.86 | 586.08 ms |

- **What `DecodeEntry` adds:** 1.4 to 1.7 µs per entry on Go 1.27.0, about two thirds on top of the plain load, and 1.1 to 1.3 µs on Go 1.26.9, a little over a quarter. It is one more encoding of the entry and a comparison of the bytes. The cost per entry stays flat with the size.
- **Round 2 of Go 1.27.0 is left out for `DecodeEntry`.** The machine got busy while it ran (load average 4.92 to 8.88): 7.61, 6.16 and 7.18 µs per entry. Round 3 repeats it. Its held rows ran last, in a run that ended at a load average of 8.89, and are 3 to 6 percent above round 1.
- **The 5.12 of Go 1.26.9, round 2** is the disturbed 51.21 ms named above.
- **The replay itself** is 0.35 to 0.51 µs of each figure. The rest is decoding.

**The other shapes** take the same addition (round 1 of each toolchain; µs per entry at 1,000, 10,000 and 100,000 entries).

| Shape | Go 1.27.0, `json.Unmarshal` | Go 1.27.0, `DecodeEntry` | Go 1.26.9, `json.Unmarshal` | Go 1.26.9, `DecodeEntry` |
| --- | --- | --- | --- | --- |
| cumulative | 2.77, 2.57, 2.65 | 4.34, 4.16, 4.21 | 4.93, 5.84, 5.20 | 6.38, 6.20, 6.23 |
| delta | 2.72, 2.72, 2.89 | 4.42, 4.26, 4.42 | 5.04, 5.05, 5.40 | 6.47, 6.28, 7.60 |
| mixed | 3.05, 6.37, 40.72 | 4.78, 8.03, 42.37 | 5.72, 9.70, 55.75 | 14.01, 10.99, 57.19 |

On Go 1.27.0 the addition is 1.5 to 1.7 µs per entry in every cell. The Go 1.26.9 column has three cells that stand out from their neighbours (5.84, 7.60 and 14.01); round 2 gives 4.96, 6.58 and 7.12 for them. The mixed shape is still quadratic, as H.8 says: 4.1 s plain and 4.2 s through `DecodeEntry` at 100,000 entries on Go 1.27.0, and 5.6 s and 5.7 s on Go 1.26.9. Decoding is not what it costs.

**One entry** (`BenchmarkDecodeEntry` and `BenchmarkEncodeEntry`, new; median of 5; a reserve, a hold and a usage entry).

| Toolchain | Load average | `DecodeEntry` | `json.Unmarshal` alone | `EncodeEntry` |
| --- | --- | --- | --- | --- |
| Go 1.27.0 | 3.62 to 4.10 | 3.3, 3.4, 3.6 µs; 1,852 B and 13 allocations (usage: 2,060 B, 20) | 2.0, 1.9, 2.1 µs; 448 B and 3 (usage: 552 B, 6) | 1.7, 1.7, 1.9 µs; 1,561 B and 13 (usage: 1,665 B, 17) |
| Go 1.26.9 | 3.75 to 5.83 | 5.1, 5.2, 5.6 µs; 1,866 B and 21, 1,874 B and 22, 2,091 B and 30 | 4.1, 4.1, 5.1 µs; 816 B and 12, 832 B and 13, 936 B and 17 | 1.4, 1.5, 1.7 µs; 1,209 B and 12 (usage: 1,313 B, 16) |

A first Go 1.27.0 run of these (load average 8.88 to 10.97) gave 5.5, 5.2 and 6.2 µs for `DecodeEntry`; I ran them again on a calmer machine and the table has that run. A second Go 1.26.9 run at a load average of 7.23 gave 5.1, 5.7 and 6.0 µs. `EncodeEntry` does more than the second encoding inside `DecodeEntry`: it also judges the entry and computes the digest.

**Allocation.** A held load of 100,000 entries allocates 364 MB in 1,401,918 allocations through `DecodeEntry` where plain decoding allocated 189 MB in 301,873 (Go 1.27.0), and 367 MB in 2,251,405 where it was 227 MB in 1,251,380 (Go 1.26.9). That is about 1,750 B and 11 allocations more per entry, or 1,400 B and 10. Two things make it up:

- **The second encoding:** 1,400 B and 10 allocations on Go 1.27.0, about 1,050 B and 9 on Go 1.26.9, as the one-entry table shows. It is garbage once the bytes are compared.
- **The decoded entry itself:** about 350 B and one allocation. `DecodeEntry` decodes into a variable that the compiler moves to the heap (`stored.go:77`; `/tmp/f3-5a/pass2/final/escape-go1.27.0.log` and `escape-go1.26.9.log`), where the benchmark used to decode in place into its slice.

**What a loaded ledger keeps** (`BenchmarkLoadedLedgerMemory`: bytes of live heap per entry, the median of three runs that differ by at most 11 bytes). I measured this at 18:14, after the rest of this section was written. Until then this paragraph said that I had not measured it again and expected the H.8 figures. `run-bench-4.sh`, `env4.txt`, `summary/memory.txt`.

| Entries in one root | 1,000 | 10,000 | 100,000 |
| --- | ---: | ---: | ---: |
| Go 1.27.0, cumulative | 963 | 887 | 827 |
| Go 1.27.0, delta | 1,325 | 1,202 | 1,107 |
| Go 1.27.0, mixed | 1,210 | 1,097 | 1,021 |
| Go 1.27.0, held | 945 | 869 | 803 |
| Go 1.26.9, cumulative | 1,002 | 932 | 871 |
| Go 1.26.9, delta | 1,361 | 1,246 | 1,151 |
| Go 1.26.9, mixed | 1,276 | 1,176 | 1,102 |
| Go 1.26.9, held | 969 | 893 | 827 |

- **`DecodeEntry` does not change it.** The same package with plain decoding in the load keeps the same amount on each toolchain, to within 3 bytes per entry at 1,000 entries and 1 byte at the larger sizes.
- **Go 1.27.0 gives the figures of section 5.5 and of H.8.**
- **Go 1.26.9 keeps 24 to 81 bytes more per entry,** about 3 to 8 percent. I did not look for the cause.

**Small roots keep more per entry.** No benchmark of the package has a root smaller than 1,000 entries, and a cache may hold many small ones. So I spread 20,000 entries over roots of one size, loaded them all and measured what they keep together. The benchmark is compiled in through `-overlay` and is not in the package (`/tmp/f3-5a/pass2/final/smallroots/`, 18:29; three runs a cell, which agree to within 0.05 percent). Go 1.26.9; Go 1.27.0 keeps 0.6 to 5 percent less.

| Entries in the root | Bytes kept per root | Bytes per entry |
| ---: | ---: | ---: |
| 1 (a reserve alone) | 2,768 | 2,768 |
| 2 | 3,327 to 4,421 | 1,663 to 2,210 |
| 3 | 3,936 to 5,232 | 1,312 to 1,744 |
| 5 | 5,264 to 6,848 | 1,052 to 1,369 |
| 10 | 10,047 to 13,510 | 1,004 to 1,351 |
| 20 | 19,479 to 25,792 | 973 to 1,289 |
| 100 | 85,207 to 113,796 | 852 to 1,137 |
| 1,000 | 969,467 to 1,361,310 | 969 to 1,361 |

In every row the lower figure is the held shape (a reserve and a lifecycle fact per attempt) and the higher is the delta shape (a reserve and usage events, each in a scope of its own).

- **The 0.8 to 1.3 KB per entry of section 5.5 and of §10.4 holds from about five entries a root upward,** on Go 1.26.9 up to 1.37 KB.
- **Below that the root itself weighs in.** A root with one entry keeps 2.8 KB; with two, 1.7 to 2.2 KB per entry.
- **So a cap counted in entries does not bound the memory at the figure §10.4 gives.** On Go 1.26.9, 50,000 cached entries keep 45 to 68 MB in roots of 1,000 to 10,000 entries, 83 to 111 MB when every root has two entries, and 138 MB when every root is a single reserve. §10.4 says about 40 to 65 MB and calls the bound strict.
- **These figures are the ledger alone, with identifiers of 36 bytes.** What the cache adds for each root is 5b's, and the identifiers of §10.2 are longer.

One way to keep the bound strict with a cap in entries is to count every cached root as two entries more than it has: (n + 2) × 1.4 KB is above every figure of both tables. That is a proposal for the service package, not something this package can do.

**One write to a cached root, on both toolchains** (`BenchmarkWith`, the package on disk). The 5 ms rule of §10.13 had only ever been measured with Go 1.27.0, so I ran this at the end: two rounds a toolchain from the same binaries, 15:38 to 15:45, at a load average of 3.39 rising to 11.38 (`run-bench-3.sh`, `env3.txt`). Each cell has the medians of the two rounds at 1,000, 10,000 and 100,000 entries.

| Shape | Go 1.27.0 | Go 1.26.9 |
| --- | --- | --- |
| cumulative | 121 to 139 µs, 0.95 to 1.00 ms, 15.6 to 17.5 ms | 124 to 132 µs, 0.99 to 1.05 ms, 15.1 ms |
| delta | 199 to 205 µs, 2.02 ms, 30.0 to 32.5 ms | 206 to 215 µs, 2.07 to 2.13 ms, 32.3 to 32.4 ms |
| mixed | 172 to 180 µs, 1.51 to 1.65 ms, 23.7 to 24.3 ms | 177 to 179 µs, 1.60 to 1.68 ms, 25.1 to 25.4 ms |
| held | 123 to 130 µs, 0.96 ms, 13.8 to 14.7 ms | 124 to 134 µs, 0.94 to 0.98 ms, 13.5 to 14.0 ms |

The two toolchains do not differ here: a write decodes nothing, and the bytes and allocations per call are the same on both. At 10,000 entries one write costs 0.94 to 2.13 ms, so the answer to the 5 ms rule stands on the release line the repository builds with. The figures are those of section 5.2 and of H.8, within the spread section 5.1 gives for this machine.

**What this means for design §10.4.** The section gives a load as 2.3 to 2.5 µs per entry and 50,000 entries as about 0.12 s, and says the strict decoder adds to it. Measured:

| | Per entry | 50,000 entries |
| --- | ---: | ---: |
| §10.4 today: Go 1.27.0, `json.Unmarshal` | 2.3 to 2.5 µs | about 0.12 s |
| Go 1.27.0 through `DecodeEntry` | 3.9 to 4.2 µs | about 0.19 to 0.20 s |
| Go 1.26.9, `json.Unmarshal` | 4.5 µs | about 0.23 s |
| Go 1.26.9 through `DecodeEntry` | 5.6 to 5.9 µs | about 0.29 s |

The last row is the one that applies: the repository builds with Go 1.26.x and 5b loads through `DecodeEntry`. Three things to weigh with it. I ran no benchmark at 50,000 entries; the column is half of the 100,000 figure, and the cost per entry is flat. The figures are from darwin/arm64 on a shared machine, not from the server's platform. The benchmark's IDs are 36 bytes, where the entry IDs of §10.2 run to 200 bytes; a longer form costs more to decode and to encode again, and I did not measure by how much. The cost stays linear either way, so the design holds. The figure does not (P.10, item 1). You corrected §10.4 at 16:11, after I wrote this paragraph; the closing note of P.10 has what it says now.

### P.10 What needs your decision

The first three are decisions. Items 4 to 9 are choices I made where your message left room; each is pinned by a test unless it says otherwise, and each needs only a yes or a correction.

**1. The load figure in design §10.4.** It was measured with Go 1.27.0 and plain decoding. The repository builds with Go 1.26.x (`server/go.mod:3`, `.github/workflows/ci.yml`, `Dockerfile:2`), and a load now goes through `DecodeEntry`. On Go 1.26.9 that is 5.6 to 5.9 µs per entry and about 0.29 s for 50,000 entries, against the 2.3 to 2.5 µs and 0.12 s the section gives (P.9). I recommend correcting the figure and leaving the design as it is: the cost is linear, and the cache cap decides how often it is paid. If 0.29 s is too much for a root above the cap, the second proposal at the end of this section is where the time would come from.

**2. A row is read only while the encoder writes the bytes it wrote.** `DecodeEntry` encodes the entry it read and compares. If a later Go release writes one valid entry differently, every row with such an entry is refused and its root is unavailable (§10.5). What I know:

- Go 1.26.9 and Go 1.27.0 write the same bytes for every valid entry I tried: all 1,112,064 Unicode scalar values in a string, and the 16,400 entries of the round-trip test.
- They differ only for a byte that is not UTF-8, which no entry carries: `validateEntry` refuses it, and `EncodeEntry` therefore never writes it.
- Three tests fail in CI before such a release can refuse a row: the pinned form with its digest (`ledger_test.go:547`), the two digests of the unsized reserve and its sized twin (`unsized_test.go:386`), and `TestStoredFormSpellsTextTheSameInEveryToolchain`, which pins the spelling of the 39 characters that have one, a digest over them, and 12 other spellings that must be refused.
- The digest of the 16,400 forms is logged, not asserted. Comparing it across toolchains is a step of `verify.sh`, not a test.

I recommend accepting the three tests as the guard for the first package. The alternative is an encoder of the package's own that writes these same bytes and does not move with the standard library. That is the first proposal below; I have not built it.

**3. The one write the type cannot forbid.** Code outside the package that holds a `*Ledger` can assign a whole Ledger over it (`*a = *b`). No method is involved, so no test of the package can see it; the doc comment forbids it for a shared Ledger. Closing it means an opaque handle, the third proposal below. I recommend leaving it: 5b's cache stores the pointer and replaces the pointer, and a review of 5b can check that nothing assigns through it.

**4. Only `EncodeEntry` knows the two limits.** `With` accepts an entry with an ID of 201 bytes or a form of 70,000, and `DecodeEntry` reads such a row. For `DecodeEntry` that is pinned (`TestDecodeEntryProvesTheFormAndNothingElse`; mutants S40 and S41). For `With` no test says either way. It follows from your rule that every new entry goes through `EncodeEntry` first (§10.2), and the table's own checks stop anything else. Say so if `With` should refuse them too.

**5. `EncodeEntry` measures the ID before it judges the entry.** An entry with a negative amount and an ID of 100,000 bytes is `ErrInvalidEntry` for its length, not `ErrNegativeAmount`, and the refusal does not repeat the ID. The reason is the error text: the ledger's refusals quote the ID.

**6. One sentinel for a wrong version and for wrong bytes.** Both are `ErrInvalidStoredForm`, as you asked for a single sentinel. The text tells them apart: a wrong version reads "form version 2, only version 1 is read". A loader that must branch on the difference would need a second sentinel.

**7. A row in the step 2 form is refused.** It lacks the key "unsized". By §10.5 its root is then unavailable, so a test database filled before 5a has to be rebuilt, not read.

**8. Two things `DecodeEntry` and `Digest` do not do.** `Entry.Digest()` still digests any entry and does not judge it; `EncodeEntry` judges. And when `encoding/json` refuses the bytes, its error is in the message as text only, so `errors.Is` finds `ErrInvalidStoredForm` in the chain and nothing of `encoding/json`.

**9. The reading of a reservation.** I named the struct `ReservationProjection` after `Projection` and gave it JSON keys in the same style (`status`, `unsized`, `reserved_ticks`, `actual_known_ticks`, `unspent_ticks`, `overrun_ticks`). Three choices in it:

- `Reserved` stays the amount of the reserve entry after the settlement; `Unspent` is what goes to zero.
- A settled reservation keeps its `Overrun`, as the ledger's `Overrun` keeps it.
- A sized reservation of 0 is not unsized: `Unsized` is false, and everything it spends is overrun.

**Proposals only.** I built none of these.

- **An encoder of the package's own.** It would append the keys and values of an entry to a buffer in the stored order and write the same bytes as today, which the existing tests would prove. The stored form would then no longer depend on the Go release (item 2), and encoding would allocate once.
- **A decoder that compares without a second allocation.** With that encoder, `DecodeEntry` could encode into a reused buffer and compare. The 10 or so allocations per entry of the second encoding would go (P.9). I have not measured what it would save in time.
- **An opaque handle for the ledger.** `Ledger` would become a small value that holds an unexported pointer, so that an assignment copies a handle and cannot overwrite a shared state (item 3). It changes the signatures of `NewLedger` and `With` and every test that holds a `*Ledger`; `Reduce` and the seven importers would not change.

**Since decided (added at 18:40 on 2026-11-18, after P.11 was written).** No message about this section reached me. I found your rulings in the task files, which you edited while I wrote: `design.md` §10.4 in its text of 16:11:58 and `implement.md` line 203 in its text of 16:58:38. As I read them:

- **Item 1:** §10.4 now gives 5.4 to 6.2 µs per entry on Go 1.26.x, about 0.3 s at 50,000 entries and about 0.6 s at 100,000, and 3.9 to 4.6 µs on Go 1.27.0, from your own runs on both toolchains. The old figure is withdrawn. P.9 has 5.6 to 5.9 µs and 3.9 to 4.2 µs, which lie inside your ranges.
- **Item 2:** accepted. The three tests are the gate. If one fails after a Go upgrade, nothing is released until the package has an encoder of its own that writes today's bytes, and the expected values are not changed. No encoder is written now.
- **Item 3:** the type stays as it is. The rule is that the cache stores the pointer and replaces the pointer, and the check of 5b looks through the service package for an assignment through one.
- **Item 4:** `With` and `DecodeEntry` do not repeat the two limits.
- **Items 5 to 9:** as implemented. The line names items 5, 6, 7 and 9; I take item 8 to be the fifth of "the other five".
- **The three proposals** are not to be built.

None of this changes the package, and I changed nothing in it after reading this. Items 1 to 9 above stand as I wrote them, so that their numbers still match your references.

**One figure more for §10.4, measured after those rulings.** It is not one of the nine items, and it is new to you. §10.4 counts 0.8 to 1.3 KB of heap per cached entry and puts the cap of 50,000 entries at about 40 to 65 MB. That holds for roots of about five entries or more. A root with one entry keeps 2.8 KB and a root with two keeps 3.3 to 4.4 KB, so 50,000 cached entries can keep 138 MB (P.9, "Small roots keep more per entry"). With the identifiers of §10.2 a stored reserve is 8 bytes longer and a usage entry an estimated 100 to 200 bytes (section 5.5), so I expect the 138 MB to move little and the figure for large roots to rise. I measured neither. The cap belongs to the service package, so this is for you or for 5b to decide; nothing in this package changes for it.

### P.11 What this supersedes above

Apart from the one line in the Summary, neither the report nor the first addendum was edited. These are the places where they are no longer the last word. H.9 lists what the first addendum superseded in the report; this table goes on from there. Every line number above this addendum is one of the file of its time.

| Place | It says | Now |
| --- | --- | --- |
| Header, State; H.9 | Complete and frozen, 15 files; then 17. | 21 files. The last edit of the package was `stored_test.go` at 08:11:28 on 2026-11-18 (P.12). |
| Summary, Tests; section 4.1; H.1; H.7 | 89 pass; then 99. Everything ran with go1.27.0. | 112 pass, 0 fail, 0 skipped, on Go 1.27.0 and on Go 1.26.9 (P.8). |
| Summary, Mutation run; appendix A; H.1; H.5 | 62 mutants; then 100, of which the whole suite catches 99. | 180. The plain run catches 171, and P.7 says how each of the other nine is caught. |
| Section 1.1 | "Nothing exported was removed or renamed." `With` "judges a batch exactly as `Apply` does". | `Apply` is no longer exported. `With` judges a batch as it did, and its comment now states the rules itself (P.2, P.4). |
| Section 1.2 | `(*Ledger).Apply`: same signature and same results. The doc comment of `Ledger` names `Apply` as the one method that writes. | The unexported `apply`, used by `Reduce` and by the tests. The doc comment is the one quoted in P.4. |
| Section 1.3; section 7.5; H.11 | 15 files and their hashes; then 17. | 21 (P.12). |
| Section 2, "Rows in the old form" | `json.Unmarshal` is lenient: a row without the key `unsized` decodes and replays without error. | `DecodeEntry` refuses such a row (P.5; P.10, item 7). |
| Section 4.2; H.7 | No floating point and standard library only, on 15 files; then on 17. | The same on 21 files. `bytes` is a new direct import (P.8). |
| Section 5.1; H.8 | The machine has go1.27.0. | So every figure of section 5 and of H.8 is a Go 1.27.0 figure. What decodes runs at about half that speed on the release line the repository builds with; a replay and a write cost the same on both (P.9). |
| Section 5.3; H.1; H.8; H.9, "When it is paid"; H.10, item 4 | A load costs 2.3 to 2.5 µs per entry: 238 to 249 ms at 100,000 entries. | That is Go 1.27.0 with `json.Unmarshal`. Through `DecodeEntry` it is 3.9 to 4.2 µs on Go 1.27.0 and 5.6 to 5.9 µs on Go 1.26.9 (P.9). |
| Section 5.5; H.8, "Memory" | Count 0.8 to 1.3 KB of live heap per cached entry; 100,000 cached entries are 80 to 130 MB. | That is Go 1.27.0 and roots of 1,000 entries or more. `DecodeEntry` does not change it. On Go 1.26.9 it is 0.83 to 1.36 KB, and roots of fewer than five entries keep more: 2.8 KB for a root of one entry (P.9). |
| H.7 | "The 5 benchmarks are unchanged." | 7 benchmarks: `BenchmarkDecodeEntry` and `BenchmarkEncodeEntry` are new, and `BenchmarkLoad` decodes through `DecodeEntry` (P.9). |
| Section 6.2, questions 2 to 8 | Open. | Answered by you. 2: the version stays 1, now as `EntryFormVersion`. 3: confirmed as implemented. 4: item 2 of this pass (P.4). 5: nothing for this package. 6 to 8: rules for 5b and step 6. |
| Section 6.4, proposals 1 to 7 | Nothing built. | Built: 1 (`DecodeEntry`), 2 (`EntryFormVersion`) and 3 (`Reservation`). 5 is built in `EncodeEntry` and not in `With` (P.10, item 4). 6 is answered another way: `Apply` is unexported. Of 4, the limit is the constant `MaxEntryIDBytes` and the builders are not built. 7 is not built. |
| Section 6.4, proposal 1; H.10, item 4 | The strict decoder is put at 1.5 to 2 µs more per entry, "and nobody has measured it". | Measured: 1.4 to 1.7 µs more on Go 1.27.0, 1.1 to 1.3 µs on Go 1.26.9 (P.9). |
| H.2; H.3 | `applyHold` at `ledger.go:340-352`, the check at `:344`. | `:377` and `:381`. The lines moved; the code did not change (P.3). |
| H.5 | Three modes: all, behaviour, earlier. | Four: `plain`, `race`, `behaviour` and `copy`. "earlier" no longer builds and was dropped (P.7). |
| H.10, item 1 | A proposal: one table of the five hold reasons. | Not adopted, by your ruling. The three switches are as they were. |
| H.10, item 2 | "`With` clones the ledger before it applies, and so does `Apply`." | The note is now a rule of design §10.4. The method is `apply`, and no code outside the package can call it. |
| H.1; H.11, "Not started" | The other three items; "That message has not reached me." | It reached me at 05:45. All three are built (P.4 to P.6). |
| H.11, "Not verified" | The cost of a strict decoder. | Measured (P.9). The other points of that list stand, and P.12 repeats those that matter here. |

Sections 3, 4.3 to 4.6, 5.2, 5.4 to 5.6, 6.1 and 6.3, appendix A and H.3, H.4 and H.6 describe their rounds. Two things changed under them in this pass. Where they write `Apply`, the method is now `apply`. And some of the tests they describe were edited: those that pinned a stored form or a digest now obtain it through `EncodeEntry` (P.5), and `checkLedger` also compares every reservation's reading with the projection (P.6). What those tests assert about the ledger is what it was.

### P.12 Housekeeping

**What I changed or created.**

- **The package**, `server/internal/budget/` of the scratch worktree `/Volumes/artisan/code/2026/mc-f3-budget-5a-20261117`: 21 files, 282,055 bytes, 7,105 lines. Against the 17 files of H.11, 4 are new, 7 changed, 4 changed only by the rename of `Apply`, and 6 are byte-identical. The table at the end of this section says which. The last edit was `stored_test.go` at 08:11:28 on 2026-11-18. `/tmp/f3-5a/pass2/final/delta/` holds the difference of each of the 11 changed files against the snapshot `/tmp/f3-5a/pass2/pre/`, once as it is and once with the rename taken out.
- **The shared checkout:** this report and nothing else. In it, the one line under the Summary and everything from the heading "Addendum: second pass" down.
- **My memory directory**, `/Users/artisan/.claude/projects/-Volumes-artisan-code-2026-multica-0-4-37/memory/`, which is outside every repository: one new note, `local-go-is-newer-than-the-repo-toolchain.md`, and its line in `MEMORY.md`. The note says that the local `go` is 1.27.0 while the repository builds with Go 1.26.x, how to run Go 1.26.9 from the module cache without a download, and what differed in this package. Delete it if you would rather not have it.
- **Scratch files** under `/tmp/f3-5a/pass2/`: about 6,850 files, 136 MB. A reboot removes them. The parts that are evidence: `final/` (the verification runs of P.8, `copyback/`, `bench/`, `smallroots/`, `delta/`, `readings/`), `mutants/` and `mutants-first-run/` (P.7), `item2/`, `item3/` and `item4/` (the red and green runs of each item), `pre/` (the 17 files before items 2 to 4), and the scripts `verify.sh`, `mutate.py` and `run-mutants.sh`.

**In this pass no file other than these 21 was written into the package directory.** The stubs of the red runs, the counting file behind the figures of P.6, the two older states of the package that P.9 measures, its benchmark of small roots and every mutated source reached the compiler through `-overlay` from a file under `/tmp`. The control X00 and the mutants X04 to X06 need the source on disk; for them the harness copies the package, `pkg/` and the two module files to its own directory under `/tmp` and mutates the copy (`mutate.py`, `copied_module`). Two things on disk agree with this. The directory's own modification time is 08:11:28, the time of the last edit, so no file was created, removed or renamed in it during the mutation runs, the verification runs or the benchmarks. And `find` shows no file of the scratch worktree outside that directory that is newer than the start of this pass (05:51:50).

**What I did not touch.**

- **The scratch worktree outside the package.** `git status --short` prints one line, `?? server/internal/budget/`. HEAD is `86fea651cc5d09360327b6cf0f8357c1f360070d`, detached, as you set it up. I committed, stashed, pushed and merged nothing and switched no branch. `git stash list` prints the same 10 stashes of the repository as at 08:44 (P.8); none is mine.
- **The step 4 tree**, `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`. I read it: hashes, `git status`, `grep`, and the overlay type-check of P.8, which wrote to `/tmp` only. Its budget directory held the same 12 first-round files at 06:01, 08:46, 08:47 and 10:48. Since 12:17:40 it holds 21 files; at 12:43 and at 17:29 they were byte-identical to the 21 of the table below. I did not write them: `implement.md` records that you copied the package back and ran the database-backed budget tests there.
- **The shared checkout** apart from this report. `design.md` and `implement.md` I only read; to read them at one fixed state I copied them to `/tmp/f3-5a/pass2/final/`.
- **No database, no agent CLI, no provider call.** The Go 1.26.9 runs used the toolchain that was already in the module cache, with `GOTOOLCHAIN=local GOPROXY=off`. With the same two settings Go 1.27.0 builds the whole server, vets and tests the package and lists the same 107 dependencies (`/tmp/f3-5a/pass2/final/offline-go1.27.0.log`, 17:37), and no log under `/tmp/f3-5a/` has a line "go: downloading". Nothing this pass ran needed a download.

**What I read in the task files, and in which state.** Both files changed while I worked.

- **`design.md`:** §10.1 to §10.13 and §12.4 to §12.8 in full, in the text of 11:16:33 (133,959 bytes). In the text of 16:11:58 (142,729 bytes, sha256 `f3b58e6c…`) I read §10.4 in full again, the lines of §10.1, §10.2, §10.5, §10.8, §10.13 and §12.5 that concern this package, and every line that names one of its identifiers. The text of 16:55:01 (142,909 bytes, sha256 `ea2a1dfd…`) is the latest I saw. It differs from the one before in a single line of §5, which is about deleting an issue (`/tmp/f3-5a/pass2/final/design-1612-1655.diff`).
- **`implement.md`:** line 27, lines 188 to 195 and line 203, in the text of 16:58:38 (56,341 bytes, sha256 `a9421426…`).
- **What I found.** No rule that the package does not follow, and none that is wrong or impossible against the code. §10.4 as it reads now describes `With`, the unexported `apply`, `EncodeEntry`, `DecodeEntry` and the two limits as the package has them. Your rulings on P.10 are in both files (P.10, closing note).
- **For the removal of the scratch worktree** that `implement.md` line 192 names as outstanding: it loses nothing. All I have in that worktree is the 21 files, and the step 4 tree holds them byte for byte. The evidence is under `/tmp`, not in the worktree. To run the checks again afterwards, give `verify.sh` the step 4 worktree and `mutate.py` its `server` directory; `run-mutants.sh` has the scratch path written in two lines, and the benchmark binaries under `final/bench/bin/` need no tree.

**Not verified.**

- **The database-backed tests of the step 4 tree.** I ran none of them, in either tree. `implement.md` lines 191 and 194 record your runs after the copy-back: the budget cases of the service (31) and of the handler (26) pass, also under `-race`, on Go 1.27.0 and on Go 1.26.9.
- **Any platform but darwin/arm64.** Every test and every figure is from this machine. The server runs on linux. Of the Go 1.26 line I ran 1.26.9 only.
- **The mutants** are written by hand and each ran once (P.7).
- **The benchmarks** ran on a shared machine under a load average of 3 to 21, with identifiers of 36 bytes. Nothing ran at 50,000 entries or with the identifiers of §10.2 (P.9). The memory figures do not depend on the load; the times do.
- **What the cache itself adds per root,** and anything under contention. There is no cache yet. The package's own test reads one shared ledger from several goroutines (P.4) and nothing more.
- **Why Go 1.26.9 keeps more per entry and decodes at half the speed.** I measured both and did not trace either into the standard library beyond the build setting named in P.5.
- **A nil `*Ledger`.** Its methods are not written for one, as before this pass. I read the code and wrote no test for it.

**The 21 files** (`/tmp/f3-5a/pass2/final/sha256.txt`, written at 08:44; hashed again at 17:29 and before this table was written, with the same result). The last column compares each file with the 17 of H.11; "renamed" counts the places where `Apply` became `apply`, and the lines are those that changed for another reason.

| File | Bytes | Lines | sha256 | Against H.11 |
| --- | ---: | ---: | --- | --- |
| `bench_test.go` | 9,313 | 301 | `cdfcd6643f4dcd514cc60c9da352e686ef31a4cbad934991104163185bb459da` | changed: 61 lines added, 4 removed; was 7,655 bytes, `4d1a50a7…` |
| `decimal.go` | 2,451 | 99 | `6db8462df4606ab587b1ab9e50a8b5ee0eade386927bb5893e9f80ebc3a5f26a` | same |
| `decimal_test.go` | 3,945 | 132 | `2c95daf7225b4142b29926f45344c4aea10cab0bbd9718fd1c6f91071acb4db4` | same |
| `edge_cases_test.go` | 15,520 | 366 | `f5218993e3ad238afe980bc05c8464e0c222dc17746b52b894eb0af25c503776` | changed: 6 words renamed, and 6 lines added, 4 removed; was 15,264 bytes, `8ff64ed0…` |
| `hold_test.go` | 29,547 | 682 | `565687875a8a2acbca72a7869191318e3da4becd7c2ce775982455efccb825c0` | rename only, 4 words; was 29,547 bytes, `3a9a7e6a…` |
| `holdset_test.go` | 6,812 | 198 | `def6475f37df70f09d368ecda25324227dc659153302e9a1763631924cbebc80` | rename only, 2 words; was 6,812 bytes, `6eb3388c…` |
| `invariants_test.go` | 17,054 | 449 | `a581a2fa5f375fb23c3d5a32e6c78c582835fc87e589e4892f7bf7df0c1b037d` | changed: 4 words renamed, and 57 lines added, 0 removed; was 14,396 bytes, `d302dc61…` |
| `ledger.go` | 17,114 | 537 | `a6a7cf5e1afaa22f45653e56ec844f74c4aaf80eed68072c6a78523f554b148d` | changed: 5 words renamed, and 48 lines added, 17 removed; was 16,095 bytes, `6f7e1769…` |
| `ledger_test.go` | 24,077 | 563 | `1031daac8b542512784b9be6ac79469d968a1d5a8f0daa4991de9b8d80fff249` | changed: 9 words renamed, and 23 lines added, 7 removed; was 23,544 bytes, `980f783d…` |
| `model.go` | 11,980 | 288 | `6b7dafbc841d5a36e60567d13c09fcf1635aaff5d5c35270f1b3da38fc9d3cf2` | changed: 39 lines added, 0 removed; was 10,157 bytes, `ec9faca7…` |
| `price.go` | 6,572 | 180 | `a2b4cd68128b4877d39620b61419973b0f6dec0d254e683c837b86125379b31a` | same |
| `price_test.go` | 12,033 | 269 | `b57dd9df6dd8d072ac2a50f762cc294a684b00e29e81b8b4c225cfbc12005646` | same |
| `readonly_test.go` | 8,596 | 233 | `1959ca56583c0cc92781a04c8dc41db508306b4125a0746af854c78772129f67` | new |
| `reservation_test.go` | 9,592 | 205 | `589f1a2f13fc292cb342e564cb099073730bdc23a1042beb7ffb9f3dfbc67ea9` | new |
| `scale_test.go` | 562 | 16 | `aaac212c787f403c89670c0dbb18287380440244d561badada6f23f7e82473c2` | same |
| `stored.go` | 4,329 | 109 | `b54a82608c00c8a555fcb1365179870d96d587fe23f718126ab10f7b280768cd` | new |
| `stored_test.go` | 32,116 | 695 | `b216c299a68fe2aac7cb559823e2fc3b32d65d0728ed8c4a7826033e5e0873bc` | new |
| `unsized_test.go` | 21,649 | 510 | `fa062fe047e52bd402db0e14373b0b237d7c6f2ae6072cb990c7259efe01b531` | changed: 12 words renamed, and 13 lines added, 8 removed; was 21,308 bytes, `a47777c8…` |
| `usage.go` | 11,418 | 333 | `90e000c435e29e47029abf1d05900c12387c004becd118749d7c3b99250ed2d9` | same |
| `usage_test.go` | 18,548 | 452 | `fcbd12daff38b47225e2be496f9eae2123621aecc8a917cde6dfb39d98dbd97a` | rename only, 2 words; was 18,548 bytes, `e323602f…` |
| `with_test.go` | 18,827 | 488 | `425bf91e48a686d96d32d13f97af937ef59bdcb24d6dfa0ea569c673e7daa6d8` | rename only, 9 words; was 18,827 bytes, `1f742fac…` |

Total: 21 files, 282,055 bytes, 7,105 lines.
