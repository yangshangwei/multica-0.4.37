import { chromium, expect } from "@playwright/test";
import { createServer } from "node:http";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { dirname, extname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

const preview = dirname(fileURLToPath(import.meta.url));
const mode = process.argv[2] ?? "audit";
const build = JSON.parse(await readFile(resolve(preview, `${mode}-build.json`), "utf8"));
const artifacts = resolve(preview, "..", `browser-${mode}`);
await mkdir(artifacts, { recursive: true });
const result = { startedAt: new Date().toISOString(), build, driverSha256: createHash("sha256").update(await readFile(fileURLToPath(import.meta.url))).digest("hex"), scenarios: {}, pageErrors: [], blockedRequests: [] };
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
const settle = page => page.evaluate(async () => {
  await document.fonts.ready;
  await Promise.allSettled(document.getAnimations().filter(animation => animation.effect?.getComputedTiming().iterations !== Infinity).map(animation => animation.finished));
  await new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done)));
});
const rectangle = locator => locator.evaluate(element => {
  const box = element.getBoundingClientRect(), css = getComputedStyle(element);
  return { width: box.width, height: box.height, top: box.top, bottom: box.bottom, left: box.left, right: box.right,
    viewportWidth: innerWidth, viewportHeight: innerHeight, overflowY: css.overflowY,
    fieldSizing: css.fieldSizing, ariaSelected: element.getAttribute("aria-selected"), ariaPressed: element.getAttribute("aria-pressed") };
});
let browser;
try {
  browser = await chromium.launch({ headless: true });
  result.engine = `Playwright Chromium ${browser.version()}`;
  for (const width of [1280, 320]) {
    const context = await browser.newContext({ viewport: { width, height: width === 320 ? 568 : 800 }, hasTouch: width === 320, isMobile: width === 320 });
    const page = await context.newPage();
    page.setDefaultTimeout(7000);
    page.on("pageerror", error => result.pageErrors.push(error.message));
    await page.route("**/*", async route => {
      if (new URL(route.request().url()).origin === origin) await route.continue();
      else { result.blockedRequests.push(route.request().url()); await route.abort(); }
    });
    await page.goto(`${origin}/?surface=fields`);
    await expect(page.getByRole("region", { name: "Field audit" })).toBeVisible();
    await page.getByRole("button", { name: "Labels", exact: true }).click();
    const design = page.getByRole("button", { name: "Design", exact: true });
    await settle(page);
    const labelMetrics = await rectangle(design);
    labelMetrics.ax = await design.ariaSnapshot();
    labelMetrics.coarsePointer = await page.evaluate(() => matchMedia("(pointer: coarse)").matches);
    await page.screenshot({ path: resolve(artifacts, `labels-${width}.png`) });
    await page.keyboard.press("Escape");
    await expect(page.getByPlaceholder("Find a label", { exact: true })).toBeHidden();
    await page.getByRole("button", { name: /^Delivery type/ }).click();
    const search = page.getByPlaceholder("Filter...", { exact: true });
    await search.fill("Choice 10");
    await settle(page);
    const searchMetrics = { query: "Choice 10", visibleOptions: await page.locator("[data-picker-item]").allTextContents() };
    await page.screenshot({ path: resolve(artifacts, `custom-search-${width}.png`) });
    await search.press("Enter");
    await expect(page.getByLabel("Selected custom values")).toContainText("select");
    await expect(search).toBeHidden();
    searchMetrics.committed = JSON.parse(await page.getByLabel("Selected custom values").textContent());
    await page.getByRole("button", { name: /^Acceptance criterion/ }).click();
    const editor = page.getByRole("textbox");
    await settle(page);
    const textMetrics = await rectangle(editor);
    textMetrics.ax = await editor.ariaSnapshot();
    textMetrics.explicitLabel = await editor.getAttribute("aria-label");
    textMetrics.labelledBy = await editor.getAttribute("aria-labelledby");
    textMetrics.fieldNamedInputCount = await page.getByRole("textbox", { name: "Acceptance criterion", exact: true }).count();
    result.scenarios[`fields-${width}`] = { labels: labelMetrics, search: searchMetrics, textEditor: textMetrics };
    await context.close();
  }
  for (const width of [1280, 320]) for (const theme of ["light", "dark"]) {
    const context = await browser.newContext({ viewport: { width, height: width === 320 ? 568 : 800 }, hasTouch: width === 320, isMobile: width === 320, colorScheme: theme });
    const page = await context.newPage();
    page.setDefaultTimeout(7000);
    page.on("pageerror", error => result.pageErrors.push(error.message));
    await page.route("**/*", async route => {
      if (new URL(route.request().url()).origin === origin) await route.continue();
      else { result.blockedRequests.push(route.request().url()); await route.abort(); }
    });
    await page.goto(`${origin}/?surface=memory`);
    await page.evaluate(theme => document.documentElement.classList.toggle("dark", theme === "dark"), theme);
    await page.getByRole("button", { name: "Version history", exact: true }).click();
    await page.getByRole("button", { name: "Purge v1", exact: true }).click();
    const dialog = page.getByRole("alertdialog");
    await expect(dialog).toBeVisible();
    const reason = "Reason for this version.\n".repeat(40);
    const editor = dialog.getByRole("textbox", { name: "Reason", exact: true });
    await editor.fill(reason);
    await settle(page);
    const measurements = { reasonBytes: Buffer.byteLength(reason, "utf8"), reasonValid: await page.evaluate(reason => window.auditFixture.reasonValid(reason), reason),
      dialog: await rectangle(dialog), textarea: await rectangle(editor), confirm: await rectangle(dialog.getByRole("button", { name: "Purge text", exact: true })),
      cancel: await rectangle(dialog.getByRole("button", { name: "Cancel", exact: true })) };
    await page.screenshot({ path: resolve(artifacts, `memory-purge-${width}-${theme}.png`) });
    result.scenarios[`memory-purge-${width}-${theme}`] = measurements;
    await context.close();
  }
} finally {
  if (browser) await browser.close();
  await new Promise(done => server.close(done));
  result.finishedAt = new Date().toISOString();
  await writeFile(resolve(artifacts, "results.json"), JSON.stringify(result, null, 2) + "\n");
}
console.log(JSON.stringify({ engine: result.engine, scenarios: result.scenarios, pageErrors: result.pageErrors, blockedRequests: result.blockedRequests }, null, 2));
