# r16 independent progress reconciliation

The retained r15 run is **1 PASS / 1 FAIL / 2 NOT_RUN**, with four planned and four reported cases. The first shared Web/Desktop role, review and QA journey passed in 75,445 ms. The second journey failed when its code-review Reason field remained disabled until the original 20-second action deadline. Late-deployment and final-completion journeys did not run. Repeated passes of the first journey count as one coverage item.

This review rehashed all **21 original output files**, the result, after audit and prior independent review. No hash or size mismatch was found. The raw result is `d236ece490d9fca4702f617fc1cd8b71540254796d1885442de3bb22b66e97c7`; the after audit is `dc4c268ca0e1ed9be30ec5f23ced1aa791ea70ec420ba998a81f8b33d096b226`. Exact references and machine-readable recommendations are in [r16-progress-reconciliation.json](r16-progress-reconciliation.json).

## Verified progress

| Scope | Retained actual evidence | Limit |
| --- | --- | --- |
| Code-review gate | No-signature advancement returned `409 stage_dependency_invalid`; self-review returned `403 stage_role_required`. Revoked, changed-head and removed/rejoined-signer decisions were rejected before a fresh review advanced to QA. | This is the controlled first journey; complete A04/A11/A12 acceptance remains open. |
| Failed-report and QA history | Actual CLI T1/T2/T3 produced report `59d74938…` for failed run `bad8d404…`, including `duplicate-callback` and two log entries. QA returned the candidate, an explicit new run `a3ef3c0e…` produced report `a345cb15…`, and QA approved that report. | Both reports bind the same candidate v2 and Git revision `b5c7a7d4…`. The fixture changed its injected fault from `duplicate_effect` to `none`; this does not prove a business-code correction. |
| Local artifact production | Two real artifact executions completed creation 201 → human approval 200 → execution 200. Both retained 2,048-byte tar archives match their hashes and contain the recorded source file. | These are local builds, with synthetic test-account approval. Release approval, deployment and rollback were not reached. |
| Cleanup at r15 after | Ten Host runs were settled with proof; unsettled runs, unreleased reservations, unknown/leased Host queue and extra runner/provider processes were zero. Three qualified synthetic producer rows and two artifact executions were retained. Gate booleans were restored with epoch 10. | The after audit retained API/bridge/Web/renderer services at that timestamp. Current stop/restart state belongs to the coordinator's subsequent receipts; this reviewer made no live process or database probe. |

The prior independent review records reviewer code reads changing from 200 to 503 and a disabled fieldset. The original 503 body and underlying SQLSTATE were not captured. Keep the read-error lock and record the root cause as unproven until the resumed diagnosis captures that evidence.

## Parent matrix recommendations

The observed parent matrix remains **0 PASS / 7 PARTIAL / 17 NOT_RUN**. Applying only the following two status changes gives **0 PASS / 9 PARTIAL / 15 NOT_RUN**. Preserve all 24 rows, historical columns and original evidence.

| Row | Recommended current status | New evidence and remaining gap |
| --- | --- | --- |
| TC-08 | `NOT_RUN` → `PARTIAL` | Record the actual unsigned/self-review/revoked/changed-head/signer-generation refusals and subsequent fresh review/QA advancement. Complete current-authority/expiry coverage and A04/A11/A12 closure remain open. |
| TC-12 | `NOT_RUN` → `PARTIAL` | Record failed-run/case/log association, human QA return, explicit new prepared run and new QA decision. State that both runs use the same code revision; business-code modification followed by regression, and complete A05/A11/A12 closure, remain unproved. |

TC-13 stays `NOT_RUN`: this run contains no actual Agent request attempting to overwrite the effective human QA decision. S4 payment, fulfillment, remote CI and deployment rows remain unchanged. Existing PARTIAL rows TC-03/05/07 can replace obsolete remaining-gap wording with the new completed first-journey evidence; in particular, TC-07's current gap must no longer say its positive Code review advance is still blocked by the old r02 timeout.

## Metadata reconciliation

At the 02:41 UTC handoff snapshot, the parent four-case counts and `r15_round_state` were already corrected. Two duplicated child fields still described older rounds: `actual_scenarios_not_run` was 3 instead of 2, and `latest_scenario_failure` still described the old Resume selector ambiguity. The coordinator should reconcile these to the retained r15 result while preserving the earlier `runtime_round_history` entries. This reviewer did not edit shared progress, CSV or metadata.

Full S2, A11 and A12 remain open. The complete 12A/18W/12Q denominator is retained. Synthetic human accounts, local PR/CI inputs and controlled providers do not establish business-person sign-off, real payment or fulfillment, remote deployment, commercial-model quality, or installed Desktop Main acceptance.

## Verification

- Rehashed 21 retained outputs and seven core result/audit/review references; zero mismatches.
- Rehashed both real tar archives and their source members; zero mismatches.
- Checked four-case/retry/terminal counts and the parent 24-row identity/status counts.
- Reviewed task artifacts, curated context and exact scope notes. No API, browser, database, model, migration or source/test command was executed.
- JSON/report consistency checks passed. Lint and type-check were not rerun because this slice changes only two research reports and explicitly preserves already accepted test evidence.
