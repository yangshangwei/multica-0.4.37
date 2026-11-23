import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repository = fileURLToPath(new URL("../", import.meta.url));
const sentinel = "synthetic-precondition-password";
const packages = [
  { name: "handler", path: "./internal/handler", diagnostic: "handler tests", required: true },
  { name: "router", path: "./cmd/server", diagnostic: "router integration tests", required: true },
  { name: "testutil", path: "./internal/testutil", diagnostic: "testutil DB fixture tests", required: false },
];

// TestMain executes even with -test.run=^$. Each child must have no route to
// inherited database credentials or a real server; all configured URLs below
// name a nonexistent Unix socket inside this test's private directory.
function childEnvironment(directory, databaseURL) {
  const env = Object.fromEntries(Object.entries(process.env).filter(([key]) =>
    !key.startsWith("PG") && key !== "DATABASE_URL" && key !== "REDIS_TEST_URL"));
  env.TMPDIR = directory;
  env.TMP = directory;
  env.TEMP = directory;
  env.PGHOST = join(directory, "s");
  env.PGPORT = "1";
  env.PGPASSFILE = join(directory, "no-pgpass");
  env.PGSERVICEFILE = join(directory, "no-pgservice");
  env.PGOPTIONS = "-c default_transaction_read_only=on -c statement_timeout=2000";
  env.GOMAXPROCS = "2";
  if (databaseURL !== undefined) env.DATABASE_URL = databaseURL;
  return env;
}

test("database test entry points reject invalid configuration before fixture setup", { timeout: 900_000 }, async (t) => {
  assert.notEqual(process.platform, "win32", "this offline socket fixture requires Linux or macOS");
  const parent = resolve(tmpdir());
  const gitRoot = spawnSync("git", ["-C", parent, "rev-parse", "--show-toplevel"], { encoding: "utf8" });
  assert.ifError(gitRoot.error);
  assert.notEqual(gitRoot.status, 0, "test state must be outside every Git worktree");
  const directory = mkdtempSync(join(parent, "m-db-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const env = childEnvironment(directory);
  const unreachable = new URL("postgres:///guard");
  unreachable.searchParams.set("user", "guard");
  unreachable.searchParams.set("password", sentinel);
  unreachable.searchParams.set("host", env.PGHOST);
  unreachable.searchParams.set("port", env.PGPORT);
  unreachable.searchParams.set("sslmode", "disable");
  unreachable.searchParams.set("connect_timeout", "2");

  for (const pkg of packages) {
    const binary = join(directory, `${pkg.name}.test`);
    const built = spawnSync("go", ["test", "-p", "2", "-c", "-o", binary, pkg.path], {
      cwd: resolve(repository, "server"), env, encoding: "utf8", timeout: 240_000, maxBuffer: 4 * 1024 * 1024,
    });
    assert.ifError(built.error);
    assert.equal(built.status, 0, `compile ${pkg.path}: ${built.stdout}${built.stderr}`);

    const cases = [
      { name: "absent", url: undefined },
      { name: "whitespace", url: " \t\r\n" },
      { name: "malformed", url: `postgres://guard:${sentinel}@[invalid` },
      { name: "unreachable", url: unreachable.href },
    ];
    for (const entry of cases) {
      await t.test(`${pkg.name}: ${entry.name}`, () => {
        const optional = !pkg.required && (entry.name === "absent" || entry.name === "whitespace");
        if (optional) {
          // A regression restoring the former hardcoded localhost fallback
          // must fail without executing that unsafe startup path.
          const source = readFileSync(resolve(repository, "server/internal/testutil/db_test.go"), "utf8");
          assert.doesNotMatch(source, /postgres(?:ql)?:\/\//,
            "optional TestMain cannot be exercised while it has a hardcoded database URL");
        }
        const run = optional
          ? "^(TestJSONRequestEncodesValuesAndPassesRawBodiesThrough|TestBuildersInsertRowsTheSchemaAccepts)$"
          : "^$";
        const result = spawnSync(binary, ["-test.v", `-test.run=${run}`], {
          cwd: directory, env: childEnvironment(directory, entry.url), encoding: "utf8", timeout: 15_000,
        });
        assert.ifError(result.error);
        const output = result.stdout + result.stderr;
        assert.ok(!output.includes(sentinel), "database diagnostics leaked a credential");
        if (optional) {
          assert.equal(result.status, 0, output);
          assert.match(output, /DATABASE_URL is not configured; DB fixture tests are disabled/);
          assert.match(output, /--- PASS: TestJSONRequestEncodesValuesAndPassesRawBodiesThrough/);
          assert.match(output, /--- SKIP: TestBuildersInsertRowsTheSchemaAccepts/);
          return;
        }
        assert.equal(result.status, 2, `${pkg.name} must reject ${entry.name}: ${output}`);
        assert.ok(output.includes(pkg.diagnostic), output);
        assert.match(output, /explicit DATABASE_URL|invalid DATABASE_URL|database connection failed/);
        assert.doesNotMatch(output, /=== RUN|--- PASS|Skipping|llm retry policy|no tests to run/,
          "test or application initialization ran after a failed database precondition");
      });
    }
  }
});
