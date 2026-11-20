# Design and implementation boundary

## Surface contract
Mode: Operate with first-use onboarding. The user's screenshot is the pinned visual reference at /var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/codex-clipboard-B1adQc.png. Code-led implementation uses genuine UI cards and geometric connectors, no generated imagery. The reference's small AI小队 marker is explicitly user-pinned, so preserve it despite the generic craft-floor eyebrow warning. Keep existing typography and surface tokens; use text-display for the hero and semantic brand/info accents for the demo. Do not introduce product-wide PRODUCT.md/DESIGN.md or restyle other routes.

First viewport: within the existing shell, spacious left aligned hero plus a 420–480px demonstration card on the right, with a subtle peek of the following card where width permits. The hero and primary CTA must fit a typical 1440×900 viewport; smaller content containers stack introduction above the demonstration, permitting vertical scrolling. Headline has two balanced lines in Chinese. The card is the signature: task assigned to a squad, connector to a leader, then selected member highlighted blue. Other demonstrations show role descriptions and handoff comments/results without pretending all agents run at once. Use compact example labels rather than real workspace data. Static cards must be non-interactive; only navigation controls and CTA links act.

## Code ownership and data flow
- packages/views/squads/components/squad-welcome.tsx (+ scoped module CSS/types only if useful): view-only introduction and three demonstration scenes. Local slide index state, semantic accessible controls, live current-slide summary; no new store or data fetch.
- packages/views/squads/components/squads-page.tsx: replace only the successful-empty directory presentation. Derive welcome visibility from actual query success, retaining prior error/loading priority and all tab/filter/list behavior. Avoid duplicate creation CTA in the welcome header if hero provides it.
- packages/views/locales/{en,zh-Hans}/squads.json: new welcome keys; delete stale empty-state keys only if globally unused.
- packages/core/docs/slugs.ts: add squads to the existing central docs slug map so offline docs routing and parity checks cover the new link.
- packages/views/squads/components/squads-page.test.tsx (+ focused welcome test if useful): update empty behavior and meaningful navigation/state/carousel regression coverage.
- Existing docs that explicitly promise direct empty-state creation-method buttons may receive a minimal same-task copy correction after repository search.

## Interaction and accessibility
Use the existing Button/AppLink and shared Lucide icon language. First slide is selected on mount; prev disabled initially, next disabled on last slide, dots can select any slide. Use labelled buttons, aria-current or aria-pressed for dots, visible focus and >=44px touch targets. Decorative previews are aria-hidden/inert and cannot duplicate interactive controls. Respect prefers-reduced-motion, no entry opacity gate or auto-playing carousel. Choose simple CSS geometry for connectors. Use container-aware sizing because sidebar/desktop tabs change the available width independently of viewport.

## Boundaries and risks
The working tree contains unrelated prior changes: leave them intact and commit/stage only task-owned files. Existing docs are internally served through docsPage, not public external URLs. Successful empty arrays alone justify onboarding: pending/disabled queries must not be mistaken for an empty list. The display is shared between Web and Desktop; no router package imports. English wrap/200% zoom and decorative preview overflow require browser verification.

Review confirmed an inherited API compatibility limit: `ApiClient.listSquads()` parses malformed successful responses through `EMPTY_SQUAD_LIST`. The view cannot distinguish that fallback from a valid empty array. This task preserves the explicitly excluded API/schema boundary; its empty-state acceptance proves Query pending/HTTP failure/filter behavior, not malformed-success detection.

## Rollback
Revert only the new welcome component, its conditional integration, translations and docs slug/notes. There is no persistence, data migration or compatibility layer.
