# Legacy supplemental execution manifest

Prepared from current fixtures and the prior `legacy-lanes.py`; preparation only, no services or tests executed. Target checkout: `/Volumes/artisan/code/2026/mc-core-qa-20261108`. Counts below are the **42 skipped identities in the 2026-11-08 original full run**, not new execution results. Match new results by spec/title; selecting an entire mixed file can also rerun previously passing cases.

## Baseline-compatible additions: 28 skipped identities

These can join the default full legacy suite with one worker and zero retries. Complete API startup configuration before starting tests; adding a variable only to Playwright does not reconfigure the running API.

Common API configuration remains `MULTICA_AUTH_MODE=legacy`, `MULTICA_DEVICE_AUTH_ENABLED=false`, `MULTICA_MANAGED_INSTALLATIONS_ENABLED=false`, `MULTICA_PLATFORM_ADMIN_ENABLED=false`, `MULTICA_KNOWLEDGE_CONTEXT_ENABLED=false`, `ALLOW_SIGNUP=true`, and the task-owned development verification code. Clear inherited cloud and commercial LLM configuration. Use the allocated database, API/Web/renderer origins, source root and actual production builds. Include Web and renderer origins in both API CORS and WS allowed origins.

Set these values in the actual indicated processes:

| Process | Variable | Required value |
| --- | --- | --- |
| API + test | `MULTICA_DEPLOYMENT_ID` | One newly selected task UUID, identical on both sides |
| Test | `COLLABORATION_E2E_API_URL`, `NEXT_PUBLIC_API_URL`, `CHANGELOG_E2E_API_URL` | Same allocated API origin |
| Test | `PLAYWRIGHT_BASE_URL` | Allocated production Web origin |
| Test | `COLLABORATION_ELECTRON_RENDERER_URL`, `CHANGELOG_ELECTRON_RENDERER_URL` | Same allocated built renderer origin |
| Test | `COLLABORATION_ELECTRON_SOURCE_ROOT` | Exact current checkout path |
| Test | `MULTICA_E2E_DESKTOP_OUTPUT_DIR` | `<checkout>/apps/desktop/out` |
| Test | `CHANGELOG_ELECTRON_PRELOAD` | `<checkout>/apps/desktop/out/preload/index.js` (explicit binding; fixture otherwise uses this checkout default) |
| Test | `MULTICA_E2E_CLI_BINARY` | Executable current-source CLI under **this checkout's** `server/bin/` |
| API | `MULTICA_MCP_TEMPLATE_DIR` | Existing empty task-owned absolute catalog directory |
| API | `MULTICA_MCP_TEMPLATE_ALLOW_HTTP` | `true` for the HTTP catalog recipe fixture |
| Test | `E2E_MCP_TEMPLATE_DIR` | Exactly the same catalog directory |
| API | `CHANGELOG_FILE` | Owned `<checkout>/.omx/state/desktop-changelog/live/<run>.json` |
| Test | `CHANGELOG_E2E_FEED` | Exactly the same feed path |

Build the CLI with `go -C server build -o bin/multica ./cmd/multica`, from the target checkout. The CLI tests reject paths outside its `server/bin/`; an installed CLI is not a substitute.

Initialize the owned feed with the current `server/internal/changelog/content/changelog.json`. The ordinary reader tests assert its existing release content, so an empty feed is insufficient. `changelog.spec.ts` refuses writes outside the exact `.omx/state/desktop-changelog/live/` prefix, publishes locally through the repository script, and restores the original bytes in `finally`. Preserve the original feed hash. MCP fixture directories and manifests are local and restored/removed by their fixture; the endpoint `https://mcp.example.test` and command `mcp-e2e-never-executed` are saved configuration, not executed transports.

Before baseline entry, verify the task database's `collaboration_execution_gate` has `creation_enabled=true`, `enabled=false`, `host_execution_enabled=false`. Preserve gate epoch monotonicity using the existing helper/transaction pattern; do not reset epochs. The newly enabled tests seed offline synthetic enrollments, authorize resources and inspect queues; they do not need a daemon, commercial model or managed-installation API mode.

| Added prerequisite | Previously skipped cases | Exact files |
| --- | ---: | --- |
| Deployment UUID, API/native bindings | 15 | `collaboration-account-desktop.spec.ts` (2), `collaboration-preparation-records-desktop.spec.ts` (1), `collaboration-mention-journeys-desktop.spec.ts` (3), `collaboration-boundaries-web.spec.ts` (1), `collaboration-command-history-web.spec.ts` (1), `collaboration-mention-journeys-web.spec.ts` (3), `collaboration-preparation-records-web.spec.ts` (1), `collaboration-preview-web.spec.ts` (1), `collaboration-resource-offers-web.spec.ts` (1), `collaboration-resources-web.spec.ts` (1) |
| Current built CLI | 6 | `cli-forbidden-reason.spec.ts` (3), `upstream-backports-cli.spec.ts` (3) |
| Changelog native bindings | 4 | `desktop-settings.spec.ts` (1), `retained-languages-desktop.spec.ts` (2), `quick-create-actor-picker.spec.ts` (1) |
| Native bindings + owned live feed | 2 | `changelog-desktop.spec.ts` (1), `changelog.spec.ts` (live publication case, 1) |
| Owned MCP catalog | 1 | `mcp-market.spec.ts` (deployment catalog case, 1) |

Full baseline command, after verified startup and environment injection:

```sh
pnpm exec playwright test --workers=1 --retries=0 --reporter=line,json --trace=retain-on-failure --output=<new-owned-artifacts-directory>
```

Set `PLAYWRIGHT_JSON_OUTPUT_FILE` to a new owned result path. This command also retains the still-unconfigured password/native-runtime skips; do not claim they are covered by this manifest. API catalog settings must not leak into standalone Go test processes.

## Same legacy API, separately prepared provenance: 2 identities

`e2e/collaboration-detail-journey.spec.ts` contains production Web and Electron manual-detail cases. They can run in the same legacy/manual-only environment once final build/listener identities are known, preferably as a separately recorded two-case run after the broad baseline.

Required test variables:

- `COLLABORATION_DETAIL_TASK=.trellis/tasks/11-08-core-business-regression`
- `COLLABORATION_DETAIL_ALLOCATION_FILE=<absolute current allocation JSON>`
- `COLLABORATION_DETAIL_PROVENANCE_FILE=<absolute current provenance JSON>`

The allocation must contain `task`, `source_root`, `api_origin`, `web_origin`, `renderer_origin`, `database`. The provenance must bind `runtime_permitted=true`, task/source identity, allocation path and SHA-256, actual origins/database, API/Web/renderer listener PIDs, commit/source IDs and launch commands. It also requires production Web Build ID, renderer/preload hashes and the five product source hashes enumerated in `e2e/fixtures/collaboration-detail-journey.ts:160`. API `/health`, `/api/config`, actual served renderer bytes, rendered Web API/WS origins, current HEAD and manual-only DB gates are verified before fixture login. Do not reuse old source hashes or old permits.

```sh
pnpm exec playwright test e2e/collaboration-detail-journey.spec.ts --workers=1 --retries=0 --reporter=line,json --trace=retain-on-failure --output=<new-owned-detail-artifacts>
```

The prior builder `.omx/private/core-e2e-20261104/runtime/detail-proposal/build-current-provenance.py` is a reference for the record shape, not authorization to execute with its historical bindings.

## Separate API variants: 12 previously skipped identities

Use separate fresh migrated task databases or the coordinated lane allocation already prepared by root. Retain fixed frontend API/WS origins if switching only the owned API behind them. Verify registry `source_env`, selected DB and live API identity after every switch. Device authentication changes anonymous navigation; each provider changes the API LLM transport, so these variants should not be enabled indiscriminately for the broad baseline.

| Variant | API changes from baseline | Test additions | Helper and exact complete spec |
| --- | --- | --- | --- |
| Device | `MULTICA_DEVICE_AUTH_ENABLED=true`; clear `MULTICA_DEVICE_AUTH_WORKSPACE`, `_WORKSPACE_NAME`, `_ROLE`; keep legacy email verification available | `E2E_DEVICE_AUTH=1` | No helper. `e2e/auth-device.spec.ts`: **1 case** |
| Assist | Device disabled; `MULTICA_LLM_BASE_URL=http://127.0.0.1:<assist-port>/v1`, `MULTICA_LLM_API_KEY=e2e-local-provider`, `MULTICA_LLM_DEFAULT_MODEL=controlled-e2e-fixture` | `E2E_ASSIST_PROVIDER_URL=http://127.0.0.1:<assist-port>` | `E2E_ASSIST_PROVIDER_PORT=<assist-port> node e2e/support/assist-provider.mjs`; `e2e/issue-assist-flow.spec.ts`: **8 cases** |
| Recommendation | Device disabled; LLM base URL points to recommendation helper `/v1`, same synthetic key/model | `ACTOR_RECOMMENDATION_PROVIDER_URL=http://127.0.0.1:<recommend-port>` plus Changelog native variables above | `ACTOR_RECOMMENDATION_PORT=<recommend-port> node e2e/fixtures/creator-recommendation-provider.mjs`; `e2e/quick-create-actor-picker-phase2.spec.ts`: **4 total cases, 3 previously skipped** |

Run each exact spec with the common one-worker/zero-retry/reporting arguments. Device, Assist and Recommendation together execute 13 cases and close 12 old skip identities; the recommendation default-precedence case already ran in the baseline. Count it once in final coverage.

Choose unused task-owned helper ports; wait for `/health`, retain helper call counts and stop only owned helper processes. Assist exposes `/requests`; recommendation `/health` returns `calls`. Keep provider protocols separate: the recommendation helper is not an Assist replacement. Never fall back to inherited LLM endpoint/key. These lanes prove real frontend/API/database transport with deterministic local model responses, not commercial-model reasoning or a real agent subprocess.

## Evidence and limits

- Reconcile **28 baseline additions + 2 provenance cases + 1 Device + 8 Assist + 3 Recommendation = 42 old skip identities** using exact spec/title and new JSON output.
- Record first failures and later new runs separately; unchanged skips and empty result files are not passes.
- Read-only source preparation has not proved the new environment or any new case. The previous 441-case coverage file binds the older `1a32` source and is only an inventory reference.
- The old `legacy-lanes.py` pins its source, candidate, allocation, old database ledger and helper hashes. Do not execute it unmodified against this new checkout.
- Password/admin, managed execution, Host/Memory/Knowledge, namespace and installed Main remain separate mandatory groups in the broader acceptance plan. This manifest does not narrow E01–E17 to legacy only.
