#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { isDeepStrictEqual } from "node:util";
import { generateChangelog } from "./generate-changelog.mjs";
import { atomicWrite, COMMIT_PATTERN, isMain, MAX_FEED_BYTES, parseArguments, parseFeed, readBounded, readFeed, REPOSITORY_PATTERN, runMain, validTimestamp, VERSION_PATTERN } from "./changelog-lib.mjs";

const ASSETS = ["changelog.json", "changelog.md", "changelog-metadata.json"];
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");
const releaseIdentity = (metadata) => `fork:${metadata.repository}:${metadata.version}`;

function github(context = {}) {
  const base = new URL(context.apiURL ?? process.env.GITHUB_API_URL ?? "https://api.github.com");
  if (base.protocol !== "https:" && !(base.protocol === "http:" && ["127.0.0.1", "localhost", "[::1]"].includes(base.hostname))) throw new Error("GitHub API URL must use HTTPS");
  const token = context.token ?? process.env.GITHUB_TOKEN;
  if (!token) throw new Error("GITHUB_TOKEN is required");
  async function request(path, { method = "GET", body, binary = false } = {}) {
    const url = path.startsWith("http") ? new URL(path) : new URL(base.href.replace(/\/$/, "") + path);
    const uploads = base.hostname === "api.github.com" && url.origin === "https://uploads.github.com";
    if (url.origin !== base.origin && !uploads) throw new Error("GitHub asset URL has an unexpected origin");
    const response = await fetch(url, {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: binary ? "application/octet-stream" : "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        ...(body ? { "Content-Type": Buffer.isBuffer(body) ? "application/octet-stream" : "application/json" } : {}),
      },
      body: body ? Buffer.isBuffer(body) ? body : JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(30_000),
    });
    if (!response.ok) {
      await response.body?.cancel();
      throw new Error(`GitHub ${method} ${url.pathname} returned HTTP ${response.status}; history was not reset`);
    }
    if (response.status === 204) return null;
    const chunks = [];
    let length = 0;
    for await (const chunk of response.body) {
      length += chunk.length;
      if (length > MAX_FEED_BYTES) throw new Error("GitHub changelog response exceeds the 2 MiB size limit");
      chunks.push(chunk);
    }
    const bytes = Buffer.concat(chunks);
    return binary ? bytes : JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
  }
  async function releases(repository) {
    const all = [];
    for (let page = 1; page <= 100; page++) {
      const batch = await request(`/repos/${repository}/releases?per_page=100&page=${page}`);
      if (!Array.isArray(batch)) throw new Error("invalid GitHub releases response");
      for (const release of batch) {
        if (!release || !Number.isSafeInteger(release.id) || typeof release.draft !== "boolean" || typeof release.prerelease !== "boolean"
          || !VERSION_PATTERN.test(release.tag_name ?? "") || !Array.isArray(release.assets)) throw new Error("invalid GitHub release metadata; refusing seed fallback");
        if (all.some((existing) => existing.tag_name === release.tag_name)) throw new Error("duplicate GitHub release tags; refusing ambiguous history");
      }
      all.push(...batch);
      if (batch.length < 100) return all;
    }
    throw new Error("GitHub release pagination limit exceeded; refusing incomplete history");
  }
  async function tagCommit(repository, tag) {
    let object = (await request(`/repos/${repository}/git/ref/tags/${encodeURIComponent(tag)}`)).object;
    for (let depth = 0; depth < 10 && object?.type === "tag"; depth++) {
      if (!COMMIT_PATTERN.test(object.sha ?? "")) throw new Error("invalid annotated tag object");
      object = (await request(`/repos/${repository}/git/tags/${object.sha}`)).object;
    }
    if (object?.type !== "commit" || !COMMIT_PATTERN.test(object.sha ?? "")) throw new Error("GitHub tag does not resolve to an immutable commit");
    return object.sha;
  }
  return { request, releases, tagCommit };
}

function validateArtifact(input, notes, metadataBytes, expectedRepository) {
  const metadata = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(metadataBytes));
  if (metadata.schema_version !== 1 || !REPOSITORY_PATTERN.test(metadata.repository ?? "") || !VERSION_PATTERN.test(metadata.version ?? "")
    || metadata.version.includes("-dirty") || (metadata.status === "prerelease") !== metadata.version.includes("-")
    || metadata.repository !== (expectedRepository ?? metadata.repository) || !["published", "prerelease"].includes(metadata.status)
    || !COMMIT_PATTERN.test(metadata.commit ?? "") || !COMMIT_PATTERN.test(metadata.base_commit ?? "") || !validTimestamp(metadata.published_at)) throw new Error("invalid changelog immutable build metadata");
  if (metadata.feed_sha256 !== digest(input) || metadata.markdown_sha256 !== digest(notes)) throw new Error("changelog artifact digest mismatch");
  const feed = parseFeed(input, metadata.repository);
  const entry = feed.releases.find((release) => release.id === releaseIdentity(metadata));
  if (!entry || ["version", "status", "commit", "base_commit", "published_at"].some((key) => entry[key] !== metadata[key]) || metadata.release_sha256 !== digest(JSON.stringify(entry))) throw new Error("changelog release identity/content differs from immutable metadata");
  return { feed, entry, metadata, bytes: input, notes, metadataBytes };
}

async function downloadArtifact(api, repository, release) {
  if (!Array.isArray(release.assets)) throw new Error(`release ${release.tag_name} has invalid assets`);
  const files = [];
  for (const name of ASSETS) {
    const matches = release.assets.filter((asset) => asset.name === name);
    if (matches.length !== 1 || !Number.isSafeInteger(matches[0].id)) throw new Error(`release ${release.tag_name} is missing required unique asset ${name}`);
    files.push(await api.request(`/repos/${repository}/releases/assets/${matches[0].id}`, { binary: true }));
  }
  const artifact = validateArtifact(...files, repository);
  if (release.tag_name !== artifact.metadata.version || release.prerelease !== (artifact.metadata.status === "prerelease")) throw new Error("GitHub release status/tag conflicts with changelog metadata");
  return artifact;
}

function includesPublished(candidate, previous) {
  return previous.releases.filter((entry) => entry.status !== "unreleased").every((entry) => candidate.releases.some((other) => isDeepStrictEqual(entry, other)));
}

function newerStableVersion(version, other) {
  const left = version.slice(1).split(".").map(BigInt);
  const right = other.slice(1).split(".").map(BigInt);
  const different = left.findIndex((part, index) => part !== right[index]);
  return different !== -1 && left[different] > right[different];
}

async function discoverHistory(api, repository, releases, seed) {
  const published = releases.filter((release) => release.draft === false);
  if (published.length === 0) return readFeed(seed, repository);
  const artifacts = [];
  for (const release of published) artifacts.push(await downloadArtifact(api, repository, release));
  // Out-of-order tag builds and older retries make GitHub's /latest endpoint
  // insufficient. Choose a proven cumulative superset, including prereleases.
  const complete = artifacts.filter((candidate) => artifacts.every((prior) => includesPublished(candidate.feed, prior.feed)));
  if (complete.length === 0) throw new Error("published release histories conflict or lack cumulative entries; repair history explicitly");
  complete.sort((left, right) => Date.parse(right.feed.generated_at) - Date.parse(left.feed.generated_at) || right.feed.releases.length - left.feed.releases.length);
  return complete[0];
}

export async function prepareRelease(options, context = {}, cwd = process.cwd()) {
  if (!REPOSITORY_PATTERN.test(options.repository ?? "") || !VERSION_PATTERN.test(options.tag ?? "") || options.tag.includes("-dirty")) throw new Error("prepare requires a valid --repository and --tag");
  const api = github(context);
  const releases = await api.releases(options.repository);
  const history = await discoverHistory(api, options.repository, releases, options.seed);
  const commit = execFileSync("git", ["rev-parse", "--verify", "--end-of-options", `refs/tags/${options.tag}^{commit}`], { cwd, encoding: "utf8" }).trim();
  if (await api.tagCommit(options.repository, options.tag) !== commit) throw new Error("remote tag differs from the immutable checkout revision");
  const existing = history.feed.releases.find((entry) => entry.id === `fork:${options.repository}:${options.tag}`);
  let publishedAt = existing?.published_at;
  const retry = releases.find((release) => release.tag_name === options.tag);
  if (!publishedAt && retry?.assets?.some((asset) => asset.name === "changelog-metadata.json")) {
    publishedAt = (await downloadArtifact(api, options.repository, retry)).metadata.published_at;
  }
  publishedAt ??= new Date().toISOString();
  const directory = resolve(cwd, options["output-dir"]);
  mkdirSync(directory, { recursive: true });
  const historyPath = join(directory, "history.json");
  atomicWrite(historyPath, history.bytes);
  const generated = generateChangelog({
    repository: options.repository,
    ref: commit,
    version: options.tag,
    status: options.tag.includes("-") ? "prerelease" : "published",
    history: historyPath,
    base: options.base,
    "first-base": options["first-base"],
    "published-at": publishedAt,
  }, cwd);
  const metadata = {
    schema_version: 1,
    repository: options.repository,
    version: generated.release.version,
    status: generated.release.status,
    commit: generated.release.commit,
    base_commit: generated.release.base_commit,
    published_at: generated.release.published_at,
    release_sha256: digest(JSON.stringify(generated.release)),
    feed_sha256: digest(generated.bytes),
    markdown_sha256: digest(generated.markdown),
  };
  atomicWrite(join(directory, ASSETS[0]), generated.bytes);
  atomicWrite(join(directory, ASSETS[1]), generated.markdown);
  atomicWrite(join(directory, ASSETS[2]), JSON.stringify(metadata, null, 2) + "\n");
  return generated;
}

export async function publishRelease(options, context = {}) {
  if (["verify", "backend", "web"].some((job) => options[job] !== "success")) throw new Error("required release prerequisite did not succeed");
  if (!REPOSITORY_PATTERN.test(options.repository ?? "") || !VERSION_PATTERN.test(options.tag ?? "")) throw new Error("publish requires an explicit --repository and --tag");
  const artifact = validateArtifact(readBounded(options.input), readBounded(options["notes-file"]), readBounded(options.metadata), options.repository);
  const { metadata, feed, entry } = artifact;
  if (metadata.version !== options.tag) throw new Error("requested tag differs from immutable changelog metadata");
  const api = github(context);
  if (await api.tagCommit(metadata.repository, metadata.version) !== metadata.commit) throw new Error("remote tag differs from immutable build revision");
  const releases = await api.releases(metadata.repository);
  // Re-running only a failed publication can reuse artifacts built before a
  // newer workflow finished. Revalidate at this final boundary without
  // regenerating bytes that no longer match the successfully built images.
  if (releases.some((release) => !release.draft)) {
    const current = await discoverHistory(api, metadata.repository, releases);
    if (!includesPublished(feed, current.feed)) throw new Error("built changelog history is stale or conflicting; rerun full preparation and all builds before publishing");
  }
  let release = releases.find((item) => item.tag_name === metadata.version);
  const body = artifact.notes.toString("utf8");
  const makeLatest = metadata.status === "published"
    && !releases.some((item) => !item.draft && !item.prerelease && newerStableVersion(item.tag_name, metadata.version)) ? "true" : "false";
  if (release && !release.draft) {
    const previous = await downloadArtifact(api, metadata.repository, release);
    if (!isDeepStrictEqual(previous.entry, entry) || !includesPublished(feed, previous.feed)) throw new Error("immutable published release conflict or history regression");
    if (previous.bytes.equals(artifact.bytes) && previous.notes.equals(artifact.notes) && previous.metadataBytes.equals(artifact.metadataBytes)) return release;
  }
  if (!release) {
    release = await api.request(`/repos/${metadata.repository}/releases`, {
      method: "POST",
      body: { tag_name: metadata.version, target_commitish: metadata.commit, name: entry.title, body, draft: true, prerelease: metadata.status === "prerelease", make_latest: "false" },
    });
  } else if (release.target_commitish !== metadata.commit || release.prerelease !== (metadata.status === "prerelease")) {
    throw new Error("existing draft/release conflicts with immutable revision or status");
  }
  if (!Number.isSafeInteger(release.id) || typeof release.upload_url !== "string" || !Array.isArray(release.assets)) throw new Error("invalid GitHub draft release response");
  const upload = release.upload_url.replace(/\{.*$/, "");
  const files = [artifact.bytes, artifact.notes, artifact.metadataBytes];
  for (const [index, name] of ASSETS.entries()) {
    const previous = release.assets.find((asset) => asset.name === name);
    if (previous) {
      const bytes = await api.request(`/repos/${metadata.repository}/releases/assets/${previous.id}`, { binary: true });
      if (bytes.equals(files[index])) continue;
      await api.request(`/repos/${metadata.repository}/releases/assets/${previous.id}`, { method: "DELETE" });
    }
    await api.request(`${upload}?name=${encodeURIComponent(name)}`, { method: "POST", body: files[index] });
  }
  return api.request(`/repos/${metadata.repository}/releases/${release.id}`, {
    method: "PATCH",
    body: { draft: false, prerelease: metadata.status === "prerelease", name: entry.title, body, make_latest: makeLatest },
  });
}

if (isMain(import.meta.url)) runMain(async () => {
  const [command, ...args] = process.argv.slice(2);
  if (command === "--help") {
    process.stdout.write("Usage: release-changelog.mjs prepare --repository OWNER/REPO --tag TAG --seed JSON --first-base COMMIT --output-dir DIR [--base COMMIT]\n       release-changelog.mjs publish --repository OWNER/REPO --tag TAG --input JSON --notes-file MD --metadata JSON --verify success --backend success --web success\nGITHUB_TOKEN is required. Both commands use this repository's exact immutable tag.\n");
  } else if (command === "prepare") {
    const options = parseArguments(args, ["repository", "tag", "seed", "first-base", "output-dir", "base"], ["repository", "tag", "seed", "first-base", "output-dir"]);
    const result = await prepareRelease(options);
    process.stdout.write(`Prepared ${result.release.id}; ${result.feed.releases.length} cumulative entries\n`);
  } else if (command === "publish") {
    const options = parseArguments(args, ["repository", "tag", "input", "notes-file", "metadata", "verify", "backend", "web"], ["repository", "tag", "input", "notes-file", "metadata", "verify", "backend", "web"]);
    const result = await publishRelease(options);
    process.stdout.write(`Published verified changelog for ${result.tag_name}\n`);
  } else throw new Error("expected prepare or publish command");
});
