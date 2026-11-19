# Blocking ETA UTC range — independent R04 check, 2026-11-18

**PASS for BK-R03-01 on the sealed R04 source.** No unintended wire, data or authority change found. This accepts the isolated fix, not a DB-backed package run or integration.

## Findings (fixed)

- **New requests:** `parseInformationEstimate` in `collaboration_information_state.go` keeps the four-digit local year (1–9999), explicit `Z` or numeric offset, and zero to six fractional digits. It now checks the UTC year too. The original overflow inputs `0001-01-01T00:00:00+00:01` and `9999-12-31T23:59:59-00:01` normalize to `0000-12-31T23:59:00Z` and `10000-01-01T00:00:59Z`; both now return invalid, not clamped times. Local year zero is still rejected even if it normalizes to year one. The original precision seed `2026-11-19T10:30:00.123456+08:00` still yields `2026-11-19T02:30:00.123456Z`.
- **Retained rows:** With valid reason, recipient and condition, `Valid:true` and `pgtype.Finite` make `0001-01-01T00:00:00.000000Z` a real `complete` ETA despite Go’s `IsZero()`; the upper endpoint `9999-12-31T23:59:59.999999Z` is also complete. UTC decides even when the stored location shows another year. The retained overflow tests use **different instants from the request seeds**: `0000-12-31T23:59:59.999999Z` and `10000-01-01T00:00:00.000000Z` (one microsecond outside), in UTC and offset locations. These, a partial pair, or sub-microsecond precision yield `unavailable` / `information_metadata_invalid` with neither condition nor ETA. Both fields absent still yields `legacy`.
- **Infinity:** pgx v5.9.2 decodes text and binary infinities as valid **nonfinite** modifiers, normally with zero `Time`; SQL NULL is invalid. R04 checks the modifier, so both signs are unavailable even with a populated `Time`. The four retained infinity cases exercised this distinction. Literal `infinity` and `-infinity` request strings cannot match the timestamp grammar; that is a source check, not a named parser test or DB round-trip.
- **Unchanged contract:** The v1 block shape and six-digit UTC formatter are unchanged. The workflow still hashes the original ETA string (legacy omitted fields stay omitted), recovers an authorized original receipt before fresh admission, and requires a new ETA to be strictly after the final DB clock. The finite-min test proves representation, not permission to write a past ETA. The existing managed-chain check consumes the corrected block state; no actor, capability, SQL, migration, stored history or backfill changed.

## Verification

Evidence: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/verification/block-utc-range-r04`. Independently rehashed both 5,077-file trees and compared paths, modes and the actual diff to `delta-r03-r04.patch`: 5,075 identical; no additions, removals or mode changes.

| Source binding | SHA-256 |
| --- | --- |
| R03 manifest | `538e4f92de2707285d285a98238260edf2acc3930512056b2f560ba01db6ad36` |
| R04 manifest | `4ce478ca64019a8e1169a0c199c53c4638e43d4b23d5a876b2150b3c6d2fbb68` |
| R04 `server/internal/service/collaboration_information_state.go` | `c0a833283691d75c6913cb83db090f38172c5f7e3881771cc962e4d4dff756fb` |
| R04 `server/internal/service/collaboration_information_metadata_test.go` | `bbf589ca15dff29665ab0dd3d28dce55fbf91a4c197d60d565a2c2e05de8588f` |

Reconciled the test source with `red/` and `green/` raw `stdout.log` JSON events, commands and exits, not just `inventory.json`. Same test bytes, argv and environment; RED used R03 production, GREEN used R04. Each full name had one run and one terminal event in each run. **Bold subtests failed RED**, as did their two parents.

| Top-level name | Exact subtest suffixes |
| --- | --- |
| `TestCollaborationInformationMetadataPresenceAndPrecision` | `old_omitted`, `managed_missing`, `missing_estimate`, `missing_condition`, `managed_complete` |
| `TestCollaborationInformationTextUsesDecodedUTF8Bytes` | None |
| `TestCollaborationInformationEstimateUTCWireRange` | `utc_min`, `utc_max`, `offset_to_min`, `offset_to_max`, **`offset_below_min`**, **`offset_above_max`**, `local_year_zero_to_min` |
| `TestCollaborationInformationBlockRetainedUTCRange` | **`utc_min`**, `utc_max`, **`offset_to_min`**, `offset_to_max`, **`below_min`**, **`above_max`**, **`offset_below_min`**, **`offset_above_max`**, `unset_eta`, `positive_infinity`, `negative_infinity`, **`positive_infinity_with_time`**, **`negative_infinity_with_time`**, `sub_microsecond`, `both_unset` |

RED exited 1 (19 pass, 12 fail); GREEN exited 0 (**31/31**: four parents, 27 children; no skips, duplicates or missing names). Go 1.27.0 darwin/arm64 compiled and type-checked 222 selected service files plus the single metadata test file as `command-line-arguments`; network denied, no package `TestMain` or DB fixtures. `vet/` used the same file list, exit 0, empty stdout/stderr, no test names. These are audited runs, not fresh executions. Independent `gofmt -l` on both files returned no paths.

## Findings (not fixed)

No remaining BK-R03-01 finding. BK-R03-02 (raw query validation) is outside R04. The parent owns integration and fresh normal/race package runs on separate owned databases, including actual pgx persistence, final-clock/replay behavior and fixture cleanup. No DB, API, Web/Desktop or full S3 acceptance is inferred here.
