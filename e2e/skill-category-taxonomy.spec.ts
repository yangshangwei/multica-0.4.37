import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { TestApiClient } from "./fixtures";

// Visual acceptance for the eight-category lifecycle taxonomy and the batch
// label menu. Component behaviour is covered by the views vitest suite; this
// spec exists to prove the real page renders all eight categories, their tone
// tokens and the tri-state label menu at wide, narrow and dark presentations.
//
// Nothing here is intercepted: categories round-trip through
// config.presentation.category and labels through the resource label endpoints.

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ||
  `http://localhost:${process.env.PORT || "8080"}`;
const APP_ORIGIN =
  process.env.PLAYWRIGHT_BASE_URL ||
  process.env.FRONTEND_ORIGIN ||
  "http://localhost:3000";
const WORKER =
  process.env.TEST_PARALLEL_INDEX ?? process.env.TEST_WORKER_INDEX ?? "0";
const RUN_ID =
  process.env.E2E_RUN_ID ??
  `${Date.now().toString(36)}-${process.pid.toString(36)}`;

// Display order is the contract: the first five trace planning -> release, the
// last three span stages. Mirrors SKILL_CATEGORIES.
const CATEGORIES = [
  "research",
  "design",
  "engineering",
  "quality",
  "operations",
  "writing",
  "data",
  "other",
] as const;

type Category = (typeof CATEGORIES)[number];

const EN_LABELS: Record<Category, string> = {
  research: "Planning & requirements",
  design: "Design & architecture",
  engineering: "Development & integration",
  quality: "Testing & quality",
  operations: "Release & operations",
  writing: "Collaboration & knowledge",
  data: "Data & automation",
  other: "General tools",
};

const ZH_LABELS: Record<Category, string> = {
  research: "需求与规划",
  design: "设计与架构",
  engineering: "开发与集成",
  quality: "测试与质量",
  operations: "发布与运维",
  writing: "协作与知识",
  data: "数据与自动化",
  other: "通用工具",
};

const WIDE = { width: 1440, height: 900 };
const NARROW = { width: 390, height: 844 };

interface SkillSeed {
  id: string;
  name: string;
  config: Record<string, unknown>;
}

async function capture(page: Page, testInfo: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, animations: "disabled" });
  await testInfo.attach(name, { path, contentType: "image/png" });
}

/**
 * Resolved tone of each category's icon tile, read from the presentation icon's
 * own `data-category` marker. Proves the tokens are distinct per category
 * rather than all collapsing onto the `other` fallback.
 */
async function categoryTones(page: Page): Promise<string[]> {
  const nav = page.getByRole("navigation", { name: "Categories" });
  const tones: string[] = [];
  for (const category of CATEGORIES) {
    const tile = nav.locator(`[data-category="${category}"]`).first();
    tones.push(
      await tile.evaluate((el) => {
        const style = getComputedStyle(el);
        return `${style.color}|${style.backgroundColor}`;
      }),
    );
  }
  return tones;
}

test("renders eight lifecycle categories and the batch label menu across presentations", async ({
  page,
}, testInfo) => {
  // A cold `next dev` compiles /skills on first navigation; keep that outside
  // the product assertion windows. Clicks get a bounded timeout of their own:
  // Playwright actions otherwise inherit the whole test budget, so one selector
  // that never resolves costs the entire run instead of naming itself.
  test.setTimeout(600_000);
  page.setDefaultTimeout(20_000);
  const api = new TestApiClient();
  const slug = `e2e-skill-tax-${WORKER}-${RUN_ID}`;
  await api.login(
    `e2e-skill-tax-${WORKER}-${RUN_ID}@multica.ai`,
    "E2E Skill Taxonomy User",
  );
  const workspace = await api.ensureWorkspace("E2E Skill Taxonomy QA", slug);
  expect(workspace.slug).toBe(slug);
  const token = api.getToken();
  if (!token) throw new Error("E2E login did not return a token");

  async function request(endpoint: string, init?: RequestInit) {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
        "X-Workspace-Slug": workspace.slug,
        ...init?.headers,
      },
    });
    if (!response.ok) {
      const body = await response.text().catch(() => "<unreadable>");
      expect(
        response.ok,
        `${init?.method ?? "GET"} ${endpoint} -> ${response.status} ${body}`,
      ).toBeTruthy();
    }
    return response;
  }

  /** Server-side language plus the SSR locale cookie, set before navigation. */
  async function useLocale(locale: "en" | "zh-Hans") {
    await request("/api/me", {
      method: "PATCH",
      body: JSON.stringify({ language: locale }),
    });
    await page.context().addCookies([
      { name: "multica-locale", value: locale, url: APP_ORIGIN },
      { name: "multica_logged_in", value: "1", url: APP_ORIGIN },
    ]);
  }

  try {
    await api.markUserOnboarded();

    // theme=system so `emulateMedia` drives light/dark. An init script that
    // pinned an explicit theme would be re-applied on every reload and would
    // silently undo the dark-mode switch.
    await page.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("theme", "system");
    }, token);
    await page.emulateMedia({ colorScheme: "light" });
    await useLocale("en");

    // One skill per category, plus two extras so counts are not all 1 and the
    // sidebar/chips numbers are actually readable in the screenshots.
    const seeds: { name: string; category: Category }[] = [
      ...CATEGORIES.map((category) => ({
        name: `${category}-primary-${RUN_ID}`,
        category,
      })),
      { name: `engineering-extra-${RUN_ID}`, category: "engineering" as Category },
      { name: `quality-extra-${RUN_ID}`, category: "quality" as Category },
    ];

    const created: SkillSeed[] = [];
    for (const seed of seeds) {
      const config =
        seed.category === "other"
          ? {}
          : { presentation: { category: seed.category } };
      const skill: SkillSeed = await (
        await request("/api/skills", {
          method: "POST",
          body: JSON.stringify({
            name: seed.name,
            description: `Seeded ${seed.category} skill for taxonomy acceptance.`,
            content: `# ${seed.name}\n\nSeeded for visual acceptance.\n`,
            config,
          }),
        })
      ).json();
      created.push(skill);
    }

    // The category must survive the write/read round-trip untouched, otherwise
    // the screenshots would only prove the client's optimistic guess.
    const listed: { id: string; name: string; config: Record<string, unknown> }[] =
      await (await request("/api/skills")).json();
    expect(listed).toHaveLength(seeds.length);
    for (const seed of seeds) {
      const row = listed.find((item) => item.name === seed.name);
      expect(row, seed.name).toBeDefined();
      if (seed.category === "other") {
        expect(row!.config).not.toHaveProperty("presentation");
      } else {
        expect(row!.config).toMatchObject({
          presentation: { category: seed.category },
        });
      }
    }

    // Three skill labels: one on both selected rows (all), one on a single
    // selected row (some), one on neither (none) — so a single popover
    // screenshot shows every tri-state indicator.
    const labelNames = {
      all: `tax-all-${RUN_ID}`,
      some: `tax-some-${RUN_ID}`,
      none: `tax-none-${RUN_ID}`,
    };
    const labels: Record<string, { id: string; name: string }> = {};
    const colors = { all: "#6366f1", some: "#f59e0b", none: "#10b981" };
    for (const key of ["all", "some", "none"] as const) {
      labels[key] = await (
        await request("/api/labels", {
          method: "POST",
          body: JSON.stringify({
            resource_type: "skill",
            name: labelNames[key],
            color: colors[key],
          }),
        })
      ).json();
    }

    const first = created.find((s) => s.name === `design-primary-${RUN_ID}`)!;
    const second = created.find((s) => s.name === `quality-primary-${RUN_ID}`)!;
    for (const skill of [first, second]) {
      await request(`/api/skills/${skill.id}/labels`, {
        method: "POST",
        body: JSON.stringify({ label_id: labels.all!.id }),
      });
    }
    await request(`/api/skills/${first.id}/labels`, {
      method: "POST",
      body: JSON.stringify({ label_id: labels.some!.id }),
    });

    // ------------------------------------------------------------ wide/light
    await page.setViewportSize(WIDE);
    await page.goto(`/${slug}/skills`, {
      waitUntil: "domcontentloaded",
      timeout: 180_000,
    });

    const nav = page.getByRole("navigation", { name: "Categories" });
    await expect(nav).toBeVisible({ timeout: 180_000 });

    // Sidebar shows All + eight categories in contract order.
    await expect(nav.getByRole("button").first()).toContainText("All");
    const navText = (await nav.innerText()).replace(/\s+/g, " ");
    let cursor = navText.indexOf("All");
    expect(cursor).toBeGreaterThanOrEqual(0);
    for (const category of CATEGORIES) {
      const at = navText.indexOf(EN_LABELS[category], cursor);
      expect(at, `${category} follows the previous category`).toBeGreaterThan(cursor);
      cursor = at;
    }

    // Counts come from the facets, not from a hand-written list.
    for (const category of CATEGORIES) {
      const expected =
        category === "engineering" || category === "quality" ? "2" : "1";
      await expect(
        nav.getByRole("button", { name: new RegExp(`^${EN_LABELS[category]}`) }),
      ).toContainText(expected);
    }

    const tones = await categoryTones(page);
    expect(
      new Set(tones).size,
      `distinct category tones: ${tones.join(" / ")}`,
    ).toBe(CATEGORIES.length);
    await capture(page, testInfo, "wide-light-card-sidebar");

    // Switch to the list presentation and reveal the opt-in Labels column, both
    // through the real toolbar affordances rather than a seeded store.
    //
    // The display-settings trigger has no accessible name of its own: it shows
    // the active sort field ("Updated"), and the string "Display" lives only in
    // its tooltip. The Switch rows are Base UI buttons inside a <label>, and a
    // button is not a labelable element, so the row text is not the switch's
    // accessible name either — both have to be reached structurally.
    await page.getByRole("button", { name: "List view" }).click();
    // Scoped to the popover trigger: the column header's sort button carries
    // the same "Updated" name.
    await page
      .locator('[data-slot="popover-trigger"]')
      .filter({ hasText: "Updated" })
      .click();
    const displayPanel = page.locator('[data-slot="popover-content"]');
    await expect(displayPanel).toBeVisible();
    await displayPanel
      .locator("label")
      .filter({ hasText: "Labels" })
      .getByRole("switch")
      .click();
    await page.keyboard.press("Escape");
    await expect(displayPanel).toBeHidden();
    await expect(page.getByRole("columnheader", { name: "Labels" })).toBeVisible();
    await expect(page.getByRole("row")).toHaveCount(seeds.length + 1);
    await capture(page, testInfo, "wide-light-list");

    // Selecting a category filters the list and stays identifiable on hover.
    const designNav = nav.getByRole("button", {
      name: new RegExp(`^${EN_LABELS.design}`),
    });
    await designNav.click();
    await expect(designNav).toHaveAttribute("aria-current", "true");
    await designNav.hover();
    // Active state must survive hover: it lives on weight + text colour.
    await expect(designNav).toHaveAttribute("aria-current", "true");
    await expect(page.getByRole("row")).toHaveCount(2); // header + one match
    await expect(page.getByText(`design-primary-${RUN_ID}`)).toBeVisible();
    await expect(page.getByText(`quality-primary-${RUN_ID}`)).toHaveCount(0);
    await capture(page, testInfo, "wide-light-category-filtered");
    await nav.getByRole("button", { name: /^All/ }).click();
    await expect(page.getByRole("row")).toHaveCount(seeds.length + 1);

    // -------------------------------------------------------- batch label menu
    async function selectRowByName(name: string) {
      const row = page.getByRole("row").filter({ hasText: name }).first();
      await row.getByRole("button").first().click();
    }
    await selectRowByName(`design-primary-${RUN_ID}`);
    await selectRowByName(`quality-primary-${RUN_ID}`);
    await expect(page.getByText("2 selected")).toBeVisible();
    await capture(page, testInfo, "wide-light-selection-bar");

    const manageLabels = page.getByRole("button", { name: "Manage labels" });
    await manageLabels.click();
    const allRow = page.getByRole("button", { name: labelNames.all, exact: true });
    const someRow = page.getByRole("button", { name: labelNames.some, exact: true });
    const noneRow = page.getByRole("button", { name: labelNames.none, exact: true });
    await expect(allRow).toBeVisible();
    // Tri-state: on both -> pressed; on one -> partial; on neither -> off.
    await expect(allRow).toHaveAttribute("aria-pressed", "true");
    await expect(someRow).toHaveAttribute("aria-pressed", "false");
    await expect(noneRow).toHaveAttribute("aria-pressed", "false");
    await capture(page, testInfo, "wide-light-label-menu-tristate");

    // Picking the partial label promotes it to every editable selected row.
    await someRow.click();
    await expect(someRow).toHaveAttribute("aria-pressed", "true", {
      timeout: 30_000,
    });
    await capture(page, testInfo, "wide-light-label-menu-after-add");
    await page.keyboard.press("Escape");

    // The endpoint answers { labels: [...] }, not a bare array.
    const firstLabels: { labels: { name: string }[] } = await (
      await request(`/api/skills/${first.id}/labels`)
    ).json();
    const secondLabels: { labels: { name: string }[] } = await (
      await request(`/api/skills/${second.id}/labels`)
    ).json();
    expect(firstLabels.labels.map((l) => l.name).sort()).toEqual(
      [labelNames.all, labelNames.some].sort(),
    );
    expect(secondLabels.labels.map((l) => l.name).sort()).toEqual(
      [labelNames.all, labelNames.some].sort(),
    );

    // ------------------------------------------------------------- wide/dark
    await page.emulateMedia({ colorScheme: "dark" });
    await page.reload({ waitUntil: "domcontentloaded", timeout: 120_000 });
    await expect(nav).toBeVisible({ timeout: 60_000 });
    await expect(page.locator("html")).toHaveClass(/dark/);
    const darkTones = await categoryTones(page);
    expect(new Set(darkTones).size).toBe(CATEGORIES.length);
    // Dark tones are their own token values, not the light ones reused.
    expect(darkTones).not.toEqual(tones);
    await capture(page, testInfo, "wide-dark-list");

    await selectRowByName(`design-primary-${RUN_ID}`);
    await selectRowByName(`quality-primary-${RUN_ID}`);
    await page.getByRole("button", { name: "Manage labels" }).click();
    await expect(allRow).toBeVisible();
    await capture(page, testInfo, "wide-dark-label-menu");
    await page.keyboard.press("Escape");

    // ----------------------------------------------------------- narrow/light
    await page.emulateMedia({ colorScheme: "light" });
    await page.setViewportSize(NARROW);
    await page.reload({ waitUntil: "domcontentloaded", timeout: 120_000 });

    // The sidebar gives way to the scrollable chip row below @2xl.
    const chips = page.getByRole("group", { name: "Categories" });
    await expect(chips).toBeVisible({ timeout: 60_000 });
    await expect(nav).toBeHidden();
    for (const category of CATEGORIES) {
      await expect(
        chips.getByRole("button", { name: new RegExp(`^${EN_LABELS[category]}`) }),
      ).toHaveCount(1);
    }
    // Nine chips: All + eight categories, horizontally scrollable.
    await expect(chips.getByRole("button")).toHaveCount(CATEGORIES.length + 1);
    const overflows = await chips.evaluate(
      (el) => el.scrollWidth > el.clientWidth,
    );
    expect(overflows, "chip row scrolls rather than wrapping").toBe(true);
    await capture(page, testInfo, "narrow-light-chips");

    // The last chip must be reachable by scrolling, not clipped away.
    const lastChip = chips.getByRole("button", {
      name: new RegExp(`^${EN_LABELS.other}`),
    });
    await lastChip.scrollIntoViewIfNeeded();
    await expect(lastChip).toBeInViewport();
    await capture(page, testInfo, "narrow-light-chips-scrolled-end");

    await selectRowByName(`design-primary-${RUN_ID}`);
    await expect(page.getByText("1 selected")).toBeVisible();
    const manageNarrow = page.getByRole("button", { name: "Manage labels" });
    await expect(manageNarrow).toBeVisible();
    await capture(page, testInfo, "narrow-light-selection-bar");
    // Soft: a batch bar that overflows a phone viewport is a finding worth
    // reporting, not a reason to abandon the remaining evidence.
    const bounds = await manageNarrow.boundingBox();
    expect(bounds).not.toBeNull();
    expect.soft(bounds!.x, "selection bar left edge within viewport").toBeGreaterThanOrEqual(0);
    expect
      .soft(bounds!.x + bounds!.width, "selection bar right edge within viewport")
      .toBeLessThanOrEqual(NARROW.width);
    await manageNarrow.click();
    await expect(allRow).toBeVisible();
    await capture(page, testInfo, "narrow-light-label-menu");
    await page.keyboard.press("Escape");

    // -------------------------------------------- Chinese display names (wide)
    await useLocale("zh-Hans");
    await page.setViewportSize(WIDE);
    await page.goto(`/${slug}/skills`, {
      waitUntil: "domcontentloaded",
      timeout: 180_000,
    });
    const zhNav = page.getByRole("navigation", { name: "分类" });
    await expect(zhNav).toBeVisible({ timeout: 60_000 });
    const zhText = (await zhNav.innerText()).replace(/\s+/g, " ");
    let zhCursor = -1;
    for (const category of CATEGORIES) {
      const at = zhText.indexOf(ZH_LABELS[category], zhCursor + 1);
      expect(at, `${category} zh label in contract order`).toBeGreaterThan(zhCursor);
      zhCursor = at;
    }
    await capture(page, testInfo, "wide-light-sidebar-zh");

    await writeFile(
      testInfo.outputPath("verification.json"),
      JSON.stringify(
        {
          status: "passed",
          workspaceSlug: slug,
          categories: CATEGORIES,
          seededSkills: created.length,
          categoryRoundTripVerified: true,
          distinctLightTones: new Set(tones).size,
          distinctDarkTones: new Set(darkTones).size,
          darkTonesDifferFromLight: true,
          chipsCount: CATEGORIES.length + 1,
          chipRowScrolls: overflows,
          labelTriStateVerified: true,
          partialLabelPromotedToAllSelected: true,
          localesChecked: ["en", "zh-Hans"],
          presentations: ["wide-light", "wide-dark", "narrow-light"],
        },
        null,
        2,
      ),
    );
  } finally {
    await api.cleanup();
    await request(`/api/workspaces/${workspace.id}`, { method: "DELETE" });
  }
});
