# Recipient picker static review

Verdict: **PASS — static review only**. No remaining blocking finding in the reviewed recipient test. E2E execution, lint and type-check are **NOT_RUN**; this report does not certify a passing product journey or complete acceptance.

Reviewed on 2026-11-05 against `acceptance-plan.md`, `scope.md`, `handoff.md`, the active task requirements and injected test-environment specs. The later acceptance plan/handoff explicitly assign the recipient file to this support lane. Review scope is only `e2e/collaboration-recipient-picker-web.spec.ts` and this research folder. Other dirty E2E files and the frozen core campaign were not reviewed or changed.

Reviewed file SHA-256: `4975f7e6198cac880216138b71b0bdfa0099c636f20416e808caae21de0b12a4`.

## Findings (fixed)

- File: `e2e/collaboration-recipient-picker-web.spec.ts:165`. The pending listbox has no matching rows and can have zero height; requiring it to be visible is not a valid open-picker assertion. The writer applied the parent's correction: require visible popup/search, an attached busy listbox, the outer dialog, and no selectable options. The identity and no-write assertions remain.
- File: `e2e/collaboration-recipient-picker-web.spec.ts:139`. Navigation preparation now sends ArrowDown only when there is no active descendant, then still requires the exact self option ID. `PropertyPicker` resets navigation on close, and `collaborationRecipientSections` puts self first; the condition cannot accept a different highlighted account.
- File: `e2e/collaboration-recipient-picker-web.spec.ts:177`. The current-response waiter is now created immediately before synchronous release and directly awaited. Failures in the earlier pending-state assertions cannot strand this waiter.
- File: `e2e/collaboration-recipient-picker-web.spec.ts:201`. The new create response promise was previously left unobserved if the button click failed. The writer applied the review recommendation to observe the response and click together with `Promise.all`.

The reviewer made no source edits; these corrections were coordinated with the parent and source owner. This report is the reviewer's only written file.

## Verified by code inspection

- All original self/recent/search, account-field privacy, Escape/outer-dialog, narrow-viewport geometry and screenshot assertions remain. The original named test, timeout and prerequisites remain; no retries or thresholds were relaxed.
- `packages/core/collaboration/recipient-search.ts` keys queries by context, trimmed query and members-only state, with retries disabled. The held `collaborator ${suffix}` query differs from both the earlier full name and empty query. It therefore exercises an uncached search for this fixture.
- The route handler fetches the real API response, marks it held only after fetch resolves, waits for explicit release, and fulfills with that response unchanged. The test requires pending state before and after Enter, unchanged self identity, no active descendant, no enabled option and zero browser creation POSTs. The later response predicate binds the exact current query and POST endpoint, then its body must contain the exact second account's UUID/name/external identity.
- The route `finally` releases first, waits for delivery, then unregisters in a nested `finally`, including when delivery rejects. It does not unregister a healthy held request before fulfillment. The current-response promise is not armed during the assertion-only pending interval.
- `server/pkg/db/queries/collaboration_recipients.sql:51` searches names only. The chosen substring matches the seeded recent account's name; the UUID is used to identify the displayed option and submitted handler, not as a presumed directory search input. The persisted history makes this account eligible in related-account mode as well as directory mode. The separate account has no membership in the source workspace, consistent with `External`.
- `packages/views/collaboration/account-picker.tsx` hides old query pages while pending, disables remaining local rows, and supplies the exact option labels asserted here. `PropertyPicker` resets active identity when candidates disappear. The corrected empty-list assertions follow that DOM structure.
- The browser POST observer starts after API-only history setup and before picker interaction. It records only `POST /api/collaboration/issues`, so the recipient-search POST is not mistaken for creation. Zero is checked after pending Enter and recipient selection; exactly one is required after explicit creation and reads.
- `creation-form.tsx` stores the selected account in the draft; `packages/core/collaboration/creation.ts` maps its UUID to `handler_user_id`; the API client sends the creation request. `server/internal/handler/collaboration_creation.go` passes that UUID through validated input to the service and returns 201. The response projection reads `handler.user_id` from the persisted assignee. The test checks request, response, navigation/title, and fresh GETs using both independently authenticated clients. This covers the intended write/read assertions without supplying a mocked creation response.

## Findings (not fixed) and limits

No unresolved blocking source finding. These limits remain explicit:

- Nothing in this review executes the test, collects its Playwright inventory, checks types/lint, starts a browser/service, or connects to a database. Existing unit-test source was inspected but not run. Runtime success, timing and browser geometry remain unproved.
- On fetch/fulfill transport failure, delivery rejects and the test fails; route removal is attempted in `finally`. A failed outstanding browser request still relies on Playwright page/context disposal. A cleanup rejection can become the surfaced error. This static review does not claim experimentally verified fault cleanup or preservation of every concurrent error.
- The existing test retains its synthetic users, source workspace and history issue; the extension also retains the created issue. `requestJSON` does not register these for `TestApiClient.cleanup`, and the file has no database teardown. Do not add a generic workspace delete that leaves collaboration relationships behind. An owned fresh database and coordinator-owned retention/disposal are required, including on failure.
- The two account GETs establish intended API persistence/access assertions, not a recipient browser/realtime/native observation. Pagination, duplicate-name ambiguity, UUID exact-lookup behavior, revocation, provider/model use and installed upgrades are outside this focused increment.
- No product contract or package boundary changed, so no `.trellis/spec/` update is required for this test-only addition. Unrelated fixture repairs remain proposals owned by their respective lanes.

## Verification

- Static source/diff and contract review: PASS for the file hash above.
- Lint: NOT_RUN — explicit shared-host/static-only restriction. An ignored root E2E file must not later be counted as linted merely because an unrelated workspace lint exits zero.
- TypeCheck: NOT_RUN — same restriction; parsing alone is not a type-check.
- Tests/Playwright collection/browser/database/build: NOT_RUN — same restriction.
- Native/Desktop, administrator, commercial-model acceptance: NOT_RUN in this support lane.

## Required future execution

1. Obtain explicit shared-host handoff from the core owner and coordinate the queued knowledge window. An estimate or idle listener is not permission. Preserve the frozen core results, first failures and performance gate.
2. Bind the exact recipient-file hash and current product source to production Web/API artifacts; verify actual database, listener/PID, build ID and rendered API/WS origins before fixtures. Use a fresh owned legacy-auth database with collaboration creation enabled; the two `TestApiClient.login` calls require that auth lane. Keep model/agent execution out of this run.
3. After handoff, run applicable fixture static checks with real file coverage, then the exact canonical test with one worker and zero retries. Retain named terminal status, first failures, traces and screenshots. A skipped prerequisite does not count as a pass.
4. Confirm the hold/pending Enter/no-POST/explicit-create/both-account-read assertions actually run. Preserve transport or teardown failures separately; record final source identity and owned resource/data retention or cleanup. Use a separately numbered run after any correction.
5. Execute the separately planned real-admin Web/Desktop lanes with their own prerequisites and databases. A successful recipient Web case cannot certify those lanes or installed Main.
