const checks = [];
// Capture only after Tab/theme repaint: finite animations done, two frames presented.
const settle = async () => {
  await expect.poll(() => page.evaluate(() => document.getAnimations()
    .filter((a) => a.playState === 'running' && a.effect?.getTiming().iterations !== Infinity).length), { timeout: 5000 }).toBe(0);
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
  await page.waitForTimeout(150);
};
await fetch('http://127.0.0.1:9252/zoom?factor=1');
await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
for (const locale of ['zh-Hans', 'en']) {
  await page.evaluate((locale) => localStorage.setItem('multica-locale', locale), locale);
  await page.reload();
  const language = locale === 'en' ? 'en' : 'zh-CN';
  await expect.poll(() => page.locator('html').getAttribute('lang')).toBe(language);
  const clearName = locale === 'en' ? 'Clear selection' : '清除选择';
  const rowName = locale === 'en' ? 'Select 测试工程师' : '选择 测试工程师';
  if (!await page.getByRole('checkbox', { name: rowName, exact: true }).count()) {
    const link = page.getByRole('link', { name: locale === 'en' ? 'Agents' : '智能体', exact: true }).first();
    if (!await link.isVisible()) await page.getByRole('button', { name: locale === 'en' ? 'Toggle left sidebar' : '切换左侧边栏', exact: true }).first().click();
    await link.click();
  }
  const clear = page.getByRole('button', { name: clearName, exact: true });
  if (await clear.count()) { await clear.click(); await expect(clear).toHaveCount(0); }
  const row = page.getByRole('checkbox', { name: rowName, exact: true });
  await row.focus(); await row.press('Space'); await expect(row).toBeChecked();
  const toolbar = clear.locator('..').locator('..');
  await expect.poll(() => toolbar.evaluate((node) => getComputedStyle(node).opacity)).toBe('1');
  if (locale === 'zh-Hans') {
    await clear.focus();
    await settle();
    await shot('agents-toolbar-wide-confirmed');
    const position = await toolbar.locator('..').evaluate((node) => getComputedStyle(node).position);
    expect(position).toBe('absolute');
  }
  await fetch('http://127.0.0.1:9252/resize?width=900&height=700');
  await fetch('http://127.0.0.1:9252/zoom?factor=2');
  await expect.poll(() => page.evaluate(() => innerWidth)).toBe(450);
  for (const theme of locale === 'en' ? ['light'] : ['light', 'dark']) {
    await page.evaluate((theme) => { document.documentElement.classList.remove('light','dark'); document.documentElement.classList.add(theme); }, theme);
    await clear.scrollIntoViewIfNeeded(); await clear.focus();
    await page.keyboard.press('Tab');
    const access = page.getByRole('button', { name:locale === 'en' ? 'Set access scope' : '设置访问范围', exact:true });
    await expect(access).toBeFocused();
    const geometry = await toolbar.evaluate((node) => {
      const rect = node.getBoundingClientRect(); const count = node.firstElementChild.querySelector('span');
      const countStyle = getComputedStyle(count); const countRect = count.getBoundingClientRect();
      const header = document.querySelector('[data-slot="list-grid-header"]') ?? document.querySelector('[role="columnheader"]')?.parentElement;
      const headerRect = header?.getBoundingClientRect();
      const overlaps = (a,b) => a && b && a.x < b.right && a.right > b.x && a.y < b.bottom && a.bottom > b.y;
      const launcher = document.querySelector('[data-chat-launcher]')?.getBoundingClientRect();
      return { toolbar:rect.toJSON(), count:countRect.toJSON(), whiteSpace:countStyle.whiteSpace,
        header:headerRect?.toJSON(), headerOverlap:!!overlaps(rect,headerRect), launcherOverlap:!!overlaps(rect,launcher),
        overflow:document.documentElement.scrollWidth-innerWidth, compactPosition:getComputedStyle(node.parentElement).position };
    });
    expect(geometry.headerOverlap).toBe(false); expect(geometry.launcherOverlap).toBe(false);
    expect(geometry.overflow).toBeLessThanOrEqual(1); expect(geometry.whiteSpace).toBe('nowrap');
    expect(geometry.count.height).toBeLessThan(25); expect(geometry.compactPosition).not.toBe('absolute');
    await settle();
    await expect(access).toBeFocused();
    const focusVisible = await access.evaluate((node) => node.matches(':focus-visible'));
    expect(focusVisible).toBe(true);
    const capture = await fetch(`http://127.0.0.1:9252/capture?name=agents-450-${locale.toLowerCase()}-${theme}-confirmed`);
    expect(capture.ok).toBe(true);
    checks.push({locale,theme,focused:await access.getAttribute('aria-label') ?? await access.innerText(),focusVisible,...geometry});
  }
  await clear.focus(); await clear.press('Space'); await expect(clear).toHaveCount(0);
  await fetch('http://127.0.0.1:9252/zoom?factor=1');
  await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
}
await record('toolbar-browser-confirmation',{settled:'captured after finite animations finished and two frames presented',checks,wideFloating:true,keyboardActionsReachable:true,keyboardClear:true,
  mixedArchiveNote:'Active and archived scopes are mutually exclusive in the page; real mixed toolbar actions remain covered by the canonical toolbar DOM suite.'});
console.log(JSON.stringify({layouts:checks.length,noHeaderOrLauncherOverlap:true,keyboardActionsReachable:true,wideFloating:true}));
