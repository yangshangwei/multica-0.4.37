const output = { recordedAt: new Date().toISOString(), cases: [] };
await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
for (const tab of ['members', 'daemon', 'behavior']) {
  await navigate(tab);
  const tree = await cdp.send('Accessibility.getFullAXTree');
  const unnamed = tree.nodes.filter(node => !node.ignored && ['combobox','switch'].includes(node.role?.value) && !(node.name?.value || '').trim()).map(node => ({ role: node.role.value, name: node.name?.value, value: node.value?.value }));
  output.cases.push({ case: `${tab}-accessible-names`, unnamed, confirmed: unnamed.length > 0 });
  await fs.writeFile(path.join(dir, 'ax', `resume-${tab}-names.txt`), await page.locator('body').ariaSnapshot());
}

await control('/ipc', { channel: 'updater:check', mode: 'normal' });
await navigate('updates');
const updatePanel = page.getByRole('tabpanel', { name: '更新', exact: true });
await updatePanel.getByRole('button', { name: '立即检查', exact: true }).click();
const updateMessage = updatePanel.getByText('已是最新版本。', { exact: true });
await expect(updateMessage).toBeVisible();
const update = await updateMessage.evaluate(element => ({
  text: element.textContent.trim(),
  announcementAncestor: element.closest('[role="status"],[role="alert"],[aria-live]')?.outerHTML.slice(0, 300) ?? null,
}));
output.cases.push({ case: 'update-result-announcement', ...update, confirmed: update.announcementAncestor === null });
await fs.writeFile(path.join(dir, 'ax', 'resume-update-result.txt'), await page.locator('body').ariaSnapshot());
await shot('resume-update-result');

await control('/viewport', { width: 900, height: 900, zoom: 1 });
await navigate('labels');
const target = page.getByRole('button', { name: '桌面场景 · 功能 的操作', exact: true });
const before = await target.evaluate(element => {
  const r = element.getBoundingClientRect(), card = element.closest('.overflow-hidden');
  return { x: r.x, right: r.right, width: r.width, viewport: innerWidth, cardWidth: card.clientWidth, cardScrollWidth: card.scrollWidth, overflowX: getComputedStyle(card).overflowX };
});
await target.focus();
const after = await target.evaluate(element => {
  const r = element.getBoundingClientRect();
  return { x: r.x, right: r.right, focused: element === document.activeElement, cardScrollLeft: element.closest('.overflow-hidden').scrollLeft };
});
output.cases.push({ case: 'label-menu-clipping', before, after, confirmed: before.x > before.viewport, qualification: 'Keyboard focus scrolls the overflow-hidden card to reveal the menu; pointer users have no visible menu or scrollbar.' });
await shot('resume-label-menu-focused-900');

await control('/viewport', { width: 900, height: 1000, zoom: 2 });
await navigate('mcp');
const outerPanel = page.getByRole('tabpanel', { name: 'MCP', exact: true });
const metrics = await outerPanel.evaluate(element => ({ width: innerWidth, height: innerHeight, documentWidth: document.documentElement.scrollWidth, panelWidth: element.clientWidth, nestedPanels: element.querySelectorAll('[role="tabpanel"]').length }));
const { scanContrast } = await import(path.resolve(dir, '../2026-10-11-desktop-core/scripts/contrast.mjs'));
const contrast = await scanContrast(page);
output.cases.push({ case: 'mcp-200pct-completed', ...metrics, contrastChecked: contrast.checked, contrastFailures: contrast.failures, confirmed: true, qualification: 'The earlier strict-mode locator error was due to nested MCP tabs, not a product failure.' });
await shot('resume-mcp-light-200pct');
await fs.writeFile(path.join(dir, 'ax', 'resume-mcp-light-200pct.txt'), await page.locator('body').ariaSnapshot());

await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
await navigate('profile');
await fs.writeFile(path.join(dir, 'probe-final-evidence-results.json'), JSON.stringify(output, null, 2));
console.log(JSON.stringify(output));
