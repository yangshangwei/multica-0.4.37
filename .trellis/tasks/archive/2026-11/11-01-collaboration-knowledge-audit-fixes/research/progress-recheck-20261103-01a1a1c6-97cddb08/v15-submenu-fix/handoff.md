# Native record submenu fixture handoff

Only `e2e/fixtures/collaboration-records.ts` changed: detach and attach hover Relations, await their original menuitem visibility and invoke the same original action. The exact two replacement blocks account for the full file delta; original action labels, dialog/selection code, caller API/mutation/identity assertions and product files are preserved.

The installed Base UI trigger defaults to hover opening with 100ms rest and ignores mouse clicks for opening. This is a source-confirmed unsupported fixture gesture; the prior native failure capture displays the correct child/parent and closed submenu. The saved red capture and original helper bytes are under this directory.

Strict TypeScript no-emit compile of the fixture/import tree using pinned Node22.23.2/TypeScript5.9.3 and repository base settings exited0. Diff check exited0. No product/browser/native tests, service/database/model calls, new dependencies or commits were performed. Native green evidence remains pending root's directed execution window. Include the Web record caller as adjacent coverage because the shared helper changed for both platforms.
