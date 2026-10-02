/* global window */
import { createRequire } from 'node:module';
import { createServer } from 'node:http';
import { createReadStream } from 'node:fs';
import { mkdir, readFile, writeFile, readdir, stat, lstat, rm } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { parseArgs } from 'node:util';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { collectArtifacts } from './update-artifacts.mjs';
const execute = promisify(execFile);
const sleep = (milliseconds) => new Promise((done) => setTimeout(done, milliseconds));

// Serve only the already-validated artifact manifest, never arbitrary paths.
export async function createUpdateFeed(directory, requests = []) {
  const files = new Map();
  for (const name of await readdir(directory)) {
    const path = join(directory, name);
    const info = await stat(path);
    if (info.isFile()) files.set(`/desktop/${name}`, { path, size: info.size });
  }
  const server = createServer((request, response) => {
    const path = new URL(request.url, 'http://127.0.0.1').pathname;
    requests.push({ method: request.method, path, range: request.headers.range ?? null });
    const file = files.get(path);
    if (!file || !['GET', 'HEAD'].includes(request.method)) { response.writeHead(404).end(); return; }
    response.setHeader('Cache-Control', path.endsWith('.yml') ? 'no-cache' : 'public, max-age=31536000, immutable');
    response.setHeader('Accept-Ranges', 'bytes');
    let start = 0; let end = file.size - 1;
    if (request.headers.range) {
      const match = /^bytes=(\d+)-(\d*)$/.exec(request.headers.range);
      if (!match) { response.writeHead(416).end(); return; }
      start = Number(match[1]); end = match[2] ? Number(match[2]) : end;
      if (start > end || end >= file.size) { response.writeHead(416).end(); return; }
      response.statusCode = 206;
      response.setHeader('Content-Range', `bytes ${start}-${end}/${file.size}`);
    }
    response.setHeader('Content-Length', end - start + 1);
    if (request.method === 'HEAD') { response.end(); return; }
    createReadStream(file.path, { start, end }).on('error', () => response.destroy()).pipe(response);
  });
  await new Promise((done, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', done); });
  return { server, url: `http://127.0.0.1:${server.address().port}/desktop` };
}

export async function claimHomeDirectory(home, owner) {
  const directory = join(home, '.multica');
  const marker = join(directory, 'windows-update-owner.json');
  await mkdir(directory);
  await writeFile(marker, owner, { flag: 'wx' });
  return async () => {
    if ((await lstat(directory)).isSymbolicLink() || await readFile(marker, 'utf8') !== owner) throw new Error('Updater home ownership mismatch');
    await rm(directory, { recursive: true });
  };
}

async function stopOwned(executable) {
  // Match the hosted workflow's PowerShell 7 module environment; PS5 cannot load its CIM modules.
  await execute('pwsh.exe', ['-NoProfile', '-NonInteractive', '-Command', '$p=$env:MULTICA_UPDATE_OWNED_INSTALL; Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($p + "\\", [StringComparison]::OrdinalIgnoreCase) } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }'], { env: { ...process.env, MULTICA_UPDATE_OWNED_INSTALL: resolve(executable, '..') }, timeout: 30_000 });
}

async function run(options) {
  if (process.platform !== 'win32' || process.arch !== 'x64' || process.env.GITHUB_ACTIONS !== 'true' || process.env.RUNNER_ENVIRONMENT !== 'github-hosted') throw new Error('Disposable GitHub-hosted Windows x64 required');
  const root = resolve(options['state-directory']);
  const executable = resolve(options.executable);
  if (executable !== join(root, 'installed', 'Multica.exe')) throw new Error('Executable must be the isolated owned installation');
  const result = { status: 'failed', previousVersion: options['previous-version'], expectedVersion: options['expected-version'], requests: [], guiLaunched: false, downloaded: false, upgraded: false, settingsRetained: false, historyMarkerRetained: false, signedAcceptance: false, offlineTrustTested: false, error: null };
  let electronApp; let feed; let cleanupHome;
  try {
    const artifacts = join(root, 'feed');
    await collectArtifacts({ source: resolve(options['candidate-directory']), destination: artifacts, allowPrerelease: true });
    feed = await createUpdateFeed(artifacts, result.requests);
    const profile = join(root, 'profile');
    if (!process.env.USERPROFILE) throw new Error('Runner home missing');
    const home = resolve(process.env.USERPROFILE);
    cleanupHome = await claimHomeDirectory(home, root);
    const appData = join(root, 'AppData', 'Roaming'); const localAppData = join(root, 'AppData', 'Local');
    for (const path of [profile, join(home, '.multica'), appData, localAppData]) await mkdir(path, { recursive: true });
    const config = { schemaVersion: 1, apiUrl: feed.url.replace('/desktop', ''), appUrl: feed.url.replace('/desktop', ''), wsUrl: feed.url.replace('http:', 'ws:').replace('/desktop', '/ws'), updateUrl: feed.url };
    await writeFile(join(home, '.multica', 'desktop.json'), JSON.stringify(config));
    await writeFile(join(home, '.multica', 'desktop_prefs.json'), JSON.stringify({ autoStart: false, autoStop: true }));
    await writeFile(join(profile, 'updater-preferences.json'), JSON.stringify({ automaticUpdates: false }));
    const env = { ...process.env, HOME: home, USERPROFILE: home, APPDATA: appData, LOCALAPPDATA: localAppData };
    delete env.ELECTRON_RUN_AS_NODE;
    // Playwright belongs to repository acceptance tooling, not the shipped desktop package.
    const { _electron } = createRequire(new URL('../../../package.json', import.meta.url))('@playwright/test');
    const launch = () => _electron.launch({ executablePath: executable, args: [`--user-data-dir=${profile}`], env, timeout: 60_000 });
    electronApp = await launch();
    result.processErrors = [];
    electronApp.process().stderr?.on('data', (chunk) => { if (result.processErrors.length < 200) result.processErrors.push(chunk.toString().slice(0, 4096)); });
    const paths = await electronApp.evaluate(({ app }) => ({ version: app.getVersion(), home: app.getPath('home'), userData: app.getPath('userData') }));
    result.initial = paths;
    if (paths.version !== options['previous-version']) throw new Error(`Baseline version mismatch: ${paths.version}`);
    if (resolve(paths.home).toLowerCase() !== home.toLowerCase() || resolve(paths.userData).toLowerCase() !== profile.toLowerCase()) throw new Error('Installed Electron did not honor isolated home/profile; refusing updater execution');
    const page = await electronApp.firstWindow();
    await page.waitForFunction(() => Boolean(window.updater), null, { timeout: 60_000 });
    result.guiLaunched = true;
    // Browser session restrictions exercise the installed updater while blocking telemetry/business egress.
    await electronApp.evaluate(({ session }) => {
      session.defaultSession.webRequest.onBeforeRequest((details, callback) => {
        const url = new URL(details.url);
        callback({ cancel: ['http:', 'https:', 'ws:', 'wss:'].includes(url.protocol) && url.hostname !== '127.0.0.1' });
      });
    });
    const marker = `updater-retention-${Date.now()}`;
    await page.evaluate(async (value) => {
      localStorage.setItem('multica-update-acceptance-history', value);
      window.__updateAcceptance = { downloaded: null, progress: [] };
      window.updater.onUpdateDownloaded((info) => { window.__updateAcceptance.downloaded = info; });
      window.updater.onDownloadProgress((info) => { window.__updateAcceptance.progress.push(info.percent); });
      await window.updater.setAutomaticUpdates(false);
    }, marker);
    result.check = await page.evaluate(() => window.updater.checkForUpdates());
    if (!result.check.ok || !result.check.available || result.check.latestVersion !== options['expected-version']) throw new Error(`Production update check failed: ${JSON.stringify(result.check)}`);
    await page.waitForFunction(() => Boolean(window.__updateAcceptance.downloaded), null, { timeout: 240_000 });
    result.download = await page.evaluate(() => window.__updateAcceptance);
    if (result.download.downloaded.version !== options['expected-version']) throw new Error('Downloaded version mismatch');
    result.downloaded = true;
    const oldAsar = await stat(join(root, 'installed', 'resources', 'app.asar'));
    await page.evaluate(() => { void window.updater.installUpdate(); });
    const deadline = Date.now() + 240_000;
    let replaced = false;
    while (Date.now() < deadline) {
      await sleep(1000);
      const current = await stat(join(root, 'installed', 'resources', 'app.asar')).catch(() => null);
      if (current && (current.mtimeMs !== oldAsar.mtimeMs || current.size !== oldAsar.size)) { replaced = true; break; }
    }
    if (!replaced) throw new Error('quitAndInstall did not replace installed payload before timeout');
    // NSIS relaunches without Playwright debug flags. Stop only this installation and relaunch it for proof.
    await sleep(5000);
    await stopOwned(executable);
    electronApp = await launch();
    const actual = await electronApp.evaluate(({ app }) => app.getVersion());
    result.installedVersion = actual;
    if (actual !== options['expected-version']) throw new Error(`Upgraded application version mismatch: ${actual}`);
    result.upgraded = true;
    const upgradedPage = await electronApp.firstWindow();
    await upgradedPage.waitForFunction(() => Boolean(window.updater), null, { timeout: 60_000 });
    const preferences = await upgradedPage.evaluate(() => window.updater.getPreferences());
    result.settingsRetained = preferences.automaticUpdates === false && JSON.parse(await readFile(join(home, '.multica', 'desktop.json'), 'utf8')).updateUrl === feed.url;
    result.historyMarkerRetained = await upgradedPage.evaluate((value) => localStorage.getItem('multica-update-acceptance-history') === value, marker);
    if (!result.settingsRetained || !result.historyMarkerRetained) throw new Error('Settings or browser history marker changed across update');
    result.status = 'passed';
  } catch (error) { result.error = error.stack ?? String(error); }
  finally {
    await electronApp?.close().catch(() => {});
    if (cleanupHome) {
      try { await stopOwned(executable); await cleanupHome(); result.ownedHomeCleanup = 'passed'; }
      catch (error) { result.status = 'failed'; result.ownedHomeCleanup = error.message; }
    }
    if (feed) { feed.server.closeAllConnections(); await new Promise((done) => feed.server.close(done)); }
    await writeFile(options.report, JSON.stringify(result, null, 2));
  }
  if (result.status !== 'passed') throw new Error(result.error);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const { values } = parseArgs({ options: Object.fromEntries(['executable', 'previous-version', 'candidate-directory', 'expected-version', 'state-directory', 'report'].map((name) => [name, { type: 'string' }])) });
  if (Object.values(values).length !== 6) throw new Error('All six probe options are required');
  await run(values);
}
