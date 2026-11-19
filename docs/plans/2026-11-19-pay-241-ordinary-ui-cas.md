# PAY-241 ordinary UI revision implementation plan

> Execution: native `trellis-implement` / `trellis-check` agents in the existing S3 task. Main owns environment, source binding, integration, specs and directed Lore commits.

**Goal:** Ordinary priority/assignee UI writes retain the numeric revision actually observed by that UI, produce a visible same-version conflict, and permit explicit fresh continuation while preserving the old versionless API.

**Architecture:** Use existing `UpdateIssueRequest.expected_revision`, Core transport/optimistic exclusion/rollback and shared UI field owners. Preserve an already supplied token through the existing optional run-confirm `issueRevision` field. A batch uses one token only when every selected rendered issue has the same known positive revision. Current cache remains classification/authority input; it does not replace the UI observation.

**Tech Stack:** Existing TypeScript, React Query, Core/Views, Next.js, Electron, Playwright and Go API. No new dependency, backend field, migration or fan-out.

## 1. Fixed boundary and ownership

Source intake: `.trellis/tasks/11-13-pay-241-s3-recovery/research/ordinary-ui-cas-intake-20261119.md/json`. Output capability is independently accepted and committed as `89740ed14`; preserve all other WIP and original failures. Allocate new private sources/configuration/database before executable verification.

The product owner may modify exactly these eight existing paths and the named owning-layer tests:

| Product path | Required behavior | Tests |
| --- | --- | --- |
| `packages/core/issues/mutations.ts` | Export a pure supplied-observation helper; mutation/cache behavior stays with existing owners | Node `issues/field-revision.test.ts`, existing `issues/mutations.test.tsx` controls |
| `packages/views/issues/actions/use-issue-actions.ts` | Capture rendered issue revision before gate or surface/direct delegation | `actions/__tests__/use-issue-actions.test.tsx` |
| `packages/views/issues/components/board-card.tsx` | Capture card revision and update callback dependencies | `components/board-card-assignee-picker.test.tsx` |
| `packages/views/issues/components/table-view.tsx` | Capture actual rendered row before gate/delegation | `components/table-view-editing.test.tsx` |
| `packages/views/issues/components/issue-detail.tsx` | Capture SubIssueRow's child revision and reuse localized conflict feedback | `components/issue-detail.test.tsx` |
| `packages/views/issues/actions/run-confirm-gate.ts` | Echo supplied `expected_revision` into existing optional intent `issueRevision` | `actions/run-confirm-gate.test.ts` |
| `packages/views/modals/run-confirm.tsx` | Retain captured revision in both single/batch Go and suppress/handoff submissions | `modals/run-confirm.test.tsx` |
| `packages/views/issues/components/batch-action-toolbar.tsx` | Use a token only for a nonempty selected rendered set with one known positive revision; keep partial/outcome rules | `components/batch-action-toolbar.test.tsx`, `.confirm.test.tsx` |

The separate E2E owner owns new `e2e/collaboration-recovery-ordinary-ui.spec.ts` and `e2e/fixtures/collaboration-recovery-ordinary-ui.ts`. It may make export-only reuse changes to `e2e/fixtures/collaboration-recovery.ts`; replacing its obsolete ordinary-UI omission assertion requires a separately recorded exact change. Do not alter accepted raw reports, unrelated helpers or client product files.

## 2. Pure observation and real callers

Write meaningful regression cases before product edits: supplied explicit token wins; rendered R survives a newer cache R+1; missing/invalid revision preserves compatibility; collaboration source data does not manufacture an ordinary token; priority and all assignee picker variants retain the captured token. Status-only/completion/drag/drop flows keep their existing semantics.

Implement adjacent to `UpdateIssueMutationInput`, using the supplied issue only:

```ts
export function withObservedIssueFieldRevision(
  issue: Pick<Issue, "revision" | "collaboration"> | undefined,
  updates: Partial<UpdateIssueRequest>,
): Partial<UpdateIssueRequest> {
  if (updates.expected_revision !== undefined || !issue || issue.collaboration) return updates;
  if (updates.priority === undefined && updates.assignee_type === undefined && updates.assignee_id === undefined) return updates;
  const revision = issue.revision;
  return typeof revision === "number" && Number.isSafeInteger(revision) && revision > 0
    ? { ...updates, expected_revision: revision }
    : updates;
}
```

Use this helper at each rendered write owner. Do not auto-fill inside `useUpdateIssue`, `onMutate`, `findSourceIssue` or a surface cache lookup. Preserve any explicit token, current collaboration routing and synchronous optimistic cancellation/rollback. Existing Core excludes `expected_revision` from optimistic entity patches.

Run Core's new Node suite and the existing mutation controls through a closed source-bound recorder. Run each changed Views suite against its actual caller/gate behavior. Retain RED and any fixture failures; fresh GREEN uses new labels. Tests must assert sent values and retained intent, not duplicate the helper matrix through every DOM mount.

## 3. Confirmation and common-revision selection

The existing run-confirm intent already has `issueRevision?: number`. Populate it only from a caller's supplied token, and consume it in modal `applyTo`; do not add revision to `GateIssue`, reread cache, renew the token on confirm, or change run-start/cancel semantics.

For batch toolbar, prove every selected rendered issue has the same safe positive revision before calling the helper with that observation. Retain the same token through Agent/squad confirmation before batch handling. Missing, empty or heterogeneous observations do not get a fabricated token. Keep source-collaboration refusal, status/backlog/completion, ordered partial/unknown outcomes and selection retention.

Regression checks cover Go/suppress/handoff/cancel for single and common-revision batch, absent tokens, stale observations, and existing assignment/completion decisions.

## 4. Actual two-client RED/GREEN

Freeze the baseline product before any edits for RED. Build matching production Web/native renderer for that baseline and use a fresh owned database/configuration. The E2E fixture must capture genuine picker PUTs using existing `HeldRequest.route.continue()` and exact `response.request()` association. It cannot add the token in an interceptor, `page.evaluate`, synthetic PUT or fetch/fulfill.

Use two complete platform tops, each with priority-first and assignee-first ordering plus a separately labelled versionless API control. Each ordering uses a distinct ordinary child under the protected collaboration root and two actual authorized UI sessions. Prefer two distinct source members when existing setup supports them; otherwise two separately authenticated clients for the same valid member still provide the required client/version competition.

1. Capture both actual child GET observations at numeric R and baseline displayed fields.
2. Hold each child's genuine PUT before clicking either real picker. Change low priority to high in one window and clear the human assignee in the other; await both requests before forwarding either.
3. Baseline RED must show the genuine requests omit required `expected_revision: R`. Release/collect every held request on failure and preserve that whole round.
4. GREEN forwards the selected winner naturally: exact 200 then exact `409 revision_conflict`, resource UUID/type, expected R and actual R+1. The losing field stays unchanged.
5. Verify existing visible conflict feedback, real authoritative refresh and the winning field in the renderer. Preserve the original failed request/body/result.
6. Explicitly reopen the losing picker after a fresh R+1 observation, capture a new genuine PUT with R+1, require 200 and final R+2 with both intended fields. Both clients converge; no automatic resend/token replacement.
7. Repeat the opposite order on a new child. The separate direct API control omits the token and succeeds under the existing compatibility contract.
8. Reconcile child revisions/fields, parent collaboration/history/session and zero new run/command/dispatch, source/config/build/PID/native/gate/connection cleanup. New failures always get new sources/configuration/database/run labels.

Exact discovery/execution shape:

```text
playwright test e2e/collaboration-recovery-ordinary-ui.spec.ts --list
playwright test e2e/collaboration-recovery-ordinary-ui.spec.ts --workers=1 --retries=0 --max-failures=0 --reporter=json
```

Main supplies exact absolute executable/configuration/argument permits. Static/discovery success is not actual UI acceptance. The old field R06 UI omission assertion must be clearly superseded in current source while its accepted historical raw evidence stays immutable; versionless API coverage remains separately named.

## 5. Check, integrate and finish S3

Native independent check reviews all eight product paths, regression matrices, pure/static results and the complete fresh client round. Main integrates only accepted owned deltas, preserving other WIP/modes, then verifies Root compatibility and updates executable specs. Directed local Lore commits exclude unrelated WIP and remain unpushed.

Use the exact parent TC-16 definition and accepted scene to decide its status; missing-revision/heterogeneous-batch/other-platform guarantees are not silently inferred. Then finish the whole S3 R01–R09 source/evidence/resource reconciliation and independent quality gate. Archive only after S3's own AC pass. Real models, funds, production publication, S4/S5 and business signatures remain separately authorized.
