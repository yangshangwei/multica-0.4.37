import { writeFileSync } from "node:fs";
import { test, expect, type Locator, type Page, type Response } from "@playwright/test";
import { legacyLaneMissing, loginLegacyOwner, openLegacySession, sql } from "./fixtures/lane-guards";

// Browser wiring/geometry only. The complete decision matrix belongs to
// packages/core/issues/quick-create-submit-state.test.ts. Fixtures use the real
// API and a registered runtime row with no daemon or paid agent behind it.
const quickCreatePath = "/api/issues/quick-create";
const recommendPath = "/api/issues/recommend-creators";
const descriptionReason = "Write a task description first";
const owners: Awaited<ReturnType<typeof loginLegacyOwner>>[] = [];
const responseFor = (response: Response, path: string) => new URL(response.url()).pathname === path && response.request().method() === "POST";

test.use({ viewport: { width: 1280, height: 900 }, locale: "en-US", trace: "retain-on-failure", screenshot: "only-on-failure" });
test.setTimeout(90_000);
test.skip(legacyLaneMissing() !== null, legacyLaneMissing() ?? "");
test.afterEach(async () => {
  for (const owner of owners.splice(0)) await owner.api.deleteFeatureWorkspace(owner.workspace.id);
});

async function fixture(page: Page, baseURL: string, options: { empty?: boolean; language?: "en" | "zh-Hans"; plainEnter?: boolean } = {}) {
  const owner = await loginLegacyOwner("blocked-feedback", "Blocked feedback acceptance");
  owners.push(owner);
  await openLegacySession(page, baseURL, owner.token, "agent");
  if (options.language === "zh-Hans") {
    await owner.api.requestJSON("/api/me", { method: "PATCH", body: { language: options.language } });
    await page.context().addCookies([{ name: "multica-locale", value: options.language, url: baseURL }]);
  }
  await page.addInitScript(({ language, plainEnter }) => {
    localStorage.setItem("multica-locale", language);
    localStorage.setItem("theme", "system");
    if (plainEnter) localStorage.setItem("multica_keyboard_shortcuts", JSON.stringify({ version: 2, state: { overrides: { send: { key: "Enter", modifiers: { primary: false, control: false, meta: false, alt: false, shift: false } } } } }));
  }, { language: options.language ?? "en", plainEnter: Boolean(options.plainEnter) });
  let runtime: { id: string; daemon_id: string } | undefined;
  let agent: { id: string; name: string } | undefined;
  if (!options.empty) {
    runtime = await owner.api.seedProjectRuntime();
    agent = await owner.api.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: { name: "Refund diagnostician", description: "Diagnose refund failures and verify fixes.", runtime_id: runtime.id, permission_mode: "private" } });
    await owner.api.requestJSON(`/api/agents/${agent.id}`, { method: "PUT", body: { autonomy_level: "contributor" } });
  }
  return { ...owner, runtime, agent };
}

function controls(page: Page, language: "en" | "zh-Hans" = "en") {
  const zh = language === "zh-Hans";
  const dialog = page.getByRole("dialog", { name: zh ? "快速创建任务" : "Quick create issue", exact: true });
  return {
    dialog,
    editor: dialog.getByRole("textbox", { name: zh ? "任务描述" : "Task description", exact: true }),
    trigger: dialog.getByRole("button", { name: zh ? /^交给(?!智能体$)/ : /^Send to (?!agent$)/ }),
    submit: dialog.getByRole("button", { name: zh ? "交给智能体" : "Send to agent", exact: true }),
    suggest: dialog.getByRole("button", { name: zh ? "帮我选智能体" : "Suggest an agent", exact: true }),
    refine: dialog.getByRole("button", { name: zh ? "帮我完善" : "Help me refine", exact: true }),
  };
}

async function open(page: Page, slug?: string, language: "en" | "zh-Hans" = "en") {
  if (slug) await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: language === "zh-Hans" ? "新建任务" : "New Issue", exact: true }).click();
  const ui = controls(page, language);
  await expect(ui.dialog).toBeVisible();
  await expect(ui.dialog).toHaveCSS("opacity", "1");
  await ui.dialog.evaluate(async dialog => { await Promise.all(dialog.getAnimations().map(animation => animation.finished)); });
  return ui;
}

function submissions(page: Page) {
  const calls: unknown[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === quickCreatePath && request.method() === "POST") calls.push(request.postDataJSON());
  });
  return calls;
}

// Playwright correctly treats aria-disabled as disabled for locator.click().
// Use a genuine pointer click to exercise the deliberately activatable control.
async function pointerActivate(page: Page, button: Locator) {
  const box = await button.boundingBox();
  if (!box) throw new Error("The activation target has no rendered box");
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
}
async function nativeEnabled(button: Locator) {
  expect(await button.evaluate(element => (element as HTMLButtonElement).disabled)).toBe(false);
}
async function submitWith(page: Page, target: Locator) {
  const response = page.waitForResponse(response => responseFor(response, quickCreatePath));
  await target.focus();
  await page.keyboard.press("ControlOrMeta+Enter");
  expect((await response).status()).toBe(202);
}

test("empty description remains in the Tab order, explains itself and routes blocked activation to the editor", async ({ page, baseURL }) => {
  const f = await fixture(page, baseURL!);
  const calls = submissions(page);
  const ui = await open(page, f.slug);
  await expect(ui.trigger).toContainText(f.agent!.name);
  await expect(ui.submit).toHaveAttribute("aria-disabled", "true");
  await nativeEnabled(ui.submit);
  await expect(ui.submit).toHaveAccessibleDescription(descriptionReason);
  await expect(ui.submit).not.toHaveAttribute("title");
  await ui.dialog.getByRole("switch").focus();
  await page.keyboard.press("Tab");
  await expect(ui.submit).toBeFocused();
  await expect(page.locator('[data-slot="tooltip-content"]')).toHaveText(descriptionReason);
  await ui.editor.focus();
  await ui.submit.hover();
  await expect(page.locator('[data-slot="tooltip-content"]')).toHaveText(descriptionReason);
  await pointerActivate(page, ui.submit);
  await expect(ui.editor).toBeFocused();
  await ui.dialog.getByRole("button", { name: "Close", exact: true }).focus();
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(ui.editor).toBeFocused();
  await expect(ui.editor).toHaveText("");
  expect(calls).toHaveLength(0);
});

test("clearing the editor and immediately submitting uses live Markdown", async ({ page, baseURL }) => {
  const f = await fixture(page, baseURL!);
  const calls = submissions(page);
  const ui = await open(page, f.slug);
  await ui.editor.fill("Delete this before handing it off.");
  await expect(ui.submit).toBeEnabled();
  const box = await ui.submit.boundingBox();
  if (!box) throw new Error("No submit geometry");
  await ui.editor.fill("");
  const clearedAt = await page.evaluate(() => performance.now());
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  expect(await page.evaluate(start => performance.now() - start, clearedAt)).toBeLessThan(150);
  await expect(ui.editor).toBeFocused();
  expect(calls).toHaveLength(0);
});

test("zero candidates has two working exits, keeps the draft and makes no recommendation request", async ({ page, baseURL }) => {
  const f = await fixture(page, baseURL!, { empty: true });
  const calls = submissions(page);
  let recommendations = 0;
  page.on("request", request => { if (new URL(request.url()).pathname === recommendPath) recommendations++; });
  let ui = await open(page, f.slug);
  const notice = ui.dialog.getByRole("status").filter({ hasText: "No agent or squad can take this" });
  await expect(notice).toBeVisible();
  await expect(ui.submit).toHaveAccessibleDescription("No agent or squad can take this");
  await nativeEnabled(ui.submit);
  await expect(ui.suggest).toHaveJSProperty("disabled", true);
  await expect(ui.suggest).toHaveAccessibleDescription("No agent or squad to suggest.");
  await ui.editor.fill("Keep this exact draft when configuring an agent.");
  await pointerActivate(page, ui.suggest);
  await pointerActivate(page, ui.submit);
  const setup = notice.getByRole("link", { name: "Set up an agent", exact: true });
  await expect(setup).toBeFocused();
  await ui.editor.press("ControlOrMeta+Enter");
  await expect(setup).toBeFocused();
  await setup.click();
  await expect(page).toHaveURL(new RegExp(`/${f.slug}/agents$`));
  await expect(ui.dialog).toBeHidden();
  ui = await open(page);
  await expect(ui.editor).toHaveText("Keep this exact draft when configuring an agent.");
  await ui.dialog.getByRole("status").filter({ hasText: "No agent or squad can take this" }).getByRole("button", { name: "Switch to Manual", exact: true }).click();
  const manual = page.getByRole("dialog", { name: "New Issue", exact: true });
  await expect(manual.locator('.ProseMirror.rich-text-editor[contenteditable="true"]').filter({ hasText: "Keep this exact draft" })).toHaveText("Keep this exact draft when configuring an agent.");
  expect(calls).toHaveLength(0);
  expect(recommendations).toBe(0);
});

for (const state of ["loading", "failed"] as const) {
  test(`${state} recipients routes activation to the persistent picker`, async ({ page, baseURL }) => {
    const f = await fixture(page, baseURL!);
    const calls = submissions(page);
    let release!: () => void;
    const gate = new Promise<void>(resolve => { release = resolve; });
    await page.route(url => url.pathname === "/api/agents", async route => {
      if (state === "loading") { await gate; await route.continue(); }
      else await route.fulfill({ status: 503, json: { error: "Controlled directory failure" } });
    });
    try {
      const ui = await open(page, f.slug);
      await ui.editor.fill("Explain why the refund processing failed.");
      const reason = state === "loading" ? "Still loading. Try again in a moment." : "Could not load the list. Open the picker to retry.";
      await expect(ui.submit).toHaveAccessibleDescription(reason, { timeout: 20_000 });
      await nativeEnabled(ui.submit);
      await pointerActivate(page, ui.submit);
      await expect(ui.trigger).toBeFocused();
      await ui.editor.press("ControlOrMeta+Enter");
      await expect(ui.trigger).toBeFocused();
      await expect(ui.dialog.getByText("No agent or squad can take this", { exact: true })).toHaveCount(0);
      expect(calls).toHaveLength(0);
    } finally { release(); await page.unrouteAll({ behavior: "wait" }); }
  });
}

test("autonomy and CLI version refusals keep their dedicated focus destinations", async ({ page, baseURL }) => {
  const f = await fixture(page, baseURL!);
  const calls = submissions(page);
  await f.api.requestJSON(`/api/agents/${f.agent!.id}`, { method: "PUT", body: { autonomy_level: "observer" } });
  await page.goto(`/${f.slug}/agents/${f.agent!.id}`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "Assign work", exact: true }).click();
  let ui = controls(page);
  await ui.editor.fill("Observe the refund failure.");
  await expect(ui.submit).toHaveAccessibleDescription("The selected creator cannot create issues. Create manually instead.");
  await pointerActivate(page, ui.submit);
  const handoff = ui.dialog.getByRole("button", { name: `Create manually and assign to ${f.agent!.name}`, exact: true });
  await expect(handoff).toBeFocused();
  await ui.editor.press("ControlOrMeta+Enter");
  await expect(handoff).toBeFocused();
  await ui.dialog.getByRole("button", { name: "Close", exact: true }).click();
  await f.api.requestJSON(`/api/agents/${f.agent!.id}`, { method: "PUT", body: { autonomy_level: "contributor" } });
  await sql("UPDATE agent_runtime SET metadata = metadata || '{\"cli_version\":\"0.1.0\"}'::jsonb WHERE id=$1", [f.runtime!.id]);
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "Assign work", exact: true }).click();
  ui = controls(page);
  await expect(ui.submit).toHaveAccessibleDescription("Daemon CLI must be ≥ 0.2.21");
  await pointerActivate(page, ui.submit);
  await expect(ui.trigger).toBeFocused();
  await ui.editor.press("ControlOrMeta+Enter");
  await expect(ui.trigger).toBeFocused();
  await expect(ui.editor).toHaveText("Observe the refund failure.");
  expect(calls).toHaveLength(0);
});

test("recommendation adoption submits from the restored picker focus", async ({ page, baseURL }) => {
  test.skip(!process.env.ACTOR_RECOMMENDATION_PROVIDER_URL, "Requires deterministic provider");
  const f = await fixture(page, baseURL!);
  const calls = submissions(page);
  const ui = await open(page, f.slug);
  await ui.editor.fill("Diagnose refund failures and verify fixes.");
  await ui.suggest.click();
  const use = ui.dialog.getByRole("button", { name: `Use ${f.agent!.name}`, exact: true });
  await expect(use).toHaveText("Use");
  await use.click();
  await expect(ui.trigger).toBeFocused();
  const response = page.waitForResponse(response => responseFor(response, quickCreatePath));
  await page.keyboard.press("ControlOrMeta+Enter");
  expect((await response).status()).toBe(202);
  await expect(ui.dialog).toBeHidden();
  expect(calls).toHaveLength(1);
  expect(calls[0]).toMatchObject({ agent_id: f.agent!.id, prompt: "Diagnose refund failures and verify fixes." });
});

for (const target of ["close", "property", "switch", "editor"] as const) {
  test(`modifier Enter sends exactly once from the ${target}`, async ({ page, baseURL }) => {
    const f = await fixture(page, baseURL!);
    const calls = submissions(page);
    const ui = await open(page, f.slug);
    await ui.editor.fill(`Hand off once from ${target}.`);
    await expect(ui.submit).toBeEnabled();
    const control = target === "close" ? ui.dialog.getByRole("button", { name: "Close", exact: true })
      : target === "property" ? ui.dialog.getByRole("button", { name: "No project", exact: true })
      : target === "switch" ? ui.dialog.getByRole("switch") : ui.editor;
    await submitWith(page, control);
    if (target !== "switch") await expect(ui.dialog).toBeHidden();
    await expect.poll(() => calls.length).toBe(1);
    const [row] = await sql<{ count: string }>("SELECT count(*)::text AS count FROM agent_task_queue WHERE agent_id=$1 AND context->>'type'='quick_create'", [f.agent!.id]);
    expect(row?.count).toBe("1");
  });
}

test("portal search and refinement answer inputs keep modifier Enter to themselves", async ({ page, baseURL }) => {
  const f = await fixture(page, baseURL!);
  const calls = submissions(page);
  const ui = await open(page, f.slug);
  await ui.editor.fill("Diagnose the refund failure.");
  await expect(ui.submit).toBeEnabled();
  await ui.trigger.click();
  const search = page.getByPlaceholder("Search names or responsibilities...", { exact: true });
  await search.fill("No matching recipient here");
  await search.press("ControlOrMeta+Enter");
  expect(calls).toHaveLength(0);
  await expect(search).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(search).toBeHidden();
  await expect(ui.dialog).toBeVisible();
  // Only the deterministic refinement response is injected; the real helper
  // creates its question textarea and the real dialog listener sees its key.
  await page.route("**/api/issues/optimize-description", route => route.fulfill({ status: 200, contentType: "text/event-stream", body: `event: done\ndata: ${JSON.stringify({ text: "Diagnose the refund failure.", questions: ["Which refund should be examined?"] })}\n\n` }));
  await ui.refine.click();
  const answer = ui.dialog.getByRole("textbox", { name: "Which refund should be examined?", exact: true });
  await answer.fill("The latest refund.");
  await answer.press("ControlOrMeta+Enter");
  await expect(answer).toBeFocused();
  await expect(ui.dialog).toBeVisible();
  expect(calls).toHaveLength(0);
});

test("plain Enter activates the focused Close button without handing off", async ({ page, baseURL }) => {
  const f = await fixture(page, baseURL!, { plainEnter: true });
  const calls = submissions(page);
  const ui = await open(page, f.slug);
  await ui.editor.fill("Keep this draft when closing with Enter.");
  await expect(ui.submit).toBeEnabled();
  await ui.dialog.getByRole("button", { name: "Close", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(ui.dialog).toBeHidden();
  expect(calls).toHaveLength(0);
});

test("pending submission and upload use native disabled", async ({ page, baseURL }) => {
  const f = await fixture(page, baseURL!);
  const calls = submissions(page);
  const ui = await open(page, f.slug);
  await ui.editor.fill("Diagnose the attached refund report.");
  await expect(ui.submit).toBeEnabled();
  let releaseUpload!: () => void;
  const uploading = new Promise<void>(resolve => { releaseUpload = resolve; });
  let releaseSubmit!: () => void;
  const submitting = new Promise<void>(resolve => { releaseSubmit = resolve; });
  try {
    await page.route("**/api/upload-file", async route => { await uploading; await route.continue(); });
    const uploadRequest = page.waitForRequest(request => new URL(request.url()).pathname === "/api/upload-file");
    await ui.dialog.locator('input[type="file"]').setInputFiles({ name: "refund.txt", mimeType: "text/plain", buffer: Buffer.from("synthetic refund report") });
    await uploadRequest;
    const uploadButton = ui.dialog.getByRole("button", { name: "Uploading…", exact: true });
    await expect(uploadButton).toHaveJSProperty("disabled", true);
    await ui.editor.press("ControlOrMeta+Enter");
    expect(calls).toHaveLength(0);
    releaseUpload();
    await expect(ui.submit).toBeEnabled();
    await page.route("**/api/issues/quick-create", async route => { await submitting; await route.continue(); });
    const accepted = page.waitForResponse(response => responseFor(response, quickCreatePath));
    await ui.submit.click();
    await expect(ui.dialog.getByRole("button", { name: "Sending…", exact: true })).toHaveJSProperty("disabled", true);
    await ui.editor.press("ControlOrMeta+Enter");
    expect(calls).toHaveLength(1);
    releaseSubmit();
    expect((await accepted).status()).toBe(202);
    await expect(ui.dialog).toBeHidden();
  } finally {
    releaseUpload();
    releaseSubmit();
    await page.unrouteAll({ behavior: "wait" });
  }
});

async function colorMetrics(button: Locator) {
  return button.evaluate(element => {
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = 1;
    const ctx = canvas.getContext("2d")!;
    const rgba = (value: string) => { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1); return Array.from(ctx.getImageData(0, 0, 1, 1).data); };
    const blend = (front: number[], back: number[], alpha: number) => front.slice(0, 3).map((v, i) => v * alpha + back[i]! * (1 - alpha));
    const ancestors: Element[] = [];
    for (let node: Element | null = element; node; node = node.parentElement) ancestors.unshift(node);
    let background = [255, 255, 255];
    let opacity = 1;
    for (const node of ancestors) {
      const style = getComputedStyle(node);
      opacity *= Number(style.opacity);
      const color = rgba(style.backgroundColor);
      background = blend(color, background, color[3]! / 255);
    }
    const style = getComputedStyle(element);
    const foreground = rgba(style.color);
    const rendered = blend(foreground, background, opacity * foreground[3]! / 255);
    const luminance = (rgb: number[]) => rgb.reduce((sum, value, index) => { const c = value / 255; return sum + [0.2126, 0.7152, 0.0722][index]! * (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4); }, 0);
    const fg = luminance(rendered); const bg = luminance(background);
    return { color: style.color, opacity, background, rendered, ratio: (Math.max(fg, bg) + 0.05) / (Math.min(fg, bg) + 0.05), box: element.getBoundingClientRect().toJSON() };
  });
}

for (const language of ["en", "zh-Hans"] as const) {
  for (const theme of ["light", "dark"] as const) {
    test(`real geometry and helper contrast: ${language}, ${theme}`, async ({ page, baseURL }, info) => {
      const f = await fixture(page, baseURL!, { language });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
      let ui = await open(page, f.slug, language);
      const metrics: unknown[] = [];
      for (const width of [1280, 375]) {
        await page.setViewportSize({ width, height: width === 375 ? 812 : 900 });
        await expect.poll(() => ui.dialog.evaluate(e => Math.round(e.getBoundingClientRect().width))).toBe(width === 375 ? 351 : 672);
        for (const state of ["empty", "ready"] as const) {
          if (state === "empty") {
            await ui.editor.focus();
            await page.keyboard.press("ControlOrMeta+A");
            await page.keyboard.press("Backspace");
            await expect(ui.editor).toHaveText("");
          } else await ui.editor.fill(language === "en" ? "Diagnose refund failures and verify fixes." : "请排查退款失败，并验证修复。");
          if (state === "empty") await expect(ui.suggest).toHaveJSProperty("disabled", true);
          else await expect(ui.suggest).toBeEnabled();
          await page.mouse.move(0, 0);
          await ui.dialog.evaluate(async dialog => { await Promise.all(dialog.getAnimations({ subtree: true }).filter(animation => animation.effect?.getComputedTiming().iterations !== Infinity).map(animation => animation.finished)); });
          await expect(ui.submit).toBeInViewport({ ratio: 1 });
          const helpers = await Promise.all([colorMetrics(ui.suggest), colorMetrics(ui.refine)]);
          expect.soft(helpers[0].color).toBe(helpers[1].color);
          expect.soft(helpers[0].opacity).toBe(helpers[1].opacity);
          for (const helper of helpers) expect.soft(helper.ratio).toBeGreaterThanOrEqual(3);
          const geometry = await ui.dialog.evaluate(dialog => {
            const recommendations = dialog.querySelector('[data-testid="creator-recommendations"]')!;
            const trigger = recommendations.querySelector('button[data-slot="popover-trigger"]')!;
            const hint = trigger.parentElement!.querySelector(":scope > p")!;
            const editor = dialog.querySelector('[contenteditable="true"]')!;
            const label = document.getElementById(editor.getAttribute("aria-labelledby")!)!;
            const r = trigger.getBoundingClientRect(); const h = hint.getBoundingClientRect(); const d = label.getBoundingClientRect();
            return { trigger: r.toJSON(), hint: h.toJSON(), description: d.toJSON(), hintToPicker: h.top - r.bottom, hintToDescription: d.top - h.bottom, border: getComputedStyle(trigger).borderTopWidth, dialog: dialog.getBoundingClientRect().toJSON(), viewport: { width: innerWidth, height: innerHeight }, documentWidth: document.documentElement.scrollWidth };
          });
          expect(Number.parseFloat(geometry.border)).toBeGreaterThan(0);
          expect.soft(geometry.hintToPicker).toBeLessThan(geometry.hintToDescription);
          expect(geometry.documentWidth).toBeLessThanOrEqual(width);
          expect(geometry.dialog.x).toBeGreaterThanOrEqual(0);
          expect(geometry.dialog.x + geometry.dialog.width).toBeLessThanOrEqual(width);
          const screenshot = `${language}-${theme}-${width}-${state}.png`;
          await page.screenshot({ path: info.outputPath(screenshot), animations: "disabled" });
          metrics.push({ language, theme, width, state, screenshot, helpers, geometry });
          writeFileSync(info.outputPath("geometry-and-contrast.json"), JSON.stringify(metrics, null, 2));
        }
      }
      await ui.dialog.getByRole("button", { name: language === "en" ? "Close" : "关闭", exact: true }).click();
      // A fresh empty workspace keeps the visual fixture independent of
      // archive WebSocket behavior, already covered by the actor lifecycle suite.
      const emptyPage = await page.context().browser()!.newPage({ viewport: { width: 1280, height: 900 } });
      try {
        const emptyFixture = await fixture(emptyPage, baseURL!, { language, empty: true });
        await emptyPage.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
        ui = await open(emptyPage, emptyFixture.slug, language);
        for (const width of [1280, 375]) {
          await emptyPage.setViewportSize({ width, height: width === 375 ? 812 : 900 });
          await expect(ui.dialog.getByRole("status").filter({ hasText: language === "en" ? "No agent or squad can take this" : "没有可以交给的智能体或AI小队" })).toBeVisible();
          await expect(ui.submit).toBeInViewport({ ratio: 1 });
          await expect(ui.editor).toBeInViewport();
          const screenshot = `${language}-${theme}-${width}-no-candidates.png`;
          await emptyPage.screenshot({ path: info.outputPath(screenshot), animations: "disabled" });
          metrics.push({ language, theme, width, state: "no-candidates", screenshot });
        }
      } finally {
        await emptyPage.close();
      }
      const file = info.outputPath("geometry-and-contrast.json");
      writeFileSync(file, JSON.stringify(metrics, null, 2));
      await info.attach("real-browser-geometry-and-contrast", { path: file, contentType: "application/json" });
    });
  }
}
