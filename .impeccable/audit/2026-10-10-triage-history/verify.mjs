import { chromium, expect } from '@playwright/test';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import path from 'node:path';

const dir = import.meta.dirname;
const root = path.resolve(dir, '../../..');
const { url } = JSON.parse(await readFile(path.join(dir, 'runtime.json'), 'utf8'));
const out = path.join(dir, process.argv[2] || 'round-1');
await mkdir(out, { recursive: true });
const labels = Object.fromEntries(await Promise.all(['en', 'zh-Hans'].map(async language => [language, JSON.parse(await readFile(path.join(root, `packages/views/locales/${language}/triage.json`), 'utf8'))])));
const browser = await chromium.launch({ headless: true });
const results = { fixture: 'actual shared TriagePage, real styles/providers; synthetic in-memory API data', timestamp: new Date().toISOString(), timezone: 'Asia/Shanghai', screenshots: [], interactions: {}, errors: [] };

async function measure(page) {
  return page.evaluate(() => {
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    const rgba = value => { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data]; };
    const luminance = rgb => rgb.slice(0, 3).map(c => { const s = c / 255; return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4; }).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
    const visible = element => { const rect = element.getBoundingClientRect(); return rect.width > 2 && rect.height > 2 && getComputedStyle(element).visibility !== 'hidden'; };
    const contrasts = [...document.querySelectorAll('main .text-muted-foreground')].filter(element => visible(element) && element.textContent.trim() && !element.closest('[disabled]')).slice(0, 30).map(element => {
      const foreground = rgba(getComputedStyle(element).color);
      let ancestor = element, background;
      while (ancestor) { const color = rgba(getComputedStyle(ancestor).backgroundColor); if (color[3] === 255) { background = color; break; } ancestor = ancestor.parentElement; }
      background ??= [255, 255, 255, 255];
      const a = luminance(foreground), b = luminance(background);
      return { text: element.textContent.trim().slice(0, 80), ratio: Number(((Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)).toFixed(2)) };
    });
    const overflow = [...document.querySelectorAll('main article, main h2, main li, main input, main button')].filter(visible).map(element => ({ element: element.tagName, text: element.textContent.trim().slice(0, 60), ...(() => { const rect = element.getBoundingClientRect(); return { left: rect.left, right: rect.right }; })() })).filter(rect => rect.left < -1 || rect.right > innerWidth + 1);
    return { viewport: { width: innerWidth, height: innerHeight }, bodyWidth: document.body.scrollWidth, dates: [...document.querySelectorAll('main h2')].map(element => element.textContent), records: document.querySelectorAll('main article').length, overflow, contrasts };
  });
}

async function openCase(name, language, theme, width, extra = '') {
  const context = await browser.newContext({ viewport: { width, height: width < 600 ? 844 : 887 }, timezoneId: 'Asia/Shanghai', colorScheme: theme, reducedMotion: 'reduce' });
  await context.route('**/*', route => {
    const target = new URL(route.request().url());
    if (['http:', 'https:'].includes(target.protocol) && target.origin !== url) return route.abort();
    return route.continue();
  });
  const page = await context.newPage();
  page.on('pageerror', error => results.errors.push({ name, error: error.message }));
  page.on('console', message => { if (message.type() === 'error') results.errors.push({ name, error: message.text() }); });
  await page.goto(`${url}/?lang=${language}&theme=${theme}&view=history${extra}`, { waitUntil: 'networkidle' });
  await page.evaluate(() => document.fonts.ready);
  await expect(page.getByRole('heading', { level: 1, name: labels[language].title })).toBeVisible();
  return { context, page };
}

async function capture(page, name) {
  const metrics = await measure(page);
  expect(metrics.bodyWidth).toBeLessThanOrEqual(metrics.viewport.width);
  expect(metrics.overflow).toEqual([]);
  await page.screenshot({ path: path.join(out, `${name}.png`) });
  results.screenshots.push({ name, ...metrics });
}

try {
  for (const [name, language, theme, width, extra] of [
    ['zh-light-wide', 'zh-Hans', 'light', 1340, ''],
    ['en-dark-wide', 'en', 'dark', 1340, ''],
    ['zh-light-narrow-long', 'zh-Hans', 'light', 390, '&long=1'],
    ['en-dark-narrow-long', 'en', 'dark', 390, '&long=1'],
  ]) {
    const { context, page } = await openCase(name, language, theme, width, extra);
    await expect(page.getByRole('article')).toHaveCount(5);
    await expect(page.getByRole('heading', { level: 2 })).toHaveCount(2);
    const times = await page.locator('article time').evaluateAll(elements => elements.map(element => ({ datetime: element.getAttribute('datetime'), full: element.querySelector('.sr-only')?.textContent })));
    expect(times.map(time => time.datetime)).toEqual(['2026-10-08T15:09:59Z', '2026-10-08T11:38:25Z', '2026-10-08T11:38:25Z', '2026-10-08T11:28:37Z', '2026-10-07T08:10:00Z']);
    expect(times.every(time => time.full?.includes('2026'))).toBe(true);
    await capture(page, name);
    if (width < 600) {
      await page.getByRole('article').last().scrollIntoViewIfNeeded();
      await expect(page.getByRole('article').last()).toBeInViewport();
      await expect(page.getByRole('button', { name: labels[language].next, exact: true })).toBeInViewport();
      await capture(page, `${name}-scrolled`);
      results.interactions.narrowScrolling = 'Long records remain in a bounded scroll area; the final event and fixed pagination are reachable';
    }
    if (name === 'zh-light-wide') {
      let reachedSummary = false;
      for (let index = 0; index < 30; index += 1) {
        await page.keyboard.press('Tab');
        reachedSummary = await page.evaluate(() => document.activeElement?.tagName === 'SUMMARY');
        if (reachedSummary) break;
      }
      expect(reachedSummary).toBe(true);
      await page.keyboard.press('Enter');
      await expect(page.locator('details[open]').first()).toBeVisible();
      await expect(page.locator('details[open]').first()).toContainText(labels['zh-Hans'].before);
      await expect(page.locator('details[open]').first()).toContainText(labels['zh-Hans'].after);
      await capture(page, 'zh-disclosure-keyboard');
      results.interactions.nativeDisclosure = 'Tab reaches native summary; Enter opens snapshot before/after values';
    }
    await context.close();
  }

  const zh = labels['zh-Hans'];
  {
    const { context, page } = await openCase('filters', 'zh-Hans', 'light', 1340);
    await page.getByRole('textbox', { name: zh.search }).fill('DQA');
    await page.getByRole('button', { name: zh.filters, exact: true }).click();
    await page.getByRole('combobox', { name: zh.result, exact: true }).click();
    await page.getByRole('option', { name: zh.reject, exact: true }).click();
    await page.getByRole('combobox', { name: zh.processed_by, exact: true }).click();
    await page.getByRole('option', { name: '杨尚伟', exact: true }).click();
    await page.getByLabel(zh.processed_after, { exact: true }).fill('2026-10-08');
    await page.getByLabel(zh.processed_before, { exact: true }).fill('2026-10-08');
    await page.getByRole('button', { name: zh.filters, exact: true }).click();
    const summary = page.getByRole('list', { name: zh.history_filters.label });
    await expect(summary).toBeVisible();
    for (const value of ['DQA', zh.reject, '杨尚伟', '2026-10-08']) await expect(summary).toContainText(value);
    await expect(page.getByRole('article')).toHaveCount(2);
    await capture(page, 'zh-applied-filters-wide');
    await page.setViewportSize({ width: 390, height: 844 });
    await capture(page, 'zh-applied-filters-narrow');
    await page.getByRole('button', { name: zh.clear_filters, exact: true }).click();
    await expect(page.getByRole('list', { name: zh.history_filters.label })).toHaveCount(0);
    await expect(page.getByRole('article')).toHaveCount(5);
    expect(new URL(page.url()).searchParams.get('view')).toBe('history');
    results.interactions.filters = 'Real filter controls update the summary; collapsed editor retains readable conditions; Clear resets list and preserves history';
    await context.close();
  }
  {
    const { context, page } = await openCase('no-match', 'zh-Hans', 'light', 900, '&q=not-a-record&offset=50&preserve=keep');
    await expect(page.getByRole('heading', { name: zh.history_no_matches })).toBeVisible();
    await expect(page.getByRole('heading', { name: zh.history_empty })).toHaveCount(0);
    await capture(page, 'zh-no-matches');
    await page.getByRole('button', { name: zh.clear_filters, exact: true }).last().click();
    await expect(page.getByRole('article')).toHaveCount(5);
    const params = new URL(page.url()).searchParams;
    expect(params.has('q')).toBe(false); expect(params.has('offset')).toBe(false);
    expect(params.get('view')).toBe('history'); expect(params.get('preserve')).toBe('keep');
    results.interactions.emptyRecovery = 'No-match copy differs from empty history; its clear action removes query/offset and retains unrelated context';
    await context.close();
  }
  {
    const { context, page } = await openCase('empty-page', 'zh-Hans', 'light', 900, '&offset=50&result=reject&preserve=keep');
    await expect(page.getByRole('heading', { name: zh.history_page_empty })).toBeVisible();
    await expect(page.getByRole('heading', { name: zh.history_no_matches })).toHaveCount(0);
    await capture(page, 'zh-empty-page');
    await page.getByRole('button', { name: zh.history_first_page, exact: true }).click();
    await expect(page.getByRole('article')).toHaveCount(2);
    const params = new URL(page.url()).searchParams;
    expect(params.has('offset')).toBe(false); expect(params.get('result')).toBe('reject');
    expect(params.get('view')).toBe('history'); expect(params.get('preserve')).toBe('keep');
    results.interactions.emptyPageRecovery = 'A positive total on an empty page offers first-page recovery and retains filter/view/unrelated parameters';
    await context.close();
  }
  {
    const { context, page } = await openCase('empty', 'en', 'light', 900, '&empty=1');
    await expect(page.getByRole('heading', { name: labels.en.history_empty })).toBeVisible();
    await expect(page.getByRole('heading', { name: labels.en.history_no_matches })).toHaveCount(0);
    await expect(page.getByRole('button', { name: labels.en.clear_filters, exact: true })).toHaveCount(0);
    await capture(page, 'en-empty-history');
    results.interactions.emptyHistory = 'Unfiltered empty history retains its original message without a misleading clear action';
    await context.close();
  }
  expect(results.errors).toEqual([]);
  console.log(JSON.stringify({ screenshots: results.screenshots.length, interactions: results.interactions, errors: results.errors }, null, 2));
} catch (error) {
  results.failure = error.stack;
  console.error(error.stack);
  process.exitCode = 1;
} finally {
  await writeFile(path.join(out, 'metrics.json'), JSON.stringify(results, null, 2));
  await browser.close();
}
