# Core implementation and verification

Core-owned continuation on 2026-11-03. No Server/Views/spec files, database,
provider account, dependency or commit was changed by this lane.

## Delivered contracts

- Added pure account/delivery schema leaves so ApiClient imports no request-bearing
  modules. Delivery history validates exact issue/submission/author/decision
  identities, bounded descending sequences and cursor progress. Present malformed
  evidence/policy is unavailable; absent old-server fields remain unknown.
- Added result/basis/evidence inputs, `return_acceptance`, `change_ai_policy`,
  self-result completion and exact pending acceptance. Generic change/reopen
  catalogs cannot manufacture review. Result/basis enforce nonblank 65,536 UTF-8
  byte limits without trimming stored prose.
- Kept one existing operation/draft lifecycle. The distinct `result` delivery form
  stores basis/source-comment snapshot without sharing the discussion slot.
  Creation stores `aiPolicy`, defaulting new forms to assisted, and sends
  `ai_policy` with the immutable original creation input.
- Result retirement requires exact content-free `delivery_confirmation` even for
  text-only work and original-key recovery. Pending acceptance freezes
  `submissionAttachmentIds` internally; those retained files are confirmed,
  never rebound. Only the consumed result bundle retires; concurrent comments
  and unsent refused forms remain intact.
- Formal unknown/contradictory receipts retain original intent. Recognized final
  refusal code/status tuples settle inside the access guard. Statusless original
  GET refusals remain refused; late account/access epochs still fence results.
- Added pure stage/next-action and AI-presence helpers. Exact pending acceptance
  blocks generic transfer/status, information returns use the recorded recipient,
  and legacy evidence never fabricates acceptance. Supported empty consoles can
  collapse while selection, facts, errors, unknown reads and recovery stay visible.
- Manual tasks deny new Resume/resource/Switch preparation and accept the exact
  `task_manual` 409 refusal across their receipt schemas. Stop, grant revocation,
  offer withdrawal and original recovery remain usable.

## Changed files

New production files:

- `packages/core/collaboration/account-schema.ts`
- `packages/core/collaboration/delivery-schema.ts`
- `packages/core/collaboration/delivery-queries.ts`
- `packages/core/collaboration/presentation.ts`

Existing production files: `api/client.ts`; collaboration `schema.ts`, `action.ts`,
`operation.ts`, `delivery-draft.ts`, `delivery-form.ts`, `creation-schema.ts`,
`creation.ts`, `index.ts`, `control-schema.ts`, `control.ts`, `resource-schema.ts`,
`resource.ts`, `switch-schema.ts`, `switch.ts`.

Canonical tests were added beside those contracts in `delivery-result.test.ts`,
`result-operation.test.ts`, `delivery-history.test.ts`, `presentation.test.ts`;
existing workflow/lifecycle/delivery-form/control/resource/Switch suites were
extended rather than adding parallel DOM matrices.

## Actual verification

- Initial `delivery-result.test.ts`:11 failed/1 passed before implementation;
  after implementation12/12 passed, then extended to14/14 passing assertions.
- Manual receipt regressions:3 failed before the new refusal contracts; manual
  preparation regressions:2 failed before the admission fences.
- `pnpm --filter @multica/core test`:331 files,4,433 tests passed at08:26:30.
  This whole-package run predates the final pending-evidence/contradictory-receipt
  additions; it is not represented as testing those later lines.
- Final pending-evidence regression reproduced a real missing confirmation
  observation, then passed after internal capture.11 focused files/252 tests
  passed after that repair.
- Final contradictory formal-receipt regressions:3 failed, then
  `result-operation.test.ts`, `action.test.ts`, `operation.test.ts`:53/53 passed.
- `resource.test.ts`:17/17 passed with explicit retained revoke/withdraw proof.
- Final authoritative empty-review repair:2 named regressions failed before the
  bounded root decision was applied; presentation/result-operation/operation
  suites then passed60/60 at08:40:40. Final typecheck, changed-file lint and
  diff whitespace check also passed.
- Full package lint and typecheck passed; later changed schema/operation/test
  files were checked again. `git diff --check -- packages/core` passed.

Earlier local test failures due to equal-millisecond draft ordering, comparing a
combined AbortSignal by object identity, and reparsing an already normalized
execution:null fixture were corrected in tests. They did not change product
sorting, cancellation or API compatibility behavior.

## Parent integration follow-ups

Server owns actual authority/atomic workflow/manual runtime proof. Shared Views
owns the precise legacy repair affordance, compact execution/history entry and
result/decision UI wiring. Production Web and actual Electron acceptance remain
parent-owned requirements; this Core lane did not execute them or real models.
Root resolved creator-handler review without a linked handoff through explicit
self-result completion only when the present delivery summary authoritatively
reports current:null. Result+basis are required; canAccept stays false. An absent
summary does not grant this repair. The helper and canonical test were aligned;
Server/Views still own actual integration proof.
