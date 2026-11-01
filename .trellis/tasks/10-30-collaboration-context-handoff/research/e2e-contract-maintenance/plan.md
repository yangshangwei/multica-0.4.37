# Bounded E2E contract maintenance plan

## Gap and behavior owner

The approved UI now uses the searchable `Recipient` PropertyPicker, with an explicit exact-lookup branch. Old tests fill labels removed from the rendered surface. Current creation lawfully defaults to the creator. Comments deliberately refresh only their authority evaluation time on reads. `Run records` contains a separate handoff view in addition to paged `li[data-record-id]` entries. Production behavior remains unchanged.

## Write boundary

- Reuse the existing exact-selection interaction in `e2e/fixtures/collaboration-offline-handoff.ts`. Separate its pure picker interaction from the existing workflow confirmation wrapper, keeping the current handoff callers' `Selected account` assertion and creation callers' `Assigned to` assertions. Importing this module performs no password fixture/API setup. Do not add old/new selector fallbacks.
- Replace obsolete recipient interactions in `collaboration-editing.ts`, `collaboration-control-gaps.ts`, `collaboration-retained-attachments.ts`, `collaboration-delivery-web.spec.ts`, `collaboration-boundaries-web.spec.ts` and the boundary-presence assertion in `collaboration-source-surface.spec.ts` only where triage identifies them.
- In `collaboration-mention-journeys.ts` and `collaboration-retained-attachments.ts`, validate the exact `author_authority.evaluated_at` timestamp separately, then compare every remaining field exactly. No recursive field stripping or broad partial comparison.
- In `collaboration-records.ts`, scope record-body paging assertions to `li[data-record-id]`; the current helper has no direct first/next HTTP assertions, so add narrow first-page/actual load-more response item and cursor checks. Keep the load-more action, notification/child reference semantics and revocation assertions. An unexpected result in the actual first page remains a product failure.
- A small canonical ENV-1 addition beside `collaboration-runtime-handoff-web.spec.ts` is a possible final scope item, but requires the root's password services permission and an acknowledged source-bound fixture setup first.

No changes to `apps/`, `packages/`, `server/`, timeout budgets, automatic retries, production API/CORS, authorization, dependencies or deployment scripts. No direct creation POST may replace the UI behavior being tested. Existing business assertions remain in place after the changed interaction/scoping.

## Verification sequence

1. Read the 29-hash triage binding and each exact current UI/API contract; record owned files and expected assertions.
2. Wait for root's READY services permission. Verify actual API/Web source and owned database before any selected canonical execution; do not discover or change other services.
3. Run one representative unmodified canonical case per category with worker1/retries0 and the existing timeouts. Save exact commands, test-source hash, API/Web source, exit, named pass/fail/skip, failure and evidence hashes. A setup failure is not a product red.
4. Make one contract-focused repair pass per category. Use current picker interaction only, preserve exact resolver 400/404/no-invite/no-notification checks, stable authority/business facts and first/next records response facts.
5. Run all affected 14 cases serially against their permitted production services. The source-surface and three retained-attachment cases explicitly require password auth; the legacy lane alone cannot cover the whole inventory. Preserve any failure after the initial interaction and report actual product defects rather than weakening later assertions.
6. Run existing static TypeScript/lint/collection checks appropriate to the E2E files. Root owns independent review, local integration and final same-candidate full acceptance; this branch's targeted results do not close those gates.
