import { writeFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import { p1Session } from "../../../../../../e2e/fixtures/project-p1";

test("verify the project welcome in the production Web renderer", async ({ page }, info) => {
  const { api, workspace } = await p1Session(page, "zh-Hans");
  const origin = info.project.use.baseURL;
  if (typeof origin !== "string") throw new Error("Task-owned Web origin is required");
  const errors: string[] = [];
  const exampleWrites: string[] = [];
  const evidence: Record<string, unknown>[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    if (request.method() !== "GET" && /\/api\/(projects|issues|agents|squads)(\/|\?|$)/.test(request.url())) {
      exampleWrites.push(`${request.method()} ${new URL(request.url()).pathname}`);
    }
  });
  const capture = async (name: string) => {
    await page.evaluate(() => document.fonts.ready);
    await page.screenshot({ path: info.outputPath(`${name}.png`), animations: "disabled" });
  };

  try {
    await page.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: origin }]);
    await page.setViewportSize({ width: 1440, height: 900 });
    for (const reference of [
      { route: "triage", title: "先收集，再决定做什么" },
      { route: "iterations", title: "让每次迭代都有共同目标" },
    ]) {
      await page.goto(`/${workspace.slug}/${reference.route}`);
      await expect(page.getByRole("heading", { name: reference.title, exact: true })).toBeVisible();
      await capture(`reference-${reference.route}-zh-desktop`);
    }

    for (const locale of ["zh-Hans", "en"] as const) {
      const chinese = locale === "zh-Hans";
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: origin }]);
      await page.goto(`/${workspace.slug}/projects`);
      const title = chinese
        ? "围绕一个目标，组织任务、智能体与 AI 小队"
        : "Bring issues, agents and AI squads together around a shared goal";
      const welcome = page.getByRole("region", { name: title, exact: true });
      const createName = chinese ? "创建第一个项目" : "Create your first project";
      const docsName = chinese ? "了解项目" : "Learn about projects";
      const create = welcome.getByRole("button", { name: createName, exact: true });
      const docs = welcome.getByRole("link", { name: docsName, exact: true });
      await expect(welcome).toBeVisible();
      await expect(page.getByRole("button", { name: createName, exact: true })).toHaveCount(1);
      await expect(welcome.getByRole("heading", { name: chinese ? "注册激活" : "Registration and activation", exact: true })).toBeVisible();
      await expect(docs).toHaveAttribute("href", `/${workspace.slug}/docs/projects`);
      await expect(welcome.locator("figure").locator("a,button,input,[tabindex='0']")).toHaveCount(0);
      await expect(welcome.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "33");

      for (const theme of ["light", "dark"] as const) {
        await page.evaluate((value) => {
          document.documentElement.classList.remove("light", "dark");
          document.documentElement.classList.add(value);
          document.documentElement.style.colorScheme = value;
          localStorage.setItem("theme", value);
        }, theme);
        for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
          await page.setViewportSize(viewport);
          await welcome.evaluate((element) => { element.scrollTop = 0; });
          await welcome.evaluate(async (element) => {
            await Promise.all(element.getAnimations({ subtree: true })
              .filter((animation) => animation.effect?.getComputedTiming().endTime !== Infinity)
              .map((animation) => animation.finished.catch(() => undefined)));
          });
          await expect(create).toBeInViewport();
          await expect(docs).toBeInViewport();
          const metrics = await welcome.evaluate((element) => {
            const layout = element.firstElementChild;
            const figure = element.querySelector("figure");
            const button = element.querySelector("button");
            if (!layout || !figure || !button) throw new Error("Welcome composition is incomplete");
            const canvas = document.createElement("canvas");
            canvas.width = canvas.height = 1;
            const context = canvas.getContext("2d", { willReadFrequently: true });
            if (!context) throw new Error("Contrast check needs a canvas");
            const rgba = (value: string) => {
              context.clearRect(0, 0, 1, 1);
              context.fillStyle = value;
              context.fillRect(0, 0, 1, 1);
              return [...context.getImageData(0, 0, 1, 1).data].map((channel, index) => index === 3 ? channel / 255 : channel);
            };
            const compose = (top: number[], bottom: number[]) => [
              ...top.slice(0, 3).map((channel, index) => channel * top[3]! + bottom[index]! * (1 - top[3]!)), 1,
            ];
            const luminance = (color: number[]) => color.slice(0, 3).map((channel) => {
              const normalized = channel / 255;
              return normalized <= 0.04045 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4;
            }).reduce((sum, channel, index) => sum + channel * [0.2126, 0.7152, 0.0722][index]!, 0);
            const contrasts = [...element.querySelectorAll("p,figcaption,h2,h3,button")].map((node) => {
              const ancestry: Element[] = [];
              for (let current: Element | null = node; current; current = current.parentElement) ancestry.unshift(current);
              const background = ancestry.reduce((color, ancestor) => compose(rgba(getComputedStyle(ancestor).backgroundColor), color), [255, 255, 255, 1]);
              const foreground = compose(rgba(getComputedStyle(node).color), background);
              const foregroundLight = luminance(foreground);
              const backgroundLight = luminance(background);
              return {
                text: node.textContent?.trim().slice(0, 80),
                fontSize: parseFloat(getComputedStyle(node).fontSize),
                fontWeight: getComputedStyle(node).fontWeight,
                ratio: Number(((Math.max(foregroundLight, backgroundLight) + 0.05) / (Math.min(foregroundLight, backgroundLight) + 0.05)).toFixed(2)),
              };
            });
            return {
              overflow: document.documentElement.scrollWidth > innerWidth + 1 || element.scrollWidth > element.clientWidth + 1,
              buttonHeight: button.getBoundingClientRect().height,
              columns: getComputedStyle(layout).gridTemplateColumns,
              figureWidth: figure.getBoundingClientRect().width,
              contrasts,
            };
          });
          evidence.push({ locale, theme, ...viewport, ...metrics });
          await writeFile(info.outputPath("layout-metrics.json"), JSON.stringify(evidence, null, 2) + "\n");
          expect(metrics.overflow).toBe(false);
          expect(metrics.buttonHeight).toBeGreaterThanOrEqual(44);
          expect(metrics.contrasts.filter(({ ratio, fontSize, fontWeight }) =>
            ratio < (fontSize >= 24 || (fontSize >= 18.67 && Number(fontWeight) >= 700) ? 3 : 4.5),
          )).toEqual([]);
          await capture(`project-welcome-${locale}-${theme}-${viewport.width}`);
          if (viewport.width === 390) {
            await welcome.locator("figcaption").scrollIntoViewIfNeeded();
            await expect(welcome.locator("figcaption")).toBeInViewport();
            await capture(`project-welcome-${locale}-${theme}-example-narrow`);
          }
        }
      }
    }

    expect(exampleWrites).toEqual([]);
    expect(errors).toEqual([]);
    await writeFile(info.outputPath("evidence.json"), JSON.stringify({
      renderer: "production Web / Chromium", workspace: workspace.slug,
      checks: { passiveExample: true, noOverflow: true, contrast: true, pageErrors: errors },
      views: evidence,
    }, null, 2) + "\n");
  } finally {
    await api.deleteFeatureWorkspace(workspace.id);
  }
});
