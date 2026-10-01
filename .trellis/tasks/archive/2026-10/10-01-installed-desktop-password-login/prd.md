# Installed Desktop password login

## Diagnosis

The reopened process is `/Applications/Multica.app` version 0.6.0 (archive dated September 28), not the checkout's development Electron process. Its renderer lacks `password_auth_available`, `password-username` and the current server-identity footer. The saved runtime config points to the correct localhost:18573 API, which declares password authentication.

## Acceptance

- A packaged build contains the current username/password login and six-character password rule.
- Preserve the old installed application as a backup and retain existing runtime config and user data.
- The app launched from `/Applications/Multica.app` shows password login on two separate cold launches, never the email/Google form.
- Capture actual renderer/server-identity evidence. No release publishing or unrelated source modifications.

## Plan

1. Build the current checkout with the existing Desktop packaging script and disabled release publishing.
2. Inspect the packaged archive and test applicable runtime-config/login components.
3. Quit the old app, move it to a backup, and install the verified local bundle.
4. Launch and inspect the actual app via Playwright Electron/CDP, quit and relaunch, then leave the updated app open.
