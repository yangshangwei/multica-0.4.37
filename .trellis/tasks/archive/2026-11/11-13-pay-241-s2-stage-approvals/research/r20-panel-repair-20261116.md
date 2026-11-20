# R20 stage source-denial presentation repair

Status: implemented and locally checked; independent review and the original four-scenario runtime remain pending.

## Behavior and change boundary

An authoritative R1 source denial clears private R4 evidence. `StageCodePanel` previously rendered the source-denial explanation inside its `record` branch, so clearing the record also removed the explanation.

The repair reuses the existing exported `collaborationStageSourceDenied(view)` helper and renders the existing localized `sourceUnavailable` status once outside that branch. Anonymous R4 denial still displays the same explanation. A pending, unavailable or failed R4 alone does not become source denial, and its existing loading/unavailable/error and retry UI is retained.

Only these product/test paths changed:

- `packages/views/collaboration/stage-code-panel.tsx`: existing helper import and banner relocation.
- `packages/views/collaboration/stage-code-panel.test.tsx`: three named denial regressions, two readable-source loading/error controls, and an unavailable-R4 false-denial assertion.

Both original leaves match the immutable R19 frontend source. Their modes remain `0644`. Core source-cache, source-generation, queries and action derivation files were checked unchanged. No SQL, tracer, locale, E2E case, deadline, dependency or platform file changed.

## Regression evidence

Each new denial regression first renders private review/report content, refreshes R1 to `can_open=false` with null child digests, then keeps the replacement R4 pending, unavailable or failed. It never resolves the replacement to anonymous evidence to obtain a pass. Assertions require the existing status text once, no private review/PR/report/decision content, no approval button and no write. The error case retains Refresh.

The original implementation produced three expected failures on the missing denial text. Nineteen tests were filtered out by the explicit RED name selector. The unused loading-control text was corrected to the actual existing locale after this frozen RED; the original source hashes and raw failure logs remain intact.

| Check | Result |
| --- | --- |
| Original-code denial regression RED | 3 FAIL on missing banner; 19 name-filtered |
| Full owning component suite GREEN | 22 PASS, 0 FAIL/SKIP |
| Existing Core source-cache/action/presentation controls | 44 PASS in 5 files, 0 FAIL/SKIP |
| Views package type-check | exit 0 |
| Scoped two-file ESLint | exit 0 |

Commands used installed pnpm and Node 22:

```sh
pnpm --filter @multica/views test collaboration/stage-code-panel.test.tsx -t 'keeps R1 source denial'
pnpm --filter @multica/views test collaboration/stage-code-panel.test.tsx
pnpm --filter @multica/core test collaboration/stage-source-cache.test.ts collaboration/stage-code-presentation.test.ts collaboration/stage-code-actions.test.ts collaboration/stage-presentation.test.ts collaboration/stage-test-presentation.test.ts
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views exec eslint collaboration/stage-code-panel.tsx collaboration/stage-code-panel.test.tsx
```

The successful tests contain 66 distinct records across six files. Every command retained argv/cwd, exit, complete stdout/stderr and source hashes before/after; the owned bytes remained stable during execution. Logs use fresh private paths and `0600` files under `r20-source-denial-panel-r01/implement/`.

Installed pnpm emitted its existing warning about ignored root `package.json` `pnpm.onlyBuiltDependencies`/`pnpm.overrides`; no configuration or dependency was changed.

## Handoff

Structured hashes, original R19 leaf parity, protected-file hashes and raw command references are in [the machine report](r20-panel-repair-20261116.json). Current panel SHA256 is `58009dc5a0ab43ffbce010b83195054488001d8454347941ef856ab99a91dc93`; test SHA256 is `97500d4429a41b9d78e0dc26797120002e8681fb8d036f35cb7b8bf28b081b79`.

Next: independent review, then bind the accepted frontend graph and rebuild production Web/Desktop before executing the original four scenarios. This implementer ran no DB/API/browser/build/Git commands and makes no S2 or parent acceptance claim. Original R19 read contention and cancellation observations remain separate evidence.
