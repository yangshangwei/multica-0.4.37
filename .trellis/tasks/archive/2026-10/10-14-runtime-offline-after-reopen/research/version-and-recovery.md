# Research: v0.6.5 credential recovery and existing v0.6.6 fix

- Query: Which release-specific behavior explains recovery after stopping the daemon, logging in again and restarting it?
- Scope: Windows Desktop credential handoff, legacy daemon token loading, release provenance and bounded existing regressions.
- Date: 2026-10-14

## Incident evidence

The user confirmed the installation uses v0.6.5, not current HEAD. The recovery script stops the old daemon, obtains login credentials again, then starts a new daemon. The supplied successful heartbeat interval was not correlated with the offline display. The script is a combined intervention; it does not prove natural token expiry, and it also replaces a possibly unhealthy process.

`authenticated` is logged when a nonempty local token is loaded, not after a remote authentication check (`server/internal/daemon/daemon.go:2109`). The subsequent renewal response, workspace registration and heartbeat ACKs are the remote-success evidence in this log. The reported expiry is 2027-01-12; it describes the credential used at that moment, not the credential before recovery.

## Confirmed code behavior

1. The legacy daemon loads its profile credential into its in-memory client at startup (`server/internal/daemon/daemon.go:2109`). Editing the profile or logging in elsewhere does not automatically reload that client's credential. Managed workspace transports have their own replacement mechanisms; do not generalize the legacy behavior to them.
2. Startup attempts renewal before workspace synchronization. Thereafter renewal is every three days (`daemon.go:3487`, `:3506`, `:3523`). A 401 logs a re-login-and-restart warning (`:3552`). It cannot mint a replacement using an invalid credential.
3. In v0.6.5, `apps/desktop/src/main/daemon-manager.ts` reused a cached `mul_` token whenever its owner sidecar matched the same user, without checking whether it had been revoked. Its `daemon:sync-token` IPC restarted the legacy daemon for a user change, not every token change. A CLI startup failure could also enter `stopped` before the delayed auth probe exposed `auth_expired`.
4. Current `syncToken` probes the exact cached credential and remints only after a confirmed 401 (`apps/desktop/src/main/daemon-manager.ts:828`, `:858`). The IPC restarts for `tokenChanged` as well as `userChanged` (`:1701`), using the existing lifecycle gate. Manual recovery mints before replacing the old profile and restarting (`:1005`). Transient errors preserve the user's session (`apps/desktop/src/renderer/src/platform/daemon-reauth.ts:19`).
5. Password binding/change preserves user identity while revoking derived PATs. Thus a token can fail before its displayed expiry and with an unchanged user ID. This is an example trigger supported by code, not evidence that the user changed their password in this incident. See the server research and `.trellis/spec/desktop/frontend/quality-guidelines.md`.

The causal candidate is: unusable cached credential → old Desktop does not refresh/restart correctly → runtime heartbeats stop being accepted → runtime and bound agent availability becomes offline → the user's re-login-and-restart script restores a valid client. The recovery deficiency is confirmed in v0.6.5; the original credential failure remains unproven.

## Release provenance

- v0.6.5 resolves to `24771b0d8`, dated 2026-10-11.
- `296c377a4aafb84a87aa6d967d5e868405e19900`, dated 2026-10-12, is `fix(desktop): restore device connectivity after password binding`.
- v0.6.6 resolves to `4797e6d97`, dated 2026-10-13.
- `git merge-base --is-ancestor 296c377a4 v0.6.6` returned exit 0.
- `git diff v0.6.6 HEAD -- apps/desktop/src/main/daemon-manager.ts apps/desktop/src/main/daemon-manager-auth.test.ts` was empty. This proves source inclusion, not installation on the user's PC or correct packaging of any particular download.
- The inspected heartbeat, renewal and sweeper implementations have no v0.6.5-to-HEAD difference. Inspected token middleware/PAT handler/WS hub files are also unchanged. `password_revoke.go` gained collaboration revocation identity handling; revoking PATs and taking the owner's runtimes offline were already present in v0.6.5.

## Secondary candidate

The server can synchronously restore an offline runtime via heartbeat without broadcasting the existing runtime invalidation. The desktop research lane also observed an infinite default query stale time and no runtime-list refetch interval. This can support stale UI after a pure resume, but cold registration does broadcast, and the user's combined script does not isolate this case. Do not expand the primary fix into that notification path unless the API is online while the UI stays offline at the same moment.

## Verification

The coordinator ran 44 Desktop auth/recovery tests and 12 daemon renewal/heartbeat tests; all passed. Fixtures use local mocks or loopback HTTP and fake process boundaries. See `../verification.md` for exact commands and limits. The separate server lane ran five additional WS/TTL tests.

No installed agent CLI was executed, no real token was read or recorded, and no product code or production data was changed.

## Related specs

- `.trellis/spec/desktop/frontend/quality-guidelines.md`
- `.trellis/spec/desktop/frontend/connection-recovery.md`
- `.trellis/spec/server/security-boundaries.md`
- `.trellis/spec/server/managed-installations.md`
