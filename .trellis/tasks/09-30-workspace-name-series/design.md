# Workspace name series design

## Approval and visual authority

The user approved the six-series proposal and requested a Trellis task to advance
it. Preserve the screenshot's existing single-column form, semantic tokens and
outline buttons. Add a split Random/menu control and a muted current-series line.
On narrow screens allow controls to wrap below the full-width input. Use Base UI
radio menu items with a visible selected mark and sample names. No new UI library.

## Boundaries and contracts

`packages/core/workspace/workspace-names.ts` owns the bilingual catalog and pure
generator. Series IDs: `workshop`, `computing`, `ai`, `space`, `nature`, `voyage`;
selection also accepts `all`. Export catalog, series IDs, selection type/validator,
and `createWorkspaceNameGenerator(random = Math.random)` returning a function
`(locale, selection) => { id, name, slug }`. A generator instance owns ephemeral
seen IDs and the last ID. Selected-pool exhaustion clears that pool's history;
avoid immediately repeating the last ID. All selects uniformly among series with
eligible entries, then uniformly within that series. Global exhaustion resets.

`packages/core/workspace/workspace-name-preferences.ts` owns a small Zustand store
using the existing StorageAdapter/defaultStorage with explicit client-effect hydration, with validated persisted
`seriesByUser` and `setSeries(userId, series)`. Export
`useWorkspaceNamePreferences` plus a factory for storage-isolated tests. Select
the current user's entry or workshop, never another account's most recent value.
Storage failure falls back to session state. Rehydration must reject malformed
values and avoid server/client hydration mismatches. Expose modules with explicit
package subpath exports, keeping UI strings out of the store.

`packages/views/workspace/workspace-name-picker.tsx` owns the reusable split
button and series menu. Props: selection, onSelectionChange, onRandom, disabled.
Series labels/action copy use the shared workspace locale namespace; examples
come from the bilingual catalog. It does not mutate form fields or own persistence.

`StepWorkspace` wires auth user -> preference, one generator per mounted form,
and the existing pending/disabled state. Keep two independent URL facts:
manually edited (protect from Random) and generated identity (protect English
URL from subsequent name typing). Random only writes URL when not manually
edited; untouched prefix continues to follow the displayed URL via applySlug.
The existing prefixTouched rule already protects manually entered prefixes.

Remove the celestial-only catalog and generator after migrating their sole
caller. Retain nameToWorkspaceSlug and conflict handling, with their existing
tests. New generator behavior has one canonical pure test layer in core.

## Validation and risk

Catalog tests enforce bilingual uniqueness, 20–30 entries/series and slug safety.
Deterministic RNG tests cover series selection, exhaustion and All sampling.
Store tests cover persistence, account isolation and malformed storage. Component
tests cover real menu keyboard behavior and form value/submission protection.
Browser checks exercise Chinese/English, narrow/desktop, menu and pending flows.

No API contract changes. Rollback removes the new picker/store/catalog and restores
the previous generator; unused personal preference storage is harmless. Existing
workspaces are never renamed or migrated.
