# MCP introduction verification

The standalone MCP page shows the introduction after the workspace membership
and saved server requests succeed and the saved inventory is empty. The tests
cover switching to ordinary management on save, returning to the introduction
when the inventory empties, and retaining the Settings tab's compact state.
Ordinary members can browse the capability market. Owners and admins can also
open the existing editor from the introduction.

## Checks

- 315 tests passed across 18 Views files, covering the MCP views and controller,
  settings recovery, locale parity, and the Skill and Agent welcomes.
- Views TypeScript, ESLint for the changed Views source and tests, and both
  Trellis context manifests passed. Impeccable reported zero findings.
- Production Next.js and Electron renderer builds used the isolated source
  snapshot. All three Playwright scenarios passed on the first attempt against
  its API. They exercised the Web welcome through saving and deleting a
  configuration, the Electron welcome in wide and narrow panes, and the
  existing capability market entry and responsive catalog.
- Eight Web layout checks covered English and Simplified Chinese in light and
  dark themes at wide and narrow widths. Both the page and the welcome stayed
  within the viewport. The narrow checks scrolled to the real example controls.
- The visual review compared compiled captures against the empty Skill and
  Agent pages. The final screenshots confirm the corrected language about
  shared workspace configuration and explicit assignment in both locales.

The test environment's API, Web and renderer ports were released. Local
evidence in `artifacts/qa/mcp-welcome-20261121/` includes the final source
hashes, build and Playwright logs, browser captures and visual verdict.

Electron acceptance uses the built renderer and preload in the established
test harness. The live save verifies configuration persistence using an
example URL. Tool execution and packaged application delivery fall outside
this acceptance scope. The server list retains its existing parsing behavior
for older API responses.
