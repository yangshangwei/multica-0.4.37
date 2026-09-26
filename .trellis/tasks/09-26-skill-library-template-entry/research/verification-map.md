# Verification map: skill library template entry

Status: planning research only. Commands below were checked against repository scripts and test sources; none were executed. No services were started and no application files were changed.

## Canonical test ownership

Shared behavior belongs in `packages/views/`, not app-specific test files. Keep parsing, schema, and state-transition matrices in their existing core suites; component tests should cover the user action, wiring, accessibility, and named regressions.

| Existing path | What it protects / how this task should use it |
| --- | --- |
| `packages/views/skills/components/skills-page.test.tsx` | Catalog/page entry wiring, independent workspace search, counts, official/provenance-backed instance navigation, localized workspace presentation. Update obsolete catalog-disclosure expectations to the approved entry design while retaining the underlying identity and search invariants where applicable. |
| `packages/views/skills/components/create-skill-dialog.test.tsx` | Creation-method order, other creation/import paths, initial category. Protect the existing `New skill` entry when adding a direct template-browser entry. |
| `packages/views/skills/components/create-skill-template-flow.test.tsx` | Preview without writes; explicit template adoption; independent editable copy; dirty-draft preservation/discard; conflict and uncertain-result recovery; locale/catalog/workspace changes. Extend the entry-point happy path here rather than copying the full matrix elsewhere. |
| `packages/views/skills/components/template-skill-create-panel.test.tsx` | Built-in versus deployment source tabs, source counts, search, selection, and deployment-empty state. Current locale coverage is English and Simplified Chinese; it is not an all-four-locale UI suite. |
| `packages/views/common/builtin-template-catalog.test.tsx` | Shared disclosure keyboard behavior and compact default. Still protects other consumers if this task only removes the skill-page use; do not rewrite its contract solely to accommodate one page. |
| `packages/views/skills/components/skill-card.test.tsx` | Card presentation/selection. Run or extend if card title navigation, accessible naming, or card layout changes are included. |
| `packages/views/skills/components/skill-category-sidebar.test.tsx`, `skill-category-chips.test.tsx` | Category controls and their counts. Include if layout/filter navigation is actually modified. |
| `packages/views/skills/lib/skill-presentation.test.ts` | All four locale catalogs, official-skill identity, bilingual search, custom-copy preservation, and synchronization with embedded source descriptions. Required for skill presentation/copy changes. |
| `packages/views/locales/parity.test.ts` | All registered locale namespaces/keys and valid plural forms. Required when translation keys are added, removed, or renamed. |
| `e2e/skill-template-creation.spec.ts` | Real API catalog → preview → cancel → adopt → edit → create. No interception of catalog/create; verifies exactly one write, independent identity, metadata/frontmatter/files, unchanged source/existing workspace copy/members/agents. Captures 1280×720 and 375×667 states and keyboard/viewport behavior. Extend to enter through the new template entry while retaining the `New skill` route coverage as appropriate. |
| `e2e/localized-template-defaults.spec.ts` | Focused test `specialist skills localize, search and open their official copies without rewriting stored content` covers English/Chinese purpose search and navigation with zero skill writes. Its current navigation explicitly expands the inline catalog and presses `Open skill`; replacing that catalog requires updating this obsolete route to the approved workspace-instance action while preserving its identity/no-write assertions. |
| `e2e/skill-category-taxonomy.spec.ts` | Real category counts, card/list modes, selected/hover state, wide/narrow and light/dark presentation, batch labels, English/Chinese labels. Broader visual regression coverage if the page shell or collection layout changes; not needed merely for planning files. |

There is no `e2e/skills.spec.ts` in the inspected checkout. Do not invent that path in the implementation plan.

## Focused runnable commands

All commands here are run from the repository root. `pnpm --filter @multica/views exec ...` uses the views package working directory, so test arguments below are package-relative. This avoids passing file filters through the root Turbo test wrapper.

### Entry and dialog behavior

```bash
pnpm --filter @multica/views exec vitest run skills/components/skills-page.test.tsx skills/components/create-skill-dialog.test.tsx skills/components/create-skill-template-flow.test.tsx skills/components/template-skill-create-panel.test.tsx --maxWorkers=2
```

### All four locales and source synchronization

```bash
pnpm --filter @multica/views exec vitest run locales/parity.test.ts skills/lib/skill-presentation.test.ts --maxWorkers=2
```

The four supported catalogs are `en`, `zh-Hans`, `ja`, and `ko`. The presentation suite reads the actual files under `server/internal/service/builtin_role_skills/`, checks that every embedded skill is represented in all four catalogs, exercises one-locale-only providers for each locale, and checks the Chinese default description against each source `SKILL.md`. These are distinct from generic key parity; run both suites. Neither proves that new labels are idiomatic or fit the available space. Review all four strings and add a small all-four-locale rendering assertion for the new entry/heading if the task changes those labels; do not multiply the existing draft/recovery matrix by four.

### Conditional shared-control regressions

```bash
pnpm --filter @multica/views exec vitest run common/builtin-template-catalog.test.tsx skills/components/skill-card.test.tsx skills/components/skill-category-sidebar.test.tsx skills/components/skill-category-chips.test.tsx --maxWorkers=2
```

Use the relevant files if their behavior or shared shell changes. Removing only the skill-page catalog invocation does not require changing the shared disclosure or tests for its other consumers.

### Lint, type checking, static inspection

```bash
pnpm --filter @multica/views lint
pnpm --filter @multica/views typecheck
pnpm exec turbo run typecheck --filter=@multica/web --filter=@multica/desktop --concurrency=1
```

The consumer typecheck command preserves Turbo's `mdx` prerequisite; shared raw TS/TSX is consumed by both web and desktop. Run the consumer check once after the shared implementation is stable. Inspect removed component imports, exports, and translation-key references with `rg` as part of the final diff review. `pnpm knip` is the repository dead-code command if deletion/export changes warrant its broader analysis; it is not a substitute for checking concrete references.

If implementation changes core template logic or the API boundary, the existing canonical suites can be run directly:

```bash
pnpm --filter @multica/core exec vitest run skills/template-draft.test.ts api/skill-template-schemas.test.ts api/skill-template-client.test.ts workspace/skill-template-queries.test.ts --maxWorkers=2
```

A views-only entry/layout change should reuse these contracts and need not modify or duplicate their matrices.

## Production browser validation

`playwright.config.ts` starts no servers. It defines Chromium, one worker, zero retries, a 60-second default timeout, and chooses `PLAYWRIGHT_BASE_URL`, then `FRONTEND_ORIGIN`, then `http://localhost:3000`. `e2e/env.ts` loads `.env.worktree` before `.env` without overriding already-exported values. Use the environment launcher to keep these values aligned with the API and database.

For an already task-owned, configured checkout environment:

```bash
make up C=api,web ARGS="--web-mode production"
make status ARGS="--json"
make env-exec ARGS="-- pnpm exec playwright test e2e/skill-template-creation.spec.ts --project=chromium --workers=1 --retries=0"
make env-exec ARGS="-- pnpm exec playwright test e2e/localized-template-defaults.spec.ts --grep 'specialist skills localize' --project=chromium --workers=1 --retries=0"
```

If the collection shell/layout changes, additionally run:

```bash
make env-exec ARGS="-- pnpm exec playwright test e2e/skill-category-taxonomy.spec.ts --project=chromium --workers=1 --retries=0"
```

After recording evidence, use `make down` only for the environment this task owns and started. Do not stop an unrelated listener to free a port. Do not use `make destroy` as routine test cleanup; it deletes environment data.

For a fully isolated verification environment, `scripts/check.sh` accepts Playwright file/CLI arguments and forwards them only to its final step:

```bash
bash scripts/check.sh e2e/skill-template-creation.spec.ts e2e/localized-template-defaults.spec.ts e2e/skill-category-taxonomy.spec.ts --project=chromium
```

This still runs all static checks, all TypeScript package tests, script regressions, isolated Go race tests and vet, API startup, and production Web build before all three required Playwright files. This alternative runs the complete localized-template-defaults spec rather than only its specialist-skills test. It is a broad verification run with a focused browser phase, not a quick unit-test command. The script selects the environment file through its normal detection unless `ENV_FILE` is explicitly supplied. Supply the actual task-configured file if both `.env` and `.env.worktree` exist and selection would be ambiguous.

The full repository gate is:

```bash
make check
```

The current `Makefile` recipe does **not** forward `ARGS` to `scripts/check.sh`; `make check ARGS="e2e/skill-template-creation.spec.ts"` does not narrow the Playwright phase. Use the direct script invocation above when limiting the browser phase to these three required files is intended. Do not run both pipelines reflexively after the required checks already pass.

## Environment prerequisites and provenance

- Node ≥22, pnpm 10.28.2 as declared by the root package, installed workspace dependencies, and the installed Playwright Chromium browser are prerequisites. The full isolated pipeline explicitly checks `node`, `go`, `pnpm`, `psql`, `lsof`, and `make` on PATH. This research did not inspect actual tool availability or install anything.
- Use a task-owned PostgreSQL database at the endpoint in `DATABASE_URL`. `TestApiClient` directly reads/deletes verification-code rows and updates onboarding fields, so matching only the browser URL is insufficient. The API URL, Web build URLs, and database must belong to the same task environment. The PostgreSQL role needs database-creation permissions for `check.sh`; migrations must succeed. A same-named Docker database does not prove the configured endpoint is correct.
- The skill-template E2E fixture expects an embed-only catalog of exactly 15 templates and the deployment source count to be zero. Keep `MULTICA_SKILL_TEMPLATE_DIR` empty/unset in the task API environment for this spec; do not remove legitimate deployed templates to make the fixture pass. The server reads this setting at startup.
- Classic verification-code authentication must be available for `TestApiClient`. `check.sh` copies the selected env file into its own task environment and appends `MULTICA_DEVICE_AUTH_ENABLED=false`, a development verification code, `APP_ENV=development`, and task-only auth-rate-limit overrides. The production-mode requirement concerns the Web build; it does not require production authentication configuration for the disposable API. A focused `make up` environment must already be configured equivalently rather than assuming those `check.sh` overrides happen automatically.
- If Redis activates per-IP auth limits, configure `RATE_LIMIT_AUTH` and `RATE_LIMIT_AUTH_VERIFY` for the task-only API's synthetic-account throughput. Do not change production defaults. Full Go Redis integration coverage also needs `REDIS_TEST_URL` on a separate Redis instance because fixtures flush their assigned logical databases.
- Use `next build` + `next start`, never `next dev` / `pnpm dev:web`, for Playwright verdicts. The launcher supplies `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_WS_URL`, and `REMOTE_API_URL` during production setup. Existing comments in `skill-template-creation.spec.ts` and `skill-category-taxonomy.spec.ts` mention accommodating cold dev compilation; those comments do not override `.trellis/spec/web/frontend/e2e-run-environment.md`.
- Run only one Web build/check per checkout. Different ports still share `apps/web/.next`; registered Web processes and the checkout build lock guard this. When an owned production build becomes stale after source changes, stop that explicitly owned environment before rebuilding. Never kill an unknown process or weaken source/commit/build/configuration checks.
- Preserve `api.running.json`, `web.running.json`, and, for `check.sh`, `verification.running.json` from `~/.multica/dev/envs/<name>/` (or `MULTICA_DEV_HOME`) with Playwright results. Record command, commit, mode, source fingerprint and build identity. The template E2E attaches screenshots plus a `verification.json`; combine these artifacts with service provenance.
- `check.sh` allocates a separate `check-*` environment with independent API and Go databases, runs phases sequentially, stops only its own processes, drops the isolated Go database without forced disconnects, and retains API data/registry evidence until TTL collection. Cleanup failure is a failing run.

## Planning-only verification versus future implementation

For the current planning deliverable, review artifact completeness, acceptance-criteria/test mapping, file references, command accuracy, locale scope, and consistency among PRD/design/implementation/test-spec documents. A scoped whitespace/diff review is appropriate. No lint, typecheck, unit tests, build, Go tests, Playwright, database setup, or service launch is warranted solely for these Markdown research/planning files. Report them as **not run — planning only**, never as passed.

For implementation, first update the canonical tests for changed behavior, run the focused views and locale/source suites, then lint/typecheck and the relevant production-browser flow. Add category/card regressions if those controls change. Use the full `make check` gate when required for release/integration or warranted by expanded cross-layer changes. No backend migration, real-agent smoke test, Electron packaging, or full mobile run follows from this scoped template-entry task alone.

## Evidence sources

- `CLAUDE.md`, root/`packages/views`/`packages/core`/`apps/web`/`apps/desktop` package scripts, and `turbo.json`.
- `Makefile`, `scripts/check.sh`, and `scripts/dev-env.sh`.
- `.trellis/spec/web/frontend/e2e-run-environment.md`, `playwright.config.ts`, `e2e/env.ts`, and `e2e/fixtures.ts`.
- Canonical test files listed above and the server's `MULTICA_SKILL_TEMPLATE_DIR` wiring.
