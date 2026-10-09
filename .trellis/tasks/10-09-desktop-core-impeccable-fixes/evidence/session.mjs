import { _electron as electron, chromium } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import { createServer } from 'node:http';

const root = process.cwd();
const dir = path.dirname(import.meta.filename);
const resumedProfile = process.env.DESKTOP_CORE_VERIFY_PROFILE;
if (resumedProfile && (!path.basename(resumedProfile).startsWith('multica-desktop-core-verify-') ||
  !path.resolve(resumedProfile).startsWith(path.resolve(os.tmpdir()) + path.sep))) {
  throw new Error('Only task-owned temporary verification profiles may be resumed');
}
let storage = { cookies: [], origins: [] };
if (!resumedProfile) {
  const source = await chromium.connectOverCDP('http://127.0.0.1:9247');
  const sourceContext = source.contexts()[0];
  storage = await sourceContext.storageState();
  const sourcePage = sourceContext.pages().find((candidate) => candidate.url().startsWith('http://localhost:5666'));
  // Electron CDP contexts can omit visited origins from storageState.
  storage.origins = [await sourcePage.evaluate(() => ({
    origin: location.origin,
    localStorage: Object.entries(localStorage).map(([name, value]) => ({ name, value })),
  }))];
  await source.close();
}
const profile = resumedProfile ?? await fs.mkdtemp(path.join(os.tmpdir(), 'multica-desktop-core-verify-'));
const app = await electron.launch({
  executablePath: path.join(root, 'apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'),
  args: [path.join(root, '.impeccable/audit/2026-10-09-desktop-core/scripts/harness.cjs'), '--remote-debugging-port=9251'],
  env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile,
    CHANGELOG_E2E_API_URL: 'http://localhost:18572',
    CHANGELOG_ELECTRON_RENDERER_URL: 'http://localhost:5666',
    CHANGELOG_ELECTRON_SYSTEM_LOCALE: 'zh-CN' },
});
await app.context().addCookies(storage.cookies);
await app.context().addInitScript((origins) => {
  const entries = origins.find((entry) => entry.origin === location.origin)?.localStorage ?? [];
  for (const { name, value } of entries) {
    if (localStorage.getItem(name) === null) localStorage.setItem(name, value);
  }
}, storage.origins);
const page = await app.firstWindow();
await page.reload();
await page.locator('[data-chat-launcher], #floating-chat-window').first().waitFor({ state: 'attached' });
await fs.writeFile(path.join(dir, 'session.json'), JSON.stringify({
  pid: process.pid, electronPid: app.process().pid, profile, cdp: 'http://127.0.0.1:9251',
  api: 'http://localhost:18572', renderer: 'http://localhost:5666',
  createdAt: new Date().toISOString(), isolated: true,
}, null, 2) + '\n');
console.log(JSON.stringify({ ready: true, isolated: true, electron: await app.evaluate(() => process.versions.electron) }));
const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname === '/zoom') {
      const factor = Number(url.searchParams.get('factor'));
      if (![1, 2].includes(factor)) throw new Error('Unsupported verification zoom');
      await app.evaluate(({ BrowserWindow }, zoom) => BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(zoom), factor);
    } else if (url.pathname === '/resize') {
      const width = Number(url.searchParams.get('width'));
      const height = Number(url.searchParams.get('height'));
      if (![900, 1440].includes(width) || ![700, 1000].includes(height)) throw new Error('Unsupported verification dimensions');
      await app.evaluate(({ BrowserWindow }, size) => BrowserWindow.getAllWindows()[0].setContentSize(...size), [width, height]);
    } else if (url.pathname === '/capture') {
      const name = url.searchParams.get('name');
      if (!/^[a-z0-9-]+$/.test(name)) throw new Error('Invalid screenshot name');
      const png = await app.evaluate(async ({ BrowserWindow }) => (await BrowserWindow.getAllWindows()[0].webContents.capturePage()).toPNG().toString('base64'));
      await fs.writeFile(path.join(dir, 'screenshots', name + '.png'), Buffer.from(png, 'base64'));
    } else if (url.pathname === '/close') {
      res.end(JSON.stringify({ closed: true }));
      await close();
      return;
    }
    res.end(JSON.stringify(await app.evaluate(({ BrowserWindow }) => ({ zoom: BrowserWindow.getAllWindows()[0].webContents.getZoomFactor() }))));
  } catch (error) {
    res.statusCode = 500;
    res.end(JSON.stringify({ error: error.message }));
  }
}).listen(9252, '127.0.0.1');
await fs.mkdir(path.join(dir, 'screenshots'), { recursive: true });
async function close() {
  server.close();
  await app.close();
  await fs.rm(profile, { recursive: true, force: true });
  process.exit(0);
}
process.on('SIGTERM', close);
process.on('SIGINT', close);
await new Promise(() => {});
