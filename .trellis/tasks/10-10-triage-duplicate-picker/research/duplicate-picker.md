# Duplicate target picker research

## Current behavior and reusable contracts

- `packages/views/triage/triage-action-dialog.tsx:124` stores the target string and separate search-modal state. Lines 184–204 parse the reference and protect against self-selection before mutation. Lines 270–301 render the input and nested picker.
- `packages/views/modals/issue-picker-modal.tsx:61` clears results for an empty query; the request uses a 300 ms debounce, `limit: 20`, and `include_closed: true`. Keep this shared modal outside the change's scope.
- `packages/views/triage/triage-ui.ts:62` exports `duplicateReference`, which accepts identifiers and workspace issue links. Its canonical parser tests live in `triage-ui.test.ts`.
- `packages/core/api/client.ts:1357` exposes schema-validated `listIssues` with explicit `workspace_id`, `q`, `limit`, `offset`, and sort parameters. `ListIssuesParams` is in `packages/core/types/api.ts:137`.
- `server/internal/handler/issue.go:1326` restricts ordinary listing to the selected workspace and formal admission. It only excludes closed issues when `open_only` is enabled. Line 1458 applies the title/number search filter; the helper starts at line 1690.
- `packages/core/issues/queries.ts:354` only flattens the first page of status buckets. It is unsuitable as the sole source for an exhaustive candidate browser.
- `packages/core/triage/queries.ts` is the appropriate existing query layer for triage-specific candidate options. Server data belongs in React Query and workspace-scoped keys; views must not call the API directly.
- `packages/views/triage/triage-action-dialog.test.tsx` currently tests decision error/retry and acceptance validation, while mocking the nested picker. Add actual picker interaction coverage at the views layer and keep pure parser cases in their canonical suite.

## Implementation constraints

- Prefer existing UI primitives and packages already installed. Preserve semantic tokens, touch targets, accessible listbox/combobox behavior, and the outer dialog's focus handling.
- Use bounded requests and reachable subsequent pages. Excluding rows locally must not make later pages unreachable. Keep query identity scoped to workspace and effective search text.
- Normalize a valid task link for searching; keep direct reference submission and server target validation authoritative. A title search is not a selected issue identity.
- New picker copy belongs in both `packages/views/locales/en/triage.json` and `packages/views/locales/zh-Hans/triage.json`; read developer conventions before editing copy.
- Regression coverage must include focus with an empty query, closed and excluded candidates, pagination, selection followed by explicit submit, keyboard dismissal, retry, and search races. Run relevant core/views tests, typecheck and lint; perform browser verification for popup focus/scroll/layout and persist the required visual verdict during UI iterations.

## Scope and delivery

The change is shared Web/Desktop frontend behavior using existing issue APIs. No schema migration or backend contract change is expected. A lightweight PRD is sufficient; implementation/check agents should read this evidence and inspect the relevant code before editing.
