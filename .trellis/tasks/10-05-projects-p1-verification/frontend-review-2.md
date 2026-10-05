# P1 frontend re-review 2

Verdict: **REQUEST CHANGES** — three concrete remaining/new issues. Review source: HEAD `8fcea88de`, including current uncommitted execution-evidence route and operation-level403 code fixes in `server/cmd/server/router.go`, `project.go`, `project_timezone.go`, and `project_write_fence.go`. Reviewed on 2026-10-05 in `/Volumes/artisan/code/2026/multica-projects-p1`.

This is an independent source and targeted-behavior review, not a restatement of `10-05-projects-p1-ui/review-fixes.md`. No source/tests were edited or temporarily replaced, no commit was created. Only this report was written. Real browser, final build, DB concurrency and performance measurements remain separate parent-owned evidence.

## Must fix

### RR-01 — HIGH: access/recovery state crosses authentication sessions

- Locations: `packages/core/projects/access.ts:20`, `:75`, `:89`; missing integration in `packages/core/platform/session-cleanup.ts:29`; copy-only display in `packages/views/projects/components/project-detail.tsx:267`.
- Trigger A: a project read/write learns a true403, user logs out and signs in as an authorized account in the same Web tab or Desktop renderer, then opens the same workspace's projects.
- Actual result: the module-level `denied` state remains set and `protectProjectRequest` rejects before trying the new authenticated request. All project reads remain blocked for the renderer lifetime. The production code has no reset/reauthorization path; only tests clear this store.
- Trigger B: project deletion captures this user's unsent body in `deleted`; user logs out and another user enters the same workspace/project URL.
- Actual result: `clearClientSessionData` resets registered draft stores and Query cache but does not reset `useProjectAccessStore.deleted`. The previous user's copyable text survives logout and is still rendered by the deleted-project surface. The new recovery buffer is sensitive client state even though it is not persisted.
- Reproduction actually run: loaded the current access module and current `clearClientSessionData` function in memory with real Zustand and QueryClient, substituting unrelated cleanup dependencies. After session cleanup the exact state still contained `denied:{'["w","*"]':true}` and `deleted:{'["x","p"]':['previous account private draft']}`. A fresh protected request returned403 with `networkCalls:0`.
- Required fix: bind project denial/recovery to the authenticated connection/session and clear sensitive recovery text at teardown. Permit a newly authenticated/re-authorized session to perform a fresh read. Keep a monotonic session generation/access epoch so old in-flight requests cannot become valid again merely because a reset returns counters to zero.
- Reverify: delete/copy buffer → real logout cleanup → another login must leave no text;403 → logout/login or explicit authorized workspace re-entry must allow a fresh network check; old-session pending read/write completion after the new login must remain rejected.

### RR-02 — MEDIUM: flushing the editor on preview transition discards a successful preview

- Locations: `packages/views/projects/components/project-progress.tsx:105`, `:125`, `:145`; editor cleanup contract `packages/views/editor/content-editor.tsx:739`.
- Trigger: type and press Preview publication within the300ms body debounce; preview succeeds before that debounce fires.
- Actual result: success sets reviewing=true, unmounting ContentEditor. The newly enabled flushPendingOnUnmount emits the pending body. `patch` unconditionally resets the preview mutation and sets reviewing=false, even when the emitted body is exactly the one already previewed. The successful preview disappears and the composer reopens without Publish. A second attempt after the quiet period can work, but the first normal fast interaction does not.
- Reproduction actually run: the current ProjectProgress, p1-mutations and progress-draft-store source were executed in an in-memory DOM with real React/Query/Zustand and an editor implementing the documented debounce/unmount flush. After one fast preview: `previews=1`, `publishButton=false`, `editorStillPresent=true`. No repository test file was written. The existing success test's immediate editor mock cannot exercise this pending flush.
- Required fix: distinguish genuine edited content from acknowledgement/flush of already previewed content, or flush/acknowledge pending editor input before requesting the preview. Preserve cancel/navigation recovery without letting a programmatic transition to review reset it.
- Reverify: edit → immediately preview → fast server success (<300ms) must stay on review with matching body and enabled Publish. Also test slow response, cancel/navigation within debounce and revocation unmount; do not trade away the FR06 fixes.

### RR-03 — MEDIUM: choosing the conflict's server version can be overwritten by stale controlled props

- Locations: `packages/views/projects/components/project-description.tsx:58`–`:64`; controlled editor synchronization in `packages/views/editor/content-editor.tsx:755` and `:825`.
- Trigger: mounted project prop is descriptionv1. A save returns409 with authorized current descriptionv2, while the separate invalidation GET is still pending or fails. User chooses Use server version before project props have caught up.
- Actual result: the handler adoptsv2 imperatively and clears the draft. That makes the controlled `value` immediately fall back to `project.description`v1. The real editor's adoptContent updates lastEmitted to its new text; therefore its external-value effect sees a clean editor and applies stalev1. The chosen version is not what remains displayed. Subsequent editing can also re-adopt the old revision through the baseline check.
- Reproduction actually run: current ProjectDescription, description draft store and save controller in an in-memory DOM, with a controlled editor applying the existing clean-value synchronization behavior and a409 currentv2 response. No latest project prop was supplied. After Use server version: expected `server v2 chosen`, actual `server v1`, durable draft `{}`. Inspected the real editor to confirm adoptContent updates lastEmitted before controlled-value synchronization (`content-editor.tsx:816`).
- Why the new regression misses it: the FR05 mock does not follow controlled value changes; its remount explicitly supplies the latest server project. That proves discarded drafts are removed but not that the still-mounted editor preserves the selected server text when the refetch lags.
- Required fix: atomically adopt the authorized conflict response into the controlled project/baseline state, or otherwise retain the adopted server value until canonical props catch up. Do not revive the old local draft to solve this.
- Reverify:409 currentv2 with delayed/failed latest GET → Use server version → immediate editor textv2; next edit uses description_revisionv2; remount remainsv2 after a successful canonical refresh. Keep the original discarded-draft regression.

## Original FR01–FR09 disposition

| Original | Independent re-review result |
| --- | --- |
| FR01 mobile inbox numeric revision | Closed for current source. Notification producer uses string revision; the real mixed project/old notification fixture is string-valued and mobile parser/navigation tests pass. |
| FR02 snapshot query parameter | Closed. Client now sends `snapshot_version`; exact outgoing URL test passes. ADR-05 changes continuation semantics, not the initial card version wire contract. |
| FR03 stale overview health | Closed. Retained-data errors render refresh-failed disclosure, health unavailable and disabled exact-risk buttons; failure/recovery component test passes. |
| FR04 same-session revoke/late completion | Original race fixed: protected request epochs, mutation success/rollback guards, candidate cache cleanup and description403 erasure are in the actual code and targeted tests pass. RR01 remains for session lifecycle. |
| FR05 rejected draft resurrection | Original durable rebase misuse removed by clearProjectDescriptionDraft. RR03 remains for the controlled-value adoption branch. |
| FR06 cancel loses pending characters | Original cancel/navigation loss fixed, with access-gated unmount flush and deletion-specific capture; targeted regressions pass. RR02 is a new interaction introduced by that flush. |
| FR07 invalid internal evidence URL | Closed at code/integration-wiring level. Issue AppLink uses workspace paths; execution opens the transcript dialog through the revision-specific authorized reader. Response identities are checked. Current source includes the required router registration, which remains uncommitted. Live Web/Electron reader verification belongs to browser lane. |
| FR08 deleted project no copy surface | Original deletion/revocation distinction implemented: no submits, copy-only local texts, later true403 clears them and late deleted reads remain404. RR01 remains because these texts survive session cleanup. |
| FR09 property failures invisible | Closed. Shared recovery is mounted for detail/list/card property writes, shows attempted fields/errors, and fetches latest revision only on explicit Retry.409,422 and operation-only403 regressions pass. |

Source/evidence403, operation privilege403 and feature-flag403 are explicitly excluded from workspace revocation in both core and mobile. Reviewed the parent-owned current server code: delete-administrator denial, timezone-administrator denial and machine actor denial emit project_permission_denied; evidence authorization emits project_evidence_forbidden; disabled updates emit project_updates_disabled. Generic scope403/401 still erase protected state. The execution reader rechecks project/update/revision membership and current source authorization; it does not use the old unscoped task-message endpoint.

The new project_update inbox label and target helper point Web/Desktop to the project plus update query parameter. ProjectProgress loads that target's revisions independently of the current list page, allowing an older notification target to open. Mobile routes to its existing project detail and keeps its explicit Web/Desktop capability limitation.

## ADR-05 C pagination review

Read pagination-adr.md plus the second architecture and Critic approvals. The reviewed implementation follows the changed contract:

- Server keeps the validated ID anchor when snapshot changes, recalculates the current formal set/count/overview under the same request transaction and emits a new version/cursor.
- UI sends the returned version on the next request and replaces the rendered page; it does not concatenate historical pages.
- Initial card refresh uses ordinary refreshed wording. A changed continuation sets sticky disclosure across subsequent refreshed=false pages.
- Explicit from-start action cancels a same-key pending query and performs fetchQuery with staleTime0 and no automatic retry; only success clears the cursor/disclosure. A failed restart retains the current page and warning.
- Empty suffix with positive total has distinct wording and restart; it does not claim there are no matching risks.

No further correctness finding was identified in the bounded C pagination source/tests. This is **not** performance acceptance and does not claim0/1/10-per-second measurements or final30-event convergence have passed.

## Independently executed checks

All commands ran in the worktree above at HEAD8fcea88de/current source:

- Core: `pnpm -C packages/core exec vitest run api/project-p1-client.test.ts projects/mutations.test.tsx projects/realtime.test.ts projects/progress-draft-store.test.ts` —4 files,36 tests passed.
- Views: `pnpm -C packages/views exec vitest run projects/components/project-review-regressions.test.tsx projects/components/project-risk-pagination.test.tsx projects/components/project-management.test.tsx inbox/components/inbox-display.test.ts` —4 files,39 tests passed.
- Mobile: `pnpm -C apps/mobile exec vitest run data/inbox-schema.test.ts lib/inbox-display.test.ts data/project-p1-api.test.ts data/queries/projects.test.ts data/realtime/project-access.test.ts data/mutations/projects.test.ts` —6 files,53 tests passed.
- Three additional read-only in-memory probes produced the RR01–RR03 results described above. They loaded current source with bounded dependency/editor substitutes; they are not labeled production-browser tests.

The parent was running full TypeScript checks; this review did not duplicate that workload or claim those results. No LSP diagnostics tool is exposed here. No performance/full build/device results are claimed. Remaining risks are the three required fixes and their named regression/real-browser confirmation; no unrelated refactor or new dependency is recommended.
