import { createHash, timingSafeEqual } from "node:crypto";
import { readFileSync } from "node:fs";
import { createServer } from "node:https";
import { pathToFileURL } from "node:url";

const digest = (value) => createHash("sha1").update(value).digest("hex");
const sha256 = (value) => createHash("sha256").update(value).digest("hex");
const isSHA = (value) => typeof value === "string" && /^[a-f0-9]{40}$/.test(value);
const safePath = (value) => typeof value === "string" && value.length > 0 && !/[\x00-\x1f\\]/.test(value) && value.split("/").every((part) => part && part !== "." && part !== ".." && part !== ".git");
const equalSecret = (actual, expected) => {
  const a = Buffer.from(actual ?? "");
  const b = Buffer.from(expected);
  return a.length === b.length && timingSafeEqual(a, b);
};

// This fixture implements only the provider contract exercised by materials.
// It never executes Git or connects to an external provider or user account.
export function createMaterialGitLab({
  key,
  cert,
  token,
  controlToken,
  repository = "acme/materials",
  baseBranch = "main",
  initialFiles = { "README.md": "# Controlled material repository\n" },
}) {
  if (!key || !cert || !token || !controlToken || token === controlToken) throw new Error("TLS and distinct fixture credentials are required");
  if (!safePath(repository) || !repository.includes("/") || !safePath(baseBranch)) throw new Error("Invalid fixture repository");
  let origin = "";
  let sequence = 0;
  const commits = new Map();
  const branches = new Map();
  const reviews = new Map();
  const faults = [];
  const effects = { branch: 0, commit: 0, review: 0, note: 0, merge: 0, external_edit: 0 };
  const requests = new Map();

  function commit(files, parents, message) {
    const id = digest(JSON.stringify({ sequence: ++sequence, parents, message, files: [...files].sort(([a], [b]) => a.localeCompare(b)) }));
    const snapshot = new Map([...files].map(([name, value]) => [name, { ...value, last_commit_id: value.last_commit_id ?? id }]));
    const value = { id, message, parent_ids: parents, files: snapshot };
    commits.set(id, value);
    return value;
  }
  const first = commit(new Map(Object.entries(initialFiles).map(([name, content]) => {
    if (!safePath(name) || typeof content !== "string") throw new Error("Invalid initial file");
    return [name, { content, mode: "100644" }];
  })), [], "Initialize the controlled material fixture");
  branches.set(baseBranch, first.id);

  const resolve = (ref) => commits.get(branches.get(ref) ?? ref);
  const commitJSON = (value) => ({ id: value.id, message: value.message, parent_ids: value.parent_ids });
  const branchJSON = (name) => ({ name, protected: name === baseBranch, commit: commitJSON(resolve(name)) });
  const fileBlob = (file) => digest(`blob ${Buffer.byteLength(file.content)}\0${file.content}`);
  const changes = (mr) => {
    const before = commits.get(mr.base_sha);
    const after = resolve(mr.source_branch);
    if (!before || !after) throw Object.assign(new Error("Missing fixture source"), { status: 409 });
    return [...new Set([...before.files.keys(), ...after.files.keys()])].sort().flatMap((name) => {
      const oldFile = before.files.get(name);
      const newFile = after.files.get(name);
      if (oldFile?.content === newFile?.content && oldFile?.mode === newFile?.mode) return [];
      return [{ old_path: name, new_path: name, a_mode: oldFile?.mode ?? "0", b_mode: newFile?.mode ?? "0", new_file: !oldFile, renamed_file: false, deleted_file: !newFile, too_large: false, collapsed: false }];
    });
  };
  function reviewJSON(mr) {
    const head = branches.get(mr.source_branch);
    return {
      iid: mr.iid,
      project_id: 1,
      source_project_id: 1,
      target_project_id: 1,
      title: mr.title,
      description: mr.description,
      web_url: `${origin}/${repository}/-/merge_requests/${mr.iid}`,
      state: mr.state,
      sha: head,
      source_branch: mr.source_branch,
      target_branch: mr.target_branch,
      draft: false,
      work_in_progress: false,
      detailed_merge_status: mr.gate,
      has_conflicts: mr.gate === "conflict",
      changes_count: String(changes(mr).length),
      diff_refs: { head_sha: head, base_sha: mr.base_sha, start_sha: mr.base_sha },
      user: { can_merge: mr.can_merge },
      head_pipeline: mr.pipeline_status === null ? null : { sha: head, status: mr.pipeline_status },
      merge_commit_sha: mr.merge_sha ?? null,
      merged_at: mr.merged_at ?? null,
      squash_commit_sha: mr.merge_sha ?? null,
      merge_user: mr.merge_sha ? { username: "multica-material-fixture" } : null,
    };
  }
  function snapshot(includeContent = false) {
    return {
      repository,
      base_branch: baseBranch,
      initial_commit: first.id,
      branches: Object.fromEntries(branches),
      effects: { ...effects },
      requests: Object.fromEntries(requests),
      reviews: [...reviews.values()].map((mr) => ({
        ...reviewJSON(mr),
        notes: mr.notes.map((note) => ({ id: note.id, body_sha256: sha256(note.body), ...(includeContent ? { body: note.body } : {}) })),
      })),
      commits: [...commits.values()].map((value) => ({
        ...commitJSON(value),
        files: Object.fromEntries([...value.files].map(([name, file]) => [name, {
          mode: file.mode,
          bytes: Buffer.byteLength(file.content),
          content_sha256: sha256(file.content),
          last_commit_id: file.last_commit_id,
          ...(includeContent ? { content: file.content } : {}),
        }])),
      })),
    };
  }
  function json(res, status, body) {
    res.writeHead(status, { "content-type": "application/json", "cache-control": "no-store" });
    res.end(JSON.stringify(body));
  }
  function fail(res, status = 400, message = "Fixture request rejected") { json(res, status, { message }); }
  async function body(req) {
    let bytes = 0;
    const chunks = [];
    for await (const chunk of req) {
      bytes += chunk.length;
      if (bytes > 4 * 1024 * 1024) throw Object.assign(new Error("Fixture request too large"), { status: 413 });
      chunks.push(chunk);
    }
    try { return JSON.parse(Buffer.concat(chunks).toString("utf8")); } catch { throw Object.assign(new Error("Invalid fixture JSON"), { status: 400 }); }
  }
  function page(res, values, params) {
    const number = Number(params.get("page") ?? 1);
    const count = Number(params.get("per_page") ?? 20);
    if (!Number.isInteger(number) || number < 1 || !Number.isInteger(count) || count < 1 || count > 100) { fail(res); return; }
    const start = (number - 1) * count;
    res.setHeader("X-Next-Page", start + count < values.length ? String(number + 1) : "");
    res.setHeader("X-Total", String(values.length));
    json(res, 200, values.slice(start, start + count));
  }
  function fault(res, phase) {
    const index = faults.findIndex((entry) => entry.phase === phase);
    if (index < 0) return false;
    const [entry] = faults.splice(index, 1);
    if (entry.mode === "disconnect") res.destroy();
    else fail(res, entry.status ?? 503, "Injected response uncertainty");
    return true;
  }

  const server = createServer({ key, cert }, async (req, res) => {
    try {
      const url = new URL(req.url, origin || "https://localhost");
      const pathname = url.pathname;
      const method = req.method;
      if (pathname === "/health") { json(res, 200, { fixture: "material-gitlab", repository }); return; }
      if (pathname.startsWith("/__control")) {
        if (!equalSecret(req.headers.authorization, `Bearer ${controlToken}`)) { fail(res, 401); return; }
        if (method === "GET" && pathname === "/__control/snapshot") { json(res, 200, snapshot(url.searchParams.get("content") === "1")); return; }
        if (method !== "POST" || pathname !== "/__control") { fail(res, 404); return; }
        const input = await body(req);
        if (input.action === "fail_next" && ["branch", "commit", "review", "note", "merge"].includes(input.phase)) {
          if (!["disconnect", "http_error"].includes(input.mode) || (input.status != null && (![429, 500, 502, 503, 504].includes(input.status)))) { fail(res); return; }
          faults.push({ phase: input.phase, mode: input.mode, status: input.status });
        } else if (input.action === "set_gate") {
          const mr = reviews.get(Number(input.number));
          if (!mr) { fail(res, 404); return; }
          if (input.status != null && typeof input.status !== "string") { fail(res); return; }
          if (input.can_merge != null && typeof input.can_merge !== "boolean") { fail(res); return; }
          if (input.pipeline_status !== undefined && input.pipeline_status !== null && typeof input.pipeline_status !== "string") { fail(res); return; }
          mr.gate = input.status ?? mr.gate;
          mr.can_merge = input.can_merge ?? mr.can_merge;
          if (input.pipeline_status !== undefined) mr.pipeline_status = input.pipeline_status;
        } else if (input.action === "edit") {
          const parent = resolve(input.branch);
          if (!parent || !branches.has(input.branch) || !input.files || typeof input.files !== "object" || Array.isArray(input.files)) { fail(res); return; }
          const files = new Map(parent.files);
          for (const [name, content] of Object.entries(input.files)) {
            if (!safePath(name) || (content !== null && typeof content !== "string")) { fail(res); return; }
            if (content === null) files.delete(name);
            else files.set(name, { content, mode: "100644" });
          }
          const made = commit(files, [parent.id], "Controlled external material edit");
          branches.set(input.branch, made.id);
          effects.external_edit++;
        } else { fail(res); return; }
        json(res, 200, snapshot());
        return;
      }
      if (!equalSecret(req.headers["private-token"], token)) { fail(res, 401); return; }
      if (method === "GET" && pathname === "/api/v4/user") { json(res, 200, { id: 1, username: "multica-material-fixture", name: "Controlled material integration" }); return; }
      const route = pathname.match(/^\/api\/v4\/projects\/([^/]+)(.*)$/);
      if (!route || ![repository, "1"].includes(decodeURIComponent(route[1]))) { fail(res, 404); return; }
      const ep = route[2];
      // Method/route counts contain neither request bodies nor credentials.
      const requestKey = `${method} ${ep.replace(/\/repository\/files\/.+/, "/repository/files/:path")}`;
      requests.set(requestKey, (requests.get(requestKey) ?? 0) + 1);
      if (method === "GET" && ep === "") { json(res, 200, { id: 1, path_with_namespace: repository, default_branch: baseBranch, web_url: `${origin}/${repository}` }); return; }
      if (method === "GET" && ep.startsWith("/repository/branches/")) {
        const name = decodeURIComponent(ep.slice("/repository/branches/".length));
        if (!branches.has(name)) { fail(res, 404); return; }
        json(res, 200, branchJSON(name)); return;
      }
      if (method === "POST" && ep === "/repository/branches") {
        const input = await body(req);
        if (!safePath(input.branch) || input.branch === baseBranch || !isSHA(input.ref) || !commits.has(input.ref)) { fail(res); return; }
        if (branches.has(input.branch)) { fail(res, 400, "Branch already exists"); return; }
        branches.set(input.branch, input.ref); effects.branch++;
        if (!fault(res, "branch")) json(res, 201, branchJSON(input.branch)); return;
      }
      if (method === "GET" && ep === "/repository/tree") {
        const current = resolve(url.searchParams.get("ref"));
        if (!current) { fail(res, 404); return; }
        const prefix = url.searchParams.get("path") ?? "";
        if (prefix && !safePath(prefix)) { fail(res); return; }
        const entries = new Map();
        for (const [name, file] of current.files) {
          if (prefix && !name.startsWith(`${prefix}/`)) continue;
          const relative = prefix ? name.slice(prefix.length + 1) : name;
          const part = relative.split("/")[0];
          const entryPath = prefix ? `${prefix}/${part}` : part;
          const isTree = relative.includes("/");
          entries.set(entryPath, { id: isTree ? digest(current.id + entryPath) : fileBlob(file), name: part, type: isTree ? "tree" : "blob", path: entryPath, mode: isTree ? "040000" : file.mode });
        }
        page(res, [...entries.values()].sort((a, b) => a.path.localeCompare(b.path)), url.searchParams); return;
      }
      if (method === "GET" && ep.startsWith("/repository/files/")) {
        const name = decodeURIComponent(ep.slice("/repository/files/".length));
        const current = resolve(url.searchParams.get("ref"));
        const file = current?.files.get(name);
        if (!file) { fail(res, 404); return; }
        json(res, 200, { file_path: name, file_name: name.split("/").at(-1), size: Buffer.byteLength(file.content), encoding: "base64", content: Buffer.from(file.content).toString("base64"), content_sha256: sha256(file.content), blob_id: fileBlob(file), commit_id: current.id, last_commit_id: file.last_commit_id, execute_filemode: file.mode === "100755" }); return;
      }
      if (method === "POST" && ep === "/repository/commits") {
        const input = await body(req);
        const parent = resolve(input.branch);
        if (!parent || !branches.has(input.branch) || input.branch === baseBranch || input.force !== false || input.start_sha !== undefined || !Array.isArray(input.actions) || input.actions.length < 1 || input.actions.length > 3 || typeof input.commit_message !== "string") { fail(res); return; }
        const files = new Map(parent.files);
        const names = new Set();
        for (const action of input.actions) {
          if (!safePath(action.file_path) || !/requirements\/[^/]+\/(prd|design|plan)\.md$/.test(action.file_path) || names.has(action.file_path) || typeof action.content !== "string" || action.encoding !== "text") { fail(res); return; }
          names.add(action.file_path);
          const existing = files.get(action.file_path);
          if (action.action === "create" ? !!existing : action.action !== "update" || !existing || action.last_commit_id !== existing.last_commit_id) { fail(res, 409); return; }
          files.set(action.file_path, { content: action.content, mode: "100644" });
        }
        const made = commit(files, [parent.id], input.commit_message);
        branches.set(input.branch, made.id); effects.commit++;
        if (!fault(res, "commit")) json(res, 201, commitJSON(made)); return;
      }
      if (method === "GET" && ep === "/merge_requests") {
        page(res, [...reviews.values()].filter((mr) => (!url.searchParams.has("source_branch") || mr.source_branch === url.searchParams.get("source_branch")) && (!url.searchParams.has("target_branch") || mr.target_branch === url.searchParams.get("target_branch")) && (url.searchParams.get("state") === "all" || mr.state === (url.searchParams.get("state") ?? "opened"))).map(reviewJSON), url.searchParams); return;
      }
      if (method === "POST" && ep === "/merge_requests") {
        const input = await body(req);
        if (!branches.has(input.source_branch) || !branches.has(input.target_branch) || input.source_branch === input.target_branch || typeof input.title !== "string" || typeof input.description !== "string") { fail(res); return; }
        if ([...reviews.values()].some((mr) => mr.source_branch === input.source_branch && mr.target_branch === input.target_branch)) { fail(res, 409); return; }
        const source = resolve(input.source_branch);
        const mr = { iid: reviews.size + 1, source_branch: input.source_branch, target_branch: input.target_branch, base_sha: source.parent_ids[0], title: input.title, description: input.description, state: "opened", gate: "mergeable", can_merge: true, pipeline_status: "success", notes: [] };
        reviews.set(mr.iid, mr); effects.review++;
        if (!fault(res, "review")) json(res, 201, reviewJSON(mr)); return;
      }
      const mrRoute = ep.match(/^\/merge_requests\/(\d+)(.*)$/);
      if (mrRoute) {
        const mr = reviews.get(Number(mrRoute[1]));
        const suffix = mrRoute[2];
        if (!mr) { fail(res, 404); return; }
        if (method === "GET" && suffix === "") { json(res, 200, reviewJSON(mr)); return; }
        if (method === "GET" && suffix === "/diffs") { page(res, changes(mr), url.searchParams); return; }
        if (suffix === "/notes") {
          if (method === "GET") { page(res, mr.notes, url.searchParams); return; }
          if (method === "POST") {
            const input = await body(req);
            if (typeof input.body !== "string") { fail(res); return; }
            const note = { id: mr.notes.length + 1, body: input.body, author: { id: 1, username: "multica-material-fixture" }, system: false };
            mr.notes.push(note); effects.note++;
            if (!fault(res, "note")) json(res, 201, note); return;
          }
        }
        if (method === "PUT" && suffix === "/merge") {
          const input = await body(req);
          if (input.sha !== branches.get(mr.source_branch)) { fail(res, 409); return; }
          if (mr.state !== "opened" || mr.gate !== "mergeable" || !mr.can_merge || (mr.pipeline_status !== null && mr.pipeline_status !== "success")) { fail(res, 405); return; }
          const target = resolve(mr.target_branch);
          const source = resolve(mr.source_branch);
          const files = new Map(target.files);
          for (const change of changes(mr)) {
            if (change.deleted_file) files.delete(change.new_path);
            else files.set(change.new_path, { ...source.files.get(change.new_path), last_commit_id: undefined });
          }
          const made = commit(files, [target.id], "Merge the approved material candidate");
          branches.set(mr.target_branch, made.id); mr.state = "merged"; mr.merge_sha = made.id; mr.merged_at = new Date().toISOString(); effects.merge++;
          if (!fault(res, "merge")) json(res, 200, reviewJSON(mr)); return;
        }
      }
      fail(res, 404);
    } catch (error) {
      if (!res.headersSent && !res.destroyed) fail(res, Number.isInteger(error?.status) ? error.status : 500);
      else res.destroy();
    }
  });

  return {
    server,
    snapshot,
    async listen(port = 0, host = "127.0.0.1") {
      await new Promise((resolve, reject) => { server.once("error", reject); server.listen(port, host, resolve); });
      const address = server.address();
      origin = `https://${host.includes(":") ? `[${host}]` : host}:${address.port}`;
      return { origin, repository, initial_commit: first.id, base_branch: baseBranch, pid: process.pid };
    },
    async close() { await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve())); },
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const fixture = createMaterialGitLab({
      key: readFileSync(process.env.MATERIAL_GITLAB_KEY_FILE),
      cert: readFileSync(process.env.MATERIAL_GITLAB_CERT_FILE),
      token: process.env.MATERIAL_GITLAB_TOKEN,
      controlToken: process.env.MATERIAL_GITLAB_CONTROL_TOKEN,
      repository: process.env.MATERIAL_GITLAB_REPOSITORY ?? "acme/materials",
      baseBranch: process.env.MATERIAL_GITLAB_BASE_BRANCH ?? "main",
    });
    const ready = await fixture.listen(Number(process.env.MATERIAL_GITLAB_PORT ?? 0), process.env.MATERIAL_GITLAB_HOST ?? "127.0.0.1");
    process.stdout.write(`${JSON.stringify({ event: "material_gitlab_ready", ...ready })}\n`);
    for (const signal of ["SIGINT", "SIGTERM"]) process.once(signal, async () => { await fixture.close(); process.exit(0); });
  } catch {
    process.stderr.write("Material GitLab fixture could not start; check private configuration.\n");
    process.exitCode = 1;
  }
}
