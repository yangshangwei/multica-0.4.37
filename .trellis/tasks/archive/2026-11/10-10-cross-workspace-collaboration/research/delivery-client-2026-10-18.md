# Shared creation and controlled attachment client

## Boundary before implementation

The server now exposes atomic collaboration creation and private attachment
delivery; the account workbench has neither a creation entry nor an attachment
reader/uploader. This lane adds their shared core/view clients and Web/native
acceptance cases, preserving the existing account routing and realtime owner.

Pure creation/attachment schemas own response validation. Request helpers own
same-key recovery and authority fences. Separate StorageAdapter records bind
pre-issue forms/uploads to Server, viewer, source workspace and draft/request
identity; an issue UUID is never fabricated from a source workspace. Uploaded
files stay private on the server. An upload whose response is lost retains its
original key and file fingerprint; recovery asks for that key, and a resend
requires the identical file. Only local form intent and selected file IDs are
persisted, never native capabilities or a parallel server-data cache.

Expected edits: new core creation/attachment schemas, scoped delivery drafts,
query/mutation helpers and regressions; narrow ApiClient/barrel/cleanup/comment
schema changes; shared creation dialog, uploader and attachment panel; account
workbench/detail/composer mounting; creation/attachment locale keys; focused
component and Web/Electron E2E cases. Native file previews reuse the existing
HTML sandbox, with authenticated reads and object-URL cleanup on access loss.

This lane does not change server authorization, public rollout gates, resource
forms, provider execution, ordinary uploads, ordinary-issue conversion or source
list/board semantics. Full source structural fields remain separate work.

## Validation

The four new API regressions first failed against the absent methods, then
passed after the API/schema layer landed. A later comment regression reproduced
false confirmation when a success receipt omitted selected attachment IDs; the
input-aware receipt matcher fixed that case before handoff to independent review.

Checks performed by this lane:

- Core API/draft/access/storage: 20 tests passed across focused runs before later additions.
- Core content/comment/delivery suite: 18 tests passed; attachment schema and
  comment receipt suite subsequently passed 13 tests, including private logging,
  intended-issue mismatch and credential-change response fencing.
- Shared creation and attachment components: 3 tests passed, exercising recipient
  confirmation, lost-upload/lost-create reload recovery, keyboard focus and object
  URL cleanup after access loss. Existing comment composer (2) and account page
  (4) tests also passed.
- Core and views `tsc --noEmit` passed. Changed source/test ESLint passed.
- Strict E2E TypeScript compilation passed using ES2022, DOM and DOM.Iterable.
  `playwright --list` found all 8 intended Web/native cases; the native fixture
  passed `node --check`. `git diff --check` passed.

The main session owns production builds and execution of the real browser/native
cases. This lane has not claimed those cases as executed. The initial build-ready
checkpoint was followed by the small receipt-correlation repair; the main session
was notified so final acceptance uses a rebuilt artifact.

## Delivered source

Core: pure `creation-schema.ts` / `attachment-schema.ts`, `creation.ts` /
`attachment.ts`, `delivery-draft.ts` / `delivery-access.ts` / `delivery-form.ts`;
narrow API methods, barrel, session/object draft cleanup and comment attachment
schema/input/receipt changes. Schemas remain independent of request modules.

Views: `creation-form.tsx`, `attachment-picker.tsx`, `attachment-panel.tsx` and
account-workbench/detail/comment-composer mounting. Shared English and Simplified
Chinese additions use only creation/attachments product labels. The existing
HTML sandbox renders authorized HTML; other preview content is fetched with
current account/issue authority, and object URLs are revoked on unmount/access
loss. Native download capabilities are never stored in form data or Query cache.

Tests: `delivery-api.test.ts`, `delivery.test.ts`, `attachment-schema.test.ts`,
comment receipt regression, `creation-form.test.tsx`, `attachment-panel.test.tsx`,
`e2e/collaboration-delivery-web.spec.ts`, and a new named creation/download/restart
case in `e2e/collaboration-account-desktop.spec.ts`. Its existing Electron fixture
now extracts the production native download IPC registration and guarded URL
helper; there is no second native harness.

## Remaining boundaries / independent review

- The backend now echoes mandatory `intended_issue_id`; client checks null for
  pre-issue uploads and the exact issue UUID for issue-intended drafts. A whole
  draft is consumed once. Every successful form starts a fresh draft identity;
  already delivered same-issue references do not require the old draft.
- An upload whose bytes have not been confirmed survives as its original key and
  file fingerprint. After a reload the original local file may need reselection;
  the client verifies the same name/type/size/SHA-256 before resending. It does
  not serialize arbitrary binary files into StorageAdapter.
- Independent `client_check` owns follow-up fixes for non-authoritative recipient
  rejection, upload cancellation and successful comment-bundle retirement. The reviewer owns final
  targeted core/views collaboration verification and will send the final source
  checkpoint to the main session; this lane does not duplicate those active edits.
- Source-page creation entry, project/parent/iteration/other source field controls,
  ordinary source list/board integration, complete comment editing controls,
  provider file preparation and full product ACs remain outside this lane.
- Public creation/execution defaults were not changed. No servers, real AI tools,
  dependencies, commits or releases were started by this lane.


## Evidence files and exact edit manifest

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl`.
This lane saved `delivery-comment-receipt-red.log`,
`delivery-client-core-typecheck.log`, and `delivery-client-views-typecheck.log`.
Other focused test results are in the Codex tool trace; final integrated test and
production/native evidence belongs to the independent checker/main session.

New files:

- `packages/core/collaboration/creation-schema.ts`
- `packages/core/collaboration/creation.ts`
- `packages/core/collaboration/attachment-schema.ts`
- `packages/core/collaboration/attachment.ts`
- `packages/core/collaboration/delivery-access.ts`
- `packages/core/collaboration/delivery-draft.ts`
- `packages/core/collaboration/delivery-form.ts`
- `packages/core/collaboration/delivery-api.test.ts`
- `packages/core/collaboration/delivery.test.ts`
- `packages/core/collaboration/attachment-schema.test.ts`
- `packages/views/collaboration/creation-form.tsx`
- `packages/views/collaboration/attachment-picker.tsx`
- `packages/views/collaboration/attachment-panel.tsx`
- `packages/views/collaboration/creation-form.test.tsx`
- `packages/views/collaboration/attachment-panel.test.tsx`
- `e2e/collaboration-delivery-web.spec.ts`
- `.trellis/tasks/10-10-cross-workspace-collaboration/research/delivery-client-2026-10-18.md`

Existing files with focused client-lane edits (shared-owner edits preserved):

- `packages/core/api/client.ts`
- `packages/core/collaboration/comment.ts`
- `packages/core/collaboration/comment.test.ts`
- `packages/core/collaboration/content-schema.ts`
- `packages/core/collaboration/draft.ts`
- `packages/core/collaboration/index.ts`
- `packages/views/collaboration/account-workbench-page.tsx`
- `packages/views/collaboration/collaboration-detail.tsx`
- `packages/views/collaboration/comment-composer.tsx`
- `packages/views/locales/en/collaboration.json`
- `packages/views/locales/zh-Hans/collaboration.json`
- `e2e/collaboration-account-desktop.spec.ts`
- `e2e/fixtures/collaboration-account-electron.mjs`
