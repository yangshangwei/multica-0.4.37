# Fork release tag line and desktop updateUrl

## Goal

Make this fork's release versioning work without env overrides, and let an
installed Desktop take automatic updates from an operator-controlled static
directory instead of the hardcoded upstream GitHub release feed.

## Background

- Every build stamp (server, CLI, Desktop package, bundled CLI) comes from
  `git describe --tags --match 'v[0-9]*'`. Local tags `v0.4.38`..`v0.4.43` are
  imported upstream tags whose commits are not ancestors of `main`, so
  `git describe` always yields `v0.4.37-N-g...`.
- `apps/desktop/electron-builder.yml` pins `publish.provider=github` to
  `multica-ai/multica`; any Desktop with internet access would "update" itself
  onto the upstream build. Intranet installs cannot update at all even though
  packaging already produces `latest.yml` and `.blockmap`.
- `~/.multica/desktop.json` (schemaVersion 1) only knows `apiUrl`, `wsUrl`,
  `appUrl`.

## Requirements

### Tag line

- Preserve every imported upstream tag under an `upstream-v<version>` ref
  before removing the bare `v<version>` local tag.
- Remove local `v0.4.38`..`v0.4.43`. Do not touch `origin` (they were never
  pushed) and do not delete anything on `upstream`.
- Configure `remote.upstream.tagOpt = --no-tags` so a later fetch does not
  reintroduce them.
- After the updateUrl change is committed, tag that commit `v0.4.44`
  (annotated). Do not push the tag; pushing triggers `release.yml` and is the
  user's decision.
- Document the rule in `.github/RELEASING.md`.

### Desktop updateUrl

- `desktop.json` stays at `schemaVersion: 1`; add optional `updateUrl`.
  Validation matches `apiUrl`: http/https, no credentials, no query/hash,
  trailing slash trimmed. Missing field means "use the built-in GitHub feed".
- Desktop main process: when `updateUrl` is configured, point electron-updater
  at it with the `generic` provider; otherwise keep the current behavior.
  Architecture channel selection (`latest-x64` on mac x64, `latest-arm64` on
  win arm64) must keep working with the generic provider.
- Saving the server address from the login page must not drop a manually
  configured `updateUrl`.
- Dev mode (`electron-vite dev`) ignores `updateUrl` exactly like it ignores
  the rest of `desktop.json`.
- Docs: `apps/docs/content/docs/desktop-app.{,zh,ja,ko}.mdx` describe the new
  field; `.github/RELEASING.md` describes the static directory layout for
  intranet Desktop updates.

## Out of scope

- CLI `multica update` and `cli-bootstrap.ts` download sources.
- Building Desktop installers in this fork's CI.
- Pushing any tag.

## Acceptance Criteria

- [ ] `git describe --tags --match 'v[0-9]*'` on the tagged commit prints `v0.4.44`.
- [ ] `git tag -l 'v0.4.3[89]' 'v0.4.4[0-3]'` is empty; `upstream-v0.4.38`..`upstream-v0.4.43` resolve to the former commits.
- [ ] `parseRuntimeConfig` accepts, normalizes, and rejects `updateUrl` per the rules above (unit tests in `apps/desktop/src/shared/runtime-config.test.ts`).
- [ ] `setupAutoUpdater` calls `setFeedURL({provider:"generic", url})` only when configured (unit tests in `apps/desktop/src/main/updater.test.ts`).
- [ ] `runtime-config:save` preserves an existing `updateUrl`.
- [ ] `pnpm --filter @multica/desktop test` and `pnpm typecheck` pass.
- [ ] Docs and RELEASING.md updated.
