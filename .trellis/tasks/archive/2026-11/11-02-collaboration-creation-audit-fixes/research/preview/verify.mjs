import { chromium, expect } from "@playwright/test";
import { createServer } from "node:http";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { extname, dirname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const preview = dirname(fileURLToPath(import.meta.url));
const mode = process.argv[2] ?? "current";
const onlyScenario = process.argv[3];
const scenarioFilter = onlyScenario ? new Set(onlyScenario.split(",")) : null;
const build = JSON.parse(await readFile(resolve(preview, `${mode}-build.json`), "utf8"));
const artifact = resolve(preview, `../browser-${mode}`, onlyScenario ? onlyScenario.toLowerCase().replace(/[^a-z0-9-]/g, "") : ".");
await mkdir(artifact, { recursive: true });
const result = { mode, fixture: "Production build of original shared components/CSS/locales, synthetic API and memory StorageAdapter", provenance: build,
  driverPid: process.pid, startedAt: new Date().toISOString(), scenarios: {}, diagnostics: {}, failures: [], pageErrors: [], blockedExternalRequests: [] };
const types = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".svg": "image/svg+xml" };
const server = createServer(async (request, response) => {
  try {
    const path = new URL(request.url, "http://127.0.0.1").pathname;
    const file = resolve(build.output, `.${path === "/" ? "/index.html" : path}`);
    if (!file.startsWith(`${resolve(build.output)}${sep}`)) { response.writeHead(403); response.end(); return; }
    const bytes = await readFile(file);
    response.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream", "Cache-Control": "no-store" });
    response.end(bytes);
  } catch { response.writeHead(404); response.end(); }
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const base = `http://127.0.0.1:${server.address().port}`;
result.listener = { origin: base, pid: process.pid, owned: true };
let browser;
const contexts = [];
const settled = (page) => page.waitForTimeout(180);

async function geometry(locator) {
  return locator.evaluate((element) => {
    const box = element.getBoundingClientRect();
    let visible = box.width > 0 && box.height > 0 && box.left >= -1 && box.right <= innerWidth + 1 && box.top >= -1 && box.bottom <= innerHeight + 1;
    for (let ancestor = element.parentElement; ancestor; ancestor = ancestor.parentElement) {
      // Body/HTML overflow is propagated to the viewport; fixed/absolute
      // portals are not clipped to the landing page's short content box.
      if (ancestor.tagName === "BODY" || ancestor.tagName === "HTML") break;
      const style = getComputedStyle(ancestor);
      const bounds = ancestor.getBoundingClientRect();
      if (/(auto|scroll|hidden|clip)/.test(style.overflowY)) visible &&= box.top >= bounds.top - 1 && box.bottom <= bounds.bottom + 1;
      // Fixed portals escape the scroll-locked document body's content box.
      if (style.position === "fixed") break;
    }
    return { width: box.width, height: box.height, top: box.top, bottom: box.bottom, left: box.left, right: box.right, visible };
  });
}

async function focusMeasurement(page, locator) {
  // Establish keyboard modality without tabbing out of a portaled picker.
  await page.keyboard.press("ArrowLeft");
  await locator.focus();
  await settled(page);
  return locator.evaluate((element) => {
    const css = getComputedStyle(element);
    const canvas = document.createElement("canvas"); canvas.width = canvas.height = 1;
    const context = canvas.getContext("2d", { willReadFrequently: true });
    const color = (value) => {
      const probe = document.createElement("span"); probe.style.color = value; element.parentElement.append(probe);
      const resolved = getComputedStyle(probe).color; probe.remove();
      context.clearRect(0, 0, 1, 1); context.fillStyle = resolved; context.fillRect(0, 0, 1, 1);
      return [...context.getImageData(0, 0, 1, 1).data];
    };
    const composite = (top, bottom) => { const alpha = top[3] / 255; return [0, 1, 2].map((index) => top[index] * alpha + bottom[index] * (1 - alpha)).concat(255); };
    const ancestors = []; for (let node = element.parentElement; node; node = node.parentElement) ancestors.push(node);
    let background = [255, 255, 255, 255];
    for (const ancestor of ancestors.reverse()) background = composite(color(getComputedStyle(ancestor).backgroundColor), background);
    const ringCSS = css.getPropertyValue("--tw-ring-color").trim();
    const outlinePresent = css.outlineStyle !== "none" && parseFloat(css.outlineWidth) > 0;
    const ringPresent = /0px 0px 0px [1-9][0-9]*(?:\.[0-9]+)?px/.test(css.boxShadow);
    const ring = color(ringPresent && ringCSS ? ringCSS : outlinePresent ? css.outlineColor : css.borderColor);
    const painted = composite(ring, background);
    const luminance = (rgb) => rgb.slice(0, 3).map((byte) => { const value = byte / 255; return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4; }).reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0);
    const first = luminance(painted), second = luminance(background);
    const ratio = (Math.max(first, second) + .05) / (Math.min(first, second) + .05);
    return { focusVisible: element.matches(":focus-visible"), ringCSS, ring, background, compositedRing: painted, ratio,
      boxShadow: css.boxShadow, outline: css.outline, borderColor: css.borderColor,
      ringPresent, outlinePresent, indicatorPainted: ringPresent || outlinePresent };
  });
}

async function identityMeasurement(locator, accountId) {
  return locator.evaluate((root, accountId) => {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    let node;
    while ((node = walker.nextNode())) {
      const start = node.textContent.indexOf(accountId);
      if (start < 0) continue;
      const range = document.createRange(); range.setStart(node, start); range.setEnd(node, start + accountId.length);
      const boxes = [...range.getClientRects()];
      let visible = boxes.length > 0 && boxes.every((box) => box.width > 0 && box.left >= -1 && box.right <= innerWidth + 1 && box.top >= -1 && box.bottom <= innerHeight + 1);
      for (let ancestor = node.parentElement; ancestor; ancestor = ancestor.parentElement) {
        if (ancestor.tagName === "BODY" || ancestor.tagName === "HTML") break;
        const style = getComputedStyle(ancestor), bounds = ancestor.getBoundingClientRect();
        if (/(hidden|clip|auto|scroll)/.test(style.overflowX)) visible &&= boxes.every((box) => box.left >= bounds.left - 1 && box.right <= bounds.right + 1);
        if (/(hidden|clip|auto|scroll)/.test(style.overflowY)) visible &&= boxes.every((box) => box.top >= bounds.top - 1 && box.bottom <= bounds.bottom + 1);
        if (style.position === "fixed") break;
      }
      return { fullText: true, visible, lines: boxes.length };
    }
    return { fullText: false, visible: false, lines: 0 };
  }, accountId);
}

async function pageFor({ width = 1280, height = 900, coarse = false, locale = "zh-Hans", theme = "light" } = {}) {
  const context = await browser.newContext({ viewport: { width, height }, hasTouch: coarse, isMobile: coarse, deviceScaleFactor: 1, colorScheme: theme });
  contexts.push(context);
  const page = await context.newPage();
  page.setDefaultTimeout(5000);
  page.setDefaultNavigationTimeout(7000);
  page.on("pageerror", (error) => result.pageErrors.push(error.message));
  await page.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (url.origin === base) await route.continue();
    else { result.blockedExternalRequests.push({ origin: url.origin, pathname: url.pathname }); await route.abort(); }
  });
  await page.goto(`${base}/?locale=${locale}`);
  await page.waitForFunction(() => Boolean(window.fixture));
  await page.evaluate((theme) => document.documentElement.classList.toggle("dark", theme === "dark"), theme);
  const translations = await page.evaluate(() => window.fixture.translations);
  await page.getByRole("button", { name: translations.creation.entry, exact: true }).click();
  await expect(page.getByRole("dialog", { name: translations.creation.title, exact: true })).toBeVisible();
  await expect(page.getByLabel(translations.creation.titleLabel, { exact: true })).toBeVisible();
  await settled(page);
  return { page, context, translations };
}

async function scenario(name, run) {
  if (scenarioFilter && !scenarioFilter.has(name)) return;
  process.stdout.write(`Starting ${name}\n`);
  try { result.scenarios[name] = await run(); }
  catch (error) { result.failures.push({ scenario: name, error: String(error.stack ?? error) }); }
  process.stdout.write(`Finished ${name}\n`);
}

try {
  browser = await chromium.launch({ headless: true });
  result.engine = `Playwright Chromium ${browser.version()}`;
  const cdp = await browser.newBrowserCDPSession();
  result.ownedBrowserProcesses = (await cdp.send("SystemInfo.getProcessInfo")).processInfo.map(({ id, type }) => ({ id, type }));
  await cdp.detach();

  for (const theme of ["light", "dark"]) await scenario(`desktop-${theme}`, async () => {
    const { page, translations: t } = await pageFor({ theme });
    const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
    const source = page.getByLabel(t.creation.source, { exact: true });
    const title = page.getByLabel(t.creation.titleLabel, { exact: true });
    await title.fill("协作弹窗修复验证");
    await page.screenshot({ path: resolve(artifact, `desktop-${theme}.png`) });
    const focus = { source: await focusMeasurement(page, source), title: await focusMeasurement(page, title), description: await focusMeasurement(page, page.getByLabel(t.creation.description, { exact: true })),
      submit: await focusMeasurement(page, page.getByRole("button", { name: t.creation.submit, exact: true })), close: await focusMeasurement(page, dialog.getByRole("button", { name: t.creation.close, exact: true })) };
    const trigger = page.getByRole("button", { name: t.creation.recipient, exact: true });
    focus.recipientTrigger = await focusMeasurement(page, trigger);
    await trigger.click();
    const list = page.getByRole("listbox", { name: t.creation.recipient, exact: true });
    await expect(list).toBeVisible();
    await settled(page);
    const popup = page.locator('[data-slot="popover-content"]').filter({ has: list });
    const search = popup.getByRole("textbox");
    focus.recipientSearch = await focusMeasurement(page, search);
    await search.press("ArrowDown");
    const semantics = await search.evaluate((input) => ({ role: input.getAttribute("role"), name: input.getAttribute("aria-label"), controls: input.getAttribute("aria-controls"), activeDescendant: input.getAttribute("aria-activedescendant"), activeExists: Boolean(document.getElementById(input.getAttribute("aria-activedescendant") ?? "")) }));
    const options = await list.locator('[role="option"]').evaluateAll((rows) => rows.map((row) => ({ id: row.id, selected: row.getAttribute("aria-selected"), disabled: row.getAttribute("aria-disabled"), text: row.textContent })));
    const ax = await page.context().newCDPSession(page);
    const tree = await ax.send("Accessibility.getFullAXTree");
    await writeFile(resolve(artifact, `recipient-ax-${theme}.json`), JSON.stringify(tree.nodes.filter((node) => ["textbox", "listbox", "option"].includes(node.role?.value)), null, 2) + "\n");
    await ax.detach();
    await page.screenshot({ path: resolve(artifact, `recipient-${theme}.png`) });
    await search.press("Escape");
    await expect(list).toHaveCount(0);
    const pickerEscape = await dialog.isVisible() && await list.count() === 0;
    const initialToggle = page.locator("summary").filter({ hasText: t.fields.initial });
    focus.initialFieldsToggle = await focusMeasurement(page, initialToggle);
    await initialToggle.click();
    const labelsTrigger = page.getByRole("button", { name: t.fields.labels, exact: true });
    focus.labelsTrigger = await focusMeasurement(page, labelsTrigger);
    await labelsTrigger.click();
    const labelSearch = page.getByPlaceholder(t.fields.findLabel, { exact: true });
    await expect(labelSearch).toBeVisible(); await settled(page);
    focus.labelsSearch = await focusMeasurement(page, labelSearch);
    await labelSearch.press("Escape");
    result.diagnostics[`desktop-${theme}`] = { focus, semantics, options, pickerEscape };
    if (mode !== "baseline") {
      expect(options.length).toBe(4); expect(semantics.controls).toBeTruthy(); expect(semantics.activeExists).toBe(true); expect(pickerEscape).toBe(true);
      for (const [name, measurement] of Object.entries(focus)) { expect(measurement.focusVisible, name).toBe(true); expect(measurement.indicatorPainted, name).toBe(true); expect(measurement.ratio, name).toBeGreaterThanOrEqual(3); }
    }
    return { focus, semantics, options, pickerEscape, horizontalOverflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth) };
  });

  for (const size of [{ width: 390, height: 740, theme: "light" }, { width: 320, height: 568, theme: "light" }, { width: 390, height: 740, theme: "dark" }, { width: 320, height: 568, theme: "dark" }]) await scenario(`coarse-${size.width}${size.theme === "dark" ? "-dark" : ""}`, async () => {
    const { page, translations: t } = await pageFor({ ...size, coarse: true });
    const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
    const title = page.getByLabel(t.creation.titleLabel, { exact: true });
    await title.fill("触控与长表单验证");
    const submit = page.getByRole("button", { name: t.creation.submit, exact: true });
    const close = dialog.getByRole("button", { name: t.creation.close, exact: true });
    const cancel = dialog.getByRole("button", { name: t.creation.cancel ?? "Cancel", exact: true });
    const initial = { submit: await geometry(submit), close: await geometry(close), cancel: await cancel.count() ? await geometry(cancel) : null };
    await page.screenshot({ path: resolve(artifact, `coarse-${size.width}-${size.theme}-default.png`) });
    const trigger = page.getByRole("button", { name: t.creation.recipient, exact: true });
    await trigger.tap();
    const list = page.getByRole("listbox", { name: t.creation.recipient, exact: true });
    await expect(list).toBeVisible();
    await settled(page);
    const rows = list.locator("button");
    const rowHeights = await rows.evaluateAll((rows) => rows.map((row) => row.getBoundingClientRect().height));
    const fifth = list.locator('[data-recipient-row]').filter({ hasText: "李明" }).nth(1).locator("button");
    const accountId = await page.evaluate(() => window.fixture.accounts[1].user_id);
    const idsVisible = await list.getByText(accountId, { exact: false }).count() > 0;
    const candidateIdentity = await identityMeasurement(fifth, accountId);
    const row = await fifth.boundingBox();
    await page.touchscreen.tap(row.x + 6, row.y + row.height - 5);
    await expect(list).toHaveCount(0);
    const selectedIdentityVisible = (await trigger.textContent()).includes(accountId);
    const selectedIdentity = await identityMeasurement(trigger, accountId);
    await page.locator("summary").filter({ hasText: t.fields.initial }).click();
    const parentTrigger = page.getByRole("button", { name: t.fields.parent, exact: true });
    await parentTrigger.tap();
    await settled(page);
    const parentSearch = page.getByPlaceholder(t.fields.findParent, { exact: true });
    const parentSearchGeometry = await geometry(parentSearch);
    const parentFocus = await focusMeasurement(page, parentSearch);
    await parentSearch.press("Escape");
    const stage = page.getByLabel(t.fields.stage, { exact: true });
    const stageGeometry = await geometry(stage);
    const stageFocus = await focusMeasurement(page, stage);
    await dialog.evaluate((dialog) => {
      const candidates = [dialog, ...dialog.querySelectorAll("*")].filter((node) => node.scrollHeight > node.clientHeight + 10 && /(auto|scroll)/.test(getComputedStyle(node).overflowY));
      candidates.forEach((node) => { node.scrollTop = node.scrollHeight; });
    });
    await settled(page);
    const bottom = { submit: await geometry(submit), close: await geometry(close), cancel: await cancel.count() ? await geometry(cancel) : null };
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    await page.screenshot({ path: resolve(artifact, `coarse-${size.width}-${size.theme}-bottom.png`) });
    await title.focus();
    const tabFocus = [];
    for (let index = 0; index < 24; index++) {
      await page.keyboard.press("Tab");
      tabFocus.push(await page.evaluate(() => {
        const element = document.activeElement, box = element.getBoundingClientRect();
        const surface = element.closest('[data-slot="dialog-content"]');
        const ancestor = element.closest('[data-slot="dialog-footer"]');
        let unoccluded = box.top >= 0 && box.bottom <= innerHeight;
        for (let parent = element.parentElement; parent && parent !== surface; parent = parent.parentElement) {
          if (/(auto|scroll|hidden)/.test(getComputedStyle(parent).overflowY)) { const bounds = parent.getBoundingClientRect(); unoccluded &&= box.top >= bounds.top - 1 && box.bottom <= bounds.bottom + 1; }
        }
        return { tag: element.tagName, label: element.getAttribute("aria-label") ?? element.id ?? element.textContent?.slice(0, 30), top: box.top, bottom: box.bottom, inDialog: Boolean(surface), inFooter: Boolean(ancestor), unoccluded };
      }));
    }
    if (mode !== "baseline") {
      result.diagnostics[`coarse-${size.width}-${size.theme}`] = { initial, bottom, rowHeights, parentSearchGeometry, stageGeometry, parentFocus, stageFocus, candidateIdentity, selectedIdentity, tabFocus, overflow };
      expect(initial.submit.visible).toBe(true); expect(initial.close.visible).toBe(true); expect(bottom.submit.visible).toBe(true); expect(bottom.close.visible).toBe(true); expect(bottom.cancel?.visible).toBe(true);
      expect(rowHeights.every((height) => height >= 44)).toBe(true); expect(initial.close.width).toBeGreaterThanOrEqual(44); expect(initial.close.height).toBeGreaterThanOrEqual(44);
      expect(parentSearchGeometry.height).toBeGreaterThanOrEqual(44); expect(stageGeometry.height).toBeGreaterThanOrEqual(44); expect(parentFocus.ratio).toBeGreaterThanOrEqual(3); expect(stageFocus.ratio).toBeGreaterThanOrEqual(3); expect(parentFocus.indicatorPainted).toBe(true); expect(stageFocus.indicatorPainted).toBe(true);
      expect(idsVisible).toBe(true); expect(selectedIdentityVisible).toBe(true); expect(candidateIdentity.visible).toBe(true); expect(selectedIdentity.visible).toBe(true);
      expect(overflow).toBe(false); expect(tabFocus.every((entry) => !entry.inDialog || entry.unoccluded)).toBe(true);
    }
    await close.tap();
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("button", { name: t.creation.entry, exact: true })).toBeFocused();
    return { coarse: await page.evaluate(() => matchMedia("(pointer: coarse)").matches), initial, bottom, rowHeights, parentSearchGeometry, stageGeometry, parentFocus, stageFocus, idsVisible, selectedIdentityVisible, candidateIdentity, selectedIdentity, tabFocus, overflow, touch: "Chromium CDP touchscreen.tap at measured target edges/centers" };
  });

  await scenario("delayed-create-source-lock", async () => {
    const { page, translations: t } = await pageFor();
    await page.getByLabel(t.creation.titleLabel, { exact: true }).fill("延迟回执验证");
    await page.evaluate(() => window.fixture.setMode("delayed"));
    await page.getByRole("button", { name: t.creation.submit, exact: true }).click();
    await page.waitForFunction(() => window.fixture.hasPending());
    const source = page.getByLabel(t.creation.source, { exact: true });
    const sourceLocked = await source.isDisabled();
    if (sourceLocked) await source.evaluate((select) => { select.value = window.fixture.secondSource; select.dispatchEvent(new Event("change", { bubbles: true })); });
    else await source.selectOption(await page.evaluate(() => window.fixture.secondSource));
    const titleRetained = await page.getByLabel(t.creation.titleLabel, { exact: true }).inputValue() === "延迟回执验证";
    await page.evaluate(() => window.fixture.finish());
    await page.waitForTimeout(250);
    const state = await page.evaluate(() => ({ navigationCalls: window.fixture.navigationCalls, records: window.fixture.records().filter((record) => record.kind === "creation").length }));
    const dialogRemaining = await page.getByRole("dialog", { name: t.creation.title, exact: true }).count();
    await page.screenshot({ path: resolve(artifact, "delayed-create-after-response.png") });
    if (mode !== "baseline") { expect(sourceLocked).toBe(true); expect(titleRetained).toBe(true); expect(state.navigationCalls).toHaveLength(1); expect(state.records).toBe(0); expect(dialogRemaining).toBe(0); }
    return { sourceLocked, titleRetained, ...state, dialogRemaining };
  });

  for (const config of [{ locale: "en" }, { locale: "zh-Hans" }, { locale: "en", width: 320, height: 568, coarse: true }, { locale: "zh-Hans", width: 320, height: 568, coarse: true }]) await scenario(`quota-${config.locale}${config.coarse ? "-narrow" : ""}`, async () => {
    const { page, translations: t } = await pageFor(config);
    const locale = config.locale;
    await page.getByLabel(t.creation.titleLabel, { exact: true }).fill("额度拒绝验证");
    await page.evaluate(() => window.fixture.setMode("reject"));
    await page.getByRole("button", { name: t.creation.submit, exact: true }).click();
    await expect(page.getByRole("button", { name: t.creation.submit, exact: true })).toBeEnabled();
    await page.waitForTimeout(150);
    const messages = await page.locator('[role="status"], [role="alert"]').allTextContents();
    const preserved = await page.getByLabel(t.creation.titleLabel, { exact: true }).inputValue();
    const posts = await page.evaluate(() => window.fixture.requests.filter((request) => request.method === "POST"));
    const sourceUnlocked = await page.getByLabel(t.creation.source, { exact: true }).isEnabled();
    const rejection = page.locator('[role="status"], [role="alert"]').filter({ hasText: t.creation.rejected }).first();
    const rejectionGeometry = await geometry(rejection);
    await page.screenshot({ path: resolve(artifact, `quota-${locale}${config.coarse ? "-narrow" : ""}.png`) });
    result.diagnostics[`quota-${locale}${config.coarse ? "-narrow" : ""}`] = { messages, rejectionGeometry, preserved, posts: posts.length, sourceUnlocked };
    if (mode !== "baseline") { expect(messages.join(" ")).toMatch(locale === "en" ? /limit|quota/i : /上限|数量|额度/); expect(preserved).toBe("额度拒绝验证"); expect(posts).toHaveLength(1); expect(sourceUnlocked).toBe(true); expect(rejectionGeometry.visible).toBe(true); }
    return { messages, rejectionGeometry, preserved, posts: posts.length, sourceUnlocked };
  });

  await scenario("parent-search-debounce", async () => {
    const { page, translations: t } = await pageFor();
    await page.locator("summary").filter({ hasText: t.fields.initial }).click();
    await page.getByRole("button", { name: t.fields.parent, exact: true }).click();
    const input = page.getByPlaceholder(t.fields.findParent, { exact: true });
    await input.pressSequentially("协作任务改进", { delay: 25 });
    const rawImmediate = await input.inputValue();
    const during = await page.evaluate(() => window.fixture.requests.filter((request) => new URLSearchParams(request.search).has("parent_search")).map((request) => new URLSearchParams(request.search).get("parent_search")));
    await page.waitForTimeout(380);
    const after = await page.evaluate(() => window.fixture.requests.filter((request) => new URLSearchParams(request.search).has("parent_search")).map((request) => new URLSearchParams(request.search).get("parent_search")));
    await input.fill("设计");
    const priorCandidateVisible = await page.getByRole("button", { name: "SRC-11 协作任务改进", exact: true }).count() > 0;
    await input.press("Enter");
    const selectedBeforeDelay = await page.getByRole("button", { name: t.fields.parent, exact: true }).textContent();
    if (mode !== "baseline") { expect(rawImmediate).toBe("协作任务改进"); expect(during).toHaveLength(0); expect(after).toEqual(["协作任务改进"]); expect(priorCandidateVisible).toBe(false); expect(selectedBeforeDelay).not.toContain("SRC-11"); }
    return { rawImmediate, during, after, priorCandidateVisible, selectedBeforeDelay };
  });

  for (const config of [{}, { locale: "en", width: 320, height: 568, coarse: true }]) await scenario(`unknown-result-original-recovery${config.coarse ? "-narrow" : ""}`, async () => {
    const { page, translations: t } = await pageFor(config);
    const title = page.getByLabel(t.creation.titleLabel, { exact: true });
    await title.fill("未知结果保留原请求");
    await page.evaluate(() => window.fixture.setMode("unknown"));
    await page.getByRole("button", { name: t.creation.submit, exact: true }).click();
    await expect(page.getByRole("button", { name: t.creation.recover, exact: true })).toBeEnabled();
    const recoveryGeometry = await geometry(page.getByRole("button", { name: t.creation.recover, exact: true }));
    if (mode !== "baseline") expect(recoveryGeometry.visible).toBe(true);
    const pendingRecord = await page.evaluate(() => window.fixture.records().find((record) => record.kind === "creation"));
    const unknownLocked = await page.getByLabel(t.creation.source, { exact: true }).isDisabled();
    const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
    await dialog.getByRole("button", { name: t.creation.close, exact: true }).click();
    await expect(dialog).toHaveCount(0); await settled(page);
    await page.getByRole("button", { name: t.creation.entry, exact: true }).click();
    await expect(title).toHaveValue("未知结果保留原请求");
    await page.evaluate(() => window.fixture.setMode("delayed-recovery"));
    await page.getByRole("button", { name: t.creation.recover, exact: true }).click();
    await page.waitForFunction(() => window.fixture.hasPending());
    const recoveryLocked = await page.getByLabel(t.creation.source, { exact: true }).isDisabled();
    await page.evaluate(() => window.fixture.finish());
    await expect(dialog).toHaveCount(0);
    const after = await page.evaluate(() => ({ navigation: window.fixture.navigationCalls, requests: window.fixture.requests, records: window.fixture.records().filter((record) => record.kind === "creation") }));
    const posts = after.requests.filter((request) => request.method === "POST" && request.path === "/api/collaboration/issues");
    const recovery = after.requests.filter((request) => request.path === `/api/collaboration/creations/${pendingRecord.requestId}`);
    expect(posts).toHaveLength(1); expect(recovery).toHaveLength(1); expect(after.navigation).toHaveLength(1); expect(after.records).toHaveLength(0);
    if (mode !== "baseline") { expect(unknownLocked).toBe(true); expect(recoveryLocked).toBe(true); }
    return { unknownLocked, recoveryLocked, recoveryGeometry, posts: posts.length, originalRequestId: pendingRecord.requestId, recoveryPath: recovery[0].path, navigation: after.navigation, records: after.records.length, draftRetainedOnReopen: true };
  });

  await scenario("exact-recipient-identity", async () => {
    const { page, translations: t } = await pageFor();
    await page.getByLabel(t.creation.titleLabel, { exact: true }).fill("精确接收人身份验证");
    const trigger = page.getByRole("button", { name: t.creation.recipient, exact: true });
    await trigger.click();
    const list = page.getByRole("listbox", { name: t.creation.recipient, exact: true });
    const popup = page.locator('[data-slot="popover-content"]').filter({ has: list });
    await popup.getByPlaceholder(t.recipientPicker.search, { exact: true }).fill("客户");
    await page.getByRole("button", { name: t.recipientPicker.exactLookup, exact: true }).click();
    const accountId = await page.evaluate(() => window.fixture.accounts[2].user_id);
    await page.getByLabel(t.recipientPicker.exactLookup, { exact: true }).fill(accountId);
    await page.getByRole("button", { name: t.recipientPicker.findExact, exact: true }).click();
    const useAccount = page.getByRole("button", { name: t.creation.useRecipient, exact: true });
    await expect(useAccount).toBeVisible();
    const exactIdentityVisible = await useAccount.evaluate((button) => button.parentElement.textContent.includes(window.fixture.accounts[2].user_id));
    await page.screenshot({ path: resolve(artifact, "recipient-exact-identity.png") });
    await useAccount.click();
    const selectedIdentityVisible = (await trigger.textContent()).includes(accountId);
    await page.getByRole("button", { name: t.creation.submit, exact: true }).click();
    await expect(page.getByRole("dialog", { name: t.creation.title, exact: true })).toHaveCount(0);
    const posts = await page.evaluate(() => window.fixture.requests.filter((request) => request.method === "POST" && request.path === "/api/collaboration/issues"));
    expect(posts).toHaveLength(1); expect(posts[0].body.handler_user_id).toBe(accountId);
    if (mode !== "baseline") { expect(exactIdentityVisible).toBe(true); expect(selectedIdentityVisible).toBe(true); }
    return { exactIdentityVisible, selectedIdentityVisible, accountId, submittedAccountId: posts[0].body.handler_user_id };
  });

  await scenario("generic-picker-dialog-compatibility", async () => {
    const { page, translations: t } = await pageFor({ width: 390, height: 740, coarse: true, locale: "en" });
    await page.getByRole("dialog", { name: t.creation.title, exact: true }).getByRole("button", { name: t.creation.close, exact: true }).tap();
    await expect(page.getByRole("dialog", { name: t.creation.title, exact: true })).toHaveCount(0);
    await settled(page);
    const entryFocus = await focusMeasurement(page, page.getByRole("button", { name: t.creation.entry, exact: true }));
    if (mode !== "baseline") { expect(entryFocus.indicatorPainted).toBe(true); expect(entryFocus.ratio).toBeGreaterThanOrEqual(3); }
    await page.getByText("Shared picker compatibility", { exact: true }).click();
    await settled(page);
    const search = page.getByRole("textbox", { name: "Filter options", exact: true });
    await search.press("ArrowDown"); await search.press("Enter");
    const genericOptionCount = await page.getByRole("option").count();
    await page.getByRole("button", { name: "Shared dialog compatibility", exact: true }).click();
    const generic = page.getByRole("dialog", { name: "Shared dialog", exact: true });
    await expect(generic).toBeVisible();
    await settled(page);
    const genericClose = await geometry(generic.getByRole("button", { name: "Close", exact: true }));
    expect(genericOptionCount).toBe(0); expect(genericClose.width).toBe(28); expect(genericClose.height).toBe(28);
    return { genericOptionCount, genericClose, retainedNativeTextbox: true, entryFocus };
  });
} finally {
  await Promise.allSettled(contexts.map((context) => context.close()));
  if (browser) await browser.close();
  await new Promise((resolve) => server.close(resolve));
  result.listenerClosed = true; result.browserClosed = true; result.finishedAt = new Date().toISOString();
  await writeFile(resolve(artifact, "results.json"), JSON.stringify(result, null, 2) + "\n");
}
process.stdout.write(JSON.stringify({ mode, scenarios: Object.keys(result.scenarios), failures: result.failures.map(({ scenario, error }) => ({ scenario, error: error.slice(0, 500) })), pageErrors: result.pageErrors, artifact }) + "\n");
if (result.failures.length || result.pageErrors.length) process.exitCode = 1;
