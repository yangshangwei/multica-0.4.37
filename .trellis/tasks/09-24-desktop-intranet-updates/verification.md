# Verification — 2026-09-24

## Running service

- Compose project: `multica-desktop-updates`
- Container: `multica-desktop-updates-desktop-updates-1` (healthy)
- Local feed: `http://127.0.0.1:18080/desktop`
- Host storage: `data/desktop-updates/public`
- Image: `nginx:stable-alpine@sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b`
- Running as uid/gid 101, read-only filesystem and bind mount, no capabilities.

Published original `0.4.48-dirty` Windows ia32 installer, blockmap and generated
`latest-ia32.yml`, explicitly opting into prerelease testing. The installer is
165,407,520 bytes. Full HTTP download SHA-512 matched metadata, blockmap matched
source bytes, HEAD and 206 Range passed. Metadata is served with no-cache.
Directory listings, private/hidden paths and PUT are rejected. Restart retained
metadata and artifact downloads. Machine-readable evidence: `http-verification.json`.

## Automated checks

- Desktop updater/preferences/runtime config/settings/notification/package/configure suites: 8 files, 98 tests passed.
- Artifact collector/publisher: 23 tests passed; covers atomic visible bytes and metadata order, bad checksums/size, safe filenames, duplicate/collision handling, immutable files, locks and prerelease policy.
- Offline scripts: 12 passed, 3 existing opt-in Docker tests skipped; includes actual collector integration and rejection before expensive builds.
- Desktop lint and node/web typecheck passed. Existing lint warning: `tab-content.tsx:54` react-hooks/exhaustive-deps. Final changed-file lint passed without warnings.
- Docs: 60 tests in 8 files passed, typecheck passed; `go test ./internal/docs/...` passed.
- `nginx -t`, Compose config, Node syntax, Bash syntax, local documentation links and `git diff --check` passed.
- Status helper verified against mocked actual non-default container port/storage after review fix.

## Independent review

No remaining functional blockers. Review fixes: inspect actual running container
port/mount for status; explicitly retag the pinned image before offline docker
save. Publisher, channels, offline wrapper and updater policy were reviewed.

## Known boundaries

- Only local loopback is exposed; LAN binding and stable DNS/TLS are documented
  but not deployed or tested from another computer.
- No signed cross-version installation, Windows elevation, macOS notarization,
  Linux package manager, public-network isolation or user-state retention test.
- Existing client desktop.json and running Desktop application are unchanged.
- Existing unrelated generated-doc drift: agents-create.json,
  self-host-quickstart.json, skills.json differ from regeneration of already-clean
  sources. Incidental generator edits were restored; only desktop-app.json belongs
  to this task. CI's global regenerate/diff check may still flag those baseline files.
- Full monorepo/Go handler/E2E suites not run: no business handlers or UI flows
  changed. No new production desktop package built; existing actual build used.
- Metadata switches independently per platform; no all-platform transaction,
  power-loss durability or automatic downgrade promise.

## Commands

```bash
make desktop-updates-status
make desktop-updates-publish SOURCE=apps/desktop/dist ALLOW_PRERELEASE=1
pnpm -C apps/desktop exec vitest run src/main/updater.test.ts src/main/updater-preferences.test.ts src/shared/runtime-config.test.ts src/main/runtime-config-loader.test.ts src/renderer/src/components/updates-settings-tab.test.tsx src/renderer/src/components/update-notification.test.tsx scripts/package.test.mjs scripts/configure-updates.test.mjs
pnpm -C apps/desktop exec vitest run scripts/update-artifacts.test.mjs
node --test scripts/offline-changelog.test.mjs
pnpm -C apps/desktop lint
pnpm -C apps/desktop typecheck
pnpm -C apps/docs test
pnpm -C apps/docs typecheck
(cd server && go test ./internal/docs/...)
docker compose -f docker-compose.desktop-updates.yml exec -T desktop-updates nginx -t
docker compose -f docker-compose.desktop-updates.yml config --quiet
bash -n scripts/desktop-updates.sh scripts/offline-installer.sh
```
