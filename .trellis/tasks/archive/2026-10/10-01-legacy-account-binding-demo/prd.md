# Legacy-account binding simulation

- Reproduce an existing account that has workspace/history but no password credential and still holds a valid pre-migration JWT.
- Show the real Desktop account-setup screen, bind a numeric username and six-character password, and preserve account/workspace/issue/comment identity.
- Verify old-token revocation, new password login and persisted login on restart.
- Capture screenshots and a concise before/after report.
- Use only a uniquely named test account/workspace and an isolated Electron profile; do not modify existing users or their sessions. Clean up the simulation data after verification.
- Document the distinction between a usable old session during the migration window and an expired/missing session requiring operator recovery.
