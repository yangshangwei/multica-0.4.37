# Legacy-account binding simulation

## Result

The existing Desktop flow successfully binds a username and password to the original user. It does not create another account.

The simulation used the real local API/database and current built Desktop renderer/preload. A dedicated fixture account was given an email, workspace, issue and comment; its password credential was then removed and a pre-migration session was issued only for that test identity. No external email or OAuth provider was invoked. Native daemon operations and browser storage were isolated.

## Steps observed

1. Restore the legacy session while its migration window is open.
2. `GET /api/me` identifies the original user and sets `requires_account_setup=true`. Business APIs remain restricted until setup.
3. Desktop automatically shows **设置登录账号**, with the existing display name prefilled.
4. Submit a numeric username and a six-character password through `POST /api/me/password/setup`.
5. The response carries the same user ID and a new session. The original workspace and history become accessible.
6. The old session returns 401. Password login succeeds for the same user.
7. Closing and reopening Electron restores the new session without asking for login or binding again.

## Preserved data

| Item | Verified result |
| --- | --- |
| User ID | `048651f3-79dd-44e7-9b77-06172c552945` before and after |
| Workspace | Original ID retained |
| Historical task | Original task ID and creator retained |
| Historical comment | Original comment ID and content retained |
| Old login credential | Rejected after binding |
| New password login | Successful |
| Desktop restart | Logged-in state restored |

Exact identifiers are in [binding-result.json](evidence/binding-result.json). Screenshots: [binding screen](evidence/before-binding.png), [after binding](evidence/after-binding.png), [after restart](evidence/after-restart.png).

## Evidence and cleanup

- Added `e2e/password-binding-desktop.spec.ts` as a repeatable simulation.
- `E2E_PASSWORD_AUTH=1 make env-exec ARGS='-- pnpm exec playwright test e2e/password-binding-desktop.spec.ts --reporter=line'`: passed.
- `git diff --check`: passed.
- Verified all five test records were removed afterward: user, password credential, workspace, issue and comment. Existing users and the running installed app were not modified.

## Binding prerequisites

The old account must be on the same server and still have a valid login session issued before the configured migration cutoff. Setup must occur before the migration deadline. The current local window ends at 2026-10-08 10:04:27 Asia/Shanghai.

If the old session is missing/expired or the migration window has ended, the user cannot self-claim an old account by entering a name/email. The deployment operator can use the existing `password-recover --user <original-user-id> --username <username>` path to attach a temporary password to the original user, then the user signs in and changes it. That recovery path was not exercised in this Desktop simulation.
