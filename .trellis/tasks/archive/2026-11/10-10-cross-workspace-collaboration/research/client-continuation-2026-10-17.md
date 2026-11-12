# Client continuation — 2026-10-17

## Boundary

The foundation has account-scoped API methods and validated human/detail/v2 execution DTOs, but no shared account workbench. Implement the shared read/action surface in `packages/core/collaboration` and `packages/views/collaboration`, then connect Web and Desktop. Server changes belong to the content and authority lanes.

Use a separate persistent Desktop account group with one tab; retain workspace groups, pins, MRU and virtual history. Account URLs deduplicate at `/account`, while navigation and mementos include the selected view/issue. Entering account scope clears the workspace mirror. Authentication remains mandatory; workspace-list availability does not gate account access.

Queries own server state. Keys include API instance/session, viewer and issue; access loss advances an epoch before cancellation with `revert:false`, clears private drafts and prevents late reads from returning. Durable action drafts contain the original payload and idempotency key, scoped by server/viewer/issue. Retries never refresh versions implicitly.

## Interface and verification plan

1. Pure account route parsing, privacy/access fences and draft recovery tests in core.
2. Shared list/detail, human actions, execution projection, comments/mention preview and notifications, using peer DTOs.
3. Web account authentication route and Desktop group/navigation/memento/boot wiring, with platform tests.
4. Targeted tests, type checks and lint; main session owns browser/native visual verification and integration evidence.

The surface extends Multica's existing operation UI: semantic tokens, concise header, current/participating list beside one detail, a shared comment stream and explicit action forms. The first viewport identifies the current handler and source without loading either workspace's directory. Runtime support is a separate fact from human permission; the current v2 `unavailable` projection must never display a successful start/resume.

## Integration contracts received

- Content uses Markdown mention links `mention://member/<user UUID>`, `agent/<resource UUID>`, `squad/<resource UUID>`, `all/all`; plain names stay inert.
- Authority exposes only fully consented issue targets and keeps paused/offline/busy targets eligible with a waiting reason. `target_id` and typed `resource_id` are distinct.
- Main session supplies isolated production Web/native environments; no application database is used by this lane.

## Handoff after the confirmed single-agent priority

The main session confirmed the latest §64 decisions and redirected available capacity to the real single-agent runtime loop. This lane stopped after the bounded headless slice. The original broader interface/tab plan above remains future work; it does not override `continuation-single-agent-2026-10-17.md` or define target/batch/control semantics. No Web, Desktop, views, locale or tab-store files were edited.

Implemented files:

- `packages/core/paths/account.ts`, its test, and `paths.ts`/`index.ts`: canonical account URLs, exact UUID selection, comment/body anchors, separate view/issue memento keys.
- `packages/core/collaboration/access.ts`, its test, and `queries.ts`: API instance/session + viewer keys, per-object access epochs, late-response rejection, authoritative denial cleanup with `revert:false`, current/participating pagination and fresh detail/status reads. The scope hook retains only the current account/server's drafts and cache after authenticated identity is known.
- `packages/core/collaboration/draft.ts` and its test: independent per-request durable keys, original payload retention, corrupt/non-persistent storage rejection, object/session/scope cleanup.
- `packages/core/collaboration/action.ts` and its test: the existing human workflow API, original version/key capture and receipt-first recovery. It does not implement execution controls.
- `packages/core/collaboration/content-schema.ts`, its test, and `content-queries.ts`: private comments, candidates, preview/receipt and notification envelopes. Comment inputs retain nullable `channel_id`/`batch_id`; an individual input is not described as a run/group. Source-only comment epochs remain nullable. Unknown/malformed protocols return unavailable and logs are redacted.
- `packages/core/api/client.ts` and `api/collaboration-content.test.ts`: peer-agreed content/notification endpoints through the existing collaboration transport, exact response identity checks, no ambient workspace headers.
- `packages/core/collaboration/schema.ts`/`index.ts`: reuse/export the existing account summary and new headless surface.
- `packages/core/platform/session-cleanup.ts` and its test: erase account drafts on logout/expiry without registering them for workspace cleanup.

Verification completed on 2026-10-17 at approximately 23:01 Asia/Shanghai:

```text
pnpm --filter @multica/core test collaboration paths/account.test.ts paths/paths.test.ts api/collaboration-content.test.ts platform/session-cleanup.test.ts
20 files, 436 tests passed (2.43 seconds).

pnpm --filter @multica/core typecheck
Passed.

pnpm --filter @multica/core exec eslint collaboration api/collaboration-content.test.ts api/client.ts paths/account.ts paths/account.test.ts paths/paths.ts paths/index.ts platform/session-cleanup.ts platform/session-cleanup.test.ts
Passed, no lint findings.

git diff --check -- packages/core .trellis/tasks/10-10-cross-workspace-collaboration/research/client-continuation-2026-10-17.md
Passed.
```

No server integration, browser, native Electron or runtime verification was performed by this lane. Main owns those environments and remaining integration. No commit was created.

Next client work should mount the account auth guard, key the private subtree by API/viewer/issue identity and withhold warm content until the mount-time access probe completes. API/query/draft foundations are ready to reuse, but no product screen yet calls them. Optional endpoint 404 is treated conservatively as access loss; revalidate the detail before retaining a lawful source-member projection. Add source-membership/realtime invalidation, scoped editor/upload/transcript cleanup and native account-tab wiring when client implementation resumes. Confirm the final backend envelopes in integration; the schemas were built from the content/authority lanes' agreed DTOs, not a running server.
