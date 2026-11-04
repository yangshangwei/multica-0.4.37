# Final Trellis check

- Date: 2026-11-03.
- Role: native trellis-check; full R1–R8 source review and affected-package mechanical verification.
- Reviewed base: HEAD `4ba3a868a02e2e1e00499447c67b0f61cf3307d7` plus the task's pinned pre-implementation context. The parallel source-picker changes are now inherited committed input.
- Current verdict: **Mechanical gates pass; confirmed native-search focus and feedback-visibility findings require the accepted final UI repair/verification before full task closure.**

## Findings (fixed)

None by this reviewer. No product, spec, preview, lifecycle or git state was modified. The implementer's local type/test fixes were retained, and all checks below ran against current source.

## Findings (not fixed)

### P1: native recipient search has no painted focus indicator

- File: `packages/views/collaboration/form-styles.ts:5`.
- Main-session measurement: native recipient search is `focus-visible` but has computed outline `none 0px` and box-shadow `none`; other light control rings measured 3.856:1.
- Cause consistent with current CSS: `outline-2` sets width while the native search still has `outline-none` / `--tw-outline-style:none`. An explicit scoped solid outline style is required alongside width and semantic color.
- Not fixed here: main visual verdict is 88/revise and the implementer has exclusive write ownership of `form-styles.ts`, `creation-form.tsx` and `field-inputs.tsx` for one accepted final correction batch. This reviewer will validate after handoff.

### P2: confirmed rejection/recovery feedback can stay below the fixed action area

- File: `packages/views/collaboration/creation-form.tsx:235`.
- Evidence: confirmed rejection paragraphs render at the end of the scrolling `CreationContents` body, while Submit is now outside that body. The saved current `research/browser-current/quota-en.png` and `quota-zh-Hans.png` show the re-enabled Submit/Cancel controls but no preservation, quota or administrator explanation in the visible body. The corresponding browser result records the messages in DOM, so text-presence assertions passed without establishing visible feedback.
- Impact: clicking the initially visible fixed Submit can still leave a visual user without the specific refusal explanation. Original unknown-result/recovery controls share the same below-fold feedback region and need a discoverability check.
- Recommended narrow repair: reveal the current creation-feedback region in the constrained dialog after relevant pending/error/recovery transitions, such as a local ref plus nearest body scroll, preserving keyboard focus and Core intent/retirement semantics. A persistent visible feedback region near actions is another option, but is a larger layout change.
- Not fixed here: this is a UX/layout integration judgment, and the main session explicitly reserved such edits until its current visual verdict. Main was notified with the exact screenshot and source evidence.

## Full-scope source review

| Requirement | Assessment |
|---|---|
| R1: source lock | Current API/viewer creation prefix plus exact create/recover key shape excludes unrelated mutations. Pending mutation covers the success-retirement interval; contentful upload/creation records keep uncertain source ownership; idle redacted recovery stays independent. Disabled select and change guard agree. Active/current-scope and Core source/actor/request fences remain. |
| R2: recipient ARIA | Stable per-picker/normalized-account IDs; actual option/selected/disabled attributes; multi-select flag; native localized textbox controls and owns an actual active descendant. Selection is independent of navigation. Removed/empty/closed states clear active ID. Existing committed-DOM sync, IME/Safari-229, Escape, footer focus and append-only paging remain. Generic callers do not receive blanket roles. |
| R3: focus | Local semantic `faint-foreground` overrides focused ring/border opacity; portal content supplies native-search/option outlines. Global UI primitives/tokens remain unchanged. Actual colors, clipping and focus-visible behavior are main-session browser gates. |
| R4: refusal | All 26 Core schema codes have explicit localized reason/action mappings plus factual fallback. The view consumes only typed confirmed rejection; quota calls out the limit and administrator action. Generic preservation copy no longer recommends unconditional retry. Draft/upload preservation tests pass. Visible presentation still needs the finding above resolved. |
| R5: identity | Full existing `user_id` is visible in candidate/exact/selected displays, enters option labels and selected trigger descriptions, and retains external wording. No username/email response expansion or inferred membership. Existing selected-name sentence stays independent. |
| R6: coarse targets | Scoped close, option, stage and portaled search minimum target styles are wired; shared global row/button sizes are unchanged. Actual settled DOMRect/touch proof remains main-owned. |
| R7: persistent actions | Normal-flow header/source/action regions surround the min-h-0 scrolling body; original native form association/required/submit is preserved. Optional Cancel works without a Dialog-specific primitive in the shared panel, including loading/error/no-source fallback branches. Source selector DOM survives keyed-form changes. Main must finish geometry/focus and feedback-visibility proof. |
| R8: debounce | Immediate raw input and 250ms trimmed query are separate; raw/query mismatch is pending before transport and excludes old parent rows. Existing source key removes old timers on switch. Query keys, AbortSignal, freshness/authority refresh, placeholder/error/retry and denial-versus-transient fallback remain intact. Original delayed-query assertions remain, with actual-start timing adapted. |

Package direction and scope remain `views -> core + ui`; no Core/API/dependency/global-token mutation or app-specific routing import was introduced. The small `form-styles.ts` constants are reused by creation and the two portals, without a new behavior layer. Existing parent popup/searchMaxLength/selected identity/retry/ref/denial contracts were preserved.

## Verification run by this reviewer

| Command | Result | Evidence |
|---|---|---|
| `pnpm --filter @multica/views exec vitest run collaboration/creation-form.test.tsx collaboration/account-picker.test.tsx collaboration/field-editing.test.tsx collaboration/field-inputs.test.tsx collaboration/attachment-picker.test.tsx issues/components/pickers/property-picker.test.tsx locales/parity.test.ts` | **7 files / 114 tests passed** | `check-targeted-tests.log`; 14.53s |
| `pnpm --filter @multica/views typecheck` | **Pass** | `check-views-typecheck.log`, process exit 0 |
| `pnpm --filter @multica/ui typecheck` | **Pass** | `check-ui-typecheck.log`, process exit 0 |
| `pnpm --filter @multica/views lint` | **Pass, 0 errors / 27 existing outside warnings** | `check-views-lint.log`, process exit 0 |
| Owned-file ESLint | Clean owned-file log; full-package successful lint covers these files | `check-owned-lint.log`; no owned warning/error output |
| `pnpm check:ui-exports` | **Pass**, 62 wildcard-exported source files clean | `check-ui-exports.log`, process exit 0 |
| Scoped `git diff --check` | **Pass** | No whitespace findings |
| Static Core-code / localized-mapper comparison | **26 / 26 covered**, no missing code; both locale action dictionaries inspected | `creation-schema.ts`, `labels.ts`, en/zh-Hans collaboration JSON |

Reused current implementation evidence rather than repeating unchanged matrices:

- Caller compatibility: 6 files / 56 tests pass in `implementation-caller-compatibility.log`; account-picker overlaps the targeted run, so counts are not additive.
- Core creation/recovery/API/scope: 5 files / 43 tests passed in the main session; no Core source changed.

Web/Desktop consuming-app typechecks passed in the main session. Current Chromium contrast/geometry/AX/touch, the final visual verdict, and accepted-source hashes remain main-session gates. No real API, existing service, user browser, Electron Main, physical touch or real screen-reader operation was performed by this reviewer.

## Before final closure

Resolve and verify the feedback-visibility finding, then confirm the final accepted browser build reflects any changed source. This report's passing mechanical commands do not establish that the current screenshots passed all R1–R8 visual acceptance.
