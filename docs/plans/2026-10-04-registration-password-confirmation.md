# Registration Password Confirmation Implementation Plan

**Goal:** Require matching passwords before creating an account, as requested and approved in this conversation.

**Architecture:** Extend the shared `PasswordForm` registration branch so Web and Desktop use the same confirmation field and validation. Confirmation stays in component state and is never sent to the API or persisted. Existing server password rules continue to apply.

**Tech Stack:** React, existing shared Input/Label/Button components, i18next, Vitest and Testing Library.

## Design

- Add a required, labelled confirmation input directly below the registration password, with `autocomplete="new-password"` and the existing visibility control.
- Compare the original strings exactly, including spaces. Show a localized inline mismatch after blur or a submission attempt; clear it as soon as the strings match.
- Reject mismatched submissions before calling the auth store and focus the confirmation input. Keep the submit control available so an attempted submission explains what needs fixing.
- Clear confirmation and its validation state after success and when switching between registration and login.
- Limit the change to registration. Account setup, password changes, the API contract and database do not need changes.

## Implementation and verification

1. Extend `packages/views/auth/password-form.test.tsx`: matching registration, missing confirmation, mismatch/recovery, exact whitespace comparison, keyboard submission, visibility, state reset and login regression. Run `pnpm --filter @multica/views test auth/password-form.test.tsx` and verify the new expectations fail before implementing.
2. Implement the field and guard in `packages/views/auth/password-form.tsx`; add matching English and Simplified Chinese keys in `packages/views/locales/{en,zh-Hans}/auth.json`.
3. Run the auth tests and locale parity tests, views lint/typecheck, and Web/Desktop typechecks. Inspect `git diff --check` and review the final diff.
4. Check the running registration UI at desktop and narrow viewport widths, including mismatch and corrected input. Save screenshots and a visual verdict under `.omx/`.

## Completion evidence

- Implemented in the shared registration form and both supported locale files; no new dependencies or backend changes.
- Red phase: registration tests failed because the confirmation field did not exist; the existing login test passed.
- `pnpm --filter @multica/views test auth locales/parity.test.ts`: 139 tests passed across 5 files.
- Views, Web and Desktop typechecks passed. Views lint reported zero errors and 26 existing warnings outside the changed files; targeted lint of both changed TypeScript files passed with zero warnings. `git diff --check` passed.
- Independent read-only review approved the change with no findings.
- Web (1280×900 and 390×844) and Electron interaction checks passed for required confirmation, exact matching including spaces, keyboard and mouse submission, shared visibility, and reset on returning to registration. The error text and error border both clear after correction.
- Browser registration requests were intercepted with a 409 response to verify request emission and payload without creating accounts. Successful registration and onboarding are covered by the component suite. Electron used the real renderer and preload with isolated native-service stubs and the development app's `webSecurity` setting.
- Screenshots, the runnable acceptance script and JSON results are in `.omx/state/password-confirmation/`. The visual verdict passes at 95/100 against the supplied registration reference, accounting for the requested additional field.
- No new architecture or shared conventions were introduced, so no Trellis spec update was needed.

## Follow-up: inline password visibility controls

The user requested the familiar eye icon inside the right edge of each password input. Replace the standalone visibility button with the existing InputGroup/Addon/Button primitives and Lucide Eye/EyeOff icons. Keep each input's visibility independent, label the icon button with the existing localized show/hide action and its field description, and preserve keyboard operation, disabled states, refs, autocomplete and validation. Reuse a small local password-input composition across registration, login, setup and password change, without adding a shared-package abstraction or dependency.

Validation: update the existing registration test to verify independent mouse/keyboard toggles without submission, and cover current/new password independence. Run the auth/locale tests, affected TypeScript checks and lint, then verify Web and Electron inputs at desktop and narrow widths. Switching from registration to login must mask the password again.

Completed: 140 auth/locale tests passed; Views, Web and Desktop typechecks passed; targeted lint passed with zero warnings; `git diff --check` passed. Independent review found no issues. Web and Electron acceptance confirmed separate mouse/keyboard toggles, icons fully inside the input groups without overlapping text, preserved confirmation validation, and masked fields after mode changes. The 390px layout has no horizontal overflow. Evidence is in `.omx/state/password-visibility/`; native services and registration responses remain isolated as described above.
