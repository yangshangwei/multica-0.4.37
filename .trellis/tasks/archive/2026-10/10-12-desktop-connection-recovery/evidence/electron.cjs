const { app, BrowserWindow, ipcMain, session } = require("electron");
const path = require("node:path");

// Real renderer/preload, Chromium storage and production server-switch helpers.
// Unrelated native services are replaced so no installed daemon or agent runs.
const profile = process.env.RECOVERY_ACCEPTANCE_PROFILE;
const rendererUrl = process.env.RECOVERY_ACCEPTANCE_RENDERER;
const configPath = process.env.RECOVERY_ACCEPTANCE_CONFIG;
if (!profile || !configPath || !rendererUrl || new URL(rendererUrl).hostname !== "127.0.0.1") {
  throw new Error("An isolated profile/config and loopback renderer are required");
}
const {
  ServerSwitchCoordinator, probeServer, clearServerCookies,
  loadRuntimeConfig, saveRuntimeConfig, parseRuntimeConfig,
  mergeRuntimeConfigInput, MainRendererMessageQueue,
  parseMainRendererChannelState, encodeIssueWindowArgument, openRuntimeConfigEditorFromWindow,
} = require(process.env.RECOVERY_ACCEPTANCE_HELPERS);
app.setName("Multica Connection Recovery Acceptance");
app.setPath("userData", profile);
const coordinator = new ServerSwitchCoordinator();
const messages = new MainRendererMessageQueue();
let runtimeConfig;
let main;
let generation = 0;
let replacementExpected = false;
const issues = new Set();
const evidence = { daemonStarts: 0, nativeTests: 0, nativeSaves: 0, resets: 0, handoffs: 0, authSessions: [] };
const localUrl = (value) => {
  const config = parseRuntimeConfig(JSON.stringify({ schemaVersion: 1, apiUrl: value }));
  if (new URL(config.apiUrl).hostname !== "127.0.0.1") throw new Error("Only loopback fixture servers are permitted");
  return config;
};
const sendMain = (channel, payload) => main.webContents.send(channel, payload);
ipcMain.on("app:get-info", (event) => { event.returnValue = { version: "0.4.40-recovery-test", os: "macos" }; });
ipcMain.on("runtime-config:get", (event) => {
  if (replacementExpected && main && event.sender === main.webContents) {
    replacementExpected = false;
    coordinator.rendererReady();
  }
  event.returnValue = runtimeConfig;
});
ipcMain.on("device-identity:get", (event) => {
  event.returnValue = { deviceId: "recovery-fixture-device", deviceName: "Isolated acceptance" };
});
ipcMain.on("freeze:get-last", (event) => { event.returnValue = null; });
ipcMain.on("auth:session-state", (_event, userId) => { evidence.authSessions.push(userId); });
ipcMain.on("main-renderer:channel-state", (event, input) => {
  if (!main || event.sender !== main.webContents) return;
  const state = parseMainRendererChannelState(input);
  if (state) messages.setReady(state.channel, state.ready, sendMain);
});
ipcMain.handle("runtime-config:open-editor", (event) => {
  const sender = BrowserWindow.fromWebContents(event.sender);
  if (coordinator.blocked) return { ok: false };
  return openRuntimeConfigEditorFromWindow({
    sourceWindow: sender, mainWindow: main, issueWindows: issues,
    openMainEditor: () => {
      evidence.handoffs += 1;
      main.show();
      main.focus();
      messages.enqueue("runtime-config:edit", null, sendMain);
    },
  });
});
ipcMain.handle("runtime-config:test", async (_event, value) => {
  evidence.nativeTests += 1;
  const started = Date.now();
  try {
    const config = localUrl(value);
    await probeServer(config.apiUrl);
    return { ok: true, latencyMs: Date.now() - started };
  } catch (error) { return { ok: false, category: "network", message: error.message }; }
});
ipcMain.handle("runtime-config:save", async (event, input) => {
  evidence.nativeSaves += 1;
  if (!main || event.sender !== main.webContents) return { ok: false, message: "Main window only" };
  try {
    const config = parseRuntimeConfig(JSON.stringify(mergeRuntimeConfigInput(runtimeConfig.config, input)));
    localUrl(config.apiUrl);
    await coordinator.run({
      probe: () => probeServer(config.apiUrl),
      freeze: () => { for (const issue of issues) if (!issue.isDestroyed()) issue.destroy(); },
      cleanup: async () => {
        const id = ++generation;
        await new Promise((resolve, reject) => {
          const finish = (error) => {
            clearTimeout(timer);
            ipcMain.removeListener("runtime-config:reset-result", listener);
            if (error) reject(error); else resolve();
          };
          const listener = (ack, result) => {
            if (ack.sender === main.webContents && result?.id === id) finish(result.ok ? undefined : new Error("Reset failed"));
          };
          const timer = setTimeout(() => finish(new Error("Reset timed out")), 15_000);
          ipcMain.on("runtime-config:reset-result", listener);
          main.webContents.send("runtime-config:reset", id);
        });
        evidence.resets += 1;
        await clearServerCookies(main.webContents.session.cookies, [runtimeConfig.config.apiUrl, config.apiUrl]);
      },
      save: async () => { runtimeConfig = { ok: true, source: "configured", config: await saveRuntimeConfig(config, configPath) }; },
      reload: () => {
        replacementExpected = true;
        messages.resetReady();
        main.webContents.reload();
      },
    });
    return { ok: true, config };
  } catch (error) { return { ok: false, message: error.message }; }
});
ipcMain.on("window:close", (event) => BrowserWindow.fromWebContents(event.sender)?.close());
ipcMain.handle("daemon:get-status", () => ({ state: "stopped", cliVersion: "fixture" }));
ipcMain.handle("daemon:get-host-name", () => "Isolated recovery acceptance");
ipcMain.handle("daemon:probe-runtimes", () => []);
ipcMain.handle("daemon:is-cli-installed", () => true);
ipcMain.handle("daemon:get-prefs", () => ({ autoStart: false, autoStop: false }));
ipcMain.handle("daemon:get-installation-metadata", () => null);
ipcMain.handle("daemon:refresh-managed-workspaces", () => ({ managed: false, accepted: false }));
for (const channel of ["daemon:set-target-api-url", "daemon:sync-token", "daemon:clear-token", "daemon:auto-start", "daemon:stop", "daemon:end-installation-session", "window:setImmersive"]) {
  ipcMain.handle(channel, () => ({ success: true }));
}
for (const channel of ["daemon:start", "daemon:restart", "daemon:retry-install"]) {
  ipcMain.handle(channel, () => { evidence.daemonStarts += 1; throw new Error("Agent execution is outside this fixture"); });
}
ipcMain.handle("updater:get-preferences", () => ({ automaticUpdatesEnabled: false }));
ipcMain.handle("shell:openExternal", () => { throw new Error("External navigation is outside this fixture"); });

globalThis.recoveryAcceptanceReady = app.whenReady().then(async () => {
  runtimeConfig = await loadRuntimeConfig({ isDev: false, env: {}, configPath });
  if (!runtimeConfig.ok) throw new Error("Fixture config failed validation");
  localUrl(runtimeConfig.config.apiUrl);
  session.defaultSession.webRequest.onBeforeRequest((details, done) => {
    const url = new URL(details.url);
    done({ cancel: ["http:", "https:", "ws:", "wss:"].includes(url.protocol) && url.hostname !== "127.0.0.1" });
  });
  const windowOptions = {
    width: 1000, height: 780, show: true,
    webPreferences: {
      preload: process.env.RECOVERY_ACCEPTANCE_PRELOAD,
      contextIsolation: true, sandbox: true,
      additionalArguments: [`--multica-locale=${process.env.RECOVERY_ACCEPTANCE_LOCALE || "en"}`],
    },
  };
  main = new BrowserWindow(windowOptions);
  globalThis.recoveryAcceptance = {
    evidence,
    load: () => main.loadURL(rendererUrl),
    async openIssue() {
      const issue = new BrowserWindow({ ...windowOptions, webPreferences: {
        ...windowOptions.webPreferences,
        additionalArguments: [...windowOptions.webPreferences.additionalArguments, encodeIssueWindowArgument({ path: "/fixture/issues/FIX-1", title: "Recovery fixture issue" })],
      } });
      issues.add(issue);
      issue.on("closed", () => issues.delete(issue));
      await issue.loadURL(rendererUrl);
      return issue.id;
    },
    resize: (width, height) => main.setContentSize(width, height),
    snapshot: () => ({ ...evidence, apiUrl: runtimeConfig.config.apiUrl, windows: BrowserWindow.getAllWindows().length, blocked: coordinator.blocked }),
  };
});
app.on("window-all-closed", () => app.quit());
