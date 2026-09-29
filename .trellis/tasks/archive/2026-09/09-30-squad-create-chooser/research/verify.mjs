import { chromium, expect } from '@playwright/test';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 375, height: 667 }, deviceScaleFactor: 2 });
const errors = [];
page.on('pageerror', e => errors.push(e.message));
const output = new URL('.', import.meta.url).pathname;
const base = 'http://localhost:5667/squad-chooser-preview.html';
try {
  for (const locale of ['zh-Hans', 'en']) {
    await page.goto(base + '?locale=' + locale);
    const trigger = page.getByRole('button', { name: locale === 'en' ? 'New Squad' : '新建AI小队', exact: true });
    await trigger.focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog');
    const close = dialog.getByRole('button', { name: locale === 'en' ? 'Close' : '关闭', exact: true });
    await expect(close).toBeFocused();
    await expect.poll(async () => dialog.evaluate(e => ({ x: e.getBoundingClientRect().x, width: e.getBoundingClientRect().width, overflow: e.scrollWidth > e.clientWidth }))).toEqual({ x: 16, width: 343, overflow: false });
    await page.screenshot({ path: output + (locale === 'en' ? 'chooser-english-narrow.png' : 'chooser-narrow.png') });
    if (locale === 'zh-Hans') {
      await page.evaluate(() => document.documentElement.classList.add('dark'));
      await dialog.screenshot({ path: output + 'chooser-dark.png' });
    }
    await page.keyboard.press('Escape');
    await expect(trigger).toBeFocused();
    await trigger.click();
    await expect(close).toBeFocused();
    await close.click();
    await expect(trigger).toBeFocused();
  }
  for (const [option, target, tabs] of [['从模板创建', 'staff-squad-template', 1], ['自定义创建', 'create-squad', 2]]) {
    await page.goto(base);
    await page.getByRole('button', { name: '新建AI小队', exact: true }).click();
    await expect(page.getByRole('button', { name: '关闭', exact: true })).toBeFocused();
    for (let i = 0; i < tabs; i++) await page.keyboard.press('Tab');
    await expect(page.getByRole('button', { name: new RegExp(option) })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('dialog', { name: target })).toBeVisible();
    await expect(page.getByRole('dialog')).toHaveCount(1);
    await expect(page.getByRole('textbox', { name: 'Handoff focus' })).toBeFocused();
  }
  expect(errors).toEqual([]);
  console.log('PASS: Chinese/English at 375px without overflow; light/dark; Enter/Tab navigation; Escape/Close return focus; both modal destinations and sibling-dialog focus handoff; no page errors.');
} finally { await browser.close(); }
