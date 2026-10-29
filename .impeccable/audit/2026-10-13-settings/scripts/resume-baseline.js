const { scanContrast } = await import(path.resolve(dir, '../2026-10-11-desktop-core/scripts/contrast.mjs'));
const tabs = ['profile','preferences','notifications','shortcuts','tokens','workspace','members','labels','triage','iterations','issue-statuses','properties','quick-actions','repositories','integrations','mcp','daemon','server','updates','behavior'];
const variants = [
  { name: 'light-200pct', width: 900, height: 1000, zoom: 2, dark: false, tabs },
  { name: 'light-900', width: 900, height: 900, zoom: 1, dark: false, tabs: ['labels','properties','quick-actions','members','shortcuts'] },
  { name: 'dark-900', width: 900, height: 900, zoom: 1, dark: true, tabs: ['labels','properties','quick-actions'] },
];
const output = { recordedAt: new Date().toISOString(), environment: 'Electron 39.8.7 on macOS, Windows settings branch via isolated IPC fixture', variants: [] };
const interactiveRoles = new Set(['button','link','textbox','combobox','checkbox','switch','radio','tab','slider','spinbutton','searchbox']);
for (const variant of variants) {
  await control('/viewport', variant);
  await page.evaluate(dark => {
    document.documentElement.classList.toggle('dark', dark);
    document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
  }, variant.dark);
  await page.waitForTimeout(400);
  for (const tab of variant.tabs) {
    const name = `resume-${tab}-${variant.name}`;
    try {
      await navigate(tab);
      const panel = page.getByRole('tabpanel');
      const metrics = await panel.evaluate(panel => {
        const overwide = [...panel.querySelectorAll('*')].filter(el =>
          el.clientWidth > 0 && el.scrollWidth > el.clientWidth + 2 &&
          /(hidden|auto|scroll)/.test(getComputedStyle(el).overflowX),
        ).map(el => ({ tag: el.tagName, class: String(el.className).slice(0, 140), client: el.clientWidth, scroll: el.scrollWidth, overflowX: getComputedStyle(el).overflowX }));
        const clippedControls = [...panel.querySelectorAll('button,input,textarea,[role="switch"],[role="combobox"]')].filter(el => el.getBoundingClientRect().width > 0).flatMap(el => {
          const rect = el.getBoundingClientRect();
          for (let ancestor = el.parentElement; ancestor; ancestor = ancestor.parentElement) {
            const bound = ancestor.getBoundingClientRect();
            if (getComputedStyle(ancestor).overflowX === 'hidden' && (rect.right > bound.right + 2 || rect.left < bound.left - 2)) {
              return [{ name: el.getAttribute('aria-label') || el.textContent.trim().slice(0, 50), role: el.getAttribute('role') || el.tagName, left: Math.round(rect.left), right: Math.round(rect.right), clipLeft: Math.round(bound.left), clipRight: Math.round(bound.right) }];
            }
          }
          return [];
        });
        return { width: innerWidth, height: innerHeight, documentWidth: document.documentElement.scrollWidth, panelWidth: panel.clientWidth, overwide, clippedControls };
      });
      const { nodes } = await cdp.send('Accessibility.getFullAXTree');
      const unnamed = nodes.filter(n => !n.ignored && interactiveRoles.has(n.role?.value) && !(n.name?.value || '').trim()).map(n => ({ role: n.role.value, backendDOMNodeId: n.backendDOMNodeId }));
      const contrast = await scanContrast(page);
      await shot(name);
      await fs.writeFile(path.join(dir, 'ax', `${name}.txt`), await page.locator('body').ariaSnapshot());
      output.variants.push({ name, tab, ...metrics, unnamed, contrastChecked: contrast.checked, contrastFailures: contrast.failures });
      console.log(JSON.stringify({ name, width: metrics.width, overwide: metrics.overwide.length, clipped: metrics.clippedControls.length, unnamed: unnamed.length, contrast: contrast.failures.length }));
    } catch (error) {
      output.variants.push({ name, tab, error: error.message });
      console.log(name, 'ERROR', error.message.slice(0, 160));
    }
    await fs.writeFile(path.join(dir, 'resume-baseline.json'), JSON.stringify(output, null, 2));
  }
}
await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
await page.evaluate(() => { document.documentElement.classList.remove('dark'); document.documentElement.style.colorScheme = 'light'; });
