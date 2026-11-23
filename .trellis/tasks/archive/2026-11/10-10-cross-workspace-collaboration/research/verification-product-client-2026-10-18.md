# Product frontend review — 2026-10-18

Independent `client_check` review of the continuation at `4dd643126`.
The parent task remains in progress. This reviewer does not own server changes,
service builds, browser processes, commits, or the implementer's active E2E files.

## Review plan

1. Load task contracts and package specifications; review tracked and untracked
   frontend files, including API boundary schemas and native download wiring.
2. Trace creation, upload, comment and delivery through original-key recovery,
   draft consumption, reload, account/source changes and access revocation.
3. Review resource, Switch and participant integration, accessible controls,
   transient failures, and attachment preview/reference behavior.
4. Report concrete findings before editing active modules; reproduce local
   defects with focused tests, make bounded fixes, and run relevant tests,
   lint and typecheck. Do not duplicate the backend review.
5. Review final Web/Electron E2E wiring and record browser/native evidence only
   after the parent runs matching production builds.

## Findings and changes

### Fixed: recipient refusal was treated as source access loss

`packages/core/collaboration/creation.ts` passed a creation POST's
`403 account_unavailable` directly into the source access guard. The backend
uses this code for an unavailable recipient as well as an unavailable creator;
it does not establish loss of source membership. The guard removed the form and
uploaded-file selections and reduced the request to a content-free recovery.

Creation now performs the explicit source-options authority read for this
ambiguous refusal. A valid or unavailable recheck retains the original request
and inputs. An actual source denial still clears private inputs. It does not
make a new request key, change the recipient, or repeat a write automatically.
Two regression cases first failed and then passed.

### Fixed: cancellation and asynchronous upload preparation

`packages/core/collaboration/attachment.ts` had no caller abort signal, and
staging captured its access fence only after asynchronous file hashing. The
shared picker also left the upload running when its form was closed or switched.

The delivery guard now exposes its captured-current check to asynchronous local
steps. File hashing and response processing check that same scope before
publishing draft state. Signals reach upload/recovery fetches, and the shared
picker aborts on explicit cancellation and unmount. Cancellation retains a saved
original upload key because the server may already have committed it. Completed
uploads release their in-memory File reference. The cancellation button is
explicitly non-submitting and has English/Chinese labels.

Core regressions cover a response arriving after cancellation, original-key GET
recovery, and an issue projection change during hashing. Shared component tests
cover cancel-button/unmount wiring without repeating the core recovery matrix.

### Fixed: attachment absence could erase authorized issue content

`packages/core/collaboration/attachment.ts` interpreted a file byte/capability
`404 not_found` as proof that the whole issue was inaccessible. The backend uses
that result for a missing file as well. Read failures now recheck issue authority
before clearing issue data. File absence preserves an authorized issue; a real
issue denial still fences and clears it. Both cases have a regression.

`packages/core/api/client.ts` also treats unstructured missing private-upload,
draft-list and recovery routes as unavailable. An older server's missing route
cannot erase the saved original upload or impersonate source access loss.

### Fixed: successful comments could retain a consumed draft

`packages/core/collaboration/comment.ts` removed the comment operation on success,
while delivery-bundle retirement depended on the original mounted composer's
callback. A form remounted before that callback could retain the consumed draft
and already-sent text.

The core success path retires only the submitted bundle before dropping the
comment operation. A local-only form identity also covers plain comments, whose
wire request correctly omits `draft_id`. Delivered same-issue file IDs may be
prepared without a draft ID. Input-aware attachment receipt matching from the
implementation lane is retained.

`packages/core/collaboration/delivery-form.ts` observes removal of a previously
persisted form and creates a new local form rather than falling back to the old
one. Explicit reset retires the stored form before replacing its local state.
Regressions cover comments with/without attachments, preservation of another
bundle, and a remounted hook observing retirement.

### Added verification: upload destination is required metadata

`packages/core/collaboration/delivery-api.test.ts` now rejects missing, malformed
or wrong-context `intended_issue_id`. Pre-issue uploads require explicit null;
existing-issue uploads require the exact issue UUID. A delivered creation file
may retain a null intention while its permanent `issue_id` is fixed.

### Fixed: durable creation refusals now permit explicit resubmission

The parent assigned the server contract to `creation_rejection` and the client
integration to this reviewer. The new pure schema validates the exact
`request_id/source_workspace_id/actor_user_id/outcome/code` result and its closed
rejection-code set. API and core checks correlate it to the original request,
source and viewer. Only HTTP 200 establishes this completed result; even a 202
with a matching rejected body remains unknown. Successful creation POST/GET
responses remain unchanged.

Creation recovery first tries the original issue GET, then consults the own
content-free `/creations/{requestId}/result` when that lookup is unsuccessful.
A POST error alone never retires a request. Only a proven durable rejection,
followed by source revalidation, removes the pending creation record while
retaining the form and unconsumed files. Actual source loss still clears private
inputs; malformed, missing, pending and unknown-commit results remain pending.

The form displays a confirmed rejection and waits for an explicit user submit.
That submit may use exactly the same input/files with a new key after conditions
change. GET, reload, reauthorization and gate recovery never POST. No rejected
input snapshot, comparison gate or cached server receipt is persisted.

Core and component regressions cover correlation, a lost rejection response,
source loss, unknown results, reload and explicit identical-input resubmission.
The added Web case disables a synthetic recipient after confirmation, verifies
the rejection, reloads with the same private file, re-enables the recipient and
explicitly resubmits unchanged input. It then replays the old rejected key after
the successful new request consumed the draft and expects the old refusal.

### Reviewed without changes

- Resource and Switch UI uses the current organizer/actual-owner affordances,
  explicit resource space, original version observations and common pending
  operation recovery. Resource-only missing options are unavailable rather than
  issue denial; the independent backend reviewer verified those actual codes.
- Participant management uses its dedicated list, preserves disabled accounts
  for creator removal and confirms handler removal's return to the creator.
- Account entry and source selection stay independent of the workspace bootstrap
  result. Source scope is separate from an issue UUID.
- Private native URLs remain confined to the exact current API origin and
  issue/file/version, and never enter persisted forms or mutation results.
  Preview uses the existing restricted rich content/HTML sandbox and revokes
  object URLs when access loss unmounts the attachment list.
- Comment display references are filtered separately from the permanent issue
  attachment list. Removing a display reference does not hide issue history.
- New Web and native Electron cases use actual creation/delivery APIs. The
  native fixture extracts the existing production download IPC handler and
  imports its existing safe-download helper; it does not replace native download
  behavior with a renderer stub.

## Findings not fixed

- No unresolved frontend defect remains in the reviewed slice. The durable
  creation refusal gap was resolved with the parent-approved server/client
  contract above.
- Production Web/Electron execution and visual acceptance belong to the parent.
  This report does not count unit tests or Playwright discovery as those runs.
  Source-page creation/structural fields and the other continuation items remain
  separate parent-task work; this slice does not complete all 97 ACs.

## Changed files owned by this review

Product changes: `packages/core/api/client.ts` (private delivery/recovery blocks
only), `packages/core/collaboration/{creation-schema,creation,attachment,comment,delivery-access,delivery-form,index}.ts`,
`packages/views/collaboration/{creation-form,attachment-picker}.tsx`, and the two
collaboration locale files. Existing receipt-matcher and other lanes' edits were
retained. No additional delivery-form snapshot fields remain.

Tests: `packages/core/collaboration/{delivery,delivery-api,comment}.test.ts`,
new `packages/core/collaboration/delivery-form.test.tsx`, and new
`packages/views/collaboration/attachment-picker.test.tsx`, the updated creation
component suite, and the added refusal/resubmission journey in
`e2e/collaboration-delivery-web.spec.ts`.

The fixes reuse the existing Query/access guards, StorageAdapter records,
original operations and shared picker. No new store, dependency, public upload
fallback, service process or commit was introduced.

## Spec synchronization

The core workbench spec still describes creation, attachments and resource forms
as absent. The parent was notified to update it for this slice and preserve the
new cancellation, unambiguous-authority, durable-rejection and bundle-retirement contracts. Server
spec changes are owned by the backend reviewer/parent.

## Verification

Evidence root:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl`.

| Check performed by this reviewer | Result | Evidence |
| --- | --- | --- |
| Recipient/source denial regression | RED (2 failures), then PASS | `client-check-source-denial-{red,green}.log` |
| Upload cancellation/hash fence/bundle retirement | RED (6 failures), then 21 tests PASS | `client-check-upload-retirement-{red,green}.log` |
| Ambiguous file denial | RED (2 failures), then 14 tests PASS | `client-check-file-denial-{red,green}.log` |
| Full core collaboration suite | 42 files / 612 tests PASS | `client-check-core-final.log` |
| Additional required upload-intention schema case | 5 tests PASS | `client-check-metadata-contract.log` |
| Full final views collaboration + locale parity | 9 files / 87 tests PASS | `client-check-views-final.log` |
| Changed product/test ESLint | PASS | `client-check-core-lint.log`, `client-check-views-lint.log` |
| Core + views typecheck | PASS | `client-check-typecheck.log` |
| Final metadata-test lint + core typecheck | PASS | `client-check-metadata-lint.log`, `client-check-core-final-typecheck.log` |
| Playwright discovery of the three integration specs | PASS: 9 tests discovered | `client-check-e2e-discovery.log` |
| Durable rejection boundary + source/form recovery | RED, then PASS in final core suite | `client-check-creation-rejection-red.log`, `client-check-core-final.log` |
| Rejection form recovery | RED, then PASS; included in final 87 tests | `client-check-rejection-form-{red,green}.log` |
| Explicit identical-input resubmission | RED, then PASS in final core/views suites | `client-check-explicit-resubmit-red.log`, final suite logs |
| Final durable-rejection core/views lint and typecheck | PASS | `client-check-creation-rejection-{core-lint,views-lint,typecheck}.log` |
| Final Web E2E strict TypeScript + discovery | PASS: 10 tests discovered across 3 integration specs | `client-check-rejection-e2e-types.log`, `client-check-rejection-e2e-discovery.log` |
| Last completed-result HTTP status fix | RED, then 29 targeted tests + API lint + core typecheck PASS | `client-check-result-status-{red,green,lint,typecheck}.log` |
| `git diff --check` | PASS | Tool output (empty, exit 0) |

Earlier focused logs capture the corresponding intermediate fix. The full
suites cover the final client behavior; the last change was confined to the
new result reader and was verified by the 29 creation/delivery tests and fresh
API lint/core typecheck.

No browser/native success is claimed by this reviewer. The parent owns the
matching final production build, service provenance and actual E2E execution.
