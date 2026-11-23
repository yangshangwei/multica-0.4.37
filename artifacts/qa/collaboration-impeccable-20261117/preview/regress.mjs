import { chromium, expect } from "@playwright/test";
import { createServer } from "node:http";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { createHash } from "node:crypto";
import { dirname, extname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const preview = dirname(fileURLToPath(import.meta.url));
const mode = process.argv[2] ?? "regression";
const build = JSON.parse(await readFile(resolve(preview, `${mode}-build.json`), "utf8"));
const artifacts = resolve(preview, "..", `browser-${mode}`);
await mkdir(artifacts, { recursive: true });
const result = { startedAt: new Date().toISOString(), build,
  driverSha256: createHash("sha256").update(await readFile(fileURLToPath(import.meta.url))).digest("hex"),
  scenarios: {}, failures: [], pageErrors: [], blockedRequests: [] };
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
await new Promise(done => server.listen(0, "127.0.0.1", done));
const origin = `http://127.0.0.1:${server.address().port}`;
let browser;
const settle = page => page.evaluate(async () => {
  await document.fonts.ready;
  await Promise.allSettled(document.getAnimations().filter(animation => animation.effect?.getComputedTiming().iterations !== Infinity).map(animation => animation.finished));
  await new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done)));
});
const rectangle = locator => locator.evaluate(element => {
  const box = element.getBoundingClientRect(), css = getComputedStyle(element);
  return { width: box.width, height: box.height, top: box.top, bottom: box.bottom, left: box.left, right: box.right,
    viewportWidth: innerWidth, viewportHeight: innerHeight, overflowY: css.overflowY, fieldSizing: css.fieldSizing };
});
const inside = box => box.width > 0 && box.height > 0 && box.left >= -1 && box.right <= box.viewportWidth + 1 && box.top >= -1 && box.bottom <= box.viewportHeight + 1;
const click = (locator, coarse) => coarse ? locator.tap() : locator.click();

async function open(surface, options = {}) {
  const size = { width: 1280, height: 800, locale: "en", theme: "light", coarse: false, enlarged: false, ...options };
  const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, hasTouch: size.coarse, isMobile: size.coarse, colorScheme: size.theme });
  const page = await context.newPage();
  page.setDefaultTimeout(7000);
  page.on("pageerror", error => result.pageErrors.push(error.message));
  await page.route("**/*", async route => {
    if (new URL(route.request().url()).origin === origin) await route.continue();
    else { result.blockedRequests.push(route.request().url()); await route.abort(); }
  });
  await page.goto(`${origin}/?surface=${surface}&locale=${size.locale}`);
  await page.evaluate(theme => document.documentElement.classList.toggle("dark", theme === "dark"), size.theme);
  if (size.enlarged) await page.addStyleTag({ content: `:root { --text-body: 28px; --text-body--line-height: 40px; --text-caption: 24px; --text-caption--line-height: 32px; --text-title-sm: 32px; --text-title-sm--line-height: 48px; --text-title: 36px; --text-title--line-height: 56px; --text-title-lg: 40px; --text-title-lg--line-height: 56px; }` });
  await page.waitForFunction(() => window.auditFixture?.ready === true);
  return { page, context, size, t: await page.evaluate(() => window.auditFixture.translations) };
}

async function scenario(name, run) {
  try { result.scenarios[name] = await run(); }
  catch (error) { result.failures.push({ scenario: name, error: String(error.stack ?? error) }); }
}

try {
  browser = await chromium.launch({ headless: true });
  result.engine = `Playwright Chromium ${browser.version()}`;
  for (const options of [
    { width: 1280, theme: "light" }, { width: 1280, theme: "dark" },
    { width: 320, height: 568, coarse: true },
    { width: 320, height: 568, coarse: true, theme: "dark", locale: "zh-Hans" },
    { width: 390, height: 740, coarse: true, theme: "dark", locale: "zh-Hans" },
  ]) await scenario(`fields-${options.width}-${options.theme ?? "light"}-${options.locale ?? "en"}`, async () => {
    const { page, context, size, t } = await open("fields", options);
    try {
      await expect(page.getByRole("region", { name: "Field audit" })).toBeVisible();
      await click(page.getByRole("button", { name: t.fields.labels, exact: true }), size.coarse);
      const design = page.getByRole("button", { name: "Design", exact: true });
      await expect(design).toHaveAttribute("aria-pressed", "true");
      await click(design, size.coarse);
      await expect(design).toHaveAttribute("aria-pressed", "false");
      await click(design, size.coarse);
      await settle(page);
      const labels = { ...await rectangle(design), ax: await design.ariaSnapshot(), coarsePointer: await page.evaluate(() => matchMedia("(pointer: coarse)").matches) };
      if (size.coarse) expect(labels.height).toBeGreaterThanOrEqual(44);
      await page.screenshot({ path: resolve(artifacts, `labels-${size.width}-${size.theme}-${size.locale}.png`) });
      await page.keyboard.press("Escape");
      await expect(page.getByPlaceholder(t.fields.findLabel, { exact: true })).toBeHidden();

      await click(page.getByRole("button", { name: /^Delivery type/ }), size.coarse);
      const search = page.getByRole("textbox", { name: size.locale === "en" ? "Filter options" : "筛选选项", exact: true });
      await search.fill("Choice 10");
      await expect(page.getByRole("button", { name: "Choice 1", exact: true })).toBeHidden();
      await expect(page.getByRole("button", { name: "Choice 10", exact: true })).toBeVisible();
      await settle(page);
      const choice = await rectangle(page.getByRole("button", { name: "Choice 10", exact: true }));
      if (size.coarse) expect(choice.height).toBeGreaterThanOrEqual(44);
      await page.screenshot({ path: resolve(artifacts, `custom-search-${size.width}-${size.theme}-${size.locale}.png`) });
      await search.press("Enter");
      await expect(page.getByLabel("Selected custom values")).toContainText('"select":"choice-10"');
      await expect(search).toBeHidden();
      await click(page.getByRole("button", { name: /^Delivery type/ }), size.coarse);
      await search.fill("NO_MATCH_EXPECTED");
      await expect(page.getByRole("button", { name: "Choice 10", exact: true })).toBeHidden();
      await search.press("Enter");
      await expect(page.getByLabel("Selected custom values")).toContainText('"select":"choice-10"');
      await expect(search).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(search).toBeHidden();

      await click(page.getByRole("button", { name: /^Review scopes/ }), size.coarse);
      await search.fill("Choice 10");
      const multi = page.getByRole("button", { name: "Choice 10", exact: true });
      await expect(multi).toHaveAttribute("aria-pressed", "false");
      await click(multi, size.coarse);
      await expect(multi).toHaveAttribute("aria-pressed", "true");
      await search.fill("NO_MATCH_EXPECTED");
      await search.press("Enter");
      await expect(page.getByLabel("Selected custom values")).toContainText('"multi":["choice-10"]');
      await page.keyboard.press("Escape");
      await expect(search).toBeHidden();

      await click(page.getByRole("button", { name: /^Acceptance criterion/ }), size.coarse);
      const editor = page.getByRole("textbox", { name: "Acceptance criterion", exact: true });
      await expect(editor).toBeFocused();
      await editor.fill("Verified criterion");
      await settle(page);
      const textEditor = { ...await rectangle(editor), ax: await editor.ariaSnapshot() };
      if (size.coarse) expect(textEditor.height).toBeGreaterThanOrEqual(44);
      expect(inside(textEditor)).toBe(true);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
      await editor.press("Enter");
      await expect(page.getByLabel("Selected custom values")).toContainText('"text":"Verified criterion"');
      await expect(editor).toBeHidden();
      await click(page.getByRole("button", { name: /^Estimated days/ }), size.coarse);
      const number = page.getByRole("spinbutton", { name: "Estimated days", exact: true });
      await number.fill("1.25");
      await number.press("Enter");
      await expect(page.getByLabel("Selected custom values")).toContainText('"number":1.25');
      await expect(number).toBeHidden();
      await click(page.getByRole("button", { name: /^Reference URL/ }), size.coarse);
      const url = page.getByRole("textbox", { name: "Reference URL", exact: true });
      await url.fill("https://example.invalid/review");
      await url.press("Enter");
      await expect(page.getByLabel("Selected custom values")).toContainText('"url":"https://example.invalid/review"');
      return { labels, choice, textEditor, correctSingleValue: "choice-10", noMatchPreserved: true, multiSelectionState: true,
        numericEnterCommit: 1.25, urlEnterCommit: true, synthesizedTouch: size.coarse };
    } finally { await context.close(); }
  });

  for (const options of [
    { width: 1280 }, { width: 1280, theme: "dark" },
    { width: 320, height: 568, coarse: true },
    { width: 320, height: 568, coarse: true, theme: "dark", locale: "zh-Hans" },
    { width: 390, height: 740, coarse: true, theme: "dark", locale: "zh-Hans" },
    { width: 640, height: 420 },
    { width: 320, height: 568, coarse: true, enlarged: true },
  ]) await scenario(`purge-${options.width}-${options.height ?? 800}-${options.theme ?? "light"}-${options.locale ?? "en"}${options.enlarged ? "-text200" : ""}`, async () => {
    const { page, context, size, t } = await open("memory", options);
    try {
      await click(page.getByRole("button", { name: t.memory.history, exact: true }), size.coarse);
      await click(page.getByRole("button", { name: t.memory.purge.replace("{{version}}", "1"), exact: true }), size.coarse);
      const dialog = page.getByRole("alertdialog");
      await expect(dialog).toBeVisible();
      const reason = "Reason for this version.\n".repeat(40);
      const editor = dialog.getByRole("textbox", { name: t.memory.purgeReason, exact: true });
      await editor.fill(reason);
      await settle(page);
      const confirm = dialog.getByRole("button", { name: t.memory.purgeConfirm, exact: true });
      const cancel = dialog.getByRole("button", { name: t.memory.cancel, exact: true });
      const measurements = { reasonBytes: Buffer.byteLength(reason, "utf8"), reasonValid: await page.evaluate(reason => window.auditFixture.reasonValid(reason), reason),
        dialog: await rectangle(dialog), textarea: await rectangle(editor), confirm: await rectangle(confirm), cancel: await rectangle(cancel),
        heading: await rectangle(dialog.getByRole("heading")), textScale: size.enlarged ? 2 : 1 };
      expect(measurements.reasonValid).toBe(true);
      expect(inside(measurements.dialog)).toBe(true);
      expect(inside(measurements.heading)).toBe(true);
      expect(inside(measurements.confirm)).toBe(true);
      expect(inside(measurements.cancel)).toBe(true);
      if (size.coarse) { expect(measurements.confirm.height).toBeGreaterThanOrEqual(44); expect(measurements.cancel.height).toBeGreaterThanOrEqual(44); }
      await expect(confirm).toBeEnabled();
      await expect(cancel).toBeEnabled();
      await page.screenshot({ path: resolve(artifacts, `purge-${size.width}-${size.height}-${size.theme}-${size.locale}${size.enlarged ? "-text200" : ""}.png`) });
      await click(cancel, size.coarse);
      await expect(dialog).toBeHidden();
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
      return measurements;
    } finally { await context.close(); }
  });

  for (const options of [{ width: 1280 }, { width: 390, height: 740, coarse: true, theme: "dark", locale: "zh-Hans" }])
    await scenario(`knowledge-${options.width}-${options.locale ?? "en"}`, async () => {
      const { page, context, size, t } = await open("knowledge", options);
      try {
        await click(page.getByRole("button", { name: /PRIVATE SUMMARY/ }), size.coarse);
        await expect(page.getByText("PRIVATE BODY", { exact: true })).toBeVisible();
        const reason = page.getByRole("textbox", { name: t.knowledge.reason, exact: true });
        await reason.fill("Keep my reason and cursor");
        await reason.focus();
        await reason.evaluate(element => { element.setSelectionRange(5, 10); window.auditReasonNode = element; });
        await page.evaluate(() => { window.auditFixture.holdCandidateRead(); window.auditFixture.refreshCandidate(); });
        await page.waitForFunction(() => window.auditFixture.candidatePending());
        await expect(page.getByText("PRIVATE BODY", { exact: true })).toBeHidden();
        await expect(reason).toBeFocused();
        await expect(reason).toBeEnabled();
        expect(await reason.evaluate(element => element === window.auditReasonNode)).toBe(true);
        expect(await reason.evaluate(element => [element.selectionStart, element.selectionEnd])).toEqual([5, 10]);
        await page.keyboard.type("LIVE");
        await expect(reason).toHaveValue("Keep LIVEason and cursor");
        await page.evaluate(() => window.auditFixture.releaseCandidate());
        await expect(page.getByText("PRIVATE BODY", { exact: true })).toBeVisible();
        await expect(reason).toBeFocused();
        expect(await reason.evaluate(element => element === window.auditReasonNode)).toBe(true);
        await reason.scrollIntoViewIfNeeded();
        await settle(page);
        await page.screenshot({ path: resolve(artifacts, `knowledge-${size.width}-${size.theme}-${size.locale}.png`) });

        await click(page.getByRole("button", { name: "Hide knowledge", exact: true }), size.coarse);
        const before = await page.evaluate(() => window.auditFixture.requests.filter(value => value === "knowledge/candidate").length);
        await page.evaluate(() => window.auditFixture.eraseKnowledge());
        await page.waitForFunction(() => !window.auditFixture.candidateCached());
        await settle(page);
        expect(await page.evaluate(() => window.auditFixture.requests.filter(value => value === "knowledge/candidate").length)).toBe(before);
        await click(page.getByRole("button", { name: "Show knowledge", exact: true }), size.coarse);
        await expect(page.getByText("PRIVATE BODY", { exact: true })).toBeVisible();
        await expect(reason).toHaveValue("Keep LIVEason and cursor");
        await expect(reason).not.toBeFocused();
        return { stableEditorNode: true, nativeSelectionPreserved: true, typingDuringRenewal: true, privateBodyErasedDuringRead: true,
          hiddenExpiryErases: true, hiddenCandidateReads: 0, freshReadOnReopen: true, noFocusOnReopen: true };
      } finally { await context.close(); }
    });

  for (const options of [{ width: 1280 }, { width: 390, height: 740, coarse: true, theme: "dark", locale: "zh-Hans" }])
    await scenario(`creation-stage-${options.width}-${options.locale ?? "en"}`, async () => {
      const { page, context, size, t } = await open("creation", options);
      try {
        await click(page.getByRole("button", { name: t.creation.entry, exact: true }), size.coarse);
        const dialog = page.getByRole("dialog", { name: t.creation.title, exact: true });
        await expect(dialog).toBeVisible();
        await dialog.getByRole("textbox", { name: t.creation.titleLabel, exact: true }).fill("Retained title during validation");
        await click(dialog.getByRole("button", { name: new RegExp(t.fields.initial) }), size.coarse);
        await click(dialog.getByRole("button", { name: t.fields.parent, exact: true }), size.coarse);
        await click(page.getByRole("button", { name: "AUD-10 Parent audit task", exact: true }), size.coarse);
        const stage = dialog.getByRole("spinbutton", { name: t.fields.stageInputLabel, exact: true });
        const submit = dialog.getByRole("button", { name: t.creation.submit, exact: true });
        await stage.fill("0");
        await expect(stage).toHaveAttribute("aria-invalid", "true");
        const descriptionIds = (await stage.getAttribute("aria-describedby")).split(" ");
        expect(descriptionIds.length).toBeGreaterThan(1);
        await expect(submit).toBeDisabled();
        await stage.scrollIntoViewIfNeeded();
        await settle(page);
        await page.screenshot({ path: resolve(artifacts, `creation-stage-${size.width}-${size.theme}-${size.locale}.png`) });
        await stage.fill("2");
        await expect(stage).not.toHaveAttribute("aria-invalid", "true");
        await expect(submit).toBeEnabled();
        await expect(dialog.getByRole("textbox", { name: t.creation.titleLabel, exact: true })).toHaveValue("Retained title during validation");
        await click(dialog.getByRole("button", { name: t.creation.cancel, exact: true }), size.coarse);
        await expect(dialog).toBeHidden();
        return { stageSpecificAccessibleError: true, invalidSubmitBlocked: true, correctionEnables: true, titleRetained: true };
      } finally { await context.close(); }
    });

  await scenario("workflow-read-retry", async () => {
    const { page, context, t } = await open("workflow");
    try {
      await page.getByRole("button", { name: t.delivery.return, exact: true }).click();
      const dialog = page.getByRole("dialog", { name: t.delivery.return, exact: true });
      const reason = dialog.getByRole("textbox", { name: t.delivery.revisionReason, exact: true });
      await reason.fill("Keep the revision request during retry");
      const submit = dialog.getByRole("button", { name: t.delivery.return, exact: true });
      await expect(submit).toBeDisabled();
      await dialog.getByRole("button", { name: t.page.refresh, exact: true }).click();
      await page.waitForFunction(() => window.auditFixture.statusPending());
      await expect(dialog.getByRole("status").filter({ hasText: t.page.loading })).toBeVisible();
      await expect(submit).toBeDisabled();
      await expect(reason).toHaveValue("Keep the revision request during retry");
      await page.evaluate(() => window.auditFixture.releaseStatus());
      await expect(submit).toBeEnabled();
      await expect(reason).toHaveValue("Keep the revision request during retry");
      await settle(page);
      await page.screenshot({ path: resolve(artifacts, "workflow-read-retry.png") });
      return { retrySameDialog: true, draftRetained: true, disabledDuringRead: true, enabledAfterAuthorizedRead: true,
        readCount: await page.evaluate(() => window.auditFixture.requests.filter(value => value === "workflow/status").length) };
    } finally { await context.close(); }
  });
} finally {
  if (browser) await browser.close();
  await new Promise(done => server.close(done));
  result.finishedAt = new Date().toISOString();
  await writeFile(resolve(artifacts, "results.json"), JSON.stringify(result, null, 2) + "\n");
}
console.log(JSON.stringify({ engine: result.engine, passed: Object.keys(result.scenarios).length, failures: result.failures,
  pageErrors: result.pageErrors, blockedRequests: result.blockedRequests }, null, 2));
if (result.failures.length || result.pageErrors.length || result.blockedRequests.length) process.exitCode = 1;
