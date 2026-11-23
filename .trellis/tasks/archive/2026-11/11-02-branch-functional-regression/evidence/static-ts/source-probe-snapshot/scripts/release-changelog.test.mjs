import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { createHash } from "node:crypto";
import { spawn } from "node:child_process";
import test from "node:test";
import { commit, git, gitFixture, repository, scripts } from "./changelog-test-helpers.mjs";

async function fakeGitHub(t) {
  const state = { releases: [], tags: {}, assets: new Map(), requests: [], failure: null, nextID: 1 };
  const server = createServer(async (request, response) => {
    const url = new URL(request.url, "http://localhost");
    const path = url.pathname;
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const bytes = Buffer.concat(chunks);
    state.requests.push({ method: request.method, path, body: bytes.toString("utf8") });
    function send(status, value) {
      response.writeHead(status, { "Content-Type": "application/json" });
      response.end(Buffer.isBuffer(value) ? value : JSON.stringify(value));
    }
    if (state.failure?.(request, path)) return send(503, { message: "fixture outage" });
    if (request.headers.authorization !== "Bearer fixture-token") return send(401, { message: "Bad credentials" });
    if (path === `/repos/${repository}/releases` && request.method === "GET") {
      return send(200, Number(url.searchParams.get("page")) > 1 ? [] : state.releases);
    }
    if (path.startsWith(`/repos/${repository}/git/ref/tags/`)) {
      const sha = state.tags[decodeURIComponent(path.split("/").at(-1))];
      return send(sha ? 200 : 404, sha ? { object: { type: "commit", sha } } : { message: "Not found" });
    }
    if (path === `/repos/${repository}/releases` && request.method === "POST") {
      const release = { ...JSON.parse(bytes), id: state.nextID++, assets: [], published_at: null };
      release.upload_url = `${state.url}/uploads/${release.id}/assets{?name,label}`;
      state.releases.unshift(release);
      return send(201, release);
    }
    const releaseMatch = new RegExp(`^/repos/${repository}/releases/(\\d+)$`).exec(path);
    if (releaseMatch && request.method === "PATCH") {
      const release = state.releases.find((item) => item.id === Number(releaseMatch[1]));
      Object.assign(release, JSON.parse(bytes));
      if (!release.draft && !release.published_at) release.published_at = "2026-09-13T00:00:00Z";
      return send(200, release);
    }
    const upload = /^\/uploads\/(\d+)\/assets$/.exec(path);
    if (upload && request.method === "POST") {
      const release = state.releases.find((item) => item.id === Number(upload[1]));
      const asset = { id: state.nextID++, name: url.searchParams.get("name"), size: bytes.length };
      asset.url = `${state.url}/repos/${repository}/releases/assets/${asset.id}`;
      release.assets.push(asset);
      state.assets.set(asset.id, bytes);
      return send(201, asset);
    }
    const assetMatch = new RegExp(`^/repos/${repository}/releases/assets/(\\d+)$`).exec(path);
    if (assetMatch && request.method === "GET") return send(200, state.assets.get(Number(assetMatch[1])));
    if (assetMatch && request.method === "DELETE") {
      for (const release of state.releases) release.assets = release.assets.filter((asset) => asset.id !== Number(assetMatch[1]));
      state.assets.delete(Number(assetMatch[1]));
      return send(204, null);
    }
    return send(404, { message: `unhandled fixture ${request.method} ${path}` });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  state.url = `http://127.0.0.1:${server.address().port}`;
  t.after(() => new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); }));
  return state;
}

function context(api) { return { apiURL: api.url, token: "fixture-token" }; }
function prepareOptions(fixture, tag, index) {
  return { repository, tag, seed: fixture.history, "first-base": fixture.base, "output-dir": join(fixture.cwd, `release-${index}`) };
}
function publishOptions(directory) {
  const metadata = JSON.parse(readFileSync(join(directory, "changelog-metadata.json")));
  return { repository, tag: metadata.version, input: join(directory, "changelog.json"), "notes-file": join(directory, "changelog.md"), metadata: join(directory, "changelog-metadata.json"), verify: "success", backend: "success", web: "success" };
}

async function releaseCommand(command, options, cwd, api) {
  const args = Object.entries(options).flatMap(([key, value]) => [`--${key}`, value]);
  const child = spawn(process.execPath, [join(scripts, "release-changelog.mjs"), command, ...args], {
    cwd,
    env: { ...process.env, GITHUB_API_URL: api.url, GITHUB_TOKEN: "fixture-token" },
  });
  let output = "";
  child.stdout.on("data", (chunk) => { output += chunk; });
  child.stderr.on("data", (chunk) => { output += chunk; });
  const [status] = await once(child, "close");
  assert.equal(status, 0, output);
}

test("real prepare and publication preserve prerelease/stable history and older retries", async (t) => {
  const { prepareRelease, publishRelease } = await import("./release-changelog.mjs");
  const fixture = gitFixture(t);
  const api = await fakeGitHub(t);
  const versions = ["v1.0.0", "v1.1.0-beta.1", "v1.1.0"];
  for (const [index, tag] of versions.entries()) {
    const revision = commit(fixture.cwd, `${index === 2 ? "fix" : "feat"}: change ${index}`);
    git(fixture.cwd, "tag", tag, revision);
    api.tags[tag] = revision;
    const options = prepareOptions(fixture, tag, index);
    const prepared = await prepareRelease(options, context(api), fixture.cwd);
    const bytes = readFileSync(join(options["output-dir"], "changelog.json"));
    const metadata = JSON.parse(readFileSync(join(options["output-dir"], "changelog-metadata.json")));
    assert.equal(metadata.commit, revision);
    assert.equal(prepared.feed.releases.filter((entry) => entry.source === "fork").length, index + 1);
    await publishRelease(publishOptions(options["output-dir"]), context(api));
    const release = api.releases.find((item) => item.tag_name === tag);
    assert.equal(release.draft, false);
    assert.equal(release.prerelease, tag.includes("-"));
    assert.equal(release.make_latest, tag.includes("-") ? "false" : "true");
    assert.deepEqual(api.assets.get(release.assets.find((asset) => asset.name === "changelog.json").id), bytes);
    const create = api.requests.find((request) => request.method === "POST" && request.path.endsWith("/releases") && JSON.parse(request.body).tag_name === tag);
    assert.equal(JSON.parse(create.body).draft, true);
    assert.equal(release.assets.length, 3);
  }
  const older = prepareOptions(fixture, versions[0], "retry");
  const retry = await prepareRelease(older, context(api), fixture.cwd);
  assert.equal(retry.feed.releases.length, 4);
  await publishRelease(publishOptions(older["output-dir"]), context(api));
  assert.equal(api.releases.length, 3);
  const mutations = api.requests.filter((request) => request.method !== "GET").length;
  await publishRelease(publishOptions(older["output-dir"]), context(api));
  assert.equal(api.requests.filter((request) => request.method !== "GET").length, mutations, "exact retries require no remote mutation");
});

test("history retrieval only bootstraps after a verified empty release list", async (t) => {
  const { prepareRelease } = await import("./release-changelog.mjs");
  const fixture = gitFixture(t);
  const target = commit(fixture.cwd, "feat: release");
  git(fixture.cwd, "tag", "v1.0.0", target);
  const api = await fakeGitHub(t);
  api.tags["v1.0.0"] = target;
  const options = prepareOptions(fixture, "v1.0.0", "first");
  await assert.rejects(prepareRelease(options, { ...context(api), token: "wrong" }, fixture.cwd), /401|credentials/);
  api.failure = (_request, path) => path.endsWith("/releases");
  await assert.rejects(prepareRelease(options, context(api), fixture.cwd), /503|outage/);
  api.failure = null;
  api.releases.push({});
  await assert.rejects(prepareRelease(options, context(api), fixture.cwd), /invalid|malformed/);
  api.releases.pop();
  api.releases.push({ id: 88, tag_name: "v0.9.0", draft: false, prerelease: false, assets: [], published_at: "2026-09-12T00:00:00Z" });
  await assert.rejects(prepareRelease(options, context(api), fixture.cwd), /missing|required|asset/);
  api.releases[0].draft = true;
  const prepared = await prepareRelease(options, context(api), fixture.cwd);
  assert.equal(prepared.feed.releases.length, 2, "unpublished drafts do not masquerade as prior releases");
});

test("failed prerequisites, tag drift, damaged metadata and upload failures prevent publication", async (t) => {
  const { prepareRelease, publishRelease } = await import("./release-changelog.mjs");
  const fixture = gitFixture(t);
  const target = commit(fixture.cwd, "feat: release");
  git(fixture.cwd, "tag", "v1.0.0", target);
  const api = await fakeGitHub(t);
  api.tags["v1.0.0"] = target;
  const options = prepareOptions(fixture, "v1.0.0", "first");
  await prepareRelease(options, context(api), fixture.cwd);
  const publication = publishOptions(options["output-dir"]);
  await assert.rejects(publishRelease({ ...publication, repository: "wrong/repo" }, context(api)), /repository|metadata/);
  await assert.rejects(publishRelease({ ...publication, tag: "v99.0.0" }, context(api)), /tag|metadata/);
  for (const key of ["verify", "backend", "web"]) {
    await assert.rejects(publishRelease({ ...publication, [key]: "failure" }, context(api)), /prerequisite/);
  }
  assert.equal(api.releases.length, 0);
  api.tags["v1.0.0"] = fixture.base;
  await assert.rejects(publishRelease(publication, context(api)), /tag|revision|immutable/);
  api.tags["v1.0.0"] = target;
  api.failure = (request, path) => request.method === "POST" && path.includes("/uploads/");
  await assert.rejects(publishRelease(publication, context(api)), /503/);
  assert.equal(api.releases[0].draft, true);
  api.failure = null;
  await publishRelease(publication, context(api));
  assert.equal(api.releases[0].draft, false);
  const asset = api.releases[0].assets.find((item) => item.name === "changelog.json");
  api.assets.set(asset.id, Buffer.from("{}"));
  await assert.rejects(prepareRelease(prepareOptions(fixture, "v1.0.0", "bad"), context(api), fixture.cwd), /schema|digest|hash|changelog/);
  writeFileSync(publication["notes-file"], "tampered notes");
  await assert.rejects(publishRelease(publication, context(api)), /digest|hash|notes/);
});

test("retrying only an older failed publication rejects its stale built history without mutation", async (t) => {
  const { prepareRelease, publishRelease } = await import("./release-changelog.mjs");
  const fixture = gitFixture(t);
  const api = await fakeGitHub(t);
  const first = commit(fixture.cwd, "feat: first release");
  git(fixture.cwd, "tag", "v1.0.0", first);
  api.tags["v1.0.0"] = first;
  const firstOptions = prepareOptions(fixture, "v1.0.0", "deferred-first");
  await prepareRelease(firstOptions, context(api), fixture.cwd);
  const deferredPublication = publishOptions(firstOptions["output-dir"]);
  api.failure = (request, path) => request.method === "POST" && path.includes("/uploads/");
  await assert.rejects(publishRelease(deferredPublication, context(api)), /503/);
  assert.equal(api.releases.find((release) => release.tag_name === "v1.0.0").draft, true);
  api.failure = null;

  const later = commit(fixture.cwd, "feat: later release");
  git(fixture.cwd, "tag", "v1.1.0", later);
  api.tags["v1.1.0"] = later;
  const laterOptions = prepareOptions(fixture, "v1.1.0", "successful-later");
  await prepareRelease(laterOptions, context(api), fixture.cwd);
  await publishRelease(publishOptions(laterOptions["output-dir"]), context(api));
  const previousState = JSON.stringify(api.releases);
  const previousMutations = api.requests.filter((request) => request.method !== "GET").length;
  await assert.rejects(publishRelease(deferredPublication, context(api)), /stale|history.*prepar|prepar.*build/i);
  assert.equal(api.requests.filter((request) => request.method !== "GET").length, previousMutations);
  assert.equal(JSON.stringify(api.releases), previousState);
  assert.equal(api.releases.find((release) => release.tag_name === "v1.1.0").make_latest, "true");

  const refreshedOptions = prepareOptions(fixture, "v1.0.0", "fresh-first");
  refreshedOptions.seed = join(fixture.cwd, "unused-seed-after-publication.json");
  await assert.rejects(prepareRelease({ ...refreshedOptions, "first-base": later }, context(api), fixture.cwd), /ancestor/);
  await releaseCommand("prepare", refreshedOptions, fixture.cwd, api);
  const refreshedFeed = JSON.parse(readFileSync(join(refreshedOptions["output-dir"], "changelog.json")));
  assert.ok(refreshedFeed.releases.some((release) => release.version === "v1.1.0"));
  const freshPublication = publishOptions(refreshedOptions["output-dir"]);
  const validFeed = readFileSync(freshPublication.input);
  const validMetadata = readFileSync(freshPublication.metadata);
  const conflicting = JSON.parse(validFeed);
  conflicting.releases.find((release) => release.version === "v1.1.0").title = "Rewritten published title";
  const conflictingBytes = Buffer.from(JSON.stringify(conflicting));
  const metadata = JSON.parse(validMetadata);
  metadata.feed_sha256 = createHash("sha256").update(conflictingBytes).digest("hex");
  writeFileSync(freshPublication.input, conflictingBytes);
  writeFileSync(freshPublication.metadata, JSON.stringify(metadata));
  await assert.rejects(publishRelease(freshPublication, context(api)), /stale|conflicting/);
  assert.equal(api.requests.filter((request) => request.method !== "GET").length, previousMutations);
  writeFileSync(freshPublication.input, validFeed);
  writeFileSync(freshPublication.metadata, validMetadata);
  await releaseCommand("publish", freshPublication, fixture.cwd, api);
  assert.equal(api.releases.find((release) => release.tag_name === "v1.0.0").make_latest, "false", "an older stable release must not regress GitHub latest even when prepared later");
  assert.equal(api.releases.find((release) => release.tag_name === "v1.0.0").draft, false);
});
