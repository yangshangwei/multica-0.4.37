import { _electron as electron } from "@playwright/test";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { createServer } from "node:http";

const profile = await fs.mkdtemp(path.join(os.tmpdir(), "multica-automation-fixes-"));
const root = process.cwd();
await fs.cp(path.join(os.homedir(), "Library/Application Support/Multica Canary work-management-local/Local Storage"), path.join(profile, "Local Storage"), { recursive: true });
const app = await electron.launch({
  executablePath: path.join(root, "apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron"),
  args: [path.join(import.meta.dirname, "harness.cjs"), "--remote-debugging-port=9477", "--no-proxy-server"],
  env: { ...process.env, AUTOMATION_AUDIT_PROFILE: profile, AUTOMATION_AUDIT_API_URL: "http://localhost:18572", AUTOMATION_AUDIT_RENDERER_URL: "http://localhost:5666", AUTOMATION_AUDIT_PRELOAD: path.join(root, "apps/desktop/out/preload/index.js") },
});
const page = await app.firstWindow();
page.setDefaultTimeout(12000);
const pageErrors = [];
page.on("pageerror", error => pageErrors.push(error.message));
await page.getByRole("link", { name: "自动化", exact: true }).and(page.locator('[data-sidebar="menu-button"]')).click();
await page.locator('[data-tab-scroll-root="autopilot-templates"]').waitFor();
const status = async () => ({
  ...(await app.evaluate(({ BrowserWindow }) => ({
    bounds: BrowserWindow.getAllWindows()[0].getBounds(),
    zoom: BrowserWindow.getAllWindows()[0].webContents.getZoomFactor(),
    counters: globalThis.automationAudit,
    electron: process.versions.electron, chromium: process.versions.chrome,
  }))),
  viewport: await page.evaluate(() => ({ width: innerWidth, height: innerHeight })),
  pageErrors,
});
let closing = false;
const close = async () => {
  if (closing) return;
  closing = true;
  server.close();
  await app.close().catch(() => {});
  await fs.rm(profile, { recursive: true, force: true });
  console.log("Audit window closed and temporary profile removed.");
  process.exit(0);
};
const server = createServer(async (request, response) => {
  try {
    const url = new URL(request.url, "http://127.0.0.1");
    if (url.pathname === "/viewport") {
      const width = Number(url.searchParams.get("width")), height = Number(url.searchParams.get("height")), zoom = Number(url.searchParams.get("zoom") || 1);
      if (width < 320 || width > 2600 || height < 480 || height > 1800 || ![1, 2].includes(zoom)) throw new Error("Unsupported audit viewport");
      await app.evaluate(({ BrowserWindow }, value) => {
        const window = BrowserWindow.getAllWindows()[0];
        window.setSize(value.width, value.height);
        window.webContents.setZoomFactor(value.zoom);
      }, { width, height, zoom });
    }
    const result = await status();
    response.setHeader("Content-Type", "application/json");
    response.end(JSON.stringify(result));
    if (url.pathname === "/close") await close();
  } catch (error) {
    response.statusCode = 500;
    response.end(JSON.stringify({ error: error.message }));
  }
}).listen(9478, "127.0.0.1");
process.on("SIGTERM", close);
process.on("SIGINT", close);
console.log(JSON.stringify({ ready: true, pid: process.pid, cdp: 9477, control: 9478, ...(await status()) }));
await new Promise(() => {});

