# Technical design

## Boundary and files

The shared automation catalog and empty-list caller own these gaps. Product edits cover `packages/views/autopilots/`, its two locale files, and the narrow origin-cleanup extension below. The parent owns task/spec/evidence files.

Expected files:
- `components/autopilot-template-catalog.tsx`: focus style/restore, concise guidance, recommendation, output and blank action.
- `components/autopilots-page.tsx`: pass the existing blank-dialog opener into the empty catalog and remove the competing empty header action.
- `components/template-create-autopilot-page.tsx` only for essential return wiring.
- Canonical adjacent catalog/list/template tests.
- `locales/en/autopilots.json` and `locales/zh-Hans/autopilots.json`.

## Focus contract

Use the existing route/tab-scoped view-state adapter with group/scroll. Save a pending template key before leaving. On the next ready gallery mount, restore scroll, then focus the card with preventScroll and consume the pending token. A stale/missing card falls back to the active filter.

Do not restore against skeletons, repeat on refetch, or steal focus from an already active dialog/another control. Preserve explicit All/zero. If all templates disappeared, fall back to the existing blank action. No new store, URL parameter, global observer or navigation abstraction.

Independent review reproduced two additional lifecycle cases within A02. Key the create shell by semantic picker/configure phase so its retained Back control cannot suppress return focus, including native Back. After a valid creation response, clear the originating gallery's pending focus before leaving; failures and unreadable success responses retain it.

The existing writer cannot target the empty-list origin while configuration is on another pathname. Extend `ScrollRestorationAdapter.setViewState` and `useViewStateWriter` with an optional third `routePathname` argument. Default writes retain current behavior. Web resolves `routePathname ?? renderedPathname`; Desktop writes the explicit path on the adapter-bound tab while preserving its current default routing. Use the trusted workspace path builder for the creation origin.

Additional required files are `packages/views/platform/scroll-restoration.tsx`, `apps/web/platform/scroll-restoration.tsx`, `apps/desktop/src/renderer/src/platform/tab-coordinator.ts`, and their adjacent tests. This is a concrete origin-cleanup requirement; no NavigationAdapter or store change is needed. Rejected alternatives: a global pending-focus store (parallel lifecycle state), URL flags (new routing surface), and clearing only the current path (leaves the empty-list token stale).

Use existing semantic foreground tokens for solid local focus rings, leaving shared `--ring` and Button defaults unchanged. Measure real computed contrast on all adjacent surfaces.

## Layout and copy

Operate mode; keep category colors, role-named typography, backend order, whole-card activation, and the three/two/one-column layout.

Reading path: title -> concise executor/default-plan guidance -> filters -> template title/recommendation -> description -> schedule/output/action. Move recommendation into the icon/title group and remove its extra footer row, fitting the existing header height where practical.

Use the catalog's existing blank-action slot for the empty-list entry as well as the creation picker. Keep it a readable secondary action with 44px compact/coarse height. The populated list retains New automation. Reuse the current create dialog.

Each prerequisite/default fact appears once above the cards; remove the distant footer duplicate. Verify output wording against patrol prompts: run_only does not pre-create a task but may create/update follow-up work. Unknown templates use conservative fallback wording.

## Compatibility and risks

No API/schema/dependency or navigation-contract change. Web/Desktop share the view and current restoration adapters. Keep success-based unfiltered empty detection and preset project/assignee behavior.

Primary risk is focus timing relative to mount, user activity and native Back. Cover delayed data, unavailable targets, no-refetch focus theft and actual Electron ordering. Inspect English/Chinese at narrow widths so the marker or guidance does not expand unnecessarily.

Rollback only task-owned view/locale/test changes, preserving unrelated work.
