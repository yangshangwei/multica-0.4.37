# Research: MCP business semantics and welcome verification

- Query: Identify the truthful MCP empty gate, existing welcome conventions, and the narrowest current-code browser verification path.
- Scope: internal
- Date: 2026-11-21

## Findings

### Business owner and safe gate

- `packages/views/mcp/mcp-page.tsx:5` only selects `<McpTab presentation="page" />`. The shared workspace controller in `packages/views/settings/components/mcp-tab.tsx:92` owns both Settings and standalone presentation. Add the welcome within that controller's page branch; do not duplicate queries/mutations in `McpPage`.
- `mcp-tab.tsx:100-111` resolves the authenticated user's workspace membership before enabling the inventory read. Only owner/admin can manage. Members can browse the capability market and inspect nonsecret summaries (`mcp-tab.tsx:271-282`, `432-436`). An empty introduction must preserve these boundaries and must not expose a create action to a plain member.
- `mcp-tab.tsx:305-313` already protects unknown inventory and missing membership. `serversQuery.data ?? []` at line 116 is a rendering convenience, not evidence of empty inventory. Require the raw query result to be defined and successful before showing the welcome.
- `packages/views/settings/components/settings-query-feedback.tsx:13-23` treats definitive, non-stale-auth 401/403 as access loss, even when Query retains data. It also retains failed-read feedback while refetching via error/data timestamps. A cached empty array can coexist with failure: include both membership and server failure/access predicates in the welcome decision, not just array length or `hasData`.
- Preserve retained populated management during transient failures. `provider-settings-recovery.test.tsx:277-300` canonically verifies that an MCP rename field, value and focus survive a server refresh failure, but disappear after confirmed workspace access loss.
- Saved templates are not saved workspace instances. The management view must not mount a catalog observer or automatically navigate to the market (`.trellis/spec/views/frontend/mcp-market.md:223-225`). Existing `mcp-tab.test.tsx:619-629` verifies no template-query calls and explicit market navigation.
- Creation is configuration persistence, not runtime connectivity or tool execution. The market spec distinguishes template recipe availability from connectivity (`mcp-market.md:19-22,35-39`), and summaries intentionally exclude URLs, commands, args, env and headers (`mcp-market.md:46-54`). Static scenes must not imply an automatic grant or validated live connection.

### Incumbent visual and interaction conventions

- `skills/components/skill-welcome.tsx` and `agents/components/agent-welcome.tsx` use a left introduction/action block and three manually selected illustrative scenes. `skill-welcome.module.css:1-17,234-250` makes the welcome its vertical scroll owner and uses a 60rem container breakpoint, accounting for desktop panes and sidebars rather than viewport width.
- `skill-welcome.module.css:29-58` clips a moving grid track into one active scene, with rounded semantic surfaces and existing shadows. Preserve incumbent styling rather than inventing a new design system or global welcome abstraction.
- `.trellis/spec/views/frontend/skill-welcome.md:19-34,65-78` defines successful-empty/error precedence, no completion flag, workspace-keyed scene reset, illustrative-only examples, hidden+inert inactive scenes, pressed position controls, disabled endpoints, polite announcements, reduced-motion support and no autoplay/dependencies.
- `McpServerDialog` already accepts a final-focus resolver (see its existing props/use); verify its exact current contract before hooking a disappearing welcome create trigger. Do not reconstruct the editor to add onboarding.
- Keep the Settings-tab empty message unchanged if the scope is standalone MCP only. Existing `mcp-tab.test.tsx:619-629` currently combines page/settings expectations and should be split: page expects the new welcome; settings retains the old management empty state.

### Canonical unit and integration coverage

- `packages/views/settings/components/mcp-tab.test.tsx:27-41` mocks Query with only `data` and `isLoading`. A new `isSuccess` gate requires truthful mock state extension. Do not weaken the production predicate to satisfy the old mock.
- That suite's path mock (`:55-60`) only provides `marketplaceTab` and navigation `push`. Adding a real docs `AppLink` requires appropriate path and navigation mocks or a focused separate welcome test harness.
- Use dedicated welcome component tests for bilingual content, keyboard/manual scene selection, hidden/inert scenes, CTA callbacks and docs target. Keep actual form/rename/token-secrecy matrices in their existing suites.
- Prefer real `QueryClient` tests for cached-empty refresh error, permission loss, empty-to-populated transition and retry. `provider-settings-recovery.test.tsx:113-128` has a usable wrapper and full navigation/path mock; avoid re-running every existing settings recovery matrix.

### Narrowest reliable visual verification

1. Use the existing production-Web Playwright route/session pattern, not an isolated mock page. `playwright.config.ts:4-20` starts no servers and selects `PLAYWRIGHT_BASE_URL`, then `FRONTEND_ORIGIN`, then localhost:3000. `.trellis/spec/web/frontend/e2e-run-environment.md:10-23` requires a production build/server and documents `make up C=api,web ARGS="--web-mode production"`, `make status ARGS="--json"`, and `make env-exec ARGS="-- pnpm exec playwright test --workers=1 --retries=0"`.
2. A dedicated `e2e/mcp-welcome.spec.ts` can follow `e2e/skill-welcome.spec.ts:30-63`: unique account+workspace, mark onboarded, English profile/cookie, token init script, closed chat, explicit theme, navigate to `/{slug}/mcp`. Assert the requested unique slug because `TestApiClient.ensureWorkspace()` can return an existing first workspace (`e2e/fixtures.ts:192-220`).
3. Use the same fresh workspace without seeding agents or skills to capture actual Skill/Agent welcome references. The existing MCP market setup creates two agents (`e2e/mcp-market.spec.ts:107-112`), so its `reference-agents` screenshot (`:149-152`) is a populated list, not the desired welcome reference.
4. Reuse the exact locale/theme/width loop and overflow geometry from `e2e/skill-welcome.spec.ts:95-130`: English and Simplified Chinese, light/dark, wide 1440 and narrow 390/320; wait `document.fonts.ready`; explicitly scroll the welcome/container to reveal narrow example controls. Use reduced motion or wait for stable animation completion before capture. Screenshot plus actual document/content-width metrics is stronger than screenshot alone.
5. Cover explicit market navigation, custom editor open/cancel/focus, no writes from illustrative scenes, docs, and a real MCP create/delete transition using a harmless saved configuration. No daemon/provider needs to start for this acceptance.
6. For native parity, `e2e/skill-welcome-desktop.spec.ts:1-40` uses `e2e/fixtures/project-p1-desktop.ts`: it serves current built renderer/preload files through a task-local reverse proxy, launches Electron with a temporary profile, and cleans both. `:13-15` supports `MULTICA_E2E_DESKTOP_OUTPUT_DIR` and requires explicit API env. Its narrow-pane technique is `setContentSize(720,700)`, then scroll the actual welcome owner; use `nativeScreenshot` for compositor output.

### Existing acceptance selectors affected

- `e2e/mcp-market.spec.ts:154-158` explicitly expects both Add custom server and Browse capability market in the standalone header for an empty workspace. If welcome actions replace duplicate header controls, update these selectors to the visible welcome action while preserving their navigation assertions.
- `e2e/mcp-desktop.spec.ts:169-174` uses the MCP sidebar route, expects one exact MCP heading and a visible browse-market control; check for duplicate exact button names if adding the hero CTA while retaining header actions.

## Files found

- `packages/views/settings/components/mcp-tab.tsx` — shared controller, permissions, feedback, saved inventory, dialogs.
- `packages/views/settings/components/settings-query-feedback.tsx` — cached-state/error and access-loss contract.
- `packages/views/settings/components/mcp-tab.test.tsx` — management semantics and current minimal Query mocks.
- `packages/views/settings/components/provider-settings-recovery.test.tsx` — real-Query MCP focus/access recovery.
- `packages/views/skills/components/skill-welcome.tsx` and `.module.css` — requested visual reference.
- `packages/views/agents/components/agent-welcome.tsx` — requested visual reference.
- `e2e/skill-welcome.spec.ts`, `e2e/agent-welcome.spec.ts` — real-Web welcome snapshots and state transitions.
- `e2e/skill-welcome-desktop.spec.ts`, `e2e/fixtures/project-p1-desktop.ts` — narrow actual-Electron welcome verification.
- `e2e/mcp-market.spec.ts`, `e2e/mcp-desktop.spec.ts` — existing MCP journeys affected by action placement.
- `playwright.config.ts`, `e2e/env.ts`, `e2e/fixtures.ts` — production baseURL, env selection and fixture data ownership.

## Related specs

- `.trellis/spec/views/frontend/mcp-market.md`
- `.trellis/spec/views/frontend/skill-welcome.md`
- `.trellis/spec/views/frontend/settings-recovery.md`
- `.trellis/spec/views/frontend/capability-market.md`
- `.trellis/spec/web/frontend/e2e-run-environment.md`

## External references

No external research was needed. Findings are tied to this checkout's code and authored contracts, not claims about current public MCP implementations.

## Caveats / Not Found

- This research did not launch/stop services, run browser tests, inspect live process ownership, or establish that any previously built asset contains the forthcoming change.
- Existing `.env` values and historical screenshot artifacts are not proof of a safe/current test environment. Verify allocated server/build provenance before running the focused suite; do not mutate or stop unrelated environments.
- `e2e/fixtures/project-p1.ts:21-31` and the Skills/Agent welcome specs use legacy email-code login plus direct database setup. Password-mode reuse needs the explicit branch from `e2e/mcp-market.spec.ts:86-103`; `markUserOnboarded()` requires an email session and cannot be reused after password login.
- Any malformed-list fallback inherited from the API parser cannot establish historical first use. This feature can truthfully describe a currently empty saved inventory, not guarantee the workspace has never had MCP before.

## Focused production runner and output conflicts

### Read-only observation at research completion

- The parent reports registry environment `check-20261119151016-76544` as mismatched for this checkout; do not reuse, stop or repair it for this task.
- A direct listener inspection found Node PID 30494 listening on 13493 and server PID 41930 listening on 18573. No `.multica/web-build.lock.d/pid` or `apps/web/.next/lock` existed at inspection. Existing `.next/BUILD_ID` and Desktop renderer/preload files existed but predated this work; they do not prove the forthcoming source is built.
- `dev-env.sh:1373-1388` reuses the first registry manifest for the checkout even if `up --name` supplies a different name. `cmd_up` also invokes global auto-GC (`:1330`). Therefore do not use a nominally new `make up --name ...` to isolate this task.
- `dev-env.sh:727-743,793-800` refuses to rebuild the shared `.next` while another registered environment for that source directory has either a Web launcher or a listener on its Web port. A mismatched listener still blocks this guard. This is intentional: a second port does not isolate build output. Do not bypass it, remove the old record, override process ownership, or kill the unknown listener.

### Existing safe allocator and functions

`scripts/check.sh:223` guards `check_main` with `BASH_SOURCE[0] == $0`, so sourcing it loads functions without running checks or starting processes. `prepare_check_environment` (`:61-135`) allocates a fresh port slot under the shared lock and registers a unique `check-*` environment regardless of existing checkout records. It copies the source env to an owner-only task env, generates an isolated database name/JWT key, forces legacy test auth only in that copy, sets production Web mode and binds API/Web/renderer origins. It does not invoke the `up` auto-GC path.

When the build-output guard permits it, the following task-local runner body uses those existing functions without broad static/Go gates:

```bash
#!/usr/bin/env bash
set -euo pipefail
# Run from the source root whose files and build outputs are owned by this QA run.
. ./scripts/check.sh
qa_source_env="$(env_file_path "${ENV_FILE:-$(detect_env_file)}")"
[ -f "$qa_source_env" ] || die "Missing QA source env"
load_env_file "$qa_source_env"
prepare_check_environment "$qa_source_env"
trap check_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"
printf 'QA environment: %s; evidence: %s\n' "$NAME" "$STATE_DIR"
ensure_database
migrate_database
CHECK_STARTED_API=true
start_api
# Wait until implementation writes are finished before these production builds.
# For a Web-only pass the next three Desktop lines can be omitted.
pnpm --filter @multica/desktop build
CHECK_STARTED_RENDERER=true
start_check_desktop_renderer
CHECK_STARTED_WEB=true
start_web
print_status_json > "$STATE_DIR/verification.running.json"
pnpm exec playwright test e2e/mcp-welcome.spec.ts --workers=1 --retries=0
# If authored, add the focused native welcome spec in this same owned allocation.
CHECK_COMPLETE=true
```

`start_api` builds the actual current Go sources, stamps the commit and checks PID/source/config ownership (`dev-env.sh:569-610`). `start_web` builds current Web sources via its normal command, checks that source/config hashes did not change during compilation, records BUILD_ID, and starts production `next start` (`:762-803`). `prepare_check_environment` exports `MULTICA_E2E_DESKTOP_OUTPUT_DIR=$REPO_ROOT/apps/desktop/out` and exact native origins (`check.sh:112-117`). The cleanup function stops only the component flags recorded by this run (`check.sh:39-59`); it retains the task API database/evidence for inspection. Never print the copied env or JWT.

### Required fallback while this checkout has a foreign build consumer

Use a private detached Git worktree or source snapshot so the QA run owns a different `apps/web/.next` and Desktop `out` directory. This is an execution option for the main session, not a step performed by research. A detached worktree keeps ordinary Git-based source identity checks working (`checkout_source_id` requires `git diff`, tracked HEAD and untracked source enumeration at `dev-env.sh:621-653`). A plain copied directory without Git metadata is not sufficient for the existing start functions.

Suggested preparation, after implementation completes:

```bash
# Choose a new private directory; retain its path and current patch/hash evidence.
git worktree add --detach "$qa_checkout" HEAD
# Overlay the current source patch (including pre-existing product edits needed
# to identify this exact checkout); never include private env files or build output.
git diff --binary HEAD -- apps packages server scripts e2e | git -C "$qa_checkout" apply --binary
```

Also copy newly untracked source files reported by `git ls-files --others --exclude-standard -z -- apps packages server scripts e2e`, preserving their relative paths; this includes the new welcome module/CSS/tests. Copy the new focused E2E file as well. Do not point the private worktree's entire app/package `node_modules` directory at the original one: its workspace-package symlinks can resolve back to original source, defeating source provenance. Install the unchanged lockfile in the private tree (`pnpm install --frozen-lockfile`, using `--offline` if the store has everything) so local workspace links resolve inside the private tree. This introduces no package dependency change; if an offline cache is incomplete, retain that result rather than claim the browser run passed.

Run the function-based runner above **from the private worktree**, with `ENV_FILE` naming the original source env only as an input. The allocator copies it before overrides and does not edit it. All compiled output and source provenance then bind to the private worktree; no existing record has that `DIR`, so the shared-build guard remains meaningful and the unrelated original listener stays untouched. Record both original and private source hashes and their copied-file mapping. Remove the private worktree only after owned components are stopped and evidence is retained; do not run global GC or destructive environment cleanup against unrelated records.

These commands have been inspected against source but were not executed during research. The exact offline package-store availability and actual production build/browser outcome remain for the main session to verify.
