# Squad welcome acceptance — 2026-11-20

The approved first-use welcome is implemented in the shared Web/Desktop squad page. It preserves the workspace/template directory and the existing creation chooser, introduces three manually selectable illustrative scenes, and uses the application's embedded squad guide.

## Verification

- Views page, creation chooser and locale parity: 93 tests passed.
- Docs anchor/bundle parity: 5 tests passed.
- Core docs schema and route consistency: 28 tests passed.
- Scoped ESLint, Views typecheck and task `git diff --check`: passed.
- Embedded documentation: `go test ./internal/docs/...` passed.
- Production API, Web and Desktop renderer builds: passed.
- Final browser replay: 3 passed, 0 skipped, 0 unexpected, 0 flaky; see `playwright-results.json` and `browser-final.exit`.
- Impeccable detector: `[]`. The initial visual verdict was 88 because Tailwind inline radius aliases did not resolve in the CSS module. The exact runtime-token formulas corrected the local panels; the final verdict is 94.

The final browser replay verifies creation and documentation navigation, the template directory, all manual examples, keyboard activation, English/Chinese layouts at 1440/1024/390 pixels, dark appearance, reduced motion, 1024×768 initial CTA visibility, and enlarged Web/Electron content. The native 200% zoom uses the real Electron compositor and scroll container. Current example panels are horizontally contained; computed outer radius is 18px; inactive examples are `aria-hidden` and `inert`; no daemon was started.

## Evidence and provenance

`summary.json` binds current task source SHA-256 values, production launch mode, build ID and actual origins to the final report. `environment.json` records the isolated API health/process identity and candidate directories. The task's static-review details are in `.trellis/tasks/11-20-squad-welcome/research/check-20261120.md` until archival.

Final captures are under `results/`. The principal views are:

- `e2e-squad-welcome-empty-sq-03d88-creation-and-docs-reachable-chromium/zh-Hans-1440.png`
- `e2e-squad-welcome-empty-sq-03d88-creation-and-docs-reachable-chromium/en-390.png`
- `e2e-squad-welcome-empty-sq-03d88-creation-and-docs-reachable-chromium/desktop-dark.png`
- `artifacts-qa-squad-welcome-09427-r-and-offline-documentation-chromium/electron-wide.png`
- `artifacts-qa-squad-welcome-09427-r-and-offline-documentation-chromium/electron-200-percent-cta.png`
- `artifacts-qa-squad-welcome-09427-r-and-offline-documentation-chromium/electron-200-percent-carousel.png`

The first browser run passed its interaction assertions, but native screenshots were taken before the latest scroll painted and geometry could precede a CSS transition. Those original results remain in `results-r1/` and `playwright-r1.json`. The final full replay adds stable painted-layout sampling and explicit viewport/bounds assertions. It uses the same product build; no further UI changes were needed.

## Reproduction

`prepare.py` creates a private test database/API and copies Web/Desktop app files into a separate candidate directory, using this checkout's shared packages and installed dependencies. It preserves source environment files and shared build output. The private environment remains outside the repository.

Run Playwright with the private fixture variables and this directory's `playwright.config.ts`. The standard product journey is `e2e/squad-welcome.spec.ts`; `native.spec.ts` supplies the task-local enlarged-content and Electron checks. `cleanup.json` records stopping the owned services and removing the synthetic database container.

## Limits

The existing `ApiClient.listSquads()` falls back to an empty array for malformed successful responses. This view distinguishes Query pending/HTTP failure/filter-empty states but cannot identify that inherited fallback. API/schema changes were excluded from the approved scope.

Electron acceptance uses the built renderer, actual preload and native routing functions with an isolated test Main/daemon boundary. It does not establish packaged Main, installer/updater compatibility or real-agent/model execution.
