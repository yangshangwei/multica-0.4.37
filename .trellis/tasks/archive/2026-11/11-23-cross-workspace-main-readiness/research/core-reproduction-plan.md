# Research: Five recorded core E2E failures

- Query: Determine current fixture contracts, prerequisites and bounded hypotheses for two localized Squad failures, S1 outsourcing, live grant revoke and terminal reopen.
- Scope: Internal source and historical task-owned evidence; no services or tests executed, no product edits or Git operations.
- Date: 2026-11-23
- Candidate supplied by coordinator: `cb192014b2f27e9bb64112db52073cbb31ce8f04`. Concurrent root changes must be excluded or explicitly rebound by the implementation owner.

## Findings

### Three independent environment lanes

| Cases | API mode | Execution prerequisites |
| --- | --- | --- |
| Two localized Squad cases | `legacy`; ordinary TestApiClient login | PostgreSQL, production Web, synthetic runtime seeded through fixture; no real Provider |
| live revoke + terminal reopen | `password`, managed installations enabled | Synthetic signed installation + MDT protocol fixture; `enabled=true`, `creation_enabled=true`, **`host_execution_enabled=false`** for the existing namespace simulation |
| S1 outsourcing | `password`, managed installations enabled, knowledge context enabled | Actual task-built daemon test binary, CLI and deterministic fixture Provider; host gate opened/restored by `openHostAcceptanceGate()` |

Use distinct fresh owned databases/configurations or explicitly restore the gate booleans between lanes. Do not place synthetic namespace tests inside the host-enabled S1 allocation without changing the gate. Do not conflate a synthetic permit with a Provider process or termination proof.

Common production bindings:

- `DATABASE_URL`: an explicit loopback disposable database, never `multica`, `postgres`, `template0` or `template1`.
- `NEXT_PUBLIC_API_URL` and `COLLABORATION_E2E_API_URL`: same owned API origin.
- `PLAYWRIGHT_BASE_URL` and `FRONTEND_ORIGIN`: owned production Web origin; keep the hostname identical to the API (historical harness deliberately used `localhost` for both).
- `REMOTE_API_URL`: owned API for Next runtime proxy; `NEXT_PUBLIC_WS_URL` and native WS origin point to that API's WS listener.
- `CORS_ALLOWED_ORIGINS` and `ALLOWED_ORIGINS`: actual allocated Web/renderer origins.
- Task-only `JWT_SECRET`, `MULTICA_DEPLOYMENT_ID`, `LOCAL_UPLOAD_DIR`; `MULTICA_CLOUD_URL` empty and `MULTICA_DEVICE_AUTH_ENABLED=false`.
- Password lanes: `MULTICA_AUTH_MODE=password`, `MULTICA_PASSWORD_LIMITER_MODE=single`, `MULTICA_MANAGED_INSTALLATIONS_ENABLED=true`, `E2E_PASSWORD_AUTH=1`. Verify `/api/config` actually exposes password, managed support and the chosen deployment ID.
- Localized lane: `MULTICA_AUTH_MODE=legacy`, managed installation/platform-admin disabled, optional task-only `MULTICA_DEV_VERIFICATION_CODE`; TestApiClient obtains its code from the selected PostgreSQL database. No password opt-in.
- Private `HOME`/temp/uploads; explicitly bind existing verified `PLAYWRIGHT_BROWSERS_PATH` when using private HOME.

The default Playwright config starts no services (`playwright.config.ts:20`). `e2e/env.ts` loads `.env.worktree` then `.env` if present. Keep unrelated environment files out of the candidate and pass the owned environment explicitly.

### Exact build and run entry points

From a private candidate checkout, with an absolute task-owned `NEW_BIN` outside source:

```sh
go -C server build -p 2 -o "$NEW_BIN/server" ./cmd/server
go -C server build -p 2 -o "$NEW_BIN/migrate" ./cmd/migrate
go -C server test -p 2 -c -o "$NEW_BIN/daemon-test" ./internal/daemon
go -C server build -p 2 -o "$NEW_BIN/multica" ./cmd/multica
go -C server build -p 2 -o "$NEW_BIN/host-provider" "$NEW_SOURCE/e2e/fixtures/collaboration-host-provider.go"
pnpm --filter @multica/web build
pnpm --filter @multica/desktop build
```

The daemon build has **no extra build tag** and `go test -c` does not execute package tests. The source and build provenance still need the coordinator's normal CLI guard/config. Bind server build commit metadata using the repository's build convention if checking `/health.commit`.

Historical exact host build reference: `artifacts/qa/full-e2e-20261120-r3/conditional/build-host-fixtures.sh:8–12`. Do not source or execute that old script because it loads the old environment manifest.

S1 variables additionally required by `e2e/fixtures/collaboration-host-runtime.ts:104–118`:

```text
COLLABORATION_HOST_DAEMON=<absolute task-built daemon-test>
COLLABORATION_HOST_PROVIDER=<absolute task-built host-provider>
COLLABORATION_HOST_CLI=<absolute task-built multica>
COLLABORATION_ELECTRON_RENDERER_URL=<owned production renderer origin>
COLLABORATION_ELECTRON_SOURCE_ROOT=<candidate checkout>
QA_KNOWLEDGE_CONTEXT=on
MULTICA_KNOWLEDGE_CONTEXT_ENABLED=true
```

Even Web-only S1 requires the renderer variable through `hostEnvironment()`. Provider probe `--fixture-kind` returns `multica-host-e2e-provider-v1`; fixture uses `__multica_collaboration_host_router_poller` entry of daemon-test (`collaboration-host-runtime.ts:349`). It copies explicit Provider/CLI binaries into private resources and never resolves a user's installed CLI.

Owned startup pattern from the historical harness is server binary in `server/`, Next `node node_modules/next/dist/bin/next start --port <web> --hostname 127.0.0.1` from `apps/web`, and `node scripts/check-desktop-renderer.mjs <absolute out/renderer> <port> <receipt.json>`. Validate live listeners and `/api/config`, not merely existing process records.

Original focused commands after targets are ready:

```sh
pnpm exec playwright test e2e/localized-template-defaults.spec.ts --grep 'Chinese squad staffing|a localized name collision' --workers=1 --retries=0
pnpm exec playwright test e2e/collaboration-live-revoke-web.spec.ts e2e/collaboration-terminal-reopen-web.spec.ts --workers=1 --retries=0
pnpm exec playwright test e2e/collaboration-business-scenarios.spec.ts --grep '^S1 outsourcing:' --workers=1 --retries=0
```

S1 timeout is 720 seconds; the two controls each allow 300 seconds. Capture JSON reporter, original failures and source/build identity separately per attempt. Do not extend deadlines to mask failures.

### Localized Squad: current locator contradicts the intended empty welcome

- `e2e/localized-template-defaults.spec.ts:75–96` opens `/<workspace>/squads`, waits for a locale-specific catalog response and clicks the **header-scoped** New Squad link at line 82.
- `packages/views/squads/components/squads-page.tsx:943–965` intentionally omits header actions when `showWelcome` and renders `SquadWelcome`.
- `packages/views/squads/components/squad-welcome.tsx:199–202` supplies the same localized creation link outside the header, routed to `paths.newSquad()`.
- Thus the empty synthetic workspace's header link cannot appear. Current-run reproduction should preserve creation/reuse/collision assertions and select the actual entry by its intended destination or unique accessible name, rather than restore a duplicate product header button.
- `useSquadTemplates` caches per locale for 30 minutes (`packages/views/agents/create/use-role-templates.ts:29–38`); do not require an extra request when a catalog is legitimately cached. The page navigation in the current helper creates a fresh document, but future SPA-only repairs must retain this distinction.
- Original private log **exists**: `.omx/private/qa-work/core-20261121.kV2VvV/agent-squad-ui-isolated/playwright.log:12–82`. Both failures show the header locator timeout and then the unresolved catalog promise. The public report directory did not carry these logs; missing public copies do not mean all raw logs were lost.

### S1: duplicate discovery and attention buttons

- `e2e/fixtures/collaboration-disclosures.ts:15–19` selects `Issue resources`, then a name regex matching either plain `Knowledge candidates` or its count.
- `packages/views/collaboration/detail-auxiliary.tsx:75–79` renders a primary button with `data-panel`, `aria-haspopup="dialog"`, and optional count.
- The same group renders an attention link-button with plain `Knowledge candidates` at line 81, so both match when knowledge needs attention.
- This explains the retained strict-mode failure without proving a product bug. A fixture repair should identify the primary discovery control while preserving keyboard/accessibility behavior and original business assertions. Do not add `.first()` solely to suppress ambiguity.
- Original private log is `.omx/private/qa-work/core-20261121.kV2VvV/host-s1-retry/playwright.log`; original report `artifacts/qa/core-business-e2e-20261121-kV2VvV/REPORT.md:27` records the strict-match condition.

### Synthetic control 409: host gate conflicts with namespace simulation

- Both specs call `seedCollaborationControlJourney` and `simulatedDaemon.permitPendingInput` (`collaboration-live-revoke-web.spec.ts:24–31`, `collaboration-terminal-reopen-web.spec.ts:26–34`).
- `e2e/fixtures/collaboration-control-journey.ts:61–95` registers a synthetic `codex` runtime, deregisters it, then re-registers and directly lists runs. It sends protocol version 2 / `linux_pid_namespace_v1` claim and permit; it does **not** register a host capabilities manifest.
- `server/internal/service/collaboration_runtime_preparation.go:180–187` rejects newly promoted groups with `execution_mode_unsupported` when `HostExecutionEnabled` is true and registered capabilities are absent or not host supervision. Namespace support is retained only for draining pre-cutover groups.
- Historical `artifacts/qa/core-business-e2e-20261121-kV2VvV/handoff-gate-enabled/cleanup.json` actually records the browser-start gate as `enabled=true, creation_enabled=true, host_execution_enabled=true`, so that allocation contradicts the synthetic fixture. Its cleanup restores false values; `gate_booleans_restored=false` is reported honestly.
- That is a strong, source-backed configuration hypothesis for the 409; historical logs only show status, so reproduce with a safe response-code observation before calling the cause proven. Private `.../handoff-gate-enabled/playwright.log:9,40` and `api.log:1097,1245` retain both GET runs 409s.
- `ListRuntimeRuns` invokes preparation during promotion (`server/internal/service/collaboration_runtime.go:626–668`, `:793`); the failure occurs before the claim body is sent, so changing its version alone cannot repair this observed stage.
- Controls should run with execution/creation enabled and host disabled, then restore original booleans while keeping epoch monotonic. If testing host cutover semantics instead, use the real host fixture as a separate case, not invented capabilities or a relaxed server gate.

## Files found

- `e2e/localized-template-defaults.spec.ts`: original locale/reuse/collision scenarios.
- `e2e/collaboration-business-scenarios.spec.ts`: original S1 with full business/knowledge closure.
- `e2e/collaboration-live-revoke-web.spec.ts`, `e2e/collaboration-terminal-reopen-web.spec.ts`: synthetic public permit tests with deliberate missing termination proof.
- `e2e/fixtures/collaboration-control-journey.ts`: signed synthetic installation and namespace protocol.
- `e2e/fixtures/collaboration-host-runtime.ts`: explicit binaries, host gate, actual poller and deterministic Provider contract.
- `.omx/private/qa-work/core-20261121.kV2VvV/run-qa.py`: historical launch source, useful as reference only; its hardcoded paths/owner/database must not be reused.

## Related specs and external references

- `.trellis/spec/web/frontend/e2e-run-environment.md`: production targets, owned origins, password/managed startup, private browser cache, business-scenario gate restoration.
- `.trellis/tasks/11-23-cross-workspace-main-readiness/{prd,design,implement}.md`: current frozen-candidate acceptance and preservation constraints.
- No external research needed; versions and commands derive from repository sources.

## Caveats / Not Found

- No current tests ran during this research. Locator/configuration findings require original-case reproduction before fixes are credited.
- Only safe code/error/status fields from historical evidence were examined; credentials, bodies and private account state were not copied into this report.
- Exact later fix commits were not queried because this research role prohibits Git operations. Root must compare supplied candidate versus newer concurrent commits before execution/final verdict.
- Root has assigned live reproduction and any fixture edits to `core_replay`; researcher owns no implementation files or runtime resources.
