# Research: creation, refusal, identity and search contracts

- Query: Find the smallest safe fixes for delayed creation feedback, refusal reasons, recipient identity and parent-query debounce.
- Scope: internal; read-only source/spec inspection, no API calls or processes.
- Date: 2026-11-02

## Files found

- `packages/views/collaboration/creation-form.tsx`: source ownership, mutation callback, draft UI and current un-debounced parent search.
- `packages/core/collaboration/creation.ts`: immutable original request, confirmed rejection and successful bundle retirement.
- `packages/core/collaboration/creation-schema.ts`: strict creation fields and 26 recognized refusal codes.
- `packages/core/collaboration/delivery-draft.ts`: immutable operation records and synchronous subscription notifications.
- `packages/core/collaboration/delivery-form.ts`: form observes core retirement instead of resurrecting a consumed draft.
- `packages/core/collaboration/delivery-access.ts` and `access.ts`: source epochs, account/API identity and private-content cleanup.
- `packages/core/collaboration/schema.ts`: exact/search account projections, no username or email returned.
- `packages/core/collaboration/recipient-sections.ts`: case-insensitive account identity deduplication.
- `packages/views/collaboration/account-picker.tsx`: existing 250ms trimmed-query debounce and exact-lookup interaction.
- `packages/views/collaboration/labels.ts`, `packages/views/iterations/labels.ts`: existing localized switch-with-default pattern.
- `packages/views/collaboration/creation-form.test.tsx`, core `delivery.test.ts`: canonical DOM wiring and core recovery/epoch matrices.
- `server/internal/service/collaboration_creation_result.go`: durable business rejection whitelist and HTTP status mapping.
- `packages/core/api/client.ts`: strict HTTP-200 own-result correlation, source correlation and request cancellation.

## Delayed success and source locking

The source `<select>` is currently always enabled (`creation-form.tsx:65`). Changing it changes the key of `SelectedCreationSource` (`:71`), unmounts `CreationForm`, and sets its `active` ref false (`:96-100`). The original request still runs. Core confirms the result and retires the entire exact draft bundle (`creation.ts:112-115`), as required even after an unmount. The view's guarded callback then intentionally declines `onCreated`. This is the precise path behind the audit's missing navigation and already-cleared recovery record.

Do **not** move retirement out of core or remove `active.current` / `isCollaborationScopeCurrent`. Those protections prevent revived consumed files and account/API contamination. `delivery-form.ts:16-20` explicitly observes retirement across remounts. `access.ts:44-46,101-128` fences account, API instance/session and access epoch; `delivery-access.ts:62-76` additionally fences source access.

The parent already subscribes to operation records (`creation-form.tsx:43-48`) and prefers the source of a contentful pending creation/upload during initialization (`:49`). Those records are saved **before** transport (`creation.ts:63`, `delivery-draft.ts:77-91`). They remain through unknown outcomes. However, records alone have a success interval: core retires them before the mutation callback has completed. A lock should include current-scope pending create/recovery mutations as well as the applicable unresolved contentful request/upload records. React Query's `useIsMutating` is an existing parent-level pattern (`views/iterations/iteration-recovery.tsx:21`, `iteration-settings-tab.tsx:43`) and avoids a new client store or duplicated mutation owner. Match current collaboration account/API prefix plus creation operation keys; unrelated account/issue mutations must not lock this picker.

Keep content-free (`input:null`) recoveries usable independently of source membership (`creation.ts:96-100`). Do not indefinitely disable all source selection merely because an old redacted recovery exists: membership loss deliberately retains that entry (`delivery-draft.ts:132-138`), and users may still create in another available source. Its **in-flight** recovery must nevertheless lock source selection until the result settles. Existing top-level recovery is account-owned (`creation-form.tsx:53-55`).

Source-authority loss during a request is separate from voluntary source switching: authoritative directory reconciliation may clear private input and replace a record with content-free recovery (`delivery-access.ts:21-45`). Retain this behavior. Fixing the manual switch does not license overriding a fresh denial or keeping an unauthorized form mounted.

### Named regressions

1. Two available sources; begin delayed creation, verify source selection is disabled and changing it cannot replace the title/form; release a valid original response and assert exactly one completion/navigation.
2. Unknown response leaves immutable request and draft/files available; remount prefers its source; recovering uses the original GET/key with zero replacement POST.
3. During delayed original-key recovery (including content-free parent recovery), source cannot change; allow again after a confirmed rejection when authority remains valid.
4. Account/API replacement or source denial before delayed response continues to fence the old callback; canonical epoch/receipt matrices remain in core, with a narrowly named view wiring regression if altered.

## Refusal presentation

Only `CollaborationCreationRejectedError.code` represents a proven refusal (`creation.ts:14-15,101-110`). A raw POST code alone is insufficient: core checks the actor/source/request-correlated own-result GET, then rechecks source authority before retiring only the pending record. Rejected forms/files stay intact; the next explicit submit obtains a new key (`creation.ts:101-110`, `delivery.test.ts:232-255`). Unknown/malformed/202 results remain uncertain (`api/client.ts:1005-1023`, `delivery.test.ts:276-292`). Presentation must consume the typed error without changing this outcome protocol.

The 26 supported codes (`creation-schema.ts:42-49`) are:

`account_unavailable`, `collaboration_unavailable`, `invalid_attachment_selection`, `attachment_not_found`, `attachment_scope_mismatch`, `attachment_draft_consumed`, `parent_issue_not_found`, `project_not_found`, `triage_review_required`, `issue_limit_reached`, `status_unavailable`, `invalid_priority`, `invalid_start_date`, `invalid_due_date`, `invalid_labels`, `invalid_properties`, `property_archived`, `invalid_property_value`, `properties_too_large`, `invalid_stage`, `parent_cycle`, `iteration_disabled`, `iteration_not_found`, `iteration_revision_conflict`, `iteration_history_move_unsupported`, `iteration_validation_failed`.

Server mapping agrees (`collaboration_creation_result.go:39-90`): limit is HTTP 402, triage review HTTP 409, missing parent/project HTTP 400, unsupported property/iteration/attachment choices have recognized statuses. Generic service unavailability is **not** automatically a settled `collaboration_unavailable` refusal (`:62-75`).

`CollaborationQueryState` (`views/collaboration/query-state.tsx:5-9`) classifies query failures/denials; it is not a creation-refusal mapper. `useCollaborationLabels().reason` (`labels.ts:27-80`) maps resource/execution codes and otherwise reports unavailable, also unsuitable as-is. Reuse the existing localized switch-with-safe-default pattern, adding a task-specific method or a small local mapping rather than an API parser or broad global-error abstraction. `useIterationLabels().validation` (`iterations/labels.ts:22-51`) supplies wording precedent for revision/triage cases. Ordinary issue creation invokes an upgrade prompt for limits (`modals/create-issue.tsx:728-730`), but no new modal or commercial action is needed here: state the limit and require an administrator to adjust the applicable condition. Do not tell users a simple retry will resolve quota, archived properties or missing recipients.

Use existing creation draft-preservation message plus the concrete cause/action, in both supported locales. Unknown future runtime codes require a factual fallback; normal parser behavior still leaves unknown wire codes uncertain.

### Named regressions

- Actual mocked POST `issue_limit_reached` plus correlated HTTP-200 rejection result renders the limit and next action, keeps the title/files, and performs no automatic resubmit.
- Existing `account_unavailable` DOM case (`creation-form.test.tsx:120-161`) should gain the recipient-specific explanation while retaining explicit identical-input/new-key resubmission.
- Other code-to-copy cases belong in one canonical mapper/label test if a pure helper is introduced; do not repeat the entire core receipt matrix through a DOM mount.

## Recipient identity and privacy

Search rows permit **only** `user_id`, `name`, `avatar_url`, `external:boolean|null` (`schema.ts:46-56`). Exact lookup and persisted selection similarly provide these four fields, with known membership on exact account summaries (`schema.ts:15-33`). Username/email can be accepted as an exact-query input, but they are not returned identity fields. Rendering invented, fetched or inferred username/email would expand the contract and may expose data outside permitted projection.

Use the already-projected account ID as a secondary identity in the row, exact-result card and selected single-account trigger/confirmation. Full normalized UUID is unambiguous, including collisions in the first few characters; a short prefix alone is not sufficient. Keep it wrap/overflow-safe and available to the accessible name/description. Preserve `external === true` badge behavior and never infer source membership from a null value (`schema.ts:65-81` masks membership for non-source members).

Selection and deduplication remain case-insensitive user-ID based (`recipient-sections.ts:13-24`, `account-picker.tsx:41-43`). The form's request sends the chosen `handler_user_id` (`creation.ts:50-53`), so no draft/schema/API change is necessary. The self row uses `scope.userId` (`account-picker.tsx:45-47`), and may retain its special self wording.

### Named regressions

- Two internal accounts with identical names and similar UUID prefixes both expose distinct IDs in candidates; selecting one retains that same ID in the trigger/confirmation and POST payload.
- Exact-result card also exposes its permitted ID; external/null membership behavior and members-only exclusion remain unchanged.

## Parent-search debounce and cancellation

`SelectedCreationSource` currently uses raw `parentSearch` in the query immediately (`creation-form.tsx:76-83`), while account search already keeps raw input and trimmed query separate with a 250ms timeout cleanup (`account-picker.tsx:69-70,83-91`). Reuse that small effect pattern in `SelectedCreationSource`: immediate raw input, delayed trimmed query, clear timer on value change/unmount. The component's existing scope/source key (`creation-form.tsx:71`) resets the pending search at source/API/viewer changes.

Keep `collaborationCreationOptions` unchanged: its source/query identity (`creation.ts:18-29`), supplied AbortSignal (`:23-25`), focus/mount authority refresh and 15-second poll (`:22`) remain important. `api/client.ts:1036-1042` sends `parent_search` only for a specific source and validates the returned source. Preserve placeholder previous data while refreshing and `optionsPending` gates for iteration revision adoption. Debouncing should neither suppress permission refresh nor reuse the prior source's response.

### Named regressions

- Rapidly enter `协作任务改进`, verify raw text is visible immediately, zero intermediate parent requests during the delay, then one query for the settled full text after 250ms.
- Change source before the delay expires; no query with old text starts under the new source, and late old source responses cannot populate the current options.
- Existing authority refresh/abort and draft ownership tests remain green; no auto-submit is introduced.

## Related specs and references

- `.trellis/spec/core/frontend/collaboration-workbench.md`: intent-before-transport, own-result retirement, source/account fences, recipient projection and canonical test layers.
- `.trellis/spec/core/frontend/api-client-auth-epoch.md`: stale credentials/API endpoint cancellation cannot affect a replacement session.
- `.trellis/spec/core/frontend/global-scope-navigation.md`: explicit account issue destinations and account-owned navigation.
- `CLAUDE.md`: server state belongs to React Query, shared client drafts to core, shared views preserve Web/Desktop parity, typed server enums include a default.
- No unfamiliar external API was introduced; no external references or live endpoint inspection was required.

## Caveats / Not Found

- This research does not reproduce behavior in a browser or execute tests. The prior copied audit is the behavioral evidence; fresh verification belongs to implement/check.
- No allowed username/email response projection exists. Full account-ID display is the minimal unambiguous fix within current API permissions.
- A blanket lock on all idle redacted recoveries could strand creation after source membership loss. Lock active recovery plus applicable contentful unresolved requests; retain independent content-free recovery.
- Core receipt and retirement semantics should remain unchanged. The delayed source-switch fix belongs in the view's source-selection boundary.
