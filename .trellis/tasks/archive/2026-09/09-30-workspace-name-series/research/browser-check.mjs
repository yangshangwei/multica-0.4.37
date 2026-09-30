import { chromium, expect } from '@playwright/test';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';

// Exercise the live Web shell with an isolated API fixture; no real account or
// workspace is created. Keep screenshots outside the source tree.
const baseURL = process.env.NAMING_QA_URL || 'http://localhost:13493';
const output = '/tmp/workspace-name-series';
mkdirSync(output, { recursive: true });
const browser = await chromium.launch({ headless: true });
const report = [];
try {
  for (const locale of ['en', 'zh-Hans']) {
    const copy = JSON.parse(readFileSync(`packages/views/locales/${locale}/workspace.json`)).name_picker;
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
    await context.addCookies([{ name: 'multica-locale', value: locale, url: baseURL }]);
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    const user = { id: '11111111-1111-4111-8111-111111111111', email: 'naming-fixture@localhost', name: 'Naming fixture', language: locale, onboarded_at: null, onboarding_questionnaire: { role: 'engineer', use_case: ['ship_code'] } };
    await page.route('**/api/**', route => {
      const path = new URL(route.request().url()).pathname;
      return route.fulfill({ json: path.startsWith('/api/me') ? user : path === '/api/config' ? { workspace_creation_disabled: false } : [] });
    });
    await page.addInitScript(() => localStorage.setItem('multica_token', 'isolated-naming-fixture'));
    const enter = async () => {
      await page.goto(`${baseURL}/onboarding`);
      await page.getByRole('button', { name: locale === 'en' ? 'Continue on web' : '在 web 端继续', exact: true }).click();
      await page.getByRole('button', { name: locale === 'en' ? 'Continue' : '继续', exact: true }).click();
      await page.locator('#ws-name').waitFor();
    };
    await enter();
    const name = page.locator('#ws-name');
    const slug = page.locator('#ws-slug');
    const prefix = page.locator('#ws-issue-prefix');
    const random = page.getByRole('button', { name: copy.random, exact: true });
    const choose = page.getByRole('button', { name: copy.choose_series, exact: true });
    await expect(page.getByText(copy.current_series.replace('{{series}}', copy.series.workshop), { exact: true })).toBeVisible();
    const seen = new Set();
    for (let i = 0; i < 8; i++) {
      await random.click();
      const value = await name.inputValue();
      expect(seen.has(value)).toBe(false);
      seen.add(value);
      await expect(slug).toHaveValue(/^[a-z0-9]+(?:-[a-z0-9]+)*-[a-z0-9]{4}$/);
      expect(await prefix.inputValue()).toBe((await slug.inputValue()).replace(/[^a-z0-9]/g, '').slice(0, 4).toUpperCase());
    }
    const shot = async suffix => {
      // Wait for the incumbent shell/menu transitions, then disable remaining
      // CSS animations so visual evidence never captures a half-faded popup.
      await page.waitForTimeout(400);
      await page.screenshot({ path: `${output}/${locale}-${suffix}.png`, animations: 'disabled' });
    };
    await shot('desktop');
    const before = [await name.inputValue(), await slug.inputValue(), await prefix.inputValue()];
    await choose.click();
    await expect(page.getByRole('menuitemradio')).toHaveCount(7);
    await shot('menu');
    await page.getByRole('menuitemradio').filter({ hasText: copy.series.nature }).click();
    expect([await name.inputValue(), await slug.inputValue(), await prefix.inputValue()]).toEqual(before);
    await expect(choose).toBeFocused();
    await random.click();
    expect(await name.inputValue()).not.toBe(before[0]);
    const generatedSlug = await slug.inputValue();
    await name.fill('Edited workspace');
    await expect(slug).toHaveValue(generatedSlug);
    await slug.fill('my-team');
    await prefix.fill('TEAM');
    await random.click();
    await expect(slug).toHaveValue('my-team');
    await expect(prefix).toHaveValue('TEAM');
    await choose.focus();
    await page.keyboard.press('ArrowDown');
    await expect(page.getByRole('menu')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('menu')).toBeHidden();
    await expect(choose).toBeFocused();
    await page.setViewportSize({ width: 390, height: 844 });
    await shot('narrow');
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
    await choose.click();
    await shot('narrow-menu');
    const bounds = await page.getByRole('menu').boundingBox();
    expect(bounds.x).toBeGreaterThanOrEqual(0);
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(390);
    expect(bounds.y + bounds.height).toBeLessThanOrEqual(844);
    await page.keyboard.press('Escape');
    await enter();
    await expect(page.getByText(copy.current_series.replace('{{series}}', copy.series.nature), { exact: true })).toBeVisible();
    expect(errors).toEqual([]);
    report.push({ locale, passed: true, uniqueNames: seen.size, noHorizontalOverflow: true, restoredPreference: 'nature', pageErrors: errors });
    await context.close();
  }
} finally {
  await browser.close();
}
writeFileSync(`${output}/report.json`, `${JSON.stringify(report, null, 2)}\n`);
console.log(JSON.stringify(report, null, 2));
