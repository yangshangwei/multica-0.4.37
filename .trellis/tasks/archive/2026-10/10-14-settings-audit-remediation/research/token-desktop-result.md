# Token and desktop settings remediation

## Boundary

Fix the PAT response boundary, acknowledgement-controlled token-result dismissal, and retained/pending revocation in TokensTab. Replace its manually mirrored token list with a user-scoped Query using the existing recovery/access helper. Keep drafts and the one-time secret local and never cache a create response. Only PAT-specific API client imports/methods may change; unrelated concurrent client edits remain intact.

Desktop scope is the two daemon switch names, close-behavior select name, and a stable manual-update result live region. Preserve platform gating and main/preload behavior. No backend, dependencies, shared primitive changes or real credentials/updater actions.

## Result

Implementation and focused verification completed on 2026-10-14. The coordinator owns final combined review and native acceptance.

- F11: create/revoke handlers share a synchronous pending guard. Creation inputs and confirmation actions stay disabled during writes. Failed revocation preserves its row, target, confirmation and retry; only a successful server result removes the row. The Base UI Select test checks native disabled semantics and verifies the popup cannot open.
- F12: Escape, close button, backdrop and Done all require the saved-token acknowledgement. The result stays in local component state, and changing accounts remounts that state. Late responses from a previous account neither reveal its token nor start its old list refresh.
- F16: list/create responses use zod and `parseWithFallback` with redacted diagnostics. Lists strip unexpected credential fields; malformed lists become retryable read failures. Malformed creation is explicitly unsuccessful, keeps the draft and does not automatically retry. JSON syntax errors also lose raw credential text before they reach logging or the UI.
- PAT list metadata is in a user-scoped Query using the existing access boundary and settings recovery feedback. Authorized refresh failures retain the input node, focus and draft.
- Create/revoke API calls now live in explicit core mutation hooks. Creation sends only name/expiry variables, delivers the one-time token to a local callback, and resolves with `undefined`. A real-component regression observes every QueryCache and MutationCache event and requires an actual successful mutation with no credential in any captured state.
- Both credential mutations disable automatic retries and use `networkMode: "always"`. This preserves the prior immediate request behavior rather than leaving an offline write queued for a later session. Hook regressions cover global retry defaults and offline create/revoke failures.
- F05: both daemon switches, the close-behavior Select and the token-expiry Select expose purpose-specific names. F17: manual updater outcomes use one stable polite live region, retain button focus and recover from IPC rejection.

## Changed files

Lane-owned source and tests, including the work retained from the preceding implementer:

- `packages/core/api/personal-access-token-schemas.ts`
- `packages/core/api/personal-access-tokens.test.ts`
- `packages/core/api/client.ts` — PAT import/method hunks only; the continuation made no further edits here.
- `packages/core/auth/token-queries.ts`
- `packages/core/auth/token-queries.test.tsx`
- `packages/core/auth/index.ts` — PAT exports only.
- `packages/views/settings/components/tokens-tab.tsx`
- `packages/views/settings/components/tokens-tab.test.tsx`
- `apps/desktop/src/renderer/src/components/daemon-settings-tab.tsx`
- `apps/desktop/src/renderer/src/components/daemon-settings-tab.test.tsx`
- `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.tsx`
- `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.test.tsx`
- `apps/desktop/src/renderer/src/components/updates-settings-tab.tsx`
- `apps/desktop/src/renderer/src/components/updates-settings-tab.test.tsx`

The existing `research/locale-token-desktop.json` fragment was merged by the coordinator. This continuation did not edit locale JSON. Desktop runtime source stayed unchanged during the continuation; only the two locale-fixture assertions needed a type-safe correction.

## Verification

Final focused runs: **51 tests passed**.

| Command | Result |
| --- | --- |
| `pnpm -C packages/core test api/personal-access-tokens.test.ts auth/token-queries.test.tsx` | 22 passed: 17 API boundary + 5 mutation-hook tests. |
| `pnpm -C packages/views test tokens-tab` | 12 passed. |
| `pnpm -C apps/desktop test daemon-settings-tab desktop-behavior-settings-tab updates-settings-tab` | 17 passed across 3 suites. |
| `pnpm -C packages/core typecheck` | Passed. |
| `pnpm -C packages/views typecheck` | Passed. |
| `pnpm -C apps/desktop typecheck` | Node and renderer checks passed. |

Scoped ESLint commands all passed with no diagnostics:

```sh
pnpm -C packages/core exec eslint api/personal-access-token-schemas.ts api/personal-access-tokens.test.ts api/client.ts auth/token-queries.ts auth/token-queries.test.tsx auth/index.ts
pnpm -C packages/views exec eslint settings/components/tokens-tab.tsx settings/components/tokens-tab.test.tsx
pnpm -C apps/desktop exec eslint src/renderer/src/components/daemon-settings-tab.tsx src/renderer/src/components/daemon-settings-tab.test.tsx src/renderer/src/components/desktop-behavior-settings-tab.tsx src/renderer/src/components/desktop-behavior-settings-tab.test.tsx src/renderer/src/components/updates-settings-tab.tsx src/renderer/src/components/updates-settings-tab.test.tsx
```

After the offline policy and test-fixture corrections, ESLint was repeated on `auth/token-queries.ts`, `auth/token-queries.test.tsx` and the two changed desktop tests; all passed. `git diff --check` over the 14 lane source/test paths above also passed. Source review confirmed that TokensTab no longer calls `api.*`, core contains no new platform dependency, and the client transport/auth lifecycle code was untouched by the continuation.

### Regression evidence during continuation

- Before moving requests to hooks, the strengthened cache test failed because creation produced no MutationCache entry. It passes with a real mutation, undefined mutation data and event-by-event cache observation.
- Before selecting `networkMode: "always"`, both offline regressions failed because writes stayed paused instead of reaching the failed request. Both pass afterward, with one attempt and a settled error.
- Moving API dispatch into React Query required waiting for its asynchronous request/observer notifications in two existing assertions; pending input disabling is still asserted immediately.
- Desktop's first renderer typecheck identified three `unknown` locale-resource accesses in the inherited new tests. Assertions now use the actual visible English/Chinese accessible names; the final full desktop typecheck passed.

## Remaining validation limits

- Native Electron geometry/AX acceptance and combined task review remain coordinator-owned. No native acceptance is claimed by this lane report.
- API tests use real `ApiClient` and `Response` objects with stubbed fetch; component/hook/desktop tests use memory fixtures. No live credentials, invitations, token revocations, agent execution or updater installation were performed.
- The repository's existing pnpm configuration warning is emitted before commands; it did not affect these successful checks.
