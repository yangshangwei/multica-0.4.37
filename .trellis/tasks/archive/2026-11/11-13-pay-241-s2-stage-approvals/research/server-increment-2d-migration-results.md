# S2 2d migration verification

2026-11-14. Root verified and applied the frozen submission-binding migrations **861–864**. This report covers DDL, schema structure and downgrade retention; it does not claim the 2d writer or policy-activation acceptance has passed.

## Frozen input

The 4,939-file independently checked 2c follow-up base plus the 12 explicit DDL/sqlc overlays reconstructs **4,949 files**, with zero undeclared files or hash mismatches. Root built a new migrator from those bytes. No peer migrations 844–860 were copied or run.

The new table has no foreign keys. Its two indexes each use their own single-statement concurrent-index migration. The immutable-update trigger and exact readiness checks are retained. The production activation flag remains false in this intermediate source.

## Database results

The four service/handler normal/race databases, two router databases, dedicated harness database and dedicated migration database all passed:

- Actual database identity checked through the private launcher.
- Zero other connections before and after each migration.
- Exact ledger transition from 831 entries through843 to **835 entries through864**, excluding the unrelated peer versions.
- Both new indexes valid, ready and live; unique submission identity and root/lifecycle lookup definitions verified.
- Structural submission readiness true and zero new foreign keys.

## Dedicated migration controls

All seven controls passed in `multica_pay241_20261114_s2_2c_migrate_r01`:

1. Empty submission history: full861–864 downgrade removes the table/indexes and returns the ledger to843.
2. Upgrade restores the exact prior schema/ledger.
3. Removing only864 leaves structural readiness true while the864 ledger entry is absent.
4. Reapplying864 restores the exact prior state.
5. A retained synthetic submission binding rejects an UPDATE with SQLSTATE23514 and remains byte-equivalent.
6. Full downgrade refuses retained submission history before removing indexes or ledger entries.
7. Direct861 table downgrade also refuses retained history without changing the table, row, indexes or ledger.

The third control is deliberately **not** proof of service admission. Runtime2d must require and lock the exact864 readiness ledger entry in addition to structural checks. The server owner confirmed this requirement and is adding the actual admission regression. The migration itself cannot require its own ledger entry during `up`, before the runner records it.

The dedicated migration database retains one explicitly synthetic submission binding and the earlier synthetic2c intent. They do not represent actual approvals, delivery, deployment or provider execution. The old intent's row digest remained unchanged throughout. Final connection count is0. Other package and harness databases were not populated by these retention probes.

## Remaining

All real writers, final transaction/time/authority checks, cleanup registration, half-downgrade runtime admission and policy activation require the completed2d source and its independent tests. The eight existing F2 cleanup-registration failures remain separate and must not be hidden by changing assertions.

Exact source, migrator, DDL, configuration hashes, commands, logs, exit codes and schema snapshots are linked from the companion JSON. Private configuration contents and credentials are not included.
