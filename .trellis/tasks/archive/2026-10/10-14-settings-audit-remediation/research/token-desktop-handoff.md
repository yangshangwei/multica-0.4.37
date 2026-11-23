# Token / desktop lane handoff

- F11/F12/F16 are owned entirely by the token lane, including TokensTab and its new regression suite. Existing list/create/revoke methods are in core/api/client.ts; schemas should live in core, use parseWithFallback and redact credentials. Types are in core/types/api.ts:565–581.
- PersonalAccessToken has id/name/token_prefix/created_at plus nullable expires_at/last_used_at. Creation extends this with the one-time token. Malformed lists should show a recoverable read failure, not a route crash or a fabricated successful creation. Keep tolerant optional-field handling where the API contract allows it.
- core/api/client.ts is concurrently dirty. Only change PAT-specific imports/method hunks; preserve the captured preexisting patch and all newer unrelated changes. No full-file rewriting.
- Existing getIssue in the API client demonstrates parseWithFallback<T|null>(..., null) followed by an explicit malformed-response error. PAT diagnostics must use redact:true.
- Existing token confirmation restricts only Done; Escape/close/backdrop must be protected too. Revocation must stay pending once and keep target/dialog on failure. Do not persist the one-time credential.
- Desktop scopes: daemon-settings-tab.tsx has two unnamed switches; desktop-behavior-settings-tab.tsx has one unnamed Select; updates-settings-tab.tsx needs a stable live result. Preserve platform-only placement and existing main IPC behavior. Actual updater contracts use automaticUpdates, and check result {ok,available}; the old audit harness was already corrected.
- Base UI Switch IDs may be placed on hidden input elements. Browser assertions must use role/name and aria-disabled/checked state, not a hidden input ID.
- No real credentials, revocation, service writes or updater installs in tests. Reuse the isolated audit fixtures.
- Parent owns shared locale JSON. Write research/locale-token-desktop.json and notify parent for merge before typechecking.
