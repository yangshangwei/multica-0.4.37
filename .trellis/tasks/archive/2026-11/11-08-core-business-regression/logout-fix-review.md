# Independent logout fix review

Reviewed the six changed auth store/hook/password-form files and their regressions, plus the actual `ApiClient.logout`, platform session cleanup and direct Desktop callers. No blocking finding. Reviewer made no auth source edits.

Cookie logout now waits for the server's cookie teardown before clearing local credentials/drafts or publishing unauthenticated state. `useLogout` supplies cleanup as a callback invoked before that state transition, then navigates only after the operation resolves. A rejected logout keeps the authenticated UI and local data available, shows a localized generic error and permits an explicit retry. It no longer races navigation/session restoration with an outstanding cookie response.

The single pending promise prevents duplicate cookie requests; the user ID and API session scope guard prevent a delayed completion from clearing a replacement identity. `ApiClient.logout` increments its credential epoch synchronously before its first await, so the store captures the correct post-start scope. Bearer logout retains synchronous behavior; direct Desktop callers use that mode. Restricted password-form sign-out now awaits and handles rejection, preserving its existing state without exposing private error text.

New tests cover delayed success, failure/retry, replacement identity, shared pending logout, cleanup/navigation ordering, and restricted-form failure. Runtime/browser validation and the implementing agent's test results remain separate evidence. This review does not claim the final full suite passed.
