import { chromium, expect } from "@playwright/test";
import { createServer } from "node:http";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { extname, dirname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const preview = dirname(fileURLToPath(import.meta.url));
const mode = process.argv[2] ?? "round-1";
const onlyScenario = process.argv[3];
const scenarioFilter = onlyScenario ? new Set(onlyScenario.split(",")) : null;
const build = JSON.parse(await readFile(resolve(preview, `${mode}-build.json`), "utf8"));
const artifact = resolve(preview, `../browser-${mode}`, process.env.ASSIST_EVIDENCE_SUFFIX ?? (onlyScenario ? "targeted" : "."));
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
const settled = (page) => page.waitForTimeout(350);

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
    const bottomColor = color(css.borderBottomColor);
    const bottomBorderPresent = css.borderBottomStyle !== "none" && parseFloat(css.borderBottomWidth) >= 2 && bottomColor[3] > 0;
    const ring = color(ringPresent && ringCSS ? ringCSS : outlinePresent ? css.outlineColor : bottomBorderPresent ? css.borderBottomColor : css.borderColor);
    const effectiveOpacity = [element, ...ancestors].reduce((opacity, item) => opacity * Number(getComputedStyle(item).opacity), 1);
    const indicator = [...ring]; indicator[3] *= effectiveOpacity;
    const painted = composite(indicator, background);
    const luminance = (rgb) => rgb.slice(0, 3).map((byte) => { const value = byte / 255; return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4; }).reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0);
    const first = luminance(painted), second = luminance(background);
    const ratio = (Math.max(first, second) + .05) / (Math.min(first, second) + .05);
    return { focusVisible: element.matches(":focus-visible"), ringCSS, ring, background, compositedRing: painted, ratio,
      boxShadow: css.boxShadow, outline: css.outline, borderColor: css.borderColor,
      ringPresent, outlinePresent, bottomBorderPresent, bottomBorderWidth: css.borderBottomWidth, bottomBorderColor: css.borderBottomColor, effectiveOpacity, indicatorPainted: ringPresent || outlinePresent || bottomBorderPresent };
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

async function pageFor({ width = 1280, height = 900, coarse = false, locale = "zh-Hans", theme = "light", longSource = false, pixelScale = 1, manualOpen = false, delaySource = false, noSource = false } = {}) {
  const context = await browser.newContext({ viewport: { width, height }, hasTouch: coarse, isMobile: coarse, deviceScaleFactor: pixelScale, colorScheme: theme });
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
  await page.goto(`${base}/?locale=${locale}${longSource ? "&longSource=1" : ""}${delaySource ? "&delaySource=1" : ""}${noSource ? "&noSource=1" : ""}`);
  await page.waitForFunction(() => Boolean(window.fixture));
  await page.evaluate((theme) => document.documentElement.classList.toggle("dark", theme === "dark"), theme);
  const translations = await page.evaluate(() => window.fixture.translations);
  if (manualOpen) return { page, context, translations };
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
    await fifth.scrollIntoViewIfNeeded(); await settled(page);
    const candidateIdentity = await identityMeasurement(fifth, accountId);
    const row = await fifth.boundingBox();
    result.diagnostics[`coarse-${size.width}-${size.theme}-candidate`] = { candidateIdentity, row };
    await page.screenshot({ path: resolve(artifact, `coarse-${size.width}-${size.theme}-picker.png`) });
    await fifth.tap();
    await expect(list).toHaveCount(0);
    const selectedIdentityVisible = (await trigger.textContent()).includes(accountId);
    await trigger.focus(); await settled(page);
    const selectedIdentity = await identityMeasurement(trigger, accountId);
    await page.locator("summary").filter({ hasText: t.fields.initial }).click();
    const parentTrigger = page.getByRole("button", { name: t.fields.parent, exact: true });
    await parentTrigger.tap();
    await settled(page);
    const parentSearch = page.getByPlaceholder(t.fields.findParent, { exact: true });
    const parentSearchGeometry = await geometry(parentSearch);
    const parentFocus = await focusMeasurement(page, parentSearch);
    await parentSearch.fill("协作任务改进");
    await page.getByRole("button").filter({ hasText: "SRC-11" }).last().click(); await settled(page);
    const stage = page.getByLabel(t.fields.stageInputLabel ?? t.fields.stage, { exact: true });
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

  for (const viewport of [{ width: 1280, height: 900 }, { width: 390, height: 740 }, { width: 320, height: 568 }]) {
    for (const theme of ["light", "dark"]) await scenario(`attachments-32-${viewport.width}-${theme}`, async () => {
      const { page, translations: t } = await pageFor({ ...viewport, theme, coarse: viewport.width < 600 });
      const title = page.getByLabel(t.creation.titleLabel, { exact: true });
      await title.fill("32 attachments must scroll without moving composer actions");
      await page.evaluate(() => window.fixture.seedAttachments(32));
      await expect(page.getByRole("checkbox")).toHaveCount(32);
      const body = page.locator("[data-collaboration-creation-body]");
      const actions = page.locator("[data-collaboration-creation-actions]");
      const attachmentTrigger = page.getByRole("button", { name: t.attachments.add, exact: true });
      const submit = page.getByRole("button", { name: t.creation.submit, exact: true });
      const before = { body: await geometry(body), actions: await geometry(actions), trigger: await geometry(attachmentTrigger), submit: await geometry(submit) };
      await page.screenshot({ path: resolve(artifact, `attachments-32-${viewport.width}-${theme}-top.png`) });
      await body.evaluate((element) => { element.scrollTop = element.scrollHeight; });
      await settled(page);
      const after = { body: await geometry(body), actions: await geometry(actions), trigger: await geometry(attachmentTrigger), submit: await geometry(submit) };
      const detailPlacement = await page.getByRole("checkbox").evaluateAll((items) => items.every((item) => Boolean(item.closest("[data-collaboration-creation-body]"))));
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth || document.body.scrollWidth > innerWidth);
      const last = page.getByRole("checkbox").last();
      await last.focus(); await settled(page);
      const lastFocus = await geometry(last);
      await page.screenshot({ path: resolve(artifact, `attachments-32-${viewport.width}-${theme}-bottom.png`) });
      const evidence = { before, after, detailPlacement, overflow, lastFocus, count: await page.getByRole("checkbox").count() };
      result.diagnostics[`attachments-32-${viewport.width}-${theme}`] = evidence;
      expect(before.body.height).toBeGreaterThan(100); expect(after.submit.visible).toBe(true); expect(after.trigger.visible).toBe(true);
      expect(Math.abs(before.actions.top - after.actions.top)).toBeLessThan(2); expect(detailPlacement).toBe(true); expect(overflow).toBe(false); expect(lastFocus.visible).toBe(true);
      if (viewport.width < 600) { expect(after.trigger.height).toBeGreaterThanOrEqual(44); expect(after.submit.height).toBeGreaterThanOrEqual(44); }
      return evidence;
    });
  }

  for (const theme of ["light", "dark"]) await scenario(`attachment-focus-${theme}`, async () => {
    const { page, translations: t } = await pageFor({ width: 320, height: 568, coarse: true, locale: "en", theme });
    await page.getByLabel(t.creation.titleLabel, { exact: true }).fill("Upload recovery focus");
    const trigger = page.getByRole("button", { name: t.attachments.add, exact: true });
    const input = page.locator('input[type="file"][multiple]');
    await trigger.focus();
    const chooserEvent = page.waitForEvent("filechooser");
    await page.keyboard.press("Enter");
    const chooser = await chooserEvent;
    await chooser.setFiles([]);
    await settled(page);
    const chooserCancelFocus = await trigger.evaluate((button) => document.activeElement === button);
    const requestCountAfterCancel = await page.evaluate(() => window.fixture.requests.filter((request) => request.path.endsWith("attachments/drafts") && request.method === "POST").length);
    await page.evaluate(() => window.fixture.setUploadMode("delayed"));
    await input.setInputFiles({ name: "pending-long-filename-for-cancel-and-recovery.txt", mimeType: "text/plain", buffer: Buffer.from("fixture") });
    const cancel = page.getByRole("button", { name: t.attachments.cancelUpload, exact: true });
    await expect(cancel).toBeVisible();
    await page.evaluate(({ addName, cancelName }) => {
      const trace = window.fixture.cancelTrace = [];
      const named = (name) => [...document.querySelectorAll("button")].find((button) => button.textContent.trim() === name);
      const compact = named(addName);
      const describe = (element) => element ? { tag: element.tagName, role: element.getAttribute("role"), text: element.textContent?.trim().slice(0, 50) } : null;
      const record = (type) => {
        trace.push({ type, at: performance.now(), active: describe(document.activeElement), cancelPresent: Boolean(named(cancelName)),
          trigger: { connected: compact?.isConnected, disabled: compact?.disabled, ariaDisabled: compact?.getAttribute("aria-disabled"), tabIndex: compact?.tabIndex } });
      };
      document.addEventListener("focusin", () => record("focusin"), true);
      document.addEventListener("click", (event) => { if (event.target.closest("button")?.textContent.trim() === cancelName) record("cancel-click-capture"); }, true);
      let prior = "";
      const observer = new MutationObserver(() => {
        const signature = JSON.stringify({ cancel: Boolean(named(cancelName)), connected: compact?.isConnected, disabled: compact?.disabled, aria: compact?.getAttribute("aria-disabled") });
        if (signature !== prior) { prior = signature; record("mutation"); }
      });
      observer.observe(document.body, { subtree: true, childList: true, attributes: true, attributeFilter: ["disabled", "aria-disabled", "tabindex"] });
      window.fixture.stopCancelTrace = () => observer.disconnect();
      record("before-cancel-focus");
    }, { addName: t.attachments.add, cancelName: t.attachments.cancelUpload });
    await cancel.focus(); await page.keyboard.press("Enter");
    await expect(cancel).toHaveCount(0); await settled(page);
    const cancelTriggerFocused = await trigger.evaluate((button) => document.activeElement === button);
    const cancelFocus = await page.evaluate(() => ({ tag: document.activeElement.tagName, text: document.activeElement.textContent?.slice(0,80), role: document.activeElement.getAttribute("role"), disabled: document.activeElement.hasAttribute("disabled"), body: document.activeElement === document.body }));
    const cancelTrace = await page.evaluate(() => { window.fixture.stopCancelTrace(); return window.fixture.cancelTrace; });
    const recovery = page.getByRole("button", { name: t.attachments.recover, exact: true });
    await expect(recovery).toBeEnabled();
    await page.screenshot({ path: resolve(artifact, `attachment-cancel-${theme}.png`) });
    await page.evaluate(() => window.fixture.setUploadMode("success"));
    await recovery.focus(); await page.keyboard.press("Enter");
    await expect(recovery).toHaveCount(0); await expect(trigger).toBeEnabled(); await settled(page);
    const recoveryFocus = await trigger.evaluate((button) => document.activeElement === button);
    const recoveryGeometry = await geometry(trigger);
    const recoveryPaint = await focusMeasurement(page, trigger);
    await page.screenshot({ path: resolve(artifact, `attachment-recovery-${theme}.png`) });
    await page.evaluate(() => window.fixture.setUploadMode("lost"));
    await input.setInputFiles({ name: "retry-focus.txt", mimeType: "text/plain", buffer: Buffer.from("retry") });
    const retry = page.getByRole("button", { name: t.attachments.retry, exact: true });
    await expect(retry).toBeEnabled();
    await page.evaluate(() => window.fixture.setUploadMode("success"));
    await retry.focus(); await page.keyboard.press("Enter");
    await expect(retry).toHaveCount(0); await settled(page);
    const retryFocus = await trigger.evaluate((button) => document.activeElement === button);
    const evidence = { chooserCancelFocus, requestCountAfterCancel, cancelFocus, cancelTriggerFocused, cancelTrace, recoveryFocus, recoveryGeometry, recoveryPaint, retryFocus };
    result.diagnostics[`attachment-focus-${theme}`] = evidence;
    expect(requestCountAfterCancel).toBe(0); expect(chooserCancelFocus).toBe(true); expect(cancelFocus.body).toBe(false); expect(cancelFocus.tag).toBe("BUTTON"); expect(cancelTriggerFocused).toBe(true);
    expect(recoveryFocus).toBe(true); expect(recoveryGeometry.visible).toBe(true); expect(recoveryPaint.indicatorPainted).toBe(true); expect(recoveryPaint.ratio).toBeGreaterThanOrEqual(3); expect(retryFocus).toBe(true);
    return evidence;
  });

  await scenario("retained-advanced-summary", async () => {
    const { page, translations: t } = await pageFor({ width: 320, height: 568, coarse: true });
    await page.getByLabel(t.creation.titleLabel, { exact: true }).fill("Retained initial fields");
    await page.evaluate(() => window.fixture.setInitialFields({ status: "scheduled", priority: "high" }));
    const details = page.locator("details"); const summary = details.locator("summary");
    await summary.scrollIntoViewIfNeeded();
    const retainedSummary = await summary.textContent();
    await page.screenshot({ path: resolve(artifact, "advanced-retained-summary.png") });
    await page.evaluate(() => window.fixture.setInitialFields({ status: "removed-status" }));
    await expect(details).toHaveAttribute("open", "");
    await expect(page.getByRole("alert").filter({ hasText: t.fields.invalidSelection })).toBeVisible();
    const invalidExpanded = await details.evaluate((element) => element.open);
    await page.screenshot({ path: resolve(artifact, "advanced-invalid-expanded.png") });
    return { retainedSummary, invalidExpanded };
  });

  await scenario("native-enter-and-browser-composition", async () => {
    const { page, translations: t } = await pageFor({ locale: "en" });
    const title = page.getByLabel(t.creation.titleLabel, { exact: true });
    await title.fill("Native Enter proof");
    await page.getByLabel(t.creation.description, { exact: true }).fill("Plain description retained");
    await title.focus();
    const cdp = await page.context().newCDPSession(page);
    await page.evaluate(() => {
      window.fixture.keyEvents = [];
      document.addEventListener("keydown", (event) => window.fixture.keyEvents.push({ key: event.key, isComposing: event.isComposing, keyCode: event.keyCode, defaultPrevented: event.defaultPrevented }), true);
      document.addEventListener("compositionstart", () => window.fixture.keyEvents.push({ type: "compositionstart" }), true);
      document.addEventListener("compositionend", () => window.fixture.keyEvents.push({ type: "compositionend" }), true);
    });
    await cdp.send("Input.imeSetComposition", { text: "中文", selectionStart: 2, selectionEnd: 2 });
    await page.keyboard.press("Enter");
    await settled(page);
    const duringCompositionPosts = await page.evaluate(() => window.fixture.requests.filter((request) => request.path === "/api/collaboration/issues" && request.method === "POST").length);
    const keyEvents = await page.evaluate(() => window.fixture.keyEvents);
    result.diagnostics["native-enter-and-browser-composition"] = { duringCompositionPosts, keyEvents };
    if (duringCompositionPosts > 0) { await cdp.detach(); throw new Error("Composition Enter submitted: " + JSON.stringify(keyEvents)); }
    await cdp.send("Input.insertText", { text: "中文" }); await cdp.detach();
    const finalTitle = await title.inputValue();
    await title.press("Enter");
    await expect(page.getByRole("dialog", { name: t.creation.title, exact: true })).toHaveCount(0);
    const posts = await page.evaluate(() => window.fixture.requests.filter((request) => request.path === "/api/collaboration/issues" && request.method === "POST"));
    const evidence = { duringCompositionPosts, postCount: posts.length, input: posts[0]?.body, finalTitle, caveat: "Chromium CDP composition; no physical OS IME tested" };
    result.diagnostics["native-enter-and-browser-composition"] = evidence;
    expect(duringCompositionPosts).toBe(0); expect(posts).toHaveLength(1); expect(posts[0].body.title).toBe(finalTitle); expect(posts[0].body.description).toBe("Plain description retained");
    return evidence;
  });

  for (const theme of ["light", "dark"]) await scenario(`english-320-empty-${theme}`, async () => {
    const { page, translations: t } = await pageFor({ width: 320, height: 568, coarse: true, locale: "en", theme });
    const source = page.getByLabel(t.creation.source, { exact: true });
    const body = page.locator("[data-collaboration-creation-body]");
    const hint = page.getByText(t.creation.titleRequired, { exact: true });
    const help = body.getByText(t.creation.help, { exact: true });
    const evidence = {
      sourceTextFit: await source.evaluate((select) => {
        const ctx = document.createElement("canvas").getContext("2d"); ctx.font = getComputedStyle(select).font;
        return { needed: ctx.measureText(select.selectedOptions[0]?.textContent ?? "").width + 20, actual: select.getBoundingClientRect().width };
      }),
      cancel: await geometry(page.getByRole("button", { name: t.creation.cancel, exact: true })),
      submit: await geometry(page.getByRole("button", { name: t.creation.submit, exact: true })),
      source: await geometry(source), body: await geometry(body),
      initialFocus: await page.evaluate(() => ({ tag: document.activeElement.tagName, role: document.activeElement.getAttribute("role"), id: document.activeElement.id })),
      sourceLabel: await source.evaluate((select) => select.selectedOptions[0]?.textContent),
      titleFocused: await page.getByLabel(t.creation.titleLabel, { exact: true }).evaluate((input) => document.activeElement === input),
      hintInitially: await hint.count() ? await geometry(hint) : null,
      help: await help.count() ? await geometry(help) : null,
      viewportOverflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
    };
    await page.screenshot({ path: resolve(artifact, `english-320-empty-${theme}.png`) });
    if (await hint.count()) { await hint.scrollIntoViewIfNeeded(); await settled(page); evidence.hintScrolled = await geometry(hint); }
    await page.screenshot({ path: resolve(artifact, `english-320-hint-${theme}.png`) });
    result.diagnostics[`english-320-empty-${theme}`] = evidence;
    expect(Math.abs(evidence.cancel.top - evidence.submit.top)).toBeLessThan(2); expect(evidence.sourceTextFit.actual).toBeGreaterThanOrEqual(evidence.sourceTextFit.needed); expect(evidence.hintInitially?.visible).toBe(true); expect(evidence.titleFocused).toBe(true); expect(evidence.viewportOverflow).toBe(false);
    return evidence;
  });

  for (const theme of ["light", "dark"]) await scenario(`reference-722-empty-${theme}`, async () => {
    const { page, translations: t } = await pageFor({ width: 722, height: 660, theme, longSource: true });
    const body = page.locator("[data-collaboration-creation-body]");
    const title = page.getByLabel(t.creation.titleLabel, { exact: true });
    const description = page.getByLabel(t.creation.description, { exact: true });
    const summary = page.locator("summary").filter({ hasText: t.fields.initial });
    const recipient = page.getByRole("button", { name: t.creation.recipient, exact: true });
    const actor = await page.evaluate(() => window.fixture.actor);
    const scroll = await body.evaluate((element) => ({ top: element.scrollTop, client: element.clientHeight, content: element.scrollHeight }));
    const titleFocus = await focusMeasurement(page, title);
    const summaryGeometry = await geometry(summary);
    const recipientIdentity = await identityMeasurement(recipient, actor);
    const emptyAttachments = await body.getByRole("group", { name: t.attachments.title, exact: true }).evaluate((group) => ({
      legendVisible: [...group.querySelectorAll("legend")].some((legend) => { const box = legend.getBoundingClientRect(); return box.height > 2 && box.width > 2; }),
      sharingText: group.textContent,
    }));
    const descriptionGeometry = await geometry(description);
    const evidence = { scroll, titleFocus, summaryGeometry, recipientIdentity, emptyAttachments, descriptionGeometry,
      body: await geometry(body), footer: await geometry(page.locator("[data-collaboration-creation-actions]")),
      overflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), viewport: { width: 722, height: 660 } };
    result.diagnostics[`reference-722-empty-${theme}`] = evidence;
    await page.screenshot({ path: resolve(artifact, `reference-722-empty-${theme}.png`) });
    await page.getByRole("dialog", { name: t.creation.title, exact: true }).screenshot({ path: resolve(artifact, `dialog-default-722-${theme}.png`) });
    expect(summaryGeometry.visible).toBe(true); expect(scroll.top).toBe(0); expect(scroll.content).toBeLessThanOrEqual(scroll.client + 1);
    expect(descriptionGeometry.height).toBeLessThanOrEqual(72); expect(recipientIdentity.visible).toBe(true); expect(emptyAttachments.legendVisible).toBe(false);
    expect(titleFocus.bottomBorderPresent).toBe(true); expect(titleFocus.ringPresent).toBe(false); expect(titleFocus.effectiveOpacity).toBe(1); expect(titleFocus.ratio).toBeGreaterThanOrEqual(3); expect(evidence.overflow).toBe(false);
    return evidence;
  });

  for (const width of [390, 320]) for (const locale of ["zh-Hans", "en"]) for (const theme of ["light", "dark"]) {
    if (width === 320 && locale === "en") continue; // Canonical English empty-source/footer scenario above.
    await scenario(`empty-${width}-${locale}-${theme}`, async () => {
      const { page, translations: t } = await pageFor({ width, height: width === 390 ? 740 : 568, coarse: true, locale, theme });
      const body = page.locator("[data-collaboration-creation-body]");
      const title = page.getByLabel(t.creation.titleLabel, { exact: true });
      const submit = page.getByRole("button", { name: t.creation.submit, exact: true });
      const cancel = page.getByRole("button", { name: t.creation.cancel, exact: true });
      const hint = page.getByText(t.creation.titleRequired, { exact: true });
      const evidence = { body: await geometry(body), submit: await geometry(submit), cancel: await geometry(cancel), hint: await geometry(hint), titleFocus: await focusMeasurement(page, title),
        overflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth) };
      await page.screenshot({ path: resolve(artifact, `empty-${width}-${locale}-${theme}.png`) });
      result.diagnostics[`empty-${width}-${locale}-${theme}`] = evidence;
      expect(evidence.submit.visible).toBe(true); expect(evidence.cancel.visible).toBe(true); expect(evidence.hint.visible).toBe(true); expect(evidence.overflow).toBe(false);
      expect(evidence.titleFocus.bottomBorderPresent).toBe(true); expect(evidence.titleFocus.ratio).toBeGreaterThanOrEqual(3);
      return evidence;
    });
  }

  for (const locale of ["zh-Hans", "en"]) for (const theme of ["light", "dark"]) await scenario(`zoom-reflow-200-${locale}-${theme}`, async () => {
    const { page, translations: t } = await pageFor({ width: 640, height: 450, pixelScale: 2, locale, theme, longSource: true });
    const title = page.getByLabel(t.creation.titleLabel, { exact: true });
    const body = page.locator("[data-collaboration-creation-body]");
    const actions = page.locator("[data-collaboration-creation-actions]");
    const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
    const initial = { body: await geometry(body), actions: await geometry(actions), close: await geometry(dialog.getByRole("button", { name: t.creation.close, exact: true })) };
    await page.screenshot({ path: resolve(artifact, `zoom-reflow-200-${locale}-${theme}-top.png`) });
    await body.evaluate((element) => { element.scrollTop = element.scrollHeight; }); await settled(page);
    const summary = await geometry(page.locator("summary").filter({ hasText: t.fields.initial }));
    const bottom = await geometry(actions);
    const overflow = await page.evaluate(() => ({ document: document.documentElement.scrollWidth > innerWidth, body: document.body.scrollWidth > innerWidth, cssViewport: [innerWidth,innerHeight], dpr: devicePixelRatio }));
    await page.screenshot({ path: resolve(artifact, `zoom-reflow-200-${locale}-${theme}-bottom.png`) });
    const evidence = { method: "200% equivalent reflow: 1280x900 device pixels represented by 640x450 CSS viewport with DPR=2; no native browser-toolbar/OS zoom claimed", initial, summary, bottom, overflow };
    result.diagnostics[`zoom-reflow-200-${locale}-${theme}`] = evidence;
    expect(initial.actions.visible).toBe(true); expect(initial.close.visible).toBe(true); expect(summary.visible).toBe(true); expect(bottom.visible).toBe(true); expect(overflow.document).toBe(false); expect(overflow.body).toBe(false);
    await page.getByRole("button", { name: t.attachments.add, exact: true }).focus();
    await title.focus(); await settled(page); expect((await geometry(title)).visible).toBe(true);
    return evidence;
  });

  for (const theme of ["light", "dark"]) await scenario(`delayed-source-focus-${theme}`, async () => {
    const config = { width: 390, height: 740, coarse: true, theme, locale: "en" };
    const { page, translations: t } = await pageFor({ ...config, manualOpen: true, delaySource: true });
    const entry = page.getByRole("button", { name: t.creation.entry, exact: true });
    const background = page.getByLabel("Background fixture input", { exact: true });
    await background.focus();
    const position = await entry.boundingBox();
    await page.evaluate(() => {
      const original = requestAnimationFrame.bind(window), cancel = cancelAnimationFrame.bind(window);
      let next = 100000; const held = new Map();
      window.requestAnimationFrame = (callback) => { const id = ++next; held.set(id, callback); return id; };
      window.cancelAnimationFrame = (id) => { if (!held.delete(id)) cancel(id); };
      window.fixture.heldFrames = () => held.size;
      window.fixture.releaseFrames = () => { window.requestAnimationFrame = original; window.cancelAnimationFrame = cancel; const callbacks = [...held.values()]; held.clear(); callbacks.forEach((callback) => original(callback)); };
    });
    await page.touchscreen.tap(position.x + position.width / 2, position.y + position.height / 2);
    const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
    const source = page.getByLabel(t.creation.source, { exact: true });
    await expect(source).toBeVisible();
    await page.waitForFunction(() => window.fixture.pendingSourceCount() > 0 && window.fixture.heldFrames() > 0, null, { polling: 20 });
    await source.focus();
    await source.selectOption(await page.evaluate(() => window.fixture.secondSource));
    const heldFrames = await page.evaluate(() => window.fixture.heldFrames());
    await page.evaluate(() => window.fixture.releaseSources());
    const title = page.getByLabel(t.creation.titleLabel, { exact: true });
    await expect(title).toBeVisible();
    const beforeFrameSourceFocus = await source.evaluate((element) => document.activeElement === element);
    await page.evaluate(() => window.fixture.releaseFrames()); await settled(page);
    const afterFrameSourceFocus = await source.evaluate((element) => document.activeElement === element);
    const selectedValue = await source.inputValue();
    await page.screenshot({ path: resolve(artifact, `focus-delayed-selected-source-${theme}.png`) });
    await dialog.getByRole("button", { name: t.creation.close, exact: true }).tap();
    await expect(dialog).toHaveCount(0); await expect(entry).toBeFocused();

    const idle = await pageFor({ ...config, manualOpen: true, delaySource: true });
    await idle.page.getByLabel("Background fixture input", { exact: true }).focus();
    await idle.page.getByRole("button", { name: idle.translations.creation.entry, exact: true }).tap();
    await expect(idle.page.getByLabel(idle.translations.creation.source, { exact: true })).toBeVisible();
    await settled(idle.page);
    const touchBackgroundFocusContained = await idle.page.evaluate(() => Boolean(document.activeElement.closest('[role="dialog"]')));
    await idle.page.evaluate(() => window.fixture.releaseSources());
    const idleTitle = idle.page.getByLabel(idle.translations.creation.titleLabel, { exact: true });
    await expect(idleTitle).toBeFocused();
    const delayedKnownTitleFocused = await idleTitle.evaluate((element) => document.activeElement === element);
    await idle.page.getByRole("button", { name: idle.translations.creation.close, exact: true }).tap();
    await expect(idle.page.getByRole("button", { name: idle.translations.creation.entry, exact: true })).toBeFocused();

    const empty = await pageFor({ ...config, manualOpen: true, noSource: true });
    await empty.page.getByRole("button", { name: empty.translations.creation.entry, exact: true }).tap();
    const emptySource = empty.page.getByLabel(empty.translations.creation.source, { exact: true });
    await expect(emptySource).toBeFocused();
    expect(await emptySource.inputValue()).toBe("");
    await empty.page.getByRole("button", { name: empty.translations.creation.close, exact: true }).tap();
    await expect(empty.page.getByRole("button", { name: empty.translations.creation.entry, exact: true })).toBeFocused();
    const evidence = { heldFrames, beforeFrameSourceFocus, afterFrameSourceFocus, selectedValue, touchBackgroundFocusContained, delayedKnownTitleFocused,
      emptySourceFocused: true, closeReturnsOpener: true, method: "Real Chromium DOM/touch with delayed mock source response; only one subcase temporarily defers rAF to make the initial-focus race deterministic, then restores native scheduling" };
    result.diagnostics[`delayed-source-focus-${theme}`] = evidence;
    expect(beforeFrameSourceFocus).toBe(true); expect(afterFrameSourceFocus).toBe(true); expect(touchBackgroundFocusContained).toBe(true);
    return evidence;
  });
  const assistLabel = async (page) => page.evaluate(() => window.fixture.translations.creation.assist ?? window.fixture.assistTranslations.instructions);
  const expandControl = (page, expanded = false) => page.getByRole("button", { name: expanded ? /Collapse|收起|缩小/ : /Expand|展开|放大/ });
  async function assistStart(page, t, text = "  原始描述\n保留空白  ") {
    const input = page.getByLabel(t.creation.description, { exact: true });
    await input.fill(text);
    await page.getByRole("button", { name: await assistLabel(page), exact: true }).click();
    await page.waitForFunction(() => window.fixture.aiState().length > 0);
    return input;
  }

  for (const size of [{ width: 1280, height: 900 }, { width: 390, height: 740 }, { width: 320, height: 568 }]) for (const theme of ["light", "dark"]) {
    await scenario(`expand-${size.width}-${theme}`, async () => {
      const { page, translations: t } = await pageFor({ ...size, theme, coarse: size.width < 600 });
      const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
      const title = page.getByLabel(t.creation.titleLabel, { exact: true });
      const description = page.getByLabel(t.creation.description, { exact: true });
      const helper = page.getByRole("button", { name: await assistLabel(page), exact: true });
      await expect(helper).toBeDisabled();
      const initial = await geometry(dialog);
      await page.screenshot({ path: resolve(artifact, `default-${size.width}-${theme}.png`) });
      await title.fill("Expansion keeps the original draft");
      await page.evaluate(() => window.fixture.seedAttachments(1));
      await expect(page.getByRole("checkbox")).toHaveCount(1);
      await assistStart(page, t);
      await page.evaluate(() => window.fixture.emitAI("text_delta", { text: "Provisional AI only" }));
      await expect(description).toHaveValue("  原始描述\n保留空白  ");
      const expand = expandControl(page); await expand.focus(); await page.keyboard.press("Enter"); await settled(page);
      const expanded = await geometry(dialog);
      const collapse = expandControl(page, true);
      await expect(collapse).toHaveAttribute("aria-pressed", "true");
      const controls = { expand: await geometry(collapse), close: await geometry(dialog.getByRole("button", { name: t.creation.close, exact: true })), footer: await geometry(page.locator("[data-collaboration-creation-actions]")) };
      await page.screenshot({ path: resolve(artifact, `expanded-${size.width}-${theme}.png`) });
      expect(expanded.left).toBeGreaterThanOrEqual(11); expect(expanded.right).toBeLessThanOrEqual(size.width - 11);
      expect(expanded.top).toBeGreaterThanOrEqual(11); expect(expanded.bottom).toBeLessThanOrEqual(size.height - 11);
      expect(controls.footer.visible).toBe(true); expect(controls.expand.right).toBeLessThanOrEqual(controls.close.left + 2);
      if (size.width < 600) { expect(controls.expand.width).toBeGreaterThanOrEqual(44); expect(controls.expand.height).toBeGreaterThanOrEqual(44); }
      await collapse.click(); await settled(page);
      expect((await geometry(dialog)).width).toBeCloseTo(initial.width, 0);
      await expect(title).toHaveValue("Expansion keeps the original draft"); await expect(description).toHaveValue("  原始描述\n保留空白  ");
      const streaming = await page.evaluate(() => window.fixture.aiState()[0]); expect(streaming.cancelled).toBe(false); expect(streaming.aborted).toBe(false);
      const ai = await page.evaluate(() => window.fixture.assistTranslations);
      await page.locator("[data-collaboration-creation-body]").getByRole("button", { name: ai.cancel, exact: true }).click();
      await dialog.getByRole("button", { name: t.creation.close, exact: true }).click(); await expect(dialog).toHaveCount(0);
      await page.getByRole("button", { name: t.creation.entry, exact: true }).click(); await settled(page);
      await expect(expandControl(page)).toHaveAttribute("aria-pressed", "false"); await expect(title).toHaveValue("Expansion keeps the original draft");
      return { initial, expanded, controls, streaming, draftRetained: true, resetOnReopen: true };
    });
  }

  for (const theme of ["light", "dark"]) await scenario(`assist-adopt-undo-${theme}`, async () => {
    const { page, translations: t } = await pageFor({ width: 390, height: 740, coarse: true, theme });
    const original = "  保留原始描述\n\n第二行  ";
    const input = await assistStart(page, t, original);
    const ai = await page.evaluate(() => window.fixture.assistTranslations);
    const partial = "尚未完成的文本";
    await page.evaluate((text) => window.fixture.emitAI("text_delta", { text }), partial);
    await expect(input).toHaveValue(original);
    const generated = "明确交付目标\n" + "验收标准：保留原有行为，检查边界和结果。\n".repeat(25);
    await page.evaluate((text) => window.fixture.emitAI("done", { text, questions: ["交付时间是什么？", "由谁验收？"] }), generated);
    await expect(input).toHaveValue(generated);
    await expect(page.getByLabel("交付时间是什么？", { exact: true })).toBeVisible();
    await page.screenshot({ path: resolve(artifact, `assist-adopt-long-${theme}.png`) });
    const clarification = page.getByLabel("交付时间是什么？", { exact: true });
    await clarification.focus(); await settled(page);
    const clarificationGeometry = await geometry(clarification);
    result.diagnostics[`long-output-clarification-${theme}`] = { clarificationGeometry, footer: await geometry(page.locator("[data-collaboration-creation-actions]")) };
    await page.screenshot({ path: resolve(artifact, `assist-long-clarification-${theme}.png`) });
    expect(clarificationGeometry.visible).toBe(true);
    const fields = await page.evaluate(() => window.fixture.requests.filter((request) => request.path === "/api/issues/optimize-description"));
    expect(fields).toHaveLength(1); expect(fields[0].workspace).toBe(await page.evaluate(() => window.fixture.source)); expect(fields[0].body.mode).toBe("manual"); expect(fields[0].body.text).toBe(original);
    expect(Object.keys(fields[0].body).sort()).toEqual(["mode", "text", "title"].filter((key) => key in fields[0].body).sort());
    await page.getByRole("button", { name: ai.undo, exact: true }).click(); await expect(input).toHaveValue(original);
    await page.getByRole("button", { name: await assistLabel(page), exact: true }).click();
    await page.waitForFunction(() => window.fixture.aiState().length === 2);
    await page.evaluate(() => window.fixture.emitAI("done", { text: "最终草稿", questions: ["交付时间是什么？"] }));
    await expect(input).toHaveValue("最终草稿"); await input.fill("用户继续编辑的最新草稿");
    await page.getByLabel("交付时间是什么？", { exact: true }).fill("下周三");
    await page.getByRole("button", { name: ai.merge_answers, exact: true }).click();
    await expect(input).toHaveValue(/用户继续编辑的最新草稿[\s\S]*下周三/);
    await page.getByRole("button", { name: ai.undo, exact: true }).click(); await expect(input).toHaveValue("用户继续编辑的最新草稿");
    return { explicitWorkspace: fields[0].workspace, rawOriginalPreserved: true, longOutputLength: generated.length, mergeLatestAndUndo: true, ambientFetches: await page.evaluate(() => window.fixture.requests.filter((request) => request.path !== "/api/issues/optimize-description" && !request.path.startsWith("/api/collaboration/")).length) };
  });

  await scenario("assist-stale-source-and-inert-preview", async () => {
    const { page, translations: t } = await pageFor({ width: 320, height: 568, coarse: true, locale: "en" });
    const input = await assistStart(page, t, "Original source draft");
    await input.fill("User edits win");
    const unsafe = '![remote](https://forbidden.invalid/image.png) <img src="/api/files/private"> [file](/api/attachments/private)';
    await page.evaluate((text) => window.fixture.emitAI("done", { text, questions: [] }), unsafe);
    await expect(input).toHaveValue("User edits win"); await expect(page.getByText(unsafe, { exact: true })).toBeVisible();
    const preview = page.getByText(unsafe, { exact: true }); expect(await preview.locator("img,a,iframe").count()).toBe(0);
    const ai = await page.evaluate(() => window.fixture.assistTranslations);
    await page.getByRole("button", { name: ai.discard, exact: true }).click();
    await page.getByRole("button", { name: await assistLabel(page), exact: true }).click();
    await page.waitForFunction(() => window.fixture.aiState().length === 2);
    await page.getByLabel(t.creation.source, { exact: true }).selectOption(await page.evaluate(() => window.fixture.secondSource));
    await expect(input).toHaveValue("");
    const lateAccepted = await page.evaluate(() => window.fixture.emitAI("done", { text: "Old source must not replace new draft", questions: [] }, 1));
    await settled(page); await expect(input).toHaveValue("");
    const state = await page.evaluate(() => window.fixture.aiState()[1]); expect(state.aborted || state.cancelled).toBe(true);
    await assistStart(page, t, "Second source description");
    const request = await page.evaluate(() => window.fixture.requests.filter((entry) => entry.path === "/api/issues/optimize-description").at(-1));
    expect(request.workspace).toBe(await page.evaluate(() => window.fixture.secondSource));
    await page.screenshot({ path: resolve(artifact, "assist-second-source.png") });
    return { staleEditsRetained: true, inertPreview: true, lateAccepted, state, selectedWorkspace: request.workspace };
  });

  await scenario("assist-errors-limits-partial-payload", async () => {
    const { page, translations: t } = await pageFor({ locale: "en", width: 320, height: 568, coarse: true });
    const input = page.getByLabel(t.creation.description, { exact: true }); const title = page.getByLabel(t.creation.titleLabel, { exact: true });
    const ai = await page.evaluate(() => window.fixture.assistTranslations);
    await title.fill("Create only accepted text"); await input.fill("Current content survives every AI error"); await page.evaluate(() => window.fixture.seedAttachments(1));
    for (const mode of ["missing", "unavailable", "error"]) {
      await page.evaluate((mode) => window.fixture.setAIMode(mode), mode);
      const retry = page.getByRole("button", { name: ai.retry, exact: true });
      if (await retry.count()) await retry.click(); else await page.getByRole("button", { name: await assistLabel(page), exact: true }).click();
      await expect(page.getByRole("alert")).toBeVisible(); await expect(input).toHaveValue("Current content survives every AI error"); await expect(page.getByRole("checkbox")).toHaveCount(1);
    }
    await page.evaluate(() => window.fixture.setAIMode("stream")); await page.getByRole("button", { name: ai.retry, exact: true }).click();
    await page.waitForFunction(() => window.fixture.aiState().length === 1);
    await page.evaluate(() => window.fixture.emitAI("text_delta", { text: "Partial should never be persisted" }));
    await page.locator("[data-collaboration-creation-body]").getByRole("button", { name: ai.cancel, exact: true }).click();
    await expect(input).toHaveValue("Current content survives every AI error");
    await input.fill("😀".repeat(20001));
    const helper = page.getByRole("button", { name: await assistLabel(page), exact: true }); await expect(helper).toBeDisabled();
    const limitCopy = await page.locator("[data-collaboration-creation-body]").innerText(); expect(limitCopy).toMatch(/20.?000/);
    await expect(page.getByRole("button", { name: t.creation.submit, exact: true })).toBeEnabled();
    await input.fill("Accepted ordinary plaintext"); await title.fill("x".repeat(501)); await expect(helper).toBeDisabled();
    expect(await page.locator("[data-collaboration-creation-body]").innerText()).toMatch(/500/); await title.fill("Create only accepted text");
    await helper.click(); await page.waitForFunction(() => window.fixture.aiState().length === 2);
    await page.evaluate(() => window.fixture.emitAI("text_delta", { text: "Another partial, never sent" }));
    await page.getByRole("button", { name: t.creation.submit, exact: true }).click();
    await expect(page.getByRole("dialog", { name: t.creation.title, exact: true })).toHaveCount(0);
    const posts = await page.evaluate(() => window.fixture.requests.filter((request) => request.path === "/api/collaboration/issues" && request.method === "POST"));
    expect(posts).toHaveLength(1); expect(posts[0].body.description).toBe("Accepted ordinary plaintext");
    return { errorsPreserveDraftFiles: true, bodyUnicodeLimit: 20001, titleLimit: 501, payload: posts[0].body };
  });

  for (const theme of ["light", "dark"]) await scenario(`assist-delivery-screenshots${theme === "dark" ? "-dark" : ""}`, async () => {
    const { page, translations: t } = await pageFor({ width: 722, height: 660, longSource: true, theme });
    const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
    const summary = page.locator("summary").filter({ hasText: t.fields.initial });
    result.diagnostics[`default-722-density-${theme}`] = { summary: await geometry(summary), body: await geometry(page.locator("[data-collaboration-creation-body]")), footer: await geometry(page.locator("[data-collaboration-creation-actions]")), scroll: await page.locator("[data-collaboration-creation-body]").evaluate((element) => ({ top: element.scrollTop, client: element.clientHeight, content: element.scrollHeight })) };
    expect((await geometry(summary)).visible).toBe(true);
    await page.screenshot({ path: resolve(artifact, `default-722-assist-${theme}.png`) });
    await dialog.screenshot({ path: resolve(artifact, `dialog-default-722-assist-${theme}.png`) });
    await page.getByLabel(t.creation.titleLabel, { exact: true }).fill("完善跨团队交付说明");
    await assistStart(page, t, "需要确认交付范围和验收方式");
    await expandControl(page).click(); await settled(page);
    await page.evaluate(() => window.fixture.emitAI("done", { text: "交付目标：明确跨团队协作范围。\n\n验收标准：双方确认结果，保留原有任务及文件。", questions: ["期望什么时候完成？"] }));
    await expect(page.getByRole("button", { name: "撤销", exact: true })).toBeVisible();
    await settled(page);
    await page.screenshot({ path: resolve(artifact, `expanded-722-ai-result-${theme}.png`) });
    await dialog.screenshot({ path: resolve(artifact, `dialog-expanded-722-ai-result-${theme}.png`) });
    return { defaultAndExpandedCaptured: true };
  });

  for (const size of [{width:901,height:1042},{width:1280,height:900},{width:390,height:740},{width:320,height:568}]) for(const theme of ["light","dark"]) {
    await scenario(`expanded-editor-${size.width}-${theme}`,async()=>{
      const {page,translations:t}=await pageFor({...size,theme,coarse:size.width<600});
      const title=page.getByLabel(t.creation.titleLabel,{exact:true});const desc=page.getByLabel(t.creation.description,{exact:true});
      const dialog=page.getByRole('dialog',{name:t.creation.title,exact:true});const body=page.locator('[data-collaboration-creation-body]');const footer=page.locator('[data-collaboration-creation-actions]');
      await title.fill('Keep draft and editor identity');await desc.fill('Short plain description');
      await desc.evaluate(el=>{window.fixture.originalTextarea=el});
      const initial={dialog:await geometry(dialog),description:await geometry(desc),footer:await geometry(footer)};
      await page.screenshot({path:resolve(artifact,`default-${size.width}-${theme}.png`)});
      await page.getByRole('button',{name:t.creation.expand,exact:true}).click();await settled(page);
      const layout=async()=>({dialog:await geometry(dialog),description:await geometry(desc),body:await geometry(body),footer:await geometry(footer),scroll:await body.evaluate(el=>({top:el.scrollTop,height:el.clientHeight,content:el.scrollHeight})),editorOverflow:await desc.evaluate(el=>({vertical:getComputedStyle(el).overflowY,client:el.clientHeight,scroll:el.scrollHeight})),documentOverflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)});
      const expanded=await layout();
      await page.screenshot({path:resolve(artifact,`expanded-${size.width}-${theme}.png`)});
      if(size.width===901)await dialog.screenshot({path:resolve(artifact,`dialog-expanded-901-${theme}.png`)});
      const summary=page.locator('summary').filter({hasText:t.fields.initial});await summary.click();await settled(page);const advanced=await layout();
      await summary.click();await page.evaluate(()=>window.fixture.seedAttachments(3));await expect(page.getByRole('checkbox')).toHaveCount(3);await settled(page);const files=await layout();
      const longText='Keep this long description without an inner scrollbox.\n'.repeat(30);await desc.fill(longText);await settled(page);const long=await layout();
      await page.screenshot({path:resolve(artifact,`expanded-long-${size.width}-${theme}.png`)});
      await desc.fill('Ongoing AI source');await page.getByRole('button',{name:t.creation.assist,exact:true}).click();await page.waitForFunction(()=>window.fixture.aiState().length===1);
      await page.evaluate(()=>window.fixture.emitAI('text_delta',{text:'Display-only stream'}));
      const collapse=page.getByRole('button',{name:t.creation.collapse,exact:true});await collapse.focus();await page.keyboard.press('Enter');await settled(page);
      const nodeSame=await desc.evaluate(el=>el===window.fixture.originalTextarea);const toggleFocused=await page.getByRole('button',{name:t.creation.expand,exact:true}).evaluate(el=>document.activeElement===el);
      await expect(desc).toHaveValue('Ongoing AI source');expect(nodeSame).toBe(true);expect(toggleFocused).toBe(true);expect((await page.evaluate(()=>window.fixture.aiState()[0])).aborted).toBe(false);
      await page.getByRole('button',{name:t.creation.expand,exact:true}).click();await expect(dialog).toHaveAttribute('data-expanded','true');await settled(page);await page.evaluate(()=>window.fixture.emitAI('done',{text:'Accepted AI result',questions:['How is this accepted?']}));
      const answer=page.getByLabel('How is this accepted?',{exact:true});await expect(answer).toBeVisible();const questions=await layout();await answer.focus();await settled(page);const answerGeometry=await geometry(answer);
      await page.screenshot({path:resolve(artifact,`expanded-questions-${size.width}-${theme}.png`)});
      const close=page.getByRole('button',{name:t.creation.close,exact:true});await close.click();await expect(dialog).toHaveCount(0);await expect(page.getByRole('button',{name:t.creation.entry,exact:true})).toBeFocused();
      await page.getByRole('button',{name:t.creation.entry,exact:true}).click();await expect(desc).toHaveValue('Accepted AI result');await expect(page.getByRole('checkbox')).toHaveCount(3);
      const resultValue={initial,expanded,advanced,files,long,questions,answerGeometry,nodeSame,toggleFocused,closeReturnsFocus:true,draftFilesRetained:true};result.diagnostics[`expanded-editor-${size.width}-${theme}`]=resultValue;
      if(size.width>=900){expect(expanded.description.height).toBeGreaterThan(initial.description.height+100);expect(advanced.description.height).toBeLessThan(expanded.description.height);expect(files.description.height).toBeLessThan(expanded.description.height);}
      expect(expanded.footer.visible).toBe(true);expect(advanced.footer.visible).toBe(true);expect(files.footer.visible).toBe(true);expect(long.footer.visible).toBe(true);expect(questions.footer.visible).toBe(true);expect(answerGeometry.visible).toBe(true);
      expect(expanded.documentOverflow).toBe(false);expect(long.documentOverflow).toBe(false);expect(long.editorOverflow.scroll).toBeLessThanOrEqual(long.editorOverflow.client+1);
      return resultValue;
    });
  }
} finally {
  await Promise.allSettled(contexts.map((context) => context.close()));
  if (browser) await browser.close();
  await new Promise((resolve) => server.close(resolve));
  result.listenerClosed = true; result.browserClosed = true; result.finishedAt = new Date().toISOString();
  await writeFile(resolve(artifact, "results.json"), JSON.stringify(result, null, 2) + "\n");
}
process.stdout.write(JSON.stringify({ mode, scenarios: Object.keys(result.scenarios), failures: result.failures.map(({ scenario, error }) => ({ scenario, error: error.slice(0, 500) })), pageErrors: result.pageErrors, artifact }) + "\n");
if (result.failures.length || result.pageErrors.length) process.exitCode = 1;
