# Collaboration workflow client verification

Date: 2026-10-13. Scope: the next P2 human workflow API contract in `packages/core`.

## Implemented

- `packages/core/collaboration/schema.ts`: optional nullable `write_context` with strict positive signed-64-bit decimal `participant_epoch` and `account_epoch`; malformed present context makes the detail unavailable. Existing read payloads without context remain valid and receive no synthesized epochs.
- The same schema module defines `CollaborationOperation` (`code`, `collaboration_revision`), its schema and redacted parser, plus camelCase write/participant/handoff input interfaces. Codes must contain a non-whitespace character but are not restricted to an enum. Unknown fields are stripped. A malformed receipt returns `null`, leaving the original operation unconfirmed.
- `packages/core/api/client.ts`: `addCollaborationParticipant`, `createCollaborationHandoff`, `removeCollaborationParticipant`, `leaveCollaborationIssue`, and `getCollaborationOperation`. The four POST methods map the original versions to snake_case and use the caller's `requestId` as `Idempotency-Key`. Handoff sends target and note, without a client-selected purpose. There is no automatic retry, new operation-key generation, or version rebasing.
- The shared collaboration fetch helper now accepts `RequestInit`, preserving existing credential/endpoint/abort fencing and private-log behavior. All requests remain account-scoped and strip ambient workspace headers. Historical operation GET needs only issue/request identity and current authentication; stored rejection codes in HTTP 200 responses are returned as receipt data. HTTP rejections keep the normal `ApiError` and original body.

## Evidence

Commands run in this checkout:

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/core test collaboration/schema.test.ts api/collaboration.test.ts` before changes | 2 files, 44 tests passed |
| `pnpm --filter @multica/core test collaboration/schema.test.ts api/collaboration-workflow.test.ts` after adding tests, before implementation | 66 failed / 6 passed, confirming missing methods/parser/context support |
| `pnpm --filter @multica/core test collaboration api/collaboration-workflow.test.ts` after implementation | 3 files, 93 tests passed; exit 0 |
| `pnpm --filter @multica/core typecheck` | Exit 0 |
| `pnpm --filter @multica/core lint` | Exit 0 |
| `git diff --check --` assigned source/test/evidence paths | Exit 0 |

The CLI printed the existing pnpm configuration-placement warning for root `pnpm.onlyBuiltDependencies` / `pnpm.overrides`; no dependency or configuration changes were made.

`collaboration/schema.test.ts` owns receipt/write-context parsing: absent/null compatibility, exact values beyond JavaScript's safe integer range, maximum signed int64, zero/negative/padded/decimal/scientific/whitespace/oversized/non-string values, unknown nonempty result codes, and private schema log sentinels. Existing ordinary issue/comment numeric revision tests remain passing.

`api/collaboration-workflow.test.ts` owns the five endpoints' client wiring: exact routes and POST bodies, authentication/content type, ambient header removal, original keys and versions through network failure/rejection/retry, actor receipt lookup without current write context, malformed JSON/receipt null results, private logging, credential changes during parsing, stale 401/403 responses, caller abort and endpoint replacement. No actual private user data or AI provider was used.

## Remaining integration work

This is API-client verification, not end-to-end P2 acceptance. Root owns server route/ACL/transaction tests and public barrel exports. Query/mutation hooks, cache invalidation, persisted recovery, and Web/Desktop UI are intentionally outside this assigned slice. Consumers must keep an unavailable write context unavailable and retain the original intent/key after an unknown receipt. No commits were created.
