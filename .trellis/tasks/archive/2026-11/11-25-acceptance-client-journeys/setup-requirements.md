# Parent setup handoff

Planning files are ready for independent review: prd.md, design.md, implement.md, implement.jsonl, check.jsonl, original-evidence.json. No fixture/product edits yet.

## Service allocation
- Dedicated migrated API database on parent PG62450 (owner acceptance-fixes-4b9d3494a2), unique API/Web/renderer ports.
- Password API with MULTICA_PASSWORD_LIMITER_MODE=single, MULTICA_DEVICE_AUTH_ENABLED=false, no Cloud URL, MULTICA_MANAGED_INSTALLATIONS_ENABLED=true, unique MULTICA_DEPLOYMENT_ID and synthetic MULTICA_DEV_VERIFICATION_CODE. Do not change auth limits.
- Production Web build plus built Desktop renderer/preload, real runtime API/WS URLs, exact allowed origins for both clients.
- Actual listener PID/process-group/commit/start commands, Web BUILD_ID, renderer/preload hashes, API binary hash and current dirty-source manifest. Supply parent allocation record path; clients will create fresh journey adapters from it.
- Task-owned HOME/profile/tmp and verified existing browser cache. Per-case actor names; sequential groups with separate API rate-limit state when needed.

## Journey admission
- Manual detail uses COLLABORATION_DETAIL_TASK, fresh COLLABORATION_DETAIL_ALLOCATION_FILE and COLLABORATION_DETAIL_PROVENANCE_FILE with actual service proofs.
- PAY creation opts into PAY241_TC01_CREATION_ACCEPTANCE=1 and E2E_PASSWORD_AUTH=1.
- Snooze needs fresh INBOX_SNOOZE_E2E_ALLOCATION with private_environment_file, URLs, ports and database. Set runtime-ready only after proving those services.
- Recovery delayed-read needs fresh natural Desktop response-stage hold proof and parent acceptance before PAY241_S3_RESPONSE_STAGE_HOLD_ACCEPTED. Existing old clearance is invalid.

## Actual packaged Main
Capture full macOS URL protocol handler set plus default before any package preparation, repeat after preparation, before/after Main and exactly restore. Parent owns package preparation unless delegated explicitly; clients must bind its receipts to actual tests. Supply fresh macOS executable and Linux app package/image, bundled CLI hashes and synthetic version-only discovery CLI; all other agent paths point into owned missing paths. No ambient HOME/profile or personal credentials. Do not register a protocol without a recorded reversible restore path.

## Old CLI
Cached image sha256:8d5ede4df2fa4b66d499803b95428b729fcb6ba753b3800e9b931b968ee16104 (linux/amd64), fixed v0.6.7 identity. Need fresh static linux/amd64 e2e/fixtures/collaboration-installed-cli-relay.go binary mode0755, private owner0700 directory. Preserve readonly/networknone/capdropALL/no-new-privileges. Client lane will inspect historical timing ledger read-only and design a fresh measured adapter before any runtime mutation. Ten operations, 240 seconds and explicit cleanup inventory remain mandatory.
