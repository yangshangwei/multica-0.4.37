# 小阿孚 rollout repair verification

## Result
The user's running Electron development app now shows 小阿孚 in the chat sidebar, header, welcome, composer, agent guidance/list and detail. The actual local database's five built-in agents and five pristine onboarding pairs were repaired. No user message was sent during verification.

## Root causes
1. Desktop renderer :5666 used API :18572, commit cfae7477e; its database `multica_multica_0_4_37_492` had migration 456, not 457. Client-side title copy had updated independently.
2. Shipped migration 457 intentionally left all messages unchanged, including unused product-authored onboarding openings. Added 458 to repair only exact pristine default pairs.
3. Eight product display strings in the Web landing changelog still named Mika (four English, four Chinese).

## Changed files and approach
- `server/migrations/458_builtin_onboarding_display_name.{up,down}.sql`: guarded atomic update under the first-send session lock; down is a no-op.
- `server/internal/migrations/builtin_onboarding_display_name_migration_test.go`: exact English/Chinese defaults, 22 boundary cases, preservation/replay/down, real first-send race and injected failure.
- `apps/web/features/landing/i18n/{en,zh}.ts`: eight editorial branding references updated.
- `.trellis/spec/server/builtin-templates.md`: pristine onboarding exception and runtime rollout verification contract.

Reuses the existing migration runner and stable identity instead of adding a UI name substitution or dependency. No layout/component logic changed.

## Code audit
Shared Web/Desktop chat header, transcript preview, new-chat composer, agent list/detail, guidance and pickers render the saved agent name/description. Current onboarding labels and backend creation/system instructions already use the new default/current saved name. Web, Desktop, Mobile, docs and packages were searched; backend audit covered built-in templates, prompts, creation/reuse, onboarding and migration paths. Remaining technical `mika` references are identity/API routes/symbols, comments or tests. Custom names and genuine historical messages remain intact intentionally.

## Verification evidence
Evidence directory: `.omx/reports/xiaoafu-rollout-repair/`.

- Observed red PostgreSQL test: existing 457 leaves the exact old product greeting unchanged (`onboarding-migration-red.log`).
- Full migrations package `go test -p 2 ./internal/migrations -count=1 -race -v`: PASS, no skips; includes 22 boundary cases, first-send concurrency and all-or-nothing failure.
- Guarded scoped Go handler/service/migrate tests: all three packages PASS, including creation/onboarding/skills and migration notice reporting.
- Core onboarding: 15 tests PASS. Shared views onboarding: 15 tests PASS. Web changelog: 3 tests PASS. Desktop: 65 files / 718 tests PASS. Total 751 TypeScript tests.
- Fresh nonmobile typecheck: 9 tasks PASS, no cache. Source lint: 6 tasks PASS, no cache, existing warnings retained. Changed Web files also pass direct ESLint.
- Default full lint initially found 18 existing errors only in ignored `apps/desktop/.gstack/squads-preview/{check,server}.mjs` scratch scripts. Source lint was rerun with `--ignore-pattern '**/.gstack/**'`; no lint config or unrelated files changed.
- Server build, `go vet ./cmd/migrate ./internal/migrations`, UI wildcard exports, task manifests and `git diff --check`: PASS.
- Actual migration CLI applied 457 and 458 to the proven local database. API :18572 was rebuilt from the verified source (initially 4924ac74c); final post-commit identity is recorded in `runtime-final.json`. All five builtin names/descriptions verified through PostgreSQL.
- All ten updated onboarding rows retain their exact IDs, creation timestamps and non-content metadata; only the self introduction and its paired quote changed (`local-after-summary.json`). Backup stored locally with mode 0600, not committed.
- Native Electron DOM assertions: chat/sidebar/welcome/starter cards, agents guidance/list, detail and new-chat composer PASS; no page errors. Final chat/list screenshots captured after entrance animation and under dark emulation; visual verdict 96/pass.

## Limits
No external deployment, packaged desktop installer or full live-agent/model invocation. Mobile was source-audited, not launched. Existing custom content and real conversation history are deliberately not rewritten. Broader unrelated server suites were not run. Unrelated pre-existing script/type declaration/critique changes remain untouched.
