# Source-task selection design

## Ownership and data flow

`CollaborationSourceInputs` owns the optional parent selector shared by new collaboration creation and source-field editing. Replace its separated search input/native select with the existing `PropertyPicker` and `PickerItem` primitives. The trigger uses the existing semantic tokens, a discoverable select affordance, and the selected identifier/title; its popover contains search, recent task rows, and local query-state feedback.

Keep `collaborationCreationOptions` and `collaborationFieldOptions` as the protected server-state owners. The creation/workflow callers forward the query pending/error/retry state. An empty search already loads candidates as part of the source options, so opening the list can render them immediately. Reopening clears the search through the established picker behavior. If debounce is needed, reuse an existing debounce pattern and keep pending results unselectable until the current search completes. Preserve existing source-authority cleanup and the user's durable creation draft on transient errors.

`CollaborationSourceSelection.parentIssueId` continues to contain only the selected UUID. Display state for a chosen candidate must remain coherent while search responses change; it must not create a persisted copy of server options or expose a previous workspace's candidate.

## Complete identifier search

`CollaborationSourceOptions` in the service already resolves the source workspace and formats parent identifiers through `util.ResolveIssuePrefix`. Compute that prefix once and pass it to `ListCollaborationSourceParents`. Extend its existing SQL predicates with an exact, case-insensitive comparison of the raw search to the resolved prefix plus `-` plus the parent number. Preserve title-substring, bare-number and UUID matching. Do not strip arbitrary prefixes: that can choose another workspace's same-number task or broaden identifier search into numeric title matches.

Regenerate sqlc's matching generated file. No migration, response shape, query cache identity or membership contract changes are required.

## Change boundary and alternatives

- Shared UI owner: `field-inputs.tsx`, creation/workflow query-state wiring, supported collaboration locale JSONs, and focused shared component/form tests. `PropertyPicker` also gains an optional `searchMaxLength` passthrough so this control preserves its existing 200-character input bound without imperative DOM edits; existing callers keep the default.
- Server owner: the source-fields query, its generated sqlc file, source-fields service and focused service/handler tests.
- Main-session ownership: task artifacts, spec capture, browser evidence and final integration/commit.
- Retaining two disconnected controls leaves results undiscoverable. A separate dialog or generic ordinary-issue query introduces a second discovery/authentication path. The existing searchable property picker and protected source-options reads satisfy the requested behavior with a smaller change.
- Existing candidate admission, terminal-task inclusion, collaboration parent inclusion and the latest-200/truncation behavior remain the source contract.

## Verification and rollback

Behavior tests must fail against the original UI or SQL before their corresponding repair. Cover actual picker primitives, query races/retry and exact parent identity in the correct shared package. Backend tests exercise the database-backed query through existing isolated fixtures. Browser inspection uses an owned production Web build and fixture account, retaining narrow/wide captures in both locales. Rollback is a focused revert of this task's commit; no data/schema changes need recovery.
