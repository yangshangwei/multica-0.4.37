import { expect, test, type Page, type Request, type Route } from "@playwright/test";
import { p1Session } from "./fixtures/project-p1";

// Core's mutations.test.tsx owns the delayed-read and cache-identity matrix.
// These journeys keep the real ordinary detail control and server retry contract.
type Issue = { id: string; identifier: string; title: string; status: string };

test.use({
  locale: "en-US",
  viewport: { width: 1440, height: 1000 },
  screenshot: "only-on-failure",
  actionTimeout: 15_000,
});

async function chooseStatus(page: Page) {
  await page.getByRole("button", { name: "Todo", exact: true }).click();
  const option = page.locator("button[data-picker-item]").filter({ hasText: /^In Progress$/ });
  await option.click();
  await expect(option).toBeHidden();
}

for (const entry of ["UUID", "identifier"] as const) {
  test(`ordinary status is immediate, rolls back and persists one retry from ${entry}`, async ({ page }) => {
    const { api, workspace } = await p1Session(page);
    let release!: () => void;
    const released = new Promise<void>((done) => { release = done; });
    let firstRoute: Promise<void> | undefined;
    let handler: ((route: Route) => Promise<void>) | undefined;
    let pattern: ((url: URL) => boolean) | undefined;
    try {
      const issue = await api.createIssue(`Optimistic status from ${entry}`, { status: "todo" }) as Issue;
      const path = `/api/issues/${issue.id}`;
      await page.goto(`/${workspace.slug}/issues/${entry === "UUID" ? issue.id : issue.identifier}`, {
        waitUntil: "domcontentloaded",
      });
      await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();

      const requests: Request[] = [];
      const isWrite = (request: Request) => new URL(request.url()).pathname === path && request.method() === "PUT";
      page.on("request", (request) => { if (isWrite(request)) requests.push(request); });
      pattern = (url) => url.pathname === path;
      handler = async (route) => {
        if (!isWrite(route.request()) || firstRoute) {
          await route.continue();
          return;
        }
        firstRoute = (async () => {
          await released;
          await route.fulfill({
            status: 503,
            contentType: "application/json",
            body: JSON.stringify({ error: "Status update unavailable" }),
          });
        })();
        await firstRoute;
      };
      await page.route(pattern, handler);

      await chooseStatus(page);
      await expect.poll(() => requests.length).toBe(1);
      expect(requests[0]!.postDataJSON()).toMatchObject({ status: "in_progress" });
      await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
      expect(await api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id, status: "todo" });

      release();
      await expect(page.getByText("Status update unavailable", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();
      expect(await api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id, status: "todo" });
      await firstRoute;
      await page.unroute(pattern, handler);

      const [retried] = await Promise.all([
        page.waitForResponse((response) => isWrite(response.request())),
        chooseStatus(page),
      ]);
      expect(retried.ok()).toBe(true);
      expect(await api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id, status: "in_progress" });
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
      expect(requests).toHaveLength(2);
      for (const request of requests) {
        expect(new URL(request.url()).pathname).toBe(path);
        expect(request.postDataJSON()).toMatchObject({ status: "in_progress" });
      }
    } finally {
      release();
      try {
        await firstRoute;
      } finally {
        try {
          if (pattern && handler) await page.unroute(pattern, handler);
        } finally {
          await api.deleteFeatureWorkspace(workspace.id);
        }
      }
    }
  });

  test(`old background detail read preserves the pending ordinary status from ${entry}`, async ({ page }, info) => {
    const { api, workspace } = await p1Session(page);
    let releaseRead!: () => void;
    let releaseWrite!: () => void;
    const readReleased = new Promise<void>((done) => { releaseRead = done; });
    const writeReleased = new Promise<void>((done) => { releaseWrite = done; });
    const backgroundReads: { request: Request; captured: boolean; handled: Promise<void> }[] = [];
    let firstWriteRoute: Promise<void> | undefined;
    let uuidReadCount = 0;
    let identifierReadCaptured = false;
    const events: unknown[] = [];
    let pattern: ((url: URL) => boolean) | undefined;
    let handler: ((route: Route) => Promise<void>) | undefined;
    try {
      const issue = await api.createIssue(`Background read status from ${entry}`, { status: "todo" }) as Issue;
      const path = `/api/issues/${issue.id}`;
      const identifierPath = `/api/issues/${issue.identifier}`;
      const writes: Request[] = [];
      page.on("request", (request) => {
        if (new URL(request.url()).pathname === path && request.method() === "PUT") {
          writes.push(request);
          events.push({ stage: "PUT dispatched", path, body: request.postDataJSON() });
        }
      });
      pattern = (url) => url.pathname === path || url.pathname === identifierPath;
      handler = async (route) => {
        const request = route.request();
        const requestPath = new URL(request.url()).pathname;
        if (request.method() === "GET") {
          if (requestPath === path) uuidReadCount++;
          const coldEntry = requestPath === identifierPath || (entry === "UUID" && uuidReadCount === 1);
          if (coldEntry) {
            const response = await route.fetch();
            expect(response.status()).toBe(200);
            const body = await response.json();
            expect(body).toMatchObject({ id: issue.id, status: "todo" });
            if (requestPath === identifierPath) identifierReadCaptured = true;
            events.push({ stage: "cold entry Todo response", path: requestPath, body });
            await route.fulfill({ response });
            return;
          }
          // The same endpoint also feeds the reactions query. Hold every
          // natural pre-write read so a reactions response cannot stand in
          // for the detail observer whose publication this test exercises.
          if (writes.length === 0) {
            const heldRead = { request, captured: false, handled: Promise.resolve() };
            backgroundReads.push(heldRead);
            events.push({ stage: "background UUID GET started", path: requestPath, uuidReadCount });
            if (entry === "identifier") expect(identifierReadCaptured).toBe(true);
            heldRead.handled = (async () => {
              const response = await route.fetch();
              expect(response.status()).toBe(200);
              const body = await response.json();
              expect(body).toMatchObject({ id: issue.id, status: "todo" });
              events.push({ stage: "real old Todo response captured", path: requestPath, body });
              heldRead.captured = true;
              await readReleased;
              await route.fulfill({ response });
              events.push({ stage: "old Todo GET fulfilled", path: requestPath });
            })();
            await heldRead.handled;
            return;
          }
        }
        if (request.method() === "PUT" && requestPath === path && !firstWriteRoute) {
          firstWriteRoute = (async () => {
            await writeReleased;
            await route.fulfill({
              status: 503,
              contentType: "application/json",
              body: JSON.stringify({ error: "Status update unavailable" }),
            });
          })();
          await firstWriteRoute;
          return;
        }
        await route.continue();
      };
      await page.route(pattern, handler);
      await page.goto(`/${workspace.slug}/issues/${entry === "UUID" ? issue.id : issue.identifier}`, {
        waitUntil: "domcontentloaded",
      });
      await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();
      await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/issues/${issue.identifier}$`));
      await expect.poll(() => backgroundReads.some((read) => read.captured), { message: "Natural background UUID reads must follow the successful cold entry" }).toBe(true);
      expect(writes).toHaveLength(0);

      await chooseStatus(page);
      await expect.poll(() => writes.length).toBe(1);
      expect(writes[0]!.postDataJSON()).toMatchObject({ status: "in_progress" });
      await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
      expect(await api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id, status: "todo" });
      events.push({ stage: "closed picker shows In Progress with PUT pending" });

      await expect.poll(() => backgroundReads.every((read) => read.captured)).toBe(true);
      const oldReadResponses = backgroundReads.map((read) => page.waitForResponse((response) => response.request() === read.request));
      releaseRead();
      for (const response of oldReadResponses) expect(await (await response).finished()).toBeNull();
      await Promise.all(backgroundReads.map((read) => read.handled));
      // Wait for the completed response to reach a native paint before reading
      // the real control; an immediate assertion could beat Query publication.
      await page.evaluate(() => new Promise<void>((done) => {
        requestAnimationFrame(() => requestAnimationFrame(() => done()));
      }));
      events.push({ stage: "old GET body finished and painted", visibleStatus: await page.getByRole("button", { name: /^(Todo|In Progress)$/, exact: true }).innerText() });
      await expect(page.locator("button[data-picker-item]").filter({ hasText: /^In Progress$/ })).toBeHidden();
      await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
      expect(writes).toHaveLength(1);
      expect(await api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id, status: "todo" });

      releaseWrite();
      await expect(page.getByText("Status update unavailable", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();
      expect(await api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id, status: "todo" });
      await firstWriteRoute;
      await page.unroute(pattern, handler);
      const [retried] = await Promise.all([
        page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === "PUT"),
        chooseStatus(page),
      ]);
      expect(retried.ok()).toBe(true);
      expect(await api.requestJSON<Issue>(path)).toMatchObject({ id: issue.id, status: "in_progress" });
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();
      expect(writes).toHaveLength(2);
    } finally {
      releaseRead();
      releaseWrite();
      await info.attach("background-status-events", {
        body: JSON.stringify({ entry, uuidReadCount, identifierReadCaptured, heldBackgroundReads: backgroundReads.length, capturedBackgroundReads: backgroundReads.filter((read) => read.captured).length, events }, null, 2),
        contentType: "application/json",
      });
      try {
        await Promise.all([...backgroundReads.map((read) => read.handled), firstWriteRoute]);
      } finally {
        try {
          if (pattern && handler) await page.unroute(pattern, handler);
        } finally {
          await api.deleteFeatureWorkspace(workspace.id);
        }
      }
    }
  });
}
