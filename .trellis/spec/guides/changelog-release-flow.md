# Changelog delivery and publication

Read this before changing the in-app changelog, release scripts, release
workflow, or offline feed installation. The first implementation is tracked in
task `09-12-desktop-changelog`.

## Boundaries and content truth

- `server/internal/changelog/content/changelog.json` is an attributed baseline
  and a frozen, explicitly unreleased fork preview. Imported upstream tags are
  not proof that the snapshot fork published or incorporated those versions.
- The saved schema is `schema_version: 1`, `generated_at`, and `releases`.
  Producers validate status/date pairing, unique source/repository/version
  identity, full commit IDs, one fork repository, and the 2 MiB UTF-8 limit.
- The Go API serves the connected deployment. It never fetches GitHub or the
  official website on behalf of a client. Source and publication status must
  remain visible; installed desktop, deployed server, and latest stable release
  are different facts.
- Latest stable means the greatest valid numeric `vX.Y.Z` in this fork's
  published history. A late publication of an older tag must not become the
  latest version. The reading timeline still sorts by publication date.
- Public commit wording belongs in `Release-note:` and optional `Changelog:`
  trailers. Do not publish raw commit bodies or Lore decision trailers.

## Live API behavior

`GET /api/changelog` uses normal API authentication. `CHANGELOG_FILE` is a
server-owned path, never a request parameter. Empty configuration uses the
embedded history; a configured file is reread on requests.

Serialize **read → validate → snapshot update** per reader instance. Locking
only the final assignment lets a delayed old read replace a newer snapshot.
Invalid or unreadable files retain the last valid snapshot, with `is_stale`
and a public warning code. Before the first valid read, the embedded baseline
is returned with the same stale signal.

ETag covers the whole API response, including `server_version`, `feed_source`,
`is_stale`, and `warning`. A newly failed refresh must not return 304 against
the earlier healthy response. Reuse the shared weak/list/wildcard comparison
and allow `If-None-Match` in CORS preflight.

Core parsing uses `parseWithFallback` with an invalid sentinel, then raises a
query error on invalid essential data. This preserves previous Query data.
Unknown string enums/additive fields remain readable and cannot create a false
stable-release badge.

## Reader and navigation

- The workspace route is `/:slug/changelog`, shared by web and desktop. Add new
  routes to paths, tab presentation, search metadata **and diagnostic route
  bucketing**. The diagnostic parity test walks all real path builders.
- Query keys include the API base URL. Override the default infinite stale
  time and disabled focus refresh; poll every 60 seconds while visible, and
  refetch on mount, focus, reconnect and tab reactivation.
- OS focus and document visibility differ. Returning OS focus should refresh,
  but an unfocused window that remains visible still receives periodic checks.
- Use the navigation adapter's hash. Native `scrollIntoView` scrolls every
  ancestor and can move the desktop shell; adjust the reader container only.
- Save consumed deep-link selection in the existing tab memento. Reopening a
  tab restores reading position; explicitly activating the selected release
  again must still jump to it. A background refresh must not move the reader.
- Update events can arrive before a workspace shell mounts. Retain them at App
  scope and inject the shell's navigation action. Restart stays usable without
  a workspace, and no navigation/i18n hook may escape its provider.

## Release transaction boundaries

1. Resolve immutable target and ancestral base once. Generate only from Git
   objects; never turn dirty working-tree changes into published notes.
2. Use explicit cumulative history. Only a verified absence of prior releases
   allows seed bootstrap; download/auth/validation failures must fail closed.
3. Generate once before actual backend consumers build. The workflow artifact,
   Docker embedded JSON, published asset and offline bundle use those bytes.
   GoReleaser currently builds the CLI, not the API; changing its tracked files
   before its clean-tree check is unnecessary and breaks release validation.
4. **Revalidate current published history immediately before publication.**
   Workflow concurrency does not protect a later “re-run failed jobs” action:
   successful preparation/build jobs may be reused after another release has
   published. Reject a stale built artifact before remote mutation and require
   full preparation/build again; do not regenerate inside the publisher.
5. Repreparing an older first tag may find no ancestral published release.
   The explicitly configured `--first-base` can supply an ancestor while the
   downloaded cumulative history remains authoritative. It is not seed reset.
6. Keep GitHub latest on the greatest stable version. Multiple GitHub assets
   cannot be replaced as one transaction; retain documented recovery and
   fail-closed verification instead of claiming multi-asset atomicity.

## Offline installation

- Default local bundles carry the checked-in preview. A release bundle uses
  `--changelog` / `CHANGELOG_ARTIFACT` from the completed release procedure.
  The build stages this exact artifact in `.changelog-build/` and passes
  `CHANGELOG_ARTIFACT_PATH`, without rewriting the tracked seed.
- Mount the containing directory, not one bind-mounted file or Kubernetes
  `subPath`; atomic rename must be visible to the running server.
- Targets require Docker/Compose, not host Node. Run the bundled publisher and
  helpers in the loaded frontend image with `--pull never`, `--network none`,
  and host UID/GID. Verify actual directory writes, not just Docker invocation.
- Paths cross three different encodings: Compose JSON, dotenv and Docker mount
  CSV. Validate complete JSON values before mkdir or mounts. Reject CR/LF;
  line-oriented `config --environment` parsing silently truncates them.
- Encode dotenv with proven double-quote/backslash escaping and literal-dollar
  handling. CSV-quote each mount source and double embedded quotes. Shell argv
  quoting alone does not make Docker's CSV valid.
- Preserve unrelated `.env` settings. Stage both outputs; install the feed
  atomically and restore it if configuration finalization fails.
- An offline upgrade also persists the selected `MULTICA_BACKEND_IMAGE`,
  `MULTICA_WEB_IMAGE` and `MULTICA_IMAGE_TAG` through that same staged writer.
  A later ordinary Compose command must keep the installed version without
  transient environment overrides. Feed-only installs omit these optional
  inputs and continue to update only changelog settings.
- Configuration/feed installation and container/database upgrades are separate
  operations. A container start or health-check failure retains the selected
  version and backup, and reports that container/database rollback was not
  performed; never describe this as an atomic deployment transaction.

## Canonical evidence

- `server/internal/changelog/feed_test.go`: schema, stale fallback, isolation,
  concurrent replacement and race testing.
- Handler/router tests: real auth, conditional requests and CORS.
- Core schema/query/selection tests: compatibility, live Query observers,
  deployment isolation and numeric stable selection.
- Reader tests: rendering, notices, focus and tab restoration.
- `scripts/*changelog*.test.mjs`: immutable/cumulative generation, delayed
  publication, no-mutation failures, offline installation and real path parsing.
- `e2e/changelog.spec.ts`: actual generator → atomic file → unchanged Go
  process → already-mounted reader; fresh session and failure recovery.
- `e2e/changelog-desktop.spec.ts`: real desktop preload/renderer/router in an
  isolated Electron shell. The fixture disables native daemon/updater side
  effects, so default acceptance cannot execute installed agent CLIs.
