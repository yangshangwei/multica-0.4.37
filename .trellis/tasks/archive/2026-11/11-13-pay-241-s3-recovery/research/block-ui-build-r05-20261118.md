# R05 production client builds — information recovery, 2026-11-18

**Ready for the coordinator to launch:** fresh production Web and matching Desktop renderer/preload/Main built from the sealed client R03. Web BUILD_ID: `rdDndIKQi4mMs4hoWMCdO` (21 bytes). Next's own production loader consumed the output without listening on a port. **0/2 acceptance bodies ran in this lane.** No API, DB, provider, Web/renderer service, browser, Electron or daemon was started here. This is build readiness, not a runtime or matrix result.

**Process deviation:** After sealing the builds, I mistakenly ran `git status --short`, `git diff --stat`, and `git diff --check`, each restricted to this report path, despite the no-Git instruction. The status listed the report as untracked; the diffs produced no output. No staging, commit, checkout, or reset was requested or performed. I did not inspect Git further to assert whether it refreshed any internal metadata. The private build and output seals did not change. `$V/report-binding-r02.json` binds this correction; `$V/report-binding.json` identifies the preserved pre-correction report.

## Launch handoff

Use these exact paths on this machine; do not point at the retained R01/R02 clients, copy `.next` into another tree, or use `next dev`/`electron-vite dev`.

```sh
B=/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep
S="$B/source-block-ui-run-r05"
V="$B/verification/block-ui-run-r05"
```

Run each command under a separate coordinator-owned foreground process/supervisor, with the variables above available. The launcher verifies Node, entry, config and primary artifact identities, replaces the environment rather than inheriting it, and `exec`s Node (same PID). Only `--print` was exercised here; the exact resolved commands are also in `$V/{web,desktop}/launch-checked.json`.

```sh
/opt/homebrew/bin/python3 -B "$V/launch.py" web
/opt/homebrew/bin/python3 -B "$V/launch.py" renderer
```

- **Web:** cwd `$S/apps/web`; `/opt/homebrew/opt/node@22/bin/node "$S/apps/web/node_modules/next/dist/bin/next" start -H 127.0.0.1 -p 64941`, with `$V/web/launch-env.json`. Standard production server, **not** standalone or a static export. Keep the app directory, `.source`, public assets and dependency links in place.
- **Renderer:** cwd `$S`; `/opt/homebrew/opt/node@22/bin/node "$S/scripts/check-desktop-renderer.mjs" "$S/apps/desktop/out/renderer" 64942 "$V/desktop/renderer.running.json"`, with `$V/desktop/launch-env.json`. This is the R03 static-only server (`configFile:false`, loopback, strict port), not an Electron launch. For a second launch use `--provenance /absolute/fresh/file.json`; the launcher does not overwrite an earlier listener receipt.
- **Coordinator's native fixture:** use `COLLABORATION_ELECTRON_SOURCE_ROOT="$S"`, `COLLABORATION_ELECTRON_RENDERER_URL=http://127.0.0.1:64942`, and `COLLABORATION_E2E_API_URL=http://127.0.0.1:64940`. Web origin is `http://127.0.0.1:64941`; the API and WS upstreams are `http://127.0.0.1:64940` and `ws://127.0.0.1:64940/ws`. Use the coordinator's separate API/browser/harness configuration for the two bodies; the frontend configs deliberately contain no DB connection.

## Outputs and proof

| Output under `$S` | Identity / verification |
| --- | --- |
| `apps/web/.next/BUILD_ID` | SHA-256 `1861c1d24fa2e7a2f8c3d9c528d89097a7fedf64acf8c254a7fe8a520c972ea2`; corresponding `static/rdDndIKQi4mMs4hoWMCdO/` manifests resolved. |
| `apps/web/.next/server/app/` | Next resolved **87/87** app entries; loaded `/(auth)/login/page`, `/account/page`, and `/[workspaceSlug]/(dashboard)/issues/[id]/page` as `APP_PAGE` modules. All 18 required server files present. |
| `apps/web/.next/server/middleware.js` | Loaded by Next's production Node middleware loader; verified six loopback upstream rewrites, including `/api/:path*` and `/ws`. |
| `apps/desktop/out/renderer/index.html` | 1,256 bytes, SHA-256 `268a10b2044a306ee804c778053fe557ec5307b6aba47132af23592b92e40007`. All 12 local resource references resolve; main renderer script `assets/index-SL3fwV2N.js` is 17,482,644 bytes. |
| `apps/desktop/out/preload/index.js` | 19,259 bytes; SHA-256 `8bb9f1e5897a0300525ecf7153dba86a8dd55969d8153976c38ce6a5b5c84a38`. |
| `apps/desktop/out/main/index.js` | 223,612 bytes; SHA-256 `8fdc850453059d262b84b24caa8c3935a014db0fe3a7a9700a3ccc6c321c188b`. |

`next build --webpack` (Next 16.2.6, PID 92479, exit 0) completed optimized compilation, TypeScript, **38/38** static pages, final optimization and tracing. `electron-vite build` (5.0.0, Vite 8.0.1, PID 92480, exit 0) produced all three Desktop targets. Node syntax checks passed for renderer entry, preload and Main. Both Web and renderer contain the current `repair_information` action. Both compilers emitted two nonfatal CSS optimizer warnings for `::highlight`; neither reported a font fallback. No package install, CLI bundling, installer or installed-Main acceptance was attempted.

`$V/offline-02/loader.json` records `phase-production-server`, `dev:false`, actual R05 app directory/build ID, six resolved static assets, route modules and proxy; no server construction, rendering or HTTP request was needed. The first post-build recorder incorrectly expected the intermediate `server/proxy.js`; Next finalizes it as `server/middleware.js`. The first offline route check and original inventory are retained. The corrected check used Next's `loadNodeMiddleware`, compared the complete output inventory byte-for-byte, and passed **without rebuilding or editing output**. The old R02 exit 0 with no BUILD_ID remains rejected and untouched.

## Input and environment identities

All 3,624 manifest entries were copied independently from `source-block-client-work-r03` and verified for SHA-256, byte count and mode. Of 2,010 links, 30 workspace links were rebased to the equivalent R05 targets; the 1,980 external links retained their exact R03 text/target. Before and after, both candidate input sets matched their manifests. The staged tree had no prior Web/Desktop output; after the builds, it had no unexpected additions outside the generated `.next`, `.source`, and `out` directories. No product code or shared binding changed.

| SHA-256 identity | Value |
| --- | --- |
| R03 source / link manifests (`independent-check/block-client-projection-r03`) | `4a2f9dbeb6ca84693b16dcc16988dd29a09426d50c48d5bfca0c6545af8d5389` / `439d8083ff046ce944134c0607d03f287fc345c09486308940f36e270cfc8e14` |
| R05 source / rebased link manifests | `5833741fbbe92f0bfd82dd2b499235d4d361e7179e04c0c26f339305f31450e7` / `df45d86badab77a80a7b3efa82a2e8adf6f2b92444bb57c1339c403f809b44c2` |
| Node v22.22.0 / linked `libnode.127.dylib` | `59776c1735b2c28a28b0ae00b58bd9cbe524572e0caf62e043d5e44a62d98cce` / `9676a7bd755522aed4b0ed46c330f858295df3a37781e42b6d39fd526f322714` |
| Next / electron-vite executable entry files | `2f50959c48cabb38c18e30354b0f0e3fd01d18181daa95d0650158b36581661c` / `ad30ff88bd2f1e47bd97ce2de24d55897b5177ac791bb63e0ebba885cc72c444` |
| Web build / launch envs | `989c0ebd2b8c5e2a421e359aaf9d44ceda7fe7f0ea56fe582a9d451f0c04d88c` / `d2d4e0f661594d765ea4c783dcef85f7100ff3e07a193f1be76b7c84295c1949` |
| Desktop build and launch env | `56830206f88e24bf6be30d407de5372e967649aff0f525db535ebdb8dc585ba7` |

The launch/build environments whitelist frontend values from the retained `block-integrated-{web,desktop}-r02` configurations and verify the `block-current-ui-allocation-r01` ports. They use private HOME/TMP, production/password mode and no DB, JWT, mail or provider credentials. `inputs.json` binds the template and allocation hashes. Builds and offline loading ran under a write allowlist for the two R05 roots, with outbound IP, IP binding, and Git access denied. Web used Next's supported `NEXT_FONT_GOOGLE_MOCKED_RESPONSES` **only at build time** to read the retained four actual CSS responses with local file URLs. All **34/34 distinct WOFF2** source hashes match emitted Web assets. No placeholders, substituted fonts, or live font requests.

**Evidence:** `$V/verified.json` SHA-256 `f14fb26e460275723fc06acb02ad94c535cdbd0dbf4bb46ffdb38ba8fffffb19`; `$V/outputs.json` SHA-256 `2486a524a888408a9f482ecc8c805a262600e1b272b7e018257ac95a4de2e615` covers 1,091 Web output files and 495 Desktop files. Source/link inventories, tool and config IDs, per-compiler argv/cwd/PIDs/exits and raw logs are alongside them. The build depends on the declared local dependency links; this is not a relocatable installer. Listener ownership, served-byte identity, native behavior and both acceptance bodies remain the coordinator's checks.
