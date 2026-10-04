# T1 core client review

Reviewed 2026-10-05. Scope: triage types, schemas, API methods, queries, mutations, cache, realtime wiring, admission compatibility and their tests against the frozen API contract. Read-only review; this report is the only reviewer-owned file. Backend/UI completion is outside this verdict.

## Verdict

**APPROVE — core lane.** Independent re-review at 2026-10-05 00:57 local confirms C1–C4 are resolved in the current source. Independently reran the five focused suites: **133 tests passed**. No remaining blocking core finding was identified. This is not approval of the full backend/UI feature or proof of every T1 acceptance scenario.

## Re-review evidence

- **C1 closed:** every mutation now wraps the public input in an invocation-owned `{workspaceId,input}` command. Both transport and cache callbacks read the command scope, so replacing observer options cannot redirect a paused operation. The added real hook test pauses settings while offline, rerenders A to B, reconnects and verifies the request and cache remain A while B is untouched.
- **C2 closed:** `matchesTriageItem` and endpoint response predicates bind workspace/issue identity before resolution; item history resolves human identifiers only when needed, execution retry checks action identity, import get/commit checks batch identity, and action receipts match the requested action. Wrong-workspace/issue/action/batch cases and identifier compatibility are tested.
- **C3 closed:** preview/results reject unselected and duplicate identities; preview revisions remain those submitted; nested receipt identity and action agree with the outer row and request; known success counts must match receipts. Import response rows are checked against the selected row numbers, matching the current backend's selected-row response loop. Schema tests also cover contradictory counts and unsafe numeric revisions.
- **C4 closed:** generic issue update/delete, label/attachment/metadata/property, comment create/update/delete and issue-reaction handlers now invalidate the known affected triage detail and workspace lists. Tests invoke the actual realtime handlers and verify unrelated details/workspaces/history remain untouched; unloaded pending updates refresh matching lists.
- Independent focused command (same five paths listed below): **5 suites / 133 tests passed**, 00:57:42, exit 0. Independent scoped `git diff --check` passed.
- Inspected implementation-owner logs: full core **205 suites / 2,504 tests passed** at 00:38; typecheck and scoped lint completed without errors. These broader checks were not rerun by the reviewer. The owner also supplied five parsed backend wire fixtures; full import/notification/browser integration remains outside this lane.

## Original findings, now resolved

### C1 — High: an offline mutation can resume in another workspace

`packages/core/triage/mutations.ts:16`, `:25`, `:34`, `:70` and the equivalent other hooks close `mutationFn` over the render's `wsId`. `onMutate` captures scope for cache callbacks only. TanStack updates options of a pending observer; with the default online network mode, an offline mutation pauses before calling its function. If the hook rerenders for workspace B, resuming the old workspace-A intention calls B's updated function. Create/settings/import-preview have no existing issue ID to stop a real write to B.

Verified against the installed `MutationObserver` with `retry:false` and the same closure/context structure: set offline, mutate on A, await paused state, replace observer options with B, reconnect. Output was:

```text
paused before workspace switch: true
[{"sentWorkspace":"workspace-B","requestId":"stable-request"},{"cachedWorkspace":"workspace-A"}]
```

Freeze request scope in the operation itself, or explicitly disable deferred offline queuing so it fails in the original workspace. Add a hook regression that starts offline, rerenders with another workspace, reconnects and proves that neither request destination nor receipt cache changes. The current navigation test switches only after the request has already been sent and cannot catch this case.

### C2 — High: parsed responses are not bound to requested resource/workspace identity

`packages/core/api/client.ts:1491`, `:1498`, `:1506`, `:1532`, `:1540`, `:1571`, `:1579` return shape-valid responses without checking their workspace, issue/action or batch identity against the request. `TriageActionResultSchema` only checks internal item/action agreement (`api/triage-schemas.ts:26`). A valid receipt for another issue still resolves as success; a list/detail response for B returned to A's query is automatically cached under A's query key. The `cacheTriageItem` guard is insufficient: queries do not use it, and declining a cache write does not reject the mutation or prevent the UI closing the submitted form.

Validate every available returned identity against the requested scope before resolving. Account for UUID normalization and the supported issue-identifier lookup, and bind action receipts to the requested action. Tests need wrong-workspace lists/details, wrong-issue receipts, wrong retry action IDs and wrong import batch IDs, with rejection and no success/cache advance.

### C3 — High: malformed batch receipts can expand selection or report another issue successful

`packages/core/api/triage-schemas.ts:36–40` accepts any preview IDs and accepts outer result `issue_id=A` with an internally valid nested action/item for B. `api/client.ts:1548`, `:1556` do not correlate rows with the submitted selected IDs or reject duplicate rows. The UI uses preview IDs to construct commit requests and outer success IDs to remove selected items (`packages/views/triage/triage-batch-dialog.tsx`), so shape validation alone violates selected-only processing and can falsely clear A.

Reject unrequested/duplicate preview or result IDs, require outer/nested receipt agreement, preserve submitted revision identity in preview, and ensure every known success has the matching committed receipt. Apply equivalent batch/row correlation to import commit responses while respecting the documented cumulative-versus-selected response shape. Include count/row contradiction tests where they affect success reporting. These are response-boundary checks, not replacements for server authorization.

### C4 — Medium: generic issue/comment events do not refresh triage-owned revisions

At the reviewed snapshot, `packages/core/realtime/use-realtime-sync.ts:1003–1065` and the comment handlers update ordinary issue caches only. Triage detail has its own `issue.revision`; comments, labels, attachments or safe title edits can bump that revision without a `triage:updated` event. The queue eventually polls, but an already selected triage-detail/action form can remain stale and repeatedly conflict. Refresh the affected workspace's triage projections for generic owner-changing events. The root already assigned this follow-up to the core owner; verify real event wiring, including labels/attachments/comment and delete, not just helper calls.

## Specification and test assessment

- All 14 JSON endpoints plus authorized CSV download exist; mixed action/import history, fields, bulk operations, explicit execution retry and import replay have matching public types.
- Workspace query keys include filters and pages. Counts are independent of UI filters. Queue/count refresh is 30 seconds. Mutations do not optimistically remove decisions or infer formal list membership.
- Missing legacy admission defaults to `not_required`; unknown, null and malformed admission inputs fail closed through the helper. Triage item schema requires an explicit admission and positive integer revision. Installed Zod rejects zero/negative/fraction/string/unsafe revisions; the existing named tests only cover absent revision, so add the boundary matrix when touching these schemas.
- Settings catches only `ApiError` 404 as unsupported; 403 and parsing failures propagate. The existing test covers 404/403, but not a network failure or 500. No code defect was found in that path.
- The initial API tests exercised one valid fixture and `{}` per endpoint, four internal receipt consistency cases, encoded CSV paths, request-ID retry and admission states. Initial hook tests covered nonoptimistic behavior, conflict invalidation, already-sent navigation, older receipt revision, mixed batch status and preview-only caching. C1–C3 coverage was missing in that snapshot and has now been added as described above.
- Independent command: `pnpm --filter @multica/core exec vitest run api/triage-client.test.ts triage/queries.test.ts triage/mutations.test.tsx realtime/use-realtime-sync-ws-instance.test.tsx diagnostics/diagnostic-context.test.ts` → **5 suites / 93 tests passed**, 2026-10-05 00:19 local. This includes incumbent realtime/diagnostic tests; it is not 93 distinct T1 scenarios.
- This review did not rerun typecheck/lint or the full core suite; their latest logs were inspected on re-review. Database races, installed-client behavior, complete backend wire coverage, Web/Desktop flows and the complete T1 requirement ledger still require integration proof.

## Incidental integration handoff

While comparing backend DTO shapes, `server/internal/handler/triage_queries.go` was observed calling `actOnTriageItem` before checking a repeated batch issue ID. A duplicate snooze/reviewer row could mutate before being reported invalid. Sent separately to the root for backend ownership; it is not counted as a core-lane finding.
