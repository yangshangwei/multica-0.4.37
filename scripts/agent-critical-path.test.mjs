import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { test } from "node:test";
import { criticalPathCases } from "./check-agent-critical-path.mjs";
import { redactSmokeLog, rejectAmbientDotenv, smokeEnvironment, validateSmokeConfig } from "./agent-critical-path.mjs";

const root = resolve(import.meta.dirname, "..");
const config = {
  version: 1, owner: "agent-critical-path", databaseName: "owned_smoke_fixture",
  databaseUrl: "postgres://fixture:private-test-password@127.0.0.1:54321/owned_smoke_fixture?sslmode=disable",
  browserPath: "/explicit/browser-cache",
};

test("exact allocation with loopback database passes", () => assert.deepEqual(validateSmokeConfig(config), config));
for (const [name, changes] of Object.entries({
  absent: { databaseUrl: undefined },
  wrong_owner: { owner: "unrelated" },
  wrong_version: { version: 2 },
  unknown_field: { shell: "do not run" },
  default_db: { databaseName: "multica" },
  system_db: { databaseName: "postgres" },
  malformed_name: { databaseName: "owned;write" },
  wrong_db: { databaseName: "different_smoke" },
  wrong_host: { databaseUrl: config.databaseUrl.replace("127.0.0.1", "production.invalid") },
  wrong_protocol: { databaseUrl: config.databaseUrl.replace("postgres:", "https:") },
  absent_port: { databaseUrl: config.databaseUrl.replace(":54321", "") },
  port_zero: { databaseUrl: config.databaseUrl.replace(":54321", ":0") },
  connection_override: { databaseUrl: `${config.databaseUrl}&host=production.invalid` },
  duplicated_option: { databaseUrl: `${config.databaseUrl}&sslmode=disable` },
  encoded_control: { databaseUrl: config.databaseUrl.replace("fixture:", "fixture%0a:") },
  browser_relative: { browserPath: "auto" },
})) {
  test(`allocation refuses ${name} without credential disclosure`, () => {
    assert.throws(() => validateSmokeConfig({ ...config, ...changes }), (error) => !error.message.includes("private-test-password"));
  });
}

test("private child environment excludes inherited accounts and API settings", () => {
  const env = smokeEnvironment(config, "/owned/run", "/tool/bin", { GOCACHE: "/cache/go", GOMODCACHE: "/cache/mod", GOPATH: "/cache/path" });
  assert.equal(env.HOME, "/owned/run/home");
  assert.equal(env.GOCACHE, "/cache/go");
  assert.equal(env.DATABASE_URL, config.databaseUrl);
  assert.equal(env.PLAYWRIGHT_BROWSERS_PATH, config.browserPath);
  assert.equal(env.APP_ENV, "development");
  assert.equal(env.MULTICA_AUTH_MODE, "legacy");
  assert.equal(env.GOFLAGS, "-p=1");
  for (const key of ["MULTICA_TOKEN", "SMTP_PASSWORD", "OPENAI_API_KEY", "REDIS_URL", "PGSERVICE", "PGOPTIONS", "ENV_FILE", "NODE_OPTIONS", "MULTICA_RESOURCE_PUBLISH_DIR"]) {
    assert.equal(env[key], undefined, key);
  }
});

test("public log copy removes connection URL, password and synthetic signer key", () => {
  const text = `${config.databaseUrl} private-test-password signer-key postgres://else:secret@localhost/db`;
  const cleaned = redactSmokeLog(text, config, { JWT_SECRET: "signer-key" });
  for (const value of [config.databaseUrl, "private-test-password", "signer-key", "else:secret"]) assert.ok(!cleaned.includes(value));
});

test("dotenv examples allowed but independently loaded root/Web env files rejected", () => {
  for (const file of [".env", ".env.worktree", ".env.local", "apps/web/.env", "apps/web/.env.production.local"]) {
    const dir = mkdtempSync(join(tmpdir(), "critical-path-env-"));
    mkdirSync(join(dir, "apps/web"), { recursive: true });
    writeFileSync(join(dir, ".env.example"), "");
    rejectAmbientDotenv(dir);
    writeFileSync(join(dir, file), "SECRET=do-not-source");
    assert.throws(() => rejectAmbientDotenv(dir), /ambient dotenv/);
  }
});

function goodReport() {
  return {
    config: { workers: 1, projects: [{ id: "chromium", name: "chromium", retries: 0, repeatEach: 1 }] },
    errors: [], stats: { expected: 4, skipped: 0, unexpected: 0, flaky: 0 },
    suites: [{ specs: criticalPathCases.map(({file,title}) => ({ file, title, ok: true, tests: [{
      projectName: "chromium", projectId: "chromium", expectedStatus: "passed", status: "expected",
      results: [{status:"passed",retry:0,errors:[]}],
    }] })) }],
  };
}

// Drive the real shell orchestration with process boundaries replaced. No
// databases, application services, browser or provider executable are started.
const fixture = String.raw`#!/usr/bin/env bash
set -euo pipefail
source "$1/scripts/agent-critical-path.sh"
fixture_dir=$2
phase=$3
export MULTICA_SMOKE_RUN_DIR="$fixture_dir"
export MULTICA_SMOKE_DATABASE_NAME=owned_smoke_fixture
export DATABASE_URL=postgres://fixture:synthetic@127.0.0.1:1/owned_smoke_fixture
export PLAYWRIGHT_BROWSERS_PATH=/fixture/browser
record() { printf '%s\n' "$*" >> "$fixture_dir/calls"; }
psql() {
  record database
  case "$phase" in missing-db) return 2;; wrong-db) printf unrelated;; *) printf owned_smoke_fixture;; esac
}
lsof() { :; }
smoke_prepare_environment() {
  record prepare
  STATE_DIR="$fixture_dir"
  LOG_DIR="$fixture_dir"
  BACKEND_PORT=18081
  FRONTEND_PORT=13001
  WEB_MODE=production
}
migrate_database() { record migrate; }
start_api() { record api; [ "$phase" != api-failure ] || return 23; }
start_web() { record web; [ "$phase" != web-failure ] || return 29; }
stop_component() { record "stop $1"; [ "$phase" != cleanup-failure ] || return 31; }
smoke_verify_rendered_config() { record rendered; [ "$phase" != rendered-mismatch ] || return 37; }
checkout_source_id() { [ "$phase" != identity-error ] || return 43; printf same-source; }
checkout_commit() { printf fixed-commit; }
api_configuration_id() { printf same-api; }
web_configuration_id() {
  if [ "$phase" = identity-drift ] && [ -f "$fixture_dir/playwright.json" ]; then printf changed; else printf same-web; fi
}
pnpm() {
  record "playwright $*"
  cp "$fixture_dir/report.json" "$fixture_dir/playwright.json"
  [ "$phase" != playwright-failure ] || return 41
}
if [ "$phase" = missing-env ]; then unset DATABASE_URL; fi
smoke_main
`;

for (const [phase, expected] of Object.entries({
  success: 0, "missing-env": 1, "missing-db": 1, "wrong-db": 1,
  "api-failure": 23, "web-failure": 29, "rendered-mismatch": 37,
  "playwright-failure": 1, "inventory-missing": 1, "identity-drift": 1, "identity-error": 43, "cleanup-failure": 1,
})) {
  test(`orchestration ${phase}`, () => {
    const dir = mkdtempSync(join(tmpdir(), "critical-path-launch-"));
    const file = join(dir, "fixture.sh"); writeFileSync(file, fixture);
    const report = goodReport();
    if (phase === "inventory-missing") report.suites[0].specs.pop();
    writeFileSync(join(dir, "report.json"), JSON.stringify(report));
    const result = spawnSync("bash", [file, root, dir, phase], { encoding: "utf8", timeout: 30_000 });
    writeFileSync(join(dir, "fixture.log"), result.stdout + result.stderr);
    assert.equal(result.status, expected, result.stdout + result.stderr);
    const calls = (() => { try { return readFileSync(join(dir, "calls"), "utf8"); } catch { return ""; } })();
    if (["missing-env", "missing-db", "wrong-db"].includes(phase)) {
      assert.ok(!/prepare|migrate|api|web|playwright|stop /.test(calls), calls);
    } else if (phase === "identity-error") {
      assert.ok(!/migrate|api|web|playwright|stop /.test(calls), calls);
      assert.equal(JSON.parse(readFileSync(join(dir, "result.json"), "utf8")).exit, 43);
    } else {
      assert.ok(calls.includes("stop api"), calls);
      if (phase !== "api-failure") assert.ok(calls.includes("stop web"), calls);
      const verdict = JSON.parse(readFileSync(join(dir, "result.json"), "utf8"));
      assert.equal(verdict.exit, expected);
      if (["success", "playwright-failure", "inventory-missing", "identity-drift", "cleanup-failure"].includes(phase)) {
        assert.equal(calls.split("\n").filter((line) => line.startsWith("playwright ")).length, 1);
        assert.ok(calls.includes("--workers=1 --retries=0"));
        assert.ok(readFileSync(join(dir, "playwright.json"), "utf8"));
      }
    }
  });
}

test("busy unowned listener is neither adopted nor terminated", () => {
  const dir = mkdtempSync(join(tmpdir(), "critical-path-busy-"));
  const result = spawnSync("bash", ["-s", "--", root, dir], { encoding: "utf8", timeout: 15_000, input: String.raw`
set -euo pipefail
source "$1/scripts/agent-critical-path.sh"
STATE_DIR=$2; LOG_DIR=$2; BACKEND_PORT=18081; FRONTEND_PORT=13001; WEB_MODE=production
health_json() { return 1; }
port_free() { return 1; }
port_listener_pid() { printf 999999; }
checkout_commit() { printf fixed; }
describe_port_owner() { printf 'unowned listener'; }
kill() { printf '%s\n' "$*" >> "$STATE_DIR/kill-attempts"; return 1; }
trap 'stop_component api' EXIT
start_api
` });
  assert.equal(result.status, 1, result.stdout + result.stderr);
  assert.match(result.stderr, /busy/);
  assert.throws(() => readFileSync(join(dir, "kill-attempts")));
});
