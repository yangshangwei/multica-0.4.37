import { test, expect, type Page } from "@playwright/test";
import { legacyLaneMissing, loginLegacyOwner, openLegacySession, rawRequest, sql } from "./fixtures/lane-guards";

// Create with agent hands the request to an agent whose only step is
// `multica issue create`, which the server refuses below Contributor
// (QuickCreateMinAutonomy, server/internal/service/task.go). These tests drive
// the real API and browser; agent rows come from POST /api/agents plus a person
// setting autonomy_level, and the runtime row has no process behind it.
// Copy: packages/views/locales/en/modals.json create_issue.{agent,actor_picker}.
const AGENT_REASON = "Autonomy level is Observer, so it cannot create issues. Create one manually and assign it to this agent.";
const SQUAD_REASON = "The leader's autonomy level is Observer, so it cannot create issues. Create one manually and assign it to this squad.";
const BLOCKED_TOOLTIP = "The selected creator cannot create issues. Create manually instead.";

async function seedCreators() {
  const owner = await loginLegacyOwner("e2e-supp-qc", "Quick-create autonomy owner");
  const runtime = await owner.api.seedProjectRuntime();
  const agentAt = async (name: string, level: "observer" | "contributor") => {
    const agent = await owner.api.requestJSON<{ id: string; name: string }>("/api/agents", {
      method: "POST", body: { name, description: `${name}: saved responsibility.`, runtime_id: runtime.id, permission_mode: "private" },
    });
    const updated = await owner.api.requestJSON<{ autonomy_level?: string }>(`/api/agents/${agent.id}`, {
      method: "PUT", body: { autonomy_level: level },
    });
    expect(updated.autonomy_level).toBe(level);
    return agent;
  };
  // Created first, so the list (created_at ASC) offers the Observer as agents[0]:
  // the pre-change fallback seeded exactly that row.
  const observer = await agentAt("Aardvark observer", "observer");
  const contributor = await agentAt("Badger contributor", "contributor");
  const squadLedBy = (name: string, leaderId: string) => owner.api.requestJSON<{ id: string; name: string }>("/api/squads", {
    method: "POST", body: { name, description: `${name} coordinates delivery.`, leader_id: leaderId },
  });
  const observerSquad = await squadLedBy("Observer-led review squad", observer.id);
  const contributorSquad = await squadLedBy("Contributor-led build squad", contributor.id);
  return { ...owner, observer, contributor, observerSquad, contributorSquad };
}

async function quickCreateTasks(agentId: string) {
  const [row] = await sql<{ count: string }>(
    "SELECT count(*)::text AS count FROM agent_task_queue WHERE agent_id = $1 AND context->>'type' = 'quick_create'", [agentId]);
  return Number(row?.count ?? "NaN");
}

const pickerButton = (page: Page, type: "agent" | "squad", id: string) =>
  page.locator(`[data-actor-key="${type}:${id}"] button[data-picker-item]`);

test.describe("quick-create autonomy guard", () => {
  test.skip(legacyLaneMissing() !== null, legacyLaneMissing() ?? "");
  test.use({ viewport: { width: 1280, height: 900 } });

  test("the creator picker lists Observer-level creators disabled with the reason and seeds a Contributor", async ({ page, baseURL }) => {
    test.setTimeout(120_000);
    page.setDefaultTimeout(15_000);
    const fixture = await seedCreators();
    const submissions: unknown[] = [];
    page.on("request", (request) => {
      if (request.method() === "POST" && new URL(request.url()).pathname === "/api/issues/quick-create") submissions.push(request.postDataJSON());
    });
    await openLegacySession(page, baseURL!, fixture.token, "agent");
    await page.goto(`/${fixture.slug}/issues`, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "New Issue", exact: true }).first().click();

    // Nothing is remembered yet, so the seed is the first agent that can create.
    const trigger = page.getByRole("button", { name: /^Send to/ });
    await expect(trigger).toContainText(fixture.contributor.name);
    await expect(trigger).not.toContainText(fixture.observer.name);

    await trigger.click();
    await expect(page.getByPlaceholder("Search names or responsibilities...", { exact: true })).toBeVisible();
    for (const [type, id, reason] of [["agent", fixture.observer.id, AGENT_REASON], ["squad", fixture.observerSquad.id, SQUAD_REASON]] as const) {
      await expect(pickerButton(page, type, id)).toBeDisabled();
      // Inline, not tooltip-only: the reason is visible text inside the row.
      await expect(pickerButton(page, type, id)).toContainText(reason);
    }
    for (const [type, id] of [["agent", fixture.contributor.id], ["squad", fixture.contributorSquad.id]] as const) {
      await expect(pickerButton(page, type, id)).toBeEnabled();
      await expect(pickerButton(page, type, id)).not.toContainText("cannot create issues");
    }
    await pickerButton(page, "squad", fixture.contributorSquad.id).click();
    await expect(trigger).toContainText(fixture.contributorSquad.name);

    const dialog = page.getByRole("dialog", { name: "Quick create issue", exact: true });
    await dialog.locator('[contenteditable="true"]').first().fill("File the slow ES query as its own task.");
    const accepted = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/issues/quick-create" && response.request().method() === "POST");
    await page.getByRole("button", { name: /^Create(?:\s|$)/ }).last().click();
    const response = await accepted;
    expect(response.status()).toBe(202);
    expect(response.request().postDataJSON()).toMatchObject({ squad_id: fixture.contributorSquad.id });
    expect(submissions).toHaveLength(1);
    // The squad leader runs the quick-create task; nothing was queued for the Observer.
    expect(await quickCreateTasks(fixture.contributor.id)).toBe(1);
    expect(await quickCreateTasks(fixture.observer.id)).toBe(0);
  });

  test("an explicitly requested Observer explains the refusal and hands off to manual creation assigned to it", async ({ page, baseURL }) => {
    test.setTimeout(120_000);
    page.setDefaultTimeout(15_000);
    const fixture = await seedCreators();
    const submissions: string[] = [];
    page.on("request", (request) => {
      if (request.method() === "POST" && new URL(request.url()).pathname === "/api/issues/quick-create") submissions.push(request.url());
    });
    await openLegacySession(page, baseURL!, fixture.token, "agent");
    // "Assign work" opens quick-create with agent_id: an explicit pick the panel honours and explains.
    await page.goto(`/${fixture.slug}/agents/${fixture.observer.id}`, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "Assign work", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Quick create issue", exact: true });
    await expect(page.getByRole("button", { name: /^Send to/ })).toContainText(fixture.observer.name);
    await expect(dialog.getByText(`${fixture.observer.name} cannot create this issue`, { exact: true })).toBeVisible();
    await expect(dialog.getByText(new RegExp(`needs autonomy level Contributor or above\\. ${fixture.observer.name} is Observer`))).toBeVisible();

    const prompt = dialog.locator('[contenteditable="true"]').first();
    await prompt.fill("Analyse why the ES query takes three minutes.");
    // Content is present, so the autonomy refusal is what keeps Create disabled.
    const create = dialog.locator(`button[title="${BLOCKED_TOOLTIP}"]`);
    await expect(create).toBeDisabled();
    await expect(create).toHaveText(/^Create/);
    await prompt.press("ControlOrMeta+Enter");
    await expect(dialog).toBeVisible();

    await dialog.getByRole("button", { name: `Create manually and assign to ${fixture.observer.name}`, exact: true }).click();
    const manual = page.getByRole("dialog", { name: "New Issue", exact: true });
    await expect(manual.getByRole("button", { name: fixture.observer.name })).toBeVisible();
    await manual.getByRole("textbox", { name: "Title (required)", exact: true }).fill("Slow ES query analysis");
    const created = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/issues" && response.request().method() === "POST");
    await manual.getByRole("button", { name: "Create Issue", exact: true }).click();
    const response = await created;
    expect(response.ok(), await response.text()).toBe(true);
    expect(response.request().postDataJSON()).toMatchObject({ assignee_type: "agent", assignee_id: fixture.observer.id });
    const issue = await response.json() as { id: string };
    expect(await fixture.api.requestJSON(`/api/issues/${issue.id}`)).toMatchObject({ assignee_type: "agent", assignee_id: fixture.observer.id });
    // Neither the button, the shortcut nor the hand-off sent a quick-create.
    expect(submissions).toEqual([]);
    expect(await quickCreateTasks(fixture.observer.id)).toBe(0);
  });

  test("the API refuses an Observer or an Observer-led squad before enqueueing and accepts Contributors", async () => {
    const fixture = await seedCreators();
    const quickCreate = (body: Record<string, string>) => rawRequest(fixture.api, "/api/issues/quick-create", {
      method: "POST", workspaceId: fixture.workspace.id, body: { ...body, prompt: "Investigate why the ES query takes three minutes" },
    });
    const refusedBodies: Record<string, string>[] = [{ agent_id: fixture.observer.id }, { squad_id: fixture.observerSquad.id }];
    for (const body of refusedBodies) {
      const refused = await quickCreate(body);
      expect(refused.status, refused.text).toBe(422);
      expect(refused.body).toMatchObject({ code: "agent_autonomy_insufficient", autonomy_level: "observer", required_autonomy: "contributor" });
      // Clients that predate the code show `error` verbatim, so it names the fix.
      expect(String(refused.body?.error)).toContain("Create the issue manually and assign it to this agent");
    }
    expect(await quickCreateTasks(fixture.observer.id)).toBe(0);

    const acceptedBodies: Record<string, string>[] = [{ agent_id: fixture.contributor.id }, { squad_id: fixture.contributorSquad.id }];
    for (const body of acceptedBodies) {
      const accepted = await quickCreate(body);
      expect(accepted.status, accepted.text).toBe(202);
      expect(accepted.body?.task_id).toEqual(expect.any(String));
    }
    // No daemon runs: both tasks stay queued for the Contributor (the squad's leader).
    expect(await quickCreateTasks(fixture.contributor.id)).toBe(2);
  });
});
