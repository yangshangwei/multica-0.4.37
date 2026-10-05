import { expect, test } from "@playwright/test";
import { p1Capture, p1Failure, p1Member, p1Project, p1Session, type P1Project } from "./fixtures/project-p1";
import { p1DescriptionEditor } from "./fixtures/project-p1-health";

// G01–G04 isolate assertions previously coupled in the goal-template journey.
// G05 retains the real conflict/failed-refetch scenario and additionally checks
// that editing after explicit server adoption sends the adopted revision.
test("P1-G01 template preview and cancellation never write the description", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const project = await p1Project(api);
    let writes = 0;
    page.on("request", (request) => { if (new URL(request.url()).pathname === `/api/projects/${project.id}` && request.method() === "PUT") writes++; });
    await page.goto(`/${workspace.slug}/projects/${project.id}`);
    await page.getByRole("button", { name: "Goal template", exact: true }).click();
    await page.getByRole("checkbox", { name: "Acceptance criteria", exact: true }).check();
    await page.getByRole("button", { name: "Preview selected sections", exact: true }).click();
    await expect(page.locator("pre")).toContainText("## Acceptance criteria");
    await expect(p1DescriptionEditor(page)).not.toContainText("Acceptance criteria");
    await page.getByRole("button", { name: "Goal template", exact: true }).click();
    await expect(page.locator("pre")).toHaveCount(0);
    expect(writes).toBe(0);
    expect(await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).toMatchObject({ description: project.description, description_revision: project.description_revision });
    await page.reload();
    await expect(p1DescriptionEditor(page)).toContainText("Keep the original customer goal.");
    await expect(p1DescriptionEditor(page)).not.toContainText("Acceptance criteria");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-G02 selected missing sections append without replacing the original goal", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const project = await p1Project(api);
    await page.goto(`/${workspace.slug}/projects/${project.id}`);
    await page.getByRole("button", { name: "Goal template", exact: true }).click();
    await page.getByRole("checkbox", { name: "Goal", exact: true }).check();
    await page.getByRole("checkbox", { name: "Acceptance criteria", exact: true }).check();
    await page.getByRole("button", { name: "Preview selected sections", exact: true }).click();
    await expect(page.locator("pre")).toContainText("## Acceptance criteria");
    await expect(page.locator("pre")).not.toContainText("## Goal");
    await page.getByRole("button", { name: "Append selected sections", exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect.poll(async () => (await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toContain("## Acceptance criteria");
    const saved = await api.requestJSON<P1Project>(`/api/projects/${project.id}`);
    expect(saved.description).toContain("Keep the original customer goal.");
    expect(saved.description?.match(/## Goal/g)).toHaveLength(1);
    expect(saved.description).not.toContain("## Out of scope");
    expect(saved.description_revision).toBe(project.description_revision + 1);
    await page.reload();
    await expect(p1DescriptionEditor(page)).toContainText("Acceptance criteria");
    await p1Capture(page, info, "selected-sections-persisted");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-G03 selecting only existing headings disables duplicate template insertion", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const project = await p1Project(api, { description: "## Goal\n\nExisting customer goal.\n\n## Acceptance criteria\n\nExisting delivery evidence." });
    await page.goto(`/${workspace.slug}/projects/${project.id}`);
    await page.getByRole("button", { name: "Goal template", exact: true }).click();
    await page.getByRole("checkbox", { name: "Goal", exact: true }).check();
    await page.getByRole("checkbox", { name: "Acceptance criteria", exact: true }).check();
    await page.getByRole("button", { name: "Preview selected sections", exact: true }).click();
    await expect(page.locator("pre")).toHaveText("All selected headings already exist. Select missing sections.");
    await expect(page.getByRole("button", { name: "Append selected sections", exact: true })).toBeDisabled();
    expect(await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).toMatchObject({ description: project.description, description_revision: project.description_revision });
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-G04 a real concurrent description conflict preserves local text for reviewed saving", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  let releaseWrite: (() => void) | undefined;
  try {
    const colleague = await p1Member(workspace);
    const project = await p1Project(api);
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/projects/${project.id}`);
    await expect(p1DescriptionEditor(page)).toContainText("Keep the original customer goal.");
    const held = new Promise<void>((resolve) => { releaseWrite = resolve; });
    let writeSeen = false;
    await page.route(`**/api/projects/${project.id}`, async (route) => {
      if (route.request().method() !== "PUT") return route.continue();
      writeSeen = true; await held; await route.continue();
    });
    const editor = p1DescriptionEditor(page);
    await editor.fill("Local customer goal awaiting comparison");
    await expect.poll(() => writeSeen).toBe(true);
    await colleague.api.requestJSON(`/api/projects/${project.id}`, { method: "PUT", body: { description: "Colleague's reviewed scope", expected_description_revision: project.description_revision } });
    releaseWrite?.();
    await expect(page.getByText("The server has a newer version. Compare before saving.", { exact: true })).toBeVisible();
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toBe("Colleague's reviewed scope");
    await expect(editor).toContainText("Local customer goal awaiting comparison");
    await expect(page.getByRole("button", { name: "Use server version", exact: true })).toBeInViewport({ ratio: 1 });
    await expect(page.getByRole("button", { name: "Save reviewed draft", exact: true })).toBeInViewport({ ratio: 1 });
    await p1Capture(page, info, "description-conflict");
    await page.unroute(`**/api/projects/${project.id}`);
    await page.getByRole("button", { name: "Save reviewed draft", exact: true }).click();
    await expect.poll(async () => (await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toContain("Local customer goal awaiting comparison");
    await page.reload();
    await expect(editor).toContainText("Local customer goal awaiting comparison");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { releaseWrite?.(); await page.unrouteAll({ behavior: "ignoreErrors" }); await api.deleteFeatureWorkspace(workspace.id); }
});

test("P1-G05 explicit server adoption survives failed refetch and uses that revision for the next edit", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  let releaseWrite: (() => void) | undefined;
  try {
    const colleague = await p1Member(workspace);
    const project = await p1Project(api);
    await page.goto(`/${workspace.slug}/projects/${project.id}`);
    await expect(p1DescriptionEditor(page)).toContainText("Keep the original customer goal.");
    let writes = 0; let blockedReads = 0; let nextRevision: number | undefined;
    const held = new Promise<void>((resolve) => { releaseWrite = resolve; });
    await page.route(`**/api/projects/${project.id}`, async (route) => {
      if (route.request().method() === "GET") {
        blockedReads++;
        return route.fulfill({ status: 503, json: { error: "Fixture delayed description refetch" } });
      }
      if (route.request().method() !== "PUT") return route.continue();
      writes++;
      if (writes === 1) await held;
      else nextRevision = route.request().postDataJSON().expected_description_revision;
      await route.continue();
    });
    const editor = p1DescriptionEditor(page);
    await editor.fill("Local draft discarded after explicit review");
    await expect.poll(() => writes).toBe(1);
    const server = await colleague.api.requestJSON<P1Project>(`/api/projects/${project.id}`, { method: "PUT", body: { description: "Server version chosen explicitly", expected_description_revision: project.description_revision } });
    releaseWrite?.();
    await expect.poll(() => blockedReads).toBeGreaterThan(0);
    await page.getByRole("button", { name: "Use server version", exact: true }).click();
    await expect(editor).toContainText("Server version chosen explicitly");
    await expect(editor).not.toContainText("Local draft discarded after explicit review");
    expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toBe("Server version chosen explicitly");
    await p1Capture(page, info, "description-adopt-server-with-read-failure");
    await editor.fill("New edit based on the adopted server version");
    await expect.poll(() => nextRevision).toBe(server.description_revision);
    await expect.poll(async () => (await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).description).toBe("New edit based on the adopted server version");
    await page.unroute(`**/api/projects/${project.id}`);
    await page.reload();
    await expect(editor).toContainText("New edit based on the adopted server version");
  } catch (error) { await p1Failure(page, info); throw error; }
  finally { releaseWrite?.(); await page.unrouteAll({ behavior: "ignoreErrors" }); await api.deleteFeatureWorkspace(workspace.id); }
});
