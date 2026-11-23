# Desktop download page: repository evidence and design direction

Research date: 2026-10-13. Planning only; no product code has changed.

## Existing deployment

- `deploy/desktop-updates/nginx.conf:21` serves `/srv/updates`; `:34` permits direct GET/HEAD files under `/desktop/` with no directory listing. `/health` is separate, and other roots currently return 404. Responses already use `Cache-Control: no-cache` and `X-Content-Type-Options: nosniff`.
- `docker-compose.desktop-updates.yml:14` mounts persistent host `public/` at `/srv/updates/desktop` read-only. The Nginx process runs as UID/GID 101 with a read-only root filesystem and `/tmp` tmpfs.
- `scripts/desktop-updates.sh:135` creates `public/` before starting Nginx. The wrapper resolves configuration with Compose and does not source dotenv. The minimal runtime requires Bash/Docker/Compose, not Node.
- `.trellis/spec/desktop/frontend/intranet-updates.md` requires immutable versioned artifacts, original YAML names, checksum validation, private staging/backups/locks, and complete artifacts before atomic channel metadata replacement. Multiple mutable files are not one transaction.

## Publication and automatic page data

- `apps/desktop/scripts/update-artifacts.mjs:55` inventories only known installer/archive/blockmap names and generated `latest*.yml`; a manually supplied JSON file is not currently included.
- `:87` exports `updateReferences`, validating version, safe local names, sizes and SHA-512. YAML parsing already uses the installed `electron-updater` parser. Reuse these contracts; do not add a new YAML dependency or write an ad hoc browser YAML parser.
- `:167` exposes complete immutable artifacts before independently renaming channel YAML.
- `:177` collects verified offline files; `:205` publishes under a storage lock. `:193` backs up existing channel metadata privately.
- Windows x64 is `latest.yml` plus `multica-desktop-<version>-windows-x64.exe`; other platform metadata must not change the page's Windows choice or activate placeholders.
- Proposed direction: derive a small static `download.json` from validated Windows x64 metadata during collect/publish. Validate and stage its bytes before exposing changes; expose this projection after its referenced installer and channel metadata. A publish is successful only after the projection is installed. JSON and YAML are individually atomic, not a shared transaction.
- If the last projection rename fails, old JSON still references a retained complete installer. Report publication failure and let an idempotent retry finish; do not promise an all-or-nothing rollback. Backup/restore both current metadata and the projection for manual rollback.
- `collect` must carry a generated projection so Node-free Nginx servers using the documented manual transfer flow can install it last. `publish` must regenerate from validated source metadata rather than trusting supplied JSON.
- Existing live releases need a documented bootstrap: recollect/publish the current complete Windows release, or explicitly generate and validate its projection. Do not imply that changing HTML alone reads an old server's nonexistent JSON automatically.

## Static page and configuration direction

- Keep page runtime independent of Next.js, the business API and Electron. No new dependencies or public resource URLs are needed.
- A single checked-in HTML file with inline CSS/JS and system fonts keeps the minimal deployment kit small and avoids coordinating multiple page asset replacements. Native `<details>` can hold FAQs.
- Install owned page bytes atomically into the existing `public/` store from the lifecycle wrapper. Keep all existing artifacts and original update URLs intact. Serve the page at `/desktop/`, with a redirect for `/desktop`; concrete file routes continue to serve downloads.
- Keep administrator-owned business URL configuration in a separate `download-config.json` in persistent storage. Supply a tracked example and initialize only when absent; never overwrite the administrator's configured value on restart, publication or page upgrade. Runtime values stay out of Git. The URL is configuration, not inferred from the download host or port.
- Fetch the projection/config from the same origin without caching. Use bounded fetches and validate shape, version/architecture/filename agreement, and HTTP(S) business URLs without credentials. Render values as text, never as executable HTML.
- Windows has loading, available and unavailable states. Missing/malformed projection or a missing installer must not produce a clickable broken download or guess a filename. macOS Apple Silicon, macOS Intel, Linux x86 and Linux ARM64 always remain visibly pending in this release.
- HTTP intranet origins may lack `navigator.clipboard`. Provide a capability-based copy path and a selectable visible URL if copying is blocked; report actual success/failure. Do not require HTTPS or pretend copy succeeded.
- Reuse Multica branding and the existing semantic color/type scale as the visual reference. A static deployment artifact does not justify loading React or building another component framework. The implementation must document any small generated/copied token subset to avoid silent drift.

## First-use guide evidence

- `apps/desktop/src/renderer/src/pages/endpoint-setup.tsx:60` presents a business HTTP(S) URL, connection test, and save action. Business URL, download URL and update URL are separate concepts.
- `packages/views/auth/password-form.tsx:95` handles registration and login; registration fields also include name (`:137`) and password confirmation (`:151`). `packages/core/auth/store.ts:169` accepts the returned authenticated session, so a mandatory second login immediately after registration would be misleading.
- `apps/desktop/src/renderer/src/App.tsx:247` routes users without completed onboarding to invitations or onboarding; an onboarded user without workspaces goes to new-workspace flow. The user confirmed both paths: accept an invitation when present, otherwise create a workspace through onboarding. `packages/views/onboarding/onboarding-flow.tsx:87` and `steps/step-runtime-connect.tsx:197` support completing onboarding while deferring runtime connection, consistent with excluding AI tool setup.
- `docs/offline-upgrade.zh-CN.md:223` documents password mode with `ALLOW_SIGNUP=true`. The static page cannot enable registration or change authentication configuration. Missing signup is an administrator/configuration issue.
- FAQs can follow real existing behavior: download/network problems, installation blocked by local policy, incorrect business URL, missing signup, username/password errors, and no available workspace. Direct unresolved deployment issues to the intranet administrator; no new ticketing/contact system is needed.
- Do not publish fictional support details, require users to disable security software, or claim installer signatures/Windows native installation were verified by an HTTP download test.

## Validation surfaces

- `apps/desktop/scripts/update-artifacts.test.mjs` is an existing Vitest Node suite with checksum-valid synthetic release fixtures, multi-platform collection, failure/no-mutation tests, and publication tests.
- `scripts/desktop-updates.test.mjs` is an existing Node test suite with fake Docker/Node executables; it is run in CI and can check static page installation and config preservation without starting production services.
- `apps/desktop/vitest.config.ts` includes `scripts/**/*.test.mjs`. Existing `jsdom` can execute the actual page for observable DOM/download/copy tests without adding dependencies; pure publication tests remain Node environment.
- Real browser and isolated Nginx verification remain necessary for HTTP clipboard restrictions, MIME types, 200/206 responses, no-public-network use, keyboard behavior and narrow viewport layout.
- Use isolated temporary storage and a separate Compose project/fixed free port. Do not point a rehearsal at the existing live installer store.
