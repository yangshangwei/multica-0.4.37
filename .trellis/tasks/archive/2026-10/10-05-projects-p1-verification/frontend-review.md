# P1 frontend and mobile compatibility review

Verdict: **REQUEST CHANGES**. Reviewed baseline `0af59c5d5` through `bb1c20dcb`, in `/Volumes/artisan/code/2026/multica-projects-p1`, on 2026-10-05. No product or test files edited; no commit created. This report owns frontend/API integration findings, not the parallel backend concurrency, migration, or browser verdict.

Reviewed against the parent design, API contract, test specification and original `projects-prd.md` P1. Stage 1 requirement compliance fails on the following concrete cases. Styling preferences and future I1/P2 features are excluded.

## Must fix

### FR-01 — HIGH: project update notification empties the mobile inbox

- Location: `server/internal/handler/project_update_notifications.go:148`, consumed by `apps/mobile/data/schemas.ts:549` and `apps/mobile/data/api.ts:458`.
- Trigger: deliver one member mention from a project update. Its inbox `details.revision` is JSON number `1` because `row.SourceRevision` is marshaled directly.
- Impact: mobile expects `details: Record<string, string>`; `InboxListSchema` rejects the complete array and `listInbox` falls back to `[]`. One new notification therefore hides all existing inbox items, rather than only losing a new feature label. This violates PRJ-014 / P1-FR-13 mixed-client compatibility.
- Evidence: executed the actual mobile Inbox schema segment in memory. Numeric revision produced `invalid_type` at `[0, "details", "revision"]`, expected string, received number. The same fixture with revision `"1"` passed. Core's `InboxItem.details` also declares string values, though its loose boundary does not reject the number.
- Fix: keep the existing details wire contract (stringify revision) or explicitly coordinate compatible parsing across installed clients. Add a mobile mixed old/new inbox response regression, not only project endpoint tests.
- Reverify: deliver a real notification, read recipient inbox on mobile API parser, confirm existing items plus the new item survive.

### FR-02 — MEDIUM: first risk drilldown does not transmit the snapshot version

- Location: `packages/core/api/client.ts:4514`; server reads `snapshot_version` at `server/internal/handler/project_health.go:173` and computes refreshed at `:213`.
- Trigger: open overview at version A; a risk task changes before clicking its card; client sends `?signal=...&version=A`.
- Impact: backend receives no snapshot version and sets `refreshed=false`, so the risk page silently shows a different set/count from the clicked card. Cursor requests do carry a version internally, so this specifically breaks the initial card-to-list transition. Violates PRJ-007 / AC-16 / API contract §2.
- Fix: serialize `params.version` as `snapshot_version`, while preserving the UI URL's separate `version` parameter if desired.
- Reverify: assert exact outgoing API query parameters; real API overview A → task update → first drilldown must return refreshed=true and show the refresh message.

### FR-03 — HIGH: failed overview refresh continues to assert old healthy results

- Location: `packages/views/projects/components/project-overview.tsx:52` through `:76`.
- Trigger: a successful overview is cached with health `clear`; manual/WS/day-boundary refresh subsequently fails with 503, network error or malformed response.
- Impact: React Query retains data together with error. The component only handles missing data, then still shows “No current risk signals”, old counts and active risk buttons. There is no failure or stale label. A calculation timestamp alone does not disclose the failed refresh. Violates design §7 and P1-FR-09 (“old values only when explicitly marked stale”, no latest healthy claim).
- Fix: render an explicit refresh failure/stale state when `query.error` coexists with data and suppress the current-health assertion. Preserve the last calculation time and offer retry.
- Reverify: seed a clear response, reject refetch, assert visible stale/error status and no current clear-health claim; recover with a subsequent success. Existing management tests do not mount this error state.

### FR-04 — HIGH: revocation cleanup can be undone by mutation completion, and description 403 does not enter cleanup

- Location: `packages/core/projects/access.ts:12`, `packages/core/projects/mutations.ts:106` / `:110`, `packages/views/projects/components/project-description.tsx:29` / `:33`.
- Trigger A: project update is already in flight; a membership event clears protected queries; the successful response arrives afterward. Trigger B: socket is unavailable and description autosave itself returns 403.
- Impact A: `cancelQueries` does not cancel mutations, and unguarded `onSuccess` restores the full Project/description cache. An optimistic failure can similarly restore previous list/detail state. Protected mutation results also remain in mutation cache. Impact B: the description controller merely displays its error; unlike progress, it has no access guard, so protected editor/candidate content remains until some unrelated read learns of revocation. Violates AC-26 / P1-FR-08 / FR-15.
- Evidence: executed the actual access and mutation callback source against a real QueryClient in memory. Immediately after clear, detail was undefined; invoking the late successful callback restored `{ description: "protected response received after revocation", ... }`.
- Fix: centralize project request/mutation access handling, capture an access epoch and reject/ignore late completions and rollbacks after revocation; clear protected mutation data and candidates as well as queries. Treat a workspace 401/403 consistently regardless of which write/read discovers it. Mobile's epoch approach already provides a local reference.
- Reverify: barrier tests for clear → late mutation success/failure; description autosave403 with no WS event and failed navigation; assert no protected query, mutation, editor or candidate content survives or refills.

### FR-05 — MEDIUM: “Use server version” preserves and later resurrects the rejected local description

- Location: `packages/views/projects/components/project-description.tsx:54` / `:56`, `packages/core/projects/description-draft-store.ts:15` / `:16`.
- Trigger: local draft differs from current server text, CAS conflicts, user explicitly chooses the server version, then closes/reopens the project/editor.
- Impact: `acknowledgeProjectDescriptionDraft` deliberately preserves differing body text while rebasing its revision. It is suitable for acknowledgement of an older in-flight save, not explicit local discard. The UI temporarily adopts server text but the durable draft still contains the rejected local text with the newest server revision. Reopening restores it and permits an unintended overwrite on retry. Violates AC-25 conflict recovery.
- Evidence: actual helper invocation with local `discarded local text`, then server adoption `chosen server version` revision 2 retained `{body:"discarded local text",baseBody:"chosen server version",revision:2}`.
- Fix: explicit adoption must remove/replace that editor's durable draft, rather than call the acknowledgement-preserving branch.
- Reverify: conflict → use server → unmount/remount → only server text, no unsaved retry; retain the distinct test where an older successful save must preserve newer typing.

### FR-06 — MEDIUM: cancel/navigation can drop the latest progress text during its debounce

- Location: `packages/views/projects/components/project-progress.tsx:116` / `:153`; `packages/views/editor/content-editor.tsx:739` / `:744`.
- Trigger: type, then cancel or leave the overview within 300 ms of the last change.
- Impact: composer persists only debounced onUpdate and does not opt into `flushPendingOnUnmount`. ContentEditor explicitly discards a pending update on unmount by default. Reopening restores older text or nothing. The existing cancellation test uses an immediate textarea mock, so it cannot expose this loss. Violates PRJ-008 draft/input recovery and the implementation's intended cancel-preserves-draft behavior.
- Fix: preserve the final local body on cancellation/unmount (using the existing flush contract with a revocation/deletion guard so cleanup does not re-persist protected content).
- Reverify: real ContentEditor or behavior-faithful fake timers: edit → cancel before 300 ms → reopen equals all typed text; navigate-away variant; revocation unmount must not repopulate draft.

### FR-07 — MEDIUM: internal evidence links do not resolve through real Web/Desktop routes

- Location: `packages/views/projects/components/project-progress.tsx:37`; producer `server/internal/handler/project_update.go:351` / `:402`.
- Trigger: publish issue or execution evidence, then click it in the update/history.
- Impact: UI emits a raw new-window anchor for `/issues/{id}` or `/tasks/{id}`. `/tasks/{id}` is not a normal Web or Desktop route (only admin task routes exist). Issue routes require `/{workspaceSlug}/issues/{id}`. Web's legacy `/issues` redirect guesses via last_workspace_slug, so a second workspace tab can send the click to the wrong workspace; Desktop lacks that Web proxy. Available evidence can therefore be impossible to review. Violates PRJ-008 and shared navigation boundaries.
- Fix: resolve internal evidence via workspace identity and the established NavigationAdapter/actual execution detail destination; reserve external anchors for URL evidence. Do not invent a task page that does not exist.
- Reverify: real Web and Electron clicks for issue and execution evidence; two workspace tabs with last workspace changed; confirm source opens in its own authorized context.

### FR-08 — MEDIUM: project deletion is treated as permission loss and destroys the promised copyable local text

- Location: `packages/core/projects/access.ts:9` / `:19`, `packages/views/projects/components/use-project-access-guard.ts:7`, `packages/views/projects/components/project-detail.tsx:264`.
- Trigger: another member deletes a project while this user has unsaved description or progress text; subsequent project query returns project_not_found404.
- Impact: 404 is classified identically to 401/403, clears both draft stores and unmounts into only “permission lost”. There is no project-deleted state or way to copy the user's own pending body. The contract explicitly distinguishes deletion from revocation. Violates original PRD §16 and P1-FR-08.
- Fix: keep 401/403 full erasure; handle an authorized project-not-found response as deletion, remove protected fetched data/evidence, stop all submits and retain only the user's own unsaved text in a copy-only recovery surface. Do not recreate the project.
- Reverify: two clients, delete during editing; verify copy works and saves are impossible; repeat with true revocation and verify text is erased.

### FR-09 — MEDIUM: non-completion property CAS failures have no visible recovery

- Location: `packages/views/projects/components/project-detail.tsx:249`, `packages/views/projects/components/projects-page.tsx:388` / `:594`; only detail error rendering is inside the completion dialog at `project-detail.tsx:630`.
- Trigger: two clients share project revision A; one changes an attribute, the other changes status (other than completed), start date or due date before it receives a refresh. Its new expected_revision causes 409; invalid final dates can similarly cause422.
- Impact: shared mutation only invalidates caches; neither normal detail property UI nor list/card controls render the error or preserve a retryable pending property change. The user sees the picker close and their edit disappear without a conflict or validation message. New CAS guards therefore introduce a silent failure path despite the contract requiring explicit latest-state retry. Violates PRD §16 / P1-FR-02 / FR-04.
- Fix: give these writes visible errors and retain the attempted fields for explicit retry against the returned/latest revision; surface date field validation. Keep completed confirmation's existing handling.
- Reverify: delayed WS/two-client status and date conflict from both detail and list/card; verify visible conflict, no overwrite, and explicit retry succeeds; invalid date order gets a clear actionable message.

## Coverage and checks

Read the changed core API/schema/project/query/draft/realtime/paths files, new shared project UI and existing page wiring, mobile API/query/mutation/realtime/schema/display compatibility, plus the relevant server response producers. Checked the original IssueSurface remains wired for the five standard task views and the project resource/default squad sections remain present. API absence-versus-zero and existing closed numerator are preserved; strict P1 parsing rejects malformed/incomplete healthy snapshots; risk UI has an independent scope and does not apply personal filters. Acceptance display separates current-description and latest acceptance, and correction drafts preserve their original description revision. These are code observations, not substitutes for the parent browser/DB evidence.

Executed at reviewed HEAD:

- `pnpm -C packages/core exec vitest run api/project-p1-client.test.ts api/project-p1-schema.test.ts projects/description-save.test.ts projects/progress-draft-store.test.ts projects/mutations.test.tsx projects/realtime.test.ts`: 6 files / 48 tests passed.
- `pnpm -C packages/views exec vitest run projects/components/project-management.test.tsx projects/components/project-detail.test.tsx projects/components/project-issue-metrics.test.ts`: 3 files / 24 tests passed.
- `pnpm -C apps/mobile exec vitest run data/project-p1-api.test.ts data/queries/projects.test.ts data/realtime/project-ws-updaters.test.ts data/realtime/project-access.test.ts data/realtime/use-projects-realtime.test.ts data/mutations/projects.test.ts`: 6 files / 37 tests passed.
- `pnpm -C packages/core typecheck`, then views typecheck, then mobile typecheck: all exit 0. No LSP diagnostics tool is exposed in this session; these package TypeScript checks provide the diagnostic evidence.
- Three independent read-only in-memory reproductions used the actual source: description adoption retains discarded draft; revoked cache is refilled by late mutation success; numeric notification revision fails mobile inbox schema. No test files or runtime state were written for these probes.

The separate mobile implementation report records the full 160-test suite and lint; this review independently reran the 37 P1 compatibility tests and mobile typecheck. It did not run iOS simulator/device, a new RN editor, production build, or browser flows; those remain separate verification lanes. Root check exclusion of mobile must continue to be explicit.

No implementation simplifications were made: this is a read-only review with one report artifact. Re-review the fixes and run the named behavioral regressions before changing this verdict to APPROVE. Future iteration/I1 cases stay explicitly pending per the parent test specification, not falsely counted as P1 proof.
