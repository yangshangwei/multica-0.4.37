# Final code check — 2026-11-20

Reviewer: independent `trellis-check` agent. Scope: the complete blocked-feedback
task across Core, shared Views, locale values, consumer selectors, and the two
updated frontend specifications. The final follow-up also independently reviewed
the new browser specification and the browser owner's recorded acceptance. No
code or browser-review blocker remains; task archival stays with the main session.

## Findings fixed

1. **A just-cleared description bypassed the focus explanation.**
   `quick-create-issue.tsx` combined the debounced `hasContent` with live Markdown
   using OR. Clearing the editor immediately before submission left the old true
   value in place. The composer then rejected empty content without routing focus
   to the editor. Submission now derives content presence from the live Markdown
   alone. The named clearing regression failed with zero focus calls before the
   correction, then passed. Companion cases retain immediate newly typed text and
   attachment-only Markdown submission. The existing composer remains responsible
   for final content normalization, upload checks and synchronous single-flight.

2. **An empty source comment ID introduced a new blocker.**
   `sourceRequired: anchorCommentId !== null` did not match the previous truthy
   condition or the existing composer guard. It is now
   `sourceRequired: Boolean(anchorCommentId)`. The named regression first observed
   `aria-disabled="true"` for an empty ID, then passed with ordinary creation and
   no source-context endpoint call. Source-required cases remain protected.

3. **Immediate stale recommendation adoption cleared silently.**
   The live editor guard correctly rejected adoption before the debounced prop
   updated, but used `clear()` and omitted the new stale status. It now calls
   `clear(true)`, reusing the existing status and retaining refusal of stale
   adoption. The existing live-editor regression was strengthened to assert both
   removal of the suggestion and the status message. It failed before the fix
   because no status existed. Successful adoption and deliberate cancellation
   remain silent.

4. **The persistent recipient hint belonged visually to the description.**
   The browser's first batch measured a 24px trigger-to-hint gap versus 12px
   hint-to-description at English 1280px; narrow states expanded the first gap
   to 52–72px because recommendation controls wrapped between them. After the
   main session recorded visual verdict 86/revise and authorized this bounded
   correction, the existing hint paragraph moved into the recommendation
   component's children group immediately after the picker. This keeps the hint
   with its control while preserving all classes, copy, focus refs, selectors and
   the visible recommendation prerequisite. The browser lane owns final geometry
   and screenshot confirmation; no cosmetic class-assertion test was added.

The fixes touched only these four files during this review:

- `packages/views/modals/quick-create-issue.tsx`
- `packages/views/modals/quick-create-issue.test.tsx`
- `packages/views/modals/creator-recommendations.tsx`
- `packages/views/modals/creator-recommendations.test.tsx`

There are four new tests and one strengthened existing assertion. No dependency,
API, storage format, server change, visual class, or new abstraction was added.

## Findings not fixed

- No remaining concrete code defect was found within the assigned scope.
- Browser focus, modifier-key behavior through real portals, actual geometry,
  contrast and the P1 visual reassessment were executed by the separate browser
  and visual-review lanes. Their retained receipts were checked below; this
  reviewer did not claim jsdom as a substitute or rerun those browsers.
- The test command prints the existing pnpm configuration-location warning and
  React/jsdom diagnostics for act scheduling and cross-document navigation. The
  final related test run has zero failures or skipped tests. Package-manager
  migration and unrelated test-harness cleanup are outside this task.

## Contract review

| Requirement | Source-level conclusion |
| --- | --- |
| BLK-1–3 | A single Core state maps to Tooltip, persistent accessible description, and focus routing. Only uploading/submitting use native disabled. Button, editor shortcut and dialog shortcut share `submit()`. Live clearing/typing now share the composer's content interpretation. |
| BLK-4–5 | Known empty eligible directory shows the explanatory status with agents-list and manual exits. Navigation saves the live prompt. The recommendation action is natively disabled and its start function refuses zero candidates. |
| BLK-6 | The pure helper's 3,072-combination test preserves the original may-submit expression. The empty-source boundary now also preserves the old caller semantics. Existing backend and composer checks remain intact. |
| AID-1–3 | Pending action becomes Cancel, errors use alert plus retry, context-invalidated work has a status, and both helpers reuse one tone constant. Shared description-assist behavioral code is unchanged; its diff is the exported tone and class use. |
| RCP-1–4 | Only the default picker trigger changes appearance; custom Chat/manual triggers retain their branch. The persistent hint now shares the picker's child group so wrapping cannot separate it. Native dialog listener excludes handled/repeated/composing keys, plain Enter and editable targets. Empty helper actions remain natively disabled. |
| VOC-1–4 | Changed locale values remain within the four planned groups. Specified recipient groups contain no 助手/对象/assistant/actors; AI state values contain no 优化/optimiz. Locale parity passes. |
| QA/spec | Pure state matrix stays in Core node tests; named interaction wiring stays in Views. Old selector copy search found no specified obsolete strings. The main session's spec update records all three fixes above. |

Recorded owner decisions (`pen-sheen`, `border-beam`, manual-mode shortcut scope)
are preserved. There are no new stores or framework routing imports. The added
Core package export and pure helper form one required change unit.

## Fresh verification

All commands below completed with exit code 0. Raw commands, timestamps and
outputs are in `check-evidence/`.

| Check | Result | Receipt |
| --- | --- | --- |
| Core typecheck | Pass | `static-checks.json`, `core-typecheck.log` |
| Views typecheck after the grouping fix | Pass | `grouping-checks.json`, `grouping-typecheck.log` |
| Core changed-file ESLint | Pass | `static-checks.json`, `core-lint.log` |
| Views changed-file ESLint | Pass, no ESLint warnings/errors; modified panel checked again after grouping | `verified-final-checks.json`, `views-final-lint.log`, `grouping-lint.log` |
| Core state helper | 8/8 pass; includes 3,072 combinations | `core-regression-results.json` |
| Related Views regression, serial worker | 12 files, 330/330 pass, zero skip | `views-final-regression-results.json` |
| Panel/shell/recommendations after grouping | 3 files, 87/87 pass, zero skip | `grouping-regression-results.json` |
| Changed-source whitespace check | Pass | `diff-check.json` |
| Locale terminology and obsolete-copy scan | Pass | `copy-audit.json` |

The 12-file regression includes quick create, recommendations and Query cache,
the shared description helper and real editor, manual creation, dialog shell,
actor picker, Chat consumer, collaboration helper/form, and locale parity.

Red receipts: `live-content-red.{json,log}` and
`source-stale-red.{json,log}`. These preserve the actual failing assertions rather
than retroactively claiming test-first coverage.

## Reused full-suite evidence and limits

The existing November 20 r2 run has **Core 5,256/5,256** across 373 files and
**Views 7,606/7,606** across 619 files after isolated replay of its original 10
failures. Original failures remain recorded; the initial full Views command did
not pass on its first attempt. Evidence:

- `artifacts/qa/full-e2e-20261120-r2/static/unit-core-command.json`
- `artifacts/qa/full-e2e-20261120-r2/static/unit-views-command.json`
- `artifacts/qa/full-e2e-20261120-r2/static/unit-latest-inventory.json`
- `artifacts/qa/full-e2e-20261120-r2/static/summary.json`

This reviewer checked 2,562 Core/Views/UI/TS-config/ESLint-config source-manifest
entries: **2,558 still match** and the only four differences are the four review
files listed above, covered by the fresh 330-case regression plus 87-case
post-grouping check. See
`r2-source-comparison-final.json`. The final task source hashes are in
`verified-source-sha256.json`.

The full Core/Views suites were not rerun by this reviewer. The evidence is the
retained complete r2 baseline plus fresh checks of this bounded correction;
neither the old whole-suite counts nor prior browser results are described as a
fresh execution of the final four-file delta. The browser lane rebuilt the
production Web candidate with the final source hashes; its binding is reviewed
below.

## Independent browser-spec review

Reviewed final `e2e/quick-create-blocked-feedback.spec.ts` without editing it.
The browser owner applied the review's three local findings:

- Pending upload/submission route gates now release in `finally` and await route
  removal, including assertion-failure paths.
- The additional empty-workspace page now closes in `finally`, including failure
  while creating its fixture or checking narrow geometry.
- The fixture's `requestJSON` call now declares the created-agent response type,
  resolving the strict TypeScript possibly-undefined error.

The five tests affected by cleanup changes (pending plus four visual cases) were
rerun by the owner: **5/5 passed**, zero skip/retry. The explicit response generic
is type-only. This reviewer then ran a targeted strict TypeScript check extending
`packages/tsconfig/base.json` and ESLint using the repository's base configuration:
both passed with no diagnostics other than pnpm's existing configuration warning.
Receipts: `e2e-typecheck-final-command.json`, `e2e-lint-final-command.json` and
their logs. The earlier ESLint attempt outside its base path was ignored and is
retained as such; it is not the passing lint evidence.

Assertion and isolation review:

- Focus, Tab order, tooltip text, persistent accessible description, native
  disabled state, exact draft preservation and explicit recommendation adoption
  are exercised through actual UI controls.
- Successful shortcuts await real HTTP 202 and assert one submitted request;
  the non-input/editor matrix additionally reads exactly one fixture-agent queue
  row. The fixture runtime has no daemon, provider credentials or agent process.
- Loading/failure/upload timing is controlled at the browser transport boundary.
  Recommendation generation uses the task-local deterministic provider; one
  clarification answer response is explicitly stubbed to exercise the real
  textarea and native key handler. These are described as UI wiring checks,
  not real-model execution evidence.
- The inherited lane guard requires explicit loopback API/database URLs and
  refuses default database names. The actual harness binds production Web,
  isolated API, temporary database and deterministic provider to owned loopback
  ports. Synthetic users have unique `example.test` identities and fresh
  workspaces; `deleteFeatureWorkspace` refuses a foreign workspace ID.
- Per-test cleanup deletes only the owned workspace. The final environment
  cleanup receipt records all five allocated ports closed and the owned database
  container removed. No real agent CLI, external model account or user's Desktop
  was used.
- Geometry checks assert actual borders, viewport bounds and picker/hint/label
  distances. Color comparisons use rendered CSS colors and opacity. Soft
  assertions still fail the test; they are not downgraded to warnings. The final
  no-candidate screenshot fixture uses a fresh workspace so stale archive-query
  state cannot masquerade as the zero-candidate condition.

### Recorded browser results checked

All paths below are under `artifacts/qa/quick-create-blocked-20261120/`.

| Evidence | Result and interpretation |
| --- | --- |
| `acceptance-final.json` | 14 mechanics passed; its four original visual failures remain recorded, so the aggregate command exited 1. |
| `visual-final-r3.json` | All four visual cases passed, completing the 18 distinct new cases. |
| `fixture-cleanup-confirmation.json` | Five affected cases passed again after fixture cleanup fixes. |
| `existing-picker.json`, `existing-autonomy.json`, `existing-recommendations.json` | 13 existing Web cases passed. The two native cases were skipped in these Web-only invocations, then explicitly covered below. |
| `native-picker.json`, `native-recommendations.json` | Both real Electron renderer cases passed, no skips. |
| `review-b/measurements.json` | Independent measured-state evidence; reviewed together with the final source/build binding, rather than inferred from unit tests. |
| `cleanup.json` | Owned services stopped, five ports closed, owned container removed. |

Production Web build `yPiCLeEob9_90sBlHK-1y` in
`candidate-environment.json` binds to the exact final panel and recommendation
source hashes below. The `candidate-source-sha256.json` entries match current
bytes; the older `source-sha256.json` and first `candidate-build.json` are retained
historical attempts, not the final binding. Report hashes, result counts and
cleanup were copied into `check-evidence/e2e-final-review-bindings.json`.

## Final product hashes

```text
quick-create-issue.tsx
9bf089caf226d7baef748e95ad9c532874cc8cea4ad792496685556161183e94
creator-recommendations.tsx
319e6b39f691352ba2542b57cff0bd6ad807613c0d4909fcbbd0a14d29162820
e2e/quick-create-blocked-feedback.spec.ts
27ef828493adb8ac1ba695cc7f03fda90c42084823b8abe4df75a0314ba4e3f0
```

No commits, task state changes, browser runs, or unrelated workspace edits were
performed by this reviewer.
