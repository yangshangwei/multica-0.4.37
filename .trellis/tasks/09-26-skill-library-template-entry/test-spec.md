# Test specification

Status: iteration 2, future verification plan, not executed. Canonical command/prerequisite research: [verification-map.md](research/verification-map.md). Execution commands: [implement.md](implement.md). Review-adopted lifetime/navigation corrections are recorded in design.

## Acceptance-to-evidence matrix

| Requirements | Canonical suite | Added or retained assertions |
| --- | --- | --- |
| R1, R2, R3 | `packages/views/skills/components/skills-page.test.tsx` | Compact entry opens direct browser; no inline template collection; workspace total/search/facets unaffected by template count/search; New skill still opens methods; mixed source counts accurately named. |
| R7, R8 | new `packages/views/skills/components/skills-page-template-session.test.tsx` | Real QueryClient + SkillsPage + CreateSkillDialog: cached same-workspace skill-list failure/retry preserves edited draft, unconfirmed submission, and pending discard/navigation snapshot despite error/normal body changes. Owns ancestor lifetime regression only. |
| R2, R5, R6 | new `packages/views/skills/lib/skill-template-discovery.test.ts` | Pure source/group totals and relatedness matrix; preserves original objects/order, includes all matches, ignores malformed/unnamed input safely; verifies summary wiring without duplicating the presentation resolver matrix. Start with `// @vitest-environment node`. |
| R6, R10 | `packages/views/skills/lib/skill-presentation.test.ts` | Existing canonical provenance, customized/default/historical description, bilingual search, current-locale-only provider, and embedded-source sync tests; add summary coverage without rewriting Chinese source defaults. |
| R6, R10 | `packages/views/locales/parity.test.ts` | All four locale keys, plural shapes, and summary registration remain in parity. Does not alone prove idiomatic copy or rendering. |
| R3, R4, R8, R9 | `packages/views/skills/components/template-skill-create-panel.test.tsx` | Direct/no-name source defaults, visible preview consistency, phase labels, short row/full preview copy, source/search/query-state table, related-count/link rendering and keyboard wiring. |
| R3, R10 | `packages/views/skills/components/create-skill-dialog.test.tsx` | Method chooser order/import/manual behavior and initial category retained; explicit template entry reaches picker; update old initial-prop callers. |
| R4, R7, R8 | `packages/views/skills/components/create-skill-template-flow.test.tsx` | Canonical root draft, discard, creation, recovery, request-workspace affinity and late-response matrix; add direct-entry and guarded original-request snapshots for every Desktop adapter intent, including existing-tab background activation. |
| R7, R9 | `packages/views/navigation/app-link.test.tsx` | Owns full shared native/adapter gesture-classification matrix. Dialog suite proves snapshot/guard/adapter integration; do not duplicate every modifier permutation in each component test. |
| R1, R3, R4, R6, R9 | `e2e/skill-template-creation.spec.ts` | Real catalog→preview→cancel→adopt→edit→create, exactly one write; unchanged source/existing copy/members/agents; new entry and retained method path; wide/narrow captures and keyboard focus. |
| R5, R6, R7 | `e2e/localized-template-defaults.spec.ts` (`specialist skills localize`) | Replace inline catalog expansion with new entry and related-instance link; preserve English/Chinese purpose search, official identity and zero-write assertions. |
| R1, R9, R10 | `e2e/skill-category-taxonomy.spec.ts` + visual checklist below | Shared page-shell regression: category/filter counts, card/list selection, hover and light/dark layout remain intact after removing the old strip. |

## Pure relation/source matrix

Use fixture records with explicit IDs and origin/source metadata; no DOM or API mocks. Every rule belongs here once; components need only zero/one/many wiring examples.

| Input | Expected result |
| --- | --- |
| Known template name + same-name skill with verified built-in origin | Included, official presented label; no claim that it is a user-created copy. |
| Same-name manual skill without origin/source; mismatched origin name/type | Excluded. Name coincidence alone proves nothing. |
| `template_source.name` exactly matches, including a renamed skill or other version | Included; version is informational lineage, not an equality/security test. |
| Missing/null/string/array/malformed source; nonstring/different source name | Excluded unless the independent verified built-in condition succeeds. |
| One official instance and two matching copies; one matches both rules | Three distinct input records returned, each once, in input order; no preferred/first-only destination. |
| Zero platform, two deployment / two platform, zero deployment / mixed / zero templates | Source totals sum to valid total; deployment-only remains browsable. |

The recognized current/historical default versus customized/unknown description matrix belongs only in `skill-presentation.test.ts`: summary may shorten recognized defaults; supplied/custom copy and full description/search remain unchanged. The discovery helper suite checks that it carries this summary separately and preserves original template objects, without repeating every locale/default permutation.

## Query and interaction cases

Use real tabs/dialog/link primitives, a controllable QueryClient, and deferred query promises. Avoid replacing interactive primitives with div mocks. Keep fixtures scoped to `workspaceId`.

1. Catalog cold pending → loaded, pending → no-data error → retry success; no false zero or premature adopt.
2. Cached nonempty or cached empty data → background refresh → failure; rows/counts remain available, error is distinct and retry preserves search/draft. Do not use `isError` alone to hide cached data.
3. Successful empty catalog differs from no search results. Search matching deployment only exposes its count; switching source shows matching preview. No off-tab or filtered-out selected template can be adopted.
4. Deployment-only initial browse chooses deployment after load. Explicit valid named seed chooses its source; absent seed falls back to browsing. Narrow view initially shows the list.
5. Workspace-skill cold/error states never claim no related skill; successful `[]` does. Cached-data error retains named links with an explicit stale hint. Retrying this query does not re-adopt or reseed a draft.
6. Single-locale-only rendering smoke for entry/phase headings in `en`, `zh-Hans`, `ja`, `ko`; customized description remains unchanged after locale switch. Keep full persistence/recovery matrices in their existing suite, not four copies.

## Stable page-session integration regression

Add `skills/components/skills-page-template-session.test.tsx`. Mount the real page, QueryClient, navigation provider and creation dialog; mock API responses at `@multica/core/api`, not skill-query hooks, dialog, session hook, or interactive primitives. Seed successful skill-list data, open the direct template entry, then invalidate/refetch the same workspace query into failure and retry success. This specifically exercises the existing `listError`/normal branch transition that panel tests miss.

- Edited draft case: adoption and field edits survive both transitions; later close still presents the dirty guard. No reseeding, implicit write, or creation callback occurs.
- Unconfirmed case: create has an unknown result before refetch failure; its warning/check-result/close protection survive error and retry, with no additional create request. The existing root suite retains the full recovery matrix.
- Pending discard case: request a guarded navigation, then fail/refetch successfully while the alert is open. The same alert and original request remain pending. Cancel restores the draft/link focus; a confirmation variant executes the original path/title/intent exactly once after reset/close.

Use separate focused cases for these three lifetimes. A real workspace UUID/slug change is intentionally different: existing session reset tests must continue to clear draft/pending action and reject late responses. Do not confuse a query-state change with a workspace change or duplicate the full root matrix in the page suite.

## Root navigation and retained safeguards

Extend the existing flow suite, with `onCreated`, navigation, create requests, and toasts observed separately.

- Pristine related link: Web in-place push and each resolved Desktop intent reset/close then call the intended adapter method exactly once, with no completion callback/toast/write.
- Dirty draft returned to picker: every Desktop adapter intent and Web push opens discard. Cancel retains edits/link focus with no adapter call; Confirm consumes the saved request, resets/closes, then performs original navigation once.
- Unknown-result protection survives Continue editing or Back and guards all Desktop adapter intents even when the visible draft is pristine. Busy submitting/checking suppresses every Desktop related-link adapter call and Web in-place push. Keep recovery `openCandidate` tests independent.
- Snapshot regression: create a request carrying source UUID/slug, destination ID/path, presented title, and background/foreground intent; change locale/preview and confirm with different modifier keys. Execution uses the original title/path/intent. A pending request is consumed once, including rapid repeated confirmation.
- Existing-tab regression: the fake Desktop adapter models an already-open background destination by activating it and unmounting the source host. Dirty/unknown Cancel and busy state prevent that activation; Confirm closes/resets first and invokes the original background call once. Do not special-case known-open tabs in shared code or change the adapter contract.
- Web-native Cmd/Ctrl(+Shift), middle, and Shift-alone handling remains with AppLink/browser and does not close/reset the current draft. The full gesture-to-intent matrix stays in `app-link.test.tsx`; root tests cover representative wiring and resolved-intent guards rather than every modifier permutation.
- Workspace A pending guard/request → switch UUID or slug to B → no stale related link, pending action, or navigation under B. Confirm cannot rebase an old path under the new workspace. Existing create/check requests still carry A, including ambient slug clearing; retain the canonical hook/API behavior.
- Preview another template never alters draft; same-template adoption resumes it; different-template adoption is guarded; chooser round-trip, close/Escape/backdrop, conflict, timeout, invalid identity, late success, and recovery of older results retain existing assertions.
- Adoption request uses full description rather than summary; final content retains YAML types, edited body, and supporting files. Existing core builder/API suites are canonical for their full matrices; do not copy those matrices into DOM tests.

## Production browser and visual evidence

Use the production Web mode and a task-owned matching API/database specified in the research map. Existing `TestApiClient` performs setup/teardown. The real template creation E2E expects exactly 15 embedded templates and no mounted catalog; exercise mixed/deployment-only/failure cases with controlled component fixtures rather than modifying a legitimate deployment. Do not intercept real catalog/create calls in the creation E2E.

Capture these states at 1280×720 and 375×667; add the new entry at 360×800: workspace collection, template list/preview, many related links, empty/no-match, draft editor, and discard warning. Cover light/dark and English/Chinese in screenshots; inspect Japanese/Korean long-label rendering via focused locale smoke/visual checks. Record source count values alongside captures.

Checklist: compact entry meets AC1/AC9 sizes; existing management-toolbar behavior is unchanged, including hidden search below `md`; full instructions reachable; long names/descriptions wrap; no horizontal overflow; footer actions visible; tab/row focus and selected-hover state distinguishable; logical Tab order; keyboard source changes; Enter on related link; narrow preview Back restores focus; modal close restores trigger; discard cancellation restores link. Measure text contrast against the existing semantic tokens in rendered light/dark states rather than inferring it from class names.

Run `visual-verdict` for each visual iteration before editing again; persist verdict JSON at `.omx/state/skill-library-template-entry/ralph-progress.json` as an evidence artifact, without implying an active runtime mode. Attach production screenshot paths, commands/results, commit/source fingerprint and `api.running.json` / `web.running.json` (plus `verification.running.json` for `check.sh`). Desktop adapter tests and consumer typechecking are required; perform a live Desktop navigation smoke when its task-owned environment is available, and explicitly report any unavailable manual gate.

Both verification routes cover creation, localized defaults/navigation, and the category-shell regression. The isolated alternative must run `bash scripts/check.sh e2e/skill-template-creation.spec.ts e2e/localized-template-defaults.spec.ts e2e/skill-category-taxonomy.spec.ts --project=chromium`; this includes the entire localized-defaults spec. A creation-only isolated run does not satisfy the planned gate.

## Scope of execution and reporting

Run the exact focused unit/lint/typecheck/browser commands in `implement.md`; no test execution is needed for planning-only Markdown. Existing core suites (`skills/template-draft.test.ts`, `api/skill-template-schemas.test.ts`, `api/skill-template-client.test.ts`, `workspace/skill-template-queries.test.ts`) need modification only if scope expands across their contracts, which requires review first. No new dependencies, backend migrations, real-agent invocation, Electron packaging, or mobile test run is implied.

A passing run requires reading the actual command results and inspecting screenshots/focus behavior. Record skipped checks as not run with a reason. Final implementation report includes changed files, deletion of the duplicate skill catalog, reused safeguards, test/visual evidence, and remaining risks. Current status: application tests/builds/browser checks not run — planning only.
