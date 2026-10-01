# Remember Desktop login

- After a successful password login, closing and reopening the same installed Desktop restores the valid session without displaying the login form.
- Persist the issued session token, never the user's password.
- Explicit logout removes the remembered session. Rejected/revoked credentials require login; temporary connectivity failures retain the session and retry.
- Verify existing shared persistence before adding any new authentication mechanism. The development app and installed app have different user-data directories and do not share sessions.
- Add regression coverage and verify actual Electron restart behavior using an isolated profile and native-service stubs so the user's running application and daemon stay untouched.
