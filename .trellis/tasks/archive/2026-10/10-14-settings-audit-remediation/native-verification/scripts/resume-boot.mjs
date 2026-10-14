// Resume the settings audit in a separate, disposable Electron profile.
// The existing harness blocks network writes and agent execution.
import { _electron as electron } from '@playwright/test';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { createServer } from 'node:http';
import { installAcceptanceSafety } from './acceptance-safety.mjs';

const profile = await fs.mkdtemp(path.join(os.tmpdir(), 'multica-settings-resume-'));
const source = path.join(os.homedir(), 'Library/Application Support/Multica Canary work-management-local/Local Storage');
await fs.cp(source, path.join(profile, 'Local Storage'), { recursive: true });
const app = await electron.launch({
  executablePath: path.resolve('apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'),
  args: [path.join(import.meta.dirname, 'harness.cjs'), '--remote-debugging-port=9367', '--no-proxy-server'],
  env: {
    ...process.env,
    CHANGELOG_ELECTRON_PROFILE: profile,
    CHANGELOG_E2E_API_URL: 'http://localhost:18572',
    CHANGELOG_ELECTRON_RENDERER_URL: 'http://localhost:5666',
    CHANGELOG_ELECTRON_SYSTEM_LOCALE: 'zh-CN',
    AUDIT_DESKTOP_OS: 'windows',
  },
});
// Match the real preload contracts. The historical audit fixture used old
// updater property names; those fixture failures are not product findings.
await app.evaluate(({ ipcMain }) => {
  ipcMain.removeHandler('updater:get-preferences');
  ipcMain.handle('updater:get-preferences', () => ({ automaticUpdates: false }));
  ipcMain.removeHandler('updater:set-automatic-updates');
  ipcMain.handle('updater:set-automatic-updates', (_event, enabled) => ({ automaticUpdates: enabled }));
  ipcMain.removeHandler('updater:check');
  ipcMain.handle('updater:check', () => ({ ok: true, available: false }));
});
await app.context().addInitScript(installAcceptanceSafety);
const page = await app.firstWindow();
await page.evaluate(installAcceptanceSafety);
const pageErrors = [];
page.on('pageerror', error => pageErrors.push(error.message));
const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname === '/shutdown') {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify({ closing: true }));
      setTimeout(close, 20);
      return;
    }
    if (url.pathname === '/viewport') {
      const width = Number(url.searchParams.get('width'));
      const height = Number(url.searchParams.get('height'));
      const zoom = Number(url.searchParams.get('zoom'));
      if (width < 900 || height < 600 || ![1, 2].includes(zoom)) throw new Error('Unsupported audit viewport');
      await app.evaluate(({ BrowserWindow }, value) => {
        const window = BrowserWindow.getAllWindows()[0];
        window.setSize(value.width, value.height);
        window.webContents.setZoomFactor(value.zoom);
      }, { width, height, zoom });
    }
    if (url.pathname === '/ipc') {
      const channel = url.searchParams.get('channel');
      const mode = url.searchParams.get('mode');
      const allowed = ['daemon:get-prefs', 'daemon:is-cli-installed', 'daemon:get-status', 'close-behavior:get', 'close-behavior:is-tray-supported', 'close-behavior:set', 'updater:check', 'runtime-config:test'];
      if (!allowed.includes(channel) || !['reject', 'normal'].includes(mode)) throw new Error('Unsupported audit IPC fixture');
      await app.evaluate(({ ipcMain }, fixture) => {
        ipcMain.removeHandler(fixture.channel);
        ipcMain.handle(fixture.channel, () => {
          if (fixture.mode === 'reject') throw new Error('Settings audit injected IPC failure');
          const values = {
            'daemon:get-prefs': { autoStart: false, autoStop: false },
            'daemon:is-cli-installed': true,
            'daemon:get-status': { state: 'stopped', cliVersion: 'audit' },
            'close-behavior:get': 'ask',
            'close-behavior:is-tray-supported': true,
            'close-behavior:set': { ok: true },
            'updater:check': { ok: true, available: false },
            'runtime-config:test': { ok: true, latencyMs: 12 },
          };
          return values[fixture.channel];
        });
      }, { channel, mode });
    }
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify({
      ...(await app.evaluate(({ BrowserWindow }) => ({
        zoom: BrowserWindow.getAllWindows()[0].webContents.getZoomFactor(),
        bounds: BrowserWindow.getAllWindows()[0].getBounds(),
        acceptance: globalThis.changelogAcceptance,
      }))),
      pageErrors,
    }));
  } catch (error) {
    res.statusCode = 500;
    res.end(JSON.stringify({ error: error.message }));
  }
}).listen(9368, '127.0.0.1');
const startup = { ready: true, recordedAt: new Date().toISOString(), bootPid: process.pid, pid: app.process().pid, profile, electron: await app.evaluate(() => process.versions.electron), url: page.url(), cdpPort: 9367, controlPort: 9368 };
await fs.writeFile(path.join(import.meta.dirname, '../runtime-resumed.json'), JSON.stringify(startup, null, 2));
console.log(JSON.stringify(startup));
let closing = false;
const close = async () => {
  if (closing) return;
  closing = true;
  server.close();
  await app.close().catch(() => {});
  await fs.rm(profile, { recursive: true, force: true });
  console.log('Resumed audit closed; temporary profile removed.');
  process.exit(0);
};
process.on('SIGTERM', close);
process.on('SIGINT', close);
await new Promise(() => {});
