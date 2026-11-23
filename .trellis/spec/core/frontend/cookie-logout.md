# Confirmed cookie logout

## Scope and signature

AuthState.logout(beforeLogout?: () => void): void | Promise<void> owns local session teardown. Cookie mode returns the pending POST /auth/logout promise; bearer/token mode tears down synchronously. onLogout is a platform callback after successful teardown, not a notification that a request merely started.

## Contract

Cookie-mode logout must await the server response that clears HttpOnly cookies before reporting unauthenticated state or navigating to /login. Reaching /login early lets a subsequent full navigation cancel that fetch and restore the still-valid cookie session. A failed request leaves auth and client-owned drafts intact and offers explicit retry.

The optional beforeLogout callback clears protected client state only after confirmed cookie clearing and before publishing unauthenticated state. useLogout passes clearClientSessionData there and awaits before navigation. Direct restricted PasswordForm logout must catch rejection and render existing localized feedback. Token/Desktop callers keep immediate local cleanup.

Deduplicate pending cookie logout. Capture ApiClient.getSessionScope after api.logout starts, because initiation synchronously advances its credential epoch. A later session or user replacement must prevent the old completion from clearing new state or navigating it away. Do not weaken existing session-expiry semantics or introduce blanket transport retries.

| Condition | Required result |
| --- | --- |
| Cookie-clear response pending | No client cleanup, unauthenticated publication or login navigation |
| Response succeeds in original session | Cleanup once, platform teardown, unauthenticated state, then navigation |
| Response fails | Preserve session/drafts; localized feedback and explicit retry |
| Duplicate click | One cookie-clear request and one teardown |
| Session/user replaced while awaiting | Old completion cannot clear replacement or navigate it |
| Bearer-mode logout | Existing synchronous local teardown |

## Canonical tests

Core auth/store.test.ts owns delayed cookie completion, failure/retry, replacement identity and deduplication. Views auth/use-logout.test.ts owns cleanup/navigation wiring; password-form.test.tsx owns direct-call rejection feedback. auth-qa.spec.ts and main-supplemental-qa.spec.ts cover real login, reload, logout and subsequent protected navigation; retain server and cookie assertions.

Wrong: fire-and-forget api.logout and immediately publish unauthenticated. Correct: return and await confirmed cookie clearing, then execute the ordered teardown. Never add a sleep to the browser test to conceal early logout success.
