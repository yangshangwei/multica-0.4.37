# Extras-A R02 revoke-body diagnosis and R03 repair — 2026-11-18

The original complete six-top R02 run is **NOT_ACCEPTED: 0 PASS / 1 FAIL / 5 NOT_RUN**, retry0. Its first two notification subcases passed, then target withdrawal failed before forwarding the already captured comment. The unchanged eighteen-subcase denominator is **2 PASS / 1 FAIL / 15 NOT_RUN**. No earlier partial PASS is combined with a future run.

## Source diagnosis and bounded plan

The actual `RevokeCollaborationGrant` HTTP body embeds only `execution_view_version` (optional), `expected_lifecycle_epoch`, `expected_participant_epoch`, `expected_account_epoch` and `expected_grant_epoch`. `decodeCollaborationWorkflowBodyLimit` calls `DisallowUnknownFields`. The new extras fixture's general workflow `authority()` additionally supplies `expected_collaboration_revision`; spreading it into this resource revoke body causes400 before grant authorization. The canonical mention fixture selects only the three queue epochs.

Only `e2e/fixtures/collaboration-recovery-extras.ts` is writable. Keep the general four-observation workflow helper and every invite/participant/remove call unchanged. At the one revoke call, select the three current queue epochs plus the exact already-observed grant epoch, and record a safe checkpoint before dispatch with current grant ID/epoch, actor/target ID, original new key/path/body. Preserve every target-local refusal, prior immutable command, notification, original receipt and zero-dispatch assertion. No production decoder, permission, timeout, storage, dependency, shared helper or spec is changed.

## Original evidence limits

The immutable R02 Playwright/ledger and API request log retain the owner revoke endpoint, actual400, actor and request time. The old `TestApiClient.requestJSON` throws on non-OK before saving JSON, and its Node request is outside the page observer. Therefore the original revoke request body/400 JSON body and full grant-row before/after comparison were **not captured**. The old source establishes which fields it constructed; this is not represented as a saved original network body. Main's shutdown audits retain grant count4 and equal complete table counts, ledger and gate during shutdown, not equal full grant bytes before and after the refused request.

The original source, database, ledgers and logs remain unchanged. The new safe checkpoint will improve a fresh round's request/grant identity proof; it does not retroactively fill missing original evidence. Child DB/API/browser/Provider/TestMain execution remains0. Main owns a fresh complete six-top replay and independent check; full S3 and Extras-B are not accepted by this repair.

## Actual R03 repair and static verification

The one revoke call now explicitly selects `expected_lifecycle_epoch`, `expected_participant_epoch`, `expected_account_epoch` and `expected_grant_epoch`. Its safe `original_owner_grant_request_before_dispatch` checkpoint retains the original new key, path/method/body and actor/target/grant identity. The general workflow observations and all later target refusal, immutable-command, receipt, notification and zero-dispatch assertions remain intact.

The authorized static runner completed at `2026-11-18T11:32:58.406681+00:00`:

```text
python3 /Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/verification/extras-a-fix-r03/run-static.py
```

Strict targeted TypeScript passed with exit0 using inherited `strict`, `noUnusedLocals`, `noUnusedParameters`, `noImplicitReturns` and `noUncheckedIndexedAccess`; emission, incremental and composite modes are disabled. The actual source graph contains639 files. Its complete before/after hashes and modes match, and a later read-only comparison also matches current files. Explicit two-file ESLint using the existing Core rules and root/Core/Views declaration lookup passed with `--no-ignore`, exit0. Both commands have empty stdout/stderr, saved command/result receipts and distinct process IDs; no test body was executed.

Both commands entered through Node22 and the existing closed noDB wrapper/config. `DATABASE_URL` and `MULTICA_TOKEN` are absent; model/account keys are blank and only `PATH`, private fixture `HOME` and `LANG` are inherited at entry. The unchanged wrapper performs its existing read-only Git probe of fixture HOME before spawning the static command. No direct child Git command, Git mutation, database/API/browser/runtime execution or production-source edit was performed.

The R03 source seal is:

| Source | SHA256 | Change |
| --- | --- | --- |
| `e2e/fixtures/collaboration-recovery-extras.ts` | `9003c65a7ccd2143e180b58def76dce9a314efcdfab292c2be5ffb04cb7537c4` | Only changed source; original R02 was `f04e7009b98666eee64496812d41ed524449073fa9db12611fee2e486318bd93` |
| `e2e/collaboration-recovery-extras.spec.ts` | `9c1030bb0291118e691ebe6412f77dc794c2ac21003420b61ab15fe07423144b` | Unchanged |
| `e2e/fixtures/collaboration-recovery.ts` | `7f125d331adcfa85dab491b3def6ec07bca084238b71eae90f8e5eafcbffb3d1` | Unchanged |
| `e2e/collaboration-recovery.spec.ts` | `10cac566342deb24c68e90fd15cdcfd1c8a6aec50c9572116dfbb683c77b67ca` | Unchanged |

All four ROOT source modes remain0644. The exact R02→R03 diff SHA256 is `17dbc8f4387abbb1cf645295a9ae24192c7bdfe5f7680f7cd222b7a5f363249b`. The noDB config SHA256 is `bd5f4982bfcc6e51816086cfa02f8ee99ebaabb5451a147873a143131e14818b`; the static summary SHA256 is `4b2d548b747efd287bf6b9c15d071f2d3e1adcf5682165fc93522e3d6520f1eb`. The adjacent JSON binds the runner, wrapper, TypeScript/ESLint configs, source snapshots, raw static receipts and original failed runtime evidence.

The original failed Playwright report SHA256 is `c7878d09523b4ddc58711f051766ccad878947cd774af672e9c80f1b843f1b5d`; its notification ledger SHA256 is `2c3ac25f93d7d7588ac61f2fcbfb1112fdbe1ef6898fc174aca5cf7639a78ad3`. API stderr line495 retains the real400, with log SHA256 `bafaf2e5f2c9ffe22d1a85067ac6de65539acbce124217ff5ea8d45fce10982e`. Shutdown audits show grant count4, equal full table counts/ledger/gate across shutdown and zero remaining API connections. These are retained shutdown facts, not a full grant-row comparison around the refused POST.

## Remaining acceptance

This is **TEST-ONLY REPAIR / STATIC PASS / RUNTIME PENDING**. The unchanged fresh-run inventory remains6 tops/18 subcases across production Web and matching Desktop. Main must freeze the new fixture with the current matching source/configuration, allocate a fresh dedicated database, run all six cases without merging earlier partial passes, retain complete request/receipt/persistent-state evidence and obtain independent acceptance. This report does not change the parent24 matrix, TC-16, task status or archive state.
