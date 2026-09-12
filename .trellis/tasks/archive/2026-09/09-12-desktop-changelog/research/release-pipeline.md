# Release pipeline and fork provenance research

Observed on 2026-09-12/13 (Asia/Shanghai). This is research, not a release
announcement. No tags, commits, remote releases, packages, or deployment settings
were changed by this research agent.

## Authoritative repository state

| Item | Observed evidence | Consequence |
| --- | --- | --- |
| Checkout | `main`, `HEAD = origin/main = 502e47d0b` at inspection | Changes already on this branch are distinct from concurrent uncommitted work. |
| Origin | `git@github.com:yangshangwei/multica-0.4.37.git` | A fork's notes must identify this repository, not silently use Multica upstream releases. |
| Upstream | `https://github.com/multica-ai/multica.git` | Upstream is a reference source, not this deployment's release ledger. |
| GitHub repository API | `GET https://api.github.com/repos/yangshangwei/multica-0.4.37` returned `private: false`, `fork: false`, `default_branch: main` | This is currently a public snapshot repository; private deployments must still be supported. |
| GitHub releases API | `GET https://api.github.com/repos/yangshangwei/multica-0.4.37/releases?per_page=10` returned `[]` | There is no observed published GitHub release to label as a released fork version. |
| Reachable local version tag | `git tag --merged HEAD --list 'v0.4.*'` returned only `v0.4.37`, pointing to `f1c059ede` | A tag is a revision marker, not evidence of publication. |
| Describe result | `git describe --tags --match 'v[0-9]*' --long --always` returned `v0.4.37-111-g502e47d0b` | There are 111 commits after the reachable local marker, including merged branches. |
| Upstream baseline | `upstream-v0.4.37` is annotated tag object `904911efd`, peeled commit `79559ebb9`; `git merge-base upstream-v0.4.37 HEAD` returned exit 1 | The histories are unrelated. `upstream-v0.4.37..HEAD` includes the initial snapshot, not merely incremental fork work. |
| Later version tags | `v0.4.38` through `v0.4.43` point to upstream objects and are not ancestors of `HEAD` | Never select the highest semver tag as this fork's release base. |
| Web version | `apps/web/package.json` is `0.4.40`, changed by `38bb2eb5d` | This records a selective feature line, not verified release publication. |
| Desktop version | Source package version is `0.1.0`; packaging uses `MULTICA_DESKTOP_VERSION` or `git describe` | The source package version must not be used as the installed version. |

Exact immutable refs for initial content curation:

- Initial fork snapshot/root commit:
  `a81eb99189ad105faa30a655473d83a023d8f30e`.
- Local `v0.4.37` commit:
  `f1c059edee4ec39d464915e9c9a83cf6c0c37925`.
- Official `upstream-v0.4.37^{commit}`:
  `79559ebb92c48746d716db30a85acdc8c3cef8ec`.
- Initial research `HEAD`: `502e47d0b`. During concurrent work, `HEAD` advanced
  to `e6b5522d4315a2c3a19c0f8ce1335ed60972193a`; the table's branch equality and
  111-commit count are time-stamped observations, not a claim of a frozen tree.

`gh` is installed at `/opt/homebrew/bin/gh` but is unauthenticated in this
session. Read-only public API requests were made with Python's standard library
instead; no credential was inspected or requested.

The worktree was already dirty in messaging integrations, Chinese template
catalogs, requirement-skill guidance, deployment configuration, and their tests.
Those files belong to other active work. A release generator must read committed
objects at its requested ref, not treat dirty filesystem content as released.
Development preview may acknowledge a dirty tree explicitly, without turning it
into a release record.

## Committed fork changes suitable for an explicitly unpublished preview

These examples were confirmed in `git log`, not inferred from current dirty
files. They can seed grouped user-facing draft notes; publication remains a
separate event.

| Theme | Representative commits |
| --- | --- |
| Private-deployment Desktop endpoint setup and deployment-owned logout page | `e81692259`, `3b5706de8`, `dd92eb7ad` |
| Device authentication defaults for self-hosted web/Desktop | `fb7b6bd64`, `f98bc0282`, `670608882` |
| Offline server upgrade and combined Desktop/server installer bundles | `3d28c619d`, `68ecd9f22` |
| Built-in agent role templates, eight squads, staffing, autonomy and approval boundaries | `ca3a79d18`, `f31d33fa2`, `a7b503ee0`, `68be48e1d`, `09dfbf350` |
| Safer daemon conflicts, repeated batch issue updates, and stale-session responses | `dd70358ff`, `6e4256ae3`, `2560b3b1d`, `e6f34a1a7` |
| Embedded Chinese documentation, in-app reader, deployment-local links | `0ef6b1dfa`, `0c96d792b`, `91ea4362a`, `3a8f1050d`, `ecae52ac7` |
| Built-in scheduled automation templates and date-stamped task titles | `69711dc44`, `301f26a16`, `779dbda3a` |
| Chinese role skills, instructions, existing defaults, and editable skill templates | `a99cfe687`, `548788037`, `c9d6cae6b`, `c4ae6ac68`, `0008e8665`, `5ceecd789`, `3a08c3b19` |
| Workspace defaults and recoverable project squad setup | `ef494ddd3`, `af2977950` |
| Windows UTF-8/UTF-16 execution output handling | `8b57ed346`, `e6cabfdea` |

Several initial private-deployment changes predate the local `v0.4.37` marker.
For an initial fork summary, select the explicit snapshot baseline or curated
entries with commit evidence. Do not claim that all of these features were
introduced after the local version marker.

## Existing release and installation mechanics

- `CLAUDE.md` and `.github/RELEASING.md`: release a reviewed commit on `main`
  through a pushed `vX.Y.Z` tag. Patch bumps are the default. The workflow has no
  manual trigger.
- `.github/workflows/release.yml`: validates semver-like tags, rejects `-dirty`,
  runs Go tests and a fail-closed vulnerability scan before publication.
- The `release` job is restricted to `github.repository_owner == 'multica-ai'`.
  GoReleaser and upstream Homebrew publication are therefore skipped for this
  repository. The `desktop` job depends on `release`, so the current fork also
  skips that Desktop publication path.
- Docker backend/web builds run for forks and push
  `ghcr.io/${github.repository_owner}/multica-{backend,web}`. The merged manifest
  gets the exact tag and SHA; `latest` is updated only for stable tags. The Helm
  publication job is upstream-only.
- `.goreleaser.yml` generates a plain commit changelog, excluding only the
  literal prefixes `docs:`, `test:`, and `chore:`. Scoped bookkeeping such as
  `chore(task):` is not excluded. There is no structured client feed or fork
  publication in this configuration.
- `apps/desktop/electron-builder.yml` hardcodes the GitHub publisher to
  `multica-ai/multica` and `releaseType: release`. `scripts/package.mjs` passes
  arbitrary builder arguments through and accepts `MULTICA_DESKTOP_VERSION`.
  Do not silently retarget the binary updater while implementing a read-only
  changelog feature.
- `apps/desktop/src/main/updater.ts` checks after five seconds and hourly when
  automatic binary updates are enabled. It forwards release notes but does not
  provide history. Its macOS x64 and Windows arm64 channels have compatibility
  constraints. The changelog reader must work independently of this preference.
- `apps/desktop/src/renderer/src/components/update-notification.tsx` currently
  opens `https://multica.ai/changelog#release-<version>` externally. This is a
  related entry point to route to the deployment's reader where appropriate.
- `scripts/offline-installer.sh` defaults the bundle to the web package version
  plus `-selective`, passes the version into Desktop packaging, and publishes
  nothing (`--publish never`). `scripts/build-offline-upgrade.sh` takes `VERSION`
  or a git-derived value. A local installer archive existing on disk is not
  equivalent to a public Release.
- `Dockerfile` copies `server/` and embeds Go build inputs but does not copy the
  full git checkout. Generate a release feed before Docker build, or consume a
  runtime feed; do not expect `.git` to be available inside the final image.

## Existing in-app distribution pattern

The docs feature is a strong boundary example:

- `server/internal/docs/embed.go` owns a checked-in generated JSON bundle.
- `server/internal/handler/docs.go` exposes authenticated JSON endpoints with
  private/no-cache plus ETag revalidation. Only allowlisted public screenshot
  assets are unauthenticated.
- `server/cmd/server/router.go` registers `/api/docs/manifest` and
  `/api/docs/page` in the authenticated route group.
- `packages/core/docs/schema.ts` uses Zod and `parseWithFallback` to survive
  installed-client/server drift. `queries.ts` correctly puts content in React
  Query, with global keys because all workspaces read the same deployment data.
- Its current `staleTime: Infinity` is appropriate only for build-immutable docs.
  Do not copy it for release discovery: an existing session must learn about
  newly published notes without a renderer/server restart.
- `/api/config` exposes `server_version` only on self-hosted deployments and
  already populates the Help version display. Installed Desktop version and
  server version may differ; show them separately rather than equating either
  one with the latest released version.
- Desktop runtime endpoints are selected from `~/.multica/desktop.json` via
  `runtime-config-loader.ts`. The reader should use the existing API client to
  reach that deployment instead of contacting a hardcoded public site.

## Recommended release/feed design

Use a deployment-owned, authenticated `/api/changelog` endpoint and a small
versioned JSON artifact. This avoids client GitHub credentials, CORS issues,
private-repository access failures, and public internet dependencies in an
intranet Desktop renderer. No database or new package dependency is needed.

1. A deterministic Node standard-library generator takes an explicit target
   ref/tag, optional base ref, repository identity, and a preview/release mode.
   Use `git` argv arrays and read commit objects; never interpolate commit text
   into shell scripts. Prefer the previous reachable *published* release base;
   require/allow an explicit base for the first snapshot-fork release.
2. Generate structured JSON and Markdown from the same data. Include schema
   version, repository/source identity, version, channel, publication date,
   revision, title, summary and categorized changes. Keep an optional curated
   override for readable Chinese summaries. Each generated item retains commit
   evidence in the artifact, though the product page need not expose hashes or
   private repository links.
3. Distinguish `unreleased` preview entries from `published` releases and
   `upstream` reference entries. Preview mode may read a dirty checkout but must
   ignore uncommitted contents; release mode must verify a clean immutable ref.
   Filter merge/bookkeeping/test-only entries; preserve meaningful legacy
   intent-style subjects and merged feature commits instead of requiring every
   historical subject to be a Conventional Commit.
4. Add an independent fork-safe publication job to the release workflow. It must
   not depend on the upstream-only GoReleaser job. After required artifact jobs
   succeed, create/update this repository's Release and attach `changelog.json`
   and Markdown using `GITHUB_TOKEN` with `contents: write`. Do not change an
   upstream Homebrew repository. Re-runs replace the same version deterministically.
5. Serve a generated embedded baseline immediately, plus a live configured
   feed source. For connected deployments, the server may poll an operator-set
   HTTPS feed URL with bounded timeout/size and ETag caching; a public current
   fork can use its latest stable Release asset URL. For private/air-gapped
   deployments, use a configured local JSON file installed by the release
   process. Keep tokens exclusively on the server if authenticated fetch is
   supported. No client-provided URL should control a server fetch.
6. For local-file publication, mount a directory and atomically rename the new
   JSON inside that directory. Mounting only a single file can pin the old inode
   after atomic replacement. Read/revalidate on requests or a short bounded
   interval, and retain the last valid feed with explicit stale/error status
   when an update is missing or malformed.
7. React Query fetches on open, refresh, reconnect/focus, and a bounded interval
   (for example one to five minutes). The Help indicator, if added, should use
   the same query. Binary automatic-update preferences must not disable release
   note discovery. Scope any persisted read marker/cache to the deployment.
8. Preserve installation and release distinctions: a completed server/web
   release does not prove a macOS/Desktop installer was built. Prefer explicit
   component availability or omit download claims until their artifacts exist.

A file-only embedded bundle without a runtime source is insufficient for the
live-discovery requirement. A GitHub asset that clients cannot actually retrieve
through the deployed API is likewise only half the publishing mechanism.

## Alternatives and tradeoffs

| Option | Assessment |
| --- | --- |
| Desktop directly lists public GitHub Releases | Small implementation, but breaks private/air-gap use, rate limits each client, and reproduces wrong-origin risks. |
| Render only the embedded build-time bundle | Reliable first install/offline baseline, but existing clients cannot see later releases until a server deploy. Keep as baseline only. |
| Runtime-mounted file only | Strong intranet default and simple validation; requires the release/deployment process to install the new artifact. Document and test that handoff, not merely file generation. |
| Server fetch plus runtime-mounted file and embedded baseline | Meets connected and disconnected distribution needs with one client API. Keep source precedence and refresh/error behavior explicit. |
| Database-backed publication API/CMS | Useful for editorial workflows later, but creates storage, authorization and migration work that current requirements do not need. |

## Cumulative history without runtime internet access

The leader's proposed scope is a runtime `CHANGELOG_FILE` plus embedded baseline,
a fork Release artifact, and an atomic intranet publication command. This is
viable when that command is part of the documented release/deployment procedure
and its hot-update behavior is exercised end to end.

Make the generator accept an explicit `--history <json>` input. The local path
must be sufficient: no GitHub query is necessary for generation or runtime
reading. For the first release, pass the curated seed (official upstream
reference plus separately marked unpublished fork summary). For later releases,
pass the previously published cumulative artifact. The generator must:

- Validate the history schema and retain every existing published entry and
  its provenance unchanged. Preserve upstream entries separately from fork
  entries. Identity should contain source/repository plus version, so equal
  upstream and fork version labels cannot overwrite one another.
- Select the prior *fork* published revision that is an ancestor of the target
  ref; use an explicit `--base` for the first fork release. Abort on ambiguous
  or non-ancestor bases instead of generating a misleading empty/full history.
- Generate only `base..target` changes, with all non-merge feature commits from
  merged branches included. Ignore dirty worktree contents.
- Replace/remove the prior unpublished preview when producing a release, then
  append or upsert the exact released version. An exact rerun must be
  deterministic; a same source+version with a different immutable revision
  must fail rather than silently rewrite history.
- Treat supplied published history as evidence, not as permission to promote
  every Git tag to a released entry. Reject malformed/duplicate input rather
  than dropping older releases and succeeding with an apparently valid feed.

CI may retrieve the prior artifact from this repository with its Actions token
into a local history file, then invoke exactly the same generator used offline.
Do this before creating/uploading the next Release asset. A first-run 404 can
use the checked-in seed; authentication errors, malformed artifacts, and other
download failures must fail the publish step instead of resetting history to
the seed. Serialize changelog publication across versions, and re-read history
inside the serialized step so concurrently built tags do not overwrite one
another's additions. Keep stable/prerelease policy explicit; a prerelease must
not become the source of the stable client's “latest release” marker.

The intranet command takes the generated cumulative JSON, validates it, writes
a temporary file in the destination directory, and renames it over
`CHANGELOG_FILE`. The destination is a mounted directory, not a single-file
mount. Since the server hot-reads that path, a new install and an already-open
client receive the same history after the next request. Keep the last published
artifact as the next offline invocation's `--history`; bundling that artifact
beside the installer/upgrade package avoids an external-service dependency.

## Concrete local verification recommendations

These are recommended checks; this research agent did not execute application
tests or publish a release.

- Generator tests with temporary git repositories: reachable versus unrelated
  higher tags, annotated tags, merged feature branches, first release, dirty
  preview versus release rejection, Unicode/newlines, scoped housekeeping,
  prereleases, stable ordering, deterministic reruns, hostile subject strings,
  duplicate versions and malformed overrides.
- A local generator dry-run against this checkout's committed `HEAD` must
  produce only an explicitly unpublished fork preview and leave the checkout,
  tags and remote state unchanged. Capture a representative Markdown+JSON
  artifact in the task's verification evidence.
- Feed/handler tests: malformed/oversized source, unauthenticated request,
  source precedence, ETag 304 and changed-content 200, independent server/client
  versions, atomic file replacement while the same server stays running, and
  last-valid/stale behavior after source failure.
- Core malformed-response and refresh tests: use `parseWithFallback`; an older
  server's 404 or an offline response must produce a clear readable state.
- Browser/Electron acceptance: open via Help on a fresh session; open the newest
  release; replace the configured feed without rebuilding/restarting the app;
  observe the same running client discover it through polling/manual refresh;
  disable automatic binary updates and repeat note discovery; then simulate
  source failure and verify existing content plus honest status.
- CI publishing test with a stub `gh` or isolated fixture endpoint: verify exact
  tag/repository, correctly formatted notes from `--notes-file`, stable versus
  prerelease selection, idempotent re-run and no publication after a failed
  prerequisite. Actual remote publication remains a distinct later action.
- Required project checks for implementation: relevant Node generator tests,
  affected package Vitest/typecheck/lint, backend tests/vet, release YAML/static
  checks, then a real Desktop reader walk-through. Broader checks follow the
  final affected-package matrix.

## Open decisions for the leader

- Pick and document a fork-owned future version/tag convention; do not move or
  reinterpret imported upstream tags.
- Decide whether the default deployment source is only an intranet file or
  also an operator-configured URL. Either must have a fully tested automatic
  release-to-runtime handoff.
- Decide how much upstream reference history to seed from the official page.
  Preserve original versions/dates/source and visibly separate it from local
  unpublished work.
- The repo currently has no published release. A fabricated "v0.4.40 released"
  record is not an acceptable substitute for showing current fork progress.
