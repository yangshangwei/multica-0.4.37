# MCP first-use introduction

## Goal

Help workspace members understand the value of MCP and reach the existing discovery or configuration flow when their workspace has no saved MCP servers. Follow the established Skill and Agent welcome presentation.

## Approval and background

- The user requested a MCP introduction shown only without MCP data, referenced the Skill and Agent pages, and explicitly requested the impeccable skill.
- The reviewed conversational plan specified a left introduction, right manual three-scene showcase, existing market/custom creation actions, successful-empty gating, and shared responsive bilingual styling.
- On 2026-11-21, the user approved creating this task and explicitly requested planning and development of that same scope. No further product decision is outstanding.
- `packages/views/mcp/mcp-page.tsx:5` renders the shared settings component with `presentation="page"`; `packages/views/settings/components/mcp-tab.tsx:58` also supports the compact Settings presentation.
- `packages/views/settings/components/mcp-tab.tsx:100-111` owns membership, server inventory and management permissions. `getSettingsQueryState()` preserves loaded data on recoverable refresh errors and withholds it on access loss.

## Requirements

- R1: Show the introduction only on the standalone MCP page, with confirmed membership and a successful, defined empty workspace MCP inventory. Failed membership or inventory refreshes must not claim a successful empty state.
- R2: Any saved workspace server suppresses the introduction, including a disabled server. Catalog recipes and agent discovery are separate resources and must not determine this state. Data transitions must switch presentation without a reload.
- R3: Use the existing visual family: semantic tokens, left value proposition and actions, right three manually selected illustrative workflow examples. Explain choosing a service, configuring a connection, and explicitly assigning it to an agent without claiming automatic assignment or successful execution.
- R4: Reuse the existing market route and custom-server dialog. Only owners/admins may open the creation flow. Ordinary members can browse the market and receive concise management guidance. Do not duplicate the same action in the page header and welcome.
- R5: Preserve the Settings-tab empty state and all populated-list editing, naming, assignment, deletion, permission and retry behavior.
- R6: Provide English and Simplified Chinese copy, keyboard-operable carousel controls, hidden and inert inactive examples, clear focus, reduced motion and responsive layouts without horizontal document overflow. Reset example selection when workspace identity changes.

## Acceptance criteria

- AC1 (R1): Initial loading, unknown/paused data, missing membership, access denial, initial error and failed empty-cache refresh render existing feedback without the introduction.
- AC2 (R2, R5): Empty success renders the introduction; populated/disabled inventory renders the list; empty → populated → empty transitions work; Settings stays compact.
- AC3 (R3, R6): All three localized examples can be selected using keyboard or pointer; inactive content is inaccessible; end controls are disabled appropriately; workspace change resets the selection.
- AC4 (R4): Market navigation uses the existing navigation adapter; permitted custom creation opens the real existing dialog and dismissal restores focus; members have no custom creation action.
- AC5 (R5, R6): Relevant component/localization tests, lint and TypeScript checks pass. Real compiled Web captures cover both supported locales/themes and desktop/narrow layouts; shared Desktop behavior is checked using an available established harness.

## Out of scope

No backend/schema/API changes, new dependencies, automatic connections or assignments, historical first-use tracking, persisted dismissal, market redesign, shared welcome-framework refactor, mobile-app changes or unrelated UI cleanup.

## Known limitations

The existing API parser may normalize malformed successful inventory responses. This UI does not change that compatibility boundary and does not infer whether a workspace ever had a server in its history.
