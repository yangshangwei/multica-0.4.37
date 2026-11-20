# F3 step 2 (pure metering): implementation report

Date: 2026-11-16. Worktree `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`, branch `codex/f3-budget-root-20261116`. Nothing is committed, staged or pushed; `git status --short` shows only `?? server/internal/budget/`.

## Result

The pure package is written and every `pure` exit row passes: B15, B17, B18, B19, B20, B21, B22, B23, B24 (pure half), B24a, B25, B26, B27, B28, B29, B30. B48 fell out of the coverage model and is included. 54 tests pass, 0 fail, 0 skip, with and without `-race`. No matrix expected value looked wrong; five rows are underspecified and are listed below with the reading I took.

## Package and files

`server/internal/budget/` (package `budget`, all files new):

| File | Lines | Content |
| --- | --- | --- |
| `model.go` | 243 | scale constant, sentinel errors, entry / usage / receipt / projection types |
| `decimal.go` | 99 | decimal-string codec, JSON form of `Ticks` and `Count`, checked add / multiply |
| `price.go` | 169 | `Rate`, `PriceSnapshot`, digest, exact estimation |
| `ledger.go` | 406 | `Ledger`, entry validation, reserve / hold / settle / gap / exclusion, projection |
| `usage.go` | 325 | cost presence states, cumulative and delta event rules |
| `decimal_test.go`, `price_test.go`, `ledger_test.go`, `usage_test.go` | 132 / 269 / 547 / 452 | tests |

It imports the standard library and `server/pkg/agent` only. There is no database, I/O, network or clock.

## Public API

- Codec: `ParseDecimal(string) (int64, error)`, `FormatDecimal(int64) (string, error)`. `Ticks` and `Count` marshal as decimal strings and refuse JSON numbers, null, escapes, signs, fractions, exponents, leading zeros, whitespace and overflow.
- Price: `USDRate(usdNumerator, usdDenominator, perTokens)`; `PriceSnapshot.Validate()`, `.Digest()`, `.Estimate(tokens, claimedPriceVersion) (Estimate, error)`.
- Ledger: `NewLedger()`, `(*Ledger).Apply(entries...) ([]Receipt, error)` (atomic batch), `(*Ledger).Projection()`, `Reduce([]Entry) (Projection, []Receipt, error)`, `Entry.Digest()`.
- Projection: `.Committed()` (A + R + H), `.TotalKnown()`, `.Remaining(limit) *Ticks`, `.UnknownCounts()`.
- Entry kinds: `reserve`, `usage`, `hold`, `settle`, `gap`, `exclusion`.
- Errors: `ErrInvalidDecimal`, `ErrOverflow`, `ErrNegativeAmount`, `ErrInvalidPrice`, `ErrUnsupportedCurrency`, `ErrInvalidEntry`, `ErrConflict`, `ErrModeMismatch`, `ErrUnknownReservation`, `ErrReleaseProofIncomplete`.

For the later persistence step: `Entry` is a comparable value with a fixed JSON stored form, and `Reduce` is a function of the ordered entry list only. Persist entries in order and replay them through `Reduce`, or `Apply` new ones onto a replayed `Ledger`.

## Scale constant

I chose `agent.CostUSDTicksPerUSD` from `server/pkg/agent`; `budget.TicksPerUSD` is defined as that constant, not as a second literal.

- `server/pkg/agent` owns the unit (adapters report in it) and is light: the whole dependency closure of `internal/budget` is `pkg/agent`, `pkg/protocol`, `pkg/redact`, `pkg/taskfailure`, `google/uuid`, `go-toml/v2` and `x/sys/unix`.
- `server/internal/metrics` re-exports the same value but pulls in Prometheus, pgx and chi: `go list -deps ... | grep -c '\.'` prints 199 for it against 13 for `internal/budget`.
- The cost: the pure package now has a compile-time dependency on the agent package. If that is unwanted, the alternative is moving the constant to a leaf package, which touches code outside this step.

## Matrix rows

| Row | Test | Result |
| --- | --- | --- |
| B15 | `TestB15ProvenRollupContributesNothing`, `TestB15UnprovenRollupIsAmbiguousNotActual` | pass |
| B17 | `TestB17PartialActualMovesReservedToActual` | pass |
| B18 | `TestB18LostContactTurnsReservedIntoUnknownHold` | pass |
| B19 | `TestB19FinalEvidenceSettlesAndReleasesOnce` | pass |
| B20 | `TestB20FinalCostAboveReservationIsRecordedAsOverrun` | pass |
| B21 | `TestB21MissingCostStaysUnknownBesideAKnownCost` | pass |
| B22 | `TestB22CostPresenceStatesStayDistinct` | pass |
| B23 | `TestB23EstimateExactIntegerNeedsNoRounding`, `TestB23EstimateFractionRoundsUpForReservation` | pass |
| B24 (pure half) | `TestB24CallerClaimedPriceVersionNeverOverridesSnapshot`, `TestB24ReplayKeepsFrozenEstimateAndActual` | pass |
| B24a | `TestB24aDecimalCodecAcceptsCanonicalStrings`, `TestB24aDecimalCodecRejectsNonCanonicalStrings`, `TestB24aTicksJSONIsADecimalStringNeverANumber`, `TestB24aOverflowRejectsTheWholeBatch` | pass |
| B25 | `TestB25CumulativeAppendsOnlyTheDifference` | pass |
| B26 | `TestB26DuplicateAndLateOlderValueChangeNothing`, `TestB26LateOlderValueIsNotOrderedByArrival` | pass |
| B27 | `TestB27UnexplainedRegressionIsQuarantined` | pass |
| B28 | `TestB28ExplicitCorrectionLowersNetAndKeepsOriginals` | pass |
| B29 | `TestB29DeltaCountsEachEventOnceAndModeIsFixed` | pass |
| B30 | `TestB30SessionAcrossInvocationsWithBaseline`, `TestB30SessionAcrossInvocationsWithoutBaseline` | pass |
| B48 | `TestB48ExclusionsAreNotApplicableNotZeroCost` | pass |

The other 28 tests cover contract section 4.2 invariants and edges the matrix does not number (settle needs every proof, hold frees nothing, one ending per reservation, late usage after settlement, entry identity conflicts, token regression, delta corrections, estimates never becoming actual, digests).

Expected values are literals computed by hand or outside Go (the two SHA-256 goldens were computed with Python). No test derives an expected value from the code under test.

## Commands and exit codes

Run from `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server`; output is in `/tmp/f3-step2-*.txt`.

| Command | Exit | Output |
| --- | --- | --- |
| `gofmt -l internal/budget` | 0 | empty |
| `go vet ./internal/budget/...` | 0 | empty |
| `go test ./internal/budget/... -count=1 -v` | 0 | PASS 54, FAIL 0, SKIP 0 |
| `go test -race ./internal/budget/... -count=1` | 0 | `ok ... 1.380s` |
| `go build ./...` | 0 | empty |
| `grep -rn 'float' internal/budget` | 1 | no hits (also none case-insensitively) |

`math/big` `Int` and `Rat` carry the estimation; no floating-point type or conversion appears in code, tests or comments.

## Red output kept

Run against signature-only stubs before any implementation: exit 1, 53 FAIL, 1 PASS (the scale constant), 0 SKIP. Full output: `/tmp/f3-step2-red-keep.txt`.

- Codec: `decimal_test.go:65: ParseDecimal("1e3") = 0, <nil>; want 0, budget: not a canonical non-negative decimal string`
- Price: `price_test.go:86: half-up estimate = {Ticks:0 ReserveTicks:0 Exact:false ...}` (`TestB23EstimateFractionRoundsUpForReservation`)
- Reducer: `ledger_test.go:117: Apply returned 0 receipts for 3 entries` (`TestB18LostContactTurnsReservedIntoUnknownHold`)
- Events: `usage_test.go:188: Apply returned 0 receipts for 2 entries` (`TestB27UnexplainedRegressionIsQuarantined`)

The reducer and event reds stop at the `apply` helper, before the numeric assertions, because the stub returned no receipts. To show those assertions bite, I ran ten single-line mutations of the finished code through `go test -overlay`; each one failed the matching row (for example, dropping the hold transition fails B18 and B19, accepting a regression fails B27, removing the baseline rule fails B30, counting a rollup fails B15, dropping the ceiling fails B23).

Two test edits came after the red run, together with the implementation: four error-table cases gained an `InvocationID` so they fail for the reason their name states, and one case (`usage without invocation`) was added.

## Underspecified rows and the reading taken

None of these changes a matrix number. Each is a decision a later step may want to overrule.

1. **B17 / B18, `remaining`.** The matrix gives `rem=null` once contact is lost but says nothing for B17. `Remaining(limit)` is `limit - (A + R + H)`, floored at 0, while the unknown set is empty (B17 with limit 250 gives 150), and absent whenever any unknown item exists.
2. **B23, displayed estimate.** The matrix fixes the reservation (3334) but not the displayed value of 3333.33... I defined two rounding versions: `half-up-v1` shows 3333, `ceil-v1` shows 3334. Both replay identically from the stored snapshot.
3. **B24, "ignored or rejected".** A caller-claimed price version is ignored and flagged (`ClaimedPriceVersionIgnored`), not an error. The handler can turn the flag into a rejection.
4. **B22 negative amount and B29 mixed mode, "rejected".** Both are errors that reject the whole batch and leave no record, so the entry ID stays free. They are not quarantined.
5. **B26, late `v1`.** Both readings are covered: the same identity resent returns the original receipt; a first-time arrival below the head is kept as `stale` and subtracts nothing.

## Behaviour beyond the matrix that needs a decision

- **Unknown items from quarantine and gaps are permanent.** No entry kind resolves them. `settle` returns `ErrReleaseProofIncomplete` while the reservation has any non-lifecycle unknown, so such a reservation keeps its allowance for good. This follows contract section 6 ("otherwise keep the hold") but means a single quarantined event pins a reservation until a resolution path exists.
- **Estimates do not draw down a reservation** and each adds an `estimate_only` unknown, so `remaining` is absent whenever any cost is estimate-only. An estimated cost entry is not required to carry its price version and digest.
- **A and E never overlap inside a cumulative scope**: a reported running total clears the scope's estimate and its cost-quality unknowns; a later estimate counts only the part above reported actual.
- **A cumulative correction** must supersede the scope's current head and be server-authorised. The reduction comes off the correcting entry's reservation and is quarantined (`correction_unresolved`) if that would go negative.
- **Baseline proof** (B30) is modelled as `BaselineEventID` equal to the scope's current head; anything else across invocations is `scope_ambiguity`.
- **A late older reported value when the head has no reported total** is kept as `stale` and not counted.
- `InvocationID` is required on every usage entry. `Apply` copies the ledger state on each call to make the batch atomic, which is linear in ledger size.

## Left out

- Converting a provider's native USD decimal string to ticks (contract section 4.1 asks for a versioned rule; no pure row covers it).
- Token totals and a per-invocation breakdown in the projection.
- Limit and policy handling, `would_refuse`, and any persistence. The database half of B24 belongs to a later step.
