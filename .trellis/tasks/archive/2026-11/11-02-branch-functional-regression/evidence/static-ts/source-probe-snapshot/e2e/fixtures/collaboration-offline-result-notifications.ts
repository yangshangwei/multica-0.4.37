import { createHash, randomUUID } from "node:crypto";
import { createServer, request as httpRequest } from "node:http";
import type { Socket } from "node:net";
import { expect, type Page } from "@playwright/test";
import pg from "pg";
import type { TestApiClient } from "../fixtures";
import { CollaborationIssueSchema, CollaborationOperationSchema, CollaborationStatusOptionsSchema,
  CollaborationWorkbenchSchema, type CollaborationIssue } from "../../packages/core/collaboration/schema";
import { CollaborationNotificationsSchema } from "../../packages/core/collaboration/content-schema";
import { CollaborationRunRecordsSchema } from "../../packages/core/collaboration/run-records-schema";
import { assertProviderInput, seedCrossClientExecution, startCrossClientDaemons } from "./collaboration-cross-client-execution";
import { lifecycleComment, lifecycleControl, refreshLifecyclePage } from "./collaboration-lifecycle-controls";
import { seedOfflineHandoff, type HandoffClients } from "./collaboration-offline-handoff";

const rootFor = (id: string) => `/api/collaboration/issues/${id}`;
const requiredKinds = ["information_requested", "information_returned", "acceptance_submitted", "ai_completed", "ai_failed", "ai_stopped"] as const;
const labels: Record<typeof requiredKinds[number], string> = { information_requested: "Information requested", information_returned: "Information returned",
  acceptance_submitted: "Submitted for acceptance", ai_completed: "AI execution completed", ai_failed: "AI execution failed", ai_stopped: "AI stop confirmed" };
const digest = (body: Buffer) => createHash("sha256").update(body).digest("hex");
const readIssue = async (actor: TestApiClient, issueId: string) => CollaborationIssueSchema.parse(await actor.requestJSON(rootFor(issueId)));
function observations(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Notification workflow requires current human observations");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}

async function notifications(actor: TestApiClient, issueId: string) {
  const page = CollaborationNotificationsSchema.parse(await actor.requestJSON("/api/me/collaboration-notifications?include_archived=true&limit=100"));
  expect(page.next_cursor).toBeNull();
  return page.items.filter((item) => item.issue_id === issueId);
}

export async function seedOfflineResultNotifications() {
  const fixture = await seedCrossClientExecution();
  const offline = await seedOfflineHandoff();
  const recipient = offline.recipient;
  expect(await recipient.client.getWorkspaces()).toEqual([]);
  const invitation = CollaborationOperationSchema.parse(await fixture.webActor.client.requestJSON(`${rootFor(fixture.issueId)}/participants`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...observations(await readIssue(fixture.webActor.client, fixture.issueId)), target_user_id: recipient.id,
    },
  }));
  expect(invitation.code).toBe("participant_added");
  expect((await notifications(recipient.client, fixture.issueId)).map((item) => item.kind)).toEqual(["participant_added"]);
  return { fixture, recipient, accountPath: `/account?view=participating&issue=${fixture.issueId}` };
}
export type OfflineResultFixture = Awaited<ReturnType<typeof seedOfflineResultNotifications>>;

/** Faults affect only task-owned transport. Outcome loss is an incomplete
 * business record; original proof bytes still come from the real supervisor. */
export async function notificationRuntimeTransport(apiUrl: string) {
  const origin = new URL(apiUrl);
  if (origin.protocol !== "http:" || !["127.0.0.1", "localhost"].includes(origin.hostname)) throw new Error("Only an owned loopback runtime may be forwarded");
  const held = new Set<string>(), lostOutcomes = new Set<string>();
  const originals = new Map<string, { bytes: Buffer; authorization: string; sha256: string }>();
  const attempts: { path: string; status: number; sha256: string | null }[] = [];
  const sockets = new Set<Socket>(), errors: string[] = [];
  const server = createServer(async (request, response) => {
    const path = request.url ?? "/";
    const proof = request.method === "POST" && /^\/api\/collaboration\/runtime\/runs\/[^/]+\/proof$/.test(path);
    const outcome = request.method === "POST" && /^\/api\/collaboration\/runtime\/runs\/[^/]+\/outcome$/.test(path);
    try {
      const chunks: Buffer[] = [];
      let size = 0;
      for await (const chunk of request) {
        const bytes = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
        size += bytes.length;
        if (size > 2_000_000) throw new Error("The bounded runtime envelope was exceeded");
        chunks.push(bytes);
      }
      const bytes = Buffer.concat(chunks);
      const sha256 = proof || outcome ? digest(bytes) : null;
      if (proof) {
        const previous = originals.get(path);
        if (previous && previous.sha256 !== sha256) errors.push("Original sealed proof bytes changed");
        if (!previous) originals.set(path, { bytes, authorization: request.headers.authorization ?? "", sha256: sha256! });
      }
      if (proof && held.has(path) || outcome && lostOutcomes.has(path)) {
        attempts.push({ path, status: 503, sha256 });
        response.writeHead(503, { "Content-Type": "application/json", "Cache-Control": "no-store", "Retry-After": "1" });
        response.end('{"code":"fixture_transport_unavailable"}');
        return;
      }
      await new Promise<void>((resolve, reject) => {
        const forwarded = httpRequest(new URL(path, origin), { method: request.method, headers: { ...request.headers, host: origin.host }, timeout: 15_000 }, (upstream) => {
          attempts.push({ path, status: upstream.statusCode ?? 502, sha256 });
          response.writeHead(upstream.statusCode ?? 502, upstream.headers);
          upstream.pipe(response);
          upstream.once("end", resolve);
          upstream.once("error", reject);
        });
        forwarded.once("error", reject);
        forwarded.once("timeout", () => forwarded.destroy(new Error("Runtime transport timed out")));
        forwarded.end(bytes);
      });
    } catch {
      errors.push("Owned runtime forwarding failed");
      if (!response.headersSent) response.writeHead(502, { "Content-Type": "application/json" });
      response.end('{"code":"fixture_transport_unavailable"}');
    }
  });
  server.on("connection", (socket) => { sockets.add(socket); socket.once("close", () => sockets.delete(socket)); });
  await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(0, "0.0.0.0", resolve); });
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("The owned runtime transport did not bind");
  const proofPath = (runId: string) => `/api/collaboration/runtime/runs/${runId}/proof`;
  return {
    apiUrl: `http://127.0.0.1:${address.port}`,
    holdProof(runId: string) { held.add(proofPath(runId)); },
    loseOutcome(runId: string) { lostOutcomes.add(`/api/collaboration/runtime/runs/${runId}/outcome`); },
    releaseProof(runId: string) { held.delete(proofPath(runId)); },
    async waitForProof(runId: string, status: 200 | 503) {
      await expect.poll(() => attempts.some((item) => item.path === proofPath(runId) && item.status === status), { timeout: 30_000 }).toBe(true);
      expect(errors).toEqual([]);
    },
    async replayProof(runId: string) {
      const original = originals.get(proofPath(runId));
      if (!original) throw new Error("There is no original natural proof to replay");
      const response = await fetch(new URL(proofPath(runId), origin), { method: "POST", headers: { Authorization: original.authorization,
        "Content-Type": "application/json" }, body: original.bytes.toString("utf8"), redirect: "error" });
      expect(response.status).toBe(200);
      return { status: response.status, original_sha256: original.sha256 };
    },
    evidence: () => ({ attempts: [...attempts], errors: [...errors], proof_hashes: Object.fromEntries([...originals].map(([path, value]) => [path, value.sha256])) }),
    async close() {
      for (const socket of sockets) socket.destroy();
      await new Promise<void>((resolve) => server.close(() => resolve()));
    },
  };
}

/** One concrete all-event receipt checks source-member/outsider isolation;
 * canonical service suites own its complete persistence/failure matrix. */
async function sourceIsolation(state: OfflineResultFixture) {
  const db = new pg.Client(process.env.DATABASE_URL!);
  await db.connect();
  try {
    const result = await db.query<{ creator_source_membership: number; recipient_source_membership: number; inbox_rows: number; subscriber_rows: number }>(
      `SELECT (SELECT count(*)::integer FROM member WHERE workspace_id=$1 AND user_id=$3) AS creator_source_membership,
        (SELECT count(*)::integer FROM member WHERE workspace_id=$1 AND user_id=$4) AS recipient_source_membership,
        (SELECT count(*)::integer FROM inbox_item WHERE issue_id=$2) AS inbox_rows,
        (SELECT count(*)::integer FROM issue_subscriber WHERE issue_id=$2) AS subscriber_rows`,
      [state.fixture.sourceWorkspace.id, state.fixture.issueId, state.fixture.webActor.id, state.recipient.id],
    );
    expect(result.rows).toEqual([{ creator_source_membership: 1, recipient_source_membership: 0, inbox_rows: 0, subscriber_rows: 0 }]);
    return result.rows[0]!;
  } finally { await db.end(); }
}

export async function publishOfflineWorkflow(state: OfflineResultFixture) {
  const { fixture, recipient } = state;
  const root = rootFor(fixture.issueId);
  const events: { kind: string; actor_id: string; request_id: string; notification_id: string; event_seq: string }[] = [];
  for (const [action, actor, fields] of [
    ["request_information", fixture.desktopActor, { purpose: "information", target_user_id: fixture.webActor.id }],
    ["return_information", fixture.webActor, {}], ["submit_acceptance", fixture.desktopActor, {}],
  ] as const) {
    const before = await readIssue(actor.client, fixture.issueId);
    const options = CollaborationStatusOptionsSchema.parse(await actor.client.requestJSON(`${root}/status-options?action=${action}`));
    expect(options.action).toBe(action);
    const requestId = randomUUID();
    const input = { ...observations(before), ...fields, expected_status: options.default_status, status_options_version: options.status_options_version,
      note: `Offline ${action} fixture`, ...(action === "return_information" ? { information_request_id: before.current_information_request!.id } : {}) };
    const path = `${root}/${action === "request_information" ? "handoffs" : action === "return_information" ? "information-returns" : "acceptance-submissions"}`;
    const receipt = CollaborationOperationSchema.parse(await actor.client.requestJSON(path, { method: "POST", headers: { "Idempotency-Key": requestId }, body: input }));
    const kind = action === "request_information" ? "information_requested" : action === "return_information" ? "information_returned" : "acceptance_submitted";
    expect(receipt.code).toBe(kind);
    const after = await readIssue(actor.client, fixture.issueId);
    expect(after.session).toEqual(before.session);
    expect(after.execution).toEqual(before.execution);
    const received = (await notifications(recipient.client, fixture.issueId)).filter((item) => item.kind === kind);
    expect(received).toHaveLength(1);
    const notice = received[0]!;
    expect((await notifications(actor.client, fixture.issueId)).filter((item) => item.event_seq === notice.event_seq)).toEqual([]);
    const other = actor.id === fixture.webActor.id ? fixture.desktopActor : fixture.webActor;
    expect((await notifications(other.client, fixture.issueId)).filter((item) => item.event_seq === notice.event_seq)).toHaveLength(1);
    const beforeReplay = await notifications(recipient.client, fixture.issueId);
    expect(CollaborationOperationSchema.parse(await actor.client.requestJSON(path, { method: "POST", headers: { "Idempotency-Key": requestId }, body: input }))).toEqual(receipt);
    expect(await notifications(recipient.client, fixture.issueId)).toEqual(beforeReplay);
    expect(await readIssue(actor.client, fixture.issueId)).toEqual(after);
    events.push({ kind, actor_id: actor.id, request_id: requestId, notification_id: notice.id, event_seq: notice.event_seq });
  }
  expect((await readIssue(fixture.webActor.client, fixture.issueId)).handler.user_id).toBe(fixture.webActor.id);
  return { events, source_isolation: await sourceIsolation(state) };
}

export async function publishOfflineRunResults(state: OfflineResultFixture, creatorPage: Page, transport: Awaited<ReturnType<typeof notificationRuntimeTransport>>) {
  const { fixture, recipient } = state;
  await refreshLifecyclePage(creatorPage, fixture);
  await lifecycleControl(creatorPage, fixture, "resume");
  const daemons = await startCrossClientDaemons({ ...fixture, apiUrl: transport.apiUrl });
  let closed = false;
  const results: { kind: string; run_id: string; group_id: string; notification_id: string; event_seq: string; proof_replay: { status: number; original_sha256: string }; missing_outcome: boolean }[] = [];
  try {
    for (const [index, mode] of ["completed", "incomplete", "stopped"].entries()) {
      const input = await lifecycleComment(creatorPage, fixture, fixture.targetA, fixture.webActor, `AC83 ${mode} input`, "web");
      const visit = await daemons.waitForVisit("A", index);
      assertProviderInput(visit, fixture, fixture.targetA, [input]);
      const runId = visit.input.identity.run_id;
      transport.holdProof(runId);
      if (mode === "incomplete") transport.loseOutcome(runId);
      if (mode === "stopped") {
        // The actual provider has claimed since the comment's waiting receipt.
        // Adopt that exact current group observation before the explicit Stop.
        await refreshLifecyclePage(creatorPage, fixture);
        const stop = await lifecycleControl(creatorPage, fixture, "stop", fixture.targetA);
        expect(stop.receipt.code).toBe("stop_requested");
        expect(visit.released_ms).toBeNull();
      } else daemons.release(visit);
      await transport.waitForProof(runId, 503);
      expect((await notifications(recipient.client, fixture.issueId)).filter((item) => item.run_id === runId)).toEqual([]);
      const beforeProof = await readIssue(fixture.webActor.client, fixture.issueId);
      expect(beforeProof.execution!.runs.find((run) => run.id === runId)).toMatchObject({ proof_state: "unknown", settled_at: null,
        business_outcome: mode === "completed" ? "success" : null });
      transport.releaseProof(runId);
      await transport.waitForProof(runId, 200);
      const kind = mode === "completed" ? "ai_completed" : mode === "incomplete" ? "ai_failed" : "ai_stopped";
      await expect.poll(async () => (await notifications(recipient.client, fixture.issueId)).filter((item) => item.run_id === runId), { timeout: 15_000 })
        .toEqual([expect.objectContaining({ kind, source_kind: "issue", source_id: null, can_open: true, issue_title: fixture.title })]);
      const notice = (await notifications(recipient.client, fixture.issueId)).find((item) => item.run_id === runId)!;
      const after = await readIssue(fixture.webActor.client, fixture.issueId);
      const group = after.execution!.groups.find((item) => item.id === visit.input.identity.group_id)!;
      expect(group.state).toBe(mode === "completed" ? "succeeded" : mode === "incomplete" ? "failed" : "stopped");
      expect(after.execution!.runs.find((run) => run.id === runId)).toMatchObject({ proof_state: "exited_proven", progress_state: "settled",
        business_outcome: mode === "completed" ? "success" : null, forced_disposition: mode === "stopped" ? "stopped" : null,
        recording_incomplete: mode !== "completed" });
      for (const actor of [fixture.webActor, fixture.desktopActor]) expect((await notifications(actor.client, fixture.issueId)).filter((item) => item.event_seq === notice.event_seq)).toHaveLength(1);
      const proofReplay = await transport.replayProof(runId);
      expect(await readIssue(fixture.webActor.client, fixture.issueId)).toEqual(after);
      expect((await notifications(recipient.client, fixture.issueId)).filter((item) => item.run_id === runId)).toEqual([notice]);
      results.push({ kind, run_id: runId, group_id: group.id, notification_id: notice.id, event_seq: notice.event_seq,
        proof_replay: proofReplay, missing_outcome: mode === "incomplete" });
    }
    expect(daemons.visits.map((visit) => visit.lane)).toEqual(["A", "A", "A"]);
    expect(daemons.callbackErrors).toEqual([]);
    const providerObservations = daemons.evidence();
    const summaries = await daemons.close();
    closed = true;
    expect(summaries.map((summary) => summary.exit_code)).toEqual([0, 0]);
    expect(summaries[0]!.messages.at(-1)).toMatchObject({ claims: 3, invalid_capacity: false, available_slots: 1, active_tasks: 0 });
    expect(summaries[1]!.messages.at(-1)).toMatchObject({ claims: 0, invalid_capacity: false, available_slots: 1, active_tasks: 0 });
    return { results, provider_observations: providerObservations, daemon_capacity: summaries.map(({ lane, messages, exit_code }) => ({ lane, messages, exit_code })),
      runtime_transport: transport.evidence(), source_isolation: await sourceIsolation(state) };
  } finally { if (!closed) await daemons.close(); }
}

async function popupFor(page: Page) {
  await page.getByRole("button", { name: /^Notifications/ }).click();
  const popup = page.locator('[data-slot="popover-content"]').filter({ has: page.getByText("Notifications", { exact: true }) });
  await expect(popup).toBeVisible();
  return popup;
}

export async function readOfflineResults(state: OfflineResultFixture, clients: HandoffClients) {
  const { fixture, recipient } = state;
  const session = await clients.open(recipient, "/account?view=participating");
  const page = session.page;
  const received = (await notifications(recipient.client, fixture.issueId)).filter((item) => requiredKinds.includes(item.kind as typeof requiredKinds[number]));
  expect(received).toHaveLength(6);
  for (const kind of requiredKinds) expect(received.filter((item) => item.kind === kind)).toHaveLength(1);
  expect(await recipient.client.getWorkspaces()).toEqual([]);
  const participating = CollaborationWorkbenchSchema.parse(await recipient.client.requestJSON("/api/me/workbench?view=participating"));
  const current = CollaborationWorkbenchSchema.parse(await recipient.client.requestJSON("/api/me/workbench?view=current"));
  expect(participating.items.map((item) => item.id)).toEqual([fixture.issueId]);
  expect(current.items).toEqual([]);
  const archived: { kind: string; id: string; href: string; run_id?: string }[] = [];
  for (const kind of requiredKinds) {
    const notice = received.find((item) => item.kind === kind)!;
    let popup = await popupFor(page);
    let row = popup.getByRole("listitem").filter({ has: page.getByText(labels[kind], { exact: true }) });
    await expect(row).toHaveCount(1);
    const link = row.getByRole("link", { name: fixture.title, exact: true });
    const href = await link.getAttribute("href");
    if (!href) throw new Error("The actual offline notification did not provide an issue destination");
    const url = new URL(href, fixture.apiUrl);
    expect(url.searchParams.get("issue")).toBe(fixture.issueId);
    if (notice.run_id) expect(url.hash).toBe(`#run-${notice.run_id}`);
    await link.click();
    await expect(popup).toBeHidden();
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    if (notice.run_id) {
      const run = page.locator(`[id="run-${notice.run_id}"]`);
      await expect(run).toBeFocused();
      await expect(run).toContainText("Termination confirmed");
      const records = run.getByRole("region", { name: "Run records", exact: true });
      await expect(records).toBeVisible();
      const snapshot = CollaborationRunRecordsSchema.parse(await recipient.client.requestJSON(`${rootFor(fixture.issueId)}/runs/${notice.run_id}/records?limit=100`));
      expect(snapshot.run.proof_state).toBe("exited_proven");
      if (kind === "ai_failed") {
        expect(snapshot.run.business_outcome).toBeNull();
        expect(snapshot.recording.status).toBe("partial");
        await expect(records).toContainText("No business result recorded");
        await expect(records).toContainText("Partial recording");
      }
    }
    popup = await popupFor(page);
    row = popup.getByRole("listitem").filter({ has: page.getByText(labels[kind], { exact: true }) });
    const before = await readIssue(fixture.webActor.client, fixture.issueId);
    for (const [action, label] of [["read", "Mark read"], ["archive", "Archive"]] as const) {
      const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/me/collaboration-notifications/${notice.id}/${action}` && response.request().method() === "POST");
      await row.getByRole("button", { name: label, exact: true }).click();
      const response = await pending;
      expect(response.status()).toBe(200);
      expect(await response.json()).toMatchObject({ id: notice.id, [action === "read" ? "read_at" : "archived_at"]: expect.any(String) });
      expect(await readIssue(fixture.webActor.client, fixture.issueId)).toEqual(before);
      expect(CollaborationWorkbenchSchema.parse(await recipient.client.requestJSON("/api/me/workbench?view=participating"))).toEqual(participating);
      expect(CollaborationWorkbenchSchema.parse(await recipient.client.requestJSON("/api/me/workbench?view=current"))).toEqual(current);
    }
    await expect(row).toHaveCount(0);
    await page.getByRole("button", { name: /^Notifications/ }).click();
    await expect(popup).toBeHidden();
    archived.push({ kind, id: notice.id, href, ...(notice.run_id ? { run_id: notice.run_id } : {}) });
  }
  return { session, archived, recipient_notices: await notifications(recipient.client, fixture.issueId), source_isolation: await sourceIsolation(state) };
}

export async function removeOfflineResultViewer(state: OfflineResultFixture, session: Awaited<ReturnType<typeof readOfflineResults>>["session"], oldNotices: { href: string; run_id?: string }[]) {
  const { fixture, recipient } = state;
  const root = rootFor(fixture.issueId);
  const removal = CollaborationOperationSchema.parse(await fixture.webActor.client.requestJSON(`${root}/participants/${recipient.id}/remove`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: observations(await readIssue(fixture.webActor.client, fixture.issueId)),
  }));
  expect(removal.code).toBe("participant_removed");
  await expect(session.page.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible({ timeout: 15_000 });
  await expect(session.page.getByRole("heading", { name: fixture.title, exact: true })).toHaveCount(0);
  const redacted = await notifications(recipient.client, fixture.issueId);
  expect(redacted.filter((item) => item.kind === "removed")).toHaveLength(1);
  for (const item of redacted) expect(item).toMatchObject({ issue_title: null, can_open: false, source_kind: "issue", source_id: null });
  expect(redacted.every((item) => item.run_id === undefined)).toBe(true);
  const old = oldNotices.find((item) => item.run_id)!;
  const url = new URL(old.href, fixture.apiUrl);
  await session.navigate(`${url.pathname}${url.search}${url.hash}`);
  await expect(session.page.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
  await expect(session.page.getByRole("heading", { name: fixture.title, exact: true })).toHaveCount(0);
  for (const path of [root, `${root}/runs/${old.run_id}/records?limit=100`]) {
    const denied = await fetch(`${fixture.apiUrl}${path}`, { headers: { Authorization: `Bearer ${recipient.client.getToken()!}` } });
    expect(denied.status).toBe(404);
  }
  expect(CollaborationWorkbenchSchema.parse(await recipient.client.requestJSON("/api/me/workbench?view=participating")).items).toEqual([]);
  expect((await readIssue(fixture.webActor.client, fixture.issueId)).id).toBe(fixture.issueId);
  return { removal, redacted, old_result_href: old.href, old_entry_denied: true, source_isolation: await sourceIsolation(state) };
}
