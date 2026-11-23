# MCP welcome independent quality review

- Date: 2026-11-21
- Role: dispatched `trellis-check`; only the two specifically approved locale-value corrections below were applied. No structural product edits, service operations or sub-agent dispatches.
- Scope: task-owned welcome TSX/CSS/typing/test, controller integration/tests, bilingual settings strings, MCP locale test and focused browser journeys/selectors.

## Findings (fixed)

### P2: Introduction overstates isolation from runtime-local MCP servers

- Location: `packages/views/locales/en/settings.json:336` and `packages/views/locales/zh-Hans/settings.json:336`, `mcp.welcome.boundary_note`.
- Original claim: each agent uses **only** the servers explicitly assigned here.
- Evidence: `server/pkg/agent/codebuddy.go:62-68` intentionally omits `--strict-mcp-config`; `:115-119` confirms managed configuration adds to the runtime's existing user, project and local MCP scopes. Workspace assignment is not a guarantee that the runtime uses no other MCP servers.
- Narrow correction: English `Configurations live in your workspace. Assign each one to the agents that need it.`; Simplified Chinese `配置保存在工作空间中，可按需分配给智能体。`
- Resolution: after the parent completed its first compiled Web/Electron visual pass, it explicitly authorized applying only these two values. Both values now contain the narrow corrections above; no other locale value or product structure was changed by this reviewer.
- Follow-up verification: `pnpm --filter @multica/views exec vitest run locales/mcp.test.ts locales/parity.test.ts mcp/mcp-welcome.test.tsx --reporter=dot` passed all 3 files / 76 tests. Scoped locale `git diff --check` passed. Parent owns resyncing the two changed JSON files into the isolated QA worktree.

## Findings (not fixed)

None remain from this review.

## Verified implementation behavior

- The welcome gate uses raw saved inventory, defined successful member/server reads, active membership and both existing failure predicates; a cached empty result with a refresh failure does not become a welcome.
- Definitive permission loss remains governed by the existing access boundary. Existing populated/Settings retention and mutations are preserved.
- Disabled saved instances suppress the introduction. Market recipes do not create a new observer or count as saved configurations.
- Welcome and populated content share the same owner, so an open configuration dialog survives an inventory transition. Its final-focus resolver reads the current live add-action ref, including the header replacement after the welcome opener unmounts.
- Ordinary members receive browse and guidance without the custom write action. Header actions are suppressed only while the welcome owns them.
- Local examples are static, manually selected, localized, hidden and inert when inactive, and scoped by the existing workspace-keyed controller. No added timer, store, dependency, API call or automatic assignment was found.
- CSS remains local and uses shared semantic tokens and container queries. Actual compiled geometry, contrast, native painting and reachability remain the parent's browser/visual gate.

## Independent verification

- Pass: `pnpm --filter @multica/views exec eslint mcp/mcp-welcome.tsx mcp/mcp-welcome.test.tsx settings/components/mcp-tab.tsx settings/components/mcp-tab.test.tsx locales/mcp.test.ts`.
- Pass: `pnpm --filter @multica/views typecheck`.
- Pass: `pnpm --filter @multica/views exec vitest run settings/components/mcp-tab.test.tsx mcp/mcp-welcome.test.tsx settings/components/provider-settings-recovery.test.tsx locales/mcp.test.ts locales/parity.test.ts skills/components/skill-welcome.test.tsx agents/components/agent-welcome.test.tsx --reporter=dot` — 7 files, 167 tests.
- Pass: scoped `git diff --check` for the tracked task-owned integration/localization/E2E-selector files.
- Existing pnpm configuration deprecation warning was emitted; it did not fail any check and is outside this UI task.

## Remaining gates

The parent reported that the first production Web/Electron browser suites and compiled visual pass passed. Parent owns resyncing the two corrected JSON files, its bounded final confirmation, impeccable detector, final spec synchronization and task closeout. No additional implementation defect or scope expansion was identified in this review.
