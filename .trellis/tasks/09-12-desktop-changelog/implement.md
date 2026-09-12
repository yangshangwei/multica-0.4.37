# Desktop changelog implementation plan

**Goal:** Ship the Help reader and commit-driven, intranet-ready release feed described in [prd.md](./prd.md).

**Architecture:** A same-server authenticated endpoint serves a hot-read deployment file with an embedded baseline. Shared Query/view packages render history in desktop and web. A Node generator produces one cumulative artifact used by builds, the fork Release, and the atomic intranet publisher.

**Tech stack:** Existing Go/Chi, Node 22 standard library, React/TanStack Query/Zod, shared app components, Vitest, Playwright, GitHub Actions. No new dependencies.

This is the bounded Trellis execution plan. The leader owns integration and verification; implementation owners execute their assigned slice directly without recursively delegating. Design/API contracts are in [design.md](./design.md). The user has already requested implementation and acceptance.

## Ownership and dependency graph

| Lane | Owner responsibility | Primary write scope | Depends on |
| --- | --- | --- | --- |
| A — [Server feed task](../09-13-changelog-feed/task.json) | Validator, embedded/source reader, HTTP endpoint and tests | `server/internal/changelog/`, `server/internal/handler/changelog.go`, related server wiring | Reviewed schema; seed fixture from D |
| B — [Release pipeline task](../09-13-changelog-release/task.json) | Generator, cumulative-history/publication commands, CI and offline distribution integration/tests | `scripts/*changelog*`, offline build/bundle/upgrade scripts, helper module if needed, `.github/workflows/release.yml`, `.github/RELEASING.md` | Reviewed schema and base/history policy |
| C — [Reader task](../09-13-changelog-ui/task.json) | Core schema/query, shared view/locales, Help and platform wiring/tests | `packages/core/changelog/`, core API/paths exports, `packages/views/changelog/`, Help/locales, desktop/web route/notification files | Reviewed API/schema; can use fixtures before A finishes |
| D — Integration | Seed, deployment docs/config, Trellis context/tasks, acceptance harness and review | Seed content, deployment settings/docs, `e2e/changelog.spec.ts`, Trellis evidence/spec notes | A/B/C for end-to-end verification |

Agree exact shared filenames before writes. A owns server wiring (`server/cmd/server/main.go`, `router.go`, `server/internal/handler/handler.go`); D asks A to add `CHANGELOG_FILE` configuration rather than editing those files concurrently. B owns release documentation; D owns self-hosting/environment documentation. C owns `package.json` exports inside core/views; D may add root script aliases only after B chooses final CLI names. No lane edits another lane's files without a handoff.

The initial worktree had unrelated dirty changes in config, docs, integrations, and templates; consult `research/initial-worktree-status.txt`. Preserve all of them and re-read shared files before a narrow patch. Never stage/reset/revert the entire tree. Planning does not create commits or tags.

## T0. Lock the contract and prepare Trellis

Owner: D. Depends on: planning review.

1. Review PRD/design/this plan against the research artifacts, including runtime fields `server_version`, `feed_source`, `is_stale`, `warning`.
2. Populate `implement.jsonl` and `check.jsonl` with relevant spec indexes and the three research files; no code-file manifest entries.
3. Add lane tasks/checklist and affected files to `task.json`, then run `python3 .trellis/scripts/task.py start .trellis/tasks/09-12-desktop-changelog` using the active Trellis session identity.
4. Freeze an inspected target commit and local v0.4.37 base for preview generation. Do not read a moving HEAD repeatedly during generation.
5. Establish shared fixtures: valid seed, updated feed, invalid/oversized feed, empty feed, unknown client enum. Fixture metadata must make synthetic releases unmistakable.

Exit evidence: reviewed artifacts, real context manifests, task `in_progress`, frozen commit IDs and preserved dirty-path inventory.

## T1. Validate and serve a refreshable feed

Owner: A. Depends on: T0 schema; D provides seed without blocking reader tests.

Create:

- `server/internal/changelog/feed.go` (types/validation/resource/instance reader; split only if useful).
- `server/internal/changelog/feed_test.go`.
- `server/internal/handler/changelog.go` and `changelog_test.go`.

Modify narrowly:

- `server/internal/handler/handler.go` — instance/config wiring.
- `server/cmd/server/main.go` — read `CHANGELOG_FILE` into config.
- `server/cmd/server/router.go` — authenticated `/api/changelog` route.

Steps:

1. Write failing canonical reader tests for embedded default, configured valid file, replacement, invalid/missing/oversized file, prior valid retention, startup fallback with stale warning, duplicate IDs, mixed-fork history rejection, and instance isolation. Add controlled concurrent reads/replacements and verify the reader with `go test -race ./internal/changelog`.
2. Implement the bounded parser, `go:embed` baseline, request-time read and synchronized last-valid state. Include stale/runtime metadata before deriving ETag.
3. Write transport tests for authentication, JSON/cache headers, weak/list/wildcard ETag matching, 304 unchanged, 200 after replacement and after stale-status change. Prefer existing test helpers; no database is needed for pure reader tests.
4. Wire the route/config. Reuse existing transport helpers without broad refactoring.
5. Run `gofmt` on modified Go files. From `server/`, run `go test ./internal/changelog ./internal/handler -run 'Changelog' -count=1` and `go vet ./internal/changelog ./internal/handler`. Expand auth/router checks where their canonical tests live.

Exit evidence: tests prove an unchanged service instance serves new content and retains explicitly stale content on failure; API contract agrees with C.

## T2. Generate deterministic cumulative notes

Owner: B. Depends on: T0 contract and frozen metadata.

Create:

- `scripts/generate-changelog.mjs` and `scripts/generate-changelog.test.mjs`.
- Shared pure validation/formatting module under `scripts/` only if generator and publisher need it.

Steps:

1. Build temporary-git tests before implementation: initial snapshot, unrelated higher tag, annotated target, merged feature branch, dirty checkout preview, release cleanliness guard, Unicode and shell-like text, scoped housekeeping, legacy subjects and trailer overrides.
2. Implement CLI argument validation and immutable ref resolution. Use `spawnSync`/`execFileSync` argv arrays; never shell-interpolate commit messages.
3. Add history tests: required history, preserve upstream and all published/prerelease entries, first explicit base, nearest ancestral publication, incomparable bases, duplicate/conflicting IDs, mixed-fork or wrong-repository rejection, same-version different commit, preview replacement and idempotent rerun. Exercise first publication → prerelease → stable → retry of the older version without dropping later history.
4. Produce JSON and Markdown from the same categorized record; use stable explicit timestamps and stable ordering. Validate before writing either artifact.
5. Run `node --test scripts/generate-changelog.test.mjs` and a preview dry run against the frozen checkout commit. Outputs go to a task evidence/temp directory, not a fake published release. Show representative input commits in the resulting notes.

Exit evidence: deterministic bytes, traceable committed-only records, meaningful initial fork preview and no history loss.

## T3. Publish the exact artifact safely

Owner: B. Depends on: T2; coordinate embedded path with A/D.

Create:

- `scripts/publish-changelog.mjs` and `scripts/publish-changelog.test.mjs`.
- A small reusable release-history retrieval/publish helper and fixture test if needed to test GitHub API/`gh` behavior independently of YAML.

Modify:

- `.github/workflows/release.yml`.
- `.github/RELEASING.md`.
- `scripts/offline-bundle.sh`, `scripts/build-offline-upgrade.sh`, and the corresponding `scripts/offline-installer.sh` / `scripts/offline-upgrade.sh` install path where required to carry and install the validated feed.

Steps:

1. Test and implement whole-file validation followed by same-directory temporary write, close/flush and atomic rename. A failed validation/write must preserve the destination.
2. Add a repository-wide workflow concurrency boundary before history retrieval. Retain existing security/build gates and upstream-only Homebrew/binary-publisher guards.
3. Retrieve previous published history from this fork with the Actions token, including prereleases and excluding drafts. The explicit no-release bootstrap is the only automatic seed fallback. Test auth/network/malformed/missing-asset failures.
4. Generate once before backend builds. Upload workflow artifacts and install the exact JSON into the embedded location in every consuming Go/Docker job; do not regenerate per build.
5. Add fork-safe publication after required verification/backend/web-manifest success, with exact-tag/repository checks, a draft until assets finish uploading, prerelease fidelity, `--notes-file`, and idempotent identity checks. Do not depend on skipped upstream-only release jobs.
6. Test the release helper with a fake executable/fixture HTTP service: success, failed prerequisite, missing history, retry, wrong immutable identity, prerelease and stable history. Inspect the actual workflow dependency graph too; helper tests alone do not prove YAML wiring.
7. Add the cumulative feed and publisher with all helper modules to the existing offline bundle/upgrade packaging flow; wire deployment installation to the configured directory. On targets without host Node, execute through the already-loaded frontend image with `docker run --pull never --entrypoint node` and containing-directory mounts. Preserve preview status for local unpublished builds. Test bundle contents and the upgrade handoff with fixture archives, host Node unavailable, and no network pull rather than merely documenting a manual copy.
8. Run `node --test scripts/publish-changelog.test.mjs` plus generator/helper and offline packaging tests, and available YAML/shell/static workflow validation. Document exact offline `--history` handoff and publication commands.

Exit evidence: failing prerequisites cannot announce a release; exact generated bytes reach build inputs/release assets; atomic file handoff works without runtime public access.

## T4. Parse and refresh from the configured deployment

Owner: C. Depends on: T0 contract; A endpoint can be fixture-backed initially.

Create:

- `packages/core/changelog/types.ts`, `schema.ts`, `queries.ts`, `index.ts`.
- `packages/core/changelog/schema.test.ts`, `queries.test.tsx` and pure selection helper/tests only as needed.

Modify:

- `packages/core/api/client.ts` — `getChangelog` using unknown JSON and `parseWithFallback`.
- `packages/core/package.json` — domain exports.
- `packages/core/paths/paths.ts`, `route-icons.ts`, and canonical path/tab tests; change `tab-subject.ts` only if the registry cannot already cover the route.

Steps:

1. Write node-environment parser tests for snake_case conversion, optional defaults, unknown fields/enums, malformed essential envelope, version/source selection and hostile plain text. An invalid essential payload must become a query error while valid empty history stays empty.
2. Add a real QueryClient/observer test with mocked transport that changes response while mounted, including fake timers, focus/reconnect, deployment key changes, stale response metadata, and failed-refresh retention.
3. Implement deployment-keyed query options overriding global `Infinity`/focus settings. Gate polling with actual active/visible state supplied through existing plumbing; refetch immediately when a kept-alive desktop tab becomes active.
4. Implement `paths.workspace(slug).changelog(...)`, title/icon registration and anchor/version lookup without assuming `window.location.hash` is the session URL.
5. Run `pnpm --filter @multica/core test changelog paths`, then core typecheck/lint. Keep parsing matrices here, not duplicated through every component test.

Exit evidence: fresh and retained Query observers receive the right deployment's history, bounded polling works and a bad response cannot erase successful cached data.

## T5. Build the shared reader and Help navigation

Owner: C. Depends on: T4 interfaces.

Create:

- `packages/views/changelog/changelog-page.tsx`, `index.ts`, `changelog-page.test.tsx`.
- `packages/views/locales/{en,zh-Hans,ja,ko}/changelog.json`.
- `apps/web/app/[workspaceSlug]/(dashboard)/changelog/page.tsx`.

Modify:

- `packages/views/package.json`, `locales/index.ts`, `i18n/resources-types.ts`.
- `packages/views/locales/{en,zh-Hans,ja,ko}/layout.json`.
- `packages/views/layout/help-launcher.tsx` and `help-launcher.test.tsx`.
- `packages/views/layout/route-icon-components.tsx` only if a new icon mapping is required.
- `apps/desktop/src/renderer/src/routes.tsx`.
- `apps/desktop/src/renderer/src/components/update-notification.tsx`, its tests, and `App.tsx`/`components/desktop-layout.tsx` only as needed for provider-safe navigation and event retention.

Steps:

1. Test the Help route and no-workspace state while preserving its existing grouped-version-row crash regression.
2. Build the timeline/header/month navigation using existing components and tokens; include source/status labels, correct stable/latest selection, independent installed/server versions and readable unknown-anchor fallback.
3. Add first-load, unsupported-server, empty, malformed/error and refresh-feedback states. Use accessible headings/status labels, keyboard navigation and wrapping; render release text as text.
4. Wire desktop and web wrappers, injecting installed desktop version only at the platform boundary. Register all four locale namespaces and type resources, not just JSON files.
5. Fix update-notification provider placement/callback design, retain downloaded version before shell mount, navigate internally when a workspace exists and retain restart behavior without a workspace. Test the chosen no-workspace behavior explicitly.
6. Run `pnpm --filter @multica/views test changelog layout/help-launcher.test.tsx locales/parity.test.ts`, `pnpm --filter @multica/desktop test update-notification`, and the affected app/core/view typecheck/lint commands.
7. Inspect the rendered reader against the official screenshot/reference and app tokens. For every visual iteration, write a visual verdict before the next edit to `.omx/state/desktop-changelog/ralph-progress.json`; this is evidence storage, not activation of an OMX runtime mode.

Exit evidence: Help opens an ordinary recognizable tab, page states/readability/locales work, web has no broken shared link, and notification navigation works inside provider scope.

## T6. Seed the real history and document deployment

Owner: D. Depends on: T2 output, A file path, B CLI contract.

Create/modify:

- `server/internal/changelog/content/changelog.json` — initial committed baseline/preview.
- `.env.example`, `SELF_HOSTING.md`, `docker-compose.selfhost.yml`, `deploy/helm/multica/values.yaml`, `deploy/helm/multica/templates/configmap.yaml` as appropriate for runtime configuration/mounts.
- `apps/docs/content/docs/environment-variables.mdx` and `.zh.mdx`; regenerate affected embedded docs through the existing generator if those sources change.
- Root `package.json` only if useful aliases are agreed with B; `.trellis/spec/` only for actual reusable knowledge.

Steps:

1. Attribute official v0.4.37 version/date/summary to the inspected website. Do not seed non-ancestor upstream later releases as fork history.
2. Generate a frozen committed fork preview after local v0.4.37. Check meaningful user-facing entries and exclude dirty work; keep `published_at=null` and `status=unreleased`.
3. Document optional `CHANGELOG_FILE`, the containing-directory mount, publication command, last-valid/stale behavior, cumulative `--history`, and one-time client feature upgrade requirement.
4. Document the automatic CI path and the intranet artifact-delivery step as one release procedure. Do not imply a server sees an artifact before that artifact reaches its deployment.
5. Preserve unrelated edits in shared deployment/environment docs. Regenerate docs bundle once after all source additions are complete and run the relevant Helm/Compose/config checks.

Exit evidence: checked-in seed reflects this fork's actual committed state, and an operator can repeat local/offline generation plus hot publication using documented commands.

## T7. End-to-end acceptance and completion audit

Owner: D with a separate review pass. Depends on: T1–T6.

Create `e2e/changelog.spec.ts` and a focused local feed-update harness if necessary. Use `TestApiClient` for authenticated setup/cleanup. Do not use a mocked transport as sole evidence for the generator → file → API → reader path.

1. Run the relevant unit/integration matrix below. Keep outputs/evidence in the Trellis task; record exact commands, revisions and results.
2. Start or reuse this checkout's verified local environment with `make status`/`make up` and inspect ownership/ports. Configure a task-owned temporary `CHANGELOG_FILE`; do not overwrite an operator feed.
3. Open Help in desktop, verify tab title/icon, navigate history and a requested version, inspect Chinese UI at normal/narrow widths and keyboard traversal.
4. Generate the next synthetic feed in a temporary git fixture, preserving prior history; publish it atomically into that local file. Keep server/client process identities unchanged. Observe the already-mounted reader update within 60 seconds plus response time; then verify a fresh session sees the same feed.
5. Test manual refresh/focus/reconnect and returning to an inactive tab. Disable automatic binary updates or assert independence through the updater integration and repeat note discovery. Record what was exercised in a real app versus deterministic unit fixtures.
6. Make the configured source invalid/missing and simulate network failure. Confirm prior content remains with a stale/failure message, then restore valid content and verify recovery.
7. Run affected package checks and Go tests/vet, then the required repository pipeline. Categorize any unrelated baseline failures with reproducible evidence and resolve task-caused failures. No default test invokes an ambient authenticated agent CLI.
8. Review all task diffs against PRD A1–A8, source truth, package boundaries, security and current Trellis specs. Record completion only when evidence proves every item; update task status/journal/specs through Trellis as appropriate. Any commits the leader makes follow Lore + conventional intent prefixes and exclude unrecognized dirty paths. Do not push tags or publish a production release.

## Verification matrix

| PRD acceptance | Canonical proof | Important assertions |
| --- | --- | --- |
| A1 | Help/desktop notification tests, paths/tab tests, real desktop smoke | Internal route, title/icon, no-workspace/provider safety, startup event retention, restart remains callable |
| A2 | Shared page happy-path/state tests, locale parity, rendered screenshots/visual verdict | Month/version navigation, four locale keys, semantic tokens, wrapping, keyboard and accessible labels |
| A3 | Seed/source audit plus core release-selection tests | Exact official baseline, frozen committed range, no dirty or invented published fork entries, version meanings separated |
| A4 | Generator/publisher → running API → mounted reader acceptance; Query refresh tests | Same processes, atomic replacement, <=60 s + request bound, first install/reopen/focus/reconnect/tab activation, deployment-isolated key |
| A5 | Go reader/transport tests; core parser/query tests; minimal UI states | Auth, empty/404/malformed/unknown enums, last-valid/source-stale, ETag updates when warnings change, no raw HTML |
| A6 | Node temporary-repository tests | Ancestry over semver, merged commits, trailers, dirty isolation, stable/prerelease history, exact reruns and conflict rejection |
| A7 | Publisher filesystem tests, release helper fixtures, offline bundle/upgrade fixture tests, actual YAML/static graph review | Same bytes in builds/assets/bundles, installed runtime feed, fork token/repo, prerequisites, no seed fallback on download failure, no publication on failed build |
| A8 | Trellis artifact/checklist and final commands | No requirement silently dropped, no unsupported passing claim, task/spec/session evidence recorded |

Focused commands are illustrative of the filenames assigned above; update this plan if implementation chooses a different canonical test filename. Exact installed command definitions remain authoritative in `package.json`/`Makefile`.

```bash
node --test scripts/generate-changelog.test.mjs scripts/publish-changelog.test.mjs
pnpm --filter @multica/core test changelog paths
pnpm --filter @multica/views test changelog layout/help-launcher.test.tsx locales/parity.test.ts
pnpm --filter @multica/desktop test update-notification
pnpm --filter @multica/core --filter @multica/views --filter @multica/web --filter @multica/desktop typecheck
pnpm --filter @multica/core --filter @multica/views --filter @multica/web --filter @multica/desktop lint
pnpm exec playwright test e2e/changelog.spec.ts
make test
pnpm typecheck
pnpm lint
pnpm test
make check
```

From `server/`, run `go test ./internal/changelog ./internal/handler -run 'Changelog' -count=1` and `go vet ./...`. Include release YAML/shell/Node syntax validation and relevant deployment config tests in final static analysis. Do not repeatedly run broad suites once green unless subsequent changes or findings justify it. Document environment failures separately from product behavior and continue through available recovery paths.
