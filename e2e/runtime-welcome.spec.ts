import { randomUUID } from "node:crypto";
import { writeFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("empty runtimes introduce connection paths and yield to registered computers", async ({ page }, info) => {
  test.setTimeout(180_000);
  const api = new TestApiClient();
  const run = `${Date.now().toString(36)}-${info.parallelIndex}`;
  await api.login(`e2e-runtime-welcome-${run}@multica.ai`, "Runtime welcome tester");
  const workspace = await api.ensureWorkspace("Runtime welcome", `runtime-welcome-${run}`);
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));

  try {
    await api.markUserOnboarded();
    const token = api.getToken();
    if (!token) throw new Error("Fixture login failed");
    const origin = info.project.use.baseURL;
    if (typeof origin !== "string") throw new Error("Playwright baseURL is required");
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "light");
    }, token);

    const layouts = [];
    for (const locale of ["en", "zh-Hans"] as const) {
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: origin }]);
      await page.goto(`/${workspace.slug}/runtimes`);
      const welcome = page.getByRole("region", {
        name: locale === "en" ? "Give your agents a computer to work on" : "给智能体一台干活的电脑",
        exact: true,
      });
      await expect(welcome).toBeVisible();
      for (const theme of ["light", "dark"] as const) {
        await page.evaluate((value) => {
          document.documentElement.classList.remove("light", "dark");
          document.documentElement.classList.add(value);
          document.documentElement.style.colorScheme = value;
        }, theme);
        for (const viewport of [
          { width: 1440, height: 900 },
          { width: 1024, height: 768 },
          { width: 390, height: 844 },
          { width: 320, height: 568 },
        ]) {
          await page.setViewportSize(viewport);
          await welcome.evaluate((element) => { element.scrollTop = 0; });
          await page.evaluate(() => document.fonts.ready);
          const add = welcome.getByRole("button", { name: locale === "en" ? "Add runtime" : "添加运行时", exact: true });
          await expect(add).toBeInViewport();
          const metrics = await welcome.evaluate((element) => {
            const card = element.querySelector('[role="group"][data-active="true"]');
            if (!card) throw new Error("Runtime connection example missing");
            return {
              documentWidth: document.documentElement.scrollWidth,
              viewportWidth: innerWidth,
              scrollWidth: element.scrollWidth,
              clientWidth: element.clientWidth,
              cardRadius: getComputedStyle(card).borderTopLeftRadius,
              visibleSlides: element.querySelectorAll('[role="group"]:not([inert])').length,
            };
          });
          layouts.push({ locale, theme, ...viewport, ...metrics });
          expect(metrics.documentWidth).toBeLessThanOrEqual(viewport.width);
          expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.clientWidth + 1);
          expect(parseFloat(metrics.cardRadius)).toBeGreaterThan(0);
          expect(metrics.visibleSlides).toBe(1);
          await page.screenshot({ path: info.outputPath(`${locale}-${viewport.width}-${theme}.png`), animations: "disabled" });
          if (viewport.width <= 390) {
            const next = welcome.getByRole("button", { name: locale === "en" ? "Next example" : "下一个示例", exact: true });
            await next.scrollIntoViewIfNeeded();
            await expect(next).toBeInViewport();
            await page.screenshot({ path: info.outputPath(`${locale}-${viewport.width}-${theme}-examples.png`), animations: "disabled" });
          }
        }
      }
    }
    const layoutPath = info.outputPath("layout-metrics.json");
    await writeFile(layoutPath, JSON.stringify(layouts, null, 2));
    await info.attach("layout metrics", { path: layoutPath, contentType: "application/json" });

    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: origin }]);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${workspace.slug}/runtimes`);
    await page.evaluate(() => {
      document.documentElement.classList.remove("dark");
      document.documentElement.classList.add("light");
      document.documentElement.style.colorScheme = "light";
    });
    const welcome = page.getByRole("region", { name: "Give your agents a computer to work on", exact: true });
    const examples = welcome.getByRole("region", { name: "Ways to connect a runtime", exact: true });
    const previous = examples.getByRole("button", { name: "Previous example", exact: true });
    const next = examples.getByRole("button", { name: "Next example", exact: true });
    await expect(previous).toBeDisabled();
    await next.focus();
    await page.keyboard.press("Enter");
    await expect(examples.getByRole("heading", { name: "Another computer, guided setup" })).toBeVisible();
    await expect(next).toBeDisabled();
    await expect(next).toBeFocused();
    await expect(examples.getByRole("status")).toHaveText("Example 2 of 2: Another computer, guided setup");
    await expect(examples.getByRole("group", { name: "This computer, connected automatically", includeHidden: true })).toHaveAttribute("inert");
    await page.screenshot({ path: info.outputPath("remote-example.png"), animations: "disabled" });
    await previous.focus();
    await page.keyboard.press("Enter");
    await expect(examples.getByRole("heading", { name: "This computer, connected automatically" })).toBeVisible();
    await expect(previous).toBeDisabled();
    await expect(previous).toBeFocused();

    await page.emulateMedia({ reducedMotion: "reduce" });
    await next.click();
    const transition = await examples.getByRole("group", { name: "Another computer, guided setup" }).evaluate((element) => {
      if (!element.parentElement) throw new Error("Carousel track missing");
      return getComputedStyle(element.parentElement).transitionDuration;
    });
    expect(transition).toBe("0s");
    await previous.click();

    const guide = welcome.getByRole("link", { name: "Read the connection guide", exact: true });
    await expect(guide).toHaveAttribute("href", `/${workspace.slug}/docs/daemon-runtimes`);
    await guide.click();
    await expect(page).toHaveURL(`/${workspace.slug}/docs/daemon-runtimes`);
    await expect(page.getByRole("heading", { name: "守护进程与运行时", exact: true })).toBeVisible();
    await page.goBack();

    const add = welcome.getByRole("button", { name: "Add runtime", exact: true });
    await add.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog", { name: "Add a computer", exact: true });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("tab", { name: "Linux", exact: true }).click();
    await expect(dialog.getByRole("button", { name: "Copy Linux installation command", exact: true })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog).not.toBeVisible();
    await expect(add).toBeFocused();

    // Real registration emits daemon:register and exercises the existing scoped
    // inventory invalidation. No daemon process, agent CLI or model is started.
    const daemonId = randomUUID();
    const registration = await api.requestJSON<{ runtimes: { id: string }[] }>("/api/daemon/register", {
      method: "POST",
      body: {
        workspace_id: workspace.id,
        daemon_id: daemonId,
        device_name: "Welcome acceptance computer",
        cli_version: "0.4.40-test",
        runtimes: [{ name: "Codex (Welcome acceptance computer)", type: "codex", version: "fixture-1", status: "online" }],
      },
    });
    expect(registration.runtimes).toHaveLength(1);
    await expect(welcome).not.toBeVisible();
    const machine = page.getByRole("link", { name: /Welcome acceptance computer/ }).filter({ hasText: "Online" });
    await expect(machine).toBeVisible();
    await expect(machine).toHaveAttribute("href", `/${workspace.slug}/runtimes/${encodeURIComponent(`local:${daemonId}`)}`);
    await page.screenshot({ path: info.outputPath("populated-list.png"), animations: "disabled" });

    await api.requestJSON("/api/daemon/deregister", { method: "POST", body: { runtime_ids: registration.runtimes.map((runtime) => runtime.id) } });
    await expect.poll(async () => {
      const inventory = await api.requestJSON<{ id: string; status: string }[]>("/api/runtimes?owner=me");
      return inventory.find((runtime) => runtime.id === registration.runtimes[0]?.id)?.status;
    }).toBe("offline");
    // Freshly deregistered computers retain the established five-minute
    // Recently lost display before changing to the Offline health label.
    const offlineMachine = page.getByRole("link", { name: /Welcome acceptance computer/ }).filter({ hasText: "Recently lost" });
    await expect(offlineMachine).toBeVisible();
    await expect(welcome).not.toBeVisible();
    await offlineMachine.click();
    await expect(page).toHaveURL(`/${workspace.slug}/runtimes/${encodeURIComponent(`local:${daemonId}`)}`);
    expect(pageErrors).toEqual([]);
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
