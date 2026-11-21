import { expect, test } from "@playwright/test";
import { p1Capture, p1NoOverflow, p1Session } from "./fixtures/project-p1";

test("an empty iteration catalogue introduces planning on desktop and mobile", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page);
  try {
    const base = `/api/workspaces/${workspace.id}`;
    expect(await api.requestJSON<{ items: unknown[] }>(`${base}/iterations`)).toMatchObject({ items: [] });

    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/${workspace.slug}/iterations`);
    const welcomeTitle = "Give each iteration a shared goal";
    const welcome = page.getByRole("region", { name: welcomeTitle, exact: true });
    await expect(welcome.getByRole("heading", { name: welcomeTitle, exact: true })).toBeVisible();
    await expect(welcome.getByRole("button", { name: "Create iteration", exact: true })).toBeVisible();
    await expect(welcome.getByRole("link", { name: "Iteration settings", exact: true })).toHaveAttribute("href", `/${workspace.slug}/settings?tab=iterations`);
    await expect(page.getByRole("region", { name: "Example", exact: true })).toBeVisible();
    await p1NoOverflow(page);
    await p1Capture(page, info, "iteration-welcome-en-desktop");

    await api.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    const origin = info.project.use.baseURL;
    if (typeof origin !== "string") throw new Error("Playwright base URL is required");
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: origin }]);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/${workspace.slug}/iterations`);

    const mobileWelcome = page.getByRole("region", { name: "让每次迭代都有共同目标", exact: true });
    await expect(mobileWelcome.getByRole("button", { name: "创建迭代", exact: true })).toBeVisible();
    await expect(mobileWelcome.getByRole("link", { name: "迭代设置", exact: true })).toBeVisible();
    await expect(page.getByRole("region", { name: "示例", exact: true })).toBeVisible();
    await p1NoOverflow(page);
    const dimensions = await mobileWelcome.evaluate((element) => ({
      documentWidth: document.documentElement.scrollWidth,
      viewportWidth: innerWidth,
      contentWidth: element.scrollWidth,
      clientWidth: element.clientWidth,
    }));
    expect(dimensions.documentWidth).toBeLessThanOrEqual(dimensions.viewportWidth);
    expect(dimensions.contentWidth).toBeLessThanOrEqual(dimensions.clientWidth + 1);
    await p1Capture(page, info, "iteration-welcome-zh-mobile");
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
