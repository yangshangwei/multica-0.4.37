# Execution

The already approved user-visible design is the scope; these artifacts formalize it without new product decisions.

1. [x] Inspect existing flows, tests and specs; create and review artifacts.
2. [x] Core lane: regression tests for deadline, metadata, manual retry and cancellation; implement shared contract and narrow request bounds.
3. [x] Desktop lane: recovery UI, pinned editor, safe details/copy, workspace fallback and child-to-main handoff; focused wiring tests.
4. [x] Integrate; run core/desktop tests and affected typecheck/lint.
5. [x] Isolated Electron acceptance and screenshots; persist visual-verdict JSON; run Impeccable detector once over final UI.
6. [x] Independent trellis-check review; resolve findings and rerun affected checks.
7. [x] Update specs, record acceptance, commit only task-owned files using Lore trailers, archive/journal via Trellis.

## Ownership and change boundary
- Core: packages/core/auth/, platform/auth-initializer*, relevant api methods/tests, workspace/hooks* and query request wiring, plus the optional i18n locale-adapter reload contract needed to preserve A4 input. No desktop/locales.
- Desktop: App.tsx, recovery/editor UI and tests, minimal recovery boundary/hook, main/preload handoff and tests, desktop i18n reload deferral, locales/{en,zh-Hans}/auth.json. No core.
- Coordinator: task/spec docs and isolated acceptance harness/artifacts.
- Review starts after both lanes finish. Do not revert other agents or pre-existing changes.
- No cleanup pass; only extraction needed to keep editor mounted, protected by persistence/cancellation regression.

## Validation
pnpm --filter @multica/core exec vitest run platform/auth-initializer.test.tsx platform/core-provider.test.tsx auth api workspace/hooks.test.tsx
pnpm --filter @multica/desktop test
pnpm --filter @multica/core typecheck
pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/desktop lint
git diff --check

No backend checks if backend is unchanged. Broaden only for new evidence.
