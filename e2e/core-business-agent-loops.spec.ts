import { randomUUID } from "node:crypto";
import { expect, type Page, type Response } from "@playwright/test";
import {
  API_BASE, authenticate, callJSON, createAgent, executionLog, openIssue, SimulatedDaemon, sql, test,
  type Actor, type InboxItem, type Issue,
} from "./fixtures/core-journeys";

// Agent loops that core-business-journeys.spec.ts (G1–G4) does not carry end to
// end: the chat reply (G5), comment-triggered runs (G6), agent-mode quick create
// reaching its inbox outcome (G7), and a running task's transcript and stop (G8).
// SimulatedDaemon speaks the real daemon HTTP protocol; no agent CLI or model
// runs, and every outcome is checked in the browser and on the server.
type ChatMessage = { id: string; role: string; content: string; task_id: string | null; failure_reason: string | null };
type QueueRow = { id: string; agent_id: string; status: string; trigger_comment_id: string | null };
type Comment = { id: string; content: string; parent_id: string | null; trigger_outcomes?: unknown[] };
type InboxDetail = InboxItem & { body: string | null; details: Record<string, string> | null };

test.use({ locale: "en-US", viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure", screenshot: "only-on-failure" });

const isPost = (response: Response, path: string | RegExp) => response.request().method() === "POST" &&
  (typeof path === "string" ? new URL(response.url()).pathname === path : path.test(new URL(response.url()).pathname));

const queueRows = (where: string, value: string) => sql<QueueRow>(
  `SELECT id::text, agent_id::text, status, trigger_comment_id::text FROM agent_task_queue WHERE ${where} = $1 ORDER BY created_at, id`, [value]);
const agentComments = (issueId: string) => sql<{ id: string; author_id: string; content: string; parent_id: string | null }>(
  "SELECT id::text, author_id::text, content, parent_id::text FROM comment WHERE issue_id = $1 AND author_type = 'agent' ORDER BY created_at, id", [issueId]);
const issueStatus = async (actor: Actor, issueId: string) => (await callJSON<Issue>(actor, `/api/issues/${issueId}`)).status;

// Chat composer (same selectors as chat-recovery-branches.spec.ts).
const chatSurface = (page: Page) => page.locator('[data-slot="chat-input-surface"]');
const chatEditor = (page: Page) => chatSurface(page).locator('[contenteditable="true"]');
const chatSend = (page: Page) => chatSurface(page).getByRole("button", { name: "Send", exact: true });
// The transcript has no landmark of its own; this is react-virtuoso's item list.
// Scoping to it keeps the session list's last-message preview out of matches.
const chatTranscript = (page: Page) => page.getByTestId("virtuoso-item-list");

// The issue's top-level comment composer. Thread replies use ReplyInput, which
// has no bg-card shell, so this never matches a reply box.
const composer = (page: Page) => page.locator("div.rounded-lg.bg-card")
  .filter({ has: page.getByRole("button", { name: "Send", exact: true }) });
const composerEditor = (page: Page) => composer(page).locator('.ProseMirror[contenteditable="true"]');
const triggerChip = (page: Page, agentName: string, state: string) =>
  composer(page).getByRole("button", { name: `${agentName} trigger: ${state}`, exact: true });

/** Mount the lazy composer and wait until Tiptap is live before typing. */
async function openComposer(page: Page) {
  await page.getByTestId("comment-composer-shell").click();
  await expect(page.getByTestId("comment-composer-shell")).toHaveCount(0);
  await expect(composerEditor(page)).toBeFocused();
  // Tiptap emits "create" in a setTimeout(0) after mount; let it pass.
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
}
/** Send through Ctrl/Cmd+Enter and return the created comment (201). */
async function submitComment(page: Page, issueId: string): Promise<Comment> {
  const posted = page.waitForResponse((response) => isPost(response, `/api/issues/${issueId}/comments`));
  await composerEditor(page).press("ControlOrMeta+Enter");
  const response = await posted;
  expect(response.status()).toBe(201);
  await expect(composerEditor(page)).toHaveText("");
  return response.json() as Promise<Comment>;
}

/** One transcript event with an explicit seq (SimulatedDaemon.messages always sends seq 1). */
async function streamTaskMessage(actor: Actor, taskId: string, seq: number, content: string) {
  const response = await fetch(`${API_BASE}/api/daemon/tasks/${taskId}/messages`, {
    method: "POST",
    headers: { Authorization: `Bearer ${actor.api.getToken()}`, "Content-Type": "application/json" },
    body: JSON.stringify({ messages: [{ seq, type: "text", content }] }),
  });
  expect(response.ok, await response.clone().text()).toBe(true);
}

/** New Issue in agent mode → Send to <agent> → Create; returns the 202 body. */
async function quickCreate(page: Page, agent: { id: string; name: string }, prompt: string) {
  await page.getByRole("button", { name: "New Issue", exact: true }).first().click();
  const dialog = page.getByRole("dialog", { name: "Quick create issue", exact: true });
  const editor = dialog.locator('[contenteditable="true"]').first();
  await expect(editor).toBeVisible();
  await editor.click();
  await expect(editor).toBeFocused();
  await editor.fill(prompt);
  await expect(editor).toHaveText(prompt);
  await dialog.getByRole("button", { name: /^Send to/ }).click();
  await page.getByPlaceholder("Search names or responsibilities...", { exact: true }).fill(agent.name);
  await page.locator(`[data-actor-key="agent:${agent.id}"] button[data-picker-item]`).click();
  await expect(dialog.getByRole("button", { name: /^Send to/ })).toContainText(agent.name);
  const accepted = page.waitForResponse((response) => isPost(response, "/api/issues/quick-create"));
  await dialog.getByRole("button", { name: /^Create(?:\s|$)/ }).last().click();
  const response = await accepted;
  expect(response.status()).toBe(202);
  expect(response.request().postDataJSON()).toMatchObject({ agent_id: agent.id, prompt });
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText(/^Sent to agent/).first()).toBeVisible();
  return response.json() as Promise<{ task_id: string }>;
}
test("G5 a chat message runs on the daemon and its reply, then its failure, appear without reload", async ({ page, owner }) => {
  const daemon = new SimulatedDaemon(owner);
  await daemon.register();
  const agent = await createAgent(owner, daemon.runtimeId);
  await authenticate(page, owner);
  await page.goto(`/${owner.workspace.slug}/chat`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "New chat", exact: true }).click();
  // A one-agent workspace starts the chat directly with that agent.
  await expect(page.getByRole("heading", { name: `Chat with ${agent.name}`, exact: true })).toBeVisible({ timeout: 30_000 });

  const question = `Summarise the release risks ${randomUUID().slice(0, 8)}`;
  await chatEditor(page).fill(question);
  await expect(chatSend(page)).toBeEnabled();
  const createdSession = page.waitForResponse((response) => isPost(response, "/api/chat/sessions"));
  const sent = page.waitForResponse((response) => isPost(response, /^\/api\/chat\/sessions\/[^/]+\/messages$/));
  await chatSend(page).click();
  expect((await createdSession).status()).toBe(201);
  const sendResponse = await sent;
  expect(sendResponse.status()).toBe(201);
  const first = await sendResponse.json() as { message_id: string; task_id: string };
  const sessionId = new URL(sendResponse.url()).pathname.split("/")[4]!;
  await expect(page).toHaveURL(new RegExp(`session=${sessionId}`));
  await expect(chatEditor(page)).toHaveText("");

  // Exactly one queued chat task, shown as the pending turn's "Queued" pill.
  expect(await queueRows("chat_session_id", sessionId)).toEqual([
    expect.objectContaining({ id: first.task_id, agent_id: agent.id, status: "queued" })]);
  await expect(page.getByText("Queued", { exact: true })).toBeVisible();

  const claimed = await daemon.claim();
  expect(claimed).toMatchObject({ id: first.task_id, chat_session_id: sessionId });
  await daemon.start(first.task_id);
  const reply = `Three risks remain open ${randomUUID().slice(0, 8)}`;
  await daemon.complete(first.task_id, reply);

  // chat:done delivers the reply to the open page; nothing is reloaded.
  await expect(chatTranscript(page).getByText(reply, { exact: true })).toBeVisible();
  await expect(page.getByText("Queued", { exact: true })).toHaveCount(0);
  const messages = () => callJSON<ChatMessage[]>(owner, `/api/chat/sessions/${sessionId}/messages`);
  expect((await messages()).map(({ role, content, task_id, failure_reason }) => ({ role, content, task_id, failure_reason }))).toEqual([
    { role: "user", content: question, task_id: first.task_id, failure_reason: null },
    { role: "assistant", content: reply, task_id: first.task_id, failure_reason: null },
  ]);
  expect((await queueRows("chat_session_id", sessionId)).map((row) => row.status)).toEqual(["completed"]);
  expect(await daemon.claim()).toBeNull();
  await page.reload();
  await expect(chatTranscript(page).getByText(reply, { exact: true })).toBeVisible({ timeout: 30_000 });

  // Failure: the next turn fails with a non-retryable reason. The transcript
  // gains one assistant failure row and no automatic retry is queued.
  const followUp = `And the rollback plan ${randomUUID().slice(0, 8)}`;
  await chatEditor(page).fill(followUp);
  await expect(chatSend(page)).toBeEnabled();
  const sentAgain = page.waitForResponse((response) => isPost(response, `/api/chat/sessions/${sessionId}/messages`));
  await chatSend(page).click();
  const second = await (await sentAgain).json() as { task_id: string };
  expect(second.task_id).not.toBe(first.task_id);
  expect((await daemon.claim())?.id).toBe(second.task_id);
  await daemon.start(second.task_id);
  const rawError = `Simulated chat runtime error ${randomUUID().slice(0, 8)}`;
  await daemon.fail(second.task_id, rawError);

  // The bubble explains the failure in chat copy; the raw error waits behind "Show details".
  const transcript = chatTranscript(page);
  await expect(transcript.getByText("The agent ran into an error and couldn't finish replying. Please try again.", { exact: true })).toBeVisible();
  await expect(transcript.getByText(rawError, { exact: true })).toBeHidden();
  await transcript.getByRole("button", { name: "Show details", exact: true }).click();
  await expect(transcript.getByText(rawError, { exact: true })).toBeVisible();
  expect((await messages()).map(({ role, task_id, failure_reason }) => ({ role, task_id, failure_reason }))).toEqual([
    { role: "user", task_id: first.task_id, failure_reason: null },
    { role: "assistant", task_id: first.task_id, failure_reason: null },
    { role: "user", task_id: second.task_id, failure_reason: null },
    { role: "assistant", task_id: second.task_id, failure_reason: "agent_error" },
  ]);
  expect((await messages())[3]!.content).toBe(rawError);
  expect((await queueRows("chat_session_id", sessionId)).map((row) => row.status)).toEqual(["completed", "failed"]);
  expect(await daemon.claim()).toBeNull();
});
test("G6 a plain comment re-engages the assignee, an @mention engages another agent and /note engages none", async ({ page, owner }) => {
  const daemon = new SimulatedDaemon(owner);
  await daemon.register();
  const assignee = await createAgent(owner, daemon.runtimeId);
  const mentioned = await createAgent(owner, daemon.runtimeId);
  const issue = await owner.api.createIssue(`Core comment loop ${randomUUID().slice(0, 6)}`, {
    status: "todo", assignee_type: "agent", assignee_id: assignee.id,
  }) as Issue;
  // Drain the assignment run so each comment below meets an idle agent.
  await expect.poll(async () => (await queueRows("issue_id", issue.id)).map((row) => row.status)).toEqual(["queued"]);
  const initial = await daemon.claim();
  await daemon.start(initial!.id);
  const firstReply = `Initial analysis posted ${randomUUID().slice(0, 8)}`;
  await daemon.complete(initial!.id, firstReply);
  await authenticate(page, owner);
  await openIssue(page, owner.workspace, issue);
  await expect(page.getByText(firstReply, { exact: true })).toBeVisible();

  // 1. A plain top-level comment: the composer previews the assignee, the
  // response carries no explicit-mention outcomes, and the server queues one
  // run for the assignee bound to this comment.
  await openComposer(page);
  const plainText = `Please also cover the migration path ${randomUUID().slice(0, 6)}`;
  await composerEditor(page).fill(plainText);
  await expect(triggerChip(page, assignee.name, "assignee")).toBeVisible();
  const plain = await submitComment(page, issue.id);
  expect(plain.content).toBe(plainText);
  expect(plain.trigger_outcomes ?? []).toEqual([]);
  await expect.poll(async () => (await queueRows("issue_id", issue.id)).map(({ agent_id, status, trigger_comment_id }) => ({ agent_id, status, trigger_comment_id })))
    .toEqual([
      { agent_id: assignee.id, status: "completed", trigger_comment_id: null },
      { agent_id: assignee.id, status: "queued", trigger_comment_id: plain.id },
    ]);
  const followUp = await daemon.claim();
  expect(followUp).toMatchObject({ trigger_comment_id: plain.id, trigger_comment_content: plainText });
  await daemon.start(followUp!.id);
  const secondReply = `Migration path covered in section 3 ${randomUUID().slice(0, 8)}`;
  await daemon.complete(followUp!.id, secondReply);
  // The synthesized reply is threaded under the comment that asked for it.
  await expect.poll(async () => (await agentComments(issue.id)).length).toBe(2);
  expect(await agentComments(issue.id)).toEqual([
    expect.objectContaining({ author_id: assignee.id, content: firstReply, parent_id: null }),
    expect.objectContaining({ author_id: assignee.id, content: secondReply, parent_id: plain.id }),
  ]);
  const threadedReply = (await agentComments(issue.id))[1]!;
  await expect(page.locator(`[id="comment-${threadedReply.id}"]`)).toContainText(secondReply);
  expect(await daemon.claim()).toBeNull();

  // 2. An explicit @mention of an unassigned agent: only that agent is
  // previewed and queued, and the response reports it as queued.
  await composerEditor(page).click();
  await expect(composerEditor(page)).toBeFocused();
  await composerEditor(page).fill("Second opinion please");
  await composerEditor(page).pressSequentially(` @${mentioned.name}`);
  const escaped = mentioned.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await page.getByRole("button", { name: new RegExp(`${escaped}.*Agent$`) }).click();
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  await expect(composerEditor(page)).toBeFocused();
  await expect(triggerChip(page, mentioned.name, "@mention")).toBeVisible();
  await expect(triggerChip(page, assignee.name, "assignee")).toHaveCount(0);
  const mention = await submitComment(page, issue.id);
  expect(mention.content).toContain(`mention://agent/${mentioned.id}`);
  expect(mention.trigger_outcomes).toEqual([{ target_type: "agent", target_id: mentioned.id, status: "queued", reason_code: "queued" }]);
  await expect.poll(async () => (await queueRows("issue_id", issue.id)).slice(2).map(({ agent_id, status, trigger_comment_id }) => ({ agent_id, status, trigger_comment_id })))
    .toEqual([{ agent_id: mentioned.id, status: "queued", trigger_comment_id: mention.id }]);
  const mentionRun = await daemon.claim();
  expect(mentionRun).toMatchObject({ trigger_comment_id: mention.id });
  await daemon.start(mentionRun!.id);
  const thirdReply = `Rollback plan reviewed by a second agent ${randomUUID().slice(0, 8)}`;
  await daemon.complete(mentionRun!.id, thirdReply);
  await expect.poll(async () => (await agentComments(issue.id)).length).toBe(3);
  const mentionReply = (await agentComments(issue.id))[2]!;
  expect(mentionReply).toMatchObject({ author_id: mentioned.id, content: thirdReply, parent_id: mention.id });
  await expect(page.locator(`[id="comment-${mentionReply.id}"]`)).toContainText(thirdReply);
  expect(await daemon.claim()).toBeNull();

  // 3. /note picked from the command menu: nothing is previewed or queued.
  await composerEditor(page).click();
  await expect(composerEditor(page)).toBeFocused();
  await composerEditor(page).pressSequentially("/no");
  await page.getByRole("button", { name: /^\/note/ }).click();
  const noteBody = `internal reminder ${randomUUID().slice(0, 6)}`;
  await composerEditor(page).pressSequentially(noteBody);
  await expect(composerEditor(page)).toHaveText(`/note ${noteBody}`);
  // A /note draft has an "empty" preview signature, so the client asks the
  // server nothing and shows no chip (use-comment-trigger-preview.ts:20-25).
  await expect(composer(page).getByRole("button", { name: /trigger:/ })).toHaveCount(0);
  const note = await submitComment(page, issue.id);
  expect(note.content).toBe(`/note ${noteBody}`);
  expect(note.trigger_outcomes ?? []).toEqual([]);
  expect((await queueRows("issue_id", issue.id)).length).toBe(3);
  expect(await daemon.claim()).toBeNull();
  // No run in this test changed the issue status: only the agent's own CLI calls would.
  expect(await issueStatus(owner, issue.id)).toBe("todo");
});
test("G7 an agent-mode quick create files the issue and notifies the requester; a run that files nothing reports why", async ({ page, owner }) => {
  const daemon = new SimulatedDaemon(owner);
  await daemon.register();
  const agent = await createAgent(owner, daemon.runtimeId);
  await authenticate(page, owner);
  await page.addInitScript(() => {
    localStorage.setItem("multica_create_mode", JSON.stringify({ state: { lastMode: "agent" }, version: 0 }));
  });
  await page.goto(`/${owner.workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
  const issueCount = async () => Number((await sql<{ n: string }>(
    "SELECT count(*)::text AS n FROM issue WHERE workspace_id = $1", [owner.workspace.id]))[0]!.n);
  const issuesBefore = await issueCount();

  const prompt = `File a bug: CSV export drops the header row ${randomUUID().slice(0, 6)}`;
  const { task_id: taskId } = await quickCreate(page, agent, prompt);
  const [queued] = await sql<{ status: string; issue_id: string | null; type: string | null }>(
    "SELECT status, issue_id::text, context->>'type' AS type FROM agent_task_queue WHERE id = $1", [taskId]);
  expect(queued).toEqual({ status: "queued", issue_id: null, type: "quick_create" });

  const claimed = await daemon.claim();
  expect(claimed).toMatchObject({ id: taskId, kind: "quick_create", quick_create_prompt: prompt });
  const taskToken = String(claimed!.auth_token ?? "");
  expect(taskToken.startsWith("mat_")).toBe(true);
  await daemon.start(taskId);

  // The agent files the issue the way the CLI does inside a quick-create run:
  // with its task token and the origin stamp (cmd_issue.go MULTICA_QUICK_CREATE_TASK_ID).
  const title = `CSV export drops the header row ${randomUUID().slice(0, 6)}`;
  const filedResponse = await fetch(`${API_BASE}/api/issues`, {
    method: "POST",
    headers: { Authorization: `Bearer ${taskToken}`, "Content-Type": "application/json" },
    body: JSON.stringify({ title, status: "todo", origin_type: "quick_create", origin_id: taskId }),
  });
  expect(filedResponse.status, await filedResponse.clone().text()).toBe(201);
  const filed = await filedResponse.json() as Issue & { creator_type: string; creator_id: string };
  expect(filed).toMatchObject({ title, creator_type: "agent", creator_id: agent.id });
  // The open issue list learns about it over the realtime channel.
  await expect(page.getByText(title, { exact: true }).first()).toBeVisible();

  // With the inbox already open, completing the run lands the outcome live.
  const inboxLoaded = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/inbox" && response.ok());
  await page.goto(`/${owner.workspace.slug}/inbox`, { waitUntil: "domcontentloaded" });
  await inboxLoaded;
  await daemon.complete(taskId, `Created ${filed.identifier}`);
  const inbox = () => callJSON<InboxDetail[]>(owner, "/api/inbox");
  await expect.poll(async () => (await inbox()).filter((item) => item.type === "quick_create_done").length).toBe(1);
  const done = (await inbox()).find((item) => item.type === "quick_create_done")!;
  expect(done).toMatchObject({ issue_id: filed.id, title, read: false });
  expect(done.details).toMatchObject({ task_id: taskId, issue_id: filed.id, identifier: filed.identifier, original_prompt: prompt });
  const doneRow = page.getByRole("listitem").filter({ has: page.getByRole("button", { name: title, exact: true }) });
  await expect(doneRow).toHaveCount(1);
  await expect(doneRow).toContainText(`Created with agent: ${filed.identifier}`);
  // Completion links the run to the issue it produced; the run itself writes no comment there.
  expect((await sql<{ status: string; issue_id: string }>("SELECT status, issue_id::text FROM agent_task_queue WHERE id = $1", [taskId]))[0])
    .toEqual({ status: "completed", issue_id: filed.id });
  expect(await agentComments(filed.id)).toEqual([]);

  // Failure: the run completes without filing anything. Its output is the CLI
  // error, which becomes the reason on a failed inbox row with no issue.
  const duplicatePrompt = `File it again: CSV export drops the header row ${randomUUID().slice(0, 6)}`;
  const { task_id: failedTaskId } = await quickCreate(page, agent, duplicatePrompt);
  expect((await daemon.claim())?.id).toBe(failedTaskId);
  await daemon.start(failedTaskId);
  const cliError = `An active issue with this title already exists: ${filed.identifier}`;
  await daemon.complete(failedTaskId, cliError);
  await expect.poll(async () => (await inbox()).filter((item) => item.type === "quick_create_failed").length).toBe(1);
  const failed = (await inbox()).find((item) => item.type === "quick_create_failed")!;
  expect(failed).toMatchObject({ issue_id: null, title: "Quick create failed", body: cliError });
  expect(failed.details).toMatchObject({ task_id: failedTaskId, error: cliError, original_prompt: duplicatePrompt });
  const failedRow = page.getByRole("listitem").filter({ has: page.getByRole("button", { name: duplicatePrompt, exact: true }) });
  await expect(failedRow).toHaveCount(1);
  await expect(failedRow).toContainText(`Failed: ${cliError}`);
  expect(await sql("SELECT id FROM issue WHERE origin_type = 'quick_create' AND origin_id = $1", [failedTaskId])).toEqual([]);
  expect(await issueCount()).toBe(issuesBefore + 1);
  expect(await daemon.claim()).toBeNull();
});
test("G8 a running task streams its transcript, leaves the issue status alone and stays stopped after a late completion", async ({ page, owner }) => {
  const daemon = new SimulatedDaemon(owner);
  await daemon.register();
  const agent = await createAgent(owner, daemon.runtimeId);
  const issue = await owner.api.createIssue(`Core running stop ${randomUUID().slice(0, 6)}`, {
    status: "todo", assignee_type: "agent", assignee_id: agent.id,
  }) as Issue;
  await expect.poll(async () => (await queueRows("issue_id", issue.id)).map((row) => row.status)).toEqual(["queued"]);
  await authenticate(page, owner);
  await openIssue(page, owner.workspace, issue);
  const log = executionLog(page);
  const claimed = await daemon.claim();
  await daemon.start(claimed!.id);
  const running = log.locator("div.group\\/execution-log-row").filter({ has: page.locator(".sr-only", { hasText: "Working" }) });
  await expect(running).toHaveCount(1);

  // Transcript: persisted per task and shown live in the running row's dialog.
  const firstLine = `Reading the issue and its comments ${randomUUID().slice(0, 6)}`;
  await streamTaskMessage(owner, claimed!.id, 1, firstLine);
  // Progress is accepted, but ReportProgress only broadcasts task:progress
  // (task.go:6471) and no client consumes it, so it never reaches the transcript.
  await daemon.progress(claimed!.id, "Reading the issue");
  const transcript = () => callJSON<Array<{ seq: number; type: string; content: string }>>(owner, `/api/tasks/${claimed!.id}/messages`);
  expect((await transcript()).map(({ seq, type, content }) => ({ seq, type, content }))).toEqual([{ seq: 1, type: "text", content: firstLine }]);
  await running.hover();
  await running.getByRole("button", { name: "View transcript", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Agent Execution Transcript", exact: true });
  await expect(dialog.getByText(firstLine)).toBeVisible();
  const secondLine = `Drafting the change ${randomUUID().slice(0, 6)}`;
  await streamTaskMessage(owner, claimed!.id, 2, `\n\n${secondLine}`);
  await expect(dialog.getByText(secondLine)).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);

  // Starting a run does not move the issue: status changes come only from the agent's CLI.
  expect(await issueStatus(owner, issue.id)).toBe("todo");
  await expect(page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();

  // Stop the running task from the row, behind its confirmation.
  await running.hover();
  await running.getByRole("button", { name: "Cancel task", exact: true }).click();
  const confirm = page.getByRole("alertdialog", { name: "Stop this task?", exact: true });
  await expect(confirm).toContainText("Running work may take a few seconds to fully halt.");
  const cancelled = page.waitForResponse((response) => isPost(response, `/api/issues/${issue.id}/tasks/${claimed!.id}/cancel`));
  await confirm.getByRole("button", { name: "Stop task", exact: true }).click();
  expect((await cancelled).ok()).toBe(true);
  await expect.poll(async () => (await queueRows("issue_id", issue.id)).map((row) => row.status)).toEqual(["cancelled"]);
  await expect(running).toHaveCount(0);
  // Cancelling revokes the run's task token, so the stopped agent can no longer call the API (task.go:860-869).
  const revoked = await fetch(`${API_BASE}/api/issues/${issue.id}`, { headers: { Authorization: `Bearer ${String(claimed!.auth_token ?? "")}` } });
  expect(String(claimed!.auth_token ?? "").startsWith("mat_")).toBe(true);
  expect(revoked.status).toBe(401);

  // The daemon's late completion is accepted idempotently and changes nothing:
  // CompleteAgentTask only matches status='running' (agent.sql:1179) and
  // CompleteTask returns the existing row before any comment fallback (task.go:4441-4453).
  const late = await daemon.complete(claimed!.id, `Late output after stop ${randomUUID().slice(0, 6)}`) as { status: string };
  expect(late.status).toBe("cancelled");
  expect((await queueRows("issue_id", issue.id)).map((row) => row.status)).toEqual(["cancelled"]);
  expect(await agentComments(issue.id)).toEqual([]);
  expect(await issueStatus(owner, issue.id)).toBe("todo");
  // A stopped run never executed the issue, so the first-execution mark stays unset.
  expect(await sql<{ first_executed_at: Date | null }>("SELECT first_executed_at FROM issue WHERE id = $1", [issue.id]))
    .toEqual([{ first_executed_at: null }]);
  expect(await daemon.claim()).toBeNull();
  await page.reload();
  await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible({ timeout: 30_000 });
  await log.getByRole("button", { name: "Show past runs (1)", exact: true }).click();
  await expect(log.locator(".sr-only", { hasText: "Cancelled" })).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();
});
