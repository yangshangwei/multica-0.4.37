# Installed Desktop password login — verification

## Cause

Reopening launched `/Applications/Multica.app` (0.6.0, archive dated September 28) rather than the recently modified development build. Inspection of the installed ASAR confirmed its renderer lacked the password-auth capability handling, username input and server-identity footer. The saved `~/.multica/desktop.json` already pointed at localhost:18573, and that server correctly declared password mode.

## Action

- Built the current checkout using `CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @multica/desktop package -- --mac --arm64 --dir --publish never`.
- Build version follows the repository's git-derived version: `0.5.2-86-g711569e04-dirty`. This is a local working-tree build, not a published release.
- Verified the staged packaged application with an isolated profile before installation.
- Quit the old installed process and backed up the entire application to `/Users/artisan/.multica/backups/desktop-2026-10-01T03-09-20-834Z/Multica.app`.
- Installed `apps/desktop/dist/mac-arm64/Multica.app` at `/Applications/Multica.app`. Existing runtime config and user data were retained.
- Opened the installed app normally after verification so it is ready for the user.

## Evidence

- Desktop runtime-config/login tests: 17 passed.
- Desktop main and renderer typechecks: passed.
- Package build: passed; ASAR contains password auth, username input and six-character guidance.
- `codesign --verify --deep --strict /Applications/Multica.app`: passed (local ad-hoc signature).
- Two separate Playwright Electron cold launches of `/Applications/Multica.app/Contents/MacOS/Multica` both confirmed:
  - `app.isPackaged === true` and actual archive path inside `/Applications/Multica.app`.
  - Existing user-data path `/Users/artisan/Library/Application Support/Multica`.
  - Chinese language, configured API `http://localhost:18573`.
  - Username/password fields visible; email input and Google button absent.
  - Registration guidance contains `6–128` and employee-ID recommendation.
  - No renderer page errors.
- Screenshots: `/tmp/multica-installed-password-launch-1.png`, `/tmp/multica-installed-password-launch-2.png`.
- Structured evidence: `/tmp/multica-installed-password-verification.json`.
- Build log: `/tmp/multica-desktop-password-package.log`.
- `git diff --check`: passed. No application source changes were necessary in this follow-up; the missing step was updating the installed binary.

## Limits

This local build is ad-hoc signed and was not published/notarized. Verification exercised cold launch and registration UI without creating an account or changing the user's session manually. The configured local update endpoint was unreachable, so no update publication was attempted.
