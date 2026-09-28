# MCP sidebar implementation plan

**Goal:** Make the existing workspace MCP library directly accessible in AI Team.

**Architecture:** Add `/:slug/mcp` and a small shared page shell which renders the existing `McpTab`. Keep the settings entry intact. Reuse the central path/icon registry and both platform routers.

**Decision:** A direct settings link would switch to settings navigation instead of keeping the requested AI Team entry selected. Copying the management UI would create two implementations. The shared page wrapper gives a dedicated destination without duplicating behavior.

1. Extend `packages/core/paths/paths.ts` and `route-icons.ts`; update existing route consistency and presentation tests.
2. Add `packages/views/mcp/index.ts` and `mcp-page.tsx`, with PageHeader, existing settings content width, scrolling, and `McpTab` composition. Export the new subpath in the package manifest.
3. Add sidebar key/order and bilingual labels. Wire the Web page and desktop route.
4. Verify existing sidebar/MCP tests, core paths tests, locale parity, affected-package typechecks/lint, and inspect the UI if a runnable environment is available.
5. Review the final diff and retain the isolated branch for the user.

Validation: `pnpm --filter @multica/core exec vitest run paths`; `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx settings/components/mcp-tab.test.tsx locales/parity.test.ts`; affected-package TypeScript and ESLint; `git diff --check`.
