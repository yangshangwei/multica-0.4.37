import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const scripts = dirname(fileURLToPath(import.meta.url));
export const repository = "example/multica";
export const timestamp = "2026-09-13T00:00:00Z";

export function scratch(t) {
  const path = mkdtempSync(join(tmpdir(), "multica-changelog-test-"));
  t.after(() => rmSync(path, { recursive: true, force: true }));
  return path;
}

export function git(cwd, ...args) {
  return execFileSync("git", ["-c", "core.hooksPath=/dev/null", ...args], {
    cwd,
    env: {
      ...process.env,
      GIT_CONFIG_GLOBAL: "/dev/null",
      GIT_CONFIG_NOSYSTEM: "1",
      GIT_AUTHOR_DATE: timestamp,
      GIT_COMMITTER_DATE: timestamp,
    },
    encoding: "utf8",
    stdio: ["pipe", "pipe", "pipe"],
  }).trim();
}

export function commit(cwd, message) {
  writeFileSync(join(cwd, "tracked.txt"), message);
  git(cwd, "add", "tracked.txt");
  git(cwd, "-c", "commit.gpgsign=false", "commit", "-m", message);
  return git(cwd, "rev-parse", "HEAD");
}

export function seed() {
  return {
    schema_version: 1,
    generated_at: timestamp,
    releases: [{
      id: "upstream:multica-ai/multica:v0.4.37",
      version: "v0.4.37",
      title: "Official reference fixture",
      published_at: "2026-08-31T00:00:00Z",
      status: "published",
      source: "upstream",
      commit: null,
      base_commit: null,
      sections: [{ category: "fixes", items: [{ text: "Official reference" }] }],
    }],
  };
}

export function gitFixture(t) {
  const cwd = scratch(t);
  git(cwd, "init", "--initial-branch=main");
  git(cwd, "config", "user.name", "Changelog Fixture");
  git(cwd, "config", "user.email", "changelog@example.invalid");
  const base = commit(cwd, "Initial snapshot");
  const history = join(cwd, "history.json");
  writeFileSync(history, JSON.stringify(seed(), null, 2) + "\n");
  return { cwd, base, history };
}

export function cli(name, cwd, args, env = {}) {
  return spawnSync(process.execPath, [resolve(scripts, name), ...args], {
    cwd,
    encoding: "utf8",
    env: { ...process.env, ...env },
  });
}

export function generate(fixture, options = {}) {
  const output = options.output ?? join(fixture.cwd, "result.json");
  const markdown = options.markdown ?? join(fixture.cwd, "notes.md");
  const args = [
    "--ref", options.ref ?? "HEAD",
    "--repository", options.repository ?? repository,
    "--version", options.version ?? "Unreleased",
    "--status", options.status ?? "unreleased",
    "--output", output,
    "--markdown", markdown,
  ];
  if (options.history !== null) args.push("--history", options.history ?? fixture.history);
  if (options.base !== null) args.push("--base", options.base ?? fixture.base);
  if (options.publishedAt !== null && options.status && options.status !== "unreleased") {
    args.push("--published-at", options.publishedAt ?? timestamp);
  }
  const result = cli("generate-changelog.mjs", fixture.cwd, args);
  return {
    ...result,
    output,
    markdown,
    feed: () => JSON.parse(readFileSync(output, "utf8")),
    bytes: () => readFileSync(output, "utf8"),
    notes: () => readFileSync(markdown, "utf8"),
  };
}
