import { createHash, randomUUID } from "node:crypto";
import { mkdtempSync, openSync, writeFileSync, existsSync, readFileSync } from "node:fs";
import { get } from "node:https";
import { dirname, join, posix } from "node:path";
import { expect, type Page, type Route, type TestInfo } from "@playwright/test";
import pg from "pg";
import type { MaterialDetail } from "@multica/core/materials";
import { p1Capture, p1Project, p1Session } from "./fixtures/project-p1";
import { openNativeProject, test } from "./fixtures/project-p1-desktop";
import { spawn } from "node:child_process";
import { selectDesktopWorkspace } from "./fixtures/collaboration-native";

test.skip(!process.env.MATERIAL_RUN_STAMP, "Requires the identity-checked material-run.mjs fixture");
test.setTimeout(120_000);

test.beforeAll(async () => {
  const record = JSON.parse(readFileSync(process.env.MATERIAL_UI_RECORD!, "utf8"));
  const expected = process.env.MATERIAL_RUN_DB;
  if (!expected || !/^multica_gitmaterial_ui_[a-z0-9_]+$/.test(expected) ||
      record.database !== expected || process.env.DATABASE_URL !== record.connection ||
      new URL(record.connection).pathname !== "/" + expected || !["127.0.0.1", "localhost"].includes(new URL(record.connection).hostname)) {
    throw new Error("Material browser allocation mismatch");
  }
  const client = new pg.Client({ connectionString: record.connection });
  await client.connect();
  try { expect((await client.query("SELECT current_database() AS db")).rows[0].db).toBe(expected); }
  finally { await client.end(); }
  for (const [base, port] of [[process.env.NEXT_PUBLIC_API_URL, record.ports.api], [process.env.PLAYWRIGHT_BASE_URL, record.ports.web]]) {
    expect(base).toBe("http://127.0.0.1:" + port);
    expect((await (await fetch(base + "/health")).json()).commit).toBe(process.env.MATERIAL_RUN_STAMP);
  }
});

type Snapshot = {
  initial_commit: string;
  repository: string;
  reviews: { iid: number; sha: string; merge_commit_sha: string | null; notes: unknown[] }[];
  commits: { id: string; files: Record<string, { content_sha256: string }> }[];
  effects: { review: number; merge: number; commit: number; note: number };
};

async function journey(page: Page, session: Awaited<ReturnType<typeof p1Session>>, info: TestInfo, native: boolean) {
  const { api, workspace, owner } = session;
  const record = JSON.parse(readFileSync(process.env.MATERIAL_UI_RECORD!, "utf8"));
  const gitOrigin = "https://127.0.0.1:" + record.ports.gitlab;
  const provider = record.provider ?? "gitlab";
  const snapshot = () => new Promise<Snapshot>((resolve, reject) => {
    get(gitOrigin + "/__control/snapshot", { ca: readFileSync(record.env.SSL_CERT_FILE), headers: { authorization: "Bearer " + record.control_token } }, (response) => {
      let data = "";
      response.setEncoding("utf8");
      response.on("data", (chunk) => { data += chunk; });
      response.on("end", () => {
        if (response.statusCode !== 200) return reject(Error("Fixture snapshot failed: " + response.statusCode));
        try { resolve(JSON.parse(data)); } catch (error) { reject(error); }
      });
    }).on("error", reject);
  });
  const before = await snapshot();
  const project = await p1Project(api, { title: "Material acceptance project" });
  const runtime = await api.seedProjectRuntime();
  const runRoot = mkdtempSync(join(process.env.MATERIAL_RUNTIME_DIR!, "runtime-"));
  const token = "mdt_" + randomUUID();
  const database = new pg.Client({ connectionString: record.connection });
  await database.connect();
  try {
    await database.query("UPDATE agent_runtime SET provider='claude',name='Material fixture runtime',metadata=$2 WHERE id=$1", [runtime.id, JSON.stringify({ capabilities: ["material-operations-v1"] })]);
    await database.query("INSERT INTO daemon_token(token_hash,workspace_id,daemon_id,user_id,expires_at) VALUES($1,$2,$3,$4,now()+interval '1 hour')", [createHash("sha256").update(token).digest("hex"), workspace.id, runtime.daemon_id, owner.id]);
  } finally { await database.end(); }
  const registration = join(runRoot, "registration.json");
  writeFileSync(registration, JSON.stringify({ API: process.env.NEXT_PUBLIC_API_URL, Token: token, Runtime: runtime.id, Daemon: runtime.daemon_id, Executable: process.env.MATERIAL_RUNTIME_EXECUTABLE, Root: runRoot }), { mode: 0o600 });
  const log = openSync(join(runRoot, "daemon.log"), "w", 0o600);
  const daemon = spawn(process.env.MATERIAL_DAEMON_BINARY!, ["-test.run", "^TestMaterialBrowserDaemon$", "-test.v"], { env: { ...process.env, PATH: dirname(process.env.MATERIAL_RUNTIME_EXECUTABLE!) + ":" + process.env.PATH, MATERIAL_BROWSER_REGISTRATION: registration, MATERIAL_RUNTIME_RUNS: join(runRoot, "runs") }, stdio: ["ignore", log, log] });
  try {
  await expect.poll(() => existsSync(join(runRoot, "daemon-ready"))).toBe(true);

  await api.requestJSON("/api/projects/" + project.id + "/resources", { method: "POST", body: { resource_type: "github_repo", label: "Fixture repository", resource_ref: { url: (provider === "github" ? "https://github.com" : gitOrigin) + "/" + before.repository + ".git" } } });
  if (native) {
    // Runtime registration is fixture setup outside the app; discard the shell
    // inventory fetched before that registration, then read the real API again.
    await page.reload();
    await openNativeProject(page, project.title);
  }
  else await page.goto("/" + workspace.slug + "/projects/" + project.id);
  const button = (name: string) => page.getByRole("button", { name, exact: true });
  const choose = async (field: string, option: string) => {
    await page.getByLabel(field, { exact: true }).click();
    await page.getByRole("option", { name: option, exact: true }).click();
  };
  await button("Engineering materials").click();
  await button("Configure workflow").click();
  await choose("Git platform", provider === "github" ? "GitHub" : "GitLab");
  await choose("Material repository resource", "Fixture repository");
  await page.getByRole("button", { name: "Runtime · none selected", exact: true }).click();
  await page.getByRole("button", { name: /^Material fixture runtime/ }).click();
  await page.getByRole("checkbox", { name: "P1 project owner" }).check();

  const auto = page.getByRole("switch", { name: "Start planning after verified merge" });
  await expect(auto).toBeChecked();
  await auto.click();
  await page.getByRole("group", { name: "Execution runtime", exact: true }).scrollIntoViewIfNeeded();
  await p1Capture(page, info, "runtime-only-settings");
  await button("Save").click();
  await button("New requirement").click();
  const title = "Auditable customer requirement";
  await page.getByLabel("Requirement title").fill(title);
  const created = page.waitForResponse((res) => res.request().method() === "POST" && new URL(res.url()).pathname.replace(/\/$/, "").endsWith("/materials") && res.ok());
  await button("Create draft").click();
  const material = (await (await created).json() as MaterialDetail).material;
  const detail = () => api.requestJSON<MaterialDetail>("/api/projects/" + project.id + "/materials/" + material.id);
  const prd = "Keep the customer's approved requirement available for audit.\n\nPlanning must wait for a human to explicitly start the work.";
  const design = "Read the immutable approved document at its merge commit before deriving the technical design.";
  const tasks = "Task 1: verify the pinned document digest.\n\nTask 2: present the proposed design for human approval.";
  await button("New revision").click();
  await page.getByLabel("Requirement document", { exact: true }).fill(prd);
  await page.getByLabel("Acceptance criteria", { exact: true }).fill("AC1: Planning starts only after explicit authorization");
  await button("Add source").click();
  await choose("Source repository", "Fixture repository");
  await page.getByLabel("Full commit SHA").fill(before.initial_commit);
  await page.getByLabel("Source file path").fill("README.md");
  const ready = page.getByRole("checkbox", { name: /Clarification is complete/ });
  await ready.check();
  await button("Save").click();
  const approve = async (reason: string, checkReadRecovery = false) => {
    const acknowledge = button("Review the displayed version");
    await expect(acknowledge.or(button("Approve this version"))).toBeVisible({ timeout: 30_000 });
    if (await acknowledge.isVisible()) await acknowledge.click();
    await expect(button("Approve this version")).toBeEnabled({ timeout: 30_000 });
    await page.getByLabel("Review reason").fill(reason);
    if (checkReadRecovery) {
      const reviewInput = await page.getByLabel("Review reason").elementHandle();
      const detailPath = "/api/projects/" + project.id + "/materials/" + material.id;
      const matchesDetail = (url: URL) => url.pathname === detailPath;
      const alert = page.getByRole("alert").filter({ hasText: "Could not load materials. Refresh to try again." });
      let releaseFailure!: () => void;
      const readyToFail = new Promise<void>((resolve) => { releaseFailure = resolve; });
      let intercepted = 0;
      let fulfilled = 0;
      const unavailable = async (route: Route) => {
        if (route.request().method() !== "GET") return route.continue();
        intercepted += 1;
        await readyToFail;
        await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "Temporary fixture outage" }) });
        fulfilled += 1;
      };
      await page.route(matchesDetail, unavailable);
      try {
        // Exercise the real read path without relying on background visibility
        // or Electron emitting Chromium's response event for a fulfilled route.
        // This shared title row contains the refresh button in both shells.
        // A header nested in Web's main does not expose the banner role.
        const materialTitleRow = page.getByRole("heading", { name: title, exact: true }).locator("..");
        await materialTitleRow.getByRole("button", { name: "Refresh", exact: true }).click();
        await expect.poll(() => intercepted).toBeGreaterThan(0);
        await page.getByLabel("Review reason").focus();
        releaseFailure();
        await expect.poll(() => fulfilled).toBeGreaterThan(0);
        await expect(alert).toBeVisible();
        await expect(page.getByLabel("Review reason")).toHaveValue(reason);
        await expect(page.getByLabel("Review reason")).toBeFocused();
        expect(await reviewInput!.evaluate((input) => input.isConnected)).toBe(true);
        await p1Capture(page, info, "review-preserved-through-read-failure");
      } finally {
        releaseFailure();
        // Drain handlers released above before a failure tears down the page.
        await page.unrouteAll({ behavior: "wait" });
      }
      await alert.getByRole("button", { name: "Refresh", exact: true }).click();
      await expect(alert).toHaveCount(0);
      await expect(page.getByLabel("Review reason")).toHaveValue(reason);
      expect(await reviewInput!.evaluate((input) => input.isConnected)).toBe(true);
      await info.attach("material-read-recovery", { contentType: "application/json", body: JSON.stringify({ intercepted, fulfilled, injected_status: 503, same_input_retained: true, recovery_notice_cleared: true }) });
      await reviewInput!.dispose();
    }
    await button("Approve this version").click();
  };
  await approve("Approve the exact requirement version", true);
  await expect(page.getByText(/Planning · Waiting for manual start/)).toBeVisible({ timeout: 30_000 });
  await page.reload();
  await expect(button("Start planning")).toBeVisible();
  const held = await detail();
  expect(held.plan?.status).toBe("waiting_manual");
  const issue = held.plan!.issue_id!;
  expect(await api.countIssueDispatches(issue)).toBe(0);
  const baseline = held.baseline!;
  const hash = (content: string) => createHash("sha256").update(content).digest("hex");
  expect(baseline.content_sha256).toBe(hash(prd));
  expect(held.decisions).toContainEqual(expect.objectContaining({ revision_id: baseline.revision_id, actor_id: owner.id, decision: "approve" }));
  const remote = await snapshot();
  const requirementMR = remote.reviews.find((item) => item.merge_commit_sha === baseline.commit);
  expect(requirementMR?.sha).toBe(held.decisions[0].head_sha);
  expect(remote.commits.find((commit) => commit.id === baseline.commit)?.files[baseline.path].content_sha256).toBe(hash(prd));
  expect((await api.requestJSON<{ description: string }>("/api/issues/" + issue)).description).toContain(prd);
  await p1Capture(page, info, "verified-baseline-manual-hold");
  await button("Start planning").click();
  await expect.poll(() => api.countIssueDispatches(issue), { timeout: 30_000 }).toBe(1);
  await expect.poll(() => existsSync(join(runRoot, "runs")) ? readFileSync(join(runRoot, "runs"), "utf8").split("\n").filter(x => x === "verify_baseline").length : 0, { timeout: 30_000 }).toBe(1);
  await button("Write a plan revision").click();
  await page.getByLabel("Technical design").fill(design);
  await page.getByLabel("Task breakdown").fill(tasks);
  await page.getByLabel("Acceptance criteria", { exact: true }).fill("AC1: Pin the requirement baseline in the task plan");
  await ready.check();
  await button("Save").click();
  await expect(page.getByText("Plan awaiting review", { exact: true })).toBeVisible({ timeout: 30_000 });
  expect((await detail()).revisions.find((item) => item.kind === "plan")?.source_baseline_id).toBe(baseline.id);
  await approve("Approve the design and execution plan");
  await expect(page.getByText("Plan approved", { exact: true })).toBeVisible({ timeout: 30_000 });
  const approved = await detail();
  const plan = approved.revisions.find((item) => item.kind === "plan")!;
  expect(approved.plan).toMatchObject({ status: "approved", revision_id: plan.id, issue_id: issue, baseline_id: baseline.id });
  expect(approved.decisions).toContainEqual(expect.objectContaining({ actor_id: owner.id, decision: "approve", revision_id: plan.id, head_sha: plan.head_sha }));
  expect(await api.countIssueDispatches(issue)).toBe(1);
  const final = await snapshot();
  const merge = final.reviews.find((item) => item.sha === plan.head_sha);
  expect(merge?.notes).toHaveLength(1);
  const files = final.commits.find((item) => item.id === merge?.merge_commit_sha)!.files;
  expect(files[posix.join(posix.dirname(baseline.path), "design.md")].content_sha256).toBe(hash(design));
  expect(files[posix.join(posix.dirname(baseline.path), "plan.md")].content_sha256).toBe(hash(tasks));
  for (const key of ["commit", "review", "merge", "note"] as const) expect(final.effects[key] - before.effects[key]).toBe(2);
  await p1Capture(page, info, "approved-design-and-plan");
  expect(readFileSync(join(runRoot, "runs"), "utf8").trim().split("\n")).toEqual(["publish", "review_sync", "merge", "verify_baseline", "publish", "review_sync", "merge"]);
  } finally {
    writeFileSync(join(runRoot, "stop"), "stop", { mode: 0o600 });
    await new Promise<void>((resolve) => { if (daemon.exitCode !== null) return resolve(); const timer = setTimeout(() => { daemon.kill(); resolve(); }, 2000); daemon.once("exit", () => { clearTimeout(timer); resolve(); }); });
    if (daemon.exitCode !== null && daemon.exitCode !== 0) throw Error("Material daemon failed; inspect " + join(runRoot, "daemon.log"));
  }
}

test("Web: verified requirement, manual planning and exact plan approval", async ({ page }, info) => {
  const session = await p1Session(page);
  await journey(page, session, info, false);
});

test("Desktop: verified requirement, manual planning and exact plan approval", async ({ native }, info) => {
  const { api, page } = native;
  const workspace = await api.requestJSON<typeof native.workspace>("/api/workspaces", { method: "POST", body: { name: "Material review archive", slug: "material-" + native.workspace.slug } });
  api.setWorkspaceId(workspace.id);
  api.setWorkspaceSlug(workspace.slug);
  try {
    await page.reload();
    await selectDesktopWorkspace(page, workspace);
    await journey(page, { ...native, workspace }, info, true);
  } finally {
    api.setWorkspaceId(native.workspace.id);
    api.setWorkspaceSlug(native.workspace.slug);
  }
});
