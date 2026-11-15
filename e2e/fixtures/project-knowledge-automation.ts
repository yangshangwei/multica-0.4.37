import { spawn, type ChildProcess } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { createRequire } from "node:module";
import pg from "pg";
import { _electron as electron, expect, type Browser, type TestInfo } from "@playwright/test";
import { z } from "zod";
import { CollaborationIssueSchema, CollaborationKnowledgeCandidatesSchema, CollaborationKnowledgeDetailSchema,
  KnowledgeAutomationJobsSchema, ProjectKnowledgePolicySchema } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { deepLink, nativeScreenshot, type NativeHarness } from "./collaboration-native";
import { openJourneySession, seedDetailJourney, type DetailJourneyActor, type DetailJourneyPlatform,
  type DetailJourneySession } from "./collaboration-detail-journey";

const root = resolve(import.meta.dirname, "../..");
// Desktop is CommonJS; load its production Node helper through that module boundary.
const { prepareManagedInstallationSession } = createRequire(import.meta.url)("../../apps/desktop/src/main/managed-session.ts") as typeof import("../../apps/desktop/src/main/managed-session");
const knowledgeHeaders = { "X-Multica-Knowledge-Version": "2" };
const sha256 = (value: string) => createHash("sha256").update(value).digest("hex");
const uuid = z.uuid();
const taskSchema = z.object({ id: uuid, status: z.string(), session_id: z.string().nullable(), error_message: z.string().nullable(),
  knowledge_context_version: z.number().nullable(), knowledge_delivery: z.unknown() });
const captureSchema = z.object({ task_id: uuid, prompt: z.string(), argv: z.array(z.string()), session_id: z.string(), pid: z.number() });

export async function ownedQuery(sql: string, values: unknown[] = []) {
  const database = process.env.P6_DATABASE;
  if (!database?.startsWith("mca_knowledge_browser_") || new URL(process.env.DATABASE_URL!).pathname !== `/${database}`) {
    throw new Error("An explicit P6 browser database is required");
  }
  const client = new pg.Client({ connectionString: process.env.DATABASE_URL });
  await client.connect();
  try {
    expect((await client.query("SELECT current_database() AS name")).rows[0].name).toBe(database);
    return (await client.query(sql, values)).rows;
  } finally { await client.end(); }
}

export async function seedAutomationJourney(platform: DetailJourneyPlatform) {
  // Reuse the established provenance/authentication and manual negative fixtures.
  const baseline = await seedDetailJourney("password");
  const marker = `P6-${platform}-${randomUUID().slice(0, 8)}`;
  const project = z.object({ id: uuid, title: z.string() }).parse(await baseline.creator.client.requestJSON("/api/projects", {
    method: "POST", body: { title: `Knowledge automation ${marker}` },
  }));
  const policy = ProjectKnowledgePolicySchema.parse(await baseline.creator.client.requestJSON(`/api/projects/${project.id}/knowledge-policy`, { headers: knowledgeHeaders }));
  expect(policy).toMatchObject({ is_enabled: true, origin: "system", revision: "0", can_manage: true });
  const create = async (title: string, aiPolicy: "assisted" | "manual", projectId?: string) => {
    const requestId = randomUUID();
    const input = { source_workspace_id: baseline.workspace.id, handler_user_id: baseline.handler.id,
      title, description: "Record the verified retry rule with its exact supporting basis.", ai_policy: aiPolicy,
      attachment_ids: [], ...(projectId ? { project_id: projectId } : {}) };
    const issue = CollaborationIssueSchema.parse(await baseline.creator.client.requestJSON("/api/collaboration/issues", {
      method: "POST", headers: { "Idempotency-Key": requestId }, body: input,
    }));
    expect(issue.ai_policy).toBe(aiPolicy);
    return { issue, requestId, input };
  };
  const shared = await create(`Accepted knowledge ${marker}`, "assisted", project.id);
  const noProject = await create(`No project ${marker}`, "assisted");
  const manual = await create(`Manual project ${marker}`, "manual", project.id);
  return { ...baseline, shared, marker, project, noProject, manual, policy,
    result: `For ${marker}, retry a failed import once using its original task identity; retain the second failure for review.`,
    basis: `Controlled verification ${marker}: both attempts retained the same task UUID and the second failure remained available.` };
}
export type AutomationFixture = Awaited<ReturnType<typeof seedAutomationJourney>>;

export async function openAutomationSession(browser: Browser, fixture: AutomationFixture, platform: DetailJourneyPlatform,
  actor: DetailJourneyActor): Promise<DetailJourneySession> {
  if (platform === "web") return openJourneySession(browser, fixture, platform, actor);
  const profile = join(process.env.P6_RUN_DIRECTORY!, `electron-${randomUUID()}`);
  mkdirSync(profile, { mode: 0o700 });
  const env = Object.fromEntries(Object.entries(process.env).filter((entry): entry is [string, string] => entry[1] !== undefined));
  delete env.ELECTRON_RUN_AS_NODE;
  const initialPath = accountPath({ view: "participating", issueId: fixture.shared.issue.id });
  const app = await electron.launch({ executablePath: join(root, "apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron"),
    args: [join(root, "e2e/fixtures/project-knowledge-electron.mjs")], env: { ...env,
      COLLABORATION_ELECTRON_PROFILE: profile, COLLABORATION_ELECTRON_DEEP_LINK: `multica://account${initialPath.slice(8)}` }, timeout: 30_000 });
  try {
    const page = await app.firstWindow();
    const errors: string[] = [];
    const accountHeaders: Record<string, string>[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("request", (request) => { if (/\/api\/(collaboration\/|me\/workbench)/.test(request.url())) accountHeaders.push(request.headers()); });
    await app.context().addInitScript(({ token }) => {
      localStorage.setItem("multica_token", token);
      localStorage.setItem("multica-locale", "en");
      localStorage.setItem("multica:chat:isOpen", "false");
    }, { token: actor.client.getToken()! });
    const start = () => app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.start());
    const native = { app, page, errors, accountHeaders, start };
    const paths = await app.evaluate(({ app: target }) => ({ home: target.getPath("home"), temp: target.getPath("temp"), userData: target.getPath("userData"), version: process.versions.electron }));
    expect(paths).toMatchObject({ home: process.env.HOME, temp: process.env.TMPDIR, userData: profile });
    writeFileSync(join(profile, "native-paths.json"), JSON.stringify(paths, null, 2), { mode: 0o600 });
    await start();
    return { page, platform, locale: "en", native, errors,
      openAccount: (id) => deepLink(app, accountPath({ view: "participating", issueId: id })),
      openSource: async () => { throw new Error("This bounded fixture uses the production account detail entry point"); },
      close: () => app.close() };
  } catch (error) { await app.close(); throw error; }
}

export async function modelControl(action: "hold" | "release", marker: string) {
  const response = await fetch(`${process.env.P6_MODEL_URL}/control/${action}`, { method: "POST", body: JSON.stringify({ marker }) });
  expect(response.status).toBe(200);
}
export async function modelCalls() {
  const response = await fetch(`${process.env.P6_MODEL_URL}/control/state`);
  expect(response.status).toBe(200);
  return z.object({ calls: z.array(z.object({ number: z.number(), marker: z.string(), stage: z.string(), completed_at: z.string().optional() }).passthrough()) }).parse(await response.json()).calls;
}
export async function knowledgeJobs(fixture: AutomationFixture, id = fixture.shared.issue.id) {
  return KnowledgeAutomationJobsSchema.parse(await fixture.creator.client.requestJSON(`/api/collaboration/issues/${id}/knowledge-jobs`, { headers: knowledgeHeaders }));
}
export async function knowledgeCandidate(fixture: AutomationFixture) {
  const list = CollaborationKnowledgeCandidatesSchema.parse(await fixture.creator.client.requestJSON(
    `/api/collaboration/issues/${fixture.shared.issue.id}/knowledge-candidates`, { headers: knowledgeHeaders }));
  expect(list.items).toHaveLength(1);
  const detail = CollaborationKnowledgeDetailSchema.parse(await fixture.creator.client.requestJSON(
    `/api/collaboration/knowledge-candidates/${list.items[0]!.candidate_id}`, { headers: knowledgeHeaders }));
  if (detail.candidate.availability !== "available") throw new Error("The authorized fixture requires readable candidate content");
  return detail.candidate;
}

export async function startProductionDaemon(fixture: AutomationFixture) {
  if (fixture.provenance.auth_mode !== "password") throw new Error("Private knowledge requires the real managed daemon fixture");
  const directory = process.env.P6_RUN_DIRECTORY!;
  const profile = `p6-${fixture.marker.slice(3)}`;
  const homeDirectory = join(directory, "daemon-homes", randomUUID());
  mkdirSync(homeDirectory, { recursive: true, mode: 0o700 });
  const pat = z.object({ token: z.string().startsWith("mul_") }).parse(await fixture.creator.client.requestJSON("/api/tokens", {
    method: "POST", body: { name: `P6 managed consumer ${fixture.marker}`, expires_in_days: 1 },
  }));
  const managed = await prepareManagedInstallationSession({ homeDirectory, profile, apiBaseUrl: fixture.provenance.api_url,
    userToken: fixture.creator.client.getToken()!, daemonToken: pat.token, userId: fixture.creator.id,
    desktopVersion: z.object({ version: z.string() }).parse(JSON.parse(readFileSync(join(root, "apps/desktop/package.json"), "utf8"))).version,
    os: process.platform === "darwin" ? "macos" : process.platform === "win32" ? "windows" : process.platform === "linux" ? "linux" : "unknown" });
  if (!managed) throw new Error("The frozen runtime did not establish a complete managed installation identity");
  let child: ChildProcess | undefined;
  try {
    const handoff = await managed.createHandoff();
    const daemonID = managed.managedDaemonId;
    const profileDir = join(homeDirectory, ".multica/profiles", profile);
    mkdirSync(profileDir, { recursive: true, mode: 0o700 });
    writeFileSync(join(profileDir, "config.json"), JSON.stringify({ token: pat.token,
      server_url: fixture.provenance.api_url, workspace_id: fixture.workspace.id,
      management_deployment_id: managed.deploymentId }), { mode: 0o600, flag: "wx" });
    const profileOwnerPath = join(profileDir, ".desktop-user-id");
    writeFileSync(profileOwnerPath, managed.userId, { mode: 0o600, flag: "wx" });
    expect(readFileSync(profileOwnerPath, "utf8")).toBe(fixture.creator.id);
    const command = ["--profile", profile, "--server-url", fixture.provenance.api_url, "daemon", "start", "--foreground",
      "--managed-handoff-stdin", "--daemon-id", daemonID, "--workspaces-root", join(directory, `workspaces-${profile}`), "--poll-interval", "500ms",
      "--heartbeat-interval", "1s", "--max-concurrent-tasks", "1", "--agent-timeout", "60s", "--no-auto-update", "--no-auto-reload"];
    const daemon = spawn(process.env.P6_BINARY!, command, { cwd: directory, env: { ...process.env, HOME: homeDirectory }, stdio: ["pipe", "pipe", "pipe"] });
    child = daemon;
    daemon.stdin!.end(handoff);
    let log = "";
    for (const stream of [daemon.stdout, daemon.stderr]) stream?.on("data", (data: Buffer) => {
      log += data.toString(); writeFileSync(join(directory, "logs", `${profile}.log`), log, { mode: 0o600 });
    });
    await expect.poll(async () => {
      if (daemon.exitCode !== null) throw new Error(`Owned managed daemon exited with ${daemon.exitCode}; private log retained`);
      return await ownedQuery("SELECT id FROM agent_runtime WHERE daemon_id=$1 AND workspace_id=$2 AND provider='claude' AND status='online'", [daemonID, fixture.workspace.id]);
    }, { timeout: 60_000, intervals: [500] }).toHaveLength(1);
    const rows = await ownedQuery("SELECT id FROM agent_runtime WHERE daemon_id=$1 AND workspace_id=$2 AND provider='claude'", [daemonID, fixture.workspace.id]);
    const runtimeId = uuid.parse(rows[0]!.id);
    const agent = z.object({ id: uuid }).parse(await fixture.creator.client.requestJSON("/api/agents", { method: "POST", body: {
      name: `Controlled consumer ${fixture.marker}`, runtime_id: runtimeId, max_concurrent_tasks: 1, model: "p6-controlled",
      custom_env: { P6_PROVIDER_CAPTURE: process.env.P6_PROVIDER_CAPTURE }, instructions: "Read the task input and report the controlled result. Do not invoke tools.",
    } }));
    const managedIdentity = { deployment_id: managed.deploymentId, installation_id: managed.installationId,
      daemon_id: daemonID, user_id: managed.userId, auth_version: managed.authVersion, profile,
      home_directory: homeDirectory, profile_owner_file: profileOwnerPath, profile_owner_sha256: sha256(managed.userId),
      handoff_sha256: sha256(handoff), handoff_transport: "stdin" };
    return { child: daemon, agentId: agent.id, runtimeId, daemonID, profile, pid: daemon.pid, command, managedIdentity,
      stop: async () => { try { await stopOwned(daemon); } finally { managed.dispose(); } } };
  } catch (error) {
    try { if (child) await stopOwned(child); } finally { managed.dispose(); }
    throw error;
  }
}
async function stopOwned(child: ChildProcess) {
  if (child.exitCode !== null) return;
  const stopped = new Promise<void>((resolve) => child.once("exit", () => resolve()));
  child.kill("SIGTERM");
  await stopped;
}

export function providerCaptures() {
  const filename = process.env.P6_PROVIDER_CAPTURE!;
  return existsSync(filename) ? readFileSync(filename, "utf8").trim().split("\n").filter(Boolean).map((line) => captureSchema.parse(JSON.parse(line))) : [];
}
export async function consumerTasks(issueId: string) {
  return (await ownedQuery("SELECT id,status,session_id,error AS error_message,knowledge_context_version,knowledge_delivery FROM agent_task_queue WHERE issue_id=$1 ORDER BY created_at,id", [issueId])).map((row) => taskSchema.parse(row));
}
export async function executeConsumer(fixture: AutomationFixture, agentId: string, suffix: string) {
  const issue = z.object({ id: uuid }).parse(await fixture.creator.client.requestJSON("/api/issues", { method: "POST", body: {
    title: `Consume accepted rule ${fixture.marker} ${suffix}`, description: "Explain the accepted project retry rule.",
    project_id: fixture.project.id, assignee_type: "agent", assignee_id: agentId, status: "in_progress",
  } }));
  await expect.poll(async () => (await consumerTasks(issue.id)).at(-1)?.status, { timeout: 90_000, intervals: [500] }).toBe("completed");
  const task = (await consumerTasks(issue.id)).at(-1)!;
  const capture = providerCaptures().find((entry) => entry.task_id === task.id);
  expect(capture).toBeDefined();
  if (!capture) throw new Error("The completed consumer task has no captured provider invocation");
  expect(capture.prompt).toContain(fixture.result);
  const delivery = z.object({ version: z.literal(2), claims: z.array(z.object({ prepared: z.object({ manifest: z.object({ protocol_version: z.literal(2),
    entries: z.array(z.object({ candidate_id: uuid, version: z.string(), content_sha256: z.string(), source_issue_id: uuid,
      validity: z.object({ revision: z.string(), accepted_event_seq: z.string(), submission_id: uuid, validated_close_epoch: z.string() }) })) }),
    manifest_sha256: z.string() }), attempts: z.array(z.object({ started: z.boolean(), bound_session_id: z.string(), refused: z.boolean(), observations: z.array(z.unknown()) })) })) }).parse(task.knowledge_delivery);
  const claim = delivery.claims.at(-1)!;
  expect(claim.prepared.manifest.entries).toHaveLength(1);
  const entry = claim.prepared.manifest.entries[0]!;
  expect(entry).toMatchObject({ source_issue_id: fixture.shared.issue.id, content_sha256: sha256(fixture.result) });
  expect(claim.attempts.some((attempt) => attempt.started && !attempt.refused && attempt.bound_session_id === capture.session_id)).toBe(true);
  expect(task.session_id).toBe(capture.session_id);
  return { issueId: issue.id, task, capture, entry, manifestSHA256: claim.prepared.manifest_sha256 };
}
export async function refuseOldContinuation(fixture: AutomationFixture, issueId: string) {
  const beforeTasks = await consumerTasks(issueId);
  const beforeCaptures = providerCaptures().length;
  const response = await fixture.creator.client.requestJSON(`/api/issues/${issueId}/comments`, { method: "POST", body: {
    content: `Continue the previous rule verification ${randomUUID()}.`,
  } });
  await expect.poll(async () => (await consumerTasks(issueId)).length, { timeout: 30_000 }).toBeGreaterThan(beforeTasks.length);
  await expect.poll(async () => (await consumerTasks(issueId)).at(-1)?.status, { timeout: 60_000, intervals: [500] }).toBe("failed");
  const task = (await consumerTasks(issueId)).at(-1)!;
  expect(task.error_message).toContain("knowledge_context_changed");
  expect(providerCaptures()).toHaveLength(beforeCaptures);
  return { task, providerInvocationsBefore: beforeCaptures, providerInvocationsAfter: providerCaptures().length, response };
}

export async function captureAutomation(session: DetailJourneySession, info: TestInfo, stage: string, regionName = "Project knowledge") {
  const path = info.outputPath(`${session.platform}-${stage}.png`);
  const region = session.page.getByRole("region", { name: regionName, exact: true });
  await region.scrollIntoViewIfNeeded();
  if (session.native) await nativeScreenshot(session.native.app, session.page, path);
  else await session.page.screenshot({ path, fullPage: false });
  await info.attach(stage, { path, contentType: "image/png" });
  return { stage, path, region: regionName, text: await region.innerText() };
}
