// Recaptures the issue-list keyboard selection screenshots from a11y.js after
// the theme colour transition settles (the 13:30 dark capture was mid-fade).
const settle = async () => {
  await expect.poll(() => page.evaluate(() => document.getAnimations()
    .filter((a) => a.playState === 'running' && a.effect?.getTiming().iterations !== Infinity).length), { timeout: 5000 }).toBe(0);
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
  await page.waitForTimeout(150);
};
await fetch('http://127.0.0.1:9252/zoom?factor=1');
await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
await page.evaluate(() => localStorage.setItem('multica-locale', 'zh-Hans'));
await page.reload();
await expect.poll(() => page.locator('html').getAttribute('lang')).toBe('zh-CN');
const link = page.getByRole('link', { name: '任务', exact: true }).first();
if (!await link.isVisible()) await page.getByRole('button', { name: '切换左侧边栏', exact: true }).first().click();
await link.click();
const layout = page.getByRole('button', { name: /^(看板|列表|表格|泳道)$/ }).first();
if (await layout.innerText() !== '列表') {
  await layout.click(); await page.getByRole('menuitemradio', { name: '列表', exact: true }).click();
}
const issue = page.getByRole('checkbox', { name: '选择 DQA-11', exact: true });
const results = [];
for (const theme of ['light', 'dark']) {
  await page.evaluate((theme) => { document.documentElement.classList.remove('light', 'dark'); document.documentElement.classList.add(theme); }, theme);
  await issue.focus(); await page.keyboard.press('Space'); await expect(issue).toBeChecked();
  await expect.poll(() => issue.evaluate((node) => getComputedStyle(node).opacity)).toBe('1');
  await settle();
  const state = await issue.evaluate((node) => ({ focused: node === document.activeElement,
    focusVisible: node.matches(':focus-visible'), canvas: getComputedStyle(document.querySelector('div.bg-page-canvas')).backgroundColor }));
  expect(state.focused).toBe(true);
  await shot(`issues-keyboard-${theme}`);
  results.push({ theme, ...state });
  await page.keyboard.press('Space'); await expect(issue).not.toBeChecked();
}
await page.evaluate(() => { document.documentElement.classList.remove('dark'); document.documentElement.classList.add('light'); });
await record('issues-keyboard-settled', { settled: 'captured after finite animations finished and two frames presented', results });
console.log(JSON.stringify(results));
