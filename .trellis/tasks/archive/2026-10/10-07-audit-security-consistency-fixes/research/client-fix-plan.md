# Client remediation plan

## R3: iteration edit baseline

The root cause is in `packages/views/iterations/iteration-form.tsx`: mount-only field state is combined with the live `iteration.revision`. `useIterationCommand` already provides exact request identity, unknown-result recovery, access/session fences and awaited query invalidation. Do not replace or duplicate it.

The form should own one editable snapshot: baseline values and revision together with current draft values. A clean form may adopt a new query snapshot atomically. A dirty form retains its baseline when props refresh. Submit only changed, currently editable fields against that baseline revision. Never substitute the latest prop revision simply because it exists.

For a genuine revision conflict, retain local input and use the existing `RevisionConflictCompare` pattern (see `projects/components/project-description.tsx`) to show the current server values and local changes with meaningful field labels. Provide an explicit way to adopt the server snapshot or rebase the user's changed fields onto the reviewed server snapshot. Rebasing must preserve server changes to untouched fields; it is never automatic on a remote refresh. Do not display API JSON, UUID-only coordinator values, or revision internals as the product flow.

After known success and a successful resource refresh, adopt the same authoritative snapshot into baseline and visible fields. A refresh failure must not invent a new revision or resubmit a committed command; retain a refresh/retry path. `IterationWriteResult` does not contain the entity revision, so it cannot be treated as an updated Iteration. `useIterationCommand` clears durable pending state only after a known result and then invalidates queries; query refresh can fail independently of the committed write.

Unknown-result recovery continues to send the stored original request through `useIterationCommand`. Disable mutation/rebase actions while that exact command is pending. A loaded pending edit's body may be older than current props and must not be rewritten. Permission/session resets remain owned by the existing hook.

Expected files: `packages/views/iterations/iteration-form.tsx`, its canonical `.test.tsx`, and only the required English/Simplified-Chinese `projects.json` strings. A small pure editable-field/baseline helper may be colocated if it meaningfully simplifies tests; do not add a persistent store, public API or general form framework. Read the conventions documents before naming/translations.

Regression cases: old baseline plus newer props remains old expected_revision; untouched remote fields are not patched; clean refresh synchronizes baseline and fields; conflict retains local input; explicit server/rebase choices behave as described; known success supports a second edit; failed post-commit refresh cannot reapply a stale edit; unknown-result retry preserves request identity and payload; status changes cannot leave forbidden fields writable.

## R4: project-scoped component lifetime

`project-detail.tsx` already keys `ProjectSquadSection`, `ProjectRiskIssues` and `ProjectIssueSurface`, but not `ProjectOverviewPanel`. Add a stable workspace/project identity key at that existing overview boundary. This resets nested composer, preview, pagination, correction target and timezone input in one place. Avoid changing tab-store navigation or remounting on revision changes.

`ContentEditor` deliberately treats defaultValue as mount-only (`content-editor.tsx:820`). Its flushPendingOnUnmount behavior is correct. A new editor lifetime must still flush the outgoing text through its old project-bound callback. Existing progress drafts are already scoped by server/workspace/project/update in `progress-draft-store.ts`; no store format or migration is needed.

Canonical regression must render the actual overview/detail boundary rather than rendering an unkeyed ProjectProgress in isolation and expecting that standalone component to solve ancestor ownership. Seed A and B query data to prevent an incidental loading unmount. Enter an A draft and navigate within the same tab before its debounce expires. Confirm A's saved draft remains under A, B has no A composer/preview/target, opening B restores only B's own draft, and returning to A restores A. Keep existing direct composer tests for preview/publish recovery.

Expected files: `packages/views/projects/components/project-detail.tsx`, `project-management.test.tsx` or `project-overview-lifecycle.test.tsx`/a colocated dedicated navigation regression; a focused Desktop route test only if needed to prove the same-tab adapter path. No global reset, broad navigation refactor or duplicate store.

## Existing evidence and validation

Audit probes are outside the repository at `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-client-audit-ac1kddig`. Convert only their meaningful new scenarios into canonical tests, not their copied surrounding suites. Both defects were runtime-reproduced; current baseline is `4a9be02e5`.

Use focused `pnpm --filter @multica/views exec vitest run ... --maxWorkers 2`, then shared core/views tests and typecheck/lint. No new dependencies. For any new DOM-free helper, use the node test environment; DOM scenarios remain mounted tests. Tests must fail for the demonstrated behavior before code changes.
