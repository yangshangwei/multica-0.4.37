import { randomUUID } from "node:crypto";
import { expect, test as base, type Page, type Response } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

// Browser + real API contracts. Only the named failure/latency request is
// intercepted; retries, uploads, messages and their persisted rows are real.
// The synthetic runtime has no daemon process, credentials or model execution.
type Session = { id: string; title: string };
type Message = { id: string; role: string; content: string; attachments?: { id: string }[] };
type Fixture = {
  api: TestApiClient;
  slug: string;
  agentId: string;
  createSession: (title: string) => Promise<Session>;
};

const test = base.extend<{ chat: Fixture }>({
  chat: async ({ page, baseURL }, use) => {
    const api = new TestApiClient();
    const slug = `chat-recovery-${randomUUID().slice(0, 8)}`;
    const username = slug.replaceAll("-", "");
    const passwordAuth = process.env.E2E_PASSWORD_AUTH === "1";
    let workspaceId: string | undefined;
    try {
      if (passwordAuth) {
        const user = await api.registerPassword(username, "Chat recovery password 2026!", "Chat recovery verifier");
        const db = new pg.Client(process.env.DATABASE_URL);
        await db.connect();
        try {
          await db.query('UPDATE "user" SET onboarded_at = now() WHERE id = $1', [user.id]);
        } finally {
          await db.end();
        }
      } else {
        await api.login(`${slug}@multica.ai`, "Chat recovery verifier");
        await api.markUserOnboarded();
      }
      const workspace = await api.ensureWorkspace("Chat recovery verification", slug);
      workspaceId = workspace.id;
      expect(workspace.slug).toBe(slug);
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
      const runtime = await api.seedProjectRuntime();
      const agent = await api.requestJSON<{ id: string }>("/api/agents", {
        method: "POST",
        body: { name: "Recovery test agent", runtime_id: runtime.id, permission_mode: "private" },
      });
      const token = api.getToken();
      if (!token || !baseURL) throw new Error("Incomplete chat browser fixture");
      await page.context().addCookies([
        { name: "multica_logged_in", value: "1", url: baseURL },
        { name: "multica-locale", value: "en", url: baseURL },
      ]);
      await page.addInitScript((authToken) => {
        localStorage.setItem("multica_token", authToken);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, token);
      await page.setViewportSize({ width: 1440, height: 960 });
      await use({
        api,
        slug,
        agentId: agent.id,
        createSession: (title) => api.requestJSON<Session>("/api/chat/sessions", {
          method: "POST", body: { agent_id: agent.id, title },
        }),
      });
    } finally {
      if (workspaceId) await api.deleteFeatureWorkspace(workspaceId);
      if (passwordAuth) await api.deletePasswordAccount(username);
    }
  },
});

const surface = (page: Page) => page.locator('[data-slot="chat-input-surface"]');
const editor = (page: Page) => surface(page).locator('[contenteditable="true"]');
const send = (page: Page) => surface(page).getByRole("button", { name: "Send", exact: true });
const messagePath = (sessionId: string) => `/api/chat/sessions/${sessionId}/messages`;
const isPost = (response: Response, path: string) =>
  response.request().method() === "POST" && new URL(response.url()).pathname === path;

async function openSession(page: Page, chat: Fixture, session: Session) {
  await page.goto(`/${chat.slug}/chat?session=${session.id}`, { waitUntil: "domcontentloaded" });
  await expect(editor(page)).toBeVisible();
}

async function messages(chat: Fixture, session: Session) {
  return chat.api.requestJSON<Message[]>(messagePath(session.id));
}

async function expectOnlyMessage(chat: Fixture, session: Session, content: string) {
  await expect.poll(async () => (await messages(chat, session)).filter((row) => row.role === "user").map((row) => row.content))
    .toEqual([content]);
}

async function rejectNextPost(page: Page, path: string) {
  const matches = (url: URL) => url.pathname === path;
  await page.route(matches, async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    await route.fulfill({ status: 503, json: { error: "Injected chat recovery failure" } });
    await page.unroute(matches);
  });
}

async function holdPosts(page: Page, path: string) {
  let release!: () => void;
  let entered!: () => void;
  const gate = new Promise<void>((resolve) => { release = resolve; });
  const started = new Promise<void>((resolve) => { entered = resolve; });
  let count = 0;
  await page.route((url) => url.pathname === path, async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    count += 1;
    entered();
    await gate;
    await route.continue();
  });
  return { release, started, count: () => count };
}

test("upload rejection preserves text and reattaching binds exactly one file on send [fault injection]", async ({ page, chat }) => {
  const session = await chat.createSession("Upload retry session");
  await openSession(page, chat, session);
  const text = "Keep this explanation while the attachment retries.";
  await editor(page).fill(text);
  const file = { name: "recovery-note.txt", mimeType: "text/plain", buffer: Buffer.from("Synthetic attachment recovery fixture") };
  await rejectNextPost(page, "/api/upload-file");
  const failedUpload = page.waitForResponse((response) => isPost(response, "/api/upload-file") && response.status() === 503);
  await surface(page).locator('input[type="file"]').setInputFiles(file);
  await failedUpload;
  await expect(page.getByText(/Couldn't upload recovery-note\.txt:/)).toBeVisible();
  await expect(editor(page)).toHaveText(text);
  await expect(send(page)).toBeEnabled();
  expect(await messages(chat, session)).toEqual([]);

  const uploadedResponse = page.waitForResponse((response) => isPost(response, "/api/upload-file") && response.status() === 200);
  await surface(page).locator('input[type="file"]').setInputFiles(file);
  const uploaded = await (await uploadedResponse).json() as { id: string };
  await expect(editor(page)).toContainText(file.name);
  const sentResponse = page.waitForResponse((response) => isPost(response, messagePath(session.id)));
  await send(page).click();
  const response = await sentResponse;
  expect(response.status()).toBe(201);
  expect(response.request().postDataJSON().attachment_ids).toEqual([uploaded.id]);
  await expect(editor(page)).toHaveText("");
  const saved = (await messages(chat, session)).filter((row) => row.role === "user");
  expect(saved).toHaveLength(1);
  expect(saved[0].content).toContain(text);
  expect(saved[0].attachments?.map((attachment) => attachment.id)).toEqual([uploaded.id]);
});

test("removing an uploaded attachment sends only the remaining text [real API]", async ({ page, chat }) => {
  const session = await chat.createSession("Remove attachment session");
  await openSession(page, chat, session);
  await editor(page).fill("Initial attachment draft");
  const uploadedResponse = page.waitForResponse((response) => isPost(response, "/api/upload-file") && response.status() === 200);
  await surface(page).locator('input[type="file"]').setInputFiles({
    name: "remove-me.txt", mimeType: "text/plain", buffer: Buffer.from("Remove this synthetic attachment"),
  });
  const uploaded = await (await uploadedResponse).json() as { id: string };
  await expect(editor(page)).toContainText("remove-me.txt");
  const text = "Send this text without the removed file.";
  await editor(page).fill(text);
  await expect(editor(page)).not.toContainText("remove-me.txt");
  const sentResponse = page.waitForResponse((response) => isPost(response, messagePath(session.id)));
  await send(page).click();
  const response = await sentResponse;
  expect(response.status()).toBe(201);
  expect(response.request().postDataJSON().attachment_ids ?? []).not.toContain(uploaded.id);
  await expectOnlyMessage(chat, session, text);
  expect((await messages(chat, session))[0].attachments ?? []).toEqual([]);
  await page.reload();
  await expect(editor(page)).toHaveText("");
});

test("session creation rejection keeps the new draft and retries without empty navigation [fault injection]", async ({ page, chat }) => {
  await page.goto(`/${chat.slug}/chat`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "New chat", exact: true }).click();
  // A one-agent workspace starts directly without mounting the picker.
  await expect(page.getByRole("heading", { name: "Chat with Recovery test agent", exact: true })).toBeVisible();
  const text = "Create this conversation only after the server accepts it.";
  await editor(page).fill(text);
  await rejectNextPost(page, "/api/chat/sessions");
  const failure = page.waitForResponse((response) => isPost(response, "/api/chat/sessions") && response.status() === 503);
  await send(page).click();
  await failure;
  await expect(page.getByText("Failed to send message", { exact: true })).toBeVisible();
  await expect(editor(page)).toHaveText(text);
  expect(new URL(page.url()).searchParams.has("session")).toBe(false);
  expect(await chat.api.requestJSON<Session[]>("/api/chat/sessions")).toEqual([]);

  const sentResponse = page.waitForResponse((response) => response.request().method() === "POST" && /\/api\/chat\/sessions\/[^/]+\/messages$/.test(new URL(response.url()).pathname));
  await send(page).click();
  expect((await sentResponse).status()).toBe(201);
  await expect(editor(page)).toHaveText("");
  const sessions = await chat.api.requestJSON<Session[]>("/api/chat/sessions");
  expect(sessions).toHaveLength(1);
  await expect(page).toHaveURL(new RegExp(`session=${sessions[0].id}`));
  await expectOnlyMessage(chat, sessions[0], text);
});

test("message rejection retains the draft and retry persists no duplicate [fault injection]", async ({ page, chat }) => {
  const session = await chat.createSession("Message retry session");
  await openSession(page, chat, session);
  const text = "Retry this message once after the connection recovers.";
  await editor(page).fill(text);
  await rejectNextPost(page, messagePath(session.id));
  const failed = page.waitForResponse((response) => isPost(response, messagePath(session.id)) && response.status() === 503);
  await send(page).click();
  await failed;
  await expect(page.getByText("Failed to send message", { exact: true })).toBeVisible();
  await expect(editor(page)).toHaveText(text);
  await expect(send(page)).toBeEnabled();
  expect(await messages(chat, session)).toEqual([]);
  const accepted = page.waitForResponse((response) => isPost(response, messagePath(session.id)));
  await send(page).click();
  expect((await accepted).status()).toBe(201);
  await expect(editor(page)).toHaveText("");
  await expectOnlyMessage(chat, session, text);
  await page.reload();
  await expect(editor(page)).toHaveText("");
  await expectOnlyMessage(chat, session, text);
});

test("repeated keyboard submit during a pending send creates only one message [latency injection]", async ({ page, chat }) => {
  const session = await chat.createSession("Single flight session");
  await openSession(page, chat, session);
  const text = "One user action must create one durable message.";
  await editor(page).fill(text);
  const held = await holdPosts(page, messagePath(session.id));
  try {
    await send(page).click();
    await held.started;
    await editor(page).press("ControlOrMeta+Enter");
    await editor(page).press("ControlOrMeta+Enter");
    await expect(editor(page)).toHaveText(text);
    await expect(send(page)).toBeDisabled();
    expect(await messages(chat, session)).toEqual([]);
    expect(held.count()).toBe(1);
    const accepted = page.waitForResponse((response) => isPost(response, messagePath(session.id)));
    held.release();
    expect((await accepted).status()).toBe(201);
    await expect(editor(page)).toHaveText("");
    await expectOnlyMessage(chat, session, text);
    expect(held.count()).toBe(1);
  } finally {
    held.release();
  }
});

test("late send acceptance preserves the other session's draft and selection [latency injection]", async ({ page, chat }) => {
  const source = await chat.createSession("Source conversation");
  const destination = await chat.createSession("Destination conversation");
  await openSession(page, chat, source);
  const text = "Only send this to the source conversation.";
  const otherDraft = "This unsent destination draft must survive.";
  await editor(page).fill(text);
  const held = await holdPosts(page, messagePath(source.id));
  try {
    await send(page).click();
    await held.started;
    await page.getByText(destination.title, { exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`session=${destination.id}`));
    await expect(editor(page)).toHaveText("");
    await editor(page).fill(otherDraft);
    const accepted = page.waitForResponse((response) => isPost(response, messagePath(source.id)));
    held.release();
    expect((await accepted).status()).toBe(201);
    await expectOnlyMessage(chat, source, text);
    await expect(editor(page)).toHaveText(otherDraft);
    await expect(page).toHaveURL(new RegExp(`session=${destination.id}`));
    expect(await messages(chat, destination)).toEqual([]);
    // Accepted messages can asynchronously replace the source's title. Reopen
    // by its public session identity and also verify drafts survive a reload.
    await openSession(page, chat, source);
    await expect(page).toHaveURL(new RegExp(`session=${source.id}`));
    await expect(editor(page)).toHaveText("");
    await openSession(page, chat, destination);
    await expect(page).toHaveURL(new RegExp(`session=${destination.id}`));
    await expect(editor(page)).toHaveText(otherDraft);
  } finally {
    held.release();
  }
});
