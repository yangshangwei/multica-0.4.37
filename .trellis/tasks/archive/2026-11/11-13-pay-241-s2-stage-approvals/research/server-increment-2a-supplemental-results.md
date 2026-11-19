# S2 server 2a — coordinator supplemental verification

Owner: `/root`. Date: 2026-11-14. This extends the original 2a report without changing its historical results. S2/A01–A12 remain open.

## Verified inputs and isolation

The original 45 source/contract hashes, 34 captured logs/exit records and four migration evidence hashes matched. The two additional static commands were rerun successfully because those report entries had no separate exit file. An initial audit stopped on that missing key; the corrected audit records the limitation.

The coordinator froze 4,864 server files outside the checkout, restoring the 45 applicable 2a bindings from the earlier increment snapshot. Every frozen file still matched after verification. This is the working HEAD plus recorded source bytes, not a claim that S2 was committed. The live server owner continued 2b separately.

Two fresh task databases were allocated and migrated through 825:

- `multica_pay241_20261114_s2_ghsnapshot_r01`: only `internal/integrations/ghsnapshot`.
- `multica_pay241_20261114_s2_migrate_r01`: only migration verification.

Each has its own external HOME/TMP, verified outside Git. The initial cluster observation was 30 connections against a limit of 100; no connection or timeout limits were raised. Final read-only audits found zero other connections to either retained database.

## Results

| Verification | Result | Limit |
| --- | --- | --- |
| Full frozen `ghsnapshot` package, `-race -count=1` | 24 top-level / 42 named PASS, zero FAIL/SKIP | Local producer/database behavior; no actual GitHub call |
| Existing stage migration runner upgrade/refusal/rollback/re-upgrade | PASS | Covers existing 805–820 stage fixture |
| Concurrent down-index registration invariant | PASS | Static registration invariant |
| Concurrent up-index registration invariant | FAIL | 16 S2 entries and eight separate F2 entries missing |
| Actual migration CLI with unmodified 821–825 files: empty down, re-upgrade | Both exit 0; expected ledger, indexes, table and column restored | Curated file directory limits this proof to the increment |
| Actual migration CLI: retained synthetic code-candidate history | Exit 1 at 825, before the first down record; all five ledger rows and three indexes preserved | Individual decision-only/revocation-only branches remain canonical test work |

The code-candidate fixture is synthetic and retained. It proves schema retention behavior, not that a real candidate was created through a business endpoint. No history was deleted to obtain the positive result: the empty rollback ran before adding the retained candidate.

## Routed findings

1. S2 migrations 806–818 and 822–824 omit the migration runner's concurrent-index cleanup registrations. An interrupted build can therefore block the ordinary retry. The original failing log is preserved and the same server owner will add the 16 S2 registrations and regression coverage. The eight F2 `report_snapshot*` entries are unrelated and are not silently included in this fix.
2. Independent review demonstrated that the R1 child digest can distinguish a private low-entropy description offline when the title is already known. The coordinator selected a narrow contract correction: explicit `content_digest:null` for source-denied children, and no fresh scope selection without a readable digest. Server and client owners were notified; the server owner must freeze the dated clarification before client edits. This report does not claim the fix has landed.

## Evidence

`server-increment-2a-supplemental-results.json` binds named test terminals, raw log hashes, the complete frozen-source manifest, database allocation/migration records, actual CLI roundtrip evidence and final database audits. Raw files are under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/`.

No public policy activation, API/daemon/browser launch, production Web/Desktop verification, real account call, payment, deployment or full S2 acceptance occurred in this supplement.
