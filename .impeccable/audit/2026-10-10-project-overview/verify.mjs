import { chromium, expect } from '@playwright/test';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const dir = import.meta.dirname;
const { url } = JSON.parse(await readFile(path.join(dir, 'runtime.json'), 'utf8'));
const out = path.join(dir, 'round-1');
await mkdir(out, { recursive: true });
const browser = await chromium.launch({ headless: true });
const results = { fixture: 'actual ProjectOverviewPanel with synthetic in-memory API', cases: [], errors: [] };

async function openCase(name, params, viewport) {
  const context = await browser.newContext({ viewport, colorScheme: params.theme === 'dark' ? 'dark' : 'light', reducedMotion: 'reduce' });
  const page = await context.newPage();
  page.on('pageerror', error => results.errors.push({ name, error: error.message }));
  page.on('console', message => { if (message.type() === 'error') results.errors.push({ name, error: message.text() }); });
  await page.goto(`${url}/?${new URLSearchParams(params)}`, { waitUntil: 'networkidle' });
  await page.evaluate(() => document.fonts.ready);
  await expect(page.getByRole('heading', { name: params.lang === 'en' ? 'System health' : '系统健康' })).toBeVisible();
  return { context, page };
}

async function inspect(page) {
  return page.evaluate(() => {
    const visible = element => { const rect = element.getBoundingClientRect(); return rect.width > 2 && rect.height > 2 && getComputedStyle(element).visibility !== 'hidden'; };
    const overflow = [...document.querySelectorAll('main h2, main h3, main li, main button, main summary, main article')]
      .filter(visible).map(element => ({ tag: element.tagName, text: element.textContent.trim().slice(0, 100), rect: element.getBoundingClientRect().toJSON() }))
      .filter(({ rect }) => rect.left < -1 || rect.right > innerWidth + 1);
    return { viewport: { width: innerWidth, height: innerHeight }, bodyWidth: document.body.scrollWidth, overflow };
  });
}

async function capture(name, page) {
  const metrics = await inspect(page);
  expect(metrics.bodyWidth).toBeLessThanOrEqual(metrics.viewport.width);
  expect(metrics.overflow).toEqual([]);
  await page.screenshot({ path: path.join(out, `${name}.png`), fullPage: true });
  results.cases.push({ name, ...metrics });
}

try {
  for (const [name, params, viewport] of [
    ['zh-light-wide', { lang: 'zh-Hans', theme: 'light' }, { width: 1280, height: 900 }],
    ['en-dark-wide', { lang: 'en', theme: 'dark' }, { width: 1280, height: 900 }],
    ['zh-light-narrow-long', { lang: 'zh-Hans', theme: 'light', long: '1' }, { width: 390, height: 844 }],
    ['en-dark-narrow-long', { lang: 'en', theme: 'dark', long: '1' }, { width: 390, height: 844 }],
  ]) {
    const { context, page } = await openCase(name, params, viewport);
    const health = page.getByRole('heading', { name: params.lang === 'en' ? 'System health' : '系统健康' }).locator('xpath=ancestor::section');
    await expect(health.getByRole('button', { name: params.lang === 'en' ? /^Blocked \d+$/ : /^阻塞 \d+$/ })).toBeEnabled();
    await expect(health.getByText(params.lang === 'en' ? 'View assessment details' : '查看判定依据')).toBeVisible();
    await health.getByText(params.lang === 'en' ? 'View assessment details' : '查看判定依据').click();
    await expect(health.locator('details[open]')).toBeVisible();
    await page.getByRole('article').first().getByText(params.lang === 'en' ? 'View details' : '查看详情').click();
    await expect(page.getByRole('article').first().locator('details[open]')).toBeVisible();
    await capture(name, page);
    await context.close();
  }

  for (const [state, expected] of [['clear', 'No current risk signals'], ['empty', 'No formal issues yet'], ['unavailable', 'Statistics are incomplete']]) {
    const { context, page } = await openCase(`state-${state}`, { lang: 'en', theme: 'light', state }, { width: 900, height: 800 });
    const health = page.getByRole('heading', { name: 'System health' }).locator('xpath=ancestor::section');
    await expect(health.getByText(expected)).toBeVisible();
    await context.close();
  }

  {
    const { context, page } = await openCase('state-stale', { lang: 'en', theme: 'light', state: 'stale' }, { width: 900, height: 800 });
    const health = page.getByRole('heading', { name: 'System health' }).locator('xpath=ancestor::section');
    await expect(health.getByText('No current risk signals')).toBeVisible();
    await page.getByRole('button', { name: 'Refresh' }).click();
    await expect(page.getByText('Refresh failed. These are the last available statistics.')).toBeVisible();
    await expect(health.getByText('No current risk signals')).toHaveCount(0);
    await expect(health.getByRole('button', { name: /^Blocked \d+$/ })).toBeDisabled();
    await context.close();
  }

  expect(results.errors).toEqual([]);
  console.log(JSON.stringify(results, null, 2));
} catch (error) {
  results.failure = error.stack;
  console.error(error.stack);
  process.exitCode = 1;
} finally {
  await writeFile(path.join(out, 'metrics.json'), JSON.stringify(results, null, 2));
  await browser.close();
}
