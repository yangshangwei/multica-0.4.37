import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { request } from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createMaterialGitLab } from "./material-gitlab.mjs";

test("controlled HTTPS GitLab preserves publication, uncertain replies and original bytes", async (t) => {
  const directory = mkdtempSync(join(tmpdir(), "multica-material-tls-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const keyPath = join(directory, "key.pem");
  const certPath = join(directory, "cert.pem");
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-keyout", keyPath, "-out", certPath, "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1"], { stdio: "ignore" });
  const cert = readFileSync(certPath);
  const fixture = createMaterialGitLab({ key: readFileSync(keyPath), cert, token: "isolated-provider-token", controlToken: "isolated-control-token" });
  const { origin, initial_commit: base } = await fixture.listen();
  t.after(() => fixture.close());

  async function call(method, path, body, control = false) {
    const bytes = body === undefined ? null : JSON.stringify(body);
    return await new Promise((resolve, reject) => {
      const req = request(new URL(path, origin), {
        method,
        ca: cert,
        headers: { "content-type": "application/json", ...(control ? { authorization: "Bearer isolated-control-token" } : { "private-token": "isolated-provider-token" }) },
      }, (res) => {
        let output = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => { output += chunk; });
        res.on("end", () => { resolve({ status: res.statusCode, body: JSON.parse(output) }); });
      });
      req.on("error", reject);
      req.end(bytes);
    });
  }
  const project = "/api/v4/projects/acme%2Fmaterials";
  const file = "requirements/REQ-1/prd.md";
  assert.equal((await call("GET", "/api/v4/user")).body.username, "multica-material-fixture");
  assert.equal((await call("GET", `${project}/repository/branches/main`)).body.commit.id, base);
  assert.equal((await call("POST", `${project}/repository/branches`, { branch: "multica/rev-1", ref: base })).status, 201);
  assert.equal((await call("POST", `${project}/repository/branches`, { branch: "multica/rev-1", ref: base })).status, 400);
  assert.equal((await call("POST", "/__control", { action: "fail_next", phase: "commit", mode: "http_error", status: 503 }, true)).status, 200);
  assert.equal((await call("POST", `${project}/repository/commits`, {
    branch: "multica/rev-1", commit_message: "Candidate revision", force: false,
    actions: [{ action: "create", file_path: file, content: "# PRD\n\nAC-1", encoding: "text" }],
  })).status, 503);
  const head = (await call("GET", `${project}/repository/branches/multica%2Frev-1`)).body.commit.id;
  assert.notEqual(head, base);
  const read = (await call("GET", `${project}/repository/files/${encodeURIComponent(file)}?ref=${head}`)).body;
  assert.equal(Buffer.from(read.content, "base64").toString("utf8"), "# PRD\n\nAC-1");
  assert.equal(read.commit_id, head);
  const tree = (await call("GET", `${project}/repository/tree?ref=${head}&path=requirements%2FREQ-1`)).body;
  assert.equal(tree[0].path, file);
  assert.equal(tree[0].mode, "100644");
  const mr = await call("POST", `${project}/merge_requests`, { source_branch: "multica/rev-1", target_branch: "main", title: "Review requirement", description: "candidate marker" });
  assert.equal(mr.status, 201);
  assert.equal(mr.body.sha, head);
  const mrPath = `${project}/merge_requests/${mr.body.iid}`;
  assert.equal((await call("GET", `${mrPath}/diffs`)).body.length, 1);
  assert.equal((await call("POST", `${mrPath}/notes`, { body: "Human decision, synchronized by the system" })).status, 201);
  await call("POST", "/__control", { action: "set_gate", number: mr.body.iid, status: "not_approved" }, true);
  assert.equal((await call("PUT", `${mrPath}/merge`, { sha: head })).status, 405);
  await call("POST", "/__control", { action: "set_gate", number: mr.body.iid, status: "mergeable" }, true);
  assert.equal((await call("PUT", `${mrPath}/merge`, { sha: base })).status, 409);
  await call("POST", "/__control", { action: "fail_next", phase: "merge", mode: "http_error", status: 503 }, true);
  assert.equal((await call("PUT", `${mrPath}/merge`, { sha: head })).status, 503);
  const merged = (await call("GET", mrPath)).body;
  assert.equal(merged.state, "merged");
  assert.notEqual(merged.merge_commit_sha, head);
  const atMerge = (await call("GET", `${project}/repository/files/${encodeURIComponent(file)}?ref=${merged.merge_commit_sha}`)).body;
  assert.equal(atMerge.content, read.content);
  await call("POST", "/__control", { action: "edit", branch: "main", files: { [file]: "Later definition" } }, true);
  assert.equal((await call("GET", `${project}/repository/files/${encodeURIComponent(file)}?ref=${head}`)).body.content, read.content);
  const state = (await call("GET", "/__control/snapshot", undefined, true)).body;
  assert.deepEqual(state.effects, { branch: 1, commit: 1, review: 1, note: 1, merge: 1, external_edit: 1 });
  assert.equal(state.commits[1].files[file].content, undefined);
  assert.equal(JSON.stringify(state).includes("isolated-provider-token"), false);
  assert.equal((await call("GET", "/__control/snapshot")).status, 401);
});
