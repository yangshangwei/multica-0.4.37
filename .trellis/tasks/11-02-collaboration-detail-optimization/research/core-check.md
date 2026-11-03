# Core review — 2026-11-03

Bounded native `trellis-check` pass over every Core file listed in
`core-implementation.md`, against R02–R10 and R13–R15, the PRD/design, the
Server wire contract and the Core UI contract. Loaded the complete saved native
hook context and CLAUDE.md. No subagents, Server/Views/E2E/spec edits, database
operations, provider calls, dependencies or commits were used.

## Findings (fixed)

### Missing optional delivery history revoked authorized state

- File: `packages/core/api/client.ts:1624`
- Issue: The new history read used the ordinary collaboration transport. An
  older server's missing-route 404 reached the issue access guard, cancelling
  the query and erasing its authorized issue projection and scoped drafts.
- Fix: Reuse `fetchCollaborationOptionalHistory`. A missing route becomes
  unavailable and revalidates the parent; a structured object `not_found`
  continues to erase protected state. The regression failed before the fix
  (1 failed/7 passed at 08:51:10 +08:00) and passed afterward.

### Committed role/file refusals lost receipts and erased unsent input

- File: `packages/core/api/client.ts:1675`
- Issue: The workflow helper discarded every 403/404 body. Actual Server
  committed refusals contain `status`, `code` and a positive decimal
  `collaboration_revision`; `prepareDelivery` emits `comment_not_found`/404
  and attachment domain refusals. The new recognized refusal map could never
  settle those POST outcomes. Contradictory declared receipts also fell through
  to access-denial cleanup instead of retaining uncertain original intent.
- Fix: Only the workflow POST helper handles declared 403/404 receipt
  envelopes. It preserves a recognized, schema-valid receipt with matching
  HTTP/body status and no delivery confirmation. A malformed/contradictory
  declared receipt is unavailable. Plain access errors, GET/read denials and
  obsolete responses retain their existing guards. No generic fetch or
  `ApiError` constructor contract changed.
- Evidence: Three committed-refusal cases failed before the positive fix
  (08:53:51 +08:00); four contradictory/malformed cases failed before the
  envelope correction (09:00:19 +08:00). Tests also verify retained result
  prose/files and issue data, erased genuine denials, and redacted API logs.

### A valid uppercase result draft UUID escaped retirement

- File: `packages/core/collaboration/operation.ts:65`
- Issue: Action parsing accepts uppercase UUIDs while stored delivery records
  normalize them. The exact form lookup compared the unnormalized observation,
  leaving a confirmed consumed result form alive for a later mount.
- Fix: Normalize the captured delivery draft UUID before the bundle lookup.
  Original request keys, prose, file confirmation and unrelated comment/new
  bundles retain their identities. The deterministic uppercase regression
  failed before the fix (1 failed/29 passed at 09:03:01 +08:00); it now passes.

## Findings (not fixed)

- Documentation scope: `collaboration-workbench.md:64` still describes only the
  old statusless `stale_revision` refusal path. Root should synchronize the
  documented recognized refusal set, declared POST receipt boundary, protected
  delivery history, result confirmation/retirement and manual preparation gates.
  Spec files are outside this lane's write scope.
- Planning wording: `design.md` called the existing delivery form encrypted.
  The actual path is `delivery-form.ts:24` → `delivery-draft.ts:86,93` →
  `platform/storage.ts`, which persists client form/intent JSON directly in
  localStorage. Root confirmed that the source brief requires preserving scoped
  recoverable client intent and encrypted **server upload bytes**, without a new
  client crypto/key lifecycle. The pre-existing plaintext client storage is
  preserved; root owns correcting the PRD/design wording and retaining this
  concrete limitation. No client-at-rest encryption is claimed by this review.

No additional mechanical Core defects remain in the reviewed paths.

## Contract verification

- Delivery/policy fields are optional for old servers and strict when present.
  Submission, issue, lifecycle, author-summary, decision and selected-file
  identities remain validated; legacy/unavailable facts never grant acceptance.
- Pending acceptance captures retained evidence IDs internally, confirms those
  IDs and the exact submission, and omits rebinding fields from its write.
- Result forms use their own purpose/draft slot. Success/recovery retires only
  the matching confirmed bundle; unknown receipts and refused forms retain
  intent, and authoritative access loss still erases protected scope.
- History enforces bounded pages, descending exact sequences, immutable-record
  consistency and cursor progress under the issue/server/viewer/access guard.
- Manual policy blocks new Resume/resource/Switch preparation while retaining
  Stop, revocation, withdrawal and original recovery. Unknown policy is not
  represented as manual enforcement; existing preview codes remain extensible.
- The new account/delivery schema leaves contain no request-bearing imports.
  React Query owns result/history server state; the reused draft modules retain
  client intent. Existing MCP partial-import and API initialization suites pass.

## Verification

- Lint: **pass**, `pnpm --filter @multica/core lint` (exit 0, final run).
- TypeCheck: **pass**, `pnpm --filter @multica/core typecheck` (exit 0, final run).
- Tests: **pass**, 28 affected suites / 559 tests, started
  2026-11-03 09:08:22 +08:00, duration 6.18 seconds, exit 0.
- Static diff check: **pass**, `git diff --check -- packages/core` (exit 0).

Final test command:

```sh
pnpm --filter @multica/core exec vitest run \
  collaboration/delivery-result.test.ts collaboration/result-operation.test.ts \
  collaboration/delivery-history.test.ts collaboration/presentation.test.ts \
  collaboration/action.test.ts collaboration/operation.test.ts \
  collaboration/delivery-form.test.tsx collaboration/comment.test.ts \
  collaboration/control-schema.test.ts collaboration/control.test.ts \
  collaboration/resource-schema.test.ts collaboration/resource.test.ts \
  collaboration/switch-schema.test.ts collaboration/switch.test.ts \
  collaboration/lifecycle-schema.test.ts collaboration/handoff-schema.test.ts \
  collaboration/schema.test.ts api/collaboration-workflow.test.ts \
  api/collaboration-lifecycle.test.ts collaboration/access.test.ts \
  workspace/mcp-template-queries.test.ts autopilots/webhook.test.ts \
  api/mcp-deployment-client.test.ts collaboration/delivery-api.test.ts \
  collaboration/delivery.test.ts api/collaboration-content.test.ts \
  collaboration/attachment-schema.test.ts collaboration/content-schema.test.ts
```

This pass changed `api/client.ts`, `collaboration/operation.ts` and the adjacent
`api/collaboration-workflow.test.ts`, `collaboration/delivery-history.test.ts`,
`collaboration/result-operation.test.ts`, plus this report. The implementation's
other Core hunks remain intact. Corrections reuse existing transports, schemas,
access guards and bundle retirement; no second state store was introduced.

The implementation lane's earlier 331-file/4,433-test run predates its final
additions and these review fixes. It is not counted as fresh evidence here;
the parent explicitly requested an affected-suite pass. Actual Server atomic
behavior, current HTTP integration, production Web and Electron acceptance,
and the eventual full-scope review remain parent-owned and were not run by this
bounded Core pass.
