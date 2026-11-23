# S5 platform integration

The Web administration area mounts the shared page at `/admin/collaboration`.
The existing Desktop account tab uses
`/account?view=current&administration=collaboration`, optionally with an exact
issue UUID. The canonical parser rejects content anchors and a participating
view mixed with administration. Its memento key retains the administration
discriminator; entering this mode never mounts ordinary collaboration reads.

The shared page uses its dedicated current password/super-admin/organization
access check. Normal account views show its entry only after that check. The
new read envelopes retain the organization authenticated in the same server
transaction, rather than trying to correlate old data with a later `/admin/me`.

## Boundary reuse plan

The existing Web `AdminSessionBoundary` already synchronously removes stale
administration queries/mutations on account, credential and server changes.
Its four existing regression cases protect this behavior. Move the unchanged
component to `packages/views/admin/`, keep the Web platform export as a thin
re-export, and mount that same component under the Desktop Query provider above
routes. This adds the required native boundary without a duplicate cache
implementation. Run the existing boundary tests and native account flow after
integration; no storage key or ordinary workspace cache semantics change.

The four boundary regression cases now live beside the shared implementation
in `packages/views/admin/admin-session-boundary.test.tsx`. Run them with
`pnpm --filter @multica/views test admin/admin-session-boundary.test.tsx`.
The old Web-only behavior test was removed; the thin platform export and native
mount receive their wiring evidence from the production Web/Electron journeys.

The API and path boundary checks live in core `admin/collaboration-api.test.ts`,
`admin/collaboration-schema.test.ts`, and `paths/account.test.ts`. The reason and
status limits use the same UTF-8 byte semantics as the Go handlers. A Chinese
334-character reason (1002 bytes) was reproduced as invalid before transport.

Current scope: platform integration in progress; no S5 browser/native pass yet.

## Native fixture reuse plan

S5 must run in a separate password environment, while the existing account
spec's beforeAll seeds legacy accounts. Extract its unchanged native launch,
deep-link, response-evidence and capture helpers into an E2E fixture module so
the new password spec can reuse the production IPC harness without importing
another spec and executing its setup. Keep the existing account journey as the
regression check against its fixed legacy verification build; no production
code or native protocol changes belong to this extraction.
