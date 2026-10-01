# ApiClient authEpoch: a 401 only ends the session it belongs to

> Pattern from the 2026-09-06 audit closure (`fix/audit-open-findings`, Lane C).

## Problem

After a re-login, a request sent under the earlier credential can still be in
flight. Its 401 answer used to tear down the *new* session: token cleared,
user reset, `onSessionExpired` fired — even though the current credential is
valid. Cookie mode has no client-side token to compare against, so "compare
the token" cannot solve it.

## Solution

`ApiClient` (`packages/core/api/client.ts`) keeps a monotonic `authEpoch`:

- bumps on every credential change: `setToken` (bearer logins/logouts), a
  successful `verifyCode` / `googleLogin` / `deviceLogin` (cookie-mode logins,
  where the client never sees a token), and on a real teardown inside
  `handleUnauthorized`;
- every request path that can react to a 401 (`fetchRaw`, `uploadFile`,
  `publishPluginPackage`) captures the epoch where its headers are built and
  passes it back: `handleUnauthorized(sentEpoch)`;
- `handleUnauthorized` returns without side effects when
  `sentEpoch !== this.authEpoch` — the rejection speaks about a superseded
  credential, not the current session.

```typescript
const sentEpoch = this.authEpoch;          // captured at header build
// ... request ...
if (res.status === 401) this.handleUnauthorized(sentEpoch);

private handleUnauthorized(sentEpoch: number) {
  if (sentEpoch !== this.authEpoch) return; // stale answer: new session won
  // ... normal teardown, then bumpAuthEpoch()
}
```

## Why an epoch and not token equality

Cookie mode has no client-side token, and the client is the one component that
knows both when the credential changed and which request a response belongs
to. Bumping inside the login methods keeps the auth store free of transport
concerns.

## Compatibility (do not break these)

- A 401 that answers a request sent under the *current* credential still tears
  the session down (same epoch), including the desktop deep link
  (`loginWithToken → setToken → getMe` 401) and the MUL-7028 mid-flight
  rejection test.
- New request paths that call `handleUnauthorized` must capture and pass the
  epoch the same way.

## Tests

`packages/core/api/client.test.ts` — "ApiClient session expiry": table-driven
stale cases (`listProjects` / `uploadFile` / `publishPluginPackage` × bearer /
cookie) hold the first response, log in again, then release the 401 and assert
user, status, stored token and the callback are untouched; a same-epoch case
asserts the teardown still happens.

## Identity recovery and downstream error consumers

`GET /api/me` returns 401 when its session user no longer exists; database
lookup failures return 500. Older servers report the missing user as 404 with
the exact `user not found` error. Normalize only that method, path, status and
message combination to 401. Other missing resources and profile writes must
not end a session. Rejection must not send a cookie deletion: that response
could reach the browser after a newer login.

Guarding `onUnauthorized` alone is insufficient. `AuthInitializer` also
consumes identity/workspace errors and must not expire a newer login itself.
Auth `ApiError` instances expose a live `isStaleAuthResponse` check so a login
completed during body parsing or rejection propagation is also protected.

The client's separate `credentialEpoch` advances when a bearer credential is
installed or explicitly cleared, after successful cookie/device login, and
when cookie logout starts. Rejection teardown clears the client's token before
the store callback; its subsequent null-to-null `setToken` does not advance
this generation. Keep this distinction from `authEpoch`: duplicate 401s under
React StrictMode must still let the active initializer attempt device login
when no newer credential replaced the rejected session.

The initializer stops stale identity recovery without logout or device-login
handoff, and ignores stale workspace rejections. Error remapping must preserve
the live ownership check. Manually constructed errors retain normal rejection
behavior when no ownership check is supplied.

Regression coverage lives in `platform/auth-initializer.test.tsx` (real client,
bearer/cookie replacement, delayed body, workspace bootstrap and StrictMode
device login), `api/missing-user-session.test.ts`, handler `auth_session_test.go`
and `e2e/missing-user-session.spec.ts`.

## Endpoint changes include multipart requests

Switching servers freezes the old client, advances its endpoint generation and
aborts its pending requests. Both `uploadFile` and `publishPluginPackage` use
raw multipart fetches, so they must explicitly share that abort controller and
check the endpoint generation after the response and after body parsing. A
caller-provided upload cancellation signal is combined with the endpoint
signal; test cancellation and reason propagation rather than signal identity.
A frozen client cannot start a new upload. Regression coverage lives in
`api/endpoint-uploads.test.ts`.
