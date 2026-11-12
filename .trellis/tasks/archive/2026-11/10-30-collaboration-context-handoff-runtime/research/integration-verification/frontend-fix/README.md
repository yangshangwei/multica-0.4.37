# Frontend integration fixes

Scope: isolated `mc-runtime-integration-20261031` checkout, source HEAD and file SHA-256 in `verification.json`. Existing imported changes are preserved. No backend/protocol/SQL edits, commits, or service restarts.

## Changes

- `run-handoff-schema.ts` now charges 256 KiB material using original envelope metadata/framing and decoded UTF-8 source contents. Canonical sizing matches Go HTML and line-separator escaping and includes the HTTP newline. Unknown metadata is counted before Zod normalization/stripping. API body bytes have an independent 2 MiB cap before JSON parsing; all reads keep existing scope/auth fences and optional-route behavior.
- `run-handoff-queries.ts` normalizes run IDs in query keys. New direct tests cover denial cleanup, unavailable/legacy route authority checks, transient parent errors, late success and denial across Server/viewer/access changes, real Query focus/online reconnect and collaboration transport issue/reconnect invalidations. Imported 15-second fallback remains unchanged; no new polling was added.
- Imported retention-chain protection passes all 27 record query tests, including different stream, changed watermarks, absent/introduced summary, unknown tail and coherent full refresh. These implementation files were verified without rewriting them.
- Handoff Handler now uses the existing issue projection only when stable account IDs agree, retaining the actual handler UUID. A mismatch shows the existing unavailable-author fallback. No account-directory fetch or source attribution changes.
- `navigation.tsx` handles only same-origin, same rendered pathname+search fragment changes via Next's supported History API, preserving push/replace, repeated explicit focus intent and Back/Forward. Other paths/queries/origins remain on the router. The source UI uses the real shared `AppLink`; no shared Next import, global click interception, location assignment or dependency change.
- Record UI tests route independent handoff requests separately instead of consuming ordered record-page mock responses.

## Navigation root cause and evidence

`navigation-initial.log` and `navigation-initial-events.json` reproduce the defect on the initial production service. A click with a correct source href generated `...#run-old#run-new`, after which the account route correctly rejected the malformed fragment and replaced it with `/account?view=current`.

Installed/running Next 16.2.6:

- `apps/web/node_modules/next/dist/client/components/router-reducer/create-initial-router-state.js:30` seeds the browser canonical URL from `location` including its initial fragment, then passes that URL into route discovery.
- `apps/web/node_modules/next/dist/client/components/segment-cache/navigation.js:156` concatenates `route.canonicalUrl + url.hash`.
- `apps/web/node_modules/next/dist/client/components/app-router.js:234` patches native pushState/replaceState to copy Next internal history state and dispatch ACTION_RESTORE; its own comments explicitly guarantee usePathname/useSearchParams updates. The fix uses this supported entry point at the Web platform boundary.

The canonical Web regression fails before the fix and passes afterward; it renders a real AppLink, checks a single exact fragment and selected issue/view, traverses history Back/Forward, verifies replace/removal and tests ordinary query/path/external navigation. Existing shared AppLink tests retain modifier/target behavior coverage.

## Checks and remaining verification

Final focused suites: **125 Core + 52 Views + 21 Web = 198 tests passed**, no skipped tests. Full `pnpm typecheck` and `pnpm lint` passed; existing lint warnings remain. `git diff --check` passed. Commands and logs are listed in `verification.json`.

Production Web/native Desktop still run the initial snapshot by instruction. Parent must rebuild and rerun the frozen E2E cases; this report does not count unit/history simulation as final platform proof. The E2E fixture owner must change its Handler `dd` assertion from UUID-only to the matching recipient name plus stable UUID. The diagnostic reproduction is intentionally tied to the initial production build and is not a replacement final E2E.
