#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { isDeepStrictEqual } from "node:util";
import { atomicWrite, CATEGORIES, COMMIT_PATTERN, isMain, parseArguments, parseFeed, readFeed, REPOSITORY_PATTERN, releaseMarkdown, runMain, validTimestamp, VERSION_PATTERN } from "./changelog-lib.mjs";

function git(cwd, args, input) {
  const result = spawnSync("git", args, { cwd, input, encoding: "utf8", maxBuffer: 32 * 1024 * 1024 });
  if (result.error || result.status !== 0) throw new Error(`git ${args[0]} failed: ${result.error?.message ?? result.stderr.trim()}`);
  return result.stdout.trim();
}

function resolveCommit(cwd, ref) {
  // eslint-disable-next-line no-control-regex -- Reject hidden argument bytes before invoking Git.
  if (!ref || ref.startsWith("-") || /[\u0000-\u0020\u007f]/.test(ref)) throw new Error("ref/base must not be option-like or contain whitespace");
  return git(cwd, ["rev-parse", "--verify", "--end-of-options", `${ref}^{commit}`]);
}

function isAncestor(cwd, ancestor, target) {
  const result = spawnSync("git", ["merge-base", "--is-ancestor", ancestor, target], { cwd, encoding: "utf8" });
  if (result.error || ![0, 1].includes(result.status)) throw new Error(`cannot validate history ancestor ${ancestor}: ${result.error?.message ?? result.stderr.trim()}`);
  return result.status === 0;
}

function commitItem(cwd, commit) {
  const message = git(cwd, ["show", "-s", "--format=%B", commit]);
  const subject = message.split("\n")[0].trim();
  // Git's trailer parser keeps prose that merely mentions these words out of
  // the public override path and supports conventional folded trailer values.
  const trailers = git(cwd, ["-c", "trailer.separators=:", "interpret-trailers", "--parse", "--no-divider"], message);
  const overrides = { "release-note": [], changelog: [] };
  for (const line of trailers.split("\n")) {
    const match = /^(Release-note|Changelog):\s*(.*)$/i.exec(line);
    if (match) overrides[match[1].toLowerCase()].push(match[2].trim());
  }
  const notes = overrides["release-note"];
  const categories = overrides.changelog;
  if (notes.length > 1 || categories.length > 1 || notes.some((note) => !note) || categories.some((category) => ![...CATEGORIES, "skip"].includes(category))
    || notes.length > 0 && categories[0] === "skip") throw new Error(`invalid or contradictory public changelog trailers in commit ${commit}`);
  if (categories[0] === "skip") return null;
  const conventional = /^(\w+)(?:\([^)]+\))?!?:\s*(.+)$/.exec(subject);
  const type = conventional?.[1].toLowerCase();
  if (!notes.length && !categories.length && ["docs", "test", "chore", "build", "ci"].includes(type)) return null;
  const category = categories[0] ?? (type === "feat" ? "features" : type === "fix" ? "fixes" : ["perf", "refactor", "style"].includes(type) ? "improvements" : "other");
  return { category, item: { text: notes[0] ?? conventional?.[2] ?? subject, commit } };
}

function selectBase(cwd, feed, repository, target, explicit, existing, firstBase) {
  if (explicit) {
    const base = resolveCommit(cwd, explicit);
    if (!isAncestor(cwd, base, target)) throw new Error(`--base ${base} is not an ancestor of ${target}`);
    if (existing && existing.base_commit !== base) throw new Error("immutable release content/base conflict");
    return base;
  }
  if (existing) return existing.base_commit;
  const ancestors = [...new Set(feed.releases
    .filter((release) => release.source === "fork" && release.id.startsWith(`fork:${repository}:`) && release.status !== "unreleased")
    .map((release) => release.commit))]
    .filter((commit) => isAncestor(cwd, commit, target));
  const nearest = ancestors.filter((candidate) => !ancestors.some((other) => other !== candidate && isAncestor(cwd, candidate, other)));
  if (nearest.length === 0) {
    if (firstBase) return selectBase(cwd, feed, repository, target, firstBase, existing);
    throw new Error("--base is required for the first ancestral fork release");
  }
  if (nearest.length !== 1) throw new Error("ambiguous incomparable release bases; provide --base explicitly");
  return nearest[0];
}

export function generateChangelog(options, cwd = process.cwd()) {
  if (!REPOSITORY_PATTERN.test(options.repository ?? "")) throw new Error("--repository must be owner/repo");
  if (!["unreleased", "published", "prerelease"].includes(options.status)) throw new Error("invalid --status");
  if (!options.history) throw new Error("--history is required, including on the first release");
  const history = readFeed(options.history, options.repository);
  const target = resolveCommit(cwd, options.ref);
  if (options.status !== "unreleased") {
    if (!VERSION_PATTERN.test(options.version ?? "") || options.version.includes("-dirty")) throw new Error("published version must be vX.Y.Z[-suffix] without -dirty");
    if ((options.status === "prerelease") !== options.version.includes("-")) throw new Error("version suffix and release status disagree");
    if (!validTimestamp(options["published-at"])) throw new Error("--published-at must be an explicit RFC3339 timestamp");
    const tag = options.ref.startsWith("refs/tags/") ? options.ref : `refs/tags/${options.ref}`;
    const isTag = spawnSync("git", ["show-ref", "--verify", "--quiet", tag], { cwd }).status === 0;
    if (!COMMIT_PATTERN.test(options.ref) && !isTag) throw new Error("release --ref must be an immutable full commit or tag");
    if (git(cwd, ["status", "--porcelain", "--untracked-files=no"])) throw new Error("release inputs have dirty tracked files; commit them before generating");
  } else if (options.version !== "Unreleased") {
    throw new Error("unreleased preview --version must be Unreleased");
  }
  const id = `fork:${options.repository}:${options.status === "unreleased" ? "unreleased" : options.version}`;
  const existing = history.feed.releases.find((release) => release.id === id && release.status !== "unreleased");
  if (existing && (existing.commit !== target || existing.status !== options.status)) throw new Error(`immutable release identity conflict for ${id}`);
  const base = selectBase(cwd, history.feed, options.repository, target, options.base, existing, options["first-base"]);
  if (!isAncestor(cwd, base, target)) throw new Error("history base is not an ancestor of the target");
  const commits = git(cwd, ["log", "--format=%H", "--no-merges", "--reverse", "--topo-order", `${base}..${target}`]).split("\n").filter(Boolean);
  const items = commits.map((commit) => commitItem(cwd, commit)).filter(Boolean);
  const release = {
    id,
    version: options.version,
    title: options.status === "unreleased" ? "Unreleased changes" : `Multica ${options.version}`,
    published_at: options.status === "unreleased" ? null : existing?.published_at ?? options["published-at"],
    status: options.status,
    source: "fork",
    commit: target,
    base_commit: base,
    sections: CATEGORIES.map((category) => ({ category, items: items.filter((item) => item.category === category).map((item) => item.item) })).filter((section) => section.items.length > 0),
  };
  if (existing && !isDeepStrictEqual(existing, release)) throw new Error(`immutable release content conflict for ${id}`);
  let feed = history.feed;
  let bytes = history.bytes;
  if (!existing) {
    const releases = history.feed.releases.filter((previous) => {
      if (previous.id === id) return false;
      if (options.status !== "unreleased" && previous.source === "fork" && previous.status === "unreleased") return !isAncestor(cwd, previous.commit, target);
      return true;
    });
    releases.push(release);
    releases.sort((left, right) => (left.status === "unreleased" ? -1 : 0) - (right.status === "unreleased" ? -1 : 0)
      || Date.parse(right.published_at ?? "1970-01-01T00:00:00Z") - Date.parse(left.published_at ?? "1970-01-01T00:00:00Z") || left.id.localeCompare(right.id, "en"));
    const time = release.published_at ?? git(cwd, ["show", "-s", "--format=%cI", target]);
    feed = { ...history.feed, generated_at: Date.parse(time) > Date.parse(history.feed.generated_at) ? time : history.feed.generated_at, releases };
    if (isDeepStrictEqual(feed, history.feed)) feed = history.feed;
    else bytes = Buffer.from(JSON.stringify(feed, null, 2) + "\n");
  }
  parseFeed(bytes, options.repository);
  return { feed, bytes, release, markdown: releaseMarkdown(release, options.repository) };
}

if (isMain(import.meta.url)) runMain(() => {
  if (process.argv.includes("--help")) {
    process.stdout.write("Usage: node scripts/generate-changelog.mjs --ref REF [--base ANCESTOR] --history PRIOR.json --repository OWNER/REPO --version VERSION --status unreleased|published|prerelease --output JSON --markdown MD [--published-at RFC3339]\n");
    return;
  }
  const options = parseArguments(process.argv.slice(2), ["ref", "base", "history", "repository", "version", "status", "output", "markdown", "published-at"], ["ref", "history", "repository", "version", "status", "output", "markdown"]);
  if (resolve(options.output) === resolve(options.markdown)) throw new Error("JSON and Markdown output paths must differ");
  const result = generateChangelog(options);
  for (const output of [options.output, options.markdown]) mkdirSync(dirname(resolve(output)), { recursive: true });
  atomicWrite(options.output, result.bytes);
  atomicWrite(options.markdown, result.markdown);
  process.stdout.write(`Generated ${result.release.id}; ${result.feed.releases.length} history entries\n`);
});
