# Password, administration and managed human collaboration lanes

Prepared against `/Volumes/artisan/code/2026/mc-core-qa-20261108`. No API, database, migration or test body was started by this preparation. Playwright `--list --reporter=json` collected **51 identities across 30 complete spec files**, exit 0. Collection statuses do not count as execution results.

The private executable controller and manifest are:

- `/Users/artisan/.multica/qa/core-20261108/password/run.py`
- `/Users/artisan/.multica/qa/core-20261108/password/manifest.json`
- `/Users/artisan/.multica/qa/core-20261108/password/collection-inventory.json`

The controller defaults to preparation-only output. Root must finish all browser work using the baseline API before invoking `--execute`; this operation switches only the already-owned API behind the existing production Web and built renderer origins. The fixed registry is `check-20261108144845-80493`. It validates that registry's source checkout and Web/renderer listener ownership before use. It never allocates another Web/renderer origin or rebuilds either client.

## Included scope

| Group | Complete specs |
| --- | --- |
| Password and native authentication | password-registration, password-branches, password-migration, password-binding-desktop, password-server-switch, mcp-desktop |
| Platform administration | platform-admin, platform-admin-initialization, platform-admin-accounts, platform-admin-controls, platform-admin-executions, platform-admin-installations-read, platform-admin-observability |
| Administration of collaboration/resources | admin-collaboration-switches, admin-resource-publishing, collaboration-administration-web, collaboration-administration-desktop |
| Human collaboration | collaboration-handoff-chain-web, collaboration-handoff-chain-desktop, collaboration-offline-handoff, collaboration-retained-attachment, collaboration-runtime-handoff-web, collaboration-runtime-handoff-desktop, collaboration-selection-boundary, collaboration-source-surface |
| Managed controls with synthetic offline installations | collaboration-control-journey-web, collaboration-control-journey-desktop, collaboration-leave-web, collaboration-live-revoke-web, collaboration-terminal-reopen-web |

The host-provider, memory-provider, knowledge ON/OFF, Linux namespace, installed CLI, packaged Main and actual model lanes remain separate. This manifest does not claim those groups are covered.

## Execution contract

Each selected spec gets a newly created local database, independent upload/catalog directories, private HOME/TMP, a new deployment UUID and JWT secret. Admin specs get distinct synthetic administrators through registration plus the current `platform-admin bootstrap` command. The initialization spec instead receives a fresh database whose name contains `admin_init`, with no administrator seeded. Password migration cutoff/deadline use current UTC and a two-day future deadline.

API pools use max 6/min 1. Password mode, managed installations and platform administration are explicit. Only the five managed-control specs start with collaboration execution enabled; host admission remains closed. Ordinary password/human collaboration specs keep execution closed. The database gate epoch advances on actual execution changes. No commercial LLM, SMTP, Cloud, external messaging or provider configuration is inherited into lane processes.

The controller builds current server and migration binaries into the private run directory only when execution begins. It retains allocation, migration/startup outputs, API health/configuration, source/config/binary identities, zero-retry Playwright JSON and per-test inventory. It stops on the first failed/skipped/retried case and restores the original API configuration/database in `finally`; failed databases and outputs remain available. Every retry needs a new run ID and fresh database.

```sh
# Read-only manifest output; no processes or databases started.
python3 /Users/artisan/.multica/qa/core-20261108/password/run.py

# Root runs only after releasing the baseline API. Use a new run ID.
python3 /Users/artisan/.multica/qa/core-20261108/password/run.py \
  --execute --run-id password-1

# A new isolated execution of selected original spec identities.
python3 /Users/artisan/.multica/qa/core-20261108/password/run.py \
  --execute --run-id password-targeted-1 --ids pw-35,pw-33,pw-34
```

## Preparation checks and remaining prerequisites

`python3 -m py_compile` and the preparation-only CLI completed successfully. Current Playwright collection completed with 51 identities; this is not runtime validation of the controller or product.

Before actual execution, root must confirm the baseline no longer uses the API, current Web/renderer builds represent the candidate product changes, cluster connection capacity remains sufficient, and the existing macOS Playwright browser cache is available. The controller verifies the served renderer and production Build ID and records current source identity; root still owns the interpretation of any fixture-only source change since the client build. No existing default database is touched, and no retained database is dropped.
