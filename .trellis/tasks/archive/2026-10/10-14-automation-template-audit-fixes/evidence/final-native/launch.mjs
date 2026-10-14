import { _electron as electron } from "@playwright/test";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { createServer } from "node:http";
import { createHash } from "node:crypto";

const evidence = import.meta.dirname;
const root = process.cwd();
const provenance = JSON.parse(await fs.readFile(path.join(evidence, "build-provenance.json"), "utf8"));
if (provenance.buildExitCode !== 0) throw new Error("A successful fresh build is required");
const build = path.join(provenance.runtime, "build");
const rendererRoot = path.join(build, "renderer");
const apiUrl = provenance.apiUrl;
const rendererUrl = provenance.rendererUrl;
const bootstrapUrl = "http://localhost:5666";
const profile = await fs.mkdtemp(path.join(os.tmpdir(), "multica-automation-final-profile-"));
const mime = {
  ".html": "text/html", ".js": "text/javascript", ".css": "text/css",
  ".json": "application/json", ".svg": "image/svg+xml", ".png": "image/png",
  ".woff": "font/woff", ".woff2": "font/woff2", ".ico": "image/x-icon",
};
const servedArtifacts = new Map();
const assetServer = createServer(async (request, response) => {
  try {
    const pathname = decodeURIComponent(new URL(request.url, rendererUrl).pathname);
    let filename = path.resolve(rendererRoot, "." + pathname);
    if (filename !== rendererRoot && !filename.startsWith(rendererRoot + path.sep)) {
      response.writeHead(403).end();
      return;
    }
    const stat = await fs.stat(filename).catch(() => null);
    if (!stat?.isFile()) filename = path.join(rendererRoot, "index.html");
    const body = await fs.readFile(filename);
    servedArtifacts.set(path.relative(build, filename), createHash("sha256").update(body).digest("hex"));
    response.writeHead(200, { "Content-Type": mime[path.extname(filename)] ?? "application/octet-stream", "Cache-Control": "no-store" });
    response.end(body);
  } catch (error) {
    response.writeHead(500).end(error.message);
  }
});
await new Promise((resolve, reject) => {
  assetServer.once("error", reject);
  assetServer.listen(Number(new URL(rendererUrl).port), "localhost", resolve);
});

let desktop;
let controlServer;
let closing = false;
const pageErrors = [];
const failedRequests = [];
const responseErrors = [];
const status = async () => ({
  ...(await desktop.evaluate(({ BrowserWindow }) => {
    const window = BrowserWindow.getAllWindows()[0];
    return {
      bounds: window.getBounds(), zoom: window.webContents.getZoomFactor(),
      counters: globalThis.automationAudit,
      electron: process.versions.electron, chromium: process.versions.chrome,
      webPreferences: { sandbox: window.webContents.getLastWebPreferences().sandbox, contextIsolation: window.webContents.getLastWebPreferences().contextIsolation, webSecurity: window.webContents.getLastWebPreferences().webSecurity },
    };
  })),
  viewport: await desktop.windows()[0].evaluate(() => ({ width: innerWidth, height: innerHeight })),
  pageErrors, failedRequests, responseErrors,
  servedArtifacts: Object.fromEntries(servedArtifacts),
});
const close = async () => {
  if (closing) return;
  closing = true;
  if (desktop) await fs.writeFile(path.join(evidence, "runtime-final.json"), JSON.stringify(await status(), null, 2) + "\n");
  controlServer?.close();
  await desktop?.close().catch(() => {});
  assetServer.close();
  await fs.rm(profile, { recursive: true, force: true });
  await fs.writeFile(path.join(evidence, "cleanup.json"), JSON.stringify({ isolatedProfileRemoved: true, nativeWindowClosed: true, staticServerClosed: true }) + "\n");
  console.log("Acceptance window closed; temporary profile removed.");
  process.exit(0);
};
process.on("SIGTERM", close);
process.on("SIGINT", close);

try {
  await fs.cp(path.join(os.homedir(), "Library/Application Support/Multica Canary work-management-local/Local Storage"), path.join(profile, "Local Storage"), { recursive: true });
  desktop = await electron.launch({
    executablePath: path.join(root, "apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron"),
    args: [path.join(evidence, "electron-main.cjs"), "--remote-debugging-port=9477", "--no-proxy-server"],
    env: {
      ...process.env,
      AUTOMATION_AUDIT_PROFILE: profile, AUTOMATION_AUDIT_API_URL: apiUrl,
      AUTOMATION_AUDIT_RENDERER_URL: rendererUrl, AUTOMATION_AUDIT_BOOTSTRAP_URL: bootstrapUrl,
      AUTOMATION_AUDIT_PRELOAD: path.join(build, "preload/index.js"),
    },
  });
  const page = await desktop.firstWindow();
  page.setDefaultTimeout(15000);
  // Read the existing local test session from its copied profile in a blank
  // same-origin document. Never print or persist the credential in evidence.
  const bootstrapDocument = bootstrapUrl + "/__automation_acceptance_bootstrap";
  await page.route(bootstrapDocument, (route) => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Native acceptance bootstrap</title>" }));
  await page.goto(bootstrapDocument);
  const token = await page.evaluate(() => localStorage.getItem("multica_token"));
  if (!token) throw new Error("Copied local test profile has no session credential");
  await page.context().addInitScript(({ token, origin }) => {
    if (location.origin !== origin) return;
    localStorage.setItem("multica_token", token);
    localStorage.setItem("multica-locale", "zh-Hans");
    localStorage.setItem("multica:chat:isOpen", "false");
  }, { token, origin: rendererUrl });
  await page.unroute(bootstrapDocument);
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("requestfailed", (request) => failedRequests.push({ method: request.method(), path: new URL(request.url()).pathname, error: request.failure()?.errorText }));
  page.on("response", (response) => {
    if (response.status() >= 400) responseErrors.push({ status: response.status(), path: new URL(response.url()).pathname });
  });
  await page.goto(rendererUrl);
  await page.getByRole("link", { name: "自动化", exact: true }).and(page.locator('[data-sidebar="menu-button"]')).click();
  await page.locator('[data-tab-scroll-root="autopilot-templates"]').waitFor();
  const initial = { ready: true, sourceCommit: provenance.sourceCommit, apiHealth: await (await fetch(apiUrl + "/health")).json(), ...(await status()) };
  await fs.writeFile(path.join(evidence, "runtime-start.json"), JSON.stringify(initial, null, 2) + "\n");
  controlServer = createServer(async (request, response) => {
    try {
      const url = new URL(request.url, "http://127.0.0.1");
      if (url.pathname === "/viewport") {
        const width = Number(url.searchParams.get("width"));
        const height = Number(url.searchParams.get("height"));
        const zoom = Number(url.searchParams.get("zoom") || 1);
        if (width < 320 || width > 2600 || height < 480 || height > 1800 || ![1, 2].includes(zoom)) throw new Error("Unsupported viewport");
        await desktop.evaluate(({ BrowserWindow }, value) => {
          const window = BrowserWindow.getAllWindows()[0];
          window.setSize(value.width, value.height);
          window.webContents.setZoomFactor(value.zoom);
        }, { width, height, zoom });
      }
      response.setHeader("Content-Type", "application/json");
      response.end(JSON.stringify(await status()));
      if (url.pathname === "/close") await close();
    } catch (error) {
      response.writeHead(500).end(JSON.stringify({ error: error.message }));
    }
  });
  await new Promise((resolve, reject) => {
    controlServer.once("error", reject);
    controlServer.listen(9478, "127.0.0.1", resolve);
  });
  console.log(JSON.stringify({ ready: true, sourceCommit: provenance.sourceCommit, cdp: 9477, control: 9478, electron: initial.electron, chromium: initial.chromium, viewport: initial.viewport, errors: initial.pageErrors }));
  await new Promise(() => {});
} catch (error) {
  console.error(error.stack);
  await desktop?.close().catch(() => {});
  assetServer.close();
  await fs.rm(profile, { recursive: true, force: true });
  process.exit(1);
}
