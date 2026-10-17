const { app, BrowserWindow, ipcMain, session } = require("electron");

const apiUrl = process.env.AUTOMATION_AUDIT_API_URL;
const rendererUrl = process.env.AUTOMATION_AUDIT_RENDERER_URL;
for (const value of [apiUrl, rendererUrl]) {
  if (!value || !["localhost", "127.0.0.1"].includes(new URL(value).hostname)) throw new Error("Audit requires local endpoints");
}
if (!process.env.AUTOMATION_AUDIT_PROFILE) throw new Error("Missing isolated profile");
app.setName("Multica Automation Audit");
app.setPath("userData", process.env.AUTOMATION_AUDIT_PROFILE);
globalThis.automationAudit = { daemonStarts: 0, installCalls: 0, externalLinks: 0, blockedWrites: 0 };
ipcMain.on("auth:session-state", () => {});
ipcMain.on("app:get-info", event => { event.returnValue = { version: "0.6.6-audit", os: "windows" }; });
ipcMain.on("runtime-config:get", event => {
  event.returnValue = { ok: true, source: "dev", config: { schemaVersion: 1, apiUrl, wsUrl: apiUrl.replace("http", "ws") + "/ws", appUrl: apiUrl } };
});
ipcMain.on("device-identity:get", event => { event.returnValue = null; });
ipcMain.on("freeze:get-last", event => { event.returnValue = null; });
ipcMain.handle("close-behavior:get", () => "ask");
ipcMain.handle("close-behavior:is-tray-supported", () => true);
ipcMain.handle("close-behavior:set", (_event, value) => ({ ok: true, value }));
ipcMain.handle("updater:check", () => ({ ok: true, available: false }));
ipcMain.handle("updater:get-preferences", () => ({ automaticUpdates: false }));
ipcMain.handle("updater:set-automatic-updates", (_event, enabled) => ({ automaticUpdates: enabled }));
ipcMain.handle("updater:install", () => { globalThis.automationAudit.installCalls++; });
ipcMain.handle("daemon:get-status", () => ({ state: "stopped", cliVersion: "audit" }));
ipcMain.handle("daemon:get-host-name", () => "Automation audit");
ipcMain.handle("daemon:probe-runtimes", () => []);
ipcMain.handle("daemon:is-cli-installed", () => true);
ipcMain.handle("daemon:get-prefs", () => ({ autoStart: false, autoStop: false }));
ipcMain.handle("daemon:set-prefs", (_event, prefs) => prefs);
for (const channel of ["daemon:set-target-api-url", "daemon:sync-token", "daemon:clear-token", "daemon:auto-start", "window:setImmersive"]) {
  ipcMain.handle(channel, () => ({ success: true }));
}
for (const channel of ["daemon:start", "daemon:restart", "daemon:retry-install"]) {
  ipcMain.handle(channel, () => { globalThis.automationAudit.daemonStarts++; throw new Error("Agent execution disabled in audit"); });
}
ipcMain.handle("shell:openExternal", () => { globalThis.automationAudit.externalLinks++; });
app.whenReady().then(async () => {
  session.defaultSession.webRequest.onBeforeRequest((details, callback) => {
    const url = new URL(details.url);
    const isRead = ["GET", "HEAD", "OPTIONS"].includes(details.method) ||
      (details.method === "POST" && ["/api/issues/table/rows", "/api/issues/table/facets", "/api/issues/table/groups"].includes(url.pathname));
    if (!isRead) { globalThis.automationAudit.blockedWrites++; callback({ cancel: true }); return; }
    callback({ cancel: ["http:", "https:", "ws:", "wss:"].includes(url.protocol) && !["localhost", "127.0.0.1"].includes(url.hostname) });
  });
  const window = new BrowserWindow({
    width: 1771, height: 1080, minWidth: 320, minHeight: 480, show: false,
    titleBarStyle: "hiddenInset", trafficLightPosition: { x: 16, y: 17 },
    webPreferences: {
      preload: process.env.AUTOMATION_AUDIT_PRELOAD, contextIsolation: true,
      sandbox: true, webSecurity: false, backgroundThrottling: false,
      additionalArguments: ["--multica-locale=zh-CN"],
    },
  });
  await window.loadURL(rendererUrl);
});
app.on("window-all-closed", () => app.quit());

