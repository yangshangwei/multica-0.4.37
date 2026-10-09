const categories = ['backlog', 'todo', 'in_progress', 'in_review', 'done', 'blocked', 'cancelled'];
// Theme switches run CSS colour transitions; measure and capture only once
// every finite animation has finished and two frames have been presented.
const settle = async () => {
  await expect.poll(() => page.evaluate(() => document.getAnimations()
    .filter((a) => a.playState === 'running' && a.effect?.getTiming().iterations !== Infinity).length), { timeout: 5000 }).toBe(0);
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
  await page.waitForTimeout(150);
};
await page.reload();
const labels = JSON.parse(await fs.readFile('packages/views/locales/zh-Hans/issues.json', 'utf8')).gantt;
const timeline = page.getByRole('region', { name: labels.timeline_label, exact: true });
await expect(timeline).toBeVisible({ timeout: 15000 });
await page.evaluate((categories) => {
  const node = document.querySelector('[data-gantt-issue-link]');
  let fiber = node[Object.keys(node).find((key) => key.startsWith('__reactFiber'))];
  while (fiber && !fiber.memoizedProps?.client?.getQueryCache) fiber = fiber.return;
  const client = fiber.memoizedProps.client;
  const query = client.getQueryCache().getAll().find((query) => query.queryKey.includes('project-gantt') && query.getObserversCount());
  if (!query?.state.data?.length) throw new Error('Real scheduled query unavailable');
  const template = query.state.data[0];
  window.__ganttOriginal = { client, key: query.queryKey, data: query.state.data };
  const items = Array.from({ length: 1000 }, (_, index) => {
    const category = categories[index % categories.length];
    return { ...template,
      id: `00000000-0000-4000-8000-${String(index + 1).padStart(12, '0')}`,
      identifier: `QA-${index + 1}`, number: index + 1,
      title: `Audit fixture ${category} ${index + 1}`,
      status: category, status_category: category, assignee_id: null, assignee_type: null,
      start_date: index === 0 ? '2020-01-01' : '2026-10-01',
      due_date: index === 0 ? '2030-12-31' : '2026-10-31',
    };
  });
  client.setQueryData(query.queryKey, items);
}, categories);
await expect.poll(() => page.locator('[data-gantt-row-index]').first().getAttribute('aria-setsize')).toMatch(/^(714|715|1000)$/);
if (Number(await page.locator('[data-gantt-row-index]').first().getAttribute('aria-setsize')) !== 1000) {
  await page.getByRole('button', { name: labels.show_completed, exact: true }).click();
}
await expect(page.locator('[data-gantt-row-index]').first()).toHaveAttribute('aria-setsize', '1000');
const contrast = [];
for (const theme of ['light', 'dark']) {
  await page.evaluate((theme) => { document.documentElement.classList.remove('light', 'dark'); document.documentElement.classList.add(theme); }, theme);
  await settle();
  const samples = await page.locator('[data-gantt-row-index]').evaluateAll((rows) => {
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    const rgba = (color) => { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = color; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data]; };
    const blend = (top, bottom) => top.slice(0, 3).map((value, i) => value * (top[3] / 255) + bottom[i] * (1 - top[3] / 255));
    const bg = (node) => node ? blend(rgba(getComputedStyle(node).backgroundColor), bg(node.parentElement)) : [255,255,255];
    const lum = (rgb) => rgb.reduce((total, value, i) => { const c = value/255; return total + (c <= .04045 ? c/12.92 : ((c+.055)/1.055)**2.4)*[.2126,.7152,.0722][i]; }, 0);
    return rows.slice(0, 7).map((row) => {
      const bar = row.querySelector('a[aria-label^="QA-"]'); const title = bar.querySelector('span');
      const style = getComputedStyle(title); const background = bg(bar); const foreground = blend(rgba(style.color), background);
      const l1 = lum(background), l2 = lum(foreground);
      return { title: title.textContent, foreground, background, ratio: (Math.max(l1,l2)+.05)/(Math.min(l1,l2)+.05) };
    });
  });
  expect(samples).toHaveLength(7);
  for (const sample of samples) expect(sample.ratio).toBeGreaterThanOrEqual(4.5);
  contrast.push({ theme, samples });
  await shot(`gantt-${theme}-extreme-fixture`);
}
const zooms = [];
for (const zoom of ['day', 'week', 'month']) {
  await page.getByRole('button', { name: labels[`zoom_${zoom}`], exact: true }).click();
  for (const end of ['first', 'last']) {
    await timeline.evaluate((node, end) => { node.scrollLeft = end === 'first' ? 0 : node.scrollWidth; node.dispatchEvent(new Event('scroll')); }, end);
    await timeline.evaluate(async () => { await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
    const metrics = await timeline.evaluate((node) => ({ elements: node.querySelectorAll('*').length, rows: node.querySelectorAll('[data-gantt-row-index]').length, scrollLeft: node.scrollLeft, scrollWidth: node.scrollWidth, clientWidth: node.clientWidth, axisText: node.firstElementChild.firstElementChild.innerText }));
    expect(metrics.elements).toBeLessThan(1600);
    expect(metrics.rows).toBeLessThan(80);
    if (end === 'last') expect(metrics.scrollLeft + metrics.clientWidth).toBeGreaterThanOrEqual(metrics.scrollWidth - 2);
    expect(metrics.axisText).toMatch(end === 'first' ? /2020/ : /2031/);
    zooms.push({ zoom, end, ...metrics });
  }
}
const firstLink = page.locator('[data-gantt-row-index="0"] [data-gantt-issue-link]');
await firstLink.focus(); await page.keyboard.press('End');
await expect(page.locator('[data-gantt-row-index="999"] [data-gantt-issue-link]')).toBeFocused();
await page.keyboard.press('Home'); await expect(firstLink).toBeFocused();
await settle();
await shot('gantt-keyboard-home');
await record('gantt-browser', { fixture: '1000 isolated renderer Query-cache issues, 2020–2030 range; no backend writes or parser claim', settled: 'colour sampled and captured after all finite animations finished', contrast, zooms, keyboardHomeEnd: true });
console.log(JSON.stringify({ minimumContrast: Math.min(...contrast.flatMap((row) => row.samples.map((sample) => sample.ratio))), maxElements: Math.max(...zooms.map((row) => row.elements)), maxRows: Math.max(...zooms.map((row) => row.rows)), keyboardHomeEnd: true }));
await page.evaluate(() => { const { client, key, data } = window.__ganttOriginal; client.setQueryData(key, data); delete window.__ganttOriginal; });
