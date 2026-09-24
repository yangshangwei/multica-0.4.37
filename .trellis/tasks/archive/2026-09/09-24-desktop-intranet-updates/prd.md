# Desktop intranet updates and local download storage

## Request
Implement the existing desktop intranet update plan and run an Nginx file server on this development machine. The user explicitly authorized task creation and development.

## Requirements
- Persist desktop installers and electron-builder update metadata on the host and serve them through a fixed /desktop/ HTTP directory.
- Preserve existing application services and client business endpoints.
- Provide repeatable service start/stop/status, artifact collection/publication, and client update-source configuration.
- Validate referenced filenames, SHA-512 and sizes before publishing; preserve old version artifacts and publish metadata last.
- Include metadata/blockmaps in offline installer delivery, including Windows ia32.
- Prevent unintended Windows ARM64 downgrade when selecting its channel.
- Document local use, intranet transport, failures and remaining real-client acceptance.
- Provide persistent deployment configuration, architecture-specific offline image export and a repeatable HTTP verification command; keep a detailed operator runbook with release records and the separate business-server upgrade handoff.

## Acceptance
- Nginx is running locally with a healthy endpoint, no directory listing, no-cache metadata, successful complete and HTTP Range downloads of actual existing build artifacts.
- Container restart retains files. Invalid releases fail without replacing active metadata; immutable filenames cannot be overwritten.
- Focused tests, lint/typecheck, configuration syntax and artifact checksum verification pass or pre-existing unrelated failures are evidenced.
- The local dirty prerelease is explicitly treated as download smoke-test material, never claimed as a stable production release.
- Signed macOS/Windows/Linux end-to-end installation remains a separately recorded real-device acceptance item; this task must not claim it from unit or HTTP tests.

## Out of scope
Release management UI, upload API, forced restart, automatic rollback, signing credentials and changes to business storage.
