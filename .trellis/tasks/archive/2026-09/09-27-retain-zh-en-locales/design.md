# Two-language product design

Status: implemented and verified; see [verification.md](verification.md).

## Decision and contracts

Retain i18next, current adapters and English default/fallback/type sources. Shrink the active UI locale set to `en | zh-Hans`. Content APIs keep their existing Chinese identifier `zh`; public docs keep the `/zh` prefix. Do not rename these different contracts for cosmetic consistency.

| Boundary | Active values | Retired-value behavior |
| --- | --- | --- |
| UI/user preference | `en`, `zh-Hans` | Exact previously supported `ja`/`ko` becomes `en`. |
| Template/Mika content | `en`, `zh` | Retired requests and stored deferred values select English before validation/dictionary access. |
| Public docs | English without a locale prefix; Chinese `/zh` | Retired path families permanently redirect to English counterparts. |
| User text/AI conversation | Unrestricted | Preserve current Unicode/IME and conversation-language behavior. |

## Preference resolution and cross-version behavior

Desktop considers saved choices separately from OS preferences, while Web combines cookie/header candidates. Merely shrinking the supported list can resolve stale cookies differently between platforms (`packages/core/i18n/pick-locale.ts:22`, `apps/web/lib/locale-routing.ts:18`).

1. Explicit retained preferences win.
2. Normalize an explicit saved `ja`/`ko` to English before candidate matching. A later Chinese OS/header preference must not override that explicit retired choice.
3. Without a saved choice, preserve existing supported-language negotiation; Japanese/Korean-only regional OS tags fall back to English. Retain unrelated malformed-tag behavior.
4. Normalize an old server preference before the supported-value guard in `packages/core/i18n/user-locale-sync.tsx:21`. This protects a new client against an old backend. Preserve one reload at most, with a no-op after remount.
5. Reuse current storage keys and adapters. Do not write storage during render, add another store, or change unrelated account-preference precedence.

Prefer extending existing resolvers. If one tiny shared core helper avoids three divergent retired-value rules, it is justified; a new language service or migration framework is not.

## Backend and data ownership

- In user-preference requests, normalize exact retired aliases, then validate `en`/`zh-Hans`. Explicit writes save the canonical value. Other invalid inputs remain rejected where they are rejected today.
- Normalize retired values in existing user-response mapping without writing to the database. Preserve unset/null behavior; omitting `language` from an unrelated profile update must not overwrite it.
- Normalize content-language aliases before Mika creation, onboarding/opening copy and execution-squad validation. Keep English template fallback and expose only `en`/`zh` as active template languages.
- Normalize old deferred execution-squad JSON when consumed, including configuration reuse. Never rename existing adopted agents/squads or replace user-owned descriptions/instructions.
- No schema migration or bulk backfill is required: the user field is nullable `VARCHAR(20)`. Runtime compatibility also handles old clients that continue submitting retired values.

Evidence: `server/migrations/060_add_user_language.up.sql:1`, `server/internal/handler/auth.go:47`, `server/internal/handler/mika_agent.go:110`, `server/internal/handler/mika_onboarding.go:69`, `server/internal/handler/mika_onboarding_opening.go:52`, `server/internal/handler/project_execution_squad.go:53`, `server/internal/service/builtin_agent_templates.go:158`.

## Ownership map

| Area | Existing files and intended change |
| --- | --- |
| Shared configuration | `packages/core/i18n/types.ts`, `pick-locale.ts`, `user-locale-sync.tsx`; shrink active language support and handle retired preferences. |
| Shared resources/settings | `packages/views/locales/index.ts`, `packages/views/settings/components/preferences-tab.tsx`; remove retired imports/options and `locales/ja/`, `locales/ko/`. Keep English imports in `packages/views/i18n/resources-types.ts`. |
| Onboarding/API request types | `packages/core/onboarding/use-bootstrap-mika.ts`, `packages/core/api/client.ts`, `packages/core/types/project.ts`, `packages/views/onboarding/templates/index.ts`, `mika.ts`, `install-runtime-issue.ts`, `packages/views/agents/create/use-role-templates.ts`. |
| Workspace defaults | `packages/views/workspace/celestial-workspace-names.ts`, `slug.ts`; remove unused locale variants, preserve general kana/Hangul input and empty-slug handling. |
| Desktop | `apps/desktop/src/renderer/src/App.tsx`, `pages/endpoint-setup.tsx`, `components/update-notification.tsx`, `globals.css`; `apps/desktop/src/main/context-menu.ts`. Custom link labels use Chinese for Chinese OS preference and English otherwise; OS-owned dialogs remain OS-controlled. |
| Web/website | `apps/web/app/layout.tsx`, `lib/locale-routing.ts`, `features/landing/i18n/types.ts`, `context.tsx`, `lib/use-cases-source.ts`, `use-cases-i18n.ts`, `docs-href.ts`; delete `features/landing/i18n/ja.ts`, `ko.ts`, and `content/use-cases/auto-data-analysis.ja.mdx`, `.ko.mdx`. |
| Server templates | `server/internal/service/builtin_agent_templates.go`, `builtin_agent_templates_roster.go`, `builtin_autopilot_templates.go`, `builtin_autopilot_templates_roster.go`, `builtin_mcp_templates.go`, `builtin_squad_templates.go`; remove retired dictionaries only where present, preserve catalog identities/defaults. |
| Maintenance | English/Chinese conventions, `.trellis/spec/views/frontend/builtin-skill-localization.md`, current language comments and affected built-in skill docs/source maps. Preserve historical changelogs. |

Update all `Record<SupportedLocale, ...>` consumers with the narrowed type in the same integrated client slice. Do not commit a type-broken intermediate client tree. Remove only proven unused keys/styles; preserve English/Chinese business logic, state ownership and routing adapters.

## Public docs and embedded help

Change `apps/docs/lib/i18n.ts`, `lib/translations.ts`, `app/api/search/route.ts` and affected tests. Before deleting `*.ja.mdx`, `*.ko.mdx` and their metadata, record every removed slug and verify its English counterpart.

Use the existing `apps/docs/next.config.mjs` redirect mechanism. Because `basePath` is `/docs`, config-relative sources are `/ja/:path*` and `/ko/:path*`, destination `/:path*`, `permanent: true`; do not duplicate `/docs` in these rules. Cover retired roots and nested pages, queries, trailing slash normalization and loops with config tests and actual built-server HTTP checks. Known removed pages must yield **308 → English page 200**. Avoid adding competing middleware rules.

Sitemap, hreflang, static params, language controls and search derive from config/source but need post-build verification. Unknown paths keep normal 404 behavior. Retired translated section hashes may not match English heading IDs; this task does not translate anchors.

Keep the Chinese help pipeline intact (`scripts/generate-docs-bundle.mjs` → `server/internal/docs/content/`). If Chinese conventions change, regenerate and review the intended bundle diff; do not demand a zero diff for an intentional source change. Preserve help slugs and required anchors. Evidence: `.trellis/spec/docs/frontend/docs-bundle.md`, `server/internal/docs/embed.go:19`.

## Alternatives and tradeoffs

- **Chosen:** remove active resources, retain narrow retired-code compatibility. Meets the two-language goal without breaking existing users or installed clients.
- **Rejected:** hide selector items only; resource maintenance and alternate entry points remain.
- **Rejected:** remove i18next; two languages still require selection/interpolation, and this greatly expands the refactor.
- **Rejected:** reject all old requests or bulk rewrite existing content; delayed setup/older clients break and user-owned data changes unnecessarily.
- **Rejected:** split the website into a new independent four-language system; it conflicts with the requested scope and adds maintenance.

## Rollout, rollback and risks

Implement backend compatibility before removing client choices; verify the integrated release before shipping. For separately deployed components, ship the compatible backend first. New frontend normalization also tolerates an old backend's stored retired preference.

No deployment is authorized by this plan. No bulk migration means reverting code/resources restores prior language availability. Newly canonicalized English preferences remain valid after rollback; their former language cannot be automatically reconstructed. Permanent redirects may be cached, so verify all destinations before publication.

Use task-owned build environments: the current checkout may have live development services, and distinct ports do not isolate `.next`. Do not kill unrelated listeners or run concurrent builds in the same checkout. Unknown existing-user counts do not block planning because compatibility is unconditional.
