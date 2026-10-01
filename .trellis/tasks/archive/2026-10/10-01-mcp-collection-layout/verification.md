# Verification — MCP collection layout

Date: 2026-10-01. Result: approved UI implemented and verified locally.

## Delivered
- Full-width standalone MCP collection with the shared header and custom-add action, using the existing workspace controller; settings retain their embedded shell.
- Separate workspace/template counts with unknown counts omitted, stable accessible tab names, bounded search and a visible filtered result count.
- Container-responsive 1/2/3-column cards, category metadata, aligned configuration actions, existing typography/surface tokens and 40px Lucide tiles (Bug/Workflow blue, Brain purple).
- Original creation, explicit assignment, rename, reuse and retry flows preserved.

## Evidence
| Check | Result |
| --- | --- |
| Existing market/discovery/settings baseline | 48 tests passed before implementation |
| New standalone heading/header-action regression | Observed failure before fix, passed afterward |
| MCP market/discovery/settings/rows/agent-tab/locales regression | 10 files, 139 tests passed |
| `pnpm --filter @multica/views typecheck` | Passed |
| `pnpm --filter @multica/views lint` | 0 errors; 26 existing warnings outside changed files |
| Scoped ESLint on all changed views files | Passed without warnings |
| Impeccable mechanical detector | No findings |
| `git diff --check` | Passed |
| Playwright collection canvas | 1440px: 3 columns; 900px: 2 columns; 390px: 1 column, no horizontal overflow |
| Playwright existing recipes/reuse flow | All 3 recipes saved; explicit assignment, rename provenance, existing-instance reuse, narrow dialogs and long names passed |
| Playwright assignment retry | Failed grant retried without recreating configuration or repeating successful grants |
| Visual review | 95/100, pass; incumbent Agents/Skills reference captures and Chinese/light + English/dark wide/narrow captures |

Browser command:
`bash scripts/dev-env.sh exec -- env E2E_PASSWORD_AUTH=1 pnpm exec playwright test e2e/mcp-market.spec.ts --grep 'collection canvas|creates all three|retries a failed' --reporter=line --output=/tmp/mcp-collection-e2e`

Result: 3 passed in 1.4 minutes. The live Next development process serves this checkout at localhost:13493. Its API at localhost:18573 is an existing password-mode server, so the original email-code fixture returned 403 before any UI assertion. Added an explicit local password-mode fixture option and resolved runtime fixture owners by authenticated user ID instead of email. No server configuration was changed. Isolated test workspaces/accounts, including three failed-setup workspaces, were cleaned up.

## Artifacts
- `.omx/artifacts/mcp-collection-layout/collection-chinese-light-wide.png`
- `.omx/artifacts/mcp-collection-layout/collection-chinese-light-narrow.png`
- `.omx/artifacts/mcp-collection-layout/collection-dark-1440.png`
- `.omx/artifacts/mcp-collection-layout/reference-skills.png`
- `.omx/artifacts/mcp-collection-layout/reference-agents.png`
- `.omx/state/mcp-collection-layout/ralph-progress.json`

## Review and limits
- Reviewed query ownership, shell lifetimes, permission branches, count semantics, focus contrast, scrolling and package boundaries directly. Native subagent review attempts failed at the model service (unsupported role model/rate limit), so no independent review is claimed.
- The initial pass exercised shared views in Chromium. Native Electron validation was subsequently completed below. The email-invitation member E2E case requires email auth and was not rerun against this password-only server; member/permission behavior remains covered by the passing component suites.
- Unrelated sidebar edits were preserved and excluded from the MCP commits.

## Follow-up: native Electron verification and nested-tab fix

The user requested additional verification. Rebuilt the actual desktop main,
preload and renderer with `pnpm --filter @multica/desktop exec electron-vite build`.
The test launches a real macOS Electron BrowserWindow with the actual preload,
renderer, desktop shell and memory router, an isolated profile and a temporary
password account. A local relay passes API responses unchanged; the existing
native fixture isolates daemon/updater services and blocks external network.

Found and fixed a real Settings embed defect: `group-data-vertical/tabs` matched
the outer Settings root and made the inner horizontal MCP tabs vertical. A new
geometry assertion failed with 44px vertical separation before the fix. Shared
`TabsList` / `TabsTrigger` now use their own Base UI `data-orientation` attributes.
MCP, skill catalog, skill template panel and dashboard override classes use the
same local variants. No new state, dependencies or per-page CSS workaround.

Fresh verification after the fix:

| Check | Result |
| --- | --- |
| Native MCP E2E | 1 passed, 5.9 seconds |
| Web MCP E2E rerun after shared-tab fix | 3 passed, 56.4 seconds |
| Native resizing | 1380px: 3 columns; 800px: 2; 420px: 1; no horizontal overflow |
| Native interactions | Header custom editor, keyboard template opening, save + explicit assignment verified against API, focus restored to opener |
| Embedded surfaces | Settings tabs align horizontally; agent discovery remains scrollable at 600px |
| Native presentation | English/dark, Chinese/light, visible configuration actions and reachable dialog footer |
| Renderer exceptions / daemon starts | 0 / 0 |
| MCP, Settings, Skills and dashboard component suites | 14 files, 219 tests passed |
| UI + Views typecheck | Passed after fix |
| Desktop node + renderer typecheck | Passed |
| Scoped UI/Views ESLint | Passed |
| Desktop rebuild and diff whitespace check | Passed |

Native command:
`bash scripts/dev-env.sh exec -- env E2E_PASSWORD_AUTH=1 pnpm exec playwright test e2e/mcp-desktop.spec.ts --reporter=line --output=/tmp/mcp-native-final`

The agent MCP screen is entered through persisted desktop-tab restoration, so
this test does not claim agent-directory click coverage. No real MCP process or
model invocation is performed. All temporary accounts, workspaces, native
processes, local relay and profile directories are cleaned up by the test.

Screenshots live in `.omx/artifacts/mcp-collection-layout/native/`, including
`native-chinese-light.png`, `native-settings-market.png`, and
`native-agent-market-narrow.png`. Native visual verdict: 96/100, pass.

## Follow-up: preserve market icons in shared configurations

User reported that saved template instances all displayed transport icons.
The row ignored `template_key`. Extracted the market's existing icon/tone/tile
markup into `common/mcp-template-icon.tsx`, reused it from market cards and the
workspace settings controller, and added an optional leading icon slot to
`McpServerRow`. Transport text remains visible. Custom/legacy configurations
without provenance keep transport icons; names are never used as identity.

- Observed all three renamed-template icon regressions fail before the fix.
- Afterward: 4 files / 59 tests passed, including rename retention, provenance
  clearing after full replacement, and a custom configuration named Playwright.
- Views typecheck and scoped ESLint passed; detector returned no findings.
- Real Web recipe/rename/reuse flow passed in 30.9s. Added equality assertions
  comparing each saved row's SVG drawing, computed icon color and tile background
  with its corresponding market card, including the renamed Playwright instance.
- Rebuilt desktop and reran native MCP flow: passed in 6.4s. Visually checked
  Chinese shared inventory and the native shared row. Temporary fixtures cleaned.
- Screenshot: `.omx/artifacts/mcp-collection-layout/shared-template-icons.png`.

No backend/API changes, configuration reads, migrations or new dependencies.

## Delivery

The user requested commit and push to `origin/main` on 2026-10-01.

- `3b474cf14`: local-orientation fix for nested tabs and consumer overrides.
- `5308ccd60`: MCP collection layout, shared template icons, regression coverage and domain contract.
- Final pre-commit component check: 14 files / 223 tests passed.
- The pre-existing sidebar navigation regrouping remains outside these commits.
