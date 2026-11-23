# Batch issue update outcomes

## Scope and wire

The existing issue batch endpoint commits each item independently. Keep
`issue_ids`, `updates` and `updated`; new results identify each requested index,
issue ID and `committed`, `rejected`, `not_attempted` or `unknown` state.
A committed prefix cannot be described as a transaction rollback of the batch.

`ApiClient.batchUpdateIssues(ids, updates, options?)` parses success and error
bodies with the shared batch schema. Freeze the requested IDs before transport.
Reject malformed, missing, reordered or duplicated item results, inconsistent
counts, unsafe revisions and successful HTTP responses containing failed items.
Valid old `{updated:n}` remains compatible; short counts cannot become full
success. An explicit request with no mutation fields may return zero/no_changes.

Completion controls are per item (`completion_bindings`); never share
`updates.completion` across the batch. Completion, project-change reason,
expected revision, handoff note and suppression controls are not cached Issue
fields or independent mutations. Do not manufacture a new operation key/retry.

## Error and cache ownership

`BatchUpdateError` retains the original ApiError status, body, auth-epoch and
retry metadata. Its parsed outcome describes known committed facts; absent or
malformed outcomes and transport loss remain uncertain. A non-success response
must not close confirmation dialogs or report full success. Do not infer a
historical write from the current issue state or implement automatic recovery
POSTs. The completion receipt API owns authorized historical recovery separately.

On mutation error, roll back only when the cached value is still this mutation's optimistic
snapshot; preserve newer WebSocket or fetched values. Then invalidate
all relevant workspace list/detail/children/derived caches to read actual server
state. Rolling back every item without refetch can permanently hide a committed
prefix. Successful complete batches retain the existing surgical refresh path.

## Shared surfaces

IssueSurfaceActions.batchUpdate returns BatchUpdateIssuesResponse. Pass that
result through adapters. Toolbar success counts use `updated`, not selection
length; zero produces no update-success message. Partial or unknown outcomes
preserve selection and existing failure feedback. Test mocks must return real
response shapes, not undefined interpreted as success.

## Validation and limits

Core owns the canonical wire/negative matrix and real QueryClient regression
for partial commits and newer updates; shared Views owns adapter/toast/dialog
behavior. Narrow protocol/component tests do not prove backend commit behavior,
authenticated routes, browser operation, global completion enforcement or M1.
The server must separately implement first-refusal stopping, non-2xx partial
results, unknown commit reporting and one notification over committed children.
