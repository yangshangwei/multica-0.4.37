# Settings recovery, persistence and accessible feedback

## Scope / trigger

Apply this contract when changing shared settings, their core queries, or the
desktop settings slots. Query recovery must preserve authorized work while
discarding a denied snapshot. Confirmations and save feedback must describe
the result of persistence, including failed and overlapping operations.

The settings directory and provider URL contracts remain in
[component guidelines](./component-guidelines.md). General retained-content
rules live in [desktop core accessibility](./desktop-core-accessibility.md).

## Signatures and owners

- `withQueryAccessBoundary<TData>(queryFn: (context: QueryFunctionContext) =>
  Promise<TData>)` in `packages/core/query-access.ts` returns a query function
  with the same result type. Use it on each protected read whose data a
  settings surface retains, including subordinate lists and paginated pickers.
- `getSettingsQueryState(query)` in
  `settings/components/settings-query-feedback.tsx` returns
  `{ hasData, failed, accessDenied }`. Its input is the existing Query result,
  not a copied local state object.
- `SettingsQueryFeedback({ query, loadingMessage, errorMessage, refreshMessage })`
  renders loading or error/Retry as a stable sibling of retained content.
- `useAutoSave<T>(options).saveNow(candidate): Promise<boolean>` settles after
  that candidate's serialized write. `flush(): void` retains its existing
  blur/debounce use. Optional `isValid(candidate)` validates the candidate
  being saved rather than a different current draft.
- `personalAccessTokensOptions(userId)` in core auth owns the account-scoped
  token list. `GET /api/tokens` and `POST /api/tokens` are parsed in ApiClient
  with the schemas in `api/personal-access-token-schemas.ts`.

## Contracts

### Reads and access loss

`data !== undefined` means a response exists; a successful `null` provider
configuration is loaded but unconfigured. An empty array is meaningful only
after a successful read. Pending or failed membership must not become a
confirmed non-admin role or enable workspace Leave.

The feedback helper retains a previous error while a cold Retry returns Query
to pending, using `errorUpdatedAt > dataUpdatedAt` during fetching. Disable
Retry while fetching and call `refetch({ cancelRefetch: false })` so another
activation cannot cancel and restart the same request. Transient refresh
failures keep the actual editor/input nodes and their drafts mounted.

An active `ApiError` with status 401 or 403 overrides cached content. The core
boundary captures the exact Query before awaiting and uses
`query.setData(undefined, { manual: true, updatedAt: 0 })` to clear both visible
data and TanStack Query's cancellation rollback snapshot. `setState` alone
clears only visible data: unmounting during an automatic retry can restore the
denied snapshot. Do not access Query's private fields or change global retry
policy to work around this. Ignore aborted requests and stale credential
responses; they cannot erase newer authorized data. Successful subsequent
fetches may restore the content.

Apply the same rule to permission reads and the data they gate. A protected
invite or sharing form must not become usable because its independent query
still has data after membership or its own access was denied. Close a pending
confirmation when its authorization is lost.

DingTalk's shared group panel receives the original groups Query and uses the
existing core inactive-group pagination options for workspace and agent scopes.
Keep active and inactive rows mounted across transient refresh errors, with
their independent feedback. Permission joins such as `agentListOptions` must
also discard denied snapshots. A denied inactive list invalidates an inactive
Forget target, but must not block a still-authorized active group's action.

Preserve contextual inference when wrapping query functions. A selector or
data-reading callback may require binding the wrapped function before
`queryOptions`; a paginated query can retain its typed outer context and call
the boundary inside that function. Do not add type assertions to hide
inference loss.

Status catalog rows receive category display labels from their owning query.
Do not mount new catalog observers solely to format a category while the
parent alternates between loading and error: retry-on-mount can create an
unbounded request loop.

### Saves and confirmations

`saveNow` returns true only after the candidate is persisted or already current.
Invalid/disabled candidates, failed writes and queued candidates replaced by a
newer request return false. Only the latest queued candidate is written next;
its caller waits for its own outcome. A requested return to the old value
still queues behind an in-flight change. Repository removal awaits this
result before deleting the row or clearing its confirmation, including the
last row and an unsaved blank row.

Every audited submit entry point shares a synchronous pending guard, including
Enter, buttons and confirmation actions. Keep the target and a retry path on
failure; prevent dismissal while a destructive confirmation is pending.
Timezone and GitHub preference controls lock competing operations until the
accepted write settles. Disabled controls must reflect the actual primitive's
semantics, which may be native `disabled` rather than `aria-disabled`.

Required names expose an associated validation error and unsaved status. Keep
all active form values through invalid-name correction and save the complete
retained draft together. This does not introduce cross-route draft persistence.
Avatar save callbacks must propagate persistence rejection to
`AvatarUploadControl`; swallowing it falsely closes the crop dialog and
announces upload success.

### Personal access tokens

List records require string `id`, `name`, `token_prefix` and `created_at`;
the ID must be nonblank. Optional nullable `expires_at` and `last_used_at`
normalize to null. List parsing strips extra fields, including any credential
accidentally supplied by a newer server. A create response additionally
requires a nonblank `token` and retains its original credential bytes.

Both endpoints use `parseWithFallback` with redacted diagnostics. Invalid JSON
syntax is remapped before its message can expose response text. An unusable
create response is an explicit `invalid_personal_access_token` error and
invalidates the list for inspection; never automatically create another token
after an ambiguous result.

Keep the one-time secret in the current view's local state. It must not enter
QueryCache, MutationCache data/variables, persisted preferences or logs, even
briefly. Core auth owns the list options and create/revoke mutation hooks;
TokensTab refetches, filters or invalidates that account's list after settlement.
Credential writes use `retry: false` and `networkMode: "always"` so they fail
immediately when offline instead of queuing for a later session. Successful token-result dismissal requires the saved-token
acknowledgement for Done, Escape, close button and backdrop alike. Revocation
settles once and keeps a failed confirmation available for retry.

### Layout and accessible feedback

Catalog columns respond to available content width, including the main
sidebar; viewport width alone cannot prove that trailing actions fit. Bound
the property editor's height locally with a scrollable body and stable
header/footer, preserving access to all options and final actions.

Use purpose-specific labels for Base UI Select/Switch controls. Their hidden
native inputs are not browser interaction targets: verify the visible role
and accessible name. Shortcut recorder descriptions expose the current binding
or unassigned value even when decorative keycaps are aria-hidden. Manual
update results use a stable status live region without moving focus. Keep its
busy check button natively focusable with `aria-disabled` plus a synchronous
handler guard: native Chromium blurs a button changed to `disabled`, although
jsdom does not. Do not restore focus on settlement; the user may have moved it.

Use `warning-foreground` and `success-foreground` for small status text. Avoid
opacity on the complete row or badge. Measure the actual foreground/background
pair in both themes; text contrast must reach 4.5:1.

## Validation and error matrix

| Situation | Required result |
| --- | --- |
| Initial read pending / failed | Loading / named error and reachable Retry; no false empty or denied state |
| Authorized data then 500 | Same content/input nodes and draft; refresh error and Retry |
| 403, then Retry returns 500 | Denied data and dependent management controls remain absent |
| 403, unmount during auto retry, remount | Cancellation cannot restore the denied snapshot |
| Cancelled or stale-auth request rejects | A newer snapshot survives |
| Invalid name then correction | Retained complete form saves after correction; no earlier success feedback |
| Confirm while already pending | One write; target remains until successful settlement |
| Removal fails or queued candidate is superseded | `saveNow` returns false; do not remove the row on that result |
| Token list is malformed | Recoverable read error, without route crash or credential logging |
| Token create result is unusable | No success dialog; refresh list and explain ambiguity |
| Unacknowledged token result receives dismissal | Dialog and secret remain available |

## Good / base / bad cases

- Good: a focused MCP rename survives a 500, but a denied read removes it and
  a later transient error cannot bring it back.
- Base: a successfully empty label list shows the normal empty-state action.
- Bad: `data ?? []` is used to infer an empty or non-admin state after a failed
  read, or a form's confirmation closes before its write result is known.

## Tests required

- `core/query-access.test.ts`: canonical denial, cancellation, auto-retry
  rollback and stale-auth matrix using real QueryClient/QueryObserver.
- Settings recovery component suites: production query wiring, initial versus
  cached errors, same-node/focus retention and independently denied subqueries.
- `use-auto-save.test.tsx` and repository recovery: candidate settlement,
  replacement, serialized return to an old value and failed last-row removal.
- Account/workspace/member suites: actual upload control rejection, required
  name correction, retained drafts and duplicate Enter/confirmation guards.
- Core PAT parser and token component suites: malformed/redacted responses,
  cache secrecy, all dialog dismissal paths, pending/retry and account identity.
- Desktop settings suites: named controls and live update results.
- Real Electron evidence: 900px and 450 CSS px pointer/keyboard reachability,
  12-option dialog, AX names/descriptions, both-theme contrast and native zoom.
  Fixtures prove frontend behavior; they do not prove real backend persistence,
  native Windows behavior or spoken screen-reader output.

## Wrong versus correct

```tsx
// Wrong: an error replaces a retained editor and destroys its focus.
if (query.isError) return <ErrorView />;

// Correct: the feedback is a stable sibling and access controls retention.
const { hasData } = getSettingsQueryState(query);
return <>{feedback}{hasData && <ExistingEditor data={query.data} />}</>;
```

```ts
// Wrong: clearing the target on dispatch loses the retry path.
void saveNow(remainingRepositories);
setDeleteTarget(null);

// Correct: clear only when that candidate has persisted.
if (await saveNow(remainingRepositories)) setDeleteTarget(null);
```
