# Acceptance and regression coverage

This document defines the acceptance coverage. Actual completed checks and isolated pre-existing findings are recorded in [verification.md](verification.md).

| Criteria | Canonical owners | Required coverage |
| --- | --- | --- |
| AC1–AC3 | `packages/core/i18n/pick-locale.test.ts`, `apps/web/lib/locale-routing.test.ts`; add `packages/core/i18n/user-locale-sync.test.tsx` if no equivalent exists | Retained saved choice; explicit retired choice with Chinese/English OS preferences → English; no choice with supported Chinese/English candidate; Japanese/Korean-only regional tags → English; malformed tags; old server preference; one reload at most and no-op after remount. |
| AC1–AC2 | `packages/views/settings/components/preferences-tab.test.tsx`, `packages/views/locales/parity.test.ts`, `locales/mcp.test.ts` | Exactly two choices; both save/sync flows and existing sync-failure behavior; retained namespace registration, interpolation/plurals and translation keys. |
| AC4–AC5 | `server/internal/handler/user_language_test.go`, `mika_agent_test.go`, `mika_onboarding_endpoint_test.go`, `mika_onboarding_opening_test.go`, `project_execution_squad_test.go`, `project_execution_squads_test.go` | Legacy values select English without 400; explicit writes canonical; old stored values returned canonically without write-on-read; omitted/null/invalid inputs retain existing rules; deferred retired-language config materializes; repeated setup preserves identity/content; permissions still enforced. |
| AC4–AC5 | `server/internal/service/builtin_agent_templates_test.go`, `builtin_autopilot_templates_test.go`, `builtin_mcp_templates_test.go`; handler template suites | Only retained active languages; complete copy; legacy requests use English defaults; catalog keys, explicit names, instructions and source synchronization preserved. |
| AC6 | `apps/desktop/src/main/context-menu.test.ts`, `src/renderer/src/components/update-notification.test.tsx`; add `src/renderer/src/pages/endpoint-setup.test.tsx` if absent | Japanese/Korean OS → English product copy; Chinese/English first-connect flows; no removed imports; retained network validation/error behavior and stable restart. |
| AC7 | Affected consumer tests and targeted reference inspection | Retired files/imports/active branches absent; intentional aliases, regression inputs, historical records and general Unicode behavior remain. Do not erase Chinese/English/business assertions with old four-language loops. |
| AC8 | `apps/docs/lib/static-params.test.ts`, `site.test.ts`, `locale-link.test.ts`; add `apps/docs/lib/retired-locale-redirects.test.ts` | Redirect roots/nested paths/queries; basePath once; no loops; English counterpart for every recorded removed slug; no retired sitemap/hreflang/static/search entries. |
| AC8–AC9 | Existing tests under `apps/docs/lib/docs-bundle/` and `server/internal/docs/` | Correct regenerated Chinese help, preserved slugs/anchors; removal of public translated docs does not delete embedded Chinese pages. |
| AC1–AC9 | `e2e/localized-template-defaults.spec.ts`, `e2e/squads-design.spec.ts`; add `e2e/retained-languages.spec.ts` | Preserve Chinese/English creation/content/identity assertions; login/reload with stale cookie/server preference; exactly two choices; successful language switch; no raw keys/reload loop. |

## Runtime acceptance

Use isolated fixture accounts/data and task-owned services:

1. Production Web/API: select both retained languages; reload/logout/login; stale cookie combined with Chinese Accept-Language; old server preference; template creation and a deferred legacy configuration.
2. Desktop: both retained languages, old local storage, Japanese/Korean OS locale simulation where available, connection setup, custom link labels, update notice and reopen. Report any platform simulation limitation explicitly.
3. Built docs server: actual initial 308 and final 200 for retired roots and all known removed slugs, with query preservation; check search, selectors, sitemap/hreflang and Chinese embedded help.
4. User text: preserve representative Japanese/Korean editor/chat content and existing IME/kana/Hangul regressions. Real agent executions or external-account calls are unnecessary.

## Evidence discipline

Before each behavior change, observe the new relevant regression fail, then pass after implementation. Existing parity/import/type checks cover mechanical resource deletion; do not add one redundant test per deleted file or mirror the same pure matrix across components.

Go handler tests use existing `server/internal/testutil` fixtures and a migrated isolated database, never the live development DB. For legacy preferences, assert database values before and after both GET and an unrelated profile update that omits `language`: the stored retired value stays unchanged while the returned representation is `en`. Default tests must not execute real installed agent CLIs.

During implementation add `verification.md` here with source identity, commands, exit statuses, AC mapping and meaningful runtime evidence. For each visual iteration use `visual-verdict` and persist its result under `.omx/state/retain-zh-en-locales/ralph-progress.json` without activating an OMX runtime solely for this task. Do not claim unrun checks passed.
