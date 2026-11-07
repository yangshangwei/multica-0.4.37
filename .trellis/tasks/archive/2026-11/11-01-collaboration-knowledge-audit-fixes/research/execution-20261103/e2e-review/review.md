# v12 E2E maintenance independent review

Verdict: **pass-with-notes**, zero blockers. Reviewed immutable base `b6c74033f1913e46418ec1b119da33236dcba805`, tracked E2E diff SHA-256 `a3f24cb7a1ed0f8e66d37be126d94faca524ec9294c3c31d38b5ba9d54647d98` and the new disclosure helper. This is source/evidence review, not final runtime clearance.

## Findings

No source issue requiring correction was found. No source file was edited.

The current production detail uses native details for Attachments, Sub-issues, Knowledge candidates and AI execution; workflow places remaining permitted actions under More actions. The helper clicks those actual visible summaries and waits for the open attribute. It adds no forced click, sleep, DOM-open mutation, retry, skip or broader timeout.

Original knowledge recovery after reload and run/comment anchor behavior are still asserted without manual reveal assistance. Approval/version/expiry/purge, file bytes/IPC, resource consent and source/privacy checks remain. External source/iteration absence now includes hidden controls, and outside native child privacy first proves the child title is visible.

Ordinary discussion now invokes the real single-action validated preflight/send path. Both helper callers use ordinary text. Explicit AI targeting still requires confirmation in production and its existing component regression is unchanged. Source navigation now waits for the canonical identifier after verifying the returned UUID.

## Evidence reuse

- Current bytes match all eight files in disclosure-replay-input.json. The retained25-case run passed with zero skip/retry/fail.
- Editing helper and Web editing spec match directed3. Desktop spec changed afterward in the reviewed disclosure increment and its journey passed in the25 run; its current bytes must not be labelled the directed3 snapshot.
- Directed3 and25 share two test identities; they are separate targeted results, not28 unique cases or a whole gate.
- No lint, typecheck or tests were re-run in this read-only pass. The final complete check and source/runtime audit remain required. The disclosure type union had one harmless additional name after its reported tsc; final full checks cover the current bytes.
- Any later lifecycle-lane integration or source change is outside this exact review and requires rebinding/review.

## Files to commit before freezing

- `e2e/collaboration-account-desktop.spec.ts`
- `e2e/collaboration-delivery-web.spec.ts`
- `e2e/collaboration-editing-web.spec.ts`
- `e2e/collaboration-knowledge.spec.ts`
- `e2e/collaboration-records-web.spec.ts`
- `e2e/collaboration-resources-web.spec.ts`
- `e2e/fixtures/collaboration-disclosures.ts`
- `e2e/fixtures/collaboration-editing.ts`
- `e2e/fixtures/collaboration-knowledge.ts`
- `e2e/fixtures/collaboration-resource-offers.ts`

Only unrelated dirty source is `apps/web/next-env.d.ts`: the exact generated `.next/dev/types/routes.d.ts` to `.next/types/routes.d.ts` import rewrite, with identical remaining bytes. Exclude that generated-only delta from the E2E commit; permit only the documented exact-line normalization in source identity.

All hashes, source references, unchanged assertions and evidence scope are recorded in `review.json`.
