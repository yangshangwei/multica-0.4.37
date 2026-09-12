import assert from "node:assert/strict";
import { mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { cli, scratch, seed } from "./changelog-test-helpers.mjs";

test("atomic publisher installs exact validated bytes and preserves a readable destination", (t) => {
  const cwd = scratch(t);
  const directory = join(cwd, "mounted-directory");
  mkdirSync(directory);
  const input = join(cwd, "next.json");
  const destination = join(directory, "changelog.json");
  const bytes = JSON.stringify(seed(), null, 2) + "\n";
  writeFileSync(input, bytes);
  writeFileSync(destination, "old feed");
  const inode = statSync(destination).ino;
  const result = cli("publish-changelog.mjs", cwd, ["--input", input, "--destination", destination]);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(destination, "utf8"), bytes);
  assert.notEqual(statSync(destination).ino, inode, "replace the directory entry rather than overwrite the live inode");
  assert.equal(statSync(destination).mode & 0o777, 0o644);
  assert.deepEqual(readdirSync(directory), ["changelog.json"]);
});

test("invalid input or failed destination rename never damages the previous feed", (t) => {
  const cwd = scratch(t);
  const input = join(cwd, "next.json");
  const destination = join(cwd, "changelog.json");
  writeFileSync(destination, "previous feed");
  for (const invalid of ["{}", "bad JSON", "x".repeat(2 * 1024 * 1024 + 1)]) {
    writeFileSync(input, invalid);
    const result = cli("publish-changelog.mjs", cwd, ["--input", input, "--destination", destination]);
    assert.notEqual(result.status, 0);
    assert.equal(readFileSync(destination, "utf8"), "previous feed");
  }
  writeFileSync(input, JSON.stringify(seed()));
  const blocked = join(cwd, "directory-instead-of-file");
  mkdirSync(blocked);
  writeFileSync(join(blocked, "keep"), "old bytes");
  const result = cli("publish-changelog.mjs", cwd, ["--input", input, "--destination", blocked]);
  assert.notEqual(result.status, 0);
  assert.equal(readFileSync(join(blocked, "keep"), "utf8"), "old bytes");
  assert.equal(readdirSync(cwd).some((name) => name.endsWith(".tmp")), false);
});
