# Research: Seven password-directed acceptance cases

- Query: Determine exact C3/password test identities, preparation requirements, native/artifact/dependency bindings and the command that must yield seven actual passes.
- Scope: internal; existing original five-file/seven-case password lane only. No services, DB/migrations, tests/listing, account/Provider execution, installs, code changes or git commands.
- Date: 2026-11-03
- Snapshot: 2026-11-03T01:19:44.606065+00:00

## Findings

Status: **ready for root setup; not run**. The current owned candidate is `304f74dd3ba4ddea862c94917c5bb0770355a351` at `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103`, source manifest ID `4622182a7e129f1712f883eaea36a4756b183f539b5733eb818b7f8df9195fcb`. The older password preparation belongs to candidate64a and is only historical `prepared_not_started` evidence. New owner/registry/ports/DB/env/results must be bound to the actual current runtime.

### Fixed inventory

One Chromium project expands the five original files to exactly seven tests: **C3 four + runtime handoff three**. No source test was edited or executed by this research.

| Identity | Source | Exact test title |
| --- | --- | --- |
| C3-web-directed-chain | `e2e/collaboration-handoff-chain-web.spec.ts:11` | Web directed handoff chain moves current todos with responsibility and only the creator completes |
| C3-native-information-return | `e2e/collaboration-handoff-chain-desktop.spec.ts:16` | Electron information return shows the original requester and note before restoring responsibility |
| C3-web-offline | `e2e/collaboration-offline-handoff.spec.ts:12` | Web offline invite and assignment need no acceptance; transferred-away work survives archive and fresh login until removal |
| C3-native-offline | `e2e/collaboration-offline-handoff.spec.ts:12` | Electron offline invite and assignment need no acceptance; transferred-away work survives archive and fresh login until removal |
| runtime-env1-startup | `e2e/collaboration-runtime-handoff-web.spec.ts:9` | ENV-1 live latest shared context preserves the run's empty startup version |
| runtime-web-recall-removal | `e2e/collaboration-runtime-handoff-web.spec.ts:47` | Web recipient recalls retained handoff sources and omitted comments, then loses private reads on removal |
| runtime-native-recall | `e2e/collaboration-runtime-handoff-desktop.spec.ts:14` | Electron deep link opens retained handoff context and honestly marks an unknown recording tail |

All seven skip unless `E2E_PASSWORD_AUTH=1`. The native information-return case additionally skips if renderer URL is absent (`collaboration-handoff-chain-desktop.spec.ts:18`); the other native cases actively reject absent/invalid renderer. Do not remove these guards or reinterpret skipped legacy results as pass. Config already has workers1, retries0 and no auto-start server (`playwright.config.ts:5`). Preserve each test's existing timeout and assertions.

### Password and service setup

`offlineHandoffEnvironment()` (`e2e/fixtures/collaboration-offline-handoff.ts:13`) requires explicit loopback API, NEXT_PUBLIC_API_URL and DATABASE_URL; API origins must match. DB cannot be empty/default multica/postgres/template0/template1. `seedOfflineHandoff:53` requires live `/api/config` auth_mode=password, creates synthetic password actors through real public auth APIs, completes onboarding, requires the external recipient to have no workspaces, creates one source workspace and requires creation-options.creation_enabled=true. No user account or installed agent is used.

Set the complete task-only password configuration: `MULTICA_AUTH_MODE=password`, `MULTICA_DEVICE_AUTH_ENABLED=false`, blank `MULTICA_CLOUD_URL`, and `MULTICA_PASSWORD_LIMITER_MODE=single` for one API. Multiple APIs require shared mode and task Redis (`server/internal/auth/password_config.go:25`, `.trellis/spec/web/frontend/e2e-run-environment.md:76`). Keep ALLOW_SIGNUP=true and DISABLE_WORKSPACE_CREATION=false (`server/cmd/server/router.go:430`). Password registration is separately limited to30/IP/5minutes and login300/IP/minute (`auth_password.go:132`, `:176`); the seven fresh fixtures normally create14 accounts, so use a fresh task-only API budget. Do not weaken production rate limits or login assertions.

Creation is a database gate, not a presumed S6 environment flag: creation options read collaboration_execution_gate.creation_enabled (`collaboration_creation_options.go:59`). Migration744 opens the old fresh default but preserves a prior explicit administrator close. Root verifies the actual task DB row/API; these cases do not require opening execution/host/knowledge gates. Runtime fixtures use public human writes plus direct transactionally seeded retained run/checkpoint/seal rows, with target/channel paused; they do not start a real Provider (`collaboration-runtime-handoff.ts:14`, `:61`).

Root must wait for the default complete check and its owned Web process to finish before changing the checkout's shared `.next` output. Use a fresh private registry (`MULTICA_DEV_HOME`), own database/ports/profiles/workspaces and a Git-external0700 TMPDIR; preserve the source deployment mode. `make up` defaults to development, so select **production explicitly**:

```sh
make up ENV_FILE=/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103/.omx/private/password-directed-20261103/source.env C=api,web ARGS='--name closure-password-20261103 --web-mode production'
```

Set the task-only environment through the root's structured private launcher. `scripts/dev-env.sh:30` owns registry and profile paths; `:48` selects MULTICA_WEB_MODE, and `:803` builds production Web then next start under a shared build lock. The same explicit ENV_FILE must reach Make and its Bash child. Root records actual allocated values rather than reusing old18247/13167/19845 assumptions.

Before building/starting, bind NEXT_PUBLIC_API_URL, REMOTE_API_URL and NEXT_PUBLIC_WS_URL to the actual password API/WS, PLAYWRIGHT_BASE_URL/FRONTEND_ORIGIN to the actual production Web, and COLLABORATION_ELECTRON_RENDERER_URL to the owned renderer. API/Web should use the same hostname literal for HttpOnly cookies. TestApiClient's API_BASE is captured at fixture module import (`e2e/fixtures.ts:13`) and e2e/env only fills unset variables; explicit runner values must precede import. Root must verify production SSR and actual page API/WS addresses, not only a200HTML response. The API's CORS_ALLOWED_ORIGINS and ALLOWED_ORIGINS must include the exact Web and renderer origins **before startup/provenance**; Electron's browser webSecurity setting does not bypass server WS origin checks (`e2e-run-environment.md:43`).

### Native and cached artifacts

The exact seven tests have **no Docker image or installed CLI prerequisite**. On this host, native opens the candidate's unpackaged Electron39.8.7 executable, executes `collaboration-account-electron.mjs`, and uses the actual built preload/renderer (`collaboration-native.ts:18`). That Main fixture extracts production routing/IPC functions and imported native modules from source, compiles a task-profile CJS file, and prevents daemon start. It is not the packaged application Main lifecycle (`collaboration-account-electron.mjs:15`, `:36`, `:86`, `:181`). These seven must not be reported as packaged Mac/Windows/Linux installer, offline image, real runtime ingestion or Provider evidence.

Fresh filesystem observations: Playwright/test/core1.58.2, pg8.20.0, Electron39.8.7, electron-vite5.0.0, TypeScript5.9.3, Vite8.0.1 and esbuild0.25.12 all exist under the owned candidate's node_modules realpaths. Native fixture resolves Vite through electron-vite's package context; absent apps/desktop/node_modules/vite is not a missing dependency. The loader uses createRequire and buildSync (`collaboration-account-electron.mjs:17`), so root must preserve this owned resolution and avoid mixing host/framework copies.

- Native entry executable SHA256 `cf74833a9b0c242e26c177005d7d61819e5bf5ed199acc794fed2eb0cc4f3149`.
- Electron Framework SHA256 `0a85572e116a6fd83549c6669d72e4ff65d86bdf6e69952db882b609d92f808f`.
- Preload index SHA256 `68a9036e201a462ffd10a1a4890e052a0b6b342c0b74d6d8ac489d9f5d8e78e5`.
- Renderer index SHA256 `9d92714285456d587bb8bcd3ecf2897ad015cf7f63d76f75fea9a541536ae0eb`.
- Native fixture SHA256 `f9021ab07635f2f48d0f90613fcd3c8558cb7262b3bdd2aca98edb31654475d2`.
- Bound renderer whole manifest:493files, `bc46754412b91fac5f6649b7b37ad537e067bb9ab6fa6f0c4012cb0fced91d18`; root rechecks after any build/config change.

The owned CLI's recorded c008.../commit304f build is included in JSON for provenance, but none of these seven launches it. Chromium1208/145.0.7632.6 and matching headless-shell1208 already exist in the local binary cache. Set a verified PLAYWRIGHT_BROWSERS_PATH explicitly if using isolated process HOME; do not download a new version. Cache existence is not a browser launch result. Owned Node22.23.2 and pnpm10.28.2 exact paths/hashes are in JSON; prepend their paths for the subprocess and preserve all test framework instances at1.58.2.

### Exact command and acceptance comparison

From `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103`, first run the same five-file selector with `--list` and compare the seven titles above. After root readiness, use the bound Node/pnpm command:

```sh
/Users/artisan/.nvm/versions/node/v22.23.2/bin/node /Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103/.omx/private/owned-workspace-links/pnpm-10.28.2/bin/pnpm.cjs exec playwright test e2e/collaboration-handoff-chain-web.spec.ts e2e/collaboration-handoff-chain-desktop.spec.ts e2e/collaboration-offline-handoff.spec.ts e2e/collaboration-runtime-handoff-web.spec.ts e2e/collaboration-runtime-handoff-desktop.spec.ts --project=chromium --workers=1 --retries=0 --reporter=json --output=/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103/.omx/private/password-directed-20261103/artifacts
```

Set `PLAYWRIGHT_JSON_OUTPUT_FILE=/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103/.omx/private/password-directed-20261103/playwright-results.json` for a new raw JSON report. The installed JSON reporter's resolveOutputFile uses that variable (`node_modules/playwright/lib/reporters/base.js:565`); output artifacts use the private path in the command. This research did not run even the list command.

Require exact seven file/title/project identities, one passed result each, retry0, no skip/flaky/interrupted/missing/extra cases, no top-level errors, and full command exit0. Preserve original test attachments: handoff-chain-evidence, native-information-return-evidence, offline-handoff-identities for both platforms, ENV-1-startup-and-latest-context, source-run-navigation, runtime-handoff-web-evidence, and runtime-handoff-native-evidence. A totals-only report cannot substitute for that inventory/provenance.

Each native context creates a unique taskTMPDIR profile, sets actual userData to it, and retains auth/daemon observations. Root should bind the actual path, owner/process, executable/framework, fixture, preload and renderer; original assertions require errors=[], account requests with no workspace headers, and daemonStarts0. Native/offline re-login and source/private-read removal must use only fixture-created accounts. Tests close their own contexts/native windows and remove only exact mkdtemp profiles in finally. Do not clean user profiles, overwrite old failures or drop diagnostic databases merely to reset a result.

### Related files and specs

- Five test files/table above — canonical acceptance assertions and guards.
- e2e/fixtures/collaboration-offline-handoff.ts:13 — explicit local/password identity, synthetic sessions, offline workflow and live authority.
- e2e/fixtures/collaboration-runtime-handoff.ts:14 — retained seed/projection/full-text/paging assertions; no ingestion proof.
- e2e/fixtures/collaboration-native.ts:18 and collaboration-account-electron.mjs:15 — actual native source/preload/renderer/profile boundary.
- playwright.config.ts:5 and e2e/env.ts:5 — original1worker/0retry/production services supplied by caller and dotenv fallback.
- server/internal/auth/password_config.go:25 — password/device/Cloud/limiter startup gate.
- server/cmd/server/router.go:430 and auth_password.go:132 — signup/workspace policy and retained real auth budgets.
- server/migrations/744_collaboration_creation_default.up.sql:6 — fresh creation default and explicit admin close preservation.
- .trellis/spec/web/frontend/e2e-run-environment.md:43, :76, :314 — renderer origins, password startup and source/API/budget identity.
- .trellis/spec/server/database-test-isolation.md — actual task DB ownership/connection and distinct original failed evidence.
- .omx/private/final-check-v11/owned-bindings-v11-provenance.json — current corrected source, tools, renderer/preload/main/CLI and owned workspace dependencies.

External references: none needed; installed Playwright1.58.2 reporter and repository source define exact commands/requirements. JSON stores every original test/fixture source hash separately because the product source manifest does not include e2e/.

## Caveats / Not Found

No password services, DB, migrations, tests/listing, browser/Electron process, synthetic account or Provider were started. Runtime availability, listener ownership, dynamic native profile assertions and actual7pass remain unproved. Cached browser/native dependencies and built artifacts are present, but root still must verify live setup, matching source/config and full report. Broader packaged/installed/offline images, cross-platform installers, checkpoint ingestion, real Provider/model behavior and knowledge/runtime admission remain separate acceptance gates.
