import assert from "node:assert/strict";
import { chmodSync, mkdirSync, readFileSync, readdirSync, symlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import fs from "node:fs";
import { syncBuiltinESMExports } from "node:module";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { cli, scratch, seed } from "./changelog-test-helpers.mjs";

function fixture(t) {
  const cwd = scratch(t);
  const input = join(cwd, "input.json");
  const destination = join(cwd, "feed.json");
  const envFile = join(cwd, ".env");
  const oldEnv = "# Existing deployment\nJWT_SECRET=keep-$exact-value\nCHANGELOG_FILE=\nOTHER_KEY=untouched\n";
  writeFileSync(input, JSON.stringify(seed(), null, 2) + "\n");
  writeFileSync(destination, "previous feed bytes");
  writeFileSync(envFile, oldEnv, { mode: 0o600 });
  const args = ["--input", input, "--destination", destination, "--deployment-env", envFile, "--directory", "/srv/multica data/changelog"];
  return { cwd, input, destination, envFile, oldEnv, args };
}

test("installer durably enables hot feed while preserving unrelated env values", (t) => {
  const setup = fixture(t);
  const result = cli("install-changelog.mjs", setup.cwd, setup.args);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(setup.destination, "utf8"), readFileSync(setup.input, "utf8"));
  const env = readFileSync(setup.envFile, "utf8");
  assert.match(env, /JWT_SECRET=keep-\$exact-value\n/);
  assert.match(env, /OTHER_KEY=untouched\n/);
  assert.match(env, /CHANGELOG_FILE='\/app\/data\/changelog\/changelog.json'/);
  assert.match(env, /CHANGELOG_DIRECTORY="\/srv\/multica data\/changelog"/);
  assert.equal((env.match(/^CHANGELOG_FILE=/gm) ?? []).length, 1);
  assert.equal(readdirSync(setup.cwd).some((name) => name.endsWith(".tmp")), false);
});

test("invalid feed and incompatible existing path leave feed and env untouched", (t) => {
  const setup = fixture(t);
  const incompatible = cli("install-changelog.mjs", setup.cwd, [...setup.args, "--current-file", "/custom/notes.json"]);
  assert.notEqual(incompatible.status, 0);
  assert.match(incompatible.stderr, /CHANGELOG_FILE|incompatible/);
  assert.equal(readFileSync(setup.destination, "utf8"), "previous feed bytes");
  assert.equal(readFileSync(setup.envFile, "utf8"), setup.oldEnv);
  writeFileSync(setup.input, "{}");
  const invalid = cli("install-changelog.mjs", setup.cwd, setup.args);
  assert.notEqual(invalid.status, 0);
  assert.equal(readFileSync(setup.destination, "utf8"), "previous feed bytes");
  assert.equal(readFileSync(setup.envFile, "utf8"), setup.oldEnv);
});

test("destination symlinks and non-writable directories fail before changing configuration", (t) => {
  const setup = fixture(t);
  const link = join(setup.cwd, "link.json");
  symlinkSync(setup.destination, link);
  const args = setup.args.map((arg) => arg === setup.destination ? link : arg);
  const result = cli("install-changelog.mjs", setup.cwd, args);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /regular|symlink/);
  assert.equal(readFileSync(setup.envFile, "utf8"), setup.oldEnv);
  if (process.getuid?.() !== 0) {
    const readonly = join(setup.cwd, "readonly");
    mkdirSync(readonly, { mode: 0o500 });
    const failure = cli("install-changelog.mjs", setup.cwd, setup.args.map((arg) => arg === setup.destination ? join(readonly, "feed.json") : arg));
    chmodSync(readonly, 0o700);
    assert.notEqual(failure.status, 0);
    assert.equal(readFileSync(setup.envFile, "utf8"), setup.oldEnv);
  }
});

test("failed env finalization restores the previous feed and removes staged files", async (t) => {
  const setup = fixture(t);
  const { installChangelog } = await import("./install-changelog.mjs");
  const rename = fs.renameSync;
  t.mock.method(fs, "renameSync", (source, destination) => {
    if (destination === setup.envFile) throw new Error("fixture env finalization failure");
    return rename(source, destination);
  });
  syncBuiltinESMExports();
  try {
    assert.throws(() => installChangelog({ input: setup.input, destination: setup.destination, "deployment-env": setup.envFile, directory: "/srv/changelog" }), /fixture env finalization failure/);
    assert.equal(readFileSync(setup.destination, "utf8"), "previous feed bytes");
    assert.equal(readFileSync(setup.envFile, "utf8"), setup.oldEnv);
    assert.equal(readdirSync(setup.cwd).some((name) => name.endsWith(".tmp")), false);
  } finally {
    t.mock.restoreAll();
    syncBuiltinESMExports();
  }
});

test("installer preserves multiline dotenv values and quotes literal directory characters", (t) => {
  const setup = fixture(t);
  const multiline = "PRIVATE_NOTE='first line\nCHANGELOG_FILE=inside-note\nlast line'\nOTHER=present\n";
  writeFileSync(setup.envFile, multiline);
  const directory = "/srv/a's $release\\literal";
  const result = cli("install-changelog.mjs", setup.cwd, setup.args.map((value) => value === "/srv/multica data/changelog" ? directory : value));
  assert.equal(result.status, 0, result.stderr);
  const contents = readFileSync(setup.envFile, "utf8");
  assert.ok(contents.startsWith(multiline));
  assert.ok(contents.includes('CHANGELOG_DIRECTORY="/srv/a\'s $$release\\\\literal"'));
});

test("newline paths are rejected without changing prior feed or deployment configuration", (t) => {
  for (const directory of ["/srv/path\nOTHER_KEY=replaced", "/srv/path\rOTHER_KEY=replaced"]) {
    const setup = fixture(t);
    const result = cli("install-changelog.mjs", setup.cwd, setup.args.map((value) => value === "/srv/multica data/changelog" ? directory : value));
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /single-line/);
    assert.equal(readFileSync(setup.destination, "utf8"), "previous feed bytes");
    assert.equal(readFileSync(setup.envFile, "utf8"), setup.oldEnv);
  }
});

test("problematic backslash combinations use the Compose-verified double-quoted encoding", (t) => {
  for (const [directory, encoded] of [
    ["/srv/path\\", '"/srv/path' + "\\\\" + '"'],
    ["/srv/a\\'s/path", '"/srv/a' + "\\\\" + "'s/path\""],
  ]) {
    const setup = fixture(t);
    const result = cli("install-changelog.mjs", setup.cwd, setup.args.map((value) => value === "/srv/multica data/changelog" ? directory : value));
    assert.equal(result.status, 0, result.stderr);
    assert.ok(readFileSync(setup.envFile, "utf8").split("\n").includes(`CHANGELOG_DIRECTORY=${encoded}`));
  }
});

test("real Compose preserves every literal path character after installation and reinstallation", { skip: process.env.MULTICA_RUN_DOCKER_CHANGELOG_SMOKE !== "1" }, async (t) => {
  const cases = [
    ["trailing backslash", "/srv/path\\"],
    ["backslash before apostrophe", "/srv/a\\'s/path"],
    ["backslash before double quote", '/srv/a\\"quote"/path'],
    ["multiple backslashes", "/srv/a\\\\'s/path\\\\"],
    ["literal dollars and both quotes", '/srv/$MULTICA_CHANGELOG_EXPANSION_FIXTURE/${MULTICA_CHANGELOG_EXPANSION_FIXTURE}/$$/"double"/\'single\''],
  ];
  for (const [label, directory] of cases) {
    await t.test(label, (t) => {
      const setup = fixture(t);
      const compose = join(setup.cwd, "compose.yaml");
      writeFileSync(compose, "services:\n  probe:\n    image: fixture-not-started\n");
      const environment = { ...process.env, MULTICA_CHANGELOG_EXPANSION_FIXTURE: "must-not-expand" };
      delete environment.CHANGELOG_DIRECTORY;
      const args = setup.args.map((value) => value === "/srv/multica data/changelog" ? directory : value);
      let firstEnv;
      for (let attempt = 0; attempt < 2; attempt++) {
        const installed = cli("install-changelog.mjs", setup.cwd, args);
        assert.equal(installed.status, 0, installed.stderr);
        const content = readFileSync(setup.envFile, "utf8");
        assert.ok(content.includes("JWT_SECRET=keep-$exact-value\n"));
        assert.ok(content.includes("OTHER_KEY=untouched\n"));
        if (attempt === 1) assert.equal(content, firstEnv);
        firstEnv = content;
        const parsed = spawnSync("docker", ["compose", "--env-file", setup.envFile, "-f", compose, "config", "--environment"], { env: environment, encoding: "utf8" });
        assert.equal(parsed.status, 0, parsed.stderr);
        const actual = parsed.stdout.split("\n").find((line) => line.startsWith("CHANGELOG_DIRECTORY="));
        assert.equal(actual, `CHANGELOG_DIRECTORY=${directory}`);
      }
    });
  }
});
