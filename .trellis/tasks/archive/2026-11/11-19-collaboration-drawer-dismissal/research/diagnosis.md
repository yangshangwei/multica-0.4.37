# Confirmed diagnosis and bounded repair

The user authorized direct repair after the investigation. Native CUA clicking the live Electron drawer close button and Escape left the same blank drawer/header/backdrop visible. Current checkout branch is codex/cross-workspace-collaboration with extensive unrelated dirty files; all target source/test files were clean at investigation start.

packages/ui/components/ui/dialog.tsx unconditionally supplies hidden={!visible || props.hidden} to both Popup and Backdrop. With default visible scope this is an own hidden:undefined property. Installed Base UI 1.3.0 merges external elementProps after hidden:!mounted, and undefined overrides true. detail-auxiliary.tsx uses keepMounted: open=false hides the body but the native closed Popup and Backdrop lose hidden.

Observed checks before repair:
- pnpm --filter @multica/views exec vitest run collaboration/collaboration-detail.test.tsx: 15 passed.
- pnpm --filter @multica/views exec vitest run collaboration/detail-auxiliary.test.tsx: 4 failed / 1 passed; closed dialog remains queryable.
- Regression originated in b6b07c3067 visibility suppression additions.

## Change boundary
Fix omission of forced visibility properties in the existing shared Dialog wrapper. Preserve caller props and inject hidden/inert/aria-hidden only when scope is invisible. Do not replace keepMounted, add local drawer state or touch other popup families. Use the existing auxiliary suite for top-right close, hidden full shell/backdrop, focus return, reopen and draft retention. Reuse existing retained-sidebar lifecycle tests for suppression behavior. Main coordinator updates the UI contract and owns native verification.

## Intended files
- packages/ui/components/ui/dialog.tsx: repair Base UI property merging.
- packages/views/collaboration/detail-auxiliary.test.tsx: named close/backdrop regression with actual primitives.
- Existing retained-sidebar lifecycle tests only if needed to protect suppression interaction.
- .trellis/spec/ui/frontend/component-guidelines.md: preserve Base UI lifecycle defaults by omitting undefined forced props.

## Validation
Run new regression red before repair, then auxiliary/detail and retained-sidebar suites. Check UI and Views types and scoped lint. Coordinator verifies live Electron using CUA; child agents must not drive UI or restart the app.
