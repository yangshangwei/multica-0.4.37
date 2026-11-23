# PAY-241 S2 client increment 1 results

- Lane: client (single owner), `packages/core`, `packages/views`, locales. No file under `server/`, `apps/`, `e2e/`, `packages/ui/`, `.trellis/spec/` was edited.
- Date: 2026-11-14. Branch `codex/cross-workspace-collaboration`, HEAD at the time of the final runs `fb9f4c07059445cea41ed19e956b3eb1861817cc` plus uncommitted work of this and other lanes. No git write command was run.
- Scope: wire-contract actions in state `inc1` only: P1–P3, R1–R3, A1–A5, O1.
- **A5 `confirm_scope` is HELD** on the team lead's instruction (see section 1a). Everything else is complete.
- **No real browser, API, database, Desktop or model was started.** Everything below is unit/component evidence against the frozen contract with stubbed transport.

## 1. Contract state built against

| Read | sha256 of `research/wire-contract.md` | Section 13 |
| --- | --- | --- |
| Start (file mtime 04:27) | `02fb1c2ca6e4eb19abeb75253b07dca4c9861de77d682a59bcf0fd9bf0ceafce` | `(none yet)` |
| Before coding A5 | same | `(none yet)` |
| Before finishing (mtime 06:41) and after the final runs | `516e6dd9066b434cc59d46d44878bb5c2fe3d357582674d445c4612779d2288b` | items 1 and 2 |

- **13.1 followed**: `Account.external` is a strict boolean and each role account is `Account | null`. Schema, panel, roles form and tests were changed after this was read (a null holder renders "Account unavailable", is never a preselected signer, and differs from "Not configured").
- **13.2**: section 10 (ordinary endpoints) only; no client code in this increment.
- The two announced open points — (a) a code-version reference on the A5 child entry, (b) deeper descendants — are **not** in section 13. A5 is built on the current 7.5 shape `{issue_id, content_digest}`. The entry shape is produced in one place (`collaborationStageScopeSelection(view).entries`) and validated in one schema (`CollaborationStageRequestSchemas.confirm_scope`).

## 1a. Update after the team lead's section-13 message

- **13.1 alignment redone as asked**: role holders now use the existing `CollaborationAccountSummarySchema` (`account-schema.ts`) made nullable; the separate stage account schema and type were removed.
- **13.2**: `execution_gated` is rendered as a neutral fact on each child row; admission refusal bodies of ordinary endpoints are not rendered by this increment.
- **A5 held.** The server lane is adding a required per-child `code` member, a confirmed code reference in R1 and descendant admission; section 13 does not record it yet (sha still `516e6dd9…`). `COLLABORATION_STAGE_SCOPE_CONFIRMATION_HELD = true` in `stage-presentation.ts` forces `can.confirmScope` to `false`, so the button is never rendered and `prepareCollaborationStageAction` refuses the action (no key is spent on the superseded shape). The panel still reports the scope as unconfirmed. The existing A5 request schema, `StageScopeForm`, `collaborationStageScopeSelection` and the `confirm_scope` matching follow the superseded 7.5 shape, are unreachable from the UI and are **not final**.
- Tests changed accordingly: the views A5 happy path was replaced by "does not offer the held scope confirmation (A5) even when the server hints it…"; `stage-actions` asserts the prepare refusal; `stage-presentation` asserts the hold.
- Reruns after these changes: six core stage suites 202/202 (`core-stage-hold-run1`); `stage-panel` + `collaboration-detail` 29/29 (`views-stage-hold-run1`); core `collaboration api` 127 files, exit 0 (`core-collaboration-hold`); views `collaboration locales issue-detail` 37 files, exit 0 (`views-collaboration-hold`); `pnpm typecheck --concurrency=1` 9/9 exit 0 (`typecheck-all-hold`); `pnpm lint --concurrency=1` exit 0, 30 pre-existing warnings, 0 in lane files (`lint-all-hold`). Tables in sections 3–5 below describe the state before the hold; the A5 row in section 5 is superseded by this section.

To release A5 once section 13 records it: update `CollaborationStageRequestSchemas.confirm_scope`, `collaborationStageScopeSelection(...).entries`, the R1 child schema, `StageScopeForm` and the `confirm_scope` branch of `matchesCollaborationStageConfirmation`, restore a views happy-path test, then set the hold constant to `false`.

## 2. Files

Created (core):
`packages/core/collaboration/stage-schema.ts`, `stage-intent.ts`, `stage-queries.ts`, `stage-actions.ts`, `stage-presentation.ts`, `stage-test-fixtures.ts`, and tests `stage-schema.test.ts`, `stage-intent.test.ts`, `stage-api.test.ts`, `stage-actions.test.ts`, `stage-queries.test.ts`, `stage-presentation.test.ts`.

Created (views):
`packages/views/collaboration/stage-panel.tsx`, `stage-forms.tsx`, `stage-proposals.tsx`, `stage-labels.ts`, `stage-panel.test.tsx`.

Modified:

| File | Change |
| --- | --- |
| `packages/core/api/client.ts` | +156: eight stage methods and three private helpers |
| `packages/core/collaboration/index.ts` | +7: stage exports |
| `packages/core/collaboration/draft.ts` | +3: stage intents are erased with the other collaboration drafts (see deviation D1) |
| `packages/core/collaboration/test-fixtures.ts` | +3: re-export of the stage fixtures (D1) |
| `packages/views/collaboration/collaboration-detail.tsx` | +2: mounts `CollaborationStagePanel` between requirements and delivered result |
| `packages/views/collaboration/collaboration-detail.test.tsx` | +5/−1: isolates the panel like its other siblings and asserts its position |
| `packages/views/locales/en/collaboration.json`, `zh-Hans/collaboration.json` | new `stage` section, appended textually; every pre-existing member is byte-identical |

`account-picker.tsx`, `creation-form.tsx`, `creation-form.test.tsx`, `field-editing.test.tsx` show as modified in `git status` but belong to another session; they were not touched.

## 3. Commands and exit codes

Logs: `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/s2-client-inc1-*`. Each command redirected to its log and appended `exit=$?`; nothing was piped through `tail`/`head`.

| Command | Log | Exit |
| --- | --- | --- |
| `pnpm --filter @multica/core exec vitest run collaboration/stage-schema.test.ts` | `core-schema-run1` | 1 (1 of 101 failed: a fixture row gave the closed confirmation schema an invalid `role_revision "0"`) |
| same after fixing that test row | `core-schema-run2` | 0 (100) |
| `… vitest run stage-intent stage-api stage-actions stage-queries stage-presentation` (tests written before the modules) | **`core-first-failure`** | 1 (5 files failed; 27 `stage-api` tests failed, 4 suites could not import their module) |
| six core stage suites after implementation | `core-stage-run2` | 1 (1 of 200: my test expected an `ApiError` where the access fence cancels the read) |
| same after correcting that expectation | `core-stage-run3` | 0 (200) |
| `pnpm --filter @multica/core exec tsc --noEmit` | `core-typecheck-run1` / `-run2` | 1 (test typing) / 0 |
| `pnpm --filter @multica/views exec tsc --noEmit` | `views-typecheck-run1` | 0 |
| `… views exec vitest run collaboration/stage-panel.test.tsx` | **`views-stage-run1`** | 1 (4 of 16) |
| same | `views-stage-run2`, `-run3`, `-run4` | 0 (16, 17, 18) |
| `… views exec vitest run collaboration locales` | `views-collaboration-run1` | 1 (2 of 400: 5 s timeouts in `field-editing` and `resource-offers`, see 6) |
| those two files alone | `views-flake-rerun1` | 0 (18) |
| `issue-detail`, `account-workbench-page`, `source-bridge`, `run-navigation` (suites that mount the detail) | `views-detail-mounts-run1` | 0 (106) |
| eslint on every file of this lane | `core-lint-files-run1`, `views-lint-files-run1` | 0, 0 warnings |
| `pnpm --filter @multica/core exec vitest run collaboration api` (final) | `core-collaboration-final` | 0 (127 files, 2473 tests) |
| `pnpm --filter @multica/views exec vitest run collaboration locales issues/components/issue-detail.test.tsx` (final) | `views-collaboration-final` | 0 (37 files, 489 tests) |
| `pnpm typecheck --concurrency=1` (final) | `typecheck-all-final` | 0 (9/9 tasks) |
| `pnpm lint --concurrency=1` (final) | `lint-all-final` | 0 (6/6 tasks) |

Lint warnings: 30 in total (`@multica/views` 27, `@multica/ui` 3), all in files this lane did not touch (agents, issues, runtimes, editor, onboarding, projects, `ui/stepper`). Files of this lane: 0 warnings.

## 4. Named test inventory (final state)

| Suite | Tests | Pass | Fail | Skip |
| --- | --- | --- | --- | --- |
| `core/collaboration/stage-schema.test.ts` | 102 | 102 | 0 | 0 |
| `core/collaboration/stage-actions.test.ts` | 38 | 38 | 0 | 0 |
| `core/collaboration/stage-api.test.ts` | 27 | 27 | 0 | 0 |
| `core/collaboration/stage-presentation.test.ts` | 17 | 17 | 0 | 0 |
| `core/collaboration/stage-intent.test.ts` | 9 | 9 | 0 | 0 |
| `core/collaboration/stage-queries.test.ts` | 9 | 9 | 0 | 0 |
| `core/collaboration/draft.test.ts` (existing, touched module) | 4 | 4 | 0 | 0 |
| `views/collaboration/stage-panel.test.tsx` | 18 | 18 | 0 | 0 |
| `views/collaboration/collaboration-detail.test.tsx` (existing, extended) | 11 | 11 | 0 | 0 |

Per-test names: `logs/s2-client-inc1-core-stage-inventory.json`, `logs/s2-client-inc1-views-stage-inventory.json`. All six core suites start with `// @vitest-environment node`.

## 5. Contract action → code → tests

Transport lives in `ApiClient`; classification, schemas and matching in `stage-schema.ts`; intents in `stage-intent.ts`; prepare/execute/recover in `stage-actions.ts`; Query keys/options in `stage-queries.ts`; UI in `stage-panel.tsx`, `stage-forms.tsx`, `stage-proposals.tsx`.

| # | Client method | Core | Views | Tests |
| --- | --- | --- | --- | --- |
| P1 | `getProjectStagePolicy(wsId, projectId)` with explicit `X-Workspace-ID` | `collaborationStagePolicyOptions`, `parseCollaborationStagePolicy` | `StagePolicy` disclosure | schema "P1 project policy"; api "P1-P3" (malformed, wrong project, denied, not found, busy, schema, absent route); panel "reads the project policy…" |
| P2 | `reviseProjectStagePolicy` (PUT, frozen key/body) | `prepareCollaborationStagePolicyRevision`, `executeCollaborationStageIntent` | policy form; enabling offered only for `activation_available === true` | actions "project policy intents"; api "PUTs a revision…"; panel "renders the temporary 503 … as not available yet" |
| P3 | `getProjectStagePolicyOperation` (GET) | `recoverCollaborationStageIntent` | pending block inside the policy disclosure | actions "recovers by GET: a missing original operation stays unknown and never PUTs"; api "recovers by GET only…" |
| R1 | `getCollaborationStage` | `collaborationStageViewOptions`, `useCollaborationStageView`, `deriveCollaborationStageState` | panel summary, roles, children | schema "R1 stage view" (24 refusals); api "R1"; queries "stage view"; presentation; panel unavailable/unknown/unmanaged/summary tests |
| R2 | `getCollaborationStageProposals` | `collaborationStageProposalsOptions` (cursor chain guard) | `StageProposals` | schema "R2…"; api "R2"; queries "proposal list pages" |
| R3 | `getCollaborationStageProposal` | `collaborationStageProposalOptions`, `collaborationStageProposalActions` | `StageProposalRecord`, decision form | schema "R3…"; api "R3"; presentation; panel accept/accepted-detail tests |
| A1 | `sendCollaborationStageRequest({action:"configure_roles"})` → `POST …/stage/roles` | `prepareCollaborationStageAction`, `collaborationStageRoleConflicts` | `StageRolesForm` | schema request bodies; actions prepare/execute; panel "configures the six roles…" |
| A2 | `…"create_proposal"` → `POST …/stage/proposals` | same | `StageProposalForm mode="create"` | schema content/source limits; actions; api route table |
| A3 | `…"revise_proposal"` → `POST …/proposals/{id}/versions` | same | `StageProposalForm mode="revise" / "modify"` | schema; actions; panel "runs modify-and-accept as two explicit requests…" |
| A4 | `…"decide_proposal"` → `POST …/proposals/{id}/decisions` | same | `StageDecisionForm` (accept / reject), step 2 of modify-accept | schema; matching matrix (16 keep-rows); panel accept, stored-refusal and modify-accept tests |
| A5 | `…"confirm_scope"` → `POST …/stage/scope-confirmations` | `collaborationStageScopeSelection` | `StageScopeForm` | schema; presentation "collaborationStageScopeSelection"; panel "confirms the explicit existing sub-issue set…" |
| O1 | `getCollaborationStageOperation` (existing route, GET) | `recoverCollaborationStageIntent`, `dismissCollaborationStageIntent` | `PendingStageRequests` | actions "GET-only recovery…" (success, stored refusal, 404, 409 incomplete, malformed, contradictory, transport, reload, discard); api "O1"; panel "keeps an unconfirmed request, recovers it by GET only…" |

Recovery contract (3.5 / 4.3) as implemented:

- An intent is stored (`multica_collaboration_stage:` prefix) before the request leaves. It holds only scope, target, action, the frozen `Idempotency-Key`, the proposal route id and the exact body text. A tampered, relocated or non-canonical record is a storage error, not an absent record.
- Retired only by: the exact confirmation (`matchesCollaborationStageConfirmation`), a stored refusal (final for the key; form input stays), or an unstored refusal on the *sole* dispatch of a never-stored intent.
- `GET 404`, `409 operation_incomplete`, malformed, contradictory, transport failure: the intent stays. Recovery never POSTs/PUTs. Resending the same key and body, or discarding the local record, are separate explicit buttons; discard appears only after a check in this mount settled nothing.
- While an issue intent is unconfirmed, new stage writes for that issue are locked.
- The existing generic recovery (`executeCollaborationAction` / `recoverCollaborationAction`) is unchanged.

All ten `stage_confirmation` variants of section 11 are in the closed union; requests exist only for the six increment-1 actions.

## 6. Deviations and decisions to review

- **D1 — two existing files outside the literal ownership list.** `draft.ts` (+3 lines) so that stage intents are erased on object access loss and on an identity change exactly like the other drafts; without it a previous account's typed proposal text would stay in localStorage. `test-fixtures.ts` (+3) because views can only import the exported `@multica/core/collaboration/test-fixtures` path. The stage fixtures are self-contained to avoid an import cycle; a core test fails if they drift from `collaborationIssueFixture`.
- **D2 — test order.** `stage-schema.ts` was written before its test. The five behavioural suites (intent, api, actions, queries, presentation) were written first and their red run is kept. The views suite was written after the panel.
- **D3 — views test transport.** `stage-panel.test.tsx` uses the real `ApiClient` with an endpoint-routed `fetch` stub, the convention of this directory, instead of mocking `@multica/core/api`.
- **D4 — access loss erases pending stage intents.** This matches the existing drafts, but it means the contract's "recovery needs only the account" cannot be used from the stored record after the issue itself was denied. Project-policy intents are not tied to an issue and survive.
- **D5 — consistency rules beyond the literal contract text.** A view is unavailable when: `roles` is null with a non-zero `role_revision` (or the two revisions differ); `scope` is null with a non-zero `scope_revision` or a `scope_state` other than `none`; a managed view lists `scope` as missing while `scope_state` is `current` (or the reverse). A1 retires `stage_roles_configured` only for a revision above the expected one and `stage_roles_unchanged` only at the expected one; P2 only for a revision above the expected one. I read the server lane's `collaboration_stage_view.go` / `stage.go` (not run) and these hold there today.
- **D6 — unknown refusal code.** A well-formed stored-refusal envelope with a code this client does not know is settled as refused with generic text; a known code with a contradictory status stays unknown.
- **D7 — missing write route.** `404/405` without a JSON code on the first dispatch is "nothing was saved · this server does not support the action yet".
- **D8 — project policy UI.** It is a disclosure inside the stage panel. P1 does not say whether the viewer is owner/admin, so the form is shown to any member who can read the policy and `stage_policy_management_denied` is rendered when refused.
- **D9 — proposal source picker.** Comments come from the loaded comment pages, runs from `issue.execution.runs`; there is no entry point from the comment list ("file as proposal") yet.
- **D10 — proposal text is rendered as plain text**, not through `RichContent`.
- Possible mismatch to confirm with the server lane: the client treats a refusal that carries `collaboration_revision` without `status` as malformed (unknown, recoverable through O1). Contract 3.3 always includes `status`; the handler code I read does too.

Flake seen once: `field-editing.test.tsx` and `resource-offers.test.tsx` each hit the 5 s timeout in `views-collaboration-run1` (load average 7–18 while the server lane ran). Neither mounts the stage panel; both passed alone and in the final run.

## 7. What client increment 2 needs

1. Exact R4 shape (code-candidate evidence, `ci_digest`, `max_expires_at`, applicability) and T4 shape; A6–A9 are frozen but R4 says "exact shape is added before the route is served".
2. The capability hint names for A6–A9 in R1 `capabilities` (today only four keys exist).
3. The M1 collaboration typed completion consumer contract (submission / self / admin variants, preview and candidate selection endpoints).
4. Activity allowlist projection and notification kinds for `stage_*` events; until then the activity API reports them as `other`.
5. The final A5 child-entry shape and the descendant admission rule, once section 13 records them.
6. A decision on D4 and D8.
7. E2E ownership: a stage canonical spec against production Web and the matching Desktop renderer.

Seams left: `CollaborationStageConfirmationSchema` already parses the four increment-2 variants; `CollaborationStageRequestSchemas`, `CollaborationStageFrozenRequest`, `CollaborationStageActionInput`, the route switch in `sendCollaborationStageRequest` and `matchesCollaborationStageConfirmation` are the places a new action is added. No placeholder UI exists for them.

## 8. Not verified

- No request reached a real server; the server's actual JSON was read in source only.
- No browser, no Electron, no screenshot: layout, wrapping at narrow widths, enlarged text, focus rings and dialog scroll are unverified visually. Accessibility was checked only through role/name queries in jsdom.
- Realtime invalidation and the 15 s view refresh were not exercised end to end; only the key prefix and the options were tested.
- The zh-Hans copy follows the conventions glossary but was not reviewed by a second reader.
- Desktop and Web wiring is covered by `pnpm typecheck` and the shared detail mount only.
- Nothing here states that a stage policy can be enabled, that S2 is accepted, or that any of the 24 parent cases passes.
