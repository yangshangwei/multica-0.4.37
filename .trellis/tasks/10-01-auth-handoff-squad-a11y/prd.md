# Fix authentication handoff and squad heading semantics

## Requirements

- Device authentication initialization must not publish a settled signed-out state before the fallback resolves, including delayed responses and StrictMode.
- A rejected or unavailable device login must still finish signed out; mid-session 401 must still expire the session.
- Squad template headings must expose the title exactly once while keeping the avatar visually present. Standalone avatars retain their accessible names.
- Keep prior test-maintenance edits and concurrent password-registration changes intact.

## Acceptance

- [x] Regression tests fail before the changes and pass after.
- [x] Real CoreProvider/ApiClient/AuthInitializer/auth store wiring is exercised for the 401-to-device handoff.
- [x] Ordinary login and terminal expiry regressions remain passing.
- [x] Template title exact accessible name and decorative avatar behavior pass.
- [x] Lint/typecheck and relevant suites pass; unavailable real-server E2E is explicitly reported.
