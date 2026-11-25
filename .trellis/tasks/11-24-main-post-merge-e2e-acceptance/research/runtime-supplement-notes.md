# Runtime supplement preparation

Status: **READY_FOR_INDEPENDENT_REVIEW**. This is preparation evidence; none of
the supplement's product journeys has run. No accepted clearance was issued.

- Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.
- Run: `main-post-merge-e2e-20261124-080504`.
- Launcher: `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/runtime-supplement/run_runtime.py`.
- Evidence: `artifacts/qa/main-post-merge-e2e-20261124-080504/runtime-supplement/`.
- External state: `/Users/artisan/.multica/qa/main-post-merge-e2e-20261124-080504/runtime-supplement/`.
- Prepared input digest: `989911dc9ddf60c162e15b47cc8c36e43f238a31a0cca67329150fa1b528de3c`.
- Inherited original input digest: `80bc39dd1a392b93766074a752e3de24719898256732a04555558020af5191d0` (`dotenv-quiet-01`).

The original runtime, source, builds, services, plan, credentials and PostgreSQL
were read-only. Preparation started no service/container, opened no SQL or HTTP
connection, and ran no browser/product test body. Offline synthetic guard tests
are reported separately below.

## Bounded change plan and completed changes

The plan was recorded before narrowing the copied code: bind exact canonical
families, remove generic optional-profile execution, reject unprepared entries,
run offline counterexamples, then publish a review-required preparation.

The four frozen executable sources are `runtime_lib.py`, `run_runtime.py`,
`family_options.py` and `supplement_selftest.py`. Only the sibling copies changed.

- Every family now admits only its named current-source files and actual
  auth/native requirements. Unknown, empty, duplicate and aliased selections
  refuse before runtime state/SQL writes.
- `collaboration-detail-journey.spec.ts` calls `seedDetailJourney()` without a
  password argument. Its manual detail family is therefore **legacy-only**;
  its gate is creation=true, execution=false, host=false. The private provenance
  binds current source/artifacts/build and owned listeners after readiness.
- Migration sets fresh RFC3339 cutoff/deadline and the pinned current server
  binary. Native password binding is an admitted migration file only with
  `--native`; its original checks remain unchanged.
- Auto initialization requires a fresh label containing `admin_auto_init` and
  refuses `--admin` prebootstrap. The canonical test still requires an empty
  initialization ledger; the launcher does not reset or seed that ledger.
- Host Provider matching uses the canonical callbacks. `host-provider` supplies
  report/failure/Git-marker/output operations. `multi-account-provider` supplies
  the memory-write/comment callback required by closure and timeout. The
  `cross-client-provider` uses Linux `/proc/self/ns/pid` and explicit A/B argv;
  it is rejected as a macOS Claude host Provider.
- Generic profiles, probes and unrelated CLI/catalog/changelog/provider flags
  cannot enter the supplement. `setup` and shared `cleanup` always refuse. The
  CLI exposes only prepare/run/audit; a new product run gets a new DB, HOME,
  configuration and ports and cannot overwrite an existing run label.
- The supplement refuses execution while the original runtime lock exists.
  This does not change the original launcher: the parent must continue to start
  the original and sibling lanes sequentially and own shared PG shutdown.
- Strict first-attempt identities, file/title reconciliation, raw errors,
  nonempty audit, source/build/tool/resource checks and failure retention remain
  active. Copied original selftest/review files are retained as references;
  the current sibling's runnable test entry is `supplement_selftest.py`.

## Supported prepared scope and future commands

These are **future commands after independent review**, not executed results.
Each file should run sequentially with its own unused label and fresh database.
The launcher collects the selected identities before bodies run and records
one worker, zero retries and complete terminal reconciliation.

```sh
QA_SUPPLEMENT=.omx/private/qa-work/main-post-merge-e2e-20261124-080504/runtime-supplement/run_runtime.py
python3 "$QA_SUPPLEMENT" run --family migration --mode password --label migration-01 e2e/password-migration.spec.ts
python3 "$QA_SUPPLEMENT" run --family migration --mode password --native --label password-binding-01 e2e/password-binding-desktop.spec.ts
python3 "$QA_SUPPLEMENT" run --family admin-init --mode password --label admin-auto-init-01 e2e/platform-admin-initialization.spec.ts
python3 "$QA_SUPPLEMENT" run --family recovery --mode password --native --label recovery-01 e2e/collaboration-recovery.spec.ts
python3 "$QA_SUPPLEMENT" run --family information --mode password --native --label information-01 e2e/collaboration-information-recovery.spec.ts
python3 "$QA_SUPPLEMENT" run --family ordinary-ui --mode password --native --label ordinary-ui-01 e2e/collaboration-recovery-ordinary-ui.spec.ts
python3 "$QA_SUPPLEMENT" run --family subscriptions --mode password --native --label subscriptions-01 e2e/collaboration-recovery-subscriptions.spec.ts
python3 "$QA_SUPPLEMENT" run --family detail --mode legacy --native --label detail-01 e2e/collaboration-detail-journey.spec.ts
python3 "$QA_SUPPLEMENT" run --family stage-approvals --mode host --native --knowledge off --label stage-approvals-01 e2e/collaboration-stage-approvals.spec.ts
python3 "$QA_SUPPLEMENT" run --family output --mode host --native --knowledge off --label output-01 e2e/collaboration-runtime-output.spec.ts
python3 "$QA_SUPPLEMENT" run --family host --mode host --native --knowledge off --label host-runtime-01 e2e/collaboration-host-runtime.spec.ts
python3 "$QA_SUPPLEMENT" run --family host --mode host --native --knowledge off --label host-switch-01 e2e/collaboration-host-switch.spec.ts
python3 "$QA_SUPPLEMENT" run --family host --mode host --native --knowledge off --label host-squad-01 e2e/collaboration-host-squad.spec.ts
python3 "$QA_SUPPLEMENT" run --family host --mode host --native --knowledge off --host-provider multi-account-provider --label multi-account-01 e2e/collaboration-multi-account-closure.spec.ts
python3 "$QA_SUPPLEMENT" run --family host --mode host --native --knowledge off --host-provider multi-account-provider --label timeout-recovery-01 e2e/collaboration-timeout-recovery.spec.ts
```

Recovery families explicitly bind their flags and an owner0700 external profile
root. Stage approvals bind the already-pinned real Git executable. Output uses
the genuine archived task
`.trellis/tasks/archive/2026-11/11-20-full-e2e-regression-20261120`, checked by its
actual metadata, with a **new current-source allocation** for this campaign's
new database, private uploads, origins and deployment. It never reuses that old
task's reports/configuration and never fabricates its task identity. The parent
must associate any later output result with this task's ledger.

This supported scope proves Web/real API and matching Electron renderer with
fixture Main, and controlled local daemon/CLI/Provider operations where the
canonical family uses them. It does not establish production Main, installed
clients, commercial-model behavior or another operating system.

## Actual verification

Commands actually executed, with output/exit in the evidence directory:

```sh
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/runtime-supplement/supplement_selftest.py
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/runtime-supplement/run_runtime.py prepare
```

- `offline-selftest-01.log` / receipt: initial 41 named tests PASS, exit0.
- `offline-selftest-02.log` / receipt: final **46 named tests PASS**, exit0.
  These are one final complete guard suite; the earlier count is not added.
  They cover exact modes/files/native/Provider admission, rejected unprepared
  options, preserved original and copied hashes, effective private environment,
  resource identity/port/image/volume drift, first-attempt reporter refusal,
  preparation overwrite refusal and absence of self-issued clearance.
- The final guard suite also creates only synthetic external files and calls
  the existing **offline** `assertRuntimeOutputAllocation` helper against the
  supplement's generated record. It opens no SQL, HTTP or browser connection.
- Four current sources pass AST parsing, Python compilation, `tabnanny`
  indentation checks and the bounded unused-import scan (zero unused imports).
  Python 3.14 was used. `ruff`, `mypy` and `pyright` are not installed; no
  additional dependency was installed and no formal Python type-check is claimed.
  Repository TS/Go lint/type-check gates remain the parent campaign's evidence;
  this preparation lane did not rerun or mutate those builds.
- `prepare-01.log` / receipt: exit0, read-only existing resource inspection.
  `prepared.json` records product tests=0, SQL=0 and services started=0 **during
  prepare**; the offline guard tests above are separate.
- `preparation-binding-01.json`: all 11 original files remain byte-identical and
  canonical; original input digest unchanged. The existing PG remains
  `c7602a619c9bd0a870772dffff52dfea8b8c650545ef0d45b35d2ffdfdea80fe` on
  loopback port **60502**, with the original image, owner and volume verified by
  the preparation resource guard. No PG setup/configuration/cleanup occurred.
- New inputs, provenance, plan, credentials, database record and clearance
  template are owner0600. External execution leaves are owner0700 on the verified
  Git-external chain. `clearance-template.json` is `REVIEW_REQUIRED`; there is
  no sibling `clearance.json`.

The parent can independently inspect the frozen four sources, exact bindings,
guard output and template before authorizing a concrete lane. An accepted
clearance must match the prepared input digest; this preparation does not issue it.

## Deferred families and exact missing preparations

| Family | Required separate preparation / honest result |
| --- | --- |
| Recovery extras and native response-stage holds | Actual natural native transport probe acceptance precedes `PAY241_S3_RESPONSE_STAGE_HOLD_ACCEPTED=1`; the sibling never sets it. Extras/runtime files outside the selected output family remain unadmitted. |
| Stage execution | Real service-test AF_UNIX control, owner0500 artifact staging, owner0700 output and eight generation UUID slots; stage approvals alone supplies none of this execution proof. |
| Project knowledge automation/P6 | Its exact `mca_knowledge_browser_*` database guard, real registered/onboarded and enrolled daemon profile plus the controlled fake Claude/provider; generic password detail does not satisfy it. |
| Cross-client Linux/installed compatibility | Actual static Linux binaries and namespace topology/host relay; current Darwin binaries cannot be labeled Linux. |
| Four critical Agent smoke | Keep the dedicated source-bound smoke config and exact four-name validator; the generic profile path is intentionally absent from this sibling. |
| Performance/L10 | Bound manifest, 100 accounts, 100 sockets ×100 samples, and prepare→propagation→writeback in one API/DB lifetime, preserving every canonical threshold/sample; fresh per-file sibling runs are not this protocol. |
| Firefox/WebKit/L8 | Required `firefox-1509` and `webkit-2248` cache directories and installation-complete markers are absent. Status NOT_RUN; no download/install attempted. Registry SHA256 `6b6e0538d6e46eef71d3e1a0dcb65f8f5ea7914033c8150529d8e207afb6030c`. |
| Full production Main/install/updater | Separate Main/package source/artifact, profile, process and exact LaunchServices restoration preparation; forbidden in this child lane. |

No product interface or authoritative `.trellis/spec/` was changed. Findings and
run-bound contracts are retained here for the parent review and result ledger.
