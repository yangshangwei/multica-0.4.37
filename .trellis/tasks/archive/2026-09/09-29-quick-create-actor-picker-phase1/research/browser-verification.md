# Browser acceptance evidence

Final run: 2026-09-29. Worktree: `/Volumes/artisan/code/2026/multica-actor-picker-phase1`.

## Result

**4 passed in 48.6 seconds**, one worker, no retries. No installed agent CLI or daemon was executed. All test-created workspaces and the Electron profile were cleaned up by the fixtures.

```sh
PLAYWRIGHT_BASE_URL=http://localhost:13061 \
NEXT_PUBLIC_API_URL=http://localhost:18141 \
CHANGELOG_ELECTRON_RENDERER_URL=http://127.0.0.1:59918 \
CHANGELOG_E2E_API_URL=http://localhost:18141 \
ACTOR_PICKER_BUILD_MODE=development \
ACTOR_PICKER_EVIDENCE_DIR=.trellis/tasks/09-29-quick-create-actor-picker-phase1/research \
pnpm exec playwright test e2e/quick-create-actor-picker.spec.ts --workers=1 --trace retain-on-failure
```

| Scenario | Evidence | Duration |
| --- | --- | --- |
| Real API actors and preferences | Four actual agents and one squad created through API, isolated no-process runtime; pinning preserves selection and draft; four favorites expose three shortcuts; full favorites/search; quick-create returns actual HTTP 202 and task ID before recent appears; reload and A → empty B → A preserve workspace isolation | 21.5s |
| Large directory | UI-only route mocks: 500 agents and 50 squads; 50-item pages reach all 550 actors; full description suffix beyond preview truncation is searchable; type intersects search; query change resets page limit; 20 browser render samples | 11.9s |
| Narrow keyboard flow | Actual shared picker in Web at 375 × 812; composition Enter does not select; ordinary Enter selects; reopening resets query/filter; Tab/Space pins without changing draft; browsing all retains input focus; Escape returns focus to creator trigger; no page horizontal overflow | 7.9s |
| Actual Electron renderer | Built desktop preload, renderer, router and shell using existing `changelog-electron.cjs`; real API actor fixture; description search, Space pin, visible pin focus, Escape return focus; native services report `daemonStarts: 0`, `externalLinks: []`, `installCalls: 0` | 4.6s |

The browser test is an integration complement to canonical core/model/component matrices, not a duplicate of all boundary cases. The real API test validates request acceptance and local recent tracking, not model execution or eventual issue generation. The 550-object test intentionally intercepts only the directory/runtime at the network boundary; its submit endpoint rejects accidental writes.

## Performance

See `browser-actor-picker-performance.json` for all samples. Final **P95: 20.7ms**, target ≤100ms.

- Web: Next.js development server; loaded directory; Chromium 145.0.7632.6.
- Machine: Apple M4, macOS/Darwin 25.2.0, Node v22.23.2; viewport 1280 × 900.
- Twenty alternating exact-name and full saved-description queries across the directory.
- `performance.now()` starts in the native input event capture handler. A requestAnimationFrame poll checks for the expected sole actor; the following frame records elapsed time. This includes React/DOM work and frame scheduling, excludes Playwright transport and initial network/compilation.
- The 100ms target is reported as an acceptance measurement, not asserted in shared CI. It is not a production-build benchmark. Electron was separately tested from an `electron-vite build` output served locally.

## Visual evidence

- `browser-actor-picker-1280.png`: three favorites, one recent, selected recent hovered, two-line responsibilities.
- `browser-actor-picker-1280-dark.png`: same persisted preferences and hover state in dark theme.
- `browser-actor-picker-375.png`: scrolled directory includes agent, squad and multilingual long name; selected row stays identifiable; search/type bar stays visible.
- `browser-actor-picker-desktop.png`: actual desktop renderer with keyboard-focused pin control.
- `browser-visual-verdict.json`: aggregate visual review, 95/pass. Historical `current-picker.png` provided the existing style reference; the new grouping and description rows are intentional design changes.

## Regressions found and resolved

The narrow browser test exposed a real Escape regression: browsing all disabled or removed the focused action, moving focus outside the nested picker and allowing Escape to dismiss the parent dialog. The UI implementation now focuses the persistent search input before changing view and keeps the Browse all action enabled. Final browser assertions confirm input focus after browsing and creator-trigger focus after Escape. Product changes were made by the UI owner.

Initial harness issues were corrected by scoping the contenteditable locator to the quick-create dialog (the app also mounts a chat editor), and comparing creator textContent consistently. The isolated API initially lacked the desktop static renderer CORS origin; the environment owner corrected it before the successful final batch.

## Static verification

The following exited zero after the final browser run:

```sh
pnpm exec tsc --noEmit --target ES2022 --module ESNext --moduleResolution Bundler --esModuleInterop --skipLibCheck e2e/quick-create-actor-picker.spec.ts
git diff --check -- e2e/quick-create-actor-picker.spec.ts
```

Changed source: only `e2e/quick-create-actor-picker.spec.ts` in this test lane. Existing TestApiClient and Electron fixtures were reused without changes or new dependencies. Remaining limits: Web performance was measured in development mode; no mobile-app or real-agent execution claims are made.
