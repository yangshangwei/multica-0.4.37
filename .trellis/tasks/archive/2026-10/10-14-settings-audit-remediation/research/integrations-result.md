# Provider and MCP settings remediation

## Boundary and implementation plan

Lane C owns the ten named provider/catalog/MCP settings components and their colocated tests. Failed initial reads currently appear as missing configuration or access; transient refresh failures replace Telegram, Composio and MCP content; GitHub accepts clicks it ignores during a pending settings write.

Add behavioral regressions first. Reuse the shared settings query feedback helper to separate unknown, failed and successful reads while retaining authorized content and input nodes. Keep provider callbacks, query keys, navigation and focus restoration. Lock competing GitHub controls through write settlement, associate the VCS copy fields with their labels, and use semantic foreground tokens for connected text. Core query denial-cache guards and locale integration remain coordinator-owned.

No backend, feature-gate, global primitive, dependency or external-service changes. Native geometry and contrast evidence is coordinator-owned. This lane uses fixture API reads/writes only.

## Validation

Implemented F02, F05 (VCS fields), F07 (connected foreground), F08 and F13.

- Provider permission/configuration failures now use the shared feedback component and retain authorized data. Catalog retries sit outside navigation links. Initially unconfigured Composio remains gated; a later 503 retains its loaded surface.
- MCP rename, VCS token input, Telegram confirmation and Composio search retain their DOM identity/focus across transient refresh failures. Definitive access loss hides protected management content; core wrappers prevent denied snapshots resurfacing on later failures.
- GitHub settings, Connect and Disconnect share a pending guard; the complete group becomes unavailable and saving is announced until settlement.
- VCS webhook fields have associated visible labels and purpose-specific copy buttons. Connected/warning text uses existing semantic foreground tokens; native contrast measurement remains with the coordinator.

Changed source: `packages/views/settings/components/{integrations-tab,github-tab,vcs-tab,slack-tab,lark-tab,dingtalk-tab,wecom-tab,telegram-tab,composio-tab,mcp-tab}.tsx`; `packages/core/{github,vcs,slack,lark,dingtalk,wecom,telegram,composio}/queries.ts` (scoped access-boundary wrappers only).

Changed tests: `provider-settings-recovery.test.tsx` (new), `github-tab.test.tsx`, `integrations-tab.test.tsx`, `mcp-tab.test.tsx`, `telegram-tab.test.tsx` in the same views directory. Locale additions were emitted in `research/locale-integrations.json` and merged by the coordinator.

Commands and evidence:

- Before fixes, the GitHub pending test failed on an enabled competing control; the provider recovery run reported 16 failures among 17 cases, including the exact MCP input becoming absent. The catalog additions separately failed on missing Retry and disappearance of cached Composio after 503.
- `pnpm -C packages/views test settings/components/{provider-settings-recovery,github-tab,integrations-tab,mcp-tab,slack-tab,lark-tab,dingtalk-tab,wecom-tab,telegram-tab,composio-tab}.test.tsx`: **208 passed, 10 files**. Log: `/tmp/multica-settings-provider-tests.log`.
- The final focused recovery run after strengthening Telegram render settlement: **17 passed**. Log: `/tmp/multica-settings-provider-final-recovery.log`.
- `pnpm -C packages/core test dingtalk/queries.test.ts`: **4 passed**. The full denial/cancellation matrix remains canonical in A's shared `query-access.test.ts`.
- Scoped ESLint over changed views/tests and eight core query files: **passed**. Logs: `/tmp/multica-settings-provider-lint.log`, `/tmp/multica-settings-provider-core-lint.log`.
- `git diff --check` over lane-owned source/tests: **passed**.
- Final `pnpm -C packages/views typecheck`: **passed**. Log: `/tmp/multica-settings-provider-final-typecheck.log`. The initial core/views typecheck exposed shared wrapper contextual inference; A corrected catalog selectors and this lane hoisted the DingTalk groups query function before `queryOptions`. The final views check includes the affected core modules.

## Limits and handoff

All provider service operations in component tests are memory fixtures; no real provider connects, disconnects or credentials were used. Native theme contrast, geometry, AX and combined-suite verification belong to the coordinator. Existing agent-binding flows and DingTalk group-pagination UI are retained. No feature gates, callback URLs, global primitives or dependencies changed. Continue with the scheduled token/desktop lane in this same implementer session.
