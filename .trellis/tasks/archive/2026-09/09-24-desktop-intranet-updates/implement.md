# Implementation and validation

- [x] Add artifact collection/publication with behavior tests; wire offline installer and its tests.
- [x] Fix Windows ARM64 downgrade policy with regression test and align desktop docs.
- [x] Add Compose/Nginx configuration, local lifecycle commands, safe client configuration helper and operational guide.
- [x] Start Nginx locally and publish existing ia32 build with explicit prerelease opt-in; verify health, HEAD/GET, full checksum, 206 Range, missing files, listings, write rejection and restart persistence.
- [x] Run focused desktop/script tests, desktop lint/typecheck, docs generation/checks and static syntax/diff checks; review full diff independently.
- [x] Record evidence and remaining installed-client acceptance in task artifacts and specs; commit only this task's changes, preserving unrelated work.

Rollback: stop the separate Compose project without deleting host data; restore desktop.json from its backup if configured. Keep historical artifacts and metadata snapshots. Do not modify existing business Compose services.

## Handoff

- [x] Extend the existing shell entry with persistent env configuration, image export, collection and HTTP verification; add command regressions to CI.
- [x] Reuse metadata reference validation and verify actual HTTP file bytes with expected-version, HEAD, Range and SHA-512 checks.
- [x] Expand the operator runbook, add the env template and explain the separate business-server upgrade workflow.
- [x] Exercise the real CLI against an isolated Nginx project and actual ia32 build; record ten checks in operations-verification.json. Real installed-client acceptance is still pending.

Local implementation and HTTP acceptance are complete. The development task is archived at the user's request; real installed-client acceptance remains a deployment follow-up in the operator runbook. The existing dirty ia32 build was served unchanged with explicit prerelease opt-in. No user Desktop configuration was modified and no application was installed or restarted.
