import { randomUUID } from "node:crypto";
import { expect, type Page } from "@playwright/test";
import type { CollaborationIssue, CollaborationMemory } from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";

export function memoryEnvironment() {
  const apiUrl = process.env.COLLABORATION_E2E_API_URL;
  const databaseUrl = process.env.DATABASE_URL;
  if (!apiUrl || !databaseUrl || !process.env.NEXT_PUBLIC_API_URL) throw new Error("Memory acceptance needs a task-owned API/database and production Web");
  for (const value of [apiUrl, databaseUrl, process.env.NEXT_PUBLIC_API_URL]) {
    if (!["127.0.0.1", "localhost"].includes(new URL(value).hostname)) throw new Error("Memory acceptance only uses loopback services");
  }
  if (["", "postgres", "multica", "template0", "template1"].includes(new URL(databaseUrl).pathname.slice(1))) throw new Error("Memory acceptance refuses a default database");
  expect(new URL(process.env.NEXT_PUBLIC_API_URL).origin).toBe(new URL(apiUrl).origin);
  return { apiUrl };
}

export function memoryAuthority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Memory fixture requires a human participant snapshot");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}

export async function seedMemoryIssue() {
  const env = memoryEnvironment();
  const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
  async function actor(label: string) {
    const client = new TestApiClient();
    const username = `memory${label}${suffix}`;
    const password = `Memory-${randomUUID()}!`;
    if (process.env.E2E_PASSWORD_AUTH === "1") await client.registerPassword(username, password, `Memory ${label}`);
    else await client.login(`${username}@example.invalid`, `Memory ${label}`);
    await client.requestJSON("/api/me/onboarding/complete", { method: "POST", body: {} });
    const user = await client.requestJSON<{ id: string }>("/api/me");
    return { client, id: user.id, async authenticate(page: Page) {
      if (process.env.E2E_PASSWORD_AUTH === "1") {
        const response = await page.request.post(`${env.apiUrl}/auth/login`, { data: { username, password } });
        expect(response.status()).toBe(200);
        const cookies = await page.context().cookies(env.apiUrl);
        expect(cookies.some(cookie => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
        expect(cookies.some(cookie => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
      } else {
        await page.addInitScript(token => localStorage.setItem("multica_token", token), client.getToken()!);
      }
      await page.addInitScript(() => {
        localStorage.setItem("multica-locale", "en");
        localStorage.setItem("multica:chat:isOpen", "false");
      });
    } };
  }
  const creator = await actor("creator");
  const participant = await actor("participant");
  const workspace = await creator.client.ensureWorkspace("Memory source", `memory-source-${suffix}`);
  const issue = await creator.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: workspace.id, handler_user_id: participant.id,
      title: `Memory browser journey ${suffix}`, description: "Synthetic collaboration context acceptance.", attachment_ids: [],
    },
  });
  const root = `/api/collaboration/issues/${issue.id}`;
  const read = () => creator.client.requestJSON<CollaborationMemory>(`${root}/memory`);
  return { ...env, creator, participant, issue, root, read, path: `/account?view=participating&issue=${issue.id}#memory` };
}

export async function saveMemoryInUI(page: Page, content: string, expectedVersion: string) {
  const panel = page.locator("#memory");
  await panel.getByRole("button", { name: "Edit shared context", exact: true }).click();
  await panel.getByLabel("Shared context (Markdown)", { exact: true }).fill(content);
  const response = page.waitForResponse(response => /\/api\/collaboration\/issues\/[^/]+\/memory$/.test(new URL(response.url()).pathname)
    && response.request().method() === "PUT");
  await panel.getByRole("button", { name: "Save shared context", exact: true }).click();
  const saved = await response;
  expect(saved.status()).toBe(200);
  expect(saved.request().postDataJSON()).toEqual({ expected_version: expectedVersion, content });
  await expect(panel.getByLabel("Shared context (Markdown)", { exact: true })).toHaveCount(0);
  return saved;
}
