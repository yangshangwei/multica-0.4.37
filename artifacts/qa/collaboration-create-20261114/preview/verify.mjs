import { chromium, expect } from "@playwright/test";
import { createServer } from "node:http";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { extname, dirname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

const preview = dirname(fileURLToPath(import.meta.url));
const mode = process.argv[2] ?? "baseline";
const filter = process.argv[3] ? new Set(process.argv[3].split(",")) : null;
const build = JSON.parse(await readFile(resolve(preview, `${mode}-build.json`), "utf8"));
const artifact = resolve(preview, `../browser-${mode}`, filter ? "targeted" : ".");
await mkdir(artifact, { recursive: true });
const result = {
  mode, build, startedAt: new Date().toISOString(), driverPid: process.pid,
  scope: "Actual shared composer and CSS with isolated mock API/SSE and memory drafts",
  driverSha256: createHash("sha256").update(await readFile(fileURLToPath(import.meta.url))).digest("hex"),
  scenarios: {}, diagnostics: {}, failures: [], pageErrors: [], blockedRequests: [],
};
const types = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".svg": "image/svg+xml" };
const server = createServer(async (request, response) => {
  try {
    const path = new URL(request.url, "http://127.0.0.1").pathname;
    const file = resolve(build.output, `.${path === "/" ? "/index.html" : path}`);
    if (!file.startsWith(`${resolve(build.output)}${sep}`)) { response.writeHead(403); response.end(); return; }
    response.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream", "Cache-Control": "no-store" });
    response.end(await readFile(file));
  } catch { response.writeHead(404); response.end(); }
});
await new Promise((done) => server.listen(0, "127.0.0.1", done));
const origin = `http://127.0.0.1:${server.address().port}`;
result.listener = { origin, owned: true, pid: process.pid };
let browser;
const contexts = [];
const settle = (page) => page.waitForTimeout(350);
const escape = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

async function rectangle(locator) {
  return locator.evaluate((element) => {
    const box = element.getBoundingClientRect();
    let visible = box.width > 0 && box.height > 0 && box.left >= -1 && box.right <= innerWidth + 1 && box.top >= -1 && box.bottom <= innerHeight + 1;
    for (let parent = element.parentElement; parent; parent = parent.parentElement) {
      if (["BODY", "HTML"].includes(parent.tagName)) break;
      const css = getComputedStyle(parent), bounds = parent.getBoundingClientRect();
      if (/(hidden|clip|auto|scroll)/.test(css.overflowX)) visible &&= box.left >= bounds.left - 1 && box.right <= bounds.right + 1;
      if (/(hidden|clip|auto|scroll)/.test(css.overflowY)) visible &&= box.top >= bounds.top - 1 && box.bottom <= bounds.bottom + 1;
      if (css.position === "fixed") break;
    }
    return { width: box.width, height: box.height, left: box.left, right: box.right, top: box.top, bottom: box.bottom, visible };
  });
}

// A text node merely present in the DOM does not prove its complete identity or
// explanation is readable. Check the painted ranges against every clip owner.
async function textRanges(locator, text) {
  return locator.evaluate((root, text) => {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    let node;
    while ((node = walker.nextNode())) {
      const offset = node.textContent.indexOf(text);
      if (offset < 0) continue;
      const range = document.createRange();
      range.setStart(node, offset); range.setEnd(node, offset + text.length);
      const boxes = [...range.getClientRects()].map((box) => ({ width: box.width, height: box.height, left: box.left, right: box.right, top: box.top, bottom: box.bottom }));
      let visible = boxes.length > 0 && boxes.every((box) => box.width > 0 && box.height > 0 && box.left >= -1 && box.right <= innerWidth + 1 && box.top >= -1 && box.bottom <= innerHeight + 1);
      for (let parent = node.parentElement; parent; parent = parent.parentElement) {
        const css = getComputedStyle(parent), bounds = parent.getBoundingClientRect();
        if (css.display === "none" || css.visibility !== "visible" || Number(css.opacity) === 0 || css.clip !== "auto" || css.clipPath !== "none") visible = false;
        if (["BODY", "HTML"].includes(parent.tagName)) break;
        if (/(hidden|clip|auto|scroll)/.test(css.overflowX)) visible &&= boxes.every((box) => box.left >= bounds.left - 1 && box.right <= bounds.right + 1);
        if (/(hidden|clip|auto|scroll)/.test(css.overflowY)) visible &&= boxes.every((box) => box.top >= bounds.top - 1 && box.bottom <= bounds.bottom + 1);
        if (css.position === "fixed") break;
      }
      return { found: true, visible, lines: boxes.length, boxes };
    }
    return { found: false, visible: false, lines: 0, boxes: [] };
  }, text);
}

async function open(options = {}) {
  const size = { width: 722, height: 660, ...options };
  const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, hasTouch: size.width < 600, isMobile: size.width < 600, colorScheme: size.theme ?? "light" });
  contexts.push(context);
  const page = await context.newPage();
  page.setDefaultTimeout(5000);
  page.setDefaultNavigationTimeout(7000);
  page.on("pageerror", (error) => result.pageErrors.push(error.message));
  await page.route("**/*", async (route) => {
    if (new URL(route.request().url()).origin === origin) await route.continue();
    else { result.blockedRequests.push(route.request().url()); await route.abort(); }
  });
  await page.goto(`${origin}/?locale=${size.locale ?? "zh-Hans"}&longSource=1`);
  await page.waitForFunction(() => Boolean(window.fixture));
  await page.evaluate((theme) => document.documentElement.classList.toggle("dark", theme === "dark"), size.theme);
  const t = await page.evaluate(() => window.fixture.translations);
  await page.getByRole("button", { name: t.creation.entry, exact: true }).click();
  const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
  await expect(dialog).toBeVisible();
  await expect(page.getByLabel(t.creation.titleLabel, { exact: true })).toBeVisible();
  await settle(page);
  return { page, dialog, context, t, size };
}

async function selectRecipient(page, t, longName = false) {
  if (longName) await page.evaluate(() => {
    const name = "跨团队订单交付验收负责人 Alexandra Morgan ".repeat(2);
    window.fixture.accounts[0].name = name;
    window.fixture.accounts[1].name = name;
  });
  const account = await page.evaluate(() => window.fixture.accounts[1]);
  const trigger = page.getByRole("button", { name: t.creation.recipient, exact: true });
  await trigger.click();
  const list = page.getByRole("listbox", { name: t.creation.recipient, exact: true });
  const choice = list.getByRole("option", { name: new RegExp(escape(account.user_id)) });
  await expect(choice).toBeVisible();
  await choice.scrollIntoViewIfNeeded();
  const rowIdentity = await textRanges(choice, account.user_id);
  await choice.click();
  await expect(list).toHaveCount(0);
  await settle(page);
  return { account, trigger, rowIdentity };
}

async function scenario(name, run) {
  if (filter && !filter.has(name)) return;
  process.stdout.write(`Starting ${name}\n`);
  try { result.scenarios[name] = await run(); }
  catch (error) { result.failures.push({ scenario: name, error: String(error.stack ?? error) }); }
  process.stdout.write(`Finished ${name}\n`);
}

try {
  browser = await chromium.launch({ headless: true });
  result.engine = `Playwright Chromium ${browser.version()}`;
  const cases = mode === "baseline"
    ? [{ width: 722, height: 660, theme: "light" }, { width: 722, height: 660, theme: "dark" }, { width: 390, height: 740, theme: "light" }, { width: 320, height: 568, theme: "dark", locale: "en" }]
    : [722, 1280, 390, 320].flatMap((width) => ["light", "dark"].map((theme) => ({ width, height: width === 722 ? 660 : width === 1280 ? 900 : width === 390 ? 740 : 568, theme })))
      .concat(["light", "dark"].map((theme) => ({ width: 320, height: 568, theme, locale: "en" })));
  for (const options of cases) await scenario(`toolbar-${options.width}-${options.theme}-${options.locale ?? "zh-Hans"}`, async () => {
    const { page, dialog, context, t, size } = await open(options);
    try {
      const title = page.getByLabel(t.creation.titleLabel, { exact: true });
      const description = page.getByLabel(t.creation.description, { exact: true });
      await title.fill("明确跨团队订单交付与验收");
      await description.fill("共同确认交付范围、责任人与验收标准。");
      const selected = await selectRecipient(page, t, size.locale === "en");
      const identity = await textRanges(selected.trigger, selected.account.user_id);
      const initialPolicyHelp = await textRanges(dialog, t.policy.creationHint);
      // Body guidance may scroll on a small viewport. It must become readable
      // through the body's existing scroll owner, rather than a hidden tooltip.
      if (mode !== "baseline" && !initialPolicyHelp.visible) {
        await page.getByText(t.policy.creationHint, { exact: true }).scrollIntoViewIfNeeded();
        await settle(page);
      }
      const policyHelp = await textRanges(dialog, t.policy.creationHint);
      const statusText = t.creation.assignedTo.replace("{{name}}", selected.account.name);
      const independentStatus = dialog.locator('[role="status"]').filter({ hasText: statusText });
      const toolbar = page.locator("[data-collaboration-creation-properties]");
      const footer = page.locator("[data-collaboration-creation-actions]");
      const before = { identity, policyHelp, policyHelpInitiallyVisible: initialPolicyHelp.visible, independentStatus: await independentStatus.count(), recipient: await rectangle(selected.trigger), toolbar: await rectangle(toolbar), footer: await rectangle(footer), description: await rectangle(description), rowIdentity: selected.rowIdentity };
      await page.screenshot({ path: resolve(artifact, `toolbar-${size.width}-${size.theme}-${size.locale ?? "zh-Hans"}.png`) });
      if (mode === "baseline") return before;
      expect(identity.visible, "Complete selected account ID must be directly readable").toBe(true);
      expect(policyHelp.visible, "AI participation explanation must be directly readable").toBe(true);
      expect(selected.rowIdentity.visible, "Same-name account choice must expose its complete ID").toBe(true);
      expect(before.independentStatus).toBe(1);
      expect(before.toolbar.visible).toBe(true); expect(before.footer.visible).toBe(true);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
      if (size.width < 600) {
        for (const control of [selected.trigger, page.getByLabel(t.policy.title, { exact: true }), toolbar.getByRole("button", { name: new RegExp(escape(t.fields.initial)) })]) {
          const box = await rectangle(control); expect(box.height).toBeGreaterThanOrEqual(44); expect(box.width).toBeGreaterThanOrEqual(44);
        }
      }
      const toggle = toolbar.getByRole("button", { name: new RegExp(escape(t.fields.initial)) });
      await toggle.focus(); await page.keyboard.press("Enter");
      await expect(toggle).toHaveAttribute("aria-expanded", "true");
      const priority = page.getByLabel(t.workflow.priority, { exact: true });
      await priority.focus(); await priority.selectOption("high");
      await settle(page); expect((await rectangle(priority)).visible).toBe(true);
      await toggle.focus(); await page.keyboard.press("Enter");
      await expect(toggle).toHaveAttribute("aria-expanded", "false");
      await expect(priority).toBeHidden(); await expect(toggle).toBeFocused();
      const disclosure = { toggle: await rectangle(toggle), footer: await rectangle(footer), selectedPriorityRetained: true, focusReturnedToToggle: true };
      await dialog.getByRole("button", { name: t.creation.close, exact: true }).click();
      await expect(dialog).toHaveCount(0);
      await expect(page.getByRole("button", { name: t.creation.entry, exact: true })).toBeFocused();
      await page.getByRole("button", { name: t.creation.entry, exact: true }).click();
      await expect(description).toHaveValue("共同确认交付范围、责任人与验收标准。");
      await expect(page.getByRole("button", { name: t.creation.recipient, exact: true })).toContainText(selected.account.user_id);
      return { ...before, disclosure, draftAndIdentityRetainedOnReopen: true };
    } finally { await context.close(); }
  });

  if (mode !== "baseline") await scenario("intermediate-date-focus", async () => {
    const { page, context, t } = await open({ width: 520, height: 568, theme: "light" });
    try {
      const toolbar = page.locator("[data-collaboration-creation-properties]");
      const body = page.locator("[data-collaboration-creation-body]");
      await toolbar.getByRole("button", { name: new RegExp(escape(t.fields.initial)) }).click();
      const date = page.getByLabel(t.workflow.startDate, { exact: true });
      await date.focus(); await settle(page);
      const before = { toolbar: await rectangle(toolbar), body: await rectangle(body), date: await rectangle(date) };
      await date.fill("2026-11-18"); await settle(page);
      const after = { toolbar: await rectangle(toolbar), body: await rectangle(body), date: await rectangle(date), focused: await date.evaluate((element) => document.activeElement === element) };
      result.diagnostics["intermediate-date-focus"] = { before, after };
      await page.screenshot({ path: resolve(artifact, "intermediate-date-focus.png") });
      expect(before.date.visible).toBe(true);
      expect(after.toolbar.height, "Field summary must not add an outer toolbar row").toBeCloseTo(before.toolbar.height, 0);
      expect(after.date.visible, "The edited Start date must remain completely visible").toBe(true);
      expect(after.focused).toBe(true);
      return { before, after };
    } finally { await context.close(); }
  });

  if (mode !== "baseline") for (const theme of ["light", "dark"]) await scenario(`long-content-recovery-${theme}`, async () => {
    const { page, dialog, context, t } = await open({ width: 390, height: 740, theme });
    try {
      const title = page.getByLabel(t.creation.titleLabel, { exact: true });
      const description = page.getByLabel(t.creation.description, { exact: true });
      const footer = page.locator("[data-collaboration-creation-actions]");
      await title.fill("验证长文本、附件与创建失败恢复");
      await description.fill("  原始说明\n保留空白  ");
      const initialFooter = await rectangle(footer);
      await page.evaluate(() => window.fixture.seedAttachments(32));
      await expect(page.getByRole("checkbox")).toHaveCount(32);
      await settle(page);
      const filesFooter = await rectangle(footer);
      expect(filesFooter.height).toBeCloseTo(initialFooter.height, 0);
      await description.evaluate((element) => { window.originalDescription = element; });
      await page.getByRole("button", { name: t.creation.assist, exact: true }).click();
      await page.waitForFunction(() => window.fixture.aiState().length === 1);
      await page.evaluate(() => window.fixture.emitAI("text_delta", { text: "Provisional text stays out of the draft" }));
      await expect(description).toHaveValue("  原始说明\n保留空白  ");
      await page.getByRole("button", { name: t.creation.expand, exact: true }).click();
      await expect(dialog).toHaveAttribute("data-expanded", "true");
      const collapse = page.getByRole("button", { name: t.creation.collapse, exact: true });
      await collapse.focus(); await page.keyboard.press("Enter");
      await expect(page.getByRole("button", { name: t.creation.expand, exact: true })).toBeFocused();
      expect(await description.evaluate((element) => element === window.originalDescription)).toBe(true);
      expect((await page.evaluate(() => window.fixture.aiState()[0])).aborted).toBe(false);
      const longText = "验收标准：保留原有协作内容，并核对交付结果。\n".repeat(35);
      await page.evaluate((text) => window.fixture.emitAI("done", { text, questions: ["期望什么时候验收？"] }), longText);
      await expect(description).toHaveValue(longText);
      const answer = page.getByLabel("期望什么时候验收？", { exact: true });
      await answer.focus(); await answer.fill("下周三，双方共同验收。");
      await settle(page);
      const longState = { answer: await rectangle(answer), footer: await rectangle(footer), editor: await description.evaluate((element) => ({ client: element.clientHeight, scroll: element.scrollHeight })), formIdentityRetained: true, streamRetainedOnResize: true };
      expect(longState.answer.visible).toBe(true); expect(longState.footer.visible).toBe(true);
      expect(longState.editor.scroll).toBeLessThanOrEqual(longState.editor.client + 1);
      await page.screenshot({ path: resolve(artifact, `long-content-${theme}.png`) });
      const ai = await page.evaluate(() => window.fixture.assistTranslations);
      await page.getByRole("button", { name: ai.undo, exact: true }).click();
      await expect(description).toHaveValue("  原始说明\n保留空白  ");
      await page.evaluate(() => window.fixture.setMode("reject"));
      await page.getByRole("button", { name: t.creation.submit, exact: true }).click();
      const rejection = page.getByText(t.creation.rejected, { exact: true });
      await expect(rejection).toBeVisible(); await settle(page);
      const refusal = { rejection: await rectangle(rejection), footer: await rectangle(footer), files: await page.getByRole("checkbox").count() };
      expect(refusal.rejection.visible).toBe(true); expect(refusal.footer.visible).toBe(true); expect(refusal.files).toBe(32);
      await expect(title).toHaveValue("验证长文本、附件与创建失败恢复");
      await expect(description).toHaveValue("  原始说明\n保留空白  ");
      await page.screenshot({ path: resolve(artifact, `creation-refused-${theme}.png`) });
      return { initialFooter, filesFooter, longState, refusal, rawUndoPreserved: true };
    } finally { await context.close(); }
  });
} finally {
  await Promise.allSettled(contexts.map((context) => context.close()));
  if (browser) await browser.close();
  await new Promise((done) => server.close(done));
  result.browserClosed = true; result.listenerClosed = true; result.finishedAt = new Date().toISOString();
  await writeFile(resolve(artifact, "results.json"), JSON.stringify(result, null, 2) + "\n");
}
process.stdout.write(JSON.stringify({ mode, artifact, completed: Object.keys(result.scenarios).length, failures: result.failures.map(({ scenario, error }) => ({ scenario, error: error.slice(0, 700) })), pageErrors: result.pageErrors, browserClosed: result.browserClosed, listenerClosed: result.listenerClosed }) + "\n");
if (result.failures.length || result.pageErrors.length) process.exitCode = 1;
