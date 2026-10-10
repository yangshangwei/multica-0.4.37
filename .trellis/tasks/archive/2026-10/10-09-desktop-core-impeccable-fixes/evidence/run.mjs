import { chromium, expect } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
const dir = path.dirname(import.meta.filename);
const browser = await chromium.connectOverCDP('http://127.0.0.1:9251');
const context = browser.contexts()[0];
const page = context.pages().find((candidate) => candidate.url().startsWith('http://localhost:5666'));
if (!page) throw new Error('Isolated verification renderer unavailable');
page.setDefaultTimeout(8000);
const cdp = await context.newCDPSession(page);
const shot = async (name) => page.screenshot({ path: path.join(dir, 'screenshots', name + '.png') });
const snap = async () => console.log((await page.locator('body').ariaSnapshot()).slice(0,9000));
const record = async (name, value) => fs.writeFile(path.join(dir, name + '.json'), JSON.stringify(value, null, 2) + '\n');
let script = '';
for await (const chunk of process.stdin) script += chunk;
try {
  await new (Object.getPrototypeOf(async () => {}).constructor)('page', 'context', 'cdp', 'fs', 'path', 'dir', 'expect', 'shot', 'snap', 'record', script)(page, context, cdp, fs, path, dir, expect, shot, snap, record);
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
} finally {
  await cdp.detach();
  await browser.close();
}
