import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { request } from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createMaterialGitHub } from "./material-github.mjs";

test("controlled GitHub preserves exact immutable files, current-head merge and review history", async (t) => {
  const dir = mkdtempSync(join(tmpdir(), "multica-github-fixture-"));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-keyout", join(dir, "key"), "-out", join(dir, "cert"), "-subj", "/CN=localhost", "-addext", "subjectAltName=IP:127.0.0.1"], { stdio: "ignore" });
  const cert = readFileSync(join(dir, "cert"));
  const fixture = createMaterialGitHub({ key: readFileSync(join(dir, "key")), cert, token: "fixture-token", controlToken: "control-token" });
  const { origin, initial_commit } = await fixture.listen(); t.after(() => fixture.close());
  async function call(method, path, body) { return new Promise((resolve, reject) => { const req = request(origin + path, { method, ca: cert, headers: { authorization: "Bearer fixture-token", "content-type": "application/json" } }, res => { let raw = ""; res.on("data", b => raw += b); res.on("end", () => resolve({ status: res.statusCode, data: JSON.parse(raw) })); }); req.on("error", reject); req.end(body ? JSON.stringify(body) : undefined); }); }
  const unicodeBody = Buffer.from(JSON.stringify({ action: "seed", files: { "existing/utf8.md": "中文证据" } }));
  const splitAt = unicodeBody.indexOf(Buffer.from("证")) + 1;
  await new Promise((resolve, reject) => {
    const req = request(origin + "/__control", { method: "POST", ca: cert, headers: { authorization: "Bearer control-token", "content-type": "application/json" } }, response => { response.resume(); response.on("end", () => response.statusCode === 200 ? resolve() : reject(Error("Seed refused"))); });
    req.on("error", reject); req.write(unicodeBody.subarray(0, splitAt)); setTimeout(() => req.end(unicodeBody.subarray(splitAt)), 20);
  });
  const seeded = fixture.snapshot().commits.at(-1);
  assert.equal(seeded.files["existing/utf8.md"].content_sha256, (await import("node:crypto")).createHash("sha256").update("中文证据").digest("hex"));
  const base = "/repos/acme/materials";
  const ref = await call("GET", base + "/git/ref/heads/main"); assert.equal(ref.data.object.sha, seeded.id);
  const parent = (await call("GET", base + "/git/commits/" + seeded.id)).data;
  await call("POST", base + "/git/refs", { ref: "refs/heads/multica/candidate", sha: seeded.id });
  const path = "requirements/REQ-1/prd.md", content = "# Exact UTF-8 requirement\n中文";
  const tree = (await call("POST", base + "/git/trees", { base_tree: parent.tree.sha, tree: [{ path, type: "blob", mode: "100644", content }] })).data;
  const c = (await call("POST", base + "/git/commits", { tree: tree.sha, parents: [seeded.id], message: "approved candidate" })).data;
  assert.equal((await call("PATCH", base + "/git/refs/heads/multica/candidate", { sha: c.sha, force: false })).status, 200);
  const pr = (await call("POST", base + "/pulls", { head: "multica/candidate", base: "main", title: "Candidate", body: "operation marker" })).data;
  assert.equal(pr.head.sha, c.sha); assert.equal(pr.changed_files, 1);
  assert.deepEqual((await call("GET", base + "/pulls/1/files")).data, [{ filename: path, status: "added" }]);
  let sha = tree.sha;
  for (const part of path.split("/")) { const listing = (await call("GET", base + "/git/trees/" + sha)).data; assert.equal(listing.truncated, false); sha = listing.tree.find(e => e.path === part).sha; }
  assert.equal(Buffer.from((await call("GET", base + "/git/blobs/" + sha)).data.content, "base64").toString(), content);
  await call("POST", base + "/issues/1/comments", { body: "Human approval marker" });
  assert.equal((await call("PUT", base + "/pulls/1/merge", { sha: initial_commit })).status, 409);
  assert.equal((await call("GET", base + "/pulls/1")).data.merged, false);
  const merged = await call("PUT", base + "/pulls/1/merge", { sha: c.sha }); assert.equal(merged.data.merged, true);
  const observed = (await call("GET", base + "/pulls/1")).data; assert.equal(observed.merge_commit_sha, merged.data.sha); assert.equal(observed.merged_by.login, "multica-material-fixture");
  assert.equal(fixture.snapshot().commits.find(x => x.id === merged.data.sha).files[path].content_sha256, fixture.snapshot().commits.find(x => x.id === c.sha).files[path].content_sha256);
  assert.equal((await call("GET", base + "/issues/1/comments")).data[0].body, "Human approval marker");
  assert.deepEqual(fixture.snapshot().effects, { branch: 1, commit: 1, review: 1, note: 1, merge: 1 });
  assert.equal((await call("POST", "/graphql", { query: "mutation{evil}" })).status, 400);
});
