import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const repository = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const pinned = readFileSync(join(repository, "docker-compose.desktop-updates.yml"), "utf8")
  .match(/DESKTOP_UPDATES_IMAGE:-([^}]+)}/)[1];
const digest = pinned.split("sha256:")[1].slice(0, 12);

function fixture(t) {
  const directory = realpathSync(mkdtempSync(join(tmpdir(), "desktop shell ")));
  const root = join(directory, "minimal deployment");
  const bin = join(directory, "bin");
  const caller = join(directory, "operator work");
  const log = join(directory, "commands.jsonl");
  for (const path of [join(root, "scripts"), join(root, "deploy/desktop-updates"), bin, caller]) mkdirSync(path, { recursive: true });
  for (const file of ["scripts/desktop-updates.sh", "docker-compose.desktop-updates.yml", "deploy/desktop-updates/nginx.conf"]) {
    copyFileSync(join(repository, file), join(root, file));
  }
  const executable = (name, body) => writeFileSync(join(bin, name), `#!${process.execPath}\n${body}`, { mode: 0o755 });
  const recorder = `const fs = require('node:fs'); const args = process.argv.slice(2); fs.appendFileSync(process.env.TEST_LOG, JSON.stringify({ command: require('node:path').basename(process.argv[1]), args, cwd: process.cwd(), storage: process.env.DESKTOP_UPDATES_STORAGE, port: process.env.DESKTOP_UPDATES_PORT, image: process.env.DESKTOP_UPDATES_IMAGE, bind: process.env.DESKTOP_UPDATES_BIND }) + '\\n');`;
  executable("docker", `${recorder}
    if (args.includes('--environment')) {
      if (process.env.TEST_CONFIG_FAIL) process.exit(9);
      process.stdout.write(process.env.TEST_RESOLVED_ENV || '');
    } else if (args[0] === 'pull' && process.env.TEST_PULL_FAIL) process.exit(11);
    else if (args[0] === 'save') {
      fs.writeFileSync(args[args.indexOf('--output') + 1], 'image bytes');
      if (process.env.TEST_SAVE_FAIL) process.exit(12);
    } else if (args[0] === 'image' && args[1] === 'inspect') process.stdout.write(process.env.TEST_PLATFORM || 'linux/amd64');
    else if (args.includes('ps') && args.includes('-q')) process.stdout.write('test-container');
    else if (args.includes('port')) process.stdout.write('0.0.0.0:18123');
    else if (args[0] === 'inspect') process.stdout.write(process.env.DESKTOP_UPDATES_STORAGE + '/public');
  `);
  executable("node", recorder);
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const env = { ...process.env, PATH: `${bin}:/usr/bin:/bin`, TEST_LOG: log };
  for (const key of Object.keys(env)) if (key.startsWith("DESKTOP_UPDATES_") || key.startsWith("COMPOSE_")) delete env[key];
  return {
    root, caller, directory, bin,
    run(args, extra = {}) {
      return spawnSync("/bin/bash", [join(root, "scripts/desktop-updates.sh"), ...args], { cwd: caller, env: { ...env, ...extra }, encoding: "utf8" });
    },
    calls() { return existsSync(log) ? readFileSync(log, "utf8").trim().split("\n").map((line) => JSON.parse(line)) : []; },
  };
}

function passed(result) { assert.equal(result.status, 0, result.stderr || result.stdout); }

test("help works without Docker or Node and describes persistent configuration", (t) => {
  const f = fixture(t);
  rmSync(join(f.bin, "docker"));
  rmSync(join(f.bin, "node"));
  const result = f.run(["help"]);
  passed(result);
  assert.match(result.stdout, /--env-file/);
  assert.match(result.stdout, /export-image/);
});

test("start resolves the persistent dotenv with Compose and creates the resolved storage", (t) => {
  const f = fixture(t);
  writeFileSync(join(f.root, "updates.env"), "DESKTOP_UPDATES_STORAGE='resolved storage'\n");
  rmSync(join(f.bin, "node"));
  passed(f.run(["start"], { TEST_RESOLVED_ENV: "DESKTOP_UPDATES_STORAGE=resolved storage\nDESKTOP_UPDATES_PORT=18123\nDESKTOP_UPDATES_BIND=0.0.0.0\nDESKTOP_UPDATES_IMAGE=intranet-nginx:amd64\nDESKTOP_UPDATES_URL=http://updates.local:18123/desktop\n" }));
  assert.ok(existsSync(join(f.root, "resolved storage/public")));
  const calls = f.calls();
  assert.deepEqual(calls[0].args.slice(0, 3), ["compose", "--env-file", join(f.root, "updates.env")]);
  const up = calls.find((call) => call.args.includes("up"));
  assert.equal(up.storage, join(f.root, "resolved storage"));
  assert.equal(up.port, "18123");
  assert.equal(up.bind, "0.0.0.0");
  assert.equal(up.image, "intranet-nginx:amd64");
  assert.deepEqual(up.args.slice(-5), ["up", "-d", "--pull", "never", "--wait"]);
});

test("explicit dotenv and config paths are caller-relative and configuration is never evaluated", (t) => {
  const f = fixture(t);
  const marker = join(f.directory, "must not exist");
  writeFileSync(join(f.caller, "operator.env"), `touch '${marker}'\n`);
  const url = "http://updates.local/desktop?literal=$(touch ignored)&value=one";
  passed(f.run(["--env-file", "operator.env", "configure", "--config", "desktop config.json"], { DESKTOP_UPDATES_URL: "http://ambient/desktop", TEST_RESOLVED_ENV: `DESKTOP_UPDATES_URL=${url}\n` }));
  assert.equal(existsSync(marker), false);
  assert.deepEqual(f.calls()[0].args.slice(0, 3), ["compose", "--env-file", join(f.caller, "operator.env")]);
  const node = f.calls().find((call) => call.command === "node");
  assert.deepEqual(node.args, [join(f.root, "apps/desktop/scripts/configure-updates.mjs"), "--url", url, "--config", join(f.caller, "desktop config.json")]);
});

test("legacy environment and default lifecycle commands work in the minimal deployment", (t) => {
  const f = fixture(t);
  rmSync(join(f.bin, "node"));
  for (const command of ["start", "status", "logs", "stop"]) passed(f.run([command], { DESKTOP_UPDATES_STORAGE: join(f.directory, "persistent releases"), DESKTOP_UPDATES_PORT: "18123" }));
  assert.ok(existsSync(join(f.directory, "persistent releases/public")));
  assert.equal(f.calls().some((call) => call.command === "node"), false);
  assert.equal(f.calls().some((call) => call.args.includes("--environment")), false);
});

test("collect and publish forward source paths and explicit prerelease opt-in", (t) => {
  const f = fixture(t);
  passed(f.run(["collect", "source build", "offline release", "--allow-prerelease"]));
  passed(f.run(["publish", "offline release", "--allow-prerelease"], { DESKTOP_UPDATES_STORAGE: join(f.directory, "store") }));
  assert.deepEqual(f.calls().map((call) => call.args), [
    [join(f.root, "apps/desktop/scripts/update-artifacts.mjs"), "collect", "--source", join(f.caller, "source build"), "--destination", join(f.caller, "offline release"), "--allow-prerelease"],
    [join(f.root, "apps/desktop/scripts/update-artifacts.mjs"), "publish", "--source", join(f.caller, "offline release"), "--destination", join(f.directory, "store"), "--allow-prerelease"],
  ]);
});

test("verify forwards URL, metadata selection and expected version", (t) => {
  const f = fixture(t);
  passed(f.run(["verify", "latest-ia32.yml", "--expected-version", "0.5.1"], { DESKTOP_UPDATES_URL: "https://updates.local/desktop" }));
  passed(f.run(["verify"]));
  assert.deepEqual(f.calls()[0].args, [join(f.root, "apps/desktop/scripts/verify-updates.mjs"), "--url", "https://updates.local/desktop", "--metadata", "latest-ia32.yml", "--expected-version", "0.5.1"]);
  assert.deepEqual(f.calls()[1].args.slice(-4), ["--url", "http://127.0.0.1:18080/desktop", "--metadata", "latest.yml"]);
});

test("custom deployment config requires an explicit client-visible URL before configure or verify", (t) => {
  const f = fixture(t);
  writeFileSync(join(f.root, "updates.env"), "DESKTOP_UPDATES_BIND=0.0.0.0\n");
  for (const command of ["configure", "verify"]) {
    const result = f.run([command], { TEST_RESOLVED_ENV: "DESKTOP_UPDATES_BIND=0.0.0.0\n" });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /DESKTOP_UPDATES_URL/);
  }
  assert.equal(f.calls().some((call) => call.command === "node"), false);
});

test("invalid CLI arguments fail before Docker calls or storage writes", (t) => {
  const f = fixture(t);
  const invalid = [["--env-file"], ["--env-file", "missing.env", "start"], ["start", "extra"], ["stop", "extra"], ["logs", "extra"], ["status", "extra"], ["publish"], ["publish", "source", "--bad"], ["collect", "source"], ["collect", "source", "dest", "--bad"], ["configure", "--url", "bad"], ["configure", "--config"], ["verify", "../latest.yml"], ["verify", "--expected-version"], ["verify", "--bad"], ["export-image", "archive.tar", "linux/s390x"]];
  for (const args of invalid) assert.notEqual(f.run(args).status, 0, JSON.stringify(args));
  assert.deepEqual(f.calls(), []);
  assert.equal(existsSync(join(f.root, "data")), false);
});

test("failed dotenv resolution makes no storage or lifecycle mutations", (t) => {
  const f = fixture(t);
  writeFileSync(join(f.root, "updates.env"), "invalid config\n");
  assert.notEqual(f.run(["start"], { TEST_CONFIG_FAIL: "1" }).status, 0);
  assert.equal(existsSync(join(f.root, "data")), false);
  assert.equal(f.calls().length, 1);
});

for (const architecture of ["amd64", "arm64"]) {
  test(`export-image pulls the pinned linux/${architecture} image and creates an archive with an architecture tag`, (t) => {
    const f = fixture(t);
    const result = f.run(["export-image", "nginx image.tar", `linux/${architecture}`], { DESKTOP_UPDATES_IMAGE: "unrelated:local", TEST_PLATFORM: `linux/${architecture}` });
    passed(result);
    assert.equal(readFileSync(join(f.caller, "nginx image.tar"), "utf8"), "image bytes");
    const tag = `multica-desktop-nginx:${digest}-${architecture}`;
    assert.ok(f.calls().some((call) => JSON.stringify(call.args) === JSON.stringify(["pull", "--platform", `linux/${architecture}`, pinned])));
    assert.ok(f.calls().some((call) => JSON.stringify(call.args) === JSON.stringify(["tag", pinned, tag])));
    assert.ok(f.calls().some((call) => call.args[0] === "save" && call.args.at(-1) === tag));
    assert.ok(f.calls().filter((call) => call.args[0] === "save" || call.args[1] === "inspect").every((call) => call.args[call.args.indexOf("--platform") + 1] === `linux/${architecture}`));
    assert.equal(f.calls().some((call) => call.args[0] === "compose"), false);
    assert.ok(result.stdout.includes(`DESKTOP_UPDATES_IMAGE=${tag}`));
    assert.deepEqual(readdirSync(f.caller), ["nginx image.tar"]);
  });
}

test("export-image never overwrites an existing archive or leaves a partial file on failure", (t) => {
  const f = fixture(t);
  writeFileSync(join(f.caller, "existing.tar"), "preserve");
  assert.notEqual(f.run(["export-image", "existing.tar", "linux/amd64"]).status, 0);
  assert.equal(readFileSync(join(f.caller, "existing.tar"), "utf8"), "preserve");
  assert.deepEqual(f.calls(), []);
  assert.notEqual(f.run(["export-image", "failed.tar", "linux/amd64"], { TEST_SAVE_FAIL: "1" }).status, 0);
  assert.deepEqual(readdirSync(f.caller), ["existing.tar"]);
});

test("export-image stops on pull failure or unexpected image architecture", (t) => {
  const f = fixture(t);
  assert.notEqual(f.run(["export-image", "failed.tar", "linux/amd64"], { TEST_PULL_FAIL: "1" }).status, 0);
  assert.notEqual(f.run(["export-image", "mismatch.tar", "linux/arm64"], { TEST_PLATFORM: "linux/amd64" }).status, 0);
  assert.equal(f.calls().some((call) => call.args[0] === "save"), false);
  assert.deepEqual(readdirSync(f.caller), []);
});
