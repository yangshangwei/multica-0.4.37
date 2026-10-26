import { randomBytes, randomUUID } from "node:crypto";
import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import type { CollaborationIssue } from "@multica/core/collaboration";
import { TestApiClient } from "./fixtures";
import { authenticateCollaborationEditor } from "./fixtures/collaboration-editing";
import { sql } from "./fixtures/core-journeys";

// Collaboration boundaries no client spec proved (collab-audit.md §2 Gap 5,
// Gap 7 and the smaller gaps): a source workspace holding collaboration
// records cannot be deleted; every hidden manager action POSTed by the wrong
// role is refused by the server and changes nothing; a source member outside
// the issue may comment/resolve/react but cannot start AI work; field edits
// and exact recipient lookup behave as specified. Issues, participants and
// handoffs are created through the real collaboration API, as
// collaboration-delivery-web does. Collaboration records are retained by
// contract, so the workspaces created here are deliberately left in the
// disposable lane database.
const apiUrl = process.env.COLLABORATION_E2E_API_URL;
test.use({ locale: "en-US", viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure", screenshot: "only-on-failure" });

type Account = { api: TestApiClient; id: string; email: string; name: string };
type Response = { status: number; body: Record<string, unknown> };
type Workspace = { id: string; slug: string; name: string };

function requireLane(needsDeployment = false) {
  test.skip(!apiUrl || !process.env.DATABASE_URL || !process.env.NEXT_PUBLIC_API_URL || (needsDeployment && !process.env.MULTICA_DEPLOYMENT_ID),
    "Requires the isolated collaboration API/database and a same-origin production Web build with creation enabled");
  for (const value of [apiUrl!, process.env.DATABASE_URL!, process.env.NEXT_PUBLIC_API_URL!]) expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(apiUrl!).origin);
}

async function account(label: string, key: string, workspace?: string): Promise<Account & { workspace?: Workspace }> {
  const api = new TestApiClient();
  const email = `boundary-${label.toLowerCase()}-${key}@example.invalid`;
  const name = `Boundary ${label} ${key}`;
  await api.login(email, name);
  const created = workspace ? await api.ensureWorkspace(workspace, `boundary-${label.toLowerCase()}-${key}`) as Workspace : undefined;
  await api.markUserOnboarded();
  const me = await api.requestJSON<{ id: string }>("/api/me");
  return { api, id: me.id, email, name, workspace: created };
}

/** A raw call whose refusal status and body are the assertion, never an exception. */
async function send(actor: Account, path: string, method = "GET", body?: unknown, headers: Record<string, string> = {}): Promise<Response> {
  const response = await fetch(`${apiUrl}${path}`, {
    method,
    headers: { Authorization: `Bearer ${actor.api.getToken()}`, ...(body === undefined ? {} : { "Content-Type": "application/json" }), ...headers },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  return { status: response.status, body: text ? JSON.parse(text) as Record<string, unknown> : {} };
}
const write = (actor: Account, path: string, body: unknown) => send(actor, path, "POST", body, { "Idempotency-Key": randomUUID() });
const root = (issueId: string) => `/api/collaboration/issues/${issueId}`;
const readIssue = (actor: Account, issueId: string) => actor.api.requestJSON<CollaborationIssue>(root(issueId));

/** The viewer's own observations. A non-participant has no write context; its
 * placeholders are irrelevant because the role check precedes every epoch check
 * (workflowInTx, server/internal/service/collaboration_workflow.go:517-555). */
function observed(issue: CollaborationIssue) {
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context?.participant_epoch ?? "1", expected_account_epoch: issue.write_context?.account_epoch ?? "1" };
}

/** Everything a wrongly accepted manager action could change, read by the creator. */
async function businessState(creator: Account, issueId: string) {
  const issue = await readIssue(creator, issueId);
  const participants = await sql<{ user_id: string; active: boolean }>(
    "SELECT user_id::text, revoked_at IS NULL AS active FROM issue_participant WHERE issue_id = $1 ORDER BY user_id", [issueId]);
  return { revision: issue.collaboration_revision, lifecycle: issue.lifecycle_epoch, status: issue.status, handler: issue.handler.user_id,
    title: issue.title, start: issue.start_date ?? null, due: issue.due_date ?? null, queue: issue.session.queue_state,
    commands: issue.execution?.commands.length ?? 0, participants };
}

async function uploadDraft(creator: Account, workspaceId: string, name: string, bytes: Buffer) {
  const form = new FormData();
  const draftId = randomUUID();
  form.set("draft_id", draftId);
  form.set("source_workspace_id", workspaceId);
  form.set("file", new Blob([new Uint8Array(bytes)], { type: "text/plain" }), name);
  const response = await fetch(`${apiUrl}/api/collaboration/attachments/drafts`, {
    method: "POST", headers: { Authorization: `Bearer ${creator.api.getToken()}`, "Idempotency-Key": randomUUID() }, body: form,
  });
  expect(response.status).toBe(201);
  const uploaded = await response.json() as { id: string; draft_id: string };
  expect(uploaded.draft_id).toBe(draftId);
  return uploaded;
}

/**
 * P owns source workspace W and creates the issue for external handler F, then
 * invites external Q; F transfers handling to external H, so F is a former
 * handler. M is an ordinary W member who never joins; R is an unrelated account.
 */
async function seedBoundary(withAttachment = false) {
  const key = randomUUID().slice(0, 8);
  const creator = await account("Creator", key, "Boundary source");
  const workspace = creator.workspace!;
  const [former, handler, peer, member, outsider] = await Promise.all(
    ["Former", "Handler", "Peer", "Member", "Outsider"].map((label) => account(label, key)));
  expect(await creator.api.requestJSON("/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const invitation = await creator.api.requestJSON<{ id: string }>(`/api/workspaces/${workspace.id}/members`, {
    method: "POST", body: { email: member!.email, role: "member" } });
  await member!.api.requestJSON(`/api/invitations/${invitation.id}/accept`, { method: "POST" });

  const fileBytes = Buffer.from(`Retained boundary attachment ${key}`);
  const draft = withAttachment ? await uploadDraft(creator, workspace.id, "boundary-note.txt", fileBytes) : null;
  const created = await write(creator, "/api/collaboration/issues", {
    source_workspace_id: workspace.id, handler_user_id: former!.id, title: `Boundary collaboration ${key}`,
    description: "Shared work whose boundaries are checked by the server.", start_date: "2026-11-02", due_date: "2026-11-20",
    ...(draft ? { draft_id: draft.draft_id, attachment_ids: [draft.id] } : { attachment_ids: [] }),
  });
  expect(created.status, JSON.stringify(created.body)).toBe(201);
  const issueId = String(created.body.id);
  const invited = await write(creator, `${root(issueId)}/participants`, { ...observed(await readIssue(creator, issueId)), target_user_id: peer!.id });
  expect(invited.body).toMatchObject({ code: "participant_added" });
  const transferred = await write(former!, `${root(issueId)}/handoffs`, {
    ...observed(await readIssue(former!, issueId)), target_user_id: handler!.id, purpose: "transfer", note: "Handing over to H." });
  expect(transferred.body).toMatchObject({ code: "transferred" });
  const issue = await readIssue(creator, issueId);
  expect(issue).toMatchObject({ creator: { user_id: creator.id }, handler: { user_id: handler!.id }, start_date: "2026-11-02", due_date: "2026-11-20" });
  return { key, creator, workspace, former: former!, handler: handler!, peer: peer!, member: member!, outsider: outsider!,
    issueId, title: issue.title, attachment: draft ? { id: draft.id, bytes: fileBytes } : null };
}
type Boundary = Awaited<ReturnType<typeof seedBoundary>>;

async function openPage(browser: Browser, actor: Account): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext({ locale: "en-US", viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  await authenticateCollaborationEditor(page, actor.api.getToken()!);
  return { context, page };
}

/** Source members open the source route; external accounts open the account route. */
async function openIssuePage(page: Page, fixture: Boundary, viewer: "source" | "account") {
  await page.goto(viewer === "source" ? `/${fixture.workspace.slug}/issues/${fixture.issueId}` : `/account?view=participating&issue=${fixture.issueId}`);
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible({ timeout: 30_000 });
}
const actionsRow = (page: Page) => page.getByRole("region", { name: "Issue actions", exact: true }).locator(":scope > div").first();

/** A human comment as its author; a non-participant sends no write context (CreateComment accepts 0). */
async function postComment(author: Account, issueId: string, content: string, targetVersions: unknown[] = []) {
  const issue = await readIssue(author, issueId);
  return write(author, `${root(issueId)}/comments`, { content, expected_lifecycle_epoch: issue.lifecycle_epoch,
    ...(issue.write_context ? { expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch } : {}),
    target_versions: targetVersions, attachment_ids: [] });
}

/** Row counts that a partial workspace teardown would change. */
async function workspaceInventory(workspaceId: string, issueId: string) {
  const [row] = await sql<Record<string, string>>(`SELECT
      (SELECT count(*) FROM workspace WHERE id = $1)::text AS workspace,
      (SELECT count(*) FROM member WHERE workspace_id = $1)::text AS members,
      (SELECT count(*) FROM issue WHERE workspace_id = $1)::text AS issues,
      (SELECT count(*) FROM issue_collaboration WHERE issue_id = $2)::text AS collaboration,
      (SELECT count(*) FROM issue_participant WHERE issue_id = $2 AND revoked_at IS NULL)::text AS active_participants,
      (SELECT count(*) FROM comment WHERE issue_id = $2)::text AS comments,
      (SELECT count(*) FROM collaboration_attachment WHERE issue_id = $2)::text AS attachments,
      (SELECT count(*) FROM collaboration_event WHERE issue_id = $2)::text AS events`, [workspaceId, issueId]);
  return row!;
}

test("Gap 5: a source workspace holding a collaboration issue cannot be deleted, open or completed, and nothing is cleaned up", async ({ browser }) => {
  test.setTimeout(180_000);
  requireLane();
  const fixture = await seedBoundary(true);
  const { creator, handler, peer, workspace, issueId } = fixture;
  const comment = await postComment(creator, issueId, `Retained discussion ${fixture.key}`);
  expect(comment.status).toBe(201);
  const ordinary = await creator.api.createIssue(`Ordinary sibling ${fixture.key}`, { status: "todo" }) as { id: string; title: string };
  const deleteDirect = () => send(creator, `/api/workspaces/${workspace.id}`, "DELETE", undefined, { "X-Workspace-ID": workspace.id });

  // Open issue: refused with the documented code, and the inventory is intact.
  // DeleteWorkspace checks WorkspaceHasCollaborationRecords inside its
  // transaction before any teardown (server/internal/handler/workspace.go:1131-1139).
  const before = await workspaceInventory(workspace.id, issueId);
  expect(before).toMatchObject({ workspace: "1", collaboration: "1", active_participants: "4", attachments: "1", comments: "1" });
  const openRefusal = await deleteDirect();
  expect(openRefusal).toEqual({ status: 409, body: { code: "workspace_has_collaboration_records", error: "This workspace contains collaboration records." } });
  expect(await workspaceInventory(workspace.id, issueId)).toEqual(before);

  // Completed by the creator, and a non-creator participant has left: still refused (AC62).
  const options = await creator.api.requestJSON<{ default_status: string; status_options_version: string }>(`${root(issueId)}/status-options?action=complete`);
  const completed = await write(creator, `${root(issueId)}/lifecycle`, { ...observed(await readIssue(creator, issueId)), action: "complete",
    status: options.default_status, status_options_version: options.status_options_version });
  expect(completed.body).toMatchObject({ code: "completed" });
  const left = await write(peer, `${root(issueId)}/leave`, observed(await readIssue(peer, issueId)));
  expect(left.body).toMatchObject({ code: "participant_left" });
  expect(await readIssue(creator, issueId)).toMatchObject({ status_category: "done", session: { queue_state: "closed" } });
  const settled = await workspaceInventory(workspace.id, issueId);
  const { events: eventsBefore, active_participants: _activeBefore, ...stableBefore } = before;
  const { events: eventsSettled, active_participants: activeSettled, ...stableSettled } = settled;
  expect(stableSettled).toEqual(stableBefore);
  expect(activeSettled).toBe("3");
  expect(Number(eventsSettled)).toBeGreaterThan(Number(eventsBefore));

  // The owner deletes through the settings dialog; it stays open with the reason.
  const { context, page } = await openPage(browser, creator);
  try {
    let deletes = 0;
    page.on("request", (request) => {
      if (request.method() === "DELETE" && new URL(request.url()).pathname === `/api/workspaces/${workspace.id}`) deletes += 1;
    });
    await page.goto(`/${workspace.slug}/settings?tab=workspace`);
    await page.getByRole("button", { name: "Delete workspace", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Delete workspace", exact: true });
    await expect(dialog).toBeVisible();
    // The typed confirmation is the workspace name (delete-workspace-dialog.tsx), not the slug.
    await dialog.getByRole("textbox").fill(workspace.name);
    const refused = page.waitForResponse((r) => r.request().method() === "DELETE" && new URL(r.url()).pathname === `/api/workspaces/${workspace.id}`);
    await dialog.getByRole("button", { name: "Delete workspace", exact: true }).click();
    const response = await refused;
    expect(response.status()).toBe(409);
    expect(await response.json()).toEqual({ code: "workspace_has_collaboration_records", error: "This workspace contains collaboration records." });
    await expect(page.getByText("This workspace contains collaboration records.", { exact: true })).toBeVisible();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("textbox")).toHaveValue(workspace.name);
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/settings\\?tab=workspace$`));
    expect(deletes).toBe(1);
    await page.screenshot({ path: test.info().outputPath("workspace-delete-refused.png"), animations: "disabled" });
  } finally { await context.close(); }

  // Nothing was partially removed (AC63): rows, membership, issue, comment, file bytes.
  expect(await workspaceInventory(workspace.id, issueId)).toEqual(settled);
  expect((await creator.api.getWorkspaces()).map((item) => item.id)).toContain(workspace.id);
  expect(await creator.api.requestJSON(`/api/issues/${ordinary.id}`)).toMatchObject({ title: ordinary.title });
  expect(await readIssue(handler, issueId)).toMatchObject({ id: issueId, status_category: "done", handler: { user_id: handler.id } });
  const comments = await handler.api.requestJSON<{ items: { id: string; content: string }[] }>(`${root(issueId)}/comments`);
  expect(comments.items).toEqual(expect.arrayContaining([expect.objectContaining({ id: comment.body.comment_id, content: `Retained discussion ${fixture.key}` })]));
  const file = await fetch(`${apiUrl}${root(issueId)}/attachments/${fixture.attachment!.id}/content`, { headers: { Authorization: `Bearer ${handler.api.getToken()}` } });
  expect(file.status).toBe(200);
  expect(Buffer.from(await file.arrayBuffer())).toEqual(fixture.attachment!.bytes);
  expect((await send(peer, root(issueId))).status).toBe(404);
});

/** Visible labels of the workflow buttons (workflow.tsx renders one per true capability). */
const visibleActions = async (page: Page) => (await actionsRow(page).getByRole("button").allTextContents()).map((text) => text.trim());

/**
 * P registers and authorizes its own agent A as an AI target, so A is in every
 * reader's mention catalogue. Only the machine identity is synthetic: lane L has
 * managed installations off, and consent requires an "enrolled" runtime
 * (collaborationIdentityCurrent, server/internal/service/collaboration_authority.go:212-252),
 * which no product route issues. This mirrors e2e/fixtures/collaboration-resources.ts.
 */
async function authorizeOwnAgent(page: Page, fixture: Boundary) {
  const runtime = await fixture.creator.api.seedProjectRuntime();
  const agentName = `Boundary agent ${fixture.key}`;
  const agent = await fixture.creator.api.requestJSON<{ id: string }>("/api/agents", { method: "POST", body: { name: agentName, runtime_id: runtime.id } });
  await sql("UPDATE agent_runtime SET status = 'offline' WHERE id = $1 AND owner_id = $2", [runtime.id, fixture.creator.id]);
  await sql(`INSERT INTO collaboration_runtime_enrollment (deployment_id, owner_user_id, execution_workspace_id, runtime_id, installation_id, public_key)
    VALUES ($1, $2, $3, $4, $5, $6)`, [process.env.MULTICA_DEPLOYMENT_ID, fixture.creator.id, fixture.workspace.id, runtime.id, randomUUID(), randomBytes(32)]);
  await page.goto(`/account?view=participating&issue=${fixture.issueId}`);
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible({ timeout: 30_000 });
  await page.getByRole("button", { name: "Manage AI resources", exact: true }).click();
  await page.getByRole("button", { name: "Add AI target", exact: true }).click();
  await page.getByLabel("Resource workspace", { exact: true }).selectOption(fixture.workspace.id);
  await page.getByLabel("AI target", { exact: true }).selectOption(`agent:${agent.id}`);
  await page.getByRole("button", { name: "Register target", exact: true }).click();
  await expect(page.getByText("Target registered. Each owner must authorize their resources.")).toBeVisible();
  await page.getByRole("button", { name: `Authorize ${agentName}`, exact: true }).click();
  await page.getByRole("button", { name: "Confirm authorization", exact: true }).click();
  await expect(page.getByText("Resource authorized. Explicit resume is still required.")).toBeVisible();
  const resources = await fixture.creator.api.requestJSON<{ targets: { target_id: string; resource_id: string; authorized: boolean }[] }>(`${root(fixture.issueId)}/resources`);
  const target = resources.targets.find((item) => item.resource_id === agent.id);
  expect(target).toMatchObject({ authorized: true });
  return { agentId: agent.id, agentName, targetId: target!.target_id };
}

type MentionCatalogue = { targets: { target_id: string; resource_id: string; target_epoch: string; config_version: string; waiting_reason: string }[]; can_submit_commands: boolean };
type CommentReceipt = { status: number; code: string; comment_id: string; targets: { target_id: string | null; state: string; code: string; command_id?: string | null }[] };

test("Gap 7: hidden manager actions POSTed by the wrong role are refused and change nothing; a non-participant's @agent never queues", async ({ browser }) => {
  test.setTimeout(240_000);
  requireLane(true);
  const fixture = await seedBoundary();
  const { creator, handler, peer, former, member, outsider, issueId } = fixture;

  // Server-side capabilities per role (CapabilitiesFor, server/internal/collaboration/access.go:79-110).
  // The issue projection always reports submit_command=false ("Submission remains
  // gated", server/internal/handler/collaboration.go:183-185); the per-role
  // submission decision is the mention catalogue's can_submit_commands, checked below.
  const caps = async (actor: Account) => {
    const c = (await readIssue(actor, issueId)).capabilities;
    expect(c.submit_command).toBe(false);
    return { invite: c.invite, transfer: c.transfer, remove: c.remove_participants, complete: c.complete, cancel: c.cancel, edit: c.edit,
      leave: c.leave, control_ai: c.control_ai, comment: c.comment, resolve: c.resolve_thread, react: c.react };
  };
  const none = { invite: false, transfer: false, remove: false, complete: false, cancel: false, edit: false };
  const commandRows = () => sql<{ author_user_id: string; state: string }>(
    "SELECT c.author_user_id::text, c.state FROM collaboration_command c JOIN collaboration_session s ON s.id = c.session_id WHERE s.issue_id = $1 ORDER BY c.seq", [issueId]);
  expect(await caps(creator)).toEqual({ invite: true, transfer: true, remove: true, complete: true, cancel: true, edit: true, leave: false, control_ai: true, comment: true, resolve: true, react: true });
  expect(await caps(handler)).toEqual({ invite: true, transfer: true, remove: false, complete: false, cancel: false, edit: true, leave: false, control_ai: true, comment: true, resolve: true, react: true });
  for (const actor of [peer, former]) expect(await caps(actor)).toEqual({ ...none, leave: true, control_ai: true, comment: true, resolve: true, react: true });
  expect(await caps(member)).toEqual({ ...none, leave: false, control_ai: false, comment: true, resolve: true, react: true });
  expect((await readIssue(member, issueId)).write_context ?? null).toBeNull();

  // A is authorized before any reader page loads, so every catalogue lists it.
  const pages: Record<string, { context: BrowserContext; page: Page }> = {};
  try {
    for (const [name, actor] of Object.entries({ creator, handler, peer, former, member })) pages[name] = await openPage(browser, actor);
    const agent = await authorizeOwnAgent(pages.creator!.page, fixture);
    const catalogue = (actor: Account) => actor.api.requestJSON<MentionCatalogue>(`${root(issueId)}/mention-targets`);
    const creatorView = await catalogue(creator);
    const memberView = await catalogue(member);
    expect(creatorView.targets.map((item) => item.target_id)).toEqual([agent.targetId]);
    expect(memberView.targets.map((item) => item.target_id)).toEqual([agent.targetId]);
    const submitters = await Promise.all([creator, handler, peer, former, member].map(async (actor) => (await catalogue(actor)).can_submit_commands));
    expect(submitters).toEqual([true, true, true, true, false]);
    expect(memberView.targets[0]!.waiting_reason).toBe("command_unavailable");
    const versions = creatorView.targets.map((item) => ({ target_id: item.target_id, expected_target_epoch: item.target_epoch, expected_config_version: item.config_version }));
    const mention = `[@${agent.agentName}](mention://agent/${agent.agentId})`;
    // Control: the same mention and versions from a participant do queue.
    const control = await postComment(creator, issueId, `${mention} please look at this`, versions);
    expect(control.status, JSON.stringify(control.body)).toBe(201);
    expect((control.body as unknown as CommentReceipt).targets).toEqual([expect.objectContaining({ target_id: agent.targetId, state: "waiting" })]);
    expect(await commandRows()).toEqual([{ author_user_id: creator.id, state: "waiting" }]);

    // Each client shows exactly the actions its role holds.
    await openIssuePage(pages.creator!.page, fixture, "source");
    const creatorActions = ["Add participant", "Transfer", "Request information", "Change status", "Complete issue", "Cancel issue", "Edit issue"];
    if ((await readIssue(creator, issueId)).source_structure !== undefined) creatorActions.push("Edit source fields", "Change iteration");
    await expect.poll(() => visibleActions(pages.creator!.page)).toEqual(creatorActions);
    await openIssuePage(pages.handler!.page, fixture, "account");
    await expect.poll(() => visibleActions(pages.handler!.page))
      .toEqual(["Add participant", "Transfer", "Request information", "Submit for acceptance", "Change status", "Edit issue"]);
    for (const name of ["peer", "former"]) {
      await openIssuePage(pages[name]!.page, fixture, "account");
      await expect.poll(() => visibleActions(pages[name]!.page)).toEqual(["Leave issue"]);
    }
    await openIssuePage(pages.member!.page, fixture, "source");
    await expect(actionsRow(pages.member!.page).getByRole("button")).toHaveCount(0);
    // Only the creator can remove participants (participants.tsx canRemove).
    for (const name of ["creator", "handler"]) {
      await pages[name]!.page.getByRole("region", { name: "Issue actions", exact: true }).getByRole("button", { name: "Participants", exact: true }).click();
      await expect(pages[name]!.page.getByRole("list", { name: "Participants", exact: true }).getByRole("listitem")).toHaveCount(4);
    }
    await expect(pages.creator!.page.getByRole("button", { name: `Remove ${peer.name}`, exact: true })).toBeVisible();
    await expect(pages.handler!.page.getByRole("button", { name: /^Remove / })).toHaveCount(0);

    // M, a source member outside the issue, comments through the real composer
    // and mentions A: the comment lands, A is refused for M's role and nothing
    // queues (collaborationCommentTargetPreview, collaboration_mentions.go:181-182).
    const memberPage = pages.member!.page;
    const composer = memberPage.getByRole("form", { name: "Comments", exact: true });
    await composer.getByLabel("Add comment", { exact: true }).fill(`Source member note ${fixture.key}`);
    await composer.getByRole("group", { name: "AI targets", exact: true }).getByRole("button", { name: agent.agentName, exact: true }).click();
    await composer.getByRole("button", { name: "Preview delivery", exact: true }).click();
    await expect(composer.getByText("Only an active participant can submit AI input", { exact: true })).toBeVisible();
    const sent = memberPage.waitForResponse((r) => new URL(r.url()).pathname === `${root(issueId)}/comments` && r.request().method() === "POST");
    await composer.getByRole("button", { name: "Send comment", exact: true }).click();
    const memberComment = await sent;
    expect(memberComment.status()).toBe(201);
    const receipt = await memberComment.json() as CommentReceipt;
    expect(memberComment.request().postDataJSON().content).toContain(`mention://agent/${agent.agentId}`);
    expect(receipt).toMatchObject({ code: "comment_created", targets: [{ target_id: agent.targetId, state: "blocked", code: "not_participant" }] });
    expect(receipt.targets[0]!.command_id ?? null).toBeNull();
    await expect(memberPage.locator(`[id="comment-${receipt.comment_id}"]`)).toContainText(`Source member note ${fixture.key}`);
    await expect(composer.getByText(`${agent.agentName} · Only an active participant can submit AI input`, { exact: true })).toBeVisible();
    expect(await commandRows()).toEqual([{ author_user_id: creator.id, state: "waiting" }]);
    expect((await readIssue(creator, issueId)).execution?.commands.map((command) => command.state)).toEqual(["waiting"]);

    // Every hidden manager action, POSTed directly with the actor's own fresh
    // observations, gets the committed 403 "forbidden" receipt
    // (workflowRoleAllowed + workflowInTx, collaboration_workflow.go:343-369,517-541).
    const complete = await creator.api.requestJSON<{ default_status: string; status_options_version: string }>(`${root(issueId)}/status-options?action=complete`);
    const cancel = await creator.api.requestJSON<{ default_status: string; status_options_version: string }>(`${root(issueId)}/status-options?action=cancel`);
    const attempts: Record<string, (actor: Account, seen: ReturnType<typeof observed>) => Promise<Response>> = {
      invite: (actor, seen) => write(actor, `${root(issueId)}/participants`, { ...seen, target_user_id: outsider.id }),
      transfer: (actor, seen) => write(actor, `${root(issueId)}/handoffs`, { ...seen, target_user_id: outsider.id, purpose: "transfer", note: "" }),
      remove: (actor, seen) => write(actor, `${root(issueId)}/participants/${peer.id === actor.id ? former.id : peer.id}/remove`, seen),
      complete: (actor, seen) => write(actor, `${root(issueId)}/lifecycle`, { ...seen, action: "complete", status: complete.default_status, status_options_version: complete.status_options_version }),
      cancel: (actor, seen) => write(actor, `${root(issueId)}/lifecycle`, { ...seen, action: "cancel", status: cancel.default_status, status_options_version: cancel.status_options_version }),
      edit: (actor, seen) => send(actor, root(issueId), "PATCH", { ...seen, title: `Hijacked by ${actor.name}` }, { "Idempotency-Key": randomUUID() }),
    };
    const matrix: [string, Account, string[]][] = [
      ["handler", handler, ["remove", "complete", "cancel"]],
      ["peer", peer, ["invite", "transfer", "remove", "complete", "cancel", "edit"]],
      ["former handler", former, ["invite", "transfer", "remove", "complete", "cancel", "edit"]],
      ["source member", member, ["invite", "transfer", "remove", "complete", "cancel", "edit"]],
    ];
    const before = await businessState(creator, issueId);
    const results: string[] = [];
    for (const [role, actor, actions] of matrix) {
      for (const action of actions) {
        const outcome = await attempts[action]!(actor, observed(await readIssue(actor, issueId)));
        results.push(`${role} ${action} -> ${outcome.status} ${String(outcome.body.code)}`);
      }
    }
    expect(results).toEqual(matrix.flatMap(([role, , actions]) => actions.map((action) => `${role} ${action} -> 403 forbidden`)));
    expect(await businessState(creator, issueId)).toEqual(before);
    expect(before.participants.map((row) => row.user_id)).not.toContain(outsider.id);
    expect((await outsider.api.requestJSON<{ items: { id: string }[] }>("/api/me/workbench?view=participating")).items).toEqual([]);

    // The same payload shape succeeds for a role that holds it, so the
    // refusals above are role decisions, not malformed requests.
    const allowed = await attempts.invite!(handler, observed(await readIssue(handler, issueId)));
    expect(allowed.body).toMatchObject({ code: "participant_added" });
    const restored = await write(creator, `${root(issueId)}/participants/${outsider.id}/remove`, observed(await readIssue(creator, issueId)));
    expect(restored.body).toMatchObject({ code: "participant_removed" });
    expect((await readIssue(creator, issueId)).title).toBe(fixture.title);
    await pages.member!.page.screenshot({ path: test.info().outputPath("source-member-agent-refused.png"), animations: "disabled" });
  } finally {
    for (const side of Object.values(pages)) await side.context.close();
  }
});

type Activity = { items: { kind: string; actor: { user_id: string; name: string } | null; changes: { field: string; before: string | null; after: string | null }[] }[] };

async function editThroughForm(page: Page, issueId: string, fields: { title?: string; start?: string; due?: string }) {
  await actionsRow(page).getByRole("button", { name: "Edit issue", exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Edit issue", exact: true }) });
  if (fields.title !== undefined) await form.getByLabel("Title", { exact: true }).fill(fields.title);
  if (fields.start !== undefined) await form.getByLabel("Start date", { exact: true }).fill(fields.start);
  if (fields.due !== undefined) await form.getByLabel("Due date", { exact: true }).fill(fields.due);
  const saved = page.waitForResponse((r) => new URL(r.url()).pathname === root(issueId) && r.request().method() === "PATCH");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await saved;
  expect(response.status()).toBe(200);
  expect(await response.json()).toMatchObject({ code: "issue_edited" });
  await expect(form).toHaveCount(0);
  return response.request().postDataJSON() as Record<string, unknown>;
}

test("Smaller gaps: title/date edits leave attributed records, recipient lookup is exact-only, a source member resolves and reacts without notifications", async ({ browser }) => {
  test.setTimeout(240_000);
  requireLane();
  const fixture = await seedBoundary();
  const { creator, handler, peer, member, outsider, issueId } = fixture;
  const handlerSide = await openPage(browser, handler);
  const creatorSide = await openPage(browser, creator);
  const peerSide = await openPage(browser, peer);
  const memberSide = await openPage(browser, member);
  try {
    // The current handler edits title and both dates through "Edit issue"
    // (workflow.tsx edit form); the creator then moves the start and clears the due date.
    const newTitle = `${fixture.title} (handler edit)`;
    await openIssuePage(handlerSide.page, fixture, "account");
    const handlerEdit = await editThroughForm(handlerSide.page, issueId, { title: newTitle, start: "2026-11-05", due: "2026-11-27" });
    expect(handlerEdit).toMatchObject({ title: newTitle, start_date: "2026-11-05", due_date: "2026-11-27" });
    await expect(handlerSide.page.getByRole("heading", { name: newTitle, exact: true })).toBeVisible();
    await creatorSide.page.goto(`/${fixture.workspace.slug}/issues/${issueId}`);
    await expect(creatorSide.page.getByRole("heading", { name: newTitle, exact: true })).toBeVisible({ timeout: 30_000 });
    const creatorEdit = await editThroughForm(creatorSide.page, issueId, { start: "2026-11-09", due: "" });
    expect(creatorEdit).toMatchObject({ title: newTitle, start_date: "2026-11-09", due_date: null });
    expect(await readIssue(peer, issueId)).toMatchObject({ title: newTitle, start_date: "2026-11-09", due_date: null });

    // Each edit is one issue_edited record with the real actor and before/after
    // values (EditIssue + collaborationEditedValues, collaboration_edit.go:134-163;
    // projectCollaborationActivity, collaboration_activity.go:162-165).
    const activity = await peer.api.requestJSON<Activity>(`${root(issueId)}/activity`);
    const edits = activity.items.filter((item) => item.kind === "issue_edited");
    expect(edits).toEqual([
      expect.objectContaining({ actor: expect.objectContaining({ user_id: creator.id }), changes: [
        { field: "start_date", before: "2026-11-05", after: "2026-11-09" }, { field: "due_date", before: "2026-11-27", after: null }] }),
      expect.objectContaining({ actor: expect.objectContaining({ user_id: handler.id }), changes: [
        { field: "title", before: fixture.title, after: newTitle }, { field: "start_date", before: "2026-11-02", after: "2026-11-05" },
        { field: "due_date", before: "2026-11-20", after: "2026-11-27" }] }),
    ]);
    // Another participant sees the attributed record in the activity panel.
    await peerSide.page.goto(`/account?view=participating&issue=${issueId}`);
    await expect(peerSide.page.getByRole("heading", { name: newTitle, exact: true })).toBeVisible({ timeout: 30_000 });
    const record = peerSide.page.getByRole("region", { name: "Activity", exact: true }).locator("li")
      .filter({ hasText: "Issue edited" }).filter({ hasText: handler.name });
    await expect(record).toHaveCount(1);
    await expect(record).toContainText(fixture.title);
    await expect(record).toContainText(newTitle);

    // Exact recipient lookup (ResolveRecipient, collaboration_recipients.go:45-97):
    // a name fragment finds nothing, malformed or partial queries are rejected,
    // and even an exact match changes nothing until a manager confirms.
    const stateBeforeLookup = await businessState(creator, issueId);
    const inviteOps = async () => Number((await sql<{ count: string }>(
      "SELECT count(*)::text FROM collaboration_operation WHERE scope_id = $1 AND kind = 'invite_participant'", [issueId]))[0]!.count);
    const invitesBefore = await inviteOps();
    const lookup = (value: string) => creatorSide.page.waitForResponse((r) => new URL(r.url()).pathname === "/api/collaboration/recipients/resolve"
      && r.request().method() === "POST" && (r.request().postDataJSON() as { value: string }).value === value);
    await actionsRow(creatorSide.page).getByRole("button", { name: "Add participant", exact: true }).click();
    const invite = creatorSide.page.locator("form").filter({ has: creatorSide.page.getByRole("heading", { name: "Add participant", exact: true }) });
    const recipient = invite.getByLabel("Exact recipient", { exact: true });
    const find = invite.getByRole("button", { name: "Find account", exact: true });
    await recipient.fill("   ");
    await expect(find).toBeDisabled();
    // The unique fragment shared by the outsider's name and email is a valid
    // username shape, so it reaches the exact query and matches no account.
    const fragment = lookup(fixture.key);
    await recipient.fill(fixture.key);
    await find.click();
    const notFound = await fragment;
    expect(notFound.status()).toBe(404);
    expect(notFound.request().postDataJSON()).toEqual({ kind: "username", value: fixture.key, issue_id: issueId });
    expect(await notFound.json()).toEqual({ code: "recipient_not_found", error: "recipient_not_found" });
    await expect(invite.getByText("No matching account. Check the full username, email or account ID.", { exact: true })).toBeVisible();
    await expect(invite.getByRole("button", { name: "Confirm", exact: true })).toBeDisabled();

    const probe = async (kind: string, value: string) => {
      const response = await send(creator, "/api/collaboration/recipients/resolve", "POST", { kind, value, issue_id: issueId });
      return `${kind}:${JSON.stringify(value)} -> ${response.status} ${String(response.body.code)}`;
    };
    expect([
      await probe("username", ""),
      await probe("username", "%"),
      await probe("username", outsider.name),
      await probe("email", outsider.email.split("@")[0]!),
      await probe("email", "@example.invalid"),
      await probe("user_id", outsider.id.slice(0, 8)),
    ]).toEqual([
      'username:"" -> 400 invalid_recipient_query',
      'username:"%" -> 400 invalid_recipient_query',
      `username:${JSON.stringify(outsider.name)} -> 400 invalid_recipient_query`,
      `email:${JSON.stringify(outsider.email.split("@")[0]!)} -> 400 invalid_recipient_query`,
      'email:"@example.invalid" -> 400 invalid_recipient_query',
      `user_id:${JSON.stringify(outsider.id.slice(0, 8))} -> 400 invalid_recipient_query`,
    ]);

    // The exact email resolves to one confirmed account; closing does not invite it.
    const exact = lookup(outsider.email);
    await recipient.fill(outsider.email);
    await find.click();
    const found = await exact;
    expect(found.status()).toBe(200);
    expect(await found.json()).toEqual({ issue_id: issueId, source_workspace_id: fixture.workspace.id,
      recipient: expect.objectContaining({ user_id: outsider.id, name: outsider.name, external: true }) });
    await expect(invite.getByText(`Selected account: ${outsider.name}`, { exact: true })).toBeVisible();
    await expect(invite.getByRole("button", { name: "Confirm", exact: true })).toBeEnabled();
    await invite.getByRole("button", { name: "Close", exact: true }).click();
    await expect(invite).toHaveCount(0);
    expect(await businessState(creator, issueId)).toEqual(stateBeforeLookup);
    expect(await inviteOps()).toBe(invitesBefore);
    expect((await outsider.api.requestJSON<{ items: unknown[] }>("/api/me/workbench?view=participating")).items).toEqual([]);

    // A source member outside the issue resolves and reacts through the UI;
    // neither writes a collaboration notification or a source inbox row, nor
    // touches the comment (ChangeDiscussion, collaboration_discussion.go:60-144).
    const thread = await postComment(creator, issueId, `Thread to settle ${fixture.key}`);
    expect(thread.status).toBe(201);
    const threadId = String(thread.body.comment_id);
    const quiet = async () => (await sql<{ notifications: string; inbox: string }>(`SELECT
        (SELECT count(*) FROM collaboration_notification WHERE issue_id = $1)::text AS notifications,
        (SELECT count(*) FROM inbox_item WHERE issue_id = $1)::text AS inbox`, [issueId]))[0]!;
    const comment = () => creator.api.requestJSON<{ comment_revision: string; content: string; resolved_by: { user_id: string } | null;
      reactions: { emoji: string; count: number }[] }>(`${root(issueId)}/comments/${threadId}`);
    const settledQuiet = await quiet();
    const commentBefore = await comment();
    expect(commentBefore.resolved_by).toBeNull();
    const creatorUnread = (await creator.api.requestJSON<{ unread_count: number }>("/api/me/collaboration-notifications?limit=100")).unread_count;
    const commandsBefore = (await readIssue(creator, issueId)).execution?.commands.length ?? 0;

    await memberSide.page.goto(`/${fixture.workspace.slug}/issues/${issueId}`);
    await expect(memberSide.page.getByRole("heading", { name: newTitle, exact: true })).toBeVisible({ timeout: 30_000 });
    const row = memberSide.page.locator(`[id="comment-${threadId}"]`);
    await expect(row).toContainText(`Thread to settle ${fixture.key}`);
    const resolved = memberSide.page.waitForResponse((r) => new URL(r.url()).pathname === `${root(issueId)}/comments/${threadId}/resolve` && r.request().method() === "POST");
    await row.getByRole("button", { name: "Resolve thread", exact: true }).click();
    const resolution = await resolved;
    expect(resolution.status()).toBe(200);
    expect(await resolution.json()).toMatchObject({ action: "resolve", code: "comment_resolved", comment_id: threadId });
    await expect(row.getByText(`Resolved by ${member.name}`, { exact: true })).toBeVisible();
    const reacted = memberSide.page.waitForResponse((r) => new URL(r.url()).pathname.startsWith(`${root(issueId)}/comments/${threadId}/reactions/`) && r.request().method() === "POST");
    await row.getByRole("button", { name: "Choose a reaction", exact: true }).click();
    await memberSide.page.getByRole("button", { name: "👍", exact: true }).click();
    const reaction = await reacted;
    expect(reaction.status()).toBe(200);
    expect(await reaction.json()).toMatchObject({ action: "add_reaction", code: "reaction_added", emoji: "👍" });
    await expect(row.getByRole("button", { name: "Remove your 👍 reaction", exact: true })).toHaveAttribute("aria-pressed", "true");

    const commentAfter = await comment();
    expect(commentAfter).toMatchObject({ comment_revision: commentBefore.comment_revision, content: commentBefore.content,
      resolved_by: expect.objectContaining({ user_id: member.id }), reactions: [expect.objectContaining({ emoji: "👍", count: 1 })] });
    expect(await quiet()).toEqual(settledQuiet);
    expect((await creator.api.requestJSON<{ unread_count: number }>("/api/me/collaboration-notifications?limit=100")).unread_count).toBe(creatorUnread);
    expect((await readIssue(creator, issueId)).execution?.commands.length ?? 0).toBe(commandsBefore);
  } finally {
    for (const side of [handlerSide, creatorSide, peerSide, memberSide]) await side.context.close();
  }
});
