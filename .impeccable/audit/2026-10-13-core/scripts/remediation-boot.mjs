import { _electron as electron } from '@playwright/test';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { createServer } from 'node:http';

const dir = import.meta.dirname;
const root = process.cwd();
const profile = await fs.mkdtemp(path.join(os.tmpdir(), 'multica-remediation-20261013-'));
const source = path.join(os.homedir(), 'Library/Application Support/Multica Canary work-management-local/Local Storage');
await fs.cp(source, path.join(profile, 'Local Storage'), { recursive: true });
const app = await electron.launch({
  executablePath: path.join(root, 'apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'),
  args: [path.join(dir, 'harness.cjs'), '--remote-debugging-port=9257'],
  env: {
    ...process.env,
    CHANGELOG_ELECTRON_PROFILE: profile,
    CHANGELOG_E2E_API_URL: 'http://localhost:18572',
    CHANGELOG_ELECTRON_RENDERER_URL: 'http://localhost:5666',
    CHANGELOG_ELECTRON_SYSTEM_LOCALE: 'zh-CN',
  },
});
const page = await app.firstWindow();
page.on('pageerror', error => console.log('RENDER_ERROR', error.message));


console.log(JSON.stringify({ ready: true, electron: await app.evaluate(() => process.versions.electron), url: page.url() }));
const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname === '/zoom') {
      const factor = Number(url.searchParams.get('factor'));
      if ([1, 2].includes(factor)) await app.evaluate(({ BrowserWindow }, value) => BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(value), factor);
    }
    if (url.pathname === '/resize') {
      const width = Number(url.searchParams.get('width'));
      const height = Number(url.searchParams.get('height'));
      if (width >= 900 && height >= 600) await app.evaluate(({ BrowserWindow }, size) => BrowserWindow.getAllWindows()[0].setSize(...size), [width, height]);
    }
    res.end(JSON.stringify(await app.evaluate(({ BrowserWindow }) => ({ zoom: BrowserWindow.getAllWindows()[0].webContents.getZoomFactor(), bounds: BrowserWindow.getAllWindows()[0].getBounds() }))));
  } catch (error) { res.statusCode = 500; res.end(error.message); }
}).listen(9258, '127.0.0.1');
const close = async () => {
  server.close();
  await app.close();
  await fs.rm(profile, { recursive: true, force: true });
  console.log('Audit Electron closed; temporary profile removed.');
  process.exit(0);
};
process.on('SIGTERM', close);
process.on('SIGINT', close);
await new Promise(() => {});
