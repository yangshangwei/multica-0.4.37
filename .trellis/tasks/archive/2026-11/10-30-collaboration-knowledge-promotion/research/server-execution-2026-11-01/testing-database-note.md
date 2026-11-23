# Explicit test database requirement

`go test -run '^$'` executes package `TestMain`; it is not a compile-only check. C2's two early handler checks omitted the dedicated environment. Exact commands, timestamps, inferred default target and the limits of cleanup evidence are in `default-db-incident.json`. Main's independent read-only post-state audit is in the primary task's `research/test-database-incident-2026-11-01/main-readonly-audit.json`; it does not establish pre-incident contents or historical SQL execution.

The handler and production-router `TestMain` functions now require a nonblank explicit `DATABASE_URL` before pool, fixture, handler or router construction. Missing and whitespace-only values exit 2 with a clear diagnostic; both literal default-database fallbacks are removed. Compile-only work uses `go build` or `go test -c`.

For every actual C2 DB suite:

```sh
source /tmp/multica_knowledge_c2_20261101_7d95f46f.env
test -n "$DATABASE_URL"
go test <explicit package/test selection>
```

Run service, handler and production-router DB suites sequentially on this exclusive database. A2 and F3 keep their own databases. Never perform additional cleanup/restoration on the default database as part of this task.

The guard regressions re-execute the newly guarded test binary with `DATABASE_URL` removed or whitespace-only. They require the early exit-2 diagnostic and reject evidence of database/router initialization. Each child has a five-second `CommandContext`, a nonexistent private `PGHOST` socket directory, and `PGPORT=1`. Because a literal URI overrides host/port environment settings, inherited `PGOPTIONS` is replaced with `-c default_transaction_read_only=on -c statement_timeout=2000`. Config-only pgx assertions prove those startup options survive an explicit URI; they open no connection. The unsafe old literal-fallback path was not re-executed for a red test; the incident is the failing-before evidence. Parent suites run only with the dedicated C2 environment.
