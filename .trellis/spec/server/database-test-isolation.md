# Database test isolation

## 1. Scope / trigger

The handler and server-router test packages initialize shared fixtures in `TestMain`. A test-name filter does not suppress this initialization. Run these packages only against an explicitly selected, task-owned database.

## 2. Commands and entry points

- `server/internal/handler/handler_test.go:TestMain`
- `server/cmd/server/integration_test.go:TestMain`
- Execute tests after loading the task's private environment file; verify the database identity without printing its credentials.
- Compile without execution with `go -C server build ./...` or `go -C server test -c -o "$TASK_TEST_BINARY" ./internal/handler`.

## 3. Environment and execution contract

Both entry points require nonblank `DATABASE_URL` before creating a pool, running fixture cleanup/setup, or constructing a router. Missing or whitespace-only values print a content-free configuration error and exit **2**. Do not restore an implicit localhost database fallback.

An explicit URL is not proof of ownership. The caller must select its recorded test database, keep credentials in the private environment file, and serialize service/handler/router suites that share a database. Browser fixtures use their own database. Different workspace IDs do not isolate singleton gates or package-global fixture cleanup.

## 4. Validation and error matrix

| Input / operation | Required result |
| --- | --- |
| `DATABASE_URL` absent or whitespace | Exit 2 before pool/setup/router; no database connection |
| Explicit task database | Normal fixture lifecycle and selected tests; record actual run/pass/skip inventory |
| `-run '^$'` | Still executes `TestMain`; never use as a compile-only check |
| `go build` / `go test -c` | Compiles without executing `TestMain` |
| Explicit but unreachable database | Existing connection handling remains; exit status alone never proves tests ran |

## 5. Good / base / bad cases

- Good: load the recorded private test environment, verify its database name, execute one suite, and confirm all expected names actually ran.
- Base: use `go build` or `go test -c` when only compilation is needed.
- Bad: an empty test filter against an older revision with no explicit environment can initialize and clean the default database even though no test body runs. Never rerun that unsafe path merely to obtain red evidence.

## 6. Required regressions

`TestHandlerTestMainRequiresExplicitDatabase` and `TestRouterTestMainRequiresExplicitDatabase` execute newly guarded test binaries with absent and whitespace-only configuration. They require exit 2 and the expected configuration error, without initialization output.

The child processes have a bounded context, a nonexistent socket, and forced `PGOPTIONS` with `default_transaction_read_only=on` and a statement timeout. Config-only pgx checks prove the read-only option survives an explicit URI; they do not connect. This also protects against a future reintroduced literal fallback. Parent suites still require a dedicated database.

## 7. Wrong vs correct

Wrong compile probe:

```sh
go -C server test ./internal/handler -run '^$'
```

Correct compile probe:

```sh
go -C server build ./internal/handler
go -C server test -c -o "$TASK_TEST_BINARY" ./internal/handler
```

For an accidental execution, preserve command/time/environment evidence and perform only a bounded read-only audit. Current absence of reserved fixture rows cannot prove their previous contents were untouched; do not perform speculative cleanup or restoration.
