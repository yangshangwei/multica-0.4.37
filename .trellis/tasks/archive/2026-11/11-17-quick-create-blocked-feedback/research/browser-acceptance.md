# Browser acceptance — 2026-11-20

Result: all 18 new browser cases and all 15 existing picker/autonomy/phase-2 cases passed against the final candidate. The final new-case verdict is composed of 14 mechanical passes plus four visual passes, followed by five targeted passes after test cleanup changes; it is not a claim that the earlier failing 18-case invocation passed.

The production Web build is `yPiCLeEob9_90sBlHK-1y`, built in a task-private copied Web directory with `pnpm exec next build --webpack`. The renderer/preload were built separately in that private copy with `pnpm exec electron-vite build`. Source/build hashes, exact commands, per-case statuses and raw-result hashes are in `artifacts/qa/quick-create-blocked-20261120/evidence/summary.json`. The task's final product source hashes still match those build inputs. Other sessions advanced Git HEAD during verification; the file hashes, not the later HEAD alone, bind these results.

## Coverage and evidence

| Run | Result | Meaning |
| --- | --- | --- |
| `acceptance-final` | 14 pass, 4 visual fixture failures | All mechanical cases passed on final product bytes |
| `visual-final-r3` | 4 pass | Full en/zh-Hans × light/dark × 1280×900/375×812 matrix, empty/ready/no-candidate states |
| `fixture-cleanup-confirmation` | 5 pass | Pending/upload and four visual cases rechecked after adding `finally` cleanup |
| `existing-picker` | 7 pass, 1 conditional native skip | Existing Web discovery/assignment/search/persistence |
| `existing-autonomy` | 3 pass | Existing UI/API authority/hand-off |
| `existing-recommendations` | 3 pass, 1 conditional native skip | Defaults, explicit recommendations and narrow Chinese layout |
| `native-picker` | 1 pass | Previously skipped native case, actual isolated Electron renderer/preload |
| `native-recommendations` | 1 pass | Previously skipped native case, actual isolated Electron renderer/preload |

The new cases cover Tab access and focus/hover Tooltip, the persistent accessible description, pointer and shortcut blocker focus, empty-list exits and draft retention, no recommendation request with zero candidates, loading/failure/autonomy/version routes, live clearing and immediate click (under 150ms from `fill` return to click completion, with the old-build failure retained), recommendation adoption followed by modifier+Enter, modifier+Enter from close/property/switch/editor, portal search and other input exclusions, plain Enter button activation, and native disabled during both upload and submission. Real quick-create acceptance returns 202 and the single-send cases verify exactly one queued task. No daemon consumes those tasks.

The 24 PNGs and `geometry-and-contrast.json` under `evidence/` contain the final real components. The same-state helper colors and opacity match in every measured language/viewport. Canvas resolves the actual computed colors to sRGB; the screenshot sample additionally checks text/background pixels in each helper's measured rectangle.

| Helper state | Light | Dark |
| --- | ---: | ---: |
| Disabled | 3.7365:1 | 4.9521:1 |
| Enabled | 5.9126:1 | 6.3398:1 |

The persistent assignment hint is 4px below the recipient trigger. Its distance to the description label is 12–80px across the empty/ready matrix. Every footer submit is fully in the viewport and the document has no horizontal overflow. The no-candidate notice uses the real empty workspace; its editor remains visible and editable. The compact English wide no-candidate state exposes 49.107% of the editor, retained as an observation: the 50% requirement belongs to recommendation-list display, not this state.

## Retained failed attempts and repairs

- The first old-build run retained the actual freshly-cleared-description focus regression and the real assignment-hint grouping violation. The product check lane fixed both; the final browser cases passed.
- The initial Tooltip locator expected `role=tooltip`, but the installed Base UI popup exposes `data-slot=tooltip-content` without that role. The test now separately asserts its real visible popup, keyboard focus and the independent accessible description.
- The first portal test pressed Escape while a selected popup was closing. The corrected test searches for no matching recipient, proves modifier+Enter sends zero requests, then closes and awaits that popup before testing the real clarification textarea.
- Initial visual probes sampled dialog animation and used an unsupported DELETE-agent route. Later fixture probes retain these failures. Stable measurements wait for actual animations; no-candidate screenshots now use a fresh empty workspace rather than exercising unrelated archive/WebSocket/onboarding behavior.
- A different session replaced the shared `.next` build while the initial read-only server used it. The resulting error-boundary probes are invalid infrastructure evidence, not product failures. Final Web runs use a private independent `.next`; this task never wrote the shared build.
- Resize/repeated `fill("")` visual fixture failures were replaced with actual select-all/Backspace input, explicit empty-text assertion and stable dialog width. An isolated diagnostic also verified both fill and keyboard clearing produce native-disabled helpers. Final four-case visual runs pass without product changes.
- A no-candidate editor ≥50% assertion was removed because it was borrowed from the separate recommendation-list contract. The measured 49.107% is documented above; full submit visibility and actual editor visibility remain asserted.
- Independent static review requested guaranteed release of delayed route handlers and guaranteed closure of the manually created empty-workspace page. Both now use `finally`; the affected five cases passed. An explicit generic fixed the test fixture's optional-agent TypeScript inference without changing runtime behavior.

All raw failed reports/traces remain locally under the task's QA directory. Do not commit them: request traces and full API logs may contain synthetic credentials. The compact `evidence/` files contain no tokens, storageState, database URL or private environment values.

## Environment and cleanup

The API and database were task-owned: a fresh ARM64 PostgreSQL container, migrated isolated database, synthetic accounts/workspaces, local deterministic recommendation provider, and no real agent daemon. Startup was `python3 artifacts/qa/quick-create-blocked-20261120/start.py`; candidate startup and native startup are recorded in the task harness. No `make up/down/destroy` ran in the shared checkout; no user Desktop was attached.

After both independent review lanes finished, the retained review workspace was deleted (204), recorded provider/API/Web/renderer process groups were terminated, and the owned database container was stopped and removed. Ports 60676, 60677, 60678, 60703 and 59042 were confirmed closed. `evidence/cleanup.json` records exact owned process groups. Private candidate builds and raw evidence remain for diagnosis.

Validation limits: these are production-Web/isolated-renderer UI and real local API checks, with deterministic AI output and selected network-boundary faults. They do not establish physical touch, real screen-reader behavior, packaged Main/installer behavior, or real paid-agent execution. The pure submit-state matrix remains covered in its canonical core unit suite. Full repository lint/typecheck and component checks are owned by the parent/check lane; this E2E file additionally passed targeted strict TypeScript review and syntax parsing.
