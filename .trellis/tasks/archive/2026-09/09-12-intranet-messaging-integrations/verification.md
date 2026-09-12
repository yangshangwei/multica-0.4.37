# Verification

## Result

- `MULTICA_MESSAGING_INTEGRATIONS_ENABLED=false` suppresses all five provider startup paths and the independent WeCom relay dispatcher. Nil-service responses reject binding/install/revoke operations; DingTalk observation deletion now uses the same unavailable guard.
- `/api/config` includes the explicit messaging policy independently of VCS.
- Shared Settings and agent UI respect the policy, disable provider queries, disregard cached configured installations, and recover stale integrations navigation.
- An independent review found the startup default still exposed messaging before config loaded. Three new failing cases (network error, invalid JSON, HTTP 503) proved it; the config store now starts disabled and enables legacy behavior only after a successfully loaded older-server response.

## Checks

- Root `pnpm typecheck`: all 9 tasks passed (web, desktop, docs, shared packages); core typecheck repeated after the startup-default correction.
- Focused core Vitest: 185 tests passed across API config schemas and AuthInitializer.
- Focused views Vitest: 58 tests passed across Settings integrations, AgentOverviewPane, and agent integrations.
- Focused Go tests on disposable `multica_messaging_zigozo`: 18 router/startup tests and 28 handler/config/DingTalk tests passed. The database was cloned from an idle test fixture, migrated to the current schema, and never shared with the running application.
- `go vet ./cmd/server ./internal/handler`, gofmt and changed-file ESLint passed.
- In-app docs generator, 43 docs-bundle tests, and `go test ./internal/docs/...` passed.
- Compose rendering preserves both messaging values while VCS stays enabled. Helm lint and configuration-rendering tests passed using a checksum-verified temporary Helm binary; no project dependency was added.
- Impeccable scan returned no findings. Live Electron Settings screenshot/AX tree contains only self-hosted Git; live agent Capabilities contains Instructions, Skills, and MCP without messaging integrations. Visual verdict persisted under `.omx/state/intranet-messaging-integrations/ralph-progress.json`.
- `git diff --check` passed.

## Local runtime

The existing checkout `.env` now sets messaging to false and VCS to true. Only the checkout's API component was restarted. The running API at localhost:18572 reports `messaging_integrations_enabled=false` and `vcs_integration_available=true`. Web was temporarily started for verification; the existing desktop was used for the final visual checks.

## Limits

- No live public messaging providers or cross-replica Redis deliveries were exercised.
- The local VCS encryption key is still unset. The Git integration entry remains available, but connecting an actual intranet Git service still requires its encryption key, instance URL, and access token.
- Existing unrelated catalog and skill changes were preserved. This task did not commit, push, publish, or deploy to a remote server.
