# Refresh deployed skill templates without restarting

## Problem

After an operator adds a skill under the configured server template directory,
the desktop template picker keeps showing its cached catalog until the client
restarts. The same shared catalog is used by the web client.

## Requirements

- Opening the template picker or selecting the deployment source fetches the current catalog.
- A visible picker discovers directory additions, edits and removals within 30 seconds.
- Background/closed pickers and the copy editor do not poll the catalog.
- Existing search, source and selection survive refresh where still applicable.
- Cached templates stay usable during network failures and retries.
- Already adopted or edited copies are never replaced by catalog refreshes.
- Preserve workspace isolation, cancellation, API shape and the existing server directory contract.
- Keep the change local to template freshness; no new dependency or database change.

## Acceptance

- Regression tests use the production QueryClient defaults and fail on the old cache policy.
- Tests cover reopening, focus/reconnect, foreground polling, hidden/inactive cleanup and retry recovery.
- A real picker test covers discovery from an empty deployment source without client restart and draft preservation.
- Relevant lint, typecheck and tests pass; the running local desktop receives the change.
