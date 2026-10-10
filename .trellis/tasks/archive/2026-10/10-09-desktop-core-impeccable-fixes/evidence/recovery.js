const results = [];
const invalidate = async (family) => page.evaluate(async (family) => {
  const node = document.querySelector('h1') ?? document.querySelector('#root button');
  let fiber = node[Object.keys(node).find((key) => key.startsWith('__reactFiber'))];
  while (fiber && !fiber.memoizedProps?.client?.getQueryCache) fiber = fiber.return;
  if (!fiber) throw new Error('QueryClient provider unavailable');
  await fiber.memoizedProps.client.invalidateQueries({ predicate: (query) => query.queryKey[0] === family && query.queryKey[1] !== '' });
}, family);

for (const spec of [
  { name: 'projects', link: '项目', endpoint: /\/api\/projects(?:\?.*)?$/, error: '无法加载项目', refresh: '无法刷新项目', empty: '还没有项目' },
  { name: 'inbox', link: '收件箱', endpoint: /\/api\/inbox(?:\?.*)?$/, error: '无法加载通知', refresh: '无法刷新通知', empty: '暂无通知' },
]) {
  await page.getByRole('link', { name: spec.link, exact: true }).first().click();
  let failures = 0;
  const fail = (route) => { failures++; return route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'audit_failure_injection' }) }); };
  await page.route(spec.endpoint, fail);
  await page.reload();
  await expect(page.getByRole('alert').filter({ hasText: spec.error })).toBeVisible();
  await expect(page.getByText(spec.empty, { exact: true })).toHaveCount(0);
  await shot(`${spec.name}-cold-500-light`);
  const initialAlert = await page.getByRole('alert').first().textContent();
  await page.unroute(spec.endpoint, fail);
  await page.getByRole('button', { name: '重试', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  const successfulContent = await page.locator('body').ariaSnapshot();
  const row = spec.name === 'projects' ? page.locator('a[href*="/projects/"]').first() : page.locator('[role="row"]').first();
  if (await row.count()) await row.evaluate((node) => { window.__auditRetainedRow = node; });
  await page.route(spec.endpoint, fail);
  await invalidate(spec.name === 'projects' ? 'projects' : 'inbox');
  await expect(page.getByRole('alert').filter({ hasText: spec.refresh })).toBeVisible();
  const retainedRow = await page.evaluate(() => !window.__auditRetainedRow || window.__auditRetainedRow.isConnected);
  expect(retainedRow).toBe(true);
  await shot(`${spec.name}-refresh-500-light`);
  await page.unroute(spec.endpoint, fail);
  await page.getByRole('button', { name: '重试', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  results.push({ name: spec.name, initialAlert, failures, retried: true, retainedRow, successfulContent: successfulContent.slice(-1200) });
}
await record('recovery-browser', results);
console.log(JSON.stringify(results.map(({ successfulContent, ...result }) => result)));
