await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
await expect.poll(() => page.evaluate(() => innerWidth)).toBe(1440);
await page.getByRole('link', { name: /^分拣台|^Triage/ }).first().click();
const requestedOffsets = [];
page.on('request', (request) => {
  const url = new URL(request.url());
  if (url.pathname === '/api/triage/items') requestedOffsets.push(Number(url.searchParams.get('offset') ?? 0));
});
const enablePagination = async () => {
  await expect.poll(() => page.evaluate(() => {
    const node = document.querySelector('h1');
    let fiber = node[Object.keys(node).find((key) => key.startsWith('__reactFiber'))];
    while (fiber && !fiber.memoizedProps?.client?.getQueryCache) fiber = fiber.return;
    const client = fiber.memoizedProps.client;
    const query = client.getQueryCache().getAll().find((query) => query.queryKey[0] === 'triage' && query.queryKey[2] === 'list' && query.getObserversCount());
    if (!query?.state.data || query.state.fetchStatus !== 'idle') return false;
    client.setQueryData(query.queryKey, { ...query.state.data, total: 101 });
    return true;
  })).toBe(true);
};
await page.reload();
await fetch('http://127.0.0.1:9252/resize?width=900&height=700');
const layouts = [];
for (const locale of ['zh-Hans', 'en']) {
  await page.evaluate((locale) => localStorage.setItem('multica-locale', locale), locale);
  await page.reload();
  for (const factor of [1, 2]) {
    await enablePagination();
    await fetch(`http://127.0.0.1:9252/zoom?factor=${factor}`);
    await expect.poll(() => page.evaluate(() => innerWidth)).toBe(900 / factor);
    const next = page.getByRole('button', { name: locale === 'en' ? 'Next page' : '下一页', exact: true });
    await expect(next).toBeEnabled();
    const nextBox = await next.boundingBox(); const fabBox = await page.locator('[data-chat-launcher]').boundingBox();
    const overlap = nextBox.x < fabBox.x + fabBox.width && nextBox.x + nextBox.width > fabBox.x && nextBox.y < fabBox.y + fabBox.height && nextBox.y + nextBox.height > fabBox.y;
    expect(overlap).toBe(false);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth);
    expect(overflow).toBeLessThanOrEqual(1);
    const capture = await fetch(`http://127.0.0.1:9252/capture?name=triage-${locale.toLowerCase()}-${factor === 2 ? '200pct' : '900'}`);
    if (!capture.ok) throw new Error('Native screenshot capture failed');
    await next.click();
    await expect(page.getByRole('button', { name: locale === 'en' ? 'Previous page' : '上一页', exact: true })).toBeEnabled();
    await page.getByRole('button', { name: locale === 'en' ? 'Previous page' : '上一页', exact: true }).click();
    await expect(next).toBeEnabled();
    layouts.push({ locale, factor, nextBox, fabBox, overlap, overflow });
  }
  await fetch('http://127.0.0.1:9252/zoom?factor=1');
}
expect(requestedOffsets).toContain(50);
await record('triage-browser', { fixture: 'Isolated renderer Query-cache total 101, real offset GET requests; no queue writes or parser claim', requestedOffsets, layouts });
await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
await page.evaluate(() => localStorage.setItem('multica-locale', 'zh-Hans'));
await page.reload();

const toggle = page.getByRole('button', { name: '切换左侧边栏', exact: true }).first();
const header = page.locator('header').filter({ has: page.getByRole('button', { name: 'New tab', exact: true }) });
const sample = () => header.evaluate(async (header) => {
  const values = [];
  for (let n=0; n<6; n++) {
    await new Promise((resolve) => requestAnimationFrame(resolve));
    const drag = header.firstElementChild;
    const canvas = header.nextElementSibling;
    values.push({ padding: parseFloat(header.style.paddingLeft), left: parseFloat(drag.style.left), margin: parseFloat(canvas.style.marginLeft) });
  }
  return values;
});
await page.emulateMedia({ reducedMotion: 'reduce' });
await toggle.click();
const reduced = await sample();
const target = reduced[0];
expect(reduced.every((row) => row.padding === target.padding && row.left === target.left && row.margin === target.margin)).toBe(true);
await page.emulateMedia({ reducedMotion: 'no-preference' });
await toggle.click();
const intermediate = await sample();
expect(new Set(intermediate.map((row) => row.padding)).size).toBeGreaterThan(1);
await page.emulateMedia({ reducedMotion: 'reduce' });
const interrupted = await sample();
expect(interrupted.every((row) => (row.padding === 0 || row.padding === 184) && row.left === row.padding && (row.margin === 2 || row.margin === 8))).toBe(true);
await record('motion-browser', { reduced, intermediate, interrupted, startupAndDynamicRegression: 'desktop-layout.test.tsx', liveInterruptedSpringStopped: true });
await page.emulateMedia({ reducedMotion: 'no-preference' });
console.log(JSON.stringify({ paginationLayouts: layouts.length, nextOffset50: true, reducedInstant: true, liveInterruptedSpringStopped: true }));
await page.evaluate(() => localStorage.setItem('multica-locale', 'zh-Hans'));
await page.reload();
