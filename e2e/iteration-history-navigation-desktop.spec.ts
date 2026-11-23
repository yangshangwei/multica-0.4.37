import { randomUUID } from "node:crypto";
import { expect, type Page, type Request } from "@playwright/test";
import { test } from "./fixtures/project-p1-desktop";
import { scopeAPI } from "./fixtures/iteration-scope-business";

function activeNativePath(page: Page) {
  return page.evaluate(() => {
    const raw = localStorage.getItem("multica_tabs");
    if (!raw) return null;
    const { state }: { state: {
      activeWorkspaceSlug: string | null;
      byWorkspace: Record<string, { activeTabId: string; tabs: { id: string; url: string }[] }>;
    } } = JSON.parse(raw);
    const group = state.activeWorkspaceSlug ? state.byWorkspace[state.activeWorkspaceSlug] : undefined;
    return group?.tabs.find(tab => tab.id === group.activeTabId)?.url ?? null;
  });
}

test("Electron navigation from expanded A to its B parent starts B history collapsed", async ({ native }, info) => {
  const { api, workspace, page } = native;
  const parent = await api.createIssue("Native history destination B", { status: "todo" });
  const issue = await api.createIssue("Native history source A", { status: "todo", parent_issue_id: parent.id });
  const harness = await scopeAPI(api, workspace);
  const cycle = await harness.create("Metadata cycle");
  await harness.move([issue.id], cycle.id, "Verify native issue history navigation");
  await harness.lifecycle(cycle.id, "start");
  expect((await harness.lifecycle(cycle.id, "end")).iteration.status).toBe("completed");

  const sourcePath = `/${workspace.slug}/issues/${issue.id}`;
  const canonicalSource = `/${workspace.slug}/issues/${issue.identifier}`;
  const canonicalDestination = `/${workspace.slug}/issues/${parent.identifier}`;
  const requestedIssueIds: string[] = [];
  const observations: unknown[] = [];
  const recordRequest = (request: Request) => {
    const url = new URL(request.url());
    const id = url.searchParams.get("issue_id");
    if (id !== null && request.method() === "GET" && url.pathname === `/api/workspaces/${workspace.id}/iterations`
      && (id === issue.id || id === parent.id)) requestedIssueIds.push(id);
  };
  page.on("request", recordRequest);
  try {
    await expect.poll(() => activeNativePath(page)).toMatch(new RegExp(`^/${workspace.slug}/`));
    // Only initial entry uses the established persisted-tab/reload pattern.
    // The measured A-to-B transition below is the real parent AppLink click.
    await page.evaluate((url) => {
      const saved = JSON.parse(localStorage.getItem("multica_tabs")!);
      const group = saved.state.byWorkspace[saved.state.activeWorkspaceSlug];
      const active = group.tabs.find((tab: { id: string }) => tab.id === group.activeTabId);
      active.url = url;
      active.resourceKey = url;
      active.history = { stack: [url], index: 0 };
      active.memento = { scroll: {}, view: {} };
      localStorage.setItem("multica_tabs", JSON.stringify(saved));
    }, sourcePath);
    await page.reload();
    await expect.poll(() => activeNativePath(page)).toBe(canonicalSource);
    await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
    const summary = page.locator("summary").filter({ hasText: /^Iteration history$/ });
    const details = page.locator("details").filter({ has: summary });
    await expect(summary).toBeVisible();
    await expect(details).toHaveCount(1);
    await summary.click();
    await expect(details).toHaveJSProperty("open", true);
    await expect(page.getByRole("link", { name: cycle.name, exact: true })).toBeVisible();
    expect(requestedIssueIds).toContain(issue.id);

    const marker = randomUUID();
    await page.evaluate(value => Reflect.set(window, "__nativeIterationHistorySession", value), marker);
    const parentLink = page.getByRole("link")
      .filter({ has: page.getByText("Sub-issue of", { exact: true }) })
      .filter({ has: page.getByText(parent.title, { exact: true }) });
    await expect(parentLink).toHaveCount(1);
    observations.push({ stage: "expanded A before parent click", path: await activeNativePath(page), nativeOpen: await details.evaluate((node: HTMLDetailsElement) => node.open) });
    await parentLink.click();
    await expect.poll(() => activeNativePath(page)).toBe(canonicalDestination);
    await expect(page.getByRole("button", { name: parent.title, exact: true })).toBeVisible();
    const currentMarker = await page.evaluate(() => Reflect.get(window, "__nativeIterationHistorySession"));
    expect(currentMarker).toBe(marker);
    await expect(summary).toBeVisible();
    await expect(details).toHaveCount(1);
    observations.push({ stage: "B committed", path: await activeNativePath(page), sameDocument: currentMarker === marker, nativeOpen: await details.evaluate((node: HTMLDetailsElement) => node.open) });
    await expect(details).toHaveJSProperty("open", false);
    await expect(page.getByRole("link", { name: cycle.name, exact: true })).toBeHidden();
    await page.evaluate(() => new Promise<void>(done => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
    expect(requestedIssueIds.filter(id => id === parent.id)).toHaveLength(0);
  } finally {
    page.off("request", recordRequest);
    await info.attach("native-history-parent-navigation", {
      body: JSON.stringify({ source: issue.id, destination: parent.id, iterationId: cycle.id, sourcePath, canonicalSource, canonicalDestination, requestedIssueIds, observations }, null, 2),
      contentType: "application/json",
    });
  }
});
