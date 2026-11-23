import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import process from "node:process";
import console from "node:console";
import { URL } from "node:url";

process.on("uncaughtException", (error) => {
  console.error(error);
  process.exit(1);
});

// Use the real native routing/IPC functions with test-owned services and disk.
// Compiling these functions avoids running index.ts's daemon/install lifecycle.
const repo = path.resolve(import.meta.dirname, "../..");
const desktop = path.join(repo, "apps/desktop");
const requireDesktop = createRequire(path.join(desktop, "package.json"));
const requireVite = createRequire(createRequire(requireDesktop.resolve("electron-vite")).resolve("vite"));
const { buildSync } = requireVite("esbuild");
const ts = requireDesktop("typescript");
const profile = process.env.COLLABORATION_ELECTRON_PROFILE;
const apiUrl = process.env.COLLABORATION_E2E_API_URL;
const rendererUrl = process.env.COLLABORATION_ELECTRON_RENDERER_URL;
const wsUrl = process.env.COLLABORATION_ELECTRON_WS_URL || `${apiUrl?.replace("http", "ws")}/ws`;
assert(profile, "An isolated Electron profile is required");
for (const value of [apiUrl, rendererUrl, wsUrl]) {
  assert(value && ["127.0.0.1", "localhost"].includes(new URL(value).hostname), "Only loopback test endpoints are allowed");
}

const source = ts.createSourceFile("index.ts", readFileSync(path.join(desktop, "src/main/index.ts"), "utf8"), ts.ScriptTarget.Latest, true);
const functionNames = [
  "sendMainRendererMessage", "focusMainWindow", "ensureMainWindow",
  "dispatchToMainRenderer", "handleDeepLink", "loadRenderer", "installWindowShortcutHandler",
];
const functions = functionNames.map((name) => {
  const node = source.statements.find((statement) => ts.isFunctionDeclaration(statement) && statement.name?.text === name);
  assert(node, `Production function ${name} is required`);
  return node.getText(source);
});
const channels = new Set([
  '"window:open-account"', '"window:open-issue"', '"window:close"',
  '"file:download-url"',
  "MAIN_RENDERER_CHANNEL_STATE_CHANNEL", "AUTH_SESSION_STATE_CHANNEL",
]);
const registrations = [];
function visit(node) {
  if (ts.isExpressionStatement(node) && ts.isCallExpression(node.expression)) {
    const call = node.expression;
    if (["ipcMain.on", "ipcMain.handle"].includes(call.expression.getText(source))
      && channels.has(call.arguments[0]?.getText(source))) registrations.push(node.getText(source));
  }
  ts.forEachChild(node, visit);
}
visit(source);
assert.equal(registrations.length, channels.size, "Every native account/issue channel must use its production registration");

const imports = [
  "./keyboard-shortcuts", "./navigation-guard", "./renderer-file-access",
  "./external-url",
  "./renderer-web-preferences", "./auth-session-coordinator",
  "../shared/auth-session", "../shared/main-renderer-messages", "../shared/account-navigation", "../shared/issue-window",
];
const nativeImports = source.statements.filter((node) => ts.isImportDeclaration(node) && imports.includes(node.moduleSpecifier.text))
  .map((node) => node.getText(source).replace(node.moduleSpecifier.getText(source), JSON.stringify(path.resolve(desktop, "src/main", node.moduleSpecifier.text))));
const compiled = path.join(profile, "desktop-account-main.cjs");
buildSync({
  stdin: {
    resolveDir: desktop,
    loader: "ts",
    contents: `
      import { app, BrowserWindow, ipcMain, session } from 'electron';
      import { join } from 'node:path';
      import { pathToFileURL } from 'node:url';
      ${nativeImports.join("\n")}
      const PROTOCOL = 'multica';
      const is = { dev: true };
      const serverSwitch = { blocked: false };
      const browserLoginAttempt = { consume: () => false };
      let mainWindow = null;
      let desktopInitialized = false;
      let authSessionGeneration = 0;
      const issueWindows = new Set();
      const mainRendererMessages = new MainRendererMessageQueue();
      const authSessionCoordinator = new AuthSessionCoordinator((window) => {
        if (!window.isDestroyed()) window.destroy();
      });
      const apiUrl = ${JSON.stringify(apiUrl)};
      process.env.ELECTRON_RENDERER_URL = ${JSON.stringify(rendererUrl)};
      app.setName('Multica Collaboration Acceptance');
      app.setPath('userData', ${JSON.stringify(profile)});
      const evidence = { daemonStarts: 0, nativeIssueRequests: [], authSessions: [], externalLinks: [] };
      globalThis.desktopAccountAcceptance = {
        evidence,
        start: () => loadRenderer(mainWindow),
        deepLink: (url) => handleDeepLink(url),
        deliverLogin: (token) => dispatchToMainRenderer('auth:token', token),
        setBlocked: (blocked) => { serverSwitch.blocked = blocked; },
      };
      ${functions.join("\n")}
      ${registrations.join("\n")}

      function newWindow(context) {
        const args = context ? [encodeIssueWindowArgument(context)] : [];
        const window = new BrowserWindow({
          width: context ? 960 : 1380, height: context ? 760 : 960,
          show: true, titleBarStyle: 'hiddenInset', trafficLightPosition: { x: 16, y: 17 },
          webPreferences: createRendererWebPreferences(${JSON.stringify(path.join(desktop, "out/preload/index.js"))}, 'en', args),
        });
        installWindowShortcutHandler(window);
        window.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
        return window;
      }
      function createWindow(defer = false) {
        mainRendererMessages.resetReady();
        mainWindow = newWindow(null);
        mainWindow.on('closed', () => { mainWindow = null; });
        if (defer) void mainWindow.loadURL('about:blank');
        else loadRenderer(mainWindow);
        return mainWindow;
      }
      function createIssueWindow(context) {
        if (serverSwitch.blocked) return;
        evidence.nativeIssueRequests.push(context);
        const window = newWindow(context);
        issueWindows.add(window);
        authSessionCoordinator.registerIssueWindow(window);
        window.on('closed', () => {
          issueWindows.delete(window);
          authSessionCoordinator.unregisterIssueWindow(window);
        });
        loadRenderer(window);
      }
      ipcMain.on('app:get-info', (event) => { event.returnValue = { version: '0.4.40-test', os: 'macos' }; });
      ipcMain.on('runtime-config:get', (event) => { event.returnValue = { ok: true, source: 'dev', config: {
        schemaVersion: 1, apiUrl, wsUrl: ${JSON.stringify(wsUrl)}, appUrl: apiUrl,
      } }; });
      ipcMain.on('device-identity:get', (event) => { event.returnValue = null; });
      ipcMain.on('freeze:get-last', (event) => { event.returnValue = null; });
      ipcMain.on('auth:session-state', (_event, userId) => { evidence.authSessions.push(userId); });
      ipcMain.handle('daemon:get-status', () => ({ state: 'stopped', cliVersion: 'test' }));
      ipcMain.handle('daemon:get-host-name', () => 'Isolated collaboration acceptance');
      ipcMain.handle('daemon:probe-runtimes', () => []);
      ipcMain.handle('daemon:is-cli-installed', () => true);
      ipcMain.handle('daemon:get-prefs', () => ({ autoStart: false, autoStop: false }));
      for (const channel of ['daemon:set-target-api-url', 'daemon:sync-token', 'daemon:clear-token', 'daemon:auto-start', 'daemon:stop', 'window:setImmersive']) {
        ipcMain.handle(channel, () => ({ success: true }));
      }
      for (const channel of ['daemon:start', 'daemon:restart', 'daemon:retry-install']) {
        ipcMain.handle(channel, () => { evidence.daemonStarts += 1; throw new Error('Native agent execution is outside this fixture'); });
      }
      ipcMain.handle('updater:get-preferences', () => ({ automaticUpdatesEnabled: false }));
      ipcMain.handle('shell:openExternal', (_event, url) => { evidence.externalLinks.push(url); });
      app.whenReady().then(() => {
        session.defaultSession.webRequest.onBeforeSendHeaders((details, callback) => {
          const url = new URL(details.url);
          callback({ cancel: ['http:', 'https:', 'ws:', 'wss:'].includes(url.protocol) && !['127.0.0.1', 'localhost'].includes(url.hostname) });
        });
        if (process.env.COLLABORATION_ELECTRON_DEEP_LINK) handleDeepLink(process.env.COLLABORATION_ELECTRON_DEEP_LINK);
        desktopInitialized = true;
        createWindow(true);
      });
      app.on('window-all-closed', () => app.quit());
    `,
  },
  bundle: true,
  platform: "node",
  format: "cjs",
  external: ["electron"],
  outfile: compiled,
  logLevel: "error",
});
createRequire(import.meta.url)(compiled);
