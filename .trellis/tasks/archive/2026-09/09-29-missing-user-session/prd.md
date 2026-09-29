# Missing-user session recovery

User reports auth init temporarily unavailable / user not found while visiting local MCP preview.
Evidence: API logs GET /api/me 404 with one unchanged session subject repeatedly; direct read-only DB query confirms zero matching users. GetMe maps all DB errors to 404; initializer treats non-401 as transient. Cookie domains do not isolate localhost ports, so this is reachable when moving between local databases.

## Fix and acceptance
- Backend GetMe returns 401 for pgx.ErrNoRows; other lookup failures return 500, not a fabricated missing user.
- API client normalizes only GET /api/me legacy 404 with exact user-not-found body to 401, preserving authEpoch guard. Ordinary missing resources/routes and outages never end a session.
- Handled 401 is a warning rather than console.error so normal session recovery does not open Next development error overlay. Do not silence unexpected errors.
- Regression tests first; verify current/missing/stale identity response and stale response after newer login, unrelated404/500 preservation.
- Browser reproduction uses a disposable fixture session whose user is absent, checks redirect to login and successful re-login into MCP market without console errors or retry loop.
- Keep the existing feature branch/worktree, no user data changes or cookie extraction, no unrelated auth refactor.

## Verification
Focused backend GetMe tests + API/auth initializer/store suites; scoped typecheck/lint; real API browser stale-session -> login -> market. Restart only isolated preview services to run current code.

## Completed review correction

The ApiClient callback guard alone did not protect AuthInitializer's own rejection path. Seven real-client integration regressions first demonstrated four newer-login races, delayed response-body parsing, delayed workspace rejection, and StrictMode device-login suppression. The final fix uses a separate credential generation for live ApiError ownership while retaining authEpoch for duplicate rejection handling. Both initializer consumers respect the live check.

Implementation and verification are complete; see verification.md.
