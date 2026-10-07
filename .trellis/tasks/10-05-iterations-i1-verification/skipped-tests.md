# Skipped E2E inventory

The accepted full check reports **249 passed, 49 skipped, zero failures**. All four I1 tests ran and passed. Every skipped test remains **not run**, never a PASS. [skipped-tests.json](skipped-tests.json) preserves exact test titles, source lines, conditions and the source-log hash.

| Source | Count | Missing opt-in fixture |
| --- | ---: | --- |
| `e2e/admin-resource-publishing.spec.ts` | 3 | Requires task-owned API, Web, database and managed storage |
| `e2e/auth-device.spec.ts` | 1 | Requires a dedicated API with MULTICA_DEVICE_AUTH_ENABLED=true |
| `e2e/changelog-desktop.spec.ts` | 1 | Requires built desktop preload and a local renderer/API fixture |
| `e2e/changelog.spec.ts` | 1 | Requires a task-owned CHANGELOG_E2E_API_URL and CHANGELOG_E2E_FEED |
| `e2e/desktop-settings.spec.ts` | 1 | Requires built desktop preload and a local renderer/API fixture |
| `e2e/mcp-desktop.spec.ts` | 1 | Requires a local password-mode API and freshly built desktop renderer/preload |
| `e2e/mcp-market.spec.ts` | 1 | Requires a task-owned directory mounted by the local API |
| `e2e/password-binding-desktop.spec.ts` | 1 | Requires a local password-mode server and migration window |
| `e2e/password-branches.spec.ts` | 12 | Requires isolated password-mode deployment with signup enabled |
| `e2e/password-migration.spec.ts` | 2 | Requires isolated password-mode deployment |
| `e2e/password-registration.spec.ts` | 1 | Requires isolated password-mode deployment |
| `e2e/password-server-switch.spec.ts` | 1 | Opt-in isolated password acceptance |
| `e2e/platform-admin-accounts.spec.ts` | 1 | Requires a task-owned password-mode production build |
| `e2e/platform-admin-controls.spec.ts` | 1 | Requires the isolated password-mode production deployment |
| `e2e/platform-admin-executions.spec.ts` | 1 | Requires the isolated password-mode production deployment |
| `e2e/platform-admin-installations-read.spec.ts` | 1 | Requires the isolated password-mode production deployment |
| `e2e/platform-admin-observability.spec.ts` | 1 | Requires the isolated password-mode production snapshot |
| `e2e/platform-admin.spec.ts` | 1 | Requires a task-owned password deployment and bootstrapped administrator |
| `e2e/quick-create-actor-picker.spec.ts` | 1 | Requires the built desktop renderer/preload and isolated local API fixture |
| `e2e/retained-languages-desktop.spec.ts` | 2 | Requires built Desktop renderer/preload and explicit isolated API/database configuration |
| `e2e/upstream-backports-cli.spec.ts` | 3 | Requires an explicitly supplied task-owned MULTICA_E2E_CLI_BINARY |
| `e2e/issue-assist-flow.spec.ts` | 8 | Requires the deterministic AI provider and isolated API configuration |
| `e2e/quick-create-actor-picker-phase2.spec.ts` | 3 | Requires the deterministic local recommendation provider; Requires isolated desktop renderer and provider; Requires deterministic local provider |

These are separate feature fixtures, not I1 acceptance scenarios. This run does not prove password-mode browser compatibility, the optional compiled-CLI matrix, other features’ standalone Electron fixtures, deterministic AI providers, resource publication, or live feed behavior. I1 old-request/API/schema/Mobile compatibility is documented separately; it is not inferred from these skipped scenarios.

The final four-case I1 rerun uses a dedicated output directory and leaves this full-suite inventory intact. Remote CI and production checks remain unexecuted under the local-only instruction.
