# S2 resource offers: bounded frontend implementation

- Date: 2026-10-21.
- Task: `10-20-collaboration-gap-recovery`, approved implementation.
- Owner: native `trellis-implement` lane `s2_resource_frontend`.
- Checkout: `/Volumes/artisan/code/2026/multica-0.4.37`.
- HEAD during verification: `8ecb98883504443a6ec1a5fb4a4c6295a985ae04`; results include the uncommitted S2 files below and the coordinator's API/barrel/locale integration.
- This is client implementation and focused verification evidence. It does not complete AC-I03, the parent task, or production/native acceptance.

## Delivered behavior

The resource panel now lets a participating actual owner explicitly supply a selected agent or complete squad from their own visible workspace inventory. The creator or handler can inspect and adopt only a supplied offer. Adoption uses the existing resource operation path; each actual owner then uses the existing independent consent form. Supply, adoption and withdrawal never invite, authorize another owner, stop work or resume execution.

The new offer schema validates protocol, identity, complete unique roster, workspace/owner/runtime correspondence, explicit terminal state facts and bounded cursor pages. It strips undisclosed fields from the public summary. Existing resource inputs, receipts and request mapping now include `offer_resource`, `adopt_resource_offer` and `withdraw_resource_offer`. Success requires the correct action/status/code, exact offer ID/version and correct issue/target scope without a grant. Adopt/withdraw responses must match the original offer ID and the exact expected-version increment. Redacted recovery remains the strict seven-field contract.

Queries remain under Server/viewer/issue access epochs with `staleTime: 0`, `retry: false` and request cancellation. The coordinator's three ApiClient readers use the existing optional-resource boundary: local missing offers do not revoke the issue, while explicit issue denial clears protected data. A delayed success or denial cannot affect the recovered epoch or retire a newer draft.

Drafts use the existing `target_resource` domain and original resource operation recovery. Supply freezes the explicit workspace and resource roster without a target-set observation. Adoption freezes the offer version and target-set version. Withdrawal remains available to the supplying participant on stale resources or a closed issue and is separate from revoking an adopted resource's grant. Client gates use current control/write context and roles, including the parser's deliberately false `submit_command` capability.

The shared UI reuses the original candidate, owner/runtime summary, confirmation and consent components. It hides old inventory contents after a current error/visibility change and disables a changed selection. A known refusal retains selection; only an explicit refresh/reselection adopts a new authority baseline. An unknown result closes the form into the existing retained-request recovery UI. Organizer read authority remains separate from nonterminal adoption authority so closed-issue offer history remains visible.

## Changed files and integration boundary

New files:

- `packages/core/collaboration/resource-offer-schema.ts`
- `packages/core/collaboration/resource-offer-schema.test.ts`
- `packages/core/collaboration/resource-offer-test-fixtures.ts`
- `packages/views/collaboration/resource-offers.tsx`
- `packages/views/collaboration/resource-offers.test.tsx`

Extended files:

- `packages/core/collaboration/resource-schema.ts` and `resource-schema.test.ts`: three action/input/receipt/request branches and their correlation checks.
- `packages/core/collaboration/resource.ts` and `resource.test.ts`: scoped offer reads, preparation, role gates and original-operation recovery tests.
- `packages/core/collaboration/resource-api.test.ts`: real ApiClient integration tests for the three new readers.
- `packages/views/collaboration/resource-panel.tsx`: import and mount of the new offer subcomponent only.
- `packages/views/collaboration/resource-form.tsx`: explicit `purpose`, optional explicit baseline refresh, current candidate matching, reusable minimal summary and refusal-preserving confirmation. Existing Register/Consent/Revoke/Switch paths keep their API contracts.
- `packages/views/collaboration/labels.ts`: offer success, terminal, stale, missing and ownership messages.
- `packages/views/collaboration/execution-panel.tsx`: **one result-display allowlist change**, adding `offer_resource`, `adopt_resource_offer`, `withdraw_resource_offer`. Its existing resource draft parser/dispatcher already routes all three; no cancellation/history/control branch was changed. S3b can continue from this file.

The coordinator owns and integrated `packages/core/api/client.ts`, `packages/core/collaboration/index.ts` and both collaboration locale files. This lane did not edit those shared files, backend/SQL/generated files, comments/mentions, dependencies, task status, commits, environments or release gates. The pre-existing baseline in `/tmp/multica-gap-recovery-20261021-h6phxwo9/files/` was preserved.

Public helpers supplied to the coordinator:

- Pure schemas/types/parsers: `CollaborationResourceOfferSchema`, `CollaborationResourceOffersPageSchema`, `CollaborationResourceOffer`, `CollaborationResourceOffersPage`, `parseCollaborationResourceOffer`, `parseCollaborationResourceOffersPage`.
- Queries/gate: `collaborationResourceOffersOptions`, `collaborationResourceOfferOptions`, `collaborationResourceOfferCandidatesOptions`, `canOfferCollaborationResources`.
- ApiClient readers: `getCollaborationResourceOfferOptions`, `listCollaborationResourceOffers`, `getCollaborationResourceOffer`; all writes and recovery continue through `writeCollaborationResource`.
- UI copy uses top-level `resources` camelCase keys, including coordinator-added `previousOffers`; there is no `execution.actions` namespace.

## Failure-first evidence

1. `pnpm --filter @multica/core test collaboration/resource-schema.test.ts`: 5 new failures, 6 existing passes. The old parser rejected the three new receipts/inputs and their exact offer request mapping.
2. `pnpm --filter @multica/core test collaboration/resource.test.ts`: 7 new failures, 4 existing passes. Missing offer queries/preparation and role gates prevented the new flows.
3. `pnpm --filter @multica/views test collaboration/resource-offers.test.tsx`: 5 failures before UI implementation; supply/adopt/withdraw controls were absent.
4. The first UI/typecheck iteration exposed a test label mismatch (`Dismiss` versus the existing `Close` copy), TypeScript discriminant narrowing and typed test-cache fixtures. These were corrected without changing the existing copy or authority contract.
5. `DEBUG_PRINT_LIMIT=700 pnpm --filter @multica/views test collaboration/resource-offers.test.tsx -t 'closed-issue|adopts current authority'`: 2 focused failures before correcting closed-issue read gating and explicit refresh of a refused supply's authority. The other 5 tests were excluded by this diagnostic filter, not counted as passes. The final unfiltered run below ran all 7 offer tests.

## Final verification

All commands below were actually run in this lane. None launched an agent provider, browser, server or database.

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/core test collaboration/resource collaboration/switch` | Exit 0; **7 files, 63 tests passed**, no failing/skipped tests. Includes new offers plus adjacent resource and Switch contracts. |
| `DEBUG_PRINT_LIMIT=1500 pnpm --filter @multica/views test collaboration/resource-offers.test.tsx collaboration/resource-panel.test.tsx collaboration/execution-panel.test.tsx` | Exit 0; **3 files, 15 tests passed**, no failing/skipped tests. |
| `pnpm --filter @multica/core typecheck` | Exit 0. |
| `pnpm --filter @multica/views typecheck` | Exit 0. |
| Core targeted ESLint command below | Exit 0. |
| Views targeted ESLint command below | Exit 0. |
| `git diff --check` | Exit 0. |

Core ESLint, run from `packages/core`:

```bash
pnpm exec eslint collaboration/resource-schema.ts collaboration/resource-schema.test.ts collaboration/resource-offer-schema.ts collaboration/resource-offer-schema.test.ts collaboration/resource-offer-test-fixtures.ts collaboration/resource.ts collaboration/resource.test.ts collaboration/resource-api.test.ts
```

Views ESLint, run from `packages/views`:

```bash
pnpm exec eslint collaboration/resource-panel.tsx collaboration/resource-form.tsx collaboration/resource-offers.tsx collaboration/resource-offers.test.tsx collaboration/execution-panel.tsx collaboration/labels.ts
```

The pnpm invocations print the existing root configuration warning that `pnpm.onlyBuiltDependencies` / `pnpm.overrides` are ignored in their old location. This lane did not alter package manager configuration or introduce dependencies.

Canonical core tests cover malformed/old/private summary data, state/roster/identity correspondence, exact operation correlation, original GET recovery, stale refusals, optional 404s, cursor loops, actual ApiClient account replacement and delayed denial/success after object-authority recovery. Views tests cover the source-hidden supplier -> organizer adoption -> two independent owner consents, no automatic Stop/Resume/invite/register, stale adoption, lost withdrawal, inventory disappearance, warm-cache organizer loss, closed-issue history/withdrawal and explicit refreshed resubmission. They use mocked HTTP and the actual shared components/ApiClient; they do not repeat the backend's complete authorization/concurrency matrix.

## Remaining coordinator work and limits

- Production Web and actual Electron offer-flow verification, current wide/narrow/200% screenshots and visual verdict remain coordinator-owned and **not run by this lane**.
- The coordinator recorded an existing-style reference before the first UI iteration at `.omx/reports/cwc-gap-recovery-20261021/visual-baseline.json` and `.omx/state/collaboration-gap-recovery/ralph-progress.json`. That is historical style evidence, not new offer-flow acceptance. No subsequent style changes were made in this lane.
- Full repository `make check`, `pnpm knip`, UI exports/native file-access checks, independent Trellis review, updated specs and the 97-item acceptance ledger remain part of coordinator integration. This report does not claim those checks passed.
- No backend, race, migration, native provider, real provider, packaged installer or cross-OS checks were run here. Existing gates remain under the parent task's contract.
- No commit, archive or external publication was performed.
