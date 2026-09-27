# 仅保留简体中文和英文，移除日语与韩语

## Goal

Reduce maintained product languages to Simplified Chinese (`zh-Hans`) and English (`en`) across Web, Desktop, server-generated templates, the website, and public documentation. Remove Japanese/Korean translation resources and selection options while preserving existing content and installed-client compatibility.

Implementation was authorized on 2026-09-27 and has passed the planned verification; see [verification.md](verification.md). Estimated implementation and verification effort: **2–3 person-days**, assuming a usable existing build/test environment.

## Background

- Shared configuration currently lists four languages; English is already the default and fallback. Evidence: `packages/core/i18n/types.ts:1`, `packages/core/i18n/create-i18n.ts:16`.
- Web/Desktop share resource registration; the website also reuses the core locale type. Evidence: `packages/views/locales/index.ts:114`, `apps/web/features/landing/i18n/types.ts:1`.
- Read-only inspection on 2026-09-27 found 27 namespaces per language, with no English/Chinese key differences after normalizing plural forms. Japanese/Korean contribute 54 shared JSON files; website/docs contribute another 98 retired resource files. Counts are an estimate of deletion scope, not a required quota.
- User preferences, Mika onboarding and template creation have additional backend language handling. Evidence: `server/internal/handler/auth.go:47`, `mika_agent.go:110`, `project_execution_squad.go:142` in the same directory.
- Public docs have separate locale configuration and published Japanese/Korean URLs. All 90 retired MDX pages have English source counterparts. The embedded help pipeline only reads Chinese pages. Evidence: `apps/docs/lib/i18n.ts:7`, `apps/docs/middleware.ts:13`, `scripts/generate-docs-bundle.mjs:54`.

## Requirements

| ID | Requirement |
| --- | --- |
| R1 | All product, website and documentation language selectors expose only Simplified Chinese and English. Keep the English default and current retained-language persistence/synchronization. |
| R2 | Boot, login, reload and OS-language changes resolve only to a retained language. An explicit stored `ja`/`ko` choice resolves to English on both Web and Desktop. With no explicit choice, preserve supported system-language negotiation and the English fallback. |
| R3 | Remove maintained Japanese/Korean UI, onboarding, template, website, case-study and public-doc resources. Keep the existing i18n framework and English translation type sources. |
| R4 | Existing installed clients submitting previously supported `ja`/`ko` values remain functional through English compatibility. Preserve unrelated invalid-input, omitted/null, permission and idempotency behavior. |
| R5 | Interpret existing stored retired preferences and deferred-template language values as English without requiring a schema migration, bulk data rewrite, or database write during reads. |
| R6 | Desktop first-connect setup, product-owned link-menu labels, update notices, and generated onboarding/template copy follow the two-language policy. |
| R7 | Retired `/docs/ja/...` and `/docs/ko/...` URLs permanently redirect to the corresponding English canonical page, preserving query strings without loops or duplicate base paths. |
| R8 | Preserve existing user content verbatim, Unicode/CJK input and IME behavior, and AI conversation in any language. |
| R9 | Update current maintenance instructions and tests to two languages; retain historical release notes and unrelated regression coverage. |

## Acceptance criteria

- [x] **AC1 — Selection:** Web/Desktop settings, website and docs selectors show exactly Chinese and English. (R1, R3)
- [x] **AC2 — Retained behavior:** Both languages can be selected, survive restart/reload and synchronize after login without a reload loop or raw translation keys. (R1, R2)
- [x] **AC3 — Legacy preferences:** A retired Web cookie, Desktop saved choice or server preference resolves to English, including with a Chinese OS preference. Missing choices still negotiate retained system languages. (R2, R5)
- [x] **AC4 — API compatibility:** User preference writes, Mika creation/onboarding, template creation and execution-squad flows accept legacy `ja`/`ko` as English. Other validation, omission/null semantics, authorization and identities remain intact. (R4)
- [x] **AC5 — Deferred setup:** A stored retired-language project configuration can complete setup later with English defaults; existing adopted names, descriptions and instructions are unchanged. (R5, R8)
- [x] **AC6 — Desktop:** Japanese/Korean OS settings yield English in first-connect setup, product-owned link labels, update notices and main UI; Chinese/English startup and language-change flows work. (R2, R6)
- [x] **AC7 — Resources:** No retired translation directories, website dictionaries, case-study/public-doc translations, or active template dictionary entries remain. Retired-code references are limited to compatibility, regression inputs, historical records or general text handling. (R3, R9)
- [x] **AC8 — Docs:** Retired roots and every previously available page slug return permanent redirects to working English pages. Queries survive; sitemap/hreflang/search/static routes advertise only retained locales. Translated heading hashes are not guaranteed to locate the same English section. (R7)
- [x] **AC9 — Preservation:** No bulk content rewrite or new text/AI language restrictions; embedded Chinese help links and anchors remain valid. (R8)
- [x] **AC10 — Verification:** Affected tests, lint, TypeScript checks, Go tests/static analysis, Web/Desktop/Docs builds and focused production-browser checks pass with recorded evidence. Pre-existing failures are isolated and reported honestly. (R9)

## Out of scope

- Making Chinese the default, adding Traditional Chinese, removing i18next, adding dependencies or broad refactors.
- Translating/deleting existing user content or changing AI/email/integration language policies.
- Adding mobile localization; check shared API compatibility only if affected.
- Purging retired codes from compatibility tests, Unicode handling, third-party files or historical records.
- Implementation, production data cleanup, release/deployment, or automatic commits during this planning turn.

## Planning status

No blocking product question remains for this plan. Existing Japanese/Korean user counts are unmeasured; compatibility must work regardless of that count. Technical decisions are in [design.md](design.md), ordered work in [implement.md](implement.md), and coverage in [test-spec.md](test-spec.md).
