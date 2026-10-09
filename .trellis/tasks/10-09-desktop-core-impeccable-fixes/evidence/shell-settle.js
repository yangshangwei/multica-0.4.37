// Replays the 13:45 zh-Hans triage capture (sidebar toggle, 900x700, zoom 1/2)
// and records the shell geometry over time, so an unsettled capture can be told
// apart from a useShellGeometry regression. Captures only after geometry settles
// and two animation frames have been presented.
const geometry = () => page.evaluate(() => {
  const header = document.querySelector('header.relative.shrink-0');
  const drag = header?.querySelector(':scope > div[aria-hidden]');
  const canvas = document.querySelector('div.bg-page-canvas');
  const sidebar = document.querySelector('[data-slot="sidebar"]');
  const firstTab = header?.querySelector('[data-tab-active], [role="tab"], a')?.getBoundingClientRect();
  return {
    innerWidth, zoomWidth: Math.round(innerWidth * devicePixelRatio),
    visibility: document.visibilityState,
    sidebarState: sidebar?.dataset.state ?? null,
    sidebarWidth: Math.round(sidebar?.getBoundingClientRect().width ?? -1),
    paddingLeft: parseFloat(header ? getComputedStyle(header).paddingLeft : 'NaN'),
    dragLeft: parseFloat(drag ? getComputedStyle(drag).left : 'NaN'),
    marginLeft: parseFloat(canvas ? getComputedStyle(canvas).marginLeft : 'NaN'),
    canvasLeft: Math.round(canvas?.getBoundingClientRect().left ?? -1),
    firstTabLeft: Math.round(firstTab?.left ?? -1),
  };
});
const frames = () => page.evaluate(() => new Promise((resolve) => {
  let n = 0; const start = performance.now();
  const tick = () => { n += 1; if (performance.now() - start < 500) requestAnimationFrame(tick); else resolve(n); };
  requestAnimationFrame(tick);
}));
const series = async (label, ms = 1200) => {
  const out = []; const start = Date.now();
  while (Date.now() - start < ms) { out.push({ t: Date.now() - start, ...(await geometry()) }); await page.waitForTimeout(60); }
  return { label, first: out[0], last: out[out.length - 1], samples: out.length,
    distinctPadding: [...new Set(out.map((s) => Math.round(s.paddingLeft)))] };
};
const settled = async () => {
  await expect.poll(async () => { const g = await geometry(); return g.paddingLeft === 184 && g.marginLeft === 8; },
    { timeout: 4000 }).toBe(true);
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
  await page.waitForTimeout(250);
};
const pngSize = async (name) => {
  const buf = await fs.readFile(path.join(dir, 'screenshots', name + '.png'));
  return { width: buf.readUInt32BE(16), height: buf.readUInt32BE(20) };
};

const result = { rafFramesPer500ms: await frames(), steps: [] };
await fetch('http://127.0.0.1:9252/zoom?factor=1');
await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
await page.evaluate(() => localStorage.setItem('multica-locale', 'zh-Hans'));
await page.reload();
await expect.poll(() => page.locator('html').getAttribute('lang')).toBe('zh-CN');
const link = page.getByRole('link', { name: /^分拣台/ }).first();
if (!await link.isVisible()) await page.getByRole('button', { name: '切换左侧边栏', exact: true }).first().click();
await link.click();
await expect(page.getByRole('heading', { name: '分拣台', exact: true })).toBeVisible();
await expect.poll(() => page.evaluate(() => {
  const n = document.querySelector('h1'); let f = n[Object.keys(n).find((k) => k.startsWith('__reactFiber'))];
  while (f && !f.memoizedProps?.client?.getQueryCache) f = f.return;
  const c = f.memoizedProps.client;
  const q = c.getQueryCache().getAll().find((q) => q.queryKey[0] === 'triage' && q.queryKey[2] === 'list' && q.getObserversCount());
  if (!q?.state.data || q.state.fetchStatus !== 'idle') return false;
  c.setQueryData(q.queryKey, { ...q.state.data, total: 101 }); return true;
})).toBe(true);
await page.evaluate(() => { document.documentElement.classList.remove('dark'); document.documentElement.classList.add('light'); });
result.steps.push({ step: 'wide-1440', ...(await geometry()) });
await fetch('http://127.0.0.1:9252/resize?width=900&height=700');
result.steps.push(await series('after-resize-900'));
for (const factor of [1, 2]) {
  await fetch('http://127.0.0.1:9252/zoom?factor=' + factor);
  await expect.poll(() => page.evaluate(() => innerWidth)).toBe(900 / factor);
  result.steps.push(await series('after-zoom-' + factor, 800));
  await expect(page.getByRole('button', { name: '下一页', exact: true })).toBeEnabled();
  await settled();
  const name = 'triage-zh-hans-' + (factor === 2 ? '200pct' : '900');
  const r = await fetch('http://127.0.0.1:9252/capture?name=' + name);
  expect(r.ok).toBe(true);
  const next = await page.getByRole('button', { name: '下一页', exact: true }).boundingBox();
  const fab = await page.locator('[data-chat-launcher]').boundingBox();
  result.steps.push({ step: 'captured-' + name, ...(await geometry()), png: await pngSize(name), next, fab,
    overlap: next.x < fab.x + fab.width && next.x + next.width > fab.x && next.y < fab.y + fab.height && next.y + next.height > fab.y });
}
await fetch('http://127.0.0.1:9252/zoom?factor=1');
await fetch('http://127.0.0.1:9252/resize?width=1440&height=1000');
await record('shell-settle-browser', result);
console.log(JSON.stringify(result.steps.map((s) => s.label ?? s.step).concat([result.rafFramesPer500ms])));
console.log(JSON.stringify(result, null, 1).slice(0, 6000));
