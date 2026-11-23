# Cookie logout completion repair

Final441 regression exposed a real logout race in auth-qa.spec.ts: the UI reached /login before the server's HttpOnly-cookie clearing response. A subsequent full protected navigation cancelled that pending logout fetch, retained auth cookies, and boot restored an authenticated user. Earlier passing runs did not close this new failure.

Sanitized trace evidence: logout POST starts1019579.764 and has status-1/no completed response; login loads with multica_auth/multica_csrf; protected navigation1019706.340 retains both, and subsequent /api/me returns200. Credentials are excluded from the extracted chronology in private logout-race/sanitized-trace-events.json.

## Changes

- core/auth/store.ts cookie-mode logout now returns its server-clearing promise. Only successful completion invokes an optional beforeLogout cleanup callback, clears local auth/workspace state, runs platform onLogout and publishes unauthenticated state. Token/Desktop logout keeps synchronous teardown. Concurrent cookie logout calls share one pending promise. The captured post-initiation API session scope and user ID fence late success from replacing a newer session.
- views/auth/use-logout.ts passes shared protected-draft/cache cleanup at that confirmed boundary, awaits completion before navigation, deduplicates clicks and displays existing localized request-failed feedback on rejection. Failed logout preserves the session and drafts for retry.
- PasswordForm's direct restricted-session Sign out control awaits the same promise, locks duplicate submissions, catches failures into its existing error UI and preserves the session.
- ApiClient/server/cookie transport behavior is unchanged; no timeout or keepalive expansion was needed to fix the observed premature-success/navigation boundary.

## Verification

Four deterministic core regressions failed before repair: early teardown, failed-cookie-clear handling, replaced-session preservation and duplicate cookie requests. After repair, core authentication/provider/initializer suites67PASS; combined core auth+onboarding4files70PASS; shared logout/password-form2files20PASS including failure/retry feedback and no early navigation. Changed-file lint in core/viewsPASS. Root owns broader typecheck/full units and production browser checks after the idle performance window.

Independent review from password_lane found no blocking issue and confirmed Desktop direct call sites remain synchronous bearer-mode callers. Exact changed files: packages/core/auth/store.ts, store.test.ts; packages/views/auth/use-logout.ts, use-logout.test.ts, password-form.tsx, password-form.test.tsx. No served QA source/build was modified by this lane.

Required root verification: rebuild affected Web/Desktop and current directory packages; rerun real email logout/protected reload, password form/session flows, device startup adjacency and complete final suite. Relevant existing specs: auth-qa,auth,main-supplemental-qa; auth-device; password-registration,password-branches,password-migration and native password-session-persistence. Unrelated password-admin feature matrices need not be replayed merely because this bounded logout completion API changed, but their selected old evidence must be tied to unchanged behavior/source scope.

Private evidence: /Users/artisan/.multica/qa/core-20261108/logout-race/ (original files, repair.patch, sanitizedtrace, red/green/unit/lint logs). This document is not a final passing E2E claim.
