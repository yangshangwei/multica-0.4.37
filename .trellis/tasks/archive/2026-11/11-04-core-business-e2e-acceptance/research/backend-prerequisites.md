# Backend execution prerequisites

Current candidate has 82 default Go packages and 10,413 statically declared Test/Fuzz names selected for this platform. This is an inventory, not a pass result.

- Local toolchain: Go 1.27.0 darwin/arm64; CI uses latest Go 1.26 patch. Keep the toolchain difference explicit.
- One migrated task database per package, allocated by root; package and actual database/schema/empty-issue identity checked immediately before testing.
- Default run uses CLI execution guard, clean inherited environment, per-package private HOME/TMP, explicit Go caches, GOMAXPROCS=2, -race -count=1 -p1 -parallel2 -timeout=40m.
- Dedicated Redis is required for REDIS_TEST_URL and MULTICA_TEST_REDIS_URL; selected instance must not be shared with API or another test worker.
- Package tests themselves include scoped scratch schema/database lifecycle in cmd/migrate, cmd/backfill_codex_usage_cache and internal/taskusagebackfill. Root must include these existing test lifecycles in the exact package permits.
- Default Go run excludes agentintegration and knowledgeintegration tags. Real-agent tags additionally require MULTICA_RUN_REAL_AGENT_SMOKE=1; no unspecified account/paid usage is authorized.
- Performance gates: internal/handler TestIterationI1P95 and TestIterationI1DisableP95 require MULTICA_RUN_I1_P95=1 and a fresh exclusive DB, without race. TestTriageQueueScaleBaseline requires MULTICA_TRIAGE_PERF=1.
- Conditional Linux/Windows source and native behavior must run on actual selected OS; compile-only output does not establish execution.
- All first failures and conditional skip reasons remain retained. A silent TestMain exit or missing expected test name is incomplete execution even if exit code is zero.
