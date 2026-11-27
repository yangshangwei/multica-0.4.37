import { readFileSync, appendFileSync } from "node:fs";
import { request } from "node:https";
import { randomUUID } from "node:crypto";

// A test-created native backend executable. Only this process performs fixture
// mutations; the separate glab invocation is strictly a read-only observer.
const config = JSON.parse(readFileSync(process.env.MATERIAL_RUNTIME_CONFIG, "utf8"));
const origin = "https://127.0.0.1:" + config.ports.gitlab;
const github = config.provider === "github";
function remote(path, method = "GET", body) {
  return new Promise((resolve, reject) => {
    const req = request(origin + (github ? "/" : "/api/v4/") + path.replace(/^\//, ""), { method, ca: readFileSync(config.env.SSL_CERT_FILE), headers: { ...(github ? { authorization: "Bearer " + config.git_token } : { "private-token": config.git_token }), "Content-Type": "application/json" } }, (res) => {
      let data = ""; res.setEncoding("utf8"); res.on("data", chunk => data += chunk); res.on("end", () => resolve({ status: res.statusCode, headers: res.headers, data }));
    });
    req.on("error", reject); req.end(body ? JSON.stringify(body) : undefined);
  });
}
const args = process.argv.slice(2);
if (args[0] === "api") {
  const method = args[args.indexOf("--method") + 1];
  let body;
  if (method !== "GET") {
    if (!github || args[1] !== "/graphql" || method !== "POST") throw Error("Observer writes forbidden");
    let raw = ""; for await (const chunk of process.stdin) { raw += chunk; if (raw.length > 32000) throw Error("Oversize query"); }
    body = JSON.parse(raw);
    if (!body.query.startsWith("query(") || body.query.includes("mutation")) throw Error("Observer mutation forbidden");
  }
  if (args[args.indexOf("--hostname") + 1] !== (github ? "github.com" : new URL(origin).host)) throw Error("Observer host mismatch");
  const r = await remote(args[1], method, body);
  process.stdout.write(`HTTP/1.1 ${r.status} Fixture\r\nContent-Type: application/json\r\n`);
  for (const key of ["x-next-page", "x-total", "x-total-pages"]) if (r.headers[key]) process.stdout.write(`${key}: ${r.headers[key]}\r\n`);
  process.stdout.write(`Content-Length: ${Buffer.byteLength(r.data)}\r\n\r\n${r.data}`);
  process.exit(r.status < 400 ? 0 : 1);
}
let input = "";
for await (const chunk of process.stdin) { input += chunk; if (input.includes("\n")) break; }
const match = input.match(/multica project material operation ([a-f0-9-]{36})/);
if (!match) throw Error("Missing material descriptor in native prompt");
async function local(action) {
  const response = await fetch(`http://127.0.0.1:${process.env.MULTICA_DAEMON_PORT}/material`, { method: "POST", headers: { authorization: "Bearer " + process.env.MULTICA_TOKEN, "Content-Type": "application/json" }, body: JSON.stringify({ operation_id: match[1], action, request_id: randomUUID() }) });
  if (!response.ok) throw Error(`Local ${action}: ${response.status} ${await response.text()}`);
  return response.json();
}
const e = await local("operation");
const base = github ? "repos/" + e.repository : "projects/" + encodeURIComponent(e.repository);
async function api(path, method, data) {
  const r = await remote(base + path, method, data);
  if (r.status >= 400) throw Error(`Fixture ${method ?? "GET"} ${path}: ${r.status}`);
  return JSON.parse(r.data);
}
if (e.action === "merge") await local("observe");
if (e.action !== "verify_baseline") await local("permit");
switch (e.action) {
  case "publish": {
    if (github) {
      const target = await api("/git/ref/heads/" + e.base_branch);
      const parent = await api("/git/commits/" + target.object.sha);
      await api("/git/refs", "POST", { ref: "refs/heads/" + e.source_branch, sha: target.object.sha });
      const tree = await api("/git/trees", "POST", { base_tree: parent.tree.sha, tree: e.files.map(f => ({ path: f.path, mode: f.mode, type: "blob", content: f.content })) });
      const commit = await api("/git/commits", "POST", { tree: tree.sha, parents: [target.object.sha], message: e.operation_key });
      await api("/git/refs/heads/" + e.source_branch, "PATCH", { sha: commit.sha, force: false });
      await api("/pulls", "POST", { head: e.source_branch, base: e.base_branch, title: e.title, body: e.body });
      break;
    }
    const target = await api("/repository/branches/" + encodeURIComponent(e.base_branch));
    await api("/repository/branches", "POST", { branch: e.source_branch, ref: target.commit.id });
    const actions = [];
    for (const f of e.files) {
      const existing = await remote(base + "/repository/files/" + encodeURIComponent(f.path) + "?ref=" + target.commit.id);
      if (existing.status !== 200 && existing.status !== 404) throw Error("Fixture before-file read failed");
      actions.push({ action: existing.status === 200 ? "update" : "create", file_path: f.path, content: f.content, encoding: "text", ...(existing.status === 200 ? { last_commit_id: JSON.parse(existing.data).last_commit_id } : {}) });
    }
    await api("/repository/commits", "POST", { branch: e.source_branch, force: false, commit_message: e.operation_key, actions });
    await api("/merge_requests", "POST", { source_branch: e.source_branch, target_branch: e.base_branch, title: e.title, description: e.body });
    break;
  }
  case "review_sync": await api(github ? `/issues/${e.review_number}/comments` : `/merge_requests/${e.review_number}/notes`, "POST", { body: e.audit_note.body }); break;
  case "merge": await api(github ? `/pulls/${e.review_number}/merge` : `/merge_requests/${e.review_number}/merge`, "PUT", { sha: e.approved_head }); break;
  case "verify_baseline": break;
  default: throw Error("Unsupported native stage");
}
if (e.action !== "verify_baseline") await local("observe");
appendFileSync(process.env.MATERIAL_RUNTIME_RUNS, e.action + "\n", { mode: 0o600 });
console.log(JSON.stringify({ type: "system", session_id: "material-browser-native" }));
console.log(JSON.stringify({ type: "result", subtype: "success", is_error: false, session_id: "material-browser-native", result: "Material runtime stage completed" }));
