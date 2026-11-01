# Reproduce this snapshot

All commands run in `/Volumes/artisan/code/2026/mc-knowledge-ui-qa-20261101`. The private `.env` holds the dedicated database and task credentials. Its allowed origins are exactly `http://localhost:13244,http://127.0.0.1:5418`. Do not use the A2 database or old runtime ports.

```sh
# Production Web/API; requires psql on PATH, as supplied by the local A2 connection template.
MULTICA_DEV_HOME=/tmp/multica-knowledge-ui-dev-20261101 make up C=api,web ARGS='--name knowledge-ui-20261101 --web-mode production'

# Native renderer/preload build, without CLI bundling or daemon discovery.
VITE_API_URL=http://localhost:18324 VITE_WS_URL=ws://localhost:18324/ws pnpm --filter @multica/desktop exec electron-vite build
python3 -m http.server 5418 --bind 127.0.0.1 --directory apps/desktop/out/renderer

# Use a fresh output directory for every intentional rerun; automatic retries stay off.
COLLABORATION_E2E_API_URL=http://localhost:18324 COLLABORATION_ELECTRON_RENDERER_URL=http://127.0.0.1:5418 COLLABORATION_ELECTRON_SOURCE_ROOT=/Volumes/artisan/code/2026/mc-knowledge-ui-qa-20261101 PLAYWRIGHT_BASE_URL=http://localhost:13244 MULTICA_DEV_HOME=/tmp/multica-knowledge-ui-dev-20261101 make env-exec ARGS='-- pnpm exec playwright test e2e/collaboration-knowledge.spec.ts --workers=1 --retries=0 --output=/tmp/multica-knowledge-ui-results/final3 --reporter=json'

pnpm exec tsc --noEmit --strict --skipLibCheck --jsx react-jsx --target esnext --module esnext --moduleResolution bundler --esModuleInterop --resolveJsonModule e2e/collaboration-knowledge.spec.ts e2e/fixtures/collaboration-knowledge.ts
pnpm exec eslint --config packages/core/eslint.config.mjs --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true}]' e2e/collaboration-knowledge.spec.ts e2e/fixtures/collaboration-knowledge.ts
```

The final E2E invocation exited 0. `first` used a line reporter and one case; `round2` ran all 8; `round3` selected Web external, target-only and Desktop; `round4` ran all 8; `round5` through `round7` selected Desktop; `final` and `final3` ran all 8. `final2` failed test-module initialization and executed zero cases. See run-inventory.json for actual results; none used automatic retries.

Cleanup performed: `MULTICA_DEV_HOME=/tmp/multica-knowledge-ui-dev-20261101 make down ARGS=knowledge-ui-20261101`, stopped the owned static server, and verified no native process/profile remains. DB `multica_mc_knowledge_ui_qa_20261101_244`, its closed gates and the detached snapshot remain. When evidence retention ends, `MULTICA_DEV_HOME=/tmp/multica-knowledge-ui-dev-20261101 make destroy ARGS=knowledge-ui-20261101` deletes only this registered environment. Remove the worktree only after retaining its manifest/patch/new-source files.

## Capture revision 2 rerun

Restart uses the same owned registry, `.env` and exact CORS origins above. The final bounded capture command was:

```sh
COLLABORATION_E2E_API_URL=http://localhost:18324 COLLABORATION_ELECTRON_RENDERER_URL=http://127.0.0.1:5418 COLLABORATION_ELECTRON_SOURCE_ROOT=/Volumes/artisan/code/2026/mc-knowledge-ui-qa-20261101 PLAYWRIGHT_BASE_URL=http://localhost:13244 MULTICA_DEV_HOME=/tmp/multica-knowledge-ui-dev-20261101 make env-exec ARGS='-- pnpm exec playwright test e2e/collaboration-knowledge.spec.ts --grep "Web external comment|Desktop native" --workers=1 --retries=0 --output=/tmp/multica-knowledge-ui-results/recapture4 --reporter=json'
```

It passed both affected journeys; choose a new output directory for any future run. Production Web restart rebuilt the same frozen product source; the native static server served the retained pinned renderer. The owned environment and static helper were stopped again after capture. See `recapture-cleanup.json`.
