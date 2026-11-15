# Recipient acceptance test extension

Changed source: `e2e/collaboration-recipient-picker-web.spec.ts` only. The existing canonical test name and self/recent/search, limited account projection, email privacy, Escape, narrow viewport and screenshot assertions remain.

The extension fills a valid issue title, confirms the default self UUID, and observes browser create requests. Starting from the self keyboard option, it holds the real API response for a fresh unique name substring. Search is name-only on the server; UUID is used to verify account identity, not as a search query. Enter while that response is held must leave the popup/search open, the empty list attached and busy, the default recipient unchanged, and create-request count zero. An empty list can have zero height, so visibility belongs to the popup/search rather than the list itself.

Releasing the hold passes the original response through `route.fulfill({ response })` unchanged. The test checks its unique recipient UUID/name, selects the exact accessible account option, and verifies the chosen UUID and assignment status. Only an explicit Create click may POST. The 201 response and authenticated reads by both source and recipient must agree on issue ID, source workspace, creator, title and intended handler; self assignment and duplicate browser creates fail. Create click/response promises are observed together. The route is released and completed before unregistering, including failure cleanup.

No new harness or dependencies were added; existing `TestApiClient` accounts and persisted recent-history setup are reused. No product code, translations, task-wide ledger, or another owner's dirty E2E files were edited.

## Static evidence

- `recipient-static-parse.json`: installed TypeScript 5.9.3 syntax parse, zero diagnostics, one unchanged canonical test name. This does not execute the spec or establish semantic type checking.
- `git diff --check -- e2e/collaboration-recipient-picker-web.spec.ts`: passed.
- `recipient-test.patch` and `recipient-test-artifacts.json`: independent patch, source/base fingerprints and artifact hashes for the coordinator's scoped integration.
- Browser, API/database, build, lint and typecheck execution: **NOT_RUN** in this lane because the shared host window remains owned by the concurrent campaign. Real-model acceptance remains **NOT_RUN**, separately deferred.

## Next exact verification

After the coordinator grants the execution window, bind a fresh isolated legacy-auth API/database and matching production Web build to the current candidate plus this patch. Verify actual database identity, API/Web process ownership, build/source hashes and rendered API/WS origins; collaboration creation must be enabled. Preserve the frozen core campaign's database and results.

In that verified child environment, use a new output directory and run once:

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE="$RECIPIENT_ACCEPTANCE_RUN/playwright.json" \
  pnpm exec playwright test e2e/collaboration-recipient-picker-web.spec.ts \
  --project=chromium --workers=1 --retries=0 --trace=on \
  --reporter=line,json --output="$RECIPIENT_ACCEPTANCE_RUN/test-results"
```

Require the one exact named test to execute without skips. Retain the first result, trace and `recipient-picker-narrow.png`; any correction gets a separately recorded run. Static review is preparation, not acceptance of this browser journey.

## Delivery locator proposal only

`delivery-attachment-locator-proposal.patch` scopes the whole-issue Attachments region by its exact Attachments heading. The new comment attachment region has no such heading. This preserves all preview/download/revocation assertions and does not use `.first()`. Per the coordinator's final scope, `e2e/collaboration-delivery-web.spec.ts` is unchanged; the proposal needs its own authorized source handoff and browser verification.
