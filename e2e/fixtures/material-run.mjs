import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { spawn, execFileSync } from "node:child_process";
import { mkdtempSync, openSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { dirname, join, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import pg from "pg";
import { createMaterialGitLab } from "./material-gitlab.mjs";

const [recordFile, expectedDatabase, webBuild, desktopOutput] = process.argv.slice(2);
assert(recordFile && expectedDatabase && webBuild && desktopOutput, "Usage: node e2e/fixtures/material-run.mjs PRIVATE_UI_JSON EXPECTED_DATABASE BUILT_WEB_DIR BUILT_DESKTOP_OUT");
assert.equal(statSync(recordFile).mode & 0o077, 0, "allocation record must be private");
const record = JSON.parse(readFileSync(recordFile, "utf8"));
const { env, ports } = record;
const dbURL = new URL(record.connection);
assert(["localhost", "127.0.0.1"].includes(dbURL.hostname));
assert(/^multica_gitmaterial_ui_[a-z0-9_]+$/.test(expectedDatabase));
assert.equal(dbURL.pathname, "/" + expectedDatabase);
assert.equal(record.database, expectedDatabase);
assert(record.connection === env.DATABASE_URL, "database allocation mismatch");
const db = new pg.Client({ connectionString: record.connection });
await db.connect();
try { assert.equal((await db.query("SELECT current_database() AS db")).rows[0].db, expectedDatabase); }
finally { await db.end(); }
const origin = (port) => "http://127.0.0.1:" + port;
assert.equal(env.NEXT_PUBLIC_API_URL, origin(ports.api));
assert.equal(env.FRONTEND_ORIGIN, origin(ports.web));
assert.equal(env.REMOTE_API_URL, origin(ports.api));
assert.equal(Number(env.PORT), ports.api);
assert.equal(new Set([ports.api, ports.web, ports.gitlab]).size, 3);
for (const port of [ports.web, ports.api, ports.gitlab]) {
  assert(Number.isInteger(port) && port > 1024 && port <= 65535);
  await new Promise((ok, fail) => {
    const probe = createServer().once("error", fail);
    probe.listen(port, "127.0.0.1", () => probe.close(ok));
  });
}
const root = resolve(import.meta.dirname, "../..");
const dir = mkdtempSync(join(dirname(resolve(recordFile)), "material-run-"));
const stamp = "material-" + randomUUID();
const childEnv = { PATH: process.env.PATH, HOME: process.env.HOME, TMPDIR: dir, ...env };
const children = [];
function launch(file, args, options) {
  const child = spawn(file, args, options);
  children.push(child);
  return child;
}
async function health(url, api) {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (children.some((child) => child.exitCode !== null)) throw Error("Fixture process exited; inspect " + dir);
    try {
      const result = await fetch(url + "/health", { signal: AbortSignal.timeout(1500) });
      const json = await result.json();
      if (result.ok && json.commit === stamp && json.pid === api.pid) return;
    } catch {}
    await delay(300);
  }
  throw Error("Could not prove API identity through " + url + "; inspect " + dir);
}
const git = createMaterialGitLab({ key: readFileSync(join(dirname(recordFile), "gitlab-key.pem")), cert: readFileSync(env.SSL_CERT_FILE), token: record.git_token, controlToken: record.control_token });
const binary = join(dir, "api");
const args = ["-C", "server", "build"];
if (process.platform === "darwin") {
  const overlay = join(dir, "overlay.json");
  writeFileSync(overlay, JSON.stringify({ Replace: { [join(root, "server/cmd/server/material_fixture_ca.go")]: join(import.meta.dirname, "material-test-ca.go.txt") } }));
  args.push("-overlay", overlay);
  childEnv.GODEBUG = "x509usefallbackroots=1";
}
execFileSync("go", [...args, "-ldflags", "-X main.commit=" + stamp, "-o", binary, "./cmd/server"], { cwd: root, stdio: "inherit" });
console.log("Verified " + expectedDatabase + "; evidence: " + dir);
try {
  await git.listen(ports.gitlab);
  const apiLog = openSync(join(dir, "api.log"), "w", 0o600);
  const api = launch(binary, [], { cwd: dir, env: childEnv, stdio: ["ignore", apiLog, apiLog] });
  await health(origin(ports.api), api);
  const webLog = openSync(join(dir, "web.log"), "w", 0o600);
  launch(process.execPath, [join(root, "apps/web/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(ports.web)], { cwd: resolve(webBuild), env: childEnv, stdio: ["ignore", webLog, webLog] });
  await health(origin(ports.web), api);
  const run = launch(process.execPath, [join(root, "node_modules/@playwright/test/cli.js"), "test", "e2e/material-workflow.spec.ts", "--workers=1", "--output", join(dir, "results")], { cwd: root, env: {
    ...childEnv, PLAYWRIGHT_BASE_URL: origin(ports.web), MATERIAL_RUN_STAMP: stamp, MATERIAL_RUN_DB: expectedDatabase, MATERIAL_UI_RECORD: resolve(recordFile), MULTICA_E2E_DESKTOP_OUTPUT_DIR: resolve(desktopOutput),
  }, stdio: "inherit" });
  process.exitCode = await new Promise((ok, fail) => run.once("error", fail).once("exit", (code) => ok(code ?? 1)));
} finally {
  for (const child of children.reverse()) if (child.exitCode === null) child.kill();
  await git.close();
}
