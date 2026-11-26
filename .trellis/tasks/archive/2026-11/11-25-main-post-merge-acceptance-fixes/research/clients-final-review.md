# Research: Final client fixture and evidence review

- Query: Independently inspect all eight client fixture/spec diffs and raw final13-case evidence, especially snooze response draining and retained C1.
- Scope: internal; source/evidence audit only.
- Date: 2026-11-25
- Worktree: `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37`.

## Findings

**Verified all13 recorded Playwright passes and the frozen/source/resource evidence. PAY, resource publishing and delayed-read results are accepted within their recorded scope. Snooze's six test passes are authentic, but its claimed error-free observation has one unresolved teardown gap below. C1 remains open; no manual-detail pass is accepted.**

### C2 — P2: A late snooze observer error escapes the final assertion

The final02 evidence for **“Web group snooze survives reload, changes time, undoes and reminds the original unread group”** contains `traffic_observation_failed` for a received200 `/api/inbox/unread-summary`, reason `client_closed`, at `2026-11-25T11:09:19.056Z`. Its next cleanup record at `11:09:19.058Z` says fulfilled, and Playwright records the case passed. This is actual retained evidence, not a hypothetical race.

Path under `.omx/private/acceptance-fixes/clients/snooze-final02/`:
`results/inbox-snooze-Inbox-snooze--cf74e-s-the-original-unread-group-chromium/inbox-snooze-evidence.json`.

`e2e/fixtures/inbox-snooze.ts:357–365` calls `traffic.settle()` and checks errors **before** `context.close()`/native close. A new response reader can start afterward; its body-read failure reaches `traffic.errors` and the evidence but no later assertion. The added request drain improves reload ordering and does not suppress errors, but does not seal the observer at shutdown. This weakness existed in the close structure already; the new run demonstrates it remains after the repair.

Action: preserve this exact late failure and qualify the report's zero-error observation claim. For strict observation acceptance, finish/seal observation through close and await/check all already-admitted response readers before considering cleanup fulfilled; any deliberately excluded post-boundary traffic needs an explicit recorded boundary and cannot retroactively relabel this failed read as observed. Verify with a deterministic late-reader control and a fresh bounded replay. Do not weaken schema assertions, erase error facts or raise scenario deadlines. This finding does not establish a product snooze/unread defect, and it does not justify rewriting the authentic Playwright terminal from PASS to FAIL.

### Eight-file source review

| File | Assessment |
| --- | --- |
| `e2e/fixtures/collaboration-pay-creation.ts:101` | Exact Participating heading matches the existing `view=participating` destination. Business assertions unchanged. |
| `e2e/admin-resource-publishing.spec.ts:149` | Uses existing sidebar helper and opens Resources & connections before Skills. Output-directory override is fixture storage only. |
| `e2e/inbox-snooze.spec.ts:44` | Uses the actual Switch workspace selector and waits for the selected workspace's Inbox href. Same wakeup/unread journey retained. |
| `e2e/fixtures/inbox-snooze.ts:229,283` | New in-flight request tracking and pending-reader drain preserve strict schemas and `errors=[]`. `body_resource_unavailable` is still an error; no blanket ignore added. C2 above remains at close. |
| `e2e/fixtures/collaboration-recovery-extras.ts:669` | Limits denial locator to the direct detail alert while retaining all protected-content/dialog/composer absence checks and35s deadline. |
| `e2e/fixtures/collaboration-installed-main.mjs:528` | Exact Current handling heading; no OR fallback; original45s deadline. Native acceptance is separate. |
| `e2e/fixtures/collaboration-detail-journey.ts:812` | Workspace resume uses current control and route/native-scope evidence. No history traversal or explicit issue reselection remains; original selected-issue/scroll/single-tab assertions remain. C1 is unresolved, so no successful replacement journey is claimed. Type-only exclusion of unsupported `repair_information` does not alter runtime actions. |
| `e2e/collaboration-detail-journey.spec.ts:311` | Passes workspace name to the helper; original business sequence remains. |

The snooze drain is used before the existing explicit reloads (`inbox-snooze.spec.ts:390,408`) and close. It does not extend the original test deadlines or change expected data. Final02 has no `body_resource_unavailable` fact; the different late `client_closed` fact above remains material to the observation claim.

## Independently verified results

| Group | Actual terminal result | Bound evidence |
| --- | --- | --- |
| PAY final02 | 2 PASS,0 retry/skip/fail |26 evidence files rehashed |
| Resource final01 | 3 PASS,0 retry/skip/fail |61 evidence files rehashed |
| Snooze final02 | 6 PASS,0 retry/skip/fail |152 evidence files rehashed;621 facts, including one late observation failure |
| Delayed final01 | 2 PASS,0 retry/skip/fail |28 evidence files rehashed; both cases actually start all3 original subcases |

All267 listed evidence files match their saved hashes. All four raw reports have zero top-level errors and one passed result per named case; completion records exit0 and preserve before/after gates. Four distinct recorded databases are `fix_clients_pay_final02`, `fix_clients_resource_final01`, `fix_clients_snooze_final02`, `fix_clients_delayed_final01`.

Both frozen trees were independently rehashed:142,828 file entries each, zero mismatches. All7,272 listed dependency symlinks per tree still resolve internally. Before/after content records are equal for all four runs. API executable and Go1.26.11 compiler hashes match their build record. Runner/probe/Web-launcher identities match. The fresh delayed-read parent review binds the actual response-probe bytes and admission; both hashes independently match. Delayed evidence marks real-model/provider and installed-Main execution absent, so none is inferred.

All four cleanup audits record absent original API/Web/renderer processes, closed allocated ports and zero other DB clients. These are verified retained receipts, not new live cleanup probes. The final handback authorizes parent/native protocol cleanup; global macOS default/registration restoration still needs its separate final receipt.

Targeted lint is **not green**: baseline/current diagnostics match and the report preserves nonzero exits. No new lint diagnostic was introduced according to the retained comparison. Typecheck and11 Main fixture-node-test passes are lane-reported retained checks; they do not prove actual packaged Main behavior.

## Final integration concerns

1. **C1** remains a product-choice/coverage gap. No history or issue-selection bypass is accepted; no manual-detail final PASS.
2. **C2** late snooze observation error must be qualified/resolved as above, despite six real test PASS terminals.
3. **Native old CLI** original240s timeout is parent-reported reproduced and remains FAIL pending actual diagnosis; fast one-shot timing does not justify shared relay or deadline changes. Main outcomes remain separately reviewed.
4. **Full frontend** first unbounded-worker failure stays retained. The corrected canonical two-worker suite is new evidence, not permission to edit unrelated product code or change test timeouts.
5. **Other retained boundaries**: D10 public403 FAIL; nine Namespace policyBLOCKED with historical causeUNKNOWN; real API-issued task-token evidence and full backend owning-package inventories must be verified by their assigned reviewers. Do not infer historical Host causality from later passes.
6. **Final resource/source closure**: reconcile exact final candidate and all lane outputs, global protocol state, owned process/container/volume cleanup and retained resource ownership. Database/template asset completeness and remaining-backend review are assigned to another independent checker.

## Files found / related specs

- Eight files above and `.trellis/tasks/11-25-acceptance-client-journeys/report.md` — implementation/report scope.
- Public `artifacts/qa/main-acceptance-fixes-20261125/clients/` final group receipts, admissions, source audits, cleanup and evidence hashes.
- Private clients final raw Playwright reports and per-case evidence — independently parsed actual terminals, subcase counts and snooze failure fact.
- Private `final-freeze-02.json`, `final-freeze-03.json` and physical external roots — independent full file/link verification.
- `clients-final-review-audit.json` — detailed independent hashes, terminals, frozen audits, subcases and observed failure.
- `CLAUDE.md`, core global-scope-navigation and web E2E-run-environment specs — preserved assertions, truthful evidence and identity/cleanup boundaries.

## Caveats / Not Found

- Reviewer did not run tests, browsers, adapters, Docker/SQL, services or Git commands. Only research artifacts were written.
- Full source/link content integrity was checked; this is not a new full build or execution. Existing receipt claims about live resources are not substituted for final global cleanup.
- Client outcome acceptance is scoped above and is not release approval. No external references were needed.
