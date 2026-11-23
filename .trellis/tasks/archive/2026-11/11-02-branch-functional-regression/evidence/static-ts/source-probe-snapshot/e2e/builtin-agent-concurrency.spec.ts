import { test, expect } from "@playwright/test";
import { legacyLaneMissing, loginLegacyOwner, openLegacySession, sql } from "./fixtures/lane-guards";

// agentconfig.DefaultMaxConcurrentTasks (server/internal/agentconfig/concurrency.go:6).
// Before 5701a7695 every built-in template carried a hand-tuned 1, 2 or 3 and
// 小阿孚 started at 3, so none of these assertions held then. The runtime row
// has no daemon process behind it; creating agents starts no work.
const SHARED_DEFAULT = 6;

interface Template { key: string; title: string; max_concurrent_tasks: number }
interface Created { id: string; max_concurrent_tasks: number }

async function concurrencyOf(ids: string[]) {
  const rows = await sql<{ id: string; max_concurrent_tasks: number }>(
    "SELECT id::text AS id, max_concurrent_tasks FROM agent WHERE id = ANY($1::uuid[])", [ids]);
  return Object.fromEntries(rows.map((row) => [row.id, row.max_concurrent_tasks]));
}

test.describe("built-in agents start at the shared concurrency default", () => {
  test.skip(legacyLaneMissing() !== null, legacyLaneMissing() ?? "");

  test("every role template, squad roster and 小阿孚 is created at the same cap as a blank agent", async () => {
    test.setTimeout(150_000);
    const owner = await loginLegacyOwner("e2e-supp-cap", "Concurrency default owner");
    const runtime = await owner.api.seedProjectRuntime();
    const blank = await owner.api.requestJSON<Created>("/api/agents", {
      method: "POST", body: { name: "Blank concurrency baseline", description: "Hand-authored baseline.", runtime_id: runtime.id, permission_mode: "private" },
    });
    expect(blank.max_concurrent_tasks).toBe(SHARED_DEFAULT);

    const { templates } = await owner.api.requestJSON<{ templates: Template[] }>("/api/agents/templates?language=en");
    expect(templates.length).toBeGreaterThan(0);
    for (const template of templates) expect(template.max_concurrent_tasks, template.key).toBe(SHARED_DEFAULT);
    const roleAgents: string[] = [];
    for (const template of templates) {
      const created = await owner.api.requestJSON<Created>("/api/agents/from-template", {
        method: "POST", body: { template_key: template.key, runtime_id: runtime.id, language: "en", permission_mode: "private" },
      });
      expect(created.max_concurrent_tasks, template.key).toBe(SHARED_DEFAULT);
      roleAgents.push(created.id);
    }

    // Squad templates staff an unlisted leader template plus any missing roles.
    const squads = await owner.api.requestJSON<{ templates: { key: string }[] }>("/api/squads/templates?language=en");
    expect(squads.templates.length).toBeGreaterThan(0);
    const squadAgents: string[] = [];
    for (const squad of squads.templates) {
      const staffed = await owner.api.requestJSON<{ squad: { leader_id: string }; created_agent_ids: string[] }>("/api/squads/from-template", {
        method: "POST", body: { template_key: squad.key, runtime_id: runtime.id, language: "en" },
      });
      squadAgents.push(staffed.squad.leader_id, ...staffed.created_agent_ids);
    }

    const mika = await owner.api.requestJSON<Created>("/api/agents/mika", {
      method: "POST", body: { runtime_id: runtime.id, language: "en" },
    });
    expect(mika.max_concurrent_tasks).toBe(SHARED_DEFAULT);

    const stored = await concurrencyOf([blank.id, ...roleAgents, ...squadAgents, mika.id]);
    const ids = [...new Set([blank.id, ...roleAgents, ...squadAgents, mika.id])];
    expect(Object.keys(stored).sort()).toEqual([...ids].sort());
    expect(new Set(Object.values(stored))).toEqual(new Set([SHARED_DEFAULT]));
  });

  test("an agent staffed from a role template in the browser shows the shared default as its concurrency", async ({ page, baseURL }) => {
    test.setTimeout(120_000);
    const owner = await loginLegacyOwner("e2e-supp-cap", "Concurrency browser owner");
    const runtime = await owner.api.seedProjectRuntime();
    const { templates } = await owner.api.requestJSON<{ templates: Template[] }>("/api/agents/templates?language=en");
    // Release Engineer started at 1 before the change.
    const template = templates.find((item) => item.key === "release-engineer");
    if (!template) throw new Error("The built-in roster no longer lists release-engineer");
    await openLegacySession(page, baseURL!, owner.token);
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${owner.slug}/agents/new/template?template=${template.key}`, { waitUntil: "domcontentloaded" });
    await expect(page.locator("#agent-create-name")).toHaveValue(template.title, { timeout: 30_000 });
    const created = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/agents/from-template" && response.request().method() === "POST");
    await page.getByRole("button", { name: /Create & open agent/ }).click();
    const response = await created;
    expect(response.status(), await response.text()).toBe(201);
    expect(response.request().postDataJSON()).toMatchObject({ template_key: template.key, runtime_id: runtime.id });
    const agent = await response.json() as Created;
    expect(agent.max_concurrent_tasks).toBe(SHARED_DEFAULT);
    await expect(page).toHaveURL((url) => url.pathname === `/${owner.slug}/agents/${agent.id}`);
    await expect(page.locator('xpath=//dt[normalize-space(.)="Concurrency"]/following-sibling::dd[1]'))
      .toHaveText(String(SHARED_DEFAULT), { timeout: 30_000 });
  });
});
