# Retain Chinese and English Implementation Plan

> For the implementing agent: use the repository Trellis execution/check workflow; use `superpowers:executing-plans` for stepwise execution where available. Implementation was authorized on 2026-09-27 and completed; results are in `verification.md`.

**Goal:** Keep only English and Simplified Chinese across the product/website, with English compatibility for retired values and working legacy docs URLs.

**Architecture:** Preserve existing i18n, adapters, routing and template ownership. Narrow active registries, normalize retired values at existing boundaries, delete obsolete resources and redirect retired docs paths.

**Tech stack:** TypeScript, React, Next.js, Electron, i18next, Go/Chi, PostgreSQL, Vitest, Playwright.

## Cleanup plan and constraints

Read [prd.md](prd.md), [design.md](design.md), [test-spec.md](test-spec.md), the full `.trellis/workflow.md` and the curated manifests. The workflow is read directly rather than injected because its size exceeds the context injection limit. One coordinated task owns this change because shared locale types couple the client slices. Task creation is not implementation authorization.

Make bounded passes: compatibility, active consumers/registries, resource deletion, then documentation/integration. Add regression protection before behavioral changes. Prefer deletion and existing helpers; no dependency, store, general migration framework or repository-wide `ja/ko` replacement.

Preserve others' edits and the pre-existing `.impeccable/` directory. Keep live services untouched during planning. Use a task-owned checkout/environment for builds and production browser checks; separate ports alone do not isolate `.next` output.

## Slice 0 — Baseline and inventory

- [x] Recheck the implementation request, rules and git state; only then run `task.py start` for this task.
- [x] Record exact retired resource paths and old docs slugs with `git ls-files` before deletion. Baseline estimate: 54 shared JSON + 98 website/docs resource files, not a deletion quota.
- [x] Confirm current source/test owners against design and run focused retained-language/parity checks. Record unrelated baseline failures without expanding scope.

## Slice 1 — Backend compatibility and catalogs

Owners: handler files `auth.go`, `mika_agent.go`, `mika_onboarding.go`, `mika_onboarding_opening.go`, `project_execution_squad.go`; catalog files under `server/internal/service/builtin_*templates*.go`; corresponding test-spec AC4–AC5 suites.

- [x] Add failing tests for retired write/read requests, Mika/onboarding and deferred project configuration. Include no-write-on-read, invalid/omitted/null, permissions, identity and adopted-content preservation.
- [x] Normalize retired aliases before existing validation/dictionary access, preserving `zh-Hans` user values versus `zh` content values and unrelated rejection rules.
- [x] Shrink active catalog language lists/copy to English/Chinese, retaining existing English fallback, template IDs and user-owned instances.
- [x] Re-run focused tests against an isolated migrated DB. No schema migration or bulk data rewrite belongs in this slice.

Checkpoint: older clients can still finish supported workflows using English defaults.

## Slice 2 — Shared clients, Desktop and website

Owners: all client files in the design ownership table. Update narrowed shared types and every consumer together before a typecheck/commit checkpoint.

- [x] Add failing resolver/sync tests for explicit retired preference, OS/header candidates and at-most-once reload; settings tests must expect two choices.
- [x] Narrow the shared locale type/list; retain English default/fallback; align Web cookie, Desktop storage and server-preference synchronization semantics.
- [x] Update resources/settings, request-language unions, onboarding copy, template mapping, workspace-name variants and all `Record<SupportedLocale, ...>` consumers.
- [x] Update Desktop connection/update/custom link-menu copy, HTML lang maps and unused Japanese-only styles. Preserve OS hooks and general CJK/IME behavior.
- [x] Update website locale names/context, case-study mapping, docs links and HTML lang. Preserve retained case-study fallback and existing URLs.
- [x] Delete retired shared JSON, website dictionaries and case-study MDX after removing their imports. Remove newly unused keys/styles only after checking references.
- [x] Update four-language test loops without deleting retained-language/business coverage. Keep purposeful retired-code and generic Unicode inputs.
- [x] Run core/views/Web/Desktop focused tests and typechecks as an integrated slice.

Checkpoint: both clients and website expose only two languages and handle stale local/server preferences.

## Slice 3 — Public docs

Owners: `apps/docs/lib/i18n.ts`, `lib/translations.ts`, `app/api/search/route.ts`, `next.config.mjs`, retired `content/docs/` resources and affected language tests/styles.

- [x] Add redirect and retained-locale tests; verify every inventoried old slug has an English counterpart.
- [x] Add permanent config-relative `/ja/:path*` and `/ko/:path*` → `/:path*` rules under the existing `/docs` basePath. Cover roots, nested paths, queries and loops.
- [x] Restrict docs UI/config/search to English/Chinese, remove retired MDX/metadata and adjust derived route/site tests.
- [x] Build/start docs on a verified free port and verify actual 308 → 200 for known pages, search, language controls, sitemap and hreflang.

Checkpoint: known old docs links work in English; no active retired language pages/search indexes remain.

## Slice 4 — Maintenance and final verification

- [x] Update English/Chinese conventions, `.trellis/spec/views/frontend/builtin-skill-localization.md`, current four-language comments and affected built-in skill contracts. Preserve historical release notes.
- [x] Update language guidance in `server/internal/service/builtin_skills/multica-autopilots/SKILL.md`, `multica-projects-and-resources/SKILL.md` and their source maps; inspect other current built-in contracts for retired advertisements.
- [x] Regenerate Chinese help if its source changes; review intentional generated diffs and preserve slugs/anchors.
- [x] Run integration checks below and the Desktop/docs runtime matrix. Record every AC in `verification.md`; obtain independent compatibility/deletion review.
- [x] Report changed files, simplifications, evidence and remaining limitations. Follow Lore/commit rules when committing is authorized; do not push/deploy or archive incomplete work.

## Commands for future execution

These commands have not been run during planning. Run from the task-owned checkout. Create the proposed test files first, or substitute an equivalent existing test owner when reused.

```bash
pnpm --filter @multica/core exec vitest run i18n/pick-locale.test.ts i18n/user-locale-sync.test.tsx --maxWorkers=2
pnpm --filter @multica/views exec vitest run locales/parity.test.ts locales/mcp.test.ts settings/components/preferences-tab.test.tsx onboarding/templates/index.test.ts agents/create/use-role-templates.test.ts workspace/slug.test.ts --maxWorkers=2
pnpm --filter @multica/web exec vitest run lib/locale-routing.test.ts lib/docs-href.test.ts lib/use-case-locale-fallback.test.ts --maxWorkers=2
pnpm --filter @multica/desktop test --maxWorkers=2
pnpm --filter @multica/docs test --maxWorkers=2
pnpm exec turbo run typecheck --filter=@multica/core --filter=@multica/views --filter=@multica/web --filter=@multica/desktop --filter=@multica/docs --concurrency=1
```

Regenerate Chinese help before tests whenever its source changed, so the bundle drift test sees the intended content:

```bash
pnpm generate:docs-bundle
```

The final integration command uses existing isolation: bounded TS tests, lint/typecheck, isolated Go/API databases, Go static/race checks, production Web build/start and selected browser specs. Do not target the current development database or a Next development server.

```bash
bash scripts/check.sh e2e/localized-template-defaults.spec.ts e2e/squads-design.spec.ts e2e/retained-languages.spec.ts --project=chromium
```

Do not also run `make check` reflexively once equivalent checks pass. Additional required generation/build/static checks:

```bash
pnpm --filter @multica/desktop build
pnpm --filter @multica/docs build
pnpm knip
git diff --check
```

Inspect Chinese bundle diffs against intentional source edits. Triage newly introduced knip findings without deleting unrelated code. Run builds sequentially within a checkout. Start the built docs server using `pnpm --filter @multica/docs exec next start --port <verified-free-port>` after substituting an actual free port; stop only that owned server.

Inspect retirement references with `rg` over packages/apps/handler/service/E2E, excluding generated/dependency directories. Do not demand zero occurrences of `ja`/`ko`: compatibility code, regression inputs, historical records and generic text handling are required exceptions.

## Ownership, review and completion

Default: one integration owner. After contracts stabilize, backend and docs can run as bounded native-agent lanes with exclusive file ownership; the integration owner owns shared client types/views/Web/Desktop. All agents preserve others' edits. No OMX runtime is required.

A separate reviewer checks stale preference precedence, reload stability, alias acceptance without weaker validation, deferred configuration, content preservation and docs basePath behavior. Estimated total effort is **2–3 person-days**, including tests; source deletion is mechanical.

Planning is complete after artifact/reference validation and independent review, while task status remains `planning`. Implementation is complete only after AC1–AC10 have recorded evidence. No test or build success is implied by this plan.
