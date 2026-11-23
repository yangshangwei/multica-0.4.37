# Audit evidence and implementation cautions

Authoritative defect report: `.impeccable/audit/2026-10-13-settings/report.md`.
Evidence manifest: `.impeccable/audit/2026-10-13-settings/verification.json`.

- F01 reproduces with 11 actual 500 Responses in about 5 seconds. Nested CategorySection observers plus a loading conditional cause repeated mounting/retry.
- F03 pointer menus are clipped at 900px; keyboard focus can scroll the overflow-hidden card and reveal them. Do not claim total keyboard inaccessibility.
- F04 12 options produce a 931px-high fixed dialog in a 450×500 CSS viewport.
- F05: eight unnamed controls; Base UI IDs often identify hidden native inputs, so use semantic role/name locators.
- F06: 23 recorders have action-only names; 11 assigned bindings are aria-hidden. The other 12 unassigned StaticText nodes are still exposed.
- F07: light warning text 2.25:1, connected badge 4.00:1; their dark counterparts pass.
- F08: MCP retains its parent draft value but unmounts the input after refresh failure. Preserve actual node/focus, not just data.
- F09/F10/F11: real component probes with memory-only API methods reproduce contradictory avatar toasts, silent invalid-name saves, and duplicate pending invitations.
- F12/F13: token Escape/close bypass and ignored second GitHub switch were reproduced without real writes.
- F14/F15 require a meaningful regression to confirm their source-only deletion/response-order diagnoses before fixing.
- F16: real ApiClient + Response(200, null) crashes only the settings route; desktop shell error boundary contains it. Preserve redacted credential diagnostics.

Existing native runner: `scripts/resume-boot.mjs` and `scripts/resume-run.mjs` under the audit directory. It launches Electron on CDP 9367/control 9368 using the existing API at localhost:18572 and renderer at localhost:5666. Profile is copied to a disposable directory and removed on shutdown. MemoryRouter leaves location.pathname as `/`; select settings tabs through the UI. Main-sidebar settings href can identify the active workspace.

Do not reuse a defect probe as a passing acceptance without inverting its assertions. CDP fulfill exposed status 0 for a requested 200 response on this Electron; use native browser Response fixtures for exact HTTP semantics. Ignore aria-hidden retained chat DOM and hidden form inputs in geometry scans. Wait after native resize before selecting a responsive directory tab. MCP has nested tabpanels.

The checkout already has extensive unrelated collaboration/backend changes. UI settings source was clean before this remediation; `packages/core/api/client.ts` and core package metadata were already dirty. Snapshot and edit only PAT-specific hunks there.
