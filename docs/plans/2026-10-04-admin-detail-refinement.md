# Administration detail and list refinement

## Goal and scope

Continue the user-approved recommendations after the overview refresh: improve audit readability, installation identification/status semantics, execution detail hierarchy, compact account/workspace rows and visible applied filters. The user has authorized implementation. Keep edits under packages/views/admin plus the two admin locale files, tests and task documents. Preserve all existing uncommitted changes.

No API, database, desktop route, global token or shared UI primitive changes. Do not run make up/gc/destroy or restart the existing desktop. Reuse web13493 and retained auditAPI18393. Shared styles must remain in the locally imported administration CSS Module.

## Design and invariants

1. Audit: translate verified known action/result codes, preserve raw codes inside details and unknown-code fallback. Summary shows historical actor and target identity. Applied means applied, not confirmed remote completion. Own-receipt authorization and full snapshots remain unchanged.
2. Installations: existing displayName first; shorten fallback UUID only visually, preserve full accessible identifier and copy/manual fallback. Status text and freshness remain independent; stale/unknown readings cannot imply current health/failure. Reuse existing state vocabulary.
3. Execution detail: title/status/source/attempt form the top summary. Time records, usage and technical context receive explicit sections; IDs remain copyable/reachable, permission-restricted content remains hidden. Controls keep confirmation, reason and concurrency behavior.
4. Account/workspace lists: reduce redundant compact-row whitespace; keep names/status/actions grouped and full IDs/details reachable. Maintain44px targets. Add missing account reset/refresh consistently with existing request and pagination semantics.
5. Filters: reuse a display-only applied-filter summary. Read only submitted URL filters, exclude cursor/implicit defaults. Standardize submit/reset button variants locally, preserve all validation/date bounds/cursor handling and route-specific scopes.

## Ownership and sequence

- Root: local presentation CSS/types, shared filter summary, account/workspace layout, locale integration, final verification.
- Audit lane: audit page, audit-specific tests, verified code dictionaries (report locale additions for root integration).
- Installation lane: installation list/common/identity presentation and tests.
- Execution lane: execution detail and tests.

First snapshot source/protected files and run existing administration regressions. Implement independent lanes; integrate locale additions serially. Review session delta against the snapshot. Run targeted tests/lint, locale parity, full typecheck and desktop isolation checks. Use existing read-only browser matrix/probes plus new fixtures for known/unknown audit values, full-ID copying, stale states, active filters and restricted execution content. Compare wide/narrow and light/dark screenshots before final corrections.

## Acceptance

- No lost baseline fields, permission gates or query semantics.
- Known audit labels readable; unknown values never silently replaced.
- Missing names remain honest; full IDs can be copied and manually selected on clipboard failure.
- Stale data is visibly stale without asserting a current healthy/failing terminal.
- No main overflow, adequate contrast and44px compact controls.
- Desktop source/shared-style boundary unchanged by this task; distinguish any unrelated concurrent edits.
- Report changed files, reused patterns, exact tests/browser evidence and native Electron visual limitation.
