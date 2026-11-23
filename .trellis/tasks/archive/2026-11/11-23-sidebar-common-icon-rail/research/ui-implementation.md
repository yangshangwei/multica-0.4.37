# Shared sidebar implementation evidence

## Owned changes

- `packages/views/layout/app-sidebar.tsx`: shared icon/full presentation, native common route links and More groups, scoped notification/chat observers, retained collaboration creation owner with visible-trigger return focus, icon workspace/account/footer presentation. Native pin menu rows reuse PinRow data resolution while sortable/unpin behavior stays in the expanded list.
- `packages/views/layout/app-sidebar.test.tsx`: seven new behavior regressions plus real Base UI pin submenu keyboard/Escape smoke; existing expanded behavior and scope tests retained.
- `packages/views/layout/page-header.tsx` and `.test.tsx`: compact-only fallback trigger; large Web rail owns its toggle, Desktop external toggle remains authoritative.
- `packages/views/locales/en/layout.json` and `zh-Hans/layout.json`: More navigation / 更多入口 and My work / 我的工作.
- No changes to dependencies, stores, API/server or platform-specific routing. No commit or staging.

## Verification

- Baseline `ui-red.log`: 6 expected failures / 67 passed, proving missing icon common/More behavior and compact header boundary before implementation.
- `ui-green.log`: 3 files / 140 tests passed (AppSidebar, PageHeader, locale parity).
- Real Base UI native pin submenu smoke exercises Enter -> Pinned submenu -> resolved exact issue href -> Escape -> visible More focus.
- `ui-lint.log`: scoped ESLint passed.
- `ui-typecheck.log`: full views `tsc --noEmit` passed.
- `git diff --check` on the owned scope passed.
- pnpm printed the existing package-level configuration deprecation warning; no dependency changes were made.

## Remaining integration verification

Production Web and actual Electron renderer/native geometry/tooltip acceptance belong to the main session. WebAccountShell provider/external opener correction belongs to the geometry agent. Full application packaging/Main/installer compatibility is outside this UI increment.

## Stable SHA256

- `packages/views/layout/app-sidebar.tsx`: `a4c88832fb6076a110ba7b261794f5b0576546b530ba5aff3405c8d6a176d957`
- `packages/views/layout/app-sidebar.test.tsx`: `e5c2e95f926f8d741b992a0bbeb5edac73fd244a1da00753a6568bd9bbc30192`
- `packages/views/layout/page-header.tsx`: `f22f8c9fb81823f4e26c6d092ba0673a889536d941fd11ed0772fa5ff4b5b6e4`
- `packages/views/layout/page-header.test.tsx`: `b2ade67147cb29f2d39bdaf9835bf140a942ac78eb67fee5dd4916c48051b786`
- `packages/views/locales/en/layout.json`: `42f7068893560f840b7cd2bc609f409c13848c742098c2840d9d0c9fd7fdacb9`
- `packages/views/locales/zh-Hans/layout.json`: `165f07b9fbdd6a9cebc21e910c47b4bc890556d9f9ce7b0669b476d4c010910d`

## R4 tooltip followup

Read the implementation visual verdict before the scoped followup. The review found that the rail toggle, Help and account controls exposed names only through native titles. Composed the existing native controls with Base UI TooltipTrigger/TooltipContent; HelpLauncher gets an optional `tooltip` Boolean set only in the actual icon rail, preserving its standalone behavior. Four meaningful real-primitive tests failed before the fix; all 89 AppSidebar/Help/PageHeader tests then passed. The tests verify keyboard-visible Help/account/toggle names, hover-visible Help and retained Enter menu action, plus absence of nested button DOM. Scoped ESLint and diff check passed; final views typecheck recorded separately.

- `packages/views/layout/app-sidebar.tsx`: `7a26276bedccf14ed4921bcd8e229a24d61fc1cceab358fc6f6d0dfc232d1e92`
- `packages/views/layout/app-sidebar.test.tsx`: `f47dce95dc1b22df8b40da7662a165527eedfd03b7bead94ec8a965c0e57a5c5`
- `packages/views/layout/help-launcher.tsx`: `06a699292d84c7dc56c75e61b266bfeaa9c809f44835aa0fca3389dcb97c1702`

## Native trigger slot followup

The production Web r2 run showed the composed rail toggle had `data-slot="tooltip-trigger"` instead of the SidebarTrigger contract. A real Tooltip regression reproduced that exact attribute drift before the fix. Preserve the native `sidebar-trigger` slot explicitly on the rendering TooltipTrigger and native `dropdown-menu-trigger` slots on the Help/account composition. The existing SidebarTrigger click callback remains the action owner.

Verification: `ui-toggle-slot-red.log` shows the expected slot mismatch; `ui-toggle-slot-green.log` records 90 passing AppSidebar/Help/PageHeader tests, including real keyboard footer/toggle tooltips, hover Help menu action, single-control DOM, slot preservation and collapse/expand click transitions. `ui-toggle-slot-typecheck.log` and `ui-toggle-slot-lint.log` passed; diff check passed. Final production AppSidebar SHA256 `794e19aaa5397516c0999de4e0b6598daf8d117e95abf4b9fae5e0a27118df8b`; HelpLauncher `3e9c50eab22ef71b2f41c0da1468c46ffcc1ac013fa53b282972266590a544c1`. No further production changes or commits.

## Compact workspace Sheet scroll followup

The actual Electron 200-percent workspace Sheet leaves zero directory height when its large Search/Start-work header and footer both stay fixed. Inspected the native screenshot and visual verdict iteration 3. Compose the existing header and directory once; only the compact presentation puts both in one SidebarContent scrollport under one shrink-0 wrapper. Large expanded/icon presentation keeps its fixed header and directory scroller. The footer stays an original sibling and remains reachable; no providers, routing, draft state, query authority or creation dialog owner changed.

`ui-compact-scroll-red.log` proves the compact actions originally lived outside the only scrollport. `ui-compact-scroll-green.log` records 91 passing AppSidebar/Help/PageHeader tests, including shared scrollport/independent footer and existing native menu/tooltips. `ui-compact-scroll-lint.log` and `ui-compact-scroll-typecheck.log` passed; diff check passed. Actual positive scrollclientHeight, scrollHeight and painted/native click acceptance remain in the unchanged main-session journey. Stable AppSidebar SHA256 `7b2b8d39a5955a2c07649618152512ae91e9ffcbc28f3a22b5711b3f2fde4294`; test `d5c07f90697ef1df6f47f892d7a6af528172995ec9d7f3449abad272cd60596e`. No further production edits or commits.
