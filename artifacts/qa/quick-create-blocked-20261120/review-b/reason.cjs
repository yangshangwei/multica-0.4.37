const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const fixture = JSON.parse(fs.readFileSync(process.env.REVIEW_AUTH_FILE, 'utf8'));
(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    await context.addCookies([{ name: 'multica-locale', value: 'en', url: 'http://localhost:60677' }, { name: 'multica_logged_in', value: '1', url: 'http://localhost:60677' }]);
    await context.addInitScript(({ token }) => {
      localStorage.setItem('multica_token', token); localStorage.setItem('multica-locale', 'en'); localStorage.setItem('theme', 'light');
      localStorage.setItem('multica:chat:isOpen', 'false'); localStorage.setItem('multica_create_mode', JSON.stringify({ state: { lastMode: 'agent' }, version: 0 }));
    }, { token: fixture.token });
    const page = await context.newPage();
    await page.goto(fixture.url, { waitUntil: 'domcontentloaded' });
    await page.getByRole('button', { name: 'New Issue', exact: true }).click();
    const dialog = page.getByRole('dialog'); const submit = dialog.getByRole('button', { name: 'Send to agent', exact: true });
    await dialog.getByRole('switch').focus(); await page.keyboard.press('Tab'); await page.waitForTimeout(400);
    const read = () => page.evaluate(() => ({ active: { tag: document.activeElement.tagName, text: document.activeElement.textContent, ariaDescription: document.activeElement.getAttribute('aria-describedby') }, tooltip: [...document.querySelectorAll('[data-slot="tooltip-content"],[role="tooltip"]')].map(el => ({ text: el.textContent, role: el.getAttribute('role'), width: el.getBoundingClientRect().width, height: el.getBoundingClientRect().height, hidden: getComputedStyle(el).visibility })) }));
    const keyboard = await read();
    await page.screenshot({ path: path.join(__dirname, 'blocked-reason-keyboard.png'), animations: 'disabled' });
    await dialog.locator('.ProseMirror[contenteditable="true"]').click();
    const box = await submit.boundingBox(); await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.locator('[data-slot="tooltip-content"]').waitFor({ state: 'visible', timeout: 3000 });
    const hover = await read();
    await page.screenshot({ path: path.join(__dirname, 'blocked-reason-hover.png'), animations: 'disabled' });
    const evidence = { keyboard, hover, note: 'Real Tab and hover paths checked against data-slot=tooltip-content; the matrix role=tooltip probe is inapplicable because this Tooltip content has no explicit role.' };
    fs.writeFileSync(path.join(__dirname, 'reason-evidence.json'), JSON.stringify(evidence, null, 2));
    console.log(JSON.stringify(evidence));
  } finally { await browser.close(); }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
