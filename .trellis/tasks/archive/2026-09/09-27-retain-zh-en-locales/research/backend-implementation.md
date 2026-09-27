# Backend implementation evidence

Implemented in `/Volumes/artisan/code/2026/multica-retain-zh-en-locales`, branch
`work/retain-zh-en-locales`. No original-checkout edits, commits, deployment,
schema migration, bulk data rewrite or real agent execution.

## Changes

- Added one handler-local exact-alias normalizer for `ja`/`ko` → `en`.
  User preferences retain `zh-Hans`; content APIs retain `zh`.
- User response mapping normalizes a local value copy. GET leaves stored values
  and timestamps unchanged. Explicit legacy PATCH saves English; omitted/null
  language leaves the stored value unchanged. Each endpoint retains its existing
  whitespace/invalid-input rules.
- Mika creation and onboarding normalize aliases before map access. Opening and
  kickoff use English, retries preserve existing identity and user-owned text.
- Deferred project-squad singleton and array JSON normalizes in memory;
  materialization and retries preserve revisions and customized instances.
- Removed Japanese/Korean catalog and Mika copy, preserving English fallback,
  canonical instructions, template keys and versions.
- Updated server template/project specs and affected built-in skill/source maps.

## Verification

An isolated database `multica_locale_backend_1790477109326` was created via existing
`scripts/dev-env.sh` helpers and migrated successfully (480 migrations).
The test runner used `scripts/go-test-with-agent-cli-guard.sh`; every Go
test command explicitly set the isolated DATABASE_URL.

1. **Red:** new language, Mika, deferred-squad and catalog regressions failed
   against the original implementation for the expected retired-language
   behavior. Both packages compiled and used the database.
   Command: `go test -p 2 -parallel 2 ./internal/handler ./internal/service -run 'Test(UpdateMeAccepts(Korean|Japanese)Language|UserLanguage|CreateMikaAgent_Retired|StartMikaOnboarding_Retired|ProjectExecutionSquad_Retired|TemplateLanguagesRetain|TemplateCatalogsRetired)' -count=1 -v`.
   Exit 1; log: `/tmp/multica_locale_backend_1790477109326/red.log`.
2. **Green:** focused retained/retired locale, template, Mika, project-squad and
   profile suites passed after implementation.
   Command: `go test -p 2 -parallel 2 ./internal/handler ./internal/service -run 'Test.*(Language|Template|Mika|ProjectExecutionSquad|UpdateMe|GetMe)' -count=1 -v`.
   Exit 0; handler 3.002s, service 1.491s.
   Log: `/tmp/multica_locale_backend_1790477109326/green.log`.
3. **Full affected packages:** `go test -p 2 -parallel 2 ./internal/service -count=1`
   passed (8.246s); `go test -p 2 -parallel 2 ./internal/handler -count=1`
   passed (52.300s). Logs: `/tmp/multica_locale_backend_1790477109326/service-full.log`,
   `/tmp/multica_locale_backend_1790477109326/handler-full.log`.
4. **Static:** `go vet -p 2 ./internal/handler ./internal/service` exited 0.
   `gofmt` applied to changed Go files; scoped `git diff --check` passed.

This proves backend AC4–AC5 and relevant preservation/invalid-input behavior.
Integrated client, race, full-repository and runtime checks remain with the
integration owner. The isolated test database was removed without FORCE after the docs
embed/handler checks passed.
