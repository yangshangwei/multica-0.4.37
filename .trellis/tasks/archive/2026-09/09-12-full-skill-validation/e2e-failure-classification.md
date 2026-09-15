# E2E failure classification

Classifies the 10 remaining failures and 2 collection-blocked cases recorded in
`verification.md` for tested commit `fe38a164b`.

Result: **0 product bugs.** 10 items were stale harness, 2 were run-environment
artifacts of the dev-mode web server. Every item now passes.

## A. Stale harness (10 items)

The test asserted an affordance or contract the product had already replaced.
Each was repaired test-side; all five repair commits touch exactly one `e2e/*.spec.ts`
file and no product source, which is what makes them harness defects rather than fixes.

| Case | Original error | Why the harness was stale | Repaired by |
| --- | --- | --- | --- |
| `agent-mcp.spec.ts:153` creator sees the MCP Apps tab | `getByRole('button', {name:'MCP Apps'})` not found | MCP Apps became a `role=tab` nested under the Capabilities tab, and it is gated by the `composio_mcp_apps` flag, which is **false by default** (`server/internal/handler/config_test.go:432`). The test neither opened Capabilities nor enabled the flag. | `bfaabf6f7` |
| `agent-mcp.spec.ts:177` non-creator does not see the tab | `getByRole('button', {name:'Activity'})` not found | Same tab-role change; the control case now asserts through Capabilities → Skills. | `bfaabf6f7` |
| `chat-attachments.spec.ts:74` upload binds to chat_session | `expect(status).toBe(200)` got `404` | The fixture seeded `chat_session` with raw SQL. Raw internal sessions deliberately reject uploads; only sessions created through `POST /api/chat/sessions` are member-visible. Test fixture defect, intended product behavior. | `99b266707` |
| `issue-table.spec.ts:86` groups 1,001 issues | `getByText(/Loaded \d+ of 1001/)` not found | The footer counter was replaced by a muted `No more` marker (`packages/views/issues/components/list-load-more-footer.tsx:65`). The repair also filters Board's `hierarchy.enabled=false` requests, which the old test miscounted as Table materialization. | `4d2804fef` |
| `issue-table.spec.ts:175` nested vs cross-group children | `expect(todoRoot.total).toBe(3)` got `0` | Query-wide totals moved to `/api/issues/table/groups`; branch pages intentionally skip the count. The test asserted an obsolete response contract. | `4d2804fef` |
| `issue-table.spec.ts:269` drops stale branch cursors | `getByText('Loaded 60 of 60')` not found | Same footer copy change as `:86`. | `4d2804fef` |
| `settings.spec.ts:5` workspace rename | `locator('button', {hasText:'Save'})` never appeared | Workspace text settings auto-save on blur; there is no Save button. | `eb8baaf3f` |
| `settings.spec.ts:46` Composio connect toast | `waitForPageText` timeout | Needed the `composio_mcp_apps` flag mocked on, plus `role=tab` / heading / toast-region locators for the current settings shell. | `eb8baaf3f` |
| `plugin-surface-document.spec.ts` (2 cases, collection-blocked) | `does not provide an export named 'buildSurfaceDocument'` | The export is `buildSurfaceFrameDocument`; the old late-listener retry/ack contract was also replaced, so the tests were rewritten rather than re-pointed. | `0b3ae5e80` |

## B. Run environment, not the product (2 items)

| Case | Original error | Evidence |
| --- | --- | --- |
| `comments.spec.ts:25` add a comment | 60 s test timeout inside `editor.fill` | The visibility assertion one line earlier passed, so the editor existed. The 60 s budget was consumed before the fill. |
| `navigation.spec.ts:12` sidebar navigation | URL stayed on `/issues`, 33 polls over 30 s | `/[workspaceSlug]/inbox` had only begun compiling behind five other route compiles; the same route served in 3.5 s and 908 ms once the web process was restarted. The retry then failed one assertion later, on the next cold route. |

Root cause for both: the run used `next dev --webpack`
(`e2e/web.log`: `@multica/web@0.4.40 dev`). Cold compiles measured **35.8 s**
(`/onboarding`), **31.0 s**, **30.8 s** and **30.4 s** (`/agents/[id]`) against
test budgets of 15 s, 30 s and 60 s. Neither spec file has been modified since.

## Proof that all 12 now pass

`.omx/reports/main-upstream-merge-20260913/e2e-summary.json` — commit `23d771ca7`,
started 2026-09-13T09:57:39Z, **82 expected, 0 unexpected, 0 skipped, 0 flaky**,
24 spec files. All 10 previously failing cases and both previously blocked plugin
cases are `expected` in that run. It began at 17:57 Beijing, after the five
repair commits landed at 10:20 Beijing the same day.

That run used a production web server: `pnpm --filter @multica/web build` followed
by `next start --port 14051` (`web.running.json`), which is also why the
dev-compile timeouts did not reappear.

Case-count drift across runs (69 → 77 → 82 → 85) is new spec files landing after
the tested commit, not cases disappearing.

## Limits of this classification

- The green evidence is at `23d771ca7`, not at current `HEAD`. The suite has not
  been re-run at `HEAD` as part of this classification.
- Session 16 of the developer journal records 85/85 web + Electron E2E on
  2026-09-15, after `23d771ca7`. That report directory was not located or
  verified here; it is corroboration, not proof.

## Follow-up worth keeping

E2E must run against `next build` + `next start`. A `next dev` run makes route
compilation, not the product, decide whether navigation and editor tests pass.

Recorded as a spec rule: `.trellis/spec/web/frontend/e2e-run-environment.md`, with a
pre-run checklist entry in `.trellis/spec/guides/index.md`.
