# S07 local backup and restoration evidence

The local database restoration rehearsal passed on 2026-10-02 at 07:10 CST.
This verifies preservation of synthetic administration and execution data. It
does not establish a production recovery objective or approve a deployment.

## Scope and isolation

- Source schema: implementation baseline `1c56f6cd6c16f11327aee2f3f000e6b19e401d0b`.
  The later rollback corrections change down-migration guards, not this schema.
- PostgreSQL: task-owned native arm64 container, local pinned pgvector image
  `sha256:215ae4d47393a980e9ef7ff7760f7dad53ec3e84bceb9fbd45cef34e50b5ba0f`.
- The capacity database had zero other connections before cloning. A new
  rehearsal source was cloned from it; non-default security/control fixtures
  were added only to that clone. Restoration used another new, empty database.
- No database was dropped, overwritten, or forcibly disconnected. The original
  capacity fixture and unrelated development databases were preserved.
- All accounts, passwords, tokens, installations and executions were synthetic.
  The credential-bearing archive remains inside the private ignored run folder.

## Procedure and result

The harness used `pg_dump` 18.6 with custom format and `--no-owner --no-acl`,
then `pg_restore --exit-on-error --no-owner --no-acl` into the empty target.
The archive is 73,544,554 bytes; SHA-256:
`1daf8e3553aae422d4f102df18a48bacbbb2f65e6af8d3ee14da07e3e35b68e7`.

All twenty table fingerprints matched after restoration. Each fingerprint
includes the row count and an order-independent digest of every full row,
including credential hashes, timestamps, versions, references and JSON data.
The comparison covered 1,000,300 tasks, 750,000 usage rows, 3,000 runtimes,
1,000 installations, 1,000 bindings, 1,011 password credentials, 11 role
bindings, pending operations, audit, alerts, and migration bookkeeping.

| Required retained state | Restored evidence |
| --- | --- |
| Administrator roles | One super administrator and ten observers; exact role-table fingerprint |
| Credential revocation | Session version 7 and required password change retained |
| Installation identity and epochs | Revoked binding at epoch 9/auth version 7, active binding at epoch 8, and all key/identity rows retained |
| Admission policy | Stopped installation with admission version 5 retained |
| Pending operation | Pending cancellation, reconciliation state, idempotency key, binding and execution references retained |
| Audit linkage | Pending operation's audit event and snapshot content retained exactly |
| Execution fences | State version, immutable dispatch timestamp, claim generation 3 and admission version 1 retained |
| Credential/binding consistency | 999 synthetic bound tokens still match active binding and credential versions; the revoked fixture remains inconsistent and therefore ineligible |
| Database objects | All 427 public indexes valid/ready; all 10 application triggers enabled |

The dump took 5.1 seconds and restoration took 20.6 seconds. Full fingerprint
validation took a further 25.6 seconds. These are local rehearsal timings with
synthetic data, not production RTO/RPO commitments or capacity measurements.

## Reproduction and limits

Ignored harness: `.omx/reports/platform-admin/s07-capacity/backup-restore.mjs`.
Run only with the owned capacity database quiescent:

```sh
node .omx/reports/platform-admin/s07-capacity/backup-restore.mjs \
  .omx/reports/platform-admin/s07-capacity/runs/s07-20261001212929-d12b32
```

Raw evidence:
`.omx/reports/platform-admin/s07-capacity/runs/s07-20261001212929-d12b32/restore-20261001230845/restore-evidence.json`.
The script records archive identity, command timings, per-table fingerprints,
and explicit invariants; it refuses to clone a source with active clients.

Database roles, infrastructure secrets, encryption keys, external storage,
point-in-time recovery, and production credentials were outside this local
database-only rehearsal. Empty personal-access/task-token tables and the empty
installation-report cursor table were fingerprinted but do not provide
populated-row coverage. Native clients were not reconnected to the restored
database, and pending cancellation delivery was not executed. The production
maintenance and binary/schema compatibility gates remain in
`release-checklist.md`.
