import { randomUUID } from "node:crypto";
import { test as base, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

type Workspace = { id: string; slug: string; name: string };
type Actor = { api: TestApiClient; workspace: Workspace; userId: string };
type Invitation = { id: string; status: string; workspace_id: string };
type Member = { id: string; user_id: string; role: string };
type Access = { owner: Actor; visitor: Actor };

const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;

const test = base.extend<{ access: Access }>({
  access: async ({}, use) => {
    const actors: Actor[] = [];
    try {
      for (const role of ["owner", "visitor"]) {
        const api = new TestApiClient();
        const slug = `access-${role}-${randomUUID().slice(0, 12)}`;
        await api.login(`${slug}@multica.ai`, `Access ${role}`);
        const workspace = await api.ensureWorkspace(`Access ${role}`, slug);
        const actor = { api, workspace, userId: "" };
        actors.push(actor);
        expect(workspace.slug, "fixture must own its workspace").toBe(slug);
        await api.markUserOnboarded();
        const user = await api.requestJSON<{ id: string }>("/api/me", {
          method: "PATCH", body: { language: "en" },
        });
        actor.userId = user.id;
      }
      await use({ owner: actors[0]!, visitor: actors[1]! });
    } finally {
      for (const actor of actors.reverse()) {
        await actor.api.deleteFeatureWorkspace(actor.workspace.id);
      }
    }
  },
});

async function authenticate(page: Page, actor: Actor) {
  const token = actor.api.getToken();
  if (!token) throw new Error("Missing fixture session");
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
    document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    document.cookie = "multica-locale=en; path=/; SameSite=Lax";
  }, token);
}

async function request(actor: Actor, path: string, method = "GET", body?: unknown, workspaceId = actor.workspace.id) {
  return fetch(`${API_BASE}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${actor.api.getToken()}`,
      "Content-Type": "application/json",
      "X-Workspace-ID": workspaceId,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

async function invite({ owner, visitor }: Access) {
  return owner.api.requestJSON<Invitation>(`/api/workspaces/${owner.workspace.id}/members`, {
    method: "POST", body: { email: visitor.api.getEmail(), role: "member" },
  });
}

async function members(owner: Actor) {
  return owner.api.requestJSON<Member[]>(`/api/workspaces/${owner.workspace.id}/members`);
}

async function expectNotJoined({ owner, visitor }: Access) {
  expect((await visitor.api.getWorkspaces()).map((workspace) => workspace.id)).not.toContain(owner.workspace.id);
  expect((await members(owner)).map((member) => member.user_id)).not.toContain(visitor.userId);
}

async function openDelete(page: Page, owner: Actor) {
  await authenticate(page, owner);
  await page.goto(`/${owner.workspace.slug}/settings?tab=workspace`);
  await page.getByRole("button", { name: "Delete workspace", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Delete workspace", exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

test("accepting an invitation joins exactly once and revisiting shows its accepted state", async ({ page, access }) => {
  const { owner, visitor } = access;
  const invitation = await invite(access);
  await authenticate(page, visitor);
  await page.goto(`/invite/${invitation.id}`);
  await page.getByRole("button", { name: "Accept & Join", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/${owner.workspace.slug}/issues$`));
  await expect(page.getByRole("button", { name: "New Issue", exact: true })).toBeVisible();
  expect((await members(owner)).filter((member) => member.user_id === visitor.userId)).toHaveLength(1);
  expect((await visitor.api.getWorkspaces()).map((workspace) => workspace.id)).toContain(owner.workspace.id);

  await page.goto(`/invite/${invitation.id}`);
  await expect(page.getByText("This invitation has already been accepted.", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Accept & Join", exact: true })).toHaveCount(0);
  expect((await request(visitor, `/api/invitations/${invitation.id}/accept`, "POST")).status).toBe(400);
  expect((await members(owner)).filter((member) => member.user_id === visitor.userId)).toHaveLength(1);
});

test("declining an invitation persists without membership and prevents later acceptance", async ({ page, access }) => {
  const { visitor } = access;
  const invitation = await invite(access);
  await authenticate(page, visitor);
  await page.goto(`/invite/${invitation.id}`);
  await page.getByRole("button", { name: "Decline", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Invitation declined", exact: true })).toBeVisible();
  await expectNotJoined(access);
  await page.reload();
  await expect(page.getByText("This invitation has already been declined.", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Accept & Join", exact: true })).toHaveCount(0);
  expect((await request(visitor, `/api/invitations/${invitation.id}/accept`, "POST")).status).toBe(400);
  expect(await visitor.api.requestJSON<Invitation>(`/api/invitations/${invitation.id}`)).toMatchObject({ status: "declined" });
  await expectNotJoined(access);
});

test("revoking an invitation while its page is open rejects stale acceptance", async ({ page, access }) => {
  const { owner, visitor } = access;
  const invitation = await invite(access);
  await authenticate(page, visitor);
  await page.goto(`/invite/${invitation.id}`);
  const accept = page.getByRole("button", { name: "Accept & Join", exact: true });
  await expect(accept).toBeVisible();
  expect((await request(owner, `/api/workspaces/${owner.workspace.id}/invitations/${invitation.id}`, "DELETE")).status).toBe(204);
  const rejected = page.waitForResponse((response) => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/invitations/${invitation.id}/accept`);
  await accept.click();
  // RevokeInvitation deletes the pending row, so stale clients receive 404.
  const rejection = await rejected;
  expect(rejection.status()).toBe(404);
  expect(await rejection.json()).toMatchObject({ error: "invitation not found" });
  await expect(page.getByText("invitation not found", { exact: true })).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/invite/${invitation.id}$`));
  await expectNotJoined(access);
  await page.reload();
  await expect(page.getByRole("heading", { name: "Invitation not found", exact: true })).toBeVisible();
  await expect(accept).toHaveCount(0);
  expect((await request(visitor, `/api/invitations/${invitation.id}`)).status).toBe(404);
  await expectNotJoined(access);
});

test("a nonexistent invitation shows a recoverable empty state", async ({ page, access }) => {
  const { visitor } = access;
  const invitationId = randomUUID();
  await authenticate(page, visitor);
  await page.goto(`/invite/${invitationId}`);
  await expect(page.getByRole("heading", { name: "Invitation not found", exact: true })).toBeVisible();
  expect((await request(visitor, `/api/invitations/${invitationId}`)).status).toBe(404);
  await page.getByRole("button", { name: "Go to dashboard", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/${visitor.workspace.slug}/issues$`));
  await expect(page.getByRole("button", { name: "New Issue", exact: true })).toBeVisible();
});

test("another account cannot read accept or decline an invitation addressed to its owner", async ({ page, access }) => {
  const { owner, visitor } = access;
  const invitation = await visitor.api.requestJSON<Invitation>(`/api/workspaces/${visitor.workspace.id}/members`, {
    method: "POST", body: { email: owner.api.getEmail(), role: "member" },
  });
  await authenticate(page, visitor);
  await page.goto(`/invite/${invitation.id}`);
  await expect(page.getByRole("heading", { name: "Invitation not found", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Accept & Join", exact: true })).toHaveCount(0);
  expect((await request(visitor, `/api/invitations/${invitation.id}`)).status).toBe(403);
  expect((await request(visitor, `/api/invitations/${invitation.id}/accept`, "POST")).status).toBe(403);
  expect((await request(visitor, `/api/invitations/${invitation.id}/decline`, "POST")).status).toBe(403);
  expect(await owner.api.requestJSON<Invitation>(`/api/invitations/${invitation.id}`)).toMatchObject({ status: "pending" });
  expect((await members(visitor)).map((member) => member.user_id)).not.toContain(owner.userId);
});

test("a nonmember cannot open or mutate another workspace and can recover to their own", async ({ page, access }) => {
  const { owner, visitor } = access;
  const issue = await owner.api.createIssue("Private workspace issue");
  await authenticate(page, visitor);
  await page.goto(`/${owner.workspace.slug}/issues`);
  await expect(page.getByRole("heading", { name: "Workspace not available", exact: true })).toBeVisible();
  await expect(page.getByText(issue.title, { exact: true })).toHaveCount(0);
  expect((await request(visitor, `/api/workspaces/${owner.workspace.id}`)).status).toBe(404);
  expect((await request(visitor, "/api/issues", "GET", undefined, owner.workspace.id)).status).toBe(404);
  expect((await request(visitor, "/api/issues", "POST", { title: "Unauthorized issue" }, owner.workspace.id)).status).toBe(404);
  expect((await request(visitor, `/api/workspaces/${owner.workspace.id}`, "PATCH", { name: "Unauthorized name" })).status).toBe(404);
  expect(await owner.api.requestJSON<Workspace>(`/api/workspaces/${owner.workspace.id}`)).toMatchObject({ name: owner.workspace.name });
  const { issues } = await owner.api.requestJSON<{ issues: { id: string; title: string }[] }>("/api/issues");
  expect(issues.map((item) => item.title)).not.toContain("Unauthorized issue");
  await page.getByRole("button", { name: "Go to my workspaces", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/${visitor.workspace.slug}/issues$`));
  await expect(page.getByRole("button", { name: "New Issue", exact: true })).toBeVisible();
});

test("[API integration] foreign issue identifiers cannot bypass the selected workspace on reads or writes", async ({ access }) => {
  const { owner, visitor } = access;
  const issue = await owner.api.createIssue("Private issue identity", { status: "backlog" });
  const ownIssue = await visitor.api.createIssue("Visible issue identity", { status: "backlog" });
  expect((await request(visitor, `/api/issues/${issue.id}`)).status).toBe(404);
  expect((await request(visitor, `/api/issues/${issue.id}`, "PUT", { title: "Cross-workspace overwrite" })).status).toBe(404);
  expect((await request(visitor, `/api/issues/${issue.id}`, "DELETE")).status).toBe(404);
  expect(await owner.api.requestJSON(`/api/issues/${issue.id}`)).toMatchObject({ title: issue.title, status: "backlog" });
  expect(await visitor.api.requestJSON(`/api/issues/${ownIssue.id}`)).toMatchObject({ title: ownIssue.title, status: "backlog" });
});

test("removing a member from an open workspace relocates the browser and revokes stale API access", async ({ page, access }) => {
  const { owner, visitor } = access;
  const invitation = await invite(access);
  const joined = await visitor.api.requestJSON<Member>(`/api/invitations/${invitation.id}/accept`, { method: "POST" });
  const issue = await owner.api.createIssue("Visible until membership is revoked");
  let authenticatedSocket = false;
  page.on("websocket", (socket) => {
    if (new URL(socket.url()).searchParams.get("workspace_slug") !== owner.workspace.slug) return;
    socket.on("framereceived", ({ payload }) => {
      if (JSON.parse(String(payload)).type === "auth_ack") authenticatedSocket = true;
    });
  });
  await authenticate(page, visitor);
  await page.goto(`/${owner.workspace.slug}/issues`);
  await expect(page.getByText(issue.title, { exact: true }).first()).toBeVisible();
  await expect.poll(() => authenticatedSocket, { message: "workspace WebSocket must authenticate before revocation" }).toBe(true);
  expect((await request(owner, `/api/workspaces/${owner.workspace.id}/members/${joined.id}`, "DELETE")).status).toBe(204);
  await expect(page).toHaveURL(new RegExp(`/${visitor.workspace.slug}/issues$`));
  await expect(page.getByText(issue.title, { exact: true })).toHaveCount(0);
  expect((await request(visitor, `/api/issues/${issue.id}`, "GET", undefined, owner.workspace.id)).status).toBe(404);
  expect((await request(visitor, `/api/issues/${issue.id}`, "PUT", { title: "Stale write" }, owner.workspace.id)).status).toBe(404);
  expect(await owner.api.requestJSON(`/api/issues/${issue.id}`)).toMatchObject({ title: issue.title });
  await expectNotJoined(access);
  await page.goto(`/${owner.workspace.slug}/issues`);
  await expect(page.getByRole("heading", { name: "Workspace not available", exact: true })).toBeVisible();
});

test("cancelling workspace deletion preserves data and resets the typed confirmation", async ({ page, access }) => {
  const { owner } = access;
  const issue = await owner.api.createIssue("Preserve after cancellation");
  let deleteRequests = 0;
  page.on("request", (req) => {
    if (req.method() === "DELETE" && new URL(req.url()).pathname === `/api/workspaces/${owner.workspace.id}`) deleteRequests += 1;
  });
  const dialog = await openDelete(page, owner);
  await dialog.getByRole("textbox").fill(owner.workspace.name);
  await expect(dialog.getByRole("button", { name: "Delete workspace", exact: true })).toBeEnabled();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole("button", { name: "Delete workspace", exact: true }).click();
  await expect(dialog.getByRole("textbox")).toHaveValue("");
  await expect(dialog.getByRole("button", { name: "Delete workspace", exact: true })).toBeDisabled();
  expect(deleteRequests).toBe(0);
  expect(await owner.api.requestJSON(`/api/issues/${issue.id}`)).toMatchObject({ title: issue.title });
  expect((await owner.api.getWorkspaces()).map((workspace) => workspace.id)).toContain(owner.workspace.id);
});

test("[browser fault injection] failed workspace deletion keeps the dialog and data until a successful retry", async ({ page, access }) => {
  const { owner } = access;
  const issue = await owner.api.createIssue("Preserve until deletion succeeds");
  const dialog = await openDelete(page, owner);
  const target = `**/api/workspaces/${owner.workspace.id}`;
  let failedDeletes = 0;
  await page.route(target, async (route) => {
    if (route.request().method() !== "DELETE") return route.continue();
    failedDeletes += 1;
    await route.fulfill({ status: 503, json: { error: "Deletion temporarily unavailable" } });
  });
  try {
    await dialog.getByRole("textbox").fill(owner.workspace.name);
    await dialog.getByRole("button", { name: "Delete workspace", exact: true }).click();
    await expect(page.getByText("Deletion temporarily unavailable", { exact: true })).toBeVisible();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("textbox")).toHaveValue(owner.workspace.name);
    await expect(dialog.getByRole("button", { name: "Delete workspace", exact: true })).toBeEnabled();
    await expect(page).toHaveURL(new RegExp(`/${owner.workspace.slug}/settings\\?tab=workspace$`));
    expect(failedDeletes).toBe(1);
    expect((await owner.api.getWorkspaces()).map((workspace) => workspace.id)).toContain(owner.workspace.id);
    expect(await owner.api.requestJSON(`/api/issues/${issue.id}`)).toMatchObject({ title: issue.title });
  } finally {
    await page.unroute(target);
  }
  const deleted = page.waitForResponse((response) => response.request().method() === "DELETE" && new URL(response.url()).pathname === `/api/workspaces/${owner.workspace.id}`);
  await dialog.getByRole("button", { name: "Delete workspace", exact: true }).click();
  expect((await deleted).status()).toBe(204);
  await expect(page).toHaveURL(/\/workspaces\/new$/);
  expect((await owner.api.getWorkspaces()).map((workspace) => workspace.id)).not.toContain(owner.workspace.id);
  expect((await request(owner, `/api/issues/${issue.id}`)).status).toBe(404);
});
