import assert from "node:assert/strict";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { commit, generate, git, gitFixture, repository, seed, timestamp } from "./changelog-test-helpers.mjs";

function pass(result) {
  assert.equal(result.status, 0, result.stderr || result.stdout);
  return result.feed();
}

function fail(result, pattern) {
  assert.notEqual(result.status, 0, "invalid generation must fail");
  assert.match(result.stderr, pattern);
}

test("preview uses immutable committed objects, includes merged work and useful legacy subjects", (t) => {
  const fixture = gitFixture(t);
  const feature = commit(fixture.cwd, "feat(reader): 阅读变更说明");
  git(fixture.cwd, "checkout", "-b", "feature");
  const branch = commit(fixture.cwd, "fix(feed): Keep the last valid history");
  git(fixture.cwd, "checkout", "main");
  git(fixture.cwd, "merge", "--no-ff", "feature", "-m", "Merge feature");
  const legacy = commit(fixture.cwd, "Prevent stale sessions from dropping new credentials");
  commit(fixture.cwd, "chore(task): Record implementation plan");
  commit(fixture.cwd, "test(feed): Add fixtures");
  const target = commit(fixture.cwd, "perf(reader): Avoid duplicate requests");
  writeFileSync(join(fixture.cwd, "tracked.txt"), "feat: DIRTY CONTENT MUST NOT SHIP");
  const run = generate(fixture, { ref: target });
  const feed = pass(run);
  const originalBytes = run.bytes();
  const originalMarkdown = run.notes();
  const entry = feed.releases.find((release) => release.source === "fork");
  assert.equal(entry.status, "unreleased");
  assert.equal(entry.published_at, null);
  assert.equal(entry.commit, target);
  assert.equal(entry.base_commit, fixture.base);
  assert.equal(entry.id, `fork:${repository}:unreleased`);
  const items = entry.sections.flatMap((section) => section.items);
  assert.ok(items.some((item) => item.commit === feature && item.text === "阅读变更说明"));
  assert.ok(items.some((item) => item.commit === branch));
  assert.ok(items.some((item) => item.commit === legacy));
  assert.equal(items.length, 4);
  assert.doesNotMatch(run.bytes() + run.notes(), /DIRTY CONTENT|Merge feature|Record implementation|Add fixtures/);
  assert.deepEqual(feed.releases.find((release) => release.source === "upstream"), seed().releases[0]);
  const repeated = generate(fixture, { ref: target });
  assert.equal(pass(repeated).generated_at, feed.generated_at);
  assert.equal(repeated.bytes(), originalBytes);
  assert.equal(repeated.notes(), originalMarkdown);
});

test("public trailers override bookkeeping and Markdown escapes hostile plain text", (t) => {
  const fixture = gitFixture(t);
  const text = "支持 [外链](javascript:alert(1)) <img> `command` $(touch should-not-exist)";
  const target = commit(fixture.cwd, `chore(feed): internal details\n\nRelease-note: ${text}\nChangelog: improvements\nConfidence: high`);
  commit(fixture.cwd, "feat(secret): internal experiment\n\nChangelog: skip");
  const run = generate(fixture);
  const entry = pass(run).releases.find((release) => release.source === "fork");
  assert.deepEqual(entry.sections, [{ category: "improvements", items: [{ text, commit: target }] }]);
  assert.match(run.notes(), /\\\[外链\\\]/);
  assert.match(run.notes(), /&lt;img&gt;/);
  assert.match(run.notes(), /\\`command\\`/);
  assert.equal(existsSync(join(fixture.cwd, "should-not-exist")), false);
});

test("rejects invalid or contradictory public trailers with commit evidence", (t) => {
  for (const trailer of ["Changelog: misc", "Release-note:", "Changelog: fixes\nChangelog: features", "Release-note: a\nRelease-note: b", "Release-note: visible\nChangelog: skip"]) {
    const fixture = gitFixture(t);
    const target = commit(fixture.cwd, `feat(reader): change\n\n${trailer}`);
    const run = generate(fixture);
    fail(run, new RegExp(target));
  }
});

test("release requires a clean tracked tree, immutable ref and explicit publication time", (t) => {
  const fixture = gitFixture(t);
  const target = commit(fixture.cwd, "feat(reader): release reader");
  const options = { version: "v1.0.0", status: "published", ref: target };
  fail(generate(fixture, { ...options, ref: "main" }), /immutable|tag|commit/i);
  fail(generate(fixture, { ...options, publishedAt: null }), /published-at/);
  writeFileSync(join(fixture.cwd, "tracked.txt"), "uncommitted release inputs");
  fail(generate(fixture, options), /dirty|tracked/i);
  assert.equal(existsSync(join(fixture.cwd, "result.json")), false);
  git(fixture.cwd, "restore", "tracked.txt");
  git(fixture.cwd, "tag", "-a", "v1.0.0", "-m", "release tag", target);
  assert.equal(pass(generate(fixture, { ...options, ref: "v1.0.0" })).releases[0].commit, target);
});

test("history and ancestral base are mandatory; unrelated high tags are never selected", (t) => {
  const fixture = gitFixture(t);
  const target = commit(fixture.cwd, "feat(reader): target");
  git(fixture.cwd, "checkout", "--orphan", "unrelated");
  const unrelated = commit(fixture.cwd, "feat(other): unrelated upstream");
  git(fixture.cwd, "tag", "v999.0.0", unrelated);
  git(fixture.cwd, "checkout", "main");
  fail(generate(fixture, { history: null, ref: target }), /history/);
  fail(generate(fixture, { base: null, ref: target }), /base/);
  fail(generate(fixture, { base: unrelated, ref: target }), /ancestor/);
  fail(generate(fixture, { ref: "--all" }), /ref|option/);
  const feed = pass(generate(fixture, { ref: target }));
  assert.equal(feed.releases[0].base_commit, fixture.base);
  assert.doesNotMatch(JSON.stringify(feed), /unrelated upstream/);
});

test("first release, prerelease, stable and older retry preserve cumulative immutable history", (t) => {
  const fixture = gitFixture(t);
  const first = commit(fixture.cwd, "feat(reader): initial reader");
  const firstOptions = { ref: first, version: "v1.0.0", status: "published" };
  const firstRun = generate(fixture, firstOptions);
  const firstFeed = pass(firstRun);
  writeFileSync(fixture.history, firstRun.bytes());
  const preview = commit(fixture.cwd, "feat(reader): prerelease improvement");
  const previewRun = generate(fixture, { base: null, ref: preview, version: "v1.1.0-beta.1", status: "prerelease", publishedAt: "2026-09-14T00:00:00Z" });
  const previewFeed = pass(previewRun);
  assert.equal(previewFeed.releases[0].base_commit, first);
  writeFileSync(fixture.history, previewRun.bytes());
  const stable = commit(fixture.cwd, "fix(reader): stable improvement");
  const stableRun = generate(fixture, { base: null, ref: stable, version: "v1.1.0", status: "published", publishedAt: "2026-09-15T00:00:00Z" });
  const stableFeed = pass(stableRun);
  assert.equal(stableFeed.releases[0].base_commit, preview);
  writeFileSync(fixture.history, stableRun.bytes());
  const expected = readFileSync(fixture.history, "utf8");
  const retry = generate(fixture, { ...firstOptions, base: null, publishedAt: "2030-01-01T00:00:00Z" });
  assert.deepEqual(pass(retry), stableFeed);
  assert.equal(retry.bytes(), expected);
  assert.equal(stableFeed.releases.length, 4);
  assert.deepEqual(stableFeed.releases.find((r) => r.version === "v1.0.0"), firstFeed.releases[0]);
  fail(generate(fixture, { ...firstOptions, ref: stable }), /immutable|conflict/);
  fail(generate(fixture, { ...firstOptions, base: first }), /content|base|conflict/);
  fail(generate(fixture, { ...firstOptions, status: "prerelease" }), /status|conflict/);
});

test("release removes a covered preview and retains a genuinely later preview", (t) => {
  const fixture = gitFixture(t);
  const first = commit(fixture.cwd, "feat(reader): initial");
  const earlier = generate(fixture, { ref: first });
  pass(earlier);
  writeFileSync(fixture.history, earlier.bytes());
  const later = commit(fixture.cwd, "feat(reader): later work");
  const laterPreview = generate(fixture, { ref: later });
  pass(laterPreview);
  writeFileSync(fixture.history, laterPreview.bytes());
  const firstRelease = generate(fixture, { ref: first, version: "v1.0.0", status: "published" });
  assert.ok(pass(firstRelease).releases.some((r) => r.status === "unreleased" && r.commit === later));
  writeFileSync(fixture.history, firstRelease.bytes());
  const covered = generate(fixture, { ref: later, base: null, version: "v1.1.0", status: "published" });
  assert.equal(pass(covered).releases.some((r) => r.status === "unreleased"), false);
});

test("incomparable ancestral releases require an explicit base", (t) => {
  const fixture = gitFixture(t);
  const left = commit(fixture.cwd, "feat(left): left branch");
  const leftRun = generate(fixture, { ref: left, version: "v1.0.0", status: "published" });
  const leftFeed = pass(leftRun);
  git(fixture.cwd, "checkout", "-b", "right", fixture.base);
  const right = commit(fixture.cwd, "feat(right): right branch");
  const rightRun = generate(fixture, { ref: right, version: "v2.0.0", status: "published" });
  const rightFeed = pass(rightRun);
  writeFileSync(fixture.history, JSON.stringify({ ...leftFeed, releases: [leftFeed.releases[0], rightFeed.releases[0], seed().releases[0]] }));
  git(fixture.cwd, "merge", "-s", "ours", "--no-ff", "main", "-m", "Merge branches");
  const target = git(fixture.cwd, "rev-parse", "HEAD");
  fail(generate(fixture, { ref: target, base: null }), /ambiguous|incomparable/);
  assert.equal(pass(generate(fixture, { ref: target, base: right })).releases[0].base_commit, right);
});

test("rejects malformed, oversized, duplicate and foreign-fork histories before output", (t) => {
  const fixture = gitFixture(t);
  const target = commit(fixture.cwd, "feat(reader): initial");
  const valid = pass(generate(fixture, { ref: target }));
  const fork = valid.releases.find((r) => r.source === "fork");
  const cases = [
    { ...valid, schema_version: 2 },
    { ...valid, generated_at: "2026-02-30T00:00:00Z" },
    { ...valid, releases: [...valid.releases, fork] },
    { ...valid, releases: [{ ...fork, id: "fork:someone/else:unreleased" }] },
    { ...valid, releases: [...valid.releases, { ...fork, id: "fork:someone/else:unreleased" }] },
    { ...valid, releases: [{ ...fork, published_at: timestamp }] },
    { ...valid, releases: [{ ...fork, sections: [{ category: "oops", items: [] }] }] },
    { ...valid, releases: [{ ...fork, commit: "short" }] },
  ];
  for (const invalid of cases) {
    writeFileSync(fixture.history, JSON.stringify(invalid));
    fail(generate(fixture, { ref: target }), /history|schema|timestamp|identity|fork|repository|category|commit|date|unreleased/i);
  }
  writeFileSync(fixture.history, " ".repeat(2 * 1024 * 1024 + 1));
  fail(generate(fixture, { ref: target }), /size|large|MiB/);
});
