import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const checker = fileURLToPath(new URL("./check-image-budget.mjs", import.meta.url));
const budget = 300 * 1024;

/** @param {import("node:test").TestContext} t */
function fixture(t) {
  const directory = mkdtempSync(join(tmpdir(), "multica-image-budget-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const hooks = join(directory, "hooks");
  mkdirSync(hooks);
  const env = {
    ...process.env,
    GIT_CONFIG_NOSYSTEM: "1",
    GIT_CONFIG_GLOBAL: join(directory, "gitconfig"),
    GIT_AUTHOR_NAME: "Image budget test",
    GIT_AUTHOR_EMAIL: "image-budget@example.invalid",
    GIT_COMMITTER_NAME: "Image budget test",
    GIT_COMMITTER_EMAIL: "image-budget@example.invalid",
    PR_BODY: "",
    GITHUB_ACTIONS: "",
    GITHUB_BASE_REF: "",
  };
  writeFileSync(env.GIT_CONFIG_GLOBAL, "");
  const git = (...args) => execFileSync("git", ["-c", `core.hooksPath=${hooks}`, "-c", "commit.gpgSign=false", ...args], {
    cwd: directory, env, encoding: "utf8", maxBuffer: 4 * 1024 * 1024,
  });
  git("init", "--quiet");
  git("commit", "--quiet", "--allow-empty", "-m", "baseline");
  return {
    directory,
    git,
    write(file, bytes) { writeFileSync(join(directory, file), Buffer.alloc(bytes)); },
    check(overrides = {}, base = "HEAD") {
      const result = spawnSync(process.execPath, [checker, "--base", base], {
        cwd: directory, env: { ...env, ...overrides }, encoding: "utf8", timeout: 30_000,
      });
      assert.ifError(result.error);
      assert.notEqual(result.status, null);
      return result;
    },
  };
}

test("accepts the exact bitmap budget and ignores non-bitmap sizes", (t) => {
  const repo = fixture(t);
  repo.write("at-limit.png", budget);
  repo.write("large.txt", budget + 1);
  repo.git("add", ".");
  const result = repo.check();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /No bitmap added or grown/);
});

test("rejects a new bitmap above the budget, including uppercase extensions", (t) => {
  const repo = fixture(t);
  repo.write("oversized.PNG", budget + 1);
  repo.git("add", ".");
  const result = repo.check();
  assert.equal(result.status, 1);
  assert.match(result.stdout, /oversized\.PNG.*new file/);
  assert.match(result.stderr, /1 bitmap\(s\) exceed/);
});

test("rejects both crossing the limit and growing an already oversized bitmap", (t) => {
  const repo = fixture(t);
  repo.write("crossing.png", budget);
  repo.write("growing.webp", budget + 1);
  repo.git("add", ".");
  repo.git("commit", "--quiet", "-m", "existing bitmaps");
  repo.write("crossing.png", budget + 1);
  repo.write("growing.webp", budget + 2);
  const result = repo.check();
  assert.equal(result.status, 1);
  assert.match(result.stdout, /crossing\.png.*up from/);
  assert.match(result.stdout, /growing\.webp.*up from/);
  assert.match(result.stderr, /2 bitmap\(s\) exceed/);
});

test("accepts shrinking an oversized bitmap and deleting another", (t) => {
  const repo = fixture(t);
  repo.write("shrinking.png", budget + 2);
  repo.write("deleted.jpg", budget + 1);
  repo.git("add", ".");
  repo.git("commit", "--quiet", "-m", "existing bitmaps");
  repo.write("shrinking.png", budget + 1);
  rmSync(join(repo.directory, "deleted.jpg"));
  assert.equal(repo.check().status, 0);
});

test("allows an explicit exemption with a reason while reporting the offender", (t) => {
  const repo = fixture(t);
  repo.write("evidence.png", budget + 1);
  repo.git("add", ".");
  const result = repo.check({ PR_BODY: "Review evidence\nOversized image exemption: preserve original capture" });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /evidence\.png/);
  assert.match(result.stdout, /PR description carries an exemption line/);
});

test("does not treat an empty exemption as approval", (t) => {
  const repo = fixture(t);
  repo.write("evidence.png", budget + 1);
  repo.git("add", ".");
  assert.equal(repo.check({ PR_BODY: "Oversized image exemption:   " }).status, 1);
});

test("checks paths with whitespace and non-ASCII characters without Git quoting", (t) => {
  const repo = fixture(t);
  const file = "capture 中文 final.png";
  repo.write(file, budget + 1);
  repo.git("add", ".");
  const result = repo.check();
  assert.equal(result.status, 1);
  assert.ok(result.stdout.includes(file), result.stderr);
  assert.match(result.stderr, /1 bitmap\(s\) exceed/);
});

test("checks tab and newline characters inside a path", { skip: process.platform === "win32" }, (t) => {
  const repo = fixture(t);
  const file = "capture\tpart\nfinal.png";
  repo.write(file, budget + 1);
  repo.git("add", ".");
  const result = repo.check();
  assert.equal(result.status, 1);
  assert.ok(result.stdout.includes(file), result.stderr);
  assert.match(result.stderr, /1 bitmap\(s\) exceed/);
});

test("fails for an invalid base instead of reporting a clean budget", (t) => {
  const repo = fixture(t);
  const result = repo.check({}, "missing-base");
  assert.notEqual(result.status, 0);
  assert.doesNotMatch(result.stdout, /No bitmap added or grown/);
  assert.match(result.stderr, /missing-base/);
});

test("fails when Git cannot start", (t) => {
  const repo = fixture(t);
  const result = repo.check({ PATH: repo.directory });
  assert.notEqual(result.status, 0);
  assert.doesNotMatch(result.stdout, /No bitmap added or grown/);
  assert.match(result.stderr, /ENOENT/);
});

test("processes a real diff larger than the synchronous buffer and checks its last bitmap", { timeout: 60_000 }, (t) => {
  const repo = fixture(t);
  for (let i = 0; i < 6000; i++) {
    repo.write(`entry-${String(i).padStart(5, "0")}-${"x".repeat(180)}.txt`, 0);
  }
  repo.write("zz-last.png", budget + 1);
  repo.git("add", ".");
  const diff = repo.git("diff", "--name-status", "--diff-filter=AM", "HEAD", "--");
  assert.ok(Buffer.byteLength(diff) > 1024 * 1024);
  const result = repo.check();
  assert.equal(result.status, 1);
  assert.match(result.stdout, /zz-last\.png.*new file/, result.stderr);
  assert.match(result.stderr, /1 bitmap\(s\) exceed/);
  assert.doesNotMatch(result.stderr, /ENOBUFS/);
});
