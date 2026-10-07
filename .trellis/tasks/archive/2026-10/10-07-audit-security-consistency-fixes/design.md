# Audit remediation design

## Goal and boundary

Close R1–R6 at their owning boundaries with targeted regressions. Keep the current public API, database schema and human workflows. Do not implement a general form framework, replace Electron's renderer protocol, change resource roles, or repair existing production records automatically.

## R1: native renderer file-request boundary

Install one session-level file-request policy from Desktop Main before loading either the main or issue renderer. Add `apps/desktop/src/main/renderer-file-access.ts` and a colocated unit suite; wire it in `apps/desktop/src/main/index.ts` using the same once-per-session ownership already used by download handlers.

For file requests, allow only a live, positively identified main-frame requester of the associated WebContents. Deny subframes and missing/destroyed/unverifiable frame contexts. Preserve the trusted renderer entry, scripts, dynamic imports, images and legitimate main-frame worker assets. The sandboxed HTML may continue to run normal JS and HTTP(S) requests, but cannot load file data/scripts/images/styles/documents through fetch, XHR, nested frames, navigation or cache reuse. The policy is native and remains in force when untrusted content navigates away from its initial srcdoc.

The existing webRequest `onBeforeSendHeaders` WebSocket handler is a different event and remains intact. Do not overwrite another onBeforeRequest subscriber; current production code has none, so this new module becomes its single explicit owner. Multiple windows sharing a session must not replace each other's listener or leave stale captured window state.

Alternatives considered: (a) a CSP prefix alone is rejected because HTTP/file self-navigation escapes it in real Electron probes; (b) fully restoring webSecurity with a custom renderer protocol/CORS migration has a much larger authentication/asset compatibility surface; (c) disabling HTML scripts removes working chart/interactive preview behavior. The native boundary is preferred because bounded experiments show it blocks the demonstrated file vector while retaining current previews. This is not a claim that disabled webSecurity provides full origin isolation: general network/CORS policy is unchanged.

Validation uses real Electron 39.8.7, fake temporary local files and local HTTP fixtures, with both attack-denial and legitimate-renderer/interactive-preview controls. Keep unit tests fast and maintain a separate explicit native regression script, tentatively `apps/desktop/scripts/verify-renderer-file-access.mjs`, using installed dependencies and temporary userData. No real user profile, credential or external provider is involved.

## R2: session bridge principal boundary

Reuse `RequireHumanActor` directly after Auth on `/api/plugin-bridge/v1`; add the same existing machine-source classification as a backstop at the start of `pluginSessionCaller`. Return before installation/member/resource reads and before hook dispatch. Preserve the dedicated `/v1` plugin bearer/callback path and the daemon plugin-hook path; the removed `/api/v1/plugin` alias stays removed.

Task and cloud-machine credentials cannot become member authority through owner membership in another workspace. Human JWT/cookie and supported human PAT callers retain existing behavior. Password mode still rejects unsupported cloud PAT authentication before this guard; callback source-version and revocation rules do not change. No success DTO/schema changes.

## R3: iteration draft and revision have one lifetime

Own editable baseline values/revision and local fields together in the form. Clean drafts may atomically adopt a newer server snapshot; dirty drafts never acquire its revision silently. Send changed mutable fields with the captured baseline revision. Existing `useIterationCommand` remains the single owner of durable request identity, pending recovery, session/permission fences and request submission.

On revision conflict, retain local input and present a clear server/local comparison using the existing conflict component/pattern. The user can adopt the server snapshot or explicitly rebase their changed fields onto the reviewed current snapshot; untouched remote fields survive. No automatic overwrite or automatic retry with a new revision.

On a known successful command, advance baseline only from a successfully refreshed, schema-validated iteration resource. Operation receipts contain no entity revision. A failed refresh must not cause an invented revision or duplicate committed write. Keep a refresh retry and preserve the local editor for transient background query errors while still immediately hiding protected content on access denial. This may require a small change to `IterationDetail`'s stale-data error branch in `iteration-page.tsx`; it is not permission to retain data after 401/403/workspace revocation or definitive resource deletion.

Local refactoring is limited to representing the edit snapshot coherently, guarded by behavior tests before edits. English and Simplified Chinese conflict/recovery copy follows the project glossary. Do not add persistent draft schema, endpoint fields or unrelated generic error handling.

## R4: project identity owns the overview lifetime

Key `ProjectOverviewPanel` at its existing call site in `project-detail.tsx` using stable workspace/project identity. Its nested composer, previews, correction target, cursors and timezone input then remount together, matching already-keyed adjacent project sections. Do not key by revision or change desktop tab-store semantics.

The existing server/workspace/project/update draft keys remain authoritative. Outgoing ContentEditor unmount flush executes with the outgoing project-bound callback; the newly mounted project editor reads only its own draft. Tests must seed cached A/B data and switch before the debounce expires, so loading fallback/remount timing cannot hide the regression.

## R5/R6: project-resource read/validate/write transaction

Reuse `runProjectTransactionAtIsolation(..., pgx.ReadCommitted, ...)`. Its workspace KEY SHARE, subscriber advisory and active-member SHARE fences come first. Acquire `LockProjectForExecutionSquad` (project FOR UPDATE) next. Only after obtaining that lock, issue separate fresh resource row/set reads; merge partial request fields and apply daemon uniqueness/capability checks through the transaction's queries, then write and commit. Publish exactly once after success.

READ COMMITTED is required here. The default repeatable-read snapshot can predate a waited-for resource insert, because resource writes do not update the parent project tuple. The exclusive parent lock alone would therefore not repair child-set phantom visibility. Do not use the shared `LockProjectForAssociation` or change default isolation for unrelated callers.

Decode/normalize request-owned input before the retry loop, then recompute stored-state-dependent fields inside every attempt. Preserve null/omitted label/position distinctions, old-client embedded-label synchronization, rename-only worktree exemptions and unknown stored JSON keys. Update the local conflict helper to take transaction-bound queries; use a callback-local Handler with Queries bound to that transaction only if necessary for existing response-writing validation helpers. Never mutate the shared Handler, recurse through HTTP handlers or begin a nested transaction.

No migration/sqlc change is expected: existing exclusive lock and child SQL fences suffice for current writers. Bundled new-project creation already validates its resource set; DeleteProjectResource's existing parent lock participates. Existing duplicate data remains explicitly rejected at execution and is not auto-deleted.

## Verification and rollout

Each lane records a regression's expected failure before product edits and its pass afterward. Real-router auth tests use genuine fixture credentials and valid requests, not forged trusted headers. Database concurrency tests use distinct active users, dedicated pgx connections and a fixture parent lock; release only after both request PIDs have a direct/transitive lock dependency on the holder. No sleeps-only race or production test hook.

Run focused checks per lane, then shared lint/typecheck/tests, relevant Go race packages, go vet and native Electron verification. Keep test databases/profiles uniquely isolated and clean only resources created for this task. Production deployment, installer packaging and version bumps are excluded. Any runtime regression rolls back the relevant small fix, not user changes or unrelated fixes.

## Evidence

- `research/report.zh-CN.md`: findings and audit limitations.
- `research/backend-fix-plan.md`: exact route matrix, helpers, lock ordering and deterministic test design.
- `research/client-fix-plan.md`: baseline/recovery and project-lifetime behavior.
- `research/desktop-html-isolation.md`: native file-boundary experiments and official API references.
