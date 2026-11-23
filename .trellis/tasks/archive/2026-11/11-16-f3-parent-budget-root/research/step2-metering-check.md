# F3 step 2 (pure metering): independent check

Date: 2026-11-16. Reviewer: `trellis-check` sub-agent. Code under review: `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server/internal/budget/` (untracked, branch `codex/f3-budget-root-20261116`, HEAD `86fea651c`). I formed this view from the code, the matrix and the contract, and read the implementer's report (`step2-metering-implementation.md`) only afterwards.

## Verdict

**Sound to build persistence on, with the three fixes below kept and four conditions carried into steps 3 and 6.**

- Every pure matrix row (B15, B17–B23, B24 pure half, B24a, B25–B30, B48) is implemented with the matrix numbers, and the tests assert those numbers as literals.
- I found three defects that give a wrong ledger result and fixed them test-first inside the package. One of them reverses a choice the implementer made deliberately (W1/W2), so it needs your decision.
- The original 54 tests killed 97 of 108 targeted mutants. With the tests I added, 112 of 113 are killed; the one survivor is an equivalent mutant.
- Nothing is committed, staged or pushed. `git status --short` in the worktree still shows only `?? server/internal/budget/`.

Conditions for later steps (details under "Design risks"):

1. Step 3 has to choose how a root's ledger is loaded: the package has no snapshot/restore, so today the only way is to replay the root's whole history on every write.
2. Step 6 needs a strict wire decoder; the stored-form decoder accepts missing fields, duplicate keys and unknown fields.
3. `CorrectionAuthorized` and the three release proofs are plain caller-settable fields; the handler must derive them server-side, and must look up a stored entry before deriving authority for a resend.
4. The recorded S2 prerequisite (no entry resolves a quarantine or gap) is wider than recorded: see D1.

## Verification

Run from `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server`, go1.27.0 darwin/arm64, after my changes.

| Command | Exit | Output file | Result |
| --- | --- | --- | --- |
| `gofmt -l internal/budget` | 0 | `/tmp/f3chk-final-gofmt.txt` | empty |
| `go vet ./internal/budget/...` | 0 | `/tmp/f3chk-final-vet.txt` | empty |
| `go test ./internal/budget/... -count=1 -v` | 0 | `/tmp/f3chk-final-test.txt` | RUN 69, PASS 69, FAIL 0, SKIP 0 |
| `go test -race ./internal/budget/... -count=3` | 0 | `/tmp/f3chk-final-race.txt` | `ok ... 13.437s` |
| `go build ./...` | 0 | `/tmp/f3chk-final-build.txt` | empty |

Before my changes the same commands gave gofmt 0, vet 0, and 54 RUN / 54 PASS / 0 FAIL / 0 SKIP (`/tmp/f3chk-test-base.txt`).

## Findings

### Wrong ledger result (all three fixed)

**W1. A cumulative correction was charged to whichever reservation sent it.** Fixed at `usage.go:215-219`.

- Input: reserve `r1` 100 and `r2` 100. One session: `e1` seq 1, invocation `i1`, under `r1`, reported 180. `e2` seq 2, invocation `i2`, under `r2`, baseline `e1`, reported 200 (so `r2` spent 20). Then an authorised correction `e3` seq 3 **under `r1`**, superseding `e2`, reported 180.
- Expected: the 20 that the correction removes was `r2`'s, so A=180, R=100, overrun=80 (A+R+H=280). Contract §5.2 also says a correction must not be applied to another attempt by default.
- Actual before the fix: accepted, A=180, **R=80, overrun=60** (A+R+H=260). `r1` went to 160 and `r2` stayed charged for 20 it no longer spent. The root looked 20 lighter than it is.
- The implementer's report states this as the intended rule ("the reduction comes off the correcting entry's reservation"). I changed it: a correction sent under a different reservation than its original is now quarantined as `correction_unresolved`. The same correction sent under `r2` is applied and gives A=180, R=100, overrun=80.

**W2. A delta correction moved spend to another reservation.** Fixed at `usage.go:285`.

- Input: reserve `r1` 100 and `r2` 100; `d1` under `r1` reported 180; `d2` under `r2` reported 20. Then an authorised `d3` **under `r2`** superseding `d1` with 170.
- Expected: `r1` spent 170, `r2` spent 20: A=190, R=80, overrun=70.
- Actual before the fix: A=190, **R=100, overrun=90**. `r1` showed 0 spent and `r2` showed 190 of 100.
- Now quarantined as `correction_unresolved`; the same correction under `r1` gives A=190, R=80, overrun=70.

**W3. Identifiers with invalid UTF-8 made a replay from storage differ from the live ledger.** Fixed at `ledger.go:180-194` and `price.go:93-103`.

- Input: `reserve` entries with ID `"a\xff"` / reservation `"r\xff"` and ID `"a\xfe"` / reservation `"r\xfe"`, 1 tick each.
- Live ledger before the fix: both accepted, R=2, two attempts.
- Their stored forms and digests were byte-identical (`encoding/json` replaces each bad byte with U+FFFD; both digests `6de462c4…4014`). Replaying the two stored forms gave **R=1, one attempt**, with the second entry reported as a replay.
- The same collision applied to `PriceSnapshot.Digest()`.
- Now every free-text field of an entry is refused with `ErrInvalidEntry`, and of a price snapshot with `ErrInvalidPrice`. The tests walk the structs by reflection, so a string field added later is covered.

### Weak tests (all closed by added tests)

The matrix-row tests themselves are sound: I recomputed every row's numbers from the matrix text and they match the literals in the tests, and both SHA-256 goldens match a hash I computed independently in Python. No test derives an expectation from the code under test. The gaps were around the rows:

| Mutant that survived the original 54 tests | Row | Now killed by |
| --- | --- | --- |
| Unauthorised delta correction applied (U18) | B28/B29 | `TestDeltaCorrectionWithoutAuthorityTargetOrScopeIsQuarantined` |
| Delta correction across scopes applied (U20) | B28 | same |
| Estimated running total allowed to go down (U14) | B27 | `TestEstimatedRunningTotalCannotGoDownUnexplained` |
| Reported total does not clear the estimate (U27) | B22 | `TestReportedTotalReplacesTheEstimateAndALaterEstimateCountsTheExcess` |
| Estimate and actual counted twice (U30) | B22 | same |
| Same hold reason recorded twice (L13) | B18 | `TestRepeatedHoldReasonIsRecordedOnce` |
| Internal A+R+H total clipped after a settled overrun (L09) | B20 | `TestOverflowBoundCountsASettledOverrun`, both random invariant tests |
| Reported cost may carry a price version/digest (U39) | B24 | `TestUsageShapesThatAreRejected` |
| Overlap entry may carry a correction/baseline (U43) | B15 | same |
| Self-correction, authority without target (U44) | B28 | same |

U18 was the one that mattered: the B28 requirement that a correction be authorised was untested for delta mode.

Two smaller observations, not changed:

- B25, B26 and B28 assert "the originals are kept" by reading the unexported `l.entries` (`usage_test.go:136,175,234`). There is no public way to read a stored entry back, so the tests cannot be written otherwise (see D2).
- The implementer's red run stopped at the `apply` helper before the numeric assertions, as their report says. My mutation run is the evidence that those assertions bite.

### Design risks (not changed)

**D1. Unknown items are permanent, and wider than the recorded S2 prerequisite.**

- A quarantined ambiguity stays in the unknown set even after a correctly baselined event later brings the scope to the right total (A=200 in B30, `rem` still null for good).
- A batch `[final usage, settle]` is rejected whole with `ErrReleaseProofIncomplete` when the reservation carries any non-lifecycle unknown (`ledger.go:282-286`), so the final usage in that batch is lost too. Callers must send usage and settle separately, and there is no "can this settle?" query.
- Estimate-only usage on a reservation makes it unsettleable.
- A quarantined or stale entry keeps its entry ID and its sequence (`usage.go:194`). A producer that fixes and resends it needs a new ID and a new sequence; the same ID with fixed content is `ErrConflict` for the whole batch.

**D2. Persistence shape.**

- `Ledger` has only unexported state and no snapshot/restore. A DB transaction must load and replay the root's full entry history under the root row lock before each `Apply`.
- `Apply` clones the whole ledger per call (`ledger.go:103`), and settle/hold scan the unknown list linearly.
- The unknown set grows per event, not per scope: 2,000 cost-less cumulative events on one scope gave 2,000 unknown items and took 0.40 s applied one by one.
- No accessor returns a stored entry, a reservation's state, or a receipt by ID.

**D3. Identifiers the contract asks for (§5.1/§5.2) that `Entry` does not carry.**

- No attempt ID separate from the reservation ID; `Coverage.IncludedAttempts` is `len(reservations)` (`ledger.go:363`).
- No producer epoch, payload digest, adapter/normalizer version, observed/reported/received time, partial/final flag, final watermark or batch key.
- Meter scope and invocation are present and required on every usage entry.
- `CorrectionAuthorized` (`model.go:121`) is part of entry equality and of the digest. If authority changes between a send and its resend, the resend is `ErrConflict`, not the original receipt.
- `ReleaseProof` is three caller-settable booleans.

**D4. Stored-form decoding is lenient** (standard `encoding/json` behaviour): a missing `reserved_ticks` decodes as 0, a duplicate key takes the last value, keys match case-insensitively, unknown fields are ignored. A reported cost with `ticks` missing decodes as 0 and is classified `zero_without_basis`, so it does not become a known zero.

**D5. Dependency on `server/pkg/agent`: I mildly disagree with it.**

- For: it is one constant with one owner, and the non-stdlib closure is 13 packages against 199 for `internal/metrics`.
- Against: the full closure is 146 packages, 144 of them from `pkg/agent`, including `os/exec` and `net`, for a package described as pure. It also blocks `pkg/agent` from ever importing budget types for the step 6 envelope.
- `internal/metrics/pricing.go:12` is already a second literal, not a re-export as the implementer's report says, so "one literal" does not hold today.
- I would declare the constant locally and assert equality with `agent.CostUSDTicksPerUSD` in an external test package. I did not change it, since it is a recorded choice.

**D6. Stored form and digest are "whatever Go's `encoding/json` emits"** (`price.go:121-122`): `<`, `>`, `&`, U+2028 and U+2029 are escaped. A verifier in another language must reproduce that. Identifiers may contain NUL, which PostgreSQL `text` and `jsonb` cannot store, and have no length limit.

**D7. Attribution follows arrival order.** If `e2` (seq 2, `r2`, 50) arrives before `e1` (seq 1, `r1`, 30), `e1` is stale and `r2` carries all 50. The root total is right; the split between reservations is not.

**D8. Smaller API points.**

- `Remaining` returns a number while open shadow reservations exist (recorded reading), and returns nil for a negative limit, which looks the same as "unknown" (`ledger.go:412`).
- `ErrConflict` also means "reservation already open" (`ledger.go:247`); `ErrReleaseProofIncomplete` also means "unknown usage outstanding".
- `PriceSnapshot.EffectiveFrom/Until` are unvalidated strings, and an estimated cost entry need not carry its price version and digest, so the DB half of B24 must enforce that.
- A late older reported value is dropped as stale when the head has no reported total (the scope stays unknown, so nothing is understated silently).

### Notes

- `Coverage.IncludedAttempts` is a JSON number; it is a small count, so this is safe.
- `sc.actualBy[...]+value-sc.actual` (`usage.go:222`) can wrap in the intermediate sum; the final value always fits, so the result is correct.
- The check at `ledger.go:308` is redundant: unreserved actual never exceeds the A+R+H total checked two lines later. That is the one surviving mutant (L31).

## Mutation table

Each mutant is a copy under `/tmp/f3chk/mut/`, wired in with `go test -overlay`; the worktree was never written by the mutation runs. Package checksums were identical before and after the first run (`/tmp/f3chk/sha-before.txt`), and `git status --short` was unchanged. The "Original" column ran against the code as delivered, the "Final" column against the package with my fixes and tests. Driver: `/tmp/f3chk/mutate.py`; raw results: `/tmp/f3chk/mut/base-results.json`, `base2-results.json` (three mutants I had to rewrite to compile), `final-results.json`.

Totals: original tests 97 killed / 11 survived of 108; final tests 112 killed / 1 survived of 113.

| ID | Row | File | Mutation | Original 54 tests | Final 69 tests |
| --- | --- | --- | --- | --- | --- |
| L01 | B17/B19 | `ledger.go` | unspent() ignores actual (A and R double count) | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB17PartialActualMovesReservedToActual` (+8) | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB17PartialActualMovesReservedToActual` (+16) |
| L02 | B18 | `ledger.go` | hold does not move R to H | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB18LostContactTurnsReservedIntoUnknownHold` (+3) | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB18LostContactTurnsReservedIntoUnknownHold` (+5) |
| L03 | B18 | `ledger.go` | Remaining gives a number while unknown is non-empty | `TestB18LostContactTurnsReservedIntoUnknownHold`, `TestB21MissingCostStaysUnknownBesideAKnownCost` (+4) | `TestB18LostContactTurnsReservedIntoUnknownHold`, `TestB21MissingCostStaysUnknownBesideAKnownCost` (+7) |
| L04 | B19 | `ledger.go` | settle releases the whole reservation | `TestB19FinalEvidenceSettlesAndReleasesOnce`, `TestB20FinalCostAboveReservationIsRecordedAsOverrun` (+1) | `TestB19FinalEvidenceSettlesAndReleasesOnce`, `TestB20FinalCostAboveReservationIsRecordedAsOverrun` (+4) |
| L05 | B19/B26 | `ledger.go` | replay not flagged | `TestB19FinalEvidenceSettlesAndReleasesOnce`, `TestB26DuplicateAndLateOlderValueChangeNothing` (+3) | `TestB19FinalEvidenceSettlesAndReleasesOnce`, `TestB26DuplicateAndLateOlderValueChangeNothing` (+4) |
| L06 | B19 | `ledger.go` | second settle (new id) releases again | `TestSettleAndHoldEndAReservationOnce` | `TestRandomDeltaLedgersMatchANaiveModel`, `TestRandomInterleavingsKeepTheInvariants` (+1) |
| L07 | B20 | `ledger.go` | actual clipped to the reservation | `TestB20FinalCostAboveReservationIsRecordedAsOverrun`, `TestLateUsageAfterSettlementStillCounts` | `TestB20FinalCostAboveReservationIsRecordedAsOverrun`, `TestCumulativeCorrectionThroughAnotherReservationIsQuarantined` (+6) |
| L08 | B20 | `ledger.go` | overrun not reported | `TestB20FinalCostAboveReservationIsRecordedAsOverrun`, `TestLateUsageAfterSettlementStillCounts` | `TestB20FinalCostAboveReservationIsRecordedAsOverrun`, `TestCumulativeCorrectionThroughAnotherReservationIsQuarantined` (+6) |
| L09 | B20 | `ledger.go` | settled weight clipped (internal committed only) | **SURVIVED** | `TestRandomDeltaLedgersMatchANaiveModel`, `TestRandomInterleavingsKeepTheInvariants` |
| L10a | B19 | `ledger.go` | settle without final-usage proof | `TestSettleRequiresEveryProof` | `TestSettleRequiresEveryProof` |
| L10b | B19 | `ledger.go` | settle without scope-complete proof | `TestSettleRequiresEveryProof` | `TestSettleRequiresEveryProof` |
| L10c | B19 | `ledger.go` | settle without no-further-execution proof | `TestSettleRequiresEveryProof` | `TestSettleRequiresEveryProof` |
| L11 | B19 | `ledger.go` | settle allowed with unresolved usage unknown | `TestSettleIsRefusedWhileUsageOfTheReservationIsUnknown` | `TestSettleIsRefusedWhileUsageOfTheReservationIsUnknown` |
| L12 | B18 | `ledger.go` | hold frees the unspent reservation | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB18LostContactTurnsReservedIntoUnknownHold` (+3) | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB18LostContactTurnsReservedIntoUnknownHold` (+6) |
| L13 | B18 | `ledger.go` | same hold reason recorded twice | **SURVIVED** | `TestRepeatedHoldReasonIsRecordedOnce` |
| L14 | B19 | `ledger.go` | hold after settle re-holds | `TestSettleAndHoldEndAReservationOnce` | `TestRandomDeltaLedgersMatchANaiveModel`, `TestRandomInterleavingsKeepTheInvariants` (+1) |
| L15 | B24a | `ledger.go` | batch not atomic | `TestB24aOverflowRejectsTheWholeBatch` | `TestZeroValueLedgerIsEmpty` |
| L16 | B26 | `ledger.go` | same id with different content treated as replay | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences` |
| L17 | B19 | `ledger.go` | unknown reservation accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences`, `TestRejectedBatchLeavesItsIdentitiesFree` |
| L18 | B48 | `ledger.go` | exclusions counted in the denominator | `TestB48ExclusionsAreNotApplicableNotZeroCost` | `TestB48ExclusionsAreNotApplicableNotZeroCost` |
| L19 | B48 | `ledger.go` | not_applicable in map order | `TestB48ExclusionsAreNotApplicableNotZeroCost` | `TestB48ExclusionsAreNotApplicableNotZeroCost`, `TestRandomInterleavingsKeepTheInvariants` |
| L20 | B48 | `ledger.go` | exclusion becomes an unknown | `TestB48ExclusionsAreNotApplicableNotZeroCost`, `TestReduceIsAPureFunctionOfTheOrderedEntries` | `TestB48ExclusionsAreNotApplicableNotZeroCost`, `TestReduceIsAPureFunctionOfTheOrderedEntries` |
| L21 | B17 | `ledger.go` | second reserve overwrites the reservation | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences`, `TestRandomInterleavingsKeepTheInvariants` |
| L22 | B22 | `ledger.go` | negative reservation accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences` |
| L23 | B24a | `ledger.go` | reserve overflow not checked | `TestB24aOverflowRejectsTheWholeBatch` | `TestB24aOverflowRejectsTheWholeBatch`, `TestOverflowBoundCountsASettledOverrun` |
| L24 | B17 | `ledger.go` | remaining not floored at zero | `TestB20FinalCostAboveReservationIsRecordedAsOverrun` | `TestB20FinalCostAboveReservationIsRecordedAsOverrun` |
| L25 | shape | `ledger.go` | fields of another kind accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences`, `TestInvalidUTF8IsRejectedInEveryStringField` |
| L26 | B19 | `ledger.go` | settle keeps the lifecycle unknown | `TestB19FinalEvidenceSettlesAndReleasesOnce`, `TestB20FinalCostAboveReservationIsRecordedAsOverrun` (+2) | `TestB19FinalEvidenceSettlesAndReleasesOnce`, `TestB20FinalCostAboveReservationIsRecordedAsOverrun` (+4) |
| L27 | B17 | `ledger.go` | projection R ignores actual | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB17PartialActualMovesReservedToActual` (+5) | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB17PartialActualMovesReservedToActual` (+12) |
| L28 | B48 | `ledger.go` | exclusion without mechanism accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences` |
| L29 | gap | `ledger.go` | arbitrary gap reason accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences`, `TestInvalidUTF8IsRejectedInEveryStringField` |
| L30 | B18 | `ledger.go` | arbitrary hold reason accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences`, `TestInvalidUTF8IsRejectedInEveryStringField` |
| L31 | B30 | `ledger.go` | unreserved actual overflow unchecked | **SURVIVED** | **SURVIVED** |
| L32 | gap | `ledger.go` | gap entry adds no unknown | `TestGapEntriesJoinTheUnknownSet` | `TestGapEntriesJoinTheUnknownSet` |
| U01 | B15 | `usage.go` | rollup counted like normal usage | `TestB15ProvenRollupContributesNothing` | `TestB15ProvenRollupContributesNothing` |
| U02 | B15 | `usage.go` | unproven overlap silently dropped (no ambiguity) | `TestB15UnprovenRollupIsAmbiguousNotActual` | `TestB15UnprovenRollupIsAmbiguousNotActual` |
| U02b | B15 | `usage.go` | unproven overlap counted like normal usage | `TestB15UnprovenRollupIsAmbiguousNotActual` | `TestB15UnprovenRollupIsAmbiguousNotActual` |
| U03 | B21 | `usage.go` | absent cost is a known zero | `TestB21MissingCostStaysUnknownBesideAKnownCost`, `TestB22CostPresenceStatesStayDistinct` (+4) | `TestB21MissingCostStaysUnknownBesideAKnownCost`, `TestB22CostPresenceStatesStayDistinct` (+4) |
| U04 | B22 | `usage.go` | zero without basis is a known zero | `TestB22CostPresenceStatesStayDistinct` | `TestB22CostPresenceStatesStayDistinct` |
| U05 | B22 | `usage.go` | unsupported currency counted as USD | `TestB22CostPresenceStatesStayDistinct` | `TestB22CostPresenceStatesStayDistinct` |
| U06 | B22 | `usage.go` | negative amounts accepted | `TestB22CostPresenceStatesStayDistinct`, `TestEntryIdentityConflictsAndUnknownReferences` | `TestB22CostPresenceStatesStayDistinct`, `TestEntryIdentityConflictsAndUnknownReferences` |
| U07 | B22 | `usage.go` | zero with basis is unknown | `TestB22CostPresenceStatesStayDistinct` | `TestB22CostPresenceStatesStayDistinct` |
| U08 | B22 | `usage.go` | estimate counted as actual | `TestB24ReplayKeepsFrozenEstimateAndActual`, `TestEstimatedCostNeverBecomesActual` | `TestB24ReplayKeepsFrozenEstimateAndActual`, `TestEstimatedCostNeverBecomesActual` (+2) |
| U09 | B25 | `usage.go` | cumulative total added whole | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB19FinalEvidenceSettlesAndReleasesOnce` (+11) | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB19FinalEvidenceSettlesAndReleasesOnce` (+14) |
| U10 | B26 | `usage.go` | late older value not treated as stale | `TestB26LateOlderValueIsNotOrderedByArrival`, `TestLateOlderValueAboveTheHeadIsQuarantined` | `TestB26LateOlderValueIsNotOrderedByArrival`, `TestLateOlderValueAboveTheHeadIsQuarantined` (+1) |
| U11 | B26 | `usage.go` | stale reported as accepted | `TestB26LateOlderValueIsNotOrderedByArrival` | `TestB26LateOlderValueIsNotOrderedByArrival` |
| U12 | B27 | `usage.go` | cost regression accepted (lowers A) | `TestB27UnexplainedRegressionIsQuarantined` | `TestB27UnexplainedRegressionIsQuarantined`, `TestRandomCumulativeScopesMatchANaiveModel` |
| U13 | B27 | `usage.go` | token regression accepted | `TestCumulativeTokenRegressionIsQuarantined` | `TestCumulativeTokenRegressionIsQuarantined` |
| U14 | B27 | `usage.go` | estimate regression accepted | **SURVIVED** | `TestEstimatedRunningTotalCannotGoDownUnexplained` |
| U15 | B28 | `usage.go` | unauthorised cumulative correction applied | `TestCorrectionWithoutAuthorityOrChainIsQuarantined` | `TestCorrectionWithoutAuthorityOrChainIsQuarantined` |
| U16 | B28 | `usage.go` | correction of a non-head entry applied | `TestCorrectionWithoutAuthorityOrChainIsQuarantined` | `TestCorrectionWithoutAuthorityOrChainIsQuarantined` |
| U17 | B28 | `usage.go` | authorised correction treated as ordinary event | `TestB28ExplicitCorrectionLowersNetAndKeepsOriginals`, `TestCorrectionCannotMakeAReservationNegative` (+1) | `TestB28ExplicitCorrectionLowersNetAndKeepsOriginals`, `TestCorrectionCannotMakeAReservationNegative` (+2) |
| U18 | B28/B29 | `usage.go` | unauthorised delta correction applied | **SURVIVED** | `TestDeltaCorrectionWithoutAuthorityTargetOrScopeIsQuarantined` |
| U19 | B28 | `usage.go` | delta entry superseded twice | `TestDeltaCorrectionReplacesItsOriginalOnce` | `TestDeltaCorrectionReplacesItsOriginalOnce` |
| U20 | B28 | `usage.go` | delta correction across scopes | **SURVIVED** | `TestDeltaCorrectionWithoutAuthorityTargetOrScopeIsQuarantined` |
| U21 | B29 | `usage.go` | mixed mode accepted | `TestB29DeltaCountsEachEventOnceAndModeIsFixed` | `TestB29DeltaCountsEachEventOnceAndModeIsFixed` |
| U22 | B28 | `usage.go` | delta correction does not remove the original | `TestDeltaCorrectionReplacesItsOriginalOnce` | `TestDeltaCorrectionReplacesItsOriginalOnce`, `TestDeltaCorrectionThroughAnotherReservationIsQuarantined` (+2) |
| U23 | B30 | `usage.go` | wrong baseline accepted | `TestBaselineThatIsNotTheScopeHeadIsAmbiguous` | `TestBaselineThatIsNotTheScopeHeadIsAmbiguous` |
| U24 | B30 | `usage.go` | new invocation without baseline accepted | `TestB30SessionAcrossInvocationsWithoutBaseline` | `TestB30SessionAcrossInvocationsWithoutBaseline` |
| U25 | B27 | `usage.go` | same sequence twice not a conflict | `TestB26LateOlderValueIsNotOrderedByArrival` | `TestB26LateOlderValueIsNotOrderedByArrival`, `TestRandomCumulativeScopesMatchANaiveModel` |
| U26 | B27 | `usage.go` | older-but-higher value silently stale | `TestLateOlderValueAboveTheHeadIsQuarantined` | `TestLateOlderValueAboveTheHeadIsQuarantined` |
| U27 | B22 | `usage.go` | reported total does not clear the estimate | **SURVIVED** | `TestReportedTotalReplacesTheEstimateAndALaterEstimateCountsTheExcess` |
| U28 | B21 | `usage.go` | reported total does not clear cost unknowns | `TestCumulativeCostCoversEarlierCostlessObservations` | `TestCumulativeCostCoversEarlierCostlessObservations`, `TestReportedTotalReplacesTheEstimateAndALaterEstimateCountsTheExcess` |
| U29 | B22 | `usage.go` | cumulative estimate adds no unknown | `TestEstimatedCostNeverBecomesActual` | `TestEstimatedCostNeverBecomesActual`, `TestEstimatedRunningTotalCannotGoDownUnexplained` (+1) |
| U30 | B22 | `usage.go` | estimate and actual double count | **SURVIVED** | `TestReportedTotalReplacesTheEstimateAndALaterEstimateCountsTheExcess` |
| U31 | B28 | `usage.go` | correction may drive a reservation negative | `TestCorrectionCannotMakeAReservationNegative` | `TestCorrectionCannotMakeAReservationNegative` |
| U34 | B22 | `usage.go` | delta estimate adds no unknown | `TestB24ReplayKeepsFrozenEstimateAndActual` | `TestB24ReplayKeepsFrozenEstimateAndActual` |
| U35 | B21 | `usage.go` | delta absent cost adds no unknown | `TestB21MissingCostStaysUnknownBesideAKnownCost`, `TestB22CostPresenceStatesStayDistinct` (+3) | `TestB21MissingCostStaysUnknownBesideAKnownCost`, `TestB22CostPresenceStatesStayDistinct` (+3) |
| U36 | B21 | `usage.go` | cumulative absent cost adds no unknown | `TestCumulativeCostCoversEarlierCostlessObservations` | `TestCumulativeCostCoversEarlierCostlessObservations` |
| U37 | shape | `usage.go` | usage without scope/invocation accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences` |
| U38 | B25 | `usage.go` | cumulative without sequence accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences` |
| U39 | B24 | `usage.go` | reported cost may carry a price snapshot | **SURVIVED** | `TestUsageShapesThatAreRejected` |
| U40 | B26 | `usage.go` | sequences never recorded | `TestB26LateOlderValueIsNotOrderedByArrival` | `TestB26LateOlderValueIsNotOrderedByArrival`, `TestRandomCumulativeScopesMatchANaiveModel` |
| U41 | B30 | `usage.go` | head invocation never updated | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB19FinalEvidenceSettlesAndReleasesOnce` (+11) | `TestAllowanceIsCountedOnceAndHoldFreesNothing`, `TestB19FinalEvidenceSettlesAndReleasesOnce` (+14) |
| U42 | B29 | `usage.go` | delta with baseline accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences` |
| U43 | B15 | `usage.go` | overlap with correction/baseline accepted | **SURVIVED** | `TestUsageShapesThatAreRejected` |
| U44 | B28 | `usage.go` | self-correction / authorised without target accepted | **SURVIVED** | `TestUsageShapesThatAreRejected` |
| U45 | B22 | `usage.go` | zero basis on a non-zero cost accepted | `TestEntryIdentityConflictsAndUnknownReferences` | `TestEntryIdentityConflictsAndUnknownReferences` |
| U46 | B28 | `usage.go` | per-reservation attribution not tracked | `TestB28ExplicitCorrectionLowersNetAndKeepsOriginals` | `TestB28ExplicitCorrectionLowersNetAndKeepsOriginals`, `TestCumulativeCorrectionThroughAnotherReservationIsQuarantined` (+2) |
| U47 | B28 | `usage.go` | delta correction keeps the original's unknown | `TestDeltaCorrectionResolvesTheOriginalsUnknown` | `TestDeltaCorrectionResolvesTheOriginalsUnknown` |
| U48 | B28 | `usage.go` | superseded flag never set | `TestDeltaCorrectionReplacesItsOriginalOnce` | `TestDeltaCorrectionReplacesItsOriginalOnce`, `TestRandomDeltaLedgersMatchANaiveModel` (+1) |
| D01 | B24a | `decimal.go` | leading zero accepted | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` |
| D02 | B24a | `decimal.go` | JSON number accepted | `TestB24aTicksJSONIsADecimalStringNeverANumber` | `TestB24aTicksJSONIsADecimalStringNeverANumber` |
| D03 | B24a | `decimal.go` | addChecked wraps | `TestB24aOverflowRejectsTheWholeBatch` | `TestB24aOverflowRejectsTheWholeBatch`, `TestOverflowBoundCountsASettledOverrun` |
| D04 | B24a | `decimal.go` | MaxInt64 itself rejected | `TestB24aDecimalCodecAcceptsCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` | `TestB24aDecimalCodecAcceptsCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` |
| D05 | B24a | `decimal.go` | 2^63 accepted and wraps | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` |
| D06 | B24a | `decimal.go` | long digit strings wrap | `TestB24aDecimalCodecRejectsNonCanonicalStrings` | `TestB24aDecimalCodecRejectsNonCanonicalStrings` |
| D07 | B24a | `decimal.go` | fraction/exponent accepted | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` |
| D08 | B24a | `decimal.go` | negative amounts formatted | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` (+1) | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` (+1) |
| D09 | B23 | `decimal.go` | mulChecked wraps | `TestUSDRateIsAnIntegerFraction` | `TestUSDRateIsAnIntegerFraction` |
| D10 | B24a | `decimal.go` | empty string is zero | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` | `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` |
| D11 | B24a | `decimal.go` | 19-digit values rejected | `TestB24aDecimalCodecAcceptsCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` | `TestB24aDecimalCodecAcceptsCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber` |
| P01 | B23 | `price.go` | half rounds down | `TestEstimateHalfUpTie`, `TestEstimateSumsDimensionsBeforeRoundingOnce` | `TestEstimateHalfUpTie`, `TestEstimateSumsDimensionsBeforeRoundingOnce` |
| P02 | B23 | `price.go` | reservation rounds down | `TestB23EstimateFractionRoundsUpForReservation`, `TestEstimateHalfUpTie` (+2) | `TestB23EstimateFractionRoundsUpForReservation`, `TestEstimateHalfUpTie` (+2) |
| P03 | B23 | `price.go` | each dimension floored before the sum | `TestB23EstimateFractionRoundsUpForReservation`, `TestEstimateHalfUpTie` (+2) | `TestB23EstimateFractionRoundsUpForReservation`, `TestEstimateHalfUpTie` (+2) |
| P04 | B24 | `price.go` | claimed version never flagged | `TestB24CallerClaimedPriceVersionNeverOverridesSnapshot` | `TestB24CallerClaimedPriceVersionNeverOverridesSnapshot` |
| P05 | B24 | `price.go` | claimed version recorded as the price version | `TestB24CallerClaimedPriceVersionNeverOverridesSnapshot` | `TestB24CallerClaimedPriceVersionNeverOverridesSnapshot` |
| P06 | B23 | `price.go` | ceil-v1 rounds half up | `TestB23EstimateFractionRoundsUpForReservation` | `TestB23EstimateFractionRoundsUpForReservation` |
| P07 | B23/B24a | `price.go` | estimate overflow wraps | `TestEstimateOverflowIsAnErrorNotWraparound` | `TestEstimateOverflowIsAnErrorNotWraparound` |
| P08 | B22 | `price.go` | non-USD snapshot priced | `TestPriceSnapshotValidation` | `TestPriceSnapshotRejectsInvalidUTF8`, `TestPriceSnapshotValidation` |
| P09 | B24 | `price.go` | digest without domain prefix | `TestB24CallerClaimedPriceVersionNeverOverridesSnapshot`, `TestEntryStoredFormRoundTripsAndDigests` (+1) | `TestB24CallerClaimedPriceVersionNeverOverridesSnapshot`, `TestEntryStoredFormRoundTripsAndDigests` (+1) |
| P11 | B22 | `price.go` | negative token count priced | `TestPriceSnapshotValidation` | `TestPriceSnapshotValidation` |
| P12 | B23 | `price.go` | reservation follows the display rounding | `TestB23EstimateFractionRoundsUpForReservation` | `TestB23EstimateFractionRoundsUpForReservation` |
| P13 | B23 | `price.go` | half-up always floors | `TestEstimateHalfUpTie`, `TestEstimateSumsDimensionsBeforeRoundingOnce` | `TestEstimateHalfUpTie`, `TestEstimateSumsDimensionsBeforeRoundingOnce` |
| P14 | B23 | `price.go` | invalid rate inputs accepted | `TestUSDRateIsAnIntegerFraction` | `TestUSDRateIsAnIntegerFraction` |
| P15 | B23 | `price.go` | zero denominator accepted | `TestPriceSnapshotValidation` | `TestPriceSnapshotValidation` |
| P16 | B24 | `price.go` | unknown rounding version accepted | `TestPriceSnapshotValidation` | `TestPriceSnapshotRejectsInvalidUTF8`, `TestPriceSnapshotValidation` |
| M01 | B23 | `model.go` | tick scale off by ten | `TestB23EstimateExactIntegerNeedsNoRounding`, `TestB23EstimateFractionRoundsUpForReservation` (+6) | `TestB23EstimateExactIntegerNeedsNoRounding`, `TestB23EstimateFractionRoundsUpForReservation` (+6) |
| U49 | B28 | `usage.go` | delta correction moves spend to another reservation | n/a (mutates reviewer fix) | `TestDeltaCorrectionThroughAnotherReservationIsQuarantined` |
| U50 | B28 | `usage.go` | cumulative correction charged to another reservation | n/a (mutates reviewer fix) | `TestCumulativeCorrectionThroughAnotherReservationIsQuarantined` |
| L33 | stored | `ledger.go` | invalid UTF-8 identifiers accepted | n/a (mutates reviewer fix) | `TestInvalidUTF8IsRejectedInEveryStringField` |
| L34 | zero | `ledger.go` | zero Ledger panics | n/a (mutates reviewer fix) | `TestZeroValueLedgerIsEmpty` |
| P17 | stored | `price.go` | invalid UTF-8 in a price snapshot accepted | n/a (mutates reviewer fix) | `TestPriceSnapshotRejectsInvalidUTF8` |

## Examined and found correct

- **Oracles.** B15 A=115 with the rollup contributing 0 and the unproven one in the unknown set; B17 30/70/0; B18 30/0/70 with `rem` null; B19 80/0/0, release 20, replay unchanged; B20 A=120, overrun 20; B21; B22 five distinct states; B23 30,000,030,000 and 3333/3334; B24; B24a; B25 150; B26; B27; B28 130; B29 100; B30 200 with baseline, 80 plus one ambiguity without; B48.
- **Invariants (§4.2).** Three seeded random tests now in the package:
  - 400 ledgers × 60 mixed valid and invalid entries, checking after every step that A+R+H equals the internal total, nothing is negative, the per-scope and per-reservation sums agree, a hold changes no known figure and frees nothing, only a settle releases, each receipt explains the change, a rejected entry leaves the ledger deeply equal, a replay of any earlier entry is a no-op, and `Reduce` of the history, of the history with a prefix repeated, and one batch versus entry-by-entry all reach the same state and the same projection JSON.
  - 300 delta ledgers against an independent model that recomputes A/R/H/overrun from the live amounts.
  - 300 cumulative scopes against a model that keeps only "highest sequence" and "current total".
- **Determinism.** No clock, randomness or package state in non-test code. Map iteration only feeds commutative sums or a sorted list; the unknown set keeps insertion order; the error chosen by a refused settle is the first unknown in insertion order.
- **Codec.** Rejected: `+1`, `-1`, `01`, `00`, empty, leading/trailing whitespace, full-width and Arabic-Indic digits, `1e3`, `1.5`, `9223372036854775808`, 20+ digit strings, JSON number `5`, `9007199254740993`, `null`, escaped digit `"1"`. Accepted: `"0"`, `"9223372036854775807"`. Decoding into `any` keeps strings; there is no float64 path.
- **Atomic batch.** A batch failing at its last entry leaves entries, receipts, reservations, scopes, sequences, exclusions and the unknown set unchanged, and its IDs can be reused with different content.
- **Overflow.** Checked on every path that changes A, R, H or E; the estimate refuses a ceiling outside int64.

## Changes I made

All inside `server/internal/budget/` in the worktree. Test count 54 → 69.

| File | Change |
| --- | --- |
| `usage.go:215-219` | Cumulative correction under a different reservation than the head entry is quarantined (W1) |
| `usage.go:282-287` | Delta correction under a different reservation than its original is quarantined (W2) |
| `ledger.go:180-194` | Entries with invalid UTF-8 in any free-text field are `ErrInvalidEntry` (W3) |
| `price.go:93-103` | Price snapshots with invalid UTF-8 are `ErrInvalidPrice` (W3) |
| `ledger.go:46,74-89` | The zero `Ledger` is usable; before, `var l Ledger; l.Apply(...)` panicked with "assignment to entry in nil map" |
| `edge_cases_test.go` (new, 12 tests) | Regression tests for the fixes and for the surviving mutants |
| `invariants_test.go` (new, 3 tests) | The random invariant and reference-model tests |

Red before the fixes (`/tmp/f3chk/red.txt`): the two cross-reservation tests, the two UTF-8 tests and the zero-value test failed; the other seven new tests passed on the delivered code. The original four test files, `decimal.go` and `model.go` are byte-identical to what was delivered.

## Not verified

- Anything involving a database, a handler or a real adapter; the DB half of B24.
- The provider-native USD decimal → tick rule and token totals in the projection, which are not in the package.
- Behaviour at scale beyond the 2,000-event probe.
- Whether real adapters ever send a correction under a different attempt than the original; W1/W2 quarantine that case instead of guessing.
