# P1 mobile compatibility verification

Date: 2026-10-05. Working directory: `/Volumes/artisan/code/2026/multica-projects-p1`.
Owner: mobile compatibility lane. This report covers the MO/node compatibility boundary, not a full new mobile progress editor or iOS visual acceptance.

## Implementation

- `apps/mobile/data/api.ts` / `schemas.ts`: reuse core's pure Project schema, retain absent optional P1 fields, preserve `done_count` as completed plus cancelled, reject malformed/foreign read and write responses, capture workspace UUID in request headers, and preserve read cancellation/timeouts. Capability fallback requires the dedicated workspace endpoint's 404 followed by a parsed readable workspace; resource 404, 400, 401, 403, network and malformed responses remain errors.
- `data/queries/projects.ts`, `mutations/projects.ts`: mobile owns its flat `Project[]` and detail caches. Read query identities remain workspace-scoped. Property writes merge the authoritative response without dropping additive fields. Description/completion and project/resource deletion await server success. Description requests pass an explicitly supplied baseline revision unchanged; this compatibility release does not infer a revision from a newer cache. A legacy description write receives an explicit 428 message directing users to Web/Desktop, and the existing edit form retains its input on failure.
- `data/realtime/project-ws-updaters.ts`, `project-access.ts`, realtime hooks/provider and `data/query-client.ts`: retain optional P1 fields under partial events, honour explicit null, reject foreign/stale versions, invalidate identity-only events, and preserve T1 formal admission (`not_required` / `accepted`, with absent field compatibility for a pre-T1 backend). Revocation synchronously cancels/removes workspace Query data, blocks late responses and WS repopulation, then unmounts the workspace editor subtree before attempting navigation. Repeated access failures use one navigation responder. Navigation failure cannot preserve sensitive cached data.
- Project header/row: label the legacy numerator as scope closure, show authoritative completed/cancelled/open split only when complete, and explicitly direct users to Web/Desktop for health, progress and acceptance editing. No new mobile editing surface, renderer or dependency was added.
- `components/inbox/detail-label.tsx`: one missing `triage: "Triage"` label was added with parent authorization to repair the pre-existing typecheck failure; mobile's existing labels are English-only. No unrelated main-worktree edits were copied.

## Baseline and TDD evidence

Baseline commands all ran in the worktree above, before mobile code edits:

| Command | Baseline result |
| --- | --- |
| `pnpm -C apps/mobile typecheck` | Failed: only `components/inbox/detail-label.tsx:37`, missing `InboxItemType.triage` mapping (TS2741) |
| `pnpm -C apps/mobile lint` | Exit 0; 0 errors, 7 existing warnings |
| `pnpm -C apps/mobile test` | 21 test files, 123 tests passed; iOS wrapper shell tests passed |

Initial new API/Query/WS test run (`vitest run data/project-p1-api.test.ts data/queries/projects.test.ts data/realtime/project-ws-updaters.test.ts`) observed **15 failing / 4 passing** tests before implementation. Failures demonstrated accepted malformed/wrong-identity responses, dropped optional fields, stale/foreign WS writes, uncaptured workspace headers, absent 428 explanation, and uncleared 403 data. The same 19 tests then passed after implementation.

The later access failure test deliberately threw from the navigation listener; it failed before isolated cleanup/listener ordering was implemented, then passed. This establishes a meaningful failure-and-fix case for the navigation-failure requirement rather than only snapshot assertions.

Raw command logs for this run: `/tmp/p1-mobile-baseline-{typecheck,lint,test}.log`, `/tmp/p1-mobile-red.log`, `/tmp/p1-mobile-red-access.log`, `/tmp/p1-mobile-final-{typecheck,lint,test}.log`.

## MO requirement coverage

| Requirement | Canonical tests / evidence |
| --- | --- |
| PRJ-014 / FR-13: old/new payloads, zero versus absent split, strict response identity | `data/project-p1-api.test.ts`: legacy closure/zero distinction, malformed/negative/unsafe/foreign identity matrix, strict create/update/list |
| FR-13: capability status matrix and explicit 428 restriction | `data/project-p1-api.test.ts`: dedicated 404 + readable workspace only; 400/401/403/500; deleted workspace; malformed/network; supported payload; description 428 and 409 |
| MO HTTP transport / 401 | Same suite: captured workspace headers, caller cancellation, 30-second timeout, existing sign-out callback |
| FR-15: mobile flat Query keys and sensitive cache removal | `data/queries/projects.test.ts`: QueryClient flat list/detail, captured signal/workspace, workspace-limited 403 cleanup |
| FR-15: partial/new/old WS compatibility and T1 formal scope | `data/realtime/project-ws-updaters.test.ts`: optional-field/null merge, stale/legacy revision handling, foreign identity, identity-only invalidation, formal admission matrix |
| AC-26 / FR-15: revocation and late-response protection | `data/realtime/project-access.test.ts`: cleanup despite navigation failure, pending Query and WS cannot refill, one responder; API suite separately verifies late successful HTTP is rejected |
| FR-15: subscriptions/reconnect | `data/realtime/use-projects-realtime.test.ts`: self versus other-member removal, progress identity-only events, triage/status category/task change invalidation, reconnect |
| FR-13 / AC-25: old property writes and version editing compatibility | `data/mutations/projects.test.ts`: no optimistic description/completion, additive fields survive property success, 403 cannot roll back/restore protected data, failed deletion keeps detail/list |

## Final checks

Final checks at approximately 2026-10-05 16:27 Asia/Shanghai:

- `pnpm -C apps/mobile typecheck`: exit 0.
- `pnpm -C apps/mobile lint`: exit 0, 0 errors and the same 7 baseline warnings; no new mobile warning.
- `pnpm -C apps/mobile test`: **27 files / 160 tests passed**, including **37 added tests**; `scripts/ios-run.test.sh` also passed.
- `git diff --check -- apps/mobile`: passed. New files were checked against git ignore rules and are not ignored.
- No new dependency; no Web hooks, store or foreign query factory imported. Shared runtime imports are pure schemas/metrics only.

## Limits and handoff

- Tests execute in mobile's existing Vitest node lane with mocked native entry points; no RN renderer, simulator, device, IPA build or live cross-client UI test was run here. The parent verification lane must keep those visual/runtime checks distinct from this MO evidence.
- The new backend contract and shared schema/metrics are owned and verified by their respective P1 lanes. This mobile commit depends on those changes being included together; it is not a standalone backport to pre-P1 shared packages.
- All 7 lint warnings are pre-existing and out of this slice's scope. The root pipeline excludes mobile, so these explicit commands remain necessary after integration changes to shared types/schemas.
- Spec-sync review: mobile-owned data/cache rules and pure import boundaries already exist in `apps/mobile/CLAUDE.md`; no new architectural exception was needed. The 428 editing restriction and access-epoch rationale are documented here and inline at the implementation boundary.
