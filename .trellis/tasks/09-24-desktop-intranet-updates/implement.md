# Implementation and validation

- [x] Add artifact collection/publication with behavior tests; wire offline installer and its tests.
- [x] Fix Windows ARM64 downgrade policy with regression test and align desktop docs.
- [x] Add Compose/Nginx configuration, local lifecycle commands, safe client configuration helper and operational guide.
- [x] Start Nginx locally and publish existing ia32 build with explicit prerelease opt-in; verify health, HEAD/GET, full checksum, 206 Range, missing files, listings, write rejection and restart persistence.
- [x] Run focused desktop/script tests, desktop lint/typecheck, docs generation/checks and static syntax/diff checks; review full diff independently.
- [x] Record evidence and remaining installed-client acceptance in task artifacts and specs; commit only this task's changes, preserving unrelated work.

Rollback: stop the separate Compose project without deleting host data; restore desktop.json from its backup if configured. Keep historical artifacts and metadata snapshots. Do not modify existing business Compose services.

## Handoff

Local implementation and HTTP acceptance are complete. Keep the task active for operator review and real installed-client acceptance. The existing dirty ia32 build was served unchanged with explicit prerelease opt-in. No user Desktop configuration was modified and no application was installed or restarted.
