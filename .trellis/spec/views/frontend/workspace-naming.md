# Workspace naming

## Catalog and ownership

The bilingual catalog and session generator live in
`packages/core/workspace/workspace-names.ts`. Series are workshop (default),
computing, AI, space, nature and voyage, with All as a selection mode. UI labels
live in the views workspace locale namespace; examples come from the catalog.
Do not reintroduce a separate celestial-only generator in views.

One generator belongs to one mounted creation form. Its seen-name history is
ephemeral; switching series keeps history and exhausting a pool resets only that
pool. All chooses among non-exhausted series before choosing a name, so catalog
size does not weight a series more heavily. Never persist generation history.

## Personal preference

`workspace-name-preferences.ts` owns a Zustand store keyed by account ID, using
the platform StorageAdapter. This preference is neither workspace data nor a
server field. Anonymous selections remain in the form. Client effects explicitly
hydrate persisted preferences; the initial server/client render uses the default.
Malformed stored entries are discarded, and storage failure must not block form
use. A series selection never mutates name, URL or task prefix.

## Field ownership

Keep manually edited URLs distinct from generated URLs. Random can replace a
generated URL but must preserve a manually edited URL. Typing into a generated
name retains the catalog's stable English URL; typing a name before randomizing
can still derive a URL through `nameToWorkspaceSlug`.

The task prefix follows the displayed URL until edited independently. Apply every
automatic URL update through the existing prefix-aware setter. Do not reset the
manual flags when choosing a series or randomizing. Creation submits exactly the
displayed values and keeps the server's conflict validation authoritative.

## Verification

Pure generator and preference matrices belong in core tests. Picker tests use
real Base UI radio-menu primitives for keyboard navigation and focus restoration.
StepWorkspace tests cover wiring, pending/disabled states and field ownership;
do not duplicate the entire catalog matrix through DOM rendering. Browser checks
cover bilingual menus, narrow wrapping and preference restoration after reload.
`e2e/workspace-name-series.spec.ts` also creates a workspace through the real API
and verifies its task identifier uses the manually selected prefix. After a
manual URL edit, assert Random still replaces the name as well as preserving the
URL and prefix; checking only the preserved fields would let a no-op pass.
