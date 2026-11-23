# Lane B — account and workspace settings

Status: implementation and focused verification complete. Native Electron acceptance and combined task review remain with the coordinator.

## Change boundary

The defects belonged to the existing settings components' query presentation, local draft validation, and write settlement. This lane retained local form drafts, existing layout/tokens, existing queries and API methods, and the current workspace-leave realtime ordering. It added no store, dependency, route, endpoint, or real-service mutation.

The coordinator extended ownership to only `memberListOptions` and `workspaceMcpServersOptions` in `packages/core/workspace/queries.ts`. Both now use lane A's `withQueryAccessBoundary`; all other query options remain untouched.

## Findings closed

| Finding | Implementation and regression evidence |
| --- | --- |
| F02 | Members, invitations, share links, workspace permissions, repository permissions, GitHub installations and repository picker reads use the shared settings query feedback. Pending/failed reads do not claim empty collections or confirmed non-admin membership. Cached transient reads retain draft input nodes. Definite membership denial hides management/invitation data; the core boundary removes denied member/MCP snapshots. Workspace Leave requires a recognized membership role and still protects a sole owner. |
| F05 | Invitation role, share-link role and share-link expiration expose distinct localized accessible names through the real Base UI Select controls. |
| F09 | Account/workspace avatar callbacks toast and rethrow persistence failures. Tests exercise the real upload control with only the canvas crop and upload transport stubbed: crop stays open, one failure toast appears, and no success is announced. Account retry then succeeds. |
| F10 | Required names expose associated errors and an explicit unsaved state. Tests edit additional fields while the name is invalid, correct it, and verify the complete retained draft is saved without replacing those input nodes. |
| F11 | Invitation Enter/button paths share a synchronous pending guard and preserve failed input. Composition Enter is ignored. Member removal, invitation revocation and issue-prefix confirmations remain open on failure, reject duplicate submission, and prevent dismissal while pending. Existing workspace deletion guards remain intact; share-link and seat-purchase entry points also reject duplicate pending operations. |
| F14 | Repository removal awaits `saveNow` before removing its row or clearing the confirmation. A failed final-row removal keeps both retry targets; a confirmed removal queues safely behind an existing edit. Candidate validation also permits discarding an unsaved blank row. |
| F15 | The timezone selector locks for the accepted write, displays saving status, and unlocks on success/failure. Tests hold the response, attempt a second interaction, then verify a later accepted choice wins and failure allows retry. |

## Changed paths

Implementation:

- `packages/views/settings/components/account-tab.tsx`
- `packages/views/settings/components/preferences-tab.tsx`
- `packages/views/settings/components/workspace-tab.tsx`
- `packages/views/settings/components/members-tab.tsx`
- `packages/views/settings/components/repositories-tab.tsx`
- `packages/views/settings/components/use-auto-save.ts`
- `packages/core/workspace/queries.ts` — only the import and two delegated query-function wrappers.

Tests:

- `packages/views/settings/components/account-tab.test.tsx` — new.
- `packages/views/settings/components/workspace-tab.recovery.test.tsx` — new, real QueryClient.
- `packages/views/settings/components/members-tab.test.tsx` — new, real QueryClient and existing billing mutation hooks.
- `packages/views/settings/components/repositories-tab.recovery.test.tsx` — new, real QueryClient.
- `packages/views/settings/components/preferences-tab.test.tsx`
- `packages/views/settings/components/use-auto-save.test.tsx`

Locale additions are in `research/locale-accounts.json`; the coordinator merged them into both settings namespaces. This lane did not edit locale JSON files directly.

## Interface changes and simplifications

`useAutoSave.saveNow(value)` now returns `Promise<boolean>`: `true` after the candidate is persisted or already current, `false` for invalid/disabled candidates, failed writes, or a queued candidate superseded by a newer save. The existing `flush(): void` interface remains unchanged. Queued callers resolve after their own serialized operation, including a requested return to the old persisted value while an earlier write is still running. Only the latest queued draft is written.

The optional `isValid(value)` predicate validates the candidate rather than an unrelated current draft. Repositories use it to remove a blank unsaved row without weakening workspace permission checks. Existing blur, debounce and GitHub import behavior remains covered by the original repository tests.

Shared query feedback replaces duplicated loading/error branches; all confirmation entry points use their owning handler's pending guard and settlement result. No new form framework or global draft mechanism was introduced.

## Verification

Before source edits:

- `pnpm -C packages/views test settings/components/repositories-tab.recovery.test.tsx settings/components/preferences-tab.test.tsx` — 7 new regressions failed, 15 existing preference tests passed. The final row disappeared while its request was pending, timezone controls remained enabled, and read failures lacked alerts/retry.
- Account/workspace/member regressions reproduced missing required-name feedback, false avatar success, duplicate invitation Enter, dismissible pending confirmations and missing recovery. One member-menu test needed to await Base UI opening; its final test uses the real menu.
- `pnpm -C packages/views test settings/components/use-auto-save.test.tsx` — 2 new await-settlement regressions failed, 3 existing tests passed.

Passing runs after implementation:

- `pnpm -C packages/views test settings/components/account-tab.test.tsx settings/components/preferences-tab.test.tsx settings/components/use-auto-save.test.tsx settings/components/workspace-tab.recovery.test.tsx settings/components/workspace-tab.test.tsx` — 38 passed.
- `pnpm -C packages/views test settings/components/members-tab.test.tsx` — 9 passed.
- `pnpm -C packages/views test settings/components/repositories-tab.test.tsx settings/components/repositories-tab.recovery.test.tsx` — 21 passed.
- After adding explicit unknown-role and unsaved-blank-row cases: `pnpm -C packages/views test settings/components/workspace-tab.recovery.test.tsx settings/components/repositories-tab.recovery.test.tsx settings/components/delete-workspace-dialog.test.tsx settings/components/seat-invite-purchase.test.ts` — 27 passed, including existing deletion/IME and seat-invite safeguards.
- `pnpm -C packages/core test workspace/queries.test.ts workspace/mutations.test.tsx workspace/mcp-template-queries.test.ts` — 22 passed.
- `pnpm -C packages/views typecheck` and `pnpm -C packages/core typecheck` — passed.
- ESLint on all lane-owned source/test files and `packages/core/workspace/queries.ts` — passed.
- `git diff --check` on lane-owned paths — passed.

The above view runs cover 85 distinct tests across 10 files; counts per command include overlapping files. Query access-denial/cancellation matrices live in lane A's canonical `packages/core/query-access.test.ts`; this lane verifies the real member-permission wiring.

## Limits and follow-ups

- The existing workspace-leave pre-navigation ordering remains unchanged, as explicitly agreed with the coordinator. Fixing its realtime race needs a separate core/lifecycle change; it is not a pattern for other destructive flows.
- All writes and collection responses in these tests are memory fixtures. No invitation, credential, purchase, repository mutation, or agent execution was performed against a live service.
- Native geometry, accessibility-tree and final English/Chinese runtime acceptance remain part of the coordinator's combined settings harness run.
- No known lane-specific test, lint or type-check failure remains.
