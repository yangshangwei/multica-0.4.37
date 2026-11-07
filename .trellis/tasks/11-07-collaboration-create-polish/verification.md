# Collaboration creation visual refinement

## Result

The user screenshot is the acceptance reference. This pass keeps approved option C and reduces its form-like visual weight:

- Default dialog height is 34rem (544px) instead of 640px; the empty description minimum is 64px instead of 96px.
- Title label and required explanation share a wrapping caption row. Title uses the role-named 20px token and a painted opaque 2px bottom focus border instead of a box ring.
- Compact account identity spans the trigger's usable width; the full UUID fits one line in the screenshot-sized reference. The independent selected-name sentence remains.
- Empty attachments retain their accessible group name and visible sharing explanation without a bold empty heading. File/loading/error/cancel/recovery states keep normal details.
- Desktop footer combines attachment entry and draft status with the action group. English 320px Cancel/Create share a row with a 6px gap.
- The native source selector has a bounded 256px desktop width, enough for the supplied source name without distancing the breadcrumb.

Changed product files: `creation-form.tsx`, `account-picker.tsx`, `attachment-picker.tsx` and `creation-form.test.tsx` in `packages/views/collaboration/`. No API/store/dependency/locale changes. Existing keyboard and recovery contracts remain the source of truth. A narrow density/focus convention was added to `.trellis/spec/views/frontend/collaboration-creation.md`.

## Visual evidence

See [final browser verification](research/browser-verification-final.md).

- [Final light dialog crop](research/browser-final-header/targeted/dialog-default-722-light.png)
- [Final dark dialog crop](research/browser-final-header/targeted/dialog-default-722-dark.png)
- [English narrow footer](research/browser-round-2/english-320-empty-light.png)
- [User-provided reference](research/user-reference.png)

The complete corrected browser matrix passed 40/40. The final single-class source-width refinement was then checked in eight affected desktop, screenshot-size and reflow scenarios; all passed. Exact source hashes are recorded in `research/final-source-check.json`. No third complete matrix is claimed. The final visual verdict is 94/pass, supported by direct screenshot comparison and concrete geometry:

- 722×660 empty Chinese draft: body client/scroll height 431/431, scrollTop 0, summary wholly visible, no initial body or document overflow.
- Full UUID appears on one line; description height 64px; 39px margin between summary and footer.
- Title focus border has effective opacity 1 and measured contrast 3.73:1 light / 4.76:1 dark.
- Both themes and locales at 390×740 and 320×568; 32-file long-name states; original creation recovery, native Enter/CDP composition and upload focus paths remain covered.

The initial browser run retained mock-actor consistency and refocus-sequence errors. They were repaired in the fixture, not in product behavior. Initial visual findings were corrected together: excess lower-body space, English action-row rounding overflow and source-name clipping.

## Verification scope

The fixture compiles real shared components/CSS/locales, uses mock fetch and page-local memory storage, blocks external traffic and stops its owned browsers/listeners. The 200% equivalent reflow check uses 640×450 CSS pixels at DPR2 for a 1280×900 device-pixel canvas; this is not native browser-toolbar or OS zoom. Real backend/Electron Main, physical OS IME/touch and spoken screen-reader validation remain outside this task.

The scoped Impeccable detector returned `[]`; Views typecheck and lint passed, with 27 pre-existing lint warnings. Focused regression tests passed 116/116. Final broad regression evidence and any test-harness stabilization are recorded below before task closure.

## Approved initial-focus integration

A final broad run exposed a timing-sensitive focus regression: Base UI resolves a target then focuses it on a deferred animation frame, which may override a source selector the user has since focused. Another session supplied a guarded callback and deterministic frame-order test; the user explicitly approved incorporating and verifying them. Independent review confirmed the mechanism. The callback returns false to prevent Base UI's unconditional queued focus and uses existing guarded lost-focus recovery while the popup remains open. The test controls frame ordering rather than adding delays or increasing timeouts.

This supersedes the initial visual-only boundary for that focused behavior. The accepted delta is retained at `research/accepted-focus-delta.json`. Focused current-code tests passed37/37, Views typecheck and scoped ESLint passed; final browser and full-suite proof are appended after completion.

## Final accepted-source checks

- `pnpm --filter @multica/views exec vitest run collaboration locales/parity.test.ts --maxWorkers=1`: **33 files /349 tests passed**,71.09s, after the approved focus correction. No raised timeouts or weakened assertions.
- Views typecheck passed on the accepted product/test hashes. Full Views lint previously passed with0errors/27existingwarnings; final scoped ESLint passed0errors/0warnings after the focus delta.
- Scoped Impeccable detector returned `[]`; diff whitespace check passed.
- Browser evidence:40 complete visual/behavior scenarios,8 affected header refinements,7 approved focus-integration scenarios, all passed with recorded exact source fingerprints and cleanup. These are incremental verified deltas, not repeated claims of a final full matrix.
- Independent Trellis check reports no remaining in-scope finding.

The first parallel unit batch hit resource timeouts; affected files passed serially. The following serial full run exposed the real deferred-focus ordering issue; it was not dismissed as resource noise. After user approval, the guarded callback and deterministic regression were reviewed and included, and the final full suite is green.
