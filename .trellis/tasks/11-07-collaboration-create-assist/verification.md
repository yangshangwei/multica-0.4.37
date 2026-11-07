# Expanded collaboration composer and AI description assistance

## Delivered

- Expand/collapse immediately before Close, with labels and pressed state; one mounted form preserves draft, selected files and ongoing AI while toggling. Reopen resets default size. Explicit user sizing overrides automatic space requested by AI.
- “帮我完善” below the plain description textarea reuses the existing description assistant and source-scoped API in manual mode. It streams provisional text, auto-adopts a validated result only for an unchanged/unblocked current draft, and supports exact raw-text Undo and clarification answers.
- A stable four-method textarea adapter and opt-in shared plain-text mode preserve whitespace and render stale suggestions inertly. Default Markdown callers retain their real ContentEditor behavior; no rich-text migration or duplicate AI state machine.
- Source/viewer/API/draft authority guards discard late output; source denial and retired-draft tests prevent resurrection. API errors404/503 preserve drafts/files. AI-only Unicode input limits are visible without truncation or preventing ordinary creation.
- Other measured UI issue addressed: collaboration-only helper padding was reduced to keep the initial-fields summary visible in the default window. Long generated content/clarifications scroll in the body while header/footer remain reachable.

## Changed files

| Area | Files |
| --- | --- |
| Collaboration shell and integration | `creation-form.tsx`, new `creation-description-assist.tsx` |
| Shared assistant reuse | `modals/issue-description-assist.tsx` |
| Regression tests | new `creation-description-assist.test.tsx`, shared `issue-description-assist.test.tsx`, one scoped Close selector type repair in `creation-form.test.tsx` |
| Copy | English and Simplified Chinese collaboration locales |
| Contracts | Collaboration creation and issue-description-assist specs |

No Core/backend/API/storage schema changes, dependencies or stores were added. Existing source/credential and file policies remain authoritative. The pre-existing Close test selector removal was reviewed and adopted: ByRoleOptions has no `exact`; string role names already match exactly, so this is a type repair without weakening the assertion.

## Browser evidence

See [final report](research/browser-verification-final.md), [default dialog](research/browser-round-2/targeted/dialog-default-722-assist-light.png), and [expanded AI result](research/browser-round-2/targeted/dialog-expanded-722-ai-result-light.png).

The first production batch passed20 affected feature scenarios;3 same-build geometry checks measured the single padding defect. The final6 affected confirmation scenarios passed after that local spacing correction. All receipts have zero page errors and closed owned browsers/listeners. The final source manifest matches product/CSS/locale bytes. These are incremental receipts, not a claim of repeating the whole matrix after the final CSS delta.

Verified across both themes and desktop/390×740/320×568: resizing preserves component identity, file selections and live stream; expanded viewport gutters are12px and coarse controls44px; selected source overrides a different ambient workspace; partials never reach create payload; completed text/Undo/clarifications, late source output, inert preview, unavailable/failure/retry/cancel, input limits, long output focus and retained upload/native-Enter/IME behavior work.

At722×660 default, the initial-fields summary is fully visible at scrollTop0 after the padding correction. There remains25px of trailing scrollable extent; this is initial summary visibility, not a zero-scroll-range claim. Visual verdict is94/pass based on the inspected default and expanded crops.

## Verification limits

The browser fixture uses actual shared components, CSS, Core API and SSE parser with mock transport and memory drafts. No real AI provider, paid quota, production backend/task submission, Electron Main, OS file chooser, physical touch/IME or spoken screen reader was exercised. Deterministic fixtures prove integration and error behavior, not live-model content quality.

The scoped Impeccable detector returned `[]`, and no new in-scope finding remains from visual inspection. Final combined test/type/lint results are recorded below before archive.

## Final combined checks

- Final serial scoped suite: **38 files /458 tests passed**, including all collaboration tests, ordinary assistant, real ContentEditor, create panels/dialog and locale parity.
- Views typecheck passed; full Views lint passed with0errors and27pre-existing warnings.
- Scoped Impeccable detector `[]`; diff-check passed.
- New shared/integration tests were observed failing before implementation; final guard tests cover actual source denial and draft retirement, not just mocked callback booleans.
- Exact final product/CSS/locale and harness hashes matched the browser provenance before commit.

All acceptance criteria are met within the documented component/integration scope. No known in-scope failure remains; verification-owned browsers and listeners are closed.
