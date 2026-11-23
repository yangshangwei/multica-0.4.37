# Independent S2 frontend check — 2026-10-21

Status: **one confirmed refusal-recovery defect fixed; one UI selection defect remains open.** This review covers S2 frontend plus the coordinator's API/barrel/locale integration. Backend, S1/S3a, S3 history, production visuals and complete AC-I03 are outside this verdict.

Source writes were frozen for the production build. The coordinator granted one narrow pre-build exception for the confirmed refusal-code fix and its canonical core regression. After that correction and verification, source writes stopped; the later UI diagnostic used files only under the external task directory.

## Findings (fixed)

### P2 — A durable visibility refusal remained an unknown resource request

- Files: `packages/core/collaboration/resource-schema.ts`, `packages/core/collaboration/resource.test.ts`.
- The actual S2 backend persists an issue-scoped HTTP 404 receipt with `action=offer_resource` and `code=resource_options_unavailable` if the selected roster fails the current inventory visibility predicate. The frontend's failure enum omitted that code. The real ApiClient transport returned the complete receipt, but parsing returned null; `executeCollaborationResource` then threw `CollaborationUnavailableError` and retained the draft as unknown. An original-key GET returned the same unrecognized result, so recovery could not settle the known refusal.
- Fix: recognize that existing backend code in the failure enum. No correlation, authorization or HTTP-status guard was weakened. The existing confirmed-result path now retires only the matching pending intent and keeps the authorized issue cached; the form can preserve its selection for explicit refresh/reselection.
- Canonical regression: `settles a durable offer visibility refusal through its original request (recovery=%s)` in `resource.test.ts`, with direct POST and lost-response/original-GET variants. Both use the real ApiClient and draft helpers, verify refusal settlement, removal of the exact pending draft, unchanged issue access, and POST/GET identities without an automatic replacement write.
- RED: both variants failed with `CollaborationUnavailableError`; 11 unrelated tests were excluded by the diagnostic filter. GREEN: the resource helper/schema/API suites passed all 32 tests. The later full bounded resource/Switch run passed 65 tests, including these two additions.

## Findings (not fixed)

### P2 — Losing organizer authority hides an open confirmation but leaves remaining actions locked

- File: `packages/views/collaboration/resource-offers.tsx`.
- Reproduction: a resource owner who is the current handler opens an unsubmitted **Adopt resource** confirmation. The issue is then transferred, so that viewer remains a participant/resource owner but loses organizer authority. The render condition hides `OfferConfirmation` when `canOrganize` becomes false. Its parent `selection` nevertheless remains non-null, so the permitted **Offer resource** button stays disabled by `selection !== null`. The confirmation's Close button has disappeared. Refresh resets cursors/queries but does not clear selection; collapsing and reopening Manage AI resources is currently required to recover the local UI.
- A repository-external test copied only the existing real component harness, mounted the actual `CollaborationExecutionPanel`, opened the confirmation, applied the current issue's role change and asserted that the hidden confirmation's remaining owner action becomes usable. It deterministically failed because **Offer resource remained disabled**. The existing warm-cache authority test covers an unopened selection and therefore missed this state.
- Evidence: `/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s2-frontend/authority-selection-red.log` and `.json`; reproducer/config in `ui-diagnostic/authority-selection.test.tsx` and `ui-diagnostic/vitest.config.ts`. Final diagnostic result: 1 executed test, 1 failure, exit 1. The initial external-path setup failure is separately retained as `authority-selection-setup-error.*` and is not product-failure evidence.
- Recommendation: close or otherwise release an invalidated **unsubmitted local selection** when its current action permission disappears, while preserving all persisted pending-request recovery. Cover an open adoption losing organizer rights; consider the same local-state boundary for a supply form hidden when an issue closes, since own-offer withdrawal remains permitted there. Keep the current ACL filtering and independent consent/withdrawal gates.
- Disposition: reported to root during the build. No UI/source/test/spec edit was applied for this finding because the active source freeze remains in force. This is the one open code finding.

## Other reviewed boundaries

- `canOfferCollaborationResources` uses current account scope, supported execution, write context, participant control and open lifecycle, independently of the parser's intentionally false `submit_command`. Organization additionally requires the actual current creator/handler and Invite capability. Closed-issue offer reads retain organizer authority independently of adoption; current providers can withdraw stale/closed unadopted offers.
- Candidate forms use explicit execution workspaces and the dedicated offer inventory reader. Current workspace/candidate failure hides prior inventory and prevents changed-roster submission. Supply does not capture target-set version. Adoption captures the selected offer version and target-set version; withdrawal captures offer version. Existing exact draft and recovery helpers preserve original intent across uncertainty.
- The new schemas validate issue/resource/workspace/owner/runtime identities, complete unique rosters and explicit terminal fields. Public summaries strip undisclosed configuration/authority fields. Request mapping uses offer IDs in paths, with strict input unions; success receipts require the appropriate action/code/status/scope and no grant. Adopt/withdraw correlate the original offer and exact next version; redacted recovery remains the strict seven-field DTO.
- The three ApiClient readers validate request/response scopes and use the existing optional-resource boundary. Missing optional offers or source-only resource denial do not erase otherwise valid issue data; explicit object denial and obsolete account/access epochs retain their existing protection. Offer queries use Server/viewer/issue keys, original cursor identities, cancellation and current authorization.
- Pagination rejects an immediately repeated cursor and the UI prevents revisiting a cursor already in its forward stack. Every rendered page filters entries using the current role, including warm cached pages. Parent resource components are keyed by issue identity. The open-selection edge is the P2 above.
- Adoption reuses existing resource confirmation/summary components and leaves each owner's consent independent. Offer withdrawal never maps to grant revoke, Stop, Resume, invitation or target registration. The execution panel change only adds the three resource action labels to its existing result allowlist. The adjacent Switch tests pass.
- New copy is under the top-level `resources` keys in both English and Chinese; forms have accessible names, native labelled selects, current-state messages and the existing wrapping/touch-target classes. No visual verdict, browser screenshot or native-product outcome is inferred from source/DOM tests.

## Verification and source provenance

All evidence below is under `/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s2-frontend/`.

| Reviewer execution | Result and artifact |
| --- | --- |
| `pnpm --filter @multica/core test collaboration/resource.test.ts -t 'settles a durable offer visibility refusal'` before the enum fix | RED: 2 failed; 11 filter-excluded; `visibility-refusal-red.log/.json` |
| `pnpm --filter @multica/core test collaboration/resource.test.ts collaboration/resource-schema.test.ts collaboration/resource-api.test.ts` after the fix | PASS: 3 files / 32 tests; `visibility-refusal-green.log/.json` |
| Targeted ESLint on `resource-schema.ts` and `resource.test.ts` | PASS, exit 0; `fix-lint.log/.json` |
| `pnpm --filter @multica/core typecheck` | PASS, exit 0; `fix-typecheck.log/.json` |
| `pnpm --filter @multica/core test collaboration/resource collaboration/switch` | PASS: 7 files / **65 tests**, zero skips; `final-core-resource-switch.log/.json` |
| `DEBUG_PRINT_LIMIT=1500 pnpm --filter @multica/views test collaboration/resource-offers.test.tsx collaboration/resource-panel.test.tsx collaboration/execution-panel.test.tsx` | PASS: 3 files / **15 tests**, zero skips; `final-views-resource.log/.json` |
| External current-role/open-selection diagnostic | FAIL: 1 executed test; `authority-selection-red.log/.json` |

The final bounded core/views commands were explicitly requested by root after the fix. Their JSON records include SHA-256 values for the S2 resource/Switch sources and tests, ApiClient, barrel, execution panel, labels and both locale files. Both report `source_stable=true`, and the recorded hashes were checked again against the reviewed tree. These current-source executions replace reliance on the implementation report's previously reported 63/15 counts; the two added core cases explain 63 → 65. The 877-test collaboration suites were not rerun.

The frontend implementation report's broader views ESLint/typecheck successes remain attributed to its author; this review did not claim to execute those again. The inspected current views tests compile and execute successfully, but the additional runtime UI regression correctly remains failing until the open finding is fixed.

## Handoff

Reviewer product changes are limited to the one refusal code and its two core regression cases. This report is the only other repository file authored in this review; UI diagnostic files/logs remain outside the checkout. No commit, archive, provider, backend, schema, dependency, styling or rollout-gate change occurred.

Root owns the open UI repair, its canonical regression and any production rebuild/visual verification needed afterward. Production Web/Electron resource-offer journeys, current wide/narrow/200% captures and complete AC-I03 remain pending. S2 frontend is ready for final approval after the open selection-state defect is resolved and checked.
