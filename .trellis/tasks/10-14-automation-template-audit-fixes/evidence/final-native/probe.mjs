import { chromium, expect } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";
const browser = await chromium.connectOverCDP("http://127.0.0.1:9477");
const context = browser.contexts()[0];
const page = context.pages().find(page => page.url().startsWith("http://localhost:13492"));
if (!page) throw new Error("Owned audit renderer not found");
page.setDefaultTimeout(7000);
const cdp = await context.newCDPSession(page);
const dir = import.meta.dirname;
const control = async (route, params = {}) => {
  const url = new URL(route, "http://127.0.0.1:9478");
  for (const [key, value] of Object.entries(params)) url.searchParams.set(key, String(value));
  const response = await fetch(url);
  if (!response.ok) throw new Error(await response.text());
  const result = await response.json();
  if (route === "/viewport") await page.waitForTimeout(350);
  return result;
};
const shot = async name => {
  const capture = await cdp.send("Page.captureScreenshot", { format: "png", captureBeyondViewport: false, fromSurface: true });
  const file = path.join(dir, name + ".png");
  await fs.writeFile(file, Buffer.from(capture.data, "base64"));
  return file;
};
const locale = async value => {
  const localeDir = path.join(process.cwd(), "packages/views/locales", value);
  const resources = Object.fromEntries(await Promise.all((await fs.readdir(localeDir)).filter(file => file.endsWith(".json")).map(async file => [file.slice(0, -5), JSON.parse(await fs.readFile(path.join(localeDir, file), "utf8"))])));
  await page.evaluate(async ({ value, resources }) => {
    const root = document.getElementById("root");
    const key = Object.keys(root).find(key => key.startsWith("__reactContainer"));
    const stack = [root[key].stateNode.current];
    let i18n, authStore;
    while (stack.length) {
      const fiber = stack.pop();
      if (fiber.memoizedProps?.i18n?.changeLanguage) i18n = fiber.memoizedProps.i18n;
      if (fiber.memoizedProps?.authStore?.getState) authStore = fiber.memoizedProps.authStore;
      if (i18n && authStore) break;
      if (fiber.child) stack.push(fiber.child);
      if (fiber.sibling) stack.push(fiber.sibling);
    }
    if (!i18n || !authStore) throw new Error("Live locale/auth fixtures not found");
    const state = authStore.getState();
    // This is the copied audit profile only; no server preference write occurs.
    authStore.setState({ user: { ...state.user, language: null } });
    for (const [namespace, bundle] of Object.entries(resources)) i18n.addResourceBundle(value, namespace, bundle, true, true);
    localStorage.setItem("multica-locale", value);
    await i18n.changeLanguage(value);
  }, { value, resources });
  await page.waitForTimeout(350);
};
const theme = async value => {
  await page.evaluate(value => { document.documentElement.classList.remove("light", "dark"); document.documentElement.classList.add(value); }, value);
  await page.waitForTimeout(350);
};
const root = () => page.locator('[data-tab-scroll-root="autopilot-templates"]');
const navigate = async (action, target) => {
  return page.evaluate(({ action, target }) => {
    const anchor = document.querySelector('[data-tab-scroll-root="autopilot-templates"]') ?? document.querySelector("h1");
    let fiber = anchor[Object.keys(anchor).find(key => key.startsWith("__reactFiber"))];
    while (fiber) {
      const value = fiber.memoizedProps?.value;
      if (typeof value?.pathname === "string" && typeof value?.push === "function" && typeof value?.back === "function") {
        if (action === "current") return value.pathname;
        value[action](target);
        return;
      }
      fiber = fiber.return;
    }
    throw new Error("Active navigation adapter not found");
  }, { action, target });
};
let script = "";
for await (const chunk of process.stdin) script += chunk;
try {
  await new (Object.getPrototypeOf(async () => {}).constructor)(
    "page", "context", "cdp", "fs", "path", "dir", "expect", "control", "shot", "locale", "theme", "root", "navigate",
    script,
  )(page, context, cdp, fs, path, dir, expect, control, shot, locale, theme, root, navigate);
} catch (error) {
  console.error(error.stack);
  process.exitCode = 1;
}
await cdp.detach();
process.exit(process.exitCode ?? 0);
