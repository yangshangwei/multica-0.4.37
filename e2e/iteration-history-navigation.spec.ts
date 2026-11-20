import { randomUUID } from "node:crypto";
import { expect, test as base, type Page, type Request, type Route } from "@playwright/test";
import { p1Session } from "./fixtures/project-p1";

// These journeys own the real UUID-to-identifier commit boundary. The original
// iterations-i1-history suite retains its complete frozen metadata assertions.
type Issue = { id: string; identifier: string; title: string; revision: number };
type Session = Awaited<ReturnType<typeof p1Session>>;
type Fixture = Session & { issue: Issue; parent: Issue; iterationId: string };

const test = base.extend<{ fixture: Fixture }>({
  fixture: async ({ page }, use) => {
    const session = await p1Session(page);
    const { api, workspace } = session;
    const path = `/api/workspaces/${workspace.id}`;
    try {
      const parent = await api.createIssue("History destination B", { status: "todo" }) as Issue;
      const issue = await api.createIssue("History source A", {
        status: "todo", parent_issue_id: parent.id,
      }) as Issue;
      const settings = await api.requestJSON<{ effective_timezone: string; revision: number }>(`${path}/iteration-settings`);
      const enabled = await api.requestJSON<{ result: { settings_revision: number } }>(`${path}/iteration-settings/enable`, {
        method: "POST",
        body: { request_id: randomUUID(), expected_revision: settings.revision, confirmed_timezone: settings.effective_timezone },
      });
      const created = await api.requestJSON<{ iteration_ids: string[] }>(`${path}/iterations`, {
        method: "POST",
        body: {
          request_id: randomUUID(), name: "Metadata cycle",
          start_date: new Date().toISOString().slice(0, 10),
          end_date: new Date(Date.now() + 13 * 86400000).toISOString().slice(0, 10),
          confirmed_timezone: settings.effective_timezone,
        },
      });
      const iterationId = created.iteration_ids[0]!;
      const common = { expected_settings_revision: enabled.result.settings_revision, reason: "Verify issue history navigation", start: null };
      const apply = async (draft: Record<string, unknown>) => {
        const preview = await api.requestJSON<{ draft: unknown; preview_hash: string; invalid_items: unknown[] }>(`${path}/iteration-previews`, { method: "POST", body: draft });
        expect(preview.invalid_items).toEqual([]);
        await api.requestJSON(`${path}/iteration-operations`, {
          method: "POST", body: { request_id: randomUUID(), draft: preview.draft, preview_hash: preview.preview_hash },
        });
      };
      await apply({
        ...common, operation: "move", iteration_id: null, expected_iteration_revision: null, expected_scope_revision: null,
        moves: [{ issue_id: issue.id, expected_issue_revision: issue.revision, expected_source_id: null, target_id: iterationId, allow_completed: false }],
      });
      const getIteration = () => api.requestJSON<{ iteration: { revision: number; scope_revision: number } }>(`${path}/iterations/${iterationId}`);
      let current = (await getIteration()).iteration;
      await apply({
        ...common, operation: "start", iteration_id: iterationId,
        expected_iteration_revision: current.revision, expected_scope_revision: current.scope_revision, moves: [],
        start: { target_id: iterationId, mode: "scheduled", terminal_choices: [] },
      });
      current = (await getIteration()).iteration;
      const currentIssue = await api.requestJSON<{ revision: number }>(`/api/issues/${issue.id}`);
      await apply({
        ...common, operation: "end", iteration_id: iterationId,
        expected_iteration_revision: current.revision, expected_scope_revision: current.scope_revision,
        moves: [{ issue_id: issue.id, expected_issue_revision: currentIssue.revision, expected_source_id: iterationId, target_id: null, allow_completed: false }],
      });
      await use({ ...session, issue, parent, iterationId });
    } finally {
      await api.deleteFeatureWorkspace(workspace.id);
    }
  },
});

test.use({
  locale: "en-US", viewport: { width: 1440, height: 1000 },
  screenshot: "only-on-failure", actionTimeout: 15_000,
});

function participationRequest(request: Request, fixture: Fixture, issueId: string) {
  const url = new URL(request.url());
  return request.method() === "GET" &&
    url.pathname === `/api/workspaces/${fixture.workspace.id}/iterations` &&
    url.searchParams.get("issue_id") === issueId;
}

function historyPanel(page: Page) {
  const summary = page.locator("summary").filter({ hasText: /^Iteration history$/ });
  const details = page.locator("details").filter({ has: summary });
  return { summary, details };
}

test("expanded history survives the readable issue commit before its history response", async ({ page, fixture }, info) => {
  const { workspace, issue, iterationId } = fixture;
  const uuidPath = `/${workspace.slug}/issues/${issue.id}`;
  const canonicalPath = `/${workspace.slug}/issues/${issue.identifier}`;
  let releaseHistory!: () => void;
  const historyReleased = new Promise<void>((done) => { releaseHistory = done; });
  const handlers: Promise<void>[] = [];
  const historyRequests: Request[] = [];
  const events: unknown[] = [];
  await page.addInitScript(({ uuidPath, canonicalPath }) => {
    const original = history.replaceState.bind(history);
    let pending: (() => void) | undefined;
    Reflect.set(window, "__iterationHistoryReleaseNormalization", () => {
      const commit = pending;
      pending = undefined;
      commit?.();
    });
    history.replaceState = (data, unused, url) => {
      const target = url == null ? null : new URL(String(url), location.href);
      if (location.pathname === uuidPath && target?.pathname === canonicalPath) {
        Reflect.set(window, "__iterationHistoryHeldNormalization", {
          from: location.pathname, to: target.pathname,
        });
        pending = () => original(data, unused, url);
        return;
      }
      original(data, unused, url);
    };
  }, { uuidPath, canonicalPath });
  const releaseNormalization = () => page.evaluate(() => {
    Reflect.get(window, "__iterationHistoryReleaseNormalization")?.();
  });
  const pattern = (url: URL) => url.pathname === `/api/workspaces/${workspace.id}/iterations` && url.searchParams.get("issue_id") === issue.id;
  const handler = async (route: Route) => {
    const request = route.request();
    if (participationRequest(request, fixture, issue.id)) {
      const pending = (async () => {
        historyRequests.push(request);
        events.push({ stage: "issue participation request held", issueId: issue.id });
        await historyReleased;
        // Keep the history response independent of the address-only commit.
        await route.continue();
      })();
      handlers.push(pending);
      await pending;
      return;
    }
    await route.continue();
  };
  await page.route(pattern, handler);
  try {
    await page.goto(uuidPath, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
    const { summary, details } = historyPanel(page);
    await expect(summary).toBeVisible();
    await expect(details).toHaveCount(1);
    await expect.poll(() => page.evaluate(() => Reflect.get(window, "__iterationHistoryHeldNormalization")), {
      message: "The actual canonical history replacement must be held before the UUID click",
    }).toEqual({ from: uuidPath, to: canonicalPath });
    await summary.evaluate((element) => {
      Reflect.set(window, "__iterationHistoryOriginal", element.parentElement);
      element.addEventListener("click", () => {
        Reflect.set(window, "__iterationHistoryClickPath", location.pathname);
      }, { once: true });
    });
    await summary.click();
    expect(await page.evaluate(() => Reflect.get(window, "__iterationHistoryClickPath"))).toBe(uuidPath);
    await expect(details).toHaveJSProperty("open", true);
    await expect.poll(() => historyRequests.length).toBeGreaterThan(0);
    events.push({ stage: "real UUID summary click opened history", pathname: new URL(page.url()).pathname });

    await releaseNormalization();
    await expect(page).toHaveURL(new RegExp(`${canonicalPath}$`));
    await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
    events.push({
      stage: "canonical page committed while history remains held",
      pathname: new URL(page.url()).pathname,
      nativeOpen: await details.evaluate((element: HTMLDetailsElement) => element.open),
      oldElementConnected: await page.evaluate(() => Reflect.get(window, "__iterationHistoryOriginal")?.isConnected),
    });
    await expect(details).toHaveJSProperty("open", true);
    expect(await page.evaluate(() => Reflect.get(window, "__iterationHistoryOriginal")?.isConnected)).toBe(true);

    const loaded = page.waitForResponse((history) => participationRequest(history.request(), fixture, issue.id) && history.status() === 200);
    releaseHistory();
    expect((await loaded).ok()).toBe(true);
    const link = page.getByRole("link", { name: "Metadata cycle", exact: true });
    await expect(link).toBeVisible();
    expect(new URL(await link.getAttribute("href") ?? "", page.url()).pathname).toBe(`/${workspace.slug}/iterations/${iterationId}`);
    await link.click();
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/iterations/${iterationId}$`));
    await expect(page.getByRole("button", { name: "Filter tasks", exact: true })).toBeVisible();
  } finally {
    await releaseNormalization();
    releaseHistory();
    const replacement = await page.evaluate(() => Reflect.get(window, "__iterationHistoryHeldNormalization"));
    await info.attach("history-navigation-events", { body: JSON.stringify({ replacement, historyRequests: historyRequests.length, events }, null, 2), contentType: "application/json" });
    try {
      await Promise.all(handlers);
    } finally {
      await page.unroute(pattern, handler);
    }
  }
});

test("navigating from expanded A to its B parent starts B history collapsed", async ({ page, fixture }, info) => {
  const { workspace, issue, parent } = fixture;
  const historyRequests: Request[] = [];
  page.on("request", (request) => {
    if (participationRequest(request, fixture, issue.id) || participationRequest(request, fixture, parent.id)) historyRequests.push(request);
  });
  await page.goto(`/${workspace.slug}/issues/${issue.id}`, { waitUntil: "domcontentloaded" });
  await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/issues/${issue.identifier}$`));
  await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
  const { summary, details } = historyPanel(page);
  await summary.click();
  await expect(details).toHaveJSProperty("open", true);
  await expect(page.getByRole("link", { name: "Metadata cycle", exact: true })).toBeVisible();
  const sessionMarker = randomUUID();
  await page.evaluate((marker) => Reflect.set(window, "__iterationHistorySession", marker), sessionMarker);
  const parentLink = page.getByRole("link")
    .filter({ has: page.getByText("Sub-issue of", { exact: true }) })
    .filter({ has: page.getByText(parent.title, { exact: true }) });
  await expect(parentLink).toHaveCount(1);
  await parentLink.click();
  await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/issues/${parent.identifier}$`));
  await expect(page.getByRole("button", { name: parent.title, exact: true })).toBeVisible();
  expect(await page.evaluate(() => Reflect.get(window, "__iterationHistorySession"))).toBe(sessionMarker);
  await expect(summary).toBeVisible();
  await expect(details).toHaveJSProperty("open", false);
  await expect(page.getByRole("link", { name: "Metadata cycle", exact: true })).toBeHidden();
  await page.evaluate(() => new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
  expect(historyRequests.filter((request) => participationRequest(request, fixture, parent.id))).toHaveLength(0);
  await info.attach("history-parent-navigation", {
    body: JSON.stringify({ source: issue.id, destination: parent.id, requestedIssueIds: historyRequests.map((request) => new URL(request.url()).searchParams.get("issue_id")) }, null, 2),
    contentType: "application/json",
  });
});
