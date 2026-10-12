# MCP agent-selection verification

## Outcome

The saved MCP configuration's assignment step now supports full-name/description search, filtered-result selection/deselection, selected-only review, clear selection, eligible unfinished counts and hidden-selection feedback. Compact identity rows reuse shared avatars and inert two-line previews. The parent retains explicit IDs, canonical eligibility, saved-instance identity, confirmed successes and failed-only retry.

The search/selection toolbar and footer stay reachable while only the agent list scrolls. The browser correction anchors absolute accessible descriptions to their list rows and removes a display override that defeated line clamping. Per-agent filtered failure details use the list scroll owner; global feedback remains visible.

## Evidence

- Before-change baseline: 41 market/discovery/MCP-locale tests passed across 3 files.
- Core assignment/request contract: 12 tests passed across 2 files, including captured workspace identity, duplicate prevention, existing disabled bindings and confirmation checks.
- Implementer broad MCP/locale/parity run: 247 tests passed across 12 files; after the browser correction, 111 affected tests passed across 4 files. The final combined run passed all 248 tests across 12 files after independent review added a refreshed-directory regression; see `research/check-verification.md`.
- Repository `pnpm typecheck`: all 9 tasks passed, including Web and Desktop consumers.
- Repository `pnpm lint`: all 6 tasks passed, with existing warnings outside this change. Task-owned ESLint files are clean.
- Impeccable mechanical detector over the two UI files: empty findings (`[]`).
- Knip final report is identical to its pre-change baseline: 17 unused-file findings and two unused-dependency findings; no new finding.
- `git diff --check` passed for task-owned code/artifacts.

Logs and browser scripts are in `.omx/artifacts/mcp-agent-selection/`. `after-browser.json` records runtime source SHA-256 values, preview identity and measured bounds. `.omx/state/mcp-agent-selection/ralph-progress.json` records the final visual verdict: 94/100, pass.

## Browser acceptance

Built a production Vite fixture around the actual shared `McpSetupDialog`, Web CSS/design tokens/primitives, QueryClient and canonical core assignment mutation, using 80 synthetic cached agents and a fixture API. Five final captures cover 1280px dark Chinese/light English, 390px coarse-pointer dark Chinese/light English and 680px dark English with long unbroken names/descriptions.

Every capture verifies no horizontal overflow, zero unintended dialog scroll offset, no escaped dialog scroll extent, reachable heading/search/footer after selection and list scrolling, at most two-line previews, and 44px coarse-pointer controls. Browser errors: zero.

Interaction checks verify save-to-heading focus, initial empty selection and disabled primary action, complete description search, scoped bulk select/deselect, no-match recovery, selected-only review and clear, Space-key selection, submission of hidden selected IDs with captured saved-server/workspace identity, and partial failure/retry that writes only the failed ID and keeps confirmed rows disabled. A search beyond the description preview limit still finds its agent.

## Limits

This is real-component browser acceptance with synthetic data, not a full application/real-backend or native Electron run. The existing human environment reports API/Web/Desktop ownership mismatch and was not reused or stopped. Backend/API contracts were not changed. No third-party MCP program, provider CLI, deployment or remote operation was executed.

The task-owned fixture previews are stopped during finish-work; screenshots and scripts remain available for inspection. Unrelated concurrent core/auth/recovery and Trellis task changes are excluded from the task commit.
