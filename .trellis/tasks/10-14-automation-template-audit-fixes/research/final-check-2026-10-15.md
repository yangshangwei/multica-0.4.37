# Final code check — automation template audit fixes

Date: 2026-10-15. Reviewer: dispatched Trellis check agent.

Reviewed implementation commit `e5fe7b909c6d3987fed1a06d033edb5dd17bd3e8` against the task PRD, design, implementation plan, injected check context, and current source at HEAD `6c24ae8c6df78b01088a95d04fe20e51381e98fd`.

The entire implementation diff was reviewed. At the baseline verification snapshot, all affected product files remained identical to that implementation commit, with no uncommitted changes. Subsequent native evidence exposed the cross-route A02 gap below. The bounded guard and staged regression now pass independent source review, post-fix checks, fresh native interactions, and final visual review. The final verdict is **96/100, pass**, with no actionable A01–A07 visual mismatch. The parent owns task state and closure.

Reviewed post-fix file blobs (uncommitted at this review):

- `packages/views/autopilots/components/template-create-autopilot-page.tsx`: `c1057fccf57f400a5938747b84acefefd3889bef`.
- `packages/views/autopilots/components/template-create-autopilot-page.test.tsx`: `e43f02a1565990d43b91de768e97d159c2ebd8b2`.

HEAD later advanced through the user's separate runtime-task/journal work to `d8228f82509a75af0bcb7fe9c389c9e6ab58a36a`. The reviewed automation paths have no intervening committed changes, and both post-fix blobs above are unchanged. The native build records that later base HEAD plus the uncommitted guard/test patch.

## Findings (fixed)

### P2 / A02 — outgoing creation page consumed the destination's focus handoff

- File: `packages/views/autopilots/components/template-create-autopilot-page.tsx:110`; the consuming ref is `autopilot-template-catalog.tsx:97`–`:113`. Regression: `template-create-autopilot-page.test.tsx:329`.
- Native reproduction: empty automation list → maintenance → documentation-check → Enter → native keyboard Back. The first restored card belongs to a temporary picker inside the outgoing creation shell. React then deletes that route's tree and focus returns to body.
- Pre-fix evidence: `evidence/final-native/focus-route-order.json` records focusin with headings `新建自动化` / `从模板开始`, followed by `connectedAfter100: false` and `activeAfter100: false`. The same-path picker return remains connected and focused. `evidence/final-native/focus-blur-stack.json` records React `commitDeletionEffectsOnFiber`; no explicit blur/focus-scope callback was found in this path. The earlier native matrix, screenshots and runtime/build provenance are retained under `evidence/final-native/pre-fix/`; they must not be mistaken for post-fix evidence.
- Cause: Desktop `DesktopNavigationProvider` subscribes to the tab store's live URL, while `RouterProvider` commits its route projection in a React transition. After Back updates the store, the still-mounted `TemplateCreateAutopilotRoute` sees the destination URL with no `template` parameter and renders its picker. The restoration adapter already resolves the destination `/autopilots` route, so that temporary picker consumes its pending token. The final `AutopilotsPage` catalog mounts without the token after the intermediate card is deleted.
- Why existing tests missed it: `renderPage` chooses the mounted page directly from `route.pathname`, so navigation state and rendered-route replacement occur together. It never renders the outgoing creation page once with the destination navigation context.
- Implemented fix: after all hooks in `TemplateCreateAutopilotPage`, compare the normalized navigation pathname with the template pathname and return `null` when leaving that route. The outgoing page never mounts a temporary picker, so the destination token remains untouched until the destination mounts. The existing same-path shell key and focus-owner guard are preserved.
- Fix boundary: a page-local guard fits this dedicated workspace route and A02. Changing Desktop's global navigation provider to rendered-location ownership would alter its contract for sidebar, overlays and other consumers; it is a broader platform change without a need in this task.
- Compatibility review: the guard strips trailing slash(es) and compares case-insensitively. `sanitizeTabPath` preserves multi-segment paths unchanged, and the installed React Router's `matchRoutes` matches `/acme/autopilots/new/template`, `/acme/autopilots/new/template/`, and `/acme/AUTOPILOTS/NEW/TEMPLATE` to this route (verified with a read-only Node probe). Both noncanonical aliases have passing reachability tests. The trusted path builder returns the canonical path; query strings and hashes are already separate in the navigation contract.
- Hook review: `useT`, `useWorkspaceId`, `useWorkspacePaths`, `useNavigation`, `useBackOrReplace`, and `useAutopilotTemplates` execute before the guard on every render. The guard does not change hook order or add state, effects, timers, interfaces, or platform imports.

The fix was made by the parent's implementation agent and independently reviewed here. This reviewer made no product/test edits, writing only this report and `research/visual-verdict-final-2026-10-15.json`. The existing catalog reuse, removed duplicate guidance, title-group recommendation, and route/tab view-state reuse remain appropriate simplifications.

## Findings (not fixed)

No unresolved source-level or scoped visual finding remains. The original P2 defect is fixed in source, covered by a red/green regression, and confirmed resolved by the parent's fresh Electron interaction evidence, independently read below.

The final post-fix visual matrix and detector have been read, and the representative screenshot comparison is complete. The parent owns persisting the final state, spec note and task checklist/journal closure.

All pnpm invocations emitted the existing configuration warning that `pnpm.onlyBuiltDependencies` and `pnpm.overrides` in root `package.json` are ignored by the installed pnpm. Every command exited successfully; no installation or dependency changes were made. This unrelated package-manager configuration was left unchanged.

## Scope and acceptance coverage

| Requirement | Code and regression evidence | Native / visual acceptance |
| --- | --- | --- |
| A01 — focus contrast | `autopilot-template-catalog.tsx:38` defines a local solid foreground ring with an explicit background offset. Cards, group controls, blank creation, and Retry use it. Shared theme tokens and Button defaults are unchanged. | Verified: the light ring is visibly clear; measured adjacent-surface contrast is at least 19.23:1 in light and 16.97:1 in dark, above 3:1. |
| A02 — return focus | Catalog writes group, scroll and template key before selection; its ready-content ref restores scroll first, consumes focus once, preserves a newer active control/dialog, and focuses the card with `preventScroll`. Missing cards fall back to the active filter, then the blank action for an empty registry. Catalog tests cover delayed data, explicit All/zero, removed cards/groups, empty registry, newer focus owners, refetch and remount. The new outgoing-page guard and staged regression close the identified cross-route gap. | Verified: 8/8 post-fix native/in-flow Back cases preserve the final connected card, group and scroll across both origins and languages. |
| A02 — creation lifetime | `template-create-autopilot-page.tsx:118` keys the shell by picker/configure phase so a retained Back control cannot suppress the return handoff. At `:411`, creation must provide a nonempty parsed ID before the origin focus key is cleared at `:415`. Both origins and success/rejected/unreadable outcomes are covered with route-scoped state; successful fresh visits retain group/scroll and no stale focus. | Real return ordering is verified; creation success/failure lifecycle remains covered by the canonical tests without executing real automations. |
| A03 — recommendation | Recommendation is inside the icon/title group at catalog `:236`; the former dedicated footer row is removed. Backend order and the three/two/one-column grid remain intact. | Verified in both languages: recommendation sits directly under the title. Chinese wide first-row cards are 196px versus 194px for following rows, replacing the baseline 222px first-row penalty. |
| A04 — defaults and timezone | Both locale descriptions state that an executor and timezone are chosen next and that default schedules/instructions can be edited after creation. The description appears before filters and cards at catalog `:142`. Configuration still previews server-owned schedule/output/prompt, and its create request excludes these fields. | Verified readable at wide/narrow sizes and native 200% zoom. All 16 matrix cases place the introduction before the first card. |
| A05 — truthful output | Known patrols say “Creates or updates issues as needed” / “按需创建或补充任务”; `create_issue` says “Creates an issue each run” / “每次运行创建任务”. The five `PATROL_KEYS` match the current backend roster and prompts. Unknown `run_only` keys keep generic copy; unknown modes do not acquire a guaranteed outcome. Existing picker and core schema tests cover forward-compatible payloads. | Verified bilingual labels remain readable; longer English footers wrap naturally without clipping. |
| A06 — prerequisite placement | Executor, timezone and editability facts share the introduction; the old footer hint and locale key were removed. No extra explanatory panel or repeated footer was introduced. | Verified as one concise paragraph above filters/cards, with no distant duplicate footer. |
| A07 — blank creation | The empty list passes its existing dialog opener to the catalog and suppresses its duplicate header action. Populated management keeps New automation. The shared blank action retains visible text and `min-h-11`, with compact/coarse input preserved. The list test proves exactly one action inside the gallery opens the existing dialog. | Verified in 4 synthesized-touch cases: both origins at widths 1771/390, blank and filter heights 44px, dialog open and focus return on dismiss. |

The unchanged list path still derives first use from the unfiltered automation response and excludes loading/errors. Existing list tests cover populated, filtered-out, initial loading and failed states. Existing template tests preserve project/assignee defaults, confirmed creation, permission checks and user edits across refetch.

## Adapter and boundary review

- Shared `packages/views/platform/scroll-restoration.tsx:89` and `:176` expose only the approved optional `routePathname` argument. The writer forwards it, and scroll-only adapters remain no-ops.
- Web `apps/web/platform/scroll-restoration.tsx:110` targets `routePathname ?? pathname`, where `pathname` is Next's rendered pathname. Existing read/default-write behavior is retained. The provider test proves explicit-origin clearing preserves other entries and current-route state.
- Desktop `apps/desktop/src/renderer/src/platform/tab-coordinator.ts:148` uses an explicit route before consulting the live-active-tab default. It still commits to the adapter-bound `tabId`. Both active and inactive owning-tab tests prove other routes and tabs remain unchanged; ordinary inactive-tab reads/writes retain their guard.
- The underlying Desktop `commitViewState` searches the owning tab, clears only one entry, and skips absent/no-op writes. No tab-store, navigation contract, URL parameter, API/schema, backend, dependency, or generated platform configuration changes were introduced by this task.
- The creation mutation awaits the existing parsed API response; `EMPTY_CREATE_AUTOPILOT_FROM_TEMPLATE_RESPONSE` has an empty ID and cannot clear the handoff. The existing canonical schema suite was run because this boundary supplies the guard's failure marker.
- The committed `.trellis/spec/views/frontend/automation-template-discovery.md` already documents the optional writer, lifecycle, fallback/focus-owner rules, owning-tab behavior, and canonical tests. Add the confirmed outgoing-route/staged-commit invariant during the bounded A02 fix and attach final native evidence during closure.

## Staged navigation regression review

The new regression lives in `packages/views/autopilots/components/template-create-autopilot-page.test.tsx`, beside the existing page-level return tests. Its optional `mountedPathname` argument decouples the rendered route from the mocked navigation `route.pathname`; existing tests retain their default behavior.

1. It mounts the real empty-list catalog, chooses maintenance, scrolls to 142, selects the hourly queue card with Enter, and mounts the real configuration page.
2. It focuses and activates configuration Back, checking the origin replace call.
3. It changes navigation pathname to `/acme/autopilots` and clears search parameters while deliberately retaining `TemplateCreateAutopilotRoute` for one render.
4. It asserts the origin's pending-focus entry remains present and no temporary gallery mounts. The red run failed here with `Expected: "hourly-queue-check"; Received: undefined`, directly reproducing the premature consumption.
5. It commits the rendered route to the final empty-list catalog and verifies the original card has focus, maintenance remains selected, scroll is 142, and the token is consumed.
6. Two new alias tests keep the picker reachable with a trailing slash and uppercase route segments. Existing same-path native/in-flow Back, refetch/focus-owner and successful-create tests remain and passed in the 41-test post-fix run.

This tests two distinct commits without an arbitrary timer. The staged condition applies to both navigation entry methods; the parent's subsequent Electron recheck confirms both actual return paths.

## Post-fix native interaction evidence

Independently read `evidence/final-native/interactions.json`, `build-provenance.json`, and `views-regression-output.txt` after the parent ran the new build. These are post-fix evidence; `pre-fix/` remains the historical baseline.

| Native check | Verified result |
| --- | --- |
| Return to gallery | 8/8: Chinese/English × empty-list/creation-gallery × in-flow/native-keyboard. Before/after group, scroll and card title agree; every `sameConnectedNodeAfter100ms` is true. |
| Native toolbar owns focus | 4/4 retain toolbar focus and consume the old pending handoff. |
| Refetch after deliberate focus | 4/4 retain the newer focus owner. |
| Coarse/synthesized touch | 4/4: both origins at 1771 and 390 widths; blank and filters measure 44px; blank dialog opens and dismissing restores focus. Chromium `Input.dispatchTouchEvent` was used, not a physical touch device. |
| Runtime | Electron 39.8.7 / Chromium 142.0.7444.265, sandbox/context isolation/web security enabled. Page errors, failed requests and response errors are empty; daemon starts, installs, external links and blocked writes remain 0. |

Build provenance records `sourceCommit=d8228f82509a75af0bcb7fe9c389c9e6ab58a36a`, `sourceHasUncommittedFix=true`, patch SHA-256 `7e9bfc7d0874908cf316052f7c097ebf94937710824c11c38b83c50877c37040`, and build exit 0. A reviewer-side read-only hash comparison checked all **14 recorded source files** against current bytes and all **15 served artifacts** against the build manifest: **0 source mismatches, 0 artifact mismatches**. This binds the passing interaction evidence to the reviewed fix rather than the earlier build.

The final post-fix matrix contains all 16 cases: Chinese/English × light/dark × 1771px wide, 886px compact, 390px narrow and native 200% zoom. Every case reports no catalog/card horizontal overflow, the introduction before cards, and the expected three/two/one-column layout. Native zoom cases explicitly record `nativeZoom: 2` with an approximately 885 × 540 CSS viewport. The post-change impeccable detector result is `[]`.

## Final visual review

Applied `/Users/artisan/.codex/skills/visual-verdict/SKILL.md`, using local image viewing only. Verdict artifact: `research/visual-verdict-final-2026-10-15.json` — **score 96, pass, category_match true, no differences requiring revision, no suggested product edits**.

Baseline images were `.impeccable/audit/2026-10-14-automation/focus-light.png` and `narrow-light.png`. The following final images were visually inspected under `evidence/final-native/`:

| Coverage | Images viewed |
| --- | --- |
| Wide focus / dark English layout | `zh-light-wide-keyboard-focus.png`, `en-dark-wide.png` |
| Narrow bilingual layouts | `en-light-narrow.png`, `zh-dark-narrow.png` |
| Native zoom / compact layout | `en-dark-zoom200.png`, `zh-light-compact.png` |
| Final restored card focus | `zh-empty-list-native-keyboard-return.png`, `en-creation-gallery-native-keyboard-return.png` |
| Settled narrow blank dialogs | `zh-empty-list-blank-touch-390.png`, `zh-creation-gallery-blank-touch-390.png` |

The final catalog keeps the incumbent typography, semantic category colors, whole-card structure and spacing system. Focus is clearly distinguishable; recommendation and setup guidance now appear where users choose templates; narrow blank creation remains labelled and large enough to find. The longer English text wraps without hiding actions or crossing card boundaries. Native 200% layout remains readable and usable. Both returned-card screenshots show the original documentation card focused in the final destination.

The two narrow dialogs were inspected only after the parent's recapture. All four touch records now state `captureAfterOpeningAnimation: true` and `dialogOpacity: "1"`; the viewed dialogs are opaque, readable and retain visible dismiss/create controls. No conclusion was drawn from the discarded animation-frame captures.

Computed evidence corroborates the visual inspection: minimum focus contrast is 16.97:1 across themes/surfaces, body contrast is at least 5.71:1, and narrow/coarse blank targets are 44px high. Fine-pointer wide/compact targets retain their intentional 36px height. No broader cosmetic change is warranted for the approved A01–A07 scope.

## Verification

The baseline commands below were run by this reviewer from `/Volumes/artisan/code/2026/multica-0.4.37` before the staged-route gap was identified. The post-fix commands were assigned to and run by `automation_focus_fix`; their existing tool results were obtained for this report rather than duplicated. No builds, servers, browser windows or native sessions were started by this reviewer.

### Post-fix red/green and quality results

Evidence source: the implementation agent's existing tool transcript and explicit result handoff; it did not write separate command logs. The reviewer independently inspected both changed files and their final blobs above.

| Exact command | Phase / exit | Result |
| --- | --- | --- |
| `pnpm --filter @multica/views test autopilots/components/template-create-autopilot-page.test.tsx -t 'preserves the empty gallery focus handoff'` | New regression before guard; exit 1, expected | 1 failed, 32 skipped; 1 failed file; 1.72s. At test `:356:44`, after `rerenderPage({}, templatePath)`, `expect(viewState.get(pendingFocusKey)).toBe("hourly-queue-check")` received `undefined`. |
| `pnpm --filter @multica/views test autopilots/components/autopilot-template-catalog.test.tsx autopilots/components/template-create-autopilot-page.test.tsx` | After guard; exit 0 | 2 files, 41 tests passed; 4.49s. |
| `pnpm --filter @multica/views test autopilots/components/autopilot-template-catalog.test.tsx autopilots/components/template-create-autopilot-page.test.tsx autopilots/components/autopilots-page.test.tsx autopilots/components/autopilot-dialog-i18n.test.ts locales/parity.test.ts platform/scroll-restoration.test.tsx` | Parent's final post-fix regression; exit 0 | 6 files, 123 tests passed; 3.88s. Persisted in `evidence/final-native/views-regression-output.txt`, independently read by reviewer. |
| `pnpm --filter @multica/views exec eslint autopilots/components/template-create-autopilot-page.tsx autopilots/components/template-create-autopilot-page.test.tsx` | After guard; exit 0 | No ESLint diagnostics. |
| `pnpm --filter @multica/views typecheck` | After guard; exit 0 | `tsc --noEmit`, no TypeScript diagnostics. |
| `git diff --check -- packages/views/autopilots/components/template-create-autopilot-page.tsx packages/views/autopilots/components/template-create-autopilot-page.test.tsx` | After guard; exit 0 | No whitespace errors. |

The 41-test and 123-test post-fix runs overlap the baseline below; their counts must not be added together.

### Baseline tests

| Exact command | Exit | Result |
| --- | ---: | --- |
| `pnpm --filter @multica/views test autopilots/components/autopilot-template-catalog.test.tsx autopilots/components/template-create-autopilot-page.test.tsx autopilots/components/autopilots-page.test.tsx autopilots/components/autopilot-dialog-i18n.test.ts locales/parity.test.ts platform/scroll-restoration.test.tsx` | 0 | 6 files passed; 120 tests passed; duration 4.37s. |
| `pnpm --filter @multica/web test platform/scroll-restoration.test.tsx` | 0 | 1 file passed; 6 tests passed; duration 1.58s. |
| `pnpm --filter @multica/desktop test src/renderer/src/platform/tab-coordinator.test.ts` | 0 | 1 file passed; 6 tests passed; duration 7.31s. |
| `pnpm --filter @multica/core test api/autopilot-template-schemas.test.ts` | 0 | 1 file passed; 8 tests passed; duration 200ms. |

Total: **9 files, 140 tests passed, 0 failed**.

### Baseline lint

Each command exited 0 with no ESLint diagnostics:

```sh
pnpm --filter @multica/views exec eslint autopilots/components/autopilot-template-catalog.tsx autopilots/components/autopilot-template-catalog.test.tsx autopilots/components/template-create-autopilot-page.tsx autopilots/components/template-create-autopilot-page.test.tsx autopilots/components/autopilots-page.tsx autopilots/components/autopilots-page.test.tsx platform/scroll-restoration.tsx platform/scroll-restoration.test.tsx
pnpm --filter @multica/web exec eslint platform/scroll-restoration.tsx platform/scroll-restoration.test.tsx
pnpm --filter @multica/desktop exec eslint src/renderer/src/platform/tab-coordinator.ts src/renderer/src/platform/tab-coordinator.test.ts
```

### Baseline TypeCheck and diff hygiene

Each command exited 0:

```sh
pnpm --filter @multica/views typecheck
pnpm --filter @multica/web typecheck
pnpm --filter @multica/desktop typecheck
git diff --check e5fe7b909c6d3987fed1a06d033edb5dd17bd3e8^ e5fe7b909c6d3987fed1a06d033edb5dd17bd3e8
```

Desktop typecheck completed both `tsconfig.node.json` and `tsconfig.web.json`. The views and Web checks completed `tsc --noEmit`. The commit diff had no whitespace errors. A final HEAD check still returned `6c24ae8c6df78b01088a95d04fe20e51381e98fd`, and the task-owned product paths remained clean.

## Closure recommendation

The bounded source fix, staged regression and scoped visual result are approved by this independent review. Post-fix tests/lint/typecheck pass; fresh native interactions confirm persistent focus on the final destination; the 16-case matrix, detector and final visual verdict pass. The parent can complete the state/spec/task/journal closure without further product changes. No new interface, global state, global navigation repair or huge suite repetition is needed. Keep the pre-fix diagnosis and final runtime/build provenance with the accepted evidence.
