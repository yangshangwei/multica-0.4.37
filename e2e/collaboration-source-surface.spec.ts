import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect, type Page } from "@playwright/test";
import { openDesktop, nativeScreenshot } from "./fixtures/collaboration-native";
import { boardCard, boardDisplay, cardColumns, chooseView, dragBoard, navigateNative, observeSharedWrites, runReorderCLI, seedSourceSurface, sourceCurrent, sourceSnapshot, statusColumn } from "./fixtures/collaboration-source-surface";

// Password cookies and CLI credentials stay outside traces.
test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

test("Web and Electron source list, board, timeline and comments preserve external identity and scoped layout order", async ({ page }, info) => {
  test.setTimeout(240_000);
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires explicit task-owned password production services");
  const fixture = await seedSourceSurface();
  const origin = info.project.use.baseURL!;
  const profile = mkdtempSync(join(tmpdir(), "multica-source-surface-"));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  const errors: string[] = [];
  const captures: string[] = [];
  const observed: Record<string, unknown>[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const sourceIdentities: unknown[] = [];
  page.on("response", async (response) => {
    if (new URL(response.url()).pathname !== "/api/issues" || response.request().method() !== "GET") return;
    try {
      const result: { issues?: { id: string; collaboration?: unknown }[] } = await response.json();
      const item = result.issues?.find((issue) => issue.id === fixture.shared.id);
      if (item?.collaboration) sourceIdentities.push(item.collaboration);
    } catch { /* Responses aborted during navigation are not accepted evidence. */ }
  });

  async function inspect(current: Page, name: string, navigate: (path: string) => Promise<void>) {
    await navigate(fixture.listPath);
    await expect(current.getByText(fixture.shared.title, { exact: true }).first()).toBeVisible();
    await chooseView(current, "List");
    const row = current.locator("a").filter({ hasText: fixture.shared.title }).first();
    await expect(row).toContainText(fixture.external.name);
    await expect(row).toContainText("External collaborator");
    await expect(row).not.toContainText("Unknown");
    await expect(row.locator(`[title="${fixture.external.name}"]`)).toBeVisible();
    await current.getByRole("button", { name: "External collaborators", exact: true }).click();
    await expect(current.getByRole("button", { name: "External collaborators", exact: true })).toHaveAttribute("aria-pressed", "true");
    await expect(current.getByText(fixture.first.title, { exact: true })).toHaveCount(0);
    await expect(current.getByText(fixture.shared.title, { exact: true }).first()).toBeVisible();
    await current.getByRole("button", { name: "External collaborators", exact: true }).click();
    await expect(current.getByRole("button", { name: "External collaborators", exact: true })).toHaveAttribute("aria-pressed", "false");
    await expect(current.getByText(fixture.first.title, { exact: true }).first()).toBeVisible();
    await chooseView(current, "Board");
    await boardDisplay(current, "Assignee");
    await expect(boardCard(current, fixture.shared.title)).toContainText("External collaborator");
    await expect(boardCard(current, fixture.shared.title)).not.toContainText("Unknown");
    await expect(current.locator(`[title="${fixture.external.name}"]`).first()).toBeVisible();
    const before = await sourceSnapshot(fixture);
    const destination = boardCard(current, fixture.first.title);
    const columns = await cardColumns(current, fixture.shared.title);
    expect(columns).toHaveLength(1);
    const noWrite = observeSharedWrites(current, fixture);
    await dragBoard(current, boardCard(current, fixture.shared.title), destination, async () => {
      expect(await sourceSnapshot(fixture)).toEqual(before);
      expect(noWrite.writes).toEqual([]);
      expect(await cardColumns(current, fixture.shared.title)).toEqual(columns);
    });
    const dialog = current.getByRole("dialog", { name: "Issue actions", exact: true });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Transfer", exact: true })).toBeVisible();
    await expect(dialog.getByRole("textbox", { name: "Exact recipient", exact: true })).toBeVisible();
    expect(noWrite.writes).toEqual([]);
    expect(await cardColumns(current, fixture.shared.title)).toEqual(columns);
    expect(await sourceSnapshot(fixture)).toEqual(before);
    await current.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    noWrite.dispose();

    await boardDisplay(current, "Status");
    const layout = await sourceSnapshot(fixture);
    const original = await sourceCurrent(fixture);
    const reordered = current.waitForResponse((response) => new URL(response.url()).pathname === `/api/issues/${fixture.shared.id}` && response.request().method() === "PUT");
    await dragBoard(current, boardCard(current, fixture.shared.title), boardCard(current, fixture.second.title), async () => {
      await info.attach(`${name}-shared-reorder-drag-over`, { contentType: "application/json", body: JSON.stringify({ columns: await cardColumns(current, fixture.shared.title),
        source: await boardCard(current, fixture.shared.title).boundingBox(), destination: await boardCard(current, fixture.second.title).boundingBox(),
        live: await current.getByRole("status").allTextContents() }, null, 2) });
    });
    const move = await reordered;
    expect(move.status()).toBe(200);
    expect(move.request().postDataJSON()).toMatchObject({ expected_revision: original.revision });
    expect(Object.keys(move.request().postDataJSON()).sort()).toEqual(["expected_revision", "position"]);
    const after = await sourceCurrent(fixture);
    expect(after.position).not.toBe(original.position);
    expect(await sourceSnapshot(fixture)).toEqual(layout);
    await current.reload();
    await expect(boardCard(current, fixture.shared.title)).toBeVisible();
    expect((await sourceCurrent(fixture)).position).toBe(after.position);
    async function assertRenderedOrder() {
      const ordered = await Promise.all([fixture.shared.id, fixture.first.id, fixture.second.id].map((id) => sourceCurrent(fixture, id)));
      const expected = ordered.sort((a, b) => a.position - b.position).map((item) => item.id);
      await expect.poll(async () => current.locator('[data-tab-scroll-root="board:status:todo"] [data-board-card] a').evaluateAll((links, ids) => links
        .map((link) => link.getAttribute("href")?.split("/").at(-1)).filter((id): id is string => typeof id === "string" && ids.includes(id)), expected)).toEqual(expected);
    }
    await assertRenderedOrder();
    await info.attach(`${name}-board-layout-request`, { contentType: "application/json", body: JSON.stringify({ before: original.position, after: after.position,
      request: move.request().postDataJSON(), workflow_unchanged: true }, null, 2) });
    const first = await sourceCurrent(fixture, fixture.first.id);
    const second = await sourceCurrent(fixture, fixture.second.id);
    const earlier = first.position < second.position ? first : second;
    const cli = await runReorderCLI(fixture, earlier.id);
    expect((await sourceCurrent(fixture)).position).toBeLessThan((await sourceCurrent(fixture, earlier.id)).position);
    expect((await sourceCurrent(fixture)).position).not.toBe(after.position);
    expect(await sourceSnapshot(fixture)).toEqual(layout);
    observed.push({ client: name, cli });

    await current.reload();
    await expect(boardCard(current, fixture.shared.title)).toBeVisible();
    await assertRenderedOrder();
    const statusBefore = await sourceSnapshot(fixture);
    const statusColumns = await cardColumns(current, fixture.shared.title);
    const statusWrites = observeSharedWrites(current, fixture);
    await dragBoard(current, boardCard(current, fixture.shared.title), statusColumn(current, "In Progress"), async () => {
      expect(statusWrites.writes).toEqual([]);
      expect(await cardColumns(current, fixture.shared.title)).toEqual(statusColumns);
    });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Change status", exact: true })).toBeVisible();
    await expect(dialog.getByLabel("Status", { exact: true })).toHaveValue("in_progress");
    expect(statusWrites.writes).toEqual([]);
    expect(await sourceSnapshot(fixture)).toEqual(statusBefore);
    if (name === "web") {
      const applied = current.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${fixture.shared.id}` && response.request().method() === "PATCH");
      await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
      expect((await applied).status()).toBe(200);
      expect(statusWrites.writes.every((write) => write.path.includes("/api/collaboration/") && write.method === "PATCH")).toBe(true);
      expect((await sourceCurrent(fixture)).status).toBe("in_progress");
    }
    await current.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    statusWrites.dispose();

    if (name === "web") {
      // Restore Todo through the public workflow before the next client's
      // independent column-local reorder.
      await current.reload();
      await expect(boardCard(current, fixture.shared.title)).toBeVisible();
      await dragBoard(current, boardCard(current, fixture.shared.title), statusColumn(current, "Todo"));
      await expect(dialog.getByLabel("Status", { exact: true })).toHaveValue("todo");
      const restored = current.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${fixture.shared.id}` && response.request().method() === "PATCH");
      await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
      expect((await restored).status()).toBe(200);
      expect((await sourceCurrent(fixture)).status).toBe("todo");
      await current.keyboard.press("Escape");
      await expect(dialog).toHaveCount(0);
    }

    const terminalBefore = await sourceSnapshot(fixture);
    const terminalColumns = await cardColumns(current, fixture.shared.title);
    const terminalWrites = observeSharedWrites(current, fixture);
    await dragBoard(current, boardCard(current, fixture.shared.title), statusColumn(current, "Done"), async () => {
      expect(terminalWrites.writes).toEqual([]);
      expect(await cardColumns(current, fixture.shared.title)).toEqual(terminalColumns);
    });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText("Use Complete, Cancel or Reopen");
    expect(terminalWrites.writes).toEqual([]);
    expect(await sourceSnapshot(fixture)).toEqual(terminalBefore);
    await current.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    terminalWrites.dispose();

    const controlBefore = await sourceCurrent(fixture, fixture.control.id);
    const ordinaryStatus = controlBefore.status === "backlog" ? "In Progress" : "Backlog";
    const ordinaryKey = ordinaryStatus === "In Progress" ? "in_progress" : "backlog";
    const ordinaryColumns = await cardColumns(current, fixture.control.title);
    const ordinaryApplied = current.waitForResponse((response) => new URL(response.url()).pathname === `/api/issues/${fixture.control.id}/move` && response.request().method() === "POST");
    await dragBoard(current, boardCard(current, fixture.control.title), statusColumn(current, ordinaryStatus), async () => {
      const card = await boardCard(current, fixture.control.title).boundingBox();
      const target = await statusColumn(current, ordinaryStatus).boundingBox();
      const columns = await cardColumns(current, fixture.control.title);
      await info.attach(`${name}-ordinary-drag-over`, { contentType: "application/json", body: JSON.stringify({ before: ordinaryColumns, columns, card, target,
        live: await current.getByRole("status").allTextContents() }, null, 2) });
      if (!target || !card) throw new Error("Ordinary drag needs its actual optimistic card and column");
      expect(columns).not.toEqual(ordinaryColumns);
      // Drop into the empty space beneath the optimistically moved card.
      // Releasing over the card itself reports a no-op same-card reorder.
      await current.mouse.move(target.x + target.width / 2, target.y + Math.min(target.height - 20, card.height + 50), { steps: 8 });
      await current.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    });
    const ordinary = await ordinaryApplied;
    expect(ordinary.status()).toBe(200);
    expect(ordinary.request().postDataJSON()).toMatchObject({ status: ordinaryKey, before_id: null, after_id: null });
    expect((await sourceCurrent(fixture, fixture.control.id)).status).toBe(ordinaryKey);
    expect(await sourceSnapshot(fixture)).toEqual(terminalBefore);

    await navigate(fixture.projectPath);
    await expect(current.getByText(fixture.project.title, { exact: true }).first()).toBeVisible();
    await chooseView(current, "Gantt");
    const timeline = current.getByRole("listitem").filter({ hasText: fixture.shared.title }).first();
    await expect(timeline).toContainText("External collaborator");
    await expect(timeline.locator(`[title="${fixture.external.name}"]`)).toBeVisible();
    await expect(timeline).not.toContainText("Unknown");
    await navigate(fixture.detailPath);
    await expect(current.getByRole("heading", { name: fixture.shared.title, exact: true })).toBeVisible();
    const comment = current.locator(`[id="comment-${fixture.comment.comment_id}"]`);
    await expect(comment).toContainText(fixture.external.name);
    await expect(comment).toContainText("External collaborator");
    await expect(comment).not.toContainText("Unknown");
    await expect(comment.locator('[data-slot="avatar"]')).toBeVisible();
    const path = info.outputPath(`${name}-source-external-comment.png`);
    if (native && current === native.page) await nativeScreenshot(native.app, current, path);
    else await current.screenshot({ path, animations: "disabled" });
    captures.push(path);
  }

  try {
    await fixture.creator.authenticate(page);
    await inspect(page, "web", async (path) => { await page.goto(new URL(path, origin).href); });
    const observerContext = await page.context().browser()!.newContext();
    try {
      const observer = await observerContext.newPage();
      await fixture.observer.authenticate(observer);
      await observer.goto(new URL(fixture.listPath, origin).href);
      await chooseView(observer, "Board");
      await boardDisplay(observer, "Assignee");
      const baseline = await sourceSnapshot(fixture);
      const columns = await cardColumns(observer, fixture.shared.title);
      const noWrite = observeSharedWrites(observer, fixture);
      await dragBoard(observer, boardCard(observer, fixture.shared.title), boardCard(observer, fixture.first.title), async () => {
        expect(noWrite.writes).toEqual([]);
        expect(await cardColumns(observer, fixture.shared.title)).toEqual(columns);
      });
      const refusal = observer.getByRole("dialog", { name: "Issue actions", exact: true });
      await expect(refusal.getByRole("alert")).toHaveText("Only the current handler or creator can make this change.");
      expect(noWrite.writes).toEqual([]);
      expect(await sourceSnapshot(fixture)).toEqual(baseline);
      noWrite.dispose();
      observed.push({ client: "web-source-only", refusal: true, ordinary_writes: 0, optimistic_move: false });
    } finally { await observerContext.close(); }

    native = await openDesktop(profile, { token: fixture.creator.client.getToken()! });
    await native.start();
    await expect(native.page.locator("body")).toContainText(fixture.workspace.name);
    await info.attach("electron-source-entry", { contentType: "application/json", body: JSON.stringify({ url: native.page.url(),
      text: await native.page.locator("body").innerText(), links: await native.page.getByRole("link").evaluateAll((nodes) => nodes.map((node) => ({ text: node.textContent,
        href: node.getAttribute("href"), sidebar: node.getAttribute("data-sidebar") }))), errors: native.errors }, null, 2) });
    await inspect(native.page, "electron", (path) => navigateNative(native!.page, fixture, path));
    expect(sourceIdentities.length).toBeGreaterThan(0);
    for (const summary of sourceIdentities) {
      expect(summary).toMatchObject({ handler: { user_id: fixture.external.id, name: fixture.external.name, external: true } });
      expect(JSON.stringify(summary)).not.toMatch(/"(?:email|username)"/);
    }
    expect(errors).toEqual([]);
    expect(native.errors).toEqual([]);
    await info.attach("source-surface-identities", { contentType: "application/json", body: JSON.stringify({ issue_id: fixture.shared.id,
      creator_id: fixture.creator.id, external_handler_id: fixture.external.id, observer_id: fixture.observer.id,
      comment_id: fixture.comment.comment_id, source_identity: sourceIdentities, observations: observed, captures,
      scope: "Actual Web/Electron source surface and CLI layout; no provider or full installed Main", canonical_clauses: ["machine reorder refusal; stage-specific authority/contention and parent wakeup"] }, null, 2) });
  } finally {
    await native?.app.close().catch(() => undefined);
    rmSync(profile, { recursive: true, force: true });
  }
});
