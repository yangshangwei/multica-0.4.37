# Record submenu fixture correction

The current native failure reaches the correct ordinary child with its parent visible, but the Relations submenu remains closed. The installed Base UI SubmenuTrigger defaults to `openOnHover=true` with a 100ms rest delay and `useClick(ignoreMouse=true, toggle=false)`. A Playwright mouse click therefore does not force submenu opening. The repository wrapper forwards those defaults; ordinary E2E date submenu paths already use hover.

The behavior gap lives in `e2e/fixtures/collaboration-records.ts`: both helpers treat a mouse click as submenu readiness. Change only those gestures to hover, wait for the original Remove/Set parent menuitem to become visibly actionable, then click that same item. Keep original dialog and actual backend identity/mutation assertions. No product/fixture data change, API shortcut, new abstraction, timeout or sleep is involved.

The unchanged v14 failed native execution and screenshot are retained as red evidence. This preparation will run only allowed type/diff checks. Native green evidence remains pending the root-owned directed environment.
