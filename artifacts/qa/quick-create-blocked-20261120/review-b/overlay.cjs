const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const fixture = JSON.parse(fs.readFileSync(process.env.REVIEW_AUTH_FILE, 'utf8'));
const launcher = '/Users/artisan/.agents/skills/impeccable/scripts/impeccable';
const out = __dirname;

(async () => {
  const browser = await chromium.launch({ headless: true });
  let liveServerStarted = false;
  const evidence = { nativeFallback: 'Native CUA was concretely unavailable to sibling assessment; fresh isolated headless Chromium used, never attached to human Chrome/Desktop.', userVisible: false, pages: [], cleanup: null };
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, colorScheme: 'light' });
    await context.addCookies([{ name: 'multica-locale', value: 'en', url: 'http://localhost:60677' }, { name: 'multica_logged_in', value: '1', url: 'http://localhost:60677' }]);
    await context.addInitScript(({ token }) => {
      localStorage.setItem('multica_token', token); localStorage.setItem('multica-locale', 'en');
      localStorage.setItem('multica:chat:isOpen', 'false'); localStorage.setItem('theme', 'light');
      localStorage.setItem('multica_create_mode', JSON.stringify({ state: { lastMode: 'agent' }, version: 0 }));
    }, { token: fixture.token });
    const page = await context.newPage(); const logs = [];
    page.on('console', message => { if (/impeccable/i.test(message.text())) logs.push({ type: message.type(), text: message.text() }); });
    await page.goto(fixture.url, { waitUntil: 'domcontentloaded' });
    await page.getByRole('button', { name: 'New Issue', exact: true }).click();
    evidence.preflight = await page.evaluate(() => {
      const oldTitle = document.title; document.title = '[Human] isolated Assessment B overlay';
      const script = document.createElement('script'); script.textContent = 'window.__assessmentBMutablePreflight = true'; document.head.append(script);
      const result = { titleMutation: document.title === '[Human] isolated Assessment B overlay', scriptExecution: window.__assessmentBMutablePreflight === true, previousTitle: oldTitle };
      script.remove(); return result;
    });
    if (!evidence.preflight.titleMutation || !evidence.preflight.scriptExecution) throw new Error('Mutable script preflight failed');
    const start = spawnSync(launcher, ['live-server', '--background'], { cwd: out, encoding: 'utf8' });
    const metadata = JSON.parse(start.stdout);
    const port = metadata.port || metadata.httpPort || (metadata.url && new URL(metadata.url).port);
    if (start.status !== 0 || !port) throw new Error('Live server did not return a usable port');
    liveServerStarted = true;
    evidence.server = { port, pid: metadata.pid, stop: 'impeccable live-server stop --keep-inject (cwd review-b)' };
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.addScriptTag({ url: `http://localhost:${port}/detect.js` });
    await page.waitForTimeout(2600);
    evidence.pages.push({ state: 'empty-1280-en-light', injection: true, logs: [...logs] });
    await page.screenshot({ path: path.join(out, 'detector-overlay.png'), animations: 'disabled' });
    evidence.overlayElements = await page.evaluate(() => [...document.querySelectorAll('[id],[class]')].filter(el => /impeccable/.test(`${el.id} ${el.className}`)).map(el => ({ tag: el.tagName, id: el.id, class: typeof el.className === 'string' ? el.className : '' })).slice(0, 30));
    await context.close();
  } catch (error) { evidence.error = error.message; process.exitCode = 1; }
  finally {
    if (liveServerStarted) {
      const stop = spawnSync(launcher, ['live-server', 'stop', '--keep-inject'], { cwd: out, encoding: 'utf8' });
      evidence.cleanup = { exitCode: stop.status, stdout: stop.stdout, stderr: stop.stderr };
    }
    await browser.close();
    fs.writeFileSync(path.join(out, 'overlay-evidence.json'), JSON.stringify(evidence, null, 2));
    console.log(JSON.stringify(evidence));
  }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
