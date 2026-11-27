import { createHash, timingSafeEqual } from "node:crypto";
import { createServer } from "node:https";

const hash = (value) => createHash("sha1").update(value).digest("hex");
const sha256 = (value) => createHash("sha256").update(value).digest("hex");
const sameSecret = (a, b) => { const left = Buffer.from(a ?? ""), right = Buffer.from(b); return left.length === right.length && timingSafeEqual(left, right); };
const safePath = (value) => typeof value === "string" && value.length > 0 && !/[\x00-\x1f\\]/.test(value) && value.split("/").every(part => part && part !== "." && part !== ".." && part !== ".git");

// The native fixture uses real GitHub-shaped requests against this loopback
// store. The independent gh process can only read these resulting snapshots.
export function createMaterialGitHub({ key, cert, token, controlToken, repository = "acme/materials", baseBranch = "main" }) {
  if (!key || !cert || !token || !controlToken || token === controlToken || !safePath(repository) || repository.split("/").length !== 2) throw Error("Invalid fixture configuration");
  const branches = new Map(), commits = new Map(), trees = new Map(), blobs = new Map(), reviews = new Map();
  const effects = { branch: 0, commit: 0, review: 0, note: 0, merge: 0 };
  let sequence = 0;
  const actor = "multica-material-fixture";
  const blob = (content) => { const sha = hash(`blob ${Buffer.byteLength(content)}\0${content}`); blobs.set(sha, content); return sha; };
  function tree(files) {
    const entries = new Map();
    for (const [path, content] of files) {
      const [name, ...rest] = path.split("/");
      if (!rest.length) entries.set(name, { path: name, mode: "100644", type: "blob", sha: blob(content) });
      else if (!entries.has(name)) {
        const children = new Map([...files].filter(([p]) => p.startsWith(name + "/")).map(([p, v]) => [p.slice(name.length + 1), v]));
        entries.set(name, { path: name, mode: "040000", type: "tree", sha: tree(children) });
      }
    }
    const value = [...entries.values()].sort((a, b) => a.path.localeCompare(b.path));
    const sha = hash(JSON.stringify(value)); trees.set(sha, { files: new Map(files), entries: value }); return sha;
  }
  function commit(files, parents, message) {
    const sha = hash(JSON.stringify({ sequence: ++sequence, files: [...files], parents, message }));
    const value = { sha, tree: { sha: tree(files) }, parents: parents.map(sha => ({ sha })), message, files: new Map(files) };
    commits.set(sha, value); return value;
  }
  const initial = commit(new Map([["README.md", "# Controlled material repository\n"]]), [], "Initialize fixture"); branches.set(baseBranch, initial.sha);
  const changed = (p) => { const before = commits.get(p.base_sha).files, after = commits.get(branches.get(p.head)).files; return [...new Set([...before.keys(), ...after.keys()])].sort().filter(path => before.get(path) !== after.get(path)).map(filename => ({ filename, status: !after.has(filename) ? "removed" : before.has(filename) ? "modified" : "added" })); };
  const review = p => ({ number: p.number, html_url: `https://github.com/${repository}/pull/${p.number}`, body: p.body, state: p.merge_sha ? "closed" : "open", draft: false, merged: !!p.merge_sha, merge_commit_sha: p.merge_sha ?? "", merged_at: p.merged_at ?? null, merged_by: p.merge_sha ? { login: actor } : null, mergeable: true, mergeable_state: "clean", changed_files: changed(p).length, head: { ref: p.head, sha: branches.get(p.head), repo: { full_name: repository } }, base: { ref: p.base, sha: p.base_sha, repo: { full_name: repository } } });
  const snapshot = () => ({ repository, initial_commit: initial.sha, base_branch: baseBranch, branches: Object.fromEntries(branches), effects: { ...effects }, reviews: [...reviews.values()].map(p => ({ iid: p.number, sha: branches.get(p.head), merge_commit_sha: p.merge_sha ?? null, notes: p.notes })), commits: [...commits.values()].map(c => ({ id: c.sha, files: Object.fromEntries([...c.files].map(([path, content]) => [path, { content_sha256: sha256(content), mode: "100644" }])) })) });
  const json = (res, status, body) => { res.writeHead(status, { "content-type": "application/json" }); res.end(JSON.stringify(body)); };
  const fail = (res, status = 400) => json(res, status, { message: "Fixture request rejected" });
  async function input(req) { const chunks = []; let size = 0; for await (const chunk of req) { size += chunk.length; if (size > 4 << 20) throw Error("Too large"); chunks.push(chunk); } return JSON.parse(Buffer.concat(chunks).toString("utf8")); }
  const server = createServer({ key, cert }, async (req, res) => {
    try {
      const url = new URL(req.url, "https://localhost");
      if (url.pathname === "/__control/snapshot" && req.method === "GET") { if (!sameSecret(req.headers.authorization, `Bearer ${controlToken}`)) return fail(res, 401); return json(res, 200, snapshot()); }
      if (url.pathname === "/__control" && req.method === "POST") {
        if (!sameSecret(req.headers.authorization, `Bearer ${controlToken}`)) return fail(res, 401);
        const b = await input(req); if (b.action !== "seed" || !b.files || typeof b.files !== "object") return fail(res);
        const files = new Map(commits.get(branches.get(baseBranch)).files);
        for (const [path, content] of Object.entries(b.files)) { if (!safePath(path) || typeof content !== "string" || files.has(path)) return fail(res); files.set(path, content); }
        const c = commit(files, [branches.get(baseBranch)], "Seed isolated existing repository"); branches.set(baseBranch, c.sha); return json(res, 200, snapshot());
      }
      if (!sameSecret(req.headers.authorization, `Bearer ${token}`)) return fail(res, 401);
      if (url.pathname === "/graphql" && req.method === "POST") {
        const body = await input(req), p = reviews.get(body.variables?.number);
        if (!p || !body.query?.startsWith("query(") || body.query.includes("mutation") || body.variables.owner + "/" + body.variables.repo !== repository) return fail(res);
        const sha = branches.get(p.head);
        return json(res, 200, { data: { repository: { pullRequest: { headRefOid: sha, baseRefName: p.base, mergeStateStatus: "CLEAN", reviewDecision: "APPROVED", commits: { nodes: [{ commit: { oid: sha, statusCheckRollup: { state: "SUCCESS" } } }] } } } } });
      }
      const prefix = "/repos/" + repository;
      if (!url.pathname.startsWith(prefix)) return fail(res, 404);
      const ep = url.pathname.slice(prefix.length), method = req.method;
      if (method === "GET" && ep === "") return json(res, 200, { full_name: repository, default_branch: baseBranch });
      if (method === "GET" && ep.startsWith("/git/ref/heads/")) { const name = decodeURIComponent(ep.slice(15)); if (!branches.has(name)) return fail(res, 404); return json(res, 200, { ref: "refs/heads/" + name, object: { type: "commit", sha: branches.get(name) } }); }
      if (method === "POST" && ep === "/git/refs") { const b = await input(req), name = b.ref?.replace(/^refs\/heads\//, ""); if (!safePath(name) || !commits.has(b.sha) || branches.has(name)) return fail(res); branches.set(name, b.sha); effects.branch++; return json(res, 201, { ref: b.ref, object: { type: "commit", sha: b.sha } }); }
      if (method === "GET" && ep.startsWith("/git/commits/")) { const c = commits.get(ep.slice(13)); if (!c) return fail(res, 404); return json(res, 200, { sha: c.sha, tree: c.tree, parents: c.parents, message: c.message }); }
      if (method === "GET" && ep.startsWith("/git/trees/")) { const sha = ep.slice(11), t = trees.get(sha); if (!t) return fail(res, 404); const recursive = (value, prefix = "") => value.entries.flatMap(entry => { const path = prefix + entry.path; return [{ ...entry, path }, ...(entry.type === "tree" ? recursive(trees.get(entry.sha), path + "/") : [])]; }); return json(res, 200, { sha, truncated: false, tree: url.searchParams.get("recursive") === "1" ? recursive(t) : t.entries }); }
      if (method === "GET" && ep.startsWith("/git/blobs/")) { const sha = ep.slice(11), c = blobs.get(sha); if (c === undefined) return fail(res, 404); return json(res, 200, { sha, encoding: "base64", content: Buffer.from(c).toString("base64"), size: Buffer.byteLength(c) }); }
      if (method === "POST" && ep === "/git/trees") {
        const b = await input(req), original = trees.get(b.base_tree); if (!original || !Array.isArray(b.tree) || b.tree.length < 1 || b.tree.length > 20) return fail(res);
        const files = new Map(original.files);
        for (const f of b.tree) { if (!safePath(f.path) || f.mode !== "100644" || f.type !== "blob" || typeof f.content !== "string") return fail(res); files.set(f.path, f.content); }
        return json(res, 201, { sha: tree(files) });
      }
      if (method === "POST" && ep === "/git/commits") { const b = await input(req), t = trees.get(b.tree); if (!t || !Array.isArray(b.parents) || b.parents.length !== 1 || !commits.has(b.parents[0]) || typeof b.message !== "string") return fail(res); const c = commit(t.files, b.parents, b.message); effects.commit++; return json(res, 201, { sha: c.sha, tree: c.tree, parents: c.parents }); }
      if (method === "PATCH" && ep.startsWith("/git/refs/heads/")) { const name = decodeURIComponent(ep.slice(16)), b = await input(req), c = commits.get(b.sha); if (name === baseBranch || !branches.has(name) || !c || b.force !== false || c.parents[0]?.sha !== branches.get(name)) return fail(res, 409); branches.set(name, c.sha); return json(res, 200, { ref: "refs/heads/" + name, object: { type: "commit", sha: c.sha } }); }
      if (method === "GET" && ep.startsWith("/branches/")) return json(res, 200, { protected: false });
      if (method === "GET" && ep.startsWith("/rules/branches/")) return json(res, 200, []);
      if (method === "GET" && ep === "/pulls") return json(res, 200, [...reviews.values()].filter(p => !url.searchParams.has("head") || url.searchParams.get("head") === repository.split("/")[0] + ":" + p.head).map(review));
      if (method === "POST" && ep === "/pulls") { const b = await input(req); if (!branches.has(b.head) || b.base !== baseBranch || [...reviews.values()].some(p => p.head === b.head)) return fail(res); const p = { number: reviews.size + 1, head: b.head, base: b.base, base_sha: branches.get(b.base), body: b.body, notes: [] }; reviews.set(p.number, p); effects.review++; return json(res, 201, review(p)); }
      const route = ep.match(/^\/(pulls|issues)\/(\d+)(.*)$/);
      const p = route && reviews.get(Number(route[2]));
      if (!p) return fail(res, 404);
      if (method === "GET" && route[1] === "pulls" && route[3] === "") return json(res, 200, review(p));
      if (method === "GET" && route[3] === "/files") return json(res, 200, changed(p));
      if (route[1] === "issues" && route[3] === "/comments") { if (method === "GET") return json(res, 200, p.notes); if (method === "POST") { const b = await input(req); if (typeof b.body !== "string") return fail(res); const note = { id: p.notes.length + 1, body: b.body, user: { login: actor } }; p.notes.push(note); effects.note++; return json(res, 201, note); } }
      if (method === "PUT" && route[3] === "/merge") {
        const b = await input(req); if (p.merge_sha || b.sha !== branches.get(p.head)) return fail(res, 409);
        const files = new Map(commits.get(branches.get(p.base)).files), source = commits.get(b.sha).files;
        for (const change of changed(p)) { if (change.status === "removed") files.delete(change.filename); else files.set(change.filename, source.get(change.filename)); }
        const c = commit(files, [branches.get(p.base)], "Merge approved material"); branches.set(p.base, c.sha); p.merge_sha = c.sha; p.merged_at = new Date().toISOString(); effects.merge++;
        return json(res, 200, { sha: c.sha, merged: true, message: "Merged" });
      }
      return fail(res, 404);
    } catch { if (!res.headersSent) fail(res, 500); else res.destroy(); }
  });
  return { snapshot, server, async listen(port = 0) { await new Promise((ok, fail) => { server.once("error", fail); server.listen(port, "127.0.0.1", ok); }); return { origin: "https://127.0.0.1:" + server.address().port, repository, initial_commit: initial.sha }; }, async close() { await new Promise((ok, fail) => server.close(error => error ? fail(error) : ok())); } };
}
