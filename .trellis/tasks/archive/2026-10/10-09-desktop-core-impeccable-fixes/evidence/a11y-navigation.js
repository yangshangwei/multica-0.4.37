await fetch('http://127.0.0.1:9252/zoom?factor=1');
await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
await expect.poll(() => page.evaluate(() => innerWidth)).toBe(1440);
const results = {};
const go = async (name) => {
  const link = page.getByRole('link', { name, exact: true }).first();
  if (!await link.isVisible()) await page.getByRole('button', { name: '切换左侧边栏', exact: true }).first().click();
  await link.click();
};
const focusThroughTab = async (start, target) => {
  await start.focus(); const path = [];
  for (let n=0; n<20; n++) {
    await page.keyboard.press('Tab');
    path.push(await page.evaluate(() => document.activeElement.getAttribute('aria-label') || document.activeElement.textContent?.trim().slice(0,60)));
    if (await target.evaluate((node) => node === document.activeElement)) return path;
  }
  throw new Error('Target not reached in twenty Tab steps');
};
await go('智能体');
const selectAll = page.getByRole('checkbox', { name:'选择当前列表中的所有智能体', exact:true });
const agentCheckbox = page.getByRole('checkbox', { name:'选择 测试工程师', exact:true });
results.agentTabPath = await focusThroughTab(selectAll, agentCheckbox);
for (const theme of ['light','dark']) {
  await page.evaluate((theme) => { document.documentElement.classList.remove('light','dark'); document.documentElement.classList.add(theme); },theme);
  await fetch('http://127.0.0.1:9252/resize?width=900&height=700');
  await fetch('http://127.0.0.1:9252/zoom?factor=2');
  await expect.poll(() => page.evaluate(() => innerWidth)).toBe(450);
  await focusThroughTab(selectAll, agentCheckbox);
  await page.keyboard.press('Space'); await expect(agentCheckbox).toBeChecked();
  await expect.poll(() => agentCheckbox.evaluate((node) => getComputedStyle(node).opacity)).toBe('1');
  const rect = await agentCheckbox.boundingBox(); expect(rect.width).toBeGreaterThan(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  await fetch(`http://127.0.0.1:9252/capture?name=agents-450-${theme}`);
  await page.keyboard.press('Space');
  await fetch('http://127.0.0.1:9252/zoom?factor=1');
  await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
}
results.agentCompact = { themes:['light','dark'], viewport:'450×350 (900×700, native zoom 2)', selectionReachable:true, noOverflow:true };
await page.getByRole('link', { name:'测试工程师', exact:true }).click();
const agentTabs = page.getByRole('tablist', { name:'智能体页面', exact:true });
const tabs = agentTabs.getByRole('tab');
await tabs.first().focus(); await page.keyboard.press('ArrowRight');
await expect(tabs.nth(1)).toBeFocused(); await expect(tabs.first()).toHaveAttribute('aria-selected','true');
await page.keyboard.press('Enter'); await expect(tabs.nth(1)).toHaveAttribute('aria-selected','true');
const panelId = await tabs.nth(1).getAttribute('aria-controls'); await expect(page.locator(`[id="${panelId}"]`)).toBeVisible();
await page.keyboard.press('Home'); await expect(tabs.first()).toBeFocused();
await page.keyboard.press('Space'); await expect(tabs.first()).toHaveAttribute('aria-selected','true');
await shot('agent-tabs-keyboard');
results.agentTabs = { manualActivation:true, homeSpace:true, controlledPanel:true };
await go('技能库');
await page.getByRole('link', { name:'架构决策记录', exact:true }).click();
const skillTabs = page.getByRole('tablist', { name:'技能分区', exact:true }).getByRole('tab');
await skillTabs.first().focus(); await page.keyboard.press('End'); await expect(skillTabs.last()).toBeFocused();
await page.keyboard.press('Enter'); await expect(skillTabs.last()).toHaveAttribute('aria-selected','true');
const fileTabs = page.getByRole('tablist', { name:'技能文件', exact:true }).getByRole('tab');
await fileTabs.first().focus(); await page.keyboard.press('Home'); await expect(fileTabs.first()).toBeFocused();
await shot('skill-tabs-keyboard');
results.skillTabs = { endEnter:true, fileTabRoving:true, controlledPanel:await fileTabs.first().getAttribute('aria-controls') };

const tabOrder = () => page.locator('button[aria-keyshortcuts]').evaluateAll((buttons) => buttons.map((node) => {
  let fiber = node[Object.keys(node).find((key) => key.startsWith('__reactFiber'))];
  while(fiber && !fiber.memoizedProps?.tab) fiber=fiber.return;
  return { id:fiber.memoizedProps.tab.id, current:node.getAttribute('aria-current'), title:node.getAttribute('aria-label') };
}));
const before = await tabOrder(); const current = page.locator('button[aria-current="page"][aria-keyshortcuts]');
await current.focus(); await page.keyboard.press('Alt+Shift+ArrowLeft');
await expect.poll(async () => (await tabOrder())[0].id).toBe(before.at(-1).id);
await expect(current).toBeFocused();
await current.click({ button:'right' });
await page.getByRole('menuitem', { name:'向右移动', exact:true }).click();
expect((await tabOrder()).map((tab) => tab.id)).toEqual(before.map((tab) => tab.id));
await expect(current).toBeFocused();
await shot('desktop-tab-reorder');
results.desktopTabs = { before, after:await tabOrder(), keyboardMove:true, clickAlternative:true, focusRetained:true };
await record('a11y-browser',results);
console.log(JSON.stringify({ checks:Object.keys(results), allPassed:true }));
