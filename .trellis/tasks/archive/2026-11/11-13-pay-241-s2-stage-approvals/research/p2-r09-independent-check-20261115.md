# PAY-241 S2 P2 r09 independent acceptance

**P2_CODE_ACCEPTED_R09_SCOPED — zero open blocking findings.** ROOT may integrate the accepted narrow P2 delta. All eighteen required profile observations pass: eight unchanged normal closures retained from r07, a new canonical normal22, and nine new exact-source race closures (canonical21 and original unused10–17). No test was retried or silently omitted. Current ROOT joint-source and four Web/Desktop scenario acceptance remain separate.

## Source and review

The accepted source is the 4,968-file private `p2-k06-repair-current-r03/source`, manifest SHA256 `80abb746c9c905bbd54e73db05bfbce0c84b02f9057b7c77b67632e3496549a1`. The reviewer independently rehashed every file and checked the sole router-test repair against each prior source and the real handler/lifecycle/M1 contracts. All 4,967 other files retain their bytes and modes. The existing router-ready test/import prefix is also identical to its r07 normal source, so those eight normal closures were reused through a separate source-reuse binding; the original failed K06 was never reused as a pass.

The public success GET now uses its actual map: HTTP200, `no-store`, omitted status, exact `completed` code/revision/confirmation and database submission identity. Reopen checks the original SQL's `paused` / `issue_reopened` contract, an exact lifecycle+1 and unchanged execution/consumption counts. The unused old grant receives the existing M1 target-mismatch `delivery_completion_conflict`, with one exact durable refusal and stable original-key GET/raw-body replay. No product/SQL order changed and no `OneOf` error shortcut was introduced.

## Complete profile results

| Profile | Mode | Top/named/leaf observations | Evidence |
| --- | --- | --- | --- |
| p2_service_closure_normal_r01 | normal | 7/11/9 | retained unchanged normal |
| p2_handler_ready_normal_r01 | normal | 1/1/1 | retained unchanged normal |
| p2_router_ready_normal_r01 | normal | 1/1/1 | retained unchanged normal |
| p2_handler_missing_864_ledger_normal_r01 | normal | 1/2/1 | retained unchanged normal |
| p2_handler_submission_id_index_normal_r01 | normal | 1/2/1 | retained unchanged normal |
| p2_handler_execution_approval_index_normal_r01 | normal | 1/2/1 | retained unchanged normal |
| p2_handler_foundation_policy_index_normal_r01 | normal | 1/2/1 | retained unchanged normal |
| p2_handler_rollback_schema_normal_r01 | normal | 1/1/1 | retained unchanged normal |
| p2_k06_reopened_normal_r09 | normal | 1/1/1 | new exact-source execution |
| p2_k06_reopened_race_r09 | race | 1/1/1 | new exact-source execution |
| p2_service_closure_race_r01 | race | 7/11/9 | new exact-source execution |
| p2_handler_ready_race_r01 | race | 1/1/1 | new exact-source execution |
| p2_router_ready_race_r01 | race | 1/1/1 | new exact-source execution |
| p2_handler_missing_864_ledger_race_r01 | race | 1/2/1 | new exact-source execution |
| p2_handler_submission_id_index_race_r01 | race | 1/2/1 | new exact-source execution |
| p2_handler_execution_approval_index_race_r01 | race | 1/2/1 | new exact-source execution |
| p2_handler_foundation_policy_index_race_r01 | race | 1/2/1 | new exact-source execution |
| p2_handler_rollback_schema_race_r01 | race | 1/1/1 | new exact-source execution |

Each mode has **nine profile observations, fifteen top observations, twenty-three named observations and seventeen leaf observations**. The four native controls deliberately share one parent test name across their separate databases. After grouping by package/test name, each mode has **twelve unique top names, twenty unique named tests and seventeen unique leaves**. Normal/race groups and exact named sets agree. These are test observations, not additions to the parent24-case business denominator.

All executed commands used ROOT's exact argv, unchanged runner/configuration/default pool/deadlines/timeouts and one PAY database suite at a time. Complete Go1.27 build/test streams contain zero failures, skips, missing/extra names, duplicated terminals, unfinished tests or parser errors. Each actual command is bound to source/configuration before/after records and read-only database/catalog snapshots.

## Actual authority, rollback and retained history

Both handler and real-JWT router enabling returned actual HTTP200; explicit disable/re-enable and original-key replay preserved their own revisions and covered business facts. The four native negative controls reached production readiness checks through the original `*pgxpool.Tx`, with an actual exact864 DELETE or single specified index DROP. Each returned503 `stage_schema_unavailable`, stored no successful policy operation and rolled back the precise catalog/ledger control.

The incomplete-submission disable case committed its own disabled revision while the index was truly missing. Original success GET/body replay remained stable; new enabling was refused. Restoration used the original migration862 single `CREATE UNIQUE INDEX CONCURRENTLY` outside a transaction. The normal OID was **15980467 → 0 → 16217550**; race was **15980467 → 0 → 16265880**. The new OIDs differ as expected; logical definitions, ready/valid/live state, other index OIDs, constraints and ledger remain equivalent.

Canonical normal and race both performed the same actual composition: stage-off M1 enforce authorization issued by the source owner to a distinct outside consumer, one committed self-completion and consumed terminal, three actual P2 policy PUTs, actual reopen, original success GET/raw POST recovery, and one fresh old-grant refusal. Actual reopen was **lifecycle2 → 3, paused, issue_reopened**, with zero added runs/commands/root tasks and consumed count1 → 1. The success body omitted status in both recorded logs. The old unused grant got409 `delivery_completion_conflict`; its terminal count stayed zero. Original history fingerprints stayed equal before/after replay and after the bounded refusal was excluded from the allowed comparison. Separate independent proof JSONs cross-check the actual log identities with retained database authorizations, terminal, policies and evaluation.

Immutable M1 history remains deliberately retained on both canonical databases: one evaluation, two policy revisions, two authorizations and one consumed terminal. The reviewer did not clear business history to obtain a green audit.

## Verification and cleanup

- Formatting: `gofmt -l` on all three P2 files passes with no output.
- Static analysis/type compilation: `go vet ./internal/handler ./cmd/server` and `go build ./internal/handler ./cmd/server` pass with no output in the credential-free metadata environment. No TypeScript changed in this frozen increment.
- All eighteen successful profile observations and their exact normal/race inventory alignment pass.
- Each runtime HOME/TMP/GOTMPDIR chain was rechecked for actual user/modes, trusted ancestors and absence of a Git root before/after execution.
- A final read-only audit of all eighteen successful databases confirms correct identity, exact schema/readiness and zero other connections. The final 4,968-file source remains unchanged; all ten recorded test runner PIDs are absent. Final-state summary SHA256 is `07bf8ed6ffe0bd421a4b41a0337c35556cdf8255bead61c52e260a56c70e2c4e`.
- The reviewer changed only its allowed reports/external evidence helpers. No LIVE product, test, migration, original configuration, Git or peer path was edited.

## Preserved failures and limits

Original r07 and r08 reports retain their FAIL verdicts, raw logs and failed databases18/20. The r07 internal-status expectation and r08 open-queue expectation were corrected only in new test sources. The original pre-Go read-only42703 auditor error and its separate field correction remain preserved. Profile19 remains unused; it is not counted as a required missing execution after ROOT replaced its descriptor. No failed or unexecuted run is relabeled as passing.

This acceptance is limited to the frozen80abb source and its P2 closure bindings. Current ROOT's5038-file joint graph, knowledge hooks/virtual-off interactions, final service/API risk verification, matching binaries, A01–A12, four actual Web/Desktop scenarios, S3–S5 and parent24-case acceptance remain separately required. No real model account, payment/funds, external deployment or production action was used. ROOT should integrate only the three owned P2 file deltas and preserve peer source; this reviewer did not authorize whole-file rollback or claim the current ROOT joint graph was tested.
