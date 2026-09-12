const { app, BrowserWindow, ipcMain, session } = require("electron");
const path = require("node:path");

// Run the real desktop preload, renderer, router and shell. Native services
// unrelated to reading releases are isolated so this smoke test never starts
// a daemon, executes an installed agent CLI, or changes a user's preferences.
const apiUrl = process.env.CHANGELOG_E2E_API_URL;
const rendererUrl = process.env.CHANGELOG_ELECTRON_RENDERER_URL;
for (const value of [apiUrl, rendererUrl]) {
  if (!value || !["localhost", "127.0.0.1"].includes(new URL(value).hostname)) {
    throw new Error("Electron changelog acceptance requires local endpoints");
  }
}
if (!process.env.CHANGELOG_ELECTRON_PROFILE) throw new Error("Missing isolated Electron profile");
app.setName("Multica Changelog Acceptance");
app.setPath("userData", process.env.CHANGELOG_ELECTRON_PROFILE);

globalThis.changelogAcceptance = { daemonStarts: 0, externalLinks: [], installCalls: 0 };
ipcMain.on("app:get-info", (event) => { event.returnValue = { version: "0.4.40-test", os: "macos" }; });
ipcMain.on("runtime-config:get", (event) => {
  event.returnValue = { ok: true, source: "dev", config: {
    schemaVersion: 1, apiUrl, wsUrl: `${apiUrl.replace("http", "ws")}/ws`, appUrl: apiUrl,
  } };
});
ipcMain.on("device-identity:get", (event) => { event.returnValue = null; });
ipcMain.on("freeze:get-last", (event) => { event.returnValue = null; });

ipcMain.handle("daemon:get-status", () => ({ state: "stopped", cliVersion: "test" }));
ipcMain.handle("daemon:get-host-name", () => "Changelog acceptance");
ipcMain.handle("daemon:probe-runtimes", () => []);
ipcMain.handle("daemon:is-cli-installed", () => true);
ipcMain.handle("daemon:get-prefs", () => ({ autoStart: false, autoStop: false }));
for (const channel of ["daemon:set-target-api-url", "daemon:sync-token", "daemon:clear-token", "daemon:auto-start", "window:setImmersive"]) {
  ipcMain.handle(channel, () => ({ success: true }));
}
for (const channel of ["daemon:start", "daemon:restart", "daemon:retry-install"]) {
  ipcMain.handle(channel, () => {
    globalThis.changelogAcceptance.daemonStarts += 1;
    throw new Error("Agent execution is outside this acceptance fixture");
  });
}
ipcMain.handle("updater:get-preferences", () => ({ automaticUpdatesEnabled: false }));
ipcMain.handle("updater:install", () => { globalThis.changelogAcceptance.installCalls += 1; });
ipcMain.handle("shell:openExternal", (_event, url) => { globalThis.changelogAcceptance.externalLinks.push(url); });

app.whenReady().then(async () => {
  session.defaultSession.webRequest.onBeforeRequest((details, callback) => {
    const url = new URL(details.url);
    callback({ cancel: ["http:", "https:", "ws:", "wss:"].includes(url.protocol) && !["localhost", "127.0.0.1"].includes(url.hostname) });
  });
  const window = new BrowserWindow({
    width: 1380, height: 1000, show: true, title: "Multica changelog acceptance",
    webPreferences: {
      preload: path.resolve(__dirname, "../../apps/desktop/out/preload/index.js"),
      contextIsolation: true, sandbox: true,
      additionalArguments: ["--multica-locale=zh-CN"],
    },
  });
  await window.loadURL(rendererUrl);
});
app.on("window-all-closed", () => app.quit());
