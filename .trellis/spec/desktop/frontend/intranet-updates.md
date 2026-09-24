# Intranet desktop release storage

The desktop main process already accepts `desktop.json.updateUrl` as a generic
electron-updater feed. Missing business configuration requires setup; a missing
update URL retains the packaged provider. Development mode does not load this
file or enable the packaged update flow.

## Release contracts

- Use `apps/desktop/scripts/update-artifacts.mjs` for offline collection and
  publication. It uses the installed electron-updater YAML parser; when upgrading
  that dependency, run its artifact tests to verify the parser import contract.
- Preserve generated channel filenames and artifact bytes. Verify SHA-512,
  sizes and safe local references before publishing. Stable versions are the
  default; prereleases require explicit opt-in for local testing.
- Keep publication staging, metadata snapshots and locks outside `public/`.
  Publish complete immutable artifacts first and atomically rename metadata
  last. Each channel switches independently; this is not a multi-channel or
  power-loss transaction. Do not overwrite a versioned filename with new bytes.
- `offline-installer.sh` must include update metadata, ZIPs and blockmaps, not
  just a glob of installer filenames. Reject unapproved prerelease VERSION
  before expensive builds.
- Setting `autoUpdater.channel` enables downgrades as a side effect. Explicitly
  restore `allowDowngrade = false` for every architecture-specific channel.

## Local service

`docker-compose.desktop-updates.yml` runs the separate Nginx project. Mount only
the host `data/desktop-updates/public` directory read-only. The packaging `dist`
directory is transient and must never be the persistent download store.
The lifecycle wrapper starts with `--pull never`; prepare or load images first.
Retag the verified image digest before saving an offline image archive, because
pulling by digest need not assign the mutable source tag.

Nginx must serve direct GET/HEAD downloads, Range requests and revalidated
metadata, with no directory listing or upload API. Default binding is loopback;
LAN binding is explicit. Status reads actual container bindings/mounts.

## Configuration and verification

`configure-updates.mjs` validates with the existing runtime parser, retains
business and unknown fields, backs up original bytes and atomically changes
only `updateUrl`. It requires Node >=22.18 for the TypeScript parser import.
Do not invent business endpoints when configuration is absent.

Use artifact/configuration/updater regressions and an actual HTTP full-file
checksum plus `206` check. Container restart must retain host artifacts.
Download checks do not prove signed installation, elevation, offline certificate
trust or user-state retention; those need an installed older client on each
target OS/architecture.
