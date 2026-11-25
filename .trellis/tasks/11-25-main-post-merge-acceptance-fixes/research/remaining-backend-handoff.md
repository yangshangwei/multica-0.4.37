# Research: Remaining backend review handoff

- Query: Bounded source review of the new remaining-backend runner.
- Scope: internal; stopped when the parent assigned this review to another independent checker.
- Date: 2026-11-25

## Findings

Two concrete observations are handed off; no overall verdict or duplicate inventory audit:

1. `.omx/private/acceptance-fixes/remaining_backend.py:95` sets only `REDIS_TEST_URL`. `server/internal/auth/password_limiter_integration_test.go:18–20` requires `MULTICA_TEST_REDIS_URL`, otherwise skips. The inspected completed auth normal receipt reported116 PASS/1 SKIP. Configure the actual opt-in variable for its separately owned Redis destination and preserve/reconcile this skipped run; do not count it as shared-limiter coverage.
2. URL database indices40+ are overridden by existing helpers: auth `pat_cache_test.go:12,27` uses11; middleware `auth_test.go:16,32` uses13; WeCom `relay_ordering_db_test.go:48,65` uses15. Each helper flushes its selected DB. These packages are serialized inside this runner, so this observation alone does not prove a collision; coordinate the real indices with other lanes and record their actual allocation. Do not rely on the URL index to establish separation.

## Files found / related specs

- `.omx/private/acceptance-fixes/remaining_backend.py` — package selection, per-package PG clones, environment and result recording.
- `.omx/private/acceptance-fixes/allocate.py` — verified-owner PG prewrite helper called by the runner.
- Above Redis tests — actual opt-in names, overridden indices and flush behavior.
- `.trellis/spec/server/database-test-isolation.md` — process environment/fixture identity and cross-package resource isolation.

## Caveats / Not Found

- Parent reassigned this review; remaining runner correctness, runtime assets and full inventories are owned by the new independent checker.
- No runner/test/service/Docker/SQL/Git execution or product edits by this reviewer. Findings came from source and existing receipt reads only.
- No external references required. Runtime final03 completion is parent-reported here and remains for later receipt verification in final integration.
